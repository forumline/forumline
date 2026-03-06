package shared

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewDBPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	// Use simple protocol to avoid prepared statement conflicts with
	// Supabase's PgBouncer in transaction pooling mode.
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Keep minimum connections alive to avoid cold-start latency
	config.MinConns = 2
	// Allow enough connections for SSE LISTEN goroutines + API queries.
	// Each LISTEN channel holds a connection for its lifetime, so we need
	// at least N_channels + headroom for concurrent API requests.
	config.MaxConns = 10
	// Recycle connections before Fly's proxy can kill them
	config.MaxConnLifetime = 10 * time.Minute
	config.MaxConnIdleTime = 2 * time.Minute
	// Frequent health checks to detect dead connections quickly
	config.HealthCheckPeriod = 15 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
