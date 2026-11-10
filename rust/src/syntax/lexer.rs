//! The lexer (`01-language.md` section 2).
//!
//! The whole text is divided into tokens before it is parsed (2.11). The
//! lexer stops at the first lexical diagnostic it meets, reading from the
//! start, so that diagnostic is the first of the parse.
//!
//! Templates nest without bound, so the lexer keeps an explicit stack of the
//! open interpolations and never recurses.

use super::token::{is_reserved, StrForm, Tok, Token, P};
use crate::diag::{code, Diagnostic, Phase};
use crate::source::{Source, Span};

/// One open interpolation: how many `{` are open inside it, and the offset of
/// the backtick of its template.
struct Interp {
    depth: u32,
    tmpl_start: usize,
}

struct Lexer<'a> {
    src: &'a Source,
    s: &'a [char],
    i: usize,
    out: Vec<Token>,
    stack: Vec<Interp>,
    /// The index in `out` of the head of each open template.
    heads: Vec<usize>,
}

/// Divide a text into tokens. The last token is `Tok::Eof`.
pub fn lex(src: &Source) -> Result<Vec<Token>, Diagnostic> {
    let mut lx = Lexer {
        src,
        s: src.chars(),
        i: 0,
        out: Vec::new(),
        stack: Vec::new(),
        heads: Vec::new(),
    };
    lx.run()?;
    Ok(lx.out)
}

const MAX_WHOLE: u128 = (1u128 << 53) - 1;

fn is_hex(c: char) -> bool {
    c.is_ascii_hexdigit()
}

fn is_word_start(c: char) -> bool {
    c.is_ascii_alphabetic() || c == '_'
}

fn is_word_char(c: char) -> bool {
    c.is_ascii_alphanumeric() || c == '_'
}

impl<'a> Lexer<'a> {
    fn err(&self, c: &'static str, start: usize, end: usize, msg: &str) -> Diagnostic {
        Diagnostic::at(Phase::Parse, c, self.src, Span::new(start, end), msg)
    }

    fn at(&self, j: usize) -> Option<char> {
        self.s.get(j).copied()
    }

    fn push(&mut self, tok: Tok, start: usize, end: usize) {
        let span = Span::new(start, end);
        self.out.push(Token {
            tok,
            span,
            diag: span,
        });
    }

    fn punct(&mut self, p: P, len: usize) -> Result<(), Diagnostic> {
        let start = self.i;
        self.i += len;
        self.push(Tok::Punct(p), start, self.i);
        Ok(())
    }

    fn refused(&mut self, len: usize) -> Result<(), Diagnostic> {
        let start = self.i;
        self.i += len;
        self.push(Tok::Refused, start, self.i);
        Ok(())
    }

    fn run(&mut self) -> Result<(), Diagnostic> {
        let n = self.s.len();
        loop {
            self.skip_space()?;
            if self.i >= n {
                if let Some(top) = self.stack.last() {
                    return Err(self.err(
                        code::UNTERMINATED_STRING,
                        top.tmpl_start,
                        n,
                        "the template has no closing backtick",
                    ));
                }
                self.push(Tok::Eof, n, n);
                return Ok(());
            }
            self.token()?;
        }
    }

    fn skip_space(&mut self) -> Result<(), Diagnostic> {
        let n = self.s.len();
        while self.i < n {
            match self.s[self.i] {
                ' ' | '\t' | '\n' | '\r' => self.i += 1,
                '/' if self.at(self.i + 1) == Some('/') => {
                    self.i += 2;
                    while self.i < n && self.s[self.i] != '\n' && self.s[self.i] != '\r' {
                        self.i += 1;
                    }
                }
                '/' if self.at(self.i + 1) == Some('*') => {
                    let start = self.i;
                    let mut j = start + 2;
                    loop {
                        if j + 1 >= n {
                            return Err(self.err(
                                code::UNTERMINATED_COMMENT,
                                start,
                                start + 2,
                                "the comment has no closing */",
                            ));
                        }
                        if self.s[j] == '*' && self.s[j + 1] == '/' {
                            break;
                        }
                        j += 1;
                    }
                    self.i = j + 2;
                }
                _ => break,
            }
        }
        Ok(())
    }

