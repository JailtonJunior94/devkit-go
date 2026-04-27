package pgxpool_manager

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestOtelTracerTraceQuery(t *testing.T) {
	parent := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: oteltrace.TraceID{1},
		SpanID:  oteltrace.SpanID{1},
	})
	tracedCtx := oteltrace.ContextWithSpanContext(context.Background(), parent)

	tests := []struct {
		name        string
		ctx         context.Context
		sql         string
		err         error
		traceSpans  bool
		wantSpan    bool
		wantName    string
		wantStatus  codes.Code
		wantMetric  bool
		wantOp      string
		wantErrAttr bool
	}{
		{
			name:       "query inherits propagated trace",
			ctx:        tracedCtx,
			sql:        "select id from users where id = $1",
			traceSpans: true,
			wantSpan:   true,
			wantName:   "db.client.operation SELECT",
			wantStatus: codes.Ok,
			wantMetric: true,
			wantOp:     "SELECT",
		},
		{
			name:       "query without trace only records metrics",
			ctx:        context.Background(),
			sql:        "insert into users(id) values($1)",
			traceSpans: true,
			wantSpan:   false,
			wantMetric: true,
			wantOp:     "INSERT",
		},
		{
			name:        "query error is recorded on span and metric",
			ctx:         tracedCtx,
			sql:         "update users set name = $1",
			err:         errors.New("database unavailable"),
			traceSpans:  true,
			wantSpan:    true,
			wantName:    "db.client.operation UPDATE",
			wantStatus:  codes.Error,
			wantMetric:  true,
			wantOp:      "UPDATE",
			wantErrAttr: true,
		},
		{
			name:       "tracing disabled keeps metrics without span",
			ctx:        tracedCtx,
			sql:        "delete from users where id = $1",
			traceSpans: false,
			wantSpan:   false,
			wantMetric: true,
			wantOp:     "DELETE",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			recorder := tracetest.NewSpanRecorder()
			tracerProvider := trace.NewTracerProvider(
				trace.WithSampler(trace.AlwaysSample()),
				trace.WithSpanProcessor(recorder),
			)
			reader := metric.NewManualReader()
			meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
			previousMeterProvider := otel.GetMeterProvider()
			otel.SetMeterProvider(meterProvider)
			t.Cleanup(func() {
				otel.SetMeterProvider(previousMeterProvider)
				require.NoError(t, meterProvider.Shutdown(context.Background()))
				require.NoError(t, tracerProvider.Shutdown(context.Background()))
			})

			tracer := &otelTracer{
				tracer:     tracerProvider.Tracer("pgx-test"),
				metrics:    newDBMetrics(DefaultConfig("postgres://user:pass@localhost:5432/app", "svc")),
				traceSpans: tt.traceSpans,
			}

			ctx := tracer.TraceQueryStart(tt.ctx, nil, pgx.TraceQueryStartData{SQL: tt.sql})
			tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{
				CommandTag: pgconn.NewCommandTag("SELECT 1"),
				Err:        tt.err,
			})

			ended := recorder.Ended()
			if tt.wantSpan {
				require.Len(t, ended, 1)
				span := ended[0]
				assert.Equal(t, tt.wantName, span.Name())
				assert.Equal(t, tt.wantStatus, span.Status().Code)
				assert.Equal(t, parent.TraceID(), span.Parent().TraceID())
				assertSpanAttribute(t, span.Attributes(), "db.operation.name", tt.wantOp)
				assertSpanAttribute(t, span.Attributes(), "component", "pgxpool_manager")
			} else {
				assert.Empty(t, ended)
			}

			if tt.wantMetric {
				var rm metricdata.ResourceMetrics
				require.NoError(t, reader.Collect(context.Background(), &rm))
				assertMetricDataPoint(t, rm, dbOperationDurationMetric, tt.wantOp, tt.wantErrAttr)
				assertMetricDataPoint(t, rm, dbOperationCountMetric, tt.wantOp, tt.wantErrAttr)
			}
		})
	}
}

