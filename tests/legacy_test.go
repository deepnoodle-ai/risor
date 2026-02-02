package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Legacy tests converted from .tm files

func TestLetVariableReturnsValue(t *testing.T) {
	// github issue: https://github.com/deepnoodle-ai/risor/issues/5
	result, err := eval(`
let a = 10
a
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "10")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestConstVariableReturnsValue(t *testing.T) {
	// github issue: https://github.com/deepnoodle-ai/risor/issues/5
	result, err := eval(`
const a = 10
a
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "10")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestListMapMethod(t *testing.T) {
	result, err := eval(`
let a = [
    "1",
    "22",
    "333",
]
a.map(function(x) { len(x) })
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[1, 2, 3]")
	assert.Equal(t, result.Type(), object.Type("list"))
}

func TestMathSqrt(t *testing.T) {
	result, err := eval(`
let x = math.sqrt(4.0)
x
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "2")
	assert.Equal(t, result.Type(), object.Type("float"))
}

func TestRawStringEscapes(t *testing.T) {
	// github issue: https://github.com/deepnoodle-ai/risor/issues/6
	// Raw strings preserve backslashes literally, so \n is two characters
	result, err := eval("`a\\ntest\\tstring\\\\`")
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `"a\\ntest\\tstring\\\\"`)
	assert.Equal(t, result.Type(), object.Type("string"))
}

func TestRawStringEquivalence(t *testing.T) {
	// github issue: https://github.com/deepnoodle-ai/risor/issues/6
	result, err := eval(`
let s = "\ntest\t\"str\\"

let raw = ` + "`" + `
test	"str\` + "`" + `

assert(s == raw)

len(s)
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "11")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestSingleQuoteString(t *testing.T) {
	result, err := eval(`
let a = 'abc'
a
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `"abc"`)
	assert.Equal(t, result.Type(), object.Type("string"))
}

func TestMissingAssignmentValue(t *testing.T) {
	_, err := eval(`
let x = 42
let y =
`)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "assignment is missing a value"))
}

func TestUnterminatedRawString(t *testing.T) {
	// github issue: https://github.com/deepnoodle-ai/risor/issues/6
	_, err := eval("let line = `hello there\n")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "unterminated string literal"))
}

func TestTemplateStringNonInterpolated(t *testing.T) {
	result, err := eval(`
let a = 10
let s = ` + "`foo {a+11} bar`" + `
s
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `"foo {a+11} bar"`)
	assert.Equal(t, result.Type(), object.Type("string"))
}

func TestTemplateStringInterpolated(t *testing.T) {
	result, err := eval(`
let a = 10
` + "`foo ${a+11} bar ${ \"ab\" + \"cd\" }`")
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `"foo 21 bar abcd"`)
	assert.Equal(t, result.Type(), object.Type("string"))
}

func TestTemplateStringWithQuotes(t *testing.T) {
	result, err := eval(`
let a = 10
` + "`foo ${a+11} bar \"ab\"`")
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `"foo 21 bar \"ab\""`)
	assert.Equal(t, result.Type(), object.Type("string"))
}

func TestTemplateStringWithFunction(t *testing.T) {
	result, err := eval(`
let inc = function(x) {
    x + 1
}
let i = 100
` + "`${inc(i)}`")
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `"101"`)
	assert.Equal(t, result.Type(), object.Type("string"))
}

