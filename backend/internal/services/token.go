// internal/services/token.go
package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/realestate/backend/internal/config"
	"github.com/realestate/backend/internal/models"
)

// TokenClaims are the JWT payload fields.
type TokenClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// TokenService handles JWT creation/validation and refresh-token lifecycle.
type TokenService struct {
	cfg   *config.Config
	redis *redis.Client
}

// NewTokenService constructs a TokenService.
func NewTokenService(cfg *config.Config, rdb *redis.Client) *TokenService {
	return &TokenService{cfg: cfg, redis: rdb}
}

// IssueAccessToken creates a signed HS256 JWT for the given user.
// Returns the raw token string and the JTI (used for blacklisting on logout).
func (s *TokenService) IssueAccessToken(user *models.User) (tokenString, jti string, err error) {
	jti = uuid.New().String()
	now := time.Now()
	ttl := time.Duration(s.cfg.JWTAccessTTLMinutes) * time.Minute

	claims := TokenClaims{
		UserID: user.ID.String(),
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err = token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", "", fmt.Errorf("token: sign access token: %w", err)
	}
	return tokenString, jti, nil
}

// ParseAccessToken validates signature + expiry and returns the claims.
// It does NOT check the Redis blacklist — that is the middleware's responsibility.
func (s *TokenService) ParseAccessToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("token: unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("token: parse: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("token: invalid claims")
	}
	return claims, nil
}

// IssueRefreshToken generates a cryptographically random 32-byte token.
// Returns (rawToken, sha256HexHash).
// Store the hash in the DB; send the raw token to the client via HttpOnly cookie.
func (s *TokenService) IssueRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("token: generate refresh token: %w", err)
	}
	raw = hex.EncodeToString(b)
	hash = hashToken(raw)
	return raw, hash, nil
}

// BlacklistToken stores the JTI in Redis with an expiry matching the token's remaining TTL.
// After this, the auth middleware will reject any request carrying this token.
func (s *TokenService) BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error {
	if err := s.redis.Set(ctx, blacklistKey(jti), "1", ttl).Err(); err != nil {
		return fmt.Errorf("token: blacklist jti %q: %w", jti, err)
	}
	return nil
}

// IsBlacklisted returns true if the JTI is present in the Redis blacklist.
func (s *TokenService) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	n, err := s.redis.Exists(ctx, blacklistKey(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("token: check blacklist jti %q: %w", jti, err)
	}
	return n > 0, nil
}

// AccessTTL returns the configured access token duration.
func (s *TokenService) AccessTTL() time.Duration {
	return time.Duration(s.cfg.JWTAccessTTLMinutes) * time.Minute
}

// RefreshTTL returns the configured refresh token duration.
func (s *TokenService) RefreshTTL() time.Duration {
	return time.Duration(s.cfg.JWTRefreshTTLDays) * 24 * time.Hour
}

// ── helpers ───────────────────────────────────────────────────────────────────

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func blacklistKey(jti string) string {
	return "blacklist:jti:" + jti
}
