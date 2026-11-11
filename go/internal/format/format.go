package format

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
)

// Width is the line width of the formatter (2.1 item 4).
const Width = 80

// Program formats a program text (2.1). It returns the formatted text, or
// the first parse diagnostic.
func Program(source string) (string, *syntax.Error, *syntax.File) {
	f := syntax.NewFile(source)
	prog, err := syntax.Parse(f)
	if err != nil {
		return "", err, f
	}
	toks, lerr := syntax.Lex(f)
	if lerr != nil { // cannot happen: the text parsed
		return "", lerr, f
	}
	out := run(f.Runes(), toks, 0, func(b *builder) part { return b.program(prog) }, Width)
	out = strings.TrimRight(out, " \n") + "\n"
	return out, nil, f
}

// run builds the document twice: first to learn the kept tokens, then with
// the comments attached (2.12), and lays it out.
func run(src []rune, toks []syntax.Token, from int, build func(*builder) part, width int) string {
	b := newBuilder(src, toks, from)
	build(b)
	b.attach()
	b.reset()
	d := build(b)
	for _, c := range b.comments {
		if !c.written {
			panic(fmt.Sprintf("format: a comment was not written: %q", c.text))
		}
	}
	return render(d, width)
}

type builder struct {
	src      []rune
	toks     []syntax.Token
	us       []unit
	starts   []int // the start of each unit, in order
	byStart  map[int]int
	byEnd    map[int]int
	match    map[int]int
	blankBef []int // line ends in the blank space directly before each unit
	comments []*comment
	lead     map[int][]*comment
	trail    map[int][]*comment
	kept     map[int]bool
	slots    map[int]*slot
	pass2    bool
	// first and last are the first and last kept units emitted since the
	// start of the element being built; elemStart marks the next unit as the
	// first of an element other than the first.
	first, last int
	elemStart   bool
}

func newBuilder(src []rune, toks []syntax.Token, from int) *builder {
	b := &builder{src: src, toks: toks, byStart: map[int]int{}, byEnd: map[int]int{}, match: map[int]int{},
		lead: map[int][]*comment{}, trail: map[int][]*comment{}, kept: map[int]bool{}, slots: map[int]*slot{}}
	b.us = units(toks)
	var open []int
	for i, u := range b.us {
		b.starts = append(b.starts, u.start)
		if _, ok := b.byStart[u.start]; !ok || u.kind != syntax.EOF {
			b.byStart[u.start] = i
		}
		if u.end > u.start {
			b.byEnd[u.end] = i
		}
		switch u.kind {
		case syntax.LParen, syntax.LBrack, syntax.LBrace, syntax.OptLBrack:
			open = append(open, i)
		case syntax.RParen, syntax.RBrack, syntax.RBrace:
			if n := len(open); n > 0 {
				b.match[open[n-1]] = i
				b.match[i] = open[n-1]
				open = open[:n-1]
			}
		}
	}
	b.comments = scanComments(src, b.us, from)
	// Blank space and the comments around it.
	b.blankBef = make([]int, len(b.us))
	ci := 0
	prev := from
	for i, u := range b.us {
		for ci < len(b.comments) && b.comments[ci].end <= u.start {
			c := b.comments[ci]
			c.before = lineEndsBetween(src, prev, c.start)
			prev = c.end
			ci++
		}
		b.blankBef[i] = lineEndsBetween(src, prev, u.start)
		prev = u.end
	}
	for k, c := range b.comments {
		next := len(src)
		if k+1 < len(b.comments) && b.comments[k+1].start < b.unitAfter(c.end) {
			next = b.comments[k+1].start
		} else {
			next = b.unitAfter(c.end)
		}
		c.after = lineEndsBetween(src, c.end, next)
	}
	return b
}

// unitAfter is the start of the first unit at or after an offset.
func (b *builder) unitAfter(off int) int {
	i := sort.SearchInts(b.starts, off)
	if i >= len(b.us) {
		return len(b.src)
	}
	return b.us[i].start
}

func (b *builder) reset() {
	b.pass2 = true
	b.slots = map[int]*slot{}
	b.first, b.last, b.elemStart = -1, -1, false
}

