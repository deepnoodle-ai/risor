package eval

import (
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
	"strconv"
	"strings"
)

// This file is the conformance walk of package F4, sections 4 and 5
// (spec/packages/f04-conversion.md).

// cfail is a failure of the walk: a path and a template with its
// parameters. The message is built only when it is asked for.
type cfail struct {
	path     []any
	template string // wrong, missing or extra
	expected *types.Type
	found    any
	number   bool // found is a host number that cannot enter
	key      string
}

func (f *cfail) params() []messages.Param {
	switch f.template {
	case "wrong":
		found := messages.Val("found", f.found)
		if f.number {
			found = messages.Word("found", "number")
		}
		return []messages.Param{messages.Word("expected", f.expected.String()), found}
	}
	return []messages.Param{messages.Val("key", f.key)}
}

func (f *cfail) message() string {
	return messages.Build("not-conforming", f.template, f.params()...)
}

// walker tests values against a type. With a run it charges steps and
// tests the depth limit (5.3, 5.4); without one it is the conformance
// function of 7, which charges nothing.
type walker struct {
	r *run
	// noPath is set for is, which builds no path (5.2 item 3).
	noPath bool
}

func (w *walker) step(n int64) {
	if w.r != nil && n > 0 {
		w.r.step(n)
	}
}

func (w *walker) extend(path []any, k any) []any {
	if w.noPath {
		return nil
	}
	out := make([]any, len(path)+1)
	copy(out, path)
	out[len(path)] = k
	return out
}

// test is Test(v, T, path) of 4.2. looked is set for the value of a
// declared field, whose step 3 is the lookup already charged.
func (w *walker) test(v any, t *types.Type, path []any, looked bool) *cfail {
	if t.IsAny() {
		return nil
	}
	v, rerr := value.Read(v)
	k := value.KindOf(v)
	if rerr != value.ReadOK {
		w.step(boolStep(!looked))
		return &cfail{path: path, template: "wrong", expected: t, number: true}
	}
	entered := (k == value.List && t.Elem() != nil) || (k == value.Map && t.Record() != nil)
	if entered && w.r != nil {
		w.r.enterWalk()
		defer func() { w.r.walk-- }()
	}
	if !looked {
		w.step(1)
	}
	wrong := func() *cfail { return &cfail{path: path, template: "wrong", expected: t, found: v} }
	switch k {
	case value.Null:
		if !t.HasNull() {
			return wrong()
		}
	case value.Bool:
		if !t.HasBool() {
			return wrong()
		}
	case value.Number:
		if !t.HasNumber() {
			return wrong()
		}
		if t.IsInt() && !value.IsWhole(v.(float64)) {
			return wrong()
		}
	case value.String:
		if !t.HasString() {
			return wrong()
		}
		if !t.IsStringAll() {
			s := v.(string)
			w.step(value.Size(s))
			if !hasLit(t.Lits(), s) {
				return wrong()
			}
		}
	case value.OpaqueKind:
		name := value.OpaqueTypeName(v)
		found := false
		for _, n := range t.OpaqueNames() {
			if n == name {
				found = true
			}
		}
		if !found {
			return wrong()
		}
	case value.List:
		el := t.Elem()
		if el == nil {
			return wrong()
		}
		n := value.Len(v)
		for i := 0; i < n; i++ {
			if f := w.test(value.At(v, i), el, w.extend(path, float64(i)), false); f != nil {
				return f
			}
		}
	case value.Map:
		r := t.Record()
		if r == nil {
			return wrong()
		}
		if u := t.Union(); u != nil {
			return w.union(v, u, path)
		}
		return w.record(v, r, path, "", false)
	default:
		return wrong()
	}
	return nil
}

