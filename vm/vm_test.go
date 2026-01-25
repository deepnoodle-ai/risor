package vm

import (
	"context"
	"testing"
	"time"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/object"
	ros "github.com/risor-io/risor/os"
	"github.com/risor-io/risor/parser"
	"github.com/stretchr/testify/require"
)

func TestAddCompilationAndExecution(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
	let x = 11
	let y = 12
	x + y
	`)
	require.Nil(t, err)

	c, err := compiler.New()
	require.Nil(t, err)

	main, err := c.Compile(program)
	require.Nil(t, err)

	constsCount := main.ConstantsCount()
	require.Equal(t, 2, constsCount)

	c1, ok := main.Constant(0).(int64)
	require.True(t, ok)
	require.Equal(t, int64(11), c1)

	c2, ok := main.Constant(1).(int64)
	require.True(t, ok)
	require.Equal(t, int64(12), c2)

	vm := New(main)
	require.Nil(t, vm.Run(context.Background()))

	tos, ok := vm.TOS()
	require.True(t, ok)
	require.Equal(t, object.NewInt(23), tos)
}

func TestConditional(t *testing.T) {
	program, err := parser.Parse(context.Background(), `
	let x = 20
	if x > 10 {
		x = 99
	}
	x
	`)
	require.Nil(t, err)

	main, err := compiler.Compile(program)
	require.Nil(t, err)

	vm := New(main)
	require.Nil(t, vm.Run(context.Background()))

	tos, ok := vm.TOS()
	require.True(t, ok)
	require.Equal(t, object.NewInt(99), tos)
}

func TestConditional3(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 5
	let y = 10
	if x > 1 {
		y
	} else {
		99
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(10), result)
}

func TestConditional4(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 5
	let y = 22
	let z = 33
	if x < 1 {
		x = y
	} else {
		x = z
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(33), result)
}

func TestLoop(t *testing.T) {
	result, err := run(context.Background(), `
	let y = 0
	for {
		y = y + 1
		if y > 10 {
			break
		}
	}
	y
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(11), result)
}

func TestForLoop2(t *testing.T) {
	result, err := run(context.Background(),
		`let x = 0; for let y = 0; y < 5; y++ { x = y }; x`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), result)
}

func TestForLoop3(t *testing.T) {
	result, err := run(context.Background(), `let x = 0; for x < 10 { x++ }; x`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(10), result)
}

func TestForRange1(t *testing.T) {
	result, err := run(context.Background(), `
	let x = [1, 2.3, "hello", true]
	let output = []
	for let i = range x {
		1 + 2
		3 + 4
	}
	99
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(99), result)
}

func TestForRange2(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for let _, value = range [5,6,7] {
		x = value
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(7), result)
}

func TestForRange3(t *testing.T) {
	result, err := run(context.Background(), `
	let x, y = [0, 0]
	for let i, value = range [5, 6, 7] {
		x = i      // should go up to 2
		y = value  // should go up to 7
	}
	[x, y]
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(2),
		object.NewInt(7),
	}), result)
}

func TestForRange4(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for let i = range ["a", "b", "c"] { x = i }
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(2), result)
}

func TestForRange5(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for range ["a", "b", "c"] { x++ }
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), result)
}

func TestForRange7(t *testing.T) {
	result, err := run(context.Background(), `
	let x = nil
	let y = nil
	let count = 0
	let f = function() { range [ "a", "b", "c" ] }
	for let i, value = f() {
		x = i      // should count 0, 1, 2
		y = value  // should go "a", "b", "c"
		count++
	}
	[x, y, count]
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(2),
		object.NewString("c"),
		object.NewInt(3),
	}), result)
}

func TestIterator(t *testing.T) {
	tests := []testCase{
		{`(range [ 33, 44, 55 ]).next()`, object.NewInt(33)},
		{`(range [ 33, 44, 55 ]).next()`, object.NewInt(33)},
		{`let i = range "abcd"; i.next(); i.entry().key`, object.NewInt(0)},
		{`let i = range "abcd"; i.next(); i.entry().value`, object.NewString("a")},
		{`let c = { a: 33, b: 44 }; (range c).next()`, object.NewString("a")},
		{`let c = { a: 33, b: 44 }; let i = range c; i.next(); i.entry().key`, object.NewString("a")},
		{`let c = { a: 33, b: 44 }; let i = range c; i.next(); i.entry().value`, object.NewInt(33)},
	}
	runTests(t, tests)
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

func TestStackPopping1(t *testing.T) {
	result, err := run(context.Background(), `
	let x = []
	for let i = 0; i < 4; i++ {
		1
		2
		3
		4
		x.append(i)
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(0),
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	}), result)
}

func TestStackPopping2(t *testing.T) {
	result, err := run(context.Background(), `
	for let i = range [1, 2, 3] {
		1
		2
		3
		4
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
}

func TestStackBehavior1(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 99
	for let i = 0; i < 4; x {
		i++
		1
		2
		3
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
}

func TestStackBehavior2(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 77
	for let i = 0; i < 4; x {
		i++
		1
		2
		3
		4
		if i > 0 {
			break // loop once
		}
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
}

func TestStackBehavior3(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 77
	if x > 0 {
		99 
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(99), result)
}

func TestStackBehavior4(t *testing.T) {
	result, err := run(context.Background(), `
	let x = -1
	if x > 0 {
		99 
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
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
	require.Nil(t, err)
	require.Equal(t, object.NewInt(2), result)
}

func TestFunctionCall(t *testing.T) {
	result, err := run(context.Background(), `
	let f = function(x) { 42 + x }
	let v = f(1)
	v + 10
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(53), result)
}

func TestSwitch1(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch x {
		case 1:
		case 2:
			21
		case 3:
			42
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(42), result)
}

func TestSwitch2(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 1
	switch x {
		case 1:
			99
		case 2:
			42
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(99), result)
}

func TestSwitch3(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch x {
		case 1:
			99
		case 2:
			42
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
}

func TestSwitch4(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch x { default: 99 }
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(99), result)
}

func TestSwitch5(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	switch x { default: 99 case 3: x; x-1 }
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(2), result)
}

func TestStr(t *testing.T) {
	result, err := run(context.Background(), `
	let s = "hello"
	s
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewString("hello"), result)
}

func TestStrLen(t *testing.T) {
	result, err := run(context.Background(), `
	let s = "hello"
	len(s)
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(5), result)
}

func TestList1(t *testing.T) {
	result, err := run(context.Background(), `
	let l = [1, 2, 3]
	l
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	}), result)
}

func TestList2(t *testing.T) {
	result, err := run(context.Background(), `
	let plusOne = function(x) { x + 1 }
	[plusOne(0), 4-2, plusOne(2)]
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	}), result)
}

func TestMap(t *testing.T) {
	result, err := run(context.Background(), `
	{"a": 1, "b": 4-2}
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewMap(map[string]object.Object{
		"a": object.NewInt(1),
		"b": object.NewInt(2),
	}), result)
}

