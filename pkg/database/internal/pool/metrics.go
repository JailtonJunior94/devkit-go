package pool

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

const DefaultScrapeInterval = 10 * time.Second

type Scraper struct {
	stop chan struct{}
	done chan struct{}
}

func NewScraper(
	statsFunc func() Stats,
	metrics observability.Metrics,
	interval time.Duration,
	attrs ...observability.Field,
) *Scraper {
	if interval <= 0 {
		interval = DefaultScrapeInterval
	}
	s := &Scraper{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go s.run(statsFunc, metrics, interval, attrs)
	return s
}

func (s *Scraper) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Scraper) run(
	statsFunc func() Stats,
	metrics observability.Metrics,
	interval time.Duration,
	attrs []observability.Field,
) {
	defer close(s.done)

	var openConns, idleConns observability.UpDownCounter
	var waitCount observability.Counter
	var waitDuration observability.Counter

	if metrics != nil {
		openConns = metrics.UpDownCounter(
			"database.pool.connections_open",
			"Number of open connections in the pool",
			"{connections}",
		)
		idleConns = metrics.UpDownCounter(
			"database.pool.connections_idle",
			"Number of idle connections in the pool",
			"{connections}",
		)
		waitCount = metrics.Counter(
			"database.pool.wait_count",
			"Number of times an acquire had to wait for a connection",
			"{events}",
		)
		waitDuration = metrics.Counter(
			"database.pool.wait_duration_ms",
			"Total time spent waiting for connections",
			"ms",
		)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var prevOpen, prevIdle int64
	var prevWaitCount int64
	var prevWaitDuration time.Duration

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			cur := statsFunc()
			if metrics == nil {
				continue
			}

			ctx := context.Background()

			curOpen := int64(cur.OpenConnections)
			curIdle := int64(cur.Idle)
			openConns.Add(ctx, curOpen-prevOpen, attrs...)
			idleConns.Add(ctx, curIdle-prevIdle, attrs...)
			prevOpen = curOpen
			prevIdle = curIdle

			deltaWait := cur.WaitCount - prevWaitCount
			if deltaWait > 0 {
				waitCount.Add(ctx, deltaWait, attrs...)
				prevWaitCount = cur.WaitCount
			}

			deltaDur := cur.WaitDuration - prevWaitDuration
			if deltaDur > 0 {
				waitDuration.Add(ctx, deltaDur.Milliseconds(), attrs...)
				prevWaitDuration = cur.WaitDuration
			}
		}
	}
}
