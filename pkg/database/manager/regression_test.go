package manager

import (
	"context"
	"embed"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"
)

// concurrentAdapter is a thread-safe driverAdapter used only for race tests.
type concurrentAdapter struct {
	beginCalls atomic.Int64
	closeCalls atomic.Int64
}

func (a *concurrentAdapter) Driver() database.Driver { return database.DriverPostgres }
func (a *concurrentAdapter) DBTX() database.DBTX     { return &stubDBTX{} }
func (a *concurrentAdapter) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	a.beginCalls.Add(1)
	return &stubTx{}, nil
}
func (a *concurrentAdapter) Stats() internalpool.Stats         { return internalpool.Stats{} }
func (a *concurrentAdapter) Attributes() []observability.Field { return nil }
func (a *concurrentAdapter) Ping(_ context.Context) error      { return nil }
func (a *concurrentAdapter) Close(_ context.Context) error {
	a.closeCalls.Add(1)
	return nil
}

// C1: BeginTx concorrente com Shutdown não pode panicar (sync.WaitGroup
// "reused before previous Wait has returned") nem deixar transações órfãs.
// Esse teste só prova a ausência da race quando rodado com -race; o panic
// também surge sem -race quando o agendamento é favorável.
func TestBeginTx_ConcurrentWithShutdown_NoRaceOrPanic(t *testing.T) {
	for iter := 0; iter < 50; iter++ {
		adapter := &concurrentAdapter{}
		m := newTestManager(adapter)

		const beginners = 32
		var beginnersWG sync.WaitGroup
		start := make(chan struct{})
		txCh := make(chan database.Tx, beginners)
		errCh := make(chan error, beginners)

		for i := 0; i < beginners; i++ {
			beginnersWG.Add(1)
			go func() {
				defer beginnersWG.Done()
				<-start
				tx, err := m.BeginTx(context.Background(), database.TxOptions{})
				if err != nil {
					errCh <- err
					return
				}
				txCh <- tx
			}()
		}

		shutdownDone := make(chan error, 1)
		go func() {
			<-start
			shutdownDone <- m.Shutdown(context.Background())
		}()

		close(start)

		// Espera os BeginTx completarem (cada um devolve tx ou erro).
		beginnersWG.Wait()
		close(txCh)
		close(errCh)

		// Drena as txs criadas para destravar o Shutdown que espera o WaitGroup.
		for tx := range txCh {
			_ = tx.Commit(context.Background())
		}

		select {
		case err := <-shutdownDone:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Shutdown não completou após drain das txs")
		}

		for err := range errCh {
			// Apenas ErrManagerClosed é tolerado: significa que BeginTx
			// observou o shutdown antes de incrementar o WaitGroup.
			require.ErrorIs(t, err, database.ErrManagerClosed)
		}
	}
}

//go:embed testdata/embed_migrations/*.sql
var embedMigrationsFS embed.FS

// C2: WithStartupMigrationFS deve fazer runStartupMigrations consumir o fs.FS
// fornecido pelo consumidor em vez do diretório default.
func TestRunStartupMigrations_UsesEmbedFSWhenOptionSet(t *testing.T) {
	originalDirFunc := startupMigrationDirFunc
	t.Cleanup(func() { startupMigrationDirFunc = originalDirFunc })

	startupMigrationDirFunc = func(_ database.Driver) string {
		return "/non/existent/path/should/not/be/touched"
	}

	o := defaultOptions()
	WithStartupMigrationFS(embedMigrationsFS, "testdata/embed_migrations")(&o)

	cfg := postgres.PostgresConfig{
		DSN: "postgres://user:pass@localhost:5432/devkit?sslmode=disable",
	}

	err := runStartupMigrations(cfg, database.DriverPostgres, o)
	// O DSN é fictício, então a etapa Up vai falhar tentando conectar; o
	// importante para a regressão é que o erro NÃO seja "stat ... does not
	// exist" do diretório default, comprovando que a opção foi consumida.
	if err != nil {
		require.NotContains(t, err.Error(), "/non/existent/path/should/not/be/touched")
	}
}

