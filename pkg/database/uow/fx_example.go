package uow

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jailtonjunior94/financial/pkg/database"
	"go.uber.org/fx"
)

// FxModule fornece o Unit of Work como dependência do Uber FX
// Use este módulo no seu fx.New() principal
var FxModule = fx.Module(
	"uow",
	fx.Provide(
		NewUnitOfWorkFromDB,
		NewUnitOfWorkWithConfig,
	),
)

// UoWConfig contém configurações para o Unit of Work
type UoWConfig struct {
	DefaultIsolation sql.IsolationLevel
	DefaultTimeout   time.Duration
}

// NewUnitOfWorkFromDB cria um UoW básico com configurações padrão
// Esta é a opção mais simples - usa configurações seguras padrão
//
// Exemplo de uso no fx.New():
//
//	fx.New(
//	    uow.FxModule,
//	    fx.Provide(NewDatabaseConnection), // sua função que retorna *sql.DB
//	    fx.Invoke(RegisterHandlers),
//	)
func NewUnitOfWorkFromDB(db *sql.DB) UnitOfWork {
	return NewUnitOfWork(db)
}

// NewUnitOfWorkWithConfig cria um UoW com configurações customizadas
// Use esta opção quando precisar controlar isolation level e timeout
//
// Exemplo de uso no fx.New():
//
//	fx.New(
//	    fx.Provide(
//	        NewDatabaseConnection,
//	        NewUoWConfig, // sua função que retorna UoWConfig
//	        uow.NewUnitOfWorkWithConfig,
//	    ),
//	    fx.Invoke(RegisterHandlers),
//	)
func NewUnitOfWorkWithConfig(db *sql.DB, config UoWConfig) UnitOfWork {
	return NewUnitOfWorkWithOptions(db, config.DefaultIsolation, config.DefaultTimeout)
}

// ExampleConfig retorna configuração de exemplo para diferentes ambientes
func ExampleConfig(env string) UoWConfig {
	switch env {
	case "production":
		return UoWConfig{
			DefaultIsolation: sql.LevelReadCommitted, // Balance entre performance e segurança
			DefaultTimeout:   10 * time.Second,       // API típica
		}
	case "financial-system":
		return UoWConfig{
			DefaultIsolation: sql.LevelRepeatableRead, // Mais seguro para operações financeiras
			DefaultTimeout:   30 * time.Second,        // Operações mais complexas
		}
	case "batch-processing":
		return UoWConfig{
			DefaultIsolation: sql.LevelReadCommitted,
			DefaultTimeout:   5 * time.Minute, // Processos longos
		}
	case "read-heavy":
		return UoWConfig{
			DefaultIsolation: sql.LevelReadCommitted,
			DefaultTimeout:   5 * time.Second, // Queries rápidas
		}
	default: // development
		return UoWConfig{
			DefaultIsolation: sql.LevelReadCommitted,
			DefaultTimeout:   30 * time.Second,
		}
	}
}

// --- EXEMPLO 1: Aplicação Simples ---

// SimpleAppParams agrupa dependências necessárias
type SimpleAppParams struct {
	fx.In

	DB  *sql.DB
	UoW UnitOfWork
}

// SimpleAppExample mostra uso básico em uma aplicação simples
func SimpleAppExample() {
	app := fx.New(
		// Prover database connection
		fx.Provide(func() (*sql.DB, error) {
			return sql.Open("postgres", "postgres://localhost/mydb?sslmode=disable")
		}),

		// Prover Unit of Work (automático via FxModule)
		fx.Provide(NewUnitOfWorkFromDB),

		// Usar UoW em handlers/services
		fx.Invoke(func(uow UnitOfWork) {
			fmt.Println("UoW pronto para uso:", uow != nil)
		}),
	)

	app.Run()
}

// --- EXEMPLO 2: Repositórios e Use Cases ---

// UserRepository exemplo de repositório que usa UoW
type UserRepository struct {
	db *sql.DB
}

// TransferRepository exemplo de repositório financeiro
type TransferRepository struct {
	db *sql.DB
}

