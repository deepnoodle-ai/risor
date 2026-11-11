//! The library's records (`04-library.md`, `suite/library.json`).
//!
//! The records give each entry's name, receiver and signature, from which
//! come the counts of the static rules and the shape test of a method call.
//! The entries themselves run in `eval::lib`.

use crate::json::{self, Json};
use crate::types::text::{parse_sig, parse_type_vars, ParamType, Sig, Type};
use crate::value::Kind;
use std::sync::OnceLock;

/// The library file, as built into the crate.
pub const SOURCE: &str = include_str!("../../suite/library.json");

/// Package F6: the records of the opt-in groups (`f06-groups.md` 3).
pub const GROUPS_SOURCE: &str = include_str!("../../suite/library-groups.json");

/// Package F13: the records of the math functions (`f13-math.md` 5).
pub const MATH_SOURCE: &str = include_str!("../../suite/library-math.json");

/// Package F7: the records of the queries (`f07-queries.md` 6.1).
pub const QUERIES_SOURCE: &str = include_str!("../../suite/library-queries.json");

/// The four opt-in groups (`f06-groups.md` 1.1).
pub const GROUPS: [&str; 4] = ["json", "time", "math", "base64"];

/// A constant of a group (`f06-groups.md` 3.3): `math.pi`.
pub struct Constant {
    pub name: String,
    pub group: String,
    pub value: f64,
    #[cfg_attr(not(test), allow(dead_code))]
    pub cases: Option<Json>,
    /// F12: the record's type text and `doc`.
    pub ty: String,
    pub doc: Option<String>,
}

pub struct Record {
    pub name: String,
    /// F12: the `signature` text of the record, verbatim, and its `doc`.
    pub signature: String,
    pub doc: Option<String>,
    /// The kind of the receiver of a method; `None` for a function.
    pub receiver: Option<Kind>,
    pub sig: Sig,
    /// The receiver as type text, for a method.
    pub recv: Option<Type>,
    /// The entry narrows through its callback (`05-types.md` 13.8).
    pub narrows: bool,
    /// The argument positions that are tested positions (`05-types.md` 10.1).
    pub tested: Vec<usize>,
    /// The parameters whose argument is rendered (`05-types.md` 7.3).
    pub renders: Vec<String>,
    /// The opt-in group of the entry, if any (`f06-groups.md` 3.2).
    pub group: Option<String>,
    /// F7: the record extends a Core entry (`f07-queries.md` 6.1 item 3).
    pub extends: bool,
    /// F7: a record of `suite/library-queries.json` that is a new entry.
    pub queries: bool,
    /// F7: the cases of a record that extends a Core entry, beside the Core
    /// record's own.
    pub ext_cases: Option<Json>,
    /// The record's formulas, as written.
    #[cfg_attr(not(test), allow(dead_code))]
    pub steps: String,
    #[cfg_attr(not(test), allow(dead_code))]
    pub units: String,
    /// The cases of the record, as written.
    #[cfg_attr(not(test), allow(dead_code))]
    pub cases: Option<Json>,
}

impl Record {
    /// For each parameter: `Some(n)` when it is a callback the entry calls
    /// with `n` arguments.
    pub fn callback(&self, i: usize) -> Option<usize> {
        match self.sig.param_at(i).map(|p| &p.ty) {
            Some(ParamType::Fn(f)) => Some(f.params.len()),
            _ => None,
        }
    }
}

pub fn records() -> &'static [Record] {
    static R: OnceLock<Vec<Record>> = OnceLock::new();
    R.get_or_init(|| {
        let doc = json::parse(SOURCE).expect("suite/library.json is valid JSON");
        let groups = json::parse(GROUPS_SOURCE).expect("suite/library-groups.json is valid JSON");
        let queries =
            json::parse(QUERIES_SOURCE).expect("suite/library-queries.json is valid JSON");
        let math = json::parse(MATH_SOURCE).expect("suite/library-math.json is valid JSON");
        let mut out: Vec<Record> = doc
            .as_arr()
            .unwrap_or_default()
            .iter()
            .chain(
                groups
                    .as_arr()
                    .unwrap_or_default()
                    .iter()
                    .filter(|e| e.get("signature").is_some()),
            )
            .chain(math.as_arr().unwrap_or_default().iter())
            .map(record)
            .collect();
        // F7 6.1: a record that `extends` a Core entry takes its place; the
        // Core record's cases stay cases of the entry, with its own added.
        for e in queries.as_arr().unwrap_or_default() {
            let mut r = record(e);
            if e.get("extends").is_some() {
                if let Some(old) = out
                    .iter_mut()
                    .find(|o| o.name == r.name && o.receiver == r.receiver)
                {
                    // The Core record's cases stay its own; the extension's
                    // are kept apart, for the runner of the new file.
                    r.ext_cases = r.cases.take();
                    r.cases = old.cases.take();
                    r.extends = true;
                    *old = r;
                    continue;
                }
            }
            r.queries = true;
            out.push(r);
        }
        out
    })
}

