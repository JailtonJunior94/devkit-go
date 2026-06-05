package fake

import (
	"context"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

var (
	_ observability.Observability = (*Provider)(nil)
	_ observability.Tracer        = (*FakeTracer)(nil)
	_ observability.Span          = (*FakeSpan)(nil)
	_ observability.SpanContext   = (*FakeSpanContext)(nil)
	_ observability.Logger        = (*FakeLogger)(nil)
	_ observability.Metrics       = (*FakeMetrics)(nil)
	_ observability.Counter       = (*FakeCounter)(nil)
	_ observability.Histogram     = (*FakeHistogram)(nil)
	_ observability.UpDownCounter = (*FakeUpDownCounter)(nil)
)

// Provider é um fake de observabilidade para testes; captura operações para inspeção.
type Provider struct {
	tracer  *FakeTracer
	logger  *FakeLogger
	metrics *FakeMetrics
}

func NewProvider() *Provider {
	return &Provider{
		tracer:  NewFakeTracer(),
		logger:  NewFakeLogger(),
		metrics: NewFakeMetrics(),
	}
}

func (p *Provider) Tracer() observability.Tracer   { return p.tracer }
func (p *Provider) Logger() observability.Logger   { return p.logger }
func (p *Provider) Metrics() observability.Metrics { return p.metrics }

func (p *Provider) Shutdown(_ context.Context) error { return nil }

type fakeSpanKey struct{}

type FakeTracer struct {
	mu    sync.RWMutex
	spans []*FakeSpan
}

func NewFakeTracer() *FakeTracer {
	return &FakeTracer{
		spans: nil,
	}
}

func (t *FakeTracer) Start(ctx context.Context, spanName string, opts ...observability.SpanOption) (context.Context, observability.Span) {
	config := observability.NewSpanConfig(opts)

	span := &FakeSpan{
		Name:       spanName,
		StartTime:  time.Now(),
		Attributes: config.Attributes(),
		Events:     nil,
	}

	t.mu.Lock()
	t.spans = append(t.spans, span)
	t.mu.Unlock()

	return context.WithValue(ctx, fakeSpanKey{}, span), span
}

func (t *FakeTracer) SpanFromContext(ctx context.Context) observability.Span {
	if span, ok := ctx.Value(fakeSpanKey{}).(*FakeSpan); ok {
		return span
	}
	return &FakeSpan{}
}

func (t *FakeTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	return context.WithValue(ctx, fakeSpanKey{}, span)
}

func (t *FakeTracer) GetSpans() []*FakeSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]*FakeSpan, len(t.spans))
	copy(result, t.spans)
	return result
}

func (t *FakeTracer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spans = nil
}

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

func (s *FakeSpan) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.EndTime = &now
}

func (s *FakeSpan) SetAttributes(fields ...observability.Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attributes = append(s.Attributes, fields...)
}

func (s *FakeSpan) SetStatus(code observability.StatusCode, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = code
	s.StatusDesc = description
}

func (s *FakeSpan) RecordError(err error, fields ...observability.Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RecordedErr = err
	s.Attributes = append(s.Attributes, fields...)
}

func (s *FakeSpan) AddEvent(name string, fields ...observability.Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Events = append(s.Events, FakeEvent{
		Name:      name,
		Timestamp: time.Now(),
		Fields:    fields,
	})
}

func (s *FakeSpan) Context() observability.SpanContext {
	return &FakeSpanContext{
		traceID: "fake-trace-id",
		spanID:  "fake-span-id",
		sampled: true,
	}
}

func (s *FakeSpan) TraceID() string { return "fake-trace-id" }
func (s *FakeSpan) SpanID() string  { return "fake-span-id" }
func (s *FakeSpan) IsSampled() bool { return true }

type FakeEvent struct {
	Name      string
	Timestamp time.Time
	Fields    []observability.Field
}

type FakeSpanContext struct {
	traceID string
	spanID  string
	sampled bool
}

func (c *FakeSpanContext) TraceID() string { return c.traceID }
func (c *FakeSpanContext) SpanID() string  { return c.spanID }
func (c *FakeSpanContext) IsSampled() bool { return c.sampled }

type FakeLogger struct {
	mu      *sync.RWMutex
	entries *[]LogEntry
	fields  []observability.Field
}

func NewFakeLogger() *FakeLogger {
	var entries []LogEntry
	return &FakeLogger{
		mu:      &sync.RWMutex{},
		entries: &entries,
		fields:  nil,
	}
}

