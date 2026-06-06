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

type Manager interface {
	Driver() database.Driver

	DBTX(ctx context.Context) database.DBTX

	BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error)
	Ping(ctx context.Context) error

	Shutdown(ctx context.Context) error
}

type DriverConfig interface {
	Validate() error
}

type Option func(*options)

type dbManager struct {
	adapter     driverAdapter
	opts        options
	mu          sync.RWMutex
	closed      bool
	shutdown    sync.Once
	shutdownErr error
	logger      *slog.Logger

	activeTx sync.WaitGroup
	scraper  *internalpool.Scraper
	inst     instrumentation
	poolDBTX database.DBTX
}

var closedDBTXSingleton database.DBTX = &closedDBTX{}

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
	mgr.poolDBTX = mgr.inst.WrapDBTX(adapter.DBTX())
	if !isNoopObservability(o.observability) {
		mgr.scraper = internalpool.NewScraper(adapter.Stats, o.observability.Metrics(), resolvePoolStatsInterval(o), adapter.Attributes()...)
	}
	return mgr, nil
}

func buildAdapter(cfg DriverConfig, o options) (driverAdapter, error) {
	if factory, ok := lookupDriverFactory(cfg); ok {
		adapter, err := factory(cfg, nil)
		if err != nil {
			return nil, err
		}
		return &externalAdapter{DriverAdapter: adapter}, nil
	}
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
		return nil, unsupportedDriverError(cfg)
	}
}

func NewFromAdapter(adapter DriverAdapter, opts ...Option) (Manager, error) {
	if adapter == nil {
		return nil, fmt.Errorf("%w: adapter is nil", database.ErrInvalidConfig)
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	internalAdapter := &externalAdapter{DriverAdapter: adapter}
	fallbackLogger := resolveLogger(o)
	mgr := &dbManager{
		adapter: internalAdapter,
		opts:    o,
		logger:  fallbackLogger,
		inst:    newInstrumentation(adapter.Driver(), adapter.Attributes(), o.observability, fallbackLogger, o.sqlLogging),
	}
	mgr.poolDBTX = mgr.inst.WrapDBTX(adapter.DBTX())
	if !isNoopObservability(o.observability) {
		mgr.scraper = internalpool.NewScraper(adapter.Stats, o.observability.Metrics(), resolvePoolStatsInterval(o), adapter.Attributes()...)
	}
	return mgr, nil
}

type externalAdapter struct {
	DriverAdapter
}

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

func (m *dbManager) DBTX(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return closedDBTXSingleton
	}
	m.mu.RUnlock()
	return m.poolDBTX
}

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

type closedRow struct{}

func (r *closedRow) Scan(_ ...any) error { return database.ErrManagerClosed }
