//! Tokens (`01-language.md` 2.3).

use crate::source::Span;

/// Punctuation: brackets, separators and operators.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum P {
    LParen,
    RParen,
    LBracket,
    RBracket,
    LBrace,
    RBrace,
    /// `?.[`
    QDotBracket,
    Comma,
    Semi,
    Colon,
    Dot,
    /// `?.`
    QDot,
    Ellipsis,
    Arrow,
    Assign,
    Question,
    Pipe,
    Bang,
    Minus,
    Plus,
    Star,
    Slash,
    Percent,
    StarStar,
    EqEq,
    NotEq,
    Lt,
    Le,
    Gt,
    Ge,
    AndAnd,
    OrOr,
    QQ,
}

impl P {
    pub fn text(self) -> &'static str {
        match self {
            P::LParen => "(",
            P::RParen => ")",
            P::LBracket => "[",
            P::RBracket => "]",
            P::LBrace => "{",
            P::RBrace => "}",
            P::QDotBracket => "?.[",
            P::Comma => ",",
            P::Semi => ";",
            P::Colon => ":",
            P::Dot => ".",
            P::QDot => "?.",
            P::Ellipsis => "...",
            P::Arrow => "=>",
            P::Assign => "=",
            P::Question => "?",
            P::Pipe => "|",
            P::Bang => "!",
            P::Minus => "-",
            P::Plus => "+",
            P::Star => "*",
            P::Slash => "/",
            P::Percent => "%",
            P::StarStar => "**",
            P::EqEq => "==",
            P::NotEq => "!=",
            P::Lt => "<",
            P::Le => "<=",
            P::Gt => ">",
            P::Ge => ">=",
            P::AndAnd => "&&",
            P::OrOr => "||",
            P::QQ => "??",
        }
    }
}

/// The form of a string literal: its opening delimiter.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum StrForm {
    Double,
    Single,
    Backtick,
    /// `"""`, a text block (package F3)
    TripleDouble,
    /// `r"`, a raw string (F3)
    RawDouble,
    /// `r'`, a raw string (F3)
    RawSingle,
    /// `r"""`, a raw text block (F3)
    RawTriple,
}

impl StrForm {
    pub fn as_str(self) -> &'static str {
        match self {
            StrForm::Double => "\"",
            StrForm::Single => "'",
            StrForm::Backtick => "`",
            StrForm::TripleDouble => "\"\"\"",
            StrForm::RawDouble => "r\"",
            StrForm::RawSingle => "r'",
            StrForm::RawTriple => "r\"\"\"",
        }
    }

    /// A `String` token of the grammar (`01-language.md` 3.1; F3 5.1): a
    /// one-line string or a raw string, which may stand as a key and a
    /// literal.
    pub fn is_line(self) -> bool {
        matches!(
            self,
            StrForm::Double | StrForm::Single | StrForm::RawDouble | StrForm::RawSingle
        )
    }
}

#[derive(Clone, Debug, PartialEq)]
pub enum Tok {
    /// A word (2.4). Reserved or not, `_` included.
    Word(String),
    Number(f64),
    /// A one-line string, or a template with no interpolation.
    Str(String, StrForm),
    /// `` `text${ ``: the start of a template with an interpolation.
    TmplHead(String),
    /// `}text${`: between two interpolations.
    TmplMiddle(String),
    /// `` }text` ``: the end of a template.
    TmplTail(String),
    Punct(P),
    /// A token that no rule accepts (2.3, the last row): `===`, `+=`, the
    /// block token, the prefixed literal and the rest. Wherever it stands it is
    /// `unexpected-token`, at the token's `diag` span.
    Refused,
    Eof,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Token {
    pub tok: Tok,
    pub span: Span,
    /// The span a diagnostic names for this token. It is `span`, but for the
    /// block token (its first three code points) and the prefixed literal (the
    /// word).
    pub diag: Span,
    /// A code the parser reports when it reaches this token, with its span:
    /// `text-block-open`, `text-block-close` or `text-block-indent` (F3 6).
    pub code: Option<(&'static str, Span)>,
}

impl Token {
    pub fn is_punct(&self, p: P) -> bool {
        matches!(self.tok, Tok::Punct(q) if q == p)
    }

    pub fn word(&self) -> Option<&str> {
        match &self.tok {
            Tok::Word(w) => Some(w),
            _ => None,
        }
    }

    pub fn is_word(&self, w: &str) -> bool {
        self.word() == Some(w)
    }
}

/// The 26 reserved words (2.4).
pub const RESERVED: [&str; 26] = [
    "true", "false", "null", "const", "if", "else", "in", "not", "try", "catch", "match", "as",
    "is", "nil", "let", "var", "for", "while", "do", "break", "continue", "return", "function",
    "throw", "finally", "import",
];

pub fn is_reserved(word: &str) -> bool {
    RESERVED.contains(&word)
}

/// A name: a word that is not reserved and is not `_`.
pub fn is_name(word: &str) -> bool {
    word != "_" && !is_reserved(word)
}
