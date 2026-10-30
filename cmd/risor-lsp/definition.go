package main

import (
	"context"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v2/internal/token"
	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/rs/zerolog/log"
)

func (s *Server) Definition(ctx context.Context, params *protocol.DefinitionParams) (protocol.Definition, error) {
	doc, err := s.cache.get(params.TextDocument.URI)
	if err != nil {
		log.Error().Err(err).Str("call", "Definition").Msg("failed to get document")
		return nil, nil
	}

	if doc.ast == nil || doc.err != nil {
		return nil, nil
	}

	lines := newLineIndex(doc.item.Text)
	line, column := lines.risorPosition(params.Position)

	// Find the symbol at the cursor position
	symbol := findSymbolAtPosition(doc.ast, line, column)
	if symbol == "" {
		return nil, nil
	}

	// Find the definition of the symbol
	def := findDefinition(doc.ast, lines, symbol)
	if def != nil {
		def.URI = params.TextDocument.URI
		return []protocol.Location{*def}, nil
	}

	return nil, nil
}

// findDefinition finds the definition location of a symbol. lines is the
// lineIndex of the program's text, which converts the range to LSP positions.
func findDefinition(program *ast.Program, lines *lineIndex, symbol string) *protocol.Location {
	for _, stmt := range program.Stmts {
		switch s := stmt.(type) {
		case *ast.Var:
			if s.Name.Name == symbol {
				return nameLocation(lines, s.Name.Pos(), symbol)
			}
		case *ast.Assign:
			if assignName(s) == symbol {
				return nameLocation(lines, s.Pos(), symbol)
			}
		}
	}

	return nil
}

// nameLocation returns the location of name, which starts at pos. The URI is
// left for the caller to fill in.
func nameLocation(lines *lineIndex, pos token.Position, name string) *protocol.Location {
	line := pos.LineNumber()
	column := pos.ColumnNumber()
	return &protocol.Location{
		Range: protocol.Range{
			Start: lines.lspPosition(line, column),
			End:   lines.lspPosition(line, column+utf8.RuneCountInString(name)),
		},
	}
}
