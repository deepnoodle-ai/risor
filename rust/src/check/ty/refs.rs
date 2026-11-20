//! Recursive types (F17 12): references, the tables that hold their
//! definitions, and unfolding.
//!
//! A reference is an alternative of a type, as an opaque name is (12.1). It
//! names one item of a component: a table, made when the component is
//! resolved, and an index in it. Its definition is set on the table when
//! every item of the component is resolved; until then the reference is not
//! complete.
//!
//! A table's definitions hold the references of their own component weakly,
//! so a recursive type is no cycle of `Arc`s and nothing leaks. Every other
//! reference holds its table. A holder of types that lives long (a compiled
//! program, a probe, an environment) keeps the tables it reaches, and the
//! type API makes the references of the types it gives strong.

use super::{join, Rec, Strs, Ty};
use std::borrow::Cow;
use std::collections::{BTreeSet, HashMap, HashSet};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, OnceLock, Weak};

static NEXT_TABLE: AtomicU64 = AtomicU64::new(1);

/// The definitions of the recursive items of one component.
pub struct Table {
    id: u64,
    defs: OnceLock<Box<[Def]>>,
}

struct Def {
    /// The definition, one level deep (12.2 item 4).
    ty: Ty,
    /// The definition with each reference at its top replaced by that
    /// reference's unfolding: what unfolding puts in the reference's place.
    top: Ty,
}

impl std::fmt::Debug for Table {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "Table({})", self.id)
    }
}

impl Table {
    pub fn new() -> Arc<Table> {
        Arc::new(Table {
            id: NEXT_TABLE.fetch_add(1, Ordering::Relaxed),
            defs: OnceLock::new(),
        })
    }

    pub fn id(&self) -> u64 {
        self.id
    }

    /// Set the definitions, in the order of the references' indexes. The
    /// references of this table within them become weak.
    pub fn set(&self, defs: Vec<Ty>) {
        let id = self.id;
        let mut memo = Memo::default();
        let mut out = Vec::with_capacity(defs.len());
        for d in &defs {
            let ty = map_refs(
                d,
                &mut |r: &Ref| (r.table == id).then(|| r.weak()),
                &mut memo,
            );
            // A definition holds no reference of its own component at its
            // top (12.2 item 4); one of another component is complete.
            let mut top = ty.clone();
            let refs = std::mem::take(&mut top.refs);
            for r in refs {
                match (r.table != id).then(|| r.unfolding()).flatten() {
                    Some(u) => add_alts(&mut top, &u),
                    None => {
                        top.refs.insert(r);
                    }
                }
            }
            out.push(Def { ty, top });
        }
        let _ = self.defs.set(out.into_boxed_slice());
    }

    fn def(&self, idx: u32) -> Option<&Def> {
        self.defs.get().and_then(|d| d.get(idx as usize))
    }
}

/// A reference (F17 12.1). Two references are the same when they are of one
/// declaration: one table and one index. They sort by name, as they print.
#[derive(Clone)]
pub struct Ref {
    link: Link,
    table: u64,
    idx: u32,
    name: Arc<str>,
}

#[derive(Clone)]
enum Link {
    Strong(Arc<Table>),
    Weak(Weak<Table>),
    /// A name compared by its spelling, with no definition: a case's `type`
    /// (12.6 item 5).
    Atom,
}

impl Ref {
    pub fn new(table: &Arc<Table>, idx: u32, name: &str) -> Ref {
        Ref {
            link: Link::Strong(table.clone()),
            table: table.id,
            idx,
            name: name.into(),
        }
    }

    /// A name with no definition, compared by its spelling.
    pub fn atom(name: &str) -> Ref {
        Ref {
            link: Link::Atom,
            table: 0,
            idx: 0,
            name: name.into(),
        }
    }

    pub fn name(&self) -> &str {
        &self.name
    }

    pub fn table_id(&self) -> u64 {
        self.table
    }

    pub fn index(&self) -> u32 {
        self.idx
    }

    /// The table, while it lives.
    pub fn table(&self) -> Option<Arc<Table>> {
        match &self.link {
            Link::Strong(t) => Some(t.clone()),
            Link::Weak(w) => {
                let t = w.upgrade();
                debug_assert!(t.is_some(), "the table of {} was dropped", self.name);
                t
            }
            Link::Atom => None,
        }
    }

