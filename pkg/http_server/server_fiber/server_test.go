package serverfiber

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServerWithFake(t *testing.T, opts ...Option) (*Server, *fake.Provider) {
	t.Helper()
	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	provider := fake.NewProvider()
	full := append([]Option{WithConfig(cfg)}, opts...)
	srv, err := New(provider, full...)
	require.NoError(t, err)
	return srv, provider
}

func newTestServerNoop(t *testing.T, opts ...Option) *Server {
	t.Helper()
	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	full := append([]Option{WithConfig(cfg)}, opts...)
	srv, err := New(noop.NewProvider(), full...)
	require.NoError(t, err)
	return srv
}

func readBody(t *testing.T, body io.ReadCloser) []byte {
	t.Helper()
	defer func() { _ = body.Close() }()
	b, err := io.ReadAll(body)
	require.NoError(t, err)
	return b
}

func TestFiberServer_DefaultErrorHandler_UsesProblemFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    fiber.Handler
		wantStatus int
		wantTitle  string
		wantDetail string
	}{
		{
			name: "preserves fiber.Error code and message",
			handler: func(_ *fiber.Ctx) error {
				return fiber.NewError(fiber.StatusNotFound, "user not found")
			},
			wantStatus: fiber.StatusNotFound,
			wantTitle:  "Not Found",
			wantDetail: "user not found",
		},
		{
			name: "maps non-fiber error to internal server error fallback",
			handler: func(_ *fiber.Ctx) error {
				return errors.New("db connection refused")
			},
			wantStatus: fiber.StatusInternalServerError,
			wantTitle:  "Internal Server Error",
			wantDetail: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv, _ := newTestServerWithFake(t)
			srv.App().Get("/boom", tt.handler)

			req := httptest.NewRequest(fiber.MethodGet, "/boom", nil)
			resp, err := srv.App().Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, problemContentType, resp.Header.Get(fiber.HeaderContentType))

			var problem common.ProblemDetail
			require.NoError(t, json.Unmarshal(readBody(t, resp.Body), &problem))
			assert.Equal(t, tt.wantStatus, problem.Status)
			assert.Equal(t, tt.wantTitle, problem.Title)
			assert.Equal(t, tt.wantDetail, problem.Detail)
			assert.Equal(t, "/boom", problem.Instance)
		})
	}
}

func TestFiberServer_DefaultErrorHandler_DoesNotLeakRawError(t *testing.T) {
	t.Parallel()

	const sensitive = "secret-db-credentials=hunter2"

	srv, _ := newTestServerWithFake(t)
	srv.App().Get("/leak", func(_ *fiber.Ctx) error {
		return errors.New(sensitive)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/leak", nil)
	resp, err := srv.App().Test(req)
	require.NoError(t, err)

	body := readBody(t, resp.Body)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.NotContains(t, string(body), sensitive)

	var problem common.ProblemDetail
	require.NoError(t, json.Unmarshal(body, &problem))
	assert.Equal(t, "internal server error", problem.Detail)
}

func TestFiberServer_RequestIDMiddleware_RegeneratesOnInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		incoming   string
		wantEcho   bool
		wantNewGen bool
	}{
		{
			name:       "rejects whitespace and generates a fresh id",
			incoming:   "  \t",
			wantNewGen: true,
		},
		{
			name:       "rejects forbidden characters and generates a fresh id",
			incoming:   "evil/../../id",
			wantNewGen: true,
		},
		{
			name:       "rejects values exceeding the max length",
			incoming:   strings.Repeat("a", common.MaxRequestIDLength+1),
			wantNewGen: true,
		},
		{
			name:     "echoes a valid id",
			incoming: "req-123_abc.def",
			wantEcho: true,
		},
		{
			name:       "generates a fresh id when missing",
			incoming:   "",
			wantNewGen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv, _ := newTestServerWithFake(t)
			srv.App().Get("/echo", func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest(fiber.MethodGet, "/echo", nil)
			if tt.incoming != "" {
				req.Header.Set(common.HeaderRequestID, tt.incoming)
			}

			resp, err := srv.App().Test(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			got := resp.Header.Get(common.HeaderRequestID)
			require.NotEmpty(t, got, "X-Request-ID must always be set in response")

			if tt.wantEcho {
				assert.Equal(t, tt.incoming, got)
				return
			}
			assert.NotEqual(t, tt.incoming, got)
			// freshly generated values follow uuid format (length 36)
			assert.Len(t, got, 36)
		})
	}
}

func TestFiberServer_RequestIDMiddleware_LogsWarnOnInvalid(t *testing.T) {
	t.Parallel()

	const rawInvalid = "this/is/not/valid"

	srv, provider := newTestServerWithFake(t)
	srv.App().Get("/warn", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/warn", nil)
	req.Header.Set(common.HeaderRequestID, rawInvalid)

	resp, err := srv.App().Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	logger, ok := provider.Logger().(*fake.FakeLogger)
	require.True(t, ok, "fake provider must expose FakeLogger")

	var warnEntry *fake.LogEntry
	for i, entry := range logger.GetEntries() {
		if entry.Message == "invalid X-Request-ID rejected" {
			warnEntry = &logger.GetEntries()[i]
			break
		}
	}
	require.NotNil(t, warnEntry, "warn log for invalid X-Request-ID must be emitted")

	wantFields := map[string]bool{
		"raw_length":  false,
		"remote_addr": false,
		"path":        false,
		"method":      false,
	}
	for _, f := range warnEntry.Fields {
		if _, tracked := wantFields[f.Key]; tracked {
			wantFields[f.Key] = true
		}
		assert.NotEqual(t, rawInvalid, f.AnyValue(), "raw value must NOT be present in any field")
	}
	for key, present := range wantFields {
		assert.True(t, present, "warn log missing field %q", key)
	}
}

func TestFiberServer_RouteIsUnmatchedFor404(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		registerGet bool
		path        string
		want        string
	}{
		{
			name:        "unmatched when no handler is registered",
			registerGet: false,
			path:        "/missing",
			want:        "unmatched",
		},
		{
			name:        "unmatched when path does not match registered handler",
			registerGet: true,
			path:        "/missing",
			want:        "unmatched",
		},
		{
			name:        "returns the registered pattern when matched",
			registerGet: true,
			path:        "/users/123",
			want:        "/users/:id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured string
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error {
				err := c.Next()
				captured = fiberRoutePattern(c)
				return err
			})
			if tt.registerGet {
				app.Get("/users/:id", func(c *fiber.Ctx) error {
					return c.SendStatus(fiber.StatusOK)
				})
			}

			req := httptest.NewRequest(fiber.MethodGet, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			assert.Equal(t, tt.want, captured)
		})
	}
}

