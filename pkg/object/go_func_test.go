package object

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

// =============================================================================
// BASIC FUNCTIONALITY TESTS
// =============================================================================

func TestGoFunc_SimpleFunction(t *testing.T) {
	fn := func(s string) string {
		return "Hello, " + s
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "greet", registry)

	assert.Equal(t, goFunc.Type(), GOFUNC)
	assert.Equal(t, goFunc.Name(), "greet")
	assert.True(t, goFunc.IsTruthy())

	result, err := goFunc.Call(context.Background(), NewString("World"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "Hello, World")
}

func TestGoFunc_MultipleArgs(t *testing.T) {
	fn := func(a, b int) int {
		return a + b
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "add", registry)

	result, err := goFunc.Call(context.Background(), NewInt(10), NewInt(20))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(30))
}

func TestGoFunc_NoArgs(t *testing.T) {
	fn := func() string {
		return "no args"
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "noArgs", registry)

	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "no args")
}

func TestGoFunc_NoReturnValue(t *testing.T) {
	called := false
	fn := func() {
		called = true
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "noop", registry)

	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)
	assert.True(t, called)
}

func TestGoFunc_MultipleReturnValues(t *testing.T) {
	fn := func(x int) (int, int) {
		return x * 2, x * 3
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "multiReturn", registry)

	result, err := goFunc.Call(context.Background(), NewInt(5))
	assert.Nil(t, err)

	list, ok := result.(*List)
	assert.True(t, ok)
	assert.Equal(t, list.Len().Value(), int64(2))

	items := list.Value()
	assert.Equal(t, items[0].(*Int).Value(), int64(10))
	assert.Equal(t, items[1].(*Int).Value(), int64(15))
}

func TestGoFunc_ThreeReturnValues(t *testing.T) {
	fn := func(x int) (int, string, bool) {
		return x, fmt.Sprintf("%d", x), x > 0
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "triple", registry)

	result, err := goFunc.Call(context.Background(), NewInt(42))
	assert.Nil(t, err)

	list, ok := result.(*List)
	assert.True(t, ok)
	assert.Equal(t, list.Len().Value(), int64(3))

	items := list.Value()
	assert.Equal(t, items[0].(*Int).Value(), int64(42))
	assert.Equal(t, items[1].(*String).Value(), "42")
	assert.Equal(t, items[2].(*Bool).Value(), true)
}

// =============================================================================
// VARIADIC FUNCTION TESTS
// =============================================================================

func TestGoFunc_VariadicFunction(t *testing.T) {
	fn := func(format string, args ...any) string {
		return fmt.Sprintf(format, args...)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sprintf", registry)

	result, err := goFunc.Call(context.Background(),
		NewString("Hello, %s! You are %d years old."),
		NewString("Alice"),
		NewInt(30))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "Hello, Alice! You are 30 years old.")
}

func TestGoFunc_VariadicNoExtraArgs(t *testing.T) {
	fn := func(format string, args ...any) string {
		return fmt.Sprintf(format, args...)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sprintf", registry)

	result, err := goFunc.Call(context.Background(), NewString("No args"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "No args")
}

func TestGoFunc_VariadicOnlyArgs(t *testing.T) {
	fn := func(nums ...int) int {
		sum := 0
		for _, n := range nums {
			sum += n
		}
		return sum
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sum", registry)

	// No args
	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(0))

	// Multiple args
	result, err = goFunc.Call(context.Background(), NewInt(1), NewInt(2), NewInt(3))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(6))
}

func TestGoFunc_VariadicManyArgs(t *testing.T) {
	fn := func(nums ...int) int {
		sum := 0
		for _, n := range nums {
			sum += n
		}
		return sum
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sum", registry)

	// 10 arguments
	args := make([]Object, 10)
	for i := range 10 {
		args[i] = NewInt(int64(i + 1))
	}

	result, err := goFunc.Call(context.Background(), args...)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(55)) // 1+2+...+10 = 55
}

func TestGoFunc_VariadicTypedSlice(t *testing.T) {
	fn := func(prefix string, strs ...string) string {
		return prefix + strings.Join(strs, ", ")
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "join", registry)

	result, err := goFunc.Call(context.Background(),
		NewString("Items: "),
		NewString("a"), NewString("b"), NewString("c"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "Items: a, b, c")
}

// =============================================================================
// CONTEXT INJECTION TESTS
// =============================================================================

func TestGoFunc_ContextInjection(t *testing.T) {
	type ctxKey string
	fn := func(ctx context.Context, x int) int {
		if v := ctx.Value(ctxKey("multiplier")); v != nil {
			return x * v.(int)
		}
		return x
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "multiply", registry)

	// Without context value
	result, err := goFunc.Call(context.Background(), NewInt(5))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(5))

	// With context value
	ctx := context.WithValue(context.Background(), ctxKey("multiplier"), 3)
	result, err = goFunc.Call(ctx, NewInt(5))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(15))
}

