package risor

import (
	"context"
	"fmt"

	"github.com/deepnoodle-ai/risor/go/v3/internal/check"
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// This file is package F16, slice D2 (spec/packages/f16-entry-points.md 12):
// Entry metadata, Entries, the Call stage (positional and by record), and
// per-entry expected results.

// Entry describes one exported entry point of a program (F16 12.6).
type Entry struct {
	// Name is the const name of the entry point.
	Name string
	// Doc is the description attached to export, or empty when none.
	Doc string
	// Params are the entry point's parameters, in written order.
	Params []EntryParam
	// InputType is the input record type for CallRecord and SchemaOf, or
	// nil when the entry point has no input record (no-input-record).
	InputType *Type
	// ResultType is the entry point's checked result type, or nil for a
	// program compiled with NoCheck.
	ResultType *Type
}

// EntryParam describes one parameter of an entry point (F16 12.6).
type EntryParam struct {
	// Name is the parameter's name, or empty when its pattern is a
	// destructuring pattern or _.
	Name string
	// Type is the parameter's annotated type.
	Type Type
	// Doc is the parameter's description, or empty when none.
	Doc string
	// Optional reports whether the parameter has a default value.
	Optional bool
}

// entryMeta holds the internal metadata of one entry point on a Program.
type entryMeta struct {
	pub       Entry
	paramType []*types.Type // each parameter's resolved type
	hasDef    []bool        // whether each parameter has a default
	min, max  int
	inputType *types.Type // nil when no input record
	singleRec bool        // true for row 1 of the input record table (12.5)
}

// maxCallConform is the maximum number of argument conformance diagnostics
// returned by Call or CallRecord (F16 12.5).
const maxCallConform = 100

// isSingleRecord reports whether t (unfolding recursive references) is a
// record with no other alternative (not a tagged union, not nullable, not
// mixed; F16 12.5).
func isSingleRecord(t *types.Type) bool {
	if t == nil {
		return false
	}
	alts := types.Alternatives(t)
	return len(alts) == 1 && alts[0].Record() != nil && alts[0].Union() == nil
}

// buildEntries builds the entry point metadata for a checked or unchecked
// program, in text order (F16 12.5, 12.6).
func buildEntries(tree *syntax.Program, res *static.Result, cr *check.Result) ([]Entry, map[string]*entryMeta) {
	var list []Entry
	var byName map[string]*entryMeta
	for _, k := range tree.Consts {
		if !k.Exported {
			continue
		}
		lam, isLam := k.Value.(*syntax.Lambda)
		np, isName := k.Pattern.(*syntax.NamePat)
		if !isLam || k.Type != nil || !isName || np.Discard {
			continue
		}
		e := Entry{
			Name:   np.Name,
			Doc:    k.Doc,
			Params: make([]EntryParam, len(lam.Params)),
		}
		meta := &entryMeta{
			paramType: make([]*types.Type, len(lam.Params)),
			hasDef:    make([]bool, len(lam.Params)),
			max:       len(lam.Params),
		}
		allNamed := true
		for i, p := range lam.Params {
			pt := res.AnnTypes[p.Type]
			if pt == nil {
				pt = types.Never
			}
			var pname string
			if pnp, ok := p.Pattern.(*syntax.NamePat); ok && !pnp.Discard {
				pname = pnp.Name
			} else {
				allNamed = false
			}
			hasDef := p.Default != nil
			if !hasDef {
				meta.min = i + 1
			}
			meta.paramType[i] = pt
			meta.hasDef[i] = hasDef
			e.Params[i] = EntryParam{
				Name:     pname,
				Type:     Type{t: pt},
				Doc:      p.Doc,
				Optional: hasDef,
			}
		}
		if len(lam.Params) == 1 && isSingleRecord(meta.paramType[0]) {
			meta.inputType = meta.paramType[0]
			meta.singleRec = true
			t := Type{t: meta.inputType}
			e.InputType = &t
		} else if allNamed {
			fields := make([]types.Field, len(lam.Params))
			for i, ep := range e.Params {
				fields[i] = types.Field{
					Key:      ep.Name,
					Type:     meta.paramType[i],
					Optional: ep.Optional,
					Doc:      ep.Doc,
				}
			}
			if rec := types.Rec(fields, types.Never); types.Limit(rec) == nil {
				meta.inputType = rec
				t := Type{t: meta.inputType}
				e.InputType = &t
			}
		}
		if cr != nil {
			if rt, ok := cr.EntryTypes[np.Name]; ok && rt != nil {
				t := Type{t: rt}
				e.ResultType = &t
			}
		}
		meta.pub = e
		list = append(list, e)
		if byName == nil {
			byName = map[string]*entryMeta{}
		}
		byName[np.Name] = meta
	}
	return list, byName
}

// cloneEntries returns a copy of es so callers cannot mutate Program state.
func cloneEntries(es []Entry) []Entry {
	if len(es) == 0 {
		return nil
	}
	out := make([]Entry, len(es))
	for i, e := range es {
		e.Params = append([]EntryParam(nil), e.Params...)
		if e.InputType != nil {
			t := *e.InputType
			e.InputType = &t
		}
		if e.ResultType != nil {
			t := *e.ResultType
			e.ResultType = &t
		}
		out[i] = e
	}
	return out
}

// Entries returns the program's exported entry points, in text order
// (F16 12.6). ResultType is nil for a program compiled with NoCheck.
func (p *Program) Entries() []Entry { return cloneEntries(p.entries) }

// HasResult reports whether the program has a result expression (F16 12.2).
// When false, Run and Measure fail with the declare error no-result.
func (p *Program) HasResult() bool { return p.p.HasResult() }

// Call calls the named entry point with positional arguments (F16 12.5).
func (p *Program) Call(ctx context.Context, name string, args []any, vars Vars) (any, error) {
	v, _, err := p.CallMeasure(ctx, name, args, vars)
	return v, err
}

// CallMeasure calls the named entry point with positional arguments and
// returns its usage, whether the call succeeds or fails (F16 12.5).
func (p *Program) CallMeasure(ctx context.Context, name string, args []any, vars Vars) (any, Usage, error) {
	m := p.entryMap[name]
	if m == nil {
		return nil, Usage{}, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadName,
			Message: fmt.Sprintf("unknown entry point %s", name)}}}
	}
	if len(args) < m.min || len(args) > m.max {
		return nil, Usage{}, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeArity,
			Message: fmt.Sprintf("entry point %s takes %d to %d arguments, got %d", name, m.min, m.max, len(args))}}}
	}
	var ds []Diagnostic
	for i, a := range args {
		rem := maxCallConform - len(ds)
		if rem <= 0 {
			break
		}
		want := m.paramType[i]
		if m.hasDef[i] {
			want = types.WithNull(want)
		}
		fs := ConformAll(a, Type{t: want}, rem)
		stop := false
		for _, d := range fs {
			if d.Code == "not-conforming" {
				path := make([]any, len(d.Path)+1)
				path[0] = i
				copy(path[1:], d.Path)
				d.Path = path
			} else {
				stop = true
			}
			ds = append(ds, d)
			if stop || len(ds) >= maxCallConform {
				stop = true
				break
			}
		}
		if stop {
			break
		}
	}
	if len(ds) > 0 {
		return nil, Usage{}, &Error{Diagnostics: ds}
	}
	present := make([]bool, len(args))
	for i := range present {
		present[i] = true
	}
	if vars == nil {
		vars = Map(nil)
	}
	v, u, d := p.p.CallEntry(ctx, name, args, present, vars)
	if d != nil {
		return nil, u, p.fail(d)
	}
	return value.Unwrap(v), u, nil
}

