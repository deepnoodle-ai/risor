//! Risor v3, written from its specification (`spec/core/`) and its
//! conformance suite (`suite/`) alone.
//!
//! The crate has no dependencies and no `unsafe`. Its layout, and the plan
//! for values that the evaluator builds on, are in `DESIGN.md` beside
//! `Cargo.toml`.
//!
//! | Module | Holds |
//! | --- | --- |
//! | [`source`] | Text as code points; offsets, positions and spans |
//! | [`diag`] | Diagnostics, phases and codes |
//! | [`syntax`] | Tokens, the lexer, the syntax tree and the parser |
//! | [`types`] | Type text (and, later, the checker) |
//! | [`json`] | A strict JSON reader for case and data files |
//! | [`schema`] | F1: the type of a JSON Schema |
//! | [`describe`] | F1: descriptions of types and environments |
//! | [`hosttype`] | F1: types from host types, and their presentation |
//! | [`template`] | F2: host templates with `${{ }}` parts |
//! | [`analysis`] | F2: the names, paths and calls a program reads |
//! | [`testing`] | The case-file runner |

#![forbid(unsafe_code)]

/// The packages of the specification this build offers (`03-host.md` 3.11),
/// in the order they are offered. This is the one list: the runner reads it
/// to choose the `until-fNN-` files it does not run.
pub const PACKAGES: [u32; 7] = [3, 1, 2, 4, 5, 6, 7];

pub mod analysis;
pub mod casemap;
pub mod check;
pub mod compile;
pub mod conform;
pub mod describe;
pub mod diag;
pub mod env;
pub mod eval;
pub mod host;
pub mod hosttype;
pub mod json;
pub mod library;
pub mod messages;
pub mod number;
pub mod schema;
pub mod source;
pub mod syntax;
pub mod template;
pub mod testing;
pub mod types;
pub mod value;

pub use analysis::{analyze, analyze_template, CallRef, Ref, Report, Step};
pub use check::{check, CheckResult, Expected};
pub use compile::{compile, compile_unchecked, Program};
pub use describe::{describe_type, print_sig, DescriptionError};
pub use diag::{Diagnostic, Error, Phase};
pub use env::{Env, EnvBuilder, Limits};
pub use eval::{Call, Fail, Outcome, RunOptions, Usage};
pub use host::{Data, Host, HostError, HostList, HostMap, NoVars, OpaqueRef, Vars, View};
pub use hosttype::{
    describe_host_type, type_of, HostType, Opaque, OpaqueType, OptionLike, RecordType, TypeCx,
    TypeError,
};
pub use schema::{ty_schema, ty_schema_text, SchemaOptions};

/// `#[derive(risor::Host)]`: the type and the presentation of a struct (F1
/// 6.3), with the feature `derive`.
#[cfg(feature = "derive")]
pub use risor_derive::Host;
pub use source::{LineSpan, Source, Span};
pub use syntax::{parse, Parsed};
pub use template::{parse_template, Mode, Segment, Template};
pub use value::Value;
