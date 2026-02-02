package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/deepnoodle-ai/risor/v2"
)

func main() {
	var script string
	flag.StringVar(&script, "script", "", "risor script to run")
	flag.Parse()

	if script == "" {
		// Example demonstrating pure computation without I/O
		script = `"hello" | strings.to_upper`
	}

	ctx := context.Background()

	result, err := risor.Eval(ctx, script)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("script eval result:", result)
}
