//! Diagnostics, in the one shape of `03-host.md` 3.7.

use crate::source::{Range, Source, Span};
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
    /// Text where a program type is read that F4 2.1 to 2.3 do not accept.
    pub const BAD_TYPE_TEXT: &str = "bad-type-text";
    pub const TEXT_BLOCK_OPEN: &str = "text-block-open";
    pub const TEXT_BLOCK_CLOSE: &str = "text-block-close";
    pub const TEXT_BLOCK_INDENT: &str = "text-block-indent";

    // `declare` codes (`03-host.md` 3.3.4).
    pub const NAME_CLASH: &str = "name-clash";
    pub const RESERVED_NAMESPACE: &str = "reserved-namespace";
    pub const BAD_NAME: &str = "bad-name";
    pub const BAD_SIGNATURE: &str = "bad-signature";
    pub const BAD_TYPE: &str = "bad-type";
    pub const BAD_CLASS: &str = "bad-class";
    pub const UNSUPPORTED_VERSION: &str = "unsupported-version";
    pub const NO_IMPLEMENTATION: &str = "no-implementation";
    /// A limit out of range (`03-host.md` 3.3.4).
    pub const BAD_LIMIT: &str = "bad-limit";

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
    pub(crate) range: Option<Range>,
    pub span: Option<Span>,
    pub message: String,
    /// For `failed`: the second argument of `fail`, or null.
    pub data: Option<Box<crate::host::Data>>,
    /// For `not-conforming` (F4 5.1): the path from the tested value to the
    /// place of the failure (`03-host.md` 3.7.8).
    pub path: Option<Box<Vec<PathStep>>>,
    /// For a `run` diagnostic: the calls in progress when the run failed,
    /// from the outermost in (`f08-errors.md` 4). `None` is an empty trace,
    /// and every other phase has none. Read it with `trace()`.
    pub trace: Option<Box<Vec<TraceEntry>>>,
}

/// An entry of a trace (`f08-errors.md` 4.2).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct TraceEntry {
    /// The span of the call, in code-point offsets and as lines and columns.
    pub(crate) range: Range,
    pub span: Span,
    /// The name of a `const`-bound lambda, or of a library entry as written.
    pub name: String,
    /// For a library entry, the index of the element; else `None`.
    pub index: Option<usize>,
}

/// A step of a path (`03-host.md` 3.7.8): a key or an index.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum PathStep {
    Key(String),
    Index(usize),
}

impl Diagnostic {
    /// The trace of a `run` diagnostic, from the outermost call in
    /// (`f08-errors.md` 4.1).
    pub fn trace(&self) -> &[TraceEntry] {
        self.trace.as_deref().map(|t| t.as_slice()).unwrap_or(&[])
    }

    /// A diagnostic at a span of a source text.
    pub(crate) fn at(
        phase: Phase,
        code: &'static str,
        src: &Source,
        range: Range,
        message: impl Into<String>,
    ) -> Diagnostic {
        Diagnostic {
            phase,
            code,
            class: None,
            range: Some(range),
            span: Some(src.line_span(range)),
            message: message.into(),
            data: None,
            path: None,
            trace: None,
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
            data: None,
            path: None,
            trace: None,
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
