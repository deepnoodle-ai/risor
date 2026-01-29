//! Risor compiler - AST to bytecode compilation.
//!
//! This crate provides the compiler for Risor, which converts AST into bytecode.

pub mod compiler;
pub mod symbol_table;

pub use compiler::{compile, Compiler, CompilerConfig, CompilerError};
pub use symbol_table::{Resolution, Scope, ScopeManager, Symbol, SymbolTable};
