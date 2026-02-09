package main

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestReplVM(t *testing.T) {
	vm, err := newReplVM(risor.Builtins())
	assert.Nil(t, err)

	// Define a function
	_, err = vm.Eval(context.Background(), "function add(a, b) { a + b }")
	assert.Nil(t, err)

	// Call the function
	result, err := vm.Call(context.Background(), "add", int64(2), int64(3))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(5))

	// Define a variable
	_, err = vm.Eval(context.Background(), "let x = 10")
	assert.Nil(t, err)

	// Get the variable
	x, err := vm.Get("x")
	assert.Nil(t, err)
	assert.Equal(t, x, int64(10))
}

// TestReplVMErrorRecovery tests that the REPL VM can recover from errors
// and continue executing new code without repeating the error.
func TestReplVMErrorRecovery(t *testing.T) {
	vm, err := newReplVM(risor.Builtins())
	assert.Nil(t, err)

	// Execute some valid code first
	_, err = vm.Eval(context.Background(), "let x = 5")
	assert.Nil(t, err)

	// Execute code that causes a runtime error
	_, err = vm.Eval(context.Background(), "1 / 0")
	assert.NotNil(t, err)

	// Now execute valid code - should not repeat the previous error
	result, err := vm.Eval(context.Background(), "x + 10")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(15))

	// Verify we can still define and use new variables
	_, err = vm.Eval(context.Background(), "let y = 20")
	assert.Nil(t, err)

	result, err = vm.Eval(context.Background(), "x + y")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(25))
}

// TestReplVMCompileErrorCorruption tests that compile errors don't corrupt
// subsequent evaluations. This reproduces a bug where a compile error (e.g.
// referencing an undefined variable) caused the REPL to return wrong results
// for subsequent valid expressions.
//
// Observed behavior:
//
//	>>> [1,2,3].each(x => x*x)
//	>>> [1,2,3].each(x => print(x))
//	compile error: undefined variable "print"
//	>>> [1,2,3].filter(x => x < 3)
//	"builtin(list.each)" string        <-- WRONG: should be [1, 2]
//	>>> [1,2,3].filter(x => x < 3)
//	                                   <-- WRONG: should be [1, 2]
func TestReplVMCompileErrorCorruption(t *testing.T) {
	ctx := context.Background()

	// Use an environment without "print" so we can trigger a compile error
	env := risor.Builtins()
	delete(env, "print")

	vm, err := newReplVM(env)
	assert.Nil(t, err)

	// Successful expression: method call that returns nil (void)
	result, err := vm.Eval(ctx, "[1,2,3].each(x => x * x)")
	assert.Nil(t, err)

	// Trigger a compile error by referencing an undefined variable
	_, err = vm.Eval(ctx, "[1,2,3].each(x => print(x))")
	assert.NotNil(t, err)

	// This should return the filtered list, not "builtin(list.each)" or nil.
	// BUG: currently returns "builtin(list.each)" - a stale TOS value from
	// the .each() call, because the compile error left corrupted bytecode
	// that causes the stack to get out of sync.
	result, err = vm.Eval(ctx, "[1,2,3].filter(x => x < 3)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), int64(2)})

	// BUG: subsequent evals may also return wrong results or nil due to
	// the cascading state corruption.
	result, err = vm.Eval(ctx, "[1,2,3].map(x => x * 10)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(10), int64(20), int64(30)})
}

// TestReplVMCompileErrorPreservesState tests that compile errors don't destroy
// previously defined variables.
func TestReplVMCompileErrorPreservesState(t *testing.T) {
	ctx := context.Background()

	vm, err := newReplVM(risor.Builtins())
	assert.Nil(t, err)

	// Define some state
	_, err = vm.Eval(ctx, "let x = 42")
	assert.Nil(t, err)

	// Trigger a compile error
	_, err = vm.Eval(ctx, "undefined_var + 1")
	assert.NotNil(t, err)

	// Previously defined variable should still be accessible
	result, err := vm.Eval(ctx, "x")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(42))

	// Should be able to use it in expressions
	result, err = vm.Eval(ctx, "x + 8")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(50))
}

