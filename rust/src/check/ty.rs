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

/// The type size limit (`05-types.md` 20.7).
pub const SIZE_LIMIT: u64 = 100_000;

/// A record's size, kept with it once asked for (`05-types.md` 20.7). A clone starts
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
    /// The record of the map alternative. For a tagged union (F5 2.2) it is
    /// flat(U) (F5 2.3), which every read of the union sees (F5 4.3, 5.1).
    pub map: Option<Arc<Rec>>,
    /// F5: when the map alternative is a tagged union, its variants, two or
    /// more, in the order of printing (F5 2.4). `map` is then their Join.
    pub variants: Option<Arc<Vec<Rec>>>,
    pub opaque: BTreeSet<String>,
}

/// The type of a key in a record: its field's type, else the rest (F4 6.2).
pub fn key_ty(r: &Rec, k: &str) -> Ty {
    r.fields
        .get(k)
        .map_or_else(|| r.rest.clone(), |f| f.ty.clone())
}

/// The tag of records (F5 2.1): the least key that each lists as required,
/// typed by a literal set and nothing else, with no string in two of the
/// sets. A key that any record fills (F1 4.4) is no tag (F5 8.1 item 6).
pub fn find_tag(recs: &[&Rec]) -> Option<String> {
    if recs.len() < 2 {
        return None;
    }
    'keys: for k in recs[0].fields.keys() {
        let mut seen: BTreeSet<&String> = BTreeSet::new();
        for r in recs {
            let Some(f) = r.fields.get(k) else {
                continue 'keys;
            };
            if f.optional || r.fills.contains_key(k) || f.ty.any || f.ty.kinds() != 1 {
                continue 'keys;
            }
            let Some(Strs::Lits(set)) = &f.ty.str else {
                continue 'keys;
            };
            if set.is_empty() {
                continue 'keys;
            }
            for x in set {
                if !seen.insert(x) {
                    continue 'keys;
                }
            }
        }
        return Some(k.clone());
    }
    None
}

/// The tag set of a variant under a tag.
fn tag_set<'a>(r: &'a Rec, tag: &str) -> &'a BTreeSet<String> {
    static EMPTY: std::sync::OnceLock<BTreeSet<String>> = std::sync::OnceLock::new();
    match r.fields.get(tag).and_then(|f| f.ty.str.as_ref()) {
        Some(Strs::Lits(l)) => l,
        _ => EMPTY.get_or_init(BTreeSet::new),
    }
}

/// The map alternative a list of variants gives (F5 4.5 item 2).
enum UnionOf {
    None,
    /// The record, and the variants of a union (two or more) or none.
    Map(Rec, Option<Vec<Rec>>),
    /// Two or more records that form no tagged union.
    NoTag,
}

fn union_of(recs: Vec<Rec>) -> UnionOf {
    match recs.len() {
        0 => UnionOf::None,
        1 => UnionOf::Map(recs.into_iter().next().expect("one record"), None),
        _ => {
            let refs: Vec<&Rec> = recs.iter().collect();
            let Some(tag) = find_tag(&refs) else {
                return UnionOf::NoTag;
            };
            let mut vs = recs;
            vs.sort_by(|a, b| {
                let x = tag_set(a, &tag).iter().next();
                let y = tag_set(b, &tag).iter().next();
                x.cmp(&y)
            });
            let mut flat = vs[0].clone();
            for v in &vs[1..] {
                flat = join_rec(&flat, v);
            }
            UnionOf::Map(flat, Some(vs))
        }
    }
}

impl Ty {
    /// A type whose map alternative is a list of variants; `None` when they
    /// form no union.
    pub fn of_variants(recs: Vec<Rec>) -> Option<Ty> {
        match union_of(recs) {
            UnionOf::NoTag => None,
            UnionOf::None => Some(Ty::never()),
            UnionOf::Map(r, vs) => Some(Ty {
                map: Some(Arc::new(r)),
                variants: vs.map(Arc::new),
                ..Ty::default()
            }),
        }
    }

    /// The variants of the map alternative (F5 1.1): a union's, or the record
    /// alone.
    pub fn variants(&self) -> Vec<&Rec> {
        match (&self.variants, &self.map) {
            (Some(vs), _) => vs.iter().collect(),
            (None, Some(r)) => vec![r.as_ref()],
            _ => Vec::new(),
        }
    }

    /// The tag of a union.
    pub fn tag(&self) -> Option<String> {
        let vs = self.variants.as_ref()?;
        find_tag(&vs.iter().collect::<Vec<_>>())
    }

