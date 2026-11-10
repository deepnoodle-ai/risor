//! Compile: name resolution and the static rules (`05-types.md` 22).
//!
//! One walk over the tree resolves every name and every call, gives every
//! binding a fixed slot (`02-evaluation.md` 11.1: no lambda is entered while
//! an earlier call of it is in progress, so each binding may have one fixed
//! place for the whole run), computes the depth measure of each call (8.4),
//! and reports the static diagnostics.
//!
//! What opens no level is walked by a loop, so the walk's recursion is
//! bounded by the source nesting.

use crate::check::ty::{join_into, Ty, TypeNames};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::env::Env;
use crate::library;
use crate::source::Span;
use crate::syntax::ast::{EntryKey, Lit, NodeId, NodeKind, PPrim, PType};
use crate::syntax::Parsed;
use std::collections::{HashMap, HashSet};

/// What compile found for a node.
#[derive(Clone, Debug, Default, PartialEq)]
pub enum Res {
    #[default]
    None,
    /// An `Ident` that reads a slot.
    Slot(u32),
    /// An `Ident` that reads a variable of the environment.
    Var(u32),
    /// An `Ident` that is a lambda argument: the name of a `const`-bound
    /// lambda.
    LambdaRef(NodeId),
    /// A call of a `const`-bound lambda.
    CallLambda(NodeId),
    /// A call of a library function.
    CallLib(u32),
    /// A call of a host function, by name or by namespace.
    CallHost(u32),
    /// A `NamePat` that binds a slot.
    Bind(u32),
    /// The slots of a map pattern's fields and rest, or a list pattern's
    /// rest.
    PatSlots {
        fields: Vec<Option<u32>>,
        rest: Option<u32>,
    },
    /// The slot of a `catch (name)`.
    Catch(Option<u32>),
    /// F4: an `As` node's program type, resolved (3.5); `None` when a static
    /// rule of 6.1 refused it.
    As(Option<Box<Ty>>),
}

/// A compiled program. It does not change, and is safe to share between
/// threads and to run many times.
pub struct Program {
    pub parsed: Parsed,
    pub env: Env,
    pub res: Vec<Res>,
    /// For each `Call` and `MethodCall`: 1 plus its nesting depth within the
    /// body that holds it (8.4).
    pub measure: Vec<u16>,
    pub slots: usize,
    /// For a program compiled with a check: its sites, which decide the
    /// class of a run failure (`05-types.md` 18.5).
    pub sites: Option<Vec<crate::check::Site>>,
}

#[derive(Clone, Copy, Debug)]
enum Bind {
    Slot(u32),
    Var(u32),
    Lambda(NodeId),
}

/// Compile a parsed program against an environment, with a check: the
/// default (`03-host.md` 3.2.1; rulings round 11 AP7). It fails exactly when
/// the check reports a diagnostic. `compile_unchecked` is the explicit
/// option without the type rules, and `check::check` the check alone.
pub fn compile(env: &Env, parsed: Parsed) -> Result<Program, Error> {
    crate::check::compile_checked(env, parsed, &crate::check::Expected::Nothing).map(|(p, _)| p)
}

/// Compile with the static rules and no type rules (`03-host.md` 3.2.1
/// item 4). A run reports classes by the column "not checked".
pub fn compile_unchecked(env: &Env, parsed: Parsed) -> Result<Program, Error> {
    let missing: Vec<Diagnostic> = env
        .data()
        .functions
        .iter()
        .filter(|f| f.imp.is_none())
        .map(|f| f.name.clone())
        .chain(
            env.data()
                .methods
                .iter()
                .filter(|m| m.imp.is_none())
                .map(|m| format!("{}.{}", m.type_name, m.name)),
        )
        .map(|n| {
            Diagnostic::bare(
                Phase::Declare,
                code::NO_IMPLEMENTATION,
                format!("{n} has no implementation"),
            )
        })
        .collect();
    if !missing.is_empty() {
        return Err(Error {
            phase: Phase::Declare,
            diagnostics: missing,
        });
    }
    let r = resolve(env, &parsed);
    if !r.diags.is_empty() {
        return Err(Error {
            phase: Phase::Check,
            diagnostics: r.diags,
        });
    }
    Ok(Program {
        env: env.clone(),
        parsed,
        res: r.res,
        measure: r.measure,
        slots: r.slots,
        sites: None,
    })
}

