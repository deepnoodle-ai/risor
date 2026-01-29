//! Lexer for the Risor scripting language.

use crate::token::{lookup_identifier, Position, Token, TokenKind};
use thiserror::Error;

/// Errors that can occur during lexing.
#[derive(Error, Debug, Clone, PartialEq, Eq)]
pub enum LexerError {
    #[error("Invalid number literal: {literal} at line {line}, column {column}")]
    InvalidNumber {
        literal: String,
        line: usize,
        column: usize,
    },

    #[error("Unterminated string literal at line {line}, column {column}")]
    UnterminatedString { line: usize, column: usize },

    #[error("Unterminated template literal at line {line}, column {column}")]
    UnterminatedTemplate { line: usize, column: usize },

    #[error("Invalid escape sequence: \\{ch} at line {line}, column {column}")]
    InvalidEscape {
        ch: char,
        line: usize,
        column: usize,
    },

    #[error("Invalid hex escape sequence at line {line}, column {column}")]
    InvalidHexEscape { line: usize, column: usize },
}

/// Saved lexer state for backtracking.
#[derive(Debug, Clone)]
pub struct LexerState {
    position: usize,
    next_position: usize,
    ch: char,
    line: usize,
    column: isize,
    line_start: usize,
}

/// Lexer tokenizes Risor source code.
pub struct Lexer {
    input: String,
    chars: Vec<char>,
    position: usize,
    next_position: usize,
    ch: char,
    line: usize,
    column: isize,
    line_start: usize,
    token_start: Position,
}

impl Lexer {
    /// Create a new lexer for the given input.
    pub fn new(input: &str) -> Self {
        let chars: Vec<char> = input.chars().collect();
        let mut lexer = Self {
            input: input.to_string(),
            chars,
            position: 0,
            next_position: 0,
            ch: '\0',
            line: 0,
            column: -1,
            line_start: 0,
            token_start: Position::default(),
        };
        lexer.read_char();
        lexer
    }

    /// Save the current lexer state for backtracking.
    pub fn save_state(&self) -> LexerState {
        LexerState {
            position: self.position,
            next_position: self.next_position,
            ch: self.ch,
            line: self.line,
            column: self.column,
            line_start: self.line_start,
        }
    }

    /// Restore a previously saved lexer state.
    pub fn restore_state(&mut self, state: LexerState) {
        self.position = state.position;
        self.next_position = state.next_position;
        self.ch = state.ch;
        self.line = state.line;
        self.column = state.column;
        self.line_start = state.line_start;
    }

    /// Get the current position.
    fn current_position(&self) -> Position {
        Position::new(
            self.position,
            self.line_start,
            self.line,
            self.column.max(0) as usize,
        )
    }

    /// Read the next character.
    fn read_char(&mut self) {
        if self.next_position >= self.chars.len() {
            self.ch = '\0';
        } else {
            self.ch = self.chars[self.next_position];
        }
        self.position = self.next_position;
        self.next_position += 1;
        self.column += 1;
    }

    /// Peek at the next character without consuming it.
    fn peek_char(&self) -> char {
        if self.next_position >= self.chars.len() {
            '\0'
        } else {
            self.chars[self.next_position]
        }
    }

    /// Peek n characters ahead without consuming.
    fn peek_char_n(&self, n: usize) -> char {
        let idx = self.position + n;
        if idx >= self.chars.len() {
            '\0'
        } else {
            self.chars[idx]
        }
    }

    /// Skip whitespace (spaces and tabs, not newlines).
    fn skip_whitespace(&mut self) {
        while self.ch == ' ' || self.ch == '\t' {
            self.read_char();
        }
    }

    /// Skip to end of line.
    fn skip_to_end_of_line(&mut self) {
        while self.ch != '\n' && self.ch != '\0' {
            self.read_char();
        }
    }

    /// Handle newline and update line tracking.
    fn handle_newline(&mut self) {
        self.line += 1;
        self.column = -1;
        self.line_start = self.next_position;
    }

