// O pacote manager expõe a interface Manager e sua Factory para o gerenciamento
// do ciclo de vida dos pools de conexões de banco de dados. Os consumidores dependem apenas do Manager;
// os adaptadores específicos dos drivers são resolvidos internamente pelo New.
package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/cockroach"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mssql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mysql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

var (
	buildAdapterFunc         = buildAdapter
	runStartupMigrationsFunc = runStartupMigrations
)

// Manager é a interface pública para gerenciar um pool de conexões de banco de dados.
type Manager interface {
	Driver() database.Driver
	// DBTX retorna a transação ativa propagada via ctx quando existir,
	// ou um DBTX baseado no pool caso contrário (Q61 / ADR-004).
	DBTX(ctx context.Context) database.DBTX
	// BeginTx inicia uma nova transação com as opções fornecidas.
	// O Tx retornado satisfaz database.DBTX e expõe Commit e Rollback.
	BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error)
	Ping(ctx context.Context) error
	// Shutdown fecha o pool graciosamente. É idempotente (sync.Once).
	// Retorna ErrShutdownTimeout se o ctx expirar antes do Close completar (RF-09).
	Shutdown(ctx context.Context) error
}

// DriverConfig é a interface de marcação para configurações de driver tipadas.
// A Factory impõe o conjunto suportado via um type switch; tipos desconhecidos
// recebem ErrInvalidConfig sem exigir um método não exportado.
type DriverConfig interface {
	Validate() error
}

// Option configura o manager.
type Option func(*options)

// dbManager é a implementação concreta de Manager.
type dbManager struct {
	adapter     driverAdapter
	opts        options
	mu          sync.RWMutex
	closed      bool
	shutdown    sync.Once
	shutdownErr error
	logger      *slog.Logger
	// activeTx rastreia transações em voo iniciadas via BeginTx.
	// Shutdown aguarda o drain completo antes de invocar adapter.Close (RF-04).
	activeTx sync.WaitGroup
	scraper  *internalpool.Scraper
	inst     instrumentation
}

// New cria um Manager para a configuração de driver fornecida.
// Valida a configuração, constrói o adaptador (que inclui o Ping inicial de 5s
// por Q48) e aplica todas as opções fornecidas.
func New(cfg DriverConfig, opts ...Option) (Manager, error) {
	resolvedCfg, err := resolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := resolvedCfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", database.ErrInvalidConfig, err)
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	adapter, err := buildAdapterFunc(resolvedCfg, o)
	if err != nil {
		return nil, err
	}
	if err := runStartupMigrationsFunc(resolvedCfg, adapter.Driver(), o); err != nil {
		_ = adapter.Close(context.Background())
		return nil, err
	}

	fallbackLogger := resolveLogger(o)
	mgr := &dbManager{
		adapter: adapter,
		opts:    o,
		logger:  fallbackLogger,
		inst:    newInstrumentation(adapter.Driver(), adapter.Attributes(), o.observability, fallbackLogger, o.sqlLogging),
	}
	if !isNoopObservability(o.observability) {
		mgr.scraper = internalpool.NewScraper(adapter.Stats, o.observability.Metrics(), resolvePoolStatsInterval(o), adapter.Attributes()...)
	}
	return mgr, nil
}

// buildAdapter instancia o adaptador de driver concreto para cfg.
// Drivers ainda não implementados retornam ErrInvalidConfig com uma mensagem descritiva.
func buildAdapter(cfg DriverConfig, o options) (driverAdapter, error) {
	switch c := cfg.(type) {
	case postgres.PostgresConfig:
		return postgres.New(c, nil)
	case cockroach.CockroachConfig:
		return cockroach.New(c, nil)
	case mysql.MySQLConfig:
		return mysql.New(c, nil)
	case mssql.MSSQLConfig:
		return mssql.New(c, nil)
	default:
		return nil, fmt.Errorf("%w: unsupported driver config type %T", database.ErrInvalidConfig, cfg)
	}
}

// resolveLogger retorna slog.Default() quando o log de SQL está habilitado,
// ou nil quando desabilitado. O adaptador usa este logger para saída de depuração de consulta.
// Quando a observabilidade é noop, o slog.Default() fornece saída estruturada (Q56).
func resolveLogger(o options) *slog.Logger {
	if !o.sqlLogging || !isNoopObservability(o.observability) {
		return nil
	}
	return slog.Default()
}

