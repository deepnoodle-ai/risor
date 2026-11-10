//! Running cases and comparing outcomes (`03-host.md` 3.10.5 to 3.10.8).
//!
//! The runner has no type checker: it applies the static rules alone, and
//! gives `skip` to a case that needs the checker (3.10.8 item 5) or a
//! library entry this build does not run yet.

use super::case::{
    load, opaque_of, Answer, Case, Expect, ExpectDiag, ExpectFailure, ExpectResult, Stub,
};
use crate::compile::{compile_unchecked as compile, Program, Res};
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
    run_file_with(path, true)
}

/// Run every case of a file: as a runner with a checker does (`checked`),
/// or as one with none.
pub fn run_file_with(path: &Path, checked: bool) -> FileReport {
    // `suite/core-check/` and F1's `suite/host-types/` (7.3 item 1) run
    // each case checked, once.
    let core_check = path
        .components()
        .any(|c| c.as_os_str() == "core-check" || c.as_os_str() == "host-types")
        || check_only_file(path);
    match load(path) {
        Ok(file) => FileReport {
            path: path.to_path_buf(),
            results: file
                .cases
                .iter()
                .map(|c| {
                    let v = if checked {
                        run_case_in(c, core_check)
                    } else {
                        run_case_unchecked(c)
                    };
                    (c.name.clone(), v)
                })
                .collect(),
        },
        Err(why) => FileReport {
            path: path.to_path_buf(),
            results: vec![("(every case of the file)".into(), Verdict::Invalid(why))],
        },
    }
}

/// `03-host.md` 3.11 item 7: in a package's directory, a file
/// whose name ends `-check.json` is run as `suite/core-check/` is: each case
/// once, checked.
pub fn check_only_file(path: &Path) -> bool {
    let in_core = path
        .parent()
        .and_then(|d| d.file_name())
        .is_some_and(|d| d == "core-run" || d == "core-check");
    !in_core
        && path
            .file_name()
            .and_then(|n| n.to_str())
            .is_some_and(|n| n.ends_with("-check.json"))
}

/// Whether the runner does not run a file (`03-host.md` 3.11 item 4):
/// its name begins `until-fNN-` and this build offers package NN.
pub fn skipped_by_package(path: &Path) -> bool {
    let Some(name) = path.file_name().and_then(|n| n.to_str()) else {
        return false;
    };
    let Some(rest) = name.strip_prefix("until-f") else {
        return false;
    };
    let digits: String = rest.chars().take_while(|c| c.is_ascii_digit()).collect();
    if digits.is_empty() || !rest[digits.len()..].starts_with('-') {
        return false;
    }
    digits
        .parse::<u32>()
        .is_ok_and(|n| crate::PACKAGES.contains(&n))
}

/// The case files under a directory that the runner does not run, by `03-host.md` 3.11 item 4.
pub fn skipped_files(dir: &Path) -> Vec<PathBuf> {
    all_json(dir)
        .into_iter()
        .filter(|p| skipped_by_package(p))
        .collect()
}

/// Every `.json` file under a directory that the runner runs, in order of
/// path: an `until-fNN-` file of a package this build offers is left out.
pub fn case_files(dir: &Path) -> Vec<PathBuf> {
    all_json(dir)
        .into_iter()
        .filter(|p| !skipped_by_package(p))
        .collect()
}