    fn token(&mut self) -> Result<(), Diagnostic> {
        let i = self.i;
        let c = self.s[i];
        let next = self.at(i + 1);
        let next2 = self.at(i + 2);
        match c {
            c if is_word_start(c) => self.word(),
            c if c.is_ascii_digit() => self.number(),
            '.' if next.is_some_and(|d| d.is_ascii_digit()) => self.number(),
            '"' | '\'' => {
                if next == Some(c) && next2 == Some(c) {
                    self.block_token(c);
                    Ok(())
                } else {
                    self.string(c)
                }
            }
            '`' => self.template_start(),
            '(' => self.punct(P::LParen, 1),
            ')' => self.punct(P::RParen, 1),
            '[' => self.punct(P::LBracket, 1),
            ']' => self.punct(P::RBracket, 1),
            '{' => {
                if let Some(top) = self.stack.last_mut() {
                    top.depth += 1;
                }
                self.punct(P::LBrace, 1)
            }
            '}' => {
                if let Some(top) = self.stack.last_mut() {
                    if top.depth == 0 {
                        return self.template_continue();
                    }
                    top.depth -= 1;
                }
                self.punct(P::RBrace, 1)
            }
            ',' => self.punct(P::Comma, 1),
            ';' => self.punct(P::Semi, 1),
            ':' => self.punct(P::Colon, 1),
            '.' => {
                if next == Some('.') && next2 == Some('.') {
                    self.punct(P::Ellipsis, 3)
                } else {
                    self.punct(P::Dot, 1)
                }
            }
            '?' => match (next, next2) {
                (Some('.'), Some('[')) => self.punct(P::QDotBracket, 3),
                (Some('.'), _) => self.punct(P::QDot, 2),
                (Some('?'), Some('=')) => self.refused(3),
                (Some('?'), _) => self.punct(P::QQ, 2),
                _ => self.punct(P::Question, 1),
            },
            '=' => match (next, next2) {
                (Some('='), Some('=')) => self.refused(3),
                (Some('='), _) => self.punct(P::EqEq, 2),
                (Some('>'), _) => self.punct(P::Arrow, 2),
                _ => self.punct(P::Assign, 1),
            },
            '!' => match (next, next2) {
                (Some('='), Some('=')) => self.refused(3),
                (Some('='), _) => self.punct(P::NotEq, 2),
                _ => self.punct(P::Bang, 1),
            },
            '+' => match next {
                Some('=') | Some('+') => self.refused(2),
                _ => self.punct(P::Plus, 1),
            },
            '-' => match next {
                Some('=') => self.refused(2),
                _ => self.punct(P::Minus, 1),
            },
            '*' => match (next, next2) {
                (Some('*'), Some('=')) => self.refused(3),
                (Some('*'), _) => self.punct(P::StarStar, 2),
                (Some('='), _) => self.refused(2),
                _ => self.punct(P::Star, 1),
            },
            '/' => match next {
                Some('=') => self.refused(2),
                _ => self.punct(P::Slash, 1),
            },
            '%' => match next {
                Some('=') => self.refused(2),
                _ => self.punct(P::Percent, 1),
            },
            '<' => match next {
                Some('=') => self.punct(P::Le, 2),
                _ => self.punct(P::Lt, 1),
            },
            '>' => match next {
                Some('=') => self.punct(P::Ge, 2),
                _ => self.punct(P::Gt, 1),
            },
            '&' => match (next, next2) {
                (Some('&'), Some('=')) => self.refused(3),
                (Some('&'), _) => self.punct(P::AndAnd, 2),
                _ => Err(self.unexpected_char()),
            },
            '|' => match (next, next2) {
                (Some('|'), Some('=')) => self.refused(3),
                (Some('|'), _) => self.punct(P::OrOr, 2),
                (Some('>'), _) => self.refused(2),
                _ => self.punct(P::Pipe, 1),
            },
            _ => Err(self.unexpected_char()),
        }
    }