// CallRecord calls the named entry point with one input record conformed to
// its input type (F16 12.5).
func (p *Program) CallRecord(ctx context.Context, name string, input any, vars Vars) (any, error) {
	v, _, err := p.CallRecordMeasure(ctx, name, input, vars)
	return v, err
}

// CallRecordMeasure calls the named entry point with one input record and
// returns its usage, whether the call succeeds or fails (F16 12.5).
func (p *Program) CallRecordMeasure(ctx context.Context, name string, input any, vars Vars) (any, Usage, error) {
	m := p.entryMap[name]
	if m == nil {
		return nil, Usage{}, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadName,
			Message: fmt.Sprintf("unknown entry point %s", name)}}}
	}
	if m.inputType == nil {
		return nil, Usage{}, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeNoInputRecord,
			Message: fmt.Sprintf("entry point %s has no input record", name)}}}
	}
	if ds := ConformAll(input, Type{t: m.inputType}, maxCallConform); len(ds) > 0 {
		return nil, Usage{}, &Error{Diagnostics: ds}
	}
	var args []any
	var present []bool
	if m.singleRec {
		args = []any{input}
		present = []bool{true}
	} else {
		args = make([]any, len(m.pub.Params))
		present = make([]bool, len(m.pub.Params))
		for i, ep := range m.pub.Params {
			if val, ok := value.Get(input, ep.Name); ok {
				args[i] = val
				present[i] = true
			}
		}
	}
	if vars == nil {
		vars = Map(nil)
	}
	v, u, d := p.p.CallEntry(ctx, name, args, present, vars)
	if d != nil {
		return nil, u, p.fail(d)
	}
	return value.Unwrap(v), u, nil
}
