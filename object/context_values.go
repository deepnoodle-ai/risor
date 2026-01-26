package object

import (
	"context"
)

type contextKey string

// CallFunc is a type signature for a function that can call a Risor function.
type CallFunc func(ctx context.Context, fn *Closure, args []Object) (Object, error)

////////////////////////////////////////////////////////////////////////////////

const callFuncKey = contextKey("risor:call")

// WithCallFunc adds an CallFunc to the context, which can be used by
// objects to call a Risor function at runtime.
func WithCallFunc(ctx context.Context, fn CallFunc) context.Context {
	return context.WithValue(ctx, callFuncKey, fn)
}

// GetCallFunc returns the CallFunc from the context, if it exists.
func GetCallFunc(ctx context.Context) (CallFunc, bool) {
	if fn, ok := ctx.Value(callFuncKey).(CallFunc); ok {
		if fn != nil {
			return fn, ok
		}
	}
	return nil, false
}
