//! Case files: reading and validation (`03-host.md` 3.10.1 to 3.10.4,
//! 3.10.10 and 3.10.11).

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
    /// F2: the expected analysis (`f02-templates-analysis.md` 4.2). A case
    /// that states it states nothing else.
    pub analysis: Option<Json>,
    /// F4: a conform result (`f04-conversion.md` 9.2).
    pub conform: Option<ConformExpect>,
    /// F6: the JSON text of the run's value (`f06-groups.md` 11.2).
    pub json: Option<String>,
}

/// A conform result (F4 9.2).
#[derive(Clone, Debug, PartialEq)]
pub struct ConformExpect {
    pub conforms: bool,
    pub path: Option<Vec<Json>>,
    pub message: Option<String>,
    pub steps: Option<u64>,
    /// F8: the failures of `ConformAll`, each a path and maybe a message
    /// (`f08-errors.md` 7.5).
    pub failures: Option<Vec<(Vec<Json>, Option<String>)>>,
}

/// F8: the three renderings a case expects, each when given
/// (`f08-errors.md` 7.3, 7.4).
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Renderings {
    pub line: Option<String>,
    pub detail: Option<String>,
    pub json: Option<String>,
}

/// F8: a case of the third kind (`f08-errors.md` 7.4): diagnostics, a
/// source text, and the renderings expected.
#[derive(Clone, Debug, PartialEq)]
pub struct RenderCase {
    pub diagnostics: Vec<Json>,
    pub source: Option<String>,
    pub expect: Renderings,
}

/// A named type of the case field `types` (F4 9.1).
#[derive(Clone, Debug, PartialEq)]
pub enum TypeDecl {
    Text(String, String),
    Schema(String, Json, SchemaOptions),
}

/// The static codes of a case (`05-types.md` 22.1): in `suite/conversion/`
/// also `untagged-union` and `check-limit` (F4 6.1 item 4).
pub fn is_static(code: &str, conversion: bool) -> bool {
    code::STATIC.contains(&code) || (conversion && matches!(code, "untagged-union" | "check-limit"))
}

impl ExpectResult {
    /// Whether the result names something only a run gives (3.10.5 step 5).
    pub fn needs_run(&self) -> bool {
        self.json.is_some()
            || self.value.is_some()
            || self.truthy.is_some()
            || self.steps.is_some()
            || self.allocated.is_some()
    }
}

/// F8: an expected trace, each entry a span, a name and an index.
pub type ExpectTrace = Vec<([u32; 4], String, Option<u64>)>;

#[derive(Clone, Debug, PartialEq)]
pub struct ExpectDiag {
    pub code: String,
    pub class: Option<String>,
    pub span: Option<[u32; 4]>,
    pub path: Option<Vec<Json>>,
    pub message: Option<String>,
    pub data: Option<Json>,
    /// F8: the trace, each entry a span, a name and an index
    /// (`f08-errors.md` 7.2).
    pub trace: Option<ExpectTrace>,
    /// F10: `"span": null`, the diagnostic has no span (10.2 item 5).
    pub span_null: bool,
}

#[derive(Clone, Debug, PartialEq)]
pub struct ExpectFailure {
    pub phase: Phase,
    pub diagnostics: Vec<ExpectDiag>,
    pub steps: Option<u64>,
    pub allocated: Option<u64>,
    /// F8: the renderings of the failure's diagnostics (`f08-errors.md` 7.3).
    pub rendered: Option<Renderings>,
}

#[derive(Clone, Debug, PartialEq)]
#[allow(clippy::large_enum_variant)]
pub enum Expect {
    Result(ExpectResult),
    Failure(ExpectFailure),
}

/// F9: the inner file of a runner case (`f09-testing.md` 3.1).
#[derive(Clone, Debug, PartialEq)]
pub enum Inner {
    /// The file as JSON data.
    Data(Json),
    /// The text of the file.
    Text(String),
}

/// F9: an entry of `verdicts` (`f09-testing.md` 3.4).
#[derive(Clone, Debug, PartialEq)]
pub struct VerdictEntry {
    pub name: String,
    pub status: String,
    pub differs: Option<Vec<String>>,
    pub fixture: Option<Vec<Json>>,
    pub message: Option<String>,
}

/// F9: a runner case (`f09-testing.md` 3).
#[derive(Clone, Debug, PartialEq)]
pub struct RunnerCase {
    pub inner: Inner,
    pub update: bool,
    pub program: bool,
    /// The directory of the file that holds the case.
    pub dir: PathBuf,
    /// `expect`: `invalid`, or the verdicts with the report and the text.
    pub invalid: bool,
    pub verdicts: Vec<VerdictEntry>,
    pub report: Option<String>,
    pub text: Option<String>,
}

