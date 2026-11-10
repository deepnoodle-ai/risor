//! The case table (`04-library.md` 3.8): `suite/casemap.json`, Unicode
//! 16.0.0, compiled into the crate and read once. The platform's own case
//! functions are never used: they follow the platform's Unicode version.

use crate::json::{self, Json};
use std::collections::HashMap;
use std::sync::OnceLock;

/// The table file, as built into the crate.
pub const SOURCE: &str = include_str!("../../suite/casemap.json");

pub struct CaseMap {
    pub upper: HashMap<char, char>,
    pub lower: HashMap<char, char>,
}

fn read(doc: &Json, field: &str) -> HashMap<char, char> {
    let cp = |h: &str| u32::from_str_radix(h, 16).ok().and_then(char::from_u32);
    doc.get(field)
        .and_then(|m| m.as_obj())
        .unwrap_or_default()
        .iter()
        .filter_map(|(k, v)| Some((cp(k)?, cp(v.as_str()?)?)))
        .collect()
}

pub fn table() -> &'static CaseMap {
    static T: OnceLock<CaseMap> = OnceLock::new();
    T.get_or_init(|| {
        let doc = json::parse(SOURCE).expect("suite/casemap.json is valid JSON");
        CaseMap {
            upper: read(&doc, "upper"),
            lower: read(&doc, "lower"),
        }
    })
}

pub fn upper(s: &str) -> String {
    let t = &table().upper;
    s.chars().map(|c| *t.get(&c).unwrap_or(&c)).collect()
}

pub fn lower(s: &str) -> String {
    let t = &table().lower;
    s.chars().map(|c| *t.get(&c).unwrap_or(&c)).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_table_of_16_0_0() {
        assert_eq!(table().upper.len(), 1477);
        assert_eq!(table().lower.len(), 1460);
        assert_eq!(
            upper("caf\u{e9}\u{df}\u{3c2}\u{1c6}\u{fb01}"),
            "CAF\u{c9}\u{df}\u{3a3}\u{1c4}\u{fb01}"
        );
        assert_eq!(
            lower("\u{130}\u{391}\u{3a3}\u{1c5}"),
            "i\u{3b1}\u{3c3}\u{1c6}"
        );
    }
}
