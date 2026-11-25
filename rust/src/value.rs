//! Values of a run (`01-language.md` 6), as `DESIGN.md` plans them.
//!
//! What a run creates is shared behind `Rc` and never changed. Host data is
//! held by reference (`HList`, `HMap`, lent strings) and read in place. A
//! host number that cannot enter the language is held as `ValueInner::BadNum`
//! inside a container until something reads it, which then fails with
//! `number-range` (`03-host.md` 3.4.4).

use crate::host::{Data, HostList, HostMap, OpaqueRef, View};
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

/// Equality of two strings, compared in place when they are short: most
/// differ in length, and a short one is cheaper to compare than to call the
/// platform's `memcmp` for.
#[inline(always)]
pub fn str_eq(a: &str, b: &str) -> bool {
    let (a, b) = (a.as_bytes(), b.as_bytes());
    if a.len() != b.len() {
        return false;
    }
    if a.len() > 16 {
        return a == b;
    }
    a.iter().zip(b).all(|(x, y)| x == y)
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
        match v.0 {
            ValueInner::List(rc) => {
                if let Ok(mut inner) = Rc::try_unwrap(rc) {
                    stack.append(&mut inner.0);
                }
            }
            ValueInner::Map(rc) => {
                if let Ok(mut inner) = Rc::try_unwrap(rc) {
                    stack.extend(std::mem::take(&mut inner.0).into_iter().map(|(_, v)| v));
                }
            }
            inner => release(Value(inner)),
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
        // Only a list or a map inside can nest; other entries drop here.
        if self
            .0
            .iter()
            .any(|(_, v)| matches!(v.0, ValueInner::List(_) | ValueInner::Map(_)))
        {
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
    pub fn from_written(entries: Vec<(Str<'h>, Value<'h>)>) -> MapVal<'h> {
        MapVal::from_written_sized(entries, None)
    }

    /// As `from_written`, for a map of `size` entries when it is known.
    pub fn from_written_sized(
        mut entries: Vec<(Str<'h>, Value<'h>)>,
        size: Option<usize>,
    ) -> MapVal<'h> {
        // A stable sort keeps the written order among equal keys; the last
        // of each run of one key is the one that stands.
        entries.sort_by(|a, b| a.0.as_str().cmp(b.0.as_str()));
        let mut out: Vec<(Str<'h>, Value<'h>)> = Vec::with_capacity(size.unwrap_or(entries.len()));
        for e in entries {
            match out.last_mut() {
                Some(last) if last.0.as_str() == e.0.as_str() => *last = e,
                _ => out.push(e),
            }
        }
        MapVal(out)
    }
}

/// A value of a run (`01-language.md` 6).
///
/// A `Value` borrows host data for the lifetime `'h` of the variables passed
/// to [`Program::run`](crate::Program::run) or a host function call. Convert
/// it to an owned [`Data`] tree with [`Value::to_data`], or inspect it in
/// place with its accessors.
#[derive(Clone)]
pub struct Value<'h>(pub(crate) ValueInner<'h>);

#[derive(Clone)]
pub(crate) enum ValueInner<'h> {
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
pub(crate) enum Kind {
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
    pub(crate) fn name(self) -> &'static str {
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

/// Drop a value, with the test for one that owns nothing inline: the drop
/// glue of `Value` is a call, and most values a run drops (numbers, host
/// data, lent strings) need none.
#[inline(always)]
pub(crate) fn release(v: Value<'_>) {
    if v.owns() {
        drop(v)
    } else {
        std::mem::forget(v)
    }
}

/// Store a value into a slot, releasing the one it replaces.
#[inline(always)]
pub(crate) fn put<'h>(slot: &mut Value<'h>, v: Value<'h>) {
    release(std::mem::replace(slot, v))
}

impl<'h> Value<'h> {
    /// Whether dropping the value does any work.
    #[inline(always)]
    pub(crate) fn owns(&self) -> bool {
        matches!(
            self.0,
            ValueInner::Str(Str::Rc(_))
                | ValueInner::List(_)
                | ValueInner::Map(_)
                | ValueInner::Opaque(_)
        )
    }

