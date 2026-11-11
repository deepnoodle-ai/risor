// Package typetext parses type text by the grammar of spec/core/05-types.md
// 3.1 and 3.2 into a tree. It checks the grammar and the token rules only.
// The rules beyond the grammar (two records in one type, an undeclared name,
// depth, a key written twice) belong to the type checker.
package typetext

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
)

// MaxBrackets is how deep brackets may nest in type text (05-types.md 20.5).
// The parser refuses deeper text, so that it needs bounded stack.
const MaxBrackets = 64

// Error is type text that does not fit the grammar.
type Error struct {
	// Offset is the code point offset of the problem in the text.
	Offset  int
	Message string
}

// Error returns the offset and the message.
func (e *Error) Error() string { return fmt.Sprintf("type text at %d: %s", e.Offset, e.Message) }

// Type is "Alt { | Alt }".
type Type struct {
	Alts []*Alt
}

// Alt is a Prim followed by Lists pairs of "[]".
type Alt struct {
	Prim  *Prim
	Lists int
}

// PrimKind is the form of a Prim.
type PrimKind uint8

// The forms of a Prim.
const (
	PrimWord   PrimKind = iota // null bool int float string any never
	PrimString                 // a string literal
	PrimMap                    // map<T>
	PrimRecord                 // {...}
	PrimName                   // an opaque type name
	PrimVar                    // a type variable
	PrimParen                  // ( T )
)

// Prim is one primary of type text.
type Prim struct {
	Kind PrimKind
	// Text is the word for PrimWord, the value for PrimString, and the name
	// for PrimName and PrimVar.
	Text string
	// Elem is the T of map<T> and of ( T ).
	Elem   *Type
	Record *Record
}

// Record is "{ fields, ...rest }".
type Record struct {
	Fields []*Field
	// HasRest reports whether the record has a Rest. Rest is nil for "..."
	// alone, whose rest is any.
	HasRest bool
	Rest    *Type
}

// Field is one field of a record.
type Field struct {
	Key      string
	Optional bool
	Type     *Type
}

// Sig is a signature: "(params) => Type [!] [where V: B, ...]".
type Sig struct {
	Params []*Param
	Result *Type
	Fails  bool
	Bounds []Bound
}

// Param is one parameter of a signature.
type Param struct {
	Name     string
	Optional bool
	Rest     bool
	// Fn is set for a parameter that takes a function; Type otherwise.
	Fn   *Fn
	Type *Type
}

// Fn is the type of a parameter that takes a lambda.
type Fn struct {
	Params []FnParam
	// Result is nil when the result is the word unknown.
	Result *Type
}

// FnParam is one parameter of an Fn.
type FnParam struct {
	Name string
	Type *Type
}

// Bound is "V: ordered" or "V: number".
type Bound struct {
	Var   string
	Bound string
}

// Method is "Recv.name: Sig".
type Method struct {
	Recv *Alt
	Name string
	Sig  *Sig
}

// MinArgs is the count of parameters that are neither optional nor rest.
func (s *Sig) MinArgs() int {
	n := 0
	for _, p := range s.Params {
		if !p.Optional && !p.Rest {
			n++
		}
	}
	return n
}

// MaxArgs is the most arguments a call may give, or -1 with a rest
// parameter.
func (s *Sig) MaxArgs() int {
	for _, p := range s.Params {
		if p.Rest {
			return -1
		}
	}
	return len(s.Params)
}

// RecordFields returns the record of a type that is exactly one record, as an
// environment's env text is (05-types.md 19.1).
func (t *Type) RecordFields() (*Record, bool) {
	if len(t.Alts) == 1 && t.Alts[0].Lists == 0 && t.Alts[0].Prim.Kind == PrimRecord {
		return t.Alts[0].Prim.Record, true
	}
	return nil, false
}

// ParseType parses a Type. Type variables are not allowed.
func ParseType(text string) (*Type, error) {
	p, err := newParser(text, false)
	if err != nil {
		return nil, err
	}
	t, err := finish(p, p.typ)
	if err != nil {
		return nil, err
	}
	if err := tooDeep(depthType(t)); err != nil {
		return nil, err
	}
	return t, nil
}

// ParseSig parses a Sig.
func ParseSig(text string) (*Sig, error) {
	p, err := newParser(text, true)
	if err != nil {
		return nil, err
	}
	s, err := finish(p, p.sig)
	if err != nil {
		return nil, err
	}
	if err := tooDeep(depthSig(s)); err != nil {
		return nil, err
	}
	return s, nil
}

