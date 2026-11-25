//! Compile: name resolution and the static rules (`05-types.md` 22).
//!
//! One walk over the tree resolves every name and every call, gives every
//! binding a slot and the place a run keeps it in (a frame for each call,
//! F17 11.5: a typed lambda may call itself, so a binding has no fixed place
//! for the whole run), computes the depth measure of each call (8.4), and
//! reports the static diagnostics.
//!
//! What opens no level is walked by a loop: the left operands of operators,
//! the value of an `as` or `is`, the steps of a chain with its projections,
//! the `else` of a conditional. So the walk's recursion is bounded by the
//! source nesting.

use crate::check::ty::{Ty, TypeNames};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::env::Env;
use crate::library;
use crate::source::Range;
use crate::syntax::ast::{EntryKey, Lit, NodeId, NodeKind, PPrim, PType, TypeLine};
use crate::syntax::Expr;
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
    /// F6: a `Member` that reads a constant of a group (`f06-groups.md` 2.4).
    Constant(f64),
    /// F4: an `As` node's program type, resolved (3.5); `None` when a static
    /// rule of 6.1 refused it.
    As(Option<Box<Ty>>),
    /// A reference of a `const`-bound lambda to its own name, in its body or
    /// defaults: a `Call` or an `Ident` lambda argument. It is
    /// `recursion-needs-types` (`05-types.md` 22.5), and no use (13.9).
    SelfRef(NodeId),
    /// An `Ident` that names a `const`-bound lambda where a value is needed:
    /// `not-a-value`, no use, and read only by hover (F12 3.13, R245).
    NotValue(NodeId),
    /// F17 13.1: an `Ident` whose binding is a function binding and that is
    /// not called: a lambda argument, or a read, which is `not-a-value`
    /// (13.2 rule 4). The slot of the function parameter, and its `Param`.
    FnRef(u32, NodeId),
    /// F17 13.2 rule 1: a `Call` of a function binding: the slot of the
    /// function parameter, and its `Param`.
    CallFn(u32, NodeId),
}

/// F16 7.3: a type name in a program type, or the name of a type line, with
/// the type it stands for there and the span of the name in its type line
/// (`None` for a type of the environment).
#[derive(Clone, Debug)]
pub struct TypeRef {
    pub span: Range,
    pub ty: Ty,
    pub declaration: Option<Range>,
}

/// A compiled program. It does not change, and is safe to share between
/// threads and to run many times.
pub struct Program {
    pub(crate) parsed: Expr,
    pub(crate) env: Env,
    pub(crate) res: Vec<Res>,
    /// For each `Call` and `MethodCall`: 1 plus its nesting depth within the
    /// body that holds it (8.4).
    pub(crate) measure: Vec<u16>,
    /// Where a run keeps each slot (F17 11.5).
    pub(crate) place: Vec<Place>,
    /// The frame of each lambda, by node; the program's, at the root.
    pub(crate) frames: Vec<Frame>,
    /// The depth of the deepest frame.
    pub(crate) max_depth: usize,
    /// For a program compiled with a check: its sites, which decide the
    /// class of a run failure (`05-types.md` 18.5).
    pub(crate) sites: Option<Vec<crate::check::Site>>,
    /// For a program compiled with a check: its type.
    pub(crate) ty: Option<Ty>,
    /// The compiled form of the hot node kinds (`eval/code.rs`).
    pub(crate) code: Vec<Option<crate::eval::code::Code>>,
    /// The slots of each lambda's parameters, when all are plain names.
    pub(crate) params: Vec<Option<Box<[Option<u32>]>>>,
    /// For each node, whether it is the last step of a coalesced read under
    /// `??` (`01-language.md` 6.6).
    pub(crate) coalesced: Vec<bool>,
    /// For each variable of the environment, its place among the variables
    /// the program reads (`u32::MAX` for one it never reads); and their
    /// count.
    pub(crate) var_index: Vec<u32>,
    pub(crate) vars_read: usize,
    /// F17 12: the tables of the program's recursive types, which the types
    /// of its conversions reach.
    #[allow(dead_code)]
    pub(crate) tables: Vec<std::sync::Arc<crate::check::ty::Table>>,
    /// F17 13.1: for each node, whether it is a higher-order lambda: a
    /// `Lambda` with a function parameter. A call of one binds lambdas.
    pub(crate) higher: Vec<bool>,
}

#[derive(Clone, Copy, Debug)]
enum Bind {
    Slot(u32),
    Var(u32),
    Lambda(NodeId),
    /// F17 13.1: a function binding: the slot of a function parameter, and
    /// its `Param`.
    Fn(u32, NodeId),
}

/// F17 13.1: the listed count of a function parameter's function type.
pub(crate) fn listed_count(ast: &crate::syntax::ast::Ast, p: NodeId) -> usize {
    match ast.kind(p) {
        NodeKind::Param { fn_: Some(f), .. } => f.params.len(),
        _ => 0,
    }
}

/// F17 13.1: whether a node is a higher-order lambda, one that has a
/// function parameter.
pub(crate) fn is_higher_order(ast: &crate::syntax::ast::Ast, l: NodeId) -> bool {
    match ast.kind(l) {
        NodeKind::Lambda { params, .. } => params
            .iter()
            .any(|p| matches!(ast.kind(*p), NodeKind::Param { fn_: Some(_), .. })),
        _ => false,
    }
}

/// Compile with the static rules and no type rules (`03-host.md` 3.2.1
/// item 4). A run reports classes by the column "not checked".
pub(crate) fn compile_unchecked(env: &Env, parsed: Expr) -> Result<Program, Error> {
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
    // The variables the program reads, numbered densely in the order they
    // are first written, so that a run holds a place only for those.
    let mut var_index = vec![u32::MAX; env.data().vars.len()];
    let mut vars_read = 0u32;
    for x in &r.res {
        if let Res::Var(v) = x {
            if let Some(d) = var_index.get_mut(*v as usize) {
                if *d == u32::MAX {
                    *d = vars_read;
                    vars_read += 1;
                }
            }
        }
    }
    let coalesced = parsed.ast.coalesced_reads();
    let code = crate::eval::code::build(&parsed.ast, &r.res, &r.place, &var_index, &coalesced);
    let params = crate::eval::code::params(&parsed.ast, &r.res, &r.place);
    let max_depth = r.frames.iter().map(|f| f.depth as usize).max().unwrap_or(0);
    let higher = (0..parsed.ast.nodes.len())
        .map(|i| is_higher_order(&parsed.ast, NodeId(i as u32)))
        .collect();
    Ok(Program {
        env: env.clone(),
        code,
        params,
        coalesced,
        var_index,
        vars_read: vars_read as usize,
        higher,
        parsed,
        res: r.res,
        measure: r.measure,
        place: r.place,
        frames: r.frames,
        max_depth,
        sites: None,
        ty: None,
        tables: r.tables,
    })
}

