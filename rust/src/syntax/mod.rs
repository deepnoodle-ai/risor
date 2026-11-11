//! Text to tree: tokens, the lexer, the syntax tree and the parser
//! (`01-language.md` sections 2 to 4, 13 and 14).

pub mod ast;
pub mod lexer;
pub(crate) mod message;
pub mod parser;
pub mod token;

#[cfg(test)]
pub(crate) use parser::parse_core;
pub use parser::{parse, Expr};
