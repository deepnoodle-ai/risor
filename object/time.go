package object

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/risor-io/risor/op"
)

// timeAttrs defines all attributes available on time objects.
var timeAttrs = []AttrSpec{
	{Name: "add_date", Doc: "Add years, months, and days", Args: []string{"years", "months", "days"}, Returns: "time"},
	{Name: "after", Doc: "Check if this time is after another", Args: []string{"other"}, Returns: "bool"},
	{Name: "before", Doc: "Check if this time is before another", Args: []string{"other"}, Returns: "bool"},
	{Name: "format", Doc: "Format time using layout string", Args: []string{"layout"}, Returns: "string"},
	{Name: "unix", Doc: "Get Unix timestamp (seconds)", Args: nil, Returns: "int"},
	{Name: "utc", Doc: "Convert to UTC timezone", Args: nil, Returns: "time"},
}

type Time struct {
	value time.Time
}

// Attrs returns the attribute specifications for time objects.
func (t *Time) Attrs() []AttrSpec {
	return timeAttrs
}

func (t *Time) SetAttr(name string, value Object) error {
	return TypeErrorf("time has no attribute %q", name)
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
		return 0, TypeErrorf("unable to compare time and %s", other.Type())
	}
	if t.value == otherStr.value {
		return 0, nil
	}
	if t.value.After(otherStr.value) {
		return 1, nil
	}
	return -1, nil
}

func (t *Time) Equals(other Object) bool {
	otherTime, ok := other.(*Time)
	if !ok {
		return false
	}
	return t.value == otherTime.value
}

func (t *Time) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for time: %v", opType)
}

func NewTime(t time.Time) *Time {
	return &Time{value: t}
}

func (t *Time) AddDate(ctx context.Context, args ...Object) (Object, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("time.add_date: expected 3 arguments, got %d", len(args))
	}

	years, err := AsInt(args[0])
	if err != nil {
		return nil, err
	}
	months, err := AsInt(args[1])
	if err != nil {
		return nil, err
	}
	days, err := AsInt(args[2])
	if err != nil {
		return nil, err
	}

	return NewTime(t.value.AddDate(int(years), int(months), int(days))), nil
}

func (t *Time) After(ctx context.Context, args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("time.after: expected 1 argument, got %d", len(args))
	}
	other, err := AsTime(args[0])
	if err != nil {
		return nil, err
	}
	return NewBool(t.value.After(other)), nil
}

func (t *Time) Before(ctx context.Context, args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("time.before: expected 1 argument, got %d", len(args))
	}
	other, err := AsTime(args[0])
	if err != nil {
		return nil, err
	}
	return NewBool(t.value.Before(other)), nil
}

func (t *Time) Format(ctx context.Context, args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("time.format: expected 1 argument, got %d", len(args))
	}
	layout, err := AsString(args[0])
	if err != nil {
		return nil, err
	}
	return NewString(t.value.Format(layout)), nil
}

func (t *Time) UTC(ctx context.Context, args ...Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("time.utc: expected 0 arguments, got %d", len(args))
	}
	return NewTime(t.value.UTC()), nil
}

func (t *Time) Unix(ctx context.Context, args ...Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("time.unix: expected 0 arguments, got %d", len(args))
	}
	return NewInt(t.value.Unix()), nil
}

func (t *Time) IsTruthy() bool {
	return !t.value.IsZero()
}

func (t *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.value.Format(time.RFC3339))
}
