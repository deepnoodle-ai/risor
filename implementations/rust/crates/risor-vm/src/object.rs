//! Risor object system - runtime values for the VM.

use std::cell::RefCell;
use std::collections::HashMap;
use std::fmt;
use std::rc::Rc;

use risor_bytecode::Code;

/// Object types for runtime values.
#[derive(Debug, Clone)]
pub enum Object {
    Nil,
    Bool(bool),
    Int(i64),
    Float(f64),
    String(Rc<str>),
    List(Rc<RefCell<Vec<Object>>>),
    Map(Rc<RefCell<RisorMap>>),
    Closure(Rc<Closure>),
    Builtin(Rc<Builtin>),
    Cell(Rc<RefCell<Object>>),
    Iter(Rc<RefCell<RisorIter>>),
    Error(Rc<str>),
}

impl Object {
    /// Get the type name of this object.
    pub fn type_name(&self) -> &'static str {
        match self {
            Object::Nil => "nil",
            Object::Bool(_) => "bool",
            Object::Int(_) => "int",
            Object::Float(_) => "float",
            Object::String(_) => "string",
            Object::List(_) => "list",
            Object::Map(_) => "map",
            Object::Closure(_) => "closure",
            Object::Builtin(_) => "builtin",
            Object::Cell(_) => "cell",
            Object::Iter(_) => "iter",
            Object::Error(_) => "error",
        }
    }

    /// Check if this object is truthy.
    pub fn is_truthy(&self) -> bool {
        match self {
            Object::Nil => false,
            Object::Bool(b) => *b,
            Object::Int(n) => *n != 0,
            Object::Float(n) => *n != 0.0,
            Object::String(s) => !s.is_empty(),
            Object::List(items) => !items.borrow().is_empty(),
            Object::Map(map) => !map.borrow().is_empty(),
            _ => true,
        }
    }

    /// Check equality with another object.
    pub fn equals(&self, other: &Object) -> bool {
        match (self, other) {
            (Object::Nil, Object::Nil) => true,
            (Object::Bool(a), Object::Bool(b)) => a == b,
            (Object::Int(a), Object::Int(b)) => a == b,
            (Object::Int(a), Object::Float(b)) => (*a as f64) == *b,
            (Object::Float(a), Object::Int(b)) => *a == (*b as f64),
            (Object::Float(a), Object::Float(b)) => a == b,
            (Object::String(a), Object::String(b)) => a == b,
            (Object::List(a), Object::List(b)) => {
                let a = a.borrow();
                let b = b.borrow();
                if a.len() != b.len() {
                    return false;
                }
                a.iter().zip(b.iter()).all(|(x, y)| x.equals(y))
            }
            (Object::Map(a), Object::Map(b)) => {
                let a = a.borrow();
                let b = b.borrow();
                if a.len() != b.len() {
                    return false;
                }
                a.entries()
                    .iter()
                    .all(|(k, v)| b.get(k).map(|bv| v.equals(&bv)).unwrap_or(false))
            }
            (Object::Error(a), Object::Error(b)) => a == b,
            _ => false,
        }
    }

    /// Get hash key for map usage.
    pub fn hash_key(&self) -> Option<String> {
        match self {
            Object::Nil => Some("nil".to_string()),
            Object::Bool(b) => Some(if *b { "true" } else { "false" }.to_string()),
            Object::Int(n) => Some(format!("int:{}", n)),
            Object::Float(n) => Some(format!("float:{}", n)),
            Object::String(s) => Some(format!("str:{}", s)),
            _ => None,
        }
    }

    /// Compare two objects for ordering.
    pub fn compare(&self, other: &Object) -> Option<std::cmp::Ordering> {
        match (self, other) {
            (Object::Int(a), Object::Int(b)) => Some(a.cmp(b)),
            (Object::Int(a), Object::Float(b)) => (*a as f64).partial_cmp(b),
            (Object::Float(a), Object::Int(b)) => a.partial_cmp(&(*b as f64)),
            (Object::Float(a), Object::Float(b)) => a.partial_cmp(b),
            (Object::String(a), Object::String(b)) => Some(a.cmp(b)),
            _ => None,
        }
    }

    /// Convert to number.
    pub fn as_number(&self) -> Option<f64> {
        match self {
            Object::Int(n) => Some(*n as f64),
            Object::Float(n) => Some(*n),
            _ => None,
        }
    }
}