/// What resolution found: each node's resolution, each call's depth
/// measure, the number of slots, and the static diagnostics in the order of
/// 22.10.
pub struct Resolution {
    pub res: Vec<Res>,
    pub measure: Vec<u16>,
    pub slots: usize,
    pub diags: Vec<Diagnostic>,
}

/// Resolve every name of a program, and apply the static rules.
pub fn resolve(env: &Env, parsed: &Parsed) -> Resolution {
    let n = parsed.ast.nodes.len();
    let mut vars = HashMap::new();
    for (i, v) in env.data().vars.iter().enumerate() {
        vars.insert(v.clone(), Bind::Var(i as u32));
    }
    let mut r = Resolver {
        env,
        p: parsed,
        scopes: vec![vars, HashMap::new()],
        res: vec![Res::None; n],
        measure: vec![0; n],
        diags: Vec::new(),
        slots: 0,
        lambda_levels: Vec::new(),
    };
    let root = parsed.ast.root;
    if let NodeKind::Program { consts, result } = parsed.ast.kind(root) {
        for &c in consts {
            r.const_line(c);
        }
        r.walk(*result);
    }
    let mut d = std::mem::take(&mut r.diags);
    sort_diagnostics(&mut d);
    Resolution {
        res: r.res,
        measure: r.measure,
        slots: r.slots as usize,
        diags: d,
    }
}

/// Order a list of `check` diagnostics by start, end, then code, with
/// duplicates removed (22.10, 6.1).
pub fn sort_diagnostics(d: &mut Vec<Diagnostic>) {
    d.sort_by(|a, b| {
        let ka = a.range.unwrap_or_default();
        let kb = b.range.unwrap_or_default();
        (ka.start, ka.end, a.code).cmp(&(kb.start, kb.end, b.code))
    });
    d.dedup_by(|a, b| a.code == b.code && a.range == b.range);
}

struct Resolver<'a> {
    env: &'a Env,
    p: &'a Parsed,
    scopes: Vec<HashMap<String, Bind>>,
    res: Vec<Res>,
    measure: Vec<u16>,
    diags: Vec<Diagnostic>,
    slots: u32,
    lambda_levels: Vec<u16>,
}

/// The counts of a callee (22.3): required, at most, and for each argument
/// whether its parameter is a callback of `n` parameters.
struct Counts {
    required: usize,
    at_most: Option<usize>,
    fn_params: Vec<Option<usize>>,
    rest_fn: Option<Option<usize>>,
}

impl Counts {
    fn param(&self, i: usize) -> Option<Option<usize>> {
        self.fn_params.get(i).copied().or(self.rest_fn)
    }
}

enum Callee {
    Unknown,
    Lambda(NodeId),
    Lib(usize),
    Host(usize),
}

