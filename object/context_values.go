package object

import (
	"context"
)

type contextKey string

// CallFunc is a type signature for a function that can call a Risor function.
type CallFunc func(ctx context.Context, fn *Function, args []Object) (Object, error)

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

////////////////////////////////////////////////////////////////////////////////

const cloneCallKey = contextKey("risor:clone-call")

// WithCloneCallFunc returns a context with a "clone-call" function
// associated. This function can be used to clone a Risor VM and then call a
// function on it synchronously.
func WithCloneCallFunc(ctx context.Context, fn CallFunc) context.Context {
	return context.WithValue(ctx, cloneCallKey, fn)
}

// GetCloneCallFunc returns the "clone-call" function from the context,
// if it exists. This function can be used to clone a Risor VM and then call a
// function on it synchronously.
func GetCloneCallFunc(ctx context.Context) (CallFunc, bool) {
	if fn, ok := ctx.Value(cloneCallKey).(CallFunc); ok {
		if fn != nil {
			return fn, ok
		}
	}
	return nil, false
}
