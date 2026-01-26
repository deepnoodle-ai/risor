package vm

import (
	"context"
	"testing"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/parser"
)

// TestObserver is a test observer that records events.
type TestObserver struct {
	NoOpObserver
	Steps   []StepEvent
	Calls   []CallEvent
	Returns []ReturnEvent
}

func (o *TestObserver) OnStep(event StepEvent) bool {
	o.Steps = append(o.Steps, event)
	return true
}

func (o *TestObserver) OnCall(event CallEvent) bool {
	o.Calls = append(o.Calls, event)
	return true
}

func (o *TestObserver) OnReturn(event ReturnEvent) bool {
	o.Returns = append(o.Returns, event)
	return true
}

func TestObserverOnStep(t *testing.T) {
	source := `let x = 1 + 2`
	ast, err := parser.Parse(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast)
	if err != nil {
		t.Fatal(err)
	}

	observer := &TestObserver{}
	vm := New(code, WithObserver(observer))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(observer.Steps) == 0 {
		t.Error("expected at least one step event")
	}

	// Check that step events have valid data
	for _, step := range observer.Steps {
		if step.OpcodeName == "" {
			t.Error("expected opcode name to be set")
		}
	}
}

func TestObserverOnCallAndReturn(t *testing.T) {
	source := `
function add(a, b) {
	return a + b
}
let result = add(1, 2)
`
	ast, err := parser.Parse(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast)
	if err != nil {
		t.Fatal(err)
	}

	observer := &TestObserver{}
	vm := New(code, WithObserver(observer))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Should have at least one call event for the add function
	if len(observer.Calls) == 0 {
		t.Error("expected at least one call event")
	}

	// Check for the add function call
	foundAdd := false
	for _, call := range observer.Calls {
		if call.FunctionName == "add" {
			foundAdd = true
			if call.ArgCount != 2 {
				t.Errorf("expected add to have 2 args, got %d", call.ArgCount)
			}
		}
	}
	if !foundAdd {
		t.Error("expected to find call event for 'add' function")
	}

	// Should have at least one return event
	if len(observer.Returns) == 0 {
		t.Error("expected at least one return event")
	}
}

func TestObserverHaltOnStep(t *testing.T) {
	source := `let x = 1 + 2 + 3 + 4`
	ast, err := parser.Parse(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast)
	if err != nil {
		t.Fatal(err)
	}

	stepCount := 0
	haltAfter := 3

	observer := &struct {
		NoOpObserver
	}{}

	// Create custom observer that halts after N steps
	haltingObserver := &haltingObserverImpl{haltAfter: haltAfter, stepCount: &stepCount}

	vm := New(code, WithObserver(haltingObserver))
	err = vm.Run(context.Background())

	if err == nil {
		t.Error("expected error when observer halts execution")
	}

	if stepCount != haltAfter {
		t.Errorf("expected %d steps before halt, got %d", haltAfter, stepCount)
	}

	// Suppress unused variable warning
	_ = observer
}

type haltingObserverImpl struct {
	NoOpObserver
	haltAfter int
	stepCount *int
}

func (o *haltingObserverImpl) OnStep(event StepEvent) bool {
	*o.stepCount++
	return *o.stepCount < o.haltAfter
}
