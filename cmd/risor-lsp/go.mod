module github.com/deepnoodle-ai/risor/v2/cmd/risor-lsp

go 1.25

replace github.com/deepnoodle-ai/risor/v2 => ../..

require (
	github.com/deepnoodle-ai/risor/v2 v2.0.0
	github.com/deepnoodle-ai/wonton v0.0.25
	github.com/jdbaldry/go-language-server-protocol v0.0.0-20211013214444-3022da0884b2
	github.com/rs/zerolog v1.34.0
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
)