func TestNonLocal(t *testing.T) {
	result, err := run(context.Background(), `
	let y = 3
	let z = 99
	let f = function() { y = 4 }
	f()
	y
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), result)
}

func TestFrameLocals1(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 1
	let f = function(x) { x = 99 }
	f(4)
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(1), result)
}

func TestFrameLocals2(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 1
	let f = function(y) { x = 99 }
	f(4)
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(99), result)
}

func TestMapKeys(t *testing.T) {
	result, err := run(context.Background(), `
	let m = {"a": 1, "b": 2}
	keys(m)
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewString("a"),
		object.NewString("b"),
	}), result)
}

func TestClosure(t *testing.T) {
	result, err := run(context.Background(), `
	let f = function(x) { function() { x } }
	let closure = f(22)
	closure()
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(22), result)
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
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), result)
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
	require.Nil(t, err)
	require.Equal(t, object.NewInt(101), result)
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
	require.Nil(t, err)
	require.Equal(t, object.NewStringList([]string{"hello", "world", "risor", "go"}), result)
}

func TestRecursiveExample1(t *testing.T) {
	result, err := run(context.Background(), `
	function twoexp(n) {
		if n == 0 {
			return 1
		} else {
			return 2 * twoexp(n-1)
		}
	}
	twoexp(4)
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(16), result)
}

