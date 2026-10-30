package risor

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Errors that end a run must not reach the script's catch or finally blocks,
// whether they are raised in the same frame or in a called function.

func TestStepLimitNotCatchableAcrossCalls(t *testing.T) {
	_, err := Eval(context.Background(), `
	let count = 0
	range(20).each(_ => {
		try { list(range(1000)).map(x => { count++; return x }) } catch (e) {}
	})
	count
	`, WithEnv(Builtins()), WithMaxSteps(1000))
	assert.True(t, errors.Is(err, vm.ErrStepLimitExceeded), "got %v", err)
}

func TestStepLimitSkipsFinally(t *testing.T) {
	_, err := Eval(context.Background(), `
	function spin() { range(100000).each(x => x) }
	function run() { try { spin() } finally { return "escaped" } }
	run()
	`, WithEnv(Builtins()), WithMaxSteps(5000))
	assert.True(t, errors.Is(err, vm.ErrStepLimitExceeded), "got %v", err)
}

func TestStackOverflowNotCatchable(t *testing.T) {
	ctx := context.Background()
	for _, src := range []string{
		`function recurse() { recurse() }
		 function run() { try { recurse() } finally { return "escaped" } }
		 run()`,
		`function recurse() { recurse() }
		 function run() { try { recurse() } catch (e) { return "caught" } }
		 run()`,
		`function recurse() { recurse() }
		 try { recurse() } catch (e) { "caught" }`,
	} {
		_, err := Eval(ctx, src)
		assert.True(t, errors.Is(err, vm.ErrStackOverflow), "%s: got %v", src, err)
	}
}

type haltOnCall struct {
	vm.NoOpObserver
	name string
}

func (o *haltOnCall) Config() vm.ObserverConfig {
	return vm.ObserverConfig{StepMode: vm.StepNone, ObserveCalls: true}
}

func (o *haltOnCall) OnCall(e vm.CallEvent) bool { return e.FunctionName != o.name }

func TestObserverHaltNotCatchable(t *testing.T) {
	_, err := Eval(context.Background(), `
	function stop() { return 1 }
	function run() { try { stop() } catch (e) { return "caught" } }
	run()
	`, WithObserver(&haltOnCall{name: "stop"}))
	assert.True(t, errors.Is(err, vm.ErrObserverHalt), "got %v", err)
}

func TestContextCancelNotCatchable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := map[string]any{
		"cancel": func(ctx context.Context) error {
			cancel()
			return ctx.Err()
		},
	}
	_, err := Eval(ctx, `
	function run() { try { cancel() } catch (e) { return "caught" } }
	run()
	`, WithEnv(env))
	assert.True(t, errors.Is(err, context.Canceled), "got %v", err)
}

// Once the run's context is canceled, any error ends the run, even an
// ordinary one that a host function returns.
func TestErrorAfterCancelNotCatchable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finallyRan := false
	env := map[string]any{
		"cancel": func() error {
			cancel()
			return errors.New("boom")
		},
		"mark": func() { finallyRan = true },
	}
	result, err := Eval(ctx, `
	function run() { try { cancel() } finally { mark(); return "escaped" } }
	run()
	`, WithEnv(env))
	assert.NotNil(t, err, "got result %v", result)
	assert.False(t, finallyRan)
}

// A step limit error raised in a callback keeps the script location and
// stack trace.
func TestFatalErrorKeepsLocation(t *testing.T) {
	_, err := Eval(context.Background(), `
	function spin() { range(100000).each(x => x) }
	spin()
	`, WithEnv(Builtins()), WithMaxSteps(5000))
	assert.True(t, errors.Is(err, vm.ErrStepLimitExceeded), "got %v", err)
	var se *object.StructuredError
	assert.True(t, errors.As(err, &se), "got %T", err)
	assert.True(t, se.Location.Line > 0, "location %v", se.Location)
	assert.True(t, len(se.Stack) > 0)
}

// A host function may return a context error of its own, such as a timeout
// on a request it made. The run's context is not done, so the script can
// catch it.
func TestHostContextErrorCatchable(t *testing.T) {
	env := map[string]any{
		"fetch": func() error { return context.DeadlineExceeded },
	}
	result, err := Eval(context.Background(), `
	try { fetch() } catch (e) { "caught" }
	`, WithEnv(env))
	assert.Nil(t, err)
	assert.Equal(t, result, "caught")
}
