package fake

import (
	"context"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Provider implements a fake observability provider for testing purposes.
// It captures all operations so they can be inspected in tests.
type Provider struct {
	tracer  *FakeTracer
	logger  *FakeLogger
	metrics *FakeMetrics
}

// NewProvider creates a new fake observability provider for testing.
func NewProvider() *Provider {
	return &Provider{
		tracer:  NewFakeTracer(),
		logger:  NewFakeLogger(),
		metrics: NewFakeMetrics(),
	}
}

// Tracer returns the fake tracer.
func (p *Provider) Tracer() observability.Tracer {
	return p.tracer
}

// Logger returns the fake logger.
func (p *Provider) Logger() observability.Logger {
	return p.logger
}

// Metrics returns the fake metrics recorder.
func (p *Provider) Metrics() observability.Metrics {
	return p.metrics
}

// FakeTracer captures all tracing operations for test assertions.
type FakeTracer struct {
	mu    sync.RWMutex
	spans []*FakeSpan
}

// NewFakeTracer creates a new fake tracer.
func NewFakeTracer() *FakeTracer {
	return &FakeTracer{
		spans: make([]*FakeSpan, 0),
	}
}

// Start creates a fake span and captures it.
func (t *FakeTracer) Start(ctx context.Context, spanName string, opts ...observability.SpanOption) (context.Context, observability.Span) {
	config := observability.NewSpanConfig(opts)

	span := &FakeSpan{
		Name:       spanName,
		StartTime:  time.Now(),
		Attributes: config.Attributes(),
		Events:     make([]FakeEvent, 0),
	}

	t.mu.Lock()
	t.spans = append(t.spans, span)
	t.mu.Unlock()

	return ctx, span
}

// SpanFromContext returns a fake span (always nil for simplicity).
func (t *FakeTracer) SpanFromContext(ctx context.Context) observability.Span {
	return &FakeSpan{}
}

// ContextWithSpan returns the context unchanged.
func (t *FakeTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	return ctx
}

// GetSpans returns all captured spans (for test assertions).
func (t *FakeTracer) GetSpans() []*FakeSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]*FakeSpan, len(t.spans))
	copy(result, t.spans)
	return result
}

// Reset clears all captured spans.
func (t *FakeTracer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spans = make([]*FakeSpan, 0)
}

// FakeSpan captures span operations for test assertions.
type FakeSpan struct {
	mu          sync.RWMutex
	Name        string
	StartTime   time.Time
	EndTime     *time.Time
	Attributes  []observability.Field
	Events      []FakeEvent
	Status      observability.StatusCode
	StatusDesc  string
	RecordedErr error
}

// End marks the span as ended.
func (s *FakeSpan) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.EndTime = &now
}

// SetAttributes adds attributes to the span.
func (s *FakeSpan) SetAttributes(fields ...observability.Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attributes = append(s.Attributes, fields...)
}

// SetStatus sets the span status.
func (s *FakeSpan) SetStatus(code observability.StatusCode, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = code
	s.StatusDesc = description
}

// RecordError records an error on the span.
func (s *FakeSpan) RecordError(err error, fields ...observability.Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RecordedErr = err
	s.Attributes = append(s.Attributes, fields...)
}

// AddEvent adds an event to the span.
func (s *FakeSpan) AddEvent(name string, fields ...observability.Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Events = append(s.Events, FakeEvent{
		Name:      name,
		Timestamp: time.Now(),
		Fields:    fields,
	})
}

// Context returns a fake span context.
func (s *FakeSpan) Context() observability.SpanContext {
	return &FakeSpanContext{
		traceID:  "fake-trace-id",
		spanID:   "fake-span-id",
		sampled:  true,
	}
}

// FakeEvent represents a recorded span event.
type FakeEvent struct {
	Name      string
	Timestamp time.Time
	Fields    []observability.Field
}

// FakeSpanContext implements a fake span context.
type FakeSpanContext struct {
	traceID string
	spanID  string
	sampled bool
}

// TraceID returns the fake trace ID.
func (c *FakeSpanContext) TraceID() string {
	return c.traceID
}

// SpanID returns the fake span ID.
func (c *FakeSpanContext) SpanID() string {
	return c.spanID
}

// IsSampled returns whether the span is sampled.
func (c *FakeSpanContext) IsSampled() bool {
	return c.sampled
}

// FakeLogger captures all log operations for test assertions.
type FakeLogger struct {
	mu      *sync.RWMutex
	entries *[]LogEntry
	fields  []observability.Field
}

// NewFakeLogger creates a new fake logger.
func NewFakeLogger() *FakeLogger {
	entries := make([]LogEntry, 0)
	return &FakeLogger{
		mu:      &sync.RWMutex{},
		entries: &entries,
		fields:  make([]observability.Field, 0),
	}
}

// Debug captures a debug log entry.
func (l *FakeLogger) Debug(ctx context.Context, msg string, fields ...observability.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelDebug,
		Message:   msg,
		Fields:    append(l.fields, fields...),
		Timestamp: time.Now(),
	})
}

// Info captures an info log entry.
func (l *FakeLogger) Info(ctx context.Context, msg string, fields ...observability.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelInfo,
		Message:   msg,
		Fields:    append(l.fields, fields...),
		Timestamp: time.Now(),
	})
}

// Warn captures a warn log entry.
func (l *FakeLogger) Warn(ctx context.Context, msg string, fields ...observability.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelWarn,
		Message:   msg,
		Fields:    append(l.fields, fields...),
		Timestamp: time.Now(),
	})
}

