package eval

import (
	"math"
	"math/bits"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// opText is the canonical text of an operator (03-host.md 3.8.3).
func opText(op string) string {
	if op == "not-in" {
		return "not in"
	}
	return op
}

func (c *compiler) unary(n *syntax.Unary) code {
	x := c.expr(n.X)
	sp := n.Span()
	if n.Op == "!" {
		return func(r *run) any {
			v := x(r)
			r.step(1)
			return !value.Truthy(v)
		}
	}
	return func(r *run) any {
		v := x(r)
		f, ok := v.(float64)
		if !ok {
			if v == nil {
				r.raise(fail("null-operand", "operand", sp, messages.Word("op", "-")))
			}
			r.raise(fail("wrong-kind", "operand", sp, messages.Word("op", "-"), messages.Val("kind", v)))
		}
		r.step(1)
		if f == 0 {
			return 0.0
		}
		return -f
	}
}

type binStep struct {
	logic uint8 // 0, or one of logicAnd, logicOr, logicNull
	fn    binFn
	y     code
	k     any // the right operand when it is a number or string literal
	span  syntax.Span
}

// literal gives the value of a number or string literal, or nil. Such an
// operand charges nothing, so it is used without a call.
func literal(n syntax.Node) any {
	switch n := n.(type) {
	case *syntax.Number:
		return n.Value
	case *syntax.String:
		return n.Value
	}
	return nil
}

const (
	logicAnd = 1 + iota
	logicOr
	logicNull
)

// binFn is one operator, chosen when the program is compiled.
type binFn func(r *run, a, b any, sp syntax.Span) any

// binFnFor gives the function of an operator: a fast path for two numbers
// (or ==, != for any values), and binop for the rest. Both charge and fail
// the same way.
func binFnFor(op string) binFn {
	switch op {
	case "==":
		return func(r *run, a, b any, sp syntax.Span) any { return r.equal(a, b, sp) }
	case "!=":
		return func(r *run, a, b any, sp syntax.Span) any { return !r.equal(a, b, sp) }
	case "+":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok {
					r.step(1)
					return r.add(fa, fb, "+", sp)
				}
			} else if sa, ok := a.(string); ok {
				if sb, ok := b.(string); ok {
					var sz int64
					if len(sa)+len(sb) >= 64 {
						sz = (value.Weight(sa) + value.Weight(sb)) / 64
					}
					r.step(1 + sz)
					r.unit(1 + sz)
					return sa + sb
				}
			}
			return r.binop("+", a, b, sp)
		}
	case "-":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok {
					r.step(1)
					return r.add(fa, -fb, "-", sp)
				}
			}
			return r.binop("-", a, b, sp)
		}
	case "*":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok {
					r.step(1)
					return r.mul(fa, fb, "*", sp)
				}
			}
			return r.binop("*", a, b, sp)
		}
	case "in":
		return func(r *run, a, b any, sp syntax.Span) any { return r.in(a, b, "in", sp) }
	case "not-in":
		return func(r *run, a, b any, sp syntax.Span) any { return !r.in(a, b, "not-in", sp) }
	case "%":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok && fb != 0 && whole(fa) && whole(fb) {
					r.step(1)
					return numBox(float64(int64(fa) % int64(fb)))
				}
			}
			return r.binop("%", a, b, sp)
		}
	case "<":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok {
					r.step(1)
					return fa < fb
				}
			}
			return r.binop(op, a, b, sp)
		}
	case "<=":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok {
					r.step(1)
					return fa <= fb
				}
			}
			return r.binop(op, a, b, sp)
		}
	case ">":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok {
					r.step(1)
					return fa > fb
				}
			}
			return r.binop(op, a, b, sp)
		}
	case ">=":
		return func(r *run, a, b any, sp syntax.Span) any {
			if fa, ok := a.(float64); ok {
				if fb, ok := b.(float64); ok {
					r.step(1)
					return fa >= fb
				}
			}
			return r.binop(op, a, b, sp)
		}
	}
	return func(r *run, a, b any, sp syntax.Span) any { return r.binop(op, a, b, sp) }
}

