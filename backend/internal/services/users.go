// internal/services/users.go
package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/realestate/backend/internal/models"
)

// UserService handles user lookups.
type UserService struct {
	db *pgxpool.Pool
}

// NewUserService constructs a UserService.
func NewUserService(db *pgxpool.Pool) *UserService {
	return &UserService{db: db}
}

// ListUsers returns all active users, optionally filtered by role.
// Only non-sensitive fields are returned.
func (s *UserService) ListUsers(ctx context.Context, role *string) ([]models.PublicUser, error) {
	query := `SELECT id, email, full_name, role, is_active, created_at FROM users WHERE is_active = true`
	args := make([]interface{}, 0, 1)

	if role != nil {
		query += ` AND role = $1`
		args = append(args, *role)
	}
	query += ` ORDER BY full_name`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("users list: %w", err)
	}
	defer rows.Close()

	var users []models.PublicUser
	for rows.Next() {
		var u models.PublicUser
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("users list scan: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
