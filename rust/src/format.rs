//! Package F12: the formatter (`f12-tools.md` 2).
//!
//! The tree is turned into a document (2.4) and the document is laid out
//! (2.5). Documents live in an arena, so a long chain of `? :` or of `else
//! if`, which the tree holds nested, is never dropped or walked by
//! recursion; the layout and the test of 2.5 item 7 use explicit stacks.
//!
//! Comments are attached to the kept tokens (2.12). Which tokens are kept is
//! known only once the document is built, so the builder runs twice: the
//! first run marks the kept tokens, the second, with the comments attached,
//! builds the document that is laid out.

use crate::diag::Error;
use crate::source::{Source, Span};
use crate::syntax::ast::{Ast, BinaryOp, CondForm, EntryKey, NodeId, NodeKind, UnaryOp};
use crate::syntax::lexer::{lex_comments, lex_part_comments};
use crate::syntax::token::{StrForm, Tok, Token, P};
use crate::template::{parse_template, Segment};

const WIDTH: i64 = 80;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
struct DocId(u32);

/// A text block, re-indented where it is written (2.7 item 2).
struct TextBlock {
    open: String,
    lines: Vec<BLine>,
}

enum BLine {
    Blank,
    /// A content line after the old indentation.
    Content(String),
    /// A line that begins inside an interpolation, as it stands.
    Raw(String),
}

enum D {
    Text(String),
    Space,
    Line,
    Soft,
    Hard,
    Blank,
    After,
    Indent(DocId),
    /// A group; `true` for a `broken-group`.
    Group(DocId, bool),
    IfBroken(DocId, DocId),
    /// The alternatives, and whether the rule says the last.
    Choice(Vec<DocId>, bool),
    Block(TextBlock),
    List(Vec<DocId>),
}

#[derive(Default)]
struct Docs {
    d: Vec<D>,
    forced: Vec<bool>,
}

impl Docs {
    fn f(&self, x: DocId) -> bool {
        self.forced[x.0 as usize]
    }

    fn push(&mut self, d: D) -> DocId {
        let f = match &d {
            D::Text(_) | D::Space | D::Line | D::Soft | D::Block(_) => false,
            D::Hard | D::Blank | D::After => true,
            D::Indent(x) => self.f(*x),
            D::Group(x, b) => *b || self.f(*x),
            D::IfBroken(_, b) => self.f(*b),
            D::Choice(a, _) => self.f(a[0]),
            D::List(xs) => xs.iter().any(|x| self.f(*x)),
        };
        self.d.push(d);
        self.forced.push(f);
        DocId(self.d.len() as u32 - 1)
    }

    fn get(&self, x: DocId) -> &D {
        &self.d[x.0 as usize]
    }
}

// ---- the layout (2.5) ----------------------------------------------------------

struct Out {
    s: String,
    /// Code points since the last line feed.
    col: i64,
    /// Something but spaces since the last line feed.
    has_text: bool,
    pending: bool,
}

impl Out {
    fn strip(&mut self) {
        while self.s.ends_with(' ') {
            self.s.pop();
            self.col -= 1;
        }
    }

    fn brk(&mut self, ind: usize) {
        self.pending = false;
        self.strip();
        if self.has_text {
            self.s.push('\n');
        }
        for _ in 0..ind {
            self.s.push(' ');
        }
        self.col = ind as i64;
        self.has_text = false;
    }

    fn blank(&mut self, ind: usize) {
        self.pending = false;
        self.strip();
        if self.s.is_empty() {
            return;
        }
        while !self.s.ends_with("\n\n") {
            self.s.push('\n');
        }
        for _ in 0..ind {
            self.s.push(' ');
        }
        self.col = ind as i64;
        self.has_text = false;
    }

    fn write(&mut self, t: &str) {
        self.s.push_str(t);
        match t.rfind('\n') {
            Some(p) => {
                let tail = &t[p + 1..];
                self.col = tail.chars().count() as i64;
                self.has_text = tail.chars().any(|c| c != ' ');
            }
            None => {
                self.col += t.chars().count() as i64;
                self.has_text |= t.chars().any(|c| c != ' ');
            }
        }
    }

    /// The spaces that begin the current line.
    fn line_indent(&self) -> usize {
        let line = match self.s.rfind('\n') {
            Some(p) => &self.s[p + 1..],
            None => &self.s[..],
        };
        line.chars().take_while(|c| *c == ' ').count()
    }
}

fn layout(docs: &Docs, root: DocId, width: Option<i64>) -> String {
    let mut out = Out {
        s: String::new(),
        col: 0,
        has_text: false,
        pending: false,
    };
    let mut stack: Vec<(DocId, usize, bool)> = vec![(root, 0, false)];
    while let Some((d, ind, flat)) = stack.pop() {
        match docs.get(d) {
            D::Text(s) => {
                if out.pending {
                    out.brk(ind);
                }
                out.write(s);
            }
            D::Space => {
                if out.pending {
                    out.brk(ind);
                } else if out.has_text {
                    out.write(" ");
                }
            }
            D::Line if flat => {
                if out.pending {
                    out.brk(ind);
                } else if out.has_text {
                    out.write(" ");
                }
            }
            D::Line | D::Hard => out.brk(ind),
            D::Soft => {
                if !flat {
                    out.brk(ind);
                }
            }
            D::Blank => out.blank(ind),
            D::After => out.pending = true,
            D::Indent(x) => stack.push((*x, ind + 2, flat)),
            D::Group(x, b) => {
                let broken = *b || docs.f(*x) || (!flat && !fits(docs, *x, &stack, out.col, width));
                stack.push((*x, ind, !broken));
            }
            D::IfBroken(a, b) => stack.push((if flat { *b } else { *a }, ind, flat)),
            D::Choice(alts, must) => {
                let last = *alts.last().expect("an alternative");
                if flat {
                    stack.push((alts[0], ind, true));
                } else if *must {
                    stack.push((last, ind, false));
                } else {
                    let n = alts.len();
                    match alts[..n - 1]
                        .iter()
                        .find(|a| fits(docs, **a, &stack, out.col, width))
                    {
                        Some(a) => stack.push((*a, ind, true)),
                        None => stack.push((last, ind, false)),
                    }
                }
            }
            D::Block(tb) => {
                if out.pending {
                    out.brk(ind);
                }
                let j = " ".repeat(out.line_indent() + 2);
                let mut s = tb.open.clone();
                s.push('\n');
                for l in &tb.lines {
                    match l {
                        BLine::Blank => {}
                        BLine::Content(c) => {
                            s.push_str(&j);
                            s.push_str(c);
                        }
                        BLine::Raw(c) => s.push_str(c),
                    }
                    s.push('\n');
                }
                s.push_str(&j);
                s.push_str("\"\"\"");
                out.write(&s);
            }
            D::List(xs) => {
                for x in xs.iter().rev() {
                    stack.push((*x, ind, flat));
                }
            }
        }
    }
    out.s
}

/// 2.5 item 7: whether `d` fits at the column, laid out flat, with what
/// follows it on the stack in its own modes.
fn fits(
    docs: &Docs,
    d: DocId,
    rest: &[(DocId, usize, bool)],
    col: i64,
    width: Option<i64>,
) -> bool {
    let Some(w) = width else {
        return true;
    };
    let mut r = w - col;
    let mut st: Vec<(DocId, bool)> = vec![(d, true)];
    let mut ri = rest.len();
    loop {
        let (x, flat) = match st.pop() {
            Some(v) => v,
            None => {
                if ri == 0 {
                    return true;
                }
                ri -= 1;
                (rest[ri].0, rest[ri].2)
            }
        };
        match docs.get(x) {
            D::Text(s) => match s.find('\n') {
                Some(p) => {
                    r -= s[..p].chars().count() as i64;
                    return r >= 0;
                }
                None => {
                    r -= s.chars().count() as i64;
                    if r < 0 {
                        return false;
                    }
                }
            },
            D::Space => {
                r -= 1;
                if r < 0 {
                    return false;
                }
            }
            D::Line => {
                if !flat {
                    return true;
                }
                r -= 1;
                if r < 0 {
                    return false;
                }
            }
            D::Soft => {
                if !flat {
                    return true;
                }
            }
            D::Hard | D::Blank | D::After => return true,
            D::Indent(y) => st.push((*y, flat)),
            D::IfBroken(a, b) => st.push((if flat { *b } else { *a }, flat)),
            D::Group(y, b) => st.push((*y, flat && !*b && !docs.f(*y))),
            D::Choice(a, _) => st.push((if flat { a[0] } else { *a.last().unwrap() }, flat)),
            D::Block(tb) => {
                r -= tb.open.chars().count() as i64;
                return r >= 0;
            }
            D::List(xs) => {
                for y in xs.iter().rev() {
                    st.push((*y, flat));
                }
            }
        }
    }
}

// ---- tokens and comments ------------------------------------------------------

/// A token of the formatter: a template or text block with its
/// interpolations is one.
struct FTok {
    span: Span,
    tok: Tok,
}

struct Cmt {
    span: Span,
    line: bool,
    /// Line ends in the blank space before and after it.
    before: u32,
    after: u32,
}

