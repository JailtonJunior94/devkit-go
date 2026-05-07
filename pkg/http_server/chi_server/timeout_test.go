package chiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// timeoutFakeProvider is a fake.Provider exposing the metrics surface used
// by chi_server. It wires in a request-scoped requestID into the request
// context for the recordTimeoutLeak path so test assertions can match the
// log fields emitted by the warn entry.
func newLeakProbe(t *testing.T) (*fake.FakeCounter, *fake.FakeLogger, *fake.Provider) {
	t.Helper()
	provider := fake.NewProvider()
	logger, ok := provider.Logger().(*fake.FakeLogger)
	require.True(t, ok)
	metrics, ok := provider.Metrics().(*fake.FakeMetrics)
	require.True(t, ok)
	counter := metrics.Counter(
		"http_handler_timeout_leak_total",
		"",
		"1",
	).(*fake.FakeCounter)
	return counter, logger, provider
}

func findWarnEntry(entries []fake.LogEntry, msg string) *fake.LogEntry {
	for i := range entries {
		if entries[i].Level == observability.LogLevelWarn && entries[i].Message == msg {
			return &entries[i]
		}
	}
	return nil
}

func TestChiServer_TimeoutMiddleware_DoesNotRaceOnConcurrentWrite(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider, WithRouteTimeout("/race", 5*time.Millisecond))

	srv.RegisterHandler(http.MethodGet, "/race", func(w http.ResponseWriter, r *http.Request) error {
		// Handler ignores ctx.Done() and tries to write after timeout
		// concurrently with the timeout middleware writing 408.
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("late"))
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/race", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	assert.Equal(t, "application/problem+json", rr.Header().Get("Content-Type"))

	// Allow the leaked goroutine to complete so the race detector can flush
	// any concurrent access. The timeoutWriter mu must serialize this write.
	time.Sleep(120 * time.Millisecond)
}

func TestChiServer_TimeoutMiddleware_WritesProblemJSON(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider, WithRouteTimeout("/slow", 5*time.Millisecond))

	released := make(chan struct{})
	srv.RegisterHandler(http.MethodGet, "/slow", func(w http.ResponseWriter, r *http.Request) error {
		<-released
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)

	var body common.ProblemDetail
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, http.StatusRequestTimeout, body.Status)
	assert.Equal(t, "Request Timeout", body.Title)
	assert.Equal(t, "Request timeout exceeded", body.Detail)
	assert.Equal(t, "/slow", body.Instance)
	close(released)
}

