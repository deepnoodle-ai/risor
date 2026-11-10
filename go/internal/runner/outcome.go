package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// Outcome runs one case and writes the line of the outcome command (ruling
// AJ): one JSON object with name, phase, value or diagnostics, and usage.
// Every expect is ignored, and every case that gets as far is run.
func Outcome(c *casefile.Case) string {
	var b strings.Builder
	b.WriteString(`{"name":`)
	b.WriteString(value.QuoteJSON(c.Name))
	st := &stubState{}
	fail := func(err error, u *risor.Usage) string {
		e := err.(*risor.Error)
		ph := string(e.Phase())
		b.WriteString(`,"phase":` + value.QuoteJSON(ph) + `,"diagnostics":[`)
		ds := e.Diagnostics
		if ph == "parse" {
			ds = ds[:1]
		}
		if ph == "declare" {
			ds = append([]risor.Diagnostic(nil), ds...)
			sort.Slice(ds, func(i, j int) bool { return ds[i].Code < ds[j].Code })
		}
		for i, d := range ds {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"code":` + value.QuoteJSON(d.Code))
			if ph != "declare" {
				b.WriteString(`,"span":`)
				if d.Span != nil && !diag.LimitCodes[d.Code] {
					a := d.Span.Array()
					fmt.Fprintf(&b, "[%d,%d,%d,%d]", a[0], a[1], a[2], a[3])
				} else {
					b.WriteString("null")
				}
			}
			if ph == "run" {
				b.WriteString(`,"class":`)
				if d.Class != "" {
					b.WriteString(value.QuoteJSON(d.Class))
				} else {
					b.WriteString("null")
				}
				b.WriteString(`,"message":` + value.QuoteJSON(d.Message))
			}
			b.WriteByte('}')
		}
		b.WriteByte(']')
		// AK3: a limit failure has no fixed usage, so none is written.
		if u != nil && !(ph == "run" && diag.LimitCodes[ds[0].Code]) {
			fmt.Fprintf(&b, `,"steps":%d,"units":%d`, u.Steps, u.Alloc)
		}
		b.WriteByte('}')
		return b.String()
	}
	env, err := buildEnv(c, st)
	if err != nil {
		return fail(err, nil)
	}
	x, err := risor.Parse(c.Source)
	if err != nil {
		return fail(err, nil)
	}
	prog, err := env.Compile(x)
	if err != nil {
		return fail(err, nil)
	}
	v, u, err := prog.Measure(context.Background(), risor.Map(c.Vars))
	if err != nil {
		return fail(err, &u)
	}
	b.WriteString(`,"phase":null,"value":`)
	writeJSON(&b, v)
	fmt.Fprintf(&b, `,"steps":%d,"units":%d}`, u.Steps, u.Alloc)
	return b.String()
}

func writeJSON(b *strings.Builder, v any) {
	v, _ = value.Read(v)
	switch value.KindOf(v) {
	case value.List:
		b.WriteByte('[')
		for i := 0; i < value.Len(v); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, value.At(v, i))
		}
		b.WriteByte(']')
	case value.Map:
		b.WriteByte('{')
		for i, k := range value.Keys(v) {
			if i > 0 {
				b.WriteByte(',')
			}
			x, _ := value.Get(v, k)
			b.WriteString(value.QuoteJSON(k) + ":")
			writeJSON(b, x)
		}
		b.WriteByte('}')
	case value.OpaqueKind:
		o, _ := v.(casefile.OpaqueValue)
		fmt.Fprintf(b, `{"$opaque":{"type":%s,"id":%d}}`, value.QuoteJSON(o.Type), o.ID)
	default:
		b.WriteString(value.Debug(v))
	}
}
