package main

import (
	"errors"
	"fmt"
	"sync"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/deepnoodle-ai/risor/v2/ast"
)

type document struct {
	// From DidOpen and DidChange
	item protocol.TextDocumentItem

	// Contains the last successfully parsed AST. If doc.err is not nil, it's out of date.
	ast                  *ast.Program
	linesChangedSinceAST map[int]bool

	// From diagnostics
	val         string
	err         error
	diagnostics []protocol.Diagnostic
}

// newCache returns a document cache.
func newCache() *cache {
	return &cache{
		mu:        sync.RWMutex{},
		docs:      make(map[protocol.DocumentURI]*document),
		diagQueue: make(map[protocol.DocumentURI]struct{}),
	}
}

// cache caches documents.
type cache struct {
	mu          sync.RWMutex
	docs        map[protocol.DocumentURI]*document
	diagMutex   sync.RWMutex
	diagQueue   map[protocol.DocumentURI]struct{}
	diagRunning sync.Map
}

// put adds or replaces a document in the cache.
// Documents are only replaced if the new document version is greater than the currently
// cached version.
func (c *cache) put(new *document) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	uri := new.item.URI
	if old, ok := c.docs[uri]; ok {
		if old.item.Version > new.item.Version {
			return errors.New("newer version of the document is already in the cache")
		}
	}
	c.docs[uri] = new

	return nil
}

// get retrieves a document from the cache.
func (c *cache) get(uri protocol.DocumentURI) (*document, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	doc, ok := c.docs[uri]
	if !ok {
		return nil, fmt.Errorf("document %s not found in cache", uri)
	}

	return doc, nil
}