// TestReplVMMultipleCompileErrors tests recovery after several consecutive
// compile errors.
func TestReplVMMultipleCompileErrors(t *testing.T) {
	ctx := context.Background()

	vm, err := newReplVM(risor.Builtins())
	assert.Nil(t, err)

	// Several compile errors in a row
	_, err = vm.Eval(ctx, "foo")
	assert.NotNil(t, err)

	_, err = vm.Eval(ctx, "bar")
	assert.NotNil(t, err)

	_, err = vm.Eval(ctx, "baz + 1")
	assert.NotNil(t, err)

	// Should still work fine after multiple errors
	result, err := vm.Eval(ctx, "1 + 2")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(3))

	// Define and use variables
	_, err = vm.Eval(ctx, "let x = 100")
	assert.Nil(t, err)

	result, err = vm.Eval(ctx, "x")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(100))
}

// TestReplVMMethodCallsAfterError tests that method calls on collections
// work correctly after compile errors. This is a specific regression test
// for a bug where .filter() returned "builtin(list.each)" after errors.
func TestReplVMMethodCallsAfterError(t *testing.T) {
	ctx := context.Background()

	vm, err := newReplVM(risor.Builtins())
	assert.Nil(t, err)

	// Valid list operation
	result, err := vm.Eval(ctx, "[1,2,3].map(x => x * 2)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(2), int64(4), int64(6)})

	// Trigger compile error
	_, err = vm.Eval(ctx, "nonexistent()")
	assert.NotNil(t, err)

	// All list methods should return correct results
	result, err = vm.Eval(ctx, "[1,2,3].filter(x => x > 1)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(2), int64(3)})

	result, err = vm.Eval(ctx, "[1,2,3].map(x => x + 10)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(11), int64(12), int64(13)})
}

// TestReplVMTranscriptReproduction is a direct reproduction of the REPL
// transcript that demonstrated the bug. It follows the exact sequence of
// inputs that led to corrupted results.
func TestReplVMTranscriptReproduction(t *testing.T) {
	ctx := context.Background()

	// Use Builtins but remove "print" to match the observed environment
	env := risor.Builtins()
	delete(env, "print")

	vm, err := newReplVM(env)
	assert.Nil(t, err)

	// >>> let x = 42
	_, err = vm.Eval(ctx, "let x = 42")
	assert.Nil(t, err)

	// >>> x
	result, err := vm.Eval(ctx, "x")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(42))

	// >>> x + 9
	result, err = vm.Eval(ctx, "x + 9")
	assert.Nil(t, err)
	assert.Equal(t, result, int64(51))

	// >>> [1,2,3]
	result, err = vm.Eval(ctx, "[1,2,3]")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), int64(2), int64(3)})

	// >>> [1,2,3].each(x => x*x)
	// .each() is void - returns nil
	_, err = vm.Eval(ctx, "[1,2,3].each(x => x*x)")
	assert.Nil(t, err)

	// >>> [1,2,3].map(x => x*x)
	result, err = vm.Eval(ctx, "[1,2,3].map(x => x*x)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), int64(4), int64(9)})

	// >>> [1,2,3].each(x => print(x))
	// compile error: "print" is not in the environment
	_, err = vm.Eval(ctx, "[1,2,3].each(x => print(x))")
	assert.NotNil(t, err)

	// >>> [1,2,3].filter(x => x < 3)
	// BUG: returns "builtin(list.each)" instead of [1, 2]
	result, err = vm.Eval(ctx, "[1,2,3].filter(x => x < 3)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), int64(2)})

	// >>> [1,2,3].filter(x => x < 3) (again)
	// BUG: returns nil instead of [1, 2]
	result, err = vm.Eval(ctx, "[1,2,3].filter(x => x < 3)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{int64(1), int64(2)})

	// >>> [1,2,3].map(x => x < 3)
	result, err = vm.Eval(ctx, "[1,2,3].map(x => x < 3)")
	assert.Nil(t, err)
	assert.Equal(t, result, []any{true, true, false})
}
