package ast

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

func n(t string) Expr               { return &Num{Text: t} }
func id(s string) Expr              { return &Ident{Name: s} }
func bin(op string, x, y Expr) Expr { return &Binary{Op: op, X: x, Y: y} }
func un(op string, x Expr) Expr     { return &Unary{Op: op, X: x} }

func TestPrintPrecedence(t *testing.T) {
	cases := []struct {
		e    Expr
		want string
	}{
		{un("-", bin("**", n("2"), n("2"))), "-(2 ** 2)"},
		{bin("**", un("-", n("2")), n("2")), "(-2) ** 2"},
		{bin("**", n("2"), un("-", n("1"))), "2 ** -1"},
		{bin("**", n("2"), bin("**", n("3"), n("4"))), "2 ** 3 ** 4"},
		{bin("**", bin("**", n("2"), n("3")), n("4")), "(2 ** 3) ** 4"},
		{un("!", un("-", bin("**", id("a"), id("b")))), "!-(a ** b)"},
		{un("-", un("-", id("a"))), "--a"},
		{bin("-", id("a"), bin("-", id("b"), id("c"))), "a - (b - c)"},
		{bin("-", bin("-", id("a"), id("b")), id("c")), "a - b - c"},
		{bin("*", bin("+", id("a"), id("b")), id("c")), "(a + b) * c"},
		{bin("||", id("a"), bin("&&", id("b"), id("c"))), "a || b && c"},
		{bin("&&", bin("||", id("a"), id("b")), id("c")), "(a || b) && c"},
		{bin("??", id("a"), bin("||", id("b"), id("c"))), "a ?? (b || c)"},
		{bin(">", bin("??", id("a"), id("b")), id("c")), "(a ?? b) > c"},
		{bin("??", bin("??", id("a"), id("b")), id("c")), "a ?? b ?? c"},
		{bin("??", id("a"), bin("??", id("b"), id("c"))), "a ?? (b ?? c)"},
		{bin("??", bin("+", id("a"), id("b")), id("c")), "a + b ?? c"},
		{bin("in", un("!", id("a")), id("b")), "(!a) in b"},
		{bin("not in", un("!", id("a")), id("b")), "(!a) not in b"},
		{bin("<", bin("<", id("a"), id("b")), id("c")), "a < b < c"},
		{bin("==", id("a"), bin("==", id("b"), id("c"))), "a == (b == c)"},
		{&Cond{Test: &Cond{Test: id("a"), Then: id("b"), Else: id("c")}, Then: id("d"), Else: id("e")}, "(a ? b : c) ? d : e"},
		{&Cond{Test: id("a"), Then: id("b"), Else: &Cond{Test: id("c"), Then: id("d"), Else: id("e")}}, "a ? b : c ? d : e"},
		{bin("+", &If{Tests: []Expr{id("c")}, Then: []*Branch{{Result: n("1")}}, Else: &Branch{Result: n("2")}}, n("3")), "(if (c) { 1 } else { 2 }) + 3"},
		{&Member{X: bin("+", id("a"), id("b")), Name: "x"}, "(a + b).x"},
		{&Member{X: un("-", id("a")), Name: "x"}, "(-a).x"},
		{un("-", &Member{X: id("a"), Name: "x", Opt: true}), "-a?.x"},
		{&Index{X: &Try{Body: &Branch{Result: id("a")}, Handler: &Branch{Result: id("b")}}, I: n("0"), Opt: true}, "(try { a } catch { b })?.[0]"},
		{&Match{Subject: &Map{Entries: []Entry{{Kind: EntryWord, Key: "a", X: n("1")}}}, Arms: []Arm{{Default: true, Result: n("0")}}}, "match ({a: 1}) { _ => 0 }"},
		{&Match{Subject: &Member{X: &Map{}, Name: "a", Opt: true}, Arms: []Arm{{Lits: []Expr{un("-", n("1")), &Str{Val: "x", Quote: '\''}}, Result: n("0")}}}, "match ({}?.a) { -1 | 'x' => 0 }"},
		{&Call{Name: "f", Args: []Expr{&Lambda{Bare: "x", Body: &Cond{Test: id("x"), Then: n("1"), Else: n("2")}}}}, "f(x => x ? 1 : 2)"},
		{&Template{Parts: []Part{{Text: "a`$"}, {X: &Template{Parts: []Part{{Text: "${"}, {X: id("b")}}}}}}, "`a\\`$${`\\${${b}`}`"},
		{&Str{Val: "a\"'\n😀", Quote: '"', Esc: 2}, `"a\"'\n\uD83D\ude00"`},
		{&Str{Val: "é\u0000", Quote: '\'', Esc: 1}, `'\u{E9}\u0000'`},
		{&Block{Consts: nil, Result: id("a")}, "(a)"},
	}
	for _, c := range cases {
		if got := PrintExpr(c.e); got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}
}

