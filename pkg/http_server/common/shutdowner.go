package common

import "context"

// Shutdowner is an interface for components that need graceful shutdown.
// This is typically implemented by observability providers that need to
// flush buffers, close connections, or perform cleanup before the application exits.
//
// Example implementations:
//   - OpenTelemetry TracerProvider
//   - Prometheus metrics registry
//   - Logging providers with buffered output
type Shutdowner interface {
	Shutdown(context.Context) error
}
