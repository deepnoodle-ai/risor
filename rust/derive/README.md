# risor-derive

Procedural derive macro (`#[derive(Host)]`) for exposing Rust structs to
[Risor v3](https://crates.io/crates/risor) with zero external dependencies.

Most users should enable the `derive` feature on `risor` rather than depending
on `risor-derive` directly:

```toml
[dependencies]
risor = { version = "3.0.0-alpha.2", features = ["derive"] }
```

## What `#[derive(Host)]` generates

For a struct with named fields, `#[derive(Host)]` implements:

- `risor::HostType` — builds the exact record type of the struct's presented
  fields.
- `risor::Host` and `risor::HostMap` — presents the struct's fields in place
  without copying.
- `risor::Decode` — decodes a Risor value back into the struct (for owned
  structs that borrow no references).

### Field attributes

| Attribute | Effect |
| --- | --- |
| *(none)* | Required field under the Rust field name (`r#type` becomes `type`) |
| `#[risor(rename = "key")]` | Present and decode the field under `"key"` |
| `#[risor(skip)]` | Omit the field from the Risor record type and presentation |
| `#[risor(absent_if_none)]` | On `Option<T>`: optional field `f?: T`, absent when `None` |
| `#[risor(default)]` | Optional field `f?: T`, always presented; decodes via `Default::default()` when absent |

## Example

```rust
use risor::Host;

#[derive(Host)]
struct Order {
    id: u64,
    #[risor(rename = "customer-name")]
    customer: String,
    #[risor(absent_if_none)]
    note: Option<String>,
    #[risor(skip)]
    cache: Vec<u8>,
}

assert_eq!(
    risor::type_of::<Order>().unwrap().to_string(),
    r#"{"customer-name": string, id: int, note?: string}"#
);
```

## Links

- API documentation: https://docs.rs/risor-derive
- Main crate: https://crates.io/crates/risor
- Repository: https://github.com/deepnoodle-ai/risor
- License: Apache-2.0
