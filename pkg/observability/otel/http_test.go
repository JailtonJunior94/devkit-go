package otel

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHTTPInstrumentationStartRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		req               HTTPRequest
		resp              HTTPResponse
		err               error
		wantSpanName      string
		wantStatus        codes.Code
		wantRequestID     string
		wantCorrelationID string
		wantMetricRoute   string
		wantErrorCount    int
	}{
		{
			name: "success status records span metrics and correlation",
			req: HTTPRequest{
				Method:        "get",
				Route:         "/users/{id}",
				Target:        "/users/123",
				RemoteAddr:    "10.0.0.1",
				UserAgent:     "devkit-test",
				RequestID:     " req-123 ",
				CorrelationID: " corr-456 ",
			},
			resp:              HTTPResponse{StatusCode: 200, Bytes: 128},
			wantSpanName:      "GET /users/{id}",
			wantStatus:        codes.Ok,
			wantRequestID:     "req-123",
			wantCorrelationID: "corr-456",
			wantMetricRoute:   "/users/{id}",
		},
		{
			name: "explicit error records span error status and error metric",
			req: HTTPRequest{
				Method:        "POST",
				Route:         "/orders",
				RequestID:     "req-err",
				CorrelationID: "corr-err",
			},
			resp:              HTTPResponse{StatusCode: 500},
			err:               errors.New("handler failed"),
			wantSpanName:      "POST /orders",
			wantStatus:        codes.Error,
			wantRequestID:     "req-err",
			wantCorrelationID: "corr-err",
			wantMetricRoute:   "/orders",
			wantErrorCount:    1,
		},
		{
			name: "absent correlation identifiers still creates traceable context",
			req: HTTPRequest{
				Method: "GET",
				Target: "/health",
			},
			resp:            HTTPResponse{StatusCode: 204},
			wantSpanName:    "HTTP GET",
			wantStatus:      codes.Ok,
			wantMetricRoute: httpUnknownRoute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer, spans := newHTTPTestTracer()
			metrics := newRecordingMetrics()
			instrumentation := newHTTPInstrumentation(tracer, metrics)

			ctx, scope := instrumentation.StartRequest(context.Background(), tt.req)
			correlation, ok := CorrelationFromContext(ctx)
			require.True(t, ok)
			assert.NotEmpty(t, correlation.TraceID)
			assert.NotEmpty(t, correlation.SpanID)
			assert.Equal(t, tt.wantRequestID, correlation.RequestID)
			assert.Equal(t, tt.wantCorrelationID, correlation.CorrelationID)
			assert.True(t, correlation.Sampled)

			if tt.err != nil {
				scope.OnError(tt.err)
			}
			scope.Finish(tt.resp)
			scope.OnError(errors.New("late error"))
			scope.Finish(HTTPResponse{StatusCode: 599})

			ended := spans.Ended()
			require.Len(t, ended, 1)
			assert.Equal(t, tt.wantSpanName, ended[0].Name())
			assert.Equal(t, tt.wantStatus, ended[0].Status().Code)

			attrs := spanAttrs(ended[0].Attributes())
			assert.Equal(t, tt.req.normalized().Method, attrs["http.request.method"])
			if tt.req.Route != "" {
				assert.Equal(t, tt.req.Route, attrs["http.route"])
			}
			assert.Equal(t, tt.wantRequestID, stringAttr(attrs, "request_id"))
			assert.Equal(t, tt.wantCorrelationID, stringAttr(attrs, "correlation_id"))
			assert.Equal(t, tt.resp.normalized().StatusCode, attrs["http.response.status_code"])

			assert.Len(t, metrics.histograms[httpRequestDurationMetric].records, 1)
			assert.Len(t, metrics.counters[httpRequestCountMetric].adds, 1)
			assert.Len(t, metrics.upDowns[httpRequestActiveMetric].adds, 2)
			assert.Equal(t, int64(1), metrics.upDowns[httpRequestActiveMetric].adds[0].value)
			assert.Equal(t, int64(-1), metrics.upDowns[httpRequestActiveMetric].adds[1].value)
			assert.Equal(t, tt.wantMetricRoute, fieldValue(metrics.counters[httpRequestCountMetric].adds[0].fields, "http.route"))
			assert.Len(t, metrics.counters[httpRequestErrorMetric].adds, tt.wantErrorCount)
		})
	}
}

