package vm

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
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

func compileLinesSource(t *testing.T, source string) *compiler.Code {
	t.Helper()
	program, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	c, err := compiler.New(&compiler.Config{Filename: "lines.risor", Source: source})
	assert.Nil(t, err)
	code, err := c.CompileAST(program)
	assert.Nil(t, err)
	return code
}

// oneLineFunctions returns a script on one line that defines n functions.
func oneLineFunctions(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "function f%d(a) { return a + %d }; ", i, i)
	}
	return b.String()
}

// Marshaled output must not repeat the source line for every instruction.
// When the script is on one line, that made the size grow with the square of
// the source size. Small inputs only: 4x the functions must give about 4x the
// output, not 16x.
func TestMarshalSizeLinearOnOneLine(t *testing.T) {
	small := compileLinesSource(t, oneLineFunctions(25))
	large := compileLinesSource(t, oneLineFunctions(100))

	smallBytecode, err := bytecode.Marshal(small.ToBytecode())
	assert.Nil(t, err)
	largeBytecode, err := bytecode.Marshal(large.ToBytecode())
	assert.Nil(t, err)
	ratio := float64(len(largeBytecode)) / float64(len(smallBytecode))
	assert.True(t, ratio < 6, "bytecode.Marshal size ratio %.1f", ratio)

	smallCompiler, err := compiler.MarshalCode(small)
	assert.Nil(t, err)
	largeCompiler, err := compiler.MarshalCode(large)
	assert.Nil(t, err)
	ratio = float64(len(largeCompiler)) / float64(len(smallCompiler))
	assert.True(t, ratio < 6, "compiler.MarshalCode size ratio %.1f", ratio)
}

const sourceLinesScript = `let x = 1
function outer() {
	function inner(a) {
		return a + "s"
	}
	return inner(x)
}
outer()
`

// runError runs code and returns the runtime error it raises.
func runError(t *testing.T, code *bytecode.Code) *errors.StructuredError {
	t.Helper()
	_, err := Run(context.Background(), code)
	assert.NotNil(t, err)
	var structured *errors.StructuredError
	assert.True(t, stderrors.As(err, &structured))
	return structured
}

// A runtime error after a round trip shows the same source line and caret as
// before it, in both marshaled formats.
func TestMarshalRoundTripKeepsSourceLines(t *testing.T) {
	code := compileLinesSource(t, sourceLinesScript)
	expected := runError(t, code.ToBytecode())
	assert.Equal(t, expected.Location.Line, 4)
	assert.Equal(t, expected.Location.Source, `		return a + "s"`)
	assert.Equal(t, expected.Location.Filename, "lines.risor")
	expectedMessage := expected.FriendlyErrorMessage()

	data, err := bytecode.Marshal(code.ToBytecode())
	assert.Nil(t, err)
	restored, err := bytecode.Unmarshal(data)
	assert.Nil(t, err)
	actual := runError(t, restored)
	assert.Equal(t, actual.Location, expected.Location)
	assert.Equal(t, actual.FriendlyErrorMessage(), expectedMessage)

	data, err = compiler.MarshalCode(code)
	assert.Nil(t, err)
	restoredCode, err := compiler.UnmarshalCode(data)
	assert.Nil(t, err)
	// Compare each restored source line with the script text, not with a
	// lookup on the restored code, so a wrong line source fails here.
	scriptLines := strings.Split(sourceLinesScript, "\n")
	checked := 0
	for _, c := range restoredCode.Flatten() {
		for i := 0; i < c.LocationsCount(); i++ {
			loc := c.LocationAt(i)
			if loc.Line < 1 {
				continue
			}
			assert.Equal(t, loc.Source, scriptLines[loc.Line-1], "line %d", loc.Line)
			checked++
		}
	}
	assert.True(t, checked > 0)
	actual = runError(t, restoredCode.ToBytecode())
	assert.Equal(t, actual.Location, expected.Location)
	assert.Equal(t, actual.FriendlyErrorMessage(), expectedMessage)
}

