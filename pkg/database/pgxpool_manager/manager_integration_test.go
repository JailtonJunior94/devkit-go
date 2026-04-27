//go:build integration

package pgxpool_manager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestIntegrationPgxPoolManagerQueryUsesPropagatedTraceAndMetrics(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgresContainer(t, ctx)

	recorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousTracerProvider := otel.GetTracerProvider()
	previousMeterProvider := otel.GetMeterProvider()
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTracerProvider)
		otel.SetMeterProvider(previousMeterProvider)
		require.NoError(t, meterProvider.Shutdown(context.Background()))
		require.NoError(t, tracerProvider.Shutdown(context.Background()))
	})

	manager, err := NewPgxPoolManager(ctx, DefaultConfig(dsn, "pgxpool-test"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, manager.Shutdown(context.Background()))
	})

	rootCtx, root := tracerProvider.Tracer("integration-test").Start(ctx, "HTTP GET /users")
	_, err = manager.Pool().Exec(rootCtx, "CREATE TEMP TABLE users(id INT PRIMARY KEY)")
	require.NoError(t, err)
	_, err = manager.Pool().Exec(rootCtx, "INSERT INTO users(id) VALUES($1)", 1)
	require.NoError(t, err)
	var id int
	require.NoError(t, manager.Pool().QueryRow(rootCtx, "SELECT id FROM users WHERE id = $1", 1).Scan(&id))
	root.End()

	beforeRootOnly := len(recorder.Ended())
	_, err = manager.Pool().Exec(context.Background(), "SELECT 1")
	require.NoError(t, err)
	afterBackgroundQuery := recorder.Ended()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))

	assert.Equal(t, 1, id)
	assert.GreaterOrEqual(t, beforeRootOnly, 4)
	assert.Len(t, afterBackgroundQuery, beforeRootOnly, "background DB query must not create a root span")
	assert.Contains(t, spanNames(afterBackgroundQuery), "db.client.operation SELECT")
	assertMetricPresent(t, rm, dbOperationDurationMetric)
	assertMetricPresent(t, rm, dbOperationCountMetric)
}

func startPostgresContainer(t *testing.T, ctx context.Context) string {
	t.Helper()

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, container.Terminate(context.Background()))
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

func spanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name())
	}
	return names
}

func assertMetricPresent(t *testing.T, rm metricdata.ResourceMetrics, name string) {
	t.Helper()

	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == name {
				return
			}
		}
	}
	t.Fatalf("metric %q not found in %+v", name, rm)
}
