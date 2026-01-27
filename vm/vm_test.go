package vm

import (
	"context"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/bytecode"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
)

func TestAddCompilationAndExecution(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
	let x = 11
	let y = 12
	x + y
	`, nil)
	assert.Nil(t, err)

	main, err := compiler.Compile(program, nil)
	assert.Nil(t, err)

	constsCount := main.ConstantCount()
	assert.Equal(t, constsCount, 2)

	c1, ok := main.ConstantAt(0).(int64)
	assert.True(t, ok)
	assert.Equal(t, c1, int64(11))

	c2, ok := main.ConstantAt(1).(int64)
	assert.True(t, ok)
	assert.Equal(t, c2, int64(12))

	vm, err := New(main)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))

	tos, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, tos, object.NewInt(23))
}

func TestConditional(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
	let x = 20
	if (x > 10) {
		x = 99
	}
	x
	`, nil)
	assert.Nil(t, err)

	main, err := compiler.Compile(program, nil)
	assert.Nil(t, err)

	vm, err := New(main)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))

	tos, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, tos, object.NewInt(99))
}

func TestConditional3(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 5
	let y = 10
	if (x > 1) {
		y
	} else {
		99
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(10))
}

func TestConditional4(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 5
	let y = 22
	let z = 33
	if (x < 1) {
		x = y
	} else {
		x = z
	}
	x
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(33))
}

func TestIndexing(t *testing.T) {
	tests := []testCase{
		{`let x = [1, 2]; x[0] = 9; x[0]`, object.NewInt(9)},
		{`let x = [1, 2]; x[-1] = 9; x[1]`, object.NewInt(9)},
		{`let x = {a: 1}; x["a"] = 9; x["a"]`, object.NewInt(9)},
		{`let x = {a: 1}; x["b"] = 9; x["b"]`, object.NewInt(9)},
	}
	runTests(t, tests)
}

func TestStackBehavior3(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 77
	if (x > 0) {
		99
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(99))
}

func TestStackBehavior4(t *testing.T) {
	result, err := run(context.Background(), `
	let x = -1
	if (x > 0) {
		99
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestAssignmentOperators(t *testing.T) {
	result, err := run(context.Background(), `
	let y = 99
	y  = 3
	y += 6
	y /= 9
	y *= 2
	y
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(2))
}

func TestFunctionCall(t *testing.T) {
	result, err := run(context.Background(), `
	let f = function(x) { 42 + x }
	let v = f(1)
	v + 10
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(53))
}

func TestSwitch1(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch (x) {
		case 1:
		case 2:
			21
		case 3:
			42
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(42))
}

func TestSwitch2(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 1
	switch (x) {
		case 1:
			99
		case 2:
			42
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(99))
}

func TestSwitch3(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch (x) {
		case 1:
			99
		case 2:
			42
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestSwitch4(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch (x) { default: 99 }
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(99))
}

func TestSwitch5(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch (x) { default: 99 case 3: x; x-1 }
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(2))
}

func TestStr(t *testing.T) {
	result, err := run(context.Background(), `
	let s = "hello"
	s
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("hello"))
}

func TestStrLen(t *testing.T) {
	result, err := run(context.Background(), `
	let s = "hello"
	len(s)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(5))
}

func TestList1(t *testing.T) {
	result, err := run(context.Background(), `
	let l = [1, 2, 3]
	l
	`)
	assert.Nil(t, err)
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewInt(1),
			object.NewInt(2),
			object.NewInt(3),
		}))
}

func TestList2(t *testing.T) {
	result, err := run(context.Background(), `
	let plusOne = function(x) { x + 1 }
	[plusOne(0), 4-2, plusOne(2)]
	`)
	assert.Nil(t, err)
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewInt(1),
			object.NewInt(2),
			object.NewInt(3),
		}))
}

func TestMap(t *testing.T) {
	result, err := run(context.Background(), `
	{"a": 1, "b": 4-2}
	`)
	assert.Nil(t, err)
	assert.Equal(t,

		result, object.NewMap(map[string]object.Object{
			"a": object.NewInt(1),
			"b": object.NewInt(2),
		}))
}

func TestNonLocal(t *testing.T) {
	result, err := run(context.Background(), `
	let y = 3
	let z = 99
	let f = function() { y = 4 }
	f()
	y
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(4))
}

func TestFrameLocals1(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 1
	let f = function(x) { x = 99 }
	f(4)
	x
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(1))
}

func TestFrameLocals2(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 1
	let f = function(y) { x = 99 }
	f(4)
	x
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(99))
}

func TestMapKeys(t *testing.T) {
	result, err := run(context.Background(), `
	let m = {"a": 1, "b": 2}
	keys(m)
	`)
	assert.Nil(t, err)
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewString("a"),
			object.NewString("b"),
		}))
}

func TestClosure(t *testing.T) {
	result, err := run(context.Background(), `
	let f = function(x) { function() { x } }
	let closure = f(22)
	closure()
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(22))
}

func TestClosureIncrementer(t *testing.T) {
	result, err := run(context.Background(), `
	let f = function(x) {
		function() { x++; x }
	}
	let incrementer = f(0)
	incrementer() // 1
	incrementer() // 2
	incrementer() // 3
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(3))
}

func TestClosureOverLocal(t *testing.T) {
	result, err := run(context.Background(), `
	let testValue = 100
	function getint() {
		let foo = testValue + 1
		function inner() {
			foo
		}
		return inner
	}
	getint()()
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(101))
}

func TestClosureManyVariables(t *testing.T) {
	result, err := run(context.Background(), `
	function foo(a, b, c) {
		return function(d) {
			return [a, b, c, d]
		}
	}
	foo("hello", "world", "risor")("go")
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewStringList([]string{"hello", "world", "risor", "go"}))
}

func TestRecursiveExample1(t *testing.T) {
	result, err := run(context.Background(), `
	function twoexp(n) {
		if (n == 0) {
			return 1
		} else {
			return 2 * twoexp(n-1)
		}
	}
	twoexp(4)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(16))
}

func TestRecursiveExample2(t *testing.T) {
	result, err := run(context.Background(), `
	function twoexp(n) {
		let a = 1
		let b = 2
		let c = a * b
		if (n == 0) {
			return 1
		} else {
			return c * twoexp(n-1)
		}
	}
	twoexp(4)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(16))
}

func TestConstant(t *testing.T) {
	_, err := run(context.Background(), `const x = 1; x = 2`)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "compile error: cannot assign to constant \"x\"\n\nlocation: unknown:1:14 (line 1, column 14)")
}

func TestConstantFunction(t *testing.T) {
	_, err := run(context.Background(), `
	function add(x, y) { x + y }
	add = "bloop"
	`)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "compile error: cannot assign to constant \"add\"\n\nlocation: unknown:3:2 (line 3, column 2)")
}

func TestStatementsNilValue(t *testing.T) {
	// The result value of a statement is always nil
	tests := []testCase{
		{`let x = 0`, object.Nil},
		{`let x = 0; x++`, object.Nil},
		{`let x = 0; x--`, object.Nil},
		{`let x = 0; x += 1`, object.Nil},
		{`let x = 0; x -= 1`, object.Nil},
		{`const x = 0`, object.Nil},
		{`let x = 0`, object.Nil},
		{`let x, y = [0, 0]`, object.Nil},
		{`let x = [1]; x[0] = 2`, object.Nil},
	}
	runTests(t, tests)
}

