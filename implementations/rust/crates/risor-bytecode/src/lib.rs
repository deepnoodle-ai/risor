//! Risor bytecode - opcode definitions for the Risor VM.
//!
//! This crate provides bytecode definitions for the Risor virtual machine.

pub mod code;
pub mod opcode;

pub use code::{Code, CodeBuilder, Constant, ExceptionHandler, SourceLocation};
pub use opcode::{BinaryOpType, CompareOpType, Op};
