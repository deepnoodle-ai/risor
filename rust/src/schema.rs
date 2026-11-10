//! Package F1: the type of a JSON Schema (`spec/packages/f01-host-types.md`
//! 3 and 4).
//!
//! The mapping walks only what sections 3.1 to 3.8 read, and checks a
//! keyword only where it reads it. A schema that a `$ref` names is mapped
//! once, by its place in the document, and its type is shared: the type's
//! element and record slots are reference-counted, so two `$ref`s to one
//! definition at each of k levels make k types, not 2^k (3.10 item 5).

use crate::check::ty::{fits, join, join_all, Field, Rec, Strs, Ty};
use crate::json::Json;
use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet};
use std::sync::Arc;

/// The three options of 4. Each is off unless given.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct SchemaOptions {
    /// `closed` (4.1)
    pub closed: bool,
    /// `absent-is-null` (4.2)
    pub absent_is_null: bool,
    /// `defaults-applied` (4.3)
    pub defaults_applied: bool,
}

impl SchemaOptions {
    /// The options named, as a case file writes them. A name that is not one
    /// of the three, or a name given twice, is an error.
    pub fn from_names<S: AsRef<str>>(names: &[S]) -> Result<SchemaOptions, String> {
        let mut o = SchemaOptions::default();
        for n in names {
            let flag = match n.as_ref() {
                "closed" => &mut o.closed,
                "absent-is-null" => &mut o.absent_is_null,
                "defaults-applied" => &mut o.defaults_applied,
                other => return Err(format!("{other:?} is not an option of a schema")),
            };
            if *flag {
                return Err(format!("the option {:?} is given twice", n.as_ref()));
            }
            *flag = true;
        }
        Ok(o)
    }
}

/// The deepest a schema, or the type it gives, may be (3.10).
const MAX_DEPTH: usize = 64;

/// The largest whole number that is `int` (`05-types.md` 2.1).
const MAX_SAFE: f64 = 9_007_199_254_740_991.0;

/// M(S) with the options given: the type of a schema (3, 4). The error is
/// the message of the `declare` error `bad-type`, and says where in the
/// schema it is, as a JSON Pointer.
pub fn ty_schema(schema: &Json, opts: SchemaOptions) -> Result<Ty, String> {
    let mut m = Mapper {
        root: schema,
        opts,
        targets: HashMap::new(),
        in_progress: HashSet::new(),
        path: Vec::new(),
    };
    let (t, _) = m.map(schema, 0)?;
    let t = if opts.closed { close(&t) } else { t };
    if type_depth(&t, &mut HashMap::new()) > MAX_DEPTH {
        return Err("the type the schema gives is deeper than 64".into());
    }
    Ok(t)
}

struct Mapper<'a> {
    root: &'a Json,
    opts: SchemaOptions,
    /// The type and depth of each schema a `$ref` named, by its place.
    targets: HashMap<usize, (Ty, usize)>,
    /// The schemas whose mapping is in progress, by place (3.7 item 7).
    in_progress: HashSet<usize>,
    /// Where the walk is, for messages.
    path: Vec<String>,
}

fn place(s: &Json) -> usize {
    s as *const Json as usize
}

type R<T> = Result<T, String>;

impl<'a> Mapper<'a> {
    fn err<T>(&self, msg: impl Into<String>) -> R<T> {
        let mut at = String::new();
        for p in &self.path {
            at.push('/');
            at.push_str(&p.replace('~', "~0").replace('/', "~1"));
        }
        if at.is_empty() {
            at.push('#');
        } else {
            at.insert(0, '#');
        }
        Err(format!("the schema at {at}: {}", msg.into()))
    }

    /// Map a subschema under the keyword path `keys`, one level down.
    fn sub(&mut self, s: &'a Json, level: usize, keys: &[&str]) -> R<(Ty, usize)> {
        for k in keys {
            self.path.push(k.to_string());
        }
        let r = self.map(s, level + 1);
        for _ in keys {
            self.path.pop();
        }
        r
    }

