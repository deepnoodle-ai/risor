package risor

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/typetext"
)

// This file is package F4, checked conversion
// (spec/packages/f04-conversion.md): named types and the conformance
// function. The operators as and is are in the parser, the checker and the
// evaluator.

// namedDecl is one named type (F4 3.1). An inherited one keeps the type it
// was resolved to in its parent (3.4 item 1).
type namedDecl struct {
	name string
	text *typetext.Type // by type text
	ty   *types.Type    // by schema or by Type, or once resolved
	err  string         // a schema's error, reported when the environment is built
	own  bool
}

// NamedType declares a named type by type text (F4 3.1): a name for the
// type, for as and is and in every type text of the environment. The text
// may name the opaque types and the named types declared before it.
func NamedType(name, text string) Decl {
	return declFunc(func(e *Env) {
		t, err := typetext.ParseType(text)
		if err != nil {
			e.errf(diag.CodeBadType, fmt.Sprintf("named type %s: %v", name, err))
			return
		}
		e.putNamed(&namedDecl{name: name, text: t})
	})
}

// NamedTypeSchema declares a named type by JSON Schema (F4 3.1 item 4),
// with the options of F1 4.
func NamedTypeSchema(name string, schema any, opts ...SchemaOption) Decl {
	return declFunc(func(e *Env) {
		t, err := mapSchema(schema, opts, name)
		d := &namedDecl{name: name, ty: t}
		if err != nil {
			d.err = err.Error()
		}
		e.putNamed(d)
	})
}

// NamedTypeOf declares a named type of a Type, from TypeOf, TypeFromSchema
// or ParseType.
func NamedTypeOf(name string, t Type) Decl {
	return declFunc(func(e *Env) {
		d := &namedDecl{name: name, ty: t.t}
		if p := t.problem(); p != "" {
			d.err = p
		}
		e.putNamed(d)
	})
}

func (e *Env) putNamed(d *namedDecl) {
	if !syntax.IsTypeName(d.name) {
		e.errf(diag.CodeBadName, fmt.Sprintf("%q is not a type name", d.name))
		return
	}
	if e.own["n:"+d.name] {
		e.errf(diag.CodeNameClash, fmt.Sprintf("named type %s is declared twice", d.name))
		return
	}
	e.own["n:"+d.name] = true
	d.own = true
	for i, x := range e.named {
		if x.name == d.name && !x.own {
			// A child's named type replaces the parent's (3.4 item 1).
			e.named = append(e.named[:i:i], e.named[i+1:]...)
			break
		}
	}
	e.named = append(e.named, d)
}

