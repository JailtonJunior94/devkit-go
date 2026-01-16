package database

import (
	"context"
	"database/sql"
)

// DBTX é uma interface que abstrai operações de banco de dados.
// É implementada por *sql.DB e *sql.Tx, permitindo que repositories
// funcionem tanto com conexões diretas quanto com transações.
//
// Design Rationale:
//   - *sql.DB: Usado para operações sem transação
//   - *sql.Tx: Usado dentro de transações (via Unit of Work)
//   - Mesma interface permite repositories reutilizáveis
//
// Esta interface intencionalmente NÃO inclui BeginTx/Commit/Rollback
// porque essas operações devem ser gerenciadas externamente (via UnitOfWork).
//
// Exemplo sem transação:
//
//	type UserRepository struct {
//	    db database.DBTX
//	}
//
//	repo := NewUserRepository(dbManager.DB())
//	user, err := repo.FindByID(ctx, "123")
//
// Exemplo com transação:
//
//	uow, err := uow.NewUnitOfWork(dbManager.DB())
//	if err != nil {
//	    return err
//	}
//	err = uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
//	    repo := NewUserRepository(tx)  // Mesmo repository, agora transacional
//	    if err := repo.UpdateUser(ctx, user); err != nil {
//	        return err  // Rollback automático
//	    }
//	    return nil  // Commit automático
//	})
//
// Thread-Safety:
//   - *sql.DB é thread-safe e pode ser compartilhado
//   - *sql.Tx NÃO é thread-safe, deve ser usado em uma única goroutine
type DBTX interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
