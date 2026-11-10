//! Running cases and comparing outcomes (`03-host.md` 3.10.5 to 3.10.8).
//!
//! Stage S1 has a parser and no evaluator. It judges a case whose outcome
//! is decided by the declare step's grammar or by the parse, and gives every
//! other case `skip` with the reason.

use super::case::{load, Case, Expect, ExpectDiag, ExpectFailure};
use crate::diag::{code, Diagnostic, Phase};
use crate::syntax::parse;
use crate::syntax::token::is_name;
use crate::types::text::{parse_sig, parse_type, Type};
use std::fmt;
use std::path::{Path, PathBuf};

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

/// What a stage gave: a result (with no value in S1) or a failure.
enum Outcome {
    Result,
    Failure(Phase, Vec<Diagnostic>),
}

/// Run one case (3.10.5).
pub fn run_case(case: &Case) -> Verdict {
    if let Some(why) = &case.skip {
        return Verdict::Skip(why.clone());
    }
    // 3.10.8 item 5 and ruling A3: what needs the type checker.
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
        Expect::Failure(f) if f.phase == Phase::Declare => {
            return Verdict::Skip(
                "S1: a declare failure needs the library's names (name-clash); judged from S2"
                    .into(),
            )
        }
        _ => {}
    }
    let outcome = match declare(case) {
        Err(diags) => Outcome::Failure(Phase::Declare, diags),
        Ok(()) => match parse(&case.source) {
            Err(e) => Outcome::Failure(Phase::Parse, e.diagnostics),
            Ok(_) => Outcome::Result,
        },
    };
    match (&case.expect, outcome) {
        (Expect::Failure(f), Outcome::Failure(phase, diags)) => {
            let diffs = compare_failure(f, phase, &diags);
            if diffs.is_empty() {
                Verdict::Pass
            } else {
                Verdict::Fail(diffs)
            }
        }
        (Expect::Result(_), Outcome::Failure(phase, diags)) => Verdict::Fail(vec![Difference {
            name: "outcome",
            expected: "a result".into(),
            found: format!("a failure of phase {phase}: {}", diags[0]),
        }]),
        (Expect::Failure(f), Outcome::Result)
            if f.phase == Phase::Parse || f.phase == Phase::Declare =>
        {
            Verdict::Fail(vec![Difference {
                name: "outcome",
                expected: format!("a failure of phase {}", f.phase),
                found: "the program parses".into(),
            }])
        }
        (Expect::Failure(f), Outcome::Result) => Verdict::Skip(format!(
            "S1: a failure of phase {} needs the static rules or the evaluator (S2)",
            f.phase
        )),
        (Expect::Result(r), Outcome::Result) => {
            if case.vars.is_some() || r.needs_run() {
                Verdict::Skip("S1: needs the evaluator (S2)".into())
            } else {
                // Nothing to compare once the program parses. The static rules
                // come with S2, which makes this verdict exact.
                Verdict::Pass
            }
        }
    }
}

/// The six check classes (`05-types.md` 17).
const CHECK_CLASSES: [&str; 6] = [
    "constant-condition",
    "dynamic",
    "index",
    "divide",
    "value",
    "host",
];

const RESERVED_NAMESPACES: [&str; 4] = ["json", "time", "math", "base64"];

