//! Token types for the Risor lexer.

use std::fmt;

/// Token kinds for the Risor language.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(u8)]
pub enum TokenKind {
    // Literals
    Int,
    Float,
    String,
    Template,
    Ident,

    // Operators
    Plus,
    Minus,
    Asterisk,
    Slash,
    Mod,
    Pow,

    // Comparison
    Eq,
    NotEq,
    Lt,
    Gt,
    LtEquals,
    GtEquals,

    // Logical
    And,
    Or,
    Bang,

    // Bitwise
    Ampersand,
    Pipe,
    Caret,
    LtLt,
    GtGt,

    // Assignment
    Assign,
    PlusEquals,
    MinusEquals,
    AsteriskEquals,
    SlashEquals,

    // Increment/Decrement
    PlusPlus,
    MinusMinus,

    // Punctuation
    LParen,
    RParen,
    LBracket,
    RBracket,
    LBrace,
    RBrace,
    Comma,
    Semicolon,
    Colon,
    Period,
    Spread,
    Arrow,
    Question,
    QuestionDot,
    Nullish,
    Backtick,

    // Keywords
    Let,
    Const,
    Function,
    Return,
    If,
    Else,
    Switch,
    Case,
    Default,
    Match,
    True,
    False,
    Nil,
    Not,
    In,
    Struct,
    Try,
    Catch,
    Finally,
    Throw,

    // Special
    Newline,
    Eof,
    Illegal,
}

impl fmt::Display for TokenKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let s = match self {
            TokenKind::Int => "INT",
            TokenKind::Float => "FLOAT",
            TokenKind::String => "STRING",
            TokenKind::Template => "TEMPLATE",
            TokenKind::Ident => "IDENT",
            TokenKind::Plus => "+",
            TokenKind::Minus => "-",
            TokenKind::Asterisk => "*",
            TokenKind::Slash => "/",
            TokenKind::Mod => "%",
            TokenKind::Pow => "**",
            TokenKind::Eq => "==",
            TokenKind::NotEq => "!=",
            TokenKind::Lt => "<",
            TokenKind::Gt => ">",
            TokenKind::LtEquals => "<=",
            TokenKind::GtEquals => ">=",
            TokenKind::And => "&&",
            TokenKind::Or => "||",
            TokenKind::Bang => "!",
            TokenKind::Ampersand => "&",
            TokenKind::Pipe => "|",
            TokenKind::Caret => "^",
            TokenKind::LtLt => "<<",
            TokenKind::GtGt => ">>",
            TokenKind::Assign => "=",
            TokenKind::PlusEquals => "+=",
            TokenKind::MinusEquals => "-=",
            TokenKind::AsteriskEquals => "*=",
            TokenKind::SlashEquals => "/=",
            TokenKind::PlusPlus => "++",
            TokenKind::MinusMinus => "--",
            TokenKind::LParen => "(",
            TokenKind::RParen => ")",
            TokenKind::LBracket => "[",
            TokenKind::RBracket => "]",
            TokenKind::LBrace => "{",
            TokenKind::RBrace => "}",
            TokenKind::Comma => ",",
            TokenKind::Semicolon => ";",
            TokenKind::Colon => ":",
            TokenKind::Period => ".",
            TokenKind::Spread => "...",
            TokenKind::Arrow => "=>",
            TokenKind::Question => "?",
            TokenKind::QuestionDot => "?.",
            TokenKind::Nullish => "??",
            TokenKind::Backtick => "`",
            TokenKind::Let => "let",
            TokenKind::Const => "const",
            TokenKind::Function => "function",
            TokenKind::Return => "return",
            TokenKind::If => "if",
            TokenKind::Else => "else",
            TokenKind::Switch => "switch",
            TokenKind::Case => "case",
            TokenKind::Default => "default",
            TokenKind::Match => "match",
            TokenKind::True => "true",
            TokenKind::False => "false",
            TokenKind::Nil => "nil",
            TokenKind::Not => "not",
            TokenKind::In => "in",
            TokenKind::Struct => "struct",
            TokenKind::Try => "try",
            TokenKind::Catch => "catch",
            TokenKind::Finally => "finally",
            TokenKind::Throw => "throw",
            TokenKind::Newline => "NEWLINE",
            TokenKind::Eof => "EOF",
            TokenKind::Illegal => "ILLEGAL",
        };
        write!(f, "{}", s)
    }
}

/// Look up an identifier to see if it's a keyword.
pub fn lookup_identifier(ident: &str) -> TokenKind {
    match ident {
        "let" => TokenKind::Let,
        "const" => TokenKind::Const,
        "function" => TokenKind::Function,
        "return" => TokenKind::Return,
        "if" => TokenKind::If,
        "else" => TokenKind::Else,
        "switch" => TokenKind::Switch,
        "case" => TokenKind::Case,
        "default" => TokenKind::Default,
        "match" => TokenKind::Match,
        "true" => TokenKind::True,
        "false" => TokenKind::False,
        "nil" => TokenKind::Nil,
        "not" => TokenKind::Not,
        "in" => TokenKind::In,
        "struct" => TokenKind::Struct,
        "try" => TokenKind::Try,
        "catch" => TokenKind::Catch,
        "finally" => TokenKind::Finally,
        "throw" => TokenKind::Throw,
        _ => TokenKind::Ident,
    }
}

/// Position in source code.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct Position {
    /// Byte offset within the file.
    pub char: usize,
    /// Byte offset of the start of the current line.
    pub line_start: usize,
    /// 0-indexed line number.
    pub line: usize,
    /// 0-indexed column number.
    pub column: usize,
}

impl Position {
    /// Create a new Position.
    pub fn new(char: usize, line_start: usize, line: usize, column: usize) -> Self {
        Self {
            char,
            line_start,
            line,
            column,
        }
    }

    /// Returns the 1-indexed line number.
    pub fn line_number(&self) -> usize {
        self.line + 1
    }

    /// Returns the 1-indexed column number.
    pub fn column_number(&self) -> usize {
        self.column + 1
    }

    /// Advance this position by n characters.
    pub fn advance(&self, n: usize) -> Self {
        Self {
            char: self.char + n,
            line_start: self.line_start,
            line: self.line,
            column: self.column + n,
        }
    }

    /// Check if this position is valid.
    pub fn is_valid(&self) -> bool {
        self.line > 0 || self.column > 0 || self.char > 0
    }
}

/// A token produced by the lexer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Token {
    /// The kind of token.
    pub kind: TokenKind,
    /// The literal string value.
    pub literal: String,
    /// Start position in source.
    pub start: Position,
    /// End position in source.
    pub end: Position,
}

impl Token {
    /// Create a new Token.
    pub fn new(kind: TokenKind, literal: String, start: Position, end: Position) -> Self {
        Self {
            kind,
            literal,
            start,
            end,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_lookup_identifier() {
        assert_eq!(lookup_identifier("let"), TokenKind::Let);
        assert_eq!(lookup_identifier("const"), TokenKind::Const);
        assert_eq!(lookup_identifier("foo"), TokenKind::Ident);
        assert_eq!(lookup_identifier("bar"), TokenKind::Ident);
    }

    #[test]
    fn test_position() {
        let pos = Position::new(10, 5, 1, 5);
        assert_eq!(pos.line_number(), 2);
        assert_eq!(pos.column_number(), 6);

        let advanced = pos.advance(3);
        assert_eq!(advanced.char, 13);
        assert_eq!(advanced.column, 8);
    }
}
