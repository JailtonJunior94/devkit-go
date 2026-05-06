package uow_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"
)

// --- fakes ---------------------------------------------------------------

// fakeTx registra chamadas de Commit/Rollback e pode simular erros.
type fakeTx struct {
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
}

func (t *fakeTx) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}
func (t *fakeTx) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}
func (t *fakeTx) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row { return nil }
func (t *fakeTx) Commit(_ context.Context) error {
	t.committed = true
	return t.commitErr
}
func (t *fakeTx) Rollback(_ context.Context) error {
	t.rolledBack = true
	return t.rollbackErr
}

// fakeManager satisfaz manager.Manager para que possa ser passado para uow.New[T].
// Apenas BeginTx é usado pelo UoW em tempo de execução; os outros métodos são stubs.
type fakeManager struct {
	mu         sync.Mutex
	tx         *fakeTx
	txFactory  func() database.Tx
	beginErr   error
	lastOpts   database.TxOptions
	beginCalls int
}

func (m *fakeManager) Driver() database.Driver { return database.DriverPostgres }
func (m *fakeManager) DBTX(_ context.Context) database.DBTX {
	return nil
}
func (m *fakeManager) BeginTx(_ context.Context, opts database.TxOptions) (database.Tx, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beginCalls++
	m.lastOpts = opts
	if m.beginErr != nil {
		return nil, m.beginErr
	}
	if m.txFactory != nil {
		return m.txFactory(), nil
	}
	return m.tx, nil
}
func (m *fakeManager) Ping(_ context.Context) error     { return nil }
func (m *fakeManager) Shutdown(_ context.Context) error { return nil }

// --- testes ----------------------------------------------------------------

func TestUnitOfWork_Do_CommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	u := uow.New[string](&fakeManager{tx: tx})

	result, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "ok", nil
	})

	require.NoError(t, err)
	require.Equal(t, "ok", result)
	require.True(t, tx.committed)
	require.False(t, tx.rolledBack)
}

func TestUnitOfWork_Do_RollsBackOnError(t *testing.T) {
	fnErr := errors.New("fn failed")
	tx := &fakeTx{}
	u := uow.New[string](&fakeManager{tx: tx})

	result, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", fnErr
	})

	require.ErrorIs(t, err, fnErr)
	require.Empty(t, result)
	require.True(t, tx.rolledBack)
	require.False(t, tx.committed)
}

func TestUnitOfWork_Do_RollsBackOnPanic(t *testing.T) {
	tx := &fakeTx{}
	u := uow.New[string](&fakeManager{tx: tx})

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
			panic("something went wrong")
		})
	}()

	require.True(t, panicked, "o pânico deve ser re-propagado")
	require.True(t, tx.rolledBack, "o rollback deve executar antes do pânico ser re-propagado")
	require.False(t, tx.committed)
}

func TestUnitOfWork_Do_PanicWithRollbackError_LogsAndRepropagates(t *testing.T) {
	// RF-26: panic deve ser repropagado. Erros de rollback no caminho de panic
	// não podem ser silenciados: devem ser logados via observability (R-O11Y-001).
	rollbackFailure := errors.New("rollback failed during panic")
	tx := &fakeTx{rollbackErr: rollbackFailure}
	obs := fake.NewProvider()
	u := uow.New[string](&fakeManager{tx: tx}, uow.WithObservability(obs))

	panicValue := errors.New("boom")
	var caught any
	func() {
		defer func() { caught = recover() }()
		_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
			panic(panicValue)
		})
	}()

	require.Equal(t, panicValue, caught, "panic original deve ser repropagado mesmo com falha no rollback")
	require.True(t, tx.rolledBack, "rollback deve ser tentado antes do re-panic")

	logger, ok := obs.Logger().(*fake.FakeLogger)
	require.True(t, ok)
	entries := logger.GetEntries()
	require.NotEmpty(t, entries, "falha de rollback no caminho de panic deve ser logada")

	var found bool
	for _, e := range entries {
		if e.Level != observability.LogLevelError {
			continue
		}
		for _, f := range e.Fields {
			if f.Key == "error" {
				found = true
			}
		}
	}
	require.True(t, found, "log de erro deve carregar o erro do rollback como field")
}

func TestUnitOfWork_Do_FnError_RollbackFailureIsLogged(t *testing.T) {
	// Regressão: rollback que falhar no caminho de erro do fn não pode ser
	// silenciosamente descartado (R-O11Y-001). O erro retornado ao caller
	// permanece o erro original do fn.
	rollbackFailure := errors.New("rollback failed during fn error")
	fnErr := errors.New("fn boom")
	tx := &fakeTx{rollbackErr: rollbackFailure}
	obs := fake.NewProvider()
	u := uow.New[string](&fakeManager{tx: tx}, uow.WithObservability(obs))

	_, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", fnErr
	})

	require.ErrorIs(t, err, fnErr, "erro retornado deve ser o erro do fn, não o do rollback")
	require.NotErrorIs(t, err, rollbackFailure, "erro do rollback não deve substituir o erro do fn")
	require.True(t, tx.rolledBack)

	logger, ok := obs.Logger().(*fake.FakeLogger)
	require.True(t, ok)

	var found bool
	for _, e := range logger.GetEntries() {
		if e.Level != observability.LogLevelError {
			continue
		}
		for _, f := range e.Fields {
			if f.Key == "error" {
				found = true
			}
		}
	}
	require.True(t, found, "log de erro deve carregar o erro do rollback como field")
}

