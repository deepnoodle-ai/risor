package main

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func newTestServer() *Server {
	return &Server{name: "test-server", version: "test", cache: newCache()}
}

func openTestDocument(t *testing.T, s *Server, uri protocol.DocumentURI, text string) {
	t.Helper()
	err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri, Text: text, Version: 1},
	})
	assert.NoError(t, err)
}

func position(uri protocol.DocumentURI, line, char uint32) protocol.TextDocumentPositionParams {
	return protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Position:     protocol.Position{Line: line, Character: char},
	}
}

// An index assignment has no name. No handler may crash on one.
func TestIndexAssignmentDoesNotCrash(t *testing.T) {
	ctx := context.Background()
	s := newTestServer()
	uri := protocol.DocumentURI("file:///index.risor")
	openTestDocument(t, s, uri, "let arr = [1, 2]\narr[0] = 1\nlet f = function() {}\narr")

	_, err := s.Completion(ctx, &protocol.CompletionParams{TextDocumentPositionParams: position(uri, 3, 3)})
	assert.NoError(t, err)

	_, err = s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: position(uri, 1, 0)})
	assert.NoError(t, err)
	_, err = s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: position(uri, 3, 1)})
	assert.NoError(t, err)

	_, err = s.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: position(uri, 3, 1)})
	assert.NoError(t, err)

	symbols, err := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	assert.NoError(t, err)
	assert.Len(t, symbols, 2) // arr and f; the index assignment is not a symbol
}

func TestDefinitionHasURI(t *testing.T) {
	s := newTestServer()
	uri := protocol.DocumentURI("file:///def.risor")
	openTestDocument(t, s, uri, "x = 1\nx")

	locations, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: position(uri, 1, 0),
	})
	assert.NoError(t, err)
	assert.Len(t, locations, 1)
	assert.Equal(t, locations[0].URI, uri)
}

func TestDidCloseRemovesDocument(t *testing.T) {
	s := newTestServer()
	uri := protocol.DocumentURI("file:///close.risor")
	openTestDocument(t, s, uri, "let x = 1")

	err := s.DidClose(context.Background(), &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	assert.NoError(t, err)
	_, err = s.cache.get(uri)
	assert.Error(t, err)
}

// Changes and saves replace the cached document rather than changing it, so
// the diagnostics goroutines and other requests can read it at the same time.
// Run with -race.
func TestConcurrentChangesAndReads(t *testing.T) {
	ctx := context.Background()
	s := newTestServer()
	uri := protocol.DocumentURI("file:///race.risor")
	openTestDocument(t, s, uri, "let x = 1")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			text := fmt.Sprintf("let x = %d\nx", i)
			if i%2 == 0 {
				text = "let x ="
			}
			err := s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
					Version:                int32(i + 2),
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: text}},
			})
			assert.NoError(t, err)
			err = s.DidSave(ctx, &protocol.DidSaveTextDocumentParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Text:         &text,
			})
			assert.NoError(t, err)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: position(uri, 1, 0)})
			_, _ = s.Completion(ctx, &protocol.CompletionParams{TextDocumentPositionParams: position(uri, 1, 0)})
			s.publishDiagnostics(uri)
		}
	}()
	wg.Wait()

	doc, err := s.cache.get(uri)
	assert.NoError(t, err)
	assert.Equal(t, doc.item.Version, int32(51))
	assert.Equal(t, doc.item.Text, "let x = 49\nx")
}
