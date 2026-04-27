package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/pgxpool_manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

const (
	defaultAddr       = ":8080"
	defaultEndpoint   = "otel-collector:4317"
	defaultService    = "devkit-go-lgtm-demo"
	defaultVersion    = "0.1.0"
	defaultEnv        = "local"
	defaultDBDSN      = ""
	shutdownTimeout   = 10 * time.Second
	simulatedDBDelay  = 25 * time.Millisecond
	readHeaderTimeout = 5 * time.Second
)

type config struct {
	Addr           string
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	OTLPProtocol   otel.OTLPProtocol
	DatabaseDSN    string
}

func configFromEnv() config {
	return config{
		Addr:           envOrDefault("APP_ADDR", defaultAddr),
		ServiceName:    envOrDefault("OTEL_SERVICE_NAME", defaultService),
		ServiceVersion: envOrDefault("OTEL_SERVICE_VERSION", defaultVersion),
		Environment:    envOrDefault("APP_ENVIRONMENT", defaultEnv),
		OTLPEndpoint:   envOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", defaultEndpoint),
		OTLPProtocol:   otel.OTLPProtocol(envOrDefault("OTEL_EXPORTER_OTLP_PROTOCOL", string(otel.ProtocolGRPC))),
		DatabaseDSN:    envOrDefault("DATABASE_DSN", defaultDBDSN),
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func newObservability(ctx context.Context, cfg config) (observability.Observability, error) {
	return otel.NewProvider(ctx, &otel.Config{
		ServiceName:          cfg.ServiceName,
		ServiceVersion:       cfg.ServiceVersion,
		Environment:          cfg.Environment,
		OTLPEndpoint:         cfg.OTLPEndpoint,
		OTLPProtocol:         cfg.OTLPProtocol,
		Insecure:             true,
		TraceSampleRate:      1,
		LogLevel:             observability.LogLevelInfo,
		LogFormat:            observability.LogFormatJSON,
		MetricExportInterval: 5,
		ConsoleLog:           true,
	})
}

type demoServer struct {
	db        *pgxpool_manager.PgxPoolManager
	obs       observability.Observability
	requests  observability.Counter
	errors    observability.Counter
	latency   observability.Histogram
	dbLatency observability.Histogram
}

func newDemoServer(obs observability.Observability, dbManagers ...*pgxpool_manager.PgxPoolManager) *demoServer {
	metrics := obs.Metrics()
	server := &demoServer{
		obs:       obs,
		requests:  metrics.Counter("lgtm_demo.http.requests", "LGTM demo HTTP requests", "{request}"),
		errors:    metrics.Counter("lgtm_demo.http.errors", "LGTM demo HTTP errors", "{error}"),
		latency:   metrics.Histogram("lgtm_demo.http.duration", "LGTM demo HTTP duration", "s"),
		dbLatency: metrics.Histogram("lgtm_demo.db.duration", "LGTM demo simulated DB duration", "s"),
	}
	if len(dbManagers) > 0 {
		server.db = dbManagers[0]
	}
	return server
}

func (s *demoServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", s.handleLive)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/users", s.handleUsers)
	mux.HandleFunc("/fail", s.handleFailure)
	return mux
}

func (s *demoServer) handleLive(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *demoServer) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *demoServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	ctx, span := s.obs.Tracer().Start(r.Context(), "GET /users",
		observability.WithSpanKind(observability.SpanKindServer),
		observability.WithAttributes(
			observability.String("http.request.method", r.Method),
			observability.String("http.route", "/users"),
			observability.String("request_id", r.Header.Get("x-request-id")),
			observability.String("correlation_id", r.Header.Get("correlation-id")),
		),
	)
	ctx = otel.ContextWithCorrelation(ctx, otel.CorrelationContext{
		RequestID:     r.Header.Get("x-request-id"),
		CorrelationID: r.Header.Get("correlation-id"),
	})
	defer span.End()

	userID := strings.TrimSpace(r.URL.Query().Get("id"))
	if userID == "" {
		s.writeError(ctx, w, span, http.StatusBadRequest, errors.New("missing id query parameter"))
		s.recordRequest(ctx, r.Method, "/users", http.StatusBadRequest, startedAt)
		return
	}

	user, err := s.lookupUser(ctx, userID)
	if err != nil {
		s.writeError(ctx, w, span, http.StatusNotFound, err)
		s.recordRequest(ctx, r.Method, "/users", http.StatusNotFound, startedAt)
		return
	}

	s.obs.Logger().Info(ctx, "lgtm demo request completed",
		observability.String("component", "lgtm-demo"),
		observability.String("user_id", user.ID),
	)
	span.SetStatus(observability.StatusCodeOK, "")
	span.SetAttributes(observability.Int("http.response.status_code", http.StatusOK))
	s.recordRequest(ctx, r.Method, "/users", http.StatusOK, startedAt)
	writeJSON(w, http.StatusOK, user)
}

