package risor

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

func TestBasicUsage(t *testing.T) {
	// By default, the environment is empty
	result, err := Eval(context.Background(), "1 + 1")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(2))
}

func TestBinaryLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0b0", 0},
		{"0b1", 1},
		{"0b10", 2},
		{"0b1010", 10},
		{"0b11111111", 255},
		{"0b1010 + 0b0101", 15}, // 10 + 5 = 15
	}
	for _, tt := range tests {
		result, err := Eval(context.Background(), tt.input)
		assert.Nil(t, err, "input: %s", tt.input)
		assert.Equal(t, result, tt.expected, "input: %s", tt.input)
	}
}

func TestXorOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0b1010 ^ 0b1100", 6},  // 10 ^ 12 = 6
		{"0b1111 ^ 0b0000", 15}, // 15 ^ 0 = 15
		{"0b1111 ^ 0b1111", 0},  // 15 ^ 15 = 0
		{"5 ^ 3", 6},            // 0101 ^ 0011 = 0110 = 6
		{"255 ^ 255", 0},
	}
	for _, tt := range tests {
		result, err := Eval(context.Background(), tt.input)
		assert.Nil(t, err, "input: %s", tt.input)
		assert.Equal(t, result, tt.expected, "input: %s", tt.input)
	}
}

func TestBytesBuiltin(t *testing.T) {
	ctx := context.Background()
	env := WithEnv(Builtins())

	t.Run("empty bytes", func(t *testing.T) {
		result, err := Eval(ctx, "bytes()", env)
		assert.Nil(t, err)
		// Empty bytes returns nil slice, check length is 0
		assert.Equal(t, result, []byte(nil))
	})

	t.Run("bytes from string", func(t *testing.T) {
		result, err := Eval(ctx, `bytes("hello")`, env)
		assert.Nil(t, err)
		assert.Equal(t, result, []byte("hello"))
	})

	t.Run("bytes from list", func(t *testing.T) {
		result, err := Eval(ctx, "bytes([72, 105])", env)
		assert.Nil(t, err)
		assert.Equal(t, result, []byte{72, 105}) // "Hi"
	})

	t.Run("bytes indexing", func(t *testing.T) {
		result, err := Eval(ctx, `bytes("abc")[0]`, env)
		assert.Nil(t, err)
		assert.Equal(t, result, byte(97)) // 'a'
	})

	t.Run("bytes len", func(t *testing.T) {
		result, err := Eval(ctx, `len(bytes("hello"))`, env)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(5))
	})
}

func TestMapShorthandSyntax(t *testing.T) {
	ctx := context.Background()

	t.Run("simple shorthand", func(t *testing.T) {
		result, err := Eval(ctx, `let a = 1; let b = 2; {a, b}`)
		assert.Nil(t, err)
		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, int64(1), m["a"])
		assert.Equal(t, int64(2), m["b"])
	})

	t.Run("mixed shorthand and explicit", func(t *testing.T) {
		result, err := Eval(ctx, `let x = 10; {x, y: 20, z: 30}`)
		assert.Nil(t, err)
		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, int64(10), m["x"])
		assert.Equal(t, int64(20), m["y"])
		assert.Equal(t, int64(30), m["z"])
	})

	t.Run("shorthand in function return", func(t *testing.T) {
		result, err := Eval(ctx, `
			function makePoint(x, y) { return {x, y} }
			makePoint(3, 4)
		`)
		assert.Nil(t, err)
		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, int64(3), m["x"])
		assert.Equal(t, int64(4), m["y"])
	})
}

