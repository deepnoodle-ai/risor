package vm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Tests for audit findings C-04, H-01, H-08, and H-09.

// C-04: reading a hoisted function before its declaration executes must yield
// object.Nil, never a raw Go nil.
func TestAuditGlobalReadBeforeInit(t *testing.T) {
	result, err := run(context.Background(), `
	let f = foo
	function foo() { return 1 }
	f
	`)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, result, object.Nil)
}

// C-04: globals that are never assigned must be initialized to object.Nil.
func TestAuditUnassignedGlobalsAreNil(t *testing.T) {
	vm, err := newVM(context.Background(), `
	let x = foo
	function foo() { return 1 }
	`)
	assert.Nil(t, err)
	code := vm.loadCode(vm.main)
	for i, g := range code.Globals {
		assert.NotNil(t, g, "global %q is raw nil", code.GlobalNameAt(i))
	}
}

// C-04: unassigned locals must be initialized to object.Nil.
func TestAuditUnassignedLocalsAreNil(t *testing.T) {
	small := "function f() { let a = 1; let b = 2; return a + b }"
	large := "function f() { let a = 1; let b = 2; let c = 3; let d = 4; let e = 5; " +
		"let g = 6; let h = 7; let i = 8; let j = 9; let k = 10; return a }"
	for _, src := range []string{small, large} {
		fn := findFunction(compileSource(t, src))
		assert.NotNil(t, fn)
		code := wrapCode(fn.Code())
		var fr frame
		// Activate twice to cover both fresh allocation and slot reuse.
		for range 2 {
			fr.ActivateCode(code)
			assert.True(t, len(fr.Locals()) > 0)
			for _, v := range fr.Locals() {
				assert.Equal(t, v, object.Object(object.Nil))
			}
			for i := range fr.Locals() {
				fr.Locals()[i] = object.NewInt(1)
			}
		}
	}
}

// H-01: pushing past MaxStackDepth between periodic checks must return
// ErrStackOverflow rather than panicking.
func TestAuditValueStackOverflowPush(t *testing.T) {
	elems := strings.TrimSuffix(strings.Repeat("1,", MaxStackDepth+10), ",")
	_, err := run(context.Background(), "["+elems+"]")
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrStackOverflow), "got %v", err)
}

// H-01: the overflow inside a callback must not double-panic in the deferred
// frame restore of callFunction.
func TestAuditValueStackOverflowInCallback(t *testing.T) {
	elems := strings.TrimSuffix(strings.Repeat("1,", MaxStackDepth+10), ",")
	_, err := run(context.Background(), "[1].map(x => ["+elems+"])")
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrStackOverflow), "got %v", err)
}

// H-01: Unpack must bounds-check before pushing.
func TestAuditValueStackOverflowUnpack(t *testing.T) {
	n := MaxStackDepth + 10
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("v%d", i)
	}
	_, err := run(context.Background(), fmt.Sprintf("let [%s] = [1]", strings.Join(names, ", ")))
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrStackOverflow), "got %v", err)
}

// H-08: a named function with the maximum parameter count plus a rest
// parameter must not overflow vm.tmp.
func TestAuditMaxParamsWithRestParam(t *testing.T) {
	for _, count := range []int{compiler.MaxArgs - 1, compiler.MaxArgs} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			params := make([]string, count)
			args := make([]string, count)
			for i := range params {
				params[i] = fmt.Sprintf("p%d", i)
				args[i] = "1"
			}
			src := fmt.Sprintf(`
			function f(%s, ...rest) { return len(rest) }
			f(%s)
			`, strings.Join(params, ", "), strings.Join(args, ", "))
			result, err := run(context.Background(), src)
			assert.Nil(t, err)
			assert.Equal(t, result, object.NewInt(0))
		})
	}
}

// H-08: CallSpread must enforce MaxArgs for all callables.
func TestAuditCallSpreadMaxArgs(t *testing.T) {
	called := false
	fn := object.NewBuiltin("count", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		called = true
		return object.NewInt(int64(len(args))), nil
	})
	_, err := run(context.Background(), fmt.Sprintf("count(...list(range(%d)))", MaxArgs+1),
		runOpts{Globals: map[string]any{"count": fn}})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "max args")
	assert.False(t, called)

	result, err := run(context.Background(), fmt.Sprintf("count(...list(range(%d)))", MaxArgs),
		runOpts{Globals: map[string]any{"count": fn}})
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(MaxArgs))
}

