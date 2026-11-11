package eval

import (
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// ConformFailure is one failure of ConformAll: a path and a message.
type ConformFailure struct {
	Path    []any
	Message string
}

// ConformAll is the walk of every failure of package F8 6.2: the failures
// of v against t, at most max of them (a max less than 1 is read as 1), in
// the order of 6.2. It charges nothing, and its first failure is the one
// Conform gives.
func ConformAll(v any, t *types.Type, max int) []ConformFailure {
	if max < 1 {
		max = 1
	}
	a := &allWalker{max: max}
	a.all(v, t, []any{})
	out := make([]ConformFailure, len(a.out))
	for i := range a.out {
		out[i] = ConformFailure{Path: a.out[i].path, Message: a.out[i].message()}
	}
	return out
}

type allWalker struct {
	max int
	out []cfail
}

func (a *allWalker) full() bool { return len(a.out) >= a.max }

func (a *allWalker) add(f cfail) { a.out = append(a.out, f) }

func extendPath(path []any, k any) []any {
	out := make([]any, len(path)+1)
	copy(out, path)
	out[len(path)] = k
	return out
}

// all is All(v, T, path) of 6.2.
func (a *allWalker) all(v any, t *types.Type, path []any) {
	if a.full() || t.IsAny() {
		return
	}
	v, rerr := value.Read(v)
	if rerr != value.ReadOK {
		a.add(cfail{path: path, template: "wrong", expected: t, number: true})
		return
	}
	wrong := func() { a.add(cfail{path: path, template: "wrong", expected: t, found: v}) }
	switch value.KindOf(v) {
	case value.Null:
		if !t.HasNull() {
			wrong()
		}
	case value.Bool:
		if !t.HasBool() {
			wrong()
		}
	case value.Number:
		if !t.HasNumber() || (t.IsInt() && !value.IsWhole(v.(float64))) {
			wrong()
		}
	case value.String:
		if !t.HasString() || (!t.IsStringAll() && !hasLit(t.Lits(), v.(string))) {
			wrong()
		}
	case value.OpaqueKind:
		name := value.OpaqueTypeName(v)
		for _, n := range t.OpaqueNames() {
			if n == name {
				return
			}
		}
		wrong()
	case value.List:
		el := t.Elem()
		if el == nil {
			wrong()
			return
		}
		n := value.Len(v)
		for i := 0; i < n && !a.full(); i++ {
			a.all(value.At(v, i), el, extendPath(path, float64(i)))
		}
	case value.Map:
		r := t.Record()
		if r == nil {
			wrong()
			return
		}
		if u := t.Union(); u != nil {
			tv, ok := value.Get(v, u.Tag)
			if !ok {
				a.add(cfail{path: extendPath(path, u.Tag), template: "missing", key: u.Tag})
				return
			}
			tt := types.TagType(u)
			tv, _ = value.Read(tv)
			s, isStr := tv.(string)
			if !isStr || !hasLit(tt.Lits(), s) {
				a.add(cfail{path: extendPath(path, u.Tag), template: "wrong", expected: tt, found: tv})
				return
			}
			a.record(v, types.VariantOf(u, s), path, u.Tag, true)
			return
		}
		a.record(v, r, path, "", false)
	default:
		wrong()
	}
}

// record is step 6 of 6.2: the declared fields in key order, then every
// other key, whatever the declared fields gave.
func (a *allWalker) record(v any, r *types.Record, path []any, tag string, hasTag bool) {
	for _, f := range r.Fields {
		if a.full() {
			return
		}
		if hasTag && f.Key == tag {
			continue
		}
		fv, ok := value.Get(v, f.Key)
		if !ok {
			if !f.Optional {
				a.add(cfail{path: extendPath(path, f.Key), template: "missing", key: f.Key})
			}
			continue
		}
		a.all(fv, f.Type, extendPath(path, f.Key))
	}
	if r.Rest.IsAny() {
		return
	}
	for _, k := range value.Keys(v) {
		if a.full() {
			return
		}
		if _, listed := r.Field(k); listed || (hasTag && k == tag) {
			continue
		}
		if r.Rest.IsNever() {
			a.add(cfail{path: extendPath(path, k), template: "extra", key: k})
			continue
		}
		fv, _ := value.Get(v, k)
		a.all(fv, r.Rest, extendPath(path, k))
	}
}
