package sqlshared

import (
	"context"
	"database/sql"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

type Result struct {
	res sql.Result
}

func (r Result) RowsAffected() (int64, error) { return r.res.RowsAffected() }

type Rows struct {
	rows *sql.Rows
}

func (r *Rows) Next() bool             { return r.rows.Next() }
func (r *Rows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *Rows) Err() error             { return r.rows.Err() }
func (r *Rows) Close() error           { return r.rows.Close() }

type Row struct {
	row *sql.Row
}

func (r *Row) Scan(dest ...any) error { return r.row.Scan(dest...) }

type PoolDBTX struct {
	db *sql.DB
}

func (d *PoolDBTX) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	res, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return Result{res: res}, nil
}

func (d *PoolDBTX) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (d *PoolDBTX) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &Row{row: d.db.QueryRowContext(ctx, query, args...)}
}

type Tx struct {
	tx *sql.Tx
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return Result{res: res}, nil
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &Row{row: t.tx.QueryRowContext(ctx, query, args...)}
}

func (t *Tx) Commit(_ context.Context) error   { return t.tx.Commit() }
func (t *Tx) Rollback(_ context.Context) error { return t.tx.Rollback() }
