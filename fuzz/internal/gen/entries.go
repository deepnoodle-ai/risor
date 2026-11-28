package gen

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/types"
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

type soundD2Param struct {
	name    string
	t       *types.Type
	ty      *Ty
	opt     bool
	pattern bool
}

type soundD2Entry struct {
	name       string
	params     []soundD2Param
	resTy      *Ty
	resText    string
	hasInput   bool
	inputType  *types.Type
	paramTypes []*types.Type
	cst        *ast.Const
}

// applySoundD2 adds F16 slice D2 forms to a soundness-mode program and returns
// a callback that populates per-value-set Call args/input when Entry is used.
func (g *Gen) applySoundD2(base *Case, prog *ast.Program, careful bool) func(c *Case, edge bool) {
	r := g.r
	g.f16 = true
	g.d2 = true

	ep := g.makeSoundEntry(careful, 2)
	prog.Consts = append(prog.Consts, ep.cst)
	// Optionally add a second exported entry point (checked even when not called).
	if r.IntN(2) == 0 {
		ep2 := g.makeSoundEntry(careful, 1)
		prog.Consts = append(prog.Consts, ep2.cst)
	}
	prog.Types = g.types

	// 50%: keep Result calling ep (normal Run with exported entry points).
	if r.IntN(2) == 0 {
		prog.Result = g.callSoundEntryExpr(ep)
		if r.IntN(2) == 0 && ep.resText != "" {
			base.EntryReturns = map[string]string{ep.name: ep.resText}
		}
		return nil
	}

	// Only add an unused failing const when invoking the Call stage so
	// Result-running cases actually evaluate Program.Result.
	if r.IntN(2) == 0 {
		unused := &ast.Const{
			Pat:   &ast.NamePat{Name: g.fresh("skip")},
			Value: scall("fail", &ast.Str{Val: "unused", Quote: '"'}),
		}
		prog.Consts = append([]*ast.Const{unused}, prog.Consts...)
	}

	// 50%: Call stage on ep. Use the expanded concrete type text (never a local
	// type line name) for wantType so soundness Judge can parse it directly and
	// verify Promise 4 tightly.
	lam := ep.cst.Value.(*ast.Lambda)
	wantType := ep.resText
	if wantType == "" && ep.resTy != nil && ep.resTy.K == KMap {
		wantType = "map<any>"
	}
	if wantType == "" {
		wantType = "any"
	}
	if r.IntN(3) > 0 {
		prog.Result = nil
		prog.Semi = false
	}
	base.Entry = ep.name
	if lam.Result == "" || r.IntN(2) == 0 {
		base.EntryReturns = map[string]string{ep.name: wantType}
	}
	base.EntryWantType = wantType

	useInput := ep.hasInput && ep.inputType != nil && r.IntN(2) == 0
	if useInput {
		base.HasInput = true
		base.EntryInputType = ep.inputType.String()
		return func(c *Case, edge bool) {
			c.Input = g.valueFor(ep.inputType, 2, edge)
		}
	}
	base.HasArgs = true
	for _, pt := range ep.paramTypes {
		base.EntryParamTypes = append(base.EntryParamTypes, pt.String())
	}
	return func(c *Case, edge bool) {
		c.Args = g.makeSoundArgs(ep, edge)
	}
}