    fn unexpected_char(&self) -> Diagnostic {
        let c = self.s[self.i];
        self.err(
            code::UNEXPECTED_CHARACTER,
            self.i,
            self.i + 1,
            &format!("the character U+{:04X} cannot stand here", c as u32),
        )
    }

    fn word(&mut self) -> Result<(), Diagnostic> {
        let start = self.i;
        let n = self.s.len();
        let mut j = start;
        while j < n && is_word_char(self.s[j]) {
            j += 1;
        }
        let text: String = self.s[start..j].iter().collect();
        // 2.3 rule 9: the prefixed literal.
        if let Some(q @ ('"' | '\'' | '`')) = self.at(j) {
            if !is_reserved(&text) {
                let mut k = j + 1;
                let end = loop {
                    match self.at(k) {
                        None => break n,
                        Some(c) if c == q => break k + 1,
                        Some('\n' | '\r') if q != '`' => break k,
                        Some(_) => k += 1,
                    }
                };
                self.i = end;
                self.out.push(Token {
                    tok: Tok::Refused,
                    span: Span::new(start, end),
                    diag: Span::new(start, j),
                });
                return Ok(());
            }
        }
        self.i = j;
        self.push(Tok::Word(text), start, j);
        Ok(())
    }

    /// 2.3 rule 7: the block token.
    fn block_token(&mut self, q: char) {
        let start = self.i;
        let n = self.s.len();
        let mut j = start + 3;
        let end = loop {
            if j + 3 > n {
                break n;
            }
            if self.s[j] == q && self.s[j + 1] == q && self.s[j + 2] == q {
                break j + 3;
            }
            j += 1;
        };
        self.i = end;
        self.out.push(Token {
            tok: Tok::Refused,
            span: Span::new(start, end),
            diag: Span::new(start, start + 3),
        });
    }

    fn number(&mut self) -> Result<(), Diagnostic> {
        let start = self.i;
        let n = self.s.len();
        let hex = self.s[start] == '0' && matches!(self.at(start + 1), Some('x' | 'X'));
        let mut j = start;
        while j < n {
            let c = self.s[j];
            let sign =
                (c == '+' || c == '-') && j > start && matches!(self.s[j - 1], 'e' | 'E') && !hex;
            let dot = c == '.'
                && match self.at(j + 1) {
                    None => true,
                    Some(d) => d.is_ascii_digit() || !(d.is_ascii_alphabetic() || d == '_'),
                };
            if !(is_word_char(c) || sign || dot) {
                break;
            }
            j += 1;
        }
        self.i = j;
        let text: Vec<char> = self.s[start..j].to_vec();
        match number_value(&text) {
            Ok(v) => {
                self.push(Tok::Number(v), start, j);
                Ok(())
            }
            Err(NumErr::Bad) => Err(self.err(
                code::BAD_NUMBER,
                start,
                j,
                "the number is not written in a form the language accepts",
            )),
            Err(NumErr::Range) => Err(self.err(
                code::LITERAL_RANGE,
                start,
                j,
                "the number is outside the range of the language",
            )),
        }
    }

    fn string(&mut self, q: char) -> Result<(), Diagnostic> {
        let start = self.i;
        let n = self.s.len();
        let mut j = start + 1;
        let mut value = String::new();
        loop {
            if j >= n {
                return Err(self.unterminated(start, n));
            }
            let c = self.s[j];
            if c == q {
                j += 1;
                break;
            }
            match c {
                '\n' | '\r' => return Err(self.unterminated(start, j)),
                '\\' => match self.at(j + 1) {
                    None => return Err(self.unterminated(start, n)),
                    Some('\n' | '\r') => return Err(self.unterminated(start, j + 1)),
                    Some(_) => {
                        let (ch, k) = self.escape(j, false)?;
                        value.push(ch);
                        j = k;
                    }
                },
                _ => {
                    value.push(c);
                    j += 1;
                }
            }
        }
        self.i = j;
        let form = if q == '"' {
            StrForm::Double
        } else {
            StrForm::Single
        };
        self.push(Tok::Str(value, form), start, j);
        Ok(())
    }