fn all_json(dir: &Path) -> Vec<PathBuf> {
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
            // `03-host.md` 3.10.3 item 5: a JSON number is a floating value.
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
    for (name, schema, opts) in &case.schemas {
        b = b.var_schema(name, schema, *opts);
    }
    for t in &case.types {
        b = match t {
            super::case::TypeDecl::Text(n, text) => b.named_type(n, text),
            super::case::TypeDecl::Schema(n, schema, opts) => b.named_type_schema(n, schema, *opts),
        };
    }
    for o in &case.opaque {
        b = b.opaque(o);
    }
    for r in &case.removed {
        b = b.remove(r);
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
                // The expected result's type is resolved when the
                // environment is, in either mode: this build has the checker
                // (`03-host.md` 3.3.1 item 2).
                if let Err(d) = crate::check::expected(&env, Some(r)) {
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

/// The environment a case declares (3.10.5 step 2), with its stubs as the
/// implementations of its functions and methods.
pub fn case_env(case: &Case) -> Result<Env, Error> {
    let mut interner = Interner::default();
    let unmatched = Arc::new(AtomicBool::new(false));
    build_env(case, &mut interner, &unmatched)
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

/// Run one case (3.10.5) as a runner with a type checker does (3.10.8 item
/// 5): a case with `"check": false` once with the
/// static rules alone; a case of `suite/core-check/`, or one that needs the
/// checker, once checked; every other case twice, checked and with the
/// static rules alone, and it passes only when both runs pass.
pub fn run_case(case: &Case) -> Verdict {
    run_case_in(case, false)
}

/// As `run_case`, for a case of `suite/core-check/` when `core_check`.
pub fn run_case_in(case: &Case, core_check: bool) -> Verdict {
    if let Some(why) = &case.skip {
        return Verdict::Skip(why.clone());
    }
    if let Expect::Result(ExpectResult {
        analysis: Some(want),
        ..
    }) = &case.expect
    {
        return run_analysis_case(case, want);
    }
    if case.conform.is_some() {
        return run_conform_case(case);
    }
    if !case.check {
        return run_case_with(case, false, true);
    }
    let needs_checker = match &case.expect {
        Expect::Result(r) => r.ty.is_some() || r.sites.is_some(),
        Expect::Failure(f) => {
            f.phase == Phase::Check
                && f.diagnostics
                    .iter()
                    .any(|d| !super::case::is_static(&d.code, case.conversion))
        }
    };
    // F5 11.2 item 2: a case with `schemas`, or that states `description`,
    // runs once, checked.
    let needs_checker = needs_checker
        || !case.schemas.is_empty()
        || matches!(&case.expect, Expect::Result(r) if r.description.is_some());
    let checked = run_case_with(case, true, true);
    if core_check || needs_checker {
        return checked;
    }
    let plain = run_case_with(case, false, false);
    match (checked, plain) {
        (Verdict::Pass, Verdict::Pass) => Verdict::Pass,
        (a, b) => {
            let mut diffs: Vec<Difference> = Vec::new();
            for v in [a, b] {
                if let Verdict::Fail(ds) = v {
                    for d in ds {
                        if !diffs.iter().any(|x| x.name == d.name) {
                            diffs.push(d);
                        }
                    }
                }
            }
            let order = [
                "outcome",
                "analysis",
                "description",
                "type",
                "sites",
                "value",
                "truthy",
                "diagnostics",
                "steps",
                "allocated",
                "functions",
            ];
            diffs.sort_by_key(|d| order.iter().position(|n| *n == d.name));
            Verdict::Fail(diffs)
        }
    }
}

/// Run one case as a runner with no type checker does (3.10.8 item 6): the
/// static rules alone at step 4, and `skip` for what needs the checker.
pub fn run_case_unchecked(case: &Case) -> Verdict {
    if let Expect::Result(ExpectResult {
        analysis: Some(want),
        ..
    }) = &case.expect
    {
        return run_analysis_case(case, want);
    }
    if !case.check {
        return run_case_with(case, false, true);
    }
    run_case_with(case, false, false)
}

/// `all_class`: compare `class` for every code, as a checked run and a case
/// with `"check": false` do (3.10.7 item 8).
fn run_case_with(case: &Case, checked: bool, all_class: bool) -> Verdict {
    if let Some(why) = &case.skip {
        return Verdict::Skip(why.clone());
    }
    match &case.expect {
        _ if checked || !case.check => {}
        Expect::Result(r) if r.ty.is_some() || r.sites.is_some() => {
            return Verdict::Skip("states type or sites: needs the type checker".into())
        }
        Expect::Failure(f)
            if f.phase == Phase::Check
                && f.diagnostics
                    .iter()
                    .any(|d| !super::case::is_static(&d.code, case.conversion)) =>
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
        Err(e) => return judge_failure(case, e.phase, &e.diagnostics, None, &unmatched, all_class),
    };
    // F1 7.2: the description is taken once the environment is built.
    let mut diffs = Vec::new();
    if let Expect::Result(ExpectResult {
        description: Some(want),
        ..
    }) = &case.expect
    {
        let got = env.describe();
        if !crate::json::same_data(want, &got) {
            diffs.push(Difference {
                name: "description",
                expected: crate::json::to_text(want),
                found: crate::json::to_text(&got),
            });
        }
    }
    let parsed = match parse_case(case) {
        Ok(p) => p,
        Err(e) => {
            return judge_failure(
                case,
                Phase::Parse,
                &e.diagnostics,
                None,
                &unmatched,
                all_class,
            )
        }
    };
    // Step 4: the check, or the static rules alone.
    let mut sites = None;
    if checked {
        let exp = match crate::check::expected(&env, case.returns.as_deref()) {
            Ok(x) => x,
            Err(d) => {
                return judge_failure(case, Phase::Declare, &[d], None, &unmatched, all_class)
            }
        };
        let r = crate::check::check(&env, &parsed, &exp);
        if !r.diagnostics.is_empty() {
            return judge_failure(
                case,
                Phase::Check,
                &r.diagnostics,
                None,
                &unmatched,
                all_class,
            );
        }
        if let Expect::Result(want) = &case.expect {
            let got = r.ty.clone().unwrap_or_default();
            if let Some(t) = &want.ty {
                let wt = crate::types::text::parse_type(t)
                    .ok()
                    .and_then(|x| crate::check::ty::from_text(&x, &env.data().names, None).ok());
                if wt.as_ref() != Some(&got) {
                    diffs.push(Difference {
                        name: "type",
                        expected: t.clone(),
                        found: crate::check::ty::print(&got),
                    });
                }
            }
            if let Some(ws) = &want.sites {
                let gs: Vec<(String, [u32; 4])> = r
                    .sites
                    .iter()
                    .map(|s| (s.class.to_string(), parsed.source.line_span(s.span).0))
                    .collect();
                let ws: Vec<(String, [u32; 4])> =
                    ws.iter().map(|s| (s.class.clone(), s.span)).collect();
                if gs != ws {
                    diffs.push(Difference {
                        name: "sites",
                        expected: format!("{ws:?}"),
                        found: format!("{gs:?}"),
                    });
                }
            }
        }
        sites = Some(r.sites);
    }
    let mut program = match compile(&env, parsed) {
        Ok(p) => p,
        Err(e) => return judge_failure(case, e.phase, &e.diagnostics, None, &unmatched, all_class),
    };
    program.sites = sites;
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
            Expect::Result(_) => verdict(diffs),
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
            let d = program.diagnostic(&f);
            judge_failure(
                case,
                Phase::Run,
                &[d],
                Some((usage.steps, usage.allocated)),
                &unmatched,
                all_class,
            )
        }
        Ok(v) => {
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
    checked: bool,
) -> Verdict {
    let mut diffs = match &case.expect {
        Expect::Result(_) => vec![Difference {
            name: "outcome",
            expected: "a result".into(),
            found: format!("a failure of phase {phase}: {}", show_all(diags)),
        }],
        Expect::Failure(f) => {
            let mut d = compare_failure(f, phase, diags, checked);
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

/// The codes whose class depends on the check (`03-host.md` 3.7.6), six of
/// them with `function-error`.
const CHECKED_CLASS: [&str; 6] = [
    "null-operand",
    "wrong-kind",
    "missing-field",
    "no-match",
    "not-conforming",
    "function-error",
];

/// Whether one expected diagnostic matches one actual (3.10.6, 3.10.7).
/// `class` is compared but for the six codes whose class depends on a
/// check this runner does not make (`03-host.md` 3.10.7 item 8).
fn diag_matches(e: &ExpectDiag, a: &Diagnostic, checked: bool) -> bool {
    if e.code != a.code {
        return false;
    }
    if let Some(c) = &e.class {
        if (checked || !CHECKED_CLASS.contains(&a.code)) && a.class != Some(c.as_str()) {
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
    // F4: the path of a `not-conforming` failure, step by step.
    match &e.path {
        None => true,
        Some(want) => a
            .path
            .as_deref()
            .is_some_and(|p| crate::json::same_data(&Json::Arr(want.clone()), &path_json(p))),
    }
}

fn compare_failure(
    f: &ExpectFailure,
    phase: Phase,
    diags: &[Diagnostic],
    checked: bool,
) -> Vec<Difference> {
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
                    .all(|(e, a)| diag_matches(e, a, checked))
        }
        // Without regard to order (item 6).
        Phase::Declare => {
            let mut used = vec![false; diags.len()];
            f.diagnostics.len() == diags.len()
                && f.diagnostics.iter().all(|e| {
                    match (0..diags.len())
                        .find(|&i| !used[i] && diag_matches(e, &diags[i], checked))
                    {
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
                    .all(|(e, a)| diag_matches(e, a, checked))
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

/// One line of the outcome command (`suite/README.md`): what a case found, with
/// its `expect` ignored.
/// The case's program (3.10.5 step 3): its text parsed as a program, or,
/// with F2's `template`, as a template taken in its mode.
fn parse_case(case: &Case) -> Result<crate::syntax::Parsed, Error> {
    match case.template {
        Some(mode) => crate::template::parse_template(&case.source).map(|t| t.program(mode)),
        None => parse(&case.source),
    }
}

/// The analysis report of the case's text as JSON text (F2 3), when it
/// parses.
fn analysis_text(case: &Case) -> Option<String> {
    match case.template {
        Some(mode) => {
            let t = crate::template::parse_template(&case.source).ok()?;
            Some(crate::analysis::analyze_template(&t, mode).to_json(t.source()))
        }
        None => {
            let p = parse(&case.source).ok()?;
            Some(crate::analysis::analyze(&p).to_json(&p.source))
        }
    }
}

/// What a conform case found (F4 9.3): `Err` is the `declare` failure.
struct Conformed {
    /// The conformance function's diagnostic, or none.
    function: Option<Diagnostic>,
    /// The run of `v as T`: pass with the value equal to the one supplied,
    /// or its failure; and its steps.
    as_run: Result<bool, Diagnostic>,
    as_steps: u64,
    /// The run of `v is T`, and its steps.
    is_run: Result<bool, Diagnostic>,
    is_steps: u64,
}

fn conform_case(case: &Case) -> Result<Conformed, Error> {
    let (text, value) = case.conform.as_ref().expect("a conform case");
    let mut interner = Interner::default();
    let unmatched = Arc::new(AtomicBool::new(false));
    let env = build_env(case, &mut interner, &unmatched)?;
    let pt = env.ty(text)?;
    let data = interner.data(value);
    let function = pt.conform(&data).err();
    let child = env.child().var("v", "any").build()?;
    let mut vars = BTreeMap::new();
    vars.insert("v".to_string(), data.clone());
    let one = |form: &str| -> (Result<bool, Diagnostic>, u64) {
        let p = match parse(&format!("v {form} {text}")).and_then(|p| compile(&child, p)) {
            Ok(p) => p,
            Err(e) => return (Err(e.diagnostics[0].clone()), 0),
        };
        let out = p.run(&vars);
        let steps = out.usage.steps;
        let r = match out.result {
            Ok(Value::Bool(b)) if form == "is" => Ok(b),
            Ok(v) if form == "as" => Ok(eq_data(&v, &data)),
            Ok(_) => Ok(false),
            Err(f) => Err(p.diagnostic(&f)),
        };
        (r, steps)
    };
    let (as_run, as_steps) = one("as");
    let (is_run, is_steps) = one("is");
    Ok(Conformed {
        function,
        as_run,
        as_steps,
        is_run,
        is_steps,
    })
}

fn path_json(p: &[crate::diag::PathStep]) -> Json {
    Json::Arr(
        p.iter()
            .map(|s| match s {
                crate::diag::PathStep::Key(k) => Json::Str(k.clone()),
                crate::diag::PathStep::Index(i) => Json::Num(*i as f64),
            })
            .collect(),
    )
}

impl Conformed {
    /// The answer of each of the three ways, and whether they agree in
    /// answer, path, message and steps.
    fn agree(&self) -> bool {
        let f = self.function.is_none();
        let a = matches!(self.as_run, Ok(true));
        let i = matches!(self.is_run, Ok(true));
        let same_fail = match (&self.function, &self.as_run) {
            (None, Ok(true)) => true,
            (Some(d), Err(e)) => {
                e.code == "not-conforming" && d.path == e.path && d.message == e.message
            }
            _ => false,
        };
        f == a && a == i && same_fail && self.is_run.is_ok() && self.as_steps == self.is_steps
    }
}

/// A conform case (F4 9.3).
fn run_conform_case(case: &Case) -> Verdict {
    let c = match conform_case(case) {
        Ok(c) => c,
        Err(e) => {
            let unmatched = AtomicBool::new(false);
            return judge_failure(case, e.phase, &e.diagnostics, None, &unmatched, true);
        }
    };
    let Expect::Result(ExpectResult {
        conform: Some(want),
        ..
    }) = &case.expect
    else {
        return Verdict::Fail(vec![Difference {
            name: "outcome",
            expected: "a failure of phase declare".into(),
            found: "a conform result".into(),
        }]);
    };
    let mut diffs = Vec::new();
    let answers = [
        c.function.is_none(),
        matches!(c.as_run, Ok(true)),
        matches!(c.is_run, Ok(true)),
    ];
    let as_ok = match &c.as_run {
        Ok(_) => true,
        Err(d) => d.code == "not-conforming",
    };
    if answers.iter().any(|a| *a != want.conforms) || !as_ok || c.is_run.is_err() {
        diffs.push(Difference {
            name: "conforms",
            expected: want.conforms.to_string(),
            found: format!(
                "function {}, as {:?}, is {:?}",
                answers[0],
                c.as_run.as_ref().map_err(show_diag),
                c.is_run.as_ref().map_err(show_diag)
            ),
        });
    }
    if let Some(p) = &want.path {
        let want = Json::Arr(p.clone());
        let got_f = c
            .function
            .as_ref()
            .and_then(|d| d.path.as_deref())
            .map(|p| path_json(p));
        let got_a = c
            .as_run
            .as_ref()
            .err()
            .and_then(|d| d.path.as_deref())
            .map(|p| path_json(p));
        let ok = |g: &Option<Json>| g.as_ref().is_some_and(|g| crate::json::same_data(&want, g));
        if !ok(&got_f) || !ok(&got_a) {
            diffs.push(Difference {
                name: "path",
                expected: crate::json::to_text(&want),
                found: format!("{got_f:?} / {got_a:?}"),
            });
        }
    }
    if let Some(m) = &want.message {
        let got_f = c.function.as_ref().map(|d| d.message.clone());
        let got_a = c.as_run.as_ref().err().map(|d| d.message.clone());
        if got_f.as_deref() != Some(m.as_str()) || got_a.as_deref() != Some(m.as_str()) {
            diffs.push(Difference {
                name: "message",
                expected: m.clone(),
                found: format!("{got_f:?} / {got_a:?}"),
            });
        }
    }
    if let Some(st) = want.steps {
        if c.as_steps != st || c.is_steps != st {
            diffs.push(Difference {
                name: "steps",
                expected: st.to_string(),
                found: format!("as {}, is {}", c.as_steps, c.is_steps),
            });
        }
    }
    verdict(diffs)
}

/// An analysis case (F2 4.2): Read, Parse, Analyze, once.
fn run_analysis_case(case: &Case, want: &Json) -> Verdict {
    let Some(text) = analysis_text(case) else {
        let e = parse_case(case).err();
        return Verdict::Fail(vec![Difference {
            name: "outcome",
            expected: "a report".into(),
            found: match e {
                Some(e) => format!("a failure of phase parse: {}", show_all(&e.diagnostics)),
                None => "no report".into(),
            },
        }]);
    };
    let got = crate::json::parse(&text).expect("a report is JSON");
    let mut bad = Vec::new();
    if let Some(m) = want.as_obj() {
        for (k, v) in m {
            if !got.get(k).is_some_and(|g| crate::json::same_data(v, g)) {
                bad.push(k.clone());
            }
        }
    }
    if bad.is_empty() {
        Verdict::Pass
    } else {
        Verdict::Fail(vec![Difference {
            name: "analysis",
            expected: crate::json::to_text(want),
            found: format!("{text} (differs in {})", bad.join(", ")),
        }])
    }
}

/// One line of the outcome command, with F2's `analysis` last for every case
/// whose text parses (4.8).
pub fn outcome_line(case: &Case) -> String {
    if let Expect::Result(ExpectResult {
        analysis: Some(_), ..
    }) = &case.expect
    {
        let mut out = String::from("{\"name\":");
        out.push_str(&json_text(&case.name));
        match parse_case(case) {
            Ok(_) => out.push_str(",\"phase\":null"),
            Err(e) => {
                out.push_str(",\"phase\":\"parse\",\"diagnostics\":[");
                if let Some(d) = e.diagnostics.first() {
                    let [a, b, c, f] = d.span.map(|s| s.0).unwrap_or([0; 4]);
                    out.push_str(&format!(
                        "{{\"code\":\"{}\",\"span\":[{a},{b},{c},{f}]}}",
                        d.code
                    ));
                }
                out.push(']');
            }
        }
        if let Some(a) = analysis_text(case) {
            out.push_str(",\"analysis\":");
            out.push_str(&a);
        }
        out.push('}');
        return out;
    }
    if case.conform.is_some() {
        let mut out = String::from("{\"name\":");
        out.push_str(&json_text(&case.name));
        match conform_case(case) {
            Err(e) => {
                out.push_str(",\"phase\":\"declare\",\"diagnostics\":[");
                let mut codes: Vec<&str> = e.diagnostics.iter().map(|d| d.code).collect();
                codes.sort();
                out.push_str(
                    &codes
                        .iter()
                        .map(|c| format!("{{\"code\":\"{c}\"}}"))
                        .collect::<Vec<_>>()
                        .join(","),
                );
                out.push(']');
            }
            Ok(c) => {
                out.push_str(",\"phase\":null");
                out.push_str(&format!(",\"conforms\":{}", c.function.is_none()));
                if let Some(d) = &c.function {
                    out.push_str(",\"path\":");
                    out.push_str(&crate::json::to_text(&path_json(
                        d.path.as_deref().map(|p| p.as_slice()).unwrap_or(&[]),
                    )));
                    out.push_str(",\"message\":");
                    out.push_str(&json_text(&d.message));
                }
                out.push_str(&format!(
                    ",\"steps\":{},\"agree\":{}",
                    c.as_steps,
                    c.agree()
                ));
            }
        }
        out.push('}');
        return out;
    }
    let mut out = outcome_core(case);
    if let Some(a) = analysis_text(case) {
        out.pop();
        out.push_str(",\"analysis\":");
        out.push_str(&a);
        out.push('}');
    }
    out
}

fn outcome_core(case: &Case) -> String {
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
    // F1 7.4: the description, when the case has `schemas`.
    if !case.schemas.is_empty() {
        out.push_str(",\"description\":");
        out.push_str(&env.description_text());
    }
    // The full checker at step 4 (S4): its result type and sites are
    // written after the name when the program checks.
    let program = match parse_case(case) {
        Err(e) => Err((Phase::Parse, e.diagnostics)),
        // `suite/README.md` item 3: a case with `"check": false` is compiled without.
        Ok(p) if !case.check => compile(&env, p).map_err(|e| (e.phase, e.diagnostics)),
        Ok(p) => match crate::check::expected(&env, case.returns.as_deref()) {
            Err(d) => Err((Phase::Declare, vec![d])),
            Ok(exp) => match crate::check::compile_checked(&env, p, &exp) {
                Err(e) => Err((e.phase, e.diagnostics)),
                Ok((prog, r)) => {
                    let ty = crate::check::ty::print(r.ty.as_ref().unwrap_or(&Default::default()));
                    out.push_str(",\"type\":");
                    out.push_str(&json_text(&ty));
                    out.push_str(",\"sites\":[");
                    for (i, s) in r.sites.iter().enumerate() {
                        let [a, b, c, d] = prog.parsed.source.line_span(s.span).0;
                        if i > 0 {
                            out.push(',');
                        }
                        out.push_str(&format!(
                            "{{\"class\":\"{}\",\"span\":[{a},{b},{c},{d}]}}",
                            s.class
                        ));
                    }
                    out.push(']');
                    Ok(prog)
                }
            },
        },
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
    let mut result = run.result.map_err(|f| program.diagnostic(&f));
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
    // `suite/README.md` item 2: a failure of class `limit` has no span and no usage.
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
/// This conversion is the binding's, outside the run (`02-evaluation.md` 8.4:
/// handing a result over is not a read): it walks with an explicit stack, so data of any depth
/// neither fails nor overflows the host stack. Only a host number that cannot
/// enter fails it, with `number-range`.
fn value_json(v: &Value<'_>, out: &mut String, _depth: u32) -> Result<(), &'static str> {
    enum Item<'v> {
        Val(Value<'v>),
        Text(&'static str),
        Key(String),
    }
    let mut stack = vec![Item::Val(v.clone())];
    while let Some(item) = stack.pop() {
        match item {
            Item::Text(t) => out.push_str(t),
            Item::Key(k) => {
                out.push_str(&json_text(&k));
                out.push(':');
            }
            Item::Val(v) => match &v {
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
                    out.push('[');
                    stack.push(Item::Text("]"));
                    let n = v.len().unwrap_or(0);
                    for i in (0..n).rev() {
                        stack.push(Item::Val(v.elem(i)));
                        if i > 0 {
                            stack.push(Item::Text(","));
                        }
                    }
                }
                Value::Map(_) | Value::HMap(_) => {
                    out.push('{');
                    stack.push(Item::Text("}"));
                    let es = v.entries();
                    for (i, (k, x)) in es.into_iter().enumerate().rev() {
                        stack.push(Item::Val(x));
                        stack.push(Item::Key(k.as_str().to_string()));
                        if i > 0 {
                            stack.push(Item::Text(","));
                        }
                    }
                }
            },
        }
    }
    Ok(())
}
