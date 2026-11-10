//! Types (`05-types.md` 2) and the operations on them (section 4), with
//! printing (3.9) and the reading of type text (3.3, 3.4, 19.5).
//!
//! A type is held as one optional slot for each kind, plus the marker `any`
//! (2.3). In Core the map slot holds at most one record. Every operation
//! recurses only into element, field and rest types, so the type depth limit
//! of 64 bounds it (20.6).

use crate::types::text::{self, Rest};
use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::sync::Arc;

#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub enum Num {
    Int,
    Float,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Strs {
    All,
    Lits(BTreeSet<String>),
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Field {
    pub ty: Ty,
    pub optional: bool,
}

/// A record: fields and a rest (2.2). A rest of `never` makes it exact.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Rec {
    pub fields: BTreeMap<String, Field>,
    pub rest: Ty,
    /// The mark *closable* of F1 (`f01-host-types.md` 4.1). Only the mapping
    /// of a schema under the option `closed` sets it, and Close drops it, so
    /// every type outside that mapping has it `false`.
    pub closable: bool,
    /// The size of the record (`05-types.md` 20.7), worked out once.
    pub size: SizeCache,
    /// F1 4.4: for each field a fill option made required, its base and its
    /// fill; the field's type is their Join. Only the mapping of a schema
    /// sets it, and it is gone from the type the mapping gives.
    pub fills: BTreeMap<String, (Ty, Ty)>,
}

/// The type size limit (`05-types.md` 20.7, ruling CG2).
pub const SIZE_LIMIT: u64 = 100_000;

/// A record's size, kept with it once asked for (ruling CG5). A clone starts
/// empty, since a cloned record may then be changed; equality ignores it.
#[derive(Default)]
pub struct SizeCache(std::sync::OnceLock<u64>);

impl Clone for SizeCache {
    fn clone(&self) -> Self {
        SizeCache::default()
    }
}

impl PartialEq for SizeCache {
    fn eq(&self, _: &Self) -> bool {
        true
    }
}

impl Eq for SizeCache {}

impl std::fmt::Debug for SizeCache {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("_")
    }
}

impl Rec {
    /// The size of the record: 1, plus each field's type, plus the rest (a
    /// rest of `never` counts 0). Saturates just above the limit.
    pub fn size(&self) -> u64 {
        *self.size.0.get_or_init(|| {
            let mut n: u64 = 1;
            for f in self.fields.values() {
                n = n.saturating_add(f.ty.size());
                if n > SIZE_LIMIT {
                    return SIZE_LIMIT + 1;
                }
            }
            if !self.rest.is_never() {
                n = n.saturating_add(self.rest.size());
            }
            n.min(SIZE_LIMIT + 1)
        })
    }
}

impl Rec {
    pub fn new(fields: BTreeMap<String, Field>, rest: Ty) -> Rec {
        Rec {
            fields,
            rest,
            closable: false,
            size: SizeCache::default(),
            fills: Default::default(),
        }
    }
}

/// A type. With `any` set, the other slots are empty. With every slot empty
/// it is `never`.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Ty {
    pub any: bool,
    pub null: bool,
    pub boolean: bool,
    pub num: Option<Num>,
    pub str: Option<Strs>,
    pub list: Option<Arc<Ty>>,
    pub map: Option<Arc<Rec>>,
    pub opaque: BTreeSet<String>,
}

