package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

func TestBytesType(t *testing.T) {
	b := NewBytes([]byte("hello"))
	assert.Equal(t, b.Type(), BYTES)
}

func TestBytesInspect(t *testing.T) {
	b := NewBytes([]byte("hello"))
	assert.Equal(t, b.Inspect(), `bytes("hello")`)
}

func TestBytesValue(t *testing.T) {
	data := []byte("hello")
	b := NewBytes(data)
	assert.Equal(t, b.Value(), data)
}

func TestBytesInterface(t *testing.T) {
	data := []byte("hello")
	b := NewBytes(data)
	assert.Equal(t, b.Interface(), data)
}

func TestBytesString(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3})
	// String shows byte values
	s := b.String()
	assert.True(t, len(s) > 0)
}

func TestBytesCompare(t *testing.T) {
	b1 := NewBytes([]byte("aaa"))
	b2 := NewBytes([]byte("bbb"))
	b3 := NewBytes([]byte("aaa"))

	// Less than
	cmp, err := b1.Compare(b2)
	assert.Nil(t, err)
	assert.True(t, cmp < 0)

	// Greater than
	cmp, err = b2.Compare(b1)
	assert.Nil(t, err)
	assert.True(t, cmp > 0)

	// Equal
	cmp, err = b1.Compare(b3)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)

	// Compare with string
	cmp, err = b1.Compare(NewString("aaa"))
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)

	// Incompatible type
	_, err = b1.Compare(NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesEquals(t *testing.T) {
	b1 := NewBytes([]byte("hello"))
	b2 := NewBytes([]byte("hello"))
	b3 := NewBytes([]byte("world"))

	// Equal bytes
	assert.True(t, b1.Equals(b2))

	// Not equal bytes
	assert.False(t, b1.Equals(b3))

	// Equal to string
	assert.True(t, b1.Equals(NewString("hello")))

	// Not equal to string
	assert.False(t, b1.Equals(NewString("world")))

	// Different type
	assert.False(t, b1.Equals(NewInt(1)))
}

func TestBytesIsTruthy(t *testing.T) {
	// Empty bytes is falsy
	assert.False(t, NewBytes([]byte{}).IsTruthy())

	// Non-empty bytes is truthy
	assert.True(t, NewBytes([]byte{1}).IsTruthy())
}

func TestBytesRunOperation(t *testing.T) {
	b1 := NewBytes([]byte("hello"))
	b2 := NewBytes([]byte("world"))

	// Add bytes
	result, err := b1.RunOperation(op.Add, b2)
	assert.Nil(t, err)
	resultBytes := result.(*Bytes)
	assert.Equal(t, string(resultBytes.Value()), "helloworld")

	// Add string
	result, err = b1.RunOperation(op.Add, NewString("!"))
	assert.Nil(t, err)
	resultBytes = result.(*Bytes)
	assert.Equal(t, string(resultBytes.Value()), "hello!")

	// Unsupported operation with bytes
	_, err = b1.RunOperation(op.Subtract, b2)
	assert.NotNil(t, err)

	// Unsupported operation with string
	_, err = b1.RunOperation(op.Subtract, NewString("!"))
	assert.NotNil(t, err)

	// Unsupported type
	_, err = b1.RunOperation(op.Add, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetItem(t *testing.T) {
	b := NewBytes([]byte{10, 20, 30})

	// Valid index
	val, err := b.GetItem(NewInt(0))
	assert.Nil(t, err)
	assert.Equal(t, val.(*Byte).value, byte(10))

	// Negative index (from end)
	val, err = b.GetItem(NewInt(-1))
	assert.Nil(t, err)
	assert.Equal(t, val.(*Byte).value, byte(30))

	// Out of range
	_, err = b.GetItem(NewInt(10))
	assert.NotNil(t, err)

	// Wrong type
	_, err = b.GetItem(NewString("test"))
	assert.NotNil(t, err)
}

func TestBytesGetSlice(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3, 4, 5})

	// Valid slice
	result, err := b.GetSlice(Slice{Start: NewInt(1), Stop: NewInt(3)})
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bytes).Value(), []byte{2, 3})

	// Nil start
	result, err = b.GetSlice(Slice{Start: nil, Stop: NewInt(2)})
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bytes).Value(), []byte{1, 2})

	// Nil stop
	result, err = b.GetSlice(Slice{Start: NewInt(3), Stop: nil})
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bytes).Value(), []byte{4, 5})
}

