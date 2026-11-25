//! The message catalog (`03-host.md` 3.8).
//!
//! The table is `suite/messages.json` itself, embedded at build time with
//! `include_str!` and read once. No message is written by hand here, and the
//! table cannot fall out of date: a change to the file is a change to the
//! build.

use crate::json::{self, Json};
use std::sync::OnceLock;

/// The catalog file, as built into the crate.
pub const SOURCE: &str = include_str!("../../suite/messages.json");

pub struct Entry {
    pub code: String,
    pub phase: String,
    pub caught: bool,
    pub params: Vec<(String, String)>,
    pub templates: Vec<(String, String)>,
}

#[cfg_attr(not(test), allow(dead_code))]
pub struct Catalog {
    pub entries: Vec<Entry>,
    /// The cases of each entry, as written.
    pub cases: Vec<(String, Json)>,
    /// F7: the cases of `suite/messages-queries.json` (`f07-queries.md` 11).
    pub query_cases: Vec<(String, Json)>,
    /// F8: the cases of `suite/messages-parse.json` and
    /// `suite/messages-check.json` (`f08-errors.md` 3.1).
    pub parse_cases: Vec<(String, Json)>,
    pub check_cases: Vec<(String, Json)>,
    /// F16: the cases of `suite/messages-entry-points.json`
    /// (`f16-entry-points.md` 9.1).
    pub entry_point_cases: Vec<(String, Json)>,
    /// F17: the cases of `suite/messages-recursion.json` (`f17-recursion.md`
    /// 9 and 13.5).
    pub recursion_cases: Vec<(String, Json)>,
    /// F19: the cases of `suite/messages-feedback.json`
    /// (`f19-author-feedback.md` 7).
    pub feedback_cases: Vec<(String, Json)>,
}

/// Package F8: the catalog files of the `parse` and `check` phases
/// (`f08-errors.md` 3.1).
pub const PARSE_SOURCE: &str = include_str!("../../suite/messages-parse.json");
pub const CHECK_SOURCE: &str = include_str!("../../suite/messages-check.json");

fn read_entry(e: &Json) -> Entry {
    let s = |k: &str| {
        e.get(k)
            .and_then(|v| v.as_str())
            .unwrap_or_default()
            .to_string()
    };
    let pairs = |k: &str| -> Vec<(String, String)> {
        e.get(k)
            .and_then(|v| v.as_obj())
            .map(|m| {
                m.iter()
                    .map(|(a, b)| (a.clone(), b.as_str().unwrap_or_default().to_string()))
                    .collect()
            })
            .unwrap_or_default()
    };
    Entry {
        phase: s("phase"),
        caught: matches!(e.get("caught"), Some(Json::Bool(true))),
        params: pairs("params"),
        templates: pairs("templates"),
        code: s("code"),
    }
}

/// Package F7: the catalog file of its three templates (`f07-queries.md` 11).
pub const QUERIES_SOURCE: &str = include_str!("../../suite/messages-queries.json");

/// Package F16: the catalog file of `name-clash` (`f16-entry-points.md`
/// 9.1).
pub const ENTRY_POINTS_SOURCE: &str = include_str!("../../suite/messages-entry-points.json");

/// Package F17: the catalog file of `circular-type`, a template of
/// `check-limit`, and five templates of existing codes (`f17-recursion.md` 9
/// and 13.5).
pub const RECURSION_SOURCE: &str = include_str!("../../suite/messages-recursion.json");

/// Package F19: the catalog file of the hint templates, the template
/// `pattern` of `maybe-absent`, and two `parse` codes
/// (`f19-author-feedback.md` 7).
pub const FEEDBACK_SOURCE: &str = include_str!("../../suite/messages-feedback.json");