    /// Start tracking a new token.
    fn start_token(&mut self) {
        self.token_start = self.current_position();
    }

    /// Create a token with the current position as the end.
    fn make_token(&self, kind: TokenKind, literal: String) -> Token {
        Token::new(
            kind,
            literal,
            self.token_start,
            self.current_position().advance(1),
        )
    }

    /// Get the next token.
    pub fn next_token(&mut self) -> Result<Token, LexerError> {
        self.skip_whitespace();

        // Handle shebang at start of file
        if self.line == 0 && self.position <= 1 && self.ch == '#' && self.peek_char() == '!' {
            self.skip_to_end_of_line();
            self.skip_whitespace();
        }

        // Handle comments
        while self.ch == '/' {
            if self.peek_char() == '/' {
                // Single-line comment
                self.skip_to_end_of_line();
                self.skip_whitespace();
            } else if self.peek_char() == '*' {
                // Multi-line comment
                self.read_char(); // consume /
                self.read_char(); // consume *
                while !(self.ch == '*' && self.peek_char() == '/') && self.ch != '\0' {
                    if self.ch == '\n' {
                        self.handle_newline();
                    }
                    self.read_char();
                }
                if self.ch != '\0' {
                    self.read_char(); // consume *
                    self.read_char(); // consume /
                }
                self.skip_whitespace();
            } else {
                break;
            }
        }

        self.start_token();

        // EOF
        if self.ch == '\0' {
            return Ok(self.make_token(TokenKind::Eof, String::new()));
        }

        // Newline
        if self.ch == '\n' {
            self.handle_newline();
            self.read_char();
            return Ok(self.make_token(TokenKind::Newline, "\n".to_string()));
        }

        // Carriage return (handle \r\n as single newline)
        if self.ch == '\r' {
            self.read_char();
            if self.ch == '\n' {
                self.handle_newline();
                self.read_char();
            }
            return Ok(self.make_token(TokenKind::Newline, "\n".to_string()));
        }

        // String literals
        if self.ch == '"' || self.ch == '\'' {
            return self.read_string(self.ch);
        }

        // Template literals
        if self.ch == '`' {
            return self.read_backtick();
        }

        // Numbers
        if self.ch.is_ascii_digit() {
            return self.read_number();
        }

        // Identifiers and keywords
        if is_letter(self.ch) {
            return Ok(self.read_identifier());
        }

        // Operators and punctuation
        if let Some(tok) = self.read_operator() {
            return Ok(tok);
        }

        // Unknown character
        let ch = self.ch;
        self.read_char();
        Ok(self.make_token(TokenKind::Illegal, ch.to_string()))
    }

    /// Read an identifier or keyword.
    fn read_identifier(&mut self) -> Token {
        let start = self.position;
        while is_letter(self.ch) || self.ch.is_ascii_digit() {
            self.read_char();
        }
        let literal: String = self.chars[start..self.position].iter().collect();
        let kind = lookup_identifier(&literal);
        self.make_token(kind, literal)
    }

