// Package format is the formatter of package F12 (spec/packages/
// f12-tools.md, section 2): the canonical text of a program, and of a
// template. It builds a document from the syntax tree and the comments, and
// lays it out by sections 2.4 and 2.5.
package format

import (
	"strings"
)

// A document is a part or a list of parts (2.4). A list is a []part.
type part any

// atom is a part with no content.
type atom uint8

const (
	pSpace atom = iota + 1
	pLine
	pSoft
	pHard
	pBlank
	pAfter // after-comment
)

// text is text(s). nl is set when s holds a line end.
type text struct {
	s  string
	nl bool
	w  int // the width of s, or of s before its first line end
}

func txt(s string) *text {
	t := &text{s: s}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		t.nl = true
		t.w = width(s[:i])
	} else {
		t.w = width(s)
	}
	return t
}

func width(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

type indent struct{ d part }

// group is group(d), or broken-group(d) when broken is set.
type group struct {
	d      part
	broken bool
}

type ifBroken struct{ a, b part }

// choice is choice(a1, ..., an). breakIf holds the parts whose forced
// break makes the choice's rule take the last alternative (2.10 item 3,
// 2.11 item 3); always forces it.
type choice struct {
	alts    []part
	breakIf []part
	always  bool
}

// textBlock is text-block(t): render gives the literal for the new
// indentation J, open is the width of its opening delimiter.
type textBlock struct {
	render func(j int) string
	open   int
}

// slot holds trailing comments of a token; they may be moved after an
// element (2.12 item 5), which empties the slot.
type slot struct{ d part }

func grp(d ...part) *group  { return &group{d: list(d)} }
func ind(d ...part) *indent { return &indent{d: list(d)} }
func list(d []part) part {
	if len(d) == 1 {
		return d[0]
	}
	return d
}

// forcedSet memoizes "holds a forced break" (2.4 item 4).
type forcedSet map[any]bool

func (f forcedSet) forced(d part) bool {
	switch d := d.(type) {
	case nil:
		return false
	case []part:
		for _, x := range d {
			if f.forced(x) {
				return true
			}
		}
		return false
	case atom:
		return d == pHard || d == pBlank || d == pAfter
	case *text, *textBlock:
		return false
	case *indent:
		return f.forced(d.d)
	case *ifBroken:
		return f.forced(d.b)
	case *slot:
		return f.forced(d.d)
	case *group:
		if v, ok := f[d]; ok {
			return v
		}
		v := d.broken || f.forced(d.d)
		f[d] = v
		return v
	case *choice:
		if v, ok := f[d]; ok {
			return v
		}
		v := len(d.alts) > 0 && f.forced(d.alts[0])
		f[d] = v
		return v
	}
	panic("format: unknown part")
}

type cmd struct {
	ind  int
	flat bool
	d    part
}

// layout writes the output of a document (2.5).
type layout struct {
	out    []rune
	width  int
	mark   bool // after-comment
	stack  []cmd
	forced forcedSet
}

func render(d part, width int) string {
	l := &layout{width: width, forced: forcedSet{}}
	l.stack = append(l.stack, cmd{0, false, d})
	l.run()
	return string(l.out)
}

func (l *layout) atLineStart() bool {
	for i := len(l.out) - 1; i >= 0; i-- {
		switch l.out[i] {
		case ' ':
			continue
		case '\n':
			return true
		default:
			return false
		}
	}
	return true
}

func (l *layout) trimSpaces() {
	for len(l.out) > 0 && l.out[len(l.out)-1] == ' ' {
		l.out = l.out[:len(l.out)-1]
	}
}

func (l *layout) spaces(n int) {
	for i := 0; i < n; i++ {
		l.out = append(l.out, ' ')
	}
}

// brk is a break (2.4 item 1).
func (l *layout) brk(ind int) {
	if l.atLineStart() {
		l.trimSpaces()
		l.spaces(ind)
		return
	}
	l.trimSpaces()
	l.out = append(l.out, '\n')
	l.spaces(ind)
}

// empty is an empty line (2.4 item 2).
func (l *layout) empty(ind int) {
	l.trimSpaces()
	if len(l.out) == 0 {
		return
	}
	for !(len(l.out) >= 2 && l.out[len(l.out)-1] == '\n' && l.out[len(l.out)-2] == '\n') {
		l.out = append(l.out, '\n')
	}
	l.spaces(ind)
}

func (l *layout) column() int {
	for i := len(l.out) - 1; i >= 0; i-- {
		if l.out[i] == '\n' {
			return len(l.out) - 1 - i
		}
	}
	return len(l.out)
}

// lead is the count of spaces that begin the output's current line.
func (l *layout) lead() int {
	start := 0
	for i := len(l.out) - 1; i >= 0; i-- {
		if l.out[i] == '\n' {
			start = i + 1
			break
		}
	}
	n := 0
	for i := start; i < len(l.out) && l.out[i] == ' '; i++ {
		n++
	}
	return n
}

// clearMark writes the break of an after-comment mark before a part
// (2.5 item 6). It reports whether the mark was set.
func (l *layout) clearMark(ind int) bool {
	if !l.mark {
		return false
	}
	l.mark = false
	l.brk(ind)
	return true
}

func (l *layout) push(c cmd) { l.stack = append(l.stack, c) }

func (l *layout) run() {
	for len(l.stack) > 0 {
		c := l.stack[len(l.stack)-1]
		l.stack = l.stack[:len(l.stack)-1]
		switch d := c.d.(type) {
		case nil:
		case []part:
			for i := len(d) - 1; i >= 0; i-- {
				l.push(cmd{c.ind, c.flat, d[i]})
			}
		case *text:
			l.clearMark(c.ind)
			l.out = append(l.out, []rune(d.s)...)
		case *textBlock:
			l.clearMark(c.ind)
			l.out = append(l.out, []rune(d.render(l.lead()+2))...)
		case atom:
			switch d {
			case pSpace:
				if !l.clearMark(c.ind) && !l.atLineStart() {
					l.out = append(l.out, ' ')
				}
			case pLine:
				if c.flat {
					if !l.clearMark(c.ind) && !l.atLineStart() {
						l.out = append(l.out, ' ')
					}
				} else {
					l.mark = false
					l.brk(c.ind)
				}
			case pSoft:
				if !c.flat {
					l.mark = false
					l.brk(c.ind)
				}
			case pHard:
				l.mark = false
				l.brk(c.ind)
			case pBlank:
				l.mark = false
				l.empty(c.ind)
			case pAfter:
				l.mark = true
			}
		case *indent:
			l.push(cmd{c.ind + 2, c.flat, d.d})
		case *slot:
			l.push(cmd{c.ind, c.flat, d.d})
		case *ifBroken:
			if c.flat {
				l.push(cmd{c.ind, c.flat, d.b})
			} else {
				l.push(cmd{c.ind, c.flat, d.a})
			}
		case *group:
			flat := false
			switch {
			case d.broken || l.forced.forced(d):
			case c.flat:
				flat = true
			default:
				flat = l.fits(d.d)
			}
			l.push(cmd{c.ind, flat, d.d})
		case *choice:
			if c.flat {
				l.push(cmd{c.ind, true, d.alts[0]})
				break
			}
			must := d.always
			for _, x := range d.breakIf {
				if l.forced.forced(x) {
					must = true
				}
			}
			last := d.alts[len(d.alts)-1]
			if must {
				l.push(cmd{c.ind, false, last})
				break
			}
			done := false
			for _, a := range d.alts[:len(d.alts)-1] {
				if l.fits(a) {
					l.push(cmd{c.ind, true, a})
					done = true
					break
				}
			}
			if !done {
				l.push(cmd{c.ind, false, last})
			}
		default:
			panic("format: unknown part")
		}
	}
}

type fitItem struct {
	d    part
	flat bool
}

// fits is 2.5 item 7: d in mode flat, then the commands below it.
func (l *layout) fits(d part) bool {
	r := l.width - l.column()
	local := []fitItem{{d, true}}
	rest := len(l.stack) - 1
	for {
		if len(local) == 0 {
			if rest < 0 {
				return true
			}
			c := l.stack[rest]
			rest--
			local = append(local, fitItem{c.d, c.flat})
			continue
		}
		it := local[len(local)-1]
		local = local[:len(local)-1]
		switch x := it.d.(type) {
		case nil:
		case []part:
			for i := len(x) - 1; i >= 0; i-- {
				local = append(local, fitItem{x[i], it.flat})
			}
		case *text:
			r -= x.w
			if x.nl {
				return r >= 0
			}
			if r < 0 {
				return false
			}
		case *textBlock:
			r -= x.open
			return r >= 0
		case atom:
			switch x {
			case pSpace:
				r--
				if r < 0 {
					return false
				}
			case pLine:
				if !it.flat {
					return true
				}
				r--
				if r < 0 {
					return false
				}
			case pSoft:
				if !it.flat {
					return true
				}
			case pHard, pBlank, pAfter:
				return true
			}
		case *indent:
			local = append(local, fitItem{x.d, it.flat})
		case *slot:
			local = append(local, fitItem{x.d, it.flat})
		case *ifBroken:
			if it.flat {
				local = append(local, fitItem{x.b, true})
			} else {
				local = append(local, fitItem{x.a, false})
			}
		case *group:
			flat := it.flat
			if x.broken || l.forced.forced(x) {
				flat = false
			}
			local = append(local, fitItem{x.d, flat})
		case *choice:
			if it.flat {
				local = append(local, fitItem{x.alts[0], true})
			} else {
				local = append(local, fitItem{x.alts[len(x.alts)-1], false})
			}
		}
	}
}
