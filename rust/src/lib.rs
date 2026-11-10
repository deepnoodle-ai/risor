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
//! | [`testing`] | The case-file runner |

#![forbid(unsafe_code)]

pub mod diag;
pub mod json;
pub mod source;
pub mod syntax;
pub mod testing;
pub mod types;

pub use diag::{Diagnostic, Error, Phase};
pub use source::{LineSpan, Source, Span};
pub use syntax::{parse, Parsed};
