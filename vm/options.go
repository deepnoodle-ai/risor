package vm

import "github.com/risor-io/risor/object"

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

// WithGlobals provides global variables with the given names.
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
