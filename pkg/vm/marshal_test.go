package vm

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// roundTripSource uses each feature whose metadata must survive marshaling:
// a rest parameter, try/catch/finally, a closure, a block variable, a blank
// identifier, and a global from the host env.
const roundTripSource = `
function tail(first, ...rest) { return rest }
function counter() {
	let n = 0
	return function() { n++; return n }
}
let c = counter()
c()
let caught = ""
let cleaned = false
try {
	throw "boom"
} catch e {
	caught = string(e)
} finally {
	cleaned = true
}
if (true) {
	let caught = "shadow"
}
let [_, second] = [1, 2]
[tail(1, 2, 3), c(), caught, cleaned, second, host]
`

func compileRoundTripSource(t *testing.T) *compiler.Code {
	t.Helper()
	program, err := parser.Parse(context.Background(), roundTripSource, nil)
	assert.Nil(t, err)
	c, err := compiler.New(&compiler.Config{
		GlobalNames: []string{"string", "host"},
		Filename:    "roundtrip.risor",
	})
	assert.Nil(t, err)
	code, err := c.CompileAST(program)
	assert.Nil(t, err)
	return code
}

func runRoundTrip(t *testing.T, code *bytecode.Code) object.Object {
	t.Helper()
	result, err := Run(context.Background(), code, WithGlobals(map[string]any{
		"string": object.NewBuiltin("string", func(ctx context.Context, args ...object.Object) (object.Object, error) {
			return object.NewString(args[0].(*object.Error).Message().Value()), nil
		}),
		"host": int64(7),
	}))
	assert.Nil(t, err)
	return result
}

func TestBytecodeMarshalRoundTripRuns(t *testing.T) {
	original := compileRoundTripSource(t).ToBytecode()
	expected := runRoundTrip(t, original)
	assert.Equal(t, expected.Inspect(), `[[2, 3], 2, "boom", true, 2, 7]`)

	data, err := bytecode.Marshal(original)
	assert.Nil(t, err)
	restored, err := bytecode.Unmarshal(data)
	assert.Nil(t, err)

	assert.Equal(t, restored.EnvKeys(), original.EnvKeys())
	assert.Equal(t, restored.GlobalNames(), original.GlobalNames())
	assert.Equal(t, restored.Filename(), "roundtrip.risor")
	assert.Equal(t, runRoundTrip(t, restored).Inspect(), expected.Inspect())
}

func TestCompilerMarshalRoundTripRuns(t *testing.T) {
	code := compileRoundTripSource(t)
	original := code.ToBytecode()
	expected := runRoundTrip(t, original)

	data, err := compiler.MarshalCode(code)
	assert.Nil(t, err)
	restoredCode, err := compiler.UnmarshalCode(data)
	assert.Nil(t, err)
	restored := restoredCode.ToBytecode()

	assert.Equal(t, restored.EnvKeys(), original.EnvKeys())
	assert.Equal(t, restored.GlobalNames(), original.GlobalNames())
	assert.Equal(t, restored.Filename(), "roundtrip.risor")
	assert.Equal(t, restored.ExceptionHandlerCount(), original.ExceptionHandlerCount())
	assert.Equal(t, runRoundTrip(t, restored).Inspect(), expected.Inspect())
}

// Only the root block is unnamed. A user function named __main__ keeps its
// name slot after a round trip, so its compiled local count still validates.
func TestCompilerMarshalRoundTripFunctionNamedMain(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
function __main__() { return 1 }
__main__()
`, nil)
	assert.Nil(t, err)
	c, err := compiler.New(nil)
	assert.Nil(t, err)
	code, err := c.CompileAST(program)
	assert.Nil(t, err)

	data, err := compiler.MarshalCode(code)
	assert.Nil(t, err)
	restoredCode, err := compiler.UnmarshalCode(data)
	assert.Nil(t, err)
	codes := restoredCode.Flatten()
	assert.Equal(t, len(codes), 2)
	assert.False(t, codes[0].IsNamed())
	assert.True(t, codes[1].IsNamed())
	assert.Equal(t, codes[1].CodeName(), "__main__")
	result, err := Run(context.Background(), restoredCode.ToBytecode())
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(1))
}

// Validation does not cover the types of values on the stack. A fault like
// a non-string map key must still come back as an error, not a crash.
func TestRunUncheckedBytecodeFaultReturnsError(t *testing.T) {
	code := bytecode.NewCode(bytecode.CodeParams{
		ID:           "main",
		Name:         "main",
		Instructions: []op.Code{op.LoadConst, 0, op.LoadConst, 0, op.BuildMap, 1},
		Constants:    []any{int64(1)},
	})
	assert.Nil(t, bytecode.Validate(code))
	_, err := Run(context.Background(), code)
	assert.NotNil(t, err)
}

// A local count larger than the code needs makes every call frame allocate
// that many slots. With deep recursion, 1.6 KB of bytecode could make the
// host allocate about 1 GB, so loading rejects it.
func TestUnmarshalRejectsOversizedLocalCount(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
	function f(n) {
		if (n > 0) { return f(n - 1) }
		return 7
	}
	f(1000)
	`, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(program, nil)
	assert.Nil(t, err)
	data, err := bytecode.Marshal(code)
	assert.Nil(t, err)

	// The unmodified bytecode loads and runs
	restored, err := bytecode.Unmarshal(data)
	assert.Nil(t, err)
	result, err := Run(context.Background(), restored)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(7))

	// Raise the local count of f to the uint16 maximum
	var state map[string]any
	assert.Nil(t, json.Unmarshal(data, &state))
	codes := state["codes"].([]any)
	assert.Equal(t, len(codes), 2)
	codes[1].(map[string]any)["local_count"] = 65535
	data, err = json.Marshal(state)
	assert.Nil(t, err)

	_, err = bytecode.Unmarshal(data)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(),
		`invalid bytecode in "f": local count 65535 is larger than the 3 locals the code uses`)
}

