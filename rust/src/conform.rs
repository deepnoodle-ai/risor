//! Package F4: the conformance walk (`spec/packages/f04-conversion.md` 4)
//! and the conformance function (7).
//!
//! The walk tests a value against a resolved type in a fixed order, with
//! exact charges, and gives the first failure with its path and message. It
//! recurses only where the type and the value both have a level. With a
//! recursive type (F17 12.7) the type no longer bounds that: a run's walk is
//! bounded by the run's depth limit, and the conformance function's by the
//! default depth limit, 200. The conformance function charges the steps a
//! run's walk charges, against a step limit of its own (7.3), so host data
//! that holds one list at many places cannot make it run long.

use crate::check::ty::{print, Num, Strs, Ty};
use crate::diag::{Diagnostic, PathStep, Phase};
use crate::messages::json_string;
use crate::value::{is_whole, size, weight, Kind, Value};

/// What the walk charges and how deep it goes (`02-evaluation.md` 8.4). A
/// run meters it; the conformance function meters it with [`Free`].
pub trait WalkMeter {
    /// Charge steps; a limit's code when passed.
    fn charge(&mut self, n: u64) -> Result<(), &'static str>;
    /// The depth test of entering a list or a map (5.4), then the count.
    fn enter(&mut self) -> Result<(), &'static str>;
    fn leave(&mut self);
}

/// The default depth limit (`03-host.md` 3.6.1), past which the
/// conformance function fails (F17 12.7 item 3).
pub const FREE_DEPTH: u32 = 200;

/// The default step limit of the conformance function (7.3,
/// `03-host.md` 3.6.1).
pub const FREE_STEPS: u64 = 1_000_000;

/// The conformance function's meter (7.1 item 1, 7.3): no run, but the
/// steps a run's walk charges, counted against `limit` (`None`: no limit).
/// It counts the levels it enters, and fails past 200 (F17 12.7 item 3).
pub struct Free {
    depth: u32,
    count: u64,
    limit: Option<u64>,
}

impl Free {
    pub fn new(limit: Option<u64>) -> Free {
        Free {
            depth: 0,
            count: 0,
            limit,
        }
    }
}

impl Default for Free {
    fn default() -> Free {
        Free::new(Some(FREE_STEPS))
    }
}

impl WalkMeter for Free {
    fn charge(&mut self, n: u64) -> Result<(), &'static str> {
        if let Some(l) = self.limit {
            if n > l - self.count {
                return Err(STEPS);
            }
        }
        self.count = self.count.saturating_add(n);
        Ok(())
    }
    fn enter(&mut self) -> Result<(), &'static str> {
        self.depth += 1;
        if self.depth > FREE_DEPTH {
            return Err("depth-exceeded");
        }
        Ok(())
    }
    fn leave(&mut self) {
        self.depth -= 1;
    }
}

/// The failure of the conformance function past its step limit (7.3):
/// phase `run`, code `steps-exceeded`, class `limit`, no path and no span.
pub fn too_many_steps(limit: u64) -> Diagnostic {
    let msg = crate::messages::build("steps-exceeded", "default", &[("limit", limit.to_string())]);
    let mut d = Diagnostic::bare(Phase::Run, "steps-exceeded", msg);
    d.class = Some("limit");
    d
}

/// The diagnostic of the conformance function at the limit whose code is
/// `code`, under the step limit `limit`.
pub fn limit_diagnostic(code: &str, limit: Option<u64>) -> Diagnostic {
    if code == "depth-exceeded" {
        too_deep()
    } else {
        too_many_steps(limit.unwrap_or(0))
    }
}

/// The failure of the conformance function past the default depth limit
/// (F17 12.7 item 3): phase `run`, code `depth-exceeded`, class `limit`, no
/// path and no span.
pub fn too_deep() -> Diagnostic {
    let msg = crate::messages::build(
        "depth-exceeded",
        "default",
        &[("limit", FREE_DEPTH.to_string())],
    );
    let mut d = Diagnostic::bare(Phase::Run, "depth-exceeded", msg);
    d.class = Some("limit");
    d
}

/// Counts steps and nothing else, for tests.
#[cfg(test)]
#[derive(Default)]
pub struct Count(pub u64);

#[cfg(test)]
impl WalkMeter for Count {
    fn charge(&mut self, n: u64) -> Result<(), &'static str> {
        self.0 += n;
        Ok(())
    }
    fn enter(&mut self) -> Result<(), &'static str> {
        Ok(())
    }
    fn leave(&mut self) {}
}

