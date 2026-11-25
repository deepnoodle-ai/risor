module github.com/deepnoodle-ai/risor/experiments/v3/bench-expr

go 1.26.1

require (
	github.com/deepnoodle-ai/expr v1.4.1
	github.com/deepnoodle-ai/risor/go/v3 v3.0.0-00010101000000-000000000000
	github.com/deepnoodle-ai/wonton v0.14.0
	github.com/expr-lang/expr v1.18.1
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/deepnoodle-ai/risor/go/v3 => ../../../go