fn line_ends(ch: &[char], a: usize, b: usize) -> u32 {
    let mut n = 0;
    let mut i = a;
    while i < b.min(ch.len()) {
        match ch[i] {
            '\n' => n += 1,
            '\r' => {
                n += 1;
                if ch.get(i + 1) == Some(&'\n') && i + 1 < b {
                    i += 1;
                }
            }
            _ => {}
        }
        i += 1;
    }
    n
}

/// The text of a span with each line end written U+000A.
fn norm(ch: &[char], span: Span) -> String {
    let mut s = String::new();
    let (a, b) = (span.start as usize, (span.end as usize).min(ch.len()));
    let mut i = a;
    while i < b {
        match ch[i] {
            '\r' => {
                s.push('\n');
                if ch.get(i + 1) == Some(&'\n') && i + 1 < b {
                    i += 1;
                }
            }
            c => s.push(c),
        }
        i += 1;
    }
    s
}

#[derive(Clone, Copy, PartialEq)]
enum Role {
    Left,
    Right,
}

enum El {
    Node(NodeId),
    PatField(NodeId, usize),
    PatRest(Span),
    ListElem(NodeId, usize),
}

struct B<'a> {
    ch: &'a [char],
    ast: &'a Ast,
    raw: &'a [Token],
    t: Vec<FTok>,
    partner: Vec<Option<usize>>,
    nl_before: Vec<u32>,
    cm: Vec<Cmt>,
    pass2: bool,
    kept: Vec<bool>,
    kidx: Vec<usize>,
    first_mark: Vec<bool>,
    close_mark: Vec<bool>,
    lead: Vec<Vec<usize>>,
    trail: Vec<Vec<usize>>,
    defer: Vec<bool>,
    docs: Docs,
}

impl<'a> B<'a> {
    fn new(ch: &'a [char], ast: &'a Ast, raw: &'a [Token], comments: &[Span]) -> B<'a> {
        // Merge each template and text block with its interpolations.
        let mut t: Vec<FTok> = Vec::new();
        let mut i = 0;
        while i < raw.len() {
            match &raw[i].tok {
                Tok::Eof => break,
                Tok::TmplHead(_) => {
                    let mut depth = 0;
                    let mut j = i;
                    while j < raw.len() {
                        match raw[j].tok {
                            Tok::TmplHead(_) => depth += 1,
                            Tok::TmplTail(_) => {
                                depth -= 1;
                                if depth == 0 {
                                    break;
                                }
                            }
                            _ => {}
                        }
                        j += 1;
                    }
                    let j = j.min(raw.len() - 1);
                    t.push(FTok {
                        span: Span {
                            start: raw[i].span.start,
                            end: raw[j].span.end,
                        },
                        tok: Tok::Str(String::new(), StrForm::Backtick),
                    });
                    i = j + 1;
                }
                tok => {
                    t.push(FTok {
                        span: raw[i].span,
                        tok: tok.clone(),
                    });
                    i += 1;
                }
            }
        }
        let n = t.len();
        let mut partner = vec![None; n];
        let mut st: Vec<usize> = Vec::new();
        for (k, x) in t.iter().enumerate() {
            match x.tok {
                Tok::Punct(P::LParen | P::LBracket | P::QDotBracket | P::LBrace) => st.push(k),
                Tok::Punct(P::RParen | P::RBracket | P::RBrace) => {
                    if let Some(o) = st.pop() {
                        partner[o] = Some(k);
                    }
                }
                _ => {}
            }
        }
        // Comments, less those inside a merged literal.
        let inside = |s: &Span| {
            t.iter()
                .any(|x| x.span.start < s.start && s.end <= x.span.end)
        };
        let cs: Vec<Span> = comments.iter().copied().filter(|c| !inside(c)).collect();
        // Items in order, for the blank space before and after each.
        let mut items: Vec<(u32, u32, bool, usize)> = t
            .iter()
            .enumerate()
            .map(|(k, x)| (x.span.start, x.span.end, false, k))
            .chain(
                cs.iter()
                    .enumerate()
                    .map(|(k, c)| (c.start, c.end, true, k)),
            )
            .collect();
        items.sort_by_key(|x| x.0);
        let mut nl_before = vec![0; n];
        let mut cm: Vec<Cmt> = cs
            .iter()
            .map(|c| Cmt {
                span: *c,
                line: ch.get(c.start as usize + 1) == Some(&'/'),
                before: 0,
                after: 0,
            })
            .collect();
        let mut prev_end = 0usize;
        for (idx, &(s, e, is_c, k)) in items.iter().enumerate() {
            let nl = line_ends(ch, prev_end, s as usize);
            if is_c {
                cm[k].before = nl;
                let next = items.get(idx + 1).map(|x| x.0 as usize).unwrap_or(ch.len());
                cm[k].after = line_ends(ch, e as usize, next);
            } else {
                nl_before[k] = nl;
            }
            prev_end = e as usize;
        }
        B {
            ch,
            ast,
            raw,
            t,
            partner,
            nl_before,
            cm,
            pass2: false,
            kept: vec![false; n],
            kidx: Vec::new(),
            first_mark: vec![false; n + 1],
            close_mark: vec![false; n + 1],
            lead: vec![Vec::new(); n + 1],
            trail: vec![Vec::new(); n + 1],
            defer: vec![false; n + 1],
            docs: Docs::default(),
        }
    }

    /// After the first run: attach each comment (2.12 items 2 and 3).
    fn attach(&mut self) {
        self.kidx = (0..self.t.len()).filter(|&k| self.kept[k]).collect();
        for ci in 0..self.cm.len() {
            let c = self.cm[ci].span;
            let p = self.t.partition_point(|x| x.span.end <= c.start);
            let trailing =
                p > 0 && line_ends(self.ch, self.t[p - 1].span.end as usize, c.start as usize) == 0;
            if trailing {
                let q = self
                    .kidx
                    .partition_point(|&k| self.t[k].span.end <= c.start);
                if q > 0 {
                    self.trail[self.kidx[q - 1]].push(ci);
                    continue;
                }
            }
            let q = self.kidx.partition_point(|&k| self.t[k].span.start < c.end);
            let at = self.kidx.get(q).copied().unwrap_or(self.t.len());
            self.lead[at].push(ci);
        }
        self.pass2 = true;
        self.docs = Docs::default();
    }

    // ---- token lookup ----

    /// The first token that starts at or after `pos`.
    fn ti(&self, pos: u32) -> usize {
        self.t.partition_point(|x| x.span.start < pos)
    }

    /// The last token that ends at or before `pos`.
    fn tb(&self, pos: u32) -> usize {
        self.t
            .partition_point(|x| x.span.end <= pos)
            .saturating_sub(1)
    }

    fn last_kept(&self, end: u32) -> Option<usize> {
        let q = self.kidx.partition_point(|&k| self.t[k].span.end <= end);
        (q > 0).then(|| self.kidx[q - 1])
    }

    fn first_kept(&self, start: u32) -> Option<usize> {
        if self.pass2 {
            let q = self.kidx.partition_point(|&k| self.t[k].span.start < start);
            return self.kidx.get(q).copied();
        }
        (self.ti(start)..self.t.len()).find(|&k| self.kept[k])
    }

    fn is_closer(&self, i: usize) -> bool {
        matches!(
            self.t.get(i).map(|x| &x.tok),
            Some(Tok::Punct(P::RParen | P::RBracket | P::RBrace))
        )
    }

    // ---- documents ----

    fn push(&mut self, d: D) -> DocId {
        self.docs.push(d)
    }

    fn text(&mut self, s: &str) -> DocId {
        self.push(D::Text(s.to_string()))
    }

    fn list(&mut self, v: Vec<DocId>) -> DocId {
        self.push(D::List(v))
    }

    fn empty(&mut self) -> DocId {
        self.list(Vec::new())
    }

    fn group(&mut self, d: DocId) -> DocId {
        self.push(D::Group(d, false))
    }

    fn indent(&mut self, d: DocId) -> DocId {
        self.push(D::Indent(d))
    }

    fn ib(&mut self, a: DocId, b: DocId) -> DocId {
        self.push(D::IfBroken(a, b))
    }

    fn sp(&mut self) -> DocId {
        self.push(D::Space)
    }

    fn line(&mut self) -> DocId {
        self.push(D::Line)
    }

    fn soft(&mut self) -> DocId {
        self.push(D::Soft)
    }

    fn hard(&mut self) -> DocId {
        self.push(D::Hard)
    }

    fn comment_text(&self, ci: usize) -> String {
        let c = &self.cm[ci];
        let s = norm(self.ch, c.span);
        if c.line {
            s.trim_end_matches([' ', '\t']).to_string()
        } else {
            s
        }
    }

    /// The leading comments of token `i` (or of the end of the text), 2.12
    /// items 6 and 7.
    fn lead_doc(&mut self, i: usize) -> DocId {
        if !self.pass2 || self.lead[i].is_empty() {
            return self.empty();
        }
        let cs = self.lead[i].clone();
        let end = i == self.t.len();
        let closer = end || self.is_closer(i);
        let blank_ok = self.first_mark[i] || self.close_mark[i] || end;
        let mut v = Vec::new();
        for (j, &ci) in cs.iter().enumerate() {
            let (line, before, after) = (self.cm[ci].line, self.cm[ci].before, self.cm[ci].after);
            v.push(if j == 0 && blank_ok && before >= 2 {
                self.push(D::Blank)
            } else {
                self.hard()
            });
            let s = self.comment_text(ci);
            v.push(self.text(&s));
            if line || after >= 1 {
                let last = j + 1 == cs.len();
                v.push(if after >= 2 && !(closer && last) {
                    self.push(D::Blank)
                } else {
                    self.hard()
                });
            } else {
                v.push(self.sp());
            }
        }
        self.list(v)
    }

    /// The trailing comments of token `i` (2.12 item 4). `opener` is
    /// `Some(has elements)` for the opening bracket of a sequence.
    fn trail_doc(&mut self, i: usize, opener: Option<bool>) -> DocId {
        if !self.pass2 || self.trail[i].is_empty() {
            return self.empty();
        }
        let cs = self.trail[i].clone();
        let mut v = Vec::new();
        for (j, &ci) in cs.iter().enumerate() {
            let line = self.cm[ci].line;
            let s = self.comment_text(ci);
            match opener {
                Some(has) if !line && j == 0 => {
                    v.push(self.text(&s));
                    if has {
                        v.push(self.sp());
                    }
                }
                _ => {
                    v.push(self.sp());
                    v.push(self.text(&s));
                }
            }
            if line {
                v.push(self.push(D::After));
            }
        }
        self.list(v)
    }

    fn tok_doc(
        &mut self,
        i: usize,
        text: Option<DocId>,
        lead: bool,
        opener: Option<bool>,
    ) -> DocId {
        if i >= self.t.len() {
            return self.empty();
        }
        self.kept[i] = true;
        let txt = match text {
            Some(d) => d,
            None => {
                let s = norm(self.ch, self.t[i].span);
                self.text(&s)
            }
        };
        if !self.pass2 {
            return txt;
        }
        let mut v = Vec::new();
        if lead {
            v.push(self.lead_doc(i));
        }
        v.push(txt);
        if !self.defer[i] {
            v.push(self.trail_doc(i, opener));
        }
        self.list(v)
    }

    fn tk(&mut self, i: usize) -> DocId {
        self.tok_doc(i, None, true, None)
    }

    fn tk_at(&mut self, pos: u32) -> DocId {
        let i = self.ti(pos);
        self.tk(i)
    }

    // ---- elements (2.9, 2.12 item 5) ----

    fn elem_begin(&mut self, outer: Span) -> Option<usize> {
        if !self.pass2 {
            return None;
        }
        let last = self.last_kept(outer.end)?;
        self.defer[last] = true;
        Some(last)
    }

    fn elem_end(&mut self, last: Option<usize>, outer: Span, not_first: bool) -> DocId {
        if !self.pass2 {
            if not_first {
                if let Some(k) = self.first_kept(outer.start) {
                    self.first_mark[k] = true;
                }
            }
            return self.empty();
        }
        match last {
            Some(l) => {
                self.defer[l] = false;
                self.trail_doc(l, None)
            }
            None => self.empty(),
        }
    }

    fn blank_before(&self, outer: Span) -> bool {
        if !self.pass2 {
            return false;
        }
        let s = self.ti(outer.start);
        let k = self.first_kept(outer.start);
        s < self.t.len() && self.nl_before[s] >= 2 && k.is_none_or(|k| self.lead[k].is_empty())
    }

    fn outer(&self, id: NodeId) -> Span {
        self.ast.node(id).outer
    }

    fn kind(&self, id: NodeId) -> &'a NodeKind {
        &self.ast.nodes[id.index()].kind
    }

