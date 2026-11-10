//! Values of a run (`01-language.md` 6), as `DESIGN.md` plans them.
//!
//! What a run creates is shared behind `Rc` and never changed. Host data is
//! held by reference (`HList`, `HMap`, lent strings) and read in place. A
//! host number that cannot enter the language is held as [`Value::BadNum`]
//! inside a container until something reads it, which then fails with
//! `number-range` (`03-host.md` 3.4.4).

use crate::host::{Data, HostList, HostMap, OpaqueRef, View};
use std::cmp::Ordering;
use std::fmt;
use std::rc::Rc;

/// The largest whole number, 2^53 − 1.
pub const MAX_WHOLE: f64 = 9007199254740991.0;

/// A string: text the run created, or text lent by the program or the host.
#[derive(Clone)]
pub enum Str<'h> {
    Rc(Rc<str>),
    Lent(&'h str),
}

impl<'h> Str<'h> {
    pub fn as_str(&self) -> &str {
        match self {
            Str::Rc(s) => s,
            Str::Lent(s) => s,
        }
    }

    pub fn new(s: String) -> Str<'h> {
        Str::Rc(Rc::from(s))
    }
}

impl PartialEq for Str<'_> {
    fn eq(&self, other: &Self) -> bool {
        self.as_str() == other.as_str()
    }
}

/// The weight of a string: code points, each above U+FFFF counted twice
/// (`02-evaluation.md` 4). It is the length in UTF-16 units.
pub fn weight(s: &str) -> u64 {
    s.chars()
        .map(|c| if c as u32 > 0xFFFF { 2 } else { 1 })
        .sum()
}

/// The size of a string: its weight divided by 64, rounded down.
pub fn size(s: &str) -> u64 {
    // The weight is at most the count of UTF-8 bytes.
    if s.len() < 64 {
        return 0;
    }
    weight(s) / 64
}

/// The size of the longest common prefix of two strings.
pub fn lcp_size(a: &str, b: &str) -> u64 {
    // The prefix's weight is at most the shorter text's count of bytes.
    if a.len().min(b.len()) < 64 {
        return 0;
    }
    let mut w = 0;
    for (x, y) in a.chars().zip(b.chars()) {
        if x != y {
            break;
        }
        w += if x as u32 > 0xFFFF { 2 } else { 1 };
    }
    w / 64
}

/// The number of code points.
pub fn cp_len(s: &str) -> usize {
    s.chars().count()
}

/// `L(n)`: 0 below 2, else the least `L` with 2 ** `L` >= `n`.
pub fn log2_ceil(n: u64) -> u64 {
    if n < 2 {
        0
    } else {
        64 - (n - 1).leading_zeros() as u64
    }
}

#[inline]
pub fn is_whole(x: f64) -> bool {
    x.fract() == 0.0 && x.abs() <= MAX_WHOLE
}

/// A list the run created. It drops without recursion.
pub struct ListVal<'h>(pub Vec<Value<'h>>);

/// A map the run created: entries sorted by key in code point order (UTF-8
/// byte order is code point order). It drops without recursion.
pub struct MapVal<'h>(pub Vec<(Str<'h>, Value<'h>)>);

fn drop_all(mut stack: Vec<Value<'_>>) {
    while let Some(v) = stack.pop() {
        match v {
            Value::List(rc) => {
                if let Ok(mut inner) = Rc::try_unwrap(rc) {
                    stack.append(&mut inner.0);
                }
            }
            Value::Map(rc) => {
                if let Ok(mut inner) = Rc::try_unwrap(rc) {
                    stack.extend(std::mem::take(&mut inner.0).into_iter().map(|(_, v)| v));
                }
            }
            _ => {}
        }
    }
}

impl Drop for ListVal<'_> {
    fn drop(&mut self) {
        if !self.0.is_empty() {
            drop_all(std::mem::take(&mut self.0));
        }
    }
}

impl Drop for MapVal<'_> {
    fn drop(&mut self) {
        if !self.0.is_empty() {
            drop_all(
                std::mem::take(&mut self.0)
                    .into_iter()
                    .map(|(_, v)| v)
                    .collect(),
            );
        }
    }
}

