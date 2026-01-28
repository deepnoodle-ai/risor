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
	return nil, object.TypeErrorf("unsupported operation for regexp: %v", opType)
}

func (r *Regexp) SetAttr(name string, value object.Object) error {
	return object.TypeErrorf("cannot set attribute %q on regexp object", name)
}

func (r *Regexp) IsTruthy() bool {
	return true
}

func (r *Regexp) Attrs() []object.AttrSpec {
	// TODO: Migrate to AttrRegistry for introspection support
	return nil
}

func (r *Regexp) GetAttr(name string) (object.Object, bool) {
	switch name {
	// Testing
	case "test", "match":
		return object.NewBuiltin("regexp.test",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("regexp.test: expected 1 argument, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				return object.NewBool(r.value.MatchString(strValue)), nil
			},
		), true

	// Finding first match
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
				match := r.value.FindString(strValue)
				if match == "" && !r.value.MatchString(strValue) {
					return object.Nil, nil
				}
				return object.NewString(match), nil
			},
		), true

	// Finding all matches
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

	// Search for index
	case "search":
		return object.NewBuiltin("regexp.search",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("regexp.search: expected 1 argument, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				loc := r.value.FindStringIndex(strValue)
				if loc == nil {
					return object.NewInt(-1), nil
				}
				// Convert byte index to rune index for Unicode correctness
				runeIndex := len([]rune(strValue[:loc[0]]))
				return object.NewInt(int64(runeIndex)), nil
			},
		), true

	// Capture groups (renamed from find_submatch)
	case "groups":
		return object.NewBuiltin("regexp.groups",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("regexp.groups: expected 1 argument, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				submatches := r.value.FindStringSubmatch(strValue)
				if submatches == nil {
					return object.Nil, nil
				}
				var matches []object.Object
				for _, match := range submatches {
					matches = append(matches, object.NewString(match))
				}
				return object.NewList(matches), nil
			},
		), true

	// All matches with capture groups
	case "find_all_groups":
		return object.NewBuiltin("regexp.find_all_groups",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) < 1 || len(args) > 2 {
					return nil, fmt.Errorf("regexp.find_all_groups: expected 1-2 arguments, got %d", len(args))
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
				allSubmatches := r.value.FindAllStringSubmatch(strValue, n)
				var result []object.Object
				for _, submatches := range allSubmatches {
					var group []object.Object
					for _, match := range submatches {
						group = append(group, object.NewString(match))
					}
					result = append(result, object.NewList(group))
				}
				return object.NewList(result), nil
			},
		), true

	// Replace with optional count
	case "replace":
		return object.NewBuiltin("regexp.replace",
			func(ctx context.Context, args ...object.Object) (object.Object, error) {
				if len(args) < 2 || len(args) > 3 {
					return nil, fmt.Errorf("regexp.replace: expected 2-3 arguments, got %d", len(args))
				}
				strValue, err := object.AsString(args[0])
				if err != nil {
					return nil, err
				}
				replaceValue, err := object.AsString(args[1])
				if err != nil {
					return nil, err
				}

				count := 0 // 0 means replace all
				if len(args) == 3 {
					c, err := object.AsInt(args[2])
					if err != nil {
						return nil, err
					}
					count = int(c)
				}

				if count == 0 {
					return object.NewString(r.value.ReplaceAllString(strValue, replaceValue)), nil
				}

				// Replace up to count matches
				result := strValue
				for i := 0; i < count; i++ {
					loc := r.value.FindStringIndex(result)
					if loc == nil {
						break
					}
					expanded := r.value.ReplaceAllString(result[loc[0]:loc[1]], replaceValue)
					result = result[:loc[0]] + expanded + result[loc[1]:]
				}
				return object.NewString(result), nil
			},
		), true

	// Replace all (convenience method, same as replace with count=0)
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

	// Split
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

	// Introspection
	case "pattern":
		return object.NewString(r.value.String()), true

	case "num_groups":
		return object.NewInt(int64(r.value.NumSubexp())), true
	}
	return nil, false
}

func NewRegexp(value *regexp.Regexp) *Regexp {
	return &Regexp{value: value}
}
