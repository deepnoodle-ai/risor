//! Package F10: the decode of a result into a host type (`f10-config.md` 6.1
//! and 6.5).
//!
//! A decode first walks every failure of the value against D(H), the type
//! derived from the host type (F8 6.2). A value that conforms is then copied
//! into the host type, which fails only where the host type is narrower than
//! D(H): an integer out of range, an `f32` out of range, a list of the wrong
//! length for an array, an opaque value. The decode is not a run: it charges
//! nothing and copies.

use crate::diag::{Diagnostic, PathStep, Phase};
use crate::host::Data;
use crate::hosttype::{HostType, Opaque, OpaqueType};
use crate::value::{Kind, Value};
use std::collections::{BTreeMap, HashMap, VecDeque};
use std::rc::Rc;
use std::sync::Arc;

/// The nesting a decode reads at most (`f10-config.md` 4.1 item 4).
const MAX_DEPTH: usize = 200;

/// Where a decode is, and what it found wrong.
#[derive(Default)]
pub struct DecodeCx {
    path: Vec<PathStep>,
    failures: Vec<Diagnostic>,
}

impl DecodeCx {
    /// A failure at the path, with a message of the binding (6.1 item 9).
    pub fn fail(&mut self, message: impl Into<String>) {
        let mut d = Diagnostic::bare(Phase::Run, "not-conforming", message);
        d.class = Some("value");
        d.path = Some(Box::new(self.path.clone()));
        self.failures.push(d);
    }

    /// Decode the value of a key of a map.
    pub fn key<T: Decode>(&mut self, key: &str, v: &Value<'_>) -> Option<T> {
        self.path.push(PathStep::Key(key.to_string()));
        let r = T::decode(v, self);
        self.path.pop();
        r
    }

    /// The failure `missing` of a required key (6.1 item 5).
    pub fn missing(&mut self, key: &str) {
        self.path.push(PathStep::Key(key.to_string()));
        let msg = crate::messages::build(
            "not-conforming",
            "missing",
            &[("key", crate::messages::json_string(key))],
        );
        self.fail(msg);
        self.path.pop();
    }

    fn elem<T: Decode>(&mut self, i: usize, v: &Value<'_>) -> Option<T> {
        self.path.push(PathStep::Index(i));
        let r = T::decode(v, self);
        self.path.pop();
        r
    }
}

/// A host type a result decodes into (`f10-config.md` 6.5). The derive of
/// F1 6.3 implements it for a struct whose fields do; a borrowed type does
/// not implement it.
pub trait Decode: Sized {
    /// The value of the type that holds `v`, or `None` after a failure.
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self>;
}

/// Decode a value into `T` (`f10-config.md` 6.1): the failures of the walk
/// of every failure against D(T), up to `max`, else the value.
pub fn decode<T: Decode + HostType>(v: &Value<'_>, max: usize) -> Result<T, Vec<Diagnostic>> {
    let ty = crate::hosttype::type_of::<T>().map_err(|e| {
        let mut d = Diagnostic::bare(Phase::Run, "not-conforming", e.0);
        d.class = Some("value");
        vec![d]
    })?;
    let ty = ty.into_ty();
    let all = crate::conform::all(v, &ty, max.max(1));
    if !all.is_empty() {
        return Err(all.iter().map(|f| f.diagnostic()).collect());
    }
    if depth(v, 0) > MAX_DEPTH {
        let mut d = Diagnostic::bare(
            Phase::Run,
            "depth-exceeded",
            "the value nests deeper than 200 levels",
        );
        d.class = Some("limit");
        return Err(vec![d]);
    }
    let mut cx = DecodeCx::default();
    match T::decode(v, &mut cx) {
        Some(x) if cx.failures.is_empty() => Ok(x),
        _ => Err(cx.failures),
    }
}

/// The nesting of a value, up to one past the bound.
fn depth(v: &Value<'_>, d: usize) -> usize {
    if d > MAX_DEPTH {
        return d;
    }
    match v.kind() {
        Kind::List => (0..v.len().unwrap_or(0))
            .map(|i| depth(&v.elem(i), d + 1))
            .max()
            .unwrap_or(d + 1),
        Kind::Map => v
            .entries()
            .iter()
            .map(|(_, x)| depth(x, d + 1))
            .max()
            .unwrap_or(d + 1),
        _ => d,
    }
}