    fn el_span(&self, e: &El) -> Span {
        match e {
            El::Node(id) => self.outer(*id),
            El::PatField(p, k) => match self.kind(*p) {
                NodeKind::MapPat { fields, .. } => {
                    let f = &fields[*k];
                    let end = f.default.map(|d| self.outer(d).end).unwrap_or(f.span.end);
                    Span {
                        start: f.span.start,
                        end: end.max(f.span.end),
                    }
                }
                _ => Span::default(),
            },
            El::PatRest(s) => *s,
            El::ListElem(p, k) => match self.kind(*p) {
                NodeKind::ListPat { elems, .. } => {
                    let e = &elems[*k];
                    let end = e.default.map(|d| self.outer(d).end).unwrap_or(e.span.end);
                    Span {
                        start: e.span.start,
                        end: end.max(e.span.end),
                    }
                }
                _ => Span::default(),
            },
        }
    }

    fn el_doc(&mut self, e: &El) -> DocId {
        match e {
            El::Node(id) => self.item(*id),
            El::PatField(p, k) => self.pat_field(*p, *k),
            El::PatRest(s) => {
                let dots = self.ti(s.start);
                let a = self.tk(dots);
                let b = self.tk(dots + 1);
                self.list(vec![a, b])
            }
            El::ListElem(p, k) => {
                let NodeKind::ListPat { elems, .. } = self.kind(*p) else {
                    return self.empty();
                };
                let (pat, def) = (elems[*k].pat, elems[*k].default);
                let pd = self.pattern(pat);
                match def {
                    Some(d) => {
                        let rest = self.default(d);
                        self.list(vec![pd, rest])
                    }
                    None => pd,
                }
            }
        }
    }

    /// `seq(o, xs, c)` (2.9).
    fn seq(&mut self, open: usize, close: usize, elems: Vec<El>) -> DocId {
        if elems.is_empty() {
            let o = self.tok_doc(open, None, true, Some(false));
            let c = self.tk(close);
            return self.list(vec![o, c]);
        }
        if !self.pass2 && close < self.close_mark.len() {
            self.close_mark[close] = true;
        }
        let o = self.tok_doc(open, None, true, Some(true));
        let n = elems.len();
        let mut inner = Vec::new();
        for (k, e) in elems.iter().enumerate() {
            let outer = self.el_span(e);
            inner.push(if k == 0 { self.soft() } else { self.line() });
            if k > 0 && self.blank_before(outer) {
                let b = self.push(D::Blank);
                let z = self.empty();
                inner.push(self.ib(b, z));
            }
            let last = self.elem_begin(outer);
            inner.push(self.el_doc(e));
            if k + 1 < n {
                inner.push(self.text(","));
            } else {
                let c = self.text(",");
                let z = self.empty();
                inner.push(self.ib(c, z));
            }
            inner.push(self.elem_end(last, outer, k > 0));
        }
        inner.push(self.lead_doc(close));
        let il = self.list(inner);
        let ind = self.indent(il);
        let s = self.soft();
        let c = self.tok_doc(close, None, false, None);
        let l = self.list(vec![o, ind, s, c]);
        self.group(l)
    }

    // ---- parentheses (2.6) ----

    fn level(&self, id: NodeId) -> u8 {
        match self.kind(id) {
            NodeKind::Cond { .. } | NodeKind::Try { .. } => 1,
            NodeKind::Binary { op, .. } => op_level(*op),
            NodeKind::As { .. } => 6,
            NodeKind::Unary { .. } => 9,
            _ => 11,
        }
    }

    fn bin_op(&self, id: NodeId) -> Option<BinaryOp> {
        match self.kind(id) {
            NodeKind::Binary { op, .. } => Some(*op),
            _ => None,
        }
    }

    fn is_not(&self, id: NodeId) -> bool {
        matches!(
            self.kind(id),
            NodeKind::Unary {
                op: UnaryOp::Not,
                ..
            }
        )
    }

    /// Parentheses around an operand of a binary operator.
    fn paren_bin(&self, op: BinaryOp, c: NodeId, role: Role) -> bool {
        let lc = self.level(c);
        let cop = self.bin_op(c);
        let coalesce = cop == Some(BinaryOp::Coalesce);
        match op {
            BinaryOp::Coalesce => match role {
                Role::Left => lc < 7 && !coalesce,
                Role::Right => lc < 7 || coalesce,
            },
            BinaryOp::Pow => match role {
                Role::Left => lc < 11,
                Role::Right => lc < 9,
            },
            _ => {
                let p = op_level(op);
                let muls = |o: BinaryOp| matches!(o, BinaryOp::Mul | BinaryOp::Div | BinaryOp::Rem);
                let eqs = |o: BinaryOp| matches!(o, BinaryOp::Eq | BinaryOp::Ne);
                let extra = match cop {
                    Some(BinaryOp::And) => op == BinaryOp::Or,
                    Some(o) if role == Role::Left && muls(o) && muls(op) => {
                        o != op || o == BinaryOp::Rem
                    }
                    Some(o) if role == Role::Left && eqs(o) && eqs(op) => true,
                    _ => false,
                };
                match role {
                    Role::Left => {
                        lc < p
                            || coalesce
                            || extra
                            || (matches!(op, BinaryOp::In | BinaryOp::NotIn) && self.is_not(c))
                    }
                    Role::Right => lc <= p || coalesce || extra,
                }
            }
        }
    }

    fn paren_receiver(&self, c: NodeId) -> bool {
        self.level(c) < 11
            || matches!(
                self.kind(c),
                NodeKind::OptChain { .. } | NodeKind::Each { .. }
            )
    }

