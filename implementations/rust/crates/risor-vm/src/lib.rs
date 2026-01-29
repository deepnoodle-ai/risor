//! Risor VM - bytecode execution for the Risor scripting language.
//!
//! This crate provides the virtual machine for Risor, which executes bytecode.

pub mod builtins;
pub mod frame;
pub mod object;
pub mod vm;

pub use builtins::create_builtins;
pub use frame::{ExceptionHandler, Frame};
pub use object::{Builtin, BuiltinContext, BuiltinFn, Closure, Object, RisorIter, RisorMap};
pub use vm::{VMConfig, VMError, VM};
