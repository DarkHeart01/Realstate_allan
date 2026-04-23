// internal/db/db.go
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/realestate/backend/internal/config"
)

// New creates and validates a pgx connection pool using the provided config.
func New(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DBConnString())
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db: ping failed: %w", err)
	}

	return pool, nil
}
