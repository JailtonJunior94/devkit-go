package serverfiber

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"runtime"
	"testing"
	"time"

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
			wantRoute:         "/users/:id",
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
			wantRoute:  "/users/:id",
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

func TestFiberServer_TimeoutMiddleware_AdoptsOfficialMiddleware_NoRace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		duration          time.Duration
		handlerSleep      time.Duration
		respectsCtx       bool
		expectStatus      int
		expectBeforeSleep bool
	}{
		{
			name:              "respects ctx returns 408 fast",
			duration:          20 * time.Millisecond,
			handlerSleep:      500 * time.Millisecond,
			respectsCtx:       true,
			expectStatus:      fiber.StatusRequestTimeout,
			expectBeforeSleep: true,
		},
		{
			name:         "fast handler returns 200",
			duration:     200 * time.Millisecond,
			handlerSleep: 1 * time.Millisecond,
			respectsCtx:  true,
			expectStatus: fiber.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := common.DefaultConfig()
			cfg.EnableHealthChecks = false
			srv, err := New(noop.NewProvider(), WithConfig(cfg), WithRouteTimeout("/work", tc.duration))
			require.NoError(t, err)

			srv.RegisterHandler(fiber.MethodGet, "/work", buildSleepHandler(tc.handlerSleep, tc.respectsCtx))

			req := httptest.NewRequest(fiber.MethodGet, "/work", nil)
			start := time.Now()
			resp, err := srv.App().Test(req, 5000)
			elapsed := time.Since(start)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, resp.Body.Close())
			}()

			assert.Equal(t, tc.expectStatus, resp.StatusCode)
			if tc.expectBeforeSleep {
				assert.Less(t, elapsed, tc.handlerSleep, "handler should not run to completion when ctx fires first")
			}
		})
	}
}

func TestFiberServer_RouteTimeout_MatchesParameterizedRoutes(t *testing.T) {
	t.Parallel()

	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	srv, err := New(noop.NewProvider(), WithConfig(cfg), WithRouteTimeout("/users/:id", 1*time.Millisecond))
	require.NoError(t, err)

	srv.RegisterHandler(fiber.MethodGet, "/users/:id", buildSleepHandler(500*time.Millisecond, true))

	req := httptest.NewRequest(fiber.MethodGet, "/users/42", nil)
	resp, err := srv.App().Test(req, 5000)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	assert.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
}

func TestFiberServer_RouteTimeout_FallsBackToGlobalTimeout(t *testing.T) {
	t.Parallel()

	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	cfg.ReadTimeout = 5 * time.Millisecond
	srv, err := New(noop.NewProvider(), WithConfig(cfg))
	require.NoError(t, err)

	srv.RegisterHandler(fiber.MethodGet, "/slow", buildSleepHandler(500*time.Millisecond, true))

	req := httptest.NewRequest(fiber.MethodGet, "/slow", nil)
	resp, err := srv.App().Test(req, 5000)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	assert.Equal(t, fiber.StatusRequestTimeout, resp.StatusCode)
}

func TestFiberServer_TimeoutMiddleware_HangingHandler_DocumentedTradeoff(t *testing.T) {
	t.Parallel()

	const (
		routeTimeout = 5 * time.Millisecond
		handlerSleep = 80 * time.Millisecond
	)

	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	srv, err := New(noop.NewProvider(), WithConfig(cfg), WithRouteTimeout("/hang", routeTimeout))
	require.NoError(t, err)

	srv.RegisterHandler(fiber.MethodGet, "/hang", buildSleepHandler(handlerSleep, false))

	req := httptest.NewRequest(fiber.MethodGet, "/hang", nil)
	start := time.Now()
	resp, err := srv.App().Test(req, 5000)
	elapsed := time.Since(start)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode,
		"NewWithContext does not interrupt a handler that ignores ctx.Done(); a 408 must NOT be returned")
	assert.GreaterOrEqual(t, elapsed, handlerSleep,
		"hung handler should block the response until it finishes — documents the trade-off")
}

