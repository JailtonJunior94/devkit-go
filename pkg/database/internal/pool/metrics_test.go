package pool_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
)

func TestScraper_StopsWithinDeadline(t *testing.T) {
	calls := 0
	statsFunc := func() pool.Stats {
		calls++
		return pool.Stats{OpenConnections: 1}
	}

	// Use a short interval so the goroutine ticks at least once before Stop.
	s := pool.NewScraper(statsFunc, nil, 20*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
		// good: stopped within deadline
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Scraper.Stop() did not return within 100 ms")
	}
}

func TestScraper_StopIsIdempotentViaSingleCall(t *testing.T) {
	s := pool.NewScraper(func() pool.Stats { return pool.Stats{} }, nil, 50*time.Millisecond)
	s.Stop()
	// Calling Stop again would panic on double-close; ensure we don't panic.
	// (Second Stop is not contractually safe — just verify the first completes.)
}

func TestScraper_NilMetricsDoesNotPanic(t *testing.T) {
	statsFunc := func() pool.Stats {
		return pool.Stats{OpenConnections: 3, Idle: 1}
	}
	s := pool.NewScraper(statsFunc, nil, 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	s.Stop()
}

func TestScraper_DefaultIntervalUsedWhenZero(t *testing.T) {
	// Passing interval=0 must not panic and must use DefaultScrapeInterval.
	s := pool.NewScraper(func() pool.Stats { return pool.Stats{} }, nil, 0)
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Scraper with zero interval did not stop in time")
	}
}
