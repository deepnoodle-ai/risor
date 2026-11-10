// Package static applies the static rules (spec/core/05-types.md 22) and
// resolves every name of a program. The evaluator compiles from its result.
package static

import (
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"sort"
	"strconv"

	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
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
)

// Binding is one binding of a name.
type Binding struct {
	Kind   BindKind
	Name   string
	Slot   int // for Value
	Lambda *syntax.Lambda
	Var    int // for EnvVar
}

// CalleeKind is what a call calls.
type CalleeKind uint8

// The kinds of callee.
const (
	Unknown CalleeKind = iota
	Lambda
	Host
	Lib
)

// Callee is the callee of a Call, or of a MethodCall that calls a namespace.
type Callee struct {
	Kind   CalleeKind
	Lambda *syntax.Lambda
	Host   *Func
	Lib    *library.Entry
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
}

// Check applies the static rules to a program and resolves its names.
func Check(prog *syntax.Program, names *Names) *Result {
	w := &walker{names: names, seen: map[diagKey]bool{}, res: &Result{
		Idents:  map[*syntax.Ident]*Binding{},
		Calls:   map[syntax.Node]*Callee{},
		NSCalls: map[*syntax.MethodCall]bool{},
		Slots:   map[any]int{},
		AsTypes: map[*syntax.As]*types.Type{},
	}}
	outer := &scope{names: map[string]*Binding{}, vars: names.Vars}
	s := &scope{parent: outer, names: map[string]*Binding{}}
	for _, c := range prog.Consts {
		w.constLine(c, s)
	}
	w.expr(prog.Result, s)
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
	if kind == Value {
		b.Slot = w.res.NSlots
		w.res.NSlots++
	}
	s.names[name] = b
	return b.Slot
}

func (w *walker) constLine(c *syntax.Const, s *scope) {
	if lam, ok := c.Value.(*syntax.Lambda); ok {
		if np, ok := c.Pattern.(*syntax.NamePat); ok && !np.Discard {
			w.lambda(lam, s)
			w.bind(np.Name, np.Span(), s, ConstLambda, lam)
			return
		}
	}
	w.expr(c.Value, s)
	w.pattern(c.Pattern, s)
}

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
		if p.Default != nil {
			w.expr(p.Default, ls)
		}
		w.pattern(p.Pattern, ls)
	}
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

// isLambdaArg reports whether an argument is a lambda argument (22.4), and
// its count of parameters.
func (w *walker) isLambdaArg(a syntax.Node, s *scope) (bool, int) {
	switch a := a.(type) {
	case *syntax.Lambda:
		return true, len(a.Params)
	case *syntax.Ident:
		if b := s.lookup(a.Name); b != nil && b.Kind == ConstLambda {
			w.res.Idents[a] = b
			return true, len(b.Lambda.Params)
		}
	}
	return false, 0
}

func (w *walker) args(args []syntax.Node, s *scope) {
	for _, a := range args {
		if ok, _ := w.isLambdaArg(a, s); ok {
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
			if b.Kind == ConstLambda {
				callee = &Callee{Kind: Lambda, Lambda: b.Lambda}
			} else {
				w.diagm("call-of-parameter", nameSpan, "", messages.Word("name", name))
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
	if callee.Kind != Unknown {
		var min, max int
		switch callee.Kind {
		case Lambda:
			min, max = lambdaCounts(callee.Lambda)
		case Host:
			min, max = callee.Host.Sig.MinArgs(), callee.Host.Sig.MaxArgs()
		case Lib:
			min, max = callee.Lib.MinArgs(), callee.Lib.MaxArgs()
		}
		full := name
		if ns != "" {
			full = ns + "." + name
		}
		if len(args) < min || (max >= 0 && len(args) > max) {
			w.diagm("arity", n.Span(), ArityTemplate(min, max), messages.Word("name", full),
				messages.Word("count", strconv.Itoa(len(args))), messages.Word("required", strconv.Itoa(min)),
				messages.Word("most", strconv.Itoa(max)))
		} else {
			for i, a := range args {
				ok, nparams := w.isLambdaArg(a, s)
				if !ok {
					continue
				}
				switch callee.Kind {
				case Lambda:
					w.diagm("lambda-to-lambda", a.Span(), "", messages.Word("name", full))
				case Host:
					// A host signature has no Fn parameter (bad-signature).
					w.diagm("not-a-value", a.Span(), "argument", messages.Word("name", full),
						messages.Word("param", hostParam(callee.Host, i)))
				case Lib:
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
		}
	}
	w.args(args, s)
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
		w.args(n.Args, s)
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
		sc := w.names.Types
		if sc == nil {
			sc = &types.Scope{}
		}
		t, ds := types.FromPType(n.Type, sc)
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
		var lits []syntax.Node
		sawDefault := false
		for _, a := range n.Arms {
			if sawDefault {
				w.diag("unreachable-arm", a.Span())
			}
			for _, l := range a.Literals {
				for _, prev := range lits {
					if litEqual(prev, l) {
						w.diag("duplicate-case", l.Span())
						break
					}
				}
				lits = append(lits, l)
			}
			if a.Default {
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
			w.args(n.Args, s)
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
