package main

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/rs/zerolog/log"
)

func (s *Server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	doc, err := s.cache.get(params.TextDocument.URI)
	if err != nil {
		log.Error().Err(err).Str("call", "Formatting").Msg("failed to get document")
		return nil, nil
	}

	// For now, implement basic formatting: ensure proper indentation and spacing
	text := doc.item.Text
	lines := strings.Split(text, "\n")

	var formattedLines []string
	indentLevel := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			formattedLines = append(formattedLines, "")
			continue
		}

		// Decrease indent for closing braces/brackets
		if strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, "]") {
			if indentLevel > 0 {
				indentLevel--
			}
		}

		// Apply indentation
		indent := strings.Repeat("  ", indentLevel) // 2 spaces per level
		formattedLine := indent + trimmed
		formattedLines = append(formattedLines, formattedLine)

		// Increase indent for opening braces/brackets
		if strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, "[") {
			indentLevel++
		}
	}

	formattedText := strings.Join(formattedLines, "\n")

	// If no changes needed, return nil
	if formattedText == text {
		return nil, nil
	}

	// The edit replaces the entire document, through the end of its last line
	index := newLineIndex(text)
	lastLine := len(lines) - 1
	lastColumn := utf8.RuneCountInString(lines[lastLine])

	return []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   index.lspPosition(lastLine+1, lastColumn+1),
			},
			NewText: formattedText,
		},
	}, nil
}