    /// A copy: inline for a value that owns nothing, else `clone`.
    #[inline(always)]
    pub(crate) fn dup(&self) -> Value<'h> {
        match &self.0 {
            ValueInner::Null => Value(ValueInner::Null),
            ValueInner::Bool(b) => Value(ValueInner::Bool(*b)),
            ValueInner::Num(n) => Value(ValueInner::Num(*n)),
            ValueInner::Str(Str::Lent(s)) => Value(ValueInner::Str(Str::Lent(s))),
            ValueInner::HList(l) => Value(ValueInner::HList(*l)),
            ValueInner::HMap(m) => Value(ValueInner::HMap(*m)),
            _ => self.clone(),
        }
    }

    /// Wrap an owned string as a [`Value`].
    pub(crate) fn str(s: String) -> Value<'h> {
        Value(ValueInner::Str(Str::new(s)))
    }

    /// Wrap a vector of values as a list [`Value`].
    pub(crate) fn list(v: Vec<Value<'h>>) -> Value<'h> {
        Value(ValueInner::List(Rc::new(ListVal(v))))
    }

    /// The language kind of this value (`01-language.md` 6.1).
    pub(crate) fn kind(&self) -> Kind {
        match &self.0 {
            ValueInner::Null => Kind::Null,
            ValueInner::Bool(_) => Kind::Bool,
            ValueInner::Num(_) | ValueInner::BadNum => Kind::Number,
            ValueInner::Str(_) => Kind::String,
            ValueInner::List(_) | ValueInner::HList(_) => Kind::List,
            ValueInner::Map(_) | ValueInner::HMap(_) => Kind::Map,
            ValueInner::Opaque(_) => Kind::Opaque,
        }
    }

    /// Whether this value is `null`.
    pub fn is_null(&self) -> bool {
        matches!(self.0, ValueInner::Null)
    }

    /// Whether this value is a list.
    pub fn is_list(&self) -> bool {
        matches!(self.0, ValueInner::List(_) | ValueInner::HList(_))
    }

    /// Whether this value is a map.
    pub fn is_map(&self) -> bool {
        matches!(self.0, ValueInner::Map(_) | ValueInner::HMap(_))
    }

    /// The boolean value when this value is a `bool`; `None` otherwise.
    pub fn as_bool(&self) -> Option<bool> {
        match self.0 {
            ValueInner::Bool(b) => Some(b),
            _ => None,
        }
    }

    /// The finite IEEE 754 binary64 number when this value is a number; `None`
    /// otherwise (including for a host number out of range).
    pub fn as_f64(&self) -> Option<f64> {
        match self.0 {
            ValueInner::Num(n) => Some(n),
            _ => None,
        }
    }

    /// The integer value when this value is a whole number within
    /// `±(2^53 - 1)`; `None` otherwise.
    pub fn as_i64(&self) -> Option<i64> {
        match self.0 {
            ValueInner::Num(n) if is_whole(n) => Some(n as i64),
            _ => None,
        }
    }

    /// Borrow the opaque handle when this value is opaque; `None` otherwise.
    pub fn as_opaque(&self) -> Option<&OpaqueRef> {
        match &self.0 {
            ValueInner::Opaque(o) => Some(o.as_ref()),
            _ => None,
        }
    }

    /// The truthiness test (`01-language.md` 6.5). It reads no element.
    pub(crate) fn truthy(&self) -> bool {
        match &self.0 {
            ValueInner::Null => false,
            ValueInner::Bool(b) => *b,
            ValueInner::Num(n) => *n != 0.0,
            ValueInner::BadNum => true,
            ValueInner::Str(s) => !s.as_str().is_empty(),
            ValueInner::List(l) => !l.0.is_empty(),
            ValueInner::HList(l) => !l.is_empty(),
            ValueInner::Map(m) => !m.0.is_empty(),
            ValueInner::HMap(m) => !m.is_empty(),
            ValueInner::Opaque(_) => true,
        }
    }

    /// Borrow the string contents when this value is a string; `None` otherwise.
    pub fn as_str(&self) -> Option<&str> {
        match &self.0 {
            ValueInner::Str(s) => Some(s.as_str()),
            _ => None,
        }
    }

    /// Whether a list or a map is empty; `None` for another kind.
    pub fn is_empty(&self) -> Option<bool> {
        self.len().map(|n| n == 0)
    }

    /// The length of a list, or the number of entries of a map.
    pub fn len(&self) -> Option<usize> {
        match &self.0 {
            ValueInner::List(l) => Some(l.0.len()),
            ValueInner::HList(l) => Some(l.len()),
            ValueInner::Map(m) => Some(m.0.len()),
            ValueInner::HMap(m) => Some(m.len()),
            _ => None,
        }
    }

    /// The element at `i` of a list, or `null` if not a list or `i` is out of
    /// range.
    pub fn elem(&self, i: usize) -> Value<'h> {
        match &self.0 {
            ValueInner::List(l) => {
                l.0.get(i)
                    .map(Value::dup)
                    .unwrap_or(Value(ValueInner::Null))
            }
            ValueInner::HList(l) => match l.get(i) {
                Some(x) => from_view(x),
                None => Value(ValueInner::Null),
            },
            _ => Value(ValueInner::Null),
        }
    }

    /// The value under a key of a map.
    pub fn field(&self, key: &str) -> Option<Value<'h>> {
        match &self.0 {
            ValueInner::Map(m) => m.get(key).map(Value::dup),
            ValueInner::HMap(m) => m.get(key).map(from_view),
            _ => None,
        }
    }

    /// The entries of a map in key order.
    pub(crate) fn entries(&self) -> Vec<(Str<'h>, Value<'h>)> {
        match &self.0 {
            ValueInner::Map(m) => m.0.clone(),
            ValueInner::HMap(m) => {
                let mut keys = m.keys();
                keys.sort_unstable();
                keys.into_iter()
                    .map(|k| {
                        (
                            Str::Lent(k),
                            m.get(k).map(from_view).unwrap_or(Value(ValueInner::Null)),
                        )
                    })
                    .collect()
            }
            _ => Vec::new(),
        }
    }

    /// Visit each key of a map in any order, without sorting or cloning
    /// values.
    pub fn for_each_key<'a>(&'a self, mut f: impl FnMut(&'a str)) {
        match &self.0 {
            ValueInner::Map(m) => {
                for (k, _) in &m.0 {
                    f(k.as_str());
                }
            }
            ValueInner::HMap(m) => {
                for k in m.keys() {
                    f(k);
                }
            }
            _ => {}
        }
    }

    /// The keys of a map in key order.
    pub(crate) fn keys(&self) -> Vec<Str<'h>> {
        match &self.0 {
            ValueInner::Map(m) => m.0.iter().map(|(k, _)| k.clone()).collect(),
            ValueInner::HMap(m) => {
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
            Data::Null => Value(ValueInner::Null),
            Data::Bool(b) => Value(ValueInner::Bool(*b)),
            Data::Int(i) => int_value(*i),
            Data::Float(f) => float_value(*f),
            Data::Str(s) => Value::str(s.clone()),
            Data::List(l) => Value::list(l.iter().map(Value::from_data).collect()),
            Data::Map(m) => Value(ValueInner::Map(Rc::new(MapVal(
                m.iter()
                    .map(|(k, v)| (Str::new(k.clone()), Value::from_data(v)))
                    .collect(),
            )))),
            Data::Opaque(o) => Value(ValueInner::Opaque(Rc::new(o.clone()))),
        }
    }

    /// An owned copy. Nesting past 1,024 levels is cut off as null: no case
    /// file can state deeper data.
    pub fn to_data(&self) -> Data {
        self.to_data_at(0, None)
    }

    /// An owned copy bounded by a total emitted node and container-slot budget,
    /// so shared DAGs and wide host containers cannot expand unboundedly when
    /// building run diagnostics (e.g., `fail`). Any container whose length
    /// exceeds the remaining slot budget collapses as a whole subtree to
    /// `Data::Null`, rather than returning a truncated list or partial map.
    pub(crate) fn to_data_bounded(&self, max_nodes: usize) -> Data {
        if max_nodes == 0 {
            return Data::Null;
        }
        let mut budget = max_nodes - 1;
        self.to_data_at(0, Some(&mut budget))
    }

    fn to_data_at(&self, depth: usize, mut budget: Option<&mut usize>) -> Data {
        if depth > 1024 {
            return Data::Null;
        }
        match &self.0 {
            ValueInner::Null | ValueInner::BadNum => Data::Null,
            ValueInner::Bool(b) => Data::Bool(*b),
            ValueInner::Num(n) => Data::Float(*n),
            ValueInner::Str(s) => Data::Str(s.as_str().to_string()),
            ValueInner::List(_) | ValueInner::HList(_) => {
                let n = self.len().unwrap_or(0);
                if let Some(b) = budget.as_deref_mut() {
                    if n > *b {
                        return Data::Null;
                    }
                    *b -= n;
                }
                let mut out = Vec::with_capacity(n.min(256));
                for i in 0..n {
                    out.push(self.elem(i).to_data_at(depth + 1, budget.as_deref_mut()));
                }
                Data::List(out)
            }
            ValueInner::Map(_) | ValueInner::HMap(_) => {
                let n = self.len().unwrap_or(0);
                if let Some(b) = budget.as_deref_mut() {
                    if n > *b {
                        return Data::Null;
                    }
                }
                let entries = self.entries();
                if let Some(b) = budget.as_deref_mut() {
                    if entries.len() > *b {
                        return Data::Null;
                    }
                    *b -= entries.len();
                }
                let mut out = std::collections::BTreeMap::new();
                for (k, v) in entries {
                    out.insert(
                        k.as_str().to_string(),
                        v.to_data_at(depth + 1, budget.as_deref_mut()),
                    );
                }
                Data::Map(out)
            }
            ValueInner::Opaque(o) => Data::Opaque(OpaqueRef::clone(o)),
        }
    }
}