func TestRecursiveExample2(t *testing.T) {
	result, err := run(context.Background(), `
	function twoexp(n) {
		let a = 1
		let b = 2
		let c = a * b
		if n == 0 {
			return 1
		} else {
			return c * twoexp(n-1)
		}
	}
	twoexp(4)
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(16), result)
}

func TestConstant(t *testing.T) {
	_, err := run(context.Background(), `const x = 1; x = 2`)
	require.NotNil(t, err)
	require.Equal(t, "compile error: cannot assign to constant \"x\"\n\nlocation: unknown:1:16 (line 1, column 16)", err.Error())
}

func TestConstantFunction(t *testing.T) {
	_, err := run(context.Background(), `
	function add(x, y) { x + y }
	add = "bloop"
	`)
	require.NotNil(t, err)
	require.Equal(t, "compile error: cannot assign to constant \"add\"\n\nlocation: unknown:3:6 (line 3, column 6)", err.Error())
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
		{`for let i = 0; i < 10; i++ { 42 }`, object.Nil},
		{`let x = 0; for let i = 0; i < 10; i++ { x = i }`, object.Nil},
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
		{`if false { 3 }`, object.Nil},
		{`if true { 3 }`, object.NewInt(3)},
		{`if false { 3 } else { 4 }`, object.NewInt(4)},
		{`if true { 3 } else { 4 }`, object.NewInt(3)},
		{`if false { 3 } else if false { 4 } else { 5 }`, object.NewInt(5)},
		{`if true { 3 } else if false { 4 } else { 5 }`, object.NewInt(3)},
		{`if false { 3 } else if true { 4 } else { 5 }`, object.NewInt(4)},
		{`let x = 1; if x > 5 { 99 } else { 100 }`, object.NewInt(100)},
		{`let x = 1; if x > 0 { 99 } else { 100 }`, object.NewInt(99)},
		{`let x = 1; let y = x > 0 ? 77 : 88; y`, object.NewInt(77)},
		{`let x = (1 > 2) ? 77 : 88; x`, object.NewInt(88)},
		{`let x = (2 > 1) ? 77 : 88; x`, object.NewInt(77)},
		{`let x = 1; switch x { case 1: 99; case 2: 100 }`, object.NewInt(99)},
		{`switch 2 { case 1: 99; case 2: 100 }`, object.NewInt(100)},
		{`switch 3 { case 1: 99; default: 42 case 2: 100 }`, object.NewInt(42)},
		{`switch 3 { case 1: 99; case 2: 100 }`, object.Nil},
		{`switch 3 { case 1, 3: 99; case 2: 100 }`, object.NewInt(99)},
		{`switch 3 { case 1: 99; case 2, 4-1: 100 }`, object.NewInt(100)},
		{`let x = 3; switch bool(x) { case true: "wow" }`, object.NewString("wow")},
		{`let x = 0; switch bool(x) { case true: "wow" }`, object.Nil},
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
		{`len(string(byte_slice([0, 1, 2])))`, object.NewInt(3)},
	}
	runTests(t, tests)
}

func TestBuiltins(t *testing.T) {
	tests := []testCase{
		{`len("hello")`, object.NewInt(5)},
		{`keys({"a": 1})`, object.NewList([]object.Object{
			object.NewString("a"),
		})},
		{`byte(9)`, object.NewByte(9)},
		{`byte_slice([9])`, object.NewByteSlice([]byte{9})},
		{`float_slice([9])`, object.NewFloatSlice([]float64{9})},
		{`type(3.14159)`, object.NewString("float")},
		{`type("hi".contains)`, object.NewString("builtin")},
		{`sprintf("%d-%d", 1, 2)`, object.NewString("1-2")},
		{`int("99")`, object.NewInt(99)},
		{`float("2.5")`, object.NewFloat(2.5)},
		{`string(99)`, object.NewString("99")},
		{`string(2.5)`, object.NewString("2.5")},
		{`ord("a")`, object.NewInt(97)},
		{`chr(97)`, object.NewString("a")},
		{`encode("hi", "hex")`, object.NewString("6869")},
		{`encode("hi", "base64")`, object.NewString("aGk=")},
		{`iter("abc").next()`, object.NewString("a")},
		{`let i = iter("abc"); i.next(); i.entry().key`, object.NewInt(0)},
		{`let i = iter("abc"); i.next(); i.entry().value`, object.NewString("a")},
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

func TestTry(t *testing.T) {
	tests := []testCase{
		{`try(1)`, object.NewInt(1)},
		{`try(1, 2)`, object.NewInt(1)},
		{`try(function() { error("oops") }, "nope")`, object.NewString("nope")},
		{`try(function() { error("oops") }, function(e) { e })`, object.Errorf("oops").WithRaised(false)},
		{`try(function() { error("oops") }, function(e) { e.error() })`, object.NewString("oops")},
		{`try(function() { error("oops") }, function() { error("oops") }, 1)`, object.NewInt(1)},
		{`let x = 0; let y = 0; let z = try(function() {
			x = 11
			error("oops1")
			x = 12
		  }, function() {
			y = 21
			error("oops2")
			y = 22
		  }, 33); [x, y, z]`, object.NewList([]object.Object{
			object.NewInt(11),
			object.NewInt(21),
			object.NewInt(33),
		})},
	}
	runTests(t, tests)
}

func TestTryEvalError(t *testing.T) {
	code := `
	try(function() { error(errors.eval_error("oops")) }, 1)
	`
	_, err := run(context.Background(), code)
	require.NotNil(t, err)
	require.Equal(t, "oops", err.Error())
	require.Equal(t, errz.EvalErrorf("oops"), err)
}

func TestTryTypeError(t *testing.T) {
	code := `
	let i = 0
	try(function() { i.append("x") }, function(e) { e.message() })
	`
	result, err := run(context.Background(), code)
	require.NoError(t, err)
	require.Equal(t, object.NewString("type error: attribute \"append\" not found on int object"), result)
}

func TestTryUnsupportedOperation(t *testing.T) {
	code := `
	let i = []
	try(function() { i + 3 }, function(e) { e.message() })
	`
	result, err := run(context.Background(), code)
	require.NoError(t, err)
	require.Equal(t, object.NewString("type error: unsupported operation for list: + on type int"), result)
}

func TestTryWithErrorValues(t *testing.T) {
	code := `
	const myerr = errors.new("errno == 1")
	try(function() {
		print("testing 1 2 3")
		error(myerr)
	}, function(e) {
		return e == myerr ? "YES" : "NO"
	})`
	result, err := run(context.Background(), code)
	require.NoError(t, err)
	require.Equal(t, object.NewString("YES"), result)
}

func TestTryWithLoop(t *testing.T) {
	code := `
	let result = []
	for let i = 0; i < 5; i++ {
		let value = try(
			function() { if i % 2 == 0 { error("Even number") } else { return i } },
			function(e) { return e.message() }
		)
		result.append(value)
	}
	result
	`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewString("Even number"),
		object.NewInt(1),
		object.NewString("Even number"),
		object.NewInt(3),
		object.NewString("Even number"),
	})
	require.Equal(t, expected, result)
}

func TestTryWithClosure(t *testing.T) {
	code := `
	function makeCounter() {
		let count = 0
		return function() {
			count++
			if count > 3 {
				error("Count exceeded")
			}
			return count
		}
	}
	let counter = makeCounter()
	let result = []
	for let i = 0; i < 5; i++ {
		let value = try(counter, function(e) { return e.message() })
		result.append(value)
	}
	result
	`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
		object.NewString("Count exceeded"),
		object.NewString("Count exceeded"),
	})
	require.Equal(t, expected, result)
}

func TestStringTemplateWithRaisedError(t *testing.T) {
	code := "`the err string is: ${error(\"oops\")}. sad!`"
	_, err := run(context.Background(), code)
	require.NotNil(t, err)
	require.Equal(t, "oops", err.Error())
}

func TestStringTemplateWithNonRaisedError(t *testing.T) {
	code := "`the err string is: ${errors.new(\"oops\")}. sad!`"
	result, err := run(context.Background(), code)
	require.NoError(t, err)
	require.Equal(t, object.NewString("the err string is: oops. sad!"), result)
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
		{`range [1]`, object.NewListIter(object.NewList([]object.Object{object.NewInt(1)}))},
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
		{"let a = 1; let b = \"ok\"; `${a + 1}-${b | strings.to_upper}`", object.NewString("2-OK")},
		{"function(a, b) { return `A: ${a} B: ${b}` }(\"hi\", \"bye\")", object.NewString("A: hi B: bye")},
	}
	runTests(t, tests)
}

func TestPipes(t *testing.T) {
	tests := []testCase{
		{`"hello" | strings.to_upper`, object.NewString("HELLO")},
		{`"hello" | len`, object.NewInt(5)},
		{`function() { "hello" }() | len`, object.NewInt(5)},
		{`["a", "b"] | strings.join(",") | strings.to_upper`, object.NewString("A,B")},
		{`function() { "a" } | call`, object.NewString("a")},
		{`"abc" | getattr("to_upper") | call`, object.NewString("ABC")},
		{`"abc" | function(s) { s.to_upper() }`, object.NewString("ABC")},
		{`[11, 12, 3] | math.sum`, object.NewFloat(26)},
		{`"42" | json.unmarshal`, object.NewFloat(42)},
	}
	runTests(t, tests)
}

func TestPipeForward(t *testing.T) {
	tests := []testCase{
		// Basic pipe forward
		{`"hello" |> strings.to_upper`, object.NewString("HELLO")},
		{`"hello" |> len`, object.NewInt(5)},
		{`[1, 2, 3] |> len`, object.NewInt(3)},
		// Chained pipe forward
		{`"hello" |> strings.to_upper |> len`, object.NewInt(5)},
		// With functions
		{`function() { "hello" }() |> len`, object.NewInt(5)},
		{`"abc" |> strings.to_upper`, object.NewString("ABC")},
		// With lambdas
		{`5 |> (x => x * 2)`, object.NewInt(10)},
		{`5 |> (x => x * 2) |> (x => x + 1)`, object.NewInt(11)},
		// With math functions
		{`[1, 2, 3] |> math.sum`, object.NewFloat(6)},
		// Combining with lambdas for multi-arg functions
		{`[1, 2, 3] |> (x => x.filter(y => y > 1)) |> len`, object.NewInt(2)},
	}
	runTests(t, tests)
}

func TestQuicksort(t *testing.T) {
	result, err := run(context.Background(), `
	function quicksort(arr) {
		if len(arr) < 2 {
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
	require.Nil(t, err)
	require.Equal(t, object.NewList(
		[]object.Object{
			object.NewInt(2),
			object.NewInt(3),
			object.NewInt(5),
			object.NewInt(10),
		}), result)
}

func TestMergesort(t *testing.T) {
	result, err := run(context.Background(), `
	function mergesort(arr) {
		let length = len(arr)
		if length <= 1 {
			return arr
		}
		let mid = length / 2
		let left = mergesort(arr[:mid])
		let right = mergesort(arr[mid:])
		let output = list(length)
		let i, j, k = [0, 0, 0]
		for i < len(left) {
			for j < len(right) && right[j] <= left[i] {
				output[k] = right[j]
				k++
				j++
			}
			output[k] = left[i]
			k++
			i++
		}
		for j < len(right) {
			output[k] = right[j]
			k++
			j++
		}
		return output
	}
	", ".join(mergesort([1, 9, -1, 4, 3, 2, 7, 8, 5, 6, 0]).map(string))
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewString("-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9"), result)
}

func TestRecursiveIsPrime(t *testing.T) {
	result, err := run(context.Background(), `
	function is_prime(n, i=2) {
		// Base cases
		if (n <= 2) { return n == 2 }
		if (n % i == 0) { return false }
		if (i * i > n) { return true }
		// Check for next divisor
    	return is_prime(n, i + 1);
	}
	let ints = []
	for let i = 1; i < 30; i++ { ints.append(i) }
	let primes = ints.filter(is_prime)
	", ".join(primes.map(string))
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewString("2, 3, 5, 7, 11, 13, 17, 19, 23, 29"), result)
}

func TestAndShortCircuit(t *testing.T) {
	// AND should short-circuit, so data[5] should not be evaluated
	result, err := run(context.Background(), `
	let data = []
	if len(data) && data[5] {
		"nope!"
	} else {
		"worked!"
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewString("worked!"), result)
}

func TestOrShortCircuit(t *testing.T) {
	// OR should short-circuit, so data[5] should not be evaluated
	result, err := run(context.Background(), `
	let data = [1]
	if len(data) || data[5] {
		"worked!"
	} else {
		"nope!"
	}
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewString("worked!"), result)
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
		{`let items = ["a", "b"]; print(...items)`, object.Nil},
	}
	runTests(t, tests)
}

func TestRestParameter(t *testing.T) {
	tests := []testCase{
		// Basic rest parameter
		{`function sum(...nums) { let t = 0; for n in nums { t = t + n }; return t }; sum(1, 2, 3)`, object.NewInt(6)},
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

func TestLoopBreak(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for let i = 0; i < 10; i++ {
		if i == 3 { break }
		x = i
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(2), result)
}

func TestLoopContinue(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for let i = 0; i < 10; i++ {
		if i > 3 { continue }
		x = i
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), result)
}

func TestRangeLoopBreak(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for let i = range [0, 1, 2, 3, 4] {
		if i == 3 { break }
		x = i
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(2), result)
}

func TestRangeLoopContinue(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for let i = range [0, 1, 2, 3, 4] {
		if i > 3 { continue }
		x = i
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), result)
}

func TestForCondition(t *testing.T) {
	result, err := run(context.Background(), `
	let c = true
	let count = 0
	for c {
		count++
		if count == 10 {
			c = false
		}
	}
	count
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(10), result)
}

func TestForIntCondition(t *testing.T) {
	result, err := run(context.Background(), `
	let count = 10
	for count { count-- }
	count
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(0), result)
}

func TestForExprCondition(t *testing.T) {
	result, err := run(context.Background(), `
	let count = 10
	for (count >= 5) { count-- }
	count
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), result)
}

func TestInvalidForCondition(t *testing.T) {
	result, err := run(context.Background(), `
	let count = 10
	for let x = 2 { count-- }
	count
	`)
	require.NoError(t, err)
	require.Equal(t, object.NewInt(8), result)
}

func TestSimpleLoopBreak(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	for {
		x++
		if x == 2 { break }
		let max = math.max(1, 2) // inject some extra instructions
	}
	x
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(2), result)
}

func TestNestedLoops(t *testing.T) {
	result, err := run(context.Background(), `
	let x, y, z = [0, 0, 0]
	for {
		x++ // This should execute 3 times total
		if x == 3 { break }
		// We should reach this point twice, with x as 1 then 2
		for let i = range [0, 1, 2, 3] {
			y++ // This should execute 8 times total
			if i > 1 { continue }
			// We should reach this point 4 times total
			for let h = 0; h < 10; h++ {
				z++ // This should execute 16 times total (4 times per inner loop)
				if h == 3 { break }
			}
		}
	}
	[x, y, z]
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(3),
		object.NewInt(8),
		object.NewInt(16),
	}), result)
}

func TestSimpleLoopContinue(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 0
	let y = 0
	for {
		x++
		if x < 2 { continue }
		// We'll reach here on x in [2, 3, 4, 5, 6]
		if x > 5 { break }
		// We'll reach here on x in [2, 3, 4, 5]; so y should increment 4 times
		y++
		let max = math.max(1, 2) // inject some extra instructions
	}
	y
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), result)
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
	require.Nil(t, err)
	require.Equal(t, object.NewInt(12), result)
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
		require.NotNil(t, err)
		require.Equal(t, tt.expectedErr, err.Error())
	}
}

type testData struct {
	Count int
}

func (t *testData) Increment() {
	t.Count++
}

func (t testData) GetCount() int {
	return t.Count
}

type testStruct struct {
	A int
	B string
	C *testData
}

func TestNestedProxies(t *testing.T) {
	s := &testStruct{
		A: 1,
		B: "foo",
		C: &testData{
			Count: 3,
		},
	}
	opts := runOpts{
		Globals: map[string]interface{}{"s": s},
	}
	result, err := run(context.Background(), `
	s.C.Increment()
	s.C.GetCount()
	`, opts)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), result)
}

func TestProxy(t *testing.T) {
	type test struct {
		Data []byte
	}
	opts := runOpts{
		Globals: map[string]interface{}{
			"s": &test{Data: []byte("foo")},
		},
	}
	result, err := run(context.Background(), `s.Data`, opts)
	require.Nil(t, err)
	require.Equal(t, object.NewByteSlice([]byte("foo")), result)
}

func TestHalt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()
	_, err := run(ctx, `for {}`)
	require.NotNil(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
}

func TestCallHalt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	vm, err := newVM(context.Background(), "function block() { for {} }")
	require.NoError(t, err)
	require.NoError(t, vm.Run(context.Background()))

	obj, err := vm.Get("block")
	require.NoError(t, err)

	fn, ok := obj.(*object.Function)
	require.True(t, ok)

	_, err = vm.Call(ctx, fn, nil)
	require.NotNil(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
}

func TestReturnGlobalVariable(t *testing.T) {
	result, err := run(context.Background(), `
	let x = 3
	function test() { x }
	test()
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), result)
}

