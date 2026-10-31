package object

import (
	"context"
	"fmt"
	"reflect"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

var _ Callable = (*GoFunc)(nil) // Ensure that *GoFunc implements Callable

// GoFunc wraps an arbitrary Go function for use in Risor.
// It uses reflection to handle argument conversion and function calls.
//
// A GoFunc equals only itself, and each conversion of a Go function gives a
// new value. So a func-typed Go struct field read twice gives two values that
// are not equal.
type GoFunc struct {
	fn         reflect.Value // The Go function
	fnType     reflect.Type  // Cached type info
	name       string        // For error messages
	numIn      int           // Input count (excluding context if present)
	isVariadic bool          // Whether the function is variadic
	hasContext bool          // First param is context.Context
	hasError   bool          // Last return is error
	registry   *TypeRegistry // For type conversion
	recv       reflect.Value // Receiver, if fn is a method bound by GoStruct
	method     int           // Method index on recv's type, if recv is set
}

// NewGoFunc creates a new GoFunc wrapping the given Go function.
// The registry is used for converting arguments and return values.
func NewGoFunc(fn reflect.Value, name string, registry *TypeRegistry) *GoFunc {
	fnType := fn.Type()
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("GoFunc: expected func, got %s", fnType.Kind()))
	}

	g := &GoFunc{
		fn:         fn,
		fnType:     fnType,
		name:       name,
		isVariadic: fnType.IsVariadic(),
		registry:   registry,
	}

	// Check if first param is context.Context
	if fnType.NumIn() > 0 && fnType.In(0).Implements(contextInterface) {
		g.hasContext = true
		g.numIn = fnType.NumIn() - 1
	} else {
		g.numIn = fnType.NumIn()
	}

	// Check if last return is error
	if fnType.NumOut() > 0 && fnType.Out(fnType.NumOut()-1).Implements(errorInterface) {
		g.hasError = true
	}

	return g
}

func (g *GoFunc) Type() Type {
	return GOFUNC
}

func (g *GoFunc) Inspect() string {
	return fmt.Sprintf("go_func(%s)", g.name)
}

func (g *GoFunc) String() string {
	return g.Inspect()
}

func (g *GoFunc) Interface() any {
	return g.fn.Interface()
}

func (g *GoFunc) Equals(other Object) bool {
	otherFunc, ok := other.(*GoFunc)
	if !ok {
		return false
	}
	if g.recv.IsValid() || otherFunc.recv.IsValid() {
		// All bound methods have the same code pointer, so compare the
		// receiver and method instead
		return g.recv.IsValid() && otherFunc.recv.IsValid() &&
			g.recv.Type() == otherFunc.recv.Type() &&
			g.recv.Pointer() == otherFunc.recv.Pointer() &&
			g.method == otherFunc.method
	}
	// Go closures made from one function literal share a code pointer, and
	// Go cannot compare func values, so compare by identity
	return g == otherFunc
}

func (g *GoFunc) Attrs() []AttrSpec {
	return nil
}

func (g *GoFunc) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (g *GoFunc) SetAttr(name string, value Object) error {
	return TypeErrorf("go_func has no attribute %q", name)
}

func (g *GoFunc) IsTruthy() bool {
	return true
}

func (g *GoFunc) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for go_func: %v", opType)
}

// Call invokes the wrapped Go function with the given Risor arguments.
// It handles context injection, argument conversion, variadic args, and error returns.
func (g *GoFunc) Call(ctx context.Context, args ...Object) (result Object, err error) {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in %s: %v", g.name, r)
			result = nil
		}
	}()

	// Validate argument count
	if err := g.validateArgCount(len(args)); err != nil {
		return nil, err
	}

	// Build the argument slice for reflection call
	callArgs, err := g.buildCallArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	// Call the function
	var results []reflect.Value
	if g.isVariadic {
		results = g.fn.CallSlice(callArgs)
	} else {
		results = g.fn.Call(callArgs)
	}

	return g.processResults(ctx, results)
}

