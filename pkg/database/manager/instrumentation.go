package manager

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

type instrumentation struct {
	driver        database.Driver
	attrs         []observability.Field
	obs           observability.Observability
	fallback      *slog.Logger
	sqlLogging    bool
	queryDuration observability.Histogram
}

func newInstrumentation(
	driver database.Driver,
	attrs []observability.Field,
	obs observability.Observability,
	fallback *slog.Logger,
	sqlLogging bool,
) instrumentation {
	if obs == nil {
		obs = noop.NewProvider()
	}

	return instrumentation{
		driver:        driver,
		attrs:         cloneFields(attrs),
		obs:           obs,
		fallback:      fallback,
		sqlLogging:    sqlLogging,
		queryDuration: obs.Metrics().Histogram("database.query.duration_ms", "Query latency by operation", "ms"),
	}
}

func (i instrumentation) WrapDBTX(dbtx database.DBTX) database.DBTX {
	return &instrumentedDBTX{base: dbtx, inst: i}
}

func (i instrumentation) WrapTx(tx database.Tx) database.Tx {
	return &instrumentedTx{
		base: tx,
		dbtx: instrumentedDBTX{base: tx, inst: i},
	}
}

func (i instrumentation) start(ctx context.Context, op string) (context.Context, observability.Span, time.Time) {
	spanName := "db." + string(i.driver) + "." + op
	fields := append(cloneFields(i.attrs), observability.String("db.operation", op))
	ctx, span := i.obs.Tracer().Start(ctx, spanName, observability.WithAttributes(fields...))
	return ctx, span, time.Now()
}

func (i instrumentation) finish(
	ctx context.Context,
	span observability.Span,
	op string,
	query string,
	args []any,
	start time.Time,
	err error,
) {
	duration := time.Since(start)
	metricFields := append(cloneFields(i.attrs), observability.String("db.operation", op))
	i.queryDuration.Record(ctx, float64(duration.Milliseconds()), metricFields...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())
	} else {
		span.SetStatus(observability.StatusCodeOK, "ok")
	}
	span.End()

	if !i.sqlLogging {
		return
	}

	fields := append(
		append(cloneFields(i.attrs), observability.String("db.operation", op)),
		observability.String("query", query),
		observability.Any("args", redactedArgs(args)),
		observability.Int64("duration_ms", duration.Milliseconds()),
	)
	if err != nil {
		fields = append(fields, observability.Error(err))
	}

	logger := i.obs.Logger()
	if logger != nil && !isNoopObservability(i.obs) {
		logger.Debug(ctx, "database query executed", fields...)
		return
	}
	if i.fallback != nil {
		i.fallback.DebugContext(ctx, "database query executed", slogFields(fields)...)
	}
}

type instrumentedDBTX struct {
	base database.DBTX
	inst instrumentation
}

func (d *instrumentedDBTX) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	ctx, span, start := d.inst.start(ctx, "exec")
	result, err := d.base.ExecContext(ctx, query, args...)
	d.inst.finish(ctx, span, "exec", query, args, start, err)
	return result, err
}

func (d *instrumentedDBTX) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	ctx, span, start := d.inst.start(ctx, "query")
	rows, err := d.base.QueryContext(ctx, query, args...)
	if err != nil {
		d.inst.finish(ctx, span, "query", query, args, start, err)
		return nil, err
	}
	return &instrumentedRows{
		base:  rows,
		ctx:   ctx,
		query: query,
		args:  append([]any(nil), args...),
		op:    "query",
		start: start,
		span:  span,
		inst:  d.inst,
	}, nil
}

func (d *instrumentedDBTX) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	ctx, span, start := d.inst.start(ctx, "query_row")
	row := d.base.QueryRowContext(ctx, query, args...)
	state := &rowSpanState{
		ctx:   ctx,
		query: query,
		args:  append([]any(nil), args...),
		op:    "query_row",
		start: start,
		span:  span,
		inst:  d.inst,
		once:  &sync.Once{},
	}
	r := &instrumentedRow{base: row, state: state}
	r.cleanup = runtime.AddCleanup(r, finishOrphanRowSpan, state)
	return r
}

