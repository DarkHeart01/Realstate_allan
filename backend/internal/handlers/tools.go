// internal/handlers/tools.go
package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
)

// ToolsHandler handles the brokerage calculator and CSV export endpoints.
type ToolsHandler struct {
	calc *services.Calculator
	db   *pgxpool.Pool
}

// NewToolsHandler constructs a ToolsHandler.
func NewToolsHandler(calc *services.Calculator, db *pgxpool.Pool) *ToolsHandler {
	return &ToolsHandler{calc: calc, db: db}
}

// ── POST /api/tools/calculator ────────────────────────────────────────────────

func (h *ToolsHandler) Calculator(w http.ResponseWriter, r *http.Request) {
	var in services.CalculatorInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid request body")
		return
	}

	result, err := h.calc.Calculate(in)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, result, "Brokerage calculated successfully")
}

// ── GET /api/tools/export/csv ─────────────────────────────────────────────────
// SUPER_ADMIN only (enforced by mw.Require at the router level).
// Streams rows as they are fetched — uses http.Flusher to avoid buffering the
// entire result set in memory.

func (h *ToolsHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	filename := fmt.Sprintf("properties-%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)

	flusher, canFlush := w.(http.Flusher)

	cw := csv.NewWriter(w)

	// Header row.
	_ = cw.Write([]string{
		"id", "listing_category", "property_type",
		"owner_name", "owner_contact",
		"price", "plot_area", "built_up_area",
		"location_lat", "location_lng",
		"is_direct_owner", "tags",
		"description", "created_at",
	})
	cw.Flush()
	if canFlush {
		flusher.Flush()
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, listing_category, property_type,
		       owner_name, owner_contact,
		       price, plot_area, built_up_area,
		       location_lat, location_lng,
		       is_direct_owner, tags,
		       description, created_at
		FROM properties
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC`)
	if err != nil {
		// Headers already written — we can't change the status code. Write a
		// sentinel row so the caller knows something went wrong.
		_ = cw.Write([]string{"ERROR", "failed to query properties", err.Error()})
		cw.Flush()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, category, propType  string
			ownerName, ownerContact string
			price                   float64
			plotArea, builtUpArea   *float64
			lat, lng                float64
			isDirectOwner           bool
			tags                    []string
			description             *string
			createdAt               time.Time
		)

		if err := rows.Scan(
			&id, &category, &propType,
			&ownerName, &ownerContact,
			&price, &plotArea, &builtUpArea,
			&lat, &lng,
			&isDirectOwner, &tags,
			&description, &createdAt,
		); err != nil {
			continue
		}

		record := []string{
			id, category, propType,
			ownerName, ownerContact,
			fmt.Sprintf("%.2f", price),
			nullableFloat(plotArea),
			nullableFloat(builtUpArea),
			fmt.Sprintf("%.6f", lat),
			fmt.Sprintf("%.6f", lng),
			boolStr(isDirectOwner),
			joinTags(tags),
			derefStr(description),
			createdAt.Format(time.RFC3339),
		}

		_ = cw.Write(record)
		cw.Flush()
		if canFlush {
			flusher.Flush()
		}
	}

	if err := rows.Err(); err != nil {
		_ = cw.Write([]string{"ERROR", "stream interrupted", err.Error()})
		cw.Flush()
	}
}

// ── CSV helpers ───────────────────────────────────────────────────────────────

func nullableFloat(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *f)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func joinTags(tags []string) string {
	result := ""
	for i, t := range tags {
		if i > 0 {
			result += "|"
		}
		result += t
	}
	return result
}
