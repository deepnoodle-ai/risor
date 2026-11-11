package risor

import (
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/go/v3/internal/check"
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/library"
	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// This file is the assist stage of package F12 (spec/packages/
// f12-tools.md, section 3): completion and hover at a position of a text
// that need not parse. Assist never fails, and reports no diagnostic.

// Completion is the result of completion (F12 3.5 item 3): the span of the
// prefix, which an editor replaces with a candidate's insert text, and the
// candidates in order.
type Completion struct {
	Span  Span
	Items []Candidate
}

// Candidate is one candidate of a completion (F12 3.12). Kind is name,
// lambda, variable, function, namespace, field, method, constant, key,
// literal or type. Detail is nil when the kind has none.
type Candidate struct {
	Label, Kind string
	Detail      *string
	Insert      string
}

// Hover is a hover record (F12 3.13). Kind is name, lambda, variable,
// function, namespace, field, method or constant; a field is nil when the
// record has none.
type Hover struct {
	Kind                 string
	Span                 Span
	Type, Signature, Doc *string
	Declaration          *Span
}

// assistPanics counts the faults assist turned into an empty result, for
// the tests: assist never fails, but a fault is still a fault.
var assistPanics atomic.Int64

func strp(s string) *string { return &s }

func isWordPart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isWordStart(r rune) bool { return isWordPart(r) && !(r >= '0' && r <= '9') }

// offsetOf is the offset of a position (F12 1.1), or false when the
// position is in no line of the text.
func offsetOf(src []rune, pos Pos) (int, bool) {
	if pos.Line < 1 || pos.Column < 1 {
		return 0, false
	}
	line, start := 1, 0
	for i := 0; line < pos.Line; i++ {
		if i >= len(src) {
			return 0, false
		}
		if src[i] == '\n' {
			line++
			start = i + 1
		}
	}
	end := start
	for end < len(src) && src[end] != '\n' {
		end++
	}
	if start+pos.Column-1 > end {
		return 0, false
	}
	return start + pos.Column - 1, true
}

// assist is one call of completion or hover.
type assist struct {
	e     *Env
	src   []rune
	file  *syntax.File
	prog  *syntax.Program
	res   *static.Result
	ty    *check.Types
	names *static.Names
	exp   *types.Type // the expected result, when it is a type
}

// analyze checks a repaired program (F12 3.3 item 4), ignoring its
// diagnostics.
func (a *assist) analyze(prog *syntax.Program, opts []CheckOption) {
	a.prog = prog
	names, varNames := a.e.staticNames()
	a.names = names
	a.res = static.Check(prog, names)
	exp, _, err := a.e.expected(opts)
	if err != nil {
		exp = check.Expect{}
	}
	_, a.ty = check.CheckTypes(prog, a.res, a.e.checkEnv(names, varNames), exp)
	if exp.Kind == check.ExpectType {
		a.exp = exp.Type
	}
}

func (a *assist) typeOf(n syntax.Node) *types.Type {
	if a.ty == nil {
		return nil
	}
	return a.ty.Nodes[n]
}

func (a *assist) slotType(key any) *types.Type {
	if a.res == nil || a.ty == nil {
		return nil
	}
	s, ok := a.res.Slots[key]
	if !ok || s < 0 || s >= len(a.ty.Slots) {
		return nil
	}
	return a.ty.Slots[s]
}

func (a *assist) span(start, end int) Span { return a.file.Span(start, end) }

// Complete lists the candidates of completion at a position of a source
// text (F12 3.2 to 3.12). opts may give the expected result.
func (e *Env) Complete(source string, pos Pos, opts ...CheckOption) (out Completion) {
	if !utf8.ValidString(source) {
		source = value.ReplaceInvalid(source)
	}
	f := syntax.NewFile(source)
	a := &assist{e: e, src: f.Runes(), file: f}
	out = Completion{Span: Span{Start: pos, End: pos}, Items: []Candidate{}}
	cut, ok := offsetOf(a.src, pos)
	if !ok {
		return out
	}
	defer func() {
		if r := recover(); r != nil { // assist never fails
			assistPanics.Add(1)
			out.Items = []Candidate{}
		}
	}()
	pre := a.src[:cut]
	ws := cut
	for ws > 0 && isWordPart(pre[ws-1]) {
		ws--
	}
	wordAt := ws
	if ws < cut && !isWordStart(pre[ws]) {
		wordAt = cut
	}
	empty := func(start int) Completion {
		return Completion{Span: a.span(start, cut), Items: []Candidate{}}
	}
	toks, end, lerr := syntax.CutLex(pre)
	var hole syntax.Token
	isString := false
	var quote rune
	if lerr != nil {
		openAtCut := lerr.End == cut && (lerr.Code == diag.CodeUnterminatedString || lerr.Code == diag.CodeUnterminatedComment)
		s := lerr.Start
		oneLine := lerr.Code == diag.CodeUnterminatedString && lerr.End == cut && s < cut &&
			(pre[s] == '"' || pre[s] == '\'') && !(s+2 < cut && pre[s+1] == pre[s] && pre[s+2] == pre[s])
		if !oneLine {
			if openAtCut {
				return empty(cut)
			}
			return empty(wordAt)
		}
		toks, end, lerr = syntax.CutLex(pre[:s])
		if lerr != nil {
			return empty(s + 1)
		}
		isString, quote = true, pre[s]
		hole = syntax.Token{Kind: syntax.TokString, Start: s, End: cut, ErrStart: s, ErrEnd: cut,
			Str: string(pre[s+1 : cut]), Form: quote}
	} else {
		lastEnd := 0
		if end > 0 {
			lastEnd = toks[end-1].End
		}
		if inLineComment(pre[lastEnd:cut]) {
			return empty(cut)
		}
		if end > 0 && toks[end-1].End == cut && toks[end-1].Kind == syntax.TokNumber {
			return empty(cut)
		}
		hs := cut
		if wordAt < cut && end > 0 && toks[end-1].Kind == syntax.Word && toks[end-1].Start == wordAt && toks[end-1].End == cut {
			hs = wordAt
			toks = append(toks[:end-1:end-1], toks[end:]...)
			end--
		}
		hole = syntax.Token{Kind: syntax.Word, Text: syntax.HoleText, Start: hs, End: cut, ErrStart: hs, ErrEnd: cut}
	}
	prefixStart := hole.Start
	if isString {
		prefixStart = hole.Start + 1
	}
	prefix := string(a.src[prefixStart:cut])
	out.Span = a.span(prefixStart, cut)
	all := make([]syntax.Token, 0, len(toks)+1)
	all = append(append(append(all, toks[:end]...), hole), toks[end:]...)
	prog := syntax.Repair(all, end+1, pre)
	var items []Candidate
	if prog == nil {
		// The fallback (3.5 item 1).
		if end > 0 && (all[end-1].Kind == syntax.Dot || all[end-1].Kind == syntax.OptDot) {
			return out
		}
		items = a.namesContext(nil)
	} else {
		a.analyze(prog, opts)
		w := &walker{a: a, hole: syntax.Span{Start: hole.Start, End: hole.End}, isString: isString}
		w.visit(prog, a.exp)
		items = w.candidates(quote)
	}
	for _, it := range items {
		if strings.HasPrefix(it.Label, prefix) {
			out.Items = append(out.Items, it)
		}
	}
	return out
}

// inLineComment reports whether a gap of blank space and comments ends
// inside a line comment.
func inLineComment(gap []rune) bool {
	for i := 0; i < len(gap); i++ {
		switch {
		case gap[i] == '/' && i+1 < len(gap) && gap[i+1] == '/':
			for i < len(gap) && gap[i] != '\n' && gap[i] != '\r' {
				i++
			}
			if i >= len(gap) {
				return true
			}
		case gap[i] == '/' && i+1 < len(gap) && gap[i+1] == '*':
			i += 2
			for i+1 < len(gap) && !(gap[i] == '*' && gap[i+1] == '/') {
				i++
			}
			i++
		}
	}
	return false
}

// binding is a name a program binds, as completion lists it.
type binding struct {
	name   string
	key    any // the slot key: *syntax.NamePat, *syntax.MapField or *syntax.Try
	lambda bool
}

// walker finds the hole in a repaired program and its context (3.5).
type walker struct {
	a        *assist
	hole     syntax.Span
	isString bool
	scopes   [][]binding
	found    bool

	ctx     string // names, keys, members, types, strings, or none
	inScope [][]binding
	recv    syntax.Node
	mapNode *syntax.Map
	mapExp  *types.Type
	strType *types.Type
	arm     *syntax.Match
}

func (w *walker) push()                  { w.scopes = append(w.scopes, nil) }
func (w *walker) pop()                   { w.scopes = w.scopes[:len(w.scopes)-1] }
func (w *walker) bind(b binding)         { w.scopes[len(w.scopes)-1] = append(w.scopes[len(w.scopes)-1], b) }
func (w *walker) at(sp syntax.Span) bool { return sp == w.hole }

func (w *walker) hit(ctx string) bool {
	w.found, w.ctx = true, ctx
	w.inScope = make([][]binding, len(w.scopes))
	for i, s := range w.scopes {
		w.inScope[i] = append([]binding(nil), s...)
	}
	return true
}

func (w *walker) isHoleWord(name string, sp syntax.Span) bool {
	return !w.isString && name == syntax.HoleText && w.at(sp)
}

func (w *walker) isHoleString(n syntax.Node) bool {
	s, ok := n.(*syntax.String)
	return ok && w.isString && w.at(s.Span())
}

// patternBindings are the names a pattern binds, in order.
func patternBindings(p syntax.Pattern) []binding {
	var out []binding
	switch p := p.(type) {
	case *syntax.NamePat:
		if !p.Discard {
			out = append(out, binding{name: p.Name, key: p})
		}
	case *syntax.MapPat:
		for _, f := range p.Fields {
			out = append(out, binding{name: f.Name, key: f})
		}
		if p.Rest != nil {
			out = append(out, binding{name: p.Rest.Name, key: p.Rest})
		}
	case *syntax.ListPat:
		for _, e := range p.Elems {
			if e.Name != nil && !e.Name.Discard {
				out = append(out, binding{name: e.Name.Name, key: e.Name})
			}
		}
		if p.Rest != nil {
			out = append(out, binding{name: p.Rest.Name, key: p.Rest})
		}
	}
	return out
}

// pattern visits a pattern's defaults, each with the names to its left in
// scope. A hole in a name the pattern binds, or in a key, has no context.
func (w *walker) pattern(p syntax.Pattern) bool {
	switch p := p.(type) {
	case *syntax.NamePat:
		if w.isHoleWord(p.Name, p.Span()) {
			return w.hit("none")
		}
	case *syntax.MapPat:
		w.push()
		defer w.pop()
		for _, f := range p.Fields {
			if w.isHoleWord(f.Name, f.NameSpan) || w.isHoleWord(f.Key, f.KeySpan) {
				return w.hit("none")
			}
			if f.Default != nil && w.visit(f.Default, nil) {
				return true
			}
			w.bind(binding{name: f.Name, key: f})
		}
		if p.Rest != nil && w.isHoleWord(p.Rest.Name, p.Rest.Span()) {
			return w.hit("none")
		}
	case *syntax.ListPat:
		w.push()
		defer w.pop()
		for _, e := range p.Elems {
			if e.Name != nil && w.isHoleWord(e.Name.Name, e.Name.Span()) {
				return w.hit("none")
			}
			if e.Default != nil && w.visit(e.Default, nil) {
				return true
			}
			if e.Name != nil && !e.Name.Discard {
				w.bind(binding{name: e.Name.Name, key: e.Name})
			}
		}
		if p.Rest != nil && w.isHoleWord(p.Rest.Name, p.Rest.Span()) {
			return w.hit("none")
		}
	}
	return false
}

func (w *walker) consts(cs []*syntax.Const) bool {
	for _, k := range cs {
		if w.pattern(k.Pattern) || w.visit(k.Value, nil) {
			return true
		}
		_, isLambda := k.Value.(*syntax.Lambda)
		for _, b := range patternBindings(k.Pattern) {
			if _, name := k.Pattern.(*syntax.NamePat); name && isLambda {
				b.lambda = true
			}
			w.bind(b)
		}
	}
	return false
}

// listElem is the expected type of an item of a list (3.9).
func listElem(exp *types.Type) *types.Type {
	if exp == nil || exp.IsAny() || exp.Elem() == nil {
		return nil
	}
	return exp.Elem()
}

// fieldExp is the expected type of the value of an entry with key k.
func fieldExp(exp *types.Type, k string) *types.Type {
	if exp == nil || exp.IsAny() {
		return nil
	}
	var out *types.Type
	for _, r := range types.Variants(exp) {
		var t *types.Type
		if f, ok := r.Field(k); ok {
			t = f.Type
		} else if r.Rest != nil && !r.Rest.IsNever() {
			t = r.Rest
		}
		if t == nil {
			continue
		}
		if out == nil {
			out = t
		} else {
			out = types.Join(out, t)
		}
	}
	return out
}

// hostParam is the expected type of argument i of a call of a host
// function.
func (w *walker) hostParam(call syntax.Node, i int) *types.Type {
	c := w.a.res.Calls[call]
	if c == nil || c.Kind != static.Host || c.Host == nil {
		return nil
	}
	s := w.a.e.hostSigs[c.Host.FullName()]
	if s == nil {
		return nil
	}
	p := s.ParamAt(i)
	if p == nil || p.Type == nil || types.HasVars(p.Type) {
		return nil
	}
	return p.Type
}

func (w *walker) args(call syntax.Node, args []syntax.Node) bool {
	for i, x := range args {
		if w.visit(x, w.hostParam(call, i)) {
			return true
		}
	}
	return false
}

func (w *walker) ptype(t *syntax.PType) bool {
	if t == nil {
		return false
	}
	for _, alt := range t.Alts {
		p := alt.Prim
		switch p.Kind {
		case syntax.PName:
			if w.isHoleWord(p.Text, p.Span) {
				return w.hit("types")
			}
		case syntax.PMap, syntax.PParen:
			if w.ptype(p.Elem) {
				return true
			}
		case syntax.PRecord:
			for _, f := range p.Fields {
				if w.isHoleWord(f.Key, f.KeySpan) {
					return w.hit("none")
				}
				if w.ptype(f.Type) {
					return true
				}
			}
			if w.ptype(p.Rest) {
				return true
			}
		}
	}
	return false
}

// visit walks a node with its expected type, and reports whether it found
// the hole.
func (w *walker) visit(n syntax.Node, exp *types.Type) bool {
	switch n := n.(type) {
	case nil:
		return false
	case *syntax.Program:
		w.push()
		if w.consts(n.Consts) {
			return true
		}
		return w.visit(n.Result, exp)
	case *syntax.Block:
		w.push()
		defer w.pop()
		if w.consts(n.Consts) {
			return true
		}
		return w.visit(n.Result, exp)
	case *syntax.Ident:
		if w.isHoleWord(n.Name, n.Span()) {
			return w.hit("names")
		}
	case *syntax.String:
		if w.isHoleString(n) {
			w.strType = exp
			return w.hit("strings")
		}
	case *syntax.Template:
		for _, p := range n.Parts {
			if p.Expr != nil && w.visit(p.Expr, nil) {
				return true
			}
		}
	case *syntax.List:
		el := listElem(exp)
		for _, x := range n.Items {
			if w.visit(x, el) {
				return true
			}
		}
	case *syntax.Spread:
		return w.visit(n.Value, nil)
	case *syntax.Map:
		for _, x := range n.Entries {
			en, ok := x.(*syntax.Entry)
			if !ok {
				if w.visit(x, nil) {
					return true
				}
				continue
			}
			if en.Shorthand && w.isHoleWord(en.Key, en.KeySpan) {
				if exp != nil && !exp.IsAny() && len(types.Variants(exp)) > 0 {
					w.mapNode, w.mapExp = n, exp
					return w.hit("keys")
				}
				return w.hit("names")
			}
			if en.KeyForm == syntax.KeyComputed {
				if w.visit(en.KeyExpr, nil) || w.visit(en.Value, nil) {
					return true
				}
				continue
			}
			if w.isHoleWord(en.Key, en.KeySpan) {
				return w.hit("none")
			}
			if w.visit(en.Value, fieldExp(exp, en.Key)) {
				return true
			}
		}
	case *syntax.Unary:
		return w.visit(n.X, nil)
	case *syntax.Binary:
		if n.Op == "==" || n.Op == "!=" {
			if w.isHoleString(n.X) {
				w.strType = w.a.typeOf(n.Y)
				return w.hit("strings")
			}
			if w.isHoleString(n.Y) {
				w.strType = w.a.typeOf(n.X)
				return w.hit("strings")
			}
		}
		return w.visit(n.X, nil) || w.visit(n.Y, nil)
	case *syntax.Cond:
		return w.visit(n.Test, nil) || w.visit(n.Then, exp) || w.visit(n.Else, exp)
	case *syntax.Member:
		if w.isHoleWord(n.Name, n.NameSpan) {
			w.recv = n.X
			return w.hit("members")
		}
		return w.visit(n.X, nil)
	case *syntax.Index:
		return w.visit(n.X, nil) || w.visit(n.Index, nil)
	case *syntax.MethodCall:
		if w.isHoleWord(n.Name, n.NameSpan) {
			w.recv = n.X
			return w.hit("members")
		}
		return w.visit(n.X, nil) || w.args(n, n.Args)
	case *syntax.OptChain:
		return w.visit(n.X, nil)
	case *syntax.Each:
		if w.visit(n.X, nil) {
			return true
		}
		for _, r := range n.Reads {
			switch r := r.(type) {
			case *syntax.Member:
				if w.isHoleWord(r.Name, r.NameSpan) {
					return w.hit("none")
				}
			case *syntax.Index:
				if w.visit(r.Index, nil) {
					return true
				}
			default:
				if w.visit(r, nil) {
					return true
				}
			}
		}
	case *syntax.Call:
		return w.args(n, n.Args)
	case *syntax.Lambda:
		w.push()
		defer w.pop()
		for _, p := range n.Params {
			if w.pattern(p.Pattern) {
				return true
			}
			if p.Default != nil && w.visit(p.Default, nil) {
				return true
			}
			for _, b := range patternBindings(p.Pattern) {
				w.bind(b)
			}
		}
		return w.visit(n.Body, nil)
	case *syntax.Match:
		if w.visit(n.Subject, nil) {
			return true
		}
		for _, arm := range n.Arms {
			for _, l := range arm.Literals {
				if w.isHoleString(l) {
					w.strType, w.arm = w.a.typeOf(n.Subject), n
					return w.hit("strings")
				}
			}
			if w.visit(arm.Result, exp) {
				return true
			}
		}
	case *syntax.Try:
		if w.visit(n.Body, exp) {
			return true
		}
		w.push()
		defer w.pop()
		if n.Name != "" {
			if w.isHoleWord(n.Name, n.NameSpan) {
				return w.hit("none")
			}
			w.bind(binding{name: n.Name, key: n})
		}
		return w.visit(n.Handler, exp)
	case *syntax.As:
		return w.visit(n.X, nil) || w.ptype(n.Type)
	}
	return false
}

// candidates are the candidates of the context found, before the prefix
// filters them.
func (w *walker) candidates(quote rune) []Candidate {
	if !w.found {
		return nil
	}
	a := w.a
	switch w.ctx {
	case "names":
		return a.namesContext(w.inScope)
	case "keys":
		return a.keysContext(w.mapNode, w.mapExp)
	case "members":
		if id, ok := w.recv.(*syntax.Ident); ok && a.isNamespace(id.Name) && !bound(w.inScope, id.Name) {
			return a.namespaceEntries(id.Name)
		}
		return a.members(a.typeOf(w.recv))
	case "types":
		return a.typesContext()
	case "strings":
		return a.stringsContext(w.strType, w.arm, w.hole, quote)
	}
	return nil
}

func bound(scopes [][]binding, name string) bool {
	for _, s := range scopes {
		for _, b := range s {
			if b.name == name {
				return true
			}
		}
	}
	return false
}

// isNamespace reports whether a name is a namespace of the environment: a
// group that is on, or a prefix of host functions.
func (a *assist) isNamespace(n string) bool {
	if a.e.groups[n] {
		return true
	}
	for _, f := range a.e.funcs {
		if f.ns == n {
			return true
		}
	}
	return false
}

func printed(t *types.Type) *string {
	if t == nil {
		return nil
	}
	return strp(t.String())
}

// namesContext is the names context (3.6): the names bound in scope, then
// the environment's variables, functions and namespaces.
func (a *assist) namesContext(scopes [][]binding) []Candidate {
	var out []Candidate
	seen := map[string]bool{}
	add := func(c Candidate) {
		if !seen[c.Label] {
			seen[c.Label] = true
			c.Insert = c.Label
			out = append(out, c)
		}
	}
	for i := len(scopes) - 1; i >= 0; i-- {
		for _, b := range scopes[i] {
			if seen[b.name] {
				continue
			}
			c := Candidate{Label: b.name, Kind: "name"}
			if b.lambda {
				c.Kind = "lambda"
			} else {
				c.Detail = printed(a.slotType(b.key))
			}
			add(c)
		}
	}
	vars := make([]string, 0, len(a.e.vars))
	for n := range a.e.vars {
		vars = append(vars, n)
	}
	sort.Strings(vars)
	for _, n := range vars {
		add(Candidate{Label: n, Kind: "variable", Detail: printed(a.e.varTypes[n])})
	}
	var fns []Candidate
	for n, en := range library.Functions {
		if !a.e.removed[n] {
			fns = append(fns, Candidate{Label: n, Kind: "function", Detail: strp(en.Signature)})
		}
	}
	for n, f := range a.e.funcs {
		if f.ns == "" {
			fns = append(fns, Candidate{Label: n, Kind: "function", Detail: printed0(sigText(a.e.hostSigs[n]))})
		}
	}
	sort.SliceStable(fns, func(i, j int) bool { return fns[i].Label < fns[j].Label })
	for _, c := range fns {
		add(c)
	}
	nss := map[string]bool{}
	for g := range a.e.groups {
		nss[g] = true
	}
	for _, f := range a.e.funcs {
		if f.ns != "" {
			nss[f.ns] = true
		}
	}
	for _, n := range sortedKeys(nss) {
		add(Candidate{Label: n, Kind: "namespace"})
	}
	return out
}

func printed0(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// namespaceEntries are the entries of a namespace (3.8).
func (a *assist) namespaceEntries(ns string) []Candidate {
	var out []Candidate
	if a.e.groups[ns] {
		for n, en := range library.GroupFunctions {
			if library.Namespace(n) == ns && !a.e.removed[n] {
				out = append(out, Candidate{Label: strings.TrimPrefix(n, ns+"."), Kind: "function", Detail: strp(en.Signature)})
			}
		}
		for n, en := range library.Constants {
			if library.Namespace(n) == ns && !a.e.removed[n] {
				out = append(out, Candidate{Label: strings.TrimPrefix(n, ns+"."), Kind: "constant", Detail: strp(en.Type)})
			}
		}
	}
	for n, f := range a.e.funcs {
		if f.ns == ns {
			out = append(out, Candidate{Label: f.name, Kind: "function", Detail: printed0(sigText(a.e.hostSigs[n]))})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	for i := range out {
		out[i].Insert = out[i].Label
	}
	return out
}

// members are the fields and the methods of a receiver's type (3.7).
func (a *assist) members(r *types.Type) []Candidate {
	if r == nil {
		return nil
	}
	r = types.Nonnull(r)
	if r.IsAny() || r.IsNever() {
		return nil
	}
	var out []Candidate
	fields := map[string]*types.Type{}
	for _, rec := range types.Variants(r) {
		for _, f := range rec.Fields {
			if !value.IsWord(f.Key) {
				continue
			}
			if t, ok := fields[f.Key]; ok {
				fields[f.Key] = types.Join(t, f.Type)
			} else {
				fields[f.Key] = f.Type
			}
		}
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, Candidate{Label: k, Kind: "field", Detail: printed(fields[k]), Insert: k})
	}
	methods := map[string]*string{}
	var order []string
	addMethod := func(name string, detail *string) {
		if _, ok := methods[name]; ok {
			return
		}
		methods[name] = detail
		order = append(order, name)
	}
	var kinds []value.Kind
	if r.HasString() {
		kinds = append(kinds, value.String)
	}
	if r.Elem() != nil {
		kinds = append(kinds, value.List)
	}
	if r.Record() != nil {
		kinds = append(kinds, value.Map)
	}
	for _, k := range kinds {
		for _, name := range methodNames(k) {
			en, ok := library.Method(k, name)
			if !ok || a.e.removed[en.EntryName()] {
				continue
			}
			addMethod(name, strp(en.Signature))
		}
	}
	ops := append([]string(nil), r.OpaqueNames()...)
	sort.Strings(ops)
	for _, tn := range ops {
		o := a.e.opaque[tn]
		if o == nil {
			continue
		}
		var ms []string
		for m := range o.methods {
			ms = append(ms, m)
		}
		sort.Strings(ms)
		for _, m := range ms {
			addMethod(m, printed0(sigText(a.e.methodSigs[tn][m])))
		}
	}
	sort.Strings(order)
	for _, m := range order {
		out = append(out, Candidate{Label: m, Kind: "method", Detail: methods[m], Insert: m})
	}
	return out
}

// methodNames are the names of the library's methods of a kind.
func methodNames(k value.Kind) []string {
	seen := map[string]bool{}
	var out []string
	for _, en := range library.Entries {
		if en.IsMethod() && en.Kind == k && !seen[en.Name] {
			seen[en.Name] = true
			out = append(out, en.Name)
		}
	}
	return out
}

// keysContext lists the keys of a map's expected records that it does not
// write yet (3.9 items 1 and 2).
func (a *assist) keysContext(m *syntax.Map, exp *types.Type) []Candidate {
	written := map[string]bool{}
	for _, x := range m.Entries {
		if en, ok := x.(*syntax.Entry); ok && en.KeyForm != syntax.KeyComputed && en.Key != syntax.HoleText {
			written[en.Key] = true
		}
	}
	keys := map[string]*types.Type{}
	for _, rec := range types.Variants(exp) {
		for _, f := range rec.Fields {
			if written[f.Key] {
				continue
			}
			if t, ok := keys[f.Key]; ok {
				keys[f.Key] = types.Join(t, f.Type)
			} else {
				keys[f.Key] = f.Type
			}
		}
	}
	var out []Candidate
	for _, k := range sortedTypeKeys(keys) {
		ins := k
		if !value.IsWord(k) {
			ins = value.QuoteJSON(k)
		}
		out = append(out, Candidate{Label: k, Kind: "key", Detail: printed(keys[k]), Insert: ins})
	}
	return out
}

func sortedTypeKeys(m map[string]*types.Type) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// typesContext is the types context (3.10).
func (a *assist) typesContext() []Candidate {
	names := map[string]bool{}
	for _, w := range []string{"any", "bool", "float", "int", "map", "never", "null", "string"} {
		names[w] = true
	}
	for _, d := range a.e.named {
		names[d.name] = true
	}
	for n := range a.e.opaque {
		names[n] = true
	}
	var out []Candidate
	for _, n := range sortedKeys(names) {
		out = append(out, Candidate{Label: n, Kind: "type", Insert: n})
	}
	return out
}

// stringsContext is the strings context (3.11).
func (a *assist) stringsContext(t *types.Type, m *syntax.Match, hole syntax.Span, quote rune) []Candidate {
	if t == nil || t.IsAny() {
		return nil
	}
	listed := map[string]bool{}
	if m != nil {
		for _, arm := range m.Arms {
			for _, l := range arm.Literals {
				if s, ok := l.(*syntax.String); ok && s.Span() != hole {
					listed[s.Value] = true
				}
			}
		}
	}
	var out []Candidate
	for _, s := range t.Lits() {
		if listed[s] {
			continue
		}
		out = append(out, Candidate{Label: s, Kind: "literal", Insert: stringContent(s, quote)})
	}
	return out
}

// stringContent writes a value as the content of a one-line string in a
// quote (3.11 item 3).
func stringContent(s string, quote rune) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == quote:
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			const hex = "0123456789abcdef"
			b.WriteString(`\u00`)
			b.WriteByte(hex[r>>4])
			b.WriteByte(hex[r&15])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Hover gives the hover record of the word at a position of a source text
// (F12 3.13), or nil. opts may give the expected result.
func (e *Env) Hover(source string, pos Pos, opts ...CheckOption) (h *Hover) {
	if !utf8.ValidString(source) {
		source = value.ReplaceInvalid(source)
	}
	f := syntax.NewFile(source)
	a := &assist{e: e, src: f.Runes(), file: f}
	off, ok := offsetOf(a.src, pos)
	if !ok || off >= len(a.src) {
		return nil
	}
	defer func() {
		if r := recover(); r != nil { // assist never fails
			assistPanics.Add(1)
			h = nil
		}
	}()
	var ws, we int
	var prog *syntax.Program
	if toks, lerr := syntax.Lex(f); lerr == nil {
		k := -1
		for i, t := range toks {
			if t.Start <= off && off < t.End {
				k = i
				break
			}
		}
		if k < 0 || toks[k].Kind != syntax.Word {
			return nil
		}
		ws, we = toks[k].Start, toks[k].End
		if p, err := syntax.Parse(f); err == nil {
			prog = p
		} else {
			cut := we
			// The ( that directly follows the word, with nothing between
			// (F12 3.2 item 3).
			if k+1 < len(toks) && toks[k+1].Kind == syntax.LParen && toks[k+1].Start == we {
				cut = toks[k+1].End
			}
			prog = a.repairAt(cut)
		}
	} else {
		if !isWordPart(a.src[off]) {
			return nil
		}
		ws, we = off, off
		for ws > 0 && isWordPart(a.src[ws-1]) {
			ws--
		}
		for we < len(a.src) && isWordPart(a.src[we]) {
			we++
		}
		if !isWordStart(a.src[ws]) {
			return nil
		}
		cut := we
		// The ( that directly follows the word, with no white space or
		// comment between (F12 3.2 item 3).
		if we < len(a.src) && a.src[we] == '(' {
			cut = we + 1
		}
		toks, _, lerr := syntax.CutLex(a.src[:cut])
		if lerr != nil {
			return nil
		}
		found := false
		for _, t := range toks {
			if t.Kind == syntax.Word && t.Start == ws && t.End == we {
				found = true
			}
		}
		if !found {
			return nil
		}
		prog = a.repairAt(cut)
	}
	if prog == nil {
		return nil
	}
	a.analyze(prog, opts)
	return a.hover(syntax.Span{Start: ws, End: we})
}

// repairAt is the repaired program of the text before a cut, or nil.
func (a *assist) repairAt(cut int) *syntax.Program {
	toks, end, lerr := syntax.CutLex(a.src[:cut])
	if lerr != nil {
		return nil
	}
	return syntax.Repair(toks, end, a.src[:cut])
}

// hover finds the word in the tree and gives its record (3.13 item 3).
func (a *assist) hover(word syntax.Span) *Hover {
	sp := a.span(word.Start, word.End)
	rec := func(kind string) *Hover { return &Hover{Kind: kind, Span: sp} }
	var out *Hover
	declOf := func(key any) *Span {
		var s syntax.Span
		switch k := key.(type) {
		case *syntax.NamePat:
			s = k.Span()
		case *syntax.MapField:
			s = k.NameSpan
		case *syntax.Try:
			s = k.NameSpan
		default:
			return nil
		}
		d := a.span(s.Start, s.End)
		return &d
	}
	bindingKey := func(b *static.Binding) any {
		for k, s := range a.res.Slots {
			if s == b.Slot {
				return k
			}
		}
		return nil
	}
	lambdaName := func(l *syntax.Lambda) *Span {
		var found *Span
		syntax.Walk(a.prog, func(n syntax.Node) bool {
			if c, ok := n.(*syntax.Const); ok && c.Value == syntax.Node(l) {
				if np, ok := c.Pattern.(*syntax.NamePat); ok {
					d := a.span(np.Span().Start, np.Span().End)
					found = &d
				}
			}
			return found == nil
		})
		return found
	}
	site := func(key any) *Hover {
		h := rec("name")
		h.Type = printed(a.slotType(key))
		h.Declaration = declOf(key)
		return h
	}
	callee := func(n syntax.Node) *Hover {
		c := a.res.Calls[n]
		if c == nil {
			return nil
		}
		switch c.Kind {
		case static.Lib:
			h := rec("function")
			h.Type = printed(a.typeOf(n))
			h.Signature = strp(c.Lib.Signature)
			h.Doc = printed0(c.Lib.Doc)
			return h
		case static.Host:
			h := rec("function")
			h.Type = printed(a.typeOf(n))
			h.Signature = printed0(sigText(a.e.hostSigs[c.Host.FullName()]))
			return h
		case static.Lambda:
			h := rec("lambda")
			h.Type = printed(a.typeOf(n))
			h.Declaration = lambdaName(c.Lambda)
			return h
		}
		return nil
	}
	checkPattern := func(p syntax.Pattern) {
		switch p := p.(type) {
		case *syntax.NamePat:
			if p.Span() == word && !p.Discard {
				out = site(p)
			}
		case *syntax.MapPat:
			for _, f := range p.Fields {
				if f.NameSpan == word {
					out = site(f)
				}
			}
			if p.Rest != nil && p.Rest.Span() == word {
				out = site(p.Rest)
			}
		case *syntax.ListPat:
			for _, el := range p.Elems {
				if el.Name != nil && el.Name.Span() == word && !el.Name.Discard {
					out = site(el.Name)
				}
			}
			if p.Rest != nil && p.Rest.Span() == word {
				out = site(p.Rest)
			}
		}
	}
	var visit func(n syntax.Node) bool
	visit = func(n syntax.Node) bool {
		if out != nil {
			return false
		}
		switch n := n.(type) {
		case *syntax.Const:
			checkPattern(n.Pattern)
		case *syntax.Param:
			checkPattern(n.Pattern)
		case *syntax.Lambda:
			for _, p := range n.Params {
				checkPattern(p.Pattern)
			}
		case *syntax.Try:
			if n.Name != "" && n.NameSpan == word {
				out = site(n)
			}
		case *syntax.Ident:
			if n.Span() != word {
				break
			}
			b := a.res.Idents[n]
			switch {
			case b == nil:
				if a.isNamespace(n.Name) {
					out = rec("namespace")
				}
			case b.Kind == static.EnvVar:
				out = rec("variable")
				out.Type = printed(a.typeOf(n))
			case b.Kind == static.ConstLambda:
				out = rec("name")
				out.Type = printed(a.typeOf(n))
				out.Declaration = lambdaName(b.Lambda)
			default:
				out = rec("name")
				out.Type = printed(a.typeOf(n))
				out.Declaration = declOf(bindingKey(b))
			}
		case *syntax.Call:
			if n.NameSpan == word {
				out = callee(n)
			}
		case *syntax.MethodCall:
			if n.NameSpan != word {
				break
			}
			if a.res.NSCalls[n] {
				out = callee(n)
				break
			}
			out = rec("method")
			out.Type = printed(a.typeOf(n))
			if r := a.typeOf(n.X); r != nil {
				out.Signature, out.Doc = a.methodSig(types.Nonnull(r), n.Name)
			}
		case *syntax.Member:
			if n.NameSpan != word {
				break
			}
			if _, ok := a.res.Consts[n]; ok {
				if id, ok := n.X.(*syntax.Ident); ok {
					if en := library.Constants[id.Name+"."+n.Name]; en != nil {
						out = rec("constant")
						out.Type = strp(en.Type)
						out.Doc = printed0(en.Doc)
						break
					}
				}
			}
			out = rec("field")
			out.Type = printed(a.typeOf(n))
		}
		return out == nil
	}
	syntax.Walk(a.prog, visit)
	return out
}

// methodSig is the signature and the doc of the method a receiver's type
// resolves, without its receiver.
func (a *assist) methodSig(r *types.Type, name string) (*string, *string) {
	var kinds []value.Kind
	if r.HasString() {
		kinds = append(kinds, value.String)
	}
	if r.Elem() != nil {
		kinds = append(kinds, value.List)
	}
	if r.Record() != nil {
		kinds = append(kinds, value.Map)
	}
	for _, k := range kinds {
		if en, ok := library.Method(k, name); ok && !a.e.removed[en.EntryName()] {
			return strp(en.Signature), printed0(en.Doc)
		}
	}
	ops := append([]string(nil), r.OpaqueNames()...)
	sort.Strings(ops)
	for _, tn := range ops {
		if s := a.e.methodSigs[tn][name]; s != nil {
			return printed0(sigText(s)), nil
		}
	}
	return nil, nil
}
