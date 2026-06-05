package postgres

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgxResult envolve o pgconn.CommandTag para satisfazer o database.Result.
type pgxResult struct {
	tag pgconn.CommandTag
}

func (r pgxResult) RowsAffected() (int64, error) {
	return r.tag.RowsAffected(), nil
}

// pgxRows envolve o pgx.Rows para satisfazer o database.Rows.
type pgxRows struct {
	rows pgx.Rows
}

func (r *pgxRows) Next() bool             { return r.rows.Next() }
func (r *pgxRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *pgxRows) Err() error             { return r.rows.Err() }
func (r *pgxRows) Close() error           { r.rows.Close(); return nil }

// pgxRow envolve o pgx.Row para satisfazer o database.Row.
type pgxRow struct {
	row pgx.Row
}

func (r *pgxRow) Scan(dest ...any) error { return r.row.Scan(dest...) }

// pgxPoolDBTX envolve o *pgxpool.Pool para satisfazer o database.DBTX.
// Usado para operações executadas diretamente no pool de conexões (fora de uma transação).
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

// Tx envolve o pgx.Tx para satisfazer o database.DBTX e expor Commit/Rollback.
// O UoW (task 4.0) usa o Tx diretamente para gerenciar os limites da transação.
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

// Commit confirma a transação.
func (t *Tx) Commit(ctx context.Context) error { return t.tx.Commit(ctx) }

// Rollback reverte a transação. É seguro chamar após o Commit.
func (t *Tx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
