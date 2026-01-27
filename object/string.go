package object

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/risor-io/risor/op"
)

// stringAttrs defines all attributes available on string objects.
// This is the single source of truth for string methods.
var stringAttrs = []AttrSpec{
	{Name: "compare", Doc: "Compare to another string (-1, 0, or 1)", Args: []string{"other"}, Returns: "int"},
	{Name: "contains", Doc: "Check if substring exists", Args: []string{"substr"}, Returns: "bool"},
	{Name: "count", Doc: "Count occurrences of substring", Args: []string{"substr"}, Returns: "int"},
	{Name: "fields", Doc: "Split on whitespace", Args: nil, Returns: "list"},
	{Name: "has_prefix", Doc: "Check if string starts with prefix", Args: []string{"prefix"}, Returns: "bool"},
	{Name: "has_suffix", Doc: "Check if string ends with suffix", Args: []string{"suffix"}, Returns: "bool"},
	{Name: "index", Doc: "Find first index of substring (-1 if not found)", Args: []string{"substr"}, Returns: "int"},
	{Name: "join", Doc: "Join list elements with this string as separator", Args: []string{"items"}, Returns: "string"},
	{Name: "last_index", Doc: "Find last index of substring (-1 if not found)", Args: []string{"substr"}, Returns: "int"},
	{Name: "repeat", Doc: "Repeat string n times", Args: []string{"count"}, Returns: "string"},
	{Name: "replace_all", Doc: "Replace all occurrences", Args: []string{"old", "new"}, Returns: "string"},
	{Name: "split", Doc: "Split by separator", Args: []string{"sep"}, Returns: "list"},
	{Name: "to_lower", Doc: "Convert to lowercase", Args: nil, Returns: "string"},
	{Name: "to_upper", Doc: "Convert to uppercase", Args: nil, Returns: "string"},
	{Name: "trim", Doc: "Trim characters from both ends", Args: []string{"chars"}, Returns: "string"},
	{Name: "trim_prefix", Doc: "Remove prefix if present", Args: []string{"prefix"}, Returns: "string"},
	{Name: "trim_space", Doc: "Trim whitespace from both ends", Args: nil, Returns: "string"},
	{Name: "trim_suffix", Doc: "Remove suffix if present", Args: []string{"suffix"}, Returns: "string"},
}

type String struct {
	value string
}

// Attrs returns the attribute specifications for string objects.
func (s *String) Attrs() []AttrSpec {
	return stringAttrs
}

func (s *String) SetAttr(name string, value Object) error {
	return TypeErrorf("string has no attribute %q", name)
}

func (s *String) Type() Type {
	return STRING
}

func (s *String) Value() string {
	return s.value
}

func (s *String) Inspect() string {
	sLen := len(s.value)
	if sLen >= 2 {
		if s.value[0] == '"' && s.value[sLen-1] == '"' {
			if strings.Count(s.value, "\"") == 2 {
				return fmt.Sprintf("'%s'", s.value)
			}
		}
	}
	return fmt.Sprintf("%q", s.value)
}

func (s *String) String() string {
	return s.value
}

