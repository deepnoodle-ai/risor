package main

import (
	"unicode/utf16"

	"github.com/deepnoodle-ai/risor/v2/internal/token"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

// Risor and LSP count positions differently:
//
//   - Risor lines and columns are 1-based, and columns count Unicode code
//     points (runes).
//   - LSP lines and characters are 0-based, and Position.character counts
//     UTF-16 code units.
//
// The column counts agree for characters in the Basic Multilingual Plane.
// They differ for characters outside it, such as emoji, which are one rune
// but two UTF-16 code units. So every position that crosses the LSP boundary
// goes through a lineIndex, which converts columns using the text of the
// line they are on.

// lineIndex finds the lines of a document's text, to convert positions
// between Risor and LSP. Lines end at "\n", as in the Risor lexer.
type lineIndex struct {
	text   string
	starts []int // byte offset of the start of each line
}

// newLineIndex returns a lineIndex for text.
func newLineIndex(text string) *lineIndex {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &lineIndex{text: text, starts: starts}
}

// line returns the text of the 0-based line n, without its "\n". It returns
// "" for a line that is not in the text.
func (li *lineIndex) line(n int) string {
	if n < 0 || n >= len(li.starts) {
		return ""
	}
	end := len(li.text)
	if n+1 < len(li.starts) {
		end = li.starts[n+1] - 1
	}
	return li.text[li.starts[n]:end]
}

// utf16Column converts a 0-based rune column on the 0-based line to a
// 0-based UTF-16 column. A column past the end of the line counts one code
// unit for each rune past the end, so the conversion does not move it. A
// negative column converts to 0.
func (li *lineIndex) utf16Column(line, runeCol int) int {
	if runeCol <= 0 {
		return 0
	}
	units := 0
	for _, r := range li.line(line) {
		units += utf16Len(r)
		runeCol--
		if runeCol == 0 {
			return units
		}
	}
	return units + runeCol
}

// runeColumn converts a 0-based UTF-16 column on the 0-based line to a
// 0-based rune column. A column in the middle of a surrogate pair converts to
// the column of the character that the pair encodes. A column past the end
// of the line counts one rune for each code unit past the end, so the
// conversion does not move it. A negative column converts to 0.
func (li *lineIndex) runeColumn(line, utf16Col int) int {
	if utf16Col <= 0 {
		return 0
	}
	runes := 0
	for _, r := range li.line(line) {
		n := utf16Len(r)
		if utf16Col < n {
			return runes
		}
		utf16Col -= n
		runes++
		if utf16Col == 0 {
			return runes
		}
	}
	return runes + utf16Col
}

// lspPosition converts a Risor position, a 1-based line and a 1-based rune
// column, to an LSP position.
func (li *lineIndex) lspPosition(line, column int) protocol.Position {
	line = max(line-1, 0)
	return protocol.Position{
		Line:      uint32(line),
		Character: uint32(li.utf16Column(line, column-1)),
	}
}

// lspRange converts a range of Risor positions to an LSP range.
func (li *lineIndex) lspRange(start, end token.Position) protocol.Range {
	return protocol.Range{
		Start: li.lspPosition(start.LineNumber(), start.ColumnNumber()),
		End:   li.lspPosition(end.LineNumber(), end.ColumnNumber()),
	}
}

// risorPosition converts an LSP position to a Risor position, a 1-based line
// and a 1-based rune column.
func (li *lineIndex) risorPosition(p protocol.Position) (line, column int) {
	return int(p.Line) + 1, li.runeColumn(int(p.Line), int(p.Character)) + 1
}

// utf16Len returns the number of UTF-16 code units that encode r. Ranging
// over a string gives utf8.RuneError for invalid UTF-8, so r is always a
// valid rune here, but count 1 for one that is not.
func utf16Len(r rune) int {
	if n := utf16.RuneLen(r); n > 0 {
		return n
	}
	return 1
}