func TestEmptyEnvByDefault(t *testing.T) {
	// Verify that the environment is empty by default (no builtins available)
	testCases := []struct {
		input       string
		expectedErr string
	}{
		{
			input:       "keys({foo: 1})",
			expectedErr: "compile error: undefined variable \"keys\"\n\nlocation: unknown:1:1",
		},
		{
			input:       "any([0, 0, 1])",
			expectedErr: "compile error: undefined variable \"any\"\n\nlocation: unknown:1:1",
		},
		{
			input:       "string(42)",
			expectedErr: "compile error: undefined variable \"string\"\n\nlocation: unknown:1:1",
		},
		{
			input:       "math.abs(-1)",
			expectedErr: "compile error: undefined variable \"math\"\n\nlocation: unknown:1:1",
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
	ctx := context.Background()
	program, err := Compile(ctx, "1 + 2")
	assert.Nil(t, err)
	assert.NotNil(t, program)

	result, err := Run(ctx, program)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(3))
}

// Test that the same Program can be run multiple times with different env
func TestProgramReuse(t *testing.T) {
	ctx := context.Background()
	program, err := Compile(ctx, "x + 1", WithEnv(map[string]any{"x": int64(0)}))
	assert.Nil(t, err)

	for i := int64(0); i < 10; i++ {
		result, err := Run(ctx, program, WithEnv(map[string]any{"x": i}))
		assert.Nil(t, err)
		assert.Equal(t, result, i+1)
	}
}

// Test concurrent execution of the same Program
func TestConcurrentExecution(t *testing.T) {
	ctx := context.Background()
	program, err := Compile(ctx, "x + 1", WithEnv(map[string]any{"x": int64(0)}))
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
	ctx := context.Background()
	// Compile with x, y, z
	code, err := Compile(ctx, "x + y + z", WithEnv(map[string]any{
		"x": int64(1),
		"y": int64(2),
		"z": int64(3),
	}))
	assert.Nil(t, err)

	// Run with same keys - should succeed
	result, err := Run(ctx, code, WithEnv(map[string]any{
		"x": int64(10),
		"y": int64(20),
		"z": int64(30),
	}))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(60))

	// Run with missing key - should fail with clear error
	_, err = Run(ctx, code, WithEnv(map[string]any{
		"x": int64(1),
		"y": int64(2),
		// missing "z"
	}))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "missing required globals"))
	assert.True(t, strings.Contains(err.Error(), "z"))

	// Run with extra keys is allowed (only missing keys cause errors)
	result, err = Run(ctx, code, WithEnv(map[string]any{
		"x":     int64(1),
		"y":     int64(2),
		"z":     int64(3),
		"extra": int64(999),
	}))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(6))
}

// Test resource limits via public API
func TestResourceLimits(t *testing.T) {
	ctx := context.Background()

	t.Run("step limit exceeded", func(t *testing.T) {
		// Use list().each() with range to iterate without deep recursion
		// Need enough iterations to exceed the step limit (checked every 1000 instructions)
		_, err := Eval(ctx, `let sum = 0; list(range(100000)).each(function(i) { sum = sum + i }); sum`,
			WithEnv(Builtins()),
			WithMaxSteps(5000))
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, ErrStepLimitExceeded)
	})

	t.Run("step limit not exceeded", func(t *testing.T) {
		result, err := Eval(ctx, `let sum = 0; list(range(10)).each(function(i) { sum = sum + i }); sum`,
			WithEnv(Builtins()),
			WithMaxSteps(100000))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(45))
	})

	t.Run("stack overflow", func(t *testing.T) {
		_, err := Eval(ctx, `function f() { f() }; f()`, WithMaxStackDepth(10))
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, ErrStackOverflow)
	})

	t.Run("timeout exceeded", func(t *testing.T) {
		// Use list().each() with range to iterate without deep recursion
		_, err := Eval(ctx, `let sum = 0; list(range(1000000)).each(function(i) { sum = sum + i }); sum`,
			WithEnv(Builtins()),
			WithTimeout(5*time.Millisecond))
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("compile cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately
		_, err := Compile(cancelCtx, `1 + 2`)
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// =============================================================================
// DESTRUCTURING PARAMETERS
// =============================================================================

func TestObjectDestructureParam(t *testing.T) {
	ctx := context.Background()

	t.Run("basic object destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({a, b}) { return a + b }
			foo({a: 1, b: 2})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(3), result)
	})

	t.Run("object destructure with default", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({x, y = 10}) { return x + y }
			foo({x: 5})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(15), result)
	})

	t.Run("object destructure with alias", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({name: n}) { return n }
			foo({name: "hello"})
		`)
		assert.Nil(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("mixed regular and destructure params", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo(multiplier, {a, b}) { return multiplier * (a + b) }
			foo(10, {a: 2, b: 3})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(50), result)
	})
}

func TestArrayDestructureParam(t *testing.T) {
	ctx := context.Background()

	t.Run("basic array destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo([a, b]) { return a + b }
			foo([1, 2])
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(3), result)
	})

	t.Run("array destructure with default", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo([x, y = 10]) { return x + y }
			foo([5])
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(15), result)
	})

	t.Run("array destructure third element", func(t *testing.T) {
		result, err := Eval(ctx, `
			function third([a, b, c]) { return c }
			third([1, 2, 3])
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(3), result)
	})
}

