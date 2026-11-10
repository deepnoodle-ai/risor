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
        Catalog {
            entries,
            cases,
            query_cases,
            parse_cases,
            check_cases,
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
