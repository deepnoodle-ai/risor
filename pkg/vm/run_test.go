package vm

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "1 + 1", nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)
	result, err := Run(ctx, code)
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.Int).Value(), int64(2))
}

func TestRunEmpty(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "", nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)
	result, err := Run(ctx, code)
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestRunError(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "let foo = 42; foo.bar", nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)
	_, err = Run(ctx, code)
	assert.NotNil(t, err)
	// Check that the error message contains the expected content
	assert.Contains(t, err.Error(), "type error: attribute \"bar\" not found on int object")
	// Check that it's a structured error with location info
	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok)
	assert.Equal(t, structuredErr.Kind, object.ErrType)
}

func TestRunError_WithLocation(t *testing.T) {
	ctx := context.Background()
	// Error on line 2, column 1 (accessing .bar on int)
	source := `let foo = 42
foo.bar`

	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{Filename: "test.risor"})
	assert.Nil(t, err)

	_, err = Run(ctx, code)
	assert.NotNil(t, err)

	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")

	// Verify location is set
	loc := structuredErr.Location
	assert.False(t, loc.IsZero(), "location should not be zero")
	assert.Equal(t, loc.Filename, "test.risor")
	assert.Equal(t, loc.Line, 2, "error should be on line 2")
}

func TestRunError_StackTrace(t *testing.T) {
	ctx := context.Background()
	// Error in nested function call
	source := `function inner() {
	let x = 42
	return x.bad_attr
}

function outer() {
	return inner()
}

outer()`

	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{Filename: "stack.risor"})
	assert.Nil(t, err)

	_, err = Run(ctx, code)
	assert.NotNil(t, err)

	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")

	// Verify stack trace is populated
	stack := structuredErr.Stack
	assert.GreaterOrEqual(t, len(stack), 2, "stack should have at least 2 frames")

	// First frame should be inner function (where error occurred)
	assert.Equal(t, stack[0].Function, "inner")

	// Second frame should be outer function (the caller)
	assert.Equal(t, stack[1].Function, "outer")
}

func TestRunError_NestedAttributeAccess(t *testing.T) {
	ctx := context.Background()
	// Type error: accessing attribute on wrong type inside nested structure
	source := `let data = {
	"value": 42
}
data.value.bad`

	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{Filename: "nested.risor"})
	assert.Nil(t, err)

	_, err = Run(ctx, code)
	assert.NotNil(t, err)

	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")
	assert.Equal(t, structuredErr.Kind, object.ErrType)

	// Verify location points to the nested access line
	loc := structuredErr.Location
	assert.Equal(t, loc.Line, 4, "error should be on line 4")
	assert.Equal(t, loc.Filename, "nested.risor")
}

func TestRunError_UndefinedMethod(t *testing.T) {
	ctx := context.Background()
	source := `let items = [1, 2, 3]
items.nonexistent()`

	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)

	_, err = Run(ctx, code)
	assert.NotNil(t, err)

	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")

	// Error should have location
	assert.False(t, structuredErr.Location.IsZero())
}

func TestRunError_FriendlyMessage(t *testing.T) {
	ctx := context.Background()
	source := `let x = 42
x.missing`

	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	code, err := compiler.Compile(ast, &compiler.Config{Filename: "friendly.risor"})
	assert.Nil(t, err)

	_, err = Run(ctx, code)
	assert.NotNil(t, err)

	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")

	// Get friendly message
	friendly := structuredErr.FriendlyErrorMessage()

	// Should contain the error type and message
	assert.Contains(t, friendly, "type error")
	// Should contain location info
	assert.Contains(t, friendly, "2:")
}

func TestRunError_AttributeErrorPointsAtAttributeName(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		source         string
		expectedLine   int
		expectedColumn int
		description    string
	}{
		{
			name:           "GetAttr on map literal",
			source:         `{a:1}.xyz`,
			expectedLine:   1,
			expectedColumn: 7, // "xyz" starts at column 7
			description:    "error should point at attribute name, not map literal",
		},
		{
			name:           "ObjectCall on map literal",
			source:         `{a:1}.fake()`,
			expectedLine:   1,
			expectedColumn: 7, // "fake" starts at column 7
			description:    "error should point at method name, not map literal",
		},
		{
			name:           "GetAttr on nested expression",
			source:         `(1 + 2).bad`,
			expectedLine:   1,
			expectedColumn: 9, // "bad" starts at column 9
			description:    "error should point at attribute name, not grouped expression",
		},
		{
			name:           "ObjectCall on list literal",
			source:         `[1, 2, 3].fake()`,
			expectedLine:   1,
			expectedColumn: 11, // "fake" starts at column 11
			description:    "error should point at method name, not list literal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := parser.Parse(ctx, tt.source, nil)
			assert.Nil(t, err)

			code, err := compiler.Compile(ast, nil)
			assert.Nil(t, err)

			_, err = Run(ctx, code)
			assert.NotNil(t, err)

			structuredErr, ok := err.(*object.StructuredError)
			assert.True(t, ok, "should be StructuredError")

			loc := structuredErr.Location
			assert.Equal(t, loc.Line, tt.expectedLine, tt.description+" (line)")
			assert.Equal(t, loc.Column, tt.expectedColumn, tt.description+" (column)")
		})
	}
}

// A runtime error shows the script's own line, with the caret under the
// failing token, also when the line is indented (#540).
func TestRunError_ShowsOriginalSourceLine(t *testing.T) {
	ctx := context.Background()
	indent := strings.Repeat(" ", 40)
	source := "let x = 42\n" + indent + "x.missing"
	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, &compiler.Config{Source: source})
	assert.Nil(t, err)

	_, err = Run(ctx, code)
	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")
	assert.Equal(t, structuredErr.Location.Line, 2)
	assert.Equal(t, structuredErr.Location.Column, 43)
	assert.Equal(t, structuredErr.Location.Source, indent+"x.missing")
	friendly := structuredErr.FriendlyErrorMessage()
	assert.Contains(t, friendly, " | "+indent+"x.missing\n")
	assert.Contains(t, friendly, " | "+indent+"  ^^^^^^^\n")

	// The test helper passes the source too.
	_, err = run(ctx, source)
	structuredErr, ok = err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")
	assert.Equal(t, structuredErr.Location.Source, indent+"x.missing")
}

// Without the source, a runtime error gives the line and column but no
// source line or caret, in the main code and in functions.
func TestRunError_WithoutSourceShowsNoSourceLine(t *testing.T) {
	ctx := context.Background()
	source := "function f() {\n    let v = 1\n    return v.missing\n}\nf()"
	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)

	_, err = Run(ctx, code)
	structuredErr, ok := err.(*object.StructuredError)
	assert.True(t, ok, "should be StructuredError")
	assert.Equal(t, structuredErr.Location.Line, 3)
	assert.Equal(t, structuredErr.Location.Column, 14)
	assert.Equal(t, structuredErr.Location.Source, "")
	for _, frame := range structuredErr.Stack {
		assert.Equal(t, frame.Location.Source, "")
	}
	assert.NotContains(t, structuredErr.FriendlyErrorMessage(), " | ")
}
