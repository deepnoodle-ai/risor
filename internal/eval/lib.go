package eval

import (
	"math"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v3/internal/casemap"
	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// libCall is one call of a library entry. A callback argument is a
// *lambdaCode.
type libCall struct {
	e       *library.Entry
	info    *entryInfo
	recv    any
	args    []any
	span    syntax.Span
	measure int
	// The trace of package F8 4 is read from the stack of calls when a run
	// fails, so a run that does not fail builds none. depth0 is the depth
	// when the call began: the entry's call of a lambda is in progress while
	// the depth is past depth0 + measure. idx is the index the entry gave
	// its last call of a lambda. A record with lam is a call by name of a
	// const-bound lambda, named name.
	depth0 int
	idx    int
	lam    bool
	name   string
	// ref is the element a lambda that takes it by reference reads, and
	// rv the receiver as a reflect.Value once a reference is made (typed.go,
	// package F11).
	ref   elemRef
	rv    reflect.Value
	rvSet bool
}

type libFunc func(r *run, c *libCall) any

// Key names an entry: "f" for a function, "kind.m" for a method.
func Key(e *library.Entry) string {
	if e.IsMethod() {
		return e.Kind.String() + "." + e.Name
	}
	return e.Name
}

// Implemented reports whether the evaluator has an entry.
func Implemented(e *library.Entry) bool {
	return implemented[Key(e)] != nil || groupImplemented[e.Name] != nil
}

// Keys of the registry, for the test that it matches library.json.
func RegistryKeys() []string {
	out := make([]string, 0, len(implemented))
	for k := range implemented {
		out = append(out, k)
	}
	return out
}

func (r *run) runEntry(c *libCall) any {
	f := c.info.impl
	if f == nil {
		r.raise(&failure{code: "internal-error", span: c.span, hasSpan: true})
	}
	return f(r, c)
}

// fenv gives a record's formula the sizes of one call (04-library.md 4.2).
type fenv struct {
	c                *libCall
	n, k, v, p, g    int64
	resultW, elemsW  int64
	keysW, walkSteps int64
}

func (f *fenv) Sym(s byte) int64 {
	switch s {
	case 'n':
		return f.n
	case 'k':
		return f.k
	case 'v':
		return f.v
	case 'p':
		return f.p
	case 'g':
		return f.g
	}
	return L(f.n)
}

func (f *fenv) arg(name string) any {
	for i, p := range f.c.e.Params {
		if p.Name == name && i < len(f.c.args) {
			return f.c.args[i]
		}
	}
	return nil
}

func (f *fenv) Weight(name string) int64 {
	switch name {
	case "s":
		if s, ok := f.c.recv.(string); ok {
			return value.Weight(s)
		}
		return 0
	case "result":
		return f.resultW
	case "elements":
		return f.elemsW
	case "keys":
		return f.keysW
	case "arguments":
		var w int64
		for _, a := range f.c.args {
			if s, ok := a.(string); ok {
				w += value.Weight(s)
			}
		}
		return w
	}
	if s, ok := f.arg(name).(string); ok {
		return value.Weight(s)
	}
	return 0
}

func (f *fenv) IsString(name string) bool { _, ok := f.arg(name).(string); return ok }
func (f *fenv) Len(string) int64          { return int64(len(f.c.args)) }
func (f *fenv) Walk(string) int64         { return f.walkSteps }

// steps charges the rest of the steps formula: the whole formula less what
// the entry already charged (its callbacks' 2 steps for each element, or the
// rendering walk).
//
// The sizes are passed by value and evaluated through the run's one fenv,
// so a charge allocates nothing. A formula calls no callback, so one fenv
// serves every call.
func (r *run) libSteps(c *libCall, f fenv, done int64) {
	if q := &c.info.steps; q.kind != 0 {
		r.step(q.eval(c, &f) - done)
		return
	}
	r.fe = f
	r.step(c.e.StepsF.Eval(&r.fe) - done)
}

func (r *run) libUnits(c *libCall, f fenv) {
	if q := &c.info.units; q.kind != 0 {
		r.unit(q.eval(c, &f))
		return
	}
	r.fe = f
	r.unit(c.e.UnitsF.Eval(&r.fe))
}

// quickFormula is a formula of one of the common shapes, evaluated without
// the formula tree, to the same value (04-library.md 4.3).
type quickFormula struct {
	kind   uint8 // 0 for none
	c0, c1 int64
	sym    byte
	arg    int
}

const (
	qConst = 1 + iota
	qSzRecv
	qSzArg
	qLinear // c0 + c1*sym; c0 < 0 when there is no addition
)

var (
	qSzRe  = regexp.MustCompile(`^sz\(([a-z]+)\)$`)
	qLinRe = regexp.MustCompile(`^(?:(\d+) \+ )?(?:(\d+)\*)?([nkv])$`)
)

func newQuick(e *library.Entry, text string, f *library.Formula) quickFormula {
	if f == nil {
		return quickFormula{}
	}
	text = strings.TrimSpace(text)
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		return quickFormula{kind: qConst, c0: n}
	}
	if m := qSzRe.FindStringSubmatch(text); m != nil {
		switch m[1] {
		case "s":
			return quickFormula{kind: qSzRecv}
		case "result", "elements", "keys", "arguments":
			return quickFormula{}
		}
		for i, p := range e.Params {
			if p.Name == m[1] {
				return quickFormula{kind: qSzArg, arg: i}
			}
		}
		return quickFormula{}
	}
	if m := qLinRe.FindStringSubmatch(text); m != nil {
		q := quickFormula{kind: qLinear, c0: -1, c1: 1, sym: m[3][0]}
		if m[1] != "" {
			q.c0, _ = strconv.ParseInt(m[1], 10, 64)
		}
		if m[2] != "" {
			q.c1, _ = strconv.ParseInt(m[2], 10, 64)
		}
		return q
	}
	return quickFormula{}
}

