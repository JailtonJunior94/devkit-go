package mssql

import (
	"context"
	"database/sql"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

// sqlResult wraps sql.Result to satisfy database.Result.
type sqlResult struct {
	res sql.Result
}

func (r sqlResult) RowsAffected() (int64, error) {
	return r.res.RowsAffected()
}

// sqlRows wraps *sql.Rows to satisfy database.Rows.
type sqlRows struct {
	rows *sql.Rows
}

func (r *sqlRows) Next() bool             { return r.rows.Next() }
func (r *sqlRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *sqlRows) Err() error             { return r.rows.Err() }
func (r *sqlRows) Close() error           { return r.rows.Close() }

// sqlRow wraps *sql.Row to satisfy database.Row.
type sqlRow struct {
	row *sql.Row
}

func (r *sqlRow) Scan(dest ...any) error { return r.row.Scan(dest...) }

// sqlDBDBTX wraps *sql.DB to satisfy database.DBTX.
// Used for operations executed directly on the connection pool (outside a transaction).
type sqlDBDBTX struct {
	db *sql.DB
}

func (d *sqlDBDBTX) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	res, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return sqlResult{res: res}, nil
}

func (d *sqlDBDBTX) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows: rows}, nil
}

func (d *sqlDBDBTX) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &sqlRow{row: d.db.QueryRowContext(ctx, query, args...)}
}

// Tx wraps *sql.Tx to satisfy database.DBTX and expose Commit/Rollback.
// The UoW uses Tx directly to manage transaction boundaries.
type Tx struct {
	tx *sql.Tx
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return sqlResult{res: res}, nil
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows: rows}, nil
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return &sqlRow{row: t.tx.QueryRowContext(ctx, query, args...)}
}

// Commit commits the transaction.
//
// Note: the ctx parameter is ignored because database/sql does not expose a
// CommitContext API. Cancelling ctx will not interrupt an in-flight commit;
// callers relying on shutdown deadlines must rely on the driver's own
// connection-level timeouts.
func (t *Tx) Commit(_ context.Context) error { return t.tx.Commit() }

// Rollback rolls back the transaction. It is safe to call after Commit.
//
// Note: the ctx parameter is ignored — see Commit for the rationale.
func (t *Tx) Rollback(_ context.Context) error { return t.tx.Rollback() }
