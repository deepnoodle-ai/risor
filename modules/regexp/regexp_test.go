package regexp

import (
	"context"
	"regexp"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/object"
	"github.com/deepnoodle-ai/risor/v2/op"
)

func TestRegexpMatch(t *testing.T) {
	// From example: https://pkg.go.dev/regexp#MatchString
	obj := NewRegexp(regexp.MustCompile(`foo.*`))
	match, ok := obj.GetAttr("match")
	assert.True(t, ok)
	result, err := match.(*object.Builtin).Call(context.Background(), object.NewString("seafood"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.True)

	obj = NewRegexp(regexp.MustCompile(`bar.*`))
	match, ok = obj.GetAttr("match")
	assert.True(t, ok)
	result, err = match.(*object.Builtin).Call(context.Background(), object.NewString("seafood"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.False)
}

func TestRegexpTest(t *testing.T) {
	// test() is an alias for match()
	obj := NewRegexp(regexp.MustCompile(`foo.*`))
	test, ok := obj.GetAttr("test")
	assert.True(t, ok)
	result, err := test.(*object.Builtin).Call(context.Background(), object.NewString("seafood"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.True)
}

func TestRegexpMatchErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`foo.*`))
	match, ok := obj.GetAttr("match")
	assert.True(t, ok)

	// Wrong argument count
	_, err := match.(*object.Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = match.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewString("b"))
	assert.NotNil(t, err)

	// Wrong type
	_, err = match.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestRegexpFind(t *testing.T) {
	// From example: https://pkg.go.dev/regexp#Regexp.Find
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	find, ok := obj.GetAttr("find")
	assert.True(t, ok)
	result, err := find.(*object.Builtin).Call(context.Background(), object.NewString("seafood fool"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("food"))
}

func TestRegexpFindNoMatch(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`xyz`))
	find, ok := obj.GetAttr("find")
	assert.True(t, ok)
	result, err := find.(*object.Builtin).Call(ctx, object.NewString("hello world"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestRegexpFindErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	find, ok := obj.GetAttr("find")
	assert.True(t, ok)

	// Wrong argument count
	_, err := find.(*object.Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = find.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestRegexpFindAll(t *testing.T) {
	// From example: https://pkg.go.dev/regexp#Regexp.FindAll
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	findAll, ok := obj.GetAttr("find_all")
	assert.True(t, ok)
	result, err := findAll.(*object.Builtin).Call(context.Background(), object.NewString("seafood fool"))
	assert.Nil(t, err)
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
	result, err := findAll.(*object.Builtin).Call(ctx, object.NewString("seafood fool"), object.NewInt(1))
	assert.Nil(t, err)
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
	_, err := findAll.(*object.Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = findAll.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewInt(1), object.NewString("c"))
	assert.NotNil(t, err)

	// Wrong type for string
	_, err = findAll.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)

	// Wrong type for limit
	_, err = findAll.(*object.Builtin).Call(ctx, object.NewString("foo"), object.NewString("invalid"))
	assert.NotNil(t, err)
}

func TestRegexpSearch(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`world`))
	search, ok := obj.GetAttr("search")
	assert.True(t, ok)

	result, err := search.(*object.Builtin).Call(ctx, object.NewString("hello world"))
	assert.Nil(t, err)
	i, ok := result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(6))
}

func TestRegexpSearchNoMatch(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`xyz`))
	search, ok := obj.GetAttr("search")
	assert.True(t, ok)

	result, err := search.(*object.Builtin).Call(ctx, object.NewString("hello world"))
	assert.Nil(t, err)
	i, ok := result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(-1))
}

func TestRegexpSearchErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`foo`))
	search, ok := obj.GetAttr("search")
	assert.True(t, ok)

	_, err := search.(*object.Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = search.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestRegexpGroups(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(foo)(bar)`))
	groups, ok := obj.GetAttr("groups")
	assert.True(t, ok)

	result, err := groups.(*object.Builtin).Call(ctx, object.NewString("foobar"))
	assert.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewString("foobar"),
		object.NewString("foo"),
		object.NewString("bar"),
	})
	assert.Equal(t, result, expected)
}

func TestRegexpGroupsNoMatch(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(foo)(bar)`))
	groups, ok := obj.GetAttr("groups")
	assert.True(t, ok)

	result, err := groups.(*object.Builtin).Call(ctx, object.NewString("hello"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestRegexpGroupsErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(foo)(bar)`))
	groups, ok := obj.GetAttr("groups")
	assert.True(t, ok)

	// Wrong argument count
	_, err := groups.(*object.Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = groups.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestRegexpFindAllGroups(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(\w+)@(\w+)`))
	findAllGroups, ok := obj.GetAttr("find_all_groups")
	assert.True(t, ok)

	result, err := findAllGroups.(*object.Builtin).Call(ctx, object.NewString("user@host admin@server"))
	assert.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewList([]object.Object{
			object.NewString("user@host"),
			object.NewString("user"),
			object.NewString("host"),
		}),
		object.NewList([]object.Object{
			object.NewString("admin@server"),
			object.NewString("admin"),
			object.NewString("server"),
		}),
	})
	assert.Equal(t, result, expected)
}

func TestRegexpFindAllGroupsWithLimit(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(\w+)@(\w+)`))
	findAllGroups, ok := obj.GetAttr("find_all_groups")
	assert.True(t, ok)

	result, err := findAllGroups.(*object.Builtin).Call(ctx, object.NewString("user@host admin@server"), object.NewInt(1))
	assert.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewList([]object.Object{
			object.NewString("user@host"),
			object.NewString("user"),
			object.NewString("host"),
		}),
	})
	assert.Equal(t, result, expected)
}

func TestRegexpFindAllGroupsErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(\w+)@(\w+)`))
	findAllGroups, ok := obj.GetAttr("find_all_groups")
	assert.True(t, ok)

	_, err := findAllGroups.(*object.Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = findAllGroups.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestRegexpReplace(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`a+`))
	replace, ok := obj.GetAttr("replace")
	assert.True(t, ok)

	// Replace all (default)
	result, err := replace.(*object.Builtin).Call(ctx, object.NewString("baaab baaab"), object.NewString("X"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("bXb bXb"))
}

func TestRegexpReplaceWithCount(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`a+`))
	replace, ok := obj.GetAttr("replace")
	assert.True(t, ok)

	// Replace only first match
	result, err := replace.(*object.Builtin).Call(ctx, object.NewString("baaab baaab"), object.NewString("X"), object.NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("bXb baaab"))
}

func TestRegexpReplaceErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`a+`))
	replace, ok := obj.GetAttr("replace")
	assert.True(t, ok)

	_, err := replace.(*object.Builtin).Call(ctx, object.NewString("a"))
	assert.NotNil(t, err)

	_, err = replace.(*object.Builtin).Call(ctx, object.NewInt(42), object.NewString("X"))
	assert.NotNil(t, err)
}

func TestRegexpReplaceAll(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`a+`))
	replaceAll, ok := obj.GetAttr("replace_all")
	assert.True(t, ok)

	result, err := replaceAll.(*object.Builtin).Call(ctx, object.NewString("baaab"), object.NewString("X"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("bXb"))
}

func TestRegexpReplaceAllWithBackreference(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`(\w+)\s+(\w+)`))
	replaceAll, ok := obj.GetAttr("replace_all")
	assert.True(t, ok)

	result, err := replaceAll.(*object.Builtin).Call(ctx, object.NewString("hello world"), object.NewString("$2 $1"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("world hello"))
}

