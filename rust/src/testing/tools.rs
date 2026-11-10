//! Package F12: format, completion and hover cases (`f12-tools.md` 7).

use super::case::{Case, Expect};
use super::run::{case_env, compare_failure_pub, form, outcome_line, Difference, Verdict};
use crate::diag::{Diagnostic, Phase};
use crate::json::{self, Json};
use crate::source::{Source, Span};

/// The kind of a case of F12.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ToolKind {
    Format,
    FormatTemplate,
    Complete,
    Hover,
}

impl ToolKind {
    pub fn field(self) -> &'static str {
        match self {
            ToolKind::Format => "format",
            ToolKind::FormatTemplate => "formatTemplate",
            ToolKind::Complete => "complete",
            ToolKind::Hover => "hover",
        }
    }
}

/// A case of F12: its kind, its text (without `‸`), the position, and the
/// expected result when it is not a failure.
#[derive(Clone, Debug, PartialEq)]
pub struct ToolCase {
    pub kind: ToolKind,
    pub text: String,
    /// For completion and hover: the code points before `‸`.
    pub offset: usize,
    /// `formatted`, `completion` or `hover`, as written.
    pub want: Option<Json>,
}

pub(super) const TOOL_FIELDS: [&str; 4] = ["format", "formatTemplate", "complete", "hover"];

const SOURCE_GROUP: [&str; 10] = [
    "source",
    "file",
    "template",
    "conform",
    "render",
    "runner",
    "format",
    "formatTemplate",
    "complete",
    "hover",
];

/// Read a case of F12 (7.1 to 7.4). `merge` reads the rest of it as a
/// program case whose source is the text.
pub(super) fn read(
    kind: ToolKind,
    header: &[(String, Json)],
    case: &[(String, Json)],
    merge: impl FnOnce(&[(String, Json)]) -> Result<Case, String>,
) -> Result<Case, String> {
    let allowed: &[&str] = match kind {
        ToolKind::Format | ToolKind::FormatTemplate => &[
            "name",
            "format",
            "formatTemplate",
            "env",
            "vars",
            "skip",
            "expect",
        ],
        ToolKind::Complete | ToolKind::Hover => &[
            "name",
            "complete",
            "hover",
            "version",
            "env",
            "opaque",
            "methods",
            "functions",
            "removed",
            "groups",
            "types",
            "returns",
            "skip",
            "expect",
        ],
    };
    let own: Vec<&str> = case.iter().map(|(k, _)| k.as_str()).collect();
    let tools = own.iter().filter(|k| TOOL_FIELDS.contains(k)).count();
    if tools != 1 {
        return Err(
            "a case has more than one of format, formatTemplate, complete and hover".into(),
        );
    }
    for (k, _) in case {
        if !allowed.contains(&k.as_str()) {
            return Err(format!("a {} case has the field {k:?}", kind.field()));
        }
    }
    for (k, _) in header {
        if k == "cases" || SOURCE_GROUP.contains(&k.as_str()) || own.contains(&k.as_str()) {
            continue;
        }
        if !allowed.contains(&k.as_str()) {
            return Err(format!(
                "a {} case has the field {k:?} from its header",
                kind.field()
            ));
        }
    }
    let get = |k: &str| case.iter().find(|(n, _)| n == k).map(|(_, v)| v);
    let raw = get(kind.field())
        .and_then(|v| v.as_str())
        .ok_or_else(|| format!("{} is not a string", kind.field()))?;
    let (text, offset) = match kind {
        ToolKind::Complete | ToolKind::Hover => {
            let marks = raw.chars().filter(|&c| c == '\u{2038}').count();
            if marks != 1 {
                return Err(format!("{} holds the mark not exactly once", kind.field()));
            }
            let mut off = raw.chars().position(|c| c == '\u{2038}').unwrap_or(0);
            if raw.starts_with('\u{FEFF}') && off > 0 {
                off -= 1;
            }
            (raw.replace('\u{2038}', ""), off)
        }
        _ => (raw.to_string(), 0),
    };
    // The expected result.
    let expect = get("expect").ok_or("the case has no expect")?;
    let eo = expect.as_obj().ok_or("expect is not an object")?;
    let result_key = match kind {
        ToolKind::Format | ToolKind::FormatTemplate => "formatted",
        ToolKind::Complete => "completion",
        ToolKind::Hover => "hover",
    };
    let failure_phase = match kind {
        ToolKind::Format | ToolKind::FormatTemplate => "parse",
        _ => "declare",
    };
    let want = if eo.iter().any(|(k, _)| k == "phase") {
        if expect.get("phase").and_then(|p| p.as_str()) != Some(failure_phase) {
            return Err(format!(
                "a {} case expects a failure of another phase",
                kind.field()
            ));
        }
        None
    } else {
        if eo.len() != 1 || eo[0].0 != result_key {
            return Err(format!(
                "expect is not of the form of a {} case",
                kind.field()
            ));
        }
        let w = &eo[0].1;
        match kind {
            ToolKind::Format | ToolKind::FormatTemplate => {
                w.as_str().ok_or("formatted is not a string")?;
            }
            ToolKind::Complete => check_completion(w)?,
            ToolKind::Hover => check_hover(w)?,
        }
        Some(w.clone())
    };
    // The rest, read as a program case.
    let mut obj: Vec<(String, Json)> = case
        .iter()
        .filter(|(k, _)| k != kind.field() && k != "expect")
        .cloned()
        .collect();
    obj.push(("source".into(), Json::Str(text.clone())));
    let e = match &want {
        Some(_) => Json::Obj(vec![("value".into(), Json::Null)]),
        None => expect.clone(),
    };
    obj.push(("expect".into(), e));
    let mut c = merge(&obj)?;
    if want.is_some() {
        c.expect = Expect::Result(Default::default());
    }
    c.expect_raw = Some(expect.clone());
    c.tool = Some(ToolCase {
        kind,
        text,
        offset,
        want,
    });
    Ok(c)
}

