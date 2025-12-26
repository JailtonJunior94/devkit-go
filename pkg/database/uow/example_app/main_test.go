package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jailtonjunior94/financial/pkg/database/uow"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	_ "github.com/lib/pq"
)

// ==============================================================================
// TESTES COM FX
// ==============================================================================

// TransferUseCaseTestSuite suite de testes para TransferMoneyUseCase
type TransferUseCaseTestSuite struct {
	suite.Suite

	app         *fxtest.App
	db          *sql.DB
	uow         uow.UnitOfWork
	accountRepo AccountRepository
	transferUC  *TransferMoneyUseCase
	ctx         context.Context
}

func TestTransferUseCaseSuite(t *testing.T) {
	suite.Run(t, new(TransferUseCaseTestSuite))
}

// SetupSuite configura ambiente de teste com FX
func (s *TransferUseCaseTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Criar aplicação FX para testes
	var (
		db          *sql.DB
		uowInstance uow.UnitOfWork
		accountRepo AccountRepository
		transferUC  *TransferMoneyUseCase
	)

	app := fxtest.New(
		s.T(),

		// Configuração de teste
		fx.Provide(func() AppConfig {
			return AppConfig{
				DatabaseURL:     "postgres://root@localhost:26257/financial_test?sslmode=disable",
				Environment:     "test",
				MaxOpenConns:    5,
				MaxIdleConns:    2,
				ConnMaxLifetime: 1 * time.Minute,
			}
		}),

		// Infraestrutura
		fx.Provide(
			NewTestDatabase,
			NewUoWConfig,
			uow.NewUnitOfWorkWithConfig,
		),

		// Repositórios
		fx.Provide(
			NewAccountRepository,
			NewTransferRepository,
		),

		// Use Cases
		fx.Provide(
			NewTransferMoneyUseCase,
		),

		// Populate para extrair dependências
		fx.Populate(&db, &uowInstance, &accountRepo, &transferUC),

		// Sem logs em testes
		fx.NopLogger,
	)

	// Iniciar aplicação
	app.RequireStart()

	// Criar tabelas
	s.Require().NoError(createTables(s.ctx, db))

	// Armazenar dependências
	s.app = app
	s.db = db
	s.uow = uowInstance
	s.accountRepo = accountRepo
	s.transferUC = transferUC
}

// TearDownSuite limpa ambiente de teste
func (s *TransferUseCaseTestSuite) TearDownSuite() {
	if s.app != nil {
		s.app.RequireStop()
	}
}

// SetupTest limpa dados antes de cada teste
func (s *TransferUseCaseTestSuite) SetupTest() {
	// Limpar dados
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM transfers")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM accounts")
}

// NewTestDatabase cria database para testes
func NewTestDatabase(lc fx.Lifecycle, cfg AppConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}

// ==============================================================================
// TESTES - CENÁRIOS DE SUCESSO
// ==============================================================================

// TestTransferMoneySuccess testa transferência bem-sucedida
func (s *TransferUseCaseTestSuite) TestTransferMoneySuccess() {
	// Arrange: Criar contas de teste
	_, err := s.db.ExecContext(s.ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2), ($3, $4)",
		"ACC001", 1000.00, "ACC002", 500.00,
	)
	s.Require().NoError(err)

	input := TransferMoneyInput{
		FromAccountID: "ACC001",
		ToAccountID:   "ACC002",
		Amount:        100.00,
	}

	// Act: Executar transferência
	err = s.transferUC.Execute(s.ctx, input)

	// Assert: Verificar sucesso
	s.NoError(err)

	// Verificar saldos finais
	var fromBalance, toBalance float64
	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $1", "ACC001").Scan(&fromBalance)
	s.NoError(err)
	s.Equal(900.00, fromBalance, "Saldo da conta origem deve ser 900")

	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $2", "ACC002").Scan(&toBalance)
	s.NoError(err)
	s.Equal(600.00, toBalance, "Saldo da conta destino deve ser 600")

	// Verificar que transferência foi registrada
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM transfers").Scan(&count)
	s.NoError(err)
	s.Equal(1, count, "Deve ter exatamente 1 transferência registrada")
}

