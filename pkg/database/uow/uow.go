package uow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

var (
	errTransactionAlreadyFinished = errors.New("transaction has already been committed or rolled back")
)

// UnitOfWork fornece uma abstração para gerenciar transações de banco de dados.
// Garante que todas as operações sejam executadas dentro de uma transação atômica.
//
// Thread-Safety:
// Múltiplas goroutines podem chamar Do() simultaneamente na mesma instância de UnitOfWork.
// Cada chamada cria uma transação independente e isolada. O UnitOfWork não mantém
// estado transacional entre chamadas.
//
// Context Handling:
// O context é verificado antes de iniciar a transação e após a execução da função callback.
// IMPORTANTE: O método Commit() do database/sql não aceita context, portanto operações de
// commit lentas NÃO podem ser canceladas via context. Considere usar timeouts no contexto
// superior para prevenir bloqueios indefinidos.
type UnitOfWork interface {
	// Do executa a função fornecida dentro de uma transação.
	// Se a função retornar erro, a transação é revertida (rollback).
	// Se a função tiver sucesso, a transação é confirmada (commit).
	// Em caso de panic, a transação é revertida e o panic é re-lançado.
	// O context é verificado antes de iniciar a transação e após a execução da função.
	Do(ctx context.Context, fn func(ctx context.Context, db database.DBTX) error) error
}

type unitOfWork struct {
	db      *sql.DB
	options *sql.TxOptions
}

// UnitOfWorkOption é uma função que configura opções do Unit of Work.
type UnitOfWorkOption func(*unitOfWork)

// WithIsolationLevel configura o nível de isolamento da transação.
// Exemplo: WithIsolationLevel(sql.LevelSerializable).
func WithIsolationLevel(level sql.IsolationLevel) UnitOfWorkOption {
	return func(u *unitOfWork) {
		if u.options == nil {
			u.options = &sql.TxOptions{}
		}
		u.options.Isolation = level
	}
}

// WithReadOnly configura a transação como somente leitura.
// Transações read-only podem oferecer melhor performance e prevenir modificações acidentais.
func WithReadOnly(readOnly bool) UnitOfWorkOption {
	return func(u *unitOfWork) {
		if u.options == nil {
			u.options = &sql.TxOptions{}
		}
		u.options.ReadOnly = readOnly
	}
}

// NewUnitOfWork cria uma nova instância de Unit of Work.
// O parâmetro db deve ser uma conexão válida de banco de dados.
// Options podem ser fornecidas para configurar o comportamento da transação.
//
// Panic:
// Esta função entra em panic se db for nil. Isso indica um erro de programação
// e deve ser corrigido no código do chamador.
//
// Exemplo:
//
//	uow := NewUnitOfWork(db, WithIsolationLevel(sql.LevelSerializable))
func NewUnitOfWork(db *sql.DB, opts ...UnitOfWorkOption) UnitOfWork {
	if db == nil {
		panic("database connection cannot be nil")
	}

	u := &unitOfWork{
		db:      db,
		options: nil,
	}

	for _, opt := range opts {
		opt(u)
	}

	return u
}

func (u *unitOfWork) Do(ctx context.Context, fn func(ctx context.Context, db database.DBTX) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before transaction start: %w", err)
	}

	tx, err := u.db.BeginTx(ctx, u.options)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	var finished atomic.Bool

	defer func() {
		if p := recover(); p != nil {
			if !finished.Load() {
				if rbErr := tx.Rollback(); rbErr != nil {
					// Não podemos retornar o erro, mas podemos incluí-lo no panic
					// para não mascarar problemas de rollback
					if !errors.Is(rbErr, sql.ErrTxDone) {
						panic(fmt.Sprintf("panic during transaction with rollback failure: panic=%v, rollback_error=%v", p, rbErr))
					}
				}
			}
			panic(p)
		}
	}()

	if err = fn(ctx, tx); err != nil {
		finished.Store(true)
		if rbErr := rollbackTx(tx); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	// Verificar se o context foi cancelado durante a execução
	if err = ctx.Err(); err != nil {
		finished.Store(true)
		if rbErr := rollbackTx(tx); rbErr != nil {
			return fmt.Errorf("context cancelled during transaction: %w, rollback error: %v", err, rbErr)
		}
		return fmt.Errorf("context cancelled during transaction: %w", err)
	}

	finished.Store(true)
	if err = tx.Commit(); err != nil {
		// Quando commit falha, a maioria dos drivers já faz rollback automático.
		// Não tentamos rollback aqui para evitar mascarar o erro real.
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func rollbackTx(tx *sql.Tx) error {
	if tx == nil {
		// Se tx for nil aqui, indica um bug no código
		panic("rollbackTx called with nil transaction - this is a bug")
	}

	if err := tx.Rollback(); err != nil {
		if errors.Is(err, sql.ErrTxDone) {
			return errTransactionAlreadyFinished
		}
		return err
	}

	return nil
}
