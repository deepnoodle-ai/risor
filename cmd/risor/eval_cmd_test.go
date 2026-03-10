package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestEvalHandler_SimpleExpression(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "1 + 2"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.True(t, contains(output, "3"))
}

func TestEvalHandler_WithCodeFlag(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "-c", "10 * 5"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.True(t, contains(output, "50"))
}

func TestEvalHandler_JsonOutput(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "-o", "json", "42"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should be the JSON value directly, not wrapped in {type, value}
	assert.True(t, contains(output, "42"))
	assert.True(t, !contains(output, "value"))
	assert.True(t, !contains(output, "type"))
}

func TestEvalHandler_Quiet(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "-q", "1 + 2"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should be empty when quiet
	assert.Equal(t, output, "")
}

func TestEvalHandler_Error(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "undefined_var"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	// Should have an error
	assert.NotNil(t, err)
}

func TestEvalHandler_NoInput(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	assert.NotNil(t, err)
	assert.True(t, contains(err.Error(), "no expression"))
}

func TestEvalHandler_JsonOutput_String(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "-o", "json", `"foo"`})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should output "foo" directly, not {"type": "string", "value": "foo"}
	assert.Equal(t, output, "\"foo\"\n")
}

func TestEvalHandler_JsonOutput_Map(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "-o", "json", "-c", `{hello: "foo"}`})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should output {"hello": "foo"} directly
	assert.True(t, contains(output, `"hello"`))
	assert.True(t, contains(output, `"foo"`))
	assert.True(t, !contains(output, "type"))
}

func TestEvalHandler_WithVarFlag(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.GlobalFlags(
		cli.Strings("var", ""),
	)
	app.Command("eval").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
			cli.String("output", "o").Enum("json", "text"),
			cli.Bool("quiet", "q"),
		).
		Run(evalHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"eval", "--var", "name=Alice", "-c", "name"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.True(t, contains(output, "Alice"))
}

func TestParseVarFlags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]any
	}{
		{
			name:     "empty",
			input:    nil,
			expected: nil,
		},
		{
			name:     "single var",
			input:    []string{"key=value"},
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "multiple vars",
			input:    []string{"a=hello", "b=world"},
			expected: map[string]any{"a": "hello", "b": "world"},
		},
		{
			name:     "value with equals",
			input:    []string{"url=http://example.com?a=1"},
			expected: map[string]any{"url": "http://example.com?a=1"},
		},
		{
			name:     "skip invalid",
			input:    []string{"noequals", "valid=yes"},
			expected: map[string]any{"valid": "yes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVarFlags(tt.input)
			if tt.expected == nil {
				assert.True(t, result == nil)
				return
			}
			assert.Equal(t, len(result), len(tt.expected))
			for k, v := range tt.expected {
				assert.Equal(t, result[k], v)
			}
		})
	}
}

func TestGetEvalExpr_MultipleInputs(t *testing.T) {
	app := cli.New("test").SetColorEnabled(false)
	var capturedErr error
	app.Command("test").
		Args("expr?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
		).
		Run(func(ctx *cli.Context) error {
			_, capturedErr = getEvalExpr(ctx)
			return capturedErr
		})

	_ = app.ExecuteArgs([]string{"test", "-c", "1+2", "another_expr"})
	assert.NotNil(t, capturedErr)
	assert.True(t, contains(capturedErr.Error(), "multiple"))
}
