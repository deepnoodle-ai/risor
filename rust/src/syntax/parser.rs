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

/// A parsed program (`05-host-api.md` E.4). It checks, compiles and runs
/// against an environment: [`Env::check`](crate::Env::check) and
/// [`Env::compile`](crate::Env::compile).
#[derive(Clone, Debug)]
pub struct Expr {
    pub(crate) source: Source,
    pub(crate) ast: Ast,
}

impl Expr {
    /// The text of the program, as parsed: a U+FEFF at the start is not part
    /// of it.
    pub fn text(&self) -> String {
        self.source.chars().iter().collect()
    }

    /// The analysis of the program (F2 3.1): the names it reads, the paths
    /// it reads through them and the calls it makes. It needs no
    /// environment and never fails.
    pub fn analyze(&self) -> crate::analysis::Analysis {
        crate::analysis::analyze(self)
    }
}

/// Parse the text of a program. A failure has phase `parse` and one
/// diagnostic: the first, with the targeted code of `f08-errors.md` 2 where
/// one applies.
pub fn parse(text: &str) -> Result<Expr, Error> {
    parse_with(text, false)
}

/// Parse as `parse` does, but give the Core diagnostic: the first diagnostic
/// as an implementation without package F8 finds it (`f08-errors.md` 2.1
/// item 1), with its message.
#[cfg(test)]
pub fn parse_core(text: &str) -> Result<Expr, Error> {
    parse_with(text, true)
}

fn parse_with(text: &str, core: bool) -> Result<Expr, Error> {
    let source = Source::new(text);
    let ast = {
        let toks = lex(&source).map_err(|d| {
            // Rows A1 to A5 are lexical: the lexer gives their codes.
            if core && !CORE_CODES.contains(&d.code) {
                let span = d.range.unwrap_or_default();
                let msg = super::message::parse_message(
                    &source,
                    code::UNEXPECTED_CHARACTER,
                    "default",
                    span,
                    &[],
                );
                return Error::one(Diagnostic::at(
                    Phase::Parse,
                    code::UNEXPECTED_CHARACTER,
                    &source,
                    span,
                    msg,
                ));
            }
            Error::one(d)
        })?;
        let mut p = Parser::new(&source, &toks);
        p.core = core;
        let root = p.program().map_err(|e| Error::one(p.finish(&e)))?;
        p.ast.root = root;
        p.ast
    };
    Ok(Expr { source, ast })
}

/// The `parse` codes of Core and of F3 and F4 (`01-language.md` 14.2).
const CORE_CODES: [&str; 14] = [
    code::UNEXPECTED_TOKEN,
    code::UNEXPECTED_CHARACTER,
    code::UNTERMINATED_STRING,
    code::UNTERMINATED_COMMENT,
    code::BAD_ESCAPE,
    code::LINE_CONTINUATION,
    code::BAD_NUMBER,
    code::LITERAL_RANGE,
    code::NESTING_EXCEEDED,
    code::RESERVED_WORD,
    code::BAD_TYPE_TEXT,
    code::TEXT_BLOCK_OPEN,
    code::TEXT_BLOCK_CLOSE,
    code::TEXT_BLOCK_INDENT,
];

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
        p.open(open).map_err(|e| p.finish(&e))?;
        let e = p.expr().map_err(|e| p.finish(&e))?;
        if !matches!(p.peek().tok, Tok::Eof) {
            let e = p.unexpected(p.peek());
            return Err(p.finish(&e));
        }
        p.close();
        ids.push(e);
    }
    Ok((p.ast, ids))
}

/// A type name (F4 2.1 item 2): an upper-case letter, then one or more of
/// `[A-Za-z0-9_]`.
pub fn is_type_name(w: &str) -> bool {
    if w == HOLE {
        return true;
    }
    let b = w.as_bytes();
    b.len() >= 2
        && b[0].is_ascii_uppercase()
        && b[1..]
            .iter()
            .all(|c| c.is_ascii_alphanumeric() || *c == b'_')
}

/// F12 3.4: the text of the hole, a word no source text can spell. The
/// parser reads it as a `Name`, a `Word` or a type name.
pub const HOLE: &str = "\u{10FFFF}";

/// F12 3.3: parse tokens that end with `Eof`, where the tokens from index
/// `limit` on were inserted by the repair, or close an interpolation the cut
/// left open. A bracket among them matches no opener (3.3 item 7). A failure
/// gives the index of the token where the parser stood.
pub(crate) fn parse_repair(src: &Source, toks: &[Token], limit: usize) -> Result<Ast, usize> {
    let mut p = Parser::new(src, toks);
    p.core = true;
    for (i, q) in p.partner.iter_mut().enumerate() {
        if i >= limit || q.is_some_and(|c| c as usize >= limit) {
            *q = None;
        }
    }
    match p.program() {
        Ok(root) => {
            p.ast.root = root;
            Ok(p.ast)
        }
        Err(_) => Err(p.err_pos.get()),
    }
}

