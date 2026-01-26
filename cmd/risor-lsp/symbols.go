package main

import (
	"context"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/token"
	"github.com/rs/zerolog/log"
)

func (s *Server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	doc, err := s.cache.get(params.TextDocument.URI)
	if err != nil {
		log.Error().Err(err).Str("call", "DocumentSymbol").Msg("failed to get document")
		return nil, nil
	}
	if doc.err != nil {
		log.Error().Err(doc.err).Str("call", "DocumentSymbol").Msg("document has error")
		return nil, nil
	}
	if doc.ast == nil {
		return nil, nil
	}

	var symbols []protocol.DocumentSymbol

	for _, stmt := range doc.ast.Statements() {
		switch stmt := stmt.(type) {
		case *ast.Var:
			name, _ := stmt.Value()
			if name != "" {
				pos := stmt.Token().StartPosition
				endPos := stmt.Token().EndPosition

				symbols = append(symbols, protocol.DocumentSymbol{
					Name: name,
					Kind: 13, // Variable
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() - 1),
						},
						End: protocol.Position{
							Line:      uint32(endPos.LineNumber() - 1),
							Character: uint32(endPos.ColumnNumber() - 1),
						},
					},
					SelectionRange: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() - 1),
						},
						End: protocol.Position{
							Line:      uint32(endPos.LineNumber() - 1),
							Character: uint32(endPos.ColumnNumber() - 1),
						},
					},
				})
			}

		// Functions are typically assigned to variables in Risor
		// We could enhance this to traverse into expressions to find function literals

		case *ast.Assign:
			name := stmt.Name()
			if name != "" {
				pos := stmt.Token().StartPosition
				endPos := stmt.Token().EndPosition

				symbols = append(symbols, protocol.DocumentSymbol{
					Name: name,
					Kind: 13, // Variable
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() - 1),
						},
						End: protocol.Position{
							Line:      uint32(endPos.LineNumber() - 1),
							Character: uint32(endPos.ColumnNumber() - 1),
						},
					},
					SelectionRange: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(pos.LineNumber() - 1),
							Character: uint32(pos.ColumnNumber() - 1),
						},
						End: protocol.Position{
							Line:      uint32(endPos.LineNumber() - 1),
							Character: uint32(endPos.ColumnNumber() - 1),
						},
					},
				})
			}
		}
	}

	// Convert to interface{} slice
	result := make([]interface{}, len(symbols))
	for i, symbol := range symbols {
		result[i] = symbol
	}

	return result, nil
}

// getLastToken attempts to get the last token from a statement
func getLastToken(stmt ast.Statement) token.Token {
	// This is a simplified approach - in a real implementation,
	// you'd want to traverse the AST to find the actual last token
	return token.Token{} // Return empty token as fallback
}