impl Ty {
    pub fn never() -> Ty {
        Ty::default()
    }
    pub fn any() -> Ty {
        Ty {
            any: true,
            ..Ty::default()
        }
    }
    pub fn null() -> Ty {
        Ty {
            null: true,
            ..Ty::default()
        }
    }
    pub fn boolean() -> Ty {
        Ty {
            boolean: true,
            ..Ty::default()
        }
    }
    pub fn int() -> Ty {
        Ty {
            num: Some(Num::Int),
            ..Ty::default()
        }
    }
    pub fn float() -> Ty {
        Ty {
            num: Some(Num::Float),
            ..Ty::default()
        }
    }
    pub fn string() -> Ty {
        Ty {
            str: Some(Strs::All),
            ..Ty::default()
        }
    }
    pub fn lit(s: &str) -> Ty {
        Ty {
            str: Some(Strs::Lits([s.to_string()].into())),
            ..Ty::default()
        }
    }
    pub fn lits<'a>(ss: impl Iterator<Item = &'a str>) -> Ty {
        Ty {
            str: Some(Strs::Lits(ss.map(String::from).collect())),
            ..Ty::default()
        }
    }
    pub fn list(e: Ty) -> Ty {
        Ty {
            list: Some(Arc::new(e)),
            ..Ty::default()
        }
    }
    pub fn rec(r: Rec) -> Ty {
        Ty {
            map: Some(Arc::new(r)),
            ..Ty::default()
        }
    }
    /// `map<T>`
    pub fn map_of(t: Ty) -> Ty {
        Ty::rec(Rec {
            fields: BTreeMap::new(),
            rest: t,
            closable: false,
            size: Default::default(),
            fills: Default::default(),
        })
    }
    /// `{}`: the empty map only.
    pub fn empty_map() -> Ty {
        Ty::map_of(Ty::never())
    }
    pub fn opaque(n: &str) -> Ty {
        Ty {
            opaque: [n.to_string()].into(),
            ..Ty::default()
        }
    }

    pub fn is_never(&self) -> bool {
        *self == Ty::default()
    }

    /// The size of the type (`05-types.md` 20.7), saturating just above the
    /// limit. A record's size is kept with it, so this is constant work for
    /// each level of list nesting.
    pub fn size(&self) -> u64 {
        if self.any {
            return 1;
        }
        let mut n: u64 = self.null as u64 + self.boolean as u64 + self.num.is_some() as u64;
        n += match &self.str {
            Some(Strs::All) => 1,
            Some(Strs::Lits(l)) => l.len() as u64,
            None => 0,
        };
        n += self.opaque.len() as u64;
        if let Some(e) = &self.list {
            n = n.saturating_add(1 + e.size());
        }
        if let Some(r) = &self.map {
            n = n.saturating_add(r.size());
        }
        // `never` is 1 when it stands alone.
        n.clamp(1, SIZE_LIMIT + 1)
    }

    /// Whether the type is over the size limit.
    pub fn too_big(&self) -> bool {
        self.size() > SIZE_LIMIT
    }

    /// The number of kinds that have an alternative.
    pub fn kinds(&self) -> usize {
        self.null as usize
            + self.boolean as usize
            + self.num.is_some() as usize
            + self.str.is_some() as usize
            + self.list.is_some() as usize
            + self.map.is_some() as usize
            + self.opaque.len()
    }

    /// Exactly the type `null`.
    pub fn is_exactly_null(&self) -> bool {
        *self == Ty::null()
    }

    pub fn with_null(mut self) -> Ty {
        if !self.any {
            self.null = true;
        }
        self
    }

    /// nonnull(T) (4.5).
    pub fn nonnull(&self) -> Ty {
        let mut t = self.clone();
        t.null = false;
        t
    }

    pub fn has_null(&self) -> bool {
        self.null
    }

    /// The record of a type whose map slot holds one.
    pub fn record(&self) -> Option<&Rec> {
        self.map.as_deref()
    }

    /// The depth of a type (2.5).
    pub fn depth(&self) -> usize {
        let mut d = 0;
        if let Some(e) = &self.list {
            d = d.max(1 + e.depth());
        }
        if let Some(r) = &self.map {
            let inner = r
                .fields
                .values()
                .map(|f| f.ty.depth())
                .chain(std::iter::once(r.rest.depth()))
                .max()
                .unwrap_or(0);
            d = d.max(1 + inner);
        }
        d
    }
}