/// The part of the declare step that needs only type text (ruling A2,
/// `03-host.md` 3.3.4). The names of the library (`name-clash`) come in S2.
fn declare(case: &Case) -> Result<(), Vec<Diagnostic>> {
    let mut out = Vec::new();
    let mut bad = |c: &'static str, msg: String| out.push(Diagnostic::bare(Phase::Declare, c, msg));
    if let Some(v) = &case.version {
        if v != "3.0" {
            bad(
                code::UNSUPPORTED_VERSION,
                format!("version {v} is not offered"),
            );
        }
    }
    let mut var_names = Vec::new();
    if let Some(env) = &case.env {
        match parse_type(env) {
            Ok(Type::Record(r)) => {
                for f in r.fields {
                    if !is_name(&f.key) {
                        bad(
                            code::BAD_NAME,
                            format!("{:?} cannot name a variable", f.key),
                        );
                    }
                    var_names.push(f.key);
                }
            }
            Ok(_) => bad(code::BAD_TYPE, "env is not a record type".into()),
            Err(e) => bad(code::BAD_TYPE, format!("env: {e}")),
        }
    }
    for (i, name) in case.opaque.iter().enumerate() {
        let b = name.as_bytes();
        let ok = b.len() >= 2
            && b[0].is_ascii_uppercase()
            && b[1..]
                .iter()
                .all(|c| c.is_ascii_alphanumeric() || *c == b'_');
        if !ok {
            bad(
                code::BAD_NAME,
                format!("{name:?} cannot name an opaque type"),
            );
        }
        if case.opaque[..i].contains(name) {
            bad(
                code::NAME_CLASH,
                format!("the opaque type {name} is declared twice"),
            );
        }
    }
    for (key, text) in &case.methods {
        match parse_sig(text) {
            Ok(s) if s.has_fn_param() => bad(
                code::BAD_SIGNATURE,
                format!("the method {key} takes a function"),
            ),
            Ok(_) => {}
            Err(e) => bad(code::BAD_SIGNATURE, format!("the method {key}: {e}")),
        }
    }
    let mut namespaces = Vec::new();
    for (name, stub) in &case.functions {
        let (ns, base) = match name.split_once('.') {
            Some((ns, base)) => (Some(ns), base),
            None => (None, name.as_str()),
        };
        for part in ns.iter().chain(std::iter::once(&base)) {
            if !is_name(part) {
                bad(code::BAD_NAME, format!("{name:?} cannot name a function"));
            }
        }
        if let Some(ns) = ns {
            if RESERVED_NAMESPACES.contains(&ns) {
                bad(
                    code::RESERVED_NAMESPACE,
                    format!("the namespace {ns} is reserved"),
                );
            }
            namespaces.push(ns.to_string());
        }
        if var_names.iter().any(|v| v == name) {
            bad(
                code::NAME_CLASH,
                format!("{name} is a variable and a function"),
            );
        }
        if let Some(text) = &stub.ty {
            match parse_sig(text) {
                Ok(s) if s.has_fn_param() => bad(
                    code::BAD_SIGNATURE,
                    format!("the function {name} takes a function"),
                ),
                Ok(_) => {}
                Err(e) => bad(code::BAD_SIGNATURE, format!("the function {name}: {e}")),
            }
        }
    }
    for v in &var_names {
        if namespaces.contains(v) {
            bad(
                code::NAME_CLASH,
                format!("the variable {v} is named like a namespace"),
            );
        }
    }
    if let Some(classes) = &case.reject {
        for c in classes {
            if !CHECK_CLASSES.contains(&c.as_str()) {
                bad(code::BAD_CLASS, format!("{c:?} is not a check class"));
            }
        }
    }
    if let Some(r) = &case.returns {
        if r != "condition" {
            if let Err(e) = parse_type(r) {
                bad(code::BAD_TYPE, format!("returns: {e}"));
            }
        }
    }
    if out.is_empty() {
        Ok(())
    } else {
        Err(out)
    }
}

fn show_span(s: Option<[u32; 4]>) -> String {
    match s {
        Some([a, b, c, d]) => format!("[{a}, {b}, {c}, {d}]"),
        None => "no span".into(),
    }
}

fn show_diag(d: &Diagnostic) -> String {
    format!("{} at {}", d.code, show_span(d.span.map(|s| s.0)))
}

fn show_expected(d: &ExpectDiag) -> String {
    match d.span {
        Some(s) => format!("{} at {}", d.code, show_span(Some(s))),
        None => d.code.clone(),
    }
}

/// Whether one expected diagnostic matches one actual (3.10.6, 3.10.7).
/// `class` is not compared: there is no checker (3.10.7 item 8).
fn diag_matches(e: &ExpectDiag, a: &Diagnostic) -> bool {
    if e.code != a.code {
        return false;
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
    // A parse or declare diagnostic has no path and no data.
    e.path.is_none() && e.data.is_none()
}

fn compare_failure(f: &ExpectFailure, phase: Phase, diags: &[Diagnostic]) -> Vec<Difference> {
    if f.phase != phase {
        return vec![Difference {
            name: "outcome",
            expected: format!("a failure of phase {}", f.phase),
            found: format!("a failure of phase {phase}: {}", show_diag(&diags[0])),
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
            found: diags.iter().map(show_diag).collect::<Vec<_>>().join("; "),
        }]
    }
}