func resolvePoolStatsInterval(o options) time.Duration {
	if o.poolStatsInterval > 0 {
		return o.poolStatsInterval
	}
	return internalpool.DefaultScrapeInterval
}

func (m *dbManager) Driver() database.Driver {
	return m.adapter.Driver()
}

// DBTX respeita a transação ativa no ctx (propagação implícita ADR-004).
// Quando o ctx não carrega transação, retorna um DBTX baseado no pool.
// Após o Shutdown, retorna um closedDBTX que reporta ErrManagerClosed em cada operação.
func (m *dbManager) DBTX(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return &closedDBTX{}
	}
	m.mu.RUnlock()
	return m.inst.WrapDBTX(m.adapter.DBTX())
}

// BeginTx inicia uma nova transação com as opções fornecidas.
// Retorna ErrManagerClosed após o Shutdown.
//
// O Add no WaitGroup ocorre dentro do RLock para sequenciá-lo com o Shutdown
// (que adquire o Lock antes de Wait). Sem isso, há janela em que Shutdown
// completa Wait com contador zero e BeginTx posteriormente faz Add — gerando
// panic "WaitGroup is reused before previous Wait has returned" ou tx órfã
// sobre pool já fechado.
func (m *dbManager) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, database.ErrManagerClosed
	}
	m.activeTx.Add(1)
	m.mu.RUnlock()

	effectiveOpts := opts
	if m.opts.readOnly {
		effectiveOpts.ReadOnly = true
	}

	tx, err := m.adapter.BeginTx(ctx, effectiveOpts)
	if err != nil {
		m.activeTx.Done()
		return nil, err
	}
	return &trackedTx{Tx: m.inst.WrapTx(tx), wg: &m.activeTx}, nil
}

// trackedTx envolve uma database.Tx para sinalizar drain ao WaitGroup do Manager
// na primeira chamada de Commit ou Rollback (RF-04).
type trackedTx struct {
	database.Tx
	wg   *sync.WaitGroup
	done sync.Once
}

func (t *trackedTx) Commit(ctx context.Context) error {
	err := t.Tx.Commit(ctx)
	t.done.Do(t.wg.Done)
	return err
}

func (t *trackedTx) Rollback(ctx context.Context) error {
	err := t.Tx.Rollback(ctx)
	t.done.Do(t.wg.Done)
	return err
}

func (m *dbManager) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return database.ErrManagerClosed
	}
	return m.adapter.Ping(ctx)
}

// Shutdown fecha o pool graciosamente. Idempotente via sync.Once.
// Se o ctx expirar antes do adapter.Close retornar, o Shutdown retorna ErrShutdownTimeout
// sem forçar o pool.Close (RF-09); a limpeza é deixada para a terminação do processo.
func (m *dbManager) Shutdown(ctx context.Context) error {
	m.shutdown.Do(func() {
		m.mu.Lock()
		m.closed = true
		m.mu.Unlock()

		shutdownCtx := ctx
		cancel := func() {}
		if m.opts.shutdownTimeout > 0 {
			shutdownCtx, cancel = context.WithTimeout(ctx, m.opts.shutdownTimeout)
		}
		defer cancel()

		// Aguarda o drain das transações em voo (RF-04). Se o ctx expirar
		// antes da drenagem, retorna ErrShutdownTimeout SEM invocar
		// adapter.Close (RF-09).
		drained := make(chan struct{})
		go func() {
			m.activeTx.Wait()
			close(drained)
		}()

		select {
		case <-shutdownCtx.Done():
			m.shutdownErr = database.ErrShutdownTimeout
			return
		case <-drained:
		}

		if m.scraper != nil {
			m.scraper.Stop()
		}

		// Após o drain, fecha o pool respeitando o tempo restante do contexto.
		done := make(chan error, 1)
		go func() {
			done <- m.adapter.Close(shutdownCtx)
		}()

		select {
		case <-shutdownCtx.Done():
			m.shutdownErr = database.ErrShutdownTimeout
		case err := <-done:
			m.shutdownErr = err
		}
	})
	return m.shutdownErr
}

// closedDBTX é retornado pelo DBTX após o Shutdown; cada operação retorna ErrManagerClosed.
type closedDBTX struct{}

func (c *closedDBTX) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, database.ErrManagerClosed
}

func (c *closedDBTX) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, database.ErrManagerClosed
}

func (c *closedDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return &closedRow{}
}

// closedRow retorna ErrManagerClosed no Scan.
type closedRow struct{}

func (r *closedRow) Scan(_ ...any) error { return database.ErrManagerClosed }
