// internal/services/auth.go
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/realestate/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors returned by AuthService methods.
var (
	ErrEmailTaken          = errors.New("auth: email already registered")
	ErrInvalidCredentials  = errors.New("auth: invalid email or password")
	ErrUserNotFound        = errors.New("auth: user not found")
	ErrInactiveUser        = errors.New("auth: account is inactive")
	ErrTokenInvalid        = errors.New("auth: refresh token invalid or expired")
)

const bcryptCost = 12

// TokenPair is the result of a successful auth operation.
type TokenPair struct {
	AccessToken  string
	AccessJTI    string
	RefreshToken string // raw token — set this in the HttpOnly cookie
	ExpiresAt    time.Time
}

// AuthService orchestrates user registration, login, token refresh, and logout.
type AuthService struct {
	db     *pgxpool.Pool
	tokens *TokenService
}

// NewAuthService constructs an AuthService.
func NewAuthService(db *pgxpool.Pool, tokens *TokenService) *AuthService {
	return &AuthService{db: db, tokens: tokens}
}

// Register creates a new user with role BROKER and returns a token pair.
func (s *AuthService) Register(ctx context.Context, email, password, fullName string) (*models.User, *TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: hash password: %w", err)
	}

	var user models.User
	err = s.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, full_name, role, is_active, created_at, updated_at
	`, email, string(hash), fullName, models.RoleBroker).Scan(
		&user.ID, &user.Email, &user.FullName, &user.Role,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, ErrEmailTaken
		}
		return nil, nil, fmt.Errorf("auth: insert user: %w", err)
	}

	pair, err := s.issueTokenPair(ctx, &user)
	if err != nil {
		return nil, nil, err
	}
	return &user, pair, nil
}

// Login verifies credentials and returns a token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, *TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var user models.User
	var passwordHash string
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, full_name, role, is_active, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&user.ID, &user.Email, &passwordHash, &user.FullName, &user.Role,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("auth: query user: %w", err)
	}

	if !user.IsActive {
		return nil, nil, ErrInactiveUser
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := s.issueTokenPair(ctx, &user)
	if err != nil {
		return nil, nil, err
	}
	return &user, pair, nil
}

// RefreshTokens validates the raw refresh token from the cookie, rotates it,
// and returns a new token pair.
func (s *AuthService) RefreshTokens(ctx context.Context, rawToken string) (*models.User, *TokenPair, error) {
	hash := hashToken(rawToken)

	var rt models.RefreshToken
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked
		FROM refresh_tokens
		WHERE token_hash = $1
	`, hash).Scan(&rt.ID, &userID, &rt.ExpiresAt, &rt.Revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrTokenInvalid
		}
		return nil, nil, fmt.Errorf("auth: lookup refresh token: %w", err)
	}

	if rt.Revoked || time.Now().After(rt.ExpiresAt) {
		return nil, nil, ErrTokenInvalid
	}

	// Revoke the old token before issuing a new one (rotation)
	if _, err := s.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE id = $1`, rt.ID,
	); err != nil {
		return nil, nil, fmt.Errorf("auth: revoke old refresh token: %w", err)
	}

	// Load the user
	var user models.User
	err = s.db.QueryRow(ctx, `
		SELECT id, email, full_name, role, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.FullName, &user.Role,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: load user for refresh: %w", err)
	}

	if !user.IsActive {
		return nil, nil, ErrInactiveUser
	}

	pair, err := s.issueTokenPair(ctx, &user)
	if err != nil {
		return nil, nil, err
	}
	return &user, pair, nil
}

// Logout revokes the refresh token in the DB and blacklists the access JTI in Redis.
func (s *AuthService) Logout(ctx context.Context, jti, rawRefreshToken string, accessTTL time.Duration) error {
	// Blacklist the access token JTI
	if err := s.tokens.BlacklistToken(ctx, jti, accessTTL); err != nil {
		return err
	}

	// Revoke the refresh token (best-effort — cookie may already be cleared)
	if rawRefreshToken != "" {
		hash := hashToken(rawRefreshToken)
		if _, err := s.db.Exec(ctx,
			`UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`, hash,
		); err != nil {
			return fmt.Errorf("auth: revoke refresh token on logout: %w", err)
		}
	}
	return nil
}

// UpsertGoogleUser creates or updates a user who authenticated via Google OAuth.
// Matches on google_id first, then email.
func (s *AuthService) UpsertGoogleUser(ctx context.Context, googleID, email, fullName string) (*models.User, *TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var user models.User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (email, google_id, full_name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (google_id) DO UPDATE
			SET email     = EXCLUDED.email,
			    full_name = EXCLUDED.full_name,
			    updated_at = NOW()
		RETURNING id, email, full_name, role, is_active, created_at, updated_at
	`, email, googleID, fullName, models.RoleBroker).Scan(
		&user.ID, &user.Email, &user.FullName, &user.Role,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		// google_id unique conflict handled above; email conflict means existing user — link account
		if isUniqueViolation(err) {
			err2 := s.db.QueryRow(ctx, `
				UPDATE users SET google_id = $1, updated_at = NOW()
				WHERE email = $2
				RETURNING id, email, full_name, role, is_active, created_at, updated_at
			`, googleID, email).Scan(
				&user.ID, &user.Email, &user.FullName, &user.Role,
				&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
			)
			if err2 != nil {
				return nil, nil, fmt.Errorf("auth: link google account: %w", err2)
			}
		} else {
			return nil, nil, fmt.Errorf("auth: upsert google user: %w", err)
		}
	}

	if !user.IsActive {
		return nil, nil, ErrInactiveUser
	}

	pair, err := s.issueTokenPair(ctx, &user)
	if err != nil {
		return nil, nil, err
	}
	return &user, pair, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// issueTokenPair mints an access token + refresh token and persists the refresh hash.
func (s *AuthService) issueTokenPair(ctx context.Context, user *models.User) (*TokenPair, error) {
	accessToken, jti, err := s.tokens.IssueAccessToken(user)
	if err != nil {
		return nil, err
	}

	rawRefresh, hashRefresh, err := s.tokens.IssueRefreshToken()
	if err != nil {
		return nil, err
	}

	ttl := s.tokens.RefreshTTL()
	expiresAt := time.Now().Add(ttl)

	_, err = s.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, user.ID, hashRefresh, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("auth: persist refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		AccessJTI:    jti,
		RefreshToken: rawRefresh,
		ExpiresAt:    expiresAt,
	}, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unique")
}