// attach attaches each comment to a kept unit (2.12 items 2 and 3).
func (b *builder) attach() {
	var keptIdx []int
	for i := range b.us {
		if b.kept[i] || b.us[i].kind == syntax.EOF {
			keptIdx = append(keptIdx, i)
		}
	}
	eof := len(b.us) - 1
	for _, c := range b.comments {
		// The unit before the comment.
		pi := sort.SearchInts(b.starts, c.start) - 1
		if pi >= 0 && countLineEnds(b.src[b.us[pi].end:c.start]) == 0 {
			// The last kept unit that ends before it: units are in order,
			// and so are their ends.
			k := sort.Search(len(keptIdx), func(k int) bool { return b.us[keptIdx[k]].end > c.start }) - 1
			if k >= 0 && b.us[keptIdx[k]].kind != syntax.EOF {
				j := keptIdx[k]
				c.trailing = true
				b.trail[j] = append(b.trail[j], c)
				continue
			}
		}
		j := eof
		if k := sort.Search(len(keptIdx), func(k int) bool { return b.us[keptIdx[k]].start >= c.end }); k < len(keptIdx) {
			j = keptIdx[k]
		}
		b.lead[j] = append(b.lead[j], c)
	}
}

// at is the unit that starts at an offset.
func (b *builder) at(off int) int {
	i, ok := b.byStart[off]
	if !ok {
		panic(fmt.Sprintf("format: no unit starts at %d", off))
	}
	return i
}

// ending is the unit that ends at an offset.
func (b *builder) ending(off int) int {
	i, ok := b.byEnd[off]
	if !ok {
		panic(fmt.Sprintf("format: no unit ends at %d", off))
	}
	return i
}

// next is the first unit of kind k at or after an offset, past the closing
// parentheses of parenthesized expressions.
func (b *builder) next(off int, k syntax.TokenKind) int {
	i := sort.SearchInts(b.starts, off)
	for i < len(b.us) && b.us[i].kind != k && b.us[i].kind == syntax.RParen {
		i++
	}
	if i >= len(b.us) || b.us[i].kind != k {
		panic(fmt.Sprintf("format: no %s after %d", k, off))
	}
	return i
}

// prev is the last unit of kind k (or k2) that ends at or before an
// offset, past the opening parentheses of parenthesized expressions.
func (b *builder) prev(off int, k, k2 syntax.TokenKind) int {
	i := sort.SearchInts(b.starts, off) - 1
	for i >= 0 && b.us[i].kind != k && b.us[i].kind != k2 && b.us[i].kind == syntax.LParen {
		i--
	}
	if i < 0 || (b.us[i].kind != k && b.us[i].kind != k2) {
		panic(fmt.Sprintf("format: no %s before %d", k, off))
	}
	return i
}

// unitText is the part that writes a unit.
func (b *builder) unitText(i int) part {
	u := b.us[i]
	t := b.toks[u.tok]
	if t.Block {
		return b.textBlock(u)
	}
	s := string(b.src[u.start:u.end])
	if u.kind == syntax.TemplateHead || (u.kind == syntax.TokString && t.Form == '`') {
		s = normalizeLineEnds(s)
	}
	return txt(s)
}

func (b *builder) note(i int) {
	if !b.pass2 {
		b.kept[i] = true
	}
	if b.first < 0 {
		b.first = i
	}
	b.last = i
}

// tok writes a kept unit with its comments.
func (b *builder) tok(i int) part {
	eligible := b.elemStart
	b.elemStart = false
	out := b.leadDoc(i, eligible, false)
	out = append(out, b.unitText(i), b.slotOf(i, false, false))
	b.note(i)
	return out
}

// open writes the opening bracket of a sequence (2.12 item 4).
func (b *builder) open(i int, hasElems bool) part {
	eligible := b.elemStart
	b.elemStart = false
	out := b.leadDoc(i, eligible, false)
	out = append(out, b.unitText(i), b.slotOf(i, true, hasElems))
	b.note(i)
	return out
}

// bare writes a closing bracket whose leading comments closeLead writes.
func (b *builder) bare(i int) part {
	b.elemStart = false
	out := []part{b.unitText(i), b.slotOf(i, false, false)}
	b.note(i)
	return out
}

// closeLead is the leading comments of a closing bracket or of the end of
// the text, written inside the indentation (2.12 item 8).
func (b *builder) closeLead(i int, hasElems bool) part {
	return b.leadDoc(i, hasElems, true)
}

// leadDoc writes leading comments (2.12 items 6 and 7).
func (b *builder) leadDoc(i int, eligible, closer bool) []part {
	if !b.pass2 {
		return nil
	}
	cs := b.lead[i]
	var out []part
	for k, c := range cs {
		if k == 0 && eligible && c.before >= 2 {
			out = append(out, pBlank)
		}
		out = append(out, pHard, txt(c.text))
		c.written = true
		if c.line || c.after > 0 {
			out = append(out, pHard)
		} else if k == len(cs)-1 {
			out = append(out, pSpace)
		}
		if c.after >= 2 && !(closer && k == len(cs)-1) {
			out = append(out, pBlank)
		}
	}
	return out
}

