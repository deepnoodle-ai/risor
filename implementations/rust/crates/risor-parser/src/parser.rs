//! Pratt parser for Risor.

use crate::ast::*;
use crate::precedence::Precedence;
use risor_lexer::{Lexer, LexerError, Position, Token, TokenKind};
use std::rc::Rc;
use thiserror::Error;

/// Errors that can occur during parsing.
#[derive(Error, Debug, Clone)]
pub enum ParserError {
    #[error("{message} at line {line}, column {column}")]
    Syntax {
        message: String,
        line: usize,
        column: usize,
    },

    #[error("lexer error: {0}")]
    Lexer(#[from] LexerError),
}

impl ParserError {
    fn new(message: impl Into<String>, pos: Position) -> Self {
        Self::Syntax {
            message: message.into(),
            line: pos.line_number(),
            column: pos.column_number(),
        }
    }
}

type PrefixParseFn = fn(&mut Parser) -> Option<Expr>;
type InfixParseFn = fn(&mut Parser, Expr) -> Option<Expr>;

/// Pratt parser for Risor source code.
pub struct Parser {
    lexer: Lexer,
    cur_token: Token,
    peek_token: Token,
    errors: Vec<ParserError>,
    max_depth: usize,
    depth: usize,
}

impl Parser {
    /// Create a new parser for the given lexer.
    pub fn new(mut lexer: Lexer) -> Result<Self, LexerError> {
        let cur_token = lexer.next_token()?;
        let peek_token = lexer.next_token()?;
        Ok(Self {
            lexer,
            cur_token,
            peek_token,
            errors: Vec::new(),
            max_depth: 500,
            depth: 0,
        })
    }

    fn next_token(&mut self) -> Result<(), LexerError> {
        self.cur_token = std::mem::replace(&mut self.peek_token, self.lexer.next_token()?);
        Ok(())
    }

    fn cur_token_is(&self, kind: TokenKind) -> bool {
        self.cur_token.kind == kind
    }

    fn peek_token_is(&self, kind: TokenKind) -> bool {
        self.peek_token.kind == kind
    }

    #[allow(dead_code)]
    fn expect_peek(&mut self, kind: TokenKind) -> bool {
        if self.peek_token_is(kind) {
            let _ = self.next_token();
            true
        } else {
            self.peek_error(kind);
            false
        }
    }

    #[allow(dead_code)]
    fn peek_error(&mut self, kind: TokenKind) {
        self.errors.push(ParserError::new(
            format!("expected {:?}, got {:?}", kind, self.peek_token.kind),
            self.peek_token.start,
        ));
    }

    fn no_prefix_parse_fn_error(&mut self, kind: TokenKind) {
        self.errors.push(ParserError::new(
            format!("unexpected token {:?}", kind),
            self.cur_token.start,
        ));
    }

    fn cur_precedence(&self) -> Precedence {
        Precedence::from_token(self.cur_token.kind)
    }

    #[allow(dead_code)]
    fn peek_precedence(&self) -> Precedence {
        Precedence::from_token(self.peek_token.kind)
    }

    fn eat_newlines(&mut self) {
        while self.cur_token_is(TokenKind::Newline) {
            let _ = self.next_token();
        }
    }

    /// Synchronize after an error by skipping to the next statement boundary.
    fn synchronize(&mut self) {
        while !self.cur_token_is(TokenKind::Eof) {
            if self.cur_token_is(TokenKind::Newline) {
                let _ = self.next_token();
                return;
            }
            match self.cur_token.kind {
                TokenKind::Let
                | TokenKind::Const
                | TokenKind::Function
                | TokenKind::Return
                | TokenKind::If
                | TokenKind::Switch
                | TokenKind::Match
                | TokenKind::Try
                | TokenKind::Throw => return,
                _ => {
                    let _ = self.next_token();
                }
            }
        }
    }

    /// Parse the entire program.
    pub fn parse(&mut self) -> Result<Program, ParserError> {
        let mut stmts = Vec::new();

        self.eat_newlines();

        while !self.cur_token_is(TokenKind::Eof) {
            if let Some(stmt) = self.parse_statement() {
                stmts.push(stmt);
            } else {
                self.synchronize();
            }
            self.eat_newlines();
        }

        if !self.errors.is_empty() {
            return Err(self.errors[0].clone());
        }

        Ok(Program { stmts })
    }

    /// Get all parse errors.
    pub fn errors(&self) -> &[ParserError] {
        &self.errors
    }

    // =========================================================================
    // Statement Parsing
    // =========================================================================

    fn parse_statement(&mut self) -> Option<Stmt> {
        match self.cur_token.kind {
            TokenKind::Let => self.parse_let(),
            TokenKind::Const => self.parse_const(),
            TokenKind::Return => Some(self.parse_return()),
            TokenKind::Throw => self.parse_throw(),
            _ => self.parse_expression_statement(),
        }
    }

    fn parse_let(&mut self) -> Option<Stmt> {
        let let_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'let'

        // Check for destructuring
        if self.cur_token_is(TokenKind::LBrace) {
            return self.parse_object_destructure(let_pos);
        }
        if self.cur_token_is(TokenKind::LBracket) {
            return self.parse_array_destructure(let_pos);
        }

        // Simple variable or multi-var
        if !self.cur_token_is(TokenKind::Ident) {
            self.errors
                .push(ParserError::new("expected identifier", self.cur_token.start));
            return None;
        }

        let first_name = Ident {
            position: self.cur_token.start,
            name: self.cur_token.literal.clone(),
        };
        let _ = self.next_token();

        // Check for multi-var (let x, y = ...)
        if self.cur_token_is(TokenKind::Comma) {
            let mut names = vec![first_name];
            while self.cur_token_is(TokenKind::Comma) {
                let _ = self.next_token(); // consume ','
                if !self.cur_token_is(TokenKind::Ident) {
                    self.errors
                        .push(ParserError::new("expected identifier", self.cur_token.start));
                    return None;
                }
                names.push(Ident {
                    position: self.cur_token.start,
                    name: self.cur_token.literal.clone(),
                });
                let _ = self.next_token();
            }
            if !self.cur_token_is(TokenKind::Assign) {
                self.errors
                    .push(ParserError::new("expected '='", self.cur_token.start));
                return None;
            }
            let _ = self.next_token(); // consume '='
            let value = self.parse_expression(Precedence::Lowest)?;
            return Some(Stmt::MultiVar(MultiVarStmt {
                let_pos,
                names,
                value,
            }));
        }

        // Simple variable
        if !self.cur_token_is(TokenKind::Assign) {
            self.errors
                .push(ParserError::new("expected '='", self.cur_token.start));
            return None;
        }
        let _ = self.next_token(); // consume '='
        let value = self.parse_expression(Precedence::Lowest)?;
        Some(Stmt::Var(VarStmt {
            let_pos,
            name: first_name,
            value,
        }))
    }

    fn parse_object_destructure(&mut self, let_pos: Position) -> Option<Stmt> {
        let lbrace = self.cur_token.start;
        let _ = self.next_token(); // consume '{'
        self.eat_newlines();

        let mut bindings = Vec::new();
        while !self.cur_token_is(TokenKind::RBrace) && !self.cur_token_is(TokenKind::Eof) {
            if !self.cur_token_is(TokenKind::Ident) {
                self.errors
                    .push(ParserError::new("expected identifier", self.cur_token.start));
                return None;
            }
            let key = self.cur_token.literal.clone();
            let mut alias = None;
            let mut default_value = None;
            let _ = self.next_token();

            // Check for alias (: alias)
            if self.cur_token_is(TokenKind::Colon) {
                let _ = self.next_token(); // consume ':'
                if !self.cur_token_is(TokenKind::Ident) {
                    self.errors
                        .push(ParserError::new("expected identifier", self.cur_token.start));
                    return None;
                }
                alias = Some(self.cur_token.literal.clone());
                let _ = self.next_token();
            }

            // Check for default (= value)
            if self.cur_token_is(TokenKind::Assign) {
                let _ = self.next_token(); // consume '='
                default_value = Some(self.parse_expression(Precedence::Lowest)?);
            }

            bindings.push(DestructureBinding {
                key,
                alias,
                default_value,
            });

            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token(); // consume ','
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RBrace) {
            self.errors
                .push(ParserError::new("expected '}'", self.cur_token.start));
            return None;
        }
        let rbrace = self.cur_token.start;
        let _ = self.next_token(); // consume '}'

        if !self.cur_token_is(TokenKind::Assign) {
            self.errors
                .push(ParserError::new("expected '='", self.cur_token.start));
            return None;
        }
        let _ = self.next_token(); // consume '='
        let value = self.parse_expression(Precedence::Lowest)?;

        Some(Stmt::ObjectDestructure(ObjectDestructureStmt {
            let_pos,
            lbrace,
            bindings,
            rbrace,
            value,
        }))
    }