    /// Whether the text of a node begins with `{`, `if` or `try`.
    fn begins_brace(&self, id: NodeId) -> bool {
        let mut n = id;
        loop {
            match self.kind(n) {
                NodeKind::Map { .. } | NodeKind::Block { .. } | NodeKind::Try { .. } => {
                    return true
                }
                NodeKind::Cond {
                    form: CondForm::If, ..
                } => return true,
                NodeKind::Cond { test, .. } => {
                    if self.level(*test) == 1 {
                        return false;
                    }
                    n = *test;
                }
                NodeKind::Binary { op, x, .. } => {
                    if self.paren_bin(*op, *x, Role::Left) {
                        return false;
                    }
                    n = *x;
                }
                NodeKind::As { x, .. } => {
                    if self.level(*x) < 6
                        || self.bin_op(*x) == Some(BinaryOp::Coalesce)
                        || self.is_not(*x)
                    {
                        return false;
                    }
                    n = *x;
                }
                NodeKind::OptChain { x } => n = *x,
                NodeKind::Member { x, .. }
                | NodeKind::Index { x, .. }
                | NodeKind::MethodCall { x, .. }
                | NodeKind::Each { x: Some(x), .. } => {
                    if self.paren_receiver(*x) && !matches!(self.kind(*x), NodeKind::Each { .. }) {
                        return false;
                    }
                    n = *x;
                }
                _ => return false,
            }
        }
    }

    fn parens(&mut self, d: DocId) -> DocId {
        let a = self.text("(");
        let b = self.text(")");
        self.list(vec![a, d, b])
    }

    fn sub(&mut self, id: NodeId, paren: bool) -> DocId {
        let d = self.node(id, true);
        if paren {
            self.parens(d)
        } else {
            d
        }
    }

    /// An item of a sequence: a spread, an entry, a parameter or an
    /// expression.
    fn item(&mut self, id: NodeId) -> DocId {
        match self.kind(id) {
            NodeKind::Param { pattern, default } => {
                let p = self.pattern(*pattern);
                match default {
                    Some(d) => {
                        let rest = self.default(*d);
                        self.list(vec![p, rest])
                    }
                    None => p,
                }
            }
            _ => self.sub(id, false),
        }
    }

    /// `sp "=" assigned(default)`.
    fn default(&mut self, d: NodeId) -> DocId {
        let s = self.sp();
        let eq = self.tb(self.outer(d).start);
        let e = self.tk(eq);
        let a = self.assigned(d);
        self.list(vec![s, e, a])
    }

    /// `assigned(v)` (2.8 item 2).
    fn assigned(&mut self, v: NodeId) -> DocId {
        match self.kind(v) {
            NodeKind::Binary { .. }
            | NodeKind::As { .. }
            | NodeKind::Cond {
                form: CondForm::Ternary,
                ..
            } => {
                let l = self.line();
                let d = self.node(v, false);
                let li = self.list(vec![l, d]);
                let i = self.indent(li);
                self.group(i)
            }
            _ => {
                let s = self.sp();
                let d = self.sub(v, false);
                self.list(vec![s, d])
            }
        }
    }

    // ---- nodes (2.8) ----

    /// The document of a node, without parentheses around it. `indent` is
    /// false for a run that is unindented (2.8 item 4).
    fn node(&mut self, id: NodeId, indent: bool) -> DocId {
        let span = self.ast.span(id);
        match self.kind(id) {
            NodeKind::Program { consts, result } => {
                let mut elems: Vec<NodeId> = consts.clone();
                elems.push(*result);
                let mut v = Vec::new();
                for (k, &e) in elems.iter().enumerate() {
                    let outer = self.outer(e);
                    if k > 0 && self.blank_before(outer) {
                        v.push(self.push(D::Blank));
                    }
                    let last = self.elem_begin(outer);
                    v.push(self.statement(e));
                    v.push(self.elem_end(last, outer, k > 0));
                    v.push(self.hard());
                }
                let end = self.t.len();
                v.push(self.lead_doc(end));
                self.list(v)
            }
            NodeKind::Const { pattern, value } => {
                let c = self.tk_at(span.start);
                let s1 = self.sp();
                let p = self.pattern(*pattern);
                let s2 = self.sp();
                let eq = self.tb(self.outer(*value).start);
                let e = self.tk(eq);
                let a = self.assigned(*value);
                let semi = self.ti(self.outer(*value).end);
                let sc = self.tk(semi);
                self.list(vec![c, s1, p, s2, e, a, sc])
            }
            NodeKind::Block { consts, result } => {
                let open = self.ti(span.start);
                let close = self.tb(span.end);
                let mut elems = consts.clone();
                elems.push(*result);
                let o = self.tk(open);
                let h = self.hard();
                let mut inner = vec![h];
                inner.extend(self.body(&elems));
                inner.push(self.lead_doc(close));
                let il = self.list(inner);
                let ind = self.indent(il);
                let h2 = self.hard();
                let c = self.tok_doc(close, None, false, None);
                self.list(vec![o, ind, h2, c])
            }
            NodeKind::Ident { .. }
            | NodeKind::Number { .. }
            | NodeKind::Bool { .. }
            | NodeKind::Null => self.tk_at(span.start),
            NodeKind::String { form, .. } => {
                let i = self.ti(span.start);
                if matches!(form, StrForm::TripleDouble | StrForm::RawTriple) {
                    let tb = self.text_block(span, &[]);
                    let d = self.push(D::Block(tb));
                    self.tok_doc(i, Some(d), true, None)
                } else {
                    self.tk(i)
                }
            }
            NodeKind::Template { parts } => {
                let i = self.ti(span.start);
                let tspan = self.t.get(i).map(|x| x.span).unwrap_or(span);
                let d = if self.ch.get(span.start as usize) == Some(&'`') {
                    let s = norm(self.ch, tspan);
                    self.text(&s)
                } else {
                    let exprs: Vec<NodeId> = parts
                        .iter()
                        .filter_map(|p| match p {
                            crate::syntax::ast::TemplatePart::Expr(e) => Some(*e),
                            _ => None,
                        })
                        .collect();
                    let tb = self.text_block(tspan, &exprs);
                    self.push(D::Block(tb))
                };
                self.tok_doc(i, Some(d), true, None)
            }
            NodeKind::List { items } => {
                let open = self.ti(span.start);
                let close = self.partner[open].unwrap_or(open);
                let els = items.iter().map(|&x| El::Node(x)).collect();
                self.seq(open, close, els)
            }
            NodeKind::Map { entries } => {
                let open = self.ti(span.start);
                let close = self.partner[open].unwrap_or(open);
                let els = entries.iter().map(|&x| El::Node(x)).collect();
                self.seq(open, close, els)
            }
            NodeKind::Entry {
                key,
                key_span,
                value,
                shorthand,
            } => {
                if *shorthand {
                    return self.tk_at(key_span.start);
                }
                let k = match key {
                    EntryKey::Computed(k) => {
                        let lb = self.tb(self.outer(*k).start);
                        let a = self.tk(lb);
                        let b = self.sub(*k, false);
                        let rb = self.ti(self.outer(*k).end);
                        let c = self.tk(rb);
                        self.list(vec![a, b, c])
                    }
                    _ => self.tk_at(key_span.start),
                };
                let colon = self.tb(self.outer(*value).start);
                let c = self.tk(colon);
                let a = self.assigned(*value);
                self.list(vec![k, c, a])
            }
            NodeKind::Spread { value } => {
                let d = self.tk_at(span.start);
                let v = self.sub(*value, false);
                self.list(vec![d, v])
            }
            NodeKind::Unary { op, x } => {
                let o = self.tk_at(span.start);
                let paren = self.level(*x) < 9 || self.bin_op(*x) == Some(BinaryOp::Pow);
                let mut v = vec![o];
                if *op == UnaryOp::Neg
                    && !paren
                    && matches!(
                        self.kind(*x),
                        NodeKind::Unary {
                            op: UnaryOp::Neg,
                            ..
                        }
                    )
                {
                    v.push(self.sp());
                }
                v.push(self.sub(*x, paren));
                self.list(v)
            }
            NodeKind::Binary { .. } => self.binary(id, indent),
            NodeKind::Cond {
                form: CondForm::If, ..
            } => self.if_chain(id),
            NodeKind::Cond { .. } => self.ternary(id),
            NodeKind::Member { .. }
            | NodeKind::Index { .. }
            | NodeKind::MethodCall { .. }
            | NodeKind::OptChain { .. }
            | NodeKind::Each { .. } => self.chain(id),
            NodeKind::Call {
                namespace,
                name,
                args,
            } => {
                let mut v = Vec::new();
                if let Some(ns) = namespace {
                    v.push(self.tk_at(ns.span.start));
                    let dot = self.ti(ns.span.end);
                    v.push(self.tk(dot));
                }
                v.push(self.tk_at(name.span.start));
                let open = self.ti(name.span.end);
                v.push(self.args(open, args));
                self.list(v)
            }
            NodeKind::Lambda { params, body } => {
                let bare = params.len() == 1
                    && matches!(
                        self.kind(params[0]),
                        NodeKind::Param {
                            pattern,
                            default: None
                        } if matches!(self.kind(*pattern), NodeKind::NamePat { .. })
                    );
                let p = if bare {
                    let NodeKind::Param { pattern, .. } = self.kind(params[0]) else {
                        unreachable!()
                    };
                    self.pattern(*pattern)
                } else {
                    let open = self.ti(span.start);
                    let close = self.partner[open].unwrap_or(open);
                    let els = params.iter().map(|&x| El::Node(x)).collect();
                    self.seq(open, close, els)
                };
                let s = self.sp();
                let arrow = self.tb(self.outer(*body).start);
                let a = self.tk(arrow);
                let b = self.assigned(*body);
                self.list(vec![p, s, a, b])
            }
            NodeKind::Param { .. } => self.item(id),
            NodeKind::Match { subject, arms } => self.match_(id, *subject, arms),
            NodeKind::Try {
                body,
                name,
                handler,
            } => {
                let tt = self.ti(span.start);
                let t = self.tk(tt);
                let s1 = self.sp();
                let bopen = tt + 1;
                let b = self.branch(bopen, *body);
                let catch = self.partner[bopen].unwrap_or(bopen) + 1;
                let s2 = self.sp();
                let c = self.tk(catch);
                let s3 = self.sp();
                let mut v = vec![t, s1, b, s2, c, s3];
                let hopen = if name.is_some() {
                    let lp = catch + 1;
                    v.push(self.tk(lp));
                    v.push(self.tk(lp + 1));
                    v.push(self.tk(lp + 2));
                    v.push(self.sp());
                    lp + 3
                } else {
                    catch + 1
                };
                v.push(self.branch(hopen, *handler));
                let l = self.list(v);
                self.group(l)
            }
            NodeKind::NamePat { .. } | NodeKind::MapPat { .. } | NodeKind::ListPat { .. } => {
                self.pattern(id)
            }
            NodeKind::As { x, .. } => {
                let paren = self.level(*x) < 6
                    || self.bin_op(*x) == Some(BinaryOp::Coalesce)
                    || self.is_not(*x);
                let a = self.sub(*x, paren);
                let s1 = self.sp();
                let w = self.ti(self.outer(*x).end);
                let k = self.tk(w);
                let s2 = self.sp();
                let NodeKind::As { ty, .. } = self.kind(id) else {
                    unreachable!()
                };
                let t = self.ptype(ty.span);
                self.list(vec![a, s1, k, s2, t])
            }
            NodeKind::Arm { .. } | NodeKind::Bad | NodeKind::Elem => self.empty(),
        }
    }

