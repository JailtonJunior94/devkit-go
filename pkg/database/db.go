package database

import (
	"context"
)

// DBTX é o contrato de execução genérico.
// Implementado tanto por uma conexão do pool quanto por uma transação ativa.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) Row
}

// Tx estende DBTX com limites transacionais: Commit e Rollback.
// Implementado por wrappers de transação específicos de cada driver.
type Tx interface {
	DBTX
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// TxOptions carrega configurações de isolamento e modo de acesso para BeginTx.
// Um TxOptions com valor zero delega todas as configurações ao padrão do driver (RF-11).
type TxOptions struct {
	Isolation IsolationLevel
	ReadOnly  bool
}

// Result representa o resultado de uma operação de escrita.
// LastInsertId é omitido intencionalmente; use RETURNING para recuperação de ID.
type Result interface {
	RowsAffected() (int64, error)
}

// Rows é um iterador sobre um conjunto de resultados de consulta.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

// Row representa o resultado de uma única linha de consulta.
type Row interface {
	Scan(dest ...any) error
}
