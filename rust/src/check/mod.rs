//! The Core checker (`05-types.md` sections 4 to 18).
//!
//! The static rules (section 22) run first, in `compile::resolve`, which also
//! gives every binding a fixed slot. The type rules walk the tree with a type
//! for each slot, the facts in force, and the sites and diagnostics found.
//! What opens no level is walked by a loop (20.6): a left-associative chain
//! of operators, the steps of a chain, the `else` of `? :` and of `if`, and
//! `const` lines. Recursion follows the source nesting, and through the
//! uses of `const`-bound lambdas, which the limits of 20.2 and 20.3 bound.

pub mod ty;

use crate::compile::{resolve, sort_diagnostics, Program, Res};
use crate::diag::{Diagnostic, Error, Phase};
use crate::env::Env;
use crate::library::{self, Record};
use crate::source::{Source, Span};
use crate::syntax::ast::{Ast, BinaryOp, CondForm, EntryKey, Lit, NodeId, NodeKind, UnaryOp};
use crate::syntax::Parsed;
use crate::types::text::{self, Bound, ParamType, Sig};
use std::collections::{BTreeMap, HashMap};
use ty::*;

/// A place where a checked program can still fail on a value (18.3).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Site {
    pub class: &'static str,
    pub span: Span,
}

/// The expected result of a check (`03-host.md` 3.2.3).
#[derive(Clone, Debug, PartialEq)]
pub enum Expected {
    Nothing,
    Type(Ty),
    Condition,
}

/// What a check gives: the program's type and its sites when it checks, and
/// its diagnostics, ordered by 6.1.
#[derive(Clone, Debug)]
pub struct CheckResult {
    pub ty: Option<Ty>,
    pub sites: Vec<Site>,
    pub diagnostics: Vec<Diagnostic>,
}

/// Read the text of an expected result: `condition` or a type.
pub fn expected(env: &Env, text: Option<&str>) -> Result<Expected, Diagnostic> {
    let bad = |m: String| Diagnostic::bare(Phase::Declare, crate::diag::code::BAD_TYPE, m);
    match text {
        None => Ok(Expected::Nothing),
        Some("condition") => Ok(Expected::Condition),
        Some(t) => {
            let parsed = text::parse_type(t).map_err(|e| bad(e.message))?;
            let ty = from_text(&parsed, &env.data().opaque, None).map_err(bad)?;
            Ok(Expected::Type(ty))
        }
    }
}

/// Check a program against an environment (`03-host.md` 3.2.1).
pub fn check(env: &Env, parsed: &Parsed, expected: &Expected) -> CheckResult {
    let r = resolve(env, parsed);
    let mut c = Checker::new(env, parsed, &r.res);
    let t = c.program(expected);
    let mut diags = r.diags;
    diags.append(&mut c.diags);
    let mut sites = Vec::new();
    for s in std::mem::take(&mut c.sites) {
        if c.rejects(s.class) {
            let code = if s.class == "dynamic" {
                "unchecked"
            } else {
                "may-fail"
            };
            let mut d = Diagnostic::at(
                Phase::Check,
                code,
                &parsed.source,
                s.span,
                format!("{code}: {} site", s.class),
            );
            d.class = Some(s.class);
            diags.push(d);
        } else {
            sites.push(s);
        }
    }
    sort_diagnostics(&mut diags);
    sites.sort_by_key(|s| (s.span.start, s.span.end, s.class));
    sites.dedup();
    if diags.is_empty() {
        CheckResult {
            ty: Some(t),
            sites,
            diagnostics: diags,
        }
    } else {
        CheckResult {
            ty: None,
            sites: Vec::new(),
            diagnostics: diags,
        }
    }
}

/// Compile with a check: the program fails to compile exactly when the
/// check reports a diagnostic (`03-host.md` 3.2.1 item 3).
pub fn compile_checked(
    env: &Env,
    parsed: Parsed,
    expected: &Expected,
) -> Result<(Program, CheckResult), Error> {
    let result = check(env, &parsed, expected);
    if !result.diagnostics.is_empty() {
        return Err(Error {
            phase: Phase::Check,
            diagnostics: result.diagnostics,
        });
    }
    let mut p = crate::compile::compile_unchecked(env, parsed)?;
    p.sites = Some(result.sites.clone());
    Ok((p, result))
}

#[derive(Clone, Debug, PartialEq)]
enum Root {
    Slot(u32),
    Var(u32),
}

/// A path (10.3): a binding and field keys.
#[derive(Clone, Debug, PartialEq)]
struct Path {
    root: Root,
    keys: Vec<String>,
}

#[derive(Clone, Copy, PartialEq)]
enum Fit {
    Result,
    Arg,
}

enum Callee<'r> {
    Lib(&'r Record),
    Host(&'r Sig, bool),
}

struct Checker<'a> {
    env: &'a Env,
    ast: &'a Ast,
    src: &'a Source,
    res: &'a [Res],
    slots: Vec<Ty>,
    facts: Vec<Path>,
    diags: Vec<Diagnostic>,
    sites: Vec<Site>,
    types: Vec<Ty>,
    lambda_facts: HashMap<u32, Vec<Path>>,
    uses: HashMap<u32, usize>,
    body_checks: usize,
    offset: i32,
    error_ty: Ty,
    /// How many places whose sites are dropped (a `try` body, the check of
    /// an unused lambda) the walk is inside. There a site is no finding, so
    /// a rejected class does not make its node `never`.
    dropping: u32,
}

fn whole(v: f64) -> bool {
    crate::value::is_whole(v)
}

impl<'a> Checker<'a> {
    fn new(env: &'a Env, parsed: &'a Parsed, res: &'a [Res]) -> Checker<'a> {
        let mut uses = HashMap::new();
        let mut slots_n = 0;
        for r in res {
            match r {
                Res::CallLambda(l) | Res::LambdaRef(l) => *uses.entry(l.0).or_insert(0) += 1,
                Res::Slot(s) | Res::Bind(s) => slots_n = slots_n.max(*s as usize + 1),
                Res::PatSlots { fields, rest } => {
                    for s in fields.iter().flatten().chain(rest.iter()) {
                        slots_n = slots_n.max(*s as usize + 1);
                    }
                }
                Res::Catch(Some(s)) => slots_n = slots_n.max(*s as usize + 1),
                _ => {}
            }
        }
        // 14.2: the error value.
        let codes = crate::messages::catalog()
            .entries
            .iter()
            .filter(|e| e.phase == "run" && e.caught)
            .map(|e| e.code.as_str());
        let mut fields = BTreeMap::new();
        fields.insert(
            "code".to_string(),
            Field {
                ty: Ty::lits(codes),
                optional: false,
            },
        );
        fields.insert(
            "data".to_string(),
            Field {
                ty: Ty::any(),
                optional: false,
            },
        );
        fields.insert(
            "message".to_string(),
            Field {
                ty: Ty::string(),
                optional: false,
            },
        );
        let mut path_elem = Ty::int();
        path_elem.str = Some(Strs::All);
        fields.insert(
            "path".to_string(),
            Field {
                ty: Ty::list(path_elem).with_null(),
                optional: false,
            },
        );
        Checker {
            env,
            ast: &parsed.ast,
            src: &parsed.source,
            res,
            slots: vec![Ty::never(); slots_n],
            facts: Vec::new(),
            diags: Vec::new(),
            sites: Vec::new(),
            types: vec![Ty::never(); parsed.ast.nodes.len()],
            lambda_facts: HashMap::new(),
            uses,
            body_checks: 0,
            offset: 0,
            error_ty: Ty::rec(Rec {
                fields,
                rest: Ty::never(),
                closable: false,
            }),
            dropping: 0,
        }
    }

    fn rejects(&self, class: &str) -> bool {
        self.env.data().reject.iter().any(|c| c == class)
    }

    fn span(&self, id: NodeId) -> Span {
        self.ast.span(id)
    }

    fn kind(&self, id: NodeId) -> &'a NodeKind {
        &self.ast.nodes[id.index()].kind
    }

