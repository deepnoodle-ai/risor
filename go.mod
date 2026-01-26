module github.com/risor-io/risor

go 1.25

require github.com/deepnoodle-ai/wonton v0.0.25

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
)

retract (
	v1.0.1 // ignores Tamarin release
	v1.0.0 // ignores Tamarin release
)