    /// M(S) and the depth of S (3.10). `level` is how far below the root
    /// the walk is: a schema below level 64 makes the root deeper than 64.
    fn map(&mut self, s: &'a Json, level: usize) -> R<(Ty, usize)> {
        if level > MAX_DEPTH {
            return self.err("the schema is deeper than 64");
        }
        // Step 1.
        let obj = match s {
            Json::Bool(true) => return Ok((Ty::any(), 0)),
            Json::Bool(false) => return Ok((Ty::never(), 0)),
            Json::Obj(o) => o,
            _ => {
                return self.err(format!(
                    "a schema is an object or a boolean, not {}",
                    kind(s)
                ))
            }
        };
        let at = place(s);
        self.in_progress.insert(at);
        let r = self.map_object(s, obj, level);
        self.in_progress.remove(&at);
        let (t, d) = r?;
        if d > MAX_DEPTH {
            return self.err("the schema is deeper than 64");
        }
        Ok((t, d))
    }

    fn map_object(
        &mut self,
        s: &'a Json,
        obj: &'a [(String, Json)],
        level: usize,
    ) -> R<(Ty, usize)> {
        let get = |k: &str| obj.iter().find(|(n, _)| n == k).map(|(_, v)| v);
        let nullable = matches!(get("nullable"), Some(Json::Bool(true)));
        // Step 2.
        if place(s) != place(self.root) && get("$id").is_some() {
            return self.err("$id below the root: the mapping reads no embedded resource");
        }
        for k in ["$dynamicRef", "$recursiveRef"] {
            if get(k).is_some() {
                return self.err(format!("{k} is not read"));
            }
        }
        // Step 3.
        if let Some(r) = get("$ref") {
            let target = self.resolve(r)?;
            let (t, d) = match self.targets.get(&place(target)) {
                Some(x) => x.clone(),
                None => {
                    if self.in_progress.contains(&place(target)) {
                        return self.err("a cycle of $ref: a type cannot refer to itself");
                    }
                    let x = self.sub(target, level, &["$ref"])?;
                    self.targets.insert(place(target), x.clone());
                    x
                }
            };
            let t = if nullable { join(&t, &Ty::null()) } else { t };
            return Ok((t, d + 1));
        }
        // Step 4: the parts.
        let mut parts: Vec<Ty> = Vec::new();
        let mut depth = 0;
        let deeper = |d: usize, depth: &mut usize| *depth = (*depth).max(d + 1);
        if get("type").is_some() || get("enum").is_some() || get("const").is_some() {
            let (t, d) = self.kind_part(obj, level)?;
            if let Some(d) = d {
                deeper(d, &mut depth);
            }
            parts.push(t);
        }
        for key in ["allOf", "anyOf", "oneOf"] {
            let Some(v) = get(key) else { continue };
            let branches = match v {
                Json::Arr(a) if !a.is_empty() => a,
                _ => return self.err(format!("{key} is not a non-empty array of schemas")),
            };
            let mut ts = Vec::with_capacity(branches.len());
            for (i, b) in branches.iter().enumerate() {
                let (t, d) = self.sub(b, level, &[key, &i.to_string()])?;
                deeper(d, &mut depth);
                ts.push(t);
            }
            let part = if key == "allOf" {
                pick(&ts)
            } else {
                if has_tag(&ts) {
                    return self.err(format!(
                        "the records of the branches of {key} have a tag: a tagged union needs F5"
                    ));
                }
                join_all(ts.iter())
            };
            parts.push(part);
        }
        // Steps 5 and 6.
        let t = pick(&parts);
        let t = if nullable { join(&t, &Ty::null()) } else { t };
        Ok((t, depth))
    }

