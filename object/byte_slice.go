package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/op"
)

type ByteSlice struct {
	*base
	value []byte
}

func (b *ByteSlice) Inspect() string {
	return fmt.Sprintf("byte_slice(%q)", b.value)
}

func (b *ByteSlice) Type() Type {
	return BYTE_SLICE
}

func (b *ByteSlice) Value() []byte {
	return b.value
}

func (b *ByteSlice) HashKey() HashKey {
	return HashKey{Type: b.Type(), StrValue: string(b.value)}
}

func (b *ByteSlice) GetAttr(name string) (Object, bool) {
	switch name {
	case "clone":
		return &Builtin{
			name: "byte_slice.clone",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("byte_slice.clone", 0, len(args))
				}
				return b.Clone()
			},
		}, true
	case "equals":
		return &Builtin{
			name: "byte_slice.equals",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.equals", 1, len(args))
				}
				return b.Equals(args[0])
			},
		}, true
	case "contains":
		return &Builtin{
			name: "byte_slice.contains",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.contains", 1, len(args))
				}
				return b.Contains(args[0])
			},
		}, true
	case "contains_any":
		return &Builtin{
			name: "byte_slice.contains_any",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.contains_any", 1, len(args))
				}
				return b.ContainsAny(args[0])
			},
		}, true
	case "contains_rune":
		return &Builtin{
			name: "byte_slice.contains_rune",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.contains_rune", 1, len(args))
				}
				return b.ContainsRune(args[0])
			},
		}, true
	case "count":
		return &Builtin{
			name: "byte_slice.count",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.count", 1, len(args))
				}
				return b.Count(args[0])
			},
		}, true
	case "has_prefix":
		return &Builtin{
			name: "byte_slice.has_prefix",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.has_prefix", 1, len(args))
				}
				return b.HasPrefix(args[0])
			},
		}, true
	case "has_suffix":
		return &Builtin{
			name: "byte_slice.has_suffix",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.has_suffix", 1, len(args))
				}
				return b.HasSuffix(args[0])
			},
		}, true
	case "index":
		return &Builtin{
			name: "byte_slice.index",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.index", 1, len(args))
				}
				return b.Index(args[0])
			},
		}, true
	case "index_any":
		return &Builtin{
			name: "byte_slice.index_any",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.index_any", 1, len(args))
				}
				return b.IndexAny(args[0])
			},
		}, true
	case "index_byte":
		return &Builtin{
			name: "byte_slice.index_byte",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.index_byte", 1, len(args))
				}
				return b.IndexByte(args[0])
			},
		}, true
	case "index_rune":
		return &Builtin{
			name: "byte_slice.index_rune",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.index_rune", 1, len(args))
				}
				return b.IndexRune(args[0])
			},
		}, true
	case "repeat":
		return &Builtin{
			name: "byte_slice.repeat",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("byte_slice.repeat", 1, len(args))
				}
				return b.Repeat(args[0])
			},
		}, true
	case "replace":
		return &Builtin{
			name: "byte_slice.replace",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 3 {
					return NewArgsError("byte_slice.replace", 3, len(args))
				}
				return b.Replace(args[0], args[1], args[2])
			},
		}, true
	case "replace_all":
		return &Builtin{
			name: "byte_slice.replace_all",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 2 {
					return NewArgsError("byte_slice.replace_all", 2, len(args))
				}
				return b.ReplaceAll(args[0], args[1])
			},
		}, true
	}
	return nil, false
}

func (b *ByteSlice) Interface() interface{} {
	return b.value
}

func (b *ByteSlice) String() string {
	return fmt.Sprintf("byte_slice(%v)", b.value)
}

func (b *ByteSlice) Compare(other Object) (int, error) {
	switch other := other.(type) {
	case *ByteSlice:
		return bytes.Compare(b.value, other.value), nil
	case *String:
		return bytes.Compare(b.value, []byte(other.value)), nil
	default:
		return 0, errz.TypeErrorf("type error: unable to compare byte_slice and %s", other.Type())
	}
}