/// The error is boxed: a large `Err` makes every `?` cost stack in a build
/// with no optimization, and the parser recurses once for each level.
type R<T> = Result<T, Box<PErr>>;

/// The first diagnostic as the parser finds it: Core's code and span, and
/// the rule of Core that gives it. `finish` maps it by `f08-errors.md` 2 and
/// builds its message.
struct PErr {
    code: &'static str,
    span: Span,
    rule: Rule,
}

/// The rule of Core that gives a diagnostic (`f08-errors.md` 2.1 item 5).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum Rule {
    /// `01-language.md` 3.5, or a row of 14.3.
    General,
    /// 3.5, where an operand must begin.
    Operand,
    /// A code that is never mapped, with the template the text chooses.
    Final,
    /// `reserved-word` by 14.2 rule 1, where the grammar needs a `Name`.
    Name,
    /// `reserved-word` by 14.2 rule 2.
    Value,
    /// `reserved-word` at a shorthand entry of a map literal: a place of
    /// rows B1 to B9, with the template `name` where the code stays.
    Shorthand,
    /// 3.4 rules 1, 2, 3, 5 and 6.
    Power,
    NotIn,
    Coalesce,
    OperandIf,
    MatchSubject,
    /// 3.4 rules 7 and 8.
    BranchMap,
    EmptyBranch,
    /// 3.4 rule 9.
    LambdaPattern,
    /// 3.4 rule 10.
    TryCall,
    /// 3.4 rule 11.
    MissingElse,
    /// 3.4 rule 12: where `catch` should stand; after `catch`; inside its
    /// parentheses.
    MissingCatch,
    TryForm,
    CatchName,
    /// F4 2.5, with the word `as` (false) or `is` (true).
    NotTest(bool),
    /// F4 2.6 item 2: `x as const` and `x is not T`.
    AsConst,
    IsNot,
    /// After `if`, its test or `else`, a token that does not fit.
    IfForm,
    /// Where the `:` of a conditional should stand; the `?` is the token.
    CondColon(u32),
    /// The row of 14.3 for the word `r` or `R`, white space, then a string.
    Prefix,
    /// `bad-type-text`, with its template.
    BadType(&'static str),
    /// Where the `:` after a string key or a computed key is needed, in a
    /// map literal or a map pattern (the template `key`).
    Key,
}

