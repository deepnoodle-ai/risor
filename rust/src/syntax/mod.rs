//! Text to tree: tokens, the lexer, the syntax tree and the parser
//! (`01-language.md` sections 2 to 4, 13 and 14).

pub mod ast;
pub mod lexer;
pub mod parser;
pub mod token;

pub use ast::{Ast, Node, NodeId, NodeKind};
pub use parser::{parse, Parsed, MAX_LEVELS};
