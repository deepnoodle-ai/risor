package syntax

import (
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
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
		return nil, targetLexical(lerr, f.src)
	}
	p := &parser{toks: toks, tok: toks[0], src: f.src}
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r)
			}
			prog, err = nil, p.target(b.err)
		}
	}()
	p.reach()
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

	// What package F8's mapping of a diagnostic reads (f08-errors.md 2.1
	// item 5): the source, the rule a raise names, the frames of the
	// productions open, and marks on tokens. They cost nothing until a
	// diagnostic is made, but for the marks, which are set on the way.
	src     []rune
	lastCtx []frame // the frames of the diagnostic being mapped
	rule    string
	tmpl    string // the template a raise names
	key     string // the key a raise names
	op      string // the as or is of F4 2.5
	q       int    // the ? of a conditional whose : is missing, plus one
	ctx     []frame
	marks   map[int]uint8

	// zone is one more than the index of the first token of the end of a
	// repaired text (package F12 3.3, tools.go), or 0. A ( whose matching
	// ) is not before it begins a parenthesized expression (3.3 item 7).
	zone int
}

// frame is a production open at the failing token, for the mapping.
type frame struct {
	kind  uint8
	open  int  // the index of its opening token
	sep   bool // a map: a :, , or ... met at its level
	entry int  // a map: the index of the first token of the entry being read
}

const (
	fMap       uint8 = iota + 1 // a map literal
	fMapPat                     // a map pattern
	fPattern                    // any pattern
	fMatchCase                  // the start of a match case
	fDefault                    // the default of a pattern, an Expr
)

// Marks on tokens.
const (
	mOperandWord uint8 = 1 << iota // a word read as a Name where a Primary stands
	mPatEnd                        // the last token of the pattern of a Const or a parameter
	mTypeEnd                       // the last token of a program type
	mBranchEnd                     // the last token of an if or a try
	mLambdaArrow                   // the => of a lambda
	mSwitchCall                    // the ) of a Call named switch
	mDefaultEnd                    // the last token of a pattern's default
)

func (p *parser) mark1(i int, m uint8) {
	if p.marks == nil {
		p.marks = map[int]uint8{}
	}
	p.marks[i] |= m
}

func (p *parser) marked(i int, m uint8) bool { return p.marks[i]&m != 0 }

func (p *parser) push(f frame) { p.ctx = append(p.ctx, f) }
func (p *parser) pop()         { p.ctx = p.ctx[:len(p.ctx)-1] }

// raise reports the current token as unexpected-token by a rule of Core.
func (p *parser) raise(rule string) {
	p.rule = rule
	p.unexpected()
}

// raiseAt reports a token as unexpected-token by a rule of Core.
func (p *parser) raiseAt(t Token, rule string) {
	p.rule = rule
	p.unexpectedAt(t)
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
	p.reach()
}

// reach reports a text block's code when the parser reaches its token,
// before anything else is tested of it (F3 6.2).
func (p *parser) reach() {
	if e := p.tok.BlockErr; e != nil {
		panic(bailout{e})
	}
}

