//! Running cases and comparing outcomes (`03-host.md` 3.10.5 to 3.10.8).
//!
//! The runner has no type checker: it applies the static rules alone, and
//! gives `skip` to a case that needs the checker (3.10.8 item 5) or a
//! library entry this build does not run yet.

use super::case::{load, opaque_of, Answer, Case, Expect, ExpectDiag, ExpectFailure, Stub};
use crate::compile::{compile, Program, Res};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::env::{check_expected, Env, HostImpl, Limits};
use crate::eval::Call;
use crate::host::{Data, HostError, OpaqueRef};
use crate::json::Json;
use crate::library;
use crate::syntax::ast::NodeKind;
use crate::syntax::parse;
use crate::value::{eq_data, Value};
use std::collections::{BTreeMap, HashMap};
use std::fmt;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

/// One thing that differed (3.10.8 item 2).
#[derive(Clone, Debug, PartialEq)]
pub struct Difference {
    pub name: &'static str,
    pub expected: String,
    pub found: String,
}

#[derive(Clone, Debug, PartialEq)]
pub enum Verdict {
    Pass,
    Fail(Vec<Difference>),
    Skip(String),
    Invalid(String),
}

impl Verdict {
    pub fn label(&self) -> &'static str {
        match self {
            Verdict::Pass => "pass",
            Verdict::Fail(_) => "fail",
            Verdict::Skip(_) => "skip",
            Verdict::Invalid(_) => "invalid",
        }
    }
}

/// The verdict of each case of one file.
#[derive(Clone, Debug)]
pub struct FileReport {
    pub path: PathBuf,
    pub results: Vec<(String, Verdict)>,
}

/// The number of cases of each verdict.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Counts {
    pub pass: usize,
    pub fail: usize,
    pub skip: usize,
    pub invalid: usize,
}

impl Counts {
    pub fn add(&mut self, v: &Verdict) {
        match v {
            Verdict::Pass => self.pass += 1,
            Verdict::Fail(_) => self.fail += 1,
            Verdict::Skip(_) => self.skip += 1,
            Verdict::Invalid(_) => self.invalid += 1,
        }
    }

    pub fn merge(&mut self, o: Counts) {
        self.pass += o.pass;
        self.fail += o.fail;
        self.skip += o.skip;
        self.invalid += o.invalid;
    }

    pub fn total(&self) -> usize {
        self.pass + self.fail + self.skip + self.invalid
    }
}

impl fmt::Display for Counts {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "pass {}, fail {}, skip {}, invalid {} (total {})",
            self.pass,
            self.fail,
            self.skip,
            self.invalid,
            self.total()
        )
    }
}

impl FileReport {
    pub fn counts(&self) -> Counts {
        let mut c = Counts::default();
        for (_, v) in &self.results {
            c.add(v);
        }
        c
    }
}

impl fmt::Display for FileReport {
    /// The file's counts, then each case that did not pass or skip, with what
    /// differed.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        writeln!(f, "{}: {}", self.path.display(), self.counts())?;
        for (name, v) in &self.results {
            match v {
                Verdict::Fail(diffs) => {
                    writeln!(f, "  fail: {name}")?;
                    for d in diffs {
                        writeln!(
                            f,
                            "    {}: expected {}, found {}",
                            d.name, d.expected, d.found
                        )?;
                    }
                }
                Verdict::Invalid(why) => writeln!(f, "  invalid: {name}: {why}")?,
                _ => {}
            }
        }
        Ok(())
    }
}

/// Run every case of a file. A file that is not valid gives one result,
/// `invalid`, named after the file, standing for all its cases.
pub fn run_file(path: &Path) -> FileReport {
    match load(path) {
        Ok(file) => FileReport {
            path: path.to_path_buf(),
            results: file
                .cases
                .iter()
                .map(|c| (c.name.clone(), run_case(c)))
                .collect(),
        },
        Err(why) => FileReport {
            path: path.to_path_buf(),
            results: vec![("(every case of the file)".into(), Verdict::Invalid(why))],
        },
    }
}