/// Join(A, B) (4.1).
pub fn join(a: &Ty, b: &Ty) -> Ty {
    if a.any || b.any {
        return Ty::any();
    }
    Ty {
        any: false,
        null: a.null || b.null,
        boolean: a.boolean || b.boolean,
        num: match (a.num, b.num) {
            (Some(x), Some(y)) => Some(x.max(y)),
            (x, y) => x.or(y),
        },
        str: match (&a.str, &b.str) {
            (Some(Strs::Lits(x)), Some(Strs::Lits(y))) => {
                Some(Strs::Lits(x.union(y).cloned().collect()))
            }
            (Some(_), Some(_)) => Some(Strs::All),
            (x, y) => x.clone().or(y.clone()),
        },
        list: match (&a.list, &b.list) {
            (Some(x), Some(y)) => Some(Arc::new(join(x, y))),
            (x, y) => x.clone().or(y.clone()),
        },
        map: match (&a.map, &b.map) {
            (Some(x), Some(y)) => Some(Arc::new(join_rec(x, y))),
            (x, y) => x.clone().or(y.clone()),
        },
        opaque: a.opaque.union(&b.opaque).cloned().collect(),
    }
}

fn join_rec(a: &Rec, b: &Rec) -> Rec {
    let mut fields = BTreeMap::new();
    let mut fills = BTreeMap::new();
    // F1 4.4: a key in both has the Join of the bases and of the fills; a
    // key in one has its base joined with the other's rest, and keeps its
    // fill. A field that carries a fill is required.
    let base = |r: &Rec, k: &str, f: &Field| r.fills.get(k).map_or(f.ty.clone(), |x| x.0.clone());
    for (k, fa) in &a.fields {
        let f = match b.fields.get(k) {
            Some(fb) => {
                let fill = match (a.fills.get(k), b.fills.get(k)) {
                    (Some(x), Some(y)) => Some(join(&x.1, &y.1)),
                    (Some(x), None) | (None, Some(x)) => Some(x.1.clone()),
                    (None, None) => None,
                };
                match fill {
                    None => Field {
                        ty: join(&fa.ty, &fb.ty),
                        optional: fa.optional || fb.optional,
                    },
                    Some(fl) => {
                        let bs = join(&base(a, k, fa), &base(b, k, fb));
                        let ty = join(&bs, &fl);
                        fills.insert(k.clone(), (bs, fl));
                        Field {
                            ty,
                            optional: false,
                        }
                    }
                }
            }
            None => match a.fills.get(k) {
                None => Field {
                    ty: join(&fa.ty, &b.rest),
                    optional: true,
                },
                Some((bs, fl)) => {
                    let bs = join(bs, &b.rest);
                    let ty = join(&bs, fl);
                    fills.insert(k.clone(), (bs, fl.clone()));
                    Field {
                        ty,
                        optional: false,
                    }
                }
            },
        };
        fields.insert(k.clone(), f);
    }
    for (k, fb) in &b.fields {
        if !a.fields.contains_key(k) {
            let f = match b.fills.get(k) {
                None => Field {
                    ty: join(&fb.ty, &a.rest),
                    optional: true,
                },
                Some((bs, fl)) => {
                    let bs = join(bs, &a.rest);
                    let ty = join(&bs, fl);
                    fills.insert(k.clone(), (bs, fl.clone()));
                    Field {
                        ty,
                        optional: false,
                    }
                }
            };
            fields.insert(k.clone(), f);
        }
    }
    Rec {
        fields,
        rest: join(&a.rest, &b.rest),
        // F1 3.6 item 8: closable when one is closable and the other is
        // closable or exact.
        closable: (a.closable || b.closable)
            && (a.closable || a.rest.is_never())
            && (b.closable || b.rest.is_never()),
        size: SizeCache::default(),
        fills,
    }
}

/// The Join of several types. Literal sets are merged in place, so joining
/// n literals is n log n work (ruling CG10), not n squared.
pub fn join_all<'a>(ts: impl Iterator<Item = &'a Ty>) -> Ty {
    let mut acc = Ty::never();
    for t in ts {
        join_into(&mut acc, t);
    }
    acc
}

