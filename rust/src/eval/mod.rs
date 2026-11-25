//! The evaluator (`02-evaluation.md`).
//!
//! A walk over the syntax tree with the resolution of `compile`. Each form
//! charges as the tables of sections 5 and 6 say, in the order of section 2.
//! What opens no level runs in a loop: a left-associative chain of operators,
//! the value of an `as` or `is`, the steps of a chain with its projections,
//! the `else` of `? :` and of `if`, `const` lines and `match` arms. So the
//! evaluator's recursion follows the source nesting and
//! the calls in progress, which the depth limit bounds (8.5 item 3).

pub(crate) mod code;
mod groups;
pub mod lib;

use crate::compile::{Frame, Place, Program, Res};
use crate::diag::{Diagnostic, Phase};
use crate::env::Limits;
use crate::host::{Data, Vars};
use crate::messages::{self, json_string, push_json_char};
use crate::number::{self, NumErr};
use crate::source::{Range, Source};
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
    #[inline]
    fn charge(&mut self, n: u64) -> Result<(), &'static str> {
        // Saturating: a host's charge near u64::MAX cannot wrap the count
        // past the limit; with no limit, the count stops at u64::MAX.
        let steps = self.steps.saturating_add(n);
        if let Some(l) = self.limits.steps {
            if steps > l {
                return Err(STEPS_EXCEEDED);
            }
        }
        self.steps = steps;
        if steps >= self.next_check {
            return self.check();
        }
        Ok(())
    }

    /// The tests of 8.3, at least once in every 1,024 steps.
    #[cold]
    #[inline(never)]
    fn check(&mut self) -> Result<(), &'static str> {
        self.next_check = self.steps.saturating_add(1024);
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
        Ok(())
    }

    #[inline]
    fn alloc(&mut self, n: u64) -> Result<(), &'static str> {
        let units = self.units.saturating_add(n);
        if let Some(l) = self.limits.allocation {
            if units > l {
                return Err(ALLOCATION_EXCEEDED);
            }
        }
        self.units = units;
        Ok(())
    }
}

/// What is left of a run's budget (`03-host.md` 3.5.2 item 3): each limit
/// less what the run has charged. `None`: the limit is off. The fields are
/// those of `Limits`, so a nested run takes them as they are.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Remaining {
    pub steps: Option<u64>,
    pub allocation: Option<u64>,
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

    /// The steps and units left in the run, counting the call's own step
    /// and what the function has charged. A charge that failed charged
    /// nothing, so it does not change this.
    pub fn remaining(&self) -> Remaining {
        Remaining {
            steps: self.m.limits.steps.map(|l| l.saturating_sub(self.m.steps)),
            allocation: self
                .m
                .limits
                .allocation
                .map(|l| l.saturating_sub(self.m.units)),
        }
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
    pub span: Option<Range>,
    /// The message of `failed` and `function-error`, which no template words.
    pub text: Option<String>,
    pub data: Option<Value<'h>>,
    /// A `number-range` from a host number: class `environment`.
    pub host_number: bool,
    /// The path of a `not-conforming` failure (F4 5.1).
    pub path: Option<Vec<crate::diag::PathStep>>,
    /// The calls in progress, the innermost first: each call adds its entry
    /// as the failure leaves it (`f08-errors.md` 4). Built only on failure.
    pub trace: Vec<(Range, String, Option<usize>)>,
    /// The inner diagnostic of a `function-error` (`03-host.md` 3.5.3).
    pub cause: Option<Box<Diagnostic>>,
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
        d.cause = self.cause.clone();
        if !self.trace.is_empty() {
            d.trace = Some(Box::new(
                self.trace
                    .iter()
                    .rev()
                    .map(|(s, name, index)| crate::diag::TraceEntry {
                        range: *s,
                        span: src.line_span(*s),
                        name: name.clone(),
                        index: *index,
                    })
                    .collect(),
            ));
        }
        d
    }
}

pub(crate) type R<'h, T> = Result<T, Box<Fail<'h>>>;

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
    pub(crate) fn outcome<'h>(&'h self, vars: &'h dyn Vars) -> Outcome<'h> {
        self.outcome_with(vars, RunOptions::default())
    }

    pub(crate) fn outcome_with<'h>(&'h self, vars: &'h dyn Vars, opts: RunOptions) -> Outcome<'h> {
        let limits = self.env.data().limits;
        let nvars = self.vars_read;
        let root = self.frames[self.parsed.ast.root.index()];
        let n = nvars + root.size as usize;
        // A program with no slots and no variables takes no buffers.
        let any = nvars > 0 || !self.place.is_empty();
        let slots = if any {
            SLOTS.with(|c| c.take())
        } else {
            Vec::new()
        };
        let mut display = DISPLAY.with(|c| c.take());
        display.clear();
        display.resize(self.max_depth + 1, 0);
        display[0] = nvars;
        let mut run = Run {
            p: self,
            ast: &self.parsed.ast,
            vars,
            // The variables as they are read, then the program's frame;
            // each call pushes its own frame after them (F17 11.5).
            slots,
            display,
            cb_frames: Vec::new(),
            closures: Vec::new(),
            closure_display: Vec::new(),
            display_saved: Vec::new(),
            vars_read: Bits::new(nvars),
            vars_missing: Bits::new(nvars),
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
            json_mode: false,
            scratch: String::new(),
            in_buf: Vec::new(),
            parts_buf: Vec::new(),
        };
        if n > 0 {
            run.slots.resize_with(n, || Value::Null);
        }
        // `02-evaluation.md` 8.4: handing the result over is not a read, so the run
        // makes no walk of it.
        let result = run.eval(self.parsed.ast.root);
        if any {
            let slots = recast(std::mem::take(&mut run.slots));
            SLOTS.with(|c| c.set(slots));
        }
        let display = std::mem::take(&mut run.display);
        DISPLAY.with(|c| c.set(display));
        let mut scratch = std::mem::take(&mut run.scratch);
        if scratch.capacity() > 0 && scratch.capacity() <= 65536 {
            scratch.clear();
            SCRATCH.with(|c| c.set(scratch));
        }
        Outcome {
            usage: Usage {
                steps: run.m.steps,
                allocated: run.m.units,
            },
            result,
        }
    }

    /// F16 12.5: positional call of an `export const` entry point.
    pub(crate) fn call_outcome_with<'h>(
        &'h self,
        name: &str,
        args: &[&'h dyn crate::host::Host],
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> (Result<Value<'h>, crate::diag::Error>, Usage) {
        use crate::diag::{code, Diagnostic, Error, PathStep, Phase};
        let Some(entry) = self.entries.iter().find(|e| e.name == name) else {
            return (
                Err(Error::one(Diagnostic::bare(
                    Phase::Declare,
                    code::BAD_NAME,
                    format!("{name} is not an entry point of the program"),
                ))),
                Usage::default(),
            );
        };
        let max = entry.params.len();
        if args.len() < entry.min_args || args.len() > max {
            return (
                Err(Error::one(Diagnostic::bare(
                    Phase::Declare,
                    code::ARITY,
                    format!(
                        "entry point {name} expects between {} and {max} arguments, got {}",
                        entry.min_args,
                        args.len()
                    ),
                ))),
                Usage::default(),
            );
        }
        let mut arg_vals: Vec<Value<'h>> = args.iter().map(|a| from_view(a.view())).collect();
        // F16 12.5 step 3: argument conformance with ConformAll (max 100 across all args).
        let mut diags: Vec<Diagnostic> = Vec::new();
        for (i, (val, param)) in arg_vals.iter().zip(&entry.params).enumerate() {
            let remaining = 100usize.saturating_sub(diags.len());
            if remaining == 0 {
                break;
            }
            let param_ty = if param.optional {
                param.ty.clone().with_null()
            } else {
                param.ty.clone()
            };
            let fs =
                crate::conform::all(val, &param_ty, remaining, Some(crate::conform::FREE_STEPS));
            let mut hit_limit = false;
            for f in &fs {
                let mut d = f.diagnostic();
                if d.code == "not-conforming" {
                    let mut p = vec![PathStep::Index(i)];
                    if let Some(inner) = d.path.take() {
                        p.extend(*inner);
                    }
                    d.path = Some(Box::new(p));
                    diags.push(d);
                } else {
                    diags.push(d);
                    hit_limit = true;
                    break;
                }
            }
            if hit_limit || diags.len() >= 100 {
                break;
            }
        }
        if !diags.is_empty() {
            release_all(&mut arg_vals);
            return (
                Err(Error {
                    phase: Phase::Run,
                    diagnostics: diags,
                }),
                Usage::default(),
            );
        }
        self.eval_entry(entry, arg_vals, vars, opts)
    }

    /// F16 12.5: call by record of an `export const` entry point.
    pub(crate) fn call_record_outcome_with<'h>(
        &'h self,
        name: &str,
        input: &'h dyn crate::host::Host,
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> (Result<Value<'h>, crate::diag::Error>, Usage) {
        use crate::diag::{code, Diagnostic, Error, Phase};
        let Some(entry) = self.entries.iter().find(|e| e.name == name) else {
            return (
                Err(Error::one(Diagnostic::bare(
                    Phase::Declare,
                    code::BAD_NAME,
                    format!("{name} is not an entry point of the program"),
                ))),
                Usage::default(),
            );
        };
        let Some(input_ty) = &entry.input_type else {
            return (
                Err(Error::one(Diagnostic::bare(
                    Phase::Declare,
                    code::NO_INPUT_RECORD,
                    format!("entry point {name} has no input record"),
                ))),
                Usage::default(),
            );
        };
        let input_val = from_view(input.view());
        let fs = crate::conform::all(&input_val, input_ty, 100, Some(crate::conform::FREE_STEPS));
        if !fs.is_empty() {
            let diags = fs.iter().map(crate::conform::Failure::diagnostic).collect();
            value::release(input_val);
            return (
                Err(Error {
                    phase: Phase::Run,
                    diagnostics: diags,
                }),
                Usage::default(),
            );
        }
        let arg_vals = if entry.input_is_row1 {
            vec![input_val]
        } else {
            let mut vals = Vec::with_capacity(entry.params.len());
            for p in &entry.params {
                let k = p.name.as_deref().expect("row 2 parameter has a name");
                vals.push(input_val.field(k).unwrap_or(Value::Null));
            }
            value::release(input_val);
            vals
        };
        self.eval_entry(entry, arg_vals, vars, opts)
    }

    /// F16 12.4: evaluate an entry point's needed non-lambda top-level `const`
    /// lines in text order, charge 1 step, add 1 to depth, bind arguments, and
    /// evaluate its body, with no `trace` entry for the host's call.
    fn eval_entry<'h>(
        &'h self,
        entry: &'h crate::compile::CompiledEntry,
        mut arg_vals: Vec<Value<'h>>,
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> (Result<Value<'h>, crate::diag::Error>, Usage) {
        let limits = self.env.data().limits;
        let nvars = self.vars_read;
        let root = self.frames[self.parsed.ast.root.index()];
        let n = nvars + root.size as usize;
        let any = nvars > 0 || !self.place.is_empty();
        let slots = if any {
            SLOTS.with(|c| c.take())
        } else {
            Vec::new()
        };
        let mut display = DISPLAY.with(|c| c.take());
        display.clear();
        display.resize(self.max_depth + 1, 0);
        display[0] = nvars;
        let mut run = Run {
            p: self,
            ast: &self.parsed.ast,
            vars,
            slots,
            display,
            cb_frames: Vec::new(),
            closures: Vec::new(),
            closure_display: Vec::new(),
            display_saved: Vec::new(),
            vars_read: Bits::new(nvars),
            vars_missing: Bits::new(nvars),
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
            json_mode: false,
            scratch: String::new(),
            in_buf: Vec::new(),
            parts_buf: Vec::new(),
        };
        if n > 0 {
            run.slots.resize_with(n, || Value::Null);
        }
        let result = run.exec_entry(entry, &mut arg_vals);
        release_all(&mut arg_vals);
        if any {
            let slots = recast(std::mem::take(&mut run.slots));
            SLOTS.with(|c| c.set(slots));
        }
        let display = std::mem::take(&mut run.display);
        DISPLAY.with(|c| c.set(display));
        let mut scratch = std::mem::take(&mut run.scratch);
        if scratch.capacity() > 0 && scratch.capacity() <= 65536 {
            scratch.clear();
            SCRATCH.with(|c| c.set(scratch));
        }
        let usage = Usage {
            steps: run.m.steps,
            allocated: run.m.units,
        };
        let result = result.map_err(|f| crate::diag::Error::one(self.diagnostic(&f)));
        (result, usage)
    }
}

