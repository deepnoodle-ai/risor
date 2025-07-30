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
	for _, stmt := range program.Statements() {
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
		_, value := n.Value()
		if value != nil {
			if symbol := findIdentInNode(value, line, column); symbol != "" {
				return symbol
			}
		}
		// Also check if we can find any identifiers by traversing the statement
		return findIdentInNodeRecursive(n, line, column)
	case *ast.Assign:
		name := n.Name()
		pos := n.Token().StartPosition
		endPos := n.Token().EndPosition

		if pos.LineNumber() == line && pos.ColumnNumber() <= column && column <= endPos.ColumnNumber() {
			return name
		}
		// Also check the value expression
		if n.Value() != nil {
			if symbol := findIdentInNode(n.Value(), line, column); symbol != "" {
				return symbol
			}
		}
	case *ast.Func:
		if n.Name() != nil {
			name := n.Name().String()
			pos := n.Token().StartPosition
			endPos := n.Token().EndPosition

			if pos.LineNumber() == line && pos.ColumnNumber() <= column && column <= endPos.ColumnNumber() {
				return name
			}
		}
	case *ast.Ident:
		// Check if this identifier is at the requested position
		pos := n.Token().StartPosition
		endPos := n.Token().EndPosition
		identValue := n.String()

		if pos.LineNumber() == line && pos.ColumnNumber() <= column && column <= endPos.ColumnNumber() {
			return identValue
		}
	case *ast.Call:
		// Check the function being called
		if function := n.Function(); function != nil {
			if symbol := findIdentInNode(function, line, column); symbol != "" {
				return symbol
			}
		}
		// Check arguments
		for _, arg := range n.Arguments() {
			if symbol := findIdentInNode(arg, line, column); symbol != "" {
				return symbol
			}
		}
	case *ast.Index:
		// Check the left side (what's being indexed)
		if left := n.Left(); left != nil {
			if symbol := findIdentInNode(left, line, column); symbol != "" {
				return symbol
			}
		}
		// Check the index expression
		if index := n.Index(); index != nil {
			if symbol := findIdentInNode(index, line, column); symbol != "" {
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
		pos := ident.Token().StartPosition
		endPos := ident.Token().EndPosition
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
		name, _ := n.Value()
		varPos := n.Token().StartPosition

		if varPos.LineNumber() == line {
			var identStartCol, identEndCol int
			if n.IsWalrus() {
				// For walrus operator "y := value", the token position is at the identifier
				identStartCol = varPos.ColumnNumber()
				identEndCol = identStartCol + len(name)
			} else {
				// For "var x = value", the identifier 'x' should be after 'var '
				identStartCol = varPos.ColumnNumber() + 4 // after "var "
				identEndCol = identStartCol + len(name)
			}
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
	for _, stmt := range program.Statements() {
		switch s := stmt.(type) {
		case *ast.Assign:
			assignName := s.Name()

			if assignName == symbol {
				return fmt.Sprintf("**%s** - Variable\n\nAssigned in this file", symbol)
			}
		case *ast.Var:
			name, _ := s.Value()
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
