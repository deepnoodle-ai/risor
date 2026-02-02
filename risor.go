package risor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/deepnoodle-ai/risor/v2/builtins"
	"github.com/deepnoodle-ai/risor/v2/bytecode"
	"github.com/deepnoodle-ai/risor/v2/compiler"
	modMath "github.com/deepnoodle-ai/risor/v2/modules/math"
	modRand "github.com/deepnoodle-ai/risor/v2/modules/rand"
	modRegexp "github.com/deepnoodle-ai/risor/v2/modules/regexp"
	"github.com/deepnoodle-ai/risor/v2/object"
	"github.com/deepnoodle-ai/risor/v2/parser"
	"github.com/deepnoodle-ai/risor/v2/syntax"
	"github.com/deepnoodle-ai/risor/v2/vm"
)

// Sentinel errors for resource limits.
var (
	ErrStepLimitExceeded = vm.ErrStepLimitExceeded
	ErrStackOverflow     = vm.ErrStackOverflow
)

// ErrNilCode is returned when Run is called with a nil Code.
var ErrNilCode = errors.New("code is nil")

// Re-export syntax types for convenient access.
type (
	SyntaxConfig     = syntax.SyntaxConfig
	Validator        = syntax.Validator
	ValidatorFunc    = syntax.ValidatorFunc
	ValidationError  = syntax.ValidationError
	ValidationErrors = syntax.ValidationErrors
	Transformer      = syntax.Transformer
	TransformerFunc  = syntax.TransformerFunc
)

// Re-export presets.
var (
	ExpressionOnly = syntax.ExpressionOnly
	BasicScripting = syntax.BasicScripting
	FullLanguage   = syntax.FullLanguage
)

// Option configures a Risor compilation or execution.
type Option func(*options)

type options struct {
	env          map[string]any
	filename     string
	observer     vm.Observer
	typeRegistry *object.TypeRegistry
	rawResult    bool
	// Resource limits
	maxSteps      int64
	maxStackDepth int
	timeout       time.Duration
	// AST validation and transformation
	syntaxConfig *syntax.SyntaxConfig
	validators   []syntax.Validator
	transformers []syntax.Transformer
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
	if o.maxSteps > 0 {
		opts = append(opts, vm.WithMaxSteps(o.maxSteps))
	}
	if o.maxStackDepth > 0 {
		opts = append(opts, vm.WithMaxStackDepth(o.maxStackDepth))
	}
	if o.timeout > 0 {
		opts = append(opts, vm.WithTimeout(o.timeout))
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
//
// # Concurrency Contract
//
// The env map is shallow-copied when this option is applied, but the values
// within the map are shared with the caller. This has several implications:
//
//   - The caller retains a reference to mutable objects passed in the env
//   - Built-in functions must be thread-safe as they may be invoked concurrently
//   - Mutable objects (slices, maps, custom types) in env are the caller's
//     responsibility to synchronize if concurrent access is possible
//   - Each VM execution gets its own shallow copy of the globals map,
//     but the underlying objects are shared
//
// For safe concurrent execution, either:
//   - Use immutable values in the environment
//   - Create fresh environment maps for each concurrent execution
//   - Synchronize access to mutable objects externally
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

// WithRawResult configures Run and Eval to return the result as an
// object.Object instead of converting it to a native Go type.
//
// By default, results are converted as follows:
//   - NilType returns nil
//   - String returns string
//   - Int returns int64
//   - Float returns float64
//   - Bool returns bool
//   - List returns []any
//   - Map returns map[string]any
//   - Other types (closures, modules, etc.) return their Inspect() string
//
// With WithRawResult(), the object.Object is returned directly, allowing
// embedders to:
//   - Inspect the exact Risor type
//   - Access type-specific methods
//   - Avoid the overhead of conversion
//   - Handle types that don't have a Go equivalent
//
// Example:
//
//	result, _ := risor.Eval(ctx, `[1, 2, 3]`, risor.WithRawResult())
//	list := result.(*object.List)
//	fmt.Println(list.Len()) // 3
func WithRawResult() Option {
	return func(o *options) {
		o.rawResult = true
	}
}

// WithMaxSteps sets the maximum number of instructions the VM will execute.
// If the limit is exceeded, the VM returns ErrStepLimitExceeded.
// A value of 0 (default) means unlimited.
//
// Example:
//
//	result, err := risor.Eval(ctx, source, risor.WithMaxSteps(10000))
//	if errors.Is(err, risor.ErrStepLimitExceeded) {
//	    // Handle step limit exceeded
//	}
func WithMaxSteps(n int64) Option {
	return func(o *options) {
		o.maxSteps = n
	}
}

// WithMaxStackDepth sets the maximum stack depth for execution.
// If exceeded, the VM returns ErrStackOverflow.
// A value of 0 (default) uses the VM's default limit.
//
// Example:
//
//	result, err := risor.Eval(ctx, source, risor.WithMaxStackDepth(100))
//	if errors.Is(err, risor.ErrStackOverflow) {
//	    // Handle stack overflow
//	}
func WithMaxStackDepth(n int) Option {
	return func(o *options) {
		o.maxStackDepth = n
	}
}

// WithTimeout sets a timeout for script execution.
// If the timeout is exceeded, the VM returns context.DeadlineExceeded.
// A value of 0 (default) means no timeout.
//
// Example:
//
//	ctx := context.Background()
//	result, err := risor.Eval(ctx, source, risor.WithTimeout(100*time.Millisecond))
//	if errors.Is(err, context.DeadlineExceeded) {
//	    // Handle timeout
//	}
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
	}
}

