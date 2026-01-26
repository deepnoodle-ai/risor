package regexp

import (
	"context"
	"regexp"

	"github.com/risor-io/risor/object"
)

func Compile(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("regexp.compile", 1, args); err != nil {
		return err
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return object.NewError(rErr)
	}
	return NewRegexp(r)
}

func Match(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("regexp.match", 2, args); err != nil {
		return err
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	matched, rErr := regexp.MatchString(pattern, str)
	if rErr != nil {
		return object.NewError(rErr)
	}
	return object.NewBool(matched)
}

func Module() *object.Module {
	return object.NewBuiltinsModule("regexp", map[string]object.Object{
		"compile": object.NewBuiltin("compile", Compile),
		"match":   object.NewBuiltin("match", Match),
	}, Compile)
}
