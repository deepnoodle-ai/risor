# Risor Language Server Design

**Date:** July 2025  
**Status:** Implemented  
**Authors:** AI Assistant

## Overview

This document describes the design and implementation approach for the Risor Language Server Protocol (LSP) server. The language server provides intelligent code editing features for Risor scripts in VS Code and other LSP-compatible editors.

## Architecture

### Core Components

The Risor language server follows a modular architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                    VS Code Extension                        │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────── │
│  │ Language Config │  │ Syntax Grammar  │  │ LSP Client    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────── │
└─────────────────────────────┬───────────────────────────────┘
                              │ JSON-RPC over stdio
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    Risor LSP Server                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────── │
│  │     Server      │  │     Cache       │  │   Providers   │ │
│  │   (main.go)     │  │   (cache.go)    │  │               │ │
│  └─────────────────┘  └─────────────────┘  │ • Completion  │ │
│  ┌─────────────────┐  ┌─────────────────┐  │ • Hover       │ │
│  │  Diagnostics    │  │   Document      │  │ • Definition  │ │
│  │    Engine       │  │   Tracking      │  │ • Symbols     │ │
│  └─────────────────┘  └─────────────────┘  │ • Formatting  │ │
└─────────────────────────────┬───────────────┴─────────────── │
                              │                               │
┌─────────────────────────────▼───────────────────────────────┐
│                   Risor Parser & AST                        │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────── │
│  │     Parser      │  │   AST Nodes     │  │  Token Types  │ │
│  │  (parser pkg)   │  │   (ast pkg)     │  │ (token pkg)   │ │
│  └─────────────────┘  └─────────────────┘  └─────────────── │
└─────────────────────────────────────────────────────────────┘
```

### Key Files and Responsibilities

#### Server Core
- **`main.go`** - Entry point, sets up stdio communication
- **`server.go`** - Main server implementation, LSP message handling
- **`cache.go`** - Document caching and AST management

#### Language Features
- **`completion.go`** - Code completion provider
- **`hover.go`** - Hover information provider  
- **`definition.go`** - Go-to-definition provider
- **`symbols.go`** - Document symbols provider
- **`formatting.go`** - Code formatting provider
- **`configuration.go`** - Configuration management

#### VS Code Extension
- **`vscode/client/src/extension.ts`** - Extension activation and LSP client
- **`vscode/package.json`** - Extension manifest and configuration
- **`vscode/syntaxes/risor.grammar.json`** - Syntax highlighting
- **`vscode/language-configuration.json`** - Language configuration

## Design Principles

### 1. Robustness
The language server is designed to handle malformed code gracefully:
- Parse errors don't crash the server
- Partial completion and hover work even with syntax errors
- Cached ASTs are preserved when possible
- Graceful degradation for unsupported features

### 2. Performance
Performance considerations throughout the implementation:
- **Incremental parsing** - Only re-parse when documents change
- **Cached ASTs** - Avoid re-parsing unchanged documents
- **Background diagnostics** - Parse errors reported asynchronously
- **Minimal allocations** - Reuse data structures where possible

### 3. Extensibility
Modular design allows for easy feature additions:
- Provider pattern for language features
- Pluggable completion sources
- Configurable diagnostics
- Extension point for custom modules

### 4. Standards Compliance
Full adherence to LSP specification:
- Standard LSP message format
- Proper capability negotiation
- Correct position encoding (UTF-16 compatible)
- Standard diagnostic severity levels

## Implementation Details

### Document Management

The `cache` component manages document lifecycle:

```go
type document struct {
    item                 protocol.TextDocumentItem
    ast                  *ast.Program          // Parsed AST
    linesChangedSinceAST map[int]bool         // Incremental tracking
    val                  string               // Document content
    err                  error                // Parse errors
    diagnostics          []protocol.Diagnostic // Cached diagnostics
}
```

**Key Features:**
- Version-based caching prevents race conditions
- Parse errors are preserved for diagnostics
- Incremental change tracking for future optimization

### Diagnostics Engine

Real-time error reporting using Risor's parser:

```go
func (s *Server) publishDiagnostics(uri protocol.DocumentURI) {
    doc, err := s.cache.get(uri)
    if err != nil {
        return
    }

    var diagnostics []protocol.Diagnostic
    if doc.err != nil {
        if parseErr, ok := doc.err.(parser.ParserError); ok {
            // Convert parser error to LSP diagnostic
            diagnostic := protocol.Diagnostic{
                Range:    convertPosition(parseErr.StartPosition(), parseErr.EndPosition()),
                Severity: 1, // Error
                Source:   "risor-lsp",
                Message:  parseErr.Message(),
            }
            diagnostics = append(diagnostics, diagnostic)
        }
    }

    s.client.PublishDiagnostics(context.Background(), &protocol.PublishDiagnosticsParams{
        URI:         uri,
        Diagnostics: diagnostics,
    })
}
```

### Completion Provider

Multi-source completion system:

1. **Keywords** - Risor language keywords (`var`, `func`, `if`, etc.)
2. **Builtins** - Built-in functions (`len`, `print`, `sprintf`, etc.)
3. **Modules** - Available modules (`os`, `strings`, `http`, etc.)
4. **Variables** - Document-local variables from AST analysis
5. **Functions** - User-defined functions from assignments

**AST Analysis:**
```go
func extractVariables(program *ast.Program) []string {
    var variables []string
    for _, stmt := range program.Statements() {
        switch s := stmt.(type) {
        case *ast.Var:
            name, _ := s.Value()
            variables = append(variables, name)
        case *ast.Assign:
            variables = append(variables, s.Name())
        }
    }
    return variables
}
```

### Position Handling

LSP uses 0-based line/column positions while Risor parser uses 1-based:

```go
func convertPosition(start, end token.Position) protocol.Range {
    return protocol.Range{
        Start: protocol.Position{
            Line:      uint32(start.LineNumber() - 1),
            Character: uint32(start.ColumnNumber() - 1),
        },
        End: protocol.Position{
            Line:      uint32(end.LineNumber() - 1),
            Character: uint32(end.ColumnNumber() - 1),
        },
    }
}
```

## VS Code Integration

### Extension Structure

The VS Code extension provides:
- **Language Registration** - Associates `.risor` files with the language
- **Syntax Highlighting** - TextMate grammar for syntax coloring
- **LSP Client** - Connects to the language server process
- **Configuration** - Settings for server behavior

### Activation Events

```json
{
  "activationEvents": ["onLanguage:risor"],
  "contributes": {
    "languages": [{
      "id": "risor",
      "aliases": ["Risor", "risor", "rsr"],
      "extensions": [".risor", ".rsr"],
      "configuration": "./language-configuration.json"
    }]
  }
}
```

### Language Configuration

Provides editor behavior for Risor:
- Comment syntax (`//` and `/* */`)
- Bracket matching (`{}`, `[]`, `()`)
- Auto-closing pairs
- Indentation rules

