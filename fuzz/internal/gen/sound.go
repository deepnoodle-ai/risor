package gen

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/fuzz/internal/types"
)

// The soundness mode: an environment of declared types, several value sets
// that conform to them, and a program written against the types. The
// program is careful (it uses a value only as its declared type allows) or
// careless (it uses an any value, a nullable value or an optional field as
// if it were known). The checker decides which programs check clean; the
// promise of 05-types.md 18.4 is tested on those.

// SoundProgram is one program and its value sets: one case for each.
type SoundProgram struct {
	Name    string
	Careful bool
	Cases   []*Case
}

// ValueSets is the number of value sets made for each program.
const ValueSets = 5

// GenerateSound makes the program with the given index of the given round.
func GenerateSound(lib *Library, opts Options, seed uint64, round, index int) *SoundProgram {
	r := Seed(seed^0x50a4d, round, index)
	g := &Gen{r: r, lib: lib, opts: opts, sound: true}
	for _, e := range lib.Functions {
		if !opts.Exclude[e.Name] {
			g.funcs = append(g.funcs, e)
		}
	}
	methods := lib.Methods
	if !opts.NoF7 {
		methods = lib.WithQueries()
	}
	for _, e := range methods {
		if !opts.Exclude[e.Name] {
			g.meths = append(g.meths, e)
		}
	}
	sp := &SoundProgram{Name: fmt.Sprintf("s%d-%d", round, index), Careful: r.IntN(2) == 0}
	base := &Case{Mode: "sound"}
	if r.IntN(5) == 0 {
		g.opaque = pick(r, []string{"Tok", "Clock"})
		base.Opaque = []string{g.opaque}
		g.soundOpaque(base)
	}
	if r.IntN(3) == 0 {
		g.soundHosts(base)
	}
	g.frames = [][]bind{nil}
	g.declared, g.valueTy = map[string]*types.Type{}, map[string]*types.Type{}
	if !opts.NoF4 && r.IntN(4) == 0 {
		g.namedTypes(base)
	}
	if !opts.NoF6 && r.IntN(4) == 0 {
		g.turnOnGroups(base)
	}
	n := 1 + r.IntN(4)
	var decls []*types.Type
	var names []string
	used := map[string]bool{}
	for range n {
		name := pick(r, varNames)
		if used[name] {
			continue
		}
		used[name] = true
		t := g.declaredType(2)
		decls = append(decls, t)
		names = append(names, name)
		g.declared[name] = t
		if len(t.Union) > 0 {
			g.unions = append(g.unions, unionVar{name: name, t: t})
		}
		g.bindName(name, g.tyForType(t, sp.Careful))
	}
	prog := g.program()
	for k := range ValueSets {
		c := *base
		c.Name = fmt.Sprintf("%s-v%d", sp.Name, k)
		c.Prog = prog
		for i, t := range decls {
			v := g.valueFor(t, 3, k == 0)
			if !types.Conforms(v, t) {
				panic(fmt.Sprintf("gen: a value %v does not conform to %s", v, t))
			}
			c.Vars = append(c.Vars, Var{Name: names[i], Type: t.String(), Value: v})
		}
		sp.Cases = append(sp.Cases, &c)
	}
	return sp
}

// declaredType makes a random type of at most the given depth.
func (g *Gen) declaredType(depth int) *types.Type {
	r := g.r
	if r.IntN(4) == 0 {
		return types.Any // any, at the top and at depth: the source of dynamic sites
	}
	n := r.IntN(14)
	if depth <= 0 && n >= 9 {
		n = r.IntN(9)
	}
	var t *types.Type
	switch n {
	case 0:
		t = types.Prim("int")
	case 1:
		t = types.Prim("float")
	case 2, 3:
		t = types.Prim("string")
	case 4:
		t = types.Prim("bool")
	case 5:
		t = &types.Type{Lits: []string{"fail", "pass"}}
		if r.IntN(2) == 0 {
			t.Lits = []string{"", "a", "b"}
		}
	case 6:
		t = types.Any
	case 7:
		if g.opaque != "" {
			t = &types.Type{Opq: []string{g.opaque}}
		} else {
			t = types.Prim("int")
		}
	case 8:
		if !g.opts.NoF4 && r.IntN(2) == 0 {
			t = g.unionType() // a tagged union (f05-unions.md 2)
		} else {
			t = &types.Type{Num: 1, Str: true} // two kinds
		}
	case 9, 10:
		t = types.ListOf(g.declaredType(depth - 1))
	case 11:
		t = &types.Type{Rec: &types.Record{Rest: g.declaredType(depth - 1)}}
	default:
		rec := &types.Record{}
		seen := map[string]bool{}
		for range 1 + r.IntN(3) {
			k := pick(r, fieldNames[:10])
			if seen[k] {
				continue
			}
			seen[k] = true
			rec.Fields = append(rec.Fields, types.Field{Key: k, Opt: r.IntN(3) == 0, T: g.declaredType(depth - 1)})
		}
		if r.IntN(3) == 0 {
			rec.Rest = types.Any
		}
		t = &types.Type{Rec: rec}
	}
	if !t.Any && r.IntN(4) == 0 {
		t = t.Copy()
		t.Null = true
	}
	return t
}