fn int_value(i: i64) -> Value<'static> {
    if (i as i128).abs() > MAX_WHOLE as i128 {
        Value(ValueInner::BadNum)
    } else {
        Value(ValueInner::Num(i as f64))
    }
}

fn float_value(f: f64) -> Value<'static> {
    if !f.is_finite() {
        Value(ValueInner::BadNum)
    } else if f == 0.0 {
        Value(ValueInner::Num(0.0))
    } else {
        Value(ValueInner::Num(f))
    }
}

/// A host view as a value. Nothing is copied.
#[inline]
pub(crate) fn from_view<'h>(v: View<'h>) -> Value<'h> {
    match v {
        View::Null => Value(ValueInner::Null),
        View::Bool(b) => Value(ValueInner::Bool(b)),
        View::Int(i) => int_value(i),
        View::UInt(u) => {
            if u as f64 > MAX_WHOLE {
                Value(ValueInner::BadNum)
            } else {
                Value(ValueInner::Num(u as f64))
            }
        }
        View::Float(f) => float_value(f),
        View::Str(s) => Value(ValueInner::Str(Str::Lent(s))),
        View::List(l) => Value(ValueInner::HList(l)),
        View::Map(m) => Value(ValueInner::HMap(m)),
        View::Opaque(o) => Value(ValueInner::Opaque(Rc::new(o))),
    }
}