    /// Read a number literal (int, float, hex, binary, octal).
    fn read_number(&mut self) -> Result<Token, LexerError> {
        let start = self.position;

        // Check for hex, binary, or octal
        if self.ch == '0' {
            let next = self.peek_char().to_ascii_lowercase();
            if next == 'x' {
                // Hexadecimal
                self.read_char(); // consume 0
                self.read_char(); // consume x
                while self.ch.is_ascii_hexdigit() {
                    self.read_char();
                }
                let literal: String = self.chars[start..self.position].iter().collect();
                self.check_trailing_alphanumeric(&literal)?;
                return Ok(self.make_token(TokenKind::Int, literal));
            } else if next == 'b' {
                // Binary
                self.read_char(); // consume 0
                self.read_char(); // consume b
                while self.ch == '0' || self.ch == '1' {
                    self.read_char();
                }
                let literal: String = self.chars[start..self.position].iter().collect();
                self.check_trailing_alphanumeric(&literal)?;
                return Ok(self.make_token(TokenKind::Int, literal));
            } else if next.is_ascii_digit() && next != '.' {
                // Octal
                self.read_char(); // consume leading 0
                while is_octal_digit(self.ch) {
                    self.read_char();
                }
                let literal: String = self.chars[start..self.position].iter().collect();
                self.check_trailing_alphanumeric(&literal)?;
                return Ok(self.make_token(TokenKind::Int, literal));
            }
        }

        // Decimal number (int or float)
        while self.ch.is_ascii_digit() {
            self.read_char();
        }

        let mut is_float = false;

        // Check for decimal point
        if self.ch == '.' && self.peek_char().is_ascii_digit() {
            is_float = true;
            self.read_char(); // consume .
            while self.ch.is_ascii_digit() {
                self.read_char();
            }
        }

        // Check for exponent
        if self.ch == 'e' || self.ch == 'E' {
            is_float = true;
            self.read_char(); // consume e/E
            if self.ch == '+' || self.ch == '-' {
                self.read_char();
            }
            while self.ch.is_ascii_digit() {
                self.read_char();
            }
        }

        let literal: String = self.chars[start..self.position].iter().collect();
        self.check_trailing_alphanumeric(&literal)?;
        let kind = if is_float {
            TokenKind::Float
        } else {
            TokenKind::Int
        };
        Ok(self.make_token(kind, literal))
    }

    /// Check for invalid trailing alphanumeric after number.
    fn check_trailing_alphanumeric(&self, literal: &str) -> Result<(), LexerError> {
        if is_letter(self.ch) {
            return Err(LexerError::InvalidNumber {
                literal: format!("{}{}", literal, self.ch),
                line: self.current_position().line_number(),
                column: self.current_position().column_number(),
            });
        }
        Ok(())
    }

    /// Read a quoted string literal.
    fn read_string(&mut self, quote: char) -> Result<Token, LexerError> {
        let mut chars = Vec::new();
        self.read_char(); // consume opening quote

        while self.ch != quote && self.ch != '\0' && self.ch != '\n' {
            if self.ch == '\\' {
                self.read_char();
                let escaped = self.read_escape_sequence()?;
                chars.push(escaped);
            } else {
                chars.push(self.ch);
                self.read_char();
            }
        }

        if self.ch != quote {
            return Err(LexerError::UnterminatedString {
                line: self.current_position().line_number(),
                column: self.current_position().column_number(),
            });
        }

        self.read_char(); // consume closing quote
        Ok(self.make_token(TokenKind::String, chars.into_iter().collect()))
    }

    /// Read an escape sequence.
    fn read_escape_sequence(&mut self) -> Result<char, LexerError> {
        let ch = self.ch;
        self.read_char();

        match ch {
            'n' => Ok('\n'),
            'r' => Ok('\r'),
            't' => Ok('\t'),
            '\\' => Ok('\\'),
            '"' => Ok('"'),
            '\'' => Ok('\''),
            'a' => Ok('\x07'), // bell
            'b' => Ok('\x08'), // backspace
            'f' => Ok('\x0C'), // form feed
            'v' => Ok('\x0B'), // vertical tab
            'e' => Ok('\x1B'), // escape
            '0' | '1' | '2' | '3' => self.read_octal_escape(ch),
            'x' => self.read_hex_escape(2),
            'u' => self.read_hex_escape(4),
            'U' => self.read_hex_escape(8),
            _ => Err(LexerError::InvalidEscape {
                ch,
                line: self.current_position().line_number(),
                column: self.current_position().column_number(),
            }),
        }
    }

    /// Read an octal escape sequence.
    fn read_octal_escape(&mut self, first_digit: char) -> Result<char, LexerError> {
        let mut value = first_digit.to_digit(8).unwrap();
        for _ in 0..2 {
            if !is_octal_digit(self.ch) {
                break;
            }
            value = value * 8 + self.ch.to_digit(8).unwrap();
            self.read_char();
        }
        Ok(char::from_u32(value).unwrap_or('\u{FFFD}'))
    }