func TestBytesSetItem(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3})

	// Valid set
	err := b.SetItem(NewInt(1), NewBytes([]byte{99}))
	assert.Nil(t, err)
	assert.Equal(t, b.Value()[1], byte(99))

	// Wrong index type
	err = b.SetItem(NewString("test"), NewBytes([]byte{1}))
	assert.NotNil(t, err)

	// Out of range
	err = b.SetItem(NewInt(10), NewBytes([]byte{1}))
	assert.NotNil(t, err)

	// Wrong value type
	err = b.SetItem(NewInt(0), NewInt(1))
	assert.NotNil(t, err)

	// Value must be single byte
	err = b.SetItem(NewInt(0), NewBytes([]byte{1, 2}))
	assert.NotNil(t, err)
}

func TestBytesDelItem(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3})
	err := b.DelItem(NewInt(0))
	assert.NotNil(t, err) // Cannot delete from bytes
}

func TestBytesContains(t *testing.T) {
	b := NewBytes([]byte("hello world"))

	// Contains
	assert.Equal(t, b.Contains(NewBytes([]byte("world"))), True)
	assert.Equal(t, b.Contains(NewString("world")), True)

	// Does not contain
	assert.Equal(t, b.Contains(NewBytes([]byte("foo"))), False)

	// Invalid type returns false
	assert.Equal(t, b.Contains(NewInt(1)), False)
}

func TestBytesLen(t *testing.T) {
	b := NewBytes([]byte("hello"))
	assert.Equal(t, b.Len().Value(), int64(5))
}

func TestBytesEnumerate(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{10, 20, 30})

	var indices []int64
	var values []byte
	b.Enumerate(ctx, func(key, value Object) bool {
		indices = append(indices, key.(*Int).Value())
		values = append(values, value.(*Byte).value)
		return true
	})

	assert.Equal(t, indices, []int64{0, 1, 2})
	assert.Equal(t, values, []byte{10, 20, 30})
}

func TestBytesEnumerateEarlyStop(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1, 2, 3, 4, 5})

	count := 0
	b.Enumerate(ctx, func(key, value Object) bool {
		count++
		return count < 2
	})

	assert.Equal(t, count, 2)
}

func TestBytesClone(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3})
	clone := b.Clone()

	assert.Equal(t, clone.Value(), b.Value())

	// Clone is independent
	clone.Value()[0] = 99
	assert.Equal(t, b.Value()[0], byte(1))
}

func TestBytesReversed(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3})
	reversed := b.Reversed()
	assert.Equal(t, reversed.Value(), []byte{3, 2, 1})
}

func TestBytesIntegers(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3})
	ints := b.Integers()

	assert.Len(t, ints, 3)
	assert.Equal(t, ints[0].(*Int).Value(), int64(1))
	assert.Equal(t, ints[1].(*Int).Value(), int64(2))
	assert.Equal(t, ints[2].(*Int).Value(), int64(3))
}

func TestBytesGetAttrClone(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1, 2, 3})

	clone, ok := b.GetAttr("clone")
	assert.True(t, ok)

	result, err := clone.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bytes).Value(), []byte{1, 2, 3})
}

func TestBytesGetAttrCloneError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1, 2, 3})
	clone, _ := b.GetAttr("clone")
	_, err := clone.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrEquals(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	equals, ok := b.GetAttr("equals")
	assert.True(t, ok)

	result, err := equals.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Nil(t, err)
	assert.Equal(t, result, True)

	result, err = equals.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Nil(t, err)
	assert.Equal(t, result, False)
}

