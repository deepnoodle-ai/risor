//! The evaluator (`02-evaluation.md`).
//!
//! A walk over the syntax tree with the resolution of `compile`. Each form
//! charges as the tables of sections 5 and 6 say, in the order of section 2.
//! What opens no level runs in a loop: a left-associative chain of operators,
//! the steps of a chain, the `else` of `? :` and of `if`, `const` lines and
//! `match` arms. So the evaluator's recursion follows the source nesting and
//! the calls in progress, which the depth limit bounds (8.5 item 3).

pub mod lib;

use crate::compile::{Program, Res};
use crate::diag::{Diagnostic, Phase};
use crate::env::Limits;
use crate::host::{Data, Vars};
use crate::messages::{self, json_string, push_json_char};
use crate::number::{self, NumErr};
use crate::source::{Source, Span};
use crate::syntax::ast::{
    AsForm, Ast, BinaryOp, EntryKey, Lit, NodeId, NodeKind, SpannedName, TemplatePart, UnaryOp,
};
use crate::value::{
    self, from_view, is_whole, lcp_size, log2_ceil, size, weight, Kind, MapVal, Str, Value,
};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::rc::Rc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Instant;

pub const STEPS_EXCEEDED: &str = "steps-exceeded";
pub const ALLOCATION_EXCEEDED: &str = "allocation-exceeded";
pub const DEPTH_EXCEEDED: &str = "depth-exceeded";
pub const DEADLINE_EXCEEDED: &str = "deadline-exceeded";
pub const CANCELLED: &str = "cancelled";

/// The steps and units a run charged (`03-host.md` 3.6.3).
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Usage {
    pub steps: u64,
    pub allocated: u64,
}

/// The budgets of a run and what it has used.
pub struct Meter {
    pub steps: u64,
    pub units: u64,
    limits: Limits,
    /// The depth count of 8.4: calls in progress and walk levels.
    pub depth: u32,
    deadline: Option<Instant>,
    cancel: Option<Arc<AtomicBool>>,
    next_check: u64,
    /// A limit that a host function's charge passed.
    stop: Option<&'static str>,
}

impl Meter {
    fn charge(&mut self, n: u64) -> Result<(), &'static str> {
        if let Some(l) = self.limits.steps {
            if self.steps + n > l {
                return Err(STEPS_EXCEEDED);
            }
        }
        self.steps += n;
        if self.steps >= self.next_check {
            // 8.3: at least once in every 1,024 steps.
            self.next_check = self.steps + 1024;
            if self
                .cancel
                .as_ref()
                .is_some_and(|c| c.load(Ordering::Relaxed))
            {
                return Err(CANCELLED);
            }
            if self.deadline.is_some_and(|d| Instant::now() >= d) {
                return Err(DEADLINE_EXCEEDED);
            }
        }
        Ok(())
    }

    fn alloc(&mut self, n: u64) -> Result<(), &'static str> {
        if let Some(l) = self.limits.allocation {
            if self.units + n > l {
                return Err(ALLOCATION_EXCEEDED);
            }
        }
        self.units += n;
        Ok(())
    }
}

/// A limit was passed; the host function should return.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Stop;

/// The handle a host function receives (`03-host.md` 3.5.2).
pub struct Call<'r> {
    m: &'r mut Meter,
}

impl Call<'_> {
    /// Charge steps to the run. A charge the budget cannot cover ends the
    /// run with the limit code when the function returns.
    pub fn charge(&mut self, steps: u64) -> Result<(), Stop> {
        if self.m.stop.is_some() {
            return Err(Stop);
        }
        self.m.charge(steps).map_err(|c| {
            self.m.stop = Some(c);
            Stop
        })
    }

    pub fn allocate(&mut self, units: u64) -> Result<(), Stop> {
        if self.m.stop.is_some() {
            return Err(Stop);
        }
        self.m.alloc(units).map_err(|c| {
            self.m.stop = Some(c);
            Stop
        })
    }

    pub fn cancelled(&self) -> bool {
        self.m
            .cancel
            .as_ref()
            .is_some_and(|c| c.load(Ordering::Relaxed))
    }
}

/// A failure of a run.
#[derive(Clone, Debug)]
pub struct Fail<'h> {
    pub code: &'static str,
    pub template: &'static str,
    pub params: Vec<(&'static str, String)>,
    pub span: Option<Span>,
    /// The message of `failed` and `function-error`, which no template words.
    pub text: Option<String>,
    pub data: Option<Value<'h>>,
    /// A `number-range` from a host number: class `environment`.
    pub host_number: bool,
    /// The path of a `not-conforming` failure (F4 5.1).
    pub path: Option<Vec<crate::diag::PathStep>>,
}

impl Fail<'_> {
    pub fn message(&self) -> String {
        match &self.text {
            Some(t) => t.clone(),
            None => messages::build(self.code, self.template, &self.params),
        }
    }

    /// The class in a program compiled without a check (`03-host.md` 3.7.6).
    pub fn class(&self) -> Option<&'static str> {
        Some(match self.code {
            "null-operand" | "wrong-kind" | "missing-field" | "no-match" => "dynamic",
            "number-range" if self.host_number => "environment",
            "number-range" => "range",
            "missing-variable" => "environment",
            "index-range" => "index",
            "divide-by-zero" => "divide",
            "failed" | "invalid-argument" | "not-conforming" => "value",
            "function-error" | "function-panic" => "host",
            "internal-error" => return None,
            _ => "limit",
        })
    }

    /// The diagnostic a host sees.
    pub fn diagnostic(&self, src: &Source) -> Diagnostic {
        let mut d = match self.span {
            Some(s) => Diagnostic::at(Phase::Run, self.code, src, s, self.message()),
            None => Diagnostic::bare(Phase::Run, self.code, self.message()),
        };
        d.class = self.class();
        d.data = self.data.as_ref().map(|v| Box::new(v.to_data()));
        d.path = self.path.clone().map(Box::new);
        d
    }
}

type R<'h, T> = Result<T, Box<Fail<'h>>>;

/// Options of one run.
#[derive(Clone, Default)]
pub struct RunOptions {
    /// Set by the host to cancel the run.
    pub cancel: Option<Arc<AtomicBool>>,
}

/// What a run gave: a value or a failure, and the usage in both cases.
pub struct Outcome<'h> {
    pub result: Result<Value<'h>, Box<Fail<'h>>>,
    pub usage: Usage,
}

impl Program {
    /// Run the program with variables lent for the run.
    pub fn run<'h>(&'h self, vars: &'h dyn Vars) -> Outcome<'h> {
        self.run_with(vars, RunOptions::default())
    }

    pub fn run_with<'h>(&'h self, vars: &'h dyn Vars, opts: RunOptions) -> Outcome<'h> {
        let limits = self.env.data().limits;
        let mut run = Run {
            p: self,
            ast: &self.parsed.ast,
            vars,
            slots: vec![Value::Null; self.slots],
            cache: vec![None; self.env.data().vars.len()],
            m: Meter {
                steps: 0,
                units: 0,
                limits,
                depth: 0,
                deadline: limits.deadline.map(|d| Instant::now() + d),
                cancel: opts.cancel,
                next_check: 1024,
                stop: None,
            },
        };
        // Ruling AH1: handing the result over is not a read, so the run
        // makes no walk of it.
        let result = run.eval(self.parsed.ast.root);
        Outcome {
            usage: Usage {
                steps: run.m.steps,
                allocated: run.m.units,
            },
            result,
        }
    }
}

