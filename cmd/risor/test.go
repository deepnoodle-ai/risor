package main

import (
	"os"

	"github.com/deepnoodle-ai/risor/v2/pkg/testing"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func testHandler(ctx *cli.Context) error {
	// Collect patterns from arguments
	patterns := ctx.Args()

	// Build test config
	cfg := &testing.Config{
		Patterns:   patterns,
		RunPattern: ctx.String("run"),
		Verbose:    ctx.Bool("verbose"),
	}

	// Run tests
	summary, err := testing.Run(ctx.Context(), cfg)
	if err != nil {
		return err
	}

	// Configure output
	useColor := !ctx.Bool("no-color") && color.ShouldColorize(os.Stdout)
	output := testing.NewOutput(testing.OutputConfig{
		Writer:   os.Stdout,
		Verbose:  cfg.Verbose,
		UseColor: useColor,
	})

	// Print results
	output.PrintResults(summary)

	// Exit with non-zero status on failure
	if !summary.Success() {
		os.Exit(1)
	}

	return nil
}