/// Where a run keeps a slot (F17 11.5): in the frame of depth `depth` on the
/// chain of the code that reads it, at index `index`. The program's frame
/// has depth 0, and a lambda's frame is one deeper than the frame of the
/// code in which the lambda is written.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Place {
    pub depth: u16,
    pub index: u32,
}

/// A frame: its depth, and the number of slots it holds.
#[derive(Clone, Copy, Debug, Default)]
pub struct Frame {
    pub depth: u16,
    pub size: u32,
}

/// What resolution found: each node's resolution, each call's depth
/// measure, where each slot is kept, and the static diagnostics in the order
/// of 22.10.
pub struct Resolution {
    pub res: Vec<Res>,
    pub measure: Vec<u16>,
    /// Where a run keeps each slot (F17 11.5): the frame's depth and the
    /// slot's index in it.
    pub place: Vec<Place>,
    /// The frame of each node that is a lambda; for the root, the program's
    /// frame.
    pub frames: Vec<Frame>,
    pub diags: Vec<Diagnostic>,
    /// F16: each annotation's program type, by the start of its span:
    /// resolved, or `None` when a static rule refused it (3.3 item 2).
    pub annots: HashMap<u32, Option<Ty>>,
    /// F16 7.3: the type names that resolved, and the names of the type
    /// lines that declare.
    pub type_refs: Vec<TypeRef>,
    /// F17 12: the tables of the program's recursive types, which the types
    /// of a check reach.
    pub tables: Vec<std::sync::Arc<crate::check::ty::Table>>,
    /// F17 13.2 item 4: the slots of the parameters written as a name with
    /// no default and no function type, whose hints name the fix.
    pub plain_params: HashSet<u32>,
}

/// Resolve every name of a program, and apply the static rules.
pub fn resolve(env: &Env, parsed: &Expr) -> Resolution {
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
        type_lines: HashMap::new(),
        type_line_names: HashSet::new(),
        walking: Vec::new(),
        self_refs: HashMap::new(),
        annots: HashMap::new(),
        type_refs: Vec::new(),
        tables: Vec::new(),
        open: vec![0],
        place: Vec::new(),
        frames: vec![Frame::default(); n],
        plain_params: HashSet::new(),
        follow: None,
    };
    let root = parsed.ast.root;
    if let NodeKind::Program {
        consts,
        result,
        types,
    } = parsed.ast.kind(root)
    {
        // F17 12.3 item 2: the type lines are resolved as one list first.
        let declared = r.type_lines_list(types);
        // F16 3.1 item 1: the type lines and the `const` lines in the order
        // of the text, then the result.
        let mut lines: Vec<(u32, Result<NodeId, usize>)> = consts
            .iter()
            .map(|&c| (parsed.ast.span(c).start, Ok(c)))
            .chain(
                types
                    .iter()
                    .enumerate()
                    .map(|(i, t)| (t.span.start, Err(i))),
            )
            .collect();
        lines.sort_by_key(|(s, _)| *s);
        for (_, l) in lines {
            match l {
                Ok(c) => r.const_line(c),
                Err(i) => r.type_line(&types[i], declared[i].clone()),
            }
        }
        r.walk(*result);
    }
    // 22.5: once for each lambda, on its reference with the least start.
    let mut refs: Vec<(Range, String)> = std::mem::take(&mut r.self_refs).into_values().collect();
    refs.sort_by_key(|(s, _)| s.start);
    for (span, name) in refs {
        r.diag(code::STATIC[11], span, "default", &[("name", name)]);
    }
    let mut d = std::mem::take(&mut r.diags);
    sort_diagnostics(&mut d);
    if let Some(f) = r.frames.get_mut(root.index()) {
        *f = Frame {
            depth: 0,
            size: r.open[0],
        };
    }
    Resolution {
        res: r.res,
        measure: r.measure,
        place: r.place,
        frames: r.frames,
        diags: d,
        annots: r.annots,
        type_refs: r.type_refs,
        tables: r.tables,
        plain_params: r.plain_params,
    }
}

/// Order a list of `check` diagnostics by start, end, then code, with
/// duplicates removed (22.10, 6.1).
pub fn sort_diagnostics(d: &mut Vec<Diagnostic>) {
    d.sort_by(|a, b| {
        let ka = a.range().unwrap_or_default();
        let kb = b.range().unwrap_or_default();
        (ka.start, ka.end, a.code).cmp(&(kb.start, kb.end, b.code))
    });
    d.dedup_by(|a, b| a.code == b.code && a.range() == b.range());
}

struct Resolver<'a> {
    env: &'a Env,
    p: &'a Expr,
    scopes: Vec<HashMap<String, Bind>>,
    res: Vec<Res>,
    measure: Vec<u16>,
    diags: Vec<Diagnostic>,
    slots: u32,
    lambda_levels: Vec<u16>,
    /// F16 3.1: the type lines that declared, by name: the type and the span
    /// of the name.
    type_lines: HashMap<String, (Ty, Range)>,
    /// The names of every type line so far, for `duplicate-name`.
    type_line_names: HashSet<String>,
    /// The `const`-bound lambdas whose text the walk is in (22.5).
    walking: Vec<NodeId>,
    /// For each lambda with a reference to its own name: the first one, and
    /// the name.
    self_refs: HashMap<u32, (Range, String)>,
    annots: HashMap<u32, Option<Ty>>,
    type_refs: Vec<TypeRef>,
    tables: Vec<std::sync::Arc<crate::check::ty::Table>>,
    /// The frames the walk is in, the program's first: the slots each holds
    /// so far (F17 11.5).
    open: Vec<u32>,
    place: Vec<Place>,
    frames: Vec<Frame>,
    plain_params: HashSet<u32>,
    /// F19 3.4 item 1: the name of the step `.f` or `?.f` after the primary
    /// the walk is about to resolve, when it is an `Ident`.
    follow: Option<String>,
}

/// A lambda argument (`05-types.md` 22.4, F17 13.1): a lambda, written in
/// place or the name of a `const`-bound one, or the name of a function
/// parameter, by its `Param`.
#[derive(Clone, Copy)]
enum LamArg {
    Lambda(NodeId),
    Fn(NodeId),
}

