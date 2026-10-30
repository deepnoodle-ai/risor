package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/rs/zerolog/log"
)

type Server struct {
	name    string
	version string
	client  protocol.ClientCloser
	cache   *cache

	// publishMu orders diagnostic publications. A publisher checks that its
	// document is still the cached one and sends while holding it, so that
	// diagnostics for a replaced or closed document are never sent after
	// the diagnostics that superseded them.
	publishMu sync.Mutex
}

func (s *Server) queueDiagnostics(uri protocol.DocumentURI) {
	log.Info().Str("uri", string(uri)).Msg("=== queueDiagnostics: spawning publishDiagnostics goroutine ===")
	go s.publishDiagnostics(uri)
}

// forceClearDiagnostics explicitly clears all diagnostics for a URI
func (s *Server) forceClearDiagnostics(uri protocol.DocumentURI) {
	log.Info().Str("uri", string(uri)).Msg("=== forceClearDiagnostics: EXPLICITLY CLEARING ALL DIAGNOSTICS ===")
	s.publishMu.Lock()
	defer s.publishMu.Unlock()

	if s.client != nil {
		diagnosticsParams := &protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: []protocol.Diagnostic{},
		}

		err := s.client.PublishDiagnostics(context.Background(), diagnosticsParams)
		if err != nil {
			log.Error().Err(err).Str("uri", string(uri)).Msg("!!! forceClearDiagnostics: FAILED !!!")
		} else {
			log.Info().Str("uri", string(uri)).Msg("=== forceClearDiagnostics: SUCCESS ===")
		}
	} else {
		log.Error().Str("uri", string(uri)).Msg("!!! forceClearDiagnostics: CLIENT IS NIL !!!")
	}
}

func (s *Server) publishDiagnostics(uri protocol.DocumentURI) {
	log.Info().Str("uri", string(uri)).Msg("=== publishDiagnostics START ===")

	doc, err := s.cache.get(uri)
	if err != nil {
		log.Error().Err(err).Str("uri", string(uri)).Msg("failed to get document for diagnostics")
		return
	}
	s.publishDocumentDiagnostics(uri, doc)
}

// publishDocumentDiagnostics publishes the diagnostics for doc, unless doc is
// no longer the cached document for uri.
func (s *Server) publishDocumentDiagnostics(uri protocol.DocumentURI, doc *document) {
	var diagnostics []protocol.Diagnostic

	// Check for parse errors
	if doc.err != nil {
		log.Info().Err(doc.err).Msg("publishDiagnostics: Found parse error")
		if parseErr, ok := doc.err.(parser.ParserError); ok {
			startPos := parseErr.StartPosition()
			endPos := parseErr.EndPosition()

			diagnostic := protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      uint32(startPos.LineNumber() - 1),   // LSP uses 0-based line numbers
						Character: uint32(startPos.ColumnNumber() - 1), // LSP uses 0-based column numbers
					},
					End: protocol.Position{
						Line:      uint32(endPos.LineNumber() - 1),
						Character: uint32(endPos.ColumnNumber() - 1),
					},
				},
				Severity: 1, // Error
				Source:   "risor-lsp",
				Message:  parseErr.Message(),
			}
			log.Info().
				Uint32("start_line", diagnostic.Range.Start.Line).
				Uint32("start_char", diagnostic.Range.Start.Character).
				Uint32("end_line", diagnostic.Range.End.Line).
				Uint32("end_char", diagnostic.Range.End.Character).
				Str("message", diagnostic.Message).
				Msg("Adding diagnostic for parse error")
			diagnostics = append(diagnostics, diagnostic)
		} else {
			// Generic error handling for non-parser errors
			diagnostic := protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: 0, Character: 0},
				},
				Severity: 1, // Error
				Source:   "risor-lsp",
				Message:  doc.err.Error(),
			}
			log.Info().Str("message", diagnostic.Message).Msg("Adding diagnostic for generic error")
			diagnostics = append(diagnostics, diagnostic)
		}
	} else {
		log.Info().Msg("publishDiagnostics: No parse errors, clearing diagnostics")
	}

	log.Info().Int("diagnostic_count", len(diagnostics)).Str("uri", string(uri)).Msg("=== SENDING DIAGNOSTICS TO VSCODE ===")

	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	if current, err := s.cache.get(uri); err != nil || current != doc {
		log.Info().Str("uri", string(uri)).Msg("publishDiagnostics: document was replaced or closed, not publishing")
		return
	}

	// Publish diagnostics to the client
	if s.client != nil {
		params := &protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnostics,
		}

		err := s.client.PublishDiagnostics(context.Background(), params)
		if err != nil {
			log.Error().Err(err).Str("uri", string(uri)).Msg("!!! FAILED TO PUBLISH DIAGNOSTICS !!!")
		} else {
			log.Info().Str("uri", string(uri)).Msg("=== SUCCESSFULLY SENT DIAGNOSTICS TO VSCODE ===")
		}
	} else {
		log.Error().Msg("!!! CLIENT IS NIL - CANNOT PUBLISH DIAGNOSTICS !!!")
	}

	log.Info().Str("uri", string(uri)).Msg("=== publishDiagnostics END ===")
}

