//! Running cases and comparing outcomes (`03-host.md` 3.10.5 to 3.10.8).
//!
//! The runner has no type checker: it applies the static rules alone, and
//! gives `skip` to a case that needs the checker (3.10.8 item 5) or a
//! library entry this build does not run yet.

use super::case::{
    load_file, opaque_of, Answer, Case, Expect, ExpectDiag, ExpectFailure, ExpectResult, Stub,
};
use crate::compile::{compile_unchecked as compile, Program, Res};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::env::{check_expected, Env, HostImpl, Limits};
use crate::eval::Call;
use crate::host::{Data, HostError, OpaqueRef};
use crate::json::Json;
use crate::library;
use crate::render::View;
use crate::syntax::ast::NodeKind;
use crate::syntax::parse;
use crate::value::{eq_data, Value};
use std::collections::{BTreeMap, HashMap};
use std::fmt;
use std::path::{Path, PathBuf};
use std::sync::Arc;

/// The first call of a stub that no row matched (3.10.4), as the report
/// writes it (`f09-testing.md` 6.2).
#[derive(Debug, Default)]
pub struct Unmatched(std::sync::Mutex<Option<String>>);

impl Unmatched {
    fn store(&self, text: String) {
        if let Ok(mut g) = self.0.lock() {
            if g.is_none() {
                *g = Some(text);
            }
        }
    }

    fn load(&self) -> Option<String> {
        self.0.lock().ok().and_then(|g| g.clone())
    }
}

/// The JSON form of `01-language.md` 12 (`f09-testing.md` 1.1): no white
/// space, keys in code point order, numbers in the rendering of 6.3.
pub fn form(j: &Json) -> String {
    let mut out = String::new();
    form_into(j, &mut out);
    out
}

fn form_into(j: &Json, out: &mut String) {
    match j {
        Json::Null => out.push_str("null"),
        Json::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
        Json::Num(n) => out.push_str(&crate::number::to_text(*n)),
        Json::Str(s) => out.push_str(&json_text(s)),
        Json::Arr(a) => {
            out.push('[');
            for (i, x) in a.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                form_into(x, out);
            }
            out.push(']');
        }
        Json::Obj(m) => {
            let mut keys: Vec<&(String, Json)> = m.iter().collect();
            keys.sort_by(|a, b| a.0.cmp(&b.0));
            out.push('{');
            for (i, (k, x)) in keys.into_iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                out.push_str(&json_text(k));
                out.push(':');
                form_into(x, out);
            }
            out.push('}');
        }
    }
}

/// A value of a run as JSON data, an opaque value as a case file writes it.
pub fn value_to_json(v: &Value<'_>) -> Json {
    match v {
        Value::Null | Value::BadNum => Json::Null,
        Value::Bool(b) => Json::Bool(*b),
        Value::Num(n) => Json::Num(*n),
        Value::Str(s) => Json::Str(s.as_str().to_string()),
        Value::Opaque(o) => {
            let id = o.value.downcast_ref::<u64>().copied().unwrap_or(0);
            Json::Obj(vec![(
                "$opaque".into(),
                Json::Obj(vec![
                    ("type".into(), Json::Str(o.type_name.to_string())),
                    ("id".into(), Json::Num(id as f64)),
                ]),
            )])
        }
        Value::List(_) | Value::HList(_) => Json::Arr(
            (0..v.len().unwrap_or(0))
                .map(|i| value_to_json(&v.elem(i)))
                .collect(),
        ),
        Value::Map(_) | Value::HMap(_) => Json::Obj(
            v.entries()
                .into_iter()
                .map(|(k, x)| (k.as_str().to_string(), value_to_json(&x)))
                .collect(),
        ),
    }
}

/// The fixture test of `f09-testing.md` 2: the first fixture that does not
/// conform, with its place and the conformance path, and the message.
fn fixture_test(case: &Case, env: &Env) -> Option<(Vec<Json>, String)> {
    if case.precondition == Some(false) {
        return None;
    }
    let data = env.data();
    let mut interner = Interner::default();
    let test = |interner: &mut Interner, v: &Json, ty: crate::check::ty::Ty| {
        let d = interner.data(v);
        crate::conform::ProgramType { ty }.conform(&d).err()
    };
    let with_path = |mut place: Vec<Json>, d: &Diagnostic| {
        if let Some(p) = &d.path {
            if let Json::Arr(a) = path_json(p) {
                place.extend(a);
            }
        }
        (place, d.message.clone())
    };
    // 2.3 item 1: the values of `vars`, in code point order of their keys.
    let mut vars: Vec<&(String, Json)> = case.vars.iter().flatten().collect();
    vars.sort_by(|a, b| a.0.cmp(&b.0));
    for (k, v) in vars {
        let Some(i) = data.vars.iter().position(|n| n == k) else {
            continue;
        };
        if let Some(d) = test(&mut interner, v, data.var_types[i].clone()) {
            let place = vec![Json::Str("vars".into()), Json::Str(k.clone())];
            return Some(with_path(place, &d));
        }
    }
    // F14 10.2: the host facts of `facts`, in order. One is tested only when
    // `vars` supplies every member, and fails when every member is null.
    for (i, f) in case.facts.iter().flatten().enumerate() {
        let value = |n: &String| {
            case.vars
                .iter()
                .flatten()
                .find(|(k, _)| k == n)
                .map(|(_, v)| v)
        };
        let all_null = f.iter().all(|n| matches!(value(n), Some(Json::Null)));
        if all_null {
            let place = vec![Json::Str("facts".into()), Json::Num(i as f64)];
            let msg = format!("at least one of {} must not be null", f.join(", "));
            return Some((place, msg));
        }
    }
    let any: crate::check::ty::Solutions = ('A'..='Z')
        .map(|c| (c, crate::check::ty::Ty::any()))
        .collect();
    let rows = |interner: &mut Interner,
                kind: &str,
                name: &str,
                stub: &Stub,
                sig: &crate::types::text::Sig|
     -> Option<(Vec<Json>, String)> {
        let result = crate::check::ty::substitute(&sig.result, &data.names, &any);
        for (i, r) in stub.calls.iter().enumerate() {
            let place = |last: &str| {
                vec![
                    Json::Str(kind.into()),
                    Json::Str(name.into()),
                    Json::Str("calls".into()),
                    Json::Num(i as f64),
                    Json::Str(last.into()),
                ]
            };
            match &r.answer {
                Answer::Result(v) => {
                    if let Some(d) = test(interner, v, result.clone()) {
                        return Some(with_path(place("result"), &d));
                    }
                }
                Answer::Error(_) if !sig.fails => {
                    return Some((
                        place("error"),
                        "the signature has no !, so the stub may not return an error".into(),
                    ));
                }
                Answer::Error(_) => {}
            }
        }
        None
    };
    // 2.3 items 2 and 3: the stubs of functions, then of methods, by name.
    let mut fs: Vec<&(String, Stub)> = case.functions.iter().collect();
    fs.sort_by(|a, b| a.0.cmp(&b.0));
    for (name, stub) in fs {
        if let Some(f) = data.functions.iter().find(|f| f.name == *name) {
            if let Some(r) = rows(&mut interner, "functions", name, stub, &f.sig) {
                return Some(r);
            }
        }
    }
    let mut ms: Vec<&(String, Stub)> = case.methods.iter().collect();
    ms.sort_by(|a, b| a.0.cmp(&b.0));
    for (key, stub) in ms {
        let (t, m) = key.split_once('.').unwrap_or_default();
        if let Some(h) = data
            .methods
            .iter()
            .find(|h| h.type_name == t && h.name == m)
        {
            if let Some(r) = rows(&mut interner, "methods", key, stub, &h.sig) {
                return Some(r);
            }
        }
    }
    None
}