func TestValidateConfigQueryLoggingProductionGuard(t *testing.T) {
	t.Parallel()

	baseConfig := func(env string) *Config {
		cfg := DefaultConfig("postgres://user:pass@localhost:5432/app", "svc")
		cfg.Environment = env
		cfg.EnableQueryLogging = true
		return cfg
	}

	t.Run("rejects production environment", func(t *testing.T) {
		t.Parallel()

		err := validateConfig(baseConfig("production"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "EnableQueryLogging=true is not allowed in production")
	})

	t.Run("rejects prod alias", func(t *testing.T) {
		t.Parallel()

		err := validateConfig(baseConfig("Prod"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "EnableQueryLogging=true is not allowed in production")
	})

	t.Run("non-production emits warning", func(t *testing.T) {
		t.Parallel()

		var captured []string
		cfg := baseConfig("development")
		cfg.Logger = func(format string, args ...any) {
			captured = append(captured, format)
		}

		require.NoError(t, validateConfig(cfg))
		require.NotEmpty(t, captured)
		assert.Contains(t, captured[len(captured)-1], "EnableQueryLogging=true")
		assert.Contains(t, captured[len(captured)-1], "sensitive data")
	})

	t.Run("query logging disabled is silent", func(t *testing.T) {
		t.Parallel()

		var captured []string
		cfg := DefaultConfig("postgres://user:pass@localhost:5432/app", "svc")
		cfg.Environment = "production"
		cfg.EnableQueryLogging = false
		cfg.Logger = func(format string, args ...any) {
			captured = append(captured, format)
		}

		require.NoError(t, validateConfig(cfg))
		for _, msg := range captured {
			assert.NotContains(t, msg, "EnableQueryLogging=true")
		}
	})
}

func TestExtractOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sql  string
		want string
	}{
		{name: "select", sql: " select * from users", want: "SELECT"},
		{name: "cte", sql: "WITH active AS (select 1) select * from active", want: "SELECT"},
		{name: "ddl", sql: "alter table users add column name text", want: "DDL"},
		{name: "transaction", sql: "commit", want: "TRANSACTION"},
		{name: "empty", sql: " \t\n", want: "UNKNOWN"},
		{name: "other", sql: "explain select 1", want: "OTHER"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, extractOperation(tt.sql))
		})
	}
}

func assertSpanAttribute(t *testing.T, attrs []attribute.KeyValue, key string, want string) {
	t.Helper()

	for _, attr := range attrs {
		if string(attr.Key) == key {
			assert.Equal(t, want, attr.Value.AsString())
			return
		}
	}
	t.Fatalf("attribute %q not found in %v", key, attrs)
}

func assertMetricDataPoint(t *testing.T, rm metricdata.ResourceMetrics, metricName, operation string, wantErrAttr bool) {
	t.Helper()

	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metricName {
				continue
			}
			switch data := m.Data.(type) {
			case metricdata.Histogram[float64]:
				require.NotEmpty(t, data.DataPoints)
				assertMetricAttrs(t, data.DataPoints[0].Attributes.ToSlice(), operation, wantErrAttr)
				return
			case metricdata.Sum[int64]:
				require.NotEmpty(t, data.DataPoints)
				assertMetricAttrs(t, data.DataPoints[0].Attributes.ToSlice(), operation, wantErrAttr)
				return
			default:
				t.Fatalf("unexpected metric data type %T for %s", m.Data, metricName)
			}
		}
	}
	t.Fatalf("metric %q not found in %+v", metricName, rm)
}

func assertMetricAttrs(t *testing.T, attrs []attribute.KeyValue, operation string, wantErrAttr bool) {
	t.Helper()

	assertSpanAttribute(t, attrs, "db.operation.name", operation)
	assertSpanAttribute(t, attrs, "component", "pgxpool_manager")
	hasErrAttr := false
	for _, attr := range attrs {
		if string(attr.Key) == "error.type" {
			hasErrAttr = true
		}
	}
	assert.Equal(t, wantErrAttr, hasErrAttr)
}
