//! The environment (`03-host.md` 3.3): declarations, and the errors of
//! phase `declare`.

use crate::diag::{code, Diagnostic, Error, Phase};
use crate::eval::Call;
use crate::host::{Data, HostError};
use crate::library;
use crate::syntax::token::is_reserved;
use crate::types::text::{parse_sig, parse_type, Rest, Sig, Type};
use crate::value::Value;
use std::sync::Arc;
use std::time::Duration;

/// The implementation of a host function or method. A method receives its
/// receiver as the first value.
pub type HostImpl =
    Arc<dyn Fn(&mut Call<'_>, &[Value<'_>]) -> Result<Data, HostError> + Send + Sync>;

#[derive(Clone)]
pub struct HostFn {
    /// The full name, `geo.distance` or `rate`.
    pub name: String,
    pub namespace: Option<String>,
    pub base: String,
    pub sig: Sig,
    pub imp: Option<HostImpl>,
}

#[derive(Clone)]
pub struct HostMethod {
    pub type_name: String,
    pub name: String,
    pub sig: Sig,
    pub imp: Option<HostImpl>,
}

/// The limits of a run (`03-host.md` 3.6.1). `None` turns a limit off.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Limits {
    pub steps: Option<u64>,
    pub allocation: Option<u64>,
    /// From 1 to 1,000.
    pub depth: u32,
    pub deadline: Option<Duration>,
}

impl Default for Limits {
    fn default() -> Limits {
        Limits {
            steps: Some(1_000_000),
            allocation: Some(1_000_000),
            depth: 200,
            deadline: None,
        }
    }
}

pub struct EnvData {
    pub vars: Vec<String>,
    pub functions: Vec<HostFn>,
    pub opaque: Vec<String>,
    pub methods: Vec<HostMethod>,
    pub limits: Limits,
    pub reject: Vec<String>,
}

/// A built environment. It does not change, and is cheap to clone and safe
/// to share between threads.
#[derive(Clone)]
pub struct Env {
    pub(crate) data: Arc<EnvData>,
}

impl Default for Env {
    fn default() -> Env {
        Env::builder()
            .build()
            .expect("the default environment builds")
    }
}

impl Env {
    pub fn builder() -> EnvBuilder {
        EnvBuilder::default()
    }

    pub fn data(&self) -> &EnvData {
        &self.data
    }

    pub fn function(&self, name: &str) -> Option<usize> {
        self.data.functions.iter().position(|f| f.name == name)
    }

    pub fn is_namespace(&self, name: &str) -> bool {
        self.data
            .functions
            .iter()
            .any(|f| f.namespace.as_deref() == Some(name))
    }

    pub fn method(&self, type_name: &str, name: &str) -> Option<usize> {
        self.data
            .methods
            .iter()
            .position(|m| m.type_name == type_name && m.name == name)
    }

    pub fn var(&self, name: &str) -> Option<usize> {
        self.data.vars.iter().position(|v| v == name)
    }
}

/// The six check classes (`05-types.md` 17).
pub const CHECK_CLASSES: [&str; 6] = [
    "constant-condition",
    "dynamic",
    "index",
    "divide",
    "value",
    "host",
];

/// A `Name` of `01-language.md` 2.4: a word that is not reserved and not
/// `_`.
pub fn is_name(s: &str) -> bool {
    let b = s.as_bytes();
    !b.is_empty()
        && (b[0].is_ascii_alphabetic() || b[0] == b'_')
        && b.iter().all(|c| c.is_ascii_alphanumeric() || *c == b'_')
        && crate::syntax::token::is_name(s)
}

const RESERVED_NAMESPACES: [&str; 4] = ["json", "time", "math", "base64"];

#[derive(Default)]
pub struct EnvBuilder {
    version: Option<String>,
    vars: Option<String>,
    functions: Vec<(String, String, Option<HostImpl>)>,
    opaque: Vec<String>,
    methods: Vec<(String, String, Option<HostImpl>)>,
    reject: Option<Vec<String>>,
    limits: Limits,
}

