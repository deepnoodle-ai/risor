package risor

import (
	"fmt"
	"sort"

	"github.com/deepnoodle-ai/risor/go/v3/internal/check"
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/typetext"
)

// CheckOption is an option of Check and Compile: the expected result
// (03-host.md 3.2.3), or NoCheck.
type CheckOption interface{ checkOption() }

type expectOption struct {
	text      string
	condition bool
}

func (expectOption) checkOption() {}

type noCheck struct{}

func (noCheck) checkOption() {}

// Expect states the expected type of the result, in type text. The result
// must fit it through its form (05-types.md 15).
func Expect(typeText string) CheckOption { return expectOption{text: typeText} }

// ExpectSchema gives the expected result by a JSON Schema (package F10
// 2.1): its type is the schema's mapping with the options given. Only
// Closed is accepted; AbsentIsNull and DefaultsApplied, and any error of
// the schema, are the declare error bad-type. A compile without a check
// still maps the schema.
func ExpectSchema(schema any, opts ...SchemaOption) CheckOption {
	return expectSchemaOption{schema: schema, opts: opts}
}

// ExpectType gives the expected result as a type: from TypeOf,
// TypeFromSchema or Env.ParseType.
func ExpectType(t Type) CheckOption { return expectTypeOption{t: t} }

type expectSchemaOption struct {
	schema any
	opts   []SchemaOption
}

func (expectSchemaOption) checkOption() {}

type expectTypeOption struct{ t Type }

func (expectTypeOption) checkOption() {}

// resultSchemaType maps a schema given as the expected result (F10 2.1).
func resultSchemaType(schema any, opts []SchemaOption) (*types.Type, error) {
	for _, o := range opts {
		if o == AbsentIsNull || o == DefaultsApplied {
			return nil, fmt.Errorf("the option %s does not apply to an expected result", string(o))
		}
	}
	return mapSchema(schema, opts)
}

// Condition states that the result will be tested by truthiness (15.3).
var Condition CheckOption = expectOption{condition: true}

// NoCheck compiles without the type rules: the static rules still apply
// (03-host.md 3.2.1 item 4), and a run failure has the class of a program
// compiled without a check (3.7.6).
var NoCheck CheckOption = noCheck{}

// Site is a place where a checked program can still fail on a value
// (05-types.md 18.3).
type Site struct {
	Class string
	Span  Span
}

// CheckResult is what a check gives for a program that checks.
type CheckResult struct {
	// Type is the result type, the checker's own. Its String method prints
	// it by 05-types.md 3.9.
	Type Type
	// Sites are the places where the program can still fail on a value.
	Sites []Site
}

// typeEnv converts the type text of the environment's declarations into
// types (05-types.md 19), reporting the errors that need the checker
// (19.5, second table).
func (e *Env) typeEnv() {
	// A child whose declarations leave the scope of type names as its
	// parent's shares the parent's types of every inherited declaration.
	p := e.parent
	if p != nil && !e.scopeChanged() {
		e.opaqueSet = p.opaqueSet
	} else {
		p = nil
		opaque := make(map[string]bool, len(e.opaque))
		for n := range e.opaque {
			opaque[n] = true
		}
		e.opaqueSet = opaque
	}
	opaque := e.opaqueSet
	sc := e.scope()
	e.varTypes = make(map[string]*types.Type, len(e.vars))
	names := make([]string, 0, 8)
	for n, d := range e.vars {
		if p != nil && p.vars[n] == d {
			if t, ok := p.varTypes[n]; ok {
				e.varTypes[n] = t
				continue
			}
		}
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if t := e.vars[n].ty; t != nil {
			if name, ok := undeclaredOpaque(t, opaque); !ok {
				e.errf(diag.CodeBadType, fmt.Sprintf("variable %s: %s is not a declared opaque type", n, name))
				continue
			}
			e.varTypes[n] = t
			continue
		}
		t, err := types.FromTextIn(e.vars[n].typ, sc, false)
		if err != nil {
			e.errf(diag.CodeBadType, fmt.Sprintf("variable %s: %v", n, err))
			continue
		}
		e.varTypes[n] = t
	}
	e.hostSigs = map[string]*types.Sig{}
	fnames := make([]string, 0, len(e.funcs))
	for n := range e.funcs {
		fnames = append(fnames, n)
	}
	sort.Strings(fnames)
	for _, n := range fnames {
		if p != nil && p.funcs[n] == e.funcs[n] {
			if s, ok := p.hostSigs[n]; ok {
				e.hostSigs[n] = s
				continue
			}
		}
		s, err := types.FromSigIn(nil, e.funcs[n].sig, sc)
		if err != nil {
			e.errf(diag.CodeBadSignature, fmt.Sprintf("function %s: %v", n, err))
			continue
		}
		e.hostSigs[n] = s
	}
	e.methodSigs = map[string]map[string]*types.Sig{}
	for tn, o := range e.opaque {
		if p != nil && p.opaque[tn] == o {
			if ms, ok := p.methodSigs[tn]; ok {
				e.methodSigs[tn] = ms
				continue
			}
		}
		ms := map[string]*types.Sig{}
		for m, f := range o.methods {
			s, err := types.FromSigIn(&typetext.Alt{Prim: &typetext.Prim{Kind: typetext.PrimName, Text: tn}}, f.sig, sc)
			if err != nil {
				e.errf(diag.CodeBadSignature, fmt.Sprintf("method %s.%s: %v", tn, m, err))
				continue
			}
			ms[m] = s
		}
		e.methodSigs[tn] = ms
	}
}