// H-09: vm.Call before vm.Run must work (or fail cleanly), not panic.
func TestAuditCallBeforeRun(t *testing.T) {
	bc := compileSource(t, "function add(a, b) { return a + b }")
	fn := findFunction(bc)
	assert.NotNil(t, fn)

	machine, err := New(bc)
	assert.Nil(t, err)
	result, err := machine.Call(context.Background(), object.NewClosure(fn),
		[]object.Object{object.NewInt(1), object.NewInt(2)})
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(3))
}

// H-09: vm.Call on a fresh VM (sp == -1) must leave the stack balanced
// across repeated calls, including calls that fail.
func TestAuditCallStackBalanced(t *testing.T) {
	ctx := context.Background()
	machine, err := newVM(ctx, `
	function ok(x) { return x * 2 }
	function bad(x) { return x + "s" }
	`)
	assert.Nil(t, err)
	assert.Nil(t, machine.Run(ctx))
	baseSP := machine.sp

	okFn, err := machine.Get("ok")
	assert.Nil(t, err)
	badFn, err := machine.Get("bad")
	assert.Nil(t, err)

	for i := 0; i < 5; i++ {
		result, err := machine.Call(ctx, okFn.(*object.Closure), []object.Object{object.NewInt(int64(i))})
		assert.Nil(t, err)
		assert.Equal(t, result, object.NewInt(int64(i*2)))
		assert.Equal(t, machine.sp, baseSP)

		_, err = machine.Call(ctx, badFn.(*object.Closure), []object.Object{object.NewInt(1)})
		assert.NotNil(t, err)
		assert.Equal(t, machine.sp, baseSP)
	}
}

// H-09: a failing callback must not leave values on the caller's stack.
func TestAuditCallbackErrorLeavesStackClean(t *testing.T) {
	ctx := context.Background()
	machine, err := newVM(ctx, `function bad(x) { return 1 + (2 + (x + "s")) }`)
	assert.Nil(t, err)
	assert.Nil(t, machine.Run(ctx))
	baseSP := machine.sp
	badFn, err := machine.Get("bad")
	assert.Nil(t, err)
	_, err = machine.callFunction(machine.initContext(ctx), badFn.(*object.Closure),
		[]object.Object{object.NewInt(1)})
	assert.NotNil(t, err)
	assert.Equal(t, machine.sp, baseSP)
}

func compileSource(t *testing.T, source string) *bytecode.Code {
	t.Helper()
	program, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	bc, err := compiler.Compile(program, nil)
	assert.Nil(t, err)
	return bc
}

func findFunction(bc *bytecode.Code) *bytecode.Function {
	for i := 0; i < bc.ConstantCount(); i++ {
		if fn, ok := bc.ConstantAt(i).(*bytecode.Function); ok {
			return fn
		}
	}
	return nil
}

// haltInTryObserver halts execution once, on the first step after a try block
// pushes its exception handler.
type haltInTryObserver struct {
	NoOpObserver
	inTry  bool
	halted bool
}

func (o *haltInTryObserver) Config() ObserverConfig {
	return NewObserverConfig(StepAll)
}

func (o *haltInTryObserver) OnStep(event StepEvent) bool {
	if o.halted {
		return true
	}
	if o.inTry {
		o.halted = true
		return false
	}
	o.inTry = event.Opcode == op.PushExcept
	return true
}

// An observer halt inside a try block ends the run without unwinding its
// handler. A later Call must not let that stale handler catch errors thrown by
// the called function.
func TestAuditCallIgnoresStaleExceptionHandlers(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, `
	function fail() { throw "boom" }
	try { 1 } catch e { 99 }
	`, nil)
	assert.Nil(t, err)
	main, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)
	vm, err := New(main, WithObserver(&haltInTryObserver{}))
	assert.Nil(t, err)
	assert.NotNil(t, vm.Run(ctx))

	obj, err := vm.Get("fail")
	assert.Nil(t, err)
	fn, ok := obj.(*object.Closure)
	assert.True(t, ok)
	result, err := vm.Call(ctx, fn, nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "boom"))
	assert.Nil(t, result)
	assert.Equal(t, vm.excStackSize, 0)
}