impl EnvBuilder {
    pub fn version(mut self, v: &str) -> Self {
        self.version = Some(v.to_string());
        self
    }

    /// The variables, as one record type: `{input: {...}, limit: int}`.
    pub fn vars(mut self, record: &str) -> Self {
        self.vars = Some(record.to_string());
        self
    }

    pub fn function(
        mut self,
        name: &str,
        sig: &str,
        imp: impl Fn(&mut Call<'_>, &[Value<'_>]) -> Result<Data, HostError> + Send + Sync + 'static,
    ) -> Self {
        self.functions
            .push((name.to_string(), sig.to_string(), Some(Arc::new(imp))));
        self
    }

    /// A function with a signature and no implementation: compile fails with
    /// `no-implementation`.
    pub fn declare_function(mut self, name: &str, sig: &str) -> Self {
        self.functions
            .push((name.to_string(), sig.to_string(), None));
        self
    }

    pub fn function_impl(mut self, name: &str, sig: &str, imp: HostImpl) -> Self {
        self.functions
            .push((name.to_string(), sig.to_string(), Some(imp)));
        self
    }

    pub fn opaque(mut self, name: &str) -> Self {
        self.opaque.push(name.to_string());
        self
    }

    /// A method of an opaque type: `"Token.owner"`, its signature without the
    /// receiver, and its implementation.
    pub fn method_impl(mut self, key: &str, sig: &str, imp: Option<HostImpl>) -> Self {
        self.methods.push((key.to_string(), sig.to_string(), imp));
        self
    }

    pub fn reject(mut self, classes: &[&str]) -> Self {
        self.reject = Some(classes.iter().map(|c| c.to_string()).collect());
        self
    }

    pub fn limits(mut self, l: Limits) -> Self {
        self.limits = l;
        self
    }

    pub fn build(self) -> Result<Env, Error> {
        let mut errs: Vec<Diagnostic> = Vec::new();
        let mut bad =
            |c: &'static str, msg: String| errs.push(Diagnostic::bare(Phase::Declare, c, msg));
        if let Some(v) = &self.version {
            if v != "3.0" {
                bad(
                    code::UNSUPPORTED_VERSION,
                    format!("version {v} is not offered"),
                );
            }
        }
        let mut vars: Vec<String> = Vec::new();
        if let Some(text) = &self.vars {
            match parse_type(text) {
                // Ruling AG4: parentheses do not make a record.
                Ok(Type::Record(r)) if text.trim_start().starts_with('{') => {
                    if r.rest != Rest::Exact || r.fields.iter().any(|f| f.optional) {
                        bad(
                            code::BAD_TYPE,
                            "the record of an environment has no optional field and no rest".into(),
                        );
                    }
                    for f in r.fields {
                        if !is_name(&f.key) {
                            bad(
                                code::BAD_NAME,
                                format!("{:?} cannot name a variable", f.key),
                            );
                        } else if vars.contains(&f.key) {
                            bad(
                                code::NAME_CLASH,
                                format!("the variable {} is declared twice", f.key),
                            );
                        }
                        vars.push(f.key);
                    }
                }
                Ok(_) => bad(code::BAD_TYPE, "the variables are not a record type".into()),
                Err(e) => bad(code::BAD_TYPE, format!("variables: {e}")),
            }
        }
        let mut opaque: Vec<String> = Vec::new();
        for name in &self.opaque {
            let b = name.as_bytes();
            let ok = b.len() >= 2
                && b[0].is_ascii_uppercase()
                && b[1..]
                    .iter()
                    .all(|c| c.is_ascii_alphanumeric() || *c == b'_');
            if !ok {
                bad(
                    code::BAD_NAME,
                    format!("{name:?} cannot name an opaque type"),
                );
            } else if opaque.contains(name) {
                bad(
                    code::NAME_CLASH,
                    format!("the opaque type {name} is declared twice"),
                );
            }
            opaque.push(name.clone());
        }
        let mut methods = Vec::new();
        for (key, text, imp) in self.methods {
            let (t, m) = key.split_once('.').unwrap_or((&key, ""));
            match parse_sig(&text) {
                Ok(s) if s.has_fn_param() => bad(
                    code::BAD_SIGNATURE,
                    format!("the method {key} takes a function"),
                ),
                Ok(sig) => methods.push(HostMethod {
                    type_name: t.to_string(),
                    name: m.to_string(),
                    sig,
                    imp,
                }),
                Err(e) => bad(code::BAD_SIGNATURE, format!("the method {key}: {e}")),
            }
        }
        let mut functions: Vec<HostFn> = Vec::new();
        for (name, text, imp) in self.functions {
            let parts: Vec<&str> = name.split('.').collect();
            let (ns, base) = match parts.as_slice() {
                [b] => (None, *b),
                [n, b] => (Some(*n), *b),
                _ => (None, parts.last().copied().unwrap_or_default()),
            };
            if parts.len() > 2 || parts.iter().any(|p| !is_name(p)) {
                bad(code::BAD_NAME, format!("{name:?} cannot name a function"));
            }
            if let Some(n) = ns {
                if RESERVED_NAMESPACES.contains(&n) {
                    bad(
                        code::RESERVED_NAMESPACE,
                        format!("the namespace {n} is reserved"),
                    );
                }
            }
            if ns.is_none() && library::function(base).is_some() {
                bad(
                    code::NAME_CLASH,
                    format!("{name} is a function of the library"),
                );
            }
            if functions.iter().any(|f| f.name == name) {
                bad(
                    code::NAME_CLASH,
                    format!("the function {name} is declared twice"),
                );
            }
            if vars.contains(&name) {
                bad(
                    code::NAME_CLASH,
                    format!("{name} is a variable and a function"),
                );
            }
            let sig = match parse_sig(&text) {
                Ok(s) if s.has_fn_param() => {
                    bad(
                        code::BAD_SIGNATURE,
                        format!("the function {name} takes a function"),
                    );
                    None
                }
                Ok(s) => Some(s),
                Err(e) => {
                    bad(code::BAD_SIGNATURE, format!("the function {name}: {e}"));
                    None
                }
            };
            if let Some(sig) = sig {
                functions.push(HostFn {
                    namespace: ns.map(String::from),
                    base: base.to_string(),
                    name,
                    sig,
                    imp,
                });
            }
        }
        for f in &functions {
            if let Some(n) = &f.namespace {
                if vars.contains(n) {
                    bad(
                        code::NAME_CLASH,
                        format!("the variable {n} is named like a namespace"),
                    );
                }
                if functions
                    .iter()
                    .any(|g| g.namespace.is_none() && g.name == *n)
                {
                    bad(
                        code::NAME_CLASH,
                        format!("the function {n} is named like a namespace"),
                    );
                }
            }
        }
        let reject = match self.reject {
            Some(classes) => {
                for c in &classes {
                    if !CHECK_CLASSES.contains(&c.as_str()) {
                        bad(code::BAD_CLASS, format!("{c:?} is not a check class"));
                    }
                }
                classes
            }
            None => vec!["constant-condition".to_string()],
        };
        if !errs.is_empty() {
            return Err(Error {
                phase: Phase::Declare,
                diagnostics: errs,
            });
        }
        Ok(Env {
            data: Arc::new(EnvData {
                vars,
                functions,
                opaque,
                methods,
                limits: self.limits,
                reject,
            }),
        })
    }
}

/// The expected result of a check or a compile (`03-host.md` 3.2.3): its
/// text must be `condition` or a type.
pub fn check_expected(text: &str) -> Result<(), Diagnostic> {
    if text == "condition" {
        return Ok(());
    }
    parse_type(text).map(|_| ()).map_err(|e| {
        Diagnostic::bare(
            Phase::Declare,
            code::BAD_TYPE,
            format!("expected result: {e}"),
        )
    })
}

/// Whether a word is reserved, for hosts that name things.
pub fn reserved(word: &str) -> bool {
    is_reserved(word)
}
