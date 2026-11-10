//! The 53 library entries (`04-library.md` 5).
//!
//! Every call goes through the same steps (4.4): the argument tests of 2.6,
//! taken from the entry's signature in `suite/library.json` (the receiver
//! first, then each argument left to right, a kind before a whole number:
//! `04-library.md` 2.2); the call's step; then the entry's own order. Every failure an
//! entry raises has the span of the whole call (2.6 item 6). Each entry does
//! work of at most a constant times what it charges: the searches are the
//! platform's linear ones (`str::find`, `str::split`, which are two-way
//! searches), the sort is a stable merge sort, and joins and repeats build
//! their result once.

use super::{fail, kind_p, value_p, Arg, Fail, Run, R};
use crate::casemap;
use crate::library::{records, Record};
use crate::messages::json_string;
use crate::number::{self, from_text, NumErr};
use crate::source::Span;
use crate::types::text::{Bound, ParamType, Type};
use crate::value::{is_whole, log2_ceil, release, size, weight, Kind, MapVal, Str, Value};
use std::cmp::Ordering;
use std::collections::{HashMap, HashSet};
use std::rc::Rc;

/// The strings of a parameter type that is a literal set and nothing else
/// (F7 6.2).
fn literal_set(t: &Type) -> Option<Vec<&str>> {
    match t {
        Type::Lit(s) => Some(vec![s.as_str()]),
        Type::Union(alts) => alts
            .iter()
            .map(|a| match a {
                Type::Lit(s) => Some(s.as_str()),
                _ => None,
            })
            .collect(),
        _ => None,
    }
}

/// The formulas each entry implements: name, receiver, steps, units. A unit
/// test holds them equal to `suite/library.json`.
pub const FORMULAS: [(&str, Option<Kind>, &str, &str); 78] = [
    ("math.pow", None, "200", "0"),
    ("math.exp", None, "200", "0"),
    ("math.log", None, "200", "0"),
    ("math.log10", None, "200", "0"),
    ("math.sin", None, "200", "0"),
    ("math.cos", None, "200", "0"),
    ("math.tan", None, "200", "0"),
    ("math.asin", None, "200", "0"),
    ("math.acos", None, "200", "0"),
    ("math.atan", None, "200", "0"),
    ("math.atan2", None, "200", "0"),
    ("minBy", Some(Kind::List), "2*n + sz(keys)", "0"),
    ("maxBy", Some(Kind::List), "2*n + sz(keys)", "0"),
    ("uniqueBy", Some(Kind::List), "2*n + sz(keys)", "1 + k"),
    ("indexBy", Some(Kind::List), "2*n + sz(keys)", "1 + 2*n"),
    ("countBy", Some(Kind::List), "2*n + sz(keys)", "1 + 2*g"),
    (
        "mapValues",
        Some(Kind::Map),
        "2*n + L*(n + sz(keys))",
        "1 + 2*n",
    ),
    (
        "json.stringify",
        None,
        "walk(x) + sz(result)",
        "1 + sz(result)",
    ),
    ("json.parse", None, "8*sz(s) + c", "u"),
    ("time.parse", None, "sz(s)", "0"),
    ("time.format", None, "sz(result)", "1 + sz(result)"),
    ("time.duration", None, "sz(s)", "0"),
    ("math.sqrt", None, "0", "0"),
    (
        "base64.encode",
        None,
        "sz(s) + sz(result)",
        "1 + sz(result)",
    ),
    ("base64.decode", None, "sz(s)", "1 + sz(result)"),
    ("len", None, "sz(x)", "0"),
    (
        "string",
        None,
        "x is string ? 0 : walk(x) + sz(result)",
        "x is string ? 0 : 1 + sz(result)",
    ),
    ("int", None, "sz(x)", "0"),
    ("float", None, "sz(x)", "0"),
    ("bool", None, "0", "0"),
    ("type", None, "0", "0"),
    ("fail", None, "0", "0"),
    ("abs", None, "0", "0"),
    ("floor", None, "0", "0"),
    ("ceil", None, "0", "0"),
    ("round", None, "0", "0"),
    ("min", None, "len(arguments) - 1 + sz(arguments)", "0"),
    ("max", None, "len(arguments) - 1 + sz(arguments)", "0"),
    ("range", None, "k", "1 + k"),
    ("fixed", None, "0", "1"),
    ("toUpperCase", Some(Kind::String), "sz(s)", "1 + sz(s)"),
    ("toLowerCase", Some(Kind::String), "sz(s)", "1 + sz(s)"),
    ("trim", Some(Kind::String), "sz(s)", "1 + sz(result)"),
    (
        "removePrefix",
        Some(Kind::String),
        "sz(s)",
        "1 + sz(result)",
    ),
    (
        "removeSuffix",
        Some(Kind::String),
        "sz(s)",
        "1 + sz(result)",
    ),
    ("startsWith", Some(Kind::String), "sz(prefix)", "0"),
    ("endsWith", Some(Kind::String), "sz(suffix)", "0"),
    ("split", Some(Kind::String), "sz(s) + p", "1 + 2*p + sz(s)"),
    (
        "replaceAll",
        Some(Kind::String),
        "sz(s) + sz(result)",
        "1 + sz(result)",
    ),
    ("slice", Some(Kind::String), "sz(s)", "1 + sz(result)"),
    ("repeat", Some(Kind::String), "sz(result)", "1 + sz(result)"),
    (
        "padStart",
        Some(Kind::String),
        "sz(result)",
        "1 + sz(result)",
    ),
    ("padEnd", Some(Kind::String), "sz(result)", "1 + sz(result)"),
    ("filter", Some(Kind::List), "2*n", "1 + k"),
    ("map", Some(Kind::List), "2*n", "1 + n"),
    ("flatMap", Some(Kind::List), "2*n + k", "1 + k"),
    ("some", Some(Kind::List), "2*v", "0"),
    ("every", Some(Kind::List), "2*v", "0"),
    ("find", Some(Kind::List), "2*v", "0"),
    ("count", Some(Kind::List), "2*n", "0"),
    ("reduce", Some(Kind::List), "2*n", "0"),
    ("sum", Some(Kind::List), "n", "0"),
    ("min", Some(Kind::List), "n + sz(elements)", "0"),
    ("max", Some(Kind::List), "n + sz(elements)", "0"),
    (
        "sort",
        Some(Kind::List),
        "n + L*(n + sz(elements))",
        "1 + n",
    ),
    (
        "sortBy",
        Some(Kind::List),
        "2*n + L*(n + sz(keys))",
        "1 + n",
    ),
    ("groupBy", Some(Kind::List), "2*n + sz(keys)", "1 + 3*g + n"),
    ("toMap", Some(Kind::List), "n + sz(keys)", "1 + 2*n"),
    ("unique", Some(Kind::List), "n + sz(elements)", "1 + k"),
    ("flat", Some(Kind::List), "n + k", "1 + k"),
    ("reverse", Some(Kind::List), "n", "1 + n"),
    ("first", Some(Kind::List), "0", "0"),
    ("last", Some(Kind::List), "0", "0"),
    ("slice", Some(Kind::List), "k", "1 + k"),
    ("join", Some(Kind::List), "n + sz(result)", "1 + sz(result)"),
    ("keys", Some(Kind::Map), "n + L*(n + sz(keys))", "1 + n"),
    ("values", Some(Kind::Map), "n + L*(n + sz(keys))", "1 + n"),
    (
        "entries",
        Some(Kind::Map),
        "n + L*(n + sz(keys))",
        "1 + 6*n",
    ),
];