/// Equality with no charge and no depth limit, for comparing with data a
/// case file states. It stops where `d` stops, so its depth is `d`'s.
pub fn eq_data(v: &Value<'_>, d: &Data) -> bool {
    match (&v.0, d) {
        (ValueInner::Null, Data::Null) => true,
        (ValueInner::Bool(a), Data::Bool(b)) => a == b,
        (ValueInner::Num(a), Data::Float(b)) => a == b,
        (ValueInner::Num(a), Data::Int(b)) => *a == *b as f64,
        (ValueInner::Str(a), Data::Str(b)) => a.as_str() == b,
        (ValueInner::Opaque(a), Data::Opaque(b)) => a.same(b),
        (ValueInner::List(_) | ValueInner::HList(_), Data::List(b)) => {
            v.len() == Some(b.len()) && b.iter().enumerate().all(|(i, x)| eq_data(&v.elem(i), x))
        }
        (ValueInner::Map(_) | ValueInner::HMap(_), Data::Map(b)) => {
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
        match &self.0 {
            ValueInner::Null => f.write_str("null"),
            ValueInner::Bool(b) => write!(f, "{b}"),
            ValueInner::Num(n) => f.write_str(&crate::number::to_text(*n)),
            ValueInner::BadNum => f.write_str("<host number out of range>"),
            ValueInner::Str(s) => write!(f, "{:?}", s.as_str()),
            ValueInner::List(_) | ValueInner::HList(_) => {
                let n = self.len().unwrap_or(0);
                f.debug_list()
                    .entries((0..n).map(|i| self.elem(i)))
                    .finish()
            }
            ValueInner::Map(_) | ValueInner::HMap(_) => {
                let es = self.entries();
                f.debug_map()
                    .entries(es.iter().map(|(k, v)| (k.as_str(), v)))
                    .finish()
            }
            ValueInner::Opaque(o) => write!(f, "{o:?}"),
        }
    }
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

#[cfg(test)]
mod layout {
    // The sizes rust/DESIGN.md's memory bound is derived from (`02-evaluation.md` 8.5 item 2).
    #[test]
    #[cfg(target_pointer_width = "64")]
    fn value_and_str_are_24_bytes() {
        assert_eq!(std::mem::size_of::<super::Value<'static>>(), 24);
        assert_eq!(std::mem::size_of::<super::Str<'static>>(), 24);
    }
}