// namedEnv resolves the named types (F4 3.1, 3.2). An inherited named type
// keeps its type. This environment's own are one list (F17 12.2): each may
// name itself and every other of the list, and the names on a cycle are
// recursive. A type given by schema or by Type names none of them.
func (e *Env) namedEnv() {
	opaque := map[string]bool{}
	for n := range e.opaque {
		opaque[n] = true
	}
	e.namedTypes = map[string]*types.Type{}
	var own []*namedDecl
	for _, d := range e.named {
		if opaque[d.name] {
			e.errf(diag.CodeNameClash, fmt.Sprintf("%s is a named type and an opaque type", d.name))
			continue
		}
		if !d.own {
			e.namedTypes[d.name] = d.ty
			continue
		}
		own = append(own, d)
	}
	index := map[string]int{}
	names := make([]string, len(own))
	for i, d := range own {
		index[d.name] = i
		names[i] = d.name
	}
	// The uses of each item, and the errors of its own text.
	uses := make([][]types.Use, len(own))
	errs := make([]string, len(own))
	for i, d := range own {
		switch {
		case d.err != "":
			errs[i] = d.err
		case d.text != nil:
			types.TextUses(d.text, func(name string, guarded bool) {
				j, ok := index[name]
				if !ok {
					j = -1
					if _, inh := e.namedTypes[name]; !inh && !opaque[name] && errs[i] == "" {
						errs[i] = name + " is not a declared opaque type or named type"
					}
				}
				uses[i] = append(uses[i], types.Use{Target: j, Guarded: guarded})
			})
		}
	}
	info := types.AnalyzeList(uses)
	b := types.NewListBuild(info, names)
	b.BuildFn = func(i int) (*types.Type, bool) {
		d := own[i]
		if d.text == nil {
			t, err := e.checkGiven(d, opaque)
			if err != "" {
				errs[i] = err
				return nil, false
			}
			return t, true
		}
		var t *types.Type
		var err error
		limit := types.Guard(func() {
			t, err = types.FromTextList(d.text, &types.Scope{Opaque: opaque, Named: e.namedTypes},
				func(name string, guarded bool) (*types.Type, bool) {
					j, ok := index[name]
					if !ok {
						return nil, false
					}
					return b.Look(i, j, guarded), true
				})
		})
		switch {
		case limit:
			errs[i] = fmt.Sprintf("the type needs more than %d pairs of types to compare", types.MaxPairs)
			return nil, false
		case err != nil:
			errs[i] = err.Error()
			return nil, false
		}
		return t, true
	}
	for _, c := range info.Order {
		ok := true
		for _, i := range c {
			if info.Direct[i] && errs[i] == "" {
				errs[i] = "it refers to itself with no list, record field or map in between"
			}
			if errs[i] != "" {
				ok = false
			}
			for _, u := range uses[i] {
				if u.Target >= 0 && info.Comp[u.Target] != info.Comp[i] && b.Failed[u.Target] {
					ok = false
				}
			}
		}
		if ok && !b.Comp(c) {
			// A definition over the size limit once the references at its
			// top were replaced (12.2 item 4, 12.6).
			for _, i := range c {
				if b.Over[i] && errs[i] == "" {
					errs[i] = fmt.Sprintf("the type is larger than %d", types.MaxSize)
				}
			}
			ok = false
		}
		if ok {
			// 12.2 item 6: a reference of the component beside a type of
			// its kind.
			for _, i := range c {
				if n, bad := b.SharedIn(i); bad && errs[i] == "" {
					errs[i] = fmt.Sprintf("%s shares a kind with another type of its union; give the union a named type of its own", n.Name)
				}
			}
			for _, i := range c {
				if errs[i] != "" {
					ok = false
				}
			}
		} else {
			ok = false
		}
		if !ok {
			for _, i := range c {
				b.Failed[i] = true
			}
		}
	}
	for i, d := range own {
		if errs[i] != "" {
			e.errf(diag.CodeBadType, fmt.Sprintf("named type %s: %s", d.name, errs[i]))
		}
		if t := b.TypeOf(i); t != nil && !b.Failed[i] {
			e.namedTypes[d.name] = t
		}
	}
	// The child sees its resolved declarations as inherited ones.
	kept := e.named[:0:0]
	for _, d := range e.named {
		if t, ok := e.namedTypes[d.name]; ok && !opaque[d.name] {
			kept = append(kept, &namedDecl{name: d.name, ty: t})
		}
	}
	e.named = kept
}

// checkGiven tests a named type given by schema or by Type.
func (e *Env) checkGiven(d *namedDecl, opaque map[string]bool) (*types.Type, string) {
	if name, ok := undeclaredOpaque(d.ty, opaque); !ok {
		return nil, name + " is not a declared opaque type"
	}
	if types.Size(d.ty) > types.MaxSize {
		return nil, fmt.Sprintf("the type is larger than %d", types.MaxSize)
	}
	if types.Depth(d.ty) > types.MaxDepth {
		return nil, fmt.Sprintf("the type is deeper than %d", types.MaxDepth)
	}
	return d.ty, ""
}

// scope is what type text may name in the environment.
func (e *Env) scope() *types.Scope {
	opaque := e.opaqueSet
	if opaque == nil {
		opaque = map[string]bool{}
		for n := range e.opaque {
			opaque[n] = true
		}
	}
	return &types.Scope{Opaque: opaque, Named: e.namedTypes}
}

