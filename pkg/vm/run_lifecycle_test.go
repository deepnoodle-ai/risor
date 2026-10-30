package vm

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

func compileWithGlobals(t *testing.T, source string, globalNames ...string) *bytecode.Code {
	t.Helper()
	program, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	bc, err := compiler.Compile(program, &compiler.Config{GlobalNames: globalNames})
	assert.Nil(t, err)
	return bc
}

// newBuiltinsVM compiles source with the test builtins plus extra globals and
// returns a VM for it.
func newBuiltinsVM(t *testing.T, source string, extra map[string]any, opts ...Option) (*VirtualMachine, *bytecode.Code) {
	t.Helper()
	globals := basicBuiltins()
	for k, v := range extra {
		globals[k] = v
	}
	var names []string
	for k := range globals {
		names = append(names, k)
	}
	code := compileWithGlobals(t, source, names...)
	machine, err := New(code, append([]Option{WithGlobals(globals)}, opts...)...)
	assert.Nil(t, err)
	return machine, code
}

// waitForGoroutines waits for the goroutine count to drop to at most n.
func waitForGoroutines(n int) int {
	deadline := time.Now().Add(2 * time.Second)
	for {
		count := runtime.NumGoroutine()
		if count <= n || time.Now().After(deadline) {
			return count
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// The goroutine that watches the context must stop when the run ends, even
// if the context is never canceled.
func TestRunStopsContextWatcher(t *testing.T) {
	code := compileSource(t, "1 + 2")
	machine, err := New(code)
	assert.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	before := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		assert.Nil(t, machine.RunCode(ctx, code))
	}
	fn, err := New(compileSource(t, "function f() { return 1 }"))
	assert.Nil(t, err)
	assert.Nil(t, fn.Run(ctx))
	f, err := fn.Get("f")
	assert.Nil(t, err)
	for i := 0; i < 50; i++ {
		_, err := fn.Call(ctx, f.(*object.Closure), nil)
		assert.Nil(t, err)
	}
	assert.True(t, waitForGoroutines(before) <= before)
}

// Canceling the context of a finished run must not abort a later run.
func TestCancelOfFinishedRunDoesNotAbortLaterRun(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	cancelFirst := object.NewBuiltin("cancel_first",
		func(ctx context.Context, args ...object.Object) (object.Object, error) {
			cancel1()
			// Give a leftover watcher of the first run time to act.
			time.Sleep(20 * time.Millisecond)
			return object.Nil, nil
		})

	machine, code := newBuiltinsVM(t, `
		let total = 0
		cancel_first()
		range(10).each(i => { total += i })
		total
	`, map[string]any{"cancel_first": cancelFirst},
		// Rely on the halt flag alone, not the periodic context check.
		WithContextCheckInterval(0))

	assert.Nil(t, machine.RunCode(ctx1, compileSource(t, "1")))

	assert.Nil(t, machine.RunCode(context.Background(), code))
	result, ok := machine.TOS()
	assert.True(t, ok)
	assert.Equal(t, result, object.NewInt(45))
}

// Runs that each end by context cancellation must not race on the halt flag.
// Run with -race.
func TestRepeatedCanceledRunsNoRace(t *testing.T) {
	machine, code := newBuiltinsVM(t, "range(1000000000).each(i => i)", nil,
		WithContextCheckInterval(0))
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		err := machine.RunCode(ctx, code)
		cancel()
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	}
}

// A global that a script changed keeps its new value in the next RunCode,
// even when WithGlobals provides the same name again.
func TestRunCodeKeepsChangedGlobals(t *testing.T) {
	ctx := context.Background()
	globals := map[string]any{"x": 1}

	machine, err := New(compileWithGlobals(t, "x = 5", "x"), WithGlobals(globals))
	assert.Nil(t, err)
	assert.Nil(t, machine.Run(ctx))

	read := compileWithGlobals(t, "x", "x")
	assert.Nil(t, machine.RunCode(ctx, read))
	result, _ := machine.TOS()
	assert.Equal(t, result, object.NewInt(5))

	// The REPL passes its environment again on every input.
	assert.Nil(t, machine.RunCode(ctx, read, WithGlobals(globals)))
	result, _ = machine.TOS()
	assert.Equal(t, result, object.NewInt(5))

	// WithGlobals on RunCode still adds names the VM does not have yet.
	assert.Nil(t, machine.RunCode(ctx, compileWithGlobals(t, "x + y", "x", "y"),
		WithGlobals(map[string]any{"y": 10})))
	result, _ = machine.TOS()
	assert.Equal(t, result, object.NewInt(15))
}

// Globals carry forward across more than one RunCode, not only to the next.
func TestRunCodeCarriesGlobalsAcrossRuns(t *testing.T) {
	ctx := context.Background()
	machine, err := New(compileSource(t, "let a = 1"))
	assert.Nil(t, err)
	assert.Nil(t, machine.Run(ctx))
	assert.Nil(t, machine.RunCode(ctx, compileSource(t, "let b = 2")))
	assert.Nil(t, machine.RunCode(ctx, compileWithGlobals(t, "a + b", "a", "b")))
	result, _ := machine.TOS()
	assert.Equal(t, result, object.NewInt(3))
}

// WithMaxSteps limits each run, not the total across runs.
func TestMaxStepsAppliesPerRun(t *testing.T) {
	ctx := context.Background()
	machine, code := newBuiltinsVM(t, "let x = 0\nrange(300).each(i => { x += i })\nx", nil,
		WithMaxSteps(5000), WithContextCheckInterval(10))
	for i := 0; i < 5; i++ {
		assert.Nil(t, machine.RunCode(ctx, code), fmt.Sprintf("run %d", i))
	}
	fnVM, _ := newBuiltinsVM(t, `
		function f() { let x = 0; range(300).each(i => { x += i }); return x }`, nil,
		WithMaxSteps(5000), WithContextCheckInterval(10))
	assert.Nil(t, fnVM.Run(ctx))
	f, err := fnVM.Get("f")
	assert.Nil(t, err)
	for i := 0; i < 5; i++ {
		_, err := fnVM.Call(ctx, f.(*object.Closure), nil)
		assert.Nil(t, err, fmt.Sprintf("call %d", i))
	}
}

// Issue #502: a function from code that RunCode replaced must read its own
// globals, not the slots of the new code at the same indexes.
func TestCallAfterRunCodeReadsFunctionGlobals(t *testing.T) {
	ctx := context.Background()
	x := compileSource(t, "let g = 1\nfunction get() { return g }")
	y := compileSource(t, "let other = 42\nlet zz = 7")

	machine, err := New(x)
	assert.Nil(t, err)
	assert.Nil(t, machine.Run(ctx))
	get, err := machine.Get("get")
	assert.Nil(t, err)
	result, err := machine.Call(ctx, get.(*object.Closure), nil)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(1))

	assert.Nil(t, machine.RunCode(ctx, y))
	result, err = machine.Call(ctx, get.(*object.Closure), nil)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(1))

	// Run reloads y. The function keeps reading its own globals.
	assert.Nil(t, machine.Run(ctx))
	result, err = machine.Call(ctx, get.(*object.Closure), nil)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(1))
}