func TestGoFunc_ContextOnly(t *testing.T) {
	fn := func(ctx context.Context) string {
		if ctx == nil {
			return "nil context"
		}
		return "has context"
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "checkCtx", registry)

	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "has context")
}

func TestGoFunc_ContextWithVariadic(t *testing.T) {
	fn := func(ctx context.Context, prefix string, nums ...int) string {
		sum := 0
		for _, n := range nums {
			sum += n
		}
		return fmt.Sprintf("%s: %d", prefix, sum)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sumWithPrefix", registry)

	result, err := goFunc.Call(context.Background(),
		NewString("Total"),
		NewInt(1), NewInt(2), NewInt(3))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "Total: 6")
}

func TestGoFunc_ContextCancellation(t *testing.T) {
	fn := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "waitOrCancel", registry)

	// Cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := goFunc.Call(ctx)
	assert.NotNil(t, err)
	assert.Equal(t, err, context.Canceled)
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

func TestGoFunc_ErrorReturn(t *testing.T) {
	fn := func(x int) (int, error) {
		if x < 0 {
			return 0, errors.New("negative value not allowed")
		}
		return x * 2, nil
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "double", registry)

	// Successful call
	result, err := goFunc.Call(context.Background(), NewInt(5))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(10))

	// Error case
	result, err = goFunc.Call(context.Background(), NewInt(-1))
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "negative value not allowed")
}

func TestGoFunc_ErrorOnly(t *testing.T) {
	fn := func(shouldFail bool) error {
		if shouldFail {
			return errors.New("failed")
		}
		return nil
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "mayFail", registry)

	// Success
	result, err := goFunc.Call(context.Background(), False)
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)

	// Failure
	_, err = goFunc.Call(context.Background(), True)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "failed")
}

func TestGoFunc_MultipleReturnsWithError(t *testing.T) {
	fn := func(x int) (int, string, error) {
		if x < 0 {
			return 0, "", errors.New("negative")
		}
		return x, fmt.Sprintf("%d", x), nil
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "process", registry)

	// Success - returns list of non-error values
	result, err := goFunc.Call(context.Background(), NewInt(42))
	assert.Nil(t, err)

	list, ok := result.(*List)
	assert.True(t, ok)
	assert.Equal(t, list.Len().Value(), int64(2))

	// Error case
	_, err = goFunc.Call(context.Background(), NewInt(-1))
	assert.NotNil(t, err)
}

func TestGoFunc_PanicRecovery(t *testing.T) {
	fn := func(x int) int {
		if x == 0 {
			panic("division by zero")
		}
		return 10 / x
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "divide", registry)

	// Normal call
	result, err := goFunc.Call(context.Background(), NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(5))

	// Panic case
	result, err = goFunc.Call(context.Background(), NewInt(0))
	assert.NotNil(t, err)
	assert.True(t, result == nil)
	assert.Contains(t, err.Error(), "panic in divide")
	assert.Contains(t, err.Error(), "division by zero")
}

