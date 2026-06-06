package manager

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"
)

// mockAdapter é um dublê de teste para o driverAdapter.
type mockAdapter struct {
	driver     database.Driver
	dbtx       database.DBTX
	tx         database.Tx
	beginErr   error
	lastTxOpts database.TxOptions
	beginCalls int
	pingErr    error
	closeErr   error
	closeSlow  time.Duration // se > 0, Close bloqueia este tempo antes de retornar
	closeCalls int
	stats      internalpool.Stats
	statsCalls atomic.Int64
}

func (a *mockAdapter) Driver() database.Driver { return a.driver }
func (a *mockAdapter) DBTX() database.DBTX     { return a.dbtx }
func (a *mockAdapter) BeginTx(_ context.Context, opts database.TxOptions) (database.Tx, error) {
	a.beginCalls++
	a.lastTxOpts = opts
	if a.beginErr != nil {
		return nil, a.beginErr
	}
	if a.tx != nil {
		return a.tx, nil
	}
	return &stubTx{}, nil
}
func (a *mockAdapter) Ping(_ context.Context) error { return a.pingErr }
func (a *mockAdapter) Stats() internalpool.Stats {
	a.statsCalls.Add(1)
	return a.stats
}
func (a *mockAdapter) Attributes() []observability.Field {
	return []observability.Field{
		observability.String("db.system", string(a.driver)),
	}
}
func (a *mockAdapter) Close(_ context.Context) error {
	a.closeCalls++
	if a.closeSlow > 0 {
		time.Sleep(a.closeSlow)
	}
	return a.closeErr
}

// stubDBTX é um DBTX minimalista para fiação no contexto.
type stubDBTX struct{}

func (s *stubDBTX) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, errors.New("not implemented")
}
func (s *stubDBTX) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, errors.New("not implemented")
}
func (s *stubDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return nil
}

type stubTx struct{}

func (s *stubTx) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}
func (s *stubTx) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}
func (s *stubTx) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return nil
}
func (s *stubTx) Commit(_ context.Context) error   { return nil }
func (s *stubTx) Rollback(_ context.Context) error { return nil }

// newTestManager constrói um *dbManager diretamente, ignorando o New (sem banco de dados real).
func newTestManager(adapter driverAdapter, opts ...Option) *dbManager {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	m := &dbManager{
		adapter: adapter,
		opts:    o,
		logger:  resolveLogger(o),
		inst:    newInstrumentation(adapter.Driver(), adapter.Attributes(), o.observability, resolveLogger(o), o.sqlLogging),
	}
	m.poolDBTX = m.inst.WrapDBTX(adapter.DBTX())
	return m
}

// --- Testes da Factory ---

func TestNew_NilConfig_ReturnsInvalidConfig(t *testing.T) {
	_, err := New(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, database.ErrInvalidConfig)
}

func TestNew_NilConfig_UsesEnvironmentDriverConfig(t *testing.T) {
	originalBuildAdapterFunc := buildAdapterFunc
	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	t.Cleanup(func() {
		buildAdapterFunc = originalBuildAdapterFunc
		runStartupMigrationsFunc = originalRunStartupMigrationsFunc
	})

	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_DSN", "postgres://env-user:env-pass@localhost/envdb?sslmode=disable")

	var gotCfg DriverConfig
	buildAdapterFunc = func(cfg DriverConfig, _ options) (driverAdapter, error) {
		gotCfg = cfg
		return &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}}, nil
	}
	runStartupMigrationsFunc = func(_ DriverConfig, _ database.Driver, _ options) error { return nil }

	mgr, err := New(nil)
	require.NoError(t, err)
	require.NotNil(t, mgr)

	pgCfg, ok := gotCfg.(postgres.PostgresConfig)
	require.True(t, ok)
	require.Equal(t, "postgres://env-user:env-pass@localhost/envdb?sslmode=disable", pgCfg.DSN)
}

func TestNew_TypedConfigTakesPrecedenceOverEnvironmentDriver(t *testing.T) {
	originalBuildAdapterFunc := buildAdapterFunc
	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	t.Cleanup(func() {
		buildAdapterFunc = originalBuildAdapterFunc
		runStartupMigrationsFunc = originalRunStartupMigrationsFunc
	})

	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("DB_DSN", "ignored-by-typed-config")

	var gotCfg DriverConfig
	buildAdapterFunc = func(cfg DriverConfig, _ options) (driverAdapter, error) {
		gotCfg = cfg
		return &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}}, nil
	}
	runStartupMigrationsFunc = func(_ DriverConfig, _ database.Driver, _ options) error { return nil }

	explicit := postgres.PostgresConfig{DSN: "postgres://typed:typed@localhost/app?sslmode=disable"}
	mgr, err := New(explicit)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.Equal(t, explicit, gotCfg)
}