    /// The kind part (3.2), and the depth of the subschemas it walks, if it
    /// walks any.
    fn kind_part(&mut self, obj: &'a [(String, Json)], level: usize) -> R<(Ty, Option<usize>)> {
        let get = |k: &str| obj.iter().find(|(n, _)| n == k).map(|(_, v)| v);
        let names = match get("type") {
            None => None,
            Some(t) => Some(self.type_names(t)?),
        };
        let values: Option<Vec<&Json>> = if let Some(c) = get("const") {
            Some(vec![c])
        } else if let Some(e) = get("enum") {
            match e {
                Json::Arr(a) => Some(a.iter().collect()),
                _ => {
                    self.path.push("enum".into());
                    let r = self.err("enum is not an array");
                    self.path.pop();
                    return r;
                }
            }
        } else {
            None
        };
        if let Some(values) = values {
            let mut ts = Vec::new();
            for v in values {
                if names
                    .as_ref()
                    .is_none_or(|ns| ns.iter().any(|n| allows(n, v)))
                {
                    match literal(v, 0) {
                        Some(t) => ts.push(t),
                        None => return self.err("a value of enum or const is deeper than 64"),
                    }
                }
            }
            return Ok((join_all(ts.iter()), None));
        }
        let names = names.unwrap_or_default();
        let mut out = Ty::never();
        let mut depth: Option<usize> = None;
        for n in &names {
            let t = match n.as_str() {
                "null" => Ty::null(),
                "boolean" => Ty::boolean(),
                "integer" => Ty::int(),
                "number" => Ty::float(),
                "string" => Ty::string(),
                "array" => {
                    let (e, d) = self.array(obj, level)?;
                    if let Some(d) = d {
                        depth = Some(depth.unwrap_or(0).max(d));
                    }
                    Ty::list(e)
                }
                _ => {
                    let (r, d) = self.object(obj, level)?;
                    if let Some(d) = d {
                        depth = Some(depth.unwrap_or(0).max(d));
                    }
                    Ty::rec(r)
                }
            };
            out = join(&out, &t);
        }
        Ok((out, depth))
    }

    /// The names of `type` (3.3), each once.
    fn type_names(&self, t: &Json) -> R<BTreeSet<String>> {
        const NAMES: [&str; 7] = [
            "null", "boolean", "integer", "number", "string", "array", "object",
        ];
        let list: Vec<&Json> = match t {
            Json::Str(_) => vec![t],
            Json::Arr(a) if !a.is_empty() => a.iter().collect(),
            _ => return self.err("type is not a name or a non-empty array of names"),
        };
        let mut out = BTreeSet::new();
        for n in list {
            match n.as_str() {
                Some(s) if NAMES.contains(&s) => {
                    out.insert(s.to_string());
                }
                Some(s) => return self.err(format!("{s:?} is not a name of type")),
                None => return self.err("type holds a value that is not a string"),
            }
        }
        Ok(out)
    }

    /// The element type of an `array` (3.5), and the greatest depth of the
    /// subschemas it walks.
    fn array(&mut self, obj: &'a [(String, Json)], level: usize) -> R<(Ty, Option<usize>)> {
        let get = |k: &str| obj.iter().find(|(n, _)| n == k).map(|(_, v)| v);
        let mut ts = Vec::new();
        let mut depth: Option<usize> = None;
        let mut note = |d: usize| depth = Some(depth.unwrap_or(0).max(d));
        if let Some(p) = get("prefixItems") {
            let Json::Arr(a) = p else {
                return self.err("prefixItems is not an array of schemas");
            };
            for (i, x) in a.iter().enumerate() {
                let (t, d) = self.sub(x, level, &["prefixItems", &i.to_string()])?;
                note(d);
                ts.push(t);
            }
        }
        match get("items") {
            None => ts.push(Ty::any()),
            Some(Json::Arr(a)) => {
                for (i, x) in a.iter().enumerate() {
                    let (t, d) = self.sub(x, level, &["items", &i.to_string()])?;
                    note(d);
                    ts.push(t);
                }
                match get("additionalItems") {
                    Some(x) => {
                        let (t, d) = self.sub(x, level, &["additionalItems"])?;
                        note(d);
                        ts.push(t);
                    }
                    None => ts.push(Ty::any()),
                }
            }
            Some(x @ (Json::Obj(_) | Json::Bool(_))) => {
                let (t, d) = self.sub(x, level, &["items"])?;
                note(d);
                ts.push(t);
            }
            Some(_) => return self.err("items is neither a schema nor an array of schemas"),
        }
        Ok((join_all(ts.iter()), depth))
    }