impl Resolver<'_> {
    fn kind(&self, id: NodeId) -> &NodeKind {
        self.p.ast.kind(id)
    }

    fn span(&self, id: NodeId) -> Span {
        self.p.ast.span(id)
    }

    fn diag(&mut self, c: &'static str, span: Span) {
        let msg = format!("{c}: {}", self.p.source.slice(span));
        self.diags
            .push(Diagnostic::at(Phase::Check, c, &self.p.source, span, msg));
    }

    /// F4 3.5 and 6.1: resolve a program type. Names first; then two records
    /// in one `PType`; then depth and size, one `check-limit` for both.
    fn program_type(&mut self, pt: &PType) -> Option<Ty> {
        let mut undefined = Vec::new();
        names_of(pt, &mut |n, span| {
            if self.env.data().names.lookup(n).is_none() {
                undefined.push(span);
            }
        });
        if !undefined.is_empty() {
            for s in undefined {
                self.diag("undefined-name", s);
            }
            return None;
        }
        let mut bad = false;
        let t = self.resolve_ptype(pt, &mut bad);
        if bad {
            return None;
        }
        if t.too_big() || t.depth() > 64 {
            self.diag("check-limit", pt.span);
            return None;
        }
        Some(t)
    }

    /// One `PType`: the Join of its alternatives, with parentheses removed.
    /// Two or more of them that are records is `untagged-union` here, and
    /// the `PType` then counts as `never` for the one around it.
    fn resolve_ptype(&mut self, pt: &PType, bad: &mut bool) -> Ty {
        let mut alts: Vec<Ty> = Vec::new();
        for a in &pt.alts {
            let mut t = self.resolve_pprim(&a.prim, bad);
            for _ in 0..a.suffixes {
                t = Ty::list(t);
            }
            alts.push(t);
        }
        if alts.iter().filter(|t| t.map.is_some()).count() >= 2 {
            self.diag("untagged-union", pt.span);
            *bad = true;
            return Ty::never();
        }
        let mut acc = Ty::never();
        for t in &alts {
            join_into(&mut acc, t);
        }
        acc
    }

    fn resolve_pprim(&mut self, p: &PPrim, bad: &mut bool) -> Ty {
        match p {
            PPrim::Null => Ty::null(),
            PPrim::Bool => Ty::boolean(),
            PPrim::Int => Ty::int(),
            PPrim::Float => Ty::float(),
            PPrim::String => Ty::string(),
            PPrim::Any => Ty::any(),
            PPrim::Never => Ty::never(),
            PPrim::Lit(s) => Ty::lit(s),
            PPrim::Map(inner) => Ty::map_of(self.resolve_ptype(inner, bad)),
            PPrim::Record { fields, rest } => {
                let mut fs = std::collections::BTreeMap::new();
                for f in fields {
                    let ty = self.resolve_ptype(&f.ty, bad);
                    fs.insert(
                        f.key.clone(),
                        crate::check::ty::Field {
                            ty,
                            optional: f.optional,
                        },
                    );
                }
                let rest = match rest {
                    None => Ty::never(),
                    Some(None) => Ty::any(),
                    Some(Some(t)) => self.resolve_ptype(t, bad),
                };
                Ty::rec(crate::check::ty::Rec::new(fs, rest))
            }
            PPrim::Name(n, _) => self.env.data().names.lookup(n).unwrap_or_default(),
            PPrim::Paren(inner) => self.resolve_paren(inner, bad),
        }
    }

    /// A parenthesized `PType` alternative: its own alternatives count for
    /// the records of the `PType` around it only through its Join.
    fn resolve_paren(&mut self, inner: &PType, bad: &mut bool) -> Ty {
        self.resolve_ptype(inner, bad)
    }

    fn lookup(&self, name: &str) -> Option<Bind> {
        self.scopes.iter().rev().find_map(|s| s.get(name).copied())
    }

    fn is_function_name(&self, name: &str) -> bool {
        self.env.lib_function(name).is_some()
            || self
                .env
                .function(name)
                .is_some_and(|i| self.env.data().functions[i].namespace.is_none())
    }

    fn set_measure(&mut self, id: NodeId) {
        let level = self.p.ast.node(id).level;
        let base = self.lambda_levels.last().copied().unwrap_or(0);
        self.measure[id.index()] = 1 + level.saturating_sub(base);
    }

    /// Bind a name in the innermost scope (22.7).
    fn bind(&mut self, name: &str, span: Span, b: Option<Bind>) -> Option<u32> {
        if name == "_" {
            return None;
        }
        if self.env.is_namespace(name) {
            self.diag(code::STATIC[6], span); // shadows-namespace
            return None;
        }
        let scope = self.scopes.len() - 1;
        if self.scopes[scope].contains_key(name) {
            self.diag(code::STATIC[5], span); // duplicate-name
        }
        let (bind, slot) = match b {
            Some(b) => (b, None),
            None => {
                let s = self.slots;
                self.slots += 1;
                (Bind::Slot(s), Some(s))
            }
        };
        self.scopes[scope].insert(name.to_string(), bind);
        slot
    }

    fn const_line(&mut self, id: NodeId) {
        let NodeKind::Const { pattern, value } = self.kind(id).clone() else {
            return;
        };
        if let (NodeKind::Lambda { .. }, NodeKind::NamePat { name: Some(n) }) =
            (self.kind(value), self.kind(pattern))
        {
            let n = n.clone();
            self.walk(value);
            let span = self.span(pattern);
            self.bind(&n, span, Some(Bind::Lambda(value)));
            return;
        }
        self.walk(value);
        self.pattern(pattern);
    }

    fn pattern(&mut self, id: NodeId) {
        match self.kind(id).clone() {
            NodeKind::NamePat { name: Some(n) } => {
                let span = self.span(id);
                if let Some(s) = self.bind(&n, span, None) {
                    self.res[id.index()] = Res::Bind(s);
                }
            }
            NodeKind::MapPat { fields, rest } => {
                let mut slots = Vec::new();
                for f in &fields {
                    if let Some(d) = f.default {
                        self.walk(d);
                    }
                    slots.push(self.bind(&f.name.name, f.name.span, None));
                }
                let rest = rest.and_then(|r| self.bind(&r.name, r.span, None));
                self.res[id.index()] = Res::PatSlots {
                    fields: slots,
                    rest,
                };
            }
            NodeKind::ListPat { elems, rest } => {
                for e in &elems {
                    if let Some(d) = e.default {
                        self.walk(d);
                    }
                    self.pattern(e.pat);
                }
                let rest = rest.and_then(|r| self.bind(&r.name, r.span, None));
                self.res[id.index()] = Res::PatSlots {
                    fields: Vec::new(),
                    rest,
                };
            }
            _ => {}
        }
    }

    /// Walk the nodes of a scope: `const` lines, then a result.
    fn scope_body(&mut self, consts: &[NodeId], result: NodeId) {
        for &c in consts {
            self.const_line(c);
        }
        self.walk(result);
    }

    fn walk(&mut self, mut id: NodeId) {
        loop {
            match self.kind(id).clone() {
                NodeKind::Program { consts, result } | NodeKind::Block { consts, result } => {
                    self.scopes.push(HashMap::new());
                    self.scope_body(&consts, result);
                    self.scopes.pop();
                    return;
                }
                NodeKind::Ident { name } => {
                    self.ident(id, &name);
                    return;
                }
                NodeKind::Template { parts } => {
                    for p in parts {
                        if let crate::syntax::ast::TemplatePart::Expr(e) = p {
                            self.walk(e);
                        }
                    }
                    return;
                }
                NodeKind::List { items } => {
                    for i in items {
                        self.walk(i);
                    }
                    return;
                }
                NodeKind::Map { entries } => {
                    self.map(&entries);
                    return;
                }
                NodeKind::Entry { key, value, .. } => {
                    if let EntryKey::Computed(k) = key {
                        self.walk(k);
                    }
                    id = value;
                }
                NodeKind::Spread { value } => id = value,
                NodeKind::Unary { x, .. } => id = x,
                NodeKind::Binary { .. } => {
                    let mut rights = Vec::new();
                    let mut n = id;
                    while let NodeKind::Binary { x, y, .. } = self.kind(n) {
                        rights.push(*y);
                        n = *x;
                    }
                    self.walk(n);
                    for y in rights.into_iter().rev() {
                        self.walk(y);
                    }
                    return;
                }
                NodeKind::Cond {
                    test, then, else_, ..
                } => {
                    self.walk(test);
                    self.walk(then);
                    id = else_;
                }
                NodeKind::OptChain { x } => id = x,
                NodeKind::Member { .. } | NodeKind::Index { .. } | NodeKind::MethodCall { .. } => {
                    self.chain(id);
                    return;
                }
                NodeKind::As { x, ty, .. } => {
                    let r = self.program_type(&ty);
                    self.res[id.index()] = Res::As(r.map(Box::new));
                    id = x;
                }
                NodeKind::Call { name, args, .. } => {
                    let span = self.span(id);
                    self.call(id, None, &name.name, name.span, &args, span);
                    return;
                }
                NodeKind::Lambda { params, body } => {
                    self.lambda_levels.push(self.p.ast.node(id).level);
                    self.scopes.push(HashMap::new());
                    for p in params {
                        if let NodeKind::Param { pattern, default } = self.kind(p).clone() {
                            if let Some(d) = default {
                                self.walk(d);
                            }
                            self.pattern(pattern);
                        }
                    }
                    self.walk(body);
                    self.scopes.pop();
                    self.lambda_levels.pop();
                    return;
                }
                NodeKind::Match { subject, arms } => {
                    self.walk(subject);
                    self.arms(&arms);
                    return;
                }
                NodeKind::Try {
                    body,
                    name,
                    handler,
                } => {
                    self.walk(body);
                    self.scopes.push(HashMap::new());
                    let slot = name.and_then(|n| self.bind(&n.name, n.span, None));
                    self.res[id.index()] = Res::Catch(slot);
                    match self.kind(handler).clone() {
                        NodeKind::Block { consts, result } => self.scope_body(&consts, result),
                        _ => self.walk(handler),
                    }
                    self.scopes.pop();
                    return;
                }
                _ => return,
            }
        }
    }

    fn ident(&mut self, id: NodeId, name: &str) {
        let span = self.span(id);
        match self.lookup(name) {
            Some(Bind::Slot(s)) => self.res[id.index()] = Res::Slot(s),
            Some(Bind::Var(v)) => self.res[id.index()] = Res::Var(v),
            Some(Bind::Lambda(_)) => self.diag(code::STATIC[2], span), // not-a-value
            None => {
                if self.env.is_namespace(name) || self.is_function_name(name) {
                    self.diag(code::STATIC[2], span);
                } else {
                    self.diag(code::STATIC[0], span); // undefined-name
                }
            }
        }
    }

    /// The steps of a chain, walked by a loop.
    fn chain(&mut self, top: NodeId) {
        let mut steps = Vec::new();
        let mut n = top;
        loop {
            match self.kind(n) {
                NodeKind::Member { x, .. } | NodeKind::Index { x, .. } => {
                    steps.push(n);
                    n = *x;
                }
                NodeKind::MethodCall { x, optional, .. } => {
                    if !*optional && self.namespace_receiver(*x) {
                        break;
                    }
                    steps.push(n);
                    n = *x;
                }
                NodeKind::OptChain { x } => n = *x,
                _ => break,
            }
        }
        // `n` is the primary, or a call of a namespace.
        match self.kind(n).clone() {
            NodeKind::MethodCall { x, name, args, .. }
                if matches!(
                    self.kind(n),
                    NodeKind::MethodCall {
                        optional: false,
                        ..
                    }
                ) && self.namespace_receiver(x) =>
            {
                let NodeKind::Ident { name: ns } = self.kind(x).clone() else {
                    unreachable!()
                };
                let span = self.span(n);
                let ns_span = self.span(x);
                self.call(n, Some((ns, ns_span)), &name.name, name.span, &args, span);
            }
            _ => self.walk(n),
        }
        for s in steps.into_iter().rev() {
            match self.kind(s).clone() {
                NodeKind::Index { index, .. } => self.walk(index),
                NodeKind::MethodCall { args, .. } => {
                    self.set_measure(s);
                    self.args(&args);
                }
                _ => {}
            }
        }
    }

    /// Whether a receiver is an `Ident` that names a namespace (5.4).
    fn namespace_receiver(&self, x: NodeId) -> bool {
        match self.kind(x) {
            NodeKind::Ident { name } => self.lookup(name).is_none() && self.env.is_namespace(name),
            _ => false,
        }
    }

    /// The lambda of a lambda argument: a `Lambda` node, or the name of a
    /// `const`-bound lambda.
    fn lambda_arg(&self, a: NodeId) -> Option<NodeId> {
        match self.kind(a) {
            NodeKind::Lambda { .. } => Some(a),
            NodeKind::Ident { name } => match self.lookup(name) {
                Some(Bind::Lambda(l)) => Some(l),
                _ => None,
            },
            _ => None,
        }
    }

    fn lambda_params(&self, l: NodeId) -> usize {
        match self.kind(l) {
            NodeKind::Lambda { params, .. } => params.len(),
            _ => 0,
        }
    }

    /// Walk the arguments of a call (22.5, last paragraph).
    fn args(&mut self, args: &[NodeId]) {
        for &a in args {
            match self.kind(a) {
                NodeKind::Ident { .. } => match self.lambda_arg(a) {
                    Some(l) => self.res[a.index()] = Res::LambdaRef(l),
                    None => self.walk(a),
                },
                _ => self.walk(a),
            }
        }
    }

    fn call(
        &mut self,
        id: NodeId,
        ns: Option<(String, Span)>,
        name: &str,
        name_span: Span,
        args: &[NodeId],
        call_span: Span,
    ) {
        self.set_measure(id);
        let callee = match &ns {
            Some((n, s)) => match self.env.function(&format!("{n}.{name}")) {
                Some(i) => Callee::Host(i),
                None => {
                    self.diag(code::STATIC[1], s.to(name_span)); // undefined-function
                    Callee::Unknown
                }
            },
            None => match self.lookup(name) {
                Some(Bind::Lambda(l)) => Callee::Lambda(l),
                Some(_) => {
                    self.diag(code::STATIC[3], name_span); // call-of-parameter
                    Callee::Unknown
                }
                None => {
                    if let Some(i) = self
                        .env
                        .function(name)
                        .filter(|&i| self.env.data().functions[i].namespace.is_none())
                    {
                        Callee::Host(i)
                    } else if let Some(i) = self.env.lib_function(name) {
                        Callee::Lib(i)
                    } else {
                        self.diag(code::STATIC[1], name_span);
                        Callee::Unknown
                    }
                }
            },
        };
        let counts = match &callee {
            Callee::Unknown => None,
            Callee::Lambda(l) => {
                let NodeKind::Lambda { params, .. } = self.kind(*l) else {
                    unreachable!()
                };
                // Ruling AC10: up to the last parameter with no default.
                let required = params
                    .iter()
                    .rposition(|p| matches!(self.kind(*p), NodeKind::Param { default: None, .. }))
                    .map_or(0, |i| i + 1);
                Some(Counts {
                    required,
                    at_most: Some(params.len()),
                    fn_params: vec![None; params.len()],
                    rest_fn: None,
                })
            }
            Callee::Lib(i) => Some(sig_counts(&library::records()[*i].sig)),
            Callee::Host(i) => Some(sig_counts(&self.env.data().functions[*i].sig)),
        };
        if let Some(c) = counts {
            let n = args.len();
            if n < c.required || c.at_most.is_some_and(|m| n > m) {
                self.diag(code::STATIC[8], call_span); // arity
            } else {
                for (i, &a) in args.iter().enumerate() {
                    let Some(l) = self.lambda_arg(a) else {
                        continue;
                    };
                    let span = self.span(a);
                    match &callee {
                        Callee::Lambda(_) => self.diag(code::STATIC[4], span), // lambda-to-lambda
                        _ => match c.param(i).flatten() {
                            None => self.diag(code::STATIC[2], span),
                            Some(k) => {
                                if self.lambda_params(l) > k {
                                    self.diag(code::STATIC[8], span);
                                }
                            }
                        },
                    }
                }
            }
        }
        self.res[id.index()] = match callee {
            Callee::Unknown => Res::None,
            Callee::Lambda(l) => Res::CallLambda(l),
            Callee::Lib(i) => Res::CallLib(i as u32),
            Callee::Host(i) => Res::CallHost(i as u32),
        };
        self.args(args);
    }

    fn map(&mut self, entries: &[NodeId]) {
        let mut seen = HashSet::new();
        for &e in entries {
            if let NodeKind::Entry { key, key_span, .. } = self.kind(e).clone() {
                let k = match key {
                    EntryKey::Word(w) => Some(w),
                    EntryKey::Str(s) => Some(s),
                    EntryKey::Computed(_) => None,
                };
                if let Some(k) = k {
                    if !seen.insert(k) {
                        self.diag(code::STATIC[7], key_span); // duplicate-key
                    }
                }
            }
            self.walk(e);
        }
    }

    fn arms(&mut self, arms: &[NodeId]) {
        let mut seen: HashSet<LitKey> = HashSet::new();
        let mut after_default = false;
        for &a in arms {
            let NodeKind::Arm { literals, result } = self.kind(a).clone() else {
                continue;
            };
            if after_default {
                let span = self.span(a);
                self.diag(code::STATIC[10], span); // unreachable-arm
            }
            match literals {
                None => after_default = true,
                Some(lits) => {
                    for l in lits {
                        if !seen.insert(lit_key(&l.lit)) {
                            self.diag(code::STATIC[9], l.span); // duplicate-case
                        }
                    }
                }
            }
            self.walk(result);
        }
    }
}

