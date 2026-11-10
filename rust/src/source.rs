//! Source text, offsets and positions.
//!
//! The specification counts in code points (`01-language.md` 6.4,
//! `03-host.md` 3.7.7). Rust strings are UTF-8, so the lexer works on a
//! `Vec<char>` and every offset in this crate is an index into it: a count of
//! code points from the start of the text. A [`Span`] holds two such offsets.
//! A [`Pos`] is a line and a column, both from 1, as a diagnostic shows it.

use std::fmt;

/// A start and an end offset, in code points. The end is exclusive.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Hash)]
pub struct Span {
    pub start: u32,
    pub end: u32,
}

impl Span {
    pub fn new(start: usize, end: usize) -> Span {
        Span {
            start: start as u32,
            end: end as u32,
        }
    }

    /// The empty span at an offset.
    pub fn at(offset: usize) -> Span {
        Span::new(offset, offset)
    }

    /// The span from the start of `self` to the end of `other`.
    pub fn to(self, other: Span) -> Span {
        Span {
            start: self.start,
            end: other.end,
        }
    }
}

/// A line and a column, both counted from 1 (`03-host.md` 3.7.7).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Pos {
    pub line: u32,
    pub column: u32,
}

/// A span as a diagnostic gives it: start line, start column, end line, end
/// column. The end is exclusive.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct LineSpan(pub [u32; 4]);

impl fmt::Display for LineSpan {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let [a, b, c, d] = self.0;
        write!(f, "[{a}, {b}, {c}, {d}]")
    }
}

/// The text of a program, held as code points, with the offsets where lines
/// start. A line ends at U+000A and at nothing else.
#[derive(Clone, Debug)]
pub struct Source {
    chars: Vec<char>,
    line_starts: Vec<u32>,
}

impl Source {
    /// A U+FEFF at the start is removed and not counted: the code point after
    /// it is line 1, column 1.
    pub fn new(text: &str) -> Source {
        let text = text.strip_prefix('\u{FEFF}').unwrap_or(text);
        let chars: Vec<char> = text.chars().collect();
        let mut line_starts = vec![0u32];
        for (i, &c) in chars.iter().enumerate() {
            if c == '\n' {
                line_starts.push(i as u32 + 1);
            }
        }
        Source { chars, line_starts }
    }

    /// The text with nothing removed: a U+FEFF at the start is a code point
    /// of it, as in a template (F2 2.1 item 2).
    pub fn raw(text: &str) -> Source {
        let chars: Vec<char> = text.chars().collect();
        let mut line_starts = vec![0u32];
        for (i, &c) in chars.iter().enumerate() {
            if c == '\n' {
                line_starts.push(i as u32 + 1);
            }
        }
        Source { chars, line_starts }
    }

    pub fn chars(&self) -> &[char] {
        &self.chars
    }

    pub fn len(&self) -> usize {
        self.chars.len()
    }

    pub fn is_empty(&self) -> bool {
        self.chars.is_empty()
    }

    /// The text of a span.
    pub fn slice(&self, span: Span) -> String {
        self.chars[span.start as usize..span.end as usize]
            .iter()
            .collect()
    }

    /// The position of an offset. An offset may be the end of the text.
    pub fn pos(&self, offset: u32) -> Pos {
        let line = match self.line_starts.binary_search(&offset) {
            Ok(i) => i,
            Err(i) => i - 1,
        };
        Pos {
            line: line as u32 + 1,
            column: offset - self.line_starts[line] + 1,
        }
    }

    pub fn line_span(&self, span: Span) -> LineSpan {
        let a = self.pos(span.start);
        let b = self.pos(span.end);
        LineSpan([a.line, a.column, b.line, b.column])
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn positions_count_code_points_and_lines_end_at_lf() {
        let s = Source::new("a\u{e9}\u{1F600}b\nc\rd");
        assert_eq!(s.pos(0), Pos { line: 1, column: 1 });
        assert_eq!(s.pos(3), Pos { line: 1, column: 4 });
        assert_eq!(s.pos(5), Pos { line: 2, column: 1 });
        // U+000D is an ordinary character.
        assert_eq!(s.pos(7), Pos { line: 2, column: 3 });
        assert_eq!(s.pos(8), Pos { line: 2, column: 4 });
    }
}