/// `acc = Join(acc, t)`, with the literal set of `acc` extended in place
/// (ruling CG10): the work is in the size of `t`, not of `acc`.
pub fn join_into(acc: &mut Ty, t: &Ty) {
    if acc.any {
        return;
    }
    if t.any {
        *acc = Ty::any();
        return;
    }
    acc.null |= t.null;
    acc.boolean |= t.boolean;
    acc.num = match (acc.num, t.num) {
        (Some(x), Some(y)) => Some(x.max(y)),
        (x, y) => x.or(y),
    };
    match (&mut acc.str, &t.str) {
        (Some(Strs::Lits(a)), Some(Strs::Lits(b))) => a.extend(b.iter().cloned()),
        (Some(a), Some(_)) => *a = Strs::All,
        (a @ None, b) => *a = b.clone(),
        (Some(_), None) => {}
    }
    acc.list = match (&acc.list, &t.list) {
        (Some(x), Some(y)) => Some(Arc::new(join(x, y))),
        (x, y) => x.clone().or(y.clone()),
    };
    acc.map = match (&acc.map, &t.map) {
        (Some(x), Some(y)) => Some(Arc::new(join_rec(x, y))),
        (x, y) => x.clone().or(y.clone()),
    };
    acc.opaque.extend(t.opaque.iter().cloned());
}

/// Fits(S, T) (4.2). `Some(uses_any)` when S fits T.
pub fn fits(s: &Ty, t: &Ty) -> Option<bool> {
    if s.is_never() || t.any {
        return Some(false);
    }
    if s.any {
        return Some(true);
    }
    let mut used = false;
    if s.null && !t.null {
        return None;
    }
    if s.boolean && !t.boolean {
        return None;
    }
    if let Some(n) = s.num {
        match (n, t.num) {
            (Num::Int, Some(_)) | (Num::Float, Some(Num::Float)) => {}
            _ => return None,
        }
    }
    if let Some(ss) = &s.str {
        match (ss, &t.str) {
            (_, Some(Strs::All)) => {}
            (Strs::Lits(a), Some(Strs::Lits(b))) if a.is_subset(b) => {}
            _ => return None,
        }
    }
    if let Some(e) = &s.list {
        used |= fits(e, t.list.as_ref()?)?;
    }
    if let Some(r) = &s.map {
        used |= fits_rec(r, t.map.as_ref()?)?;
    }
    if !s.opaque.is_subset(&t.opaque) {
        return None;
    }
    Some(used)
}

fn fits_rec(s: &Rec, t: &Rec) -> Option<bool> {
    let mut used = false;
    for (k, ft) in &t.fields {
        match s.fields.get(k) {
            Some(fs) => {
                used |= fits(&fs.ty, &ft.ty)?;
                if fs.optional && !ft.optional {
                    return None;
                }
            }
            None => {
                if !ft.optional {
                    return None;
                }
                used |= fits(&s.rest, &ft.ty)?;
            }
        }
    }
    for (k, fs) in &s.fields {
        if !t.fields.contains_key(k) {
            used |= fits(&fs.ty, &t.rest)?;
        }
    }
    used |= fits(&s.rest, &t.rest)?;
    Some(used)
}

/// Merge(R, M) (4.3): the entries of a map of type M written over a map of
/// record type R. `m` is `None` for `any`.
pub fn merge(r: &Rec, m: Option<&Rec>) -> Rec {
    let Some(m) = m else {
        return Rec {
            fields: r
                .fields
                .iter()
                .map(|(k, f)| {
                    (
                        k.clone(),
                        Field {
                            ty: Ty::any(),
                            optional: f.optional,
                        },
                    )
                })
                .collect(),
            rest: Ty::any(),
            closable: false,
            size: Default::default(),
            fills: Default::default(),
        };
    };
    let mut fields = BTreeMap::new();
    for (k, fm) in &m.fields {
        let f = if !fm.optional {
            fm.clone()
        } else {
            let (rt, req) = match r.fields.get(k) {
                Some(fr) => (&fr.ty, !fr.optional),
                None => (&r.rest, false),
            };
            Field {
                ty: join(&fm.ty, rt),
                optional: !req,
            }
        };
        fields.insert(k.clone(), f);
    }
    for (k, fr) in &r.fields {
        if !m.fields.contains_key(k) {
            let f = if m.rest.is_never() {
                fr.clone()
            } else {
                Field {
                    ty: join(&fr.ty, &m.rest),
                    optional: fr.optional,
                }
            };
            fields.insert(k.clone(), f);
        }
    }
    Rec {
        fields,
        rest: join(&r.rest, &m.rest),
        closable: false,
        size: Default::default(),
        fills: Default::default(),
    }
}

