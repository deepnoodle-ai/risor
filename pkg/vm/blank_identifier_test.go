package vm

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

// A catch clause may name the blank identifier to discard the exception.
func TestCatchBlankIdentifier(t *testing.T) {
	runTests(t, []testCase{
		{`try { throw "boom" } catch _ { 42 }`, object.NewInt(42)},
		{`function f() { return try { throw "boom" } catch _ { 7 } }; f()`, object.NewInt(7)},
		{`try { throw "boom" } catch _ { 1 } finally { 2 }`, object.NewInt(1)},
		{`try { 5 } catch _ { 1 }`, object.NewInt(5)},
	})
}

// A function named with the blank identifier is an anonymous function: the
// name binds nothing.
func TestFunctionNamedBlankIdentifier(t *testing.T) {
	runTests(t, []testCase{
		{`function _() { return 1 }; 2`, object.NewInt(2)},
		{`let f = function _() { return 1 }; f()`, object.NewInt(1)},
		{`function outer() { function _() { return 1 }; return 3 }; outer()`, object.NewInt(3)},
		{`function outer() { let f = function _(a) { return a }; return f(4) }; outer()`, object.NewInt(4)},
		{`let f = function _(a, ...rest) { return len(rest) }; f(1, 2, 3)`, object.NewInt(2)},
	})
}
