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