/// Truthy(T) (4.4).
pub fn truthy(t: &Ty) -> Ty {
    if t.any {
        return Ty::any();
    }
    Ty {
        any: false,
        null: false,
        boolean: t.boolean,
        num: t.num,
        str: match &t.str {
            Some(Strs::Lits(l)) => {
                let l: BTreeSet<String> = l.iter().filter(|s| !s.is_empty()).cloned().collect();
                if l.is_empty() {
                    None
                } else {
                    Some(Strs::Lits(l))
                }
            }
            s => s.clone(),
        },
        list: t.list.clone().filter(|e| !e.is_never()),
        map: t
            .map
            .clone()
            .filter(|r| !(r.fields.is_empty() && r.rest.is_never())),
        opaque: t.opaque.clone(),
    }
}

/// Falsy(T) (4.4).
pub fn falsy(t: &Ty) -> Ty {
    if t.any {
        return Ty::any();
    }
    Ty {
        any: false,
        null: t.null,
        boolean: t.boolean,
        num: t.num,
        str: match &t.str {
            Some(Strs::All) => Some(Strs::Lits([String::new()].into())),
            Some(Strs::Lits(l)) if l.contains("") => Some(Strs::Lits([String::new()].into())),
            _ => None,
        },
        list: t.list.as_ref().map(|_| Arc::new(Ty::never())),
        map: t
            .map
            .as_ref()
            .filter(|r| r.fields.values().all(|f| f.optional))
            .map(|_| {
                Arc::new(Rec {
                    fields: BTreeMap::new(),
                    rest: Ty::never(),
                    closable: false,
                    size: Default::default(),
                    fills: Default::default(),
                })
            }),
        opaque: BTreeSet::new(),
    }
}

/// overlap(A, B) (4.5).
pub fn overlap(a: &Ty, b: &Ty) -> bool {
    if a.is_never() || b.is_never() {
        return false;
    }
    if a.any || b.any {
        return true;
    }
    (a.null && b.null)
        || (a.boolean && b.boolean)
        || (a.num.is_some() && b.num.is_some())
        || match (&a.str, &b.str) {
            (Some(Strs::Lits(x)), Some(Strs::Lits(y))) => !x.is_disjoint(y),
            (Some(_), Some(_)) => true,
            _ => false,
        }
        || (a.list.is_some() && b.list.is_some())
        || (a.map.is_some() && b.map.is_some())
        || !a.opaque.is_disjoint(&b.opaque)
}

/// has-opaque(T) (4.5).
pub fn has_opaque(t: &Ty) -> bool {
    if t.any {
        return false;
    }
    !t.opaque.is_empty()
        || t.list.as_ref().is_some_and(|e| has_opaque(e))
        || t.map
            .as_ref()
            .is_some_and(|r| r.fields.values().any(|f| has_opaque(&f.ty)) || has_opaque(&r.rest))
}

/// has-any(T) (ruling AO2): T is `any`, or an element, field or rest type of
/// T has `any`, at any depth.
pub fn has_any(t: &Ty) -> bool {
    t.any
        || t.list.as_ref().is_some_and(|e| has_any(e))
        || t.map
            .as_ref()
            .is_some_and(|r| r.fields.values().any(|f| has_any(&f.ty)) || has_any(&r.rest))
}