func (s *String) GetAttr(name string) (Object, bool) {
	switch name {
	case "contains":
		return &Builtin{
			name: "string.contains",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.contains: expected 1 argument, got %d", len(args))
				}
				return s.Contains(args[0]), nil
			},
		}, true
	case "has_prefix":
		return &Builtin{
			name: "string.has_prefix",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.has_prefix: expected 1 argument, got %d", len(args))
				}
				return s.HasPrefix(args[0])
			},
		}, true
	case "has_suffix":
		return &Builtin{
			name: "string.has_suffix",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.has_suffix: expected 1 argument, got %d", len(args))
				}
				return s.HasSuffix(args[0])
			},
		}, true
	case "count":
		return &Builtin{
			name: "string.count",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.count: expected 1 argument, got %d", len(args))
				}
				return s.Count(args[0])
			},
		}, true
	case "join":
		return &Builtin{
			name: "string.join",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.join: expected 1 argument, got %d", len(args))
				}
				return s.Join(args[0])
			},
		}, true
	case "split":
		return &Builtin{
			name: "string.split",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.split: expected 1 argument, got %d", len(args))
				}
				return s.Split(args[0])
			},
		}, true
	case "fields":
		return &Builtin{
			name: "string.fields",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("string.fields: expected 0 arguments, got %d", len(args))
				}
				return s.Fields(), nil
			},
		}, true
	case "index":
		return &Builtin{
			name: "string.index",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.index: expected 1 argument, got %d", len(args))
				}
				return s.Index(args[0])
			},
		}, true
	case "last_index":
		return &Builtin{
			name: "string.last_index",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.last_index: expected 1 argument, got %d", len(args))
				}
				return s.LastIndex(args[0])
			},
		}, true
	case "replace_all":
		return &Builtin{
			name: "string.replace_all",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("string.replace_all: expected 2 arguments, got %d", len(args))
				}
				return s.ReplaceAll(args[0], args[1])
			},
		}, true
	case "to_lower":
		return &Builtin{
			name: "string.to_lower",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("string.to_lower: expected 0 arguments, got %d", len(args))
				}
				return s.ToLower(), nil
			},
		}, true
	case "to_upper":
		return &Builtin{
			name: "string.to_upper",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("string.to_upper: expected 0 arguments, got %d", len(args))
				}
				return s.ToUpper(), nil
			},
		}, true
	case "trim":
		return &Builtin{
			name: "string.trim",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.trim: expected 1 argument, got %d", len(args))
				}
				return s.Trim(args[0])
			},
		}, true
	case "trim_prefix":
		return &Builtin{
			name: "string.trim_prefix",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.trim_prefix: expected 1 argument, got %d", len(args))
				}
				return s.TrimPrefix(args[0])
			},
		}, true
	case "trim_space":
		return &Builtin{
			name: "string.trim_space",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("string.trim_space: expected 0 arguments, got %d", len(args))
				}
				return s.TrimSpace(), nil
			},
		}, true
	case "trim_suffix":
		return &Builtin{
			name: "string.trim_suffix",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.trim_suffix: expected 1 argument, got %d", len(args))
				}
				return s.TrimSuffix(args[0])
			},
		}, true
	case "compare":
		return &Builtin{
			name: "string.compare",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.compare: expected 1 argument, got %d", len(args))
				}
				result, err := s.Compare(args[0])
				if err != nil {
					return nil, err
				}
				return NewInt(int64(result)), nil
			},
		}, true
	case "repeat":
		return &Builtin{
			name: "string.repeat",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("string.repeat: expected 1 argument, got %d", len(args))
				}
				return s.Repeat(args[0])
			},
		}, true
	}
	return nil, false
}

func (s *String) Interface() interface{} {
	return s.value
}

func (s *String) Compare(other Object) (int, error) {
	otherStr, ok := other.(*String)
	if !ok {
		return 0, TypeErrorf("unable to compare string and %s", other.Type())
	}
	if s.value == otherStr.value {
		return 0, nil
	}
	if s.value > otherStr.value {
		return 1, nil
	}
	return -1, nil
}

func (s *String) Equals(other Object) bool {
	otherString, ok := other.(*String)
	if !ok {
		return false
	}
	return s.value == otherString.value
}

func (s *String) IsTruthy() bool {
	return s.value != ""
}

func (s *String) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	switch right := right.(type) {
	case *String:
		return s.runOperationString(opType, right)
	default:
		return nil, newTypeErrorf("unsupported operation for string: %v on type %s", opType, right.Type())
	}
}

func (s *String) runOperationString(opType op.BinaryOpType, right *String) (Object, error) {
	switch opType {
	case op.Add:
		return NewString(s.value + right.value), nil
	default:
		return nil, newTypeErrorf("unsupported operation for string: %v on type %s", opType, right.Type())
	}
}