// ==============================================================================
// TESTES - CENÁRIOS DE ERRO
// ==============================================================================

// TestInsufficientBalance testa erro de saldo insuficiente
func (s *TransferUseCaseTestSuite) TestInsufficientBalance() {
	// Arrange: Conta com saldo insuficiente
	_, err := s.db.ExecContext(s.ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2), ($3, $4)",
		"ACC001", 50.00, "ACC002", 500.00,
	)
	s.Require().NoError(err)

	input := TransferMoneyInput{
		FromAccountID: "ACC001",
		ToAccountID:   "ACC002",
		Amount:        100.00, // Maior que saldo disponível
	}

	// Act
	err = s.transferUC.Execute(s.ctx, input)

	// Assert: Deve retornar erro
	s.Error(err)
	s.Contains(err.Error(), "insufficient balance")

	// Verificar que NENHUMA operação foi persistida (rollback)
	var fromBalance, toBalance float64
	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $1", "ACC001").Scan(&fromBalance)
	s.NoError(err)
	s.Equal(50.00, fromBalance, "Saldo da conta origem não deve ter mudado")

	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $2", "ACC002").Scan(&toBalance)
	s.NoError(err)
	s.Equal(500.00, toBalance, "Saldo da conta destino não deve ter mudado")

	// Verificar que transferência NÃO foi registrada
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM transfers").Scan(&count)
	s.NoError(err)
	s.Equal(0, count, "Não deve ter transferências registradas")
}

// TestAccountNotFound testa erro quando conta não existe
func (s *TransferUseCaseTestSuite) TestAccountNotFound() {
	// Arrange: Apenas uma conta existe
	_, err := s.db.ExecContext(s.ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2)",
		"ACC001", 1000.00,
	)
	s.Require().NoError(err)

	input := TransferMoneyInput{
		FromAccountID: "ACC001",
		ToAccountID:   "ACC999", // Não existe
		Amount:        100.00,
	}

	// Act
	err = s.transferUC.Execute(s.ctx, input)

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "to account not found")

	// Verificar rollback
	var fromBalance float64
	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $1", "ACC001").Scan(&fromBalance)
	s.NoError(err)
	s.Equal(1000.00, fromBalance, "Saldo não deve ter mudado")
}

// TestNegativeAmount testa validação de valor negativo
func (s *TransferUseCaseTestSuite) TestNegativeAmount() {
	// Arrange
	_, err := s.db.ExecContext(s.ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2), ($3, $4)",
		"ACC001", 1000.00, "ACC002", 500.00,
	)
	s.Require().NoError(err)

	input := TransferMoneyInput{
		FromAccountID: "ACC001",
		ToAccountID:   "ACC002",
		Amount:        -100.00, // Valor negativo
	}

	// Act
	err = s.transferUC.Execute(s.ctx, input)

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "amount must be positive")
}

// TestSameAccount testa transferência para mesma conta
func (s *TransferUseCaseTestSuite) TestSameAccount() {
	// Arrange
	_, err := s.db.ExecContext(s.ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2)",
		"ACC001", 1000.00,
	)
	s.Require().NoError(err)

	input := TransferMoneyInput{
		FromAccountID: "ACC001",
		ToAccountID:   "ACC001", // Mesma conta
		Amount:        100.00,
	}

	// Act
	err = s.transferUC.Execute(s.ctx, input)

	// Assert
	s.Error(err)
	s.Contains(err.Error(), "cannot transfer to same account")
}

// ==============================================================================
// TESTES - ATOMICIDADE
// ==============================================================================

// TestAtomicity testa que operações são atômicas
func (s *TransferUseCaseTestSuite) TestAtomicity() {
	// Este teste verifica que se QUALQUER operação falhar,
	// TODAS as operações são revertidas

	// Arrange
	_, err := s.db.ExecContext(s.ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2), ($3, $4)",
		"ACC001", 1000.00, "ACC002", 500.00,
	)
	s.Require().NoError(err)

	// Simular erro após debitar mas antes de creditar
	// (na prática, isso seria feito com um mock repository)

	// Para este teste, vamos usar valor muito alto
	input := TransferMoneyInput{
		FromAccountID: "ACC001",
		ToAccountID:   "ACC999", // Conta não existe - vai falhar
		Amount:        100.00,
	}

	// Act
	err = s.transferUC.Execute(s.ctx, input)

	// Assert: Deve ter falhado
	s.Error(err)

	// Verificar que conta origem NÃO foi debitada (rollback)
	var fromBalance float64
	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $1", "ACC001").Scan(&fromBalance)
	s.NoError(err)
	s.Equal(1000.00, fromBalance, "Saldo deve permanecer inalterado após rollback")
}