/// Every `.json` file under a directory, in order of path.
pub fn case_files(dir: &Path) -> Vec<PathBuf> {
    let mut out = Vec::new();
    let mut stack = vec![dir.to_path_buf()];
    while let Some(d) = stack.pop() {
        let Ok(entries) = std::fs::read_dir(&d) else {
            continue;
        };
        for e in entries.flatten() {
            let p = e.path();
            if p.is_dir() {
                stack.push(p);
            } else if p.extension().is_some_and(|x| x == "json") {
                out.push(p);
            }
        }
    }
    out.sort();
    out
}

/// Opaque values of one case: one host value for each type and id
/// (3.10.3 item 8).
#[derive(Default)]
struct Interner {
    map: HashMap<(String, u64), OpaqueRef>,
}

impl Interner {
    fn data(&mut self, v: &Json) -> Data {
        if let Some((t, id)) = opaque_of(v) {
            let o = self
                .map
                .entry((t.to_string(), id))
                .or_insert_with(|| OpaqueRef::new(t, id))
                .clone();
            return Data::Opaque(o);
        }
        match v {
            Json::Null => Data::Null,
            Json::Bool(b) => Data::Bool(*b),
            // Ruling T7: a JSON number is a floating value.
            Json::Num(n) => Data::Float(*n),
            Json::Str(s) => Data::Str(s.clone()),
            Json::Arr(a) => Data::List(a.iter().map(|x| self.data(x)).collect()),
            Json::Obj(m) => Data::Map(m.iter().map(|(k, x)| (k.clone(), self.data(x))).collect()),
        }
    }
}

struct RowData {
    args: Option<Vec<Data>>,
    answer: Result<Data, String>,
    charge: u64,
    allocate: u64,
}

/// A stub as a host function (3.10.4).
fn stub_impl(stub: &Stub, interner: &mut Interner, unmatched: Arc<AtomicBool>) -> HostImpl {
    let rows: Vec<RowData> = stub
        .calls
        .iter()
        .map(|r| RowData {
            args: r
                .args
                .as_ref()
                .map(|a| a.iter().map(|x| interner.data(x)).collect()),
            answer: match &r.answer {
                Answer::Result(v) => Ok(interner.data(v)),
                Answer::Error(e) => Err(e.clone()),
            },
            charge: r.charge,
            allocate: r.allocate,
        })
        .collect();
    Arc::new(move |call: &mut Call<'_>, args: &[Value<'_>]| {
        for r in &rows {
            let fits = match &r.args {
                None => true,
                Some(a) => a.len() == args.len() && a.iter().zip(args).all(|(d, v)| eq_data(v, d)),
            };
            if fits {
                if call.charge(r.charge).is_err() || call.allocate(r.allocate).is_err() {
                    return Err(HostError("a limit was passed".into()));
                }
                return r.answer.clone().map_err(HostError);
            }
        }
        unmatched.store(true, Ordering::Relaxed);
        Err(HostError("no row of the stub matches the call".into()))
    })
}

/// The case's environment, as a child of the suite's base environment.
fn build_env(
    case: &Case,
    interner: &mut Interner,
    unmatched: &Arc<AtomicBool>,
) -> Result<Env, Error> {
    let mut b = Env::builder();
    if let Some(v) = &case.version {
        b = b.version(v);
    }
    if let Some(e) = &case.env {
        b = b.vars(e);
    }
    for o in &case.opaque {
        b = b.opaque(o);
    }
    for (key, stub) in &case.methods {
        let imp = stub_impl(stub, interner, unmatched.clone());
        b = b.method_impl(key, stub.ty.as_deref().unwrap_or_default(), Some(imp));
    }
    for (name, stub) in &case.functions {
        let imp = stub_impl(stub, interner, unmatched.clone());
        b = b.function_impl(name, stub.ty.as_deref().unwrap_or_default(), imp);
    }
    if let Some(r) = &case.reject {
        let r: Vec<&str> = r.iter().map(String::as_str).collect();
        b = b.reject(&r);
    }
    let mut limits = Limits::default();
    if let Some(s) = case.limits.steps {
        limits.steps = Some(s);
    }
    if let Some(a) = case.limits.allocation {
        limits.allocation = Some(a);
    }
    if let Some(d) = case.limits.depth {
        limits.depth = d as u32;
    }
    b = b.limits(limits);
    let mut errs = match b.build() {
        Ok(env) => {
            if let Some(r) = &case.returns {
                if let Err(d) = check_expected(r) {
                    return Err(Error::one(d));
                }
            }
            return Ok(env);
        }
        Err(e) => e,
    };
    if let Some(r) = &case.returns {
        if let Err(d) = check_expected(r) {
            errs.diagnostics.push(d);
        }
    }
    Err(errs)
}

