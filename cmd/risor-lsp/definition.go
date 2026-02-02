package main

import (
	"context"

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

	// Convert LSP position to 1-based line/column
	line := int(params.Position.Line) + 1
	column := int(params.Position.Character) + 1

	// Find the symbol at the cursor position
	symbol := findSymbolAtPosition(doc.ast, line, column)
	if symbol == "" {
		return nil, nil
	}

	// Find the definition of the symbol
	def := findDefinition(doc.ast, symbol)
	if def != nil {
		return []protocol.Location{*def}, nil
	}

	return nil, nil
}

// findDefinition finds the definition location of a symbol
func findDefinition(program *ast.Program, symbol string) *protocol.Location {
	for _, stmt := range program.Stmts {
		switch s := stmt.(type) {
		case *ast.Var:
			name := s.Name.Name
			if name == symbol {
				pos := s.Pos()
				return &protocol.Location{
					URI: "", // Will be filled in by caller
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() - 1),
						},
						End: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() + len(name) - 1),
						},
					},
				}
			}
		case *ast.Assign:
			name := s.Name.Name
			if name == symbol {
				pos := s.Pos()
				return &protocol.Location{
					URI: "", // Will be filled in by caller
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() - 1),
						},
						End: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() + len(name) - 1),
						},
					},
				}
			}
		}
	}

	return nil
}
