//! The library's records (`04-library.md`, `suite/library.json`).
//!
//! The records give each entry's name, receiver and signature, from which
//! come the counts of the static rules and the shape test of a method call.
//! The entries themselves run in `eval::lib`.

use crate::json::{self, Json};
use crate::types::text::{parse_sig, ParamType, Sig};
use crate::value::Kind;
use std::sync::OnceLock;

/// The library file, as built into the crate.
pub const SOURCE: &str = include_str!("../../suite/library.json");

pub struct Record {
    pub name: String,
    /// The kind of the receiver of a method; `None` for a function.
    pub receiver: Option<Kind>,
    pub sig: Sig,
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