func TestNakedReturn(t *testing.T) {
	result, err := run(context.Background(), `function test(a) { return }; test(15)`)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
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
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))

	globals := vm.GlobalNames()
	globalsMap := map[string]bool{}
	for _, g := range globals {
		globalsMap[g] = true
	}
	require.True(t, globalsMap["count"])
	require.True(t, globalsMap["inc"])
	require.True(t, globalsMap["m"])
	require.True(t, globalsMap["foo"])
}

func TestGetGlobal(t *testing.T) {
	ctx := context.Background()
	source := `function inc(a, b) { a + b }`
	vm, err := newVM(ctx, source)
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))

	obj, err := vm.Get("inc")
	require.Nil(t, err)
	fn, ok := obj.(*object.Function)
	require.True(t, ok)
	require.Equal(t, "inc", fn.Name())
}

func TestCall(t *testing.T) {
	ctx := context.Background()
	source := `function inc(a, b) { a + b }`
	vm, err := newVM(ctx, source)
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))

	obj, err := vm.Get("inc")
	require.Nil(t, err)
	fn, ok := obj.(*object.Function)
	require.True(t, ok)

	result, err := vm.Call(ctx, fn, []object.Object{
		object.NewInt(9),
		object.NewInt(1),
	})
	require.Nil(t, err)
	require.Equal(t, object.NewInt(10), result)
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
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))

	obj, err := vm.Get("counter")
	require.Nil(t, err)
	counter, ok := obj.(*object.Function)
	require.True(t, ok)

	// The counter's first value will be 11. Confirm it counts up from there.
	for i := int64(11); i < 100; i++ {
		obj, err := vm.Call(ctx, counter, []object.Object{})
		require.Nil(t, err)
		require.Equal(t, object.NewInt(i), obj)
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
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))
	result, ok := vm.TOS()
	require.True(t, ok)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(3),
		object.NewInt(4),
		object.NewInt(5),
	}), result)
}