/// The library entries the program may call that this build does not run.
fn missing_entries(p: &Program) -> Vec<String> {
    let mut out = Vec::new();
    for (i, n) in p.parsed.ast.nodes.iter().enumerate() {
        match (&p.res[i], &n.kind) {
            (Res::CallLib(e), _) if !library::implemented(*e as usize) => {
                out.push(library::records()[*e as usize].name.clone());
            }
            (Res::None, NodeKind::MethodCall { name, .. }) => {
                let recs = library::records();
                let any = recs
                    .iter()
                    .any(|r| r.receiver.is_some() && r.name == name.name);
                let runs = recs.iter().enumerate().any(|(j, r)| {
                    r.receiver.is_some() && r.name == name.name && library::implemented(j)
                });
                let opaque = p.env.data().methods.iter().any(|m| m.name == name.name);
                if any && !runs && !opaque {
                    out.push(format!(".{}", name.name));
                }
            }
            _ => {}
        }
    }
    out.sort();
    out.dedup();
    out
}

/// Run one case (3.10.5).
pub fn run_case(case: &Case) -> Verdict {
    if let Some(why) = &case.skip {
        return Verdict::Skip(why.clone());
    }
    match &case.expect {
        Expect::Result(r) if r.ty.is_some() || r.sites.is_some() => {
            return Verdict::Skip("states type or sites: needs the type checker".into())
        }
        Expect::Failure(f)
            if f.phase == Phase::Check
                && f.diagnostics
                    .iter()
                    .any(|d| !code::STATIC.contains(&d.code.as_str())) =>
        {
            return Verdict::Skip(
                "expects a check code outside the static rules: needs the type checker".into(),
            )
        }
        _ => {}
    }
    let mut interner = Interner::default();
    let unmatched = Arc::new(AtomicBool::new(false));
    let env = match build_env(case, &mut interner, &unmatched) {
        Ok(e) => e,
        Err(e) => return judge_failure(case, e.phase, &e.diagnostics, None, &unmatched),
    };
    let parsed = match parse(&case.source) {
        Ok(p) => p,
        Err(e) => return judge_failure(case, Phase::Parse, &e.diagnostics, None, &unmatched),
    };
    let program = match compile(&env, parsed) {
        Ok(p) => p,
        Err(e) => return judge_failure(case, e.phase, &e.diagnostics, None, &unmatched),
    };
    let missing = missing_entries(&program);
    if !missing.is_empty() {
        return Verdict::Skip(format!("needs library entry {}", missing.join(", ")));
    }
    let needs_run = case.vars.is_some()
        || match &case.expect {
            Expect::Result(r) => r.needs_run(),
            Expect::Failure(f) => f.phase == Phase::Run,
        };
    if !needs_run {
        return match &case.expect {
            Expect::Result(_) => Verdict::Pass,
            Expect::Failure(f) => Verdict::Fail(vec![Difference {
                name: "outcome",
                expected: format!("a failure of phase {}", f.phase),
                found: "a result".into(),
            }]),
        };
    }
    let vars: BTreeMap<String, Data> = case
        .vars
        .iter()
        .flatten()
        .map(|(k, v)| (k.clone(), interner.data(v)))
        .collect();
    let out = program.run(&vars);
    let usage = out.usage;
    match out.result {
        Err(f) => {
            let d = f.diagnostic(&program.parsed.source);
            judge_failure(
                case,
                Phase::Run,
                &[d],
                Some((usage.steps, usage.allocated)),
                &unmatched,
            )
        }
        Ok(v) => {
            let mut diffs = Vec::new();
            match &case.expect {
                Expect::Failure(f) => diffs.push(Difference {
                    name: "outcome",
                    expected: format!("a failure of phase {}", f.phase),
                    found: format!("the value {v:?}"),
                }),
                Expect::Result(r) => {
                    if let Some(want) = &r.value {
                        let want = interner.data(want);
                        if !eq_data(&v, &want) {
                            diffs.push(Difference {
                                name: "value",
                                expected: format!("{want:?}"),
                                found: format!("{v:?}"),
                            });
                        }
                    }
                    if let Some(t) = r.truthy {
                        if v.truthy() != t {
                            diffs.push(Difference {
                                name: "truthy",
                                expected: t.to_string(),
                                found: v.truthy().to_string(),
                            });
                        }
                    }
                    count_diffs(
                        &mut diffs,
                        r.steps,
                        r.allocated,
                        (usage.steps, usage.allocated),
                    );
                }
            }
            functions_diff(&mut diffs, &unmatched);
            verdict(diffs)
        }
    }
}

