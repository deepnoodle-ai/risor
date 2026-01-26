package time

import (
	"context"
	"time"

	"github.com/risor-io/risor/object"
)

func Now(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("time.now", 0, args); err != nil {
		return err
	}
	return object.NewTime(time.Now())
}

func Unix(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("time.unix", 2, args); err != nil {
		return err
	}
	sec, err := object.AsInt(args[0])
	if err != nil {
		return err
	}
	nsec, err := object.AsInt(args[1])
	if err != nil {
		return err
	}
	return object.NewTime(time.Unix(sec, nsec))
}

func Parse(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("time.parse", 2, args); err != nil {
		return err
	}
	layout, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	value, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	t, parseErr := time.Parse(layout, value)
	if parseErr != nil {
		return object.NewError(parseErr)
	}
	return object.NewTime(t)
}

func Since(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("time.since", 1, args); err != nil {
		return err
	}
	t, err := object.AsTime(args[0])
	if err != nil {
		return err
	}
	return object.NewFloat(time.Since(t).Seconds())
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