func (g *GoFunc) validateArgCount(numArgs int) error {
	if g.isVariadic {
		// Variadic functions need at least numIn-1 arguments
		minArgs := g.numIn - 1
		if numArgs < minArgs {
			return fmt.Errorf("%s: expected at least %d argument(s), got %d", g.name, minArgs, numArgs)
		}
	} else {
		if numArgs != g.numIn {
			return fmt.Errorf("%s: expected %d argument(s), got %d", g.name, g.numIn, numArgs)
		}
	}
	return nil
}

func (g *GoFunc) buildCallArgs(ctx context.Context, args []Object) ([]reflect.Value, error) {
	fnType := g.fnType
	totalIn := fnType.NumIn()

	var callArgs []reflect.Value

	// All arguments share one element budget, so a spread call cannot
	// convert more than the list length limit in total.
	budget := newConvBudget(ctx)

	// Add context if needed
	startIdx := 0
	if g.hasContext {
		callArgs = append(callArgs, reflect.ValueOf(ctx))
		startIdx = 1
	}

	if g.isVariadic {
		// For variadic functions, we need to build a slice for the last parameter
		// Convert non-variadic arguments
		nonVariadicCount := g.numIn - 1
		for i := 0; i < nonVariadicCount; i++ {
			targetType := fnType.In(startIdx + i)
			goVal, err := g.registry.toGoValue(args[i], targetType, nil, budget)
			if err != nil {
				return nil, fmt.Errorf("%s: argument %d: %w", g.name, i+1, err)
			}
			callArgs = append(callArgs, goVal)
		}

		// Build variadic slice
		variadicType := fnType.In(totalIn - 1) // This is a slice type
		elemType := variadicType.Elem()
		variadicSlice := reflect.MakeSlice(variadicType, 0, len(args)-nonVariadicCount)
		for i := nonVariadicCount; i < len(args); i++ {
			goVal, err := g.registry.toGoValue(args[i], elemType, nil, budget)
			if err != nil {
				return nil, fmt.Errorf("%s: variadic argument %d: %w", g.name, i+1, err)
			}
			variadicSlice = reflect.Append(variadicSlice, goVal)
		}
		callArgs = append(callArgs, variadicSlice)
	} else {
		// Convert all arguments
		for i := 0; i < len(args); i++ {
			targetType := fnType.In(startIdx + i)
			goVal, err := g.registry.toGoValue(args[i], targetType, nil, budget)
			if err != nil {
				return nil, fmt.Errorf("%s: argument %d: %w", g.name, i+1, err)
			}
			callArgs = append(callArgs, goVal)
		}
	}

	return callArgs, nil
}

// processResults converts the function's results to Risor values with the
// limits in ctx. All results share one element budget, as the arguments do in
// buildCallArgs.
func (g *GoFunc) processResults(ctx context.Context, results []reflect.Value) (Object, error) {
	numOut := len(results)
	if numOut == 0 {
		return Nil, nil
	}

	// Check for error in last position
	if g.hasError {
		// IsZero, unlike IsNil, accepts an error of a value type, such as
		// syscall.Errno, whose zero value means no error.
		errVal := results[numOut-1]
		if !errVal.IsZero() {
			return nil, errVal.Interface().(error)
		}
		results = results[:numOut-1]
		numOut--
	}

	// No return values (or only error)
	if numOut == 0 {
		return Nil, nil
	}

	budget := newConvBudget(ctx)

	// Single return value
	if numOut == 1 {
		return g.registry.fromGoPath(results[0].Interface(), nil, budget)
	}

	// Multiple return values -> List
	items := make([]Object, numOut)
	for i, rv := range results {
		obj, err := g.registry.fromGoPath(rv.Interface(), nil, budget)
		if err != nil {
			return nil, fmt.Errorf("%s: return value %d: %w", g.name, i+1, err)
		}
		items[i] = obj
	}
	return NewList(items), nil
}

// Name returns the name of the wrapped function.
func (g *GoFunc) Name() string {
	return g.name
}
