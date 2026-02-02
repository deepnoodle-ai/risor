package main

import (
	"bytes"
	"errors"
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

	// Should be JSON formatted
	assert.True(t, contains(output, "value"))
	assert.True(t, contains(output, "42"))
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

func TestToGoValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "int",
			input:    object.NewInt(42),
			expected: int64(42),
		},
		{
			name:     "float",
			input:    object.NewFloat(3.14),
			expected: 3.14,
		},
		{
			name:     "string",
			input:    object.NewString("hello"),
			expected: "hello",
		},
		{
			name:     "bool true",
			input:    object.True,
			expected: true,
		},
		{
			name:     "bool false",
			input:    object.False,
			expected: false,
		},
		{
			name:     "nil object",
			input:    object.Nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toGoValue(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestToGoValue_List(t *testing.T) {
	list := object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	})

	result := toGoValue(list)
	arr, ok := result.([]any)
	assert.True(t, ok)
	assert.Equal(t, len(arr), 3)
	assert.Equal(t, arr[0], int64(1))
	assert.Equal(t, arr[1], int64(2))
	assert.Equal(t, arr[2], int64(3))
}

func TestToGoValue_Map(t *testing.T) {
	m := object.NewMap(map[string]object.Object{
		"a": object.NewInt(1),
		"b": object.NewString("hello"),
	})

	result := toGoValue(m)
	mp, ok := result.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, mp["a"], int64(1))
	assert.Equal(t, mp["b"], "hello")
}

func TestToGoValue_Error(t *testing.T) {
	err := object.NewError(errors.New("test error"))
	result := toGoValue(err)

	mp, ok := result.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, mp["error"], true)
	assert.Equal(t, mp["message"], "test error")
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
