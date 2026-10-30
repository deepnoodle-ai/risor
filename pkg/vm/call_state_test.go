package vm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Call applies the timeout set with WithTimeout, as Run does.
func TestCallAppliesTimeout(t *testing.T) {
	ctx := context.Background()
	vm, _ := newBuiltinsVM(t, `function spin() { range(1000000000).each(x => x) }`, nil,
		WithTimeout(50*time.Millisecond))
	assert.Nil(t, vm.Run(ctx))
	obj, err := vm.Get("spin")
	assert.Nil(t, err)

	// A safety net, so that the test fails instead of hanging
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err = vm.Call(ctx, obj.(*object.Closure), nil)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)
	assert.True(t, time.Since(start) < 2*time.Second, "took %v", time.Since(start))
}

// A run that fails in the middle of an expression leaves no values on the
// value stack, so a later Call has the whole stack.
func TestFailedRunClearsValueStack(t *testing.T) {
	ctx := context.Background()
	ones := strings.Repeat("1, ", 1000)
	vm, err := newVM(ctx, `function f() { return [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] }
	[`+ones+`1 + "bad"]`)
	assert.Nil(t, err)
	assert.NotNil(t, vm.Run(ctx))
	assert.Equal(t, vm.sp, -1)

	obj, err := vm.Get("f")
	assert.Nil(t, err)
	result, err := vm.Call(ctx, obj.(*object.Closure), nil)
	assert.Nil(t, err)
	assert.Equal(t, len(result.(*object.List).Value()), 10)
}

// The stack trace of an error in Call starts at the called function. It
// does not include the main code that an earlier Run finished.
func TestCallStackTraceOmitsMain(t *testing.T) {
	ctx := context.Background()
	vm, err := newVM(ctx, `function inner() { throw "boom" }
function outer() { inner() }`)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))
	obj, err := vm.Get("outer")
	assert.Nil(t, err)
	_, err = vm.Call(ctx, obj.(*object.Closure), nil)
	var se *object.StructuredError
	assert.True(t, errors.As(err, &se), "got %v", err)
	var names []string
	for _, frame := range se.Stack {
		names = append(names, frame.Function)
	}
	assert.Equal(t, names, []string{"inner", "outer"})

	// The VM still reads the globals of the main code after Call
	_, err = vm.Get("outer")
	assert.Nil(t, err)
}