    /// Read a hex escape sequence with n digits.
    fn read_hex_escape(&mut self, n: usize) -> Result<char, LexerError> {
        let mut hex = String::new();
        for _ in 0..n {
            if !self.ch.is_ascii_hexdigit() {
                return Err(LexerError::InvalidHexEscape {
                    line: self.current_position().line_number(),
                    column: self.current_position().column_number(),
                });
            }
            hex.push(self.ch);
            self.read_char();
        }
        let value = u32::from_str_radix(&hex, 16).unwrap();
        Ok(char::from_u32(value).unwrap_or('\u{FFFD}'))
    }

    /// Read a backtick template literal (raw string).
    fn read_backtick(&mut self) -> Result<Token, LexerError> {
        let mut chars = Vec::new();
        self.read_char(); // consume opening backtick

        while self.ch != '`' && self.ch != '\0' {
            if self.ch == '\n' {
                self.handle_newline();
            }
            chars.push(self.ch);
            self.read_char();
        }

        if self.ch != '`' {
            return Err(LexerError::UnterminatedTemplate {
                line: self.current_position().line_number(),
                column: self.current_position().column_number(),
            });
        }

        self.read_char(); // consume closing backtick
        Ok(self.make_token(TokenKind::Template, chars.into_iter().collect()))
    }

    /// Read an operator or punctuation token.
    fn read_operator(&mut self) -> Option<Token> {
        let ch = self.ch;
        let next = self.peek_char();

        // Three-character operators
        if ch == '.' && next == '.' && self.peek_char_n(2) == '.' {
            self.read_char();
            self.read_char();
            self.read_char();
            return Some(self.make_token(TokenKind::Spread, "...".to_string()));
        }

        // Two-character operators
        let two_char = match (ch, next) {
            ('=', '=') => Some((TokenKind::Eq, "==")),
            ('=', '>') => Some((TokenKind::Arrow, "=>")),
            ('!', '=') => Some((TokenKind::NotEq, "!=")),
            ('<', '=') => Some((TokenKind::LtEquals, "<=")),
            ('<', '<') => Some((TokenKind::LtLt, "<<")),
            ('>', '=') => Some((TokenKind::GtEquals, ">=")),
            ('>', '>') => Some((TokenKind::GtGt, ">>")),
            ('&', '&') => Some((TokenKind::And, "&&")),
            ('|', '|') => Some((TokenKind::Or, "||")),
            ('+', '+') => Some((TokenKind::PlusPlus, "++")),
            ('+', '=') => Some((TokenKind::PlusEquals, "+=")),
            ('-', '-') => Some((TokenKind::MinusMinus, "--")),
            ('-', '=') => Some((TokenKind::MinusEquals, "-=")),
            ('*', '*') => Some((TokenKind::Pow, "**")),
            ('*', '=') => Some((TokenKind::AsteriskEquals, "*=")),
            ('/', '=') => Some((TokenKind::SlashEquals, "/=")),
            ('?', '?') => Some((TokenKind::Nullish, "??")),
            ('?', '.') => Some((TokenKind::QuestionDot, "?.")),
            _ => None,
        };

        if let Some((kind, literal)) = two_char {
            self.read_char();
            self.read_char();
            return Some(self.make_token(kind, literal.to_string()));
        }

        // Single-character operators
        let single_char = match ch {
            '+' => Some(TokenKind::Plus),
            '-' => Some(TokenKind::Minus),
            '*' => Some(TokenKind::Asterisk),
            '/' => Some(TokenKind::Slash),
            '%' => Some(TokenKind::Mod),
            '=' => Some(TokenKind::Assign),
            '!' => Some(TokenKind::Bang),
            '<' => Some(TokenKind::Lt),
            '>' => Some(TokenKind::Gt),
            '&' => Some(TokenKind::Ampersand),
            '|' => Some(TokenKind::Pipe),
            '^' => Some(TokenKind::Caret),
            '(' => Some(TokenKind::LParen),
            ')' => Some(TokenKind::RParen),
            '[' => Some(TokenKind::LBracket),
            ']' => Some(TokenKind::RBracket),
            '{' => Some(TokenKind::LBrace),
            '}' => Some(TokenKind::RBrace),
            ',' => Some(TokenKind::Comma),
            ';' => Some(TokenKind::Semicolon),
            ':' => Some(TokenKind::Colon),
            '.' => Some(TokenKind::Period),
            '?' => Some(TokenKind::Question),
            _ => None,
        };

        if let Some(kind) = single_char {
            self.read_char();
            return Some(self.make_token(kind, ch.to_string()));
        }

        None
    }

