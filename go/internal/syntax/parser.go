package syntax

import (
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
)

// MaxLevels is the most levels of nesting that may be open at once
// (01-language.md 13.2). A host cannot change it.
const MaxLevels = 100

// Parse divides the text of f into tokens and parses them into a Program
// (01-language.md 3). It returns the first parse diagnostic (14.1) when the
// text does not parse. Parsing stops at that diagnostic.
func Parse(f *File) (prog *Program, err *Error) {
	toks, lerr := Lex(f)
	if lerr != nil {
		return nil, lerr
	}
	p := &parser{toks: toks, tok: toks[0]}
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r)
			}
			prog, err = nil, b.err
		}
	}()
	return p.program(), nil
}

// parser is a hand-written recursive descent parser. Binary operators are
// parsed by precedence climbing (a Pratt parser). Every form that opens no
// level of nesting is parsed by a loop, so that the stack the parser uses is
// bounded by the 100 levels of 13.2.
type parser struct {
	toks    []Token
	i       int
	tok     Token
	prevEnd int
	level   int
}

type bailout struct{ err *Error }

type mark struct{ start, depth int }

func (p *parser) mark() mark { return mark{p.tok.Start, p.level} }

func (p *parser) base(m mark) base {
	return base{span: Span{m.start, p.prevEnd}, depth: m.depth}
}

func (p *parser) next() {
	p.prevEnd = p.tok.End
	if p.i < len(p.toks)-1 {
		p.i++
	}
	p.tok = p.toks[p.i]
}