func TestPostfixOperators(t *testing.T) {
	tests := []testCase{
		// Simple variable postfix
		{`let x = 5; x++; x`, object.NewInt(6)},
		{`let x = 5; x--; x`, object.NewInt(4)},
		// Index postfix with literal index
		{`let arr = [1, 2, 3]; arr[0]++; arr[0]`, object.NewInt(2)},
		{`let arr = [10, 20, 30]; arr[1]--; arr[1]`, object.NewInt(19)},
		{`let arr = [1, 2, 3]; arr[0]++; arr[1]++; arr[2]++; arr`, object.NewList([]object.Object{
			object.NewInt(2), object.NewInt(3), object.NewInt(4),
		})},
		// Index postfix with variable index
		{`let arr = [1, 2, 3]; let i = 1; arr[i]++; arr[i]`, object.NewInt(3)},
		{`let arr = [10, 20, 30]; let i = 2; arr[i]--; arr`, object.NewList([]object.Object{
			object.NewInt(10), object.NewInt(20), object.NewInt(29),
		})},
		// Index postfix with expression index
		{`let arr = [1, 2, 3, 4]; arr[1 + 1]++; arr[2]`, object.NewInt(4)},
		// Map index postfix
		{`let m = {"a": 1}; m["a"]++; m["a"]`, object.NewInt(2)},
		{`let m = {"x": 10}; m["x"]--; m["x"]`, object.NewInt(9)},
		// Map with variable key
		{`let m = {"key": 5}; let k = "key"; m[k]++; m["key"]`, object.NewInt(6)},
		// Attribute postfix
		{`let obj = {x: 5}; obj.x++; obj.x`, object.NewInt(6)},
		{`let obj = {value: 10}; obj.value--; obj.value`, object.NewInt(9)},
		// Nested attribute postfix
		{`let obj = {inner: {count: 0}}; obj.inner.count++; obj.inner.count`, object.NewInt(1)},
		// Multiple operations
		{`let arr = [0]; arr[0]++; arr[0]++; arr[0]++; arr[0]`, object.NewInt(3)},
		{`let obj = {n: 0}; obj.n++; obj.n++; obj.n`, object.NewInt(2)},
		// Multiple postfix operations on different indices
		{`let arr = [0, 0, 0]; arr[0]++; arr[1]++; arr[2]++; arr`, object.NewList([]object.Object{
			object.NewInt(1), object.NewInt(1), object.NewInt(1),
		})},
		// Postfix with float (should work with numeric addition)
		{`let obj = {f: 1.5}; obj.f++; obj.f`, object.NewFloat(2.5)},
	}
	runTests(t, tests)
}

func TestArithmetic(t *testing.T) {
	tests := []testCase{
		{`1 + 2`, object.NewInt(3)},
		{`1 + 2 + 3`, object.NewInt(6)},
		{`1 + 2 * 3`, object.NewInt(7)},
		{`(1 + 2) * 3`, object.NewInt(9)},
		{`5 - 3`, object.NewInt(2)},
		{`12 / 4`, object.NewInt(3)},
		{`3 * (4 + 2)`, object.NewInt(18)},
		{`1.5 + 1.5`, object.NewFloat(3.0)},
		{`1.5 + 2`, object.NewFloat(3.5)},
		{`2 + 1.5`, object.NewFloat(3.5)},
		{`2 ** 3`, object.NewInt(8)},
		{`2.0 ** 3.0`, object.NewFloat(8.0)},
		{`1 % 3`, object.NewInt(1)},
		{`3 % 3`, object.NewInt(0)},
		{`11 % 3`, object.NewInt(2)},
		{`-11`, object.NewInt(-11)},
		{`let x = -11; -x`, object.NewInt(11)},
		{`-1.5`, object.NewFloat(-1.5)},
		{`3 & 1`, object.NewInt(1)},
		{`3 & 3`, object.NewInt(3)},
	}
	runTests(t, tests)
}

func TestNumericComparisons(t *testing.T) {
	tests := []testCase{
		// Integers
		{`3 < 5`, object.True},
		{`3 <= 5`, object.True},
		{`3 > 5`, object.False},
		{`3 >= 5`, object.False},
		{`3 == 5`, object.False},
		{`3 != 5`, object.True},
		{`2 < 2`, object.False},
		{`2 <= 2`, object.True},
		{`2 > 2`, object.False},
		{`2 >= 2`, object.True},
		{`2 == 2`, object.True},
		{`2 != 2`, object.False},
		// Mixed integers and floats
		{`3.0 < 5`, object.True},
		{`3.0 <= 5`, object.True},
		{`3.0 > 5`, object.False},
		{`3.0 >= 5`, object.False},
		{`3.0 == 5`, object.False},
		{`3.0 != 5`, object.True},
		{`2.0 < 2`, object.False},
		{`2.0 <= 2`, object.True},
		{`2.0 > 2`, object.False},
		{`2.0 >= 2`, object.True},
		{`2.0 == 2`, object.True},
		{`2.0 != 2`, object.False},
		// Floats
		{`3.0 < 5.0`, object.True},
		{`3.0 <= 5.0`, object.True},
		{`3.0 > 5.0`, object.False},
		{`3.0 >= 5.0`, object.False},
		{`3.0 == 5.0`, object.False},
		{`3.0 != 5.0`, object.True},
		{`2.0 < 2.0`, object.False},
		{`2.0 <= 2.0`, object.True},
		{`2.0 > 2.0`, object.False},
		{`2.0 >= 2.0`, object.True},
		{`2.0 == 2.0`, object.True},
		{`2.0 != 2.0`, object.False},
	}
	runTests(t, tests)
}

func TestBooleans(t *testing.T) {
	tests := []testCase{
		{`true`, object.True},
		{`false`, object.False},
		{`!true`, object.False},
		{`!false`, object.True},
		{`!!true`, object.True},
		{`!!false`, object.False},
		{`false == false`, object.True},
		{`false == true`, object.False},
		{`false != false`, object.False},
		{`false != true`, object.True},
		{`true == true`, object.True},
		{`true == false`, object.False},
		{`true != true`, object.False},
		{`true != false`, object.True},
		{`type(true)`, object.NewString("bool")},
		{`type(false)`, object.NewString("bool")},
	}
	runTests(t, tests)
}

func TestTruthiness(t *testing.T) {
	tests := []testCase{
		{`!0`, object.True},
		{`!5`, object.False},
		{`![]`, object.True},
		{`![1]`, object.False},
		{`!{}`, object.True},
		{`!""`, object.True},
		{`!"a"`, object.False},
		{`bool(0)`, object.False},
		{`bool(5)`, object.True},
		{`bool([])`, object.False},
		{`bool([1])`, object.True},
		{`bool({})`, object.False},
		{`bool({foo: 1})`, object.True},
	}
	runTests(t, tests)
}

func TestControlFlow(t *testing.T) {
	tests := []testCase{
		{`if (false) { 3 }`, object.Nil},
		{`if (true) { 3 }`, object.NewInt(3)},
		{`if (false) { 3 } else { 4 }`, object.NewInt(4)},
		{`if (true) { 3 } else { 4 }`, object.NewInt(3)},
		{`if (false) { 3 } else if (false) { 4 } else { 5 }`, object.NewInt(5)},
		{`if (true) { 3 } else if (false) { 4 } else { 5 }`, object.NewInt(3)},
		{`if (false) { 3 } else if (true) { 4 } else { 5 }`, object.NewInt(4)},
		{`let x = 1; if (x > 5) { 99 } else { 100 }`, object.NewInt(100)},
		{`let x = 1; if (x > 0) { 99 } else { 100 }`, object.NewInt(99)},
		{`let x = 1; let y = if (x > 0) { 77 } else { 88 }; y`, object.NewInt(77)},
		{`let x = if (1 > 2) { 77 } else { 88 }; x`, object.NewInt(88)},
		{`let x = if (2 > 1) { 77 } else { 88 }; x`, object.NewInt(77)},
		{`let x = 1; switch (x) { case 1: 99; case 2: 100 }`, object.NewInt(99)},
		{`switch (2) { case 1: 99; case 2: 100 }`, object.NewInt(100)},
		{`switch (3) { case 1: 99; default: 42 case 2: 100 }`, object.NewInt(42)},
		{`switch (3) { case 1: 99; case 2: 100 }`, object.Nil},
		{`switch (3) { case 1, 3: 99; case 2: 100 }`, object.NewInt(99)},
		{`switch (3) { case 1: 99; case 2, 4-1: 100 }`, object.NewInt(100)},
		{`let x = 3; switch (bool(x)) { case true: "wow" }`, object.NewString("wow")},
		{`let x = 0; switch (bool(x)) { case true: "wow" }`, object.Nil},
	}
	runTests(t, tests)
}

