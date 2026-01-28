package main

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/parser"
)

func TestFormatProgram(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple variable",
			input:    "let x=1",
			expected: "let x = 1\n",
		},
		{
			name:     "variable with expression",
			input:    "let x=1+2*3",
			expected: "let x = 1 + 2 * 3\n",
		},
		{
			name:     "constant",
			input:    "const PI=3.14",
			expected: "const PI = 3.14\n",
		},
		{
			name:  "function",
			input: "function add(a,b){return a+b}",
			expected: `function add(a, b) {
    return a + b
}
`,
		},
		{
			name:  "if statement",
			input: "if(x>0){\"positive\"}",
			expected: `if (x > 0) {
    "positive"
}
`,
		},
		{
			name:  "if-else",
			input: "if(x>0){\"positive\"}else{\"negative\"}",
			expected: `if (x > 0) {
    "positive"
} else {
    "negative"
}
`,
		},
		{
			name:     "list",
			input:    "[1,2,3]",
			expected: "[1, 2, 3]\n",
		},
		{
			name:     "map",
			input:    "{name:\"Alice\",age:30}",
			expected: "{name: \"Alice\", age: 30}\n",
		},
		{
			name:     "empty map",
			input:    "{}",
			expected: "{}\n",
		},
		{
			name:     "method call",
			input:    "s.upper()",
			expected: "s.upper()\n",
		},
		{
			name:     "index access",
			input:    "list[0]",
			expected: "list[0]\n",
		},
		{
			name:     "slice",
			input:    "list[1:3]",
			expected: "list[1:3]\n",
		},
		{
			name:     "prefix operator",
			input:    "!x",
			expected: "!x\n",
		},
		{
			name:     "return statement",
			input:    "return 42",
			expected: "return 42\n",
		},
		{
			name:     "nil literal",
			input:    "nil",
			expected: "nil\n",
		},
		{
			name:     "bool true",
			input:    "true",
			expected: "true\n",
		},
		{
			name:     "bool false",
			input:    "false",
			expected: "false\n",
		},
		{
			name:     "in expression",
			input:    "x in list",
			expected: "x in list\n",
		},
		{
			name:     "not in expression",
			input:    "x not in list",
			expected: "x not in list\n",
		},
		{
			name:     "spread operator",
			input:    "[...a]",
			expected: "[...a]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := parser.Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			result := formatProgram(program)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestFormatterIndentation(t *testing.T) {
	input := `function outer() {
function inner() {
return 1
}
return inner()
}`
	expected := `function outer() {
    function inner() {
        return 1
    }
    return inner()
}
`
	program, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	result := formatProgram(program)
	assert.Equal(t, result, expected)
}

func TestFormatterMultipleStatements(t *testing.T) {
	input := "let x = 1\nlet y = 2\nlet z = x + y"
	expected := `let x = 1

let y = 2

let z = x + y
`
	program, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	result := formatProgram(program)
	assert.Equal(t, result, expected)
}

func TestFormatterFunctionWithDefaults(t *testing.T) {
	input := "function greet(name, greeting = \"Hello\") { return greeting + name }"
	program, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	result := formatProgram(program)
	assert.True(t, len(result) > 0)
	// Verify it contains the default parameter
	assert.True(t, contains(result, "greeting = \"Hello\"") || contains(result, "greeting=\"Hello\""))
}

func TestFormatterTryCatch(t *testing.T) {
	input := "try { throw error(\"oops\") } catch e { e }"
	program, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	result := formatProgram(program)
	assert.True(t, contains(result, "try"))
	assert.True(t, contains(result, "catch"))
}

func TestFormatterSwitch(t *testing.T) {
	input := `switch (x) {
case 1: "one"
case 2: "two"
default: "other"
}`
	program, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	result := formatProgram(program)
	assert.True(t, contains(result, "switch"))
	assert.True(t, contains(result, "case 1:"))
	assert.True(t, contains(result, "default:"))
}

func TestFormatterDestructuring(t *testing.T) {
	t.Run("object destructure", func(t *testing.T) {
		input := "let {name, age} = person"
		program, err := parser.Parse(context.Background(), input, nil)
		assert.Nil(t, err)

		result := formatProgram(program)
		assert.True(t, contains(result, "let {"))
		assert.True(t, contains(result, "name"))
		assert.True(t, contains(result, "age"))
	})

	t.Run("array destructure", func(t *testing.T) {
		input := "let [first, second] = list"
		program, err := parser.Parse(context.Background(), input, nil)
		assert.Nil(t, err)

		result := formatProgram(program)
		assert.True(t, contains(result, "let ["))
		assert.True(t, contains(result, "first"))
		assert.True(t, contains(result, "second"))
	})
}

// Helper to avoid using strings.Contains directly in assertions
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFormatterWriteIndent(t *testing.T) {
	f := &Formatter{indent: 2}
	f.writeIndent()
	assert.Equal(t, f.buf.String(), "        ") // 2 * 4 spaces
}

func TestFormatterFormatNode_Nil(t *testing.T) {
	f := &Formatter{}
	f.formatNode(nil) // Should not panic
	assert.Equal(t, f.buf.String(), "")
}

func TestFormatterFormatNode_UnknownType(t *testing.T) {
	f := &Formatter{}
	// BadExpr is a valid AST node but not explicitly handled
	f.formatNode(&ast.BadExpr{})
	// Should produce a fallback comment
	assert.True(t, contains(f.buf.String(), "BadExpr"))
}