    fn with_def<R>(&self, f: impl FnOnce(&Def) -> R) -> Option<R> {
        match &self.link {
            Link::Strong(t) => t.def(self.idx).map(f),
            Link::Weak(_) => {
                let t = self.table()?;
                let r = t.def(self.idx).map(f);
                r
            }
            Link::Atom => None,
        }
    }

    /// Whether its definition is known (12.1).
    pub fn complete(&self) -> bool {
        self.with_def(|_| ()).is_some()
    }

    /// Its definition, one level deep.
    pub fn definition(&self) -> Option<Ty> {
        self.with_def(|d| d.ty.clone())
    }

    /// What unfolding puts in its place: its definition, with no reference
    /// at its top.
    pub fn unfolding(&self) -> Option<Ty> {
        self.with_def(|d| d.top.clone())
    }

    /// The same reference, holding its table.
    pub fn strong(&self) -> Ref {
        match &self.link {
            Link::Weak(_) => match self.table() {
                Some(t) => Ref {
                    link: Link::Strong(t),
                    ..self.clone()
                },
                None => self.clone(),
            },
            _ => self.clone(),
        }
    }

    fn weak(&self) -> Ref {
        match &self.link {
            Link::Strong(t) => Ref {
                link: Link::Weak(Arc::downgrade(t)),
                ..self.clone()
            },
            _ => self.clone(),
        }
    }
}

impl PartialEq for Ref {
    fn eq(&self, o: &Ref) -> bool {
        self.table == o.table && self.idx == o.idx && self.name == o.name
    }
}

impl Eq for Ref {}

impl PartialOrd for Ref {
    fn partial_cmp(&self, o: &Ref) -> Option<std::cmp::Ordering> {
        Some(self.cmp(o))
    }
}

impl Ord for Ref {
    fn cmp(&self, o: &Ref) -> std::cmp::Ordering {
        (&*self.name, self.table, self.idx).cmp(&(&*o.name, o.table, o.idx))
    }
}

impl std::hash::Hash for Ref {
    fn hash<H: std::hash::Hasher>(&self, h: &mut H) {
        self.table.hash(h);
        self.idx.hash(h);
    }
}

impl std::fmt::Debug for Ref {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.name)
    }
}

impl Ty {
    /// Whether a reference is among its alternatives.
    pub fn has_refs(&self) -> bool {
        !self.refs.is_empty()
    }

    /// The unfolding (F17 12.1): each complete reference among its
    /// alternatives replaced by the alternatives of its definition. A rule
    /// that reads a type's alternatives, kinds, record, element or variants
    /// reads this (12.5).
    pub fn view(&self) -> Cow<'_, Ty> {
        if self.refs.is_empty() {
            Cow::Borrowed(self)
        } else {
            Cow::Owned(unfold(self))
        }
    }

    /// The type with no reference among its alternatives.
    pub fn without_refs(&self) -> Ty {
        let mut t = self.clone();
        t.refs = BTreeSet::new();
        t
    }
}

/// The unfolding of a type. A reference that is not complete stays.
pub fn unfold(t: &Ty) -> Ty {
    if t.refs.is_empty() {
        return t.clone();
    }
    let mut out = t.without_refs();
    for r in &t.refs {
        match r.unfolding() {
            Some(u) => add_alts(&mut out, &u),
            None => {
                if !out.any {
                    out.refs.insert(r.clone());
                }
            }
        }
    }
    out
}

/// Put the alternatives of `u` beside those of `t`. Two of one kind are
/// joined; the unfolding of a resolved type meets none.
pub fn add_alts(t: &mut Ty, u: &Ty) {
    if t.any {
        return;
    }
    if u.any {
        *t = Ty::any();
        return;
    }
    if (t.list.is_some() && u.list.is_some()) || (t.map.is_some() && u.map.is_some()) {
        let refs: BTreeSet<Ref> = t.refs.union(&u.refs).cloned().collect();
        let j = join(&t.without_refs(), &u.without_refs());
        *t = j;
        if !t.any {
            t.refs = refs;
        }
        return;
    }
    t.null |= u.null;
    t.boolean |= u.boolean;
    t.num = match (t.num, u.num) {
        (Some(x), Some(y)) => Some(x.max(y)),
        (x, y) => x.or(y),
    };
    match (&mut t.str, &u.str) {
        (Some(Strs::Lits(a)), Some(Strs::Lits(b))) => a.extend(b.iter().cloned()),
        (Some(a), Some(_)) => *a = Strs::All,
        (a @ None, b) => *a = b.clone(),
        (Some(_), None) => {}
    }
    if t.list.is_none() {
        t.list = u.list.clone();
    }
    if t.map.is_none() {
        t.map = u.map.clone();
        t.variants = u.variants.clone();
    }
    t.opaque.extend(u.opaque.iter().cloned());
    t.refs.extend(u.refs.iter().cloned());
}

