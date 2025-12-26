package uow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

var (
	ErrTransactionAlreadyFinished = errors.New("transaction has already been committed or rolled back")
)

// UnitOfWork fornece uma abstração para gerenciar transações de banco de dados.
// Garante que todas as operações sejam executadas dentro de uma transação atômica.
type UnitOfWork interface {
	// DBTX retorna a conexão de banco de dados subjacente.
	// AVISO: Use este método com extrema cautela. Operações executadas diretamente
	// através desta conexão NÃO serão parte de nenhuma transação gerenciada pelo
	// Unit of Work. Prefira sempre usar o método Do() para garantir atomicidade.
	// Este método existe apenas para casos específicos onde você precisa executar
	// operações fora do contexto transacional e entende completamente as implicações.
	DBTX() database.DBTX

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
// Exemplo: WithIsolationLevel(sql.LevelSerializable)
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
// Exemplo:
//
//	uow := NewUnitOfWork(db, WithIsolationLevel(sql.LevelSerializable))
func NewUnitOfWork(db *sql.DB, opts ...UnitOfWorkOption) UnitOfWork {
	u := &unitOfWork{
		db:      db,
		options: nil,
	}

	for _, opt := range opts {
		opt(u)
	}

	return u
}

func (u *unitOfWork) DBTX() database.DBTX {
	return u.db
}

func (u *unitOfWork) Do(ctx context.Context, fn func(ctx context.Context, db database.DBTX) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before transaction start: %w", err)
	}

	tx, err := u.db.BeginTx(ctx, u.options)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	var finished bool

	defer func() {
		if p := recover(); p != nil {
			if !finished {
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
		finished = true
		if rbErr := rollbackTx(tx); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	// Verificar se o context foi cancelado durante a execução
	if err = ctx.Err(); err != nil {
		finished = true
		if rbErr := rollbackTx(tx); rbErr != nil {
			return fmt.Errorf("context cancelled during transaction: %w, rollback error: %v", err, rbErr)
		}
		return fmt.Errorf("context cancelled during transaction: %w", err)
	}

	finished = true
	if err = tx.Commit(); err != nil {
		if rbErr := rollbackTx(tx); rbErr != nil {
			if !errors.Is(rbErr, ErrTransactionAlreadyFinished) {
				return fmt.Errorf("commit error: %w, rollback error: %v", err, rbErr)
			}
		}
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func rollbackTx(tx *sql.Tx) error {
	if tx == nil {
		return ErrTransactionAlreadyFinished
	}

	if err := tx.Rollback(); err != nil {
		if errors.Is(err, sql.ErrTxDone) {
			return ErrTransactionAlreadyFinished
		}
		return err
	}

	return nil
}
