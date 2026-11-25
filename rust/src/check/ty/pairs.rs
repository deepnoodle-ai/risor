//! The checker's computations (F17 12.5): Fits, Join, Equal and disjoint,
//! which follow references by pairs, with the bound of 4,096 pairs entered.
//!
//! A computation runs on an explicit stack of the pairs it has entered, so
//! its native recursion is that of one pair's rule, which the depth of a
//! type bounds, and not the number of pairs in progress, which the bound
//! allows to reach 4,096. When a rule meets a pair it must enter, it stops
//! with `Need`; the machine pushes the pair, runs its rule to the end, and
//! runs the rule that met it again from its start. The outcomes of the
//! pairs that rule met before are replayed from its log, in order, so the
//! second run takes the same path and goes on past the pair. Each outcome a
//! rule meets that depends on the computation's state (a pair in progress,
//! a pair that has failed, a pair entered) is in the log; the rest of a rule
//! depends on its two types alone.

use super::refs::{add_alts, Kinds};
use super::{Field, Num, Rec, SizeCache, Strs, Ty};
use std::cell::Cell;
use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::hash::{Hash, Hasher};
use std::sync::Arc;

/// The number of pairs a computation enters at most (12.5 item 7).
pub const BOUND: u32 = 4096;

thread_local! {
    static PASSED: Cell<bool> = const { Cell::new(false) };
    /// Whether a computation entered a pair with a reference that is not
    /// complete: it looked into a reference whose definition is not known
    /// (F17 12.4 item 4).
    static LOOKED: Cell<bool> = const { Cell::new(false) };
}

/// Fits(S, T) as [`super::fits`], or `Err(())` when the computation would
/// look into a reference that is not complete: one that a schema's mapping
/// has met, whose definition is not known yet (F17 12.4 item 4, R312). It
/// looks into one only when it enters a pair that holds it.
pub fn fits_looking(s: &Ty, t: &Ty) -> Result<Option<bool>, ()> {
    let prev = LOOKED.with(|l| l.replace(false));
    let r = super::fits(s, t);
    let looked = LOOKED.with(|l| l.replace(prev));
    if looked {
        Err(())
    } else {
        Ok(r)
    }
}

/// A watch on the bound: whether a computation passed it since the watch
/// began. Watches nest; each sees the computations made while it lives, and
/// an outer one does not see what an inner one saw.
pub struct Watch {
    prev: bool,
}

/// Begin to watch the bound.
pub fn watch() -> Watch {
    Watch {
        prev: PASSED.with(|p| p.replace(false)),
    }
}

impl Watch {
    /// Whether a computation passed the bound while this watch lived.
    pub fn passed(&self) -> bool {
        PASSED.with(|p| p.get())
    }
}

impl Drop for Watch {
    fn drop(&mut self) {
        PASSED.with(|p| p.set(self.prev));
    }
}