    /// A `const` line or a result.
    fn statement(&mut self, e: NodeId) -> DocId {
        if matches!(self.kind(e), NodeKind::Const { .. }) {
            self.node(e, true)
        } else {
            self.sub(e, false)
        }
    }

    /// The elements of a block or a branch, separated by `H` (2.8, 2.9).
    fn body(&mut self, elems: &[NodeId]) -> Vec<DocId> {
        let mut v = Vec::new();
        for (k, &e) in elems.iter().enumerate() {
            let outer = self.outer(e);
            if k > 0 {
                v.push(self.hard());
                if self.blank_before(outer) {
                    v.push(self.push(D::Blank));
                }
            }
            let last = self.elem_begin(outer);
            v.push(self.statement(e));
            v.push(self.elem_end(last, outer, k > 0));
        }
        v
    }

    /// A branch of `if` or `try`: `"{" i(L body) L "}"`.
    fn branch(&mut self, open: usize, content: NodeId) -> DocId {
        let close = self.partner.get(open).copied().flatten().unwrap_or(open);
        let elems: Vec<NodeId> = match self.kind(content) {
            NodeKind::Block { consts, result } => {
                let mut e = consts.clone();
                e.push(*result);
                e
            }
            _ => vec![content],
        };
        let o = self.tk(open);
        let l = self.line();
        let mut inner = vec![l];
        inner.extend(self.body(&elems));
        inner.push(self.lead_doc(close));
        let il = self.list(inner);
        let ind = self.indent(il);
        let l2 = self.line();
        let c = self.tok_doc(close, None, false, None);
        self.list(vec![o, ind, l2, c])
    }

    fn if_chain(&mut self, id: NodeId) -> DocId {
        let mut parts = Vec::new();
        let mut n = id;
        let mut first = true;
        while let NodeKind::Cond {
            test, then, else_, ..
        } = self.kind(n)
        {
            let mut it = self.ti(self.ast.span(n).start);
            if matches!(self.t.get(it).map(|x| &x.tok), Some(Tok::Punct(P::LBrace))) {
                it += 1;
            }
            if !first {
                parts.push(self.sp());
            }
            parts.push(self.tk(it));
            parts.push(self.sp());
            let lp = it + 1;
            let rp = self.partner[lp].unwrap_or(lp);
            parts.push(self.tk(lp));
            let s1 = self.soft();
            let td = self.node(*test, false);
            let tl = self.list(vec![s1, td]);
            let ti = self.indent(tl);
            let s2 = self.soft();
            let gl = self.list(vec![ti, s2]);
            parts.push(self.group(gl));
            parts.push(self.tk(rp));
            parts.push(self.sp());
            let bopen = rp + 1;
            parts.push(self.branch(bopen, *then));
            let else_tok = self.partner[bopen].unwrap_or(bopen) + 1;
            parts.push(self.sp());
            parts.push(self.tk(else_tok));
            parts.push(self.sp());
            if matches!(
                self.kind(*else_),
                NodeKind::Cond {
                    form: CondForm::If,
                    ..
                }
            ) {
                n = *else_;
                first = false;
                // The space before `if` is the one after `else`.
                parts.pop();
                continue;
            }
            parts.push(self.branch(else_tok + 1, *else_));
            break;
        }
        let l = self.list(parts);
        self.group(l)
    }

    fn ternary(&mut self, id: NodeId) -> DocId {
        // The chain of `? :` in its `else`, built from the innermost.
        let mut chain = Vec::new();
        let mut n = id;
        while let NodeKind::Cond {
            form: CondForm::Ternary,
            test,
            then,
            else_,
        } = self.kind(n)
        {
            chain.push((*test, *then, *else_));
            n = *else_;
        }
        let mut parts = Vec::new();
        for &(test, then, else_) in &chain {
            let tp = self.level(test) == 1;
            let td = self.sub(test, tp);
            let q = self.tb(self.outer(then).start);
            let qd = self.tk(q);
            let thd = self.sub(then, false);
            let c = self.tb(self.outer(else_).start);
            let cd = self.tk(c);
            parts.push((td, qd, thd, cd));
        }
        let mut below = self.sub(n, false);
        for (td, qd, thd, cd) in parts.into_iter().rev() {
            let l1 = self.line();
            let s1 = self.sp();
            let l2 = self.line();
            let s2 = self.sp();
            let il = self.list(vec![l1, qd, s1, thd, l2, cd, s2, below]);
            let i = self.indent(il);
            let gl = self.list(vec![td, i]);
            below = self.group(gl);
        }
        below
    }

    fn binary(&mut self, id: NodeId, indent: bool) -> DocId {
        let Some(op) = self.bin_op(id) else {
            return self.empty();
        };
        let p = op_level(op);
        let mut ops: Vec<NodeId> = Vec::new();
        let mut xs: Vec<NodeId> = Vec::new();
        let mut n = id;
        while let NodeKind::Binary { op: o, x, y } = self.kind(n) {
            xs.push(*y);
            ops.push(n);
            if let Some(xo) = self.bin_op(*x) {
                if op_level(xo) == p && xo != BinaryOp::Pow && !self.paren_bin(*o, *x, Role::Left) {
                    n = *x;
                    continue;
                }
            }
            xs.push(*x);
            break;
        }
        ops.reverse();
        xs.reverse();
        let op0 = self.bin_op(ops[0]).expect("a binary");
        let pl = self.paren_bin(op0, xs[0], Role::Left);
        let d0 = self.sub(xs[0], pl);
        let mut rest = Vec::new();
        let mut opdocs = Vec::new();
        for (i, &o) in ops.iter().enumerate() {
            let oop = self.bin_op(o).expect("a binary");
            let y = xs[i + 1];
            let k = self.tb(self.outer(y).start);
            let od = if oop == BinaryOp::NotIn {
                let a = self.tk(k.saturating_sub(1));
                let s = self.sp();
                let b = self.tk(k);
                self.list(vec![a, s, b])
            } else {
                self.tk(k)
            };
            opdocs.push(od);
            let pr = self.paren_bin(oop, y, Role::Right);
            rest.push(self.sub(y, pr));
        }
        // g(x1 sp o1 i(L x2 sp o2 L x3 ... L xn))
        let s = self.sp();
        let mut tail = Vec::new();
        for i in 0..rest.len() {
            tail.push(self.line());
            tail.push(rest[i]);
            if i + 1 < rest.len() {
                tail.push(self.sp());
                tail.push(opdocs[i + 1]);
            }
        }
        let tl = self.list(tail);
        let t = if indent { self.indent(tl) } else { tl };
        let l = self.list(vec![d0, s, opdocs[0], t]);
        self.group(l)
    }

