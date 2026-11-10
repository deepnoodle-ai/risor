//! Package F1: the description of a type, a signature and an environment,
//! and an environment built from a description
//! (`spec/packages/f01-host-types.md` 5).

use crate::check::ty::{print, sig_type, Ty};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::env::{Env, Limits};
use crate::json::{self, Json};
use crate::types::text::{Bound, ParamType, Sig};
use std::fmt;

/// The description of a type: its printed text (5.1).
pub fn describe_type(t: &Ty) -> String {
    print(t)
}

/// A signature printed (5.2). `opaque` is the environment's opaque types.
pub fn print_sig(sig: &Sig, opaque: &[String]) -> String {
    let ty = |t| print(&sig_type(t, opaque));
    let params: Vec<String> = sig
        .params
        .iter()
        .map(|p| {
            let t = match &p.ty {
                ParamType::Type(t) => ty(t),
                // A host signature has no parameter of a function type
                // (`03-host.md` 3.3.4); a library one is printed for
                // completeness.
                ParamType::Fn(f) => {
                    let ps: Vec<String> = f
                        .params
                        .iter()
                        .map(|(n, t)| format!("{n}: {}", ty(t)))
                        .collect();
                    match &f.result {
                        Some(r) => format!("({}) => {}", ps.join(", "), ty(r)),
                        None => format!("({}) => any", ps.join(", ")),
                    }
                }
            };
            format!(
                "{}{}{}: {t}",
                if p.rest { "..." } else { "" },
                p.name,
                if p.optional { "?" } else { "" }
            )
        })
        .collect();
    let mut out = format!("({}) => {}", params.join(", "), ty(&sig.result));
    if sig.fails {
        out.push_str(" !");
    }
    if !sig.bounds.is_empty() {
        let mut bounds = sig.bounds.clone();
        bounds.sort_by_key(|b| b.0);
        let bs: Vec<String> = bounds
            .iter()
            .map(|(v, b)| {
                let name = match b {
                    Bound::Ordered => "ordered",
                    Bound::Number => "number",
                };
                format!("{v}: {name}")
            })
            .collect();
        out.push_str(" where ");
        out.push_str(&bs.join(", "));
    }
    out
}

fn obj(mut entries: Vec<(String, Json)>) -> Json {
    entries.sort_by(|a, b| a.0.cmp(&b.0));
    Json::Obj(entries)
}

fn limit(n: Option<u64>) -> Json {
    match n {
        Some(n) => Json::Num(n as f64),
        None => Json::Null,
    }
}

impl Env {
    /// The description of the environment (5.3), with the keys of every
    /// object in code point order.
    pub fn describe(&self) -> Json {
        let d = self.data();
        let variables = d
            .vars
            .iter()
            .zip(&d.var_types)
            .map(|(n, t)| (n.clone(), Json::Str(print(t))))
            .collect();
        let functions = d
            .functions
            .iter()
            .map(|f| (f.name.clone(), Json::Str(print_sig(&f.sig, &d.opaque))))
            .collect();
        let opaque = d
            .opaque
            .iter()
            .map(|o| {
                let methods = d
                    .methods
                    .iter()
                    .filter(|m| m.type_name == *o)
                    .map(|m| (m.name.clone(), Json::Str(print_sig(&m.sig, &d.opaque))))
                    .collect();
                (o.clone(), obj(methods))
            })
            .collect();
        let mut reject = d.reject.clone();
        reject.sort();
        reject.dedup();
        obj(vec![
            ("version".into(), Json::Str("3.0".into())),
            ("variables".into(), obj(variables)),
            ("functions".into(), obj(functions)),
            ("opaque".into(), obj(opaque)),
            ("removed".into(), Json::Arr(Vec::new())),
            (
                "reject".into(),
                Json::Arr(reject.into_iter().map(Json::Str).collect()),
            ),
            (
                "limits".into(),
                obj(vec![
                    ("steps".into(), limit(d.limits.steps)),
                    ("allocation".into(), limit(d.limits.allocation)),
                    ("depth".into(), Json::Num(d.limits.depth as f64)),
                ]),
            ),
        ])
    }

    /// The text of the description (5.3 item 5): its JSON form, which two
    /// implementations give alike for one environment.
    pub fn description_text(&self) -> String {
        json::to_text(&self.describe())
    }

