package syntax

import (
	"sort"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
)

// File is a source text as code points, with the table that turns an offset
// into a line and a column (03-host.md 3.7.7).
type File struct {
	src        []rune
	lineStarts []int
}

// NewFile builds a File from source text. Bytes that are not valid UTF-8
// become U+FFFD, which begins no token. A U+FEFF at the start is removed and
// is not counted in positions (01-language.md 2.1 rule 2).
func NewFile(source string) *File {
	src := []rune(source)
	if len(src) > 0 && src[0] == 0xFEFF {
		src = src[1:]
	}
	starts := []int{0}
	for i, r := range src {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &File{src: src, lineStarts: starts}
}

// Len returns the number of code points in the text.
func (f *File) Len() int { return len(f.src) }

// Runes returns the text as code points. The caller must not change it.
func (f *File) Runes() []rune { return f.src }

// Pos returns the position of a code point offset.
func (f *File) Pos(off int) diag.Pos {
	i := sort.Search(len(f.lineStarts), func(i int) bool { return f.lineStarts[i] > off }) - 1
	return diag.Pos{Line: i + 1, Column: off - f.lineStarts[i] + 1}
}

// Span returns the span of two code point offsets.
func (f *File) Span(start, end int) diag.Span {
	return diag.Span{Start: f.Pos(start), End: f.Pos(end)}
}

// Text returns the source text between two offsets.
func (f *File) Text(start, end int) string { return string(f.src[start:end]) }