func TestBytesGetAttrEqualsError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	equals, _ := b.GetAttr("equals")
	_, err := equals.(*Builtin).Call(ctx)
	assert.NotNil(t, err)
}

func TestBytesGetAttrContains(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	contains, ok := b.GetAttr("contains")
	assert.True(t, ok)

	result, err := contains.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Nil(t, err)
	assert.Equal(t, result, True)
}

func TestBytesGetAttrContainsError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	contains, _ := b.GetAttr("contains")
	_, err := contains.(*Builtin).Call(ctx)
	assert.NotNil(t, err)
}

func TestBytesGetAttrContainsAny(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	containsAny, ok := b.GetAttr("contains_any")
	assert.True(t, ok)

	result, err := containsAny.(*Builtin).Call(ctx, NewString("aeiou"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, true)

	result, err = containsAny.(*Builtin).Call(ctx, NewString("xyz"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrContainsAnyError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	containsAny, _ := b.GetAttr("contains_any")

	// Wrong arg count
	_, err := containsAny.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = containsAny.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrContainsRune(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	containsRune, ok := b.GetAttr("contains_rune")
	assert.True(t, ok)

	result, err := containsRune.(*Builtin).Call(ctx, NewString("e"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, true)

	result, err = containsRune.(*Builtin).Call(ctx, NewString("z"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrContainsRuneError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	containsRune, _ := b.GetAttr("contains_rune")

	// Wrong arg count
	_, err := containsRune.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = containsRune.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)

	// Too many characters
	_, err = containsRune.(*Builtin).Call(ctx, NewString("abc"))
	assert.NotNil(t, err)
}

func TestBytesGetAttrCount(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello hello"))

	count, ok := b.GetAttr("count")
	assert.True(t, ok)

	result, err := count.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(2))
}

func TestBytesGetAttrCountError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	count, _ := b.GetAttr("count")

	// Wrong arg count
	_, err := count.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = count.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrHasPrefix(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	hasPrefix, ok := b.GetAttr("has_prefix")
	assert.True(t, ok)

	result, err := hasPrefix.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, true)

	result, err = hasPrefix.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrHasPrefixError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	hasPrefix, _ := b.GetAttr("has_prefix")

	_, err := hasPrefix.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = hasPrefix.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrHasSuffix(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	hasSuffix, ok := b.GetAttr("has_suffix")
	assert.True(t, ok)

	result, err := hasSuffix.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, true)

	result, err = hasSuffix.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrHasSuffixError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	hasSuffix, _ := b.GetAttr("has_suffix")

	_, err := hasSuffix.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = hasSuffix.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrIndex(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	index, ok := b.GetAttr("index")
	assert.True(t, ok)

	result, err := index.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(6))

	result, err = index.(*Builtin).Call(ctx, NewBytes([]byte("foo")))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	index, _ := b.GetAttr("index")

	_, err := index.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = index.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrIndexAny(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	indexAny, ok := b.GetAttr("index_any")
	assert.True(t, ok)

	result, err := indexAny.(*Builtin).Call(ctx, NewString("aeiou"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(1)) // 'e'

	result, err = indexAny.(*Builtin).Call(ctx, NewString("xyz"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexAnyError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	indexAny, _ := b.GetAttr("index_any")

	_, err := indexAny.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = indexAny.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrIndexByte(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{10, 20, 30})

	indexByte, ok := b.GetAttr("index_byte")
	assert.True(t, ok)

	result, err := indexByte.(*Builtin).Call(ctx, NewBytes([]byte{20}))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(1))

	result, err = indexByte.(*Builtin).Call(ctx, NewBytes([]byte{99}))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexByteError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	indexByte, _ := b.GetAttr("index_byte")

	// Wrong arg count
	_, err := indexByte.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = indexByte.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)

	// Must be single byte
	_, err = indexByte.(*Builtin).Call(ctx, NewBytes([]byte{1, 2}))
	assert.NotNil(t, err)
}

func TestBytesGetAttrIndexRune(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	indexRune, ok := b.GetAttr("index_rune")
	assert.True(t, ok)

	result, err := indexRune.(*Builtin).Call(ctx, NewString("l"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(2))

	result, err = indexRune.(*Builtin).Call(ctx, NewString("z"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexRuneError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	indexRune, _ := b.GetAttr("index_rune")

	// Wrong arg count
	_, err := indexRune.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = indexRune.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)

	// Must be single character
	_, err = indexRune.(*Builtin).Call(ctx, NewString("abc"))
	assert.NotNil(t, err)
}

func TestBytesGetAttrRepeat(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("ab"))

	repeat, ok := b.GetAttr("repeat")
	assert.True(t, ok)

	result, err := repeat.(*Builtin).Call(ctx, NewInt(3))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bytes).Value(), []byte("ababab"))
}

func TestBytesGetAttrRepeatError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	repeat, _ := b.GetAttr("repeat")

	_, err := repeat.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = repeat.(*Builtin).Call(ctx, NewString("test"))
	assert.NotNil(t, err)
}

func TestBytesRepeatNegativeCount(t *testing.T) {
	b := NewBytes([]byte("ab"))

	// Negative count should return an error
	_, err := b.Repeat(NewInt(-1))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "negative repeat count")

	// Zero count is valid
	result, err := b.Repeat(NewInt(0))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bytes).Value(), []byte{})
}

func TestBytesGetAttrReplace(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello hello hello"))

	replace, ok := b.GetAttr("replace")
	assert.True(t, ok)

	// Replace with limit
	result, err := replace.(*Builtin).Call(ctx, NewBytes([]byte("hello")), NewBytes([]byte("hi")), NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, string(result.(*Bytes).Value()), "hi hi hello")
}

func TestBytesGetAttrReplaceError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	replace, _ := b.GetAttr("replace")

	// Wrong arg count
	_, err := replace.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewBytes([]byte{2}))
	assert.NotNil(t, err)

	// Wrong type for old
	_, err = replace.(*Builtin).Call(ctx, NewInt(1), NewBytes([]byte{2}), NewInt(1))
	assert.NotNil(t, err)

	// Wrong type for new
	_, err = replace.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewInt(1), NewInt(1))
	assert.NotNil(t, err)

	// Wrong type for count
	_, err = replace.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewBytes([]byte{2}), NewString("x"))
	assert.NotNil(t, err)
}