/// A lambda passed to a library entry (`Run::callback`): when its
/// parameters are all plain names, their slots, its body and the body's
/// closure.
pub(crate) struct Callback<'h> {
    l: NodeId,
    /// F17 13.4: the closure of a lambda bound to a function parameter.
    closure: Option<u32>,
    frame: Frame,
    /// For the fast path: where the frame the entry's calls share starts.
    base: usize,
    #[allow(clippy::type_complexity)]
    fast: Option<(&'h [Option<u32>], NodeId, Option<&'h code::Code>)>,
}

/// A lambda to call: its node and, for the lambda bound to a function
/// parameter (F17 13.4), its closure, an index into `Run::closures`.
#[derive(Clone, Copy)]
pub(crate) struct Fun {
    l: NodeId,
    closure: Option<u32>,
}

/// An argument of a library call: a value, or a lambda.
#[derive(Clone)]
enum Arg<'h> {
    Val(Value<'h>),
    Lam(Fun),
}

/// The arguments of a library call (`Run::args`): up to three in place,
/// with their count, or more in a vector.
enum Args<'h> {
    Few([Arg<'h>; 3], usize),
    Many(Vec<Arg<'h>>),
}

impl<'h> Args<'h> {
    fn new() -> Args<'h> {
        let none = Fun {
            l: NodeId(0),
            closure: None,
        };
        Args::Few([Arg::Lam(none), Arg::Lam(none), Arg::Lam(none)], 0)
    }

    fn as_slice(&self) -> &[Arg<'h>] {
        match self {
            Args::Few(a, n) => &a[..*n],
            Args::Many(v) => v,
        }
    }
}

pub(crate) struct Run<'h> {
    p: &'h Program,
    ast: &'h Ast,
    vars: &'h dyn Vars,
    /// The variables the program reads, then the frames of the calls in
    /// progress, the program's first (F17 11.5).
    slots: Vec<Value<'h>>,
    /// For each depth of frame, where the frame on the chain of the code
    /// now running starts in `slots`. A call of a lambda sets the entry of
    /// its depth, and gives it back when it ends: the entries below it are
    /// those of the code in which the lambda was written, since a lambda
    /// runs only where it is in scope and never outlives that code.
    display: Vec<usize>,
    /// The frames that library entries in progress share among the calls
    /// of their callbacks, with the entry of `display` each replaced.
    cb_frames: Vec<(Frame, usize)>,
    /// F17 13.4: the lambdas bound to the function parameters of the calls
    /// in progress, each with where its entries of `display` start in
    /// `closure_display`: the entries below its frame's depth as they were
    /// where it was evaluated, which a call of it puts back while it runs.
    /// A function parameter's slot holds the index of its closure here, as
    /// a number; static rule 4 keeps any read of it as a value from
    /// compiling. A call of a higher-order lambda drops what its arguments
    /// added when it ends.
    closures: Vec<(NodeId, u32)>,
    closure_display: Vec<usize>,
    /// The entries of `display` that a call of a closure replaced.
    display_saved: Vec<usize>,
    /// The variables asked of the host, and those it did not have: a run
    /// asks for each name at most once (`03-host.md` 3.4.1).
    vars_read: Bits,
    vars_missing: Bits,
    m: Meter,
    /// F6: the rendering walk of `json.stringify`, whose opaque failure has
    /// the template `json` (`f06-groups.md` 6.1 item 5).
    json_mode: bool,
    /// The elements of a list literal tested by `in`, kept for reuse.
    in_buf: Vec<Value<'h>>,
    /// The values of a template's parts, kept for reuse.
    parts_buf: Vec<Value<'h>>,
    /// A buffer for building text, kept for reuse.
    scratch: String,
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

#[cold]
#[inline(never)]
fn fail<'h>(
    code: &'static str,
    template: &'static str,
    span: Range,
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
        cause: None,
        path: None,
        trace: Vec::new(),
    })
}

/// A library entry's trace entry, added as a failure leaves its call of a
/// lambda at the element `index` (`f08-errors.md` 4.1, 4.2).
#[cold]
#[inline(never)]
fn traced<'h>(mut f: Box<Fail<'h>>, span: Range, entry: &str, index: usize) -> Box<Fail<'h>> {
    f.trace.push((span, entry.to_string(), Some(index)));
    f
}

/// `function-error` from a host function's error (`03-host.md` 3.5.3): the
/// text, or the message of the inner diagnostic, which is kept as the
/// cause. The error carries the inner diagnostic's `data` (F19 5.1): for
/// `failed` and for a `function-error` that carries data, that data; else
/// null.
#[cold]
#[inline(never)]
fn function_error<'h>(e: crate::host::HostError, span: Range) -> Box<Fail<'h>> {
    let mut f = fail("function-error", "default", span, vec![]);
    f.text = Some(e.message());
    if let crate::host::HostError::Risor(inner) = e {
        f.cause = inner.diagnostics.into_iter().next().map(Box::new);
        if let Some(c) = f.cause.as_deref() {
            if matches!(c.code, "failed" | "function-error") {
                f.data = c.data.as_deref().map(Value::from_data);
            }
        }
    }
    f
}

