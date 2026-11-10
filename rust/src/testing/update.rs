//! Package F9: update mode (`f09-testing.md` 4). A runner writes the outcome
//! of a case into its `expect`, and keeps every other code point of the file.

use super::case::Case;
use super::run::{
    capture, case_lines, failure_json, form, path_json, result_json, run_case, whole_columns,
    Captured, Difference, Verdict,
};
use crate::diag::{code, Phase};
use crate::json::Json;

/// The shape of an `expect` (4.4): a result, a failure of a phase, or a
/// conform result.
#[derive(PartialEq)]
enum Shape {
    Result,
    Failure(String),
    Conform,
}

fn shape(e: &Json) -> Shape {
    if let Some(p) = e.get("phase").and_then(|p| p.as_str()) {
        Shape::Failure(p.to_string())
    } else if e.get("conforms").is_some() {
        Shape::Conform
    } else {
        Shape::Result
    }
}

fn keys(e: &Json) -> Vec<String> {
    e.as_obj()
        .map(|m| m.iter().map(|(k, _)| k.clone()).collect())
        .unwrap_or_default()
}

/// The depth of a JSON value, the value itself at level 1.
fn depth(j: &Json) -> usize {
    match j {
        Json::Arr(a) => 1 + a.iter().map(depth).max().unwrap_or(0),
        Json::Obj(m) => 1 + m.iter().map(|(_, x)| depth(x)).max().unwrap_or(0),
        _ => 1,
    }
}

/// The new `expect` of a case (4.4), or `None` when 4.3 does not write it.
fn new_expect(case: &Case, cap: &Captured) -> Option<Json> {
    let old = case.expect_raw.as_ref();
    let same = |s: Shape| old.is_some_and(|o| shape(o) == s);
    let obj =
        |m: Vec<(&str, Json)>| Json::Obj(m.into_iter().map(|(k, v)| (k.to_string(), v)).collect());
    match cap {
        Captured::Failure {
            phase,
            diags,
            usage,
        } => {
            if *phase == Phase::Declare
                && (!same(Shape::Failure("declare".into())) || diags.len() > 1)
            {
                return None;
            }
            let limit = diags.iter().any(|d| code::LIMIT.contains(&d.code));
            if !same(Shape::Failure(phase.as_str().into())) {
                return Some(failure_json(*phase, diags, None));
            }
            let old = old?;
            // The columns: `code` and every field some old diagnostic has,
            // among those the phase may write.
            let may: &[&str] = match phase {
                Phase::Run => &["code", "class", "span", "path", "message", "data", "trace"],
                _ => whole_columns(*phase),
            };
            let mut cols: Vec<String> = vec!["code".into()];
            for d in old
                .get("diagnostics")
                .and_then(|d| d.as_arr())
                .unwrap_or_default()
            {
                for k in keys(d) {
                    if may.contains(&k.as_str()) && !cols.contains(&k) {
                        cols.push(k);
                    }
                }
            }
            let mut out = failure_json(*phase, diags, Some(&cols));
            let Json::Obj(m) = &mut out else {
                return None;
            };
            if let (Some((steps, units)), false) = (usage, limit) {
                if old.get("steps").is_some() {
                    m.push(("steps".into(), Json::Num(*steps as f64)));
                }
                if old.get("allocated").is_some() {
                    m.push(("allocated".into(), Json::Num(*units as f64)));
                }
            }
            if let (Some(r), false) = (old.get("rendered"), limit) {
                let ds = match phase {
                    Phase::Parse => &diags[..diags.len().min(1)],
                    _ => &diags[..],
                };
                let views: Vec<crate::render::View> =
                    ds.iter().map(crate::render::View::of).collect();
                let mut forms = Vec::new();
                for k in keys(r) {
                    let t = match k.as_str() {
                        "line" => crate::render::line(&views),
                        "detail" => crate::render::detail(&views, Some(&case.source)),
                        "json" => crate::render::json(&views),
                        _ => continue,
                    };
                    forms.push((k, Json::Str(t)));
                }
                m.push(("rendered".into(), Json::Obj(forms)));
            }
            Some(out)
        }
        Captured::Result {
            ty,
            sites,
            description,
            ran,
        } => {
            if ran.as_ref().is_some_and(|r| r.opaque_key) {
                return None;
            }
            if !same(Shape::Result) {
                let r = ran.as_ref().map(|r| (r.value.clone(), r.steps, r.units));
                return Some(result_json(ty.as_deref(), r));
            }
            let mut m = Vec::new();
            for k in keys(old?) {
                let v = match k.as_str() {
                    "type" => ty.clone().map(Json::Str),
                    "sites" => sites.clone(),
                    "value" => ran.as_ref().map(|r| r.value.clone()),
                    "truthy" => ran.as_ref().map(|r| Json::Bool(r.truthy)),
                    "steps" => ran.as_ref().map(|r| Json::Num(r.steps as f64)),
                    "allocated" => ran.as_ref().map(|r| Json::Num(r.units as f64)),
                    "description" => description.clone(),
                    "json" => ran.as_ref().and_then(|r| r.json.clone()).map(Json::Str),
                    "analysis" => {
                        super::run::analysis_text(case).and_then(|t| crate::json::parse(&t).ok())
                    }
                    _ => None,
                };
                if let Some(v) = v {
                    m.push((k, v));
                }
            }
            Some(Json::Obj(m))
        }
        Captured::Conform(c) => {
            if !c.agree() {
                return None;
            }
            let conforms = c.function.is_none();
            let path = c
                .function
                .as_ref()
                .map(|d| path_json(d.path.as_deref().map(|p| p.as_slice()).unwrap_or(&[])));
            let message = c.function.as_ref().map(|d| Json::Str(d.message.clone()));
            let failures = c.all.as_ref().map(|all| {
                Json::Arr(
                    all.iter()
                        .map(|d| {
                            obj(vec![
                                (
                                    "path",
                                    path_json(
                                        d.path.as_deref().map(|p| p.as_slice()).unwrap_or(&[]),
                                    ),
                                ),
                                ("message", Json::Str(d.message.clone())),
                            ])
                        })
                        .collect(),
                )
            });
            let steps = Json::Num(c.as_steps as f64);
            let mut m = vec![("conforms", Json::Bool(conforms))];
            if !same(Shape::Conform) {
                if !conforms {
                    m.push(("path", path.unwrap_or(Json::Null)));
                    m.push(("message", message.unwrap_or(Json::Null)));
                }
                m.push(("steps", steps));
                return Some(obj(m));
            }
            let old = old?;
            if !conforms && old.get("path").is_some() {
                m.push(("path", path.unwrap_or(Json::Null)));
            }
            if !conforms && old.get("message").is_some() {
                m.push(("message", message.unwrap_or(Json::Null)));
            }
            if old.get("steps").is_some() {
                m.push(("steps", steps));
            }
            if let (Some(f), true) = (failures, old.get("failures").is_some()) {
                m.push(("failures", f));
            }
            Some(obj(m))
        }
    }
}

