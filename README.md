# Risor

[![CircleCI](https://dl.circleci.com/status-badge/img/gh/risor-io/risor/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/risor-io/risor/tree/main)
[![Apache-2.0 license](https://img.shields.io/badge/License-Apache%202.0-brightgreen.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/risor-io/risor)
[![Go Report Card](https://goreportcard.com/badge/github.com/risor-io/risor?style=flat-square)](https://goreportcard.com/report/github.com/risor-io/risor)
[![Releases](https://img.shields.io/github/release/risor-io/risor/all.svg?style=flat-square)](https://github.com/risor-io/risor/releases)

Risor is a fast and flexible scripting language for Go developers and DevOps.

Its modules integrate the Go standard library, making it easy to use functions
that you're already familiar with as a Go developer.

Scripts are compiled to bytecode and then run on a lightweight virtual machine.
Risor is written in pure Go.

## Documentation

Documentation is available at [risor.io](https://risor.io).

You might also want to try evaluating Risor scripts [from your browser](https://risor.io/#editor).

## Syntax Example

Here's a short example of how Risor feels like a hybrid of Go and Python. This
demonstrates using string methods and chained calls:

```go
let array = ["gophers", "are", "burrowing", "rodents"]

let sentence = " ".join(array).to_upper()

// sentence is "GOPHERS ARE BURROWING RODENTS"
```

## Getting Started

You might want to head over to [Getting Started](https://risor.io/docs) in the
documentation. That said, here are tips for both the CLI and the Go library.

### Risor CLI and REPL

If you use [Homebrew](https://brew.sh/), you can install the
[Risor](https://formulae.brew.sh/formula/risor) CLI as follows:

```
brew install risor
```

Having done that, just run `risor` to start the CLI or `risor -h` to see
usage information.

Execute a code snippet directly using the `-c` option:

```go
risor -c "time.now()"
```

Start the REPL by running `risor` with no options.

### Build and Install the CLI from Source

Build the CLI from source as follows:

```bash
git clone git@github.com:risor-io/risor.git
cd risor/cmd/risor
go install .
```

### Go Library

Use `go get` to add Risor as a dependency of your Go program:

```bash
go get github.com/risor-io/risor
```

Here's an example of using the `risor.Eval` API to evaluate some code:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/risor-io/risor"
)

func main() {
	ctx := context.Background()
	script := "math.sqrt(input)"

	// Start with the standard library and add custom variables
	env := risor.Builtins()
	env["input"] = 4

	result, err := risor.Eval(ctx, script, risor.WithEnv(env))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The square root of 4 is:", result)
}
```

## Built-in Functions and Modules

24 built-in functions are included and are documented [here](https://risor.io/docs/builtins).
These include type conversions (`int`, `float`, `string`, `list`, `set`, etc.),
container operations (`len`, `keys`, `delete`, `chunk`), and utilities like
`filter`, `sorted`, `encode`, and `decode`.

The `encode` and `decode` builtins support multiple formats: `base64`, `base32`,
`hex`, `json`, `csv`, and `urlquery`.

Four modules are included: `math`, `rand`, `regexp`, and `time`.

## Go Interface

It is trivial to embed Risor in your Go program in order to evaluate scripts
that have access to arbitrary Go structs and other types.

The simplest way to use Risor is to call the `Eval` function and provide the script source code.
By default, the environment is empty. Use `risor.Builtins()` to get the standard library:

```go
result, err := risor.Eval(ctx, "math.min([5, 2, 7])", risor.WithEnv(risor.Builtins()))
// result is 2
```

Provide input to the script using `WithEnv`:

```go
env := risor.Builtins()
env["input"] = 16
result, err := risor.Eval(ctx, "input | math.sqrt", risor.WithEnv(env))
// result is 4.0
```

Use the same mechanism to inject a struct. You can then access fields or call
methods on the struct from the Risor script:

```go
type Example struct {
    Message string
}
example := &Example{"abc"}
env := risor.Builtins()
env["ex"] = example
result, err := risor.Eval(ctx, "len(ex.Message)", risor.WithEnv(env))
// result is 3
```

## Adding Custom Modules

Risor is designed to have minimal external dependencies in its core libraries.
You can add custom modules to the environment when embedding Risor:

```go
import (
    "github.com/risor-io/risor"
    "github.com/risor-io/risor/object"
)

func main() {
    env := risor.Builtins()
    env["custom"] = object.NewBuiltinsModule("custom", map[string]object.Object{
        "hello": object.NewBuiltin("hello", myHelloFunc),
    })
    result, err := risor.Eval(ctx, source, risor.WithEnv(env))
    // ...
}
```

## Syntax Highlighting

A [Risor VSCode extension](https://marketplace.visualstudio.com/items?itemName=CurtisMyzie.risor-language)
is already available which currently only offers syntax highlighting.

You can also make use of the [Risor TextMate grammar](./vscode/syntaxes/risor.grammar.json).

## Benchmarking

There are two Makefile commands that assist with benchmarking and CPU profiling:

```
make bench
make pprof
```

## Contributing

Risor is intended to be a community project. You can lend a hand in various ways:

- Please ask questions and share ideas in [GitHub discussions](https://github.com/risor-io/risor/discussions)
- Share Risor on any social channels that may appreciate it
- Open GitHub issue or a pull request for any bugs you find
- Star the project on GitHub

### Contributing New Modules

Adding modules to Risor is a great way to get involved with the project.
See [this guide](https://risor.io/docs/contributing_modules) for details.

## Community Projects

- [RSX: Package Risor Scripts into Go Binaries](https://github.com/rubiojr/rsx)
- [Awesome Risor](https://github.com/rubiojr/awesome-risor)
- [tree-sitter-risor](https://github.com/applejag/tree-sitter-risor)
- [bench_go_scripting](https://github.com/mna/bench_go_scripting)

## Discuss the Project

Please visit the [GitHub discussions](https://github.com/risor-io/risor/discussions)
page to share thoughts and questions.

There is also a `#risor` Slack channel on the [Gophers Slack](https://gophers.slack.com).

## Credits

Check [CREDITS.md](./CREDITS.md).

## License

Released under the [Apache License, Version 2.0](./LICENSE).

Copyright Curtis Myzie / [github.com/myzie](https://github.com/myzie).