// slotOf is the slot of a unit's trailing comments (2.12 item 4).
func (b *builder) slotOf(i int, opening, hasElems bool) *slot {
	s := &slot{}
	b.slots[i] = s
	if !b.pass2 {
		return s
	}
	var out []part
	for k, c := range b.trail[i] {
		if k == 0 && opening && !c.line {
			out = append(out, txt(c.text))
			if hasElems {
				out = append(out, pSpace)
			}
		} else {
			out = append(out, pSpace, txt(c.text))
		}
		c.written = true
		if c.line {
			out = append(out, pAfter)
		}
	}
	s.d = list(out)
	if len(out) == 0 {
		s.d = nil
	}
	return s
}

// moveTrail moves the trailing comments of a unit to a new slot (2.12 item
// 5).
func (b *builder) moveTrail(i int) part {
	s := b.slots[i]
	if i < 0 || s == nil || s.d == nil {
		return nil
	}
	ns := &slot{d: s.d}
	s.d = nil
	b.slots[i] = ns
	return ns
}

// element builds the document of an element and gives its first and last
// kept units.
func (b *builder) element(notFirst bool, build func() part) (part, int, int) {
	saved := b.first
	b.first = -1
	b.elemStart = notFirst
	d := build()
	b.elemStart = false
	first, last := b.first, b.last
	b.first = saved
	if b.first < 0 {
		b.first = first
	}
	return d, first, last
}

// el is a built element of a sequence.
type el struct {
	d, moved part
	blank    bool
	hug      part // the hugged document of a last argument, or nil
}

// item is an element to build: its source span and its builder.
type item struct {
	start, end int
	build      func() (part, part) // the document, and the hugged one
}

// elements builds the elements of a sequence whose opening unit is open.
// The comma after element k is the first comma after its end.
func (b *builder) elements(open int, items []item) []el {
	out := make([]el, len(items))
	firstSrc := open + 1
	for k, it := range items {
		var hug part
		d, first, last := b.element(k > 0, func() part {
			d, h := it.build()
			hug = h
			return d
		})
		e := el{d: d, hug: hug}
		if k > 0 && b.blankBef[firstSrc] >= 2 && (first < 0 || len(b.lead[first]) == 0) {
			e.blank = true
		}
		e.moved = b.moveTrail(last)
		out[k] = e
		if k+1 < len(items) {
			firstSrc = b.next(it.end, syntax.Comma) + 1
		}
	}
	return out
}

func comma() part { return txt(",") }

// seq is 2.9 item 2.
func seq(open part, els []el, closeLead, close part) part {
	if len(els) == 0 {
		return []part{open, closeLead, close}
	}
	inner := []part{pSoft}
	for k, e := range els {
		if k > 0 {
			inner = append(inner, pLine)
			if e.blank {
				inner = append(inner, &ifBroken{a: pBlank})
			}
		}
		inner = append(inner, e.d)
		if k < len(els)-1 {
			inner = append(inner, comma(), e.moved)
		} else {
			inner = append(inner, &ifBroken{a: comma()}, e.moved, closeLead)
		}
	}
	return grp(open, &indent{d: inner}, pSoft, close)
}

// seqOf builds a whole sequence from its units and items.
func (b *builder) seqOf(open, close int, items []item) part {
	o := b.open(open, len(items) > 0)
	els := b.elements(open, items)
	cl := b.closeLead(close, len(items) > 0)
	return seq(o, els, cl, b.bare(close))
}

func plain(f func() part) func() (part, part) {
	return func() (part, part) { return f(), nil }
}

// body is the const lines and the result of a program, a block or a
// branch, separated by H (2.8, 2.9 item 4). firstResult is the unit that
// begins the result when there are const lines.
func (b *builder) body(consts []*syntax.Const, result syntax.Node, sep atom) []part {
	var out []part
	n := len(consts) + 1
	for k := 0; k < n; k++ {
		var node syntax.Node
		var firstSrc int
		if k < len(consts) {
			node = consts[k]
			firstSrc = b.at(consts[k].Span().Start)
		} else {
			node = result
			if len(consts) > 0 {
				firstSrc = b.ending(consts[len(consts)-1].Span().End) + 1
			}
		}
		d, first, last := b.element(k > 0, func() part { return b.node(node) })
		if k > 0 {
			out = append(out, sep)
			if b.blankBef[firstSrc] >= 2 && (first < 0 || len(b.lead[first]) == 0) {
				out = append(out, pBlank)
			}
		}
		out = append(out, d, b.moveTrail(last))
	}
	return out
}

func (b *builder) program(p *syntax.Program) part {
	out := b.body(p.Consts, p.Result, pHard)
	out = append(out, pHard)
	out = append(out, b.closeLead(len(b.us)-1, true))
	return out
}