    fn parse_array_destructure(&mut self, let_pos: Position) -> Option<Stmt> {
        let lbrack = self.cur_token.start;
        let _ = self.next_token(); // consume '['
        self.eat_newlines();

        let mut elements = Vec::new();
        while !self.cur_token_is(TokenKind::RBracket) && !self.cur_token_is(TokenKind::Eof) {
            if !self.cur_token_is(TokenKind::Ident) {
                self.errors
                    .push(ParserError::new("expected identifier", self.cur_token.start));
                return None;
            }
            let name = Ident {
                position: self.cur_token.start,
                name: self.cur_token.literal.clone(),
            };
            let mut default_value = None;
            let _ = self.next_token();

            // Check for default (= value)
            if self.cur_token_is(TokenKind::Assign) {
                let _ = self.next_token(); // consume '='
                default_value = Some(self.parse_expression(Precedence::Lowest)?);
            }

            elements.push(ArrayDestructureElement {
                name,
                default_value,
            });

            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token(); // consume ','
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RBracket) {
            self.errors
                .push(ParserError::new("expected ']'", self.cur_token.start));
            return None;
        }
        let rbrack = self.cur_token.start;
        let _ = self.next_token(); // consume ']'

        if !self.cur_token_is(TokenKind::Assign) {
            self.errors
                .push(ParserError::new("expected '='", self.cur_token.start));
            return None;
        }
        let _ = self.next_token(); // consume '='
        let value = self.parse_expression(Precedence::Lowest)?;

        Some(Stmt::ArrayDestructure(ArrayDestructureStmt {
            let_pos,
            lbrack,
            elements,
            rbrack,
            value,
        }))
    }

    fn parse_const(&mut self) -> Option<Stmt> {
        let const_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'const'

        if !self.cur_token_is(TokenKind::Ident) {
            self.errors
                .push(ParserError::new("expected identifier", self.cur_token.start));
            return None;
        }
        let name = Ident {
            position: self.cur_token.start,
            name: self.cur_token.literal.clone(),
        };
        let _ = self.next_token();

        if !self.cur_token_is(TokenKind::Assign) {
            self.errors
                .push(ParserError::new("expected '='", self.cur_token.start));
            return None;
        }
        let _ = self.next_token(); // consume '='

        let value = self.parse_expression(Precedence::Lowest)?;

        Some(Stmt::Const(ConstStmt {
            const_pos,
            name,
            value,
        }))
    }

    fn parse_return(&mut self) -> Stmt {
        let return_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'return'

        // Check for empty return
        if self.cur_token_is(TokenKind::Newline)
            || self.cur_token_is(TokenKind::Eof)
            || self.cur_token_is(TokenKind::RBrace)
        {
            return Stmt::Return(ReturnStmt {
                return_pos,
                value: None,
            });
        }

        let value = self.parse_expression(Precedence::Lowest);
        Stmt::Return(ReturnStmt {
            return_pos,
            value,
        })
    }

    fn parse_throw(&mut self) -> Option<Stmt> {
        let throw_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'throw'

        let value = self.parse_expression(Precedence::Lowest)?;

        Some(Stmt::Throw(ThrowStmt { throw_pos, value }))
    }

    fn parse_expression_statement(&mut self) -> Option<Stmt> {
        let expr = self.parse_expression(Precedence::Lowest)?;

        // Check for assignment
        if self.cur_token_is(TokenKind::Assign) || self.is_compound_assign() {
            return self.parse_assignment(expr);
        }

        // Check for postfix (++ or --)
        if self.cur_token_is(TokenKind::PlusPlus) || self.cur_token_is(TokenKind::MinusMinus) {
            let op_pos = self.cur_token.start;
            let op = self.cur_token.literal.clone();
            let _ = self.next_token();
            return Some(Stmt::Postfix(PostfixStmt {
                operand: expr,
                op_pos,
                op,
            }));
        }

        Some(Stmt::Expr(expr))
    }

    fn is_compound_assign(&self) -> bool {
        matches!(
            self.cur_token.kind,
            TokenKind::PlusEquals
                | TokenKind::MinusEquals
                | TokenKind::AsteriskEquals
                | TokenKind::SlashEquals
        )
    }

    fn parse_assignment(&mut self, target: Expr) -> Option<Stmt> {
        let op_pos = self.cur_token.start;
        let op = self.cur_token.literal.clone();
        let _ = self.next_token(); // consume operator

        let value = self.parse_expression(Precedence::Lowest)?;

        // Property assignment
        if let Expr::GetAttr(get_attr) = target {
            return Some(Stmt::SetAttr(SetAttrStmt {
                object: get_attr.object,
                period: get_attr.period,
                attr: get_attr.attr,
                op_pos,
                op,
                value,
            }));
        }

        // Index assignment or simple assignment
        match target {
            Expr::Ident(ident) => Some(Stmt::Assign(AssignStmt {
                target: AssignTarget::Ident(ident),
                op_pos,
                op,
                value,
            })),
            Expr::Index(index) => Some(Stmt::Assign(AssignStmt {
                target: AssignTarget::Index(index),
                op_pos,
                op,
                value,
            })),
            _ => {
                self.errors
                    .push(ParserError::new("invalid assignment target", target.pos()));
                None
            }
        }
    }

    // =========================================================================
    // Expression Parsing
    // =========================================================================

    fn parse_expression(&mut self, precedence: Precedence) -> Option<Expr> {
        self.depth += 1;
        if self.depth > self.max_depth {
            self.errors.push(ParserError::new(
                "maximum expression depth exceeded",
                self.cur_token.start,
            ));
            self.depth -= 1;
            return None;
        }

        let prefix_fn = self.get_prefix_fn(self.cur_token.kind);
        let Some(prefix_fn) = prefix_fn else {
            self.no_prefix_parse_fn_error(self.cur_token.kind);
            self.depth -= 1;
            return None;
        };

        let mut left = prefix_fn(self)?;

        // Handle chaining operators after newlines
        loop {
            if self.cur_token_is(TokenKind::Eof) {
                break;
            }

            // Skip newlines for chaining operators
            if self.cur_token_is(TokenKind::Newline) {
                if self.peek_token_is(TokenKind::Period)
                    || self.peek_token_is(TokenKind::QuestionDot)
                {
                    let _ = self.next_token(); // consume newline
                    continue;
                }
                break;
            }

            if precedence >= self.cur_precedence() {
                break;
            }

            let infix_fn = self.get_infix_fn(self.cur_token.kind);
            let Some(infix_fn) = infix_fn else {
                break;
            };

            left = infix_fn(self, left)?;
        }

        self.depth -= 1;
        Some(left)
    }

    fn get_prefix_fn(&self, kind: TokenKind) -> Option<PrefixParseFn> {
        match kind {
            TokenKind::Ident => Some(Parser::parse_ident),
            TokenKind::Int => Some(Parser::parse_int),
            TokenKind::Float => Some(Parser::parse_float),
            TokenKind::String => Some(Parser::parse_string),
            TokenKind::Template => Some(Parser::parse_template),
            TokenKind::True | TokenKind::False => Some(Parser::parse_bool),
            TokenKind::Nil => Some(Parser::parse_nil),
            TokenKind::Bang | TokenKind::Minus | TokenKind::Not => Some(Parser::parse_prefix),
            TokenKind::LParen => Some(Parser::parse_grouped),
            TokenKind::LBracket => Some(Parser::parse_list),
            TokenKind::LBrace => Some(Parser::parse_map),
            TokenKind::If => Some(Parser::parse_if),
            TokenKind::Switch => Some(Parser::parse_switch),
            TokenKind::Match => Some(Parser::parse_match),
            TokenKind::Function => Some(Parser::parse_func),
            TokenKind::Spread => Some(Parser::parse_spread),
            TokenKind::Try => Some(Parser::parse_try),
            _ => None,
        }
    }