// WithSyntax applies a syntax configuration that restricts allowed constructs.
// The validator runs after parsing and before any transformers.
//
// Example:
//
//	// Only allow expressions - no variable declarations or control flow
//	result, err := risor.Eval(ctx, "price * quantity",
//	    risor.WithEnv(map[string]any{"price": 100.0, "quantity": 5}),
//	    risor.WithSyntax(risor.ExpressionOnly),
//	)
func WithSyntax(config SyntaxConfig) Option {
	return func(o *options) {
		o.syntaxConfig = &config
	}
}

// WithValidator adds a custom validator to run after parsing.
// Multiple validators can be added; they run in order.
// Validation runs before transformation.
//
// Example:
//
//	// Disallow accessing the "secret" variable
//	noSecrets := risor.ValidatorFunc(func(p *ast.Program) []risor.ValidationError {
//	    var errs []risor.ValidationError
//	    for node := range ast.Preorder(p) {
//	        if ident, ok := node.(*ast.Ident); ok && ident.Name == "secret" {
//	            errs = append(errs, risor.ValidationError{
//	                Message:  "access to 'secret' is not allowed",
//	                Node:     node,
//	                Position: node.Pos(),
//	            })
//	        }
//	    }
//	    return errs
//	})
func WithValidator(v Validator) Option {
	return func(o *options) {
		o.validators = append(o.validators, v)
	}
}

// WithTransform adds a transformer to run after validation.
// Multiple transformers can be added; they run in order.
// Each transformer receives the output of the previous one.
//
// Example:
//
//	// Double all integer literals
//	doubler := risor.TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
//	    // Walk and transform...
//	    return p, nil
//	})
//	result, err := risor.Eval(ctx, source, risor.WithTransform(doubler))
func WithTransform(t Transformer) Option {
	return func(o *options) {
		o.transformers = append(o.transformers, t)
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
	}
}

// validateGlobals checks that the env keys match the globals expected by the
// bytecode. Returns an error if there's a mismatch.
//
// This uses EnvKeys() which tracks only the globals that were provided via
// the environment at compile time (not globals defined within the script).
func validateGlobals(code *bytecode.Code, env map[string]any) error {
	// EnvKeys returns only the globals from compile-time env,
	// not script-defined globals like functions or let bindings.
	required := code.EnvKeys()
	if len(required) == 0 {
		return nil // No external globals were required at compile time
	}

	// Build a set of provided keys
	provided := make(map[string]bool, len(env))
	for k := range env {
		provided[k] = true
	}

	// Find any required keys that are missing
	var missing []string
	for _, name := range required {
		if !provided[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required globals: %v (bytecode was compiled with: %v)",
			missing, required)
	}

	return nil
}