func TestInterpolatedStringClosures1(t *testing.T) {
	ctx := context.Background()
	source := "function foo(a, b, c) {\n" +
		"	return function(d) {\n" +
		"		return `${strings.to_upper(a)}-${b}-${c}-${d}`\n" +
		"	}\n" +
		"}\n" +
		"foo(\"foo\", \"bar\", \"baz\")(\"go\")"
	vm, err := newVM(ctx, source)
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))
	result, ok := vm.TOS()
	require.True(t, ok)
	require.Equal(t, object.NewString("FOO-bar-baz-go"), result)
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
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))
	result, ok := vm.TOS()
	require.True(t, ok)
	require.Equal(t, object.NewString("a: HEY b: bar count: 40 x: 4"), result)
}

func TestClone(t *testing.T) {
	ctx := context.Background()
	source := `
	let x = 3
	function inc() {
		x++
	}
	inc()
	x
	`
	vm, err := newVM(ctx, source)
	require.Nil(t, err)
	require.Nil(t, vm.Run(ctx))
	result, ok := vm.TOS()
	require.True(t, ok)
	require.Equal(t, object.NewInt(4), result)

	clone, err := vm.Clone()
	require.Nil(t, err)
	value, err := clone.Get("x")
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), value)
}

func TestCloneWithAnonymousFunc(t *testing.T) {
	registered := map[string]*object.Function{}

	// Custom built-in function to be called from the Risor script to register
	// an anonymous function
	registerFunc := func(ctx context.Context, args ...object.Object) object.Object {
		name := args[0].(*object.String).Value()
		fn := args[1].(*object.Function)
		registered[name] = fn
		return object.Nil
	}

	ctx := context.Background()
	source := `
	let x = 3
	register("inc", function() {
		x++
		return x
	})
	`
	globals := map[string]any{
		"register": object.NewBuiltin("register", registerFunc),
	}
	machine, err := newVM(ctx, source, runOpts{Globals: globals})
	require.Nil(t, err)
	require.Nil(t, machine.Run(ctx))

	// x should be 3 in the original VM
	value, err := machine.Get("x")
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), value)

	// Confirm the "inc" function was registered
	incFunc, ok := registered["inc"]
	require.True(t, ok)

	// Create a clone of the VM and confirm it also has x = 3
	clone, err := machine.Clone()
	require.Nil(t, err)
	value, err = clone.Get("x")
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), value)

	// Call the "inc" function in the clone and confirm it increments x to 4
	// in both the clone and the original VM
	_, err = clone.Call(ctx, incFunc, nil)
	require.Nil(t, err)

	// Clone's x is now 4
	value, err = clone.Get("x")
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), value)

	// Original's x is now 4
	value, err = machine.Get("x")
	require.Nil(t, err)
	require.Equal(t, object.NewInt(4), value)
}

func TestIncrementalEvaluation(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "let x = 3")
	require.Nil(t, err)

	comp, err := compiler.New()
	require.Nil(t, err)
	main, err := comp.Compile(ast)
	require.Nil(t, err)

	v := New(main)
	require.Nil(t, v.Run(ctx))
	value, err := v.Get("x")
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), value)

	ast, err = parser.Parse(ctx, "x + 7")
	require.Nil(t, err)
	_, err = comp.Compile(ast)
	require.Nil(t, err)
	require.Nil(t, v.Run(ctx))
	value, err = v.Get("x")
	require.Nil(t, err)
	require.Equal(t, object.NewInt(3), value)

	tos, ok := v.TOS()
	require.True(t, ok)
	require.Equal(t, object.NewInt(10), tos)
}

func TestImports(t *testing.T) {
	tests := []testCase{
		{`import simple_math; simple_math.add(3, 4)`, object.NewInt(7)},
		{`import simple_math; int(simple_math.pi)`, object.NewInt(3)},
		{`import data; data.mydata["count"]`, object.NewInt(1)},
		{`import data; data.mydata["count"] = 3; data.mydata["count"]`, object.NewInt(3)},
		{`import data as d; d.mydata["count"]`, object.NewInt(1)},
		{`import math as m; m.min(3,-7)`, object.NewFloat(-7)},
	}
	runTests(t, tests)
}

func TestFromImport(t *testing.T) {
	tests := []testCase{
		{`from a.data import mapValue; mapValue["3"]`, object.NewInt(3)},
		{`from a.funcs import plusOne; plusOne(1)`, object.NewInt(2)},
		{`from a import funcs; funcs.plusOne(1)`, object.NewInt(2)},
		{`from a.b import data as b_data; from a.funcs import plusOne; plusOne(b_data.mapValue["1"]) `, object.NewInt(2)},
		{`from math import min; min(3,-7)`, object.NewFloat(-7)},
		{`from math import min as m; m(3,-7)`, object.NewFloat(-7)},
		{
			`from math import (min as a, max as b); [a(1,2), b(1,2)]`,
			object.NewList([]object.Object{
				object.NewFloat(1),
				object.NewFloat(2),
			}),
		},
	}
	runTests(t, tests)
}

func TestESStyleImport(t *testing.T) {
	tests := []testCase{
		{`import { min } from "math"; min(3, -7)`, object.NewFloat(-7)},
		{
			`import { min, max } from "math"; [min(1,2), max(1,2)]`,
			object.NewList([]object.Object{
				object.NewFloat(1),
				object.NewFloat(2),
			}),
		},
		{`import { min as m } from "math"; m(3, -7)`, object.NewFloat(-7)},
		{
			`import { min as a, max as b } from "math"; [a(1,2), b(1,2)]`,
			object.NewList([]object.Object{
				object.NewFloat(1),
				object.NewFloat(2),
			}),
		},
		{`import { round } from "math"; round(3.7)`, object.NewFloat(4)},
	}
	runTests(t, tests)
}

func TestBadImports(t *testing.T) {
	ctx := context.Background()
	type testCase struct {
		input     string
		expectErr string
	}
	tests := []testCase{
		{`import foo`, `import error: module "foo" not found`},
		{`import foo as bar`, `import error: module "foo" not found`},
		{`import math as`, `parse error: unexpected end of file while parsing an import statement (expected identifier)`},
		{`from foo import bar`, `import error: module "foo" not found`},
		{`from a.b import c`, `import error: module "a/b" not found`},
		{`from a.b import c as d`, `import error: module "a/b" not found`},
		{`from math import foo`, `import error: cannot import name "foo" from "math"`},
		{`from math`, `parse error: from-import is missing import statement`},
		{`from math import`, `parse error: unexpected end of file while parsing a from-import statement (expected identifier)`},
		{`from math import min as`, `parse error: unexpected end of file while parsing a from-import statement (expected identifier)`},
	}
	for _, tt := range tests {
		_, err := run(ctx, tt.input)
		require.NotNil(t, err)
		require.Equal(t, tt.expectErr, err.Error())
	}
}

func TestModifyModule(t *testing.T) {
	_, err := run(context.Background(), `math.max = 123`)
	require.Error(t, err)
	require.Equal(t, "type error: cannot modify module attributes", err.Error())
}

func TestEarlyForRangeReturn(t *testing.T) {
	code := `
function operation(c) {
	for range [1, 2, 3] {
		return "result"
	}
}
function main() {
	let items = ['ab', 'cd']
	let results = []
	for let _, item = range items {
		let value = operation(item)
		results.append(value)
	}
	return results
}
main()
`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewString("result"),
		object.NewString("result"),
	}), result)
}

