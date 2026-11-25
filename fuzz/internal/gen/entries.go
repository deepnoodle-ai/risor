package gen

import (
	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// This file writes the forms of package F16, slice D2
// (spec/packages/f16-entry-points.md 12): export const entry points,
// programs with no result expression, positional and record calls, and
// entryReturns. In loose mode it also generates bad-export forms, bad entry
// names, wrong arities, non-conforming arguments, and no-input-record calls.

// applyD2 turns a generated program into an entry-point case of slice D2.
func (g *Gen) applyD2(c *Case) {
	r := g.r
	g.f16 = true
	g.d2 = true

	p := c.Prog
	name := g.fresh("ep")

	// Build an entry point with 0 to 2 typed parameters (or a single record).
	type epParam struct {
		name   string
		ty     *Ty
		tyText string
		val    any
		hasDef bool
	}

	var params []epParam
	singleRecord := false
	switch r.IntN(4) {
	case 0:
		// 0 parameters.
	case 1:
		// Single record parameter (Row 1).
		singleRecord = true
		pn := g.fresh("q")
		params = []epParam{{
			name:   pn,
			ty:     recordOf([]TField{{Name: "a", T: tyInt}, {Name: "s", T: tyStr}}),
			tyText: "{a: int, s: string}",
			val:    map[string]any{"a": float64(r.IntN(20) - 5), "s": pick(r, []string{"ok", "hi", ""})},
		}}
	case 2:
		// One simple parameter (Row 2).
		pn := g.fresh("x")
		if r.IntN(2) == 0 {
			params = []epParam{{name: pn, ty: tyInt, tyText: "int", val: float64(r.IntN(20) - 5)}}
		} else {
			params = []epParam{{name: pn, ty: tyStr, tyText: "string", val: pick(r, []string{"a", "hello", ""})}}
		}
	default:
		// Two parameters, second optionally defaulted (Row 2).
		p1 := g.fresh("x")
		p2 := g.fresh("y")
		hasDef := r.IntN(2) == 0
		params = []epParam{
			{name: p1, ty: tyInt, tyText: "int", val: float64(r.IntN(15) - 3)},
			{name: p2, ty: tyInt, tyText: "int", val: float64(r.IntN(10)), hasDef: hasDef},
		}
	}

	g.push()
	lam := &ast.Lambda{}
	for _, ep := range params {
		ap := ast.Param{Pat: &ast.NamePat{Name: ep.name}, Type: ep.tyText}
		if g.loose && r.IntN(8) == 0 {
			ap.Type = "" // bad-export: unannotated
		}
		if ep.hasDef {
			ap.Default = &ast.Num{Text: "1"}
		}
		lam.Params = append(lam.Params, ap)
		g.bindName(ep.name, ep.ty)
	}
	resTy := tyInt
	if r.IntN(2) == 0 {
		resTy = tyStr
	}
	lam.Body = g.expr(resTy, 2)
	if r.IntN(3) == 0 {
		lam.Result = annotText(resTy)
	}
	g.pop()

	expConst := &ast.Const{
		Exported: true,
		Pat:      &ast.NamePat{Name: name},
		Value:    lam,
	}
	if g.loose && r.IntN(10) == 0 {
		// bad-export: not-lambda
		expConst.Value = &ast.Num{Text: "1"}
	}
	if g.loose && r.IntN(10) == 0 {
		// bad-export: binds-nothing beside an export
		p.Consts = append(p.Consts, &ast.Const{Pat: &ast.NamePat{Name: "_"}, Value: &ast.Num{Text: "0"}})
	}
	p.Consts = append(p.Consts, expConst)

	// Half of D2 cases omit the result expression.
	if r.IntN(2) == 0 && c.Returns == "" {
		p.Result = nil
		p.Semi = false
	}

	// Optional entryReturns.
	if r.IntN(4) == 0 {
		rt := annotText(resTy)
		if g.loose && r.IntN(3) == 0 {
			rt = "bool"
		}
		target := name
		if g.loose && r.IntN(6) == 0 {
			target = "missing_ep"
		}
		c.EntryReturns = map[string]string{target: rt}
	}

	// Choose how the case runs: positional Call, CallRecord, or Run.
	if p.Result != nil && r.IntN(4) == 0 {
		return // run program result expression with export const present
	}
	c.Entry = name
	if g.loose && r.IntN(10) == 0 {
		c.Entry = "no_such_entry"
		c.HasArgs = true
		c.Args = []any{}
		return
	}

	if r.IntN(2) == 0 {
		// Positional call.
		c.HasArgs = true
		args := make([]any, 0, len(params))
		for i, ep := range params {
			if ep.hasDef && i == len(params)-1 && r.IntN(3) == 0 {
				continue // omit defaulted arg
			}
			v := ep.val
			if g.loose && r.IntN(5) == 0 {
				v = true // wrong type -> not-conforming
			}
			args = append(args, v)
		}
		if g.loose && r.IntN(6) == 0 {
			args = append(args, float64(1), float64(2)) // arity mismatch
		}
		c.Args = args
	} else {
		// Call by record.
		c.HasInput = true
		if singleRecord {
			c.Input = params[0].val
		} else {
			m := map[string]any{}
			for _, ep := range params {
				if ep.hasDef && r.IntN(3) == 0 {
					continue
				}
				m[ep.name] = ep.val
			}
			c.Input = m
		}
		if g.loose && r.IntN(5) == 0 {
			c.Input = "not-a-record"
		}
	}
}
