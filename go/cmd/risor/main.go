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
//
// The commands, flags, help and shell completions are built with
// wonton/cli. The rules the chapter fixes and wonton/cli does not keep are
// kept here, before the arguments reach it (prepareArgs).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deepnoodle-ai/wonton/cli"
)

func main() {
	os.Exit(runStdin(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// synopsis is the chapter's synopsis (F12 4.1), shown by help.
const synopsis = `Synopsis:
  risor eval FILE [--type TEXT | --schema FILE] [--format text|json] [--pretty]
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

With no command, risor starts the REPL.`

// envHelp says what ENV stands for, in the help of the commands that take it.
const envHelp = "ENV: --var NAME=TEXT, --vars FILE.json, --env NAME, --description FILE,\n" +
	"     --group NAME, --steps N, --allocation N, --depth N"

// version is the version of the language this program implements.
const version = "risor 3.0"

// The commands of the chapter (F12 4.1), and those wonton/cli adds.
var commands = map[string]bool{
	"eval": true, "check": true, "fmt": true, "test": true,
	"help": true, "version": true, "completion": true,
}

// valueFlags are the flags that take a value; boolFlags take none.
var (
	valueFlags = map[string]bool{
		"var": true, "vars": true, "env": true, "description": true, "group": true,
		"steps": true, "allocation": true, "depth": true,
		"type": true, "schema": true, "format": true, "max-failures": true,
		"color": true, "run": true, "suite": true,
	}
	boolFlags = map[string]bool{
		"pretty": true, "print-type": true, "write": true, "check": true,
		"template": true, "update": true,
	}
	// colorWords are flags wonton/cli takes for itself, from any command,
	// unless the program defines them. This program does not: each is an
	// unknown flag (F12 4.9).
	colorWords = map[string]bool{
		"--no-color": true, "--no-colour": true, "--colour": true, "--force-color": true,
	}
)

// run runs the command line with no standard input, and returns the exit
// status.
func run(args []string, stdout, stderr io.Writer) int {
	return runStdin(args, strings.NewReader(""), stdout, stderr)
}

// runStdin runs the command line and returns the exit status.
func runStdin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--version" {
		fmt.Fprintln(stdout, version)
		return 0
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && !commands[args[0]] {
		// wonton/cli gives a word it does not know to the root command, the
		// REPL, as an argument; the chapter calls it an unknown command.
		return commandLineError(stderr, cmdErrorf("unknown command %q\n\nRun 'risor help' for usage.", args[0]))
	}
	args, err := prepareArgs(args)
	if err != nil {
		return commandLineError(stderr, err)
	}
	status := 0
	app := newApp(stdin, stdout, stderr, &status)
	if err := app.ExecuteArgs(args); err != nil {
		if cli.IsHelpRequested(err) {
			return 0
		}
		var exit *cli.ExitError
		if errors.As(err, &exit) {
			return exit.Code
		}
		// Every other error of wonton/cli is one of the command line: an
		// unknown flag, a flag with no value or a bad one, a missing or
		// extra argument (F12 4.9). Its status is 2, not wonton's 1.
		return commandLineError(stderr, err)
	}
	return status
}

// prepareArgs keeps three rules of the chapter that wonton/cli does not:
//
//  1. A flag that takes a value takes the next argument, whatever it is:
//     `--run -x` and `--var a=-1` are values, and so is a word that names
//     a command (`risor --env test`). wonton/cli refuses a value that
//     looks like a flag, and takes a command name for the command. So
//     `--name value` is joined into `--name=value`.
//  2. A flag that takes no value is written alone: `--write=true` is a
//     value of the wrong form, which wonton/cli would accept.
//  3. --no-color, --no-colour, --colour and --force-color are unknown
//     flags, which wonton/cli would take for itself.
//
// Nothing after `--` is changed.
func prepareArgs(args []string) ([]string, error) {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return append(out, args[i:]...), nil
		}
		if colorWords[a] {
			return nil, cmdErrorf("unknown flag: %s", a)
		}
		name, hasPrefix := strings.CutPrefix(a, "--")
		if !hasPrefix {
			out = append(out, a)
			continue
		}
		name, _, hasValue := strings.Cut(name, "=")
		switch {
		case valueFlags[name] && !hasValue && i+1 < len(args):
			out = append(out, a+"="+args[i+1])
			i++
		case boolFlags[name] && hasValue:
			return nil, cmdErrorf("--%s takes no value", name)
		default:
			out = append(out, a)
		}
	}
	return out, nil
}