// TransferUseCase exemplo de use case que usa UoW para atomicidade
type TransferUseCase struct {
	uow          UnitOfWork
	userRepo     *UserRepository
	transferRepo *TransferRepository
}

// NewUserRepository construtor compatível com FX
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// NewTransferRepository construtor compatível com FX
func NewTransferRepository(db *sql.DB) *TransferRepository {
	return &TransferRepository{db: db}
}

// NewTransferUseCase construtor compatível com FX
func NewTransferUseCase(
	uow UnitOfWork,
	userRepo *UserRepository,
	transferRepo *TransferRepository,
) *TransferUseCase {
	return &TransferUseCase{
		uow:          uow,
		userRepo:     userRepo,
		transferRepo: transferRepo,
	}
}

// Execute executa transferência com garantia de atomicidade
func (uc *TransferUseCase) Execute(ctx context.Context, fromUserID, toUserID string, amount float64) error {
	return uc.uow.Do(ctx, func(ctx context.Context, tx database.DBExecutor) error {
		// Todas as operações dentro desta função são atômicas
		// Se qualquer operação falhar, TODAS são revertidas

		// 1. Debitar do usuário origem
		// 2. Creditar no usuário destino
		// 3. Registrar transferência

		return nil // commit automático se retornar nil
	})
}

// UseCaseAppExample mostra estrutura completa com use cases
func UseCaseAppExample() {
	app := fx.New(
		// Infraestrutura
		fx.Provide(
			func() (*sql.DB, error) {
				return sql.Open("postgres", "postgres://localhost/financial?sslmode=disable")
			},
			NewUnitOfWorkFromDB,
		),

		// Repositórios
		fx.Provide(
			NewUserRepository,
			NewTransferRepository,
		),

		// Use Cases
		fx.Provide(
			NewTransferUseCase,
		),

		// Registrar handlers HTTP
		fx.Invoke(func(transferUC *TransferUseCase) {
			fmt.Println("TransferUseCase registrado:", transferUC != nil)
		}),
	)

	app.Run()
}

// --- EXEMPLO 3: Múltiplos UoW com Configurações Diferentes ---

// ReadOnlyUoW marcador para UoW read-only
type ReadOnlyUoW UnitOfWork

// WriteUoW marcador para UoW de escrita
type WriteUoW UnitOfWork

// NewReadOnlyUoW cria UoW otimizado para leitura
func NewReadOnlyUoW(db *sql.DB) ReadOnlyUoW {
	return NewUnitOfWorkWithOptions(
		db,
		sql.LevelReadCommitted,
		5*time.Second, // Timeout mais curto para reads
	)
}

// NewWriteUoW cria UoW otimizado para escrita
func NewWriteUoW(db *sql.DB) WriteUoW {
	return NewUnitOfWorkWithOptions(
		db,
		sql.LevelRepeatableRead, // Mais seguro para writes
		30*time.Second,          // Timeout mais longo
	)
}

// ReportService usa UoW read-only
type ReportService struct {
	readUoW ReadOnlyUoW
}

// NewReportService construtor
func NewReportService(readUoW ReadOnlyUoW) *ReportService {
	return &ReportService{readUoW: readUoW}
}

// PaymentService usa UoW de escrita
type PaymentService struct {
	writeUoW WriteUoW
}

// NewPaymentService construtor
func NewPaymentService(writeUoW WriteUoW) *PaymentService {
	return &PaymentService{writeUoW: writeUoW}
}

// MultiUoWAppExample mostra uso de múltiplos UoW especializados
func MultiUoWAppExample() {
	app := fx.New(
		fx.Provide(
			func() (*sql.DB, error) {
				return sql.Open("postgres", "postgres://localhost/financial?sslmode=disable")
			},
			NewReadOnlyUoW,
			NewWriteUoW,
		),

		fx.Provide(
			NewReportService,
			NewPaymentService,
		),

		fx.Invoke(func(reports *ReportService, payments *PaymentService) {
			fmt.Println("Services prontos")
		}),
	)

	app.Run()
}

