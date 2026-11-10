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
// Every expect is ignored, and every case that gets as far is run. A program
// that checks has its type and sites after its name (stage S4).
func Outcome(c *casefile.Case) string {
	// F2 4.8: the report is the last field, for every case whose text
	// parses. An analysis case has name, phase and analysis.
	var x *risor.Expr
	var perr error
	if c.Conform == nil {
		x, perr = parseCase(c)
	}
	if c.Expect != nil && c.Expect.Analysis != nil {
		var b strings.Builder
		b.WriteString(`{"name":` + value.QuoteJSON(c.Name))
		if perr != nil {
			d := perr.(*risor.Error).Diagnostics[0]
			a := d.Span.Array()
			fmt.Fprintf(&b, `,"phase":"parse","diagnostics":[{"code":%s,"span":[%d,%d,%d,%d],"message":%s}]}`, value.QuoteJSON(d.Code), a[0], a[1], a[2], a[3], value.QuoteJSON(d.Message))
			return b.String()
		}
		b.WriteString(`,"phase":null,"analysis":` + AnalysisJSON(x.Analyze()) + "}")
		return b.String()
	}
	if c.Conform != nil {
		return conformLine(c)
	}
	if c.Expect != nil && c.Expect.Runner != nil { // F9 9.4
		return runnerLine(c)
	}
	if c.Render != nil { // F8 8: name, then the three forms
		r := c.Render
		detail := risor.RenderDetail(r.Diagnostics)
		if r.Source != nil {
			detail = risor.RenderDetail(r.Diagnostics, *r.Source)
		}
		return `{"name":` + value.QuoteJSON(c.Name) + `,"line":` + value.QuoteJSON(risor.RenderLine(r.Diagnostics)) +
			`,"detail":` + value.QuoteJSON(detail) + `,"json":` + value.QuoteJSON(risor.RenderJSON(r.Diagnostics)) + "}"
	}
	line := outcome(c)
	if perr == nil {
		line = line[:len(line)-1] + `,"analysis":` + AnalysisJSON(x.Analyze()) + "}"
	}
	return line
}

func outcome(c *casefile.Case) string {
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
				if !diag.LimitCodes[d.Code] { // F8 8
					b.WriteString(`,"trace":[`)
					for j, t := range d.Trace {
						if j > 0 {
							b.WriteByte(',')
						}
						a := t.Span.Array()
						fmt.Fprintf(&b, `{"span":[%d,%d,%d,%d],"name":%s,"index":`, a[0], a[1], a[2], a[3], value.QuoteJSON(t.Name))
						if t.Index < 0 {
							b.WriteString("null}")
						} else {
							fmt.Fprintf(&b, "%d}", t.Index)
						}
					}
					b.WriteByte(']')
				}
			} else if ph == "parse" || ph == "check" { // F8 8
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
	env, err := buildEnv(nil, c, st)
	if err != nil {
		return fail(err, nil)
	}
	if c.Schemas != nil || c.Types != nil {
		// F1 7.4: after name, before type.
		if d, err := env.Describe(); err == nil {
			b.WriteString(`,"description":`)
			b.Write(d)
		}
	}
	x, err := parseCase(c)
	if err != nil {
		return fail(err, nil)
	}
	var opts []risor.CheckOption
	unchecked := c.Check != nil && !*c.Check
	if unchecked {
		// Ruling AP3: compiled without a check; no type and no sites.
		opts = append(opts, risor.NoCheck)
	} else if c.Returns != nil {
		if *c.Returns == "condition" {
			opts = append(opts, risor.Condition)
		} else {
			opts = append(opts, risor.Expect(*c.Returns))
		}
	}
	if !unchecked {
		cr, err := env.Check(x, opts...)
		if err != nil {
			return fail(err, nil)
		}
		b.WriteString(`,"type":` + value.QuoteJSON(cr.Type) + `,"sites":[`)
		for i, st := range cr.Sites {
			if i > 0 {
				b.WriteByte(',')
			}
			a := st.Span.Array()
			fmt.Fprintf(&b, `{"class":%s,"span":[%d,%d,%d,%d]}`, value.QuoteJSON(st.Class), a[0], a[1], a[2], a[3])
		}
		b.WriteByte(']')
	}
	prog, err := env.Compile(x, opts...)
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

// conformLine is the outcome line of a conform case (F4 9.5).
func conformLine(c *casefile.Case) string {
	var b strings.Builder
	b.WriteString(`{"name":` + value.QuoteJSON(c.Name))
	o := doConform(nil, c)
	if o.declare != nil {
		ds := append([]risor.Diagnostic(nil), o.declare.Diagnostics...)
		sort.Slice(ds, func(i, j int) bool { return ds[i].Code < ds[j].Code })
		b.WriteString(`,"phase":"declare","diagnostics":[`)
		for i, d := range ds {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"code":` + value.QuoteJSON(d.Code) + `}`)
		}
		b.WriteString("]}")
		return b.String()
	}
	fmt.Fprintf(&b, `,"phase":null,"conforms":%t`, o.conforms)
	if !o.conforms {
		b.WriteString(`,"path":`)
		writeJSON(&b, o.path)
		b.WriteString(`,"message":` + value.QuoteJSON(o.message))
	}
	fmt.Fprintf(&b, `,"steps":%d,"agree":%t`, o.steps, o.agree)
	if c.Conform.Max != nil { // F8 8
		b.WriteString(`,"failures":[`)
		for i, f := range o.all {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"path":`)
			writeJSON(&b, f.Path)
			b.WriteString(`,"message":` + value.QuoteJSON(f.Message) + `}`)
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.String()
}

// runnerLine is the outcome line of a runner case (F9 9.4): name, then
// invalid or verdicts, then report, then text with update.
func runnerLine(c *casefile.Case) string {
	var b strings.Builder
	b.WriteString(`{"name":` + value.QuoteJSON(c.Name))
	lf, rs, err := innerRun(defaultCtx, c)
	if err != nil {
		b.WriteString(`,"invalid":true}`)
		return b.String()
	}
	b.WriteString(`,"verdicts":` + verdictsForm(rs))
	b.WriteString(`,"report":` + value.QuoteJSON(Report(rs)))
	if c.Update {
		b.WriteString(`,"text":` + value.QuoteJSON(RunnerText(lf, rs)))
	}
	b.WriteByte('}')
	return b.String()
}
