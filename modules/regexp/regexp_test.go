package regexp

import (
	"context"
	"regexp"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/op"
)

func TestRegexpMatch(t *testing.T) {
	// From example: https://pkg.go.dev/regexp#MatchString
	obj := NewRegexp(regexp.MustCompile(`foo.*`))
	match, ok := obj.GetAttr("match")
	assert.True(t, ok)
	result := match.(*object.Builtin).Call(context.Background(), object.NewString("seafood"))
	assert.Equal(t, result, object.True)

	obj = NewRegexp(regexp.MustCompile(`bar.*`))
	match, ok = obj.GetAttr("match")
	assert.True(t, ok)
	result = match.(*object.Builtin).Call(context.Background(), object.NewString("seafood"))
	assert.Equal(t, result, object.False)
}

func TestRegexpMatchErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`foo.*`))
	match, ok := obj.GetAttr("match")
	assert.True(t, ok)

	// Wrong argument count
	result := match.(*object.Builtin).Call(ctx)
	assert.True(t, object.IsError(result))

	result = match.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewString("b"))
	assert.True(t, object.IsError(result))

	// Wrong type
	result = match.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestRegexpFind(t *testing.T) {
	// From example: https://pkg.go.dev/regexp#Regexp.Find
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	find, ok := obj.GetAttr("find")
	assert.True(t, ok)
	result := find.(*object.Builtin).Call(context.Background(), object.NewString("seafood fool"))
	assert.Equal(t, result, object.NewString("food"))
}

func TestRegexpFindErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	find, ok := obj.GetAttr("find")
	assert.True(t, ok)

	// Wrong argument count
	result := find.(*object.Builtin).Call(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type
	result = find.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestRegexpFindAll(t *testing.T) {
	// From example: https://pkg.go.dev/regexp#Regexp.FindAll
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	findAll, ok := obj.GetAttr("find_all")
	assert.True(t, ok)
	result := findAll.(*object.Builtin).Call(context.Background(), object.NewString("seafood fool"))
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewString("food"),
			object.NewString("fool"),
		}))
}

func TestRegexpFindAllWithLimit(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	findAll, ok := obj.GetAttr("find_all")
	assert.True(t, ok)

	// Limit to 1 match
	result := findAll.(*object.Builtin).Call(ctx, object.NewString("seafood fool"), object.NewInt(1))
	assert.Equal(t, result, object.NewList([]object.Object{
		object.NewString("food"),
	}))
}

func TestRegexpFindAllErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	findAll, ok := obj.GetAttr("find_all")
	assert.True(t, ok)

	// Wrong argument count
	result := findAll.(*object.Builtin).Call(ctx)
	assert.True(t, object.IsError(result))

	result = findAll.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewInt(1), object.NewString("c"))
	assert.True(t, object.IsError(result))

	// Wrong type for string
	result = findAll.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))

	// Wrong type for limit
	result = findAll.(*object.Builtin).Call(ctx, object.NewString("foo"), object.NewString("invalid"))
	assert.True(t, object.IsError(result))
}

func TestRegexpFindSubmatch(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(foo)(bar)`))
	findSubmatch, ok := obj.GetAttr("find_submatch")
	assert.True(t, ok)

	result := findSubmatch.(*object.Builtin).Call(ctx, object.NewString("foobar"))
	expected := object.NewList([]object.Object{
		object.NewString("foobar"),
		object.NewString("foo"),
		object.NewString("bar"),
	})
	assert.Equal(t, result, expected)
}

func TestRegexpFindSubmatchErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(foo)(bar)`))
	findSubmatch, ok := obj.GetAttr("find_submatch")
	assert.True(t, ok)

	// Wrong argument count
	result := findSubmatch.(*object.Builtin).Call(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type
	result = findSubmatch.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestRegexpReplaceAll(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`a+`))
	replaceAll, ok := obj.GetAttr("replace_all")
	assert.True(t, ok)

	result := replaceAll.(*object.Builtin).Call(ctx, object.NewString("baaab"), object.NewString("X"))
	assert.Equal(t, result, object.NewString("bXb"))
}