impl fmt::Display for Object {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Object::Nil => write!(f, "nil"),
            Object::Bool(b) => write!(f, "{}", b),
            Object::Int(n) => write!(f, "{}", n),
            Object::Float(n) => write!(f, "{}", n),
            Object::String(s) => write!(f, "{}", s),
            Object::List(items) => {
                let items = items.borrow();
                let parts: Vec<String> = items.iter().map(|i| format!("{}", i)).collect();
                write!(f, "[{}]", parts.join(", "))
            }
            Object::Map(map) => {
                let map = map.borrow();
                let pairs: Vec<String> = map
                    .entries()
                    .iter()
                    .map(|(k, v)| format!("{}: {}", k, v))
                    .collect();
                write!(f, "{{{}}}", pairs.join(", "))
            }
            Object::Closure(c) => {
                let name = if c.code.name.is_empty() {
                    "anonymous"
                } else {
                    &c.code.name
                };
                write!(f, "<closure: {}>", name)
            }
            Object::Builtin(b) => write!(f, "<builtin: {}>", b.name),
            Object::Cell(c) => write!(f, "<cell: {}>", c.borrow()),
            Object::Iter(i) => write!(f, "<iter: {} items>", i.borrow().remaining()),
            Object::Error(e) => write!(f, "error: {}", e),
        }
    }
}

/// Closure - a function with captured free variables.
#[derive(Debug)]
pub struct Closure {
    pub code: Rc<Code>,
    pub free_vars: Vec<Rc<RefCell<Object>>>,
}

impl Closure {
    pub fn new(code: Rc<Code>, free_vars: Vec<Rc<RefCell<Object>>>) -> Self {
        Self { code, free_vars }
    }
}

/// Built-in function signature (function pointer, for non-capturing builtins).
pub type BuiltinFn = fn(&mut dyn BuiltinContext, &[Object]) -> Result<Object, String>;

/// Built-in function signature (boxed closure, for capturing builtins like methods).
pub type BuiltinClosureFn = Box<dyn Fn(&mut dyn BuiltinContext, &[Object]) -> Result<Object, String>>;

/// Context provided to builtin functions.
pub trait BuiltinContext {
    fn call_function(&mut self, func: &Object, args: &[Object]) -> Result<Object, String>;
}

/// Built-in function wrapper.
pub struct Builtin {
    pub name: String,
    func: BuiltinKind,
}

enum BuiltinKind {
    Fn(BuiltinFn),
    Closure(BuiltinClosureFn),
}

impl Builtin {
    pub fn new(name: impl Into<String>, func: BuiltinFn) -> Self {
        Self {
            name: name.into(),
            func: BuiltinKind::Fn(func),
        }
    }

    pub fn with_closure(
        name: impl Into<String>,
        func: impl Fn(&mut dyn BuiltinContext, &[Object]) -> Result<Object, String> + 'static,
    ) -> Self {
        Self {
            name: name.into(),
            func: BuiltinKind::Closure(Box::new(func)),
        }
    }

    pub fn call(&self, ctx: &mut dyn BuiltinContext, args: &[Object]) -> Result<Object, String> {
        match &self.func {
            BuiltinKind::Fn(f) => f(ctx, args),
            BuiltinKind::Closure(f) => f(ctx, args),
        }
    }
}

impl fmt::Debug for Builtin {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "Builtin {{ name: {} }}", self.name)
    }
}

