// internal/handlers/users.go
package handlers

import (
	"net/http"

	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
)

// UsersHandler handles the user list endpoint.
type UsersHandler struct {
	svc *services.UserService
}

// NewUsersHandler constructs a UsersHandler.
func NewUsersHandler(svc *services.UserService) *UsersHandler {
	return &UsersHandler{svc: svc}
}

// GET /api/users?role=BROKER
// SUPER_ADMIN only — enforced at the router level via mw.Require.
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	var role *string
	if v := r.URL.Query().Get("role"); v != "" {
		role = &v
	}

	users, err := h.svc.ListUsers(r.Context(), role)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list users")
		return
	}

	respond.JSON(w, http.StatusOK, users, "Users retrieved successfully")
}
