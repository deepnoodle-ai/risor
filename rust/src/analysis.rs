//! Package F2: analysis (`spec/packages/f02-templates-analysis.md` 3).
//!
//! Analysis reads a parsed program, or a template's program in a mode, with
//! no environment: the names it reads, the paths it reads through them and
//! the calls it makes. It never fails, applies no static or type rule, and
//! walks the tree with an explicit stack, so that a chain or an operator
//! chain of any length costs it no host stack (3.1 item 5).

use crate::host::Data;
use crate::json::Json;
use crate::source::{Source, Span};
use crate::syntax::ast::{EntryKey, NodeId, NodeKind, TemplatePart, UnaryOp};
use crate::syntax::Expr;
use crate::template::{Segment, Template, TemplateMode};

/// A step of a path (3.5). Package F7 adds a star step beside the key.
#[derive(Clone, Debug, PartialEq)]
pub enum Step {
    Key(String),
    /// F7 9.1: a star.
    Star,
}

/// A reference: one read of a free name, with its path (3.5).
#[derive(Clone, Debug, PartialEq)]
pub struct Ref {
    pub name: String,
    pub path: Vec<Step>,
    pub span: Span,
}

/// A call of a free function name, or `n.m(...)` with a free `n` (3.6).
#[derive(Clone, Debug, PartialEq)]
pub(crate) struct CallRef {
    pub name: String,
    /// The receiver's name for a call of the form `n.m(...)`; `None` for a
    /// `Call`.
    pub namespace: Option<String>,
    pub span: Span,
    /// The span of each argument.
    pub args: Vec<Span>,
    /// The node of each argument, for [`CallSite::args`].
    pub(crate) arg_nodes: Vec<NodeId>,
    /// The call stands inside a `Lambda`: in its body or a default.
    pub lambda: bool,
}

/// The report of an analysis (3.3).
#[derive(Clone, Debug)]
pub struct Analysis {
    /// The names of the reads, each once, in code point order.
    pub names: Vec<String>,
    /// One reference for each read, by the start of the read.
    pub refs: Vec<Ref>,
    /// The calls, by the start of their span.
    pub calls: Vec<CallSite>,
    /// The value of the program when it is a literal (3.7).
    pub literal: Option<Data>,
    /// A template's segments; `None` for a program.
    pub segments: Option<Vec<Segment>>,
    /// The text analyzed, for the spans of [`Analysis::json`].
    pub(crate) source: Source,
}

impl PartialEq for Analysis {
    fn eq(&self, o: &Analysis) -> bool {
        self.names == o.names
            && self.refs == o.refs
            && self.calls == o.calls
            && self.literal == o.literal
            && self.segments == o.segments
    }
}

/// The analysis of a parsed program (3.1).
pub(crate) fn analyze(p: &Expr) -> Analysis {
    let mut w = Walker {
        p,
        scopes: Vec::new(),
        refs: Vec::new(),
        calls: Vec::new(),
    };
    w.run();
    let mut refs = w.refs;
    refs.sort_by_key(|r| r.span.start);
    let mut calls = w.calls;
    calls.sort_by_key(|c| c.span.start);
    let calls = calls.into_iter().map(|c| call_site(p, c)).collect();
    let mut names: Vec<String> = refs.iter().map(|r| r.name.clone()).collect();
    names.sort();
    names.dedup();
    Analysis {
        names,
        refs,
        calls,
        literal: literal_of(p).map(|j| json_data(&j)),
        segments: None,
        source: p.source.clone(),
    }
}

/// The record of a call in an analysis (`05-host-api.md` E.2 item 7).
#[derive(Clone, Debug)]
pub struct CallSite {
    /// The name as called: `lookup`, or `xs.map` for `n.m(...)`.
    pub name: String,
    /// The receiver's name for a call of the form `n.m(...)`; `None` for a
    /// call by name.
    pub namespace: Option<String>,
    pub span: Span,
    /// Each argument as a program of its own text, with spans in the whole
    /// text (3.6 item 5); `None` for a lambda. An argument that reads a name
    /// the program binds does not compile alone.
    pub args: Vec<Option<Expr>>,
    /// The span of each argument.
    pub arg_spans: Vec<Span>,
    /// The call stands inside a lambda: in its body or a default.
    pub in_lambda: bool,
}