func TestGoFunc_PanicWithNonStringValue(t *testing.T) {
	fn := func() {
		panic(42) // Panic with int
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "panicInt", registry)

	_, err := goFunc.Call(context.Background())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "panic in panicInt")
	assert.Contains(t, err.Error(), "42")
}

// =============================================================================
// ARGUMENT VALIDATION TESTS
// =============================================================================

func TestGoFunc_WrongArgumentCount(t *testing.T) {
	fn := func(a, b int) int {
		return a + b
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "add", registry)

	// Too few arguments
	_, err := goFunc.Call(context.Background(), NewInt(1))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected 2 argument(s), got 1")

	// Too many arguments
	_, err = goFunc.Call(context.Background(), NewInt(1), NewInt(2), NewInt(3))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected 2 argument(s), got 3")
}

func TestGoFunc_VariadicMinArgs(t *testing.T) {
	fn := func(required string, optional ...int) string {
		return fmt.Sprintf("%s: %d items", required, len(optional))
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "format", registry)

	// Missing required arg
	_, err := goFunc.Call(context.Background())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected at least 1 argument(s), got 0")

	// With required arg only
	result, err := goFunc.Call(context.Background(), NewString("items"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "items: 0 items")
}

func TestGoFunc_TypeConversionError(t *testing.T) {
	fn := func(x int) int {
		return x * 2
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "double", registry)

	// Wrong type
	_, err := goFunc.Call(context.Background(), NewString("not a number"))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "argument 1")
}

// =============================================================================
// TYPE CONVERSION TESTS
// =============================================================================

func TestGoFunc_IntToFloat(t *testing.T) {
	fn := func(x float64) float64 {
		return x * 2.0
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "double", registry)

	// Int converts to float
	result, err := goFunc.Call(context.Background(), NewInt(5))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Float).Value(), 10.0)
}

func TestGoFunc_FloatToInt(t *testing.T) {
	fn := func(x int) int {
		return x * 2
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "double", registry)

	// Float converts to int (truncates)
	result, err := goFunc.Call(context.Background(), NewFloat(5.7))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(10)) // 5.7 -> 5, 5*2 = 10
}

func TestGoFunc_ByteConversion(t *testing.T) {
	fn := func(b byte) byte {
		return b + 1
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "incByte", registry)

	result, err := goFunc.Call(context.Background(), NewByte(65)) // 'A'
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(66)) // 'B'
}

func TestGoFunc_BoolConversion(t *testing.T) {
	fn := func(b bool) bool {
		return !b
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "not", registry)

	result, err := goFunc.Call(context.Background(), True)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).Value(), false)
}

func TestGoFunc_SliceParameter(t *testing.T) {
	fn := func(nums []int) int {
		sum := 0
		for _, n := range nums {
			sum += n
		}
		return sum
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sumSlice", registry)

	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	result, err := goFunc.Call(context.Background(), list)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(6))
}

func TestGoFunc_MapParameter(t *testing.T) {
	fn := func(m map[string]int) int {
		sum := 0
		for _, v := range m {
			sum += v
		}
		return sum
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sumMap", registry)

	m := NewMap(map[string]Object{
		"a": NewInt(1),
		"b": NewInt(2),
		"c": NewInt(3),
	})
	result, err := goFunc.Call(context.Background(), m)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(6))
}

func TestGoFunc_AnyParameter(t *testing.T) {
	fn := func(x any) string {
		return fmt.Sprintf("%T", x)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "typeOf", registry)

	result, err := goFunc.Call(context.Background(), NewInt(42))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "int64")

	result, err = goFunc.Call(context.Background(), NewString("hello"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "string")
}

func TestGoFunc_TimeParameter(t *testing.T) {
	fn := func(t time.Time) string {
		return t.Format("2006-01-02")
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "formatDate", registry)

	now := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	result, err := goFunc.Call(context.Background(), NewTime(now))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "2024-06-15")
}