    fn get_infix_fn(&self, kind: TokenKind) -> Option<InfixParseFn> {
        match kind {
            TokenKind::Plus
            | TokenKind::Minus
            | TokenKind::Asterisk
            | TokenKind::Slash
            | TokenKind::Mod
            | TokenKind::Pow
            | TokenKind::Eq
            | TokenKind::NotEq
            | TokenKind::Lt
            | TokenKind::Gt
            | TokenKind::LtEquals
            | TokenKind::GtEquals
            | TokenKind::And
            | TokenKind::Or
            | TokenKind::Ampersand
            | TokenKind::Caret
            | TokenKind::LtLt
            | TokenKind::GtGt
            | TokenKind::Nullish => Some(Parser::parse_infix),
            TokenKind::Pipe => Some(Parser::parse_pipe_infix),
            TokenKind::LParen => Some(Parser::parse_call),
            TokenKind::LBracket => Some(Parser::parse_index),
            TokenKind::Period => Some(Parser::parse_get_attr),
            TokenKind::QuestionDot => Some(Parser::parse_optional_chain),
            TokenKind::In => Some(Parser::parse_in),
            TokenKind::Not => Some(Parser::parse_not_in),
            _ => None,
        }
    }

    // =========================================================================
    // Literal Parsing
    // =========================================================================

    fn parse_ident(&mut self) -> Option<Expr> {
        let ident = Ident {
            position: self.cur_token.start,
            name: self.cur_token.literal.clone(),
        };
        let _ = self.next_token();

        // Check for arrow function (x => ...)
        if self.cur_token_is(TokenKind::Arrow) {
            return self.parse_arrow_func(vec![FuncParam::Ident(ident)]);
        }

        Some(Expr::Ident(ident))
    }

    fn parse_int(&mut self) -> Option<Expr> {
        let literal = self.cur_token.literal.clone();
        let value = if literal.starts_with("0x") || literal.starts_with("0X") {
            i64::from_str_radix(&literal[2..], 16).ok()?
        } else if literal.starts_with("0b") || literal.starts_with("0B") {
            i64::from_str_radix(&literal[2..], 2).ok()?
        } else if literal.starts_with('0') && literal.len() > 1 && !literal.contains('.') {
            // Octal
            i64::from_str_radix(&literal[1..], 8).ok()?
        } else {
            literal.parse().ok()?
        };
        let node = IntLit {
            position: self.cur_token.start,
            literal,
            value,
        };
        let _ = self.next_token();
        Some(Expr::Int(node))
    }

    fn parse_float(&mut self) -> Option<Expr> {
        let literal = self.cur_token.literal.clone();
        let value: f64 = literal.parse().ok()?;
        let node = FloatLit {
            position: self.cur_token.start,
            literal,
            value,
        };
        let _ = self.next_token();
        Some(Expr::Float(node))
    }

    fn parse_string(&mut self) -> Option<Expr> {
        let node = StringLit {
            position: self.cur_token.start,
            value: self.cur_token.literal.clone(),
        };
        let _ = self.next_token();
        Some(Expr::String(node))
    }

    fn parse_template(&mut self) -> Option<Expr> {
        // For now, treat templates as raw strings (no interpolation)
        let node = StringLit {
            position: self.cur_token.start,
            value: self.cur_token.literal.clone(),
        };
        let _ = self.next_token();
        Some(Expr::String(node))
    }

    fn parse_bool(&mut self) -> Option<Expr> {
        let value = self.cur_token_is(TokenKind::True);
        let node = BoolLit {
            position: self.cur_token.start,
            value,
        };
        let _ = self.next_token();
        Some(Expr::Bool(node))
    }

    fn parse_nil(&mut self) -> Option<Expr> {
        let node = NilLit {
            position: self.cur_token.start,
        };
        let _ = self.next_token();
        Some(Expr::Nil(node))
    }

    // =========================================================================
    // Operator Parsing
    // =========================================================================

    fn parse_prefix(&mut self) -> Option<Expr> {
        let op_pos = self.cur_token.start;
        let op = self.cur_token.literal.clone();
        let _ = self.next_token();

        // Special handling for - before ** (right-associative)
        let precedence = if op == "-"
            && self.cur_token_is(TokenKind::Int)
            && self.peek_token_is(TokenKind::Pow)
        {
            Precedence::Power
        } else {
            Precedence::Prefix
        };

        let right = self.parse_expression(precedence)?;

        Some(Expr::Prefix(Box::new(PrefixExpr { op_pos, op, right })))
    }

    fn parse_infix(&mut self, left: Expr) -> Option<Expr> {
        let op_pos = self.cur_token.start;
        let op = self.cur_token.literal.clone();
        let precedence = self.cur_precedence();
        let _ = self.next_token();

        // Right-associative for **
        let next_precedence = if op == "**" {
            // Use one level lower for right-associativity
            match precedence {
                Precedence::Power => Precedence::Product,
                other => other,
            }
        } else {
            precedence
        };

        let right = self.parse_expression(next_precedence)?;

        Some(Expr::Infix(Box::new(InfixExpr {
            left,
            op_pos,
            op,
            right,
        })))
    }

    fn parse_spread(&mut self) -> Option<Expr> {
        let ellipsis = self.cur_token.start;
        let _ = self.next_token(); // consume '...'

        // Check if there's an expression following
        if self.cur_token_is(TokenKind::Comma)
            || self.cur_token_is(TokenKind::RParen)
            || self.cur_token_is(TokenKind::RBracket)
            || self.cur_token_is(TokenKind::RBrace)
        {
            return Some(Expr::Spread(Box::new(SpreadExpr {
                ellipsis,
                expr: None,
            })));
        }

        let expr = self.parse_expression(Precedence::Lowest)?;

        Some(Expr::Spread(Box::new(SpreadExpr {
            ellipsis,
            expr: Some(expr),
        })))
    }

    // =========================================================================
    // Collection Parsing
    // =========================================================================

    fn parse_list(&mut self) -> Option<Expr> {
        let lbrack = self.cur_token.start;
        let _ = self.next_token(); // consume '['
        self.eat_newlines();

        let mut items = Vec::new();
        while !self.cur_token_is(TokenKind::RBracket) && !self.cur_token_is(TokenKind::Eof) {
            let item = self.parse_expression(Precedence::Lowest)?;
            items.push(item);

            self.eat_newlines();
            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token(); // consume ','
            self.eat_newlines();
        }

        self.eat_newlines();
        if !self.cur_token_is(TokenKind::RBracket) {
            self.errors
                .push(ParserError::new("expected ']'", self.cur_token.start));
            return None;
        }
        let rbrack = self.cur_token.start;
        let _ = self.next_token(); // consume ']'

        Some(Expr::List(ListLit {
            lbrack,
            items,
            rbrack,
        }))
    }