pub fn catalog() -> &'static Catalog {
    static C: OnceLock<Catalog> = OnceLock::new();
    C.get_or_init(|| {
        let doc = json::parse(SOURCE).expect("suite/messages.json is valid JSON");
        let mut entries = Vec::new();
        let mut cases = Vec::new();
        for e in doc.as_arr().unwrap_or_default() {
            let entry = read_entry(e);
            if let Some(c) = e.get("cases") {
                cases.push((entry.code.clone(), c.clone()));
            }
            entries.push(entry);
        }
        // F7 11 item 2: an entry of the second file adds templates to the
        // entry of its code.
        let q = json::parse(QUERIES_SOURCE).expect("suite/messages-queries.json is valid JSON");
        let mut query_cases = Vec::new();
        for e in q.as_arr().unwrap_or_default() {
            let code = e.get("code").and_then(|v| v.as_str()).unwrap_or_default();
            if let Some(c) = e.get("cases") {
                query_cases.push((code.to_string(), c.clone()));
            }
            if let Some(entry) = entries.iter_mut().find(|x| x.code == code) {
                for (k, v) in e
                    .get("templates")
                    .and_then(|t| t.as_obj())
                    .unwrap_or_default()
                {
                    entry
                        .templates
                        .push((k.clone(), v.as_str().unwrap_or_default().to_string()));
                }
            }
        }
        // F8 3.1 items 6 and 7: each entry of the two files is the entry of
        // a code no other file has.
        let mut more = |text: &str, out: &mut Vec<(String, Json)>| {
            let d = json::parse(text).expect("a catalog file is valid JSON");
            for e in d.as_arr().unwrap_or_default() {
                let entry = read_entry(e);
                if let Some(c) = e.get("cases") {
                    out.push((entry.code.clone(), c.clone()));
                }
                entries.push(entry);
            }
        };
        let mut parse_cases = Vec::new();
        let mut check_cases = Vec::new();
        more(PARSE_SOURCE, &mut parse_cases);
        more(CHECK_SOURCE, &mut check_cases);
        // F16 9.1 and F17 13.5: an entry of a code another file has adds
        // templates to it; an entry of a code no other file has is added
        // whole.
        let mut added = |text: &str, out: &mut Vec<(String, Json)>| {
            let d = json::parse(text).expect("a catalog file is valid JSON");
            for e in d.as_arr().unwrap_or_default() {
                let entry = read_entry(e);
                if let Some(c) = e.get("cases") {
                    out.push((entry.code.clone(), c.clone()));
                }
                match entries.iter_mut().find(|x| x.code == entry.code) {
                    Some(x) => {
                        x.templates.extend(entry.templates);
                        // F19 3.5: a template may take a parameter the
                        // entry did not have, `fix`.
                        for p in entry.params {
                            if !x.params.iter().any(|(n, _)| *n == p.0) {
                                x.params.push(p);
                            }
                        }
                    }
                    None => entries.push(entry),
                }
            }
        };
        let mut entry_point_cases = Vec::new();
        added(ENTRY_POINTS_SOURCE, &mut entry_point_cases);
        let mut recursion_cases = Vec::new();
        added(RECURSION_SOURCE, &mut recursion_cases);
        let mut feedback_cases = Vec::new();
        added(FEEDBACK_SOURCE, &mut feedback_cases);
        Catalog {
            entries,
            cases,
            query_cases,
            parse_cases,
            check_cases,
            entry_point_cases,
            recursion_cases,
            feedback_cases,
        }
    })
}

fn entry(code: &str) -> Option<&'static Entry> {
    catalog().entries.iter().find(|e| e.code == code)
}

/// Whether `catch` catches the code (`caught`).
pub fn caught(code: &str) -> bool {
    entry(code).is_some_and(|e| e.caught)
}

/// A rendering longer than 64 code points is cut to 61 and `...` (3.8.3).
pub fn cut(s: &str) -> String {
    if s.chars().count() > 64 {
        let mut t: String = s.chars().take(61).collect();
        t.push_str("...");
        t
    } else {
        s.to_string()
    }
}

/// Visible text (`f08-errors.md` 1.1): each control character as an escape.
pub fn visible(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '\t' => out.push_str("\\t"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            c if (c as u32) < 0x20 || (0x7f..=0x9f).contains(&(c as u32)) => {
                out.push_str(&format!("\\u00{:02x}", c as u32))
            }
            c => out.push(c),
        }
    }
    out
}

/// A parameter of kind `char` (`f08-errors.md` 3.2).
pub fn char_param(c: char) -> String {
    let n = c as u32;
    if (0x21..=0x7e).contains(&n) {
        format!("U+{n:04X} ({c})")
    } else {
        format!("U+{n:04X}")
    }
}

/// Fill a template of a code with rendered parameters. An entry with one
/// template names it `default`.
pub fn build(code: &str, template: &str, params: &[(&str, String)]) -> String {
    let Some(e) = entry(code) else {
        return code.to_string();
    };
    let text = e
        .templates
        .iter()
        .find(|(n, _)| n == template)
        .or_else(|| e.templates.first())
        .map(|(_, t)| t.as_str())
        .unwrap_or(code);
    let mut out = String::new();
    let mut rest = text;
    while let Some(i) = rest.find('{') {
        out.push_str(&rest[..i]);
        let after = &rest[i + 1..];
        match after.find('}') {
            Some(j) => {
                let name = &after[..j];
                match params.iter().find(|(n, _)| *n == name) {
                    // F19 3.1: a parameter of kind `fix` is never cut.
                    Some((_, v)) if e.params.iter().any(|(n, k)| n == name && k == "fix") => {
                        out.push_str(v)
                    }
                    Some((_, v)) => out.push_str(&cut(v)),
                    None => {
                        out.push('{');
                        out.push_str(name);
                        out.push('}');
                    }
                }
                rest = &after[j + 1..];
            }
            None => {
                out.push_str(&rest[i..]);
                rest = "";
            }
        }
    }
    out.push_str(rest);
    // `03-host.md` 3.8: every hole of a template is filled. A hole left open is a
    // fault at the place that built the message.
    debug_assert!(
        !e.params
            .iter()
            .any(|(n, _)| out.contains(&format!("{{{n}}}"))),
        "message of {code} ({template}) left a parameter unfilled: {out}"
    );
    out
}