    fn match_(&mut self, id: NodeId, subject: NodeId, arms: &[NodeId]) -> DocId {
        let span = self.ast.span(id);
        let m = self.tk_at(span.start);
        let s1 = self.sp();
        let sp = self.begins_brace(subject);
        let sd = self.sub(subject, sp);
        let s2 = self.sp();
        let lb = self.ti(self.outer(subject).end);
        let rb = self.partner[lb].unwrap_or(lb);
        let o = self.tk(lb);
        let n = arms.len();
        let mut inner = Vec::new();
        for (k, &a) in arms.iter().enumerate() {
            let outer = self.arm_span(a);
            inner.push(self.line());
            if k > 0 && self.blank_before(outer) {
                let b = self.push(D::Blank);
                let z = self.empty();
                inner.push(self.ib(b, z));
            }
            let last = self.elem_begin(outer);
            inner.push(self.arm(a));
            if k + 1 < n {
                inner.push(self.text(","));
            } else {
                let c = self.text(",");
                let z = self.empty();
                inner.push(self.ib(c, z));
            }
            inner.push(self.elem_end(last, outer, k > 0));
        }
        inner.push(self.lead_doc(rb));
        let il = self.list(inner);
        let ind = self.indent(il);
        let l = self.line();
        let c = self.tok_doc(rb, None, false, None);
        let gl = self.list(vec![m, s1, sd, s2, o, ind, l, c]);
        self.group(gl)
    }

    fn arm_span(&self, a: NodeId) -> Span {
        let s = self.ast.span(a);
        match self.kind(a) {
            NodeKind::Arm { result, .. } => Span {
                start: s.start,
                end: s.end.max(self.outer(*result).end),
            },
            _ => s,
        }
    }

    fn arm(&mut self, a: NodeId) -> DocId {
        let NodeKind::Arm { literals, result } = self.kind(a) else {
            return self.empty();
        };
        let mut v = Vec::new();
        match literals {
            None => v.push(self.tk_at(self.ast.span(a).start)),
            Some(ls) => {
                for (k, l) in ls.iter().enumerate() {
                    if k > 0 {
                        v.push(self.sp());
                        let bar = self.tb(l.span.start);
                        v.push(self.tk(bar));
                        v.push(self.sp());
                    }
                    let mut i = self.ti(l.span.start);
                    while i < self.t.len() && self.t[i].span.end <= l.span.end {
                        v.push(self.tk(i));
                        i += 1;
                    }
                }
            }
        }
        v.push(self.sp());
        let arrow = self.tb(self.outer(*result).start);
        v.push(self.tk(arrow));
        v.push(self.assigned(*result));
        self.list(v)
    }

    // ---- patterns ----

    fn pattern(&mut self, id: NodeId) -> DocId {
        let span = self.ast.span(id);
        match self.kind(id) {
            NodeKind::MapPat { fields, rest } => {
                let open = self.ti(span.start);
                let close = self.partner[open].unwrap_or(open);
                let mut els: Vec<El> = (0..fields.len()).map(|k| El::PatField(id, k)).collect();
                if let Some(r) = rest {
                    let dots = self.tb(r.span.start);
                    els.push(El::PatRest(Span {
                        start: self.t[dots].span.start,
                        end: r.span.end,
                    }));
                }
                self.seq(open, close, els)
            }
            NodeKind::ListPat { elems, rest } => {
                let open = self.ti(span.start);
                let close = self.partner[open].unwrap_or(open);
                let mut els: Vec<El> = (0..elems.len()).map(|k| El::ListElem(id, k)).collect();
                if let Some(r) = rest {
                    let dots = self.tb(r.span.start);
                    els.push(El::PatRest(Span {
                        start: self.t[dots].span.start,
                        end: r.span.end,
                    }));
                }
                self.seq(open, close, els)
            }
            _ => self.tk_at(span.start),
        }
    }

    fn pat_field(&mut self, p: NodeId, k: usize) -> DocId {
        let NodeKind::MapPat { fields, .. } = self.kind(p) else {
            return self.empty();
        };
        let f = &fields[k];
        let kt = self.ti(f.key_span.start);
        let word = matches!(self.t.get(kt).map(|x| &x.tok), Some(Tok::Word(_)));
        let mut v = Vec::new();
        if word && f.key == f.name.name {
            v.push(self.tk_at(f.name.span.start));
        } else {
            v.push(self.tk(kt));
            let colon = self.ti(f.key_span.end);
            v.push(self.tk(colon));
            v.push(self.sp());
            v.push(self.tk_at(f.name.span.start));
        }
        if let Some(d) = f.default {
            v.push(self.default(d));
        }
        self.list(v)
    }

    // ---- calls and chains ----

    fn huggable(&self, a: NodeId) -> bool {
        match self.kind(a) {
            NodeKind::Map { .. } | NodeKind::List { .. } | NodeKind::Block { .. } => true,
            NodeKind::Lambda { body, .. } => matches!(
                self.kind(*body),
                NodeKind::Map { .. }
                    | NodeKind::List { .. }
                    | NodeKind::Block { .. }
                    | NodeKind::Match { .. }
                    | NodeKind::Try { .. }
                    | NodeKind::Cond {
                        form: CondForm::If,
                        ..
                    }
            ),
            _ => false,
        }
    }

    /// The arguments of a call (2.10), from the `(` at `open`.
    fn args(&mut self, open: usize, args: &[NodeId]) -> DocId {
        let close = self.partner.get(open).copied().flatten().unwrap_or(open);
        if args.is_empty() || !self.huggable(*args.last().unwrap()) {
            let els = args.iter().map(|&x| El::Node(x)).collect();
            return self.seq(open, close, els);
        }
        if !self.pass2 && close < self.close_mark.len() {
            self.close_mark[close] = true;
        }
        let o = self.tok_doc(open, None, true, Some(true));
        let n = args.len();
        let mut xs = Vec::new();
        let mut ts = Vec::new();
        let mut blanks = Vec::new();
        for (k, &a) in args.iter().enumerate() {
            let outer = self.outer(a);
            blanks.push(k > 0 && self.blank_before(outer));
            let last = self.elem_begin(outer);
            xs.push(self.sub(a, false));
            ts.push(self.elem_end(last, outer, k > 0));
        }
        let lc = self.lead_doc(close);
        let c = self.tok_doc(close, None, false, None);
        let must = xs[..n - 1].iter().any(|x| self.docs.f(*x));
        let comma = self.text(",");
        let one_line = |b: &mut B, last: DocId| {
            let mut v = vec![o];
            for k in 0..n {
                v.push(if k + 1 == n { last } else { xs[k] });
                if k + 1 < n {
                    v.push(comma);
                    v.push(ts[k]);
                    v.push(b.sp());
                } else {
                    v.push(ts[k]);
                }
            }
            v.push(lc);
            v.push(c);
            b.list(v)
        };
        let a1 = one_line(self, xs[n - 1]);
        let hugged = self.hug(xs[n - 1]);
        let a2 = one_line(self, hugged);
        let mut inner = Vec::new();
        for k in 0..n {
            inner.push(if k == 0 { self.soft() } else { self.line() });
            if blanks[k] {
                let b = self.push(D::Blank);
                let z = self.empty();
                inner.push(self.ib(b, z));
            }
            inner.push(xs[k]);
            if k + 1 < n {
                inner.push(comma);
            } else {
                let z = self.empty();
                inner.push(self.ib(comma, z));
            }
            inner.push(ts[k]);
        }
        inner.push(lc);
        let il = self.list(inner);
        let ind = self.indent(il);
        let s = self.soft();
        let gl = self.list(vec![o, ind, s, c]);
        let a3 = self.group(gl);
        self.push(D::Choice(vec![a1, a2, a3], must))
    }

    /// The document with its outer group a `broken-group` (2.10 item 2).
    fn hug(&mut self, d: DocId) -> DocId {
        match self.docs.get(d) {
            D::Group(x, _) => {
                let x = *x;
                self.push(D::Group(x, true))
            }
            D::List(xs) if !xs.is_empty() => {
                let mut v = xs.clone();
                let last = v.pop().expect("an element");
                let h = self.hug(last);
                v.push(h);
                self.list(v)
            }
            _ => d,
        }
    }

    fn chain(&mut self, top: NodeId) -> DocId {
        let mut rev: Vec<Step> = Vec::new();
        let mut n = match self.kind(top) {
            NodeKind::OptChain { x } => *x,
            _ => top,
        };
        let mut via_call = true;
        loop {
            match self.kind(n) {
                NodeKind::Member { x, .. } => {
                    rev.push(Step::Mem(n));
                    n = *x;
                    via_call = false;
                }
                NodeKind::Index { x, .. } => {
                    rev.push(Step::Idx(n));
                    n = *x;
                    via_call = false;
                }
                NodeKind::MethodCall { x, .. } => {
                    rev.push(Step::Call(n));
                    n = *x;
                    via_call = true;
                }
                NodeKind::Each { x: Some(r), .. } if via_call => {
                    let r = *r;
                    let after = self.outer(r).end;
                    let mut s = self.each_steps(n, after);
                    s.reverse();
                    rev.extend(s);
                    n = r;
                    via_call = false;
                }
                _ => break,
            }
        }
        rev.reverse();
        let steps = rev;
        let pd = match self.kind(n) {
            NodeKind::Call { .. } => self.node(n, true),
            _ => {
                let p = self.paren_receiver(n);
                self.sub(n, p)
            }
        };
        let first_call = steps
            .iter()
            .position(|s| matches!(s, Step::Call(_)))
            .unwrap_or(steps.len());
        let mut recv = vec![pd];
        for s in &steps[..first_call] {
            recv.push(self.step(s));
        }
        let recv = self.list(recv);
        let mut links: Vec<(DocId, bool)> = Vec::new();
        let mut cur: Vec<DocId> = Vec::new();
        let mut cur_lead = false;
        for s in &steps[first_call..] {
            if let Step::Call(c) = s {
                if !cur.is_empty() {
                    let l = self.list(std::mem::take(&mut cur));
                    links.push((l, cur_lead));
                }
                cur_lead = self.call_has_lead(*c);
            }
            cur.push(self.step(s));
        }
        if !cur.is_empty() {
            let l = self.list(cur);
            links.push((l, cur_lead));
        }
        if links.is_empty() {
            return recv;
        }
        if links.len() == 1 && !links[0].1 {
            return self.list(vec![recv, links[0].0]);
        }
        let mut inner = Vec::new();
        for (l, _) in &links {
            inner.push(self.soft());
            inner.push(*l);
        }
        let il = self.list(inner);
        let ind = self.indent(il);
        let c2l = self.list(vec![recv, ind]);
        let c2 = self.push(D::Group(c2l, true));
        if links.len() == 1 {
            return c2;
        }
        let nl = links.len();
        let must =
            links[..nl - 1].iter().any(|(l, _)| self.docs.f(*l)) || links.iter().any(|(_, h)| *h);
        let mut c1v = vec![recv];
        c1v.extend(links.iter().map(|(l, _)| *l));
        let c1 = self.list(c1v);
        self.push(D::Choice(vec![c1, c2], must))
    }