func boolStep(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func hasLit(lits []string, s string) bool {
	lo, hi := 0, len(lits)
	for lo < hi {
		m := (lo + hi) / 2
		switch {
		case lits[m] == s:
			return true
		case lits[m] < s:
			lo = m + 1
		default:
			hi = m
		}
	}
	return false
}

// record is 4.3.
// tag is the tag of a union's variant, passed over among the fields;
// hasTag says there is one (a key may be any string, "" among them).
func (w *walker) record(v any, r *types.Record, path []any, tag string, hasTag bool) *cfail {
	present := 0
	if hasTag {
		present = 1 // the tag: already looked up and tested (F5 7.2 step 3)
	}
	for _, f := range r.Fields {
		if hasTag && f.Key == tag {
			continue
		}
		w.step(1 + value.Size(f.Key))
		fv, ok := value.Get(v, f.Key)
		if !ok {
			if !f.Optional {
				return &cfail{path: w.extend(path, f.Key), template: "missing", key: f.Key}
			}
			continue
		}
		present++
		if c := w.test(fv, f.Type, w.extend(path, f.Key), true); c != nil {
			return c
		}
	}
	m := value.Len(v) - present
	if r.Rest.IsAny() || m == 0 {
		return nil
	}
	var others []string
	var weight int64
	for _, k := range value.Keys(v) {
		if _, listed := r.Field(k); !listed {
			others = append(others, k)
			weight += value.Weight(k)
		}
	}
	if r.Rest.IsNever() {
		w.step(int64(m) + weight/64)
		return &cfail{path: w.extend(path, others[0]), template: "extra", key: others[0]}
	}
	w.step(weight / 64)
	var first *cfail
	for _, k := range others {
		fv, _ := value.Get(v, k)
		if c := w.test(fv, r.Rest, w.extend(path, k), false); c != nil && first == nil {
			first = c
		}
	}
	return first
}

// union is the walk of a map against a tagged union (F5 7.2): the tag
// first, then the variant its value names, by the rows of a record.
func (w *walker) union(v any, u *types.Union, path []any) *cfail {
	w.step(1 + value.Size(u.Tag))
	tv, ok := value.Get(v, u.Tag)
	if !ok {
		return &cfail{path: w.extend(path, u.Tag), template: "missing", key: u.Tag}
	}
	if c := w.test(tv, types.TagType(u), w.extend(path, u.Tag), true); c != nil {
		return c
	}
	s, _ := value.Read(tv)
	return w.record(v, types.VariantOf(u, s.(string)), path, u.Tag, true)
}

// Conform is the conformance function of F4 7: nil when v conforms to t,
// else the failure's path and message. It charges nothing and is bounded by
// the value and the type.
func Conform(v any, t *types.Type) (path []any, msg string, ok bool) {
	w := &walker{}
	f := w.test(v, t, []any{}, false)
	if f == nil {
		return nil, "", true
	}
	return f.path, f.message(), false
}

// as compiles an As node (F4 5.1, 5.2).
func (c *compiler) as(n *syntax.As) code {
	x := c.expr(n.X)
	t := c.res.AsTypes[n]
	sp := n.Span()
	if n.Form == "is" {
		return func(r *run) any {
			v := x(r)
			w := walker{r: r, noPath: true}
			return w.test(v, t, nil, false) == nil
		}
	}
	return func(r *run) any {
		v := x(r)
		w := walker{r: r}
		if f := w.test(v, t, []any{}, false); f != nil {
			r.raise(&failure{code: "not-conforming", template: f.template, params: f.params(),
				span: sp, hasSpan: true, path: f.path, hasPath: true})
		}
		return v
	}
}

// JSONText is the JSON text of 03-host.md 3.2.4: the JSON form of a value a
// run returned, keys in code point order. It fails with wrong-kind on an
// opaque value, number-range on a host number that cannot enter the language,
// and depth-exceeded past the default depth limit; code and message are
// returned, and ok is false.
func JSONText(v any) (text, code, msg string, ok bool) {
	return jsonText(v, DefaultLimits.Depth, "json")
}

// RenderText is the rendering of a value (01-language.md 12) under maxDepth:
// empty for null, the string itself for a string, and the JSON form for
// booleans, numbers, lists and maps. It fails with wrong-kind on an opaque
// value, number-range on a host number that cannot enter the language, and
// depth-exceeded past maxDepth.
func RenderText(v any, maxDepth int) (text, code, msg string, ok bool) {
	if maxDepth <= 0 {
		maxDepth = DefaultLimits.Depth
	}
	r, rerr := value.Read(v)
	if rerr == value.ReadRange {
		return "", "number-range", messages.Build("number-range", "host"), false
	}
	switch value.KindOf(r) {
	case value.Null:
		return "", "", "", true
	case value.String:
		return r.(string), "", "", true
	}
	return jsonText(v, maxDepth, "text")
}

func jsonText(v any, maxDepth int, opaqueTemplate string) (text, code, msg string, ok bool) {
	var b strings.Builder
	var walk func(v any, depth int) bool
	walk = func(v any, depth int) bool {
		var rerr value.ReadError
		v, rerr = value.Read(v)
		if rerr == value.ReadRange {
			code, msg = "number-range", messages.Build("number-range", "host")
			return false
		}
		switch value.KindOf(v) {
		case value.Null:
			b.WriteString("null")
		case value.Bool:
			if v.(bool) {
				b.WriteString("true")
			} else {
				b.WriteString("false")
			}
		case value.Number:
			b.WriteString(value.FormatNumber(v.(float64)))
		case value.String:
			b.WriteByte('"')
			value.WriteEscaped(&b, v.(string))
			b.WriteByte('"')
		case value.List, value.Map:
			if depth+1 > maxDepth {
				code, msg = "depth-exceeded", messages.Build("depth-exceeded", "", messages.Word("limit", strconv.Itoa(maxDepth)))
				return false
			}
			if value.KindOf(v) == value.List {
				b.WriteByte('[')
				for i, n := 0, value.Len(v); i < n; i++ {
					if i > 0 {
						b.WriteByte(',')
					}
					if !walk(value.At(v, i), depth+1) {
						return false
					}
				}
				b.WriteByte(']')
				return true
			}
			b.WriteByte('{')
			for i, k := range value.Keys(v) {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteByte('"')
				value.WriteEscaped(&b, k)
				b.WriteString(`":`)
				x, _ := value.Get(v, k)
				if !walk(x, depth+1) {
					return false
				}
			}
			b.WriteByte('}')
		default:
			code, msg = "wrong-kind", messages.Build("wrong-kind", opaqueTemplate)
			return false
		}
		return true
	}
	if !walk(v, 0) {
		return "", code, msg, false
	}
	return b.String(), "", "", true
}