func TestArrowFunctionDestructureParam(t *testing.T) {
	ctx := context.Background()

	t.Run("arrow with array destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			let sum = ([a, b]) => a + b
			sum([3, 4])
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(7), result)
	})

	t.Run("arrow with object destructure shorthand", func(t *testing.T) {
		result, err := Eval(ctx, `
			let add = ({a, b}) => a + b
			add({a: 10, b: 20})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(30), result)
	})

	t.Run("arrow with object destructure alias", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({name: n, value: v}) => n + "=" + string(v)
			fn({name: "x", value: 42})
		`, WithEnv(Builtins()))
		assert.Nil(t, err)
		assert.Equal(t, "x=42", result)
	})

	t.Run("arrow with object destructure default", func(t *testing.T) {
		result, err := Eval(ctx, `
			let greet = ({name, greeting = "Hello"}) => greeting + ", " + name
			greet({name: "World"})
		`)
		assert.Nil(t, err)
		assert.Equal(t, "Hello, World", result)
	})

	t.Run("arrow with object destructure default used", func(t *testing.T) {
		result, err := Eval(ctx, `
			let calc = ({a = 1, b = 2}) => a + b
			calc({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(3), result)
	})

	t.Run("arrow with object destructure default overridden", func(t *testing.T) {
		result, err := Eval(ctx, `
			let calc = ({a = 1, b = 2}) => a + b
			calc({a: 10})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(12), result)
	})

	t.Run("arrow with mixed shorthand alias default", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({x, y: yVal, z = 100}) => x + yVal + z
			fn({x: 1, y: 2})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(103), result)
	})
}

// =============================================================================
// SHORTHAND MAP SYNTAX - COMPREHENSIVE TESTS
// =============================================================================

