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
    /// The same tables for the ASCII code points, by index.
    ascii_upper: [char; 128],
    ascii_lower: [char; 128],
}

/// The table for the ASCII code points, by index.
fn ascii(t: &HashMap<char, char>) -> [char; 128] {
    std::array::from_fn(|i| {
        let c = char::from(i as u8);
        *t.get(&c).unwrap_or(&c)
    })
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
        let (upper, lower) = (read(&doc, "upper"), read(&doc, "lower"));
        CaseMap {
            ascii_upper: ascii(&upper),
            ascii_lower: ascii(&lower),
            upper,
            lower,
        }
    })
}

fn map(s: &str, t: &HashMap<char, char>, ascii: &[char; 128]) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        out.push(match ascii.get(c as usize) {
            Some(m) => *m,
            None => *t.get(&c).unwrap_or(&c),
        });
    }
    out
}

pub fn upper(s: &str) -> String {
    let t = table();
    map(s, &t.upper, &t.ascii_upper)
}

pub fn lower(s: &str) -> String {
    let t = table();
    map(s, &t.lower, &t.ascii_lower)
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
