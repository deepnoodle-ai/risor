//! Package F12: the assist stage, completion and hover (`f12-tools.md` 3).
//!
//! The text before the cut is lexed, the hole put in place of the prefix,
//! and the tokens repaired into a program by inserting tokens at the end
//! (3.3). The repaired program is checked for the types of its nodes and of
//! the names in scope at the hole; its diagnostics are not kept.

use crate::check::ty::{from_text, join, print, Strs, Ty};
use crate::check::{check_probe, Probe, Want};
use crate::compile::Res;
use crate::describe::print_sig;
use crate::env::Env;
use crate::library;
use crate::source::{Range, Source, Span};
use crate::syntax::ast::{Ast, EntryKey, Lit, NodeId, NodeKind, PPrim, PType};
use crate::syntax::lexer::{lex_cut, CUT_OPEN};
use crate::syntax::parser::{parse_repair, HOLE};
use crate::syntax::token::{StrForm, Tok, Token, P};
use crate::syntax::Expr;
use crate::value::Kind;

/// A candidate of a completion (3.12).
#[derive(Clone, Debug, PartialEq)]
pub struct Candidate {
    pub label: String,
    pub kind: &'static str,
    pub detail: Option<String>,
    pub insert: String,
}

/// A completion: the prefix span and the candidates (3.5 item 3).
#[derive(Clone, Debug, PartialEq)]
pub struct Completion {
    pub span: Span,
    pub items: Vec<Candidate>,
}

/// A hover record (3.13).
#[derive(Clone, Debug, PartialEq)]
pub struct Hover {
    pub kind: &'static str,
    pub span: Span,
    pub ty: Option<String>,
    pub signature: Option<String>,
    pub doc: Option<String>,
    pub declaration: Option<Span>,
}

impl RawCompletion {
    pub(crate) fn public(self, src: &Source) -> Completion {
        Completion {
            span: src.line_span(self.span),
            items: self.items,
        }
    }
}

impl RawHover {
    pub(crate) fn public(self, src: &Source) -> Hover {
        Hover {
            kind: self.kind,
            span: src.line_span(self.span),
            ty: self.ty,
            signature: self.signature,
            doc: self.doc,
            declaration: self.declaration.map(|d| src.line_span(d)),
        }
    }
}

/// A completion in code-point offsets.
#[derive(Clone, Debug, PartialEq)]
pub(crate) struct RawCompletion {
    pub span: Range,
    pub items: Vec<Candidate>,
}

/// A hover record in code-point offsets.
#[derive(Clone, Debug, PartialEq)]
pub(crate) struct RawHover {
    pub kind: &'static str,
    pub span: Range,
    pub ty: Option<String>,
    pub signature: Option<String>,
    pub doc: Option<String>,
    pub declaration: Option<Range>,
}

fn is_word_char(c: char) -> bool {
    c.is_ascii_alphanumeric() || c == '_'
}

/// A word of `01-language.md` 2.4 rule 1.
fn is_word(s: &str) -> bool {
    let mut cs = s.chars();
    cs.next()
        .is_some_and(|c| c.is_ascii_alphabetic() || c == '_')
        && cs.all(is_word_char)
}

fn mk(tok: Tok, at: usize) -> Token {
    let span = Range::new(at, at);
    Token {
        tok,
        span,
        diag: span,
        code: None,
    }
}

/// The text before a cut, lexed (3.2 item 4).
enum Lexed {
    /// The tokens, with no `Eof`, and the tokens that close the
    /// interpolations the cut left open.
    Ok(Vec<Token>, Vec<Token>),
    /// The cut is inside a one-line string that starts at the offset, with
    /// its quote; the tokens are those before it.
    InString(usize, char, Vec<Token>, Vec<Token>),
    Fail,
}

fn lex_before(ch: &[char], cut: usize, strings: bool) -> Lexed {
    let text: String = ch[..cut].iter().collect();
    let src = Source::raw(&text);
    let r = lex_cut(&src);
    let closers = |open: &[bool]| -> Vec<Token> {
        open.iter()
            .map(|_| mk(Tok::TmplTail(String::new()), cut))
            .collect()
    };
    match r.error {
        None => {
            // A line comment that runs to the cut holds it (3.2 item 5).
            if r.comments
                .iter()
                .any(|c| c.end as usize == cut && ch.get(c.start as usize + 1) == Some(&'/'))
            {
                return Lexed::Fail;
            }
            let mut toks = r.tokens;
            toks.retain(|t| !matches!(t.tok, Tok::Eof));
            Lexed::Ok(toks, Vec::new())
        }
        Some(d) if d.code == CUT_OPEN => Lexed::Ok(r.tokens, closers(&r.open)),
        Some(d) if d.code == crate::diag::code::UNTERMINATED_STRING && strings => {
            let s = d.range.map(|s| s.start as usize).unwrap_or(0);
            let q = ch.get(s).copied().unwrap_or(' ');
            let triple = ch.get(s + 1) == Some(&q) && ch.get(s + 2) == Some(&q);
            if !(q == '"' || q == '\'') || triple || d.range.is_none_or(|r| r.end as usize != cut) {
                return Lexed::Fail;
            }
            match lex_before(ch, s, false) {
                Lexed::Ok(t, p) => Lexed::InString(s, q, t, p),
                _ => Lexed::Fail,
            }
        }
        Some(_) => Lexed::Fail,
    }
}