func TestUnitOfWork_Do_PanicValuePreserved(t *testing.T) {
	sentinel := errors.New("sentinel panic value")
	tx := &fakeTx{}
	u := uow.New[string](&fakeManager{tx: tx})

	var caught any
	func() {
		defer func() { caught = recover() }()
		_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
			panic(sentinel)
		})
	}()

	require.Equal(t, sentinel, caught, "o valor original do pânico deve ser preservado")
}

func TestUnitOfWork_Do_ErrNestedTransaction_SameInstance(t *testing.T) {
	tx := &fakeTx{}
	u := uow.New[string](&fakeManager{tx: tx})

	var innerErr error
	_, _ = u.Do(context.Background(), func(ctx context.Context, _ database.DBTX) (string, error) {
		// Mesma instância: tx propagada via context é detectada → ErrNestedTransaction.
		_, innerErr = u.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
			return "inner", nil
		})
		return "", innerErr
	})

	require.ErrorIs(t, innerErr, database.ErrNestedTransaction)
}

func TestUnitOfWork_Do_NestedWithFreshContext_ReturnsErrNestedTransaction(t *testing.T) {
	mgr := &fakeManager{tx: &fakeTx{}}
	u := uow.New[string](mgr)

	var innerErr error
	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		_, innerErr = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
			return "inner", nil
		})
		return "", innerErr
	})

	require.ErrorIs(t, innerErr, database.ErrNestedTransaction)
	require.Equal(t, 1, mgr.beginCalls, "nested call must not open a second transaction")
}

func TestUnitOfWork_Do_ErrNestedTransaction_DifferentInstance(t *testing.T) {
	tx1 := &fakeTx{}
	tx2 := &fakeTx{}
	u1 := uow.New[string](&fakeManager{tx: tx1})
	u2 := uow.New[string](&fakeManager{tx: tx2})

	var innerErr error
	_, _ = u1.Do(context.Background(), func(ctx context.Context, _ database.DBTX) (string, error) {
		// ctx carrega uma tx injetada pelo u1.Do; u2.Do deve detectá-la e recusar.
		_, innerErr = u2.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
			return "inner", nil
		})
		return "", innerErr
	})

	require.ErrorIs(t, innerErr, database.ErrNestedTransaction)
}

func TestUnitOfWork_Do_BeginTxError_Propagated(t *testing.T) {
	beginErr := errors.New("cannot begin")
	u := uow.New[string](&fakeManager{beginErr: beginErr})

	_, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.ErrorIs(t, err, beginErr)
}

func TestUnitOfWork_Do_CommitError_Propagated(t *testing.T) {
	commitErr := errors.New("commit failed")
	tx := &fakeTx{commitErr: commitErr}
	u := uow.New[string](&fakeManager{tx: tx})

	_, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "result", nil
	})

	require.ErrorIs(t, err, commitErr)
}

func TestUnitOfWork_Do_WithIsolation_AppliedToBeginTx(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr, uow.WithIsolation(sql.LevelSerializable))

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.Equal(t, database.LevelSerializable, mgr.lastOpts.Isolation)
}

func TestUnitOfWork_Do_WithReadOnly_AppliedToBeginTx(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr, uow.WithReadOnly(true))

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.True(t, mgr.lastOpts.ReadOnly)
}

func TestUnitOfWork_Do_CtxCancelled_RollsBack(t *testing.T) {
	tx := &fakeTx{}
	u := uow.New[string](&fakeManager{tx: tx})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // já cancelado

	_, err := u.Do(ctx, func(ctx context.Context, _ database.DBTX) (string, error) {
		return "", fmt.Errorf("fn: %w", ctx.Err())
	})

	require.Error(t, err)
	require.True(t, tx.rolledBack)
}

func TestUnitOfWork_Do_Commits_EmitsTxMetricsAndSpan(t *testing.T) {
	tx := &fakeTx{}
	obs := fake.NewProvider()
	u := uow.New[string](&fakeManager{tx: tx}, uow.WithObservability(obs))

	_, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "ok", nil
	})
	require.NoError(t, err)

	commitCounter := obs.Metrics().(*fake.FakeMetrics).GetCounter("database.tx.committed")
	require.NotNil(t, commitCounter)
	require.NotEmpty(t, commitCounter.GetValues())

	durationHist := obs.Metrics().(*fake.FakeMetrics).GetHistogram("database.tx.duration_ms")
	require.NotNil(t, durationHist)
	require.NotEmpty(t, durationHist.GetValues())

	spans := obs.Tracer().(*fake.FakeTracer).GetSpans()
	require.NotEmpty(t, spans)
	require.Equal(t, "db.postgres.tx", spans[0].Name)
}