func TestBytesGetAttrReplaceAll(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello hello hello"))

	replaceAll, ok := b.GetAttr("replace_all")
	assert.True(t, ok)

	result, err := replaceAll.(*Builtin).Call(ctx, NewBytes([]byte("hello")), NewBytes([]byte("hi")))
	assert.Nil(t, err)
	assert.Equal(t, string(result.(*Bytes).Value()), "hi hi hi")
}

func TestBytesGetAttrReplaceAllError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	replaceAll, _ := b.GetAttr("replace_all")

	// Wrong arg count
	_, err := replaceAll.(*Builtin).Call(ctx, NewBytes([]byte{1}))
	assert.NotNil(t, err)

	// Wrong type for old
	_, err = replaceAll.(*Builtin).Call(ctx, NewInt(1), NewBytes([]byte{2}))
	assert.NotNil(t, err)

	// Wrong type for new
	_, err = replaceAll.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewInt(1))
	assert.NotNil(t, err)
}

func TestBytesGetAttrInvalid(t *testing.T) {
	b := NewBytes([]byte{1})
	_, ok := b.GetAttr("invalid_method")
	assert.False(t, ok)
}

func TestBytesMarshalJSON(t *testing.T) {
	b := NewBytes([]byte("hello"))
	data, err := b.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, string(data), `"hello"`)
}

func TestNewBytes(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3})
	assert.Equal(t, b.Value(), []byte{1, 2, 3})
}
