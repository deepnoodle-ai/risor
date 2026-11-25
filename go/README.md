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

## Host functions from Go functions

A host function is a `Function`: a name, a signature in type text and an
implementation. `risor.Func` derives the signature from a Go function's type
and adapts the function, and `risor.Methods[T]` makes an opaque type of the
exported methods of a Go type `T`. Before:

```go
risor.Function{Name: "upper", Type: "(s: string) => string",
	Impl: func(c *risor.Call, args []any) (any, error) {
		return strings.ToUpper(args[0].(string)), nil
	}}
```

After:

```go
risor.Func("upper", strings.ToUpper, risor.Params("s"))
```

A host registers a few functions and one opaque type
([`example_test.go`](example_test.go), `Example_func`):

```go
type Account struct {
	Owner string
	Cents int64
}

func (*Account) OpaqueType() string        { return "Account" }
func (a *Account) Name() string            { return a.Owner }
func (a *Account) Balance() int64          { return a.Cents }
func (a *Account) CanPay(cents int64) bool { return cents <= a.Cents }

env, err := risor.NewEnv(
	risor.Var("id", "string"),
	risor.Methods[*Account]("Account", risor.ForMethod("CanPay", risor.Params("cents"))),
	risor.Func("account", findAccount, risor.Params("id")), // (id: string) => Account !
	risor.Func("upper", strings.ToUpper),                   // (a1: string) => string
)
```

| Go | Signature |
| --- | --- |
| A first `*risor.Call` or `context.Context` | Not a parameter. The function gets the call handle, to charge steps and units, or the run's context |
| Each other parameter | A parameter whose type is `TypeOf` of its Go type. Go keeps no parameter names, so they are `a1`, `a2`, ... unless `risor.Params` names them |
| No result, `R`, `error`, `(R, error)` | `=> null`, `=> R`, `=> null !`, `=> R !` |
| A method of `Methods[T]` | Its Go name with the first word in lower case: `Text` gives `text`, `URL` `url`, `HTTPStatus` `httpStatus`, `UserID` `userID`. `risor.Rename` and `risor.Skip` change the set |

- **Errors.** A Go type with no derived type (a channel, a function, a
  variadic function, other results) is a declaration error that `NewEnv`
  reports: `bad-type`, or `bad-signature` for a function parameter, wrong
  `Params` and two methods with one name.
- **Go interfaces.** A parameter or result of a Go interface type other than
  `any` is refused with `bad-type`, so that a later feature can give
  interfaces a meaning. To use one now, write the signature in the
  `Function`'s `Type`: Func then supplies only the implementation.
- **Calls.** An argument is converted to its Go type by the rules of
  `Decode`. One that does not convert fails the call with `function-error`.
  A returned error is `function-error` and a panic `function-panic`, as for
  any host function. An integer result past 2^53 − 1 fails where the run
  reads it, with `number-range`.
- **Speed.** Common shapes (up to three parameters of `string`, `int`,
  `int64`, `float64`, `bool` and `any`, see `Func`) get an adapter with no
  reflection, a few nanoseconds over a hand-written one. The rest use
  reflection, about 150 ns a call. `Function.Reflective` tells which.

The command-line program:

```bash
go install github.com/deepnoodle-ai/risor/go/cmd/risor@latest
```

Run the tests from this directory. The conformance suite in `../suite` runs
with them; tests that need it are skipped when it is not there.

```bash
go test ./...
```
