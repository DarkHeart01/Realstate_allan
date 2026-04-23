// internal/models/refresh_token.go
package models

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken maps to the refresh_tokens table.
// TokenHash stores the SHA-256 hex digest of the raw token — never the raw token itself.
type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}