/// Whether the cut stands inside a comment or a literal.
fn inside_literal(ch: &[char], cut: usize) -> bool {
    let text: String = ch[..cut].iter().collect();
    let r = lex_cut(&Source::raw(&text));
    match r.error {
        None => r
            .comments
            .iter()
            .any(|c| c.end as usize == cut && ch.get(c.start as usize + 1) == Some(&'/')),
        Some(d) => {
            matches!(
                d.code,
                crate::diag::code::UNTERMINATED_STRING | crate::diag::code::UNTERMINATED_COMMENT
            ) && d.range.is_some_and(|r| r.end as usize == cut)
        }
    }
}

/// The tokens the repair may insert, in order (3.3 item 1).
fn insert_list() -> Vec<Tok> {
    vec![
        Tok::Word("null".into()),
        Tok::Punct(P::RParen),
        Tok::Punct(P::RBracket),
        Tok::Punct(P::RBrace),
        Tok::Punct(P::Semi),
        Tok::Punct(P::Colon),
        Tok::Punct(P::Arrow),
        Tok::Punct(P::Assign),
        Tok::Punct(P::LBrace),
        Tok::Word("else".into()),
        Tok::Word("catch".into()),
        Tok::Punct(P::Gt),
    ]
}

/// The closer of the innermost bracket still open in the current
/// interpolation.
fn innermost(toks: &[Token]) -> Option<Tok> {
    let mut st: Vec<Option<P>> = Vec::new();
    for t in toks {
        match &t.tok {
            Tok::Punct(P::LParen) => st.push(Some(P::RParen)),
            Tok::Punct(P::LBracket | P::QDotBracket) => st.push(Some(P::RBracket)),
            Tok::Punct(P::LBrace) => st.push(Some(P::RBrace)),
            Tok::Punct(P::RParen | P::RBracket | P::RBrace) => {
                if matches!(st.last(), Some(Some(_))) {
                    st.pop();
                }
            }
            Tok::TmplHead(_) => st.push(None),
            Tok::TmplMiddle(_) => {
                while let Some(Some(_)) = st.last() {
                    st.pop();
                }
            }
            Tok::TmplTail(_) => {
                while let Some(Some(_)) = st.last() {
                    st.pop();
                }
                st.pop();
            }
            _ => {}
        }
    }
    st.last().copied().flatten().map(Tok::Punct)
}

/// The repair (3.3): tokens inserted at the end until the program is
/// complete.
fn repair(src: &Source, pre: &[Token], post: &[Token], at: usize) -> Option<Ast> {
    let limit = pre.len();
    let mut ins: Vec<Token> = Vec::new();
    let build = |ins: &[Token]| -> Vec<Token> {
        let mut v: Vec<Token> = pre.to_vec();
        v.extend(ins.iter().cloned());
        v.extend(post.iter().cloned());
        v.push(mk(Tok::Eof, at));
        v
    };
    for _ in 0..(4 * limit + 64) {
        match parse_repair(src, &build(&ins), limit) {
            Ok(a) => return Some(a),
            Err(pos) if pos < limit => return None,
            Err(_) => {}
        }
        let k = limit + ins.len();
        let mut all: Vec<Token> = pre.to_vec();
        all.extend(ins.iter().cloned());
        let mut cands: Vec<Tok> = innermost(&all).into_iter().collect();
        cands.extend(insert_list());
        let mut accepted = false;
        for c in cands {
            ins.push(mk(c, at));
            match parse_repair(src, &build(&ins), limit) {
                Ok(a) => return Some(a),
                Err(pos) if pos > k => {
                    accepted = true;
                    break;
                }
                Err(_) => {
                    ins.pop();
                }
            }
        }
        if !accepted {
            return None;
        }
    }
    None
}

// ---- the tree ------------------------------------------------------------------

fn children(k: &NodeKind) -> Vec<NodeId> {
    let mut v = Vec::new();
    match k {
        NodeKind::Program { consts, result } | NodeKind::Block { consts, result } => {
            v.extend(consts);
            v.push(*result);
        }
        NodeKind::Const { pattern, value } => v.extend([*pattern, *value]),
        NodeKind::Template { parts } => {
            for p in parts {
                if let crate::syntax::ast::TemplatePart::Expr(e) = p {
                    v.push(*e);
                }
            }
        }
        NodeKind::List { items } => v.extend(items),
        NodeKind::Map { entries } => v.extend(entries),
        NodeKind::Entry { key, value, .. } => {
            if let EntryKey::Computed(k) = key {
                v.push(*k);
            }
            v.push(*value);
        }
        NodeKind::Spread { value } => v.push(*value),
        NodeKind::Unary { x, .. } => v.push(*x),
        NodeKind::Binary { x, y, .. } => v.extend([*x, *y]),
        NodeKind::Cond {
            test, then, else_, ..
        } => v.extend([*test, *then, *else_]),
        NodeKind::Member { x, .. } | NodeKind::OptChain { x } | NodeKind::As { x, .. } => {
            v.push(*x)
        }
        NodeKind::Index { x, index, .. } => v.extend([*x, *index]),
        NodeKind::Call { args, .. } => v.extend(args),
        NodeKind::MethodCall { x, args, .. } => {
            v.push(*x);
            v.extend(args);
        }
        NodeKind::Lambda { params, body } => {
            v.extend(params);
            v.push(*body);
        }
        NodeKind::Param { pattern, default } => {
            v.push(*pattern);
            v.extend(default);
        }
        NodeKind::Match { subject, arms } => {
            v.push(*subject);
            v.extend(arms);
        }
        NodeKind::Arm { result, .. } => v.push(*result),
        NodeKind::Try { body, handler, .. } => v.extend([*body, *handler]),
        NodeKind::MapPat { fields, .. } => v.extend(fields.iter().filter_map(|f| f.default)),
        NodeKind::ListPat { elems, .. } => {
            for e in elems {
                v.push(e.pat);
                v.extend(e.default);
            }
        }
        NodeKind::Each { x, reads, .. } => {
            v.extend(x);
            v.extend(reads);
        }
        _ => {}
    }
    v
}