    fn parse_map(&mut self) -> Option<Expr> {
        let lbrace = self.cur_token.start;
        let _ = self.next_token(); // consume '{'
        self.eat_newlines();

        let mut items = Vec::new();
        while !self.cur_token_is(TokenKind::RBrace) && !self.cur_token_is(TokenKind::Eof) {
            // Check for spread
            if self.cur_token_is(TokenKind::Spread) {
                let _ = self.next_token(); // consume '...'
                let value = self.parse_expression(Precedence::Lowest)?;
                items.push(MapItem { key: None, value });
            } else {
                // Key-value pair
                let key = self.parse_expression(Precedence::Lowest)?;

                // Shorthand syntax { a } is equivalent to { a: a }
                if self.cur_token_is(TokenKind::Comma)
                    || self.cur_token_is(TokenKind::RBrace)
                    || self.cur_token_is(TokenKind::Newline)
                {
                    if let Expr::Ident(_) = &key {
                        items.push(MapItem {
                            key: Some(key.clone()),
                            value: key,
                        });
                    } else {
                        self.errors
                            .push(ParserError::new("expected ':'", self.cur_token.start));
                        return None;
                    }
                } else {
                    if !self.cur_token_is(TokenKind::Colon) {
                        self.errors
                            .push(ParserError::new("expected ':'", self.cur_token.start));
                        return None;
                    }
                    let _ = self.next_token(); // consume ':'
                    self.eat_newlines();
                    let value = self.parse_expression(Precedence::Lowest)?;
                    items.push(MapItem {
                        key: Some(key),
                        value,
                    });
                }
            }

            self.eat_newlines();
            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token(); // consume ','
            self.eat_newlines();
        }

        self.eat_newlines();
        if !self.cur_token_is(TokenKind::RBrace) {
            self.errors
                .push(ParserError::new("expected '}'", self.cur_token.start));
            return None;
        }
        let rbrace = self.cur_token.start;
        let _ = self.next_token(); // consume '}'

        Some(Expr::Map(MapLit {
            lbrace,
            items,
            rbrace,
        }))
    }

    // =========================================================================
    // Grouping and Arrow Functions
    // =========================================================================

    fn parse_grouped(&mut self) -> Option<Expr> {
        let _lparen = self.cur_token.start;
        let _ = self.next_token(); // consume '('
        self.eat_newlines();

        // Check for empty parens (arrow function with no params)
        if self.cur_token_is(TokenKind::RParen) {
            let _ = self.next_token(); // consume ')'
            if self.cur_token_is(TokenKind::Arrow) {
                return self.parse_arrow_func(vec![]);
            }
            self.errors
                .push(ParserError::new("unexpected ')'", self.cur_token.start));
            return None;
        }

        // Parse first expression
        let first = self.parse_expression(Precedence::Lowest)?;

        // Check for arrow function with multiple params
        if self.cur_token_is(TokenKind::Comma) {
            // Collect identifiers for arrow function params
            let Expr::Ident(first_ident) = first else {
                self.errors.push(ParserError::new(
                    "expected identifier in parameter list",
                    first.pos(),
                ));
                return None;
            };
            let mut params = vec![FuncParam::Ident(first_ident)];
            while self.cur_token_is(TokenKind::Comma) {
                let _ = self.next_token(); // consume ','
                self.eat_newlines();
                if !self.cur_token_is(TokenKind::Ident) {
                    self.errors
                        .push(ParserError::new("expected identifier", self.cur_token.start));
                    return None;
                }
                params.push(FuncParam::Ident(Ident {
                    position: self.cur_token.start,
                    name: self.cur_token.literal.clone(),
                }));
                let _ = self.next_token();
            }
            self.eat_newlines();
            if !self.cur_token_is(TokenKind::RParen) {
                self.errors
                    .push(ParserError::new("expected ')'", self.cur_token.start));
                return None;
            }
            let _ = self.next_token(); // consume ')'
            if self.cur_token_is(TokenKind::Arrow) {
                return self.parse_arrow_func(params);
            }
            self.errors
                .push(ParserError::new("expected '=>'", self.cur_token.start));
            return None;
        }

        self.eat_newlines();
        if !self.cur_token_is(TokenKind::RParen) {
            self.errors
                .push(ParserError::new("expected ')'", self.cur_token.start));
            return None;
        }
        let _ = self.next_token(); // consume ')'

        // Check for arrow function with single param in parens
        if self.cur_token_is(TokenKind::Arrow) {
            if let Expr::Ident(ident) = first {
                return self.parse_arrow_func(vec![FuncParam::Ident(ident)]);
            }
        }

        Some(first)
    }

    fn parse_arrow_func(&mut self, params: Vec<FuncParam>) -> Option<Expr> {
        let arrow_pos = self.cur_token.start;
        let _ = self.next_token(); // consume '=>'
        self.eat_newlines();

        // Parse body (expression or block)
        if self.cur_token_is(TokenKind::LBrace) {
            let body = self.parse_block()?;
            let func_pos = params
                .first()
                .map(|p| match p {
                    FuncParam::Ident(i) => i.position,
                    FuncParam::ObjectDestructure { lbrace, .. } => *lbrace,
                    FuncParam::ArrayDestructure { lbrack, .. } => *lbrack,
                })
                .unwrap_or(arrow_pos);
            return Some(Expr::Func(Rc::new(FuncLit {
                func_pos,
                name: None,
                lparen: arrow_pos,
                params,
                defaults: vec![],
                rest_param: None,
                rparen: arrow_pos,
                body,
            })));
        }

        // Expression body - wrap in return
        let expr = self.parse_expression(Precedence::Lowest)?;
        let expr_pos = expr.pos();
        let expr_end = expr.end();

        let return_stmt = Stmt::Return(ReturnStmt {
            return_pos: expr_pos,
            value: Some(expr),
        });
        let body = Block {
            lbrace: expr_pos,
            stmts: vec![return_stmt],
            rbrace: expr_end,
        };

        let func_pos = params
            .first()
            .map(|p| match p {
                FuncParam::Ident(i) => i.position,
                FuncParam::ObjectDestructure { lbrace, .. } => *lbrace,
                FuncParam::ArrayDestructure { lbrack, .. } => *lbrack,
            })
            .unwrap_or(arrow_pos);

        Some(Expr::Func(Rc::new(FuncLit {
            func_pos,
            name: None,
            lparen: arrow_pos,
            params,
            defaults: vec![],
            rest_param: None,
            rparen: arrow_pos,
            body,
        })))
    }

    // =========================================================================
    // Function Parsing
    // =========================================================================

    fn parse_func(&mut self) -> Option<Expr> {
        let func_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'function'

        // Optional function name
        let name = if self.cur_token_is(TokenKind::Ident) {
            let n = Some(Ident {
                position: self.cur_token.start,
                name: self.cur_token.literal.clone(),
            });
            let _ = self.next_token();
            n
        } else {
            None
        };

        if !self.cur_token_is(TokenKind::LParen) {
            self.errors
                .push(ParserError::new("expected '('", self.cur_token.start));
            return None;
        }
        let lparen = self.cur_token.start;
        let _ = self.next_token(); // consume '('
        self.eat_newlines();

        // Parse parameters
        let mut params = Vec::new();
        let mut defaults = Vec::new();
        let mut rest_param = None;

        while !self.cur_token_is(TokenKind::RParen) && !self.cur_token_is(TokenKind::Eof) {
            // Check for rest parameter
            if self.cur_token_is(TokenKind::Spread) {
                let _ = self.next_token(); // consume '...'
                if !self.cur_token_is(TokenKind::Ident) {
                    self.errors
                        .push(ParserError::new("expected identifier", self.cur_token.start));
                    return None;
                }
                rest_param = Some(Ident {
                    position: self.cur_token.start,
                    name: self.cur_token.literal.clone(),
                });
                let _ = self.next_token();
                break; // rest param must be last
            }

            // Check for destructuring parameters
            if self.cur_token_is(TokenKind::LBrace) {
                let param = self.parse_object_destructure_param()?;
                params.push(param);
            } else if self.cur_token_is(TokenKind::LBracket) {
                let param = self.parse_array_destructure_param()?;
                params.push(param);
            } else if self.cur_token_is(TokenKind::Ident) {
                let param = Ident {
                    position: self.cur_token.start,
                    name: self.cur_token.literal.clone(),
                };
                let _ = self.next_token();

                // Check for default value
                if self.cur_token_is(TokenKind::Assign) {
                    let _ = self.next_token(); // consume '='
                    let default_val = self.parse_expression(Precedence::Lowest)?;
                    defaults.push((param.name.clone(), default_val));
                }

                params.push(FuncParam::Ident(param));
            } else {
                self.errors
                    .push(ParserError::new("expected parameter", self.cur_token.start));
                return None;
            }

            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token(); // consume ','
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RParen) {
            self.errors
                .push(ParserError::new("expected ')'", self.cur_token.start));
            return None;
        }
        let rparen = self.cur_token.start;
        let _ = self.next_token(); // consume ')'

        let body = self.parse_block()?;

        Some(Expr::Func(Rc::new(FuncLit {
            func_pos,
            name,
            lparen,
            params,
            defaults,
            rest_param,
            rparen,
            body,
        })))
    }