    fn call_has_lead(&self, c: NodeId) -> bool {
        if !self.pass2 {
            return false;
        }
        let NodeKind::MethodCall { name, .. } = self.kind(c) else {
            return false;
        };
        let dot = self.tb(name.span.start);
        !self.lead[dot].is_empty()
    }

    /// The steps of a projection (F7), from its star.
    fn each_steps(&self, e: NodeId, after: u32) -> Vec<Step> {
        let NodeKind::Each { reads, .. } = self.kind(e) else {
            return Vec::new();
        };
        let lb = self.ti(after);
        let mut out = vec![Step::Star(lb)];
        let mut cur = self.t.get(lb + 2).map(|x| x.span.end).unwrap_or(after);
        let (plain, inner) = match reads.last() {
            Some(&r) if matches!(self.kind(r), NodeKind::Each { .. }) => {
                (reads.len().checked_sub(2).map(|i| reads[i]), Some(r))
            }
            Some(&r) => (Some(r), None),
            None => (None, None),
        };
        if let Some(r) = plain {
            let mut s = Vec::new();
            let mut n = r;
            loop {
                match self.kind(n) {
                    NodeKind::Member { x, .. } => {
                        s.push(Step::Mem(n));
                        n = *x;
                    }
                    NodeKind::Index { x, .. } => {
                        s.push(Step::Idx(n));
                        n = *x;
                    }
                    _ => break,
                }
            }
            s.reverse();
            out.extend(s);
            cur = self.ast.span(r).end;
        }
        if let Some(i) = inner {
            out.extend(self.each_steps(i, cur));
        }
        out
    }

    fn step(&mut self, s: &Step) -> DocId {
        match *s {
            Step::Mem(n) => {
                let NodeKind::Member { name_span, .. } = self.kind(n) else {
                    return self.empty();
                };
                let dot = self.tb(name_span.start);
                let a = self.tk(dot);
                let b = self.tk_at(name_span.start);
                self.list(vec![a, b])
            }
            Step::Idx(n) => {
                let NodeKind::Index { index, .. } = self.kind(n) else {
                    return self.empty();
                };
                let lb = self.tb(self.outer(*index).start);
                let a = self.tk(lb);
                let b = self.sub(*index, false);
                let rb = self.ti(self.outer(*index).end);
                let c = self.tk(rb);
                self.list(vec![a, b, c])
            }
            Step::Star(lb) => {
                let a = self.tk(lb);
                let b = self.tk(lb + 1);
                let c = self.tk(lb + 2);
                self.list(vec![a, b, c])
            }
            Step::Call(n) => {
                let NodeKind::MethodCall { name, args, .. } = self.kind(n) else {
                    return self.empty();
                };
                let dot = self.tb(name.span.start);
                let a = self.tk(dot);
                let b = self.tk_at(name.span.start);
                let open = self.ti(name.span.end);
                let c = self.args(open, args);
                self.list(vec![a, b, c])
            }
        }
    }

    // ---- program types (2.13) ----

    fn ptype(&mut self, span: Span) -> DocId {
        let mut v = Vec::new();
        let mut k = self.ti(span.start);
        let mut prev: Option<usize> = None;
        while k < self.t.len() && self.t[k].span.end <= span.end {
            let tok = self.t[k].tok.clone();
            match tok {
                Tok::Punct(P::Comma) => {
                    let next_close = matches!(
                        self.t.get(k + 1).map(|x| &x.tok),
                        Some(Tok::Punct(P::RBrace))
                    );
                    if !next_close {
                        v.push(self.text(","));
                    }
                    if let Some(p) = prev {
                        if self.pass2 {
                            self.defer[p] = false;
                            v.push(self.trail_doc(p, None));
                        }
                    }
                    if !next_close {
                        v.push(self.sp());
                    }
                    prev = None;
                }
                Tok::Punct(P::Pipe) => {
                    v.push(self.sp());
                    v.push(self.tk(k));
                    v.push(self.sp());
                    prev = Some(k);
                }
                Tok::Punct(P::Colon) => {
                    v.push(self.tk(k));
                    v.push(self.sp());
                    prev = Some(k);
                }
                _ => {
                    let next_comma = matches!(
                        self.t.get(k + 1).map(|x| &x.tok),
                        Some(Tok::Punct(P::Comma))
                    );
                    if next_comma && self.pass2 {
                        self.defer[k] = true;
                    }
                    v.push(self.tk(k));
                    prev = Some(k);
                }
            }
            k += 1;
        }
        self.list(v)
    }

    // ---- text blocks (2.7 item 2) ----

    fn text_block(&self, span: Span, exprs: &[NodeId]) -> TextBlock {
        let ch = self.ch;
        let (a, b) = (span.start as usize, span.end as usize);
        let raw_form = ch.get(a) == Some(&'r');
        let delim = if raw_form { 4 } else { 3 };
        let open: String = ch[a..a + delim].iter().collect();
        // The interpolations, each from after its `${` to its `}`.
        let mut regions: Vec<(usize, usize)> = Vec::new();
        for &e in exprs {
            let o = self.outer(e);
            let p = self.raw.partition_point(|x| x.span.end <= o.start);
            let q = self.raw.partition_point(|x| x.span.start < o.end);
            let s = p
                .checked_sub(1)
                .map(|i| self.raw[i].span.end as usize)
                .unwrap_or(a);
            let t = self
                .raw
                .get(q)
                .map(|x| x.span.start as usize + 1)
                .unwrap_or(b);
            regions.push((s, t));
        }
        let in_interp = |o: usize| regions.iter().any(|&(s, t)| s <= o && o < t);
        let mut j = a + delim;
        while j < b && (ch[j] == ' ' || ch[j] == '\t') {
            j += 1;
        }
        if j < b && ch[j] == '\r' {
            j += 1;
        }
        if j < b && ch[j] == '\n' {
            j += 1;
        }
        let body_end = b.saturating_sub(3);
        // Source lines of the content.
        let mut lines: Vec<(usize, usize)> = Vec::new();
        let mut s = j;
        let mut i = j;
        while i < body_end {
            match ch[i] {
                '\n' => {
                    lines.push((s, i));
                    s = i + 1;
                }
                '\r' => {
                    lines.push((s, i));
                    if i + 1 < body_end && ch[i + 1] == '\n' {
                        i += 1;
                    }
                    s = i + 1;
                }
                _ => {}
            }
            i += 1;
        }
        let closing = (s, body_end);
        let ind = closing.1 - closing.0;
        let mut out = Vec::new();
        for &(ls, le) in &lines {
            let text: String = ch[ls..le].iter().collect();
            if in_interp(ls) {
                out.push(BLine::Raw(text));
            } else if text.chars().all(|c| c == ' ' || c == '\t') {
                out.push(BLine::Blank);
            } else {
                let k = (ls + ind).min(le);
                out.push(BLine::Content(ch[k..le].iter().collect()));
            }
        }
        TextBlock { open, lines: out }
    }
}

#[derive(Clone, Copy)]
enum Step {
    Mem(NodeId),
    Idx(NodeId),
    /// The token of its `[` or `?.[`, and whether it is optional.
    Star(usize),
    Call(NodeId),
}

fn op_level(op: BinaryOp) -> u8 {
    match op {
        BinaryOp::Coalesce => 2,
        BinaryOp::Or => 3,
        BinaryOp::And => 4,
        BinaryOp::Eq | BinaryOp::Ne => 5,
        BinaryOp::Lt
        | BinaryOp::Le
        | BinaryOp::Gt
        | BinaryOp::Ge
        | BinaryOp::In
        | BinaryOp::NotIn => 6,
        BinaryOp::Add | BinaryOp::Sub => 7,
        BinaryOp::Mul | BinaryOp::Div | BinaryOp::Rem => 8,
        BinaryOp::Pow => 10,
    }
}