struct Tree<'a> {
    env: &'a Env,
    ast: &'a Ast,
    parent: Vec<Option<NodeId>>,
    probe: Probe,
    expected: &'a Want,
}

impl<'a> Tree<'a> {
    fn new(env: &'a Env, parsed: &'a Expr, expected: &'a Want, probe: Option<NodeId>) -> Tree<'a> {
        let ast = &parsed.ast;
        let mut parent = vec![None; ast.nodes.len()];
        for (i, n) in ast.nodes.iter().enumerate() {
            for c in children(&n.kind) {
                if let Some(p) = parent.get_mut(c.index()) {
                    *p = Some(NodeId(i as u32));
                }
            }
        }
        let probe = check_probe(env, parsed, expected, probe);
        Tree {
            env,
            ast,
            parent,
            probe,
            expected,
        }
    }

    fn kind(&self, id: NodeId) -> &'a NodeKind {
        &self.ast.nodes[id.index()].kind
    }

    fn res(&self, id: NodeId) -> &Res {
        &self.probe.res[id.index()]
    }

    fn ty(&self, id: NodeId) -> Ty {
        self.probe
            .types
            .get(id.index())
            .cloned()
            .unwrap_or_default()
    }

    fn slot_ty(&self, s: u32, at: bool) -> Ty {
        let slots = match (&self.probe.at, at) {
            (Some(a), true) => a,
            _ => &self.probe.slots,
        };
        slots.get(s as usize).cloned().unwrap_or_default()
    }

    /// The path from the root to a node.
    fn path(&self, id: NodeId) -> Vec<NodeId> {
        let mut v = vec![id];
        let mut n = id;
        while let Some(p) = self.parent[n.index()] {
            v.push(p);
            n = p;
        }
        v.reverse();
        v
    }

    /// The names a pattern binds, in order, with their slots and spans.
    fn pattern_names(&self, pat: NodeId) -> Vec<(String, Option<u32>, Range)> {
        let mut out = Vec::new();
        match self.kind(pat) {
            NodeKind::NamePat { name: Some(n) } if n != "_" => {
                let s = match self.res(pat) {
                    Res::Bind(s) => Some(*s),
                    _ => None,
                };
                out.push((n.clone(), s, self.ast.span(pat)));
            }
            NodeKind::MapPat { fields, rest } => {
                let (fs, r) = match self.res(pat) {
                    Res::PatSlots { fields, rest } => (fields.clone(), *rest),
                    _ => (Vec::new(), None),
                };
                for (i, f) in fields.iter().enumerate() {
                    if f.name.name != "_" {
                        out.push((
                            f.name.name.clone(),
                            fs.get(i).copied().flatten(),
                            f.name.span,
                        ));
                    }
                }
                if let Some(rn) = rest {
                    out.push((rn.name.clone(), r, rn.span));
                }
            }
            NodeKind::ListPat { elems, rest } => {
                for e in elems {
                    out.extend(self.pattern_names(e.pat));
                }
                if let Some(rn) = rest {
                    let r = match self.res(pat) {
                        Res::PatSlots { rest, .. } => *rest,
                        _ => None,
                    };
                    out.push((rn.name.clone(), r, rn.span));
                }
            }
            _ => {}
        }
        out
    }

