//! Package F19: the fixes that four check messages take from tables
//! (`f19-author-feedback.md` 3).
//!
//! A fix is found by exact text in a table: a row of
//! `suite/foreign-names.json`, or the `wrong` list or the name of an entry
//! the environment offers. It is not a suggestion found by search (3.4
//! item 7).

use crate::env::Env;
use crate::json;
use crate::library::{self, Record};
use crate::value::Kind;
use std::sync::OnceLock;

/// The table of foreign names (3.3), as built into the crate.
pub const FOREIGN_SOURCE: &str = include_str!("../data/foreign-names.json");

/// A row of `suite/foreign-names.json` (3.3).
pub struct Foreign {
    pub written: String,
    pub place: String,
    pub kinds: Vec<String>,
    /// The entries the fix names, as a removal names them: the row applies
    /// only when the environment offers every one (3.3 item 2).
    pub entries: Vec<String>,
    pub fix: String,
}

pub fn foreign() -> &'static [Foreign] {
    static F: OnceLock<Vec<Foreign>> = OnceLock::new();
    F.get_or_init(|| {
        let doc = json::parse(FOREIGN_SOURCE).expect("suite/foreign-names.json is valid JSON");
        doc.as_arr()
            .unwrap_or_default()
            .iter()
            .map(|e| {
                let s = |k: &str| {
                    e.get(k)
                        .and_then(|v| v.as_str())
                        .unwrap_or_default()
                        .to_string()
                };
                let list = |k: &str| -> Vec<String> {
                    e.get(k)
                        .and_then(|k| k.as_arr())
                        .map(|a| {
                            a.iter()
                                .filter_map(|k| k.as_str().map(String::from))
                                .collect()
                        })
                        .unwrap_or_default()
                };
                Foreign {
                    written: s("written"),
                    place: s("place"),
                    kinds: list("kinds"),
                    entries: list("entries"),
                    fix: s("fix"),
                }
            })
            .collect()
    })
}

/// The fix of the foreign row of a text and a place that applies (3.3): a
/// row whose entries the environment does not all offer is passed over.
fn foreign_fix(env: &Env, written: &str, place: &str, kind: Option<&str>) -> Option<String> {
    foreign()
        .iter()
        .find(|r| {
            r.written == written
                && r.place == place
                && (place != "method" || kind.is_some_and(|k| r.kinds.iter().any(|x| x == k)))
                && r.entries.iter().all(|n| is_offered(env, n))
        })
        .map(|r| r.fix.clone())
}

/// Whether the environment offers the entry a removal would name `name`.
fn is_offered(env: &Env, name: &str) -> bool {
    library::records().iter().enumerate().any(|(i, r)| {
        library::entry_name(i) == name
            && !env.data().removed.contains(&i)
            && r.group.as_ref().is_none_or(|g| env.group_on(g))
    })
}

/// The receiver kind of a method's record, as a word.
fn kind_word(k: Kind) -> &'static str {
    match k {
        Kind::String => "string",
        Kind::List => "list",
        _ => "map",
    }
}

/// The entries the environment offers (1.1): in a group that is on, and not
/// removed.
fn offered(env: &Env) -> impl Iterator<Item = &'static Record> + '_ {
    library::records()
        .iter()
        .enumerate()
        .filter(move |(i, r)| {
            !env.data().removed.contains(i) && r.group.as_ref().is_none_or(|g| env.group_on(g))
        })
        .map(|(_, r)| r)
}

/// The entry form of a record (3.4 item 2).
fn form(r: &Record) -> String {
    match r.receiver {
        None => format!("use {}(...)", r.name),
        Some(_) => format!("use x.{}(...)", r.name),
    }
}

/// The entry forms of the matching records (3.4 item 3): distinct, sorted
/// by code point, the first three, joined with ` or `. None when no record
/// matches.
fn forms<'a>(rs: impl Iterator<Item = &'a Record>) -> Option<String> {
    let mut out: Vec<String> = rs.map(form).collect();
    if out.is_empty() {
        return None;
    }
    out.sort();
    out.dedup();
    out.truncate(3);
    Some(out.join(" or "))
}

fn in_wrong(r: &Record, n: &str) -> bool {
    r.wrong.iter().any(|w| w == n)
}

/// `undefined-name` of template `name` (3.4 rows 1 and 2, item 1). `field`
/// is the name of a step `.f` or `?.f` after the identifier.
pub fn name_fix(env: &Env, name: &str, field: Option<&str>) -> Option<String> {
    if let Some(f) = field {
        let text = format!("{name}.{f}");
        if let Some(fix) = foreign_fix(env, &text, "name-field", None) {
            return Some(fix);
        }
        if let Some(fix) = forms(offered(env).filter(|r| in_wrong(r, &text))) {
            return Some(fix);
        }
    }
    foreign_fix(env, name, "name", None)
}

/// `undefined-function` (3.4 rows 1 and 3). `name` is the name as written,
/// with its namespace.
pub fn function_fix(env: &Env, name: &str) -> Option<String> {
    foreign_fix(env, name, "function", None)
        .or_else(|| forms(offered(env).filter(|r| in_wrong(r, name))))
        .or_else(|| forms(offered(env).filter(|r| r.receiver.is_some() && r.name == name)))
}

/// `no-method` (3.4 rows 1 and 4). `kind` is the receiver kind: `string`,
/// `list`, `map` or `number`; any other receiver takes no hint (item 5).
pub fn method_fix(env: &Env, kind: Option<&str>, name: &str) -> Option<String> {
    let kind = kind.filter(|k| matches!(*k, "string" | "list" | "map" | "number"))?;
    foreign_fix(env, name, "method", Some(kind))
        .or_else(|| {
            forms(
                offered(env).filter(|r| {
                    r.receiver.is_some_and(|k| kind_word(k) == kind) && in_wrong(r, name)
                }),
            )
        })
        .or_else(|| forms(offered(env).filter(|r| r.receiver.is_none() && in_wrong(r, name))))
        .or_else(|| forms(offered(env).filter(|r| r.receiver.is_none() && r.name == name)))
}

/// `kind`, template `field-read`, on a field (3.4 row 5).
pub fn field_read_fix(env: &Env, field: &str) -> Option<String> {
    forms(offered(env).filter(|r| r.receiver.is_none() && in_wrong(r, field)))
}