/// One case, with the header laid over it (3.10.2).
#[derive(Clone, Debug, PartialEq)]
pub struct Case {
    pub name: String,
    /// The program text, from `source` or read from `file`.
    pub source: String,
    /// F2: the text is a template, taken in this mode (4.1).
    pub template: Option<crate::template::Mode>,
    pub version: Option<String>,
    pub env: Option<String>,
    /// F1: variables declared by schema, with their options (7.1).
    pub schemas: Vec<(String, Json, SchemaOptions)>,
    /// F4: named types, in order (9.1).
    pub types: Vec<TypeDecl>,
    /// F6: the opt-in groups the case turns on (`f06-groups.md` 11.1).
    pub groups: Vec<String>,
    /// F4: a case of the second kind: a type text and a host value (9.2).
    pub conform: Option<(String, Json)>,
    /// F8: the maximum of `ConformAll` in a conform case (`f08-errors.md` 7.5).
    pub conform_max: Option<usize>,
    /// F8: a case of the third kind (`f08-errors.md` 7.4).
    pub render: Option<RenderCase>,
    /// F9: a runner case (`f09-testing.md` 3).
    pub runner: Option<RunnerCase>,
    /// F12: a format, completion or hover case (`f12-tools.md` 7).
    pub tool: Option<super::tools::ToolCase>,
    /// The `expect` as written, or `None` when the case has none (update
    /// mode, `f09-testing.md` 4.2).
    pub expect_raw: Option<Json>,
    /// The base environment of the file it was loaded from (F9 7.4).
    pub base: Option<Base>,
    /// The case is of `suite/conversion/`, where two more codes are static.
    pub conversion: bool,
    pub opaque: Vec<String>,
    /// `Type.method` and its stub (3.10.4 item 2).
    pub methods: Vec<(String, Stub)>,
    pub functions: Vec<(String, Stub)>,
    pub reject: Option<Vec<String>>,
    /// The removed library entries (`03-host.md` 3.10.3).
    pub removed: Vec<String>,
    /// F14: the host facts `someNotNull`, each as written (`f14-host-facts.md`
    /// 10.1). `None` when the case gives no `facts`.
    pub facts: Option<Vec<Vec<String>>>,
    pub returns: Option<String>,
    /// F10: the expected result by schema, with its options (`f10-config.md`
    /// 10.1).
    pub returns_schema: Option<(Json, SchemaOptions)>,
    /// F10: the case is run as a load (10.2).
    pub load: bool,
    /// `None` when the case gives no `vars`.
    pub vars: Option<Vec<(String, Json)>>,
    pub limits: Limits,
    pub skip: Option<String>,
    pub precondition: Option<bool>,
    /// `false`: the program is one the type rules reject on purpose; it is
    /// run with the static rules alone (`03-host.md` 3.10.3).
    pub check: bool,
    pub expect: Expect,
}

/// A case file that was read and found valid.
#[derive(Clone, Debug)]
pub struct CaseFile {
    pub path: PathBuf,
    pub cases: Vec<Case>,
    /// The file's text, which update mode rewrites (`f09-testing.md` 4.5).
    pub text: Option<String>,
    /// A program case file (`f09-testing.md` 5), which is never written.
    pub program: bool,
    /// The base environment the file was loaded with (F9 7.4).
    pub base: Option<Base>,
}

/// The base environment of a loaded file, in which its cases run.
#[derive(Clone)]
pub struct Base(pub crate::env::Env);

impl std::fmt::Debug for Base {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("Base")
    }
}

impl PartialEq for Base {
    fn eq(&self, o: &Base) -> bool {
        std::sync::Arc::ptr_eq(&self.0.data, &o.0.data)
    }
}

/// Why a case file is invalid (`03-host.md` 3.10.10): the runner's own
/// reason (`f09-testing.md` 7.1).
#[derive(Clone, Debug, PartialEq)]
pub struct FileError(pub String);

impl std::fmt::Display for FileError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "the case file is invalid: {}", self.0)
    }
}

impl std::error::Error for FileError {}