func TestRegexpReplaceAllErrors(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`a+`))
	replaceAll, ok := obj.GetAttr("replace_all")
	assert.True(t, ok)

	// Wrong argument count
	_, err := replaceAll.(*object.Builtin).Call(ctx, object.NewString("a"))
	assert.NotNil(t, err)

	// Wrong type for string
	_, err = replaceAll.(*object.Builtin).Call(ctx, object.NewInt(42), object.NewString("X"))
	assert.NotNil(t, err)

	// Wrong type for replacement
	_, err = replaceAll.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewInt(42))
	assert.NotNil(t, err)
}

func TestRegexpSplit(t *testing.T) {
	ctx := context.Background()
	obj := NewRegexp(regexp.MustCompile(`\s+`))
	split, ok := obj.GetAttr("split")
	assert.True(t, ok)

	result, err := split.(*object.Builtin).Call(ctx, object.NewString("hello   world  foo"))
	assert.Nil(t, err)
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
	result, err := split.(*object.Builtin).Call(ctx, object.NewString("hello world foo"), object.NewInt(2))
	assert.Nil(t, err)
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
	_, err := split.(*object.Builtin).Call(ctx)
	assert.NotNil(t, err)

	_, err = split.(*object.Builtin).Call(ctx, object.NewString("a"), object.NewInt(1), object.NewString("c"))
	assert.NotNil(t, err)

	// Wrong type for string
	_, err = split.(*object.Builtin).Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)

	// Wrong type for limit
	_, err = split.(*object.Builtin).Call(ctx, object.NewString("foo"), object.NewString("invalid"))
	assert.NotNil(t, err)
}

func TestRegexpPattern(t *testing.T) {
	obj := NewRegexp(regexp.MustCompile(`foo\d+`))
	pattern, ok := obj.GetAttr("pattern")
	assert.True(t, ok)
	s, ok := pattern.(*object.String)
	assert.True(t, ok)
	assert.Equal(t, s.Value(), `foo\d+`)
}

func TestRegexpNumGroups(t *testing.T) {
	obj := NewRegexp(regexp.MustCompile(`(foo)(\d+)(bar)?`))
	numGroups, ok := obj.GetAttr("num_groups")
	assert.True(t, ok)
	i, ok := numGroups.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(3))
}

func TestCompile(t *testing.T) {
	ctx := context.Background()

	// Valid pattern
	result, err := Compile(ctx, object.NewString(`foo.*`))
	assert.Nil(t, err)
	_, ok := result.(*Regexp)
	assert.True(t, ok)

	// Invalid pattern
	_, err = Compile(ctx, object.NewString(`[invalid`))
	assert.NotNil(t, err)
}

func TestCompileErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Compile(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = Compile(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestMatch(t *testing.T) {
	ctx := context.Background()

	// Matching
	result, err := Match(ctx, object.NewString(`foo.*`), object.NewString("foobar"))
	assert.Nil(t, err)
	b, ok := result.(*object.Bool)
	assert.True(t, ok)
	assert.True(t, b.Value())

	// Not matching (pattern matches from start with ^)
	result, err = Match(ctx, object.NewString(`^foo.*`), object.NewString("barfoo"))
	assert.Nil(t, err)
	b, ok = result.(*object.Bool)
	assert.True(t, ok)
	assert.False(t, b.Value())

	// Invalid pattern
	_, err = Match(ctx, object.NewString(`[invalid`), object.NewString("test"))
	assert.NotNil(t, err)
}

func TestMatchErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Match(ctx, object.NewString("foo"))
	assert.NotNil(t, err)

	// Wrong type for pattern
	_, err = Match(ctx, object.NewInt(42), object.NewString("test"))
	assert.NotNil(t, err)

	// Wrong type for string
	_, err = Match(ctx, object.NewString("foo"), object.NewInt(42))
	assert.NotNil(t, err)
}

func TestEscape(t *testing.T) {
	ctx := context.Background()

	result, err := Escape(ctx, object.NewString(`foo.bar[0]`))
	assert.Nil(t, err)
	s, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Equal(t, s.Value(), `foo\.bar\[0\]`)
}

func TestEscapeErrors(t *testing.T) {
	ctx := context.Background()

	_, err := Escape(ctx)
	assert.NotNil(t, err)

	_, err = Escape(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestModuleReplace(t *testing.T) {
	ctx := context.Background()

	// Replace all
	result, err := Replace(ctx, object.NewString(`a+`), object.NewString("baaab"), object.NewString("X"))
	assert.Nil(t, err)
	s, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Equal(t, s.Value(), "bXb")
}

func TestModuleReplaceWithCount(t *testing.T) {
	ctx := context.Background()

	// Replace with count
	result, err := Replace(ctx, object.NewString(`a+`), object.NewString("baaab baaab"), object.NewString("X"), object.NewInt(1))
	assert.Nil(t, err)
	s, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Equal(t, s.Value(), "bXb baaab")
}

func TestModuleReplaceErrors(t *testing.T) {
	ctx := context.Background()

	_, err := Replace(ctx, object.NewString(`a+`), object.NewString("test"))
	assert.NotNil(t, err)

	_, err = Replace(ctx, object.NewString(`[invalid`), object.NewString("test"), object.NewString("X"))
	assert.NotNil(t, err)
}

func TestModuleSplit(t *testing.T) {
	ctx := context.Background()

	result, err := Split(ctx, object.NewString(`\s+`), object.NewString("hello world"))
	assert.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewString("hello"),
		object.NewString("world"),
	})
	assert.Equal(t, result, expected)
}

func TestModuleSplitErrors(t *testing.T) {
	ctx := context.Background()

	_, err := Split(ctx, object.NewString(`\s+`))
	assert.NotNil(t, err)
}

func TestModuleFind(t *testing.T) {
	ctx := context.Background()

	result, err := Find(ctx, object.NewString(`\d+`), object.NewString("abc123def"))
	assert.Nil(t, err)
	s, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Equal(t, s.Value(), "123")
}

func TestModuleFindNoMatch(t *testing.T) {
	ctx := context.Background()

	result, err := Find(ctx, object.NewString(`\d+`), object.NewString("abcdef"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestModuleFindAll(t *testing.T) {
	ctx := context.Background()

	result, err := FindAll(ctx, object.NewString(`\d+`), object.NewString("abc123def456"))
	assert.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewString("123"),
		object.NewString("456"),
	})
	assert.Equal(t, result, expected)
}

func TestModuleSearch(t *testing.T) {
	ctx := context.Background()

	result, err := Search(ctx, object.NewString(`world`), object.NewString("hello world"))
	assert.Nil(t, err)
	i, ok := result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(6))
}

func TestModuleSearchNoMatch(t *testing.T) {
	ctx := context.Background()

	result, err := Search(ctx, object.NewString(`xyz`), object.NewString("hello world"))
	assert.Nil(t, err)
	i, ok := result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(-1))
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
	assert.True(t, r1.Equals(r2))

	// Different underlying regexp (same pattern but different object)
	assert.False(t, r1.Equals(r3))

	// Different type
	assert.False(t, r1.Equals(object.NewString("foo")))
}

func TestRegexpMarshalJSON(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	data, err := r.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, string(data), "foo")
}

func TestRegexpRunOperation(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	_, err := r.RunOperation(op.Add, object.NewString("bar"))
	assert.NotNil(t, err)
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

func TestRegexpGetAttrInvalid(t *testing.T) {
	r := NewRegexp(regexp.MustCompile(`foo`))
	_, ok := r.GetAttr("invalid_method")
	assert.False(t, ok)
}

func TestModule(t *testing.T) {
	m := Module()
	assert.NotNil(t, m)
	assert.Equal(t, m.Name().Value(), "regexp")

	// Verify all functions exist
	functions := []string{
		"compile", "match", "escape", "replace", "split", "find", "find_all", "search",
	}
	for _, name := range functions {
		_, ok := m.GetAttr(name)
		assert.True(t, ok, "missing function: %s", name)
	}
}