/// A literal of a `match` arm as a key equal by `==` (`01-language.md` 6.7):
/// `1` and `1.0` are one key, and so are `0` and `-0`. A set of keys finds a
/// literal listed twice in n log n work (ruling CG10).
#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub(crate) enum LitKey {
    Num(u64),
    Str(String),
    Bool(bool),
    Null,
}

pub(crate) fn lit_key(l: &Lit) -> LitKey {
    match l {
        Lit::Number(x) => LitKey::Num(if *x == 0.0 {
            0.0f64.to_bits()
        } else {
            x.to_bits()
        }),
        Lit::Str(s) => LitKey::Str(s.clone()),
        Lit::Bool(b) => LitKey::Bool(*b),
        Lit::Null => LitKey::Null,
    }
}

/// Every type name of a program type, with its span, in order.
fn names_of(pt: &PType, f: &mut dyn FnMut(&str, Span)) {
    for a in &pt.alts {
        match &a.prim {
            PPrim::Name(n, s) => f(n, *s),
            PPrim::Map(inner) | PPrim::Paren(inner) => names_of(inner, f),
            PPrim::Record { fields, rest } => {
                for fl in fields {
                    names_of(&fl.ty, f);
                }
                if let Some(Some(r)) = rest {
                    names_of(r, f);
                }
            }
            _ => {}
        }
    }
}

fn sig_counts(sig: &crate::types::text::Sig) -> Counts {
    use crate::types::text::ParamType;
    let fnp = |p: &crate::types::text::Param| match &p.ty {
        ParamType::Fn(f) => Some(f.params.len()),
        _ => None,
    };
    let rest_fn = sig.params.last().filter(|p| p.rest).map(fnp);
    Counts {
        required: sig.required(),
        at_most: sig.at_most(),
        fn_params: sig.params.iter().map(fnp).collect(),
        rest_fn,
    }
}
