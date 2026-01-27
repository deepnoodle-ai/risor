package risor

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

func TestBasicUsage(t *testing.T) {
	// By default, the environment is empty
	result, err := Eval(context.Background(), "1 + 1")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(2))
}

func TestEmptyEnvByDefault(t *testing.T) {
	// Verify that the environment is empty by default (no builtins available)
	testCases := []struct {
		input       string
		expectedErr string
	}{
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
		{
			input:       "math.abs(-1)",
			expectedErr: "compile error: undefined variable \"math\"\n\nlocation: unknown:1:1 (line 1, column 1)",
		},
	}
	for _, tc := range testCases {
		_, err := Eval(context.Background(), tc.input)
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), tc.expectedErr)
	}
}

func TestWithBuiltins(t *testing.T) {
	// Test that Builtins() provides the standard library
	testCases := []struct {
		input    string
		expected any
	}{
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
		{
			input:    "math.abs(-1)",
			expected: int64(1),
		},
	}
	for _, tc := range testCases {
		result, err := Eval(context.Background(), tc.input, WithEnv(Builtins()))
		assert.Nil(t, err)
		assert.Equal(t, result, tc.expected)
	}
}

func TestWithEnv(t *testing.T) {
	// Test providing custom environment
	result, err := Eval(context.Background(), "x + y", WithEnv(map[string]any{
		"x": int64(10),
		"y": int64(20),
	}))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(30))
}

func TestWithEnvAdditive(t *testing.T) {
	// Test that multiple WithEnv calls are additive
	result, err := Eval(context.Background(), "x + y",
		WithEnv(map[string]any{"x": int64(10)}),
		WithEnv(map[string]any{"y": int64(20)}),
	)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(30))
}

func TestWithEnvOverride(t *testing.T) {
	// Test that later WithEnv calls override earlier ones
	result, err := Eval(context.Background(), "x",
		WithEnv(map[string]any{"x": int64(10)}),
		WithEnv(map[string]any{"x": int64(99)}),
	)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(99))
}

func TestCustomizeBuiltins(t *testing.T) {
	// Test that users can customize the standard library
	env := Builtins()
	delete(env, "math") // Remove a module
	env["custom"] = int64(42)

	// math should not be available
	_, err := Eval(context.Background(), "math.abs(-1)", WithEnv(env))
	assert.NotNil(t, err)

	// custom should be available
	result, err := Eval(context.Background(), "custom", WithEnv(env))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(42))
}

func TestBuiltinsFunc(t *testing.T) {
	env := Builtins()
	expectedNames := []string{
		"math",
		"rand",
		"regexp",
		"time",
		"keys",
		"len",
		"string",
	}
	for _, name := range expectedNames {
		_, ok := env[name]
		assert.True(t, ok, "expected builtin %s", name)
	}
}

// Test the Compile/Run API
func TestCompileRun(t *testing.T) {
	program, err := Compile("1 + 2")
	assert.Nil(t, err)
	assert.NotNil(t, program)

	result, err := Run(context.Background(), program)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(3))
}

// Test that the same Program can be run multiple times with different env
func TestProgramReuse(t *testing.T) {
	program, err := Compile("x + 1", WithEnv(map[string]any{"x": int64(0)}))
	assert.Nil(t, err)

	for i := int64(0); i < 10; i++ {
		result, err := Run(context.Background(), program, WithEnv(map[string]any{"x": i}))
		assert.Nil(t, err)
		assert.Equal(t, result, i+1)
	}
}

// Test concurrent execution of the same Program
func TestConcurrentExecution(t *testing.T) {
	program, err := Compile("x + 1", WithEnv(map[string]any{"x": int64(0)}))
	assert.Nil(t, err)

	var wg sync.WaitGroup
	results := make([]int64, 10)
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result, err := Run(context.Background(), program, WithEnv(map[string]any{"x": int64(id)}))
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
		assert.Nil(t, errors[i], "goroutine %d had an error", i)
		assert.Equal(t, results[i], int64(i+1), "goroutine %d had wrong result", i)
	}
}

// Test custom type registry
func TestWithTypeRegistry(t *testing.T) {
	// Define a custom type
	type Point struct {
		X, Y int
	}

	// Create a custom registry that knows how to convert Point
	registry := NewTypeRegistry().
		RegisterFromGo(reflect.TypeOf(Point{}), func(v any) (object.Object, error) {
			p := v.(Point)
			return object.NewMap(map[string]object.Object{
				"x": object.NewInt(int64(p.X)),
				"y": object.NewInt(int64(p.Y)),
			}), nil
		}).
		Build()

	// Use the custom registry
	result, err := Eval(context.Background(), "point.x + point.y",
		WithEnv(map[string]any{"point": Point{X: 10, Y: 20}}),
		WithTypeRegistry(registry),
	)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(30))
}

// Test RisorValuer interface for custom types
type customValue struct {
	data string
}

func (c customValue) RisorValue() object.Object {
	return object.NewString("custom:" + c.data)
}

func TestRisorValuerIntegration(t *testing.T) {
	// Types implementing RisorValuer are automatically converted
	result, err := Eval(context.Background(), "val",
		WithEnv(map[string]any{"val": customValue{data: "test"}}),
	)
	assert.Nil(t, err)
	assert.Equal(t, result, "custom:test")
}

// Test WithRawResult option
func TestWithRawResult(t *testing.T) {
	// Without WithRawResult, we get a native Go value
	result1, err := Eval(context.Background(), "[1, 2, 3]")
	assert.Nil(t, err)
	assert.Equal(t, result1, []any{int64(1), int64(2), int64(3)})

	// With WithRawResult, we get the object.Object directly
	result2, err := Eval(context.Background(), "[1, 2, 3]", WithRawResult())
	assert.Nil(t, err)

	// Verify it's a *object.List
	list, ok := result2.(*object.List)
	assert.True(t, ok, "expected *object.List")
	assert.Equal(t, list.Len().Value(), int64(3))
}

// Test WithRawResult for types without Go equivalent
func TestWithRawResultClosure(t *testing.T) {
	// Closures normally return their Inspect() string
	result1, err := Eval(context.Background(), "function() { return 1 }")
	assert.Nil(t, err)
	_, isString := result1.(string)
	assert.True(t, isString, "expected string representation")

	// With WithRawResult, we get the Closure object
	result2, err := Eval(context.Background(), "function() { return 1 }", WithRawResult())
	assert.Nil(t, err)
	_, isClosure := result2.(*object.Closure)
	assert.True(t, isClosure, "expected *object.Closure")
}

// Test global name validation between compile and run
func TestGlobalNameValidation(t *testing.T) {
	// Compile with x, y, z
	code, err := Compile("x + y + z", WithEnv(map[string]any{
		"x": int64(1),
		"y": int64(2),
		"z": int64(3),
	}))
	assert.Nil(t, err)

	// Run with same keys - should succeed
	result, err := Run(context.Background(), code, WithEnv(map[string]any{
		"x": int64(10),
		"y": int64(20),
		"z": int64(30),
	}))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(60))

	// Run with missing key - should fail with clear error
	_, err = Run(context.Background(), code, WithEnv(map[string]any{
		"x": int64(1),
		"y": int64(2),
		// missing "z"
	}))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "missing required globals"))
	assert.True(t, strings.Contains(err.Error(), "z"))

	// Run with extra keys is allowed (only missing keys cause errors)
	result, err = Run(context.Background(), code, WithEnv(map[string]any{
		"x":     int64(1),
		"y":     int64(2),
		"z":     int64(3),
		"extra": int64(999),
	}))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(6))
}

