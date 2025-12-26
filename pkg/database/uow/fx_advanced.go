package uow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jailtonjunior94/financial/pkg/database"
	"go.uber.org/fx"
)

// ==============================================================================
// PADRÃO 1: MÚLTIPLOS BANCOS DE DADOS
// ==============================================================================

// PrimaryDB marcador para banco principal
type PrimaryDB struct {
	*sql.DB
}

// ReplicaDB marcador para réplica read-only
type ReplicaDB struct {
	*sql.DB
}

// PrimaryUoW Unit of Work para banco principal
type PrimaryUoW UnitOfWork

// ReplicaUoW Unit of Work para réplica (read-only)
type ReplicaUoW UnitOfWork

// NewPrimaryDB cria conexão com banco principal
func NewPrimaryDB(lc fx.Lifecycle, primaryURL string) (PrimaryDB, error) {
	db, err := sql.Open("postgres", primaryURL)
	if err != nil {
		return PrimaryDB{}, err
	}

	// Configurações para writes
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return PrimaryDB{db}, nil
}

// NewReplicaDB cria conexão com réplica
func NewReplicaDB(lc fx.Lifecycle, replicaURL string) (ReplicaDB, error) {
	db, err := sql.Open("postgres", replicaURL)
	if err != nil {
		return ReplicaDB{}, err
	}

	// Configurações para reads (mais conexões)
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return ReplicaDB{db}, nil
}

// NewPrimaryUoW cria UoW para writes no banco principal
func NewPrimaryUoW(db PrimaryDB) PrimaryUoW {
	return PrimaryUoW(NewUnitOfWorkWithOptions(
		db.DB,
		sql.LevelReadCommitted,
		30*time.Second,
	))
}

// NewReplicaUoW cria UoW para reads na réplica
func NewReplicaUoW(db ReplicaDB) ReplicaUoW {
	return ReplicaUoW(NewUnitOfWorkWithOptions(
		db.DB,
		sql.LevelReadCommitted,
		5*time.Second, // Timeout menor para reads
	))
}

// MultiDatabaseExample exemplo de uso com múltiplos bancos
func MultiDatabaseExample() {
	app := fx.New(
		fx.Provide(
			func() string { return "postgres://primary:5432/db" }, // Primary URL
			func() string { return "postgres://replica:5432/db" }, // Replica URL
			NewPrimaryDB,
			NewReplicaDB,
			NewPrimaryUoW,
			NewReplicaUoW,
		),

		fx.Invoke(func(writeUoW PrimaryUoW, readUoW ReplicaUoW) {
			// Use writeUoW para writes
			// Use readUoW para reads
			fmt.Println("Multi-database configurado")
		}),
	)

	app.Run()
}

// ==============================================================================
// PADRÃO 2: RETRY LOGIC
// ==============================================================================

// RetryConfig configuração de retry
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// RetryableUoW Unit of Work com retry logic
type RetryableUoW interface {
	UnitOfWork
	DoWithRetry(ctx context.Context, fn func(ctx context.Context, tx database.DBExecutor) error) error
}

type retryableUoW struct {
	UnitOfWork
	config RetryConfig
}

// NewRetryableUoW cria UoW com retry logic
func NewRetryableUoW(base UnitOfWork, config RetryConfig) RetryableUoW {
	return &retryableUoW{
		UnitOfWork: base,
		config:     config,
	}
}

// DoWithRetry executa com retry automático para erros retriáveis
func (r *retryableUoW) DoWithRetry(ctx context.Context, fn func(ctx context.Context, tx database.DBExecutor) error) error {
	var lastErr error
	delay := r.config.InitialDelay

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		err := r.Do(ctx, fn)
		if err == nil {
			return nil // Sucesso
		}

		lastErr = err

		// Verificar se erro é retriável
		if !isRetriableError(err) {
			return err // Erro não-retriável, retornar imediatamente
		}

		// Última tentativa, não fazer retry
		if attempt == r.config.MaxAttempts {
			break
		}

		// Exponential backoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			delay = time.Duration(float64(delay) * r.config.Multiplier)
			if delay > r.config.MaxDelay {
				delay = r.config.MaxDelay
			}
		}
	}

	return fmt.Errorf("max retry attempts (%d) reached: %w", r.config.MaxAttempts, lastErr)
}

