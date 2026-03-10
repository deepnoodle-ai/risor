package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
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
	assert.Equal(t, output, "{\n  \"hello\": \"foo\"\n}\n")
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

func TestEvalHandler_WithVarJSONFlag(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.GlobalFlags(
		cli.Strings("var", ""),
		cli.Strings("var-json", ""),
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

	err := app.ExecuteArgs([]string{"eval", "--var-json", `data={"name":"Alice"}`, "-c", "data.name"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.True(t, contains(output, "Alice"))
}

func TestParseJSONVarFlags(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		expectErr bool
	}{
		{name: "empty", input: nil},
		{name: "object", input: []string{`data={"a":1}`}},
		{name: "array", input: []string{`arr=[1,2,3]`}},
		{name: "number", input: []string{`n=42`}},
		{name: "bad json", input: []string{`x=not json`}, expectErr: true},
		{name: "malformed flag", input: []string{`noequals`}, expectErr: true},
		{name: "empty key", input: []string{`={"a":1}`}, expectErr: true},
		{name: "empty value", input: []string{`x=`}, expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseJSONVarFlags(tt.input)
			if tt.expectErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			if tt.input == nil {
				assert.True(t, result == nil)
			}
		})
	}
}

func TestParseJSONVarFlags_Values(t *testing.T) {
	result, err := parseJSONVarFlags([]string{
		`obj={"name":"Alice"}`,
		`arr=[1,2,3]`,
		`num=42`,
		`flag=true`,
		`nothing=null`,
	})
	assert.Nil(t, err)
	assert.Equal(t, len(result), 5)

	obj, ok := result["obj"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, obj["name"], "Alice")

	arr, ok := result["arr"].([]any)
	assert.True(t, ok)
	assert.Equal(t, len(arr), 3)

	assert.Equal(t, result["num"], float64(42))
	assert.Equal(t, result["flag"], true)
	assert.True(t, result["nothing"] == nil)
}

func TestParseVarFlags(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		expected  map[string]any
		expectErr bool
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
			name:      "malformed flag",
			input:     []string{"noequals"},
			expectErr: true,
		},
		{
			name:      "empty key",
			input:     []string{"=value"},
			expectErr: true,
		},
		{
			name:     "empty value",
			input:    []string{"key="},
			expected: map[string]any{"key": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseVarFlags(tt.input)
			if tt.expectErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
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

func TestEvalHandler_StdinVariable(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	// Create a pipe to simulate piped stdin
	stdinR, stdinW, _ := os.Pipe()
	stdinW.WriteString(`{"name": "Alice"}`)
	stdinW.Close()

	oldStdin := os.Stdin
	os.Stdin = stdinR
	defer func() { os.Stdin = oldStdin }()

	app := cli.New("risor").SetColorEnabled(false)
	app.GlobalFlags(
		cli.Strings("var", ""),
		cli.Bool("stdin", ""),
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

	err := app.ExecuteArgs([]string{"eval", "-c", "stdin"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// The stdin value is a string, so the output is JSON-encoded with escapes
	expected, _ := json.Marshal(`{"name": "Alice"}`)
	assert.Equal(t, output, string(expected)+"\n")
}

func TestEvalHandler_Print(t *testing.T) {
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

	err := app.ExecuteArgs([]string{"eval", "-c", `print("hello", 42)`})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// print should output without quotes on strings
	assert.True(t, contains(output, "hello 42"))
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

func TestNewPrintBuiltin(t *testing.T) {
	fn := newPrintBuiltin()
	assert.Equal(t, fn.Name(), "print")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result, err := fn.Call(context.Background(),
		object.NewString("hello"),
		object.NewInt(42),
		object.NewFloat(3.14),
		object.True,
		object.Nil,
	)

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.True(t, contains(output, "hello"))
	assert.True(t, contains(output, "42"))
	assert.True(t, contains(output, "3.14"))
	assert.True(t, contains(output, "true"))
	assert.True(t, contains(output, "null"))
}

func TestNewPrintBuiltin_NoArgs(t *testing.T) {
	fn := newPrintBuiltin()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result, err := fn.Call(context.Background())

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	// Should just be a newline
	assert.Equal(t, buf.String(), "\n")
}
