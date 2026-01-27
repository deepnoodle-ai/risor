package time

import (
	"context"
	"fmt"
	"time"

	"github.com/risor-io/risor/object"
)

func Now(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("time.now: expected 0 arguments, got %d", len(args))
	}
	return object.NewTime(time.Now()), nil
}

func Unix(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("time.unix: expected 2 arguments, got %d", len(args))
	}
	sec, err := object.AsInt(args[0])
	if err != nil {
		return nil, err
	}
	nsec, err := object.AsInt(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewTime(time.Unix(sec, nsec)), nil
}

func Parse(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("time.parse: expected 2 arguments, got %d", len(args))
	}
	layout, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	value, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}
	t, parseErr := time.Parse(layout, value)
	if parseErr != nil {
		return nil, parseErr
	}
	return object.NewTime(t), nil
}

func Since(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("time.since: expected 1 argument, got %d", len(args))
	}
	t, err := object.AsTime(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(time.Since(t).Seconds()), nil
}

func Module() *object.Module {
	return object.NewBuiltinsModule("time", map[string]object.Object{
		"now":         object.NewBuiltin("now", Now),
		"parse":       object.NewBuiltin("parse", Parse),
		"since":       object.NewBuiltin("since", Since),
		"unix":        object.NewBuiltin("unix", Unix),
		"ANSIC":       object.NewString(time.ANSIC),
		"UnixDate":    object.NewString(time.UnixDate),
		"RubyDate":    object.NewString(time.RubyDate),
		"RFC822":      object.NewString(time.RFC822),
		"RFC822Z":     object.NewString(time.RFC822Z),
		"RFC850":      object.NewString(time.RFC850),
		"RFC1123":     object.NewString(time.RFC1123),
		"RFC1123Z":    object.NewString(time.RFC1123Z),
		"RFC3339":     object.NewString(time.RFC3339),
		"RFC3339Nano": object.NewString(time.RFC3339Nano),
		"Kitchen":     object.NewString(time.Kitchen),
		"Stamp":       object.NewString(time.Stamp),
		"StampMilli":  object.NewString(time.StampMilli),
		"StampMicro":  object.NewString(time.StampMicro),
		"StampNano":   object.NewString(time.StampNano),
	})
}
