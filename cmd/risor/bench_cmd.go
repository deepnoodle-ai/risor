package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/tui"
	"github.com/risor-io/risor"
)

// BenchResult holds benchmark statistics
type BenchResult struct {
	Iterations    int     `json:"iterations"`
	Warmup        int     `json:"warmup"`
	TotalNs       int64   `json:"total_ns"`
	TotalDuration string  `json:"total_duration"`
	OpsPerSec     float64 `json:"ops_per_sec"`
	MinNs         int64   `json:"min_ns"`
	MaxNs         int64   `json:"max_ns"`
	AvgNs         int64   `json:"avg_ns"`
	MedianNs      int64   `json:"median_ns"`
	P95Ns         int64   `json:"p95_ns"`
	P99Ns         int64   `json:"p99_ns"`
}

func benchHandler(ctx *cli.Context) error {
	// Get code from -c flag, --stdin, or file argument
	code, err := getBenchCode(ctx)
	if err != nil {
		return err
	}

	iterations := ctx.Int("iterations")
	if iterations <= 0 {
		iterations = 1000
	}

	warmup := ctx.Int("warmup")
	if warmup < 0 {
		warmup = 100
	}

	outputFormat := ctx.String("output")

	// Styles
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	labelStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220})
	valueStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 220, B: 100})
	mutedStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 120, G: 120, B: 130})

	if outputFormat != "json" {
		fmt.Println(tui.Sprint(tui.Text("Risor Benchmark").Style(titleStyle)))
		fmt.Println()

		// Show configuration
		fmt.Println(tui.Sprint(tui.Group(
			tui.Text("Iterations: ").Style(labelStyle),
			tui.Text("%d", iterations).Style(valueStyle),
		)))
		fmt.Println(tui.Sprint(tui.Group(
			tui.Text("Warmup:     ").Style(labelStyle),
			tui.Text("%d", warmup).Style(valueStyle),
		)))
		fmt.Println()

		// First, verify the code runs without error
		fmt.Println(tui.Sprint(tui.Text("Verifying code...").Style(mutedStyle)))
	}

	env := risor.Builtins()
	_, verifyErr := risor.Eval(context.Background(), code, risor.WithEnv(env))
	if verifyErr != nil {
		return fmt.Errorf("code error: %w", verifyErr)
	}

	if outputFormat != "json" {
		fmt.Println(tui.Sprint(tui.Text("Code verified OK").Style(valueStyle)))
		fmt.Println()

		// Warmup phase
		if warmup > 0 {
			fmt.Println(tui.Sprint(tui.Text("Warming up...").Style(mutedStyle)))
		}
	}

	for i := 0; i < warmup; i++ {
		env := risor.Builtins()
		_, _ = risor.Eval(context.Background(), code, risor.WithEnv(env))
	}

	// Force GC before benchmark
	runtime.GC()

	if outputFormat != "json" {
		fmt.Println(tui.Sprint(tui.Text("Running benchmark...").Style(mutedStyle)))
	}

	var totalDuration time.Duration
	var minDuration time.Duration = time.Hour
	var maxDuration time.Duration

	durations := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		env := risor.Builtins()
		start := time.Now()
		_, _ = risor.Eval(context.Background(), code, risor.WithEnv(env))
		elapsed := time.Since(start)

		durations[i] = elapsed
		totalDuration += elapsed

		if elapsed < minDuration {
			minDuration = elapsed
		}
		if elapsed > maxDuration {
			maxDuration = elapsed
		}
	}

	// Calculate statistics
	avgDuration := totalDuration / time.Duration(iterations)

	// Calculate median
	sortDurations(durations)
	medianDuration := durations[iterations/2]

	// Calculate p95 and p99
	p95Duration := durations[int(float64(iterations)*0.95)]
	p99Duration := durations[int(float64(iterations)*0.99)]

	// JSON output
	if outputFormat == "json" {
		result := BenchResult{
			Iterations:    iterations,
			Warmup:        warmup,
			TotalNs:       totalDuration.Nanoseconds(),
			TotalDuration: totalDuration.Round(time.Microsecond).String(),
			OpsPerSec:     float64(iterations) / totalDuration.Seconds(),
			MinNs:         minDuration.Nanoseconds(),
			MaxNs:         maxDuration.Nanoseconds(),
			AvgNs:         avgDuration.Nanoseconds(),
			MedianNs:      medianDuration.Nanoseconds(),
			P95Ns:         p95Duration.Nanoseconds(),
			P99Ns:         p99Duration.Nanoseconds(),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Print results
	fmt.Println()
	fmt.Println(tui.Sprint(tui.Text("RESULTS").Style(titleStyle)))
	fmt.Println(tui.Sprint(tui.Text("%s", repeatStr("-", 40)).Style(mutedStyle)))

	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("Total time:  ").Style(labelStyle),
		tui.Text("%v", totalDuration.Round(time.Microsecond)).Style(valueStyle),
	)))
	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("Ops/sec:     ").Style(labelStyle),
		tui.Text("%.2f", float64(iterations)/totalDuration.Seconds()).Style(valueStyle),
	)))
	fmt.Println()

	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("Min:         ").Style(labelStyle),
		tui.Text("%v", minDuration.Round(time.Microsecond)).Style(valueStyle),
	)))
	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("Max:         ").Style(labelStyle),
		tui.Text("%v", maxDuration.Round(time.Microsecond)).Style(valueStyle),
	)))
	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("Avg:         ").Style(labelStyle),
		tui.Text("%v", avgDuration.Round(time.Microsecond)).Style(valueStyle),
	)))
	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("Median:      ").Style(labelStyle),
		tui.Text("%v", medianDuration.Round(time.Microsecond)).Style(valueStyle),
	)))
	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("p95:         ").Style(labelStyle),
		tui.Text("%v", p95Duration.Round(time.Microsecond)).Style(valueStyle),
	)))
	fmt.Println(tui.Sprint(tui.Group(
		tui.Text("p99:         ").Style(labelStyle),
		tui.Text("%v", p99Duration.Round(time.Microsecond)).Style(valueStyle),
	)))

	// Print histogram
	fmt.Println()
	fmt.Println(tui.Sprint(tui.Text("DISTRIBUTION").Style(titleStyle)))
	printHistogram(durations, mutedStyle, valueStyle)

	return nil
}

