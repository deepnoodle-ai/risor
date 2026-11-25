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
    /// The declared opaque type name.
    pub type_name: Arc<str>,
    /// The underlying shared host value.
    pub value: Arc<dyn Any + Send + Sync>,
}

impl OpaqueRef {
    /// Wrap `value` as an opaque host value carrying `type_name`.
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
    /// The language's `null`.
    Null,
    /// A boolean value.
    Bool(bool),
    /// A value of a host integer type. Outside ±(2^53 − 1) it cannot enter.
    Int(i64),
    /// An unsigned host integer. Above 2^53 − 1 it cannot enter.
    UInt(u64),
    /// A value of a host floating type. It enters whenever it is finite.
    Float(f64),
    /// A borrowed UTF-8 string.
    Str(&'a str),
    /// A borrowed sequence read in place.
    List(&'a dyn HostList),
    /// A borrowed string-keyed map or record read in place.
    Map(&'a dyn HostMap),
    /// An opaque host value carrying its declared type name.
    Opaque(OpaqueRef),
}

/// A host value that the evaluator can read in place (`03-host.md` 3.4.2).
pub trait Host {
    /// Present this value to the evaluator.
    fn view(&self) -> View<'_>;
}

/// A host sequence read in place by index (`03-host.md` 3.4.3).
pub trait HostList {
    /// Number of elements in the list.
    fn len(&self) -> usize;
    /// Element at `index`, or `None` if out of bounds.
    fn get(&self, index: usize) -> Option<View<'_>>;
    /// Whether the list has no elements.
    fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

/// A host string-keyed mapping or struct read in place by key (`03-host.md` 3.4.3).
pub trait HostMap {
    /// Number of present entries in the map.
    fn len(&self) -> usize;
    /// `None` is absent. A field present and null is `Some(View::Null)`.
    fn get(&self, key: &str) -> Option<View<'_>>;
    /// The keys, in any order. The evaluator sorts them when order shows.
    fn keys(&self) -> Vec<&str>;
    /// Whether the map has no present entries.
    fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

/// The variables of a run, by name. A run asks for each name at most once,
/// and only for the names it reads (`03-host.md` 3.4.1).
pub trait Vars {
    /// Return the value for variable `name`, or `None` if not supplied.
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
    /// The language's `null`.
    Null,
    /// A boolean value.
    Bool(bool),
    /// A signed integer value.
    Int(i64),
    /// A floating-point value.
    Float(f64),
    /// An owned string value.
    Str(String),
    /// An owned list of values.
    List(Vec<Data>),
    /// An owned map from string keys to values.
    Map(BTreeMap<String, Data>),
    /// An opaque host value.
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
        crate::hosttype::btree_get(self, name).map(Host::view)
    }
}

/// Variables from a map of names to host values.
impl<T: Host> Vars for std::collections::HashMap<String, T> {
    fn get(&self, name: &str) -> Option<View<'_>> {
        std::collections::HashMap::get(self, name).map(Host::view)
    }
}

/// The error a host function returns. The run sees `function-error`
/// (`03-host.md` 3.5.3).
///
/// A failure of this binding converts with `?`: a host function that runs a
/// nested program returns its [`Error`](crate::Error) as it is.
#[derive(Clone, Debug, PartialEq)]
pub enum HostError {
    /// Any other error. Its text is the message of `function-error`.
    Text(String),
    /// A failure of a binding of Risor, such as that of a nested run
    /// (3.5.3 item 5). The message of `function-error` is the message of its
    /// first diagnostic, the inner diagnostic, whole; the inner diagnostic
    /// is the run diagnostic's [`cause`](crate::Diagnostic::cause). Only the
    /// message reaches the run: the code stays `function-error`, and the
    /// inner code, class, span, path and data do not.
    Risor(Box<crate::diag::Error>),
}

impl HostError {
    /// The message of the run's `function-error`: the text, or the message
    /// of the inner diagnostic.
    pub fn message(&self) -> String {
        match self {
            HostError::Text(t) => t.clone(),
            HostError::Risor(e) => match e.diagnostics.first() {
                Some(d) => d.message.clone(),
                None => e.to_string(),
            },
        }
    }
}

impl From<String> for HostError {
    fn from(t: String) -> HostError {
        HostError::Text(t)
    }
}

impl From<&str> for HostError {
    fn from(t: &str) -> HostError {
        HostError::Text(t.to_string())
    }
}

impl From<crate::diag::Error> for HostError {
    fn from(e: crate::diag::Error) -> HostError {
        HostError::Risor(Box::new(e))
    }
}

impl fmt::Display for HostError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            HostError::Text(t) => f.write_str(t),
            HostError::Risor(e) => write!(f, "{e}"),
        }
    }
}

impl std::error::Error for HostError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            HostError::Text(_) => None,
            HostError::Risor(e) => Some(e.as_ref()),
        }
    }
}