fn kind_fail<T>(cx: &mut DecodeCx, what: &str) -> Option<T> {
    cx.fail(format!("the value is not {what}"));
    None
}

impl Decode for bool {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        match v {
            Value::Bool(b) => Some(*b),
            _ => kind_fail(cx, "a bool"),
        }
    }
}

impl Decode for String {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        match v.as_str() {
            Some(s) => Some(s.to_string()),
            None => kind_fail(cx, "a string"),
        }
    }
}

macro_rules! integer {
    ($($t:ty),*) => {$(
        impl Decode for $t {
            fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
                let Value::Num(n) = v else {
                    return kind_fail(cx, "a number");
                };
                // 6.1 item 7: whole and within the type's range.
                if n.fract() == 0.0 && *n >= <$t>::MIN as f64 && *n <= <$t>::MAX as f64 {
                    Some(*n as $t)
                } else {
                    cx.fail(format!(
                        "{} does not fit the host type {}",
                        crate::number::to_text(*n),
                        stringify!($t)
                    ));
                    None
                }
            }
        }
    )*};
}

integer!(i8, i16, i32, i64, isize, u8, u16, u32, u64, usize);

impl Decode for f64 {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        match v {
            Value::Num(n) => Some(*n),
            _ => kind_fail(cx, "a number"),
        }
    }
}

impl Decode for f32 {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        let Value::Num(n) = v else {
            return kind_fail(cx, "a number");
        };
        // 6.1 item 11: rounded to nearest, ties to even; beyond the largest
        // finite value of `f32`, a failure.
        let x = *n as f32;
        if x.is_finite() {
            Some(x)
        } else {
            cx.fail(format!(
                "{} does not fit the host type f32",
                crate::number::to_text(*n)
            ));
            None
        }
    }
}

fn list<T: Decode>(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Vec<T>> {
    if v.kind() != Kind::List {
        return kind_fail(cx, "a list");
    }
    let mut out = Vec::new();
    let mut ok = true;
    for i in 0..v.len().unwrap_or(0) {
        match cx.elem::<T>(i, &v.elem(i)) {
            Some(x) => out.push(x),
            None => ok = false,
        }
    }
    ok.then_some(out)
}

impl<T: Decode> Decode for Vec<T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        list(v, cx)
    }
}

impl<T: Decode> Decode for VecDeque<T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        list(v, cx).map(VecDeque::from)
    }
}

impl<T: Decode, const N: usize> Decode for [T; N] {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        let n = v.len().unwrap_or(0);
        if v.kind() == Kind::List && n != N {
            cx.fail(format!(
                "a list of {n} elements does not fit an array of {N}"
            ));
            return None;
        }
        list::<T>(v, cx).and_then(|l| l.try_into().ok())
    }
}

fn map<T: Decode>(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Vec<(String, T)>> {
    if v.kind() != Kind::Map {
        return kind_fail(cx, "a map");
    }
    let mut out = Vec::new();
    let mut ok = true;
    for (k, x) in v.entries() {
        match cx.key::<T>(k.as_str(), &x) {
            Some(y) => out.push((k.as_str().to_string(), y)),
            None => ok = false,
        }
    }
    ok.then_some(out)
}

impl<T: Decode> Decode for HashMap<String, T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        map(v, cx).map(|m| m.into_iter().collect())
    }
}

impl<T: Decode> Decode for BTreeMap<String, T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        map(v, cx).map(|m| m.into_iter().collect())
    }
}

impl<T: Decode> Decode for Option<T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        match v {
            Value::Null => Some(None),
            _ => T::decode(v, cx).map(Some),
        }
    }
}

impl<T: Decode> Decode for Box<T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        T::decode(v, cx).map(Box::new)
    }
}

impl<T: Decode> Decode for Rc<T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        T::decode(v, cx).map(Rc::new)
    }
}

impl<T: Decode> Decode for Arc<T> {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        T::decode(v, cx).map(Arc::new)
    }
}