func TestLength(t *testing.T) {
	tests := []testCase{
		{`len("")`, object.NewInt(0)},
		{`len([])`, object.NewInt(0)},
		{`len({})`, object.NewInt(0)},
		{`len("hello")`, object.NewInt(5)},
		{`len([1, 2, 3])`, object.NewInt(3)},
		{`len({"abc": 1})`, object.NewInt(1)},
		{`len("ᛛᛥ")`, object.NewInt(2)},
	}
	runTests(t, tests)
}

func TestBuiltins(t *testing.T) {
	tests := []testCase{
		{`len("hello")`, object.NewInt(5)},
		{`keys({"a": 1})`, object.NewList([]object.Object{
			object.NewString("a"),
		})},
		{`type(3.14159)`, object.NewString("float")},
		{`type("hi".contains)`, object.NewString("builtin")},
		{`sprintf("%d-%d", 1, 2)`, object.NewString("1-2")},
		{`int("99")`, object.NewInt(99)},
		{`float("2.5")`, object.NewFloat(2.5)},
		{`string(99)`, object.NewString("99")},
		{`string(2.5)`, object.NewString("2.5")},
		{`encode("hi", "hex")`, object.NewString("6869")},
		{`encode("hi", "base64")`, object.NewString("aGk=")},
		{`reversed("abc")`, object.NewString("cba")},
		{`reversed([1, 2, 3])`, object.NewList([]object.Object{
			object.NewInt(3),
			object.NewInt(2),
			object.NewInt(1),
		})},
		{`sorted([3, -2, 2])`, object.NewList([]object.Object{
			object.NewInt(-2),
			object.NewInt(2),
			object.NewInt(3),
		})},
		{`any([])`, object.False},
		{`any([0, false, {}])`, object.False},
		{`any([0, false, {foo: 42}])`, object.True},
		{`all([])`, object.True},
		{`all([1, false, {foo: 42}])`, object.False},
		{`all([1, true, {foo: 42}])`, object.True},
	}
	runTests(t, tests)
}

func TestTryKeyword(t *testing.T) {
	tests := []testCase{
		// Basic try/catch
		{`let r = "ok"; try { r = "try" } catch e { r = "catch" }; r`, object.NewString("try")},
		{`let r = "ok"; try { throw "err" } catch e { r = "catch" }; r`, object.NewString("catch")},
		// Try with no error
		{`let x = 1; try { x = 2 } catch e { x = 3 }; x`, object.NewInt(2)},
		// Try with error
		{`let x = 1; try { throw "oops"; x = 2 } catch e { x = 3 }; x`, object.NewInt(3)},
	}
	runTests(t, tests)
}

func TestTryEvalError(t *testing.T) {
	code := `
	try { throw "oops" } catch e { 1 }
	`
	_, err := run(context.Background(), code)
	// With the new try/catch, thrown errors are caught
	assert.Nil(t, err)
}

func TestTryTypeError(t *testing.T) {
	code := `
	let i = 0
	let msg = ""
	try { i.append("x") } catch e { msg = string(e) }
	msg
	`
	result, err := run(context.Background(), code)
	assert.NoError(t, err)
	// Check that the error message contains the expected content (may include location info)
	resultStr, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Contains(t, resultStr.Value(), "type error: attribute \"append\" not found on int object")
}

func TestTryUnsupportedOperation(t *testing.T) {
	code := `
	let i = []
	let msg = ""
	try { i + 3 } catch e { msg = string(e) }
	msg
	`
	result, err := run(context.Background(), code)
	assert.NoError(t, err)
	// Check that the error message contains the expected content (may include location info)
	resultStr, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Contains(t, resultStr.Value(), "type error: unsupported operation for list: + on type int")
}

func TestTryWithErrorValues(t *testing.T) {
	code := `
	const myerr = "errno == 1"
	let result = ""
	try {
		let x = "testing 1 2 3"
		throw myerr
	} catch e {
		result = if (string(e) == "errno == 1") { "YES" } else { "NO" }
	}
	result`
	result, err := run(context.Background(), code)
	assert.NoError(t, err)
	assert.Equal(t, result, object.NewString("YES"))
}

func TestStringTemplateWithThrow(t *testing.T) {
	// throw triggers exception handling - the template is never completed
	code := `
	function raise(msg) { throw msg }
	` + "`the err string is: ${raise(\"oops\")}. sad!`"
	_, err := run(context.Background(), code)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "oops")
}

func TestStringTemplateWithErrorValue(t *testing.T) {
	// Error values (not thrown) are stringified in templates
	code := `
	let err = error("something went wrong")
	` + "`the error is: ${err}`"
	result, err := run(context.Background(), code)
	assert.NoError(t, err)
	assert.Equal(t, result, object.NewString("the error is: something went wrong"))
}

func TestStringTemplateWithValue(t *testing.T) {
	code := "`the message is: ${\"oops\"}. sad!`"
	result, err := run(context.Background(), code)
	assert.NoError(t, err)
	assert.Equal(t, result, object.NewString("the message is: oops. sad!"))
}

func TestMultiVarAssignment(t *testing.T) {
	tests := []testCase{
		{`let a, b = [3, 4]; a`, object.NewInt(3)},
		{`let a, b = [3, 4]; b`, object.NewInt(4)},
		{`let a, b, c = [3, 4, 5]; a`, object.NewInt(3)},
		{`let a, b, c = [3, 4, 5]; b`, object.NewInt(4)},
		{`let a, b, c = [3, 4, 5]; c`, object.NewInt(5)},
		{`let a, b = "ᛛᛥ"; a`, object.NewString("ᛛ")},
		{`let a, b = "ᛛᛥ"; b`, object.NewString("ᛥ")},
		{`let a, b = {foo: 1, bar: 2}; a`, object.NewString("bar")},
		{`let a, b = {foo: 1, bar: 2}; b`, object.NewString("foo")},
	}
	runTests(t, tests)
}

func TestFunctions(t *testing.T) {
	tests := []testCase{
		{`function add(x, y) { x + y }; add(3, 4)`, object.NewInt(7)},
		{`function add(x, y) { x + y }; add(3, 4) + 5`, object.NewInt(12)},
		{`function inc(x, amount=1) { x + amount }; inc(3)`, object.NewInt(4)},
		{`function factorial(n) { if (n == 1) { return 1 } else { return n * factorial(n - 1) } }; factorial(5)`, object.NewInt(120)},
		{`let z = 10; let y = function(x, inc=100) { x + z + inc }; y(3)`, object.NewInt(113)},
		{`function(x="a", y="b") { x + y }()`, object.NewString("ab")},
		{`function(x="a", y="b") { x + y + "c" }()`, object.NewString("abc")},
		{`function(x="a", y="b") { x + y + "c" }("W")`, object.NewString("Wbc")},
		{`function(x="a", y="b") { x + y + "c" }("W", "X")`, object.NewString("WXc")},
		{`function(x="a", y="b") { return "X"; x + y + "c" }()`, object.NewString("X")},
		{`let x = 1; function() { let y = 10; x + y }()`, object.NewInt(11)},
		{`let x = 1; function() { function() { let y = 10; x + y } }()()`, object.NewInt(11)},
	}
	runTests(t, tests)
}

func TestContainers(t *testing.T) {
	tests := []testCase{
		{`true`, object.True},
		{`[1,2,3][2]`, object.NewInt(3)},
		{`"hello"[1]`, object.NewString("e")},
		{`{"x": 10, "y": 20}["x"]`, object.NewInt(10)},
		{`3 in [1, 2, 3]`, object.True},
		{`4 in [1, 2, 3]`, object.False},
		{`{"foo": "bar"}["foo"]`, object.NewString("bar")},
		{`{foo: "bar"}["foo"]`, object.NewString("bar")},
		{`[1, 2, 3, 4, 5].filter(function(x) { x > 3 })`, object.NewList(
			[]object.Object{object.NewInt(4), object.NewInt(5)})},
	}
	runTests(t, tests)
}

