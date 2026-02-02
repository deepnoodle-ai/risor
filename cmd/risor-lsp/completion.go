package main

import (
	"context"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/deepnoodle-ai/risor/v2/ast"
	"github.com/rs/zerolog/log"
)

// Risor keywords for completion
var risorKeywords = []string{
	"case", "catch", "const", "default", "else", "false", "finally",
	"function", "if", "in", "let", "nil", "not", "return", "struct",
	"switch", "throw", "true", "try",
}

// Common built-in functions
var risorBuiltins = []string{
	"all", "any", "assert", "bool", "byte", "call", "chunk", "coalesce",
	"decode", "encode", "filter", "float", "getattr",
	"int", "keys", "len", "list", "reversed",
	"sorted", "sprintf", "string", "type",
}

// Common modules
var risorModules = []string{
	"math", "rand", "regexp", "strings", "time",
}

func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	doc, err := s.cache.get(params.TextDocument.URI)
	if err != nil {
		log.Error().Err(err).Str("call", "Completion").Msg("failed to get document")
		return &protocol.CompletionList{IsIncomplete: false, Items: nil}, nil
	}

	var items []protocol.CompletionItem

	// Add keywords
	for _, keyword := range risorKeywords {
		items = append(items, protocol.CompletionItem{
			Label:  keyword,
			Kind:   14, // Keyword
			Detail: "Risor keyword",
		})
	}

	// Add built-in functions
	for _, builtin := range risorBuiltins {
		items = append(items, protocol.CompletionItem{
			Label:      builtin,
			Kind:       3, // Function
			Detail:     "Built-in function",
			InsertText: builtin + "()",
		})
	}

	// Add modules
	for _, module := range risorModules {
		items = append(items, protocol.CompletionItem{
			Label:  module,
			Kind:   9, // Module
			Detail: "Risor module",
		})
	}

	// Add variables from the current document's AST
	if doc.ast != nil && doc.err == nil {
		variables := extractVariables(doc.ast)
		for _, variable := range variables {
			items = append(items, protocol.CompletionItem{
				Label:  variable,
				Kind:   6, // Variable
				Detail: "Variable",
			})
		}

		// Add functions from the current document's AST
		functions := extractFunctions(doc.ast)
		for _, function := range functions {
			items = append(items, protocol.CompletionItem{
				Label:      function,
				Kind:       3, // Function
				Detail:     "User-defined function",
				InsertText: function + "()",
			})
		}
	}

	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil
}

// extractVariables finds all variable names in the AST
func extractVariables(program *ast.Program) []string {
	var variables []string
	variableSet := make(map[string]bool)

	for _, stmt := range program.Stmts {
		switch s := stmt.(type) {
		case *ast.Var:
			name := s.Name.Name
			if name != "" && !variableSet[name] {
				variables = append(variables, name)
				variableSet[name] = true
			}
		case *ast.Assign:
			name := s.Name.Name
			if name != "" && !variableSet[name] {
				variables = append(variables, name)
				variableSet[name] = true
			}
		}
	}

	return variables
}

// extractFunctions finds all function names in the AST
func extractFunctions(program *ast.Program) []string {
	var functions []string
	functionSet := make(map[string]bool)

	for _, stmt := range program.Stmts {
		switch s := stmt.(type) {
		case *ast.Assign:
			// Check if we're assigning a function to a variable
			if _, ok := s.Value.(*ast.Func); ok {
				name := s.Name.Name
				if name != "" && !functionSet[name] {
					functions = append(functions, name)
					functionSet[name] = true
				}
			}
		case *ast.Var:
			// Check if we're declaring a variable with a function value
			name := s.Name.Name
			if _, ok := s.Value.(*ast.Func); ok && name != "" {
				if !functionSet[name] {
					functions = append(functions, name)
					functionSet[name] = true
				}
			}
		}
	}

	return functions
}
