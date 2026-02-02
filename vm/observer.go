package vm

import (
	"github.com/deepnoodle-ai/risor/v2/object"
	"github.com/deepnoodle-ai/risor/v2/op"
)

// StepMode controls when OnStep callbacks are triggered.
type StepMode uint8

const (
	// StepAll calls OnStep for every instruction.
	// Use for: detailed tracing, instruction-level debugging.
	StepAll StepMode = iota

	// StepNone never calls OnStep.
	// Use for: profilers that only need Call/Return events.
	StepNone

	// StepSampled calls OnStep every N instructions.
	// Use for: statistical CPU profiling.
	StepSampled

	// StepOnLine calls OnStep when the source location changes.
	// Use for: coverage tools, line-level debugging, breakpoint-based debuggers.
	StepOnLine
)

// ObserverConfig specifies what events an observer wants to receive.
// Use NewObserverConfig() to create configs with safe defaults.
type ObserverConfig struct {
	// StepMode controls OnStep callback frequency.
	StepMode StepMode

	// SampleInterval is the number of instructions between OnStep calls
	// when StepMode is StepSampled. Must be > 0; values <= 0 are treated as 1.
	// Ignored for other modes.
	SampleInterval int

	// ObserveCalls enables OnCall callbacks.
	ObserveCalls bool

	// ObserveReturns enables OnReturn callbacks.
	ObserveReturns bool
}

// NewObserverConfig creates a config with safe defaults.
// ObserveCalls and ObserveReturns default to true.
func NewObserverConfig(mode StepMode) ObserverConfig {
	return ObserverConfig{
		StepMode:       mode,
		SampleInterval: 1000, // Reasonable default for sampling
		ObserveCalls:   true,
		ObserveReturns: true,
	}
}

// NormalizeConfig validates and clamps config values.
// Note: Does NOT set defaults for ObserveCalls/ObserveReturns - callers
// should use NewObserverConfig() to get safe defaults.
func NormalizeConfig(cfg ObserverConfig) ObserverConfig {
	// Treat SampleInterval <= 0 as 1 (every instruction, same as StepAll)
	if cfg.StepMode == StepSampled && cfg.SampleInterval <= 0 {
		cfg.SampleInterval = 1
	}
	return cfg
}

// Observer is an interface for observing VM execution events.
// Implementations can be used for profiling, debugging, code coverage,
// or detailed execution tracing without modifying Risor's core.
//
// All methods are optional - implementations can embed NoOpObserver
// to provide default no-op implementations for methods they don't need.
//
// Observer methods are called synchronously during VM execution.
// Implementations should be fast to avoid impacting performance.
type Observer interface {
	// Config returns the observer's configuration.
	// Called once when the observer is attached to the VM.
	Config() ObserverConfig

	// OnStep is called based on the StepMode in the observer's config.
	// Returns false to halt execution immediately.
	OnStep(event StepEvent) bool

	// OnCall is called when a function is invoked (if ObserveCalls is true).
	// Returns false to halt execution immediately.
	OnCall(event CallEvent) bool

	// OnReturn is called when a function returns (if ObserveReturns is true).
	// Returns false to halt execution immediately.
	OnReturn(event ReturnEvent) bool
}

// StepEvent contains information about a single instruction step.
type StepEvent struct {
	// IP is the instruction pointer (index into the instruction array).
	IP int

	// Opcode is the operation being executed.
	Opcode op.Code

	// OpcodeName is the human-readable name of the opcode.
	OpcodeName string

	// Location is the source location of the instruction.
	Location object.SourceLocation

	// StackDepth is the current depth of the value stack.
	StackDepth int

	// FrameDepth is the current depth of the call stack.
	FrameDepth int
}

// CallEvent contains information about a function call.
type CallEvent struct {
	// FunctionName is the name of the function being called.
	// Anonymous functions will have an empty name.
	FunctionName string

	// ArgCount is the number of arguments passed to the function.
	ArgCount int

	// Location is the source location of the call site.
	Location object.SourceLocation

	// FrameDepth is the call stack depth after the call.
	FrameDepth int
}

// ReturnEvent contains information about a function return.
type ReturnEvent struct {
	// FunctionName is the name of the function returning.
	FunctionName string

	// Location is the source location of the return.
	Location object.SourceLocation

	// FrameDepth is the call stack depth after returning.
	FrameDepth int
}

// NoOpObserver is an Observer implementation that does nothing.
// Embed this in your observer to provide default implementations
// for methods you don't need.
//
// Important: NoOpObserver uses StepAll mode by default with ObserveCalls
// and ObserveReturns enabled. Override Config() in your observer to
// use a different mode.
type NoOpObserver struct{}

func (NoOpObserver) Config() ObserverConfig {
	return NewObserverConfig(StepAll)
}

func (NoOpObserver) OnStep(StepEvent) bool     { return true }
func (NoOpObserver) OnCall(CallEvent) bool     { return true }
func (NoOpObserver) OnReturn(ReturnEvent) bool { return true }

// Ensure NoOpObserver implements Observer.
var _ Observer = NoOpObserver{}
