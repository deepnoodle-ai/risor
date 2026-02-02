package object

import (
	"context"
)

type contextKey string

// CallFunc is a function type for invoking closures. The VM registers its
// implementation via WithCallFunc, and Closure.Call() retrieves it to execute
// the closure's bytecode.
//
// External code should use the Callable interface instead of CallFunc directly.
// Both *Builtin and *Closure implement Callable, providing a uniform way to
// invoke functions without knowing their concrete type.
type CallFunc func(ctx context.Context, fn *Closure, args []Object) (Object, error)

////////////////////////////////////////////////////////////////////////////////

const callFuncKey = contextKey("risor:call")

// WithCallFunc stores a CallFunc in the context. Called by the VM during
// initialization to enable Closure.Call() to execute bytecode.
func WithCallFunc(ctx context.Context, fn CallFunc) context.Context {
	return context.WithValue(ctx, callFuncKey, fn)
}

// GetCallFunc retrieves the CallFunc from the context. Used internally by
// Closure.Call() to execute the closure's bytecode. External code should
// use the Callable interface instead.
func GetCallFunc(ctx context.Context) (CallFunc, bool) {
	if fn, ok := ctx.Value(callFuncKey).(CallFunc); ok {
		if fn != nil {
			return fn, ok
		}
	}
	return nil, false
}