// Error captures an error log entry.
func (l *FakeLogger) Error(ctx context.Context, msg string, fields ...observability.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelError,
		Message:   msg,
		Fields:    append(l.fields, fields...),
		Timestamp: time.Now(),
	})
}

// With creates a child logger with additional fields.
func (l *FakeLogger) With(fields ...observability.Field) observability.Logger {
	return &FakeLogger{
		mu:      l.mu,
		entries: l.entries,
		fields:  append(l.fields, fields...),
	}
}

// GetEntries returns all captured log entries (for test assertions).
func (l *FakeLogger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]LogEntry, len(*l.entries))
	copy(result, *l.entries)
	return result
}

// Reset clears all captured log entries.
func (l *FakeLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = make([]LogEntry, 0)
}

// LogEntry represents a captured log entry.
type LogEntry struct {
	Level     observability.LogLevel
	Message   string
	Fields    []observability.Field
	Timestamp time.Time
}

// FakeMetrics captures all metrics operations for test assertions.
type FakeMetrics struct {
	mu         sync.RWMutex
	counters   map[string]*FakeCounter
	histograms map[string]*FakeHistogram
	upDowns    map[string]*FakeUpDownCounter
}

// NewFakeMetrics creates a new fake metrics recorder.
func NewFakeMetrics() *FakeMetrics {
	return &FakeMetrics{
		counters:   make(map[string]*FakeCounter),
		histograms: make(map[string]*FakeHistogram),
		upDowns:    make(map[string]*FakeUpDownCounter),
	}
}

// Counter returns or creates a fake counter.
func (m *FakeMetrics) Counter(name, description, unit string) observability.Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, exists := m.counters[name]; exists {
		return c
	}

	c := &FakeCounter{
		Name:        name,
		Description: description,
		Unit:        unit,
		values:      make([]CounterValue, 0),
	}
	m.counters[name] = c
	return c
}

// Histogram returns or creates a fake histogram.
func (m *FakeMetrics) Histogram(name, description, unit string) observability.Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, exists := m.histograms[name]; exists {
		return h
	}

	h := &FakeHistogram{
		Name:        name,
		Description: description,
		Unit:        unit,
		values:      make([]HistogramValue, 0),
	}
	m.histograms[name] = h
	return h
}

// UpDownCounter returns or creates a fake up-down counter.
func (m *FakeMetrics) UpDownCounter(name, description, unit string) observability.UpDownCounter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if u, exists := m.upDowns[name]; exists {
		return u
	}

	u := &FakeUpDownCounter{
		Name:        name,
		Description: description,
		Unit:        unit,
		values:      make([]CounterValue, 0),
	}
	m.upDowns[name] = u
	return u
}

// Gauge is a no-op for fake metrics.
func (m *FakeMetrics) Gauge(name, description, unit string, callback observability.GaugeCallback) error {
	return nil
}

// GetCounter returns a counter by name for test assertions.
func (m *FakeMetrics) GetCounter(name string) *FakeCounter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[name]
}

// GetHistogram returns a histogram by name for test assertions.
func (m *FakeMetrics) GetHistogram(name string) *FakeHistogram {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.histograms[name]
}

// GetUpDownCounter returns an up-down counter by name for test assertions.
func (m *FakeMetrics) GetUpDownCounter(name string) *FakeUpDownCounter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upDowns[name]
}

// FakeCounter captures counter operations.
type FakeCounter struct {
	mu          sync.RWMutex
	Name        string
	Description string
	Unit        string
	values      []CounterValue
}

// Add captures a counter increment.
func (c *FakeCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = append(c.values, CounterValue{
		Value:     value,
		Fields:    fields,
		Timestamp: time.Now(),
	})
}

// Increment increments the counter by 1.
func (c *FakeCounter) Increment(ctx context.Context, fields ...observability.Field) {
	c.Add(ctx, 1, fields...)
}

// GetValues returns all captured values.
func (c *FakeCounter) GetValues() []CounterValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]CounterValue, len(c.values))
	copy(result, c.values)
	return result
}

// CounterValue represents a captured counter value.
type CounterValue struct {
	Value     int64
	Fields    []observability.Field
	Timestamp time.Time
}

// FakeHistogram captures histogram operations.
type FakeHistogram struct {
	mu          sync.RWMutex
	Name        string
	Description string
	Unit        string
	values      []HistogramValue
}

// Record captures a histogram value.
func (h *FakeHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, HistogramValue{
		Value:     value,
		Fields:    fields,
		Timestamp: time.Now(),
	})
}

// GetValues returns all captured values.
func (h *FakeHistogram) GetValues() []HistogramValue {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]HistogramValue, len(h.values))
	copy(result, h.values)
	return result
}

// HistogramValue represents a captured histogram value.
type HistogramValue struct {
	Value     float64
	Fields    []observability.Field
	Timestamp time.Time
}

// FakeUpDownCounter captures up-down counter operations.
type FakeUpDownCounter struct {
	mu          sync.RWMutex
	Name        string
	Description string
	Unit        string
	values      []CounterValue
}

// Add captures an up-down counter change.
func (u *FakeUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.values = append(u.values, CounterValue{
		Value:     value,
		Fields:    fields,
		Timestamp: time.Now(),
	})
}

// GetValues returns all captured values.
func (u *FakeUpDownCounter) GetValues() []CounterValue {
	u.mu.RLock()
	defer u.mu.RUnlock()
	result := make([]CounterValue, len(u.values))
	copy(result, u.values)
	return result
}