/// An argument of a library call: a value, or a lambda.
#[derive(Clone)]
enum Arg<'h> {
    Val(Value<'h>),
    Lam(NodeId),
}

struct Run<'h> {
    p: &'h Program,
    ast: &'h Ast,
    vars: &'h dyn Vars,
    slots: Vec<Value<'h>>,
    cache: Vec<Option<Option<Value<'h>>>>,
    m: Meter,
}

/// The walk's meter in a run: the run's steps and depth count (F4 5.3,
/// 5.4).
struct RunMeter<'a> {
    m: &'a mut Meter,
}

impl crate::conform::WalkMeter for RunMeter<'_> {
    fn charge(&mut self, n: u64) -> Result<(), &'static str> {
        self.m.charge(n)
    }
    fn enter(&mut self) -> Result<(), &'static str> {
        if self.m.depth + 1 > self.m.limits.depth {
            return Err(DEPTH_EXCEEDED);
        }
        self.m.depth += 1;
        Ok(())
    }
    fn leave(&mut self) {
        self.m.depth -= 1;
    }
}

fn kind_p<'h>(v: &Value<'h>) -> String {
    v.kind().name().to_string()
}

/// A parameter of kind `value` (`03-host.md` 3.8.3).
fn value_p(v: &Value<'_>) -> String {
    match v {
        Value::Null => "null".into(),
        Value::Bool(b) => b.to_string(),
        Value::Num(n) => number::to_text(*n),
        Value::Str(s) => json_string(s.as_str()),
        _ => v.kind().name().to_string(),
    }
}

fn op_text(op: BinaryOp) -> &'static str {
    match op {
        BinaryOp::NotIn => "not in",
        _ => op.as_str(),
    }
}

fn fail<'h>(
    code: &'static str,
    template: &'static str,
    span: Span,
    params: Vec<(&'static str, String)>,
) -> Box<Fail<'h>> {
    Box::new(Fail {
        code,
        template,
        params,
        span: Some(span),
        text: None,
        data: None,
        host_number: false,
        path: None,
    })
}

impl<'h> Run<'h> {
    // ---- charges ------------------------------------------------------

    fn limit(&self, code: &'static str, span: Option<Span>) -> Box<Fail<'h>> {
        let l = &self.m.limits;
        let limit = match code {
            STEPS_EXCEEDED => l.steps.map(|n| n as f64),
            ALLOCATION_EXCEEDED => l.allocation.map(|n| n as f64),
            DEPTH_EXCEEDED => Some(l.depth as f64),
            _ => None,
        };
        Box::new(Fail {
            code,
            template: "default",
            params: limit
                .map(|n| vec![("limit", number::to_text(n))])
                .unwrap_or_default(),
            span,
            text: None,
            data: None,
            host_number: false,
            path: None,
        })
    }

    fn charge(&mut self, n: u64) -> R<'h, ()> {
        self.m.charge(n).map_err(|c| self.limit(c, None))
    }