func TestChiServer_TimeoutMiddleware_IncrementsLeakCounter(t *testing.T) {
	t.Parallel()

	counter, logger, provider := newLeakProbe(t)
	srv := newTestServer(t, provider,
		WithRouteTimeout("/leak", 5*time.Millisecond),
		WithTimeoutCleanup(20*time.Millisecond),
	)

	released := make(chan struct{})
	srv.RegisterHandler(http.MethodGet, "/leak", func(w http.ResponseWriter, r *http.Request) error {
		// Handler deliberately ignores ctx.Done()
		<-released
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/leak", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)

	// Cleanup window expired; expect exactly one leak recorded.
	values := counter.GetValues()
	require.Len(t, values, 1, "leak counter must record exactly one increment")
	assert.Equal(t, int64(1), values[0].Value)

	fields := indexFields(values[0].Fields)
	assert.Equal(t, "chi", fields["adapter"])
	assert.Equal(t, "/leak", fields["route"])

	warn := findWarnEntry(logger.GetEntries(), "http handler timeout leak")
	require.NotNil(t, warn, "leak warn log must be emitted")
	warnFields := indexFields(warn.Fields)
	assert.Equal(t, "/leak", warnFields["route"])
	assert.Equal(t, "/leak", warnFields["path"])

	close(released)
}

func TestChiServer_TimeoutMiddleware_ZeroCleanupDoesNotIncrementCounter(t *testing.T) {
	t.Parallel()

	counter, logger, provider := newLeakProbe(t)
	srv := newTestServer(t, provider,
		WithRouteTimeout("/no-leak", 5*time.Millisecond),
		WithTimeoutCleanup(0),
	)

	released := make(chan struct{})
	srv.RegisterHandler(http.MethodGet, "/no-leak", func(w http.ResponseWriter, r *http.Request) error {
		<-released
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/no-leak", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)

	// Allow time for any spurious cleanup goroutine to misfire.
	time.Sleep(30 * time.Millisecond)

	assert.Empty(t, counter.GetValues(), "cleanup=0 must NOT increment leak counter (RF-4.6)")
	assert.Nil(t, findWarnEntry(logger.GetEntries(), "http handler timeout leak"),
		"cleanup=0 must NOT emit leak warn log")

	close(released)
}

func TestChiServer_TimeoutMiddleware_PanicReachesRecoverMiddleware(t *testing.T) {
	t.Parallel()

	_, logger, provider := newLeakProbe(t)
	srv := newTestServer(t, provider, WithRouteTimeout("/panic", 50*time.Millisecond))

	srv.RegisterHandler(http.MethodGet, "/panic", func(http.ResponseWriter, *http.Request) error {
		panic("boom")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var found bool
	for _, e := range logger.GetEntries() {
		if e.Level == observability.LogLevelError && e.Message == "panic recovered" {
			found = true
			break
		}
	}
	assert.True(t, found, "outer recover middleware must log the panic")
}

func TestChiServer_TimeoutMiddleware_WriteAfterTimeoutIsNoop(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider,
		WithRouteTimeout("/late", 5*time.Millisecond),
		WithTimeoutCleanup(50*time.Millisecond),
	)

	wrote := make(chan struct{})
	srv.RegisterHandler(http.MethodGet, "/late", func(w http.ResponseWriter, r *http.Request) error {
		// Wait long enough for the timeout to fire, then attempt to write.
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ignored"))
		close(wrote)
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/late", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	assert.NotContains(t, rr.Body.String(), "ignored",
		"writes after timeout must be no-op")

	select {
	case <-wrote:
	case <-time.After(200 * time.Millisecond):
	}
}

func TestChiServer_RegisterHandler_PerRouteTimeoutFiresBeforeGlobal(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider,
		WithReadTimeout(500*time.Millisecond),
		WithRouteTimeout("/fast", 5*time.Millisecond),
	)

	released := make(chan struct{})
	srv.RegisterHandler(http.MethodGet, "/fast", func(w http.ResponseWriter, r *http.Request) error {
		// Ignore ctx.Done() so the per-route timeout is the only deadline
		// that can fire and we can deterministically assert the 408 wire
		// response without racing against handler completion.
		<-released
		w.WriteHeader(http.StatusOK)
		return nil
	})

	start := time.Now()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	srv.router.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	assert.Less(t, elapsed, 250*time.Millisecond,
		"per-route timeout must fire well before the global ReadTimeout")
	close(released)
}

func TestChiServer_GlobalTimeoutMiddleware_DoesNotReadRouteTimeouts(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	// Configure a per-route timeout for a path the user-level RegisterHandler
	// will NOT register; this exercises the top-level fallback path
	// (chi.Router.Get bypassing RegisterHandler).
	srv := newTestServer(t, provider,
		WithReadTimeout(20*time.Millisecond),
		WithRouteTimeout("/legacy", 5*time.Millisecond),
	)

	var hits int32
	released := make(chan struct{})
	srv.router.Get("/legacy", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		// Handler intentionally ignores ctx.Done() so the global timeout
		// is the only deadline that can fire; we keep the handler hung
		// until the test releases it after asserting the wire response.
		<-released
		w.WriteHeader(http.StatusOK)
	})

	start := time.Now()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/legacy", nil)
	srv.router.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	// If the global middleware were honoring routeTimeouts, it would fire at
	// 5ms instead of 20ms; require at least the global window to confirm.
	assert.GreaterOrEqual(t, elapsed, 18*time.Millisecond,
		"top-level timeoutMiddleware must apply globalTimeout, not routeTimeouts")
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
	close(released)
}

func TestChiServer_GlobalTimeoutMiddleware_408PreservesSecurityHeaders(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	// Drive the global timeout path (no RegisterHandler) and verify that the
	// 408 response carries security headers set by securityHeadersMiddleware
	// inside the timeout goroutine. Regression for the prior behavior where
	// tw.h (holding security/cors headers) was discarded before writing 408.
	srv := newTestServer(t, provider, WithReadTimeout(5*time.Millisecond))

	released := make(chan struct{})
	srv.router.Get("/sec", func(w http.ResponseWriter, _ *http.Request) {
		<-released
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sec", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"),
		"408 must carry X-Frame-Options from securityHeadersMiddleware")
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"),
		"408 must carry X-Content-Type-Options")
	assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy"),
		"408 must carry Content-Security-Policy")
	assert.NotEmpty(t, rr.Header().Get("Strict-Transport-Security"),
		"408 must carry HSTS")
	close(released)
}

func TestChiServer_PerRouteTimeout_408PreservesSecurityHeaders(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	// Counterpart to the global-timeout regression: a route registered via
	// RegisterHandler with a per-route timeout must also surface security
	// headers in 408 responses. The fix relies on registerMiddlewares
	// installing securityHeadersMiddleware UPSTREAM of timeoutMiddleware so
	// the underlying ResponseWriter already carries security headers when
	// wrapWithTimeout writes the 408.
	srv := newTestServer(t, provider, WithRouteTimeout("/r", 5*time.Millisecond))

	released := make(chan struct{})
	srv.RegisterHandler(http.MethodGet, "/r", func(_ http.ResponseWriter, _ *http.Request) error {
		<-released
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/r", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"),
		"per-route 408 must carry X-Frame-Options")
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"),
		"per-route 408 must carry X-Content-Type-Options")
	assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy"),
		"per-route 408 must carry Content-Security-Policy")
	close(released)
}