impl<'h> MapVal<'h> {
    pub fn get(&self, key: &str) -> Option<&Value<'h>> {
        self.0
            .binary_search_by(|(k, _)| k.as_str().cmp(key))
            .ok()
            .map(|i| &self.0[i].1)
    }

    /// Build from entries in written order: a later key replaces an earlier
    /// one.
    pub fn from_written(mut entries: Vec<(Str<'h>, Value<'h>)>) -> MapVal<'h> {
        // A stable sort keeps the written order among equal keys; the last
        // of each run of one key is the one that stands.
        entries.sort_by(|a, b| a.0.as_str().cmp(b.0.as_str()));
        let mut out: Vec<(Str<'h>, Value<'h>)> = Vec::with_capacity(entries.len());
        for e in entries {
            match out.last_mut() {
                Some(last) if last.0.as_str() == e.0.as_str() => *last = e,
                _ => out.push(e),
            }
        }
        MapVal(out)
    }
}

/// A value of a run.
#[derive(Clone)]
pub enum Value<'h> {
    Null,
    Bool(bool),
    Num(f64),
    Str(Str<'h>),
    List(Rc<ListVal<'h>>),
    Map(Rc<MapVal<'h>>),
    HList(&'h dyn HostList),
    HMap(&'h dyn HostMap),
    /// Behind `Rc`, so that a value is small.
    Opaque(Rc<OpaqueRef>),
    /// A host number that cannot enter. Reading it fails.
    BadNum,
}

/// The seven kinds (`01-language.md` 6.1).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Kind {
    Null,
    Bool,
    Number,
    String,
    List,
    Map,
    Opaque,
}

impl Kind {
    /// What `type(x)` gives.
    pub fn name(self) -> &'static str {
        match self {
            Kind::Null => "null",
            Kind::Bool => "bool",
            Kind::Number => "number",
            Kind::String => "string",
            Kind::List => "list",
            Kind::Map => "map",
            Kind::Opaque => "opaque",
        }
    }
}

impl<'h> Value<'h> {
    pub fn str(s: String) -> Value<'h> {
        Value::Str(Str::new(s))
    }

    pub fn list(v: Vec<Value<'h>>) -> Value<'h> {
        Value::List(Rc::new(ListVal(v)))
    }

    pub fn kind(&self) -> Kind {
        match self {
            Value::Null => Kind::Null,
            Value::Bool(_) => Kind::Bool,
            Value::Num(_) | Value::BadNum => Kind::Number,
            Value::Str(_) => Kind::String,
            Value::List(_) | Value::HList(_) => Kind::List,
            Value::Map(_) | Value::HMap(_) => Kind::Map,
            Value::Opaque(_) => Kind::Opaque,
        }
    }

    pub fn is_null(&self) -> bool {
        matches!(self, Value::Null)
    }

    /// The truthiness test (`01-language.md` 6.5). It reads no element.
    pub fn truthy(&self) -> bool {
        match self {
            Value::Null => false,
            Value::Bool(b) => *b,
            Value::Num(n) => *n != 0.0,
            Value::BadNum => true,
            Value::Str(s) => !s.as_str().is_empty(),
            Value::List(l) => !l.0.is_empty(),
            Value::HList(l) => !l.is_empty(),
            Value::Map(m) => !m.0.is_empty(),
            Value::HMap(m) => !m.is_empty(),
            Value::Opaque(_) => true,
        }
    }

    pub fn as_str(&self) -> Option<&str> {
        match self {
            Value::Str(s) => Some(s.as_str()),
            _ => None,
        }
    }

    /// Whether a list or a map is empty; `None` for another kind.
    pub fn is_empty(&self) -> Option<bool> {
        self.len().map(|n| n == 0)
    }

    /// The length of a list, or the number of entries of a map.
    pub fn len(&self) -> Option<usize> {
        match self {
            Value::List(l) => Some(l.0.len()),
            Value::HList(l) => Some(l.len()),
            Value::Map(m) => Some(m.0.len()),
            Value::HMap(m) => Some(m.len()),
            _ => None,
        }
    }

    /// The element at `i` of a list, which must be in range. It may be
    /// `BadNum`; the reader tests it.
    pub fn elem(&self, i: usize) -> Value<'h> {
        match self {
            Value::List(l) => l.0[i].clone(),
            Value::HList(l) => l.get(i).map(from_view).unwrap_or(Value::Null),
            _ => Value::Null,
        }
    }

    /// The value under a key of a map.
    pub fn field(&self, key: &str) -> Option<Value<'h>> {
        match self {
            Value::Map(m) => m.get(key).cloned(),
            Value::HMap(m) => m.get(key).map(from_view),
            _ => None,
        }
    }

    /// The entries of a map in key order.
    pub fn entries(&self) -> Vec<(Str<'h>, Value<'h>)> {
        match self {
            Value::Map(m) => m.0.clone(),
            Value::HMap(m) => {
                let mut keys = m.keys();
                keys.sort_unstable();
                keys.into_iter()
                    .map(|k| (Str::Lent(k), m.get(k).map(from_view).unwrap_or(Value::Null)))
                    .collect()
            }
            _ => Vec::new(),
        }
    }

    /// The keys of a map in key order.
    pub fn keys(&self) -> Vec<Str<'h>> {
        match self {
            Value::Map(m) => m.0.iter().map(|(k, _)| k.clone()).collect(),
            Value::HMap(m) => {
                let mut keys = m.keys();
                keys.sort_unstable();
                keys.into_iter().map(Str::Lent).collect()
            }
            _ => Vec::new(),
        }
    }

    /// A host value tree, as a run's value (a host function's result).
    pub fn from_data(d: &Data) -> Value<'static> {
        match d {
            Data::Null => Value::Null,
            Data::Bool(b) => Value::Bool(*b),
            Data::Int(i) => int_value(*i),
            Data::Float(f) => float_value(*f),
            Data::Str(s) => Value::str(s.clone()),
            Data::List(l) => Value::list(l.iter().map(Value::from_data).collect()),
            Data::Map(m) => Value::Map(Rc::new(MapVal(
                m.iter()
                    .map(|(k, v)| (Str::new(k.clone()), Value::from_data(v)))
                    .collect(),
            ))),
            Data::Opaque(o) => Value::Opaque(Rc::new(o.clone())),
        }
    }

    /// An owned copy. Nesting past 1,024 levels is cut off as null: no case
    /// file can state deeper data.
    pub fn to_data(&self) -> Data {
        self.to_data_at(0)
    }

    fn to_data_at(&self, depth: usize) -> Data {
        if depth > 1024 {
            return Data::Null;
        }
        match self {
            Value::Null | Value::BadNum => Data::Null,
            Value::Bool(b) => Data::Bool(*b),
            Value::Num(n) => Data::Float(*n),
            Value::Str(s) => Data::Str(s.as_str().to_string()),
            Value::List(_) | Value::HList(_) => {
                let n = self.len().unwrap_or(0);
                Data::List((0..n).map(|i| self.elem(i).to_data_at(depth + 1)).collect())
            }
            Value::Map(_) | Value::HMap(_) => Data::Map(
                self.entries()
                    .into_iter()
                    .map(|(k, v)| (k.as_str().to_string(), v.to_data_at(depth + 1)))
                    .collect(),
            ),
            Value::Opaque(o) => Data::Opaque(OpaqueRef::clone(o)),
        }
    }
}