    /// The record of an `object` (3.8, 4), and the greatest depth of the
    /// subschemas it walks.
    fn object(&mut self, obj: &'a [(String, Json)], level: usize) -> R<(Rec, Option<usize>)> {
        let get = |k: &str| obj.iter().find(|(n, _)| n == k).map(|(_, v)| v);
        let mut depth: Option<usize> = None;
        let mut note = |d: usize| depth = Some(depth.unwrap_or(0).max(d));
        let additional = get("additionalProperties");
        let patterns = get("patternProperties");
        // R0, the open rest.
        let rest = match additional {
            None => Ty::any(),
            Some(a) => {
                let (mut r0, d) = self.sub(a, level, &["additionalProperties"])?;
                note(d);
                if let Some(p) = patterns {
                    let Json::Obj(ps) = p else {
                        return self.err("patternProperties is not an object of schemas");
                    };
                    for (k, x) in ps {
                        let (t, d) = self.sub(x, level, &["patternProperties", k])?;
                        note(d);
                        r0 = join(&r0, &t);
                    }
                }
                r0
            }
        };
        let required: Vec<&str> = match get("required") {
            None => Vec::new(),
            Some(Json::Arr(a)) => {
                let mut out = Vec::new();
                for x in a {
                    match x.as_str() {
                        Some(s) => out.push(s),
                        None => return self.err("required holds a value that is not a string"),
                    }
                }
                out
            }
            Some(_) => return self.err("required is not an array of strings"),
        };
        let mut fields: BTreeMap<String, Field> = BTreeMap::new();
        let props = get("properties");
        if let Some(p) = props {
            let Json::Obj(ps) = p else {
                return self.err("properties is not an object of schemas");
            };
            for (k, x) in ps {
                let (t, d) = self.sub(x, level, &["properties", k])?;
                note(d);
                let req = required.contains(&k.as_str());
                let default = match x {
                    Json::Obj(o) => o.iter().find(|(n, _)| n == "default").map(|(_, v)| v),
                    _ => None,
                };
                let field = if req {
                    Field {
                        ty: t,
                        optional: false,
                    }
                } else if let (true, Some(dv)) = (self.opts.defaults_applied, default) {
                    let Some(lit) = literal(dv, 0) else {
                        return self.err("a default is deeper than 64");
                    };
                    Field {
                        ty: join(&t, &lit),
                        optional: false,
                    }
                } else if self.opts.absent_is_null {
                    Field {
                        ty: join(&t, &Ty::null()),
                        optional: false,
                    }
                } else {
                    Field {
                        ty: t,
                        optional: true,
                    }
                };
                fields.insert(k.clone(), field);
            }
        }
        for k in &required {
            if !fields.contains_key(*k) {
                fields.insert(
                    k.to_string(),
                    Field {
                        ty: rest.clone(),
                        optional: false,
                    },
                );
            }
        }
        let closable =
            self.opts.closed && props.is_some() && additional.is_none() && patterns.is_none();
        Ok((
            Rec {
                fields,
                rest,
                closable,
            },
            depth,
        ))
    }

    /// The schema a `$ref` names (3.7).
    fn resolve(&self, r: &Json) -> R<&'a Json> {
        let Some(text) = r.as_str() else {
            return self.err("$ref is not a string");
        };
        let Some(frag) = text.strip_prefix('#') else {
            return self.err(format!(
                "$ref {text:?} is not local: it does not start with #"
            ));
        };
        let Some(pointer) = percent_decode(frag) else {
            return self.err(format!("$ref {text:?} is not well percent-encoded UTF-8"));
        };
        if pointer.is_empty() {
            return Ok(self.root);
        }
        let Some(tail) = pointer.strip_prefix('/') else {
            return self.err(format!("$ref {text:?} is not a JSON Pointer"));
        };
        let mut at: &'a Json = self.root;
        for raw in tail.split('/') {
            let Some(token) = unescape(raw) else {
                return self.err(format!("$ref {text:?} has a ~ that is not ~0 or ~1"));
            };
            let next = match at {
                Json::Obj(m) => m.iter().find(|(k, _)| *k == token).map(|(_, v)| v),
                Json::Arr(a) => index(&token).and_then(|i| a.get(i)),
                _ => None,
            };
            match next {
                Some(n) => at = n,
                None => return self.err(format!("$ref {text:?} names nothing")),
            }
        }
        Ok(at)
    }
}

