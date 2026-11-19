package gen

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// This file writes the forms of package F16, slice D1
// (spec/packages/f16-entry-points.md): type lines, and annotations on const
// lines, parameters and results. In a typed case an annotation is the
// generator's own type of the value, so it fits; in a loose case it is
// sometimes another type, so that the checker's mismatches are tried too.

// annotText is the program type text of a rough type, or "" when the rough
// type says too little to write one. A record is written open, as the
// generator knows some of a map's keys and not always all.
func annotText(t *Ty) string {
	if t == nil {
		return ""
	}
	switch t.K {
	case KAny:
		return "any"
	case KNum:
		return "float"
	case KInt:
		return "int"
	case KStr:
		return "string"
	case KBool:
		return "bool"
	case KNull:
		return "null"
	case KOpq:
		return t.Opq
	case KList:
		if t.Elem == nil {
			return "any[]"
		}
		e := annotText(t.Elem)
		if e == "" {
			return ""
		}
		return e + "[]"
	case KMap:
		if t.Fields == nil {
			return ""
		}
		var parts []string
		for _, f := range t.Fields {
			ft := annotText(f.T)
			if ft == "" {
				return ""
			}
			parts = append(parts, typeKey(f.Name)+": "+ft)
		}
		return "{" + strings.Join(append(parts, "..."), ", ") + "}"
	}
	return ""
}

// wrongTexts are annotations for a loose case: each fits some values and not
// others.
var wrongTexts = []string{"int", "string", "bool", "null", "int[]", "{a: int}", "string | null", "any", "Nope"}

// annot gives an annotation for a value of rough type t, or "". It sets
// g.f16 when it gives one. Now and then a record's text becomes a type line,
// and the annotation its name.
func (g *Gen) annot(t *Ty) string {
	if g.opts.NoF16 {
		return ""
	}
	r := g.r
	text := annotText(t)
	if g.loose && r.IntN(4) == 0 {
		text = pick(r, wrongTexts)
	}
	if text == "" {
		return ""
	}
	g.f16 = true
	if strings.HasPrefix(text, "{") && r.IntN(3) == 0 {
		return g.typeLine(text)
	}
	return text
}

// typeLine gives the name of a type line whose type is text, declaring one
// when there is none yet.
func (g *Gen) typeLine(text string) string {
	for _, tl := range g.types {
		if tl.Type == text {
			return tl.Name
		}
	}
	name := fmt.Sprintf("Tl%d", len(g.types))
	g.types = append(g.types, ast.TypeLine{Name: name, Type: text})
	return name
}

// annotChance reports whether a form of package F16 is written here.
func (g *Gen) annotChance(n int) bool { return !g.opts.NoF16 && g.r.IntN(n) == 0 }