/// A failure of the walk (4.4): its path, and a template of
/// `not-conforming` with its parameters.
#[derive(Clone, Debug, PartialEq)]
pub struct Failure {
    pub path: Vec<PathStep>,
    pub template: &'static str,
    pub params: Vec<(&'static str, String)>,
}

impl Failure {
    pub fn message(&self) -> String {
        crate::messages::build("not-conforming", self.template, &self.params)
    }

    /// The diagnostic the conformance function gives (7.1): phase `run`,
    /// class `value`, the path and message, no span. The walk of every
    /// failure that went past the default depth limit ends with
    /// [`too_deep`].
    pub fn diagnostic(&self) -> Diagnostic {
        if self.template == DEEP {
            return too_deep();
        }
        if self.template == STEPS {
            let limit = self.params.first().map_or(0, |p| p.1.parse().unwrap_or(0));
            return too_many_steps(limit);
        }
        let mut d = Diagnostic::bare(Phase::Run, "not-conforming", self.message());
        d.class = Some("value");
        d.path = Some(Box::new(self.path.clone()));
        d
    }
}

/// The template of the last entry of [`all`] when the walk went past the
/// default depth limit.
const DEEP: &str = "depth-exceeded";

/// The template of the last entry of [`all`] when the walk went past its
/// step limit (`f08-errors.md` 6.4 step 3); its parameter is the limit.
const STEPS: &str = "steps-exceeded";

/// Test(v, T, []) (4.2). `Ok(None)` is pass; `Err` is a limit's code.
pub fn test<M: WalkMeter>(
    v: &Value<'_>,
    t: &Ty,
    m: &mut M,
) -> Result<Option<Failure>, &'static str> {
    let mut w = Walk {
        m,
        path: Vec::new(),
    };
    w.test(v, t, false)
}

struct Walk<'m, M: WalkMeter> {
    m: &'m mut M,
    path: Vec<PathStep>,
}

fn wrong(path: &[PathStep], t: &Ty, v: &Value<'_>) -> Failure {
    let found = match v {
        Value::Null => "null".to_string(),
        Value::Bool(b) => b.to_string(),
        Value::Num(n) => crate::number::to_text(*n),
        Value::Str(s) => json_string(s.as_str()),
        // 4.4 item 3, and the kind of a list, a map or an opaque value.
        _ => v.kind().name().to_string(),
    };
    Failure {
        path: path.to_vec(),
        template: "wrong",
        params: vec![("expected", print(t)), ("found", found)],
    }
}