func (p *parser) peek(k int) Token {
	if p.i+k < len(p.toks) {
		return p.toks[p.i+k]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) fail(code string, start, end int, msg string) {
	panic(bailout{&Error{Code: code, Start: start, End: end, Message: msg}})
}

// unexpected reports the current token as unexpected-token.
func (p *parser) unexpected() {
	t := p.tok
	s, e := t.ErrStart, t.ErrEnd
	// "not in" is one operator: its span runs to the end of "in" (P7).
	if t.isWord("not") && p.peek(1).isWord("in") {
		e = p.peek(1).End
	}
	// 01-language.md 14.3: the word r or R, white space, then a quote.
	if t.Kind == TokString && (t.Form == '"' || t.Form == '\'') && p.i > 0 {
		prev := p.toks[p.i-1]
		if prev.Kind == Word && (prev.Text == "r" || prev.Text == "R") && prev.End < t.Start {
			s, e = prev.Start, prev.End
		}
	}
	p.fail(diag.CodeUnexpectedToken, s, e, "unexpected "+t.describe())
}

func (p *parser) unexpectedAt(t Token) {
	p.fail(diag.CodeUnexpectedToken, t.ErrStart, t.ErrEnd, "unexpected "+t.describe())
}

func (p *parser) reservedWord(t Token) {
	p.fail(diag.CodeReservedWord, t.Start, t.End, "`"+t.Text+"` is a reserved word")
}

func (p *parser) expect(k TokenKind) Token {
	if p.tok.Kind != k {
		p.unexpected()
	}
	t := p.tok
	p.next()
	return t
}

// enter opens a level of nesting at the token that spans start to end.
func (p *parser) enter(start, end int) {
	p.level++
	if p.level > MaxLevels {
		p.fail(diag.CodeNestingExceeded, start, end, "the text is nested more than 100 levels deep")
	}
}

func (p *parser) exit() { p.level-- }

// open checks that the current token is k, consumes it and opens a level.
func (p *parser) open(k TokenKind) Token {
	t := p.expect(k)
	p.enter(t.Start, t.End)
	return t
}

// requireName consumes a Name, or reports reserved-word or unexpected-token.
func (p *parser) requireName() Token {
	t := p.tok
	if t.Kind == Word {
		if IsName(t.Text) {
			p.next()
			return t
		}
		if reserved[t.Text] {
			p.reservedWord(t)
		}
	}
	p.unexpected()
	return t
}

func (p *parser) program() *Program {
	m := p.mark()
	var consts []*Const
	for p.tok.isWord("const") {
		consts = append(consts, p.constLine())
	}
	res := p.expr()
	if p.tok.Kind == Semi {
		p.next()
	}
	if p.tok.Kind != EOF {
		p.unexpected()
	}
	return &Program{base: p.base(m), Consts: consts, Result: res}
}

func (p *parser) constLine() *Const {
	m := p.mark()
	p.next() // const
	pat := p.pattern()
	p.expect(Assign)
	np, ok := pat.(*NamePat)
	val := p.argOrExpr(ok && !np.Discard)
	p.expect(Semi)
	return &Const{base: p.base(m), Pattern: pat, Value: val}
}

// argOrExpr parses the place of an Arg or of a const value (3.2 rule 5).
// allowLambda is false where a lambda is an error at its "=>" (3.4 rule 9).
func (p *parser) argOrExpr(allowLambda bool) Node {
	t := p.tok
	if t.Kind == Word && (IsName(t.Text) || t.Text == "_") && p.peek(1).Kind == Arrow {
		return p.lambda(allowLambda)
	}
	if t.Kind == LParen && p.parenLambda() {
		return p.lambda(allowLambda)
	}
	return p.expr()
}

// parenLambda reports whether the "(" at the current place begins a Lambda:
// the token after its matching ")" is "=>" (3.2 rules 5 and 6). A template is
// one token for this count.
func (p *parser) parenLambda() bool {
	depth := 0
	for j := p.i + 1; j < len(p.toks); j++ {
		switch p.toks[j].Kind {
		case LParen, LBrack, OptLBrack, LBrace:
			depth++
		case TemplateHead:
			d := 1
			for j++; j < len(p.toks) && d > 0; j++ {
				switch p.toks[j].Kind {
				case TemplateHead:
					d++
				case TemplateTail:
					d--
				}
			}
			j--
		case RParen, RBrack, RBrace:
			depth--
			if depth < 0 {
				return p.toks[j].Kind == RParen && j+1 < len(p.toks) && p.toks[j+1].Kind == Arrow
			}
		case TemplateMiddle, TemplateTail, EOF:
			return false
		}
	}
	return false
}

func (p *parser) lambda(allow bool) *Lambda {
	m := p.mark()
	var params []*Param
	if p.tok.Kind == Word {
		t := p.tok
		p.next()
		np := &NamePat{base: base{span: Span{t.Start, t.End}, depth: m.depth}, Name: t.Text, Discard: t.Text == "_"}
		if np.Discard {
			np.Name = ""
		}
		params = []*Param{{base: np.base, Pattern: np}}
	} else {
		p.open(LParen)
		if p.tok.Kind != RParen {
			for {
				pm := p.mark()
				pat := p.pattern()
				var def Node
				if p.tok.Kind == Assign {
					p.next()
					def = p.expr()
				}
				params = append(params, &Param{base: p.base(pm), Pattern: pat, Default: def})
				if p.tok.Kind == Comma {
					p.next()
					if p.tok.Kind == RParen {
						break
					}
					continue
				}
				break
			}
		}
		p.expect(RParen)
		p.exit()
	}
	if p.tok.Kind != Arrow {
		p.unexpected()
	}
	if !allow {
		p.unexpectedAt(p.tok)
	}
	p.next()
	body := p.expr()
	return &Lambda{base: p.base(m), Params: params, Body: body}
}

func (p *parser) pattern() Pattern {
	t := p.tok
	switch t.Kind {
	case Word:
		if t.Text == "_" || IsName(t.Text) {
			p.next()
			np := &NamePat{base: base{span: Span{t.Start, t.End}, depth: p.level}, Name: t.Text}
			if t.Text == "_" {
				np.Name, np.Discard = "", true
			}
			return np
		}
		if reserved[t.Text] {
			p.reservedWord(t)
		}
	case LBrace:
		return p.mapPat()
	case LBrack:
		return p.listPat()
	}
	p.unexpected()
	return nil
}

func (p *parser) namePat(t Token, depth int) *NamePat {
	return &NamePat{base: base{span: Span{t.Start, t.End}, depth: depth}, Name: t.Text}
}

func (p *parser) mapPat() *MapPat {
	m := p.mark()
	p.open(LBrace)
	mp := &MapPat{}
	if p.tok.Kind != RBrace {
		for {
			if p.tok.Kind == Ellipsis {
				p.next()
				d := p.level
				mp.Rest = p.namePat(p.requireName(), d)
				if p.tok.Kind == Comma {
					p.next()
				}
				break
			}
			f := &MapField{}
			fs := p.tok.Start
			switch {
			case p.tok.Kind == Word && p.peek(1).Kind == Colon:
				f.KeyForm, f.Key, f.KeySpan = KeyWord, p.tok.Text, Span{p.tok.Start, p.tok.End}
				p.next()
				p.next()
				nt := p.requireName()
				f.Name, f.NameSpan = nt.Text, Span{nt.Start, nt.End}
			case p.tok.Kind == Word:
				nt := p.requireName()
				f.KeyForm, f.Key, f.Name, f.NameSpan = KeyWord, nt.Text, nt.Text, Span{nt.Start, nt.End}
				f.KeySpan = f.NameSpan
			case p.tok.Kind == TokString && p.tok.Form != '`':
				f.KeyForm, f.Key, f.KeySpan = KeyString, p.tok.Str, Span{p.tok.Start, p.tok.End}
				p.next()
				p.expect(Colon)
				nt := p.requireName()
				f.Name, f.NameSpan = nt.Text, Span{nt.Start, nt.End}
			default:
				p.unexpected()
			}
			if p.tok.Kind == Assign {
				p.next()
				f.Default = p.expr()
			}
			f.Span = Span{fs, p.prevEnd}
			mp.Fields = append(mp.Fields, f)
			if p.tok.Kind == Comma {
				p.next()
				if p.tok.Kind == RBrace {
					break
				}
				continue
			}
			break
		}
	}
	p.expect(RBrace)
	p.exit()
	mp.base = p.base(m)
	return mp
}

func (p *parser) listPat() *ListPat {
	m := p.mark()
	p.open(LBrack)
	lp := &ListPat{}
	if p.tok.Kind != RBrack {
		for {
			if p.tok.Kind == Ellipsis {
				p.next()
				d := p.level
				lp.Rest = p.namePat(p.requireName(), d)
				if p.tok.Kind == Comma {
					p.next()
				}
				break
			}
			e := &ListElem{}
			es := p.tok.Start
			if p.tok.isWord("_") {
				e.Name = &NamePat{base: base{span: Span{p.tok.Start, p.tok.End}, depth: p.level}, Discard: true}
				p.next()
			} else {
				d := p.level
				e.Name = p.namePat(p.requireName(), d)
			}
			if p.tok.Kind == Assign {
				p.next()
				e.Default = p.expr()
			}
			e.Span = Span{es, p.prevEnd}
			lp.Elems = append(lp.Elems, e)
			if p.tok.Kind == Comma {
				p.next()
				if p.tok.Kind == RBrack {
					break
				}
				continue
			}
			break
		}
	}
	p.expect(RBrack)
	p.exit()
	lp.base = p.base(m)
	return lp
}

// expr parses an Expr: an If, a Try, or a Binary with any number of "? :"
// parts. The else part of "? :" is parsed by the loop, not by recursion.
func (p *parser) expr() Node {
	if p.tok.isWord("if") {
		return p.ifExpr()
	}
	if p.tok.isWord("try") {
		return p.tryExpr()
	}
	type pend struct {
		m          mark
		test, then Node
	}
	var pending []pend
	var result Node
	for {
		m := p.mark()
		b := p.binary()
		if p.tok.Kind != Question {
			result = b
			break
		}
		q := p.tok
		p.next()
		p.enter(q.Start, q.End)
		then := p.expr()
		p.expect(Colon)
		p.exit()
		pending = append(pending, pend{m, b, then})
		if p.tok.isWord("if") {
			result = p.ifExpr()
			break
		}
		if p.tok.isWord("try") {
			result = p.tryExpr()
			break
		}
	}
	for i := len(pending) - 1; i >= 0; i-- {
		pd := pending[i]
		result = &Cond{base: p.base(pd.m), Form: "?:", Test: pd.test, Then: pd.then, Else: result}
	}
	return result
}

func (p *parser) ifExpr() Node {
	type br struct {
		m          mark
		test, then Node
	}
	var brs []br
	var final Node
	for {
		m := p.mark()
		p.next() // if
		p.open(LParen)
		test := p.expr()
		p.expect(RParen)
		p.exit()
		then := p.branch()
		brs = append(brs, br{m, test, then})
		if !p.tok.isWord("else") {
			p.unexpected()
		}
		p.next()
		if p.tok.isWord("if") {
			continue
		}
		final = p.branch()
		break
	}
	for i := len(brs) - 1; i >= 0; i-- {
		b := brs[i]
		final = &Cond{base: p.base(b.m), Form: "if", Test: b.test, Then: b.then, Else: final}
	}
	return final
}

// branch parses a Branch: braces around const lines and one expression.
func (p *parser) branch() Node {
	if p.tok.Kind != LBrace {
		p.unexpected()
	}
	m := p.mark()
	lb := p.tok
	p.next()
	p.enter(lb.Start, lb.End)
	switch p.tok.Kind {
	case RBrace, Ellipsis: // 3.4 rules 8 and 7
		p.unexpectedAt(lb)
	}
	var consts []*Const
	for p.tok.isWord("const") {
		consts = append(consts, p.constLine())
	}
	e := p.expr()
	if p.tok.Kind == Colon || p.tok.Kind == Comma { // 3.4 rule 7
		p.unexpectedAt(lb)
	}
	p.expect(RBrace)
	p.exit()
	if len(consts) > 0 {
		return &Block{base: p.base(m), Consts: consts, Result: e}
	}
	return e
}

func (p *parser) tryExpr() Node {
	m := p.mark()
	tt := p.tok
	p.next()
	if p.tok.Kind == LParen { // 3.4 rule 10
		p.unexpectedAt(tt)
	}
	body := p.branch()
	if !p.tok.isWord("catch") {
		p.unexpected()
	}
	p.next()
	t := &Try{Body: body}
	if p.tok.Kind == LParen {
		p.open(LParen)
		nt := p.requireName()
		t.Name, t.NameSpan = nt.Text, Span{nt.Start, nt.End}
		p.expect(RParen)
		p.exit()
	}
	t.Handler = p.branch()
	t.base = p.base(m)
	return t
}

// binary parses a Binary (3.2 rule 4): an Add, then either a Coal or the
// rest of an Or.
func (p *parser) binary() Node {
	m := p.mark()
	left, bang := p.climb(7)
	if p.tok.Kind == Coalesce {
		for p.tok.Kind == Coalesce {
			op := p.tok
			p.next()
			p.enter(op.Start, op.End)
			right, _ := p.climb(7)
			p.exit()
			left = &Binary{base: p.base(m), Op: "??", OpPos: Span{op.Start, op.End}, X: left, Y: right}
		}
		return left
	}
	left, _ = p.climbFrom(m, left, bang, 3)
	return left
}

func (p *parser) climb(min int) (Node, *Token) {
	m := p.mark()
	x, bang := p.unary(nil)
	return p.climbFrom(m, x, bang, min)
}

// binop gives the binary operator at the current place, its level (3.3), its
// span and its count of tokens. "not" not followed by "in" gives "not?".
func (p *parser) binop() (op string, prec, start, end, n int) {
	t := p.tok
	switch t.Kind {
	case OrOr:
		return "||", 3, t.Start, t.End, 1
	case AndAnd:
		return "&&", 4, t.Start, t.End, 1
	case Eq, Ne:
		return t.Kind.String(), 5, t.Start, t.End, 1
	case Lt, Le, Gt, Ge:
		return t.Kind.String(), 6, t.Start, t.End, 1
	case Plus, Minus:
		return t.Kind.String(), 7, t.Start, t.End, 1
	case Star, Slash, Percent:
		return t.Kind.String(), 8, t.Start, t.End, 1
	case Word:
		switch t.Text {
		case "in":
			return "in", 6, t.Start, t.End, 1
		case "not":
			if n := p.peek(1); n.isWord("in") {
				return "not-in", 6, t.Start, n.End, 2
			}
			return "not?", 6, t.Start, t.End, 1
		}
	}
	return "", 0, 0, 0, 0
}

// climbFrom continues a left-associative chain of binary operators of level
// min or tighter, whose first operand is left. bang is the "!" token when
// left is a "!" expression written without parentheses.
func (p *parser) climbFrom(m mark, left Node, bang *Token, min int) (Node, *Token) {
	for {
		if p.tok.isWord("as") || p.tok.isWord("is") { // 14.3: follow-on, at the level of Rel
			if min > 6 {
				return left, bang
			}
			if bang != nil {
				p.unexpectedAt(*bang)
			}
			p.unexpected()
		}
		op, prec, s, e, n := p.binop()
		if op == "" || prec < min {
			return left, bang
		}
		if op == "not?" {
			p.next()
			p.unexpected()
		}
		if (op == "in" || op == "not-in") && bang != nil { // 3.4 rule 2
			p.unexpectedAt(*bang)
		}
		for k := 0; k < n; k++ {
			p.next()
		}
		p.enter(s, e)
		right, _ := p.climb(prec + 1)
		p.exit()
		left = &Binary{base: p.base(m), Op: op, OpPos: Span{s, e}, X: left, Y: right}
		bang = nil
	}
}

// unary parses a Unary. outer is the outermost prefix operator of the run of
// prefix operators this Unary is in, which is the span of 3.4 rules 1 and 2
// (ruling P6). The "!" token it returns is the outermost operator when that
// operator is "!".
func (p *parser) unary(outer *Token) (Node, *Token) {
	if p.tok.Kind == Bang || p.tok.Kind == Minus {
		op := p.tok
		m := p.mark()
		p.next()
		p.enter(op.Start, op.End)
		o := outer
		if o == nil {
			o = &op
		}
		x, _ := p.unary(o)
		p.exit()
		u := &Unary{base: p.base(m), Op: op.Kind.String(), X: x}
		if op.Kind == Bang {
			return u, &op
		}
		return u, nil
	}
	return p.power(outer), nil
}

func (p *parser) power(prefix *Token) Node {
	m := p.mark()
	x := p.chain()
	if p.tok.Kind != Power {
		return x
	}
	if prefix != nil { // 3.4 rule 1
		p.unexpectedAt(*prefix)
	}
	op := p.tok
	p.next()
	p.enter(op.Start, op.End)
	y, _ := p.unary(nil)
	p.exit()
	return &Binary{base: p.base(m), Op: "**", OpPos: Span{op.Start, op.End}, X: x, Y: y}
}

// chain parses a Chain: a primary and its steps, by a loop (7.4).
func (p *parser) chain() Node {
	m := p.mark()
	x := p.primary()
	optional := false
	for {
		switch p.tok.Kind {
		case Dot, OptDot:
			opt := p.tok.Kind == OptDot
			p.next()
			if p.tok.Kind != Word {
				p.unexpected()
			}
			name := p.tok
			p.next()
			if p.tok.Kind == LParen {
				args := p.args()
				x = &MethodCall{base: p.base(m), X: x, Name: name.Text, NameSpan: Span{name.Start, name.End}, Args: args, Optional: opt}
			} else {
				x = &Member{base: p.base(m), X: x, Name: name.Text, NameSpan: Span{name.Start, name.End}, Optional: opt}
			}
			optional = optional || opt
		case LBrack, OptLBrack:
			opt := p.tok.Kind == OptLBrack
			lb := p.tok
			p.next()
			p.enter(lb.Start, lb.End)
			idx := p.expr()
			p.expect(RBrack)
			p.exit()
			x = &Index{base: p.base(m), X: x, Index: idx, Optional: opt}
			optional = optional || opt
		default:
			if optional {
				x = &OptChain{base: p.base(m), X: x}
			}
			return x
		}
	}
}

func (p *parser) args() []Node {
	p.open(LParen)
	var args []Node
	if p.tok.Kind != RParen {
		for {
			args = append(args, p.argOrExpr(true))
			if p.tok.Kind == Comma {
				p.next()
				if p.tok.Kind == RParen {
					break
				}
				continue
			}
			break
		}
	}
	p.expect(RParen)
	p.exit()
	return args
}

func (p *parser) primary() Node {
	m := p.mark()
	t := p.tok
	switch t.Kind {
	case TokNumber:
		p.next()
		return &Number{base: p.base(m), Value: t.Num}
	case TokString:
		p.next()
		return &String{base: p.base(m), Value: t.Str, Form: t.Form}
	case TemplateHead:
		return p.template()
	case LParen:
		p.open(LParen)
		e := p.expr()
		p.expect(RParen)
		p.exit()
		return e
	case LBrack:
		return p.list()
	case LBrace:
		if p.peek(1).isWord("const") && p.peek(2).Kind != Colon { // 3.2 rule 1
			return p.block()
		}
		return p.mapLit()
	case Word:
		switch t.Text {
		case "true", "false":
			p.next()
			return &Bool{base: p.base(m), Value: t.Text == "true"}
		case "null":
			p.next()
			return &Null{base: p.base(m)}
		case "match":
			return p.match()
		case "if", "try": // 3.4 rule 5
			p.unexpected()
		}
		if reserved[t.Text] {
			p.reservedWord(t)
		}
		if t.Text == "_" {
			p.unexpected()
		}
		p.next()
		if p.tok.Kind == LParen {
			args := p.args()
			return &Call{base: p.base(m), Name: t.Text, NameSpan: Span{t.Start, t.End}, Args: args}
		}
		return &Ident{base: p.base(m), Name: t.Text}
	}
	p.unexpected()
	return nil
}

func (p *parser) template() Node {
	m := p.mark()
	head := p.tok
	parts := []TemplatePart{{Text: head.Str}}
	p.next()
	p.enter(head.End-2, head.End)
	for {
		e := p.expr()
		parts = append(parts, TemplatePart{Expr: e})
		t := p.tok
		switch t.Kind {
		case TemplateMiddle:
			p.exit()
			parts = append(parts, TemplatePart{Text: t.Str})
			p.next()
			p.enter(t.End-2, t.End)
		case TemplateTail:
			p.exit()
			parts = append(parts, TemplatePart{Text: t.Str})
			p.next()
			return &Template{base: p.base(m), Parts: parts}
		default:
			p.unexpected()
		}
	}
}

func (p *parser) list() Node {
	m := p.mark()
	p.open(LBrack)
	var items []Node
	if p.tok.Kind != RBrack {
		for {
			if p.tok.Kind == Ellipsis {
				sm := p.mark()
				p.next()
				v := p.expr()
				items = append(items, &Spread{base: p.base(sm), Value: v})
			} else {
				items = append(items, p.expr())
			}
			if p.tok.Kind == Comma {
				p.next()
				if p.tok.Kind == RBrack {
					break
				}
				continue
			}
			break
		}
	}
	p.expect(RBrack)
	p.exit()
	return &List{base: p.base(m), Items: items}
}

func (p *parser) block() Node {
	m := p.mark()
	p.open(LBrace)
	var consts []*Const
	for p.tok.isWord("const") {
		consts = append(consts, p.constLine())
	}
	res := p.expr()
	p.expect(RBrace)
	p.exit()
	return &Block{base: p.base(m), Consts: consts, Result: res}
}

func (p *parser) mapLit() Node {
	m := p.mark()
	p.open(LBrace)
	var entries []Node
	if p.tok.Kind != RBrace {
		for {
			em := p.mark()
			t := p.tok
			switch {
			case t.Kind == Ellipsis:
				p.next()
				v := p.expr()
				entries = append(entries, &Spread{base: p.base(em), Value: v})
			case t.Kind == Word && p.peek(1).Kind == Colon: // 3.2 rule 7
				p.next()
				p.next()
				v := p.expr()
				entries = append(entries, &Entry{base: p.base(em), KeyForm: KeyWord, Key: t.Text, KeySpan: Span{t.Start, t.End}, Value: v})
			case t.Kind == Word:
				nt := p.requireName()
				b := p.base(em)
				entries = append(entries, &Entry{base: b, KeyForm: KeyWord, Key: nt.Text, KeySpan: b.span,
					Value: &Ident{base: b, Name: nt.Text}, Shorthand: true})
			case t.Kind == TokString && t.Form != '`':
				p.next()
				p.expect(Colon)
				v := p.expr()
				entries = append(entries, &Entry{base: p.base(em), KeyForm: KeyString, Key: t.Str, KeySpan: Span{t.Start, t.End}, Value: v})
			case t.Kind == LBrack:
				p.open(LBrack)
				k := p.expr()
				p.expect(RBrack)
				p.exit()
				p.expect(Colon)
				v := p.expr()
				entries = append(entries, &Entry{base: p.base(em), KeyForm: KeyComputed, KeyExpr: k, Value: v})
			default:
				p.unexpected()
			}
			if p.tok.Kind == Comma {
				p.next()
				if p.tok.Kind == RBrace {
					break
				}
				continue
			}
			break
		}
	}
	p.expect(RBrace)
	p.exit()
	return &Map{base: p.base(m), Entries: entries}
}

func (p *parser) match() Node {
	m := p.mark()
	mt := p.tok
	p.next()
	p.enter(mt.Start, mt.End)
	if p.tok.Kind == LBrace || p.tok.isWord("if") || p.tok.isWord("try") { // 3.4 rule 6
		p.unexpected()
	}
	subject := p.expr()
	p.exit()
	p.open(LBrace)
	var arms []*Arm
	for {
		am := p.mark()
		arm := &Arm{}
		if p.tok.isWord("_") {
			arm.Default = true
			p.next()
		} else {
			for {
				arm.Literals = append(arm.Literals, p.literal())
				if p.tok.Kind == Pipe {
					p.next()
					continue
				}
				break
			}
		}
		p.expect(Arrow)
		arm.Result = p.expr()
		arm.base = p.base(am)
		arms = append(arms, arm)
		if p.tok.Kind == Comma {
			p.next()
			if p.tok.Kind == RBrace {
				break
			}
			continue
		}
		break
	}
	p.expect(RBrace)
	p.exit()
	return &Match{base: p.base(m), Subject: subject, Arms: arms}
}

// literal parses a Lit of a match arm.
func (p *parser) literal() Node {
	m := p.mark()
	t := p.tok
	switch {
	case t.Kind == Minus:
		p.next()
		if p.tok.Kind != TokNumber {
			p.unexpected()
		}
		v := -p.tok.Num
		if v == 0 {
			v = 0 // one zero
		}
		p.next()
		return &Number{base: p.base(m), Value: v}
	case t.Kind == TokNumber:
		p.next()
		return &Number{base: p.base(m), Value: t.Num}
	case t.Kind == TokString && t.Form != '`':
		p.next()
		return &String{base: p.base(m), Value: t.Str, Form: t.Form}
	case t.isWord("true") || t.isWord("false"):
		p.next()
		return &Bool{base: p.base(m), Value: t.Text == "true"}
	case t.isWord("null"):
		p.next()
		return &Null{base: p.base(m)}
	case t.Kind == Word && reserved[t.Text]:
		p.reservedWord(t)
	}
	p.unexpected()
	return nil
}