fn verdict(diffs: Vec<Difference>) -> Verdict {
    if diffs.is_empty() {
        Verdict::Pass
    } else {
        Verdict::Fail(diffs)
    }
}

fn count_diffs(
    diffs: &mut Vec<Difference>,
    steps: Option<u64>,
    units: Option<u64>,
    got: (u64, u64),
) {
    if let Some(s) = steps {
        if s != got.0 {
            diffs.push(Difference {
                name: "steps",
                expected: s.to_string(),
                found: got.0.to_string(),
            });
        }
    }
    if let Some(a) = units {
        if a != got.1 {
            diffs.push(Difference {
                name: "allocated",
                expected: a.to_string(),
                found: got.1.to_string(),
            });
        }
    }
}

fn functions_diff(diffs: &mut Vec<Difference>, unmatched: &AtomicBool) {
    if unmatched.load(Ordering::Relaxed) {
        diffs.push(Difference {
            name: "functions",
            expected: "every call of a stub matches a row".into(),
            found: "a call that matches no row".into(),
        });
    }
}

fn judge_failure(
    case: &Case,
    phase: Phase,
    diags: &[Diagnostic],
    usage: Option<(u64, u64)>,
    unmatched: &AtomicBool,
) -> Verdict {
    let mut diffs = match &case.expect {
        Expect::Result(_) => vec![Difference {
            name: "outcome",
            expected: "a result".into(),
            found: format!("a failure of phase {phase}: {}", show_all(diags)),
        }],
        Expect::Failure(f) => {
            let mut d = compare_failure(f, phase, diags);
            if d.is_empty() {
                if let Some(u) = usage {
                    count_diffs(&mut d, f.steps, f.allocated, u);
                }
            }
            d
        }
    };
    functions_diff(&mut diffs, unmatched);
    verdict(diffs)
}

fn show_all(diags: &[Diagnostic]) -> String {
    diags.iter().map(show_diag).collect::<Vec<_>>().join("; ")
}

fn show_span(s: Option<[u32; 4]>) -> String {
    match s {
        Some([a, b, c, d]) => format!("[{a}, {b}, {c}, {d}]"),
        None => "no span".into(),
    }
}

fn show_diag(d: &Diagnostic) -> String {
    let mut s = format!("{} at {}", d.code, show_span(d.span.map(|s| s.0)));
    if d.phase == Phase::Run {
        s.push_str(&format!(" {:?}", d.message));
        if let Some(x) = &d.data {
            s.push_str(&format!(" data {x:?}"));
        }
    }
    s
}

fn show_expected(d: &ExpectDiag) -> String {
    let mut s = match d.span {
        Some(sp) => format!("{} at {}", d.code, show_span(Some(sp))),
        None => d.code.clone(),
    };
    if let Some(m) = &d.message {
        s.push_str(&format!(" {m:?}"));
    }
    if let Some(x) = &d.data {
        s.push_str(&format!(" data {x}"));
    }
    s
}

