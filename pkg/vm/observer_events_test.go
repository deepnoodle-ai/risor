package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

func compileObserved(t *testing.T, source string) *bytecode.Code {
	t.Helper()
	ast, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	var names []string
	for name := range basicBuiltins() {
		names = append(names, name)
	}
	code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: names, Source: source})
	assert.Nil(t, err)
	return code
}

// CallEvent.Location is where the call is made, not the start of the body.
func TestObserverCallLocationIsCallSite(t *testing.T) {
	code := compileObserved(t, `
function add(a, b) {
	return a + b
}
let result = add(1, 2)
`)
	observer := &TestObserver{}
	vm, err := New(code, WithObserver(observer), WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))
	assert.Equal(t, len(observer.Calls), 1)
	assert.Equal(t, observer.Calls[0].FunctionName, "add")
	assert.Equal(t, observer.Calls[0].Location.Line, 5)
	assert.Equal(t, observer.Calls[0].FrameDepth, 2)
}

// A call from Go with VirtualMachine.Call has no call site in the script.
func TestObserverCallFromGoHasNoLocation(t *testing.T) {
	code := compileObserved(t, `
function add(a, b) {
	return a + b
}
let x = 1
`)
	observer := &TestObserver{}
	vm, err := New(code, WithObserver(observer), WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))
	fn, err := vm.Get("add")
	assert.Nil(t, err)
	_, err = vm.Call(context.Background(), fn.(*object.Closure),
		[]object.Object{object.NewInt(1), object.NewInt(2)})
	assert.Nil(t, err)
	assert.Equal(t, len(observer.Calls), 1)
	assert.Equal(t, observer.Calls[0].Location, object.SourceLocation{})
}

// A Call that fails its argument checks leaves the VM able to read globals.
func TestFailedCallKeepsGlobals(t *testing.T) {
	code := compileObserved(t, `function add(a, b) { return a + b }`)
	vm, err := New(code, WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))
	fn, err := vm.Get("add")
	assert.Nil(t, err)
	_, err = vm.Call(context.Background(), fn.(*object.Closure), nil)
	assert.NotNil(t, err)
	_, err = vm.Get("add")
	assert.Nil(t, err)
}

// A function that exits with an exception still reports OnReturn, so every
// OnCall has a matching OnReturn.
func TestObserverReturnOnException(t *testing.T) {
	code := compileObserved(t, `
function boom() { throw "x" }
function outer() { boom() }
let r = try { outer() } catch e { 1 }
`)
	observer := &TestObserver{}
	vm, err := New(code, WithObserver(observer), WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))
	var calls, returns []string
	for _, c := range observer.Calls {
		calls = append(calls, c.FunctionName)
	}
	for _, r := range observer.Returns {
		returns = append(returns, r.FunctionName)
	}
	assert.Equal(t, calls, []string{"outer", "boom"})
	assert.Equal(t, returns, []string{"boom", "outer"})
	assert.Equal(t, observer.Returns[0].FrameDepth, 2)
	assert.Equal(t, observer.Returns[1].FrameDepth, 1)
}

// StepOnLine reports the first line of every call, even when the same
// callback runs again on the same line.
func TestObserverStepOnLineRepeatedCallback(t *testing.T) {
	code := compileObserved(t, `[10, 20, 30].map(x => x + 1)`)
	observer := &StepOnLineObserver{}
	vm, err := New(code, WithObserver(observer), WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(context.Background()))
	// The main code, then each of the three callback calls. The main code
	// runs no further instructions after map returns.
	assert.Equal(t, observer.Lines, []int{1, 1, 1, 1})
}

// Each run reports its first line, even when the previous run ended on
// the same line of the same code.
func TestObserverStepOnLineEachRun(t *testing.T) {
	code := compileObserved(t, `let x = 1`)
	observer := &StepOnLineObserver{}
	vm, err := New(code, WithObserver(observer), WithGlobals(basicBuiltins()))
	assert.Nil(t, err)
	assert.Nil(t, vm.RunCode(context.Background(), code))
	first := len(observer.Lines)
	assert.True(t, first > 0)
	assert.Nil(t, vm.RunCode(context.Background(), code))
	assert.Equal(t, len(observer.Lines), 2*first)
}