func TestFreeVariables(t *testing.T) {
	code := `
	function test(count) {
		let l = []
		function() {
			let y = count
			if true {
				l.append(y)
			}
		}()
		return l
	}
	test(5)
	`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{object.NewInt(5)}), result)
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
		{`let l = [1, 2]; for let k = range l { l[k] *= 2 }; l`, object.NewList([]object.Object{
			object.NewInt(2),
			object.NewInt(4),
		})},
	}
	runTests(t, tests)
}

func TestFunctionStack(t *testing.T) {
	code := `
	for let i = range 1 {
		try(function() {
		  42
		  error("kaboom")
		})
	  }
	`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
}

func TestFunctionStackNewErr(t *testing.T) {
	code := `
	for let i = range 1 {
		try(function() {
		  42
		}, function(e) {
		  error("kaboom")
		})
	  }
	`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
}

func TestMultivar(t *testing.T) {
	code := `
	let x, y = [1, 2]
	[x, y]
	`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
	}), result)
}

func TestExecWithDir(t *testing.T) {
	code := `exec(["cat", "jabberwocky.txt"], {dir: "fixtures"}).stdout.split("\n")[0]`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	require.Equal(t, object.NewString("'Twas brillig, and the slithy toves"), result)
}

func TestExecOldWayWithDir(t *testing.T) {
	code := `exec("cat", ["jabberwocky.txt"], {dir: "fixtures"}).stdout.split("\n")[0]`
	result, err := run(context.Background(), code)
	require.Nil(t, err)
	require.Equal(t, object.NewString("'Twas brillig, and the slithy toves"), result)
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
	require.Nil(t, err)
	require.Equal(t, object.NewString("FOO"), result)
}

