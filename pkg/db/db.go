// Package db manages the PostgreSQL connection pool.
// It exposes a global *pgxpool.Pool used by the repository layer.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the application-wide database connection pool.
var Pool *pgxpool.Pool

// Connect initialises the connection pool using the provided database URL.
// The URL format follows the standard postgres:// DSN and supports Railway-style
// SSL configuration via query parameters (e.g. ?sslmode=require).
func Connect(ctx context.Context, dbURL string) error {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return fmt.Errorf("db: failed to parse config: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("db: failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("db: ping failed: %w", err)
	}

	Pool = pool
	return nil
}

// Close shuts down the connection pool gracefully.
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}