// A function from earlier code whose globals are laid out the same way as the
// new code (as in the REPL) shares the new code's globals, so its writes are
// visible to later runs.
func TestFunctionFromEarlierCodeSharesGlobals(t *testing.T) {
	ctx := context.Background()
	x := compileSource(t, "let n = 0\nfunction inc() { n++; return n }")
	machine, err := New(x)
	assert.Nil(t, err)
	assert.Nil(t, machine.Run(ctx))

	y := compileSource(t, "let n = 0\nfunction inc() { n++; return n }\ninc()\ninc()\nn")
	assert.Nil(t, machine.RunCode(ctx, y))
	result, _ := machine.TOS()
	assert.Equal(t, result, object.NewInt(2))

	inc, err := machine.Get("inc")
	assert.Nil(t, err)
	result, err = machine.Call(ctx, inc.(*object.Closure), nil)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(3))
	n, err := machine.Get("n")
	assert.Nil(t, err)
	assert.Equal(t, n, object.NewInt(3))
}

// Source lines are split once per code tree, not once per instruction.
func TestSourceLinesSplitOnce(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&sb, "let v%d = %d\n", i, i)
	}
	code := compileSource(t, sb.String())
	assert.True(t, code.LocationCount() > 2000)
	assert.Equal(t, code.GetSourceLine(1500), "let v1499 = 1499")
	allocs := testing.AllocsPerRun(10, func() { code.GetSourceLine(1500) })
	assert.Equal(t, allocs, 0.0)
	lc := wrapCode(code)
	assert.Equal(t, lc.LocationAt(lc.LocationCount()-1).Source, "let v1999 = 1999")
}