func getBenchCode(ctx *cli.Context) (string, error) {
	codeSet := ctx.IsSet("code")
	stdinSet := ctx.Bool("stdin")
	fileProvided := ctx.Arg(0) != ""

	// Check for conflicting input sources
	count := 0
	if codeSet {
		count++
	}
	if stdinSet {
		count++
	}
	if fileProvided {
		count++
	}
	if count > 1 {
		return "", errors.New("multiple input sources specified")
	}
	if count == 0 {
		return "", errors.New("no input provided")
	}

	if stdinSet {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	if fileProvided {
		data, err := os.ReadFile(ctx.Arg(0))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return ctx.String("code"), nil
}

// sortDurations sorts durations in ascending order (simple insertion sort for clarity)
func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func printHistogram(durations []time.Duration, mutedStyle, valueStyle tui.Style) {
	if len(durations) == 0 {
		return
	}

	// Create buckets
	minD := durations[0]
	maxD := durations[len(durations)-1]
	bucketCount := 10
	bucketSize := (maxD - minD) / time.Duration(bucketCount)
	if bucketSize == 0 {
		bucketSize = time.Microsecond
	}

	buckets := make([]int, bucketCount)
	for _, d := range durations {
		bucket := int((d - minD) / bucketSize)
		if bucket >= bucketCount {
			bucket = bucketCount - 1
		}
		buckets[bucket]++
	}

	// Find max for scaling
	maxCount := 0
	for _, count := range buckets {
		if count > maxCount {
			maxCount = count
		}
	}

	// Print histogram
	barWidth := 30
	for i, count := range buckets {
		lower := minD + time.Duration(i)*bucketSize
		upper := lower + bucketSize

		// Scale bar
		barLen := 0
		if maxCount > 0 {
			barLen = count * barWidth / maxCount
		}
		bar := repeatStr("█", barLen) + repeatStr("░", barWidth-barLen)

		tui.Print(tui.Group(
			tui.Text("%6v-%6v ", lower.Round(time.Microsecond), upper.Round(time.Microsecond)).Style(mutedStyle),
			tui.Text("%s", bar).Style(valueStyle),
			tui.Text(" %d", count).Style(mutedStyle),
		))
	}
}
