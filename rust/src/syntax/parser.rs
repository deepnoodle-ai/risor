//! The parser (`01-language.md` section 3), written by hand.
//!
//! Binary operators are read by precedence climbing, a form of Pratt parsing.
//! The parser stops at its first diagnostic: only the first is specified
//! (14.1), and `03-host.md` 3.7.2 lets a parser stop there.
//!
//! **The native stack.** The parser recurses only where the text opens a
//! level (13.1), and at most 100 levels are open, so its depth is bounded.
//! What opens no level is read by a loop: the operands of a left-associative
//! operator, the steps of a chain, the `else` of `? :`, an `else if` chain and
//! `const` lines.

use super::ast::*;
use super::lexer::lex;
use super::token::{is_name, is_reserved, StrForm, Tok, Token, P};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::source::{Source, Span};

/// At most this many levels may be open at once (13.2).
pub const MAX_LEVELS: u32 = 100;

/// A parsed program.
#[derive(Clone, Debug)]
pub struct Parsed {
    pub source: Source,
    pub ast: Ast,
}

/// Parse the text of a program. A failure has phase `parse` and one
/// diagnostic: the first.
pub fn parse(text: &str) -> Result<Parsed, Error> {
    let source = Source::new(text);
    let ast = {
        let toks = lex(&source).map_err(Error::one)?;
        let mut p = Parser::new(&source, &toks);
        let root = p.program().map_err(|d| Error::one(*d))?;
        p.ast.root = root;
        p.ast
    };
    Ok(Parsed { source, ast })
}

/// Parse the parts of a template (F2 2.2, 2.3). `toks` holds each part's
/// tokens followed by its end: an `Eof` token with the span of the closing
/// delimiter, or the lone closing brace and then an `Eof`. `parts` gives, for
/// each part, the index of its first token and the span of its `${{`, which
/// opens one level. Returns the arena with each part's expression.
pub(crate) fn parse_parts(
    src: &Source,
    toks: &[Token],
    parts: &[(usize, Span)],
) -> Result<(Ast, Vec<NodeId>), Diagnostic> {
    let mut p = Parser::new(src, toks);
    let mut ids = Vec::new();
    for &(start, open) in parts {
        p.pos = start;
        p.open(open).map_err(|d| *d)?;
        let e = p.expr().map_err(|d| *d)?;
        if !matches!(p.peek().tok, Tok::Eof) {
            return Err(*p.unexpected(p.peek()));
        }
        p.close();
        ids.push(e);
    }
    Ok((p.ast, ids))
}

/// A type name (F4 2.1 item 2): an upper-case letter, then one or more of
/// `[A-Za-z0-9_]`.
pub fn is_type_name(w: &str) -> bool {
    let b = w.as_bytes();
    b.len() >= 2
        && b[0].is_ascii_uppercase()
        && b[1..]
            .iter()
            .all(|c| c.is_ascii_alphanumeric() || *c == b'_')
}

/// The error is boxed: a large `Err` makes every `?` cost stack in a build
/// with no optimization, and the parser recurses once for each level.
type R<T> = Result<T, Box<Diagnostic>>;

const PREC_OR: u8 = 3;
const PREC_REL: u8 = 6;
const PREC_ADD: u8 = 7;

struct Parser<'a> {
    src: &'a Source,
    t: &'a [Token],
    pos: usize,
    level: u32,
    ast: Ast,
    /// For each opener (`(`, `[`, `?.[`, `{`), the index of its matching
    /// closer by the count of 3.2 rule 6.
    partner: Vec<Option<u32>>,
    /// For each template head, the index of its tail.
    tmpl_end: Vec<Option<u32>>,
    /// The levels open when each token was read.
    tok_level: Vec<u16>,
    /// The greatest level opened since it was last reset.
    max_level: u32,
}