func TestMakeTimeoutHandler_ZeroDurationReturnsSameHandler(t *testing.T) {
	t.Parallel()

	called := false
	next := fiber.Handler(func(c *fiber.Ctx) error {
		called = true
		return c.SendStatus(fiber.StatusOK)
	})

	wrapped := makeTimeoutHandler(0, next)

	require.Equal(t, handlerName(next), handlerName(wrapped),
		"makeTimeoutHandler must return next unmodified when d <= 0")

	app := fiber.New()
	app.Get("/", wrapped)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()
	assert.True(t, called)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestMakeTimeoutHandler_PositiveDurationDelegatesToOfficialMiddleware(t *testing.T) {
	t.Parallel()

	next := fiber.Handler(func(_ *fiber.Ctx) error { return nil })
	wrapped := makeTimeoutHandler(10*time.Millisecond, next)

	wrappedName := handlerName(wrapped)
	require.Contains(t, wrappedName, "NewWithContext",
		"makeTimeoutHandler with d>0 must delegate to fibermwtimeout.NewWithContext")
	require.NotEqual(t, handlerName(next), wrappedName,
		"wrapped handler must differ from the original next when d>0")
}

func TestFiberServer_CorsMiddleware_PanicsOnInvalidConfig(t *testing.T) {
	t.Parallel()

	// corsMiddleware must fail-fast on a malformed origins string. Validate()
	// guards New(), but the middleware itself is the last line of defense
	// when something bypasses validation. Mirrors chi_server's fail-fast in
	// corsMiddleware. Regression for the prior silent error swallowing.
	require.PanicsWithValue(t,
		"invalid CORS configuration: wildcard (*) cannot be combined with other origins",
		func() {
			_ = corsMiddleware("*,https://example.com")
		},
	)
}

func TestFiberServer_RegisterHandler_DoesNotDoubleWrapTimeoutWithoutPerRoute(t *testing.T) {
	t.Parallel()

	// When no per-route timeout is set, RegisterHandler must register the
	// handler as-is. The global Use(makeTimeoutHandler(ReadTimeout, ...))
	// already covers the route; double-wrapping with the same ReadTimeout is
	// pure churn. Regression for the prior behavior that always wrapped.
	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	cfg.ReadTimeout = 1 * time.Second // global timeout enabled
	srv, err := New(noop.NewProvider(), WithConfig(cfg))
	require.NoError(t, err)

	sentinel := fiber.Handler(func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	srv.RegisterHandler(fiber.MethodGet, "/no-rt", sentinel)

	var registered fiber.Handler
	for _, route := range srv.App().GetRoutes(true) {
		if route.Method == fiber.MethodGet && route.Path == "/no-rt" {
			require.NotEmpty(t, route.Handlers, "registered route must have at least one handler")
			registered = route.Handlers[len(route.Handlers)-1]
			break
		}
	}
	require.NotNil(t, registered, "expected registered handler for /no-rt")

	require.Equal(t, handlerName(sentinel), handlerName(registered),
		"RegisterHandler without WithRouteTimeout must NOT wrap the handler with NewWithContext")
}

func TestFiberServer_RegisterHandler_WrapsWhenPerRouteTimeoutSet(t *testing.T) {
	t.Parallel()

	// Counterpart to the previous regression: when WithRouteTimeout is set,
	// RegisterHandler MUST wrap the handler with the per-route deadline so
	// that route gets the shorter timeout regardless of the global one.
	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	srv, err := New(noop.NewProvider(), WithConfig(cfg),
		WithRouteTimeout("/with-rt", 50*time.Millisecond),
	)
	require.NoError(t, err)

	sentinel := fiber.Handler(func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	srv.RegisterHandler(fiber.MethodGet, "/with-rt", sentinel)

	var registered fiber.Handler
	for _, route := range srv.App().GetRoutes(true) {
		if route.Method == fiber.MethodGet && route.Path == "/with-rt" {
			registered = route.Handlers[len(route.Handlers)-1]
			break
		}
	}
	require.NotNil(t, registered)

	require.Contains(t, handlerName(registered), "NewWithContext",
		"RegisterHandler with WithRouteTimeout must wrap via fibermwtimeout.NewWithContext")
	require.NotEqual(t, handlerName(sentinel), handlerName(registered),
		"wrapped handler must differ from the original sentinel")
}

func buildSleepHandler(d time.Duration, respectsCtx bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !respectsCtx {
			time.Sleep(d)
			return c.SendStatus(fiber.StatusOK)
		}
		select {
		case <-time.After(d):
			return c.SendStatus(fiber.StatusOK)
		case <-c.UserContext().Done():
			return c.UserContext().Err()
		}
	}
}

func handlerName(h fiber.Handler) string {
	pc := reflect.ValueOf(h).Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}
	return fn.Name()
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

// --- Recover middleware ---

func TestFiberServer_RecoverMiddleware_NoPanic(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(recoverMiddleware(noop.NewProvider()))
	app.Get("/ok", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/ok", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestFiberServer_RecoverMiddleware_PanicReturns500(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(recoverMiddleware(noop.NewProvider()))
	app.Get("/boom", func(_ *fiber.Ctx) error {
		panic("test panic")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/boom", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, problemContentType, resp.Header.Get(fiber.HeaderContentType))

	var problem common.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&problem))
	assert.Equal(t, fiber.StatusInternalServerError, problem.Status)
	assert.Equal(t, "Internal Server Error", problem.Title)
	assert.Equal(t, "Internal server error", problem.Detail)
	assert.Equal(t, "/boom", problem.Instance)
}

// --- CORS middleware ---

func TestFiberServer_CorsMiddleware_SetsBasicHeaders(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(corsMiddleware("*"))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Headers"))
}

func TestFiberServer_CorsMiddleware_HandlesPreflight(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(corsMiddleware("https://example.com"))
	app.Options("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestFiberServer_CorsMiddleware_SpecificOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		requestOrigin string
		wantStatus    int
		wantOrigin    string
	}{
		{
			name:          "allowed specific origin is echoed back",
			requestOrigin: "https://example.com",
			wantStatus:    fiber.StatusOK,
			wantOrigin:    "https://example.com",
		},
		{
			name:          "disallowed origin is rejected with 403",
			requestOrigin: "https://evil.com",
			wantStatus:    fiber.StatusForbidden,
			wantOrigin:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{
				ErrorHandler: func(c *fiber.Ctx, err error) error {
					var fiberErr *fiber.Error
					if errors.As(err, &fiberErr) {
						return c.Status(fiberErr.Code).SendString(fiberErr.Message)
					}
					return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
				},
			})
			app.Use(corsMiddleware("https://example.com"))
			app.Get("/", func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest(fiber.MethodGet, "/", nil)
			req.Header.Set("Origin", tt.requestOrigin)
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			if tt.wantOrigin != "" {
				assert.Equal(t, tt.wantOrigin, resp.Header.Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestFiberServer_CorsMiddleware_NoOriginHeader_Passthrough(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(corsMiddleware("https://example.com"))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	// No Origin header — CORS middleware must pass through without error.
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}
