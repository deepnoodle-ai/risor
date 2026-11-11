# Risor v3 for Go

The Go implementation of [Risor v3](https://github.com/deepnoodle-ai/risor),
a small expression language that a host program embeds. This is an alpha and
a request for comment: the language and this API will change before 3.0.

```bash
go get github.com/deepnoodle-ai/risor/go/v3@latest
```

```go
import risor "github.com/deepnoodle-ai/risor/go/v3"
```

[`example_test.go`](example_test.go) is a complete example. The language is
defined by the [specification](https://github.com/deepnoodle-ai/risor/tree/main/spec)
and the [conformance suite](https://github.com/deepnoodle-ai/risor/tree/main/suite);
the TypeScript and Rust implementations are in
[`ts/`](https://github.com/deepnoodle-ai/risor/tree/main/ts) and
[`rust/`](https://github.com/deepnoodle-ai/risor/tree/main/rust).

The command-line program:

```bash
go install github.com/deepnoodle-ai/risor/go/v3/cmd/risor@latest
```

Run the tests from this directory. The conformance suite in `../suite` runs
with them; tests that need it are skipped when it is not there.

```bash
go test ./...
```