func TestMapShorthandComprehensive(t *testing.T) {
	ctx := context.Background()

	t.Run("shorthand preserves variable values", func(t *testing.T) {
		result, err := Eval(ctx, `
			let name = "Alice"
			let age = 30
			let city = "NYC"
			{name, age, city}
		`)
		assert.Nil(t, err)
		m := result.(map[string]any)
		assert.Equal(t, m["name"], "Alice")
		assert.Equal(t, m["age"], int64(30))
		assert.Equal(t, m["city"], "NYC")
	})

	t.Run("shorthand with computed values", func(t *testing.T) {
		result, err := Eval(ctx, `
			let x = 10
			let y = x * 2
			{x, y}
		`)
		assert.Nil(t, err)
		m := result.(map[string]any)
		assert.Equal(t, m["x"], int64(10))
		assert.Equal(t, m["y"], int64(20))
	})

	t.Run("shorthand in nested map", func(t *testing.T) {
		result, err := Eval(ctx, `
			let id = 1
			let name = "test"
			{outer: {id, name}}
		`)
		assert.Nil(t, err)
		m := result.(map[string]any)
		inner := m["outer"].(map[string]any)
		assert.Equal(t, inner["id"], int64(1))
		assert.Equal(t, inner["name"], "test")
	})

	t.Run("shorthand with spread", func(t *testing.T) {
		result, err := Eval(ctx, `
			let base = {a: 1, b: 2}
			let c = 3
			{...base, c}
		`)
		assert.Nil(t, err)
		m := result.(map[string]any)
		assert.Equal(t, m["a"], int64(1))
		assert.Equal(t, m["b"], int64(2))
		assert.Equal(t, m["c"], int64(3))
	})

	t.Run("shorthand in list of maps", func(t *testing.T) {
		result, err := Eval(ctx, `
			let makeItem = function(i) {
				let id = i
				let value = i * 10
				return {id, value}
			}
			[makeItem(0), makeItem(1), makeItem(2)]
		`)
		assert.Nil(t, err)
		list := result.([]any)
		assert.Len(t, list, 3)
		assert.Equal(t, list[0].(map[string]any)["id"], int64(0))
		assert.Equal(t, list[2].(map[string]any)["value"], int64(20))
	})

	t.Run("shorthand as function argument", func(t *testing.T) {
		result, err := Eval(ctx, `
			function process(obj) { return obj.x + obj.y }
			let x = 5
			let y = 10
			process({x, y})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(15))
	})

	t.Run("shorthand with string keys containing expressions", func(t *testing.T) {
		result, err := Eval(ctx, `
			let count = 42
			let total = 100
			{count, total, "ratio": count / total}
		`)
		assert.Nil(t, err)
		m := result.(map[string]any)
		assert.Equal(t, m["count"], int64(42))
		assert.Equal(t, m["total"], int64(100))
	})
}

func TestArrowDestructureEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("deeply nested access in destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({config}) => config.settings.value
			fn({config: {settings: {value: 42}}})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(42))
	})

	t.Run("destructure with method call on extracted value", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({text}) => text.to_upper()
			fn({text: "hello"})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, "HELLO")
	})

	t.Run("array destructure in arrow with expression", func(t *testing.T) {
		result, err := Eval(ctx, `
			let sum = ([a, b, c]) => a + b + c
			sum([1, 2, 3])
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(6))
	})

	t.Run("destructure with nil default", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({value = nil}) => value
			fn({})
		`)
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("destructure with string default", func(t *testing.T) {
		result, err := Eval(ctx, `
			let greet = ({name = "stranger"}) => "Hello, " + name
			greet({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, "Hello, stranger")
	})

	t.Run("multiple arrow functions with destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			let getX = ({x}) => x
			let getY = ({y}) => y
			let point = {x: 10, y: 20}
			getX(point) + getY(point)
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(30))
	})

	t.Run("arrow destructure in higher order function", func(t *testing.T) {
		result, err := Eval(ctx, `
			let points = [{x: 1, y: 2}, {x: 3, y: 4}, {x: 5, y: 6}]
			points.map(({x, y}) => x + y)
		`, WithEnv(Builtins()))
		assert.Nil(t, err)
		assert.Equal(t, result, []any{int64(3), int64(7), int64(11)})
	})

	t.Run("arrow destructure in filter", func(t *testing.T) {
		result, err := Eval(ctx, `
			let items = [{active: true, value: 1}, {active: false, value: 2}, {active: true, value: 3}]
			items.filter(({active}) => active).map(({value}) => value)
		`, WithEnv(Builtins()))
		assert.Nil(t, err)
		assert.Equal(t, result, []any{int64(1), int64(3)})
	})

	t.Run("arrow destructure with immediately invoked function", func(t *testing.T) {
		result, err := Eval(ctx, `
			(({a, b}) => a * b)({a: 6, b: 7})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(42))
	})
}

func TestDestructureDefaultExpressions(t *testing.T) {
	ctx := context.Background()

	t.Run("default with arithmetic expression", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({x = 1 + 2 + 3}) => x
			fn({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(6))
	})

	t.Run("default with function call", func(t *testing.T) {
		result, err := Eval(ctx, `
			function defaultValue() { return 42 }
			let fn = ({x = defaultValue()}) => x
			fn({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(42))
	})

	t.Run("default with list literal", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({items = [1, 2, 3]}) => items
			fn({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, []any{int64(1), int64(2), int64(3)})
	})

	t.Run("default with map literal", func(t *testing.T) {
		result, err := Eval(ctx, `
			let fn = ({config = {debug: true}}) => config.debug
			fn({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, true)
	})
}

// =============================================================================
// METHOD CHAINING ACROSS NEWLINES - INTEGRATION TESTS
// =============================================================================

func TestMethodChainingAcrossNewlinesIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("fluent list operations", func(t *testing.T) {
		result, err := Eval(ctx, `
			[1, 2, 3, 4, 5]
				.filter(x => x > 2)
				.map(x => x * 2)
		`, WithEnv(Builtins()))
		assert.Nil(t, err)
		assert.Equal(t, []any{int64(6), int64(8), int64(10)}, result)
	})

	t.Run("string method chain", func(t *testing.T) {
		result, err := Eval(ctx, `
			"hello world"
				.to_upper()
				.split(" ")
		`, WithEnv(Builtins()))
		assert.Nil(t, err)
		assert.Equal(t, []any{"HELLO", "WORLD"}, result)
	})

	t.Run("optional chaining across newlines", func(t *testing.T) {
		result, err := Eval(ctx, `
			let obj = {a: {b: {c: 42}}}
			obj
				?.a
				?.b
				?.c
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(42), result)
	})

	t.Run("optional chaining with nil", func(t *testing.T) {
		result, err := Eval(ctx, `
			let obj = {a: nil}
			obj
				?.a
				?.b
				?.c
		`)
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("mixed dot and optional chain", func(t *testing.T) {
		// Note: Array index [0] must be on same line as .items since [ is not
		// a chaining operator. Only . and ?. can follow newlines.
		result, err := Eval(ctx, `
			let obj = {a: {b: {c: 42}}}
			obj
				.a
				?.b
				.c
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(42), result)
	})
}

// =============================================================================
// DESTRUCTURING PARAMETERS - EDGE CASE INTEGRATION TESTS
// =============================================================================

func TestDestructureParamEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("nested object access", func(t *testing.T) {
		result, err := Eval(ctx, `
			function getX({point}) { return point.x }
			getX({point: {x: 10, y: 20}})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(10), result)
	})

	t.Run("default with expression", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({x = 1 + 2}) { return x }
			foo({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(3), result)
	})

	t.Run("all defaults used", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({a = 1, b = 2, c = 3}) { return a + b + c }
			foo({})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(6), result)
	})

	t.Run("some defaults used", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({a = 1, b = 2, c = 3}) { return a + b + c }
			foo({b: 10})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(14), result)
	})

	t.Run("alias with computation", func(t *testing.T) {
		result, err := Eval(ctx, `
			function doubled({value: v}) { return v * 2 }
			doubled({value: 21})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(42), result)
	})

	t.Run("array destructure order", func(t *testing.T) {
		result, err := Eval(ctx, `
			function order([a, b, c]) { return [c, b, a] }
			order([1, 2, 3])
		`)
		assert.Nil(t, err)
		assert.Equal(t, []any{int64(3), int64(2), int64(1)}, result)
	})

	t.Run("array destructure partial", func(t *testing.T) {
		// Note: Risor requires array destructure count to match array length.
		// Partial unpacking is not supported, so pass exactly one element.
		result, err := Eval(ctx, `
			function first([a]) { return a }
			first([1])
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(1), result)
	})

	t.Run("empty object destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({}) { return 42 }
			foo({a: 1, b: 2})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(42), result)
	})

	t.Run("empty array destructure", func(t *testing.T) {
		// Note: Empty array destructure must be passed an empty array
		// since Risor requires exact array length match.
		result, err := Eval(ctx, `
			function foo([]) { return 42 }
			foo([])
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(42), result)
	})

	t.Run("destructure in callback", func(t *testing.T) {
		result, err := Eval(ctx, `
			let items = [{x: 1}, {x: 2}, {x: 3}]
			let sum = 0
			items.each(function({x}) { sum = sum + x })
			sum
		`, WithEnv(Builtins()))
		assert.Nil(t, err)
		assert.Equal(t, int64(6), result)
	})

	t.Run("multiple destructure params", func(t *testing.T) {
		result, err := Eval(ctx, `
			function combine({a}, {b}, {c}) { return a + b + c }
			combine({a: 1}, {b: 2}, {c: 3})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(6), result)
	})

	t.Run("regular and destructure interleaved", func(t *testing.T) {
		result, err := Eval(ctx, `
			function calc(mult, {a, b}, divisor) { return (a + b) * mult / divisor }
			calc(2, {a: 3, b: 7}, 5)
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(4), result)
	})
}

func TestDestructureParamClosures(t *testing.T) {
	ctx := context.Background()

	t.Run("destructure captures variable", func(t *testing.T) {
		result, err := Eval(ctx, `
			function makeAdder({amount}) {
				return function(x) { return x + amount }
			}
			let addFive = makeAdder({amount: 5})
			addFive(10)
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(15), result)
	})

	t.Run("nested destructure closures", func(t *testing.T) {
		result, err := Eval(ctx, `
			function outer({a}) {
				return function({b}) {
					return a + b
				}
			}
			let inner = outer({a: 10})
			inner({b: 5})
		`)
		assert.Nil(t, err)
		assert.Equal(t, int64(15), result)
	})
}