fn int_value(i: i64) -> Value<'static> {
    if (i as i128).abs() > MAX_WHOLE as i128 {
        Value::BadNum
    } else {
        Value::Num(i as f64)
    }
}

fn float_value(f: f64) -> Value<'static> {
    if !f.is_finite() {
        Value::BadNum
    } else if f == 0.0 {
        Value::Num(0.0)
    } else {
        Value::Num(f)
    }
}

/// A host view as a value. Nothing is copied.
pub fn from_view<'h>(v: View<'h>) -> Value<'h> {
    match v {
        View::Null => Value::Null,
        View::Bool(b) => Value::Bool(b),
        View::Int(i) => int_value(i),
        View::UInt(u) => {
            if u as f64 > MAX_WHOLE {
                Value::BadNum
            } else {
                Value::Num(u as f64)
            }
        }
        View::Float(f) => float_value(f),
        View::Str(s) => Value::Str(Str::Lent(s)),
        View::List(l) => Value::HList(l),
        View::Map(m) => Value::HMap(m),
        View::Opaque(o) => Value::Opaque(Rc::new(o)),
    }
}

/// Equality with no charge and no depth limit, for comparing with data a
/// case file states. It stops where `d` stops, so its depth is `d`'s.
pub fn eq_data(v: &Value<'_>, d: &Data) -> bool {
    match (v, d) {
        (Value::Null, Data::Null) => true,
        (Value::Bool(a), Data::Bool(b)) => a == b,
        (Value::Num(a), Data::Float(b)) => a == b,
        (Value::Num(a), Data::Int(b)) => *a == *b as f64,
        (Value::Str(a), Data::Str(b)) => a.as_str() == b,
        (Value::Opaque(a), Data::Opaque(b)) => a.same(b),
        (Value::List(_) | Value::HList(_), Data::List(b)) => {
            v.len() == Some(b.len()) && b.iter().enumerate().all(|(i, x)| eq_data(&v.elem(i), x))
        }
        (Value::Map(_) | Value::HMap(_), Data::Map(b)) => {
            v.len() == Some(b.len())
                && b.iter()
                    .all(|(k, x)| v.field(k).is_some_and(|y| eq_data(&y, x)))
        }
        _ => false,
    }
}