/// Map data structure with ordered keys.
#[derive(Debug, Default)]
pub struct RisorMap {
    data: HashMap<String, Object>,
    keys: HashMap<String, Object>,
}

impl RisorMap {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn get(&self, key: &Object) -> Option<Object> {
        let hash_key = key.hash_key()?;
        self.data.get(&hash_key).cloned()
    }

    pub fn set(&mut self, key: Object, value: Object) {
        if let Some(hash_key) = key.hash_key() {
            self.data.insert(hash_key.clone(), value);
            self.keys.insert(hash_key, key);
        }
    }

    pub fn has(&self, key: &Object) -> bool {
        key.hash_key()
            .map(|h| self.data.contains_key(&h))
            .unwrap_or(false)
    }

    pub fn delete(&mut self, key: &Object) -> bool {
        if let Some(hash_key) = key.hash_key() {
            self.keys.remove(&hash_key);
            self.data.remove(&hash_key).is_some()
        } else {
            false
        }
    }

    pub fn len(&self) -> usize {
        self.data.len()
    }

    pub fn is_empty(&self) -> bool {
        self.data.is_empty()
    }

    pub fn keys(&self) -> Vec<Object> {
        self.keys.values().cloned().collect()
    }

    pub fn values(&self) -> Vec<Object> {
        self.data.values().cloned().collect()
    }

    pub fn entries(&self) -> Vec<(Object, Object)> {
        self.keys
            .iter()
            .filter_map(|(hash_key, key)| {
                self.data.get(hash_key).map(|v| (key.clone(), v.clone()))
            })
            .collect()
    }

    pub fn merge(&mut self, other: &RisorMap) {
        for (key, value) in other.entries() {
            self.set(key, value);
        }
    }
}

/// Iterator for iteration operations.
#[derive(Debug)]
pub struct RisorIter {
    items: Vec<Object>,
    index: usize,
}

impl RisorIter {
    pub fn new(items: Vec<Object>) -> Self {
        Self { items, index: 0 }
    }

    pub fn next(&mut self) -> Option<Object> {
        if self.index >= self.items.len() {
            return None;
        }
        let item = self.items[self.index].clone();
        self.index += 1;
        Some(item)
    }

    pub fn remaining(&self) -> usize {
        self.items.len() - self.index
    }

    pub fn to_list(&self) -> Vec<Object> {
        self.items[self.index..].to_vec()
    }
}

/// Convert from JavaScript/Rust value to Risor object.
pub fn from_i64(n: i64) -> Object {
    Object::Int(n)
}

pub fn from_f64(n: f64) -> Object {
    Object::Float(n)
}

pub fn from_bool(b: bool) -> Object {
    Object::Bool(b)
}

pub fn from_string(s: impl Into<Rc<str>>) -> Object {
    Object::String(s.into())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_object_equality() {
        assert!(Object::Nil.equals(&Object::Nil));
        assert!(Object::Bool(true).equals(&Object::Bool(true)));
        assert!(!Object::Bool(true).equals(&Object::Bool(false)));
        assert!(Object::Int(42).equals(&Object::Int(42)));
        assert!(Object::Int(42).equals(&Object::Float(42.0)));
        assert!(Object::String("hello".into()).equals(&Object::String("hello".into())));
    }

    #[test]
    fn test_object_truthiness() {
        assert!(!Object::Nil.is_truthy());
        assert!(!Object::Bool(false).is_truthy());
        assert!(Object::Bool(true).is_truthy());
        assert!(!Object::Int(0).is_truthy());
        assert!(Object::Int(1).is_truthy());
        assert!(!Object::String("".into()).is_truthy());
        assert!(Object::String("hello".into()).is_truthy());
    }

    #[test]
    fn test_map_operations() {
        let mut map = RisorMap::new();
        map.set(Object::String("key".into()), Object::Int(42));
        assert!(map.has(&Object::String("key".into())));
        assert!(map.get(&Object::String("key".into())).unwrap().equals(&Object::Int(42)));
    }
}