func (q *quickFormula) eval(c *libCall, f *fenv) int64 {
	switch q.kind {
	case qConst:
		return q.c0
	case qSzRecv:
		if s, ok := c.recv.(string); ok {
			return value.Size(s)
		}
		return 0
	case qSzArg:
		if q.arg < len(c.args) {
			if s, ok := c.args[q.arg].(string); ok {
				return value.Size(s)
			}
		}
		return 0
	}
	var y int64
	switch q.sym {
	case 'n':
		y = f.n
	case 'k':
		y = f.k
	default:
		y = f.v
	}
	if q.c1 != 1 {
		if q.c1 != 0 && y > (1<<62)/q.c1 {
			y = 1 << 62
		} else {
			y = q.c1 * y
		}
	}
	if q.c0 < 0 {
		return y
	}
	x := q.c0 + y
	if x > 1<<62 || x < 0 {
		return 1 << 62
	}
	return x
}

// entryInfo is what a call of an entry needs, found once: the
// implementation, the argument tests with the kinds as an array, and the
// formulas that are a constant.
type entryInfo struct {
	impl         libFunc
	params       []paramInfo
	rest         bool // the last parameter takes the rest
	minmax       bool
	steps, units quickFormula
}

type paramInfo struct {
	test  bool
	kinds [value.OpaqueKind + 1]bool
	whole bool
	lits  []string // a literal set (F7 6.2)
}

var infos = func() map[*library.Entry]*entryInfo {
	m := map[*library.Entry]*entryInfo{}
	for _, e := range library.AllEntries {
		m[e] = newInfo(e)
	}
	return m
}()

// infoOf gives the information of an entry.
func infoOf(e *library.Entry) *entryInfo {
	if in, ok := infos[e]; ok {
		return in
	}
	return newInfo(e)
}

func newInfo(e *library.Entry) *entryInfo {
	impl := implemented[Key(e)]
	if impl == nil {
		impl = groupImplemented[e.Name]
	}
	in := &entryInfo{impl: impl, minmax: !e.IsMethod() && (e.Name == "min" || e.Name == "max")}
	for _, p := range e.Params {
		pi := paramInfo{test: p.Callback < 0 && !p.Any, whole: p.Whole, lits: p.Lits}
		for k, ok := range p.Kinds {
			if ok && int(k) < len(pi.kinds) {
				pi.kinds[k] = true
			}
		}
		in.params = append(in.params, pi)
	}
	if n := len(e.Params); n > 0 && e.Params[n-1].Rest {
		in.rest = true
	}
	in.steps = newQuick(e, e.Steps, e.StepsF)
	in.units = newQuick(e, e.Units, e.UnitsF)
	return in
}

func (r *run) charge(c *libCall, f fenv) {
	r.libSteps(c, f, 0)
	r.libUnits(c, f)
}

// testArgs makes the tests of 04-library.md 2.6: each argument left to
// right, the kind before the whole number. They charge nothing.
func (r *run) testArgs(c *libCall) {
	in := c.info
	if in.minmax {
		// The ordered test over the arguments (3.6): no charge.
		r.ordered(c, len(c.args), func(i int) any { return c.args[i] }, false)
		return
	}
	for i, v := range c.args {
		var p *paramInfo
		switch {
		case i < len(in.params):
			p = &in.params[i]
		case in.rest:
			p = &in.params[len(in.params)-1]
		default:
			continue
		}
		if !p.test {
			continue
		}
		k := value.KindOf(v)
		if p.lits != nil { // F7 6.2: a string of the set, else wrong-kind order
			if s, ok := v.(string); !ok || !containsStr(p.lits, s) {
				r.raise(fail("wrong-kind", "order", c.span, messages.Word("name", c.e.Name), messages.Val("found", v)))
			}
			continue
		}
		if !p.kinds[k] {
			r.raise(fail("wrong-kind", "argument", c.span, messages.Word("name", c.e.Name), messages.Val("kind", v)))
		}
		if p.whole && k == value.Number && !whole(v.(float64)) {
			r.raise(fail("wrong-kind", "whole", c.span, messages.Val("found", v)))
		}
	}
}

