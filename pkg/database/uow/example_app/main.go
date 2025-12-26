package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jailtonjunior94/financial/pkg/database"
	"github.com/jailtonjunior94/financial/pkg/database/uow"
	"go.uber.org/fx"

	_ "github.com/lib/pq"
)

// ==============================================================================
// DOMAIN LAYER - Entidades e Interfaces
// ==============================================================================

// Account representa uma conta bancária
type Account struct {
	ID      string
	Balance float64
}

// Transfer representa uma transferência
type Transfer struct {
	ID          string
	FromAccount string
	ToAccount   string
	Amount      float64
	CreatedAt   time.Time
}

// AccountRepository interface do repositório
type AccountRepository interface {
	FindByID(ctx context.Context, tx database.DBExecutor, id string) (*Account, error)
	UpdateBalance(ctx context.Context, tx database.DBExecutor, id string, balance float64) error
}

// TransferRepository interface do repositório
type TransferRepository interface {
	Create(ctx context.Context, tx database.DBExecutor, transfer *Transfer) error
}

// ==============================================================================
// INFRASTRUCTURE LAYER - Implementação dos Repositórios
// ==============================================================================

type accountRepository struct {
	db *sql.DB
}

// NewAccountRepository cria repositório de contas
func NewAccountRepository(db *sql.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) FindByID(ctx context.Context, tx database.DBExecutor, id string) (*Account, error) {
	var account Account
	query := `SELECT id, balance FROM accounts WHERE id = $1`
	err := tx.QueryRowContext(ctx, query, id).Scan(&account.ID, &account.Balance)
	if err != nil {
		return nil, fmt.Errorf("failed to find account: %w", err)
	}
	return &account, nil
}

func (r *accountRepository) UpdateBalance(ctx context.Context, tx database.DBExecutor, id string, balance float64) error {
	query := `UPDATE accounts SET balance = $1 WHERE id = $2`
	_, err := tx.ExecContext(ctx, query, balance, id)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}
	return nil
}

type transferRepository struct {
	db *sql.DB
}

// NewTransferRepository cria repositório de transferências
func NewTransferRepository(db *sql.DB) TransferRepository {
	return &transferRepository{db: db}
}

