//! Diagnostics, in the one shape of `03-host.md` 3.7.

use crate::source::{LineSpan, Source, Span};
use std::fmt;

/// The phase of a failure (`03-host.md` 3.7.2).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Phase {
    Declare,
    Parse,
    Check,
    Run,
}

impl Phase {
    pub fn as_str(self) -> &'static str {
        match self {
            Phase::Declare => "declare",
            Phase::Parse => "parse",
            Phase::Check => "check",
            Phase::Run => "run",
        }
    }
}

impl fmt::Display for Phase {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// The `parse` codes of Core (`01-language.md` 14.2).
pub mod code {
    pub const UNEXPECTED_TOKEN: &str = "unexpected-token";
    pub const UNEXPECTED_CHARACTER: &str = "unexpected-character";
    pub const UNTERMINATED_STRING: &str = "unterminated-string";
    pub const UNTERMINATED_COMMENT: &str = "unterminated-comment";
    pub const BAD_ESCAPE: &str = "bad-escape";
    pub const LINE_CONTINUATION: &str = "line-continuation";
    pub const BAD_NUMBER: &str = "bad-number";
    pub const LITERAL_RANGE: &str = "literal-range";
    pub const NESTING_EXCEEDED: &str = "nesting-exceeded";
    pub const RESERVED_WORD: &str = "reserved-word";

    // `declare` codes (`03-host.md` 3.3.4).
    pub const NAME_CLASH: &str = "name-clash";
    pub const RESERVED_NAMESPACE: &str = "reserved-namespace";
    pub const BAD_NAME: &str = "bad-name";
    pub const BAD_SIGNATURE: &str = "bad-signature";
    pub const BAD_TYPE: &str = "bad-type";
    pub const BAD_CLASS: &str = "bad-class";
    pub const UNSUPPORTED_VERSION: &str = "unsupported-version";
    pub const NO_IMPLEMENTATION: &str = "no-implementation";

    /// The eleven static codes (`05-types.md` 22.1).
    pub const STATIC: [&str; 11] = [
        "undefined-name",
        "undefined-function",
        "not-a-value",
        "call-of-parameter",
        "lambda-to-lambda",
        "duplicate-name",
        "shadows-namespace",
        "duplicate-key",
        "arity",
        "duplicate-case",
        "unreachable-arm",
    ];

    /// The five codes of class `limit` (`03-host.md` 3.6.1).
    pub const LIMIT: [&str; 5] = [
        "steps-exceeded",
        "allocation-exceeded",
        "depth-exceeded",
        "deadline-exceeded",
        "cancelled",
    ];
}

/// One problem found by a stage.
///
/// `range` is the span in code-point offsets, for the implementation's own
/// use. `span` is the same span as lines and columns, which is what the
/// specification compares.
#[derive(Clone, Debug, PartialEq)]
pub struct Diagnostic {
    pub phase: Phase,
    pub code: &'static str,
    pub class: Option<&'static str>,
    pub range: Option<Span>,
    pub span: Option<LineSpan>,
    pub message: String,
}

impl Diagnostic {
    /// A diagnostic at a span of a source text.
    pub fn at(
        phase: Phase,
        code: &'static str,
        src: &Source,
        range: Span,
        message: impl Into<String>,
    ) -> Diagnostic {
        Diagnostic {
            phase,
            code,
            class: None,
            range: Some(range),
            span: Some(src.line_span(range)),
            message: message.into(),
        }
    }

    /// A diagnostic with no span, as a `declare` diagnostic is.
    pub fn bare(phase: Phase, code: &'static str, message: impl Into<String>) -> Diagnostic {
        Diagnostic {
            phase,
            code,
            class: None,
            range: None,
            span: None,
            message: message.into(),
        }
    }
}

impl fmt::Display for Diagnostic {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} {}", self.phase, self.code)?;
        if let Some(s) = self.span {
            write!(f, " at {s}")?;
        }
        if !self.message.is_empty() {
            write!(f, ": {}", self.message)?;
        }
        Ok(())
    }
}

/// A failure of a stage: one phase and one or more diagnostics.
#[derive(Clone, Debug, PartialEq)]
pub struct Error {
    pub phase: Phase,
    pub diagnostics: Vec<Diagnostic>,
}

impl Error {
    pub fn one(d: Diagnostic) -> Error {
        Error {
            phase: d.phase,
            diagnostics: vec![d],
        }
    }

    /// The first diagnostic. An error always has one.
    pub fn first(&self) -> &Diagnostic {
        &self.diagnostics[0]
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.first())?;
        if self.diagnostics.len() > 1 {
            write!(f, " (and {} more)", self.diagnostics.len() - 1)?;
        }
        Ok(())
    }
}

impl std::error::Error for Error {}
