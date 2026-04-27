package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

func TestParseBenchmarkLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantResult benchmarkResult
		wantErr    string
	}{
		{
			name:   "parses benchmark result with cpu suffix",
			line:   "BenchmarkStart-10     1000000     155.2 ns/op     64 B/op     2 allocs/op",
			wantOK: true,
			wantResult: benchmarkResult{
				name:     "BenchmarkStart",
				nsOp:     155.2,
				bytesOp:  64,
				allocsOp: 2,
			},
		},
		{
			name:   "ignores non benchmark line",
			line:   "PASS",
			wantOK: false,
		},
		{
			name:    "rejects missing benchmem metrics",
			line:    "BenchmarkStart-10     1000000     155.2 ns/op",
			wantErr: "invalid benchmark line",
		},
		{
			name:    "rejects unparsable metric",
			line:    "BenchmarkStart-10     1000000     nope ns/op     64 B/op     2 allocs/op",
			wantErr: "parse ns/op",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok, err := parseBenchmarkLine(tt.line)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.wantOK {
				t.Fatalf("unexpected ok: %v", ok)
			}
			if got != tt.wantResult {
				t.Fatalf("unexpected result: %#v", got)
			}
		})
	}
}

func TestCompareBenchmarkResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		baseline         map[string]benchmarkResult
		current          map[string]benchmarkResult
		budget           float64
		wantRegressions  int
		wantFirstMetric  string
		wantFirstPercent float64
	}{
		{
			name: "accepts results within budget",
			baseline: map[string]benchmarkResult{
				"BenchmarkStart": {name: "BenchmarkStart", nsOp: 100, bytesOp: 80, allocsOp: 4},
			},
			current: map[string]benchmarkResult{
				"BenchmarkStart": {name: "BenchmarkStart", nsOp: 104, bytesOp: 80, allocsOp: 4},
			},
			budget: 5,
		},
		{
			name: "ignores timing drift and reports only benchmem regressions",
			baseline: map[string]benchmarkResult{
				"BenchmarkStart": {name: "BenchmarkStart", nsOp: 100, bytesOp: 80, allocsOp: 4},
			},
			current: map[string]benchmarkResult{
				"BenchmarkStart": {name: "BenchmarkStart", nsOp: 106, bytesOp: 86, allocsOp: 4},
			},
			budget:           5,
			wantRegressions:  1,
			wantFirstMetric:  "B/op",
			wantFirstPercent: 7.5,
		},
		{
			name: "reports allocation regression from zero baseline",
			baseline: map[string]benchmarkResult{
				"BenchmarkSpanFromContext_NoSpan": {name: "BenchmarkSpanFromContext_NoSpan", nsOp: 1, bytesOp: 0, allocsOp: 0},
			},
			current: map[string]benchmarkResult{
				"BenchmarkSpanFromContext_NoSpan": {name: "BenchmarkSpanFromContext_NoSpan", nsOp: 1, bytesOp: 8, allocsOp: 1},
			},
			budget:           5,
			wantRegressions:  2,
			wantFirstMetric:  "B/op",
			wantFirstPercent: 100,
		},
		{
			name: "reports missing benchmark",
			baseline: map[string]benchmarkResult{
				"BenchmarkPropagationInject": {name: "BenchmarkPropagationInject", nsOp: 100, bytesOp: 80, allocsOp: 4},
			},
			current:          map[string]benchmarkResult{},
			budget:           5,
			wantRegressions:  1,
			wantFirstMetric:  "missing",
			wantFirstPercent: 100,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			regressions := compareBenchmarkResults(tt.baseline, tt.current, tt.budget)

			if len(regressions) != tt.wantRegressions {
				t.Fatalf("unexpected regression count: %d", len(regressions))
			}
			if tt.wantRegressions == 0 {
				return
			}
			if regressions[0].metric != tt.wantFirstMetric {
				t.Fatalf("unexpected metric: %q", regressions[0].metric)
			}
			if regressions[0].percent != tt.wantFirstPercent {
				t.Fatalf("unexpected percent: %.2f", regressions[0].percent)
			}
		})
	}
}

func TestFormatRegressionErrorUsesTypedBenchmarkError(t *testing.T) {
	t.Parallel()

	err := formatRegressionError([]regression{
		{name: "BenchmarkStart", metric: "B/op", base: 100, actual: 106, percent: 6},
	})

	if !errors.Is(err, observability.ErrBenchmarkRegression) {
		t.Fatalf("expected ErrBenchmarkRegression, got %v", err)
	}
	if !strings.Contains(err.Error(), "BenchmarkStart B/op regressed 6.00%") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}
