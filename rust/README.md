# risor

> **Risor v3 is a release candidate.** Until `3.0.0`, only fixes change the
> language and host API.

Risor is a small expression language that a host program embeds. The host
gives it data and gets a value back. Typical uses are a condition in a
workflow step, a template, a query over JSON, and a config file.

This crate is the Rust implementation. It has no external dependencies, uses
`#![forbid(unsafe_code)]`, and shares a conformance suite with the Go and
TypeScript implementations so all three agree on results, errors, and resource
counts.

## Installation

Add `risor` to your `Cargo.toml`:

```toml
[dependencies]
risor = "3.0.0-rc.1"
```

Enable the `derive` feature to use `#[derive(risor::Host)]` on Rust structs:

```toml
[dependencies]
risor = { version = "3.0.0-rc.1", features = ["derive"] }
```

## Run a program

```rust
use risor::{parse, CompileOptions, Data, Env};
use std::collections::BTreeMap;

let env = Env::builder()
    .var("order", "{total: float, items: {sku: string}[]}")
    .build()
    .unwrap();
let expr = parse("order.total > 100 && len(order.items) > 0").unwrap();
let program = env.compile(&expr, CompileOptions::default()).unwrap();

let mut item = BTreeMap::new();
item.insert("sku".into(), Data::Str("A1".into()));
let mut order = BTreeMap::new();
order.insert("total".into(), Data::Float(120.0));
order.insert("items".into(), Data::List(vec![Data::Map(item)]));
let mut vars = BTreeMap::new();
vars.insert("order".into(), Data::Map(order));

let value = program.run(&vars).unwrap();
assert_eq!(value.as_bool(), Some(true));
```

`parse` reads the text, and `compile` checks the program against the declared
variables. Given variables that match their declared types, a checked program
cannot fail with an unknown name, a missing field, a null, or a wrong kind. A
program that does not parse or check returns a `risor::Error` carrying
structured diagnostics:

```rust
let err = env
    .compile(&parse("order.totl > 100").unwrap(), CompileOptions::default())
    .err()
    .unwrap();
assert_eq!(err.first().code, "no-field");
```

## Expose Rust structs with `derive`

With `features = ["derive"]`, `#[derive(risor::Host)]` implements `HostType`,
`Host`, `HostMap`, and (for owned structs) `Decode`, reading host data in
place with no intermediate copies:

```rust
use risor::{parse, CompileOptions, Env, Host};
use std::collections::HashMap;

#[derive(Host)]
struct Item {
    sku: String,
}

#[derive(Host)]
struct Order {
    total: f64,
    items: Vec<Item>,
    #[risor(absent_if_none)]
    note: Option<String>,
}

let env = Env::builder().var_host::<Order>("order").build().unwrap();
let expr = parse(r#"order.total > 100 && (order?.note ?? "") != "hold""#).unwrap();
let program = env.compile(&expr, CompileOptions::default()).unwrap();

let mut vars = HashMap::new();
vars.insert(
    "order".into(),
    Order {
        total: 120.0,
        items: vec![Item { sku: "A1".into() }],
        note: None,
    },
);
assert_eq!(program.run(&vars).unwrap().as_bool(), Some(true));
```

## Limits

Steps, allocation units, a deadline, and a depth limit bound every run. The
depth limit defaults to `200` and may be set up to `400`:

```rust
use risor::{parse, CompileOptions, Data, Env, Limits};
use std::collections::BTreeMap;
use std::time::Duration;

let env = Env::builder()
    .var("n", "int")
    .limits(Limits {
        steps: Some(10_000),
        allocation: Some(100_000),
        depth: 200,
        deadline: Some(Duration::from_millis(50)),
    })
    .build()
    .unwrap();

let expr = parse("range(n).map(i => i * i)").unwrap();
let program = env.compile(&expr, CompileOptions::default()).unwrap();
let mut vars = BTreeMap::new();
vars.insert("n".into(), Data::Int(1_000_000));

let err = program.run(&vars).unwrap_err();
assert_eq!(err.first().code, "steps-exceeded");
```

A source text has at most `SOURCE_LIMIT` (1,048,576) code points. Calls that
accept source text (`parse_with`, `parse_template_with`, `format_with`,
`Env::complete_with`, and `Env::hover_with`) accept a lower
`ParseOptions::source_limit`, and `load_with` accepts `LoadOptions::source_limit`.

Because evaluation recurses on the calling thread up to `Limits::depth`, see
the `# Stack` section in the crate documentation on
[docs.rs/risor](https://docs.rs/risor) for required thread stack sizes.

## Tools and configuration

| API | Purpose |
| --- | --- |
| `format`, `format_template` | Canonical source and template formatting |
| `Env::complete`, `Env::hover` | Editor completion and hover metadata (types, signatures, `/** ... */` docs) |
| `type_from_schema`, `schema_of` | Convert between JSON Schema and Risor types |
| `Env::describe`, `Env::from_description` | Serialize and restore an environment description as JSON |
| `load`, `load_value`, `load_with` | Evaluate, type-check, conform, and decode Risor config files |
| `parse_template` | Parse `${{ ... }}` templates and inspect or evaluate them |

## Links

- API documentation: https://docs.rs/risor
- Specification and conformance suite: https://github.com/deepnoodle-ai/risor/tree/main/spec
- Go implementation: `github.com/deepnoodle-ai/risor/go/v3`
- TypeScript implementation: `@deepnoodle/risor`
- License: Apache-2.0
