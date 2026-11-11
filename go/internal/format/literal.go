package format

import (
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
)

// textBlock is the part of a text block (2.7 item 2).
func (b *builder) textBlock(u unit) part {
	src := b.src
	// The pieces of this block: its own head, middles and tail.
	var pieces [][2]int
	if u.kind == syntax.TemplateHead {
		depth := 0
		for i := u.tok; i <= u.last; i++ {
			t := b.toks[i]
			switch t.Kind {
			case syntax.TemplateHead:
				depth++
				if depth == 1 {
					pieces = append(pieces, [2]int{t.Start, t.End})
				}
			case syntax.TemplateMiddle:
				if depth == 1 {
					pieces = append(pieces, [2]int{t.Start, t.End})
				}
			case syntax.TemplateTail:
				if depth == 1 {
					pieces = append(pieces, [2]int{t.Start, t.End})
				}
				depth--
			}
		}
	}
	inInterp := func(p int) bool {
		for k := 0; k+1 < len(pieces); k++ {
			if pieces[k][1] <= p && p <= pieces[k+1][0] {
				return true
			}
		}
		return false
	}
	delim := 3
	if src[u.start] == 'r' {
		delim = 4
	}
	open := string(src[u.start : u.start+delim])
	p := u.start + delim
	for p < u.end && (src[p] == ' ' || src[p] == '\t') {
		p++
	}
	if p < u.end && src[p] == '\r' && p+1 < u.end && src[p+1] == '\n' {
		p += 2
	} else if p < u.end && (src[p] == '\n' || src[p] == '\r') {
		p++
	}
	contentStart := p
	closeQuote := u.end - 3
	closeLine := closeQuote
	for closeLine > contentStart && (src[closeLine-1] == ' ' || src[closeLine-1] == '\t') {
		closeLine--
	}
	indentLen := closeQuote - closeLine
	type line struct {
		text   string
		interp bool
		blank  bool
	}
	var lines []line
	for p < closeLine {
		q := p
		for q < closeLine && src[q] != '\n' && src[q] != '\r' {
			q++
		}
		ln := line{interp: inInterp(p)}
		raw := src[p:q]
		ln.blank = strings.Trim(string(raw), " \t") == ""
		if ln.interp {
			ln.text = string(raw)
		} else if !ln.blank {
			if len(raw) >= indentLen {
				ln.text = string(raw[indentLen:])
			} else {
				ln.text = string(raw)
			}
		}
		lines = append(lines, ln)
		if q < len(src) && src[q] == '\r' && q+1 < len(src) && src[q+1] == '\n' {
			p = q + 2
		} else {
			p = q + 1
		}
	}
	return &textBlock{open: delim, render: func(j int) string {
		pad := strings.Repeat(" ", j)
		var sb strings.Builder
		sb.WriteString(open)
		sb.WriteByte('\n')
		for _, ln := range lines {
			switch {
			case ln.interp:
				sb.WriteString(ln.text)
			case ln.blank:
			default:
				sb.WriteString(pad)
				sb.WriteString(ln.text)
			}
			sb.WriteByte('\n')
		}
		sb.WriteString(pad)
		sb.WriteString(`"""`)
		return sb.String()
	}}
}

// Template formats a template text (2.14). It returns the formatted text,
// or the first parse diagnostic.
func Template(source string) (string, *syntax.Error, *syntax.File) {
	f := syntax.NewTemplateFile(source)
	t, err := syntax.ParseTemplate(f)
	if err != nil {
		return "", err, f
	}
	src := f.Runes()
	var sb strings.Builder
	for _, s := range t.Segments {
		whole := string(src[s.Span.Start:s.Span.End])
		if s.Expr == nil {
			sb.WriteString(whole)
			continue
		}
		toks := syntax.PartTokens(f, s)
		e := s.Expr
		from := s.Span.Start + 3
		hasLine := false
		for _, c := range scanComments(src, units(toks), from) {
			if c.line {
				hasLine = true
			}
		}
		out := run(src, toks, from, func(b *builder) part {
			d := b.node(e)
			return []part{d, b.closeLead(len(b.us)-1, true)}
		}, 1<<30)
		out = strings.TrimRight(out, " ")
		if hasLine || strings.ContainsRune(out, '\n') {
			sb.WriteString(whole)
			continue
		}
		sb.WriteString("${{ " + strings.TrimLeft(out, " ") + " }}")
	}
	return sb.String(), nil, f
}