fn span_ok(v: &Json) -> bool {
    v.as_arr()
        .is_some_and(|a| a.len() == 4 && a.iter().all(|x| x.as_integer().is_some_and(|n| n >= 1.0)))
}

fn check_completion(w: &Json) -> Result<(), String> {
    let m = w.as_obj().ok_or("completion is not an object")?;
    for (k, _) in m {
        if k != "span" && k != "items" {
            return Err(format!("completion has the field {k:?}"));
        }
    }
    if !w.get("span").is_some_and(span_ok) {
        return Err("completion.span is not a span".into());
    }
    let items = w
        .get("items")
        .and_then(|i| i.as_arr())
        .ok_or("completion.items is not an array")?;
    const KINDS: [&str; 11] = [
        "name",
        "lambda",
        "variable",
        "function",
        "namespace",
        "field",
        "method",
        "constant",
        "key",
        "literal",
        "type",
    ];
    for it in items {
        let o = it.as_obj().ok_or("a candidate is not an object")?;
        for (k, v) in o {
            let ok = match k.as_str() {
                "label" | "insert" => v.as_str().is_some(),
                "kind" => v.as_str().is_some_and(|s| KINDS.contains(&s)),
                "detail" => matches!(v, Json::Str(_) | Json::Null),
                _ => false,
            };
            if !ok {
                return Err(format!("a candidate has the field {k:?} of a wrong form"));
            }
        }
        if it.get("label").is_none() || it.get("kind").is_none() {
            return Err("a candidate has no label or kind".into());
        }
    }
    Ok(())
}

fn check_hover(w: &Json) -> Result<(), String> {
    if matches!(w, Json::Null) {
        return Ok(());
    }
    let o = w.as_obj().ok_or("hover is neither null nor an object")?;
    const KINDS: [&str; 8] = [
        "name",
        "lambda",
        "variable",
        "function",
        "namespace",
        "field",
        "method",
        "constant",
    ];
    for (k, v) in o {
        let ok = match k.as_str() {
            "kind" => v.as_str().is_some_and(|s| KINDS.contains(&s)),
            "span" => span_ok(v),
            "type" | "signature" | "doc" => matches!(v, Json::Str(_) | Json::Null),
            "declaration" => matches!(v, Json::Null) || span_ok(v),
            _ => false,
        };
        if !ok {
            return Err(format!(
                "the hover record has the field {k:?} of a wrong form"
            ));
        }
    }
    if w.get("kind").is_none() || w.get("span").is_none() {
        return Err("the hover record has no kind or span".into());
    }
    Ok(())
}

fn span_json(src: &Source, s: Span) -> Json {
    Json::Arr(
        src.line_span(s)
            .0
            .iter()
            .map(|&x| Json::Num(x as f64))
            .collect(),
    )
}

fn opt(s: &Option<String>) -> Json {
    match s {
        Some(s) => Json::Str(s.clone()),
        None => Json::Null,
    }
}

