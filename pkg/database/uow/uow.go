package uow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jailtonjunior94/financial/pkg/database"
)

// txKey é usado para armazenar a transação ativa no contexto
type txKey struct{}

// TxOptions configura opções para a transação
type TxOptions struct {
	Isolation sql.IsolationLevel
	ReadOnly  bool
	Timeout   time.Duration
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, tx database.DBExecutor) error) error
	DoWithOptions(ctx context.Context, opts *TxOptions, fn func(ctx context.Context, tx database.DBExecutor) error) error
	GetDB() *sql.DB
}

type unitOfWork struct {
	db               *sql.DB
	defaultIsolation sql.IsolationLevel
	defaultTimeout   time.Duration
}

// NewUnitOfWork cria um novo Unit of Work com isolation level SERIALIZABLE por padrão
// para prevenir lost updates, write skew e phantom reads
func NewUnitOfWork(db *sql.DB) UnitOfWork {
	return &unitOfWork{
		db:               db,
		defaultIsolation: sql.LevelSerializable, // Mais seguro para prevenir anomalias
		defaultTimeout:   30 * time.Second,      // Timeout padrão para evitar transações longas
	}
}

// NewUnitOfWorkWithOptions cria um Unit of Work com configurações customizadas
func NewUnitOfWorkWithOptions(db *sql.DB, isolation sql.IsolationLevel, timeout time.Duration) UnitOfWork {
	return &unitOfWork{
		db:               db,
		defaultIsolation: isolation,
		defaultTimeout:   timeout,
	}
}

func (u *unitOfWork) GetDB() *sql.DB {
	return u.db
}

func (u *unitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx database.DBExecutor) error) error {
	return u.DoWithOptions(ctx, nil, fn)
}

func (u *unitOfWork) DoWithOptions(ctx context.Context, opts *TxOptions, fn func(ctx context.Context, tx database.DBExecutor) error) error {
	// CORREÇÃO CRÍTICA 1: Detectar transações aninhadas
	if ctx.Value(txKey{}) != nil {
		return errors.New("nested transactions are not allowed: a transaction is already active in this context")
	}

	// Configurar opções da transação
	isolation := u.defaultIsolation
	timeout := u.defaultTimeout
	readOnly := false

	if opts != nil {
		if opts.Isolation != 0 {
			isolation = opts.Isolation
		}
		if opts.Timeout > 0 {
			timeout = opts.Timeout
		}
		readOnly = opts.ReadOnly
	}

	// CORREÇÃO CRÍTICA 2: Context com timeout para proteger contra transações longas
	txCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Iniciar transação com isolation level configurável
	txOptions := &sql.TxOptions{
		Isolation: isolation,
		ReadOnly:  readOnly,
	}

	tx, err := u.db.BeginTx(txCtx, txOptions)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Marcar contexto com transação ativa para detectar aninhamento
	txCtx = context.WithValue(txCtx, txKey{}, tx)

	// CORREÇÃO CRÍTICA 3: Proteção robusta contra panic duplo e garantia de rollback
	var committed bool
	defer func() {
		if p := recover(); p != nil {
			// Proteger contra panic durante rollback
			func() {
				defer func() {
					if panicErr := recover(); panicErr != nil {
						// Log do panic secundário seria ideal aqui
						// mas não temos logger injetado ainda
						_ = panicErr
					}
				}()
				// Tentar rollback apenas se não commitou
				if !committed {
					_ = tx.Rollback()
				}
			}()
			panic(p) // Re-lançar panic original
		}
	}()

	// Executar função com a transação
	execErr := fn(txCtx, tx)

	// CORREÇÃO CRÍTICA 4: Tratar rollback adequadamente
	if execErr != nil {
		// Tentar rollback
		if rbErr := tx.Rollback(); rbErr != nil {
			// Ignorar sql.ErrTxDone pois significa que a transação já foi finalizada
			// (pode acontecer com context cancellation)
			if !errors.Is(rbErr, sql.ErrTxDone) {
				return fmt.Errorf("transaction error: %w, rollback error: %v", execErr, rbErr)
			}
		}
		return execErr
	}

	// CORREÇÃO CRÍTICA 5: Commit com tentativa de rollback em caso de falha
	if err = tx.Commit(); err != nil {
		// Se commit falha, tentar rollback (embora geralmente seja tarde demais)
		// Isso é defensivo e ajuda em alguns cenários de erro
		if rbErr := tx.Rollback(); rbErr != nil {
			if !errors.Is(rbErr, sql.ErrTxDone) {
				return fmt.Errorf("failed to commit transaction: %w, rollback error: %v", err, rbErr)
			}
		}
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	committed = true
	return nil
}