/// White space (3.9): exactly 25 code points.
pub fn is_space(c: char) -> bool {
    matches!(c,
        '\u{9}'..='\u{d}' | ' ' | '\u{a0}' | '\u{1680}' | '\u{2000}'..='\u{200a}'
        | '\u{2028}' | '\u{2029}' | '\u{202f}' | '\u{205f}' | '\u{3000}' | '\u{feff}')
}

/// The text of an optional string argument, read where it is.
fn text<'a>(v: &'a Option<Value<'_>>) -> &'a str {
    v.as_ref().and_then(|v| v.as_str()).unwrap_or_default()
}

/// The weight of several strings taken together, divided by 64.
fn sz_all<'a>(ss: impl Iterator<Item = &'a str>) -> u64 {
    ss.map(weight).sum::<u64>() / 64
}

/// Whether a value fits a parameter type by kind (2.6 item 3), and whether
/// the fit asks for a whole number.
fn fits(t: &Type, v: &Value<'_>, bounds: &[(char, Bound)]) -> (bool, bool) {
    let k = v.kind();
    match t {
        Type::Any => (true, false),
        Type::Null => (k == Kind::Null, false),
        Type::Bool => (k == Kind::Bool, false),
        Type::Int => (k == Kind::Number, true),
        Type::Float => (k == Kind::Number, false),
        Type::String | Type::Lit(_) => (k == Kind::String, false),
        Type::List(_) => (k == Kind::List, false),
        Type::MapOf(_) | Type::Record(_) => (k == Kind::Map, false),
        Type::Name(_) => (k == Kind::Opaque, false),
        Type::Never => (false, false),
        Type::Var(c) => match bounds.iter().find(|(b, _)| b == c).map(|(_, b)| *b) {
            Some(Bound::Number) => (k == Kind::Number, false),
            Some(Bound::Ordered) => (matches!(k, Kind::Number | Kind::String), false),
            None => (true, false),
        },
        Type::Union(alts) => {
            let mut whole = true;
            let mut ok = false;
            for a in alts {
                let (f, w) = fits(a, v, bounds);
                if f {
                    ok = true;
                    whole &= w;
                }
            }
            (ok, ok && whole)
        }
    }
}

/// The test of one argument (2.6 item 3), read once from its parameter's
/// type.
enum ArgTest {
    /// A callback, `any`, or a type variable with no bound.
    None,
    /// The kinds the parameter takes, as bits by `Kind`, and those of them
    /// for which it asks a whole number.
    Kinds { allowed: u8, whole: u8 },
    /// A parameter bound `ordered`.
    Ordered,
    /// A literal set (F7 6.2).
    Literals(Box<[Box<str>]>),
}

/// The tests of a record's arguments, by parameter.
struct Plan {
    params: Vec<ArgTest>,
    /// The last parameter is a rest parameter: it tests every later
    /// argument.
    rest: bool,
}

fn plans() -> &'static [Plan] {
    static P: std::sync::OnceLock<Vec<Plan>> = std::sync::OnceLock::new();
    P.get_or_init(|| records().iter().map(plan).collect())
}

/// The plan of a record, from its signature, as `fits` reads a type.
fn plan(rec: &Record) -> Plan {
    let sig = &rec.sig;
    let kinds = [
        Kind::Null,
        Kind::Bool,
        Kind::Number,
        Kind::String,
        Kind::List,
        Kind::Map,
        Kind::Opaque,
    ];
    let params = sig
        .params
        .iter()
        .map(|p| {
            let ParamType::Type(t) = &p.ty else {
                return ArgTest::None;
            };
            let t = match (t, p.rest) {
                (Type::List(e), true) => e.as_ref(),
                (t, _) => t,
            };
            if let Type::Var(c) = t {
                if sig.bounds.contains(&(*c, Bound::Ordered)) {
                    return ArgTest::Ordered;
                }
            }
            if let Some(set) = literal_set(t) {
                return ArgTest::Literals(set.into_iter().map(Box::from).collect());
            }
            let (mut allowed, mut whole) = (0u8, 0u8);
            for k in kinds {
                let probe = match k {
                    Kind::Null => Value::Null,
                    Kind::Bool => Value::Bool(false),
                    Kind::Number => Value::Num(0.5),
                    Kind::String => Value::Str(Str::Lent("")),
                    Kind::List => Value::list(Vec::new()),
                    Kind::Map => Value::Map(Rc::new(MapVal(Vec::new()))),
                    Kind::Opaque => Value::Opaque(Rc::new(crate::host::OpaqueRef::new("", ()))),
                };
                let (ok, w) = fits(t, &probe, &sig.bounds);
                if ok {
                    allowed |= 1 << k as u8;
                    if w {
                        whole |= 1 << k as u8;
                    }
                }
            }
            if allowed == 0x7f && whole == 0 {
                ArgTest::None
            } else {
                ArgTest::Kinds { allowed, whole }
            }
        })
        .collect();
    Plan {
        params,
        rest: sig.params.last().is_some_and(|p| p.rest),
    }
}

/// The distinct strings of a sequence, each numbered in order of first
/// appearance. Few distinct keys are the common case, and are found by a
/// scan with no hashing; past [`Distinct::SCAN`] keys, a hash map.
struct Distinct<'a> {
    keys: Vec<&'a str>,
    map: Option<HashMap<&'a str, usize>>,
}

impl<'a> Distinct<'a> {
    const SCAN: usize = 16;

    fn new() -> Distinct<'a> {
        Distinct {
            keys: Vec::new(),
            map: None,
        }
    }

    /// The number of `k`, and whether it is new.
    #[inline]
    fn index(&mut self, k: &'a str) -> (usize, bool) {
        if let Some(m) = &mut self.map {
            let n = m.len();
            return match m.entry(k) {
                std::collections::hash_map::Entry::Occupied(e) => (*e.get(), false),
                std::collections::hash_map::Entry::Vacant(e) => {
                    e.insert(n);
                    self.keys.push(k);
                    (n, true)
                }
            };
        }
        if let Some(i) = self.keys.iter().position(|x| *x == k) {
            return (i, false);
        }
        let n = self.keys.len();
        self.keys.push(k);
        if self.keys.len() > Self::SCAN {
            self.map = Some(self.keys.iter().enumerate().map(|(i, k)| (*k, i)).collect());
        }
        (n, true)
    }
}

