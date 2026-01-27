package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/op"
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

func TestBytesHashKey(t *testing.T) {
	b := NewBytes([]byte("hello"))
	hk := b.HashKey()
	assert.Equal(t, hk.Type, BYTES)
	assert.Equal(t, hk.StrValue, "hello")
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
	assert.Equal(t, b1.Equals(b2), True)

	// Not equal bytes
	assert.Equal(t, b1.Equals(b3), False)

	// Equal to string
	assert.Equal(t, b1.Equals(NewString("hello")), True)

	// Not equal to string
	assert.Equal(t, b1.Equals(NewString("world")), False)

	// Different type
	assert.Equal(t, b1.Equals(NewInt(1)), False)
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
	result := b1.RunOperation(op.Add, b2)
	resultBytes := result.(*Bytes)
	assert.Equal(t, string(resultBytes.Value()), "helloworld")

	// Add string
	result = b1.RunOperation(op.Add, NewString("!"))
	resultBytes = result.(*Bytes)
	assert.Equal(t, string(resultBytes.Value()), "hello!")

	// Unsupported operation with bytes
	result = b1.RunOperation(op.Subtract, b2)
	assert.True(t, IsError(result))

	// Unsupported operation with string
	result = b1.RunOperation(op.Subtract, NewString("!"))
	assert.True(t, IsError(result))

	// Unsupported type
	result = b1.RunOperation(op.Add, NewInt(1))
	assert.True(t, IsError(result))
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

	result := clone.(*Builtin).Call(ctx)
	assert.Equal(t, result.(*Bytes).Value(), []byte{1, 2, 3})
}

func TestBytesGetAttrCloneError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1, 2, 3})
	clone, _ := b.GetAttr("clone")
	result := clone.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrEquals(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	equals, ok := b.GetAttr("equals")
	assert.True(t, ok)

	result := equals.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Equal(t, result, True)

	result = equals.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Equal(t, result, False)
}

func TestBytesGetAttrEqualsError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	equals, _ := b.GetAttr("equals")
	result := equals.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))
}

func TestBytesGetAttrContains(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	contains, ok := b.GetAttr("contains")
	assert.True(t, ok)

	result := contains.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Equal(t, result, True)
}

func TestBytesGetAttrContainsError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	contains, _ := b.GetAttr("contains")
	result := contains.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))
}

func TestBytesGetAttrContainsAny(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	containsAny, ok := b.GetAttr("contains_any")
	assert.True(t, ok)

	result := containsAny.(*Builtin).Call(ctx, NewString("aeiou"))
	assert.Equal(t, result.(*Bool).value, true)

	result = containsAny.(*Builtin).Call(ctx, NewString("xyz"))
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrContainsAnyError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	containsAny, _ := b.GetAttr("contains_any")

	// Wrong arg count
	result := containsAny.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Wrong type
	result = containsAny.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrContainsRune(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	containsRune, ok := b.GetAttr("contains_rune")
	assert.True(t, ok)

	result := containsRune.(*Builtin).Call(ctx, NewString("e"))
	assert.Equal(t, result.(*Bool).value, true)

	result = containsRune.(*Builtin).Call(ctx, NewString("z"))
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrContainsRuneError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	containsRune, _ := b.GetAttr("contains_rune")

	// Wrong arg count
	result := containsRune.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Wrong type
	result = containsRune.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))

	// Too many characters
	result = containsRune.(*Builtin).Call(ctx, NewString("abc"))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrCount(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello hello"))

	count, ok := b.GetAttr("count")
	assert.True(t, ok)

	result := count.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Equal(t, result.(*Int).Value(), int64(2))
}

