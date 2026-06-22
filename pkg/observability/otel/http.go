package otel

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

const (
	httpRequestDurationMetric = "http.server.request.duration"
	httpRequestCountMetric    = "http.server.request.count"
	httpRequestActiveMetric   = "http.server.request.active"
	httpRequestErrorMetric    = "http.server.request.error.count"

	httpUnknownRoute = "unknown"
)

type HTTPInstrumentation interface {
	StartRequest(ctx context.Context, req HTTPRequest) (context.Context, HTTPRequestScope)
}

type HTTPRequestScope interface {
	OnError(err error)
	Finish(resp HTTPResponse)
}

type HTTPRequest struct {
	Method        string
	Route         string
	Target        string
	RemoteAddr    string
	UserAgent     string
	RequestID     string
	CorrelationID string
}

type HTTPResponse struct {
	StatusCode int
	Bytes      int64
}

type httpInstrumentation struct {
	tracer observability.Tracer

	duration observability.Histogram
	count    observability.Counter
	active   observability.UpDownCounter
	errors   observability.Counter
}

func newHTTPInstrumentation(tracer observability.Tracer, metrics observability.Metrics) HTTPInstrumentation {
	if tracer == nil || metrics == nil {
		return noopHTTPInstrumentation{}
	}

	return &httpInstrumentation{
		tracer: tracer,
		duration: metrics.HistogramWithBuckets(
			httpRequestDurationMetric,
			"HTTP server request duration",
			"s",
			[]float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0},
		),
		count: metrics.Counter(
			httpRequestCountMetric,
			"HTTP server request count",
			"{request}",
		),
		active: metrics.UpDownCounter(
			httpRequestActiveMetric,
			"Active HTTP server requests",
			"{request}",
		),
		errors: metrics.Counter(
			httpRequestErrorMetric,
			"HTTP server request error count",
			"{error}",
		),
	}
}

func (h *httpInstrumentation) StartRequest(ctx context.Context, req HTTPRequest) (context.Context, HTTPRequestScope) {
	req = req.normalized()
	fields := req.spanFields()

	ctx, span := h.tracer.Start(ctx, req.spanName(), observability.WithSpanKind(observability.SpanKindServer), observability.WithAttributes(fields...))
	correlation := CorrelationContext{
		RequestID:     req.RequestID,
		CorrelationID: req.CorrelationID,
	}.withSpan(span)
	ctx = ContextWithCorrelation(ctx, correlation)

	metricFields := req.metricFields()
	h.active.Add(ctx, 1, metricFields...)

	return ctx, &httpRequestScope{
		ctx:          ctx,
		span:         span,
		startedAt:    time.Now(),
		req:          req,
		metricFields: metricFields,
		duration:     h.duration,
		count:        h.count,
		active:       h.active,
		errors:       h.errors,
	}
}

type httpRequestScope struct {
	ctx          context.Context
	span         observability.Span
	startedAt    time.Time
	req          HTTPRequest
	metricFields []observability.Field
	duration     observability.Histogram
	count        observability.Counter
	active       observability.UpDownCounter
	errors       observability.Counter

	mu  sync.Mutex
	err error
	end bool
}

