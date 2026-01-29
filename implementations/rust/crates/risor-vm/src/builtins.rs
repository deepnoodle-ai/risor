//! Built-in functions for Risor.

use std::cell::RefCell;
use std::collections::HashMap;
use std::rc::Rc;

use crate::object::{Builtin, Object, RisorIter};

/// Create the standard builtins map.
pub fn create_builtins() -> HashMap<String, Object> {
    let mut builtins = HashMap::new();

    // print - output values to console (with newline)
    builtins.insert(
        "print".to_string(),
        Object::Builtin(Rc::new(Builtin::new("print", |_, args| {
            let parts: Vec<String> = args
                .iter()
                .map(|arg| match arg {
                    Object::String(s) => s.to_string(),
                    _ => format!("{}", arg),
                })
                .collect();
            println!("{}", parts.join(" "));
            Ok(Object::Nil)
        }))),
    );

    // len - get length of string, list, or map
    builtins.insert(
        "len".to_string(),
        Object::Builtin(Rc::new(Builtin::new("len", |_, args| {
            if args.len() != 1 {
                return Err("len() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::String(s) => Ok(Object::Int(s.len() as i64)),
                Object::List(items) => Ok(Object::Int(items.borrow().len() as i64)),
                Object::Map(map) => Ok(Object::Int(map.borrow().len() as i64)),
                other => Err(format!("len() not supported for {}", other.type_name())),
            }
        }))),
    );

    // type - get type name of value
    builtins.insert(
        "type".to_string(),
        Object::Builtin(Rc::new(Builtin::new("type", |_, args| {
            if args.len() != 1 {
                return Err("type() takes exactly 1 argument".to_string());
            }
            Ok(Object::String(args[0].type_name().into()))
        }))),
    );

    // string - convert to string
    builtins.insert(
        "string".to_string(),
        Object::Builtin(Rc::new(Builtin::new("string", |_, args| {
            if args.len() != 1 {
                return Err("string() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::String(_) => Ok(args[0].clone()),
                other => Ok(Object::String(format!("{}", other).into())),
            }
        }))),
    );

    // int - convert to integer
    builtins.insert(
        "int".to_string(),
        Object::Builtin(Rc::new(Builtin::new("int", |_, args| {
            if args.len() != 1 {
                return Err("int() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Int(_) => Ok(args[0].clone()),
                Object::Float(f) => Ok(Object::Int(*f as i64)),
                Object::Bool(b) => Ok(Object::Int(if *b { 1 } else { 0 })),
                Object::String(s) => s
                    .parse::<i64>()
                    .map(Object::Int)
                    .map_err(|_| format!("cannot convert \"{}\" to int", s)),
                other => Err(format!("cannot convert {} to int", other.type_name())),
            }
        }))),
    );

    // float - convert to float
    builtins.insert(
        "float".to_string(),
        Object::Builtin(Rc::new(Builtin::new("float", |_, args| {
            if args.len() != 1 {
                return Err("float() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Float(_) => Ok(args[0].clone()),
                Object::Int(n) => Ok(Object::Float(*n as f64)),
                Object::String(s) => s
                    .parse::<f64>()
                    .map(Object::Float)
                    .map_err(|_| format!("cannot convert \"{}\" to float", s)),
                other => Err(format!("cannot convert {} to float", other.type_name())),
            }
        }))),
    );

    // bool - convert to boolean
    builtins.insert(
        "bool".to_string(),
        Object::Builtin(Rc::new(Builtin::new("bool", |_, args| {
            if args.len() != 1 {
                return Err("bool() takes exactly 1 argument".to_string());
            }
            Ok(Object::Bool(args[0].is_truthy()))
        }))),
    );

    // list - create or convert to list
    builtins.insert(
        "list".to_string(),
        Object::Builtin(Rc::new(Builtin::new("list", |_, args| {
            if args.is_empty() {
                return Ok(Object::List(Rc::new(RefCell::new(Vec::new()))));
            }
            match &args[0] {
                Object::List(items) => {
                    Ok(Object::List(Rc::new(RefCell::new(items.borrow().clone()))))
                }
                Object::String(s) => {
                    let chars: Vec<Object> = s
                        .chars()
                        .map(|c| Object::String(c.to_string().into()))
                        .collect();
                    Ok(Object::List(Rc::new(RefCell::new(chars))))
                }
                Object::Iter(iter) => Ok(Object::List(Rc::new(RefCell::new(iter.borrow().to_list())))),
                Object::Map(map) => Ok(Object::List(Rc::new(RefCell::new(map.borrow().keys())))),
                other => Err(format!("cannot convert {} to list", other.type_name())),
            }
        }))),
    );

    // iter - create iterator
    builtins.insert(
        "iter".to_string(),
        Object::Builtin(Rc::new(Builtin::new("iter", |_, args| {
            if args.is_empty() {
                return Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(
                    Vec::new(),
                )))));
            }
            match &args[0] {
                Object::List(items) => Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(
                    items.borrow().clone(),
                ))))),
                Object::String(s) => {
                    let chars: Vec<Object> = s
                        .chars()
                        .map(|c| Object::String(c.to_string().into()))
                        .collect();
                    Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(chars)))))
                }
                Object::Map(map) => Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(
                    map.borrow().keys(),
                ))))),
                Object::Iter(_) => Ok(args[0].clone()),
                other => Err(format!(
                    "cannot create iterator from {}",
                    other.type_name()
                )),
            }
        }))),
    );

    // range - create range iterator
    builtins.insert(
        "range".to_string(),
        Object::Builtin(Rc::new(Builtin::new("range", |_, args| {
            let (start, end, step) = match args.len() {
                1 => {
                    let end = match &args[0] {
                        Object::Int(n) => *n,
                        _ => return Err("range() arguments must be integers".to_string()),
                    };
                    (0, end, 1)
                }
                2 => {
                    let start = match &args[0] {
                        Object::Int(n) => *n,
                        _ => return Err("range() arguments must be integers".to_string()),
                    };
                    let end = match &args[1] {
                        Object::Int(n) => *n,
                        _ => return Err("range() arguments must be integers".to_string()),
                    };
                    (start, end, 1)
                }
                3 => {
                    let start = match &args[0] {
                        Object::Int(n) => *n,
                        _ => return Err("range() arguments must be integers".to_string()),
                    };
                    let end = match &args[1] {
                        Object::Int(n) => *n,
                        _ => return Err("range() arguments must be integers".to_string()),
                    };
                    let step = match &args[2] {
                        Object::Int(n) => *n,
                        _ => return Err("range() arguments must be integers".to_string()),
                    };
                    (start, end, step)
                }
                _ => return Err("range() takes 1 to 3 arguments".to_string()),
            };

            if step == 0 {
                return Err("range() step cannot be zero".to_string());
            }

            let mut items = Vec::new();
            if step > 0 {
                let mut i = start;
                while i < end {
                    items.push(Object::Int(i));
                    i += step;
                }
            } else {
                let mut i = start;
                while i > end {
                    items.push(Object::Int(i));
                    i += step;
                }
            }
            Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(items)))))
        }))),
    );

    // error - create error
    builtins.insert(
        "error".to_string(),
        Object::Builtin(Rc::new(Builtin::new("error", |_, args| {
            if args.len() != 1 {
                return Err("error() takes exactly 1 argument".to_string());
            }
            let msg = match &args[0] {
                Object::String(s) => s.to_string(),
                other => format!("{}", other),
            };
            Ok(Object::Error(msg.into()))
        }))),
    );

    // assert - assertion for testing
    builtins.insert(
        "assert".to_string(),
        Object::Builtin(Rc::new(Builtin::new("assert", |_, args| {
            if args.is_empty() {
                return Err("assert() requires at least 1 argument".to_string());
            }
            if !args[0].is_truthy() {
                let msg = if args.len() > 1 {
                    match &args[1] {
                        Object::String(s) => s.to_string(),
                        other => format!("{}", other),
                    }
                } else {
                    "assertion failed".to_string()
                };
                return Err(msg);
            }
            Ok(Object::Nil)
        }))),
    );

    // keys - get map keys
    builtins.insert(
        "keys".to_string(),
        Object::Builtin(Rc::new(Builtin::new("keys", |_, args| {
            if args.len() != 1 {
                return Err("keys() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Map(map) => Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(
                    map.borrow().keys(),
                ))))),
                other => Err(format!("keys() requires map, got {}", other.type_name())),
            }
        }))),
    );

    // values - get map values
    builtins.insert(
        "values".to_string(),
        Object::Builtin(Rc::new(Builtin::new("values", |_, args| {
            if args.len() != 1 {
                return Err("values() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Map(map) => Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(
                    map.borrow().values(),
                ))))),
                other => Err(format!("values() requires map, got {}", other.type_name())),
            }
        }))),
    );

    // sorted - return sorted list
    builtins.insert(
        "sorted".to_string(),
        Object::Builtin(Rc::new(Builtin::new("sorted", |_, args| {
            if args.len() != 1 {
                return Err("sorted() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::List(items) => {
                    let mut sorted = items.borrow().clone();
                    sorted.sort_by(|a, b| a.compare(b).unwrap_or(std::cmp::Ordering::Equal));
                    Ok(Object::List(Rc::new(RefCell::new(sorted))))
                }
                other => Err(format!("sorted() requires list, got {}", other.type_name())),
            }
        }))),
    );

    // reversed - return reversed list or string
    builtins.insert(
        "reversed".to_string(),
        Object::Builtin(Rc::new(Builtin::new("reversed", |_, args| {
            if args.len() != 1 {
                return Err("reversed() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::List(items) => {
                    let mut reversed = items.borrow().clone();
                    reversed.reverse();
                    Ok(Object::List(Rc::new(RefCell::new(reversed))))
                }
                Object::String(s) => {
                    let reversed: String = s.chars().rev().collect();
                    Ok(Object::String(reversed.into()))
                }
                other => Err(format!(
                    "reversed() requires list or string, got {}",
                    other.type_name()
                )),
            }
        }))),
    );

    // min - minimum value
    builtins.insert(
        "min".to_string(),
        Object::Builtin(Rc::new(Builtin::new("min", |_, args| {
            if args.is_empty() {
                return Err("min() requires at least 1 argument".to_string());
            }
            let items: Vec<Object> = if args.len() == 1 {
                match &args[0] {
                    Object::List(list) => list.borrow().clone(),
                    _ => args.to_vec(),
                }
            } else {
                args.to_vec()
            };

            if items.is_empty() {
                return Err("min() of empty sequence".to_string());
            }

            let mut min = &items[0];
            for item in items.iter().skip(1) {
                if item.compare(min) == Some(std::cmp::Ordering::Less) {
                    min = item;
                }
            }
            Ok(min.clone())
        }))),
    );

    // max - maximum value
    builtins.insert(
        "max".to_string(),
        Object::Builtin(Rc::new(Builtin::new("max", |_, args| {
            if args.is_empty() {
                return Err("max() requires at least 1 argument".to_string());
            }
            let items: Vec<Object> = if args.len() == 1 {
                match &args[0] {
                    Object::List(list) => list.borrow().clone(),
                    _ => args.to_vec(),
                }
            } else {
                args.to_vec()
            };

            if items.is_empty() {
                return Err("max() of empty sequence".to_string());
            }

            let mut max = &items[0];
            for item in items.iter().skip(1) {
                if item.compare(max) == Some(std::cmp::Ordering::Greater) {
                    max = item;
                }
            }
            Ok(max.clone())
        }))),
    );

    // sum - sum of numbers
    builtins.insert(
        "sum".to_string(),
        Object::Builtin(Rc::new(Builtin::new("sum", |_, args| {
            if args.is_empty() {
                return Err("sum() requires at least 1 argument".to_string());
            }
            let items: &[Object] = if args.len() == 1 {
                match &args[0] {
                    Object::List(list) => return sum_list(&list.borrow()),
                    _ => args,
                }
            } else {
                args
            };

            sum_list(items)
        }))),
    );

    // abs - absolute value
    builtins.insert(
        "abs".to_string(),
        Object::Builtin(Rc::new(Builtin::new("abs", |_, args| {
            if args.len() != 1 {
                return Err("abs() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Int(n) => Ok(Object::Int(n.abs())),
                Object::Float(n) => Ok(Object::Float(n.abs())),
                other => Err(format!("abs() requires number, got {}", other.type_name())),
            }
        }))),
    );

    // round - round to nearest integer
    builtins.insert(
        "round".to_string(),
        Object::Builtin(Rc::new(Builtin::new("round", |_, args| {
            if args.len() != 1 {
                return Err("round() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Int(_) => Ok(args[0].clone()),
                Object::Float(n) => Ok(Object::Int(n.round() as i64)),
                other => Err(format!("round() requires number, got {}", other.type_name())),
            }
        }))),
    );

    // floor - floor value
    builtins.insert(
        "floor".to_string(),
        Object::Builtin(Rc::new(Builtin::new("floor", |_, args| {
            if args.len() != 1 {
                return Err("floor() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Int(_) => Ok(args[0].clone()),
                Object::Float(n) => Ok(Object::Int(n.floor() as i64)),
                other => Err(format!("floor() requires number, got {}", other.type_name())),
            }
        }))),
    );

    // ceil - ceiling value
    builtins.insert(
        "ceil".to_string(),
        Object::Builtin(Rc::new(Builtin::new("ceil", |_, args| {
            if args.len() != 1 {
                return Err("ceil() takes exactly 1 argument".to_string());
            }
            match &args[0] {
                Object::Int(_) => Ok(args[0].clone()),
                Object::Float(n) => Ok(Object::Int(n.ceil() as i64)),
                other => Err(format!("ceil() requires number, got {}", other.type_name())),
            }
        }))),
    );

    builtins
}

fn sum_list(items: &[Object]) -> Result<Object, String> {
    let mut total = 0.0;
    let mut is_float = false;

    for item in items {
        match item {
            Object::Int(n) => total += *n as f64,
            Object::Float(n) => {
                total += n;
                is_float = true;
            }
            other => return Err(format!("cannot sum {}", other.type_name())),
        }
    }

    if is_float {
        Ok(Object::Float(total))
    } else {
        Ok(Object::Int(total as i64))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_builtins_exist() {
        let builtins = create_builtins();
        assert!(builtins.contains_key("print"));
        assert!(builtins.contains_key("len"));
        assert!(builtins.contains_key("type"));
        assert!(builtins.contains_key("range"));
        assert!(builtins.contains_key("assert"));
    }
}