type instrumentedTx struct {
	base database.Tx
	dbtx instrumentedDBTX
}

func (t *instrumentedTx) ExecContext(ctx context.Context, query string, args ...any) (database.Result, error) {
	return t.dbtx.ExecContext(ctx, query, args...)
}

func (t *instrumentedTx) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	return t.dbtx.QueryContext(ctx, query, args...)
}

func (t *instrumentedTx) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return t.dbtx.QueryRowContext(ctx, query, args...)
}

func (t *instrumentedTx) Commit(ctx context.Context) error {
	ctx, span, start := t.dbtx.inst.start(ctx, "commit")
	err := t.base.Commit(ctx)
	t.dbtx.inst.finish(ctx, span, "commit", "", nil, start, err)
	return err
}

func (t *instrumentedTx) Rollback(ctx context.Context) error {
	ctx, span, start := t.dbtx.inst.start(ctx, "rollback")
	err := t.base.Rollback(ctx)
	t.dbtx.inst.finish(ctx, span, "rollback", "", nil, start, err)
	return err
}

type instrumentedRows struct {
	base  database.Rows
	ctx   context.Context
	query string
	args  []any
	op    string
	start time.Time
	span  observability.Span
	inst  instrumentation
	once  sync.Once
}

func (r *instrumentedRows) Next() bool {
	next := r.base.Next()
	if !next {
		r.finish(r.base.Err())
	}
	return next
}

func (r *instrumentedRows) Scan(dest ...any) error {
	return r.base.Scan(dest...)
}

func (r *instrumentedRows) Close() error {
	err := r.base.Close()
	r.finish(err)
	return err
}

func (r *instrumentedRows) Err() error {
	err := r.base.Err()
	if err != nil {
		r.finish(err)
	}
	return err
}

func (r *instrumentedRows) finish(err error) {
	r.once.Do(func() {
		r.inst.finish(r.ctx, r.span, r.op, r.query, r.args, r.start, err)
	})
}

type rowSpanState struct {
	ctx   context.Context
	query string
	args  []any
	op    string
	start time.Time
	span  observability.Span
	inst  instrumentation
	once  *sync.Once
}

type instrumentedRow struct {
	base    database.Row
	state   *rowSpanState
	cleanup runtime.Cleanup
}

func (r *instrumentedRow) Scan(dest ...any) error {
	err := r.base.Scan(dest...)
	r.state.once.Do(func() {
		r.state.inst.finish(r.state.ctx, r.state.span, r.state.op, r.state.query, r.state.args, r.state.start, err)
	})
	r.cleanup.Stop()
	return err
}

func finishOrphanRowSpan(state *rowSpanState) {
	state.once.Do(func() {
		state.inst.finish(state.ctx, state.span, state.op, state.query, state.args, state.start, nil)
	})
}

func redactedArgs(args []any) []string {
	redacted := make([]string, len(args))
	for idx := range args {
		redacted[idx] = "?"
	}
	return redacted
}

func cloneFields(fields []observability.Field) []observability.Field {
	if len(fields) == 0 {
		return nil
	}
	cloned := make([]observability.Field, len(fields))
	copy(cloned, fields)
	return cloned
}

func slogFields(fields []observability.Field) []any {
	attrs := make([]any, 0, len(fields)*2)
	for _, field := range fields {
		attrs = append(attrs, field.Key, field.AnyValue())
	}
	return attrs
}

type noopMarker interface {
	IsNoop() bool
}

func isNoopObservability(obs observability.Observability) bool {
	if obs == nil {
		return true
	}
	if m, ok := obs.(noopMarker); ok {
		return m.IsNoop()
	}
	return false
}