// binary compiles a left-associative chain of operators as one loop over its
// left spine, so a sum of 100,000 terms costs no stack (01-language.md 13.2).
func (c *compiler) binary(n *syntax.Binary) code {
	g := c.binaryGeneral(n)
	// F11: a planned string field compared with a string literal is
	// compared in place (typed.go).
	if n.Op == "==" || n.Op == "!=" {
		if lit, ok := n.Y.(*syntax.String); ok {
			if t := c.typedStrEq(n.X, lit.Value, n.Op == "!=", g); t != nil {
				return t
			}
		}
	}
	return g
}

func (c *compiler) binaryGeneral(n *syntax.Binary) code {
	var spine []*syntax.Binary
	var x syntax.Node = n
	for {
		b, ok := x.(*syntax.Binary)
		if !ok || b.Op == "**" {
			break
		}
		spine = append(spine, b)
		x = b.X
	}
	base := c.expr(x)
	steps := make([]binStep, len(spine))
	for i := range spine {
		b := spine[len(spine)-1-i]
		st := binStep{y: c.expr(b.Y), k: literal(b.Y), span: b.Span()}
		switch b.Op {
		case "&&":
			st.logic = logicAnd
		case "||":
			st.logic = logicOr
		case "??":
			st.logic = logicNull
		default:
			st.fn = binFnFor(b.Op)
			if b.Op == "in" || b.Op == "not-in" {
				if y := constList(b.Y); y != nil {
					st.y = y
				}
			}
		}
		steps[i] = st
	}
	if len(steps) > 1 && allPlus(spine) {
		return plusChain(base, steps)
	}
	if len(steps) == 1 && steps[0].logic == 0 {
		fn, y, sp := steps[0].fn, steps[0].y, steps[0].span
		// A name or a literal operand is read in place, without a call.
		xs, xSlot := c.valueSlot(x)
		ys, ySlot := c.valueSlot(spine[0].Y)
		k := steps[0].k
		switch {
		case xSlot && k != nil:
			return func(r *run) any { return fn(r, r.slots[xs], k, sp) }
		case xSlot && ySlot:
			return func(r *run) any { return fn(r, r.slots[xs], r.slots[ys], sp) }
		case xSlot:
			return func(r *run) any {
				a := r.slots[xs]
				return fn(r, a, y(r), sp)
			}
		case k != nil:
			return func(r *run) any { return fn(r, base(r), k, sp) }
		case ySlot:
			return func(r *run) any { return fn(r, base(r), r.slots[ys], sp) }
		}
		return func(r *run) any {
			a := base(r)
			return fn(r, a, y(r), sp)
		}
	}
	if len(steps) == 2 && steps[0].logic == 0 && steps[1].logic == 0 && steps[0].k != nil && steps[1].k != nil {
		f0, k0, sp0 := steps[0].fn, steps[0].k, steps[0].span
		f1, k1, sp1 := steps[1].fn, steps[1].k, steps[1].span
		return func(r *run) any { return f1(r, f0(r, base(r), k0, sp0), k1, sp1) }
	}
	return func(r *run) any {
		v := base(r)
		for i := range steps {
			s := &steps[i]
			switch s.logic {
			case logicAnd:
				if value.Truthy(v) {
					v = s.y(r)
				}
			case logicOr:
				if !value.Truthy(v) {
					v = s.y(r)
				}
			case logicNull:
				if v == nil {
					v = s.y(r)
				}
			default:
				if s.k != nil {
					v = s.fn(r, v, s.k, s.span)
				} else {
					v = s.fn(r, v, s.y(r), s.span)
				}
			}
		}
		return v
	}
}

func allPlus(spine []*syntax.Binary) bool {
	for _, b := range spine {
		if b.Op != "+" {
			return false
		}
	}
	return true
}

