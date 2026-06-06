package pool

import "time"

type Stats struct {
	OpenConnections int
	Idle            int
	WaitCount       int64
	WaitDuration    time.Duration
}

func Diff(current, previous Stats) Stats {
	return Stats{
		OpenConnections: current.OpenConnections,
		Idle:            current.Idle,
		WaitCount:       current.WaitCount - previous.WaitCount,
		WaitDuration:    current.WaitDuration - previous.WaitDuration,
	}
}
