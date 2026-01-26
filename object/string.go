package object

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/risor-io/risor/op"
)

type String struct {
	*base
	value string
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

func (s *String) HashKey() HashKey {
	return HashKey{Type: s.Type(), StrValue: s.value}
}

func (s *String) GetAttr(name string) (Object, bool) {
	switch name {
	case "contains":
		return &Builtin{
			name: "string.contains",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.contains", 1, len(args))
				}
				return s.Contains(args[0])
			},
		}, true
	case "has_prefix":
		return &Builtin{
			name: "string.has_prefix",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.has_prefix", 1, len(args))
				}
				return s.HasPrefix(args[0])
			},
		}, true
	case "has_suffix":
		return &Builtin{
			name: "string.has_suffix",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.has_suffix", 1, len(args))
				}
				return s.HasSuffix(args[0])
			},
		}, true
	case "count":
		return &Builtin{
			name: "string.count",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.count", 1, len(args))
				}
				return s.Count(args[0])
			},
		}, true
	case "join":
		return &Builtin{
			name: "string.join",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.join", 1, len(args))
				}
				return s.Join(args[0])
			},
		}, true
	case "split":
		return &Builtin{
			name: "string.split",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.split", 1, len(args))
				}
				return s.Split(args[0])
			},
		}, true
	case "fields":
		return &Builtin{
			name: "string.fields",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("string.fields", 0, len(args))
				}
				return s.Fields()
			},
		}, true
	case "index":
		return &Builtin{
			name: "string.index",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.index", 1, len(args))
				}
				return s.Index(args[0])
			},
		}, true
	case "last_index":
		return &Builtin{
			name: "string.last_index",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.last_index", 1, len(args))
				}
				return s.LastIndex(args[0])
			},
		}, true
	case "replace_all":
		return &Builtin{
			name: "string.replace_all",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 2 {
					return NewArgsError("string.replace_all", 2, len(args))
				}
				return s.ReplaceAll(args[0], args[1])
			},
		}, true
	case "to_lower":
		return &Builtin{
			name: "string.to_lower",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("string.to_lower", 0, len(args))
				}
				return s.ToLower()
			},
		}, true
	case "to_upper":
		return &Builtin{
			name: "string.to_upper",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("string.to_upper", 0, len(args))
				}
				return s.ToUpper()
			},
		}, true
	case "trim":
		return &Builtin{
			name: "string.trim",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.trim", 1, len(args))
				}
				return s.Trim(args[0])
			},
		}, true
	case "trim_prefix":
		return &Builtin{
			name: "string.trim_prefix",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.trim_prefix", 1, len(args))
				}
				return s.TrimPrefix(args[0])
			},
		}, true
	case "trim_space":
		return &Builtin{
			name: "string.trim_space",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("string.trim_space", 0, len(args))
				}
				return s.TrimSpace()
			},
		}, true
	case "trim_suffix":
		return &Builtin{
			name: "string.trim_suffix",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.trim_suffix", 1, len(args))
				}
				return s.TrimSuffix(args[0])
			},
		}, true
	case "compare":
		return &Builtin{
			name: "string.compare",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.compare", 1, len(args))
				}
				result, err := s.Compare(args[0])
				if err != nil {
					return NewError(err)
				}
				return NewInt(int64(result))
			},
		}, true
	case "repeat":
		return &Builtin{
			name: "string.repeat",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("string.repeat", 1, len(args))
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
		return 0, TypeErrorf("type error: unable to compare string and %s", other.Type())
	}
	if s.value == otherStr.value {
		return 0, nil
	}
	if s.value > otherStr.value {
		return 1, nil
	}
	return -1, nil
}

func (s *String) Equals(other Object) Object {
	if other.Type() == STRING && s.value == other.(*String).value {
		return True
	}
	return False
}

func (s *String) IsTruthy() bool {
	return s.value != ""
}

func (s *String) RunOperation(opType op.BinaryOpType, right Object) Object {
	switch right := right.(type) {
	case *String:
		return s.runOperationString(opType, right)
	default:
		return TypeErrorf("type error: unsupported operation for string: %v on type %s", opType, right.Type())
	}
}