/// F17 11.1: a lambda is typed when it has a result annotation and every
/// parameter has an annotation, of a program type or a function type. It is
/// read from the text alone.
pub(crate) fn is_typed(ast: &crate::syntax::ast::Ast, l: NodeId) -> bool {
    let NodeKind::Lambda { params, result, .. } = ast.kind(l) else {
        return false;
    };
    result.is_some()
        && params.iter().all(|p| {
            matches!(
                ast.kind(*p),
                NodeKind::Param { ty: Some(_), .. } | NodeKind::Param { fn_: Some(_), .. }
            )
        })
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
    /// F17 13.2 rule 1: a function binding, by its slot and its `Param`.
    Fn(u32, NodeId),
    Lib(usize),
    Host(usize),
}

impl Resolver<'_> {
    fn kind(&self, id: NodeId) -> &NodeKind {
        self.p.ast.kind(id)
    }

    fn span(&self, id: NodeId) -> Range {
        self.p.ast.span(id)
    }

    /// A diagnostic, with its message from the catalog (`f08-errors.md` 3.5).
    fn diag(&mut self, c: &'static str, span: Range, template: &str, params: &[(&str, String)]) {
        let msg = crate::messages::build(c, template, params);
        self.diags
            .push(Diagnostic::at(Phase::Check, c, &self.p.source, span, msg));
    }

    /// `undefined-function`, with the hint of F19 3.4 when the lookup gives
    /// a fix.
    fn undefined_function(&mut self, span: Range, full: &str) {
        match crate::feedback::function_fix(self.env, full) {
            Some(fix) => {
                let p = [("name", full.to_string()), ("fix", fix)];
                self.diag(code::STATIC[1], span, "hint", &p);
            }
            None => {
                let p = [("name", full.to_string())];
                self.diag(code::STATIC[1], span, "default", &p);
            }
        }
    }

    /// The source text at a span, as visible text (`f08-errors.md` 3.2).
    fn text(&self, span: Range) -> String {
        self.p.source.param_text(span)
    }

    /// `arity` (`f08-errors.md` 3.5 item 5).
    fn arity(
        &mut self,
        span: Range,
        name: &str,
        count: usize,
        required: usize,
        most: Option<usize>,
    ) {
        let template = match most {
            None => "at-least",
            Some(m) if m == required => "exact",
            Some(_) => "range",
        };
        let params = [
            ("name", name.to_string()),
            ("count", count.to_string()),
            ("required", required.to_string()),
            ("most", most.unwrap_or_default().to_string()),
        ];
        self.diag("arity", span, template, &params);
    }

    /// F4 3.5 and 6.1: resolve a program type. Names first; then two records
    /// in one `PType`; then depth and size, one `check-limit` for both.
    fn program_type(&mut self, pt: &PType) -> Option<Ty> {
        let mut undefined = Vec::new();
        let mut found = Vec::new();
        names_of(pt, true, &mut |n, span, _| match self.type_lookup(n) {
            None => undefined.push(span),
            Some((ty, declaration)) => found.push(TypeRef {
                span,
                ty,
                declaration,
            }),
        });
        self.type_refs.extend(found);
        if !undefined.is_empty() {
            for s in undefined {
                let name = self.text(s);
                self.diag("undefined-name", s, "type", &[("name", name)]);
            }
            return None;
        }
        let env = self.env;
        let lines = std::mem::take(&mut self.type_lines);
        let mut lookup = |n: &str, _top: bool| -> Ty {
            env.data()
                .names
                .lookup(n)
                .or_else(|| lines.get(n).map(|x| x.0.clone()))
                .unwrap_or_default()
        };
        let t = self.checked_type(pt, &mut lookup);
        self.type_lines = lines;
        t
    }

    /// Resolve a program type whose names are known, and apply the rules of
    /// F4 6.1 after the names: two records with no tag, the bound of pairs
    /// (F17 12.5 item 7), then size and depth.
    fn checked_type(&mut self, pt: &PType, lookup: &mut dyn FnMut(&str, bool) -> Ty) -> Option<Ty> {
        let mut untagged = Vec::new();
        let w = crate::check::ty::watch();
        let t = ptype_ty(pt, true, lookup, &mut untagged);
        let passed = w.passed();
        drop(w);
        if !untagged.is_empty() {
            for s in untagged {
                self.diag("untagged-union", s, "default", &[]);
            }
            return None;
        }
        if passed {
            self.diag("check-limit", pt.span, "pairs", &[]);
            return None;
        }
        // F4 3.5: depth (step 5) before size (step 6).
        if t.depth() > 64 {
            self.diag("check-limit", pt.span, "depth", &[]);
            return None;
        }
        if t.too_big() {
            self.diag("check-limit", pt.span, "size", &[]);
            return None;
        }
        Some(t)
    }

    /// A type name in scope (F16 3.1 item 2): a named or opaque type of the
    /// environment, or a type line before this place, with the span of its
    /// name.
    fn type_lookup(&self, n: &str) -> Option<(Ty, Option<Range>)> {
        if let Some(t) = self.env.data().names.lookup(n) {
            return Some((t, None));
        }
        self.type_lines.get(n).map(|(t, s)| (t.clone(), Some(*s)))
    }

    /// Resolve an annotation, and keep what it resolved to (F16 3.3).
    fn annotation(&mut self, pt: &PType) {
        let r = self.program_type(pt);
        self.annots.insert(pt.span.start, r);
    }

    /// F17 12.2, 12.3: resolve the type lines of a program as one list,
    /// before the walk. Gives what each line declares, by its index in
    /// `types`; `None` for a line that fails.
    fn type_lines_list(&mut self, types: &[TypeLine]) -> Vec<Option<Ty>> {
        use crate::check::list;
        use crate::check::ty::{beside_its_kind, Ref, Table};
        let n = types.len();
        let env = self.env;
        let names = &env.data().names;
        // 12.2 item 1: a line that is `name-clash` names nothing.
        let clash: Vec<bool> = types
            .iter()
            .map(|t| names.lookup(&t.name).is_some())
            .collect();
        let mut order: Vec<usize> = (0..n).collect();
        order.sort_by_key(|&i| types[i].span.start);
        let mut rank = vec![0usize; n];
        for (r, &i) in order.iter().enumerate() {
            rank[i] = r;
        }
        // The lines of each name that does not clash, in order of place.
        let mut by_name: HashMap<&str, Vec<usize>> = HashMap::new();
        for &j in &order {
            if !clash[j] {
                by_name.entry(types[j].name.as_str()).or_default().push(j);
            }
        }
        // The line a name names from line i: the last of that name at or
        // before it, else the first after it.
        let named = |i: usize, name: &str| -> Option<usize> {
            let lines = by_name.get(name)?;
            let at = lines.partition_point(|&j| rank[j] <= rank[i]);
            Some(lines[at.saturating_sub(1)])
        };
        #[derive(Clone)]
        enum Target {
            Env(Ty),
            Line(usize),
            Nothing,
        }
        // Each name of each line: its span, whether at a top place, and what
        // it names.
        let mut line_names: Vec<Vec<(Range, bool, Target)>> = Vec::with_capacity(n);
        let mut uses: Vec<Vec<(usize, bool)>> = Vec::with_capacity(n);
        for (i, t) in types.iter().enumerate() {
            let mut ns = Vec::new();
            let mut us = Vec::new();
            names_of(&t.ty, true, &mut |name, span, top| {
                let target = match names.lookup(name) {
                    Some(ty) => Target::Env(ty),
                    None => match named(i, name) {
                        Some(j) => {
                            us.push((j, top));
                            Target::Line(j)
                        }
                        None => Target::Nothing,
                    },
                };
                ns.push((span, top, target));
            });
            line_names.push(ns);
            uses.push(us);
        }
        let plan = list::plan(&list::Graph { uses });
        let mut comp_of = vec![0usize; n];
        for (c, comp) in plan.components.iter().enumerate() {
            for &i in comp {
                comp_of[i] = c;
            }
        }
        let mut declared: Vec<Option<Ty>> = vec![None; n];
        let mut failed = vec![false; n];
        // Each line's index in its component's table.
        let mut at_in_comp = vec![0u32; n];
        for (c, comp) in plan.components.iter().enumerate() {
            // 12.2 items 2, 3 and 7: an undefined name, a cycle with no
            // guard, or a name of a failed line of another component.
            let mut bad = false;
            for &i in comp {
                for (span, _, target) in &line_names[i] {
                    let undefined = match target {
                        Target::Nothing => true,
                        Target::Line(j) => comp_of[*j] != c && failed[*j],
                        Target::Env(_) => false,
                    };
                    if undefined {
                        let name = self.text(*span);
                        self.diag("undefined-name", *span, "type", &[("name", name)]);
                        bad = true;
                    }
                }
            }
            for &i in comp {
                if plan.direct[i] {
                    let p = [("name", types[i].name.clone())];
                    self.diag("circular-type", types[i].name_span, "direct", &p);
                    bad = true;
                }
            }
            if bad {
                for &i in comp {
                    failed[i] = true;
                }
                continue;
            }
            if !plan.recursive[comp[0]] {
                let i = comp[0];
                let decl = &declared;
                let mut lookup = |name: &str, _top: bool| -> Ty {
                    match names.lookup(name) {
                        Some(t) => t,
                        None => named(i, name)
                            .and_then(|j| decl[j].clone())
                            .unwrap_or_default(),
                    }
                };
                match self.checked_type(&types[i].ty, &mut lookup) {
                    Some(t) => declared[i] = Some(t),
                    None => failed[i] = true,
                }
                continue;
            }
            // A recursive component: a table, and each line resolved after
            // the lines it names at top places.
            let table = Table::new();
            for (k, &i) in comp.iter().enumerate() {
                at_in_comp[i] = k as u32;
            }
            let at = &at_in_comp;
            let idx = |i: usize| at[i];
            let mut seq: Vec<usize> = comp.clone();
            seq.sort_by_key(|&i| plan.top_rank[i]);
            let mut defs: Vec<Option<Ty>> = vec![None; comp.len()];
            let mut text_bad = false;
            for &i in &seq {
                let decl = &declared;
                let defs_now = &defs;
                let table = &table;
                let mut lookup = |name: &str, top: bool| -> Ty {
                    if let Some(t) = names.lookup(name) {
                        return t;
                    }
                    let Some(j) = named(i, name) else {
                        return Ty::never();
                    };
                    if comp_of[j] == c {
                        if top {
                            return defs_now[idx(j) as usize].clone().unwrap_or_default();
                        }
                        return Ty {
                            refs: [Ref::new(table, idx(j), &types[j].name)].into(),
                            ..Ty::default()
                        };
                    }
                    let d = decl[j].clone().unwrap_or_default();
                    match (top, plan.recursive[j]) {
                        (true, true) => d
                            .refs
                            .iter()
                            .next()
                            .and_then(|r| r.definition())
                            .unwrap_or_default(),
                        _ => d,
                    }
                };
                match self.checked_type(&types[i].ty, &mut lookup) {
                    Some(t) => defs[idx(i) as usize] = Some(t),
                    None => text_bad = true,
                }
            }
            if !text_bad {
                let ds: Vec<Ty> = defs.iter().map(|d| d.clone().unwrap_or_default()).collect();
                table.set(ds.clone());
                // 12.2 item 6: a reference beside a type of its kind.
                for &i in comp {
                    if let Some(name) = beside_its_kind(&ds[idx(i) as usize]) {
                        self.diag(
                            "circular-type",
                            types[i].name_span,
                            "union",
                            &[("name", name)],
                        );
                        text_bad = true;
                        continue;
                    }
                    // 12.6 item 1: the definition is tested with each
                    // reference at its top replaced (12.2 item 4), which a
                    // reference through a line that is not recursive makes
                    // larger than the line's text.
                    let top = Ref::new(&table, idx(i), &types[i].name)
                        .unfolding()
                        .unwrap_or_default();
                    // F4 3.5: depth (step 5) before size (step 6).
                    let template = if top.depth() > 64 {
                        "depth"
                    } else if top.too_big() {
                        "size"
                    } else {
                        continue;
                    };
                    self.diag("check-limit", types[i].ty.span, template, &[]);
                    text_bad = true;
                }
            }
            if text_bad {
                for &i in comp {
                    failed[i] = true;
                }
                continue;
            }
            for &i in comp {
                declared[i] = Some(Ty {
                    refs: [Ref::new(&table, idx(i), &types[i].name)].into(),
                    ..Ty::default()
                });
            }
            self.tables.push(table);
        }
        // F16 7.3: each name of a line's type that stands for a type.
        for ns in &line_names {
            for (span, _, target) in ns {
                let r = match target {
                    Target::Env(t) => Some((t.clone(), None)),
                    Target::Line(j) => declared[*j].clone().map(|t| (t, Some(types[*j].name_span))),
                    Target::Nothing => None,
                };
                if let Some((ty, declaration)) = r {
                    self.type_refs.push(TypeRef {
                        span: *span,
                        ty,
                        declaration,
                    });
                }
            }
        }
        declared
    }

    /// A type line in the walk (F16 3.1 items 2 to 6): its own diagnostics,
    /// and what it declares for the annotations after it. Its type was
    /// resolved with the list (F17 12.3).
    fn type_line(&mut self, t: &TypeLine, declared: Option<Ty>) {
        let p = [("name", t.name.clone())];
        let env_ty = self.env.data().names.lookup(&t.name);
        if env_ty.is_some() {
            self.diag("name-clash", t.name_span, "default", &p);
        } else if !self.type_line_names.insert(t.name.clone()) {
            self.diag(code::STATIC[5], t.name_span, "default", &p);
        }
        match (env_ty, declared) {
            (Some(e), _) => self.type_refs.push(TypeRef {
                span: t.name_span,
                ty: e,
                declaration: None,
            }),
            (None, Some(ty)) => {
                self.type_refs.push(TypeRef {
                    span: t.name_span,
                    ty: ty.clone(),
                    declaration: Some(t.name_span),
                });
                self.type_lines.insert(t.name.clone(), (ty, t.name_span));
            }
            // R240: a later line stands also when its type fails; it
            // declares nothing, and the earlier declaration ends here.
            (None, None) => {
                self.type_lines.remove(&t.name);
            }
        }
    }

    /// Whether a reference of lambda `l` stands in its own text and is
    /// `recursion-needs-types` (22.5). F17 11.2: a self-reference of a typed
    /// lambda is not; it is a reference as any other.
    fn is_self_ref(&self, l: NodeId) -> bool {
        self.walking.contains(&l) && !is_typed(&self.p.ast, l)
    }

    /// A reference of a lambda to its own name (22.5): the first is kept.
    fn self_ref(&mut self, l: NodeId, span: Range, name: &str) {
        let e = self
            .self_refs
            .entry(l.0)
            .or_insert_with(|| (span, name.to_string()));
        if span.start < e.0.start {
            *e = (span, name.to_string());
        }
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
    fn bind(&mut self, name: &str, span: Range, b: Option<Bind>) -> Option<u32> {
        if name == "_" {
            return None;
        }
        if self.env.is_namespace(name) {
            self.diag(
                code::STATIC[6],
                span,
                "default",
                &[("name", name.to_string())],
            );
            return None;
        }
        let scope = self.scopes.len() - 1;
        if self.scopes[scope].contains_key(name) {
            self.diag(
                code::STATIC[5],
                span,
                "default",
                &[("name", name.to_string())],
            );
        }
        let (bind, slot) = match b {
            Some(b) => (b, None),
            None => {
                let s = self.slots;
                self.slots += 1;
                let depth = self.open.len() - 1;
                self.place.push(Place {
                    depth: depth as u16,
                    index: self.open[depth],
                });
                self.open[depth] += 1;
                (Bind::Slot(s), Some(s))
            }
        };
        self.scopes[scope].insert(name.to_string(), bind);
        slot
    }

    fn const_line(&mut self, id: NodeId) {
        let NodeKind::Const {
            pattern, value, ty, ..
        } = self.kind(id).clone()
        else {
            return;
        };
        if let Some(t) = &ty {
            self.annotation(t);
        }
        if let (NodeKind::Lambda { .. }, NodeKind::NamePat { name: Some(n) }) =
            (self.kind(value), self.kind(pattern))
        {
            // 22.7: the name is bound first, then the lambda is walked with
            // the name in scope (`01-language.md` 5 rule 6).
            let n = n.clone();
            let span = self.span(pattern);
            self.bind(&n, span, Some(Bind::Lambda(value)));
            self.walking.push(value);
            self.walk(value);
            self.walking.pop();
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
                NodeKind::Program { consts, result, .. } | NodeKind::Block { consts, result } => {
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
                // The left operands of operators and the values of `as` and
                // `is` are one spine, which opens no level (13.1, F4 2.8):
                // a loop goes down it. A type is resolved on the way down,
                // a right operand on the way up.
                NodeKind::Binary { .. } | NodeKind::As { .. } => {
                    let mut rights = Vec::new();
                    let mut n = id;
                    loop {
                        match self.kind(n) {
                            NodeKind::Binary { x, y, .. } => {
                                rights.push(*y);
                                n = *x;
                            }
                            NodeKind::As { x, ty, .. } => {
                                let (x, ty) = (*x, ty.clone());
                                let r = self.program_type(&ty);
                                self.res[n.index()] = Res::As(r.map(Box::new));
                                n = x;
                            }
                            _ => break,
                        }
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
                // A projection on a subject is a step of its chain (F7 2.5),
                // so a loop walks it with the other steps.
                NodeKind::Each { x: Some(_), .. } => {
                    self.chain(id);
                    return;
                }
                NodeKind::Each { reads, .. } => {
                    self.reads(&reads);
                    return;
                }
                NodeKind::Call { name, args, .. } => {
                    let span = self.span(id);
                    self.call(id, None, &name.name, name.span, &args, span);
                    return;
                }
                NodeKind::Lambda {
                    params,
                    body,
                    result,
                } => {
                    // F17 13.1: only a `const`-bound lambda has hint
                    // parameters. `const_line` pushes the lambda it binds.
                    let const_bound = self.walking.last() == Some(&id);
                    self.lambda_levels.push(self.p.ast.node(id).level);
                    self.scopes.push(HashMap::new());
                    // F17 11.5: each call of the lambda has its own frame,
                    // for its parameters and every name its body binds.
                    self.open.push(0);
                    for p in params {
                        if let NodeKind::Param {
                            pattern,
                            default,
                            ty,
                            fn_,
                            ..
                        } = self.kind(p).clone()
                        {
                            if let Some(t) = &ty {
                                self.annotation(t);
                            }
                            // F17 13.3: the checker reads each listed type
                            // and the result, `None` with a diagnostic.
                            if let Some(f) = &fn_ {
                                for (_, _, t) in &f.params {
                                    self.annotation(t);
                                }
                                if let Some(r) = &f.result {
                                    self.annotation(r);
                                }
                            }
                            if let Some(d) = default {
                                self.walk(d);
                            }
                            self.pattern(pattern);
                            // F17 13.1 item 1: a function parameter makes a
                            // function binding, bound as a parameter is.
                            // 13.2 item 4: a hint parameter is one of a
                            // `const`-bound lambda, written as a name with
                            // no default and no function type.
                            if let (NodeKind::NamePat { name: Some(n) }, Res::Bind(s)) =
                                (self.kind(pattern).clone(), &self.res[pattern.index()])
                            {
                                let s = *s;
                                if fn_.is_some() {
                                    if let Some(scope) = self.scopes.last_mut() {
                                        scope.insert(n, Bind::Fn(s, p));
                                    }
                                } else if default.is_none() && const_bound {
                                    self.plain_params.insert(s);
                                }
                            }
                        }
                    }
                    if let Some(r) = &result {
                        self.annotation(r);
                    }
                    self.walk(body);
                    let size = self.open.pop().unwrap_or(0);
                    self.frames[id.index()] = Frame {
                        depth: self.open.len() as u16,
                        size,
                    };
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
            Some(Bind::Lambda(l)) => {
                self.res[id.index()] = Res::NotValue(l);
                // F17 13.5: a higher-order lambda can only be called.
                let template = if is_higher_order(&self.p.ast, l) {
                    "higher-order"
                } else {
                    "lambda"
                };
                self.diag(
                    code::STATIC[2],
                    span,
                    template,
                    &[("name", name.to_string())],
                );
            }
            // F17 13.2 rule 4: a function is not a value.
            Some(Bind::Fn(s, p)) => {
                self.res[id.index()] = Res::FnRef(s, p);
                self.diag(
                    code::STATIC[2],
                    span,
                    "function",
                    &[("name", name.to_string())],
                );
            }
            None => {
                let p = [("name", name.to_string())];
                if self.env.is_namespace(name) {
                    self.diag(code::STATIC[2], span, "namespace", &p);
                } else if self.is_function_name(name) {
                    self.diag(code::STATIC[2], span, "function", &p);
                } else {
                    let follow = self.follow.take();
                    match crate::feedback::name_fix(self.env, name, follow.as_deref()) {
                        // F19 3.5: the hint, with the fix of the lookup.
                        Some(fix) => {
                            let p = [("name", name.to_string()), ("fix", fix)];
                            self.diag(code::STATIC[0], span, "hint", &p)
                        }
                        None => self.diag(code::STATIC[0], span, "name", &p),
                    }
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
                NodeKind::Member { x, .. }
                | NodeKind::Index { x, .. }
                | NodeKind::Each { x: Some(x), .. } => {
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
        // F6 2.4 item 2: a plain `Member` on a group that is on that names a
        // constant reads it; its receiver is not walked.
        if let Some(&first) = steps.last() {
            if let (
                NodeKind::Member {
                    name,
                    optional: false,
                    ..
                },
                NodeKind::Ident { name: ns },
            ) = (self.kind(first).clone(), self.kind(n).clone())
            {
                if self.lookup(&ns).is_none() && self.env.group_on(&ns) {
                    if let Some(v) = self.env.constant(&ns, &name) {
                        self.res[first.index()] = Res::Constant(v);
                        steps.pop();
                        self.steps(steps);
                        return;
                    }
                }
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
            NodeKind::Ident { .. } => {
                // F19 3.4 item 1: the step `.f` or `?.f` after the name.
                self.follow = steps.last().and_then(|&f| match self.kind(f) {
                    NodeKind::Member { name, .. } => Some(name.clone()),
                    NodeKind::MethodCall { name, .. } => Some(name.name.clone()),
                    _ => None,
                });
                self.walk(n);
                self.follow = None;
            }
            _ => self.walk(n),
        }
        self.steps(steps);
    }

    /// What the steps of a chain hold, from the step next to the primary
    /// outward: the index of an `Index`, the arguments of a method call, the
    /// reads of a projection.
    fn steps(&mut self, steps: Vec<NodeId>) {
        for s in steps.into_iter().rev() {
            match self.kind(s).clone() {
                NodeKind::Index { index, .. } => self.walk(index),
                NodeKind::MethodCall { args, name, .. } => {
                    self.set_measure(s);
                    // F17 13.2 rule 8, at every method call.
                    self.higher_order_args(&args, &name.name);
                    self.args(&args);
                }
                NodeKind::Each { reads, .. } => self.reads(&reads),
                _ => {}
            }
        }
    }

    /// F7 5.7: each read of a projection in order: the index of an `Index`,
    /// and a later `Each` with its reads.
    fn reads(&mut self, reads: &[NodeId]) {
        for &r in reads {
            match self.kind(r) {
                NodeKind::Index { index, .. } => {
                    let index = *index;
                    self.walk(index);
                }
                NodeKind::Each { .. } => self.walk(r),
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

    /// A lambda argument (22.4, F17 13.1): a `Lambda` node, the name of a
    /// `const`-bound lambda, or the name of a function parameter.
    fn lambda_arg(&self, a: NodeId) -> Option<LamArg> {
        match self.kind(a) {
            NodeKind::Lambda { .. } => Some(LamArg::Lambda(a)),
            NodeKind::Ident { name } => match self.lookup(name) {
                Some(Bind::Lambda(l)) => Some(LamArg::Lambda(l)),
                Some(Bind::Fn(_, p)) => Some(LamArg::Fn(p)),
                _ => None,
            },
            _ => None,
        }
    }

    /// The count of parameters of a lambda argument: a lambda's, or the
    /// listed count of a function parameter's type (F17 13.1).
    fn lambda_params(&self, l: LamArg) -> usize {
        match l {
            LamArg::Lambda(l) => match self.kind(l) {
                NodeKind::Lambda { params, .. } => params.len(),
                _ => 0,
            },
            LamArg::Fn(p) => listed_count(&self.p.ast, p),
        }
    }

    /// F17 13.2 rule 8: each lambda argument that is a higher-order lambda,
    /// written there or named, is `lambda-to-lambda` whatever the callee,
    /// and no other rule tests it. Returns which arguments it reported.
    fn higher_order_args(&mut self, args: &[NodeId], callee: &str) -> Vec<bool> {
        let mut out = vec![false; args.len()];
        for (i, &a) in args.iter().enumerate() {
            if let Some(LamArg::Lambda(l)) = self.lambda_arg(a) {
                if is_higher_order(&self.p.ast, l) {
                    let span = self.span(a);
                    let p = [("name", callee.to_string())];
                    self.diag(code::STATIC[4], span, "higher-order", &p);
                    out[i] = true;
                }
            }
        }
        out
    }

    /// Walk the arguments of a call (22.5, last paragraph). A function
    /// parameter's name as a lambda argument is no read (F17 13.2 item 3).
    fn args(&mut self, args: &[NodeId]) {
        for &a in args {
            match self.kind(a).clone() {
                NodeKind::Ident { name } => match self.lookup(&name) {
                    Some(Bind::Lambda(l)) if self.is_self_ref(l) => {
                        let span = self.span(a);
                        self.self_ref(l, span, &name);
                        self.res[a.index()] = Res::SelfRef(l);
                    }
                    Some(Bind::Lambda(l)) => self.res[a.index()] = Res::LambdaRef(l),
                    Some(Bind::Fn(s, p)) => self.res[a.index()] = Res::FnRef(s, p),
                    _ => self.walk(a),
                },
                _ => self.walk(a),
            }
        }
    }

    /// The name of a parameter written as a name.
    fn param_name(&self, p: NodeId) -> String {
        match self.kind(p) {
            NodeKind::Param { pattern, .. } => match self.kind(*pattern) {
                NodeKind::NamePat { name: Some(n) } => n.clone(),
                _ => String::new(),
            },
            _ => String::new(),
        }
    }

    /// F17 13.2 item 4: whether an argument is an `Ident` whose innermost
    /// binding is a parameter written as a name with no default.
    fn is_plain_param(&self, a: NodeId) -> bool {
        match self.kind(a) {
            NodeKind::Ident { name } => {
                matches!(self.lookup(name), Some(Bind::Slot(s)) if self.plain_params.contains(&s))
            }
            _ => false,
        }
    }

    fn call(
        &mut self,
        id: NodeId,
        ns: Option<(String, Range)>,
        name: &str,
        name_span: Range,
        args: &[NodeId],
        call_span: Range,
    ) {
        self.set_measure(id);
        let full = match &ns {
            Some((n, _)) => format!("{n}.{name}"),
            None => name.to_string(),
        };
        let callee = match &ns {
            Some((n, s)) => match self.env.function(&format!("{n}.{name}")) {
                Some(i) => Callee::Host(i),
                // F6 2.4 item 1: an entry of a group that is on.
                None if self.env.lib_function(&format!("{n}.{name}")).is_some() => Callee::Lib(
                    self.env
                        .lib_function(&format!("{n}.{name}"))
                        .expect("an entry"),
                ),
                None => {
                    self.undefined_function(s.to(name_span), &full);
                    Callee::Unknown
                }
            },
            None => match self.lookup(name) {
                Some(Bind::Lambda(l)) => Callee::Lambda(l),
                Some(Bind::Fn(s, p)) => Callee::Fn(s, p),
                // F17 13.2 rule 1: the hint names the fix for a parameter
                // written as a name with no default.
                Some(b) => {
                    let template = match b {
                        Bind::Slot(s) if self.plain_params.contains(&s) => "fn-hint",
                        _ => "default",
                    };
                    let p = [("name", full.clone())];
                    self.diag(code::STATIC[3], name_span, template, &p);
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
                        self.undefined_function(name_span, &full);
                        Callee::Unknown
                    }
                }
            },
        };
        // F17 13.2 rule 8 decides first and alone, at every call.
        let reported = self.higher_order_args(args, &full);
        let counts = match &callee {
            Callee::Unknown => None,
            Callee::Lambda(l) => {
                let NodeKind::Lambda { params, .. } = self.kind(*l) else {
                    unreachable!()
                };
                // Up to the last parameter with no default.
                let required = params
                    .iter()
                    .rposition(|p| matches!(self.kind(*p), NodeKind::Param { default: None, .. }))
                    .map_or(0, |i| i + 1);
                // F17 13.2 rules 2, 3 and 9: a function parameter takes a
                // lambda argument of at most its listed count.
                let fn_params = params
                    .iter()
                    .map(|p| match self.kind(*p) {
                        NodeKind::Param { fn_: Some(f), .. } => Some(f.params.len()),
                        _ => None,
                    })
                    .collect();
                Some(Counts {
                    required,
                    at_most: Some(params.len()),
                    fn_params,
                    rest_fn: None,
                })
            }
            // F17 13.2 rule 7: exactly the listed count.
            Callee::Fn(_, p) => {
                let n = listed_count(&self.p.ast, *p);
                Some(Counts {
                    required: n,
                    at_most: Some(n),
                    fn_params: vec![None; n],
                    rest_fn: None,
                })
            }
            Callee::Lib(i) => Some(sig_counts(&library::records()[*i].sig)),
            Callee::Host(i) => Some(sig_counts(&self.env.data().functions[*i].sig)),
        };
        if let Some(c) = counts {
            let n = args.len();
            if n < c.required || c.at_most.is_some_and(|m| n > m) {
                self.arity(call_span, &full, n, c.required, c.at_most);
            } else {
                for (i, &a) in args.iter().enumerate() {
                    if reported[i] {
                        continue;
                    }
                    let span = self.span(a);
                    let lam = self.lambda_arg(a);
                    if let Callee::Lambda(callee_l) = &callee {
                        let NodeKind::Lambda { params, .. } = self.kind(*callee_l) else {
                            continue;
                        };
                        let param = params[i];
                        match (c.param(i).flatten(), lam) {
                            // Rule 9.
                            (Some(k), Some(l)) => {
                                if self.lambda_params(l) > k {
                                    let p = [("name", full.clone()), ("most", k.to_string())];
                                    self.diag(code::STATIC[8], span, "parameters", &p);
                                }
                            }
                            // Rule 3.
                            (Some(_), None) => {
                                if self.is_plain_param(a) {
                                    let p = [("name", self.text(span))];
                                    self.diag("arg-mismatch", span, "parameter-at-fn", &p);
                                } else {
                                    let p =
                                        [("param", self.param_name(param)), ("name", full.clone())];
                                    self.diag("arg-mismatch", span, "function", &p);
                                }
                            }
                            // Rule 2: `fn-hint`, naming the parameter, when
                            // it is written as a name with no default; a
                            // pattern, `_` or a default takes a value.
                            (None, Some(_)) => match self.kind(param) {
                                NodeKind::Param {
                                    pattern,
                                    default: None,
                                    ..
                                } if matches!(
                                    self.kind(*pattern),
                                    NodeKind::NamePat { name: Some(_) }
                                ) =>
                                {
                                    let p = [("name", self.param_name(param))];
                                    self.diag(code::STATIC[4], span, "fn-hint", &p)
                                }
                                _ => {
                                    let p = [("name", full.clone())];
                                    self.diag(code::STATIC[4], span, "default", &p)
                                }
                            },
                            (None, None) => {}
                        }
                        continue;
                    }
                    let Some(l) = lam else {
                        continue;
                    };
                    match &callee {
                        // Rule 7: each listed parameter takes a value.
                        Callee::Fn(_, p) => {
                            let word = match self.kind(*p) {
                                NodeKind::Param { fn_: Some(f), .. } => {
                                    f.params.get(i).map(|w| w.0.clone()).unwrap_or_default()
                                }
                                _ => String::new(),
                            };
                            let p = [("name", full.clone()), ("param", word)];
                            self.diag(code::STATIC[2], span, "argument", &p)
                        }
                        _ => match c.param(i).flatten() {
                            None => {
                                let sig = match &callee {
                                    Callee::Lib(j) => Some(&library::records()[*j].sig),
                                    Callee::Host(j) => Some(&self.env.data().functions[*j].sig),
                                    _ => None,
                                };
                                let param = sig
                                    .and_then(|g| g.param_at(i))
                                    .map(|q| q.name.clone())
                                    .unwrap_or_default();
                                let p = [("name", full.clone()), ("param", param)];
                                self.diag(code::STATIC[2], span, "argument", &p)
                            }
                            Some(k) => {
                                if self.lambda_params(l) > k {
                                    let p = [("name", full.clone()), ("most", k.to_string())];
                                    self.diag(code::STATIC[8], span, "parameters", &p);
                                }
                            }
                        },
                    }
                }
            }
        }
        self.res[id.index()] = match callee {
            Callee::Unknown => Res::None,
            // 22.5: a call of the lambda inside its own text.
            Callee::Lambda(l) if self.is_self_ref(l) => {
                self.self_ref(l, name_span, name);
                Res::SelfRef(l)
            }
            Callee::Lambda(l) => Res::CallLambda(l),
            Callee::Fn(s, p) => Res::CallFn(s, p),
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
                    let key = crate::messages::json_string(&k);
                    if !seen.insert(k) {
                        self.diag(code::STATIC[7], key_span, "default", &[("key", key)]);
                    }
                }
            }
            self.walk(e);
        }
    }

    /// The arms of a `match`, with `duplicate-case` and `unreachable-arm`
    /// (22.8, as F17 4.1 states them with guards). A literal is compared
    /// with the earlier literals of its own arm and with those of earlier
    /// plain arms; a guarded arm may list again what an earlier guarded arm
    /// lists. Only a plain `_` makes the arms after it unreachable.
    fn arms(&mut self, arms: &[NodeId]) {
        let mut seen: HashSet<LitKey> = HashSet::new();
        let mut after_default = false;
        for &a in arms {
            let NodeKind::Arm {
                literals,
                guard,
                result,
            } = self.kind(a).clone()
            else {
                continue;
            };
            if after_default {
                let span = self.span(a);
                self.diag(code::STATIC[10], span, "default", &[]);
            }
            let plain = guard.is_none();
            match literals {
                None => after_default |= plain,
                Some(lits) => {
                    let mut own: HashSet<LitKey> = HashSet::new();
                    for l in lits {
                        let k = lit_key(&l.lit);
                        if seen.contains(&k) || !own.insert(k) {
                            let text = self.text(l.span);
                            self.diag(code::STATIC[9], l.span, "default", &[("text", text)]);
                        }
                    }
                    if plain {
                        seen.extend(own);
                    }
                }
            }
            // 22.4 with F17 4.1: the guard, then the result.
            if let Some(g) = guard {
                self.walk(g);
            }
            self.walk(result);
        }
    }
}

/// A literal of a `match` arm as a key equal by `==` (`01-language.md` 6.7):
/// `1` and `1.0` are one key, and so are `0` and `-0`. A set of keys finds a
/// literal listed twice in n log n work (`05-types.md` 20.7).
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

/// Every type name of a program type, with its span and whether it stands
/// at a top place (F17 12.1), in order.
fn names_of(pt: &PType, top: bool, f: &mut dyn FnMut(&str, Range, bool)) {
    for a in &pt.alts {
        let top = top && a.suffixes == 0;
        match &a.prim {
            PPrim::Name(n, s) => f(n, *s, top),
            PPrim::Paren(inner) => names_of(inner, top, f),
            PPrim::Map(inner) => names_of(inner, false, f),
            PPrim::Record { fields, rest } => {
                for fl in fields {
                    names_of(&fl.ty, false, f);
                }
                if let Some(Some(r)) = rest {
                    names_of(r, false, f);
                }
            }
            _ => {}
        }
    }
}

/// One `PType`: the Join of its alternatives, with parentheses removed. Two
/// or more of them that are records is `untagged-union`, whose span is
/// added to `untagged`; the `PType` then counts as `never` for the one
/// around it. `lookup` gives the type a name stands for, and is told
/// whether it stands at a top place (F17 12.2 item 4).
fn ptype_ty(
    pt: &PType,
    top: bool,
    lookup: &mut dyn FnMut(&str, bool) -> Ty,
    untagged: &mut Vec<Range>,
) -> Ty {
    let mut alts: Vec<Ty> = Vec::new();
    for a in &pt.alts {
        let mut t = pprim_ty(&a.prim, top && a.suffixes == 0, lookup, untagged);
        for _ in 0..a.suffixes {
            t = Ty::list(t);
        }
        alts.push(t);
    }
    // F5 3.1, 3.2: two or more records form a tagged union, or are
    // `untagged-union` on this `PType`.
    match crate::check::ty::union_of_alts(&alts) {
        Some(t) => t,
        None => {
            untagged.push(pt.span);
            Ty::never()
        }
    }
}

fn pprim_ty(
    p: &PPrim,
    top: bool,
    lookup: &mut dyn FnMut(&str, bool) -> Ty,
    untagged: &mut Vec<Range>,
) -> Ty {
    match p {
        PPrim::Null => Ty::null(),
        PPrim::Bool => Ty::boolean(),
        PPrim::Int => Ty::int(),
        PPrim::Float => Ty::float(),
        PPrim::String => Ty::string(),
        PPrim::Any => Ty::any(),
        PPrim::Never => Ty::never(),
        PPrim::Lit(s) => Ty::lit(s),
        PPrim::Map(inner) => Ty::map_of(ptype_ty(inner, false, lookup, untagged)),
        PPrim::Record { fields, rest } => {
            let mut fs = std::collections::BTreeMap::new();
            for f in fields {
                let ty = ptype_ty(&f.ty, false, lookup, untagged);
                fs.insert(
                    f.key.clone(),
                    crate::check::ty::Field {
                        ty,
                        optional: f.optional,
                        doc: crate::check::ty::Doc(f.doc.clone()),
                    },
                );
            }
            let rest = match rest {
                None => Ty::never(),
                Some(None) => Ty::any(),
                Some(Some(t)) => ptype_ty(t, false, lookup, untagged),
            };
            Ty::rec(crate::check::ty::Rec::new(fs, rest))
        }
        PPrim::Name(n, _) => lookup(n, top),
        // A parenthesized `PType` alternative: its own alternatives count
        // for the records of the `PType` around it only through its Join.
        PPrim::Paren(inner) => ptype_ty(inner, top, lookup, untagged),
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
