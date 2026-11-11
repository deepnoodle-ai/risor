//! Risor v3, written from its specification (`spec/core/`) and its
//! conformance suite (`suite/`) alone.
//!
//! The crate has no dependencies and no `unsafe`. Its root exports the
//! public surface of `05-host-api.md` Part E (E.3 to E.8, with the options,
//! field types and result records of E.2 item 5) and the conveniences of
//! E.1 row 5, and nothing else; the case-file runner is [`testing`]. Its layout is in `DESIGN.md` beside
//! `Cargo.toml`.
//!
//! ```
//! use risor::{parse, CompileOptions, Env, NoVars};
//! let env = Env::builder().var("n", "int").build().unwrap();
//! let x = parse("n + 1").unwrap();
//! let p = env.compile(&x, CompileOptions::default()).unwrap();
//! assert_eq!(p.check_result().unwrap().ty.to_string(), "int");
//! let mut vars = std::collections::BTreeMap::new();
//! vars.insert("n".to_string(), risor::Data::Int(41));
//! assert_eq!(risor::json_text(&p.run(&vars).unwrap()).unwrap(), "42");
//! # let _ = NoVars;
//! ```

#![forbid(unsafe_code)]

/// The packages of the specification this build offers (`03-host.md` 3.11),
/// in the order they are offered. This is the one list: the runner reads it
/// to choose the `until-fNN-` files it does not run.
pub(crate) const PACKAGES: [u32; 14] = [3, 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 13, 14, 12];

mod analysis;
mod api;
mod assist;
mod casemap;
mod check;
mod compile;
mod config;
mod conform;
mod decode;
mod describe;
mod diag;
mod env;
mod eval;
mod format;
mod host;
mod hosttype;
mod json;
mod library;
mod math;
mod messages;
mod number;
mod render;
mod schema;
mod source;
mod syntax;
mod template;
pub mod testing;
mod typ;
mod types;
mod value;

#[cfg(test)]
mod tests;

// E.3: the environment.
pub use env::{Env, EnvBuilder, LibraryNames, Limits};

// E.4: programs.
pub use analysis::{Analysis, CallSite, Ref, Step};
pub use api::{CheckResult, CompileOptions, Expected, Site};
pub use compile::Program;
pub use eval::{RunOptions, Usage};
pub use host::{NoVars, Vars};
pub use syntax::{parse, Expr};

// E.5: templates.
pub use format::format_template;
pub use template::{parse_template, Segment, Template, TemplateMode};

// E.6: host functions and values.
pub use api::{json_text, truthy};
pub use eval::{Call, Stop};
pub use host::{Data, HostError, OpaqueRef};
pub use value::Value;

// E.7: types, conformance and config files.
pub use config::{load, load_value, LoadError, LoadOptions};
pub use decode::{decode, Decode, DecodeCx};
pub use schema::SchemaOptions;
pub use typ::{
    conform, conform_all, exact_record_type, join, list_type, literal_type, map_type, nullable,
    opaque_type_named, record_type, tagged_union_type, type_from_schema, union_type, FieldType,
    Type, TypeKind,
};

// E.1 row 5: the conveniences for the host's own types.
pub use host::{Host, HostList, HostMap, View};
pub use hosttype::{
    type_of, HostType, Opaque, OpaqueType, OptionLike, RecordType, TypeCx, TypeError,
};
/// `#[derive(risor::Host)]`: the type and the presentation of a struct (F1
/// 6.3), with the feature `derive`.
#[cfg(feature = "derive")]
pub use risor_derive::Host;

// E.8: diagnostics and tools.
pub use assist::{Candidate, Completion, Hover};
pub use diag::{Diagnostic, Error, PathStep, Phase, TraceEntry};
pub use format::format;
pub use render::{render_detail, render_json, render_line};
pub use source::{Pos, Span};

/// What the derive of `risor-derive` and the crate's own examples reach
/// that is not part of the surface. Not for hosts: it may change at any
/// time.
#[doc(hidden)]
pub mod __private {
    pub use crate::json::{parse as parse_json, Json};
    pub use crate::math::eval as math_eval;
    pub use crate::value::{eq_data, Kind};
}
