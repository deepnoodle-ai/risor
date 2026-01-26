package fmt

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/object"
)

func Errorf(ctx context.Context, args ...object.Object) object.Object {
	numArgs := len(args)
	if numArgs < 1 {
		return object.TypeErrorf("type error: fmt.errorf() takes 1 or more arguments (%d given)", len(args))
	}
	format, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	var values []interface{}
	for _, arg := range args[1:] {
		values = append(values, object.PrintableValue(arg))
	}
	return object.NewError(fmt.Errorf(format, values...)).WithRaised(false)
}

func Sprintf(ctx context.Context, args ...object.Object) object.Object {
	numArgs := len(args)
	if numArgs < 1 {
		return object.TypeErrorf("type error: fmt.sprintf() takes 1 or more arguments (%d given)", len(args))
	}
	format, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	var values []interface{}
	for _, arg := range args[1:] {
		values = append(values, object.PrintableValue(arg))
	}
	return object.NewString(fmt.Sprintf(format, values...))
}

func Builtins() map[string]object.Object {
	return map[string]object.Object{
		"errorf":  object.NewBuiltin("errorf", Errorf),
		"sprintf": object.NewBuiltin("sprintf", Sprintf),
	}
}

func Module() *object.Module {
	return object.NewBuiltinsModule("fmt", map[string]object.Object{
		"errorf":  object.NewBuiltin("errorf", Errorf),
		"sprintf": object.NewBuiltin("sprintf", Sprintf),
	})
}
