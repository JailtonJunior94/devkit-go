package uow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/petermattis/goid"
)

const rollbackOnPanicTimeout = 5 * time.Second

type UnitOfWork[T any] interface {
	Do(ctx context.Context, fn func(ctx context.Context, tx database.DBTX) (T, error), opts ...Option) (T, error)
}

type txBeginner interface {
	BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error)
}

type unitOfWork[T any] struct {
	mgr        txBeginner
	opts       options
	driver     database.Driver
	reentrant  reentrancyGuard
	txTimer    observability.Histogram
	txCommit   observability.Counter
	txRollback observability.Counter
}

func New[T any](mgr manager.Manager, opts ...Option) UnitOfWork[T] {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	driver := mgr.Driver()
	return &unitOfWork[T]{
		mgr:        mgr,
		opts:       o,
		driver:     driver,
		txTimer: o.observability.Metrics().HistogramWithBuckets(
			"database.tx.duration_ms",
			"Transaction duration by outcome",
			"ms",
			[]float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		),
		txCommit:   o.observability.Metrics().Counter("database.tx.committed", "Committed transactions", "{transactions}"),
		txRollback: o.observability.Metrics().Counter("database.tx.rolledback", "Rolled back transactions", "{transactions}"),
	}
}

func (u *unitOfWork[T]) Do(ctx context.Context, fn func(ctx context.Context, tx database.DBTX) (T, error), opts ...Option) (result T, err error) {
	var zero T

	if !u.reentrant.Enter() {
		return zero, database.ErrNestedTransaction
	}
	defer u.reentrant.Leave()

	if _, ok := database.FromContext(ctx); ok {
		return zero, database.ErrNestedTransaction
	}

	metricAttrs := []observability.Field{
		observability.String("db.system", string(u.driver)),
	}
	ctx, span := u.opts.observability.Tracer().Start(
		ctx,
		fmt.Sprintf("db.%s.tx", u.driver),
		observability.WithAttributes(metricAttrs...),
	)
	start := time.Now()
	outcome := "error"
	defer func() {
		u.txTimer.Record(ctx, float64(time.Since(start).Milliseconds()), append(metricAttrs, observability.String("outcome", outcome))...)
		span.End()
	}()

	effectiveOpts := u.opts
	for _, opt := range opts {
		opt(&effectiveOpts)
	}

	txOpts := database.TxOptions{
		Isolation: effectiveOpts.isolation,
		ReadOnly:  effectiveOpts.readOnly,
	}

	tx, txErr := u.mgr.BeginTx(ctx, txOpts)
	if txErr != nil {
		span.RecordError(txErr)
		span.SetStatus(observability.StatusCodeError, txErr.Error())
		return zero, fmt.Errorf("uow: begin tx: %w", txErr)
	}

	txCtx := database.WithTx(ctx, tx)

	defer func() {
		if r := recover(); r != nil {
			if rbErr := u.rollbackWithFreshContext(ctx, tx, "uow.rollback_on_panic", "uow: rollback after panic failed"); rbErr != nil {
				span.RecordError(rbErr)
			}
			u.txRollback.Increment(ctx, metricAttrs...)
			outcome = "panic"
			if panicErr, ok := r.(error); ok {
				span.RecordError(panicErr)
				span.SetStatus(observability.StatusCodeError, panicErr.Error())
			} else {
				span.SetStatus(observability.StatusCodeError, "panic")
				span.AddEvent("panic", observability.Any("value", r))
			}
			panic(r)
		}
	}()

	result, err = fn(txCtx, tx)

	if err != nil {
		if rbErr := u.rollbackWithFreshContext(ctx, tx, "uow.rollback_on_error", "uow: rollback after fn error failed"); rbErr != nil {
			span.RecordError(rbErr)
		}
		u.txRollback.Increment(ctx, metricAttrs...)
		outcome = "rolled_back"
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())
		return zero, err
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {

		if rbErr := u.rollbackWithFreshContext(ctx, tx, "uow.rollback_on_commit_failure", "uow: rollback after commit failure"); rbErr != nil {
			span.RecordError(rbErr)
		}
		u.txRollback.Increment(ctx, metricAttrs...)
		outcome = "rolled_back"
		span.RecordError(commitErr)
		span.SetStatus(observability.StatusCodeError, commitErr.Error())
		return zero, fmt.Errorf("uow: commit: %w", commitErr)
	}

	u.txCommit.Increment(ctx, metricAttrs...)
	outcome = "committed"
	span.SetStatus(observability.StatusCodeOK, "ok")
	return result, nil
}

func (u *unitOfWork[T]) rollbackWithFreshContext(
	ctx context.Context,
	tx database.Tx,
	operation string,
	message string,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackOnPanicTimeout)
	defer cancel()

	if err := tx.Rollback(rollbackCtx); err != nil {
		u.opts.observability.Logger().Error(
			ctx,
			message,
			observability.String("operation", operation),
			observability.String("layer", "database"),
			observability.String("entity", "uow"),
			observability.Error(err),
		)
		return err
	}

	return nil
}

type reentrancyGuard struct {
	mu     sync.Mutex
	active map[int64]struct{}
}

func (g *reentrancyGuard) Enter() bool {
	gid := currentGoroutineID()

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.active[gid]; exists {
		return false
	}
	if g.active == nil {
		g.active = make(map[int64]struct{})
	}
	g.active[gid] = struct{}{}
	return true
}

func (g *reentrancyGuard) Leave() {
	gid := currentGoroutineID()

	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.active, gid)
}

func currentGoroutineID() int64 {
	return goid.Get()
}
