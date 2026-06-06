package pgxshared

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

type Result struct {
	tag pgconn.CommandTag
}

func (r Result) RowsAffected() (int64, error) {
	return r.tag.RowsAffected(), nil
}

type Rows struct {
	rows pgx.Rows
}

func (r *Rows) Next() bool             { return r.rows.Next() }
func (r *Rows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *Rows) Err() error             { return r.rows.Err() }
func (r *Rows) Close() error           { r.rows.Close(); return nil }

type Row struct {
	row pgx.Row
}

func (r *Row) Scan(dest ...any) error { return r.row.Scan(dest...) }

type PoolDBTX struct {
	pool *pgxpool.Pool
}

func (p *PoolDBTX) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	tag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return Result{tag: tag}, nil
}

func (p *PoolDBTX) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (p *PoolDBTX) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &Row{row: p.pool.QueryRow(ctx, query, args...)}
}

type Tx struct {
	tx pgx.Tx
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	tag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return Result{tag: tag}, nil
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &Row{row: t.tx.QueryRow(ctx, query, args...)}
}

func (t *Tx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *Tx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