func TestContextDone(t *testing.T) {
	// Context with no deadline does not return a Done channel
	ctx := context.Background()
	d := ctx.Done()
	require.Nil(t, d)

	// Context with deadline returns a Done channel
	tctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	d = tctx.Done()
	require.NotNil(t, d)

	// Context with cancel returns a Done channel
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d = cctx.Done()
	require.NotNil(t, d)
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
			require.Nil(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestNestedRangeLoopBreak tests that breaking from an inner for-range loop in a nested loop
// doesn't cause stack overflow.
func TestNestedRangeLoopBreak(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "Basic case with single iteration",
			input: `
				let items1 = [1]
				let items2 = [1, 2]
				let count = 0
				for range items1 {
					for range items2 {
						count += 1
						break
					}
				}
				count
			`,
			expected: object.NewInt(1),
		},
		{
			name: "Multiple iterations in outer loop",
			input: `
				let items1 = [1, 2, 3]
				let items2 = [1, 2]
				let count = 0
				for range items1 {
					for range items2 {
						count += 1
						break
					}
				}
				count
			`,
			expected: object.NewInt(3),
		},
		{
			name: "Multiple iterations with indexed loop",
			input: `
				let items1 = [1, 2, 3]
				let items2 = [1, 2, 3]
				let result = []
				for let i, _ = range items1 {
					for let j, _ = range items2 {
						result = result + [[i, j]]
						break
					}
				}
				result
			`,
			expected: object.NewList([]object.Object{
				object.NewList([]object.Object{object.NewInt(0), object.NewInt(0)}),
				object.NewList([]object.Object{object.NewInt(1), object.NewInt(0)}),
				object.NewList([]object.Object{object.NewInt(2), object.NewInt(0)}),
			}),
		},
		{
			name: "Many iterations to ensure no stack overflow",
			input: `
				let count = 0
				for range 100 {
					for range 33 {
						count += 1
						break
					}
				}
				count
			`,
			expected: object.NewInt(100),
		},
		{
			name: "Break from single loop",
			input: `
				let count = 0
				for range 100 {
					count += 77
					break
					let x = "should not be here"
				}
				count
			`,
			expected: object.NewInt(77),
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(ctx, tt.input)
			require.Nil(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestDeeplyNestedRangeLoopBreak tests deeply nested for-range loops with breaks
// to ensure stack doesn't overflow
func TestDeeplyNestedRangeLoopBreak(t *testing.T) {
	// This test creates a deeply nested set of for-range loops (3 levels)
	// with break statements at each level
	input := `
		let count = 0
		// Create a more complex nesting case with multiple breaks
		for let i = range 5 {
			for let j = range 5 {
				if j == 3 {
					break  // Break from j loop
				}
				for let k = range 5 {
					count += 1
					if k == 2 {
						break  // Break from k loop
					}
				}
			}
		}
		count
	`

	ctx := context.Background()
	result, err := run(ctx, input)
	require.Nil(t, err)

	// We should get 5 (outer loops) * 3 (middle loops before break) * 3 (inner loops before break) = 45
	require.Equal(t, object.NewInt(45), result)
}

func TestClonedVMOS(t *testing.T) {
	code := `os.stdout.write("hello\n")`
	ctx := context.Background()
	ast, err := parser.Parse(ctx, code)
	require.Nil(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	main, err := compiler.Compile(ast, compiler.WithGlobalNames(globalNames))
	require.Nil(t, err)

	stdout := ros.NewBufferFile([]byte{})
	vos := ros.NewVirtualOS(ctx, ros.WithStdout(stdout))

	vm1 := New(main, WithOS(vos), WithGlobals(globals))
	require.Nil(t, vm1.Run(ctx))
	require.Equal(t, "hello\n", string(stdout.Bytes()))

	vm2, err := vm1.Clone()
	require.Nil(t, err)
	require.Nil(t, vm2.Run(ctx))
	require.Equal(t, "hello\nhello\n", string(stdout.Bytes()))
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
			if op == "add" {
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
			if n == 0 {
				return true
			}
			return is_odd(n - 1)
		}
		
		function is_odd(n) {
			if n == 0 {
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
			if n <= 0 {
				return 0
			}
			return n + countdown_b(n - 1)
		}
		
		function countdown_b(n) {
			if n <= 0 {
				return 0
			}
			return n + countdown_a(n - 1)
		}
		
		countdown_a(5)
		`, object.NewInt(15)},

		// More complex mutual recursion
		{`
		function fibonacci_a(n) {
			if n <= 1 {
				return n
			}
			return fibonacci_b(n - 1) + fibonacci_a(n - 2)
		}
		
		function fibonacci_b(n) {
			if n <= 1 {
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
			if x > 10 {
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
			switch op {
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

func TestForwardDeclarationWithLoops(t *testing.T) {
	tests := []testCase{
		// Forward declaration with for loops
		{`
		function sum_with_helper(n) {
			let total = 0
			for let i = 1; i <= n; i++ {
				total += process_number(i)
			}
			return total
		}
		
		function process_number(x) {
			return x * 2
		}
		
		sum_with_helper(5)
		`, object.NewInt(30)},

		// Forward declaration with range loops
		{`
		function process_list(items) {
			let result = []
			for let _, item = range items {
				result.append(transform_item(item))
			}
			return result
		}
		
		function transform_item(x) {
			return x + 10
		}
		
		process_list([1, 2, 3])
		`, object.NewList([]object.Object{
			object.NewInt(11),
			object.NewInt(12),
			object.NewInt(13),
		})},
	}
	runTests(t, tests)
}

func TestComplexForwardDeclarationScenarios(t *testing.T) {
	tests := []testCase{
		// Multiple forward declarations with dependencies
		{`
		function main_processor() {
			let data = prepare_data()
			let processed = process_data(data)
			return finalize_data(processed)
		}
		
		function prepare_data() {
			return [1, 2, 3, 4, 5]
		}
		
		function process_data(items) {
			let result = []
			for let _, item = range items {
				result.append(transform_value(item))
			}
			return result
		}
		
		function transform_value(x) {
			return multiply_by_factor(x, 3)
		}
		
		function multiply_by_factor(value, factor) {
			return value * factor
		}
		
		function finalize_data(items) {
			return calculate_sum(items)
		}
		
		function calculate_sum(items) {
			let total = 0
			for let _, item = range items {
				total += item
			}
			return total
		}
		
		main_processor()
		`, object.NewInt(45)},

		// Forward declaration with error handling
		{`
		function safe_processor(x) {
			let result = try(
				function() { return risky_operation(x) },
				function(e) { return fallback_operation(x) }
			)
			return result
		}
		
		function risky_operation(x) {
			if x < 0 {
				error("negative number")
			}
			return x * 2
		}
		
		function fallback_operation(x) {
			return 0
		}
		
		[safe_processor(5), safe_processor(-5)]
		`, object.NewList([]object.Object{
			object.NewInt(10),
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
			require.NotNil(t, err)
			require.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestRunCode(t *testing.T) {
	ctx := context.Background()

	// Create a VM with initial code
	vm, err := newVM(ctx, "let x = 10; let y = 20; x + y")
	require.NoError(t, err)

	// Run the initial code
	require.NoError(t, vm.Run(ctx))

	result, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result.(*object.Int).Value(), int64(30))

	// Compile and run different code on the same VM
	ast2, err := parser.Parse(ctx, "let a = 5; let b = 15; a * b")
	require.NoError(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, compiler.WithGlobalNames(globalNames))
	require.NoError(t, err)

	// Run the second code on the same VM
	require.NoError(t, vm.RunCode(ctx, code2))

	result2, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result2.(*object.Int).Value(), int64(75))

	// Run a third piece of code
	source3 := `
		let name = "Risor"
		let greeting = "Hello, " + name + "!"
		greeting
	`
	ast3, err := parser.Parse(ctx, source3)
	require.NoError(t, err)

	code3, err := compiler.Compile(ast3, compiler.WithGlobalNames(globalNames))
	require.NoError(t, err)
	require.NoError(t, vm.RunCode(ctx, code3))

	result3, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result3.(*object.String).Value(), "Hello, Risor!")
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
	require.NoError(t, err)
	require.NoError(t, vm.Run(ctx))

	result, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result.(*object.Int).Value(), int64(200))

	// Run different code that also uses globals
	source2 := `
		let newResult = baseValue + multiplier
		newResult
	`
	ast2, err := parser.Parse(ctx, source2)
	require.NoError(t, err)

	var globalNames []string
	for k := range customGlobals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, compiler.WithGlobalNames(globalNames))
	require.NoError(t, err)
	require.NoError(t, vm.RunCode(ctx, code2))

	result2, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result2.(*object.Int).Value(), int64(102))
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
	require.NoError(t, err)
	require.NoError(t, vm.Run(ctx))

	result, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result.(*object.Int).Value(), int64(30))

	// Run code with a different function
	source2 := `
		function multiply(x, y) {
			return x * y
		}
		multiply(6, 7)
	`
	ast2, err := parser.Parse(ctx, source2)
	require.NoError(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, compiler.WithGlobalNames(globalNames))
	require.NoError(t, err)
	require.NoError(t, vm.RunCode(ctx, code2))

	result2, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result2.(*object.Int).Value(), int64(42))
}

func TestRunCodeOnVM(t *testing.T) {
	ctx := context.Background()

	// Create a VM with initial code
	vm, err := newVM(ctx, "let x = 42; x")
	require.NoError(t, err)
	require.NoError(t, vm.Run(ctx))

	// Compile a different piece of code
	ast2, err := parser.Parse(ctx, "let y = 100; let z = 200; y + z")
	require.NoError(t, err)

	globals := basicBuiltins()
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}

	code2, err := compiler.Compile(ast2, compiler.WithGlobalNames(globalNames))
	require.NoError(t, err)
	result, err := RunCodeOnVM(ctx, vm, code2)
	require.NoError(t, err)
	require.Equal(t, result.(*object.Int).Value(), int64(300))
}

func TestRunCodeFirst(t *testing.T) {
	ctx := context.Background()
	vm, err := newVM(ctx, `
		function add(a, b) { return a + b }
		add(10, 20)
	`)
	require.NoError(t, err)
	require.NoError(t, vm.RunCode(ctx, vm.main))
	result, exists := vm.TOS()
	require.True(t, exists)
	require.Equal(t, result.(*object.Int).Value(), int64(30))
}

func TestNewEmpty(t *testing.T) {
	ctx := context.Background()
	compile := func(source string) *compiler.Code {
		ast, err := parser.Parse(ctx, source)
		require.NoError(t, err)
		code, err := compiler.Compile(ast)
		require.NoError(t, err)
		return code
	}

	// Test creating a VM without main code
	vm, err := NewEmpty()
	require.NoError(t, err)

	// Test that Run() returns an error when no main code is provided
	err = vm.Run(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, "no main code available")

	// Test that RunCode() works with specific code
	code := compile(`let x = 42; x`)
	err = vm.RunCode(ctx, code)
	require.NoError(t, err)

	// Verify the result is on the stack
	result, ok := vm.TOS()
	require.True(t, ok)
	intResult, ok := result.(*object.Int)
	require.True(t, ok)
	require.Equal(t, intResult.Value(), int64(42))

	// Test that Call() works with functions
	fnCode := compile(`function add(a, b) { return a + b }`)
	err = vm.RunCode(ctx, fnCode)
	require.NoError(t, err)

	addFn, err := vm.Get("add")
	require.NoError(t, err)

	result, err = vm.Call(ctx, addFn.(*object.Function), []object.Object{
		object.NewInt(10),
		object.NewInt(20),
	})
	require.NoError(t, err)

	intResult, ok = result.(*object.Int)
	require.True(t, ok)
	require.Equal(t, intResult.Value(), int64(30))
}

func TestNewEmptyClone(t *testing.T) {
	ctx := context.Background()
	compile := func(source string) *compiler.Code {
		ast, err := parser.Parse(ctx, source)
		require.NoError(t, err)
		code, err := compiler.Compile(ast)
		require.NoError(t, err)
		return code
	}

	// Test cloning a VM without main code
	vm, err := NewEmpty()
	require.NoError(t, err)

	// Run some code to set up state
	code := compile(`let x = 100`)
	err = vm.RunCode(ctx, code)
	require.NoError(t, err)

	// Clone the VM
	clone, err := vm.Clone()
	require.NoError(t, err)

	// Verify the clone also has no main code
	require.Nil(t, clone.main)

	// Verify Run() fails on clone too
	err = clone.Run(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, "no main code available")

	// Verify RunCode() works on clone
	newCode := compile(`let y = 200; y`)
	err = clone.RunCode(ctx, newCode)
	require.NoError(t, err)

	// Verify result
	result, ok := clone.TOS()
	require.True(t, ok)
	intResult, ok := result.(*object.Int)
	require.True(t, ok)
	require.Equal(t, intResult.Value(), int64(200))
}

func TestForIn1(t *testing.T) {
	result, err := run(context.Background(), `
	let sum = 0
	for x in [1, 2, 3] {
		sum = sum + x
	}
	sum
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(6), result)
}

func TestForIn2(t *testing.T) {
	result, err := run(context.Background(), `
	let fruits = ["apple", "banana", "cherry"]
	let last = ""
	for fruit in fruits {
		last = fruit
	}
	last
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewString("cherry"), result)
}

func TestForIn3(t *testing.T) {
	result, err := run(context.Background(), `
	let items = []
	for x in [10, 20, 30] {
		items.append(x * 2)
	}
	items
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(20),
		object.NewInt(40),
		object.NewInt(60),
	}), result)
}

func TestForInString(t *testing.T) {
	result, err := run(context.Background(), `
	let chars = []
	for c in "hello" {
		chars.append(c)
	}
	chars
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewString("h"),
		object.NewString("e"),
		object.NewString("l"),
		object.NewString("l"),
		object.NewString("o"),
	}), result)
}

func TestForInBreakContinue(t *testing.T) {
	result, err := run(context.Background(), `
	let sum = 0
	for x in [1, 2, 3, 4, 5] {
		if x == 3 {
			continue
		}
		if x == 5 {
			break
		}
		sum = sum + x
	}
	sum
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(7), result) // 1 + 2 + 4, skipping 3 and breaking before 5
}

func TestForInWithMaps(t *testing.T) {
	result, err := run(context.Background(), `
	let data = {a: 1, b: 2, c: 3}
	let result = []
	for key in data {
		result.append(key)
	}
	result.sort()
	result
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	}), result)
}

func TestForInWithRangeFunction(t *testing.T) {
	result, err := run(context.Background(), `
	let total = 0
	for i in range(5) {
		total = total + i
	}
	total
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(10), result) // 0+1+2+3+4 = 10
}

func TestForInNestedLoops(t *testing.T) {
	result, err := run(context.Background(), `
	let pairs = []
	for x in [1, 2] {
		for y in [3, 4] {
			pairs.append([x, y])
		}
	}
	pairs
	`)
	require.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewList([]object.Object{object.NewInt(1), object.NewInt(3)}),
		object.NewList([]object.Object{object.NewInt(1), object.NewInt(4)}),
		object.NewList([]object.Object{object.NewInt(2), object.NewInt(3)}),
		object.NewList([]object.Object{object.NewInt(2), object.NewInt(4)}),
	})
	require.Equal(t, expected, result)
}

func TestForInWithComplexExpressions(t *testing.T) {
	result, err := run(context.Background(), `
	let data = [[1, 2], [3, 4], [5, 6]]
	let sum = 0
	for row in data {
		for val in row {
			sum = sum + val
		}
	}
	sum
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(21), result) // 1+2+3+4+5+6 = 21
}

func TestForInWithFunctionCalls(t *testing.T) {
	result, err := run(context.Background(), `
	let getData = function() { 
		return [10, 20, 30] 
	}
	let sum = 0
	for val in getData() {
		sum = sum + val
	}
	sum
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(60), result) // 10+20+30 = 60
}

func TestForInWithMethodChaining(t *testing.T) {
	result, err := run(context.Background(), `
	let getData = function() { return [1, 2, 3] }
	let sum = 0
	for val in getData() {
		sum = sum + val
	}
	sum
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(6), result) // 1+2+3 = 6
}

func TestForInEmptyIterable(t *testing.T) {
	result, err := run(context.Background(), `
	let count = 0
	for x in [] {
		count++
	}
	count
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(0), result)
}

func TestForInSingleElement(t *testing.T) {
	result, err := run(context.Background(), `
	let value = nil
	for x in [42] {
		value = x
	}
	value
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewInt(42), result)
}

func TestForInVariableScoping(t *testing.T) {
	result, err := run(context.Background(), `
	let x = "outer"
	for x in ["inner"] {
		// x is now "inner" in loop scope
	}
	x  // Should still be "outer" after loop
	`)
	require.Nil(t, err)
	require.Equal(t, object.NewString("outer"), result)
}

func TestForInWithDifferentTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "iterate over mixed types",
			input: `
			let result = []
			for item in [1, "hello", true, 3.14] {
				result.append(item)
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewInt(1),
				object.NewString("hello"),
				object.True,
				object.NewFloat(3.14),
			}),
		},
		{
			name: "iterate over string bytes",
			input: `
			let result = []
			for b in "abc" {
				result.append(b)
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewString("a"), // string character
				object.NewString("b"), // string character
				object.NewString("c"), // string character
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			require.Nil(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestForInErrorConditions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name: "non-iterable object",
			input: `
			for x in true {
				x
			}
			`,
			expectError: true,
		},
		{
			name: "nil iterable",
			input: `
			let data = nil
			for x in data {
				x
			}
			`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(context.Background(), tt.input)
			if tt.expectError {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
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
			require.Nil(t, err, "unexpected error: %v", err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMultiVariableForIn(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected object.Object
	}{
		{
			name: "for i, v in array",
			input: `
			let result = []
			for i, v in [10, 20, 30] {
				result.append([i, v])
			}
			result
			`,
			expected: object.NewList([]object.Object{
				object.NewList([]object.Object{object.NewInt(0), object.NewInt(10)}),
				object.NewList([]object.Object{object.NewInt(1), object.NewInt(20)}),
				object.NewList([]object.Object{object.NewInt(2), object.NewInt(30)}),
			}),
		},
		{
			name: "for k, v in map",
			input: `
			let m = {"a": 1, "b": 2}
			let mapKeys = []
			let mapVals = []
			for k, v in m {
				mapKeys.append(k)
				mapVals.append(v)
			}
			[len(mapKeys), len(mapVals)]
			`,
			expected: object.NewList([]object.Object{object.NewInt(2), object.NewInt(2)}),
		},
		{
			name: "single variable for-in still works",
			input: `
			let sum = 0
			for v in [1, 2, 3] {
				sum = sum + v
			}
			sum
			`,
			expected: object.NewInt(6),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := run(context.Background(), tt.input)
			require.Nil(t, err, "unexpected error: %v", err)
			require.Equal(t, tt.expected, result)
		})
	}
}
