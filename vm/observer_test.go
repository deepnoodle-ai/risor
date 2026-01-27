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

func (o *TestObserver) Config() ObserverConfig {
	return NewObserverConfig(StepAll)
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
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
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
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
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
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
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

func (o *haltingObserverImpl) Config() ObserverConfig {
	return NewObserverConfig(StepAll)
}

func (o *haltingObserverImpl) OnStep(event StepEvent) bool {
	*o.stepCount++
	return *o.stepCount < o.haltAfter
}

// --- Tests for configurable observer modes ---

// StepNoneObserver tests StepNone mode which only observes calls/returns.
type StepNoneObserver struct {
	NoOpObserver
	Calls   []CallEvent
	Returns []ReturnEvent
}

func (o *StepNoneObserver) Config() ObserverConfig {
	return NewObserverConfig(StepNone)
}

func (o *StepNoneObserver) OnCall(event CallEvent) bool {
	o.Calls = append(o.Calls, event)
	return true
}

func (o *StepNoneObserver) OnReturn(event ReturnEvent) bool {
	o.Returns = append(o.Returns, event)
	return true
}

func TestObserverStepNone(t *testing.T) {
	source := `
function add(a, b) {
	return a + b
}
let result = add(1, 2)
`
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
	if err != nil {
		t.Fatal(err)
	}

	observer := &StepNoneObserver{}
	vm := New(code, WithObserver(observer))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// StepNone mode: OnStep should never be called (we didn't override it)
	// but OnCall and OnReturn should still work
	if len(observer.Calls) == 0 {
		t.Error("expected OnCall to be called in StepNone mode")
	}
	if len(observer.Returns) == 0 {
		t.Error("expected OnReturn to be called in StepNone mode")
	}
}

// StepOnLineObserver tests StepOnLine mode which only fires on line changes.
type StepOnLineObserver struct {
	NoOpObserver
	Lines []int // Collect unique lines seen
}

func (o *StepOnLineObserver) Config() ObserverConfig {
	cfg := NewObserverConfig(StepOnLine)
	cfg.ObserveCalls = false
	cfg.ObserveReturns = false
	return cfg
}

func (o *StepOnLineObserver) OnStep(event StepEvent) bool {
	o.Lines = append(o.Lines, event.Location.Line)
	return true
}

func TestObserverStepOnLine(t *testing.T) {
	source := `let x = 1
let y = 2
let z = x + y`
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
	if err != nil {
		t.Fatal(err)
	}

	observer := &StepOnLineObserver{}
	vm := New(code, WithObserver(observer))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// StepOnLine mode: should fire once per line change
	// Line 0 events are suppressed, so we expect lines 1, 2, 3
	if len(observer.Lines) < 3 {
		t.Errorf("expected at least 3 line change events, got %d", len(observer.Lines))
	}

	// Check that each line appears
	seenLines := make(map[int]bool)
	for _, line := range observer.Lines {
		seenLines[line] = true
	}
	for expectedLine := 1; expectedLine <= 3; expectedLine++ {
		if !seenLines[expectedLine] {
			t.Errorf("expected to see line %d in StepOnLine mode", expectedLine)
		}
	}
}

// StepSampledObserver tests StepSampled mode.
type StepSampledObserver struct {
	NoOpObserver
	StepCount int
	Interval  int
}

func (o *StepSampledObserver) Config() ObserverConfig {
	cfg := NewObserverConfig(StepSampled)
	cfg.SampleInterval = o.Interval
	cfg.ObserveCalls = false
	cfg.ObserveReturns = false
	return cfg
}

func (o *StepSampledObserver) OnStep(event StepEvent) bool {
	o.StepCount++
	return true
}

func TestObserverStepSampled(t *testing.T) {
	// Create a longer source to generate many instructions
	source := `let a = 1
let b = 2
let c = 3
let d = 4
let e = 5
let f = a + b + c + d + e`
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
	if err != nil {
		t.Fatal(err)
	}

	// First run with StepAll to count total instructions
	allObserver := &TestObserver{}
	vm := New(code, WithObserver(allObserver))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	totalInstructions := len(allObserver.Steps)

	// Now run with StepSampled at interval of 3
	interval := 3
	sampledObserver := &StepSampledObserver{Interval: interval}
	vm2 := New(code, WithObserver(sampledObserver))
	err = vm2.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Should have approximately totalInstructions/interval callbacks
	expectedCallbacks := totalInstructions / interval
	// Allow some tolerance for off-by-one
	if sampledObserver.StepCount < expectedCallbacks-1 || sampledObserver.StepCount > expectedCallbacks+1 {
		t.Errorf("expected ~%d sampled callbacks (total=%d, interval=%d), got %d",
			expectedCallbacks, totalInstructions, interval, sampledObserver.StepCount)
	}
}

func TestObserverSampleIntervalZero(t *testing.T) {
	// Test that SampleInterval <= 0 is normalized to 1 (every instruction)
	source := `let x = 1 + 2`
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create observer with SampleInterval of 0
	observer := &StepSampledObserver{Interval: 0}
	vm := New(code, WithObserver(observer))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// With interval normalized to 1, should fire on every instruction
	if observer.StepCount == 0 {
		t.Error("expected OnStep to be called when SampleInterval=0 (normalized to 1)")
	}
}

// DisabledCallsReturnsObserver tests disabling OnCall/OnReturn.
type DisabledCallsReturnsObserver struct {
	NoOpObserver
	Steps   []StepEvent
	Calls   []CallEvent
	Returns []ReturnEvent
}

func (o *DisabledCallsReturnsObserver) Config() ObserverConfig {
	cfg := NewObserverConfig(StepAll)
	cfg.ObserveCalls = false
	cfg.ObserveReturns = false
	return cfg
}

func (o *DisabledCallsReturnsObserver) OnStep(event StepEvent) bool {
	o.Steps = append(o.Steps, event)
	return true
}

func (o *DisabledCallsReturnsObserver) OnCall(event CallEvent) bool {
	o.Calls = append(o.Calls, event)
	return true
}

func (o *DisabledCallsReturnsObserver) OnReturn(event ReturnEvent) bool {
	o.Returns = append(o.Returns, event)
	return true
}

func TestObserverDisabledCallsReturns(t *testing.T) {
	source := `
function foo() {
	return 42
}
foo()
`
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
	if err != nil {
		t.Fatal(err)
	}

	observer := &DisabledCallsReturnsObserver{}
	vm := New(code, WithObserver(observer))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// OnStep should still be called
	if len(observer.Steps) == 0 {
		t.Error("expected OnStep to be called")
	}

	// OnCall and OnReturn should NOT be called when disabled
	if len(observer.Calls) > 0 {
		t.Error("expected OnCall NOT to be called when ObserveCalls=false")
	}
	if len(observer.Returns) > 0 {
		t.Error("expected OnReturn NOT to be called when ObserveReturns=false")
	}
}

func TestObserverZeroValueConfig(t *testing.T) {
	// Test that zero-value ObserverConfig{} disables calls/returns
	// This documents the footgun mentioned in the design doc
	cfg := ObserverConfig{} // Zero value
	if cfg.ObserveCalls != false {
		t.Error("expected zero-value ObserveCalls to be false")
	}
	if cfg.ObserveReturns != false {
		t.Error("expected zero-value ObserveReturns to be false")
	}

	// Compare with NewObserverConfig which has safe defaults
	safeCfg := NewObserverConfig(StepAll)
	if safeCfg.ObserveCalls != true {
		t.Error("expected NewObserverConfig to set ObserveCalls to true")
	}
	if safeCfg.ObserveReturns != true {
		t.Error("expected NewObserverConfig to set ObserveReturns to true")
	}
}

func TestNormalizeConfig(t *testing.T) {
	// Test NormalizeConfig with various SampleInterval values
	tests := []struct {
		name     string
		input    ObserverConfig
		expected int
	}{
		{
			name: "positive interval unchanged",
			input: ObserverConfig{
				StepMode:       StepSampled,
				SampleInterval: 100,
			},
			expected: 100,
		},
		{
			name: "zero interval normalized to 1",
			input: ObserverConfig{
				StepMode:       StepSampled,
				SampleInterval: 0,
			},
			expected: 1,
		},
		{
			name: "negative interval normalized to 1",
			input: ObserverConfig{
				StepMode:       StepSampled,
				SampleInterval: -5,
			},
			expected: 1,
		},
		{
			name: "non-sampled mode ignores interval",
			input: ObserverConfig{
				StepMode:       StepAll,
				SampleInterval: 0, // Should remain 0 for non-sampled modes
			},
			expected: 0, // Not normalized for non-StepSampled modes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeConfig(tt.input)
			if result.SampleInterval != tt.expected {
				t.Errorf("expected SampleInterval=%d, got %d", tt.expected, result.SampleInterval)
			}
		})
	}
}

func TestStepOnLineCrossFunctionCall(t *testing.T) {
	// Test that StepOnLine fires when entering a function even if on the same line
	// or when the code object changes
	source := `function id(x) { return x }
let y = id(42)`
	ast, err := parser.Parse(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(ast, nil)
	if err != nil {
		t.Fatal(err)
	}

	observer := &StepOnLineObserver{}
	vm := New(code, WithObserver(observer))
	err = vm.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Should see both line 1 (function def and body) and line 2 (call site)
	// Even if function body is on same line as def, code object change should trigger
	if len(observer.Lines) < 2 {
		t.Errorf("expected at least 2 line change events for cross-function call, got %d", len(observer.Lines))
	}
}