func (s *String) Reversed() *String {
	runes := []rune(s.value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return NewString(string(runes))
}

func (s *String) GetItem(key Object) (Object, *Error) {
	indexObj, ok := key.(*Int)
	if !ok {
		return nil, TypeErrorf("string index must be an int (got %s)", key.Type())
	}
	runes := []rune(s.value)
	index, err := ResolveIndex(indexObj.value, int64(len(runes)))
	if err != nil {
		return nil, NewError(err)
	}
	return NewString(string(runes[index])), nil
}

func (s *String) GetSlice(slice Slice) (Object, *Error) {
	runes := []rune(s.value)
	start, stop, err := ResolveIntSlice(slice, int64(len(runes)))
	if err != nil {
		return nil, NewError(err)
	}
	resultRunes := runes[start:stop]
	return NewString(string(resultRunes)), nil
}

func (s *String) SetItem(key, value Object) *Error {
	return TypeErrorf("set item is unsupported for string")
}

func (s *String) DelItem(key Object) *Error {
	return TypeErrorf("del item is unsupported for string")
}

func (s *String) Contains(obj Object) *Bool {
	other, err := AsString(obj)
	if err != nil {
		return False
	}
	return NewBool(strings.Contains(s.value, other))
}

func (s *String) HasPrefix(obj Object) (Object, error) {
	prefix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewBool(strings.HasPrefix(s.value, prefix)), nil
}

func (s *String) HasSuffix(obj Object) (Object, error) {
	suffix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewBool(strings.HasSuffix(s.value, suffix)), nil
}

func (s *String) Count(obj Object) (Object, error) {
	substr, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(strings.Count(s.value, substr))), nil
}

func (s *String) Join(obj Object) (Object, error) {
	ls, err := AsList(obj)
	if err != nil {
		return nil, err
	}
	var strs []string
	for _, item := range ls.Value() {
		itemStr, err := AsString(item)
		if err != nil {
			return nil, err
		}
		strs = append(strs, itemStr)
	}
	return NewString(strings.Join(strs, s.value)), nil
}

func (s *String) Split(obj Object) (Object, error) {
	sep, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewStringList(strings.Split(s.value, sep)), nil
}

func (s *String) Fields() Object {
	return NewStringList(strings.Fields(s.value))
}

func (s *String) Index(obj Object) (Object, error) {
	substr, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(strings.Index(s.value, substr))), nil
}

func (s *String) LastIndex(obj Object) (Object, error) {
	substr, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(strings.LastIndex(s.value, substr))), nil
}

func (s *String) ReplaceAll(old, new Object) (Object, error) {
	oldStr, err := AsString(old)
	if err != nil {
		return nil, err
	}
	newStr, err := AsString(new)
	if err != nil {
		return nil, err
	}
	return NewString(strings.ReplaceAll(s.value, oldStr, newStr)), nil
}

func (s *String) ToLower() Object {
	return NewString(strings.ToLower(s.value))
}

func (s *String) ToUpper() Object {
	return NewString(strings.ToUpper(s.value))
}

func (s *String) Trim(obj Object) (Object, error) {
	chars, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewString(strings.Trim(s.value, chars)), nil
}

func (s *String) TrimPrefix(obj Object) (Object, error) {
	prefix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewString(strings.TrimPrefix(s.value, prefix)), nil
}

func (s *String) TrimSuffix(obj Object) (Object, error) {
	suffix, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewString(strings.TrimSuffix(s.value, suffix)), nil
}

func (s *String) TrimSpace() Object {
	return NewString(strings.TrimSpace(s.value))
}

func (s *String) Repeat(obj Object) (Object, error) {
	count, err := AsInt(obj)
	if err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, newValueErrorf("negative repeat count")
	}
	return NewString(strings.Repeat(s.value, int(count))), nil
}

func (s *String) Len() *Int {
	return NewInt(int64(len([]rune(s.value))))
}

func (s *String) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for i, r := range s.value {
		if !fn(NewInt(int64(i)), NewString(string(r))) {
			return
		}
	}
}

func (s *String) Runes() []Object {
	runes := []rune(s.value)
	result := make([]Object, len(runes))
	for i, r := range runes {
		result[i] = NewString(string(r))
	}
	return result
}

func (s *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

func NewString(s string) *String {
	return &String{value: s}
}