func (b *ByteSlice) Equals(other Object) Object {
	switch other := other.(type) {
	case *ByteSlice:
		cmp := bytes.Compare(b.value, other.value)
		if cmp == 0 {
			return True
		}
		return False
	case *String:
		cmp := bytes.Compare(b.value, []byte(other.value))
		if cmp == 0 {
			return True
		}
		return False
	}
	return False
}

func (b *ByteSlice) IsTruthy() bool {
	return len(b.value) > 0
}

func (b *ByteSlice) RunOperation(opType op.BinaryOpType, right Object) Object {
	switch right := right.(type) {
	case *ByteSlice:
		return b.runOperationBytes(opType, right)
	case *String:
		return b.runOperationString(opType, right)
	default:
		return TypeErrorf("type error: unsupported operation for byte_slice: %v on type %s", opType, right.Type())
	}
}

func (b *ByteSlice) runOperationBytes(opType op.BinaryOpType, right *ByteSlice) Object {
	switch opType {
	case op.Add:
		result := make([]byte, len(b.value)+len(right.value))
		copy(result, b.value)
		copy(result[len(b.value):], right.value)
		return NewByteSlice(result)
	default:
		return TypeErrorf("type error: unsupported operation for byte_slice: %v on type %s", opType, right.Type())
	}
}

func (b *ByteSlice) runOperationString(opType op.BinaryOpType, right *String) Object {
	switch opType {
	case op.Add:
		rightBytes := []byte(right.value)
		result := make([]byte, len(b.value)+len(rightBytes))
		copy(result, b.value)
		copy(result[len(b.value):], rightBytes)
		return NewByteSlice(result)
	default:
		return TypeErrorf("type error: unsupported operation for byte_slice: %v on type %s", opType, right.Type())
	}
}

func (b *ByteSlice) GetItem(key Object) (Object, *Error) {
	indexObj, ok := key.(*Int)
	if !ok {
		return nil, TypeErrorf("type error: byte_slice index must be an int (got %s)", key.Type())
	}
	index, err := ResolveIndex(indexObj.value, int64(len(b.value)))
	if err != nil {
		return nil, NewError(err)
	}
	return NewByte(b.value[index]), nil
}

func (b *ByteSlice) GetSlice(slice Slice) (Object, *Error) {
	start, stop, err := ResolveIntSlice(slice, int64(len(b.value)))
	if err != nil {
		return nil, NewError(err)
	}
	return NewByteSlice(b.value[start:stop]), nil
}

func (b *ByteSlice) SetItem(key, value Object) *Error {
	indexObj, ok := key.(*Int)
	if !ok {
		return TypeErrorf("type error: index must be an int (got %s)", key.Type())
	}
	index, err := ResolveIndex(indexObj.value, int64(len(b.value)))
	if err != nil {
		return NewError(err)
	}
	data, convErr := AsBytes(value)
	if convErr != nil {
		return convErr
	}
	if len(data) != 1 {
		return Errorf("value error: value must be a single byte (got %d)", len(data))
	}
	b.value[index] = data[0]
	return nil
}

func (b *ByteSlice) DelItem(key Object) *Error {
	return TypeErrorf("type error: cannot delete from byte_slice")
}

func (b *ByteSlice) Contains(obj Object) *Bool {
	data, err := AsBytes(obj)
	if err != nil {
		return False
	}
	return NewBool(bytes.Contains(b.value, data))
}

func (b *ByteSlice) Len() *Int {
	return NewInt(int64(len(b.value)))
}

func (b *ByteSlice) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for i, v := range b.value {
		if !fn(NewInt(int64(i)), NewByte(v)) {
			return
		}
	}
}

func (b *ByteSlice) Clone() *ByteSlice {
	value := make([]byte, len(b.value))
	copy(value, b.value)
	return NewByteSlice(value)
}

