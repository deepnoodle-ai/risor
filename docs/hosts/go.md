# Embedding Risor v3 in Go

This guide shows how to embed Risor v3 in a Go application using the zero-dependency module `github.com/deepnoodle-ai/risor/go/v3`.

Every Go example in this guide is compiled and executed in CI against `./go` with `GOPROXY=off`.

## Contents

1. [Install and quick start](#1-install-and-quick-start)
2. [Environments and types](#2-environments-and-types)
3. [Host functions and opaque values](#3-host-functions-and-opaque-values)
4. [Limits, usage, and cancellation](#4-limits-usage-and-cancellation)
5. [Static analysis and templates](#5-static-analysis-and-templates)
6. [Conformance of host data and config loading](#6-conformance-of-host-data-and-config-loading)
7. [`SchemaOf`: exporting JSON Schema](#7-schemaof-exporting-json-schema)
8. [Entry points (`export const`)](#8-entry-points-export-const)
9. [The long-lived host pattern](#9-the-long-lived-host-pattern)
10. [Testing expressions and editor tools](#10-testing-expressions-and-editor-tools)

---

## 1. Install and quick start

Add the library module to your Go project. It has **no external dependencies** (`go/go.mod` contains no `require` directives).

```bash
go get github.com/deepnoodle-ai/risor/go/v3@latest
```

A Go host works in four stages ([`spec/core/03-host.md`](../../spec/core/03-host.md) 3.2):

1. **`risor.Parse`** turns source text into an immutable `*risor.Expr`. Parsing needs no environment.
2. **`env.Check`** (optional standalone step) checks types against a `*risor.Env` and reports the result type, residual failure sites, and exported entry points.
3. **`env.Compile`** checks (by default) and compiles the expression into an immutable `*risor.Program`.
4. **`prog.Run`** (or **`prog.Measure`**) evaluates the compiled program with per-run variables and a `context.Context`.

Every stage reports failures as a `*risor.Error` holding one or more `risor.Diagnostic` values of a single phase (`declare`, `parse`, `check`, or `run`). Use `risor.AsInt`, `risor.AsFloat`, `risor.Truthy`, and `risor.JSONText` to inspect results, and `risor.RenderLine` / `risor.RenderDetail` / `risor.RenderJSON` to format errors.

<!-- risor-verify: {"mode": "go", "expectStdout": "result=true total=120 json=true\nerror[no-field] 1:6-1:10: {age: int, name: string} has no field \"nmae\"\n"} -->
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

type User struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func main() {
	userType, err := risor.TypeOf[User]()
	if err != nil {
		log.Fatal(err)
	}
	env, err := risor.NewEnv(
		risor.VarType("user", userType),
		risor.Var("discount", "int"),
	)
	if err != nil {
		log.Fatal(err)
	}

	expr, err := risor.Parse(`user.age >= 18 && discount > 0`)
	if err != nil {
		log.Fatal(err)
	}
	prog, err := env.Compile(expr, risor.Expect("bool"))
	if err != nil {
		log.Fatal(err)
	}

	out, err := prog.Run(context.Background(), risor.Map{
		"user":     User{Name: "Ada", Age: 36},
		"discount": 15,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Read numeric results with AsInt / AsFloat and JSON with JSONText.
	priceProg, err := env.Compile(mustParse(`100 + 20`))
	if err != nil {
		log.Fatal(err)
	}
	priceVal, err := priceProg.Run(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	total, _ := risor.AsInt(priceVal)
	text, err := risor.JSONText(out)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("result=%v total=%d json=%s\n", risor.Truthy(out), total, text)

	// Render diagnostics consistently across all stages.
	badExpr, _ := risor.Parse(`user.nmae`)
	if _, err := env.Check(badExpr); err != nil {
		var re *risor.Error
		if errors.As(err, &re) {
			fmt.Print(risor.RenderLine(re.Diagnostics))
		}
	}
}

func mustParse(src string) *risor.Expr {
	x, err := risor.Parse(src)
	if err != nil {
		panic(err)
	}
	return x
}
```

> **Tip:** Always use `risor.Truthy(v)` when treating an arbitrary result as a condition—in Risor, empty lists `[]` and empty maps `{}` are falsy, unlike Go slices or maps. When checking a condition expression, pass `risor.Condition` to `env.Check` or `env.Compile` so constant conditions are caught at check time.

---

## 2. Environments and types

A `*risor.Env` is immutable once built and safe to share across goroutines. You can declare variables in four ways:

| Declaration | Input | Notes |
| --- | --- | --- |
| `risor.Var(name, typeText)` | Type text (`"int"`, `"{id: string}[]"`) | Parsed against declared opaque and named types. |
| `risor.VarRecord(recordText)` | One record type (`"{a: int, b: string}"`) | Declares one variable per field. |
| `risor.VarType(name, t)` | A `risor.Type` from `TypeOf`, `TypeFromSchema`, `ParseType`, or structural constructors | No parsing at `NewEnv` time; `TypeOf[T]()` also enables the fast typed field-access layer (`TypedLayer`). |
| `risor.VarSchema(name, schema, opts...)` | JSON Schema as `[]byte`, `json.RawMessage`, or Go maps/slices | Supports `risor.Closed`, `risor.AbsentIsNull`, and `risor.DefaultsApplied`. |

You can also build types directly without text (`ExactRecordType`, `RecordType`, `ListType`, `MapType`, `Nullable`, `LiteralType`, `TaggedUnionType`, `Join`), declare named types (`NamedType`, `NamedTypeOf`, `NamedTypeSchema`), enable opt-in library groups (`Groups("json", "time", "math", "base64")`), remove library entries (`Without("len", "string.repeat")`), configure rejected check classes (`Reject(...)`), and assert host facts (`SomeNotNull("primary", "fallback")`).

### Child environments and portable descriptions

`env.With(...)` derives a child environment without mutating the parent. A child shares its parent's compiled types for unchanged declarations, replaces declarations of the same name, and cannot re-enable entries removed by its parent.

`env.Describe()` emits a canonical JSON description of the environment (variables, function signatures, opaque types, named types, groups, removed entries, rejected classes, host facts, and limits). Another host—even in TypeScript or Rust, such as a browser editor—can call `FromDescription` to check and autocomplete programs against the exact same types without needing your Go function implementations.

<!-- risor-verify: {"mode": "go", "expectStdout": "eventType={kind: \"click\", x: int} | {code: string, kind: \"key\"}\nchildResult=ok fallback=backup\nroundTripSame=true\n"} -->
```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

func main() {
	// Structural type API: tagged union without writing type text.
	click := risor.ExactRecordType(
		risor.FieldType{Key: "kind", Type: risor.LiteralType("click")},
		risor.FieldType{Key: "x", Type: risor.TypeInt},
	)
	key := risor.ExactRecordType(
		risor.FieldType{Key: "kind", Type: risor.LiteralType("key")},
		risor.FieldType{Key: "code", Type: risor.TypeString},
	)
	eventType := risor.TaggedUnionType(click, key)
	if err := eventType.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("eventType=%s\n", eventType)

	// Declare variables via JSON Schema, named types, groups, and host facts.
	configSchema := []byte(`{
		"type": "object",
		"required": ["env"],
		"properties": {
			"env": {"enum": ["dev", "prod"]},
			"retries": {"type": "integer"}
		},
		"additionalProperties": false
	}`)

	parent, err := risor.NewEnv(
		risor.NamedTypeOf("Event", eventType),
		risor.VarSchema("cfg", configSchema),
		risor.Var("primary", "string | null"),
		risor.Var("fallback", "string | null"),
		risor.SomeNotNull("primary", "fallback"),
		risor.Groups("json"),
		risor.Without("string.repeat"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Child environment adds a variable without mutating parent.
	child, err := parent.With(risor.Var("ev", "Event"))
	if err != nil {
		log.Fatal(err)
	}

	expr, err := risor.Parse(`{
		status: match ev.kind { "click" => if (ev.x >= 0) { "ok" } else { "neg" }, "key" => ev.code },
		chosen: primary ?? fallback,
	}`)
	if err != nil {
		log.Fatal(err)
	}
	prog, err := child.Compile(expr)
	if err != nil {
		log.Fatal(err)
	}
	out, err := prog.Run(context.Background(), risor.Map{
		"ev":       map[string]any{"kind": "click", "x": 12},
		"primary":  nil,
		"fallback": "backup",
	})
	if err != nil {
		log.Fatal(err)
	}
	m := out.(map[string]any)
	fmt.Printf("childResult=%v fallback=%v\n", m["status"], m["chosen"])

	// Export and re-import an environment description for remote checking.
	desc, err := child.Describe()
	if err != nil {
		log.Fatal(err)
	}
	fromDesc, err := risor.FromDescription(desc)
	if err != nil {
		log.Fatal(err)
	}
	rebuilt, err := risor.NewEnv(fromDesc)
	if err != nil {
		log.Fatal(err)
	}
	desc2, err := rebuilt.Describe()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("roundTripSame=%v\n", bytes.Equal(desc, desc2))
}
```

> **Note:** An environment built with `FromDescription` has no host function or method implementations; calling `Compile` on it fails with `no-implementation` if any host function or method is declared. Use `Check`, `Complete`, and `Hover` on it, or override functions with `rebuilt.With(...)`.

---

## 3. Host functions and opaque values

Host functions are synchronous. Declare them either:

- **Manually** with `risor.Function{Name, Type, Impl, Doc}` receiving `(c *risor.Call, args []any) (any, error)`—giving full control over custom signatures, polymorphism, and resource accounting; or
- **By derivation** with `risor.Func(name, goFn, opts...)` and `risor.Methods[T](opaqueName, opts...)`, which derive Risor signatures from Go functions/methods and use reflection-free fast paths for common shapes.

Key rules for host functions ([`spec/core/03-host.md`](../../spec/core/03-host.md) 3.5):

- Mark fallible functions with `!` at the end of their signature: `(key: string) => string !`. If a function without `!` returns an error in a checked program, the run fails with `function-error` of class `environment`.
- Always charge proportional work with `c.Charge(steps)` and result memory with `c.Allocate(units)`. If either returns a non-nil error, return it immediately so the run ends with an uncatchable limit failure (`steps-exceeded` / `allocation-exceeded`).
- Respect `c.Context()` for long or cancellable operations.
- Returning an `error` produces `function-error` (catchable with `try { ... } catch`). Panicking produces `function-panic` (not catchable).
- Namespaces use one dot prefix (`"store.get"`). Library namespaces (`json`, `time`, `math`, `base64`, `regexp`) are reserved.

### Opaque types

An opaque value is a host value whose internal state stays in Go and is accessed only through declared methods. Implement `risor.Opaque` by defining `OpaqueType() string` on your type:

`OpaqueType()` must return a constant name without dereferencing its receiver (because `TypeOf` calls it on the zero value / nil pointer). Two opaque values are `==` in Risor when they are `==` in Go.

<!-- risor-verify: {"mode": "go", "expectStdout": "slug=hello-world token=alice:admin caught=missing key \"nope\"\n"} -->
```go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

type Session struct {
	User string
	Role string
}

func (*Session) OpaqueType() string { return "Session" }
func (s *Session) UserName() string { return s.User }
func (s *Session) Label(sep string) string {
	return s.User + sep + s.Role
}

func main() {
	store := map[string]string{"plan": "pro"}

	// Manual host function with step/allocation charging and error handling.
	storeGet := risor.Function{
		Name: "store.get",
		Type: "(key: string) => string !",
		Doc:  "Look up a string value by key.",
		Impl: func(c *risor.Call, args []any) (any, error) {
			if err := c.Context().Err(); err != nil {
				return nil, err
			}
			key := args[0].(string)
			if err := c.Charge(1); err != nil {
				return nil, err
			}
			val, ok := store[key]
			if !ok {
				return nil, fmt.Errorf("missing key %q", key)
			}
			if err := c.Allocate(int64(len(val)/64 + 1)); err != nil {
				return nil, err
			}
			return val, nil
		},
	}

	// Derived host function from a plain Go function.
	slug := risor.Func("slug", func(c *risor.Call, s string) (string, error) {
		if err := c.Charge(int64(len(s)/64 + 1)); err != nil {
			return "", err
		}
		out := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "-"))
		if err := c.Allocate(int64(len(out)/64 + 1)); err != nil {
			return "", err
		}
		return out, nil
	}, risor.Params("text"))

	// Derived opaque type methods from *Session.
	sessionMethods := risor.Methods[*Session](
		"Session",
		risor.ForMethod("Label", risor.Params("sep")),
		risor.Skip("UserName"),
	)

	env, err := risor.NewEnv(
		storeGet,
		slug,
		sessionMethods,
		risor.Var("sess", "Session"),
	)
	if err != nil {
		log.Fatal(err)
	}

	expr, err := risor.Parse(`{
		slug: slug("Hello World"),
		token: sess.label(":"),
		caught: try { store.get("nope") } catch (e) { e.message },
	}`)
	if err != nil {
		log.Fatal(err)
	}
	prog, err := env.Compile(expr)
	if err != nil {
		log.Fatal(err)
	}
	out, err := prog.Run(context.Background(), risor.Map{
		"sess": &Session{User: "alice", Role: "admin"},
	})
	if err != nil {
		log.Fatal(err)
	}
	m := out.(map[string]any)
	fmt.Printf("slug=%s token=%s caught=%s\n", m["slug"], m["token"], m["caught"])
}
```

---

## 4. Limits, usage, and cancellation

Limits are declared on the environment and enforced deterministically for every program compiled in it:

| Limit | Default | Disable / Special | Error code |
| --- | --- | --- | --- |
| `Steps` | `1_000_000` | `risor.NoLimit` (off), `risor.ZeroLimit` (`0`) | `steps-exceeded` |
| `Allocation` | `1_000_000` | `risor.NoLimit` (off), `risor.ZeroLimit` (`0`) | `allocation-exceeded` |
| `Deadline` | `0` (off) | `time.Duration` measured from run start | `deadline-exceeded` |
| `Depth` | `200` | `1` to `400` (cannot be disabled) | `depth-exceeded` |
| `SourceLimit` | `risor.MaxSource` (1,048,576 code points) | Option on `Parse`, `ParseTemplate`, `Format`, `Load`, `Complete`, `Hover` | `source-too-large` |

All five runtime limit codes (`steps-exceeded`, `allocation-exceeded`, `deadline-exceeded`, `depth-exceeded`, `cancelled`) have class `limit` and **cannot be caught** by `try { ... } catch`.

Use `prog.Measure(ctx, vars)` to inspect `risor.Usage{Steps, Allocated}` whether the run succeeds or fails.

<!-- risor-verify: {"mode": "go", "expectStdout": "sum=45 steps>0=true\nlimitCode=steps-exceeded\ncancelCode=cancelled\nsourceCode=source-too-large\n"} -->
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

func diagCode(err error) string {
	var re *risor.Error
	if errors.As(err, &re) && len(re.Diagnostics) > 0 {
		return re.Diagnostics[0].Code
	}
	return ""
}

func main() {
	env, err := risor.NewEnv(risor.Limits{
		Steps:      5_000,
		Allocation: 2_000,
		Deadline:   500 * time.Millisecond,
		Depth:      50,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Measure usage on a normal run.
	expr, err := risor.Parse(`range(0, 10).reduce((a, b) => a + b, 0)`)
	if err != nil {
		log.Fatal(err)
	}
	prog, err := env.Compile(expr)
	if err != nil {
		log.Fatal(err)
	}
	val, usage, err := prog.Measure(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	sum, _ := risor.AsInt(val)
	fmt.Printf("sum=%d steps>0=%v\n", sum, usage.Steps > 0)

	// Step limit cannot be caught by try/catch.
	heavy, _ := risor.Parse(`try { len(range(0, 50000)) } catch { 0 }`)
	heavyProg, err := env.Compile(heavy)
	if err != nil {
		log.Fatal(err)
	}
	_, _, err = heavyProg.Measure(context.Background(), nil)
	fmt.Printf("limitCode=%s\n", diagCode(err))

	// Cancel a run via context.Context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loopExpr, _ := risor.Parse(`len(range(0, 4000).map(n => n + 1))`)
	loopProg, err := env.Compile(loopExpr)
	if err != nil {
		log.Fatal(err)
	}
	_, err = loopProg.Run(ctx, nil)
	fmt.Printf("cancelCode=%s\n", diagCode(err))

	// Enforce a smaller source size limit at parse time.
	_, err = risor.Parse(`"this source is too long"`, risor.SourceLimit(8))
	fmt.Printf("sourceCode=%s\n", diagCode(err))
}
```

---

## 5. Static analysis and templates

### Static analysis (`Expr.Analyze`)

Because `Parse` requires no environment, you can inspect a parsed program *before* building an environment—for example, to fetch only the database records or workflow step outputs an expression actually references.

`expr.Analyze()` never fails and charges nothing. It reports:

- `Names`: sorted unique free identifiers read by the program.
- `Refs`: every free read in source order with its static property/projection `Path` (`Step{Key, Star}`) and `Span`.
- `Calls` / `CallsOf(names...)`: every call site with `Name`, `Namespace`, `InLambda`, `ArgSpans`, and `Args` (each non-lambda argument as its own `*risor.Expr` whose spans point into the original source!).
- `IsLiteral` / `Literal`: whether the program is a constant literal and its value.

### Templates (`ParseTemplate`)

`risor.ParseTemplate` parses text containing `${{ expr }}` interpolations (escape a literal `${{` as `$${{`). Choose a `TemplateMode`:

- `risor.ModeString`: always compiles to a `string` program.
- `risor.ModeValue`: if `tmpl.IsSingle()` is true (one `${{ ... }}` part and no surrounding text), returns the expression's typed value directly without stringifying it; otherwise behaves like `ModeString`.

You can also inspect individual parts with `tmpl.Segments()` and `tmpl.Part(i)` (returning an `*Expr` with spans in the full template text).

<!-- risor-verify: {"mode": "go", "expectStdout": "names=[order user] callArg=order.id\nmsg=Hi Ada, 2 items singleVal=36\n"} -->
```go
package main

import (
	"context"
	"fmt"
	"log"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

func main() {
	expr, err := risor.Parse(`lookup(order.id) && user.active`)
	if err != nil {
		log.Fatal(err)
	}
	a := expr.Analyze()
	lookupCalls := a.CallsOf("lookup")
	fmt.Printf("names=%v callArg=%s\n", a.Names, lookupCalls[0].Args[0].Text())

	env, err := risor.NewEnv(
		risor.VarRecord("{user: {name: string, age: int}, items: string[]}"),
	)
	if err != nil {
		log.Fatal(err)
	}
	vars := risor.Map{
		"user":  map[string]any{"name": "Ada", "age": 36},
		"items": []any{"a", "b"},
	}

	// ModeString: always produces a string.
	msgTmpl, err := risor.ParseTemplate(`Hi ${{ user.name }}, ${{ len(items) }} items`)
	if err != nil {
		log.Fatal(err)
	}
	msgProg, err := env.Compile(msgTmpl.Program(risor.ModeString))
	if err != nil {
		log.Fatal(err)
	}
	msg, err := msgProg.Run(context.Background(), vars)
	if err != nil {
		log.Fatal(err)
	}

	// ModeValue on a single-part template preserves the expression's type (int).
	valTmpl, err := risor.ParseTemplate(`${{ user.age }}`)
	if err != nil {
		log.Fatal(err)
	}
	if !valTmpl.IsSingle() {
		log.Fatal("expected single template")
	}
	valProg, err := env.Compile(valTmpl.Program(risor.ModeValue))
	if err != nil {
		log.Fatal(err)
	}
	val, err := valProg.Run(context.Background(), vars)
	if err != nil {
		log.Fatal(err)
	}
	age, _ := risor.AsInt(val)
	fmt.Printf("msg=%s singleVal=%d\n", msg, age)
}
```

---

## 6. Conformance of host data and config loading

Declared types are a promise that the evaluator does not blindly trust ([`spec/core/03-host.md`](../../spec/core/03-host.md) 3.4.8). To validate untrusted host data *before* a run—or to validate a run's output—use:

- `risor.Conform(v, t, opts...)`: returns `nil` if `v` conforms to `t`, or an `*Error` with the first `not-conforming` diagnostic (including its structured `Path`).
- `risor.ConformAll(v, t, max, opts...)`: collects up to `max` failures. Both charge steps against the default 1,000,000-step conformance limit (overridable via `risor.ConformSteps(n)`).

### Config files (`Load`, `LoadValue`, `Decode`)

`risor.Load[T]` runs all six config-loading steps in order and decodes the result into a Go struct `*T` (preserving defaults already populated in `dst` for optional omitted fields):

| Step | Constant | What happens |
| --- | --- | --- |
| 0 | `StepDeclare` | Derive `TypeOf[T]` and validate options |
| 1 | `StepParse` | Parse source text |
| 2 | `StepCheck` | Type-check and compile against `T` (always checked) |
| 3 | `StepRun` | Evaluate with `ctx` and `vars` |
| 4 | `StepConform` | Run `ConformAll` up to `MaxFailures(n)` |
| 5 | `StepDecode` | Decode into `dst` (copied; shares no memory with inputs) |

Failures return `*risor.LoadError`, which records `Step`, `Diagnostics`, and `Usage` (when step 3 ran), and unwraps to `*risor.Error`. Use `risor.LoadValue` when you want the dynamic value without decoding into a Go struct, or `risor.Decode(v, &dst)` directly on any Risor result.

<!-- risor-verify: {"mode": "go", "expectStdout": "conformErrs=2 paths=[[port] [tls]]\nloaded=0.0.0.0:8443 retries=3\nloadFailStep=5\n"} -->
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

type ServerConfig struct {
	Host    string `json:"host"`
	Port    uint16 `json:"port"`
	Retries int    `json:"retries,omitempty"`
}

func main() {
	env, err := risor.NewEnv(risor.Var("prod", "bool"))
	if err != nil {
		log.Fatal(err)
	}

	// Validate untrusted host data with ConformAll.
	t, err := env.ParseType("{port: int, tls: bool}")
	if err != nil {
		log.Fatal(err)
	}
	diags := risor.ConformAll(map[string]any{"port": "bad", "tls": 123}, t, 10)
	fmt.Printf("conformErrs=%d paths=[%v %v]\n", len(diags), diags[0].Path, diags[1].Path)

	// Load a config program into a Go struct with defaults preserved.
	cfg := ServerConfig{Retries: 3}
	src := `{
		host: if (prod) { "0.0.0.0" } else { "127.0.0.1" },
		port: if (prod) { 8443 } else { 8080 },
	}`
	err = risor.Load(context.Background(), env, src, risor.Map{"prod": true}, &cfg, risor.MaxFailures(5))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("loaded=%s:%d retries=%d\n", cfg.Host, cfg.Port, cfg.Retries)

	// An integer that overflows uint16 fails at StepDecode (step 5).
	var bad ServerConfig
	err = risor.Load(context.Background(), env, `{host: "localhost", port: 70000}`, nil, &bad)
	var le *risor.LoadError
	if errors.As(err, &le) {
		fmt.Printf("loadFailStep=%d\n", le.Step)
	}
}
```

---

## 7. `SchemaOf`: exporting JSON Schema

`risor.SchemaOf(t)` exports any non-opaque `risor.Type` as canonical JSON Schema (`json.RawMessage`, keys in code point order, no extra whitespace):

- Works on types derived from Go structs (`TypeOf[T]()`), checked program result types (`cr.Type`), parsed type text (`env.ParseType(...)`), and entry point input types (`*entry.InputType`).
- Preserves doc comments on record fields and entry parameters as `"description"`.
- Emits recursive types once under `"$defs"`.
- Round-trips cleanly with `risor.TypeFromSchema`.
- Refuses types containing an opaque type or type variable with `declare` error `bad-type`.

<!-- risor-verify: {"mode": "go", "expectStdout": "schema={\"additionalProperties\":false,\"properties\":{\"city\":{\"description\":\"Destination city.\",\"type\":\"string\"},\"units\":{\"enum\":[\"C\",\"F\"],\"type\":\"string\"}},\"required\":[\"city\"],\"type\":\"object\"}\nroundTripEqual=true opaqueCode=bad-type\n"} -->
```go
package main

import (
	"errors"
	"fmt"
	"log"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

func main() {
	env, err := risor.NewEnv(
		risor.OpaqueType{Name: "Token"},
		risor.Var("req", `{/** Destination city. */ city: string, units?: "C" | "F"}`),
	)
	if err != nil {
		log.Fatal(err)
	}

	expr, _ := risor.Parse(`req`)
	cr, err := env.Check(expr)
	if err != nil {
		log.Fatal(err)
	}

	raw, err := risor.SchemaOf(cr.Type)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("schema=%s\n", string(raw))

	back, err := risor.TypeFromSchema(raw)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("roundTripEqual=%v ", back.Equal(cr.Type))

	_, err = risor.SchemaOf(risor.OpaqueTypeNamed("Token"))
	var re *risor.Error
	if errors.As(err, &re) {
		fmt.Printf("opaqueCode=%s\n", re.Diagnostics[0].Code)
	}
}
```

---

## 8. Entry points (`export const`)

A Risor program can export one or more typed functions with `export const`, with or without a final result expression. This is ideal for toolkits, rule packs, and LLM-callable tools:

- **`prog.Entries()`** (and `cr.Entries`) returns `[]risor.Entry` in source order, with `Name`, `Doc`, `Params`, `InputType`, and `ResultType`.
- **`risor.SchemaOf(*entry.InputType)`** turns an entry point's parameters (including parameter doc comments and optional defaults) directly into a JSON Schema for tool definitions or UI forms.
- **`risor.ExpectEntry` / `ExpectEntryType` / `ExpectEntrySchema`** enforce expected return types on individual entry points during `Check` and `Compile`.
- **`prog.Call` / `CallMeasure`** invoke an entry point with positional `[]any` arguments, conforming each argument before execution.
- **`prog.CallRecord` / `CallRecordMeasure`** invoke an entry point with a single input map/struct conformed against `InputType`.
- If a program has no final result expression, `prog.HasResult()` is `false` and `prog.Run` fails with `declare` error `no-result`.

<!-- risor-verify: {"mode": "go", "expectStdout": "hasResult=false entry=quote doc=Calculate an order total.\npositional=120 record=135\nschemaHasDesc=true\n"} -->
```go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

func main() {
	env, err := risor.NewEnv(risor.Var("taxRate", "float"))
	if err != nil {
		log.Fatal(err)
	}

	src := `
const multiplier = 1 + taxRate;

/** Calculate an order total. */
export const quote = (
  /** Subtotal in USD. */
  subtotal: float,
  /** Optional shipping fee. */
  shipping: float = 0.0,
): float => subtotal * multiplier + shipping;
`
	expr, err := risor.Parse(src)
	if err != nil {
		log.Fatal(err)
	}
	prog, err := env.Compile(expr, risor.ExpectEntry("quote", "float"))
	if err != nil {
		log.Fatal(err)
	}

	entry := prog.Entries()[0]
	fmt.Printf("hasResult=%v entry=%s doc=%s\n", prog.HasResult(), entry.Name, entry.Doc)

	ctx := context.Background()
	vars := risor.Map{"taxRate": 0.2}

	v1, err := prog.Call(ctx, "quote", []any{100.0}, vars)
	if err != nil {
		log.Fatal(err)
	}
	v2, err := prog.CallRecord(ctx, "quote", map[string]any{
		"subtotal": 100.0,
		"shipping": 15.0,
	}, vars)
	if err != nil {
		log.Fatal(err)
	}
	f1, _ := risor.AsFloat(v1)
	f2, _ := risor.AsFloat(v2)
	fmt.Printf("positional=%.0f record=%.0f\n", f1, f2)

	inputSchema, err := risor.SchemaOf(*entry.InputType)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("schemaHasDesc=%v\n", strings.Contains(string(inputSchema), `"description":"Subtotal in USD."`))
}
```

---

## 9. The long-lived host pattern

Servers, workflow engines, and UI event loops embed Risor using the pattern in [`spec/core/03-host.md`](../../spec/core/03-host.md) 3.2.5:

1. **Build one `*risor.Env` at startup** with your domain types, host functions, and limits.
2. **Parse, check, and compile once** when user rules or handlers are saved or loaded.
3. **Reuse the immutable `*risor.Program` concurrently** across requests or events—each run gets its own `Vars`, budget, and `context.Context`.
4. **The host owns all state.** Each event runs with `{state, ev}` and returns data: the next state and a list of tagged-union actions (`{kind: "notify", ...} | {kind: "noop"}`), which the host applies in Go.

### Poisoned runs and nested runs

- **Poisoned run:** When a host function detects a fatal condition and must sever the run from the host immediately—even if the Risor program wraps the call in `try { ... } catch`—keep an `atomic.Bool` per run alongside a `context.CancelFunc`. Set the flag, cancel the context, refuse all subsequent host calls while the flag is set, and discard the run's return value if poisoned.
- **Nested run:** When a host function evaluates another Risor program (such as a tool calling another tool), pass the caller's remaining budget via `toolEnv.With(risor.Budget(c.Remaining()))`, forward `c.Context()`, and charge back `used.Steps` and `used.Allocated`. If the nested run hit `steps-exceeded` or `allocation-exceeded`, charge `math.MaxInt64` so the caller fails with the same uncatchable limit code.

<!-- risor-verify: {"mode": "go", "expectStdout": "concurrent=[2 4 6 8]\nnestedResult=12 nestedSteps>0=true\nnestedLimitCode=steps-exceeded\n"} -->
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

func main() {
	// 1. Build environment once, compile handler once, run concurrently.
	handlerEnv, err := risor.NewEnv(
		risor.Var("state", "{count: int}"),
		risor.Var("ev", `{kind: "inc", by: int} | {kind: "reset"}`),
		risor.Limits{Steps: 10_000, Allocation: 10_000},
	)
	if err != nil {
		log.Fatal(err)
	}
	handlerExpr, err := risor.Parse(`
		match ev.kind {
			"inc" => {count: state.count + ev.by},
			"reset" => {count: 0},
		}
	`)
	if err != nil {
		log.Fatal(err)
	}
	handlerProg, err := handlerEnv.Compile(handlerExpr, risor.Expect("{count: int}"))
	if err != nil {
		log.Fatal(err)
	}

	results := make([]int64, 4)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			out, err := handlerProg.Run(context.Background(), risor.Map{
				"state": map[string]any{"count": idx + 1},
				"ev":    map[string]any{"kind": "inc", "by": idx + 1},
			})
			if err != nil {
				log.Fatal(err)
			}
			n, _ := risor.AsInt(out.(map[string]any)["count"])
			results[idx] = n
		}(i)
	}
	wg.Wait()
	fmt.Printf("concurrent=%v\n", results)

	// 2. Nested run sharing the caller's remaining step & allocation budget.
	toolEnv, err := risor.NewEnv(risor.Var("xs", "int[]"))
	if err != nil {
		log.Fatal(err)
	}
	toolExpr, err := risor.Parse(`xs.map(n => n * 2).reduce((a, b) => a + b, 0)`)
	if err != nil {
		log.Fatal(err)
	}

	runTool := risor.Function{
		Name: "runTool",
		Type: "(xs: int[]) => int !",
		Impl: func(c *risor.Call, args []any) (any, error) {
			child, err := toolEnv.With(risor.Budget(c.Remaining()))
			if err != nil {
				return nil, err
			}
			p, err := child.Compile(toolExpr)
			if err != nil {
				return nil, err
			}
			v, used, runErr := p.Measure(c.Context(), risor.Map{"xs": args[0]})
			if err := c.Charge(used.Steps); err != nil {
				return nil, err
			}
			if err := c.Allocate(used.Allocated); err != nil {
				return nil, err
			}
			var re *risor.Error
			if errors.As(runErr, &re) && len(re.Diagnostics) > 0 {
				switch re.Diagnostics[0].Code {
				case "steps-exceeded":
					return nil, c.Charge(math.MaxInt64)
				case "allocation-exceeded":
					return nil, c.Allocate(math.MaxInt64)
				}
			}
			return v, runErr
		},
	}

	parentEnv, err := risor.NewEnv(runTool, risor.Limits{Steps: 500, Allocation: 500})
	if err != nil {
		log.Fatal(err)
	}
	callerExpr, _ := risor.Parse(`runTool([1, 2, 3])`)
	callerProg, err := parentEnv.Compile(callerExpr)
	if err != nil {
		log.Fatal(err)
	}
	v, u, err := callerProg.Measure(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	n, _ := risor.AsInt(v)
	fmt.Printf("nestedResult=%d nestedSteps>0=%v\n", n, u.Steps > 0)

	// Tight budget causes nested run to stop caller with steps-exceeded.
	tightEnv, _ := parentEnv.With(risor.Limits{Steps: 5})
	tightProg, _ := tightEnv.Compile(callerExpr)
	_, err = tightProg.Run(context.Background(), nil)
	var re *risor.Error
	if errors.As(err, &re) {
		fmt.Printf("nestedLimitCode=%s\n", re.Diagnostics[0].Code)
	}
}
```

---

## 10. Testing expressions and editor tools

### Case-file testing with `risortest`

Package `github.com/deepnoodle-ai/risor/go/v3/risortest` runs Risor JSON (`.test.json`) and program (`.test.risor`) case files directly inside `go test` against your application's real `*risor.Env`. Set `RISOR_UPDATE=1` to update expected outputs in place.

<!-- risor-verify: {"mode": "go-test", "files": {"testdata/discount.test.json": "{\n  \"env\": \"{vip: bool, total: float}\",\n  \"cases\": [\n    {\n      \"name\": \"vip gets 20 percent\",\n      \"source\": \"if (vip) { total * 0.8 } else { total }\",\n      \"vars\": {\"vip\": true, \"total\": 100},\n      \"expect\": {\"value\": 80}\n    },\n    {\n      \"name\": \"regular pays full price\",\n      \"source\": \"if (vip) { total * 0.8 } else { total }\",\n      \"vars\": {\"vip\": false, \"total\": 100},\n      \"expect\": {\"value\": 100}\n    }\n  ]\n}\n"}} -->
```go
package discount_test

import (
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/risortest"
)

func TestDiscountRules(t *testing.T) {
	env, err := risor.NewEnv(
		risor.Var("vip", "bool"),
		risor.Var("total", "float"),
	)
	if err != nil {
		t.Fatal(err)
	}
	risortest.Run(t, env, "testdata/discount.test.json")
}
```

### Formatting, completion, and hover

Risor ships built-in editor primitives that never require a language server:

- `risor.Format(source)` and `risor.FormatTemplate(source)` produce canonical formatting (80 columns, 2-space indent).
- `env.Complete(source, pos)` and `env.Hover(source, pos)` work on incomplete or partially typed expressions and never fail.

<!-- risor-verify: {"mode": "go", "expectStdout": "formatted=1 + 2\ncompletions=[name]\nhoverType=string\n"} -->
```go
package main

import (
	"fmt"
	"log"
	"strings"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

func main() {
	formatted, err := risor.Format("1+2")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("formatted=%s\n", strings.TrimSpace(formatted))

	env, err := risor.NewEnv(risor.VarRecord("{user: {name: string, age: int}}"))
	if err != nil {
		log.Fatal(err)
	}

	comp := env.Complete("user.na", risor.Pos{Line: 1, Column: 8})
	labels := make([]string, len(comp.Items))
	for i, item := range comp.Items {
		labels[i] = item.Label
	}
	fmt.Printf("completions=%v\n", labels)

	hover := env.Hover("user.name", risor.Pos{Line: 1, Column: 7})
	if hover != nil && hover.Type != nil {
		fmt.Printf("hoverType=%s\n", *hover.Type)
	}
}
```

---

## Further reading

- [Risor v3 Language Guide](../guide.md)
- [Host specification (`spec/core/03-host.md`)](../../spec/core/03-host.md)
- [TypeScript Host Guide](typescript.md)
- [Rust Host Guide](rust.md)