    fn alloc(&mut self, n: u64) -> R<'h, ()> {
        self.m.alloc(n).map_err(|c| self.limit(c, None))
    }

    /// The depth test of 8.4 item 7, for adding `n`.
    fn depth_test(&self, n: u32, span: Span) -> R<'h, ()> {
        if self.m.depth + n > self.m.limits.depth {
            return Err(self.limit(DEPTH_EXCEEDED, Some(span)));
        }
        Ok(())
    }

    fn host_number(&self, span: Option<Span>) -> Box<Fail<'h>> {
        Box::new(Fail {
            code: "number-range",
            template: "host",
            params: Vec::new(),
            span,
            text: None,
            data: None,
            host_number: true,
            path: None,
        })
    }

    /// Read a value: a host number that cannot enter fails here.
    fn read(&self, v: Value<'h>, span: Span) -> R<'h, Value<'h>> {
        match v {
            Value::BadNum => Err(self.host_number(Some(span))),
            v => Ok(v),
        }
    }

    fn internal(&self, span: Span) -> Box<Fail<'h>> {
        fail("internal-error", "default", span, Vec::new())
    }

    fn res(&self, id: NodeId) -> &'h Res {
        &self.p.res[id.index()]
    }

    // ---- the walk -------------------------------------------------------

    fn eval(&mut self, mut id: NodeId) -> R<'h, Value<'h>> {
        let ast = self.ast;
        loop {
            let node = &ast.nodes[id.index()];
            match &node.kind {
                NodeKind::Program { consts, result } | NodeKind::Block { consts, result } => {
                    for &c in consts {
                        self.const_line(c)?;
                    }
                    id = *result;
                }
                NodeKind::Number { value } => return Ok(Value::Num(*value)),
                NodeKind::String { value, .. } => return Ok(Value::Str(Str::Lent(value))),
                NodeKind::Bool { value } => return Ok(Value::Bool(*value)),
                NodeKind::Null => return Ok(Value::Null),
                NodeKind::Template { parts } => return self.template(node.span, parts),
                NodeKind::Ident { name } => return self.ident(id, name),
                NodeKind::List { items } => return self.list(items),
                NodeKind::Map { entries } => return self.map(entries),
                NodeKind::Unary { op, x } => {
                    let v = self.eval(*x)?;
                    return self.unary(*op, v, node.span);
                }
                NodeKind::Binary { .. } => return self.binary(id),
                NodeKind::Cond {
                    test, then, else_, ..
                } => {
                    let t = self.eval(*test)?;
                    id = if t.truthy() { *then } else { *else_ };
                }
                NodeKind::MethodCall { args, .. } if matches!(self.res(id), Res::CallHost(_)) => {
                    return self.call(id, args);
                }
                NodeKind::Member { .. }
                | NodeKind::Index { .. }
                | NodeKind::MethodCall { .. }
                | NodeKind::OptChain { .. } => return self.chain(id),
                NodeKind::Call { args, .. } => return self.call(id, args),
                NodeKind::Match { subject, arms } => {
                    id = self.match_arm(node.span, *subject, arms)?;
                }
                NodeKind::Try { body, handler, .. } => return self.try_(id, *body, *handler),
                NodeKind::As { form, x, .. } => return self.as_test(id, *form, *x, node.span),
                _ => return Err(self.internal(node.span)),
            }
        }
    }

    /// F4 5.1 and 5.2: `x as T` and `x is T`, by the walk of 4.
    fn as_test(&mut self, id: NodeId, form: AsForm, x: NodeId, span: Span) -> R<'h, Value<'h>> {
        let v = self.eval(x)?;
        let Res::As(Some(t)) = self.res(id) else {
            return Err(self.internal(span));
        };
        let r = {
            let mut m = RunMeter { m: &mut self.m };
            crate::conform::test(&v, t, &mut m)
        };
        match r {
            Err(code) => Err(self.limit(code, Some(span))),
            Ok(None) => Ok(if form == AsForm::As {
                v
            } else {
                Value::Bool(true)
            }),
            Ok(Some(_)) if form == AsForm::Is => Ok(Value::Bool(false)),
            Ok(Some(f)) => {
                let mut fl = fail("not-conforming", f.template, span, f.params);
                fl.path = Some(f.path);
                Err(fl)
            }
        }
    }

    fn const_line(&mut self, c: NodeId) -> R<'h, ()> {
        let NodeKind::Const { pattern, value } = self.ast.kind(c) else {
            return Err(self.internal(self.ast.span(c)));
        };
        if let NodeKind::Lambda { .. } = self.ast.kind(*value) {
            // A lambda evaluated: 1 step, nothing bound at run time.
            return self.charge(1);
        }
        let v = self.eval(*value)?;
        self.bind_pattern(*pattern, v)
    }

    fn ident(&mut self, id: NodeId, name: &'h str) -> R<'h, Value<'h>> {
        let span = self.ast.span(id);
        match self.res(id) {
            Res::Slot(s) => Ok(self.slots[*s as usize].clone()),
            Res::Var(v) => {
                let v = *v as usize;
                if self.cache[v].is_none() {
                    let got = self.vars.get(name).map(from_view);
                    self.cache[v] = Some(got);
                }
                match self.cache[v].clone().flatten() {
                    Some(val) => self.read(val, span),
                    None => Err(fail(
                        "missing-variable",
                        "default",
                        span,
                        vec![("name", name.to_string())],
                    )),
                }
            }
            _ => Err(self.internal(span)),
        }
    }

    // ---- literals -------------------------------------------------------

    fn template(&mut self, span: Span, parts: &'h [TemplatePart]) -> R<'h, Value<'h>> {
        let mut out = String::new();
        for p in parts {
            match p {
                TemplatePart::Text(t) => out.push_str(t),
                TemplatePart::Expr(e) => {
                    let v = self.eval(*e)?;
                    self.render(&v, &mut out, true, span)?;
                }
            }
        }
        let sz = size(&out);
        self.charge(1 + sz)?;
        self.alloc(1 + sz)?;
        Ok(Value::str(out))
    }

    /// The rendering of a value, and its walk (`02-evaluation.md` 5.5).
    fn render(&mut self, v: &Value<'h>, out: &mut String, top: bool, span: Span) -> R<'h, ()> {
        match v {
            Value::Str(s) => {
                if top {
                    out.push_str(s.as_str());
                } else {
                    out.push('"');
                    for c in s.as_str().chars() {
                        push_json_char(out, c);
                    }
                    out.push('"');
                }
            }
            Value::Num(n) => out.push_str(&number::to_text(*n)),
            Value::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
            Value::Null => {
                if !top {
                    out.push_str("null");
                }
            }
            Value::BadNum => return Err(self.host_number(Some(span))),
            Value::Opaque(_) => return Err(fail("wrong-kind", "text", span, Vec::new())),
            Value::List(_) | Value::HList(_) => {
                self.depth_test(1, span)?;
                let n = v.len().unwrap_or(0);
                self.charge(n as u64)?;
                self.m.depth += 1;
                out.push('[');
                for i in 0..n {
                    if i > 0 {
                        out.push(',');
                    }
                    let e = self.read(v.elem(i), span)?;
                    self.render(&e, out, false, span)?;
                }
                out.push(']');
                self.m.depth -= 1;
            }
            Value::Map(_) | Value::HMap(_) => {
                self.depth_test(1, span)?;
                let es = v.entries();
                let n = es.len() as u64;
                let s: u64 = es.iter().map(|(k, _)| weight(k.as_str())).sum::<u64>() / 64;
                self.charge(n + log2_ceil(n) * (n + s))?;
                self.m.depth += 1;
                out.push('{');
                for (i, (k, x)) in es.into_iter().enumerate() {
                    if i > 0 {
                        out.push(',');
                    }
                    out.push('"');
                    for c in k.as_str().chars() {
                        push_json_char(out, c);
                    }
                    out.push_str("\":");
                    let x = self.read(x, span)?;
                    self.render(&x, out, false, span)?;
                }
                out.push('}');
                self.m.depth -= 1;
            }
        }
        Ok(())
    }

    fn list(&mut self, items: &'h [NodeId]) -> R<'h, Value<'h>> {
        let mut out = Vec::with_capacity(items.len());
        for &item in items {
            if let NodeKind::Spread { value } = self.ast.kind(item) {
                let span = self.ast.span(item);
                let v = self.eval(*value)?;
                match v.kind() {
                    Kind::Null => return Err(fail("null-operand", "spread", span, vec![])),
                    Kind::List => {}
                    _ => {
                        return Err(fail(
                            "wrong-kind",
                            "spread-list",
                            span,
                            vec![("kind", kind_p(&v))],
                        ))
                    }
                }
                let n = v.len().unwrap_or(0);
                self.charge(n as u64)?;
                out.extend((0..n).map(|i| v.elem(i)));
            } else {
                out.push(self.eval(item)?);
            }
        }
        self.alloc(1 + out.len() as u64)?;
        Ok(Value::list(out))
    }

    fn map(&mut self, entries: &'h [NodeId]) -> R<'h, Value<'h>> {
        let mut es: Vec<(Str<'h>, Value<'h>)> = Vec::with_capacity(entries.len());
        for &e in entries {
            let span = self.ast.span(e);
            match self.ast.kind(e) {
                NodeKind::Spread { value } => {
                    let v = self.eval(*value)?;
                    match v.kind() {
                        Kind::Null => return Err(fail("null-operand", "spread", span, vec![])),
                        Kind::Map => {}
                        _ => {
                            return Err(fail(
                                "wrong-kind",
                                "spread-map",
                                span,
                                vec![("kind", kind_p(&v))],
                            ))
                        }
                    }
                    let copied = v.entries();
                    let s: u64 = copied.iter().map(|(k, _)| weight(k.as_str())).sum::<u64>() / 64;
                    self.charge(copied.len() as u64 + s)?;
                    es.extend(copied);
                }
                NodeKind::Entry { key, value, .. } => {
                    let k = match key {
                        EntryKey::Word(w) | EntryKey::Str(w) => Str::Lent(w.as_str()),
                        EntryKey::Computed(k) => {
                            let kv = self.eval(*k)?;
                            match kv {
                                Value::Str(s) => s,
                                Value::Null => {
                                    return Err(fail("null-operand", "key", span, vec![]))
                                }
                                other => {
                                    return Err(fail(
                                        "wrong-kind",
                                        "key",
                                        span,
                                        vec![("found", value_p(&other))],
                                    ))
                                }
                            }
                        }
                    };
                    let v = self.eval(*value)?;
                    self.charge(size(k.as_str()))?;
                    es.push((k, v));
                }
                _ => return Err(self.internal(span)),
            }
        }
        let m = MapVal::from_written(es);
        self.alloc(1 + 2 * m.0.len() as u64)?;
        Ok(Value::Map(Rc::new(m)))
    }

    // ---- operators ------------------------------------------------------

    fn unary(&mut self, op: UnaryOp, v: Value<'h>, span: Span) -> R<'h, Value<'h>> {
        match op {
            UnaryOp::Not => {
                self.charge(1)?;
                Ok(Value::Bool(!v.truthy()))
            }
            UnaryOp::Neg => match v {
                Value::Num(n) => {
                    self.charge(1)?;
                    Ok(Value::Num(number::neg(n)))
                }
                Value::Null => Err(fail(
                    "null-operand",
                    "operand",
                    span,
                    vec![("op", "-".into())],
                )),
                other => Err(fail(
                    "wrong-kind",
                    "operand",
                    span,
                    vec![("op", "-".into()), ("kind", kind_p(&other))],
                )),
            },
        }
    }

    /// A chain of binary operators along its left operands, by a loop.
    fn binary(&mut self, id: NodeId) -> R<'h, Value<'h>> {
        let mut stack = Vec::new();
        let mut n = id;
        while let NodeKind::Binary { x, .. } = self.ast.kind(n) {
            stack.push(n);
            n = *x;
        }
        let mut v = self.eval(n)?;
        while let Some(b) = stack.pop() {
            let NodeKind::Binary { op, y, .. } = self.ast.kind(b) else {
                unreachable!()
            };
            let span = self.ast.span(b);
            v = match op {
                BinaryOp::And => {
                    if v.truthy() {
                        self.eval(*y)?
                    } else {
                        v
                    }
                }
                BinaryOp::Or => {
                    if v.truthy() {
                        v
                    } else {
                        self.eval(*y)?
                    }
                }
                BinaryOp::Coalesce => {
                    if v.is_null() {
                        self.eval(*y)?
                    } else {
                        v
                    }
                }
                _ => {
                    let r = self.eval(*y)?;
                    self.binop(*op, v, r, span)?
                }
            };
        }
        Ok(v)
    }

    fn operands(&self, op: BinaryOp, a: &Value<'h>, b: &Value<'h>, span: Span) -> Box<Fail<'h>> {
        if a.is_null() || b.is_null() {
            return fail(
                "null-operand",
                "operand",
                span,
                vec![("op", op_text(op).into())],
            );
        }
        fail(
            "wrong-kind",
            "operands",
            span,
            vec![
                ("op", op_text(op).into()),
                ("left", kind_p(a)),
                ("right", kind_p(b)),
            ],
        )
    }

    fn arith(&self, op: BinaryOp, r: Result<f64, NumErr>, span: Span) -> R<'h, Value<'h>> {
        match r {
            Ok(x) => Ok(Value::Num(x)),
            Err(NumErr::Range) => Err(fail(
                "number-range",
                "operator",
                span,
                vec![("op", op_text(op).into())],
            )),
            Err(NumErr::DivZero) if op == BinaryOp::Pow => {
                Err(fail("divide-by-zero", "power", span, vec![]))
            }
            Err(NumErr::DivZero) => Err(fail(
                "divide-by-zero",
                "divisor",
                span,
                vec![("op", op_text(op).into())],
            )),
        }
    }

    fn binop(&mut self, op: BinaryOp, a: Value<'h>, b: Value<'h>, span: Span) -> R<'h, Value<'h>> {
        match op {
            BinaryOp::Eq | BinaryOp::Ne => {
                let r = self.eq(&a, &b, span)?;
                Ok(Value::Bool(r == (op == BinaryOp::Eq)))
            }
            BinaryOp::In | BinaryOp::NotIn => {
                let r = self.contains(op, &a, &b, span)?;
                Ok(Value::Bool(r == (op == BinaryOp::In)))
            }
            BinaryOp::Add => match (&a, &b) {
                (Value::Num(x), Value::Num(y)) => {
                    self.charge(1)?;
                    self.arith(op, number::add(*x, *y), span)
                }
                (Value::Str(x), Value::Str(y)) => {
                    let sz = (weight(x.as_str()) + weight(y.as_str())) / 64;
                    self.charge(1 + sz)?;
                    self.alloc(1 + sz)?;
                    let mut s = String::with_capacity(x.as_str().len() + y.as_str().len());
                    s.push_str(x.as_str());
                    s.push_str(y.as_str());
                    Ok(Value::str(s))
                }
                _ => Err(self.operands(op, &a, &b, span)),
            },
            BinaryOp::Sub | BinaryOp::Mul | BinaryOp::Div | BinaryOp::Rem | BinaryOp::Pow => {
                let (Value::Num(x), Value::Num(y)) = (&a, &b) else {
                    return Err(self.operands(op, &a, &b, span));
                };
                let (x, y) = (*x, *y);
                if op == BinaryOp::Pow && !is_whole(y) {
                    return Err(fail(
                        "wrong-kind",
                        "whole",
                        span,
                        vec![("found", value_p(&b))],
                    ));
                }
                self.charge(1)?;
                let r = match op {
                    BinaryOp::Sub => number::sub(x, y),
                    BinaryOp::Mul => number::mul(x, y),
                    BinaryOp::Div => number::div(x, y),
                    BinaryOp::Rem => number::rem(x, y),
                    _ => number::pow(x, y),
                };
                self.arith(op, r, span)
            }
            BinaryOp::Lt | BinaryOp::Le | BinaryOp::Gt | BinaryOp::Ge => {
                let ord = match (&a, &b) {
                    (Value::Num(x), Value::Num(y)) => {
                        self.charge(1)?;
                        x.partial_cmp(y)
                    }
                    (Value::Str(x), Value::Str(y)) => {
                        self.charge(1 + lcp_size(x.as_str(), y.as_str()))?;
                        Some(x.as_str().cmp(y.as_str()))
                    }
                    _ => return Err(self.operands(op, &a, &b, span)),
                };
                let ord = ord.unwrap_or(std::cmp::Ordering::Equal);
                Ok(Value::Bool(match op {
                    BinaryOp::Lt => ord.is_lt(),
                    BinaryOp::Le => ord.is_le(),
                    BinaryOp::Gt => ord.is_gt(),
                    _ => ord.is_ge(),
                }))
            }
            _ => Err(self.internal(span)),
        }
    }

    /// `==` with the charges of 5.3. A comparison of two lists of one length
    /// or two maps of one size is a walk.
    fn eq(&mut self, a: &Value<'h>, b: &Value<'h>, span: Span) -> R<'h, bool> {
        let (ka, kb) = (a.kind(), b.kind());
        if ka != kb {
            self.charge(1)?;
            return Ok(false);
        }
        match (a, b) {
            (Value::Str(x), Value::Str(y)) => {
                self.charge(1 + lcp_size(x.as_str(), y.as_str()))?;
                Ok(x.as_str() == y.as_str())
            }
            _ if ka == Kind::List => {
                let n = a.len().unwrap_or(0);
                if n != b.len().unwrap_or(0) {
                    self.charge(1)?;
                    return Ok(false);
                }
                self.depth_test(1, span)?;
                self.m.depth += 1;
                let mut equal = true;
                for i in 0..n {
                    self.charge(1)?;
                    let x = self.read(a.elem(i), span)?;
                    let y = self.read(b.elem(i), span)?;
                    if !self.eq(&x, &y, span)? {
                        equal = false;
                        break;
                    }
                }
                self.m.depth -= 1;
                Ok(equal)
            }
            _ if ka == Kind::Map => {
                let n = a.len().unwrap_or(0);
                if n != b.len().unwrap_or(0) {
                    self.charge(1)?;
                    return Ok(false);
                }
                self.depth_test(1, span)?;
                self.m.depth += 1;
                let es = a.entries();
                let s: u64 = es.iter().map(|(k, _)| weight(k.as_str())).sum::<u64>() / 64;
                self.charge(s)?;
                let mut equal = true;
                for (k, x) in es {
                    self.charge(1)?;
                    match b.field(k.as_str()) {
                        Some(y) => {
                            let x = self.read(x, span)?;
                            let y = self.read(y, span)?;
                            if !self.eq(&x, &y, span)? {
                                equal = false;
                            }
                        }
                        None => equal = false,
                    }
                }
                self.m.depth -= 1;
                Ok(equal)
            }
            _ => {
                self.charge(1)?;
                Ok(scalar_eq(a, b))
            }
        }
    }

    /// `x in y` (`01-language.md` 6.8; `02-evaluation.md` 5.3).
    fn contains(&mut self, op: BinaryOp, x: &Value<'h>, y: &Value<'h>, span: Span) -> R<'h, bool> {
        let opp = || vec![("op", op_text(op).to_string())];
        match y.kind() {
            Kind::Null => Err(fail("null-operand", "operand", span, opp())),
            Kind::List => {
                let n = y.len().unwrap_or(0);
                for i in 0..n {
                    let e = self.read(y.elem(i), span)?;
                    self.charge(1)?;
                    let equal = match (x, &e) {
                        (Value::Str(a), Value::Str(b)) => {
                            self.charge(lcp_size(a.as_str(), b.as_str()))?;
                            a.as_str() == b.as_str()
                        }
                        _ if x.kind() == e.kind() && matches!(x.kind(), Kind::List | Kind::Map) => {
                            if x.len() == e.len() {
                                self.eq(x, &e, span)?
                            } else {
                                false
                            }
                        }
                        _ => x.kind() == e.kind() && scalar_eq(x, &e),
                    };
                    if equal {
                        return Ok(true);
                    }
                }
                Ok(false)
            }
            Kind::String | Kind::Map => {
                let Value::Str(xs) = x else {
                    return Err(self.operands(op, x, y, span));
                };
                let xs = xs.as_str();
                if let Value::Str(ys) = y {
                    let ys = ys.as_str();
                    self.charge(1 + size(ys))?;
                    // Ruling AC11: a needle longer than the text is answered
                    // from the lengths. `str::contains` is linear.
                    Ok(xs.len() <= ys.len() && ys.contains(xs))
                } else {
                    self.charge(1 + size(xs))?;
                    Ok(y.field(xs).is_some())
                }
            }
            _ => Err(fail(
                "wrong-kind",
                "operands",
                span,
                vec![
                    ("op", op_text(op).into()),
                    ("left", kind_p(x)),
                    ("right", kind_p(y)),
                ],
            )),
        }
    }

    // ---- chains -----------------------------------------------------------

    fn chain(&mut self, top: NodeId) -> R<'h, Value<'h>> {
        let inner = match self.ast.kind(top) {
            NodeKind::OptChain { x } => *x,
            _ => top,
        };
        let mut steps = Vec::new();
        let mut n = inner;
        loop {
            match self.ast.kind(n) {
                NodeKind::Member { x, .. } | NodeKind::Index { x, .. } => {
                    steps.push(n);
                    n = *x;
                }
                NodeKind::MethodCall { x, .. } if !matches!(self.res(n), Res::CallHost(_)) => {
                    steps.push(n);
                    n = *x;
                }
                _ => break,
            }
        }
        let mut v = self.eval(n)?;
        for &s in steps.iter().rev() {
            match self.step(s, v)? {
                Some(nv) => v = nv,
                None => return Ok(Value::Null),
            }
        }
        Ok(v)
    }

    /// One step of a chain. `None` is a null receiver of an optional step:
    /// the whole chain is null.
    fn step(&mut self, s: NodeId, v: Value<'h>) -> R<'h, Option<Value<'h>>> {
        let ast = self.ast;
        let span = ast.span(s);
        match &ast.nodes[s.index()].kind {
            NodeKind::Member { name, optional, .. } => {
                if v.is_null() {
                    if *optional {
                        return Ok(None);
                    }
                    return Err(fail(
                        "null-operand",
                        "field",
                        span,
                        vec![("name", name.clone())],
                    ));
                }
                if v.kind() != Kind::Map {
                    return Err(fail(
                        "wrong-kind",
                        "field",
                        span,
                        vec![("name", name.clone()), ("kind", kind_p(&v))],
                    ));
                }
                self.charge(1 + size(name))?;
                match v.field(name) {
                    Some(x) => Ok(Some(self.read(x, span)?)),
                    None if *optional => Ok(Some(Value::Null)),
                    None => Err(fail(
                        "missing-field",
                        "field",
                        span,
                        vec![("name", name.clone())],
                    )),
                }
            }
            NodeKind::Index {
                index, optional, ..
            } => {
                if v.is_null() {
                    if *optional {
                        return Ok(None);
                    }
                    return Err(fail("null-operand", "index", span, vec![]));
                }
                let k = v.kind();
                if !matches!(k, Kind::List | Kind::String | Kind::Map) {
                    return Err(fail(
                        "wrong-kind",
                        "index",
                        span,
                        vec![("kind", kind_p(&v))],
                    ));
                }
                let iv = self.eval(*index)?;
                if iv.is_null() {
                    return Err(fail("null-operand", "key", span, vec![]));
                }
                match k {
                    Kind::Map => {
                        let Value::Str(key) = &iv else {
                            return Err(fail(
                                "wrong-kind",
                                "key",
                                span,
                                vec![("found", value_p(&iv))],
                            ));
                        };
                        self.charge(1 + size(key.as_str()))?;
                        match v.field(key.as_str()) {
                            Some(x) => Ok(Some(self.read(x, span)?)),
                            None if *optional => Ok(Some(Value::Null)),
                            None => Err(fail(
                                "missing-field",
                                "key",
                                span,
                                vec![("key", json_string(key.as_str()))],
                            )),
                        }
                    }
                    _ => {
                        let i = match iv {
                            Value::Num(i) if is_whole(i) => i,
                            other => {
                                return Err(fail(
                                    "wrong-kind",
                                    "whole",
                                    span,
                                    vec![("found", value_p(&other))],
                                ))
                            }
                        };
                        if let Value::Str(st) = &v {
                            let st = st.as_str();
                            self.charge(1 + size(st))?;
                            let found = if i >= 0.0 {
                                st.chars().nth(i as usize)
                            } else {
                                None
                            };
                            match found {
                                Some(c) => {
                                    self.alloc(1)?;
                                    Ok(Some(Value::str(c.to_string())))
                                }
                                None if *optional => Ok(Some(Value::Null)),
                                None => Err(fail(
                                    "index-range",
                                    "index",
                                    span,
                                    vec![
                                        ("index", number::to_text(i)),
                                        ("kind", "string".into()),
                                        ("length", value::cp_len(st).to_string()),
                                    ],
                                )),
                            }
                        } else {
                            self.charge(1)?;
                            let n = v.len().unwrap_or(0);
                            if i >= 0.0 && (i as usize) < n {
                                Ok(Some(self.read(v.elem(i as usize), span)?))
                            } else if *optional {
                                Ok(Some(Value::Null))
                            } else {
                                Err(fail(
                                    "index-range",
                                    "index",
                                    span,
                                    vec![
                                        ("index", number::to_text(i)),
                                        ("kind", "list".into()),
                                        ("length", n.to_string()),
                                    ],
                                ))
                            }
                        }
                    }
                }
            }
            NodeKind::MethodCall {
                name,
                args,
                optional,
                ..
            } => {
                if v.is_null() {
                    if *optional {
                        return Ok(None);
                    }
                    return Err(fail(
                        "null-operand",
                        "method",
                        span,
                        vec![("name", name.name.clone())],
                    ));
                }
                self.method(s, v, name, args, span).map(Some)
            }
            _ => Err(self.internal(span)),
        }
    }

    /// Whether an argument is a lambda: written in place, or the name of a
    /// `const`-bound lambda.
    fn lambda_of(&self, a: NodeId) -> Option<NodeId> {
        match (self.ast.kind(a), self.res(a)) {
            (NodeKind::Lambda { .. }, _) => Some(a),
            (_, Res::LambdaRef(l)) => Some(*l),
            _ => None,
        }
    }

    fn lambda_params(&self, l: NodeId) -> usize {
        match self.ast.kind(l) {
            NodeKind::Lambda { params, .. } => params.len(),
            _ => 0,
        }
    }

    fn eval_args(&mut self, args: &'h [NodeId]) -> R<'h, Vec<Arg<'h>>> {
        let mut out = Vec::with_capacity(args.len());
        for &a in args {
            if let NodeKind::Lambda { .. } = self.ast.kind(a) {
                self.charge(1)?;
                out.push(Arg::Lam(a));
            } else if let Res::LambdaRef(l) = self.res(a) {
                out.push(Arg::Lam(*l));
            } else {
                out.push(Arg::Val(self.eval(a)?));
            }
        }
        Ok(out)
    }

    /// A method call: the method by the receiver's kind, the shape test,
    /// the arguments, then the entry (`04-library.md` 2.2, ruling AC1).
    fn method(
        &mut self,
        s: NodeId,
        recv: Value<'h>,
        name: &'h SpannedName,
        args: &'h [NodeId],
        span: Span,
    ) -> R<'h, Value<'h>> {
        let kind = recv.kind();
        let nm = || ("name", name.name.clone());
        let no_method = || {
            fail(
                "wrong-kind",
                "method",
                span,
                vec![("kind", kind.name().into()), nm()],
            )
        };
        let count = || {
            fail(
                "wrong-kind",
                "count",
                span,
                vec![("count", args.len().to_string()), nm()],
            )
        };
        if let Value::Opaque(o) = &recv {
            let env = &self.p.env;
            let Some(mi) = env.method(&o.type_name, &name.name) else {
                return Err(no_method());
            };
            let m = &env.data().methods[mi];
            let n = args.len();
            if n < m.sig.required() || m.sig.at_most().is_some_and(|a| n > a) {
                return Err(count());
            }
            for (i, &a) in args.iter().enumerate() {
                if self.lambda_of(a).is_some() {
                    return Err(fail(
                        "wrong-kind",
                        "not-callback",
                        span,
                        vec![("count", (i + 1).to_string()), nm()],
                    ));
                }
            }
            let mut vals = vec![recv.clone()];
            for &a in args {
                vals.push(self.eval(a)?);
            }
            self.charge(1)?;
            let imp = m.imp.clone();
            let label = format!("{}.{}", m.type_name, m.name);
            return self.invoke_host(imp, &vals, &label, span);
        }
        let idx = match kind {
            Kind::String | Kind::List | Kind::Map => self.p.env.lib_method(kind, &name.name),
            _ => None,
        };
        let Some(idx) = idx else {
            return Err(no_method());
        };
        let rec = &crate::library::records()[idx];
        let n = args.len();
        if n < rec.sig.required() || rec.sig.at_most().is_some_and(|a| n > a) {
            return Err(count());
        }
        for (i, &a) in args.iter().enumerate() {
            let lam = self.lambda_of(a).is_some();
            match (rec.callback(i), lam) {
                (Some(_), false) => {
                    return Err(fail(
                        "wrong-kind",
                        "callback",
                        span,
                        vec![("count", (i + 1).to_string()), nm()],
                    ))
                }
                (None, true) => {
                    return Err(fail(
                        "wrong-kind",
                        "not-callback",
                        span,
                        vec![("count", (i + 1).to_string()), nm()],
                    ))
                }
                _ => {}
            }
        }
        for (i, &a) in args.iter().enumerate() {
            if let (Some(k), Some(l)) = (rec.callback(i), self.lambda_of(a)) {
                if self.lambda_params(l) > k {
                    return Err(fail(
                        "wrong-kind",
                        "parameters",
                        span,
                        vec![nm(), ("count", k.to_string())],
                    ));
                }
            }
        }
        let vals = self.eval_args(args)?;
        let measure = self.p.measure[s.index()] as u32;
        self.lib_method(idx, recv, vals, span, measure)
    }

    // ---- calls ------------------------------------------------------------

    fn call(&mut self, id: NodeId, args: &'h [NodeId]) -> R<'h, Value<'h>> {
        let span = self.ast.span(id);
        match self.res(id) {
            Res::CallLambda(l) => {
                let l = *l;
                let mut vals = Vec::with_capacity(args.len());
                for &a in args {
                    vals.push(self.eval(a)?);
                }
                self.charge(1)?;
                let measure = self.p.measure[id.index()] as u32;
                self.invoke(l, &vals, measure, span)
            }
            Res::CallLib(i) => {
                let i = *i as usize;
                let vals = self.eval_args(args)?;
                self.lib_function(i, vals, span)
            }
            Res::CallHost(i) => {
                let f = &self.p.env.data().functions[*i as usize];
                let mut vals = Vec::with_capacity(args.len());
                for &a in args {
                    vals.push(self.eval(a)?);
                }
                self.charge(1)?;
                self.invoke_host(f.imp.clone(), &vals, &f.name, span)
            }
            _ => Err(self.internal(span)),
        }
    }

    fn invoke_host(
        &mut self,
        imp: Option<crate::env::HostImpl>,
        vals: &[Value<'h>],
        name: &str,
        span: Span,
    ) -> R<'h, Value<'h>> {
        let Some(imp) = imp else {
            return Err(self.internal(span));
        };
        let r = {
            let mut call = Call { m: &mut self.m };
            catch_unwind(AssertUnwindSafe(|| imp(&mut call, vals)))
        };
        if let Some(c) = self.m.stop.take() {
            return Err(self.limit(c, Some(span)));
        }
        match r {
            Err(_) => Err(fail(
                "function-panic",
                "default",
                span,
                vec![("name", name.to_string())],
            )),
            Ok(Err(e)) => {
                let mut f = fail("function-error", "default", span, vec![]);
                f.text = Some(e.0);
                Err(f)
            }
            Ok(Ok(d)) => self.read(Value::from_data(&d), span),
        }
    }

    /// Call a lambda: by name after its step, or by the library after the two
    /// steps of the element. The depth test, the parameters, the body.
    fn invoke(
        &mut self,
        l: NodeId,
        args: &[Value<'h>],
        measure: u32,
        span: Span,
    ) -> R<'h, Value<'h>> {
        self.depth_test(measure, span)?;
        self.m.depth += measure;
        let ast = self.ast;
        let NodeKind::Lambda { params, body } = &ast.nodes[l.index()].kind else {
            return Err(self.internal(span));
        };
        for (i, &p) in params.iter().enumerate() {
            let NodeKind::Param { pattern, default } = ast.kind(p) else {
                continue;
            };
            let a = args.get(i).cloned();
            let v = match (a, default) {
                (Some(v), _) if !v.is_null() => v,
                (_, Some(d)) => self.eval(*d)?,
                (a, None) => a.unwrap_or(Value::Null),
            };
            self.bind_pattern(*pattern, v)?;
        }
        let v = self.eval(*body)?;
        self.m.depth -= measure;
        Ok(v)
    }

    // ---- patterns ---------------------------------------------------------

    fn bind_pattern(&mut self, pat: NodeId, v: Value<'h>) -> R<'h, ()> {
        let ast = self.ast;
        let span = ast.span(pat);
        match &ast.nodes[pat.index()].kind {
            NodeKind::NamePat { .. } => {
                if let Res::Bind(s) = self.res(pat) {
                    self.slots[*s as usize] = v;
                }
                Ok(())
            }
            NodeKind::MapPat { fields, rest } => {
                match v.kind() {
                    Kind::Null => return Err(fail("null-operand", "pattern", span, vec![])),
                    Kind::Map => {}
                    _ => {
                        return Err(fail(
                            "wrong-kind",
                            "pattern-map",
                            span,
                            vec![("kind", kind_p(&v))],
                        ))
                    }
                }
                let (slots, rest_slot) = match self.res(pat) {
                    Res::PatSlots { fields, rest } => (fields.as_slice(), *rest),
                    _ => (&[][..], None),
                };
                for (i, f) in fields.iter().enumerate() {
                    self.charge(1 + size(&f.key))?;
                    let got = v.field(&f.key);
                    let val = match got {
                        Some(x) if !x.is_null() => self.read(x, f.span)?,
                        other => match f.default {
                            Some(d) => self.eval(d)?,
                            None if other.is_some() => Value::Null,
                            None => {
                                return Err(fail(
                                    "missing-field",
                                    "pattern",
                                    f.span,
                                    vec![("key", json_string(&f.key))],
                                ))
                            }
                        },
                    };
                    if let Some(Some(s)) = slots.get(i) {
                        self.slots[*s as usize] = val;
                    }
                }
                if let Some(r) = rest {
                    let named: Vec<&str> = fields.iter().map(|f| f.key.as_str()).collect();
                    let left: Vec<(Str<'h>, Value<'h>)> = v
                        .entries()
                        .into_iter()
                        .filter(|(k, _)| !named.contains(&k.as_str()))
                        .collect();
                    let s: u64 = left.iter().map(|(k, _)| weight(k.as_str())).sum::<u64>() / 64;
                    self.charge(left.len() as u64 + s)?;
                    self.alloc(1 + 2 * left.len() as u64)?;
                    let _ = r;
                    if let Some(slot) = rest_slot {
                        self.slots[slot as usize] = Value::Map(Rc::new(MapVal(left)));
                    }
                }
                Ok(())
            }
            NodeKind::ListPat { elems, rest } => {
                match v.kind() {
                    Kind::Null => return Err(fail("null-operand", "pattern", span, vec![])),
                    Kind::List => {}
                    _ => {
                        return Err(fail(
                            "wrong-kind",
                            "pattern-list",
                            span,
                            vec![("kind", kind_p(&v))],
                        ))
                    }
                }
                let n = v.len().unwrap_or(0);
                for (i, e) in elems.iter().enumerate() {
                    self.charge(1)?;
                    let got = if i < n {
                        Some(self.read(v.elem(i), e.span)?)
                    } else {
                        None
                    };
                    let val = match got {
                        Some(x) if !x.is_null() => x,
                        other => match e.default {
                            Some(d) => self.eval(d)?,
                            None if other.is_some() => Value::Null,
                            None => {
                                return Err(fail(
                                    "index-range",
                                    "pattern",
                                    e.span,
                                    vec![("length", n.to_string()), ("index", i.to_string())],
                                ))
                            }
                        },
                    };
                    self.bind_pattern(e.pat, val)?;
                }
                if rest.is_some() {
                    let left: Vec<Value<'h>> = (elems.len()..n).map(|i| v.elem(i)).collect();
                    self.charge(left.len() as u64)?;
                    self.alloc(1 + left.len() as u64)?;
                    if let Res::PatSlots {
                        rest: Some(slot), ..
                    } = self.res(pat)
                    {
                        self.slots[*slot as usize] = Value::list(left);
                    }
                }
                Ok(())
            }
            _ => Err(self.internal(span)),
        }
    }

    // ---- match and try ----------------------------------------------------

    fn match_arm(&mut self, span: Span, subject: NodeId, arms: &'h [NodeId]) -> R<'h, NodeId> {
        let s = self.eval(subject)?;
        for &a in arms {
            let NodeKind::Arm { literals, result } = self.ast.kind(a) else {
                continue;
            };
            self.charge(1)?;
            let Some(lits) = literals else {
                return Ok(*result);
            };
            for l in lits {
                let equal = match (&l.lit, &s) {
                    (Lit::Str(x), Value::Str(y)) => {
                        self.charge(lcp_size(x, y.as_str()))?;
                        x == y.as_str()
                    }
                    (Lit::Number(x), Value::Num(y)) => x == y,
                    (Lit::Bool(x), Value::Bool(y)) => x == y,
                    (Lit::Null, Value::Null) => true,
                    _ => false,
                };
                if equal {
                    return Ok(*result);
                }
            }
        }
        Err(fail(
            "no-match",
            "default",
            span,
            vec![("found", value_p(&s))],
        ))
    }

    fn try_(&mut self, id: NodeId, body: NodeId, handler: NodeId) -> R<'h, Value<'h>> {
        let depth = self.m.depth;
        match self.eval(body) {
            Ok(v) => Ok(v),
            Err(f) if messages::caught(f.code) => {
                self.m.depth = depth;
                if let Res::Catch(Some(slot)) = self.res(id) {
                    let slot = *slot as usize;
                    let msg = f.message();
                    if f.text.is_some() {
                        self.alloc(9)?;
                    } else {
                        let sz = size(&msg);
                        self.charge(1 + sz)?;
                        self.alloc(9 + 1 + sz)?;
                    }
                    // F4 5.5: the path of a `not-conforming` failure, 1 + n
                    // units.
                    let path = match &f.path {
                        Some(p) => {
                            self.alloc(1 + p.len() as u64)?;
                            Value::list(
                                p.iter()
                                    .map(|s| match s {
                                        crate::diag::PathStep::Key(k) => Value::str(k.clone()),
                                        crate::diag::PathStep::Index(i) => Value::Num(*i as f64),
                                    })
                                    .collect(),
                            )
                        }
                        None => Value::Null,
                    };
                    let entries = vec![
                        (Str::Lent("code"), Value::Str(Str::Lent(f.code))),
                        (Str::Lent("data"), f.data.clone().unwrap_or(Value::Null)),
                        (Str::Lent("message"), Value::str(msg)),
                        (Str::Lent("path"), path),
                    ];
                    self.slots[slot] = Value::Map(Rc::new(MapVal(entries)));
                }
                self.eval(handler)
            }
            Err(f) => Err(f),
        }
    }
}

fn scalar_eq(a: &Value<'_>, b: &Value<'_>) -> bool {
    match (a, b) {
        (Value::Null, Value::Null) => true,
        (Value::Bool(x), Value::Bool(y)) => x == y,
        (Value::Num(x), Value::Num(y)) => x == y,
        (Value::Str(x), Value::Str(y)) => x.as_str() == y.as_str(),
        (Value::Opaque(x), Value::Opaque(y)) => x.same(y),
        _ => false,
    }
}

/// The JSON text of a result (`03-host.md` 3.2.4): keys in code point order,
/// numbers in the rendering of `01-language.md`. It fails with
/// `wrong-kind` on an opaque value and `depth-exceeded` past depth 200.
pub fn to_json(v: &Value<'_>) -> Result<String, &'static str> {
    fn go(v: &Value<'_>, out: &mut String, depth: u32) -> Result<(), &'static str> {
        match v {
            Value::Null => out.push_str("null"),
            Value::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
            Value::Num(n) => out.push_str(&number::to_text(*n)),
            Value::BadNum => return Err("number-range"),
            Value::Str(s) => {
                out.push('"');
                for c in s.as_str().chars() {
                    push_json_char(out, c);
                }
                out.push('"');
            }
            Value::Opaque(_) => return Err("wrong-kind"),
            Value::List(_) | Value::HList(_) => {
                if depth + 1 > 200 {
                    return Err(DEPTH_EXCEEDED);
                }
                out.push('[');
                for i in 0..v.len().unwrap_or(0) {
                    if i > 0 {
                        out.push(',');
                    }
                    go(&v.elem(i), out, depth + 1)?;
                }
                out.push(']');
            }
            Value::Map(_) | Value::HMap(_) => {
                if depth + 1 > 200 {
                    return Err(DEPTH_EXCEEDED);
                }
                out.push('{');
                for (i, (k, x)) in v.entries().iter().enumerate() {
                    if i > 0 {
                        out.push(',');
                    }
                    go(&Value::Str(k.clone()), out, depth + 1)?;
                    out.push(':');
                    go(x, out, depth + 1)?;
                }
                out.push('}');
            }
        }
        Ok(())
    }
    let mut out = String::new();
    go(v, &mut out, 0)?;
    Ok(out)
}

impl<'h> Value<'h> {
    /// An owned copy of a value as host data, for a host that keeps a result.
    pub fn owned(&self) -> Data {
        self.to_data()
    }
}

impl Program {
    /// The class of a run failure (`03-host.md` 3.7.6, `05-types.md` 18.5):
    /// by the checked column when the program was compiled with a check.
    pub fn class_of(&self, f: &Fail<'_>) -> Option<&'static str> {
        let Some(sites) = &self.sites else {
            return f.class();
        };
        let at = |class: &str| f.span.is_some_and(|s| self.at_site(sites, class, s));
        // Ruling CH1: a site of class `index`, `divide`, `value` or `host`
        // is on the node that can fail; a failure is at it only when it is
        // on the failing node itself.
        let on = |class: &str| {
            f.span
                .is_some_and(|s| sites.iter().any(|x| x.class == class && x.span == s))
        };
        match f.code {
            "null-operand" | "wrong-kind" | "missing-field" => Some(if at("dynamic") {
                "dynamic"
            } else {
                "environment"
            }),
            "no-match" => Some("environment"),
            // Ruling CE2: the mark `!` is a promise. Away from a `host` site
            // a function or method not marked `!` broke it.
            "function-error" => Some(if on("host") { "host" } else { "environment" }),
            "not-conforming" => Some(if on("value") { "value" } else { "environment" }),
            _ => f.class(),
        }
    }

    /// Whether a failure at a node's span is at a site of a class: the site is
    /// on the failing node, or on its operand, receiver, index or argument
    /// (`05-types.md` 18.3). Ruling CH1: this reach is for `dynamic` sites.
    fn at_site(&self, sites: &[crate::check::Site], class: &str, span: Span) -> bool {
        let ast = &self.parsed.ast;
        let mut spans = vec![span];
        for (i, n) in ast.nodes.iter().enumerate() {
            if n.span != span {
                continue;
            }
            let mut kids: Vec<NodeId> = Vec::new();
            match &n.kind {
                NodeKind::Unary { x, .. }
                | NodeKind::Member { x, .. }
                | NodeKind::OptChain { x } => kids.push(*x),
                NodeKind::Binary { x, y, .. } => kids.extend([*x, *y]),
                NodeKind::Index { x, index, .. } => kids.extend([*x, *index]),
                NodeKind::MethodCall { x, args, .. } => {
                    kids.push(*x);
                    kids.extend(args.iter().copied());
                    kids.extend(self.lambda_bodies(args));
                }
                NodeKind::Call { args, .. } => {
                    kids.extend(args.iter().copied());
                    kids.extend(self.lambda_bodies(args));
                }
                NodeKind::Spread { value } => kids.push(*value),
                NodeKind::Entry { key, value, .. } => {
                    if let EntryKey::Computed(k) = key {
                        kids.push(*k);
                    }
                    kids.push(*value);
                }
                NodeKind::Template { parts } => kids.extend(parts.iter().filter_map(|p| match p {
                    TemplatePart::Expr(e) => Some(*e),
                    _ => None,
                })),
                NodeKind::Match { subject, .. } => kids.push(*subject),
                NodeKind::MapPat { .. } | NodeKind::ListPat { .. } => {
                    // The value bound by a `const` is the pattern's operand.
                    for c in &ast.nodes {
                        if let NodeKind::Const { pattern, value } = &c.kind {
                            if pattern.index() == i {
                                kids.push(*value);
                            }
                        }
                    }
                }
                _ => {}
            }
            spans.extend(kids.iter().map(|k| ast.span(*k)));
        }
        // Ruling AO6 (c): a field or an element of a pattern that fails is at
        // a site on its key or name, or on the value the pattern binds.
        for (i, n) in ast.nodes.iter().enumerate() {
            let hit = match &n.kind {
                NodeKind::MapPat { fields, .. } => {
                    fields.iter().find(|f| f.span == span).map(|f| f.key_span)
                }
                NodeKind::ListPat { elems, .. } => elems
                    .iter()
                    .find(|e| e.span == span)
                    .map(|e| ast.span(e.pat)),
                _ => None,
            };
            if let Some(k) = hit {
                spans.push(k);
                spans.push(n.span);
                for c in &ast.nodes {
                    if let NodeKind::Const { pattern, value } = &c.kind {
                        if pattern.index() == i {
                            spans.push(ast.span(*value));
                        }
                    }
                }
            }
        }
        sites
            .iter()
            .any(|s| s.class == class && spans.contains(&s.span))
    }

    /// Ruling CB3: the body of each lambda that is an argument, or that an
    /// argument names, for a failure of a callback's result, key or value.
    fn lambda_bodies(&self, args: &[NodeId]) -> Vec<NodeId> {
        let ast = &self.parsed.ast;
        let mut out = Vec::new();
        for &a in args {
            let l = match (&ast.nodes[a.index()].kind, &self.res[a.index()]) {
                (NodeKind::Lambda { .. }, _) => a,
                (_, Res::LambdaRef(l)) => *l,
                _ => continue,
            };
            if let NodeKind::Lambda { body, .. } = &ast.nodes[l.index()].kind {
                out.push(*body);
            }
        }
        out
    }

    /// The diagnostic of a run failure, with its class by the program's
    /// check.
    pub fn diagnostic(&self, f: &Fail<'_>) -> Diagnostic {
        let mut d = f.diagnostic(&self.parsed.source);
        d.class = self.class_of(f);
        d
    }
}