    fn parse_object_destructure_param(&mut self) -> Option<FuncParam> {
        let lbrace = self.cur_token.start;
        let _ = self.next_token(); // consume '{'
        self.eat_newlines();

        let mut bindings = Vec::new();
        while !self.cur_token_is(TokenKind::RBrace) && !self.cur_token_is(TokenKind::Eof) {
            if !self.cur_token_is(TokenKind::Ident) {
                self.errors
                    .push(ParserError::new("expected identifier", self.cur_token.start));
                return None;
            }
            let key = self.cur_token.literal.clone();
            let mut alias = None;
            let mut default_value = None;
            let _ = self.next_token();

            if self.cur_token_is(TokenKind::Colon) {
                let _ = self.next_token();
                if !self.cur_token_is(TokenKind::Ident) {
                    self.errors
                        .push(ParserError::new("expected identifier", self.cur_token.start));
                    return None;
                }
                alias = Some(self.cur_token.literal.clone());
                let _ = self.next_token();
            }

            if self.cur_token_is(TokenKind::Assign) {
                let _ = self.next_token();
                default_value = Some(self.parse_expression(Precedence::Lowest)?);
            }

            bindings.push(DestructureBinding {
                key,
                alias,
                default_value,
            });

            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token();
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RBrace) {
            self.errors
                .push(ParserError::new("expected '}'", self.cur_token.start));
            return None;
        }
        let rbrace = self.cur_token.start;
        let _ = self.next_token();

        Some(FuncParam::ObjectDestructure {
            lbrace,
            bindings,
            rbrace,
        })
    }

    fn parse_array_destructure_param(&mut self) -> Option<FuncParam> {
        let lbrack = self.cur_token.start;
        let _ = self.next_token(); // consume '['
        self.eat_newlines();

        let mut elements = Vec::new();
        while !self.cur_token_is(TokenKind::RBracket) && !self.cur_token_is(TokenKind::Eof) {
            if !self.cur_token_is(TokenKind::Ident) {
                self.errors
                    .push(ParserError::new("expected identifier", self.cur_token.start));
                return None;
            }
            let name = Ident {
                position: self.cur_token.start,
                name: self.cur_token.literal.clone(),
            };
            let mut default_value = None;
            let _ = self.next_token();

            if self.cur_token_is(TokenKind::Assign) {
                let _ = self.next_token();
                default_value = Some(self.parse_expression(Precedence::Lowest)?);
            }

            elements.push(ArrayDestructureElement {
                name,
                default_value,
            });

            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token();
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RBracket) {
            self.errors
                .push(ParserError::new("expected ']'", self.cur_token.start));
            return None;
        }
        let rbrack = self.cur_token.start;
        let _ = self.next_token();

        Some(FuncParam::ArrayDestructure {
            lbrack,
            elements,
            rbrack,
        })
    }

    // =========================================================================
    // Block Parsing
    // =========================================================================

    fn parse_block(&mut self) -> Option<Block> {
        if !self.cur_token_is(TokenKind::LBrace) {
            self.errors
                .push(ParserError::new("expected '{'", self.cur_token.start));
            return None;
        }
        let lbrace = self.cur_token.start;
        let _ = self.next_token(); // consume '{'
        self.eat_newlines();

        let mut stmts = Vec::new();
        while !self.cur_token_is(TokenKind::RBrace) && !self.cur_token_is(TokenKind::Eof) {
            if let Some(stmt) = self.parse_statement() {
                stmts.push(stmt);
            }
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RBrace) {
            self.errors
                .push(ParserError::new("expected '}'", self.cur_token.start));
            return None;
        }
        let rbrace = self.cur_token.start;
        let _ = self.next_token(); // consume '}'

        Some(Block {
            lbrace,
            stmts,
            rbrace,
        })
    }

    // =========================================================================
    // Access Expressions
    // =========================================================================

    fn parse_call(&mut self, func: Expr) -> Option<Expr> {
        let lparen = self.cur_token.start;
        let _ = self.next_token(); // consume '('
        self.eat_newlines();

        let mut args = Vec::new();
        while !self.cur_token_is(TokenKind::RParen) && !self.cur_token_is(TokenKind::Eof) {
            let arg = self.parse_expression(Precedence::Lowest)?;
            args.push(arg);

            if !self.cur_token_is(TokenKind::Comma) {
                break;
            }
            let _ = self.next_token(); // consume ','
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RParen) {
            self.errors
                .push(ParserError::new("expected ')'", self.cur_token.start));
            return None;
        }
        let rparen = self.cur_token.start;
        let _ = self.next_token(); // consume ')'

        Some(Expr::Call(Box::new(CallExpr {
            func,
            lparen,
            args,
            rparen,
        })))
    }

    fn parse_index(&mut self, left: Expr) -> Option<Expr> {
        let lbrack = self.cur_token.start;
        let _ = self.next_token(); // consume '['
        self.eat_newlines();

        // Check for slice [:high]
        if self.cur_token_is(TokenKind::Colon) {
            let _ = self.next_token(); // consume ':'
            self.eat_newlines();
            let high = if !self.cur_token_is(TokenKind::RBracket) {
                Some(self.parse_expression(Precedence::Lowest)?)
            } else {
                None
            };
            if !self.cur_token_is(TokenKind::RBracket) {
                self.errors
                    .push(ParserError::new("expected ']'", self.cur_token.start));
                return None;
            }
            let rbrack = self.cur_token.start;
            let _ = self.next_token();
            return Some(Expr::Slice(Box::new(SliceExpr {
                object: left,
                lbrack,
                low: None,
                high,
                rbrack,
            })));
        }

        let index = self.parse_expression(Precedence::Lowest)?;

        // Check for slice [low:high]
        if self.cur_token_is(TokenKind::Colon) {
            let _ = self.next_token(); // consume ':'
            self.eat_newlines();
            let high = if !self.cur_token_is(TokenKind::RBracket) {
                Some(self.parse_expression(Precedence::Lowest)?)
            } else {
                None
            };
            if !self.cur_token_is(TokenKind::RBracket) {
                self.errors
                    .push(ParserError::new("expected ']'", self.cur_token.start));
                return None;
            }
            let rbrack = self.cur_token.start;
            let _ = self.next_token();
            return Some(Expr::Slice(Box::new(SliceExpr {
                object: left,
                lbrack,
                low: Some(index),
                high,
                rbrack,
            })));
        }

        if !self.cur_token_is(TokenKind::RBracket) {
            self.errors
                .push(ParserError::new("expected ']'", self.cur_token.start));
            return None;
        }
        let rbrack = self.cur_token.start;
        let _ = self.next_token(); // consume ']'

        Some(Expr::Index(Box::new(IndexExpr {
            object: left,
            lbrack,
            index,
            rbrack,
        })))
    }

    fn parse_get_attr(&mut self, left: Expr) -> Option<Expr> {
        let period = self.cur_token.start;
        let _ = self.next_token(); // consume '.'

        if !self.cur_token_is(TokenKind::Ident) {
            self.errors
                .push(ParserError::new("expected identifier", self.cur_token.start));
            return None;
        }
        let attr = Ident {
            position: self.cur_token.start,
            name: self.cur_token.literal.clone(),
        };
        let _ = self.next_token();

        // Check for method call
        if self.cur_token_is(TokenKind::LParen) {
            let lparen = self.cur_token.start;
            let _ = self.next_token(); // consume '('
            self.eat_newlines();

            let mut args = Vec::new();
            while !self.cur_token_is(TokenKind::RParen) && !self.cur_token_is(TokenKind::Eof) {
                let arg = self.parse_expression(Precedence::Lowest)?;
                args.push(arg);

                if !self.cur_token_is(TokenKind::Comma) {
                    break;
                }
                let _ = self.next_token();
                self.eat_newlines();
            }

            if !self.cur_token_is(TokenKind::RParen) {
                self.errors
                    .push(ParserError::new("expected ')'", self.cur_token.start));
                return None;
            }
            let rparen = self.cur_token.start;
            let _ = self.next_token();

            let call = CallExpr {
                func: Expr::Ident(attr),
                lparen,
                args,
                rparen,
            };
            return Some(Expr::ObjectCall(Box::new(ObjectCallExpr {
                object: left,
                period,
                call,
                optional: false,
            })));
        }

        Some(Expr::GetAttr(Box::new(GetAttrExpr {
            object: left,
            period,
            attr,
            optional: false,
        })))
    }

