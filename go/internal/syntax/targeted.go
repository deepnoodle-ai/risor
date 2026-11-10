package syntax

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
)

// This file is package F8, errors (spec/packages/f08-errors.md 2, 3.4): the
// targeted parse codes, found by mapping the diagnostic Core reports, and the
// message of a parse diagnostic built from the catalog. Detection is Core's;
// nothing here runs unless a diagnostic was made.

// targetLexical maps a lexical diagnostic by the rows A1 to A5 (2.3).
func targetLexical(e *Error, src []rune) *Error {
	e.keepCore()
	if e.Code != diag.CodeUnexpectedCharacter {
		return e
	}
	p := e.Start
	at := func(i int) rune {
		if i >= 0 && i < len(src) {
			return src[i]
		}
		return -1
	}
	c, next := at(p), at(p+1)
	switch {
	case c == '#' && (next == '"' || next == '\'' || (at(p-1) == 'r' && !isWordPart(at(p-2)))):
		e.Code, e.Template = "string-prefix", "guard"
	case c == '#':
		e.Code = "hash-comment"
	case c == '$' && (next == '"' || next == '\'' || next == '`'):
		e.Code, e.Template = "string-prefix", "prefix"
	case c == '$' && next != '{':
		e.Code = "context-symbol"
	case c == '&':
		e.Code, e.Template = "bitwise-operator", "and"
	}
	return e
}

// reservedCodes are B1 to B8 (2.4).
var reservedCodes = map[string]string{
	"let": "let-binding", "var": "let-binding",
	"for": "loop-statement", "while": "loop-statement", "do": "loop-statement",
	"break": "loop-statement", "continue": "loop-statement",
	"return": "return-statement", "function": "function-declaration",
	"throw": "throw-statement", "finally": "finally-clause",
	"import": "import-statement", "nil": "nil-literal",
}

var compound = map[string]bool{"+=": true, "-=": true, "*=": true, "/=": true, "%=": true,
	"**=": true, "??=": true, "||=": true, "&&=": true}

// tokenAt is the index of the token a diagnostic starts at.
func (p *parser) tokenAt(start int) int {
	for j, t := range p.toks {
		if t.ErrStart == start || t.Start == start {
			return j
		}
	}
	return len(p.toks) - 1
}

// operandEnd reports whether a token can end a complete operand.
func operandEnd(t Token) bool {
	switch t.Kind {
	case TokNumber, TokString, RParen, RBrack, RBrace, TemplateTail:
		return true
	case Word:
		return IsName(t.Text) || t.Text == "true" || t.Text == "false" || t.Text == "null" || t.Text == "_"
	}
	return false
}

// target maps a parser's diagnostic by the rows of 2.4 to 2.8, in order.
func (p *parser) target(e *Error) *Error {
	e.keepCore()
	switch e.Code {
	case diag.CodeReservedWord:
		return p.targetReserved(e)
	case diag.CodeUnexpectedToken:
		return p.targetToken(e)
	}
	return e
}

// innermost is the innermost frame of the kinds open at the diagnostic.
func (p *parser) innermost(kinds ...uint8) *frame {
	for i := len(p.lastCtx) - 1; i >= 0; i-- {
		for _, k := range kinds {
			if p.lastCtx[i].kind == k {
				return &p.lastCtx[i]
			}
		}
	}
	return nil
}

// entryKey reports whether the tokens of the entry being read in the
// innermost open map literal end at token k as a key or a shorthand entry:
// a word, a string, or a computed key [e]. Neither is an operand, so a =
// after one is not an assignment (f08-errors.md 1.1, operand word; 2.7, E7
// note 1; 2.1 item 7).
func (p *parser) entryKey(k int) bool {
	if len(p.lastCtx) == 0 {
		return false
	}
	f := &p.lastCtx[len(p.lastCtx)-1]
	if f.kind != fMap || f.entry > k {
		return false
	}
	first := p.toks[f.entry]
	if f.entry == k {
		return first.Kind == Word || first.Kind == TokString
	}
	if first.Kind != LBrack || p.toks[k].Kind != RBrack {
		return false
	}
	depth := 0
	for i := f.entry; i <= k; i++ {
		switch p.toks[i].Kind {
		case LBrack, OptLBrack:
			depth++
		case RBrack:
			depth--
			if depth == 0 && i < k {
				return false
			}
		}
	}
	return depth == 0
}