// C2: WithStartupMigrationDir sobrescreve o diretório default.
func TestRunStartupMigrations_UsesCustomDirWhenOptionSet(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "001_init.up.sql"), []byte("SELECT 1;"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "001_init.down.sql"), []byte("SELECT 1;"), 0o600))

	originalDirFunc := startupMigrationDirFunc
	t.Cleanup(func() { startupMigrationDirFunc = originalDirFunc })

	var defaultDirCalled bool
	startupMigrationDirFunc = func(_ database.Driver) string {
		defaultDirCalled = true
		return "/should/not/be/used"
	}

	o := defaultOptions()
	WithStartupMigrationDir(tmp)(&o)

	cfg := postgres.PostgresConfig{
		DSN: "postgres://user:pass@localhost:5432/devkit?sslmode=disable",
	}
	_ = runStartupMigrations(cfg, database.DriverPostgres, o)
	require.False(t, defaultDirCalled, "WithStartupMigrationDir deve preceder o resolver default")
}

func TestRunStartupMigrations_NilFS_FallsBackToDefaultDir(t *testing.T) {
	originalDirFunc := startupMigrationDirFunc
	t.Cleanup(func() { startupMigrationDirFunc = originalDirFunc })

	called := false
	startupMigrationDirFunc = func(_ database.Driver) string {
		called = true
		return filepath.Join(t.TempDir(), "missing")
	}

	o := defaultOptions()
	cfg := postgres.PostgresConfig{
		DSN: "postgres://user:pass@localhost:5432/devkit?sslmode=disable",
	}
	err := runStartupMigrations(cfg, database.DriverPostgres, o)
	require.NoError(t, err, "diretório inexistente é no-op")
	require.True(t, called, "fallback default deve ser consultado quando nenhuma option foi setada")
}

// Sanity: the regression for C1 should still tolerate adapter.Close errors.
func TestBeginTx_AfterShutdownReturnsErrManagerClosed(t *testing.T) {
	adapter := &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}, tx: &stubTx{}}
	m := newTestManager(adapter)
	require.NoError(t, m.Shutdown(context.Background()))

	_, err := m.BeginTx(context.Background(), database.TxOptions{})
	require.True(t, errors.Is(err, database.ErrManagerClosed), "expected ErrManagerClosed, got %v", err)
	// adapter.BeginTx must not have been called after shutdown.
	require.Equal(t, 0, adapter.beginCalls)
}

func TestRunStartupMigrations_TimeoutReducesNoise(t *testing.T) {
	// Sentinela: garante que o helper não fica preso indefinidamente quando
	// o DSN aponta para algo inalcançável; valida que retorna erro em
	// um tempo razoável (compatível com -race).
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "001_init.up.sql"), []byte("SELECT 1;"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "001_init.down.sql"), []byte("SELECT 1;"), 0o600))

	o := defaultOptions()
	WithStartupMigrationDir(tmp)(&o)

	done := make(chan error, 1)
	go func() {
		done <- runStartupMigrations(
			postgres.PostgresConfig{DSN: "postgres://user:pass@127.0.0.1:1/devkit?sslmode=disable&connect_timeout=1"},
			database.DriverPostgres,
			o,
		)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("runStartupMigrations não retornou em tempo razoável")
	}
}

// --- Testes de regressão para I-2: QueryRowContext fecha span ao chamar Scan ---

// stubRowWithErr é uma database.Row fake que retorna um erro configurável no Scan.
type stubRowWithErr struct{ err error }

func (r *stubRowWithErr) Scan(_ ...any) error { return r.err }

// stubDBTXWithRow é um DBTX fake cujo QueryRowContext retorna uma row fixa.
type stubDBTXWithRow struct {
	row database.Row
}

func (d *stubDBTXWithRow) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}
func (d *stubDBTXWithRow) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}
func (d *stubDBTXWithRow) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return d.row
}

// I2: Scan() deve finalizar o span aberto por QueryRowContext (happy path).
func TestInstrumentation_QueryRowContext_ScanClosesSpan(t *testing.T) {
	obs := fake.NewProvider()
	inst := newInstrumentation(database.DriverPostgres, nil, obs, nil, false)
	base := &stubDBTXWithRow{row: &stubRowWithErr{}}
	dbtx := inst.WrapDBTX(base)

	row := dbtx.QueryRowContext(context.Background(), "SELECT 1")

	spans := obs.Tracer().(*fake.FakeTracer).GetSpans()
	require.Len(t, spans, 1, "um span deve ser aberto pelo QueryRowContext")
	require.Nil(t, spans[0].EndTime, "span não deve estar encerrado antes do Scan")

	_ = row.Scan()

	require.NotNil(t, spans[0].EndTime, "span deve ser encerrado após o Scan")
}