/// The outcome written whole, for the difference `outcome` of a case with
/// no `expect`.
fn whole(cap: &Captured) -> Json {
    match cap {
        Captured::Failure { phase, diags, .. } => failure_json(*phase, diags, None),
        Captured::Result { ty, ran, .. } => result_json(
            ty.as_deref(),
            ran.as_ref().map(|r| (r.value.clone(), r.steps, r.units)),
        ),
        Captured::Conform(c) => {
            Json::Obj(vec![("conforms".into(), Json::Bool(c.function.is_none()))])
        }
    }
}

/// One case in update mode (4.3): its status, and the `expect` to write.
fn update_case(case: &Case) -> (Verdict, Option<Json>) {
    let v = run_case(case);
    if case.runner.is_some() || case.render.is_some() {
        return (v, None);
    }
    match &v {
        Verdict::Skip(_) | Verdict::Fixture(..) | Verdict::Invalid(_) => return (v, None),
        Verdict::Pass if case.expect_raw.is_some() => return (v, None),
        Verdict::Fail(ds) if ds.iter().any(|d| d.name == "functions") => return (v, None),
        _ => {}
    }
    let mut cap = capture(case, false);
    let old_result = case
        .expect_raw
        .as_ref()
        .is_some_and(|e| shape(e) == Shape::Result);
    if matches!(cap, Captured::Result { .. }) && !old_result {
        cap = capture(case, true);
    }
    match new_expect(case, &cap) {
        Some(e) => (Verdict::Updated, Some(e)),
        None => {
            let v = match v {
                Verdict::Fail(ds) => Verdict::Fail(ds),
                _ => Verdict::Fail(vec![Difference {
                    name: "outcome",
                    expected: "nothing".into(),
                    found: form(&whole(&cap)),
                }]),
            };
            (v, None)
        }
    }
}

/// The positions in a case file's text that update mode needs (4.5): for
/// each case object, the byte span of the value of its `expect`, or the end
/// of the value of its last member.
struct Spot {
    expect: Option<(usize, usize)>,
    last_end: usize,
    /// The nesting level of the case object, the top value at level 1.
    level: usize,
}

struct Scan<'a> {
    t: &'a [u8],
    i: usize,
}

