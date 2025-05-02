package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/risor-io/risor"
	ros "github.com/risor-io/risor/os"
)

func main() {
	var script string
	flag.StringVar(&script, "script", "", "risor script to run")
	flag.Parse()

	if script == "" {
		script = "os.stdin.read() | strings.to_upper | print"
	}

	ctx := context.Background()

	stdin := ros.NewBufferFile([]byte("hello"))
	stdout := ros.NewBufferFile(nil)

	virtualOS := ros.NewVirtualOS(ctx,
		ros.WithStdin(stdin),
		ros.WithStdout(stdout))

	result, err := risor.Eval(ctx, script, risor.WithOS(virtualOS))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("script eval result:", result)
	fmt.Println("stdout buffer:", string(stdout.Bytes()))
}