    /// 3.6 items 1 to 3: the names in scope at a node, innermost first.
    fn scope_names(&self, id: NodeId) -> Vec<Candidate> {
        let path = self.path(id);
        let mut scopes: Vec<Vec<Candidate>> = Vec::new();
        for w in path.windows(2) {
            let (a, c) = (w[0], w[1]);
            let mut names: Vec<(String, Option<u32>, bool)> = Vec::new();
            match self.kind(a) {
                NodeKind::Program { consts, .. } | NodeKind::Block { consts, .. } => {
                    for &k in consts {
                        if k == c {
                            break;
                        }
                        if let NodeKind::Const { pattern, value } = self.kind(k) {
                            let lambda = matches!(self.kind(*value), NodeKind::Lambda { .. })
                                && matches!(self.kind(*pattern), NodeKind::NamePat { .. });
                            for (n, s, _) in self.pattern_names(*pattern) {
                                names.push((n, s, lambda));
                            }
                        }
                    }
                }
                NodeKind::Lambda { params, body } => {
                    for &p in params {
                        if p == c {
                            break;
                        }
                        if let NodeKind::Param { pattern, .. } = self.kind(p) {
                            for (n, s, _) in self.pattern_names(*pattern) {
                                names.push((n, s, false));
                            }
                        }
                    }
                    let _ = body;
                }
                NodeKind::MapPat { fields, .. } => {
                    let all = self.pattern_names(a);
                    for (i, f) in fields.iter().enumerate() {
                        if f.default == Some(c) {
                            for (n, s, _) in all.iter().take(i) {
                                names.push((n.clone(), *s, false));
                            }
                            break;
                        }
                    }
                }
                NodeKind::ListPat { elems, .. } => {
                    for (i, e) in elems.iter().enumerate() {
                        if e.default == Some(c) {
                            for e2 in &elems[..i] {
                                for (n, s, _) in self.pattern_names(e2.pat) {
                                    names.push((n, s, false));
                                }
                            }
                            break;
                        }
                    }
                }
                NodeKind::Try {
                    name: Some(n),
                    handler,
                    ..
                } if *handler == c => {
                    let s = match self.res(a) {
                        Res::Catch(s) => *s,
                        _ => None,
                    };
                    names.push((n.name.clone(), s, false));
                }
                _ => {}
            }
            let cands = names
                .into_iter()
                .map(|(n, s, lambda)| Candidate {
                    insert: n.clone(),
                    label: n,
                    kind: if lambda { "lambda" } else { "name" },
                    detail: if lambda {
                        None
                    } else {
                        s.map(|s| print(&self.slot_ty(s, true)))
                    },
                })
                .collect();
            scopes.push(cands);
        }
        let mut out: Vec<Candidate> = Vec::new();
        for scope in scopes.into_iter().rev() {
            for c in scope {
                if !out.iter().any(|o| o.label == c.label) {
                    out.push(c);
                }
            }
        }
        out
    }

    /// The expected type of a node (3.9).
    fn expected_of(&self, id: NodeId) -> Option<Ty> {
        let path = self.path(id);
        let mut e: Option<Ty> = match self.expected {
            Want::Type(t) => Some(t.clone()),
            _ => None,
        };
        for w in path.windows(2) {
            let (a, c) = (w[0], w[1]);
            e = match self.kind(a) {
                NodeKind::Program { result, .. } => (*result == c).then(|| e.clone()).flatten(),
                NodeKind::Block { result, .. } => (*result == c).then(|| e.clone()).flatten(),
                NodeKind::Cond { test, .. } => (*test != c).then(|| e.clone()).flatten(),
                NodeKind::Match { subject, .. } => (*subject != c).then(|| e.clone()).flatten(),
                NodeKind::Arm { result, .. } => (*result == c).then(|| e.clone()).flatten(),
                NodeKind::Try { .. } => e.clone(),
                NodeKind::List { .. } => {
                    if matches!(self.kind(c), NodeKind::Spread { .. }) {
                        None
                    } else {
                        e.as_ref()
                            .and_then(|t| t.list.as_ref().map(|x| x.as_ref().clone()))
                    }
                }
                NodeKind::Map { .. } => match self.kind(c) {
                    NodeKind::Entry { key, .. } => {
                        let k = match key {
                            EntryKey::Word(k) | EntryKey::Str(k) => Some(k.clone()),
                            EntryKey::Computed(_) => None,
                        };
                        match (k, &e) {
                            (Some(k), Some(t)) => {
                                let mut acc: Option<Ty> = None;
                                for r in t.variants() {
                                    let ft = match r.fields.get(&k) {
                                        Some(f) => Some(f.ty.clone()),
                                        None if !r.rest.is_never() => Some(r.rest.clone()),
                                        None => None,
                                    };
                                    if let Some(ft) = ft {
                                        acc = Some(match acc {
                                            Some(x) => join(&x, &ft),
                                            None => ft,
                                        });
                                    }
                                }
                                acc
                            }
                            _ => None,
                        }
                    }
                    _ => None,
                },
                NodeKind::Entry { value, .. } => (*value == c).then(|| e.clone()).flatten(),
                NodeKind::Call { args, .. } | NodeKind::MethodCall { args, .. } => {
                    match self.res(a) {
                        Res::CallHost(i) => {
                            let f = &self.env.data().functions[*i as usize];
                            args.iter().position(|x| *x == c).and_then(|k| {
                                let p = f.sig.params.get(k)?;
                                match &p.ty {
                                    crate::types::text::ParamType::Type(t) => {
                                        from_text(t, &self.env.data().names, None).ok()
                                    }
                                    _ => None,
                                }
                            })
                        }
                        _ => None,
                    }
                }
                _ => None,
            };
        }
        e
    }
}

// ---- completion ----------------------------------------------------------------

