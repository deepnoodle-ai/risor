package risor

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// readBox has fields and a method for the tests of the limits on converting
// Go values to Risor (#599).
type readBox struct {
	Items  []int64
	Index  map[string]int64
	Secret []int64
}

func (b *readBox) All() []int64 { return b.Items }

func newReadBox(n int) *readBox {
	return &readBox{Items: make([]int64, n), Secret: make([]int64, n)}
}

// Reading a field of a Go struct and calling a Go function convert the Go
// value with the limits from WithLimits (#599).
func TestWithLimitsFromGo(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})
	run := func(src string, box *readBox) (any, error) {
		env := Builtins()
		env["box"] = box
		env["items"] = func(n int) []int64 { return make([]int64, n) }
		return Eval(ctx, src, WithEnv(env), limits)
	}

	for _, src := range []string{
		`len(box.Items)`,
		`box?.Items`,
		`let { Items } = box; Items`,
		`let { Items = [] } = box; Items`,
		`getattr(box, "Items")`,
		`getattr(box, "Items", [])`,
		`box.All()`,
		`items(11)`,
	} {
		_, err := run(src, newReadBox(11))
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "value too large to convert"), src, err.Error())
	}

	// The error is a catchable value error
	result, err := run(`try { box.Items } catch e { e.kind() }`, newReadBox(11))
	assert.Nil(t, err)
	assert.Equal(t, result, "value error")

	// A map counts its entries
	box := newReadBox(0)
	box.Index = map[string]int64{}
	for i := range 11 {
		box.Index[fmt.Sprint(i)] = int64(i)
	}
	_, err = run(`box.Index`, box)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())

	// Values at the limit convert
	for _, src := range []string{
		`len(box.Items)`,
		`len(getattr(box, "Items"))`,
		`len(box.All())`,
		`len(items(10))`,
	} {
		result, err := run(src, newReadBox(10))
		assert.Nil(t, err, src)
		assert.Equal(t, result, int64(10), src)
	}

	// A missing attribute is still missing
	result, err = run(`[box?.Missing, getattr(box, "Missing", 1)]`, newReadBox(11))
	assert.Nil(t, err)
	assert.Equal(t, result, []any{nil, int64(1)})
}

// Without WithLimits, reading a field uses the default limits.
func TestDefaultLimitsFromGo(t *testing.T) {
	// A slice of empty structs takes no memory, however long it is
	type wide struct{ Items []struct{} }
	env := WithEnv(map[string]any{
		"box": &wide{Items: make([]struct{}, object.DefaultMaxListLength+1)},
	})
	_, err := Eval(context.Background(), `box.Items`, env)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())
}

// plainReadBox embeds *object.GoStruct and defines neither GetAttr nor
// ReadAttr.
type plainReadBox struct {
	*object.GoStruct
}

// An embedder that does not opt in gets the promoted GoStruct.GetAttr, which
// uses the default limits (#599).
func TestWithLimitsStructEmbedderReadDefaultLimits(t *testing.T) {
	box := newReadBox(11)
	plain := &plainReadBox{object.NewGoStruct(reflect.ValueOf(box), object.DefaultRegistry())}
	envMap := Builtins()
	envMap["box"] = plain
	limits := WithLimits(Limits{MaxListLength: 10})

	result, err := Eval(context.Background(), `len(box.Items)`, WithEnv(envMap), limits)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(11))
}

// readerBox embeds *object.GoStruct, hides a field in both GetAttr and
// ReadAttr, and opts in to the VM context with ReadAttr.
type readerBox struct {
	*object.GoStruct
	getAttrCalls int
}

var _ object.AttrReader = (*readerBox)(nil)

func (b *readerBox) GetAttr(name string) (object.Object, bool) {
	b.getAttrCalls++
	if name == "Secret" {
		return nil, false
	}
	return b.GoStruct.GetAttr(name)
}

func (b *readerBox) ReadAttr(ctx context.Context, name string) (object.Object, bool, error) {
	if name == "Secret" {
		return nil, false, nil
	}
	return b.GoStruct.GetAttrContext(ctx, name)
}

// An embedder that implements AttrReader gets the limits from WithLimits on
// field reads (#599).
func TestWithLimitsStructEmbedderReadAttr(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})
	reader := &readerBox{GoStruct: object.NewGoStruct(reflect.ValueOf(newReadBox(11)), object.DefaultRegistry())}
	envMap := Builtins()
	envMap["box"] = reader
	env := WithEnv(envMap)

	for _, src := range []string{`box.Items`, `getattr(box, "Items")`} {
		_, err := Eval(ctx, src, env, limits)
		assert.NotNil(t, err, src)
		assert.True(t, strings.Contains(err.Error(), "value too large to convert"), err.Error())
	}

	// ReadAttr hides the field
	_, err := Eval(ctx, `box.Secret`, env, limits)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), `attribute "Secret" not found`), err.Error())
	result, err := Eval(ctx, `[box?.Secret, getattr(box, "Secret", 1)]`, env, limits)
	assert.Nil(t, err)
	assert.Equal(t, result, []any{nil, int64(1)})

	// A field within the limit reads
	result, err = Eval(ctx, `len(box.Index)`, env, limits)
	assert.Nil(t, err)
	assert.Equal(t, result, int64(0))

	// The VM and getattr called ReadAttr, not GetAttr
	assert.Equal(t, reader.getAttrCalls, 0)
}