func TestUnitOfWork_Do_Rollback_EmitsRollbackMetric(t *testing.T) {
	tx := &fakeTx{}
	obs := fake.NewProvider()
	u := uow.New[string](&fakeManager{tx: tx}, uow.WithObservability(obs))

	_, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", errors.New("boom")
	})
	require.Error(t, err)

	rollbackCounter := obs.Metrics().(*fake.FakeMetrics).GetCounter("database.tx.rolledback")
	require.NotNil(t, rollbackCounter)
	require.NotEmpty(t, rollbackCounter.GetValues())
}

func TestUnitOfWork_Do_TxInjectedInCtx(t *testing.T) {
	tx := &fakeTx{}
	u := uow.New[string](&fakeManager{tx: tx})

	var ctxTx database.DBTX
	var ok bool
	_, _ = u.Do(context.Background(), func(ctx context.Context, _ database.DBTX) (string, error) {
		ctxTx, ok = database.FromContext(ctx)
		return "", nil
	})

	require.True(t, ok, "tx deve ser injetada no ctx")
	require.NotNil(t, ctxTx)
}

func TestUnitOfWork_Do_DefaultOptions_NilIsolationAndFalseReadOnly(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr)

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.Equal(t, database.LevelDefault, mgr.lastOpts.Isolation)
	require.False(t, mgr.lastOpts.ReadOnly)
}

// I1: rollback emergencial após panic deve usar ctx desacoplado do caller,
// caso contrário um ctx já cancelado bloqueia o rollback antes mesmo de tentar.
type ctxCapturingTx struct {
	*fakeTx
	rollbackCtx context.Context
	rollbackErr error
}

func (t *ctxCapturingTx) Rollback(ctx context.Context) error {
	t.rollbackCtx = ctx
	t.rollbackErr = ctx.Err()
	return t.fakeTx.Rollback(ctx)
}

func TestUnitOfWork_Do_PanicWithCancelledCtx_RollsBackWithFreshCtx(t *testing.T) {
	captured := &ctxCapturingTx{fakeTx: &fakeTx{}}
	mgr := &fakeManager{txFactory: func() database.Tx { return captured }}
	u := uow.New[string](mgr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	defer func() {
		_ = recover()
	}()
	_, _ = u.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
		panic("boom")
	})

	require.True(t, captured.rolledBack, "rollback emergencial deve executar mesmo com ctx cancelado")
	require.NotNil(t, captured.rollbackCtx, "rollback deve receber um ctx novo")
	require.NoError(t, captured.rollbackCtx.Err(), "ctx do rollback emergencial não pode estar cancelado")
}

func TestUnitOfWork_Do_AllowsConcurrentTopLevelCallsOnSameInstance(t *testing.T) {
	mgr := &fakeManager{
		txFactory: func() database.Tx { return &fakeTx{} },
	}
	u := uow.New[string](mgr)

	release := make(chan struct{})
	entered := make(chan struct{}, 2)
	errs := make(chan error, 2)

	run := func() {
		_, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
			entered <- struct{}{}
			<-release
			return "ok", nil
		})
		errs <- err
	}

	go run()
	go run()

	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("expected both top-level calls to begin without nested transaction rejection")
		}
	}

	close(release)

	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, 2, mgr.beginCalls)
}

func TestUnitOfWork_Do_FnErrorWithCancelledCtx_RollsBackWithFreshCtx(t *testing.T) {
	captured := &ctxCapturingTx{fakeTx: &fakeTx{}}
	mgr := &fakeManager{txFactory: func() database.Tx { return captured }}
	u := uow.New[string](mgr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := u.Do(ctx, func(ctx context.Context, _ database.DBTX) (string, error) {
		return "", fmt.Errorf("fn: %w", ctx.Err())
	})

	require.Error(t, err)
	require.True(t, captured.rolledBack, "rollback deve executar mesmo com ctx cancelado")
	require.NotNil(t, captured.rollbackCtx, "rollback deve receber um ctx novo")
	require.NoError(t, captured.rollbackErr, "ctx do rollback normal não pode chegar cancelado")
}

func TestUnitOfWork_Do_CommitFailureWithCancelledCtx_RollsBackWithFreshCtx(t *testing.T) {
	captured := &ctxCapturingTx{fakeTx: &fakeTx{commitErr: errors.New("commit failed")}}
	mgr := &fakeManager{txFactory: func() database.Tx { return captured }}
	u := uow.New[string](mgr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := u.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
		return "ok", nil
	})

	require.Error(t, err)
	require.True(t, captured.rolledBack, "rollback defensivo após falha de commit deve executar")
	require.NotNil(t, captured.rollbackCtx, "rollback deve receber um ctx novo")
	require.NoError(t, captured.rollbackErr, "ctx do rollback após commit não pode chegar cancelado")
}
