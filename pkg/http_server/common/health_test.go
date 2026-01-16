package common

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestExecuteHealthChecks_NoChecks(t *testing.T) {
	ctx := context.Background()
	checks := make(map[string]HealthCheckFunc)

	results, hasErrors := ExecuteHealthChecks(ctx, checks, 5*time.Second, 10)

	if results != nil {
		t.Errorf("expected nil results for no checks, got %v", results)
	}

	if hasErrors {
		t.Error("expected no errors for no checks")
	}
}

func TestExecuteHealthChecks_AllHealthy(t *testing.T) {
	ctx := context.Background()
	checks := map[string]HealthCheckFunc{
		"check1": func(ctx context.Context) error { return nil },
		"check2": func(ctx context.Context) error { return nil },
		"check3": func(ctx context.Context) error { return nil },
	}

	results, hasErrors := ExecuteHealthChecks(ctx, checks, 5*time.Second, 10)

	if hasErrors {
		t.Error("expected no errors")
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for name, result := range results {
		if result.Status != "healthy" {
			t.Errorf("check %s: expected status 'healthy', got '%s'", name, result.Status)
		}

		if result.Error != "" {
			t.Errorf("check %s: expected no error, got '%s'", name, result.Error)
		}
	}
}

func TestExecuteHealthChecks_SomeUnhealthy(t *testing.T) {
	ctx := context.Background()
	checks := map[string]HealthCheckFunc{
		"check1": func(ctx context.Context) error { return nil },
		"check2": func(ctx context.Context) error { return errors.New("database connection failed") },
		"check3": func(ctx context.Context) error { return nil },
	}

	results, hasErrors := ExecuteHealthChecks(ctx, checks, 5*time.Second, 10)

	if !hasErrors {
		t.Error("expected errors")
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Check1 should be healthy
	if results["check1"].Status != "healthy" {
		t.Errorf("check1: expected status 'healthy', got '%s'", results["check1"].Status)
	}

	// Check2 should be unhealthy
	if results["check2"].Status != "unhealthy" {
		t.Errorf("check2: expected status 'unhealthy', got '%s'", results["check2"].Status)
	}

	if results["check2"].Error != "database connection failed" {
		t.Errorf("check2: expected error 'database connection failed', got '%s'", results["check2"].Error)
	}

	// Check3 should be healthy
	if results["check3"].Status != "healthy" {
		t.Errorf("check3: expected status 'healthy', got '%s'", results["check3"].Status)
	}
}

func TestExecuteHealthChecks_Timeout(t *testing.T) {
	ctx := context.Background()
	checks := map[string]HealthCheckFunc{
		"slow": func(ctx context.Context) error {
			// Simulate slow check that blocks longer than timeout
			select {
			case <-time.After(5 * time.Second): // Wait 5 seconds
				return nil
			case <-ctx.Done():
				// Context cancelled - this is what we expect
				return ctx.Err()
			}
		},
	}

	// Timeout after 100ms (much faster than the 5s the check would take)
	results, hasErrors := ExecuteHealthChecks(ctx, checks, 100*time.Millisecond, 10)

	if !hasErrors {
		t.Error("expected timeout error")
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results["slow"].Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy' for timeout, got '%s'", results["slow"].Status)
	}

	// The error message should either be "timeout" or context error
	if results["slow"].Error != "timeout" && results["slow"].Error != "context deadline exceeded" {
		t.Errorf("expected error 'timeout' or context error, got '%s'", results["slow"].Error)
	}
}

func TestExecuteHealthChecks_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	checks := map[string]HealthCheckFunc{
		"check1": func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	// Cancel immediately
	cancel()

	results, hasErrors := ExecuteHealthChecks(ctx, checks, 5*time.Second, 10)

	if !hasErrors {
		t.Error("expected errors due to context cancellation")
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results["check1"].Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got '%s'", results["check1"].Status)
	}
}

// Helper function to format check names uniquely
func formatCheckName(i int) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if i < 10 {
		return "check_0" + string(digits[i])
	}
	return "check_" + string(digits[i/10]) + string(digits[i%10])
}

func TestExecuteHealthChecks_ConcurrencyLimit(t *testing.T) {
	ctx := context.Background()
	const numChecks = 50 // Reduced for test performance
	const maxConcurrent = 10

	// Track concurrent executions
	var currentConcurrent int
	var maxObservedConcurrent int
	var mu sync.Mutex

	checks := make(map[string]HealthCheckFunc)
	for i := 0; i < numChecks; i++ {
		// Use unique names for each check
		name := formatCheckName(i)
		checks[name] = func(ctx context.Context) error {
			mu.Lock()
			currentConcurrent++
			if currentConcurrent > maxObservedConcurrent {
				maxObservedConcurrent = currentConcurrent
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond) // Simulate work

			mu.Lock()
			currentConcurrent--
			mu.Unlock()

			return nil
		}
	}

	results, hasErrors := ExecuteHealthChecks(ctx, checks, 10*time.Second, maxConcurrent)

	if hasErrors {
		t.Error("expected no errors")
	}

	if len(results) != numChecks {
		t.Errorf("expected %d results, got %d", numChecks, len(results))
	}

	// Verify concurrency was limited
	// Allow some margin due to timing
	if maxObservedConcurrent > maxConcurrent+2 {
		t.Errorf("expected max concurrent <= %d (with margin), got %d", maxConcurrent+2, maxObservedConcurrent)
	}
}

func TestExecuteHealthChecks_CheckRespectsContext(t *testing.T) {
	ctx := context.Background()
	checks := map[string]HealthCheckFunc{
		"respectful": func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				return errors.New("check did not respect context")
			}
		},
	}

	// Timeout quickly
	results, hasErrors := ExecuteHealthChecks(ctx, checks, 100*time.Millisecond, 10)

	if !hasErrors {
		t.Error("expected errors")
	}

	if results["respectful"].Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got '%s'", results["respectful"].Status)
	}
}

func TestExecuteHealthChecks_MixedResults(t *testing.T) {
	ctx := context.Background()
	checks := map[string]HealthCheckFunc{
		"healthy1": func(ctx context.Context) error { return nil },
		"healthy2": func(ctx context.Context) error { return nil },
		"unhealthy1": func(ctx context.Context) error {
			return errors.New("redis connection failed")
		},
		"unhealthy2": func(ctx context.Context) error {
			return errors.New("elasticsearch unavailable")
		},
	}

	results, hasErrors := ExecuteHealthChecks(ctx, checks, 5*time.Second, 10)

	if !hasErrors {
		t.Error("expected errors")
	}

	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}

	// Verify healthy checks
	healthyChecks := []string{"healthy1", "healthy2"}
	for _, name := range healthyChecks {
		if results[name].Status != "healthy" {
			t.Errorf("%s: expected status 'healthy', got '%s'", name, results[name].Status)
		}
	}

	// Verify unhealthy checks
	if results["unhealthy1"].Status != "unhealthy" {
		t.Error("unhealthy1: expected status 'unhealthy'")
	}

	if results["unhealthy2"].Status != "unhealthy" {
		t.Error("unhealthy2: expected status 'unhealthy'")
	}
}

// Benchmark
func BenchmarkExecuteHealthChecks_10Checks(b *testing.B) {
	ctx := context.Background()
	checks := make(map[string]HealthCheckFunc)

	for i := 0; i < 10; i++ {
		name := "check" + string(rune('0'+i))
		checks[name] = func(ctx context.Context) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExecuteHealthChecks(ctx, checks, 5*time.Second, 10)
	}
}
