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

	// Get Risor options
	opts, err := getRisorOptions(ctx, true)
	if err != nil {
		return err
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

func getRisorOptions(ctx *cli.Context, injectStdin bool) ([]risor.Option, error) {
	var opts []risor.Option
	if !ctx.Bool("no-default-globals") {
		opts = append(opts, risor.WithEnv(risor.Builtins()))
	}
	// Provide print in CLI mode (not available in library mode by design)
	opts = append(opts, risor.WithEnv(map[string]any{
		"print": newPrintBuiltin(),
	}))
	// Auto-inject stdin as a variable when data is piped and stdin isn't
	// being used to read code (via --stdin flag).
	if injectStdin && !ctx.Bool("stdin") && cli.IsPiped() {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		if len(data) > 0 {
			opts = append(opts, risor.WithEnv(map[string]any{
				"stdin": string(data),
			}))
		}
	}
	// --var and --var-json flags come last so they can override auto-detected stdin
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

// parseJSONVarFlag parses a --var-json flag value as a JSON object.
func parseJSONVarFlag(value string) (map[string]any, error) {
	if value == "" {
		return nil, nil
	}
	var vars map[string]any
	if err := json.Unmarshal([]byte(value), &vars); err != nil {
		return nil, fmt.Errorf("--var-json: invalid JSON object: %w", err)
	}
	return vars, nil
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
	formatter := errors.NewFormatter(useColor)

	// Check for multi-error types (parser errors with multiple errors)
	if multiErr, ok := err.(interface {
		ToFormattedMultiple() []*errors.FormattedError
	}); ok {
		formatted := multiErr.ToFormattedMultiple()
		return goerrors.New(formatter.FormatMultiple(formatted))
	}

	// Check for single formattable errors (StructuredError, CompileError, etc.)
	if formattable, ok := err.(errors.FormattableError); ok {
		formatted := formattable.ToFormatted()
		return goerrors.New(formatter.Format(formatted))
	}

	return err
}

func newPrintBuiltin() *object.Builtin {
	return object.NewBuiltin("print", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		values := make([]any, len(args))
		for i, arg := range args {
			values[i] = object.PrintableValue(arg)
		}
		fmt.Println(values...)
		return object.Nil, nil
	})
}
