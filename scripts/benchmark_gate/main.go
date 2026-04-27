package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

const defaultBudgetPercent = 5.0

type benchmarkResult struct {
	name     string
	nsOp     float64
	bytesOp  float64
	allocsOp float64
}

type regression struct {
	name    string
	metric  string
	actual  float64
	base    float64
	percent float64
}

func main() {
	baselinePath := flag.String("baseline", "pkg/observability/testdata/benchmarks/otel_benchmarks.txt", "baseline benchmark output")
	inputPath := flag.String("input", "", "current benchmark output; stdin is used when empty")
	budgetPercent := flag.Float64("budget", defaultBudgetPercent, "maximum allowed regression percentage")
	flag.Parse()

	if err := run(*baselinePath, *inputPath, *budgetPercent, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(baselinePath, inputPath string, budgetPercent float64, stdin io.Reader, stdout io.Writer) error {
	if baselinePath == "" {
		return errors.New("benchmark gate: baseline path is required")
	}

	budget, err := observability.NewBenchmarkBudget("observability_otel", budgetPercent, baselinePath)
	if err != nil {
		return err
	}

	baselineFile, err := os.Open(budget.BaselineFile())
	if err != nil {
		return fmt.Errorf("benchmark gate: open baseline: %w", err)
	}
	defer func() {
		_ = baselineFile.Close()
	}()

	current, err := readBenchmarkInput(inputPath, stdin)
	if err != nil {
		return err
	}
	defer func() {
		_ = current.Close()
	}()

	baselineResults, err := parseBenchmarkResults(baselineFile)
	if err != nil {
		return fmt.Errorf("benchmark gate: parse baseline: %w", err)
	}
	currentResults, err := parseBenchmarkResults(current)
	if err != nil {
		return fmt.Errorf("benchmark gate: parse current results: %w", err)
	}

	regressions := compareBenchmarkResults(baselineResults, currentResults, budget.MaxRegressionPercent())
	if len(regressions) > 0 {
		return formatRegressionError(regressions)
	}

	_, _ = fmt.Fprintf(stdout, "benchmark gate passed: %d benchmarks within %.2f%% budget\n", len(currentResults), budget.MaxRegressionPercent())
	return nil
}

func readBenchmarkInput(inputPath string, stdin io.Reader) (io.ReadCloser, error) {
	if inputPath == "" {
		return io.NopCloser(stdin), nil
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("benchmark gate: open current results: %w", err)
	}
	return inputFile, nil
}

func parseBenchmarkResults(reader io.Reader) (map[string]benchmarkResult, error) {
	results := make(map[string]benchmarkResult)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		result, ok, err := parseBenchmarkLine(scanner.Text())
		if err != nil {
			return nil, err
		}
		if ok {
			results[result.name] = result
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("no benchmark results found")
	}
	return results, nil
}

func parseBenchmarkLine(line string) (benchmarkResult, bool, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "Benchmark") {
		return benchmarkResult{}, false, nil
	}
	if len(fields) < 7 {
		return benchmarkResult{}, false, fmt.Errorf("invalid benchmark line %q", line)
	}

	nsOp, err := parseMetric(fields, "ns/op")
	if err != nil {
		return benchmarkResult{}, false, fmt.Errorf("%s: %w", fields[0], err)
	}
	bytesOp, err := parseMetric(fields, "B/op")
	if err != nil {
		return benchmarkResult{}, false, fmt.Errorf("%s: %w", fields[0], err)
	}
	allocsOp, err := parseMetric(fields, "allocs/op")
	if err != nil {
		return benchmarkResult{}, false, fmt.Errorf("%s: %w", fields[0], err)
	}

	return benchmarkResult{
		name:     normalizeBenchmarkName(fields[0]),
		nsOp:     nsOp,
		bytesOp:  bytesOp,
		allocsOp: allocsOp,
	}, true, nil
}

func parseMetric(fields []string, unit string) (float64, error) {
	for i := 1; i < len(fields)-1; i++ {
		if fields[i+1] != unit {
			continue
		}
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", unit, err)
		}
		return value, nil
	}
	return 0, fmt.Errorf("missing %s", unit)
}

func normalizeBenchmarkName(name string) string {
	if index := strings.LastIndexByte(name, '-'); index > len("Benchmark") {
		return name[:index]
	}
	return name
}

func compareBenchmarkResults(baseline, current map[string]benchmarkResult, budgetPercent float64) []regression {
	regressions := make([]regression, 0)
	for name, baselineResult := range baseline {
		currentResult, ok := current[name]
		if !ok {
			regressions = append(regressions, regression{name: name, metric: "missing", percent: 100})
			continue
		}
		regressions = appendMetricRegression(regressions, name, "B/op", baselineResult.bytesOp, currentResult.bytesOp, budgetPercent)
		regressions = appendMetricRegression(regressions, name, "allocs/op", baselineResult.allocsOp, currentResult.allocsOp, budgetPercent)
	}
	return regressions
}

func appendMetricRegression(regressions []regression, name, metric string, baseline, actual, budgetPercent float64) []regression {
	if baseline == 0 {
		if actual == 0 {
			return regressions
		}
		return append(regressions, regression{name: name, metric: metric, actual: actual, base: baseline, percent: 100})
	}

	percent := ((actual - baseline) / baseline) * 100
	if percent <= budgetPercent {
		return regressions
	}
	return append(regressions, regression{name: name, metric: metric, actual: actual, base: baseline, percent: percent})
}

func formatRegressionError(regressions []regression) error {
	lines := make([]string, 0, len(regressions)+1)
	lines = append(lines, "benchmark gate failed:")
	for _, regression := range regressions {
		if regression.metric == "missing" {
			lines = append(lines, fmt.Sprintf("- %s missing from current benchmark output", regression.name))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s %s regressed %.2f%% (baseline %.2f, actual %.2f)", regression.name, regression.metric, regression.percent, regression.base, regression.actual))
	}
	return observability.NewBenchmarkRegressionError(strings.Join(lines, "\n"), regressions[0].percent)
}