// sub is the document of x in its place: in parentheses when 2.6 says.
func (b *builder) sub(parent syntax.Node, r role, x syntax.Node) part {
	if needParens(parent, r, x) {
		return []part{txt("("), b.node(x), txt(")")}
	}
	return b.node(x)
}

// assigned is 2.8 item 2.
func (b *builder) assigned(v syntax.Node) part {
	switch n := v.(type) {
	case *syntax.Binary:
		return grp(ind(pLine, b.binary(n, true)))
	case *syntax.As:
		return grp(ind(pLine, b.node(v)))
	case *syntax.Cond:
		if n.Form == "?:" {
			return grp(ind(pLine, b.node(v)))
		}
	}
	return []part{pSpace, b.node(v)}
}

// node is the document of a node, without parentheses of its own.
func (b *builder) node(n syntax.Node) part {
	switch n := n.(type) {
	case *syntax.Const:
		c := b.at(n.Span().Start)
		eq := b.next(n.Pattern.Span().End, syntax.Assign)
		semi := b.ending(n.Span().End)
		return []part{b.tok(c), pSpace, b.pattern(n.Pattern), pSpace, b.tok(eq), b.assigned(n.Value), b.tok(semi)}
	case *syntax.Block:
		lb, rb := b.at(n.Span().Start), b.ending(n.Span().End)
		o := b.tok(lb)
		bd := b.body(n.Consts, n.Result, pHard)
		cl := b.closeLead(rb, true)
		return []part{o, &indent{d: append(append([]part{pHard}, bd...), cl)}, pHard, b.bare(rb)}
	case *syntax.Ident, *syntax.Number, *syntax.String, *syntax.Template, *syntax.Bool, *syntax.Null:
		return b.tok(b.at(n.Span().Start))
	case *syntax.List:
		lb, rb := b.at(n.Span().Start), b.ending(n.Span().End)
		items := make([]item, len(n.Items))
		for k, x := range n.Items {
			x := x
			items[k] = item{x.Span().Start, x.Span().End, plain(func() part { return b.spreadOr(x) })}
		}
		return b.seqOf(lb, rb, items)
	case *syntax.Map:
		lb, rb := b.at(n.Span().Start), b.ending(n.Span().End)
		items := make([]item, len(n.Entries))
		for k, x := range n.Entries {
			x := x
			items[k] = item{x.Span().Start, x.Span().End, plain(func() part { return b.entry(x) })}
		}
		return b.seqOf(lb, rb, items)
	case *syntax.Unary:
		op := b.tok(b.at(n.Span().Start))
		out := []part{op}
		if u, ok := n.X.(*syntax.Unary); ok && n.Op == "-" && u.Op == "-" && !needParens(n, rOperand, n.X) {
			out = append(out, pSpace)
		}
		return append(out, b.sub(n, rOperand, n.X))
	case *syntax.Binary:
		return b.binary(n, false)
	case *syntax.As:
		x := b.sub(n, rLeft, n.X)
		kw := b.next(n.X.Span().End, syntax.Word)
		return []part{x, pSpace, b.tok(kw), pSpace, b.ptype(n.Type)}
	case *syntax.Cond:
		if n.Form == "if" {
			return b.ifChain(n)
		}
		test := b.sub(n, rTest, n.Test)
		q := b.next(n.Test.Span().End, syntax.Question)
		qd := b.tok(q)
		then := b.node(n.Then)
		c := b.next(n.Then.Span().End, syntax.Colon)
		cd := b.tok(c)
		els := b.node(n.Else)
		return grp(test, ind(pLine, qd, pSpace, then, pLine, cd, pSpace, els))
	case *syntax.Try:
		tt := b.at(n.Span().Start)
		lb := tt + 1
		rb := b.match[lb]
		out := []part{b.tok(tt), pSpace, b.branch(n.Body, lb, rb)}
		catch := rb + 1
		out = append(out, pSpace, b.tok(catch), pSpace)
		lb2 := catch + 1
		if n.Name != "" {
			lp := catch + 1
			out = append(out, b.tok(lp), b.tok(lp+1), b.tok(lp+2), pSpace)
			lb2 = lp + 3
		}
		out = append(out, b.branch(n.Handler, lb2, b.match[lb2]))
		return grp(out...)
	case *syntax.Call:
		name := b.at(n.NameSpan.Start)
		return []part{b.tok(name), b.args(n.Args, name+1, b.ending(n.Span().End))}
	case *syntax.Member, *syntax.Index, *syntax.MethodCall, *syntax.OptChain, *syntax.Each:
		return b.chain(n)
	case *syntax.Lambda:
		d, _ := b.lambda(n)
		return d
	case *syntax.Match:
		return b.match_(n)
	}
	panic(fmt.Sprintf("format: no document for %T", n))
}