/// The codes whose class depends on the check (`03-host.md` 3.7.6).
const CHECKED_CLASS: [&str; 5] = [
    "null-operand",
    "wrong-kind",
    "missing-field",
    "no-match",
    "not-conforming",
];

/// Whether one expected diagnostic matches one actual (3.10.6, 3.10.7).
/// `class` is compared but for the five codes whose class depends on a
/// check this runner does not make (rulings round 10 AM1).
fn diag_matches(e: &ExpectDiag, a: &Diagnostic) -> bool {
    if e.code != a.code {
        return false;
    }
    if let Some(c) = &e.class {
        if !CHECKED_CLASS.contains(&a.code) && a.class != Some(c.as_str()) {
            return false;
        }
    }
    if let Some(s) = e.span {
        if !code::LIMIT.contains(&a.code) && a.span.map(|x| x.0) != Some(s) {
            return false;
        }
    }
    if let Some(m) = &e.message {
        if *m != a.message {
            return false;
        }
    }
    if let Some(want) = &e.data {
        let mut i = Interner::default();
        let want = i.data(want);
        match &a.data {
            Some(got) => {
                let v = Value::from_data(got);
                if !eq_data(&v, &want) {
                    return false;
                }
            }
            None => return false,
        }
    }
    // Core raises no `not-conforming`, so no diagnostic has a path.
    e.path.is_none()
}

fn compare_failure(f: &ExpectFailure, phase: Phase, diags: &[Diagnostic]) -> Vec<Difference> {
    if f.phase != phase {
        return vec![Difference {
            name: "outcome",
            expected: format!("a failure of phase {}", f.phase),
            found: format!("a failure of phase {phase}: {}", show_all(diags)),
        }];
    }
    let ok = match phase {
        // Compared with the start of what the parser reported (item 5).
        Phase::Parse => {
            f.diagnostics.len() <= diags.len()
                && f.diagnostics
                    .iter()
                    .zip(diags)
                    .all(|(e, a)| diag_matches(e, a))
        }
        // Without regard to order (item 6).
        Phase::Declare => {
            let mut used = vec![false; diags.len()];
            f.diagnostics.len() == diags.len()
                && f.diagnostics.iter().all(|e| {
                    match (0..diags.len()).find(|&i| !used[i] && diag_matches(e, &diags[i])) {
                        Some(i) => {
                            used[i] = true;
                            true
                        }
                        None => false,
                    }
                })
        }
        Phase::Check | Phase::Run => {
            f.diagnostics.len() == diags.len()
                && f.diagnostics
                    .iter()
                    .zip(diags)
                    .all(|(e, a)| diag_matches(e, a))
        }
    };
    if ok {
        Vec::new()
    } else {
        vec![Difference {
            name: "diagnostics",
            expected: f
                .diagnostics
                .iter()
                .map(show_expected)
                .collect::<Vec<_>>()
                .join("; "),
            found: show_all(diags),
        }]
    }
}