    /// An environment built from a description (5.4): for check only. Each
    /// type text is read on its own. Its functions and methods have no
    /// implementation, so a compile against it fails with
    /// `no-implementation` while it declares one.
    pub fn from_description(d: &Json) -> Result<Env, DescriptionError> {
        let form = |m: &str| DescriptionError::Form(m.to_string());
        let o = d
            .as_obj()
            .ok_or_else(|| form("a description is a JSON object"))?;
        const KEYS: [&str; 7] = [
            "version",
            "variables",
            "functions",
            "opaque",
            "removed",
            "reject",
            "limits",
        ];
        for (k, _) in o {
            if !KEYS.contains(&k.as_str()) {
                return Err(form(&format!("a description has no key {k:?}")));
            }
        }
        let get = |k: &str| {
            d.get(k)
                .ok_or_else(|| form(&format!("the key {k} is missing")))
        };
        let strings_of =
            |v: &Json, what: &str| -> Result<Vec<(String, String)>, DescriptionError> {
                let m = v
                    .as_obj()
                    .ok_or_else(|| form(&format!("{what} is not an object")))?;
                m.iter()
                    .map(|(k, x)| {
                        x.as_str()
                            .map(|s| (k.clone(), s.to_string()))
                            .ok_or_else(|| form(&format!("{what}.{k} is not a string")))
                    })
                    .collect()
            };
        let list_of = |v: &Json, what: &str| -> Result<Vec<String>, DescriptionError> {
            let a = v
                .as_arr()
                .ok_or_else(|| form(&format!("{what} is not an array")))?;
            a.iter()
                .map(|x| {
                    x.as_str()
                        .map(String::from)
                        .ok_or_else(|| form(&format!("{what} holds a value that is not a string")))
                })
                .collect()
        };
        let mut b = Env::builder();
        let version = get("version")?
            .as_str()
            .ok_or_else(|| form("version is not a string"))?;
        b = b.version(version);
        for (name, text) in strings_of(get("variables")?, "variables")? {
            b = b.var(&name, &text);
        }
        for (name, text) in strings_of(get("functions")?, "functions")? {
            b = b.declare_function(&name, &text);
        }
        let opaque = get("opaque")?
            .as_obj()
            .ok_or_else(|| form("opaque is not an object"))?;
        for (t, methods) in opaque {
            b = b.opaque(t);
            for (m, text) in strings_of(methods, &format!("opaque.{t}"))? {
                b = b.method_impl(&format!("{t}.{m}"), &text, None);
            }
        }
        let removed = list_of(get("removed")?, "removed")?;
        if let Some(r) = removed.first() {
            return Err(form(&format!(
                "removed names {r:?}: this build does not offer removing a library entry"
            )));
        }
        let reject = list_of(get("reject")?, "reject")?;
        let reject: Vec<&str> = reject.iter().map(String::as_str).collect();
        b = b.reject(&reject);
        // Limits (BB24): a negative or fractional number is `bad-limit`; a
        // value of another JSON kind, or a missing key, is a form error.
        let l = get("limits")?;
        let lo = l.as_obj().ok_or_else(|| form("limits is not an object"))?;
        for (k, _) in lo {
            if !["steps", "allocation", "depth"].contains(&k.as_str()) {
                return Err(form(&format!("limits has no key {k:?}")));
            }
        }
        let mut bad: Vec<Diagnostic> = Vec::new();
        let mut count = |k: &str, nullable: bool| -> Result<Option<f64>, DescriptionError> {
            match l.get(k) {
                None => Err(form(&format!("limits.{k} is missing"))),
                Some(Json::Null) if nullable => Ok(None),
                Some(Json::Num(n)) => {
                    let ok = n.fract() == 0.0 && *n >= 0.0;
                    if !ok {
                        bad.push(Diagnostic::bare(
                            Phase::Declare,
                            code::BAD_LIMIT,
                            format!("the {k} limit {n} is not a whole number of 0 or more"),
                        ));
                    }
                    Ok(if ok { Some(*n) } else { None })
                }
                Some(_) => Err(form(&format!("limits.{k} is not a number"))),
            }
        };
        let defaults = Limits::default();
        let steps = count("steps", true)?;
        let allocation = count("allocation", true)?;
        let depth = count("depth", false)?;
        let null_or = |k: &str| matches!(l.get(k), Some(Json::Null));
        let limits = Limits {
            steps: match steps {
                Some(n) => Some(n as u64),
                None if null_or("steps") => None,
                None => defaults.steps,
            },
            allocation: match allocation {
                Some(n) => Some(n as u64),
                None if null_or("allocation") => None,
                None => defaults.allocation,
            },
            // A depth outside 1 to 1,000 is reported by the builder.
            depth: match depth {
                Some(n) if n <= 1000.0 => n as u32,
                Some(_) => 0,
                None => defaults.depth,
            },
            deadline: None,
        };
        b = b.limits(limits);
        match b.build() {
            Ok(env) if bad.is_empty() => Ok(env),
            Ok(_) => Err(DescriptionError::Declare(Error {
                phase: Phase::Declare,
                diagnostics: bad,
            })),
            Err(mut e) => {
                e.diagnostics.extend(bad);
                Err(DescriptionError::Declare(e))
            }
        }
    }

    /// As `from_description`, from the description's JSON text.
    pub fn from_description_text(text: &str) -> Result<Env, DescriptionError> {
        let d = json::parse(text).map_err(DescriptionError::Form)?;
        Env::from_description(&d)
    }
}

/// Why an environment could not be built from a description (5.4 item 2).
#[derive(Debug)]
pub enum DescriptionError {
    /// The description is not a JSON object of the form of 5.3: the
    /// binding's own error.
    Form(String),
    /// What it declares has `declare` errors (`03-host.md` 3.3.4).
    Declare(Error),
}

impl fmt::Display for DescriptionError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            DescriptionError::Form(m) => write!(f, "not a description: {m}"),
            DescriptionError::Declare(e) => {
                let codes: Vec<String> = e
                    .diagnostics
                    .iter()
                    .map(|d| format!("{}: {}", d.code, d.message))
                    .collect();
                write!(f, "declare errors: {}", codes.join("; "))
            }
        }
    }
}

impl std::error::Error for DescriptionError {}