// ParseType resolves type text in the environment (F4 3.5, 7.1 item 4): a
// named type becomes the type it stands for. An error is the declare error
// bad-type.
func (e *Env) ParseType(text string) (Type, error) {
	tt, err := typetext.ParseType(text)
	var t *types.Type
	if err == nil {
		t, err = types.FromTextIn(tt, e.scope(), false)
	}
	if err != nil {
		return Type{}, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType, Message: err.Error()}}}
	}
	return Type{t: t}, nil
}

// ConformOption is an option of Conform and ConformAll.
type ConformOption interface{ applyConform(*conformConfig) }

type conformConfig struct{ steps int64 }

type conformSteps int64

func (n conformSteps) applyConform(c *conformConfig) { c.steps = int64(n) }

// ConformSteps sets the step limit of Conform and ConformAll (F4 7.3): n
// steps, or NoLimit for none. Any other number below 0 is read as 0. Without
// it the limit is the default step limit, 1,000,000, whatever the limits of
// the environment that resolved the type.
func ConformSteps(n int64) ConformOption { return conformSteps(n) }

func conformLimit(opts []ConformOption) int64 {
	c := conformConfig{steps: eval.DefaultLimits.Steps}
	for _, o := range opts {
		if o != nil {
			o.applyConform(&c)
		}
	}
	if c.steps < 0 && c.steps != NoLimit {
		c.steps = 0
	}
	return c.steps
}

// limitDiagnostic is the diagnostic of the conformance function at a limit:
// phase run, class limit, no path and no span.
func limitDiagnostic(code, msg string) Diagnostic {
	return Diagnostic{Phase: PhaseRun, Code: code, Class: "limit", Message: msg, Trace: []diag.TraceEntry{}}
}

// Conform tests a value against a type by the conformance walk of F4 4: the
// test x is T and x as T make. It returns nil when v conforms, else an
// *Error with one diagnostic: phase run, code not-conforming, class value,
// the path and the message of the first failure, and no span. It is not a
// run, but it charges the steps a run's walk charges, against the default
// step limit or the one ConformSteps gives: past it, the diagnostic is
// steps-exceeded, of class limit, with no path (F4 7.3). Host data that
// shares a list at many places is charged at each. A value nested past the
// default depth limit is depth-exceeded, of class limit, with no path (F17
// 12.7). It changes nothing.
func Conform(v any, t Type, opts ...ConformOption) error {
	if p := t.problem(); p != "" {
		return &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType, Message: p}}}
	}
	path, code, msg, ok := eval.ConformCode(v, t.t, conformLimit(opts))
	if ok {
		return nil
	}
	if code != "not-conforming" { // F17 12.7, F4 7.3: no path
		return &Error{Diagnostics: []Diagnostic{limitDiagnostic(code, msg)}}
	}
	return &Error{Diagnostics: []Diagnostic{{Phase: PhaseRun, Code: code, Class: "value",
		Message: msg, Path: path, Trace: []diag.TraceEntry{}}}}
}

// ConformAll reports every failure of v against t, at most max of them, in
// the order of package F8 6.2. A max less than 1 is read as 1. Each is a
// not-conforming diagnostic of class value with a path and no span; the
// list is empty exactly when v conforms, and its first diagnostic is the
// one Conform gives. It counts steps as Conform does (F8 6.4), under the
// same option: a limit ends the list with steps-exceeded or depth-exceeded,
// with no path. It changes nothing.
func ConformAll(v any, t Type, max int, opts ...ConformOption) []Diagnostic {
	if p := t.problem(); p != "" {
		return []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType, Message: p}}
	}
	fs := eval.ConformAll(v, t.t, max, conformLimit(opts))
	out := make([]Diagnostic, len(fs))
	for i, f := range fs {
		if f.Code != "not-conforming" {
			out[i] = limitDiagnostic(f.Code, f.Message)
			continue
		}
		out[i] = Diagnostic{Phase: PhaseRun, Code: f.Code, Class: "value", Message: f.Message,
			Path: f.Path, Trace: []diag.TraceEntry{}}
	}
	return out
}
