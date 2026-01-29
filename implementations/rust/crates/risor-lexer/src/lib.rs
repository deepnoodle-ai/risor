//! Risor lexer - tokenization for the Risor scripting language.
//!
//! This crate provides the lexer for Risor, which converts source code into tokens
//! for parsing.
//!
//! # Example
//!
//! ```
//! use risor_lexer::{Lexer, TokenKind};
//!
//! let mut lexer = Lexer::new("let x = 42");
//! let token = lexer.next_token().unwrap();
//! assert_eq!(token.kind, TokenKind::Let);
//! ```

pub mod lexer;
pub mod token;

pub use lexer::{tokenize, Lexer, LexerError, LexerState};
pub use token::{lookup_identifier, Position, Token, TokenKind};