// ordered tests that a set is all numbers or all strings (3.6).
// With charge, a failure first charges 1 step for each value tested (4.5).
func (r *run) ordered(c *libCall, n int, at func(int) any, charge bool) {
	var first value.Kind
	for i := 0; i < n; i++ {
		v := at(i)
		k := value.KindOf(v)
		bad := k != value.Number && k != value.String
		if i == 0 {
			first = k
		} else if k != first {
			bad = true
		}
		if bad {
			if charge {
				r.step(int64(i + 1))
			}
			r.raise(fail("wrong-kind", "unordered", c.span, messages.Word("name", c.e.Name), messages.Val("kind", v)))
		}
	}
}

// elemFail raises an element test's failure after charging the elements
// tested (04-library.md 4.5).
func (r *run) elemFail(tested int, f *failure) {
	r.step(int64(tested))
	r.raise(f)
}

// callback calls a lambda for one element: 2 steps, the depth test, the
// call (04-library.md 4.4).
// The index i is made a value only when the lambda has a parameter for it.
func (r *run) callback(c *libCall, lam *lambdaCode, e any, i int) any {
	r.step(2)
	c.idx = i
	r.enterCall(c.measure)
	if lam.nparams > 0 {
		lam.bind(r, 0, e, true)
	}
	if lam.nparams > 1 {
		lam.bind(r, 1, numBox(float64(i)), true)
		for k := 2; k < lam.nparams; k++ {
			lam.bind(r, k, nil, false)
		}
	}
	v := lam.body(r)
	r.depth -= c.measure
	return v
}

// callback2 is callback with two values before the index, for reduce.
func (r *run) callback2(c *libCall, lam *lambdaCode, a, e any, i int) any {
	r.step(2)
	c.idx = i
	r.enterCall(c.measure)
	if lam.nparams > 0 {
		lam.bind(r, 0, a, true)
	}
	if lam.nparams > 1 {
		lam.bind(r, 1, e, true)
	}
	if lam.nparams > 2 {
		lam.bind(r, 2, numBox(float64(i)), true)
		for k := 3; k < lam.nparams; k++ {
			lam.bind(r, k, nil, false)
		}
	}
	v := lam.body(r)
	r.depth -= c.measure
	return v
}

func (c *libCall) elem(r *run, i int) any {
	if xs, ok := c.recv.([]any); ok {
		return r.read(xs[i], c.span)
	}
	return r.read(value.At(c.recv, i), c.span)
}

func (c *libCall) n() int { return value.Len(c.recv) }

func (r *run) entryRange(c *libCall) {
	r.raise(fail("number-range", "entry", c.span, messages.Word("name", c.e.Name)))
}

func (r *run) invalid(c *libCall, template string, params ...messages.Param) {
	r.raise(fail("invalid-argument", template, c.span, params...))
}

func strW(vals []any) int64 {
	var w int64
	for _, v := range vals {
		if s, ok := v.(string); ok {
			w += value.Weight(s)
		}
	}
	return w
}

// less is the order of 3.6 on two values of one ordered kind.
func less(a, b any) bool {
	if x, ok := a.(float64); ok {
		return x < b.(float64)
	}
	return a.(string) < b.(string)
}

// stableSort sorts idx by less, stably, with a top-down merge sort: at most
// n × ceil(log2 n) comparisons and as many moves (3.7). Go's sort.Stable
// moves an element up to n × log2(n)² times, which no charge pays for.
func stableSort(idx []int, lessIdx func(a, b int) bool) {
	buf := make([]int, len(idx))
	var rec func(lo, hi int)
	rec = func(lo, hi int) {
		if hi-lo < 2 {
			return
		}
		mid := (lo + hi) / 2
		rec(lo, mid)
		rec(mid, hi)
		if !lessIdx(idx[mid], idx[mid-1]) {
			return
		}
		copy(buf[lo:hi], idx[lo:hi])
		i, j, o := lo, mid, lo
		for i < mid && j < hi {
			if lessIdx(buf[j], buf[i]) {
				idx[o] = buf[j]
				j++
			} else {
				idx[o] = buf[i]
				i++
			}
			o++
		}
		for ; i < mid; i++ {
			idx[o] = buf[i]
			o++
		}
		for ; j < hi; j++ {
			idx[o] = buf[j]
			o++
		}
	}
	rec(0, len(idx))
}

// occurrences finds the occurrences of a non-empty t in s, left to right
// without overlap (3.5), in work linear in len(s): a t longer
// than s is answered from the lengths; a short t is found by the platform's
// search, whose work is at most len(s) × len(t) with len(t) under 64; a long
// t by Knuth-Morris-Pratt.
func occurrences(s, t string, each func(i int)) int {
	if len(t) > len(s) {
		return 0
	}
	count := 0
	if len(t) < 64 {
		for i := 0; i <= len(s)-len(t); {
			j := strings.Index(s[i:], t)
			if j < 0 {
				break
			}
			if each != nil {
				each(i + j)
			}
			count++
			i += j + len(t)
		}
		return count
	}
	failTab := make([]int, len(t))
	for i, k := 1, 0; i < len(t); i++ {
		for k > 0 && t[i] != t[k] {
			k = failTab[k-1]
		}
		if t[i] == t[k] {
			k++
		}
		failTab[i] = k
	}
	for i, k := 0, 0; i < len(s); i++ {
		for k > 0 && s[i] != t[k] {
			k = failTab[k-1]
		}
		if s[i] == t[k] {
			k++
		}
		if k == len(t) {
			if each != nil {
				each(i - len(t) + 1)
			}
			count++
			k = 0
		}
	}
	return count
}

