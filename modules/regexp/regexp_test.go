package regexp

import (
	"context"
	"regexp"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
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

func TestRegexpFind(t *testing.T) {
	// From example: https://pkg.go.dev/regexp#Regexp.Find
	obj := NewRegexp(regexp.MustCompile(`foo.?`))
	find, ok := obj.GetAttr("find")
	assert.True(t, ok)
	result := find.(*object.Builtin).Call(context.Background(), object.NewString("seafood fool"))
	assert.Equal(t, result, object.NewString("food"))
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