fn kind(j: &Json) -> &'static str {
    match j {
        Json::Null => "null",
        Json::Bool(_) => "a boolean",
        Json::Num(_) => "a number",
        Json::Str(_) => "a string",
        Json::Arr(_) => "an array",
        Json::Obj(_) => "an object",
    }
}

/// Whether the name of `type` allows the value (3.3).
fn allows(name: &str, v: &Json) -> bool {
    match (name, v) {
        ("null", Json::Null) => true,
        ("boolean", Json::Bool(_)) => true,
        ("integer", Json::Num(n)) => n.fract() == 0.0,
        ("number", Json::Num(_)) => true,
        ("string", Json::Str(_)) => true,
        ("array", Json::Arr(_)) => true,
        ("object", Json::Obj(_)) => true,
        _ => false,
    }
}

/// The literal type of a JSON value (3.4), or `None` when it is deeper than
/// 64.
pub fn literal(v: &Json, level: usize) -> Option<Ty> {
    if level > MAX_DEPTH {
        return None;
    }
    Some(match v {
        Json::Null => Ty::null(),
        Json::Bool(_) => Ty::boolean(),
        Json::Num(n) => {
            if n.fract() == 0.0 && n.abs() <= MAX_SAFE {
                Ty::int()
            } else {
                Ty::float()
            }
        }
        Json::Str(s) => Ty::lit(s),
        Json::Arr(a) => {
            let mut e = Ty::never();
            for x in a {
                e = join(&e, &literal(x, level + 1)?);
            }
            Ty::list(e)
        }
        Json::Obj(m) => {
            let mut fields = BTreeMap::new();
            for (k, x) in m {
                fields.insert(
                    k.clone(),
                    Field {
                        ty: literal(x, level + 1)?,
                        optional: false,
                    },
                );
            }
            Ty::rec(Rec::new(fields, Ty::never()))
        }
    })
}

/// A type with one record alternative and nothing else (3.6 item 2.2).
fn record_only(t: &Ty) -> Option<&Rec> {
    if !t.any && t.kinds() == 1 {
        t.record()
    } else {
        None
    }
}

/// Pick(T1, ..., Tn) (3.6 item 2).
pub fn pick(ts: &[Ty]) -> Ty {
    let left: Vec<&Ty> = ts.iter().filter(|t| !t.any).collect();
    if left.is_empty() {
        return Ty::any();
    }
    if left.len() == 1 {
        return left[0].clone();
    }
    let recs: Option<Vec<&Rec>> = left.iter().map(|t| record_only(t)).collect();
    if let Some(recs) = recs {
        let mut acc = recs[0].clone();
        for r in &recs[1..] {
            acc = meet(&acc, r);
        }
        return Ty::rec(acc);
    }
    for (i, t) in left.iter().enumerate() {
        let all = left
            .iter()
            .enumerate()
            .all(|(j, u)| i == j || fits(t, u) == Some(false));
        if all {
            return (*t).clone();
        }
    }
    left[0].clone()
}

/// Meet(A, B) of two records (3.6 item 3, 8).
pub fn meet(a: &Rec, b: &Rec) -> Rec {
    let keys: BTreeSet<&String> = a.fields.keys().chain(b.fields.keys()).collect();
    let mut fields = BTreeMap::new();
    for k in keys {
        let fa = a.fields.get(k);
        let fb = b.fields.get(k);
        let ta = fa.map_or(&a.rest, |f| &f.ty);
        let tb = fb.map_or(&b.rest, |f| &f.ty);
        let required = fa.is_some_and(|f| !f.optional) || fb.is_some_and(|f| !f.optional);
        fields.insert(
            k.clone(),
            Field {
                ty: pick(&[ta.clone(), tb.clone()]),
                optional: !required,
            },
        );
    }
    Rec {
        fields,
        rest: pick(&[a.rest.clone(), b.rest.clone()]),
        closable: a.closable && b.closable,
    }
}

