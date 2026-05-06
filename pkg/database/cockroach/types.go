package cockroach

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgxResult wraps pgconn.CommandTag to satisfy database.Result.
type pgxResult struct {
	tag pgconn.CommandTag
}

func (r pgxResult) RowsAffected() (int64, error) {
	return r.tag.RowsAffected(), nil
}

// pgxRows wraps pgx.Rows to satisfy database.Rows.
type pgxRows struct {
	rows pgx.Rows
}

func (r *pgxRows) Next() bool             { return r.rows.Next() }
func (r *pgxRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *pgxRows) Err() error             { return r.rows.Err() }
func (r *pgxRows) Close() error           { r.rows.Close(); return nil }

// pgxRow wraps pgx.Row to satisfy database.Row.
type pgxRow struct {
	row pgx.Row
}

func (r *pgxRow) Scan(dest ...any) error { return r.row.Scan(dest...) }

// pgxPoolDBTX wraps *pgxpool.Pool to satisfy database.DBTX.
type pgxPoolDBTX struct {
	pool *pgxpool.Pool
}

func (p *pgxPoolDBTX) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	tag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgxResult{tag: tag}, nil
}

func (p *pgxPoolDBTX) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &pgxRows{rows: rows}, nil
}

func (p *pgxPoolDBTX) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &pgxRow{row: p.pool.QueryRow(ctx, query, args...)}
}

// Tx wraps pgx.Tx to satisfy database.DBTX and expose Commit/Rollback.
type Tx struct {
	tx pgx.Tx
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	tag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgxResult{tag: tag}, nil
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &pgxRows{rows: rows}, nil
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &pgxRow{row: t.tx.QueryRow(ctx, query, args...)}
}

// Commit commits the transaction.
func (t *Tx) Commit(ctx context.Context) error { return t.tx.Commit(ctx) }

// Rollback rolls back the transaction. It is safe to call after Commit.
func (t *Tx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