    fn diag(&mut self, code: &'static str, span: Span) {
        let msg = format!("{code}: {}", self.src.slice(span));
        self.diags
            .push(Diagnostic::at(Phase::Check, code, self.src, span, msg));
    }

    fn diag_class(&mut self, code: &'static str, span: Span, class: &'static str) {
        let msg = format!("{code}: {}", self.src.slice(span));
        let mut d = Diagnostic::at(Phase::Check, code, self.src, span, msg);
        d.class = Some(class);
        self.diags.push(d);
    }

    fn site(&mut self, class: &'static str, span: Span) {
        self.sites.push(Site { class, span });
    }

    /// A `dynamic` site. Returns whether the class is rejected: then the
    /// node takes the type `never` (17).
    fn dyn_site(&mut self, span: Span) -> bool {
        self.site("dynamic", span);
        self.dropping == 0 && self.rejects("dynamic")
    }

    fn opaque(&self) -> &'a [String] {
        &self.env.data().opaque
    }

    // ---- facts --------------------------------------------------------------

    /// The path an expression is, if it is one (10.3). By a loop.
    fn path_of(&self, id: NodeId) -> Option<Path> {
        let mut keys = Vec::new();
        let mut n = id;
        loop {
            match self.kind(n) {
                NodeKind::Ident { .. } => {
                    let root = match &self.res[n.index()] {
                        Res::Slot(s) => Root::Slot(*s),
                        Res::Var(v) => Root::Var(*v),
                        _ => return None,
                    };
                    keys.reverse();
                    return Some(Path { root, keys });
                }
                NodeKind::Member { x, name, .. } => {
                    keys.push(name.clone());
                    n = *x;
                }
                NodeKind::Index { x, index, .. } => match self.kind(*index) {
                    NodeKind::String { value, .. } => {
                        keys.push(value.clone());
                        n = *x;
                    }
                    _ => return None,
                },
                NodeKind::OptChain { x } => n = *x,
                _ => return None,
            }
        }
    }

    fn covered(&self, p: &Path) -> bool {
        self.facts
            .iter()
            .any(|f| f.root == p.root && f.keys.starts_with(&p.keys))
    }

    /// The true and false facts of a tested expression (10.5).
    fn facts_of(&self, id: NodeId) -> (Vec<Path>, Vec<Path>) {
        if let Some(p) = self.path_of(id) {
            return (vec![p], vec![]);
        }
        match self.kind(id) {
            NodeKind::Unary {
                op: UnaryOp::Not,
                x,
            } => {
                let (t, f) = self.facts_of(*x);
                (f, t)
            }
            NodeKind::Binary {
                op: op @ (BinaryOp::Eq | BinaryOp::Ne),
                x,
                y,
            } => {
                let p = match (self.kind(*x), self.kind(*y)) {
                    (_, NodeKind::Null) => self.path_of(*x),
                    (NodeKind::Null, _) => self.path_of(*y),
                    _ => None,
                };
                match (p, op) {
                    (Some(p), BinaryOp::Ne) => (vec![p], vec![]),
                    (Some(p), _) => (vec![], vec![p]),
                    _ => (vec![], vec![]),
                }
            }
            NodeKind::Binary {
                op: op @ (BinaryOp::And | BinaryOp::Or),
                ..
            } => {
                // A chain of one operator along its left operands, by a loop.
                let op = *op;
                let mut operands = Vec::new();
                let mut n = id;
                while let NodeKind::Binary { op: o, x, y } = self.kind(n) {
                    if *o != op {
                        break;
                    }
                    operands.push(*y);
                    n = *x;
                }
                operands.push(n);
                let all: Vec<(Vec<Path>, Vec<Path>)> =
                    operands.iter().map(|&o| self.facts_of(o)).collect();
                let union = |v: Vec<Vec<Path>>| {
                    let mut out: Vec<Path> = Vec::new();
                    for p in v.into_iter().flatten() {
                        if !out.contains(&p) {
                            out.push(p);
                        }
                    }
                    out
                };
                let inter = |v: Vec<Vec<Path>>| {
                    let mut it = v.into_iter();
                    let first = it.next().unwrap_or_default();
                    let rest: Vec<Vec<Path>> = it.collect();
                    first
                        .into_iter()
                        .filter(|p| rest.iter().all(|r| r.contains(p)))
                        .collect::<Vec<_>>()
                };
                let (ts, fs): (Vec<_>, Vec<_>) = all.into_iter().unzip();
                if op == BinaryOp::And {
                    (union(ts), inter(fs))
                } else {
                    (inter(ts), union(fs))
                }
            }
            _ => (vec![], vec![]),
        }
    }

    /// A tested position (10.1, 10.2).
    fn tested(&mut self, id: NodeId, t: &Ty) {
        if !self.rejects("constant-condition") || t.any || t.is_never() {
            return;
        }
        let span = self.span(id);
        if falsy(t).is_never() {
            self.diag_class("always-truthy", span, "constant-condition");
        } else if truthy(t).is_never() {
            self.diag_class("always-falsy", span, "constant-condition");
        }
    }

    // ---- the walk ---------------------------------------------------------------

    fn program(&mut self, expected: &Expected) -> Ty {
        let NodeKind::Program { consts, result } = self.kind(self.ast.root) else {
            return Ty::never();
        };
        for &c in consts {
            self.const_line(c);
        }
        let t = self.synth(*result);
        match expected {
            Expected::Nothing => {}
            Expected::Type(e) => self.fit_form(*result, e, Fit::Result),
            Expected::Condition => self.tested(*result, &t),
        }
        t
    }

    fn synth(&mut self, id: NodeId) -> Ty {
        let t = self.synth_inner(id);
        self.types[id.index()] = t.clone();
        t
    }

    fn synth_inner(&mut self, id: NodeId) -> Ty {
        let span = self.span(id);
        match self.kind(id) {
            NodeKind::Program { .. } => Ty::never(),
            NodeKind::Block { consts, result } => {
                for &c in consts {
                    self.const_line(c);
                }
                self.synth(*result)
            }
            NodeKind::Ident { .. } => self.ident(id),
            NodeKind::Number { value } => {
                if whole(*value) {
                    Ty::int()
                } else {
                    Ty::float()
                }
            }
            NodeKind::String { value, .. } => Ty::lit(value),
            NodeKind::Bool { .. } => Ty::boolean(),
            NodeKind::Null => Ty::null(),
            NodeKind::Template { parts } => {
                for p in parts {
                    if let crate::syntax::ast::TemplatePart::Expr(e) = p {
                        let t = self.synth(*e);
                        self.rendered(*e, &t);
                    }
                }
                Ty::string()
            }
            NodeKind::List { items } => self.list(id, items),
            NodeKind::Map { entries } => self.map(id, entries),
            NodeKind::Unary { op, x } => {
                let t = self.synth(*x);
                match op {
                    UnaryOp::Not => {
                        self.tested(*x, &t);
                        Ty::boolean()
                    }
                    UnaryOp::Neg => {
                        let xs = self.span(*x);
                        let mut t = t;
                        if t.null {
                            self.diag("maybe-null", xs);
                            t = t.nonnull();
                        }
                        if t.any {
                            if self.dyn_site(xs) {
                                return Ty::never();
                            }
                            return Ty::any();
                        }
                        if fits(&t, &Ty::float()).is_none() {
                            self.diag("kind", xs);
                            return Ty::never();
                        }
                        t
                    }
                }
            }
            NodeKind::Binary { .. } => self.binary(id),
            NodeKind::Cond { .. } => self.cond(id),
            NodeKind::Member { .. } | NodeKind::Index { .. } | NodeKind::OptChain { .. } => {
                self.chain(id)
            }
            NodeKind::MethodCall { args, .. } => {
                if let Res::CallHost(i) = self.res[id.index()] {
                    let sig = &self.env.data().functions[i as usize].sig;
                    self.call(Callee::Host(sig, sig.fails), None, args, id, false)
                } else {
                    self.chain(id)
                }
            }
            NodeKind::Call { args, .. } => match self.res[id.index()] {
                Res::CallLambda(l) => self.use_lambda(l, args, id),
                Res::CallLib(i) => self.call(
                    Callee::Lib(&library::records()[i as usize]),
                    None,
                    args,
                    id,
                    false,
                ),
                Res::CallHost(i) => {
                    let sig = &self.env.data().functions[i as usize].sig;
                    self.call(Callee::Host(sig, sig.fails), None, args, id, false)
                }
                _ => {
                    self.plain_args(args);
                    Ty::never()
                }
            },
            NodeKind::Match { subject, arms } => self.match_(id, *subject, arms),
            NodeKind::Try { body, handler, .. } => {
                let n = self.sites.len();
                self.dropping += 1;
                let b = self.synth(*body);
                self.dropping -= 1;
                self.sites.truncate(n);
                if let Res::Catch(Some(s)) = self.res[id.index()] {
                    self.slots[s as usize] = self.error_ty.clone();
                }
                let h = self.synth(*handler);
                join(&b, &h)
            }
            NodeKind::Spread { value } => self.synth(*value),
            _ => {
                let _ = span;
                Ty::never()
            }
        }
    }

    fn const_line(&mut self, c: NodeId) {
        let NodeKind::Const { pattern, value } = self.kind(c) else {
            return;
        };
        if let (NodeKind::Lambda { params, body }, NodeKind::NamePat { name: Some(_) }) =
            (self.kind(*value), self.kind(*pattern))
        {
            self.lambda_facts.insert(value.0, self.facts.clone());
            if self.uses.get(&value.0).copied().unwrap_or(0) == 0 {
                // 13.9: an unused lambda is checked once, with every
                // parameter `any`. Its sites are dropped.
                let n = self.sites.len();
                for &p in params {
                    if let NodeKind::Param { pattern, default } = self.kind(p) {
                        let t = match default {
                            Some(d) => {
                                let dt = self.synth(*d);
                                join(&Ty::any(), &dt)
                            }
                            None => Ty::any(),
                        };
                        let s = self.span(*pattern);
                        self.bind_pattern(*pattern, t, s);
                    }
                }
                self.dropping += 1;
                self.synth(*body);
                self.dropping -= 1;
                self.sites.truncate(n);
            }
            return;
        }
        let v = self.synth(*value);
        let s = self.span(*value);
        self.bind_pattern(*pattern, v, s);
    }

    fn ident(&mut self, id: NodeId) -> Ty {
        let (root, t) = match &self.res[id.index()] {
            Res::Slot(s) => (Root::Slot(*s), self.slots[*s as usize].clone()),
            Res::Var(v) => (
                Root::Var(*v),
                self.env.data().var_types[*v as usize].clone(),
            ),
            _ => return Ty::never(),
        };
        if self.covered(&Path { root, keys: vec![] }) {
            t.nonnull()
        } else {
            t
        }
    }

    /// A rendered expression (ruling AO2): has-opaque is `kind`, has-any a
    /// `dynamic` site; null is accepted.
    fn rendered(&mut self, e: NodeId, t: &Ty) {
        let s = self.span(e);
        if has_opaque(t) {
            self.diag("kind", s);
        } else if has_any(t) {
            self.dyn_site(s);
        }
    }

    /// An operand that must fit a type, with the null rule (1.3, 8.3). `None`
    /// after `kind`.
    fn operand(&mut self, id: NodeId, t: Ty, target: &Ty) -> Option<Ty> {
        let s = self.span(id);
        let mut t = t;
        if t.null {
            self.diag("maybe-null", s);
            t = t.nonnull();
        }
        match fits(&t, target) {
            None => {
                self.diag("kind", s);
                None
            }
            Some(used) => {
                if used && self.dyn_site(s) {
                    return Some(Ty::never());
                }
                Some(t)
            }
        }
    }

    fn depth_check(&mut self, id: NodeId, t: Ty) -> Ty {
        if t.depth() > 64 {
            let s = self.span(id);
            self.diag("check-limit", s);
            return Ty::never();
        }
        t
    }

    fn list(&mut self, id: NodeId, items: &'a [NodeId]) -> Ty {
        let mut j = Ty::never();
        for &item in items {
            if let NodeKind::Spread { value } = self.kind(item) {
                let mut t = self.synth(*value);
                let vs = self.span(*value);
                if t.null {
                    self.diag("maybe-null", vs);
                    t = t.nonnull();
                }
                if t.any {
                    if !self.dyn_site(vs) {
                        j = join(&j, &Ty::any());
                    }
                } else if t.is_never() {
                } else if let (1, Some(e)) = (t.kinds(), &t.list) {
                    j = join(&j, e);
                } else {
                    self.diag("kind", vs);
                }
                self.types[item.index()] = t;
            } else {
                let t = self.synth(item);
                j = join(&j, &t);
            }
        }
        self.depth_check(id, Ty::list(j))
    }

    fn map(&mut self, id: NodeId, entries: &'a [NodeId]) -> Ty {
        let mut rec = Rec {
            fields: BTreeMap::new(),
            rest: Ty::never(),
            closable: false,
        };
        for &e in entries {
            match self.kind(e) {
                NodeKind::Entry { key, value, .. } => match key {
                    EntryKey::Word(k) | EntryKey::Str(k) => {
                        let t = self.synth(*value);
                        rec.fields.insert(
                            k.clone(),
                            Field {
                                ty: t,
                                optional: false,
                            },
                        );
                    }
                    EntryKey::Computed(k) => {
                        let kt = self.synth(*k);
                        self.operand(*k, kt, &Ty::string());
                        let v = self.synth(*value);
                        rec = merge(&rec, Ty::map_of(v).record());
                    }
                },
                NodeKind::Spread { value } => {
                    let mut t = self.synth(*value);
                    let vs = self.span(*value);
                    if t.null {
                        self.diag("maybe-null", vs);
                        t = t.nonnull();
                    }
                    if t.any {
                        if !self.dyn_site(vs) {
                            rec = merge(&rec, None);
                        }
                    } else if t.is_never() {
                    } else if t.kinds() == 1 && t.map.is_some() {
                        rec = merge(&rec, t.record());
                    } else {
                        self.diag("kind", vs);
                    }
                    self.types[e.index()] = t;
                }
                _ => {}
            }
        }
        self.depth_check(id, Ty::rec(rec))
    }

    // ---- operators --------------------------------------------------------------

    fn binary(&mut self, id: NodeId) -> Ty {
        let mut stack = Vec::new();
        let mut n = id;
        while let NodeKind::Binary { x, .. } = self.kind(n) {
            stack.push(n);
            n = *x;
        }
        let mut cur = self.synth(n);
        // The facts of the left operand, carried along a chain of `&&` and
        // `||` so that a long chain costs no more than its length (20.6).
        let mut carried: Option<(Vec<Path>, Vec<Path>)> = None;
        while let Some(b) = stack.pop() {
            let NodeKind::Binary { op, x, y } = self.kind(b) else {
                continue;
            };
            let (x, y) = (*x, *y);
            let left = carried.take();
            cur = match op {
                BinaryOp::And | BinaryOp::Or => {
                    self.tested(x, &cur);
                    let (t, f) = left.unwrap_or_else(|| self.facts_of(x));
                    let k = self.facts.len();
                    self.facts.extend(if *op == BinaryOp::And {
                        t.clone()
                    } else {
                        f.clone()
                    });
                    let bt = self.synth(y);
                    self.facts.truncate(k);
                    let (ty, fy) = self.facts_of(y);
                    carried = Some(if *op == BinaryOp::And {
                        (union(t, ty), inter(f, fy))
                    } else {
                        (inter(t, ty), union(f, fy))
                    });
                    if *op == BinaryOp::And {
                        join(&falsy(&cur), &bt)
                    } else {
                        join(&truthy(&cur), &bt)
                    }
                }
                BinaryOp::Coalesce => {
                    let bt = self.synth(y);
                    join(&cur.nonnull(), &bt)
                }
                BinaryOp::Eq | BinaryOp::Ne => {
                    let bt = self.synth(y);
                    // Ruling AO5: not when a side is `never`.
                    if !overlap(&cur, &bt)
                        && !cur.is_never()
                        && !bt.is_never()
                        && !cur.is_exactly_null()
                        && !bt.is_exactly_null()
                    {
                        let s = self.span(b);
                        self.diag("never-equal", s);
                    }
                    Ty::boolean()
                }
                BinaryOp::In | BinaryOp::NotIn => {
                    let bt = self.synth(y);
                    self.in_op(b, x, cur, y, bt)
                }
                _ => {
                    let bt = self.synth(y);
                    self.arith(*op, b, x, cur, y, bt)
                }
            };
            self.types[b.index()] = cur.clone();
        }
        cur
    }

    fn number_node(&self, id: NodeId) -> Option<f64> {
        match self.kind(id) {
            NodeKind::Number { value } => Some(*value),
            _ => None,
        }
    }

    fn arith(&mut self, op: BinaryOp, node: NodeId, x: NodeId, a: Ty, y: NodeId, b: Ty) -> Ty {
        let (xs, ys, ns) = (self.span(x), self.span(y), self.span(node));
        let (mut a, mut b) = (a, b);
        if a.null {
            self.diag("maybe-null", xs);
            a = a.nonnull();
        }
        if b.null {
            self.diag("maybe-null", ys);
            b = b.nonnull();
        }
        let mut any = false;
        if a.any {
            any = true;
            if self.dyn_site(xs) {
                a = Ty::never();
            }
        }
        if b.any {
            any = true;
            if self.dyn_site(ys) {
                b = Ty::never();
            }
        }
        any = any && (a.any || b.any);
        let fit = |t: &Ty, target: Ty| fits(t, &target).is_some();
        let order = matches!(
            op,
            BinaryOp::Lt | BinaryOp::Le | BinaryOp::Gt | BinaryOp::Ge
        );
        match op {
            BinaryOp::Add | BinaryOp::Lt | BinaryOp::Le | BinaryOp::Gt | BinaryOp::Ge => {
                let (af, as_) = (fit(&a, Ty::float()), fit(&a, Ty::string()));
                let (bf, bs) = (fit(&b, Ty::float()), fit(&b, Ty::string()));
                let mut kinded = false;
                if !af && !as_ {
                    self.diag("kind", xs);
                    kinded = true;
                }
                if !bf && !bs {
                    self.diag("kind", ys);
                    kinded = true;
                }
                if !kinded && ((af && !as_ && bs && !bf) || (as_ && !af && bf && !bs)) {
                    self.diag("kind", ns);
                    kinded = true;
                }
                if order {
                    return Ty::boolean();
                }
                if kinded {
                    return Ty::never();
                }
                if any {
                    return Ty::any();
                }
                if fit(&a, Ty::int()) && fit(&b, Ty::int()) {
                    Ty::int()
                } else if af && bf {
                    Ty::float()
                } else {
                    Ty::string()
                }
            }
            BinaryOp::Sub | BinaryOp::Mul | BinaryOp::Rem | BinaryOp::Div => {
                let mut kinded = false;
                if !fit(&a, Ty::float()) {
                    self.diag("kind", xs);
                    kinded = true;
                }
                if !fit(&b, Ty::float()) {
                    self.diag("kind", ys);
                    kinded = true;
                }
                let lit = self.number_node(y);
                if op == BinaryOp::Div && lit == Some(0.0) {
                    self.diag("zero-divisor", ys);
                }
                if matches!(op, BinaryOp::Rem | BinaryOp::Div) && !lit.is_some_and(|v| v != 0.0) {
                    self.site("divide", ns);
                }
                if kinded {
                    return Ty::never();
                }
                if any {
                    return Ty::any();
                }
                if op != BinaryOp::Div && fit(&a, Ty::int()) && fit(&b, Ty::int()) {
                    Ty::int()
                } else {
                    Ty::float()
                }
            }
            BinaryOp::Pow => {
                let mut kinded = false;
                if !fit(&a, Ty::float()) {
                    self.diag("kind", xs);
                    kinded = true;
                }
                if !fit(&b, Ty::int()) {
                    self.diag("kind", ys);
                    kinded = true;
                }
                let yl = self.number_node(y);
                let xl = self.number_node(x);
                if yl.is_none() && !xl.is_some_and(|v| v != 0.0) {
                    self.site("divide", ns);
                }
                if kinded {
                    return Ty::never();
                }
                if any {
                    return Ty::any();
                }
                if fit(&a, Ty::int()) && yl.is_some() {
                    Ty::int()
                } else {
                    Ty::float()
                }
            }
            _ => Ty::never(),
        }
    }

    fn in_op(&mut self, node: NodeId, x: NodeId, a: Ty, y: NodeId, b: Ty) -> Ty {
        let (xs, ys) = (self.span(x), self.span(y));
        let mut b = b;
        if b.null {
            self.diag("maybe-null", ys);
            b = b.nonnull();
        }
        if b.any {
            self.dyn_site(ys);
            return Ty::boolean();
        }
        if b.is_never() {
            return Ty::boolean();
        }
        if let (1, Some(e)) = (b.kinds(), &b.list) {
            if !overlap(&a, e)
                && !a.is_never()
                && !e.is_never()
                && !a.is_exactly_null()
                && !e.is_exactly_null()
            {
                let s = self.span(node);
                self.diag("never-equal", s);
            }
            return Ty::boolean();
        }
        if b.kinds() == 1 && (b.str.is_some() || b.map.is_some()) {
            let mut a = a;
            if a.null {
                self.diag("maybe-null", xs);
                a = a.nonnull();
            }
            match fits(&a, &Ty::string()) {
                None => self.diag("kind", xs),
                Some(true) => {
                    self.dyn_site(xs);
                }
                Some(false) => {}
            }
            return Ty::boolean();
        }
        self.diag("kind", ys);
        Ty::boolean()
    }

    // ---- choice -----------------------------------------------------------------

    fn cond(&mut self, id: NodeId) -> Ty {
        let k = self.facts.len();
        let mut acc = Ty::never();
        let mut n = id;
        let mut chain: Vec<(NodeId, Ty)> = Vec::new();
        while let NodeKind::Cond {
            test, then, else_, ..
        } = self.kind(n)
        {
            let t = self.synth(*test);
            self.tested(*test, &t);
            let (tf, ff) = self.facts_of(*test);
            let k2 = self.facts.len();
            self.facts.extend(tf);
            let bt = self.synth(*then);
            self.facts.truncate(k2);
            self.facts.extend(ff);
            acc = join(&acc, &bt);
            chain.push((n, bt));
            n = *else_;
        }
        let et = self.synth(n);
        acc = join(&acc, &et);
        self.facts.truncate(k);
        let mut below = et;
        for (c, bt) in chain.into_iter().rev() {
            below = join(&bt, &below);
            self.types[c.index()] = below.clone();
        }
        // The type of each `Cond` of the chain is the Join from it down; the
        // outer one is what the walk returns.
        acc
    }

    fn lit_ty(l: &Lit) -> Ty {
        match l {
            Lit::Number(v) => {
                if whole(*v) {
                    Ty::int()
                } else {
                    Ty::float()
                }
            }
            Lit::Str(s) => Ty::lit(s),
            Lit::Bool(_) => Ty::boolean(),
            Lit::Null => Ty::null(),
        }
    }

    fn match_(&mut self, id: NodeId, subject: NodeId, arms: &'a [NodeId]) -> Ty {
        let s = self.synth(subject);
        let p = self.path_of(subject);
        let k = self.facts.len();
        let mut acc = Ty::never();
        let mut default = false;
        let mut lits: Vec<&Lit> = Vec::new();
        for &a in arms {
            let NodeKind::Arm { literals, result } = self.kind(a) else {
                continue;
            };
            let mut has_null = false;
            match literals {
                None => default = true,
                Some(ls) => {
                    for l in ls {
                        if !matches!(l.lit, Lit::Null)
                            && !s.is_never()
                            && !overlap(&Self::lit_ty(&l.lit), &s)
                        {
                            self.diag("never-equal", l.span);
                        }
                        has_null |= matches!(l.lit, Lit::Null);
                        lits.push(&l.lit);
                    }
                }
            }
            let rt = self.synth(*result);
            acc = join(&acc, &rt);
            if has_null {
                if let Some(p) = &p {
                    self.facts.push(p.clone());
                }
            }
        }
        self.facts.truncate(k);
        if !default && !s.is_never() {
            let covered = finite_values(&s).is_some_and(|vals| {
                vals.iter().all(|v| {
                    lits.iter().any(|l| match (v, l) {
                        (FiniteValue::Null, Lit::Null) => true,
                        (FiniteValue::Bool(a), Lit::Bool(b)) => a == b,
                        (FiniteValue::Str(a), Lit::Str(b)) => a == b,
                        _ => false,
                    })
                })
            });
            if !covered {
                let sp = self.span(id);
                self.diag("non-exhaustive", sp);
            }
        }
        acc
    }

    // ---- patterns -----------------------------------------------------------------

    fn set_names(&mut self, pat: NodeId, t: &Ty) {
        match &self.res[pat.index()] {
            Res::Bind(s) => self.slots[*s as usize] = t.clone(),
            Res::PatSlots { fields, rest } => {
                for s in fields.iter().flatten().chain(rest.iter()) {
                    self.slots[*s as usize] = t.clone();
                }
                if let NodeKind::ListPat { elems, .. } = self.kind(pat) {
                    for e in elems {
                        self.set_names(e.pat, t);
                    }
                }
            }
            _ => {}
        }
    }

    /// Bind a pattern against a type (12.2). `bound` is the span of the
    /// value bound (12.3).
    fn bind_pattern(&mut self, pat: NodeId, v: Ty, bound: Span) {
        match self.kind(pat) {
            NodeKind::NamePat { .. } => self.set_names(pat, &v),
            NodeKind::MapPat { fields, rest } => {
                let mut v = v;
                if v.null {
                    self.diag("maybe-null", bound);
                    v = v.nonnull();
                }
                if v.any {
                    let t = if self.dyn_site(bound) {
                        Ty::never()
                    } else {
                        Ty::any()
                    };
                    self.set_names(pat, &t);
                    for f in fields {
                        if let Some(d) = f.default {
                            self.synth(d);
                        }
                    }
                    return;
                }
                if v.is_never() || !(v.kinds() == 1 && v.map.is_some()) {
                    if !v.is_never() {
                        self.diag("kind", bound);
                    }
                    self.set_names(pat, &Ty::never());
                    return;
                }
                let rec = v.map.as_ref().unwrap().as_ref().clone();
                let slots = match &self.res[pat.index()] {
                    Res::PatSlots { fields, rest } => (fields.clone(), *rest),
                    _ => (Vec::new(), None),
                };
                for (i, f) in fields.iter().enumerate() {
                    let t = match f.default {
                        Some(d) => {
                            let read = match rec.fields.get(&f.key) {
                                Some(fl) if !fl.optional => fl.ty.clone(),
                                Some(fl) => fl.ty.clone().with_null(),
                                None if rec.rest.any => Ty::any(),
                                None if rec.rest.is_never() => {
                                    self.diag("no-field", f.key_span);
                                    Ty::never()
                                }
                                None => rec.rest.clone().with_null(),
                            };
                            let dt = self.synth(d);
                            join(&read.nonnull(), &dt)
                        }
                        None => match rec.fields.get(&f.key) {
                            Some(fl) if !fl.optional => fl.ty.clone(),
                            Some(fl) => {
                                self.diag("maybe-absent", f.key_span);
                                fl.ty.clone()
                            }
                            None if rec.rest.any => {
                                // Ruling AO6 (a): the site is on the key.
                                if self.dyn_site(f.key_span) {
                                    Ty::never()
                                } else {
                                    Ty::any()
                                }
                            }
                            None if rec.rest.is_never() => {
                                self.diag("no-field", f.key_span);
                                Ty::never()
                            }
                            None => {
                                self.diag("maybe-absent", f.key_span);
                                rec.rest.clone()
                            }
                        },
                    };
                    if let Some(Some(s)) = slots.0.get(i) {
                        self.slots[*s as usize] = t;
                    }
                }
                if rest.is_some() {
                    let mut r = rec.clone();
                    for f in fields {
                        r.fields.remove(&f.key);
                    }
                    if let Some(s) = slots.1 {
                        self.slots[s as usize] = Ty::rec(r);
                    }
                }
            }
            NodeKind::ListPat { elems, rest } => {
                let mut v = v;
                if v.null {
                    self.diag("maybe-null", bound);
                    v = v.nonnull();
                }
                if v.any {
                    let t = if self.dyn_site(bound) {
                        Ty::never()
                    } else {
                        Ty::any()
                    };
                    self.set_names(pat, &t);
                    for e in elems {
                        match e.default {
                            Some(d) => {
                                self.synth(d);
                            }
                            None => {
                                // Ruling AO6 (b).
                                let s = self.span(e.pat);
                                self.site("index", s);
                            }
                        }
                    }
                    return;
                }
                if v.is_never() || !(v.kinds() == 1 && v.list.is_some()) {
                    if !v.is_never() {
                        self.diag("kind", bound);
                    }
                    self.set_names(pat, &Ty::never());
                    return;
                }
                let e = v.list.as_ref().unwrap().as_ref().clone();
                for el in elems {
                    let t = match el.default {
                        Some(d) => {
                            let dt = self.synth(d);
                            join(&e.nonnull(), &dt)
                        }
                        None => {
                            let s = self.span(el.pat);
                            self.site("index", s);
                            e.clone()
                        }
                    };
                    self.set_names(el.pat, &t);
                }
                if rest.is_some() {
                    if let Res::PatSlots { rest: Some(s), .. } = self.res[pat.index()] {
                        self.slots[s as usize] = v.clone();
                    }
                }
            }
            _ => {}
        }
    }

    // ---- chains -------------------------------------------------------------------

    fn chain(&mut self, top: NodeId) -> Ty {
        let inner = match self.kind(top) {
            NodeKind::OptChain { x } => *x,
            _ => top,
        };
        let mut steps = Vec::new();
        let mut n = inner;
        loop {
            match self.kind(n) {
                NodeKind::Member { x, .. } | NodeKind::Index { x, .. } => {
                    steps.push(n);
                    n = *x;
                }
                NodeKind::MethodCall { x, .. }
                    if !matches!(self.res[n.index()], Res::CallHost(_)) =>
                {
                    steps.push(n);
                    n = *x;
                }
                _ => break,
            }
        }
        let mut cur = self.synth(n);
        let mut path = match self.kind(n) {
            NodeKind::Ident { .. } => self.path_of(n),
            _ => None,
        };
        let mut skipped = false;
        let mut recv = n;
        for &s in steps.iter().rev() {
            let rs = self.span(recv);
            let optional = match self.kind(s) {
                NodeKind::Member { optional, .. }
                | NodeKind::Index { optional, .. }
                | NodeKind::MethodCall { optional, .. } => *optional,
                _ => false,
            };
            if cur.null {
                if optional {
                    skipped = true;
                } else {
                    self.diag("maybe-null", rs);
                }
                cur = cur.nonnull();
            }
            if !(cur.any || cur.is_never() || cur.kinds() == 1) {
                self.diag("kind", rs);
                cur = Ty::never();
            }
            let key: Option<(String, Span)> = match self.kind(s) {
                NodeKind::Member {
                    name, name_span, ..
                } => Some((name.clone(), *name_span)),
                NodeKind::Index { index, .. } => match self.kind(*index) {
                    // A `String` key reads a field of a record (9.2). On any
                    // other receiver it is an index like any other.
                    NodeKind::String { value, .. }
                        if cur.map.is_some() || cur.any || cur.is_never() =>
                    {
                        Some((value.clone(), self.span(*index)))
                    }
                    _ => None,
                },
                _ => None,
            };
            let next_path = match (&path, &key) {
                (Some(p), Some((k, _))) => {
                    let mut p = p.clone();
                    p.keys.push(k.clone());
                    Some(p)
                }
                _ => None,
            };
            cur = if cur.any {
                let rejected = self.dyn_site(rs);
                match self.kind(s) {
                    NodeKind::Index { index, .. } => {
                        self.synth(*index);
                    }
                    NodeKind::MethodCall { args, .. } => self.any_args(args),
                    _ => {}
                }
                if rejected {
                    Ty::never()
                } else {
                    Ty::any()
                }
            } else if cur.is_never() {
                match self.kind(s) {
                    NodeKind::Index { index, .. } => {
                        self.synth(*index);
                    }
                    NodeKind::MethodCall { args, .. } => self.plain_args(args),
                    _ => {}
                }
                Ty::never()
            } else {
                match self.kind(s) {
                    NodeKind::MethodCall { name, args, x, .. } => {
                        self.method(s, *x, cur, &name.name, name.span, args)
                    }
                    NodeKind::Index { index, .. } if key.is_none() => {
                        self.index(s, rs, cur, *index, optional)
                    }
                    _ => {
                        let (k, ks) = key.clone().unwrap_or_default();
                        if let NodeKind::Index { index, .. } = self.kind(s) {
                            self.types[index.index()] = Ty::lit(&k);
                        }
                        let present = next_path.as_ref().is_some_and(|p| self.covered(p));
                        self.read_field(&cur, &k, ks, rs, optional, present)
                    }
                }
            };
            path = next_path;
            if path.as_ref().is_some_and(|p| self.covered(p)) {
                cur = cur.nonnull();
            }
            self.types[s.index()] = cur.clone();
            recv = s;
        }
        if skipped {
            cur = cur.with_null();
        }
        cur
    }

    /// A read of a field (9.2).
    fn read_field(
        &mut self,
        cur: &Ty,
        key: &str,
        ks: Span,
        rs: Span,
        optional: bool,
        present: bool,
    ) -> Ty {
        let Some(rec) = cur.record() else {
            self.diag("kind", rs);
            return Ty::never();
        };
        match rec.fields.get(key) {
            Some(f) if !f.optional || present => f.ty.clone(),
            Some(f) => {
                if optional {
                    f.ty.clone().with_null()
                } else {
                    self.diag("maybe-absent", ks);
                    f.ty.clone()
                }
            }
            None if rec.rest.any => {
                // Ruling AO13: under a fact the read cannot fail.
                if !optional && !present && self.dyn_site(rs) {
                    return Ty::never();
                }
                Ty::any()
            }
            None if rec.rest.is_never() => {
                self.diag("no-field", ks);
                Ty::never()
            }
            None => {
                if present {
                    rec.rest.clone()
                } else if optional {
                    rec.rest.clone().with_null()
                } else {
                    self.diag("maybe-absent", ks);
                    rec.rest.clone()
                }
            }
        }
    }

    /// An index read with an expression that is not a `String` node (9.2).
    fn index(&mut self, s: NodeId, rs: Span, cur: Ty, index: NodeId, optional: bool) -> Ty {
        let it = self.synth(index);
        let ss = self.span(s);
        if let Some(rec) = cur.record() {
            let v = join_all(
                rec.fields
                    .values()
                    .map(|f| &f.ty)
                    .chain(std::iter::once(&rec.rest)),
            );
            let any_rest = rec.rest.any;
            // Ruling AO11: one mistake, one diagnostic.
            if self.operand(index, it, &Ty::string()).is_none() {
                return Ty::never();
            }
            if optional {
                return v.with_null();
            }
            if any_rest {
                if self.dyn_site(rs) {
                    return Ty::never();
                }
            } else {
                let is = self.span(index);
                self.diag("maybe-absent", is);
            }
            return v;
        }
        let elem = if let Some(e) = &cur.list {
            e.as_ref().clone()
        } else if cur.str.is_some() {
            Ty::string()
        } else {
            self.diag("kind", rs);
            return Ty::never();
        };
        self.operand(index, it, &Ty::int());
        if optional {
            elem.with_null()
        } else {
            self.site("index", ss);
            elem
        }
    }

    // ---- calls --------------------------------------------------------------------

    /// The lambda of a lambda argument (13.2).
    fn lambda_of(&self, a: NodeId) -> Option<(NodeId, bool)> {
        match (self.kind(a), &self.res[a.index()]) {
            (NodeKind::Lambda { .. }, _) => Some((a, false)),
            (_, Res::LambdaRef(l)) => Some((*l, true)),
            _ => None,
        }
    }

    fn plain_args(&mut self, args: &'a [NodeId]) {
        for &a in args {
            if self.lambda_of(a).is_none() {
                self.synth(a);
            }
        }
    }

    /// The arguments of a method call on `any` (13.3): plain arguments are
    /// synthesized, and each lambda is checked with `any` parameters.
    fn any_args(&mut self, args: &'a [NodeId]) {
        for &a in args {
            match self.lambda_of(a) {
                None => {
                    self.synth(a);
                }
                Some((l, named)) => {
                    let n = match self.kind(l) {
                        NodeKind::Lambda { params, .. } => params.len(),
                        _ => 0,
                    };
                    self.check_lambda(l, named, a, &vec![Ty::any(); n]);
                }
            }
        }
    }

    fn method(
        &mut self,
        s: NodeId,
        x: NodeId,
        cur: Ty,
        name: &str,
        name_span: Span,
        args: &'a [NodeId],
    ) -> Ty {
        // The receiver, as argument zero, is fitted with its type after the
        // null step of 9.1, not as the chain before it gave it.
        self.types[x.index()] = cur.clone();
        let kind = if cur.str.is_some() {
            Some(crate::value::Kind::String)
        } else if cur.list.is_some() {
            Some(crate::value::Kind::List)
        } else if cur.map.is_some() {
            Some(crate::value::Kind::Map)
        } else {
            None
        };
        if let Some(k) = kind {
            if let Some(i) = library::method(k, name) {
                let rec = &library::records()[i];
                return self.call(Callee::Lib(rec), Some((x, cur)), args, s, true);
            }
        } else if let Some(t) = cur.opaque.iter().next() {
            if let Some(mi) = self.env.method(t, name) {
                let sig = &self.env.data().methods[mi].sig;
                return self.call(Callee::Host(sig, sig.fails), None, args, s, true);
            }
        }
        self.diag("no-method", name_span);
        self.plain_args(args);
        Ty::never()
    }

    /// Solve one call of a library entry, a host function or a host method
    /// (13.5).
    fn call(
        &mut self,
        callee: Callee<'_>,
        recv: Option<(NodeId, Ty)>,
        args: &'a [NodeId],
        call: NodeId,
        method: bool,
    ) -> Ty {
        let cs = self.span(call);
        let (sig, recv_text, narrows, tested, renders, site) = match &callee {
            Callee::Lib(r) => (
                &r.sig,
                r.recv.as_ref(),
                r.narrows,
                r.tested.clone(),
                r.renders.clone(),
                if r.sig.fails { Some("value") } else { None },
            ),
            Callee::Host(s, f) => (
                *s,
                None,
                false,
                Vec::new(),
                Vec::new(),
                if *f { Some("host") } else { None },
            ),
        };
        let sig: Sig = sig.clone();
        let recv_text = recv_text.cloned();
        // 1. Count.
        let n = args.len();
        if n < sig.required() || sig.at_most().is_some_and(|m| n > m) {
            if method {
                self.diag("arity", cs);
            }
            self.plain_args(args);
            return Ty::never();
        }
        let mut sol = Solutions::new();
        // 2. Plain arguments, the receiver first.
        if let (Some(rt), Some((_, rty))) = (&recv_text, &recv) {
            bind(rt, rty, &mut sol);
        }
        let mut plain: Vec<(NodeId, text::Type)> = Vec::new();
        let mut lambdas: Vec<(NodeId, NodeId, bool, text::FnType)> = Vec::new();
        for (i, &a) in args.iter().enumerate() {
            let Some(p) = sig.param_at(i) else { continue };
            match (&p.ty, self.lambda_of(a)) {
                (ParamType::Fn(_), None) => {
                    self.synth(a);
                    let s = self.span(a);
                    self.diag("arg-mismatch", s);
                }
                (ParamType::Type(_), Some(_)) => {
                    if method {
                        let s = self.span(a);
                        self.diag("not-a-value", s);
                    }
                }
                (ParamType::Type(t), None) => {
                    let at = self.synth(a);
                    if tested.contains(&i) {
                        self.tested(a, &at);
                    }
                    if renders.contains(&p.name) {
                        self.rendered(a, &at);
                    }
                    let pt = match (p.rest, t) {
                        (true, text::Type::List(e)) => e.as_ref().clone(),
                        _ => t.clone(),
                    };
                    bind(&pt, &at, &mut sol);
                    plain.push((a, pt));
                }
                (ParamType::Fn(f), Some((l, named))) => lambdas.push((a, l, named, f.clone())),
            }
        }
        // 3. Lambda arguments.
        let mut narrowed: Option<Ty> = None;
        for (a, l, named, f) in lambdas {
            let as_ = self.span(a);
            let lparams = match self.kind(l) {
                NodeKind::Lambda { params, .. } => params.clone(),
                _ => continue,
            };
            if lparams.len() > f.params.len() {
                if method {
                    self.diag("arity", as_);
                }
                continue;
            }
            if f.params
                .iter()
                .any(|(_, q)| vars(q).iter().any(|v| !sol.contains_key(v)))
            {
                self.diag("cannot-infer", as_);
                continue;
            }
            let qs: Vec<Ty> = f
                .params
                .iter()
                .map(|(_, q)| substitute(q, self.opaque(), &sol))
                .collect();
            let (dn, sn) = (self.diags.len(), self.sites.len());
            let before = sol.clone();
            let b = self.check_lambda(l, named, a, &qs);
            let body = match self.kind(l) {
                NodeKind::Lambda { body, .. } => *body,
                _ => l,
            };
            let bs = self.span(body);
            match &f.result {
                None => {
                    // 13.7: a predicate.
                    self.tested(body, &b);
                    if narrows {
                        narrowed = Some(self.narrow(l, &qs));
                    }
                }
                Some(r) => {
                    let rv = vars(r);
                    // 3a, with ruling AO3: also when B fits A0 only by a fit
                    // that used any.
                    let seed = rv.iter().find(|x| {
                        f.params.iter().any(|(_, q)| vars(q).contains(x))
                            && before.get(x).is_some_and(|a0| fits(&b, a0) != Some(false))
                    });
                    if let Some(&x) = seed {
                        // 3a. The seed rule.
                        self.diags.truncate(dn);
                        self.sites.truncate(sn);
                        let a0 = before.get(&x).cloned().unwrap_or_default();
                        sol.insert(x, join(&a0, &b));
                        let qs: Vec<Ty> = f
                            .params
                            .iter()
                            .map(|(_, q)| substitute(q, self.opaque(), &sol))
                            .collect();
                        let b2 = self.check_lambda(l, named, a, &qs);
                        let rt = substitute(r, self.opaque(), &sol);
                        match fits(&b2, &rt) {
                            None => self.diag("arg-mismatch", bs),
                            Some(true) => {
                                self.dyn_site(bs);
                            }
                            _ => {}
                        }
                    } else {
                        // Ruling AO1: the body is always fitted, after bind.
                        bind(r, &b, &mut sol);
                        let rt = substitute(r, self.opaque(), &sol);
                        match fits(&b, &rt) {
                            None => self.diag("arg-mismatch", bs),
                            Some(true) => {
                                self.dyn_site(bs);
                            }
                            _ => {}
                        }
                    }
                }
            }
        }
        // 4. Bounds.
        for (v, bound) in &sig.bounds {
            let s = sol.get(v).cloned().unwrap_or_default();
            let held = match bound {
                Bound::Ordered => fits(&s, &Ty::float()).or_else(|| fits(&s, &Ty::string())),
                Bound::Number => fits(&s, &Ty::float()),
            };
            match held {
                None => self.diag("arg-mismatch", cs),
                Some(true) => {
                    self.dyn_site(cs);
                }
                _ => {}
            }
        }
        // 5. Fit, through the form.
        if let (Some(rt), Some((rn, _))) = (&recv_text, &recv) {
            let t = substitute(rt, self.opaque(), &sol);
            self.fit_form(*rn, &t, Fit::Arg);
        }
        for (a, pt) in plain {
            let t = substitute(&pt, self.opaque(), &sol);
            self.fit_form(a, &t, Fit::Arg);
        }
        // 6. Result.
        if let (Some(nt), Some(text::Type::List(e))) = (&narrowed, &recv_text) {
            if let text::Type::Var(c) = e.as_ref() {
                sol.insert(*c, nt.clone());
            }
        }
        let result = substitute(&sig.result, self.opaque(), &sol);
        if let Some(class) = site {
            self.site(class, cs);
        }
        self.depth_check(call, result)
    }

    /// 13.8: the type the element parameter has under the body's true facts.
    fn narrow(&mut self, l: NodeId, qs: &[Ty]) -> Ty {
        let elem = qs.first().cloned().unwrap_or_default();
        let NodeKind::Lambda { params, body } = self.kind(l) else {
            return elem;
        };
        let Some(&p) = params.first() else {
            return elem;
        };
        let NodeKind::Param { pattern, .. } = self.kind(p) else {
            return elem;
        };
        let Res::Bind(slot) = self.res[pattern.index()] else {
            return elem;
        };
        let (t, _) = self.facts_of(*body);
        if t.iter().any(|f| f.root == Root::Slot(slot)) {
            elem.nonnull()
        } else {
            elem
        }
    }

    /// The expanded nesting of a node (20.2).
    fn expanded(&self, id: NodeId) -> i32 {
        self.ast.node(id).level as i32 + self.offset
    }

    /// Begin a use of a `const`-bound lambda (13.9, 20.2, 20.3): `None` and
    /// `check-limit` when a limit is passed; else the state to restore.
    fn begin_use(&mut self, l: NodeId, at: NodeId) -> Option<(Vec<Path>, i32)> {
        let lvl = self.ast.node(l).level as i32;
        let exp = self.expanded(at);
        // Ruling AO10: E(U) + 1 + d(L) > 100, where d(L) is the greatest
        // level open inside L less n(L). A use that is not checked is not
        // counted (AO9).
        let d = self.ast.lambda_levels.get(&l.0).map_or(lvl, |m| *m as i32) - lvl;
        if exp + 1 + d > 100 {
            let s = self.span(at);
            self.diag("check-limit", s);
            return None;
        }
        self.body_checks += 1;
        if self.body_checks > 1000 {
            let s = self.span(at);
            self.diag("check-limit", s);
            return None;
        }
        let facts = std::mem::replace(
            &mut self.facts,
            self.lambda_facts.get(&l.0).cloned().unwrap_or_default(),
        );
        let offset = self.offset;
        self.offset = exp + 1 - lvl;
        Some((facts, offset))
    }

    fn end_use(&mut self, saved: (Vec<Path>, i32)) {
        self.facts = saved.0;
        self.offset = saved.1;
    }

    /// Check a lambda against parameter types (13.5 step 3), written in place
    /// or named. Returns the body's type.
    fn check_lambda(&mut self, l: NodeId, named: bool, at: NodeId, qs: &[Ty]) -> Ty {
        let saved = if named {
            match self.begin_use(l, at) {
                Some(s) => Some(s),
                None => return Ty::never(),
            }
        } else {
            None
        };
        let NodeKind::Lambda { params, body } = self.kind(l) else {
            return Ty::never();
        };
        for (i, &p) in params.iter().enumerate() {
            if let NodeKind::Param { pattern, default } = self.kind(p) {
                let a = qs.get(i).cloned().unwrap_or_default();
                let t = match default {
                    Some(d) => {
                        let dt = self.synth(*d);
                        join(&a.nonnull(), &dt)
                    }
                    None => a,
                };
                let s = self.span(*pattern);
                self.bind_pattern(*pattern, t, s);
            }
        }
        let b = self.synth(*body);
        if let Some(s) = saved {
            self.end_use(s);
        }
        b
    }

    /// A call of a `const`-bound lambda by name (13.9).
    fn use_lambda(&mut self, l: NodeId, args: &'a [NodeId], call: NodeId) -> Ty {
        let NodeKind::Lambda { params, body } = self.kind(l) else {
            return Ty::never();
        };
        let required = params
            .iter()
            .rposition(|p| matches!(self.kind(*p), NodeKind::Param { default: None, .. }))
            .map_or(0, |i| i + 1);
        if args.len() < required || args.len() > params.len() {
            self.plain_args(args);
            return Ty::never();
        }
        let mut ats = Vec::new();
        for &a in args {
            ats.push(if self.lambda_of(a).is_some() {
                Ty::never()
            } else {
                self.synth(a)
            });
        }
        let Some(saved) = self.begin_use(l, call) else {
            return Ty::never();
        };
        for (i, &p) in params.iter().enumerate() {
            if let NodeKind::Param { pattern, default } = self.kind(p) {
                let t = match (ats.get(i), default) {
                    (Some(a), Some(d)) => {
                        let dt = self.synth(*d);
                        join(&a.nonnull(), &dt)
                    }
                    (None, Some(d)) => self.synth(*d),
                    (Some(a), None) => a.clone(),
                    (None, None) => Ty::never(),
                };
                let s = self.span(*pattern);
                self.bind_pattern(*pattern, t, s);
            }
        }
        let b = self.synth(*body);
        self.end_use(saved);
        b
    }

    // ---- through the form (15.1) ------------------------------------------------------

    fn report(&mut self, span: Span, at: &Ty, target: &Ty, mode: Fit) {
        match mode {
            Fit::Result => self.diag("result-mismatch", span),
            Fit::Arg => {
                if at.null && fits(&at.nonnull(), target).is_some() {
                    self.diag("maybe-null", span);
                } else {
                    self.diag("arg-mismatch", span);
                }
            }
        }
    }

    fn fit_form(&mut self, e: NodeId, t: &Ty, mode: Fit) {
        let mut e = e;
        loop {
            let et = self.types[e.index()].clone();
            let es = self.span(e);
            if let Some(used) = fits(&et, t) {
                if used {
                    self.dyn_site(es);
                }
                return;
            }
            match self.kind(e) {
                NodeKind::List { items } if t.list.is_some() => {
                    let elem = t.list.as_ref().unwrap().as_ref().clone();
                    for &item in items {
                        if let NodeKind::Spread { value } = self.kind(item) {
                            let vt = self.types[value.index()].clone();
                            let lt = Ty::list(elem.clone());
                            match fits(&vt, &lt) {
                                None => {
                                    let vs = self.span(*value);
                                    self.report(vs, &vt, &lt, mode);
                                }
                                Some(true) => {
                                    let vs = self.span(*value);
                                    self.dyn_site(vs);
                                }
                                _ => {}
                            }
                        } else {
                            self.fit_form(item, &elem, mode);
                        }
                    }
                    return;
                }
                NodeKind::Map { entries } if t.map.is_some() && self.plain_map(entries) => {
                    let r = t.map.as_ref().unwrap().as_ref().clone();
                    let mut seen = Vec::new();
                    for &en in entries {
                        let NodeKind::Entry {
                            key,
                            key_span,
                            value,
                            ..
                        } = self.kind(en)
                        else {
                            continue;
                        };
                        let k = match key {
                            EntryKey::Word(k) | EntryKey::Str(k) => k.clone(),
                            EntryKey::Computed(_) => continue,
                        };
                        match r.fields.get(&k) {
                            Some(f) => self.fit_form(*value, &f.ty, mode),
                            None if r.rest.is_never() => {
                                let vt = self.types[value.index()].clone();
                                self.report(*key_span, &vt, &Ty::never(), mode);
                            }
                            None => self.fit_form(*value, &r.rest, mode),
                        }
                        seen.push(k);
                    }
                    if r.fields
                        .iter()
                        .any(|(k, f)| !f.optional && !seen.contains(k))
                    {
                        self.report(es, &et, t, mode);
                    }
                    return;
                }
                NodeKind::Cond { then, else_, .. } => {
                    self.fit_form(*then, t, mode);
                    e = *else_;
                }
                NodeKind::Match { arms, .. } => {
                    for &a in arms {
                        if let NodeKind::Arm { result, .. } = self.kind(a) {
                            self.fit_form(*result, t, mode);
                        }
                    }
                    return;
                }
                NodeKind::Block { result, .. } => e = *result,
                _ => {
                    self.report(es, &et, t, mode);
                    return;
                }
            }
        }
    }

    fn plain_map(&self, entries: &[NodeId]) -> bool {
        entries.iter().all(|&e| {
            matches!(
                self.kind(e),
                NodeKind::Entry {
                    key: EntryKey::Word(_) | EntryKey::Str(_),
                    ..
                }
            )
        })
    }
}

