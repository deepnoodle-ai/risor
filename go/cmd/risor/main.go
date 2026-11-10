// Command risor is the command line tool of Risor v3. test runs case files
// (package F9 8 of the specification); eval and check load a config file
// (package F10 9):
//
//	risor eval FILE [--type TEXT | --schema FILE] [--format text|json]
//	risor check FILE [--type TEXT | --schema FILE]
//	risor test [PATH ...] [--run TEXT ...] [--update] [--format text|json]
//	risor test --suite DIR [--format text|json]
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `usage: risor eval FILE [--type TEXT | --schema FILE] [--format text|json]
       risor check FILE [--type TEXT | --schema FILE]
       risor test [PATH ...] [--run TEXT ...] [--update] [--format text|json]
       risor test --suite DIR [--format text|json]
`

// run runs the command line and returns the exit status.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "test":
		return testCommand(args[1:], stdout, stderr)
	case "eval":
		return evalCommand(args[1:], stdout, stderr)
	case "check":
		return checkCommand(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "risor: unknown command %q\n%s", args[0], usage)
	return 2
}