func TestRegexpReplaceAllWithBackreference(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(\w+)\s+(\w+)`))
	replaceAll, ok := obj.GetAttr("replace_all")
	assert.True(t, ok)

	result := replaceAll.(*object.Builtin).Call(ctx, object.NewString("hello world"), object.NewString("$2 $1"))
	assert.Equal(t, result, object.NewString("world hello"))
}

func TestRegexpReplaceAllErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`a+`))
	replaceAll, ok := obj.GetAttr("replace_all")
	assert.True(t, ok)

	// Wrong argument count
	result := replaceAll.(*object.Builtin).Call(ctx, object.NewString("a"))
	assert.True(t, object.IsError(result))

	// Wrong type for string
	result = replaceAll.(*object.Builtin).Call(ctx, object.NewInt(42), object.NewString("X"))
	assert.True(t, object.IsError(result))

	// Wrong type for replacement
	result = replaceAll.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestRegexpSplit(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`\s+`))
	split, ok := obj.GetAttr("split")
	assert.True(t, ok)

	result := split.(*object.Builtin).Call(ctx, object.NewString("hello   world  foo"))
	expected := object.NewList([]object.Object{
		object.NewString("hello"),
		object.NewString("world"),
		object.NewString("foo"),
	})
	assert.Equal(t, result, expected)
}

func TestRegexpSplitWithLimit(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`\s+`))
	split, ok := obj.GetAttr("split")
	assert.True(t, ok)

	// Limit to 2 parts
	result := split.(*object.Builtin).Call(ctx, object.NewString("hello world foo"), object.NewInt(2))
	expected := object.NewList([]object.Object{
		object.NewString("hello"),
		object.NewString("world foo"),
	})
	assert.Equal(t, result, expected)
}

func TestRegexpSplitErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`\s+`))
	split, ok := obj.GetAttr("split")
	assert.True(t, ok)

	// Wrong argument count
	result := split.(*object.Builtin).Call(ctx)
	assert.True(t, object.IsError(result))

	result = split.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewInt(1), object.NewString("c"))
	assert.True(t, object.IsError(result))

	// Wrong type for string
	result = split.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))

	// Wrong type for limit
	result = split.(*object.Builtin).Call(ctx, object.NewString("foo"), object.NewString("invalid"))
	assert.True(t, object.IsError(result))
}

func TestCompile(t *testing.T) {
	ctx := context.Background()

	// Valid pattern
	result := Compile(ctx, object.NewString(`foo.*`))
	_, ok := result.(*Regexp)
	assert.True(t, ok)

	// Invalid pattern
	result = Compile(ctx, object.NewString(`[invalid`))
	assert.True(t, object.IsError(result))
}

func TestCompileErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := Compile(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type
	result = Compile(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestMatch(t *testing.T) {
	ctx := context.Background()

	// Matching
	result := Match(ctx, object.NewString(`foo.*`), object.NewString("foobar"))
	b, ok := result.(*object.Bool)
	assert.True(t, ok)
	assert.True(t, b.Value())

	// Not matching (pattern matches from start with ^)
	result = Match(ctx, object.NewString(`^foo.*`), object.NewString("barfoo"))
	b, ok = result.(*object.Bool)
	assert.True(t, ok)
	assert.False(t, b.Value())

	// Invalid pattern
	result = Match(ctx, object.NewString(`[invalid`), object.NewString("test"))
	assert.True(t, object.IsError(result))
}

func TestMatchErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := Match(ctx, object.NewString("foo"))
	assert.True(t, object.IsError(result))

	// Wrong type for pattern
	result = Match(ctx, object.NewInt(42), object.NewString("test"))
	assert.True(t, object.IsError(result))

	// Wrong type for string
	result = Match(ctx, object.NewString("foo"), object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestRegexpType(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	assert.Equal(t, r.Type(), REGEXP)
}

func TestRegexpInspect(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	assert.Equal(t, r.Inspect(), `regexp("foo")`)
}

func TestRegexpString(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	assert.Equal(t, r.String(), `regexp("foo")`)
}

func TestRegexpInterface(t *testing.T) {
	re := regexp.MustCompile(`foo`)
	r := NewRegexp(re)
	assert.Equal(t, r.Interface(), re)
}

func TestRegexpHashKey(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	hk := r.HashKey()
	assert.Equal(t, hk.Type, REGEXP)
	assert.Equal(t, hk.StrValue, "foo")
}

func TestRegexpCompare(t *testing.T) {
	r1 := NewRegexp(regexp.MustCompile(`aaa`))
	r2 := NewRegexp(regexp.MustCompile(`bbb`))
	r3 := NewRegexp(regexp.MustCompile(`aaa`))

	// Different values
	cmp, err := r1.Compare(r2)
	assert.Nil(t, err)
	assert.True(t, cmp < 0)

	cmp, err = r2.Compare(r1)
	assert.Nil(t, err)
	assert.True(t, cmp > 0)

	// Same pattern, different objects - returns -1 (not equal due to pointer comparison fallback)
	cmp, err = r1.Compare(r3)
	assert.Nil(t, err)
	assert.Equal(t, cmp, -1)

	// Compare with same object
	cmp, err = r1.Compare(r1)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)

	// Compare with different type
	cmp, err = r1.Compare(object.NewString("foo"))
	assert.Nil(t, err)
	assert.NotEqual(t, cmp, 0)
}

func TestRegexpEquals(t *testing.T) {
	re := regexp.MustCompile(`foo`)
	r1 := NewRegexp(re)
	r2 := NewRegexp(re)
	r3 := NewRegexp(regexp.MustCompile(`foo`))

	// Same underlying regexp
	assert.Equal(t, r1.Equals(r2), object.True)

	// Different underlying regexp (same pattern but different object)
	assert.Equal(t, r1.Equals(r3), object.False)

	// Different type
	assert.Equal(t, r1.Equals(object.NewString("foo")), object.False)
}

func TestRegexpMarshalJSON(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	data, err := r.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, string(data), "foo")
}

func TestRegexpRunOperation(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	result := r.RunOperation(op.Add, object.NewString("bar"))
	assert.True(t, object.IsError(result))
}

func TestRegexpSetAttr(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	err := r.SetAttr("test", object.NewString("value"))
	assert.NotNil(t, err)
}

func TestRegexpIsTruthy(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	assert.True(t, r.IsTruthy())
}

func TestRegexpCost(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	assert.Equal(t, r.Cost(), 0)
}

func TestRegexpGetAttrInvalid(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	_, ok := r.GetAttr("invalid_method")
	assert.False(t, ok)
}

func TestModule(t *testing.T) {
	m := Module()
	assert.NotNil(t, m)
	assert.Equal(t, m.Name().Value(), "regexp")

	// Verify functions exist
	_, ok := m.GetAttr("compile")
	assert.True(t, ok)

	_, ok = m.GetAttr("match")
	assert.True(t, ok)
}