impl PartialEq for CallSite {
    fn eq(&self, o: &CallSite) -> bool {
        self.name == o.name
            && self.namespace == o.namespace
            && self.span == o.span
            && self.arg_spans == o.arg_spans
            && self.in_lambda == o.in_lambda
            && self
                .args
                .iter()
                .map(|a| a.as_ref().map(Expr::text))
                .eq(o.args.iter().map(|a| a.as_ref().map(Expr::text)))
    }
}

/// A call's record, with each argument as a program of its own.
fn call_site(program: &Expr, c: CallRef) -> CallSite {
    let args = c
        .arg_nodes
        .iter()
        .map(|&a| {
            if matches!(program.ast.kind(a), NodeKind::Lambda { .. }) {
                return None;
            }
            let mut ast = program.ast.clone();
            let span = ast.span(a);
            let root = ast.push(
                NodeKind::Program {
                    consts: Vec::new(),
                    result: a,
                },
                span,
                0,
            );
            ast.root = root;
            Some(Expr {
                source: program.source.clone(),
                ast,
            })
        })
        .collect();
    CallSite {
        name: c.name,
        namespace: c.namespace,
        span: c.span,
        args,
        arg_spans: c.args,
        in_lambda: c.lambda,
    }
}

/// The analysis of a template in a mode: of its program in that mode, with
/// its segments (3.1 item 6).
pub(crate) fn analyze_template(t: &Template, mode: TemplateMode) -> Analysis {
    let mut r = analyze(&t.program(mode));
    r.segments = Some(t.segments().to_vec());
    r
}

enum Task {
    Visit(NodeId, bool),
    Push,
    Pop,
    Bind(String),
    /// The names of a pattern, with its defaults in order (3.2).
    Pattern(NodeId, bool),
}

struct Walker<'p> {
    p: &'p Expr,
    scopes: Vec<Vec<String>>,
    refs: Vec<Ref>,
    calls: Vec<CallRef>,
}