// lambdaMap reports whether a map's { is the token after the => of a lambda.
func (p *parser) lambdaMap(f *frame) bool {
	return f != nil && f.kind == fMap && f.open > 0 && p.marked(f.open-1, mLambdaArrow)
}

func (p *parser) targetReserved(e *Error) *Error {
	p.lastCtx = e.ctx
	w := string(p.src[e.Start:e.End])
	value := e.Rule == "14.2.2" || e.Rule == "shorthand"
	if value {
		if code, ok := reservedCodes[w]; ok {
			e.Code = code
			return e
		}
	}
	if e.Rule == "shorthand" { // B9: the first entry of a lambda body map
		if f := p.innermost(fMap); p.lambdaMap(f) && f.entry == f.open+1 {
			e.Code = "block-without-const"
			e.Start, e.End = p.toks[f.open].Start, p.toks[f.open].End
			return e
		}
	}
	e.Template = "name"
	if e.Rule == "14.2.2" {
		e.Template = "operand"
	}
	return e
}

func (p *parser) targetToken(e *Error) *Error {
	p.lastCtx = e.ctx
	j := p.tokenAt(e.Start)
	t := p.toks[j]
	var prev, next Token
	if j > 0 {
		prev = p.toks[j-1]
	}
	if j+1 < len(p.toks) {
		next = p.toks[j+1]
	}
	set := func(code, template string) *Error {
		e.Code, e.Template = code, template
		return e
	}
	setSpan := func(s, end int) { e.Start, e.End = s, end }

	// 2.5: tokens that no rule accepts.
	if t.Kind == Illegal && e.Start == t.ErrStart {
		txt := t.Text
		switch {
		case txt == "===":
			return set("strict-equality", "equal")
		case txt == "!==":
			return set("strict-equality", "not-equal")
		case compound[txt]:
			return set("assignment", "compound")
		case txt == "++":
			return set("assignment", "increment")
		case txt == "|>":
			return set("pipe-operator", "")
		case strings.HasPrefix(txt, "'''"):
			return set("text-block-quote", "")
		}
		// A prefixed literal: a word and a quote or a backtick.
		if i := strings.IndexAny(txt, "\"'`"); i > 0 {
			word, q := txt[:i], txt[i]
			if word == "r" && t.Start+4 <= len(p.src) && string(p.src[t.Start+1:t.Start+4]) == "'''" {
				return set("text-block-quote", "")
			}
			if e.Rule == "operand" || prev.Kind == Dot || prev.Kind == OptDot {
				if q == '`' {
					return set("tagged-template", "")
				}
				return set("string-prefix", "prefix")
			}
		}
		return e
	}
	if e.Rule == "14.3-r" {
		return set("string-prefix", "space")
	}
	// 2.6: rules of the grammar.
	switch e.Rule {
	case "3.4.1":
		return set("needs-parentheses", "power")
	case "3.4.2":
		return set("needs-parentheses", "in")
	case "3.4.3":
		return set("needs-parentheses", "coalesce")
	case "3.4.5":
		return set("needs-parentheses", "operand")
	case "3.4.6":
		return set("needs-parentheses", "match-subject")
	case "3.4.7":
		return set("branch-like-map", "")
	case "3.4.8":
		return set("empty-branch", "")
	case "3.4.9":
		return set("lambda-in-pattern", "")
	case "3.4.10":
		return set("try-call", "")
	case "3.4.11":
		return set("missing-else", "")
	case "3.4.12-catch":
		if t.isWord("finally") {
			return set("finally-clause", "")
		}
		return set("missing-catch", "")
	case "3.4.12-form":
		return set("try-form", "")
	case "3.4.12-name":
		return set("catch-name", "")
	case "F4.2.5":
		return set("needs-parentheses", "not-test")
	}
	if e.Rule == "" && j > 0 && p.marked(j-1, mBranchEnd) && continuesOperand(t, next) { // 3.4 rule 4
		return set("needs-parentheses", "after-branch")
	}
	// 2.7: other text, in order.
	switch {
	case e.Rule == "E1":
		return set("as-const", "")
	case e.Rule == "E2":
		return set("negated-is", "")
	case t.isWord("finally"):
		return set("finally-clause", "")
	case e.Rule == "E4":
		return set("if-form", "form")
	case e.Rule == "E5":
		return set("if-form", "call")
	case e.Rule == "E6":
		return set("try-form", "")
	case t.Kind == Assign && j > 0 && operandEnd(prev) && p.innermost(fPattern, fMapPat) == nil && !p.entryKey(j-1):
		return set("assignment", "assign")
	case e.Rule == "E8":
		return set("slice-syntax", "")
	case e.Rule == "E9":
		return set("type-annotation", "")
	case t.Kind == Pipe:
		return set("bitwise-operator", "or")
	case e.Rule == "E11":
		return set("star-form", "")
	case e.Rule == "E12":
		return set("filter-step", "")
	case t.isWord("satisfies"):
		return set("satisfies-operator", "")
	case t.isWord("instanceof"):
		return set("instanceof-operator", "")
	}
	opWord := func(words ...string) bool {
		if j == 0 || prev.Kind != Word || !p.marked(j-1, mOperandWord) {
			return false
		}
		for _, w := range words {
			if prev.Text == w {
				return true
			}
		}
		return false
	}
	switch {
	case opWord("switch", "case"):
		setSpan(prev.Start, prev.End)
		return set("switch-statement", "")
	case t.Kind == LBrace && j > 0 && p.marked(j-1, mSwitchCall):
		// The word switch of the Call that ends at the token before.
		depth := 0
		for k := j - 1; k >= 0; k-- {
			switch p.toks[k].Kind {
			case RParen:
				depth++
			case LParen:
				depth--
			}
			if depth == 0 && k > 0 {
				setSpan(p.toks[k-1].Start, p.toks[k-1].End)
				break
			}
		}
		return set("switch-statement", "")
	case t.isWord("case") && p.innermost(fMatchCase) != nil:
		return set("switch-statement", "")
	case opWord("typeof"):
		setSpan(prev.Start, prev.End)
		return set("typeof-operator", "")
	case opWord("interface", "enum", "type"):
		setSpan(prev.Start, prev.End)
		e.Code, e.Template = "type-declaration", ""
		e.Op = prev.Text
		return e
	case opWord("export"):
		setSpan(prev.Start, prev.End)
		return set("export-statement", "")
	case t.Kind == Lt && e.Rule == "operand" && t.End < len(p.src) && p.src[t.End] == '<':
		return set("heredoc", "")
	case t.Kind == Lt && e.Rule == "operand":
		return set("angle-cast", "")
	case e.Rule == "cond" && p.q > 0 && e.q < len(p.src) && p.src[e.q] == '[':
		setSpan(e.q-1, e.q)
		return set("optional-index", "")
	}
	if t.Kind == Minus {
		if f := p.innermost(fMap, fMapPat); f != nil {
			if k, end, ok := p.hyphenKey(f.entry); ok {
				w := p.toks[f.entry]
				e.Key = k
				setSpan(w.Start, end)
				return set("unquoted-key", "")
			}
		}
	}
	// E23, but not for a = directly after a string key or a computed key,
	// which is a map literal with the wrong separator.
	if f := p.innermost(fMap); p.lambdaMap(f) && !f.sep && !(t.Kind == Assign && p.entryKey(j-1) && prev.Kind != Word) {
		setSpan(p.toks[f.open].Start, p.toks[f.open].End)
		return set("block-without-const", "")
	}
	if j > 0 && p.marked(j-1, mTypeEnd) {
		switch t.Kind {
		case Plus, Minus, Star, Slash, Percent, Power, Dot, OptDot, OptLBrack, LBrack, LParen, LBrace:
			return set("needs-parentheses", "after-type")
		}
	}
	if t.Kind == Bang && j > 0 && operandEnd(prev) && next.isWord("is") {
		return set("negated-is", "")
	}
	return e
}

