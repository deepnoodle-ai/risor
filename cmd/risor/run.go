package main

import (
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
	opts := getRisorOptions(ctx)

	// Check if we should run REPL
	if shouldRunRepl(ctx) {
		return runRepl(ctx.Context(), getReplEnv(ctx))
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

func getRisorOptions(ctx *cli.Context) []risor.Option {
	var opts []risor.Option
	if !ctx.Bool("no-default-globals") {
		opts = append(opts, risor.WithEnv(risor.Builtins()))
	}
	return opts
}

func getReplEnv(ctx *cli.Context) map[string]any {
	if ctx.Bool("no-default-globals") {
		return nil
	}
	return risor.Builtins()
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
