// internal/middleware/rbac.go
package middleware

import (
	"net/http"

	"github.com/realestate/backend/internal/respond"
)

// Require returns an HTTP middleware that allows access only to requests
// whose authenticated role is in the provided roles list.
//
// Usage (with chi):
//
//	r.With(mw.Require(models.RoleSuperAdmin)).Get("/admin/users", handler)
func Require(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromCtx(r.Context())
			if role == "" {
				respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
				return
			}
			if _, ok := allowed[role]; !ok {
				respond.Error(w, http.StatusForbidden, "FORBIDDEN", "you do not have permission to access this resource")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
