//! Risor parser - AST construction for the Risor scripting language.
//!
//! This crate provides the parser for Risor, which converts tokens into an AST.

pub mod ast;
pub mod parser;
pub mod precedence;

pub use ast::*;
pub use parser::{parse, Parser, ParserError};
pub use precedence::Precedence;