func TestBytesGetAttrCountError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	count, _ := b.GetAttr("count")

	// Wrong arg count
	result := count.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Wrong type
	result = count.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrHasPrefix(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	hasPrefix, ok := b.GetAttr("has_prefix")
	assert.True(t, ok)

	result := hasPrefix.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Equal(t, result.(*Bool).value, true)

	result = hasPrefix.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrHasPrefixError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	hasPrefix, _ := b.GetAttr("has_prefix")

	result := hasPrefix.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	result = hasPrefix.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrHasSuffix(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	hasSuffix, ok := b.GetAttr("has_suffix")
	assert.True(t, ok)

	result := hasSuffix.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Equal(t, result.(*Bool).value, true)

	result = hasSuffix.(*Builtin).Call(ctx, NewBytes([]byte("hello")))
	assert.Equal(t, result.(*Bool).value, false)
}

func TestBytesGetAttrHasSuffixError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	hasSuffix, _ := b.GetAttr("has_suffix")

	result := hasSuffix.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	result = hasSuffix.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrIndex(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello world"))

	index, ok := b.GetAttr("index")
	assert.True(t, ok)

	result := index.(*Builtin).Call(ctx, NewBytes([]byte("world")))
	assert.Equal(t, result.(*Int).Value(), int64(6))

	result = index.(*Builtin).Call(ctx, NewBytes([]byte("foo")))
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	index, _ := b.GetAttr("index")

	result := index.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	result = index.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrIndexAny(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	indexAny, ok := b.GetAttr("index_any")
	assert.True(t, ok)

	result := indexAny.(*Builtin).Call(ctx, NewString("aeiou"))
	assert.Equal(t, result.(*Int).Value(), int64(1)) // 'e'

	result = indexAny.(*Builtin).Call(ctx, NewString("xyz"))
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexAnyError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	indexAny, _ := b.GetAttr("index_any")

	result := indexAny.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	result = indexAny.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrIndexByte(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{10, 20, 30})

	indexByte, ok := b.GetAttr("index_byte")
	assert.True(t, ok)

	result := indexByte.(*Builtin).Call(ctx, NewBytes([]byte{20}))
	assert.Equal(t, result.(*Int).Value(), int64(1))

	result = indexByte.(*Builtin).Call(ctx, NewBytes([]byte{99}))
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexByteError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	indexByte, _ := b.GetAttr("index_byte")

	// Wrong arg count
	result := indexByte.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Wrong type
	result = indexByte.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))

	// Must be single byte
	result = indexByte.(*Builtin).Call(ctx, NewBytes([]byte{1, 2}))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrIndexRune(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello"))

	indexRune, ok := b.GetAttr("index_rune")
	assert.True(t, ok)

	result := indexRune.(*Builtin).Call(ctx, NewString("l"))
	assert.Equal(t, result.(*Int).Value(), int64(2))

	result = indexRune.(*Builtin).Call(ctx, NewString("z"))
	assert.Equal(t, result.(*Int).Value(), int64(-1))
}

func TestBytesGetAttrIndexRuneError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	indexRune, _ := b.GetAttr("index_rune")

	// Wrong arg count
	result := indexRune.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Wrong type
	result = indexRune.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))

	// Must be single character
	result = indexRune.(*Builtin).Call(ctx, NewString("abc"))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrRepeat(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("ab"))

	repeat, ok := b.GetAttr("repeat")
	assert.True(t, ok)

	result := repeat.(*Builtin).Call(ctx, NewInt(3))
	assert.Equal(t, result.(*Bytes).Value(), []byte("ababab"))
}

func TestBytesGetAttrRepeatError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	repeat, _ := b.GetAttr("repeat")

	result := repeat.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	result = repeat.(*Builtin).Call(ctx, NewString("test"))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrReplace(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello hello hello"))

	replace, ok := b.GetAttr("replace")
	assert.True(t, ok)

	// Replace with limit
	result := replace.(*Builtin).Call(ctx, NewBytes([]byte("hello")), NewBytes([]byte("hi")), NewInt(2))
	assert.Equal(t, string(result.(*Bytes).Value()), "hi hi hello")
}

func TestBytesGetAttrReplaceError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	replace, _ := b.GetAttr("replace")

	// Wrong arg count
	result := replace.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewBytes([]byte{2}))
	assert.True(t, IsError(result))

	// Wrong type for old
	result = replace.(*Builtin).Call(ctx, NewInt(1), NewBytes([]byte{2}), NewInt(1))
	assert.True(t, IsError(result))

	// Wrong type for new
	result = replace.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewInt(1), NewInt(1))
	assert.True(t, IsError(result))

	// Wrong type for count
	result = replace.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewBytes([]byte{2}), NewString("x"))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrReplaceAll(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte("hello hello hello"))

	replaceAll, ok := b.GetAttr("replace_all")
	assert.True(t, ok)

	result := replaceAll.(*Builtin).Call(ctx, NewBytes([]byte("hello")), NewBytes([]byte("hi")))
	assert.Equal(t, string(result.(*Bytes).Value()), "hi hi hi")
}

func TestBytesGetAttrReplaceAllError(t *testing.T) {
	ctx := context.Background()
	b := NewBytes([]byte{1})
	replaceAll, _ := b.GetAttr("replace_all")

	// Wrong arg count
	result := replaceAll.(*Builtin).Call(ctx, NewBytes([]byte{1}))
	assert.True(t, IsError(result))

	// Wrong type for old
	result = replaceAll.(*Builtin).Call(ctx, NewInt(1), NewBytes([]byte{2}))
	assert.True(t, IsError(result))

	// Wrong type for new
	result = replaceAll.(*Builtin).Call(ctx, NewBytes([]byte{1}), NewInt(1))
	assert.True(t, IsError(result))
}

func TestBytesGetAttrInvalid(t *testing.T) {
	b := NewBytes([]byte{1})
	_, ok := b.GetAttr("invalid_method")
	assert.False(t, ok)
}

func TestBytesCost(t *testing.T) {
	b := NewBytes([]byte{1, 2, 3, 4, 5})
	assert.Equal(t, b.Cost(), 5)
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
