package main

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

// TestLanguageServerIntegration demonstrates testing the language server
// with a complete Risor file, simulating real VS Code interactions
func TestLanguageServerIntegration(t *testing.T) {
	// Sample Risor code that demonstrates various language features
	risorCode := `// Example Risor program
let config = {
    "host": "localhost",
    "port": 8080,
    "debug": true
}

// Function to process user data
let process_user = function(user_id, name) {
    if (user_id <= 0) {
        return "Invalid user ID"
    }

    let user_data = {
        "id": user_id,
        "name": name,
        "status": "active"
    }

    return user_data
}

// Main processing logic using functional style
let users = [0, 1, 2, 3, 4].map(i => process_user(i, "User_" + string(i)))

// Filter active users
let active_users = users.filter(u => u["status"] == "active")

// Print count
println("Total users: " + string(len(active_users)))`

	// Create a server instance
	server := &Server{
		name:    "test-risor-lsp",
		version: "1.0.0-test",
		cache:   newCache(),
	}

	uri := protocol.DocumentURI("file:///example.risor")

	// Test 1: Document parsing and caching
	t.Run("DocumentParsing", func(t *testing.T) {
		err := setTestDocument(server.cache, uri, risorCode)
		assert.NoError(t, err, "Failed to cache document")

		doc, err := server.cache.get(uri)
		assert.NoError(t, err, "Failed to retrieve document")

		assert.NoError(t, doc.err, "Document parsing failed")

		assert.NotNil(t, doc.ast, "Expected AST to be parsed")

		statements := doc.ast.Stmts
		assert.NotEmpty(t, statements, "Expected statements in AST")

		t.Logf("Successfully parsed %d statements", len(statements))
	})

	// Test 2: Completion at various positions
	t.Run("Completion", func(t *testing.T) {
		// Test completion at line 26 (in the active_users line where users is in scope)
		params := &protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Position:     protocol.Position{Line: 25, Character: 20}, // In "let active_users = users..."
			},
		}

		result, err := server.Completion(context.Background(), params)
		assert.NoError(t, err, "Completion failed")

		assert.NotNil(t, result, "Expected completion result")
		assert.NotEmpty(t, result.Items, "Expected completion items")

		// Should include variables like "users", keywords, and builtins
		hasUsers := false
		hasKeywords := false
		hasBuiltins := false

		for _, item := range result.Items {
			switch item.Label {
			case "users":
				hasUsers = true
			case "let", "if", "const":
				hasKeywords = true
			case "len", "print", "println":
				hasBuiltins = true
			}
		}

		assert.True(t, hasUsers, "Expected 'users' variable in completion")
		assert.True(t, hasKeywords, "Expected keywords in completion")
		assert.True(t, hasBuiltins, "Expected builtin functions in completion")

		t.Logf("Completion returned %d items", len(result.Items))
	})

	// Test 3: Hover information
	t.Run("Hover", func(t *testing.T) {
		// Test hover over the "process_user" function name
		params := &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Position:     protocol.Position{Line: 7, Character: 0}, // At "process_user"
			},
		}

		result, err := server.Hover(context.Background(), params)
		assert.NoError(t, err, "Hover failed")

		// Note: hover might not find anything with our simple position-based implementation
		// This is expected for this test
		if result != nil && result.Contents.Value != "" {
			t.Logf("Hover content: %s", result.Contents.Value)
		} else {
			t.Logf("No hover content found (expected with simple implementation)")
		}
	})

	// Test 4: Document symbols
	t.Run("DocumentSymbols", func(t *testing.T) {
		params := &protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		}

		result, err := server.DocumentSymbol(context.Background(), params)
		assert.NoError(t, err, "DocumentSymbol failed")

		assert.NotNil(t, result, "Expected document symbols result")
		assert.NotEmpty(t, result, "Expected document symbols")

		// Should find variables like "config", "process_user", "users"
		symbolNames := []string{}
		for _, symbolInterface := range result {
			if symbol, ok := symbolInterface.(protocol.DocumentSymbol); ok {
				symbolNames = append(symbolNames, symbol.Name)
			}
		}

		expectedSymbols := []string{"config", "process_user", "users"}
		for _, expected := range expectedSymbols {
			found := false
			for _, name := range symbolNames {
				if name == expected {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected symbol '%s' not found in %v", expected, symbolNames)
		}

		t.Logf("Found symbols: %v", symbolNames)
	})

	// Test 5: Go-to-definition
	t.Run("Definition", func(t *testing.T) {
		// Test definition for "process_user" usage
		params := &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Position:     protocol.Position{Line: 20, Character: 12}, // At "process_user" call
			},
		}

		result, err := server.Definition(context.Background(), params)
		assert.NoError(t, err, "Definition failed")

		// This might not find anything with our simple implementation,
		// but shouldn't error
		if result != nil {
			t.Logf("Definition result type: %T", result)
		}
	})
}