// plusChain is a chain of + whose run of string operands is joined once at
// its end. Each + charges what binop charges, from the weights of its
// operands, at the same point; an operand that is not a string first makes
// the text so far, so a failure sees the same left operand.
func plusChain(base code, steps []binStep) code {
	return func(r *run) any {
		v := base(r)
		var parts [8]string
		np := 0
		var w int64 // weight of the text so far, while np > 0
		flush := func() {
			if np > 0 {
				v = joinParts(parts[:np])
				np = 0
			}
		}
		for i := range steps {
			st := &steps[i]
			y := st.y(r)
			if np > 0 {
				if ys, ok := y.(string); ok && np < len(parts) {
					wy := value.Weight(ys)
					sz := (w + wy) / 64
					r.step(1 + sz)
					r.unit(1 + sz)
					parts[np], w = ys, w+wy
					np++
					continue
				}
				flush()
			} else if vs, ok := v.(string); ok {
				if ys, ok := y.(string); ok {
					wv, wy := value.Weight(vs), value.Weight(ys)
					sz := (wv + wy) / 64
					r.step(1 + sz)
					r.unit(1 + sz)
					parts[0], parts[1], np, w = vs, ys, 2, wv+wy
					continue
				}
			}
			v = st.fn(r, v, y, st.span)
		}
		flush()
		return v
	}
}

func joinParts(ps []string) string {
	n := 0
	for _, p := range ps {
		n += len(p)
	}
	var b strings.Builder
	b.Grow(n)
	for _, p := range ps {
		b.WriteString(p)
	}
	return b.String()
}

// constList compiles a list literal of literals on the right of in. The
// operator only reads the list and does not keep it, so the run uses one list
// made at compile time; it charges the units of the literal as before.
func constList(n syntax.Node) code {
	l, ok := n.(*syntax.List)
	if !ok {
		return nil
	}
	items := make([]any, len(l.Items))
	for i, it := range l.Items {
		switch it := it.(type) {
		case *syntax.Number:
			items[i] = it.Value
		case *syntax.String:
			items[i] = it.Value
		case *syntax.Bool:
			items[i] = it.Value
		case *syntax.Null:
		default:
			return nil
		}
	}
	units := 1 + int64(len(items))
	var list any = items
	return func(r *run) any {
		r.unit(units)
		return list
	}
}

func (c *compiler) power(n *syntax.Binary) code {
	x, y := c.expr(n.X), c.expr(n.Y)
	sp := n.Span()
	return func(r *run) any { return r.binop("**", x(r), y(r), sp) }
}

func whole(f float64) bool { return value.IsWhole(f) }

// numBox makes a number a value without an allocation for a small whole
// number.
func numBox(f float64) any { return value.Num(f) }

func (r *run) rangeFail(op string, sp syntax.Span) {
	r.raise(fail("number-range", "operator", sp, messages.Word("op", opText(op))))
}

// finite checks an IEEE result and gives the one zero.
func (r *run) finite(f float64, op string, sp syntax.Span) float64 {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		r.rangeFail(op, sp)
	}
	if f == 0 {
		return 0
	}
	return f
}

