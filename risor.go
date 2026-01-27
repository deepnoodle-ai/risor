package risor

import (
	"context"
	"maps"
	"slices"

	"github.com/risor-io/risor/builtins"
	"github.com/risor-io/risor/bytecode"
	"github.com/risor-io/risor/compiler"
	modMath "github.com/risor-io/risor/modules/math"
	modRand "github.com/risor-io/risor/modules/rand"
	modRegexp "github.com/risor-io/risor/modules/regexp"
	modTime "github.com/risor-io/risor/modules/time"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/risor-io/risor/vm"
)

// Option configures a Risor compilation or execution.
type Option func(*options)

type options struct {
	env          map[string]any
	filename     string
	observer     vm.Observer
	typeRegistry *object.TypeRegistry
}

func collectOptions(opts ...Option) *options {
	o := &options{env: map[string]any{}}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

func (o *options) compilerConfig() *compiler.Config {
	cfg := &compiler.Config{}
	if len(o.env) > 0 {
		cfg.GlobalNames = slices.Sorted(maps.Keys(o.env))
	}
	if o.filename != "" {
		cfg.Filename = o.filename
	}
	return cfg
}

func (o *options) vmOpts() []vm.Option {
	var opts []vm.Option
	if len(o.env) > 0 {
		opts = append(opts, vm.WithGlobals(o.env))
	}
	if o.observer != nil {
		opts = append(opts, vm.WithObserver(o.observer))
	}
	if o.typeRegistry != nil {
		opts = append(opts, vm.WithTypeRegistry(o.typeRegistry))
	}
	return opts
}

// WithEnv provides environment variables that are made available to Risor
// scripts. This option is additive, so multiple WithEnv options may be
// supplied. If the same key is supplied multiple times, the last value wins.
//
// By default, the environment is empty. Use Builtins() to get the standard
// library:
//
//	result, _ := risor.Eval(ctx, source, risor.WithEnv(risor.Builtins()))
func WithEnv(env map[string]any) Option {
	return func(o *options) {
		maps.Copy(o.env, env)
	}
}

// WithFilename sets the filename for the source code being evaluated.
// This is used for error messages and stack traces.
func WithFilename(filename string) Option {
	return func(o *options) {
		o.filename = filename
	}
}

// WithObserver sets an observer for VM execution events.
// The observer receives callbacks for instruction steps, function calls,
// and function returns. This enables profilers, debuggers, code coverage
// tools, and execution tracers.
func WithObserver(observer vm.Observer) Option {
	return func(o *options) {
		o.observer = observer
	}
}

// WithTypeRegistry sets a custom type registry for Go/Risor type conversions.
// Use NewTypeRegistry() to create a registry with custom converters.
//
// Example:
//
//	registry := risor.NewTypeRegistry().
//	    RegisterFromGo(reflect.TypeOf(MyType{}), convertMyType).
//	    Build()
//	result, _ := risor.Eval(ctx, source, risor.WithTypeRegistry(registry))
func WithTypeRegistry(registry *object.TypeRegistry) Option {
	return func(o *options) {
		o.typeRegistry = registry
	}
}

// NewTypeRegistry creates a RegistryBuilder for custom type conversions.
// Use this to add support for custom Go types in Risor scripts.
//
// Example:
//
//	registry := risor.NewTypeRegistry().
//	    RegisterFromGo(reflect.TypeOf(User{}), func(v any) (object.Object, error) {
//	        u := v.(User)
//	        return object.NewMap(map[string]object.Object{
//	            "id":   object.NewInt(int64(u.ID)),
//	            "name": object.NewString(u.Name),
//	        }), nil
//	    }).
//	    Build()
//
//	result, _ := risor.Eval(ctx, source,
//	    risor.WithEnv(risor.Builtins()),
//	    risor.WithTypeRegistry(registry))
func NewTypeRegistry() *object.RegistryBuilder {
	return object.NewRegistryBuilder()
}

// Builtins returns a map of standard builtins and modules for Risor scripts.
// This includes only the builtins and modules that are always available,
// without pulling in additional Go dependencies.
//
// By default, the Risor environment is empty. Use this function to get the
// standard library:
//
//	result, _ := risor.Eval(ctx, source, risor.WithEnv(risor.Builtins()))
//
// To customize the environment, modify the returned map:
//
//	env := risor.Builtins()
//	env["myvar"] = myValue           // add custom variable
//	delete(env, "math")              // remove a module
//	result, _ := risor.Eval(ctx, source, risor.WithEnv(env))
func Builtins() map[string]any {
	env := map[string]any{}
	for k, v := range builtins.Builtins() {
		env[k] = v
	}
	for k, v := range defaultModules() {
		env[k] = v
	}
	return env
}

func defaultModules() map[string]object.Object {
	return map[string]object.Object{
		"math":   modMath.Module(),
		"rand":   modRand.Module(),
		"regexp": modRegexp.Module(),
		"time":   modTime.Module(),
	}
}

// Compile parses and compiles source code into executable bytecode.
// The returned Code is immutable and safe for concurrent use.
// Multiple goroutines can execute the same Code simultaneously.
func Compile(source string, opts ...Option) (*bytecode.Code, error) {
	o := collectOptions(opts...)

	var parserOpts []parser.Option
	if o.filename != "" {
		parserOpts = append(parserOpts, parser.WithFilename(o.filename))
	}
	ast, err := parser.Parse(context.Background(), source, parserOpts...)
	if err != nil {
		return nil, err
	}

	// Pass the original source to the compiler for better error messages
	cfg := o.compilerConfig()
	cfg.Source = source

	return compiler.Compile(ast, cfg)
}

// Run executes compiled bytecode and returns the result as a native Go value.
// Each call creates fresh runtime state, allowing concurrent execution of the
// same Code.
func Run(ctx context.Context, code *bytecode.Code, opts ...Option) (any, error) {
	o := collectOptions(opts...)
	result, err := vm.Run(ctx, code, o.vmOpts()...)
	if err != nil {
		return nil, err
	}
	// Convert to Go value
	interfaceVal := result.Interface()
	// For objects that don't have a Go equivalent (modules, closures),
	// return their string representation
	if interfaceVal == nil {
		if _, isNil := result.(*object.NilType); !isNil {
			return result.Inspect(), nil
		}
	}
	return interfaceVal, nil
}

// Eval is a convenience function that compiles and runs source code.
// It is equivalent to Compile() followed by Run().
// Returns the result as a native Go value.
func Eval(ctx context.Context, source string, opts ...Option) (any, error) {
	code, err := Compile(source, opts...)
	if err != nil {
		return nil, err
	}
	return Run(ctx, code, opts...)
}
