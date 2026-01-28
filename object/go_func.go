package object

import (
	"context"
	"fmt"
	"reflect"

	"github.com/risor-io/risor/op"
)

var _ Callable = (*GoFunc)(nil) // Ensure that *GoFunc implements Callable

// GoFunc wraps an arbitrary Go function for use in Risor.
// It uses reflection to handle argument conversion and function calls.
type GoFunc struct {
	fn         reflect.Value // The Go function
	fnType     reflect.Type  // Cached type info
	name       string        // For error messages
	numIn      int           // Input count (excluding context if present)
	isVariadic bool          // Whether the function is variadic
	hasContext bool          // First param is context.Context
	hasError   bool          // Last return is error
	registry   *TypeRegistry // For type conversion
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
	return g.fn.Pointer() == otherFunc.fn.Pointer()
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

	return g.processResults(results)
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
			goVal, err := g.registry.ToGo(args[i], targetType)
			if err != nil {
				return nil, fmt.Errorf("%s: argument %d: %w", g.name, i+1, err)
			}
			callArgs = append(callArgs, reflect.ValueOf(goVal))
		}

		// Build variadic slice
		variadicType := fnType.In(totalIn - 1) // This is a slice type
		elemType := variadicType.Elem()
		variadicSlice := reflect.MakeSlice(variadicType, 0, len(args)-nonVariadicCount)
		for i := nonVariadicCount; i < len(args); i++ {
			goVal, err := g.registry.ToGo(args[i], elemType)
			if err != nil {
				return nil, fmt.Errorf("%s: variadic argument %d: %w", g.name, i+1, err)
			}
			variadicSlice = reflect.Append(variadicSlice, reflect.ValueOf(goVal))
		}
		callArgs = append(callArgs, variadicSlice)
	} else {
		// Convert all arguments
		for i := 0; i < len(args); i++ {
			targetType := fnType.In(startIdx + i)
			goVal, err := g.registry.ToGo(args[i], targetType)
			if err != nil {
				return nil, fmt.Errorf("%s: argument %d: %w", g.name, i+1, err)
			}
			callArgs = append(callArgs, reflect.ValueOf(goVal))
		}
	}

	return callArgs, nil
}

func (g *GoFunc) processResults(results []reflect.Value) (Object, error) {
	numOut := len(results)
	if numOut == 0 {
		return Nil, nil
	}

	// Check for error in last position
	if g.hasError {
		errVal := results[numOut-1]
		if !errVal.IsNil() {
			return nil, errVal.Interface().(error)
		}
		results = results[:numOut-1]
		numOut--
	}

	// No return values (or only error)
	if numOut == 0 {
		return Nil, nil
	}

	// Single return value
	if numOut == 1 {
		return g.registry.FromGo(results[0].Interface())
	}

	// Multiple return values -> List
	items := make([]Object, numOut)
	for i, rv := range results {
		obj, err := g.registry.FromGo(rv.Interface())
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