func (s *httpRequestScope) SetRoute(route string) {
	normalized := strings.TrimSpace(route)
	if normalized == "" || normalized == s.req.Route {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.end {
		return
	}

	oldFields := append([]observability.Field(nil), s.metricFields...)
	s.req.Route = normalized
	s.metricFields = s.req.metricFields()

	s.active.Add(s.ctx, -1, oldFields...)
	s.active.Add(s.ctx, 1, s.metricFields...)
	s.span.SetAttributes(observability.String("http.route", normalized))
}

func (s *httpRequestScope) OnError(err error) {
	if err == nil {
		return
	}

	s.mu.Lock()
	if s.end {
		s.mu.Unlock()
		return
	}
	if s.err == nil {
		s.err = err
	} else {
		s.err = errors.Join(s.err, err)
	}
	s.span.RecordError(err)
	s.span.SetStatus(observability.StatusCodeError, err.Error())
	s.mu.Unlock()
}

func (s *httpRequestScope) Finish(resp HTTPResponse) {
	resp = resp.normalized()

	s.mu.Lock()
	if s.end {
		s.mu.Unlock()
		return
	}
	s.end = true
	err := s.err
	s.mu.Unlock()

	durationSeconds := time.Since(s.startedAt).Seconds()
	status := strconv.Itoa(resp.StatusCode)
	fields := append([]observability.Field{}, s.metricFields...)
	fields = append(fields, observability.String("http.response.status_code", status))

	s.span.SetAttributes(resp.spanFields()...)
	s.duration.Record(s.ctx, durationSeconds, fields...)
	s.count.Increment(s.ctx, fields...)
	s.active.Add(s.ctx, -1, s.metricFields...)

	if err != nil || resp.StatusCode >= 500 {
		s.errors.Increment(s.ctx, fields...)
		if err == nil {
			s.span.SetStatus(observability.StatusCodeError, "http status "+status)
		}
	} else {
		s.span.SetStatus(observability.StatusCodeOK, "")
	}

	s.span.End()
}

func (r HTTPRequest) normalized() HTTPRequest {
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if r.Method == "" {
		r.Method = "HTTP"
	}
	r.Route = strings.TrimSpace(r.Route)
	r.Target = strings.TrimSpace(r.Target)
	r.RemoteAddr = strings.TrimSpace(r.RemoteAddr)
	r.UserAgent = strings.TrimSpace(r.UserAgent)
	r.RequestID = normalizeHeaderValue(r.RequestID)
	r.CorrelationID = normalizeHeaderValue(r.CorrelationID)
	return r
}

func (r HTTPRequest) spanName() string {
	if r.Route != "" {
		return r.Method + " " + r.Route
	}
	return "HTTP " + r.Method
}

func (r HTTPRequest) spanFields() []observability.Field {
	fields := []observability.Field{
		observability.String("http.request.method", r.Method),
	}
	if r.Route != "" {
		fields = append(fields, observability.String("http.route", r.Route))
	}
	if r.Target != "" {
		fields = append(fields, observability.String("url.path", r.Target))
	}
	if r.RemoteAddr != "" {
		fields = append(fields, observability.String("client.address", r.RemoteAddr))
	}
	if r.UserAgent != "" {
		fields = append(fields, observability.String("user_agent.original", r.UserAgent))
	}
	if r.RequestID != "" {
		fields = append(fields, observability.String("request_id", r.RequestID))
	}
	if r.CorrelationID != "" {
		fields = append(fields, observability.String("correlation_id", r.CorrelationID))
	}
	return fields
}

func (r HTTPRequest) metricFields() []observability.Field {
	route := r.Route
	if route == "" {
		route = httpUnknownRoute
	}
	return []observability.Field{
		observability.String("http.request.method", r.Method),
		observability.String("http.route", route),
	}
}

func (r HTTPResponse) normalized() HTTPResponse {
	if r.StatusCode <= 0 {
		r.StatusCode = 200
	}
	return r
}

func (r HTTPResponse) spanFields() []observability.Field {
	fields := []observability.Field{
		observability.Int("http.response.status_code", r.StatusCode),
	}
	if r.Bytes > 0 {
		fields = append(fields, observability.Int64("http.response.body.size", r.Bytes))
	}
	return fields
}

func (c CorrelationContext) withSpan(span observability.Span) CorrelationContext {
	c.TraceID = span.TraceID()
	c.SpanID = span.SpanID()
	c.Sampled = span.IsSampled()
	return c
}

type noopHTTPInstrumentation struct{}

func (noopHTTPInstrumentation) StartRequest(ctx context.Context, _ HTTPRequest) (context.Context, HTTPRequestScope) {
	return ctx, noopHTTPRequestScope{}
}

type noopHTTPRequestScope struct{}

func (noopHTTPRequestScope) OnError(_ error)       {}
func (noopHTTPRequestScope) Finish(_ HTTPResponse) {}
