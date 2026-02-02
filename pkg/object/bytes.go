package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

var bytesMethods = NewMethodRegistry[*Bytes]("bytes")

func init() {
	bytesMethods.Define("clone").
		Doc("Create a copy of the bytes").
		Returns("bytes").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.Clone(), nil
		})

	bytesMethods.Define("contains").
		Doc("Check if bytes contains a subsequence").
		Arg("b").
		Returns("bool").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.Contains(args[0]), nil
		})

	bytesMethods.Define("contains_any").
		Doc("Check if bytes contains any of the given characters").
		Arg("chars").
		Returns("bool").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.ContainsAny(args[0])
		})

	bytesMethods.Define("contains_rune").
		Doc("Check if bytes contains a rune").
		Arg("r").
		Returns("bool").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.ContainsRune(args[0])
		})

	bytesMethods.Define("count").
		Doc("Count occurrences of subsequence").
		Arg("b").
		Returns("int").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.Count(args[0])
		})

	bytesMethods.Define("equals").
		Doc("Check equality with another bytes").
		Arg("other").
		Returns("bool").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return NewBool(b.Equals(args[0])), nil
		})

	bytesMethods.Define("has_prefix").
		Doc("Check if bytes starts with prefix").
		Arg("prefix").
		Returns("bool").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.HasPrefix(args[0])
		})

	bytesMethods.Define("has_suffix").
		Doc("Check if bytes ends with suffix").
		Arg("suffix").
		Returns("bool").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.HasSuffix(args[0])
		})

	bytesMethods.Define("index").
		Doc("Find first index of subsequence (-1 if not found)").
		Arg("b").
		Returns("int").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.Index(args[0])
		})

	bytesMethods.Define("index_any").
		Doc("Find first index of any character (-1 if not found)").
		Arg("chars").
		Returns("int").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.IndexAny(args[0])
		})

	bytesMethods.Define("index_byte").
		Doc("Find first index of byte (-1 if not found)").
		Arg("b").
		Returns("int").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.IndexByte(args[0])
		})

	bytesMethods.Define("index_rune").
		Doc("Find first index of rune (-1 if not found)").
		Arg("r").
		Returns("int").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.IndexRune(args[0])
		})

	bytesMethods.Define("repeat").
		Doc("Repeat bytes n times").
		Arg("count").
		Returns("bytes").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.Repeat(args[0])
		})

	bytesMethods.Define("replace").
		Doc("Replace n occurrences").
		Args("old", "new", "n").
		Returns("bytes").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.Replace(args[0], args[1], args[2])
		})

	bytesMethods.Define("replace_all").
		Doc("Replace all occurrences").
		Args("old", "new").
		Returns("bytes").
		Impl(func(b *Bytes, ctx context.Context, args ...Object) (Object, error) {
			return b.ReplaceAll(args[0], args[1])
		})
}

type Bytes struct {
	value []byte
}

func (b *Bytes) Attrs() []AttrSpec {
	return bytesMethods.Specs()
}

func (b *Bytes) GetAttr(name string) (Object, bool) {
	return bytesMethods.GetAttr(b, name)
}

func (b *Bytes) SetAttr(name string, value Object) error {
	return TypeErrorf("bytes has no attribute %q", name)
}

func (b *Bytes) Inspect() string {
	return fmt.Sprintf("bytes(%q)", b.value)
}

func (b *Bytes) Type() Type {
	return BYTES
}

func (b *Bytes) Value() []byte {
	return b.value
}

func (b *Bytes) Interface() interface{} {
	return b.value
}

func (b *Bytes) String() string {
	return fmt.Sprintf("bytes(%v)", b.value)
}

func (b *Bytes) Compare(other Object) (int, error) {
	switch other := other.(type) {
	case *Bytes:
		return bytes.Compare(b.value, other.value), nil
	case *String:
		return bytes.Compare(b.value, []byte(other.value)), nil
	default:
		return 0, TypeErrorf("unable to compare bytes and %s", other.Type())
	}
}

func (b *Bytes) Equals(other Object) bool {
	switch other := other.(type) {
	case *Bytes:
		return bytes.Equal(b.value, other.value)
	case *String:
		return bytes.Equal(b.value, []byte(other.value))
	}
	return false
}

func (b *Bytes) IsTruthy() bool {
	return len(b.value) > 0
}

func (b *Bytes) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	switch right := right.(type) {
	case *Bytes:
		return b.runOperationBytes(opType, right)
	case *String:
		return b.runOperationString(opType, right)
	default:
		return nil, newTypeErrorf("unsupported operation for bytes: %v on type %s", opType, right.Type())
	}
}

func (b *Bytes) runOperationBytes(opType op.BinaryOpType, right *Bytes) (Object, error) {
	switch opType {
	case op.Add:
		result := make([]byte, len(b.value)+len(right.value))
		copy(result, b.value)
		copy(result[len(b.value):], right.value)
		return NewBytes(result), nil
	default:
		return nil, newTypeErrorf("unsupported operation for bytes: %v on type %s", opType, right.Type())
	}
}

func (b *Bytes) runOperationString(opType op.BinaryOpType, right *String) (Object, error) {
	switch opType {
	case op.Add:
		rightBytes := []byte(right.value)
		result := make([]byte, len(b.value)+len(rightBytes))
		copy(result, b.value)
		copy(result[len(b.value):], rightBytes)
		return NewBytes(result), nil
	default:
		return nil, newTypeErrorf("unsupported operation for bytes: %v on type %s", opType, right.Type())
	}
}