    fn parse_optional_chain(&mut self, left: Expr) -> Option<Expr> {
        let period = self.cur_token.start;
        let _ = self.next_token(); // consume '?.'

        if !self.cur_token_is(TokenKind::Ident) {
            self.errors
                .push(ParserError::new("expected identifier", self.cur_token.start));
            return None;
        }
        let attr = Ident {
            position: self.cur_token.start,
            name: self.cur_token.literal.clone(),
        };
        let _ = self.next_token();

        // Check for method call
        if self.cur_token_is(TokenKind::LParen) {
            let lparen = self.cur_token.start;
            let _ = self.next_token(); // consume '('
            self.eat_newlines();

            let mut args = Vec::new();
            while !self.cur_token_is(TokenKind::RParen) && !self.cur_token_is(TokenKind::Eof) {
                let arg = self.parse_expression(Precedence::Lowest)?;
                args.push(arg);

                if !self.cur_token_is(TokenKind::Comma) {
                    break;
                }
                let _ = self.next_token();
                self.eat_newlines();
            }

            if !self.cur_token_is(TokenKind::RParen) {
                self.errors
                    .push(ParserError::new("expected ')'", self.cur_token.start));
                return None;
            }
            let rparen = self.cur_token.start;
            let _ = self.next_token();

            let call = CallExpr {
                func: Expr::Ident(attr),
                lparen,
                args,
                rparen,
            };
            return Some(Expr::ObjectCall(Box::new(ObjectCallExpr {
                object: left,
                period,
                call,
                optional: true,
            })));
        }

        Some(Expr::GetAttr(Box::new(GetAttrExpr {
            object: left,
            period,
            attr,
            optional: true,
        })))
    }

    // =========================================================================
    // Control Flow
    // =========================================================================

    fn parse_if(&mut self) -> Option<Expr> {
        let if_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'if'

        // Optional parentheses around condition
        let has_paren = self.cur_token_is(TokenKind::LParen);
        if has_paren {
            let _ = self.next_token();
        }

        let condition = self.parse_expression(Precedence::Lowest)?;

        if has_paren {
            if !self.cur_token_is(TokenKind::RParen) {
                self.errors
                    .push(ParserError::new("expected ')'", self.cur_token.start));
                return None;
            }
            let _ = self.next_token();
        }

        let consequence = self.parse_block()?;

        let alternative = if self.cur_token_is(TokenKind::Else) {
            let _ = self.next_token(); // consume 'else'

            // else if
            if self.cur_token_is(TokenKind::If) {
                let else_if = self.parse_if()?;
                let else_if_pos = else_if.pos();
                let else_if_end = else_if.end();
                Some(Block {
                    lbrace: else_if_pos,
                    stmts: vec![Stmt::Expr(else_if)],
                    rbrace: else_if_end,
                })
            } else {
                Some(self.parse_block()?)
            }
        } else {
            None
        };

        Some(Expr::If(Box::new(IfExpr {
            if_pos,
            condition,
            consequence,
            alternative,
        })))
    }

    fn parse_switch(&mut self) -> Option<Expr> {
        let switch_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'switch'

        // Parentheses around value
        if !self.cur_token_is(TokenKind::LParen) {
            self.errors
                .push(ParserError::new("expected '('", self.cur_token.start));
            return None;
        }
        let _ = self.next_token();

        let value = self.parse_expression(Precedence::Lowest)?;

        if !self.cur_token_is(TokenKind::RParen) {
            self.errors
                .push(ParserError::new("expected ')'", self.cur_token.start));
            return None;
        }
        let _ = self.next_token();

        if !self.cur_token_is(TokenKind::LBrace) {
            self.errors
                .push(ParserError::new("expected '{'", self.cur_token.start));
            return None;
        }
        let lbrace = self.cur_token.start;
        let _ = self.next_token();
        self.eat_newlines();

        let mut cases = Vec::new();
        while !self.cur_token_is(TokenKind::RBrace) && !self.cur_token_is(TokenKind::Eof) {
            let clause = self.parse_case_clause()?;
            cases.push(clause);
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RBrace) {
            self.errors
                .push(ParserError::new("expected '}'", self.cur_token.start));
            return None;
        }
        let rbrace = self.cur_token.start;
        let _ = self.next_token();

        Some(Expr::Switch(Box::new(SwitchExpr {
            switch_pos,
            value,
            lbrace,
            cases,
            rbrace,
        })))
    }

    fn parse_case_clause(&mut self) -> Option<CaseClause> {
        let case_pos = self.cur_token.start;
        let is_default = self.cur_token_is(TokenKind::Default);

        if !self.cur_token_is(TokenKind::Case) && !self.cur_token_is(TokenKind::Default) {
            self.errors.push(ParserError::new(
                "expected 'case' or 'default'",
                self.cur_token.start,
            ));
            return None;
        }
        let _ = self.next_token();

        let exprs = if !is_default {
            let mut exprs = Vec::new();
            let expr = self.parse_expression(Precedence::Lowest)?;
            exprs.push(expr);

            while self.cur_token_is(TokenKind::Comma) {
                let _ = self.next_token();
                let e = self.parse_expression(Precedence::Lowest)?;
                exprs.push(e);
            }
            Some(exprs)
        } else {
            None
        };

        if !self.cur_token_is(TokenKind::Colon) {
            self.errors
                .push(ParserError::new("expected ':'", self.cur_token.start));
            return None;
        }
        let colon = self.cur_token.start;
        let _ = self.next_token();
        self.eat_newlines();

        // Parse case body statements until next case/default/rbrace
        let mut stmts = Vec::new();
        while !self.cur_token_is(TokenKind::Case)
            && !self.cur_token_is(TokenKind::Default)
            && !self.cur_token_is(TokenKind::RBrace)
            && !self.cur_token_is(TokenKind::Eof)
        {
            if let Some(stmt) = self.parse_statement() {
                stmts.push(stmt);
            }
            self.eat_newlines();
        }

        let body = Block {
            lbrace: colon,
            stmts,
            rbrace: self.cur_token.start,
        };
        Some(CaseClause {
            case_pos,
            exprs,
            colon,
            body,
            is_default,
        })
    }

    fn parse_match(&mut self) -> Option<Expr> {
        let match_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'match'

        let subject = self.parse_expression(Precedence::Lowest)?;

        if !self.cur_token_is(TokenKind::LBrace) {
            self.errors
                .push(ParserError::new("expected '{'", self.cur_token.start));
            return None;
        }
        let lbrace = self.cur_token.start;
        let _ = self.next_token();
        self.eat_newlines();

        let mut arms = Vec::new();
        let mut default_arm = None;

        while !self.cur_token_is(TokenKind::RBrace) && !self.cur_token_is(TokenKind::Eof) {
            let arm = self.parse_match_arm()?;

            if matches!(arm.pattern, Pattern::Wildcard(_)) {
                default_arm = Some(arm);
            } else {
                arms.push(arm);
            }

            if self.cur_token_is(TokenKind::Comma) {
                let _ = self.next_token();
            }
            self.eat_newlines();
        }

        if !self.cur_token_is(TokenKind::RBrace) {
            self.errors
                .push(ParserError::new("expected '}'", self.cur_token.start));
            return None;
        }
        let rbrace = self.cur_token.start;
        let _ = self.next_token();

        Some(Expr::Match(Box::new(MatchExpr {
            match_pos,
            subject,
            lbrace,
            arms,
            default_arm,
            rbrace,
        })))
    }

