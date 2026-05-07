package serverfiber

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/gofiber/fiber/v2"
)

// newBenchmarkFiberServer creates a Fiber server configured for benchmarks.
// noop observability avoids lock contention from the fake provider's capture
// buffers. Health checks and metrics endpoints are disabled to reduce setup noise.
func newBenchmarkFiberServer(b *testing.B, opts ...Option) *Server {
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

// BenchmarkFiberServer_HotPath_FullMiddlewareChain measures throughput for a
// successful request traversing the full middleware chain:
// recover → requestID → global timeout → security → cors (body limit enforced
// natively by Fiber via fiber.Config.BodyLimit).
// A new *http.Request is created per iteration because app.Test consumes it.
func BenchmarkFiberServer_HotPath_FullMiddlewareChain(b *testing.B) {
	srv := newBenchmarkFiberServer(b)
	srv.RegisterHandler(fiber.MethodGet, "/ping", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(fiber.MethodGet, "/ping", nil)
		req.Header.Set("Origin", "https://example.com")
		_, _ = srv.App().Test(req, -1)
	}
}

// BenchmarkFiberServer_TimeoutPath measures the per-route timeout path when the
// handler blocks on c.UserContext().Done(). The 1 ms deadline derives from the
// WithRouteTimeout option installed via RegisterHandler. The handler returns
// context.DeadlineExceeded which the official fiber timeout middleware maps to
// fiber.ErrRequestTimeout (408).
func BenchmarkFiberServer_TimeoutPath(b *testing.B) {
	srv := newBenchmarkFiberServer(b, WithRouteTimeout("/slow", time.Millisecond))
	srv.RegisterHandler(fiber.MethodGet, "/slow", func(c *fiber.Ctx) error {
		<-c.UserContext().Done()
		return c.UserContext().Err()
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(fiber.MethodGet, "/slow", nil)
		_, _ = srv.App().Test(req, -1)
	}
}

// BenchmarkFiberServer_ErrorPath measures the error-handler path: the handler
// returns a *fiber.Error that ProblemFromError maps to an RFC 7807
// application/problem+json response (preserving the fiber error code/message).
func BenchmarkFiberServer_ErrorPath(b *testing.B) {
	srv := newBenchmarkFiberServer(b)
	benchErr := fiber.NewError(fiber.StatusBadRequest, "benchmark typed error")
	srv.App().Get("/err", func(_ *fiber.Ctx) error {
		return benchErr
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(fiber.MethodGet, "/err", nil)
		_, _ = srv.App().Test(req, -1)
	}
}