## Testing Strategy

### Unit Tests

**Component Testing:**
- `server_test.go` - Core server functionality
- Cache operations (get/put)
- AST parsing and error handling
- Provider functions (completion, hover, symbols)

**Test Data:**
- Valid Risor code samples
- Invalid code for error testing
- Edge cases and boundary conditions

### Integration Tests

**End-to-End Testing:**
- `integration_test.go` - Full language server workflow
- Document lifecycle simulation
- LSP feature testing with real Risor code
- Error handling and recovery

**Test Files:**
- `testdata/example.risor` - Comprehensive language feature demo
- `testdata/invalid.risor` - Syntax error scenarios

### Manual Testing

**VS Code Extension:**
1. Install extension via VSIX package
2. Open `.risor` files
3. Test completion (Ctrl+Space)
4. Test hover information
5. Test go-to-definition (F12)
6. Verify syntax error highlighting

## Configuration

### Server Settings

```json
{
  "risor.maxNumberOfProblems": 100,
  "risor.trace.server": "off|messages|verbose",
  "risor.enableEvalDiagnostics": false,
  "risor.enableLintDiagnostics": true
}
```

### Capability Negotiation

The server announces its capabilities during initialization:

```go
return &protocol.InitializeResult{
    Capabilities: protocol.ServerCapabilities{
        CompletionProvider: protocol.CompletionOptions{
            TriggerCharacters: []string{"."},
        },
        HoverProvider:              true,
        DefinitionProvider:         true,
        DocumentSymbolProvider:     true,
        DocumentFormattingProvider: true,
        TextDocumentSync: protocol.TextDocumentSyncOptions{
            OpenClose: true,
            Change:    protocol.TextDocumentSyncKindFull,
            Save:      protocol.SaveOptions{IncludeText: true},
        },
    },
}
```

## Future Enhancements

### Planned Features

1. **Semantic Analysis**
   - Type inference for better completion
   - Variable scope analysis
   - Dead code detection

2. **Advanced Completion**
   - Context-aware suggestions
   - Snippet expansion
   - Import statement completion

3. **Refactoring**
   - Rename symbol
   - Extract function
   - Inline variable

4. **Debugging Support**
   - Debug adapter protocol
   - Breakpoint support
   - Variable inspection

### Performance Improvements

1. **Incremental Parsing**
   - Parse only changed sections
   - Smart AST invalidation
   - Syntax tree preservation

2. **Caching Optimization**
   - Cross-document symbol cache
   - Module resolution cache
   - Completion result caching

3. **Concurrency**
   - Background parsing
   - Parallel diagnostic checking
   - Async completion generation

## Deployment

### Binary Distribution

```bash
# Build the language server
cd cmd/risor-lsp
go build -o risor-lsp .

# Install VS Code extension
cd ../../vscode
npm install
npx vsce package
code --install-extension risor-*.vsix
```

### Development Setup

```bash
# Run tests
go test -v ./cmd/risor-lsp/...

# Test with sample files
./cmd/risor-lsp/risor-lsp < test-input.json

# Debug mode
./cmd/risor-lsp/risor-lsp --log-level=debug
```

## Conclusion

The Risor language server provides a solid foundation for intelligent code editing with comprehensive LSP feature support. The modular architecture enables easy extension and maintenance, while the robust error handling ensures a smooth user experience even with malformed code.

The implementation demonstrates best practices for language server development:
- Clean separation of concerns
- Comprehensive error handling
- Performance-conscious design
- Standards compliance
- Thorough testing

This design serves as both documentation and a blueprint for future language server implementations in the Risor ecosystem.