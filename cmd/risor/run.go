package main

import (
	"context"
	"encoding/json"
	goerrors "errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func runHandler(ctx *cli.Context) error {
	// Handle CPU profiling
	if profilePath := ctx.String("cpu-profile"); profilePath != "" {
		f, err := os.Create(profilePath)
		if err != nil {
			return err
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
		handleSigForProfiler()
	}

	// Check if we should run REPL
	if shouldRunRepl(ctx) {
		replEnv, err := getReplEnv(ctx)
		if err != nil {
			return err
		}
		return runRepl(ctx.Context(), replEnv)
	}

	// Get the code to execute
	code, err := getRisorCode(ctx)
	if err != nil {
		return err
	}

	// Get Risor options
	opts, err := getRisorOptions(ctx)
	if err != nil {
		return err
	}
	opts, err = withPipedStdin(ctx, code, opts)
	if err != nil {
		return err
	}

	// Execute the code
	start := time.Now()
	if file := ctx.Arg(0); file != "" {
		opts = append(opts, risor.WithFilename(file))
	}

	result, err := risor.Eval(ctx.Context(), code, opts...)
	if err != nil {
		return formatRisorError(ctx, err)
	}
	dt := time.Since(start)

	// Print the result
	output, err := formatOutput(ctx, result)
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Println(output)
	}

	// Optionally print execution time
	if ctx.Bool("timing") {
		fmt.Printf("%v\n", dt)
	}

	return nil
}

func versionHandler(ctx *cli.Context) error {
	format := strings.ToLower(ctx.String("output"))
	if format == "json" {
		info, err := json.MarshalIndent(map[string]any{
			"version": version,
			"commit":  commit,
			"date":    date,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(info))
	} else {
		fmt.Println(version)
	}
	return nil
}

func getRisorOptions(ctx *cli.Context) ([]risor.Option, error) {
	var opts []risor.Option
	if !ctx.Bool("no-default-globals") {
		opts = append(opts, risor.WithEnv(risor.Builtins()))
	}
	// Provide print in CLI mode (not available in library mode by design)
	opts = append(opts, risor.WithEnv(map[string]any{
		"print": newPrintBuiltin(),
	}))
	if vars, err := parseVarFlags(ctx.Strings("var")); err != nil {
		return nil, err
	} else if len(vars) > 0 {
		opts = append(opts, risor.WithEnv(vars))
	}
	if vars, err := parseJSONVarFlag(ctx.String("var-json")); err != nil {
		return nil, err
	} else if len(vars) > 0 {
		opts = append(opts, risor.WithEnv(vars))
	}
	return opts, nil
}

// withPipedStdin adds piped input as the stdin variable when stdin isn't
// being used to read code (via --stdin flag) and the code reads the global
// stdin. Reading blocks until the pipe closes, so code that doesn't read
// stdin must not trigger it.
func withPipedStdin(ctx *cli.Context, code string, opts []risor.Option) ([]risor.Option, error) {
	if ctx.Bool("stdin") || !cli.IsPiped() || !readsStdin(code, opts) {
		return opts, nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	if len(data) > 0 {
		opts = append(opts, risor.WithEnv(map[string]any{
			"stdin": string(data),
		}))
	}
	return opts, nil
}

// readsStdin reports whether code reads a global stdin variable that opts
// don't define. The compiler resolves the name, so a map key, a parameter, or
// the script's own variable named stdin doesn't count, and neither does a
// stdin set with --var. Code that fails to compile for another reason is
// reported as not reading stdin; evaluation reports the error.
func readsStdin(code string, opts []risor.Option) bool {
	ctx := context.Background()
	if _, err := risor.Compile(ctx, code, opts...); err == nil {
		return false
	}
	withStdin := append(opts[:len(opts):len(opts)], risor.WithEnv(map[string]any{"stdin": ""}))
	_, err := risor.Compile(ctx, code, withStdin...)
	return err == nil
}

// parseJSONVarFlag parses a --var-json flag value as a JSON object.
func parseJSONVarFlag(value string) (map[string]any, error) {
	if value == "" {
		return nil, nil
	}
	if !json.Valid([]byte(value)) {
		return nil, fmt.Errorf("--var-json: not valid JSON (expected a JSON object, e.g. '{\"key\": \"value\"}')")
	}
	var vars map[string]any
	if err := json.Unmarshal([]byte(value), &vars); err != nil {
		return nil, fmt.Errorf("--var-json: expected a JSON object (e.g. '{\"key\": \"value\"}'), got %s", jsonTypeLabel(value))
	}
	return vars, nil
}

func jsonTypeLabel(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return "empty string"
	}
	switch value[0] {
	case '[':
		return "an array"
	case '"':
		return "a string"
	case 't', 'f':
		return "a boolean"
	case 'n':
		return "null"
	default:
		if value[0] == '-' || (value[0] >= '0' && value[0] <= '9') {
			return "a number"
		}
		return "unknown type"
	}
}

// parseVarFlags parses --var key=value flags into a map.
func parseVarFlags(flags []string) (map[string]any, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	vars := make(map[string]any, len(flags))
	for _, flag := range flags {
		key, value, ok := strings.Cut(flag, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("malformed --var flag: expected key=value, got %q", flag)
		}
		vars[key] = value
	}
	return vars, nil
}

func getReplEnv(ctx *cli.Context) (map[string]any, error) {
	var env map[string]any
	if !ctx.Bool("no-default-globals") {
		env = risor.Builtins()
	}
	mergeInto := func(vars map[string]any) {
		if env == nil {
			env = make(map[string]any)
		}
		for k, v := range vars {
			env[k] = v
		}
	}
	mergeInto(map[string]any{"print": newPrintBuiltin()})
	if vars, err := parseVarFlags(ctx.Strings("var")); err != nil {
		return nil, err
	} else if len(vars) > 0 {
		mergeInto(vars)
	}
	if vars, err := parseJSONVarFlag(ctx.String("var-json")); err != nil {
		return nil, err
	} else if len(vars) > 0 {
		mergeInto(vars)
	}
	return env, nil
}

func shouldRunRepl(ctx *cli.Context) bool {
	// No REPL if explicitly disabled
	if ctx.Bool("no-repl") {
		return false
	}
	// No REPL if reading from stdin
	if ctx.Bool("stdin") {
		return false
	}
	// No REPL if code provided via -c
	if ctx.IsSet("code") {
		return false
	}
	// No REPL if file provided
	if ctx.Arg(0) != "" {
		return false
	}
	// Only run REPL if interactive
	return ctx.Interactive()
}

func getRisorCode(ctx *cli.Context) (string, error) {
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
		return "", goerrors.New("multiple input sources specified")
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

func handleSigForProfiler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		pprof.StopCPUProfile()
		os.Exit(1)
	}()
}