func TestGoFunc_BytesParameter(t *testing.T) {
	fn := func(data []byte) int {
		return len(data)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "byteLen", registry)

	result, err := goFunc.Call(context.Background(), NewBytes([]byte("hello")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(5))
}

func TestGoFunc_StringToBytes(t *testing.T) {
	fn := func(data []byte) string {
		return string(data)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "bytesToStr", registry)

	// String should convert to []byte
	result, err := goFunc.Call(context.Background(), NewString("hello"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "hello")
}

// =============================================================================
// RETURN VALUE TESTS
// =============================================================================

func TestGoFunc_ReturnSlice(t *testing.T) {
	fn := func() []int {
		return []int{1, 2, 3}
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "getSlice", registry)

	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)

	list, ok := result.(*List)
	assert.True(t, ok)
	assert.Equal(t, list.Len().Value(), int64(3))
}

func TestGoFunc_ReturnMap(t *testing.T) {
	fn := func() map[string]int {
		return map[string]int{"a": 1, "b": 2}
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "getMap", registry)

	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)

	m, ok := result.(*Map)
	assert.True(t, ok)
	assert.Equal(t, m.Size(), 2)
}

func TestGoFunc_ReturnNil(t *testing.T) {
	fn := func() *int {
		return nil
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "getPtr", registry)

	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)
}

func TestGoFunc_ReturnTime(t *testing.T) {
	fn := func() time.Time {
		return time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "getTime", registry)

	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)

	tm, ok := result.(*Time)
	assert.True(t, ok)
	assert.Equal(t, tm.Value().Year(), 2024)
}

// =============================================================================
// UNICODE AND SPECIAL CHARACTERS
// =============================================================================

func TestGoFunc_UnicodeStrings(t *testing.T) {
	fn := func(s string) int {
		return len([]rune(s))
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "runeCount", registry)

	result, err := goFunc.Call(context.Background(), NewString("Hello, 世界! 🌍"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(12)) // H e l l o ,   世 界 !   🌍
}

func TestGoFunc_EmptyString(t *testing.T) {
	fn := func(s string) bool {
		return s == ""
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "isEmpty", registry)

	result, err := goFunc.Call(context.Background(), NewString(""))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).Value(), true)
}

// =============================================================================
// OBJECT INTERFACE TESTS
// =============================================================================

func TestGoFunc_Equals(t *testing.T) {
	fn := func() {}

	registry := DefaultRegistry()
	goFunc1 := NewGoFunc(reflect.ValueOf(fn), "fn", registry)
	goFunc2 := NewGoFunc(reflect.ValueOf(fn), "fn", registry)
	goFunc3 := NewGoFunc(reflect.ValueOf(func() {}), "fn", registry)

	assert.True(t, goFunc1.Equals(goFunc2))
	assert.False(t, goFunc1.Equals(goFunc3))
	assert.False(t, goFunc1.Equals(NewInt(1)))
}

func TestGoFunc_Inspect(t *testing.T) {
	fn := func() {}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "myFunc", registry)

	assert.Equal(t, goFunc.Inspect(), "go_func(myFunc)")
	assert.Equal(t, goFunc.String(), "go_func(myFunc)")
}

func TestGoFunc_Attrs(t *testing.T) {
	fn := func() {}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "fn", registry)

	// GoFunc has no attributes
	assert.Nil(t, goFunc.Attrs())

	_, ok := goFunc.GetAttr("anything")
	assert.False(t, ok)

	err := goFunc.SetAttr("anything", NewInt(1))
	assert.NotNil(t, err)
}

func TestGoFunc_RunOperation(t *testing.T) {
	fn := func() {}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "fn", registry)

	_, err := goFunc.RunOperation(1, NewInt(1))
	assert.NotNil(t, err)
}