// Closures that capture a variable two or more function levels out, from the
// tests for #512. The intermediate functions pass the cell on with LOAD_CELL.
var loadCellSources = []struct {
	source string
	want   object.Object
}{
	{`
	function f1() {
		let a = 1; let b = 42
		return function f2() { return function f3() { return b } }
	}
	f1()()()`, object.NewInt(42)},
	{`
	function f1() {
		let x = "deep"
		return () => () => () => x
	}
	f1()()()()`, object.NewString("deep")},
	{`
	function f1() {
		let a = "a"
		return function f2() {
			let b = "b"
			return function f3() { return () => a + b }
		}
	}
	f1()()()()`, object.NewString("ab")},
	{`
	function counter() {
		let n = 0
		return {
			inc: () => () => { n++; return n },
			read: () => () => n,
		}
	}
	let c = counter()
	let inc = c.inc()
	inc(); inc(); inc()
	c.read()()`, object.NewInt(3)},
}

func compileLoadCellSource(t *testing.T, source string) *bytecode.Code {
	t.Helper()
	program, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(program, nil)
	assert.Nil(t, err)
	return code
}

// loadCellOperands returns the position of each LOAD_CELL operand in the
// marshaled instructions of each code block.
func loadCellOperands(t *testing.T, state map[string]any) [][2]int {
	t.Helper()
	var found [][2]int
	for i, c := range state["codes"].([]any) {
		instrs, _ := c.(map[string]any)["instructions"].([]any)
		for pos := 0; pos < len(instrs); {
			opcode := op.Code(instrs[pos].(float64))
			if opcode == op.LoadCell {
				found = append(found, [2]int{i, pos + 1})
			}
			pos += 1 + op.GetInfo(opcode).OperandCount
		}
	}
	return found
}

func TestLoadCellClosuresValidateAfterRoundTrip(t *testing.T) {
	for _, tt := range loadCellSources {
		code := compileLoadCellSource(t, tt.source)
		assert.Nil(t, bytecode.Validate(code))
		data, err := bytecode.Marshal(code)
		assert.Nil(t, err)

		var state map[string]any
		assert.Nil(t, json.Unmarshal(data, &state))
		assert.True(t, len(loadCellOperands(t, state)) > 0, "no LOAD_CELL in compiled code")

		restored, err := bytecode.Unmarshal(data)
		assert.Nil(t, err)
		assert.Nil(t, bytecode.Validate(restored))
		result, err := Run(context.Background(), restored)
		assert.Nil(t, err)
		assert.Equal(t, result, tt.want)
	}
}

func TestUnmarshalRejectsLoadCellOutOfRange(t *testing.T) {
	data, err := bytecode.Marshal(compileLoadCellSource(t, loadCellSources[0].source))
	assert.Nil(t, err)

	// f2 is created with one captured cell, so its LOAD_CELL may only use
	// free variable index 0. Point it at index 1.
	var state map[string]any
	assert.Nil(t, json.Unmarshal(data, &state))
	operands := loadCellOperands(t, state)
	assert.Equal(t, len(operands), 1)
	codes := state["codes"].([]any)
	instrs := codes[operands[0][0]].(map[string]any)["instructions"].([]any)
	assert.Equal(t, instrs[operands[0][1]], float64(0))
	instrs[operands[0][1]] = 1
	data, err = json.Marshal(state)
	assert.Nil(t, err)

	_, err = bytecode.Unmarshal(data)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), `invalid bytecode in "f2": LOAD_CELL at offset 0: `+
		`free variable index 1 out of range (count 1)`)
}

// The VM checks LOAD_CELL at run time too, for code that skipped Validate
// or a function a host calls with fewer captures than the script gives it.
func TestRunUncheckedLoadCellReturnsError(t *testing.T) {
	code := bytecode.NewCode(bytecode.CodeParams{
		ID:           "main",
		Name:         "main",
		Instructions: []op.Code{op.LoadCell, 0},
	})
	assert.NotNil(t, bytecode.Validate(code))
	_, err := Run(context.Background(), code)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "runtime error: no free variable at index 0")
}

// runFuzzedCode runs code with tight limits. The VM must return, with or
// without an error, and never panic past Run.
func runFuzzedCode(code *bytecode.Code) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _ = Run(ctx, code, WithMaxSteps(10000))
}

func FuzzUnmarshalAndRun(f *testing.F) {
	seed, err := bytecode.Marshal(compileFuzzSeed(f).ToBytecode())
	assert.Nil(f, err)
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		code, err := bytecode.Unmarshal(data)
		if err != nil {
			return
		}
		runFuzzedCode(code)
	})
}

func FuzzCompilerUnmarshalCodeAndRun(f *testing.F) {
	seed, err := compiler.MarshalCode(compileFuzzSeed(f))
	assert.Nil(f, err)
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		code, err := compiler.UnmarshalCode(data)
		if err != nil {
			return
		}
		runFuzzedCode(code.ToBytecode())
	})
}

func compileFuzzSeed(f *testing.F) *compiler.Code {
	program, err := parser.Parse(context.Background(), roundTripSource, nil)
	assert.Nil(f, err)
	c, err := compiler.New(&compiler.Config{GlobalNames: []string{"string", "host"}})
	assert.Nil(f, err)
	code, err := c.CompileAST(program)
	assert.Nil(f, err)
	return code
}
