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

pub struct Record {
    pub name: String,
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
    /// The record's formulas, as written.
    pub steps: String,
    pub units: String,
    /// The cases of the record, as written.
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
        doc.as_arr()
            .unwrap_or_default()
            .iter()
            .map(|e| {
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
                    steps: s("steps").to_string(),
                    units: s("units").to_string(),
                    cases: e.get("cases").cloned(),
                }
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
    records()
        .iter()
        .position(|r| r.receiver == Some(kind) && r.name == name)
}

/// The name of an entry as an environment removes it (`03-host.md` 3.3.1,
/// `04-library.md` 1.1): `len`, `string.repeat`, `list.map`, `map.keys`.
pub fn entry_name(idx: usize) -> String {
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
    (0..records().len()).find(|&i| entry_name(i) == name)
}

/// Whether some kind has a library method of this name.
pub fn is_method_name(name: &str) -> bool {
    records()
        .iter()
        .any(|r| r.receiver.is_some() && r.name == name)
}

/// The entries this build runs: all 53. A runner names any other in a
/// skip.
pub fn implemented(idx: usize) -> bool {
    crate::eval::lib::FORMULAS
        .iter()
        .any(|(n, k, _, _)| *n == records()[idx].name && *k == records()[idx].receiver)
}