func (s *demoServer) handleFailure(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.obs.Tracer().Start(r.Context(), "GET /fail",
		observability.WithSpanKind(observability.SpanKindServer),
		observability.WithAttributes(
			observability.String("http.request.method", r.Method),
			observability.String("http.route", "/fail"),
		),
	)
	ctx = otel.ContextWithCorrelation(ctx, otel.CorrelationContext{
		RequestID:     r.Header.Get("x-request-id"),
		CorrelationID: r.Header.Get("correlation-id"),
	})
	defer span.End()

	err := errors.New("intentional lgtm demo failure")
	s.writeError(ctx, w, span, http.StatusInternalServerError, err)
	s.recordRequest(ctx, r.Method, "/fail", http.StatusInternalServerError, time.Now())
}

type user struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *demoServer) lookupUser(ctx context.Context, id string) (user, error) {
	if s.db != nil {
		return s.lookupUserPostgres(ctx, id)
	}

	startedAt := time.Now()
	ctx, span := s.obs.Tracer().Start(ctx, "demo.db.lookup_user",
		observability.WithSpanKind(observability.SpanKindClient),
		observability.WithAttributes(
			observability.String("db.system.name", "in-memory"),
			observability.String("db.operation.name", "lookup_user"),
		),
	)
	defer span.End()

	timer := time.NewTimer(simulatedDBDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		err := fmt.Errorf("lookup user: %w", ctx.Err())
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())
		return user{}, err
	case <-timer.C:
	}

	s.dbLatency.Record(ctx, time.Since(startedAt).Seconds(),
		observability.String("db.operation.name", "lookup_user"),
	)

	if id == "missing" {
		err := fmt.Errorf("user %q not found", id)
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())
		return user{}, err
	}

	span.SetStatus(observability.StatusCodeOK, "")
	return user{
		ID:    id,
		Name:  "LGTM Demo User",
		Email: "demo@example.com",
	}, nil
}

func (s *demoServer) lookupUserPostgres(ctx context.Context, id string) (user, error) {
	var found user
	err := s.db.Pool().QueryRow(ctx,
		"SELECT id, name, email FROM users WHERE id = $1",
		id,
	).Scan(&found.ID, &found.Name, &found.Email)
	if err != nil {
		return user{}, fmt.Errorf("lookup user: %w", err)
	}
	return found, nil
}

func setupDatabase(ctx context.Context, dsn, serviceName string) (*pgxpool_manager.PgxPoolManager, error) {
	cfg := pgxpool_manager.DefaultConfig(dsn, serviceName)
	manager, err := pgxpool_manager.NewPgxPoolManager(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if _, err := manager.Pool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL
		)
	`); err != nil {
		_ = manager.Shutdown(ctx)
		return nil, fmt.Errorf("create users table: %w", err)
	}

	if _, err := manager.Pool().Exec(ctx, `
		INSERT INTO users (id, name, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email
	`, "42", "LGTM Demo User", "demo@example.com"); err != nil {
		_ = manager.Shutdown(ctx)
		return nil, fmt.Errorf("seed users table: %w", err)
	}

	return manager, nil
}

func (s *demoServer) writeError(ctx context.Context, w http.ResponseWriter, span observability.Span, status int, err error) {
	span.RecordError(err)
	span.SetStatus(observability.StatusCodeError, err.Error())
	span.SetAttributes(observability.Int("http.response.status_code", status))
	s.errors.Increment(ctx,
		observability.Int("http.response.status_code", status),
	)
	s.obs.Logger().Error(ctx, "lgtm demo request failed",
		observability.String("component", "lgtm-demo"),
		observability.Error(err),
		observability.Int("http.response.status_code", status),
	)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *demoServer) recordRequest(ctx context.Context, method, route string, status int, startedAt time.Time) {
	fields := []observability.Field{
		observability.String("http.request.method", method),
		observability.String("http.route", route),
		observability.Int("http.response.status_code", status),
	}
	s.requests.Increment(ctx, fields...)
	s.latency.Record(ctx, time.Since(startedAt).Seconds(), fields...)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func main() {
	cfg := configFromEnv()
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	obs, err := newObservability(rootCtx, cfg)
	if err != nil {
		stop()
		slog.Error("failed to initialize observability", "error", err)
		os.Exit(1)
	}
	defer stop()

	var dbManager *pgxpool_manager.PgxPoolManager
	if cfg.DatabaseDSN != "" {
		dbManager, err = setupDatabase(rootCtx, cfg.DatabaseDSN, cfg.ServiceName)
		if err != nil {
			slog.Error("failed to initialize database", "error", err)
			_ = obs.Shutdown(rootCtx)
			stop()
			return
		}
	}

	demo := newDemoServer(obs, dbManager)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           demo.routes(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting lgtm demo", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-rootCtx.Done():
	case err := <-errCh:
		if err != nil {
			slog.Error("lgtm demo stopped with error", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown http server", "error", err)
	}
	if dbManager != nil {
		if err := dbManager.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown database", "error", err)
		}
	}
	if err := obs.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown observability", "error", err)
	}
}