impl<'p> Walker<'p> {
    fn kind(&self, id: NodeId) -> &'p NodeKind {
        self.p.ast.kind(id)
    }

    fn span(&self, id: NodeId) -> Span {
        self.p.ast.span(id)
    }

    fn free(&self, name: &str) -> bool {
        !self.scopes.iter().any(|s| s.iter().any(|n| n == name))
    }

    fn run(&mut self) {
        let mut stack = vec![Task::Visit(self.p.ast.root, false)];
        while let Some(t) = stack.pop() {
            match t {
                Task::Push => self.scopes.push(Vec::new()),
                Task::Pop => {
                    self.scopes.pop();
                }
                Task::Bind(n) => {
                    if self.scopes.is_empty() {
                        self.scopes.push(Vec::new());
                    }
                    self.scopes.last_mut().expect("a scope").push(n);
                }
                Task::Pattern(pat, lam) => {
                    let mut tasks = Vec::new();
                    self.pattern_tasks(pat, lam, &mut tasks);
                    stack.extend(tasks.into_iter().rev());
                }
                Task::Visit(id, lam) => {
                    let mut tasks = Vec::new();
                    self.visit(id, lam, &mut tasks);
                    stack.extend(tasks.into_iter().rev());
                }
            }
        }
    }

    /// The tasks of a pattern: each default, then the name it guards, in
    /// order, so a default sees only the names to its left.
    fn pattern_tasks(&self, pat: NodeId, lam: bool, out: &mut Vec<Task>) {
        match self.kind(pat) {
            NodeKind::NamePat { name: Some(n) } => out.push(Task::Bind(n.clone())),
            NodeKind::MapPat { fields, rest } => {
                for f in fields {
                    if let Some(d) = f.default {
                        out.push(Task::Visit(d, lam));
                    }
                    out.push(Task::Bind(f.name.name.clone()));
                }
                if let Some(r) = rest {
                    out.push(Task::Bind(r.name.clone()));
                }
            }
            NodeKind::ListPat { elems, rest } => {
                for e in elems {
                    if let Some(d) = e.default {
                        out.push(Task::Visit(d, lam));
                    }
                    out.push(Task::Pattern(e.pat, lam));
                }
                if let Some(r) = rest {
                    out.push(Task::Bind(r.name.clone()));
                }
            }
            _ => {}
        }
    }

    fn visit(&mut self, id: NodeId, lam: bool, out: &mut Vec<Task>) {
        match self.kind(id) {
            NodeKind::Program { consts, result } | NodeKind::Block { consts, result } => {
                out.push(Task::Push);
                for &c in consts {
                    if let NodeKind::Const { pattern, value } = self.kind(c) {
                        // A name is not in scope in its own value.
                        out.push(Task::Visit(*value, lam));
                        out.push(Task::Pattern(*pattern, lam));
                    }
                }
                out.push(Task::Visit(*result, lam));
                out.push(Task::Pop);
            }
            NodeKind::Ident { name } => {
                if self.free(name) {
                    self.refs.push(Ref {
                        name: name.clone(),
                        path: Vec::new(),
                        span: self.span(id),
                    });
                }
            }
            NodeKind::Member { .. }
            | NodeKind::Index { .. }
            | NodeKind::MethodCall { .. }
            | NodeKind::OptChain { .. }
            | NodeKind::Each { .. } => self.chain(id, lam, out),
            NodeKind::Call { name, args, .. } => {
                if self.free(&name.name) {
                    self.calls.push(CallRef {
                        name: name.name.clone(),
                        namespace: None,
                        span: self.span(id),
                        args: args.iter().map(|a| self.span(*a)).collect(),
                        arg_nodes: args.clone(),
                        lambda: lam,
                    });
                }
                for &a in args {
                    out.push(Task::Visit(a, lam));
                }
            }
            NodeKind::Lambda { params, body } => {
                out.push(Task::Push);
                for &p in params {
                    if let NodeKind::Param { pattern, default } = self.kind(p) {
                        if let Some(d) = default {
                            out.push(Task::Visit(*d, true));
                        }
                        out.push(Task::Pattern(*pattern, true));
                    }
                }
                out.push(Task::Visit(*body, true));
                out.push(Task::Pop);
            }
            NodeKind::Try {
                body,
                name,
                handler,
            } => {
                out.push(Task::Push);
                out.push(Task::Visit(*body, lam));
                out.push(Task::Pop);
                out.push(Task::Push);
                if let Some(n) = name {
                    out.push(Task::Bind(n.name.clone()));
                }
                out.push(Task::Visit(*handler, lam));
                out.push(Task::Pop);
            }
            NodeKind::Template { parts } => {
                for p in parts {
                    if let TemplatePart::Expr(e) = p {
                        out.push(Task::Visit(*e, lam));
                    }
                }
            }
            NodeKind::List { items } => {
                out.extend(items.iter().map(|&i| Task::Visit(i, lam)));
            }
            NodeKind::Map { entries } => {
                out.extend(entries.iter().map(|&i| Task::Visit(i, lam)));
            }
            NodeKind::Entry { key, value, .. } => {
                if let EntryKey::Computed(k) = key {
                    out.push(Task::Visit(*k, lam));
                }
                out.push(Task::Visit(*value, lam));
            }
            NodeKind::Spread { value } => out.push(Task::Visit(*value, lam)),
            // F4 1.4: the subject of `x as T` and `x is T` is walked as any
            // operand; the program type names nothing, and a path stops at
            // the `As` node.
            NodeKind::As { x, .. } => out.push(Task::Visit(*x, lam)),
            NodeKind::Unary { x, .. } => out.push(Task::Visit(*x, lam)),
            NodeKind::Binary { x, y, .. } => {
                // A long operator chain is a spine: its left operands are
                // walked by the loop of `run`, one task each.
                out.push(Task::Visit(*x, lam));
                out.push(Task::Visit(*y, lam));
            }
            NodeKind::Cond {
                test, then, else_, ..
            } => {
                out.push(Task::Visit(*test, lam));
                out.push(Task::Visit(*then, lam));
                out.push(Task::Visit(*else_, lam));
            }
            NodeKind::Match { subject, arms } => {
                out.push(Task::Visit(*subject, lam));
                for &a in arms {
                    if let NodeKind::Arm { result, .. } = self.kind(a) {
                        out.push(Task::Visit(*result, lam));
                    }
                }
            }
            _ => {}
        }
    }

    /// A chain (`01-language.md` 7.4), from its last step down to its
    /// primary. The path of a read runs from the primary outward (3.5).
    fn chain(&mut self, top: NodeId, lam: bool, out: &mut Vec<Task>) {
        let ast = &self.p.ast;
        let mut steps: Vec<NodeId> = Vec::new();
        let mut cur = top;
        loop {
            let n = ast.node(cur);
            if cur != top && n.parenthesized() {
                break;
            }
            match &n.kind {
                NodeKind::OptChain { x } => cur = *x,
                NodeKind::Member { x, .. }
                | NodeKind::Index { x, .. }
                | NodeKind::MethodCall { x, .. }
                | NodeKind::Each { x: Some(x), .. } => {
                    steps.push(cur);
                    cur = *x;
                }
                _ => break,
            }
        }
        steps.reverse();
        // F7 9.1: a projection's star and reads stand in place.
        let mut flat: Vec<NodeId> = Vec::new();
        for &s in &steps {
            flat_steps(ast, s, &mut flat);
        }
        let steps = flat;
        let primary = cur;
        let free_read = match ast.kind(primary) {
            NodeKind::Ident { name } if self.free(name) => Some(name.clone()),
            _ => None,
        };
        match (&free_read, ast.kind(primary)) {
            (Some(name), _) => {
                let mut path = Vec::new();
                let mut end = ast.span(primary).end;
                if !ast.node(primary).parenthesized() {
                    for &s in &steps {
                        match ast.kind(s) {
                            NodeKind::Member {
                                name, name_span, ..
                            } => {
                                path.push(Step::Key(name.clone()));
                                end = name_span.end;
                            }
                            NodeKind::Index { index, .. } => match ast.kind(*index) {
                                NodeKind::String { value, .. } => {
                                    path.push(Step::Key(value.clone()));
                                    end = ast.span(s).end;
                                }
                                _ => break,
                            },
                            NodeKind::Each { star, .. } => {
                                path.push(Step::Star);
                                end = star.end;
                            }
                            _ => break,
                        }
                    }
                }
                self.refs.push(Ref {
                    name: name.clone(),
                    path,
                    span: Span::new(ast.span(primary).start as usize, end as usize),
                });
                // A method call on a free name has the form of a call of a
                // namespace (3.6 item 3).
                if let Some(&first) = steps.first() {
                    if let NodeKind::MethodCall {
                        name: m,
                        args,
                        optional: false,
                        ..
                    } = ast.kind(first)
                    {
                        self.calls.push(CallRef {
                            name: format!("{name}.{}", m.name),
                            namespace: Some(name.clone()),
                            span: ast.span(first),
                            args: args.iter().map(|a| ast.span(*a)).collect(),
                            arg_nodes: args.clone(),
                            lambda: lam,
                        });
                    }
                }
            }
            (None, NodeKind::Ident { .. }) => {}
            (None, _) => out.push(Task::Visit(primary, lam)),
        }
        for &s in &steps {
            match ast.kind(s) {
                NodeKind::Index { index, .. } => out.push(Task::Visit(*index, lam)),
                NodeKind::MethodCall { args, .. } => {
                    out.extend(args.iter().map(|&a| Task::Visit(a, lam)));
                }
                _ => {}
            }
        }
    }
}

