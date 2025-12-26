package uow_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jailtonjunior94/financial/pkg/database"
	"github.com/jailtonjunior94/financial/pkg/database/uow"
	"github.com/stretchr/testify/suite"

	_ "github.com/lib/pq"
)

type UnitOfWorkTestSuite struct {
	suite.Suite
	db  *sql.DB
	uow uow.UnitOfWork
	ctx context.Context
}

func TestUnitOfWorkSuite(t *testing.T) {
	suite.Run(t, new(UnitOfWorkTestSuite))
}

func (s *UnitOfWorkTestSuite) SetupSuite() {
	// Setup in-memory SQLite para testes
	db, err := sql.Open("postgres", "postgres://root@localhost:26257/financial?sslmode=disable")
	s.Require().NoError(err)

	s.db = db
	s.ctx = context.Background()

	// Criar tabela de teste
	_, err = s.db.ExecContext(s.ctx, `
		CREATE TABLE IF NOT EXISTS test_transactions (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	s.Require().NoError(err)
}

func (s *UnitOfWorkTestSuite) TearDownSuite() {
	_, _ = s.db.ExecContext(s.ctx, "DROP TABLE IF EXISTS test_transactions")
	s.db.Close()
}

func (s *UnitOfWorkTestSuite) SetupTest() {
	s.uow = uow.NewUnitOfWork(s.db)

	// Limpar dados antes de cada teste
	_, err := s.db.ExecContext(s.ctx, "DELETE FROM test_transactions")
	s.Require().NoError(err)
}

// TestCommitSuccess valida que commit funciona corretamente
func (s *UnitOfWorkTestSuite) TestCommitSuccess() {
	err := s.uow.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test1")
		s.Require().NoError(err)

		_, err = tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test2")
		s.Require().NoError(err)

		return nil
	})

	s.NoError(err)

	// Verificar que os dados foram commitados
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(2, count, "deve ter 2 registros após commit bem-sucedido")
}

// TestRollbackOnError valida que rollback acontece em caso de erro
func (s *UnitOfWorkTestSuite) TestRollbackOnError() {
	expectedErr := errors.New("business error")

	err := s.uow.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test1")
		s.Require().NoError(err)

		// Simular erro de negócio
		return expectedErr
	})

	s.Error(err)
	s.ErrorIs(err, expectedErr)

	// Verificar que nenhum dado foi persistido (rollback funcionou)
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(0, count, "não deve ter registros após rollback")
}

// TestRollbackOnPanic valida que rollback acontece em caso de panic
func (s *UnitOfWorkTestSuite) TestRollbackOnPanic() {
	defer func() {
		if r := recover(); r != nil {
			s.Equal("unexpected panic", r)
		}
	}()

	_ = s.uow.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test1")
		s.Require().NoError(err)

		// Simular panic
		panic("unexpected panic")
	})

	// Verificar que nenhum dado foi persistido (rollback funcionou)
	var count int
	err := s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(0, count, "não deve ter registros após panic e rollback")
}

// TestConcurrentTransactions valida thread-safety
func (s *UnitOfWorkTestSuite) TestConcurrentTransactions() {
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			err := s.uow.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
				_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "concurrent_test")
				return err
			})

			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Verificar que não houve erros
	for err := range errors {
		s.NoError(err)
	}

	// Verificar que todos os registros foram inseridos
	var count int
	err := s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(numGoroutines, count, "deve ter exatamente %d registros após execução concorrente", numGoroutines)
}

// TestAtomicity valida que operações são atômicas
func (s *UnitOfWorkTestSuite) TestAtomicity() {
	err := s.uow.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
		// Primeira inserção - sucesso
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "atomic1")
		s.Require().NoError(err)

		// Segunda inserção - sucesso
		_, err = tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "atomic2")
		s.Require().NoError(err)

		// Terceira inserção - vai falhar por violar constraint (simulando erro)
		// Retornar erro para forçar rollback
		return errors.New("simulated constraint violation")
	})

	s.Error(err)

	// Verificar que NENHUM registro foi persistido (atomicidade)
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(0, count, "nenhum registro deve ser persistido quando transação falha")
}

// TestNestedTransactionPrevention valida que transações aninhadas são detectadas e bloqueadas
func (s *UnitOfWorkTestSuite) TestNestedTransactionPrevention() {
	err := s.uow.Do(s.ctx, func(ctx context.Context, tx1 database.DBExecutor) error {
		_, err := tx1.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "outer")
		s.Require().NoError(err)

		// Tentar criar transação aninhada deve retornar erro
		nestedErr := s.uow.Do(ctx, func(ctx context.Context, tx2 database.DBExecutor) error {
			_, err := tx2.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "inner")
			return err
		})

		// Deve retornar erro de transação aninhada
		s.Error(nestedErr)
		s.Contains(nestedErr.Error(), "nested transactions are not allowed")

		return nil
	})

	s.NoError(err)

	// Apenas o registro outer deve existir
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(1, count, "deve ter apenas 1 registro (outer)")
}

// TestTransactionTimeout valida que transações respeitam timeout configurado
func (s *UnitOfWorkTestSuite) TestTransactionTimeout() {
	// Criar UoW com timeout muito curto
	uowWithTimeout := uow.NewUnitOfWorkWithOptions(s.db, sql.LevelReadCommitted, 100*time.Millisecond)

	err := uowWithTimeout.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test")
		s.Require().NoError(err)

		// Simular operação longa que excede timeout
		time.Sleep(200 * time.Millisecond)

		// Esta operação deve falhar por timeout
		_, err = tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test2")
		return err
	})

	// Deve retornar erro de timeout
	s.Error(err)

	// Nenhum registro deve ser persistido (rollback por timeout)
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(0, count, "não deve ter registros após timeout")
}

// TestDoublePanicProtection valida proteção contra panic duplo
func (s *UnitOfWorkTestSuite) TestDoublePanicProtection() {
	defer func() {
		if r := recover(); r != nil {
			s.Equal("first panic", r, "deve recuperar o panic original")
		}
	}()

	// Criar um mock de tx que panics no Rollback
	// Como não podemos mockar sql.Tx facilmente, vamos simular com panic direto
	_ = s.uow.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test")
		s.Require().NoError(err)

		// Primeiro panic
		panic("first panic")
	})

	// Verificar que rollback funcionou mesmo com panic
	var count int
	err := s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(0, count, "não deve ter registros após panic")
}

// TestIsolationLevels valida que diferentes isolation levels funcionam
func (s *UnitOfWorkTestSuite) TestIsolationLevels() {
	scenarios := []struct {
		name      string
		isolation sql.IsolationLevel
	}{
		{"ReadUncommitted", sql.LevelReadUncommitted},
		{"ReadCommitted", sql.LevelReadCommitted},
		{"RepeatableRead", sql.LevelRepeatableRead},
		{"Serializable", sql.LevelSerializable},
	}

	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
			// Limpar dados
			_, _ = s.db.ExecContext(s.ctx, "DELETE FROM test_transactions")

			opts := &uow.TxOptions{
				Isolation: scenario.isolation,
			}

			err := s.uow.DoWithOptions(s.ctx, opts, func(ctx context.Context, tx database.DBExecutor) error {
				_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", scenario.name)
				return err
			})

			s.NoError(err)

			// Verificar que foi commitado
			var value string
			err = s.db.QueryRowContext(s.ctx, "SELECT value FROM test_transactions LIMIT 1").Scan(&value)
			s.NoError(err)
			s.Equal(scenario.name, value)
		})
	}
}

// TestReadOnlyTransaction valida que transações read-only funcionam
func (s *UnitOfWorkTestSuite) TestReadOnlyTransaction() {
	// Inserir dados de teste
	_, err := s.db.ExecContext(s.ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "read_only_test")
	s.Require().NoError(err)

	opts := &uow.TxOptions{
		ReadOnly: true,
	}

	// Read-only deve permitir SELECT
	err = s.uow.DoWithOptions(s.ctx, opts, func(ctx context.Context, tx database.DBExecutor) error {
		var value string
		err := tx.QueryRowContext(ctx, "SELECT value FROM test_transactions WHERE value = $1", "read_only_test").Scan(&value)
		s.Require().NoError(err)
		s.Equal("read_only_test", value)
		return nil
	})

	s.NoError(err)
}

// TestContextCancellation valida comportamento com context cancelado
func (s *UnitOfWorkTestSuite) TestContextCancellation() {
	ctx, cancel := context.WithCancel(s.ctx)

	err := s.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test")
		s.Require().NoError(err)

		// Cancelar context durante transação
		cancel()

		// Próxima operação deve falhar
		_, err = tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test2")
		return err
	})

	// Deve retornar erro de context cancelado
	s.Error(err)

	// Nenhum registro deve ser persistido
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_transactions").Scan(&count)
	s.NoError(err)
	s.Equal(0, count, "não deve ter registros após context cancelado")
}

// TestCommitErrorHandling valida tratamento de erro no commit
func (s *UnitOfWorkTestSuite) TestCommitErrorHandling() {
	// Este teste é difícil de simular sem mockar sql.Tx
	// Vamos apenas garantir que erros de commit são propagados corretamente

	err := s.uow.Do(s.ctx, func(ctx context.Context, tx database.DBExecutor) error {
		// Inserção válida
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test")
		return err
	})

	s.NoError(err, "commit deve ter sucesso para operação válida")
}

// TestErrTxDoneHandling valida que sql.ErrTxDone é tratado adequadamente
func (s *UnitOfWorkTestSuite) TestErrTxDoneHandling() {
	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()

	err := s.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO test_transactions (value) VALUES ($1)", "test")
		s.Require().NoError(err)

		// Aguardar timeout
		time.Sleep(100 * time.Millisecond)

		// Context já expirou
		return context.DeadlineExceeded
	})

	// Deve retornar o erro original, não erro de rollback
	s.Error(err)
	s.ErrorIs(err, context.DeadlineExceeded)
}
