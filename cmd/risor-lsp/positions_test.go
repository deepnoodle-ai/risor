package main

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/internal/token"
	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func TestLineIndexLine(t *testing.T) {
	li := newLineIndex("a\r\nbc\n\nd")
	assert.Equal(t, li.line(0), "a\r") // \r is a column, as in the lexer
	assert.Equal(t, li.line(1), "bc")
	assert.Equal(t, li.line(2), "")
	assert.Equal(t, li.line(3), "d")
	assert.Equal(t, li.line(4), "")
	assert.Equal(t, li.line(-1), "")

	empty := newLineIndex("")
	assert.Equal(t, empty.line(0), "")
	assert.Equal(t, empty.line(1), "")
}

func TestLineIndexColumns(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		rune  int
		utf16 int
	}{
		{"ascii start", "let x = 1", 0, 0},
		{"ascii middle", "let x = 1", 4, 4},
		{"ascii end", "let x = 1", 9, 9},
		{"latin", "é = 1", 2, 2},
		{"cjk", "中文 = 1", 3, 3},
		{"emoji before", "😀 x", 2, 3},
		{"emoji after", "x 😀", 3, 4},
		{"two emoji", "😀😀x", 2, 4},
		{"mixed", "é中😀x", 4, 5},
		{"past end", "😀", 3, 4},
		{"past end of empty line", "", 2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := newLineIndex("first\n" + tt.line + "\nlast")
			assert.Equal(t, li.utf16Column(1, tt.rune), tt.utf16)
			assert.Equal(t, li.runeColumn(1, tt.utf16), tt.rune)
		})
	}
}

func TestLineIndexColumnInSurrogatePair(t *testing.T) {
	li := newLineIndex("a😀b")
	// UTF-16 column 2 is between the two halves of the emoji. It converts
	// to the column of the emoji.
	assert.Equal(t, li.runeColumn(0, 2), 1)
	assert.Equal(t, li.runeColumn(0, 3), 2)
}

func TestLineIndexOutOfRange(t *testing.T) {
	li := newLineIndex("😀")
	assert.Equal(t, li.utf16Column(0, -1), 0)
	assert.Equal(t, li.runeColumn(0, -1), 0)
	// A line that is not in the text has no characters to convert.
	assert.Equal(t, li.utf16Column(5, 3), 3)
	assert.Equal(t, li.runeColumn(5, 3), 3)
}

func TestLineIndexLSPPositions(t *testing.T) {
	li := newLineIndex("x = 1\n/* 😀 */ y")

	// Risor positions are 1-based; LSP positions are 0-based.
	assert.Equal(t, li.lspPosition(1, 1), protocol.Position{Line: 0, Character: 0})
	assert.Equal(t, li.lspPosition(2, 10), protocol.Position{Line: 1, Character: 10})

	line, column := li.risorPosition(protocol.Position{Line: 1, Character: 10})
	assert.Equal(t, line, 2)
	assert.Equal(t, column, 10)

	// A zero (unset) Risor position does not wrap around.
	assert.Equal(t, li.lspPosition(0, 0), protocol.Position{Line: 0, Character: 0})

	// token.Position is 0-based; "y" is at rune columns 8 to 9.
	rng := li.lspRange(
		token.Position{Line: 1, Column: 8},
		token.Position{Line: 1, Column: 9},
	)
	assert.Equal(t, rng, protocol.Range{
		Start: protocol.Position{Line: 1, Character: 9},
		End:   protocol.Position{Line: 1, Character: 10},
	})
}

// The emoji before a token is two UTF-16 code units, so the token's LSP
// columns are one more than its rune columns.

func TestDiagnosticRangeAfterEmoji(t *testing.T) {
	client := &recordingClient{}
	s := newTestServer()
	s.client = client
	uri := protocol.DocumentURI("file:///diag-emoji.risor")
	doc := newDocument(context.Background(), protocol.TextDocumentItem{
		URI: uri, Text: "let a = 1\n/* 😀 */ let x = )", Version: 1,
	})
	assert.NoError(t, s.cache.put(doc))

	s.publishDocumentDiagnostics(uri, doc)
	assert.Len(t, client.published, 1)
	assert.Len(t, client.published[0].Diagnostics, 1)
	// The ")" is at rune column 16 of line 1.
	assert.Equal(t, client.published[0].Diagnostics[0].Range, protocol.Range{
		Start: protocol.Position{Line: 1, Character: 17},
		End:   protocol.Position{Line: 1, Character: 17},
	})
}

func TestDefinitionAfterEmoji(t *testing.T) {
	s := newTestServer()
	uri := protocol.DocumentURI("file:///def-emoji.risor")
	openTestDocument(t, s, uri, "/* 😀 */ let café = 1\n/* 😀😀 */ café")

	// "café" on line 1 is at rune column 9, UTF-16 column 11.
	locations, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: position(uri, 1, 11),
	})
	assert.NoError(t, err)
	assert.Len(t, locations, 1)
	// "café" on line 0 is at rune columns 12 to 16.
	assert.Equal(t, locations[0].Range, protocol.Range{
		Start: protocol.Position{Line: 0, Character: 13},
		End:   protocol.Position{Line: 0, Character: 17},
	})
}

func TestHoverAfterEmoji(t *testing.T) {
	s := newTestServer()
	uri := protocol.DocumentURI("file:///hover-emoji.risor")
	openTestDocument(t, s, uri, "let x = 1\n/* 😀😀 */ x")

	// "x" on line 1 is at rune column 9, UTF-16 column 11.
	hover, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: position(uri, 1, 11),
	})
	assert.NoError(t, err)
	assert.NotNil(t, hover)
	assert.Contains(t, hover.Contents.Value, "**x**")
}

func TestDocumentSymbolAfterEmoji(t *testing.T) {
	s := newTestServer()
	uri := protocol.DocumentURI("file:///symbol-emoji.risor")
	openTestDocument(t, s, uri, "/* 😀 */ x = 1")

	symbols, err := s.DocumentSymbol(context.Background(), &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	assert.NoError(t, err)
	assert.Len(t, symbols, 1)
	symbol := symbols[0].(protocol.DocumentSymbol)
	// "x = 1" is at rune columns 8 to 13.
	assert.Equal(t, symbol.Range, protocol.Range{
		Start: protocol.Position{Line: 0, Character: 9},
		End:   protocol.Position{Line: 0, Character: 14},
	})
}

func TestFormattingEndsAtLastUTF16Column(t *testing.T) {
	s := newTestServer()
	uri := protocol.DocumentURI("file:///format-emoji.risor")
	openTestDocument(t, s, uri, "let a = 1\n  x = \"😀é\"")

	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	assert.NoError(t, err)
	assert.Len(t, edits, 1)
	// The last line is 10 runes, 14 bytes, and 11 UTF-16 code units.
	assert.Equal(t, edits[0].Range.End, protocol.Position{Line: 1, Character: 11})
}