// ==============================================================================
// TESTES - CONCORRÊNCIA
// ==============================================================================

// TestConcurrentTransfers testa múltiplas transferências concorrentes
func (s *TransferUseCaseTestSuite) TestConcurrentTransfers() {
	// Arrange: Criar contas
	_, err := s.db.ExecContext(s.ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2), ($3, $4)",
		"ACC001", 1000.00, "ACC002", 0.00,
	)
	s.Require().NoError(err)

	// Act: Executar 10 transferências concorrentes de 100.00 cada
	const numTransfers = 10
	const amountPerTransfer = 100.00

	errChan := make(chan error, numTransfers)

	for i := 0; i < numTransfers; i++ {
		go func() {
			input := TransferMoneyInput{
				FromAccountID: "ACC001",
				ToAccountID:   "ACC002",
				Amount:        amountPerTransfer,
			}
			errChan <- s.transferUC.Execute(s.ctx, input)
		}()
	}

	// Coletar resultados
	var successCount int
	for i := 0; i < numTransfers; i++ {
		err := <-errChan
		if err == nil {
			successCount++
		}
	}

	// Assert: Todas devem ter sucesso
	s.Equal(numTransfers, successCount, "Todas as transferências devem ter sucesso")

	// Verificar saldos finais
	var fromBalance, toBalance float64
	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $1", "ACC001").Scan(&fromBalance)
	s.NoError(err)
	s.Equal(0.00, fromBalance, "Conta origem deve ter saldo zero")

	err = s.db.QueryRowContext(s.ctx, "SELECT balance FROM accounts WHERE id = $2", "ACC002").Scan(&toBalance)
	s.NoError(err)
	s.Equal(1000.00, toBalance, "Conta destino deve ter saldo 1000")

	// Verificar número de transferências registradas
	var count int
	err = s.db.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM transfers").Scan(&count)
	s.NoError(err)
	s.Equal(numTransfers, count, "Deve ter exatamente 10 transferências registradas")
}

// ==============================================================================
// BENCHMARK
// ==============================================================================

// BenchmarkTransfer benchmark de transferência
func BenchmarkTransfer(b *testing.B) {
	// Setup
	var (
		db         *sql.DB
		transferUC *TransferMoneyUseCase
	)

	app := fxtest.New(
		b,
		fx.Provide(
			func() AppConfig {
				return AppConfig{
					DatabaseURL:     "postgres://root@localhost:26257/financial_bench?sslmode=disable",
					Environment:     "test",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				}
			},
			NewTestDatabase,
			NewUoWConfig,
			uow.NewUnitOfWorkWithConfig,
			NewAccountRepository,
			NewTransferRepository,
			NewTransferMoneyUseCase,
		),
		fx.Populate(&db, &transferUC),
		fx.NopLogger,
	)

	app.RequireStart()
	defer app.RequireStop()

	ctx := context.Background()

	// Criar tabelas e dados
	_ = createTables(ctx, db)
	_, _ = db.ExecContext(ctx, "DELETE FROM transfers")
	_, _ = db.ExecContext(ctx, "DELETE FROM accounts")
	_, _ = db.ExecContext(ctx,
		"INSERT INTO accounts (id, balance) VALUES ($1, $2), ($3, $4)",
		"ACC001", 1000000.00, "ACC002", 0.00,
	)

	b.ResetTimer()

	// Benchmark
	for i := 0; i < b.N; i++ {
		input := TransferMoneyInput{
			FromAccountID: "ACC001",
			ToAccountID:   "ACC002",
			Amount:        1.00,
		}
		_ = transferUC.Execute(ctx, input)
	}
}
