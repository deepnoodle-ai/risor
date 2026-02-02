package risor

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/dis"
)

func TestCodeStats(t *testing.T) {
	source := `
let x = 1
let y = 2

function add(a, b) {
	return a + b
}

function multiply(a, b) {
	return a * b
}

let result = add(x, y)
`
	code, err := Compile(context.Background(), source)
	assert.Nil(t, err)

	stats := code.Stats()

	assert.True(t, stats.InstructionCount > 0, "expected non-zero instruction count")
	assert.True(t, stats.ConstantCount > 0, "expected non-zero constant count")
	assert.True(t, stats.GlobalCount > 0, "expected non-zero global count")
	assert.Equal(t, stats.FunctionCount, 2)
	assert.True(t, stats.SourceBytes > 0, "expected non-zero source bytes")
}

func TestCodeDisassemble(t *testing.T) {
	source := `let x = 1 + 2`
	code, err := Compile(context.Background(), source)
	assert.Nil(t, err)

	instructions, err := dis.Disassemble(code)
	assert.Nil(t, err)

	var buf bytes.Buffer
	dis.Print(instructions, &buf)
	disasm := buf.String()

	assert.True(t, disasm != "", "expected non-empty disassembly")
	assert.True(t, strings.Contains(disasm, "LOAD_CONST"), "expected disassembly to contain LOAD_CONST")
}

func TestCodeFunctionNames(t *testing.T) {
	source := `
function add(a, b) {
	return a + b
}

function subtract(a, b) {
	return a - b
}

// Anonymous function
let f = function(x) { return x * 2 }
`
	code, err := Compile(context.Background(), source)
	assert.Nil(t, err)

	names := code.FunctionNames()

	// Should have at least 2 named functions
	assert.True(t, len(names) >= 2, "expected at least 2 function names, got %d", len(names))

	// Check for expected function names
	foundAdd := false
	foundSubtract := false
	for _, name := range names {
		if name == "add" {
			foundAdd = true
		}
		if name == "subtract" {
			foundSubtract = true
		}
	}

	assert.True(t, foundAdd, "expected to find 'add' in function names")
	assert.True(t, foundSubtract, "expected to find 'subtract' in function names")
}

func TestCodeGlobalNames(t *testing.T) {
	source := `
let x = 1
let y = 2
let z = x + y
`
	code, err := Compile(context.Background(), source)
	assert.Nil(t, err)

	names := code.GlobalNames()

	// GlobalNames includes builtins plus user-defined globals
	// Check that our user-defined globals are present
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, expected := range []string{"x", "y", "z"} {
		assert.True(t, nameSet[expected], "expected to find %q in global names", expected)
	}
}
