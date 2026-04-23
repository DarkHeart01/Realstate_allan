// internal/respond/respond.go
package respond

import (
	"encoding/json"
	"net/http"
)

// ── Envelope types ────────────────────────────────────────────────────────────

type successEnvelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

type paginatedEnvelope struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data"`
	Message    string      `json:"message"`
	Pagination Pagination  `json:"pagination"`
}

type errorEnvelope struct {
	Success   bool   `json:"success"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// Pagination holds paging metadata returned in list responses.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
}

// ── Public helpers ────────────────────────────────────────────────────────────

// JSON writes a success envelope with the given HTTP status and data payload.
func JSON(w http.ResponseWriter, status int, data interface{}, message string) {
	write(w, status, successEnvelope{
		Success: true,
		Data:    data,
		Message: message,
	})
}

// Paginated writes a success envelope with pagination metadata.
func Paginated(w http.ResponseWriter, data interface{}, message string, limit, offset, count int) {
	write(w, http.StatusOK, paginatedEnvelope{
		Success: true,
		Data:    data,
		Message: message,
		Pagination: Pagination{
			Limit:  limit,
			Offset: offset,
			Count:  count,
		},
	})
}

// Error writes an error envelope with a snake_case error code.
func Error(w http.ResponseWriter, status int, code, message string) {
	write(w, status, errorEnvelope{
		Success:   false,
		ErrorCode: code,
		Message:   message,
	})
}

// ── internal ─────────────────────────────────────────────────────────────────

func write(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
