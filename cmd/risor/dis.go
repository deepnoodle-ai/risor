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
	opts := getRisorOptions(ctx)

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
		var fn *bytecode.Function
		for i := 0; i < compiledCode.ConstantCount(); i++ {
			obj, ok := compiledCode.ConstantAt(i).(*bytecode.Function)
			if !ok {
				continue
			}
			if obj.Name() == funcName {
				fn = obj
				break
			}
		}
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
