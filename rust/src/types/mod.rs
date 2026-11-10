//! Types (`05-types.md`): the parser of type text. The type representation
//! and the checker are in `check`.

pub mod text;

pub use text::{parse_method, parse_sig, parse_type, Sig, Type, TypeTextError};