// isRetriableError verifica se erro é retriável
func isRetriableError(err error) bool {
	// Erros retriáveis comuns em bancos de dados
	retriableErrors := []string{
		"deadlock",
		"lock timeout",
		"serialization failure",
		"could not serialize access",
		"connection refused",
		"connection reset",
		"i/o timeout",
	}

	errMsg := err.Error()
	for _, retriable := range retriableErrors {
		if contains(errMsg, retriable) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr))
}

// RetryExample exemplo de uso com retry
func RetryExample() {
	app := fx.New(
		fx.Provide(
			func() (*sql.DB, error) {
				return sql.Open("postgres", "postgres://localhost/db?sslmode=disable")
			},
			NewUnitOfWorkFromDB,
			func(base UnitOfWork) RetryableUoW {
				return NewRetryableUoW(base, RetryConfig{
					MaxAttempts:  3,
					InitialDelay: 100 * time.Millisecond,
					MaxDelay:     1 * time.Second,
					Multiplier:   2.0,
				})
			},
		),

		fx.Invoke(func(uow RetryableUoW) {
			// Retry automático em caso de deadlock
			err := uow.DoWithRetry(context.Background(), func(ctx context.Context, tx database.DBExecutor) error {
				// Sua lógica aqui
				return nil
			})
			if err != nil {
				fmt.Printf("Erro após retries: %v\n", err)
			}
		}),
	)

	app.Run()
}

// ==============================================================================
// PADRÃO 3: CIRCUIT BREAKER
// ==============================================================================

// CircuitBreakerState estados do circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreakerConfig configuração do circuit breaker
type CircuitBreakerConfig struct {
	MaxFailures  int
	ResetTimeout time.Duration
}

// CircuitBreakerUoW Unit of Work com circuit breaker
type CircuitBreakerUoW interface {
	UnitOfWork
	GetState() CircuitBreakerState
	Reset()
}

type circuitBreakerUoW struct {
	UnitOfWork
	config       CircuitBreakerConfig
	failures     int
	state        CircuitBreakerState
	lastFailTime time.Time
}

// NewCircuitBreakerUoW cria UoW com circuit breaker
func NewCircuitBreakerUoW(base UnitOfWork, config CircuitBreakerConfig) CircuitBreakerUoW {
	return &circuitBreakerUoW{
		UnitOfWork: base,
		config:     config,
		state:      StateClosed,
	}
}

func (cb *circuitBreakerUoW) Do(ctx context.Context, fn func(ctx context.Context, tx database.DBExecutor) error) error {
	// Verificar se circuit está aberto
	if cb.state == StateOpen {
		// Verificar se timeout expirou
		if time.Since(cb.lastFailTime) > cb.config.ResetTimeout {
			cb.state = StateHalfOpen
			cb.failures = 0
		} else {
			return errors.New("circuit breaker is open")
		}
	}

	// Executar operação
	err := cb.UnitOfWork.Do(ctx, fn)

	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()

		// Abrir circuit se atingiu max failures
		if cb.failures >= cb.config.MaxFailures {
			cb.state = StateOpen
		}

		return err
	}

	// Sucesso - fechar circuit
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
	}
	cb.failures = 0

	return nil
}

func (cb *circuitBreakerUoW) GetState() CircuitBreakerState {
	return cb.state
}

func (cb *circuitBreakerUoW) Reset() {
	cb.state = StateClosed
	cb.failures = 0
}

// CircuitBreakerExample exemplo de uso com circuit breaker
func CircuitBreakerExample() {
	app := fx.New(
		fx.Provide(
			func() (*sql.DB, error) {
				return sql.Open("postgres", "postgres://localhost/db?sslmode=disable")
			},
			NewUnitOfWorkFromDB,
			func(base UnitOfWork) CircuitBreakerUoW {
				return NewCircuitBreakerUoW(base, CircuitBreakerConfig{
					MaxFailures:  5,
					ResetTimeout: 30 * time.Second,
				})
			},
		),

		fx.Invoke(func(uow CircuitBreakerUoW) {
			err := uow.Do(context.Background(), func(ctx context.Context, tx database.DBExecutor) error {
				// Sua lógica aqui
				return nil
			})

			if err != nil {
				if errors.Is(err, errors.New("circuit breaker is open")) {
					fmt.Println("Circuit breaker aberto - serviço temporariamente indisponível")
				}
			}
		}),
	)

	app.Run()
}

// ==============================================================================
// PADRÃO 4: OBSERVABILIDADE (METRICS + TRACING)
// ==============================================================================

