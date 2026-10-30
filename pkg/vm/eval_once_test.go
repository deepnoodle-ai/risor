package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Compound assignment and postfix operators evaluate the receiver and index
// expressions exactly once.
func TestCompoundAssignmentEvaluatesOnce(t *testing.T) {
	prelude := `
	let calls = 0
	let arr = [10, 20, 30]
	let obj = {x: 1}
	function idx() { calls++; return calls - 1 }
	function get() { calls++; return obj }
	`
	tests := []testCase{
		{prelude + `arr[idx()] += 5; [arr, calls]`, object.NewList([]object.Object{
			object.NewList([]object.Object{object.NewInt(15), object.NewInt(20), object.NewInt(30)}),
			object.NewInt(1),
		})},
		{prelude + `arr[idx()] *= 2; [arr, calls]`, object.NewList([]object.Object{
			object.NewList([]object.Object{object.NewInt(20), object.NewInt(20), object.NewInt(30)}),
			object.NewInt(1),
		})},
		{prelude + `arr[idx()]++; [arr, calls]`, object.NewList([]object.Object{
			object.NewList([]object.Object{object.NewInt(11), object.NewInt(20), object.NewInt(30)}),
			object.NewInt(1),
		})},
		{prelude + `arr[idx()]--; [arr, calls]`, object.NewList([]object.Object{
			object.NewList([]object.Object{object.NewInt(9), object.NewInt(20), object.NewInt(30)}),
			object.NewInt(1),
		})},
		{prelude + `let lists = [arr]; function first() { calls++; return lists[0] }; first()[1] += 1; [arr, calls]`, object.NewList([]object.Object{
			object.NewList([]object.Object{object.NewInt(10), object.NewInt(21), object.NewInt(30)}),
			object.NewInt(1),
		})},
		{prelude + `get().x += 1; [obj.x, calls]`, object.NewList([]object.Object{
			object.NewInt(2), object.NewInt(1),
		})},
		{prelude + `get().x++; [obj.x, calls]`, object.NewList([]object.Object{
			object.NewInt(2), object.NewInt(1),
		})},
		{prelude + `get().x--; [obj.x, calls]`, object.NewList([]object.Object{
			object.NewInt(0), object.NewInt(1),
		})},
		// Plain receivers still work, and nothing is left on the stack
		{`let m = {a: [1, 2]}; m.a[0] += 10; m.a[1]++; m.a`, object.NewList([]object.Object{
			object.NewInt(11), object.NewInt(3),
		})},
		{`[1, 2, 3].map(i => { let a = [i]; a[0] += i; a[0]++; let o = {v: i}; o.v *= 10; a[0] + o.v })`, object.NewList([]object.Object{
			object.NewInt(13), object.NewInt(25), object.NewInt(37),
		})},
	}
	runTests(t, tests)
}

// x |> obj?.method() with a nil obj short-circuits to nil instead of
// calling nil.
func TestPipeOptionalMethodCall(t *testing.T) {
	tests := []testCase{
		{`let obj = nil; 5 |> obj?.double()`, object.Nil},
		{`let obj = {double: x => x * 2}; 5 |> obj?.double()`, object.NewInt(10)},
		{`let obj = {add: (a, b) => a + b}; 5 |> obj?.add(1)`, object.NewInt(6)},
		{`let obj = nil; let calls = 0; function inc(x) { calls++; return x }; 5 |> obj?.double() |> inc; calls`, object.NewInt(1)},
		{`let obj = nil; (5 |> obj?.double()) ?? "none"`, object.NewString("none")},
	}
	runTests(t, tests)
}

// obj?.attr returns nil for a missing attribute, as the README promises for
// "config?.defaults?.theme ?? \"light\"". obj?.method() still raises an
// error for a missing method on a non-nil object.
func TestOptionalAttrMissing(t *testing.T) {
	runTests(t, []testCase{
		{`let config = {}; config?.defaults?.theme ?? "light"`, object.NewString("light")},
		{`let config = {defaults: {}}; config?.defaults?.theme ?? "light"`, object.NewString("light")},
		{`let config = {defaults: {theme: "dark"}}; config?.defaults?.theme ?? "light"`, object.NewString("dark")},
		{`let obj = nil; obj?.missing`, object.Nil},
	})
	_, err := run(context.Background(), `let obj = {a: 1}; obj?.missing()`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Array destructuring defaults run left to right, and a default can refer
// to an earlier name.
func TestArrayDestructureDefaultOrder(t *testing.T) {
	tests := []testCase{
		{`let [x = 10, y = x + 1] = []; [x, y]`, object.NewList([]object.Object{
			object.NewInt(10), object.NewInt(11),
		})},
		{`let [x = 10, y = x + 1] = [1]; [x, y]`, object.NewList([]object.Object{
			object.NewInt(1), object.NewInt(2),
		})},
		{`let log = []; let [a = log.append(1), b = log.append(2)] = []; log`, object.NewList([]object.Object{
			object.NewInt(1), object.NewInt(2),
		})},
		{`let [a, _, c = 3] = [1, 2]; [a, c]`, object.NewList([]object.Object{
			object.NewInt(1), object.NewInt(3),
		})},
		{`function f() { let [x = 10, y = x + 1] = [5]; return [x, y] }; f()`, object.NewList([]object.Object{
			object.NewInt(5), object.NewInt(6),
		})},
		{`function f([x = 10, y = x + 1]) { return [x, y] }; f([])`, object.NewList([]object.Object{
			object.NewInt(10), object.NewInt(11),
		})},
		{`let f = ([a, b = a * 2, c = b + 1]) => [a, b, c]; f([1])`, object.NewList([]object.Object{
			object.NewInt(1), object.NewInt(2), object.NewInt(3),
		})},
	}
	runTests(t, tests)
}

// A destructuring default that refers to a later name fails the same way in
// let and in a function parameter: it is undefined, unless an outer variable
// has that name, in which case both forms read the outer variable.
func TestDestructureDefaultLaterName(t *testing.T) {
	ctx := context.Background()
	for _, src := range []string{
		`let [a = b, b = 1] = []`,
		`function f([a = b, b = 1]) { return [a, b] }; f([])`,
		`let f = ([a = b, b = 1]) => [a, b]; f([])`,
		`let {a = b, b = 1} = {}`,
		`function f({a = b, b = 1}) { return [a, b] }; f({})`,
		`function f([a = b], [b]) { return [a, b] }; f([], [1])`,
	} {
		t.Run(src, func(t *testing.T) {
			_, err := run(ctx, src)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), `undefined variable "b"`)
		})
	}
	five, one := object.NewInt(5), object.NewInt(1)
	runTests(t, []testCase{
		{`let b = 5; let g = () => { let [a = b, b = 1] = []; return [a, b] }; g()`, object.NewList([]object.Object{five, one})},
		{`let b = 5; function f([a = b, b = 1]) { return [a, b] }; f([])`, object.NewList([]object.Object{five, one})},
		{`let b = 5; function f({a = b, b = 1}) { return [a, b] }; f({})`, object.NewList([]object.Object{five, one})},
	})
}

// A named function with a destructured parameter can call itself.
func TestDestructuredParamRecursion(t *testing.T) {
	runTests(t, []testCase{
		{`function f([n]) { if (n > 0) { return f([n - 1]) + 1 }
		return 0 }
		f([3])`, object.NewInt(3)},
		{`function f({n}, ...rest) { if (n > 0) { return f({n: n - 1}) + 1 }
		return 0 }
		f({n: 2})`, object.NewInt(2)},
	})
}
