package shared

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ObservablePool wraps pgxpool.Pool and logs all query errors automatically.
type ObservablePool struct {
	*pgxpool.Pool
}

func NewObservablePool(pool *pgxpool.Pool) *ObservablePool {
	return &ObservablePool{Pool: pool}
}

func (o *ObservablePool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := o.Pool.Exec(ctx, sql, args...)
	if err != nil {
		slog.ErrorContext(ctx, "db.Exec failed", "sql", truncate(sql, 100), "err", err)
	}
	return tag, err
}

func (o *ObservablePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &observableRow{row: o.Pool.QueryRow(ctx, sql, args...), sql: sql}
}

type observableRow struct {
	row pgx.Row
	sql string
}

func (r *observableRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if err != nil && err != pgx.ErrNoRows {
		slog.Error("db.QueryRow.Scan failed", "sql", truncate(r.sql, 100), "err", err)
	}
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
