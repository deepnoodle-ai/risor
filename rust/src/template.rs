//! Package F2: host templates (`spec/packages/f02-templates-analysis.md` 2).
//!
//! A template is a host text with `${{ }}` parts. [`parse_template`] reads
//! it whole first, every part's tokens included (2.3 item 3), then parses the
//! parts in order. The program of each mode (2.5) is an ordinary
//! [`Parsed`], which checks, compiles and runs as any program does.

use crate::diag::Error;
use crate::source::{Source, Span};
use crate::syntax::ast::{Ast, NodeId, NodeKind, TemplatePart};
use crate::syntax::lexer::lex_part;
use crate::syntax::parser::parse_parts;
use crate::syntax::token::{StrForm, Tok, Token, P};
use crate::syntax::Parsed;

/// How a host takes the program of a template (2.5).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Mode {
    /// The value is always a string.
    String,
    /// A single template gives its expression's value as it is.
    Value,
}

impl Mode {
    pub fn from_name(s: &str) -> Option<Mode> {
        match s {
            "string" => Some(Mode::String),
            "value" => Some(Mode::Value),
            _ => None,
        }
    }
}

/// A segment of a template (2.4).
#[derive(Clone, Debug, PartialEq)]
pub enum Segment {
    /// A text run: its value, with each `$${{` read as `${{`, and the span it
    /// was read from.
    Text { value: String, span: Span },
    /// A part: the span of its expression, and of the whole part from the `$`
    /// of `${{` to the end of its closing delimiter.
    Part { expr: Span, span: Span },
}

/// A parsed template.
#[derive(Clone, Debug)]
pub struct Template {
    source: Source,
    /// The parts' expressions; the program nodes are added for each mode.
    ast: Ast,
    parts: Vec<NodeId>,
    segments: Vec<Segment>,
}

/// Parse a template text (2.1 to 2.3). A failure has phase `parse` and one
/// diagnostic: the first. A U+FEFF at the start is a code point of the text.
pub fn parse_template(text: &str) -> Result<Template, Error> {
    let source = Source::raw(text);
    let s = source.chars();
    let n = s.len();
    let mut segments: Vec<Segment> = Vec::new();
    let mut toks: Vec<Token> = Vec::new();
    let mut part_starts: Vec<(usize, Span)> = Vec::new();
    let mut part_spans: Vec<Span> = Vec::new();
    let mut run = String::new();
    let mut run_start = 0usize;
    let mut i = 0usize;
    let close_run = |run: &mut String, start: usize, end: usize, segs: &mut Vec<Segment>| {
        if end > start {
            segs.push(Segment::Text {
                value: std::mem::take(run),
                span: Span::new(start, end),
            });
        }
    };
    while i < n {
        if s[i] == '$'
            && s.get(i + 1) == Some(&'$')
            && s.get(i + 2) == Some(&'{')
            && s.get(i + 3) == Some(&'{')
        {
            run.push_str("${{");
            i += 4;
            continue;
        }
        if s[i] == '$' && s.get(i + 1) == Some(&'{') && s.get(i + 2) == Some(&'{') {
            close_run(&mut run, run_start, i, &mut segments);
            let dollar = i;
            let (mut ptoks, brace) = lex_part(&source, i + 3, dollar).map_err(Error::one)?;
            part_starts.push((toks.len(), Span::new(dollar, dollar + 3)));
            toks.append(&mut ptoks);
            let end = if s.get(brace + 1) == Some(&'}') {
                // The closing delimiter `}}`.
                let sp = Span::new(brace, brace + 2);
                toks.push(Token {
                    tok: Tok::Eof,
                    span: sp,
                    diag: sp,
                    code: None,
                });
                brace + 2
            } else {
                // A lone closing brace (2.2 item 5): `unexpected-token` where
                // the parser reaches it.
                let sp = Span::new(brace, brace + 1);
                toks.push(Token {
                    tok: Tok::Punct(P::RBrace),
                    span: sp,
                    diag: sp,
                    code: None,
                });
                let e = Span::at(brace + 1);
                toks.push(Token {
                    tok: Tok::Eof,
                    span: e,
                    diag: e,
                    code: None,
                });
                brace + 1
            };
            part_spans.push(Span::new(dollar, end));
            segments.push(Segment::Part {
                expr: Span::at(0),
                span: Span::new(dollar, end),
            });
            i = end;
            run_start = i;
            continue;
        }
        run.push(s[i]);
        i += 1;
    }
    close_run(&mut run, run_start, n, &mut segments);
    let (ast, parts) = if part_starts.is_empty() {
        (Ast::default(), Vec::new())
    } else {
        parse_parts(&source, &toks, &part_starts).map_err(Error::one)?
    };
    let mut k = 0;
    for seg in &mut segments {
        if let Segment::Part { expr, .. } = seg {
            *expr = ast.span(parts[k]);
            k += 1;
        }
    }
    Ok(Template {
        source,
        ast,
        parts,
        segments,
    })
}

impl Template {
    /// The template text.
    pub fn source(&self) -> &Source {
        &self.source
    }

    /// The text runs and parts, in order (2.4).
    pub fn segments(&self) -> &[Segment] {
        &self.segments
    }

    /// Exactly one part, with no text run before or after it.
    pub fn is_single(&self) -> bool {
        self.segments.len() == 1 && matches!(self.segments[0], Segment::Part { .. })
    }

    /// The number of parts.
    pub fn parts(&self) -> usize {
        self.parts.len()
    }

    /// A part as a program of its own: its expression alone, with spans in
    /// the whole text. Each part is an expression of its own (2.5 item 6).
    pub fn part_program(&self, i: usize) -> Option<Parsed> {
        let e = *self.parts.get(i)?;
        let mut ast = self.ast.clone();
        let span = ast.span(e);
        let root = ast.push(
            NodeKind::Program {
                consts: Vec::new(),
                result: e,
            },
            span,
            0,
        );
        ast.root = root;
        Some(Parsed {
            source: self.source.clone(),
            ast,
        })
    }

    /// The program of the template in a mode (2.5).
    pub fn program(&self, mode: Mode) -> Parsed {
        let mut ast = self.ast.clone();
        let whole = Span::new(0, self.source.len());
        let (result, span) = if mode == Mode::Value && self.is_single() {
            (self.parts[0], ast.span(self.parts[0]))
        } else if self.parts.is_empty() {
            let value = self
                .segments
                .iter()
                .map(|s| match s {
                    Segment::Text { value, .. } => value.as_str(),
                    Segment::Part { .. } => "",
                })
                .collect::<String>();
            let id = ast.push(
                NodeKind::String {
                    value,
                    form: StrForm::Host,
                },
                whole,
                0,
            );
            (id, whole)
        } else {
            let mut k = 0;
            let parts = self
                .segments
                .iter()
                .map(|s| match s {
                    Segment::Text { value, .. } => TemplatePart::Text(value.clone()),
                    Segment::Part { .. } => {
                        k += 1;
                        TemplatePart::Expr(self.parts[k - 1])
                    }
                })
                .collect();
            let id = ast.push(NodeKind::Template { parts }, whole, 0);
            (id, whole)
        };
        let root = ast.push(
            NodeKind::Program {
                consts: Vec::new(),
                result,
            },
            span,
            0,
        );
        ast.root = root;
        Parsed {
            source: self.source.clone(),
            ast,
        }
    }
}
