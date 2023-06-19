# Tamarin

Tamarin is a fast and flexible embedded scripting language for Go projects.

```go
["welcome", "to", "tamarin", "👋"] | strings.join(" ")
```

## Why Choose Tamarin?

Tamarin may be a great fit for your project if any of this is valuable to you:

- **General purpose**: Work with JSON, HTTP, database connections, and more.
- **Fast**: The [fastest](https://raw.githubusercontent.com/cloudcmds/tamarin/main/bench/fib35.png) pure-Go scripting language _(as of June 2023)_
- **Familiar**: Friendly syntax for Go and Python developers.
- **Expressive**: Easily express lists, maps, sets and transformations on them.
- **Pipe expressions**: Quickly create processing pipelines.
- **Customizable**: Add your own types and built-in functions.
- **Single binary**: The Tamarin binary includes built-in libraries and packages.

## Usage Patterns

Tamarin is designed to be versatile and accommodate a variety of usage patterns:

- **REPL**: Tamarin offers a Read-Evaluate-Print-Loop (REPL) that you can use to interactively write and test scripts. This is perfect for experimentation and debugging.

- **Library**: Tamarin can be imported as a library into existing Go projects. It provides a simple API for running scripts and interacting with the results, in isolated environments for sandboxing.

- **Executable scripts**: Tamarin scripts can also be marked as executable, providing a simple way to leverage Tamarin in your build scripts, automation, and other tasks.

- **API**: (Coming soon) A service and API will be provided for remotely executing and managing Tamarin scripts. This will allow integration into various web applications, potentially with self-hosted and a managed cloud version.

## Use Cases

### Configuration

A common use case for embedded scripting languages is to make an application
dynamically configurable, without the need for a recompile. In this case,
an embedded scripting language can provide a way to load and run configuration
scripts at runtime.

### Hot-reloading and Modularity

In large applications, being able to dynamically load, execute, and unload
scripts while the application is still running can lead to more modular code and
a faster development cycle. Tamarin can offer a flexible way to achieve this.

### End-User Scripting

If you want to provide a way for users to customize your application's behavior
or extend its functionality, Tamarin can be a good choice. Setups like this are
seen in many video games and software tools that provide APIs for modders and
plugin creators.

### Prototyping

Tamarin can be used for quick prototyping within your Go application. Thanks to
streamlined syntax and faster development cycles, this can be a great way to
implement a first version of new features. Later, if the feature is well-received,
the Tamarin code can be easily transformed into raw Go as needed.

### Interacting with Different Environments

Tamarin is convenient for working with different environments like OS commands,
web APIs, and databases. Because Tamarin has excellent libraries built-in, you
can jump right to the interesting work without spending time researching third
party libraries.

### Glue Code

Scripting languages are often used as "glue code". Tamarin is handy for stitching
together different systems due to its lightweight nature. Running a Tamarin script
(instead of compiling a Go binary) is great for small integration tasks to support
a larger application written in Go.

## Getting Started

Head over to [Quick Start](quick-start.md) for information on how to start using
Tamarin.
