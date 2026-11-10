//! The environment (`03-host.md` 3.3): declarations, and the errors of
//! phase `declare`.

use crate::check::ty::{from_text, Ty};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::eval::Call;
use crate::host::{Data, HostError};
use crate::hosttype::{opaque_names, type_of, HostType};
use crate::json::Json;
use crate::library;
use crate::schema::{ty_schema, SchemaOptions};
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
    /// The type of each variable, as the checker reads it.
    pub var_types: Vec<crate::check::ty::Ty>,
    pub functions: Vec<HostFn>,
    pub opaque: Vec<String>,
    pub methods: Vec<HostMethod>,
    pub limits: Limits,
    pub reject: Vec<String>,
    /// The removed library entries (`03-host.md` 3.3.1 item 6), as indices
    /// of `library::records()`, in order and each once.
    pub removed: Vec<usize>,
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

    /// The library function of a name that the environment has: not
    /// removed.
    pub fn lib_function(&self, name: &str) -> Option<usize> {
        library::function(name).filter(|i| !self.data.removed.contains(i))
    }

    /// The library method of a kind and a name that the environment has.
    pub fn lib_method(&self, kind: crate::value::Kind, name: &str) -> Option<usize> {
        library::method(kind, name).filter(|i| !self.data.removed.contains(i))
    }

    /// The names of the removed entries, as 3.3.1 writes them, in code point
    /// order.
    pub fn removed_names(&self) -> Vec<String> {
        let mut out: Vec<String> = self
            .data
            .removed
            .iter()
            .map(|&i| library::entry_name(i))
            .collect();
        out.sort();
        out
    }

    /// A child environment (`03-host.md` 3.3.2): its declarations are added
    /// to this one's, and a name it declares replaces this one's, in the
    /// child only. It may remove further entries and cannot bring one back.
    /// Its rejected classes and limits, when given, apply in it only.
    pub fn child(&self) -> EnvBuilder {
        EnvBuilder {
            parent: Some(self.clone()),
            ..EnvBuilder::default()
        }
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

/// A variable declared alone: in type text, or by a JSON Schema (F1).
#[derive(Clone)]
enum VarDecl {
    Text(String),
    Schema(Json, SchemaOptions),
    /// A type derived from a host type (F1 6), or why it could not be.
    Host(Result<Ty, String>),
}

#[derive(Default)]
pub struct EnvBuilder {
    version: Option<String>,
    vars: Option<String>,
    one_vars: Vec<(String, VarDecl)>,
    functions: Vec<(String, String, Option<HostImpl>)>,
    opaque: Vec<String>,
    methods: Vec<(String, String, Option<HostImpl>)>,
    reject: Option<Vec<String>>,
    limits: Option<Limits>,
    removed: Vec<String>,
    parent: Option<Env>,
}

impl EnvBuilder {
    /// A library entry the environment does not have (`03-host.md` 3.3.1):
    /// a function by its name, `len`; a method by its receiver's kind, a dot
    /// and its name, `string.repeat`, `list.map`, `map.keys`. A name that is
    /// no entry is the `declare` error `bad-name`.
    pub fn remove(mut self, entry: &str) -> Self {
        self.removed.push(entry.to_string());
        self
    }

    pub fn version(mut self, v: &str) -> Self {
        self.version = Some(v.to_string());
        self
    }

    /// The variables, as one record type: `{input: {...}, limit: int}`.
    pub fn vars(mut self, record: &str) -> Self {
        self.vars = Some(record.to_string());
        self
    }

    /// One variable and its type text, read on its own as declared type
    /// text (`05-types.md` 19.5), not inside the record of `vars`: its type
    /// may be 64 deep.
    pub fn var(mut self, name: &str, ty: &str) -> Self {
        self.one_vars
            .push((name.to_string(), VarDecl::Text(ty.to_string())));
        self
    }

    /// A variable declared by a JSON Schema, with the options given (F1,
    /// `f01-host-types.md` 2). Its type is M(S); the schema is not kept. An
    /// error in the schema is the `declare` error `bad-type`.
    pub fn var_schema(mut self, name: &str, schema: &Json, opts: SchemaOptions) -> Self {
        self.one_vars
            .push((name.to_string(), VarDecl::Schema(schema.clone(), opts)));
        self
    }

    /// A variable whose type is derived from the host type `T` (F1 6). A
    /// type that cannot be derived, or that names an opaque type the
    /// environment does not declare, is the `declare` error `bad-type`.
    pub fn var_host<T: HostType + ?Sized>(mut self, name: &str) -> Self {
        let t = type_of::<T>().map_err(|e| e.0);
        self.one_vars.push((name.to_string(), VarDecl::Host(t)));
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
        self.limits = Some(l);
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
        let parent = self.parent.as_ref().map(|p| p.data.clone());
        let mut removed: Vec<usize> = parent
            .as_ref()
            .map(|p| p.removed.clone())
            .unwrap_or_default();
        for name in &self.removed {
            match library::entry_of(name) {
                Some(i) => removed.push(i),
                None => bad(
                    code::BAD_NAME,
                    format!("{name:?} names no entry of the library"),
                ),
            }
        }
        removed.sort_unstable();
        removed.dedup();
        let mut opaque: Vec<String> = parent
            .as_ref()
            .map(|p| p.opaque.clone())
            .unwrap_or_default();
        let inherited_opaque = opaque.len();
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
            } else if opaque[inherited_opaque..].contains(name) {
                bad(
                    code::NAME_CLASH,
                    format!("the opaque type {name} is declared twice"),
                );
            }
            // A child's opaque type of a parent's name replaces it.
            if !opaque[..inherited_opaque].contains(name) {
                opaque.push(name.clone());
            }
        }
        let mut vars: Vec<String> = Vec::new();
        let mut var_types = Vec::new();
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
                        // 19.5, the rules beyond the grammar.
                        match from_text(&f.ty, &opaque, None) {
                            Ok(t) => var_types.push(t),
                            Err(e) => {
                                bad(code::BAD_TYPE, format!("the variable {}: {e}", f.key));
                                var_types.push(Ty::any());
                            }
                        }
                        vars.push(f.key);
                    }
                }
                Ok(_) => bad(code::BAD_TYPE, "the variables are not a record type".into()),
                Err(e) => bad(code::BAD_TYPE, format!("variables: {e}")),
            }
        }
        for (name, decl) in &self.one_vars {
            if !is_name(name) {
                bad(code::BAD_NAME, format!("{name:?} cannot name a variable"));
            } else if vars.contains(name) {
                bad(
                    code::NAME_CLASH,
                    format!("the variable {name} is declared twice"),
                );
            }
            let ty = match decl {
                VarDecl::Text(text) => match parse_type(text) {
                    Ok(t) => from_text(&t, &opaque, None)
                        .map_err(|e| format!("the variable {name}: {e}")),
                    Err(e) => Err(format!("the variable {name}: {e}")),
                },
                VarDecl::Schema(schema, opts) => {
                    ty_schema(schema, *opts).map_err(|e| format!("the variable {name}: {e}"))
                }
                VarDecl::Host(t) => match t {
                    Ok(t) => {
                        let mut names = std::collections::BTreeSet::new();
                        opaque_names(t, &mut names, &mut Default::default());
                        match names.iter().find(|n| !opaque.contains(n)) {
                            Some(n) => Err(format!(
                                "the variable {name}: {n} is not a declared opaque type"
                            )),
                            None => Ok(t.clone()),
                        }
                    }
                    Err(e) => Err(format!("the variable {name}: {e}")),
                },
            };
            match ty {
                Ok(t) => var_types.push(t),
                Err(e) => {
                    bad(code::BAD_TYPE, e);
                    var_types.push(Ty::any());
                }
            }
            vars.push(name.clone());
        }
        let mut methods = Vec::new();
        for (key, text, imp) in self.methods {
            let (t, m) = key.split_once('.').unwrap_or((&key, ""));
            if !opaque.iter().any(|o| o == t) {
                bad(
                    code::BAD_SIGNATURE,
                    format!("the receiver of {key} is not a declared opaque type"),
                );
            }
            match parse_sig(&text) {
                Ok(s) if s.has_fn_param() => bad(
                    code::BAD_SIGNATURE,
                    format!("the method {key} takes a function"),
                ),
                Ok(sig) if !sig_types(&sig, &opaque, &mut bad) => {}
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
            // A removed function is not one the environment has (3.3.1
            // item 6).
            if ns.is_none() && library::function(base).is_some_and(|i| !removed.contains(&i)) {
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
                Ok(s) if sig_types(&s, &opaque, &mut bad) => Some(s),
                Ok(_) => None,
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
        let limits = self
            .limits
            .or(parent.as_ref().map(|p| p.limits))
            .unwrap_or_default();
        // Ruling AM4. Steps and units are unsigned, so only the depth can
        // be out of range.
        if !(1..=1000).contains(&limits.depth) {
            bad(
                code::BAD_LIMIT,
                format!("the depth limit {} is outside 1 to 1,000", limits.depth),
            );
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
            None => match &parent {
                Some(p) => p.reject.clone(),
                None => vec!["constant-condition".to_string()],
            },
        };
        // A child's declarations replace its parent's of the same name
        // (3.3.2): the parent's others are kept.
        if let Some(p) = &parent {
            let taken =
                |n: &str| vars.iter().any(|v| v == n) || functions.iter().any(|f| f.name == n);
            let mut pv: Vec<String> = Vec::new();
            let mut pt = Vec::new();
            for (n, t) in p.vars.iter().zip(&p.var_types) {
                if !taken(n) {
                    pv.push(n.clone());
                    pt.push(t.clone());
                }
            }
            let pf: Vec<HostFn> = p
                .functions
                .iter()
                .filter(|f| !taken(&f.name))
                .cloned()
                .collect();
            pv.append(&mut vars);
            pt.append(&mut var_types);
            vars = pv;
            var_types = pt;
            let mut pf = pf;
            pf.append(&mut functions);
            functions = pf;
            let mut pm: Vec<HostMethod> = p
                .methods
                .iter()
                // Ruling CH4: a child's opaque type replaces the parent's
                // whole, methods included; a child's method alone adds or
                // replaces that one method.
                .filter(|m| {
                    !self.opaque.contains(&m.type_name)
                        && !methods
                            .iter()
                            .any(|c: &HostMethod| c.type_name == m.type_name && c.name == m.name)
                })
                .cloned()
                .collect();
            pm.append(&mut methods);
            methods = pm;
        }
        // Ruling CH2: the clash rules apply to the whole environment, after
        // a child's replacements.
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
        if !errs.is_empty() {
            return Err(Error {
                phase: Phase::Declare,
                diagnostics: errs,
            });
        }
        Ok(Env {
            data: Arc::new(EnvData {
                vars,
                var_types,
                functions,
                opaque,
                methods,
                limits,
                reject,
                removed,
            }),
        })
    }
}

/// The rules of 19.5 beyond the grammar, for a signature: every type names
/// declared opaque types only and is at most 64 deep (`bad-type`), and a rest
/// parameter is a list (`bad-signature`). Returns whether it holds.
fn sig_types(sig: &Sig, opaque: &[String], bad: &mut impl FnMut(&'static str, String)) -> bool {
    use crate::types::text::ParamType;
    let none = crate::check::ty::Solutions::new();
    let mut ok = true;
    let check = |t: &Type, ok: &mut bool, bad: &mut dyn FnMut(&'static str, String)| {
        // Ruling AO4: every error in a signature is `bad-signature`.
        if let Err(e) = from_text(t, opaque, Some(&none)) {
            bad(code::BAD_SIGNATURE, e);
            *ok = false;
        }
    };
    for p in &sig.params {
        match &p.ty {
            ParamType::Type(t) => {
                check(t, &mut ok, bad);
                if p.rest && !matches!(t, Type::List(_)) {
                    bad(
                        code::BAD_SIGNATURE,
                        format!("the rest parameter {} is not a list", p.name),
                    );
                    ok = false;
                }
            }
            ParamType::Fn(f) => {
                for (_, t) in &f.params {
                    check(t, &mut ok, bad);
                }
                if let Some(r) = &f.result {
                    check(r, &mut ok, bad);
                }
            }
        }
    }
    check(&sig.result, &mut ok, bad);
    ok
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
