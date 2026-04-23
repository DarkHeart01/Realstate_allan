// internal/handlers/tags.go
package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/realestate/backend/internal/respond"
)

// TagsHandler handles tag autocomplete.
type TagsHandler struct {
	db *pgxpool.Pool
}

// NewTagsHandler constructs a TagsHandler.
func NewTagsHandler(db *pgxpool.Pool) *TagsHandler {
	return &TagsHandler{db: db}
}

// GET /api/tags?q=<prefix>
func (h *TagsHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "q parameter is required")
		return
	}

	const query = `
		SELECT DISTINCT tag
		FROM properties, unnest(tags) AS tag
		WHERE deleted_at IS NULL
		  AND tag ILIKE $1
		ORDER BY tag
		LIMIT 10`

	rows, err := h.db.Query(r.Context(), query, fmt.Sprintf("%s%%", q))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to query tags")
		return
	}
	defer rows.Close()

	tags := make([]string, 0, 10)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to scan tags")
			return
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read tags")
		return
	}

	respond.JSON(w, http.StatusOK, tags, "")
}
