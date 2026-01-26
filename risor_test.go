package risor

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/risor-io/risor/vm"
	"github.com/stretchr/testify/require"
)

func TestBasicUsage(t *testing.T) {
	result, err := Eval(context.Background(), "1 + 1")
	require.Nil(t, err)
	require.Equal(t, int64(2), result)
}

func TestConfirmNoBuiltins(t *testing.T) {
	type testCase struct {
		input       string
		expectedErr string
	}
	testCases := []testCase{
		{
			input:       "keys({foo: 1})",
			expectedErr: "compile error: undefined variable \"keys\"\n\nlocation: unknown:1:1 (line 1, column 1)",
		},
		{
			input:       "any([0, 0, 1])",
			expectedErr: "compile error: undefined variable \"any\"\n\nlocation: unknown:1:1 (line 1, column 1)",
		},
		{
			input:       "string(42)",
			expectedErr: "compile error: undefined variable \"string\"\n\nlocation: unknown:1:1 (line 1, column 1)",
		},
	}
	for _, tc := range testCases {
		_, err := Eval(context.Background(), tc.input, WithoutDefaultGlobals())
		require.NotNil(t, err)
		require.Equal(t, tc.expectedErr, err.Error())
	}
}

func TestDefaultGlobals(t *testing.T) {
	type testCase struct {
		input    string
		expected any
	}
	testCases := []testCase{
		{
			input:    "keys({foo: 1})",
			expected: []any{"foo"},
		},
		{
			input:    "any([0, 0, 1])",
			expected: true,
		},
		{
			input:    "let x = 0; try { throw \"boom\" } catch e { x = 42 }; x",
			expected: int64(42),
		},
		{
			input:    "string(42)",
			expected: "42",
		},
	}
	for _, tc := range testCases {
		result, err := Eval(context.Background(), tc.input)
		require.Nil(t, err)
		require.Equal(t, tc.expected, result)
	}
}

func TestWithDenyList(t *testing.T) {
	type testCase struct {
		input       string
		expectedErr error
	}
	testCases := []testCase{
		{
			input:       "keys({foo: 1})",
			expectedErr: nil,
		},
		{
			input:       "any([0, 0, 1])",
			expectedErr: errors.New("compile error: undefined variable \"any\"\n\nlocation: unknown:1:1 (line 1, column 1)"),
		},
		{
			input:       "math.abs(-1)",
			expectedErr: errors.New("compile error: undefined variable \"math\"\n\nlocation: unknown:1:1 (line 1, column 1)"),
		},
	}
	for _, tc := range testCases {
		_, err := Eval(context.Background(), tc.input, WithoutGlobals("any", "math"))
		// t.Logf("want: %q; got: %v", tc.expectedErr, err)
		if tc.expectedErr != nil {
			require.NotNil(t, err)
			require.Equal(t, tc.expectedErr.Error(), err.Error())
			continue
		}
		require.Nil(t, err)
	}
}

func TestWithoutDefaultGlobals(t *testing.T) {
	_, err := Eval(context.Background(), "json.marshal(42)", WithoutDefaultGlobals())
	require.NotNil(t, err)
	require.Equal(t, errors.New("compile error: undefined variable \"json\"\n\nlocation: unknown:1:1 (line 1, column 1)"), err)
}

func TestWithoutDefaultGlobal(t *testing.T) {
	_, err := Eval(context.Background(), "json.marshal(42)", WithoutGlobal("json"))
	require.NotNil(t, err)
	require.Equal(t, errors.New("compile error: undefined variable \"json\"\n\nlocation: unknown:1:1 (line 1, column 1)"), err)
}

func TestEvalCode(t *testing.T) {
	ctx := context.Background()

	source := `
	let x = 2
	let y = 3
	function add(a, b) { a + b }
	let result = add(x, y)
	x = 99
	result
	`

	ast, err := parser.Parse(ctx, source)
	require.Nil(t, err)
	code, err := compiler.Compile(ast)
	require.Nil(t, err)

	// Should be able to evaluate the precompiled code any number of times
	for i := 0; i < 100; i++ {
		result, err := EvalCode(ctx, code)
		require.Nil(t, err)
		require.Equal(t, object.NewInt(5), result)
	}
}

func TestCall(t *testing.T) {
	ctx := context.Background()
	source := `
	function add(a, b) { a + b }
	`
	ast, err := parser.Parse(ctx, source)
	require.Nil(t, err)
	code, err := compiler.Compile(ast)
	require.Nil(t, err)

	result, err := Call(ctx, code, "add", []object.Object{
		object.NewInt(9),
		object.NewInt(1),
	})
	require.Nil(t, err)
	require.Equal(t, object.NewInt(10), result)
}

func TestWithoutGlobal1(t *testing.T) {
	cfg := newConfig(
		WithoutDefaultGlobals(),
		WithGlobals(map[string]any{
			"foo": object.NewInt(1),
			"bar": object.NewInt(2),
		}),
		WithoutGlobal("bar"),
	)
	require.Equal(t, map[string]any{
		"foo": object.NewInt(1),
	}, cfg.Globals())
}

func TestWithoutGlobal2(t *testing.T) {
	cfg := newConfig(
		WithoutDefaultGlobals(),
		WithGlobals(map[string]any{
			"foo": object.NewInt(1),
			"bar": object.NewInt(2),
		}),
		WithoutGlobal("xyz"),
	)
	require.Equal(t, map[string]any{
		"foo": object.NewInt(1),
		"bar": object.NewInt(2),
	}, cfg.Globals())
}