/// Whether a computation passed the bound since the innermost watch began.
pub fn passed() -> bool {
    PASSED.with(|p| p.get())
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(super) enum Kind {
    Fits,
    Join,
    Equal,
    Disjoint,
}

/// The outcome of a pair.
#[derive(Clone, Debug)]
pub(super) enum Out {
    Fit(Option<bool>),
    Ty(Ty),
    Bool(bool),
}

impl Out {
    fn fit(self) -> Option<bool> {
        match self {
            Out::Fit(f) => f,
            _ => None,
        }
    }
    fn ty(self) -> Ty {
        match self {
            Out::Ty(t) => t,
            _ => Ty::any(),
        }
    }
    fn boolean(self) -> bool {
        match self {
            Out::Bool(b) => b,
            _ => false,
        }
    }
}

/// A pair the rule must enter before it can go on.
#[derive(Debug)]
pub(super) struct Need;

pub(super) type Step<T> = Result<T, Need>;

struct Frame {
    kind: Kind,
    a: Ty,
    b: Ty,
    key: u64,
    log: Vec<Out>,
    pos: usize,
}

/// A key of a pair from the top of its two types alone: equal pairs have
/// equal keys. It narrows the pairs in progress or failed that a pair may
/// equal to those of its key, so a lookup need not walk them all. Distinct
/// pairs may share a key, and a rule that meets many pairs runs again from
/// its start after each, replaying the outcomes before it: the work is
/// bounded by the bound times the work of one rule, not linear in the pairs
/// entered.
fn key(kind: Kind, a: &Ty, b: &Ty) -> u64 {
    fn top(t: &Ty, h: &mut impl Hasher) {
        (t.any, t.null, t.boolean, t.num.map(|n| n == Num::Int)).hash(h);
        match &t.str {
            None => 0u8.hash(h),
            Some(Strs::All) => 1u8.hash(h),
            Some(Strs::Lits(l)) => (2u8, l.len()).hash(h),
        }
        t.list.is_some().hash(h);
        t.map.as_ref().map(|r| r.fields.len()).hash(h);
        t.variants.as_ref().map(|v| v.len()).hash(h);
        t.opaque.len().hash(h);
        t.refs.hash(h);
    }
    let mut h = std::collections::hash_map::DefaultHasher::new();
    (kind as u8).hash(&mut h);
    top(a, &mut h);
    top(b, &mut h);
    h.finish()
}

/// One computation (12.5 item 1).
#[derive(Default)]
pub(super) struct Cx {
    /// The pairs entered and in progress, the innermost last.
    frames: Vec<Frame>,
    /// The frames by the key of their pair.
    open: HashMap<u64, Vec<usize>>,
    /// The log of the rule that began the computation.
    root_log: Vec<Out>,
    root_pos: usize,
    entered: u32,
    failed: Vec<(Kind, Ty, Ty)>,
    /// The failed pairs by key.
    failed_at: HashMap<u64, Vec<usize>>,
    need: Option<(Kind, Ty, Ty, u64)>,
}

/// Run one computation. `root` is the rule that begins it; it may run more
/// than once. Past the bound the computation ends with `fallback`, and the
/// watches see it.
pub(super) fn run<T>(fallback: impl FnOnce() -> T, mut root: impl FnMut(&mut Cx) -> Step<T>) -> T {
    let mut cx = Cx::default();
    loop {
        if cx.frames.is_empty() {
            cx.root_pos = 0;
            if let Ok(t) = root(&mut cx) {
                return t;
            }
        } else {
            let (kind, a, b) = {
                let f = cx.frames.last_mut().expect("a frame");
                f.pos = 0;
                (f.kind, f.a.clone(), f.b.clone())
            };
            if let Ok(out) = cx.entered_rule(kind, &a, &b) {
                let f = cx.frames.pop().expect("a frame");
                if let Some(v) = cx.open.get_mut(&f.key) {
                    v.pop();
                }
                let failed = matches!(
                    (&out, kind),
                    (Out::Fit(None), Kind::Fits)
                        | (Out::Bool(false), Kind::Equal)
                        | (Out::Bool(true), Kind::Disjoint)
                );
                if failed {
                    cx.failed_at.entry(f.key).or_default().push(cx.failed.len());
                    cx.failed.push((kind, f.a, f.b));
                }
                match cx.frames.last_mut() {
                    Some(p) => p.log.push(out),
                    None => cx.root_log.push(out),
                }
                continue;
            }
        }
        let Some((kind, a, b, key)) = cx.need.take() else {
            return fallback();
        };
        if cx.entered >= BOUND {
            PASSED.with(|p| p.set(true));
            return fallback();
        }
        cx.entered += 1;
        if a.refs.iter().chain(&b.refs).any(|r| !r.complete()) {
            LOOKED.with(|l| l.set(true));
        }
        cx.open.entry(key).or_default().push(cx.frames.len());
        cx.frames.push(Frame {
            kind,
            a,
            b,
            key,
            log: Vec::new(),
            pos: 0,
        });
    }
}

macro_rules! fit {
    ($e:expr, $used:ident) => {
        match $e? {
            None => return Ok(None),
            Some(u) => $used |= u,
        }
    };
}

impl Cx {
    /// The outcome of a pair at steps 3 to 5 of 12.5 item 3: replayed, in
    /// progress, failed, or to enter.
    fn decide(&mut self, kind: Kind, a: &Ty, b: &Ty) -> Step<Out> {
        let (log, pos) = match self.frames.last_mut() {
            Some(f) => (&mut f.log, &mut f.pos),
            None => (&mut self.root_log, &mut self.root_pos),
        };
        if *pos < log.len() {
            *pos += 1;
            return Ok(log[*pos - 1].clone());
        }
        let k = key(kind, a, b);
        let same = |x: &Ty, y: &Ty| x == a && y == b;
        let open = self.open.get(&k).is_some_and(|v| {
            v.iter().any(|&i| {
                let f = &self.frames[i];
                f.kind == kind && same(&f.a, &f.b)
            })
        });
        let failed = || {
            self.failed_at.get(&k).is_some_and(|v| {
                v.iter().any(|&i| {
                    let (fk, x, y) = &self.failed[i];
                    *fk == kind && same(x, y)
                })
            })
        };
        let known = if open {
            Some(match kind {
                Kind::Fits => Out::Fit(Some(false)),
                Kind::Join => Out::Ty(Ty::any()),
                Kind::Equal => Out::Bool(true),
                Kind::Disjoint => Out::Bool(false),
            })
        } else if failed() {
            Some(match kind {
                Kind::Fits => Out::Fit(None),
                Kind::Join => Out::Ty(Ty::any()),
                Kind::Equal => Out::Bool(false),
                Kind::Disjoint => Out::Bool(true),
            })
        } else {
            None
        };
        match known {
            Some(o) => {
                let (log, pos) = match self.frames.last_mut() {
                    Some(f) => (&mut f.log, &mut f.pos),
                    None => (&mut self.root_log, &mut self.root_pos),
                };
                log.push(o.clone());
                *pos += 1;
                Ok(o)
            }
            None => {
                self.need = Some((kind, a.clone(), b.clone(), k));
                Err(Need)
            }
        }
    }

    /// The rule of a pair once entered: on the unfoldings.
    fn entered_rule(&mut self, kind: Kind, a: &Ty, b: &Ty) -> Step<Out> {
        Ok(match kind {
            Kind::Fits => Out::Fit(self.fits_core(&a.view(), &b.view())?),
            Kind::Join => Out::Ty(self.join_entered(a, b)?),
            Kind::Equal => Out::Bool(self.equal_core(&a.view(), &b.view())?),
            Kind::Disjoint => Out::Bool(self.disjoint_core(&a.view(), &b.view())?),
        })
    }

    // ---- Fits ---------------------------------------------------------------

    /// Fits(S, T) (`05-types.md` 4.2, F17 12.5 item 3). `Some(uses_any)`
    /// when S fits T.
    pub(super) fn fits(&mut self, s: &Ty, t: &Ty) -> Step<Option<bool>> {
        if s.is_never() || t.any {
            return Ok(Some(false));
        }
        if s.any {
            return Ok(Some(true));
        }
        if s.refs.is_empty() && t.refs.is_empty() {
            return self.fits_core(s, t);
        }
        let dropped;
        let s = if s.refs.iter().any(|r| t.refs.contains(r)) {
            let mut x = s.clone();
            x.refs.retain(|r| !t.refs.contains(r));
            dropped = x;
            &dropped
        } else {
            s
        };
        if s.is_never() {
            return Ok(Some(false));
        }
        Ok(self.decide(Kind::Fits, s, t)?.fit())
    }

    /// Core's rule, on two types with no complete reference at their top.
    /// The alternatives of S in the order of 12.5 item 6.
    fn fits_core(&mut self, s: &Ty, t: &Ty) -> Step<Option<bool>> {
        if s.is_never() || t.any {
            return Ok(Some(false));
        }
        if s.any {
            return Ok(Some(true));
        }
        let mut used = false;
        if s.boolean && !t.boolean {
            return Ok(None);
        }
        if let Some(n) = s.num {
            match (n, t.num) {
                (Num::Int, Some(_)) | (Num::Float, Some(Num::Float)) => {}
                _ => return Ok(None),
            }
        }
        if let Some(ss) = &s.str {
            match (ss, &t.str) {
                (_, Some(Strs::All)) => {}
                (Strs::Lits(a), Some(Strs::Lits(b))) if a.is_subset(b) => {}
                _ => return Ok(None),
            }
        }
        if let Some(e) = &s.list {
            let Some(te) = &t.list else {
                return Ok(None);
            };
            fit!(self.fits(e, te), used);
        }
        if s.map.is_some() {
            if t.map.is_none() {
                return Ok(None);
            }
            // F5 4.2: each variant of S fits some variant of T; the fit uses
            // any when some variant fits none without using any.
            let tv = t.variants();
            for sv in s.variants() {
                let mut best: Option<bool> = None;
                for x in &tv {
                    match self.fits_rec(sv, x)? {
                        Some(false) => {
                            best = Some(false);
                            break;
                        }
                        Some(true) => best = Some(true),
                        None => {}
                    }
                }
                match best {
                    None => return Ok(None),
                    Some(u) => used |= u,
                }
            }
        }
        if !s.opaque.is_subset(&t.opaque) {
            return Ok(None);
        }
        if s.null && !t.null {
            return Ok(None);
        }
        // A reference that is not complete fits only itself.
        if !s.refs.is_subset(&t.refs) {
            return Ok(None);
        }
        Ok(Some(used))
    }

    /// Two records (12.5 item 6): T's fields in key order, the mark before
    /// the type; then S's other fields against T's rest; then the rests.
    fn fits_rec(&mut self, s: &Rec, t: &Rec) -> Step<Option<bool>> {
        let mut used = false;
        for (k, ft) in &t.fields {
            match s.fields.get(k) {
                Some(fs) => {
                    if fs.optional && !ft.optional {
                        return Ok(None);
                    }
                    fit!(self.fits(&fs.ty, &ft.ty), used);
                }
                None => {
                    if !ft.optional {
                        return Ok(None);
                    }
                    fit!(self.fits(&s.rest, &ft.ty), used);
                }
            }
        }
        for (k, fs) in &s.fields {
            if !t.fields.contains_key(k) {
                fit!(self.fits(&fs.ty, &t.rest), used);
            }
        }
        fit!(self.fits(&s.rest, &t.rest), used);
        Ok(Some(used))
    }

    // ---- Join ---------------------------------------------------------------

    /// Join(A, B) (`05-types.md` 4.1, F17 12.5 item 4).
    pub(super) fn join(&mut self, a: &Ty, b: &Ty) -> Step<Ty> {
        if a.any || b.any {
            return Ok(Ty::any());
        }
        if a.is_never() {
            return Ok(b.clone());
        }
        if b.is_never() {
            return Ok(a.clone());
        }
        if a.refs.is_empty() && b.refs.is_empty() {
            return self.join_core(a, b);
        }
        if a.is_exactly_null() {
            return Ok(b.clone().with_null());
        }
        if b.is_exactly_null() {
            return Ok(a.clone().with_null());
        }
        if a.refs.iter().chain(&b.refs).any(|r| !r.complete()) {
            let mut out = self.join_core(&a.without_refs(), &b.without_refs())?;
            if !out.any {
                out.refs = a.refs.union(&b.refs).cloned().collect();
            }
            return Ok(out);
        }
        if a == b {
            return Ok(a.clone());
        }
        if a.view().any || b.view().any {
            return Ok(Ty::any());
        }
        if self.fits(a, b)? == Some(false) {
            return Ok(b.clone());
        }
        if self.fits(b, a)? == Some(false) {
            return Ok(a.clone());
        }
        Ok(self.decide(Kind::Join, a, b)?.ty())
    }

    /// Step 8 of 12.5 item 4: each reference that shares a kind other than
    /// null with the other side, and that the other side does not have, is
    /// unfolded; the two are joined by Core's rule; the other references
    /// stand beside.
    fn join_entered(&mut self, a: &Ty, b: &Ty) -> Step<Ty> {
        let (ka, kb) = (Kinds::of(a), Kinds::of(b));
        let mut kept: BTreeSet<super::Ref> = BTreeSet::new();
        let mut x = a.without_refs();
        for r in &a.refs {
            let u = r.unfolding();
            match u {
                Some(u) if !b.refs.contains(r) && Kinds::of(&u).shares(&kb, false) => {
                    add_alts(&mut x, &u)
                }
                _ => {
                    kept.insert(r.clone());
                }
            }
        }
        let mut y = b.without_refs();
        for r in &b.refs {
            let u = r.unfolding();
            match u {
                Some(u) if !a.refs.contains(r) && Kinds::of(&u).shares(&ka, false) => {
                    add_alts(&mut y, &u)
                }
                _ => {
                    kept.insert(r.clone());
                }
            }
        }
        let refs_x = std::mem::take(&mut x.refs);
        let refs_y = std::mem::take(&mut y.refs);
        let mut out = self.join_core(&x, &y)?;
        if !out.any {
            out.refs.extend(kept);
            out.refs.extend(refs_x);
            out.refs.extend(refs_y);
        }
        Ok(out)
    }

    /// Core's Join of two types with no reference at their top, whose parts
    /// are joined by this rule: the list's elements, then the map
    /// alternatives.
    fn join_core(&mut self, a: &Ty, b: &Ty) -> Step<Ty> {
        if a.any || b.any {
            return Ok(Ty::any());
        }
        let list = match (&a.list, &b.list) {
            // No shortcut for one shared element type: the Join builds
            // its records, and they carry no description (F16 11.4).
            (Some(x), Some(y)) => Some(Arc::new(self.join(x, y)?)),
            (x, y) => x.clone().or(y.clone()),
        };
        // F5 4.1: a union survives only beside the same union; Join never
        // makes one. Two unions are the same by structure when they are
        // Equal, references followed, and the Join is A's union (F17 12.5
        // item 4, R310).
        let same_union = match (&a.variants, &b.variants) {
            (Some(_), Some(_)) if a.variants == b.variants => true,
            (Some(x), Some(y)) if x.len() == y.len() => {
                let mut same = true;
                for (p, q) in x.iter().zip(y.iter()) {
                    if !self.equal_rec(p, q)? {
                        same = false;
                        break;
                    }
                }
                same
            }
            _ => false,
        };
        let (map, variants) = match (&a.map, &b.map) {
            (Some(_), Some(_)) if same_union => (a.map.clone(), a.variants.clone()),
            (Some(x), Some(y)) => (Some(Arc::new(self.join_rec(x, y)?)), None),
            (Some(_), None) => (a.map.clone(), a.variants.clone()),
            (None, Some(_)) => (b.map.clone(), b.variants.clone()),
            (None, None) => (None, None),
        };
        Ok(Ty {
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
            list,
            map,
            variants,
            opaque: a.opaque.union(&b.opaque).cloned().collect(),
            refs: a.refs.union(&b.refs).cloned().collect(),
        })
    }

    /// The Join of two records: the keys of both in code point order, then
    /// the rests. F1 4.4: a key in both has the Join of the bases and of the
    /// fills; a key in one has its base joined with the other's rest, and
    /// keeps its fill. A field that carries a fill is required.
    pub(super) fn join_rec(&mut self, a: &Rec, b: &Rec) -> Step<Rec> {
        let mut fields = BTreeMap::new();
        let mut fills = BTreeMap::new();
        let base =
            |r: &Rec, k: &str, f: &Field| r.fills.get(k).map_or(f.ty.clone(), |x| x.0.clone());
        let keys: BTreeSet<&String> = a.fields.keys().chain(b.fields.keys()).collect();
        for k in keys {
            let f = match (a.fields.get(k), b.fields.get(k)) {
                (Some(fa), Some(fb)) => {
                    let fill = match (a.fills.get(k), b.fills.get(k)) {
                        (Some(x), Some(y)) => Some(self.join(&x.1, &y.1)?),
                        (Some(x), None) | (None, Some(x)) => Some(x.1.clone()),
                        (None, None) => None,
                    };
                    match fill {
                        None => Field {
                            ty: self.join(&fa.ty, &fb.ty)?,
                            optional: fa.optional || fb.optional,
                            doc: Default::default(),
                        },
                        Some(fl) => {
                            let bs = self.join(&base(a, k, fa), &base(b, k, fb))?;
                            let ty = self.join(&bs, &fl)?;
                            fills.insert(k.clone(), (bs, fl));
                            Field {
                                ty,
                                optional: false,
                                doc: Default::default(),
                            }
                        }
                    }
                }
                (Some(f1), None) | (None, Some(f1)) => {
                    let (r1, r2) = if a.fields.contains_key(k) {
                        (a, b)
                    } else {
                        (b, a)
                    };
                    match r1.fills.get(k) {
                        None => Field {
                            ty: self.join(&f1.ty, &r2.rest)?,
                            optional: true,
                            doc: Default::default(),
                        },
                        Some((bs, fl)) => {
                            let bs = self.join(bs, &r2.rest)?;
                            let ty = self.join(&bs, fl)?;
                            fills.insert(k.clone(), (bs, fl.clone()));
                            Field {
                                ty,
                                optional: false,
                                doc: Default::default(),
                            }
                        }
                    }
                }
                (None, None) => continue,
            };
            fields.insert(k.clone(), f);
        }
        Ok(Rec {
            fields,
            rest: self.join(&a.rest, &b.rest)?,
            // F1 3.6 item 8: closable when one is closable and the other is
            // closable or exact.
            closable: (a.closable || b.closable)
                && (a.closable || a.rest.is_never())
                && (b.closable || b.rest.is_never()),
            size: SizeCache::default(),
            fills,
        })
    }

    // ---- Equal --------------------------------------------------------------

    /// Equal(A, B): the same by structure (F5 2.5, F17 12.5 item 5).
    pub(super) fn equal(&mut self, a: &Ty, b: &Ty) -> Step<bool> {
        if a == b {
            return Ok(true);
        }
        if a.refs.is_empty() && b.refs.is_empty() {
            return self.equal_core(a, b);
        }
        Ok(self.decide(Kind::Equal, a, b)?.boolean())
    }

    /// The alternatives in the order of printing; two records' keys in code
    /// point order, each field's mark before its type, then the rests.
    fn equal_core(&mut self, a: &Ty, b: &Ty) -> Step<bool> {
        if a.any != b.any
            || a.boolean != b.boolean
            || a.num != b.num
            || a.str != b.str
            || a.list.is_some() != b.list.is_some()
        {
            return Ok(false);
        }
        if let (Some(x), Some(y)) = (&a.list, &b.list) {
            if !self.equal(x, y)? {
                return Ok(false);
            }
        }
        // The map after the list, in the order of printing (12.5 item 6).
        if a.map.is_some() != b.map.is_some() {
            return Ok(false);
        }
        if a.map.is_some() {
            let (av, bv) = (a.variants(), b.variants());
            if av.len() != bv.len() {
                return Ok(false);
            }
            for (x, y) in av.iter().zip(&bv) {
                if !self.equal_rec(x, y)? {
                    return Ok(false);
                }
            }
        }
        Ok(a.opaque == b.opaque && a.refs == b.refs && a.null == b.null)
    }

    /// Two records' keys in code point order, each once, each field's mark
    /// before its type, then the rests (12.5 item 6): a key that one lacks
    /// ends the test where the order meets it, after the keys before it.
    fn equal_rec(&mut self, x: &Rec, y: &Rec) -> Step<bool> {
        let (mut xs, mut ys) = (x.fields.iter().peekable(), y.fields.iter().peekable());
        loop {
            match (xs.peek(), ys.peek()) {
                (None, None) => break,
                (Some((kx, fx)), Some((ky, fy))) if kx == ky => {
                    if fx.optional != fy.optional || !self.equal(&fx.ty, &fy.ty)? {
                        return Ok(false);
                    }
                    xs.next();
                    ys.next();
                }
                _ => return Ok(false),
            }
        }
        self.equal(&x.rest, &y.rest)
    }

    // ---- disjoint -----------------------------------------------------------

    /// disjoint(A, B) (F4 6.2, F17 12.5 item 5): true when no value conforms
    /// to both. Sound and not complete.
    pub(super) fn disjoint(&mut self, a: &Ty, b: &Ty) -> Step<bool> {
        if a.is_never() || b.is_never() {
            return Ok(true);
        }
        if a.any || b.any {
            return Ok(false);
        }
        if a.refs.is_empty() && b.refs.is_empty() {
            return self.disjoint_core(a, b);
        }
        if a == b || a.refs.iter().any(|r| b.refs.contains(r)) {
            return Ok(false);
        }
        Ok(self.decide(Kind::Disjoint, a, b)?.boolean())
    }

    /// The alternatives in the order of printing.
    fn disjoint_core(&mut self, a: &Ty, b: &Ty) -> Step<bool> {
        if a.is_never() || b.is_never() {
            return Ok(true);
        }
        if a.any || b.any {
            return Ok(false);
        }
        if (a.boolean && b.boolean) || (a.num.is_some() && b.num.is_some()) {
            return Ok(false);
        }
        match (&a.str, &b.str) {
            (Some(Strs::Lits(x)), Some(Strs::Lits(y))) if x.is_disjoint(y) => {}
            (Some(_), Some(_)) => return Ok(false),
            _ => {}
        }
        if a.list.is_some() && b.list.is_some() {
            return Ok(false);
        }
        if a.map.is_some() && b.map.is_some() {
            // F5 4.5: they share a value unless every variant of A is
            // disjoint from every variant of B.
            let bv = b.variants();
            for x in a.variants() {
                for y in &bv {
                    if !self.disjoint_rec(x, y)? {
                        return Ok(false);
                    }
                }
            }
        }
        if !a.opaque.is_disjoint(&b.opaque) || !a.refs.is_disjoint(&b.refs) {
            return Ok(false);
        }
        if a.null && b.null {
            return Ok(false);
        }
        Ok(true)
    }

    /// The record row of disjoint (F4 6.2): disjoint when a key that either
    /// requires, in code point order, has disjoint types in the two.
    fn disjoint_rec(&mut self, x: &Rec, y: &Rec) -> Step<bool> {
        let field = |r: &Rec, k: &str| r.fields.get(k).map_or(r.rest.clone(), |f| f.ty.clone());
        let keys: BTreeSet<&String> = x
            .fields
            .iter()
            .chain(y.fields.iter())
            .filter(|(_, f)| !f.optional)
            .map(|(k, _)| k)
            .collect();
        for k in keys {
            if self.disjoint(&field(x, k), &field(y, k))? {
                return Ok(true);
            }
        }
        Ok(false)
    }
}
