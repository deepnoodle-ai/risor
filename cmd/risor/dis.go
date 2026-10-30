package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/dis"
	"github.com/deepnoodle-ai/wonton/cli"
)

func disHandler(ctx *cli.Context) error {
	opts, err := getRisorOptions(ctx, false)
	if err != nil {
		return err
	}

	// Get code from -c flag, --stdin, or file argument
	code, err := getDisCode(ctx)
	if err != nil {
		return err
	}

	// Compile the input code
	compiledCode, err := risor.Compile(context.Background(), code, opts...)
	if err != nil {
		return err
	}
	targetCode := compiledCode

	// If a function name was provided, disassemble its code only
	if funcName := ctx.String("func"); funcName != "" {
		fn := findFunction(compiledCode, funcName)
		if fn == nil {
			return fmt.Errorf("function %q not found", funcName)
		}
		targetCode = fn.Code()
	}

	// Disassemble and print the instructions
	instructions, err := dis.Disassemble(targetCode)
	if err != nil {
		return err
	}
	dis.Print(instructions, os.Stdout)
	return nil
}

// findFunction returns the first function named name in code or in any code
// nested within it, so functions defined inside other functions are found.
func findFunction(code *bytecode.Code, name string) *bytecode.Function {
	for _, c := range code.Flatten() {
		for i := 0; i < c.ConstantCount(); i++ {
			if fn, ok := c.ConstantAt(i).(*bytecode.Function); ok && fn.Name() == name {
				return fn
			}
		}
	}
	return nil
}

func getDisCode(ctx *cli.Context) (string, error) {
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