// ParseRecv parses the receiver of a method, an Alt.
func ParseRecv(text string) (*Alt, error) {
	p, err := newParser(text, true)
	if err != nil {
		return nil, err
	}
	a, err := finish(p, p.alt)
	if err != nil {
		return nil, err
	}
	if err := tooDeep(depthAlt(a)); err != nil {
		return nil, err
	}
	return a, nil
}

// ParseMethod parses a Method: "Recv.name: Sig".
func ParseMethod(text string) (*Method, error) {
	p, err := newParser(text, true)
	if err != nil {
		return nil, err
	}
	m, err := finish(p, func() *Method {
		recv := p.alt()
		p.expect(syntax.Dot)
		name := p.expectWord()
		p.expect(syntax.Colon)
		return &Method{Recv: recv, Name: name, Sig: p.sig()}
	})
	if err != nil {
		return nil, err
	}
	if err := tooDeep(max(depthAlt(m.Recv), depthSig(m.Sig))); err != nil {
		return nil, err
	}
	return m, nil
}

type parser struct {
	toks     []syntax.Token
	i        int
	allowVar bool
	depth    int
}

type bail struct{ err *Error }

func newParser(text string, allowVar bool) (*parser, error) {
	toks, lerr := syntax.Lex(syntax.NewFile(text))
	if lerr != nil {
		return nil, &Error{Offset: lerr.Start, Message: lerr.Message}
	}
	return &parser{toks: toks, allowVar: allowVar}, nil
}

func finish[T any](p *parser, f func() T) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bail)
			if !ok {
				panic(r)
			}
			var zero T
			out, err = zero, b.err
		}
	}()
	out = f()
	if p.tok().Kind != syntax.EOF {
		p.fail("unexpected text after the type")
	}
	return out, nil
}

func (p *parser) tok() syntax.Token { return p.toks[p.i] }