func TestStrings(t *testing.T) {
	tests := []testCase{
		{`"hello" + " " + "world"`, object.NewString("hello world")},
		{`"hello".contains("e")`, object.True},
		{`"hello".contains("x")`, object.False},
		{`"hello".contains("ello")`, object.True},
		{`"hello".contains("ellx")`, object.False},
		{`"hello".contains("")`, object.True},
		{`"hello"[0]`, object.NewString("h")},
		{`"hello"[1]`, object.NewString("e")},
		{`"hello"[-1]`, object.NewString("o")},
		{`"hello"[-2]`, object.NewString("l")},
		{"let a = 1; let b = \"ok\"; `${a + 1}-${b.to_upper()}`", object.NewString("2-OK")},
		{"function(a, b) { return `A: ${a} B: ${b}` }(\"hi\", \"bye\")", object.NewString("A: hi B: bye")},
	}
	runTests(t, tests)
}

func TestPipes(t *testing.T) {
	tests := []testCase{
		{`"hello".to_upper()`, object.NewString("HELLO")},
		{`"hello" | len`, object.NewInt(5)},
		{`function() { "hello" }() | len`, object.NewInt(5)},
		{`",".join(["a", "b"]).to_upper()`, object.NewString("A,B")},
		{`function() { "a" } | call`, object.NewString("a")},
		{`"abc" | getattr("to_upper") | call`, object.NewString("ABC")},
		{`"abc" | function(s) { s.to_upper() }`, object.NewString("ABC")},
		{`[11, 12, 3] | math.sum`, object.NewFloat(26)},
	}
	runTests(t, tests)
}

func TestQuicksort(t *testing.T) {
	result, err := run(context.Background(), `
	function quicksort(arr) {
		if (len(arr) < 2) {
			return arr
		} else {
			let pivot = arr[0]
			let less = arr[1:].filter(function(x) { x <= pivot })
			let more = arr[1:].filter(function(x) { x > pivot })
			return quicksort(less) + [pivot] + quicksort(more)
		}
	}
	quicksort([10, 5, 2, 3])
	`)
	assert.Nil(t, err)
	assert.Equal(t,

		result, object.NewList(
			[]object.Object{
				object.NewInt(2),
				object.NewInt(3),
				object.NewInt(5),
				object.NewInt(10),
			}))
}

func TestAndShortCircuit(t *testing.T) {
	// AND should short-circuit, so data[5] should not be evaluated
	result, err := run(context.Background(), `
	let data = []
	if (len(data) && data[5]) {
		"nope!"
	} else {
		"worked!"
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("worked!"))
}

func TestOrShortCircuit(t *testing.T) {
	// OR should short-circuit, so data[5] should not be evaluated
	result, err := run(context.Background(), `
	let data = [1]
	if (len(data) || data[5]) {
		"worked!"
	} else {
		"nope!"
	}
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("worked!"))
}

func TestNullishCoalescing(t *testing.T) {
	tests := []testCase{
		// Basic nil case
		{`nil ?? "default"`, object.NewString("default")},
		// Non-nil value
		{`"value" ?? "default"`, object.NewString("value")},
		// Falsy but non-nil values should NOT trigger default
		{`0 ?? 42`, object.NewInt(0)},
		{`false ?? true`, object.False},
		{`"" ?? "default"`, object.NewString("")},
		// Chained nullish coalescing
		{`nil ?? nil ?? "final"`, object.NewString("final")},
		{`nil ?? "first" ?? "second"`, object.NewString("first")},
		// With expressions
		{`let x = nil; x ?? 10`, object.NewInt(10)},
		{`let x = 5; x ?? 10`, object.NewInt(5)},
		// Comparison with OR (different behavior)
		{`0 || 42`, object.NewInt(42)}, // OR uses truthiness
		{`0 ?? 42`, object.NewInt(0)},  // ?? only checks nil
	}
	runTests(t, tests)
}

func TestSpreadOperator(t *testing.T) {
	tests := []testCase{
		// Array spread
		{`let a = [1, 2]; [...a]`, object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)})},
		{`let a = [1, 2]; [0, ...a, 3]`, object.NewList([]object.Object{
			object.NewInt(0), object.NewInt(1), object.NewInt(2), object.NewInt(3),
		})},
		{`let a = [1]; let b = [2]; [...a, ...b]`, object.NewList([]object.Object{
			object.NewInt(1), object.NewInt(2),
		})},
		// Function call spread
		{`function sum(a, b, c) { return a + b + c }; let args = [1, 2, 3]; sum(...args)`, object.NewInt(6)},
		{
			`function foo(a, b, c, d) { return [a, b, c, d] }; let x = [2, 3]; foo(1, ...x, 4)`,
			object.NewList([]object.Object{
				object.NewInt(1), object.NewInt(2), object.NewInt(3), object.NewInt(4),
			}),
		},
		// Spread with string concatenation
		{`function join(a, b) { return a + b }; let items = ["a", "b"]; join(...items)`, object.NewString("ab")},
	}
	runTests(t, tests)
}

func TestRestParameter(t *testing.T) {
	tests := []testCase{
		// Rest with regular params
		{
			`function foo(a, ...rest) { return [a, rest] }; foo(1, 2, 3)`,
			object.NewList([]object.Object{
				object.NewInt(1),
				object.NewList([]object.Object{object.NewInt(2), object.NewInt(3)}),
			}),
		},
		// Rest with no extra args
		{`function test(...args) { return args }; test()`, object.NewList([]object.Object{})},
		// Rest collects all remaining
		{`function test(a, b, ...rest) { return len(rest) }; test(1, 2, 3, 4, 5)`, object.NewInt(3)},
	}
	runTests(t, tests)
}

func TestObjectDestructuring(t *testing.T) {
	tests := []testCase{
		// Basic destructuring
		{
			`let obj = { a: 1, b: 2 }; let { a, b } = obj; [a, b]`,
			object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)}),
		},
		// With aliases
		{
			`let obj = { name: "Alice", age: 30 }; let { name: n, age: a } = obj; [n, a]`,
			object.NewList([]object.Object{object.NewString("Alice"), object.NewInt(30)}),
		},
		// Single property
		{`let obj = { x: 42 }; let { x } = obj; x`, object.NewInt(42)},
		// From function return
		{
			`function getUser() { return { id: 1, active: true } }; let { id, active } = getUser(); [id, active]`,
			object.NewList([]object.Object{object.NewInt(1), object.True}),
		},
		// Mixed aliases and non-aliases
		{
			`let obj = { a: 1, b: 2, c: 3 }; let { a, b: x, c } = obj; [a, x, c]`,
			object.NewList([]object.Object{object.NewInt(1), object.NewInt(2), object.NewInt(3)}),
		},
	}
	runTests(t, tests)
}

func TestArrayDestructuring(t *testing.T) {
	tests := []testCase{
		// Basic array destructuring
		{
			`let [a, b] = [1, 2]; [a, b]`,
			object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)}),
		},
		// Three elements
		{`let [x, y, z] = [10, 20, 30]; x + y + z`, object.NewInt(60)},
		// With string (unpacks characters)
		{
			`let [a, b, c] = "xyz"; [a, b, c]`,
			object.NewList([]object.Object{object.NewString("x"), object.NewString("y"), object.NewString("z")}),
		},
		// From function return
		{
			`function getCoords() { return [100, 200] }; let [x, y] = getCoords(); [x, y]`,
			object.NewList([]object.Object{object.NewInt(100), object.NewInt(200)}),
		},
		// Single element
		{`let [a] = [42]; a`, object.NewInt(42)},
		// Mixed types
		{
			`let [s, n, b] = ["hello", 42, true]; [s, n, b]`,
			object.NewList([]object.Object{object.NewString("hello"), object.NewInt(42), object.True}),
		},
	}
	runTests(t, tests)
}