// valueFor makes a value that conforms to t, favouring the edges: null,
// absent optional keys, empty containers, empty strings, zero, false, the
// ends of the whole range, extra keys on an open record.
func (g *Gen) valueFor(t *types.Type, depth int, edge bool) any {
	r := g.r
	if t.Any {
		return g.hostValue(depth - 1)
	}
	var alts []string
	if t.Null {
		alts = append(alts, "null")
	}
	if t.Bool {
		alts = append(alts, "bool")
	}
	if t.Num != 0 {
		alts = append(alts, "num")
	}
	if t.Str || len(t.Lits) > 0 {
		alts = append(alts, "str")
	}
	if t.List != nil {
		alts = append(alts, "list")
	}
	if t.Rec != nil || len(t.Union) > 0 {
		alts = append(alts, "rec")
	}
	if len(t.Opq) > 0 {
		alts = append(alts, "opq")
	}
	if len(alts) == 0 {
		return nil // never: no value; a caller does not ask for one
	}
	edge = edge || r.IntN(2) == 0
	a := pick(r, alts)
	if t.Null && edge && r.IntN(3) == 0 {
		a = "null"
	}
	switch a {
	case "null":
		return nil
	case "bool":
		return edge && r.IntN(2) == 0 || r.IntN(2) == 0
	case "num":
		if t.Num == 1 {
			if edge {
				return pick(r, []float64{0, maxWhole, -maxWhole, 1, -1})
			}
			return float64(r.IntN(200) - 100)
		}
		if edge {
			return pick(r, []float64{0, 0.5, 1e308, 5e-324, -1e308, maxWhole + 1})
		}
		return pick(r, numValues)
	case "str":
		if len(t.Lits) > 0 && !t.Str {
			return pick(r, t.Lits)
		}
		if edge {
			return pick(r, []string{"", "😀", "é", "a"})
		}
		return pick(r, strValues)
	case "list":
		if t.List.IsNever() || edge && r.IntN(2) == 0 || depth <= 0 {
			return []any{}
		}
		out := make([]any, 1+r.IntN(3))
		for i := range out {
			out[i] = g.valueFor(t.List, depth-1, edge)
		}
		return out
	case "rec":
		rec := t.Rec
		if len(t.Union) > 0 {
			rec = pick(r, t.Union) // a union: one variant (f05-unions.md 7.1)
		}
		out := map[string]any{}
		for _, f := range rec.Fields {
			if f.Opt && (edge || r.IntN(2) == 0) {
				continue
			}
			if f.T.IsNever() {
				if f.Opt {
					continue
				}
				return nil // cannot happen: a required never field is not made
			}
			out[f.Key] = g.valueFor(f.T, depth-1, edge)
		}
		if rec.Rest != nil && !rec.Rest.IsNever() && (r.IntN(2) == 0 || len(rec.Fields) == 0 && !edge) {
			for range 1 + r.IntN(2) {
				k := pick(r, []string{"zz", "extra", "n", "a-b", ""})
				if rec.Fields != nil && hasField(rec, k) {
					continue
				}
				out[k] = g.valueFor(rec.Rest, depth-1, edge)
			}
		}
		return out
	default:
		return map[string]any{"$opaque": map[string]any{"type": pick(r, t.Opq), "id": float64(1 + r.IntN(3))}}
	}
}

func hasField(r *types.Record, k string) bool {
	for _, f := range r.Fields {
		if f.Key == k {
			return true
		}
	}
	return false
}