// expected reads the expected result of the options.
func (e *Env) expected(opts []CheckOption) (check.Expect, bool, error) {
	exp := check.Expect{}
	checked := true
	for _, o := range opts {
		switch o := o.(type) {
		case noCheck:
			checked = false
		case expectOption:
			if o.condition {
				exp = check.Expect{Kind: check.ExpectCondition}
				continue
			}
			tt, err := typetext.ParseType(o.text)
			var t *types.Type
			if err == nil {
				t, err = types.FromTextIn(tt, e.scope(), false)
			}
			if err != nil {
				return exp, checked, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType,
					Message: "the expected result: " + err.Error()}}}
			}
			exp = check.Expect{Kind: check.ExpectType, Type: t}
		case expectSchemaOption:
			t, err := resultSchemaType(o.schema, o.opts)
			if err != nil {
				return exp, checked, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType,
					Message: "the expected result: " + err.Error()}}}
			}
			exp = check.Expect{Kind: check.ExpectType, Type: t}
		case expectTypeOption:
			if p := o.t.problem(); p != "" {
				return exp, checked, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType,
					Message: "the expected result: " + p}}}
			}
			exp = check.Expect{Kind: check.ExpectType, Type: o.t.t}
		}
	}
	return exp, checked, nil
}

func (e *Env) checkEnv(names *static.Names, varNames []string) *check.Env {
	ce := &check.Env{Hosts: e.hostSigs, Methods: e.methodSigs, Reject: map[string]bool{}, Removed: e.removed,
		Facts: e.factIndexes(varNames)}
	for _, n := range varNames {
		ce.VarTypes = append(ce.VarTypes, e.varTypes[n])
	}
	rej := e.reject
	if rej == nil {
		rej = []string{"constant-condition"}
	}
	for _, r := range rej {
		ce.Reject[r] = true
	}
	return ce
}

// analyze applies the static rules and, when checked, the type rules. It
// returns the names' resolution, the check's result (nil when not checked)
// and the check's failure.
func (e *Env) analyze(x *Expr, names *static.Names, varNames []string, opts []CheckOption) (*static.Result, *check.Result, error) {
	exp, checked, err := e.expected(opts)
	if err != nil {
		return nil, nil, err
	}
	res := static.Check(x.tree, names)
	var ds []Diagnostic
	// The message is built only when the check fails (package F8 3.5).
	add := func(code string, s, end int, class, tmpl string, ps []messages.Param) {
		sp := x.file.Span(s, end)
		ps = append(ps, messages.Word("text", syntax.Visible(x.file.Text(s, end))))
		ds = append(ds, Diagnostic{Phase: PhaseCheck, Code: code, Span: &sp, Class: class,
			Message: messages.Build(code, tmpl, ps...)})
	}
	for _, d := range res.Diags {
		add(d.Code, d.Span.Start, d.Span.End, "", d.Tmpl, d.Params)
	}
	var cr *check.Result
	if checked {
		cr = check.Check(x.tree, res, e.cached().checkEnv(e), exp)
		for _, d := range cr.Diags {
			add(d.Code, d.Span.Start, d.Span.End, d.Class, d.Tmpl, d.Params)
		}
	}
	if len(ds) > 0 {
		return nil, nil, &Error{Diagnostics: sortDiags(ds)}
	}
	return res, cr, nil
}

