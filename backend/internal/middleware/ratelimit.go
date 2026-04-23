// internal/middleware/ratelimit.go
// Redis sliding-window rate limiter.
//
// Algorithm:
//   ZADD key <now_ns> <now_ns>
//   ZREMRANGEBYSCORE key 0 <window_start_ns>   — prune old entries
//   ZCARD key                                   — count requests in window
//   EXPIRE key <window_seconds>
//
// All four commands run inside a Redis MULTI/EXEC pipeline for atomicity.
//
// Key format: "ratelimit:<route_group>:<client_ip>"
package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/realestate/backend/internal/respond"
)

// RateLimit returns a middleware that limits requests to limit per window for
// each client IP.  routeGroup is a short label used in the Redis key
// (e.g. "auth_login", "global").
func RateLimit(rdb *redis.Client, routeGroup string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			key := fmt.Sprintf("ratelimit:%s:%s", routeGroup, ip)

			exceeded, err := checkLimit(r.Context(), rdb, key, limit, window)
			if err != nil {
				// On Redis failure, allow the request (fail-open).
				log.Printf("[ratelimit] redis error for key %s: %v (allowing request)", key, err)
				next.ServeHTTP(w, r)
				return
			}

			if exceeded {
				retryAfter := int(window.Seconds())
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				respond.Error(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
					"Too many requests. Try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkLimit records a hit for key and returns true if the limit is exceeded.
func checkLimit(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()

	pipe := rdb.Pipeline()

	// Add the current request timestamp as both score and member (unique per ns).
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: strconv.FormatInt(now, 10)})

	// Remove entries outside the window.
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))

	// Count remaining entries.
	cardCmd := pipe.ZCard(ctx, key)

	// Auto-expire the key after the window to avoid unbounded growth.
	pipe.Expire(ctx, key, window+time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("ratelimit: pipeline exec: %w", err)
	}

	count := cardCmd.Val()
	return count > int64(limit), nil
}

// realIP returns the best-effort real IP from the request, respecting the
// X-Real-IP header set by Nginx upstream.
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// Take the first IP (closest to client).
		for i, c := range ip {
			if c == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	// RemoteAddr is "ip:port".
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
