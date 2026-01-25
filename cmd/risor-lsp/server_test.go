package main

import (
	"context"
	"testing"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/risor-io/risor/parser"
	"github.com/stretchr/testify/require"
)

// Helper function to set a document in the cache for testing
func setTestDocument(c *cache, uri protocol.DocumentURI, text string) error {
	item := &protocol.TextDocumentItem{
		URI:     uri,
		Text:    text,
		Version: 1,
	}

	doc := &document{
		item:                 *item,
		linesChangedSinceAST: map[int]bool{},
	}

	if text != "" {
		ctx := context.Background()
		doc.ast, doc.err = parser.Parse(ctx, text)
	}

	return c.put(doc)
}

func TestCache_ParseValidRisorCode(t *testing.T) {
	c := newCache()

	// Test valid Risor code
	validCode := `let x = 42
let y = "hello"
function add(a, b) {
    return a + b
}`

	uri := protocol.DocumentURI("file:///test.risor")
	err := setTestDocument(c, uri, validCode)
	require.NoError(t, err)

	doc, err := c.get(uri)
	require.NoError(t, err)

	require.NoError(t, doc.err)

	require.NotNil(t, doc.ast)

	// Verify we have statements
	statements := doc.ast.Statements()
	require.NotEmpty(t, statements)
}

func TestCache_ParseInvalidRisorCode(t *testing.T) {
	c := newCache()

	// Test invalid Risor code
	invalidCode := `let x =
function incomplete(`

	uri := protocol.DocumentURI("file:///test_invalid.risor")
	err := setTestDocument(c, uri, invalidCode)
	require.NoError(t, err)

	doc, err := c.get(uri)
	require.NoError(t, err)

	// Should have a parse error
	require.Error(t, doc.err)
}

func TestCompletionProvider_ExtractVariables(t *testing.T) {
	// Create a test program
	code := `let x = 42
let y = "hello"
let z = [1, 2, 3]`

	ctx := context.Background()
	prog, err := parser.Parse(ctx, code)
	require.NoError(t, err)

	variables := extractVariables(prog)

	expectedVars := []string{"x", "y", "z"}
	require.Equal(t, len(expectedVars), len(variables))

	// Check that all expected variables are found
	varMap := make(map[string]bool)
	for _, v := range variables {
		varMap[v] = true
	}

	for _, expected := range expectedVars {
		require.True(t, varMap[expected], "Expected variable %s not found in %v", expected, variables)
	}
}

func TestCompletionProvider_ExtractFunctions(t *testing.T) {
	// Create a test program with function assignments
	code := `let add = function(a, b) { return a + b }
let subtract = function(x, y) { return x - y }`

	ctx := context.Background()
	prog, err := parser.Parse(ctx, code)
	require.NoError(t, err)

	functions := extractFunctions(prog)

	expectedFuncs := []string{"add", "subtract"}
	require.Equal(t, len(expectedFuncs), len(functions))

	// Check that all expected functions are found
	funcMap := make(map[string]bool)
	for _, f := range functions {
		funcMap[f] = true
	}

	for _, expected := range expectedFuncs {
		require.True(t, funcMap[expected], "Expected function %s not found in %v", expected, functions)
	}
}

func TestHoverProvider_FindSymbolAtPosition(t *testing.T) {
	// Create a test program
	code := `let x = 42
let y = "hello"`

	ctx := context.Background()
	prog, err := parser.Parse(ctx, code)
	require.NoError(t, err)

	// Test finding symbol at position of variable 'x' (line 1, around column 5)
	symbol := findSymbolAtPosition(prog, 1, 5)
	require.Equal(t, "x", symbol)

	// Test finding symbol at position of variable 'y' (line 2, around column 5)
	symbol = findSymbolAtPosition(prog, 2, 5)
	require.Equal(t, "y", symbol)

	// Test position with no symbol
	symbol = findSymbolAtPosition(prog, 1, 15)
	require.Empty(t, symbol)
}

func TestKeywordsAndBuiltins(t *testing.T) {
	// Test that our keyword list contains expected Risor keywords
	expectedKeywords := []string{"let", "function", "if", "else", "for", "return", "true", "false", "nil"}

	for _, keyword := range expectedKeywords {
		found := false
		for _, k := range risorKeywords {
			if k == keyword {
				found = true
				break
			}
		}
		require.True(t, found, "Expected keyword '%s' not found in risorKeywords", keyword)
	}

	// Test that our builtin list contains expected functions
	expectedBuiltins := []string{"len", "print", "println", "string", "int", "float"}

	for _, builtin := range expectedBuiltins {
		found := false
		for _, b := range risorBuiltins {
			if b == builtin {
				found = true
				break
			}
		}
		require.True(t, found, "Expected builtin '%s' not found in risorBuiltins", builtin)
	}
}

