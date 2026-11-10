//! Types (`05-types.md`). In S1 this is the parser of type text. The type
//! representation and the checker come later (milestone M8).

pub mod text;

pub use text::{parse_method, parse_sig, parse_type, Sig, Type, TypeTextError};
