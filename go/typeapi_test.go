package risor_test

import (
	"context"
	"fmt"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

func must(t risor.Type, err error) risor.Type {
	if err != nil {
		panic(err)
	}
	return t
}

// A host builds the type of a step's output, declares a variable of it,
// and reads the type of the result back.
func Example_typeAPI() {
	status := must(risor.LiteralType("ok", "failed"))
	output := must(risor.RecordType([]risor.FieldType{
		{Key: "status", Type: status},
		{Key: "score", Type: risor.TypeInt, Optional: true},
	}, risor.TypeNever))
	env, _ := risor.NewEnv(risor.VarType("step", output))
	x, _ := risor.Parse(`step?.score ?? 0`)
	_, res, err := env.CheckCompile(x)
	if err != nil {
		panic(err)
	}
	fmt.Println(output)
	fmt.Println(res.ResultType, res.ResultType.Kind() == risor.KindInt)
	// Output:
	// {score?: int, status: "failed" | "ok"}
	// int true
}

func TestTypeAPIConstructors(t *testing.T) {
	list := must(risor.ListType(risor.TypeString))
	assert.Equal(t, list.String(), "string[]")
	e, ok := list.Elem()
	assert.True(t, ok)
	assert.True(t, e.Equal(risor.TypeString))
	m := must(risor.MapType(risor.TypeInt))
	assert.Equal(t, m.String(), "map<int>")
	open := must(risor.RecordType([]risor.FieldType{{Key: "a", Type: risor.TypeBool}}, risor.TypeAny))
	assert.Equal(t, open.String(), "{a: bool, ...}")
	assert.False(t, open.IsExact())
	rest, _ := open.Rest()
	assert.Equal(t, rest.Kind(), risor.KindAny)
	n := must(risor.Nullable(list))
	assert.True(t, n.IsNullable())
	assert.Equal(t, n.Kind(), risor.KindMixed)
	assert.Equal(t, len(n.Alternatives()), 2)
	assert.True(t, n.NonNull().Equal(list))
	tok := must(risor.OpaqueTypeNamed("Token"))
	assert.Equal(t, tok.OpaqueName(), "Token")
	assert.Equal(t, tok.Kind(), risor.KindOpaque)
	lit := must(risor.LiteralType("b", "a", "b"))
	assert.Equal(t, lit.Literals(), []string{"a", "b"})
	assert.Equal(t, risor.TypeString.Literals(), []string(nil))
	_, err := risor.RecordType([]risor.FieldType{{Key: "a"}, {Key: "a"}}, risor.TypeNever)
	assert.Error(t, err)
	_, err = risor.OpaqueTypeNamed("lower")
	assert.Error(t, err)
	_, err = risor.LiteralType()
	assert.Error(t, err)
	assert.Equal(t, risor.TypeNever.Kind(), risor.KindNever)
	var zero risor.Type
	assert.True(t, zero.IsZero())
}

func TestTypeAPIUnions(t *testing.T) {
	circle := must(risor.ExactRecordType(risor.FieldType{Key: "kind", Type: must(risor.LiteralType("circle"))},
		risor.FieldType{Key: "r", Type: risor.TypeFloat}))
	rect := must(risor.ExactRecordType(risor.FieldType{Key: "kind", Type: must(risor.LiteralType("rect"))},
		risor.FieldType{Key: "w", Type: risor.TypeFloat}))
	shape := must(risor.TaggedUnionType(circle, rect))
	assert.Equal(t, shape.Kind(), risor.KindUnion)
	tag, ok := shape.Tag()
	assert.True(t, ok)
	assert.Equal(t, tag, "kind")
	assert.Equal(t, len(shape.Variants()), 2)
	env, _ := risor.NewEnv()
	parsed, err := env.ParseType(`{kind: "circle", r: float} | {kind: "rect", w: float}`)
	assert.NoError(t, err)
	assert.True(t, shape.Equal(parsed))
	// Records with no tag form no union; their Join is one record.
	a := must(risor.ExactRecordType(risor.FieldType{Key: "a", Type: risor.TypeInt}))
	b := must(risor.ExactRecordType(risor.FieldType{Key: "b", Type: risor.TypeInt}))
	_, err = risor.UnionType(a, b)
	assert.Error(t, err)
	_, err = risor.TaggedUnionType(risor.TypeInt, a)
	assert.Error(t, err)
	j := must(risor.Join(a, b))
	assert.Equal(t, j.String(), "{a?: int, b?: int}")
	mixed := must(risor.UnionType(risor.TypeInt, risor.TypeNull, risor.TypeString))
	assert.Equal(t, mixed.String(), "int | string | null")
}

func TestTypeAPIFitsAndJoin(t *testing.T) {
	ok := must(risor.LiteralType("ok"))
	assert.True(t, ok.Fits(risor.TypeString))
	assert.False(t, risor.TypeString.Fits(ok))
	assert.True(t, risor.TypeInt.Fits(risor.TypeFloat))
	j := must(risor.Join(risor.TypeInt, risor.TypeNull))
	assert.True(t, j.Equal(must(risor.Nullable(risor.TypeInt))))
}

func TestTypeAPILimits(t *testing.T) {
	ty := risor.TypeInt
	var err error
	for i := 0; i < 64 && err == nil; i++ {
		ty, err = risor.ListType(ty)
	}
	assert.NoError(t, err)
	_, err = risor.ListType(ty)
	assert.Error(t, err) // depth 65
	// A record of records doubling in size passes 100,000 parts.
	r := risor.TypeInt
	for i := 0; i < 20 && err == nil; i++ {
		r, err = risor.ExactRecordType(risor.FieldType{Key: "l", Type: r}, risor.FieldType{Key: "r", Type: r})
	}
	assert.Error(t, err)
}

func TestCheckResultType(t *testing.T) {
	items := must(risor.ListType(must(risor.ExactRecordType(risor.FieldType{Key: "name", Type: risor.TypeString}))))
	env, _ := risor.NewEnv(risor.VarType("items", items))
	x, _ := risor.Parse(`items[*].name`)
	cr, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, cr.ResultType.String(), cr.Type)
	assert.True(t, cr.ResultType.Equal(must(risor.ListType(risor.TypeString))))
	p, cr2, err := env.CheckCompile(x)
	assert.NoError(t, err)
	assert.True(t, cr2.ResultType.Equal(cr.ResultType))
	v, err := p.Run(context.Background(), risor.Map{"items": []any{map[string]any{"name": "a"}}})
	assert.NoError(t, err)
	assert.Equal(t, v, any([]any{"a"}))
	_, none, err := env.CheckCompile(x, risor.NoCheck)
	assert.NoError(t, err)
	assert.True(t, none == nil)
}