/// A span as JSON.
pub(super) fn span_json(s: [u32; 4]) -> Json {
    Json::Arr(s.iter().map(|n| Json::Num(*n as f64)).collect())
}

/// E of the difference `outcome`: the whole `expect`, or `nothing`.
fn expected_text(case: &Case) -> String {
    match &case.expect_raw {
        Some(e) => form(e),
        None => "nothing".into(),
    }
}

/// A result as an `expect` written whole (`f09-testing.md` 4.4): `type` when
/// checked; `value`, `steps` and `allocated` when it ran.
pub(super) fn result_json(ty: Option<&str>, ran: Option<(Json, u64, u64)>) -> Json {
    let mut m = Vec::new();
    if let Some(t) = ty {
        m.push(("type".to_string(), Json::Str(t.to_string())));
    }
    if let Some((v, steps, units)) = ran {
        m.push(("value".into(), v));
        m.push(("steps".into(), Json::Num(steps as f64)));
        m.push(("allocated".into(), Json::Num(units as f64)));
    }
    Json::Obj(m)
}

/// The columns a phase may write (`f09-testing.md` 4.4), written whole.
pub(super) fn whole_columns(phase: Phase) -> &'static [&'static str] {
    match phase {
        Phase::Declare => &["code"],
        Phase::Parse => &["code", "span", "message"],
        Phase::Check => &["code", "class", "span", "message"],
        Phase::Run => &["code", "class", "span", "path", "message", "data"],
    }
}

/// One diagnostic with the columns named that it has (`f09-testing.md`
/// 4.4): `class` when it has one; `span` and `trace` never for a limit
/// code; `path` for `not-conforming`; `data` for `failed`; no message for
/// `declare`.
pub fn diag_json(d: &Diagnostic, cols: &[&str]) -> Json {
    let limit = code::LIMIT.contains(&d.code);
    let mut m = Vec::new();
    for c in cols {
        match *c {
            "code" => m.push(("code".to_string(), Json::Str(d.code.to_string()))),
            "class" => {
                if let Some(c) = d.class {
                    m.push(("class".into(), Json::Str(c.to_string())));
                }
            }
            "span" => {
                if let (Some(s), false) = (d.span, limit) {
                    m.push(("span".into(), span_json(s.0)));
                }
            }
            "message" if d.phase != Phase::Declare => {
                m.push(("message".into(), Json::Str(d.message.clone())))
            }
            "path" if d.code == "not-conforming" => {
                if let Some(p) = &d.path {
                    m.push(("path".into(), path_json(p)));
                }
            }
            "data" if d.code == "failed" => {
                let v = d
                    .data
                    .as_deref()
                    .map(|x| value_to_json(&Value::from_data(x)))
                    .unwrap_or(Json::Null);
                m.push(("data".into(), v));
            }
            "trace" if d.phase == Phase::Run && !limit => {
                let t = d
                    .trace()
                    .iter()
                    .map(|e| {
                        Json::Obj(vec![
                            ("span".into(), span_json(e.span.0)),
                            ("name".into(), Json::Str(e.name.clone())),
                            (
                                "index".into(),
                                e.index.map_or(Json::Null, |i| Json::Num(i as f64)),
                            ),
                        ])
                    })
                    .collect();
                m.push(("trace".into(), Json::Arr(t)));
            }
            _ => {}
        }
    }
    Json::Obj(m)
}

/// A failure as an `expect`: `phase` and `diagnostics`, with the columns
/// given, or written whole.
pub(super) fn failure_json(phase: Phase, diags: &[Diagnostic], cols: Option<&[String]>) -> Json {
    let whole: Vec<String> = whole_columns(phase).iter().map(|c| c.to_string()).collect();
    let cols: Vec<&str> = cols.unwrap_or(&whole).iter().map(|c| c.as_str()).collect();
    let ds: &[Diagnostic] = match phase {
        Phase::Parse | Phase::Run => &diags[..diags.len().min(1)],
        _ => diags,
    };
    Json::Obj(vec![
        ("phase".into(), Json::Str(phase.as_str().into())),
        (
            "diagnostics".into(),
            Json::Arr(ds.iter().map(|d| diag_json(d, &cols)).collect()),
        ),
    ])
}

/// One thing that differed (3.10.8 item 2). `expected` and `found` are the
/// E and F of `f09-testing.md` 6.2; for `functions`, `found` is the text
/// after the name.
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
    /// F9 2.4: a fixture that fails its test, with its place and path and
    /// the message.
    Fixture(Vec<Json>, String),
    /// F9 4.3: written by update mode.
    Updated,
}

impl Verdict {
    pub fn label(&self) -> &'static str {
        match self {
            Verdict::Pass => "pass",
            Verdict::Fail(_) => "fail",
            Verdict::Skip(_) => "skip",
            Verdict::Invalid(_) | Verdict::Fixture(..) => "invalid",
            Verdict::Updated => "updated",
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
    pub updated: usize,
}

impl Counts {
    pub fn add(&mut self, v: &Verdict) {
        match v {
            Verdict::Pass => self.pass += 1,
            Verdict::Fail(_) => self.fail += 1,
            Verdict::Skip(_) => self.skip += 1,
            Verdict::Invalid(_) | Verdict::Fixture(..) => self.invalid += 1,
            Verdict::Updated => self.updated += 1,
        }
    }