func (s *String) runOperationString(opType op.BinaryOpType, right *String) Object {
	switch opType {
	case op.Add:
		return NewString(s.value + right.value)
	default:
		return TypeErrorf("type error: unsupported operation for string: %v on type %s", opType, right.Type())
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
		return nil, TypeErrorf("type error: string index must be an int (got %s)", key.Type())
	}
	runes := []rune(s.value)
	index, err := ResolveIndex(indexObj.value, int64(len(runes)))
	if err != nil {
		return nil, Errorf(err.Error())
	}
	return NewString(string(runes[index])), nil
}

func (s *String) GetSlice(slice Slice) (Object, *Error) {
	runes := []rune(s.value)
	start, stop, err := ResolveIntSlice(slice, int64(len(runes)))
	if err != nil {
		return nil, Errorf(err.Error())
	}
	resultRunes := runes[start:stop]
	return NewString(string(resultRunes)), nil
}

func (s *String) SetItem(key, value Object) *Error {
	return TypeErrorf("type error: set item is unsupported for string")
}

func (s *String) DelItem(key Object) *Error {
	return TypeErrorf("type error: del item is unsupported for string")
}

func (s *String) Contains(obj Object) *Bool {
	other, err := AsString(obj)
	if err != nil {
		return False
	}
	return NewBool(strings.Contains(s.value, other))
}

func (s *String) HasPrefix(obj Object) Object {
	prefix, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewBool(strings.HasPrefix(s.value, prefix))
}

func (s *String) HasSuffix(obj Object) Object {
	suffix, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewBool(strings.HasSuffix(s.value, suffix))
}

func (s *String) Count(obj Object) Object {
	substr, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewInt(int64(strings.Count(s.value, substr)))
}

func (s *String) Join(obj Object) Object {
	ls, err := AsList(obj)
	if err != nil {
		return err
	}
	var strs []string
	for _, item := range ls.Value() {
		itemStr, err := AsString(item)
		if err != nil {
			return err
		}
		strs = append(strs, itemStr)
	}
	return NewString(strings.Join(strs, s.value))
}

func (s *String) Split(obj Object) Object {
	sep, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewStringList(strings.Split(s.value, sep))
}

func (s *String) Fields() Object {
	return NewStringList(strings.Fields(s.value))
}

func (s *String) Index(obj Object) Object {
	substr, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewInt(int64(strings.Index(s.value, substr)))
}

func (s *String) LastIndex(obj Object) Object {
	substr, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewInt(int64(strings.LastIndex(s.value, substr)))
}

func (s *String) ReplaceAll(old, new Object) Object {
	oldStr, err := AsString(old)
	if err != nil {
		return err
	}
	newStr, err := AsString(new)
	if err != nil {
		return err
	}
	return NewString(strings.ReplaceAll(s.value, oldStr, newStr))
}

func (s *String) ToLower() Object {
	return NewString(strings.ToLower(s.value))
}

func (s *String) ToUpper() Object {
	return NewString(strings.ToUpper(s.value))
}

func (s *String) Trim(obj Object) Object {
	chars, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewString(strings.Trim(s.value, chars))
}

func (s *String) TrimPrefix(obj Object) Object {
	prefix, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewString(strings.TrimPrefix(s.value, prefix))
}

func (s *String) TrimSuffix(obj Object) Object {
	suffix, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewString(strings.TrimSuffix(s.value, suffix))
}

func (s *String) TrimSpace() Object {
	return NewString(strings.TrimSpace(s.value))
}

func (s *String) Repeat(obj Object) Object {
	count, err := AsInt(obj)
	if err != nil {
		return err
	}
	if count < 0 {
		return Errorf("value error: negative repeat count")
	}
	return NewString(strings.Repeat(s.value, int(count)))
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

func (s *String) Cost() int {
	return len(s.value)
}

func (s *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

func NewString(s string) *String {
	return &String{value: s}
}