// formatRisorError formats a Risor error with colors and professional styling.
func formatRisorError(ctx *cli.Context, err error) error {
	useColor := !ctx.Bool("no-color") && color.ShouldColorize(os.Stderr)
	if formatted := formattedErrors(err); formatted != nil {
		return goerrors.New(errors.NewFormatter(useColor).FormatMultiple(formatted))
	}
	return err
}

// formattedErrors converts a Risor error to the errors to display, or returns
// nil if err is not a Risor error.
func formattedErrors(err error) []*errors.FormattedError {
	// Check for multi-error types (parser errors with multiple errors)
	if multiErr, ok := err.(interface {
		ToFormattedMultiple() []*errors.FormattedError
	}); ok {
		return multiErr.ToFormattedMultiple()
	}

	// Check for single formattable errors (StructuredError, CompileError, etc.)
	if formattable, ok := err.(errors.FormattableError); ok {
		return []*errors.FormattedError{formattable.ToFormatted()}
	}

	return nil
}

func newPrintBuiltin() *object.Builtin {
	return object.NewBuiltin("print", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		// Check before formatting, as string() does, so that a value whose
		// text is too large, such as a list that holds one list many
		// times, is never formatted
		if err := object.CheckFormatSize(ctx, "print", args...); err != nil {
			return nil, err
		}
		values := make([]any, len(args))
		for i, arg := range args {
			switch arg.(type) {
			case *object.List, *object.Map, *object.Cell, *object.Partial:
				// Apply the byte limit in ctx, not the default of Inspect
				values[i] = object.InspectWithLimits(ctx, arg)
			default:
				values[i] = object.PrintableValue(arg)
			}
		}
		fmt.Println(values...)
		return object.Nil, nil
	})
}