func (p *parser) peek(k int) Token {
	if p.i+k < len(p.toks) {
		return p.toks[p.i+k]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) fail(code string, start, end int, msg string) {
	e := &Error{Code: code, Start: start, End: end, Message: msg, Rule: p.rule, at: p.i, Op: p.op, q: p.q, Template: p.tmpl, Key: p.key}
	e.ctx = append([]frame(nil), p.ctx...)
	panic(bailout{e})
}

// unexpected reports the current token as unexpected-token.
func (p *parser) unexpected() {
	t := p.tok
	s, e := t.ErrStart, t.ErrEnd
	// "not in" is one operator: its span runs to the end of "in".
	if t.isWord("not") && p.peek(1).isWord("in") {
		e = p.peek(1).End
	}
	// 01-language.md 14.3: the word r or R, white space, then a quote.
	if t.Kind == TokString && (t.Form == '"' || t.Form == '\'' || t.Form == 'r') && p.i > 0 {
		prev := p.toks[p.i-1]
		if prev.Kind == Word && (prev.Text == "r" || prev.Text == "R") && prev.End < t.Start {
			s, e = prev.Start, prev.End
			p.rule = "14.3-r"
		}
	}
	p.fail(diag.CodeUnexpectedToken, s, e, "unexpected "+t.describe())
}

func (p *parser) unexpectedAt(t Token) {
	p.fail(diag.CodeUnexpectedToken, t.ErrStart, t.ErrEnd, "unexpected "+t.describe())
}

func (p *parser) reservedWord(t Token) {
	if p.rule == "" {
		p.rule = "14.2.1"
	}
	p.fail(diag.CodeReservedWord, t.Start, t.End, "`"+t.Text+"` is a reserved word")
}

// keyColon reads the : after a string key or a computed key. Any other
// token there is unexpected-token with the template key (F8 3.4).
func (p *parser) keyColon() {
	if p.tok.Kind != Colon {
		p.tmpl = "key"
	}
	p.expect(Colon)
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
	p.mark1(p.i-1, mPatEnd)
	if p.tok.Kind == Colon {
		p.raise("E9")
	}
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
		if p.zone > 0 && j >= p.zone-1 {
			return false
		}
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
				p.mark1(p.i-1, mPatEnd)
				if p.tok.Kind == Colon {
					p.raise("E9")
				}
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
	if !allow { // 3.4 rule 9
		p.raiseAt(p.tok, "3.4.9")
	}
	p.mark1(p.i, mLambdaArrow)
	p.next()
	body := p.expr()
	return &Lambda{base: p.base(m), Params: params, Body: body}
}

// patDefault reads the default of a pattern: an Expr inside the pattern,
// so the rows of F8 that read an Expr apply in it (E7).
func (p *parser) patDefault() Node {
	p.push(frame{kind: fDefault})
	defer p.pop()
	d := p.expr()
	p.mark1(p.i-1, mDefaultEnd)
	return d
}

func (p *parser) pattern() Pattern {
	p.push(frame{kind: fPattern})
	defer p.pop()
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
	p.push(frame{kind: fMapPat, open: p.i - 1})
	defer p.pop()
	if p.tok.Kind != RBrace {
		for {
			p.ctx[len(p.ctx)-1].entry = p.i
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
			case p.tok.Kind == TokString && p.tok.Form != '`' && !p.tok.Block:
				f.KeyForm, f.Key, f.KeySpan = KeyString, p.tok.Str, Span{p.tok.Start, p.tok.End}
				p.next()
				p.keyColon()
				nt := p.requireName()
				f.Name, f.NameSpan = nt.Text, Span{nt.Start, nt.End}
			default:
				p.unexpected()
			}
			if p.tok.Kind == Assign {
				p.next()
				f.Default = p.patDefault()
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
				e.Default = p.patDefault()
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
		if p.tok.Kind != Colon {
			p.q = q.Start + 1
			p.raise("cond")
		}
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
		if p.tok.Kind != LParen {
			p.raise("E4")
		}
		p.open(LParen)
		test := p.expr()
		if p.tok.Kind == Comma {
			p.raise("E5")
		}
		p.expect(RParen)
		p.exit()
		if p.tok.Kind != LBrace {
			p.raise("E4")
		}
		then := p.branch()
		brs = append(brs, br{m, test, then})
		if !p.tok.isWord("else") { // 3.4 rule 11
			p.raise("3.4.11")
		}
		p.next()
		if p.tok.isWord("if") {
			continue
		}
		if p.tok.Kind != LBrace {
			p.raise("E4")
		}
		final = p.branch()
		break
	}
	p.mark1(p.i-1, mBranchEnd)
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
	case RBrace: // 3.4 rule 8
		p.raiseAt(lb, "3.4.8")
	case Ellipsis: // 3.4 rule 7
		p.raiseAt(lb, "3.4.7")
	}
	var consts []*Const
	for p.tok.isWord("const") {
		consts = append(consts, p.constLine())
	}
	e := p.expr()
	if p.tok.Kind == Colon || p.tok.Kind == Comma { // 3.4 rule 7
		p.raiseAt(lb, "3.4.7")
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
		p.raiseAt(tt, "3.4.10")
	}
	if p.tok.Kind != LBrace {
		p.raise("E6")
	}
	body := p.branch()
	if !p.tok.isWord("catch") { // 3.4 rule 12
		p.raise("3.4.12-catch")
	}
	p.next()
	t := &Try{Body: body}
	if p.tok.Kind != LParen && p.tok.Kind != LBrace {
		p.raise("3.4.12-form")
	}
	if p.tok.Kind == LParen {
		p.open(LParen)
		if nt := p.tok; nt.Kind != Word || (!IsName(nt.Text) && !reserved[nt.Text]) {
			p.raise("3.4.12-name")
		}
		nt := p.requireName()
		t.Name, t.NameSpan = nt.Text, Span{nt.Start, nt.End}
		if p.tok.Kind != RParen {
			p.raise("3.4.12-name")
		}
		p.expect(RParen)
		p.exit()
		if p.tok.Kind != LBrace {
			p.raise("3.4.12-form")
		}
	}
	t.Handler = p.branch()
	t.base = p.base(m)
	p.mark1(p.i-1, mBranchEnd)
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
		if op, prec, _, _, _ := p.binop(); (op != "" && prec <= 6) || p.tok.isWord("as") || p.tok.isWord("is") {
			p.raise("3.4.3") // 3.4 rule 3
		}
		return left
	}
	left, _ = p.climbFrom(m, left, bang, 3)
	if p.tok.Kind == Coalesce {
		p.raise("3.4.3")
	}
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
		if p.tok.isWord("as") || p.tok.isWord("is") { // F4 2.1: at the level of Rel
			if min > 6 {
				return left, bang
			}
			kw := p.tok
			if bang != nil { // F4 2.5
				p.rule, p.op = "F4.2.5", kw.Text
				p.unexpectedAt(*bang)
			}
			if nt := p.peek(1); kw.Text == "as" && nt.isWord("const") {
				p.raise("E1") // F4 2.6 item 2
			} else if kw.Text == "is" && nt.isWord("not") {
				p.raise("E2")
			}
			p.next()
			p.enter(kw.Start, kw.End)
			t := p.ptype()
			p.exit()
			p.mark1(p.i-1, mTypeEnd)
			left = &As{base: p.base(m), Form: kw.Text, X: left, Type: t}
			bang = nil
			// Only a Rel operator or a looser one may follow a type (2.4).
			if _, prec, _, _, _ := p.binop(); prec > 6 {
				p.unexpected()
			}
			continue
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
			p.raiseAt(*bang, "3.4.2")
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
// prefix operators this Unary is in, which is the span of 3.4 rules 1 and 2. The "!" token it returns is the outermost operator when that
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
		p.raiseAt(*prefix, "3.4.1")
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
	// A projection being read (F7 2.3): its outer Each, the Each of the
	// level being read, and the levels its stars opened.
	var outer, cur *Each
	var eaches []*Each
	stars := 0
	endProjection := func() {
		if outer == nil {
			return
		}
		for _, e := range eaches {
			e.span = Span{m.start, p.prevEnd}
		}
		p.level -= stars
		x, outer, cur, eaches, stars = outer, nil, nil, nil, 0
	}
	for {
		// A star: [ or ?.[ whose next token is * (F7 2.1).
		if (p.tok.Kind == LBrack || p.tok.Kind == OptLBrack) && p.peek(1).Kind == Star {
			opt := p.tok.Kind == OptLBrack
			lb := p.tok
			p.next()
			p.enter(lb.Start, lb.End)
			stars++
			p.next() // *
			if p.tok.Kind != RBrack {
				p.unexpected()
			}
			p.next()
			e := &Each{base: base{depth: m.depth}, Optional: opt, StarSpan: Span{m.start, p.prevEnd}}
			if outer == nil {
				e.X = x
				outer = e
				optional = optional || opt
			} else {
				cur.Reads = append(cur.Reads, e)
			}
			cur = e
			eaches = append(eaches, e)
			continue
		}
		if outer != nil {
			switch p.tok.Kind {
			case Dot, OptDot:
				if p.peek(1).Kind == Word && p.peek(2).Kind != LParen {
					opt := p.tok.Kind == OptDot
					p.next()
					name := p.tok
					p.next()
					cur.Reads = append(cur.Reads, &Member{base: p.base(m), Name: name.Text, NameSpan: Span{name.Start, name.End}, Optional: opt})
					continue
				}
				endProjection() // a method call ends the projection
			case LBrack, OptLBrack:
				opt := p.tok.Kind == OptLBrack
				lb := p.tok
				p.next()
				p.enter(lb.Start, lb.End)
				p.indexStart()
				idx := p.expr()
				if p.tok.Kind == Colon {
					p.raise("E8")
				}
				p.expect(RBrack)
				p.exit()
				cur.Reads = append(cur.Reads, &Index{base: p.base(m), Index: idx, Optional: opt})
				continue
			default:
				endProjection()
			}
		}
		switch p.tok.Kind {
		case Dot, OptDot:
			opt := p.tok.Kind == OptDot
			p.next()
			if p.tok.Kind == Star {
				p.raise("E11")
			}
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
			p.indexStart()
			idx := p.expr()
			if p.tok.Kind == Colon {
				p.raise("E8")
			}
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
		if _, isName := e.(*Ident); isName && p.tok.Kind == Colon && p.toks[p.i-2].Kind == LParen {
			p.raise("E9")
		}
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
			p.raise("3.4.5")
		}
		if reserved[t.Text] {
			if p.peek(1).Kind == Arrow {
				p.rule = "14.2.1" // a parameter
			} else {
				p.rule = "14.2.2"
			}
			p.reservedWord(t)
		}
		if t.Text == "_" {
			p.unexpected()
		}
		p.next()
		if p.tok.Kind == LParen {
			args := p.args()
			if t.Text == "switch" {
				p.mark1(p.i-1, mSwitchCall)
			}
			return &Call{base: p.base(m), Name: t.Text, NameSpan: Span{t.Start, t.End}, Args: args}
		}
		p.mark1(p.i-1, mOperandWord)
		return &Ident{base: p.base(m), Name: t.Text}
	}
	p.rule = "operand"
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
	p.push(frame{kind: fMap, open: p.i - 1})
	defer p.pop()
	fr := func() *frame { return &p.ctx[len(p.ctx)-1] }
	if p.tok.Kind != RBrace {
		for {
			em := p.mark()
			t := p.tok
			fr().entry = p.i
			switch {
			case t.Kind == Ellipsis:
				fr().sep = true
				p.next()
				v := p.expr()
				entries = append(entries, &Spread{base: p.base(em), Value: v})
			case t.Kind == Word && p.peek(1).Kind == Colon: // 3.2 rule 7
				p.next()
				p.next()
				fr().sep = true
				v := p.expr()
				entries = append(entries, &Entry{base: p.base(em), KeyForm: KeyWord, Key: t.Text, KeySpan: Span{t.Start, t.End}, Value: v})
			case t.Kind == Word:
				if reserved[t.Text] {
					p.rule = "shorthand"
				}
				nt := p.requireName()
				b := p.base(em)
				entries = append(entries, &Entry{base: b, KeyForm: KeyWord, Key: nt.Text, KeySpan: b.span,
					Value: &Ident{base: b, Name: nt.Text}, Shorthand: true})
			case t.Kind == TokString && t.Form != '`' && !t.Block:
				p.next()
				p.keyColon()
				fr().sep = true
				v := p.expr()
				entries = append(entries, &Entry{base: p.base(em), KeyForm: KeyString, Key: t.Str, KeySpan: Span{t.Start, t.End}, Value: v})
			case t.Kind == LBrack:
				p.open(LBrack)
				k := p.expr()
				p.expect(RBrack)
				p.exit()
				p.keyColon()
				fr().sep = true
				v := p.expr()
				entries = append(entries, &Entry{base: p.base(em), KeyForm: KeyComputed, KeyExpr: k, Value: v})
			default:
				// The start of an entry is not where an operand must begin
				// (F8 2.2): no row of C8, C9 or E20 applies here.
				p.rule = "entry"
				p.unexpected()
			}
			if p.tok.Kind == Comma {
				fr().sep = true
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
		p.raise("3.4.6")
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
			for first := true; ; first = false {
				if first {
					p.push(frame{kind: fMatchCase, open: p.i})
				}
				arm.Literals = append(arm.Literals, p.literal())
				if first {
					p.pop()
				}
				if p.tok.Kind == Pipe {
					p.next()
					continue
				}
				break
			}
		}
		if p.tok.Kind != Arrow {
			// After a Lit or _, not an Expr: no complete operand (F8 E7).
			p.rule = "arm"
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
	case t.Kind == TokString && t.Form != '`' && !t.Block:
		p.next()
		return &String{base: p.base(m), Value: t.Str, Form: t.Form}
	case t.isWord("true") || t.isWord("false"):
		p.next()
		return &Bool{base: p.base(m), Value: t.Text == "true"}
	case t.isWord("null"):
		p.next()
		return &Null{base: p.base(m)}
	case t.Kind == Word && reserved[t.Text]:
		p.rule = "14.2.2"
		p.reservedWord(t)
	}
	p.unexpected()
	return nil
}

// indexStart makes the tests of the start of an index step (F8 2.7 E11,
// E12): a ] or a ? directly after its [.
func (p *parser) indexStart() {
	switch p.tok.Kind {
	case Colon:
		p.raise("E8")
	case RBrack:
		p.raise("E11")
	case Question:
		p.raise("E12")
	}
}