    fn unterminated(&self, start: usize, end: usize) -> Diagnostic {
        self.err(
            code::UNTERMINATED_STRING,
            start,
            end,
            "the string has no closing quote",
        )
    }

    /// Read the escape at `j`, where `s[j]` is `\` and a code point follows
    /// it. Returns the code point and the offset after the escape (2.7).
    fn escape(&self, j: usize, template: bool) -> Result<(char, usize), Diagnostic> {
        let e = self.s[j + 1];
        let simple = match e {
            '"' => Some('"'),
            '\'' => Some('\''),
            '\\' => Some('\\'),
            '/' => Some('/'),
            'b' => Some('\u{8}'),
            'f' => Some('\u{c}'),
            'n' => Some('\n'),
            'r' => Some('\r'),
            't' => Some('\t'),
            '`' if template => Some('`'),
            '$' if template => Some('$'),
            _ => None,
        };
        if let Some(c) = simple {
            return Ok((c, j + 2));
        }
        if e != 'u' {
            return Err(self.bad_escape(j, j + 2));
        }
        let n = self.s.len();
        let k = j + 2;
        if self.at(k) == Some('{') {
            let mut m = k + 1;
            while m < n && is_hex(self.s[m]) {
                m += 1;
            }
            let count = m - (k + 1);
            if count == 0 {
                return Err(self.bad_escape(j, k + 1));
            }
            if count > 6 || self.at(m) != Some('}') {
                return Err(self.bad_escape(j, m));
            }
            let v = hex_value(&self.s[k + 1..m]);
            return match char::from_u32(v) {
                Some(c) => Ok((c, m + 1)),
                None => Err(self.bad_escape(j, m + 1)),
            };
        }
        let m = self.four_hex(k);
        if m - k < 4 {
            return Err(self.bad_escape(j, m));
        }
        let v = hex_value(&self.s[k..m]);
        if (0xD800..=0xDBFF).contains(&v) {
            // A high surrogate must be followed directly by a low one.
            if self.at(m) == Some('\\') && self.at(m + 1) == Some('u') {
                let m2 = self.four_hex(m + 2);
                if m2 - (m + 2) == 4 {
                    let lo = hex_value(&self.s[m + 2..m2]);
                    if (0xDC00..=0xDFFF).contains(&lo) {
                        let cp = 0x10000 + ((v - 0xD800) << 10) + (lo - 0xDC00);
                        return Ok((char::from_u32(cp).unwrap_or('\u{FFFD}'), m2));
                    }
                }
            }
            return Err(self.bad_escape(j, m));
        }
        match char::from_u32(v) {
            Some(c) => Ok((c, m)),
            None => Err(self.bad_escape(j, m)),
        }
    }

    /// The offset after at most four hex digits from `k`.
    fn four_hex(&self, k: usize) -> usize {
        let mut m = k;
        while m < k + 4 && m < self.s.len() && is_hex(self.s[m]) {
            m += 1;
        }
        m
    }

    fn bad_escape(&self, start: usize, end: usize) -> Diagnostic {
        self.err(
            code::BAD_ESCAPE,
            start,
            end,
            "the escape is not one the language accepts",
        )
    }

    fn template_start(&mut self) -> Result<(), Diagnostic> {
        let start = self.i;
        let (text, end, interp) = self.template_text(start + 1, start)?;
        self.i = end;
        if interp {
            self.heads.push(self.out.len());
            self.push(Tok::TmplHead(text), start, end);
            self.stack.push(Interp {
                depth: 0,
                tmpl_start: start,
            });
        } else {
            self.push(Tok::Str(text, StrForm::Backtick), start, end);
        }
        Ok(())
    }

