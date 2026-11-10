//! Package F1: types from host types (`spec/packages/f01-host-types.md` 6).
//!
//! [`HostType`] gives the type of a Rust type, and [`Host`] (with
//! [`HostList`] and [`HostMap`]) presents its values to a run, in place. The
//! two agree: every value of a type conforms to its derived type as it is
//! presented (6.1). This module implements both for the standard types of
//! the table of 6.3. The derive `Host` of the crate `risor-derive`
//! implements both for a struct with named fields.
//!
//! | Rust | Type | Presented as |
//! | --- | --- | --- |
//! | A type wrapped in [`Opaque`] | Its opaque name ([`OpaqueType::NAME`]) | The opaque value, one identity for each `Opaque` |
//! | `bool`, `String`, `str`, `Cow<str>` | `bool`, `string` | Itself |
//! | Every integer type | `int` | Itself; outside ±(2^53 − 1) it cannot enter |
//! | `f32`, `f64` | `float` | Itself |
//! | `Vec<T>`, `[T]`, `[T; N]`, `VecDeque<T>` | `T[]` | A list |
//! | `HashMap<String, T>`, `BTreeMap<String, T>` | `map<T>` | A map |
//! | `Option<T>` | `T \| null` | `None` is null |
//! | `&T`, `Box<T>`, `Rc<T>`, `Arc<T>` | The type of `T` | As `T` |
//! | [`Data`], the crate's owned value | `any` | Itself |
//! | A struct with `#[derive(risor_derive::Host)]` | An exact record | A map of its fields |
//!
//! [`Host`]: crate::host::Host

use crate::check::ty::{join, Field, Rec, Ty};
use crate::host::{Data, Host, HostList, HostMap, OpaqueRef, View};
use std::any::Any;
use std::borrow::Cow;
use std::collections::{BTreeMap, BTreeSet, HashMap, VecDeque};
use std::fmt;
use std::marker::PhantomData;
use std::rc::Rc;
use std::sync::Arc;

/// The deepest a derived type may be (`05-types.md` 2.5).
const MAX_DEPTH: usize = 64;

/// A Rust type with a Risor type (6.1).
pub trait HostType {
    /// The type. Call it through [`TypeCx::of`], which finds a type that
    /// contains itself.
    fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError>;
}

/// Why a type could not be derived: the `declare` error `bad-type` when an
/// environment declares it.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct TypeError(pub String);

impl fmt::Display for TypeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for TypeError {}

/// The state of one derivation: the records in progress, for the rule that
/// a type cannot contain itself, and the records already derived, so a
/// struct used many times is derived once and its type shared.
#[derive(Default)]
pub struct TypeCx {
    in_progress: Vec<&'static str>,
    done: HashMap<&'static str, Ty>,
}

impl TypeCx {
    pub fn new() -> TypeCx {
        TypeCx::default()
    }

    /// The type of `T`.
    pub fn of<T: HostType + ?Sized>(&mut self) -> Result<Ty, TypeError> {
        let t = T::host_type(self)?;
        // Ruling CG8: each host type's derived type is tested.
        if t.too_big() {
            return Err(TypeError(format!(
                "the type of {} is larger than {}",
                std::any::type_name::<T>(),
                crate::check::ty::SIZE_LIMIT
            )));
        }
        Ok(t)
    }

    /// The exact record of the struct `S`, whose fields `fields` adds. The
    /// derive calls this. A struct met again while its own record is being
    /// derived is `bad-type`: a type cannot refer to itself.
    pub fn record<S: ?Sized>(
        &mut self,
        fields: impl FnOnce(&mut TypeCx, &mut RecordType) -> Result<(), TypeError>,
    ) -> Result<Ty, TypeError> {
        let name = std::any::type_name::<S>();
        if let Some(t) = self.done.get(name) {
            return Ok(t.clone());
        }
        if self.in_progress.contains(&name) {
            return Err(TypeError(format!(
                "the type {name} contains itself: a type cannot refer to itself"
            )));
        }
        if self.in_progress.len() >= MAX_DEPTH {
            return Err(TypeError(format!(
                "the type {name} is deeper than {MAX_DEPTH}"
            )));
        }
        self.in_progress.push(name);
        let mut r = RecordType::default();
        let out = fields(self, &mut r);
        self.in_progress.pop();
        out?;
        let t = Ty::rec(Rec::new(r.fields, Ty::never()));
        self.done.insert(name, t.clone());
        Ok(t)
    }
}

/// The fields of a record being derived.
#[derive(Default)]
pub struct RecordType {
    fields: BTreeMap<String, Field>,
}