fn record(e: &Json) -> Record {
    let s = |k: &str| e.get(k).and_then(|v| v.as_str()).unwrap_or_default();
    let receiver = e.get("receiver").and_then(|r| r.as_str()).map(|r| {
        if r == "string" {
            Kind::String
        } else if r.ends_with("[]") {
            Kind::List
        } else {
            Kind::Map
        }
    });
    Record {
        name: s("name").to_string(),
        signature: s("signature").to_string(),
        doc: e.get("doc").and_then(|d| d.as_str()).map(String::from),
        receiver,
        sig: parse_sig(s("signature")).expect("library signatures parse"),
        recv: e
            .get("receiver")
            .and_then(|r| r.as_str())
            .map(|r| parse_type_vars(r).expect("library receivers parse")),
        narrows: e.get("narrows").is_some(),
        renders: e
            .get("renders")
            .and_then(|t| t.as_arr())
            .map(|a| {
                a.iter()
                    .filter_map(|n| n.as_str().map(String::from))
                    .collect()
            })
            .unwrap_or_default(),
        tested: e
            .get("tested")
            .and_then(|t| t.as_arr())
            .map(|a| {
                a.iter()
                    .filter_map(|n| n.as_f64())
                    .map(|n| n as usize)
                    .collect()
            })
            .unwrap_or_default(),
        // `core` is the default group, always on (`f06-groups.md` 1.1).
        extends: false,
        queries: false,
        ext_cases: None,
        group: e
            .get("group")
            .and_then(|g| g.as_str())
            .filter(|g| *g != "core")
            .map(String::from),
        steps: s("steps").to_string(),
        units: s("units").to_string(),
        cases: e.get("cases").cloned(),
    }
}

/// The constants of the groups (`f06-groups.md` 3.3).
pub fn constants() -> &'static [Constant] {
    static C: OnceLock<Vec<Constant>> = OnceLock::new();
    C.get_or_init(|| {
        let doc = json::parse(GROUPS_SOURCE).expect("suite/library-groups.json is valid JSON");
        doc.as_arr()
            .unwrap_or_default()
            .iter()
            .filter(|e| e.get("type").is_some())
            .map(|e| Constant {
                name: e
                    .get("name")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default()
                    .to_string(),
                group: e
                    .get("group")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default()
                    .to_string(),
                value: e.get("value").and_then(|v| v.as_f64()).unwrap_or_default(),
                cases: e.get("cases").cloned(),
                ty: e
                    .get("type")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default()
                    .to_string(),
                doc: e.get("doc").and_then(|d| d.as_str()).map(String::from),
            })
            .collect()
    })
}

/// The library function of a name.
pub fn function(name: &str) -> Option<usize> {
    records()
        .iter()
        .position(|r| r.receiver.is_none() && r.name == name)
}

/// The library method of a kind and a name.
pub fn method(kind: Kind, name: &str) -> Option<usize> {
    // The first record of each receiver kind and name, found once.
    static M: OnceLock<std::collections::HashMap<String, [Option<usize>; 7]>> = OnceLock::new();
    let m = M.get_or_init(|| {
        let mut m: std::collections::HashMap<String, [Option<usize>; 7]> =
            std::collections::HashMap::new();
        for (i, r) in records().iter().enumerate() {
            if let Some(k) = r.receiver {
                m.entry(r.name.clone()).or_default()[k as usize].get_or_insert(i);
            }
        }
        m
    });
    m.get(name).and_then(|a| a[kind as usize])
}

/// The name of an entry as an environment removes it (`03-host.md` 3.3.1,
/// `04-library.md` 1.1): `len`, `string.repeat`, `list.map`, `map.keys`.
pub fn entry_name(idx: usize) -> String {
    let n = records().len();
    if idx >= n {
        return constants()[idx - n].name.clone();
    }
    let r = &records()[idx];
    match r.receiver {
        None => r.name.clone(),
        Some(k) => format!("{}.{}", kind_word(k), r.name),
    }
}

fn kind_word(k: Kind) -> &'static str {
    match k {
        Kind::String => "string",
        Kind::List => "list",
        _ => "map",
    }
}

/// The entry an environment's removed name names, if any.
pub fn entry_of(name: &str) -> Option<usize> {
    (0..records().len() + constants().len()).find(|&i| entry_name(i) == name)
}

/// The entries this build runs: all 53. A runner names any other in a
/// skip.
pub fn implemented(idx: usize) -> bool {
    crate::eval::lib::FORMULAS
        .iter()
        .any(|(n, k, _, _)| *n == records()[idx].name && *k == records()[idx].receiver)
}
