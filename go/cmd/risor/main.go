// Command risor is the command-line program of Risor v3 (package F12 4 of
// the specification): eval and check load a config file (package F10 9),
// fmt formats programs and templates, test runs case files (package F9 8),
// and with no command it starts the REPL (package F12 5).
//
//	risor eval FILE [--type TEXT | --schema FILE] [--format text|json] [--pretty]
//	               [ENV ...] [--max-failures N] [--color WHEN]
//	risor check FILE [--type TEXT | --schema FILE] [--format text|json]
//	               [--print-type] [ENV ...] [--color WHEN]
//	risor fmt [PATH ...] [--write | --check] [--template] [--color WHEN]
//	risor test [PATH ...] [--run TEXT ...] [--update] [--format text|json]
//	               [--color WHEN]
//	risor test --suite DIR [--format text|json] [--color WHEN]
//	risor [ENV ...]
//	risor help | risor version
//
// ENV stands for --var NAME=TEXT, --vars FILE.json, --env NAME,
// --description FILE, --group NAME, --steps N, --allocation N and
// --depth N.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(runStdin(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = `usage: risor eval FILE [--type TEXT | --schema FILE] [--format text|json] [--pretty]
                 [ENV ...] [--max-failures N] [--color WHEN]
       risor check FILE [--type TEXT | --schema FILE] [--format text|json]
                 [--print-type] [ENV ...] [--color WHEN]
       risor fmt [PATH ...] [--write | --check] [--template] [--color WHEN]
       risor test [PATH ...] [--run TEXT ...] [--update] [--format text|json] [--color WHEN]
       risor test --suite DIR [--format text|json] [--color WHEN]
       risor [ENV ...]
       risor help | risor version
ENV: --var NAME=TEXT, --vars FILE.json, --env NAME, --description FILE,
     --group NAME, --steps N, --allocation N, --depth N
`

// version is the version of the language this program implements.
const version = "risor 3.0"

// run runs the command line with no standard input, and returns the exit
// status.
func run(args []string, stdout, stderr io.Writer) int {
	return runStdin(args, strings.NewReader(""), stdout, stderr)
}

// runStdin runs the command line and returns the exit status.
func runStdin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") && args[0] != "-h" && args[0] != "--help" && args[0] != "--version" {
		return replCommand(args, stdin, stdout, stderr)
	}
	switch args[0] {
	case "test":
		return testCommand(args[1:], stdin, stdout, stderr)
	case "eval":
		return evalCommand(args[1:], stdin, stdout, stderr)
	case "check":
		return checkCommand(args[1:], stdin, stdout, stderr)
	case "fmt":
		return fmtCommand(args[1:], stdin, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "--version":
		fmt.Fprintln(stdout, version)
		return 0
	}
	fmt.Fprintf(stderr, "risor: unknown command %q\n%s", args[0], usage)
	return 2
}