/// Whether two or more records of the branches of one `anyOf` or `oneOf`
/// have a tag (3.9).
fn has_tag(branches: &[Ty]) -> bool {
    let recs: Vec<&Rec> = branches.iter().filter_map(|t| t.record()).collect();
    if recs.len() < 2 {
        return false;
    }
    'keys: for k in recs[0].fields.keys() {
        let mut seen: BTreeSet<&String> = BTreeSet::new();
        for r in &recs {
            let Some(f) = r.fields.get(k) else {
                continue 'keys;
            };
            if f.optional || f.ty.any || f.ty.kinds() != 1 {
                continue 'keys;
            }
            let Some(Strs::Lits(set)) = &f.ty.str else {
                continue 'keys;
            };
            for s in set {
                if !seen.insert(s) {
                    continue 'keys;
                }
            }
        }
        return true;
    }
    false
}

/// Close(T) (4.1 item 3): the rest of every closable record of T is
/// `never`, at every depth. Shared parts are closed once.
pub fn close(t: &Ty) -> Ty {
    Closer::default().ty(t)
}

#[derive(Default)]
struct Closer {
    lists: HashMap<usize, Arc<Ty>>,
    recs: HashMap<usize, Arc<Rec>>,
}

impl Closer {
    fn ty(&mut self, t: &Ty) -> Ty {
        let mut out = t.clone();
        if let Some(e) = &t.list {
            let key = Arc::as_ptr(e) as usize;
            let c = match self.lists.get(&key) {
                Some(c) => c.clone(),
                None => {
                    let c = Arc::new(self.ty(e));
                    self.lists.insert(key, c.clone());
                    c
                }
            };
            out.list = Some(c);
        }
        if let Some(r) = &t.map {
            let key = Arc::as_ptr(r) as usize;
            let c = match self.recs.get(&key) {
                Some(c) => c.clone(),
                None => {
                    let fields = r
                        .fields
                        .iter()
                        .map(|(k, f)| {
                            (
                                k.clone(),
                                Field {
                                    ty: self.ty(&f.ty),
                                    optional: f.optional,
                                },
                            )
                        })
                        .collect();
                    let rest = if r.closable {
                        Ty::never()
                    } else {
                        self.ty(&r.rest)
                    };
                    let c = Arc::new(Rec::new(fields, rest));
                    self.recs.insert(key, c.clone());
                    c
                }
            };
            out.map = Some(c);
        }
        out
    }
}

/// The depth of a type (`05-types.md` 2.5), with shared parts measured once.
fn type_depth(t: &Ty, memo: &mut HashMap<usize, usize>) -> usize {
    let mut d = 0;
    if let Some(e) = &t.list {
        let key = Arc::as_ptr(e) as usize;
        let inner = match memo.get(&key) {
            Some(x) => *x,
            None => {
                let x = type_depth(e, memo);
                memo.insert(key, x);
                x
            }
        };
        d = d.max(1 + inner);
    }
    if let Some(r) = &t.map {
        let key = Arc::as_ptr(r) as usize;
        let inner = match memo.get(&key) {
            Some(x) => *x,
            None => {
                let mut x = type_depth(&r.rest, memo);
                for f in r.fields.values() {
                    x = x.max(type_depth(&f.ty, memo));
                }
                memo.insert(key, x);
                x
            }
        };
        d = d.max(1 + inner);
    }
    d
}

/// Percent-decoding (RFC 3986) of the text after `#`. `None` for a `%` not
/// followed by two hexadecimal digits, or bytes that are not UTF-8.
fn percent_decode(s: &str) -> Option<String> {
    let b = s.as_bytes();
    let mut out = Vec::with_capacity(b.len());
    let mut i = 0;
    while i < b.len() {
        if b[i] == b'%' {
            let hex = |c: u8| (c as char).to_digit(16);
            let hi = hex(*b.get(i + 1)?)?;
            let lo = hex(*b.get(i + 2)?)?;
            out.push((hi * 16 + lo) as u8);
            i += 3;
        } else {
            out.push(b[i]);
            i += 1;
        }
    }
    String::from_utf8(out).ok()
}