impl fmt::Debug for Value<'_> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Deep values are shown cut, so that showing one never overflows the
        // stack.
        thread_local!(static DEPTH: std::cell::Cell<u32> = const { std::cell::Cell::new(0) });
        let d = DEPTH.with(|c| c.get());
        if d > 64 {
            return f.write_str("...");
        }
        DEPTH.with(|c| c.set(d + 1));
        let r = self.fmt_inner(f);
        DEPTH.with(|c| c.set(d));
        r
    }
}

impl Value<'_> {
    fn fmt_inner(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Value::Null => f.write_str("null"),
            Value::Bool(b) => write!(f, "{b}"),
            Value::Num(n) => f.write_str(&crate::number::to_text(*n)),
            Value::BadNum => f.write_str("<host number out of range>"),
            Value::Str(s) => write!(f, "{:?}", s.as_str()),
            Value::List(_) | Value::HList(_) => {
                let n = self.len().unwrap_or(0);
                f.debug_list()
                    .entries((0..n).map(|i| self.elem(i)))
                    .finish()
            }
            Value::Map(_) | Value::HMap(_) => {
                let es = self.entries();
                f.debug_map()
                    .entries(es.iter().map(|(k, v)| (k.as_str(), v)))
                    .finish()
            }
            Value::Opaque(o) => write!(f, "{o:?}"),
        }
    }
}

/// Code point order of two strings.
pub fn cmp_str(a: &str, b: &str) -> Ordering {
    a.cmp(b)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn deep_lists_drop_without_recursion() {
        let mut v = Value::list(vec![]);
        for _ in 0..1_000_000 {
            v = Value::list(vec![v]);
        }
        drop(v);
    }

    #[test]
    fn weights_and_l() {
        assert_eq!(weight("a\u{1F600}"), 3);
        assert_eq!(size(&"a".repeat(130)), 2);
        assert_eq!(size(&"\u{1F600}".repeat(32)), 1);
        for (n, l) in [
            (0, 0),
            (1, 0),
            (2, 1),
            (3, 2),
            (4, 2),
            (5, 3),
            (8, 3),
            (9, 4),
            (64, 6),
            (65, 7),
            (1000, 10),
        ] {
            assert_eq!(log2_ceil(n), l, "{n}");
        }
    }
}
