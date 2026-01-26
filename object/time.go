package object

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/risor-io/risor/op"
)

type Time struct {
	*base
	value time.Time
}

func (t *Time) Type() Type {
	return TIME
}

func (t *Time) Value() time.Time {
	return t.value
}

func (t *Time) Inspect() string {
	return fmt.Sprintf("time(%q)", t.value.Format(time.RFC3339))
}

func (t *Time) GetAttr(name string) (Object, bool) {
	switch name {
	case "add_date":
		return NewBuiltin("time.add_date", t.AddDate), true
	case "before":
		return NewBuiltin("time.before", t.Before), true
	case "after":
		return NewBuiltin("time.after", t.After), true
	case "format":
		return NewBuiltin("time.format", t.Format), true
	case "utc":
		return NewBuiltin("time.utc", t.UTC), true
	case "unix":
		return NewBuiltin("time.unix", t.Unix), true
	default:
		return nil, false
	}
}

func (t *Time) Interface() interface{} {
	return t.value
}

func (t *Time) String() string {
	return t.Inspect()
}

func (t *Time) Compare(other Object) (int, error) {
	otherStr, ok := other.(*Time)
	if !ok {
		return 0, TypeErrorf("type error: unable to compare time and %s", other.Type())
	}
	if t.value == otherStr.value {
		return 0, nil
	}
	if t.value.After(otherStr.value) {
		return 1, nil
	}
	return -1, nil
}

func (t *Time) Equals(other Object) Object {
	if other.Type() == TIME && t.value == other.(*Time).value {
		return True
	}
	return False
}

func (t *Time) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for time: %v", opType)
}

func NewTime(t time.Time) *Time {
	return &Time{value: t}
}

func (t *Time) AddDate(ctx context.Context, args ...Object) Object {
	if len(args) != 3 {
		return NewArgsError("time.add_date", 3, len(args))
	}

	years, err := AsInt(args[0])
	if err != nil {
		return err
	}
	months, err := AsInt(args[1])
	if err != nil {
		return err
	}
	days, err := AsInt(args[2])
	if err != nil {
		return err
	}

	return NewTime(t.value.AddDate(int(years), int(months), int(days)))
}

func (t *Time) After(ctx context.Context, args ...Object) Object {
	if len(args) != 1 {
		return NewArgsError("time.after", 1, len(args))
	}
	other, err := AsTime(args[0])
	if err != nil {
		return err
	}
	return NewBool(t.value.After(other))
}

func (t *Time) Before(ctx context.Context, args ...Object) Object {
	if len(args) != 1 {
		return NewArgsError("time.before", 1, len(args))
	}
	other, err := AsTime(args[0])
	if err != nil {
		return err
	}
	return NewBool(t.value.Before(other))
}

func (t *Time) Format(ctx context.Context, args ...Object) Object {
	if len(args) != 1 {
		return NewArgsError("time.format", 1, len(args))
	}
	layout, err := AsString(args[0])
	if err != nil {
		return err
	}
	return NewString(t.value.Format(layout))
}

func (t *Time) UTC(ctx context.Context, args ...Object) Object {
	if len(args) != 0 {
		return NewArgsError("time.utc", 0, len(args))
	}
	return NewTime(t.value.UTC())
}

func (t *Time) Unix(ctx context.Context, args ...Object) Object {
	if len(args) != 0 {
		return NewArgsError("time.unix", 0, len(args))
	}
	return NewInt(t.value.Unix())
}

func (t *Time) IsTruthy() bool {
	return !t.value.IsZero()
}

func (t *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.value.Format(time.RFC3339))
}