// The round trip: random trees of operators are printed, read back by a
// small parser written from the grammar of 01-language.md 3.1 and 3.4, and
// compared with the tree. A tree that would parse differently, or not at
// all, fails.
func TestPrintRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 20000 {
		e := randTree(r, 5)
		src := PrintExpr(e)
		p := &miniParser{toks: lex(src)}
		got, err := p.expr()
		if err == nil && p.pos != len(p.toks) {
			err = fmt.Errorf("text left at token %d", p.pos)
		}
		if err != nil {
			t.Fatalf("case %d: %s: %v", i, src, err)
		}
		if want := shape(e); shape(got) != want {
			t.Fatalf("case %d: %s parses as %s, want %s", i, src, shape(got), want)
		}
	}
}

var ops = []string{"??", "||", "&&", "==", "!=", "<", ">=", "in", "not in", "+", "-", "*", "/", "%", "**"}

func randTree(r *rand.Rand, d int) Expr {
	if d == 0 || r.IntN(5) == 0 {
		return id(string(rune('a' + r.IntN(5))))
	}
	switch r.IntN(10) {
	case 0, 1:
		return un([]string{"-", "!"}[r.IntN(2)], randTree(r, d-1))
	case 2:
		return &Cond{Test: randTree(r, d-1), Then: randTree(r, d-1), Else: randTree(r, d-1)}
	case 3:
		return &Member{X: randTree(r, d-1), Name: "f", Opt: r.IntN(2) == 0}
	default:
		return bin(ops[r.IntN(len(ops))], randTree(r, d-1), randTree(r, d-1))
	}
}

// shape writes a tree fully parenthesized, without Paren nodes.
func shape(e Expr) string {
	switch e := e.(type) {
	case *Ident:
		return e.Name
	case *Unary:
		return "(" + e.Op + shape(e.X) + ")"
	case *Binary:
		return "(" + shape(e.X) + " " + e.Op + " " + shape(e.Y) + ")"
	case *Cond:
		return "(" + shape(e.Test) + " ? " + shape(e.Then) + " : " + shape(e.Else) + ")"
	case *Member:
		return "(" + shape(e.X) + dot(e.Opt) + e.Name + ")"
	case *Paren:
		return shape(e.X)
	}
	return "?"
}

func lex(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == ' ':
			i++
		case c >= 'a' && c <= 'z':
			j := i
			for j < len(s) && s[j] >= 'a' && s[j] <= 'z' {
				j++
			}
			out = append(out, s[i:j])
			i = j
		default:
			for _, t := range []string{"?.", "??", "||", "&&", "==", "!=", ">=", "**", "?", ":", "(", ")", "<", "+", "-", "*", "/", "%", "!", "."} {
				if strings.HasPrefix(s[i:], t) {
					out = append(out, t)
					i += len(t)
					break
				}
			}
		}
	}
	return out
}

type miniParser struct {
	toks []string
	pos  int
}