func TestNew_TypedConfig_MergesEnvironmentDefaults(t *testing.T) {
	originalBuildAdapterFunc := buildAdapterFunc
	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	t.Cleanup(func() {
		buildAdapterFunc = originalBuildAdapterFunc
		runStartupMigrationsFunc = originalRunStartupMigrationsFunc
	})

	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("DB_DSN", "postgres://env-should-not-win@localhost/envdb?sslmode=disable")
	t.Setenv("DB_HOST", "env-host")
	t.Setenv("DB_PORT", "5439")
	t.Setenv("DB_USER", "env-user")
	t.Setenv("DB_PASSWORD", "env-pass")
	t.Setenv("DB_DATABASE", "env-db")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("DB_SEARCH_PATH", "tenant_env")

	var gotCfg DriverConfig
	buildAdapterFunc = func(cfg DriverConfig, _ options) (driverAdapter, error) {
		gotCfg = cfg
		return &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}}, nil
	}
	runStartupMigrationsFunc = func(_ DriverConfig, _ database.Driver, _ options) error { return nil }

	explicit := postgres.PostgresConfig{
		Host: "typed-host",
	}

	mgr, err := New(explicit)
	require.NoError(t, err)
	require.NotNil(t, mgr)

	require.Equal(t, postgres.PostgresConfig{
		Host:       "typed-host",
		Port:       5439,
		User:       "env-user",
		Password:   "env-pass",
		Database:   "env-db",
		SSLMode:    "require",
		SearchPath: "tenant_env",
	}, gotCfg)
}

func TestNew_TypedConfig_IgnoresInvalidEnvironmentDefaultsWhenStructAlreadyDefinesDriverConfig(t *testing.T) {
	originalBuildAdapterFunc := buildAdapterFunc
	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	t.Cleanup(func() {
		buildAdapterFunc = originalBuildAdapterFunc
		runStartupMigrationsFunc = originalRunStartupMigrationsFunc
	})

	t.Setenv("DB_DRIVER", "not-a-real-driver")
	t.Setenv("DB_PORT", "not-a-number")
	t.Setenv("DB_DSN", "postgres://env-should-not-win@localhost/envdb?sslmode=disable")

	var gotCfg DriverConfig
	buildAdapterFunc = func(cfg DriverConfig, _ options) (driverAdapter, error) {
		gotCfg = cfg
		return &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}}, nil
	}
	runStartupMigrationsFunc = func(_ DriverConfig, _ database.Driver, _ options) error { return nil }

	explicit := postgres.PostgresConfig{
		DSN: "postgres://typed:typed@localhost/app?sslmode=disable",
	}

	mgr, err := New(explicit)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.Equal(t, explicit, gotCfg)
}

func TestNew_UnsupportedDriverConfig_ReturnsInvalidConfig(t *testing.T) {
	cfg := &unsupportedConfig{}
	_, err := New(cfg)
	require.ErrorIs(t, err, database.ErrInvalidConfig)
}

type unsupportedConfig struct{}

func (u *unsupportedConfig) Validate() error { return nil }

func TestNew_RunsStartupMigrationsBeforeReturning(t *testing.T) {
	originalBuildAdapterFunc := buildAdapterFunc
	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	t.Cleanup(func() {
		buildAdapterFunc = originalBuildAdapterFunc
		runStartupMigrationsFunc = originalRunStartupMigrationsFunc
	})

	var called bool
	buildAdapterFunc = func(_ DriverConfig, _ options) (driverAdapter, error) {
		return &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}}, nil
	}
	runStartupMigrationsFunc = func(cfg DriverConfig, driver database.Driver, _ options) error {
		called = true
		require.IsType(t, postgres.PostgresConfig{}, cfg)
		require.Equal(t, database.DriverPostgres, driver)
		return nil
	}

	mgr, err := New(postgres.PostgresConfig{DSN: "postgres://user:pass@localhost/testdb?sslmode=disable"})
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.True(t, called, "manager.New deve executar a etapa síncrona de startup migration antes de retornar")
}