// whiteSpace is the set of 3.9.
func whiteSpace(c rune) bool {
	switch {
	case c >= 0x09 && c <= 0x0D, c == 0x20, c == 0xA0, c == 0x1680,
		c >= 0x2000 && c <= 0x200A, c == 0x2028, c == 0x2029, c == 0x202F,
		c == 0x205F, c == 0x3000, c == 0xFEFF:
		return true
	}
	return false
}

func trimSpace(s string) string { return strings.TrimFunc(s, whiteSpace) }

// isJSONNumber reports whether s is in the grammar of text to number
// (01-language.md 6.3 rule 2).
func isJSONNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	digits := func() int {
		j := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		return i - j
	}
	if i < len(s) && s[i] == '0' {
		i++
	} else if digits() == 0 {
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		if digits() == 0 {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if digits() == 0 {
			return false
		}
	}
	return i == len(s)
}

// readNumber is text to number for int and float (3.10).
func (r *run) readNumber(c *libCall, x string) float64 {
	t := trimSpace(x)
	if !isJSONNumber(t) {
		r.invalid(c, "not-a-number", messages.Val("text", x))
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || math.IsInf(f, 0) {
		r.invalid(c, "not-finite", messages.Val("text", x))
	}
	if f == 0 {
		return 0
	}
	return f
}

// runePrefixW is the weight of the first m code points of s.
func runePrefixW(s string, m int) int64 {
	var w int64
	for _, c := range s {
		if m == 0 {
			break
		}
		w++
		if c > 0xFFFF {
			w++
		}
		m--
	}
	return w
}

// resolve is the rule of positions (3.10).
func resolve(p float64, n int) int {
	if p < 0 {
		p += float64(n)
	}
	if p < 0 {
		return 0
	}
	if p > float64(n) {
		return n
	}
	return int(p)
}

func (c *libCall) sliceBounds(n int) (int, int) {
	start := resolve(c.args[0].(float64), n)
	end := n
	if len(c.args) > 1 {
		end = resolve(c.args[1].(float64), n)
	}
	if start > end {
		end = start
	}
	return start, end
}

// byteIndex returns the byte offset of code point i of s.
func byteIndex(s string, i int) int {
	if i == 0 {
		return 0
	}
	n := 0
	for b := range s {
		if n == i {
			return b
		}
		n++
	}
	return len(s)
}

func satMul(a, b int64) int64 {
	if a != 0 && b > (1<<62)/a {
		return 1 << 62
	}
	return a * b
}

func zero(f float64) float64 {
	if f == 0 {
		return 0
	}
	return f
}

var implemented = map[string]libFunc{
	// 5.1 Conversion, inspection and errors.
	"len": func(r *run, c *libCall) any {
		x := c.args[0]
		r.charge(c, fenv{c: c})
		if s, ok := x.(string); ok {
			return numBox(float64(utf8.RuneCountInString(s)))
		}
		return numBox(float64(value.Len(x)))
	},
	"string": func(r *run, c *libCall) any {
		x := c.args[0]
		f := fenv{c: c}
		if s, ok := x.(string); ok {
			r.charge(c, f)
			return s
		}
		before := r.steps
		b := renderer{r: r, span: c.span, keepTo: renderKeep}
		b.top(x)
		f.walkSteps, f.resultW = r.steps-before, b.weight()
		r.libSteps(c, f, f.walkSteps)
		r.libUnits(c, f)
		return b.text(func(b *renderer) { b.top(x) })
	},
	"int": func(r *run, c *libCall) any {
		r.charge(c, fenv{c: c})
		v, ok := c.args[0].(float64)
		if !ok {
			v = r.readNumber(c, c.args[0].(string))
		}
		t := math.Trunc(v)
		if !whole(t) {
			r.entryRange(c)
		}
		return zero(t)
	},
	"float": func(r *run, c *libCall) any {
		r.charge(c, fenv{c: c})
		if v, ok := c.args[0].(float64); ok {
			return v
		}
		return r.readNumber(c, c.args[0].(string))
	},
	"bool": func(r *run, c *libCall) any { return value.Truthy(c.args[0]) },
	"type": func(r *run, c *libCall) any { return value.KindOf(c.args[0]).String() },
	"fail": func(r *run, c *libCall) any {
		f := &failure{code: "failed", span: c.span, hasSpan: true, msg: c.args[0].(string), ownMsg: true, hasData: true}
		if len(c.args) > 1 {
			f.data = c.args[1]
		}
		r.raise(f)
		return nil
	},

	// 5.2 Numbers.
	"abs": func(r *run, c *libCall) any { r.charge(c, fenv{c: c}); return zero(math.Abs(c.args[0].(float64))) },
	"floor": func(r *run, c *libCall) any {
		return r.toWhole(c, math.Floor(c.args[0].(float64)))
	},
	"ceil": func(r *run, c *libCall) any {
		return r.toWhole(c, math.Ceil(c.args[0].(float64)))
	},
	"round": func(r *run, c *libCall) any {
		return r.toWhole(c, math.Round(c.args[0].(float64)))
	},
	"min": func(r *run, c *libCall) any { return r.pick(c, c.args, true) },
	"max": func(r *run, c *libCall) any { return r.pick(c, c.args, false) },
	"range": func(r *run, c *libCall) any {
		a, b := 0.0, c.args[0].(float64)
		if len(c.args) > 1 {
			a, b = b, c.args[1].(float64)
		}
		k := int64(0)
		if b > a {
			k = int64(b - a)
		}
		r.charge(c, fenv{c: c, k: k})
		out := make([]any, k)
		for i := range out {
			out[i] = zero(a + float64(i))
		}
		return out
	},
	"fixed": func(r *run, c *libCall) any {
		x, d := c.args[0].(float64), c.args[1].(float64)
		if d < 0 || d > 20 {
			r.invalid(c, "bad-digits", messages.Val("digits", d))
		}
		if math.Abs(x) >= 1e21 {
			r.invalid(c, "too-large", messages.Val("x", x))
		}
		r.charge(c, fenv{c: c})
		return fixed(x, int(d))
	},

	// 5.3 Strings.
	"string.toUpperCase": func(r *run, c *libCall) any {
		r.charge(c, fenv{c: c})
		return casemap.Upper(c.recv.(string))
	},
	"string.toLowerCase": func(r *run, c *libCall) any {
		r.charge(c, fenv{c: c})
		return casemap.Lower(c.recv.(string))
	},
	"string.trim": func(r *run, c *libCall) any {
		return r.substring(c, trimSpace(c.recv.(string)))
	},
	"string.removePrefix": func(r *run, c *libCall) any {
		return r.substring(c, strings.TrimPrefix(c.recv.(string), c.args[0].(string)))
	},
	"string.removeSuffix": func(r *run, c *libCall) any {
		return r.substring(c, strings.TrimSuffix(c.recv.(string), c.args[0].(string)))
	},
	"string.startsWith": func(r *run, c *libCall) any {
		r.charge(c, fenv{c: c})
		return strings.HasPrefix(c.recv.(string), c.args[0].(string))
	},
	"string.endsWith": func(r *run, c *libCall) any {
		r.charge(c, fenv{c: c})
		return strings.HasSuffix(c.recv.(string), c.args[0].(string))
	},
	"string.split": func(r *run, c *libCall) any {
		s, sep := c.recv.(string), c.args[0].(string)
		var p int
		if sep == "" {
			p = utf8.RuneCountInString(s)
		} else {
			p = occurrences(s, sep, nil) + 1
		}
		r.charge(c, fenv{c: c, p: int64(p)})
		out := make([]any, 0, p)
		if sep == "" {
			for _, cp := range s {
				out = append(out, string(cp))
			}
			return out
		}
		last := 0
		occurrences(s, sep, func(i int) {
			out = append(out, s[last:i])
			last = i + len(sep)
		})
		return append(out, s[last:])
	},
	"string.replaceAll": func(r *run, c *libCall) any {
		s, old, nw := c.recv.(string), c.args[0].(string), c.args[1].(string)
		if old == "" {
			r.invalid(c, "empty-search")
		}
		n := int64(occurrences(s, old, nil))
		f := fenv{c: c, resultW: value.Weight(s) + n*(value.Weight(nw)-value.Weight(old))}
		r.charge(c, f)
		var b strings.Builder
		b.Grow(len(s) + int(n)*(len(nw)-len(old)))
		last := 0
		occurrences(s, old, func(i int) {
			b.WriteString(s[last:i])
			b.WriteString(nw)
			last = i + len(old)
		})
		b.WriteString(s[last:])
		return b.String()
	},
	"string.slice": func(r *run, c *libCall) any {
		s := c.recv.(string)
		start, end := c.sliceBounds(utf8.RuneCountInString(s))
		bs := byteIndex(s, start)
		be := bs + byteIndex(s[bs:], end-start)
		return r.substring(c, s[bs:be])
	},
	"string.repeat": func(r *run, c *libCall) any {
		s, n := c.recv.(string), c.args[0].(float64)
		if n < 0 {
			r.invalid(c, "negative-count", messages.Val("count", n))
		}
		r.charge(c, fenv{c: c, resultW: satMul(int64(n), value.Weight(s))})
		if s == "" || n == 0 {
			// The count may not fit an int where an int has 32 bits.
			return ""
		}
		if float64(int(n)) != n {
			// A 32-bit build cannot hold the result. Fail as a fault of the
			// implementation rather than repeat a truncated count.
			panic("string.repeat: the count does not fit an int")
		}
		return strings.Repeat(s, int(n))
	},
	"string.padStart": func(r *run, c *libCall) any { return r.pad(c, true) },
	"string.padEnd":   func(r *run, c *libCall) any { return r.pad(c, false) },

	// 5.4 Lists.
	"list.filter": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		// A first capacity of at most 8, a constant, saves the first growths
		// without a size taken from the data.
		out := make([]any, 0, min(n, 8))
		for i := 0; i < n; i++ {
			v, e, boxed := r.callbackAt(c, lam, i)
			if value.Truthy(v) {
				if !boxed {
					e = c.elem(r, i)
				}
				out = append(out, e)
			}
		}
		f := fenv{c: c, n: int64(n), k: int64(len(out))}
		r.libSteps(c, f, 2*int64(n))
		r.libUnits(c, f)
		return out
	},
	"list.map": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		out := make([]any, n)
		for i := 0; i < n; i++ {
			out[i], _, _ = r.callbackAt(c, lam, i)
		}
		f := fenv{c: c, n: int64(n)}
		r.libSteps(c, f, 2*int64(n))
		r.libUnits(c, f)
		return out
	},
	"list.flatMap": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		var parts []any
		k := 0
		for i := 0; i < n; i++ {
			v, _, _ := r.callbackAt(c, lam, i)
			if value.KindOf(v) != value.List {
				r.raise(fail("wrong-kind", "result", c.span, messages.Word("name", c.e.Name), messages.Val("kind", v)))
			}
			parts = append(parts, v)
			k += value.Len(v)
		}
		f := fenv{c: c, n: int64(n), k: int64(k)}
		r.libSteps(c, f, 2*int64(n))
		r.libUnits(c, f)
		out := make([]any, 0, k)
		for _, p := range parts {
			for j := 0; j < value.Len(p); j++ {
				out = append(out, value.At(p, j))
			}
		}
		return out
	},
	"list.some": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		for i := 0; i < n; i++ {
			if v, _, _ := r.callbackAt(c, lam, i); value.Truthy(v) {
				return true
			}
		}
		return false
	},
	"list.every": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		for i := 0; i < n; i++ {
			if v, _, _ := r.callbackAt(c, lam, i); !value.Truthy(v) {
				return false
			}
		}
		return true
	},
	"list.find": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		for i := 0; i < n; i++ {
			v, e, boxed := r.callbackAt(c, lam, i)
			if value.Truthy(v) {
				if !boxed {
					e = c.elem(r, i)
				}
				return e
			}
		}
		return nil
	},
	"list.count": func(r *run, c *libCall) any {
		lam, k, n := c.args[0].(*lambdaCode), 0, c.n()
		for i := 0; i < n; i++ {
			if v, _, _ := r.callbackAt(c, lam, i); value.Truthy(v) {
				k++
			}
		}
		return numBox(float64(k))
	},
	"list.reduce": func(r *run, c *libCall) any {
		lam, acc, n := c.args[0].(*lambdaCode), c.args[1], c.n()
		for i := 0; i < n; i++ {
			e, _ := c.elemArg(r, lam, i)
			acc = r.callback2(c, lam, acc, e, i)
		}
		return acc
	},
	"list.sum": func(r *run, c *libCall) any {
		n := c.n()
		vals := make([]float64, n)
		for i := 0; i < n; i++ {
			e := c.elem(r, i)
			x, ok := e.(float64)
			if !ok {
				r.elemFail(i+1, fail("wrong-kind", "element", c.span, messages.Word("name", c.e.Name), messages.Val("kind", e)))
			}
			vals[i] = x
		}
		r.charge(c, fenv{c: c, n: int64(n)})
		total := 0.0
		for _, x := range vals {
			if whole(total) && whole(x) {
				s := int64(total) + int64(x)
				if s > value.MaxWhole || s < -value.MaxWhole {
					r.entryRange(c)
				}
				total = float64(s)
				continue
			}
			total += x
			if math.IsInf(total, 0) || math.IsNaN(total) {
				r.entryRange(c)
			}
		}
		return zero(total)
	},
	"list.min": func(r *run, c *libCall) any { return r.pickList(c, true) },
	"list.max": func(r *run, c *libCall) any { return r.pickList(c, false) },
	"list.sort": func(r *run, c *libCall) any {
		vals := r.orderedElems(c)
		r.charge(c, fenv{c: c, n: int64(len(vals)), elemsW: strW(vals)})
		idx := make([]int, len(vals))
		for i := range idx {
			idx[i] = i
		}
		stableSort(idx, func(a, b int) bool { return less(vals[a], vals[b]) })
		out := make([]any, len(vals))
		for i, j := range idx {
			out[i] = vals[j]
		}
		return out
	},
	"list.sortBy": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		elems, keys := make([]any, n), make([]any, n)
		for i := 0; i < n; i++ {
			elems[i] = c.elem(r, i)
			keys[i] = r.callback(c, lam, elems[i], i)
		}
		r.ordered(c, n, func(i int) any { return keys[i] }, false)
		f := fenv{c: c, n: int64(n), keysW: strW(keys)}
		r.libSteps(c, f, 2*int64(n))
		r.libUnits(c, f)
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		stableSort(idx, func(a, b int) bool { return less(keys[a], keys[b]) })
		out := make([]any, n)
		for i, j := range idx {
			out[i] = elems[j]
		}
		return out
	},
	"list.groupBy": func(r *run, c *libCall) any {
		lam, n := c.args[0].(*lambdaCode), c.n()
		groups := map[string][]any{}
		var w int64
		for i := 0; i < n; i++ {
			e := c.elem(r, i)
			k := r.callback(c, lam, e, i)
			ks, ok := k.(string)
			if !ok {
				r.raise(fail("wrong-kind", "result", c.span, messages.Word("name", c.e.Name), messages.Val("kind", k)))
			}
			w += value.Weight(ks)
			groups[ks] = append(groups[ks], e)
		}
		f := fenv{c: c, n: int64(n), g: int64(len(groups)), keysW: w}
		r.libSteps(c, f, 2*int64(n))
		r.libUnits(c, f)
		out := make(map[string]any, len(groups))
		for k, v := range groups {
			out[k] = v
		}
		return out
	},
	"list.toMap": func(r *run, c *libCall) any {
		n := c.n()
		keys, vals := make([]string, n), make([]any, n)
		var w int64
		name := messages.Word("name", c.e.Name)
		for i := 0; i < n; i++ {
			e := c.elem(r, i)
			if value.KindOf(e) != value.Map {
				r.elemFail(i+1, fail("wrong-kind", "element", c.span, name, messages.Val("kind", e)))
			}
			k, ok := value.Get(e, "key")
			if !ok {
				r.elemFail(i+1, fail("missing-field", "field", c.span, messages.Word("name", "key")))
			}
			k = r.read(k, c.span)
			ks, ok := k.(string)
			if !ok {
				r.elemFail(i+1, fail("wrong-kind", "key", c.span, messages.Val("found", k)))
			}
			v, ok := value.Get(e, "value")
			if !ok {
				r.elemFail(i+1, fail("missing-field", "field", c.span, messages.Word("name", "value")))
			}
			keys[i], vals[i] = ks, v
			w += value.Weight(ks)
		}
		r.charge(c, fenv{c: c, n: int64(n), keysW: w})
		out := make(map[string]any, n)
		for i, k := range keys {
			out[k] = vals[i]
		}
		return out
	},
	"list.unique": func(r *run, c *libCall) any {
		vals := r.orderedElems(c)
		r.libSteps(c, fenv{c: c, n: int64(len(vals)), elemsW: strW(vals)}, 0)
		seen := make(map[any]bool, len(vals))
		out := []any{}
		for _, v := range vals {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
		r.libUnits(c, fenv{c: c, n: int64(len(vals)), k: int64(len(out))})
		return out
	},
	"list.flat": func(r *run, c *libCall) any {
		n, k := c.n(), 0
		lists := make([]any, n)
		for i := 0; i < n; i++ {
			e := c.elem(r, i)
			if value.KindOf(e) != value.List {
				r.elemFail(i+1, fail("wrong-kind", "element", c.span, messages.Word("name", c.e.Name), messages.Val("kind", e)))
			}
			lists[i] = e
			k += value.Len(e)
		}
		r.charge(c, fenv{c: c, n: int64(n), k: int64(k)})
		out := make([]any, 0, k)
		for _, l := range lists {
			for j := 0; j < value.Len(l); j++ {
				out = append(out, value.At(l, j))
			}
		}
		return out
	},
	"list.reverse": func(r *run, c *libCall) any {
		n := c.n()
		r.charge(c, fenv{c: c, n: int64(n)})
		out := make([]any, n)
		for i := 0; i < n; i++ {
			out[i] = value.At(c.recv, n-1-i)
		}
		return out
	},
	"list.first": func(r *run, c *libCall) any {
		r.charge(c, fenv{c: c, n: int64(c.n())})
		if c.n() == 0 {
			return nil
		}
		return c.elem(r, 0)
	},
	"list.last": func(r *run, c *libCall) any {
		n := c.n()
		r.charge(c, fenv{c: c, n: int64(n)})
		if n == 0 {
			return nil
		}
		return c.elem(r, n-1)
	},
	"list.slice": func(r *run, c *libCall) any {
		n := c.n()
		start, end := c.sliceBounds(n)
		r.charge(c, fenv{c: c, n: int64(n), k: int64(end - start)})
		out := make([]any, end-start)
		for i := range out {
			out[i] = value.At(c.recv, start+i)
		}
		return out
	},
	"list.join": func(r *run, c *libCall) any {
		n, sep := c.n(), c.args[0].(string)
		parts := make([]string, n)
		var w int64
		for i := 0; i < n; i++ {
			e := c.elem(r, i)
			s, ok := e.(string)
			if !ok {
				r.elemFail(i+1, fail("wrong-kind", "element", c.span, messages.Word("name", c.e.Name), messages.Val("kind", e)))
			}
			parts[i] = s
			w += value.Weight(s)
		}
		if n > 1 {
			w += int64(n-1) * value.Weight(sep)
		}
		r.charge(c, fenv{c: c, n: int64(n), resultW: w})
		return strings.Join(parts, sep)
	},

	// 5.5 Maps.
	"map.keys": func(r *run, c *libCall) any {
		keys := r.sortedKeys(c)
		out := make([]any, len(keys))
		for i, k := range keys {
			out[i] = k
		}
		return out
	},
	"map.values": func(r *run, c *libCall) any {
		keys := r.sortedKeys(c)
		out := make([]any, len(keys))
		for i, k := range keys {
			out[i], _ = value.Get(c.recv, k)
		}
		return out
	},
	"map.entries": func(r *run, c *libCall) any {
		keys := r.sortedKeys(c)
		out := make([]any, len(keys))
		for i, k := range keys {
			v, _ := value.Get(c.recv, k)
			out[i] = map[string]any{"key": k, "value": v}
		}
		return out
	},
}