/// What a completion or hover case found, as JSON, or the declare failure.
fn assist_result(case: &Case, tc: &ToolCase) -> Result<Json, crate::diag::Error> {
    let env = case_env(case)?;
    let expected =
        crate::check::expected(&env, case.returns.as_deref()).map_err(crate::diag::Error::one)?;
    let src = Source::new(&tc.text);
    Ok(match tc.kind {
        ToolKind::Complete => {
            let c = crate::assist::complete(&env, &tc.text, &expected, tc.offset);
            let items = c
                .items
                .iter()
                .map(|i| {
                    Json::Obj(vec![
                        ("label".into(), Json::Str(i.label.clone())),
                        ("kind".into(), Json::Str(i.kind.into())),
                        ("detail".into(), opt(&i.detail)),
                        ("insert".into(), Json::Str(i.insert.clone())),
                    ])
                })
                .collect();
            Json::Obj(vec![
                ("span".into(), span_json(&src, c.span)),
                ("items".into(), Json::Arr(items)),
            ])
        }
        _ => match crate::assist::hover(&env, &tc.text, &expected, tc.offset) {
            None => Json::Null,
            Some(h) => Json::Obj(vec![
                ("kind".into(), Json::Str(h.kind.into())),
                ("span".into(), span_json(&src, h.span)),
                ("type".into(), opt(&h.ty)),
                ("signature".into(), opt(&h.signature)),
                ("doc".into(), opt(&h.doc)),
                (
                    "declaration".into(),
                    h.declaration
                        .map(|d| span_json(&src, d))
                        .unwrap_or(Json::Null),
                ),
            ]),
        },
    })
}

fn format_text(tc: &ToolCase, text: &str) -> Result<String, crate::diag::Error> {
    match tc.kind {
        ToolKind::FormatTemplate => crate::format::format_template(text),
        _ => crate::format::format(text),
    }
}

/// Run a case of F12.
pub(super) fn run(case: &Case, tc: &ToolCase) -> Verdict {
    match tc.kind {
        ToolKind::Format | ToolKind::FormatTemplate => run_format(case, tc),
        _ => run_assist(case, tc),
    }
}

fn outcome_diff(case: &Case, found: String) -> Difference {
    Difference {
        name: "outcome",
        expected: case.expect_raw.as_ref().map(form).unwrap_or_default(),
        found,
    }
}

fn run_format(case: &Case, tc: &ToolCase) -> Verdict {
    let mut diffs = Vec::new();
    match (format_text(tc, &tc.text), &case.expect) {
        (Err(e), Expect::Failure(f)) => {
            diffs.extend(compare_failure_pub(case, f, e.phase, &e.diagnostics));
        }
        (Err(e), _) => diffs.push(outcome_diff(case, failure_text(e.phase, &e.diagnostics))),
        (Ok(s), Expect::Failure(_)) => diffs.push(outcome_diff(case, form(&Json::Str(s)))),
        (Ok(s), _) => {
            if let Some(Json::Str(w)) = &tc.want {
                if *w != s {
                    diffs.push(Difference {
                        name: "formatted",
                        expected: form(&Json::Str(w.clone())),
                        found: form(&Json::Str(s.clone())),
                    });
                }
            }
            match format_text(tc, &s) {
                Ok(again) if again == s => {}
                other => diffs.push(Difference {
                    name: "idempotent",
                    expected: form(&Json::Str(s.clone())),
                    found: match other {
                        Ok(a) => form(&Json::Str(a)),
                        Err(e) => failure_text(e.phase, &e.diagnostics),
                    },
                }),
            }
            if let Some(why) = tree_law(tc, &s) {
                diffs.push(Difference {
                    name: "tree",
                    expected: "the tree of the source".into(),
                    found: why,
                });
            }
            if case.vars.is_some() && tc.kind == ToolKind::Format {
                let a = run_outcome(case, &tc.text);
                let b = run_outcome(case, &s);
                if a != b {
                    diffs.push(Difference {
                        name: "run",
                        expected: a,
                        found: b,
                    });
                }
            }
        }
    }
    if diffs.is_empty() {
        Verdict::Pass
    } else {
        Verdict::Fail(diffs)
    }
}

fn failure_text(phase: Phase, diags: &[Diagnostic]) -> String {
    let d = diags.first();
    let span = d
        .and_then(|d| d.span)
        .map(|s| format!("{:?}", s.0))
        .unwrap_or_default();
    format!(
        "{{\"phase\":\"{phase}\",\"code\":\"{}\",\"span\":{span}}}",
        d.map(|d| d.code).unwrap_or("")
    )
}

/// The law `tree` (7.1 item 4).
fn tree_law(tc: &ToolCase, s: &str) -> Option<String> {
    match tc.kind {
        ToolKind::FormatTemplate => {
            let a = crate::template::parse_template(&tc.text).ok()?;
            let b = match crate::template::parse_template(s) {
                Ok(b) => b,
                Err(_) => return Some("the formatted template does not parse".into()),
            };
            if crate::format::template_canon(&a) != crate::format::template_canon(&b) {
                return Some("the template's segments or parts differ".into());
            }
            None
        }
        _ => {
            let a = crate::syntax::parse(&tc.text).ok()?;
            let b = match crate::syntax::parse(s) {
                Ok(b) => b,
                Err(_) => return Some("the formatted text does not parse".into()),
            };
            let ca = crate::format::canon(&a.ast, a.ast.root, a.source.chars());
            let cb = crate::format::canon(&b.ast, b.ast.root, b.source.chars());
            (ca != cb).then(|| format!("{cb} where the source has {ca}"))
        }
    }
}

