// internal/models/user.go
package models

import (
	"time"

	"github.com/google/uuid"
)

// Role constants — keep in sync with the DB CHECK constraint.
const (
	RoleSuperAdmin = "SUPER_ADMIN"
	RoleBroker     = "BROKER"
	RoleClient     = "CLIENT"
)

// User maps to the users table.
type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash *string    `json:"-"`           // omitted from all API responses
	GoogleID     *string    `json:"-"`
	FullName     string     `json:"full_name"`
	Role         string     `json:"role"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PublicUser is the safe representation returned to clients — no hashes.
type PublicUser struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// ToPublic strips sensitive fields.
func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      u.Role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
	}
}
