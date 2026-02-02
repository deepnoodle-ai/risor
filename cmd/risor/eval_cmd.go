package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/object"
)

func evalHandler(ctx *cli.Context) error {
	// Get expression from -c flag, --stdin, or positional argument
	expr, err := getEvalExpr(ctx)
	if err != nil {
		return err
	}

	outputFormat := ctx.String("output")
	quiet := ctx.Bool("quiet")

	// Evaluate with standard library
	result, err := risor.Eval(context.Background(), expr, risor.WithEnv(risor.Builtins()))
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
	if outputFormat == "json" {
		out := map[string]any{
			"value": toGoValue(result),
			"type":  fmt.Sprintf("%T", result),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	// Default text output
	if result != nil {
		fmt.Println(result)
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

// toGoValue converts a Risor object to a Go value for JSON serialization
func toGoValue(obj any) any {
	if obj == nil {
		return nil
	}

	switch v := obj.(type) {
	case *object.Int:
		return v.Value()
	case *object.Float:
		return v.Value()
	case *object.String:
		return v.Value()
	case *object.Bool:
		return v.Value()
	case *object.NilType:
		return nil
	case *object.List:
		items := v.Value()
		result := make([]any, len(items))
		for i, item := range items {
			result[i] = toGoValue(item)
		}
		return result
	case *object.Map:
		result := make(map[string]any)
		for k, val := range v.Value() {
			result[k] = toGoValue(val)
		}
		return result
	case *object.Error:
		return map[string]any{
			"error":   true,
			"message": v.Message().Value(),
		}
	default:
		return fmt.Sprintf("%v", v)
	}
}
