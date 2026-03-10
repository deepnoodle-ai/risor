package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/wonton/cli"
)

func evalHandler(ctx *cli.Context) error {
	// Get expression from -c flag, --stdin, or positional argument
	expr, err := getEvalExpr(ctx)
	if err != nil {
		return err
	}

	outputFormat := ctx.String("output")
	quiet := ctx.Bool("quiet")

	// Build options
	opts := getRisorOptions(ctx)

	// Evaluate
	result, err := risor.Eval(context.Background(), expr, opts...)
	if err != nil {
		if outputFormat == "json" {
			out := map[string]any{
				"error": err.Error(),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}
		return err
	}

	if quiet {
		return nil
	}

	// Output result
	output, err := formatOutput(ctx, result)
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Println(output)
	}
	return nil
}

func getEvalExpr(ctx *cli.Context) (string, error) {
	codeSet := ctx.IsSet("code")
	stdinSet := ctx.Bool("stdin")
	exprProvided := ctx.Arg(0) != ""

	// Check for conflicting input sources
	count := 0
	if codeSet {
		count++
	}
	if stdinSet {
		count++
	}
	if exprProvided {
		count++
	}
	if count > 1 {
		return "", errors.New("multiple input sources specified")
	}
	if count == 0 {
		return "", errors.New("no expression provided")
	}

	if stdinSet {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	if exprProvided {
		return ctx.Arg(0), nil
	}

	return ctx.String("code"), nil
}
