package eval

import (
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// ConformFailure is one failure of ConformAll: a path and a message.
type ConformFailure struct {
	Path    []any
	Message string
	// Code is not-conforming; or depth-exceeded past the default depth
	// limit (F17 12.7), or steps-exceeded past the step limit (F8 6.4),
	// either of which ends the walk.
	Code string
}

// ConformAll is the walk of every failure of package F8 6.2: the failures
// of v against t, at most max of them (a max less than 1 is read as 1), in
// the order of 6.2, under the step limit (NoLimit for none). Its first
// failure is the one Conform gives.
//
// Steps are counted by F8 6.4: first the walk of Conform, whose limit
// failure is the whole answer; then All, from a new count, charging what
// that walk charges at the same places, and ending with steps-exceeded past
// the limit.
func ConformAll(v any, t *types.Type, max int, limit int64) []ConformFailure {
	if max < 1 {
		max = 1
	}
	// Its first failure is Conform's (F8 6.1). When the walk of Conform
	// passes a limit, that failure ends it, and ConformAll ends with it
	// (F17 12.7 item 3, F8 6.4 step 1): the list is that failure alone. All
	// alone could find an ordinary failure first, at a key of a rest that
	// Conform also tests, and then the limit.
	_, code, msg, ok := ConformCode(v, t, limit)
	if ok {
		// It conforms within the limit, and All would charge the same
		// steps (F8 6.4 item 2): the list is empty.
		return nil
	}
	if code != "not-conforming" {
		return []ConformFailure{{Message: msg, Code: code}}
	}
	a := &allWalker{max: max, m: &meter{limit: limit}}
	if metered(func() { a.all(v, t, []any{}) }) {
		a.out = append(a.out, cfail{template: "steps", limit: limit})
	}
	out := make([]ConformFailure, len(a.out))
	for i := range a.out {
		code := "not-conforming"
		switch a.out[i].template {
		case "depth":
			code = "depth-exceeded"
		case "steps":
			code = "steps-exceeded"
		}
		out[i] = ConformFailure{Path: a.out[i].path, Message: a.out[i].message(), Code: code}
	}
	return out
}

type allWalker struct {
	max   int
	out   []cfail
	level int
	deep  bool
	m     *meter
}

func (a *allWalker) full() bool { return a.deep || len(a.out) >= a.max }

func (a *allWalker) add(f cfail) { a.out = append(a.out, f) }

func extendPath(path []any, k any) []any {
	out := make([]any, len(path)+1)
	copy(out, path)
	out[len(path)] = k
	return out
}

// all is All(v, T, path) of 6.2.
func (a *allWalker) all(v any, t *types.Type, path []any) {
	a.allAt(v, t, path, false)
}

// allAt is All; looked is set for the value of a declared field, whose
// step is the lookup already charged (F8 6.4 step 2).
func (a *allWalker) allAt(v any, t *types.Type, path []any, looked bool) {
	if a.full() || t.IsAny() {
		return
	}
	v, rerr := value.Read(v)
	if rerr != value.ReadOK {
		if !looked {
			a.m.charge(1)
		}
		a.add(cfail{path: path, template: "wrong", expected: t, number: true})
		return
	}
	wrong := func() { a.add(cfail{path: path, template: "wrong", expected: t, found: v}) }
	k := value.KindOf(v)
	if (k == value.List && t.Elem() != nil) || (k == value.Map && t.Record() != nil) {
		// Past the default depth limit the walk ends with depth-exceeded,
		// as the first failure's walk does (F17 12.7).
		if a.level+1 > DefaultLimits.Depth {
			a.out = append(a.out, cfail{template: "depth"})
			a.deep = true
			return
		}
		a.level++
		defer func() { a.level-- }()
	}
	// The step of F4 4.2 step 3, after the depth test (F8 6.4 step 2).
	if !looked {
		a.m.charge(1)
	}
	switch k {
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
		if !t.HasString() {
			wrong()
			break
		}
		if !t.IsStringAll() {
			a.m.charge(value.Size(v.(string)))
			if !hasLit(t.Lits(), v.(string)) {
				wrong()
			}
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
			a.m.charge(1 + value.Size(u.Tag))
			tv, ok := value.Get(v, u.Tag)
			if !ok {
				a.add(cfail{path: extendPath(path, u.Tag), template: "missing", key: u.Tag})
				return
			}
			tt := types.TagType(u)
			tv, rerr := value.Read(tv)
			if rerr != value.ReadOK {
				a.add(cfail{path: extendPath(path, u.Tag), template: "wrong", expected: tt, number: true})
				return
			}
			s, isStr := tv.(string)
			if isStr {
				a.m.charge(value.Size(s))
			}
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
		a.m.charge(1 + value.Size(f.Key))
		fv, ok := value.Get(v, f.Key)
		if !ok {
			if !f.Optional {
				a.add(cfail{path: extendPath(path, f.Key), template: "missing", key: f.Key})
			}
			continue
		}
		a.allAt(fv, f.Type, extendPath(path, f.Key), true)
	}
	if r.Rest.IsAny() || a.full() {
		return
	}
	// The other keys, charged as F4 4.3 steps 4 and 5 charge them.
	keys := value.Keys(v)
	var m, weight int64
	for _, k := range keys {
		if _, listed := r.Field(k); listed || (hasTag && k == tag) {
			continue
		}
		m++
		weight += value.Weight(k)
	}
	if r.Rest.IsNever() {
		a.m.charge(m + weight/64)
	} else {
		a.m.charge(weight / 64)
	}
	for _, k := range keys {
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