/// A string in its JSON form, built only as far as the cut needs.
pub fn json_string(s: &str) -> String {
    let mut out = String::from("\"");
    let mut n = 1;
    for c in s.chars() {
        if n > 70 {
            break;
        }
        push_json_char(&mut out, c);
        n += 1;
    }
    if n <= 70 {
        out.push('"');
    }
    out
}

/// One code point of a string in its JSON form (`01-language.md` 12).
pub fn push_json_char(out: &mut String, c: char) {
    match c {
        '"' => out.push_str("\\\""),
        '\\' => out.push_str("\\\\"),
        '\u{8}' => out.push_str("\\b"),
        '\u{c}' => out.push_str("\\f"),
        '\n' => out.push_str("\\n"),
        '\r' => out.push_str("\\r"),
        '\t' => out.push_str("\\t"),
        c if (c as u32) < 0x20 => out.push_str(&format!("\\u00{:02x}", c as u32)),
        c => out.push(c),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_catalog_has_19_run_codes_and_64_templates() {
        let c = catalog();
        let run: Vec<_> = c.entries.iter().filter(|e| e.phase == "run").collect();
        assert_eq!(run.len(), 19);
        let templates: usize = run.iter().map(|e| e.templates.len().max(1)).sum::<usize>()
            - run.iter().filter(|e| e.templates.is_empty()).count();
        assert!(templates >= 64);
        assert!(caught("divide-by-zero"));
        assert!(!caught("steps-exceeded"));
    }

    /// F8 3.1, F16 9.1 and F17 9 and 13.5: `messages-check.json` has 30 codes
    /// and 72 templates, `messages-parse.json` 60 codes and 99 templates;
    /// F16's file adds the code `name-clash`; F17's adds the code
    /// `circular-type`, with two templates, a template of `check-limit`, and
    /// five templates of four existing codes: 32 check codes and 81 templates.
    /// F19 7 item 1: 62 parse codes with 103 templates, 32 check codes with
    /// 86 templates, and 113 codes in all.
    #[test]
    fn the_feedback_catalog_adds_two_parse_codes_and_nine_templates() {
        let c = catalog();
        let of = |ph: &str| -> (usize, usize) {
            let es: Vec<_> = c.entries.iter().filter(|e| e.phase == ph).collect();
            (es.len(), es.iter().map(|e| e.templates.len()).sum())
        };
        assert_eq!(of("parse"), (62, 103));
        assert_eq!(of("check"), (33, 90));
        assert_eq!(
            c.entries
                .iter()
                .filter(|e| matches!(e.phase.as_str(), "run" | "parse" | "check"))
                .count(),
            114
        );
        let fix = "x".repeat(100);
        let m = build(
            "undefined-name",
            "hint",
            &[("name", "Math".into()), ("fix", fix.clone())],
        );
        assert_eq!(m, format!("Math is not defined; {fix}"));
    }

    #[test]
    fn the_check_catalog_has_33_codes_and_90_templates() {
        let count = |text: &str| {
            let d = json::parse(text).expect("valid JSON");
            let es = d.as_arr().unwrap_or_default().to_vec();
            let t: usize = es
                .iter()
                .map(|e| {
                    e.get("templates")
                        .and_then(|t| t.as_obj())
                        .map_or(0, |m| m.len())
                })
                .sum();
            (es.len(), t)
        };
        assert_eq!(count(CHECK_SOURCE), (30, 72));
        assert_eq!(count(PARSE_SOURCE), (60, 99));
        assert_eq!(count(ENTRY_POINTS_SOURCE), (2, 5));
        assert_eq!(count(RECURSION_SOURCE), (6, 8));
        let c = catalog();
        let check: Vec<_> = c.entries.iter().filter(|e| e.phase == "check").collect();
        assert_eq!(check.len(), 33);
        // F19 adds five templates (`f19-author-feedback.md` 7 item 1);
        // F16 slice D2 adds `bad-export` with four (`f16-entry-points.md` 12.9).
        assert_eq!(check.iter().map(|e| e.templates.len()).sum::<usize>(), 90);
        assert_eq!(
            build("circular-type", "direct", &[("name", "Loop".into())]),
            "Loop refers to itself outside any list, map or record"
        );
        assert_eq!(
            build("call-of-parameter", "fn-hint", &[("name", "f".into())]),
            "f is a parameter; to call it, give it a function type, as f: (x: int) => int"
        );
        assert_eq!(
            build("lambda-to-lambda", "fn-hint", &[("name", "v".into())]),
            "v is a parameter; to pass a function to it, give it a function type, as v: (x: int) => int"
        );
        assert_eq!(
            build("lambda-to-lambda", "default", &[("name", "ap".into())]),
            "a function cannot be passed to ap at a parameter that takes a value"
        );
        assert!(entry("recursion-needs-types").is_some());
    }

    #[test]
    fn templates_fill_and_cut() {
        assert_eq!(
            build("divide-by-zero", "divisor", &[("op", "/".into())]),
            "the divisor of / is zero"
        );
        let long = "x".repeat(100);
        let m = build("missing-variable", "default", &[("name", long)]);
        assert!(m.ends_with("..."));
    }
}