// TestLanguageServerWithErrors tests how the language server handles syntax errors
func TestLanguageServerWithErrors(t *testing.T) {
	server := &Server{
		name:    "test-risor-lsp",
		version: "1.0.0-test",
		cache:   newCache(),
	}

	// Code with syntax errors
	invalidCode := `let x = 42
function incomplete(
let y = "missing closing brace"
if (true) {
    // missing closing brace`

	uri := protocol.DocumentURI("file:///invalid.risor")

	err := setTestDocument(server.cache, uri, invalidCode)
	assert.NoError(t, err, "Failed to cache document")

	doc, err := server.cache.get(uri)
	assert.NoError(t, err, "Failed to retrieve document")

	// Should have a parse error
	assert.Error(t, doc.err, "Expected parse error for invalid code")

	t.Logf("Parse error (as expected): %v", doc.err)

	// Test that we can still provide basic completion even with errors
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 10},
		},
	}

	result, err := server.Completion(context.Background(), params)
	assert.NoError(t, err, "Completion failed")

	// Should still provide keywords and builtins even with syntax errors
	assert.NotNil(t, result, "Expected completion result")
	assert.NotEmpty(t, result.Items, "Expected completion items even with syntax errors")

	t.Logf("Completion with errors returned %d items", len(result.Items))
}

// TestRisorCodeExamples tests the language server with various Risor code patterns
func TestRisorCodeExamples(t *testing.T) {
	examples := map[string]string{
		"variables": `let name = "Risor"
let age = 25
let is_valid = true`,

		"functions": `let add = function(a, b) { return a + b }
let greet = function(name) {
    return "Hello, " + name + "!"
}`,

		"control_flow": `let age = 18
if (age >= 18) {
    let status = "adult"
} else {
    let status = "minor"
}

let items = [1, 2, 3, 4, 5].map(i => i * 2)`,

		"data_structures": `let person = {
    "name": "Alice",
    "age": 30,
    "hobbies": ["reading", "coding"]
}

let numbers = [1, 2, 3, 4, 5]`,
	}

	server := &Server{
		name:    "test-risor-lsp",
		version: "1.0.0-test",
		cache:   newCache(),
	}

	for name, code := range examples {
		t.Run(name, func(t *testing.T) {
			uri := protocol.DocumentURI("file:///" + name + ".risor")

			err := setTestDocument(server.cache, uri, code)
			assert.NoError(t, err, "Failed to cache document")

			doc, err := server.cache.get(uri)
			assert.NoError(t, err, "Failed to retrieve document")

			assert.NoError(t, doc.err, "Parse error in %s", name)

			assert.NotNil(t, doc.ast, "No AST parsed for %s", name)

			statements := doc.ast.Stmts
			assert.NotEmpty(t, statements, "No statements found in %s", name)

			t.Logf("Example '%s': parsed %d statements successfully", name, len(statements))

			// Test that completion works for each example
			params := &protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: uri},
					Position:     protocol.Position{Line: 0, Character: 0},
				},
			}

			result, err := server.Completion(context.Background(), params)
			assert.NoError(t, err, "Completion failed for %s", name)

			assert.NotNil(t, result, "No completion result for %s", name)
			assert.NotEmpty(t, result.Items, "No completion items for %s", name)

			t.Logf("Example '%s': completion returned %d items", name, len(result.Items))
		})
	}
}
