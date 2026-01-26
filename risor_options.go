package risor

import (
	"github.com/risor-io/risor/importer"
	"github.com/risor-io/risor/vm"
)

// Option describes a function used to configure a Risor evaluation.
type Option func(*config)

// WithGlobals provides global variables that are made available to Risor
// evaluations. This option is additive, so multiple WithGlobals options
// may be supplied. If the same key is supplied multiple times, the last
// supplied value is used.
func WithGlobals(globals map[string]any) Option {
	return func(cfg *config) {
		for k, v := range globals {
			cfg.globals[k] = v
		}
	}
}

// WithGlobal supplies a single named global variable to the Risor evaluation.
func WithGlobal(name string, value any) Option {
	return func(cfg *config) {
		cfg.globals[name] = value
	}
}

// WithoutGlobal opts out of a given global builtin or module. If the name can't
// be resolved, this is a no-op. This does operate on nested modules.
func WithoutGlobal(name string) Option {
	return func(cfg *config) {
		cfg.denylist[name] = true
	}
}

// WithoutGlobals removes multiple global builtins or modules.
func WithoutGlobals(names ...string) Option {
	return func(cfg *config) {
		for _, name := range names {
			cfg.denylist[name] = true
		}
	}
}

// WithGlobalOverride replaces the a global or module builtin with the given value
func WithGlobalOverride(name string, value any) Option {
	return func(cfg *config) {
		cfg.overrides[name] = value
	}
}

// WithoutDefaultGlobals opts out of all default global builtins and modules.
func WithoutDefaultGlobals() Option {
	return func(cfg *config) {
		cfg.withoutDefaultGlobals = true
	}
}

// WithImporter supplies an Importer that will be used to execute import statements.
func WithImporter(i importer.Importer) Option {
	return func(cfg *config) {
		cfg.importer = i
	}
}

// WithLocalImporter enables importing Risor modules from the given directory.
func WithLocalImporter(path string) Option {
	return func(cfg *config) {
		cfg.localImportPath = path
	}
}

// WithFilename sets the filename for the source code being evaluated.
func WithFilename(filename string) Option {
	return func(cfg *config) {
		cfg.filename = filename
	}
}

// WithVM specifies an existing Virtual Machine to use for the evaluation.
//
// Deprecated: Use the VM wrapper type instead for stateful execution.
// This option will be removed in v2.
func WithVM(vm *vm.VirtualMachine) Option {
	return func(cfg *config) {
		cfg.vm = vm
	}
}
