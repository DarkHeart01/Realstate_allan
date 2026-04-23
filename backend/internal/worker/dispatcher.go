// internal/worker/dispatcher.go
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/realestate/backend/internal/models"
)

// TwilioSender abstracts the Twilio Messages API so handlers can be tested
// without making real HTTP calls.  The production implementation is
// *twilioHTTPSender; tests inject a mock.
type TwilioSender interface {
	SendWhatsApp(ctx context.Context, to, message string) (string, error)
	SendSMS(ctx context.Context, to, message string) (string, error)
}

// twilioHTTPSender is the real TwilioSender that calls the Twilio REST API.
type twilioHTTPSender struct {
	sid      string
	auth     string
	waFrom   string // "whatsapp:+14155238886"
	smsFrom  string // "+14155238886"
	httpClient *http.Client
}

func (s *twilioHTTPSender) SendWhatsApp(ctx context.Context, to, message string) (string, error) {
	return s.send(ctx, s.waFrom, "whatsapp:"+normalisePhone(to), message)
}

func (s *twilioHTTPSender) SendSMS(ctx context.Context, to, message string) (string, error) {
	return s.send(ctx, s.smsFrom, normalisePhone(to), message)
}

func (s *twilioHTTPSender) send(ctx context.Context, from, to, body string) (string, error) {
	endpoint := fmt.Sprintf(
		"https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json",
		s.sid,
	)
	form := url.Values{}
	form.Set("From", from)
	form.Set("To", to)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("twilio: build request: %w", err)
	}
	req.SetBasicAuth(s.sid, s.auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("twilio: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result struct {
		SID          string `json:"sid"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"message"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("twilio: decode response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("twilio: API error %d: %s", result.ErrorCode, result.ErrorMessage)
	}
	return result.SID, nil
}

// Dispatcher holds shared dependencies for all worker task handlers.
// Construct it once at startup, then pass to NewMux.
type Dispatcher struct {
	db           *pgxpool.Pool
	workerClient *Client // for enqueueing follow-on tasks
	sender       TwilioSender
	gcsBucket    string // used to form GCS URIs for Vision API

	// Kept for legacy twilioSend callers within this file; injected sender takes
	// precedence in HandleWhatsApp / HandleSMS.
	twilioSID     string
	twilioAuth    string
	twilioWAFrom  string
	twilioSMSFrom string
}

// DispatcherConfig holds all Dispatcher constructor inputs.
type DispatcherConfig struct {
	DB             *pgxpool.Pool
	WorkerClient   *Client
	TwilioSID      string
	TwilioAuth     string
	TwilioWAFrom   string
	TwilioSMSFrom  string
	GCSBucket      string
}

// NewDispatcher constructs a Dispatcher.
func NewDispatcher(cfg DispatcherConfig) *Dispatcher {
	d := &Dispatcher{
		db:            cfg.DB,
		workerClient:  cfg.WorkerClient,
		twilioSID:     cfg.TwilioSID,
		twilioAuth:    cfg.TwilioAuth,
		twilioWAFrom:  cfg.TwilioWAFrom,
		twilioSMSFrom: cfg.TwilioSMSFrom,
		gcsBucket:     cfg.GCSBucket,
	}
	// Build a real TwilioSender only when credentials are present.
	if cfg.TwilioSID != "" && cfg.TwilioAuth != "" {
		d.sender = &twilioHTTPSender{
			sid:     cfg.TwilioSID,
			auth:    cfg.TwilioAuth,
			waFrom:  cfg.TwilioWAFrom,
			smsFrom: cfg.TwilioSMSFrom,
		}
	}
	return d
}

// withSender returns a shallow copy of the Dispatcher with the given TwilioSender
// injected.  Used in tests to swap in a mock without modifying production code.
func (d *Dispatcher) withSender(s TwilioSender) *Dispatcher {
	cp := *d
	cp.sender = s
	return &cp
}

// ── OCR handler ───────────────────────────────────────────────────────────────

// HandleOCR processes TaskOCRProcessImage:
// 1. Parses payload
// 2. Records PENDING ocr_result row (if not already exists)
// 3. Note: actual Vision API call is done synchronously in the HTTP handler
//    (services/ocr.go). The async task is retained for retry / audit purposes.
func (d *Dispatcher) HandleOCR(ctx context.Context, t *asynq.Task) error {
	var p OCRPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("ocr handler: unmarshal: %w", err)
	}

	log.Printf("[worker/ocr] processing photo %s for property %s", p.PhotoID, p.PropertyID)

	// Update ocr_results row to DONE status (the sync handler already set it).
	// If row doesn't exist yet (background task ran before sync path), we upsert.
	const q = `
		INSERT INTO ocr_results (photo_id, property_id, status)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (photo_id) DO NOTHING`
	if _, err := d.db.Exec(ctx, q, p.PhotoID, p.PropertyID, models.OCRStatusDone); err != nil {
		log.Printf("[worker/ocr] upsert failed: %v", err)
		// Non-fatal — don't fail the task.
	}

	log.Printf("[worker/ocr] done for photo %s", p.PhotoID)
	return nil
}

// ── Stale listing handler ─────────────────────────────────────────────────────

// HandleStale processes TaskNotifyStale:
// 1. Parses payload (property ID, owner contact, days unsold)
// 2. Inserts a notification_jobs row (PENDING)
// 3. Formats a reminder message
// 4. Enqueues WhatsApp task (primary) and SMS task (fallback)
func (d *Dispatcher) HandleStale(ctx context.Context, t *asynq.Task) error {
	var p StalePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("stale handler: unmarshal: %w", err)
	}

	log.Printf("[worker/stale] property %s unsold %d days, contact %s",
		p.PropertyID, p.DaysUnsold, maskContact(p.OwnerContact))

	// Insert notification_jobs record.
	var jobID string
	const insertQ = `
		INSERT INTO notification_jobs (property_id, job_type, status, scheduled_at)
		VALUES ($1::uuid, $2, $3, NOW())
		RETURNING id`
	if err := d.db.QueryRow(ctx, insertQ,
		p.PropertyID, models.JobTypeStaleListing, models.JobStatusPending,
	).Scan(&jobID); err != nil {
		return fmt.Errorf("stale handler: insert job: %w", err)
	}

	message := formatStaleMessage(p.DaysUnsold)

	if d.twilioWAFrom != "" && p.OwnerContact != "" {
		to := normaliseWhatsAppNumber(p.OwnerContact)
		if err := d.workerClient.EnqueueWhatsApp(ctx, to, message, jobID); err != nil {
			log.Printf("[worker/stale] enqueue WhatsApp failed: %v", err)
		}
	}

	if d.twilioSMSFrom != "" && p.OwnerContact != "" {
		to := normalisePhone(p.OwnerContact)
		if err := d.workerClient.EnqueueSMS(ctx, to, message, jobID); err != nil {
			log.Printf("[worker/stale] enqueue SMS failed: %v", err)
		}
	}

	return nil
}

// ── WhatsApp handler ──────────────────────────────────────────────────────────

// HandleWhatsApp processes TaskNotifyWhatsApp:
// Calls the Twilio WhatsApp Messages API and updates the notification_jobs row.
func (d *Dispatcher) HandleWhatsApp(ctx context.Context, t *asynq.Task) error {
	var p WhatsAppPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("whatsapp handler: unmarshal: %w", err)
	}

	if d.sender == nil {
		log.Printf("[worker/whatsapp] Twilio not configured — skipping send to %s", maskContact(p.To))
		return d.markJobFailed(ctx, p.NotificationJobID, "Twilio not configured")
	}

	sid, err := d.sender.SendWhatsApp(ctx, p.To, p.Message)
	if err != nil {
		_ = d.markJobFailed(ctx, p.NotificationJobID, err.Error())
		return fmt.Errorf("whatsapp handler: twilio send: %w", err)
	}

	log.Printf("[worker/whatsapp] sent to %s, SID=%s", maskContact(p.To), sid)
	return d.markJobSent(ctx, p.NotificationJobID, sid)
}

// ── SMS handler ───────────────────────────────────────────────────────────────

// HandleSMS processes TaskNotifySMS:
// Calls the Twilio SMS Messages API and updates the notification_jobs row.
func (d *Dispatcher) HandleSMS(ctx context.Context, t *asynq.Task) error {
	var p SMSPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("sms handler: unmarshal: %w", err)
	}

	if d.sender == nil {
		log.Printf("[worker/sms] Twilio not configured — skipping send to %s", maskContact(p.To))
		return d.markJobFailed(ctx, p.NotificationJobID, "Twilio not configured")
	}

	sid, err := d.sender.SendSMS(ctx, p.To, p.Message)
	if err != nil {
		_ = d.markJobFailed(ctx, p.NotificationJobID, err.Error())
		return fmt.Errorf("sms handler: twilio send: %w", err)
	}

	log.Printf("[worker/sms] sent to %s, SID=%s", maskContact(p.To), sid)
	return d.markJobSent(ctx, p.NotificationJobID, sid)
}

// ── Twilio helpers ────────────────────────────────────────────────────────────
// twilioSend is retained for any callers outside HandleWhatsApp/HandleSMS
// (e.g. direct ad-hoc sends from other handlers). New callers should prefer the
// injected d.sender interface.

// ── DB helpers ────────────────────────────────────────────────────────────────

func (d *Dispatcher) markJobSent(ctx context.Context, jobID, twilioSID string) error {
	now := time.Now()
	_, err := d.db.Exec(ctx,
		`UPDATE notification_jobs SET status=$1, twilio_sid=$2, sent_at=$3 WHERE id=$4::uuid`,
		models.JobStatusSent, twilioSID, now, jobID,
	)
	return err
}

func (d *Dispatcher) markJobFailed(ctx context.Context, jobID, reason string) error {
	_, err := d.db.Exec(ctx,
		`UPDATE notification_jobs SET status=$1 WHERE id=$2::uuid`,
		models.JobStatusFailed, jobID,
	)
	if err != nil {
		log.Printf("[worker] markJobFailed: %v", err)
	}
	return nil // don't surface DB errors as task errors
}

// ── Message formatting ────────────────────────────────────────────────────────

func formatStaleMessage(daysUnsold int) string {
	return fmt.Sprintf(
		"Hi! Your property listing has been active for %d days without an update. "+
			"Please contact us to update your listing details or confirm it is still available. "+
			"Listings are automatically marked inactive after 30 days of no activity.",
		daysUnsold,
	)
}

// ── Phone normalisation ───────────────────────────────────────────────────────

// normalisePhone strips non-digit characters and adds +91 if not already E.164.
func normalisePhone(contact string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, contact)

	if len(digits) == 10 {
		return "+91" + digits
	}
	if strings.HasPrefix(digits, "91") && len(digits) == 12 {
		return "+" + digits
	}
	if strings.HasPrefix(digits, "0") && len(digits) == 11 {
		return "+91" + digits[1:]
	}
	return "+" + digits
}

func normaliseWhatsAppNumber(contact string) string {
	return normalisePhone(contact)
}

// maskContact redacts all but the last 4 digits for safe logging.
func maskContact(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(s)-4) + s[len(s)-4:]
}