func (b *ByteSlice) Reversed() *ByteSlice {
	value := make([]byte, len(b.value))
	for i := 0; i < len(b.value); i++ {
		value[i] = b.value[len(b.value)-i-1]
	}
	return NewByteSlice(value)
}

func (b *ByteSlice) Integers() []Object {
	result := make([]Object, len(b.value))
	for i, v := range b.value {
		result[i] = NewInt(int64(v))
	}
	return result
}

func (b *ByteSlice) ContainsAny(obj Object) Object {
	chars, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewBool(bytes.ContainsAny(b.value, chars))
}

func (b *ByteSlice) ContainsRune(obj Object) Object {
	s, err := AsString(obj)
	if err != nil {
		return err
	}
	if len(s) != 1 {
		return Errorf("byte_slice.contains_rune: argument must be a single character")
	}
	return NewBool(bytes.ContainsRune(b.value, rune(s[0])))
}

func (b *ByteSlice) Count(obj Object) Object {
	data, err := AsBytes(obj)
	if err != nil {
		return err
	}
	return NewInt(int64(bytes.Count(b.value, data)))
}

func (b *ByteSlice) HasPrefix(obj Object) Object {
	data, err := AsBytes(obj)
	if err != nil {
		return err
	}
	return NewBool(bytes.HasPrefix(b.value, data))
}

func (b *ByteSlice) HasSuffix(obj Object) Object {
	data, err := AsBytes(obj)
	if err != nil {
		return err
	}
	return NewBool(bytes.HasSuffix(b.value, data))
}

func (b *ByteSlice) Index(obj Object) Object {
	data, err := AsBytes(obj)
	if err != nil {
		return err
	}
	return NewInt(int64(bytes.Index(b.value, data)))
}

func (b *ByteSlice) IndexAny(obj Object) Object {
	chars, err := AsString(obj)
	if err != nil {
		return err
	}
	return NewInt(int64(bytes.IndexAny(b.value, chars)))
}

func (b *ByteSlice) IndexByte(obj Object) Object {
	data, err := AsBytes(obj)
	if err != nil {
		return err
	}
	if len(data) != 1 {
		return Errorf("byte_slice.index_byte: argument must be a single byte")
	}
	return NewInt(int64(bytes.IndexByte(b.value, data[0])))
}

func (b *ByteSlice) IndexRune(obj Object) Object {
	s, err := AsString(obj)
	if err != nil {
		return err
	}
	if len(s) != 1 {
		return Errorf("byte_slice.index_rune: argument must be a single character")
	}
	return NewInt(int64(bytes.IndexRune(b.value, rune(s[0]))))
}

func (b *ByteSlice) Repeat(obj Object) Object {
	count, err := AsInt(obj)
	if err != nil {
		return err
	}
	return NewByteSlice(bytes.Repeat(b.value, int(count)))
}

func (b *ByteSlice) Replace(old, new, count Object) Object {
	oldBytes, err := AsBytes(old)
	if err != nil {
		return err
	}
	newBytes, err := AsBytes(new)
	if err != nil {
		return err
	}
	n, err := AsInt(count)
	if err != nil {
		return err
	}
	return NewByteSlice(bytes.Replace(b.value, oldBytes, newBytes, int(n)))
}

func (b *ByteSlice) ReplaceAll(old, new Object) Object {
	oldBytes, err := AsBytes(old)
	if err != nil {
		return err
	}
	newBytes, err := AsBytes(new)
	if err != nil {
		return err
	}
	return NewByteSlice(bytes.ReplaceAll(b.value, oldBytes, newBytes))
}

func (b *ByteSlice) Cost() int {
	return len(b.value)
}

func (b *ByteSlice) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(b.value))
}

func NewByteSlice(value []byte) *ByteSlice {
	return &ByteSlice{value: value}
}
