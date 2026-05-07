package chiserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChiServer_WithShutdownTimeout_PropagatesToConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "100ms", timeout: 100 * time.Millisecond},
		{name: "5s", timeout: 5 * time.Second},
		{name: "explicit 1s overrides default", timeout: 1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv, err := New(fake.NewProvider(),
				WithServiceName("test"),
				WithServiceVersion("0.0.0"),
				WithEnvironment("test"),
				WithShutdownTimeout(tt.timeout),
			)
			require.NoError(t, err)
			assert.Equal(t, tt.timeout, srv.config.ShutdownTimeout)
		})
	}
}

func TestChiServer_Shutdown_DerivesFromParentContext(t *testing.T) {
	t.Parallel()

	srv, err := New(fake.NewProvider(),
		WithServiceName("test"),
		WithServiceVersion("0.0.0"),
		WithEnvironment("test"),
		WithShutdownTimeout(2*time.Second),
	)
	require.NoError(t, err)

	parent, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err = srv.Shutdown(parent)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 500*time.Millisecond,
		"Shutdown must honor the parent context, not block on the configured timeout")
}

func TestChiServer_Shutdown_RespectsParentDeadline(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	srv, err := New(fake.NewProvider(),
		withRawAddress(addr),
		WithServiceName("test"),
		WithServiceVersion("0.0.0"),
		WithEnvironment("test"),
		WithShutdownTimeout(30*time.Second),
	)
	require.NoError(t, err)

	parent, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- srv.Start(parent) }()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		assert.NoError(t, err)
		assert.Less(t, elapsed, 2*time.Second,
			"Start must return shortly after parent deadline expires, not wait for ShutdownTimeout")
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return within parent deadline budget")
	}
}

func TestChiServer_Shutdown_UsesConfiguredTimeoutWhenParentHasNone(t *testing.T) {
	t.Parallel()

	srv, err := New(fake.NewProvider(),
		WithServiceName("test"),
		WithServiceVersion("0.0.0"),
		WithEnvironment("test"),
		WithShutdownTimeout(50*time.Millisecond),
	)
	require.NoError(t, err)

	hookedServer := &http.Server{
		Addr:    srv.config.Address,
		Handler: srv.router,
	}
	srv.httpServer = hookedServer

	parent, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	require.NoError(t, srv.Shutdown(parent))
	assert.Equal(t, 50*time.Millisecond, srv.config.ShutdownTimeout)
}

func TestChiServer_Shutdown_DefaultShutdownTimeoutIs30s(t *testing.T) {
	t.Parallel()

	cfg := common.DefaultConfig()
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
}

// withRawAddress sets the listening address verbatim, bypassing the
// ":" prefix coercion done by WithPort. Used in tests that need the
// full host:port form returned by net.Listen.
func withRawAddress(addr string) Option {
	return func(s *Server) {
		s.config.Address = addr
	}
}