impl<M: WalkMeter> Walk<'_, M> {
    /// `looked_up`: the value of a declared field, whose step 3 is the
    /// lookup already charged.
    fn test(
        &mut self,
        v: &Value<'_>,
        t: &Ty,
        looked_up: bool,
    ) -> Result<Option<Failure>, &'static str> {
        // 1.
        if t.any {
            return Ok(None);
        }
        // F17 12.7 item 1: a type with references is tested by its
        // unfolding; a failure prints it by its name.
        let u = t.view();
        if u.any {
            return Ok(None);
        }
        // 2.
        let enters = matches!(v.kind(), Kind::List) && u.list.is_some()
            || matches!(v.kind(), Kind::Map) && u.map.is_some();
        if enters {
            self.m.enter()?;
        }
        let r = self.body(v, t, &u, looked_up);
        // 9.
        if enters {
            self.m.leave();
        }
        r
    }

    /// `t` is the type as written, `u` its unfolding.
    fn body(
        &mut self,
        v: &Value<'_>,
        t: &Ty,
        u: &Ty,
        looked_up: bool,
    ) -> Result<Option<Failure>, &'static str> {
        // 3.
        if !looked_up {
            self.m.charge(1)?;
        }
        // 4, 5, 6.
        match v {
            Value::BadNum => Ok(Some(wrong(&self.path, t, v))),
            Value::Null if u.null => Ok(None),
            Value::Bool(_) if u.boolean => Ok(None),
            Value::Num(n) => match u.num {
                Some(Num::Float) => Ok(None),
                Some(Num::Int) if is_whole(*n) => Ok(None),
                _ => Ok(Some(wrong(&self.path, t, v))),
            },
            Value::Str(s) => match &u.str {
                Some(Strs::All) => Ok(None),
                Some(Strs::Lits(l)) => {
                    self.m.charge(size(s.as_str()))?;
                    if l.contains(s.as_str()) {
                        Ok(None)
                    } else {
                        Ok(Some(wrong(&self.path, t, v)))
                    }
                }
                None => Ok(Some(wrong(&self.path, t, v))),
            },
            Value::Opaque(o) if u.opaque.contains(o.type_name.as_ref()) => Ok(None),
            Value::List(_) | Value::HList(_) if u.list.is_some() => {
                // 7.
                let Some(e) = u.list.as_deref() else {
                    return Ok(None);
                };
                let n = v.len().unwrap_or(0);
                for i in 0..n {
                    self.path.push(PathStep::Index(i));
                    let r = self.test(&v.elem(i), e, false)?;
                    self.path.pop();
                    if r.is_some() {
                        return Ok(r);
                    }
                }
                Ok(None)
            }
            Value::Map(_) | Value::HMap(_) if u.variants.is_some() => self.union(v, u),
            Value::Map(_) | Value::HMap(_) if u.map.is_some() => match u.map.as_deref() {
                Some(r) => self.record(v, r, None),
                None => Ok(None),
            },
            _ => Ok(Some(wrong(&self.path, t, v))),
        }
    }

    /// F5 7.2: a map against a tagged union. The tag is read first, then the
    /// variant its value selects is tested by 4.3, the tag passed over.
    fn union(&mut self, v: &Value<'_>, t: &Ty) -> Result<Option<Failure>, &'static str> {
        let Some(flat) = t.map.as_deref() else {
            return Ok(None);
        };
        let tag = t.tag().unwrap_or_default();
        self.m.charge(1 + size(&tag))?;
        let Some(x) = v.field(&tag) else {
            let mut path = self.path.clone();
            path.push(PathStep::Key(tag.clone()));
            return Ok(Some(Failure {
                path,
                template: "missing",
                params: vec![("key", json_string(&tag))],
            }));
        };
        let tag_ty = flat
            .fields
            .get(&tag)
            .map(|f| f.ty.clone())
            .unwrap_or_default();
        self.path.push(PathStep::Key(tag.clone()));
        let r = self.test(&x, &tag_ty, true)?;
        self.path.pop();
        if r.is_some() {
            return Ok(r);
        }
        let s = x.as_str().unwrap_or_default();
        let vs: &[crate::check::ty::Rec] = t.variants.as_deref().map_or(&[], |v| v.as_slice());
        let Some(var) = vs.iter().find(|r| {
            matches!(r.fields.get(&tag).and_then(|f| f.ty.str.as_ref()), Some(Strs::Lits(l)) if l.contains(s))
        }) else {
            return Ok(Some(wrong(&self.path, &tag_ty, &x)));
        };
        self.record(v, var, Some(&tag))
    }

    /// 4.3. `skip`: a union's tag, already looked up and tested (F5 7.2
    /// step 3).
    fn record(
        &mut self,
        v: &Value<'_>,
        r: &crate::check::ty::Rec,
        skip: Option<&str>,
    ) -> Result<Option<Failure>, &'static str> {
        let mut present = usize::from(skip.is_some());
        // 1. Declared fields, in code point order of their keys.
        for (k, f) in &r.fields {
            if skip == Some(k.as_str()) {
                continue;
            }
            self.m.charge(1 + size(k))?;
            match v.field(k) {
                None => {
                    if !f.optional {
                        let mut path = self.path.clone();
                        path.push(PathStep::Key(k.clone()));
                        return Ok(Some(Failure {
                            path,
                            template: "missing",
                            params: vec![("key", json_string(k))],
                        }));
                    }
                }
                Some(x) => {
                    present += 1;
                    self.path.push(PathStep::Key(k.clone()));
                    let res = self.test(&x, &f.ty, true)?;
                    self.path.pop();
                    if res.is_some() {
                        return Ok(res);
                    }
                }
            }
        }
        // 2, 3.
        let m = v.len().unwrap_or(0).saturating_sub(present);
        if r.rest.any || m == 0 {
            return Ok(None);
        }
        let others: Vec<String> = v
            .keys()
            .into_iter()
            .map(|k| k.as_str().to_string())
            .filter(|k| !r.fields.contains_key(k))
            .collect();
        let together: u64 = others.iter().map(|k| weight(k)).sum::<u64>() / 64;
        // 4. An exact record.
        if r.rest.is_never() {
            self.m.charge(m as u64 + together)?;
            let k = others.first().cloned().unwrap_or_default();
            let mut path = self.path.clone();
            path.push(PathStep::Key(k.clone()));
            return Ok(Some(Failure {
                path,
                template: "extra",
                params: vec![("key", json_string(&k))],
            }));
        }
        // 5. Another rest: every other key is tested.
        self.m.charge(together)?;
        let mut first: Option<Failure> = None;
        for k in &others {
            let x = v.field(k).unwrap_or(Value::Null);
            self.path.push(PathStep::Key(k.clone()));
            let res = self.test(&x, &r.rest, false)?;
            self.path.pop();
            if first.is_none() {
                first = res;
            }
        }
        Ok(first)
    }
}

