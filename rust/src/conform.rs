//! Package F4: the conformance walk (`spec/packages/f04-conversion.md` 4)
//! and the conformance function (7).
//!
//! The walk tests a value against a resolved type in a fixed order, with
//! exact charges, and gives the first failure with its path and message. It
//! recurses only where the type has a level, so its depth is at most the
//! type's, 64, whatever the data.

use crate::check::ty::{print, Num, Strs, Ty};
use crate::diag::{Diagnostic, PathStep, Phase};
use crate::messages::json_string;
use crate::value::{is_whole, size, weight, Kind, Value};

/// What the walk charges and how deep it goes (`02-evaluation.md` 8.4). A
/// run meters it; the conformance function does not.
pub trait WalkMeter {
    /// Charge steps; a limit's code when passed.
    fn charge(&mut self, n: u64) -> Result<(), &'static str>;
    /// The depth test of entering a list or a map (5.4), then the count.
    fn enter(&mut self) -> Result<(), &'static str>;
    fn leave(&mut self);
}

/// No limit and no count: the conformance function (7.1 item 1).
pub struct Free;

impl WalkMeter for Free {
    fn charge(&mut self, _: u64) -> Result<(), &'static str> {
        Ok(())
    }
    fn enter(&mut self) -> Result<(), &'static str> {
        Ok(())
    }
    fn leave(&mut self) {}
}

/// Counts steps and nothing else, for tests.
#[derive(Default)]
pub struct Count(pub u64);

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
    /// class `value`, the path and message, no span.
    pub fn diagnostic(&self) -> Diagnostic {
        let mut d = Diagnostic::bare(Phase::Run, "not-conforming", self.message());
        d.class = Some("value");
        d.path = Some(Box::new(self.path.clone()));
        d
    }
}

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
        // 2.
        let enters = matches!(v.kind(), Kind::List) && t.list.is_some()
            || matches!(v.kind(), Kind::Map) && t.map.is_some();
        if enters {
            self.m.enter()?;
        }
        let r = self.body(v, t, looked_up);
        // 9.
        if enters {
            self.m.leave();
        }
        r
    }

    fn body(
        &mut self,
        v: &Value<'_>,
        t: &Ty,
        looked_up: bool,
    ) -> Result<Option<Failure>, &'static str> {
        // 3.
        if !looked_up {
            self.m.charge(1)?;
        }
        // 4, 5, 6.
        match v {
            Value::BadNum => Ok(Some(wrong(&self.path, t, v))),
            Value::Null if t.null => Ok(None),
            Value::Bool(_) if t.boolean => Ok(None),
            Value::Num(n) => match t.num {
                Some(Num::Float) => Ok(None),
                Some(Num::Int) if is_whole(*n) => Ok(None),
                _ => Ok(Some(wrong(&self.path, t, v))),
            },
            Value::Str(s) => match &t.str {
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
            Value::Opaque(o) if t.opaque.contains(o.type_name.as_ref()) => Ok(None),
            Value::List(_) | Value::HList(_) if t.list.is_some() => {
                // 7.
                let e = t.list.as_deref().expect("a list alternative");
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
            Value::Map(_) | Value::HMap(_) if t.map.is_some() => self.record(v, t),
            _ => Ok(Some(wrong(&self.path, t, v))),
        }
    }

    /// 4.3.
    fn record(&mut self, v: &Value<'_>, t: &Ty) -> Result<Option<Failure>, &'static str> {
        let r = t.map.as_deref().expect("a map alternative");
        let mut present = 0usize;
        // 1. Declared fields, in code point order of their keys.
        for (k, f) in &r.fields {
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

/// A type resolved by an environment, for the conformance function (7).
#[derive(Clone, Debug)]
pub struct ProgramType {
    pub ty: Ty,
}

impl ProgramType {
    /// The conformance function (7.1): `Ok(())` when the value conforms,
    /// else the diagnostic of the first failure. It is not a run: no limit,
    /// no usage.
    pub fn conform(&self, value: &crate::host::Data) -> Result<(), Diagnostic> {
        let v = Value::from_data(value);
        self.conform_value(&v)
    }

    /// The same on a run's value, or a host value read in place.
    pub fn conform_value(&self, v: &Value<'_>) -> Result<(), Diagnostic> {
        match test(v, &self.ty, &mut Free) {
            Ok(None) => Ok(()),
            Ok(Some(f)) => Err(f.diagnostic()),
            Err(_) => Ok(()),
        }
    }

    /// The type, printed by structure (`05-types.md` 3.9).
    pub fn print(&self) -> String {
        print(&self.ty)
    }
}