func (r *run) toWhole(c *libCall, f float64) any {
	r.charge(c, fenv{c: c})
	if !whole(f) {
		r.entryRange(c)
	}
	return zero(f)
}

// pick is the functions min and max: the first least or greatest argument.
func (r *run) pick(c *libCall, vals []any, min bool) any {
	r.charge(c, fenv{c: c})
	best := vals[0]
	for _, v := range vals[1:] {
		if (min && less(v, best)) || (!min && less(best, v)) {
			best = v
		}
	}
	return best
}

func (r *run) orderedElems(c *libCall) []any {
	n := c.n()
	vals := make([]any, n)
	for i := 0; i < n; i++ {
		vals[i] = c.elem(r, i)
	}
	r.ordered(c, n, func(i int) any { return vals[i] }, true)
	return vals
}

func (r *run) pickList(c *libCall, min bool) any {
	vals := r.orderedElems(c)
	r.charge(c, fenv{c: c, n: int64(len(vals)), elemsW: strW(vals)})
	if len(vals) == 0 {
		return nil
	}
	best := vals[0]
	for _, v := range vals[1:] {
		if (min && less(v, best)) || (!min && less(best, v)) {
			best = v
		}
	}
	return best
}

func (r *run) substring(c *libCall, s string) any {
	r.charge(c, fenv{c: c, resultW: value.Weight(s)})
	return s
}

