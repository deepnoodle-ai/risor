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
		Version(version).
		AddCompletionCommand()

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

	// Test command
	app.Command("test").
		Description("Run tests").
		Args("patterns...").
		Flags(
			cli.Bool("verbose", "v").Help("Verbose output"),
			cli.String("run", "r").Help("Run only tests matching pattern"),
		).
		Run(testHandler)

	// Documentation command
	app.Command("doc").
		Alias("d").
		Description("Browse language documentation").
		Args("topic?").
		Flags(
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
		).
		Run(docHandler)

	// AST command
	app.Command("ast").
		Description("Display the AST for Risor code").
		Args("file?").
		Flags(
			cli.String("code", "c").Help("Code to parse"),
			cli.Bool("stdin", "").Help("Read code from stdin"),
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
		).
		Run(astHandler)

	// Format command
	app.Command("fmt").
		Alias("f").
		Description("Format Risor source code").
		Args("file?").
		Flags(
			cli.String("code", "c").Help("Code to format"),
			cli.Bool("stdin", "").Help("Read code from stdin"),
			cli.Bool("write", "w").Help("Write result to source file"),
		).
		Run(fmtHandler)

	// Examples command
	app.Command("examples").
		Alias("ex").
		Description("Browse code examples").
		Args("name?").
		Flags(
			cli.Bool("run", "r").Help("Run the example"),
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
		).
		Run(examplesHandler)

	// Eval command
	app.Command("eval").
		Alias("e").
		Description("Evaluate an expression").
		Args("expr?").
		Flags(
			cli.String("code", "c").Help("Expression to evaluate"),
			cli.Bool("stdin", "").Help("Read from stdin"),
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
			cli.Bool("quiet", "q").Help("Suppress output"),
		).
		Run(evalHandler)

	// Lint command
	app.Command("lint").
		Description("Check code for issues").
		Args("file?").
		Flags(
			cli.String("code", "c").Help("Code to check"),
			cli.Bool("stdin", "").Help("Read code from stdin"),
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
		).
		Run(lintHandler)

	// Benchmark command
	app.Command("bench").
		Description("Benchmark code execution").
		Args("file?").
		Flags(
			cli.String("code", "c").Help("Code to benchmark"),
			cli.Bool("stdin", "").Help("Read code from stdin"),
			cli.Int("iterations", "n").Help("Number of iterations").Default(1000),
			cli.Int("warmup", "w").Help("Warmup iterations").Default(100),
			cli.String("output", "o").Enum("json", "text").Help("Output format"),
		).
		Run(benchHandler)

	if err := app.Execute(); err != nil {
		if cli.IsHelpRequested(err) {
			return
		}
		printError(err.Error())
		os.Exit(cli.GetExitCode(err))
	}
}

func printError(msg string) {
	if color.ShouldColorize(os.Stderr) {
		msg = color.Red.Apply(msg)
	}
	os.Stderr.WriteString(msg + "\n")
}
