// internal/redis/redis.go
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/realestate/backend/internal/config"
)

// New creates a go-redis client from REDIS_URL and verifies connectivity with PING.
func New(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse URL: %w", err)
	}

	client := redis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	return client, nil
}
