package main

import (
	"context"
	"fmt"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/risor-io/risor/ast"
)

func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc, err := s.cache.get(params.TextDocument.URI)
	if err != nil {
		return nil, nil
	}

	if doc.ast == nil {
		return nil, nil
	}

	if doc.err != nil {
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

	// Generate hover information for the symbol
	info := getSymbolInfo(symbol, doc.ast)

	if info == "" {
		// Check if it's a built-in function
		if contains(risorBuiltins, symbol) {
			info = fmt.Sprintf("**%s()** - Built-in function", symbol)
		} else if contains(risorKeywords, symbol) {
			info = fmt.Sprintf("**%s** - Risor keyword", symbol)
		} else {
			return nil, nil
		}
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: info,
		},
	}, nil
}

// findSymbolAtPosition finds the symbol (identifier) at the given line/column position
func findSymbolAtPosition(program *ast.Program, line, column int) string {
	// Simple implementation: look through statements for identifiers at the position
	for _, stmt := range program.Stmts {
		if symbol := findIdentInNode(stmt, line, column); symbol != "" {
			return symbol
		}
	}
	return ""
}

// findIdentInNode recursively searches for identifiers in a node
func findIdentInNode(node ast.Node, line, column int) string {
	switch n := node.(type) {
	case *ast.Var:
		// For var statements, we need to check all child expressions recursively
		if n.Value != nil {
			if symbol := findIdentInNode(n.Value, line, column); symbol != "" {
				return symbol
			}
		}
		// Also check if we can find any identifiers by traversing the statement
		return findIdentInNodeRecursive(n, line, column)
	case *ast.Assign:
		name := n.Name.Name
		pos := n.Pos()
		endPos := n.End()

		if pos.LineNumber() == line && pos.ColumnNumber() <= column && column <= endPos.ColumnNumber() {
			return name
		}
		// Also check the value expression
		if n.Value != nil {
			if symbol := findIdentInNode(n.Value, line, column); symbol != "" {
				return symbol
			}
		}
	case *ast.Func:
		if n.Name != nil {
			name := n.Name.String()
			pos := n.Pos()
			endPos := n.End()

			if pos.LineNumber() == line && pos.ColumnNumber() <= column && column <= endPos.ColumnNumber() {
				return name
			}
		}
	case *ast.Ident:
		// Check if this identifier is at the requested position
		pos := n.Pos()
		endPos := n.End()
		identValue := n.String()

		if pos.LineNumber() == line && pos.ColumnNumber() <= column && column <= endPos.ColumnNumber() {
			return identValue
		}
	case *ast.Call:
		// Check the function being called
		if n.Fun != nil {
			if symbol := findIdentInNode(n.Fun, line, column); symbol != "" {
				return symbol
			}
		}
		// Check arguments
		for _, arg := range n.Args {
			if symbol := findIdentInNode(arg, line, column); symbol != "" {
				return symbol
			}
		}
	case *ast.Index:
		// Check the left side (what's being indexed)
		if n.X != nil {
			if symbol := findIdentInNode(n.X, line, column); symbol != "" {
				return symbol
			}
		}
		// Check the index expression
		if n.Index != nil {
			if symbol := findIdentInNode(n.Index, line, column); symbol != "" {
				return symbol
			}
		}
	default:
	}

	return ""
}

// findIdentInNodeRecursive performs a more thorough recursive search through node children
func findIdentInNodeRecursive(node ast.Node, line, column int) string {
	// Check if this node itself is an identifier
	if ident, ok := node.(*ast.Ident); ok {
		pos := ident.Pos()
		endPos := ident.End()
		identValue := ident.String()
		if pos.LineNumber() == line && pos.ColumnNumber() <= column && column <= endPos.ColumnNumber() {
			return identValue
		}
	}

	// For statements like Var, we need to use reflection or other methods to traverse
	// Since we can't access private fields, let's try a different approach
	// by checking if the position matches the expected identifier positions

	switch n := node.(type) {
	case *ast.Var:
		name := n.Name.Name
		varPos := n.Pos()

		if varPos.LineNumber() == line {
			// For "let x = value", the identifier 'x' is after "let "
			identStartCol := varPos.ColumnNumber() + 4 // after "let "
			identEndCol := identStartCol + len(name)
			if identStartCol <= column && column <= identEndCol {
				return name
			}
		}
	default:
	}
	return ""
}

// getSymbolInfo returns documentation or information about a symbol
func getSymbolInfo(symbol string, program *ast.Program) string {
	// Check variables and functions in all statements
	for _, stmt := range program.Stmts {
		switch s := stmt.(type) {
		case *ast.Assign:
			assignName := s.Name.Name

			if assignName == symbol {
				return fmt.Sprintf("**%s** - Variable\n\nAssigned in this file", symbol)
			}
		case *ast.Var:
			name := s.Name.Name
			if name == symbol {
				return fmt.Sprintf("**%s** - Variable\n\nDeclared in this file", symbol)
			}
		// Check if statement contains expressions with functions (e.g., function assignments)
		default:
			// For now, we don't traverse into expressions within statements
			// This could be enhanced later to find functions assigned to variables
		}
	}
	return ""
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