/// The outcome of a text, checked and run with the case's environment and
/// `vars`, with every span taken out.
fn run_outcome(case: &Case, text: &str) -> String {
    let mut c = case.clone();
    c.tool = None;
    c.source = text.to_string();
    let line = outcome_line(&c);
    let mut j = json::parse(&line).unwrap_or(Json::Null);
    strip(&mut j);
    form(&j)
}

fn strip(j: &mut Json) {
    match j {
        Json::Obj(m) => {
            m.retain(|(k, _)| k != "span" && k != "name");
            for (_, v) in m {
                strip(v);
            }
        }
        Json::Arr(a) => {
            for v in a {
                strip(v);
            }
        }
        _ => {}
    }
}

fn run_assist(case: &Case, tc: &ToolCase) -> Verdict {
    let found = assist_result(case, tc);
    let mut diffs = Vec::new();
    match (found, &case.expect) {
        (Err(e), Expect::Failure(f)) => {
            diffs.extend(compare_failure_pub(case, f, e.phase, &e.diagnostics));
        }
        (Err(e), _) => diffs.push(outcome_diff(case, failure_text(e.phase, &e.diagnostics))),
        (Ok(j), Expect::Failure(_)) => diffs.push(outcome_diff(case, form(&j))),
        (Ok(j), _) => {
            let want = tc.want.clone().unwrap_or(Json::Null);
            let same = match tc.kind {
                ToolKind::Complete => same_completion(&want, &j),
                _ => same_hover(&want, &j),
            };
            if !same {
                diffs.push(Difference {
                    name: if tc.kind == ToolKind::Complete {
                        "completion"
                    } else {
                        "hover"
                    },
                    expected: form(&want),
                    found: form(&j),
                });
            }
        }
    }
    if diffs.is_empty() {
        Verdict::Pass
    } else {
        Verdict::Fail(diffs)
    }
}

fn same_completion(want: &Json, found: &Json) -> bool {
    if !json::same_data(
        want.get("span").unwrap_or(&Json::Null),
        found.get("span").unwrap_or(&Json::Null),
    ) {
        return false;
    }
    let (Some(w), Some(f)) = (
        want.get("items").and_then(|i| i.as_arr()),
        found.get("items").and_then(|i| i.as_arr()),
    ) else {
        return false;
    };
    w.len() == f.len()
        && w.iter().zip(f).all(|(a, b)| {
            a.as_obj()
                .unwrap_or_default()
                .iter()
                .all(|(k, v)| b.get(k).is_some_and(|x| json::same_data(v, x)))
        })
}

fn same_hover(want: &Json, found: &Json) -> bool {
    match (want, found) {
        (Json::Null, Json::Null) => true,
        (Json::Obj(w), Json::Obj(_)) => w
            .iter()
            .all(|(k, v)| found.get(k).is_some_and(|x| json::same_data(v, x))),
        _ => false,
    }
}

/// The line of the `outcome` command for a case of F12 (7.7).
pub(super) fn outcome(case: &Case, tc: &ToolCase) -> String {
    let mut m: Vec<(String, Json)> = vec![("name".into(), Json::Str(case.name.clone()))];
    let fail = |m: &mut Vec<(String, Json)>, phase: Phase, ds: &[Diagnostic]| {
        m.push(("phase".into(), Json::Str(phase.to_string())));
        let mut ds: Vec<&Diagnostic> = if phase == Phase::Parse {
            ds.iter().take(1).collect()
        } else {
            ds.iter().collect()
        };
        ds.sort_by_key(|d| if phase == Phase::Declare { d.code } else { "" });
        let items = ds
            .iter()
            .map(|d| {
                let mut o = vec![("code".to_string(), Json::Str(d.code.into()))];
                if phase != Phase::Declare {
                    o.push((
                        "span".into(),
                        match d.span {
                            Some(s) => {
                                Json::Arr(s.0.iter().map(|&x| Json::Num(x as f64)).collect())
                            }
                            None => Json::Null,
                        },
                    ));
                }
                Json::Obj(o)
            })
            .collect();
        m.push(("diagnostics".into(), Json::Arr(items)));
    };
    match tc.kind {
        ToolKind::Format | ToolKind::FormatTemplate => match format_text(tc, &tc.text) {
            Ok(s) => m.push(("formatted".into(), Json::Str(s))),
            Err(e) => fail(&mut m, e.phase, &e.diagnostics),
        },
        _ => match assist_result(case, tc) {
            Ok(j) => m.push((
                if tc.kind == ToolKind::Complete {
                    "completion"
                } else {
                    "hover"
                }
                .into(),
                j,
            )),
            Err(e) => fail(&mut m, e.phase, &e.diagnostics),
        },
    }
    json::to_text(&Json::Obj(m))
}