/// The values of a finite type (4.5), or `None` when it is not finite.
pub fn finite_values(t: &Ty) -> Option<Vec<FiniteValue>> {
    if t.any || t.num.is_some() || t.list.is_some() || t.map.is_some() || !t.opaque.is_empty() {
        return None;
    }
    let mut out = Vec::new();
    if t.null {
        out.push(FiniteValue::Null);
    }
    if t.boolean {
        out.push(FiniteValue::Bool(true));
        out.push(FiniteValue::Bool(false));
    }
    match &t.str {
        Some(Strs::All) => return None,
        Some(Strs::Lits(l)) => out.extend(l.iter().map(|s| FiniteValue::Str(s.clone()))),
        None => {}
    }
    Some(out)
}

#[derive(Clone, Debug, PartialEq)]
pub enum FiniteValue {
    Null,
    Bool(bool),
    Str(String),
}

// ---- printing (3.9) ---------------------------------------------------------

fn json_form(s: &str) -> String {
    let mut out = String::from("\"");
    for c in s.chars() {
        crate::messages::push_json_char(&mut out, c);
    }
    out.push('"');
    out
}

fn is_word(k: &str) -> bool {
    let b = k.as_bytes();
    !b.is_empty()
        && (b[0].is_ascii_alphabetic() || b[0] == b'_')
        && b.iter().all(|c| c.is_ascii_alphanumeric() || *c == b'_')
}

/// The number of parts the type prints at its top level.
fn parts(t: &Ty) -> usize {
    if t.any {
        return 1;
    }
    let lits = match &t.str {
        Some(Strs::Lits(l)) => l.len(),
        Some(Strs::All) => 1,
        None => 0,
    };
    t.kinds() - t.str.is_some() as usize + lits
}

pub fn print(t: &Ty) -> String {
    if t.any {
        return "any".into();
    }
    if t.is_never() {
        return "never".into();
    }
    let mut ps: Vec<String> = Vec::new();
    if t.boolean {
        ps.push("bool".into());
    }
    match t.num {
        Some(Num::Int) => ps.push("int".into()),
        Some(Num::Float) => ps.push("float".into()),
        None => {}
    }
    match &t.str {
        Some(Strs::All) => ps.push("string".into()),
        Some(Strs::Lits(l)) => ps.extend(l.iter().map(|s| json_form(s))),
        None => {}
    }
    if let Some(e) = &t.list {
        if parts(e) > 1 {
            ps.push(format!("({})[]", print(e)));
        } else {
            ps.push(format!("{}[]", print(e)));
        }
    }
    if let Some(r) = &t.map {
        ps.push(print_rec(r));
    }
    // A type variable of a signature being printed is held as an opaque name
    // that starts with VAR_MARK, so it sorts after every opaque name.
    ps.extend(
        t.opaque
            .iter()
            .map(|o| o.strip_prefix(VAR_MARK).unwrap_or(o).to_string()),
    );
    if t.null {
        ps.push("null".into());
    }
    ps.join(" | ")
}

fn print_rec(r: &Rec) -> String {
    if r.fields.is_empty() {
        if r.rest.is_never() {
            return "{}".into();
        }
        return format!("map<{}>", print(&r.rest));
    }
    let mut ps: Vec<String> = r
        .fields
        .iter()
        .map(|(k, f)| {
            let key = if is_word(k) { k.clone() } else { json_form(k) };
            format!(
                "{key}{}: {}",
                if f.optional { "?" } else { "" },
                print(&f.ty)
            )
        })
        .collect();
    if r.rest.any {
        ps.push("...".into());
    } else if !r.rest.is_never() {
        ps.push(format!("...{}", print(&r.rest)));
    }
    format!("{{{}}}", ps.join(", "))
}

// ---- type text to types ------------------------------------------------------

/// The mark of a type variable held as an opaque name, for printing a
/// signature (`f01-host-types.md` 5.2): no opaque name holds it, and it sorts
/// after every character an opaque name may have.
pub const VAR_MARK: char = '\u{10FFFF}';

/// The type of a signature's type text, for printing: each type variable
/// stands as itself, after the opaque names and before `null`.
pub fn sig_type(t: &text::Type, opaque: &[String]) -> Ty {
    let sol: Solutions = ('A'..='Z')
        .map(|c| (c, Ty::opaque(&format!("{VAR_MARK}{c}"))))
        .collect();
    substitute(t, opaque, &sol)
}