// sortDiags makes the check diagnostics a set, ordered by start, end and
// code (05-types.md 6.1).
func sortDiags(ds []Diagnostic) []Diagnostic {
	seen := map[[5]any]bool{}
	var out []Diagnostic
	for _, d := range ds {
		a := d.Span.Array()
		k := [5]any{a[0], a[1], a[2], a[3], d.Code}
		if !seen[k] {
			seen[k] = true
			out = append(out, d)
		}
	}
	less := func(a, b [4]int) int {
		for i := range a {
			if a[i] != b[i] {
				return a[i] - b[i]
			}
		}
		return 0
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Span.Array(), out[j].Span.Array()
		start := less([4]int{a[0], a[1]}, [4]int{b[0], b[1]})
		if start != 0 {
			return start < 0
		}
		end := less([4]int{a[2], a[3]}, [4]int{b[2], b[3]})
		if end != 0 {
			return end < 0
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// Check checks a program against the environment (03-host.md 3.2): its
// result type and its sites, or an *Error of phase check.
func (e *Env) Check(x *Expr, opts ...CheckOption) (*CheckResult, error) {
	names, varNames := e.staticNames()
	_, cr, err := e.analyze(x, names, varNames, opts)
	if err != nil {
		return nil, err
	}
	return checkResult(x, cr), nil
}

// checkResult is the binding's form of a check's result, or nil.
func checkResult(x *Expr, cr *check.Result) *CheckResult {
	if cr == nil {
		return nil
	}
	out := &CheckResult{Type: Type{t: cr.Type}}
	for _, s := range cr.Sites {
		out.Sites = append(out.Sites, Site{Class: s.Class, Span: x.file.Span(s.Span.Start, s.Span.End)})
	}
	return out
}

// dynamicSpans turns the check's dynamic spans into source spans.
func dynamicSpans(x *Expr, cr *check.Result) map[[4]int]bool {
	if cr == nil {
		return nil
	}
	out := map[[4]int]bool{}
	for sp := range cr.Dynamic {
		out[x.file.Span(sp.Start, sp.End).Array()] = true
	}
	return out
}

// hostSpans turns the check's host-site spans into source spans.
func hostSpans(x *Expr, cr *check.Result) map[[4]int]bool {
	if cr == nil {
		return nil
	}
	out := map[[4]int]bool{}
	for sp := range cr.Host {
		out[x.file.Span(sp.Start, sp.End).Array()] = true
	}
	return out
}

// valueSpans turns the check's value-site spans into source spans.
func valueSpans(x *Expr, cr *check.Result) map[[4]int]bool {
	if cr == nil {
		return nil
	}
	out := map[[4]int]bool{}
	for sp := range cr.Value {
		out[x.file.Span(sp.Start, sp.End).Array()] = true
	}
	return out
}

// checkedClass sets the class of a run failure in a checked program, by
// the checked column of 03-host.md 3.7.6.
func checkedClass(d *Diagnostic, dynamic, host, val map[[4]int]bool) {
	switch d.Code {
	case "function-error":
		// The mark ! is a promise (05-types.md 18.5).
		if d.Span == nil || !host[d.Span.Array()] {
			d.Class = "environment"
		}
	case "null-operand", "wrong-kind", "missing-field":
		if d.Span != nil && dynamic[d.Span.Array()] {
			d.Class = "dynamic"
		} else {
			d.Class = "environment"
		}
	case "no-match":
		d.Class = "environment"
	case "not-conforming":
		// F4 6.6: value at a value site, environment elsewhere.
		if d.Span != nil && val[d.Span.Array()] {
			d.Class = "value"
		} else {
			d.Class = "environment"
		}
	}
}

// envTypes are an environment's declarations as types.
type envTypes struct {
	opaqueSet  map[string]bool
	namedTypes map[string]*types.Type // the resolved named types (F4 3)
	varTypes   map[string]*types.Type
	hostSigs   map[string]*types.Sig
	methodSigs map[string]map[string]*types.Sig
}

// staticNames are the names the static rules read (05-types.md 22.2),
// made once for the environment.
func (e *Env) staticNames() (*static.Names, []string) {
	c := e.cached()
	return c.names, c.varNames
}