    /// At the `}` that closes an interpolation.
    fn template_continue(&mut self) -> Result<(), Diagnostic> {
        let start = self.i;
        let ts = self.stack.pop().map(|t| t.tmpl_start).unwrap_or(start);
        let (text, end, interp) = self.template_text(start + 1, ts)?;
        self.i = end;
        // Ruling V1: a template where no token may stand is reported with
        // the span of the whole template, and a `}` that closes an
        // interpolation too early with the span of that `}`.
        if interp {
            self.push(Tok::TmplMiddle(text), start, end);
            self.stack.push(Interp {
                depth: 0,
                tmpl_start: ts,
            });
        } else {
            self.push(Tok::TmplTail(text), start, end);
            if let Some(h) = self.heads.pop() {
                self.out[h].diag = Span::new(ts, end);
            }
        }
        if let Some(t) = self.out.last_mut() {
            t.diag = Span::new(start, start + 1);
        }
        Ok(())
    }

    /// Read a text part from `j` to a closing backtick or a `${`. Returns its
    /// value, the offset after the delimiter, and whether it was `${`.
    fn template_text(
        &self,
        mut j: usize,
        tmpl_start: usize,
    ) -> Result<(String, usize, bool), Diagnostic> {
        let n = self.s.len();
        let mut text = String::new();
        loop {
            if j >= n {
                return Err(self.err(
                    code::UNTERMINATED_STRING,
                    tmpl_start,
                    n,
                    "the template has no closing backtick",
                ));
            }
            match self.s[j] {
                '`' => return Ok((text, j + 1, false)),
                '$' if self.at(j + 1) == Some('{') => return Ok((text, j + 2, true)),
                '\\' => match self.at(j + 1) {
                    None => {
                        return Err(self.err(
                            code::UNTERMINATED_STRING,
                            tmpl_start,
                            n,
                            "the template has no closing backtick",
                        ))
                    }
                    Some('\n' | '\r') => {
                        return Err(self.err(
                            code::LINE_CONTINUATION,
                            j,
                            j + 1,
                            "a backslash cannot stand before a line end in a template",
                        ))
                    }
                    Some(_) => {
                        let (c, k) = self.escape(j, true)?;
                        text.push(c);
                        j = k;
                    }
                },
                '\r' => {
                    text.push('\n');
                    j += if self.at(j + 1) == Some('\n') { 2 } else { 1 };
                }
                c => {
                    text.push(c);
                    j += 1;
                }
            }
        }
    }
}

fn hex_value(digits: &[char]) -> u32 {
    digits
        .iter()
        .fold(0u32, |acc, c| acc * 16 + c.to_digit(16).unwrap_or(0))
}

enum NumErr {
    Bad,
    Range,
}

