package risor

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
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
		t, err := mapSchema(schema, opts)
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
		if t.t == nil {
			d.err = "no type"
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

// namedEnv resolves the named types in their order (F4 3.1, 3.2). An
// inherited named type keeps its type; this environment's own are resolved
// against the opaque types and the named types before them.
func (e *Env) namedEnv() {
	opaque := map[string]bool{}
	for n := range e.opaque {
		opaque[n] = true
	}
	e.namedTypes = map[string]*types.Type{}
	kept := e.named[:0:0]
	for _, d := range e.named {
		if opaque[d.name] {
			e.errf(diag.CodeNameClash, fmt.Sprintf("%s is a named type and an opaque type", d.name))
			continue
		}
		if !d.own {
			e.namedTypes[d.name] = d.ty
			kept = append(kept, d)
			continue
		}
		t, err := e.resolveNamed(d, opaque)
		if err != "" {
			e.errf(diag.CodeBadType, fmt.Sprintf("named type %s: %s", d.name, err))
			continue
		}
		nd := &namedDecl{name: d.name, ty: t}
		e.namedTypes[d.name] = t
		kept = append(kept, nd)
	}
	// The child sees its resolved declarations as inherited ones.
	e.named = kept
}

func (e *Env) resolveNamed(d *namedDecl, opaque map[string]bool) (*types.Type, string) {
	if d.err != "" {
		return nil, d.err
	}
	if d.text != nil {
		t, err := types.FromTextIn(d.text, &types.Scope{Opaque: opaque, Named: e.namedTypes}, false)
		if err != nil {
			return nil, err.Error()
		}
		return t, ""
	}
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
	return Type{t}, nil
}

// Conform tests a value against a type by the conformance walk of F4 4: the
// test x is T and x as T make. It returns nil when v conforms, else an
// *Error with one diagnostic: phase run, code not-conforming, class value,
// the path and the message of the first failure, and no span. It is not a
// run: it charges nothing and has no limits, and its work is bounded by the
// value and the type. It changes nothing.
func Conform(v any, t Type) error {
	if t.t == nil {
		return &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType, Message: "no type"}}}
	}
	path, msg, ok := eval.Conform(v, t.t)
	if ok {
		return nil
	}
	return &Error{Diagnostics: []Diagnostic{{Phase: PhaseRun, Code: "not-conforming", Class: "value",
		Message: msg, Path: path, Trace: []diag.TraceEntry{}}}}
}

// ConformAll reports every failure of v against t, at most max of them, in
// the order of package F8 6.2. A max less than 1 is read as 1. Each is a
// not-conforming diagnostic of class value with a path and no span; the
// list is empty exactly when v conforms, and its first diagnostic is the
// one Conform gives. It charges nothing and changes nothing.
func ConformAll(v any, t Type, max int) []Diagnostic {
	if t.t == nil {
		return []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType, Message: "no type"}}
	}
	fs := eval.ConformAll(v, t.t, max)
	out := make([]Diagnostic, len(fs))
	for i, f := range fs {
		out[i] = Diagnostic{Phase: PhaseRun, Code: "not-conforming", Class: "value", Message: f.Message,
			Path: f.Path, Trace: []diag.TraceEntry{}}
	}
	return out
}
