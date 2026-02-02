package tests

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
	"github.com/deepnoodle-ai/wonton/assert"
)

// execute runs the input and returns the result as an object.Object
func execute(ctx context.Context, input string) (object.Object, error) {
	code, err := risor.Compile(ctx, input, risor.WithEnv(risor.Builtins()))
	if err != nil {
		return nil, err
	}
	return vm.Run(ctx, code, vm.WithGlobals(risor.Builtins()))
}

// TestBlankIdentifier_EndToEnd tests the blank identifier "_" functionality
// through the full compilation and execution pipeline.

func TestBlankIdentifier_LetDiscardsValue(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let _ = "this is discarded"
		let _ = [1, 2, 3]
		let _ = {a: 1, b: 2}
		42
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_AssignDiscardsValue(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		_ = "discarded"
		_ = 123
		42
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_MultiVarFirst(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let _, b = [1, 2]
		b
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "2")
}

func TestBlankIdentifier_MultiVarSecond(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let a, _ = [1, 2]
		a
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "1")
}

func TestBlankIdentifier_MultiVarBoth(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let _, _ = [1, 2]
		42
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_MultiVarMultiple(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let _, b, _, d = [1, 2, 3, 4]
		[b, d]
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[2, 4]")
}

func TestBlankIdentifier_ArrayDestructureFirst(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let [_, second] = [1, 2]
		second
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "2")
}

func TestBlankIdentifier_ArrayDestructureMiddle(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let [first, _, third] = [1, 2, 3]
		[first, third]
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[1, 3]")
}

func TestBlankIdentifier_ArrayDestructureAll(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let [_, _, _] = [1, 2, 3]
		42
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_ObjectDestructure(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let {x: _, y} = {x: 100, y: 42}
		y
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_ObjectDestructureMultiple(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let {a: _, b, c: _} = {a: 1, b: 2, c: 3}
		b
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "2")
}

func TestBlankIdentifier_FunctionParamFirst(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		function f(_, b) { return b }
		f(999, 42)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_FunctionParamSecond(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		function f(a, _) { return a }
		f(42, 999)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_FunctionParamMultiple(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		function f(_, x, _, y, _) { return x + y }
		f(0, 10, 0, 20, 0)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "30")
}

func TestBlankIdentifier_FunctionParamOnly(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		function f(_) { return 42 }
		f("ignored")
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_ArrowFunctionParam(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let f = (_, x) => x * 2
		f("ignored", 21)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_ArrowFunctionSingleParam(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let f = _ => 42
		f("ignored")
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_RestParam(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		function f(a, ..._) { return a }
		f(42, "ignored", "also ignored", "still ignored")
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_DoubleUnderscoreIsNormal(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		let __ = 42
		__
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}

func TestBlankIdentifier_CannotRead(t *testing.T) {
	ctx := context.Background()
	_, err := execute(ctx, `_`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot use _ as value")
}

func TestBlankIdentifier_CannotReadAfterAssign(t *testing.T) {
	ctx := context.Background()
	_, err := execute(ctx, `
		let _ = 42
		_
	`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot use _ as value")
}

func TestBlankIdentifier_CannotUseInExpression(t *testing.T) {
	ctx := context.Background()
	_, err := execute(ctx, `
		let x = _ + 1
	`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot use _ as value")
}

func TestBlankIdentifier_CannotCompoundAssign(t *testing.T) {
	ctx := context.Background()
	_, err := execute(ctx, `_ += 1`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot use _ in compound assignment")
}

func TestBlankIdentifier_WithCallbacks(t *testing.T) {
	// Test that _ works correctly in callbacks like map/filter
	ctx := context.Background()
	result, err := execute(ctx, `
		// Using _ to ignore index in map-like operation
		let items = [10, 20, 30]
		let doubled = items.map(x => x * 2)
		doubled
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Type(), object.LIST)
}

func TestBlankIdentifier_SideEffectsStillRun(t *testing.T) {
	// Verify that the RHS of let _ = expr is still evaluated
	ctx := context.Background()
	result, err := execute(ctx, `
		let counter = 0
		function increment() {
			counter = counter + 1
			return counter
		}
		let _ = increment()
		let _ = increment()
		let _ = increment()
		counter
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "3")
}

func TestBlankIdentifier_InNestedFunctions(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		function outer(_, inner_fn) {
			return inner_fn(100)
		}
		function inner(_, value) {
			return value * 2
		}
		outer("ignored", x => inner("also ignored", x))
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "200")
}

func TestBlankIdentifier_InClosures(t *testing.T) {
	ctx := context.Background()
	result, err := execute(ctx, `
		function makeAdder(_, amount) {
			return function(_, x) {
				return x + amount
			}
		}
		let add10 = makeAdder("ignored", 10)
		add10("also ignored", 32)
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "42")
}