/// The arguments a call did not bind, released.
#[inline(never)]
fn release_all(args: &mut [Value<'_>]) {
    for a in args.iter_mut() {
        value::release(std::mem::replace(a, Value::Null));
    }
}

impl<'h> Run<'h> {
    // ---- charges ------------------------------------------------------

    #[cold]
    #[inline(never)]
    fn limit(&self, code: &'static str, span: Option<Range>) -> Box<Fail<'h>> {
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
            cause: None,
            path: None,
            trace: Vec::new(),
        })
    }

    #[inline]
    fn charge(&mut self, n: u64) -> R<'h, ()> {
        self.m.charge(n).map_err(|c| self.limit(c, None))
    }

    #[inline]
    fn alloc(&mut self, n: u64) -> R<'h, ()> {
        self.m.alloc(n).map_err(|c| self.limit(c, None))
    }

    /// The depth test of 8.4 item 7, for adding `n`.
    fn depth_test(&self, n: u32, span: Range) -> R<'h, ()> {
        if self.m.depth + n > self.m.limits.depth {
            return Err(self.limit(DEPTH_EXCEEDED, Some(span)));
        }
        Ok(())
    }

    #[cold]
    #[inline(never)]
    fn host_number(&self, span: Option<Range>) -> Box<Fail<'h>> {
        Box::new(Fail {
            code: "number-range",
            template: "host",
            params: Vec::new(),
            span,
            text: None,
            data: None,
            host_number: true,
            cause: None,
            path: None,
            trace: Vec::new(),
        })
    }

    /// Read a value: a host number that cannot enter fails here.
    fn read(&self, v: Value<'h>, span: Range) -> R<'h, Value<'h>> {
        match v {
            Value::BadNum => Err(self.host_number(Some(span))),
            v => Ok(v),
        }
    }

    #[cold]
    #[inline(never)]
    fn internal(&self, span: Range) -> Box<Fail<'h>> {
        fail("internal-error", "default", span, Vec::new())
    }

    fn res(&self, id: NodeId) -> &'h Res {
        &self.p.res[id.index()]
    }

    // ---- the walk -------------------------------------------------------

    /// Evaluate a node: by its closure when it has one (`code.rs`), else by
    /// the walk.
    #[inline]
    fn eval(&mut self, id: NodeId) -> R<'h, Value<'h>> {
        if let Some(Some(c)) = self.p.code.get(id.index()) {
            return c(self);
        }
        self.walk(id)
    }

    #[inline(never)]
    fn walk(&mut self, mut id: NodeId) -> R<'h, Value<'h>> {
        let ast = self.ast;
        let mut first = true;
        loop {
            if !first {
                if let Some(Some(c)) = self.p.code.get(id.index()) {
                    return c(self);
                }
            }
            first = false;
            // Stack: this frame is live at every level the walk recurses
            // through, so each arm is one call, and what an arm would keep
            // in this frame is in the function it calls.
            let node = &ast.nodes[id.index()];
            let next = match &node.kind {
                NodeKind::Program {
                    consts,
                    result: Some(result),
                    ..
                }
                | NodeKind::Block { consts, result } => self.const_lines(consts, *result),
                NodeKind::Template { parts } => return self.template(node.span, parts),
                NodeKind::List { items } => return self.list(items),
                NodeKind::Map { entries } => return self.map(entries),
                NodeKind::Unary { op, x } => return self.unary_node(*op, *x, node.span),
                NodeKind::Binary { .. } => return self.binary(id),
                NodeKind::Cond {
                    test, then, else_, ..
                } => self.branch(*test, *then, *else_),
                NodeKind::MethodCall { args, .. }
                    if matches!(self.res(id), Res::CallHost(_) | Res::CallLib(_)) =>
                {
                    return self.call(id, args);
                }
                NodeKind::Member { .. } if matches!(self.res(id), Res::Constant(_)) => {
                    return self.leaf(id)
                }
                NodeKind::Member { .. }
                | NodeKind::Index { .. }
                | NodeKind::MethodCall { .. }
                | NodeKind::OptChain { .. }
                | NodeKind::Each { .. } => return self.chain(id),
                NodeKind::Call { args, .. } => return self.call(id, args),
                NodeKind::Match { subject, arms } => self.match_arm(node.span, *subject, arms),
                NodeKind::Try { body, handler, .. } => return self.try_(id, *body, *handler),
                NodeKind::As { form, x, .. } => return self.as_test(id, *form, *x),
                _ => return self.leaf(id),
            };
            id = next?;
        }
    }

    /// A node of the walk that has no children: a literal, a name or a
    /// constant (F6 5.3: its read charges nothing).
    #[inline(never)]
    fn leaf(&mut self, id: NodeId) -> R<'h, Value<'h>> {
        let node = &self.ast.nodes[id.index()];
        match &node.kind {
            NodeKind::Number { value } => Ok(Value::Num(*value)),
            NodeKind::String { value, .. } => Ok(Value::Str(Str::Lent(value))),
            NodeKind::Bool { value } => Ok(Value::Bool(*value)),
            NodeKind::Null => Ok(Value::Null),
            NodeKind::Ident { .. } => self.ident(id),
            NodeKind::Member { .. } => match self.res(id) {
                Res::Constant(v) => Ok(Value::Num(*v)),
                _ => Err(self.internal(node.span)),
            },
            _ => Err(self.internal(node.span)),
        }
    }

    /// The `const` lines of a program or a block, then its result, which the
    /// walk evaluates next.
    #[inline(never)]
    fn const_lines(&mut self, consts: &'h [NodeId], result: NodeId) -> R<'h, NodeId> {
        for &c in consts {
            self.const_line(c)?;
        }
        Ok(result)
    }

    /// A prefix operator and its operand.
    #[inline(never)]
    fn unary_node(&mut self, op: UnaryOp, x: NodeId, span: Range) -> R<'h, Value<'h>> {
        let v = self.eval(x)?;
        self.unary(op, v, span)
    }

    /// The test of a conditional, and the branch it chooses, which the walk
    /// evaluates next.
    #[inline(never)]
    fn branch(&mut self, test: NodeId, then: NodeId, else_: NodeId) -> R<'h, NodeId> {
        let t = self.eval(test)?;
        Ok(if t.truthy() { then } else { else_ })
    }

    /// F4 5.1 and 5.2: `x as T` and `x is T`, by the walk of 4.
    fn as_test(&mut self, id: NodeId, form: AsForm, x: NodeId) -> R<'h, Value<'h>> {
        // A value that is itself an `as`, an `is` or an operator opens no
        // level (F4 2.8): the loop of the spine evaluates it.
        if self.on_spine(x) {
            return self.spine(id);
        }
        let v = self.eval(x)?;
        self.as_step(id, form, v)
    }

    /// One `as` or `is` of a spine, its value evaluated.
    #[inline(never)]
    fn as_step(&mut self, id: NodeId, form: AsForm, v: Value<'h>) -> R<'h, Value<'h>> {
        let span = self.ast.span(id);
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
        let NodeKind::Const { pattern, value, .. } = self.ast.kind(c) else {
            return Err(self.internal(self.ast.span(c)));
        };
        if let NodeKind::Lambda { .. } = self.ast.kind(*value) {
            // A lambda evaluated: 1 step, nothing bound at run time.
            return self.charge(1);
        }
        let v = self.eval(*value)?;
        self.bind_pattern(*pattern, v)
    }

    /// The read of the variable at place `v` among those the program reads
    /// (`Program::var_index`), written at `id`. The host is asked for it
    /// at its first read only.
    #[inline]
    fn var(&mut self, v: usize, id: NodeId) -> R<'h, Value<'h>> {
        if !self.vars_read.test(v) {
            self.fetch_var(v, id);
        }
        match &self.slots[v] {
            _ if self.vars_missing.test(v) => Err(self.missing_var(id)),
            Value::BadNum => Err(self.host_number(Some(self.ast.span(id)))),
            x => Ok(x.dup()),
        }
    }

    #[inline(never)]
    fn fetch_var(&mut self, v: usize, id: NodeId) {
        self.vars_read.set(v);
        let NodeKind::Ident { name } = self.ast.kind(id) else {
            return;
        };
        match self.vars.get(name) {
            Some(got) => value::put(&mut self.slots[v], from_view(got)),
            None => self.vars_missing.set(v),
        }
    }

    #[cold]
    #[inline(never)]
    fn missing_var(&self, id: NodeId) -> Box<Fail<'h>> {
        let name = match self.ast.kind(id) {
            NodeKind::Ident { name } => name.clone(),
            _ => String::new(),
        };
        fail(
            "missing-variable",
            "default",
            self.ast.span(id),
            vec![("name", name)],
        )
    }

    fn ident(&mut self, id: NodeId) -> R<'h, Value<'h>> {
        let span = self.ast.span(id);
        match self.res(id) {
            Res::Slot(s) => Ok(self.slots[self.slot(*s)].dup()),
            Res::Var(v) => self.var(self.p.var_index[*v as usize] as usize, id),
            _ => Err(self.internal(span)),
        }
    }

    // ---- literals -------------------------------------------------------

    fn template(&mut self, span: Range, parts: &'h [TemplatePart]) -> R<'h, Value<'h>> {
        let mut out = Rendering::new(self.take_scratch());
        let mut vals = std::mem::take(&mut self.parts_buf);
        for p in parts {
            match p {
                TemplatePart::Text(t) => out.push(t),
                TemplatePart::Expr(e) => {
                    let v = self.eval(*e)?;
                    self.render(&v, &mut out, true, span)?;
                    vals.push(v);
                }
            }
        }
        let texts = parts.iter().map(|p| match p {
            TemplatePart::Text(t) => Some(t.as_str()),
            TemplatePart::Expr(_) => None,
        });
        self.template_done(out, vals, texts, span)
    }

    /// The run's buffer for building text, empty: the thread's at its first
    /// use in a run.
    fn take_scratch(&mut self) -> String {
        let mut out = std::mem::take(&mut self.scratch);
        if out.capacity() == 0 {
            out = SCRATCH.with(|c| c.take());
        }
        out.clear();
        out
    }

    /// The end of a template: the charge, then the string (`02-evaluation.md`
    /// 5.5). `vals` are the values of its expression parts, and `texts` its
    /// parts in order, `None` for an expression part. The buffers go back to
    /// the run.
    fn template_done<'a>(
        &mut self,
        out: Rendering,
        mut vals: Vec<Value<'h>>,
        texts: impl Iterator<Item = Option<&'a str>>,
        span: Range,
    ) -> R<'h, Value<'h>> {
        let sz = out.weight() / 64;
        self.charge(1 + sz)?;
        self.alloc(1 + sz)?;
        let s = self.rendered(out, |r, o| {
            let mut vs = vals.iter();
            for t in texts {
                match t {
                    Some(t) => o.push(t),
                    None => r.render(vs.next().unwrap_or(&Value::Null), o, true, span)?,
                }
            }
            Ok(())
        })?;
        let v = Value::Str(Str::Rc(Rc::from(s.as_str())));
        self.scratch = s;
        for v in vals.drain(..) {
            crate::value::release(v);
        }
        self.parts_buf = vals;
        Ok(v)
    }

    /// The text of a rendering whose charges are made: the walk's, or when
    /// the walk did not keep it, the text that `again` renders once more,
    /// with no charges.
    fn rendered(
        &mut self,
        out: Rendering,
        again: impl FnOnce(&mut Self, &mut Rendering) -> R<'h, ()>,
    ) -> R<'h, String> {
        if out.keep_to > 0 {
            return Ok(out.buf);
        }
        let mut p = Rendering {
            buf: String::with_capacity(out.n),
            keep_to: usize::MAX,
            w: 0,
            n: 0,
        };
        again(self, &mut p)?;
        Ok(p.buf)
    }

    /// The rendering of a value, and its walk (`02-evaluation.md` 5.5). The
    /// second rendering of `Run::rendered` charges nothing.
    fn render(&mut self, v: &Value<'h>, out: &mut Rendering, top: bool, span: Range) -> R<'h, ()> {
        match v {
            Value::Str(s) => {
                if top {
                    out.push(s.as_str());
                } else {
                    out.quote(s.as_str());
                }
            }
            Value::Num(n) => out.number(*n),
            Value::Bool(b) => out.push(if *b { "true" } else { "false" }),
            Value::Null => {
                if !top {
                    out.push("null");
                }
            }
            Value::BadNum => return Err(self.host_number(Some(span))),
            Value::Opaque(_) => {
                let t = if self.json_mode { "json" } else { "text" };
                return Err(fail("wrong-kind", t, span, Vec::new()));
            }
            Value::List(_) | Value::HList(_) => {
                let n = v.len().unwrap_or(0);
                let paid = out.paid();
                if !paid {
                    self.depth_test(1, span)?;
                    self.charge(n as u64)?;
                    self.m.depth += 1;
                }
                out.push("[");
                for i in 0..n {
                    if i > 0 {
                        out.push(",");
                    }
                    let e = self.read(v.elem(i), span)?;
                    self.render(&e, out, false, span)?;
                }
                out.push("]");
                if !paid {
                    self.m.depth -= 1;
                }
            }
            Value::Map(_) | Value::HMap(_) => {
                let paid = out.paid();
                if !paid {
                    self.depth_test(1, span)?;
                }
                let es = v.entries();
                if !paid {
                    let n = es.len() as u64;
                    let s: u64 = es.iter().map(|(k, _)| weight(k.as_str())).sum::<u64>() / 64;
                    self.charge(n + log2_ceil(n) * (n + s))?;
                    self.m.depth += 1;
                }
                out.push("{");
                for (i, (k, x)) in es.into_iter().enumerate() {
                    if i > 0 {
                        out.push(",");
                    }
                    out.quote(k.as_str());
                    out.push(":");
                    let x = self.read(x, span)?;
                    self.render(&x, out, false, span)?;
                }
                out.push("}");
                if !paid {
                    self.m.depth -= 1;
                }
            }
        }
        Ok(())
    }

    /// A list literal. Stack: the frame is live while each item is
    /// evaluated, so a spread and the building are functions of their own.
    fn list(&mut self, items: &'h [NodeId]) -> R<'h, Value<'h>> {
        // The items' values; a spread's is its list. The list's length is
        // found before the list is built (`02-evaluation.md` 6.2 item 3).
        let mut vals = Vec::with_capacity(items.len());
        let mut spreads = false;
        let mut n = 0usize;
        for &item in items {
            if let NodeKind::Spread { value } = self.ast.kind(item) {
                n += self.list_spread(item, *value, &mut vals)?;
                spreads = true;
            } else {
                vals.push(self.eval(item)?);
                n += 1;
            }
        }
        self.list_done(items, vals, spreads, n)
    }

    /// A spread of a list literal: its list, tested and charged, onto
    /// `vals`, and its length.
    #[inline(never)]
    fn list_spread(
        &mut self,
        item: NodeId,
        value: NodeId,
        vals: &mut Vec<Value<'h>>,
    ) -> R<'h, usize> {
        let span = self.ast.span(item);
        let v = self.eval(value)?;
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
        let k = v.len().unwrap_or(0);
        self.charge(k as u64)?;
        vals.push(v);
        Ok(k)
    }

    /// The end of a list literal: its units, then the list of `n` elements
    /// from the items' values.
    #[inline(never)]
    fn list_done(
        &mut self,
        items: &'h [NodeId],
        vals: Vec<Value<'h>>,
        spreads: bool,
        n: usize,
    ) -> R<'h, Value<'h>> {
        self.alloc(1 + n as u64)?;
        if !spreads {
            return Ok(Value::list(vals));
        }
        let mut out = Vec::with_capacity(n);
        for (&item, v) in items.iter().zip(vals) {
            if let NodeKind::Spread { .. } = self.ast.kind(item) {
                out.extend((0..v.len().unwrap_or(0)).map(|i| v.elem(i)));
            } else {
                out.push(v);
            }
        }
        Ok(Value::list(out))
    }

    /// A map literal. Stack: the frame is live while each entry is
    /// evaluated, so an entry, a spread and the building are functions of
    /// their own.
    fn map(&mut self, entries: &'h [NodeId]) -> R<'h, Value<'h>> {
        // The entries written, in order: a key and its value, or a spread's
        // map, with its index in `spreads`.
        let mut es: Vec<(Str<'h>, Value<'h>)> = Vec::with_capacity(entries.len());
        let mut spreads: Vec<usize> = Vec::new();
        let mut nk = 0usize; // the keys written, at least the map's entries
        for &e in entries {
            match self.ast.kind(e) {
                NodeKind::Spread { value } => {
                    nk += self.map_spread(e, *value, &mut es, &mut spreads)?;
                }
                NodeKind::Entry { key, value, .. } => {
                    self.map_entry(e, key, *value, &mut es)?;
                    nk += 1;
                }
                _ => return Err(self.internal(self.ast.span(e))),
            }
        }
        self.map_done(es, spreads, nk)
    }

    /// A spread of a map literal: its map, tested and charged, onto `es`,
    /// its place onto `spreads`, and its count of keys.
    #[inline(never)]
    fn map_spread(
        &mut self,
        e: NodeId,
        value: NodeId,
        es: &mut Vec<(Str<'h>, Value<'h>)>,
        spreads: &mut Vec<usize>,
    ) -> R<'h, usize> {
        let span = self.ast.span(e);
        let v = self.eval(value)?;
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
        let mut keys = 0usize;
        let mut w = 0u64;
        v.for_each_key(|k| {
            keys += 1;
            w += weight(k);
        });
        self.charge(keys as u64 + w / 64)?;
        spreads.push(es.len());
        es.push((Str::Lent(""), v));
        Ok(keys)
    }

    /// An entry of a map literal: its key, its value, then the key's charge.
    #[inline(never)]
    fn map_entry(
        &mut self,
        e: NodeId,
        key: &'h EntryKey,
        value: NodeId,
        es: &mut Vec<(Str<'h>, Value<'h>)>,
    ) -> R<'h, ()> {
        let k = match key {
            EntryKey::Word(w) | EntryKey::Str(w) => Str::Lent(w.as_str()),
            EntryKey::Computed(k) => self.computed_key(e, *k)?,
        };
        let v = self.eval(value)?;
        self.charge(size(k.as_str()))?;
        es.push((k, v));
        Ok(())
    }

    /// A computed key `[k]` of the map entry `e`.
    #[inline(never)]
    fn computed_key(&mut self, e: NodeId, k: NodeId) -> R<'h, Str<'h>> {
        let span = self.ast.span(e);
        let kv = self.eval(k)?;
        match kv {
            Value::Str(s) => Ok(s),
            Value::Null => Err(fail("null-operand", "key", span, vec![])),
            other => Err(fail(
                "wrong-kind",
                "key",
                span,
                vec![("found", value_p(&other))],
            )),
        }
    }

    /// The end of a map literal: its units, then the map, from the entries
    /// written and the spreads at the places `spreads`. `nk` is the count
    /// of keys written.
    #[inline(never)]
    fn map_done(
        &mut self,
        es: Vec<(Str<'h>, Value<'h>)>,
        spreads: Vec<usize>,
        nk: usize,
    ) -> R<'h, Value<'h>> {
        if spreads.is_empty() {
            let m = MapVal::from_written(es);
            self.alloc(1 + 2 * m.0.len() as u64)?;
            return Ok(Value::Map(Rc::new(m)));
        }
        // The map's units come before the map (`02-evaluation.md` 6.2 item
        // 3). When nk keys could pass the limit, the number of entries is
        // found first, without building: the keys written, each counted once.
        // The pass is linear in nk, which the spreads have charged as steps.
        let is_spread = |i: usize| spreads.binary_search(&i).is_ok();
        let room = self
            .m
            .limits
            .allocation
            .map(|l| l.saturating_sub(self.m.units));
        let mut size = None;
        if room.is_some_and(|room| 1 + 2 * nk as u64 > room) {
            let mut seen = std::collections::HashSet::with_capacity(es.len());
            for (i, (key, v)) in es.iter().enumerate() {
                if is_spread(i) {
                    v.for_each_key(|k| {
                        seen.insert(k);
                    });
                } else {
                    seen.insert(key.as_str());
                }
            }
            self.alloc(1 + 2 * seen.len() as u64)?;
            size = Some(seen.len());
        }
        // The entries written, in order: scratch space for the stable sort
        // of `MapVal::from_written`.
        let mut written = Vec::with_capacity(nk);
        for (i, (key, v)) in es.into_iter().enumerate() {
            if is_spread(i) {
                written.extend(v.entries());
            } else {
                written.push((key, v));
            }
        }
        let m = MapVal::from_written_sized(written, size);
        if size.is_none() {
            self.alloc(1 + 2 * m.0.len() as u64)?;
        }
        Ok(Value::Map(Rc::new(m)))
    }

    // ---- operators ------------------------------------------------------

    fn unary(&mut self, op: UnaryOp, v: Value<'h>, span: Range) -> R<'h, Value<'h>> {
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

    /// A binary operator and its operands.
    fn binary(&mut self, id: NodeId) -> R<'h, Value<'h>> {
        let NodeKind::Binary { x, .. } = self.ast.kind(id) else {
            unreachable!()
        };
        // The common case: a left operand that is not itself an operator,
        // an `as` or an `is`.
        if !matches!(
            self.ast.kind(*x),
            NodeKind::Binary { .. } | NodeKind::As { .. }
        ) {
            let v = self.eval(*x)?;
            return self.binary_step(id, v);
        }
        self.spine(id)
    }

    /// Whether the value of an `as` or `is` goes on with its spine: an
    /// operator, an `as` or an `is` that has no closure of its own.
    #[inline]
    fn on_spine(&self, x: NodeId) -> bool {
        matches!(
            self.ast.kind(x),
            NodeKind::Binary { .. } | NodeKind::As { .. }
        ) && !matches!(self.p.code.get(x.index()), Some(Some(_)))
    }

    /// A spine: the left operands of operators and the values of `as` and
    /// `is`, from the node `id` down. They open no level (13.1, F4 2.8), so
    /// a loop goes down the spine and comes back up, and a long one costs
    /// no stack.
    #[inline(never)]
    fn spine(&mut self, id: NodeId) -> R<'h, Value<'h>> {
        let mut stack = Spine::new();
        let mut n = id;
        loop {
            match self.ast.kind(n) {
                NodeKind::Binary { x, .. } => {
                    stack.push(n);
                    n = *x;
                }
                NodeKind::As { x, .. } => {
                    stack.push(n);
                    n = *x;
                    if !self.on_spine(n) {
                        break;
                    }
                }
                _ => break,
            }
        }
        let mut v = self.eval(n)?;
        while let Some(b) = stack.pop() {
            v = match self.ast.kind(b) {
                NodeKind::As { form, .. } => self.as_step(b, *form, v)?,
                _ => self.binary_step(b, v)?,
            };
        }
        Ok(v)
    }

    /// One operator of a chain, its left operand evaluated.
    #[inline]
    fn binary_step(&mut self, b: NodeId, v: Value<'h>) -> R<'h, Value<'h>> {
        let NodeKind::Binary { op, y, .. } = self.ast.kind(b) else {
            unreachable!()
        };
        let op = *op;
        match op {
            BinaryOp::And => {
                if v.truthy() {
                    self.eval(*y)
                } else {
                    Ok(v)
                }
            }
            BinaryOp::Or => {
                if v.truthy() {
                    Ok(v)
                } else {
                    self.eval(*y)
                }
            }
            BinaryOp::Coalesce => {
                if v.is_null() {
                    self.eval(*y)
                } else {
                    Ok(v)
                }
            }
            _ => {
                let r = self.eval(*y)?;
                self.operate(op, v, r, self.ast.span(b))
            }
        }
    }

    /// An operator that is not `&&`, `||` or `??`, on its two operands.
    #[inline]
    fn operate(
        &mut self,
        op: BinaryOp,
        v: Value<'h>,
        r: Value<'h>,
        span: Range,
    ) -> R<'h, Value<'h>> {
        // Two numbers: the charge of 1, then the operation, as `binop` does
        // it.
        if let (Value::Num(a), Value::Num(c)) = (&v, &r) {
            let (a, c) = (*a, *c);
            let quick = match op {
                BinaryOp::Lt => Some(Value::Bool(a < c)),
                BinaryOp::Le => Some(Value::Bool(a <= c)),
                BinaryOp::Gt => Some(Value::Bool(a > c)),
                BinaryOp::Ge => Some(Value::Bool(a >= c)),
                BinaryOp::Eq => Some(Value::Bool(a == c)),
                BinaryOp::Ne => Some(Value::Bool(a != c)),
                // A result that fails is left to `binop`, which charges
                // first and then fails.
                BinaryOp::Add => number::add(a, c).ok().map(Value::Num),
                BinaryOp::Sub => number::sub(a, c).ok().map(Value::Num),
                BinaryOp::Mul => number::mul(a, c).ok().map(Value::Num),
                BinaryOp::Div => number::div(a, c).ok().map(Value::Num),
                BinaryOp::Rem => number::rem(a, c).ok().map(Value::Num),
                _ => None,
            };
            if let Some(q) = quick {
                self.charge(1)?;
                return Ok(q);
            }
        }
        // Two strings, `==` or `!=`: as `eq` charges them.
        if let (Value::Str(a), Value::Str(c), BinaryOp::Eq | BinaryOp::Ne) = (&v, &r, op) {
            let (a, c) = (a.as_str(), c.as_str());
            self.charge(1 + lcp_size(a, c))?;
            return Ok(Value::Bool((a == c) == (op == BinaryOp::Eq)));
        }
        self.binop(op, v, r, span)
    }

    #[cold]
    #[inline(never)]
    fn operands(&self, op: BinaryOp, a: &Value<'h>, b: &Value<'h>, span: Range) -> Box<Fail<'h>> {
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

    fn arith(&self, op: BinaryOp, r: Result<f64, NumErr>, span: Range) -> R<'h, Value<'h>> {
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

    #[inline(never)]
    fn binop(&mut self, op: BinaryOp, a: Value<'h>, b: Value<'h>, span: Range) -> R<'h, Value<'h>> {
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
                    let (x, y) = (x.as_str(), y.as_str());
                    let sz = if x.len() + y.len() < 64 {
                        0
                    } else {
                        (weight(x) + weight(y)) / 64
                    };
                    self.charge(1 + sz)?;
                    self.alloc(1 + sz)?;
                    // Joined in the run's text buffer, then copied once.
                    let mut out = self.take_scratch();
                    out.push_str(x);
                    out.push_str(y);
                    let v = Value::Str(Str::Rc(Rc::from(out.as_str())));
                    self.scratch = out;
                    Ok(v)
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
    fn eq(&mut self, a: &Value<'h>, b: &Value<'h>, span: Range) -> R<'h, bool> {
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

    /// `x in [a, b, ...]` with no spread: the items evaluated, the list's
    /// units charged, then each element tested, as for the list built.
    fn in_literal(&mut self, x: &Value<'h>, items: &[code::Code], span: Range) -> R<'h, bool> {
        let mut vals = std::mem::take(&mut self.in_buf);
        vals.clear();
        let found = self.in_items(x, items, &mut vals, span);
        vals.clear();
        self.in_buf = vals;
        found
    }

    fn in_items(
        &mut self,
        x: &Value<'h>,
        items: &[code::Code],
        vals: &mut Vec<Value<'h>>,
        span: Range,
    ) -> R<'h, bool> {
        for i in items {
            vals.push(i(self)?);
        }
        self.alloc(1 + vals.len() as u64)?;
        for e in vals.iter() {
            if self.in_elem(x, e, span)? {
                return Ok(true);
            }
        }
        Ok(false)
    }

    /// `x in [a, b, ...]` where every item is a scalar written in place:
    /// the list's units, then each item as `in_elem` tests it.
    fn in_scalars(&mut self, x: &Value<'h>, items: &[code::Scalar], units: u64) -> R<'h, bool> {
        self.alloc(units)?;
        for e in items {
            self.charge(1)?;
            let equal = match (x, e) {
                (Value::Str(a), code::Scalar::Str(b)) => {
                    let a = a.as_str();
                    self.charge(lcp_size(a, b))?;
                    value::str_eq(a, b)
                }
                (Value::Num(a), code::Scalar::Num(b)) => a == b,
                (Value::Bool(a), code::Scalar::Bool(b)) => a == b,
                (Value::Null, code::Scalar::Null) => true,
                _ => false,
            };
            if equal {
                return Ok(true);
            }
        }
        Ok(false)
    }

    /// One element of a list tested by `in`: 1 step, then the comparison.
    #[inline]
    fn in_elem(&mut self, x: &Value<'h>, e: &Value<'h>, span: Range) -> R<'h, bool> {
        self.charge(1)?;
        Ok(match (x, e) {
            (Value::Str(a), Value::Str(b)) => {
                self.charge(lcp_size(a.as_str(), b.as_str()))?;
                a.as_str() == b.as_str()
            }
            _ if x.kind() == e.kind() && matches!(x.kind(), Kind::List | Kind::Map) => {
                if x.len() == e.len() {
                    self.eq(x, e, span)?
                } else {
                    false
                }
            }
            _ => x.kind() == e.kind() && scalar_eq(x, e),
        })
    }

    /// `x in y` (`01-language.md` 6.8; `02-evaluation.md` 5.3).
    fn contains(&mut self, op: BinaryOp, x: &Value<'h>, y: &Value<'h>, span: Range) -> R<'h, bool> {
        let opp = || vec![("op", op_text(op).to_string())];
        match y.kind() {
            Kind::Null => Err(fail("null-operand", "operand", span, opp())),
            Kind::List => {
                let n = y.len().unwrap_or(0);
                for i in 0..n {
                    let e = self.read(y.elem(i), span)?;
                    if self.in_elem(x, &e, span)? {
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
                    // A needle longer than the text is answered
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
        let mut steps = Spine::new();
        let mut n = inner;
        loop {
            match self.ast.kind(n) {
                NodeKind::Member { .. } if matches!(self.res(n), Res::Constant(_)) => break,
                // F7: a projection is a step whose receiver is its `x`.
                NodeKind::Each { x: Some(x), .. } => {
                    steps.push(n);
                    n = *x;
                }
                NodeKind::Member { x, .. } | NodeKind::Index { x, .. } => {
                    steps.push(n);
                    n = *x;
                }
                NodeKind::MethodCall { x, .. }
                    if !matches!(self.res(n), Res::CallHost(_) | Res::CallLib(_)) =>
                {
                    steps.push(n);
                    n = *x;
                }
                _ => break,
            }
        }
        let mut v = self.eval(n)?;
        while let Some(s) = steps.pop() {
            match self.step(s, v)? {
                Some(nv) => v = nv,
                None => return Ok(Value::Null),
            }
        }
        Ok(v)
    }

    /// The test of a star's receiver (F7 3.1): `None` for a null receiver of
    /// `?.[*]`; else the list.
    fn star_receiver(&self, v: &Value<'h>, optional: bool, star: Range) -> R<'h, Option<()>> {
        match v.kind() {
            crate::value::Kind::Null if optional => Ok(None),
            crate::value::Kind::Null => Err(fail("null-operand", "star", star, vec![])),
            crate::value::Kind::List => Ok(Some(())),
            _ => Err(fail("wrong-kind", "star", star, vec![("kind", kind_p(v))])),
        }
    }

    /// One level of a projection (F7 3.2, 3.3): each element of `recv` in
    /// order, its reads applied, and a later star's level in turn.
    fn level(
        &mut self,
        recv: &Value<'h>,
        reads: &'h [NodeId],
        out: &mut Vec<Value<'h>>,
    ) -> R<'h, ()> {
        let n = recv.len().unwrap_or(0);
        'elements: for i in 0..n {
            self.charge(1)?;
            let e = recv.elem(i);
            if reads.is_empty() {
                out.push(e);
                continue;
            }
            let mut cur = match e {
                Value::BadNum => return Err(self.host_number(Some(self.ast.span(reads[0])))),
                e => e,
            };
            for &r in reads {
                if let NodeKind::Each {
                    reads: inner,
                    optional,
                    star,
                    ..
                } = &self.ast.nodes[r.index()].kind
                {
                    match self.star_receiver(&cur, *optional, *star)? {
                        None => out.push(Value::Null),
                        Some(()) => self.level(&cur, inner, out)?,
                    }
                    continue 'elements;
                }
                match self.step(r, cur)? {
                    Some(v) => cur = v,
                    None => {
                        // An optional step on null skips the rest of the
                        // projection for this element.
                        out.push(Value::Null);
                        continue 'elements;
                    }
                }
            }
            out.push(cur);
        }
        Ok(())
    }

    /// A field read `.name` or `?.name`, the step of a chain. `None` is a
    /// null receiver of an optional read. The last step of a coalesced read
    /// under `??` gives null for an absent key (`01-language.md` 6.6).
    #[inline]
    pub(crate) fn member(
        &mut self,
        v: Value<'h>,
        name: &str,
        optional: bool,
        coalesced: bool,
        span: Range,
    ) -> R<'h, Option<Value<'h>>> {
        match &v {
            Value::Map(_) | Value::HMap(_) => {}
            Value::Null if optional => return Ok(None),
            _ => return Err(self.member_fail(&v, name, span)),
        }
        self.charge(1 + size(name))?;
        let found = match &v {
            Value::Map(m) => m.get(name).map(Value::dup),
            Value::HMap(m) => {
                let m: &'h dyn crate::host::HostMap = *m;
                m.get(name).map(from_view)
            }
            _ => None,
        };
        value::release(v);
        match found {
            Some(Value::BadNum) => Err(self.host_number(Some(span))),
            Some(x) => Ok(Some(x)),
            None if optional || coalesced => Ok(Some(Value::Null)),
            None => Err(fail(
                "missing-field",
                "field",
                span,
                vec![("name", name.to_string())],
            )),
        }
    }

    #[cold]
    #[inline(never)]
    fn member_fail(&self, v: &Value<'h>, name: &str, span: Range) -> Box<Fail<'h>> {
        if v.is_null() {
            return fail(
                "null-operand",
                "field",
                span,
                vec![("name", name.to_string())],
            );
        }
        fail(
            "wrong-kind",
            "field",
            span,
            vec![("name", name.to_string()), ("kind", kind_p(v))],
        )
    }

    /// One step of a chain. `None` is a null receiver of an optional step:
    /// the whole chain is null.
    ///
    /// Stack: a step's own work is in a function of its own, so that this
    /// frame, which is live while an index or an argument is evaluated,
    /// holds none of it (`DESIGN.md`, Stack).
    fn step(&mut self, s: NodeId, v: Value<'h>) -> R<'h, Option<Value<'h>>> {
        let ast = self.ast;
        match &ast.nodes[s.index()].kind {
            NodeKind::Each { .. } => self.each_step(s, v),
            NodeKind::Member { name, optional, .. } => {
                let coalesced = self.p.coalesced[s.index()];
                self.member(v, name, *optional, coalesced, ast.span(s))
            }
            NodeKind::Index { .. } => self.index_step(s, v),
            NodeKind::MethodCall { .. } => self.method(s, v),
            _ => Err(self.internal(ast.span(s))),
        }
    }

    /// A projection `[*]` or `?.[*]`, the step of a chain (F7 3).
    #[inline(never)]
    fn each_step(&mut self, s: NodeId, v: Value<'h>) -> R<'h, Option<Value<'h>>> {
        let NodeKind::Each {
            reads,
            optional,
            star,
            ..
        } = &self.ast.nodes[s.index()].kind
        else {
            return Err(self.internal(self.ast.span(s)));
        };
        if self.star_receiver(&v, *optional, *star)?.is_none() {
            return Ok(None);
        }
        let mut out = Vec::new();
        self.level(&v, reads, &mut out)?;
        self.alloc(1 + out.len() as u64)?;
        Ok(Some(Value::list(out)))
    }

    /// An index `[i]` or `?.[i]`, the step of a chain: the receiver's test,
    /// the index, then the read (`index_read`).
    #[inline(never)]
    pub(crate) fn index_step(&mut self, s: NodeId, v: Value<'h>) -> R<'h, Option<Value<'h>>> {
        let ast = self.ast;
        let NodeKind::Index {
            index, optional, ..
        } = &ast.nodes[s.index()].kind
        else {
            return Err(self.internal(ast.span(s)));
        };
        let span = ast.span(s);
        let k = v.kind();
        if !matches!(k, Kind::List | Kind::String | Kind::Map) {
            return self.index_receiver(&v, *optional, span);
        }
        let iv = self.eval(*index)?;
        let coalesced = self.p.coalesced[s.index()];
        self.index_read(v, k, iv, *optional, coalesced, span)
    }

    /// The receiver of an index that is not a list, a string or a map:
    /// `None` for a null receiver of `?.[`, else the failure.
    #[cold]
    #[inline(never)]
    fn index_receiver(
        &self,
        v: &Value<'h>,
        optional: bool,
        span: Range,
    ) -> R<'h, Option<Value<'h>>> {
        if v.is_null() {
            if optional {
                return Ok(None);
            }
            return Err(fail("null-operand", "index", span, vec![]));
        }
        Err(fail("wrong-kind", "index", span, vec![("kind", kind_p(v))]))
    }

    /// The read of an index whose receiver `v`, of kind `k`, and index `iv`
    /// are in hand.
    #[inline(never)]
    fn index_read(
        &mut self,
        v: Value<'h>,
        k: Kind,
        iv: Value<'h>,
        optional: bool,
        coalesced: bool,
        span: Range,
    ) -> R<'h, Option<Value<'h>>> {
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
                    None if optional || coalesced => Ok(Some(Value::Null)),
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
                        None if optional => Ok(Some(Value::Null)),
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
                    } else if optional {
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

    /// For the shape test (`04-library.md` 2.2): the count of parameters of
    /// an argument that is a lambda, written in place or the name of a
    /// `const`-bound lambda; for a function parameter's name, the listed
    /// count of its function type (F17 13.4 item 1). `None` for a value.
    fn lambda_count(&self, a: NodeId) -> Option<usize> {
        let count = |l: NodeId| match self.ast.kind(l) {
            NodeKind::Lambda { params, .. } => params.len(),
            _ => 0,
        };
        match (self.ast.kind(a), self.res(a)) {
            (NodeKind::Lambda { .. }, _) => Some(count(a)),
            (_, Res::LambdaRef(l)) => Some(count(*l)),
            (_, Res::FnRef(_, p)) => Some(crate::compile::listed_count(self.ast, *p)),
            _ => None,
        }
    }

    /// One argument of a library call: a lambda written in place (1 step),
    /// the name of one, a function parameter's name, or a value.
    #[inline]
    fn arg(&mut self, a: NodeId) -> R<'h, Arg<'h>> {
        if let NodeKind::Lambda { .. } = self.ast.kind(a) {
            self.charge(1).map(|()| {
                Arg::Lam(Fun {
                    l: a,
                    closure: None,
                })
            })
        } else if let Res::LambdaRef(l) = self.res(a) {
            Ok(Arg::Lam(Fun {
                l: *l,
                closure: None,
            }))
        } else if let Res::FnRef(s, _) = self.res(a) {
            self.bound(*s, self.ast.span(a)).map(Arg::Lam)
        } else {
            self.eval(a).map(Arg::Val)
        }
    }

    /// F17 13.4: the lambda bound to the function parameter of slot `s`, in
    /// the call now running, with its closure.
    #[inline(never)]
    fn bound(&self, s: u32, span: Range) -> R<'h, Fun> {
        let c = match &self.slots[self.slot(s)] {
            Value::Num(i) => *i as usize,
            _ => usize::MAX,
        };
        match self.closures.get(c) {
            Some((l, _)) => Ok(Fun {
                l: *l,
                closure: Some(c as u32),
            }),
            None => Err(self.internal(span)),
        }
    }

    /// F17 13.4: a lambda given at a function parameter, bound with the
    /// entries of `display` below its frame's depth, as they are where it
    /// is evaluated. The value is the closure's index, which the parameter's
    /// slot holds.
    fn closure(&mut self, l: NodeId) -> Value<'h> {
        let depth = self.p.frames[l.index()].depth as usize;
        let start = self.closure_display.len();
        self.closure_display
            .extend_from_slice(&self.display[..depth]);
        self.closures.push((l, start as u32));
        Value::Num((self.closures.len() - 1) as f64)
    }

    /// The arguments of a library call into `out`, left to right. A call of
    /// up to three arguments holds them in place.
    ///
    /// Stack: they are written to the caller's buffer, not handed to a
    /// closure, so no frame of theirs is live while the entry runs and
    /// calls a lambda.
    fn args(&mut self, args: &'h [NodeId], out: &mut Args<'h>) -> R<'h, ()> {
        if args.len() > 3 {
            return self.args_many(args, out);
        }
        if let Args::Few(v, n) = out {
            for &a in args {
                v[*n] = self.arg(a)?;
                *n += 1;
            }
        }
        Ok(())
    }

    /// The arguments of a library call of more than three.
    #[inline(never)]
    fn args_many(&mut self, args: &'h [NodeId], out: &mut Args<'h>) -> R<'h, ()> {
        let mut v = Vec::with_capacity(args.len());
        for &a in args {
            v.push(self.arg(a)?);
        }
        *out = Args::Many(v);
        Ok(())
    }

    /// A method call, the step of a chain: the method by the receiver's
    /// kind, the shape test, the arguments, then the entry (`04-library.md`
    /// 2.2). `None` is a null receiver of `?.`.
    ///
    /// Stack: this frame is live while the entry calls a lambda, so the
    /// tests and the call of a host method are functions of their own.
    #[inline(never)]
    pub(crate) fn method(&mut self, s: NodeId, recv: Value<'h>) -> R<'h, Option<Value<'h>>> {
        let ast = self.ast;
        let NodeKind::MethodCall {
            name,
            args,
            optional,
            ..
        } = &ast.nodes[s.index()].kind
        else {
            return Err(self.internal(ast.span(s)));
        };
        let span = ast.span(s);
        if recv.is_null() {
            if *optional {
                return Ok(None);
            }
            return Err(self.method_null(name, span));
        }
        if let Value::Opaque(_) = &recv {
            return self.opaque_method(recv, name, args, span).map(Some);
        }
        let idx = self.method_entry(recv.kind(), name, args, span)?;
        let measure = self.p.measure[s.index()] as u32;
        let mut vals = Args::new();
        self.args(args, &mut vals)?;
        let mark = self.cb_frames.len();
        let r = self.lib_method(idx, recv, vals.as_slice(), span, measure);
        self.end_callbacks(mark);
        match r {
            Ok(v) => Ok(Some(v)),
            Err(f) => Err(f),
        }
    }

    #[cold]
    #[inline(never)]
    fn method_null(&self, name: &SpannedName, span: Range) -> Box<Fail<'h>> {
        fail(
            "null-operand",
            "method",
            span,
            vec![("name", name.name.clone())],
        )
    }

    /// A method of a host type, on an opaque receiver: the shape test, the
    /// arguments, the step, then the host's implementation.
    #[inline(never)]
    fn opaque_method(
        &mut self,
        recv: Value<'h>,
        name: &'h SpannedName,
        args: &'h [NodeId],
        span: Range,
    ) -> R<'h, Value<'h>> {
        let Value::Opaque(o) = &recv else {
            return Err(self.internal(span));
        };
        let env = &self.p.env;
        let Some(mi) = env.method(&o.type_name, &name.name) else {
            return Err(fail(
                "wrong-kind",
                "method",
                span,
                vec![
                    ("kind", recv.kind().name().into()),
                    ("name", name.name.clone()),
                ],
            ));
        };
        let m = &env.data().methods[mi];
        let n = args.len();
        if n < m.sig.required() || m.sig.at_most().is_some_and(|a| n > a) {
            return Err(fail(
                "wrong-kind",
                "count",
                span,
                vec![
                    ("count", args.len().to_string()),
                    ("name", name.name.clone()),
                ],
            ));
        }
        for (i, &a) in args.iter().enumerate() {
            if self.lambda_count(a).is_some() {
                return Err(fail(
                    "wrong-kind",
                    "not-callback",
                    span,
                    vec![("count", (i + 1).to_string()), ("name", name.name.clone())],
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
        self.invoke_host(imp, &vals, &label, span)
    }

    /// The library method of a receiver of kind `kind`, and the shape test
    /// of its call: the count of arguments, then which are lambdas, then
    /// the parameters of each lambda (`04-library.md` 2.2).
    #[inline(never)]
    fn method_entry(
        &self,
        kind: Kind,
        name: &'h SpannedName,
        args: &'h [NodeId],
        span: Range,
    ) -> R<'h, usize> {
        let nm = || ("name", name.name.clone());
        let idx = match kind {
            Kind::String | Kind::List | Kind::Map => self.p.env.lib_method(kind, &name.name),
            _ => None,
        };
        let Some(idx) = idx else {
            return Err(fail(
                "wrong-kind",
                "method",
                span,
                vec![("kind", kind.name().into()), nm()],
            ));
        };
        let count = || {
            fail(
                "wrong-kind",
                "count",
                span,
                vec![("count", args.len().to_string()), nm()],
            )
        };
        let rec = &crate::library::records()[idx];
        let n = args.len();
        if n < rec.sig.required() || rec.sig.at_most().is_some_and(|a| n > a) {
            return Err(count());
        }
        for (i, &a) in args.iter().enumerate() {
            let lam = self.lambda_count(a).is_some();
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
            if let (Some(k), Some(n)) = (rec.callback(i), self.lambda_count(a)) {
                if n > k {
                    return Err(fail(
                        "wrong-kind",
                        "parameters",
                        span,
                        vec![nm(), ("count", k.to_string())],
                    ));
                }
            }
        }
        Ok(idx)
    }

    // ---- calls ------------------------------------------------------------

    fn call(&mut self, id: NodeId, args: &'h [NodeId]) -> R<'h, Value<'h>> {
        let span = self.ast.span(id);
        match self.res(id) {
            Res::CallLambda(l) if self.p.higher[l.index()] => self.call_higher(*l, id, args, span),
            Res::CallFn(s, _) => self.call_fn(*s, id, args, span),
            Res::CallLambda(l) => {
                let l = *l;
                let mut vals = Vec::with_capacity(args.len());
                for &a in args {
                    vals.push(self.eval(a)?);
                }
                self.charge(1)?;
                let measure = self.p.measure[id.index()] as u32;
                // `f08-errors.md` 4.1: in progress from its depth test to its end.
                match self.invoke(l, &mut vals, measure, span) {
                    Ok(v) => Ok(v),
                    Err(f) => Err(self.call_traced(f, id, span)),
                }
            }
            Res::CallLib(i) => {
                let measure = self.p.measure[id.index()] as u32;
                self.call_lib(*i as usize, args, span, measure)
            }
            Res::CallHost(i) => self.call_host(*i as usize, args, span),
            _ => Err(self.internal(span)),
        }
    }

    /// F17 13.4: a call of a higher-order lambda by name. A lambda argument
    /// at a function parameter is bound as a closure where it stands: a
    /// lambda written there charges 1 step, a name nothing. The closures the
    /// arguments added are dropped when the call ends, whether it gave a
    /// value or failed.
    #[inline(never)]
    fn call_higher(
        &mut self,
        l: NodeId,
        id: NodeId,
        args: &'h [NodeId],
        span: Range,
    ) -> R<'h, Value<'h>> {
        let marks = (self.closures.len(), self.closure_display.len());
        let r = self.call_higher_args(l, id, args, span);
        self.closures.truncate(marks.0);
        self.closure_display.truncate(marks.1);
        r
    }

    fn call_higher_args(
        &mut self,
        l: NodeId,
        id: NodeId,
        args: &'h [NodeId],
        span: Range,
    ) -> R<'h, Value<'h>> {
        let mut vals = Vec::with_capacity(args.len());
        for &a in args {
            let v = match (self.ast.kind(a), self.res(a)) {
                (NodeKind::Lambda { .. }, _) => {
                    self.charge(1)?;
                    self.closure(a)
                }
                (_, Res::LambdaRef(f)) => self.closure(*f),
                // A function parameter given on: the closure it holds.
                (_, Res::FnRef(s, _)) => {
                    let i = self.slot(*s);
                    self.slots[i].clone()
                }
                _ => self.eval(a)?,
            };
            vals.push(v);
        }
        self.charge(1)?;
        let measure = self.p.measure[id.index()] as u32;
        match self.invoke(l, &mut vals, measure, span) {
            Ok(v) => Ok(v),
            Err(f) => Err(self.call_traced(f, id, span)),
        }
    }

    /// F17 13.4: a call of a function parameter, a call by name of its
    /// bound lambda (`02-evaluation.md` 11.2). Its trace entry is named by
    /// the parameter.
    #[inline(never)]
    fn call_fn(&mut self, s: u32, id: NodeId, args: &'h [NodeId], span: Range) -> R<'h, Value<'h>> {
        let mut vals = Vec::with_capacity(args.len());
        for &a in args {
            vals.push(self.eval(a)?);
        }
        self.charge(1)?;
        let measure = self.p.measure[id.index()] as u32;
        let f = self.bound(s, span)?;
        match self.invoke_fun(f, &mut vals, measure, span) {
            Ok(v) => Ok(v),
            Err(f) => Err(self.call_traced(f, id, span)),
        }
    }

    /// Call a lambda; for a closure, with the entries of `display` below
    /// its frame's depth as they were where it was evaluated (F17 13.4 item
    /// 2), and those of the caller put back when it ends.
    fn invoke_fun(
        &mut self,
        f: Fun,
        args: &mut [Value<'h>],
        measure: u32,
        span: Range,
    ) -> R<'h, Value<'h>> {
        let Some(c) = f.closure else {
            return self.invoke(f.l, args, measure, span);
        };
        let (l, start) = self.closures[c as usize];
        let depth = self.p.frames[l.index()].depth as usize;
        let start = start as usize;
        let saved = self.display_saved.len();
        self.display_saved.extend_from_slice(&self.display[..depth]);
        self.display[..depth].copy_from_slice(&self.closure_display[start..start + depth]);
        let r = self.invoke(l, args, measure, span);
        self.display[..depth].copy_from_slice(&self.display_saved[saved..]);
        self.display_saved.truncate(saved);
        r
    }

    /// A call of a library function: the arguments, then the entry.
    #[inline(never)]
    fn call_lib(
        &mut self,
        i: usize,
        args: &'h [NodeId],
        span: Range,
        measure: u32,
    ) -> R<'h, Value<'h>> {
        let mut vals = Args::new();
        self.args(args, &mut vals)?;
        let mark = self.cb_frames.len();
        let r = self.lib_function(i, vals.as_slice(), span, measure);
        self.end_callbacks(mark);
        r
    }

    /// A call of a host function: the arguments, the call's step, then the
    /// host's implementation.
    #[inline(never)]
    fn call_host(&mut self, i: usize, args: &'h [NodeId], span: Range) -> R<'h, Value<'h>> {
        let f = &self.p.env.data().functions[i];
        let mut vals = Vec::with_capacity(args.len());
        for &a in args {
            vals.push(self.eval(a)?);
        }
        self.charge(1)?;
        self.invoke_host(f.imp.clone(), &vals, &f.name, span)
    }

    fn invoke_host(
        &mut self,
        imp: Option<crate::env::HostImpl>,
        vals: &[Value<'h>],
        name: &str,
        span: Range,
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
            Ok(Err(e)) => Err(function_error(e, span)),
            Ok(Ok(d)) => self.read(Value::from_data(&d), span),
        }
    }

    /// A library entry's call of a lambda whose parameters are not all
    /// plain names, at the element `index`: a failure that leaves it has the
    /// entry's trace entry (`f08-errors.md` 4.1, 4.2).
    #[inline(never)]
    fn invoke_for(
        &mut self,
        entry: &str,
        index: usize,
        f: Fun,
        args: &mut [Value<'h>],
        measure: u32,
        span: Range,
    ) -> R<'h, Value<'h>> {
        let r = self.invoke_fun(f, args, measure, span);
        release_all(args);
        match r {
            Ok(v) => Ok(v),
            Err(f) => Err(traced(f, span, entry, index)),
        }
    }

    /// The trace entry of a call by name, added as a failure leaves it
    /// (`f08-errors.md` 4.1).
    #[cold]
    #[inline(never)]
    fn call_traced(&self, mut f: Box<Fail<'h>>, id: NodeId, span: Range) -> Box<Fail<'h>> {
        if let NodeKind::Call { name, .. } = self.ast.kind(id) {
            f.trace.push((span, name.name.clone(), None));
        }
        f
    }

    /// A lambda passed to a library entry, resolved once for all the calls
    /// the entry makes of it.
    ///
    /// The calls an entry makes of a lambda never overlap, so on the fast
    /// path they share one frame (F17 11.5): it is pushed here, each call
    /// binds its names in it again, and the caller of the entry drops it
    /// when the entry ends, by `end_callbacks`.
    ///
    /// A closure (F17 13.4) takes the slow path: each call binds its own
    /// frame, with the entries of `display` of the place it was evaluated.
    pub(crate) fn callback(&mut self, f: Fun) -> Callback<'h> {
        let p = self.p;
        let l = f.l;
        let fast = match (p.params.get(l.index()), self.ast.kind(l)) {
            _ if f.closure.is_some() => None,
            (Some(Some(slots)), NodeKind::Lambda { body, .. }) => Some((
                &slots[..],
                *body,
                p.code.get(body.index()).and_then(|c| c.as_ref()),
            )),
            _ => None,
        };
        let frame = p.frames[l.index()];
        let mut base = 0;
        if fast.is_some() && frame.size > 0 {
            let saved = self.push_frame(frame);
            base = self.display[frame.depth as usize];
            self.cb_frames.push((frame, saved));
        }
        Callback {
            l,
            closure: f.closure,
            frame,
            base,
            fast,
        }
    }

    /// The lambda a callback calls.
    fn fun(cb: &Callback<'h>) -> Fun {
        Fun {
            l: cb.l,
            closure: cb.closure,
        }
    }

    /// The end of a library entry: the frames its callbacks share, pushed
    /// since `mark`, are dropped, whether the entry gave a value or failed.
    #[inline(always)]
    fn end_callbacks(&mut self, mark: usize) {
        while self.cb_frames.len() > mark {
            if let Some((f, saved)) = self.cb_frames.pop() {
                self.pop_frame(f, saved);
            }
        }
    }

    /// A library entry's call of a callback with two arguments, as
    /// `invoke_for` makes it.
    ///
    /// Stack: this frame is live while the lambda runs, so it holds only the
    /// fast path; a lambda with patterns or defaults goes to `invoke_for`,
    /// and a failure's trace entry is added out of line.
    #[inline]
    #[allow(clippy::too_many_arguments)]
    pub(crate) fn call2(
        &mut self,
        cb: &Callback<'h>,
        entry: &str,
        index: usize,
        a: Value<'h>,
        b: Value<'h>,
        measure: u32,
        span: Range,
    ) -> R<'h, Value<'h>> {
        let Some((slots, body, code)) = cb.fast else {
            return self.invoke_for(entry, index, Self::fun(cb), &mut [a, b], measure, span);
        };
        if let Err(f) = self.enter2(cb, slots, a, b, measure, span) {
            return Err(traced(f, span, entry, index));
        }
        match self.body(body, code, measure) {
            Ok(v) => Ok(v),
            Err(f) => Err(traced(f, span, entry, index)),
        }
    }

    /// A library entry's call of a callback with three arguments.
    #[inline]
    #[allow(clippy::too_many_arguments)]
    pub(crate) fn call3(
        &mut self,
        cb: &Callback<'h>,
        entry: &str,
        index: usize,
        args: [Value<'h>; 3],
        measure: u32,
        span: Range,
    ) -> R<'h, Value<'h>> {
        let Some((slots, body, code)) = cb.fast else {
            let mut args = args;
            return self.invoke_for(entry, index, Self::fun(cb), &mut args, measure, span);
        };
        if let Err(f) = self.enter3(cb, slots, args, measure, span) {
            return Err(traced(f, span, entry, index));
        }
        match self.body(body, code, measure) {
            Ok(v) => Ok(v),
            Err(f) => Err(traced(f, span, entry, index)),
        }
    }

    /// The depth test of a call, and the depth it adds (`invoke`).
    #[inline]
    fn enter(&mut self, measure: u32, span: Range) -> R<'h, ()> {
        self.depth_test(measure, span)?;
        self.m.depth += measure;
        Ok(())
    }

    /// The depth test of a library entry's call of a lambda whose
    /// parameters are all plain names, and the depth it adds; then each of
    /// two arguments into its parameter's slot. Both are released when the
    /// test fails.
    ///
    /// Stack and speed: an optimized build inlines it, which saves calls for
    /// each element and costs no stack. An unoptimized build keeps it a
    /// function of its own: inlined, its slots would join a frame that is
    /// live while the lambda runs. `debug_assertions` stands for the build
    /// (`DESIGN.md`, Stack). `enter3`, `bind`, `next_element` and `keep_if`
    /// do the same.
    #[cfg_attr(debug_assertions, inline)]
    #[cfg_attr(not(debug_assertions), inline(always))]
    fn enter2(
        &mut self,
        cb: &Callback<'h>,
        slots: &[Option<u32>],
        a: Value<'h>,
        b: Value<'h>,
        measure: u32,
        span: Range,
    ) -> R<'h, ()> {
        if let Err(f) = self.enter(measure, span) {
            value::release(a);
            value::release(b);
            return Err(f);
        }
        let base = self.share(cb);
        self.bind(base, slots, 0, a);
        self.bind(base, slots, 1, b);
        Ok(())
    }

    /// `enter2` for three arguments.
    #[cfg_attr(debug_assertions, inline)]
    #[cfg_attr(not(debug_assertions), inline(always))]
    fn enter3(
        &mut self,
        cb: &Callback<'h>,
        slots: &[Option<u32>],
        [a, b, c]: [Value<'h>; 3],
        measure: u32,
        span: Range,
    ) -> R<'h, ()> {
        if let Err(f) = self.enter(measure, span) {
            value::release(a);
            value::release(b);
            value::release(c);
            return Err(f);
        }
        let base = self.share(cb);
        self.bind(base, slots, 0, a);
        self.bind(base, slots, 1, b);
        self.bind(base, slots, 2, c);
        Ok(())
    }

    /// The frame a callback's calls share, made the frame of its depth for
    /// this call: a call in between of another lambda of that depth set its
    /// own and set it back, so this is a store of what is there already.
    #[cfg_attr(debug_assertions, inline)]
    #[cfg_attr(not(debug_assertions), inline(always))]
    fn share(&mut self, cb: &Callback<'h>) -> usize {
        if cb.frame.size > 0 {
            self.display[cb.frame.depth as usize] = cb.base;
        }
        cb.base
    }

    /// Argument `i` into its parameter's slot. A lambda declares at most as
    /// many parameters as the entry passes; an argument past them is
    /// released.
    #[cfg_attr(debug_assertions, inline)]
    #[cfg_attr(not(debug_assertions), inline(always))]
    fn bind(&mut self, base: usize, slots: &[Option<u32>], i: usize, v: Value<'h>) {
        match slots.get(i) {
            Some(Some(s)) => value::put(&mut self.slots[base + *s as usize], v),
            _ => value::release(v),
        }
    }

    /// Where slot `s` is in `slots`: in the frame of its depth on the chain
    /// of the code now running (F17 11.5).
    #[inline(always)]
    pub(crate) fn slot(&self, s: u32) -> usize {
        self.at(self.p.place[s as usize])
    }

    #[inline(always)]
    pub(crate) fn at(&self, p: Place) -> usize {
        self.display[p.depth as usize] + p.index as usize
    }

    /// A new frame for a call of a lambda, its slots null (F17 11.5 item
    /// 1): the entry of its depth in `display` is set to it, and the entry
    /// it replaces is kept for `pop_frame`. A lambda that binds no name has
    /// no frame: nothing reads one.
    #[cfg_attr(debug_assertions, inline)]
    #[cfg_attr(not(debug_assertions), inline(always))]
    fn push_frame(&mut self, f: Frame) -> usize {
        if f.size == 0 {
            return 0;
        }
        let base = self.slots.len();
        self.slots
            .resize_with(base + f.size as usize, || Value::Null);
        std::mem::replace(&mut self.display[f.depth as usize], base)
    }

    /// The end of a call: its frame is dropped, and the entry of its depth
    /// set back. A failure ends it too, so the names of the calls it leaves
    /// keep their values (F17 11.5 item 3).
    #[cfg_attr(debug_assertions, inline)]
    #[cfg_attr(not(debug_assertions), inline(always))]
    fn pop_frame(&mut self, f: Frame, saved: usize) {
        if f.size == 0 {
            return;
        }
        let base = std::mem::replace(&mut self.display[f.depth as usize], saved);
        self.drop_slots(base);
    }

    /// The slots from `base` on, released. Out of line: it is no part of
    /// the frame that is live while a lambda runs.
    #[inline(never)]
    fn drop_slots(&mut self, base: usize) {
        while self.slots.len() > base {
            if let Some(v) = self.slots.pop() {
                value::release(v);
            }
        }
    }

    /// The body of a lambda whose parameters are bound, and the end of its
    /// call. A failure leaves the depth as it is: a `catch` restores it.
    #[inline(always)]
    fn body(
        &mut self,
        body: NodeId,
        code: Option<&'h code::Code>,
        measure: u32,
    ) -> R<'h, Value<'h>> {
        let r = match code {
            Some(c) => c(self),
            None => self.eval(body),
        };
        if r.is_ok() {
            self.m.depth -= measure;
        }
        r
    }

    /// Call a lambda: by name after its step, or by the library after the two
    /// steps of the element. The depth test, the parameters, the body.
    ///
    /// Stack: the frame is live while the body runs, so the binding of
    /// parameters is a function of its own.
    fn invoke(
        &mut self,
        l: NodeId,
        args: &mut [Value<'h>],
        measure: u32,
        span: Range,
    ) -> R<'h, Value<'h>> {
        self.depth_test(measure, span)?;
        self.m.depth += measure;
        let NodeKind::Lambda { body, .. } = &self.ast.nodes[l.index()].kind else {
            return Err(self.internal(span));
        };
        let frame = self.p.frames[l.index()];
        let saved = self.push_frame(frame);
        let r = match self.bind_params(l, args) {
            Ok(()) => {
                let code = self.p.code.get(body.index()).and_then(|c| c.as_ref());
                self.body(*body, code, measure)
            }
            Err(f) => Err(f),
        };
        self.pop_frame(frame, saved);
        r
    }

    /// F16 12.4 item 2: execute an entry call inside a fresh `Run`.
    fn exec_entry(
        &mut self,
        entry: &'h crate::compile::CompiledEntry,
        args: &mut [Value<'h>],
    ) -> R<'h, Value<'h>> {
        // 1. Needed non-lambda top-level `const` lines in text order.
        for &c in &entry.needs {
            self.const_line(c)?;
        }
        // 2. Charge 1 step and add 1 to depth.
        self.charge(1)?;
        let span = self.ast.span(entry.const_node);
        self.depth_test(1, span)?;
        self.m.depth += 1;
        let l = entry.lambda_node;
        let NodeKind::Lambda { params, body, .. } = &self.ast.nodes[l.index()].kind else {
            return Err(self.internal(span));
        };
        // 3. Each supplied host argument is read (`03-host.md` 3.4.4).
        for (v, &pid) in args.iter().zip(params.iter()) {
            if matches!(v, Value::BadNum) {
                return Err(self.host_number(Some(self.ast.span(pid))));
            }
        }
        let frame = self.p.frames[l.index()];
        let saved = self.push_frame(frame);
        let r = match self.bind_params(l, args) {
            Ok(()) => {
                let code = self.p.code.get(body.index()).and_then(|c| c.as_ref());
                // 4 & 5. Evaluate body and subtract 1 from depth on return.
                self.body(*body, code, 1)
            }
            Err(f) => Err(f),
        };
        self.pop_frame(frame, saved);
        r
    }

    /// The arguments of a call into the parameters of lambda `l`.
    #[inline(never)]
    fn bind_params(&mut self, l: NodeId, args: &mut [Value<'h>]) -> R<'h, ()> {
        let p = self.p;
        let ast = self.ast;
        let NodeKind::Lambda { params, .. } = &ast.nodes[l.index()].kind else {
            return Ok(());
        };
        if let Some(Some(slots)) = p.params.get(l.index()) {
            // Each parameter a name with no default: the argument, or null.
            let base = self.display[p.frames[l.index()].depth as usize];
            for (i, s) in slots.iter().enumerate() {
                let v = match args.get_mut(i) {
                    Some(a) => std::mem::replace(a, Value::Null),
                    None => Value::Null,
                };
                match s {
                    Some(s) => value::put(&mut self.slots[base + *s as usize], v),
                    None => value::release(v),
                }
            }
        } else {
            for (i, &p) in params.iter().enumerate() {
                let NodeKind::Param {
                    pattern, default, ..
                } = ast.kind(p)
                else {
                    continue;
                };
                let a = args.get_mut(i).map(|a| std::mem::replace(a, Value::Null));
                let v = match (a, default) {
                    (Some(v), _) if !v.is_null() => v,
                    (_, Some(d)) => self.eval(*d)?,
                    (a, None) => a.unwrap_or(Value::Null),
                };
                self.bind_pattern(*pattern, v)?;
            }
        }
        Ok(())
    }

    // ---- patterns ---------------------------------------------------------

    fn bind_pattern(&mut self, pat: NodeId, v: Value<'h>) -> R<'h, ()> {
        let ast = self.ast;
        let span = ast.span(pat);
        match &ast.nodes[pat.index()].kind {
            NodeKind::NamePat { .. } => {
                match self.res(pat) {
                    Res::Bind(s) => {
                        let i = self.slot(*s);
                        value::put(&mut self.slots[i], v)
                    }
                    _ => value::release(v),
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
                    match slots.get(i) {
                        Some(Some(s)) => {
                            let i = self.slot(*s);
                            value::put(&mut self.slots[i], val)
                        }
                        _ => value::release(val),
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
                        let i = self.slot(slot);
                        self.slots[i] = Value::Map(Rc::new(MapVal(left)));
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
                        let i = self.slot(*slot);
                        self.slots[i] = Value::list(left);
                    }
                }
                Ok(())
            }
            _ => Err(self.internal(span)),
        }
    }

    // ---- match and try ----------------------------------------------------

    /// The arm a `match` takes, whose result the walk evaluates next. An
    /// arm's guard is evaluated after a literal of it matched, or at `_`,
    /// and the arm is taken when the guard is truthy (F17 3).
    fn match_arm(&mut self, span: Range, subject: NodeId, arms: &'h [NodeId]) -> R<'h, NodeId> {
        let s = self.eval(subject)?;
        for &a in arms {
            let NodeKind::Arm {
                literals,
                guard,
                result,
            } = self.ast.kind(a)
            else {
                continue;
            };
            self.charge(1)?;
            let matched = match literals {
                None => true,
                Some(lits) => {
                    let mut equal = false;
                    for l in lits {
                        equal = match (&l.lit, &s) {
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
                            break;
                        }
                    }
                    equal
                }
            };
            if !matched {
                continue;
            }
            match guard {
                None => return Ok(*result),
                Some(g) => {
                    if self.eval(*g)?.truthy() {
                        return Ok(*result);
                    }
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
                    let slot = self.slot(*slot);
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

/// Text being rendered (`01-language.md` 12): a template, or the result of
/// `string` or `json.stringify`.
///
/// The string's units come before it is built (`02-evaluation.md` 6.2 item
/// 3), so the walk keeps the text only while it is short: past
/// `RENDER_KEEP` bytes it measures the rest, and the text is rendered again,
/// uncharged, once the charges are made (`Run::rendered`). A short text is
/// built in one pass.
pub(crate) struct Rendering {
    buf: String,
    /// The length up to which `buf` keeps the text: `RENDER_KEEP` at first;
    /// 0 once the text has passed it, when `buf` is empty and `w` and `n`
    /// are the weight and the length in bytes of the text; and no bound for
    /// the second rendering, whose walk is charged.
    keep_to: usize,
    w: u64,
    n: usize,
}

/// The length in bytes up to which the walk keeps its text.
const RENDER_KEEP: usize = 64 << 10;

impl Rendering {
    pub(crate) fn new(buf: String) -> Self {
        Rendering {
            buf,
            keep_to: RENDER_KEEP,
            w: 0,
            n: 0,
        }
    }

    /// Whether this is the second rendering, which charges nothing.
    fn paid(&self) -> bool {
        self.keep_to == usize::MAX
    }

    /// Whether `n` more bytes of text go into `buf`. When they do not, the
    /// caller measures them.
    #[inline]
    fn keep(&mut self, n: usize) -> bool {
        if self.buf.len() + n <= self.keep_to {
            return true;
        }
        if self.keep_to > 0 {
            self.w = weight(&self.buf);
            self.n = self.buf.len();
            self.buf = String::new();
            self.keep_to = 0;
        }
        false
    }

    pub(crate) fn push(&mut self, s: &str) {
        if self.keep(s.len()) {
            self.buf.push_str(s);
        } else {
            self.w += weight(s);
            self.n += s.len();
        }
    }

    /// The JSON form of `s`. Its escapes can take `buf` past `keep_to` by at
    /// most 5 times the length of `s`.
    fn quote(&mut self, s: &str) {
        if self.keep(s.len() + 2) {
            self.buf.push('"');
            for c in s.chars() {
                push_json_char(&mut self.buf, c);
            }
            self.buf.push('"');
            return;
        }
        // The quotes and escapes are ASCII.
        let x = 2 + s
            .bytes()
            .map(|b| match b {
                b'"' | b'\\' | 0x08 | 0x0c | b'\n' | b'\r' | b'\t' => 1,
                0..=0x1f => 5,
                _ => 0,
            })
            .sum::<usize>();
        self.w += weight(s) + x as u64;
        self.n += s.len() + x;
    }

    fn number(&mut self, x: f64) {
        // A number's text is at most 24 bytes.
        if self.keep(24) {
            number::push_text(&mut self.buf, x);
        } else {
            let mut s = String::new();
            number::push_text(&mut s, x);
            self.push(&s);
        }
    }

    pub(crate) fn weight(&self) -> u64 {
        if self.keep_to == 0 {
            self.w
        } else {
            weight(&self.buf)
        }
    }
}

// The buffers of a run, kept by the thread from one run to the next, empty,
// so that a run allocates none of them again: the slots, taken by a run that
// has any, the display of frames (F17 11.5), and the text buffer of
// templates, taken at a template's first use. A buffer of `'static` values
// serves a run of any lifetime.
thread_local! {
    static SLOTS: std::cell::Cell<Vec<Value<'static>>> = const {
        std::cell::Cell::new(Vec::new())
    };
    static SCRATCH: std::cell::Cell<String> = const { std::cell::Cell::new(String::new()) };
    static DISPLAY: std::cell::Cell<Vec<usize>> = const { std::cell::Cell::new(Vec::new()) };
}

/// An empty vector of another element type, with the same allocation: a
/// collect from a vector's own iterator into one of the same layout is done
/// in place. A buffer too large to keep is dropped.
fn recast<A, B>(mut v: Vec<A>) -> Vec<B> {
    v.clear();
    if v.capacity() > 4096 {
        return Vec::new();
    }
    v.into_iter().filter_map(|_| None).collect()
}

/// A set of small numbers: in place up to 64, else allocated.
enum Bits {
    Small(u64),
    Large(Vec<bool>),
}

impl Bits {
    fn new(n: usize) -> Bits {
        if n <= 64 {
            Bits::Small(0)
        } else {
            Bits::Large(vec![false; n])
        }
    }

    #[inline]
    fn test(&self, i: usize) -> bool {
        match self {
            Bits::Small(b) => b >> i & 1 == 1,
            Bits::Large(v) => v[i],
        }
    }

    #[inline]
    fn set(&mut self, i: usize) {
        match self {
            Bits::Small(b) => *b |= 1 << i,
            Bits::Large(v) => v[i] = true,
        }
    }
}

/// The nodes along the spine of a chain or of an operator chain: a stack
/// that holds its first few in place, so that a short spine allocates
/// nothing.
struct Spine {
    inline: [NodeId; 8],
    len: usize,
    spill: Vec<NodeId>,
}

impl Spine {
    fn new() -> Spine {
        Spine {
            inline: [NodeId(0); 8],
            len: 0,
            spill: Vec::new(),
        }
    }

    #[inline]
    fn push(&mut self, n: NodeId) {
        if self.len < 8 {
            self.inline[self.len] = n;
        } else {
            self.spill.push(n);
        }
        self.len += 1;
    }

    #[inline]
    fn pop(&mut self) -> Option<NodeId> {
        if self.len == 0 {
            return None;
        }
        self.len -= 1;
        if self.len >= 8 {
            self.spill.pop()
        } else {
            Some(self.inline[self.len])
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
            Value::Num(n) => number::push_text(out, *n),
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
        // `05-types.md` 18.3: a site of class `index`, `divide`, `value` or `host`
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
            // `05-types.md` 18.4, 18.5: the mark `!` is a promise. Away from a `host` site
            // a function or method not marked `!` broke it.
            "function-error" => Some(if on("host") { "host" } else { "environment" }),
            "not-conforming" => Some(if on("value") { "value" } else { "environment" }),
            _ => f.class(),
        }
    }

    /// Whether a failure at a node's span is at a site of a class: the site is
    /// on the failing node, or on its operand, receiver, index or argument
    /// (`05-types.md` 18.3). This reach is for `dynamic` sites.
    fn at_site(&self, sites: &[crate::check::Site], class: &str, span: Range) -> bool {
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
                        if let NodeKind::Const { pattern, value, .. } = &c.kind {
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
        // F7 5.3: a star's failure is at a site on its receiver: the chain
        // before the first star, or the read before a later star.
        for n in &ast.nodes {
            let NodeKind::Each { x, reads, star, .. } = &n.kind else {
                continue;
            };
            if *star == span {
                if let Some(x) = x {
                    spans.push(ast.span(*x));
                }
            }
            if let Some(&last) = reads.last() {
                if let NodeKind::Each { star: inner, .. } = ast.kind(last) {
                    if *inner == span {
                        spans.push(match reads.len() {
                            1 => *star,
                            k => ast.span(reads[k - 2]),
                        });
                    }
                }
            }
        }
        // `05-types.md` 18.3: a field or an element of a pattern that fails is at
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
                    if let NodeKind::Const { pattern, value, .. } = &c.kind {
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

    /// `05-types.md` 18.3: the body of each lambda that is an argument, or that an
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
        // A limit diagnostic has a span whose value is not specified
        // (`03-host.md` 3.7.1). A charge has no node at hand: the span is
        // the whole program's.
        if d.span.is_none() && crate::diag::code::LIMIT.contains(&f.code) {
            let s = self.parsed.ast.node(self.parsed.ast.root).span;
            d.set_range(s);
            d.span = Some(self.parsed.source.line_span(s));
        }
        d.class = self.class_of(f);
        d
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The weight and length measured for a string not kept are those of
    /// the JSON form kept for it.
    #[test]
    fn a_measured_quote_is_the_kept_one() {
        let mut cases: Vec<String> = (0u8..0x80).map(|c| (c as char).to_string()).collect();
        cases.extend(["é😀", "\"\\\n\u{1}é\u{7f}", ""].map(String::from));
        for s in &cases {
            let mut kept = Rendering::new(String::new());
            kept.quote(s);
            let mut measured = Rendering::new(String::new());
            measured.keep_to = 0;
            measured.quote(s);
            assert_eq!(measured.n, kept.buf.len(), "{s:?}");
            assert_eq!(measured.weight(), kept.weight(), "{s:?}");
        }
    }
}
