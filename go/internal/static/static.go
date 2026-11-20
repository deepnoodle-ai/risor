// Package static applies the static rules (spec/core/05-types.md 22) and
// resolves every name of a program. The evaluator compiles from its result.
package static

import (
	"github.com/deepnoodle-ai/risor/go/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"sort"
	"strconv"

	"github.com/deepnoodle-ai/risor/go/v3/internal/library"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/typetext"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// Func is a host function of the environment.
type Func struct {
	Name      string // without the namespace
	Namespace string
	Sig       *typetext.Sig
	Index     int
}

// FullName is "ns.f" or "f".
func (f *Func) FullName() string {
	if f.Namespace == "" {
		return f.Name
	}
	return f.Namespace + "." + f.Name
}

// Names are what the rules read from the environment (22.2).
type Names struct {
	Vars       map[string]int   // variable name to its index
	Funcs      map[string]*Func // by full name
	Namespaces map[string]bool
	Library    map[string]*library.Entry // library functions the environment has
	// Types is what a program type may name: the opaque types and the
	// named types (package F4 6.1).
	Types *types.Scope
	// Constants are the constants of the groups that are on, by full name
	// (package F6 2.4).
	Constants map[string]float64
}

// BindKind is the kind of a binding.
type BindKind uint8

// The kinds of binding.
const (
	Value       BindKind = iota // a const name, a parameter, a pattern name, a catch name
	ConstLambda                 // a const-bound lambda
	EnvVar                      // a variable of the environment
	// FnParam is a parameter with a function type: a function binding
	// (f17-recursion.md 13.1). It has a slot, which holds the lambda passed.
	FnParam
)

// Binding is one binding of a name.
type Binding struct {
	Kind   BindKind
	Name   string
	Slot   int // for Value and FnParam
	Lambda *syntax.Lambda
	Var    int // for EnvVar
	// Param is the parameter that binds the name, for a parameter written as
	// a name, and for FnParam (f17-recursion.md 13.2).
	Param *syntax.Param
}

// HintParam reports whether a value binding is a parameter written as a
// name with no default: the form a parameter with a function type takes.
// Its call and its use as a function name the fix (f17-recursion.md 13.5).
func (b *Binding) HintParam() bool {
	return b != nil && b.Kind == Value && b.Param != nil && b.Param.Default == nil
}

// CalleeKind is what a call calls.
type CalleeKind uint8

// The kinds of callee.
const (
	Unknown CalleeKind = iota
	Lambda
	Host
	Lib
	// Param is a parameter with a function type (f17-recursion.md 13.2).
	Param
)

// Callee is the callee of a Call, or of a MethodCall that calls a namespace.
type Callee struct {
	Kind   CalleeKind
	Lambda *syntax.Lambda
	Host   *Func
	Lib    *library.Entry
	// Binding is the binding of a Param callee.
	Binding *Binding
}

// Diag is one static diagnostic.
type Diag struct {
	Code string
	Span syntax.Span
	// Tmpl and Params are the message's template and parameters (package
	// F8 3.5). The text of the span is added where the message is built.
	Tmpl   string
	Params []messages.Param
}

// Result is what the rules found.
type Result struct {
	Diags   []Diag
	Idents  map[*syntax.Ident]*Binding
	Calls   map[syntax.Node]*Callee
	NSCalls map[*syntax.MethodCall]bool
	// Slots maps each *syntax.NamePat, *syntax.MapField and *syntax.Try
	// that binds a name to its slot.
	Slots  map[any]int
	NSlots int
	// AsTypes holds the resolved type of each As node whose program type
	// has no diagnostic (package F4 3.5, 6.1).
	AsTypes map[*syntax.As]*types.Type
	// Consts holds each Member that reads a constant of a group, with its
	// value (F6 2.4 item 2).
	Consts map[*syntax.Member]float64
	// Coalesced holds the last step, a Member or an Index, of each
	// coalesced read under ?? (01-language.md 6.6). The node alone decides
	// it, so a checked and an unchecked compile agree.
	Coalesced map[syntax.Node]bool
	// AnnTypes holds the resolved type of each program type of an
	// annotation or a type line that has no diagnostic (package F16 3.1):
	// the type of a const line, a parameter, a result, and the parameters
	// and result of an Fn.
	AnnTypes map[*syntax.PType]*types.Type
	// SelfRefs holds each reference of a const-bound lambda to its own
	// name: a Call by that name, or an Ident given as a lambda argument
	// (01-language.md 5 rule 6, 14.4; recursion-needs-types).
	SelfRefs map[syntax.Node]bool
}

type scope struct {
	parent *scope
	names  map[string]*Binding
	// vars are the environment's variables, for the outermost scope: a
	// binding is made for one when a name reads it, not for every one.
	vars map[string]int
}

func (s *scope) lookup(n string) *Binding {
	for ; s != nil; s = s.parent {
		if b, ok := s.names[n]; ok {
			return b
		}
		if i, ok := s.vars[n]; ok {
			b := &Binding{Kind: EnvVar, Name: n, Var: i}
			s.names[n] = b
			return b
		}
	}
	return nil
}

type walker struct {
	names *Names
	res   *Result
	seen  map[diagKey]bool
	// notLast holds each ?? node whose operands are all operands of its
	// chain other than the last (01-language.md 6.6).
	notLast map[*syntax.Binary]bool
	// types is what a program type may name at the place being walked: the
	// environment's type names and the type lines before it (F16 3.1).
	types *types.Scope
	// lineNames holds the name of each type line walked (F16 3.1).
	lineNames map[string]bool
	// self holds, for each const-bound lambda that refers to its own name,
	// its first reference in the text (05-types.md 22.5).
	self map[*syntax.Lambda]selfRef
	// last is the binding bind made last.
	last *Binding
}

// selfRef is a reference of a const-bound lambda to its own name.
type selfRef struct {
	name string
	sp   syntax.Span
}

// Check applies the static rules to a program and resolves its names.
func Check(prog *syntax.Program, names *Names) *Result {
	w := &walker{names: names, seen: map[diagKey]bool{}, res: &Result{
		Idents:    map[*syntax.Ident]*Binding{},
		Calls:     map[syntax.Node]*Callee{},
		NSCalls:   map[*syntax.MethodCall]bool{},
		Slots:     map[any]int{},
		AsTypes:   map[*syntax.As]*types.Type{},
		Coalesced: map[syntax.Node]bool{},
		AnnTypes:  map[*syntax.PType]*types.Type{},
		SelfRefs:  map[syntax.Node]bool{},
	}}
	w.types = &types.Scope{Opaque: map[string]bool{}, Named: map[string]*types.Type{}}
	if names.Types != nil {
		for k, v := range names.Types.Opaque {
			w.types.Opaque[k] = v
		}
		for k, v := range names.Types.Named {
			w.types.Named[k] = v
		}
	}
	outer := &scope{names: map[string]*Binding{}, vars: names.Vars}
	s := &scope{parent: outer, names: map[string]*Binding{}}
	// The const lines and the type lines, in the order of the text.
	ci, ti := 0, 0
	for ci < len(prog.Consts) || ti < len(prog.Types) {
		if ti < len(prog.Types) && (ci >= len(prog.Consts) || prog.Types[ti].Span.Start < prog.Consts[ci].Span().Start) {
			w.typeLine(prog.Types[ti])
			ti++
			continue
		}
		w.constLine(prog.Consts[ci], s)
		ci++
	}
	w.expr(prog.Result, s)
	// recursion-needs-types, on the first reference of each lambda to its
	// own name, unless every parameter and the result carry a type
	// (01-language.md 14.4; 05-types.md 22.5; F17 11.2).
	for l, r := range w.self {
		if !l.Typed() {
			w.diagm("recursion-needs-types", r.sp, "", nameP(r.name))
		}
	}
	sort.Slice(w.res.Diags, func(i, j int) bool {
		a, b := w.res.Diags[i], w.res.Diags[j]
		if a.Span.Start != b.Span.Start {
			return a.Span.Start < b.Span.Start
		}
		if a.Span.End != b.Span.End {
			return a.Span.End < b.Span.End
		}
		return a.Code < b.Code
	})
	return w.res
}

type diagKey struct {
	code string
	sp   syntax.Span
}

func (w *walker) diag(code string, sp syntax.Span) { w.diagm(code, sp, "") }

// diagm records a diagnostic with its message's template and parameters.
func (w *walker) diagm(code string, sp syntax.Span, tmpl string, ps ...messages.Param) {
	k := diagKey{code, sp}
	if !w.seen[k] {
		w.seen[k] = true
		w.res.Diags = append(w.res.Diags, Diag{Code: code, Span: sp, Tmpl: tmpl, Params: ps})
	}
}

func nameP(n string) messages.Param { return messages.Word("name", n) }

// bind binds a name in scope s (22.7). It returns the slot, or -1.
func (w *walker) bind(name string, sp syntax.Span, s *scope, kind BindKind, lam *syntax.Lambda) int {
	if name == "" || name == "_" {
		return -1
	}
	if w.names.Namespaces[name] {
		w.diagm("shadows-namespace", sp, "", nameP(name))
		return -1
	}
	if _, ok := s.names[name]; ok {
		w.diagm("duplicate-name", sp, "", nameP(name))
	}
	b := &Binding{Kind: kind, Name: name, Lambda: lam, Slot: -1}
	if kind == Value || kind == FnParam {
		b.Slot = w.res.NSlots
		w.res.NSlots++
	}
	s.names[name] = b
	w.last = b
	return b.Slot
}

func (w *walker) constLine(c *syntax.Const, s *scope) {
	if lam, ok := c.Value.(*syntax.Lambda); ok && c.Type == nil {
		if np, ok := c.Pattern.(*syntax.NamePat); ok && !np.Discard {
			// The name is bound first: it is in scope in the lambda's body
			// and defaults (01-language.md 5 rule 6; 22.7).
			w.bind(np.Name, np.Span(), s, ConstLambda, lam)
			w.lambda(lam, s)
			return
		}
	}
	w.annotation(c.Type)
	w.expr(c.Value, s)
	w.pattern(c.Pattern, s)
}

// typeLine walks a type line of package F16 (3.1): name-clash with a type
// name of the environment, duplicate-name with an earlier type line, and
// the static rules of F4 6.1 for its type. A line declares its name for the
// rest of the program when neither name-clash nor a diagnostic of its type
// stands; of two lines with one name, the later stands.
func (w *walker) typeLine(tl *syntax.TypeLine) {
	clash := false
	if env := w.names.Types; env != nil {
		_, named := env.Named[tl.Name]
		clash = named || env.Opaque[tl.Name]
	}
	switch {
	case clash:
		w.diagm("name-clash", tl.NameSpan, "", nameP(tl.Name))
	case w.lineNames[tl.Name]:
		w.diagm("duplicate-name", tl.NameSpan, "", nameP(tl.Name))
	}
	if w.lineNames == nil {
		w.lineNames = map[string]bool{}
	}
	w.lineNames[tl.Name] = true
	t := w.resolve(tl.Type)
	if clash {
		return
	}
	if t != nil {
		w.types.Named[tl.Name] = t
	} else {
		delete(w.types.Named, tl.Name)
	}
}

// resolve resolves a program type with the static rules of F4 6.1, in the
// type names in scope, and records its type when it has no diagnostic.
func (w *walker) resolve(pt *syntax.PType) *types.Type {
	t, ds := types.FromPType(pt, w.types)
	for _, d := range ds {
		switch d.Code {
		case "undefined-name":
			w.diagm(d.Code, d.Span, "type", nameP(d.Name))
		case "check-limit":
			w.diagm(d.Code, d.Span, d.Tmpl)
		default:
			w.diag(d.Code, d.Span)
		}
	}
	if t != nil {
		w.res.AnnTypes[pt] = t
	}
	return t
}

// annotation resolves the program type of an annotation, if there is one.
func (w *walker) annotation(pt *syntax.PType) {
	if pt != nil {
		w.resolve(pt)
	}
}

// selfRef records a reference of a const-bound lambda to its own name: a
// reference whose binding is the lambda, inside the lambda's own text.
func (w *walker) selfRef(n syntax.Node, b *Binding) {
	if b.Lambda == nil || !within(n.Span(), b.Lambda.Span()) {
		return
	}
	w.res.SelfRefs[n] = true
	sp := n.Span()
	if c, ok := n.(*syntax.Call); ok {
		sp = c.NameSpan
	}
	if w.self == nil {
		w.self = map[*syntax.Lambda]selfRef{}
	}
	if r, ok := w.self[b.Lambda]; !ok || sp.Start < r.sp.Start {
		w.self[b.Lambda] = selfRef{name: b.Name, sp: sp}
	}
}

func within(a, b syntax.Span) bool { return a.Start >= b.Start && a.End <= b.End }

func (w *walker) pattern(p syntax.Pattern, s *scope) {
	switch p := p.(type) {
	case *syntax.NamePat:
		if slot := w.bind(p.Name, p.Span(), s, Value, nil); slot >= 0 {
			w.res.Slots[p] = slot
		}
	case *syntax.MapPat:
		for _, f := range p.Fields {
			if f.Default != nil {
				w.expr(f.Default, s)
			}
			if slot := w.bind(f.Name, f.NameSpan, s, Value, nil); slot >= 0 {
				w.res.Slots[f] = slot
			}
		}
		if p.Rest != nil {
			w.pattern(p.Rest, s)
		}
	case *syntax.ListPat:
		for _, e := range p.Elems {
			if e.Default != nil {
				w.expr(e.Default, s)
			}
			w.pattern(e.Name, s)
		}
		if p.Rest != nil {
			w.pattern(p.Rest, s)
		}
	}
}

func (w *walker) lambda(l *syntax.Lambda, s *scope) {
	ls := &scope{parent: s, names: map[string]*Binding{}}
	for _, p := range l.Params {
		w.annotation(p.Type)
		if p.Fn != nil {
			for _, q := range p.Fn.Params {
				w.annotation(q.Type)
			}
			w.annotation(p.Fn.Result)
		}
		if p.Default != nil {
			w.expr(p.Default, ls)
		}
		np, isName := p.Pattern.(*syntax.NamePat)
		if p.Fn != nil && isName {
			// A parameter with a function type is a function binding, with
			// a slot for the lambda passed (f17-recursion.md 13.2).
			w.last = nil
			if slot := w.bind(np.Name, np.Span(), ls, FnParam, nil); slot >= 0 {
				w.res.Slots[np] = slot
				w.last.Param = p
			}
			continue
		}
		w.last = nil
		w.pattern(p.Pattern, ls)
		if isName && w.last != nil {
			w.last.Param = p
		}
	}
	w.annotation(l.Result)
	w.expr(l.Body, ls)
}

// branch walks a branch, a block or a body in a new scope.
func (w *walker) branch(n syntax.Node, s *scope) {
	if b, ok := n.(*syntax.Block); ok {
		bs := &scope{parent: s, names: map[string]*Binding{}}
		for _, c := range b.Consts {
			w.constLine(c, bs)
		}
		w.expr(b.Result, bs)
		return
	}
	w.expr(n, s)
}

// isLambdaArg reports whether an argument is a lambda argument (22.4; with
// F17 13.1, also the name of a parameter with a function type), and its
// count of parameters: for a parameter with a function type, the count its
// function type lists.
func (w *walker) isLambdaArg(a syntax.Node, s *scope) (bool, int) {
	switch a := a.(type) {
	case *syntax.Lambda:
		return true, len(a.Params)
	case *syntax.Ident:
		b := s.lookup(a.Name)
		switch {
		case b != nil && b.Kind == ConstLambda:
			w.res.Idents[a] = b
			w.selfRef(a, b)
			return true, len(b.Lambda.Params)
		case b != nil && b.Kind == FnParam:
			w.res.Idents[a] = b
			return true, len(b.Param.Fn.Params)
		}
	}
	return false, 0
}

// HigherOrder reports whether a lambda has a parameter with a function type
// (f17-recursion.md 13.1): it cannot be passed as a lambda argument.
func HigherOrder(l *syntax.Lambda) bool {
	if l == nil {
		return false
	}
	for _, p := range l.Params {
		if p.Fn != nil {
			return true
		}
	}
	return false
}

// higherOrderArg reports whether a lambda argument is a lambda with a
// parameter with a function type: a Lambda node, or the name of a
// const-bound lambda (f17-recursion.md 13.2 rule 8).
func (w *walker) higherOrderArg(a syntax.Node) bool {
	switch a := a.(type) {
	case *syntax.Lambda:
		return HigherOrder(a)
	case *syntax.Ident:
		if b := w.res.Idents[a]; b != nil && b.Kind == ConstLambda {
			return HigherOrder(b.Lambda)
		}
	}
	return false
}

// args walks the arguments of a Call or a MethodCall. A lambda argument
// that has a parameter with a function type is lambda-to-lambda wherever it
// stands, whatever the callee (f17-recursion.md 13.2 rule 8); name is the
// callee as written.
func (w *walker) args(name string, args []syntax.Node, s *scope) {
	for _, a := range args {
		if ok, _ := w.isLambdaArg(a, s); ok {
			if w.higherOrderArg(a) {
				w.diagm("lambda-to-lambda", a.Span(), "higher-order", messages.Word("name", name))
			}
			if l, isL := a.(*syntax.Lambda); isL {
				w.lambda(l, s)
			}
			continue
		}
		w.expr(a, s)
	}
}

// lambdaCounts are the counts of a const-bound lambda (22.3).
func lambdaCounts(l *syntax.Lambda) (min, max int) {
	for i, p := range l.Params {
		if p.Default == nil {
			min = i + 1
		}
	}
	return min, len(l.Params)
}

func (w *walker) call(n syntax.Node, ns, name string, nameSpan syntax.Span, args []syntax.Node, s *scope) {
	callee := &Callee{}
	switch {
	case ns != "":
		if f, ok := w.names.Funcs[ns+"."+name]; ok {
			callee = &Callee{Kind: Host, Host: f}
		} else if e, ok := w.names.Library[ns+"."+name]; ok { // F6 2.5
			callee = &Callee{Kind: Lib, Lib: e}
		} else {
			w.diagm("undefined-function", nameSpan, "", messages.Word("name", ns+"."+name))
		}
	default:
		if b := s.lookup(name); b != nil {
			switch b.Kind {
			case ConstLambda:
				callee = &Callee{Kind: Lambda, Lambda: b.Lambda}
				w.selfRef(n, b)
			case FnParam:
				// A parameter with a function type may be called
				// (f17-recursion.md 13.2 rule 1).
				callee = &Callee{Kind: Param, Binding: b}
			default:
				tmpl := ""
				if b.HintParam() {
					tmpl = "fn-hint"
				}
				w.diagm("call-of-parameter", nameSpan, tmpl, messages.Word("name", name))
			}
		} else if f, ok := w.names.Funcs[name]; ok {
			callee = &Callee{Kind: Host, Host: f}
		} else if e, ok := w.names.Library[name]; ok {
			callee = &Callee{Kind: Lib, Lib: e}
		} else {
			w.diagm("undefined-function", nameSpan, "", messages.Word("name", name))
		}
	}
	w.res.Calls[n] = callee
	full := name
	if ns != "" {
		full = ns + "." + name
	}
	if callee.Kind != Unknown {
		var min, max int
		switch callee.Kind {
		case Lambda:
			min, max = lambdaCounts(callee.Lambda)
		case Param:
			min = len(callee.Binding.Param.Fn.Params)
			max = min
		case Host:
			min, max = callee.Host.Sig.MinArgs(), callee.Host.Sig.MaxArgs()
		case Lib:
			min, max = callee.Lib.MinArgs(), callee.Lib.MaxArgs()
		}
		if len(args) < min || (max >= 0 && len(args) > max) {
			w.diagm("arity", n.Span(), ArityTemplate(min, max), messages.Word("name", full),
				messages.Word("count", strconv.Itoa(len(args))), messages.Word("required", strconv.Itoa(min)),
				messages.Word("most", strconv.Itoa(max)))
		} else {
			for i, a := range args {
				w.argTest(callee, full, i, a, s)
			}
		}
	}
	w.args(full, args, s)
}

// argTest applies the tests of 22.5 to argument i of a call whose count is
// right, with those of f17-recursion.md 13.2 for a parameter with a
// function type.
func (w *walker) argTest(callee *Callee, full string, i int, a syntax.Node, s *scope) {
	ok, nparams := w.isLambdaArg(a, s)
	if ok && w.higherOrderArg(a) {
		return // rule 8 (args) decides this argument
	}
	switch callee.Kind {
	case Lambda:
		p := callee.Lambda.Params[i]
		switch {
		case p.Fn == nil:
			if ok {
				w.diagm("lambda-to-lambda", a.Span(), "", messages.Word("name", full))
			}
		case !ok:
			// A value at a parameter with a function type (rule 3).
			pn := ""
			if np, isName := p.Pattern.(*syntax.NamePat); isName {
				pn = np.Name
			}
			if id, isID := a.(*syntax.Ident); isID {
				if b := s.lookup(id.Name); b.HintParam() {
					w.diagm("arg-mismatch", a.Span(), "parameter-at-fn", messages.Word("name", id.Name))
					return
				}
			}
			w.diagm("arg-mismatch", a.Span(), "function", messages.Word("param", pn), messages.Word("name", full))
		case nparams > len(p.Fn.Params):
			// rule 9
			w.diagm("arity", a.Span(), "parameters", messages.Word("name", full),
				messages.Word("most", strconv.Itoa(len(p.Fn.Params))))
		}
	case Param:
		// A function type's parameters are never functions: each takes a
		// value, as a parameter of a function that is not an Fn does.
		if ok {
			w.diagm("not-a-value", a.Span(), "argument", messages.Word("name", full),
				messages.Word("param", callee.Binding.Param.Fn.Params[i].Name))
		}
	case Host:
		if ok {
			// A host signature has no Fn parameter (bad-signature).
			w.diagm("not-a-value", a.Span(), "argument", messages.Word("name", full),
				messages.Word("param", hostParam(callee.Host, i)))
		}
	case Lib:
		if !ok {
			return
		}
		p := callee.Lib.ParamAt(i)
		if p == nil || p.Callback < 0 {
			pn := ""
			if p != nil {
				pn = p.Name
			}
			w.diagm("not-a-value", a.Span(), "argument", messages.Word("name", full), messages.Word("param", pn))
		} else if nparams > p.Callback {
			w.diagm("arity", a.Span(), "parameters", messages.Word("name", full),
				messages.Word("most", strconv.Itoa(p.Callback)))
		}
	}
}

func (w *walker) expr(n syntax.Node, s *scope) {
	switch n := n.(type) {
	case nil:
	case *syntax.Ident:
		b := s.lookup(n.Name)
		switch {
		case b != nil && b.Kind == ConstLambda:
			w.res.Idents[n] = b
			w.diagm("not-a-value", n.Span(), "lambda", nameP(n.Name))
		case b != nil && b.Kind == FnParam:
			// A parameter with a function type is not a value
			// (f17-recursion.md 13.2 rule 4).
			w.res.Idents[n] = b
			w.diagm("not-a-value", n.Span(), "function", nameP(n.Name))
		case b != nil:
			w.res.Idents[n] = b
		case w.names.Namespaces[n.Name]:
			w.diagm("not-a-value", n.Span(), "namespace", nameP(n.Name))
		case w.isFunction(n.Name):
			w.diagm("not-a-value", n.Span(), "function", nameP(n.Name))
		default:
			w.diagm("undefined-name", n.Span(), "name", nameP(n.Name))
		}
	case *syntax.Call:
		w.call(n, "", n.Name, n.NameSpan, n.Args, s)
	case *syntax.MethodCall:
		if id, ok := n.X.(*syntax.Ident); ok && !n.Optional && w.names.Namespaces[id.Name] {
			w.res.NSCalls[n] = true
			w.call(n, id.Name, n.Name, syntax.Span{Start: id.Span().Start, End: n.NameSpan.End}, n.Args, s)
			return
		}
		w.chainBase(n.X, s)
		w.args(n.Name, n.Args, s)
	case *syntax.Member:
		if w.constant(n) {
			return
		}
		w.chainBase(n.X, s)
	case *syntax.Index:
		w.chainBase(n.X, s)
		w.expr(n.Index, s)
	case *syntax.OptChain:
		w.expr(n.X, s)
	case *syntax.Block:
		w.branch(n, s)
	case *syntax.Cond:
		// An else chain is walked by a loop.
		for {
			w.expr(n.Test, s)
			w.branch(n.Then, s)
			next, ok := n.Else.(*syntax.Cond)
			if !ok {
				w.branch(n.Else, s)
				return
			}
			n = next
		}
	case *syntax.As:
		w.expr(n.X, s)
		if t := w.resolve(n.Type); t != nil {
			w.res.AsTypes[n] = t
		}
	case *syntax.Try:
		w.branch(n.Body, s)
		hs := &scope{parent: s, names: map[string]*Binding{}}
		if n.Name != "" {
			if slot := w.bind(n.Name, n.NameSpan, hs, Value, nil); slot >= 0 {
				w.res.Slots[n] = slot
			}
		}
		if b, ok := n.Handler.(*syntax.Block); ok {
			for _, c := range b.Consts {
				w.constLine(c, hs)
			}
			w.expr(b.Result, hs)
		} else {
			w.expr(n.Handler, hs)
		}
	case *syntax.Lambda:
		// A lambda where no argument may stand cannot parse; walk it anyway.
		w.lambda(n, s)
	case *syntax.Map:
		keys := map[string]bool{}
		for _, e := range n.Entries {
			switch e := e.(type) {
			case *syntax.Entry:
				if e.KeyForm == syntax.KeyComputed {
					w.expr(e.KeyExpr, s)
				} else {
					if keys[e.Key] {
						w.diagm("duplicate-key", e.KeySpan, "", messages.Val("key", e.Key))
					}
					keys[e.Key] = true
				}
				w.expr(e.Value, s)
			case *syntax.Spread:
				w.expr(e.Value, s)
			}
		}
	case *syntax.Match:
		w.expr(n.Subject, s)
		// lits holds the literals of the plain arms so far. A literal of a
		// guarded arm is compared with them and with the earlier literals
		// of its own arm, and is not added (F17 4.1).
		var lits []syntax.Node
		sawDefault := false
		for _, a := range n.Arms {
			if sawDefault {
				w.diag("unreachable-arm", a.Span())
			}
			plain := len(lits)
			for _, l := range a.Literals {
				for _, prev := range lits {
					if litEqual(prev, l) {
						w.diag("duplicate-case", l.Span())
						break
					}
				}
				lits = append(lits, l)
			}
			if a.Guard != nil {
				lits = lits[:plain]
				w.expr(a.Guard, s)
			} else if a.Default {
				sawDefault = true
			}
			w.expr(a.Result, s)
		}
	case *syntax.Binary:
		if n.Op == "**" {
			w.expr(n.X, s)
			w.expr(n.Y, s)
			return
		}
		// A left-associative chain is walked by a loop over its left spine.
		var rights []syntax.Node
		var x syntax.Node = n
		for {
			b, ok := x.(*syntax.Binary)
			if !ok || b.Op == "**" {
				break
			}
			w.coalesce(b)
			rights = append(rights, b.Y)
			x = b.X
		}
		w.expr(x, s)
		for i := len(rights) - 1; i >= 0; i-- {
			w.expr(rights[i], s)
		}
	default:
		for _, c := range syntax.Children(n) {
			w.expr(c, s)
		}
	}
}

// coalesce marks the operands of a ?? node that are not the last operand
// of its chain (01-language.md 6.6): its left operand always, and its right
// operand when the node itself is not last. The left spine is visited from
// the outermost node in, so a node is marked before it is visited.
func (w *walker) coalesce(b *syntax.Binary) {
	if b.Op != "??" {
		return
	}
	w.notLastOperand(b.X)
	if w.notLast[b] {
		w.notLastOperand(b.Y)
	}
}

// notLastOperand records an operand of a ?? chain other than its last: a
// ?? node, whose own operands are then not last either, or a coalesced
// read, by the node of its last step.
func (w *walker) notLastOperand(n syntax.Node) {
	if b, ok := n.(*syntax.Binary); ok {
		if b.Op == "??" {
			if w.notLast == nil {
				w.notLast = map[*syntax.Binary]bool{}
			}
			w.notLast[b] = true
		}
		return
	}
	if step := syntax.CoalescedStep(n); step != nil {
		w.res.Coalesced[step] = true
	}
}

// chainBase walks the receiver of a step. A chain is walked by a loop.
func (w *walker) chainBase(x syntax.Node, s *scope) {
	// Collect the steps down to the base, then walk from the base out.
	var steps []syntax.Node
	for {
		switch n := x.(type) {
		case *syntax.Member:
			if w.constant(n) {
				break
			}
			steps = append(steps, n)
			x = n.X
			continue
		case *syntax.Index:
			steps = append(steps, n)
			x = n.X
			continue
		case *syntax.MethodCall:
			if id, ok := n.X.(*syntax.Ident); ok && !n.Optional && w.names.Namespaces[id.Name] {
				break
			}
			steps = append(steps, n)
			x = n.X
			continue
		}
		break
	}
	w.expr(x, s)
	for i := len(steps) - 1; i >= 0; i-- {
		switch n := steps[i].(type) {
		case *syntax.Index:
			w.expr(n.Index, s)
		case *syntax.MethodCall:
			w.args(n.Name, n.Args, s)
		}
	}
}

// constant resolves a Member that reads a constant of a group that is on
// (F6 2.4 item 2): a plain "." on an Ident that names the group.
func (w *walker) constant(n *syntax.Member) bool {
	if n.Optional || len(w.names.Constants) == 0 {
		return false
	}
	id, ok := n.X.(*syntax.Ident)
	if !ok || !w.names.Namespaces[id.Name] {
		return false
	}
	v, ok := w.names.Constants[id.Name+"."+n.Name]
	if !ok {
		return false
	}
	if w.res.Consts == nil {
		w.res.Consts = map[*syntax.Member]float64{}
	}
	w.res.Consts[n] = v
	return true
}

func (w *walker) isFunction(name string) bool {
	if f, ok := w.names.Funcs[name]; ok && f.Namespace == "" {
		return true
	}
	_, ok := w.names.Library[name]
	return ok
}

func litEqual(a, b syntax.Node) bool {
	switch a := a.(type) {
	case *syntax.Number:
		b, ok := b.(*syntax.Number)
		return ok && a.Value == b.Value
	case *syntax.String:
		b, ok := b.(*syntax.String)
		return ok && a.Value == b.Value
	case *syntax.Bool:
		b, ok := b.(*syntax.Bool)
		return ok && a.Value == b.Value
	case *syntax.Null:
		_, ok := b.(*syntax.Null)
		return ok
	}
	return false
}

// KindOfLiteral is used by the evaluator: the kind of a match literal.
func KindOfLiteral(n syntax.Node) value.Kind {
	switch n.(type) {
	case *syntax.Number:
		return value.Number
	case *syntax.String:
		return value.String
	case *syntax.Bool:
		return value.Bool
	}
	return value.Null
}

// ArityTemplate chooses the template of arity by the counts (F8 3.5).
func ArityTemplate(min, max int) string {
	switch {
	case max < 0:
		return "at-least"
	case max == min:
		return "exact"
	}
	return "range"
}

// hostParam is the name of a host function's parameter at argument i.
func hostParam(f *Func, i int) string {
	ps := f.Sig.Params
	if i < len(ps) {
		return ps[i].Name
	}
	if n := len(ps); n > 0 && ps[n-1].Rest {
		return ps[n-1].Name
	}
	return ""
}
