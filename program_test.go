package risor

import (
	"strings"
	"testing"
)

func TestProgramStats(t *testing.T) {
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
	program, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	stats := program.Stats()

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

	if stats.SourceBytes != len(source) {
		t.Errorf("expected source bytes %d, got %d", len(source), stats.SourceBytes)
	}
}

func TestProgramDisassemble(t *testing.T) {
	source := `let x = 1 + 2`
	program, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	dis, err := program.Disassemble()
	if err != nil {
		t.Fatal(err)
	}

	if dis == "" {
		t.Error("expected non-empty disassembly")
	}

	// Should contain some common opcodes
	if !strings.Contains(dis, "LOAD_CONST") {
		t.Error("expected disassembly to contain LOAD_CONST")
	}
}

func TestProgramFunctionNames(t *testing.T) {
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
	program, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	names := program.FunctionNames()

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

func TestProgramGlobalNames(t *testing.T) {
	source := `
let x = 1
let y = 2
let z = x + y
`
	program, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	names := program.GlobalNames()

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
