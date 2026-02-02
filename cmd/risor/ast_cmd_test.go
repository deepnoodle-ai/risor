package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestPrintAST(t *testing.T) {
	// Disable colors and capture output
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() {
		color.Enabled = oldEnabled
	}()

	tests := []struct {
		name     string
		code     string
		contains []string
	}{
		{
			name:     "simple integer",
			code:     "42",
			contains: []string{"Program", "Int", "42"},
		},
		{
			name:     "variable declaration",
			code:     "let x = 1",
			contains: []string{"Program", "Var", "x", "Int", "1"},
		},
		{
			name:     "binary expression",
			code:     "1 + 2",
			contains: []string{"Infix", "+", "Int"},
		},
		{
			name:     "function",
			code:     "function add(a, b) { return a + b }",
			contains: []string{"Func", "add", "Block", "Return", "Infix"},
		},
		{
			name:     "if statement",
			code:     "if (x > 0) { 1 } else { 2 }",
			contains: []string{"If", "condition", "then", "else"},
		},
		{
			name:     "string literal",
			code:     `"hello"`,
			contains: []string{"String", "hello"},
		},
		{
			name:     "list literal",
			code:     "[1, 2, 3]",
			contains: []string{"List", "3 items", "Int"},
		},
		{
			name:     "map literal",
			code:     "{a: 1, b: 2}",
			contains: []string{"Map", "2 pairs"},
		},
		{
			name:     "method call",
			code:     "s.upper()",
			contains: []string{"ObjectCall", "Ident", "upper"},
		},
		{
			name:     "index expression",
			code:     "list[0]",
			contains: []string{"Index", "Ident", "Int"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := parser.Parse(context.Background(), tt.code, nil)
			assert.Nil(t, err)

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printAST(program)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			for _, expected := range tt.contains {
				assert.True(t, contains(output, expected),
					"expected output to contain %q, got: %s", expected, output)
			}
		})
	}
}

func TestPrintNode_Nil(t *testing.T) {
	// Should not panic
	printNode(nil, "", true)
}

func TestPrintNode_Literals(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() {
		color.Enabled = oldEnabled
	}()

	tests := []struct {
		name     string
		node     ast.Node
		contains string
	}{
		{
			name:     "nil literal",
			node:     &ast.Nil{},
			contains: "Nil",
		},
		{
			name:     "bool true",
			node:     &ast.Bool{Value: true},
			contains: "true",
		},
		{
			name:     "bool false",
			node:     &ast.Bool{Value: false},
			contains: "false",
		},
		{
			name:     "float",
			node:     &ast.Float{Value: 3.14},
			contains: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printNode(tt.node, "", true)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			assert.True(t, contains(output, tt.contains))
		})
	}
}

func TestPrintChildrenReflect(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() {
		color.Enabled = oldEnabled
	}()

	// Test with a node that uses reflection fallback
	node := &ast.BadExpr{}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printChildrenReflect(node, "  ")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	// BadExpr has no children, so output should be empty
	assert.Equal(t, buf.String(), "")
}

func TestAstHandler(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() {
		color.Enabled = oldEnabled
	}()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("ast").
		Args("file?").
		Flags(
			cli.String("code", "c").Help("Code to parse"),
			cli.Bool("stdin", "").Help("Read code from stdin"),
		).
		Run(astHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"ast", "-c", "let x = 1 + 2"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.True(t, contains(output, "Program"))
	assert.True(t, contains(output, "Var"))
	assert.True(t, contains(output, "Infix"))
}

func TestGetAstCode_NoInput(t *testing.T) {
	app := cli.New("test").SetColorEnabled(false)
	var capturedErr error
	app.Command("test").
		Args("file?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
		).
		Run(func(ctx *cli.Context) error {
			_, capturedErr = getAstCode(ctx)
			return capturedErr
		})

	_ = app.ExecuteArgs([]string{"test"})
	assert.NotNil(t, capturedErr)
	assert.True(t, contains(capturedErr.Error(), "no input"))
}

func TestGetAstCode_MultipleInputs(t *testing.T) {
	app := cli.New("test").SetColorEnabled(false)
	var capturedErr error
	app.Command("test").
		Args("file?").
		Flags(
			cli.String("code", "c"),
			cli.Bool("stdin", ""),
		).
		Run(func(ctx *cli.Context) error {
			_, capturedErr = getAstCode(ctx)
			return capturedErr
		})

	_ = app.ExecuteArgs([]string{"test", "-c", "1+2", "somefile.risor"})
	assert.NotNil(t, capturedErr)
	assert.True(t, contains(capturedErr.Error(), "multiple"))
}