// Metrics interface para coletar métricas
type Metrics interface {
	RecordTransactionDuration(duration time.Duration, success bool)
	RecordTransactionError(err error)
	IncrementActiveTransactions()
	DecrementActiveTransactions()
}

// ObservableUoW Unit of Work com observabilidade
type ObservableUoW interface {
	UnitOfWork
}

type observableUoW struct {
	UnitOfWork
	metrics Metrics
}

// NewObservableUoW cria UoW com observabilidade
func NewObservableUoW(base UnitOfWork, metrics Metrics) ObservableUoW {
	return &observableUoW{
		UnitOfWork: base,
		metrics:    metrics,
	}
}

func (o *observableUoW) Do(ctx context.Context, fn func(ctx context.Context, tx database.DBExecutor) error) error {
	start := time.Now()
	o.metrics.IncrementActiveTransactions()
	defer o.metrics.DecrementActiveTransactions()

	err := o.UnitOfWork.Do(ctx, fn)

	duration := time.Since(start)
	o.metrics.RecordTransactionDuration(duration, err == nil)

	if err != nil {
		o.metrics.RecordTransactionError(err)
	}

	return err
}

// ==============================================================================
// PADRÃO 5: COMPOSIÇÃO COMPLETA (RETRY + CIRCUIT BREAKER + OBSERVABILITY)
// ==============================================================================

// EnhancedUoW Unit of Work com todas as features
type EnhancedUoW interface {
	RetryableUoW
	CircuitBreakerUoW
	ObservableUoW
}

// NewEnhancedUoW cria UoW com todas as features
func NewEnhancedUoW(
	base UnitOfWork,
	retryConfig RetryConfig,
	cbConfig CircuitBreakerConfig,
	metrics Metrics,
) EnhancedUoW {
	// Composição em camadas
	observed := NewObservableUoW(base, metrics)
	withCircuitBreaker := NewCircuitBreakerUoW(observed, cbConfig)
	withRetry := NewRetryableUoW(withCircuitBreaker, retryConfig)

	// Type assertion para compor interfaces
	// Em produção, crie um tipo concreto que implementa todas as interfaces
	return withRetry.(EnhancedUoW)
}

// ==============================================================================
// PADRÃO 6: IDEMPOTENCY
// ==============================================================================

// IdempotencyStore armazena resultados de operações idempotentes
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (interface{}, bool, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// IdempotentUoW Unit of Work com suporte a idempotência
type IdempotentUoW interface {
	UnitOfWork
	DoIdempotent(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context, tx database.DBExecutor) error) error
}

type idempotentUoW struct {
	UnitOfWork
	store IdempotencyStore
}

// NewIdempotentUoW cria UoW com idempotência
func NewIdempotentUoW(base UnitOfWork, store IdempotencyStore) IdempotentUoW {
	return &idempotentUoW{
		UnitOfWork: base,
		store:      store,
	}
}

func (i *idempotentUoW) DoIdempotent(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func(ctx context.Context, tx database.DBExecutor) error,
) error {
	// Verificar se já foi executado
	result, exists, err := i.store.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check idempotency: %w", err)
	}

	if exists {
		// Operação já foi executada, retornar resultado armazenado
		if result != nil {
			return result.(error)
		}
		return nil
	}

	// Executar operação
	err = i.UnitOfWork.Do(ctx, fn)

	// Armazenar resultado
	if storeErr := i.store.Set(ctx, key, err, ttl); storeErr != nil {
		// Log do erro mas não retornar (operação já foi executada)
		fmt.Printf("Warning: failed to store idempotency result: %v\n", storeErr)
	}

	return err
}

// ==============================================================================
// EXEMPLO COMPLETO DE PRODUÇÃO
// ==============================================================================

// ProductionUoWModule módulo FX completo para produção
var ProductionUoWModule = fx.Module(
	"production-uow",
	fx.Provide(
		// Database connections
		NewPrimaryDB,
		NewReplicaDB,

		// Base UoW
		NewPrimaryUoW,
		NewReplicaUoW,

		// Enhanced UoW com todas as features
		func(base PrimaryUoW, metrics Metrics) EnhancedUoW {
			return NewEnhancedUoW(
				UnitOfWork(base),
				RetryConfig{
					MaxAttempts:  3,
					InitialDelay: 100 * time.Millisecond,
					MaxDelay:     1 * time.Second,
					Multiplier:   2.0,
				},
				CircuitBreakerConfig{
					MaxFailures:  5,
					ResetTimeout: 30 * time.Second,
				},
				metrics,
			)
		},
	),
)