func (b *Bytes) GetItem(key Object) (Object, *Error) {
	indexObj, ok := key.(*Int)
	if !ok {
		return nil, TypeErrorf("bytes index must be an int (got %s)", key.Type())
	}
	index, err := ResolveIndex(indexObj.value, int64(len(b.value)))
	if err != nil {
		return nil, NewError(err)
	}
	return NewByte(b.value[index]), nil
}

func (b *Bytes) GetSlice(slice Slice) (Object, *Error) {
	start, stop, err := ResolveIntSlice(slice, int64(len(b.value)))
	if err != nil {
		return nil, NewError(err)
	}
	return NewBytes(b.value[start:stop]), nil
}

func (b *Bytes) SetItem(key, value Object) *Error {
	indexObj, ok := key.(*Int)
	if !ok {
		return TypeErrorf("index must be an int (got %s)", key.Type())
	}
	index, err := ResolveIndex(indexObj.value, int64(len(b.value)))
	if err != nil {
		return NewError(err)
	}
	data, convErr := AsBytes(value)
	if convErr != nil {
		return NewError(convErr)
	}
	if len(data) != 1 {
		return NewError(newValueErrorf("value must be a single byte (got %d)", len(data)))
	}
	b.value[index] = data[0]
	return nil
}

func (b *Bytes) DelItem(key Object) *Error {
	return TypeErrorf("cannot delete from bytes")
}

func (b *Bytes) Contains(obj Object) *Bool {
	data, err := AsBytes(obj)
	if err != nil {
		return False
	}
	return NewBool(bytes.Contains(b.value, data))
}

func (b *Bytes) Len() *Int {
	return NewInt(int64(len(b.value)))
}

func (b *Bytes) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for i, v := range b.value {
		if !fn(NewInt(int64(i)), NewByte(v)) {
			return
		}
	}
}

func (b *Bytes) Clone() *Bytes {
	value := make([]byte, len(b.value))
	copy(value, b.value)
	return NewBytes(value)
}

func (b *Bytes) Reversed() *Bytes {
	value := make([]byte, len(b.value))
	for i := 0; i < len(b.value); i++ {
		value[i] = b.value[len(b.value)-i-1]
	}
	return NewBytes(value)
}

func (b *Bytes) Integers() []Object {
	result := make([]Object, len(b.value))
	for i, v := range b.value {
		result[i] = NewInt(int64(v))
	}
	return result
}

func (b *Bytes) ContainsAny(obj Object) (Object, error) {
	chars, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewBool(bytes.ContainsAny(b.value, chars)), nil
}

func (b *Bytes) ContainsRune(obj Object) (Object, error) {
	s, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	if len(s) != 1 {
		return nil, fmt.Errorf("bytes.contains_rune: argument must be a single character")
	}
	return NewBool(bytes.ContainsRune(b.value, rune(s[0]))), nil
}

func (b *Bytes) Count(obj Object) (Object, error) {
	data, err := AsBytes(obj)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(bytes.Count(b.value, data))), nil
}

func (b *Bytes) HasPrefix(obj Object) (Object, error) {
	data, err := AsBytes(obj)
	if err != nil {
		return nil, err
	}
	return NewBool(bytes.HasPrefix(b.value, data)), nil
}

func (b *Bytes) HasSuffix(obj Object) (Object, error) {
	data, err := AsBytes(obj)
	if err != nil {
		return nil, err
	}
	return NewBool(bytes.HasSuffix(b.value, data)), nil
}

func (b *Bytes) Index(obj Object) (Object, error) {
	data, err := AsBytes(obj)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(bytes.Index(b.value, data))), nil
}

func (b *Bytes) IndexAny(obj Object) (Object, error) {
	chars, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(bytes.IndexAny(b.value, chars))), nil
}

func (b *Bytes) IndexByte(obj Object) (Object, error) {
	data, err := AsBytes(obj)
	if err != nil {
		return nil, err
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("bytes.index_byte: argument must be a single byte")
	}
	return NewInt(int64(bytes.IndexByte(b.value, data[0]))), nil
}

func (b *Bytes) IndexRune(obj Object) (Object, error) {
	s, err := AsString(obj)
	if err != nil {
		return nil, err
	}
	if len(s) != 1 {
		return nil, fmt.Errorf("bytes.index_rune: argument must be a single character")
	}
	return NewInt(int64(bytes.IndexRune(b.value, rune(s[0])))), nil
}

func (b *Bytes) Repeat(obj Object) (Object, error) {
	count, err := AsInt(obj)
	if err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, newValueErrorf("negative repeat count")
	}
	return NewBytes(bytes.Repeat(b.value, int(count))), nil
}

func (b *Bytes) Replace(old, new, count Object) (Object, error) {
	oldBytes, err := AsBytes(old)
	if err != nil {
		return nil, err
	}
	newBytes, err := AsBytes(new)
	if err != nil {
		return nil, err
	}
	n, err := AsInt(count)
	if err != nil {
		return nil, err
	}
	return NewBytes(bytes.Replace(b.value, oldBytes, newBytes, int(n))), nil
}

func (b *Bytes) ReplaceAll(old, new Object) (Object, error) {
	oldBytes, err := AsBytes(old)
	if err != nil {
		return nil, err
	}
	newBytes, err := AsBytes(new)
	if err != nil {
		return nil, err
	}
	return NewBytes(bytes.ReplaceAll(b.value, oldBytes, newBytes)), nil
}

func (b *Bytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(b.value))
}

func NewBytes(value []byte) *Bytes {
	return &Bytes{value: value}
}
