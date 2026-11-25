module github.com/deepnoodle-ai/risor/go/cmd/risor

go 1.26.0

replace github.com/deepnoodle-ai/risor/go/v3 => ../..

require (
	github.com/deepnoodle-ai/risor/go/v3 v3.0.0-00010101000000-000000000000
	github.com/deepnoodle-ai/wonton v0.14.0
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/yuin/goldmark v1.8.5 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/term v0.47.0 // indirect
)