// --- EXEMPLO 4: Lifecycle Hooks ---

// DatabaseManager gerencia ciclo de vida da conexão
type DatabaseManager struct {
	db *sql.DB
}

// NewDatabaseManager cria manager com lifecycle hooks
func NewDatabaseManager(lc fx.Lifecycle, connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configurar pool de conexões
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			fmt.Println("🔌 Conectando ao banco de dados...")
			if err := db.PingContext(ctx); err != nil {
				return fmt.Errorf("failed to ping database: %w", err)
			}
			fmt.Println("✅ Database conectado com sucesso")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			fmt.Println("🔌 Fechando conexão com banco de dados...")
			if err := db.Close(); err != nil {
				return fmt.Errorf("failed to close database: %w", err)
			}
			fmt.Println("✅ Database desconectado com sucesso")
			return nil
		},
	})

	return db, nil
}

// LifecycleAppExample mostra gerenciamento completo de lifecycle
func LifecycleAppExample() {
	app := fx.New(
		fx.Provide(
			// Database com lifecycle management
			func(lc fx.Lifecycle) (*sql.DB, error) {
				return NewDatabaseManager(lc, "postgres://localhost/financial?sslmode=disable")
			},
			NewUnitOfWorkFromDB,
		),

		fx.Invoke(func(uow UnitOfWork) {
			fmt.Println("Aplicação inicializada com UoW")
		}),
	)

	app.Run()
}

// --- EXEMPLO 5: Estrutura Completa de Produção ---

// AppConfig configuração da aplicação
type AppConfig struct {
	DatabaseURL     string
	Environment     string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewAppConfig carrega configuração (pode vir de env vars, config file, etc)
func NewAppConfig() AppConfig {
	return AppConfig{
		DatabaseURL:     "postgres://localhost/financial?sslmode=disable",
		Environment:     "production",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
	}
}

// NewDatabase cria conexão com banco configurado
func NewDatabase(lc fx.Lifecycle, cfg AppConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configurar pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

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

// NewUoWConfigFromApp cria configuração do UoW baseado no ambiente
func NewUoWConfigFromApp(cfg AppConfig) UoWConfig {
	return ExampleConfig(cfg.Environment)
}

// ProductionAppExample estrutura completa para produção
func ProductionAppExample() {
	app := fx.New(
		// Configuração
		fx.Provide(NewAppConfig),

		// Infraestrutura
		fx.Provide(
			NewDatabase,
			NewUoWConfigFromApp,
			NewUnitOfWorkWithConfig,
		),

		// Repositórios (layer de dados)
		fx.Provide(
			NewUserRepository,
			NewTransferRepository,
			// ... outros repositórios
		),

		// Use Cases (layer de aplicação)
		fx.Provide(
			NewTransferUseCase,
			// ... outros use cases
		),

		// HTTP Handlers (layer de apresentação)
		fx.Invoke(func(
			transferUC *TransferUseCase,
			// outros use cases...
		) {
			// Registrar rotas HTTP aqui
			fmt.Println("HTTP handlers registrados")
		}),

		// Logging
		fx.WithLogger(func() fx.Printer {
			return fx.Printer(nil) // use seu logger aqui
		}),
	)

	app.Run()
}

// --- EXEMPLO 6: Testing com FX ---

// NewTestDatabase cria database in-memory para testes
func NewTestDatabase() (*sql.DB, error) {
	return sql.Open("postgres", "postgres://localhost:26257/financial_test?sslmode=disable")
}

// NewTestApp cria aplicação para testes
func NewTestApp(t interface{ Cleanup(func()) }) *fx.App {
	var db *sql.DB
	var uow UnitOfWork

	app := fx.New(
		fx.Provide(NewTestDatabase),
		fx.Provide(NewUnitOfWorkFromDB),
		fx.Populate(&db, &uow),
		fx.NopLogger, // Sem logs em testes
	)

	// Cleanup automático
	t.Cleanup(func() {
		if db != nil {
			db.Close()
		}
	})

	return app
}