/// The steps of a chain in order, a projection's reads after its star.
fn flat_steps(ast: &crate::syntax::ast::Ast, s: NodeId, out: &mut Vec<NodeId>) {
    out.push(s);
    if let NodeKind::Each { reads, .. } = ast.kind(s) {
        for &r in reads {
            flat_steps(ast, r, out);
        }
    }
}

/// The literal of a program (3.7): no `const` line, and a result that is a
/// literal expression.
fn literal_of(p: &Expr) -> Option<Json> {
    let NodeKind::Program { consts, result } = p.ast.kind(p.ast.root) else {
        return None;
    };
    if !consts.is_empty() {
        return None;
    }
    literal(p, *result)
}

/// A literal expression's value. Lists and maps open a level each, so the
/// recursion is bounded by the nesting limit.
fn literal(p: &Expr, id: NodeId) -> Option<Json> {
    Some(match p.ast.kind(id) {
        NodeKind::Null => Json::Null,
        NodeKind::Bool { value } => Json::Bool(*value),
        NodeKind::Number { value } => Json::Num(*value),
        NodeKind::String { value, .. } => Json::Str(value.clone()),
        NodeKind::Unary {
            op: UnaryOp::Neg,
            x,
        } => match p.ast.kind(*x) {
            NodeKind::Number { value } => Json::Num(if *value == 0.0 { 0.0 } else { -value }),
            _ => return None,
        },
        NodeKind::List { items } => {
            let mut out = Vec::new();
            for &i in items {
                if matches!(p.ast.kind(i), NodeKind::Spread { .. }) {
                    return None;
                }
                out.push(literal(p, i)?);
            }
            Json::Arr(out)
        }
        NodeKind::Map { entries } => {
            let mut out: Vec<(String, Json)> = Vec::new();
            for &e in entries {
                let NodeKind::Entry {
                    key,
                    value,
                    shorthand: false,
                    ..
                } = p.ast.kind(e)
                else {
                    return None;
                };
                let k = match key {
                    EntryKey::Word(w) => w.clone(),
                    EntryKey::Str(s) => s.clone(),
                    EntryKey::Computed(_) => return None,
                };
                let v = literal(p, *value)?;
                // A key written twice takes its later value.
                out.retain(|(o, _)| *o != k);
                out.push((k, v));
            }
            Json::Obj(out)
        }
        _ => return None,
    })
}