// formatFlag is --format text|json.
func formatFlag() cli.Flag {
	return cli.String("format").Default("text").Enum("text", "json").Help("The form of the output")
}

// colorFlagDef is --color WHEN (F12 4.2 item 4).
func colorFlagDef() cli.Flag {
	return cli.String("color").Default("auto").Enum("auto", "always", "never").Help("Colour the output")
}

// newApp builds the commands. Each handler sets *status, the exit status.
func newApp(stdin io.Reader, stdout, stderr io.Writer, status *int) *cli.App {
	app := cli.New("risor").
		Description("Risor, a small expression language to embed: evaluate, check, format and test programs").
		Long(synopsis)
	app.SetStdin(stdin).SetStdout(stdout).SetStderr(stderr)
	// The colour of help follows the rule of F12 4.2 item 4 for auto.
	help := colorFlag{"auto"}
	app.SetColorEnabled(help.on(stdout, stdout))
	handle := func(cmd func(*cli.Context) int) cli.Handler {
		return func(ctx *cli.Context) error {
			*status = cmd(ctx)
			return nil
		}
	}
	app.Main().
		Description("Start the REPL").
		Long("risor [ENV ...] [--color WHEN]\n\n" + envHelp).
		Flags(replFlagDefs()...).
		Run(handle(replCommand))
	app.Command("eval").
		Description("Evaluate a program and write its value").
		Long("risor eval FILE [--type TEXT | --schema FILE] [--format text|json] [--pretty]\n" +
			"                [ENV ...] [--max-failures N] [--color WHEN]\n\n" + envHelp).
		AddArg(&cli.Arg{Name: "file", Description: "The program; - is standard input", Required: true}).
		Flags(configFlagDefs(true)...).
		Run(handle(func(ctx *cli.Context) int { return configCommand(ctx, true) }))
	app.Command("check").
		Description("Check a program without running it").
		Long("risor check FILE [--type TEXT | --schema FILE] [--format text|json]\n" +
			"                [--print-type] [ENV ...] [--color WHEN]\n\n" + envHelp).
		AddArg(&cli.Arg{Name: "file", Description: "The program; - is standard input", Required: true}).
		Flags(configFlagDefs(false)...).
		Run(handle(func(ctx *cli.Context) int { return configCommand(ctx, false) }))
	app.Command("fmt").
		Description("Format programs and templates").
		Long("risor fmt [PATH ...] [--write | --check] [--template] [--color WHEN]").
		AddArg(&cli.Arg{Name: "path", Description: "A file, or a directory searched for .risor files; none or - is standard input", Variadic: true}).
		Flags(fmtFlagDefs()...).
		Run(handle(fmtCommand))
	app.Command("test").
		Description("Run case files, or the conformance suite").
		Long("risor test [PATH ...] [--run TEXT ...] [--update] [--format text|json] [--color WHEN]\n" +
			"risor test --suite DIR [--format text|json] [--color WHEN]").
		AddArg(&cli.Arg{Name: "path", Description: "A case file, or a directory searched for case files; none is the current directory", Variadic: true}).
		Flags(testFlagDefs()...).
		Run(handle(testCommand))
	app.Command("version").
		Description("Write the version").
		Run(handle(func(ctx *cli.Context) int {
			fmt.Fprintln(ctx.Stdout(), version)
			return 0
		}))
	app.AddCompletionCommand()
	return app
}
