package main

import (
	"errors"
	"fmt"
	"sync"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
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

// put adds or replaces a document in the cache. Cached documents are shared
// across goroutines, so a document must not be changed once it is put.
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

// replace replaces a document that is already in the cache, with the same
// version rule as put. It returns an error if the document is not in the
// cache, for example because it was closed, so that a change that arrives
// after a close does not put the closed document back.
func (c *cache) replace(new *document) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	uri := new.item.URI
	old, ok := c.docs[uri]
	if !ok {
		return fmt.Errorf("document %s not found in cache", uri)
	}
	if old.item.Version > new.item.Version {
		return errors.New("newer version of the document is already in the cache")
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

// remove deletes a document from the cache.
func (c *cache) remove(uri protocol.DocumentURI) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.docs, uri)
}
