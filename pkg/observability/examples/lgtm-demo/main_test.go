package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want config
	}{
		{
			name: "uses explicit container configuration",
			env: map[string]string{
				"APP_ADDR":                    ":9090",
				"APP_ENVIRONMENT":             "staging",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "collector:4317",
				"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
				"OTEL_SERVICE_NAME":           "custom-demo",
				"OTEL_SERVICE_VERSION":        "1.2.3",
				"DATABASE_DSN":                "postgres://user:pass@db:5432/demo?sslmode=disable",
			},
			want: config{
				Addr:           ":9090",
				ServiceName:    "custom-demo",
				ServiceVersion: "1.2.3",
				Environment:    "staging",
				OTLPEndpoint:   "collector:4317",
				OTLPProtocol:   otel.ProtocolGRPC,
				DatabaseDSN:    "postgres://user:pass@db:5432/demo?sslmode=disable",
			},
		},
		{
			name: "falls back to local defaults",
			env:  map[string]string{},
			want: config{
				Addr:           defaultAddr,
				ServiceName:    defaultService,
				ServiceVersion: defaultVersion,
				Environment:    defaultEnv,
				OTLPEndpoint:   defaultEndpoint,
				OTLPProtocol:   otel.ProtocolGRPC,
				DatabaseDSN:    defaultDBDSN,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{
				"APP_ADDR",
				"APP_ENVIRONMENT",
				"OTEL_EXPORTER_OTLP_ENDPOINT",
				"OTEL_EXPORTER_OTLP_PROTOCOL",
				"OTEL_SERVICE_NAME",
				"OTEL_SERVICE_VERSION",
				"DATABASE_DSN",
			} {
				t.Setenv(key, "")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			assert.Equal(t, tt.want, configFromEnv())
		})
	}
}

func TestDemoServerLookupUser(t *testing.T) {
	tests := []struct {
		name    string
		ctx     func() context.Context
		userID  string
		wantErr error
	}{
		{
			name:   "returns user after simulated db span",
			ctx:    context.Background,
			userID: "42",
		},
		{
			name:    "returns not found for missing user",
			ctx:     context.Background,
			userID:  "missing",
			wantErr: errors.New("not found"),
		},
		{
			name: "respects canceled context before simulated db completes",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			userID:  "42",
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := fake.NewProvider()
			server := newDemoServer(obs)

			user, err := server.lookupUser(tt.ctx(), tt.userID)

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.Equal(t, tt.userID, user.ID)
				return
			}
			require.Error(t, err)
			assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()))
		})
	}
}

func TestDemoServerHTTPFlow(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantSpans  int
	}{
		{
			name:       "successful user request emits http and db spans",
			target:     "/users?id=42",
			wantStatus: http.StatusOK,
			wantSpans:  2,
		},
		{
			name:       "missing id returns bad request with http span",
			target:     "/users",
			wantStatus: http.StatusBadRequest,
			wantSpans:  1,
		},
		{
			name:       "not found user returns error with http and db spans",
			target:     "/users?id=missing",
			wantStatus: http.StatusNotFound,
			wantSpans:  2,
		},
		{
			name:       "failure endpoint returns server error with http span",
			target:     "/fail",
			wantStatus: http.StatusInternalServerError,
			wantSpans:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := fake.NewProvider()
			server := newDemoServer(obs)
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req.Header.Set("x-request-id", "req-123")
			req.Header.Set("correlation-id", "corr-123")
			rec := httptest.NewRecorder()

			server.routes().ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			tracer := obs.Tracer().(*fake.FakeTracer)
			assert.Len(t, tracer.GetSpans(), tt.wantSpans)
		})
	}
}