/// The ordered test of 3.6 over values in order: the index and kind of the
/// first value that is neither a number nor a string, or not of the first
/// value's kind.
fn ordered<'a, 'h: 'a>(vals: impl Iterator<Item = &'a Value<'h>>) -> Result<(), (usize, Kind)> {
    let mut first = None;
    for (i, v) in vals.enumerate() {
        let k = v.kind();
        if !matches!(k, Kind::Number | Kind::String) || first.is_some_and(|f| f != k) {
            return Err((i, k));
        }
        first.get_or_insert(k);
    }
    Ok(())
}

/// The order of two values of one ordered kind (3.6).
fn cmp(a: &Value<'_>, b: &Value<'_>) -> Ordering {
    match (a, b) {
        (Value::Num(x), Value::Num(y)) => x.partial_cmp(y).unwrap_or(Ordering::Equal),
        (Value::Str(x), Value::Str(y)) => x.as_str().cmp(y.as_str()),
        _ => Ordering::Equal,
    }
}

/// The resolved start and end of a slice against a length (3.10 item 4).
fn positions(start: f64, end: Option<f64>, len: usize) -> (usize, usize) {
    let len_f = len as f64;
    let resolve = |p: f64| {
        let p = if p < 0.0 { p + len_f } else { p };
        p.clamp(0.0, len_f) as usize
    };
    let s = resolve(start);
    let e = end.map_or(len, resolve);
    (s, e.max(s))
}

/// The text of `x` rounded to `digits` places, half away from zero, from the
/// exact value of the double (5.2.8). `|x|` is below 1e21.
fn fixed_text(x: f64, digits: u32) -> String {
    let bits = x.abs().to_bits();
    let exp = ((bits >> 52) & 0x7ff) as i32;
    let frac = bits & ((1u64 << 52) - 1);
    let (m, e) = if exp == 0 {
        (frac as u128, -1074)
    } else {
        ((frac | (1 << 52)) as u128, exp - 1075)
    };
    // The digits of the rounded value times 10^digits, as text.
    let q = if e >= 0 {
        let mut t = (m << e).to_string();
        if m == 0 {
            t = "0".into();
        }
        t.push_str(&"0".repeat(digits as usize));
        t
    } else {
        let k = (-e) as u32;
        let n = m * 10u128.pow(digits);
        let q = if k >= 127 {
            0
        } else {
            let q = n >> k;
            let rem = n & ((1u128 << k) - 1);
            if rem >= 1u128 << (k - 1) {
                q + 1
            } else {
                q
            }
        };
        q.to_string()
    };
    let d = digits as usize;
    let q = format!("{q:0>width$}", width = d + 1);
    let zero = q.bytes().all(|b| b == b'0');
    let (int, fr) = q.split_at(q.len() - d);
    let mut out = String::new();
    if x < 0.0 && !zero {
        out.push('-');
    }
    out.push_str(int);
    if d > 0 {
        out.push('.');
        out.push_str(fr);
    }
    out
}