    fn parse_match_arm(&mut self) -> Option<MatchArm> {
        // Parse pattern
        let pattern = if self.cur_token_is(TokenKind::Ident) && self.cur_token.literal == "_" {
            let pos = self.cur_token.start;
            let _ = self.next_token();
            Pattern::Wildcard(pos)
        } else {
            let expr = self.parse_expression(Precedence::Lowest)?;
            Pattern::Literal(expr)
        };

        // Optional guard (if condition)
        let guard = if self.cur_token_is(TokenKind::If) {
            let _ = self.next_token();
            Some(self.parse_expression(Precedence::Lowest)?)
        } else {
            None
        };

        if !self.cur_token_is(TokenKind::Arrow) {
            self.errors
                .push(ParserError::new("expected '=>'", self.cur_token.start));
            return None;
        }
        let arrow = self.cur_token.start;
        let _ = self.next_token();
        self.eat_newlines();

        let result = self.parse_expression(Precedence::Lowest)?;

        Some(MatchArm {
            pattern,
            guard,
            arrow,
            result,
        })
    }

    // =========================================================================
    // Membership and Pipe
    // =========================================================================

    fn parse_in(&mut self, left: Expr) -> Option<Expr> {
        let in_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'in'

        let right = self.parse_expression(Precedence::LessGreater)?;

        Some(Expr::In(Box::new(InExpr {
            left,
            in_pos,
            right,
        })))
    }

    fn parse_not_in(&mut self, left: Expr) -> Option<Expr> {
        let not_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'not'

        if !self.cur_token_is(TokenKind::In) {
            self.errors
                .push(ParserError::new("expected 'in'", self.cur_token.start));
            return None;
        }
        let _ = self.next_token(); // consume 'in'

        let right = self.parse_expression(Precedence::LessGreater)?;

        Some(Expr::NotIn(Box::new(NotInExpr {
            left,
            not_pos,
            right,
        })))
    }

    fn parse_pipe_infix(&mut self, left: Expr) -> Option<Expr> {
        let mut exprs = vec![left];

        while self.cur_token_is(TokenKind::Pipe) {
            let _ = self.next_token(); // consume '|'
            self.eat_newlines();
            // Parse with Pipe + 1 precedence for left-associativity
            let right = self.parse_expression(Precedence::Cond)?; // Cond is just above Pipe
            exprs.push(right);
        }

        Some(Expr::Pipe(PipeExpr { exprs }))
    }

    // =========================================================================
    // Try/Catch/Finally
    // =========================================================================

