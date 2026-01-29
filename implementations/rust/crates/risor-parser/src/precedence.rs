//! Operator precedence levels for Pratt parsing.

use risor_lexer::TokenKind;

/// Precedence levels (higher = tighter binding).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
#[repr(u8)]
pub enum Precedence {
    Lowest = 1,
    Nullish = 2,     // ??
    Pipe = 3,        // |
    Cond = 4,        // && ||
    Assign = 5,      // =
    Equals = 6,      // == !=
    LessGreater = 7, // > < >= <= in
    Sum = 8,         // + -
    Product = 9,     // * / % & ^ >> <<
    Power = 10,      // ** (right-associative)
    Prefix = 11,     // -X !X
    Call = 12,       // fn()
    Index = 13,      // arr[i] obj.prop
    OptChain = 14,   // ?.
    Highest = 15,
}

impl Precedence {
    /// Get the precedence for a token kind.
    pub fn from_token(kind: TokenKind) -> Self {
        match kind {
            TokenKind::Nullish => Precedence::Nullish,
            TokenKind::Assign => Precedence::Assign,
            TokenKind::Eq | TokenKind::NotEq => Precedence::Equals,
            TokenKind::Lt
            | TokenKind::LtEquals
            | TokenKind::Gt
            | TokenKind::GtEquals
            | TokenKind::In
            | TokenKind::Not => Precedence::LessGreater,
            TokenKind::Plus | TokenKind::PlusEquals | TokenKind::Minus | TokenKind::MinusEquals => {
                Precedence::Sum
            }
            TokenKind::Slash
            | TokenKind::SlashEquals
            | TokenKind::Asterisk
            | TokenKind::AsteriskEquals
            | TokenKind::Mod
            | TokenKind::Ampersand
            | TokenKind::Caret
            | TokenKind::GtGt
            | TokenKind::LtLt => Precedence::Product,
            TokenKind::Pow => Precedence::Power,
            TokenKind::And | TokenKind::Or => Precedence::Cond,
            TokenKind::Pipe => Precedence::Pipe,
            TokenKind::LParen => Precedence::Call,
            TokenKind::Period | TokenKind::LBracket => Precedence::Index,
            TokenKind::QuestionDot => Precedence::OptChain,
            _ => Precedence::Lowest,
        }
    }
}