    pub fn merge(&mut self, o: Counts) {
        self.pass += o.pass;
        self.fail += o.fail;
        self.skip += o.skip;
        self.invalid += o.invalid;
        self.updated += o.updated;
    }

    pub fn total(&self) -> usize {
        self.pass + self.fail + self.skip + self.invalid + self.updated
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
                Verdict::Fixture(fx, msg) => writeln!(
                    f,
                    "  invalid: {name}: fixture {}: {msg}",
                    form(&Json::Arr(fx.clone()))
                )?,
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
    match load_file(path) {
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
            results: vec![("(every case of the file)".into(), Verdict::Invalid(why.0))],
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
fn stub_impl(
    name: &str,
    stub: &Stub,
    interner: &mut Interner,
    unmatched: Arc<Unmatched>,
) -> HostImpl {
    let name = name.to_string();
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
        let args: Vec<Json> = args.iter().map(value_to_json).collect();
        unmatched.store(format!(
            "no row of {name} matches {}",
            form(&Json::Arr(args))
        ));
        Err(HostError("no row of the stub matches the call".into()))
    })
}

/// The case's environment, as a child of the suite's base environment.
fn build_env(
    case: &Case,
    interner: &mut Interner,
    unmatched: &Arc<Unmatched>,
) -> Result<Env, Error> {
    build_env_in(case, interner, unmatched, None)
}

/// The case's environment, as a child of `base` when one is given
/// (`03-host.md` 3.10.9; `f09-testing.md` 7.1).
fn build_env_in(
    case: &Case,
    interner: &mut Interner,
    unmatched: &Arc<Unmatched>,
    base: Option<&Env>,
) -> Result<Env, Error> {
    let mut b = match base {
        Some(e) => e.child(),
        None => Env::builder(),
    };
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
    for g in &case.groups {
        b = b.group(g);
    }
    for f in case.facts.iter().flatten() {
        let names: Vec<&str> = f.iter().map(String::as_str).collect();
        b = b.some_not_null(&names);
    }
    for (key, stub) in &case.methods {
        let imp = stub_impl(key, stub, interner, unmatched.clone());
        b = b.method_impl(key, stub.ty.as_deref().unwrap_or_default(), Some(imp));
    }
    for (name, stub) in &case.functions {
        let imp = stub_impl(name, stub, interner, unmatched.clone());
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
            // F10 2.1 item 6: the schema is mapped in either mode.
            if let Some((sch, o)) = &case.returns_schema {
                if let Err(d) = crate::check::expected_schema(sch, *o) {
                    return Err(Error::one(d));
                }
            }
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

/// The expected result of a case: by `returns`, or by schema (F10 10.1).
fn case_expected(case: &Case, env: &Env) -> Result<crate::check::Expected, Diagnostic> {
    match &case.returns_schema {
        Some((s, o)) => crate::check::expected_schema(s, *o),
        None => crate::check::expected(env, case.returns.as_deref()),
    }
}

/// F10 10.2 item 4, step 5a: the conformance function on a load's value and
/// its expected type. A failure has no span.
fn load_step(case: &Case, env: &Env, v: &Value<'_>) -> Option<Diagnostic> {
    if !case.load {
        return None;
    }
    match case_expected(case, env) {
        Ok(crate::check::Expected::Type(ty)) => {
            crate::conform::ProgramType { ty }.conform_value(v).err()
        }
        _ => None,
    }
}

/// The environment a case declares (3.10.5 step 2), with its stubs as the
/// implementations of its functions and methods.
pub fn case_env(case: &Case) -> Result<Env, Error> {
    let mut interner = Interner::default();
    let unmatched = Arc::new(Unmatched::default());
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

/// What a helper gives a run (`f09-testing.md` 7.1): a base environment,
/// and variables for the names a case does not supply.
#[derive(Clone, Copy, Default)]
pub struct Ctx<'a> {
    pub base: Option<&'a Env>,
    pub vars: Option<&'a dyn crate::host::Vars>,
}

/// The case's variables over the host's.
struct Layered<'a> {
    case: &'a BTreeMap<String, Data>,
    host: Option<&'a dyn crate::host::Vars>,
}

impl crate::host::Vars for Layered<'_> {
    fn get(&self, name: &str) -> Option<crate::host::View<'_>> {
        match self.case.get(name) {
            Some(d) => Some(crate::host::Host::view(d)),
            None => self.host.and_then(|h| h.get(name)),
        }
    }
}

/// Run one case with a helper's base and variables.
pub fn run_case_ctx(case: &Case, ctx: Ctx<'_>) -> Verdict {
    run_case_full(case, false, ctx)
}

/// As `run_case`, for a case of `suite/core-check/` when `core_check`.
pub fn run_case_in(case: &Case, core_check: bool) -> Verdict {
    run_case_full(case, core_check, Ctx::default())
}

fn run_case_full(case: &Case, core_check: bool, ctx: Ctx<'_>) -> Verdict {
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
    if let Some(r) = &case.render {
        return run_render_case(r);
    }
    if let Some(r) = &case.runner {
        return run_runner_case(r);
    }
    if let Some(t) = &case.tool {
        return super::tools::run(case, t);
    }
    if case.conform.is_some() {
        return run_conform_case(case);
    }
    if !case.check {
        return run_case_with(case, false, true, ctx);
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
        || case.load
        || !case.schemas.is_empty()
        || matches!(&case.expect, Expect::Result(r) if r.description.is_some());
    let checked = run_case_with(case, true, true, ctx);
    if core_check || needs_checker {
        return checked;
    }
    let plain = run_case_with(case, false, false, ctx);
    match (checked, plain) {
        (Verdict::Pass, Verdict::Pass) => Verdict::Pass,
        // F9 2.2: the test does not depend on the check.
        (v @ Verdict::Fixture(..), _) | (_, v @ Verdict::Fixture(..)) => v,
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
                "json",
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
    // F9 9.3 item 2: a runner case is run once, and has no second pass.
    if let Some(r) = &case.runner {
        return run_runner_case(r);
    }
    if let Some(t) = &case.tool {
        if let Some(why) = &case.skip {
            return Verdict::Skip(why.clone());
        }
        return super::tools::run(case, t);
    }
    if let Some(r) = &case.render {
        return run_render_case(r);
    }
    if let Expect::Result(ExpectResult {
        analysis: Some(want),
        ..
    }) = &case.expect
    {
        return run_analysis_case(case, want);
    }
    if !case.check {
        return run_case_with(case, false, true, Ctx::default());
    }
    run_case_with(case, false, false, Ctx::default())
}

/// `all_class`: compare `class` for every code, as a checked run and a case
/// with `"check": false` do (3.10.7 item 8).
fn run_case_with(case: &Case, checked: bool, all_class: bool, ctx: Ctx<'_>) -> Verdict {
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
    let unmatched = Arc::new(Unmatched::default());
    let env = match build_env_in(case, &mut interner, &unmatched, ctx.base) {
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
                expected: form(want),
                found: form(&got),
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
    let mut found_ty: Option<String> = None;
    if checked {
        let exp = match case_expected(case, &env) {
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
        found_ty = r.ty.as_ref().map(crate::check::ty::print);
        if let Expect::Result(want) = &case.expect {
            let got = r.ty.clone().unwrap_or_default();
            if let Some(t) = &want.ty {
                let wt = crate::types::text::parse_type(t)
                    .ok()
                    .and_then(|x| crate::check::ty::from_text(&x, &env.data().names, None).ok());
                if wt.as_ref() != Some(&got) {
                    diffs.push(Difference {
                        name: "type",
                        expected: json_text(t),
                        found: json_text(&crate::check::ty::print(&got)),
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
                    let list = |l: &[(String, [u32; 4])]| {
                        Json::Arr(
                            l.iter()
                                .map(|(c, sp)| {
                                    Json::Obj(vec![
                                        ("class".into(), Json::Str(c.clone())),
                                        ("span".into(), span_json(*sp)),
                                    ])
                                })
                                .collect(),
                        )
                    };
                    diffs.push(Difference {
                        name: "sites",
                        expected: form(&list(&ws)),
                        found: form(&list(&gs)),
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
        || case.load
        || match &case.expect {
            Expect::Result(r) => r.needs_run(),
            Expect::Failure(f) => f.phase == Phase::Run,
        };
    if !needs_run {
        return match &case.expect {
            Expect::Result(_) => verdict(diffs),
            Expect::Failure(_) => Verdict::Fail(vec![Difference {
                name: "outcome",
                expected: expected_text(case),
                found: form(&result_json(found_ty.as_deref(), None)),
            }]),
        };
    }
    // F9 2.2: the fixture test, before the run.
    if let Some((fx, msg)) = fixture_test(case, &env) {
        return Verdict::Fixture(fx, msg);
    }
    let vars: BTreeMap<String, Data> = case
        .vars
        .iter()
        .flatten()
        .map(|(k, v)| (k.clone(), interner.data(v)))
        .collect();
    let layered = Layered {
        case: &vars,
        host: ctx.vars,
    };
    let out = program.run(&layered);
    let usage = out.usage;
    if let Ok(v) = &out.result {
        if let Some(d) = load_step(case, &env, v) {
            return judge_failure(
                case,
                Phase::Run,
                &[d],
                Some((usage.steps, usage.allocated)),
                &unmatched,
                all_class,
            );
        }
    }
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
                Expect::Failure(_) => diffs.push(Difference {
                    name: "outcome",
                    expected: expected_text(case),
                    found: form(&result_json(
                        found_ty.as_deref(),
                        Some((value_to_json(&v), usage.steps, usage.allocated)),
                    )),
                }),
                Expect::Result(r) => {
                    if let Some(want_json) = &r.value {
                        let want = interner.data(want_json);
                        if !eq_data(&v, &want) {
                            diffs.push(Difference {
                                name: "value",
                                expected: form(r.value.as_ref().unwrap_or(&Json::Null)),
                                found: form(&value_to_json(&v)),
                            });
                        }
                    }
                    // F6 11.2: the binding's JSON text of the value.
                    if let Some(want) = &r.json {
                        let got = crate::eval::to_json(&v);
                        if got.as_deref() != Ok(want.as_str()) {
                            diffs.push(Difference {
                                name: "json",
                                expected: json_text(want),
                                found: match &got {
                                    Ok(t) => json_text(t),
                                    Err(e) => json_text(&format!("{e:?}")),
                                },
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

fn functions_diff(diffs: &mut Vec<Difference>, unmatched: &Unmatched) {
    if let Some(found) = unmatched.load() {
        diffs.push(Difference {
            name: "functions",
            expected: String::new(),
            found,
        });
    }
}

fn judge_failure(
    case: &Case,
    phase: Phase,
    diags: &[Diagnostic],
    usage: Option<(u64, u64)>,
    unmatched: &Unmatched,
    checked: bool,
) -> Verdict {
    let mut diffs = match &case.expect {
        Expect::Result(_) => vec![Difference {
            name: "outcome",
            expected: expected_text(case),
            found: form(&failure_json(phase, diags, None)),
        }],
        Expect::Failure(f) => {
            let mut d = compare_failure(case, f, phase, diags, checked);
            if d.is_empty() {
                if let Some(r) = &f.rendered {
                    let ds = if phase == Phase::Parse {
                        &diags[..diags.len().min(1)]
                    } else {
                        diags
                    };
                    let views: Vec<View> = ds.iter().map(View::of).collect();
                    rendered_diffs(&mut d, r, &views, Some(&case.source), "rendered");
                }
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

/// F8 7.3, 7.4: each rendering given, compared whole. `prefix` names the
/// difference: `rendered`, or nothing for a render case, whose
/// differences are `line`, `detail` and `json`.
fn rendered_diffs(
    diffs: &mut Vec<Difference>,
    want: &super::case::Renderings,
    views: &[View<'_>],
    source: Option<&str>,
    prefix: &'static str,
) {
    let forms: [(&'static str, &Option<String>, String); 3] = [
        ("line", &want.line, crate::render::line(views)),
        ("detail", &want.detail, crate::render::detail(views, source)),
        ("json", &want.json, crate::render::json(views)),
    ];
    for (name, w, got) in forms {
        if let Some(w) = w {
            if *w != got {
                diffs.push(Difference {
                    name: if prefix.is_empty() { name } else { prefix },
                    expected: format!("{name}: {w:?}"),
                    found: format!("{got:?}"),
                });
            }
        }
    }
}

/// The diagnostics of a render case, owned, for views of them.
struct Owned {
    phase: String,
    code: String,
    class: Option<String>,
    message: String,
    span: Option<[u32; 4]>,
    path: Option<Vec<crate::diag::PathStep>>,
    data: Option<Data>,
    trace: Vec<([u32; 4], String, Option<usize>)>,
}

fn owned_diags(r: &super::case::RenderCase) -> Vec<Owned> {
    let mut interner = Interner::default();
    r.diagnostics
        .iter()
        .map(|d| {
            let s = |k: &str| d.get(k).and_then(|x| x.as_str()).map(String::from);
            Owned {
                phase: s("phase").unwrap_or_default(),
                code: s("code").unwrap_or_default(),
                class: s("class"),
                message: s("message").unwrap_or_default(),
                span: d.get("span").map(|x| {
                    let mut out = [0u32; 4];
                    for (i, n) in x.as_arr().unwrap_or_default().iter().take(4).enumerate() {
                        out[i] = n.as_f64().unwrap_or(0.0) as u32;
                    }
                    out
                }),
                path: d.get("path").and_then(|p| p.as_arr()).map(|a| {
                    a.iter()
                        .map(|st| match st {
                            Json::Str(k) => crate::diag::PathStep::Key(k.clone()),
                            n => crate::diag::PathStep::Index(n.as_f64().unwrap_or(0.0) as usize),
                        })
                        .collect()
                }),
                data: d.get("data").map(|x| interner.data(x)),
                trace: d
                    .get("trace")
                    .and_then(|t| t.as_arr())
                    .map(|a| {
                        a.iter()
                            .map(|e| {
                                let mut sp = [0u32; 4];
                                if let Some(x) = e.get("span").and_then(|x| x.as_arr()) {
                                    for (i, n) in x.iter().take(4).enumerate() {
                                        sp[i] = n.as_f64().unwrap_or(0.0) as u32;
                                    }
                                }
                                let name = e
                                    .get("name")
                                    .and_then(|n| n.as_str())
                                    .unwrap_or_default()
                                    .to_string();
                                let index = e
                                    .get("index")
                                    .and_then(|n| n.as_integer())
                                    .map(|n| n as usize);
                                (sp, name, index)
                            })
                            .collect()
                    })
                    .unwrap_or_default(),
            }
        })
        .collect()
}

fn views_of(ds: &[Owned]) -> Vec<View<'_>> {
    ds.iter()
        .map(|d| View {
            phase: &d.phase,
            code: &d.code,
            class: d.class.as_deref(),
            message: &d.message,
            span: d.span,
            path: d.path.as_deref(),
            data: d.data.as_ref(),
            trace: d
                .trace
                .iter()
                .map(|(s, n, i)| (*s, n.as_str(), *i))
                .collect(),
        })
        .collect()
}

/// A case of the third kind (`f08-errors.md` 7.4): run once, no program.
fn run_render_case(r: &super::case::RenderCase) -> Verdict {
    let owned = owned_diags(r);
    let views = views_of(&owned);
    let mut diffs = Vec::new();
    rendered_diffs(&mut diffs, &r.expect, &views, r.source.as_deref(), "");
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
    if d.phase != Phase::Declare {
        s.push_str(&format!(" {:?}", d.message));
        if let Some(x) = &d.data {
            s.push_str(&format!(" data {x:?}"));
        }
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
    // F10 10.2 item 5: `"span": null` is compared.
    if e.span_null && a.span.is_some() {
        return false;
    }
    if let Some(m) = &e.message {
        if *m != a.message {
            return false;
        }
    }
    // F8 7.2: the trace, but for a limit code.
    if let Some(t) = &e.trace {
        if !code::LIMIT.contains(&a.code) {
            let got: Vec<([u32; 4], &str, Option<u64>)> = a
                .trace()
                .iter()
                .map(|x| (x.span.0, x.name.as_str(), x.index.map(|i| i as u64)))
                .collect();
            let want: Vec<([u32; 4], &str, Option<u64>)> =
                t.iter().map(|(s, n, i)| (*s, n.as_str(), *i)).collect();
            if got != want {
                return false;
            }
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

/// `compare_failure` of a checked run, for the cases of F12.
pub(super) fn compare_failure_pub(
    case: &Case,
    f: &ExpectFailure,
    phase: Phase,
    diags: &[Diagnostic],
) -> Vec<Difference> {
    compare_failure(case, f, phase, diags, true)
}

fn compare_failure(
    case: &Case,
    f: &ExpectFailure,
    phase: Phase,
    diags: &[Diagnostic],
    checked: bool,
) -> Vec<Difference> {
    if f.phase != phase {
        return vec![Difference {
            name: "outcome",
            expected: expected_text(case),
            found: form(&failure_json(phase, diags, None)),
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
        // F9 6.2: the found list with the columns of the expected one.
        let raw = case
            .expect_raw
            .as_ref()
            .and_then(|e| e.get("diagnostics"))
            .cloned()
            .unwrap_or(Json::Arr(Vec::new()));
        let mut cols: Vec<String> = vec!["code".into()];
        for d in raw.as_arr().unwrap_or_default() {
            for (k, _) in d.as_obj().unwrap_or_default() {
                if !cols.contains(k) {
                    cols.push(k.clone());
                }
            }
        }
        let found = failure_json(phase, diags, Some(&cols));
        vec![Difference {
            name: "diagnostics",
            expected: form(&raw),
            found: form(found.get("diagnostics").unwrap_or(&Json::Null)),
        }]
    }
}

/// One line of the outcome command (`suite/README.md`): what a case found, with
/// its `expect` ignored.
/// The case's program (3.10.5 step 3): its text parsed as a program, or,
/// with F2's `template`, as a template taken in its mode.
pub(super) fn parse_case(case: &Case) -> Result<crate::syntax::Parsed, Error> {
    match case.template {
        Some(mode) => crate::template::parse_template(&case.source).map(|t| t.program(mode)),
        None => parse(&case.source),
    }
}

/// The analysis report of the case's text as JSON text (F2 3), when it
/// parses.
pub(super) fn analysis_text(case: &Case) -> Option<String> {
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
pub(super) struct Conformed {
    /// The conformance function's diagnostic, or none.
    pub(super) function: Option<Diagnostic>,
    /// The run of `v as T`: pass with the value equal to the one supplied,
    /// or its failure; and its steps.
    pub(super) as_run: Result<bool, Diagnostic>,
    pub(super) as_steps: u64,
    /// The run of `v is T`, and its steps.
    pub(super) is_run: Result<bool, Diagnostic>,
    pub(super) is_steps: u64,
    /// F8: the list of `ConformAll`, when the case has `max`.
    pub(super) all: Option<Vec<Diagnostic>>,
}

pub(super) fn conform_case(case: &Case) -> Result<Conformed, Error> {
    let (text, value) = case.conform.as_ref().expect("a conform case");
    let mut interner = Interner::default();
    let unmatched = Arc::new(Unmatched::default());
    let env = build_env(case, &mut interner, &unmatched)?;
    let pt = env.ty(text)?;
    let data = interner.data(value);
    let function = pt.conform(&data).err();
    let all = case.conform_max.map(|m| pt.conform_all(&data, m));
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
        all,
    })
}

pub(super) fn path_json(p: &[crate::diag::PathStep]) -> Json {
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
    pub(super) fn agree(&self) -> bool {
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
            let unmatched = Unmatched::default();
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
            found: answers[0].to_string(),
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
                expected: form(&want),
                found: form(&got_f.or(got_a).unwrap_or(Json::Null)),
            });
        }
    }
    if let Some(m) = &want.message {
        let got_f = c.function.as_ref().map(|d| d.message.clone());
        let got_a = c.as_run.as_ref().err().map(|d| d.message.clone());
        if got_f.as_deref() != Some(m.as_str()) || got_a.as_deref() != Some(m.as_str()) {
            diffs.push(Difference {
                name: "message",
                expected: json_text(m),
                found: got_f.or(got_a).map_or("null".into(), |g| json_text(&g)),
            });
        }
    }
    if let Some(st) = want.steps {
        if c.as_steps != st || c.is_steps != st {
            diffs.push(Difference {
                name: "steps",
                expected: st.to_string(),
                found: if c.as_steps != st {
                    c.as_steps
                } else {
                    c.is_steps
                }
                .to_string(),
            });
        }
    }
    // F8 7.5: `ConformAll`, and its agreement with the function of F4 7.
    if let Some(all) = &c.all {
        let first = all.first();
        let agrees = match (&c.function, first) {
            (None, None) => true,
            (Some(d), Some(a)) => d.path == a.path && d.message == a.message,
            _ => false,
        };
        if !agrees && !diffs.iter().any(|d| d.name == "conforms") {
            diffs.push(Difference {
                name: "conforms",
                expected: (c.function.is_none()).to_string(),
                found: first.is_none().to_string(),
            });
        }
        if let Some(fs) = &want.failures {
            let same = fs.len() == all.len()
                && fs.iter().zip(all).all(|((p, m), d)| {
                    let path = path_json(d.path.as_deref().map(|p| p.as_slice()).unwrap_or(&[]));
                    crate::json::same_data(&Json::Arr(p.clone()), &path)
                        && m.as_ref().is_none_or(|m| *m == d.message)
                });
            if !same {
                diffs.push(Difference {
                    name: "failures",
                    expected: form(&Json::Arr(
                        fs.iter()
                            .map(|(p, m)| {
                                let mut o = vec![("path".to_string(), Json::Arr(p.clone()))];
                                if let Some(m) = m {
                                    o.push(("message".into(), Json::Str(m.clone())));
                                }
                                Json::Obj(o)
                            })
                            .collect(),
                    )),
                    found: form(&Json::Arr(
                        all.iter()
                            .map(|d| {
                                let p = path_json(
                                    d.path.as_deref().map(|p| p.as_slice()).unwrap_or(&[]),
                                );
                                Json::Obj(vec![
                                    ("path".into(), p),
                                    ("message".into(), Json::Str(d.message.clone())),
                                ])
                            })
                            .collect(),
                    )),
                });
            }
        }
    }
    let order = [
        "outcome", "conforms", "path", "message", "steps", "failures",
    ];
    diffs.sort_by_key(|d| order.iter().position(|n| *n == d.name));
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
            expected: form(want),
            found: crate::json::parse(&text).map(|g| form(&g)).unwrap_or(text),
        }])
    }
}

/// One line of the outcome command, with F2's `analysis` last for every case
/// whose text parses (4.8).
pub fn outcome_line(case: &Case) -> String {
    if let Some(t) = &case.tool {
        return super::tools::outcome(case, t);
    }
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
    // F9 9.4: a runner case writes its verdicts, its report and its text.
    if let Some(rc) = &case.runner {
        let mut out = String::from("{\"name\":");
        out.push_str(&json_text(&case.name));
        match inner_cases(rc) {
            Err(_) => out.push_str(",\"invalid\":true"),
            Ok(cases) => {
                let (results, text) = if rc.update && !rc.program {
                    let t = match &rc.inner {
                        super::case::Inner::Text(t) => t.clone(),
                        _ => String::new(),
                    };
                    let (rs, new) = super::update::update_text(&t, &cases);
                    (rs, Some(new.unwrap_or(t)))
                } else {
                    let rs: Vec<(String, Verdict)> = cases
                        .iter()
                        .map(|c| (c.name.clone(), run_case(c)))
                        .collect();
                    let t = match (&rc.inner, rc.update) {
                        (super::case::Inner::Text(t), true) => Some(t.clone()),
                        _ => None,
                    };
                    (rs, t)
                };
                let entries: Vec<Json> = results
                    .iter()
                    .map(|(n, v)| {
                        let mut m = vec![
                            ("name".to_string(), Json::Str(n.clone())),
                            ("status".to_string(), Json::Str(v.label().into())),
                        ];
                        if let Verdict::Fail(ds) = v {
                            m.push((
                                "differs".into(),
                                Json::Arr(ds.iter().map(|d| Json::Str(d.name.into())).collect()),
                            ));
                        }
                        if let Verdict::Fixture(fx, msg) = v {
                            m.push(("fixture".into(), Json::Arr(fx.clone())));
                            m.push(("message".into(), Json::Str(msg.clone())));
                        }
                        Json::Obj(m)
                    })
                    .collect();
                out.push_str(",\"verdicts\":");
                out.push_str(&form(&Json::Arr(entries)));
                out.push_str(",\"report\":");
                out.push_str(&json_text(&report_text(&results)));
                if let Some(t) = text {
                    out.push_str(",\"text\":");
                    out.push_str(&json_text(&t));
                }
            }
        }
        out.push('}');
        return out;
    }
    // F8 8: a render case writes its three forms.
    if let Some(r) = &case.render {
        let owned = owned_diags(r);
        let views = views_of(&owned);
        let mut out = String::from("{\"name\":");
        out.push_str(&json_text(&case.name));
        out.push_str(",\"line\":");
        out.push_str(&json_text(&crate::render::line(&views)));
        out.push_str(",\"detail\":");
        out.push_str(&json_text(&crate::render::detail(
            &views,
            r.source.as_deref(),
        )));
        out.push_str(",\"json\":");
        out.push_str(&json_text(&crate::render::json(&views)));
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
                // F8 8: the list of `ConformAll`.
                if let Some(all) = &c.all {
                    out.push_str(",\"failures\":[");
                    for (i, d) in all.iter().enumerate() {
                        if i > 0 {
                            out.push(',');
                        }
                        out.push_str("{\"path\":");
                        out.push_str(&crate::json::to_text(&path_json(
                            d.path.as_deref().map(|p| p.as_slice()).unwrap_or(&[]),
                        )));
                        out.push_str(",\"message\":");
                        out.push_str(&json_text(&d.message));
                        out.push('}');
                    }
                    out.push(']');
                }
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
    let unmatched = Arc::new(Unmatched::default());
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
            }
            // F8 8: the message of a parse, a check and a run diagnostic.
            if phase != Phase::Declare {
                out.push_str(",\"message\":");
                out.push_str(&json_text(&d.message));
            }
            // F10 10.4: the path of a failure of step 5a.
            if let (Phase::Run, Some(p)) = (phase, &d.path) {
                out.push_str(",\"path\":");
                out.push_str(&crate::json::to_text(&path_json(p)));
            }
            // F8 8: the trace of a run diagnostic, but for class `limit`.
            if phase == Phase::Run && d.class != Some("limit") {
                out.push_str(",\"trace\":[");
                for (i, e) in d.trace().iter().enumerate() {
                    if i > 0 {
                        out.push(',');
                    }
                    let [a, b, c, f] = e.span.0;
                    out.push_str(&format!("{{\"span\":[{a},{b},{c},{f}],\"name\":"));
                    out.push_str(&json_text(&e.name));
                    out.push_str(",\"index\":");
                    match e.index {
                        Some(n) => out.push_str(&n.to_string()),
                        None => out.push_str("null"),
                    }
                    out.push('}');
                }
                out.push(']');
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
    // F1 7.4: the description, when the case has `schemas`; F14 10.4: or
    // `facts`.
    if !case.schemas.is_empty() || case.facts.is_some() {
        out.push_str(",\"description\":");
        out.push_str(&env.description_text());
    }
    // The full checker at step 4 (S4): its result type and sites are
    // written after the name when the program checks.
    let program = match parse_case(case) {
        Err(e) => Err((Phase::Parse, e.diagnostics)),
        // `suite/README.md` item 3: a case with `"check": false` is compiled without.
        Ok(p) if !case.check => compile(&env, p).map_err(|e| (e.phase, e.diagnostics)),
        Ok(p) => match case_expected(case, &env) {
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
    // F10 10.4: a load that fails at step 5a.
    if let Ok(v) = &result {
        if let Some(d) = load_step(case, &env, v) {
            result = Err(d);
        }
    }
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

pub(super) fn json_text(s: &str) -> String {
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

/// The lines of a case in the report (`f09-testing.md` 6.1, 6.2).
pub fn case_lines(name: &str, v: &Verdict) -> String {
    let vis = crate::messages::visible;
    match v {
        Verdict::Pass => String::new(),
        Verdict::Fail(diffs) => {
            let mut out = format!("  FAIL {}\n", vis(name));
            for d in diffs {
                if d.name == "functions" {
                    out.push_str(&format!("    functions: {}\n", d.found));
                } else {
                    out.push_str(&format!(
                        "    {}: expected {}, found {}\n",
                        d.name, d.expected, d.found
                    ));
                }
            }
            out
        }
        Verdict::Fixture(fx, msg) => {
            let path: Vec<crate::diag::PathStep> = fx
                .iter()
                .map(|s| match s {
                    Json::Str(k) => crate::diag::PathStep::Key(k.clone()),
                    n => crate::diag::PathStep::Index(n.as_f64().unwrap_or(0.0) as usize),
                })
                .collect();
            format!(
                "  INVALID {}\n    fixture {}: {}\n",
                vis(name),
                crate::render::path_text(&path),
                vis(msg)
            )
        }
        Verdict::Skip(why) => format!("  SKIP {}: {}\n", vis(name), vis(why)),
        Verdict::Updated => format!("  UPDATED {}\n", vis(name)),
        Verdict::Invalid(why) => format!("  INVALID {}\n    {}\n", vis(name), vis(why)),
    }
}

/// The report of a file's cases (`f09-testing.md` 6).
pub fn report_text(results: &[(String, Verdict)]) -> String {
    results.iter().map(|(n, v)| case_lines(n, v)).collect()
}

/// The cases of an inner file (`f09-testing.md` 3.2), or why it is invalid.
fn inner_cases(rc: &super::case::RunnerCase) -> Result<Vec<Case>, String> {
    use super::case::Inner;
    let doc = match &rc.inner {
        Inner::Data(j) => j.clone(),
        Inner::Text(t) if rc.program => super::program::program_value(t)?,
        Inner::Text(t) => crate::json::parse(t)?,
    };
    if rc.update {
        super::case::load_doc_lax(&doc, &rc.dir)
    } else {
        super::case::load_doc(&doc, &rc.dir)
    }
}

/// A runner case (`f09-testing.md` 3.2, 3.5), run once.
fn run_runner_case(rc: &super::case::RunnerCase) -> Verdict {
    let mut diffs = Vec::new();
    let cases = match inner_cases(rc) {
        Err(why) => {
            if !rc.invalid {
                diffs.push(Difference {
                    name: "invalid",
                    expected: "false".into(),
                    found: json_text(&why),
                });
            }
            return verdict(diffs);
        }
        Ok(c) => c,
    };
    if rc.invalid {
        return Verdict::Fail(vec![Difference {
            name: "invalid",
            expected: "true".into(),
            found: "false".into(),
        }]);
    }
    // 4.7: a program case file is never written.
    let (results, text) = if rc.update && !rc.program {
        let text = match &rc.inner {
            super::case::Inner::Text(t) => t.clone(),
            _ => String::new(),
        };
        let (rs, new) = super::update::update_text(&text, &cases);
        (rs, Some(new.unwrap_or(text)))
    } else {
        let rs: Vec<(String, Verdict)> = cases
            .iter()
            .map(|c| (c.name.clone(), run_case(c)))
            .collect();
        let text = match (&rc.inner, rc.update) {
            (super::case::Inner::Text(t), true) => Some(t.clone()),
            _ => None,
        };
        (rs, text)
    };
    let entry_ok = |e: &super::case::VerdictEntry, (name, v): &(String, Verdict)| {
        e.name == *name
            && e.status == v.label()
            && e.differs.as_ref().is_none_or(|ds| match v {
                Verdict::Fail(got) => {
                    ds.len() == got.len() && ds.iter().zip(got).all(|(a, b)| a == b.name)
                }
                _ => false,
            })
            && e.fixture.as_ref().is_none_or(|fx| match v {
                Verdict::Fixture(got, _) => {
                    crate::json::same_data(&Json::Arr(fx.clone()), &Json::Arr(got.clone()))
                }
                _ => false,
            })
            && e.message.as_ref().is_none_or(|m| match v {
                Verdict::Fixture(_, got) => m == got,
                _ => false,
            })
    };
    let same = rc.verdicts.len() == results.len()
        && rc
            .verdicts
            .iter()
            .zip(&results)
            .all(|(e, r)| entry_ok(e, r));
    if !same {
        diffs.push(Difference {
            name: "verdicts",
            expected: rc
                .verdicts
                .iter()
                .map(|e| format!("{} {}", e.name, e.status))
                .collect::<Vec<_>>()
                .join("; "),
            found: results
                .iter()
                .map(|(n, v)| {
                    let mut s = format!("{n} {}", v.label());
                    if let Verdict::Fail(ds) = v {
                        s.push_str(&format!(
                            " {:?}",
                            ds.iter().map(|d| d.name).collect::<Vec<_>>()
                        ));
                    }
                    if let Verdict::Fixture(fx, m) = v {
                        s.push_str(&format!(" {} {m:?}", form(&Json::Arr(fx.clone()))));
                    }
                    s
                })
                .collect::<Vec<_>>()
                .join("; "),
        });
    }
    if let Some(want) = &rc.report {
        let got = report_text(&results);
        if *want != got {
            diffs.push(Difference {
                name: "report",
                expected: json_text(want),
                found: json_text(&got),
            });
        }
    }
    if let (Some(want), Some(got)) = (&rc.text, &text) {
        if want != got {
            diffs.push(Difference {
                name: "text",
                expected: json_text(want),
                found: json_text(got),
            });
        }
    }
    verdict(diffs)
}

/// What a case gave, for update mode (`f09-testing.md` 4.4): from the checked
/// run, or the run with the static rules alone under `"check": false`.
#[allow(clippy::large_enum_variant)]
pub(super) enum Captured {
    Failure {
        phase: Phase,
        diags: Vec<Diagnostic>,
        usage: Option<(u64, u64)>,
    },
    Result {
        ty: Option<String>,
        sites: Option<Json>,
        description: Option<Json>,
        ran: Option<Ran>,
    },
    Conform(Conformed),
}

/// A result of a run.
pub(super) struct Ran {
    pub(super) value: Json,
    /// The value holds a map of the language with the key `$opaque`.
    pub(super) opaque_key: bool,
    pub(super) truthy: bool,
    pub(super) json: Option<String>,
    pub(super) steps: u64,
    pub(super) units: u64,
}

fn has_opaque_key(v: &Value<'_>) -> bool {
    match v {
        Value::List(_) | Value::HList(_) => {
            (0..v.len().unwrap_or(0)).any(|i| has_opaque_key(&v.elem(i)))
        }
        Value::Map(_) | Value::HMap(_) => v
            .entries()
            .into_iter()
            .any(|(k, x)| k.as_str() == "$opaque" || has_opaque_key(&x)),
        _ => false,
    }
}

/// The outcome of a case, through every stage when `force_run`.
pub(super) fn capture(case: &Case, force_run: bool) -> Captured {
    if case.conform.is_some() {
        return match conform_case(case) {
            Ok(c) => Captured::Conform(c),
            Err(e) => Captured::Failure {
                phase: e.phase,
                diags: e.diagnostics,
                usage: None,
            },
        };
    }
    let fail = |phase: Phase, diags: Vec<Diagnostic>| Captured::Failure {
        phase,
        diags,
        usage: None,
    };
    let mut interner = Interner::default();
    let unmatched = Arc::new(Unmatched::default());
    let env = match build_env(case, &mut interner, &unmatched) {
        Ok(e) => e,
        Err(e) => return fail(e.phase, e.diagnostics),
    };
    let description = Some(env.describe());
    let parsed = match parse_case(case) {
        Ok(p) => p,
        Err(e) => return fail(Phase::Parse, e.diagnostics),
    };
    let (mut ty, mut sites, mut site_list) = (None, None, None);
    if case.check {
        let exp = match case_expected(case, &env) {
            Ok(x) => x,
            Err(d) => return fail(Phase::Declare, vec![d]),
        };
        let r = crate::check::check(&env, &parsed, &exp);
        if !r.diagnostics.is_empty() {
            return fail(Phase::Check, r.diagnostics);
        }
        ty = r.ty.as_ref().map(crate::check::ty::print);
        sites = Some(Json::Arr(
            r.sites
                .iter()
                .map(|s| {
                    Json::Obj(vec![
                        ("class".into(), Json::Str(s.class.to_string())),
                        ("span".into(), span_json(parsed.source.line_span(s.span).0)),
                    ])
                })
                .collect(),
        ));
        site_list = Some(r.sites);
    }
    let mut program = match compile(&env, parsed) {
        Ok(p) => p,
        Err(e) => return fail(e.phase, e.diagnostics),
    };
    program.sites = site_list;
    let needs_run = force_run
        || case.load
        || case.vars.is_some()
        || match &case.expect {
            Expect::Result(r) => r.needs_run(),
            Expect::Failure(f) => f.phase == Phase::Run,
        };
    if !needs_run {
        return Captured::Result {
            ty,
            sites,
            description,
            ran: None,
        };
    }
    let vars: BTreeMap<String, Data> = case
        .vars
        .iter()
        .flatten()
        .map(|(k, v)| (k.clone(), interner.data(v)))
        .collect();
    let out = program.run(&vars);
    let usage = (out.usage.steps, out.usage.allocated);
    if let Ok(v) = &out.result {
        if let Some(d) = load_step(case, &env, v) {
            return Captured::Failure {
                phase: Phase::Run,
                diags: vec![d],
                usage: Some(usage),
            };
        }
    }
    match out.result {
        Err(f) => Captured::Failure {
            phase: Phase::Run,
            diags: vec![program.diagnostic(&f)],
            usage: Some(usage),
        },
        Ok(v) => Captured::Result {
            ty,
            sites,
            description,
            ran: Some(Ran {
                value: value_to_json(&v),
                opaque_key: has_opaque_key(&v),
                truthy: v.truthy(),
                json: crate::eval::to_json(&v).ok(),
                steps: usage.0,
                units: usage.1,
            }),
        },
    }
}
