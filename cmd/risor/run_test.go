package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/color"
)

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
	go func() { done <- newApp().SetColorEnabled(false).ExecuteArgs(args) }()
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

func TestRun_ClosurePipedStdinInjected(t *testing.T) {
	out := runWithStdin(t, "hello", true, "-c", `let f = () => stdin; print(f())`)
	assert.Equal(t, out, "hello\n")
}

func TestRun_VarStdinDoesNotReadPipe(t *testing.T) {
	out := runWithStdin(t, "hello", false, "--var", "stdin=flag", "-c", "print(stdin)")
	assert.Equal(t, out, "flag\n")
}

func TestRun_VarJSONStdinDoesNotReadPipe(t *testing.T) {
	out := runWithStdin(t, "hello", false, "--var-json", `{"stdin": "json"}`, "-c", "print(stdin)")
	assert.Equal(t, out, "json\n")
}

func TestRun_StdinNameNotGlobalDoesNotReadPipe(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"map key", `print({stdin: 1}["stdin"])`, "1\n"},
		{"parameter", `function f(stdin) { return stdin }; print(f(2))`, "2\n"},
		{"own variable", `let stdin = 3; print(stdin)`, "3\n"},
		{"destructured", `let {stdin} = {stdin: 4}; print(stdin)`, "4\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runWithStdin(t, "ignored", false, "-c", tt.code)
			assert.Equal(t, out, tt.want)
		})
	}
}

func TestRun_CompileErrorDoesNotReadPipe(t *testing.T) {
	oldStdin := os.Stdin
	stdinR, stdinW, err := os.Pipe()
	assert.Nil(t, err)
	defer stdinW.Close()
	os.Stdin = stdinR
	defer func() { os.Stdin = oldStdin }()

	done := make(chan error, 1)
	go func() {
		done <- newApp().SetColorEnabled(false).ExecuteArgs([]string{"--no-color", "-c", "print(stdin, missing)"})
	}()
	select {
	case err := <-done:
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "undefined variable"))
	case <-time.After(5 * time.Second):
		t.Fatal("risor blocked on an open stdin pipe")
	}
}
