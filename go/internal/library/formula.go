package library

import (
	"fmt"
	"strconv"
	"strings"
)

// Formula is a steps or units formula of a record, in the grammar of
// 04-library.md 4.3.
type Formula struct {
	text string
	root fnode
}

// Env gives a formula the values of its symbols and names.
type Env interface {
	// Sym is the value of n, k, v, p, g or L.
	Sym(s byte) int64
	// Weight is w(name): the weight of a string, or of the strings among the
	// values a name stands for (elements, keys, arguments), added. 0 for a
	// value that is not a string.
	Weight(name string) int64
	// IsString reports whether the argument of that name is a string.
	IsString(name string) bool
	// Len is len(name): only len(arguments).
	Len(name string) int64
	// Walk is walk(name): what the rendering walk charged.
	Walk(name string) int64
}

// String returns the formula's text.
func (f *Formula) String() string { return f.text }

// Eval computes the formula.
func (f *Formula) Eval(e Env) int64 { return f.root.eval(e) }

type fnode interface{ eval(Env) int64 }

type fnum int64
type fsym byte
type fcall struct{ fn, name string }
type fbin struct {
	op   byte
	x, y fnode
}
type fcond struct {
	name string
	a, b fnode
}

func (n fnum) eval(Env) int64   { return int64(n) }
func (n fsym) eval(e Env) int64 { return e.Sym(byte(n)) }
func (n fcall) eval(e Env) int64 {
	switch n.fn {
	case "sz":
		return e.Weight(n.name) / 64
	case "w":
		return e.Weight(n.name)
	case "len":
		return e.Len(n.name)
	}
	return e.Walk(n.name)
}
func (n fbin) eval(e Env) int64 {
	x, y := n.x.eval(e), n.y.eval(e)
	switch n.op {
	case '+':
		return sat(x + y)
	case '-':
		return x - y
	}
	if x != 0 && y > (1<<62)/x {
		return 1 << 62
	}
	return x * y
}
func (n fcond) eval(e Env) int64 {
	if e.IsString(n.name) {
		return n.a.eval(e)
	}
	return n.b.eval(e)
}

func sat(x int64) int64 {
	if x > 1<<62 || x < 0 {
		return 1 << 62
	}
	return x
}

type fparser struct {
	toks []string
	i    int
}

// ParseFormula parses formula text.
func ParseFormula(text string) (*Formula, error) {
	p := &fparser{toks: tokenize(text)}
	var root fnode
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("formula %q: %v", text, r)
			}
		}()
		if len(p.toks) > 2 && p.toks[1] == "is" {
			name := p.toks[0]
			p.i = 2
			p.expect("string")
			p.expect("?")
			a := p.sum()
			p.expect(":")
			root = fcond{name: name, a: a, b: p.sum()}
		} else {
			root = p.sum()
		}
		if p.i != len(p.toks) {
			panic("text after the formula")
		}
	}()
	if err != nil {
		return nil, err
	}
	return &Formula{text: text, root: root}, nil
}

func tokenize(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ':
			i++
		case strings.IndexByte("+-*():?", c) >= 0:
			out = append(out, string(c))
			i++
		default:
			j := i
			for j < len(s) && strings.IndexByte(" +-*():?", s[j]) < 0 {
				j++
			}
			out = append(out, s[i:j])
			i = j
		}
	}
	return out
}

func (p *fparser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *fparser) expect(t string) {
	if p.peek() != t {
		panic("expected " + t)
	}
	p.i++
}

func (p *fparser) sum() fnode {
	x := p.term()
	for p.peek() == "+" || p.peek() == "-" {
		op := p.peek()[0]
		p.i++
		x = fbin{op: op, x: x, y: p.term()}
	}
	return x
}

func (p *fparser) term() fnode {
	x := p.factor()
	for p.peek() == "*" {
		p.i++
		x = fbin{op: '*', x: x, y: p.factor()}
	}
	return x
}

func (p *fparser) factor() fnode {
	t := p.peek()
	p.i++
	switch {
	case t == "(":
		x := p.sum()
		p.expect(")")
		return x
	case t != "" && t[0] >= '0' && t[0] <= '9':
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			panic(err)
		}
		return fnum(n)
	case len(t) == 1 && strings.Contains("nkvpgLcu", t): // c and u: package F6 3.4
		return fsym(t[0])
	case t == "sz" || t == "w" || t == "len" || t == "walk":
		p.expect("(")
		name := p.peek()
		p.i++
		p.expect(")")
		return fcall{fn: t, name: name}
	}
	panic("unexpected " + t)
}