/// All(v, T, []) of `f08-errors.md` 6.2: every failure, up to `max`, under
/// the step limit `limit` (`None`: no limit) by 6.4. First the walk of 4.2:
/// a limit there is the whole list. Then All, from a new count, charging at
/// the places that walk charges. Past the default depth limit it ends with
/// a failure whose diagnostic is [`too_deep`] (F17 12.7 item 3), past the
/// step limit with one whose diagnostic is [`too_many_steps`].
pub fn all(v: &Value<'_>, t: &Ty, max: usize, limit: Option<u64>) -> Vec<Failure> {
    let limit_failure = |template: &'static str| Failure {
        path: Vec::new(),
        template,
        params: if template == STEPS {
            vec![("limit", limit.unwrap_or(0).to_string())]
        } else {
            Vec::new()
        },
    };
    // 6.4 step 1.
    if let Err(code) = test(v, t, &mut Free::new(limit)) {
        return vec![limit_failure(if code == DEEP { DEEP } else { STEPS })];
    }
    let mut a = All {
        path: Vec::new(),
        out: Vec::new(),
        max,
        depth: 0,
        deep: false,
        count: 0,
        limit,
        stopped: false,
    };
    a.walk(v, t, false);
    if a.deep {
        a.out.push(limit_failure(DEEP));
    } else if a.stopped {
        a.out.push(limit_failure(STEPS));
    }
    a.out
}

/// The walk of every failure. It recurses only where the type and the value
/// both have a level, and no deeper than the default depth limit.
struct All {
    path: Vec<PathStep>,
    out: Vec<Failure>,
    max: usize,
    depth: u32,
    /// The walk went past the default depth limit: it ends.
    deep: bool,
    /// The steps charged, and the limit (6.4 step 2).
    count: u64,
    limit: Option<u64>,
    /// A charge would have passed the limit: the walk ends (6.4 step 3).
    stopped: bool,
}

impl All {
    fn full(&self) -> bool {
        self.deep || self.stopped || self.out.len() >= self.max
    }

    /// Charge n steps; past the limit the charge is not made and the walk
    /// ends. False when it ended.
    fn charge(&mut self, n: u64) -> bool {
        if self.stopped {
            return false;
        }
        if let Some(l) = self.limit {
            if n > l - self.count {
                self.stopped = true;
                return false;
            }
        }
        self.count = self.count.saturating_add(n);
        true
    }

    fn add(&mut self, f: Failure) {
        if !self.full() {
            self.out.push(f);
        }
    }

    fn at(&self, k: &str) -> Vec<PathStep> {
        let mut p = self.path.clone();
        p.push(PathStep::Key(k.to_string()));
        p
    }

    /// `looked_up`: the value of a declared field, whose step is the lookup
    /// already charged (6.4 step 2).
    fn walk(&mut self, v: &Value<'_>, t: &Ty, looked_up: bool) {
        // 1.
        if self.full() || t.any {
            return;
        }
        let u = t.view();
        if u.any {
            return;
        }
        let enters = matches!(v.kind(), Kind::List) && u.list.is_some()
            || matches!(v.kind(), Kind::Map) && u.map.is_some();
        if enters {
            self.depth += 1;
            if self.depth > FREE_DEPTH {
                self.deep = true;
                self.depth -= 1;
                return;
            }
        }
        // The step of F4 4.2 step 3, after the depth test.
        if !looked_up && !self.charge(1) {
            if enters {
                self.depth -= 1;
            }
            return;
        }
        self.walk_body(v, t, &u);
        if enters {
            self.depth -= 1;
        }
    }