/// The kinds of a type's unfolding other than null, for "shares a kind"
/// (F17 12.2 items 5 and 6, 12.5 item 4).
#[derive(Default)]
pub struct Kinds {
    any: bool,
    boolean: bool,
    num: bool,
    str: bool,
    list: bool,
    map: bool,
    null: bool,
    opaque: BTreeSet<String>,
}

impl Kinds {
    /// The kinds of a type: a reference counts its definition's, and one
    /// that is not complete counts none.
    pub fn of(t: &Ty) -> Kinds {
        let v = t.view();
        Kinds {
            any: v.any,
            boolean: v.boolean,
            num: v.num.is_some(),
            str: v.str.is_some(),
            list: v.list.is_some(),
            map: v.map.is_some(),
            null: v.null,
            opaque: v.opaque.clone(),
        }
    }

    /// Whether the two share a kind; `null` counts when `with_null`.
    pub fn shares(&self, o: &Kinds, with_null: bool) -> bool {
        (self.boolean && o.boolean)
            || (self.num && o.num)
            || (self.str && o.str)
            || (self.list && o.list)
            || (self.map && o.map)
            || (with_null && self.null && o.null)
            || !self.opaque.is_disjoint(&o.opaque)
    }

    pub fn is_any(&self) -> bool {
        self.any
    }
}

/// The memo of a walk that rebuilds a type, by the address of each part, so
/// a part shared in the type is rebuilt once.
#[derive(Default)]
pub struct Memo {
    lists: HashMap<*const Ty, Arc<Ty>>,
    maps: HashMap<*const Rec, Arc<Rec>>,
    variants: HashMap<*const Vec<Rec>, Arc<Vec<Rec>>>,
}

/// The type with each reference, at any depth, replaced by what `f` gives
/// (itself when `None`). Definitions are not followed. The recursion follows
/// the type's depth.
pub fn map_refs(t: &Ty, f: &mut dyn FnMut(&Ref) -> Option<Ref>, memo: &mut Memo) -> Ty {
    let mut out = t.clone();
    if !t.refs.is_empty() {
        out.refs = t
            .refs
            .iter()
            .map(|r| f(r).unwrap_or_else(|| r.clone()))
            .collect();
    }
    if let Some(e) = &t.list {
        let k = Arc::as_ptr(e);
        let n = match memo.lists.get(&k) {
            Some(n) => n.clone(),
            None => {
                let n = Arc::new(map_refs(e, f, memo));
                memo.lists.insert(k, n.clone());
                n
            }
        };
        out.list = Some(n);
    }
    if let Some(r) = &t.map {
        let k = Arc::as_ptr(r);
        let n = match memo.maps.get(&k) {
            Some(n) => n.clone(),
            None => {
                let n = Arc::new(map_rec(r, f, memo));
                memo.maps.insert(k, n.clone());
                n
            }
        };
        out.map = Some(n);
    }
    if let Some(vs) = &t.variants {
        let k = Arc::as_ptr(vs);
        let n = match memo.variants.get(&k) {
            Some(n) => n.clone(),
            None => {
                let n = Arc::new(vs.iter().map(|v| map_rec(v, f, memo)).collect::<Vec<_>>());
                memo.variants.insert(k, n.clone());
                n
            }
        };
        out.variants = Some(n);
    }
    out
}

fn map_rec(r: &Rec, f: &mut dyn FnMut(&Ref) -> Option<Ref>, memo: &mut Memo) -> Rec {
    let mut out = r.clone();
    for fl in out.fields.values_mut() {
        fl.ty = map_refs(&fl.ty, f, memo);
    }
    out.rest = map_refs(&r.rest, f, memo);
    for (b, fl) in out.fills.values_mut() {
        *b = map_refs(b, f, memo);
        *fl = map_refs(fl, f, memo);
    }
    out
}

/// The type with every reference, at any depth, holding its table.
pub fn strengthen(t: &Ty) -> Ty {
    if !has_refs_deep(t) {
        return t.clone();
    }
    map_refs(t, &mut |r: &Ref| Some(r.strong()), &mut Memo::default())
}