func TestFiberServer_HealthEndpoint_UsesCommonExecuteHealthChecks(t *testing.T) {
	t.Parallel()

	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = true

	checks := map[string]HealthCheckFunc{
		"db": func(_ context.Context) error {
			return errors.New("connection refused")
		},
		"cache": func(_ context.Context) error {
			return nil
		},
	}

	srv, err := New(fake.NewProvider(), WithConfig(cfg), WithHealthChecks(checks))
	require.NoError(t, err)

	req := httptest.NewRequest(fiber.MethodGet, "/health", nil)
	resp, err := srv.App().Test(req)
	require.NoError(t, err)

	body := readBody(t, resp.Body)
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

	var status common.HealthStatus
	require.NoError(t, json.Unmarshal(body, &status))
	assert.Equal(t, "unhealthy", status.Status)

	require.Contains(t, status.Checks, "db")
	require.Contains(t, status.Checks, "cache")
	assert.Equal(t, "unhealthy", status.Checks["db"].Status)
	assert.Equal(t, "connection refused", status.Checks["db"].Error)
	assert.Equal(t, "healthy", status.Checks["cache"].Status)
}

func TestFiberServer_WithEnvironment_PropagatesToConfig(t *testing.T) {
	t.Parallel()

	srv := newTestServerNoop(t, WithEnvironment("staging"))
	assert.Equal(t, "staging", srv.config.Environment)
}

func TestFiberServer_WithShutdownTimeout_PropagatesToConfig(t *testing.T) {
	t.Parallel()

	srv := newTestServerNoop(t, WithShutdownTimeout(7*time.Second))
	assert.Equal(t, 7*time.Second, srv.config.ShutdownTimeout)
}

func TestFiberServer_Shutdown_DerivesFromParentContext(t *testing.T) {
	t.Parallel()

	srv := newTestServerNoop(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	require.NoError(t, srv.Shutdown(ctx))
	elapsed := time.Since(start)
	assert.LessOrEqual(t, elapsed, 2*time.Second,
		"Shutdown must respect the deadline carried by the parent context")
}

// TestFiberServer_Start_NilContext_DoesNotPanic covers the nil-ctx defense
// added for parity with chi_server. context.WithTimeout panics with
// "cannot create context from nil parent" if reached with ctx == nil; the
// fix at the top of Start normalizes ctx to context.Background() before any
// nil-sensitive call. The test drives the sigChan branch via SIGTERM (which
// signal.Notify in Start intercepts so the test process is not killed) so
// the WithTimeout(ctx, ...) call is actually reached.
//
// Not parallel: relies on process-level signal delivery.
func TestFiberServer_Start_NilContext_DoesNotPanicOnSignalShutdown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM signal injection is unreliable on Windows test runners")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	srv := newTestServerNoop(t, WithShutdownTimeout(100*time.Millisecond))
	srv.config.Address = addr

	done := make(chan struct{})
	var panicVal any
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				panicVal = r
			}
		}()
		_ = srv.Start(nil)
	}()

	// Allow Start to bind, register signal.Notify and enter the select.
	time.Sleep(150 * time.Millisecond)

	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, proc.Signal(syscall.SIGTERM))

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after SIGTERM (nil-ctx defense may be missing)")
	}

	require.Nil(t, panicVal,
		"Start(nil) must not panic when shutdown branch derives the shutdownCtx from a nil parent")
}