func TestHTTPInstrumentationScopeSetRouteRebindsMetricsAndSpanAttrs(t *testing.T) {
	t.Parallel()

	tracer, spans := newHTTPTestTracer()
	metrics := newRecordingMetrics()
	instrumentation := newHTTPInstrumentation(tracer, metrics)

	_, scope := instrumentation.StartRequest(context.Background(), HTTPRequest{
		Method: "GET",
		Route:  "unmatched",
		Target: "/users/123",
	})

	setter, ok := scope.(interface{ SetRoute(string) })
	require.True(t, ok)

	setter.SetRoute("/users/{id}")
	scope.Finish(HTTPResponse{StatusCode: 200})

	ended := spans.Ended()
	require.Len(t, ended, 1)

	attrs := spanAttrs(ended[0].Attributes())
	assert.Equal(t, "/users/{id}", attrs["http.route"])
	assert.Equal(t, "/users/{id}", fieldValue(metrics.counters[httpRequestCountMetric].adds[0].fields, "http.route"))

	require.Len(t, metrics.upDowns[httpRequestActiveMetric].adds, 4)
	assert.Equal(t, int64(1), metrics.upDowns[httpRequestActiveMetric].adds[0].value)
	assert.Equal(t, "unmatched", fieldValue(metrics.upDowns[httpRequestActiveMetric].adds[0].fields, "http.route"))
	assert.Equal(t, int64(-1), metrics.upDowns[httpRequestActiveMetric].adds[1].value)
	assert.Equal(t, "unmatched", fieldValue(metrics.upDowns[httpRequestActiveMetric].adds[1].fields, "http.route"))
	assert.Equal(t, int64(1), metrics.upDowns[httpRequestActiveMetric].adds[2].value)
	assert.Equal(t, "/users/{id}", fieldValue(metrics.upDowns[httpRequestActiveMetric].adds[2].fields, "http.route"))
	assert.Equal(t, int64(-1), metrics.upDowns[httpRequestActiveMetric].adds[3].value)
	assert.Equal(t, "/users/{id}", fieldValue(metrics.upDowns[httpRequestActiveMetric].adds[3].fields, "http.route"))
}

func TestHTTPInstrumentationProviderHook(t *testing.T) {
	t.Parallel()

	assert.IsType(t, noopHTTPInstrumentation{}, (*Provider)(nil).HTTP())

	tracer, _ := newHTTPTestTracer()
	metrics := newRecordingMetrics()
	hook := newHTTPInstrumentation(tracer, metrics)
	provider := &Provider{runtime: &runtime{http: hook}}

	assert.Same(t, hook, provider.HTTP())
}

func newHTTPTestTracer() (*otelTracer, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(recorder),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	return newOtelTracer(provider.Tracer("http-test")), recorder
}

func spanAttrs(attrs []attribute.KeyValue) map[string]any {
	result := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		switch attr.Value.Type() {
		case attribute.STRING:
			result[string(attr.Key)] = attr.Value.AsString()
		case attribute.INT64:
			result[string(attr.Key)] = int(attr.Value.AsInt64())
		default:
			result[string(attr.Key)] = attr.Value.AsInterface()
		}
	}
	return result
}

func stringAttr(attrs map[string]any, key string) string {
	value, _ := attrs[key].(string)
	return value
}

func fieldValue(fields []observability.Field, key string) string {
	for _, field := range fields {
		if field.Key == key {
			return field.StringValue()
		}
	}
	return ""
}

type recordingMetrics struct {
	counters   map[string]*recordingCounter
	histograms map[string]*recordingHistogram
	upDowns    map[string]*recordingUpDownCounter
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{
		counters:   map[string]*recordingCounter{},
		histograms: map[string]*recordingHistogram{},
		upDowns:    map[string]*recordingUpDownCounter{},
	}
}

func (m *recordingMetrics) Counter(name, _, _ string) observability.Counter {
	counter := &recordingCounter{}
	m.counters[name] = counter
	return counter
}

func (m *recordingMetrics) Histogram(name, _, _ string) observability.Histogram {
	histogram := &recordingHistogram{}
	m.histograms[name] = histogram
	return histogram
}

func (m *recordingMetrics) HistogramWithBuckets(name, description, unit string, _ []float64) observability.Histogram {
	return m.Histogram(name, description, unit)
}

func (m *recordingMetrics) UpDownCounter(name, _, _ string) observability.UpDownCounter {
	upDown := &recordingUpDownCounter{}
	m.upDowns[name] = upDown
	return upDown
}

func (m *recordingMetrics) Gauge(string, string, string, observability.GaugeCallback) error {
	return nil
}

type metricAdd struct {
	value  int64
	fields []observability.Field
}

type recordingCounter struct {
	adds []metricAdd
}

func (c *recordingCounter) Add(_ context.Context, value int64, fields ...observability.Field) {
	c.adds = append(c.adds, metricAdd{value: value, fields: append([]observability.Field(nil), fields...)})
}

func (c *recordingCounter) Increment(ctx context.Context, fields ...observability.Field) {
	c.Add(ctx, 1, fields...)
}

type histogramRecord struct {
	value  float64
	fields []observability.Field
}

type recordingHistogram struct {
	records []histogramRecord
}

func (h *recordingHistogram) Record(_ context.Context, value float64, fields ...observability.Field) {
	h.records = append(h.records, histogramRecord{value: value, fields: append([]observability.Field(nil), fields...)})
}

type recordingUpDownCounter struct {
	adds []metricAdd
}

func (u *recordingUpDownCounter) Add(_ context.Context, value int64, fields ...observability.Field) {
	u.adds = append(u.adds, metricAdd{value: value, fields: append([]observability.Field(nil), fields...)})
}