impl<'a> Parser<'a> {
    fn new(src: &'a Source, t: &'a [Token]) -> Parser<'a> {
        // 3.2 rule 6: a template with all of its interpolations is one
        // token for the count outside it. Inside, each interpolation counts
        // its own openers and closers.
        let mut partner = vec![None; t.len()];
        let mut tmpl_end = vec![None; t.len()];
        let mut stacks: Vec<Vec<usize>> = vec![Vec::new()];
        let mut heads: Vec<usize> = Vec::new();
        for (i, tok) in t.iter().enumerate() {
            match tok.tok {
                Tok::Punct(P::LParen | P::LBracket | P::QDotBracket | P::LBrace) => {
                    stacks.last_mut().expect("a stack").push(i)
                }
                Tok::Punct(P::RParen | P::RBracket | P::RBrace) => {
                    if let Some(o) = stacks.last_mut().expect("a stack").pop() {
                        partner[o] = Some(i as u32);
                    }
                }
                Tok::TmplHead(_) => {
                    stacks.push(Vec::new());
                    heads.push(i);
                }
                Tok::TmplMiddle(_) => stacks.last_mut().expect("a stack").clear(),
                // The end of a template's part (F2): each part counts alone.
                Tok::Eof => {
                    stacks = vec![Vec::new()];
                    heads.clear();
                }
                Tok::TmplTail(_) if stacks.len() > 1 => {
                    stacks.pop();
                    if let Some(h) = heads.pop() {
                        tmpl_end[h] = Some(i as u32);
                    }
                }
                _ => {}
            }
        }
        Parser {
            src,
            t,
            pos: 0,
            level: 0,
            ast: Ast::default(),
            partner,
            tmpl_end,
            tok_level: vec![0; t.len()],
            max_level: 0,
        }
    }

    // ---- tokens -------------------------------------------------------

    fn peek(&self) -> &'a Token {
        &self.t[self.pos]
    }

    fn peek_n(&self, n: usize) -> &'a Token {
        &self.t[(self.pos + n).min(self.t.len() - 1)]
    }

    fn at_punct(&self, p: P) -> bool {
        self.peek().is_punct(p)
    }

    fn at_word(&self, w: &str) -> bool {
        self.peek().is_word(w)
    }

    fn bump(&mut self) -> &'a Token {
        let tok = &self.t[self.pos];
        self.tok_level[self.pos] = self.level as u16;
        if self.pos + 1 < self.t.len() {
            self.pos += 1;
        }
        tok
    }

    /// Consume a word. The word `r` or `R` followed by white space and then a
    /// one-line string is `unexpected-token` at the word (14.3).
    fn bump_word(&mut self) -> R<&'a Token> {
        let tok = self.peek();
        if matches!(tok.word(), Some("r" | "R"))
            && matches!(
                self.peek_n(1).tok,
                Tok::Str(
                    _,
                    StrForm::Double | StrForm::Single | StrForm::RawDouble | StrForm::RawSingle
                )
            )
        {
            return Err(self.unexpected_at(tok.span));
        }
        Ok(self.bump())
    }

    fn expect(&mut self, p: P) -> R<&'a Token> {
        if self.at_punct(p) {
            Ok(self.bump())
        } else {
            Err(self.unexpected(self.peek()))
        }
    }

    // ---- diagnostics --------------------------------------------------

    #[cold]
    #[inline(never)]
    fn diag(&self, c: &'static str, span: Span, msg: String) -> Box<Diagnostic> {
        Box::new(Diagnostic::at(Phase::Parse, c, self.src, span, msg))
    }

    #[cold]
    fn unexpected(&self, tok: &Token) -> Box<Diagnostic> {
        // F3 6.2: a text block's code is reported where the parser reaches
        // its token, before any test of whether it fits.
        if let Some((c, span)) = tok.code {
            return self.diag(c, span, c.to_string());
        }
        let mut span = tok.diag;
        // `01-language.md` 14.3: where the token that cannot follow is the operator
        // `not in`, the span runs from `not` to the end of `in`.
        if tok.is_word("not") && std::ptr::eq(tok, self.peek()) && self.peek_n(1).is_word("in") {
            span = span.to(self.peek_n(1).span);
        }
        let msg = match &tok.tok {
            Tok::Eof => "the text ends where more is needed".to_string(),
            _ => format!("unexpected {}", self.src.slice(span)),
        };
        self.diag(code::UNEXPECTED_TOKEN, span, msg)
    }

    #[cold]
    fn unexpected_at(&self, span: Span) -> Box<Diagnostic> {
        self.diag(
            code::UNEXPECTED_TOKEN,
            span,
            format!("unexpected {}", self.src.slice(span)),
        )
    }

    #[cold]
    fn reserved(&self, tok: &Token) -> Box<Diagnostic> {
        self.diag(
            code::RESERVED_WORD,
            tok.span,
            format!("{} is a reserved word", self.src.slice(tok.span)),
        )
    }

    // ---- levels (13) ----------------------------------------------------

    fn open(&mut self, span: Span) -> R<()> {
        if self.level >= MAX_LEVELS {
            return Err(self.diag(
                code::NESTING_EXCEEDED,
                span,
                "the text nests more than 100 levels deep".to_string(),
            ));
        }
        self.level += 1;
        self.max_level = self.max_level.max(self.level);
        Ok(())
    }

    fn close(&mut self) {
        self.level -= 1;
    }

    // ---- nodes --------------------------------------------------------

    fn node(&mut self, kind: NodeKind, span: Span) -> NodeId {
        let i = self.t.partition_point(|t| t.span.start < span.start);
        let level = self.tok_level.get(i).copied().unwrap_or(0);
        self.ast.push(kind, span, level)
    }

    fn outer(&self, id: NodeId) -> Span {
        self.ast.node(id).outer
    }

    fn binary_node(&mut self, op: BinaryOp, x: NodeId, y: NodeId) -> NodeId {
        let span = self.outer(x).to(self.outer(y));
        self.node(NodeKind::Binary { op, x, y }, span)
    }

    // ---- programs, blocks and branches ----------------------------------

    fn program(&mut self) -> R<NodeId> {
        let first = self.peek().span.start as usize;
        let mut consts = Vec::new();
        while self.at_word("const") {
            consts.push(self.const_line()?);
        }
        let result = self.expr()?;
        let mut end = self.outer(result).end as usize;
        if self.at_punct(P::Semi) {
            end = self.bump().span.end as usize;
        }
        if !matches!(self.peek().tok, Tok::Eof) {
            return Err(self.unexpected(self.peek()));
        }
        Ok(self.node(NodeKind::Program { consts, result }, Span::new(first, end)))
    }

    fn const_line(&mut self) -> R<NodeId> {
        let kw = self.bump();
        let pattern = self.pattern()?;
        self.expect(P::Assign)?;
        let pattern_is_name = matches!(self.ast.kind(pattern), NodeKind::NamePat { name: Some(_) });
        let value = if self.lambda_starts() {
            self.lambda(Some(pattern_is_name))?
        } else {
            self.expr()?
        };
        let semi = self.expect(P::Semi)?;
        Ok(self.node(NodeKind::Const { pattern, value }, kw.span.to(semi.span)))
    }

    /// `{` `{ Const }` `Expr` `}` after `if (...)`, `else`, `try` and `catch`
    /// (3.2 rule 2). Returns the node and the end of the `}`.
    fn branch(&mut self) -> R<(NodeId, u32)> {
        if !self.at_punct(P::LBrace) {
            return Err(self.unexpected(self.peek()));
        }
        let lb = self.bump();
        self.open(lb.span)?;
        // 3.4 rules 7 and 8, at the `...` and at the `}`.
        if self.at_punct(P::Ellipsis) || self.at_punct(P::RBrace) {
            return Err(self.unexpected_at(lb.span));
        }
        let mut consts = Vec::new();
        while self.at_word("const") {
            consts.push(self.const_line()?);
        }
        let e = self.expr()?;
        if self.at_punct(P::Colon) || self.at_punct(P::Comma) {
            return Err(self.unexpected_at(lb.span));
        }
        let rb = self.expect(P::RBrace)?;
        self.close();
        if consts.is_empty() {
            Ok((e, rb.span.end))
        } else {
            let id = self.node(NodeKind::Block { consts, result: e }, lb.span.to(rb.span));
            Ok((id, rb.span.end))
        }
    }

    /// A `Block` where a `Primary` stands: `{` `const` ... (3.2 rule 1).
    fn block(&mut self) -> R<NodeId> {
        let lb = self.bump();
        self.open(lb.span)?;
        let mut consts = Vec::new();
        while self.at_word("const") {
            consts.push(self.const_line()?);
        }
        let result = self.expr()?;
        let rb = self.expect(P::RBrace)?;
        self.close();
        Ok(self.node(NodeKind::Block { consts, result }, lb.span.to(rb.span)))
    }

    // ---- expressions ----------------------------------------------------

    fn expr(&mut self) -> R<NodeId> {
        if self.at_word("if") {
            return self.if_expr();
        }
        if self.at_word("try") {
            return self.try_expr();
        }
        let first = self.binary()?;
        if !self.at_punct(P::Question) {
            return Ok(first);
        }
        // `a ? x : b ? y : c`: the else chain is read by a loop.
        let mut pending = Vec::new();
        let mut test = first;
        let last = loop {
            let q = self.bump();
            self.open(q.span)?;
            let then = self.expr()?;
            self.expect(P::Colon)?;
            self.close();
            pending.push((test, then));
            if self.at_word("if") {
                break self.if_expr()?;
            }
            if self.at_word("try") {
                break self.try_expr()?;
            }
            let b = self.binary()?;
            if self.at_punct(P::Question) {
                test = b;
                continue;
            }
            break b;
        };
        let mut acc = last;
        for (test, then) in pending.into_iter().rev() {
            let span = self.outer(test).to(self.outer(acc));
            acc = self.node(
                NodeKind::Cond {
                    form: CondForm::Ternary,
                    test,
                    then,
                    else_: acc,
                },
                span,
            );
        }
        Ok(acc)
    }

    fn if_expr(&mut self) -> R<NodeId> {
        let mut parts = Vec::new();
        loop {
            let kw = self.bump();
            if !self.at_punct(P::LParen) {
                return Err(self.unexpected(self.peek()));
            }
            let lp = self.bump();
            self.open(lp.span)?;
            let test = self.expr()?;
            self.expect(P::RParen)?;
            self.close();
            let (then, _) = self.branch()?;
            parts.push((kw.span.start, test, then));
            // 3.4 rule 11.
            if !self.at_word("else") {
                return Err(self.unexpected(self.peek()));
            }
            self.bump();
            if self.at_word("if") {
                continue;
            }
            let (else_, end) = self.branch()?;
            let mut acc = else_;
            for (start, test, then) in parts.into_iter().rev() {
                acc = self.node(
                    NodeKind::Cond {
                        form: CondForm::If,
                        test,
                        then,
                        else_: acc,
                    },
                    Span { start, end },
                );
            }
            return Ok(acc);
        }
    }

    fn try_expr(&mut self) -> R<NodeId> {
        let kw = self.bump();
        // 3.4 rule 10.
        if self.at_punct(P::LParen) {
            return Err(self.unexpected_at(kw.span));
        }
        let (body, _) = self.branch()?;
        if !self.at_word("catch") {
            return Err(self.unexpected(self.peek()));
        }
        self.bump();
        let mut name = None;
        if self.at_punct(P::LParen) {
            let lp = self.bump();
            self.open(lp.span)?;
            name = Some(self.name()?);
            self.expect(P::RParen)?;
            self.close();
        }
        let (handler, end) = self.branch()?;
        Ok(self.node(
            NodeKind::Try {
                body,
                name,
                handler,
            },
            Span {
                start: kw.span.start,
                end,
            },
        ))
    }

    /// `Binary = Or | Coal` (3.2 rule 4).
    fn binary(&mut self) -> R<NodeId> {
        let left = self.bin(PREC_ADD)?;
        if self.at_punct(P::QQ) {
            let mut acc = left;
            while self.at_punct(P::QQ) {
                let op = self.bump();
                self.open(op.span)?;
                let right = self.bin(PREC_ADD)?;
                self.close();
                acc = self.binary_node(BinaryOp::Coalesce, acc, right);
            }
            return Ok(acc);
        }
        self.bin_rest(left, PREC_OR)
    }

    fn binop(&self) -> Option<(BinaryOp, u8)> {
        let t = self.peek();
        Some(match &t.tok {
            Tok::Punct(p) => match p {
                P::OrOr => (BinaryOp::Or, 3),
                P::AndAnd => (BinaryOp::And, 4),
                P::EqEq => (BinaryOp::Eq, 5),
                P::NotEq => (BinaryOp::Ne, 5),
                P::Lt => (BinaryOp::Lt, 6),
                P::Le => (BinaryOp::Le, 6),
                P::Gt => (BinaryOp::Gt, 6),
                P::Ge => (BinaryOp::Ge, 6),
                P::Plus => (BinaryOp::Add, 7),
                P::Minus => (BinaryOp::Sub, 7),
                P::Star => (BinaryOp::Mul, 8),
                P::Slash => (BinaryOp::Div, 8),
                P::Percent => (BinaryOp::Rem, 8),
                _ => return None,
            },
            Tok::Word(w) if w == "in" => (BinaryOp::In, 6),
            Tok::Word(w) if w == "not" => (BinaryOp::NotIn, 6),
            _ => return None,
        })
    }

    fn bin(&mut self, min: u8) -> R<NodeId> {
        let lhs = self.unary()?;
        self.bin_rest(lhs, min)
    }

    fn bin_rest(&mut self, mut lhs: NodeId, min: u8) -> R<NodeId> {
        loop {
            // F4 2.4: `as` and `is` stand at level 6, left-associative.
            if min <= PREC_REL && (self.at_word("as") || self.at_word("is")) {
                lhs = self.as_test(lhs)?;
                continue;
            }
            let Some((op, prec)) = self.binop() else {
                break;
            };
            if prec < min {
                break;
            }
            if matches!(op, BinaryOp::In | BinaryOp::NotIn) {
                // 3.4 rule 2, found at the `in`.
                if let Some(bang) = self.bare_not(lhs) {
                    return Err(self.unexpected_at(bang));
                }
            }
            let op_span = if op == BinaryOp::NotIn {
                let not = self.bump();
                if !self.at_word("in") {
                    return Err(self.unexpected(self.peek()));
                }
                let kw = self.bump();
                not.span.to(kw.span)
            } else {
                self.bump().span
            };
            self.open(op_span)?;
            let rhs = self.bin(prec + 1)?;
            self.close();
            lhs = self.binary_node(op, lhs, rhs);
        }
        Ok(lhs)
    }

    // ---- F4: `x as T`, `x is T` and program types ------------------------

    /// `x as T` or `x is T`, at the `as` or `is` (F4 2).
    fn as_test(&mut self, lhs: NodeId) -> R<NodeId> {
        // 2.5: found at the `as` or `is`, before the type is read.
        if let Some(bang) = self.bare_not(lhs) {
            return Err(self.unexpected_at(bang));
        }
        let kw = self.bump();
        let form = if kw.is_word("as") {
            AsForm::As
        } else {
            AsForm::Is
        };
        // 2.6 item 2: Core's errors at the `as` or the `is`.
        if (form == AsForm::As && self.at_word("const"))
            || (form == AsForm::Is && self.at_word("not"))
        {
            return Err(self.unexpected_at(kw.span));
        }
        self.open(kw.span)?;
        let ty = self.ptype()?;
        self.close();
        // 2.4: an operator of levels 7 to 10 cannot follow a type.
        if let Some((_, prec)) = self.binop() {
            if prec > PREC_REL {
                return Err(self.unexpected(self.peek()));
            }
        }
        if self.at_punct(P::StarStar) {
            return Err(self.unexpected(self.peek()));
        }
        let span = self.outer(lhs).to(ty.span);
        Ok(self.node(
            NodeKind::As {
                form,
                x: lhs,
                ty: Box::new(ty),
            },
            span,
        ))
    }

    /// `bad-type-text` at a token (2.6): its code first, a refused token as
    /// `unexpected-token`, a template or text block over its whole span.
    fn bad_type_text(&self, tok: &Token) -> Box<Diagnostic> {
        if let Some((c, span)) = tok.code {
            return self.diag(c, span, c.to_string());
        }
        if matches!(tok.tok, Tok::Refused) {
            return self.unexpected(tok);
        }
        let span = tok.diag;
        self.diag(
            code::BAD_TYPE_TEXT,
            span,
            format!("{} cannot stand in a type", self.src.slice(span)),
        )
    }

    /// `PType = PAlt { "|" PAlt }`.
    fn ptype(&mut self) -> R<PType> {
        let first = self.palt()?;
        let mut span = first.span;
        let mut alts = vec![first];
        while self.at_punct(P::Pipe) {
            self.bump();
            let a = self.palt()?;
            span = span.to(a.span);
            alts.push(a);
        }
        Ok(PType { alts, span })
    }

    /// `PAlt = PPrim { "[" "]" }`. The levels of the suffixes add up and
    /// last to the end of the alternative (2.8).
    fn palt(&mut self) -> R<PAlt> {
        let (prim, mut span) = self.pprim()?;
        let mut suffixes = 0;
        while self.at_punct(P::LBracket) && self.peek_n(1).is_punct(P::RBracket) {
            let lb = self.bump();
            if let Err(e) = self.open(lb.span) {
                for _ in 0..suffixes {
                    self.close();
                }
                return Err(e);
            }
            let rb = self.bump();
            span = span.to(rb.span);
            suffixes += 1;
        }
        for _ in 0..suffixes {
            self.close();
        }
        Ok(PAlt {
            prim,
            suffixes,
            span,
        })
    }

    fn pprim(&mut self) -> R<(PPrim, Span)> {
        let t = self.peek();
        let simple = |p: PPrim| Ok((p, t.span));
        match &t.tok {
            Tok::Word(w) => {
                let p = match w.as_str() {
                    "null" => PPrim::Null,
                    "bool" => PPrim::Bool,
                    "int" => PPrim::Int,
                    "float" => PPrim::Float,
                    "string" => PPrim::String,
                    "any" => PPrim::Any,
                    "never" => PPrim::Never,
                    "map" => {
                        self.bump();
                        let lt = self.peek();
                        if !lt.is_punct(P::Lt) {
                            return Err(self.bad_type_text(lt));
                        }
                        self.bump();
                        self.open(lt.span)?;
                        let inner = self.ptype()?;
                        let gt = self.peek();
                        if !gt.is_punct(P::Gt) {
                            return Err(self.bad_type_text(gt));
                        }
                        self.bump();
                        self.close();
                        return Ok((PPrim::Map(Box::new(inner)), t.span.to(gt.span)));
                    }
                    w if is_type_name(w) => {
                        self.bump();
                        return Ok((PPrim::Name(w.to_string(), t.span), t.span));
                    }
                    _ => return Err(self.bad_type_text(t)),
                };
                self.bump();
                simple(p)
            }
            Tok::Str(s, f) if f.is_line() && t.code.is_none() => {
                self.bump();
                simple(PPrim::Lit(s.clone()))
            }
            Tok::Punct(P::LParen) => {
                self.bump();
                self.open(t.span)?;
                let inner = self.ptype()?;
                let rp = self.peek();
                if !rp.is_punct(P::RParen) {
                    return Err(self.bad_type_text(rp));
                }
                self.bump();
                self.close();
                Ok((PPrim::Paren(Box::new(inner)), t.span.to(rp.span)))
            }
            Tok::Punct(P::LBrace) => self.precord(),
            _ => Err(self.bad_type_text(t)),
        }
    }

    /// `PRecord` (2.1, 2.6).
    fn precord(&mut self) -> R<(PPrim, Span)> {
        let lb = self.bump();
        self.open(lb.span)?;
        let mut fields: Vec<PField> = Vec::new();
        let mut rest: Option<Option<Box<PType>>> = None;
        loop {
            let t = self.peek();
            if t.is_punct(P::RBrace) {
                break;
            }
            if t.is_punct(P::Ellipsis) {
                self.bump();
                let n = self.peek();
                rest = Some(if n.is_punct(P::Comma) || n.is_punct(P::RBrace) {
                    None
                } else {
                    Some(Box::new(self.ptype()?))
                });
                if self.at_punct(P::Comma) {
                    self.bump();
                }
                let n = self.peek();
                if !n.is_punct(P::RBrace) {
                    return Err(self.bad_type_text(n));
                }
                break;
            }
            let key = match &t.tok {
                Tok::Word(w) => w.clone(),
                Tok::Str(s, f) if f.is_line() && t.code.is_none() => s.clone(),
                _ => return Err(self.bad_type_text(t)),
            };
            if fields.iter().any(|f| f.key == key) {
                return Err(self.bad_type_text(t));
            }
            self.bump();
            let mut optional = false;
            if self.at_punct(P::Question) {
                self.bump();
                optional = true;
            }
            let c = self.peek();
            if !c.is_punct(P::Colon) {
                return Err(self.bad_type_text(c));
            }
            self.bump();
            let ty = self.ptype()?;
            fields.push(PField { key, optional, ty });
            let n = self.peek();
            if n.is_punct(P::Comma) {
                self.bump();
            } else if !n.is_punct(P::RBrace) {
                return Err(self.bad_type_text(n));
            }
        }
        let rb = self.bump();
        self.close();
        Ok((PPrim::Record { fields, rest }, lb.span.to(rb.span)))
    }

    /// The span of the `!` when `id` is a `!` expression written without
    /// parentheses.
    fn bare_not(&self, id: NodeId) -> Option<Span> {
        let n = self.ast.node(id);
        match n.kind {
            NodeKind::Unary {
                op: UnaryOp::Not, ..
            } if !n.parenthesized() => Some(Span {
                start: n.span.start,
                end: n.span.start + 1,
            }),
            _ => None,
        }
    }

    fn unary(&mut self) -> R<NodeId> {
        self.prefixed(None)
    }

    /// `Unary`. `first` is the outermost prefix operator already read, which
    /// is the span of a 3.4 rule 1 error (`01-language.md` 3.4 rule 1).
    fn prefixed(&mut self, first: Option<Span>) -> R<NodeId> {
        let t = self.peek();
        let op = match t.tok {
            Tok::Punct(P::Bang) => UnaryOp::Not,
            Tok::Punct(P::Minus) => UnaryOp::Neg,
            _ => return self.power(first),
        };
        self.bump();
        self.open(t.span)?;
        let x = self.prefixed(first.or(Some(t.span)))?;
        self.close();
        let span = t.span.to(self.outer(x));
        Ok(self.node(NodeKind::Unary { op, x }, span))
    }

    /// `Power = Chain [ "**" Unary ]`. `prefix` is the `-` or `!` that applies
    /// directly to it, if any (3.4 rule 1).
    fn power(&mut self, prefix: Option<Span>) -> R<NodeId> {
        let base = self.chain()?;
        if !self.at_punct(P::StarStar) {
            return Ok(base);
        }
        if let Some(p) = prefix {
            return Err(self.unexpected_at(p));
        }
        let op = self.bump();
        self.open(op.span)?;
        let rhs = self.unary()?;
        self.close();
        Ok(self.binary_node(BinaryOp::Pow, base, rhs))
    }

    fn chain(&mut self) -> R<NodeId> {
        let mut x = self.primary()?;
        let start = self.outer(x).start;
        let mut optional_any = false;
        loop {
            let t = self.peek();
            match t.tok {
                Tok::Punct(p @ (P::Dot | P::QDot)) => {
                    let optional = p == P::QDot;
                    self.bump();
                    if self.peek().word().is_none() {
                        return Err(self.unexpected(self.peek()));
                    }
                    let nt = self.bump_word()?;
                    let name = nt.word().unwrap_or_default().to_string();
                    if self.at_punct(P::LParen) {
                        let (args, end) = self.args()?;
                        x = self.node(
                            NodeKind::MethodCall {
                                x,
                                name: SpannedName {
                                    name,
                                    span: nt.span,
                                },
                                args,
                                optional,
                            },
                            Span { start, end },
                        );
                    } else {
                        x = self.node(
                            NodeKind::Member {
                                x,
                                name,
                                name_span: nt.span,
                                optional,
                            },
                            Span {
                                start,
                                end: nt.span.end,
                            },
                        );
                    }
                    optional_any |= optional;
                }
                // F7 2.1: a `[` or `?.[` whose next token is `*` begins a star.
                Tok::Punct(p @ (P::LBracket | P::QDotBracket))
                    if self.peek_n(1).is_punct(P::Star) =>
                {
                    optional_any |= p == P::QDotBracket;
                    x = self.projection(x, start)?;
                }
                Tok::Punct(p @ (P::LBracket | P::QDotBracket)) => {
                    let optional = p == P::QDotBracket;
                    self.bump();
                    self.open(t.span)?;
                    let index = self.expr()?;
                    let rb = self.expect(P::RBracket)?;
                    self.close();
                    x = self.node(
                        NodeKind::Index { x, index, optional },
                        Span {
                            start,
                            end: rb.span.end,
                        },
                    );
                    optional_any |= optional;
                }
                _ => break,
            }
        }
        if optional_any {
            let span = Span {
                start,
                end: self.ast.span(x).end,
            };
            x = self.node(NodeKind::OptChain { x }, span);
        }
        Ok(x)
    }

    /// A projection (F7 2.1 to 2.5), at its first star: the outer `Each`. It
    /// ends before a method call or where the chain ends. Each star opens a
    /// level that lasts to the end of the projection.
    fn projection(&mut self, recv: NodeId, start: u32) -> R<NodeId> {
        struct Level {
            optional: bool,
            star: Span,
            reads: Vec<NodeId>,
            prev: Option<NodeId>,
        }
        let mut levels: Vec<Level> = Vec::new();
        let mut end = self.ast.span(recv).end;
        let result = (|| -> R<()> {
            'star: loop {
                // A star: `[` or `?.[`, `*`, `]`.
                let lb = self.bump();
                let optional = lb.is_punct(P::QDotBracket);
                self.open(lb.span)?;
                levels.push(Level {
                    optional,
                    star: lb.span,
                    reads: Vec::new(),
                    prev: None,
                });
                self.bump(); // `*`
                if !self.at_punct(P::RBracket) {
                    return Err(self.unexpected(self.peek()));
                }
                let rb = self.bump();
                end = rb.span.end;
                let star = Span { start, end };
                levels.last_mut().expect("a level").star = star;
                // The reads of this level.
                loop {
                    let t = self.peek();
                    match t.tok {
                        Tok::Punct(p @ (P::Dot | P::QDot)) => {
                            if self.peek_n(1).word().is_some() && self.peek_n(2).is_punct(P::LParen)
                            {
                                return Ok(());
                            }
                            self.bump();
                            if self.peek().word().is_none() {
                                return Err(self.unexpected(self.peek()));
                            }
                            let nt = self.bump_word()?;
                            let name = nt.word().unwrap_or_default().to_string();
                            let lv = levels.last_mut().expect("a level");
                            let x = match lv.prev {
                                Some(x) => x,
                                None => {
                                    let s = lv.star;
                                    self.node(NodeKind::Elem, s)
                                }
                            };
                            end = nt.span.end;
                            let n = self.node(
                                NodeKind::Member {
                                    x,
                                    name,
                                    name_span: nt.span,
                                    optional: p == P::QDot,
                                },
                                Span { start, end },
                            );
                            let lv = levels.last_mut().expect("a level");
                            lv.reads.push(n);
                            lv.prev = Some(n);
                        }
                        Tok::Punct(P::LBracket | P::QDotBracket)
                            if self.peek_n(1).is_punct(P::Star) =>
                        {
                            continue 'star;
                        }
                        Tok::Punct(p @ (P::LBracket | P::QDotBracket)) => {
                            self.bump();
                            self.open(t.span)?;
                            let index = self.expr()?;
                            let rb = self.expect(P::RBracket)?;
                            self.close();
                            let lv = levels.last_mut().expect("a level");
                            let x = match lv.prev {
                                Some(x) => x,
                                None => {
                                    let s = lv.star;
                                    self.node(NodeKind::Elem, s)
                                }
                            };
                            end = rb.span.end;
                            let n = self.node(
                                NodeKind::Index {
                                    x,
                                    index,
                                    optional: p == P::QDotBracket,
                                },
                                Span { start, end },
                            );
                            let lv = levels.last_mut().expect("a level");
                            lv.reads.push(n);
                            lv.prev = Some(n);
                        }
                        _ => return Ok(()),
                    }
                }
            }
        })();
        for _ in 0..levels.len() {
            self.close();
        }
        result?;
        // Fold the levels from the innermost: a later star is the last read
        // of the level before it.
        let span = Span { start, end };
        let mut inner: Option<NodeId> = None;
        while let Some(mut lv) = levels.pop() {
            if let Some(i) = inner.take() {
                lv.reads.push(i);
            }
            let x = if levels.is_empty() { Some(recv) } else { None };
            inner = Some(self.node(
                NodeKind::Each {
                    x,
                    reads: lv.reads,
                    optional: lv.optional,
                    star: lv.star,
                },
                span,
            ));
        }
        Ok(inner.expect("one level"))
    }

    /// `Args`. Returns the arguments and the end of the `)`.
    fn args(&mut self) -> R<(Vec<NodeId>, u32)> {
        let lp = self.bump();
        self.open(lp.span)?;
        let mut args = Vec::new();
        loop {
            if self.at_punct(P::RParen) {
                break;
            }
            let a = if self.lambda_starts() {
                self.lambda(None)?
            } else {
                self.expr()?
            };
            args.push(a);
            if self.at_punct(P::Comma) {
                self.bump();
            } else {
                break;
            }
        }
        let rp = self.expect(P::RParen)?;
        self.close();
        Ok((args, rp.span.end))
    }

    fn primary(&mut self) -> R<NodeId> {
        let t = self.peek();
        if t.code.is_some() {
            return Err(self.unexpected(t));
        }
        match &t.tok {
            Tok::Number(v) => {
                self.bump();
                Ok(self.node(NodeKind::Number { value: *v }, t.span))
            }
            Tok::Str(v, form) => {
                self.bump();
                Ok(self.node(
                    NodeKind::String {
                        value: v.clone(),
                        form: *form,
                    },
                    t.span,
                ))
            }
            Tok::TmplHead(_) => self.template(),
            Tok::Word(w) => match w.as_str() {
                "true" | "false" => {
                    self.bump();
                    Ok(self.node(NodeKind::Bool { value: w == "true" }, t.span))
                }
                "null" => {
                    self.bump();
                    Ok(self.node(NodeKind::Null, t.span))
                }
                "match" => self.match_expr(),
                // 3.4 rule 5.
                "if" | "try" => Err(self.unexpected(t)),
                "_" => Err(self.unexpected(t)),
                w if is_reserved(w) => Err(self.reserved(t)),
                _ => {
                    let nt = self.bump_word()?;
                    let name = SpannedName {
                        name: w.clone(),
                        span: nt.span,
                    };
                    if self.at_punct(P::LParen) {
                        let (args, end) = self.args()?;
                        Ok(self.node(
                            NodeKind::Call {
                                namespace: None,
                                name,
                                args,
                            },
                            Span {
                                start: nt.span.start,
                                end,
                            },
                        ))
                    } else {
                        Ok(self.node(NodeKind::Ident { name: w.clone() }, nt.span))
                    }
                }
            },
            Tok::Punct(P::LParen) => {
                self.bump();
                self.open(t.span)?;
                let e = self.expr()?;
                let rp = self.expect(P::RParen)?;
                self.close();
                self.ast.nodes[e.index()].outer = t.span.to(rp.span);
                Ok(e)
            }
            Tok::Punct(P::LBracket) => self.list(),
            Tok::Punct(P::LBrace) => {
                if self.peek_n(1).is_word("const") && !self.peek_n(2).is_punct(P::Colon) {
                    self.block()
                } else {
                    self.map()
                }
            }
            _ => Err(self.unexpected(t)),
        }
    }

    fn template(&mut self) -> R<NodeId> {
        let head = self.bump();
        let mut parts = Vec::new();
        if let Tok::TmplHead(s) = &head.tok {
            parts.push(TemplatePart::Text(s.clone()));
        }
        let mut dollar = Span {
            start: head.span.end - 2,
            end: head.span.end,
        };
        loop {
            self.open(dollar)?;
            let e = self.expr()?;
            let t = self.peek();
            match &t.tok {
                Tok::TmplMiddle(s) => {
                    self.bump();
                    self.close();
                    parts.push(TemplatePart::Expr(e));
                    parts.push(TemplatePart::Text(s.clone()));
                    dollar = Span {
                        start: t.span.end - 2,
                        end: t.span.end,
                    };
                }
                Tok::TmplTail(s) => {
                    self.bump();
                    self.close();
                    parts.push(TemplatePart::Expr(e));
                    parts.push(TemplatePart::Text(s.clone()));
                    return Ok(self.node(NodeKind::Template { parts }, head.span.to(t.span)));
                }
                _ => return Err(self.unexpected(t)),
            }
        }
    }

    fn list(&mut self) -> R<NodeId> {
        let lb = self.bump();
        self.open(lb.span)?;
        let mut items = Vec::new();
        loop {
            if self.at_punct(P::RBracket) {
                break;
            }
            let item = if self.at_punct(P::Ellipsis) {
                self.spread()?
            } else {
                self.expr()?
            };
            items.push(item);
            if self.at_punct(P::Comma) {
                self.bump();
            } else {
                break;
            }
        }
        let rb = self.expect(P::RBracket)?;
        self.close();
        Ok(self.node(NodeKind::List { items }, lb.span.to(rb.span)))
    }

    fn spread(&mut self) -> R<NodeId> {
        let dots = self.bump();
        let value = self.expr()?;
        let span = dots.span.to(self.outer(value));
        Ok(self.node(NodeKind::Spread { value }, span))
    }

    fn map(&mut self) -> R<NodeId> {
        let lb = self.bump();
        self.open(lb.span)?;
        let mut entries = Vec::new();
        loop {
            if self.at_punct(P::RBrace) {
                break;
            }
            entries.push(self.entry()?);
            if self.at_punct(P::Comma) {
                self.bump();
            } else {
                break;
            }
        }
        let rb = self.expect(P::RBrace)?;
        self.close();
        Ok(self.node(NodeKind::Map { entries }, lb.span.to(rb.span)))
    }

    fn entry(&mut self) -> R<NodeId> {
        let t = self.peek();
        match &t.tok {
            Tok::Punct(P::Ellipsis) => self.spread(),
            // 3.2 rule 7.
            Tok::Word(w) if self.peek_n(1).is_punct(P::Colon) => {
                self.bump_word()?;
                self.bump();
                let value = self.expr()?;
                let span = t.span.to(self.outer(value));
                Ok(self.node(
                    NodeKind::Entry {
                        key: EntryKey::Word(w.clone()),
                        key_span: t.span,
                        value,
                        shorthand: false,
                    },
                    span,
                ))
            }
            Tok::Word(w) => {
                if is_reserved(w) {
                    return Err(self.reserved(t));
                }
                if w == "_" {
                    return Err(self.unexpected(t));
                }
                self.bump_word()?;
                let value = self.node(NodeKind::Ident { name: w.clone() }, t.span);
                Ok(self.node(
                    NodeKind::Entry {
                        key: EntryKey::Word(w.clone()),
                        key_span: t.span,
                        value,
                        shorthand: true,
                    },
                    t.span,
                ))
            }
            Tok::Str(s, f) if f.is_line() => {
                self.bump();
                self.expect(P::Colon)?;
                let value = self.expr()?;
                let span = t.span.to(self.outer(value));
                Ok(self.node(
                    NodeKind::Entry {
                        key: EntryKey::Str(s.clone()),
                        key_span: t.span,
                        value,
                        shorthand: false,
                    },
                    span,
                ))
            }
            Tok::Punct(P::LBracket) => {
                self.bump();
                self.open(t.span)?;
                let k = self.expr()?;
                let rb = self.expect(P::RBracket)?;
                self.close();
                self.expect(P::Colon)?;
                let value = self.expr()?;
                let span = t.span.to(self.outer(value));
                Ok(self.node(
                    NodeKind::Entry {
                        key: EntryKey::Computed(k),
                        key_span: t.span.to(rb.span),
                        value,
                        shorthand: false,
                    },
                    span,
                ))
            }
            _ => Err(self.unexpected(t)),
        }
    }

    fn match_expr(&mut self) -> R<NodeId> {
        let kw = self.bump();
        self.open(kw.span)?;
        // 3.4 rule 6.
        let t = self.peek();
        if t.is_punct(P::LBrace) || t.is_word("if") || t.is_word("try") {
            return Err(self.unexpected(t));
        }
        let subject = self.expr()?;
        self.close();
        if !self.at_punct(P::LBrace) {
            return Err(self.unexpected(self.peek()));
        }
        let lb = self.bump();
        self.open(lb.span)?;
        let mut arms = Vec::new();
        loop {
            arms.push(self.arm()?);
            if self.at_punct(P::Comma) {
                self.bump();
                if self.at_punct(P::RBrace) {
                    break;
                }
            } else {
                break;
            }
        }
        let rb = self.expect(P::RBrace)?;
        self.close();
        Ok(self.node(NodeKind::Match { subject, arms }, kw.span.to(rb.span)))
    }

    fn arm(&mut self) -> R<NodeId> {
        let start = self.peek().span;
        let literals = if self.at_word("_") {
            self.bump();
            None
        } else {
            let mut lits = vec![self.lit()?];
            while self.at_punct(P::Pipe) {
                self.bump();
                lits.push(self.lit()?);
            }
            Some(lits)
        };
        self.expect(P::Arrow)?;
        let result = self.expr()?;
        let span = start.to(self.outer(result));
        Ok(self.node(NodeKind::Arm { literals, result }, span))
    }

    fn lit(&mut self) -> R<ArmLit> {
        let t = self.peek();
        let lit = match &t.tok {
            Tok::Punct(P::Minus) => {
                self.bump();
                let n = self.peek();
                return match n.tok {
                    Tok::Number(v) => {
                        self.bump();
                        let v = if v == 0.0 { 0.0 } else { -v };
                        Ok(ArmLit {
                            lit: Lit::Number(v),
                            span: t.span.to(n.span),
                        })
                    }
                    _ => Err(self.unexpected(n)),
                };
            }
            Tok::Number(v) => Lit::Number(*v),
            Tok::Str(s, f) if f.is_line() => Lit::Str(s.clone()),
            Tok::Word(w) if w == "true" => Lit::Bool(true),
            Tok::Word(w) if w == "false" => Lit::Bool(false),
            Tok::Word(w) if w == "null" => Lit::Null,
            // `01-language.md` 14.2: a reserved word is `reserved-word`.
            Tok::Word(w) if is_reserved(w) => return Err(self.reserved(t)),
            _ => return Err(self.unexpected(t)),
        };
        self.bump();
        Ok(ArmLit { lit, span: t.span })
    }

    // ---- lambdas (3.2 rules 5 and 6) ------------------------------------

    fn lambda_starts(&self) -> bool {
        let t = self.peek();
        match &t.tok {
            Tok::Word(w) if is_name(w) || w == "_" => self.peek_n(1).is_punct(P::Arrow),
            Tok::Punct(P::LParen) => match self.partner[self.pos] {
                Some(c) => {
                    let c = c as usize;
                    self.t[c].is_punct(P::RParen)
                        && self.t.get(c + 1).is_some_and(|n| n.is_punct(P::Arrow))
                }
                None => false,
            },
            _ => false,
        }
    }

    /// A lambda. `in_const` is `Some(pattern is a Name)` for the value of a
    /// `const` line, and `None` for an argument.
    fn lambda(&mut self, in_const: Option<bool>) -> R<NodeId> {
        let outer_max = std::mem::replace(&mut self.max_level, self.level);
        let id = self.lambda_inner(in_const)?;
        let inner = self.max_level;
        self.ast.lambda_levels.insert(id.0, inner as u16);
        self.max_level = outer_max.max(inner);
        Ok(id)
    }

    fn lambda_inner(&mut self, in_const: Option<bool>) -> R<NodeId> {
        let start = self.peek().span;
        let mut params = Vec::new();
        if let Some(w) = self.peek().word() {
            let nt = self.bump_word()?;
            let name = if w == "_" { None } else { Some(w.to_string()) };
            let pattern = self.node(NodeKind::NamePat { name }, nt.span);
            params.push(self.node(
                NodeKind::Param {
                    pattern,
                    default: None,
                },
                nt.span,
            ));
        } else {
            let lp = self.bump();
            self.open(lp.span)?;
            loop {
                if self.at_punct(P::RParen) {
                    break;
                }
                params.push(self.param()?);
                if self.at_punct(P::Comma) {
                    self.bump();
                } else {
                    break;
                }
            }
            self.expect(P::RParen)?;
            self.close();
        }
        let arrow = self.peek();
        if !arrow.is_punct(P::Arrow) {
            return Err(self.unexpected(arrow));
        }
        // 3.4 rule 9, found at the `=>`.
        if in_const == Some(false) {
            return Err(self.unexpected_at(arrow.span));
        }
        self.bump();
        let body = self.expr()?;
        let span = start.to(self.outer(body));
        Ok(self.node(NodeKind::Lambda { params, body }, span))
    }

    fn param(&mut self) -> R<NodeId> {
        let pattern = self.pattern()?;
        let mut span = self.ast.span(pattern);
        let mut default = None;
        if self.at_punct(P::Assign) {
            self.bump();
            let d = self.expr()?;
            span = span.to(self.outer(d));
            default = Some(d);
        }
        Ok(self.node(NodeKind::Param { pattern, default }, span))
    }

    // ---- patterns -------------------------------------------------------

    /// A `Name` (14.2 for a reserved word).
    fn name(&mut self) -> R<SpannedName> {
        let t = self.peek();
        match &t.tok {
            Tok::Word(w) if is_name(w) => {
                self.bump_word()?;
                Ok(SpannedName {
                    name: w.clone(),
                    span: t.span,
                })
            }
            Tok::Word(w) if is_reserved(w) => Err(self.reserved(t)),
            _ => Err(self.unexpected(t)),
        }
    }

    fn pattern(&mut self) -> R<NodeId> {
        let t = self.peek();
        match &t.tok {
            Tok::Word(w) if w == "_" => {
                self.bump();
                Ok(self.node(NodeKind::NamePat { name: None }, t.span))
            }
            Tok::Word(_) => {
                let n = self.name()?;
                Ok(self.node(NodeKind::NamePat { name: Some(n.name) }, n.span))
            }
            Tok::Punct(P::LBrace) => self.map_pat(),
            Tok::Punct(P::LBracket) => self.list_pat(),
            // 3.5 rule 3: a template where a pattern must stand is one
            // token, and its interpolations are not reached.
            Tok::TmplHead(_) if t.code.is_none() => {
                let end =
                    self.tmpl_end[self.pos].map_or(t.span, |e| t.span.to(self.t[e as usize].span));
                Err(self.unexpected_at(end))
            }
            _ => Err(self.unexpected(t)),
        }
    }

    fn map_pat(&mut self) -> R<NodeId> {
        let lb = self.bump();
        self.open(lb.span)?;
        let mut fields = Vec::new();
        let mut rest = None;
        loop {
            if self.at_punct(P::RBrace) {
                break;
            }
            if self.at_punct(P::Ellipsis) {
                self.bump();
                rest = Some(self.name()?);
                if self.at_punct(P::Comma) {
                    self.bump();
                }
                break;
            }
            fields.push(self.pat_field()?);
            if self.at_punct(P::Comma) {
                self.bump();
            } else {
                break;
            }
        }
        let rb = self.expect(P::RBrace)?;
        self.close();
        Ok(self.node(NodeKind::MapPat { fields, rest }, lb.span.to(rb.span)))
    }

    fn pat_field(&mut self) -> R<PatField> {
        let t = self.peek();
        let (key, name) = match &t.tok {
            // 3.2 rule 8.
            Tok::Word(w) if self.peek_n(1).is_punct(P::Colon) => {
                self.bump_word()?;
                self.bump();
                (w.clone(), self.name()?)
            }
            Tok::Word(_) => {
                let n = self.name()?;
                (n.name.clone(), n)
            }
            Tok::Str(s, f) if f.is_line() => {
                self.bump();
                self.expect(P::Colon)?;
                (s.clone(), self.name()?)
            }
            _ => return Err(self.unexpected(t)),
        };
        let (default, end) = self.pat_default(name.span)?;
        Ok(PatField {
            key,
            key_span: t.span,
            name,
            default,
            span: t.span.to(end),
        })
    }

    /// An optional `= Expr`. Returns the default and the span the element ends
    /// with.
    fn pat_default(&mut self, last: Span) -> R<(Option<NodeId>, Span)> {
        if self.at_punct(P::Assign) {
            self.bump();
            let d = self.expr()?;
            Ok((Some(d), self.outer(d)))
        } else {
            Ok((None, last))
        }
    }

    fn list_pat(&mut self) -> R<NodeId> {
        let lb = self.bump();
        self.open(lb.span)?;
        let mut elems = Vec::new();
        let mut rest = None;
        loop {
            if self.at_punct(P::RBracket) {
                break;
            }
            if self.at_punct(P::Ellipsis) {
                self.bump();
                rest = Some(self.name()?);
                if self.at_punct(P::Comma) {
                    self.bump();
                }
                break;
            }
            let t = self.peek();
            let pat = if t.is_word("_") {
                self.bump();
                self.node(NodeKind::NamePat { name: None }, t.span)
            } else {
                let n = self.name()?;
                self.node(NodeKind::NamePat { name: Some(n.name) }, n.span)
            };
            let (default, end) = self.pat_default(t.span)?;
            elems.push(PatElem {
                pat,
                default,
                span: t.span.to(end),
            });
            if self.at_punct(P::Comma) {
                self.bump();
            } else {
                break;
            }
        }
        let rb = self.expect(P::RBracket)?;
        self.close();
        Ok(self.node(NodeKind::ListPat { elems, rest }, lb.span.to(rb.span)))
    }
}