/// A mapped diagnostic: its code, its template, its span, and its
/// parameters other than `token` and `char`.
type Target = (
    &'static str,
    &'static str,
    Span,
    Vec<(&'static str, String)>,
);

/// Marks on tokens, for the conditions of `f08-errors.md` 2.
const MARK_MAP: u8 = 1;
const MARK_MAP_PATTERN: u8 = 2;
const MARK_INDEX: u8 = 3;
const MARK_PAREN: u8 = 4;
const MARK_IF_TEST: u8 = 5;
const MARK_OPERAND_WORD: u8 = 6;
const MARK_LAMBDA_ARROW: u8 = 7;

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
    /// For `f08-errors.md` 2: a mark on each token, and the token where the
    /// last expression, `if` or `try`, program type, `const` pattern and
    /// parameter pattern ended, and where the last `match` case began.
    mark: Vec<u8>,
    expr_end: usize,
    if_end: usize,
    type_end: usize,
    const_pat_end: usize,
    param_end: usize,
    arm_start: usize,
    /// Give the Core diagnostic, unmapped.
    core: bool,
    /// Where the parser stood at its last error (F12 3.3).
    err_pos: std::cell::Cell<usize>,
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
            mark: vec![0; t.len()],
            expr_end: usize::MAX,
            if_end: usize::MAX,
            type_end: usize::MAX,
            const_pat_end: usize::MAX,
            param_end: usize::MAX,
            arm_start: usize::MAX,
            core: false,
            err_pos: std::cell::Cell::new(0),
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
            return Err(self.perr(code::UNEXPECTED_TOKEN, tok.span, Rule::Prefix));
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
    fn perr(&self, code: &'static str, span: Span, rule: Rule) -> Box<PErr> {
        self.err_pos.set(self.pos);
        Box::new(PErr { code, span, rule })
    }

    /// An error with the rule that gives it, unless it is a code that is
    /// never mapped.
    #[cold]
    fn tag(&self, mut e: Box<PErr>, rule: Rule) -> Box<PErr> {
        if e.code == code::UNEXPECTED_TOKEN {
            e.rule = rule;
        }
        e
    }

    #[cold]
    fn unexpected(&self, tok: &Token) -> Box<PErr> {
        // F3 6.2: a text block's code is reported where the parser reaches
        // its token, before any test of whether it fits.
        if let Some((c, span)) = tok.code {
            return self.perr(c, span, Rule::Final);
        }
        let mut span = tok.diag;
        // `01-language.md` 14.3: where the token that cannot follow is the operator
        // `not in`, the span runs from `not` to the end of `in`.
        if tok.is_word("not") && std::ptr::eq(tok, self.peek()) && self.peek_n(1).is_word("in") {
            span = span.to(self.peek_n(1).span);
        }
        self.perr(code::UNEXPECTED_TOKEN, span, Rule::General)
    }

    #[cold]
    fn unexpected_at(&self, span: Span) -> Box<PErr> {
        self.perr(code::UNEXPECTED_TOKEN, span, Rule::General)
    }

    #[cold]
    fn reserved(&self, tok: &Token, rule: Rule) -> Box<PErr> {
        self.perr(code::RESERVED_WORD, tok.span, rule)
    }

    // ---- levels (13) ----------------------------------------------------

    fn open(&mut self, span: Span) -> R<()> {
        if self.level >= MAX_LEVELS {
            return Err(self.perr(code::NESTING_EXCEEDED, span, Rule::Final));
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
        self.const_pat_end = self.pos;
        self.expect(P::Assign)?;
        let pattern_is_name = matches!(self.ast.kind(pattern), NodeKind::NamePat { name: Some(_) });
        self.reserved_param()?;
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
        if self.at_punct(P::Ellipsis) {
            return Err(self.perr(code::UNEXPECTED_TOKEN, lb.span, Rule::BranchMap));
        }
        if self.at_punct(P::RBrace) {
            return Err(self.perr(code::UNEXPECTED_TOKEN, lb.span, Rule::EmptyBranch));
        }
        let mut consts = Vec::new();
        while self.at_word("const") {
            consts.push(self.const_line()?);
        }
        let e = self.expr()?;
        if self.at_punct(P::Colon) || self.at_punct(P::Comma) {
            return Err(self.perr(code::UNEXPECTED_TOKEN, lb.span, Rule::BranchMap));
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
        let e = self.expr_inner()?;
        self.expr_end = self.pos;
        Ok(e)
    }

    /// An `if` or a `try`, and the token after its end.
    fn whole(&mut self) -> R<NodeId> {
        let e = if self.at_word("if") {
            self.if_expr()?
        } else {
            self.try_expr()?
        };
        self.if_end = self.pos;
        Ok(e)
    }

    fn expr_inner(&mut self) -> R<NodeId> {
        if self.at_word("if") || self.at_word("try") {
            return self.whole();
        }
        let first = self.binary()?;
        if !self.at_punct(P::Question) {
            return Ok(first);
        }
        // `a ? x : b ? y : c`: the else chain is read by a loop.
        let mut pending = Vec::new();
        let mut test = first;
        let last = loop {
            let qi = self.pos as u32;
            let q = self.bump();
            self.open(q.span)?;
            let then = self.expr()?;
            if !self.at_punct(P::Colon) {
                let e = self.unexpected(self.peek());
                return Err(self.tag(e, Rule::CondColon(qi)));
            }
            self.bump();
            self.close();
            pending.push((test, then));
            if self.at_word("if") || self.at_word("try") {
                break self.whole()?;
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
                let e = self.unexpected(self.peek());
                return Err(self.tag(e, Rule::IfForm));
            }
            self.mark[self.pos] = MARK_IF_TEST;
            let lp = self.bump();
            self.open(lp.span)?;
            let test = self.expr()?;
            self.expect(P::RParen)?;
            self.close();
            if !self.at_punct(P::LBrace) {
                let e = self.unexpected(self.peek());
                return Err(self.tag(e, Rule::IfForm));
            }
            let (then, _) = self.branch()?;
            parts.push((kw.span.start, test, then));
            // 3.4 rule 11.
            if !self.at_word("else") {
                let e = self.unexpected(self.peek());
                return Err(self.tag(e, Rule::MissingElse));
            }
            self.bump();
            if self.at_word("if") {
                continue;
            }
            if !self.at_punct(P::LBrace) {
                let e = self.unexpected(self.peek());
                return Err(self.tag(e, Rule::IfForm));
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
            return Err(self.perr(code::UNEXPECTED_TOKEN, kw.span, Rule::TryCall));
        }
        let (body, _) = self.branch()?;
        // 3.4 rule 12.
        if !self.at_word("catch") {
            let e = self.unexpected(self.peek());
            return Err(self.tag(e, Rule::MissingCatch));
        }
        self.bump();
        let mut name = None;
        if self.at_punct(P::LParen) {
            let lp = self.bump();
            self.open(lp.span)?;
            let t = self.peek();
            match t.word() {
                Some(w) if is_reserved(w) => return Err(self.reserved(t, Rule::Name)),
                Some(w) if is_name(w) => {}
                _ => {
                    let e = self.unexpected(t);
                    return Err(self.tag(e, Rule::CatchName));
                }
            }
            name = Some(self.name()?);
            if !self.at_punct(P::RParen) {
                let e = self.unexpected(self.peek());
                return Err(self.tag(e, Rule::CatchName));
            }
            self.bump();
            self.close();
        }
        if !self.at_punct(P::LBrace) {
            let e = self.unexpected(self.peek());
            return Err(self.tag(e, Rule::TryForm));
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
            // 3.4 rule 3, at the operator that cannot follow.
            if self.mixes_with_coalesce() {
                let e = self.unexpected(self.peek());
                return Err(self.tag(e, Rule::Coalesce));
            }
            return Ok(acc);
        }
        let e = self.bin_rest(left, PREC_OR)?;
        if self.at_punct(P::QQ) {
            let e = self.unexpected(self.peek());
            return Err(self.tag(e, Rule::Coalesce));
        }
        Ok(e)
    }

    /// An operator that does not mix with `??` (3.4 rule 3; F4 2.4).
    fn mixes_with_coalesce(&self) -> bool {
        let t = self.peek();
        matches!(
            t.tok,
            Tok::Punct(P::OrOr | P::AndAnd | P::EqEq | P::NotEq | P::Lt | P::Le | P::Gt | P::Ge)
        ) || matches!(t.word(), Some("in" | "not" | "as" | "is"))
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
                    return Err(self.perr(code::UNEXPECTED_TOKEN, bang, Rule::NotIn));
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
            let is = self.at_word("is");
            return Err(self.perr(code::UNEXPECTED_TOKEN, bang, Rule::NotTest(is)));
        }
        let kw = self.bump();
        let form = if kw.is_word("as") {
            AsForm::As
        } else {
            AsForm::Is
        };
        // 2.6 item 2: Core's errors at the `as` or the `is`.
        if form == AsForm::As && self.at_word("const") {
            return Err(self.perr(code::UNEXPECTED_TOKEN, kw.span, Rule::AsConst));
        }
        if form == AsForm::Is && self.at_word("not") {
            return Err(self.perr(code::UNEXPECTED_TOKEN, kw.span, Rule::IsNot));
        }
        self.open(kw.span)?;
        let ty = self.ptype()?;
        self.close();
        self.type_end = self.pos;
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
    /// `place` names the template of `f08-errors.md` 3.4 for the place.
    fn bad_type_text(&self, tok: &Token, place: &'static str) -> Box<PErr> {
        if let Some((c, span)) = tok.code {
            return self.perr(c, span, Rule::Final);
        }
        if matches!(tok.tok, Tok::Refused) {
            return self.unexpected(tok);
        }
        let literal = matches!(tok.tok, Tok::TmplHead(_))
            || matches!(
                tok.tok,
                Tok::Str(
                    _,
                    StrForm::Backtick | StrForm::TripleDouble | StrForm::RawTriple
                )
            );
        let template = if literal {
            "literal"
        } else if place == "map-close" && tok.is_punct(P::Ge) {
            "split"
        } else if place != "expected" {
            place
        } else {
            match &tok.tok {
                Tok::Number(_) | Tok::Punct(P::Minus) => "value",
                Tok::Word(w) => match w.as_str() {
                    "true" | "false" => "value",
                    "number" => "number-word",
                    "boolean" => "boolean-word",
                    "undefined" | "void" | "nil" => "null-word",
                    "object" => "object-word",
                    w if w.len() == 1 && w.as_bytes()[0].is_ascii_uppercase() => "letter",
                    w if w.starts_with(|c: char| c.is_ascii_lowercase() || c == '_') => "lower",
                    w if is_reserved(w) => "lower",
                    _ => "expected",
                },
                _ => "expected",
            }
        };
        self.perr(code::BAD_TYPE_TEXT, tok.diag, Rule::BadType(template))
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
                            return Err(self.bad_type_text(lt, "map-open"));
                        }
                        self.bump();
                        self.open(lt.span)?;
                        let inner = self.ptype()?;
                        let gt = self.peek();
                        if !gt.is_punct(P::Gt) {
                            return Err(self.bad_type_text(gt, "map-close"));
                        }
                        self.bump();
                        self.close();
                        return Ok((PPrim::Map(Box::new(inner)), t.span.to(gt.span)));
                    }
                    w if is_type_name(w) => {
                        self.bump();
                        return Ok((PPrim::Name(w.to_string(), t.span), t.span));
                    }
                    _ => return Err(self.bad_type_text(t, "expected")),
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
                    return Err(self.bad_type_text(rp, "paren"));
                }
                self.bump();
                self.close();
                Ok((PPrim::Paren(Box::new(inner)), t.span.to(rp.span)))
            }
            Tok::Punct(P::LBrace) => self.precord(),
            _ => Err(self.bad_type_text(t, "expected")),
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
                    return Err(self.bad_type_text(n, "record"));
                }
                break;
            }
            let key = match &t.tok {
                Tok::Word(w) => w.clone(),
                Tok::Str(s, f) if f.is_line() && t.code.is_none() => s.clone(),
                _ => return Err(self.bad_type_text(t, "record")),
            };
            if fields.iter().any(|f| f.key == key) {
                return Err(self.bad_type_text(t, "duplicate"));
            }
            self.bump();
            let mut optional = false;
            if self.at_punct(P::Question) {
                self.bump();
                optional = true;
            }
            let c = self.peek();
            if !c.is_punct(P::Colon) {
                return Err(self.bad_type_text(c, "record"));
            }
            self.bump();
            let ty = self.ptype()?;
            fields.push(PField { key, optional, ty });
            let n = self.peek();
            if n.is_punct(P::Comma) {
                self.bump();
            } else if !n.is_punct(P::RBrace) {
                return Err(self.bad_type_text(n, "record"));
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
            return Err(self.perr(code::UNEXPECTED_TOKEN, p, Rule::Power));
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
                    self.mark[self.pos] = MARK_INDEX;
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
                            self.mark[self.pos] = MARK_INDEX;
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
            self.reserved_param()?;
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
                "if" | "try" => Err(self.perr(code::UNEXPECTED_TOKEN, t.span, Rule::OperandIf)),
                "_" => Err(self.unexpected(t)),
                w if is_reserved(w) => Err(self.reserved(t, Rule::Value)),
                _ => {
                    let wi = self.pos;
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
                        self.mark[wi] = MARK_OPERAND_WORD;
                        Ok(self.node(NodeKind::Ident { name: w.clone() }, nt.span))
                    }
                }
            },
            Tok::Punct(P::LParen) => {
                self.mark[self.pos] = MARK_PAREN;
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
            _ => {
                let e = self.unexpected(t);
                Err(self.tag(e, Rule::Operand))
            }
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
        self.mark[self.pos] = MARK_MAP;
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
                    return Err(self.reserved(t, Rule::Shorthand));
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
                self.key_colon()?;
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
                self.key_colon()?;
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
            let e = self.unexpected(t);
            return Err(self.tag(e, Rule::MatchSubject));
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
        self.arm_start = self.pos;
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
            Tok::Word(w) if is_reserved(w) => return Err(self.reserved(t, Rule::Value)),
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
            return Err(self.perr(code::UNEXPECTED_TOKEN, arrow.span, Rule::LambdaPattern));
        }
        self.mark[self.pos] = MARK_LAMBDA_ARROW;
        self.bump();
        let body = self.expr()?;
        let span = start.to(self.outer(body));
        Ok(self.node(NodeKind::Lambda { params, body }, span))
    }

    fn param(&mut self) -> R<NodeId> {
        let pattern = self.pattern()?;
        self.param_end = self.pos;
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
            Tok::Word(w) if is_reserved(w) => Err(self.reserved(t, Rule::Name)),
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
        self.mark[self.pos] = MARK_MAP_PATTERN;
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
                self.key_colon()?;
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

    /// The `:` after a string key or a computed key.
    fn key_colon(&mut self) -> R<()> {
        if self.at_punct(P::Colon) {
            self.bump();
            return Ok(());
        }
        let e = self.unexpected(self.peek());
        Err(self.tag(e, Rule::Key))
    }

    /// 14.2 rule 1: a reserved word followed by `=>`, where an argument or
    /// the value of a `const` may stand, is a parameter.
    fn reserved_param(&self) -> R<()> {
        let t = self.peek();
        if t.word().is_some_and(is_reserved) && self.peek_n(1).is_punct(P::Arrow) {
            return Err(self.reserved(t, Rule::Name));
        }
        Ok(())
    }

    // ---- the first diagnostic (`f08-errors.md` 2 and 3) -------------------

    /// The diagnostic of a parse error: the Core diagnostic mapped by the
    /// rows of `f08-errors.md` 2, with its message (3.4).
    #[cold]
    fn finish(&self, e: &PErr) -> Diagnostic {
        let (code, template, span, extra) = if self.core {
            let template = match e.rule {
                Rule::Name => "name",
                Rule::BadType(t) => t,
                _ => "",
            };
            (e.code, template, e.span, Vec::new())
        } else {
            self.target(e)
        };
        let msg = super::message::parse_message(self.src, code, template, span, &extra);
        Diagnostic::at(Phase::Parse, code, self.src, span, msg)
    }

    fn is_opener(t: &Token) -> bool {
        matches!(
            t.tok,
            Tok::Punct(P::LParen | P::LBracket | P::QDotBracket | P::LBrace)
        )
    }

    /// The innermost opener that is still open at token `i`.
    fn enclosing(&self, i: usize) -> Option<usize> {
        (0..i).rev().find(|&j| {
            Self::is_opener(&self.t[j]) && self.partner[j].is_none_or(|c| c as usize >= i)
        })
    }

    /// Whether token `i` is the `{` of a map literal that is the body of a
    /// lambda: the token before it is the lambda's `=>`.
    fn lambda_map(&self, i: usize) -> bool {
        i >= 1 && self.mark[i] == MARK_MAP && self.mark[i - 1] == MARK_LAMBDA_ARROW
    }

    /// The code, the template, the span and the other parameters of the
    /// first diagnostic (`f08-errors.md` 2.1).
    fn target(&self, e: &PErr) -> Target {
        let ti = self
            .t
            .partition_point(|t| t.span.start < e.span.start)
            .min(self.t.len() - 1);
        let t = &self.t[ti];
        let keep = |template: &'static str| (e.code, template, e.span, Vec::new());
        match (e.code, e.rule) {
            (code::RESERVED_WORD, Rule::Name) => keep("name"),
            (code::RESERVED_WORD, _) => {
                // 2.4.
                let c = match t.word().unwrap_or_default() {
                    "let" | "var" => "let-binding",
                    "for" | "while" | "do" | "break" | "continue" => "loop-statement",
                    "return" => "return-statement",
                    "function" => "function-declaration",
                    "throw" => "throw-statement",
                    "finally" => "finally-clause",
                    "import" => "import-statement",
                    "nil" => "nil-literal",
                    _ if ti >= 1 && self.lambda_map(ti - 1) => {
                        return (
                            "block-without-const",
                            "default",
                            self.t[ti - 1].span,
                            Vec::new(),
                        )
                    }
                    _ if e.rule == Rule::Shorthand => return keep("name"),
                    _ => return keep("operand"),
                };
                (c, "default", e.span, Vec::new())
            }
            (code::BAD_TYPE_TEXT, Rule::BadType(template)) => {
                let mut extra = Vec::new();
                if template == "duplicate" {
                    let key = match &t.tok {
                        Tok::Word(w) => w.clone(),
                        Tok::Str(s, _) => s.clone(),
                        _ => String::new(),
                    };
                    extra.push(("key", crate::messages::json_string(&key)));
                }
                (e.code, template, e.span, extra)
            }
            (code::UNEXPECTED_TOKEN, _) => self.target_token(e, ti),
            _ => keep(""),
        }
    }

    /// 2.5 to 2.7, for `unexpected-token`.
    fn target_token(&self, e: &PErr, ti: usize) -> Target {
        let t = &self.t[ti];
        let prev = |k: usize| ti.checked_sub(k).map(|j| &self.t[j]);
        let next = self.t.get(ti + 1);
        let at = |c: &'static str, template: &'static str| (c, template, e.span, Vec::new());
        let at_span = |c: &'static str, span: Span| {
            (c, "default", span, Vec::<(&'static str, String)>::new())
        };
        let src = self.src.chars();
        let char_at = |i: u32| src.get(i as usize).copied();

        // 2.5: tokens that no rule accepts.
        if matches!(t.tok, Tok::Refused) {
            let text = self.src.slice(t.span);
            match text.as_str() {
                "===" => return at("strict-equality", "equal"),
                "!==" => return at("strict-equality", "not-equal"),
                "+=" | "-=" | "*=" | "/=" | "%=" | "**=" | "??=" | "||=" | "&&=" => {
                    return at("assignment", "compound")
                }
                "++" => return at("assignment", "increment"),
                "|>" => return at("pipe-operator", "default"),
                _ => {}
            }
            let quotes = |i: u32| (i..i + 3).all(|k| char_at(k) == Some('\''));
            if quotes(t.span.start) || (self.src.slice(t.diag) == "r" && quotes(t.diag.end)) {
                return at("text-block-quote", "default");
            }
            let word_end = text
                .find(|c: char| !(c.is_ascii_alphanumeric() || c == '_'))
                .unwrap_or(0);
            if word_end > 0 {
                let after_dot = prev(1).is_some_and(|p| p.is_punct(P::Dot) || p.is_punct(P::QDot));
                if e.rule == Rule::Operand || after_dot {
                    return if text[word_end..].starts_with('`') {
                        at("tagged-template", "default")
                    } else {
                        at("string-prefix", "prefix")
                    };
                }
            }
            // Where the `:` after a key is needed, the template `key`.
            if e.rule == Rule::Key {
                return at(e.code, "key");
            }
            return at(e.code, "default");
        }

        // The rules of 3.4 (2.6), and C10.
        match e.rule {
            Rule::Prefix => return at("string-prefix", "space"),
            Rule::Power => return at("needs-parentheses", "power"),
            Rule::NotIn => return at("needs-parentheses", "in"),
            Rule::Coalesce => return at("needs-parentheses", "coalesce"),
            Rule::OperandIf => return at("needs-parentheses", "operand"),
            Rule::MatchSubject => return at("needs-parentheses", "match-subject"),
            Rule::BranchMap => return at("branch-like-map", "default"),
            Rule::EmptyBranch => return at("empty-branch", "default"),
            Rule::LambdaPattern => return at("lambda-in-pattern", "default"),
            Rule::TryCall => return at("try-call", "default"),
            Rule::MissingElse => return at("missing-else", "default"),
            Rule::MissingCatch if t.is_word("finally") => return at("finally-clause", "default"),
            Rule::MissingCatch => return at("missing-catch", "default"),
            Rule::TryForm => return at("try-form", "default"),
            Rule::CatchName => return at("catch-name", "default"),
            Rule::NotTest(is) => {
                let op = if is { "is" } else { "as" };
                return (
                    "needs-parentheses",
                    "not-test",
                    e.span,
                    vec![("op", op.to_string())],
                );
            }
            _ => {}
        }
        // 3.4 rule 4: after the last branch of an `if` or the handler of a
        // `try`, an operator or a step.
        if ti == self.if_end {
            let step = matches!(
                t.tok,
                Tok::Punct(
                    P::OrOr
                        | P::AndAnd
                        | P::EqEq
                        | P::NotEq
                        | P::Lt
                        | P::Le
                        | P::Gt
                        | P::Ge
                        | P::Plus
                        | P::Minus
                        | P::Star
                        | P::Slash
                        | P::Percent
                        | P::StarStar
                        | P::QQ
                        | P::Dot
                        | P::QDot
                        | P::LBracket
                        | P::QDotBracket
                        | P::Question
                        | P::LParen
                )
            ) || matches!(t.word(), Some("in" | "not" | "as" | "is"));
            if step {
                return at("needs-parentheses", "after-branch");
            }
        }

        // 2.7, in order.
        let enclosing = self.enclosing(ti);
        let operand_word = |k: usize, words: &[&str]| {
            ti >= k
                && self.mark[ti - k] == MARK_OPERAND_WORD
                && self.t[ti - k].word().is_some_and(|w| words.contains(&w))
        };
        if e.rule == Rule::AsConst {
            return at("as-const", "default");
        }
        if e.rule == Rule::IsNot {
            return at("negated-is", "default");
        }
        if t.is_word("finally") {
            return at("finally-clause", "default");
        }
        if e.rule == Rule::IfForm {
            return at("if-form", "form");
        }
        if t.is_punct(P::Comma) && enclosing.is_some_and(|o| self.mark[o] == MARK_IF_TEST) {
            return at("if-form", "call");
        }
        if prev(1).is_some_and(|p| p.is_word("try"))
            && !t.is_punct(P::LBrace)
            && !t.is_punct(P::LParen)
        {
            return at("try-form", "default");
        }
        if t.is_punct(P::Assign) && ti == self.expr_end {
            return at("assignment", "assign");
        }
        if t.is_punct(P::Colon) && enclosing.is_some_and(|o| self.mark[o] == MARK_INDEX) {
            return at("slice-syntax", "default");
        }
        if t.is_punct(P::Colon)
            && (ti == self.const_pat_end
                || ti == self.param_end
                || (ti >= 2
                    && self.t[ti - 1].word().is_some_and(is_name)
                    && self.mark[ti - 2] == MARK_PAREN))
        {
            return at("type-annotation", "default");
        }
        if t.is_punct(P::Pipe) {
            return at("bitwise-operator", "or");
        }
        if (t.is_punct(P::Star)
            && prev(1).is_some_and(|p| p.is_punct(P::Dot) || p.is_punct(P::QDot)))
            || (t.is_punct(P::RBracket) && ti >= 1 && self.mark[ti - 1] == MARK_INDEX)
        {
            return at("star-form", "default");
        }
        if t.is_punct(P::Question) && ti >= 1 && self.mark[ti - 1] == MARK_INDEX {
            return at("filter-step", "default");
        }
        if t.is_word("satisfies") {
            return at("satisfies-operator", "default");
        }
        if t.is_word("instanceof") {
            return at("instanceof-operator", "default");
        }
        // E15.
        if operand_word(1, &["switch", "case"]) {
            return at_span("switch-statement", self.t[ti - 1].span);
        }
        if t.is_punct(P::LBrace) && prev(1).is_some_and(|p| p.is_punct(P::RParen)) {
            let close = ti - 1;
            let open = (0..close)
                .rev()
                .find(|&j| self.partner[j] == Some(close as u32));
            if let Some(o) = open {
                let named = o >= 1
                    && self.t[o].is_punct(P::LParen)
                    && self.t[o - 1].is_word("switch")
                    && !(o >= 2
                        && (self.t[o - 2].is_punct(P::Dot) || self.t[o - 2].is_punct(P::QDot)));
                if named {
                    return at_span("switch-statement", self.t[o - 1].span);
                }
            }
        }
        if t.is_word("case") && ti == self.arm_start {
            return at_span("switch-statement", t.span);
        }
        if operand_word(1, &["typeof"]) {
            return at_span("typeof-operator", self.t[ti - 1].span);
        }
        if operand_word(1, &["interface", "enum", "type"]) {
            let span = self.t[ti - 1].span;
            return ("type-declaration", "default", span, Vec::new());
        }
        if operand_word(1, &["export"]) {
            return at_span("export-statement", self.t[ti - 1].span);
        }
        if t.is_punct(P::Lt) && e.rule == Rule::Operand {
            return if char_at(t.span.end) == Some('<') {
                at("heredoc", "default")
            } else {
                at("angle-cast", "default")
            };
        }
        if let Rule::CondColon(q) = e.rule {
            let q = &self.t[q as usize];
            if char_at(q.span.end) == Some('[') {
                return at_span("optional-index", q.span);
            }
        }
        if let Some(r) = self.unquoted_key(ti, enclosing) {
            return r;
        }
        // E23, but not for a `=` where the `:` after a key is needed.
        let key_eq = e.rule == Rule::Key && t.is_punct(P::Assign);
        if let Some(o) = enclosing.filter(|&o| self.lambda_map(o) && !key_eq) {
            let mut depth = 0i32;
            let mut plain = true;
            for k in o + 1..ti {
                let x = &self.t[k];
                if Self::is_opener(x) {
                    depth += 1;
                } else if matches!(x.tok, Tok::Punct(P::RParen | P::RBracket | P::RBrace)) {
                    depth -= 1;
                } else if depth == 0
                    && (x.is_punct(P::Colon) || x.is_punct(P::Comma) || x.is_punct(P::Ellipsis))
                {
                    plain = false;
                    break;
                }
            }
            if plain {
                return at_span("block-without-const", self.t[o].span);
            }
        }
        if ti == self.type_end
            && matches!(
                t.tok,
                Tok::Punct(
                    P::Plus
                        | P::Minus
                        | P::Star
                        | P::Slash
                        | P::Percent
                        | P::StarStar
                        | P::Dot
                        | P::QDot
                        | P::QDotBracket
                        | P::LBracket
                        | P::LParen
                        | P::LBrace
                )
            )
        {
            return at("needs-parentheses", "after-type");
        }
        if t.is_punct(P::Bang) && ti == self.expr_end && next.is_some_and(|n| n.is_word("is")) {
            return at("negated-is", "default");
        }
        if e.rule == Rule::Key {
            return at(e.code, "key");
        }
        at(e.code, "default")
    }

    /// E22: a `-` in a map literal or a map pattern, in an entry or a field
    /// that begins with a word joined to words or numbers by `-`, then `:`.
    fn unquoted_key(&self, ti: usize, enclosing: Option<usize>) -> Option<Target> {
        let o = enclosing?;
        if !self.t[ti].is_punct(P::Minus)
            || !(self.mark[o] == MARK_MAP || self.mark[o] == MARK_MAP_PATTERN)
            || ti < 2
        {
            return None;
        }
        let w = &self.t[ti - 1];
        w.word()?;
        let before = &self.t[ti - 2];
        if !(ti - 2 == o || before.is_punct(P::Comma)) {
            return None;
        }
        let mut j = ti;
        let mut last = ti - 1;
        loop {
            let m = &self.t[j];
            let n = self.t.get(j + 1)?;
            let joined = m.is_punct(P::Minus)
                && m.span.start == self.t[last].span.end
                && n.span.start == m.span.end
                && matches!(n.tok, Tok::Word(_) | Tok::Number(_));
            if !joined {
                return None;
            }
            last = j + 1;
            j += 2;
            let after = self.t.get(j)?;
            if after.is_punct(P::Colon) {
                break;
            }
            if !after.is_punct(P::Minus) {
                return None;
            }
        }
        let span = w.span.to(self.t[last].span);
        let key = crate::messages::json_string(&self.src.slice(span));
        Some(("unquoted-key", "default", span, vec![("key", key)]))
    }
}
