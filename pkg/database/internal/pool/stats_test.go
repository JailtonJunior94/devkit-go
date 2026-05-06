package pool_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/stretchr/testify/require"
)

func TestDiff_GaugeFieldsKeepCurrentValue(t *testing.T) {
	prev := pool.Stats{OpenConnections: 3, Idle: 1, WaitCount: 10, WaitDuration: 100 * time.Millisecond}
	cur := pool.Stats{OpenConnections: 5, Idle: 2, WaitCount: 15, WaitDuration: 150 * time.Millisecond}

	d := pool.Diff(cur, prev)

	require.Equal(t, 5, d.OpenConnections)
	require.Equal(t, 2, d.Idle)
	require.Equal(t, int64(5), d.WaitCount)
	require.Equal(t, 50*time.Millisecond, d.WaitDuration)
}

func TestDiff_ZeroPreviousReturnsCurrentValues(t *testing.T) {
	cur := pool.Stats{OpenConnections: 10, Idle: 4, WaitCount: 20, WaitDuration: 200 * time.Millisecond}

	d := pool.Diff(cur, pool.Stats{})

	require.Equal(t, 10, d.OpenConnections)
	require.Equal(t, 4, d.Idle)
	require.Equal(t, int64(20), d.WaitCount)
	require.Equal(t, 200*time.Millisecond, d.WaitDuration)
}

func TestDiff_CountersDeltaCanBeZero(t *testing.T) {
	same := pool.Stats{OpenConnections: 5, Idle: 2, WaitCount: 7, WaitDuration: 70 * time.Millisecond}

	d := pool.Diff(same, same)

	require.Equal(t, 5, d.OpenConnections)
	require.Equal(t, 2, d.Idle)
	require.Equal(t, int64(0), d.WaitCount)
	require.Equal(t, time.Duration(0), d.WaitDuration)
}
