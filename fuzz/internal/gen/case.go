package gen

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// Mode is how a case was made.
type Mode string

// The three modes.
const (
	Typed   Mode = "typed"
	Loose   Mode = "loose"
	Mangled Mode = "mangled"
)

// Var is one variable of a case: declared in env with a type, and supplied in
// vars unless Missing.
type Var struct {
	Name    string
	Type    string
	Value   any
	Missing bool
}

// Row is one row of a stub.
type Row struct {
	Args     []any // nil: matches every call
	HasArgs  bool
	Result   any
	Error    string // used when IsError
	IsError  bool
	Charge   int
	Allocate int
}

// Stub is a stubbed host function or opaque method.
type Stub struct {
	Name string // "f", "ns.f", or "Type.method"
	Type string
	Rows []Row
}

// Limits are the limits of a case; a zero pointer field is not given.
type Limits struct {
	Steps, Allocation, Depth *int
}

// Case is one generated case. Prog is nil for a mangled case, whose Source
// is the text itself.
type Case struct {
	Name      string
	Mode      Mode
	Prog      *ast.Program
	Source    string
	Vars      []Var
	Opaque    []string
	Methods   []Stub
	Functions []Stub
	Limits    *Limits
	// NoCheck writes "check": false: the case is compiled with the static
	// rules alone (03-host.md 3.10.3).
	NoCheck bool
	// Broken writes "precondition": false: a variable's value does not
	// conform to its declared type, on purpose.
	Broken bool
}

// Text gives the program text: the printed tree, or the mangled source.
func (c *Case) Text() string {
	if c.Prog != nil {
		return ast.Print(c.Prog)
	}
	return c.Source
}

// Clone copies a case deeply enough for the reducer to change it.
func (c *Case) Clone() *Case {
	d := *c
	if c.Prog != nil {
		d.Prog = ast.Clone(c.Prog)
	}
	d.Vars = append([]Var(nil), c.Vars...)
	for i := range d.Vars {
		d.Vars[i].Value = cloneValue(d.Vars[i].Value)
	}
	d.Opaque = append([]string(nil), c.Opaque...)
	d.Methods = append([]Stub(nil), c.Methods...)
	d.Functions = append([]Stub(nil), c.Functions...)
	if c.Limits != nil {
		l := *c.Limits
		d.Limits = &l
	}
	return &d
}

func cloneValue(v any) any {
	switch v := v.(type) {
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = cloneValue(x)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, x := range v {
			out[k] = cloneValue(x)
		}
		return out
	}
	return v
}

// obj writes a JSON object with its keys in the order given.
type obj struct {
	b bytes.Buffer
	n int
}

func (o *obj) field(k string, raw []byte) {
	if o.n == 0 {
		o.b.WriteByte('{')
	} else {
		o.b.WriteString(", ")
	}
	o.n++
	o.b.WriteString(jsonString(k))
	o.b.WriteString(": ")
	o.b.Write(raw)
}

func (o *obj) done() []byte {
	if o.n == 0 {
		return []byte("{}")
	}
	o.b.WriteByte('}')
	return o.b.Bytes()
}

// jsonString writes s as a JSON string, with no HTML escaping.
func jsonString(s string) string {
	return string(jsonValue(s))
}

// jsonValue writes a host value as JSON: keys sorted, numbers in the
// shortest form that reads back as the same double, -0 kept.
func jsonValue(v any) []byte {
	var b bytes.Buffer
	writeValue(&b, v)
	return b.Bytes()
}

func writeValue(b *bytes.Buffer, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case float64:
		b.WriteString(formatNumber(v))
	case string:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(v)
		b.Write(bytes.TrimRight(buf.Bytes(), "\n"))
	case []any:
		b.WriteByte('[')
		for i, x := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writeValue(b, x)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			writeValue(b, k)
			b.WriteString(": ")
			writeValue(b, v[k])
		}
		b.WriteByte('}')
	default:
		panic("gen: not a host value")
	}
}

