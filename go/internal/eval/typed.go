package eval

import (
	"reflect"

	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is the typed layer of package F11 (04-runtime.md B.3, decision
// D22). When a checked program reads fields of a variable declared from a
// Go type, the compiler plans each field read as an index path through the
// Go struct, and the run follows it with reflect by index: no lookup of a
// key, no copy of a struct into an interface on the way. The same holds for
// a lambda's element parameter in a list method, and for the reads of a
// projection, over a slice of structs.
//
// It changes no result. A planned read is made only when the value has the
// Go type the plan was made for, the field is present, and every pointer on
// the way is not nil; else the general path runs, from the start, and the
// typed path has charged nothing. The steps of the reads are charged at
// once, before the leaf is read, as the general path charges them before
// its last read; the leaf is read as the general path reads it.

// typedStep is one planned field read.
type typedStep struct {
	f *value.StructField
	// deref is set when the field holds a pointer to the next struct.
	deref bool
}

// typedPlan is a run of field reads from a value of a Go type.
type typedPlan struct {
	root  reflect.Type // a plain struct type
	steps []typedStep
	total int64 // the steps of the reads
	span  syntax.Span
	// basic is the leaf's kind when the leaf's type is a predeclared type
	// of that kind, which the run reads without reflection's boxing.
	basic reflect.Kind
}

// structOfType gives the plain struct type a value of Go type t is read
// as, deref through one pointer, or nil.
func structOfType(t reflect.Type) (reflect.Type, bool) {
	if t == nil {
		return nil, false
	}
	ptr := false
	if t.Kind() == reflect.Pointer {
		t, ptr = t.Elem(), true
	}
	if !value.PlainStruct(t) {
		return nil, false
	}
	return t, ptr
}

// planReads plans the leading member steps of a chain from Go type t. It
// gives nil when not even the first can be planned.
func planReads(t reflect.Type, steps []*stepCode) *typedPlan {
	root, _ := structOfType(t)
	if root == nil {
		return nil
	}
	p := &typedPlan{root: root}
	cur := root
	for _, s := range steps {
		if s.kind != stepMember || cur == nil {
			break
		}
		f, ok := value.FieldByKey(cur, s.name)
		if !ok {
			break
		}
		next, ptr := structOfType(f.Type)
		p.steps = append(p.steps, typedStep{f: f, deref: ptr})
		p.total += 1 + s.size
		p.span = s.span
		cur = next
	}
	if len(p.steps) == 0 {
		return nil
	}
	lt := p.steps[len(p.steps)-1].f.Type
	if lt.PkgPath() == "" && lt.Name() == lt.Kind().String() {
		switch lt.Kind() {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
			reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Float64:
			p.basic = lt.Kind()
		}
	}
	return p
}

// rootValue gives the struct a value is read as, when the value has the
// plan's Go type: the struct, or a non-nil pointer to it.
func (p *typedPlan) rootValue(v any) (reflect.Value, bool) {
	t := reflect.TypeOf(v)
	if t == p.root {
		return reflect.ValueOf(v), true
	}
	if t != nil && t.Kind() == reflect.Pointer && t.Elem() == p.root {
		rv := reflect.ValueOf(v)
		if rv.IsNil() {
			return reflect.Value{}, false
		}
		return rv.Elem(), true
	}
	return reflect.Value{}, false
}

// walk follows the plan from a struct value. It reports false where the
// general path must run: a field absent or a nil pointer on the way.
func (p *typedPlan) walk(rv reflect.Value) (reflect.Value, bool) {
	last := len(p.steps) - 1
	for i := range p.steps {
		st := &p.steps[i]
		fv, ok := value.FieldOf(rv, st.f)
		if !ok {
			return reflect.Value{}, false
		}
		if i == last {
			return fv, true
		}
		if st.deref {
			if fv.IsNil() {
				return reflect.Value{}, false
			}
			fv = fv.Elem()
		}
		rv = fv
	}
	return reflect.Value{}, false
}

// leaf reads the last field as the general path reads it: the value the
// field holds, through r.read.
func (r *run) typedLeaf(p *typedPlan, fv reflect.Value) any {
	switch p.basic {
	case reflect.String:
		// Interface boxes without a copy when the field is not addressable,
		// as in a struct held in an interface.
		return r.read(fv.Interface(), p.span)
	case reflect.Bool:
		return fv.Bool()
	case reflect.Int, reflect.Int64:
		if x := fv.Int(); x <= value.MaxWhole && x >= -value.MaxWhole {
			return value.Num(float64(x))
		}
	case reflect.Int8, reflect.Int16, reflect.Int32:
		return value.Num(float64(fv.Int()))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return value.Num(float64(fv.Uint()))
	case reflect.Float64:
		if f := fv.Float(); f != 0 {
			return value.Num(f)
		}
	}
	return r.read(fv.Interface(), p.span)
}

// run makes the planned reads on v, or reports false.
func (r *run) typedReads(p *typedPlan, v any) (any, bool) {
	rv, ok := p.rootValue(v)
	if !ok {
		return nil, false
	}
	fv, ok := p.walk(rv)
	if !ok {
		return nil, false
	}
	r.step(p.total)
	return r.typedLeaf(p, fv), true
}

// typedRoot is the Go type of a chain's base, when the typed layer knows
// it: a variable declared from a Go type, or a lambda's element parameter.
// For a variable it also gives the variable's dense index.
func (c *compiler) typedRoot(x syntax.Node) (reflect.Type, *static.Binding) {
	id, ok := x.(*syntax.Ident)
	if !ok {
		return nil, nil
	}
	b := c.res.Idents[id]
	if b == nil {
		return nil, nil
	}
	switch b.Kind {
	case static.EnvVar:
		if c.cfg.GoTypes == nil {
			return nil, nil
		}
		return c.cfg.GoTypes[b.Name], b
	case static.Value:
		return c.goSlots[b.Slot], b
	}
	return nil, nil
}

// typedChain compiles the leading field reads of a chain on a typed base,
// with general as the code of the same reads by the general path. It gives
// nil when the layer does not apply.
func (c *compiler) typedChain(x syntax.Node, steps []*stepCode, general func(k int) code) (code, int) {
	t, b := c.typedRoot(x)
	if t == nil {
		return nil, 0
	}
	p := planReads(t, steps)
	if p == nil {
		return nil, 0
	}
	k := len(p.steps)
	if k < len(steps) {
		// An optional step that finds null makes the whole chain null, so
		// a prefix that is not the whole chain has no optional step.
		for _, s := range steps[:k] {
			if s.optional {
				return nil, 0
			}
		}
	}
	fallback := general(k)
	c.plans++
	if b.Kind == static.Value {
		slot := b.Slot
		return func(r *run) any {
			if v, ok := r.typedReads(p, r.slots[slot]); ok {
				return v
			}
			return fallback(r)
		}, k
	}
	i := c.varIndex(b)
	name, sp := b.Name, x.Span()
	return func(r *run) any {
		if v, ok := r.typedReads(p, r.varRaw(i, name, sp)); ok {
			return v
		}
		return fallback(r)
	}, k
}

// varIndex is the dense index of a variable, as ident gives it.
func (c *compiler) varIndex(b *static.Binding) int {
	i, ok := c.vars[b.Var]
	if !ok {
		i = len(c.vars)
		c.vars[b.Var] = i
	}
	return i
}

// varRaw is a variable's value as the host gave it, unread, with the
// failure missing-variable as ident raises it.
func (r *run) varRaw(i int, name string, sp syntax.Span) any {
	if !r.varSeen[i] {
		v, ok := r.vars.Var(name)
		if !ok {
			r.raise(fail("missing-variable", "", sp, messages.Word("name", name)))
		}
		r.varVals[i], r.varSeen[i] = v, true
	}
	return r.varVals[i]
}

// stepGoType advances the Go type of a chain's value over one step.
func stepGoType(t reflect.Type, s *stepCode) reflect.Type {
	if t == nil || s.kind != stepMember {
		return nil
	}
	st, _ := structOfType(t)
	if st == nil {
		return nil
	}
	f, ok := value.FieldByKey(st, s.name)
	if !ok {
		return nil
	}
	return f.Type
}

// elemStruct is the element type of a slice or array of plain structs.
func elemStruct(t reflect.Type) reflect.Type {
	if t == nil || (t.Kind() != reflect.Slice && t.Kind() != reflect.Array) {
		return nil
	}
	if st, _ := structOfType(t.Elem()); st != nil {
		return t.Elem()
	}
	return nil
}

// elementParam is the parameter of a list method's lambda that is given
// the element (04-library.md): the first, or the second for reduce.
var elementParam = map[string]int{
	"filter": 0, "map": 0, "flatMap": 0, "some": 0, "every": 0, "find": 0, "count": 0,
	"sortBy": 0, "groupBy": 0, "minBy": 0, "maxBy": 0, "uniqueBy": 0, "indexBy": 0, "countBy": 0,
	"reduce": 1,
}

// typedArgs compiles a method's arguments, giving the element parameter
// of an in-place lambda the element type of a receiver of known Go type.
func (c *compiler) typedArgs(recv reflect.Type, m *syntax.MethodCall) []argCode {
	el := elemStruct(recv)
	idx, ok := elementParam[m.Name]
	if el == nil || !ok || c.cfg.GoTypes == nil {
		return c.args(m.Args)
	}
	var set []int
	for _, a := range m.Args {
		l, ok := a.(*syntax.Lambda)
		if !ok || idx >= len(l.Params) {
			continue
		}
		if np, ok := l.Params[idx].Pattern.(*syntax.NamePat); ok && l.Params[idx].Default == nil {
			if s := c.slot(np); s >= 0 {
				c.goSlots[s] = el
				set = append(set, s)
			}
		}
	}
	out := c.args(m.Args)
	for _, s := range set {
		delete(c.goSlots, s)
	}
	return out
}

// typedEach plans the reads of a projection over a slice of structs: one
// level, member reads only.
func (c *compiler) typedEach(recv reflect.Type, ec *eachCode) {
	el := elemStruct(recv)
	if el == nil || ec.next != nil || len(ec.reads) == 0 {
		return
	}
	p := planReads(el, ec.reads)
	if p == nil || len(p.steps) != len(ec.reads) {
		return
	}
	ec.typed, ec.typedSlice = p, recv
	c.plans++
}

// typedLevel is level for a planned projection: each element whose reads
// the plan can make is read by it; any other by the general path.
func (r *run) typedLevel(ec *eachCode, recv any, out *[]any) bool {
	rv := reflect.ValueOf(recv)
	if rv.Type() != ec.typedSlice {
		return false
	}
	p := ec.typed
	n := rv.Len()
	for i := 0; i < n; i++ {
		r.step(1)
		el := rv.Index(i)
		var sv reflect.Value
		ok := true
		if el.Kind() == reflect.Pointer {
			if el.IsNil() {
				ok = false
			} else {
				sv = el.Elem()
			}
		} else {
			sv = el
		}
		if ok {
			if fv, ok := p.walk(sv); ok {
				r.step(p.total)
				*out = append(*out, r.typedLeaf(p, fv))
				continue
			}
		}
		r.levelElem(ec, value.At(recv, i), out)
	}
	return true
}
