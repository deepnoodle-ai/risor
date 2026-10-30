package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func newRunApp() *cli.App {
	app := cli.New("risor").SetColorEnabled(false)
	app.GlobalFlags(
		cli.String("code", "c"),
		cli.Bool("stdin", ""),
		cli.Strings("var", ""),
		cli.String("var-json", ""),
		cli.String("cpu-profile", ""),
		cli.Bool("no-color", ""),
		cli.Bool("no-default-globals", ""),
	)
	app.Main().
		Args("file?").
		Flags(
			cli.Bool("timing", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("no-repl", ""),
		).
		Run(runHandler)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)
	return app
}

// runWithStdin runs the CLI with stdin set to a pipe holding input. When
// closeStdin is false the pipe stays open, as with an inherited pipe whose
// writer never exits. It fails the test if the run does not finish.
func runWithStdin(t *testing.T, input string, closeStdin bool, args ...string) string {
	t.Helper()
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	stdinR, stdinW, err := os.Pipe()
	assert.Nil(t, err)
	stdinW.WriteString(input)
	if closeStdin {
		stdinW.Close()
	} else {
		defer stdinW.Close()
	}
	oldStdin := os.Stdin
	os.Stdin = stdinR
	defer func() { os.Stdin = oldStdin }()

	oldStdout := os.Stdout
	outR, outW, err := os.Pipe()
	assert.Nil(t, err)
	os.Stdout = outW
	defer func() { os.Stdout = oldStdout }()

	done := make(chan error, 1)
	go func() { done <- newRunApp().ExecuteArgs(args) }()
	select {
	case err := <-done:
		assert.Nil(t, err)
	case <-time.After(5 * time.Second):
		t.Fatalf("risor %v blocked on an open stdin pipe", args)
	}

	outW.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(outR)
	return buf.String()
}

func TestRun_OpenStdinPipeNotRead(t *testing.T) {
	out := runWithStdin(t, "ignored", false, "-c", `print("hi")`)
	assert.Equal(t, out, "hi\n")
}

func TestRun_FileWithOpenStdinPipeNotRead(t *testing.T) {
	out := runWithStdin(t, "ignored", false, "fixtures/ex1.risor")
	assert.Equal(t, out, "7\n")
}

func TestEval_OpenStdinPipeNotRead(t *testing.T) {
	out := runWithStdin(t, "ignored", false, "eval", "1 + 2")
	assert.Equal(t, out, "3\n")
}

func TestRun_PipedStdinInjected(t *testing.T) {
	out := runWithStdin(t, "hello", true, "-c", `print(stdin + "!")`)
	assert.Equal(t, out, "hello!\n")
}

func TestRun_VarOverridesPipedStdin(t *testing.T) {
	out := runWithStdin(t, "hello", true, "--var", "stdin=flag", "-c", "print(stdin)")
	assert.Equal(t, out, "flag\n")
}
