//! Case files: reading and validation (`03-host.md` 3.10.1 to 3.10.4,
//! 3.10.10 and 3.10.11, with ruling A4 for `precondition`).

use crate::diag::{code, Phase};
use crate::json::{self, Json};
use crate::schema::SchemaOptions;
use crate::types::text::{parse_type, Type};
use std::path::{Path, PathBuf};

/// A host function given as a table of rows (3.10.4).
#[derive(Clone, Debug, PartialEq)]
pub struct Stub {
    /// The signature text. `None` keeps the base environment's signature.
    pub ty: Option<String>,
    pub calls: Vec<Row>,
}

#[derive(Clone, Debug, PartialEq)]
pub enum Answer {
    Result(Json),
    Error(String),
}

#[derive(Clone, Debug, PartialEq)]
pub struct Row {
    pub args: Option<Vec<Json>>,
    pub answer: Answer,
    pub charge: u64,
    pub allocate: u64,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct Limits {
    pub steps: Option<u64>,
    pub allocation: Option<u64>,
    pub depth: Option<u64>,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Site {
    pub class: String,
    pub span: [u32; 4],
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct ExpectResult {
    pub ty: Option<String>,
    pub sites: Option<Vec<Site>>,
    pub value: Option<Json>,
    pub truthy: Option<bool>,
    pub steps: Option<u64>,
    pub allocated: Option<u64>,
    /// F1: the description of the case's environment (`f01-host-types.md`
    /// 7.2).
    pub description: Option<Json>,
}

impl ExpectResult {
    /// Whether the result names something only a run gives (3.10.5 step 5).
    pub fn needs_run(&self) -> bool {
        self.value.is_some()
            || self.truthy.is_some()
            || self.steps.is_some()
            || self.allocated.is_some()
    }
}

#[derive(Clone, Debug, PartialEq)]
pub struct ExpectDiag {
    pub code: String,
    pub class: Option<String>,
    pub span: Option<[u32; 4]>,
    pub path: Option<Vec<Json>>,
    pub message: Option<String>,
    pub data: Option<Json>,
}

#[derive(Clone, Debug, PartialEq)]
pub struct ExpectFailure {
    pub phase: Phase,
    pub diagnostics: Vec<ExpectDiag>,
    pub steps: Option<u64>,
    pub allocated: Option<u64>,
}

#[derive(Clone, Debug, PartialEq)]
pub enum Expect {
    Result(ExpectResult),
    Failure(ExpectFailure),
}

/// One case, with the header laid over it (3.10.2).
#[derive(Clone, Debug, PartialEq)]
pub struct Case {
    pub name: String,
    /// The program text, from `source` or read from `file`.
    pub source: String,
    pub version: Option<String>,
    pub env: Option<String>,
    /// F1: variables declared by schema, with their options (7.1).
    pub schemas: Vec<(String, Json, SchemaOptions)>,
    pub opaque: Vec<String>,
    /// `Type.method` and its stub (3.10.4 item 2).
    pub methods: Vec<(String, Stub)>,
    pub functions: Vec<(String, Stub)>,
    pub reject: Option<Vec<String>>,
    pub returns: Option<String>,
    /// `None` when the case gives no `vars`.
    pub vars: Option<Vec<(String, Json)>>,
    pub limits: Limits,
    pub skip: Option<String>,
    pub precondition: Option<bool>,
    /// `false`: the program is one the type rules reject on purpose; it is
    /// run with the static rules alone (rulings round 11 AP1).
    pub check: bool,
    pub expect: Expect,
}

/// A case file that was read and found valid.
#[derive(Clone, Debug)]
pub struct CaseFile {
    pub path: PathBuf,
    pub cases: Vec<Case>,
}

const HEADER_FIELDS: [&str; 15] = [
    "schemas",
    "check",
    "cases",
    "source",
    "file",
    "version",
    "env",
    "opaque",
    "methods",
    "functions",
    "reject",
    "returns",
    "vars",
    "limits",
    "precondition",
];

const CASE_FIELDS: [&str; 17] = [
    "schemas",
    "check",
    "name",
    "source",
    "file",
    "version",
    "env",
    "opaque",
    "methods",
    "functions",
    "reject",
    "returns",
    "vars",
    "limits",
    "skip",
    "expect",
    "precondition",
];

const SITE_CLASSES: [&str; 5] = ["dynamic", "index", "divide", "value", "host"];

const DIAG_CLASSES: [&str; 9] = [
    "constant-condition",
    "dynamic",
    "index",
    "divide",
    "value",
    "host",
    "environment",
    "range",
    "limit",
];

type V<T> = Result<T, String>;

/// Read a case file. An `Err` is the reason the file is invalid: every case
/// of it then has the verdict `invalid` (3.10.10).
pub fn load(path: &Path) -> V<CaseFile> {
    let text = std::fs::read_to_string(path).map_err(|e| format!("cannot read the file: {e}"))?;
    let doc = json::parse(&text)?;
    let dir = path.parent().unwrap_or(Path::new("."));
    let cases = load_doc(&doc, dir)?;
    Ok(CaseFile {
        path: path.to_path_buf(),
        cases,
    })
}

/// Read the cases of a parsed document: an object with `cases`, or an array
/// of cases with no header (3.10.1). `dir` resolves `file`.
pub fn load_doc(doc: &Json, dir: &Path) -> V<Vec<Case>> {
    load_doc_with(doc, dir, false)
}

/// As `load_doc`, but a case may have no `expect` (round 10 AJ): such a case
/// expects an empty result.
pub fn load_doc_lax(doc: &Json, dir: &Path) -> V<Vec<Case>> {
    load_doc_with(doc, dir, true)
}

fn load_doc_with(doc: &Json, dir: &Path, lax: bool) -> V<Vec<Case>> {
    let (header, cases): (&[(String, Json)], &[Json]) = match doc {
        Json::Arr(a) => (&[], a),
        Json::Obj(m) => {
            let cases = doc
                .get("cases")
                .ok_or("the file object has no field cases")?
                .as_arr()
                .ok_or("cases is not an array")?;
            (m, cases)
        }
        _ => return Err("the file is neither an object with cases nor an array".into()),
    };
    for (k, v) in header {
        if !HEADER_FIELDS.contains(&k.as_str()) {
            return Err(format!(
                "the header has the field {k:?}, which 3.10.3 does not list"
            ));
        }
        if k != "cases" {
            check_field(k, v).map_err(|e| format!("header: {e}"))?;
        }
    }
    let mut out = Vec::new();
    for (i, c) in cases.iter().enumerate() {
        let obj = c
            .as_obj()
            .ok_or_else(|| format!("case {i} is not an object"))?;
        for (k, v) in obj {
            if !CASE_FIELDS.contains(&k.as_str()) {
                return Err(format!(
                    "case {i} has the field {k:?}, which 3.10.3 does not list"
                ));
            }
            check_field(k, v).map_err(|e| format!("case {i}: {e}"))?;
        }
        let case = merge(header, obj, dir, lax).map_err(|e| format!("case {i}: {e}"))?;
        if out.iter().any(|o: &Case| o.name == case.name) {
            return Err(format!("the name {:?} is used twice", case.name));
        }
        out.push(case);
    }
    Ok(out)
}

/// Lay a case over the header (3.10.2) and read its fields.
fn merge(header: &[(String, Json)], case: &[(String, Json)], dir: &Path, lax: bool) -> V<Case> {
    let own = |k: &str| case.iter().find(|(n, _)| n == k).map(|(_, v)| v);
    let hdr = |k: &str| header.iter().find(|(n, _)| n == k).map(|(_, v)| v);
    let get = |k: &str| own(k).or_else(|| hdr(k));

    let name = own("name")
        .ok_or("the case has no name")?
        .as_str()
        .unwrap_or_default()
        .to_string();
    let expect = match own("expect") {
        Some(e) => read_expect(e)?,
        None if lax => Expect::Result(ExpectResult::default()),
        None => return Err("the case has no expect".into()),
    };

    // `source` and `file` count as one field.
    let (source, file) = if own("source").is_some() || own("file").is_some() {
        (own("source"), own("file"))
    } else {
        (hdr("source"), hdr("file"))
    };
    let source = match (source, file) {
        (Some(s), None) => s.as_str().unwrap_or_default().to_string(),
        (None, Some(f)) => {
            let rel = f.as_str().unwrap_or_default();
            let mut p = dir.to_path_buf();
            for part in rel.split('/') {
                p.push(part);
            }
            std::fs::read_to_string(&p).map_err(|e| format!("cannot read the file {rel:?}: {e}"))?
        }
        (Some(_), Some(_)) => return Err("the case has both source and file".into()),
        (None, None) => return Err("the case has neither source nor file".into()),
    };

    let strings = |v: Option<&Json>| -> Vec<String> {
        v.and_then(|v| v.as_arr())
            .map(|a| {
                a.iter()
                    .filter_map(|s| s.as_str().map(String::from))
                    .collect()
            })
            .unwrap_or_default()
    };
    let env = get("env").and_then(|v| v.as_str()).map(String::from);
    let vars = get("vars").and_then(|v| v.as_obj()).map(|m| m.to_vec());
    let mut schemas = Vec::new();
    if let Some(m) = get("schemas").and_then(|v| v.as_obj()) {
        for (name, decl) in m {
            let schema = decl.get("schema").cloned().unwrap_or(Json::Null);
            let names: Vec<&str> = decl
                .get("options")
                .and_then(|o| o.as_arr())
                .map(|a| a.iter().filter_map(|x| x.as_str()).collect())
                .unwrap_or_default();
            let opts = SchemaOptions::from_names(&names)?;
            schemas.push((name.clone(), schema, opts));
        }
    }
    if let Some(vars) = &vars {
        let names: Vec<&str> = schemas.iter().map(|(n, _, _)| n.as_str()).collect();
        check_vars(env.as_deref(), &names, vars)?;
    }
    let mut functions = Vec::new();
    if let Some(m) = get("functions").and_then(|v| v.as_obj()) {
        for (fname, stub) in m {
            let stub = read_stub(stub);
            // 3.10.10 item 11. The suite's base environment declares no host
            // function, so every stub needs its type.
            if stub.ty.is_none() {
                return Err(format!(
                    "the stub {fname:?} has no type, and the base environment does not declare it"
                ));
            }
            functions.push((fname.clone(), stub));
        }
    }
    let mut methods = Vec::new();
    if let Some(m) = get("methods").and_then(|v| v.as_obj()) {
        for (key, stub) in m {
            let stub = read_stub(stub);
            if stub.ty.is_none() {
                return Err(format!(
                    "the method stub {key:?} has no type, and the base environment does not declare it"
                ));
            }
            methods.push((key.clone(), stub));
        }
    }
    let limits = get("limits")
        .map(|l| Limits {
            steps: l
                .get("steps")
                .and_then(|v| v.as_integer())
                .map(|v| v as u64),
            allocation: l
                .get("allocation")
                .and_then(|v| v.as_integer())
                .map(|v| v as u64),
            depth: l
                .get("depth")
                .and_then(|v| v.as_integer())
                .map(|v| v as u64),
        })
        .unwrap_or_default();
    // 3.10.10 item 14: every `$opaque` object names a declared type.
    let opaque = strings(get("opaque"));
    let check_opaque = |v: &Json, what: &str| -> V<()> {
        check_opaque_values(v, &opaque).map_err(|e| format!("{what}: {e}"))
    };
    if let Some(vars) = &vars {
        for (k, v) in vars {
            check_opaque(v, &format!("vars.{k}"))?;
        }
    }
    for (n, stub) in functions.iter().chain(methods.iter()) {
        for r in &stub.calls {
            for a in r.args.iter().flatten() {
                check_opaque(a, n)?;
            }
            if let Answer::Result(v) = &r.answer {
                check_opaque(v, n)?;
            }
        }
    }
    if let Expect::Result(r) = &expect {
        if let Some(v) = &r.value {
            check_opaque(v, "expect.value")?;
        }
    }
    let check = !matches!(get("check"), Some(Json::Bool(false)));
    // 3.10.10 item 17: an unchecked case states nothing that needs the check.
    if !check {
        let bad = match &expect {
            Expect::Result(r) => r.ty.is_some() || r.sites.is_some(),
            Expect::Failure(f) => {
                f.phase == Phase::Check
                    && f.diagnostics
                        .iter()
                        .any(|d| !code::STATIC.contains(&d.code.as_str()))
            }
        };
        if bad {
            return Err("a case with \"check\": false states what needs the type checker".into());
        }
    }
    Ok(Case {
        name,
        source,
        version: get("version").and_then(|v| v.as_str()).map(String::from),
        env,
        schemas,
        opaque,
        methods,
        functions,
        reject: get("reject").map(|v| strings(Some(v))),
        returns: get("returns").and_then(|v| v.as_str()).map(String::from),
        vars,
        limits,
        skip: own("skip").and_then(|v| v.as_str()).map(String::from),
        check,
        precondition: match get("precondition") {
            Some(Json::Bool(b)) => Some(*b),
            _ => None,
        },
        expect,
    })
}

/// 3.10.10 item 9, as F1 7.1 item 5 reads it: every key of `vars` is a
/// field of `env` or a key of `schemas`. When `env` does not parse as a
/// record, the case fails at its declare step instead.
fn check_vars(env: Option<&str>, schemas: &[&str], vars: &[(String, Json)]) -> V<()> {
    let fields: Option<Vec<String>> = match env {
        None => Some(Vec::new()),
        Some(text) => match parse_type(text) {
            Ok(Type::Record(r)) => Some(r.fields.into_iter().map(|f| f.key).collect()),
            _ => None,
        },
    };
    if let Some(fields) = fields {
        for (k, _) in vars {
            if !fields.contains(k) && !schemas.contains(&k.as_str()) {
                return Err(format!(
                    "the key {k:?} of vars is neither a field of env nor a key of schemas"
                ));
            }
        }
    }
    Ok(())
}

fn read_stub(v: &Json) -> Stub {
    match v {
        Json::Str(s) => Stub {
            ty: Some(s.clone()),
            calls: Vec::new(),
        },
        _ => Stub {
            ty: v.get("type").and_then(|t| t.as_str()).map(String::from),
            calls: v
                .get("calls")
                .and_then(|c| c.as_arr())
                .map(|rows| rows.iter().map(read_row).collect())
                .unwrap_or_default(),
        },
    }
}

fn read_row(r: &Json) -> Row {
    let count = |k: &str| r.get(k).and_then(|v| v.as_integer()).unwrap_or(0.0) as u64;
    Row {
        args: r.get("args").and_then(|a| a.as_arr()).map(|a| a.to_vec()),
        answer: match r.get("result") {
            Some(v) => Answer::Result(v.clone()),
            None => Answer::Error(
                r.get("error")
                    .and_then(|e| e.as_str())
                    .unwrap_or_default()
                    .to_string(),
            ),
        },
        charge: count("charge"),
        allocate: count("allocate"),
    }
}

fn read_expect(v: &Json) -> V<Expect> {
    let obj = v.as_obj().ok_or("expect is not an object")?;
    let has = |k: &str| obj.iter().any(|(n, _)| n == k);
    let count = |k: &str| v.get(k).and_then(|x| x.as_integer()).map(|x| x as u64);
    if has("phase") {
        for (k, _) in obj {
            if !["phase", "diagnostics", "steps", "allocated"].contains(&k.as_str()) {
                return Err(format!("expect is a failure and has the field {k:?}"));
            }
        }
        let phase = match v.get("phase").and_then(|p| p.as_str()) {
            Some("declare") => Phase::Declare,
            Some("parse") => Phase::Parse,
            Some("check") => Phase::Check,
            Some("run") => Phase::Run,
            _ => return Err("expect.phase is not a phase".into()),
        };
        let diags = v
            .get("diagnostics")
            .ok_or("expect is a failure with no diagnostics")?
            .as_arr()
            .ok_or("expect.diagnostics is not an array")?;
        if diags.is_empty() {
            return Err("expect.diagnostics is empty".into());
        }
        let mut diagnostics = Vec::new();
        for d in diags {
            diagnostics.push(read_diag(d)?);
        }
        // Rulings AC19 and AG5.
        if matches!(phase, Phase::Parse | Phase::Declare) && diagnostics.len() != 1 {
            return Err(format!(
                "a failure of phase {phase} states exactly one diagnostic"
            ));
        }
        for k in ["steps", "allocated"] {
            if has(k) {
                check_count(v.get(k).unwrap_or(&Json::Null))
                    .map_err(|e| format!("expect.{k}: {e}"))?;
                if phase != Phase::Run {
                    return Err(format!("expect.{k} in a failure of phase {phase}"));
                }
                // Ruling A5.
                if diagnostics
                    .iter()
                    .any(|d| code::LIMIT.contains(&d.code.as_str()))
                {
                    return Err(format!("expect.{k} with a limit code"));
                }
            }
        }
        return Ok(Expect::Failure(ExpectFailure {
            phase,
            diagnostics,
            steps: count("steps"),
            allocated: count("allocated"),
        }));
    }
    for (k, x) in obj {
        match k.as_str() {
            "type" => {
                x.as_str().ok_or("expect.type is not a string")?;
            }
            "sites" => {
                let a = x.as_arr().ok_or("expect.sites is not an array")?;
                for s in a {
                    let so = s.as_obj().ok_or("a site is not an object")?;
                    for (sk, sv) in so {
                        match sk.as_str() {
                            "class" => {
                                if !sv.as_str().is_some_and(|c| SITE_CLASSES.contains(&c)) {
                                    return Err(
                                        "a site has a class that is not a site class".into()
                                    );
                                }
                            }
                            "span" => check_span(sv)?,
                            _ => return Err(format!("a site has the field {sk:?}")),
                        }
                    }
                    if s.get("class").is_none() || s.get("span").is_none() {
                        return Err("a site needs class and span".into());
                    }
                }
            }
            "value" => {}
            "truthy" => {
                if !matches!(x, Json::Bool(_)) {
                    return Err("expect.truthy is not a boolean".into());
                }
            }
            "steps" | "allocated" => check_count(x).map_err(|e| format!("expect.{k}: {e}"))?,
            "description" => check_description(x)?,
            _ => return Err(format!("expect is a result and has the field {k:?}")),
        }
    }
    Ok(Expect::Result(ExpectResult {
        ty: v.get("type").and_then(|t| t.as_str()).map(String::from),
        sites: v.get("sites").and_then(|s| s.as_arr()).map(|a| {
            a.iter()
                .map(|s| Site {
                    class: s
                        .get("class")
                        .and_then(|c| c.as_str())
                        .unwrap_or_default()
                        .to_string(),
                    span: span_of(s.get("span").unwrap_or(&Json::Null)),
                })
                .collect()
        }),
        value: v.get("value").cloned(),
        truthy: match v.get("truthy") {
            Some(Json::Bool(b)) => Some(*b),
            _ => None,
        },
        steps: count("steps"),
        allocated: count("allocated"),
        description: v.get("description").cloned(),
    }))
}

/// F1 7.5: `expect.description` is an object with exactly the seven keys of
/// a description.
fn check_description(x: &Json) -> V<()> {
    const KEYS: [&str; 7] = [
        "version",
        "variables",
        "functions",
        "opaque",
        "removed",
        "reject",
        "limits",
    ];
    let m = x.as_obj().ok_or("expect.description is not an object")?;
    for (k, _) in m {
        if !KEYS.contains(&k.as_str()) {
            return Err(format!("expect.description has the field {k:?}"));
        }
    }
    for k in KEYS {
        if x.get(k).is_none() {
            return Err(format!("expect.description has no field {k:?}"));
        }
    }
    Ok(())
}

/// F1 7.1: the form of `schemas`.
fn check_schemas(v: &Json) -> V<()> {
    let m = v.as_obj().ok_or("schemas is not an object")?;
    for (name, decl) in m {
        let d = decl
            .as_obj()
            .ok_or_else(|| format!("schemas.{name} is not an object"))?;
        for (k, x) in d {
            match k.as_str() {
                "schema" => {}
                "options" => {
                    let a = x
                        .as_arr()
                        .ok_or_else(|| format!("schemas.{name}.options is not an array"))?;
                    let names: Vec<&str> = a.iter().filter_map(|o| o.as_str()).collect();
                    if names.len() != a.len() {
                        return Err(format!(
                            "schemas.{name}.options holds a value that is not a string"
                        ));
                    }
                    SchemaOptions::from_names(&names)
                        .map_err(|e| format!("schemas.{name}.options: {e}"))?;
                }
                _ => return Err(format!("schemas.{name} has the field {k:?}")),
            }
        }
        if decl.get("schema").is_none() {
            return Err(format!("schemas.{name} has no schema"));
        }
    }
    Ok(())
}

fn read_diag(d: &Json) -> V<ExpectDiag> {
    let obj = d.as_obj().ok_or("a diagnostic is not an object")?;
    for (k, v) in obj {
        match k.as_str() {
            "code" => {
                let c = v.as_str().ok_or("a diagnostic's code is not a string")?;
                if !is_code(c) {
                    return Err(format!("{c:?} is not written as a code"));
                }
            }
            "class" => {
                if !v.as_str().is_some_and(|c| DIAG_CLASSES.contains(&c)) {
                    return Err("a diagnostic has a class that is not a class".into());
                }
            }
            "span" => check_span(v)?,
            "path" => {
                let a = v.as_arr().ok_or("a path is not an array")?;
                if a.len() > 64 {
                    return Err("a path has more than 64 steps".into());
                }
                for s in a {
                    let ok = matches!(s, Json::Str(_)) || s.as_integer().is_some_and(|n| n >= 0.0);
                    if !ok {
                        return Err("a step of a path is neither a string nor an index".into());
                    }
                }
            }
            "message" => {
                v.as_str().ok_or("a diagnostic's message is not a string")?;
            }
            "data" => {}
            _ => return Err(format!("a diagnostic has the field {k:?}")),
        }
    }
    Ok(ExpectDiag {
        code: d
            .get("code")
            .ok_or("a diagnostic has no code")?
            .as_str()
            .unwrap_or_default()
            .to_string(),
        class: d.get("class").and_then(|c| c.as_str()).map(String::from),
        span: d.get("span").map(span_of),
        path: d.get("path").and_then(|p| p.as_arr()).map(|a| a.to_vec()),
        message: d.get("message").and_then(|m| m.as_str()).map(String::from),
        data: d.get("data").cloned(),
    })
}

/// `^[a-z]+(-[a-z0-9]+)*$`
fn is_code(c: &str) -> bool {
    let mut parts = c.split('-');
    let first = parts.next().unwrap_or_default();
    !first.is_empty()
        && first.bytes().all(|b| b.is_ascii_lowercase())
        && parts.all(|p| {
            !p.is_empty()
                && p.bytes()
                    .all(|b| b.is_ascii_lowercase() || b.is_ascii_digit())
        })
}

fn span_of(v: &Json) -> [u32; 4] {
    let mut out = [0; 4];
    if let Some(a) = v.as_arr() {
        for (i, n) in a.iter().take(4).enumerate() {
            out[i] = n.as_f64().unwrap_or(0.0) as u32;
        }
    }
    out
}

fn check_span(v: &Json) -> V<()> {
    let a = v.as_arr().ok_or("a span is not an array")?;
    if a.len() != 4 || !a.iter().all(|n| n.as_integer().is_some_and(|n| n >= 1.0)) {
        return Err("a span is not four whole numbers of 1 or more".into());
    }
    Ok(())
}

fn check_count(v: &Json) -> V<()> {
    match v.as_integer() {
        Some(n) if n >= 0.0 => Ok(()),
        _ => Err("not a whole number of 0 or more".into()),
    }
}

fn is_version(s: &str) -> bool {
    let part = |p: &str| {
        !p.is_empty() && p.bytes().all(|b| b.is_ascii_digit()) && (p == "0" || !p.starts_with('0'))
    };
    match s.split_once('.') {
        Some((a, b)) => part(a) && part(b),
        None => false,
    }
}

/// `^[A-Z][A-Za-z0-9_]+\.[A-Za-z_][A-Za-z0-9_]*$`
fn is_method_key(s: &str) -> bool {
    let Some((t, m)) = s.split_once('.') else {
        return false;
    };
    let tb = t.as_bytes();
    let mb = m.as_bytes();
    tb.len() >= 2
        && tb[0].is_ascii_uppercase()
        && tb[1..]
            .iter()
            .all(|b| b.is_ascii_alphanumeric() || *b == b'_')
        && !mb.is_empty()
        && (mb[0].is_ascii_alphabetic() || mb[0] == b'_')
        && mb.iter().all(|b| b.is_ascii_alphanumeric() || *b == b'_')
}

/// The JSON type of one field of the header or of a case (3.10.11).
fn check_field(k: &str, v: &Json) -> V<()> {
    let string = |what: &str| -> V<()> {
        v.as_str()
            .map(|_| ())
            .ok_or_else(|| format!("{what} is not a string"))
    };
    let strings = |what: &str| -> V<()> {
        let a = v
            .as_arr()
            .ok_or_else(|| format!("{what} is not an array"))?;
        if a.iter().all(|s| s.as_str().is_some()) {
            Ok(())
        } else {
            Err(format!("{what} holds a value that is not a string"))
        }
    };
    match k {
        "name" | "source" | "env" | "returns" | "skip" => string(k),
        "file" => match v.as_str() {
            Some(s) if !s.is_empty() => Ok(()),
            _ => Err("file is not a non-empty string".into()),
        },
        "version" => match v.as_str() {
            Some(s) if is_version(s) => Ok(()),
            _ => Err("version is not major.minor".into()),
        },
        "opaque" | "reject" => strings(k),
        "methods" => {
            let m = v.as_obj().ok_or("methods is not an object")?;
            for (mk, mv) in m {
                if !is_method_key(mk) {
                    return Err(format!("the method key {mk:?} is not Type.method"));
                }
                check_stub(mv).map_err(|e| format!("the method {mk:?}: {e}"))?;
            }
            Ok(())
        }
        "functions" => {
            let m = v.as_obj().ok_or("functions is not an object")?;
            for (fk, stub) in m {
                check_stub(stub).map_err(|e| format!("the stub {fk:?}: {e}"))?;
            }
            Ok(())
        }
        "vars" => v
            .as_obj()
            .map(|_| ())
            .ok_or_else(|| "vars is not an object".into()),
        "limits" => {
            let m = v.as_obj().ok_or("limits is not an object")?;
            for (lk, lv) in m {
                let n = lv
                    .as_integer()
                    .ok_or_else(|| format!("limits.{lk} is not a whole number"))?;
                match lk.as_str() {
                    "steps" | "allocation" if n >= 0.0 => {}
                    "depth" if (1.0..=1000.0).contains(&n) => {}
                    "steps" | "allocation" | "depth" => {
                        return Err(format!("limits.{lk} is out of range"))
                    }
                    _ => return Err(format!("limits has the field {lk:?}")),
                }
            }
            Ok(())
        }
        "precondition" | "check" => match v {
            Json::Bool(_) => Ok(()),
            _ => Err("precondition is not a boolean".into()),
        },
        "expect" => Ok(()),
        "schemas" => check_schemas(v),
        _ => Err(format!("unknown field {k:?}")),
    }
}

fn check_stub(v: &Json) -> V<()> {
    match v {
        Json::Str(_) => Ok(()),
        Json::Obj(m) => {
            for (k, x) in m {
                match k.as_str() {
                    "type" => {
                        x.as_str().ok_or("type is not a string")?;
                    }
                    "calls" => {
                        let rows = x.as_arr().ok_or("calls is not an array")?;
                        for r in rows {
                            check_row(r)?;
                        }
                    }
                    _ => return Err(format!("a stub has the field {k:?}")),
                }
            }
            Ok(())
        }
        _ => Err("a stub is neither a string nor an object".into()),
    }
}

fn check_row(r: &Json) -> V<()> {
    let m = r.as_obj().ok_or("a row is not an object")?;
    for (k, x) in m {
        match k.as_str() {
            "args" => {
                x.as_arr().ok_or("args is not an array")?;
            }
            "result" => {}
            "error" => {
                x.as_str().ok_or("error is not a string")?;
            }
            "charge" | "allocate" => check_count(x).map_err(|e| format!("{k}: {e}"))?,
            _ => return Err(format!("a row has the field {k:?}")),
        }
    }
    match (r.get("result").is_some(), r.get("error").is_some()) {
        (true, false) | (false, true) => Ok(()),
        _ => Err("a row needs exactly one of result and error".into()),
    }
}

/// 3.10.3 items 7 and 9: a JSON object with the key `$opaque` is exactly
/// `{"$opaque": {"type": T, "id": n}}`, with `T` a declared opaque type.
fn check_opaque_values(v: &Json, declared: &[String]) -> V<()> {
    match v {
        Json::Arr(a) => a.iter().try_for_each(|x| check_opaque_values(x, declared)),
        Json::Obj(m) => {
            if m.iter().any(|(k, _)| k == "$opaque") {
                let ok = m.len() == 1
                    && match &m[0].1 {
                        Json::Obj(inner) => {
                            inner.len() == 2
                                && inner.iter().all(|(k, x)| match k.as_str() {
                                    "type" => {
                                        x.as_str().is_some_and(|t| declared.iter().any(|d| d == t))
                                    }
                                    "id" => x.as_integer().is_some_and(|n| n >= 0.0),
                                    _ => false,
                                })
                        }
                        _ => false,
                    };
                if ok {
                    Ok(())
                } else {
                    Err("an $opaque object that is not of the form of 3.10.3 item 7".into())
                }
            } else {
                m.iter()
                    .try_for_each(|(_, x)| check_opaque_values(x, declared))
            }
        }
        _ => Ok(()),
    }
}

/// The `$opaque` object of a JSON value, if it is one: its type and id.
pub fn opaque_of(v: &Json) -> Option<(&str, u64)> {
    let inner = v.as_obj().filter(|m| m.len() == 1 && m[0].0 == "$opaque")?;
    let t = inner[0].1.get("type")?.as_str()?;
    let id = inner[0].1.get("id")?.as_integer()?;
    Some((t, id as u64))
}
