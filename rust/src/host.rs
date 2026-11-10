//! Host data, read in place (`03-host.md` 3.4), as design `05` Part D
//! sketches for Rust.
//!
//! A host presents its values through [`Host`], [`HostList`] and [`HostMap`].
//! A run borrows them for its lifetime and reads each value only where the
//! program reads it: nothing is copied when a run starts. [`Data`] is a
//! ready-made host value tree for hosts that have no types of their own, and
//! the type a host function returns.

use std::any::Any;
use std::collections::BTreeMap;
use std::fmt;
use std::sync::Arc;

/// An opaque value: a host value of a declared opaque type (`03-host.md`
/// 3.4.6). Two opaque values are the same host value when they share one
/// allocation.
#[derive(Clone)]
pub struct OpaqueRef {
    pub type_name: Arc<str>,
    pub value: Arc<dyn Any + Send + Sync>,
}

impl OpaqueRef {
    pub fn new(type_name: &str, value: impl Any + Send + Sync) -> OpaqueRef {
        OpaqueRef {
            type_name: Arc::from(type_name),
            value: Arc::new(value),
        }
    }

    /// The identity test of 3.4.6.
    pub fn same(&self, other: &OpaqueRef) -> bool {
        std::ptr::addr_eq(Arc::as_ptr(&self.value), Arc::as_ptr(&other.value))
    }
}

impl fmt::Debug for OpaqueRef {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "opaque {} at {:p}",
            self.type_name,
            Arc::as_ptr(&self.value)
        )
    }
}

impl PartialEq for OpaqueRef {
    fn eq(&self, other: &Self) -> bool {
        self.same(other)
    }
}

/// What the evaluator may ask of a host value.
pub enum View<'a> {
    Null,
    Bool(bool),
    /// A value of a host integer type. Outside ±(2^53 − 1) it cannot enter.
    Int(i64),
    UInt(u64),
    /// A value of a host floating type. It enters whenever it is finite.
    Float(f64),
    Str(&'a str),
    List(&'a dyn HostList),
    Map(&'a dyn HostMap),
    Opaque(OpaqueRef),
}

pub trait Host {
    fn view(&self) -> View<'_>;
}

pub trait HostList {
    fn len(&self) -> usize;
    fn get(&self, index: usize) -> Option<View<'_>>;
    fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

pub trait HostMap {
    fn len(&self) -> usize;
    /// `None` is absent. A field present and null is `Some(View::Null)`.
    fn get(&self, key: &str) -> Option<View<'_>>;
    /// The keys, in any order. The evaluator sorts them when order shows.
    fn keys(&self) -> Vec<&str>;
    fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

/// The variables of a run, by name. A run asks for each name at most once,
/// and only for the names it reads (`03-host.md` 3.4.1).
pub trait Vars {
    fn get(&self, name: &str) -> Option<View<'_>>;
}

/// No variables.
pub struct NoVars;

impl Vars for NoVars {
    fn get(&self, _: &str) -> Option<View<'_>> {
        None
    }
}

/// A tree of host values.
#[derive(Clone, Debug, PartialEq)]
pub enum Data {
    Null,
    Bool(bool),
    Int(i64),
    Float(f64),
    Str(String),
    List(Vec<Data>),
    Map(BTreeMap<String, Data>),
    Opaque(OpaqueRef),
}

impl Host for Data {
    fn view(&self) -> View<'_> {
        match self {
            Data::Null => View::Null,
            Data::Bool(b) => View::Bool(*b),
            Data::Int(i) => View::Int(*i),
            Data::Float(f) => View::Float(*f),
            Data::Str(s) => View::Str(s),
            Data::List(l) => View::List(l),
            Data::Map(m) => View::Map(m),
            Data::Opaque(o) => View::Opaque(o.clone()),
        }
    }
}

/// Variables from a map of names to data. (It is not generic, so that an
/// empty `BTreeMap::new()` needs no annotation; a map of other host values
/// is a `HashMap`.)
impl Vars for BTreeMap<String, Data> {
    fn get(&self, name: &str) -> Option<View<'_>> {
        BTreeMap::get(self, name).map(Host::view)
    }
}

/// Variables from a map of names to host values.
impl<T: Host> Vars for std::collections::HashMap<String, T> {
    fn get(&self, name: &str) -> Option<View<'_>> {
        std::collections::HashMap::get(self, name).map(Host::view)
    }
}

/// The error a host function returns: `function-error` with this text.
#[derive(Clone, Debug, PartialEq)]
pub struct HostError(pub String);

impl fmt::Display for HostError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for HostError {}
