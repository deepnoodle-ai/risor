package risor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/risor-io/risor/dis"
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
	code, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	stats := code.Stats()

	if stats.InstructionCount == 0 {
		t.Error("expected non-zero instruction count")
	}

	if stats.ConstantCount == 0 {
		t.Error("expected non-zero constant count")
	}

	if stats.GlobalCount == 0 {
		t.Error("expected non-zero global count")
	}

	if stats.FunctionCount != 2 {
		t.Errorf("expected 2 functions, got %d", stats.FunctionCount)
	}

	// Source bytes may differ from len(source) due to AST stringification
	if stats.SourceBytes == 0 {
		t.Error("expected non-zero source bytes")
	}
}

func TestCodeDisassemble(t *testing.T) {
	source := `let x = 1 + 2`
	code, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	instructions, err := dis.Disassemble(code)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	dis.Print(instructions, &buf)
	disasm := buf.String()

	if disasm == "" {
		t.Error("expected non-empty disassembly")
	}

	// Should contain some common opcodes
	if !strings.Contains(disasm, "LOAD_CONST") {
		t.Error("expected disassembly to contain LOAD_CONST")
	}
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
	code, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	names := code.FunctionNames()

	// Should have at least 2 named functions
	if len(names) < 2 {
		t.Errorf("expected at least 2 function names, got %d", len(names))
	}

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

	if !foundAdd {
		t.Error("expected to find 'add' in function names")
	}
	if !foundSubtract {
		t.Error("expected to find 'subtract' in function names")
	}
}

func TestCodeGlobalNames(t *testing.T) {
	source := `
let x = 1
let y = 2
let z = x + y
`
	code, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	names := code.GlobalNames()

	// GlobalNames includes builtins plus user-defined globals
	// Check that our user-defined globals are present
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, expected := range []string{"x", "y", "z"} {
		if !nameSet[expected] {
			t.Errorf("expected to find %q in global names", expected)
		}
	}
}
