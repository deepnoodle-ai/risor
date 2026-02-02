package object

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

var timeMethods = NewMethodRegistry[*Time]("time")

func init() {
	timeMethods.Define("add_date").
		Doc("Add years, months, and days").
		Args("years", "months", "days").
		Returns("time").
		Impl((*Time).AddDate)

	timeMethods.Define("after").
		Doc("Check if this time is after another").
		Arg("other").
		Returns("bool").
		Impl((*Time).After)

	timeMethods.Define("before").
		Doc("Check if this time is before another").
		Arg("other").
		Returns("bool").
		Impl((*Time).Before)

	timeMethods.Define("format").
		Doc("Format time using layout string").
		Arg("layout").
		Returns("string").
		Impl((*Time).Format)

	timeMethods.Define("unix").
		Doc("Get Unix timestamp (seconds)").
		Returns("int").
		Impl((*Time).Unix)

	timeMethods.Define("utc").
		Doc("Convert to UTC timezone").
		Returns("time").
		Impl((*Time).UTC)
}

type Time struct {
	value time.Time
}

func (t *Time) Attrs() []AttrSpec {
	return timeMethods.Specs()
}

func (t *Time) GetAttr(name string) (Object, bool) {
	return timeMethods.GetAttr(t, name)
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

func (t *Time) Interface() interface{} {
	return t.value
}

func (t *Time) String() string {
	return t.Inspect()
}

func (t *Time) Compare(other Object) (int, error) {
	otherTime, ok := other.(*Time)
	if !ok {
		return 0, TypeErrorf("unable to compare time and %s", other.Type())
	}
	if t.value.Equal(otherTime.value) {
		return 0, nil
	}
	if t.value.After(otherTime.value) {
		return 1, nil
	}
	return -1, nil
}

func (t *Time) Equals(other Object) bool {
	otherTime, ok := other.(*Time)
	if !ok {
		return false
	}
	return t.value.Equal(otherTime.value)
}

func (t *Time) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for time: %v", opType)
}

func NewTime(t time.Time) *Time {
	return &Time{value: t}
}

func (t *Time) AddDate(ctx context.Context, args ...Object) (Object, error) {
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
	other, err := AsTime(args[0])
	if err != nil {
		return nil, err
	}
	return NewBool(t.value.After(other)), nil
}

func (t *Time) Before(ctx context.Context, args ...Object) (Object, error) {
	other, err := AsTime(args[0])
	if err != nil {
		return nil, err
	}
	return NewBool(t.value.Before(other)), nil
}

func (t *Time) Format(ctx context.Context, args ...Object) (Object, error) {
	layout, err := AsString(args[0])
	if err != nil {
		return nil, err
	}
	return NewString(t.value.Format(layout)), nil
}

func (t *Time) UTC(ctx context.Context, args ...Object) (Object, error) {
	return NewTime(t.value.UTC()), nil
}

func (t *Time) Unix(ctx context.Context, args ...Object) (Object, error) {
	return NewInt(t.value.Unix()), nil
}

func (t *Time) IsTruthy() bool {
	return !t.value.IsZero()
}

func (t *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.value.Format(time.RFC3339))
}