impl RecordType {
    /// A field under `key`: optional when the binding may present it as
    /// absent (6.1).
    pub fn field(&mut self, key: &str, ty: Ty, optional: bool) -> Result<(), TypeError> {
        if self
            .fields
            .insert(key.to_string(), Field { ty, optional })
            .is_some()
        {
            return Err(TypeError(format!("two fields are presented as {key:?}")));
        }
        Ok(())
    }
}

/// The type of `T`, at most 64 deep (6.1, row Limits).
pub fn type_of<T: HostType + ?Sized>() -> Result<Ty, TypeError> {
    let t = TypeCx::new().of::<T>()?;
    // `05-types.md` 20.7: a struct used many times is shared, so its type
    // written out can pass the size limit.
    if t.too_big() {
        return Err(TypeError(format!(
            "the type of {} is larger than {}",
            std::any::type_name::<T>(),
            crate::check::ty::SIZE_LIMIT
        )));
    }
    if depth(&t, &mut HashMap::new()) > MAX_DEPTH {
        return Err(TypeError(format!(
            "the type of {} is deeper than {MAX_DEPTH}",
            std::any::type_name::<T>()
        )));
    }
    Ok(t)
}

/// The description of the type of `T`: its printed text (5.1).
pub fn describe_host_type<T: HostType + ?Sized>() -> Result<String, TypeError> {
    type_of::<T>().map(|t| crate::check::ty::print(&t))
}

/// The depth of a type, with shared parts measured once.
pub(crate) fn depth(t: &Ty, memo: &mut HashMap<usize, usize>) -> usize {
    let mut d = 0;
    if let Some(e) = &t.list {
        let k = Arc::as_ptr(e) as usize;
        let x = match memo.get(&k) {
            Some(x) => *x,
            None => {
                let x = depth(e, memo);
                memo.insert(k, x);
                x
            }
        };
        d = d.max(1 + x);
    }
    if let Some(r) = &t.map {
        let k = Arc::as_ptr(r) as usize;
        let x = match memo.get(&k) {
            Some(x) => *x,
            None => {
                let mut x = depth(&r.rest, memo);
                for f in r.fields.values() {
                    x = x.max(depth(&f.ty, memo));
                }
                memo.insert(k, x);
                x
            }
        };
        d = d.max(1 + x);
    }
    d
}

/// The opaque names a type uses, with shared parts visited once.
pub(crate) fn opaque_names(t: &Ty, out: &mut BTreeSet<String>, seen: &mut BTreeSet<usize>) {
    out.extend(t.opaque.iter().cloned());
    if let Some(e) = &t.list {
        if seen.insert(Arc::as_ptr(e) as usize) {
            opaque_names(e, out, seen);
        }
    }
    if let Some(r) = &t.map {
        if seen.insert(Arc::as_ptr(r) as usize) {
            opaque_names(&r.rest, out, seen);
            for f in r.fields.values() {
                opaque_names(&f.ty, out, seen);
            }
        }
    }
}

// ---- opaque types -------------------------------------------------------------

/// A Rust type registered as opaque: values of it enter a run as opaque
/// values of the type `NAME`, which the environment must declare. The name
/// is never guessed (6.1, row Opaque).
pub trait OpaqueType: Any + Send + Sync {
    const NAME: &'static str;
}

/// A value of an opaque type, as a field or element of host data. It holds
/// one allocation, so every read presents the same host value
/// (`03-host.md` 3.4.6).
pub struct Opaque<T: OpaqueType> {
    value: OpaqueRef,
    _t: PhantomData<fn() -> T>,
}

impl<T: OpaqueType> Opaque<T> {
    pub fn new(v: T) -> Opaque<T> {
        Opaque {
            value: OpaqueRef::new(T::NAME, v),
            _t: PhantomData,
        }
    }

    pub fn get(&self) -> &T {
        self.value
            .value
            .downcast_ref::<T>()
            .expect("an Opaque<T> holds a T")
    }

    pub fn opaque_ref(&self) -> &OpaqueRef {
        &self.value
    }
}

impl<T: OpaqueType> Clone for Opaque<T> {
    fn clone(&self) -> Self {
        Opaque {
            value: self.value.clone(),
            _t: PhantomData,
        }
    }
}

impl<T: OpaqueType> fmt::Debug for Opaque<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.value.fmt(f)
    }
}

impl<T: OpaqueType> HostType for Opaque<T> {
    fn host_type(_: &mut TypeCx) -> Result<Ty, TypeError> {
        Ok(Ty::opaque(T::NAME))
    }
}

impl<T: OpaqueType> Host for Opaque<T> {
    fn view(&self) -> View<'_> {
        View::Opaque(self.value.clone())
    }
}

// ---- absent_if_none -----------------------------------------------------------

/// The types a field marked `#[risor(absent_if_none)]` may have: `Option<T>`
/// alone. The derive names it, so the mark on another type fails the build.
pub trait OptionLike {
    type Inner: HostType + Host + ?Sized;
    fn present(&self) -> Option<&Self::Inner>;
}