/// One line of the outcome command (round 10 AJ): what a case found, with
/// its `expect` ignored.
pub fn outcome_line(case: &Case) -> String {
    let mut out = String::from("{\"name\":");
    out.push_str(&json_text(&case.name));
    let mut interner = Interner::default();
    let unmatched = Arc::new(AtomicBool::new(false));
    let fail_line = |out: &mut String, phase: Phase, diags: &[Diagnostic]| {
        out.push_str(&format!(",\"phase\":\"{phase}\",\"diagnostics\":["));
        let mut ds: Vec<&Diagnostic> = match phase {
            Phase::Parse => diags.iter().take(1).collect(),
            _ => diags.iter().collect(),
        };
        if phase == Phase::Declare {
            ds.sort_by_key(|d| d.code);
        }
        for (i, d) in ds.iter().enumerate() {
            if i > 0 {
                out.push(',');
            }
            out.push_str(&format!("{{\"code\":\"{}\"", d.code));
            if phase != Phase::Declare {
                out.push_str(",\"span\":");
                match d.span {
                    Some(s) => {
                        let [a, b, c, e] = s.0;
                        out.push_str(&format!("[{a},{b},{c},{e}]"));
                    }
                    None => out.push_str("null"),
                }
            }
            if phase == Phase::Run {
                out.push_str(",\"class\":");
                match d.class {
                    Some(c) => out.push_str(&format!("\"{c}\"")),
                    None => out.push_str("null"),
                }
                out.push_str(",\"message\":");
                out.push_str(&json_text(&d.message));
            }
            out.push('}');
        }
        out.push(']');
    };
    let env = match build_env(case, &mut interner, &unmatched) {
        Ok(e) => e,
        Err(e) => {
            fail_line(&mut out, e.phase, &e.diagnostics);
            out.push('}');
            return out;
        }
    };
    let program = match parse(&case.source) {
        Err(e) => Err((Phase::Parse, e.diagnostics)),
        Ok(p) => compile(&env, p).map_err(|e| (e.phase, e.diagnostics)),
    };
    let program = match program {
        Ok(p) => p,
        Err((phase, d)) => {
            fail_line(&mut out, phase, &d);
            out.push('}');
            return out;
        }
    };
    let vars: BTreeMap<String, Data> = case
        .vars
        .iter()
        .flatten()
        .map(|(k, v)| (k.clone(), interner.data(v)))
        .collect();
    let run = program.run(&vars);
    let mut result = run.result.map_err(|f| f.diagnostic(&program.parsed.source));
    let mut text = String::new();
    if let Ok(v) = &result {
        if let Err(code) = value_json(v, &mut text, 0) {
            let msg = crate::messages::build(code, "host", &[]);
            let mut d = Diagnostic::bare(Phase::Run, code, msg);
            d.class = Some(if code == "number-range" {
                "environment"
            } else {
                "limit"
            });
            result = Err(d);
        }
    }
    // Ruling AK3: a failure of class `limit` has no span and no usage.
    let limit = match result {
        Ok(_) => {
            out.push_str(",\"phase\":null,\"value\":");
            out.push_str(&text);
            false
        }
        Err(mut d) => {
            let limit = d.class == Some("limit");
            if limit {
                d.span = None;
            }
            fail_line(&mut out, Phase::Run, &[d]);
            limit
        }
    };
    if !limit {
        out.push_str(&format!(
            ",\"steps\":{},\"units\":{}",
            run.usage.steps, run.usage.allocated
        ));
    }
    out.push('}');
    out
}

fn json_text(s: &str) -> String {
    let mut out = String::from("\"");
    for c in s.chars() {
        crate::messages::push_json_char(&mut out, c);
    }
    out.push('"');
    out
}

/// A result as JSON, with an opaque value written as a case file writes it.
fn value_json(v: &Value<'_>, out: &mut String, depth: u32) -> Result<(), &'static str> {
    match v {
        Value::Null => out.push_str("null"),
        Value::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
        Value::Num(n) => out.push_str(&crate::number::to_text(*n)),
        Value::BadNum => return Err("number-range"),
        Value::Str(s) => out.push_str(&json_text(s.as_str())),
        Value::Opaque(o) => {
            let id = o.value.downcast_ref::<u64>().copied().unwrap_or(0);
            out.push_str(&format!(
                "{{\"$opaque\":{{\"type\":{},\"id\":{id}}}}}",
                json_text(&o.type_name)
            ));
        }
        Value::List(_) | Value::HList(_) => {
            if depth >= 200 {
                return Err("depth-exceeded");
            }
            out.push('[');
            for i in 0..v.len().unwrap_or(0) {
                if i > 0 {
                    out.push(',');
                }
                value_json(&v.elem(i), out, depth + 1)?;
            }
            out.push(']');
        }
        Value::Map(_) | Value::HMap(_) => {
            if depth >= 200 {
                return Err("depth-exceeded");
            }
            out.push('{');
            for (i, (k, x)) in v.entries().iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                out.push_str(&json_text(k.as_str()));
                out.push(':');
                value_json(x, out, depth + 1)?;
            }
            out.push('}');
        }
    }
    Ok(())
}
