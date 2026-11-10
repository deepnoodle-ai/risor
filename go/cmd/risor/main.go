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
// wonton/cli. The one rule the chapter fixes and wonton/cli does not keep is
// kept here, before the arguments reach it (checkBoolFlags).
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

// boolFlags are the flags that take no value.
var boolFlags = map[string]bool{
	"pretty": true, "print-type": true, "write": true, "check": true,
	"template": true, "update": true,
}

// run runs the command line with no standard input, and returns the exit
// status.
func run(args []string, stdout, stderr io.Writer) int {
	return runStdin(args, strings.NewReader(""), stdout, stderr)
}

// runStdin runs the command line and returns the exit status.
func runStdin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if err := checkBoolFlags(args); err != nil {
		return commandLineError(stderr, err)
	}
	status := 0
	app := newApp(stdin, stdout, stderr, &status)
	if err := app.ExecuteArgs(args); err != nil {
		if cli.IsHelpRequested(err) {
			return 0
		}
		// An error of wonton/cli is a *cli.UsageError, an error of the
		// command line (F12 4.9): status 2.
		code := cli.GetExitCode(err)
		var exit *cli.ExitError
		if !errors.As(err, &exit) {
			fmt.Fprintf(stderr, "risor: %v\n", err)
		}
		return code
	}
	return status
}

// checkBoolFlags keeps a rule of the chapter that wonton/cli does not: a
// flag that takes no value is written alone, so `--write=true` is a value
// of the wrong form (F12 4.9), which wonton/cli would accept. Nothing after
// `--` is looked at, and the word after a flag that takes a value is that
// flag's value, as wonton/cli reads it.
func checkBoolFlags(args []string) error {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return nil
		}
		name, ok := strings.CutPrefix(a, "--")
		if !ok {
			continue
		}
		name, _, hasValue := strings.Cut(name, "=")
		switch {
		case boolFlags[name] && hasValue:
			return cmdErrorf("--%s takes no value", name)
		case !boolFlags[name] && !hasValue && name != "help" && name != "version":
			i++ // the flag's value
		}
	}
	return nil
}

// formatFlag is --format text|json.
func formatFlag() cli.Flag {
	return cli.String("format").Default("text").Enum("text", "json").Help("The form of the output")
}

// colorFlagDef is --color WHEN (F12 4.2 item 4).
func colorFlagDef() cli.Flag {
	return cli.String("color").Placeholder("WHEN").Default("auto").Enum("auto", "always", "never").Help("Colour the output")
}

// newApp builds the commands. Each handler sets *status, the exit status.
func newApp(stdin io.Reader, stdout, stderr io.Writer, status *int) *cli.App {
	app := cli.New("risor").
		Description("Risor, a small expression language to embed: evaluate, check, format and test programs").
		Long(synopsis).
		// --no-color, --no-colour, --colour and --force-color are unknown
		// flags (F12 4.9); --color is each command's own (4.2 item 4).
		ColorFlags(false).
		VersionFunc(func(w io.Writer) { fmt.Fprintln(w, version) })
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
		NoArgs().
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
	// VersionFunc serves --version. The built-in version command of
	// wonton/cli is missing from help and from the completion scripts, so
	// the program defines its own, which takes its place.
	app.Command("version").
		Description("Write the version").
		NoArgs().
		Run(handle(func(ctx *cli.Context) int {
			fmt.Fprintln(ctx.Stdout(), version)
			return 0
		}))
	app.AddCompletionCommand()
	return app
}