impl<T: HostType + Host> OptionLike for Option<T> {
    type Inner = T;
    fn present(&self) -> Option<&T> {
        self.as_ref()
    }
}

// ---- the standard types --------------------------------------------------------

macro_rules! scalar {
    ($ty:expr, $($t:ty),*) => {$(
        impl HostType for $t {
            fn host_type(_: &mut TypeCx) -> Result<Ty, TypeError> {
                Ok($ty)
            }
        }
    )*};
}

scalar!(Ty::boolean(), bool);
scalar!(Ty::string(), String, str);
scalar!(
    Ty::int(),
    i8,
    i16,
    i32,
    i64,
    i128,
    isize,
    u8,
    u16,
    u32,
    u64,
    u128,
    usize
);
scalar!(Ty::float(), f32, f64);
scalar!(Ty::any(), Data);

impl HostType for Cow<'_, str> {
    fn host_type(_: &mut TypeCx) -> Result<Ty, TypeError> {
        Ok(Ty::string())
    }
}

impl Host for bool {
    fn view(&self) -> View<'_> {
        View::Bool(*self)
    }
}

impl Host for String {
    fn view(&self) -> View<'_> {
        View::Str(self)
    }
}

impl Host for str {
    fn view(&self) -> View<'_> {
        View::Str(self)
    }
}

impl Host for Cow<'_, str> {
    fn view(&self) -> View<'_> {
        View::Str(self)
    }
}

macro_rules! signed {
    ($($t:ty),*) => {$(
        impl Host for $t {
            fn view(&self) -> View<'_> {
                View::Int(*self as i64)
            }
        }
    )*};
}
macro_rules! unsigned {
    ($($t:ty),*) => {$(
        impl Host for $t {
            fn view(&self) -> View<'_> {
                View::UInt(*self as u64)
            }
        }
    )*};
}
signed!(i8, i16, i32, i64, isize);
unsigned!(u8, u16, u32, u64, usize);

/// Outside the range of `i64` a value is far outside ±(2^53 − 1), so it is
/// presented at the end of that range and cannot enter, as 6.1 says.
impl Host for i128 {
    fn view(&self) -> View<'_> {
        View::Int((*self).clamp(i64::MIN as i128, i64::MAX as i128) as i64)
    }
}

impl Host for u128 {
    fn view(&self) -> View<'_> {
        View::UInt((*self).min(u64::MAX as u128) as u64)
    }
}

impl Host for f32 {
    fn view(&self) -> View<'_> {
        View::Float(*self as f64)
    }
}

impl Host for f64 {
    fn view(&self) -> View<'_> {
        View::Float(*self)
    }
}

impl<T: HostType> HostType for Option<T> {
    fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError> {
        Ok(join(&cx.of::<T>()?, &Ty::null()))
    }
}

impl<T: Host> Host for Option<T> {
    fn view(&self) -> View<'_> {
        match self {
            Some(v) => v.view(),
            None => View::Null,
        }
    }
}

macro_rules! pointer {
    ($($p:ident),*) => {$(
        impl<T: HostType + ?Sized> HostType for $p<T> {
            fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError> {
                cx.of::<T>()
            }
        }
        impl<T: Host + ?Sized> Host for $p<T> {
            fn view(&self) -> View<'_> {
                (**self).view()
            }
        }
    )*};
}
pointer!(Box, Rc, Arc);

impl<T: HostType + ?Sized> HostType for &T {
    fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError> {
        cx.of::<T>()
    }
}

impl<T: Host + ?Sized> Host for &T {
    fn view(&self) -> View<'_> {
        (**self).view()
    }
}

macro_rules! list {
    ($([$($g:tt)*] $t:ty),*) => {$(
        impl<$($g)*> HostType for $t {
            fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError> {
                Ok(Ty::list(cx.of::<T>()?))
            }
        }
        impl<$($g)*> Host for $t {
            fn view(&self) -> View<'_> {
                View::List(self)
            }
        }
        impl<$($g)*> HostList for $t {
            fn len(&self) -> usize {
                <[T]>::len(&self[..])
            }
            fn get(&self, index: usize) -> Option<View<'_>> {
                self[..].get(index).map(Host::view)
            }
        }
    )*};
}
list!([T: HostType + Host] Vec<T>, [T: HostType + Host, const N: usize] [T; N]);

impl<T: HostType> HostType for [T] {
    fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError> {
        Ok(Ty::list(cx.of::<T>()?))
    }
}

/// A slice is unsized, so a borrowed slice is what presents it.
impl<T: Host> Host for &[T] {
    fn view(&self) -> View<'_> {
        View::List(self)
    }
}

