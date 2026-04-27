//go:build integration

package postgres_otelsql

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
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestIntegrationDBManagerQueryUsesPropagatedTrace(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgresContainer(t, ctx)

	recorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	previousTracerProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTracerProvider)
		require.NoError(t, tracerProvider.Shutdown(context.Background()))
	})

	manager, err := NewDBManager(ctx, DefaultConfig(dsn, "postgres-otelsql-test"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, manager.Shutdown(context.Background()))
	})

	rootCtx, root := tracerProvider.Tracer("integration-test").Start(ctx, "HTTP GET /users")
	_, err = manager.DB().ExecContext(rootCtx, "CREATE TEMP TABLE users(id INT PRIMARY KEY)")
	require.NoError(t, err)
	_, err = manager.DB().ExecContext(rootCtx, "INSERT INTO users(id) VALUES($1)", 1)
	require.NoError(t, err)
	var id int
	require.NoError(t, manager.DB().QueryRowContext(rootCtx, "SELECT id FROM users WHERE id = $1", 1).Scan(&id))
	root.End()

	beforeRootOnly := len(recorder.Ended())
	_, err = manager.DB().ExecContext(context.Background(), "SELECT 1")
	require.NoError(t, err)
	afterBackgroundQuery := recorder.Ended()

	assert.Equal(t, 1, id)
	assert.GreaterOrEqual(t, beforeRootOnly, 4)
	assert.Len(t, afterBackgroundQuery, beforeRootOnly, "background DB query must not create a root span")
	assert.Contains(t, spanNames(afterBackgroundQuery), "db.client.operation SELECT")
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
