# Embedding Risor v3 in Rust

This guide shows how to embed Risor v3 in a Rust application using the `risor` crate (and its optional `derive` feature backed by `risor-derive`).

Every Rust example in this guide is compiled and executed in CI against `./rust` with `--all-features --offline`.

## Contents

1. [Install and quick start](#1-install-and-quick-start)
2. [Environments and types](#2-environments-and-types)
3. [Host functions and opaque values](#3-host-functions-and-opaque-values)
4. [Limits, usage, and cancellation](#4-limits-usage-and-cancellation)
5. [Static analysis and templates](#5-static-analysis-and-templates)
6. [Conformance of host data and config loading](#6-conformance-of-host-data-and-config-loading)
7. [`schema_of`: exporting JSON Schema](#7-schema_of-exporting-json-schema)
8. [Entry points (`export const`)](#8-entry-points-export-const)
9. [The long-lived host pattern](#9-the-long-lived-host-pattern)
10. [Testing expressions and editor tools](#10-testing-expressions-and-editor-tools)

---

## 1. Install and quick start

`risor` uses `#![forbid(unsafe_code)]` and has **no third-party dependencies**. Enabling the `derive` feature adds only this repository's `risor-derive` proc-macro crate (which itself has zero dependencies) and exports `#[derive(risor::Host)]`.

```toml
[dependencies]
risor = { git = "https://github.com/deepnoodle-ai/risor", features = ["derive"] }
```

A Rust host works in four stages ([`spec/core/03-host.md`](../../spec/core/03-host.md) 3.2):

1. **`risor::parse`** turns source text into an immutable `Expr` (no environment needed).
2. **`env.check`** / **`env.check_with`** checks types and returns a `CheckResult` (`ty`, `sites`, `entries`).
3. **`env.compile`** checks (by default) and compiles into an immutable `Program` (`Send + Sync`).
4. **`prog.run`** / **`prog.measure`** borrows your host data for the duration of the run (`&'h dyn Vars`) and returns `Value<'h>`—reading host structs in place without copying.

Inspect results with `Value` accessors (`as_bool`, `as_i64`, `as_f64`, `as_str`, `field`, `elem`), `truthy(&v)`, and `json_text(&v)`. Render failures with `render_line`, `render_detail`, and `render_json`.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::collections::HashMap;
use risor::{
    json_text, parse, render_line, truthy, CompileOptions, Env, Expected, Host,
};

#[derive(Host)]
struct User {
    name: String,
    age: i64,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let env = Env::builder()
        .var_host::<User>("user")
        .var("discount", "int")
        .build()?;

    let expr = parse("user.age >= 18 && discount > 0")?;
    let prog = env.compile(
        &expr,
        CompileOptions {
            expected: Expected::text("bool"),
            ..CompileOptions::default()
        },
    )?;

    // Supply variables through a custom Vars impl or a map.
    struct RunVars<'a> {
        user: &'a User,
        discount: i64,
    }
    impl risor::Vars for RunVars<'_> {
        fn get(&self, name: &str) -> Option<risor::View<'_>> {
            match name {
                "user" => Some(self.user.view()),
                "discount" => Some(risor::View::Int(self.discount)),
                _ => None,
            }
        }
    }

    let ada = User {
        name: "Ada".into(),
        age: 36,
    };
    {
        let vars = RunVars {
            user: &ada,
            discount: 15,
        };
        let out = prog.run(&vars)?;
        assert_eq!(out.as_bool(), Some(true));
        assert!(truthy(&out));
        assert_eq!(json_text(&out)?, "true");
    }

    // HashMap<String, T: Host> also implements Vars directly.
    let users = HashMap::from([("user".to_string(), ada)]);
    let name_prog = env.compile(&parse("user.name")?, CompileOptions::default())?;
    assert_eq!(name_prog.run(&users)?.as_str(), Some("Ada"));

    // Render diagnostics consistently across all stages.
    let err = env.check(&parse("user.nmae")?, &Expected::None).unwrap_err();
    assert_eq!(
        render_line(&err.diagnostics),
        "error[no-field] 1:6-1:10: {age: int, name: string} has no field \"nmae\"\n"
    );
    Ok(())
}
```

> **Thread stack sizing:** A run recurses on its calling thread up to `Limits::depth`. Give the thread at least **1 MiB** (optimized) / **2 MiB** (unoptimized) for the default depth limit of `200`, or **2 MiB** / **4 MiB** for the maximum depth limit of `400` (see [`rust/DESIGN.md`](../../rust/DESIGN.md), verified in CI by `the_deepest_programs_run_within_the_stated_stack`). Standard `std::thread::spawn` threads have 2 MiB by default; use `std::thread::Builder::new().stack_size(...)` when running deeper or on constrained threads.

---

## 2. Environments and types

Build an `Env` with `Env::builder()` and finish with `.build()`. An `Env` is cheap to `.clone()` (`Arc`-backed) and `Send + Sync`.

| Builder method | Input | Notes |
| --- | --- | --- |
| `.var(name, type_text)` | Type text (`"int"`, `"{id: string}[]"`) | Resolved against declared opaque and named types. |
| `.var_record(record_text)` | One record type (`"{a: int, b: string}"`) | Declares one variable per field. |
| `.var_type(name, &ty)` | A `risor::Type` | From structural builders, `type_of`, `type_from_schema`, or `env.parse_type`. |
| `.var_host::<T>(name)` | Any `T: HostType` | Derives the Risor type from a Rust type (including `#[derive(risor::Host)]`). |
| `.var_schema(name, json, opts)` | JSON Schema text + `SchemaOptions` | Supports `closed`, `absent_is_null`, and `defaults_applied`. |

`#[derive(risor::Host)]` supports field attributes:

- `#[risor(rename = "port-number")]` to set the Risor field key
- `#[risor(default)]` to make a field optional when decoding (uses `Default::default()`)
- `#[risor(absent_if_none)]` on `Option<T>` to omit the key when `None` (making the field optional `field?: T` instead of `field: T | null`)
- `#[risor(skip)]` to exclude a field from Risor

### Child environments and portable descriptions

`env.with()` returns an `EnvBuilder` rooted at `env` without mutating the parent. `env.description_text()` produces the canonical JSON description (and `env.describe()` returns it as `Data`); `Env::from_description(&text)` rebuilds a check-only environment from that JSON.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::collections::BTreeMap;
use risor::{
    exact_record_type, literal_type, parse, tagged_union_type, CompileOptions, Data, Env,
    FieldType, SchemaOptions, Type,
};

fn field(key: &str, ty: Type) -> FieldType {
    FieldType {
        key: key.into(),
        ty,
        optional: false,
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Structural type API: build a tagged union without type text.
    let click = exact_record_type(&[
        field("kind", literal_type(&["click"])),
        field("x", Type::INT),
    ]);
    let key = exact_record_type(&[
        field("kind", literal_type(&["key"])),
        field("code", Type::STRING),
    ]);
    let event_type = tagged_union_type(&[click, key]);
    assert!(event_type.err().is_none());
    assert_eq!(
        event_type.to_string(),
        r#"{kind: "click", x: int} | {code: string, kind: "key"}"#
    );

    let cfg_schema = r#"{
        "type": "object",
        "required": ["env"],
        "properties": {"env": {"enum": ["dev", "prod"]}},
        "additionalProperties": false
    }"#;

    let parent = Env::builder()
        .named_type_of("Event", &event_type)
        .var_schema("cfg", cfg_schema, SchemaOptions::default())
        .var("primary", "string | null")
        .var("fallback", "string | null")
        .some_not_null(&["primary", "fallback"])
        .groups(&["json"])
        .without(&["string.repeat"])
        .build()?;

    // Child environment adds `ev` without mutating `parent`.
    let child = parent.with().var("ev", "Event").build()?;

    let prog = child.compile(
        &parse(
            r#"{
                status: match ev.kind {
                    "click" => if (ev.x >= 0) { "ok" } else { "neg" },
                    "key" => ev.code,
                },
                chosen: primary ?? fallback,
            }"#,
        )?,
        CompileOptions::default(),
    )?;

    let ev = Data::Map(BTreeMap::from([
        ("kind".into(), Data::Str("click".into())),
        ("x".into(), Data::Int(12)),
    ]));
    let vars = BTreeMap::from([
        ("ev".to_string(), ev),
        ("primary".to_string(), Data::Null),
        ("fallback".to_string(), Data::Str("backup".into())),
    ]);
    let out = prog.run(&vars)?;
    assert_eq!(out.field("status").unwrap().as_str(), Some("ok"));
    assert_eq!(out.field("chosen").unwrap().as_str(), Some("backup"));

    // Portable environment description round-trip.
    let desc = child.description_text();
    let rebuilt = Env::from_description(&desc)?;
    assert_eq!(rebuilt.description_text(), desc);
    Ok(())
}
```

---

## 3. Host functions and opaque values

Declare host functions on `EnvBuilder` with `.function(name, sig, closure)` (and optional `.function_doc(name, doc)`). Each closure has signature:

```text
Fn(&mut Call<'_>, &[Value<'_>]) -> Result<Data, HostError> + Send + Sync + 'static
```

Key rules ([`spec/core/03-host.md`](../../spec/core/03-host.md) 3.5):

- Mark fallible signatures with `!` (`"(key: string) => string !"`).
- Charge steps and allocation with `call.charge(steps)` and `call.allocate(units)`. Both return `Result<(), Stop>`; on `Err(Stop)`, return immediately so the run ends with the uncatchable limit code.
- Check `call.cancelled()` before blocking or doing heavy work.
- Return `Err(HostError::from("..."))` for a catchable `function-error` (or `?` a `risor::Error` directly—`HostError` implements `From<risor::Error>`). Panicking produces an uncatchable `function-panic`.

### Opaque values

Declare an opaque type with `.opaque("Session")` and attach methods with `.method("Session", "label", "(sep: string) => string", closure)`. Methods receive the receiver as `args[0]`. Wrap host state in `OpaqueRef::new("Session", value)` and downcast it inside methods via `args[0].as_opaque()` and `.value.downcast_ref::<Session>()`.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::collections::BTreeMap;
use std::sync::Arc;
use risor::{
    parse, CompileOptions, Data, Env, HostError, OpaqueRef, Stop,
};

struct Session {
    user: String,
    role: String,
}

fn stop_err(_: Stop) -> HostError {
    HostError::from("budget exceeded")
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let store = Arc::new(BTreeMap::from([("plan".to_string(), "pro".to_string())]));
    let store_ref = Arc::clone(&store);

    let env = Env::builder()
        .opaque("Session")
        .opaque_doc("Session", "An authenticated user session.")
        .method("Session", "label", "(sep: string) => string", |call, args| {
            let sess = args[0]
                .as_opaque()
                .and_then(|o| o.value.downcast_ref::<Session>())
                .ok_or_else(|| HostError::from("expected Session"))?;
            let sep = args[1].as_str().unwrap_or("");
            let out = format!("{}{sep}{}", sess.user, sess.role);
            call.allocate((out.len() as u64) / 64 + 1).map_err(stop_err)?;
            Ok(Data::Str(out))
        })
        .method_doc("Session", "label", "Format user and role separated by sep.")
        .function("store.get", "(key: string) => string !", move |call, args| {
            if call.cancelled() {
                return Err(HostError::from("cancelled"));
            }
            call.charge(1).map_err(stop_err)?;
            let key = args[0].as_str().unwrap_or("");
            let val = store_ref
                .get(key)
                .ok_or_else(|| HostError::from(format!("missing key \"{key}\"")))?;
            call.allocate((val.len() as u64) / 64 + 1).map_err(stop_err)?;
            Ok(Data::Str(val.clone()))
        })
        .function("slug", "(text: string) => string", |call, args| {
            let text = args[0].as_str().unwrap_or("");
            call.charge((text.len() as u64) / 64 + 1).map_err(stop_err)?;
            let out = text.trim().to_lowercase().replace(' ', "-");
            call.allocate((out.len() as u64) / 64 + 1)
                .map_err(stop_err)?;
            Ok(Data::Str(out))
        })
        .var("sess", "Session")
        .build()?;

    let prog = env.compile(
        &parse(
            r#"{
                slug: slug("Hello World"),
                token: sess.label(":"),
                caught: try { store.get("nope") } catch (e) { e.message },
            }"#,
        )?,
        CompileOptions::default(),
    )?;

    let session = OpaqueRef::new(
        "Session",
        Session {
            user: "alice".into(),
            role: "admin".into(),
        },
    );
    let vars = BTreeMap::from([("sess".to_string(), Data::Opaque(session))]);
    let out = prog.run(&vars)?;
    assert_eq!(out.field("slug").unwrap().as_str(), Some("hello-world"));
    assert_eq!(out.field("token").unwrap().as_str(), Some("alice:admin"));
    assert_eq!(
        out.field("caught").unwrap().as_str(),
        Some("missing key \"nope\"")
    );
    Ok(())
}
```

---

## 4. Limits, usage, and cancellation

Configure execution limits on `EnvBuilder::limits(Limits { ... })`:

| Field | Default | Disable | Error code |
| --- | --- | --- | --- |
| `steps` | `Some(1_000_000)` | `None` | `steps-exceeded` |
| `allocation` | `Some(1_000_000)` | `None` | `allocation-exceeded` |
| `deadline` | `None` (off) | `None` | `deadline-exceeded` |
| `depth` | `200` | `1..=400` (cannot disable) | `depth-exceeded` |
| `ParseOptions::source_limit` | `SOURCE_LIMIT` (1,048,576) | Option on `parse_with`, `parse_template_with`, `format_with`, `load_with`, `complete_with`, `hover_with` | `source-too-large` |

Pass `RunOptions { cancel: Some(Arc<AtomicBool>) }` to `prog.run_with` or `prog.measure_with` to cancel a run (`cancelled`). `prog.measure` / `prog.measure_with` return `(Result<Value<'_>, Error>, Usage)` so you always receive usage counts even on failure. Limit errors cannot be caught by `try { ... } catch`.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::sync::atomic::AtomicBool;
use std::sync::Arc;
use std::time::Duration;
use risor::{
    parse, parse_with, CompileOptions, Env, Limits, NoVars, ParseOptions, RunOptions,
};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let env = Env::builder()
        .limits(Limits {
            steps: Some(5_000),
            allocation: Some(2_000),
            deadline: Some(Duration::from_millis(500)),
            depth: 50,
        })
        .build()?;

    // Measure usage on a successful run.
    let prog = env.compile(
        &parse("range(0, 10).reduce((a, b) => a + b, 0)")?,
        CompileOptions::default(),
    )?;
    let (res, usage) = prog.measure(&NoVars);
    assert_eq!(res?.as_i64(), Some(45));
    assert!(usage.steps > 0);

    // Step limit cannot be caught inside Risor.
    let heavy = env.compile(
        &parse("try { len(range(0, 50000)) } catch { 0 }")?,
        CompileOptions::default(),
    )?;
    let (res, _) = heavy.measure(&NoVars);
    assert_eq!(res.unwrap_err().first().code, "steps-exceeded");

    // Cancel via Arc<AtomicBool>.
    let cancel = Arc::new(AtomicBool::new(true));
    let loop_prog = env.compile(
        &parse("len(range(0, 4000).map(n => n + 1))")?,
        CompileOptions::default(),
    )?;
    let err = loop_prog
        .run_with(&NoVars, RunOptions { cancel: Some(cancel) })
        .unwrap_err();
    assert_eq!(err.first().code, "cancelled");

    // Restrict source size at parse time.
    let err = parse_with(
        r#""this source is too long""#,
        ParseOptions { source_limit: 8 },
    )
    .unwrap_err();
    assert_eq!(err.first().code, "source-too-large");
    Ok(())
}
```

---

## 5. Static analysis and templates

`expr.analyze()` works before an environment exists and never fails. It reports:

- `names`: sorted unique free names read.
- `refs`: static paths (`name`, `path: Vec<Step>`, `span`).
- `calls` / `calls_of(&[...])`: call sites with `args: Vec<Option<Expr>>` (each non-lambda argument as its own `Expr` with spans into the full source), `arg_spans`, and `in_lambda`.
- `literal`: `Option<Data>` when the program is a literal.

`parse_template(text)` parses `${{ expr }}` templates. Use `TemplateMode::String` for string output, `TemplateMode::Value` to preserve the expression's value on single-part templates (`is_single()`), and `tmpl.part(i)` for individual parts.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::collections::BTreeMap;
use risor::{
    parse, parse_template, CompileOptions, Data, Env, TemplateMode,
};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let expr = parse("lookup(order.id) && user.active")?;
    let analysis = expr.analyze();
    assert_eq!(analysis.names, ["order", "user"]);
    let lookup_calls = analysis.calls_of(&["lookup"]);
    assert_eq!(
        lookup_calls[0].args[0].as_ref().unwrap().text(),
        "order.id"
    );

    let env = Env::builder()
        .var_record("{user: {name: string, age: int}, items: string[]}")
        .build()?;
    let vars = BTreeMap::from([
        (
            "user".to_string(),
            Data::Map(BTreeMap::from([
                ("name".into(), Data::Str("Ada".into())),
                ("age".into(), Data::Int(36)),
            ])),
        ),
        (
            "items".to_string(),
            Data::List(vec![Data::Str("a".into()), Data::Str("b".into())]),
        ),
    ]);

    // TemplateMode::String produces a string.
    let msg_tmpl = parse_template("Hi ${{ user.name }}, ${{ len(items) }} items")?;
    let msg_prog = env.compile(&msg_tmpl.program(TemplateMode::String), CompileOptions::default())?;
    assert_eq!(msg_prog.run(&vars)?.as_str(), Some("Hi Ada, 2 items"));

    // TemplateMode::Value on a single-part template preserves the int result.
    let val_tmpl = parse_template("${{ user.age }}")?;
    assert!(val_tmpl.is_single());
    let val_prog = env.compile(&val_tmpl.program(TemplateMode::Value), CompileOptions::default())?;
    assert_eq!(val_prog.run(&vars)?.as_i64(), Some(36));
    Ok(())
}
```

---

## 6. Conformance of host data and config loading

Validate host values against any `Type` with `conform(&v, &t)` (or `conform_with` for a custom `ConformOptions { steps }`) and `conform_all(&v, &t, max)`.

### Config files (`load`, `load_with`, `load_value`, `decode`)

With `#[derive(risor::Host)]`, `risor::load::<T>(&env, src, &vars, max)` runs all six config-loading steps in order—declare (`STEP_DECLARE = 0`), parse (`1`), check (`2`), run (`3`), conform (`4`), and decode into `T` (`5`)—returning `Result<T, LoadError>`. Use `load_with` to pass `LoadOptions` (such as `source_limit` or `RunOptions { cancel }`), `load_value` to get owned `Data` without decoding into a Rust struct, and `risor::decode::<T>(&val)` to decode any `Value`.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::collections::BTreeMap;
use risor::{
    conform_all, decode, load, Env, Host, Data, Value, STEP_DECODE,
};

#[derive(Host, Debug, PartialEq)]
struct ServerConfig {
    host: String,
    port: u16,
    #[risor(default)]
    retries: i64,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let env = Env::builder().var("prod", "bool").build()?;

    // Validate untrusted host data with conform_all.
    let t = env.parse_type("{port: int, tls: bool}")?;
    let bad_data = Data::Map(BTreeMap::from([
        ("port".into(), Data::Str("bad".into())),
        ("tls".into(), Data::Int(123)),
    ]));
    let bad_val = Value::from_data(&bad_data);
    let diags = conform_all(&bad_val, &t, 10);
    assert_eq!(diags.len(), 2);

    // Load a config program directly into a Rust struct.
    let src = r#"{
        host: if (prod) { "0.0.0.0" } else { "127.0.0.1" },
        port: if (prod) { 8443 } else { 8080 },
    }"#;
    let vars = BTreeMap::from([("prod".to_string(), Data::Bool(true))]);
    let cfg: ServerConfig = load(&env, src, &vars, 5)?;
    assert_eq!(
        cfg,
        ServerConfig {
            host: "0.0.0.0".into(),
            port: 8443,
            retries: 0,
        }
    );

    // Standalone decode into a Rust type.
    let list_data = Data::List(vec![Data::Int(1), Data::Int(2)]);
    assert_eq!(decode::<Vec<u8>>(&Value::from_data(&list_data))?, vec![1, 2]);

    // Port overflow fails at STEP_DECODE (step 5).
    let err = load::<ServerConfig>(
        &env,
        r#"{host: "localhost", port: 70000}"#,
        &risor::NoVars,
        5,
    )
    .unwrap_err();
    assert_eq!(err.step, STEP_DECODE);
    Ok(())
}
```

---

## 7. `schema_of`: exporting JSON Schema

`risor::schema_of(&ty)` returns the canonical JSON Schema string for any non-opaque `Type` (keys in code point order, no whitespace, field doc comments as `"description"`, recursive types under `"$defs"`). Combine it with `#[derive(risor::Host)]` and `risor::type_of::<T>()` to export JSON Schema directly from your Rust structs, or from an entry point's `input_type`.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use risor::{
    opaque_type_named, parse, schema_of, type_from_schema, type_of, Env, Expected, Host,
    SchemaOptions,
};

#[derive(Host)]
struct WeatherRequest {
    city: String,
    #[risor(absent_if_none)]
    units: Option<String>,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 1. Derive JSON Schema from a Rust struct via #[derive(Host)] + type_of + schema_of.
    let struct_ty = type_of::<WeatherRequest>()?;
    let struct_schema = schema_of(&struct_ty)?;
    assert_eq!(
        struct_schema,
        r#"{"additionalProperties":false,"properties":{"city":{"type":"string"},"units":{"type":"string"}},"required":["city"],"type":"object"}"#
    );

    // 2. Field doc comments in Risor type text become "description" in JSON Schema.
    let env = Env::builder()
        .opaque("Token")
        .var("req", r#"{/** Destination city. */ city: string, units?: "C" | "F"}"#)
        .build()?;
    let checked = env.check(&parse("req")?, &Expected::None)?;
    let schema = schema_of(&checked.ty)?;
    assert_eq!(
        schema,
        r#"{"additionalProperties":false,"properties":{"city":{"description":"Destination city.","type":"string"},"units":{"enum":["C","F"],"type":"string"}},"required":["city"],"type":"object"}"#
    );

    // Round-trip back to an equal Type.
    let back = type_from_schema(&schema, SchemaOptions::default())?;
    assert!(back.equal(&checked.ty));

    // Opaque types have no JSON Schema.
    let err = schema_of(&opaque_type_named("Token")).unwrap_err();
    assert_eq!(err.first().code, "bad-type");
    Ok(())
}
```

---

## 8. Entry points (`export const`)

Export typed functions from a Risor program using `export const`, with or without a final result expression:

- **`prog.entries()`** (and `cr.entries`) returns `&[Entry]` in source order (`name`, `doc`, `params`, `input_type`, `result_type`).
- **`schema_of(entry.input_type.as_ref().unwrap())`** turns an entry point's parameters (and doc comments) into a JSON Schema.
- **`CompileOptions::expect_entry` / `expect_entry_type` / `expect_entry_schema`** check individual entry return types.
- **`prog.call` / `call_with` / `call_measure`** invoke positionally with `&[&dyn Host]`; **`prog.call_record` / `call_record_with` / `call_record_measure`** invoke with one input record (`&dyn Host`) conformed against `input_type`.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::collections::BTreeMap;
use risor::{parse, schema_of, CompileOptions, Data, Env};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let env = Env::builder().var("taxRate", "float").build()?;

    let src = r#"
const multiplier = 1 + taxRate;

/** Calculate an order total. */
export const quote = (
  /** Subtotal in USD. */
  subtotal: float,
  /** Optional shipping fee. */
  shipping: float = 0.0,
): float => subtotal * multiplier + shipping;
"#;

    let opts = CompileOptions::default().expect_entry("quote", "float");
    let prog = env.compile(&parse(src)?, opts)?;

    assert!(!prog.has_result());
    let entry = &prog.entries()[0];
    assert_eq!(entry.name, "quote");
    assert_eq!(entry.doc.as_deref(), Some("Calculate an order total."));

    let vars = BTreeMap::from([("taxRate".to_string(), Data::Float(0.2))]);

    // Positional call (using the default shipping = 0.0).
    let v1 = prog.call("quote", &[&100.0f64], &vars)?;
    assert_eq!(v1.as_f64(), Some(120.0));

    // Call by record.
    let input = Data::Map(BTreeMap::from([
        ("subtotal".into(), Data::Float(100.0)),
        ("shipping".into(), Data::Float(15.0)),
    ]));
    let v2 = prog.call_record("quote", &input, &vars)?;
    assert_eq!(v2.as_f64(), Some(135.0));

    // Export the entry point's input schema with parameter descriptions.
    let input_schema = schema_of(entry.input_type.as_ref().unwrap())?;
    assert!(input_schema.contains(r#""description":"Subtotal in USD.""#));
    Ok(())
}
```

---

## 9. The long-lived host pattern

Servers, workflow engines, and UI event loops follow [`spec/core/03-host.md`](../../spec/core/03-host.md) 3.2.5:

1. **Build one `Env` at startup** with your domain types, host functions, and limits.
2. **Parse, check, and compile once** when rules or handlers are loaded.
3. **Share `&Program` (or `Arc<Program>`) across threads**—`Program` is `Send + Sync`, and each run gets its own variables, budget, and `RunOptions { cancel }`.
4. **The host owns all state.** Each event runs with `{ state, ev }` and returns data (next state + tagged-union actions) that the host applies in Rust.

### Poisoned runs and nested runs

- **Poisoned run:** Keep a per-run `Arc<AtomicBool>` (`poisoned`) alongside your cancellation flag. When a host function must cut off the run, set both flags, immediately return `Err(HostError::from(...))` from any subsequent host calls while poisoned, and discard the run's outcome if `poisoned` is set.
- **Nested run:** Inside a host function that runs another program, build a child env with `Limits { steps: left.steps, allocation: left.allocation, ..tool_env.limits() }` from `call.remaining()`, pass the parent's host-owned `Arc<AtomicBool>` cancellation flag through `measure_with(&vars, RunOptions { cancel: Some(...) })`, charge back `used.steps` and `used.allocated`, and on `steps-exceeded` or `allocation-exceeded` overcharge (`call.charge(u64::MAX)` / `call.allocate(u64::MAX)`) so the caller ends with the same uncatchable limit code.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use std::collections::BTreeMap;
use std::sync::{
    atomic::AtomicBool,
    Arc,
};
use risor::{
    parse, CompileOptions, Data, Env, Expected, HostError, Limits, NoVars, RunOptions, Stop,
};

fn stop_err(_: Stop) -> HostError {
    HostError::from("budget exceeded")
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 1. Build env once, compile once, share &Program across concurrent threads.
    let handler_env = Env::builder()
        .var("state", "{count: int}")
        .var("ev", r#"{kind: "inc", by: int} | {kind: "reset"}"#)
        .limits(Limits {
            steps: Some(10_000),
            allocation: Some(10_000),
            ..Limits::default()
        })
        .build()?;

    let handler_prog = handler_env.compile(
        &parse(
            r#"match ev.kind {
                "inc" => {count: state.count + ev.by},
                "reset" => {count: 0},
            }"#,
        )?,
        CompileOptions {
            expected: Expected::text("{count: int}"),
            ..CompileOptions::default()
        },
    )?;

    let counts: Vec<i64> = std::thread::scope(|s| {
        let handles: Vec<_> = (1..=4i64)
            .map(|i| {
                let prog = &handler_prog;
                s.spawn(move || {
                    let vars = BTreeMap::from([
                        (
                            "state".to_string(),
                            Data::Map(BTreeMap::from([("count".into(), Data::Int(i))])),
                        ),
                        (
                            "ev".to_string(),
                            Data::Map(BTreeMap::from([
                                ("kind".into(), Data::Str("inc".into())),
                                ("by".into(), Data::Int(i)),
                            ])),
                        ),
                    ]);
                    let count = prog
                        .run(&vars)
                        .unwrap()
                        .field("count")
                        .unwrap()
                        .as_i64()
                        .unwrap();
                    count
                })
            })
            .collect();
        handles.into_iter().map(|h| h.join().unwrap()).collect()
    });
    assert_eq!(counts, [2, 4, 6, 8]);

    // 2. Nested run sharing the caller's remaining budget and cancellation flag.
    let tool_env = Env::builder().var("xs", "int[]").build()?;
    let tool_expr = parse("xs.map(n => n * 2).reduce((a, b) => a + b, 0)")?;
    let cancel = Arc::new(AtomicBool::new(false));
    let child_cancel = Arc::clone(&cancel);

    let parent_env = Env::builder()
        .limits(Limits {
            steps: Some(500),
            allocation: Some(500),
            ..Limits::default()
        })
        .function("runTool", "(xs: int[]) => int !", move |call, args| {
            let left = call.remaining();
            let child = tool_env
                .with()
                .limits(Limits {
                    steps: left.steps,
                    allocation: left.allocation,
                    ..tool_env.limits()
                })
                .build()?;
            let p = child.compile(&tool_expr, CompileOptions::default())?;
            let mut vars = BTreeMap::new();
            vars.insert("xs".to_string(), args[0].to_data());
            let (result, used) = p.measure_with(
                &vars,
                RunOptions {
                    cancel: Some(Arc::clone(&child_cancel)),
                },
            );
            call.charge(used.steps).map_err(stop_err)?;
            call.allocate(used.allocated).map_err(stop_err)?;
            match result {
                Ok(v) => Ok(v.to_data()),
                Err(e) => match e.first().code {
                    "steps-exceeded" => Err(call.charge(u64::MAX).map_err(stop_err).unwrap_err()),
                    "allocation-exceeded" => {
                        Err(call.allocate(u64::MAX).map_err(stop_err).unwrap_err())
                    }
                    _ => Err(e.into()),
                },
            }
        })
        .build()?;

    let caller_expr = parse("runTool([1, 2, 3])")?;
    let caller_prog = parent_env.compile(&caller_expr, CompileOptions::default())?;
    let (res, usage) = caller_prog.measure_with(
        &NoVars,
        RunOptions {
            cancel: Some(Arc::clone(&cancel)),
        },
    );
    assert_eq!(res?.as_i64(), Some(12));
    assert!(usage.steps > 0);

    // Tight budget causes nested run to stop caller with steps-exceeded.
    let tight_env = parent_env
        .with()
        .limits(Limits {
            steps: Some(5),
            ..parent_env.limits()
        })
        .build()?;
    let err = tight_env
        .compile(&caller_expr, CompileOptions::default())?
        .run(&NoVars)
        .unwrap_err();
    assert_eq!(err.first().code, "steps-exceeded");
    Ok(())
}
```

---

## 10. Testing expressions and editor tools

### Case-file testing with `risor::testing`

Use `risor::testing::load("tests/discount.test.json", &env)` (or `testing::from_text` for inline fixtures) to run Risor case files inside `cargo test` against your application's `Env`. Call `report.assert_ok()` to fail the test with a formatted diff report on any failure. Set `RISOR_UPDATE=1` to update JSON case files in place.

### Formatting, completion, and hover

Use `risor::format` / `risor::format_template` for canonical formatting and `env.complete` / `env.hover` for editor assistance on incomplete input.

<!-- risor-verify: {"mode": "rust"} -->
```rust
use risor::{format, testing, Env, Expected, NoVars, Pos};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Run an inline case file against the host's environment.
    let rule_env = Env::builder()
        .var("vip", "bool")
        .var("total", "float")
        .build()?;
    let cases_json = r#"{
        "env": "{vip: bool, total: float}",
        "cases": [
            {
                "name": "vip gets 20 percent",
                "source": "if (vip) { total * 0.8 } else { total }",
                "vars": {"vip": true, "total": 100},
                "expect": {"value": 80}
            },
            {
                "name": "regular pays full price",
                "source": "if (vip) { total * 0.8 } else { total }",
                "vars": {"vip": false, "total": 100},
                "expect": {"value": 100}
            }
        ]
    }"#;
    let file = testing::from_text(cases_json, false, &rule_env)?;
    let report = file.run(&NoVars);
    report.assert_ok();
    assert_eq!(report.verdicts().len(), 2);

    // Formatting, completion, and hover.
    assert_eq!(format("1+2")?, "1 + 2\n");

    let editor_env = Env::builder()
        .var_record("{user: {name: string, age: int}}")
        .build()?;
    let comp = editor_env.complete("user.na", Pos { line: 1, column: 8 }, &Expected::None);
    assert_eq!(comp.items.len(), 1);
    assert_eq!(comp.items[0].label, "name");

    let hover = editor_env
        .hover("user.name", Pos { line: 1, column: 7 }, &Expected::None)
        .unwrap();
    assert_eq!(hover.ty.as_deref(), Some("string"));
    Ok(())
}
```

---

## Further reading

- [Risor v3 Language Guide](../guide.md)
- [Host specification (`spec/core/03-host.md`)](../../spec/core/03-host.md)
- [Go Host Guide](go.md)
- [TypeScript Host Guide](typescript.md)
