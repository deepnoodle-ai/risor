package main

import (
	"context"
	"testing"

	"github.com/risor-io/risor"
	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, int64(5), result)

	// Define a variable
	_, err = vm.Eval(context.Background(), "let x = 10")
	assert.Nil(t, err)

	// Get the variable
	x, err := vm.Get("x")
	assert.Nil(t, err)
	assert.Equal(t, int64(10), x)
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
	assert.NotNil(t, err, "expected division by zero error")

	// Now execute valid code - should not repeat the previous error
	result, err := vm.Eval(context.Background(), "x + 10")
	assert.Nil(t, err, "expected no error after recovery")
	assert.Equal(t, int64(15), result, "expected x + 10 = 15")

	// Verify we can still define and use new variables
	_, err = vm.Eval(context.Background(), "let y = 20")
	assert.Nil(t, err)

	result, err = vm.Eval(context.Background(), "x + y")
	assert.Nil(t, err)
	assert.Equal(t, int64(25), result, "expected x + y = 25")
}