impl Decode for Data {
    fn decode(v: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        // 6.1 item 8: an opaque value is not data.
        fn opaque(v: &Value<'_>) -> bool {
            match v.kind() {
                Kind::List => (0..v.len().unwrap_or(0)).any(|i| opaque(&v.elem(i))),
                Kind::Map => v.entries().iter().any(|(_, x)| opaque(x)),
                _ => matches!(v, Value::Opaque(_)),
            }
        }
        if opaque(v) {
            cx.fail("an opaque value is not decoded");
            return None;
        }
        Some(v.to_data())
    }
}

impl<T: OpaqueType> Decode for Opaque<T> {
    fn decode(_: &Value<'_>, cx: &mut DecodeCx) -> Option<Self> {
        cx.fail("an opaque value is not decoded");
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::host::Data;

    fn v(d: Data) -> Value<'static> {
        Value::from_data(&d)
    }

    fn map(entries: &[(&str, Data)]) -> Data {
        Data::Map(
            entries
                .iter()
                .map(|(k, x)| (k.to_string(), x.clone()))
                .collect(),
        )
    }

    #[test]
    fn scalars_and_lists() {
        assert_eq!(decode::<bool>(&v(Data::Bool(true)), 1).ok(), Some(true));
        assert_eq!(
            decode::<String>(&v(Data::Str("a".into())), 1).ok(),
            Some("a".into())
        );
        assert_eq!(decode::<i64>(&v(Data::Float(3.0)), 1).ok(), Some(3));
        assert_eq!(decode::<f64>(&v(Data::Float(1.5)), 1).ok(), Some(1.5));
        let l = Data::List(vec![Data::Float(1.0), Data::Float(2.0)]);
        assert_eq!(decode::<Vec<u8>>(&v(l.clone()), 1).ok(), Some(vec![1, 2]));
        assert_eq!(decode::<[u8; 2]>(&v(l.clone()), 1).ok(), Some([1, 2]));
        assert!(decode::<[u8; 3]>(&v(l), 1).is_err());
    }

    #[test]
    fn narrower_than_the_type() {
        // 6.1 item 7: `int` for every integer type, the range by the host's.
        let e = decode::<u8>(&v(Data::Float(300.0)), 1).unwrap_err();
        assert_eq!(e[0].code, "not-conforming");
        assert!(decode::<f32>(&v(Data::Float(1e300)), 1).is_err());
        assert_eq!(decode::<f32>(&v(Data::Float(0.1)), 1).ok(), Some(0.1f32));
        // Not whole: the walk's own failure.
        let e = decode::<i32>(&v(Data::Float(1.5)), 1).unwrap_err();
        assert_eq!(e[0].message, "expected int, found 1.5");
    }

    #[test]
    fn null_and_options() {
        assert_eq!(decode::<Option<i64>>(&v(Data::Null), 1).ok(), Some(None));
        let e = decode::<i64>(&v(Data::Null), 1).unwrap_err();
        assert_eq!(e[0].message, "expected int, found null");
        assert_eq!(e[0].path.as_deref().map(|p| p.len()), Some(0));
    }

    #[test]
    fn maps_and_data() {
        let m = map(&[("a", Data::Float(1.0)), ("b", Data::Float(2.0))]);
        let got = decode::<BTreeMap<String, i64>>(&v(m.clone()), 1).unwrap();
        assert_eq!(got.get("b"), Some(&2));
        assert_eq!(
            decode::<HashMap<String, i64>>(&v(m.clone()), 1)
                .unwrap()
                .len(),
            2
        );
        assert_eq!(decode::<Data>(&v(m.clone()), 1).ok(), Some(m));
        let e = decode::<BTreeMap<String, i64>>(&v(map(&[("a", Data::Str("x".into()))])), 1)
            .unwrap_err();
        assert_eq!(
            e[0].path.as_deref().cloned(),
            Some(vec![PathStep::Key("a".into())])
        );
    }

    #[test]
    fn every_failure_up_to_the_maximum() {
        let m = map(&[("a", Data::Str("x".into())), ("b", Data::Str("y".into()))]);
        assert_eq!(
            decode::<BTreeMap<String, i64>>(&v(m.clone()), 5)
                .unwrap_err()
                .len(),
            2
        );
        assert_eq!(
            decode::<BTreeMap<String, i64>>(&v(m), 1).unwrap_err().len(),
            1
        );
    }
}
