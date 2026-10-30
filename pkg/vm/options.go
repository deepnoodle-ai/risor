package vm

import (
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

// Option is a configuration function for a Virtual Machine.
type Option func(*VirtualMachine)

// WithInstructionOffset sets the starting instruction offset for the next
// RunCode call. This survives resetForNewCode() and is used by the REPL to
// skip past previously executed (or errored) code in incremental compilation.
func WithInstructionOffset(offset int) Option {
	return func(vm *VirtualMachine) {
		vm.requestedIP = offset
	}
}

// WithGlobals provides global variables with the given names. When passed to
// RunCode, a value replaces the current value for that name, including a
// value that a script changed. Globals that the call does not name keep the
// values that earlier runs left in them.
func WithGlobals(globals map[string]any) Option {
	return func(vm *VirtualMachine) {
		for name, value := range globals {
			vm.inputGlobals[name] = value
		}
	}
}

// WithContextCheckInterval sets how often the VM checks ctx.Done() during
// execution. The interval is specified in number of instructions. A value of 0
// disables deterministic checking, relying only on the background goroutine
// that monitors the context. The default is DefaultContextCheckInterval (1000).
//
// Lower values provide more responsive cancellation but may slightly impact
// performance due to more frequent checks. Higher values reduce overhead but
// delay cancellation detection.
func WithContextCheckInterval(interval int) Option {
	return func(vm *VirtualMachine) {
		vm.contextCheckInterval = interval
	}
}

// WithObserver sets an observer for VM execution events.
// The observer receives callbacks for instruction steps, function calls,
// and function returns. This enables profilers, debuggers, code coverage
// tools, and execution tracers without modifying Risor's core.
//
// Observer methods are called synchronously during execution, so
// implementations should be fast to avoid impacting performance.
// Returning false from any observer method halts execution immediately.
func WithObserver(observer Observer) Option {
	return func(vm *VirtualMachine) {
		vm.observer = observer
	}
}

// WithTypeRegistry sets the type registry for Go/Risor type conversions.
// If not set, object.DefaultRegistry() is used.
func WithTypeRegistry(registry *object.TypeRegistry) Option {
	return func(vm *VirtualMachine) {
		vm.typeRegistry = registry
	}
}

// WithLimits sets the allocation limits for operations that can produce large
// values, such as string.repeat(), concatenation, and list(range(...)).
// Exceeding a limit raises a value error in the script. Zero or negative
// fields use the defaults (see object.DefaultLimits). Without this option,
// limits stored in the context with object.WithLimits apply.
func WithLimits(limits object.Limits) Option {
	return func(vm *VirtualMachine) {
		vm.limits = &limits
	}
}

// WithMaxSteps sets the maximum number of instructions the VM will execute in
// each run (each call to Run, RunCode, or Call).
// If the limit is exceeded, the VM will return ErrStepLimitExceeded.
// A value of 0 (default) means unlimited.
//
// Step counting includes all VM instructions, including those executed in
// callbacks invoked by methods like list.each() and list.map(). This ensures
// that resource limits work consistently regardless of code structure.
//
// The limit is exact: a run may execute n instructions, and the next one
// returns ErrStepLimitExceeded. It holds even when WithContextCheckInterval
// disables the periodic context checks.
func WithMaxSteps(n int64) Option {
	return func(vm *VirtualMachine) {
		vm.maxSteps = n
	}
}

// WithMaxStackDepth sets both the maximum value stack depth and call frame
// depth for the VM. If either limit is exceeded, the VM will return
// ErrStackOverflow. A value of 0 (default) uses the global MaxStackDepth
// and MaxFrameDepth constants.
//
// This is a convenience function that sets both limits to the same value.
// Use WithMaxValueStackDepth and WithMaxFrameDepth for fine-grained control.
func WithMaxStackDepth(n int) Option {
	return func(vm *VirtualMachine) {
		vm.maxValueStackDepth = n
		vm.maxFrameDepth = n
	}
}

// WithMaxValueStackDepth sets the maximum value stack depth for the VM.
// The value stack holds intermediate values during expression evaluation.
// Each push checks the limit. If exceeded, the VM will return ErrStackOverflow.
// A value of 0 (default) uses the global MaxStackDepth constant.
func WithMaxValueStackDepth(n int) Option {
	return func(vm *VirtualMachine) {
		vm.maxValueStackDepth = n
	}
}

// WithMaxFrameDepth sets the maximum call frame depth for the VM.
// This limits how deep function calls can be nested (recursion depth).
// If exceeded, the VM will return ErrStackOverflow.
// A value of 0 (default) uses the global MaxFrameDepth constant.
func WithMaxFrameDepth(n int) Option {
	return func(vm *VirtualMachine) {
		vm.maxFrameDepth = n
	}
}

// WithMaxLocals sets the maximum total number of local variable slots across
// all active function call frames. Each call checks the budget before its
// frame allocates locals, and returns ErrStackOverflow if the call would
// exceed it. The error ends the run: script try/catch cannot catch it, and
// finally blocks do not run. The entry frame of a run
// does not count. Zero (the default) or a negative value uses DefaultMaxLocals.
//
// Compiled source rarely comes near the default. The budget matters for
// loaded bytecode, where one function can use up to 65,535 slots.
func WithMaxLocals(n int) Option {
	return func(vm *VirtualMachine) {
		vm.maxLocals = n
	}
}

// WithTimeout sets a timeout for VM execution.
// If the timeout is exceeded, the VM will return context.DeadlineExceeded.
// A value of 0 (default) means no timeout.
func WithTimeout(d time.Duration) Option {
	return func(vm *VirtualMachine) {
		vm.timeout = d
	}
}