func (r *run) pad(c *libCall, start bool) any {
	s := c.recv.(string)
	length := c.args[0].(float64)
	fill := " "
	if len(c.args) > 1 {
		fill = c.args[1].(string)
	}
	n := utf8.RuneCountInString(s)
	if fill == "" || length <= float64(n) {
		return r.substring(c, s)
	}
	m := int64(length) - int64(n)
	fr := int64(utf8.RuneCountInString(fill))
	fw := value.Weight(fill)
	w := value.Weight(s) + satMul(m/fr, fw) + runePrefixW(fill, int(m%fr))
	r.charge(c, fenv{c: c, resultW: w})
	var b strings.Builder
	padding := strings.Repeat(fill, int(m/fr))
	rem := fill[:byteIndex(fill, int(m%fr))]
	if !start {
		b.WriteString(s)
	}
	b.WriteString(padding)
	b.WriteString(rem)
	if start {
		b.WriteString(s)
	}
	return b.String()
}

func (r *run) sortedKeys(c *libCall) []string {
	n := value.Len(c.recv)
	var w int64
	value.ForEachKey(c.recv, func(k string) { w += value.Weight(k) })
	r.charge(c, fenv{c: c, n: int64(n), keysW: w})
	return value.Keys(c.recv)
}

// fixed writes the exact value of x rounded to d digits after the point,
// half away from zero, with no exponent (5.2.8).
func fixed(x float64, d int) string {
	// A double has an exact decimal expansion of at most 1,074 digits after
	// the point; 1,100 digits hold it whole.
	exact := new(big.Float).SetFloat64(math.Abs(x)).Text('f', 1100)
	ip, fp, _ := strings.Cut(exact, ".")
	digits := []byte(ip + fp[:d])
	if fp[d] >= '5' {
		i := len(digits) - 1
		for ; i >= 0; i-- {
			if digits[i] == '9' {
				digits[i] = '0'
				continue
			}
			digits[i]++
			break
		}
		if i < 0 {
			digits = append([]byte{'1'}, digits...)
		}
	}
	intLen := len(digits) - d
	ipart := strings.TrimLeft(string(digits[:intLen]), "0")
	if ipart == "" {
		ipart = "0"
	}
	out := ipart
	if d > 0 {
		out += "." + string(digits[intLen:])
	}
	if x < 0 && strings.Trim(string(digits), "0") != "" {
		out = "-" + out
	}
	return out
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
