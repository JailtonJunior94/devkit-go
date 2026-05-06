package manager

import (
	"log/slog"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"
)

func TestDefaultOptions_HasExpectedDefaults(t *testing.T) {
	o := defaultOptions()
	require.Equal(t, defaultShutdownTimeout, o.shutdownTimeout)
	require.False(t, o.sqlLogging)
	require.False(t, o.readOnly)
	require.NotNil(t, o.observability)
}

func TestWithShutdownTimeout_SetsValue(t *testing.T) {
	o := defaultOptions()
	WithShutdownTimeout(30 * time.Second)(&o)
	require.Equal(t, 30*time.Second, o.shutdownTimeout)
}

func TestWithShutdownTimeout_ZeroOrNegative_Ignored(t *testing.T) {
	o := defaultOptions()
	original := o.shutdownTimeout
	WithShutdownTimeout(0)(&o)
	require.Equal(t, original, o.shutdownTimeout)
}

func TestWithSQLLogging_EnablesLogging(t *testing.T) {
	o := defaultOptions()
	WithSQLLogging(true)(&o)
	require.True(t, o.sqlLogging)
}

func TestWithSQLLogging_DisablesLogging(t *testing.T) {
	o := defaultOptions()
	o.sqlLogging = true
	WithSQLLogging(false)(&o)
	require.False(t, o.sqlLogging)
}

func TestWithObservability_SetsProvider(t *testing.T) {
	o := defaultOptions()
	custom := noop.NewProvider()
	WithObservability(custom)(&o)
	require.Equal(t, custom, o.observability)
}

func TestWithObservability_Nil_KeepsDefault(t *testing.T) {
	o := defaultOptions()
	original := o.observability
	WithObservability(nil)(&o)
	require.Equal(t, original, o.observability)
}

func TestWithReadOnly_SetsFlag(t *testing.T) {
	o := defaultOptions()
	WithReadOnly(true)(&o)
	require.True(t, o.readOnly)
}

func TestWithPoolStatsInterval_SetsValue(t *testing.T) {
	o := defaultOptions()
	WithPoolStatsInterval(30 * time.Second)(&o)
	require.Equal(t, 30*time.Second, o.poolStatsInterval)
}

func TestWithPoolStatsInterval_ZeroOrNegative_Ignored(t *testing.T) {
	o := defaultOptions()
	WithPoolStatsInterval(0)(&o)
	require.Equal(t, time.Duration(0), o.poolStatsInterval)
}

// --- resolveLogger ---

func TestResolveLogger_SQLLoggingDisabled_ReturnsNil(t *testing.T) {
	o := defaultOptions()
	require.Nil(t, resolveLogger(o))
}

func TestResolveLogger_SQLLoggingEnabled_ReturnsSlogDefault(t *testing.T) {
	o := defaultOptions()
	o.sqlLogging = true
	logger := resolveLogger(o)
	require.NotNil(t, logger)
	require.Equal(t, slog.Default(), logger)
}

func TestResolveLogger_SQLLoggingEnabled_NoopObs_FallbackSlogDefault(t *testing.T) {
	// Este é o cenário crítico Q56: WithSQLLogging(true) + noop obs → slog.Default()
	o := defaultOptions() // noop por padrão
	WithSQLLogging(true)(&o)
	logger := resolveLogger(o)
	require.NotNil(t, logger, "esperava o fallback slog.Default() quando o obs é noop")
	require.Equal(t, slog.Default(), logger)
}