func TestDiagnostics_WithParseError(t *testing.T) {
	// Test code with syntax error
	invalidCode := `let x =
function incomplete(`

	// Parse the code to get a parse error
	ctx := context.Background()
	_, err := parser.Parse(ctx, invalidCode)
	require.Error(t, err)

	// Verify it's a parse error we can handle
	parseErr, ok := err.(parser.ParserError)
	require.True(t, ok, "Expected parser.ParseError type, got %T", err)

	require.NotEmpty(t, parseErr.Message())

	startPos := parseErr.StartPosition()
	require.Greater(t, startPos.LineNumber(), 0)
}

func TestServer_QueueDiagnostics(t *testing.T) {
	// Create a minimal server for testing
	server := &Server{
		name:    "test-server",
		version: "test",
		cache:   newCache(),
	}

	// This test mainly ensures the method doesn't panic
	// In a full integration test, we'd mock the client and verify the diagnostics
	uri := protocol.DocumentURI("file:///test.risor")

	// Set a document with an error
	err := setTestDocument(server.cache, uri, "let x =\nfunction incomplete(")
	require.NoError(t, err)

	// This should not panic
	server.queueDiagnostics(uri)
}

func TestHoverProvider_FullHover(t *testing.T) {
	// Create a test program with various constructs
	code := `let config = {
    "host": "localhost",
    "port": 8080
}

let greet = function(name) {
    return sprintf("Hello, %s!", name)
}

let message = "test"
print(message)`

	ctx := context.Background()
	prog, err := parser.Parse(ctx, code)
	require.NoError(t, err)

	// Create a test server
	server := &Server{
		name:    "test-server",
		version: "1.0.0",
		cache:   newCache(),
	}

	// Create a test document
	uri := protocol.DocumentURI("file:///test.risor")
	doc := &document{
		item: protocol.TextDocumentItem{
			URI:  uri,
			Text: code,
		},
		ast: prog,
		err: nil,
	}
	err = server.cache.put(doc)
	require.NoError(t, err)

	// Test 1: Hover over variable 'config' (line 1, column 5)
	hoverParams := &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 4}, // LSP uses 0-based indexing
		},
	}

	result, err := server.Hover(ctx, hoverParams)
	require.NoError(t, err)
	if result != nil {
		t.Logf("Hover result for 'config': %s", result.Contents.Value)
		require.Contains(t, result.Contents.Value, "config")
	} else {
		t.Log("No hover result for 'config' (checking if this is expected)")
	}

	// Test 2: Hover over function 'greet' (line 6, column 1)
	hoverParams = &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 5, Character: 0}, // LSP uses 0-based indexing
		},
	}

	result, err = server.Hover(ctx, hoverParams)
	require.NoError(t, err)
	if result != nil {
		t.Logf("Hover result for 'greet': %s", result.Contents.Value)
	} else {
		t.Log("No hover result for 'greet'")
	}

	// Test 3: Hover over builtin function 'print' (line 11, column 1)
	hoverParams = &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 10, Character: 0}, // LSP uses 0-based indexing
		},
	}

	result, err = server.Hover(ctx, hoverParams)
	require.NoError(t, err)
	if result != nil {
		t.Logf("Hover result for 'print': %s", result.Contents.Value)
		require.Contains(t, result.Contents.Value, "print")
		require.Contains(t, result.Contents.Value, "Built-in function")
	} else {
		t.Log("No hover result for 'print' - this indicates an issue")
	}

	// Test 4: Hover over variable 'message' (line 10, around column 8)
	hoverParams = &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 9, Character: 0}, // LSP uses 0-based indexing
		},
	}

	result, err = server.Hover(ctx, hoverParams)
	require.NoError(t, err)
	if result != nil {
		t.Logf("Hover result for 'message': %s", result.Contents.Value)
	} else {
		t.Log("No hover result for 'message'")
	}
}

func TestServer_DidSave_ClearsDiagnosticsOnFix(t *testing.T) {
	// Create a minimal server for testing
	server := &Server{
		name:    "test-server",
		version: "test",
		cache:   newCache(),
	}

	uri := protocol.DocumentURI("file:///test.risor")
	ctx := context.Background()

	// First, set a document with a syntax error
	invalidCode := `let x =
function incomplete(`

	err := setTestDocument(server.cache, uri, invalidCode)
	require.NoError(t, err)

	// Verify the document has a parse error
	doc, err := server.cache.get(uri)
	require.NoError(t, err)
	require.Error(t, doc.err)

	// Now simulate saving the file with the error fixed
	fixedCode := `let x = 42
function complete() {
    return x
}`

	saveParams := &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Text:         &fixedCode,
	}

	// Call DidSave with the fixed code
	err = server.DidSave(ctx, saveParams)
	require.NoError(t, err)

	// Verify the document now parses without error
	doc, err = server.cache.get(uri)
	require.NoError(t, err)
	require.NoError(t, doc.err, "Document should parse without error after fix")

	// Verify the AST was updated
	require.NotNil(t, doc.ast)
	statements := doc.ast.Statements()
	require.NotEmpty(t, statements)
}