func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	old, err := s.cache.get(params.TextDocument.URI)
	if err != nil {
		log.Error().Err(err).Str("uri", string(params.TextDocument.URI)).Msg("failed to get document for change")
		return err
	}

	// Cached documents are read by other goroutines, so build a new document
	// rather than changing the cached one.
	item := old.item
	for _, change := range params.ContentChanges {
		if change.Range == nil {
			// Full document update
			item.Text = change.Text
		} else {
			// Incremental update - for now, we'll treat it as full update
			// TODO: Implement proper incremental updates
			item.Text = change.Text
		}
	}
	item.Version = params.TextDocument.Version

	// Reparse the document
	doc := newDocument(ctx, item)
	if err := s.cache.replace(doc); err != nil {
		log.Error().Err(err).Str("uri", string(params.TextDocument.URI)).Msg("failed to cache changed document")
		return err
	}
	if doc.err != nil {
		log.Error().Err(doc.err).Msg("parse program failed after change")
	} else {
		log.Info().Msg("parse program ok after change")
		// Force clear diagnostics when parsing succeeds (clear as you type!)
		s.forceClearDiagnostics(params.TextDocument.URI)
	}

	defer s.queueDiagnostics(params.TextDocument.URI)
	return nil
}

func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) (err error) {
	defer s.queueDiagnostics(params.TextDocument.URI)
	log.Info().Str("filename", params.TextDocument.URI.SpanURI().Filename()).Msg("DidOpen")
	doc := newDocument(ctx, params.TextDocument)
	if params.TextDocument.Text != "" {
		if doc.err != nil {
			log.Error().Err(doc.err).Msg("parse program failed")
		} else {
			log.Info().Msg("parse program ok")
		}
	}
	return s.cache.put(doc)
}

// newDocument returns a document for the given item, with its text parsed.
func newDocument(ctx context.Context, item protocol.TextDocumentItem) *document {
	doc := &document{
		item:                 item,
		linesChangedSinceAST: map[int]bool{},
	}
	if item.Text != "" {
		doc.ast, doc.err = parser.Parse(ctx, item.Text, nil)
	}
	return doc
}

func (s *Server) Initialize(ctx context.Context, params *protocol.ParamInitialize) (*protocol.InitializeResult, error) {
	log.Info().Msg("Initialize")
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			CompletionProvider: protocol.CompletionOptions{
				TriggerCharacters: []string{"."},
			},
			HoverProvider:              true,
			DefinitionProvider:         true,
			DocumentFormattingProvider: true,
			DocumentSymbolProvider:     true,
			ExecuteCommandProvider: protocol.ExecuteCommandOptions{
				Commands: []string{},
			},
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				Change:    protocol.Full,
				OpenClose: true,
				Save: protocol.SaveOptions{
					IncludeText: true,
				},
			},
		},
		ServerInfo: struct {
			Name    string `json:"name"`
			Version string `json:"version,omitempty"`
		}{
			Name:    s.name,
			Version: s.version,
		},
	}, nil
}

