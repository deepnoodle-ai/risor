package eval

import (
	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// libCall is one call of a library entry. A callback argument is a
// *lambdaCode.
type libCall struct {
	e       *library.Entry
	recv    any
	args    []any
	span    syntax.Span
	measure int
}

type libFunc func(r *run, c *libCall) any

// implemented maps "f" for a function and "kind.m" for a method to its
// implementation. An entry not here is not built yet (stage S3).
var implemented = map[string]libFunc{
	"len":          libLen,
	"type":         libType,
	"bool":         libBool,
	"string":       libString,
	"fail":         libFail,
	"list.map":     libMap,
	"list.filter":  libFilter,
	"list.count":   libCount,
	"list.some":    libSome,
	"list.every":   libEvery,
	"list.find":    libFind,
	"list.reduce":  libReduce,
	"map.keys":     libKeys,
	"list.first":   libFirst,
	"list.last":    libLast,
	"list.reverse": libReverse,
}

// Key names an entry as Implemented does.
func Key(e *library.Entry) string {
	if e.IsMethod() {
		return e.Kind.String() + "." + e.Name
	}
	return e.Name
}

// Implemented reports whether the evaluator has an entry.
func Implemented(e *library.Entry) bool { return implemented[Key(e)] != nil }

func (r *run) runEntry(c *libCall) any {
	f := implemented[Key(c.e)]
	if f == nil {
		r.raise(&failure{code: "internal-error", span: c.span, hasSpan: true})
	}
	return f(r, c)
}

// testArgs makes the tests of 04-library.md 2.6: each argument left to
// right, the kind before the whole number (ruling AC6). They charge nothing.
func (r *run) testArgs(c *libCall) {
	name := messages.Word("name", c.e.Name)
	for i, v := range c.args {
		p := c.e.ParamAt(i)
		if p == nil || p.Callback >= 0 || p.Any {
			continue
		}
		k := value.KindOf(v)
		if !p.Kinds[k] {
			if p.Ordered {
				r.raise(fail("wrong-kind", "unordered", c.span, name, messages.Val("kind", v)))
			}
			r.raise(fail("wrong-kind", "argument", c.span, name, messages.Val("kind", v)))
		}
		if p.Whole && k == value.Number && !whole(v.(float64)) {
			r.raise(fail("wrong-kind", "whole", c.span, messages.Val("found", v)))
		}
	}
}

// callback calls a lambda for one element: 2 steps, the depth test, the
// call (04-library.md 4.4).
func (r *run) callback(c *libCall, lam *lambdaCode, args ...any) any {
	r.step(2)
	r.enterCall(c.measure)
	v := lam.invoke(r, args)
	r.depth -= c.measure
	return v
}

func libLen(r *run, c *libCall) any {
	x := c.args[0]
	if s, ok := x.(string); ok {
		r.step(value.Size(s))
		return float64(value.RuneCount(s))
	}
	return float64(value.Len(x))
}

func libType(r *run, c *libCall) any { return value.KindOf(c.args[0]).String() }

func libBool(r *run, c *libCall) any { return value.Truthy(c.args[0]) }

func libString(r *run, c *libCall) any {
	x := c.args[0]
	if s, ok := x.(string); ok {
		return s
	}
	var b renderer
	b.r, b.span = r, c.span
	b.top(x)
	s := b.String()
	sz := value.Size(s)
	r.step(sz)
	r.unit(1 + sz)
	return s
}

func libFail(r *run, c *libCall) any {
	f := &failure{code: "failed", span: c.span, hasSpan: true, msg: c.args[0].(string), ownMsg: true, hasData: true}
	if len(c.args) > 1 {
		f.data = c.args[1]
	}
	r.raise(f)
	return nil
}

func (c *libCall) elem(r *run, i int) any { return r.read(value.At(c.recv, i), c.span) }

func libMap(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	n := value.Len(c.recv)
	out := make([]any, n)
	for i := 0; i < n; i++ {
		out[i] = r.callback(c, lam, c.elem(r, i), float64(i))
	}
	r.unit(1 + int64(n))
	return out
}

func libFilter(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	n := value.Len(c.recv)
	var out []any
	for i := 0; i < n; i++ {
		e := c.elem(r, i)
		if value.Truthy(r.callback(c, lam, e, float64(i))) {
			out = append(out, e)
		}
	}
	r.unit(1 + int64(len(out)))
	if out == nil {
		out = []any{}
	}
	return out
}

func libCount(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	n := value.Len(c.recv)
	k := 0
	for i := 0; i < n; i++ {
		if value.Truthy(r.callback(c, lam, c.elem(r, i), float64(i))) {
			k++
		}
	}
	return float64(k)
}

func libSome(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	n := value.Len(c.recv)
	for i := 0; i < n; i++ {
		if value.Truthy(r.callback(c, lam, c.elem(r, i), float64(i))) {
			return true
		}
	}
	return false
}

func libEvery(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	n := value.Len(c.recv)
	for i := 0; i < n; i++ {
		if !value.Truthy(r.callback(c, lam, c.elem(r, i), float64(i))) {
			return false
		}
	}
	return true
}

func libFind(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	n := value.Len(c.recv)
	for i := 0; i < n; i++ {
		e := c.elem(r, i)
		if value.Truthy(r.callback(c, lam, e, float64(i))) {
			return e
		}
	}
	return nil
}

func libReduce(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	acc := c.args[1]
	n := value.Len(c.recv)
	for i := 0; i < n; i++ {
		acc = r.callback(c, lam, acc, c.elem(r, i), float64(i))
	}
	return acc
}

func libKeys(r *run, c *libCall) any {
	keys := value.Keys(c.recv)
	n := int64(len(keys))
	var w int64
	for _, k := range keys {
		w += value.Weight(k)
	}
	r.step(n + L(n)*(n+w/64))
	r.unit(1 + n)
	out := make([]any, len(keys))
	for i, k := range keys {
		out[i] = k
	}
	return out
}

func libFirst(r *run, c *libCall) any {
	if value.Len(c.recv) == 0 {
		return nil
	}
	return c.elem(r, 0)
}

func libLast(r *run, c *libCall) any {
	n := value.Len(c.recv)
	if n == 0 {
		return nil
	}
	return c.elem(r, n-1)
}

func libReverse(r *run, c *libCall) any {
	n := value.Len(c.recv)
	r.step(int64(n))
	r.unit(1 + int64(n))
	out := make([]any, n)
	for i := 0; i < n; i++ {
		out[i] = value.At(c.recv, n-1-i)
	}
	return out
}