/// The solutions of a signature's type variables, while a call is solved.
pub type Solutions = HashMap<char, Ty>;

/// Read a type text as a type (3.3, 3.4). `opaque` is the environment's
/// declared opaque types. `vars` gives the solution of each `Var` (an
/// unbound one is `never`); with `None`, a `Var` is an error. The error is
/// the message of `bad-type`.
pub fn from_text(
    t: &text::Type,
    opaque: &[String],
    vars: Option<&Solutions>,
) -> Result<Ty, String> {
    let ty = convert(t, opaque, vars)?;
    if ty.too_big() {
        return Err(format!("the type is larger than {SIZE_LIMIT}"));
    }
    if ty.depth() > 64 {
        return Err("the type is deeper than 64".into());
    }
    Ok(ty)
}

/// A signature's type with its type variables replaced by their solutions
/// (13.5). No depth limit: the checker tests the depth where 20.4 says.
pub fn substitute(t: &text::Type, opaque: &[String], sol: &Solutions) -> Ty {
    convert(t, opaque, Some(sol)).unwrap_or_default()
}

fn convert(t: &text::Type, opaque: &[String], vars: Option<&Solutions>) -> Result<Ty, String> {
    Ok(match t {
        text::Type::Null => Ty::null(),
        text::Type::Bool => Ty::boolean(),
        text::Type::Int => Ty::int(),
        text::Type::Float => Ty::float(),
        text::Type::String => Ty::string(),
        text::Type::Any => Ty::any(),
        text::Type::Never => Ty::never(),
        text::Type::Lit(s) => Ty::lit(s),
        text::Type::MapOf(e) => Ty::map_of(convert(e, opaque, vars)?),
        text::Type::Record(r) => {
            let mut fields = BTreeMap::new();
            for f in &r.fields {
                let ty = convert(&f.ty, opaque, vars)?;
                if fields
                    .insert(
                        f.key.clone(),
                        Field {
                            ty,
                            optional: f.optional,
                        },
                    )
                    .is_some()
                {
                    return Err(format!("the key {:?} is written twice in a record", f.key));
                }
            }
            let rest = match &r.rest {
                Rest::Exact => Ty::never(),
                Rest::Any => Ty::any(),
                Rest::Typed(t) => convert(t, opaque, vars)?,
            };
            Ty::rec(Rec::new(fields, rest))
        }
        text::Type::Name(n) => {
            if !opaque.iter().any(|o| o == n) {
                return Err(format!("{n} is not a declared opaque type"));
            }
            Ty::opaque(n)
        }
        text::Type::Var(c) => match vars {
            Some(v) => v.get(c).cloned().unwrap_or_default(),
            None => return Err(format!("the type variable {c} stands outside a signature")),
        },
        text::Type::List(e) => Ty::list(convert(e, opaque, vars)?),
        text::Type::Union(alts) => {
            let mut acc = Ty::never();
            for a in alts {
                let t = convert(a, opaque, vars)?;
                if acc.map.is_some() && t.map.is_some() {
                    return Err("two records in one type".into());
                }
                join_into(&mut acc, &t);
            }
            acc
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::text::parse_type;

    fn ty(s: &str) -> Ty {
        from_text(&parse_type(s).unwrap(), &["Time".into()], None).unwrap()
    }

    #[test]
    fn printing_round_trips() {
        for s in [
            "never[]",
            "int[][]",
            "\"fail\" | \"pass\" | null",
            "int[] | null",
            "(int | null)[]",
            "{action: string, ...}",
            "{note?: string, owner: string | null}",
            "map<any>",
            "map<float>",
            "{}",
            "bool | int | string | int[] | {a: int} | Time | null",
            "{\"a-b\": int, if: string}",
            "{a: int, ...string}",
            "any",
            "never",
            "(\"a\" | \"b\")[]",
        ] {
            let t = ty(s);
            assert_eq!(print(&t), s);
            assert_eq!(ty(&print(&t)), t);
        }
    }

    #[test]
    fn join_laws() {
        let ts: Vec<Ty> = [
            "int",
            "float",
            "\"a\"",
            "string",
            "null",
            "int[]",
            "{a: int}",
            "{b?: string, ...}",
            "map<int>",
            "Time",
            "any",
            "never",
            "bool | null",
            "{a: \"x\", b: int}",
        ]
        .iter()
        .map(|s| ty(s))
        .collect();
        for a in &ts {
            assert_eq!(join(a, a), *a, "idempotent {}", print(a));
            assert_eq!(join(a, &Ty::never()), *a);
            assert_eq!(join(&truthy(a), &falsy(a)), *a, "truthy+falsy {}", print(a));
            for b in &ts {
                assert_eq!(join(a, b), join(b, a), "commutative");
                // Each side fits the join.
                assert!(fits(a, &join(a, b)).is_some(), "{} in join", print(a));
                for c in &ts {
                    assert_eq!(join(&join(a, b), c), join(a, &join(b, c)), "associative");
                }
            }
        }
        assert_eq!(print(&join(&ty("int"), &ty("float"))), "float");
        assert_eq!(
            print(&join(&ty("{a: int}"), &ty("{b: string}"))),
            "{a?: int, b?: string}"
        );
        assert_eq!(
            print(&join(&ty("{a: int}"), &ty("map<string>"))),
            "{a?: int | string, ...string}"
        );
    }

    #[test]
    fn fits_rules() {
        let f = |a: &str, b: &str| fits(&ty(a), &ty(b));
        assert_eq!(f("{a: int}", "map<int>"), Some(false));
        assert_eq!(f("{a: int}", "{a: int, ...}"), Some(false));
        assert_eq!(f("{a: int}", "{a: int, b?: string}"), Some(false));
        assert_eq!(f("{a: int, b: int}", "{a: int}"), None);
        assert_eq!(f("{a?: int}", "{a: int}"), None);
        assert_eq!(f("any", "int"), Some(true));
        assert_eq!(f("int", "any"), Some(false));
        assert_eq!(f("never", "int"), Some(false));
        assert_eq!(f("any[]", "int[]"), Some(true));
        assert_eq!(f("\"a\"", "\"a\" | \"b\""), Some(false));
        assert_eq!(f("string", "\"a\""), None);
        assert_eq!(f("int", "float"), Some(false));
        assert_eq!(f("float", "int"), None);
        assert_eq!(f("int | null", "int"), None);
    }

    #[test]
    fn merge_rules() {
        let r = |s: &str| ty(s).map.unwrap();
        // 7.6
        let m = merge(&r("{a: int}"), Some(&r("{b: string}")));
        assert_eq!(print(&Ty::rec(m)), "{a: int, b: string}");
        let m = merge(&r("{a: int}"), Some(&r("map<\"x\">")));
        assert_eq!(print(&Ty::rec(m)), "{a: int | \"x\", ...\"x\"}");
    }

    #[test]
    fn truthy_and_falsy() {
        assert_eq!(print(&falsy(&ty("{id: int}"))), "never");
        assert_eq!(print(&falsy(&ty("Time"))), "never");
        assert_eq!(print(&truthy(&ty("null"))), "never");
        assert_eq!(
            print(&falsy(&ty("string | int[] | {a?: int}"))),
            "int".replace("int", "\"\" | never[] | {}")
        );
        assert_eq!(print(&truthy(&ty("\"\" | \"a\""))), "\"a\"");
    }

    #[test]
    fn two_records_are_an_error() {
        let e = from_text(&parse_type("{a: int} | {b: string}").unwrap(), &[], None);
        assert!(e.is_err());
        let e = from_text(&parse_type("Order").unwrap(), &[], None);
        assert!(e.is_err());
        let e = from_text(&parse_type("{x: {a: int, a: int}}").unwrap(), &[], None);
        assert!(e.is_err());
    }
}