func TestNew_StartupMigrationFailure_ClosesAdapterAndReturnsError(t *testing.T) {
	originalBuildAdapterFunc := buildAdapterFunc
	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	t.Cleanup(func() {
		buildAdapterFunc = originalBuildAdapterFunc
		runStartupMigrationsFunc = originalRunStartupMigrationsFunc
	})

	adapter := &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}}
	buildAdapterFunc = func(_ DriverConfig, _ options) (driverAdapter, error) {
		return adapter, nil
	}

	startupErr := fmt.Errorf("%w: startup failed", database.ErrMigrationFailed)
	runStartupMigrationsFunc = func(_ DriverConfig, _ database.Driver, _ options) error {
		return startupErr
	}

	mgr, err := New(postgres.PostgresConfig{DSN: "postgres://user:pass@localhost/testdb?sslmode=disable"})
	require.Nil(t, mgr)
	require.ErrorIs(t, err, database.ErrMigrationFailed)
	require.Equal(t, 1, adapter.closeCalls, "o adapter deve ser fechado quando a startup migration falha")
}

// --- Testes de precedência do DBTX ---

func TestDBTX_TxInContext_ReturnsTx(t *testing.T) {
	tx := &stubDBTX{}
	pool := &stubDBTX{}

	adapter := &mockAdapter{dbtx: pool}
	m := newTestManager(adapter)

	ctx := database.WithTx(context.Background(), tx)
	got := m.DBTX(ctx)

	require.Equal(t, tx, got, "esperava que a tx do contexto fosse retornada")
}

func TestDBTX_NoTxInContext_ReturnsPool(t *testing.T) {
	pool := &stubDBTX{}
	adapter := &mockAdapter{dbtx: pool}
	m := newTestManager(adapter)

	got := m.DBTX(context.Background())

	require.NotNil(t, got, "esperava um DBTX utilizável quando o contexto não tem tx")
}

func TestDBTX_AfterShutdown_ReturnsClosedDBTX(t *testing.T) {
	adapter := &mockAdapter{dbtx: &stubDBTX{}}
	m := newTestManager(adapter)

	ctx := context.Background()
	require.NoError(t, m.Shutdown(ctx))

	got := m.DBTX(ctx)

	_, execErr := got.ExecContext(ctx, "SELECT 1")
	require.ErrorIs(t, execErr, database.ErrManagerClosed)
}

func TestDBTX_AfterShutdown_ContextTxStillHonoured(t *testing.T) {
	tx := &stubDBTX{}
	adapter := &mockAdapter{dbtx: &stubDBTX{}}
	m := newTestManager(adapter)

	require.NoError(t, m.Shutdown(context.Background()))

	ctx := database.WithTx(context.Background(), tx)
	got := m.DBTX(ctx)

	require.Equal(t, tx, got, "a tx ativa no ctx deve ser retornada mesmo após o Shutdown")
}

// --- Driver e Ping ---

func TestDriver_ReturnsAdapterDriver(t *testing.T) {
	adapter := &mockAdapter{driver: database.DriverPostgres}
	m := newTestManager(adapter)
	require.Equal(t, database.DriverPostgres, m.Driver())
}

func TestPing_PropagatesAdapterError(t *testing.T) {
	pingErr := errors.New("connection refused")
	adapter := &mockAdapter{pingErr: pingErr}
	m := newTestManager(adapter)

	err := m.Ping(context.Background())
	require.ErrorIs(t, err, pingErr)
}

func TestPing_AfterShutdown_ReturnsErrManagerClosed(t *testing.T) {
	adapter := &mockAdapter{dbtx: &stubDBTX{}}
	m := newTestManager(adapter)

	require.NoError(t, m.Shutdown(context.Background()))

	err := m.Ping(context.Background())
	require.ErrorIs(t, err, database.ErrManagerClosed)
}

func TestBeginTx_ManagerReadOnlyForcesReadOnly(t *testing.T) {
	adapter := &mockAdapter{
		dbtx: &stubDBTX{},
		tx:   &stubTx{},
	}
	mgr := newTestManager(adapter, WithReadOnly(true))

	_, err := mgr.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, adapter.beginCalls)
	require.True(t, adapter.lastTxOpts.ReadOnly, "WithReadOnly(true) no manager deve forçar transações read-only")
}

type stubResult struct {
	rows int64
}

func (r stubResult) RowsAffected() (int64, error) { return r.rows, nil }

func TestDBTX_ExecContext_EmitsMetricsSpanAndSanitizedSQLLog(t *testing.T) {
	obs := fake.NewProvider()
	dbtx := &execRecordingDBTX{}
	adapter := &mockAdapter{
		driver: database.DriverPostgres,
		dbtx:   dbtx,
	}
	mgr := newTestManager(adapter, WithObservability(obs), WithSQLLogging(true))

	_, err := mgr.DBTX(context.Background()).ExecContext(context.Background(), "SELECT * FROM users WHERE secret = $1", "super-secret")
	require.NoError(t, err)

	hist := obs.Metrics().(*fake.FakeMetrics).GetHistogram("database.query.duration_ms")
	require.NotNil(t, hist)
	require.NotEmpty(t, hist.GetValues())

	spans := obs.Tracer().(*fake.FakeTracer).GetSpans()
	require.NotEmpty(t, spans)
	require.Equal(t, "db.postgres.exec", spans[0].Name)

	entries := obs.Logger().(*fake.FakeLogger).GetEntries()
	require.Len(t, entries, 1)
	require.Equal(t, observability.LogLevelDebug, entries[0].Level)
	for _, field := range entries[0].Fields {
		if field.Key == "args" {
			require.Equal(t, []string{"?"}, field.AnyValue())
		}
	}
}