    fn parse_try(&mut self) -> Option<Expr> {
        let try_pos = self.cur_token.start;
        let _ = self.next_token(); // consume 'try'

        let body = self.parse_block()?;

        let mut catch_ident = None;
        let mut catch_block = None;
        let mut finally_block = None;

        if self.cur_token_is(TokenKind::Catch) {
            let _ = self.next_token(); // consume 'catch'

            // Optional catch variable
            if self.cur_token_is(TokenKind::Ident) {
                catch_ident = Some(Ident {
                    position: self.cur_token.start,
                    name: self.cur_token.literal.clone(),
                });
                let _ = self.next_token();
            }

            catch_block = Some(self.parse_block()?);
        }

        if self.cur_token_is(TokenKind::Finally) {
            let _ = self.next_token(); // consume 'finally'
            finally_block = Some(self.parse_block()?);
        }

        if catch_block.is_none() && finally_block.is_none() {
            self.errors.push(ParserError::new(
                "try requires catch or finally",
                try_pos,
            ));
            return None;
        }

        Some(Expr::Try(Box::new(TryExpr {
            try_pos,
            body,
            catch_ident,
            catch_block,
            finally_block,
        })))
    }
}

/// Parse source code into an AST.
pub fn parse(source: &str) -> Result<Program, ParserError> {
    let lexer = Lexer::new(source);
    let mut parser = Parser::new(lexer)?;
    parser.parse()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse_ok(source: &str) -> Program {
        parse(source).expect(&format!("Failed to parse: {}", source))
    }

    fn parse_expr(source: &str) -> Expr {
        let prog = parse_ok(source);
        assert_eq!(prog.stmts.len(), 1);
        match &prog.stmts[0] {
            Stmt::Expr(expr) => expr.clone(),
            other => panic!("Expected expression statement, got {:?}", other),
        }
    }

    // =========================================================================
    // Literal Tests
    // =========================================================================

    #[test]
    fn test_integers() {
        let expr = parse_expr("42");
        assert!(matches!(expr, Expr::Int(IntLit { value: 42, .. })));
    }

    #[test]
    fn test_hex_integers() {
        let expr = parse_expr("0xFF");
        assert!(matches!(expr, Expr::Int(IntLit { value: 255, .. })));
    }

    #[test]
    fn test_binary_integers() {
        let expr = parse_expr("0b1010");
        assert!(matches!(expr, Expr::Int(IntLit { value: 10, .. })));
    }

    #[test]
    fn test_floats() {
        let expr = parse_expr("3.14");
        if let Expr::Float(FloatLit { value, .. }) = expr {
            assert!((value - 3.14).abs() < 0.001);
        } else {
            panic!("Expected float");
        }
    }

    #[test]
    fn test_strings() {
        let expr = parse_expr(r#""hello""#);
        assert!(matches!(
            expr,
            Expr::String(StringLit { value, .. }) if value == "hello"
        ));
    }

    #[test]
    fn test_booleans() {
        let expr = parse_expr("true");
        assert!(matches!(expr, Expr::Bool(BoolLit { value: true, .. })));

        let expr = parse_expr("false");
        assert!(matches!(expr, Expr::Bool(BoolLit { value: false, .. })));
    }

    #[test]
    fn test_nil() {
        let expr = parse_expr("nil");
        assert!(matches!(expr, Expr::Nil(_)));
    }

    #[test]
    fn test_identifier() {
        let expr = parse_expr("foo");
        assert!(matches!(expr, Expr::Ident(Ident { name, .. }) if name == "foo"));
    }

    // =========================================================================
    // Operator Tests
    // =========================================================================

    #[test]
    fn test_prefix_operators() {
        let expr = parse_expr("-42");
        assert!(matches!(expr, Expr::Prefix(_)));

        let expr = parse_expr("!true");
        assert!(matches!(expr, Expr::Prefix(_)));
    }

    #[test]
    fn test_infix_operators() {
        let expr = parse_expr("1 + 2");
        assert!(matches!(expr, Expr::Infix(_)));

        let expr = parse_expr("3 * 4");
        assert!(matches!(expr, Expr::Infix(_)));
    }

    #[test]
    fn test_precedence() {
        let expr = parse_expr("1 + 2 * 3");
        // Should parse as 1 + (2 * 3)
        if let Expr::Infix(infix) = expr {
            assert_eq!(infix.op, "+");
            assert!(matches!(infix.right, Expr::Infix(_)));
        } else {
            panic!("Expected infix");
        }
    }

    #[test]
    fn test_power_right_associative() {
        let expr = parse_expr("2 ** 3 ** 4");
        // Should parse as 2 ** (3 ** 4)
        if let Expr::Infix(infix) = expr {
            assert_eq!(infix.op, "**");
            assert!(matches!(infix.right, Expr::Infix(_)));
        } else {
            panic!("Expected infix");
        }
    }

    // =========================================================================
    // Collection Tests
    // =========================================================================

    #[test]
    fn test_list() {
        let expr = parse_expr("[1, 2, 3]");
        if let Expr::List(list) = expr {
            assert_eq!(list.items.len(), 3);
        } else {
            panic!("Expected list");
        }
    }

    #[test]
    fn test_empty_list() {
        let expr = parse_expr("[]");
        if let Expr::List(list) = expr {
            assert_eq!(list.items.len(), 0);
        } else {
            panic!("Expected list");
        }
    }

    #[test]
    fn test_map() {
        let expr = parse_expr("{a: 1, b: 2}");
        if let Expr::Map(map) = expr {
            assert_eq!(map.items.len(), 2);
        } else {
            panic!("Expected map");
        }
    }

    #[test]
    fn test_empty_map() {
        let expr = parse_expr("{}");
        if let Expr::Map(map) = expr {
            assert_eq!(map.items.len(), 0);
        } else {
            panic!("Expected map");
        }
    }

    #[test]
    fn test_newlines_in_collections() {
        let expr = parse_expr("[\n  1,\n  2,\n  3\n]");
        if let Expr::List(list) = expr {
            assert_eq!(list.items.len(), 3);
        } else {
            panic!("Expected list");
        }
    }

    // =========================================================================
    // Function Tests
    // =========================================================================

    #[test]
    fn test_function() {
        let expr = parse_expr("function(x) { return x }");
        assert!(matches!(expr, Expr::Func(_)));
    }

    #[test]
    fn test_named_function() {
        let expr = parse_expr("function add(a, b) { return a + b }");
        if let Expr::Func(func) = expr {
            assert!(func.name.is_some());
            assert_eq!(func.name.as_ref().unwrap().name, "add");
        } else {
            panic!("Expected function");
        }
    }

    #[test]
    fn test_arrow_function() {
        let expr = parse_expr("x => x * 2");
        assert!(matches!(expr, Expr::Func(_)));
    }

    #[test]
    fn test_arrow_function_parens() {
        let expr = parse_expr("(x) => x * 2");
        assert!(matches!(expr, Expr::Func(_)));
    }

    #[test]
    fn test_arrow_function_multi_params() {
        let expr = parse_expr("(x, y) => x + y");
        if let Expr::Func(func) = expr {
            assert_eq!(func.params.len(), 2);
        } else {
            panic!("Expected function");
        }
    }

    #[test]
    fn test_arrow_function_no_params() {
        let expr = parse_expr("() => 42");
        if let Expr::Func(func) = expr {
            assert_eq!(func.params.len(), 0);
        } else {
            panic!("Expected function");
        }
    }

    // =========================================================================
    // Statement Tests
    // =========================================================================

    #[test]
    fn test_let_statement() {
        let prog = parse_ok("let x = 42");
        assert_eq!(prog.stmts.len(), 1);
        assert!(matches!(prog.stmts[0], Stmt::Var(_)));
    }

    #[test]
    fn test_const_statement() {
        let prog = parse_ok("const x = 42");
        assert_eq!(prog.stmts.len(), 1);
        assert!(matches!(prog.stmts[0], Stmt::Const(_)));
    }

    #[test]
    fn test_return_statement() {
        let prog = parse_ok("return 42");
        assert!(matches!(prog.stmts[0], Stmt::Return(_)));
    }

    #[test]
    fn test_empty_return() {
        let prog = parse_ok("return");
        if let Stmt::Return(ret) = &prog.stmts[0] {
            assert!(ret.value.is_none());
        } else {
            panic!("Expected return");
        }
    }

    #[test]
    fn test_assignment() {
        let prog = parse_ok("x = 42");
        assert!(matches!(prog.stmts[0], Stmt::Assign(_)));
    }

    #[test]
    fn test_compound_assignment() {
        let prog = parse_ok("x += 1");
        if let Stmt::Assign(assign) = &prog.stmts[0] {
            assert_eq!(assign.op, "+=");
        } else {
            panic!("Expected assignment");
        }
    }

    #[test]
    fn test_postfix() {
        let prog = parse_ok("x++");
        assert!(matches!(prog.stmts[0], Stmt::Postfix(_)));
    }

    // =========================================================================
    // Access Expression Tests
    // =========================================================================

    #[test]
    fn test_function_call() {
        let expr = parse_expr("foo(1, 2)");
        assert!(matches!(expr, Expr::Call(_)));
    }

    #[test]
    fn test_index() {
        let expr = parse_expr("arr[0]");
        assert!(matches!(expr, Expr::Index(_)));
    }

    #[test]
    fn test_slice() {
        let expr = parse_expr("arr[1:3]");
        assert!(matches!(expr, Expr::Slice(_)));
    }

    #[test]
    fn test_get_attr() {
        let expr = parse_expr("obj.prop");
        assert!(matches!(expr, Expr::GetAttr(_)));
    }

    #[test]
    fn test_method_call() {
        let expr = parse_expr("obj.method()");
        assert!(matches!(expr, Expr::ObjectCall(_)));
    }

    #[test]
    fn test_optional_chain() {
        let expr = parse_expr("obj?.prop");
        if let Expr::GetAttr(get_attr) = expr {
            assert!(get_attr.optional);
        } else {
            panic!("Expected get attr");
        }
    }

    // =========================================================================
    // Control Flow Tests
    // =========================================================================

    #[test]
    fn test_if_expression() {
        let expr = parse_expr("if true { 1 }");
        assert!(matches!(expr, Expr::If(_)));
    }

    #[test]
    fn test_if_else() {
        let expr = parse_expr("if true { 1 } else { 2 }");
        if let Expr::If(if_expr) = expr {
            assert!(if_expr.alternative.is_some());
        } else {
            panic!("Expected if");
        }
    }

    #[test]
    fn test_switch() {
        let expr = parse_expr("switch (x) { case 1: a }");
        assert!(matches!(expr, Expr::Switch(_)));
    }

    #[test]
    fn test_match() {
        let expr = parse_expr("match x { 1 => \"one\", _ => \"other\" }");
        assert!(matches!(expr, Expr::Match(_)));
    }

    // =========================================================================
    // Membership Tests
    // =========================================================================

    #[test]
    fn test_in() {
        let expr = parse_expr("x in list");
        assert!(matches!(expr, Expr::In(_)));
    }

    #[test]
    fn test_not_in() {
        let expr = parse_expr("x not in list");
        assert!(matches!(expr, Expr::NotIn(_)));
    }

    // =========================================================================
    // Pipe Tests
    // =========================================================================

    #[test]
    fn test_pipe() {
        let expr = parse_expr("a | b | c");
        if let Expr::Pipe(pipe) = expr {
            assert_eq!(pipe.exprs.len(), 3);
        } else {
            panic!("Expected pipe");
        }
    }

    // =========================================================================
    // Try/Catch Tests
    // =========================================================================

    #[test]
    fn test_try_catch() {
        let expr = parse_expr("try { a } catch { b }");
        assert!(matches!(expr, Expr::Try(_)));
    }

    #[test]
    fn test_try_catch_finally() {
        let expr = parse_expr("try { a } catch e { b } finally { c }");
        if let Expr::Try(try_expr) = expr {
            assert!(try_expr.catch_block.is_some());
            assert!(try_expr.catch_ident.is_some());
            assert!(try_expr.finally_block.is_some());
        } else {
            panic!("Expected try");
        }
    }

    // =========================================================================
    // Destructuring Tests
    // =========================================================================

    #[test]
    fn test_object_destructure() {
        let prog = parse_ok("let { a, b } = obj");
        assert!(matches!(prog.stmts[0], Stmt::ObjectDestructure(_)));
    }

    #[test]
    fn test_array_destructure() {
        let prog = parse_ok("let [a, b] = arr");
        assert!(matches!(prog.stmts[0], Stmt::ArrayDestructure(_)));
    }

    #[test]
    fn test_multi_var() {
        let prog = parse_ok("let x, y = [1, 2]");
        assert!(matches!(prog.stmts[0], Stmt::MultiVar(_)));
    }

    // =========================================================================
    // Error Recovery Tests
    // =========================================================================

    #[test]
    fn test_error_recovery() {
        // Invalid statement should not crash, and valid statement should still parse
        let result = parse("let = 42\nlet x = 1");
        assert!(result.is_err()); // Should report the first error
    }

    #[test]
    fn test_multiple_statements() {
        let prog = parse_ok("let x = 1\nlet y = 2");
        assert_eq!(prog.stmts.len(), 2);
    }

    // =========================================================================
    // Spread Tests
    // =========================================================================

    #[test]
    fn test_spread_in_list() {
        let expr = parse_expr("[1, ...arr, 2]");
        if let Expr::List(list) = expr {
            assert_eq!(list.items.len(), 3);
            assert!(matches!(list.items[1], Expr::Spread(_)));
        } else {
            panic!("Expected list");
        }
    }

    #[test]
    fn test_spread_in_map() {
        let expr = parse_expr("{a: 1, ...other}");
        if let Expr::Map(map) = expr {
            assert_eq!(map.items.len(), 2);
        } else {
            panic!("Expected map");
        }
    }
}