func TestDestructuringDefaults(t *testing.T) {
	tests := []testCase{
		// Array destructuring with defaults - empty array
		{
			`let [a = 10, b = 20] = []; [a, b]`,
			object.NewList([]object.Object{object.NewInt(10), object.NewInt(20)}),
		},
		// Array destructuring with defaults - partial array
		{
			`let [a = 10, b = 20] = [5]; [a, b]`,
			object.NewList([]object.Object{object.NewInt(5), object.NewInt(20)}),
		},
		// Array destructuring with defaults - full array
		{
			`let [a = 10, b = 20] = [5, 6]; [a, b]`,
			object.NewList([]object.Object{object.NewInt(5), object.NewInt(6)}),
		},
		// Object destructuring with defaults - empty object
		{
			`let { x = 10, y = 20 } = {}; [x, y]`,
			object.NewList([]object.Object{object.NewInt(10), object.NewInt(20)}),
		},
		// Object destructuring with defaults - partial object
		{
			`let { x = 10, y = 20 } = { x: 5 }; [x, y]`,
			object.NewList([]object.Object{object.NewInt(5), object.NewInt(20)}),
		},
		// Object destructuring with alias and default
		{`let { name: n = "default" } = {}; n`, object.NewString("default")},
		{`let { name: n = "default" } = { name: "Alice" }; n`, object.NewString("Alice")},
		// Mixed - some with defaults, some without
		{
			`let [a, b = 20] = [5]; [a, b]`,
			object.NewList([]object.Object{object.NewInt(5), object.NewInt(20)}),
		},
	}
	runTests(t, tests)
}

func TestObjectSpread(t *testing.T) {
	tests := []testCase{
		// Basic object spread
		{`let a = {x: 1}; let b = {...a}; b.x`, object.NewInt(1)},
		// Spread with additional properties
		{`let a = {x: 1}; let b = {...a, y: 2}; b.y`, object.NewInt(2)},
		// Property override
		{`let a = {x: 1, y: 2}; let b = {...a, y: 99}; b.y`, object.NewInt(99)},
		// Multiple spreads
		{
			`let a = {x: 1}; let c = {y: 2}; let d = {...a, ...c}; [d.x, d.y]`,
			object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)}),
		},
		// Later spread overrides earlier
		{`let a = {x: 1}; let c = {x: 99}; let d = {...a, ...c}; d.x`, object.NewInt(99)},
		// Spread with computed properties
		{`let a = {x: 1}; let b = {...a, y: 2 + 3}; b.y`, object.NewInt(5)},
	}
	runTests(t, tests)
}

func TestOptionalChaining(t *testing.T) {
	tests := []testCase{
		// Property access on non-nil
		{`let obj = { name: "test" }; obj?.name`, object.NewString("test")},
		// Property access on nil
		{`let obj = nil; obj?.name`, object.Nil},
		// Method call on non-nil
		{`let s = "hello"; s?.to_upper()`, object.NewString("HELLO")},
		// Method call on nil
		{`let s = nil; s?.to_upper()`, object.Nil},
		// Chained optional access
		{`let obj = { inner: { value: 42 } }; obj?.inner?.value`, object.NewInt(42)},
		{`let obj = { inner: nil }; obj?.inner?.value`, object.Nil},
		{`let obj = nil; obj?.inner?.value`, object.Nil},
		// Mixed with regular access
		{`let obj = { a: { b: 1 } }; obj.a?.b`, object.NewInt(1)},
		{`let obj = { a: nil }; obj.a?.b`, object.Nil},
		// With nullish coalescing
		{`let obj = nil; obj?.name ?? "default"`, object.NewString("default")},
		{`let obj = { name: "test" }; obj?.name ?? "default"`, object.NewString("test")},
	}
	runTests(t, tests)
}

