// Command outcome runs every case of one case file and writes one line of
// JSON for each, for the differential fuzzer (spec rulings, round 10 AJ).
// It ignores every expect. Run it from go/ with
// "GOWORK=off go run ./cmd/outcome FILE".
package main

import (
	"fmt"
	"os"

	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/runner"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: outcome FILE")
		os.Exit(2)
	}
	f, err := casefile.LoadNoExpect(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, c := range f.Cases {
		fmt.Println(runner.Outcome(c))
	}
}
