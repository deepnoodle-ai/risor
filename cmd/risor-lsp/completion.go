package main

import (
	"context"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/risor-io/risor/ast"
	"github.com/rs/zerolog/log"
)

// Risor keywords for completion
var risorKeywords = []string{
	"as", "break", "case", "const", "continue", "default", "defer", "else",
	"false", "for", "from", "function", "if", "import", "in", "let", "nil", "not",
	"range", "return", "struct", "switch", "true",
}

// Common built-in functions
var risorBuiltins = []string{
	"all", "any", "assert", "bool", "buffer", "byte_slice", "byte", "call",
	"chr", "chunk", "coalesce", "decode", "delete", "encode",
	"error", "float_slice", "float", "getattr", "hash", "int", "is_hashable",
	"iter", "jmespath", "keys", "len", "list", "make", "map", "ord", "print", "println",
	"reversed", "set", "sorted", "sprintf", "string", "try", "type",
}

// Common modules
var risorModules = []string{
	"aws", "base64", "bcrypt", "bytes", "cli", "color", "dns", "echarts",
	"errors", "exec", "filepath", "fmt", "gha", "github", "goquery",
	"htmltomarkdown", "http", "image", "isatty", "jmespath", "json",
	"kubernetes", "math", "net", "os", "pgx", "playwright", "qrcode",
	"rand", "redis", "regexp", "sched", "semver", "shlex", "slack",
	"sql", "ssh", "strconv", "strings", "tablewriter", "template",
	"time", "uuid", "vault", "yaml",
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

	for _, stmt := range program.Statements() {
		switch s := stmt.(type) {
		case *ast.Var:
			name, _ := s.Value()
			if name != "" && !variableSet[name] {
				variables = append(variables, name)
				variableSet[name] = true
			}
		case *ast.Assign:
			name := s.Name()
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

	for _, stmt := range program.Statements() {
		switch s := stmt.(type) {
		case *ast.Assign:
			// Check if we're assigning a function to a variable
			if _, ok := s.Value().(*ast.Func); ok {
				name := s.Name()
				if name != "" && !functionSet[name] {
					functions = append(functions, name)
					functionSet[name] = true
				}
			}
		case *ast.Var:
			// Check if we're declaring a variable with a function value
			name, value := s.Value()
			if _, ok := value.(*ast.Func); ok && name != "" {
				if !functionSet[name] {
					functions = append(functions, name)
					functionSet[name] = true
				}
			}
		}
	}

	return functions
}