func (r *run) binop(op string, a, b any, sp syntax.Span) any {
	switch op {
	case "==":
		return r.equal(a, b, sp)
	case "!=":
		return !r.equal(a, b, sp)
	case "in":
		return r.in(a, b, op, sp)
	case "not-in":
		return !r.in(a, b, op, sp)
	}
	// Null first, then the kinds.
	if a == nil || b == nil {
		r.raise(fail("null-operand", "operand", sp, messages.Word("op", opText(op))))
	}
	fa, aNum := a.(float64)
	fb, bNum := b.(float64)
	switch op {
	case "+":
		if aNum && bNum {
			r.step(1)
			return r.add(fa, fb, "+", sp)
		}
		sa, aStr := a.(string)
		sb, bStr := b.(string)
		if aStr && bStr {
			sz := (value.Weight(sa) + value.Weight(sb)) / 64
			r.step(1 + sz)
			r.unit(1 + sz)
			return sa + sb
		}
	case "-", "*", "/", "%", "**":
		if aNum && bNum {
			if op == "**" && !whole(fb) {
				r.raise(fail("wrong-kind", "whole", sp, messages.Val("found", fb)))
			}
			r.step(1)
			switch op {
			case "-":
				return r.add(fa, -fb, "-", sp)
			case "*":
				return r.mul(fa, fb, op, sp)
			case "/":
				if fb == 0 {
					r.raise(fail("divide-by-zero", "divisor", sp, messages.Word("op", "/")))
				}
				return numBox(r.finite(fa/fb, op, sp))
			case "%":
				if fb == 0 {
					r.raise(fail("divide-by-zero", "divisor", sp, messages.Word("op", "%")))
				}
				if whole(fa) && whole(fb) {
					m := int64(fa) % int64(fb)
					return numBox(float64(m))
				}
				return numBox(r.finite(math.Mod(fa, fb), op, sp))
			case "**":
				return r.pow(fa, fb, sp)
			}
		}
	case "<", "<=", ">", ">=":
		if aNum && bNum {
			r.step(1)
			return compare(op, cmpFloat(fa, fb))
		}
		sa, aStr := a.(string)
		sb, bStr := b.(string)
		if aStr && bStr {
			r.step(1 + value.Size(value.CommonPrefix(sa, sb)))
			return compare(op, strings.Compare(sa, sb))
		}
	}
	r.raise(fail("wrong-kind", "operands", sp, messages.Word("op", opText(op)),
		messages.Val("left", a), messages.Val("right", b)))
	return nil
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func compare(op string, c int) bool {
	switch op {
	case "<":
		return c < 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	}
	return c >= 0
}

// add is a + b by the table of 02-evaluation.md 3.2: exact on whole
// numbers, IEEE otherwise.
//
// For whole a and b the IEEE sum is the exact sum whenever the exact sum is
// at most 2^53 - 1 in magnitude, and is at least 2^53 in magnitude when the
// exact sum is: so the test on the IEEE sum is the test on the exact one.
func (r *run) add(a, b float64, op string, sp syntax.Span) any {
	s := a + b
	if whole(a) && whole(b) {
		if s > value.MaxWhole || s < -value.MaxWhole {
			r.rangeFail(op, sp)
		}
		return numBox(s)
	}
	return numBox(r.finite(s, op, sp))
}

// mul is a * b. As for add, the IEEE product of whole numbers is exact when
// the exact product is whole, and over 2^53 - 1 in magnitude when it is not.
func (r *run) mul(a, b float64, op string, sp syntax.Span) any {
	p := a * b
	if whole(a) && whole(b) {
		if p > value.MaxWhole || p < -value.MaxWhole {
			r.rangeFail(op, sp)
		}
		return numBox(p)
	}
	return numBox(r.finite(p, op, sp))
}

// mulWhole multiplies two whole numbers exactly and reports whether the
// product is whole.
func mulWhole(a, b int64) (int64, bool) {
	neg := (a < 0) != (b < 0)
	ua, ub := uint64(abs64(a)), uint64(abs64(b))
	hi, lo := bits.Mul64(ua, ub)
	if hi != 0 || lo > value.MaxWhole {
		return 0, false
	}
	p := int64(lo)
	if neg {
		p = -p
	}
	return p, true
}

func abs64(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}

// pow is a ** b, b whole, by 01-language.md 6.2.
func (r *run) pow(a, b float64, sp syntax.Span) any {
	if b < 0 {
		if a == 0 {
			r.raise(fail("divide-by-zero", "power", sp))
		}
		p := ieeePow(a, uint64(-b))
		if math.IsInf(p, 0) {
			return 0.0
		}
		if p == 0 {
			r.rangeFail("**", sp)
		}
		return r.finite(1/p, "**", sp)
	}
	e := uint64(b)
	if whole(a) {
		res := int64(1)
		ia := int64(a)
		for i := 63 - bits.LeadingZeros64(e); i >= 0 && e != 0; i-- {
			var ok bool
			if res, ok = mulWhole(res, res); !ok {
				r.rangeFail("**", sp)
			}
			if e>>uint(i)&1 == 1 {
				if res, ok = mulWhole(res, ia); !ok {
					r.rangeFail("**", sp)
				}
			}
		}
		return float64(res)
	}
	return r.finite(ieeePow(a, e), "**", sp)
}

func ieeePow(a float64, e uint64) float64 {
	res := 1.0
	if e == 0 {
		return 1
	}
	for i := 63 - bits.LeadingZeros64(e); i >= 0; i-- {
		res = res * res
		if e>>uint(i)&1 == 1 {
			res = res * a
		}
	}
	return res
}

// equal is a == b, charging by 02-evaluation.md 5.3.
func (r *run) equal(a, b any, sp syntax.Span) bool {
	switch x := a.(type) {
	case float64:
		if y, ok := b.(float64); ok {
			r.step(1)
			return x == y
		}
	case string:
		if y, ok := b.(string); ok {
			r.step(1 + value.Size(value.CommonPrefix(x, y)))
			return x == y
		}
	}
	ka, kb := value.KindOf(a), value.KindOf(b)
	if ka != kb {
		r.step(1)
		return false
	}
	switch ka {
	case value.Null:
		r.step(1)
		return true
	case value.Bool:
		r.step(1)
		return a.(bool) == b.(bool)
	case value.Number:
		r.step(1)
		return num(a) == num(b)
	case value.String:
		sa, sb := a.(string), b.(string)
		r.step(1 + value.Size(value.CommonPrefix(sa, sb)))
		return sa == sb
	case value.OpaqueKind:
		r.step(1)
		return value.SameOpaque(a, b)
	case value.List:
		n := value.Len(a)
		if n != value.Len(b) {
			r.step(1)
			return false
		}
		return r.listEqual(a, b, n, sp)
	case value.Map:
		n := value.Len(a)
		if n != value.Len(b) {
			r.step(1)
			return false
		}
		return r.mapEqual(a, b, sp)
	}
	return false
}

func num(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	x, _ := value.Read(v)
	f, _ := x.(float64)
	return f
}

func (r *run) listEqual(a, b any, n int, sp syntax.Span) bool {
	r.enterWalk()
	eq := true
	for i := 0; i < n; i++ {
		r.step(1)
		if !r.equal(r.read(value.At(a, i), sp), r.read(value.At(b, i), sp), sp) {
			eq = false
			break
		}
	}
	r.walk--
	return eq
}

func (r *run) mapEqual(a, b any, sp syntax.Span) bool {
	r.enterWalk()
	// The entries are visited in key order when some value is a list or a
	// map, whose walk could meet the depth limit: the order then decides
	// which limit a run passes first (02-evaluation.md 5.3 item
	// 3). Otherwise the order cannot show, and no sort is made.
	var keys []string
	var w int64
	nested := false
	value.ForEachKey(a, func(k string) {
		keys = append(keys, k)
		w += value.Weight(k)
		if !nested {
			x, _ := value.Get(a, k)
			switch value.KindOf(x) {
			case value.List, value.Map:
				nested = true
			}
		}
	})
	if nested {
		sort.Strings(keys)
	}
	r.step(w / 64)
	eq := true
	for _, k := range keys {
		r.step(1)
		bv, ok := value.Get(b, k)
		if !ok {
			eq = false
			continue
		}
		av, _ := value.Get(a, k)
		if !r.equal(r.read(av, sp), r.read(bv, sp), sp) {
			eq = false
		}
	}
	r.walk--
	return eq
}

// in is x in y (01-language.md 6.8).
func (r *run) in(x, y any, op string, sp syntax.Span) bool {
	ky := value.KindOf(y)
	switch ky {
	case value.Null:
		r.raise(fail("null-operand", "operand", sp, messages.Word("op", opText(op))))
	case value.List:
		if ys, ok := y.([]any); ok {
			if xs, ok := x.(string); ok {
				// A string sought in a list: only a string element can be
				// equal, and it is charged by its common prefix.
				for _, e := range ys {
					e = r.read(e, sp)
					r.step(1)
					if es, ok := e.(string); ok {
						r.step(value.Size(value.CommonPrefix(xs, es)))
						if xs == es {
							return true
						}
					}
				}
				return false
			}
		}
		n := value.Len(y)
		kx := value.KindOf(x)
		for i := 0; i < n; i++ {
			e := r.read(value.At(y, i), sp)
			r.step(1)
			ke := value.KindOf(e)
			switch {
			case kx == value.String && ke == value.String:
				xs, es := x.(string), e.(string)
				r.step(value.Size(value.CommonPrefix(xs, es)))
				if xs == es {
					return true
				}
			case kx == value.List && ke == value.List && value.Len(x) == value.Len(e),
				kx == value.Map && ke == value.Map && value.Len(x) == value.Len(e):
				if r.equal(x, e, sp) {
					return true
				}
			default:
				if plainEqual(x, e) {
					return true
				}
			}
		}
		return false
	case value.String, value.Map:
		xs, ok := x.(string)
		if !ok {
			if x == nil {
				r.raise(fail("null-operand", "operand", sp, messages.Word("op", opText(op))))
			}
			r.raise(fail("wrong-kind", "operands", sp, messages.Word("op", opText(op)),
				messages.Val("left", x), messages.Val("right", y)))
		}
		if ky == value.Map {
			r.step(1 + value.Size(xs))
			_, found := value.Get(y, xs)
			return found
		}
		ys := y.(string)
		r.step(1 + value.Size(ys))
		return contains(ys, xs)
	}
	r.raise(fail("wrong-kind", "operands", sp, messages.Word("op", opText(op)),
		messages.Val("left", x), messages.Val("right", y)))
	return false
}

// plainEqual compares two values of which at most one is a list or a map, or
// lists of different lengths, or maps of different sizes: no walk is needed.
func plainEqual(a, b any) bool {
	ka, kb := value.KindOf(a), value.KindOf(b)
	if ka != kb {
		return false
	}
	switch ka {
	case value.Null:
		return true
	case value.Bool:
		return a.(bool) == b.(bool)
	case value.Number:
		return num(a) == num(b)
	case value.String:
		return a.(string) == b.(string)
	case value.OpaqueKind:
		return value.SameOpaque(a, b)
	}
	return false
}

// contains reports whether t occurs in s, in work linear in the length of s: a text longer than s is answered from the lengths, and the
// search is Knuth-Morris-Pratt over the bytes of valid UTF-8.
func contains(s, t string) bool {
	if len(t) > len(s) {
		return false
	}
	if len(t) == 0 {
		return true
	}
	if len(t) < 64 {
		return strings.Contains(s, t)
	}
	fail := make([]int, len(t))
	for i, k := 1, 0; i < len(t); i++ {
		for k > 0 && t[i] != t[k] {
			k = fail[k-1]
		}
		if t[i] == t[k] {
			k++
		}
		fail[i] = k
	}
	for i, k := 0, 0; i < len(s); i++ {
		for k > 0 && s[i] != t[k] {
			k = fail[k-1]
		}
		if s[i] == t[k] {
			k++
		}
		if k == len(t) {
			return true
		}
	}
	return false
}

// L is L(n) of 02-evaluation.md 1.2.
func L(n int64) int64 {
	if n < 2 {
		return 0
	}
	return int64(bits.Len64(uint64(n - 1)))
}

// renderer builds the rendering of values (01-language.md 12) and charges
// the rendering walk (02-evaluation.md 5.5).
//
// The string's units come before it is built (02-evaluation.md 6.2 item 3),
// so the walk keeps the text only while it is short: past renderKeep bytes it
// measures the rest, and text renders the values again, uncharged, once the
// caller has charged. A short text is built in one pass.
type renderer struct {
	b    strings.Builder
	r    *run
	span syntax.Span
	// opaque is the template of wrong-kind for an opaque value: text, or
	// json for json.stringify (package F6 6.1).
	opaque string
	// keepTo is the length up to which b keeps the text: renderKeep at
	// first; -1 once the text has passed it, when b is empty and w and n are
	// the weight and the length in bytes of the text; and no bound for the
	// second rendering, whose walk is charged.
	keepTo int
	w      int64
	n      int
}

// renderKeep is the length in bytes up to which the walk keeps its text.
const renderKeep = 64 << 10

// keep reports whether n more bytes of text go into b. When they do not,
// the caller measures them.
func (b *renderer) keep(n int) bool {
	return b.b.Len()+n <= b.keepTo || b.stop()
}

// stop turns the walk from keeping its text to measuring it. It reports
// false.
//
//go:noinline
func (b *renderer) stop() bool {
	if b.keepTo >= 0 {
		b.keepTo, b.w, b.n = -1, value.Weight(b.b.String()), b.b.Len()
		b.b = strings.Builder{}
	}
	return false
}

// measure counts s, or its JSON form when quoted, as text not kept.
//
//go:noinline
func (b *renderer) measure(s string, quoted bool) {
	x := 0
	if quoted {
		x = 2 + value.EscapeExtra(s) // the quotes and escapes are ASCII
	}
	b.w += value.Weight(s) + int64(x)
	b.n += len(s) + x
}

func (b *renderer) write(s string) {
	if b.keep(len(s)) {
		b.b.WriteString(s)
	} else {
		b.measure(s, false)
	}
}

// quote writes the JSON form of s. Its escapes can take b past keepTo by at
// most 5 times the length of s.
func (b *renderer) quote(s string) {
	if b.keep(len(s) + 2) {
		b.b.WriteByte('"')
		value.WriteEscaped(&b.b, s)
		b.b.WriteByte('"')
	} else {
		b.measure(s, true)
	}
}

// paid reports whether this is the second rendering, which charges nothing.
func (b *renderer) paid() bool { return b.keepTo == math.MaxInt }

// weight is the weight of the text.
func (b *renderer) weight() int64 {
	if b.keepTo < 0 {
		return b.w
	}
	return value.Weight(b.b.String())
}

// text is the string, once its charges are made. When the walk did not keep
// it, again renders it once more, into a renderer that charges nothing.
func (b *renderer) text(again func(*renderer)) string {
	if b.keepTo >= 0 {
		return b.b.String()
	}
	p := renderer{r: b.r, span: b.span, keepTo: math.MaxInt}
	p.b.Grow(b.n)
	again(&p)
	return p.b.String()
}

// top renders a value as a template part or the argument of string.
func (b *renderer) top(v any) {
	switch x := v.(type) {
	case nil:
	case string:
		b.write(x)
	case float64:
		b.write(value.FormatNumber(x))
	case bool:
		if x {
			b.write("true")
		} else {
			b.write("false")
		}
	default:
		b.json(v)
	}
}

func (b *renderer) json(v any) {
	r := b.r
	switch value.KindOf(v) {
	case value.Null:
		b.write("null")
	case value.String:
		b.quote(v.(string))
	case value.Number, value.Bool:
		b.top(v)
	case value.List:
		n := value.Len(v)
		if !b.paid() {
			r.enterWalk()
			r.step(int64(n))
		}
		b.write("[")
		for i := 0; i < n; i++ {
			if i > 0 {
				b.write(",")
			}
			b.json(r.read(value.At(v, i), b.span))
		}
		b.write("]")
		if !b.paid() {
			r.walk--
		}
	case value.Map:
		if !b.paid() {
			r.enterWalk()
		}
		keys := value.Keys(v)
		if !b.paid() {
			n := int64(len(keys))
			var w int64
			for _, k := range keys {
				w += value.Weight(k)
			}
			r.step(n + L(n)*(n+w/64))
		}
		b.write("{")
		for i, k := range keys {
			if i > 0 {
				b.write(",")
			}
			b.quote(k)
			b.write(":")
			x, _ := value.Get(v, k)
			b.json(r.read(x, b.span))
		}
		b.write("}")
		if !b.paid() {
			r.walk--
		}
	default:
		t := b.opaque
		if t == "" {
			t = "text"
		}
		r.raise(fail("wrong-kind", t, b.span))
	}
}