impl<'h> Run<'h> {
    fn val(&self, args: &[Arg<'h>], i: usize) -> Option<Value<'h>> {
        match args.get(i) {
            Some(Arg::Val(v)) => Some(v.dup()),
            _ => None,
        }
    }

    fn lam(&self, args: &[Arg<'h>], i: usize, span: Span) -> R<'h, crate::syntax::ast::NodeId> {
        match args.get(i) {
            Some(Arg::Lam(l)) => Ok(*l),
            _ => Err(self.internal(span)),
        }
    }

    fn unordered(&self, name: &str, k: Kind, span: Span) -> Box<Fail<'h>> {
        fail(
            "wrong-kind",
            "unordered",
            span,
            vec![("name", name.to_string()), ("kind", k.name().to_string())],
        )
    }

    fn element(&self, name: &str, v: &Value<'h>, span: Span) -> Box<Fail<'h>> {
        fail(
            "wrong-kind",
            "element",
            span,
            vec![("name", name.to_string()), ("kind", kind_p(v))],
        )
    }

    pub(super) fn entry_range(&self, name: &str, span: Span) -> Box<Fail<'h>> {
        fail(
            "number-range",
            "entry",
            span,
            vec![("name", name.to_string())],
        )
    }

    /// A failed element scan charges 1 step for each element tested, the
    /// failing one included (4.5), then raises its failure.
    fn scan_failed(&mut self, tested: usize, f: Box<Fail<'h>>) -> R<'h, Value<'h>> {
        self.charge(tested as u64)?;
        Err(f)
    }

    /// The tests of 2.6 against the record's signature: kinds, and whole
    /// numbers for `int`, each argument left to right (`04-library.md` 2.2). For a
    /// parameter bound `ordered` every failure of kind is `unordered` (`04-library.md` 2.2).
    /// The tests are read from the record's plan ([`plan`]), built once.
    #[inline]
    fn arg_tests(&self, idx: usize, args: &[Arg<'h>], span: Span) -> R<'h, ()> {
        let plan = &plans()[idx];
        let mut ordered_first: Option<Kind> = None;
        for (i, a) in args.iter().enumerate() {
            let Arg::Val(v) = a else { continue };
            let test = match plan.params.get(i) {
                Some(t) => t,
                None if plan.rest => match plan.params.last() {
                    Some(t) => t,
                    None => continue,
                },
                None => continue,
            };
            match test {
                ArgTest::None => {}
                ArgTest::Kinds { allowed, whole } => {
                    let k = 1u8 << v.kind() as u8;
                    if allowed & k == 0 {
                        return Err(self.arg_kind(idx, v, span));
                    }
                    if whole & k != 0 {
                        if let Value::Num(n) = v {
                            if !is_whole(*n) {
                                return Err(fail(
                                    "wrong-kind",
                                    "whole",
                                    span,
                                    vec![("found", value_p(v))],
                                ));
                            }
                        }
                    }
                }
                ArgTest::Ordered => {
                    let k = v.kind();
                    if !matches!(k, Kind::Number | Kind::String)
                        || ordered_first.is_some_and(|f| f != k)
                    {
                        return Err(self.unordered(&records()[idx].name, k, span));
                    }
                    ordered_first.get_or_insert(k);
                }
                // F7 6.2: a parameter whose type is a literal set.
                ArgTest::Literals(set) => {
                    let ok = matches!(v, Value::Str(x) if set.iter().any(|s| &**s == x.as_str()));
                    if !ok {
                        return Err(fail(
                            "wrong-kind",
                            "order",
                            span,
                            vec![("name", records()[idx].name.clone()), ("found", value_p(v))],
                        ));
                    }
                }
            }
        }
        Ok(())
    }

    #[cold]
    #[inline(never)]
    fn arg_kind(&self, idx: usize, v: &Value<'h>, span: Span) -> Box<Fail<'h>> {
        fail(
            "wrong-kind",
            "argument",
            span,
            vec![("name", records()[idx].name.clone()), ("kind", kind_p(v))],
        )
    }

    pub(super) fn lib_function(
        &mut self,
        idx: usize,
        args: &[Arg<'h>],
        span: Span,
    ) -> R<'h, Value<'h>> {
        let rec = &records()[idx];
        let name = rec.name.as_str();
        self.arg_tests(idx, args, span)?;
        self.charge(1)?;
        let x = self.val(args, 0).unwrap_or(Value::Null);
        let num = |v: &Value<'h>| match v {
            Value::Num(n) => *n,
            _ => 0.0,
        };
        match name {
            "len" => {
                let n = match &x {
                    Value::Str(s) => {
                        self.charge(size(s.as_str()))?;
                        s.as_str().chars().count()
                    }
                    _ => x.len().unwrap_or(0),
                };
                Ok(Value::Num(n as f64))
            }
            "string" => {
                if let Value::Str(_) = x {
                    return Ok(x);
                }
                let mut rbuf = super::RenderBuf::new(String::new());
                self.render_walk(&x, &mut rbuf, true, span)?;
                let sz = rbuf.weight() / 64;
                self.charge(sz)?;
                self.alloc(1 + sz)?;
                if !rbuf.counting {
                    return Ok(Value::str(rbuf.buf));
                }
                let mut out = String::with_capacity(rbuf.bytes);
                self.write_rendered(&x, &mut out, true, span)?;
                Ok(Value::str(out))
            }
            "int" | "float" => {
                let n = match &x {
                    Value::Str(s) => {
                        self.charge(size(s.as_str()))?;
                        let text = s.as_str().trim_matches(is_space);
                        let p = |form: &'static str| {
                            fail(
                                "invalid-argument",
                                form,
                                span,
                                vec![("text", json_string(s.as_str()))],
                            )
                        };
                        match from_text(text) {
                            None => return Err(p("not-a-number")),
                            Some(Err(())) => return Err(p("not-finite")),
                            Some(Ok(n)) => n,
                        }
                    }
                    v => num(v),
                };
                if name == "float" {
                    return Ok(Value::Num(n));
                }
                let t = n.trunc();
                if !is_whole(t) {
                    return Err(self.entry_range(name, span));
                }
                Ok(Value::Num(if t == 0.0 { 0.0 } else { t }))
            }
            "bool" => Ok(Value::Bool(x.truthy())),
            "type" => Ok(Value::Str(Str::Lent(x.kind().name()))),
            "fail" => {
                let mut f = fail("failed", "default", span, vec![]);
                f.text = Some(x.as_str().unwrap_or_default().to_string());
                f.data = Some(self.val(args, 1).unwrap_or(Value::Null));
                Err(f)
            }
            "abs" => Ok(Value::Num(number::neg(-num(&x)).abs())),
            "floor" | "ceil" | "round" => {
                let n = num(&x);
                let r = match name {
                    "floor" => n.floor(),
                    "ceil" => n.ceil(),
                    // Half away from zero, of the exact value.
                    _ => n.round(),
                };
                if !is_whole(r) {
                    return Err(self.entry_range(name, span));
                }
                Ok(Value::Num(if r == 0.0 { 0.0 } else { r }))
            }
            "min" | "max" => {
                let vals: Vec<Value<'h>> =
                    (0..args.len()).filter_map(|i| self.val(args, i)).collect();
                let s = sz_all(vals.iter().filter_map(|v| v.as_str()));
                self.charge(vals.len() as u64 - 1 + s)?;
                let want = if name == "min" {
                    Ordering::Less
                } else {
                    Ordering::Greater
                };
                let mut best = vals[0].clone();
                for v in &vals[1..] {
                    if cmp(v, &best) == want {
                        best = v.clone();
                    }
                }
                Ok(best)
            }
            "range" => {
                let (a, b) = match self.val(args, 1) {
                    Some(b) => (num(&x), num(&b)),
                    None => (0.0, num(&x)),
                };
                let k = if b > a { (b - a) as u64 } else { 0 };
                self.charge(k)?;
                self.alloc(1 + k)?;
                Ok(Value::list(
                    (0..k).map(|i| Value::Num(a + i as f64)).collect(),
                ))
            }
            "fixed" => {
                let d = num(&self.val(args, 1).unwrap_or(Value::Null));
                if !(0.0..=20.0).contains(&d) {
                    return Err(fail(
                        "invalid-argument",
                        "bad-digits",
                        span,
                        vec![("digits", number::to_text(d))],
                    ));
                }
                let xv = num(&x);
                if xv.abs() >= 1e21 {
                    return Err(fail(
                        "invalid-argument",
                        "too-large",
                        span,
                        vec![("x", number::to_text(xv))],
                    ));
                }
                self.alloc(1)?;
                Ok(Value::str(fixed_text(xv, d as u32)))
            }
            // F13 3: the value test, the 200 steps, then the result.
            n if n.starts_with("math.") && n != "math.sqrt" => {
                let a: Vec<f64> = (0..args.len())
                    .map(|i| self.val(args, i).map_or(0.0, |v| num(&v)))
                    .collect();
                if crate::math::not_real(n, &a) {
                    return Err(fail(
                        "invalid-argument",
                        "not-real",
                        span,
                        vec![("name", n.to_string())],
                    ));
                }
                self.charge(200)?;
                let r = crate::math::eval(n, &a);
                if !r.is_finite() {
                    return Err(self.entry_range(n, span));
                }
                Ok(Value::Num(r))
            }
            // F6: an entry of an opt-in group.
            n if n.contains('.') => self.group_entry(n, x, span),
            _ => Err(self.internal(span)),
        }
    }

    pub(super) fn lib_method(
        &mut self,
        idx: usize,
        recv: Value<'h>,
        args: &[Arg<'h>],
        span: Span,
        measure: u32,
    ) -> R<'h, Value<'h>> {
        let rec = &records()[idx];
        self.arg_tests(idx, args, span)?;
        self.charge(1)?;
        match rec.receiver {
            Some(Kind::String) => self.string_method(rec, recv, args, span),
            Some(Kind::Map) if rec.name == "mapValues" => {
                self.map_values(recv, args, span, measure)
            }
            Some(Kind::Map) => self.map_method(rec, recv, span),
            _ if rec.queries => self.query_method(rec, recv, args, span, measure),
            _ => self.list_method(rec, recv, args, span, measure),
        }
    }

    fn string_method(
        &mut self,
        rec: &Record,
        recv: Value<'h>,
        args: &[Arg<'h>],
        span: Span,
    ) -> R<'h, Value<'h>> {
        // Nothing is copied before it is charged: the receiver and the
        // arguments are read where they are.
        let s = recv.as_str().unwrap_or_default();
        let sz_s = size(s);
        let a0 = self.val(args, 0);
        let a1 = self.val(args, 1);
        let num = |v: &Option<Value<'h>>| match v {
            Some(Value::Num(n)) => Some(*n),
            _ => None,
        };
        // Steps, units, then the result, for an entry that charges by
        // `sz(s)` and `1 + sz(result)`.
        let by_s = |run: &mut Self, result: &str| -> R<'h, Value<'h>> {
            run.charge(sz_s)?;
            run.alloc(1 + size(result))?;
            Ok(Value::str(result.to_string()))
        };
        match rec.name.as_str() {
            "toUpperCase" | "toLowerCase" => {
                self.charge(sz_s)?;
                self.alloc(1 + sz_s)?;
                Ok(Value::str(if rec.name == "toUpperCase" {
                    casemap::upper(s)
                } else {
                    casemap::lower(s)
                }))
            }
            "trim" => by_s(self, s.trim_matches(is_space)),
            "removePrefix" => by_s(self, s.strip_prefix(text(&a0)).unwrap_or(s)),
            "removeSuffix" => by_s(self, s.strip_suffix(text(&a0)).unwrap_or(s)),
            "startsWith" | "endsWith" => {
                let p = text(&a0);
                // The lengths answer first when the prefix is the longer.
                let r = p.len() <= s.len()
                    && if rec.name == "startsWith" {
                        s.starts_with(p)
                    } else {
                        s.ends_with(p)
                    };
                self.charge(size(p))?;
                Ok(Value::Bool(r))
            }
            "split" => {
                let sep = text(&a0);
                // A separator longer than the text is answered
                // from the lengths.
                let p = if sep.is_empty() {
                    s.chars().count()
                } else if sep.len() > s.len() {
                    1
                } else {
                    s.matches(sep).count() + 1
                };
                self.charge(sz_s + p as u64)?;
                self.alloc(1 + 2 * p as u64 + sz_s)?;
                let parts: Vec<Value<'h>> = if sep.is_empty() {
                    s.chars().map(|c| Value::str(c.to_string())).collect()
                } else if sep.len() > s.len() {
                    vec![Value::str(s.to_string())]
                } else {
                    s.split(sep).map(|t| Value::str(t.to_string())).collect()
                };
                Ok(Value::list(parts))
            }
            "replaceAll" => {
                let old = text(&a0);
                let new = text(&a1);
                if old.is_empty() {
                    return Err(fail("invalid-argument", "empty-search", span, vec![]));
                }
                let c = if old.len() > s.len() {
                    0
                } else {
                    s.matches(old).count()
                } as u128;
                // `new` is measured only when it is used.
                let wn = if c == 0 { 0 } else { weight(new) as u128 };
                let w = (weight(s) as u128 + c * wn).saturating_sub(c * weight(old) as u128);
                let sz_r = (w / 64).min(u64::MAX as u128) as u64;
                self.charge(sz_s.saturating_add(sz_r))?;
                self.alloc(sz_r.saturating_add(1))?;
                Ok(Value::str(if c == 0 {
                    s.to_string()
                } else {
                    s.replace(old, new)
                }))
            }
            "slice" => {
                let n = s.chars().count();
                let (a, b) = positions(num(&a0).unwrap_or(0.0), num(&a1), n);
                let r: String = s.chars().skip(a).take(b - a).collect();
                by_s(self, &r)
            }
            "repeat" => {
                let count = num(&a0).unwrap_or(0.0);
                if count < 0.0 {
                    return Err(fail(
                        "invalid-argument",
                        "negative-count",
                        span,
                        vec![("count", number::to_text(count))],
                    ));
                }
                let w = count as u128 * weight(s) as u128;
                let sz_r = (w / 64).min(u64::MAX as u128) as u64;
                self.charge(sz_r)?;
                self.alloc(sz_r.saturating_add(1))?;
                let r = if s.is_empty() {
                    String::new()
                } else {
                    s.repeat(count as usize)
                };
                Ok(Value::str(r))
            }
            "padStart" | "padEnd" => {
                let length = num(&a0).unwrap_or(0.0);
                let fill = match &a1 {
                    Some(_) => text(&a1),
                    None => " ",
                };
                let n = s.chars().count();
                let missing = if fill.is_empty() || length <= n as f64 {
                    0usize
                } else {
                    (length - n as f64) as usize
                };
                // Only as much of `fill` is read as the result uses: its first
                // `missing` code points. When `fill` is shorter, all of it is
                // used at least once, and the result is charged for it.
                let head: Vec<char> = fill.chars().take(missing).collect();
                let cw = |c: &char| if *c as u32 > 0xFFFF { 2u128 } else { 1 };
                let (copies, rest) = if head.len() == missing {
                    (0, missing)
                } else {
                    (missing / head.len(), missing % head.len())
                };
                let w = weight(s) as u128
                    + copies as u128 * head.iter().map(cw).sum::<u128>()
                    + head[..rest].iter().map(cw).sum::<u128>();
                let sz_r = (w / 64).min(u64::MAX as u128) as u64;
                self.charge(sz_r)?;
                self.alloc(sz_r.saturating_add(1))?;
                let unit: String = head.iter().collect();
                let mut pad = unit.repeat(copies);
                pad.extend(&head[..rest]);
                Ok(Value::str(if rec.name == "padStart" {
                    pad + s
                } else {
                    s.to_string() + &pad
                }))
            }
            _ => Err(self.internal(span)),
        }
    }

    /// F7 6.9: `mapValues`. The key order is charged first; then each
    /// entry, in key order: 2 steps, then `f(value, key)`.
    fn map_values(
        &mut self,
        recv: Value<'h>,
        args: &[Arg<'h>],
        span: Span,
        measure: u32,
    ) -> R<'h, Value<'h>> {
        let es = recv.entries();
        let n = es.len() as u64;
        let s = sz_all(es.iter().map(|(k, _)| k.as_str()));
        self.charge(log2_ceil(n) * (n + s))?;
        let f = self.lam(args, 0, span)?;
        let f = self.callback(f);
        let mut out = Vec::with_capacity(es.len());
        for (i, (k, v)) in es.into_iter().enumerate() {
            self.charge(2)?;
            let v = self.read(v, span)?;
            let r = self.call2(&f, "mapValues", i, v, Value::Str(k.clone()), measure, span)?;
            out.push((k, r));
        }
        self.alloc(1 + 2 * n)?;
        Ok(Value::Map(Rc::new(MapVal(out))))
    }

    /// F7 6.4 to 6.8: `minBy`, `maxBy`, `uniqueBy`, `indexBy`, `countBy`.
    fn query_method(
        &mut self,
        rec: &Record,
        recv: Value<'h>,
        args: &[Arg<'h>],
        span: Span,
        measure: u32,
    ) -> R<'h, Value<'h>> {
        let name = rec.name.as_str();
        let n = recv.len().unwrap_or(0);
        let f = self.lam(args, 0, span)?;
        let f = self.callback(f);
        let mut elems: Vec<Value<'h>> = Vec::with_capacity(n);
        let mut keys: Vec<Value<'h>> = Vec::with_capacity(n);
        let string_keys = matches!(name, "indexBy" | "countBy");
        for i in 0..n {
            self.charge(2)?;
            let e = self.read(recv.elem(i), span)?;
            let k = self.call2(&f, name, i, e.dup(), Value::Num(i as f64), measure, span)?;
            if string_keys && k.kind() != Kind::String {
                return Err(fail(
                    "wrong-kind",
                    "result",
                    span,
                    vec![("name", name.to_string()), ("kind", kind_p(&k))],
                ));
            }
            elems.push(e);
            keys.push(k);
        }
        if !string_keys {
            if let Err((_, k)) = ordered(keys.iter()) {
                return Err(self.unordered(name, k, span));
            }
        }
        self.charge(sz_all(keys.iter().filter_map(|k| k.as_str())))?;
        match name {
            "minBy" | "maxBy" => {
                let want = if name == "minBy" {
                    Ordering::Less
                } else {
                    Ordering::Greater
                };
                let mut best: Option<usize> = None;
                for i in 0..n {
                    match best {
                        None => best = Some(i),
                        Some(b) if cmp(&keys[i], &keys[b]) == want => best = Some(i),
                        _ => {}
                    }
                }
                Ok(best.map_or(Value::Null, |b| elems[b].clone()))
            }
            "uniqueBy" => {
                // Numbers are equal by value, strings by code point.
                let mut seen_n: HashSet<u64> = HashSet::new();
                let mut seen_s = Distinct::new();
                let mut out = Vec::new();
                for (e, k) in elems.into_iter().zip(keys.iter()) {
                    let new = match k {
                        Value::Num(x) => seen_n.insert(if *x == 0.0 { 0 } else { x.to_bits() }),
                        Value::Str(x) => seen_s.index(x.as_str()).1,
                        _ => seen_s.index("").1,
                    };
                    if new {
                        out.push(e);
                    }
                }
                self.alloc(1 + out.len() as u64)?;
                Ok(Value::list(out))
            }
            "indexBy" => {
                self.alloc(1 + 2 * n as u64)?;
                let es: Vec<(Str<'h>, Value<'h>)> = keys
                    .into_iter()
                    .zip(elems)
                    .filter_map(|(k, e)| match k {
                        Value::Str(s) => Some((s, e)),
                        _ => None,
                    })
                    .collect();
                Ok(Value::Map(Rc::new(MapVal::from_written(es))))
            }
            _ => {
                // countBy
                let mut seen = Distinct::new();
                let mut counts: Vec<(Str<'h>, u64)> = Vec::new();
                for k in &keys {
                    if let Value::Str(s) = k {
                        let (i, new) = seen.index(s.as_str());
                        if new {
                            counts.push((s.clone(), 0));
                        }
                        counts[i].1 += 1;
                    }
                }
                let g = counts.len() as u64;
                self.alloc(1 + 2 * g)?;
                let es: Vec<(Str<'h>, Value<'h>)> = counts
                    .into_iter()
                    .map(|(s, c)| (s, Value::Num(c as f64)))
                    .collect();
                Ok(Value::Map(Rc::new(MapVal::from_written(es))))
            }
        }
    }

    fn map_method(&mut self, rec: &Record, recv: Value<'h>, _span: Span) -> R<'h, Value<'h>> {
        let es = recv.entries();
        let n = es.len() as u64;
        let s = sz_all(es.iter().map(|(k, _)| k.as_str()));
        self.charge(n + log2_ceil(n) * (n + s))?;
        match rec.name.as_str() {
            "keys" => {
                self.alloc(1 + n)?;
                Ok(Value::list(
                    es.into_iter().map(|(k, _)| Value::Str(k)).collect(),
                ))
            }
            "values" => {
                self.alloc(1 + n)?;
                Ok(Value::list(es.into_iter().map(|(_, v)| v).collect()))
            }
            _ => {
                self.alloc(1 + 6 * n)?;
                Ok(Value::list(
                    es.into_iter()
                        .map(|(k, v)| {
                            Value::Map(Rc::new(MapVal(vec![
                                (Str::Lent("key"), Value::Str(k)),
                                (Str::Lent("value"), v),
                            ])))
                        })
                        .collect(),
                ))
            }
        }
    }

    fn list_method(
        &mut self,
        rec: &Record,
        recv: Value<'h>,
        args: &[Arg<'h>],
        span: Span,
        measure: u32,
    ) -> R<'h, Value<'h>> {
        let name = rec.name.as_str();
        let n = recv.len().unwrap_or(0);
        match name {
            // `map` and `filter`, the most frequent, each in a loop of its
            // own: the same steps as the loop below.
            "map" => {
                let f = self.lam(args, 0, span)?;
                let f = self.callback(f);
                let mut out: Vec<Value<'h>> = Vec::with_capacity(n);
                for i in 0..n {
                    self.charge(2)?;
                    let e = self.read(recv.elem(i), span)?;
                    out.push(self.call2(&f, name, i, e, Value::Num(i as f64), measure, span)?);
                }
                self.alloc(1 + out.len() as u64)?;
                Ok(Value::list(out))
            }
            "filter" => {
                let f = self.lam(args, 0, span)?;
                let f = self.callback(f);
                let mut out: Vec<Value<'h>> = Vec::new();
                for i in 0..n {
                    self.charge(2)?;
                    let e = self.read(recv.elem(i), span)?;
                    let r =
                        self.call2(&f, name, i, e.dup(), Value::Num(i as f64), measure, span)?;
                    if r.truthy() {
                        out.push(e)
                    } else {
                        release(e)
                    }
                    release(r)
                }
                self.alloc(1 + out.len() as u64)?;
                Ok(Value::list(out))
            }
            "flatMap" | "some" | "every" | "find" | "count" | "sortBy" | "groupBy" => {
                let f = self.lam(args, 0, span)?;
                let mut out: Vec<Value<'h>> = Vec::new();
                let mut keys: Vec<Value<'h>> = Vec::new();
                let mut counted = 0usize;
                #[derive(Clone, Copy, PartialEq)]
                enum Op {
                    Map,
                    Filter,
                    FlatMap,
                    Some,
                    Every,
                    Find,
                    Count,
                    GroupBy,
                    SortBy,
                }
                let op = match name {
                    "map" => {
                        out.reserve(n);
                        Op::Map
                    }
                    "filter" => Op::Filter,
                    "flatMap" => Op::FlatMap,
                    "some" => Op::Some,
                    "every" => Op::Every,
                    "find" => Op::Find,
                    "count" => Op::Count,
                    "groupBy" => Op::GroupBy,
                    _ => Op::SortBy,
                };
                let f = self.callback(f);
                // The element is kept only where the result uses it.
                let keep = matches!(op, Op::Filter | Op::Find | Op::GroupBy | Op::SortBy);
                for i in 0..n {
                    self.charge(2)?;
                    let e = self.read(recv.elem(i), span)?;
                    let (arg, e) = if keep { (e.dup(), e) } else { (e, Value::Null) };
                    let r = self.call2(&f, name, i, arg, Value::Num(i as f64), measure, span)?;
                    match op {
                        Op::Map => out.push(r),
                        Op::Filter => {
                            if r.truthy() {
                                out.push(e)
                            } else {
                                release(e)
                            }
                            release(r)
                        }
                        Op::FlatMap => {
                            if r.kind() != Kind::List {
                                return Err(fail(
                                    "wrong-kind",
                                    "result",
                                    span,
                                    vec![("name", name.to_string()), ("kind", kind_p(&r))],
                                ));
                            }
                            for j in 0..r.len().unwrap_or(0) {
                                out.push(r.elem(j));
                            }
                        }
                        Op::Some if r.truthy() => return Ok(Value::Bool(true)),
                        Op::Every if !r.truthy() => return Ok(Value::Bool(false)),
                        Op::Find if r.truthy() => return Ok(e),
                        Op::Count => {
                            counted += r.truthy() as usize;
                            release(r)
                        }
                        Op::Some | Op::Every | Op::Find => {
                            release(r);
                            release(e)
                        }
                        Op::GroupBy => {
                            if r.kind() != Kind::String {
                                return Err(fail(
                                    "wrong-kind",
                                    "result",
                                    span,
                                    vec![("name", name.to_string()), ("kind", kind_p(&r))],
                                ));
                            }
                            keys.push(r);
                            out.push(e);
                        }
                        Op::SortBy => {
                            keys.push(r);
                            out.push(e);
                        }
                    }
                }
                match name {
                    "map" | "filter" => {
                        self.alloc(1 + out.len() as u64)?;
                        Ok(Value::list(out))
                    }
                    "flatMap" => {
                        let k = out.len() as u64;
                        self.charge(k)?;
                        self.alloc(1 + k)?;
                        Ok(Value::list(out))
                    }
                    "some" => Ok(Value::Bool(false)),
                    "every" => Ok(Value::Bool(true)),
                    "find" => Ok(Value::Null),
                    "count" => Ok(Value::Num(counted as f64)),
                    "sortBy" => {
                        if let Err((_, k)) = ordered(keys.iter()) {
                            return Err(self.unordered(name, k, span));
                        }
                        let nn = n as u64;
                        let s = sz_all(keys.iter().filter_map(|k| k.as_str()));
                        self.charge(log2_ceil(nn) * (nn + s))?;
                        self.alloc(1 + nn)?;
                        let mut idx: Vec<usize> = (0..n).collect();
                        // F7 6.3: `"desc"` is a stable sort in descending
                        // order of the keys.
                        let desc = matches!(self.val(args, 1), Some(Value::Str(d)) if d.as_str() == "desc");
                        if desc {
                            idx.sort_by(|&a, &b| cmp(&keys[b], &keys[a]));
                        } else {
                            idx.sort_by(|&a, &b| cmp(&keys[a], &keys[b]));
                        }
                        Ok(Value::list(idx.into_iter().map(|i| out[i].dup()).collect()))
                    }
                    _ => {
                        // groupBy
                        let s = sz_all(keys.iter().filter_map(|k| k.as_str()));
                        self.charge(s)?;
                        let mut seen = Distinct::new();
                        let mut groups: Vec<(Str<'h>, Vec<Value<'h>>)> = Vec::new();
                        for (k, e) in keys.iter().zip(out.iter()) {
                            let Value::Str(ks) = k else { continue };
                            let (i, new) = seen.index(ks.as_str());
                            if new {
                                groups.push((ks.clone(), Vec::new()));
                            }
                            groups[i].1.push(e.dup());
                        }
                        let g = groups.len() as u64;
                        self.alloc(1 + 3 * g + n as u64)?;
                        let es: Vec<(Str<'h>, Value<'h>)> = groups
                            .into_iter()
                            .map(|(k, list)| (k, Value::list(list)))
                            .collect();
                        Ok(Value::Map(Rc::new(MapVal::from_written(es))))
                    }
                }
            }
            "reduce" => {
                let f = self.lam(args, 0, span)?;
                let f = self.callback(f);
                let mut acc = self.val(args, 1).unwrap_or(Value::Null);
                for i in 0..n {
                    self.charge(2)?;
                    let e = self.read(recv.elem(i), span)?;
                    acc = self.call3(&f, name, i, [acc, e, Value::Num(i as f64)], measure, span)?;
                }
                Ok(acc)
            }
            "sum" => {
                let mut xs = Vec::with_capacity(n);
                for i in 0..n {
                    let e = self.read(recv.elem(i), span)?;
                    match e {
                        Value::Num(x) => xs.push(x),
                        other => {
                            let f = self.element(name, &other, span);
                            return self.scan_failed(i + 1, f);
                        }
                    }
                }
                self.charge(n as u64)?;
                let mut t = 0.0;
                for x in xs {
                    t = match number::add(t, x) {
                        Ok(v) => v,
                        Err(NumErr::Range | NumErr::DivZero) => {
                            return Err(self.entry_range(name, span))
                        }
                    };
                }
                Ok(Value::Num(t))
            }
            "min" | "max" | "sort" | "unique" => {
                let xs = self.ordered_scan(name, &recv, span)?;
                let nn = n as u64;
                let s = sz_all(xs.iter().filter_map(|v| v.as_str()));
                match name {
                    "min" | "max" => {
                        self.charge(nn + s)?;
                        let want = if name == "min" {
                            Ordering::Less
                        } else {
                            Ordering::Greater
                        };
                        let mut best: Option<Value<'h>> = None;
                        for v in xs {
                            if best.as_ref().is_none_or(|b| cmp(&v, b) == want) {
                                best = Some(v);
                            }
                        }
                        Ok(best.unwrap_or(Value::Null))
                    }
                    "sort" => {
                        self.charge(nn + log2_ceil(nn) * (nn + s))?;
                        self.alloc(1 + nn)?;
                        let mut xs = xs;
                        xs.sort_by(cmp);
                        Ok(Value::list(xs))
                    }
                    _ => {
                        self.charge(nn + s)?;
                        let mut seen_n: HashSet<u64> = HashSet::new();
                        let mut seen_s = Distinct::new();
                        let keep: Vec<bool> = xs
                            .iter()
                            .map(|v| match v {
                                Value::Num(x) => {
                                    seen_n.insert(if *x == 0.0 { 0 } else { x.to_bits() })
                                }
                                Value::Str(t) => seen_s.index(t.as_str()).1,
                                _ => false,
                            })
                            .collect();
                        drop(seen_s);
                        let out: Vec<Value<'h>> = xs
                            .into_iter()
                            .zip(keep)
                            .filter_map(|(v, k)| k.then_some(v))
                            .collect();
                        self.alloc(1 + out.len() as u64)?;
                        Ok(Value::list(out))
                    }
                }
            }
            "toMap" => {
                let mut es: Vec<(Str<'h>, Value<'h>)> = Vec::with_capacity(n);
                for i in 0..n {
                    let e = self.read(recv.elem(i), span)?;
                    if e.kind() != Kind::Map {
                        let f = self.element(name, &e, span);
                        return self.scan_failed(i + 1, f);
                    }
                    let missing = |f: &str| {
                        fail(
                            "missing-field",
                            "field",
                            span,
                            vec![("name", f.to_string())],
                        )
                    };
                    let Some(k) = e.field("key") else {
                        return self.scan_failed(i + 1, missing("key"));
                    };
                    let k = self.read(k, span)?;
                    let Value::Str(ks) = k else {
                        let f = fail("wrong-kind", "key", span, vec![("found", value_p(&k))]);
                        return self.scan_failed(i + 1, f);
                    };
                    let Some(v) = e.field("value") else {
                        return self.scan_failed(i + 1, missing("value"));
                    };
                    es.push((ks, v));
                }
                let s = sz_all(es.iter().map(|(k, _)| k.as_str()));
                self.charge(n as u64 + s)?;
                self.alloc(1 + 2 * n as u64)?;
                Ok(Value::Map(Rc::new(MapVal::from_written(es))))
            }
            "flat" => {
                let mut k = 0usize;
                for i in 0..n {
                    let e = self.read(recv.elem(i), span)?;
                    if e.kind() != Kind::List {
                        let f = self.element(name, &e, span);
                        return self.scan_failed(i + 1, f);
                    }
                    k += e.len().unwrap_or(0);
                }
                self.charge((n + k) as u64)?;
                self.alloc(1 + k as u64)?;
                let mut out = Vec::with_capacity(k);
                for i in 0..n {
                    let e = recv.elem(i);
                    for j in 0..e.len().unwrap_or(0) {
                        out.push(e.elem(j));
                    }
                }
                Ok(Value::list(out))
            }
            "reverse" => {
                self.charge(n as u64)?;
                self.alloc(1 + n as u64)?;
                Ok(Value::list((0..n).rev().map(|i| recv.elem(i)).collect()))
            }
            "first" | "last" => {
                if n == 0 {
                    return Ok(Value::Null);
                }
                let i = if name == "first" { 0 } else { n - 1 };
                self.read(recv.elem(i), span)
            }
            "slice" => {
                let num = |v: Option<Value<'h>>| match v {
                    Some(Value::Num(x)) => Some(x),
                    _ => None,
                };
                let (a, b) = positions(
                    num(self.val(args, 0)).unwrap_or(0.0),
                    num(self.val(args, 1)),
                    n,
                );
                let k = (b - a) as u64;
                self.charge(k)?;
                self.alloc(1 + k)?;
                Ok(Value::list((a..b).map(|i| recv.elem(i)).collect()))
            }
            "join" => {
                let sep = self
                    .val(args, 0)
                    .and_then(|v| v.as_str().map(String::from))
                    .unwrap_or_default();
                let mut parts: Vec<Str<'h>> = Vec::with_capacity(n);
                for i in 0..n {
                    let e = self.read(recv.elem(i), span)?;
                    match e {
                        Value::Str(s) => parts.push(s),
                        other => {
                            let f = self.element(name, &other, span);
                            return self.scan_failed(i + 1, f);
                        }
                    }
                }
                let w: u128 = parts
                    .iter()
                    .map(|p| weight(p.as_str()) as u128)
                    .sum::<u128>()
                    + (n.saturating_sub(1) as u128) * weight(&sep) as u128;
                let sz_r = (w / 64).min(u64::MAX as u128) as u64;
                self.charge((n as u64).saturating_add(sz_r))?;
                self.alloc(sz_r.saturating_add(1))?;
                let bytes: usize = parts.iter().map(|p| p.as_str().len()).sum::<usize>()
                    + n.saturating_sub(1) * sep.len();
                let mut out = String::with_capacity(bytes);
                for (i, p) in parts.iter().enumerate() {
                    if i > 0 {
                        out.push_str(&sep);
                    }
                    out.push_str(p.as_str());
                }
                Ok(Value::str(out))
            }
            _ => Err(self.internal(span)),
        }
    }

    /// The elements of a list under the ordered test of 3.6, each read as it
    /// is tested. A failed test charges by 4.5, then fails.
    fn ordered_scan(&mut self, name: &str, recv: &Value<'h>, span: Span) -> R<'h, Vec<Value<'h>>> {
        let n = recv.len().unwrap_or(0);
        let mut xs: Vec<Value<'h>> = Vec::with_capacity(n);
        let mut first: Option<Kind> = None;
        for i in 0..n {
            let e = self.read(recv.elem(i), span)?;
            let k = e.kind();
            if !matches!(k, Kind::Number | Kind::String) || first.is_some_and(|f| f != k) {
                let f = self.unordered(name, k, span);
                self.charge(i as u64 + 1)?;
                return Err(f);
            }
            first.get_or_insert(k);
            xs.push(e);
        }
        Ok(xs)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_formulas_are_the_records() {
        let recs = records();
        // 53 of Core, 8 of F6, 6 of F7 and 11 of F13 (`f13-math.md` 1.1).
        assert_eq!(recs.len(), 78);
        for r in recs {
            let f = FORMULAS
                .iter()
                .find(|(n, k, _, _)| *n == r.name && *k == r.receiver)
                .unwrap_or_else(|| panic!("no entry for {}", r.name));
            assert_eq!(
                (f.2, f.3),
                (r.steps.as_str(), r.units.as_str()),
                "{}",
                r.name
            );
        }
    }

    #[test]
    fn fixed_rounds_the_exact_value() {
        for (x, d, t) in [
            (2.5, 0, "3"),
            (-2.5, 0, "-3"),
            (1.005, 2, "1.00"),
            (-0.001, 2, "0.00"),
            (1e20, 2, "100000000000000000000.00"),
            (0.1, 20, "0.10000000000000000555"),
            (7.0, 0, "7"),
            (12.345, 1, "12.3"),
            (0.0, 3, "0.000"),
            (5e-324, 20, "0.00000000000000000000"),
        ] {
            assert_eq!(fixed_text(x, d), t, "{x} {d}");
        }
    }
}