func TestManyLocals(t *testing.T) {
	result, err := run(context.Background(), `
	function example(x) {
		let a = x + 1
		let b = a + 1
		let c = b + 1
		let d = c + 1
		let e = d + 1
		let f = e + 1
		let g = f + 1
		let h = g + 1
		let i = h + 1
		let j = i + 1
		let k = j + 1
		let l = k + 1
		return l
	}
	example(0)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(12))
}

func TestIncorrectArgCount(t *testing.T) {
	type testCase struct {
		input       string
		expectedErr string
	}
	tests := []testCase{
		{`function ex() { 1 }; ex(1)`, "args error: function \"ex\" takes 0 arguments (1 given)"},
		{`function ex(x) { x }; ex()`, "args error: function \"ex\" takes 1 argument (0 given)"},
		{`function ex(x) { x }; ex(1, 2)`, "args error: function \"ex\" takes 1 argument (2 given)"},
		{`function ex(x, y) { 1 }; ex()`, "args error: function \"ex\" takes 2 arguments (0 given)"},
		{`function ex(x, y) { 1 }; ex(0)`, "args error: function \"ex\" takes 2 arguments (1 given)"},
		{`function ex(x, y) { 1 }; ex(1, 2, 3)`, "args error: function \"ex\" takes 2 arguments (3 given)"},
		{`function ex() { 1 }; [1, 2].filter(ex)`, "args error: function \"ex\" takes 0 arguments (1 given)"},
		{`function ex() { 1 }; "foo" | ex`, "args error: function \"ex\" takes 0 arguments (1 given)"},
		{`"foo" | "bar"`, "type error: object is not callable (got string)"},
	}
	for _, tt := range tests {
		_, err := run(context.Background(), tt.input)
		assert.NotNil(t, err)
		// Check that the error message contains the expected content (may include location info)
		assert.Contains(t, err.Error(), tt.expectedErr)
	}
}

func TestWithContextCheckInterval(t *testing.T) {
	// Test that WithContextCheckInterval properly sets the interval
	ast, err := parser.Parse(context.Background(), `1 + 1`, nil)
	assert.NoError(t, err)
	main, err := compiler.Compile(ast, nil)
	assert.NoError(t, err)

	// Test with custom interval
	vm, err := New(main, WithContextCheckInterval(500))
	assert.Nil(t, err)
	assert.Equal(t, vm.contextCheckInterval, 500)

	// Test with zero (disabled)
	vm2, err := New(main, WithContextCheckInterval(0))
	assert.Nil(t, err)
	assert.Equal(t, vm2.contextCheckInterval, 0)

	// Test default value
	vm3, err := New(main)
	assert.Nil(t, err)
	assert.Equal(t, vm3.contextCheckInterval, DefaultContextCheckInterval)
}

func TestReturnGlobalVariable(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	function test() { x }
	test()
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(3))
}

func TestNakedReturn(t *testing.T) {
	result, err := run(context.Background(), `function test(a) { return }; test(15)`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestGlobalNames(t *testing.T) {
	ctx := context.Background()
	source := `
	let count = 1
	function inc(a, b) { a + b }
	let m = {one: 1}
	let foo = function() { "bar" }
	`
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))

	globals := vm.GlobalNames()
	globalsMap := map[string]bool{}
	for _, g := range globals {
		globalsMap[g] = true
	}
	assert.True(t, globalsMap["count"])
	assert.True(t, globalsMap["inc"])
	assert.True(t, globalsMap["m"])
	assert.True(t, globalsMap["foo"])
}

func TestGetGlobal(t *testing.T) {
	ctx := context.Background()
	source := `function inc(a, b) { a + b }`
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))

	obj, err := vm.Get("inc")
	assert.Nil(t, err)
	fn, ok := obj.(*object.Closure)
	assert.True(t, ok)
	assert.Equal(t, fn.Name(), "inc")
}

func TestCall(t *testing.T) {
	ctx := context.Background()
	source := `function inc(a, b) { a + b }`
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))

	obj, err := vm.Get("inc")
	assert.Nil(t, err)
	fn, ok := obj.(*object.Closure)
	assert.True(t, ok)

	result, err := vm.Call(ctx, fn, []object.Object{
		object.NewInt(9),
		object.NewInt(1),
	})
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(10))
}

func TestCallWithClosure(t *testing.T) {
	ctx := context.Background()
	source := `
	function get_counter() {
		let count = 10
		return function() {
			count++
			return count
		}
	}
	let counter = get_counter()
	`
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))

	obj, err := vm.Get("counter")
	assert.Nil(t, err)
	counter, ok := obj.(*object.Closure)
	assert.True(t, ok)

	// The counter's first value will be 11. Confirm it counts up from there.
	for i := int64(11); i < 100; i++ {
		obj, err := vm.Call(ctx, counter, []object.Object{})
		assert.Nil(t, err)
		assert.Equal(t, obj, object.NewInt(i))
	}
}

func TestFreeVariableAssignment(t *testing.T) {
	ctx := context.Background()
	source := `
	function get_counters() {
		let a = 0
		let b = 0
		let c = 0
		function incA() {
			a++
			return a
		}
		function incB() {
			b++
			return b
		}
		function incC() {
			c++
			return c
		}
		return [incA, incB, incC]
	}
	let incA, incB, incC = get_counters()
	incA(); incA()                 // 1, 2
	incB(); incB(); incB()         // 1, 2, 3
	incC(); incC(); incC(); incC() // 1, 2, 3, 4
	[incA(), incB(), incC()]       // [3, 4, 5]
	`
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))
	result, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewInt(3),
			object.NewInt(4),
			object.NewInt(5),
		}))
}

func TestInterpolatedStringClosures1(t *testing.T) {
	ctx := context.Background()
	source := "function foo(a, b, c) {\n" +
		"	return function(d) {\n" +
		"		return `${a.to_upper()}-${b}-${c}-${d}`\n" +
		"	}\n" +
		"}\n" +
		"foo(\"foo\", \"bar\", \"baz\")(\"go\")"
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))
	result, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, result, object.NewString("FOO-bar-baz-go"))
}

func TestInterpolatedStringClosures2(t *testing.T) {
	ctx := context.Background()
	source := "let x = 3\n" +
		"function foo(a, b=\"bar\") {\n" +
		"	let count = 42\n" +
		"	return function(a) {\n" +
		"		return `a: ${a} b: ${b} count: ${count-2} x: ${x+1}`\n" +
		"	}\n" +
		"}\n" +
		"foo(\"IGNORED\")(\"HEY\")"
	vm, err := newVM(ctx, source)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))
	result, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, result, object.NewString("a: HEY b: bar count: 40 x: 4"))
}

func TestIncrementalEvaluation(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "let x = 3", nil)
	assert.Nil(t, err)

	main, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)

	v, err := New(main)
	assert.Nil(t, err)
	assert.Nil(t, v.Run(ctx))
	value, err := v.Get("x")
	assert.Nil(t, err)
	assert.Equal(t, value, object.NewInt(3))

	// For incremental evaluation, we need to tell the compiler about globals
	// that were defined in previous runs
	ast, err = parser.Parse(ctx, "x + 7", nil)
	assert.Nil(t, err)
	code2, err := compiler.Compile(ast, &compiler.Config{GlobalNames: []string{"x"}})
	assert.Nil(t, err)
	assert.Nil(t, v.RunCode(ctx, code2))
	value, err = v.Get("x")
	assert.Nil(t, err)
	assert.Equal(t, value, object.NewInt(3))

	tos, ok := v.TOS()
	assert.True(t, ok)
	assert.Equal(t, tos, object.NewInt(10))
}

func TestModifyModule(t *testing.T) {
	_, err := run(context.Background(), `math.max = 123`)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "type error: cannot modify module attributes")
}

func TestFreeVariables(t *testing.T) {
	code := `
	function test(count) {
		let l = []
		function() {
			let y = count
			if (true) {
				l.append(y)
			}
		}()
		return l
	}
	test(5)
	`
	result, err := run(context.Background(), code)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewList([]object.Object{object.NewInt(5)}))
}

func TestMaps(t *testing.T) {
	tests := []testCase{
		{`{"a": 1}`, object.NewMap(map[string]object.Object{
			"a": object.NewInt(1),
		})},
		{`{"a": 1,}`, object.NewMap(map[string]object.Object{
			"a": object.NewInt(1),
		})},
		{`{"a": 1,
		  }`, object.NewMap(map[string]object.Object{
			"a": object.NewInt(1),
		})},
		{`{"a": 1,
		   "b": 2}`, object.NewMap(map[string]object.Object{
			"a": object.NewInt(1),
			"b": object.NewInt(2),
		})},
		{`{"a": 1,
			"b": 2
		}`, object.NewMap(map[string]object.Object{
			"a": object.NewInt(1),
			"b": object.NewInt(2),
		})},
		{`let m = {"a": 1, "b": 2}; m["a"] *= 8; m`, object.NewMap(map[string]object.Object{
			"a": object.NewInt(8),
			"b": object.NewInt(2),
		})},
	}
	runTests(t, tests)
}

func TestLists(t *testing.T) {
	tests := []testCase{
		{`[1,2,3]`, object.NewList([]object.Object{
			object.NewInt(1),
			object.NewInt(2),
			object.NewInt(3),
		})},
		{`[1,
		   2,
		   3]`, object.NewList([]object.Object{
			object.NewInt(1),
			object.NewInt(2),
			object.NewInt(3),
		})},
		{`[1,
		   2,]`, object.NewList([]object.Object{
			object.NewInt(1),
			object.NewInt(2),
		})},
		{`[1,
		2
		]`, object.NewList([]object.Object{
			object.NewInt(1),
			object.NewInt(2),
		})},
	}
	runTests(t, tests)
}

func TestMultivar(t *testing.T) {
	code := `
	let x, y = [1, 2]
	[x, y]
	`
	result, err := run(context.Background(), code)
	assert.Nil(t, err)
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewInt(1),
			object.NewInt(2),
		}))
}

func TestReturnNamedFunction(t *testing.T) {
	code := `
	function test() {
		return function foo() {
			return "FOO"
		}
	}
	let f = test()
	f()
	`
	result, err := run(context.Background(), code)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("FOO"))
}

func TestContextDone(t *testing.T) {
	// Context with no deadline does not return a Done channel
	ctx := context.Background()
	d := ctx.Done()
	assert.Nil(t, d)

	// Context with deadline returns a Done channel
	tctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	d = tctx.Done()
	assert.NotNil(t, d)

	// Context with cancel returns a Done channel
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d = cctx.Done()
	assert.NotNil(t, d)
}

type testCase struct {
	input    string
	expected object.Object
}

func runTests(t *testing.T, tests []testCase) {
	t.Helper()
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Helper()
			result, err := run(ctx, tt.input)
			assert.Nil(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestFunctionForwardDeclaration(t *testing.T) {
	tests := []testCase{
		// Basic forward declaration - function called before definition
		{`
		function main() {
			return helper(5)
		}
		
		function helper(x) {
			return x * 2
		}
		
		main()
		`, object.NewInt(10)},

		// Forward declaration with multiple functions
		{`
		function start() {
			return first() + second()
		}
		
		function first() {
			return 10
		}
		
		function second() {
			return 20
		}
		
		start()
		`, object.NewInt(30)},

		// Forward declaration with nested calls
		{`
		function outer() {
			return inner() + 5
		}
		
		function inner() {
			return deepest() * 2
		}
		
		function deepest() {
			return 7
		}
		
		outer()
		`, object.NewInt(19)},

		// Forward declaration with default parameters
		{`
		function calculator(op="add") {
			if (op == "add") {
				return adder(5, 3)
			} else {
				return multiplier(5, 3)
			}
		}

		function adder(a, b) {
			return a + b
		}

		function multiplier(a, b) {
			return a * b
		}

		calculator()
		`, object.NewInt(8)},

		// Forward declaration with closures
		{`
		function makeCounter() {
			let count = 0
			return function() {
				count++
				return incrementHelper(count)
			}
		}
		
		function incrementHelper(n) {
			return n * 10
		}
		
		let counter = makeCounter()
		counter()
		`, object.NewInt(10)},
	}
	runTests(t, tests)
}

func TestMutualRecursion(t *testing.T) {
	tests := []testCase{
		// Basic mutual recursion - even/odd
		{`
		function is_even(n) {
			if (n == 0) {
				return true
			}
			return is_odd(n - 1)
		}

		function is_odd(n) {
			if (n == 0) {
				return false
			}
			return is_even(n - 1)
		}

		[is_even(4), is_odd(4), is_even(5), is_odd(5)]
		`, object.NewList([]object.Object{
			object.True,
			object.False,
			object.False,
			object.True,
		})},

		// Mutual recursion with return values
		{`
		function countdown_a(n) {
			if (n <= 0) {
				return 0
			}
			return n + countdown_b(n - 1)
		}

		function countdown_b(n) {
			if (n <= 0) {
				return 0
			}
			return n + countdown_a(n - 1)
		}

		countdown_a(5)
		`, object.NewInt(15)},

		// More complex mutual recursion
		{`
		function fibonacci_a(n) {
			if (n <= 1) {
				return n
			}
			return fibonacci_b(n - 1) + fibonacci_a(n - 2)
		}

		function fibonacci_b(n) {
			if (n <= 1) {
				return n
			}
			return fibonacci_a(n - 1) + fibonacci_b(n - 2)
		}

		fibonacci_a(6)
		`, object.NewInt(8)},
	}
	runTests(t, tests)
}

func TestForwardDeclarationWithConditionals(t *testing.T) {
	tests := []testCase{
		// Forward declaration with if statements
		{`
		function process(x) {
			if (x > 10) {
				return big_handler(x)
			} else {
				return small_handler(x)
			}
		}

		function big_handler(x) {
			return x * 2
		}

		function small_handler(x) {
			return x + 10
		}

		[process(5), process(15)]
		`, object.NewList([]object.Object{
			object.NewInt(15),
			object.NewInt(30),
		})},

		// Forward declaration with switch
		{`
		function router(op) {
			switch (op) {
				case "add":
					return op_add(5, 3)
				case "sub":
					return op_sub(5, 3)
				default:
					return op_default()
			}
		}
		
		function op_add(a, b) {
			return a + b
		}
		
		function op_sub(a, b) {
			return a - b
		}
		
		function op_default() {
			return 0
		}
		
		[router("add"), router("sub"), router("unknown")]
		`, object.NewList([]object.Object{
			object.NewInt(8),
			object.NewInt(2),
			object.NewInt(0),
		})},
	}
	runTests(t, tests)
}

func TestForwardDeclarationEdgeCases(t *testing.T) {
	tests := []testCase{
		// Forward declaration with nested function returning global function
		{`
		function outer() {
			function inner() {
				return "inner"
			}
			
			return inner() + " " + global_helper()
		}
		
		function global_helper() {
			return "outer"
		}
		
		outer()
		`, object.NewString("inner outer")},

		// Forward declaration with anonymous functions
		{`
		function factory() {
			return function() {
				return delayed_function()
			}
		}
		
		function delayed_function() {
			return "delayed"
		}
		
		let fn = factory()
		fn()
		`, object.NewString("delayed")},

		// Forward declaration with function as parameter
		{`
		function processor(fn) {
			return fn(5)
		}
		
		function main() {
			return processor(multiplier)
		}
		
		function multiplier(x) {
			return x * 3
		}
		
		main()
		`, object.NewInt(15)},
	}
	runTests(t, tests)
}

func TestForwardDeclarationErrors(t *testing.T) {
	ctx := context.Background()
	type testCase struct {
		name        string
		input       string
		expectedErr string
	}

	tests := []testCase{
		{
			name: "undefined function call",
			input: `
			function caller() {
				return nonexistent_function()
			}
			caller()
			`,
			expectedErr: "undefined variable \"nonexistent_function\"",
		},
		{
			name: "function redefinition error",
			input: `
			function duplicate() {
				return 1
			}
			
			function duplicate() {
				return 2
			}
			
			duplicate()
			`,
			expectedErr: "function \"duplicate\" redefined",
		},
		{
			name: "circular dependency with undefined function",
			input: `
			function a() {
				return b() + c()  // c() is never defined
			}
			
			function b() {
				return a()
			}
			
			a()
			`,
			expectedErr: "undefined variable \"c\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(ctx, tt.input)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestRunCode(t *testing.T) {
	ctx := context.Background()

	// Create a VM with initial code
	vm, err := newVM(ctx, "let x = 10; let y = 20; x + y")
	assert.NoError(t, err)

	// Run the initial code
	assert.NoError(t, vm.Run(ctx))

	result, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, int64(30), result.(*object.Int).Value())

	// Compile and run different code on the same VM
	ast2, err := parser.Parse(ctx, "let a = 5; let b = 15; a * b", nil)
	assert.NoError(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, &compiler.Config{GlobalNames: globalNames})
	assert.NoError(t, err)

	// Run the second code on the same VM
	assert.NoError(t, vm.RunCode(ctx, code2))

	result2, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, int64(75), result2.(*object.Int).Value())

	// Run a third piece of code
	source3 := `
		let name = "Risor"
		let greeting = "Hello, " + name + "!"
		greeting
	`
	ast3, err := parser.Parse(ctx, source3, nil)
	assert.NoError(t, err)

	code3, err := compiler.Compile(ast3, &compiler.Config{GlobalNames: globalNames})
	assert.NoError(t, err)
	assert.NoError(t, vm.RunCode(ctx, code3))

	result3, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, "Hello, Risor!", result3.(*object.String).Value())
}

func TestRunCodeWithGlobalVariables(t *testing.T) {
	ctx := context.Background()

	// Create a VM with custom globals
	customGlobals := map[string]interface{}{
		"baseValue":  100,
		"multiplier": 2,
	}

	source1 := `
		let result = baseValue * multiplier
		result
	`
	vm, err := newVM(ctx, source1, runOpts{Globals: customGlobals})
	assert.NoError(t, err)
	assert.NoError(t, vm.Run(ctx))

	result, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, int64(200), result.(*object.Int).Value())

	// Run different code that also uses globals
	source2 := `
		let newResult = baseValue + multiplier
		newResult
	`
	ast2, err := parser.Parse(ctx, source2, nil)
	assert.NoError(t, err)

	var globalNames []string
	for k := range customGlobals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, &compiler.Config{GlobalNames: globalNames})
	assert.NoError(t, err)
	assert.NoError(t, vm.RunCode(ctx, code2))

	result2, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, int64(102), result2.(*object.Int).Value())
}

func TestRunCodeFunctions(t *testing.T) {
	ctx := context.Background()

	// Test that functions work correctly when running multiple code objects
	source1 := `
		function add(a, b) {
			return a + b
		}
		add(10, 20)
	`
	vm, err := newVM(ctx, source1)
	assert.NoError(t, err)
	assert.NoError(t, vm.Run(ctx))

	result, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, int64(30), result.(*object.Int).Value())

	// Run code with a different function
	source2 := `
		function multiply(x, y) {
			return x * y
		}
		multiply(6, 7)
	`
	ast2, err := parser.Parse(ctx, source2, nil)
	assert.NoError(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, &compiler.Config{GlobalNames: globalNames})
	assert.NoError(t, err)
	assert.NoError(t, vm.RunCode(ctx, code2))

	result2, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, int64(42), result2.(*object.Int).Value())
}

func TestRunCodeOnVM(t *testing.T) {
	ctx := context.Background()

	// Create a VM with initial code
	vm, err := newVM(ctx, "let x = 42; x")
	assert.NoError(t, err)
	assert.NoError(t, vm.Run(ctx))

	// Compile a different piece of code
	ast2, err := parser.Parse(ctx, "let y = 100; let z = 200; y + z", nil)
	assert.NoError(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, &compiler.Config{GlobalNames: globalNames})
	assert.NoError(t, err)
	result, err := RunCodeOnVM(ctx, vm, code2)
	assert.NoError(t, err)
	assert.Equal(t, int64(300), result.(*object.Int).Value())
}

func TestRunCodeFirst(t *testing.T) {
	ctx := context.Background()
	vm, err := newVM(ctx, `
		function add(a, b) { return a + b }
		add(10, 20)
	`)
	assert.NoError(t, err)
	assert.NoError(t, vm.RunCode(ctx, vm.main))
	result, exists := vm.TOS()
	assert.True(t, exists)
	assert.Equal(t, int64(30), result.(*object.Int).Value())
}

func TestNewEmpty(t *testing.T) {
	ctx := context.Background()
	compile := func(source string) *bytecode.Code {
		ast, err := parser.Parse(ctx, source, nil)
		assert.NoError(t, err)
		code, err := compiler.Compile(ast, nil)
		assert.NoError(t, err)
		return code
	}

	// Test creating a VM without main code
	vm, err := NewEmpty()
	assert.NoError(t, err)

	// Test that Run() returns an error when no main code is provided
	err = vm.Run(ctx)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "no main code available")

	// Test that RunCode() works with specific code
	code := compile(`let x = 42; x`)
	err = vm.RunCode(ctx, code)
	assert.NoError(t, err)

	// Verify the result is on the stack
	result, ok := vm.TOS()
	assert.True(t, ok)
	intResult, ok := result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(42), intResult.Value())

	// Test that Call() works with functions
	fnCode := compile(`function add(a, b) { return a + b }`)
	err = vm.RunCode(ctx, fnCode)
	assert.NoError(t, err)

	addFn, err := vm.Get("add")
	assert.NoError(t, err)

	result, err = vm.Call(ctx, addFn.(*object.Closure), []object.Object{
		object.NewInt(10),
		object.NewInt(20),
	})
	assert.NoError(t, err)

	intResult, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(30), intResult.Value())
}

func TestArrowFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name:     "simple arrow function",
			input:    `let add = (x, y) => x + y; add(2, 3)`,
			expected: object.NewInt(5),
		},
		{
			name:     "arrow function no params",
			input:    `let f = () => 42; f()`,
			expected: object.NewInt(42),
		},
		{
			name:     "arrow function single param",
			input:    `let double = (x) => x * 2; double(5)`,
			expected: object.NewInt(10),
		},
		{
			name:     "arrow function with block body",
			input:    `let f = (x) => { return x + 1 }; f(10)`,
			expected: object.NewInt(11),
		},
		{
			name:     "arrow function with default parameter",
			input:    `let greet = (name = "world") => "hello " + name; greet()`,
			expected: object.NewString("hello world"),
		},
		{
			name:     "arrow function default parameter override",
			input:    `let greet = (name = "world") => "hello " + name; greet("claude")`,
			expected: object.NewString("hello claude"),
		},
		{
			name:     "arrow function as callback",
			input:    `[1, 2, 3].map((x) => x * 2)`,
			expected: object.NewList([]object.Object{object.NewInt(2), object.NewInt(4), object.NewInt(6)}),
		},
		{
			name:     "arrow function filter",
			input:    `[1, 2, 3, 4, 5].filter((x) => x > 2)`,
			expected: object.NewList([]object.Object{object.NewInt(3), object.NewInt(4), object.NewInt(5)}),
		},
		{
			name:     "immediately invoked arrow function",
			input:    `((x) => x + 1)(5)`,
			expected: object.NewInt(6),
		},
		{
			name:     "single param no parens",
			input:    `let double = x => x * 2; double(5)`,
			expected: object.NewInt(10),
		},
		{
			name:     "single param no parens as callback",
			input:    `[1, 2, 3].map(x => x * 10)`,
			expected: object.NewList([]object.Object{object.NewInt(10), object.NewInt(20), object.NewInt(30)}),
		},
		{
			name:     "arrow function returning arrow function",
			input:    `let makeAdder = (x) => (y) => x + y; let add5 = makeAdder(5); add5(3)`,
			expected: object.NewInt(8),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestTryCatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "basic try/catch with throw",
			input: `
			let result = "initial"
			try {
				throw "error"
			} catch e {
				result = "caught"
			}
			result
			`,
			expected: object.NewString("caught"),
		},
		{
			name: "try block succeeds, catch not executed",
			input: `
			let result = "success"
			try {
				result = "try"
			} catch e {
				result = "catch"
			}
			result
			`,
			expected: object.NewString("try"),
		},
		{
			name: "catch without binding",
			input: `
			let result = "initial"
			try {
				throw "oops"
			} catch {
				result = "handled"
			}
			result
			`,
			expected: object.NewString("handled"),
		},
		{
			name: "try/finally without catch",
			input: `
			let cleanup = false
			try {
				cleanup = true
			} finally {
				cleanup = true
			}
			cleanup
			`,
			expected: object.True,
		},
		{
			name: "finally runs after catch",
			input: `
			let steps = []
			try {
				throw "err"
			} catch e {
				steps = steps + ["catch"]
			} finally {
				steps = steps + ["finally"]
			}
			steps
			`,
			expected: object.NewList([]object.Object{
				object.NewString("catch"),
				object.NewString("finally"),
			}),
		},
		{
			name: "throw string value",
			input: `
			let msg = ""
			try {
				throw "my error message"
			} catch e {
				msg = string(e)
			}
			msg
			`,
			expected: object.NewString("my error message"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			assert.Nil(t, err, "unexpected error: %v", err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestThrowWithoutCatch(t *testing.T) {
	// Test that an unhandled throw propagates as an error
	_, err := run(context.Background(), `throw "unhandled"`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unhandled")
}

func TestMaxSteps(t *testing.T) {
	ctx := context.Background()
	// Use many arithmetic operations to consume steps
	source := `
	let x = 0
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x = x + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	x
	`
	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	main, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)

	// Run with a very low step limit - should hit limit quickly
	vm, err := New(main, WithMaxSteps(50), WithContextCheckInterval(10))
	assert.Nil(t, err)
	err = vm.Run(ctx)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrStepLimitExceeded)
}

func TestMaxStepsNotExceeded(t *testing.T) {
	ctx := context.Background()
	// Use list().each() with range to sum numbers
	source := `
	let sum = 0
	list(range(10)).each(function(i) { sum = sum + i })
	sum
	`
	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	main, err := compiler.Compile(ast, &compiler.Config{GlobalNames: globalNames})
	assert.Nil(t, err)

	// Run with a high step limit that won't be exceeded
	vm, err := New(main, WithGlobals(globals), WithMaxSteps(100000))
	assert.Nil(t, err)
	err = vm.Run(ctx)
	assert.Nil(t, err)

	tos, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, tos, object.NewInt(45))
}

func TestMaxStackDepth(t *testing.T) {
	ctx := context.Background()
	// Recursive function that will overflow with a low stack limit
	source := `
function recurse(n) {
	if (n > 0) {
		return recurse(n - 1) + 1
	}
	return 0
}
recurse(100)
`
	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	main, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)

	// Run with a stack depth limit that will be exceeded
	vm, err := New(main, WithMaxStackDepth(10))
	assert.Nil(t, err)
	err = vm.Run(ctx)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "stack overflow")
}

func TestTimeout(t *testing.T) {
	ctx := context.Background()
	// Use list().each() with range to iterate for a long time
	source := `
	let sum = 0
	list(range(1000000)).each(function(i) { sum = sum + i })
	sum
	`
	ast, err := parser.Parse(ctx, source, nil)
	assert.Nil(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	main, err := compiler.Compile(ast, &compiler.Config{GlobalNames: globalNames})
	assert.Nil(t, err)

	// Run with a very short timeout
	vm, err := New(main, WithGlobals(globals), WithTimeout(5*time.Millisecond))
	assert.Nil(t, err)
	err = vm.Run(ctx)
	assert.NotNil(t, err)
	assert.Equal(t, err, context.DeadlineExceeded)
}
