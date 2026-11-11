package risor_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A host builds the type of a step's output, declares a variable of it,
// and reads the type of the result back.
func Example_typeAPI() {
	status := risor.LiteralType("ok", "failed")
	output := risor.RecordType([]risor.FieldType{
		{Key: "status", Type: status},
		{Key: "score", Type: risor.TypeInt, Optional: true},
	}, risor.TypeNever)
	env, _ := risor.NewEnv(risor.VarType("step", output))
	x, _ := risor.Parse(`step?.score ?? 0`)
	p, err := env.Compile(x)
	if err != nil {
		panic(err)
	}
	fmt.Println(output)
	res := p.CheckResult()
	fmt.Println(res.Type, res.Type.Kind() == risor.KindInt)
	// Output:
	// {score?: int, status: "failed" | "ok"}
	// int true
}

func TestTypeAPIConstructors(t *testing.T) {
	list := risor.ListType(risor.TypeString)
	assert.Equal(t, list.String(), "string[]")
	e, ok := list.Elem()
	assert.True(t, ok)
	assert.True(t, e.Equal(risor.TypeString))
	m := risor.MapType(risor.TypeInt)
	assert.Equal(t, m.String(), "map<int>")
	open := risor.RecordType([]risor.FieldType{{Key: "a", Type: risor.TypeBool}}, risor.TypeAny)
	assert.Equal(t, open.String(), "{a: bool, ...}")
	assert.False(t, open.IsExact())
	rest, _ := open.Rest()
	assert.Equal(t, rest.Kind(), risor.KindAny)
	n := risor.Nullable(list)
	assert.True(t, n.IsNullable())
	assert.Equal(t, n.Kind(), risor.KindMixed)
	assert.Equal(t, len(n.Alternatives()), 2)
	assert.True(t, n.NonNull().Equal(list))
	tok := risor.OpaqueTypeNamed("Token")
	assert.Equal(t, tok.OpaqueName(), "Token")
	assert.Equal(t, tok.Kind(), risor.KindOpaque)
	lit := risor.LiteralType("b", "a", "b")
	assert.Equal(t, lit.Literals(), []string{"a", "b"})
	assert.Equal(t, risor.TypeString.Literals(), []string(nil))
	for _, bad := range []risor.Type{
		risor.RecordType([]risor.FieldType{{Key: "a"}, {Key: "a"}}, risor.TypeNever),
		risor.OpaqueTypeNamed("lower"),
		risor.LiteralType(),
	} {
		assert.Error(t, bad.Err())
		assert.Equal(t, bad.Kind(), risor.KindInvalid)
		assert.Equal(t, bad.String(), "<invalid type>")
		assert.False(t, bad.IsZero())
	}
	assert.Equal(t, risor.TypeNever.Kind(), risor.KindNever)
	assert.NoError(t, risor.TypeInt.Err())
	var zero risor.Type
	assert.True(t, zero.IsZero())
	assert.Equal(t, zero.Kind(), risor.KindNever)
	f, ok := open.Field("a")
	assert.True(t, ok)
	assert.True(t, f.Type.Equal(risor.TypeBool))
	_, ok = open.Field("b")
	assert.False(t, ok)
}

func TestTypeAPIUnions(t *testing.T) {
	circle := risor.ExactRecordType(risor.FieldType{Key: "kind", Type: risor.LiteralType("circle")},
		risor.FieldType{Key: "r", Type: risor.TypeFloat})
	rect := risor.ExactRecordType(risor.FieldType{Key: "kind", Type: risor.LiteralType("rect")},
		risor.FieldType{Key: "w", Type: risor.TypeFloat})
	shape := risor.TaggedUnionType(circle, rect)
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
	a := risor.ExactRecordType(risor.FieldType{Key: "a", Type: risor.TypeInt})
	b := risor.ExactRecordType(risor.FieldType{Key: "b", Type: risor.TypeInt})
	assert.Error(t, risor.UnionType(a, b).Err())
	assert.Error(t, risor.TaggedUnionType(risor.TypeInt, a).Err())
	j := risor.Join(a, b)
	assert.Equal(t, j.String(), "{a?: int, b?: int}")
	mixed := risor.UnionType(risor.TypeInt, risor.TypeNull, risor.TypeString)
	assert.Equal(t, mixed.String(), "int | string | null")
}

func TestTypeAPIFitsAndJoin(t *testing.T) {
	ok := risor.LiteralType("ok")
	assert.True(t, ok.Fits(risor.TypeString))
	assert.False(t, risor.TypeString.Fits(ok))
	assert.True(t, risor.TypeInt.Fits(risor.TypeFloat))
	j := risor.Join(risor.TypeInt, risor.TypeNull)
	assert.True(t, j.Equal(risor.Nullable(risor.TypeInt)))
}

func TestTypeAPILimits(t *testing.T) {
	ty := risor.TypeInt
	for i := 0; i < 64; i++ {
		ty = risor.ListType(ty)
	}
	assert.NoError(t, ty.Err())
	deep := risor.ListType(ty)
	assert.Error(t, deep.Err()) // depth 65
	// An invalid type passes through every constructor and Join, and a
	// declaration refuses it with its reason.
	carried := risor.Join(risor.TypeInt, risor.MapType(risor.Nullable(deep)))
	assert.Equal(t, carried.Err(), deep.Err())
	assert.False(t, carried.Fits(risor.TypeAny))
	assert.False(t, carried.Equal(carried))
	_, err := risor.NewEnv(risor.VarType("v", carried))
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "deeper than 64"), err.Error())
	// A record of records doubling in size passes 100,000 parts.
	r := risor.TypeInt
	for i := 0; i < 20; i++ {
		r = risor.ExactRecordType(risor.FieldType{Key: "l", Type: r}, risor.FieldType{Key: "r", Type: r})
	}
	assert.Error(t, r.Err())
	// Types compose in one expression.
	assert.Equal(t, risor.ListType(risor.Nullable(risor.TypeString)).String(), "(string | null)[]")
}

func TestCheckResultType(t *testing.T) {
	items := risor.ListType(risor.ExactRecordType(risor.FieldType{Key: "name", Type: risor.TypeString}))
	env, _ := risor.NewEnv(risor.VarType("items", items))
	x, _ := risor.Parse(`items[*].name`)
	cr, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, cr.Type.String(), "string[]")
	assert.True(t, cr.Type.Equal(risor.ListType(risor.TypeString)))
	p, err := env.Compile(x)
	assert.NoError(t, err)
	assert.True(t, p.CheckResult().Type.Equal(cr.Type))
	v, err := p.Run(context.Background(), risor.Map{"items": []any{map[string]any{"name": "a"}}})
	assert.NoError(t, err)
	assert.Equal(t, v, any([]any{"a"}))
	unchecked, err := env.Compile(x, risor.NoCheck)
	assert.NoError(t, err)
	assert.True(t, unchecked.CheckResult() == nil)
}