func (l *FakeLogger) Debug(ctx context.Context, msg string, fields ...observability.Field) {
	allFields := make([]observability.Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelDebug,
		Message:   msg,
		Fields:    allFields,
		Timestamp: time.Now(),
	})
}

func (l *FakeLogger) Info(ctx context.Context, msg string, fields ...observability.Field) {
	allFields := make([]observability.Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelInfo,
		Message:   msg,
		Fields:    allFields,
		Timestamp: time.Now(),
	})
}

func (l *FakeLogger) Warn(ctx context.Context, msg string, fields ...observability.Field) {
	allFields := make([]observability.Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelWarn,
		Message:   msg,
		Fields:    allFields,
		Timestamp: time.Now(),
	})
}

func (l *FakeLogger) Error(ctx context.Context, msg string, fields ...observability.Field) {
	allFields := make([]observability.Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:     observability.LogLevelError,
		Message:   msg,
		Fields:    allFields,
		Timestamp: time.Now(),
	})
}

func (l *FakeLogger) With(fields ...observability.Field) observability.Logger {
	newFields := make([]observability.Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)

	return &FakeLogger{
		mu:      l.mu,
		entries: l.entries,
		fields:  newFields,
	}
}

func (l *FakeLogger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]LogEntry, len(*l.entries))
	copy(result, *l.entries)
	return result
}

func (l *FakeLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = nil
}

type LogEntry struct {
	Level     observability.LogLevel
	Message   string
	Fields    []observability.Field
	Timestamp time.Time
}

type FakeMetrics struct {
	mu         sync.RWMutex
	counters   map[string]*FakeCounter
	histograms map[string]*FakeHistogram
	upDowns    map[string]*FakeUpDownCounter
}

func NewFakeMetrics() *FakeMetrics {
	return &FakeMetrics{
		counters:   make(map[string]*FakeCounter),
		histograms: make(map[string]*FakeHistogram),
		upDowns:    make(map[string]*FakeUpDownCounter),
	}
}

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
		values:      nil,
	}
	m.counters[name] = c
	return c
}

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
		values:      nil,
	}
	m.histograms[name] = h
	return h
}

func (m *FakeMetrics) HistogramWithBuckets(name, description, unit string, buckets []float64) observability.Histogram {
	return m.Histogram(name, description, unit)
}

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
		values:      nil,
	}
	m.upDowns[name] = u
	return u
}

func (m *FakeMetrics) Gauge(name, description, unit string, callback observability.GaugeCallback) error {
	return nil
}

func (m *FakeMetrics) GetCounter(name string) *FakeCounter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[name]
}

func (m *FakeMetrics) GetHistogram(name string) *FakeHistogram {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.histograms[name]
}

func (m *FakeMetrics) GetUpDownCounter(name string) *FakeUpDownCounter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upDowns[name]
}

type FakeCounter struct {
	mu          sync.RWMutex
	Name        string
	Description string
	Unit        string
	values      []CounterValue
}

func (c *FakeCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = append(c.values, CounterValue{
		Value:     value,
		Fields:    fields,
		Timestamp: time.Now(),
	})
}

func (c *FakeCounter) Increment(ctx context.Context, fields ...observability.Field) {
	c.Add(ctx, 1, fields...)
}

func (c *FakeCounter) GetValues() []CounterValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]CounterValue, len(c.values))
	copy(result, c.values)
	return result
}

type CounterValue struct {
	Value     int64
	Fields    []observability.Field
	Timestamp time.Time
}

type FakeHistogram struct {
	mu          sync.RWMutex
	Name        string
	Description string
	Unit        string
	values      []HistogramValue
}

func (h *FakeHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, HistogramValue{
		Value:     value,
		Fields:    fields,
		Timestamp: time.Now(),
	})
}

func (h *FakeHistogram) GetValues() []HistogramValue {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]HistogramValue, len(h.values))
	copy(result, h.values)
	return result
}

type HistogramValue struct {
	Value     float64
	Fields    []observability.Field
	Timestamp time.Time
}

type FakeUpDownCounter struct {
	mu          sync.RWMutex
	Name        string
	Description string
	Unit        string
	values      []CounterValue
}

func (u *FakeUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.values = append(u.values, CounterValue{
		Value:     value,
		Fields:    fields,
		Timestamp: time.Now(),
	})
}

func (u *FakeUpDownCounter) GetValues() []CounterValue {
	u.mu.RLock()
	defer u.mu.RUnlock()
	result := make([]CounterValue, len(u.values))
	copy(result, u.values)
	return result
}
