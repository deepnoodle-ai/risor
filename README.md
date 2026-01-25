<h1><img src="https://github.com/risor-io/risor/raw/main/static/images/risor-logo-nopad.png" alt="Risor logo" height="64" valign="middle"> Risor</h1>

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
demonstrates using Risor's pipe expressions to apply a series of transformations:

```go
array := ["gophers", "are", "burrowing", "rodents"]

sentence := array | strings.join(" ") | strings.to_upper

print(sentence)
```

Output:

```
GOPHERS ARE BURROWING RODENTS
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
	result, err := risor.Eval(ctx, script, risor.WithGlobal("input", 4))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The square root of 4 is:", result)
}
```

## Built-in Functions and Modules

30+ built-in functions are included and are documented [here](https://risor.io/docs/builtins).

Modules are included that generally wrap the equivalent Go package. For example,
there is direct correspondence between `base64`, `bytes`, `filepath`, `json`, `math`, `os`,
`rand`, `regexp`, `strconv`, `strings`, and `time` Risor modules and
the Go standard library.

## Go Interface

It is trivial to embed Risor in your Go program in order to evaluate scripts
that have access to arbitrary Go structs and other types.

The simplest way to use Risor is to call the `Eval` function and provide the script source code.
The result is returned as a Risor object:

```go
result, err := risor.Eval(ctx, "math.min([5, 2, 7])")
// result is 2, as an *object.Int
```

Provide input to the script using Risor options:

```go
result, err := risor.Eval(ctx, "input | strings.to_upper", risor.WithGlobal("input", "hello"))
// result is "HELLO", as an *object.String
```

Use the same mechanism to inject a struct. You can then access fields or call
methods on the struct from the Risor script:

```go
type Example struct {
    Message string
}
example := &Example{"abc"}
result, err := risor.Eval(ctx, "len(ex.Message)", risor.WithGlobal("ex", example))
// result is 3, as an *object.Int
```

## Optional Modules

Risor is designed to have minimal external dependencies in its core libraries.
Two optional modules are available that use `golang.org/x/crypto`:

| Name   | Path                               | Go Get Command                                    |
| ------ | ---------------------------------- | ------------------------------------------------- |
| bcrypt | [modules/bcrypt](./modules/bcrypt) | `go get github.com/risor-io/risor/modules/bcrypt` |
| ssh    | [modules/ssh](./modules/ssh)       | `go get github.com/risor-io/risor/modules/ssh`    |

These modules are included by default when using the Risor CLI. When building
Risor into your own program, you'll need to opt-in using `go get` and then add
the modules as globals:

```go
import (
    "github.com/risor-io/risor"
    "github.com/risor-io/risor/modules/bcrypt"
    "github.com/risor-io/risor/modules/ssh"
)

func main() {
    source := `bcrypt.hash("secret")`
    result, err := risor.Eval(ctx, source,
        risor.WithGlobals(map[string]any{
            "bcrypt": bcrypt.Module(),
            "ssh":    ssh.Module(),
        }))
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