fn union(mut a: Vec<Path>, b: Vec<Path>) -> Vec<Path> {
    for p in b {
        if !a.contains(&p) {
            a.push(p);
        }
    }
    a
}

fn inter(a: Vec<Path>, b: Vec<Path>) -> Vec<Path> {
    a.into_iter().filter(|p| b.contains(p)).collect()
}

/// The type variables in a type text.
fn vars(t: &text::Type) -> Vec<char> {
    let mut out = Vec::new();
    fn go(t: &text::Type, out: &mut Vec<char>) {
        match t {
            text::Type::Var(c) => {
                if !out.contains(c) {
                    out.push(*c)
                }
            }
            text::Type::List(e) | text::Type::MapOf(e) => go(e, out),
            text::Type::Record(r) => {
                for f in &r.fields {
                    go(&f.ty, out);
                }
                if let text::Rest::Typed(t) = &r.rest {
                    go(t, out);
                }
            }
            text::Type::Union(a) => a.iter().for_each(|t| go(t, out)),
            _ => {}
        }
    }
    go(t, &mut out);
    out
}

fn gain(sol: &mut Solutions, c: char, t: &Ty) {
    let j = match sol.get(&c) {
        Some(old) => join(old, t),
        None => t.clone(),
    };
    sol.insert(c, j);
}

