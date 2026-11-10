package eval

import (
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is the projection of package F7 (f07-queries.md 3, 4).

// eachCode is one level of a projection: its star, its reads, and the
// next level.
type eachCode struct {
	optional bool
	starSpan syntax.Span
	reads    []*stepCode
	next     *eachCode
}

func (c *compiler) eachCode(n *syntax.Each) *eachCode {
	ec := &eachCode{optional: n.Optional, starSpan: n.StarSpan}
	for _, rd := range n.Reads {
		switch t := rd.(type) {
		case *syntax.Member:
			ec.reads = append(ec.reads, &stepCode{kind: stepMember, optional: t.Optional, name: t.Name,
				size: value.Size(t.Name), span: t.Span()})
		case *syntax.Index:
			ec.reads = append(ec.reads, &stepCode{kind: stepIndex, optional: t.Optional, idx: c.expr(t.Index), span: t.Span()})
		case *syntax.Each:
			ec.next = c.eachCode(t)
		}
	}
	return ec
}

// starTest is 3.1 items 2 and 3 for a non-null receiver, and item 2 for a
// plain star.
func (r *run) starTest(ec *eachCode, v any) {
	switch value.KindOf(v) {
	case value.List:
		return
	case value.Null:
		r.raise(fail("null-operand", "star", ec.starSpan))
	}
	r.raise(fail("wrong-kind", "star", ec.starSpan, messages.Val("kind", v)))
}

// project evaluates a projection on the receiver of its first star. The
// chain's step loop has already made a null receiver of a first ?.[*] the
// chain's null.
func (r *run) project(ec *eachCode, v any) any {
	r.starTest(ec, v)
	// The scratch list is the result: sized for one level, it is built once
	// in the common case (4.1 item 2).
	out := make([]any, 0, value.Len(v))
	r.level(ec, v, &out)
	r.unit(1 + int64(len(out))) // once, after the last step (4.1)
	return out
}

// level visits the elements of one level, in order, depth first.
func (r *run) level(ec *eachCode, recv any, out *[]any) {
	n := value.Len(recv)
	for i := 0; i < n; i++ {
		r.step(1)
		el := value.At(recv, i)
		if len(ec.reads) == 0 && ec.next == nil {
			*out = append(*out, el) // copied, not read (3.6)
			continue
		}
		v := r.read(el, ec.starSpan)
		skipped := false
		for _, s := range ec.reads {
			if v == nil && s.optional {
				skipped = true
				break
			}
			if s.kind == stepMember {
				v = r.member(s, v)
			} else {
				v = r.index(s, v)
			}
		}
		switch {
		case skipped:
			*out = append(*out, nil)
		case ec.next == nil:
			*out = append(*out, v)
		case v == nil && ec.next.optional:
			*out = append(*out, nil)
		default:
			r.starTest(ec.next, v)
			r.level(ec.next, v, out)
		}
	}
}