func TestGoFunc_Interface(t *testing.T) {
	fn := func(x int) int { return x }

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "fn", registry)

	result := goFunc.Interface()
	assert.NotNil(t, result)

	// Should be able to call the original function
	originalFn, ok := result.(func(int) int)
	assert.True(t, ok)
	assert.Equal(t, originalFn(5), 5)
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestGoFunc_NilArgument(t *testing.T) {
	fn := func(s *string) string {
		if s == nil {
			return "nil"
		}
		return *s
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "deref", registry)

	// Nil object should convert to nil pointer
	result, err := goFunc.Call(context.Background(), Nil)
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "nil")
}

func TestGoFunc_PointerArgument(t *testing.T) {
	fn := func(x *int) {
		*x = *x * 2
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "doubleInPlace", registry)

	// Note: Risor int is immutable, so this creates a new pointer
	result, err := goFunc.Call(context.Background(), NewInt(5))
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)
}

func TestGoFunc_ManyArguments(t *testing.T) {
	fn := func(a, b, c, d, e, f, g, h, i, j int) int {
		return a + b + c + d + e + f + g + h + i + j
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sum10", registry)

	result, err := goFunc.Call(context.Background(),
		NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5),
		NewInt(6), NewInt(7), NewInt(8), NewInt(9), NewInt(10))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(55))
}

func TestGoFunc_ReturnFunc(t *testing.T) {
	fn := func(x int) func(int) int {
		return func(y int) int {
			return x + y
		}
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "makeAdder", registry)

	result, err := goFunc.Call(context.Background(), NewInt(10))
	assert.Nil(t, err)

	// Result should be a GoFunc wrapping the returned function
	innerFunc, ok := result.(*GoFunc)
	assert.True(t, ok)

	// Call the inner function
	result2, err := innerFunc.Call(context.Background(), NewInt(5))
	assert.Nil(t, err)
	assert.Equal(t, result2.(*Int).Value(), int64(15))
}

func TestGoFunc_NestedSlice(t *testing.T) {
	fn := func(matrix [][]int) int {
		sum := 0
		for _, row := range matrix {
			for _, val := range row {
				sum += val
			}
		}
		return sum
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "sumMatrix", registry)

	// Create a 2x3 matrix
	row1 := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	row2 := NewList([]Object{NewInt(4), NewInt(5), NewInt(6)})
	matrix := NewList([]Object{row1, row2})

	result, err := goFunc.Call(context.Background(), matrix)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(21))
}

func TestGoFunc_StructReturn(t *testing.T) {
	type Point struct {
		X, Y int
	}

	fn := func(x, y int) Point {
		return Point{X: x, Y: y}
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "makePoint", registry)

	result, err := goFunc.Call(context.Background(), NewInt(10), NewInt(20))
	assert.Nil(t, err)

	// Should return a GoStruct
	gs, ok := result.(*GoStruct)
	assert.True(t, ok)

	xVal, ok := gs.GetAttr("X")
	assert.True(t, ok)
	assert.Equal(t, xVal.(*Int).Value(), int64(10))
}

func TestGoFunc_InterfaceParameter(t *testing.T) {
	fn := func(s fmt.Stringer) string {
		if s == nil {
			return "nil"
		}
		return s.String()
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "stringify", registry)

	// Note: Passing Nil to an interface parameter causes a panic because
	// reflect.Call requires a valid Value. This is a known limitation.
	// Interface parameters work when you pass actual values that implement
	// the interface.
	_, err := goFunc.Call(context.Background(), Nil)
	// This returns an error due to the reflection limitation
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "panic")
}

func TestGoFunc_ZeroValues(t *testing.T) {
	fn := func(i int, f float64, s string, b bool) string {
		return fmt.Sprintf("%d,%f,%s,%t", i, f, s, b)
	}

	registry := DefaultRegistry()
	goFunc := NewGoFunc(reflect.ValueOf(fn), "format", registry)

	result, err := goFunc.Call(context.Background(),
		NewInt(0), NewFloat(0.0), NewString(""), False)
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "0,0.000000,,false")
}
