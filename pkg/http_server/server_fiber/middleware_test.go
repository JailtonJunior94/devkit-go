package serverfiber

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	otelobs "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedObservabilityMiddleware_Fiber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		path              string
		handler           fiber.Handler
		wantStatus        int
		wantRoute         string
		wantRequestID     string
		wantCorrelationID string
		wantError         bool
	}{
		{
			name: "preserves correlation headers and finishes successful request",
			path: "/users/123",
			handler: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusAccepted).SendString("ok")
			},
			wantStatus:        fiber.StatusAccepted,
			wantRoute:         "/users/{param}",
			wantRequestID:     "req-123",
			wantCorrelationID: "corr-456",
		},
		{
			name: "records returned handler error",
			path: "/users/500",
			handler: func(_ *fiber.Ctx) error {
				return fiber.NewError(fiber.StatusInternalServerError, "handler failed")
			},
			wantStatus: fiber.StatusInternalServerError,
			wantRoute:  "/users/{param}",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hook := &recordingHTTPInstrumentation{}
			srv := newObservedFiberServer(t, hook, tt.handler)

			req := httptest.NewRequest(fiber.MethodGet, tt.path, nil)
			if tt.wantRequestID != "" {
				req.Header.Set("X-Request-ID", tt.wantRequestID)
			}
			if tt.wantCorrelationID != "" {
				req.Header.Set("Correlation-ID", tt.wantCorrelationID)
			}

			resp, err := srv.App().Test(req)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, resp.Body.Close())
			}()

			require.Len(t, hook.requests, 1)
			assert.Equal(t, fiber.MethodGet, hook.requests[0].Method)
			assert.Equal(t, tt.wantRoute, hook.requests[0].Route)
			assert.Equal(t, tt.path, hook.requests[0].Target)
			assert.Equal(t, tt.wantCorrelationID, hook.requests[0].CorrelationID)
			if tt.wantRequestID == "" {
				assert.NotEmpty(t, hook.requests[0].RequestID)
				assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
			} else {
				assert.Equal(t, tt.wantRequestID, hook.requests[0].RequestID)
				assert.Equal(t, tt.wantRequestID, resp.Header.Get("X-Request-ID"))
			}

			require.Len(t, hook.scopes, 1)
			require.Len(t, hook.scopes[0].responses, 1)
			assert.Equal(t, tt.wantStatus, hook.scopes[0].responses[0].StatusCode)
			if tt.wantError {
				require.Len(t, hook.scopes[0].errors, 1)
				assert.Contains(t, hook.scopes[0].errors[0].Error(), "handler failed")
			} else {
				assert.Empty(t, hook.scopes[0].errors)
			}
		})
	}
}

func TestSharedObservabilityMiddleware_FiberNoHookIsNoop(t *testing.T) {
	t.Parallel()

	srv, err := New(noop.NewProvider(), WithTracing(), WithOTelMetrics())
	require.NoError(t, err)
	srv.App().Get("/users/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/users/123", nil)
	resp, err := srv.App().Test(req)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func newObservedFiberServer(
	t *testing.T,
	hook *recordingHTTPInstrumentation,
	handler fiber.Handler,
) *Server {
	t.Helper()

	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	cfg.EnableTracing = true
	cfg.EnableOTelMetrics = true

	srv, err := New(&observabilityWithHTTP{Provider: noop.NewProvider(), hook: hook}, WithConfig(cfg))
	require.NoError(t, err)
	srv.App().Get("/users/:id", handler)
	return srv
}

type observabilityWithHTTP struct {
	*noop.Provider
	hook otelobs.HTTPInstrumentation
}

func (o *observabilityWithHTTP) HTTP() otelobs.HTTPInstrumentation {
	return o.hook
}

type recordingHTTPInstrumentation struct {
	requests []otelobs.HTTPRequest
	scopes   []*recordingHTTPRequestScope
}

func (r *recordingHTTPInstrumentation) StartRequest(
	ctx context.Context,
	req otelobs.HTTPRequest,
) (context.Context, otelobs.HTTPRequestScope) {
	scope := &recordingHTTPRequestScope{}
	r.requests = append(r.requests, req)
	r.scopes = append(r.scopes, scope)
	return ctx, scope
}

type recordingHTTPRequestScope struct {
	errors    []error
	responses []otelobs.HTTPResponse
}

func (r *recordingHTTPRequestScope) OnError(err error) {
	if err != nil {
		r.errors = append(r.errors, err)
	}
}

func (r *recordingHTTPRequestScope) Finish(resp otelobs.HTTPResponse) {
	r.responses = append(r.responses, resp)
}

func TestRecordingHTTPRequestScope_RecordsOnlyNonNilErrors(t *testing.T) {
	t.Parallel()

	expected := errors.New("expected")
	scope := &recordingHTTPRequestScope{}
	scope.OnError(nil)
	scope.OnError(expected)

	require.Len(t, scope.errors, 1)
	assert.ErrorIs(t, scope.errors[0], expected)
}
