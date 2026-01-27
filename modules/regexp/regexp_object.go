package regexp

import (
	"context"
	"fmt"
	"regexp"

	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/op"
)

const REGEXP object.Type = "regexp"

type Regexp struct {
	value *regexp.Regexp
}

func (r *Regexp) Type() object.Type {
	return REGEXP
}

func (r *Regexp) Inspect() string {
	return fmt.Sprintf("regexp(%q)", r.value.String())
}

func (r *Regexp) String() string {
	return r.Inspect()
}

func (r *Regexp) Interface() interface{} {
	return r.value
}

func (r *Regexp) Compare(other object.Object) (int, error) {
	typeComp := object.CompareTypes(r, other)
	if typeComp != 0 {
		return typeComp, nil
	}
	otherRegex := other.(*Regexp)
	if r.value == otherRegex.value {
		return 0, nil
	}
	if r.value.String() > otherRegex.value.String() {
		return 1, nil
	}
	return -1, nil
}

func (r *Regexp) Equals(other object.Object) bool {
	switch other := other.(type) {
	case *Regexp:
		return r.value == other.value
	}
	return false
}

func (r *Regexp) MarshalJSON() ([]byte, error) {
	return []byte(r.value.String()), nil
}

func (r *Regexp) RunOperation(opType op.BinaryOpType, right object.Object) (object.Object, error) {
	return nil, fmt.Errorf("type error: unsupported operation for regexp: %v", opType)
}

func (r *Regexp) SetAttr(name string, value object.Object) error {
	return fmt.Errorf("type error: cannot set attribute %q on regexp object", name)
}

func (r *Regexp) IsTruthy() bool {
	return true
}

func (r *Regexp) GetAttr(name string) (object.Object, bool) {
	switch name {
	case "match":
		return object.NewBuiltin("regexp.match",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("regexp.match: expected 1 argument, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				return object.NewBool(r.value.MatchString(strValue)), nil
			},
		), true
	case "find":
		return object.NewBuiltin("regexp.find",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("regexp.find: expected 1 argument, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				return object.NewString(r.value.FindString(strValue)), nil
			},
		), true
	case "find_all":
		return object.NewBuiltin("regexp.find_all",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) < 1 || len(args) > 2 {
					return nil, fmt.Errorf("regexp.find_all: expected 1-2 arguments, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				n := -1
				if len(args) == 2 {
					i64, err := object.AsInt(args[1])
					if err != nil {
						return nil, err
					}
					n = int(i64)
				}
				var matches []object.Object
				for _, match := range r.value.FindAllString(strValue, n) {
					matches = append(matches, object.NewString(match))
				}
				return object.NewList(matches), nil
			},
		), true
	case "find_submatch":
		return object.NewBuiltin("regexp.find_submatch",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("regexp.find_submatch: expected 1 argument, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				var matches []object.Object
				for _, match := range r.value.FindStringSubmatch(strValue) {
					matches = append(matches, object.NewString(match))
				}
				return object.NewList(matches), nil
			},
		), true
	case "replace_all":
		return object.NewBuiltin("regexp.replace_all",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("regexp.replace_all: expected 2 arguments, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				replaceValue, err := object.AsString(args[1])
				if err != nil {
					return nil, err
				}
				return object.NewString(r.value.ReplaceAllString(strValue, replaceValue)), nil
			},
		), true
	case "split":
		return object.NewBuiltin("regexp.split",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) < 1 || len(args) > 2 {
					return nil, fmt.Errorf("regexp.split: expected 1-2 arguments, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				n := -1
				if len(args) == 2 {
					i64, err := object.AsInt(args[1])
					if err != nil {
						return nil, err
					}
					n = int(i64)
				}
				matches := r.value.Split(strValue, n)
				matchObjects := make([]object.Object, 0, len(matches))
				for _, match := range matches {
					matchObjects = append(matchObjects, object.NewString(match))
				}
				return object.NewList(matchObjects), nil
			},
		), true
	}
	return nil, false
}

func NewRegexp(value *regexp.Regexp) *Regexp {
	return &Regexp{value: value}
}
