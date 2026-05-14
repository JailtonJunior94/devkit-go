package otel

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// recordingHandler captures slog.Records for assertion in tests.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
	enabled bool
}

func newRecordingHandler() *recordingHandler { return &recordingHandler{enabled: true} }

func (r *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return r.enabled }
func (r *recordingHandler) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec.Clone())
	return nil
}
func (r *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return r }
func (r *recordingHandler) WithGroup(_ string) slog.Handler      { return r }
func (r *recordingHandler) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

// recordingProcessor captures ReadOnlySpans on OnEnd for assertion in tests.
type recordingProcessor struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (r *recordingProcessor) OnStart(_ context.Context, _ sdktrace.ReadWriteSpan) {}
func (r *recordingProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, s)
}
func (r *recordingProcessor) Shutdown(_ context.Context) error   { return nil }
func (r *recordingProcessor) ForceFlush(_ context.Context) error { return nil }
func (r *recordingProcessor) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.spans)
}
func (r *recordingProcessor) spanName(i int) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.spans[i].Name()
}

// ---------------------------------------------------------------------------
// WithExtraLogHandler option tests
// ---------------------------------------------------------------------------

func TestWithExtraLogHandler_AppendsToConfig(t *testing.T) {
	h1 := newRecordingHandler()
	h2 := newRecordingHandler()
	cfg := &Config{}

	WithExtraLogHandler(h1)(cfg)
	WithExtraLogHandler(h2)(cfg)

	assert.Len(t, cfg.ExtraLogHandlers, 2)
	assert.Equal(t, h1, cfg.ExtraLogHandlers[0])
	assert.Equal(t, h2, cfg.ExtraLogHandlers[1])
}

func TestWithExtraLogHandler_HandlerReceivesLog(t *testing.T) {
	recorder := newRecordingHandler()
	mh := newMultiHandler(recorder)
	logger := slog.New(mh)

	logger.Info("hello from test")

	assert.Equal(t, 1, recorder.len())
}

func TestMultiHandler_FanOutToAllHandlers(t *testing.T) {
	r1 := newRecordingHandler()
	r2 := newRecordingHandler()
	mh := newMultiHandler(r1, r2)
	logger := slog.New(mh)

	logger.Info("broadcast")
	logger.Warn("another")

	assert.Equal(t, 2, r1.len())
	assert.Equal(t, 2, r2.len())
}

func TestMultiHandler_DisabledHandlerSkipped(t *testing.T) {
	active := newRecordingHandler()
	inactive := &recordingHandler{enabled: false}
	mh := newMultiHandler(inactive, active)
	logger := slog.New(mh)

	logger.Info("only active should receive")

	assert.Equal(t, 1, active.len())
	assert.Equal(t, 0, inactive.len())
}

func TestMultiHandler_Enabled_ReturnsTrueIfAnyEnabled(t *testing.T) {
	inactive := &recordingHandler{enabled: false}
	active := newRecordingHandler()
	mh := newMultiHandler(inactive, active)

	assert.True(t, mh.Enabled(context.Background(), slog.LevelInfo))
}

func TestMultiHandler_Enabled_ReturnsFalseIfNoneEnabled(t *testing.T) {
	i1 := &recordingHandler{enabled: false}
	i2 := &recordingHandler{enabled: false}
	mh := newMultiHandler(i1, i2)

	assert.False(t, mh.Enabled(context.Background(), slog.LevelInfo))
}

// ---------------------------------------------------------------------------
// WithExtraSpanProcessor option tests
// ---------------------------------------------------------------------------

func TestWithExtraSpanProcessor_AppendsToConfig(t *testing.T) {
	p1 := &recordingProcessor{}
	p2 := &recordingProcessor{}
	cfg := &Config{}

	WithExtraSpanProcessor(p1)(cfg)
	WithExtraSpanProcessor(p2)(cfg)

	assert.Len(t, cfg.ExtraSpanProcessors, 2)
	assert.Equal(t, p1, cfg.ExtraSpanProcessors[0])
	assert.Equal(t, p2, cfg.ExtraSpanProcessors[1])
}

func TestWithExtraSpanProcessor_ProcessorReceivesSpan(t *testing.T) {
	recorder := &recordingProcessor{}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(noopExporter{}),
		sdktrace.WithSpanProcessor(recorder),
	)
	tracer := tp.Tracer("test-tracer")

	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	assert.Equal(t, 1, recorder.len())
	assert.Equal(t, "test-span", recorder.spanName(0))
}

func TestWithExtraSpanProcessor_MultipleProcessorsAllReceiveSpan(t *testing.T) {
	r1 := &recordingProcessor{}
	r2 := &recordingProcessor{}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(noopExporter{}),
		sdktrace.WithSpanProcessor(r1),
		sdktrace.WithSpanProcessor(r2),
	)
	tracer := tp.Tracer("test-tracer")

	_, span := tracer.Start(context.Background(), "multi-span")
	span.End()

	assert.Equal(t, 1, r1.len())
	assert.Equal(t, 1, r2.len())
	assert.Equal(t, "multi-span", r1.spanName(0))
	assert.Equal(t, "multi-span", r2.spanName(0))
}

// noopExporter is a minimal sdktrace.SpanExporter that discards all spans.
type noopExporter struct{}

func (noopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error { return nil }
func (noopExporter) Shutdown(_ context.Context) error                               { return nil }