func TestBeginTx_CommitEmitsSpanAndMetric(t *testing.T) {
	// Regressão: Commit/Rollback do Tx instrumentado devem abrir span
	// e registrar métrica de duração (R-O11Y-001).
	obs := fake.NewProvider()
	adapter := &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}, tx: &stubTx{}}
	mgr := newTestManager(adapter, WithObservability(obs))

	tx, err := mgr.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(context.Background()))

	spans := obs.Tracer().(*fake.FakeTracer).GetSpans()
	var commitSpan bool
	for _, s := range spans {
		if s.Name == "db.postgres.commit" {
			commitSpan = true
		}
	}
	require.True(t, commitSpan, "Commit deve emitir span db.postgres.commit")

	hist := obs.Metrics().(*fake.FakeMetrics).GetHistogram("database.query.duration_ms")
	require.NotNil(t, hist)
	require.NotEmpty(t, hist.GetValues(), "Commit deve registrar métrica de duração")
}

func TestBeginTx_RollbackEmitsSpanAndMetric(t *testing.T) {
	obs := fake.NewProvider()
	adapter := &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}, tx: &stubTx{}}
	mgr := newTestManager(adapter, WithObservability(obs))

	tx, err := mgr.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(context.Background()))

	spans := obs.Tracer().(*fake.FakeTracer).GetSpans()
	var rbSpan bool
	for _, s := range spans {
		if s.Name == "db.postgres.rollback" {
			rbSpan = true
		}
	}
	require.True(t, rbSpan, "Rollback deve emitir span db.postgres.rollback")
}

func TestNew_WithPoolStatsInterval_StartsManagerOwnedScraper(t *testing.T) {
	originalBuildAdapterFunc := buildAdapterFunc
	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	t.Cleanup(func() {
		buildAdapterFunc = originalBuildAdapterFunc
		runStartupMigrationsFunc = originalRunStartupMigrationsFunc
	})

	adapter := &mockAdapter{
		driver: database.DriverPostgres,
		dbtx:   &stubDBTX{},
		stats:  internalpool.Stats{OpenConnections: 3, Idle: 1},
	}
	buildAdapterFunc = func(_ DriverConfig, _ options) (driverAdapter, error) {
		return adapter, nil
	}
	runStartupMigrationsFunc = func(_ DriverConfig, _ database.Driver, _ options) error { return nil }

	obs := fake.NewProvider()
	mgr, err := New(
		postgres.PostgresConfig{DSN: "postgres://user:pass@localhost/testdb?sslmode=disable"},
		WithObservability(obs),
		WithPoolStatsInterval(10*time.Millisecond),
	)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	require.Eventually(t, func() bool {
		return adapter.statsCalls.Load() > 0
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func TestResolveMigrationDSN_PostgresStructuredConfig_ReturnsURL(t *testing.T) {
	dsn, err := resolveMigrationDSN(postgres.PostgresConfig{
		Host:       "localhost",
		Port:       5432,
		User:       "app",
		Password:   "secret",
		Database:   "devkit",
		SSLMode:    "require",
		SearchPath: "tenant_a",
	})

	require.NoError(t, err)
	require.Equal(t, "pgx5://app:secret@localhost:5432/devkit?search_path=tenant_a&sslmode=require", dsn)
}

// --- closedDBTX ---

func TestClosedDBTX_AllOpsReturnManagerClosed(t *testing.T) {
	c := &closedDBTX{}
	ctx := context.Background()

	_, execErr := c.ExecContext(ctx, "SELECT 1")
	require.ErrorIs(t, execErr, database.ErrManagerClosed)

	_, queryErr := c.QueryContext(ctx, "SELECT 1")
	require.ErrorIs(t, queryErr, database.ErrManagerClosed)

	scanErr := c.QueryRowContext(ctx, "SELECT 1").Scan()
	require.ErrorIs(t, scanErr, database.ErrManagerClosed)
}

type execRecordingDBTX struct{}

func (d *execRecordingDBTX) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return stubResult{rows: 1}, nil
}

func (d *execRecordingDBTX) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}

func (d *execRecordingDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return nil
}
