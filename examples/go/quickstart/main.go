package main

import (
	"context"
	"fmt"
	"log"

	"github.com/risor-io/risor"
)

func main() {
	ctx := context.Background()
	script := "math.sqrt(input)"

	// Start with the standard library and add custom variables
	env := risor.Builtins()
	env["input"] = 4

	result, err := risor.Eval(ctx, script, risor.WithEnv(env))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The square root of 4 is:", result)
}
