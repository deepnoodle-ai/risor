package main

import (
	"context"
	"fmt"
	"log"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

func main() {
	ctx := context.Background()
	env := risor.Builtins()

	// Example 1: Script that returns normally
	fmt.Println("=== Example 1: Successful execution ===")
	result, err := risor.Eval(ctx, `[1, 2, 3].map(x => x * 2)`, risor.WithEnv(env))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %v (type: %T)\n\n", result, result)

	// Example 2: Script with a syntax error
	fmt.Println("=== Example 2: Syntax error ===")
	_, err = risor.Eval(ctx, `let x = `, risor.WithEnv(env))
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
	}

	// Example 3: Script with a runtime error
	fmt.Println("=== Example 3: Runtime error ===")
	_, err = risor.Eval(ctx, `
		function divide(a, b) {
			if (b == 0) {
				throw error("division by zero: %d / %d", a, b)
			}
			return a / b
		}
		divide(10, 0)
	`, risor.WithEnv(env))
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
	}

	// Example 4: Script handles its own error
	fmt.Println("=== Example 4: Script handles error internally ===")
	result, err = risor.Eval(ctx, `
		let result = try {
			let x = int("not a number")
			x * 2
		} catch e {
			{
				error: true,
				message: e.message(),
				fallback: -1
			}
		}
		result
	`, risor.WithEnv(env))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %v\n\n", result)

	// Example 5: Type assertion on results
	fmt.Println("=== Example 5: Working with result types ===")
	result, err = risor.Eval(ctx, `{name: "Alice", score: 95}`, risor.WithEnv(env))
	if err != nil {
		log.Fatal(err)
	}

	// Result could be a Risor object or a Go type
	switch v := result.(type) {
	case *object.Map:
		fmt.Println("Got a Risor Map:")
		for _, key := range v.SortedKeys() {
			val := v.Get(key)
			fmt.Printf("  %s = %v\n", key, val.Inspect())
		}
	case map[string]any:
		fmt.Println("Got a Go map:")
		for k, val := range v {
			fmt.Printf("  %s = %v\n", k, val)
		}
	default:
		fmt.Printf("Got unexpected type: %T\n", v)
	}
	fmt.Println()

	// Example 6: Filename for better error messages
	fmt.Println("=== Example 6: Error with filename ===")
	_, err = risor.Eval(ctx, `
		let data = loadData()  // undefined function
	`, risor.WithEnv(env), risor.WithFilename("config.risor"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