fn env_names(env: &Env) -> Vec<Candidate> {
    let d = env.data();
    let mut out: Vec<Candidate> = Vec::new();
    let mut vars: Vec<(String, String)> = d
        .vars
        .iter()
        .zip(&d.var_types)
        .map(|(n, t)| (n.clone(), print(t)))
        .collect();
    vars.sort();
    for (n, t) in vars {
        out.push(cand(&n, "variable", Some(t)));
    }
    let mut fns: Vec<(String, String)> = Vec::new();
    for (i, r) in library::records().iter().enumerate() {
        if r.receiver.is_none() && r.group.is_none() && env.lib_function(&r.name) == Some(i) {
            fns.push((r.name.clone(), r.signature.clone()));
        }
    }
    for f in &d.functions {
        if f.namespace.is_none() {
            fns.push((f.name.clone(), print_sig(&f.sig, &d.names)));
        }
    }
    fns.sort();
    for (n, s) in fns {
        if !out.iter().any(|o| o.label == n) {
            out.push(cand(&n, "function", Some(s)));
        }
    }
    let mut ns: Vec<String> = d.groups.clone();
    for f in &d.functions {
        if let Some(n) = &f.namespace {
            ns.push(n.clone());
        }
    }
    ns.sort();
    ns.dedup();
    for n in ns {
        if !out.iter().any(|o| o.label == n) {
            out.push(cand(&n, "namespace", None));
        }
    }
    out
}

fn cand(label: &str, kind: &'static str, detail: Option<String>) -> Candidate {
    Candidate {
        label: label.to_string(),
        kind,
        detail,
        insert: label.to_string(),
    }
}

fn members(env: &Env, r: &Ty) -> Vec<Candidate> {
    let r = r.nonnull();
    if r.any || r.is_never() {
        return Vec::new();
    }
    let mut fields: std::collections::BTreeMap<String, Ty> = Default::default();
    for rec in r.variants() {
        for (k, f) in &rec.fields {
            if is_word(k) {
                let t = match fields.get(k) {
                    Some(x) => join(x, &f.ty),
                    None => f.ty.clone(),
                };
                fields.insert(k.clone(), t);
            }
        }
    }
    let mut out: Vec<Candidate> = fields
        .iter()
        .map(|(k, t)| cand(k, "field", Some(print(t))))
        .collect();
    let mut methods: std::collections::BTreeMap<String, String> = Default::default();
    let kinds = [
        (r.str.is_some(), Kind::String),
        (r.list.is_some(), Kind::List),
        (r.map.is_some(), Kind::Map),
    ];
    for (has, kind) in kinds {
        if !has {
            continue;
        }
        for (i, rec) in library::records().iter().enumerate() {
            if rec.receiver == Some(kind) && env.lib_method(kind, &rec.name) == Some(i) {
                methods
                    .entry(rec.name.clone())
                    .or_insert_with(|| rec.signature.clone());
            }
        }
    }
    let d = env.data();
    for o in &r.opaque {
        for m in d.methods.iter().filter(|m| m.type_name == *o) {
            methods
                .entry(m.name.clone())
                .or_insert_with(|| print_sig(&m.sig, &d.names));
        }
    }
    out.extend(
        methods
            .iter()
            .map(|(k, s)| cand(k, "method", Some(s.clone()))),
    );
    out
}