/// Match a number token against `Number` (2.5) and give its value.
fn number_value(t: &[char]) -> Result<f64, NumErr> {
    let digit_run = |t: &[char], mut i: usize, hex: bool| -> Option<usize> {
        // Digit { [ "_" ] Digit }, from i. Returns the end, or None.
        let ok = |c: char| if hex { is_hex(c) } else { c.is_ascii_digit() };
        if i >= t.len() || !ok(t[i]) {
            return None;
        }
        i += 1;
        loop {
            if i < t.len() && ok(t[i]) {
                i += 1;
            } else if i + 1 < t.len() && t[i] == '_' && ok(t[i + 1]) {
                i += 2;
            } else {
                return Some(i);
            }
        }
    };
    if t.len() >= 2 && t[0] == '0' && t[1] == 'x' {
        let end = digit_run(t, 2, true).ok_or(NumErr::Bad)?;
        if end != t.len() {
            return Err(NumErr::Bad);
        }
        let mut v: u128 = 0;
        for &c in &t[2..] {
            if let Some(d) = c.to_digit(16) {
                v = (v * 16 + d as u128).min(MAX_WHOLE + 1);
            }
        }
        if v > MAX_WHOLE {
            return Err(NumErr::Range);
        }
        return Ok(v as f64);
    }
    // Int = "0" | NonZero { [ "_" ] Digit }.
    let mut i = match t.first() {
        Some('0') => 1,
        Some(c) if c.is_ascii_digit() => digit_run(t, 0, false).ok_or(NumErr::Bad)?,
        _ => return Err(NumErr::Bad),
    };
    let mut integer = true;
    if i < t.len() && t[i] == '.' {
        i = digit_run(t, i + 1, false).ok_or(NumErr::Bad)?;
        integer = false;
    }
    if i < t.len() && (t[i] == 'e' || t[i] == 'E') {
        i += 1;
        if i < t.len() && (t[i] == '+' || t[i] == '-') {
            i += 1;
        }
        i = digit_run(t, i, false).ok_or(NumErr::Bad)?;
        integer = false;
    }
    if i != t.len() {
        return Err(NumErr::Bad);
    }
    let clean: String = t.iter().filter(|&&c| c != '_').collect();
    if integer {
        let mut v: u128 = 0;
        for c in clean.chars() {
            v = (v * 10 + c.to_digit(10).unwrap_or(0) as u128).min(MAX_WHOLE + 1);
        }
        if v > MAX_WHOLE {
            return Err(NumErr::Range);
        }
        return Ok(v as f64);
    }
    // Rust's parse rounds to nearest, ties to even, and gives zero for a value
    // too small to represent.
    let v: f64 = clean.parse().map_err(|_| NumErr::Bad)?;
    if !v.is_finite() {
        return Err(NumErr::Range);
    }
    Ok(v)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn toks(s: &str) -> Vec<Tok> {
        lex(&Source::new(s))
            .unwrap()
            .into_iter()
            .map(|t| t.tok)
            .collect()
    }

    #[test]
    fn number_tokens() {
        assert_eq!(
            toks("5.a"),
            vec![
                Tok::Number(5.0),
                Tok::Punct(P::Dot),
                Tok::Word("a".into()),
                Tok::Eof
            ]
        );
        assert_eq!(toks("0x1F"), vec![Tok::Number(31.0), Tok::Eof]);
        assert_eq!(toks("1_000"), vec![Tok::Number(1000.0), Tok::Eof]);
        assert_eq!(toks("1e-400"), vec![Tok::Number(0.0), Tok::Eof]);
        assert_eq!(
            toks("9007199254740993.0"),
            vec![Tok::Number(9007199254740992.0), Tok::Eof]
        );
    }

    #[test]
    fn longest_tokens() {
        assert_eq!(
            toks("a?.[b"),
            vec![
                Tok::Word("a".into()),
                Tok::Punct(P::QDotBracket),
                Tok::Word("b".into()),
                Tok::Eof
            ]
        );
        assert_eq!(
            toks("--x"),
            vec![
                Tok::Punct(P::Minus),
                Tok::Punct(P::Minus),
                Tok::Word("x".into()),
                Tok::Eof
            ]
        );
    }

    #[test]
    fn templates_nest() {
        let t = toks("`a${`b${1}c`}d`");
        assert_eq!(
            t,
            vec![
                Tok::TmplHead("a".into()),
                Tok::TmplHead("b".into()),
                Tok::Number(1.0),
                Tok::TmplTail("c".into()),
                Tok::TmplTail("d".into()),
                Tok::Eof
            ]
        );
        assert_eq!(
            toks("`x\r\ny`"),
            vec![Tok::Str("x\ny".into(), StrForm::Backtick), Tok::Eof]
        );
    }

    #[test]
    fn deep_template_nesting_does_not_recurse() {
        let depth = 200_000;
        let mut s = String::new();
        for _ in 0..depth {
            s.push_str("`${");
        }
        s.push('1');
        for _ in 0..depth {
            s.push_str("}`");
        }
        let t = lex(&Source::new(&s)).unwrap();
        assert_eq!(t.len(), 2 * depth + 2);
    }
}