func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	log.Info().Str("filename", params.TextDocument.URI.SpanURI().Filename()).Str("uri", string(params.TextDocument.URI)).Msg("=== DidSave START ===")

	// Debug: Check if text is provided
	if params.Text == nil {
		log.Warn().Str("uri", string(params.TextDocument.URI)).Msg("DidSave: No text provided in save params - cannot re-parse")
		defer s.queueDiagnostics(params.TextDocument.URI)
		return nil
	}

	log.Info().Int("text_length", len(*params.Text)).Str("uri", string(params.TextDocument.URI)).Msg("DidSave: Text provided - will re-parse")

	// If text is provided, re-parse the document
	old, err := s.cache.get(params.TextDocument.URI)
	if err != nil {
		log.Error().Err(err).Str("uri", string(params.TextDocument.URI)).Msg("failed to get document for save")
		return err
	}

	log.Info().Str("old_text_length", fmt.Sprintf("%d", len(old.item.Text))).Str("new_text_length", fmt.Sprintf("%d", len(*params.Text))).Msg("DidSave: Updating document text")

	// Replace the cached document with one for the saved text
	item := old.item
	item.Text = *params.Text
	doc := newDocument(ctx, item)
	if err := s.cache.replace(doc); err != nil {
		log.Error().Err(err).Str("uri", string(params.TextDocument.URI)).Msg("failed to cache saved document")
		return err
	}
	if doc.err != nil {
		log.Error().Err(doc.err).Str("uri", string(params.TextDocument.URI)).Msg("parse program failed after save - will publish error diagnostic")
	} else {
		log.Info().Str("uri", string(params.TextDocument.URI)).Msg("parse program ok after save - will clear diagnostics")
		// Force clear diagnostics when parsing succeeds
		s.forceClearDiagnostics(params.TextDocument.URI)
	}

	log.Info().Str("uri", string(params.TextDocument.URI)).Msg("=== DidSave: queuing diagnostics ===")
	defer s.queueDiagnostics(params.TextDocument.URI)
	return nil
}

func (s *Server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	log.Info().Str("filename", params.TextDocument.URI.SpanURI().Filename()).Str("uri", string(params.TextDocument.URI)).Msg("=== DidClose START - force clearing diagnostics ===")

	// Remove the document first, so that a pending publisher does not send
	// its diagnostics after they are cleared.
	s.cache.remove(params.TextDocument.URI)

	// Clear diagnostics for closed document
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	if s.client != nil {
		diagnosticsParams := &protocol.PublishDiagnosticsParams{
			URI:         params.TextDocument.URI,
			Diagnostics: []protocol.Diagnostic{},
		}

		log.Info().Str("uri", string(diagnosticsParams.URI)).Msg("=== DidClose: FORCE CLEARING DIAGNOSTICS ===")

		err := s.client.PublishDiagnostics(context.Background(), diagnosticsParams)
		if err != nil {
			log.Error().Err(err).Str("uri", string(diagnosticsParams.URI)).Msg("!!! DidClose: FAILED TO CLEAR DIAGNOSTICS !!!")
		} else {
			log.Info().Str("uri", string(diagnosticsParams.URI)).Msg("=== DidClose: SUCCESSFULLY CLEARED DIAGNOSTICS ===")
		}
	} else {
		log.Error().Msg("!!! DidClose: CLIENT IS NIL !!!")
	}

	log.Info().Str("uri", string(params.TextDocument.URI)).Msg("=== DidClose END ===")
	return nil
}