impl<T: Host> HostList for &[T] {
    fn len(&self) -> usize {
        <[T]>::len(self)
    }
    fn get(&self, index: usize) -> Option<View<'_>> {
        <[T]>::get(self, index).map(Host::view)
    }
}

impl<T: HostType + Host> HostType for VecDeque<T> {
    fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError> {
        Ok(Ty::list(cx.of::<T>()?))
    }
}

impl<T: HostType + Host> Host for VecDeque<T> {
    fn view(&self) -> View<'_> {
        View::List(self)
    }
}

impl<T: HostType + Host> HostList for VecDeque<T> {
    fn len(&self) -> usize {
        VecDeque::len(self)
    }
    fn get(&self, index: usize) -> Option<View<'_>> {
        VecDeque::get(self, index).map(Host::view)
    }
}

macro_rules! map {
    ($($m:ident),*) => {$(
        impl<T: HostType + Host> HostType for $m<String, T> {
            fn host_type(cx: &mut TypeCx) -> Result<Ty, TypeError> {
                Ok(Ty::map_of(cx.of::<T>()?))
            }
        }
        impl<T: HostType + Host> Host for $m<String, T> {
            fn view(&self) -> View<'_> {
                View::Map(self)
            }
        }
        impl<T: HostType + Host> HostMap for $m<String, T> {
            fn len(&self) -> usize {
                $m::len(self)
            }
            fn get(&self, key: &str) -> Option<View<'_>> {
                $m::get(self, key).map(Host::view)
            }
            fn keys(&self) -> Vec<&str> {
                $m::keys(self).map(String::as_str).collect()
            }
        }
    )*};
}
map!(HashMap, BTreeMap);

#[cfg(test)]
mod tests {
    use super::*;
    use crate::check::ty::print;

    struct Time;
    impl OpaqueType for Time {
        const NAME: &'static str = "Time";
    }

    fn text<T: HostType + ?Sized>() -> String {
        describe_host_type::<T>().unwrap_or_else(|e| format!("error: {e}"))
    }

    #[test]
    fn the_rows_of_the_table() {
        assert_eq!(text::<bool>(), "bool");
        assert_eq!(text::<String>(), "string");
        assert_eq!(text::<&str>(), "string");
        assert_eq!(text::<Cow<str>>(), "string");
        assert_eq!(text::<u8>(), "int");
        assert_eq!(text::<i128>(), "int");
        assert_eq!(text::<usize>(), "int");
        assert_eq!(text::<f32>(), "float");
        assert_eq!(text::<Vec<i64>>(), "int[]");
        assert_eq!(text::<[String]>(), "string[]");
        assert_eq!(text::<[f64; 3]>(), "float[]");
        assert_eq!(text::<VecDeque<bool>>(), "bool[]");
        assert_eq!(text::<HashMap<String, f64>>(), "map<float>");
        assert_eq!(text::<BTreeMap<String, Vec<u8>>>(), "map<int[]>");
        assert_eq!(text::<Option<String>>(), "string | null");
        assert_eq!(text::<Option<Option<i32>>>(), "int | null");
        assert_eq!(text::<Vec<Option<i32>>>(), "(int | null)[]");
        assert_eq!(text::<Box<Arc<Rc<String>>>>(), "string");
        assert_eq!(text::<Data>(), "any");
        assert_eq!(text::<Opaque<Time>>(), "Time");
        assert_eq!(text::<Option<Opaque<Time>>>(), "Time | null");
    }

    #[test]
    fn depth_64_and_65() {
        type L1<T> = Vec<T>;
        type L4<T> = L1<L1<L1<L1<T>>>>;
        type L16<T> = L4<L4<L4<L4<T>>>>;
        type L64<T> = L16<L16<L16<L16<T>>>>;
        assert_eq!(text::<L64<i64>>(), format!("int{}", "[]".repeat(64)));
        assert!(text::<L64<Vec<i64>>>().starts_with("error"));
    }

    #[test]
    fn values_present_as_their_types() {
        let v: Vec<Option<u32>> = vec![Some(1), None];
        match v.view() {
            View::List(l) => {
                assert_eq!(l.len(), 2);
                assert!(matches!(l.get(0), Some(View::UInt(1))));
                assert!(matches!(l.get(1), Some(View::Null)));
            }
            _ => panic!("a list"),
        }
        let o = Opaque::new(Time);
        let (View::Opaque(a), View::Opaque(b)) = (o.view(), o.view()) else {
            panic!("opaque")
        };
        assert!(a.same(&b), "one identity for every read");
        assert!(matches!(u128::MAX.view(), View::UInt(u64::MAX)));
        assert_eq!(print(&type_of::<Vec<u8>>().unwrap()), "int[]");
    }
}
