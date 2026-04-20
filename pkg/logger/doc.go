// Package logger provides a basic structured logging interface with a Zap implementation.
//
// Deprecated: Use pkg/observability instead. The Observability interface exposes
// a Logger with context propagation, OpenTelemetry correlation, and structured
// fields via the zero-allocation Field type. This package is maintained for
// backwards compatibility only and will be removed in a future major version.
package logger
