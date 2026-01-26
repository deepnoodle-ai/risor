package vm

import (
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/op"
)

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
	// OnStep is called before each instruction is executed.
	// Returns false to halt execution immediately.
	OnStep(event StepEvent) bool

	// OnCall is called when entering a function.
	// Returns false to halt execution immediately.
	OnCall(event CallEvent) bool

	// OnReturn is called when returning from a function.
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
type NoOpObserver struct{}

func (NoOpObserver) OnStep(StepEvent) bool     { return true }
func (NoOpObserver) OnCall(CallEvent) bool     { return true }
func (NoOpObserver) OnReturn(ReturnEvent) bool { return true }

// Ensure NoOpObserver implements Observer.
var _ Observer = NoOpObserver{}
