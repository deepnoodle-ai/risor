package vm

import (
	"github.com/risor-io/risor/importer"
	"github.com/risor-io/risor/os"
)

// Option is a configuration function for a Virtual Machine.
type Option func(*VirtualMachine)

// WithInstructionOffset sets the initial instruction offset.
func WithInstructionOffset(offset int) Option {
	return func(vm *VirtualMachine) {
		vm.ip = offset
	}
}

// WithImporter is used to supply an Importer to the Virtual Machine.
func WithImporter(importer importer.Importer) Option {
	return func(vm *VirtualMachine) {
		vm.importer = importer
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

// WithOS sets custom OS implementation in the context. This context is present
// in the invocation of Risor builtins, this OS will be used for all related
// functionality.
func WithOS(os os.OS) Option {
	return func(vm *VirtualMachine) {
		vm.os = os
	}
}