impl Scan<'_> {
    fn ws(&mut self) {
        while self.i < self.t.len() && matches!(self.t[self.i], b' ' | b'\t' | b'\n' | b'\r') {
            self.i += 1;
        }
    }

    /// Skip one value; its byte span.
    fn value(&mut self) -> Option<(usize, usize)> {
        self.ws();
        let start = self.i;
        match *self.t.get(self.i)? {
            b'{' | b'[' => {
                let mut depth = 0usize;
                while self.i < self.t.len() {
                    match self.t[self.i] {
                        b'"' => {
                            self.string()?;
                            continue;
                        }
                        b'{' | b'[' => depth += 1,
                        b'}' | b']' => {
                            depth -= 1;
                            if depth == 0 {
                                self.i += 1;
                                return Some((start, self.i));
                            }
                        }
                        _ => {}
                    }
                    self.i += 1;
                }
                None
            }
            b'"' => {
                self.string()?;
                Some((start, self.i))
            }
            _ => {
                while self.i < self.t.len()
                    && !matches!(
                        self.t[self.i],
                        b',' | b'}' | b']' | b' ' | b'\t' | b'\n' | b'\r'
                    )
                {
                    self.i += 1;
                }
                Some((start, self.i))
            }
        }
    }

    fn string(&mut self) -> Option<(usize, usize)> {
        let start = self.i;
        self.i += 1;
        while self.i < self.t.len() {
            match self.t[self.i] {
                b'\\' => self.i += 2,
                b'"' => {
                    self.i += 1;
                    return Some((start, self.i));
                }
                _ => self.i += 1,
            }
        }
        None
    }

    /// The members of the object at the position: each key and the span of
    /// its value.
    fn members(&mut self) -> Option<Vec<(String, (usize, usize))>> {
        self.ws();
        if self.t.get(self.i) != Some(&b'{') {
            return None;
        }
        self.i += 1;
        let mut out = Vec::new();
        loop {
            self.ws();
            match self.t.get(self.i)? {
                b'}' => {
                    self.i += 1;
                    return Some(out);
                }
                b',' => {
                    self.i += 1;
                    continue;
                }
                _ => {}
            }
            let (ks, ke) = self.string()?;
            let key = std::str::from_utf8(&self.t[ks..ke]).ok()?;
            let key = match crate::json::parse(key).ok()? {
                Json::Str(k) => k,
                _ => return None,
            };
            self.ws();
            self.i += 1; // the colon
            let span = self.value()?;
            out.push((key, span));
        }
    }

    /// The elements of the array at the position, each a byte offset.
    fn elements(&mut self) -> Option<Vec<usize>> {
        self.ws();
        if self.t.get(self.i) != Some(&b'[') {
            return None;
        }
        self.i += 1;
        let mut out = Vec::new();
        loop {
            self.ws();
            match self.t.get(self.i)? {
                b']' => return Some(out),
                b',' => {
                    self.i += 1;
                    continue;
                }
                _ => {}
            }
            out.push(self.i);
            self.value()?;
        }
    }
}

fn spots(text: &str) -> Option<Vec<Spot>> {
    let t = text.as_bytes();
    let mut s = Scan { t, i: 0 };
    s.ws();
    let (starts, level) = if t.get(s.i) == Some(&b'[') {
        (s.elements()?, 2)
    } else {
        let ms = s.members()?;
        let (_, (cs, _)) = ms.into_iter().find(|(k, _)| k == "cases")?;
        s.i = cs;
        (s.elements()?, 3)
    };
    let mut out = Vec::new();
    for st in starts {
        s.i = st;
        let ms = s.members()?;
        out.push(Spot {
            expect: ms.iter().find(|(k, _)| k == "expect").map(|(_, sp)| *sp),
            last_end: ms.last().map(|(_, (_, e))| *e).unwrap_or(st + 1),
            level,
        });
    }
    Some(out)
}

/// Run the cases of a file in update mode (4.2): each case's update status,
/// and the new text of the file when at least one case is written.
pub fn update_text(text: &str, cases: &[Case]) -> (Vec<(String, Verdict)>, Option<String>) {
    let places = spots(text);
    let mut results = Vec::new();
    let mut edits: Vec<(usize, usize, String)> = Vec::new();
    for (i, case) in cases.iter().enumerate() {
        let (mut v, e) = update_case(case);
        if let Some(e) = e {
            let spot = places.as_ref().and_then(|p| p.get(i));
            // 4.3: a new `expect` that would nest the file too deep.
            match spot {
                Some(sp) if sp.level + depth(&e) <= crate::json::MAX_DEPTH => {
                    let f = form(&e);
                    match sp.expect {
                        Some((a, b)) => edits.push((a, b, f)),
                        None => {
                            edits.push((sp.last_end, sp.last_end, format!(", \"expect\": {f}")))
                        }
                    }
                }
                _ => {
                    v = Verdict::Fail(vec![Difference {
                        name: "outcome",
                        expected: match &case.expect_raw {
                            Some(x) => form(x),
                            None => "nothing".into(),
                        },
                        found: form(&e),
                    }]);
                }
            }
        }
        results.push((case.name.clone(), v));
    }
    if edits.is_empty() {
        return (results, None);
    }
    let mut out = text.to_string();
    for (a, b, f) in edits.into_iter().rev() {
        out.replace_range(a..b, &f);
    }
    (results, Some(out))
}

/// The report lines of an update (4.8).
pub fn update_report(results: &[(String, Verdict)]) -> String {
    results.iter().map(|(n, v)| case_lines(n, v)).collect()
}