// Compile parses and compiles source code into executable bytecode.
// The returned Code is immutable and safe for concurrent use.
// Multiple goroutines can execute the same Code simultaneously.
//
// # Global Name Binding
//
// The compiled bytecode is bound to the specific global names present in the
// environment at compile time. Global variable references are resolved by
// index, not by name, during execution. This means:
//
//   - The same Code can be reused with different env maps that have the same
//     keys (values may differ)
//   - Using Code with an env that has different keys will cause undefined
//     behavior (wrong values or panics)
//   - To introspect which globals a Code requires, use Code.GlobalNames()
//
// Example:
//
//	// Compile with env keys "x" and "y"
//	code, _ := risor.Compile(ctx, "x + y", risor.WithEnv(map[string]any{"x": 1, "y": 2}))
//
//	// OK: same keys, different values
//	result1, _ := risor.Run(ctx, code, risor.WithEnv(map[string]any{"x": 10, "y": 20}))
//
//	// NOT OK: different keys - will fail or produce wrong results
//	// risor.Run(ctx, code, risor.WithEnv(map[string]any{"a": 1, "b": 2}))
//
//	// Introspect required globals
//	names := code.GlobalNames() // returns []string{"x", "y"}
func Compile(ctx context.Context, source string, opts ...Option) (*bytecode.Code, error) {
	o := collectOptions(opts...)

	var parserCfg *parser.Config
	if o.filename != "" {
		parserCfg = &parser.Config{Filename: o.filename}
	}
	program, err := parser.Parse(ctx, source, parserCfg)
	if err != nil {
		return nil, err
	}

	// Validate syntax config (if specified)
	if o.syntaxConfig != nil {
		validator := syntax.NewSyntaxValidator(*o.syntaxConfig)
		if errs := validator.Validate(program); len(errs) > 0 {
			return nil, syntax.NewValidationErrors(errs)
		}
	}

	// Run custom validators
	for _, v := range o.validators {
		if errs := v.Validate(program); len(errs) > 0 {
			return nil, syntax.NewValidationErrors(errs)
		}
	}

	// Run transformers
	for _, t := range o.transformers {
		program, err = t.Transform(program)
		if err != nil {
			return nil, err
		}
	}

	// Pass the original source to the compiler for better error messages
	cfg := o.compilerConfig()
	cfg.Source = source

	return compiler.Compile(program, cfg)
}

// Run executes compiled bytecode and returns the result.
// Each call creates fresh runtime state, allowing concurrent execution of the
// same Code.
//
// # Global Name Validation
//
// Run validates that the env keys match the globals expected by the bytecode.
// If there's a mismatch, Run returns an error explaining what's wrong.
// This prevents subtle bugs where the wrong values are accessed at runtime.
//
// # Result Conversion
//
// By default, the result is converted to a native Go value using these rules:
//
//   - NilType → nil
//   - String → string
//   - Int → int64
//   - Float → float64
//   - Bool → bool
//   - Byte → byte
//   - Bytes → []byte
//   - List → []any (elements recursively converted)
//   - Map → map[string]any (values recursively converted)
//   - Time → time.Time
//
// For types without a Go equivalent (closures, modules, errors, etc.),
// the Inspect() string representation is returned.
//
// Use WithRawResult() to receive the object.Object directly without conversion.
func Run(ctx context.Context, code *bytecode.Code, opts ...Option) (any, error) {
	if code == nil {
		return nil, ErrNilCode
	}

	o := collectOptions(opts...)

	// Validate that env keys match the globals expected by the bytecode
	if err := validateGlobals(code, o.env); err != nil {
		return nil, err
	}

	result, err := vm.Run(ctx, code, o.vmOpts()...)
	if err != nil {
		return nil, err
	}

	// Return raw object.Object if requested
	if o.rawResult {
		return result, nil
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
//
// The result is converted to a native Go value by default. See Run for
// the conversion rules. Use WithRawResult() to receive the object.Object
// directly.
func Eval(ctx context.Context, source string, opts ...Option) (any, error) {
	code, err := Compile(ctx, source, opts...)
	if err != nil {
		return nil, err
	}
	return Run(ctx, code, opts...)
}
