package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/require"
)

func TestShutdown_Success(t *testing.T) {
	adapter := &mockAdapter{dbtx: &stubDBTX{}}
	m := newTestManager(adapter)

	err := m.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestShutdown_Idempotent(t *testing.T) {
	adapter := &mockAdapter{dbtx: &stubDBTX{}}
	m := newTestManager(adapter)

	require.NoError(t, m.Shutdown(context.Background()))
	require.NoError(t, m.Shutdown(context.Background()), "o segundo Shutdown deve ser um no-op")
	require.Equal(t, 1, adapter.closeCalls, "adapter.Close deve ser chamado exatamente uma vez")
}

func TestShutdown_Timeout_ReturnsErrShutdownTimeout(t *testing.T) {
	// adapter.Close leva mais tempo do que o prazo do contexto.
	adapter := &mockAdapter{
		dbtx:      &stubDBTX{},
		closeSlow: 500 * time.Millisecond,
	}
	m := newTestManager(adapter)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := m.Shutdown(ctx)
	require.ErrorIs(t, err, database.ErrShutdownTimeout)
}

func TestShutdown_UsesConfiguredTimeout_WhenCallerContextHasNoDeadline(t *testing.T) {
	closeDone := make(chan struct{})
	slow := &slowCloseAdapter{
		stubDBTX: &stubDBTX{},
		done:     closeDone,
		delay:    200 * time.Millisecond,
	}
	m := newTestManager(slow, WithShutdownTimeout(20*time.Millisecond))

	start := time.Now()
	err := m.Shutdown(context.Background())
	elapsed := time.Since(start)

	require.ErrorIs(t, err, database.ErrShutdownTimeout)
	require.Less(t, elapsed, 150*time.Millisecond, "configured shutdown timeout should bound Shutdown even without caller deadline")

	select {
	case <-closeDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("adapter.Close background goroutine did not complete")
	}
}

func TestShutdown_Timeout_DoesNotForceClose(t *testing.T) {
	// RF-09: no timeout o manager NÃO deve forçar o pool.Close().
	// Verificado checando se o adapter.Close foi iniciado (goroutine lançada)
	// mas o manager retornou antes de completar; chamadas subsequentes são no-ops.
	closeDone := make(chan struct{})
	slow := &slowCloseAdapter{
		stubDBTX: &stubDBTX{},
		done:     closeDone,
		delay:    200 * time.Millisecond,
	}
	m := newTestManager(slow)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := m.Shutdown(ctx)
	require.ErrorIs(t, err, database.ErrShutdownTimeout)

	// A goroutine lançada pelo Shutdown deve eventualmente terminar.
	select {
	case <-closeDone:
		// adapter.Close foi executado até o fim em background — comportamento RF-09 correto.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("a goroutine do adapter.Close não completou dentro do tempo esperado")
	}
}

func TestShutdown_AdapterCloseError_Propagated(t *testing.T) {
	closeErr := errors.New("pool close failed")
	adapter := &mockAdapter{
		dbtx:     &stubDBTX{},
		closeErr: closeErr,
	}
	m := newTestManager(adapter)

	err := m.Shutdown(context.Background())
	require.ErrorIs(t, err, closeErr)
}

func TestShutdown_Idempotent_PreservesOriginalError(t *testing.T) {
	// Regressão: chamadas subsequentes a Shutdown devem retornar o erro
	// canônico produzido pela primeira execução, não nil.
	closeErr := errors.New("pool close failed")
	adapter := &mockAdapter{dbtx: &stubDBTX{}, closeErr: closeErr}
	m := newTestManager(adapter)

	first := m.Shutdown(context.Background())
	require.ErrorIs(t, first, closeErr)

	second := m.Shutdown(context.Background())
	require.ErrorIs(t, second, closeErr, "segundo Shutdown deve preservar o erro original")
	require.Equal(t, 1, adapter.closeCalls, "adapter.Close deve ser invocado uma única vez")
}

func TestShutdown_Idempotent_PreservesTimeoutError(t *testing.T) {
	adapter := &mockAdapter{
		dbtx:      &stubDBTX{},
		closeSlow: 200 * time.Millisecond,
	}
	m := newTestManager(adapter)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	first := m.Shutdown(ctx)
	require.ErrorIs(t, first, database.ErrShutdownTimeout)

	second := m.Shutdown(context.Background())
	require.ErrorIs(t, second, database.ErrShutdownTimeout, "erro de timeout deve persistir entre chamadas")
}

func TestShutdown_SetsClosed_PreventingPoolAccess(t *testing.T) {
	adapter := &mockAdapter{dbtx: &stubDBTX{}}
	m := newTestManager(adapter)
	require.NoError(t, m.Shutdown(context.Background()))

	// Após o shutdown, a flag closed deve ser definida.
	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()
	require.True(t, closed)
}

func TestShutdown_WaitsForInFlightTxBeforeClose(t *testing.T) {
	// RF-04: Shutdown deve aguardar drain das transações ativas antes de fechar o pool.
	adapter := &mockAdapter{dbtx: &stubDBTX{}, tx: &stubTx{}}
	m := newTestManager(adapter)

	tx, err := m.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- m.Shutdown(context.Background())
	}()

	// Dá tempo do Shutdown atingir o Wait.
	select {
	case <-shutdownDone:
		t.Fatal("Shutdown retornou antes do drain das transações em voo")
	case <-time.After(50 * time.Millisecond):
	}
	require.Equal(t, 0, adapter.closeCalls, "adapter.Close não pode ser chamado enquanto houver tx ativa")

	require.NoError(t, tx.Commit(context.Background()))

	select {
	case err := <-shutdownDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Shutdown não completou após drain")
	}
	require.Equal(t, 1, adapter.closeCalls, "adapter.Close deve ser invocado uma única vez após o drain")
}

func TestShutdown_TimeoutBeforeDrain_DoesNotCloseAdapter(t *testing.T) {
	// RF-09: timeout antes do drain → ErrShutdownTimeout SEM forçar Close no pool.
	adapter := &mockAdapter{dbtx: &stubDBTX{}, tx: &stubTx{}}
	m := newTestManager(adapter)

	tx, err := m.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	shutErr := m.Shutdown(ctx)
	require.ErrorIs(t, shutErr, database.ErrShutdownTimeout)
	require.Equal(t, 0, adapter.closeCalls, "adapter.Close não pode ser chamado se o drain estourou o timeout (RF-09)")
}

// slowCloseAdapter ajuda a verificar o RF-09: adapter.Close roda em background mesmo após timeout.
type slowCloseAdapter struct {
	stubDBTX *stubDBTX
	delay    time.Duration
	done     chan struct{}
}

func (a *slowCloseAdapter) Driver() database.Driver { return database.DriverPostgres }
func (a *slowCloseAdapter) DBTX() database.DBTX     { return a.stubDBTX }
func (a *slowCloseAdapter) Stats() internalpool.Stats {
	return internalpool.Stats{}
}
func (a *slowCloseAdapter) Attributes() []observability.Field { return nil }
func (a *slowCloseAdapter) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (a *slowCloseAdapter) Ping(_ context.Context) error { return nil }
func (a *slowCloseAdapter) Close(_ context.Context) error {
	time.Sleep(a.delay)
	close(a.done)
	return nil
}
