package format

import (
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
)

// unit is a token of the formatted text's view of the source: a token, or a
// whole template or text block with its interpolations (2.7 item 1).
type unit struct {
	tok        int // index of its first token
	last       int // index of its last token
	start, end int
	kind       syntax.TokenKind
}

// comment is one comment of the source (2.12).
type comment struct {
	start, end int
	text       string // as written by 2.12 item 9
	line       bool   // a line comment
	nl         bool   // the text holds a line end
	before     int    // line ends in the blank space directly before it
	after      int    // line ends in the blank space directly after it
	trailing   bool
	written    bool
}

// countLineEnds counts the line ends of a text: CR LF is one.
func countLineEnds(src []rune) int {
	n := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\n':
			n++
		case '\r':
			n++
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
		}
	}
	return n
}

// normalizeLineEnds writes every line end as U+000A.
func normalizeLineEnds(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// units groups the tokens into units. The EOF token is the last unit.
func units(toks []syntax.Token) []unit {
	var out []unit
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		u := unit{tok: i, last: i, start: t.Start, end: t.End, kind: t.Kind}
		if t.Kind == syntax.TemplateHead {
			depth := 0
			j := i
			for ; j < len(toks); j++ {
				switch toks[j].Kind {
				case syntax.TemplateHead:
					depth++
				case syntax.TemplateTail:
					depth--
				}
				if depth == 0 {
					break
				}
			}
			if j >= len(toks) {
				j = len(toks) - 1
			}
			u.last, u.end = j, toks[j].End
			i = j
		}
		out = append(out, u)
	}
	return out
}

// scanComments finds the comments in the gaps between units, from the
// offset from.
func scanComments(src []rune, us []unit, from int) []*comment {
	var out []*comment
	prevEnd := from
	for _, u := range us {
		gapEnd := u.start
		i := prevEnd
		for i < gapEnd {
			if src[i] == '/' && i+1 < gapEnd && src[i+1] == '/' {
				j := i
				for j < gapEnd && src[j] != '\n' && src[j] != '\r' {
					j++
				}
				t := strings.TrimRight(string(src[i:j]), " \t")
				out = append(out, &comment{start: i, end: j, text: t, line: true})
				i = j
				continue
			}
			if src[i] == '/' && i+1 < gapEnd && src[i+1] == '*' {
				j := i + 2
				for j+1 < gapEnd && !(src[j] == '*' && src[j+1] == '/') {
					j++
				}
				j += 2
				if j > gapEnd {
					j = gapEnd
				}
				t := normalizeLineEnds(string(src[i:j]))
				out = append(out, &comment{start: i, end: j, text: t, nl: strings.ContainsRune(t, '\n')})
				i = j
				continue
			}
			i++
		}
		if u.end > prevEnd {
			prevEnd = u.end
		}
	}
	return out
}

// lineEndsBetween counts the line ends in the blank space between two
// offsets that hold only blank space.
func lineEndsBetween(src []rune, a, b int) int {
	if a >= b {
		return 0
	}
	return countLineEnds(src[a:b])
}