    fn walk_body(&mut self, v: &Value<'_>, t: &Ty, u: &Ty) {
        match v {
            // 2, 3, 4.
            Value::BadNum => self.add(wrong(&self.path, t, v)),
            Value::Null if u.null => {}
            Value::Bool(_) if u.boolean => {}
            Value::Num(n) => match u.num {
                Some(Num::Float) => {}
                Some(Num::Int) if is_whole(*n) => {}
                _ => self.add(wrong(&self.path, t, v)),
            },
            Value::Str(s) => match &u.str {
                Some(Strs::All) => {}
                Some(Strs::Lits(l)) => {
                    if self.charge(size(s.as_str())) && !l.contains(s.as_str()) {
                        self.add(wrong(&self.path, t, v));
                    }
                }
                None => self.add(wrong(&self.path, t, v)),
            },
            Value::Opaque(o) if u.opaque.contains(o.type_name.as_ref()) => {}
            // 5.
            Value::List(_) | Value::HList(_) if u.list.is_some() => {
                let Some(e) = u.list.as_deref() else {
                    return;
                };
                for i in 0..v.len().unwrap_or(0) {
                    self.path.push(PathStep::Index(i));
                    self.walk(&v.elem(i), e, false);
                    self.path.pop();
                    if self.full() {
                        return;
                    }
                }
            }
            // 7.
            Value::Map(_) | Value::HMap(_) if u.variants.is_some() => self.union(v, u),
            // 6.
            Value::Map(_) | Value::HMap(_) if u.map.is_some() => {
                if let Some(r) = u.map.as_deref() {
                    self.record(v, r, None);
                }
            }
            _ => self.add(wrong(&self.path, t, v)),
        }
    }

    /// Step 7: the tag first; a missing or unknown tag is one failure and
    /// ends the walk of that value.
    fn union(&mut self, v: &Value<'_>, t: &Ty) {
        let Some(flat) = t.map.as_deref() else {
            return;
        };
        let tag = t.tag().unwrap_or_default();
        if !self.charge(1 + size(&tag)) {
            return;
        }
        let Some(x) = v.field(&tag) else {
            self.add(Failure {
                path: self.at(&tag),
                template: "missing",
                params: vec![("key", json_string(&tag))],
            });
            return;
        };
        let tag_ty = flat
            .fields
            .get(&tag)
            .map(|f| f.ty.clone())
            .unwrap_or_default();
        let s = x.as_str().unwrap_or_default();
        if matches!(x, Value::Str(_)) && !self.charge(size(s)) {
            return;
        }
        let vs: &[crate::check::ty::Rec] = t.variants.as_deref().map_or(&[], |v| v.as_slice());
        let var = match &x {
            Value::Str(_) => vs.iter().find(|r| {
                matches!(r.fields.get(&tag).and_then(|f| f.ty.str.as_ref()), Some(Strs::Lits(l)) if l.contains(s))
            }),
            _ => None,
        };
        match var {
            Some(var) => self.record(v, var, Some(&tag)),
            None => self.add(wrong(&self.at(&tag), &tag_ty, &x)),
        }
    }

    /// Step 6. `skip`: a union's tag, counted as a declared key v has.
    fn record(&mut self, v: &Value<'_>, r: &crate::check::ty::Rec, skip: Option<&str>) {
        for (k, f) in &r.fields {
            if skip == Some(k.as_str()) {
                continue;
            }
            if !self.charge(1 + size(k)) {
                return;
            }
            match v.field(k) {
                None if !f.optional => self.add(Failure {
                    path: self.at(k),
                    template: "missing",
                    params: vec![("key", json_string(k))],
                }),
                None => {}
                Some(x) => {
                    self.path.push(PathStep::Key(k.clone()));
                    self.walk(&x, &f.ty, true);
                    self.path.pop();
                }
            }
            if self.full() {
                return;
            }
        }
        if r.rest.any {
            return;
        }
        let mut others: Vec<String> = v
            .keys()
            .into_iter()
            .map(|k| k.as_str().to_string())
            .filter(|k| !r.fields.contains_key(k))
            .collect();
        others.sort();
        if self.full() {
            return;
        }
        // The other keys, charged as F4 4.3 steps 4 and 5 charge them.
        let together: u64 = others.iter().map(|k| weight(k)).sum::<u64>() / 64;
        let m = if r.rest.is_never() {
            others.len() as u64
        } else {
            0
        };
        if !self.charge(m + together) {
            return;
        }
        for k in &others {
            if r.rest.is_never() {
                self.add(Failure {
                    path: self.at(k),
                    template: "extra",
                    params: vec![("key", json_string(k))],
                });
            } else {
                let x = v.field(k).unwrap_or(Value::Null);
                self.path.push(PathStep::Key(k.clone()));
                self.walk(&x, &r.rest, false);
                self.path.pop();
            }
            if self.full() {
                return;
            }
        }
    }
}