func TestListMutationMethods(t *testing.T) {
	result, err := eval(`
let a = [1,2,3]
a.append(4)
assert(a[3] == 4)

a.clear()
assert(len(a) == 0)

a.extend([1,2])
assert(len(a) == 2)

a.extend([3,4])
assert(len(a) == 4)

a
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[1, 2, 3, 4]")
	assert.Equal(t, result.Type(), object.Type("list"))
}

func TestFunctionScopeIsolation(t *testing.T) {
	result, err := eval(`
function square(x) { x * x }

assert(square(2) == 4)

let x = 10

// This confirms the temporary scope for function execution, which also uses
// a variable named x doesn't update the outer scope's x variable.
x
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "10")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestMethodAsFirstClassValue(t *testing.T) {
	result, err := eval(`
let l = [1, 2, 3]

let funcs = [l.append]

funcs[0](4)
funcs[0](5)

l
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[1, 2, 3, 4, 5]")
	assert.Equal(t, result.Type(), object.Type("list"))
}

func TestFunctionWithDefaultParams(t *testing.T) {
	result, err := eval(`let f = function(a="one", b=3.4) { 99 }
f`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `func(a="one", b=3.4) { 99 }`)
	assert.Equal(t, result.Type(), object.Type("function"))
}

func TestNamedFunctionWithDefaultParams(t *testing.T) {
	result, err := eval(`function f(a, b=2) { "foo" }
f`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `func f(a, b=2) { "foo" }`)
	assert.Equal(t, result.Type(), object.Type("function"))
}

func TestBlankIdentifierBasic(t *testing.T) {
	result, err := eval(`
// Blank identifier in let discards values
let _ = "discarded"
let _ = [1, 2, 3]

// Regular assignment to _ discards values
_ = "also discarded"

// Return a value to verify the test
42
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestBlankIdentifierDestructure(t *testing.T) {
	result, err := eval(`
// Array destructuring - discard first element
let [_, second] = [1, 2]

// Array destructuring - discard middle element
let [first, _, third] = [10, 20, 30]

// Object destructuring - discard 'x' property
let {x: _, y} = {x: 100, y: 5}

// Verify values: second=2, first=10, third=30, y=5
second + first / 10 + third / 10 + y  // 2 + 1 + 3 + 5 = 11
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "11")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestBlankIdentifierFuncParam(t *testing.T) {
	result, err := eval(`
// Function that ignores first parameter
function ignoreFirst(_, b) {
    return b * 2
}

// Function that ignores second parameter
function ignoreSecond(a, _) {
    return a * 3
}

// Function that ignores multiple parameters
function ignoreMany(_, x, _, y, _) {
    return x + y
}

// Arrow function with blank identifier
let double = (_, n) => n * 2

// Verify: ignoreFirst(100, 5) = 10, ignoreSecond(4, 999) = 12
// ignoreMany(1, 2, 3, 4, 5) = 2 + 4 = 6, double(0, 1) = 2
ignoreFirst(100, 5) + ignoreSecond(4, 999) + ignoreMany(1, 2, 3, 4, 5) + double(0, 1)
// 10 + 12 + 6 + 2 = 30
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "30")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestBlankIdentifierMultiVar(t *testing.T) {
	result, err := eval(`
// Discard first value, keep second
let _, b = [1, 2]

// Discard second value, keep first
let c, _ = [10, 20]

// Discard both ends, keep middle
let _, middle, _ = [100, 200, 300]

// Verify values are correct
// b = 2, c = 10, middle = 200
b + (c / 10) + (middle / 100)  // 2 + 1 + 2 = 5
`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "5")
	assert.Equal(t, result.Type(), object.Type("int"))
}

func TestNotInOperator(t *testing.T) {
	// Basic "not in" with list - element present
	result, err := eval(`"a" not in ["a", "b", "c"]`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "false")
	assert.Equal(t, result.Type(), object.Type("bool"))
}

func TestNotInOperatorTrue(t *testing.T) {
	// "not in" with element not in list
	result, err := eval(`"d" not in ["a", "b", "c"]`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "true")
	assert.Equal(t, result.Type(), object.Type("bool"))
}

func TestNotInOperatorMap(t *testing.T) {
	// "not in" with map - addresses the original feature request
	result, err := eval(`"element" not in {"key": "value", "another": "test"}`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "true")
	assert.Equal(t, result.Type(), object.Type("bool"))
}

func TestNotInOperatorPrecedence(t *testing.T) {
	// "not in" with logical AND - should be: (2 not in [1,3]) && true
	result, err := eval(`2 not in [1, 3] && true`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "true")
	assert.Equal(t, result.Type(), object.Type("bool"))
}

func TestNotInEquivalence(t *testing.T) {
	// Both expressions should evaluate to the same result
	result, err := eval(`("element" not in {"key": "value"}) == !("element" in {"key": "value"})`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "true")
	assert.Equal(t, result.Type(), object.Type("bool"))
}

func TestNotInComprehensive(t *testing.T) {
	// Original feature request test
	result, err := eval(`"element" not in {"key1": "value1", "key2": "value2"}`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "true")
	assert.Equal(t, result.Type(), object.Type("bool"))
}

// Helper function to execute Risor code and return the result as object.Object
func eval(code string) (object.Object, error) {
	ctx := context.Background()
	compiled, err := risor.Compile(ctx, code, risor.WithEnv(risor.Builtins()))
	if err != nil {
		return nil, err
	}
	// Use vm.Run directly to get object.Object for accurate test comparison
	return vm.Run(ctx, compiled, vm.WithGlobals(risor.Builtins()))
}