fn namespace_entries(env: &Env, ns: &str) -> Vec<Candidate> {
    let mut v: Vec<(String, &'static str, Option<String>)> = Vec::new();
    let pre = format!("{ns}.");
    if env.group_on(ns) {
        for r in library::records() {
            if r.group.as_deref() == Some(ns) && env.lib_function(&r.name).is_some() {
                if let Some(b) = r.name.strip_prefix(&pre) {
                    v.push((b.to_string(), "function", Some(r.signature.clone())));
                }
            }
        }
        for c in library::constants() {
            if c.group == ns {
                if let Some(b) = c.name.strip_prefix(&pre) {
                    if env.constant(ns, b).is_some() {
                        v.push((b.to_string(), "constant", Some(c.ty.clone())));
                    }
                }
            }
        }
    }
    let d = env.data();
    for f in &d.functions {
        if f.namespace.as_deref() == Some(ns) {
            v.push((
                f.base.clone(),
                "function",
                Some(print_sig(&f.sig, &d.names)),
            ));
        }
    }
    v.sort_by(|a, b| a.0.cmp(&b.0));
    v.into_iter().map(|(l, k, d)| cand(&l, k, d)).collect()
}

fn type_words(env: &Env) -> Vec<Candidate> {
    let d = env.data();
    let mut v: Vec<String> = [
        "any", "bool", "float", "int", "map", "never", "null", "string",
    ]
    .iter()
    .map(|s| s.to_string())
    .collect();
    v.extend(d.names.named.iter().map(|(n, _)| n.clone()));
    v.extend(d.opaque.iter().cloned());
    v.sort();
    v.dedup();
    v.into_iter().map(|n| cand(&n, "type", None)).collect()
}

/// A string written as the content of a one-line string in the quote q
/// (3.11 item 3).
fn escape(s: &str, q: char) -> String {
    let mut out = String::new();
    for c in s.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            c if c == q => {
                out.push('\\');
                out.push(c);
            }
            '\u{8}' => out.push_str("\\b"),
            '\u{c}' => out.push_str("\\f"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out
}

fn has_type_hole(t: &PType) -> bool {
    t.alts.iter().any(|a| match &a.prim {
        PPrim::Name(n, _) => n == HOLE,
        PPrim::Map(x) | PPrim::Paren(x) => has_type_hole(x),
        PPrim::Record { fields, rest } => {
            fields.iter().any(|f| has_type_hole(&f.ty))
                || matches!(rest, Some(Some(x)) if has_type_hole(x))
        }
        _ => false,
    })
}

/// RawCompletion at `offset`, a count of code points of `text` after a U+FEFF
/// at its start (3.5).
pub(crate) fn complete(env: &Env, text: &str, expected: &Want, offset: usize) -> RawCompletion {
    let src = Source::new(text);
    let ch = src.chars();
    let at = Range::new(offset, offset);
    let empty = |span: Range| RawCompletion {
        span,
        items: Vec::new(),
    };
    if offset > ch.len() {
        return empty(at);
    }
    let mut p = offset;
    while p > 0 && is_word_char(ch[p - 1]) {
        p -= 1;
    }
    if p < offset && ch[p].is_ascii_digit() {
        return empty(at);
    }
    let pspan = Range::new(p, offset);
    let prefix: String = ch[p..offset].iter().collect();
    let filter = |items: Vec<Candidate>, pre: &str| -> Vec<Candidate> {
        items
            .into_iter()
            .filter(|c| c.label.starts_with(pre))
            .collect()
    };
    let cut_src = Source::raw(&ch[..offset].iter().collect::<String>());
    match lex_before(ch, p, true) {
        // A cut inside a comment or a literal has no prefix (3.2 item 2).
        Lexed::Fail if inside_literal(ch, offset) => empty(at),
        Lexed::Fail => empty(pspan),
        Lexed::InString(s, q, pre, post) => {
            let value: String = ch[s + 1..offset].iter().collect();
            let sspan = Range::new(s + 1, offset);
            let form = if q == '"' {
                StrForm::Double
            } else {
                StrForm::Single
            };
            let hole_span = Range::new(s, offset);
            let mut toks = pre;
            let mut hole = mk(Tok::Str(value.clone(), form), s);
            hole.span = hole_span;
            hole.diag = hole_span;
            toks.push(hole);
            let Some(ast) = repair(&cut_src, &toks, &post, offset) else {
                return empty(sspan);
            };
            let parsed = Expr {
                source: cut_src.clone(),
                ast,
            };
            let items = strings_at(env, &parsed, expected, hole_span, q);
            RawCompletion {
                span: sspan,
                items: filter(items, &value),
            }
        }
        Lexed::Ok(pre, post) => {
            let fallback = |pre: &[Token]| {
                if pre
                    .last()
                    .is_some_and(|t| t.is_punct(P::Dot) || t.is_punct(P::QDot))
                {
                    Vec::new()
                } else {
                    env_names(env)
                }
            };
            let mut toks = pre.clone();
            let mut hole = mk(Tok::Word(HOLE.into()), p);
            hole.span = pspan;
            hole.diag = pspan;
            toks.push(hole);
            let Some(ast) = repair(&cut_src, &toks, &post, offset) else {
                return RawCompletion {
                    span: pspan,
                    items: filter(fallback(&pre), &prefix),
                };
            };
            let parsed = Expr {
                source: cut_src.clone(),
                ast,
            };
            let items = word_hole(env, &parsed, expected);
            RawCompletion {
                span: pspan,
                items: filter(items, &prefix),
            }
        }
    }
}

/// The candidates where the hole is a word (3.5).
fn word_hole(env: &Env, parsed: &Expr, expected: &Want) -> Vec<Candidate> {
    let ast = &parsed.ast;
    let find = |f: &dyn Fn(&NodeKind) -> bool| {
        ast.nodes
            .iter()
            .position(|n| f(&n.kind))
            .map(|i| NodeId(i as u32))
    };
    if let Some(id) = find(&|k| matches!(k, NodeKind::Ident { name } if name == HOLE)) {
        let tree = Tree::new(env, parsed, expected, Some(id));
        // 3.5: the name of a shorthand entry of a map with an expected record.
        if let Some(e) = tree.parent[id.index()] {
            if let NodeKind::Entry {
                shorthand: true, ..
            } = tree.kind(e)
            {
                if let Some(m) = tree.parent[e.index()] {
                    if let Some(k) = keys(&tree, m, e) {
                        return k;
                    }
                }
            }
        }
        let mut out = tree.scope_names(id);
        for c in env_names(env) {
            if !out.iter().any(|o| o.label == c.label) {
                out.push(c);
            }
        }
        return out;
    }
    let member = find(&|k| match k {
        NodeKind::Member { name, .. } => name == HOLE,
        NodeKind::MethodCall { name, .. } => name.name == HOLE,
        _ => false,
    });
    if let Some(id) = member {
        let tree = Tree::new(env, parsed, expected, None);
        let x = match tree.kind(id) {
            NodeKind::Member { x, .. } | NodeKind::MethodCall { x, .. } => *x,
            _ => return Vec::new(),
        };
        if let NodeKind::Ident { name } = tree.kind(x) {
            let bound = matches!(tree.res(x), Res::Slot(_) | Res::Var(_) | Res::LambdaRef(_));
            if !bound && env.is_namespace(name) {
                return namespace_entries(env, name);
            }
        }
        return members(env, &tree.ty(x));
    }
    if find(&|k| matches!(k, NodeKind::As { ty, .. } if has_type_hole(ty))).is_some() {
        return type_words(env);
    }
    Vec::new()
}

/// The keys context (3.9 item 1), when the map has an expected record.
fn keys(tree: &Tree<'_>, map: NodeId, hole_entry: NodeId) -> Option<Vec<Candidate>> {
    let NodeKind::Map { entries } = tree.kind(map) else {
        return None;
    };
    let e = tree.expected_of(map)?;
    let recs = e.variants();
    if recs.is_empty() {
        return None;
    }
    let mut written: Vec<String> = Vec::new();
    for &x in entries {
        if x == hole_entry {
            continue;
        }
        if let NodeKind::Entry {
            key: EntryKey::Word(k) | EntryKey::Str(k),
            ..
        } = tree.kind(x)
        {
            written.push(k.clone());
        }
    }
    let mut ks: std::collections::BTreeMap<String, Ty> = Default::default();
    for r in recs {
        for (k, f) in &r.fields {
            if written.contains(k) {
                continue;
            }
            let t = match ks.get(k) {
                Some(x) => join(x, &f.ty),
                None => f.ty.clone(),
            };
            ks.insert(k.clone(), t);
        }
    }
    Some(
        ks.into_iter()
            .map(|(k, t)| Candidate {
                insert: if is_word(&k) {
                    k.clone()
                } else {
                    crate::messages::json_string(&k)
                },
                label: k,
                kind: "key",
                detail: Some(print(&t)),
            })
            .collect(),
    )
}

/// The strings context (3.11), where the hole is the string literal at
/// `span`.
fn strings_at(env: &Env, parsed: &Expr, expected: &Want, span: Range, q: char) -> Vec<Candidate> {
    let ast = &parsed.ast;
    let tree = Tree::new(env, parsed, expected, None);
    let lits = |t: &Ty, skip: &[String]| -> Vec<Candidate> {
        match &t.str {
            Some(Strs::Lits(set)) => set
                .iter()
                .filter(|s| !skip.contains(s))
                .map(|s| Candidate {
                    label: s.clone(),
                    kind: "literal",
                    detail: None,
                    insert: escape(s, q),
                })
                .collect(),
            _ => Vec::new(),
        }
    };
    // The literal of a `match` arm.
    for (i, n) in ast.nodes.iter().enumerate() {
        if let NodeKind::Match { subject, arms } = &n.kind {
            let mut hit = false;
            let mut others: Vec<String> = Vec::new();
            for &a in arms {
                if let NodeKind::Arm {
                    literals: Some(ls), ..
                } = tree.kind(a)
                {
                    for l in ls {
                        if l.span == span {
                            hit = true;
                        } else if let Lit::Str(s) = &l.lit {
                            others.push(s.clone());
                        }
                    }
                }
            }
            if hit {
                let _ = i;
                return lits(&tree.ty(*subject), &others);
            }
        }
    }
    let Some(id) = ast
        .nodes
        .iter()
        .position(|n| matches!(n.kind, NodeKind::String { .. }) && n.span == span)
        .map(|i| NodeId(i as u32))
    else {
        return Vec::new();
    };
    if let Some(p) = tree.parent[id.index()] {
        if let NodeKind::Binary {
            op: crate::syntax::ast::BinaryOp::Eq | crate::syntax::ast::BinaryOp::Ne,
            x,
            y,
        } = tree.kind(p)
        {
            let other = if *x == id { *y } else { *x };
            return lits(&tree.ty(other), &[]);
        }
    }
    match tree.expected_of(id) {
        Some(t) => lits(&t, &[]),
        None => Vec::new(),
    }
}

// ---- hover -----------------------------------------------------------------------

/// RawHover at `offset` (3.13).
pub(crate) fn hover(env: &Env, text: &str, expected: &Want, offset: usize) -> Option<RawHover> {
    let src = Source::new(text);
    let ch = src.chars();
    if offset >= ch.len() || !is_word_char(ch[offset]) {
        return None;
    }
    let mut ws = offset;
    while ws > 0 && is_word_char(ch[ws - 1]) {
        ws -= 1;
    }
    let mut we = offset;
    while we < ch.len() && is_word_char(ch[we]) {
        we += 1;
    }
    if ch[ws].is_ascii_digit() {
        return None;
    }
    // The word must be a token of the text: not in a comment or a literal.
    match lex_before(ch, we, false) {
        Lexed::Ok(t, _)
            if t.last()
                .is_some_and(|l| l.span == Range::new(ws, we) && l.word().is_some()) => {}
        _ => return None,
    }
    let wspan = Range::new(ws, we);
    let parsed = match crate::syntax::parse(text) {
        Ok(p) => p,
        Err(_) => {
            let cut = if ch.get(we) == Some(&'(') { we + 1 } else { we };
            let Lexed::Ok(pre, post) = lex_before(ch, cut, false) else {
                return None;
            };
            let cut_src = Source::raw(&ch[..cut].iter().collect::<String>());
            let ast = repair(&cut_src, &pre, &post, cut)?;
            Expr {
                source: cut_src,
                ast,
            }
        }
    };
    let tree = Tree::new(env, &parsed, expected, None);
    hover_in(&tree, wspan)
}

fn hover_in(tree: &Tree<'_>, w: Range) -> Option<RawHover> {
    let env = tree.env;
    let d = env.data();
    let rec = |kind: &'static str, ty: Option<String>| RawHover {
        kind,
        span: w,
        ty,
        signature: None,
        doc: None,
        declaration: None,
    };
    for (i, n) in tree.ast.nodes.iter().enumerate() {
        let id = NodeId(i as u32);
        match &n.kind {
            NodeKind::Ident { name } if n.span == w => {
                let t = Some(print(&tree.ty(id)));
                return match tree.res(id) {
                    Res::Slot(s) => Some(RawHover {
                        declaration: binding_site(tree, *s),
                        ..rec("name", t)
                    }),
                    Res::Var(_) => Some(rec("variable", t)),
                    _ if env.is_namespace(name) => Some(rec("namespace", None)),
                    _ => None,
                };
            }
            NodeKind::Call {
                namespace, name, ..
            } => {
                if namespace.as_ref().is_some_and(|ns| ns.span == w) {
                    return Some(rec("namespace", None));
                }
                if name.span != w {
                    continue;
                }
                return call_hover(tree, id, rec("function", Some(print(&tree.ty(id)))));
            }
            NodeKind::MethodCall { x, name, .. } if name.span == w => {
                let t = Some(print(&tree.ty(id)));
                if matches!(tree.res(id), Res::CallLib(_) | Res::CallHost(_)) {
                    return call_hover(tree, id, rec("function", t));
                }
                let r = tree.ty(*x).nonnull();
                let mut h = rec("method", t);
                let kinds = [
                    (r.str.is_some(), Kind::String),
                    (r.list.is_some(), Kind::List),
                    (r.map.is_some(), Kind::Map),
                ];
                for (has, k) in kinds {
                    if has {
                        if let Some(i) = env.lib_method(k, &name.name) {
                            let r = &library::records()[i];
                            h.signature = Some(r.signature.clone());
                            h.doc = r.doc.clone();
                            return Some(h);
                        }
                    }
                }
                for o in &r.opaque {
                    if let Some(m) = env.method(o, &name.name) {
                        h.signature = Some(print_sig(&d.methods[m].sig, &d.names));
                        return Some(h);
                    }
                }
                return Some(h);
            }
            NodeKind::Member {
                x, name_span, name, ..
            } if *name_span == w => {
                if matches!(tree.res(id), Res::Constant(_)) {
                    let ns = match tree.kind(*x) {
                        NodeKind::Ident { name } => name.clone(),
                        _ => String::new(),
                    };
                    let full = format!("{ns}.{name}");
                    let c = library::constants().iter().find(|c| c.name == full);
                    return Some(RawHover {
                        doc: c.and_then(|c| c.doc.clone()),
                        ..rec("constant", Some(print(&tree.ty(id))))
                    });
                }
                return Some(rec("field", Some(print(&tree.ty(id)))));
            }
            NodeKind::NamePat { .. } | NodeKind::MapPat { .. } | NodeKind::ListPat { .. } => {
                for (_, s, sp) in tree.pattern_names(id) {
                    if sp == w {
                        return Some(RawHover {
                            declaration: Some(w),
                            ..rec("name", s.map(|s| print(&tree.slot_ty(s, false))))
                        });
                    }
                }
            }
            NodeKind::Try { name: Some(nm), .. } if nm.span == w => {
                let s = match tree.res(id) {
                    Res::Catch(s) => *s,
                    _ => None,
                };
                return Some(RawHover {
                    declaration: Some(w),
                    ..rec("name", s.map(|s| print(&tree.slot_ty(s, false))))
                });
            }
            _ => {}
        }
    }
    None
}

fn call_hover(tree: &Tree<'_>, id: NodeId, mut h: RawHover) -> Option<RawHover> {
    let d = tree.env.data();
    match tree.res(id) {
        Res::CallLib(i) => {
            let r = &library::records()[*i as usize];
            h.signature = Some(r.signature.clone());
            h.doc = r.doc.clone();
        }
        Res::CallHost(i) => {
            h.signature = Some(print_sig(&d.functions[*i as usize].sig, &d.names));
        }
        Res::CallLambda(l) => {
            h.kind = "lambda";
            h.declaration = tree.ast.nodes.iter().find_map(|n| match &n.kind {
                NodeKind::Const { pattern, value } if value == l => Some(tree.ast.span(*pattern)),
                _ => None,
            });
        }
        _ => {}
    }
    Some(h)
}

/// The span of the name that binds a slot.
fn binding_site(tree: &Tree<'_>, s: u32) -> Option<Range> {
    for (i, n) in tree.ast.nodes.iter().enumerate() {
        let id = NodeId(i as u32);
        match &n.kind {
            NodeKind::NamePat { .. } | NodeKind::MapPat { .. } | NodeKind::ListPat { .. } => {
                for (_, slot, sp) in tree.pattern_names(id) {
                    if slot == Some(s) {
                        return Some(sp);
                    }
                }
            }
            NodeKind::Try { name: Some(nm), .. } => {
                if matches!(tree.res(id), Res::Catch(Some(x)) if *x == s) {
                    return Some(nm.span);
                }
            }
            _ => {}
        }
    }
    None
}
