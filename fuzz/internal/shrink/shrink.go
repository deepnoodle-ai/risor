// Package shrink reduces a case while it keeps a difference.
//
// The reducer is greedy. Each round it makes a batch of smaller candidates:
// a const line, list item, entry, argument or arm removed; a subtree
// replaced by one of its children or by a literal; a literal shortened; a
// variable, stub, row or limit removed or made simpler; and, for a mangled
// case, a run of characters removed. It tests the whole batch at once and
// keeps the first candidate that still shows the same difference.
package shrink

import (
	"sort"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
)

// Test runs a batch of candidates and gives, for each, the signature of its
// difference, or "" when the implementations agree.
type Test func(cands []*gen.Case) []string

// Options bound the work of the reducer.
type Options struct {
	Rounds int // at most this many rounds; 0 means 80
	Batch  int // at most this many candidates a round; 0 means 400
}

// Shrink reduces c while test gives the signature sig.
func Shrink(c *gen.Case, sig string, test Test, opt Options) *gen.Case {
	if opt.Rounds == 0 {
		opt.Rounds = 80
	}
	if opt.Batch == 0 {
		opt.Batch = 400
	}
	cur := c
	size := len(cur.JSON())
	for range opt.Rounds {
		var cands []*gen.Case
		for _, k := range Candidates(cur) {
			if len(k.JSON()) < size {
				cands = append(cands, k)
				if len(cands) == opt.Batch {
					break
				}
			}
		}
		if len(cands) == 0 {
			break
		}
		sigs := test(cands)
		found := false
		for i, s := range sigs {
			if s == sig {
				cur = cands[i]
				size = len(cur.JSON())
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return cur
}

// Candidates gives the smaller cases one change away from c, larger changes
// first.
func Candidates(c *gen.Case) []*gen.Case {
	var out []*gen.Case
	add := func(f func(k *gen.Case) bool) {
		k := c.Clone()
		if f(k) {
			out = append(out, k)
		}
	}
	// The environment.
	if c.Limits != nil {
		add(func(k *gen.Case) bool { k.Limits = nil; return true })
	}
	if c.Returns != "" {
		add(func(k *gen.Case) bool { k.Returns = ""; return true })
	}
	for i := range c.Types {
		add(func(k *gen.Case) bool { k.Types = append(k.Types[:i:i], k.Types[i+1:]...); return true })
	}
	for i := range c.Removed {
		add(func(k *gen.Case) bool { k.Removed = append(k.Removed[:i:i], k.Removed[i+1:]...); return true })
	}
	for i := range c.Functions {
		add(func(k *gen.Case) bool { k.Functions = append(k.Functions[:i:i], k.Functions[i+1:]...); return true })
	}
	if len(c.Opaque) > 0 {
		add(func(k *gen.Case) bool { k.Opaque, k.Methods = nil, nil; return true })
	}
	for i := range c.Methods {
		add(func(k *gen.Case) bool { k.Methods = append(k.Methods[:i:i], k.Methods[i+1:]...); return true })
	}
	for i := range c.Vars {
		add(func(k *gen.Case) bool { k.Vars = append(k.Vars[:i:i], k.Vars[i+1:]...); return true })
	}
	// The program.
	if c.Prog != nil {
		out = append(out, treeCandidates(c)...)
	} else {
		out = append(out, textCandidates(c)...)
	}
	// Smaller host values and stubs.
	for i := range c.Vars {
		for _, v := range simpler(c.Vars[i].Value) {
			add(func(k *gen.Case) bool { k.Vars[i].Value = v; return true })
		}
	}
	for i := range c.Functions {
		for j := range c.Functions[i].Rows {
			if len(c.Functions[i].Rows) > 1 {
				add(func(k *gen.Case) bool {
					s := k.Functions[i]
					s.Rows = append(append([]gen.Row(nil), s.Rows[:j]...), s.Rows[j+1:]...)
					k.Functions[i] = s
					return true
				})
			}
		}
	}
	return out
}

func treeCandidates(c *gen.Case) []*gen.Case {
	var out []*gen.Case
	slots, lists := ast.Sites(c.Prog)
	for li, l := range lists {
		for j := range l.Len {
			k := c.Clone()
			_, kl := ast.Sites(k.Prog)
			kl[li].Remove(j)
			out = append(out, k)
		}
	}
	for pi := range ast.PatternSites(c.Prog) {
		k := c.Clone()
		*ast.PatternSites(k.Prog)[pi] = &ast.NamePat{Name: "z"}
		out = append(out, k)
	}
	for si, s := range slots {
		var repl []ast.Expr
		repl = append(repl, ast.Children(*s)...)
		switch e := (*s).(type) {
		case *ast.Num:
			if e.Text != "0" && e.Text != "1" {
				repl = append(repl, &ast.Num{Text: "0"}, &ast.Num{Text: "1"})
			}
		case *ast.Str:
			if e.Val != "" {
				rs := []rune(e.Val)
				repl = append(repl, &ast.Str{Val: "", Quote: '"'}, &ast.Str{Val: string(rs[:len(rs)/2]), Quote: e.Quote, Esc: e.Esc})
			}
			if e.Esc != 0 || e.Quote != '"' {
				repl = append(repl, &ast.Str{Val: e.Val, Quote: '"'})
			}
		case *ast.Ident:
			repl = append(repl, &ast.Num{Text: "0"})
		case *ast.Null, *ast.Bool, *ast.Lambda:
		default:
			repl = append(repl, &ast.Num{Text: "0"}, &ast.Null{})
		}
		for _, r := range repl {
			k := c.Clone()
			ks, _ := ast.Sites(k.Prog)
			*ks[si] = ast.CloneExpr(r)
			out = append(out, k)
		}
	}
	return out
}

func textCandidates(c *gen.Case) []*gen.Case {
	var out []*gen.Case
	rs := []rune(c.Source)
	for n := len(rs) / 2; n >= 1; n /= 2 {
		for i := 0; i+n <= len(rs); i += n {
			k := c.Clone()
			k.Source = string(rs[:i]) + string(rs[i+n:])
			out = append(out, k)
		}
		if len(out) > 300 {
			break
		}
	}
	return out
}

// simpler gives smaller host values in place of v.
func simpler(v any) []any {
	var out []any
	switch v := v.(type) {
	case []any:
		out = append(out, []any{})
		for i := range v {
			w := append(append([]any(nil), v[:i]...), v[i+1:]...)
			out = append(out, w)
		}
		for i := range v {
			for _, s := range simpler(v[i]) {
				w := append([]any(nil), v...)
				w[i] = s
				out = append(out, w)
			}
		}
		if len(v) == 1 {
			out = append(out, v[0])
		}
	case map[string]any:
		if _, ok := v["$opaque"]; ok {
			return nil
		}
		out = append(out, map[string]any{})
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			w := map[string]any{}
			for k2, x := range v {
				if k2 != k {
					w[k2] = x
				}
			}
			out = append(out, w)
		}
	case string:
		if v != "" {
			rs := []rune(v)
			out = append(out, "", string(rs[:len(rs)/2]))
		}
	case float64:
		if v != 0 && v != 1 {
			out = append(out, 0.0, 1.0)
		}
	case bool:
		out = append(out, nil)
	}
	return out
}