const HEADER_FIELDS: [&str; 23] = [
    "facts",
    "returnsSchema",
    "load",
    "groups",
    "types",
    "conform",
    "template",
    "removed",
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

const CASE_FIELDS: [&str; 33] = [
    "format",
    "formatTemplate",
    "complete",
    "hover",
    "facts",
    "returnsSchema",
    "load",
    "runner",
    "update",
    "program",
    "render",
    "groups",
    "types",
    "conform",
    "template",
    "removed",
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

thread_local! {
    /// The base environment of the file being loaded (F9 7.1).
    static LOAD_BASE: std::cell::RefCell<Option<crate::env::Env>> =
        const { std::cell::RefCell::new(None) };
}

/// Read a value of the base environment of the load, when there is one.
fn base_sig(f: impl FnOnce(&crate::env::Env) -> Option<String>) -> Option<String> {
    LOAD_BASE.with(|b| b.borrow().as_ref().and_then(f))
}

/// Load with the base environment set for the rules that read it.
fn with_base<T>(base: Option<&crate::env::Env>, f: impl FnOnce() -> T) -> T {
    let old = LOAD_BASE.with(|b| b.replace(base.cloned()));
    let r = f();
    LOAD_BASE.with(|b| *b.borrow_mut() = old);
    r
}

/// Load a case file against a base environment (`f09-testing.md` 7.4): a
/// stub with no `type` takes the base's signature, and an invalid file is
/// reported here, before any case runs.
pub fn load(path: impl AsRef<Path>, base: &crate::env::Env) -> Result<CaseFile, FileError> {
    let f = with_base(Some(base), || load_file(path.as_ref()))?;
    Ok(f.with(base))
}

impl CaseFile {
    /// The file and each case, with the base they run in.
    fn with(mut self, base: &crate::env::Env) -> CaseFile {
        let b = Base(base.clone());
        for c in &mut self.cases {
            c.base = Some(b.clone());
        }
        self.base = Some(b);
        self
    }
}

/// Load a case file in the suite's base environment, the default one. An
/// `Err` is the reason the file is invalid: every case of it then has the
/// verdict `invalid` (3.10.10).
pub fn load_file(path: &Path) -> Result<CaseFile, FileError> {
    let text = std::fs::read_to_string(path)
        .map_err(|e| FileError(format!("cannot read the file: {e}")))?;
    // F9 5.1: a name that ends in `.test.risor` is a program case file.
    let program = path
        .file_name()
        .and_then(|n| n.to_str())
        .is_some_and(|n| n.ends_with(".test.risor"));
    let dir = path.parent().unwrap_or(Path::new("."));
    let mut f = read_text(&text, program, dir)?;
    f.path = path.to_path_buf();
    Ok(f)
}

/// A case file from its text (`f09-testing.md` 7.4): a program case file
/// when `program`. A `file` in it is relative to the current directory.
pub fn from_text(text: &str, program: bool, base: &crate::env::Env) -> Result<CaseFile, FileError> {
    let f = with_base(Some(base), || read_text(text, program, Path::new(".")))?;
    Ok(f.with(base))
}

fn read_text(text: &str, program: bool, dir: &Path) -> Result<CaseFile, FileError> {
    let doc = if program {
        super::program::program_value(text).map_err(FileError)?
    } else {
        json::parse(text).map_err(FileError)?
    };
    let update = std::env::var("RISOR_UPDATE").is_ok_and(|v| v == "1");
    let cases = if update {
        load_doc_lax(&doc, dir)
    } else {
        load_doc(&doc, dir)
    }
    .map_err(FileError)?;
    Ok(CaseFile {
        path: PathBuf::new(),
        cases,
        text: Some(text.to_string()),
        program,
        base: None,
    })
}

/// Read the cases of a parsed document: an object with `cases`, or an array
/// of cases with no header (3.10.1). `dir` resolves `file`.
pub fn load_doc(doc: &Json, dir: &Path) -> V<Vec<Case>> {
    load_doc_with(doc, dir, false)
}

/// As `load_doc`, but a case may have no `expect` (`suite/README.md` item 1): such a case
/// expects an empty result.
pub fn load_doc_lax(doc: &Json, dir: &Path) -> V<Vec<Case>> {
    load_doc_with(doc, dir, true)
}

fn load_doc_with(doc: &Json, dir: &Path, lax: bool) -> V<Vec<Case>> {
    // F4 6.1 item 4 and F5 11.2 item 3: two more static codes there.
    let conversion = dir
        .file_name()
        .is_some_and(|d| d == "conversion" || d == "unions");
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
        let mut case = merge(header, obj, dir, lax).map_err(|e| format!("case {i}: {e}"))?;
        case.conversion = conversion;
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
    // F12 7: a format, completion or hover case.
    for kind in [
        super::tools::ToolKind::Format,
        super::tools::ToolKind::FormatTemplate,
        super::tools::ToolKind::Complete,
        super::tools::ToolKind::Hover,
    ] {
        if own(kind.field()).is_some() {
            return super::tools::read(kind, header, case, |obj| merge(header, obj, dir, lax));
        }
    }
    // F8 7.4: a case of the third kind.
    if let Some(r) = own("render") {
        return render_case(name, r, header, case);
    }
    // F9 3: a runner case.
    if let Some(r) = own("runner") {
        return runner_case(name, r, header, case, dir);
    }
    if own("update").is_some() || own("program").is_some() {
        return Err("update or program in a case with no runner".into());
    }
    let expect = match own("expect") {
        Some(e) => read_expect(e)?,
        None if lax => Expect::Result(ExpectResult::default()),
        None => return Err("the case has no expect".into()),
    };

    // `source`, `file` and, with F4, `conform` count as one field.
    let (source, file, conform) =
        if own("source").is_some() || own("file").is_some() || own("conform").is_some() {
            (own("source"), own("file"), own("conform"))
        } else {
            (hdr("source"), hdr("file"), hdr("conform"))
        };
    let conform_max = conform
        .and_then(|c| c.get("max"))
        .and_then(|m| m.as_integer())
        .map(|m| m as usize);
    if let Expect::Result(ExpectResult {
        conform: Some(c), ..
    }) = &expect
    {
        if c.failures.is_some() && conform_max.is_none() {
            return Err("a conform result has failures and conform has no max".into());
        }
    }
    let conform = match (conform, source, file) {
        (Some(c), None, None) => Some((
            c.get("type")
                .and_then(|t| t.as_str())
                .unwrap_or_default()
                .to_string(),
            c.get("value").cloned().unwrap_or(Json::Null),
        )),
        (Some(_), _, _) => return Err("the case has conform beside source or file".into()),
        _ => None,
    };
    let source = match (source, file) {
        _ if conform.is_some() => String::new(),
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
            let mut stub = read_stub(stub);
            // 3.10.10 item 11: a stub with no type takes the base's
            // signature (F9 7.1).
            if stub.ty.is_none() {
                stub.ty = base_sig(|b| {
                    let d = b.data();
                    let f = &d.functions[b.function(fname)?];
                    Some(crate::describe::print_sig(&f.sig, &d.names))
                });
            }
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
            let mut stub = read_stub(stub);
            if stub.ty.is_none() {
                stub.ty = base_sig(|b| {
                    let d = b.data();
                    let (t, n) = key.split_once('.')?;
                    let m = d.methods.iter().find(|m| m.type_name == t && m.name == n)?;
                    Some(crate::describe::print_sig(&m.sig, &d.names))
                });
            }
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
                    && f.diagnostics.iter().any(|d| {
                        !is_static(&d.code, dir.file_name().is_some_and(|n| n == "conversion"))
                    })
            }
        };
        if bad {
            return Err("a case with \"check\": false states what needs the type checker".into());
        }
    }
    // F4 9.2: a conform case has few fields, and expects a conform result
    // or a `declare` failure; a conform result needs a conform case.
    let conform_result = matches!(&expect, Expect::Result(r) if r.conform.is_some());
    if conform.is_some() {
        for (k, _) in header.iter().chain(case.iter()) {
            if ![
                "name", "version", "opaque", "types", "conform", "skip", "expect", "cases",
            ]
            .contains(&k.as_str())
            {
                return Err(format!("a conform case has the field {k:?}"));
            }
        }
        let ok = conform_result
            || matches!(&expect, Expect::Failure(f) if f.phase == Phase::Declare)
            || (lax && own("expect").is_none());
        if !ok {
            return Err(
                "a conform case expects neither a conform result nor a declare failure".into(),
            );
        }
    } else if conform_result {
        return Err("a conform result in a case without conform".into());
    }
    let mut types = Vec::new();
    if let Some(a) = get("types").and_then(|v| v.as_arr()) {
        for t in a {
            let name = t
                .get("name")
                .and_then(|n| n.as_str())
                .unwrap_or_default()
                .to_string();
            if let Some(text) = t.get("type").and_then(|x| x.as_str()) {
                types.push(TypeDecl::Text(name, text.to_string()));
            } else {
                let names: Vec<&str> = t
                    .get("options")
                    .and_then(|o| o.as_arr())
                    .map(|a| a.iter().filter_map(|x| x.as_str()).collect())
                    .unwrap_or_default();
                types.push(TypeDecl::Schema(
                    name,
                    t.get("schema").cloned().unwrap_or(Json::Null),
                    SchemaOptions::from_names(&names)?,
                ));
            }
        }
    }
    // F10 10.1: `returns` and `returnsSchema` count as one field.
    let (returns, rs) = if own("returns").is_some() || own("returnsSchema").is_some() {
        (own("returns"), own("returnsSchema"))
    } else {
        (hdr("returns"), hdr("returnsSchema"))
    };
    if returns.is_some() && rs.is_some() {
        return Err("a case with both returns and returnsSchema".into());
    }
    let returns_schema = match rs {
        None => None,
        Some(r) => {
            let names: Vec<&str> = r
                .get("options")
                .and_then(|o| o.as_arr())
                .map(|a| a.iter().filter_map(|x| x.as_str()).collect())
                .unwrap_or_default();
            Some((
                r.get("schema").cloned().unwrap_or(Json::Null),
                SchemaOptions::from_names(&names)?,
            ))
        }
    };
    // F10 10.2.
    let load = matches!(get("load"), Some(Json::Bool(true)));
    if load {
        let typed = returns_schema.is_some()
            || returns
                .and_then(|r| r.as_str())
                .is_some_and(|r| r != "condition");
        if !typed {
            return Err("a load case with no expected type".into());
        }
        if matches!(get("check"), Some(Json::Bool(false))) {
            return Err("a load case with \"check\": false".into());
        }
    }
    if !load {
        if let Expect::Failure(f) = &expect {
            if f.diagnostics.iter().any(|d| d.span_null) {
                return Err("\"span\": null outside a load case".into());
            }
        }
    }
    // F2 4.2 item 3: an analysis case declares no environment.
    if matches!(&expect, Expect::Result(r) if r.analysis.is_some()) {
        for k in [
            "env",
            "opaque",
            "methods",
            "functions",
            "reject",
            "returns",
            "vars",
            "limits",
            "check",
            "precondition",
            "version",
        ] {
            if get(k).is_some() {
                return Err(format!("an analysis case has the field {k:?}"));
            }
        }
    }
    Ok(Case {
        name,
        template: get("template")
            .and_then(|v| v.as_str())
            .and_then(crate::template::Mode::from_name),
        source,
        types,
        groups: strings(get("groups")),
        conform,
        conform_max,
        render: None,
        runner: None,
        tool: None,
        expect_raw: own("expect").cloned(),
        conversion: false,
        version: get("version").and_then(|v| v.as_str()).map(String::from),
        env,
        schemas,
        opaque,
        methods,
        functions,
        reject: get("reject").map(|v| strings(Some(v))),
        removed: strings(get("removed")),
        facts: get("facts")
            .and_then(|v| v.as_arr())
            .map(|a| a.iter().map(|f| strings(f.get("someNotNull"))).collect()),
        returns: returns.and_then(|v| v.as_str()).map(String::from),
        returns_schema,
        load,
        vars,
        limits,
        skip: own("skip").and_then(|v| v.as_str()).map(String::from),
        check,
        precondition: match get("precondition") {
            Some(Json::Bool(b)) => Some(*b),
            _ => None,
        },
        expect,
        base: None,
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
            if !["phase", "diagnostics", "steps", "allocated", "rendered"].contains(&k.as_str()) {
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
        // `03-host.md` 3.10.10.
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
                // `03-host.md` 3.10.10.
                if diagnostics
                    .iter()
                    .any(|d| code::LIMIT.contains(&d.code.as_str()))
                {
                    return Err(format!("expect.{k} with a limit code"));
                }
            }
        }
        // F8 7.1, 7.2, 7.3 and 7.6.
        if phase == Phase::Declare && diagnostics.iter().any(|d| d.message.is_some()) {
            return Err("a message in a diagnostic of phase declare".into());
        }
        if phase != Phase::Run && diagnostics.iter().any(|d| d.trace.is_some()) {
            return Err(format!("a trace in a diagnostic of phase {phase}"));
        }
        let rendered = match v.get("rendered") {
            None => None,
            Some(r) => {
                if phase == Phase::Declare {
                    return Err("rendered in a failure of phase declare".into());
                }
                if diagnostics
                    .iter()
                    .any(|d| code::LIMIT.contains(&d.code.as_str()))
                {
                    return Err("rendered in a failure of a limit code".into());
                }
                Some(read_renderings(r).map_err(|e| format!("expect.rendered: {e}"))?)
            }
        };
        return Ok(Expect::Failure(ExpectFailure {
            phase,
            diagnostics,
            steps: count("steps"),
            allocated: count("allocated"),
            rendered,
        }));
    }
    if has("conforms") {
        let mut c = ConformExpect {
            conforms: false,
            path: None,
            message: None,
            steps: None,
            failures: None,
        };
        for (k, x) in obj {
            match k.as_str() {
                "conforms" => match x {
                    Json::Bool(b) => c.conforms = *b,
                    _ => return Err("expect.conforms is not a boolean".into()),
                },
                "path" => {
                    let a = x.as_arr().ok_or("expect.path is not an array")?;
                    for s in a {
                        let ok =
                            matches!(s, Json::Str(_)) || s.as_integer().is_some_and(|n| n >= 0.0);
                        if !ok {
                            return Err("a step of a path is neither a string nor an index".into());
                        }
                    }
                    c.path = Some(a.to_vec());
                }
                "message" => {
                    c.message = Some(
                        x.as_str()
                            .ok_or("expect.message is not a string")?
                            .to_string(),
                    );
                }
                "steps" => {
                    check_count(x).map_err(|e| format!("expect.steps: {e}"))?;
                    c.steps = x.as_integer().map(|n| n as u64);
                }
                "failures" => {
                    let a = x.as_arr().ok_or("expect.failures is not an array")?;
                    let mut out = Vec::new();
                    for f in a {
                        let m = f.as_obj().ok_or("a failure is not an object")?;
                        for (fk, _) in m {
                            if fk != "path" && fk != "message" {
                                return Err(format!("a failure has the field {fk:?}"));
                            }
                        }
                        let p = f
                            .get("path")
                            .and_then(|p| p.as_arr())
                            .ok_or("a failure has no path that is an array")?;
                        for st in p {
                            let ok = matches!(st, Json::Str(_))
                                || st.as_integer().is_some_and(|n| n >= 0.0);
                            if !ok {
                                return Err(
                                    "a step of a path is neither a string nor an index".into()
                                );
                            }
                        }
                        let msg = match f.get("message") {
                            None => None,
                            Some(m) => Some(
                                m.as_str()
                                    .ok_or("a failure's message is not a string")?
                                    .to_string(),
                            ),
                        };
                        out.push((p.to_vec(), msg));
                    }
                    c.failures = Some(out);
                }
                _ => return Err(format!("a conform result has the field {k:?}")),
            }
        }
        if c.conforms && (c.path.is_some() || c.message.is_some()) {
            return Err("a conform result with conforms true has a path or a message".into());
        }
        return Ok(Expect::Result(ExpectResult {
            conform: Some(c),
            ..ExpectResult::default()
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
            "json" => {
                x.as_str().ok_or("expect.json is not a string")?;
            }
            "analysis" => {
                if obj.len() != 1 {
                    return Err("expect has analysis and another field".into());
                }
                check_analysis(x)?;
            }
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
        analysis: v.get("analysis").cloned(),
        conform: None,
        json: v.get("json").and_then(|j| j.as_str()).map(String::from),
    }))
}

/// F2 4.3 and 4.5 item 4: the form of an expected `analysis`.
fn check_analysis(x: &Json) -> V<()> {
    let m = x.as_obj().ok_or("expect.analysis is not an object")?;
    let span = |v: &Json| check_span(v);
    let exact = |o: &Json, keys: &[&str], what: &str| -> V<()> {
        let om = o
            .as_obj()
            .ok_or_else(|| format!("an element of {what} is not an object"))?;
        if om.len() != keys.len() || !keys.iter().all(|k| o.get(k).is_some()) {
            return Err(format!(
                "an element of {what} does not have exactly {keys:?}"
            ));
        }
        Ok(())
    };
    for (k, v) in m {
        match k.as_str() {
            "names" => {
                let a = v.as_arr().ok_or("analysis.names is not an array")?;
                if !a.iter().all(|n| n.as_str().is_some()) {
                    return Err("analysis.names holds a value that is not a string".into());
                }
            }
            "refs" => {
                let a = v.as_arr().ok_or("analysis.refs is not an array")?;
                for r in a {
                    exact(r, &["name", "path", "span"], "refs")?;
                    r.get("name")
                        .and_then(|n| n.as_str())
                        .ok_or("a reference's name is not a string")?;
                    let p = r
                        .get("path")
                        .and_then(|p| p.as_arr())
                        .ok_or("a reference's path is not an array")?;
                    // F7 9.3: a key, or the star step `{"star": true}`.
                    let step_ok = |s: &Json| {
                        s.as_str().is_some()
                            || matches!(s.as_obj(), Some(o) if o.len() == 1 && o[0].0 == "star" && o[0].1 == Json::Bool(true))
                    };
                    if !p.iter().all(step_ok) {
                        return Err("a path holds a step that is neither a key nor a star".into());
                    }
                    span(r.get("span").unwrap_or(&Json::Null))?;
                }
            }
            "calls" => {
                let a = v.as_arr().ok_or("analysis.calls is not an array")?;
                for c in a {
                    exact(c, &["name", "namespace", "span", "args", "lambda"], "calls")?;
                    c.get("name")
                        .and_then(|n| n.as_str())
                        .ok_or("a call's name is not a string")?;
                    if !matches!(c.get("namespace"), Some(Json::Str(_) | Json::Null)) {
                        return Err("a call's namespace is neither a string nor null".into());
                    }
                    span(c.get("span").unwrap_or(&Json::Null))?;
                    for s in c
                        .get("args")
                        .and_then(|a| a.as_arr())
                        .ok_or("a call's args is not an array")?
                    {
                        span(s)?;
                    }
                    if !matches!(c.get("lambda"), Some(Json::Bool(_))) {
                        return Err("a call's lambda is not a boolean".into());
                    }
                }
            }
            "literal" => match v {
                Json::Bool(false) => {}
                Json::Obj(o) if o.len() == 1 && o[0].0 == "value" => {}
                _ => return Err("analysis.literal is neither false nor {\"value\": v}".into()),
            },
            "segments" => match v {
                Json::Null => {}
                Json::Arr(a) => {
                    for g in a {
                        if g.get("text").is_some() {
                            exact(g, &["text", "span"], "segments")?;
                            g.get("text")
                                .and_then(|t| t.as_str())
                                .ok_or("a text segment's text is not a string")?;
                        } else {
                            exact(g, &["part", "span"], "segments")?;
                            span(g.get("part").unwrap_or(&Json::Null))?;
                        }
                        span(g.get("span").unwrap_or(&Json::Null))?;
                    }
                }
                _ => return Err("analysis.segments is neither null nor an array".into()),
            },
            _ => return Err(format!("analysis has the field {k:?}")),
        }
    }
    Ok(())
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
        // F4 8: `types`; F6 2.7: `groups`; F14 9.1: `facts`.
        if k == "types" || k == "groups" || k == "facts" {
            continue;
        }
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

/// F4 9.1: the form of `types`.
fn check_types(v: &Json) -> V<()> {
    let a = v.as_arr().ok_or("types is not an array")?;
    for t in a {
        let m = t.as_obj().ok_or("an entry of types is not an object")?;
        for (k, _) in m {
            if !["name", "type", "schema", "options"].contains(&k.as_str()) {
                return Err(format!("an entry of types has the field {k:?}"));
            }
        }
        if !t.get("name").is_some_and(|n| n.as_str().is_some()) {
            return Err("an entry of types has no name that is a string".into());
        }
        match (t.get("type"), t.get("schema")) {
            (Some(x), None) => {
                x.as_str().ok_or("an entry's type is not a string")?;
                if t.get("options").is_some() {
                    return Err("an entry of types has options beside type".into());
                }
            }
            (None, Some(_)) => {
                if let Some(o) = t.get("options") {
                    let a = o.as_arr().ok_or("options is not an array")?;
                    let names: Vec<&str> = a.iter().filter_map(|x| x.as_str()).collect();
                    if names.len() != a.len() {
                        return Err("options holds a value that is not a string".into());
                    }
                    SchemaOptions::from_names(&names)?;
                }
            }
            _ => return Err("an entry of types has both type and schema, or neither".into()),
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
            // F10 10.2 item 5: null, in a load case.
            "span" if *v == Json::Null => {}
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
            "trace" => {
                read_trace(v)?;
            }
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
        span: d.get("span").filter(|v| **v != Json::Null).map(span_of),
        span_null: d.get("span") == Some(&Json::Null),
        path: d.get("path").and_then(|p| p.as_arr()).map(|a| a.to_vec()),
        message: d.get("message").and_then(|m| m.as_str()).map(String::from),
        data: d.get("data").cloned(),
        trace: match d.get("trace") {
            Some(t) => Some(read_trace(t)?),
            None => None,
        },
    })
}

/// F8 7.2: a trace, a list of entries with exactly `span`, `name` and
/// `index`.
fn read_trace(v: &Json) -> V<ExpectTrace> {
    let a = v.as_arr().ok_or("a trace is not an array")?;
    let mut out = Vec::new();
    for e in a {
        let m = e.as_obj().ok_or("a trace entry is not an object")?;
        if m.len() != 3 {
            return Err("a trace entry does not have exactly span, name and index".into());
        }
        let span = e.get("span").ok_or("a trace entry has no span")?;
        check_span(span)?;
        let name = e
            .get("name")
            .and_then(|n| n.as_str())
            .ok_or("a trace entry has no name that is a string")?;
        let index = match e.get("index") {
            Some(Json::Null) => None,
            Some(n) => match n.as_integer() {
                Some(i) if i >= 0.0 => Some(i as u64),
                _ => return Err("a trace entry's index is not a whole number or null".into()),
            },
            None => return Err("a trace entry has no index".into()),
        };
        out.push((span_of(span), name.to_string(), index));
    }
    Ok(out)
}

/// F8 7.3: an object with one or more of `line`, `detail` and `json`.
fn read_renderings(v: &Json) -> V<Renderings> {
    let m = v.as_obj().ok_or("not an object")?;
    if m.is_empty() {
        return Err("no rendering".into());
    }
    let mut r = Renderings::default();
    for (k, x) in m {
        let t = x
            .as_str()
            .ok_or_else(|| format!("{k} is not a string"))?
            .to_string();
        match k.as_str() {
            "line" => r.line = Some(t),
            "detail" => r.detail = Some(t),
            "json" => r.json = Some(t),
            _ => return Err(format!("the field {k:?}")),
        }
    }
    Ok(r)
}

/// F8 7.4: a case with `render`.
fn render_case(
    name: String,
    r: &Json,
    header: &[(String, Json)],
    case: &[(String, Json)],
) -> V<Case> {
    for (k, _) in case {
        if !["name", "opaque", "render", "skip", "expect"].contains(&k.as_str()) {
            return Err(format!("a render case has the field {k:?}"));
        }
    }
    for (k, _) in header {
        if ["source", "file", "conform"].contains(&k.as_str()) {
            return Err(format!("a render case under a header with {k:?}"));
        }
    }
    let m = r.as_obj().ok_or("render is not an object")?;
    for (k, _) in m {
        if k != "diagnostics" && k != "source" {
            return Err(format!("render has the field {k:?}"));
        }
    }
    let source = match r.get("source") {
        None => None,
        Some(s) => Some(
            s.as_str()
                .ok_or("render.source is not a string")?
                .to_string(),
        ),
    };
    let ds = r
        .get("diagnostics")
        .and_then(|d| d.as_arr())
        .ok_or("render.diagnostics is not an array")?;
    if ds.is_empty() {
        return Err("render.diagnostics is empty".into());
    }
    let own = |k: &str| case.iter().find(|(n, _)| n == k).map(|(_, v)| v);
    let get = |k: &str| own(k).or_else(|| header.iter().find(|(n, _)| n == k).map(|(_, v)| v));
    let opaque: Vec<String> = get("opaque")
        .and_then(|v| v.as_arr())
        .map(|a| {
            a.iter()
                .filter_map(|s| s.as_str().map(String::from))
                .collect()
        })
        .unwrap_or_default();
    for d in ds {
        let dm = d.as_obj().ok_or("a diagnostic is not an object")?;
        for (k, x) in dm {
            match k.as_str() {
                "phase" => {
                    if !matches!(x.as_str(), Some("declare" | "parse" | "check" | "run")) {
                        return Err("a diagnostic's phase is not a phase".into());
                    }
                }
                "code" | "message" | "class" => {
                    x.as_str()
                        .ok_or_else(|| format!("a diagnostic's {k} is not a string"))?;
                }
                "span" => check_span(x)?,
                "path" => {
                    let a = x.as_arr().ok_or("a path is not an array")?;
                    for st in a {
                        let ok =
                            matches!(st, Json::Str(_)) || st.as_integer().is_some_and(|n| n >= 0.0);
                        if !ok {
                            return Err("a step of a path is neither a string nor an index".into());
                        }
                    }
                }
                "data" => check_opaque_values(x, &opaque)?,
                "trace" => {
                    read_trace(x)?;
                }
                _ => return Err(format!("a diagnostic has the field {k:?}")),
            }
        }
        for k in ["phase", "code", "message"] {
            if d.get(k).is_none() {
                return Err(format!("a diagnostic has no {k}"));
            }
        }
        let code = d.get("code").and_then(|c| c.as_str()).unwrap_or_default();
        let phase = d.get("phase").and_then(|c| c.as_str()).unwrap_or_default();
        if d.get("data").is_some() && code != "failed" {
            return Err("data on a code other than failed".into());
        }
        if d.get("path").is_some() && code != "not-conforming" {
            return Err("path on a code other than not-conforming".into());
        }
        if d.get("trace").is_some() && phase != "run" {
            return Err("a trace on a phase other than run".into());
        }
        // 7.4 item 3: a span inside the source text, when there is one.
        if let (Some(sp), Some(text)) = (d.get("span"), source.as_deref()) {
            let [l1, c1, l2, c2] = span_of(sp);
            let text = text.strip_prefix('\u{feff}').unwrap_or(text);
            let lines: Vec<usize> = text.split('\n').map(|l| l.chars().count()).collect();
            let inside = |l: u32, c: u32| {
                lines
                    .get(l as usize - 1)
                    .is_some_and(|n| c as usize <= n + 1)
            };
            if !inside(l1, c1) || !inside(l2, c2) || (l2, c2) < (l1, c1) {
                return Err("a span outside the source text".into());
            }
        }
    }
    let expect = read_renderings(own("expect").ok_or("the case has no expect")?)
        .map_err(|e| format!("expect: {e}"))?;
    Ok(Case {
        name,
        source: source.clone().unwrap_or_default(),
        template: None,
        version: None,
        env: None,
        schemas: Vec::new(),
        types: Vec::new(),
        groups: Vec::new(),
        conform: None,
        conform_max: None,
        runner: None,
        tool: None,
        expect_raw: own("expect").cloned(),
        render: Some(RenderCase {
            diagnostics: ds.to_vec(),
            source,
            expect,
        }),
        conversion: false,
        opaque,
        methods: Vec::new(),
        functions: Vec::new(),
        reject: None,
        removed: Vec::new(),
        facts: None,
        returns: None,
        returns_schema: None,
        load: false,
        vars: None,
        limits: Limits::default(),
        skip: own("skip").and_then(|v| v.as_str()).map(String::from),
        precondition: None,
        check: true,
        expect: Expect::Result(ExpectResult::default()),
        base: None,
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
        "format" | "formatTemplate" | "complete" | "hover" => string(k),
        "file" => match v.as_str() {
            Some(s) if !s.is_empty() => Ok(()),
            _ => Err("file is not a non-empty string".into()),
        },
        "version" => match v.as_str() {
            Some(s) if is_version(s) => Ok(()),
            _ => Err("version is not major.minor".into()),
        },
        "opaque" | "reject" | "removed" => strings(k),
        // F14 10.1.
        "facts" => {
            let a = v.as_arr().ok_or("facts is not an array")?;
            for f in a {
                let m = f.as_obj().ok_or("an entry of facts is not an object")?;
                if m.len() != 1 || m[0].0 != "someNotNull" {
                    return Err("an entry of facts is not {\"someNotNull\": [names]}".into());
                }
                let names = m[0].1.as_arr().ok_or("someNotNull is not an array")?;
                if names.iter().any(|n| n.as_str().is_none()) {
                    return Err("someNotNull holds a value that is not a string".into());
                }
            }
            Ok(())
        }
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
        "load" => match v {
            Json::Bool(_) => Ok(()),
            _ => Err("load is not true or false".into()),
        },
        "returnsSchema" => {
            let m = v.as_obj().ok_or("returnsSchema is not an object")?;
            for (k, x) in m {
                match k.as_str() {
                    "schema" => {}
                    "options" => {
                        let a = x.as_arr().ok_or("returnsSchema.options is not an array")?;
                        let names: Vec<&str> = a.iter().filter_map(|o| o.as_str()).collect();
                        if names.len() != a.len() {
                            return Err(
                                "returnsSchema.options holds a value that is not a string".into()
                            );
                        }
                        SchemaOptions::from_names(&names)?;
                    }
                    _ => return Err(format!("returnsSchema has the field {k:?}")),
                }
            }
            if v.get("schema").is_none() {
                return Err("returnsSchema has no schema".into());
            }
            Ok(())
        }
        "precondition" | "check" => match v {
            Json::Bool(_) => Ok(()),
            _ => Err("precondition is not a boolean".into()),
        },
        "expect" => Ok(()),
        "schemas" => check_schemas(v),
        "types" => check_types(v),
        "groups" => {
            let a = v.as_arr().ok_or("groups is not an array")?;
            let mut seen: Vec<&str> = Vec::new();
            for g in a {
                let g = g
                    .as_str()
                    .ok_or("groups holds a value that is not a string")?;
                if seen.contains(&g) {
                    return Err(format!("groups names {g:?} twice"));
                }
                seen.push(g);
            }
            Ok(())
        }
        "conform" => {
            let m = v.as_obj().ok_or("conform is not an object")?;
            // F8 7.5: and `max`, a whole number of 1 or more.
            let max_ok = match v.get("max") {
                None => true,
                Some(x) => x.as_integer().is_some_and(|n| n >= 1.0),
            };
            if !max_ok {
                return Err("conform.max is not a whole number of 1 or more".into());
            }
            let n = 2 + usize::from(v.get("max").is_some());
            if m.len() != n
                || v.get("value").is_none()
                || !v.get("type").is_some_and(|t| t.as_str().is_some())
            {
                return Err("conform is not {\"type\": text, \"value\": v}".into());
            }
            Ok(())
        }
        "render" => Ok(()),
        "runner" => match v {
            Json::Str(_) | Json::Obj(_) | Json::Arr(_) => Ok(()),
            _ => Err("runner is not a string, an object or an array".into()),
        },
        "update" | "program" => match v {
            Json::Bool(_) => Ok(()),
            _ => Err(format!("{k} is not true or false")),
        },
        "template" => match v.as_str() {
            Some("string" | "value") => Ok(()),
            _ => Err("template is neither \"string\" nor \"value\"".into()),
        },
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

/// F9 3: a runner case, and its `expect` (3.3, 9.2).
fn runner_case(
    name: String,
    r: &Json,
    header: &[(String, Json)],
    case: &[(String, Json)],
    dir: &Path,
) -> V<Case> {
    const OWN: [&str; 6] = ["name", "runner", "update", "program", "skip", "expect"];
    for (k, _) in case {
        if !OWN.contains(&k.as_str()) {
            return Err(format!("a runner case has the field {k:?}"));
        }
    }
    // After the header rule: the header's fields reach the case, but for
    // those that count with `runner` as one field.
    for (k, _) in header {
        if k != "cases" && !["source", "file", "conform", "render"].contains(&k.as_str()) {
            return Err(format!("a runner case under a header with {k:?}"));
        }
    }
    let own = |k: &str| case.iter().find(|(n, _)| n == k).map(|(_, v)| v);
    let flag = |k: &str| matches!(own(k), Some(Json::Bool(true)));
    let (update, program) = (flag("update"), flag("program"));
    let inner = match r {
        Json::Str(t) => Inner::Text(t.clone()),
        _ => {
            if update || program {
                return Err("update or program true beside a runner that is not a string".into());
            }
            Inner::Data(r.clone())
        }
    };
    let e = own("expect").ok_or("the case has no expect")?;
    let m = e.as_obj().ok_or("expect is not an object")?;
    let mut rc = RunnerCase {
        inner,
        update,
        program,
        dir: dir.to_path_buf(),
        invalid: false,
        verdicts: Vec::new(),
        report: None,
        text: None,
    };
    if let Some(x) = e.get("invalid") {
        if *x != Json::Bool(true) || m.len() != 1 {
            return Err("expect.invalid is not true alone".into());
        }
        rc.invalid = true;
    } else {
        let vs = e
            .get("verdicts")
            .ok_or("a runner case expects neither invalid nor verdicts")?
            .as_arr()
            .ok_or("verdicts is not a list")?;
        for (k, x) in m {
            match k.as_str() {
                "verdicts" => {}
                "report" => {
                    rc.report = Some(x.as_str().ok_or("report is not a string")?.to_string())
                }
                "text" => {
                    if !update {
                        return Err("text in a runner case without update".into());
                    }
                    rc.text = Some(x.as_str().ok_or("text is not a string")?.to_string());
                }
                _ => return Err(format!("the expect of a runner case has the field {k:?}")),
            }
        }
        for v in vs {
            rc.verdicts.push(verdict_entry(v, update)?);
        }
    }
    Ok(Case {
        name,
        source: String::new(),
        template: None,
        version: None,
        env: None,
        schemas: Vec::new(),
        types: Vec::new(),
        groups: Vec::new(),
        conform: None,
        conform_max: None,
        render: None,
        runner: Some(rc),
        tool: None,
        expect_raw: Some(e.clone()),
        conversion: false,
        opaque: Vec::new(),
        methods: Vec::new(),
        functions: Vec::new(),
        reject: None,
        removed: Vec::new(),
        facts: None,
        returns: None,
        returns_schema: None,
        load: false,
        vars: None,
        limits: Limits::default(),
        skip: own("skip").and_then(|v| v.as_str()).map(String::from),
        precondition: None,
        check: true,
        expect: Expect::Result(ExpectResult::default()),
        base: None,
    })
}

/// F9 3.4 and 9.2 item 6.
fn verdict_entry(v: &Json, update: bool) -> V<VerdictEntry> {
    let m = v.as_obj().ok_or("a verdict is not an object")?;
    let mut e = VerdictEntry {
        name: String::new(),
        status: String::new(),
        differs: None,
        fixture: None,
        message: None,
    };
    for (k, x) in m {
        match k.as_str() {
            "name" => {
                e.name = x
                    .as_str()
                    .ok_or("a verdict's name is not a string")?
                    .to_string()
            }
            "status" => {
                let s = x.as_str().ok_or("a verdict's status is not a string")?;
                let ok =
                    matches!(s, "pass" | "fail" | "skip" | "invalid") || (s == "updated" && update);
                if !ok {
                    return Err(format!("the status {s:?}"));
                }
                e.status = s.to_string();
            }
            "differs" => {
                let a = x.as_arr().ok_or("differs is not a list")?;
                let mut out = Vec::new();
                for d in a {
                    out.push(
                        d.as_str()
                            .ok_or("differs holds a value that is not a string")?
                            .to_string(),
                    );
                }
                e.differs = Some(out);
            }
            "fixture" => {
                let a = x.as_arr().ok_or("fixture is not a path")?;
                for st in a {
                    let ok =
                        matches!(st, Json::Str(_)) || st.as_integer().is_some_and(|n| n >= 0.0);
                    if !ok {
                        return Err("fixture is not a path".into());
                    }
                }
                e.fixture = Some(a.to_vec());
            }
            "message" => e.message = Some(x.as_str().ok_or("message is not a string")?.to_string()),
            _ => return Err(format!("a verdict has the field {k:?}")),
        }
    }
    if v.get("name").is_none() || v.get("status").is_none() {
        return Err("a verdict needs name and status".into());
    }
    if e.differs.is_some() && e.status != "fail" {
        return Err("differs beside a status other than fail".into());
    }
    if (e.fixture.is_some() || e.message.is_some()) && e.status != "invalid" {
        return Err("fixture or message beside a status other than invalid".into());
    }
    Ok(e)
}