func TestDestructureWithRestParam(t *testing.T) {
	ctx := context.Background()

	t.Run("destructure before rest", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({a}, ...rest) { return [a, rest] }
			foo({a: 1}, 2, 3, 4)
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, []any{int64(1), []any{int64(2), int64(3), int64(4)}})
	})
}

func TestMultilineDestructureParams(t *testing.T) {
	ctx := context.Background()

	t.Run("object destructure with newlines", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({
				a,
				b
			}) { return a + b }
			foo({a: 1, b: 2})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("array destructure with newlines", func(t *testing.T) {
		result, err := Eval(ctx, `
			function bar([
				x,
				y
			]) { return x * y }
			bar([3, 4])
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(12))
	})

	t.Run("mixed params with newlines", func(t *testing.T) {
		result, err := Eval(ctx, `
			function calc(
				multiplier,
				{a, b},
				[c, d],
				suffix
			) {
				return (a + b + c + d) * multiplier + suffix
			}
			calc(10, {a: 1, b: 2}, [3, 4], 5)
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(105)) // (1+2+3+4) * 10 + 5 = 105
	})

	t.Run("object destructure with defaults and newlines", func(t *testing.T) {
		result, err := Eval(ctx, `
			function greet({
				name,
				greeting = "Hello"
			}) {
				return greeting + ", " + name
			}
			greet({name: "World"})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, "Hello, World")
	})

	t.Run("trailing comma in object destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			function foo({
				a,
				b,
			}) { return a + b }
			foo({a: 10, b: 20})
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(30))
	})

	t.Run("trailing comma in array destructure", func(t *testing.T) {
		result, err := Eval(ctx, `
			function bar([
				x,
				y,
			]) { return x - y }
			bar([100, 30])
		`)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(70))
	})
}

// =============================================================================
// GO FUNCTIONS AND STRUCTS IN ENVIRONMENT
// =============================================================================

func TestGoFuncInEnv(t *testing.T) {
	ctx := context.Background()

	t.Run("simple Go function", func(t *testing.T) {
		env := map[string]any{
			"double": func(x int) int { return x * 2 },
		}
		result, err := Eval(ctx, "double(21)", WithEnv(env))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(42))
	})

	t.Run("sprintf-style function", func(t *testing.T) {
		env := map[string]any{
			"sprintf": func(format string, args ...any) string {
				return strings.ReplaceAll(format, "%s", args[0].(string))
			},
		}
		result, err := Eval(ctx, `sprintf("Hello, %s!", "world")`, WithEnv(env))
		assert.Nil(t, err)
		assert.Equal(t, result, "Hello, world!")
	})

	t.Run("function with context", func(t *testing.T) {
		type ctxKey string
		env := map[string]any{
			"getValue": func(ctx context.Context) string {
				if v := ctx.Value(ctxKey("test")); v != nil {
					return v.(string)
				}
				return "default"
			},
		}
		ctxWithValue := context.WithValue(ctx, ctxKey("test"), "from-context")
		result, err := Eval(ctxWithValue, "getValue()", WithEnv(env))
		assert.Nil(t, err)
		assert.Equal(t, result, "from-context")
	})

	t.Run("function returning error", func(t *testing.T) {
		env := map[string]any{
			"mayFail": func(shouldFail bool) (string, error) {
				if shouldFail {
					return "", errors.New("intentional error")
				}
				return "success", nil
			},
		}
		result, err := Eval(ctx, "mayFail(false)", WithEnv(env))
		assert.Nil(t, err)
		assert.Equal(t, result, "success")
	})
}

type TestTweet struct {
	Text string
	Len  int
}

func (t *TestTweet) Summary() string {
	return t.Text[:min(t.Len, len(t.Text))]
}

func TestGoStructInEnv(t *testing.T) {
	ctx := context.Background()

	t.Run("struct field access", func(t *testing.T) {
		env := map[string]any{
			"tweet": &TestTweet{Text: "Hello, world!", Len: 5},
		}
		result, err := Eval(ctx, "tweet.Len", WithEnv(env))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(5))
	})

	t.Run("struct method call", func(t *testing.T) {
		env := map[string]any{
			"tweet": &TestTweet{Text: "Hello, world!", Len: 5},
		}
		result, err := Eval(ctx, "tweet.Summary()", WithEnv(env))
		assert.Nil(t, err)
		assert.Equal(t, result, "Hello")
	})

	t.Run("slice of structs", func(t *testing.T) {
		tweets := []*TestTweet{
			{Text: "First", Len: 10},
			{Text: "Second", Len: 20},
		}
		env := map[string]any{
			"tweets": tweets,
		}
		result, err := Eval(ctx, "tweets[1].Len", WithEnv(env))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(20))
	})

	t.Run("struct field modification", func(t *testing.T) {
		tweet := &TestTweet{Text: "Original", Len: 10}
		env := map[string]any{
			"tweet": tweet,
		}
		_, err := Eval(ctx, "tweet.Len = 99", WithEnv(env))
		assert.Nil(t, err)
		// Verify the underlying struct was modified
		assert.Equal(t, tweet.Len, 99)
	})
}