// formatNumber writes a number as ECMAScript's Number::toString does
// (01-language.md 6.3), which is also valid JSON; -0 is written "-0".
func formatNumber(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	if f < 0 {
		return "-" + formatNumber(-f)
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±x
	mant, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	k, n := len(digits), x+1
	switch {
	case k <= n && n <= 21:
		return digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + digits
	}
	out := digits[:1]
	if k > 1 {
		out += "." + digits[1:]
	}
	if n-1 >= 0 {
		return out + "e+" + strconv.Itoa(n-1)
	}
	return out + "e-" + strconv.Itoa(1-n)
}

// JSON writes the case as an object of a case file. It has no expect: the
// outcome command ignores expect (suite/README.md).
func (c *Case) JSON() []byte {
	var o obj
	o.field("name", jsonValue(c.Name))
	o.field("source", jsonValue(c.Text()))
	if len(c.Vars) > 0 {
		parts := make([]string, len(c.Vars))
		for i, v := range c.Vars {
			parts[i] = typeKey(v.Name) + ": " + v.Type
		}
		o.field("env", jsonValue("{"+strings.Join(parts, ", ")+"}"))
	}
	if len(c.Opaque) > 0 {
		names := make([]any, len(c.Opaque))
		for i, n := range c.Opaque {
			names[i] = n
		}
		o.field("opaque", jsonValue(names))
	}
	if len(c.Methods) > 0 {
		o.field("methods", stubsJSON(c.Methods))
	}
	if len(c.Functions) > 0 {
		o.field("functions", stubsJSON(c.Functions))
	}
	var vars obj
	for _, v := range c.Vars {
		if !v.Missing {
			vars.field(v.Name, jsonValue(v.Value))
		}
	}
	o.field("vars", vars.done())
	if c.NoCheck {
		o.field("check", []byte("false"))
	}
	if c.Broken {
		o.field("precondition", []byte("false"))
	}
	if l := c.Limits; l != nil && (l.Steps != nil || l.Allocation != nil || l.Depth != nil) {
		var lo obj
		if l.Steps != nil {
			lo.field("steps", []byte(strconv.Itoa(*l.Steps)))
		}
		if l.Allocation != nil {
			lo.field("allocation", []byte(strconv.Itoa(*l.Allocation)))
		}
		if l.Depth != nil {
			lo.field("depth", []byte(strconv.Itoa(*l.Depth)))
		}
		o.field("limits", lo.done())
	}
	return o.done()
}

func stubsJSON(stubs []Stub) []byte {
	var o obj
	for _, s := range stubs {
		var so obj
		so.field("type", jsonValue(s.Type))
		var rows bytes.Buffer
		rows.WriteByte('[')
		for i, r := range s.Rows {
			if i > 0 {
				rows.WriteString(", ")
			}
			var ro obj
			if r.HasArgs {
				args := r.Args
				if args == nil {
					args = []any{}
				}
				ro.field("args", jsonValue(args))
			}
			if r.IsError {
				ro.field("error", jsonValue(r.Error))
			} else {
				ro.field("result", jsonValue(r.Result))
			}
			if r.Charge > 0 {
				ro.field("charge", []byte(strconv.Itoa(r.Charge)))
			}
			if r.Allocate > 0 {
				ro.field("allocate", []byte(strconv.Itoa(r.Allocate)))
			}
			rows.Write(ro.done())
		}
		rows.WriteByte(']')
		so.field("calls", rows.Bytes())
		o.field(s.Name, so.done())
	}
	return o.done()
}

// File writes a case file: a JSON array of cases, one to a line.
func File(cases []*Case) []byte {
	var b bytes.Buffer
	b.WriteString("[\n")
	for i, c := range cases {
		b.Write(c.JSON())
		if i < len(cases)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("]\n")
	return b.Bytes()
}