// hyphenKey reads the key of E22 from the first token of an entry: a word,
// then pairs of - and a word or a number, each directly after the one
// before, then :.
func (p *parser) hyphenKey(i int) (key string, end int, ok bool) {
	if i >= len(p.toks) || p.toks[i].Kind != Word {
		return "", 0, false
	}
	last := p.toks[i]
	k := i + 1
	pairs := 0
	for k+1 < len(p.toks) && p.toks[k].Kind == Minus && p.toks[k].Start == last.End &&
		(p.toks[k+1].Kind == Word || p.toks[k+1].Kind == TokNumber) && p.toks[k+1].Start == p.toks[k].End {
		last = p.toks[k+1]
		k += 2
		pairs++
	}
	if pairs == 0 || k >= len(p.toks) || p.toks[k].Kind != Colon {
		return "", 0, false
	}
	return string(p.src[p.toks[i].Start:last.End]), last.End, true
}

// visible writes a text with its control characters escaped (1.1).
func visible(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || (r >= 0x7F && r <= 0x9F):
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Visible is visible text (f08-errors.md 1.1), for the check phase.
func Visible(s string) string { return visible(s) }

// tokenText is the rendering of the kind token (3.2).
func tokenText(src []rune, s, e int) string {
	if s >= e || s >= len(src) {
		return "end of input"
	}
	if e > len(src) {
		e = len(src)
	}
	t := string(src[s:e])
	if strings.HasPrefix(t, "not") && strings.HasSuffix(t, "in") && len(t) > 5 && !isWordPart(src[s+3]) {
		return "not in"
	}
	return visible(t)
}

// charText is the rendering of the kind char (3.2).
func charText(r rune) string {
	s := fmt.Sprintf("U+%04X", r)
	if r >= 0x21 && r <= 0x7E {
		s += " (" + string(r) + ")"
	}
	return s
}

// templateOf chooses the template of a Core code that has several (3.4).
func templateOf(e *Error, src []rune) string {
	if e.Template != "" {
		return e.Template
	}
	switch e.Code {
	case diag.CodeUnterminatedString:
		switch {
		case strings.Contains(e.Message, "template"):
			return "template"
		case strings.Contains(e.Message, "text block"):
			return "text-block"
		case strings.Contains(e.Message, "part"):
			return "part"
		}
		return "string"
	case diag.CodeBadEscape:
		switch {
		case strings.Contains(e.Message, "surrogate"):
			return "surrogate"
		case strings.Contains(e.Message, "names no code point"):
			return "range"
		case strings.Contains(e.Message, "\\u"):
			return "unicode"
		}
		return "unknown"
	case diag.CodeLiteralRange:
		t := string(src[e.Start:e.End])
		if !strings.HasPrefix(t, "0x") && strings.ContainsAny(t, ".eE") {
			return "finite"
		}
		return "whole"
	case diag.CodeTextBlockIndent:
		if e.Start < len(src) {
			switch src[e.Start] {
			case '\t':
				return "tab"
			case ' ':
				return "space"
			}
		}
	}
	return ""
}

// buildMessage builds the message of a parse diagnostic from the catalog
// (3.3, 3.4).
func buildMessage(e *Error, src []rune) string {
	params := []messages.Param{
		messages.Word("token", tokenText(src, e.Start, e.End)),
		messages.Val("key", e.Key),
		messages.Word("op", e.Op),
	}
	if e.Start < len(src) {
		params = append(params, messages.Word("char", charText(src[e.Start])))
	}
	return messages.Build(e.Code, templateOf(e, src), params...)
}

// continuesOperand reports whether a token would continue the expression
// before it as an operand: a binary operator, ?, ??, a step, as or is. Such a
// token after an if or a try is 3.4 rule 4.
func continuesOperand(t, next Token) bool {
	switch t.Kind {
	case OrOr, AndAnd, Eq, Ne, Lt, Le, Gt, Ge, Plus, Minus, Star, Slash, Percent, Power,
		Coalesce, Question, Dot, OptDot, LBrack, OptLBrack, LParen:
		return true
	case Word:
		return t.Text == "in" || t.Text == "as" || t.Text == "is" || (t.Text == "not" && next.isWord("in"))
	}
	return false
}

// core is the Core diagnostic of a text (F8 1.1), kept for tests of the
// rules before the mapping.
type core struct {
	code       string
	start, end int
}

func (e *Error) keepCore() {
	if e.core == nil {
		e.core = &core{e.Code, e.Start, e.End}
	}
}
