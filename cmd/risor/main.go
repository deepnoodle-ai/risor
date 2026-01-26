package main

import (
	"os"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	app := cli.New("risor").
		Description("Fast and flexible scripting for Go developers").
		Version(version)

	// Global flags
	app.GlobalFlags(
		cli.String("code", "c").Help("Code to evaluate"),
		cli.Bool("stdin", "").Help("Read code from stdin"),
		cli.String("cpu-profile", "").Help("Capture CPU profile"),
		cli.Bool("no-color", "").Env("NO_COLOR").Help("Disable colored output"),
		cli.Bool("no-default-globals", "").Help("Disable the standard library"),
	)

	// Root command: runs code or starts REPL
	app.Main().
		Args("file?").
		Flags(
			cli.Bool("timing", "").Help("Show execution time"),
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
			cli.Bool("no-repl", "").Help("Disable the REPL"),
		).
		Run(runHandler)

	// Version command with JSON support
	app.Command("version").
		Description("Print version information").
		Flags(
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
		).
		Run(versionHandler)

	// Disassemble command
	app.Command("dis").
		Description("Disassemble Risor bytecode").
		Args("file?").
		Flags(
			cli.String("code", "c").Help("Code to disassemble"),
			cli.Bool("stdin", "").Help("Read code from stdin"),
			cli.String("func", "").Help("Function to disassemble"),
		).
		Run(disHandler)

	if err := app.Execute(); err != nil {
		if _, ok := err.(*cli.HelpRequested); ok {
			return
		}
		printError(err.Error())
		os.Exit(1)
	}
}

func printError(msg string) {
	if color.ShouldColorize(os.Stderr) {
		msg = color.Red.Apply(msg)
	}
	os.Stderr.WriteString(msg + "\n")
}