impl Analysis {
    /// The report as JSON text, in the field order of 3.3 and 4.3, with
    /// spans as `[line, column, end line, end column]`.
    pub fn json(&self) -> String {
        let source = &self.source;
        let span = |s: Span| {
            let [a, b, c, d] = source.line_span(s).0;
            format!("[{a},{b},{c},{d}]")
        };
        let st = |s: &str| crate::json::to_text(&Json::Str(s.to_string()));
        let mut out = String::from("{\"names\":[");
        out.push_str(
            &self
                .names
                .iter()
                .map(|n| st(n))
                .collect::<Vec<_>>()
                .join(","),
        );
        out.push_str("],\"refs\":[");
        let refs: Vec<String> = self
            .refs
            .iter()
            .map(|r| {
                let path: Vec<String> = r
                    .path
                    .iter()
                    .map(|s| match s {
                        Step::Key(k) => st(k),
                        Step::Star => "{\"star\":true}".to_string(),
                    })
                    .collect();
                format!(
                    "{{\"name\":{},\"path\":[{}],\"span\":{}}}",
                    st(&r.name),
                    path.join(","),
                    span(r.span)
                )
            })
            .collect();
        out.push_str(&refs.join(","));
        out.push_str("],\"calls\":[");
        let calls: Vec<String> = self
            .calls
            .iter()
            .map(|c| {
                format!(
                    "{{\"name\":{},\"namespace\":{},\"span\":{},\"args\":[{}],\"lambda\":{}}}",
                    st(&c.name),
                    c.namespace.as_deref().map_or("null".to_string(), st),
                    span(c.span),
                    c.arg_spans
                        .iter()
                        .map(|a| span(*a))
                        .collect::<Vec<_>>()
                        .join(","),
                    c.in_lambda
                )
            })
            .collect();
        out.push_str(&calls.join(","));
        out.push_str("],\"literal\":");
        match &self.literal {
            None => out.push_str("false"),
            Some(v) => {
                out.push_str("{\"value\":");
                out.push_str(&crate::json::to_text(&data_json(v)));
                out.push('}');
            }
        }
        out.push_str(",\"segments\":");
        match &self.segments {
            None => out.push_str("null"),
            Some(segs) => {
                let s: Vec<String> = segs
                    .iter()
                    .map(|g| match g {
                        Segment::Text { value, span: sp } => {
                            format!("{{\"text\":{},\"span\":{}}}", st(value), span(*sp))
                        }
                        Segment::Part { expr, span: sp } => {
                            format!("{{\"part\":{},\"span\":{}}}", span(*expr), span(*sp))
                        }
                    })
                    .collect();
                out.push('[');
                out.push_str(&s.join(","));
                out.push(']');
            }
        }
        out.push('}');
        out
    }
}

/// A literal's JSON value as host data: a whole number in the safe range is
/// an `Int`, another number a `Float`.
fn json_data(j: &Json) -> Data {
    match j {
        Json::Null => Data::Null,
        Json::Bool(b) => Data::Bool(*b),
        Json::Num(n) if crate::value::is_whole(*n) && n.abs() <= 9_007_199_254_740_991.0 => {
            Data::Int(*n as i64)
        }
        Json::Num(n) => Data::Float(*n),
        Json::Str(s) => Data::Str(s.clone()),
        Json::Arr(a) => Data::List(a.iter().map(json_data).collect()),
        Json::Obj(o) => Data::Map(o.iter().map(|(k, v)| (k.clone(), json_data(v))).collect()),
    }
}

/// The JSON value of a literal's host data, which `json_data` gave.
fn data_json(d: &Data) -> Json {
    match d {
        Data::Null | Data::Opaque(_) => Json::Null,
        Data::Bool(b) => Json::Bool(*b),
        Data::Int(i) => Json::Num(*i as f64),
        Data::Float(f) => Json::Num(*f),
        Data::Str(s) => Json::Str(s.clone()),
        Data::List(a) => Json::Arr(a.iter().map(data_json).collect()),
        Data::Map(m) => Json::Obj(m.iter().map(|(k, v)| (k.clone(), data_json(v))).collect()),
    }
}
