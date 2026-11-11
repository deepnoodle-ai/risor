// Package render writes diagnostics as text in the three forms of package
// F8 5: one line, detail and JSON. Each form is the same text, code point
// for code point, in every implementation.
package render

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// Line is the one-line form of each diagnostic (5.2).
func Line(ds []diag.Diagnostic) string {
	var b strings.Builder
	for i := range ds {
		line(&b, &ds[i])
		b.WriteByte('\n')
	}
	return b.String()
}

// Detail is the detail form of each diagnostic (5.3). With hasSource
// false there is no source text, and no excerpt is written.
func Detail(ds []diag.Diagnostic, source string, hasSource bool) string {
	var b strings.Builder
	var lines []string
	if hasSource {
		lines = strings.Split(strings.TrimPrefix(source, "\uFEFF"), "\n")
	}
	for i := range ds {
		d := &ds[i]
		line(&b, d)
		b.WriteByte('\n')
		if d.Span != nil && hasSource {
			excerpt(&b, lines, *d.Span)
		}
		if d.Phase == diag.PhaseRun {
			for j := len(d.Trace) - 1; j >= 0; j-- {
				t := d.Trace[j]
				b.WriteString("  in ")
				b.WriteString(t.Name)
				if t.Index >= 0 {
					b.WriteString(", element ")
					b.WriteString(strconv.Itoa(t.Index))
				}
				b.WriteString(", at ")
				span(&b, t.Span)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// JSON is the JSON form of each diagnostic, one object to a line (5.5).
func JSON(ds []diag.Diagnostic) string {
	var b strings.Builder
	for i := range ds {
		jsonOf(&b, &ds[i])
		b.WriteByte('\n')
	}
	return b.String()
}

func line(b *strings.Builder, d *diag.Diagnostic) {
	b.WriteString("error[")
	b.WriteString(d.Code)
	b.WriteByte(']')
	if d.Span != nil {
		b.WriteByte(' ')
		span(b, *d.Span)
	}
	b.WriteString(": ")
	b.WriteString(Visible(d.Message))
	if len(d.Path) > 0 {
		b.WriteString(" at ")
		b.WriteString(Path(d.Path))
	}
}

func span(b *strings.Builder, s diag.Span) {
	b.WriteString(strconv.Itoa(s.Start.Line))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(s.Start.Column))
	b.WriteByte('-')
	b.WriteString(strconv.Itoa(s.End.Line))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(s.End.Column))
}

// Path writes a path from its steps (5.1 item 5).
func Path(p []any) string {
	var b strings.Builder
	for _, s := range p {
		switch s := s.(type) {
		case string:
			if value.IsWord(s) {
				b.WriteByte('.')
				b.WriteString(value.Cut(s))
			} else {
				b.WriteByte('[')
				b.WriteString(value.Cut(value.QuoteJSON(s)))
				b.WriteByte(']')
			}
		case int:
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(s))
			b.WriteByte(']')
		case float64:
			b.WriteByte('[')
			b.WriteString(value.FormatNumber(s))
			b.WriteByte(']')
		}
	}
	return b.String()
}

// Visible writes each control character as an escape (F8 1.1).
func Visible(s string) string {
	clean := true
	for _, r := range s {
		if r < 0x20 || (r >= 0x7F && r <= 0x9F) {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	const hex = "0123456789abcdef"
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || (r >= 0x7F && r <= 0x9F):
			b.WriteString(`\u00`)
			b.WriteByte(hex[r>>4])
			b.WriteByte(hex[r&15])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// excerpt writes the excerpt and the caret line (5.4).
func excerpt(b *strings.Builder, lines []string, sp diag.Span) {
	l := sp.Start.Line
	var text []rune
	if l >= 1 && l <= len(lines) {
		text = []rune(lines[l-1])
	}
	n := len(text)
	a := sp.Start.Column
	bb := n + 1
	if sp.End.Line == l {
		bb = sp.End.Column
	}
	if bb <= a {
		bb = a + 1
	}
	s := 1
	if n > 120 {
		s = a - 40
		if s < 1 {
			s = 1
		}
		if s > n-119 {
			s = n - 119
		}
	}
	b.WriteString("    ")
	lead := 0
	if s > 1 {
		b.WriteString("...")
		lead = 3
	}
	end := s + 119 // the last column written
	if end > n {
		end = n
	}
	for c := s; c <= end; c++ {
		r := text[c-1]
		if r < 0x20 || (r >= 0x7F && r <= 0x9F) {
			r = ' '
		}
		b.WriteRune(r)
	}
	if n > 120 && s+120 <= n {
		b.WriteString("...")
	}
	b.WriteString("\n    ")
	b.WriteString(strings.Repeat(" ", lead+a-s))
	stop := bb
	if s+120 < stop {
		stop = s + 120
	}
	k := stop - a
	if k < 1 {
		k = 1
	}
	b.WriteString(strings.Repeat("^", k))
	b.WriteByte('\n')
}

func jsonOf(b *strings.Builder, d *diag.Diagnostic) {
	b.WriteByte('{')
	if d.Class != "" {
		b.WriteString(`"class":`)
		b.WriteString(value.QuoteJSON(d.Class))
		b.WriteByte(',')
	}
	b.WriteString(`"code":`)
	b.WriteString(value.QuoteJSON(d.Code))
	if d.Code == "failed" && d.HasData {
		b.WriteString(`,"data":`)
		b.WriteString(Data(d.Data))
	}
	b.WriteString(`,"message":`)
	b.WriteString(value.QuoteJSON(d.Message))
	if d.Code == "not-conforming" || d.Path != nil {
		b.WriteString(`,"path":[`)
		for i, s := range d.Path {
			if i > 0 {
				b.WriteByte(',')
			}
			switch s := s.(type) {
			case string:
				b.WriteString(value.QuoteJSON(s))
			case int:
				b.WriteString(strconv.Itoa(s))
			case float64:
				b.WriteString(value.FormatNumber(s))
			}
		}
		b.WriteByte(']')
	}
	b.WriteString(`,"phase":`)
	b.WriteString(value.QuoteJSON(string(d.Phase)))
	if d.Span != nil {
		b.WriteString(`,"span":`)
		spanJSON(b, *d.Span)
	}
	if d.Phase == diag.PhaseRun {
		b.WriteString(`,"trace":[`)
		for i, t := range d.Trace {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"index":`)
			if t.Index < 0 {
				b.WriteString("null")
			} else {
				b.WriteString(strconv.Itoa(t.Index))
			}
			b.WriteString(`,"name":`)
			b.WriteString(value.QuoteJSON(t.Name))
			b.WriteString(`,"span":`)
			spanJSON(b, t.Span)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
}

func spanJSON(b *strings.Builder, s diag.Span) {
	b.WriteByte('[')
	b.WriteString(strconv.Itoa(s.Start.Line))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(s.Start.Column))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(s.End.Line))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(s.End.Column))
	b.WriteByte(']')
}

// The bounds of data in the JSON form (5.6).
const (
	maxDataDepth  = 200
	maxDataPoints = 10000
)

// Data is the JSON form of the data of failed (5.6): each opaque value is
// null, and the whole is null when it would nest deeper than 200 or hold
// more than 10,000 code points. It writes at most that much.
func Data(v any) string {
	w := &dataWriter{}
	if !w.walk(v, 0) {
		return "null"
	}
	return w.b.String()
}

type dataWriter struct {
	b        strings.Builder
	pos, cps int
}

// put counts the code points written since the last count, and reports
// whether the text is still within the bound.
func (w *dataWriter) put() bool {
	s := w.b.String()
	w.cps += utf8.RuneCountInString(s[w.pos:])
	w.pos = len(s)
	return w.cps <= maxDataPoints
}

func (w *dataWriter) str(s string) bool {
	if utf8.RuneCountInString(s) > maxDataPoints-w.cps { // the escaped form is no shorter
		return false
	}
	w.b.WriteByte('"')
	value.WriteEscaped(&w.b, s)
	w.b.WriteByte('"')
	return w.put()
}

func (w *dataWriter) walk(v any, depth int) bool {
	v, _ = value.Read(v)
	switch value.KindOf(v) {
	case value.Bool:
		if v.(bool) {
			w.b.WriteString("true")
		} else {
			w.b.WriteString("false")
		}
		return w.put()
	case value.Number:
		w.b.WriteString(value.FormatNumber(v.(float64)))
		return w.put()
	case value.String:
		return w.str(v.(string))
	case value.List:
		if depth+1 > maxDataDepth {
			return false
		}
		n := value.Len(v)
		if 2*n+1 > maxDataPoints-w.cps {
			return false
		}
		w.b.WriteByte('[')
		for i := 0; i < n; i++ {
			if i > 0 {
				w.b.WriteByte(',')
			}
			if !w.walk(value.At(v, i), depth+1) {
				return false
			}
		}
		w.b.WriteByte(']')
		return w.put()
	case value.Map:
		if depth+1 > maxDataDepth {
			return false
		}
		if 4*value.Len(v)+1 > maxDataPoints-w.cps {
			return false
		}
		w.b.WriteByte('{')
		for i, k := range value.Keys(v) {
			if i > 0 {
				w.b.WriteByte(',')
			}
			if !w.str(k) {
				return false
			}
			w.b.WriteByte(':')
			x, _ := value.Get(v, k)
			if !w.walk(x, depth+1) {
				return false
			}
		}
		w.b.WriteByte('}')
		return w.put()
	}
	// null, and an opaque value
	w.b.WriteString("null")
	return w.put()
}