func TestChiServer_RegisterHandler_NoRouteTimeout_RunsWithoutWrap(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider) // no WithRouteTimeout -> wrap not applied

	called := false
	srv.RegisterHandler(http.MethodGet, "/plain", func(w http.ResponseWriter, _ *http.Request) error {
		called = true
		w.WriteHeader(http.StatusOK)
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	srv.router.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestChiServer_TimeoutMiddleware_HandlerCompletesBeforeTimeoutPasses(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider, WithRouteTimeout("/quick", 200*time.Millisecond))

	srv.RegisterHandler(http.MethodGet, "/quick", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/quick", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "done", strings.TrimSpace(rr.Body.String()))
}

func TestChiServer_TimeoutMiddleware_HandlerSetsHeadersUnderDeadline(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider, WithRouteTimeout("/headers", 200*time.Millisecond))

	srv.RegisterHandler(http.MethodGet, "/headers", func(w http.ResponseWriter, _ *http.Request) error {
		// Setting custom headers must NOT panic on a route protected by
		// wrapWithTimeout — the buffered timeoutWriter map must be backed
		// by a usable http.Header instance even on the per-route path.
		w.Header().Set("X-Custom", "ok")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/headers", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Header().Get("X-Custom"))
	assert.Equal(t, "text/plain", rr.Header().Get("Content-Type"))
	assert.Equal(t, "ok", rr.Body.String())
}

func TestChiServer_TimeoutMiddleware_ErrorReturnedByHandlerSurfacesAs500(t *testing.T) {
	t.Parallel()

	_, _, provider := newLeakProbe(t)
	srv := newTestServer(t, provider, WithRouteTimeout("/err", 100*time.Millisecond))

	srv.RegisterHandler(http.MethodGet, "/err", func(http.ResponseWriter, *http.Request) error {
		return errors.New("downstream")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestChiServer_NewLeakCounter_NilWhenMetricsAbsent(t *testing.T) {
	t.Parallel()

	got := newLeakCounter(nilMetricsProvider{})
	assert.Nil(t, got, "newLeakCounter must return nil when Metrics() is nil")
}

func TestChiServer_RecordTimeoutLeak_NoCounterStillLogs(t *testing.T) {
	t.Parallel()

	_, logger, provider := newLeakProbe(t)
	srv, err := New(provider,
		WithServiceName("t"), WithServiceVersion("0"), WithEnvironment("test"),
	)
	require.NoError(t, err)
	srv.leakCounter = nil // simulate absent metrics surface

	ctx := context.WithValue(context.Background(), requestIDKey, "rid-9")
	srv.recordTimeoutLeak(ctx, "/x", "")

	warn := findWarnEntry(logger.GetEntries(), "http handler timeout leak")
	require.NotNil(t, warn)
	fields := indexFields(warn.Fields)
	assert.Equal(t, "rid-9", fields["request_id"])
	assert.Equal(t, "/x", fields["route"])
	assert.Empty(t, fields["path"])
}

// nilMetricsProvider is a minimal observability.Observability whose Metrics()
// returns nil so tests can drive the nil-counter branch in newLeakCounter.
type nilMetricsProvider struct{}

func (nilMetricsProvider) Tracer() observability.Tracer   { return nil }
func (nilMetricsProvider) Logger() observability.Logger   { return nil }
func (nilMetricsProvider) Metrics() observability.Metrics { return nil }
func (nilMetricsProvider) Shutdown(context.Context) error { return nil }