/// Whether a reference stands anywhere in the type, definitions not
/// followed.
pub fn has_refs_deep(t: &Ty) -> bool {
    let mut seen: HashSet<usize> = HashSet::new();
    let mut stack: Vec<&Ty> = vec![t];
    while let Some(t) = stack.pop() {
        if !t.refs.is_empty() {
            return true;
        }
        if let Some(e) = &t.list {
            if seen.insert(Arc::as_ptr(e) as usize) {
                stack.push(e);
            }
        }
        let recs: Vec<&Rec> = match (&t.variants, &t.map) {
            (Some(vs), Some(m)) => vs.iter().chain(std::iter::once(m.as_ref())).collect(),
            (None, Some(m)) => vec![m.as_ref()],
            _ => Vec::new(),
        };
        for r in recs {
            if !seen.insert(r as *const Rec as usize) {
                continue;
            }
            stack.extend(r.fields.values().map(|f| &f.ty));
            stack.push(&r.rest);
            for (b, f) in r.fills.values() {
                stack.push(b);
                stack.push(f);
            }
        }
    }
    false
}

/// Each reference that the types reach, following definitions, once each,
/// in the order met. The walk keeps its own stack, so a long chain of
/// definitions needs no native recursion.
pub fn reach<'a>(ts: impl Iterator<Item = &'a Ty>) -> Vec<Ref> {
    let mut found: Vec<Ref> = Vec::new();
    let mut seen_refs: HashSet<(u64, u32)> = HashSet::new();
    let mut seen: HashSet<usize> = HashSet::new();
    // Definitions are cloned out of their tables; the tables are kept here
    // so that the addresses of their parts stay unique during the walk.
    let mut keep: Vec<Ty> = Vec::new();
    let mut stack: Vec<Ty> = ts.cloned().collect();
    while let Some(t) = stack.pop() {
        for r in &t.refs {
            if seen_refs.insert((r.table, r.idx)) {
                found.push(r.strong());
                if let Some(d) = r.definition() {
                    stack.push(d);
                }
            }
        }
        if let Some(e) = &t.list {
            if seen.insert(Arc::as_ptr(e) as usize) {
                stack.push(e.as_ref().clone());
            }
        }
        let mut recs: Vec<Rec> = Vec::new();
        if let Some(vs) = &t.variants {
            if seen.insert(Arc::as_ptr(vs) as usize) {
                recs.extend(vs.iter().cloned());
            }
        }
        if let Some(m) = &t.map {
            if seen.insert(Arc::as_ptr(m) as usize) {
                recs.push(m.as_ref().clone());
            }
        }
        for r in recs {
            stack.extend(r.fields.values().map(|f| f.ty.clone()));
            stack.push(r.rest.clone());
        }
        keep.push(t);
    }
    found
}

/// Whether the definition can be falsy for Truthy (F17 12.5 item 8): it has
/// `null`, the literal `""`, `never[]` or a record that can be empty.
pub fn can_be_falsy(u: &Ty) -> bool {
    u.null
        || matches!(&u.str, Some(Strs::Lits(l)) if l.contains(""))
        || u.list.as_ref().is_some_and(|e| e.is_never())
        || u.variants()
            .iter()
            .any(|r| r.fields.values().all(|f| f.optional))
}

/// F17 12.2 item 6: the first reference of a definition that shares a kind
/// other than null with another alternative of a type where it stands, in
/// a walk that takes a type before its parts. References are not followed.
pub fn beside_its_kind(t: &Ty) -> Option<String> {
    if !t.refs.is_empty() {
        for r in &t.refs {
            let own = Kinds::of(&Ty {
                refs: [r.clone()].into(),
                ..Ty::default()
            });
            let mut others = t.clone();
            others.refs.remove(r);
            if own.shares(&Kinds::of(&others), false) {
                return Some(r.name().to_string());
            }
        }
    }
    if let Some(e) = &t.list {
        if let Some(n) = beside_its_kind(e) {
            return Some(n);
        }
    }
    for r in t.variants() {
        for f in r.fields.values() {
            if let Some(n) = beside_its_kind(&f.ty) {
                return Some(n);
            }
        }
        if let Some(n) = beside_its_kind(&r.rest) {
            return Some(n);
        }
    }
    None
}