/// A reference token of a JSON Pointer with `~1` and `~0` read.
fn unescape(raw: &str) -> Option<String> {
    let mut out = String::with_capacity(raw.len());
    let mut cs = raw.chars();
    while let Some(c) = cs.next() {
        if c == '~' {
            match cs.next() {
                Some('0') => out.push('~'),
                Some('1') => out.push('/'),
                _ => return None,
            }
        } else {
            out.push(c);
        }
    }
    Some(out)
}

/// An array index of a JSON Pointer: `0`, or a digit from 1 to 9 followed
/// by digits.
fn index(t: &str) -> Option<usize> {
    let b = t.as_bytes();
    let ok = !b.is_empty() && b.iter().all(u8::is_ascii_digit) && (b == b"0" || b[0] != b'0');
    if ok {
        t.parse().ok()
    } else {
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::check::ty::{from_text, print};
    use crate::json::parse;
    use crate::types::text::parse_type;

    fn ty(s: &str) -> Ty {
        from_text(&parse_type(s).unwrap(), &[], None).unwrap()
    }

    fn map(schema: &str, opts: &[&str]) -> String {
        let j = parse(schema).unwrap();
        match ty_schema(&j, SchemaOptions::from_names(opts).unwrap()) {
            Ok(t) => print(&t),
            Err(e) => format!("error: {e}"),
        }
    }

    #[test]
    fn pick_drops_any_and_takes_the_first_that_fits() {
        assert_eq!(print(&pick(&[])), "any");
        assert_eq!(print(&pick(&[Ty::any(), Ty::any()])), "any");
        assert_eq!(print(&pick(&[Ty::any(), ty("string")])), "string");
        assert_eq!(print(&pick(&[ty("string"), ty("int")])), "string");
        assert_eq!(print(&pick(&[ty("string"), ty("never")])), "never");
        assert_eq!(print(&pick(&[ty("string | int"), ty("string")])), "string");
        assert_eq!(print(&pick(&[ty("int"), ty("float")])), "int");
        // A record with another alternative does not meet.
        assert_eq!(
            print(&pick(&[ty("{a: int, ...} | null"), ty("{b: int, ...}")])),
            "{a: int, ...} | null"
        );
    }

    #[test]
    fn meet_laws() {
        let m = |a: &str, b: &str| print(&pick(&[ty(a), ty(b)]));
        assert_eq!(m("{a?: int}", "{b: string, ...}"), "{a?: int, b: never}");
        assert_eq!(m("map<float>", "{a: int, ...}"), "{a: int, ...float}");
        assert_eq!(m("{a: int, ...}", "{a?: float, ...}"), "{a: int, ...}");
        assert_eq!(
            m("{p: {x: int, ...}, ...}", "{p?: {y: string, ...}, ...}"),
            "{p: {x: int, y: string, ...}, ...}"
        );
        // Idempotent, and for records whose shared keys agree, the meet fits
        // both sides.
        let recs = [
            "{a: int, ...}",
            "{b?: string, ...}",
            "map<any>",
            "{}",
            "{a: int}",
        ];
        for a in recs {
            assert_eq!(m(a, a), print(&ty(a)), "idempotent {a}");
            for b in recs {
                let met = pick(&[ty(a), ty(b)]);
                assert!(fits(&met, &ty(a)).is_some(), "{a} meet {b} fits {a}");
                assert!(fits(&met, &ty(b)).is_some(), "{a} meet {b} fits {b}");
            }
        }
    }

    #[test]
    fn marks_pass_through_meet_and_join_and_close_is_deep() {
        let open = |closable: bool, k: &str| {
            let mut fields = BTreeMap::new();
            fields.insert(
                k.to_string(),
                Field {
                    ty: Ty::int(),
                    optional: true,
                },
            );
            Rec {
                fields,
                rest: Ty::any(),
                closable,
            }
        };
        let both = meet(&open(true, "a"), &open(true, "b"));
        assert!(both.closable);
        assert!(!meet(&open(true, "a"), &open(false, "b")).closable);
        let j = join(&Ty::rec(open(true, "a")), &Ty::rec(open(true, "b")));
        assert!(j.record().unwrap().closable);
        let nested = Ty::list(Ty::rec(Rec {
            fields: [(
                "p".to_string(),
                Field {
                    ty: Ty::rec(both),
                    optional: false,
                },
            )]
            .into(),
            rest: Ty::any(),
            closable: true,
        }));
        assert_eq!(print(&close(&nested)), "{p: {a?: int, b?: int}}[]");
    }

    #[test]
    fn shared_ref_targets_map_once() {
        // A doubling chain: each definition names the next twice.
        let n = 30;
        let mut defs = Vec::new();
        for i in 0..n {
            defs.push(format!(
                "\"d{i}\": {{\"type\": \"object\", \"properties\": {{\"a\": {{\"$ref\": \"#/$defs/d{j}\"}}, \"b\": {{\"$ref\": \"#/$defs/d{j}\"}}}}}}",
                j = i + 1
            ));
        }
        defs.push(format!("\"d{n}\": {{\"type\": \"integer\"}}"));
        let schema = format!(
            "{{\"$defs\": {{{}}}, \"$ref\": \"#/$defs/d0\"}}",
            defs.join(", ")
        );
        let j = parse(&schema).unwrap();
        let start = std::time::Instant::now();
        for closed in [false, true] {
            let opts = SchemaOptions {
                closed,
                ..SchemaOptions::default()
            };
            let t = ty_schema(&j, opts).unwrap();
            assert_eq!(type_depth(&t, &mut HashMap::new()), n);
        }
        assert!(start.elapsed().as_secs() < 5);
    }

    #[test]
    fn examples_of_the_chapter() {
        let cases: &[(&str, &[&str], &str)] = &[
            (
                r#"{"type": "object", "properties": {"a": {"type": "string"}}}"#,
                &[],
                "{a?: string, ...}",
            ),
            (r#"{"type": "number", "multipleOf": 1}"#, &[], "float"),
            (r#"{"type": ["integer", "number"]}"#, &[], "float"),
            (
                r#"{"type": ["null", "boolean", "integer", "number", "string", "array", "object"]}"#,
                &[],
                "bool | float | string | any[] | map<any> | null",
            ),
            (
                r#"{"enum": [2.0, 9007199254740992, "a"]}"#,
                &[],
                "float | \"a\"",
            ),
            (
                r#"{"type": "integer", "enum": [1e20, 1.5, "x"]}"#,
                &[],
                "float",
            ),
            (r#"{"allOf": [{"type": "string"}, false]}"#, &[], "never"),
            (r#"{"type": "array", "items": false}"#, &[], "never[]"),
            (
                r#"{"type": "object", "required": ["id"], "additionalProperties": false}"#,
                &[],
                "{id: never}",
            ),
            (
                r#"{"type": "object", "patternProperties": {"^x": {"type": "integer"}}, "additionalProperties": {"type": "string"}}"#,
                &[],
                "map<int | string>",
            ),
            (
                r#"{"anyOf": [{"type": "object", "properties": {"kind": {"const": "circle"}}, "required": ["kind"]}, {"type": "object", "properties": {"kind": {"const": "rect"}}, "required": ["kind"]}]}"#,
                &[],
                "error",
            ),
            (
                r#"{"type": "object", "properties": {"a": false}}"#,
                &["absent-is-null"],
                "{a: null, ...}",
            ),
        ];
        for (s, o, want) in cases {
            let got = map(s, o);
            if *want == "error" {
                assert!(got.starts_with("error"), "{s}: {got}");
            } else {
                assert_eq!(got, *want, "{s}");
            }
        }
    }

    #[test]
    fn pointers() {
        assert_eq!(percent_decode("/a%20b").as_deref(), Some("/a b"));
        assert_eq!(percent_decode("/%zz"), None);
        assert_eq!(percent_decode("/%ff"), None);
        assert_eq!(unescape("a~1b~0c").as_deref(), Some("a/b~c"));
        assert_eq!(unescape("a~2"), None);
        assert_eq!(unescape("a~"), None);
        assert_eq!(index("0"), Some(0));
        assert_eq!(index("01"), None);
        assert_eq!(index("12"), Some(12));
    }
}