func (g *Gen) makeSoundEntry(careful bool, depth int) *soundD2Entry {
	r := g.r
	name := g.fresh("ep")
	ep := &soundD2Entry{name: name}
	lam := &ast.Lambda{}

	// Self-recursive exported entry point (~15% when E1 enabled).
	if !g.opts.NoE1 && r.IntN(7) == 0 {
		g.e1 = true
		pt := types.Prim("int")
		pname := g.fresh("n")
		ep.params = []soundD2Param{{name: pname, t: pt, ty: tyInt}}
		ep.hasInput = true
		ep.inputType = &types.Type{Rec: &types.Record{Fields: []types.Field{{Key: pname, T: pt}}}}
		ep.paramTypes = []*types.Type{pt}
		ep.resTy = tyInt
		ep.resText = "int"
		lam.Params = []ast.Param{{Pat: &ast.NamePat{Name: pname}, Type: "int"}}
		lam.Result = "int"
		lam.Body = scond(done(pname), snum(0), sbin("+", scall(name, less(pname)), snum(1)))
		ep.cst = &ast.Const{Exported: true, Pat: &ast.NamePat{Name: name}, Value: lam}
		g.bindLam(name, &lamSig{params: []*Ty{tyInt}, req: 1, result: tyInt})
		return ep
	}

	shape := r.IntN(5)
	g.push()
	switch shape {
	case 0:
		// 0-param entry point (often unannotated result, exercising site retention).
		ep.hasInput = true
		ep.inputType = &types.Type{Rec: &types.Record{}}
	case 1:
		// Single record parameter (Row 1).
		rec := g.soundEntryRecordType()
		rt := &types.Type{Rec: rec}
		typeText := rt.String()
		if r.IntN(2) == 0 {
			typeText = g.typeLine(typeText)
		}
		pname := g.fresh("req")
		ty := g.tyForType(rt, careful)
		lam.Params = append(lam.Params, ast.Param{
			Pat:  &ast.NamePat{Name: pname},
			Type: typeText,
		})
		g.bindName(pname, ty)
		ep.params = []soundD2Param{{name: pname, t: rt, ty: ty}}
		ep.hasInput = true
		ep.inputType = rt
		ep.paramTypes = []*types.Type{rt}
	case 2:
		// Destructured pattern parameter (Row 3, positional Call only).
		if r.IntN(2) == 0 {
			tA := types.Prim("int")
			tB := types.Prim("string")
			rec := &types.Record{Fields: []types.Field{
				{Key: "a", T: tA},
				{Key: "b", Opt: true, T: tB},
			}}
			rt := &types.Type{Rec: rec}
			aName := g.fresh("a")
			bName := g.fresh("b")
			pat := &ast.MapPat{Fields: []ast.Field{
				{Key: "a", Name: aName},
				{Key: "b", Name: bName, Default: &ast.Str{Val: "d", Quote: '"'}},
			}}
			lam.Params = append(lam.Params, ast.Param{Pat: pat, Type: rt.String()})
			g.bindName(aName, tyInt)
			g.bindName(bName, tyStr)
			ep.params = []soundD2Param{{t: rt, ty: g.tyForType(rt, careful), pattern: true}}
			ep.hasInput = false
			ep.paramTypes = []*types.Type{rt}
		} else {
			lt := types.ListOf(types.Prim("int"))
			xName := g.fresh("x")
			yName := g.fresh("y")
			pat := &ast.ListPat{Elems: []ast.Elem{
				{Name: xName, Default: snum(0)},
				{Name: yName, Default: snum(1)},
			}}
			lam.Params = append(lam.Params, ast.Param{Pat: pat, Type: "int[]"})
			g.bindName(xName, tyInt)
			g.bindName(yName, tyInt)
			ep.params = []soundD2Param{{t: lt, ty: listOf(tyInt, -1), pattern: true}}
			ep.hasInput = false
			ep.paramTypes = []*types.Type{lt}
		}
	default:
		// 1 to 3 named parameters, optionally with defaults (Row 2).
		np := 1 + r.IntN(3)
		rec := &types.Record{}
		for i := range np {
			pname := fmt.Sprintf("p%d", i+1)
			pt := g.soundEntryParamType()
			ty := g.tyForType(pt, careful)
			p := ast.Param{Pat: &ast.NamePat{Name: pname}, Type: pt.String()}
			opt := false
			if i == np-1 && r.IntN(2) == 0 || r.IntN(4) == 0 {
				if def := g.soundDefaultFor(pt); def != nil {
					p.Default = def
					opt = true
				}
			}
			lam.Params = append(lam.Params, p)
			g.bindName(pname, ty)
			ep.params = append(ep.params, soundD2Param{name: pname, t: pt, ty: ty, opt: opt})
			rec.Fields = append(rec.Fields, types.Field{Key: pname, Opt: opt, T: pt})
			callT := pt
			if opt {
				callT = pt.Copy()
				callT.Null = true
			}
			ep.paramTypes = append(ep.paramTypes, callT)
		}
		ep.hasInput = true
		ep.inputType = &types.Type{Rec: rec}
	}

	resTy := g.randTy(1)
	lam.Body = g.expr(resTy, depth)
	g.pop()

	ep.resTy = resTy
	rtText := annotText(resTy)
	if rtText != "" {
		ep.resText = rtText
		if r.IntN(2) == 0 || shape != 0 {
			lam.Result = rtText
		}
	}

	ep.cst = &ast.Const{Exported: true, Pat: &ast.NamePat{Name: name}, Value: lam}

	sig := &lamSig{result: resTy}
	for i, p := range ep.params {
		sig.params = append(sig.params, p.ty)
		if !p.opt {
			sig.req = i + 1
		}
	}
	g.bindLam(name, sig)
	return ep
}

func (g *Gen) soundEntryRecordType() *types.Record {
	r := g.r
	rec := &types.Record{}
	keys := []string{"a", "b", "flag", "n", "s"}
	n := 1 + r.IntN(3)
	for i := range n {
		rec.Fields = append(rec.Fields, types.Field{
			Key: keys[i],
			Opt: i > 0 && r.IntN(3) == 0,
			T:   g.soundEntryParamType(),
		})
	}
	if r.IntN(4) == 0 {
		rec.Rest = types.Any
	}
	return rec
}

func (g *Gen) soundEntryParamType() *types.Type {
	r := g.r
	switch r.IntN(8) {
	case 0, 1:
		return types.Prim("int")
	case 2:
		return types.Prim("float")
	case 3, 4:
		return types.Prim("string")
	case 5:
		return types.Prim("bool")
	case 6:
		return types.ListOf(types.Prim("int"))
	default:
		if g.opaque != "" && r.IntN(2) == 0 {
			return &types.Type{Opq: []string{g.opaque}}
		}
		return types.Any
	}
}

func (g *Gen) soundDefaultFor(t *types.Type) ast.Expr {
	switch {
	case t.Any:
		return snum(0)
	case t.Num == 1:
		return snum(g.r.IntN(10))
	case t.Num == 2:
		return &ast.Num{Text: "1.5"}
	case t.Str:
		return &ast.Str{Val: "def", Quote: '"'}
	case t.Bool:
		return &ast.Bool{V: false}
	case t.List != nil:
		return slist()
	case t.Null:
		return &ast.Null{}
	}
	return nil
}

func (g *Gen) callSoundEntryExpr(ep *soundD2Entry) ast.Expr {
	args := make([]ast.Expr, len(ep.params))
	for i, p := range ep.params {
		args[i] = g.expr(p.ty, 1)
	}
	return scall(ep.name, args...)
}

func (g *Gen) makeSoundArgs(ep *soundD2Entry, edge bool) []any {
	r := g.r
	n := len(ep.params)
	if edge && n > 0 {
		for n > 0 && ep.params[n-1].opt && r.IntN(2) == 0 {
			n--
		}
	}
	out := make([]any, n)
	for i := range n {
		if ep.params[i].opt && edge && r.IntN(3) == 0 {
			out[i] = nil
			continue
		}
		out[i] = g.valueFor(ep.params[i].t, 2, edge)
	}
	return out
}
