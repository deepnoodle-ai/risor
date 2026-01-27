package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/risor-io/risor"
)

func main() {
	ctx := context.Background()

	// Script that uses input variable
	script := `
		let result = input * input
		sprintf("Square of %d is %d", input, result)
	`

	// Compile once with a template environment
	// The env keys must match at runtime, but values can differ
	templateEnv := risor.Builtins()
	templateEnv["input"] = 0 // placeholder value

	code, err := risor.Compile(ctx, script, risor.WithEnv(templateEnv))
	if err != nil {
		log.Fatal("compile error:", err)
	}

	// Run the same compiled code in parallel with different inputs
	inputs := []int{2, 3, 4, 5, 6, 7, 8, 9, 10}
	results := make([]string, len(inputs))

	var wg sync.WaitGroup
	for i, input := range inputs {
		wg.Add(1)
		go func(idx int, val int) {
			defer wg.Done()

			// Each goroutine gets its own environment
			// Keys must match what was used at compile time
			env := risor.Builtins()
			env["input"] = val

			// Run creates fresh runtime state for concurrent execution
			result, err := risor.Run(ctx, code, risor.WithEnv(env))
			if err != nil {
				results[idx] = fmt.Sprintf("error: %v", err)
				return
			}

			results[idx] = fmt.Sprint(result)
		}(i, input)
	}

	wg.Wait()

	fmt.Println("Concurrent execution results:")
	for i, result := range results {
		fmt.Printf("  Input %d: %s\n", inputs[i], result)
	}
}