// Older versions wrote the filename and source line of every location.
// Unmarshal still reads that data, and takes source lines from the code's
// source rather than from the location.
func TestUnmarshalOldFormatLocationSource(t *testing.T) {
	bytecodeData := `{"codes":[{"id":"main","name":"main","instructions":[` +
		fmt.Sprint(int(op.Nil)) + `,` + fmt.Sprint(int(op.ReturnValue)) +
		`],"constants":[],"names":[],"source":"let x = 1","filename":"old.risor",` +
		`"locations":[{"filename":"old.risor","line":1,"column":5,"source":"stale"},` +
		`{"filename":"old.risor","line":1,"column":1,"source":"stale"}]}]}`
	restored, err := bytecode.Unmarshal([]byte(bytecodeData))
	assert.Nil(t, err)
	assert.Equal(t, restored.LocationAt(0).Column, 5)
	assert.Equal(t, restored.GetSourceLine(restored.LocationAt(0).Line), "let x = 1")

	data, err := compiler.MarshalCode(compileLinesSource(t, "let x = 1\nx"))
	assert.Nil(t, err)
	old := strings.ReplaceAll(string(data), `"line":`, `"source":"stale","line":`)
	assert.True(t, strings.Contains(old, `"source":"stale"`))
	restoredCode, err := compiler.UnmarshalCode([]byte(old))
	assert.Nil(t, err)
	assert.True(t, restoredCode.LocationsCount() > 0)
	for i := 0; i < restoredCode.LocationsCount(); i++ {
		loc := restoredCode.LocationAt(i)
		assert.Equal(t, loc.Source, restoredCode.GetSourceLine(loc.Line))
		assert.NotEqual(t, loc.Source, "stale")
	}
}

// A location whose line number has no source line passes validation, as code
// built without source does. It gets an empty source line, and a runtime
// error at that location is reported without a snippet.
func TestUnmarshalLocationWithoutSourceLine(t *testing.T) {
	for _, line := range []int{-1, 0, 2, 1 << 40} {
		t.Run(fmt.Sprint(line), func(t *testing.T) {
			// Instructions: LOAD_CONST 0, LOAD_CONST 1, BINARY_OP add,
			// RETURN_VALUE, which fails at run time: int + string.
			data := fmt.Sprintf(`{"codes":[{"id":"main","name":"main","instructions":[%d,0,%d,1,%d,%d,%d],`+
				`"constants":[{"type":"int","value":1},{"type":"string","value":"s"}],"names":[],`+
				`"source":"1 + \"s\"","locations":[{"line":%d,"column":3},{"line":%d,"column":3},`+
				`{"line":%d,"column":3},{"line":%d,"column":3},{"line":%d,"column":3},{"line":%d,"column":3},{"line":%d,"column":3}]}]}`,
				op.LoadConst, op.LoadConst, op.BinaryOp, op.Add, op.ReturnValue,
				line, line, line, line, line, line, line)
			code, err := bytecode.Unmarshal([]byte(data))
			assert.Nil(t, err)
			assert.Nil(t, bytecode.Validate(code))
			assert.Equal(t, code.GetSourceLine(line), "")
			runErr := runError(t, code)
			assert.Equal(t, runErr.Location.Source, "")
			assert.False(t, strings.Contains(runErr.FriendlyErrorMessage(), " | "))
		})
	}
}

// Code compiled without its original source stores the program's string
// form, whose lines can be shorter than the columns of its locations. Such
// code must still load, so Unmarshal does not check columns against source
// lines.
func TestUnmarshalAcceptsColumnsPastStoredSource(t *testing.T) {
	indent := strings.Repeat(" ", 40)
	program, err := parser.Parse(context.Background(),
		indent+"let x = 1\n"+indent+"x + 1", nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(program, nil)
	assert.Nil(t, err)

	pastLine := false
	for i := 0; i < code.LocationCount(); i++ {
		loc := code.LocationAt(i)
		if loc.Column > len(code.GetSourceLine(loc.Line))+1 {
			pastLine = true
		}
	}
	assert.True(t, pastLine)

	data, err := bytecode.Marshal(code)
	assert.Nil(t, err)
	restored, err := bytecode.Unmarshal(data)
	assert.Nil(t, err)
	result, err := Run(context.Background(), restored)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(2))
}