// tyForType gives the rough type a program uses a variable as. A careful
// program uses only what the type promises; a careless one takes an any
// value, a nullable value, an optional field or one of several kinds as if
// it were known.
func (g *Gen) tyForType(t *types.Type, careful bool) *Ty {
	if t.Any {
		if careful {
			return tyAny
		}
		return g.randTy(1)
	}
	var kinds []*Ty
	if t.Bool {
		kinds = append(kinds, tyBool)
	}
	switch t.Num {
	case 1:
		kinds = append(kinds, tyInt)
	case 2:
		kinds = append(kinds, tyNum)
	}
	if t.Str || len(t.Lits) > 0 {
		kinds = append(kinds, tyStr)
	}
	if t.List != nil {
		el := g.tyForType(t.List, careful)
		if t.List.IsNever() {
			el = tyAny
		}
		kinds = append(kinds, listOf(el, -1))
	}
	if len(t.Union) > 0 {
		// A careless program reads one variant's fields with no test of the
		// tag; the checker reports maybe-absent there.
		v := pick(g.r, t.Union)
		var fs []TField
		for _, f := range v.Fields {
			fs = append(fs, TField{Name: f.Key, T: g.tyForType(f.T, careful)})
		}
		kinds = append(kinds, recordOf(fs))
		if careful {
			kinds = append(kinds, tyAny) // with a union, careful reads go by a tag test
		}
	}
	if t.Rec != nil {
		if len(t.Rec.Fields) == 0 {
			kinds = append(kinds, mapAny())
		} else {
			var fs []TField
			for _, f := range t.Rec.Fields {
				if f.Opt && careful {
					continue
				}
				fs = append(fs, TField{Name: f.Key, T: g.tyForType(f.T, careful)})
			}
			if fs == nil && careful {
				kinds = append(kinds, mapAny())
			} else {
				kinds = append(kinds, recordOf(fs))
			}
		}
	}
	for _, o := range t.Opq {
		kinds = append(kinds, &Ty{K: KOpq, Opq: o, Len: -1})
	}
	switch {
	case len(kinds) == 0:
		return tyNull
	case careful && (len(kinds) > 1 || t.Null):
		return tyAny
	default:
		return pick(g.r, kinds)
	}
}

func (g *Gen) soundOpaque(c *Case) {
	o := g.opaque
	opq := func(id int) any {
		return map[string]any{"$opaque": map[string]any{"type": o, "id": float64(id)}}
	}
	ot := &Ty{K: KOpq, Opq: o, Len: -1}
	c.Methods = []Stub{
		{Name: o + ".label", Type: "() => string", Rows: []Row{
			{HasArgs: true, Args: []any{opq(1)}, Result: "one", Charge: 1},
			{Result: "other"},
		}},
		{Name: o + ".next", Type: "(n: int) => " + o, Rows: []Row{{Result: opq(3)}}},
		{Name: o + ".size", Type: "() => int", Rows: []Row{{Result: 3.0}}},
		{Name: o + ".broken", Type: "() => string !", Rows: []Row{
			{HasArgs: true, Args: []any{opq(2)}, IsError: true, Error: "the clock is broken"},
			{Result: "fine"},
		}},
	}
	g.methods = []opqMethod{
		{name: "label", result: tyStr},
		{name: "next", params: []*Ty{tyInt}, result: ot},
		{name: "size", result: tyInt},
		{name: "broken", result: tyStr},
	}
}

func (g *Gen) soundHosts(c *Case) {
	g.ns = "geo"
	c.Functions = []Stub{
		{Name: "lookup", Type: "(key: string) => int | null", Rows: []Row{
			{HasArgs: true, Args: []any{"a"}, Result: 1.0},
			{HasArgs: true, Args: []any{"b"}, Result: nil},
			{Result: 0.0},
		}},
		{Name: "fetch", Type: "(key: string) => string !", Rows: []Row{
			{HasArgs: true, Args: []any{"a"}, IsError: true, Error: "no such key"},
			{HasArgs: true, Args: []any{""}, IsError: true, Error: "empty key"},
			{Result: "v", Charge: 1},
		}},
		{Name: "score", Type: "(x: any, y?: int) => int", Rows: []Row{{Result: 7.0}}},
		{Name: "geo.dist", Type: "(...xs: float[]) => float", Rows: []Row{{Result: 2.5}}},
	}
	g.hosts = []hostFn{
		{name: "lookup", params: []*Ty{tyStr}, req: 1, result: tyAny},
		{name: "fetch", params: []*Ty{tyStr}, req: 1, result: tyStr},
		{name: "score", params: []*Ty{tyAny, tyInt}, req: 1, result: tyInt},
		{ns: "geo", name: "dist", params: []*Ty{tyNum}, rest: true, result: tyNum},
	}
}