    /// Get the line text for a given position.
    pub fn get_line_text(&self, pos: &Position) -> &str {
        let end = self.input[pos.line_start..]
            .find('\n')
            .map(|i| pos.line_start + i)
            .unwrap_or(self.input.len());
        &self.input[pos.line_start..end]
    }
}

/// Check if a character is a letter (for identifiers).
fn is_letter(ch: char) -> bool {
    ch.is_ascii_alphabetic() || ch == '_'
}

/// Check if a character is an octal digit.
fn is_octal_digit(ch: char) -> bool {
    ch >= '0' && ch <= '7'
}

/// Tokenize an input string into a vector of tokens.
pub fn tokenize(input: &str) -> Result<Vec<Token>, LexerError> {
    let mut lexer = Lexer::new(input);
    let mut tokens = Vec::new();
    loop {
        let tok = lexer.next_token()?;
        let is_eof = tok.kind == TokenKind::Eof;
        tokens.push(tok);
        if is_eof {
            break;
        }
    }
    Ok(tokens)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_empty_input() {
        let tokens = tokenize("").unwrap();
        assert_eq!(tokens.len(), 1);
        assert_eq!(tokens[0].kind, TokenKind::Eof);
    }

    #[test]
    fn test_identifiers() {
        let tokens = tokenize("foo bar _baz").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Ident);
        assert_eq!(tokens[0].literal, "foo");
        assert_eq!(tokens[1].kind, TokenKind::Ident);
        assert_eq!(tokens[1].literal, "bar");
        assert_eq!(tokens[2].kind, TokenKind::Ident);
        assert_eq!(tokens[2].literal, "_baz");
    }

    #[test]
    fn test_keywords() {
        let tokens = tokenize("let const function return if else true false nil").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Let);
        assert_eq!(tokens[1].kind, TokenKind::Const);
        assert_eq!(tokens[2].kind, TokenKind::Function);
        assert_eq!(tokens[3].kind, TokenKind::Return);
        assert_eq!(tokens[4].kind, TokenKind::If);
        assert_eq!(tokens[5].kind, TokenKind::Else);
        assert_eq!(tokens[6].kind, TokenKind::True);
        assert_eq!(tokens[7].kind, TokenKind::False);
        assert_eq!(tokens[8].kind, TokenKind::Nil);
    }

    #[test]
    fn test_integers() {
        let tokens = tokenize("42 0 123456789").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Int);
        assert_eq!(tokens[0].literal, "42");
        assert_eq!(tokens[1].kind, TokenKind::Int);
        assert_eq!(tokens[1].literal, "0");
        assert_eq!(tokens[2].kind, TokenKind::Int);
        assert_eq!(tokens[2].literal, "123456789");
    }

    #[test]
    fn test_floats() {
        let tokens = tokenize("3.14 0.5 1.0 2.5e10 1e-5 3E+2").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Float);
        assert_eq!(tokens[0].literal, "3.14");
        assert_eq!(tokens[1].kind, TokenKind::Float);
        assert_eq!(tokens[1].literal, "0.5");
        assert_eq!(tokens[2].kind, TokenKind::Float);
        assert_eq!(tokens[2].literal, "1.0");
        assert_eq!(tokens[3].kind, TokenKind::Float);
        assert_eq!(tokens[3].literal, "2.5e10");
        assert_eq!(tokens[4].kind, TokenKind::Float);
        assert_eq!(tokens[4].literal, "1e-5");
        assert_eq!(tokens[5].kind, TokenKind::Float);
        assert_eq!(tokens[5].literal, "3E+2");
    }

    #[test]
    fn test_hex_numbers() {
        let tokens = tokenize("0xFF 0x0 0xDEADBEEF 0xAbCd").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Int);
        assert_eq!(tokens[0].literal, "0xFF");
        assert_eq!(tokens[1].kind, TokenKind::Int);
        assert_eq!(tokens[1].literal, "0x0");
        assert_eq!(tokens[2].kind, TokenKind::Int);
        assert_eq!(tokens[2].literal, "0xDEADBEEF");
        assert_eq!(tokens[3].kind, TokenKind::Int);
        assert_eq!(tokens[3].literal, "0xAbCd");
    }

    #[test]
    fn test_binary_numbers() {
        let tokens = tokenize("0b1010 0b0 0b11111111").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Int);
        assert_eq!(tokens[0].literal, "0b1010");
        assert_eq!(tokens[1].kind, TokenKind::Int);
        assert_eq!(tokens[1].literal, "0b0");
        assert_eq!(tokens[2].kind, TokenKind::Int);
        assert_eq!(tokens[2].literal, "0b11111111");
    }

    #[test]
    fn test_octal_numbers() {
        let tokens = tokenize("0755 0644").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Int);
        assert_eq!(tokens[0].literal, "0755");
        assert_eq!(tokens[1].kind, TokenKind::Int);
        assert_eq!(tokens[1].literal, "0644");
    }

    #[test]
    fn test_invalid_number() {
        let result = tokenize("123abc");
        assert!(result.is_err());
    }

    #[test]
    fn test_strings() {
        let tokens = tokenize(r#""hello" "world""#).unwrap();
        assert_eq!(tokens[0].kind, TokenKind::String);
        assert_eq!(tokens[0].literal, "hello");
        assert_eq!(tokens[1].kind, TokenKind::String);
        assert_eq!(tokens[1].literal, "world");
    }

    #[test]
    fn test_single_quoted_strings() {
        let tokens = tokenize("'hello' 'world'").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::String);
        assert_eq!(tokens[0].literal, "hello");
        assert_eq!(tokens[1].kind, TokenKind::String);
        assert_eq!(tokens[1].literal, "world");
    }

    #[test]
    fn test_escape_sequences() {
        let tokens = tokenize(r#""hello\nworld" "tab\there""#).unwrap();
        assert_eq!(tokens[0].literal, "hello\nworld");
        assert_eq!(tokens[1].literal, "tab\there");
    }

    #[test]
    fn test_hex_escapes() {
        let tokens = tokenize(r#""\x41\x42\x43""#).unwrap();
        assert_eq!(tokens[0].literal, "ABC");
    }

    #[test]
    fn test_unicode_escapes() {
        let tokens = tokenize(r#""\u0048\u0065\u006C\u006C\u006F""#).unwrap();
        assert_eq!(tokens[0].literal, "Hello");
    }

    #[test]
    fn test_octal_escapes() {
        let tokens = tokenize(r#""\101\102\103""#).unwrap();
        assert_eq!(tokens[0].literal, "ABC");
    }

    #[test]
    fn test_unterminated_string() {
        let result = tokenize(r#""hello"#);
        assert!(matches!(result, Err(LexerError::UnterminatedString { .. })));
    }

    #[test]
    fn test_invalid_escape() {
        let result = tokenize(r#""\z""#);
        assert!(matches!(result, Err(LexerError::InvalidEscape { .. })));
    }

    #[test]
    fn test_template_literals() {
        let tokens = tokenize("`hello world`").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Template);
        assert_eq!(tokens[0].literal, "hello world");
    }

    #[test]
    fn test_multiline_template() {
        let tokens = tokenize("`hello\nworld`").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Template);
        assert_eq!(tokens[0].literal, "hello\nworld");
    }

    #[test]
    fn test_raw_template() {
        let tokens = tokenize(r"`hello\nworld`").unwrap();
        assert_eq!(tokens[0].literal, r"hello\nworld");
    }

    #[test]
    fn test_arithmetic_operators() {
        let tokens = tokenize("+ - * / % **").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Plus);
        assert_eq!(tokens[1].kind, TokenKind::Minus);
        assert_eq!(tokens[2].kind, TokenKind::Asterisk);
        assert_eq!(tokens[3].kind, TokenKind::Slash);
        assert_eq!(tokens[4].kind, TokenKind::Mod);
        assert_eq!(tokens[5].kind, TokenKind::Pow);
    }

    #[test]
    fn test_comparison_operators() {
        let tokens = tokenize("== != < > <= >=").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Eq);
        assert_eq!(tokens[1].kind, TokenKind::NotEq);
        assert_eq!(tokens[2].kind, TokenKind::Lt);
        assert_eq!(tokens[3].kind, TokenKind::Gt);
        assert_eq!(tokens[4].kind, TokenKind::LtEquals);
        assert_eq!(tokens[5].kind, TokenKind::GtEquals);
    }

    #[test]
    fn test_logical_operators() {
        let tokens = tokenize("&& || !").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::And);
        assert_eq!(tokens[1].kind, TokenKind::Or);
        assert_eq!(tokens[2].kind, TokenKind::Bang);
    }

    #[test]
    fn test_bitwise_operators() {
        let tokens = tokenize("& | ^ << >>").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Ampersand);
        assert_eq!(tokens[1].kind, TokenKind::Pipe);
        assert_eq!(tokens[2].kind, TokenKind::Caret);
        assert_eq!(tokens[3].kind, TokenKind::LtLt);
        assert_eq!(tokens[4].kind, TokenKind::GtGt);
    }

    #[test]
    fn test_assignment_operators() {
        let tokens = tokenize("= += -= *= /=").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Assign);
        assert_eq!(tokens[1].kind, TokenKind::PlusEquals);
        assert_eq!(tokens[2].kind, TokenKind::MinusEquals);
        assert_eq!(tokens[3].kind, TokenKind::AsteriskEquals);
        assert_eq!(tokens[4].kind, TokenKind::SlashEquals);
    }

    #[test]
    fn test_increment_decrement() {
        let tokens = tokenize("++ --").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::PlusPlus);
        assert_eq!(tokens[1].kind, TokenKind::MinusMinus);
    }

    #[test]
    fn test_special_operators() {
        let tokens = tokenize("=> ?? ?. ...").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Arrow);
        assert_eq!(tokens[1].kind, TokenKind::Nullish);
        assert_eq!(tokens[2].kind, TokenKind::QuestionDot);
        assert_eq!(tokens[3].kind, TokenKind::Spread);
    }

    #[test]
    fn test_punctuation() {
        let tokens = tokenize("( ) [ ] { } , ; : .").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::LParen);
        assert_eq!(tokens[1].kind, TokenKind::RParen);
        assert_eq!(tokens[2].kind, TokenKind::LBracket);
        assert_eq!(tokens[3].kind, TokenKind::RBracket);
        assert_eq!(tokens[4].kind, TokenKind::LBrace);
        assert_eq!(tokens[5].kind, TokenKind::RBrace);
        assert_eq!(tokens[6].kind, TokenKind::Comma);
        assert_eq!(tokens[7].kind, TokenKind::Semicolon);
        assert_eq!(tokens[8].kind, TokenKind::Colon);
        assert_eq!(tokens[9].kind, TokenKind::Period);
    }

    #[test]
    fn test_comments() {
        let tokens = tokenize("foo // comment\nbar").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Ident);
        assert_eq!(tokens[0].literal, "foo");
        assert_eq!(tokens[1].kind, TokenKind::Newline);
        assert_eq!(tokens[2].kind, TokenKind::Ident);
        assert_eq!(tokens[2].literal, "bar");
    }

    #[test]
    fn test_multiline_comments() {
        let tokens = tokenize("foo /* this is\na comment */ bar").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Ident);
        assert_eq!(tokens[0].literal, "foo");
        assert_eq!(tokens[1].kind, TokenKind::Ident);
        assert_eq!(tokens[1].literal, "bar");
    }

    #[test]
    fn test_shebang() {
        let tokens = tokenize("#!/usr/bin/env risor\nfoo").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Newline);
        assert_eq!(tokens[1].kind, TokenKind::Ident);
        assert_eq!(tokens[1].literal, "foo");
    }

    #[test]
    fn test_newlines() {
        let tokens = tokenize("foo\nbar").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Ident);
        assert_eq!(tokens[1].kind, TokenKind::Newline);
        assert_eq!(tokens[2].kind, TokenKind::Ident);
    }

    #[test]
    fn test_crlf() {
        let tokens = tokenize("foo\r\nbar").unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Ident);
        assert_eq!(tokens[1].kind, TokenKind::Newline);
        assert_eq!(tokens[2].kind, TokenKind::Ident);
    }

    #[test]
    fn test_position_tracking() {
        let tokens = tokenize("foo\nbar").unwrap();
        assert_eq!(tokens[0].start.line, 0);
        assert_eq!(tokens[0].start.column, 0);
        assert_eq!(tokens[2].start.line, 1);
        assert_eq!(tokens[2].start.column, 0);
    }

    #[test]
    fn test_save_restore_state() {
        let mut lexer = Lexer::new("foo bar baz");
        let tok1 = lexer.next_token().unwrap();
        assert_eq!(tok1.literal, "foo");

        let state = lexer.save_state();
        let tok2 = lexer.next_token().unwrap();
        assert_eq!(tok2.literal, "bar");

        lexer.restore_state(state);
        let tok3 = lexer.next_token().unwrap();
        assert_eq!(tok3.literal, "bar");
    }

    #[test]
    fn test_let_statement() {
        let tokens = tokenize("let x = 42").unwrap();
        let kinds: Vec<_> = tokens.iter().map(|t| t.kind).collect();
        assert_eq!(
            kinds,
            vec![
                TokenKind::Let,
                TokenKind::Ident,
                TokenKind::Assign,
                TokenKind::Int,
                TokenKind::Eof,
            ]
        );
    }

    #[test]
    fn test_function_definition() {
        let tokens = tokenize("function add(a, b) { return a + b }").unwrap();
        let kinds: Vec<_> = tokens.iter().map(|t| t.kind).collect();
        assert_eq!(
            kinds,
            vec![
                TokenKind::Function,
                TokenKind::Ident,
                TokenKind::LParen,
                TokenKind::Ident,
                TokenKind::Comma,
                TokenKind::Ident,
                TokenKind::RParen,
                TokenKind::LBrace,
                TokenKind::Return,
                TokenKind::Ident,
                TokenKind::Plus,
                TokenKind::Ident,
                TokenKind::RBrace,
                TokenKind::Eof,
            ]
        );
    }

    #[test]
    fn test_arrow_function() {
        let tokens = tokenize("x => x * 2").unwrap();
        let kinds: Vec<_> = tokens.iter().map(|t| t.kind).collect();
        assert_eq!(
            kinds,
            vec![
                TokenKind::Ident,
                TokenKind::Arrow,
                TokenKind::Ident,
                TokenKind::Asterisk,
                TokenKind::Int,
                TokenKind::Eof,
            ]
        );
    }

    #[test]
    fn test_match_expression() {
        let tokens = tokenize(r#"match x { 1 => "one", _ => "other" }"#).unwrap();
        assert_eq!(tokens[0].kind, TokenKind::Match);
        assert_eq!(tokens[1].kind, TokenKind::Ident);
        assert_eq!(tokens[2].kind, TokenKind::LBrace);
    }
}
