package pool

import "time"

// Stats is a point-in-time snapshot of pool metrics.
type Stats struct {
	OpenConnections int
	Idle            int
	WaitCount       int64
	WaitDuration    time.Duration
}

// Diff returns the delta between current and previous snapshots.
// Gauge fields (OpenConnections, Idle) keep the current absolute value.
// Counter fields (WaitCount, WaitDuration) are subtracted to give the interval delta.
func Diff(current, previous Stats) Stats {
	return Stats{
		OpenConnections: current.OpenConnections,
		Idle:            current.Idle,
		WaitCount:       current.WaitCount - previous.WaitCount,
		WaitDuration:    current.WaitDuration - previous.WaitDuration,
	}
}
