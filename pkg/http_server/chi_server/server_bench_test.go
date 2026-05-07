package chiserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

// newBenchmarkChiServer creates a chi server configured for benchmarks.
// noop observability avoids lock contention from the fake provider's capture
// buffers. Health checks and metrics endpoints are disabled to reduce setup noise.
func newBenchmarkChiServer(b *testing.B, opts ...Option) *Server {
	b.Helper()

	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	cfg.EnableMetrics = false
	cfg.EnableCORS = true
	cfg.CORSOrigins = "*"

	all := make([]Option, 0, 1+len(opts))
	all = append(all, WithConfig(cfg))
	all = append(all, opts...)

	srv, err := New(noop.NewProvider(), all...)
	if err != nil {
		b.Fatal(err)
	}

	return srv
}

// BenchmarkChiServer_HotPath_FullMiddlewareChain measures throughput for a
// successful request traversing the full middleware chain:
// recover → requestID → security → cors → bodyLimit → global timeout.
// Each iteration reuses the same *http.Request (no body, safe to share).
func BenchmarkChiServer_HotPath_FullMiddlewareChain(b *testing.B) {
	srv := newBenchmarkChiServer(b)
	srv.RegisterHandler(http.MethodGet, "/ping", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://example.com")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		srv.router.ServeHTTP(rec, req)
	}
}

// BenchmarkChiServer_TimeoutPath measures the per-route timeout path when the
// handler blocks until the 1 ms deadline fires. WithTimeoutCleanup(0) disables
// the leak-counter path so only the core timeout machinery is measured.
// A new *http.Request is allocated per iteration so the context derivation
// inside wrapWithTimeout starts from a clean, uncancelled parent.
func BenchmarkChiServer_TimeoutPath(b *testing.B) {
	srv := newBenchmarkChiServer(b,
		WithRouteTimeout("/slow", time.Millisecond),
		WithTimeoutCleanup(0),
	)
	srv.RegisterHandler(http.MethodGet, "/slow", func(w http.ResponseWriter, r *http.Request) error {
		<-r.Context().Done()
		return nil
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/slow", nil)
		srv.router.ServeHTTP(rec, req)
	}
}

// BenchmarkChiServer_ErrorPath measures the error-handler path: the handler
// returns a typed error that adaptHandler captures and routes through the
// default ErrorHandler → ProblemFromError → JSON encoding.
// The same *http.Request is reused across iterations (no body, no mutation).
func BenchmarkChiServer_ErrorPath(b *testing.B) {
	srv := newBenchmarkChiServer(b)
	benchErr := errors.New("benchmark typed error")
	srv.RegisterHandler(http.MethodGet, "/err", func(_ http.ResponseWriter, _ *http.Request) error {
		return benchErr
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		srv.router.ServeHTTP(rec, req)
	}
}