func (p *parser) peek(k int) syntax.Token {
	if p.i+k < len(p.toks) {
		return p.toks[p.i+k]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) next() {
	if p.i < len(p.toks)-1 {
		p.i++
	}
}

func (p *parser) fail(msg string) {
	panic(bail{&Error{Offset: p.tok().Start, Message: msg}})
}

func (p *parser) is(k syntax.TokenKind) bool { return p.tok().Kind == k }

func (p *parser) isWord(w string) bool {
	t := p.tok()
	return t.Kind == syntax.Word && t.Text == w
}

func (p *parser) expect(k syntax.TokenKind) {
	if !p.is(k) {
		p.fail(fmt.Sprintf("expected %s", k))
	}
	p.next()
}

func (p *parser) expectWord() string {
	if !p.is(syntax.Word) {
		p.fail("expected a word")
	}
	w := p.tok().Text
	p.next()
	return w
}

func (p *parser) open(k syntax.TokenKind) {
	p.expect(k)
	p.depth++
	if p.depth > MaxBrackets {
		p.fail("type text is nested more than 64 brackets deep")
	}
}

func (p *parser) close(k syntax.TokenKind) {
	p.expect(k)
	p.depth--
}

var typeWords = map[string]bool{
	"null": true, "bool": true, "int": true, "float": true, "string": true,
	"any": true, "never": true,
}

// wrongWords are the words of 05-types.md 3.8, with the form to write.
var wrongWords = map[string]string{
	"number": "int or float", "boolean": "bool", "undefined": "null",
	"void": "null", "nil": "null", "object": "a record or map<T>",
	"condition": "a type", "unknown": "a type",
}

func isUpper(r byte) bool { return r >= 'A' && r <= 'Z' }

func (p *parser) typ() *Type {
	t := &Type{Alts: []*Alt{p.alt()}}
	for p.is(syntax.Pipe) {
		p.next()
		t.Alts = append(t.Alts, p.alt())
	}
	return t
}

func (p *parser) alt() *Alt {
	a := &Alt{Prim: p.prim()}
	for p.is(syntax.LBrack) && p.peek(1).Kind == syntax.RBrack {
		p.next()
		p.next()
		a.Lists++
	}
	return a
}

func (p *parser) prim() *Prim {
	t := p.tok()
	switch t.Kind {
	case syntax.TokString:
		if t.Form == '`' || t.Block {
			p.fail("a backtick literal is not type text")
		}
		p.next()
		return &Prim{Kind: PrimString, Text: t.Str}
	case syntax.LParen:
		p.open(syntax.LParen)
		e := p.typ()
		p.close(syntax.RParen)
		return &Prim{Kind: PrimParen, Elem: e}
	case syntax.LBrace:
		return &Prim{Kind: PrimRecord, Record: p.record()}
	case syntax.Word:
		w := t.Text
		switch {
		case typeWords[w]:
			p.next()
			return &Prim{Kind: PrimWord, Text: w}
		case w == "map":
			p.next()
			p.open(syntax.Lt)
			e := p.typ()
			p.close(syntax.Gt)
			return &Prim{Kind: PrimMap, Elem: e}
		case isUpper(w[0]) && len(w) == 1:
			if !p.allowVar {
				p.fail("a type variable may stand only in a signature")
			}
			p.next()
			return &Prim{Kind: PrimVar, Text: w}
		case isUpper(w[0]):
			p.next()
			return &Prim{Kind: PrimName, Text: w}
		}
		if s, ok := wrongWords[w]; ok {
			p.fail(fmt.Sprintf("`%s` is not a type; write %s", w, s))
		}
		p.fail(fmt.Sprintf("`%s` is not a type", w))
	}
	p.fail("expected a type")
	return nil
}

func (p *parser) record() *Record {
	p.open(syntax.LBrace)
	r := &Record{}
	if !p.is(syntax.RBrace) {
		for {
			if p.is(syntax.Ellipsis) {
				p.next()
				r.HasRest = true
				if !p.is(syntax.Comma) && !p.is(syntax.RBrace) {
					r.Rest = p.typ()
				}
				if p.is(syntax.Comma) {
					p.next()
				}
				break
			}
			f := &Field{}
			switch t := p.tok(); {
			case t.Kind == syntax.Word:
				f.Key = t.Text
			case t.Kind == syntax.TokString && t.Form != '`' && !t.Block:
				f.Key = t.Str
			default:
				p.fail("expected a field")
			}
			p.next()
			if p.is(syntax.Question) {
				f.Optional = true
				p.next()
			}
			p.expect(syntax.Colon)
			f.Type = p.typ()
			r.Fields = append(r.Fields, f)
			if p.is(syntax.Comma) {
				p.next()
				if p.is(syntax.RBrace) {
					break
				}
				continue
			}
			break
		}
	}
	p.close(syntax.RBrace)
	return r
}

func (p *parser) sig() *Sig {
	s := &Sig{}
	p.open(syntax.LParen)
	if !p.is(syntax.RParen) {
		for {
			s.Params = append(s.Params, p.param())
			if !p.is(syntax.Comma) {
				break
			}
			p.next()
		}
	}
	p.close(syntax.RParen)
	p.expect(syntax.Arrow)
	s.Result = p.typ()
	if p.is(syntax.Bang) {
		s.Fails = true
		p.next()
	}
	if p.isWord("where") {
		p.next()
		for {
			v := p.tok()
			if v.Kind != syntax.Word || len(v.Text) != 1 || !isUpper(v.Text[0]) {
				p.fail("expected a type variable")
			}
			p.next()
			p.expect(syntax.Colon)
			b := p.tok()
			if !(b.Kind == syntax.Word && (b.Text == "ordered" || b.Text == "number")) {
				p.fail("expected ordered or number")
			}
			p.next()
			s.Bounds = append(s.Bounds, Bound{Var: v.Text, Bound: b.Text})
			if !p.is(syntax.Comma) {
				break
			}
			p.next()
		}
	}
	return s
}

func (p *parser) param() *Param {
	pm := &Param{}
	if p.is(syntax.Ellipsis) {
		pm.Rest = true
		p.next()
	}
	pm.Name = p.expectWord()
	if p.is(syntax.Question) {
		pm.Optional = true
		p.next()
	}
	p.expect(syntax.Colon)
	// 3.2: "(" then ")", or "(" then a Word then ":", starts an Fn.
	if p.is(syntax.LParen) && (p.peek(1).Kind == syntax.RParen ||
		(p.peek(1).Kind == syntax.Word && p.peek(2).Kind == syntax.Colon)) {
		pm.Fn = p.fn()
	} else {
		pm.Type = p.typ()
	}
	return pm
}

func (p *parser) fn() *Fn {
	f := &Fn{}
	p.open(syntax.LParen)
	if !p.is(syntax.RParen) {
		for {
			name := p.expectWord()
			p.expect(syntax.Colon)
			f.Params = append(f.Params, FnParam{Name: name, Type: p.typ()})
			if !p.is(syntax.Comma) {
				break
			}
			p.next()
		}
	}
	p.close(syntax.RParen)
	p.expect(syntax.Arrow)
	if p.isWord("unknown") {
		p.next()
		return f
	}
	f.Result = p.typ()
	return f
}

// String prints the tree back as type text, for tests. It is not the
// printing of 05-types.md 3.9, which needs the checker's types.
func (t *Type) String() string {
	parts := make([]string, len(t.Alts))
	for i, a := range t.Alts {
		parts[i] = a.String()
	}
	return strings.Join(parts, " | ")
}

// String prints the alternative back as type text.
func (a *Alt) String() string {
	return a.Prim.String() + strings.Repeat("[]", a.Lists)
}

// String prints the primary back as type text.
func (p *Prim) String() string {
	switch p.Kind {
	case PrimWord, PrimName, PrimVar:
		return p.Text
	case PrimString:
		return fmt.Sprintf("%q", p.Text)
	case PrimMap:
		return "map<" + p.Elem.String() + ">"
	case PrimParen:
		return "(" + p.Elem.String() + ")"
	case PrimRecord:
		var parts []string
		for _, f := range p.Record.Fields {
			opt := ""
			if f.Optional {
				opt = "?"
			}
			parts = append(parts, fmt.Sprintf("%s%s: %s", f.Key, opt, f.Type))
		}
		if p.Record.HasRest {
			if p.Record.Rest == nil {
				parts = append(parts, "...")
			} else {
				parts = append(parts, "..."+p.Record.Rest.String())
			}
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "?"
}

// String prints the signature back as type text.
func (s *Sig) String() string {
	var parts []string
	for _, pm := range s.Params {
		pre, opt := "", ""
		if pm.Rest {
			pre = "..."
		}
		if pm.Optional {
			opt = "?"
		}
		var ty string
		if pm.Fn != nil {
			var fp []string
			for _, q := range pm.Fn.Params {
				fp = append(fp, q.Name+": "+q.Type.String())
			}
			res := "unknown"
			if pm.Fn.Result != nil {
				res = pm.Fn.Result.String()
			}
			ty = "(" + strings.Join(fp, ", ") + ") => " + res
		} else {
			ty = pm.Type.String()
		}
		parts = append(parts, pre+pm.Name+opt+": "+ty)
	}
	out := "(" + strings.Join(parts, ", ") + ") => " + s.Result.String()
	if s.Fails {
		out += " !"
	}
	if len(s.Bounds) > 0 {
		var bs []string
		for _, b := range s.Bounds {
			bs = append(bs, b.Var+": "+b.Bound)
		}
		out += " where " + strings.Join(bs, ", ")
	}
	return out
}

// The depth of type text (05-types.md 20.5): a "[]" suffix is 1
// more than what it follows; a bracketed form is 1 more than the deepest of
// its contents; any other text is 0.

func depthType(t *Type) int {
	d := 0
	for _, a := range t.Alts {
		d = max(d, depthAlt(a))
	}
	return d
}

func depthAlt(a *Alt) int { return depthPrim(a.Prim) + a.Lists }

func depthPrim(p *Prim) int {
	switch p.Kind {
	case PrimMap, PrimParen:
		return 1 + depthType(p.Elem)
	case PrimRecord:
		d := 0
		for _, f := range p.Record.Fields {
			d = max(d, depthType(f.Type))
		}
		if p.Record.Rest != nil {
			d = max(d, depthType(p.Record.Rest))
		}
		return 1 + d
	}
	return 0
}

func depthSig(s *Sig) int {
	in := 0
	for _, pm := range s.Params {
		if pm.Fn != nil {
			fi := 0
			for _, q := range pm.Fn.Params {
				fi = max(fi, depthType(q.Type))
			}
			fd := 1 + fi
			if pm.Fn.Result != nil {
				fd = max(fd, depthType(pm.Fn.Result))
			}
			in = max(in, fd)
		} else {
			in = max(in, depthType(pm.Type))
		}
	}
	return max(1+in, depthType(s.Result))
}

func tooDeep(d int) error {
	if d > MaxBrackets {
		return &Error{Message: "type text is nested more than 64 deep"}
	}
	return nil
}