/// Build the document twice (the module's comment says why) and lay it out.
fn run(
    ch: &[char],
    ast: &Ast,
    root: NodeId,
    raw: &[Token],
    comments: &[Span],
    width: Option<i64>,
) -> String {
    let mut b = B::new(ch, ast, raw, comments);
    b.node(root, true);
    b.attach();
    let d = b.node(root, true);
    let mut s = layout(&b.docs, d, width);
    while s.ends_with([' ', '\n']) {
        s.pop();
    }
    s
}

/// The formatted text of a program (2.1), or its `parse` diagnostics.
pub fn format(text: &str) -> Result<String, Error> {
    let parsed = crate::syntax::parse(text)?;
    let src = &parsed.source;
    let (toks, comments) = lex_comments(src).map_err(Error::one)?;
    let mut s = run(
        src.chars(),
        &parsed.ast,
        parsed.ast.root,
        &toks,
        &comments,
        Some(WIDTH),
    );
    s.push('\n');
    Ok(s)
}

/// The formatted text of a template (2.14), or its `parse` diagnostics.
pub fn format_template(text: &str) -> Result<String, Error> {
    let t = parse_template(text)?;
    let src: &Source = t.source();
    let ch = src.chars();
    let mut out = String::new();
    let mut pi = 0;
    for seg in t.segments() {
        match seg {
            Segment::Text { span, .. } => out.push_str(&src.slice(*span)),
            Segment::Part { span, .. } => {
                let as_is = src.slice(*span);
                let i = pi;
                pi += 1;
                let (toks, cms) =
                    lex_part_comments(src, span.start as usize + 3, span.start as usize)
                        .map_err(Error::one)?;
                let line_comment = cms
                    .iter()
                    .any(|c| ch.get(c.start as usize + 1) == Some(&'/'));
                let Some(p) = t.part_program(i) else {
                    out.push_str(&as_is);
                    continue;
                };
                if line_comment {
                    out.push_str(&as_is);
                    continue;
                }
                let s = run(ch, &p.ast, p.ast.root, &toks, &cms, None);
                if s.contains('\n') {
                    out.push_str(&as_is);
                } else {
                    out.push_str("${{ ");
                    out.push_str(&s);
                    out.push_str(" }}");
                }
            }
        }
    }
    Ok(out)
}

// ---- equal trees (2.2 item 1) --------------------------------------------------

/// A text that two trees share exactly when they are equal, spans aside.
pub fn canon(ast: &Ast, root: NodeId, ch: &[char]) -> String {
    use std::fmt::Write;
    let mut out = String::new();
    let mut work: Vec<Option<NodeId>> = vec![Some(root)];
    while let Some(w) = work.pop() {
        let Some(id) = w else {
            out.push(')');
            continue;
        };
        let n = ast.node(id);
        let _ = write!(out, "({}", n.kind.name());
        let mut kids: Vec<NodeId> = Vec::new();
        match &n.kind {
            NodeKind::Program { consts, result } | NodeKind::Block { consts, result } => {
                kids.extend(consts);
                kids.push(*result);
            }
            NodeKind::Const { pattern, value } => kids.extend([*pattern, *value]),
            NodeKind::Ident { name } => {
                let _ = write!(out, " {name:?}");
            }
            NodeKind::Number { value } => {
                let _ = write!(out, " {}", value.to_bits());
            }
            NodeKind::String { value, form } => {
                let _ = write!(out, " {form:?} {value:?}");
            }
            NodeKind::Template { parts } => {
                let s = n.span.start as usize;
                let form = if ch.get(s) == Some(&'`') {
                    "`"
                } else if ch.get(s) == Some(&'r') {
                    "r3"
                } else {
                    "3"
                };
                let _ = write!(out, " {form}");
                for p in parts {
                    match p {
                        crate::syntax::ast::TemplatePart::Text(t) => {
                            let _ = write!(out, " {t:?}");
                        }
                        crate::syntax::ast::TemplatePart::Expr(e) => {
                            out.push_str(" $");
                            kids.push(*e);
                        }
                    }
                }
            }
            NodeKind::Bool { value } => {
                let _ = write!(out, " {value}");
            }
            NodeKind::Null | NodeKind::Bad | NodeKind::Elem => {}
            NodeKind::List { items } => kids.extend(items),
            NodeKind::Map { entries } => kids.extend(entries),
            NodeKind::Entry {
                key,
                value,
                shorthand,
                ..
            } => {
                match key {
                    EntryKey::Word(k) => {
                        let _ = write!(out, " w{k:?}");
                    }
                    EntryKey::Str(k) => {
                        let _ = write!(out, " s{k:?}");
                    }
                    EntryKey::Computed(k) => {
                        out.push_str(" c");
                        kids.push(*k);
                    }
                }
                let _ = write!(out, " {shorthand}");
                kids.push(*value);
            }
            NodeKind::Spread { value } => kids.push(*value),
            NodeKind::Unary { op, x } => {
                let _ = write!(out, " {op:?}");
                kids.push(*x);
            }
            NodeKind::Binary { op, x, y } => {
                let _ = write!(out, " {op:?}");
                kids.extend([*x, *y]);
            }
            NodeKind::Cond {
                form,
                test,
                then,
                else_,
            } => {
                let _ = write!(out, " {form:?}");
                kids.extend([*test, *then, *else_]);
            }
            NodeKind::Member {
                x, name, optional, ..
            } => {
                let _ = write!(out, " {name:?} {optional}");
                kids.push(*x);
            }
            NodeKind::Index { x, index, optional } => {
                let _ = write!(out, " {optional}");
                kids.extend([*x, *index]);
            }
            NodeKind::Call {
                namespace,
                name,
                args,
            } => {
                let _ = write!(
                    out,
                    " {:?} {:?}",
                    namespace.as_ref().map(|n| &n.name),
                    name.name
                );
                kids.extend(args);
            }
            NodeKind::MethodCall {
                x,
                name,
                args,
                optional,
            } => {
                let _ = write!(out, " {:?} {optional}", name.name);
                kids.push(*x);
                kids.extend(args);
            }
            NodeKind::OptChain { x } => kids.push(*x),
            NodeKind::Lambda { params, body } => {
                kids.extend(params);
                kids.push(*body);
            }
            NodeKind::Param { pattern, default } => {
                kids.push(*pattern);
                kids.extend(default);
            }
            NodeKind::Match { subject, arms } => {
                kids.push(*subject);
                kids.extend(arms);
            }
            NodeKind::Arm { literals, result } => {
                match literals {
                    None => out.push_str(" _"),
                    Some(ls) => {
                        for l in ls {
                            let _ = write!(out, " {:?}", l.lit);
                        }
                    }
                }
                kids.push(*result);
            }
            NodeKind::Try {
                body,
                name,
                handler,
            } => {
                let _ = write!(out, " {:?}", name.as_ref().map(|n| &n.name));
                kids.extend([*body, *handler]);
            }
            NodeKind::NamePat { name } => {
                let _ = write!(out, " {name:?}");
            }
            NodeKind::MapPat { fields, rest } => {
                for f in fields {
                    let _ = write!(out, " {:?}:{:?}", f.key, f.name.name);
                    kids.extend(f.default);
                }
                let _ = write!(out, " {:?}", rest.as_ref().map(|r| &r.name));
            }
            NodeKind::ListPat { elems, rest } => {
                for e in elems {
                    kids.push(e.pat);
                    kids.extend(e.default);
                }
                let _ = write!(out, " {:?}", rest.as_ref().map(|r| &r.name));
            }
            NodeKind::As { form, x, ty } => {
                let _ = write!(out, " {form:?} {}", ptype_canon(ty));
                kids.push(*x);
            }
            NodeKind::Each {
                x, reads, optional, ..
            } => {
                let _ = write!(out, " {optional} {}", x.is_some());
                kids.extend(x);
                kids.extend(reads);
            }
        }
        work.push(None);
        for k in kids.into_iter().rev() {
            work.push(Some(k));
        }
    }
    out
}

fn ptype_canon(t: &crate::syntax::ast::PType) -> String {
    use crate::syntax::ast::PPrim;
    let mut s = String::from("(");
    for a in &t.alts {
        let p = match &a.prim {
            PPrim::Map(x) => format!("map<{}>", ptype_canon(x)),
            PPrim::Paren(x) => format!("({})", ptype_canon(x)),
            PPrim::Name(n, _) => n.clone(),
            PPrim::Record { fields, rest } => {
                let mut r = String::from("{");
                for f in fields {
                    r.push_str(&format!(
                        "{:?}{}:{},",
                        f.key,
                        f.optional,
                        ptype_canon(&f.ty)
                    ));
                }
                match rest {
                    None => {}
                    Some(None) => r.push_str("..."),
                    Some(Some(x)) => r.push_str(&format!("...{}", ptype_canon(x))),
                }
                r + "}"
            }
            other => format!("{other:?}"),
        };
        s.push_str(&format!("{p}{}|", a.suffixes));
    }
    s + ")"
}

/// The values of a template's text runs and the trees of its parts.
pub fn template_canon(t: &crate::template::Template) -> String {
    let mut out = String::new();
    let mut i = 0;
    for seg in t.segments() {
        match seg {
            Segment::Text { value, .. } => out.push_str(&format!("{value:?};")),
            Segment::Part { .. } => {
                if let Some(p) = t.part_program(i) {
                    out.push_str(&canon(&p.ast, p.ast.root, p.source.chars()));
                }
                out.push(';');
                i += 1;
            }
        }
    }
    out
}