/// bind(P, A) (13.4).
fn bind(p: &text::Type, a: &Ty, sol: &mut Solutions) {
    if a.any {
        for v in vars(p) {
            gain(sol, v, &Ty::any());
        }
        return;
    }
    match p {
        text::Type::Var(c) => gain(sol, *c, a),
        text::Type::Union(alts) if alts.iter().any(|t| matches!(t, text::Type::Null)) => {
            let rest: Vec<text::Type> = alts
                .iter()
                .filter(|t| !matches!(t, text::Type::Null))
                .cloned()
                .collect();
            let p2 = match rest.len() {
                0 => return,
                1 => rest[0].clone(),
                _ => text::Type::Union(rest),
            };
            bind(&p2, &a.nonnull(), sol);
        }
        text::Type::List(e) => {
            if let Some(ae) = &a.list {
                bind(e, ae, sol);
            }
        }
        text::Type::MapOf(e) => {
            if let Some(r) = &a.map {
                let j = join_all(
                    r.fields
                        .values()
                        .map(|f| &f.ty)
                        .chain(std::iter::once(&r.rest)),
                );
                bind(e, &j, sol);
            }
        }
        text::Type::Record(pr) => {
            if let Some(r) = &a.map {
                for f in &pr.fields {
                    if let Some(af) = r.fields.get(&f.key) {
                        bind(&f.ty, &af.ty, sol);
                    }
                }
            }
        }
        _ => {}
    }
}

/// Which `Cond` form a node has, for callers that care.
pub fn cond_form(ast: &Ast, id: NodeId) -> Option<CondForm> {
    match ast.kind(id) {
        NodeKind::Cond { form, .. } => Some(*form),
        _ => None,
    }
}
