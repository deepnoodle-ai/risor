package regexp

import (
	"context"
	"fmt"
	"regexp"

	"github.com/risor-io/risor/object"
)

func Compile(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("regexp.compile: expected 1 argument, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	r, rErr := regexp.Compile(pattern)
	if rErr != nil {
		return nil, rErr
	}
	return NewRegexp(r), nil
}

func Match(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("regexp.match: expected 2 arguments, got %d", len(args))
	}
	pattern, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	str, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}
	matched, rErr := regexp.MatchString(pattern, str)
	if rErr != nil {
		return nil, rErr
	}
	return object.NewBool(matched), nil
}

func Module() *object.Module {
	return object.NewBuiltinsModule("regexp", map[string]object.Object{
		"compile": object.NewBuiltin("compile", Compile),
		"match":   object.NewBuiltin("match", Match),
	}, Compile)
}
