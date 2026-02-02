package testing

import (
	"fmt"
	"io"
	"strings"

	"github.com/deepnoodle-ai/wonton/color"
)

// OutputConfig configures output formatting.
type OutputConfig struct {
	// Writer is where output is written. Default is os.Stdout.
	Writer io.Writer

	// Verbose shows t.log() output for all tests.
	Verbose bool

	// UseColor enables ANSI color codes.
	UseColor bool
}

// Output handles formatting and printing test results.
type Output struct {
	w        io.Writer
	verbose  bool
	useColor bool
}

// NewOutput creates a new Output formatter.
func NewOutput(cfg OutputConfig) *Output {
	return &Output{
		w:        cfg.Writer,
		verbose:  cfg.Verbose,
		useColor: cfg.UseColor,
	}
}

// StartTest prints the "=== RUN" line for a test.
func (o *Output) StartTest(name string) {
	fmt.Fprintf(o.w, "=== RUN   %s\n", name)
}

// EndTest prints the result line for a test (--- PASS, --- FAIL, etc.).
func (o *Output) EndTest(result *TestResult) {
	status := result.Status.String()
	duration := result.Duration.Seconds()

	// Color the status
	var statusStr string
	switch result.Status {
	case StatusPassed:
		statusStr = o.colorize(color.Green, "--- PASS:")
	case StatusFailed:
		statusStr = o.colorize(color.Red, "--- FAIL:")
	case StatusSkipped:
		statusStr = o.colorize(color.Yellow, "--- SKIP:")
	case StatusError:
		statusStr = o.colorize(color.Red, "--- ERROR:")
	default:
		statusStr = fmt.Sprintf("--- %s:", status)
	}

	fmt.Fprintf(o.w, "%s %s (%.3fs)\n", statusStr, result.Name, duration)

	// Print skip reason
	if result.Status == StatusSkipped && result.SkipReason != "" {
		fmt.Fprintf(o.w, "    %s\n", result.SkipReason)
	}

	// Print error
	if result.Status == StatusError && result.Error != nil {
		fmt.Fprintf(o.w, "    %s\n", result.Error.Error())
	}

	// Print assertion failures
	for _, failure := range result.Failures {
		o.printFailure(&failure)
	}

	// Print logs if verbose or if test failed
	if o.verbose || result.Status == StatusFailed {
		for _, log := range result.Logs {
			fmt.Fprintf(o.w, "    %s\n", log)
		}
	}
}

// printFailure prints details of an assertion failure.
func (o *Output) printFailure(f *AssertionError) {
	// Location prefix
	loc := ""
	if f.File != "" {
		loc = f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		loc += ": "
	}

	// Message
	fmt.Fprintf(o.w, "    %s%s\n", loc, f.Message)

	// Got/Want values
	if f.Got != nil {
		fmt.Fprintf(o.w, "        %s:  %s\n",
			o.colorize(color.Red, "got"),
			f.Got.Inspect())
	}
	if f.Want != nil {
		fmt.Fprintf(o.w, "        %s: %s\n",
			o.colorize(color.Green, "want"),
			f.Want.Inspect())
	}
}

// CompileError prints a compilation error for a test file.
func (o *Output) CompileError(filename string, err error) {
	fmt.Fprintf(o.w, "%s %s\n",
		o.colorize(color.Red, "COMPILE ERROR:"),
		filename)
	fmt.Fprintf(o.w, "    %s\n", err.Error())
}

// Summary prints the final summary line.
func (o *Output) Summary(summary *Summary) {
	fmt.Fprintln(o.w)

	// Overall status
	if summary.Success() {
		fmt.Fprintln(o.w, o.colorize(color.Green, "PASS"))
	} else {
		fmt.Fprintln(o.w, o.colorize(color.Red, "FAIL"))
	}

	// Counts
	parts := []string{}
	if summary.Passed > 0 {
		parts = append(parts, o.colorize(color.Green, fmt.Sprintf("%d passed", summary.Passed)))
	}
	if summary.Failed > 0 {
		parts = append(parts, o.colorize(color.Red, fmt.Sprintf("%d failed", summary.Failed)))
	}
	if summary.Skipped > 0 {
		parts = append(parts, o.colorize(color.Yellow, fmt.Sprintf("%d skipped", summary.Skipped)))
	}
	if summary.Errors > 0 {
		parts = append(parts, o.colorize(color.Red, fmt.Sprintf("%d errors", summary.Errors)))
	}

	if len(parts) > 0 {
		fmt.Fprintln(o.w, strings.Join(parts, ", "))
	}
}

// colorize applies color if enabled.
func (o *Output) colorize(c color.Color, s string) string {
	if o.useColor {
		return c.Apply(s)
	}
	return s
}

// PrintResults prints all results in Go test style.
func (o *Output) PrintResults(summary *Summary) {
	// Print compile errors first
	for _, file := range summary.Files {
		if file.CompileErr != nil {
			o.CompileError(file.Filename, file.CompileErr)
		}
	}

	// Print test results
	for _, file := range summary.Files {
		for _, test := range file.Tests {
			o.StartTest(test.Name)
			o.EndTest(test)
		}
	}

	o.Summary(summary)
}
