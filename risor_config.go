package risor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/importer"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/os"
	"github.com/risor-io/risor/vm"
)

// Config assists in configuring Risor evaluations.
type Config struct {
	globals               map[string]any
	overrides             map[string]any
	denylist              map[string]bool
	importer              importer.Importer
	os                    os.OS
	localImportPath       string
	withoutDefaultGlobals bool
	initialized           bool
	filename              string
	vm                    *vm.VirtualMachine
}

// NewConfig returns a new Risor Config. Use the Risor options functions
// to customize the configuration.
func NewConfig(opts ...Option) *Config {
	cfg := &Config{
		globals:   map[string]any{},
		overrides: map[string]any{},
		denylist:  map[string]bool{},
	}
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(cfg)
		}
		cfg.init()
	}
	return cfg
}

// Globals returns a map of all global variables that should be available in a
// Risor evaluation.
func (cfg *Config) Globals() map[string]any {
	cfg.init()
	globalsCopy := map[string]any{}
	for k, v := range cfg.globals {
		globalsCopy[k] = v
	}
	return globalsCopy
}

// CombinedGlobals returns a map of all global variables that should be
// available in a Risor evaluation.
//
// Deprecated: Use Globals instead.
func (cfg *Config) CombinedGlobals() map[string]any {
	cfg.init()
	globalsCopy := map[string]any{}
	for k, v := range cfg.globals {
		globalsCopy[k] = v
	}
	return globalsCopy
}

// GlobalNames returns a list of all global variables names that should be
// available in a Risor evaluation.
func (cfg *Config) GlobalNames() []string {
	cfg.init()
	var names []string
	for name := range cfg.globals {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (cfg *Config) init() error {
	if cfg.initialized {
		return nil
	}
	cfg.initialized = true
	cfg.applyDefaultGlobals()
	cfg.applyDenylist()
	if err := cfg.applyOverrides(); err != nil {
		return err
	}
	return nil
}

func (cfg *Config) applyDefaultGlobals() {
	if cfg.withoutDefaultGlobals {
		return
	}
	for k, v := range DefaultGlobals(DefaultGlobalsOpts{}) {
		cfg.globals[k] = v
	}
}

func (cfg *Config) applyDenylist() {
	for name := range cfg.denylist {
		parts := strings.SplitN(name, ".", 2)
		if len(parts) == 1 {
			delete(cfg.globals, name)
			continue
		}
		// Resolve the module (which could be nested) and then remove the
		// named attribute from it
		moduleName, attr := parts[0], parts[1]
		if obj, ok := cfg.globals[moduleName]; ok {
			if m, ok := obj.(*object.Module); ok {
				removeModuleAttr(m, attr)
			}
		}
	}
}

func (cfg *Config) applyOverrides() error {
	for name, value := range cfg.overrides {
		parts := strings.Split(name, ".")
		if len(parts) == 1 {
			cfg.globals[name] = value
			continue
		}
		valueObj := object.FromGoType(value)
		if valueObj == nil || valueObj.Type() == object.ERROR {
			return fmt.Errorf("init error: invalid value for global override: %v", value)
		}
		moduleName := parts[0]
		nestedModulePath := parts[1 : len(parts)-1]
		attrName := parts[len(parts)-1]
		if obj, ok := cfg.globals[moduleName]; ok {
			if m, ok := obj.(*object.Module); ok {
				if targetMod, ok := resolveModule(m, nestedModulePath); ok {
					targetMod.Override(attrName, valueObj)
				}
			}
		}
	}
	return nil
}

// CompilerOpts returns compiler options derived from this configuration.
func (cfg *Config) CompilerOpts() []compiler.Option {
	cfg.init()
	globalNames := cfg.GlobalNames()
	var opts []compiler.Option
	if len(globalNames) > 0 {
		opts = append(opts, compiler.WithGlobalNames(globalNames))
	}
	if cfg.filename != "" {
		opts = append(opts, compiler.WithFilename(cfg.filename))
	}
	return opts
}

// VMOpts returns virtual machine options derived from this configuration.
func (cfg *Config) VMOpts() []vm.Option {
	cfg.init()
	var opts []vm.Option
	globals := cfg.globals
	if len(globals) > 0 {
		opts = append(opts, vm.WithGlobals(globals))
	}
	importer := cfg.importer
	if importer == nil && cfg.localImportPath != "" {
		var names []string
		for name := range globals {
			names = append(names, name)
		}
		importer = newLocalImporter(names, cfg.localImportPath)
	}
	if importer != nil {
		opts = append(opts, vm.WithImporter(importer))
	}
	if cfg.os != nil {
		opts = append(opts, vm.WithOS(cfg.os))
	}
	return opts
}

func newLocalImporter(globalNames []string, sourceDir string) importer.Importer {
	return importer.NewLocalImporter(importer.LocalImporterOptions{
		GlobalNames: globalNames,
		SourceDir:   sourceDir,
		Extensions:  []string{".risor", ".rsr"},
	})
}

func resolveModule(m *object.Module, attr []string) (*object.Module, bool) {
	if len(attr) == 0 {
		return m, true
	}
	var result *object.Module
	for _, name := range attr {
		if obj, ok := m.GetAttr(name); ok {
			if modObj, ok := obj.(*object.Module); ok {
				result = modObj
				continue
			}
		}
		return nil, false
	}
	return result, true
}

func removeModuleAttr(m *object.Module, attr string) {
	parts := strings.Split(attr, ".")
	partsLen := len(parts)
	if partsLen == 1 {
		m.Override(attr, nil)
		return
	}
	name := parts[partsLen-1]
	modPath := parts[:partsLen-1]
	if mod, ok := resolveModule(m, modPath); ok {
		mod.Override(name, nil)
	}
}