func (r *transferRepository) Create(ctx context.Context, tx database.DBExecutor, transfer *Transfer) error {
	query := `
		INSERT INTO transfers (id, from_account, to_account, amount, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.ExecContext(ctx, query,
		transfer.ID,
		transfer.FromAccount,
		transfer.ToAccount,
		transfer.Amount,
		transfer.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create transfer: %w", err)
	}
	return nil
}

// ==============================================================================
// APPLICATION LAYER - Use Cases
// ==============================================================================

// TransferMoneyInput entrada do use case
type TransferMoneyInput struct {
	FromAccountID string
	ToAccountID   string
	Amount        float64
}

// TransferMoneyUseCase orquestra transferência entre contas
type TransferMoneyUseCase struct {
	uow          uow.UnitOfWork
	accountRepo  AccountRepository
	transferRepo TransferRepository
}

// NewTransferMoneyUseCase construtor do use case
func NewTransferMoneyUseCase(
	uow uow.UnitOfWork,
	accountRepo AccountRepository,
	transferRepo TransferRepository,
) *TransferMoneyUseCase {
	return &TransferMoneyUseCase{
		uow:          uow,
		accountRepo:  accountRepo,
		transferRepo: transferRepo,
	}
}

// Execute executa transferência com garantia de atomicidade
func (uc *TransferMoneyUseCase) Execute(ctx context.Context, input TransferMoneyInput) error {
	// Validações básicas
	if input.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if input.FromAccountID == input.ToAccountID {
		return fmt.Errorf("cannot transfer to same account")
	}

	// Executar transação atômica
	return uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
		// 1. Buscar conta origem com lock pessimista (SELECT FOR UPDATE)
		fromAccount, err := uc.accountRepo.FindByID(ctx, tx, input.FromAccountID)
		if err != nil {
			return fmt.Errorf("from account not found: %w", err)
		}

		// 2. Buscar conta destino com lock pessimista
		toAccount, err := uc.accountRepo.FindByID(ctx, tx, input.ToAccountID)
		if err != nil {
			return fmt.Errorf("to account not found: %w", err)
		}

		// 3. Validar saldo suficiente
		if fromAccount.Balance < input.Amount {
			return fmt.Errorf("insufficient balance: have %.2f, need %.2f",
				fromAccount.Balance, input.Amount)
		}

		// 4. Debitar da conta origem
		newFromBalance := fromAccount.Balance - input.Amount
		if err := uc.accountRepo.UpdateBalance(ctx, tx, fromAccount.ID, newFromBalance); err != nil {
			return fmt.Errorf("failed to debit from account: %w", err)
		}

		// 5. Creditar na conta destino
		newToBalance := toAccount.Balance + input.Amount
		if err := uc.accountRepo.UpdateBalance(ctx, tx, toAccount.ID, newToBalance); err != nil {
			return fmt.Errorf("failed to credit to account: %w", err)
		}

		// 6. Registrar transferência
		transfer := &Transfer{
			ID:          fmt.Sprintf("TRF-%d", time.Now().UnixNano()),
			FromAccount: fromAccount.ID,
			ToAccount:   toAccount.ID,
			Amount:      input.Amount,
			CreatedAt:   time.Now(),
		}
		if err := uc.transferRepo.Create(ctx, tx, transfer); err != nil {
			return fmt.Errorf("failed to create transfer record: %w", err)
		}

		log.Printf("✅ Transfer successful: %s -> %s (%.2f)",
			fromAccount.ID, toAccount.ID, input.Amount)

		// Se qualquer operação falhar, TUDO é revertido automaticamente
		// Se retornar nil, commit é feito automaticamente
		return nil
	})
}

// ==============================================================================
// PRESENTATION LAYER - HTTP Handlers
// ==============================================================================

// TransferHandler handler HTTP para transferências
type TransferHandler struct {
	transferUC *TransferMoneyUseCase
}

// NewTransferHandler cria handler
func NewTransferHandler(transferUC *TransferMoneyUseCase) *TransferHandler {
	return &TransferHandler{transferUC: transferUC}
}

// HandleTransfer processa requisição de transferência
func (h *TransferHandler) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	// Em produção, use um parser JSON adequado
	fromAccount := r.URL.Query().Get("from")
	toAccount := r.URL.Query().Get("to")
	amount := 100.0 // Simplificado para exemplo

	input := TransferMoneyInput{
		FromAccountID: fromAccount,
		ToAccountID:   toAccount,
		Amount:        amount,
	}

	err := h.transferUC.Execute(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Transfer completed successfully")
}

// ==============================================================================
// INFRASTRUCTURE - Database Setup
// ==============================================================================

// AppConfig configuração da aplicação
type AppConfig struct {
	DatabaseURL     string
	ServerPort      string
	Environment     string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewAppConfig carrega configuração
func NewAppConfig() AppConfig {
	return AppConfig{
		DatabaseURL:     "postgres://root@localhost:26257/financial?sslmode=disable",
		ServerPort:      ":8080",
		Environment:     "development",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// NewDatabase cria e configura conexão com banco
func NewDatabase(lc fx.Lifecycle, cfg AppConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configurar pool de conexões
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(30 * time.Second)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("🔌 Conectando ao banco de dados...")
			if err := db.PingContext(ctx); err != nil {
				return fmt.Errorf("failed to ping database: %w", err)
			}
			log.Println("✅ Banco de dados conectado")

			// Criar tabelas de exemplo (em produção, use migrations)
			if err := createTables(ctx, db); err != nil {
				return fmt.Errorf("failed to create tables: %w", err)
			}
			log.Println("✅ Tabelas criadas/verificadas")

			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("🔌 Fechando conexão com banco...")
			return db.Close()
		},
	})

	return db, nil
}

func createTables(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			balance DECIMAL(15,2) NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS transfers (
			id TEXT PRIMARY KEY,
			from_account TEXT NOT NULL,
			to_account TEXT NOT NULL,
			amount DECIMAL(15,2) NOT NULL,
			created_at TIMESTAMP NOT NULL
		)`,
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	// Inserir contas de exemplo se não existirem
	_, _ = db.ExecContext(ctx, `
		INSERT INTO accounts (id, balance)
		VALUES ('ACC001', 1000.00), ('ACC002', 500.00)
		ON CONFLICT (id) DO NOTHING
	`)

	return nil
}

// NewUoWConfig cria configuração do Unit of Work
func NewUoWConfig(cfg AppConfig) uow.UoWConfig {
	return uow.ExampleConfig(cfg.Environment)
}

// ==============================================================================
// HTTP SERVER
// ==============================================================================

// HTTPServer servidor HTTP
type HTTPServer struct {
	server  *http.Server
	handler *TransferHandler
}

// NewHTTPServer cria servidor HTTP
func NewHTTPServer(
	lc fx.Lifecycle,
	cfg AppConfig,
	handler *TransferHandler,
) *HTTPServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/transfer", handler.HandleTransfer)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: mux,
	}

	httpServer := &HTTPServer{
		server:  srv,
		handler: handler,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Printf("🚀 Servidor HTTP iniciando na porta %s", cfg.ServerPort)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Printf("❌ Erro no servidor HTTP: %v", err)
				}
			}()
			log.Println("✅ Servidor HTTP pronto")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("🛑 Desligando servidor HTTP...")
			return srv.Shutdown(ctx)
		},
	})

	return httpServer
}

// ==============================================================================
// MAIN APPLICATION
// ==============================================================================

func main() {
	app := fx.New(
		// ========== CONFIGURAÇÃO ==========
		fx.Provide(NewAppConfig),

		// ========== INFRAESTRUTURA ==========
		fx.Provide(
			NewDatabase,
			NewUoWConfig,
			uow.NewUnitOfWorkWithConfig,
		),

		// ========== REPOSITÓRIOS ==========
		fx.Provide(
			NewAccountRepository,
			NewTransferRepository,
		),

		// ========== USE CASES ==========
		fx.Provide(
			NewTransferMoneyUseCase,
		),

		// ========== HANDLERS ==========
		fx.Provide(
			NewTransferHandler,
		),

		// ========== HTTP SERVER ==========
		fx.Provide(
			NewHTTPServer,
		),

		// ========== INVOKE ==========
		fx.Invoke(func(*HTTPServer) {
			// Apenas para garantir que o servidor seja instanciado
		}),
	)

	app.Run()
}