func (b *builder) spreadOr(x syntax.Node) part {
	if s, ok := x.(*syntax.Spread); ok {
		return []part{b.tok(b.at(s.Span().Start)), b.node(s.Value)}
	}
	return b.node(x)
}

func (b *builder) entry(x syntax.Node) part {
	switch e := x.(type) {
	case *syntax.Spread:
		return b.spreadOr(e)
	case *syntax.Entry:
		if e.Shorthand {
			return b.tok(b.at(e.Span().Start))
		}
		if e.KeyForm == syntax.KeyComputed {
			lb := b.prev(e.KeyExpr.Span().Start, syntax.LBrack, syntax.LBrack)
			lbd := b.tok(lb)
			k := b.node(e.KeyExpr)
			rb := b.next(e.KeyExpr.Span().End, syntax.RBrack)
			return []part{lbd, k, b.tok(rb), b.tok(rb + 1), b.assigned(e.Value)}
		}
		key := b.at(e.KeySpan.Start)
		return []part{b.tok(key), b.tok(key + 1), b.assigned(e.Value)}
	}
	panic("format: not an entry")
}

// binary is a run of one level (2.8 items 3 and 4).
func (b *builder) binary(n *syntax.Binary, unindented bool) part {
	var nodes []*syntax.Binary // innermost first
	cur := n
	for {
		nodes = append(nodes, cur)
		if cur.Op == "**" {
			break
		}
		l, ok := cur.X.(*syntax.Binary)
		if !ok || l.Op == "**" || binLevel[l.Op] != binLevel[cur.Op] || needParens(cur, rLeft, l) {
			break
		}
		cur = l
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	first := []part{b.sub(nodes[0], rLeft, nodes[0].X), pSpace, b.op(nodes[0])}
	var rest []part
	for k, bn := range nodes {
		rest = append(rest, pLine, b.sub(bn, rRight, bn.Y))
		if k+1 < len(nodes) {
			rest = append(rest, pSpace, b.op(nodes[k+1]))
		}
	}
	if unindented {
		return grp(append(first, rest...)...)
	}
	return grp(append(first, &indent{d: rest})...)
}

func (b *builder) op(n *syntax.Binary) part {
	i := b.at(n.OpPos.Start)
	if n.Op == "not-in" {
		return []part{b.tok(i), pSpace, b.tok(i + 1)}
	}
	return b.tok(i)
}

// test is the test of an if: a run is unindented (2.8 item 4).
func (b *builder) test(x syntax.Node) part {
	if bn, ok := x.(*syntax.Binary); ok {
		return b.binary(bn, true)
	}
	return b.node(x)
}

// branch is the document of a branch whose braces are the units lb and rb.
func (b *builder) branch(content syntax.Node, lb, rb int) part {
	o := b.tok(lb)
	var bd []part
	if blk, ok := content.(*syntax.Block); ok {
		bd = b.body(blk.Consts, blk.Result, pHard)
	} else {
		bd = b.body(nil, content, pHard)
	}
	cl := b.closeLead(rb, true)
	inner := append(append([]part{pLine}, bd...), cl)
	return []part{o, &indent{d: inner}, pLine, b.bare(rb)}
}

func (b *builder) ifChain(n *syntax.Cond) part {
	var out []part
	cur := n
	for {
		it := b.at(cur.Span().Start)
		lp := it + 1
		rp := b.match[lp]
		out = append(out, b.tok(it), pSpace, b.tok(lp), grp(ind(pSoft, b.test(cur.Test)), pSoft), b.tok(rp), pSpace)
		lb := rp + 1
		rb := b.match[lb]
		out = append(out, b.branch(cur.Then, lb, rb))
		els := rb + 1
		out = append(out, pSpace, b.tok(els), pSpace)
		if c2, ok := cur.Else.(*syntax.Cond); ok && c2.Form == "if" {
			cur = c2
			continue
		}
		lb2 := els + 1
		out = append(out, b.branch(cur.Else, lb2, b.match[lb2]))
		break
	}
	return grp(out...)
}

// huggable reports whether a last argument is hugged (2.10 item 1).
func huggable(x syntax.Node) bool {
	switch n := x.(type) {
	case *syntax.Map, *syntax.List, *syntax.Block:
		return true
	case *syntax.Lambda:
		switch m := n.Body.(type) {
		case *syntax.Map, *syntax.List, *syntax.Block, *syntax.Match, *syntax.Try:
			return true
		case *syntax.Cond:
			return m.Form == "if"
		}
	}
	return false
}

// hugOf is the document of x with its outer group broken (2.10 item 2).
func hugOf(x syntax.Node, d part) part {
	switch x.(type) {
	case *syntax.Map, *syntax.List, *syntax.Match, *syntax.Cond, *syntax.Try:
		if g, ok := d.(*group); ok {
			return &group{d: g.d, broken: true}
		}
	}
	return d
}

// args is the arguments of a call (2.10).
func (b *builder) args(args []syntax.Node, lp, rp int) part {
	items := make([]item, len(args))
	for k, a := range args {
		a := a
		items[k] = item{a.Span().Start, a.Span().End, func() (part, part) {
			if l, ok := a.(*syntax.Lambda); ok {
				return b.lambda(l)
			}
			d := b.node(a)
			return d, hugOf(a, d)
		}}
	}
	if len(args) == 0 || !huggable(args[len(args)-1]) {
		return b.seqOf(lp, rp, items)
	}
	o := b.open(lp, true)
	els := b.elements(lp, items)
	cl := b.closeLead(rp, true)
	c := b.bare(rp)
	row := func(hug bool) part {
		out := []part{o}
		for k, e := range els {
			d := e.d
			if hug && k == len(els)-1 {
				d = e.hug
			}
			out = append(out, d)
			if k < len(els)-1 {
				out = append(out, comma(), e.moved, pSpace)
			} else {
				out = append(out, e.moved)
			}
		}
		return append(out, cl, c)
	}
	var breakIf []part
	for _, e := range els[:len(els)-1] {
		breakIf = append(breakIf, e.d)
	}
	return &choice{alts: []part{row(false), row(true), seq(o, els, cl, c)}, breakIf: breakIf}
}

// lambda gives the document of a lambda and its hugged document.
func (b *builder) lambda(n *syntax.Lambda) (part, part) {
	var params part
	var arrow int
	if len(n.Params) == 1 {
		if p, ok := n.Params[0].Pattern.(*syntax.NamePat); ok && n.Params[0].Default == nil {
			params = b.tok(b.at(p.Span().Start))
			arrow = b.next(p.Span().End, syntax.Arrow)
		}
	}
	if params == nil {
		lp := b.at(n.Span().Start)
		rp := b.match[lp]
		arrow = rp + 1
		items := make([]item, len(n.Params))
		for k, p := range n.Params {
			p := p
			items[k] = item{p.Span().Start, p.Span().End, plain(func() part { return b.param(p) })}
		}
		params = b.seqOf(lp, rp, items)
	}
	ar := b.tok(arrow)
	switch n.Body.(type) {
	case *syntax.Binary, *syntax.As:
		return []part{params, pSpace, ar, b.assigned(n.Body)}, nil
	case *syntax.Cond:
		if n.Body.(*syntax.Cond).Form == "?:" {
			return []part{params, pSpace, ar, b.assigned(n.Body)}, nil
		}
	}
	bd := b.node(n.Body)
	d := []part{params, pSpace, ar, pSpace, bd}
	h := []part{params, pSpace, ar, pSpace, hugOf(n.Body, bd)}
	return d, h
}

func (b *builder) param(p *syntax.Param) part {
	d := b.pattern(p.Pattern)
	if p.Default == nil {
		return d
	}
	eq := b.next(p.Pattern.Span().End, syntax.Assign)
	return []part{d, pSpace, b.tok(eq), b.assigned(p.Default)}
}

func (b *builder) withDefault(d part, nameEnd int, def syntax.Node) part {
	if def == nil {
		return d
	}
	eq := b.next(nameEnd, syntax.Assign)
	return []part{d, pSpace, b.tok(eq), b.assigned(def)}
}

func (b *builder) pattern(p syntax.Pattern) part {
	switch p := p.(type) {
	case *syntax.NamePat:
		return b.tok(b.at(p.Span().Start))
	case *syntax.MapPat:
		lb, rb := b.at(p.Span().Start), b.ending(p.Span().End)
		var items []item
		for _, f := range p.Fields {
			f := f
			items = append(items, item{f.Span.Start, f.Span.End, plain(func() part {
				var d part
				if f.KeyForm == syntax.KeyWord && f.Key == f.Name {
					d = b.tok(b.at(f.NameSpan.Start))
				} else {
					k := b.at(f.KeySpan.Start)
					d = []part{b.tok(k), b.tok(k + 1), pSpace, b.tok(b.at(f.NameSpan.Start))}
				}
				return b.withDefault(d, f.NameSpan.End, f.Default)
			})})
		}
		if p.Rest != nil {
			r := p.Rest
			dots := b.at(r.Span().Start) - 1
			items = append(items, item{b.us[dots].start, r.Span().End, plain(func() part {
				return []part{b.tok(dots), b.tok(dots + 1)}
			})})
		}
		return b.seqOf(lb, rb, items)
	case *syntax.ListPat:
		lb, rb := b.at(p.Span().Start), b.ending(p.Span().End)
		var items []item
		for _, e := range p.Elems {
			e := e
			items = append(items, item{e.Span.Start, e.Span.End, plain(func() part {
				return b.withDefault(b.tok(b.at(e.Name.Span().Start)), e.Name.Span().End, e.Default)
			})})
		}
		if p.Rest != nil {
			r := p.Rest
			dots := b.at(r.Span().Start) - 1
			items = append(items, item{b.us[dots].start, r.Span().End, plain(func() part {
				return []part{b.tok(dots), b.tok(dots + 1)}
			})})
		}
		return b.seqOf(lb, rb, items)
	}
	panic("format: unknown pattern")
}

func (b *builder) match_(n *syntax.Match) part {
	mt := b.at(n.Span().Start)
	out := []part{b.tok(mt), pSpace, b.sub(n, rSubject, n.Subject), pSpace}
	lb := b.next(n.Subject.Span().End, syntax.LBrace)
	rb := b.ending(n.Span().End)
	out = append(out, b.tok(lb))
	var inner []part
	firstSrc := lb + 1
	for k, a := range n.Arms {
		a := a
		d, first, last := b.element(k > 0, func() part { return b.arm(a) })
		inner = append(inner, pLine)
		if k > 0 && b.blankBef[firstSrc] >= 2 && (first < 0 || len(b.lead[first]) == 0) {
			inner = append(inner, &ifBroken{a: pBlank})
		}
		moved := b.moveTrail(last)
		inner = append(inner, d)
		if k < len(n.Arms)-1 {
			inner = append(inner, comma(), moved)
			firstSrc = b.next(a.Span().End, syntax.Comma) + 1
		} else {
			inner = append(inner, &ifBroken{a: comma()}, moved, b.closeLead(rb, true))
		}
	}
	out = append(out, &indent{d: inner}, pLine, b.bare(rb))
	return grp(out...)
}

func (b *builder) arm(a *syntax.Arm) part {
	var out []part
	var end int
	if a.Default {
		u := b.at(a.Span().Start)
		out = append(out, b.tok(u))
		end = b.us[u].end
	}
	for k, l := range a.Literals {
		if k > 0 {
			out = append(out, pSpace, b.tok(b.next(end, syntax.Pipe)), pSpace)
		}
		u := b.at(l.Span().Start)
		if b.us[u].kind == syntax.Minus {
			out = append(out, b.tok(u), b.tok(u+1))
		} else {
			out = append(out, b.tok(u))
		}
		end = l.Span().End
	}
	ar := b.next(end, syntax.Arrow)
	return append(out, pSpace, b.tok(ar), b.assigned(a.Result))
}

// chain is 2.11.
func (b *builder) chain(top syntax.Node) part {
	if oc, ok := top.(*syntax.OptChain); ok {
		top = oc.X
	}
	var rev []syntax.Node
	var primary syntax.Node
	var primParent syntax.Node
	cur := top
	for {
		var x syntax.Node
		switch s := cur.(type) {
		case *syntax.Member:
			x = s.X
		case *syntax.Index:
			x = s.X
		case *syntax.MethodCall:
			x = s.X
		case *syntax.Each:
			x = s.X
		}
		if x == nil {
			primary = cur
			break
		}
		rev = append(rev, cur)
		if needParens(cur, rReceiver, x) {
			primary, primParent = x, cur
			break
		}
		switch x.(type) {
		case *syntax.Member, *syntax.Index, *syntax.MethodCall, *syntax.Each:
			cur = x
			continue
		}
		primary, primParent = x, cur
		break
	}
	var pd part
	if primParent != nil {
		pd = b.sub(primParent, rReceiver, primary)
	} else {
		pd = b.node(primary)
	}
	type step struct {
		d     part
		call  bool
		first int
	}
	var steps []step
	var addEach func(e *syntax.Each)
	addReads := func(reads []syntax.Node) {
		for _, r := range reads {
			switch r := r.(type) {
			case *syntax.Member:
				name := b.at(r.NameSpan.Start)
				steps = append(steps, step{[]part{b.tok(name - 1), b.tok(name)}, false, name - 1})
			case *syntax.Index:
				lb := b.prev(r.Index.Span().Start, syntax.LBrack, syntax.OptLBrack)
				lbd := b.tok(lb)
				ix := b.node(r.Index)
				rb := b.ending(r.Span().End)
				steps = append(steps, step{[]part{lbd, ix, b.tok(rb)}, false, lb})
			case *syntax.Each:
				addEach(r)
			}
		}
	}
	addEach = func(e *syntax.Each) {
		rb := b.ending(e.StarSpan.End)
		steps = append(steps, step{[]part{b.tok(rb - 2), b.tok(rb - 1), b.tok(rb)}, false, rb - 2})
		addReads(e.Reads)
	}
	for i := len(rev) - 1; i >= 0; i-- {
		switch s := rev[i].(type) {
		case *syntax.Member:
			name := b.at(s.NameSpan.Start)
			steps = append(steps, step{[]part{b.tok(name - 1), b.tok(name)}, false, name - 1})
		case *syntax.Index:
			lb := b.prev(s.Index.Span().Start, syntax.LBrack, syntax.OptLBrack)
			lbd := b.tok(lb)
			ix := b.node(s.Index)
			rb := b.ending(s.Span().End)
			steps = append(steps, step{[]part{lbd, ix, b.tok(rb)}, false, lb})
		case *syntax.MethodCall:
			name := b.at(s.NameSpan.Start)
			dot := b.tok(name - 1)
			nm := b.tok(name)
			args := b.args(s.Args, name+1, b.ending(s.Span().End))
			steps = append(steps, step{[]part{dot, nm, args}, true, name - 1})
		case *syntax.Each:
			addEach(s)
		}
	}
	recv := []part{pd}
	var links [][]part
	var linkFirst []int
	for _, s := range steps {
		if s.call {
			links = append(links, []part{s.d})
			linkFirst = append(linkFirst, s.first)
			continue
		}
		if len(links) == 0 {
			recv = append(recv, s.d)
		} else {
			links[len(links)-1] = append(links[len(links)-1], s.d)
		}
	}
	if len(links) == 0 {
		return recv
	}
	if len(links) == 1 && len(b.lead[linkFirst[0]]) == 0 {
		return []part{recv, links[0]}
	}
	broken := []part{}
	for _, l := range links {
		broken = append(broken, pSoft, part(l))
	}
	c2 := &group{d: []part{recv, &indent{d: broken}}, broken: true}
	if len(links) == 1 {
		return c2
	}
	for _, f := range linkFirst { // 2.11 item 4
		if len(b.lead[f]) > 0 {
			return c2
		}
	}
	c1 := []part{recv}
	var breakIf []part
	for i, l := range links {
		c1 = append(c1, part(l))
		if i < len(links)-1 {
			breakIf = append(breakIf, part(l))
		}
	}
	return &choice{alts: []part{c1, c2}, breakIf: breakIf}
}

// ptype is a program type, on one line (2.13).
func (b *builder) ptype(t *syntax.PType) part {
	var out []part
	for k, a := range t.Alts {
		if k > 0 {
			pipe := b.ending(t.Alts[k-1].Span.End) + 1
			out = append(out, pSpace, b.tok(pipe), pSpace)
		}
		out = append(out, b.pprim(a.Prim))
		u := b.ending(a.Prim.Span.End)
		for i := 0; i < a.Lists; i++ {
			out = append(out, b.tok(u+1), b.tok(u+2))
			u += 2
		}
	}
	return out
}

func (b *builder) pprim(p *syntax.PPrim) part {
	switch p.Kind {
	case syntax.PWord, syntax.PString, syntax.PName:
		return b.tok(b.at(p.Span.Start))
	case syntax.PMap:
		m := b.at(p.Span.Start)
		return []part{b.tok(m), b.tok(m + 1), b.ptype(p.Elem), b.tok(b.ending(p.Span.End))}
	case syntax.PParen:
		lp := b.at(p.Span.Start)
		return []part{b.tok(lp), b.ptype(p.Elem), b.tok(b.ending(p.Span.End))}
	case syntax.PRecord:
		lb, rb := b.at(p.Span.Start), b.ending(p.Span.End)
		out := []part{b.tok(lb)}
		for k, f := range p.Fields {
			if k > 0 {
				out = append(out, pSpace)
			}
			key := b.at(f.KeySpan.Start)
			out = append(out, b.tok(key))
			c := key + 1
			if f.Optional {
				out = append(out, b.tok(c))
				c++
			}
			out = append(out, b.tok(c), pSpace, b.ptype(f.Type))
			if k < len(p.Fields)-1 || p.HasRest {
				out = append(out, comma(), b.moveTrail(b.last))
			}
		}
		if p.HasRest {
			if len(p.Fields) > 0 {
				out = append(out, pSpace)
			}
			var dots int
			if p.Rest != nil {
				dots = b.at(p.Rest.Span.Start) - 1
			} else {
				dots = rb - 1
				if b.us[dots].kind == syntax.Comma {
					dots--
				}
			}
			out = append(out, b.tok(dots))
			if p.Rest != nil {
				out = append(out, b.ptype(p.Rest))
			}
		}
		return append(out, b.tok(rb))
	}
	panic("format: unknown type")
}