func TestWithoutGlobal3(t *testing.T) {
	cfg := newConfig(
		WithGlobal("foo", object.NewInt(1)),
		WithoutGlobal("foo"),
	)
	_, hasFoo := cfg.Globals()["foo"]
	require.False(t, hasFoo)
}

func TestWithoutGlobal4(t *testing.T) {
	cfg := newConfig(
		WithoutDefaultGlobals(),
		WithGlobals(map[string]any{
			"foo": object.NewBuiltinsModule("foo", map[string]object.Object{
				"bar": object.NewBuiltinsModule("bar", map[string]object.Object{
					"baz": object.NewInt(1),
					"qux": object.NewInt(2),
				}),
			}),
		}),
		WithoutGlobal("foo.bar.baz"),
	)
	require.Equal(t, map[string]any{
		"foo": object.NewBuiltinsModule("foo", map[string]object.Object{
			"bar": object.NewBuiltinsModule("bar", map[string]object.Object{
				"qux": object.NewInt(2),
			}),
		}),
	}, cfg.Globals())
}

func TestWithGlobalOverride(t *testing.T) {
	cfg := newConfig(
		WithoutDefaultGlobals(),
		WithGlobals(map[string]any{
			"foo": object.NewInt(1),
			"bar": object.NewBuiltinsModule("bar", map[string]object.Object{
				"baz": object.NewInt(1),
				"qux": object.NewInt(2),
			}),
		}),
		WithGlobalOverride("foo", object.NewString("FOO")),
		WithGlobalOverride("bar.baz", object.NewString("BAZ")),
	)

	require.Equal(t, map[string]any{
		"foo": object.NewString("FOO"),
		"bar": object.NewBuiltinsModule("bar", map[string]object.Object{
			"baz": object.NewString("BAZ"),
			"qux": object.NewInt(2),
		}),
	}, cfg.Globals())
}

func TestStructFieldModification(t *testing.T) {
	type Object struct {
		A int
	}

	testCases := []struct {
		script   string
		expected int64
	}{
		{"Object.A = 9; Object.A *= 3; Object.A", 27},
		{"Object.A = 10; Object.A += 5; Object.A", 15},
		{"Object.A = 10; Object.A -= 3; Object.A", 7},
		{"Object.A = 20; Object.A /= 4; Object.A", 5},
	}

	for _, tc := range testCases {
		result, err := Eval(context.Background(),
			tc.script,
			WithGlobal("Object", &Object{}))

		require.Nil(t, err, "script: %s", tc.script)
		require.Equal(t, tc.expected, result, "script: %s", tc.script)
	}
}

func TestWithExistingVM(t *testing.T) {
	ctx := context.Background()

	vm, err := vm.NewEmpty()
	require.Nil(t, err)

	result, err := Eval(ctx,
		"foo",
		WithVM(vm),
		WithGlobals(map[string]any{"foo": object.NewInt(3)}),
	)
	require.Nil(t, err)
	require.Equal(t, int64(3), result)

	result, err = Eval(ctx,
		"bar",
		WithVM(vm),
		WithGlobals(map[string]any{"bar": object.NewInt(4)}),
	)
	require.Nil(t, err)
	require.Equal(t, int64(4), result)
}

func TestDefaultGlobalsFunc(t *testing.T) {
	globals := DefaultGlobals(DefaultGlobalsOpts{})
	expectedNames := []string{ // non-exhaustive
		"math",
		"rand",
		"regexp",
		"time",
	}
	for _, name := range expectedNames {
		_, ok := globals[name]
		require.True(t, ok, "expected global %s", name)
	}
}

// Test the new Compile/Run API
func TestCompileRun(t *testing.T) {
	program, err := Compile("1 + 2")
	require.Nil(t, err)
	require.NotNil(t, program)

	result, err := Run(context.Background(), program)
	require.Nil(t, err)
	require.Equal(t, int64(3), result)
}

// Test that the same Program can be run multiple times
func TestProgramReuse(t *testing.T) {
	program, err := Compile("x + 1", WithGlobal("x", int64(0)))
	require.Nil(t, err)

	for i := int64(0); i < 10; i++ {
		result, err := Run(context.Background(), program, WithGlobal("x", i))
		require.Nil(t, err)
		require.Equal(t, i+1, result)
	}
}

// Test concurrent execution of the same Program
func TestConcurrentExecution(t *testing.T) {
	program, err := Compile("x + 1", WithGlobal("x", int64(0)))
	require.Nil(t, err)

	var wg sync.WaitGroup
	results := make([]int64, 10)
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result, err := Run(context.Background(), program, WithGlobal("x", int64(id)))
			if err != nil {
				errors[id] = err
				return
			}
			results[id] = result.(int64)
		}(i)
	}
	wg.Wait()

	// Verify all goroutines succeeded with correct results
	for i := 0; i < 10; i++ {
		require.Nil(t, errors[i], "goroutine %d had an error", i)
		require.Equal(t, int64(i+1), results[i], "goroutine %d had wrong result", i)
	}
}

// Test the VM wrapper for REPL-style execution
func TestVMWrapper(t *testing.T) {
	vm, err := NewVM()
	require.Nil(t, err)

	// Define a function
	_, err = vm.Eval(context.Background(), "function add(a, b) { a + b }")
	require.Nil(t, err)

	// Call the function
	result, err := vm.Call(context.Background(), "add", int64(2), int64(3))
	require.Nil(t, err)
	require.Equal(t, int64(5), result)

	// Define a variable
	_, err = vm.Eval(context.Background(), "let x = 10")
	require.Nil(t, err)

	// Get the variable
	x, err := vm.Get("x")
	require.Nil(t, err)
	require.Equal(t, int64(10), x)
}