    /// This type with its map alternative replaced by a list of variants
    /// (F5 4.5 item 2); a list that forms no union falls back to its Join.
    pub fn with_variants(&self, recs: Vec<Rec>) -> Ty {
        let mut t = self.clone();
        t.variants = None;
        t.map = None;
        match union_of(recs.clone()) {
            UnionOf::Map(r, vs) => {
                t.map = Some(Arc::new(r));
                t.variants = vs.map(Arc::new);
            }
            UnionOf::None => {}
            UnionOf::NoTag => {
                let mut flat = recs[0].clone();
                for v in &recs[1..] {
                    flat = join_rec(&flat, v);
                }
                t.map = Some(Arc::new(flat));
            }
        }
        t
    }
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
        match &self.variants {
            // F5 2.6: the sum of the variants' sizes; flat is not counted.
            Some(vs) => {
                for v in vs.iter() {
                    n = n.saturating_add(v.size());
                }
            }
            None => {
                if let Some(r) = &self.map {
                    n = n.saturating_add(r.size());
                }
            }
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
            (Some(_), Some(_)) if a.variants.is_some() && a.variants == b.variants => a.map.clone(),
            (Some(x), Some(y)) => Some(Arc::new(join_rec(x, y))),
            (x, y) => x.clone().or(y.clone()),
        },
        // F5 4.1: a union survives only beside the same union; Join never
        // makes one.
        variants: match (&a.map, &b.map) {
            (Some(_), Some(_)) => a.variants.clone().filter(|_| a.variants == b.variants),
            (Some(_), None) => a.variants.clone(),
            (None, Some(_)) => b.variants.clone(),
            _ => None,
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
/// n literals is n log n work, not n squared.
pub fn join_all<'a>(ts: impl Iterator<Item = &'a Ty>) -> Ty {
    let mut acc = Ty::never();
    for t in ts {
        join_into(&mut acc, t);
    }
    acc
}

/// `acc = Join(acc, t)`, with the literal set of `acc` extended in place: the work is in the size of `t`, not of `acc`.
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
    let same = acc.variants.is_some() && acc.variants == t.variants;
    acc.variants = match (&acc.map, &t.map) {
        (Some(_), Some(_)) => acc.variants.clone().filter(|_| same),
        (Some(_), None) => acc.variants.clone(),
        (None, Some(_)) => t.variants.clone(),
        _ => None,
    };
    acc.map = match (&acc.map, &t.map) {
        (Some(_), Some(_)) if same => acc.map.clone(),
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
    if s.map.is_some() {
        t.map.as_ref()?;
        // F5 4.2: each variant of S fits some variant of T; the fit uses any
        // when some variant fits none without using any.
        let tv = t.variants();
        for sv in s.variants() {
            let mut best: Option<bool> = None;
            for x in &tv {
                match fits_rec(sv, x) {
                    Some(false) => {
                        best = Some(false);
                        break;
                    }
                    Some(true) => best = Some(true),
                    None => {}
                }
            }
            used |= best?;
        }
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
        // F5 4.4: a union's truthy part is the union.
        variants: t.variants.clone(),
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
        variants: None,
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

/// has-any(T) (`05-types.md` 7.3): T is `any`, or an element, field or rest type of
/// T has `any`, at any depth.
pub fn has_any(t: &Ty) -> bool {
    t.any
        || t.list.as_ref().is_some_and(|e| has_any(e))
        || t.map
            .as_ref()
            .is_some_and(|r| r.fields.values().any(|f| has_any(&f.ty)) || has_any(&r.rest))
}

/// disjoint(A, B) (F4 6.2): true when no value conforms to both. Sound and
/// not complete.
pub fn disjoint(a: &Ty, b: &Ty) -> bool {
    if a.is_never() || b.is_never() {
        return true;
    }
    if a.any || b.any {
        return false;
    }
    if (a.null && b.null) || (a.boolean && b.boolean) || (a.num.is_some() && b.num.is_some()) {
        return false;
    }
    if !a.opaque.is_disjoint(&b.opaque) {
        return false;
    }
    match (&a.str, &b.str) {
        (Some(Strs::Lits(x)), Some(Strs::Lits(y))) if x.is_disjoint(y) => {}
        (Some(_), Some(_)) => return false,
        _ => {}
    }
    if a.list.is_some() && b.list.is_some() {
        return false;
    }
    if a.map.is_some() && b.map.is_some() {
        // F5 4.5: they share a value unless every variant of A is disjoint
        // from every variant of B.
        return a
            .variants()
            .iter()
            .all(|x| b.variants().iter().all(|y| disjoint_rec(x, y)));
    }
    true
}

/// The record row of disjoint (F4 6.2).
fn disjoint_rec(x: &Rec, y: &Rec) -> bool {
    {
        let field = |r: &Rec, k: &str| r.fields.get(k).map_or(r.rest.clone(), |f| f.ty.clone());
        let required = x
            .fields
            .iter()
            .chain(y.fields.iter())
            .filter(|(_, f)| !f.optional)
            .map(|(k, _)| k);
        for k in required {
            if disjoint(&field(x, k), &field(y, k)) {
                return true;
            }
        }
        false
    }
}

/// without(X, T) (F4 6.2): what is left of X when a value of type X is
/// known not to conform to T.
pub fn without(x: &Ty, t: &Ty) -> Ty {
    if x.any {
        return Ty::any();
    }
    let sure = |a: Ty| fits(&a, t) == Some(false);
    let mut out = x.clone();
    if out.null && sure(Ty::null()) {
        out.null = false;
    }
    if out.boolean && sure(Ty::boolean()) {
        out.boolean = false;
    }
    if let Some(n) = out.num {
        let one = Ty {
            num: Some(n),
            ..Ty::default()
        };
        if sure(one) {
            out.num = None;
        }
    }
    if let Some(st) = out.str.clone() {
        let one = Ty {
            str: Some(st.clone()),
            ..Ty::default()
        };
        if sure(one) {
            out.str = None;
        } else if let (Strs::Lits(l), Some(Strs::Lits(tl))) = (&st, &t.str) {
            let left: BTreeSet<String> = l.difference(tl).cloned().collect();
            out.str = if left.is_empty() {
                None
            } else {
                Some(Strs::Lits(left))
            };
        }
    }
    if let Some(e) = &out.list {
        if sure(Ty::list(e.as_ref().clone())) {
            out.list = None;
        }
    }
    if out.map.is_some() {
        // F5 4.5: the map alternative loses each variant that surely fits T.
        let left: Vec<Rec> = x
            .variants()
            .into_iter()
            .filter(|v| !sure(Ty::rec((*v).clone())))
            .cloned()
            .collect();
        if left.len() != x.variants().len() {
            out = out.with_variants(left);
        }
    }
    let opaque: BTreeSet<String> = out
        .opaque
        .iter()
        .filter(|o| !sure(Ty::opaque(o)))
        .cloned()
        .collect();
    out.opaque = opaque;
    out
}

/// Unite(A, B) (F5 4.5): the Join, but variants of the two map
/// alternatives that form a tagged union stay a union.
pub fn unite(a: &Ty, b: &Ty) -> Ty {
    let j = join(a, b);
    if j.any || a.map.is_none() || b.map.is_none() {
        return j;
    }
    let mut vs: Vec<Rec> = Vec::new();
    for v in a.variants().into_iter().chain(b.variants()) {
        if !vs.iter().any(|w| w == v) {
            vs.push(v.clone());
        }
    }
    if vs.len() >= 2 {
        if let Some(t) = Ty::of_variants(vs) {
            let mut out = j;
            out.map = t.map;
            out.variants = t.variants;
            return out;
        }
    }
    j
}

/// hit(M, f, S) (F5 4.5): the variants in which the type of f is not
/// disjoint from the literal set of S.
pub fn hit(t: &Ty, f: &str, s: &BTreeSet<String>) -> Vec<Rec> {
    let lits = Ty {
        str: Some(Strs::Lits(s.clone())),
        ..Ty::default()
    };
    t.variants()
        .into_iter()
        .filter(|v| !disjoint(&key_ty(v, f), &lits))
        .cloned()
        .collect()
}

/// all(M, f, S) (F5 4.5): the variants that list f as required with a type
/// that surely fits the literal set of S.
pub fn all_of(t: &Ty, f: &str, s: &BTreeSet<String>) -> Vec<Rec> {
    let lits = Ty {
        str: Some(Strs::Lits(s.clone())),
        ..Ty::default()
    };
    t.variants()
        .into_iter()
        .filter(|v| {
            v.fields
                .get(f)
                .is_some_and(|fl| !fl.optional && fits(&fl.ty, &lits) == Some(false))
        })
        .cloned()
        .collect()
}

/// The type of a list of variants (F5 4.5 item 2).
pub fn variants_ty(vs: Vec<Rec>) -> Ty {
    Ty::never().with_variants(vs)
}

/// The alternatives of one `Type`, joined (`05-types.md` 3.4), with their
/// records forming the map alternative (F5 3.1): one record, or a tagged
/// union of two or more. `None` when two or more records form none.
pub fn union_of_alts(ts: &[Ty]) -> Option<Ty> {
    let mut acc = Ty::never();
    let mut recs: Vec<Rec> = Vec::new();
    for t in ts {
        recs.extend(t.variants().into_iter().cloned());
        let mut rest = t.clone();
        rest.map = None;
        rest.variants = None;
        join_into(&mut acc, &rest);
    }
    if acc.any {
        return Some(acc);
    }
    if recs.len() >= 2 {
        let u = Ty::of_variants(recs)?;
        acc.map = u.map;
        acc.variants = u.variants;
    } else if let Some(r) = recs.pop() {
        acc.map = Some(Arc::new(r));
    }
    Some(acc)
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
    let vars = t.variants.as_ref().map_or(0, |v| v.len().saturating_sub(1));
    t.kinds() - t.str.is_some() as usize + lits + vars
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
    // F5 2.4: a union's variants stand where the record stands.
    match &t.variants {
        Some(vs) => ps.extend(vs.iter().map(print_rec)),
        None => {
            if let Some(r) = &t.map {
                ps.push(print_rec(r));
            }
        }
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
pub fn sig_type<N: TypeNames + ?Sized>(t: &text::Type, opaque: &N) -> Ty {
    let sol: Solutions = ('A'..='Z')
        .map(|c| (c, Ty::opaque(&format!("{VAR_MARK}{c}"))))
        .collect();
    substitute(t, opaque, &sol)
}

/// The type names an environment gives (`03-host.md` 3.3.1; F4 3): its
/// opaque types and, with F4, its named types.
pub trait TypeNames {
    fn lookup(&self, name: &str) -> Option<Ty>;
}

impl TypeNames for [String] {
    fn lookup(&self, name: &str) -> Option<Ty> {
        self.iter().any(|o| o == name).then(|| Ty::opaque(name))
    }
}

impl<const N: usize> TypeNames for [String; N] {
    fn lookup(&self, name: &str) -> Option<Ty> {
        self.as_slice().lookup(name)
    }
}

impl TypeNames for Vec<String> {
    fn lookup(&self, name: &str) -> Option<Ty> {
        self.as_slice().lookup(name)
    }
}

/// The opaque types and the named types of an environment (F4 3.1). A named
/// type's type is held once and shared.
#[derive(Clone, Debug, Default)]
pub struct Names {
    pub opaque: Vec<String>,
    pub named: Vec<(String, Ty)>,
}

impl TypeNames for Names {
    fn lookup(&self, name: &str) -> Option<Ty> {
        if self.opaque.iter().any(|o| o == name) {
            return Some(Ty::opaque(name));
        }
        self.named
            .iter()
            .find(|(n, _)| n == name)
            .map(|(_, t)| t.clone())
    }
}

/// The solutions of a signature's type variables, while a call is solved.
pub type Solutions = HashMap<char, Ty>;

/// Read a type text as a type (3.3, 3.4). `opaque` is the environment's
/// declared opaque types. `vars` gives the solution of each `Var` (an
/// unbound one is `never`); with `None`, a `Var` is an error. The error is
/// the message of `bad-type`.
pub fn from_text(
    t: &text::Type,
    opaque: &(impl TypeNames + ?Sized),
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
pub fn substitute<N: TypeNames + ?Sized>(t: &text::Type, opaque: &N, sol: &Solutions) -> Ty {
    convert(t, opaque, Some(sol)).unwrap_or_default()
}

fn convert<N: TypeNames + ?Sized>(
    t: &text::Type,
    opaque: &N,
    vars: Option<&Solutions>,
) -> Result<Ty, String> {
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
        text::Type::Name(n) => match opaque.lookup(n) {
            Some(t) => t,
            None => return Err(format!("{n} is not a declared opaque type or named type")),
        },
        text::Type::Var(c) => match vars {
            Some(v) => v.get(c).cloned().unwrap_or_default(),
            None => return Err(format!("the type variable {c} stands outside a signature")),
        },
        text::Type::List(e) => Ty::list(convert(e, opaque, vars)?),
        text::Type::Union(alts) => {
            let ts: Vec<Ty> = alts
                .iter()
                .map(|a| convert(a, opaque, vars))
                .collect::<Result<_, _>>()?;
            union_of_alts(&ts)
                .ok_or_else(|| "two records in one type that form no tagged union".to_string())?
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
