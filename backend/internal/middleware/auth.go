// internal/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
)

// contextKey is a private type to avoid collisions in context values.
type contextKey string

const (
	ContextKeyUserID contextKey = "userID"
	ContextKeyEmail  contextKey = "email"
	ContextKeyRole   contextKey = "role"
	ContextKeyJTI    contextKey = "jti"
)

// Authenticator holds dependencies needed to validate JWTs.
type Authenticator struct {
	tokens *services.TokenService
}

// NewAuthenticator constructs an Authenticator middleware.
func NewAuthenticator(tokens *services.TokenService) *Authenticator {
	return &Authenticator{tokens: tokens}
}

// Authenticate is an HTTP middleware that:
//  1. Extracts the Bearer token from the Authorization header.
//  2. Validates the JWT signature and expiry.
//  3. Checks the JTI against the Redis blacklist.
//  4. Injects userID, email, role, and jti into the request context.
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractBearerToken(r)
		if tokenString == "" {
			respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or malformed Authorization header")
			return
		}

		claims, err := a.tokens.ParseAccessToken(tokenString)
		if err != nil {
			respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
			return
		}

		// Check Redis blacklist
		blacklisted, err := a.tokens.IsBlacklisted(r.Context(), claims.ID)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not validate token")
			return
		}
		if blacklisted {
			respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "token has been revoked")
			return
		}

		// Inject claims into context
		ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
		ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
		ctx = context.WithValue(ctx, ContextKeyJTI, claims.ID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Context accessors ─────────────────────────────────────────────────────────

// UserIDFromCtx extracts the authenticated user's UUID string from context.
func UserIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyUserID).(string)
	return v
}

// EmailFromCtx extracts the authenticated user's email from context.
func EmailFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyEmail).(string)
	return v
}

// RoleFromCtx extracts the authenticated user's role from context.
func RoleFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyRole).(string)
	return v
}

// JTIFromCtx extracts the JWT ID (JTI) from context.
func JTIFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyJTI).(string)
	return v
}

// ── helper ────────────────────────────────────────────────────────────────────

func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
