// This package implements a Risor language server.
// It ships in the Risor release archives and the Homebrew cask next to
// the risor CLI. The VSCode extension finds it on PATH.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jdbaldry/go-language-server-protocol/jsonrpc2"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// version is set at build time by GoReleaser.
var version = "dev"

func main() {
	name := "risor-language-server"

	logOutput := io.Discard
	if logFile, err := openLogFile(); err == nil {
		defer logFile.Close()
		logOutput = logFile
	}

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(logOutput)

	ctx := context.Background()
	stream := jsonrpc2.NewHeaderStream(NewDefaultStdio())
	conn := jsonrpc2.NewConn(stream)
	client := protocol.ClientDispatcher(conn)

	s := Server{
		name:    name,
		version: version,
		client:  client,
		cache:   newCache(),
	}

	conn.Go(ctx, protocol.Handlers(
		protocol.ServerHandler(&s, jsonrpc2.MethodNotFound),
	))
	<-conn.Done()

	if err := conn.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

// openLogFile opens the server log in the user cache directory, for example
// ~/Library/Caches/risor/risor-lsp.log on macOS. Editors start the server in
// the workspace folder, so the log must not go in the working directory.
func openLogFile() (*os.File, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cacheDir, "risor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(
		filepath.Join(dir, "risor-lsp.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
}