func (p *miniParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func (p *miniParser) next() string { t := p.peek(); p.pos++; return t }

// Expr = Binary [ "?" Expr ":" Expr ].
func (p *miniParser) expr() (Expr, error) {
	b, err := p.binary()
	if err != nil {
		return nil, err
	}
	if p.peek() != "?" {
		return b, nil
	}
	p.next()
	a, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.next() != ":" {
		return nil, fmt.Errorf("want :")
	}
	c, err := p.expr()
	if err != nil {
		return nil, err
	}
	return &Cond{Test: b, Then: a, Else: c}, nil
}

// Binary = Or | Coal; Coal = Add "??" Add { "??" Add }.
func (p *miniParser) binary() (Expr, error) {
	a, err := p.level(LevelAdd)
	if err != nil {
		return nil, err
	}
	if p.peek() == "??" {
		for p.peek() == "??" {
			p.next()
			b, err := p.level(LevelAdd)
			if err != nil {
				return nil, err
			}
			a = bin("??", a, b)
		}
		if l := p.opLevel(); l >= LevelOr && l <= LevelRel {
			return nil, fmt.Errorf("?? mixed with %s (3.4 rule 3)", p.peek())
		}
		return a, nil
	}
	return p.climb(a, LevelOr)
}

func (p *miniParser) opLevel() int {
	t := p.peek()
	if t == "not" || t == "in" {
		return LevelRel
	}
	if t == "**" || t == "??" {
		return 0
	}
	return BinaryLevel(t)
}

// climb continues the left operand a with operators of level min and up,
// to LevelMul.
func (p *miniParser) climb(a Expr, min int) (Expr, error) {
	for {
		l := p.opLevel()
		if l < min || l == 0 || l > LevelMul {
			return a, nil
		}
		op := p.next()
		if op == "not" {
			if p.next() != "in" {
				return nil, fmt.Errorf("want in")
			}
			op = "not in"
		}
		if l == LevelRel && (op == "in" || op == "not in") {
			if u, ok := a.(*Unary); ok && u.Op == "!" {
				return nil, fmt.Errorf("! before in (3.4 rule 2)")
			}
		}
		b, err := p.level(l + 1)
		if err != nil {
			return nil, err
		}
		a = bin(op, a, b)
	}
}

// level reads an operand of the given level and up.
func (p *miniParser) level(l int) (Expr, error) {
	if l > LevelMul {
		return p.unary()
	}
	a, err := p.unary()
	if err != nil {
		return nil, err
	}
	return p.climb(a, l)
}

// Unary = ( "!" | "-" ) Unary | Power.
func (p *miniParser) unary() (Expr, error) {
	if t := p.peek(); t == "!" || t == "-" {
		p.next()
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		if b, ok := x.(*Binary); ok && b.Op == "**" {
			return nil, fmt.Errorf("prefix before ** (3.4 rule 1)")
		}
		return un(t, x), nil
	}
	return p.power()
}

// Power = Chain [ "**" Unary ].
func (p *miniParser) power() (Expr, error) {
	c, err := p.chain()
	if err != nil {
		return nil, err
	}
	if p.peek() != "**" {
		return c, nil
	}
	p.next()
	y, err := p.unary()
	if err != nil {
		return nil, err
	}
	return bin("**", c, y), nil
}

func (p *miniParser) chain() (Expr, error) {
	var x Expr
	switch t := p.next(); {
	case t == "(":
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		if p.next() != ")" {
			return nil, fmt.Errorf("want )")
		}
		x = &Paren{X: e}
	case t >= "a" && t <= "z" && t != "in" && t != "not":
		x = id(t)
	default:
		return nil, fmt.Errorf("unexpected %q", t)
	}
	for p.peek() == "." || p.peek() == "?." {
		opt := p.next() == "?."
		x = &Member{X: x, Name: p.next(), Opt: opt}
	}
	return x, nil
}
