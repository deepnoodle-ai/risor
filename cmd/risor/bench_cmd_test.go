package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestBenchHandler_SimpleCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping benchmark test in short mode")
	}

	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("bench").
		Args("file?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.Int("iterations", "n").Default(10),
			cli.Int("warmup", "w").Default(2),
		).
		Run(benchHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"bench", "-c", "1 + 2", "-n", "10", "-w", "2"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show benchmark results
	assert.True(t, contains(output, "RESULTS"))
	assert.True(t, contains(output, "Ops/sec"))
	assert.True(t, contains(output, "Min"))
	assert.True(t, contains(output, "Max"))
	assert.True(t, contains(output, "Avg"))
	assert.True(t, contains(output, "Median"))
}

func TestBenchHandler_Error(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("bench").
		Args("file?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.Int("iterations", "n").Default(10),
			cli.Int("warmup", "w").Default(2),
		).
		Run(benchHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"bench", "-c", "undefined_var", "-n", "10", "-w", "2"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	// Should have an error
	assert.NotNil(t, err)
	assert.True(t, contains(err.Error(), "error"))
}

func TestBenchHandler_NoInput(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("bench").
		Args("file?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.Int("iterations", "n").Default(10),
			cli.Int("warmup", "w").Default(2),
		).
		Run(benchHandler)

	err := app.ExecuteArgs([]string{"bench"})

	assert.NotNil(t, err)
	assert.True(t, contains(err.Error(), "no input"))
}

func TestSortDurations(t *testing.T) {
	durations := []time.Duration{
		5 * time.Millisecond,
		2 * time.Millisecond,
		8 * time.Millisecond,
		1 * time.Millisecond,
		3 * time.Millisecond,
	}

	sortDurations(durations)

	// Should be sorted in ascending order
	for i := 1; i < len(durations); i++ {
		assert.True(t, durations[i-1] <= durations[i],
			"durations should be sorted: %v >= %v at index %d",
			durations[i-1], durations[i], i)
	}

	assert.Equal(t, durations[0], 1*time.Millisecond)
	assert.Equal(t, durations[len(durations)-1], 8*time.Millisecond)
}

func TestSortDurations_Empty(t *testing.T) {
	var durations []time.Duration
	sortDurations(durations) // Should not panic
	assert.Equal(t, len(durations), 0)
}

func TestSortDurations_Single(t *testing.T) {
	durations := []time.Duration{5 * time.Millisecond}
	sortDurations(durations)
	assert.Equal(t, durations[0], 5*time.Millisecond)
}

func TestSortDurations_AlreadySorted(t *testing.T) {
	durations := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
	}

	sortDurations(durations)

	assert.Equal(t, durations[0], 1*time.Millisecond)
	assert.Equal(t, durations[1], 2*time.Millisecond)
	assert.Equal(t, durations[2], 3*time.Millisecond)
}

func TestRepeatStr(t *testing.T) {
	tests := []struct {
		s        string
		n        int
		expected string
	}{
		{"a", 3, "aaa"},
		{"ab", 2, "abab"},
		{"x", 0, ""},
		{"", 5, ""},
		{"-", 10, "----------"},
	}

	for _, tt := range tests {
		result := repeatStr(tt.s, tt.n)
		assert.Equal(t, result, tt.expected)
	}
}

func TestPrintHistogram(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	durations := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}

	// Sort first as required
	sortDurations(durations)

	// Capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create minimal styles
	mutedStyle := mutedStyle // Use existing styles from ast_cmd.go
	valueStyle := valueStyle

	printHistogram(durations, mutedStyle, valueStyle)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should have some output (histogram bars)
	assert.True(t, len(output) > 0)
}

func TestPrintHistogram_Empty(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	var durations []time.Duration

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printHistogram(durations, mutedStyle, valueStyle)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Empty input should produce no output
	assert.Equal(t, output, "")
}

func TestGetBenchCode_MultipleInputs(t *testing.T) {
	app := cli.New("test").SetColorEnabled(false)
	var capturedErr error
	app.Command("test").
		Args("file?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
		).
		Run(func(ctx *cli.Context) error {
			_, capturedErr = getBenchCode(ctx)
			return capturedErr
		})

	_ = app.ExecuteArgs([]string{"test", "-c", "1+2", "somefile.risor"})
	assert.NotNil(t, capturedErr)
	assert.True(t, contains(capturedErr.Error(), "multiple"))
}