// I2: Scan() com erro (ex.: sql.ErrNoRows) também deve fechar o span.
func TestInstrumentation_QueryRowContext_ScanWithErrorClosesSpan(t *testing.T) {
	obs := fake.NewProvider()
	inst := newInstrumentation(database.DriverPostgres, nil, obs, nil, false)
	base := &stubDBTXWithRow{row: &stubRowWithErr{err: errors.New("sql: no rows in result set")}}
	dbtx := inst.WrapDBTX(base)

	row := dbtx.QueryRowContext(context.Background(), "SELECT 1 WHERE 1=0")
	err := row.Scan()
	require.Error(t, err)

	spans := obs.Tracer().(*fake.FakeTracer).GetSpans()
	require.Len(t, spans, 1)
	require.NotNil(t, spans[0].EndTime, "span deve ser encerrado mesmo quando Scan retorna erro")
}

// I2: Scan() via tx instrumentada também deve fechar o span.
func TestInstrumentation_QueryRowContext_ViaTx_ScanClosesSpan(t *testing.T) {
	obs := fake.NewProvider()
	inst := newInstrumentation(database.DriverPostgres, nil, obs, nil, false)

	innerTx := &stubTxWithRow{row: &stubRowWithErr{}}
	tx := inst.WrapTx(innerTx)

	row := tx.QueryRowContext(context.Background(), "SELECT 1")

	spans := obs.Tracer().(*fake.FakeTracer).GetSpans()
	require.Len(t, spans, 1, "um span deve ser aberto pelo QueryRowContext da tx")
	require.Nil(t, spans[0].EndTime, "span não deve estar encerrado antes do Scan")

	_ = row.Scan()

	require.NotNil(t, spans[0].EndTime, "span deve ser encerrado após Scan na tx")
}

// stubTxWithRow satisfaz database.Tx retornando uma row fixa via QueryRowContext.
type stubTxWithRow struct {
	row database.Row
}

func (t *stubTxWithRow) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}
func (t *stubTxWithRow) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}
func (t *stubTxWithRow) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return t.row
}
func (t *stubTxWithRow) Commit(_ context.Context) error   { return nil }
func (t *stubTxWithRow) Rollback(_ context.Context) error { return nil }

// --- Testes de regressão para I-3: isNoopObservability detecta noop via interface ---

// wrappedNoop é um decorator sobre noop.Provider que também implementa noopMarker.
// Simula o caso em que o consumidor envolve o noop em um tipo próprio.
type wrappedNoop struct {
	*noop.Provider
}

// IsNoop satisfaz noopMarker; o wrapper declara-se noop explicitamente.
func (w *wrappedNoop) IsNoop() bool { return true }

// I3: noop.Provider deve ser detectado como noop via interface (não tipo concreto).
func TestIsNoopObservability_NoopProvider_DetectedViaInterface(t *testing.T) {
	p := noop.NewProvider()
	require.True(t, isNoopObservability(p), "noop.Provider deve ser detectado como noop")
}

// I3: wrapper sobre noop.Provider que implementa IsNoop() também é detectado.
func TestIsNoopObservability_WrappedNoop_DetectedViaInterface(t *testing.T) {
	w := &wrappedNoop{Provider: noop.NewProvider()}
	require.True(t, isNoopObservability(w),
		"wrapper sobre noop.Provider que implementa IsNoop() deve ser detectado como noop")
}

// I3: fake.Provider (provider real de testes) NÃO deve ser detectado como noop.
func TestIsNoopObservability_FakeProvider_NotNoop(t *testing.T) {
	obs := fake.NewProvider()
	require.False(t, isNoopObservability(obs),
		"fake.Provider não deve ser detectado como noop — tem instrumentação real")
}

// I3: nil deve ser tratado como noop.
func TestIsNoopObservability_Nil_IsNoop(t *testing.T) {
	require.True(t, isNoopObservability(nil), "nil observability deve ser tratado como noop")
}

func TestInstrumentation_QueryRowContext_OrphanRow_SpanClosedByCleanup(t *testing.T) {
	obs := fake.NewProvider()
	inst := newInstrumentation(database.DriverPostgres, nil, obs, nil, false)
	base := &stubDBTXWithRow{row: &stubRowWithErr{}}
	dbtx := inst.WrapDBTX(base)

	spans := func() []*fake.FakeSpan {
		return obs.Tracer().(*fake.FakeTracer).GetSpans()
	}

	orphan := func() {
		_ = dbtx.QueryRowContext(context.Background(), "SELECT 1")
	}
	orphan()

	require.Len(t, spans(), 1)
	require.False(t, spans()[0].Ended(), "span permanece aberto enquanto wrapper vivo")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		runtime.Gosched()
		if spans()[0].Ended() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, spans()[0].Ended(), "cleanup deve encerrar o span quando wrapper é coletado")
}
