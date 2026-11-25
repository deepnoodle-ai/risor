//! The public type (`05-host-api.md` E.7, ruling R179): a value that a host
//! builds, reads, joins and tests, and that a check gives as a program's
//! type.
//!
//! A [`Type`] wraps the checker's own type, so a type a host builds is the
//! type the checker reads, and a check result's type is the checker's. Its
//! text form ([`std::fmt::Display`]) is the printing of `05-types.md` 3.9.
//!
//! A constructor never fails. A construction that has no type, or that
//! passes a limit of `05-types.md` 20.4 or 20.7, gives an invalid type, and
//! [`Type::err`] reports it.

use crate::check::ty::{self as cty, Num, Rec, Strs, Ty};
use crate::diag::{code, Diagnostic, Error, Phase};
use crate::schema::SchemaOptions;
use crate::value::Value;
use std::collections::{BTreeMap, BTreeSet};
use std::fmt;
use std::sync::Arc;

/// A type of the language (`05-types.md` 2), or an invalid type and why. It
/// is immutable, cheap to clone and safe to share between threads.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Type {
    ty: Ty,
    /// The message of the `bad-type` failure of an invalid type.
    bad: Option<Arc<str>>,
}

/// The kind of a type of one alternative, as [`Type::kind`] gives it.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum TypeKind {
    /// The bottom type `never`, which has no values.
    Never,
    /// The top type `any`, which admits every value.
    Any,
    /// The `null` type.
    Null,
    /// The `bool` type.
    Bool,
    /// The `int` type (whole numbers within `±(2^53 - 1)`).
    Int,
    /// The `float` type (finite IEEE 754 binary64 numbers).
    Float,
    /// Every string, or a set of string literals.
    String,
    /// A list type `T[]`.
    List,
    /// Any map type: a record or `map<T>`.
    Record,
    /// A tagged union of records.
    Union,
    /// An opaque host type, identified by name.
    Opaque,
    /// A type variable of a signature.
    Var,
    /// Two or more alternatives: `T | null` is mixed.
    Mixed,
    /// An invalid type: [`Type::err`] says why.
    Invalid,
}

/// A field of a record type.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FieldType {
    /// The field's map key.
    pub key: String,
    /// The field's value type.
    pub ty: Type,
    /// The field may be absent.
    pub optional: bool,
}

const fn base() -> Ty {
    Ty {
        any: false,
        null: false,
        boolean: false,
        num: None,
        str: None,
        list: None,
        map: None,
        variants: None,
        opaque: BTreeSet::new(),
        refs: BTreeSet::new(),
    }
}

const fn of(ty: Ty) -> Type {
    Type { ty, bad: None }
}

fn invalid(m: impl Into<String>) -> Type {
    Type {
        ty: Ty::never(),
        bad: Some(Arc::from(m.into())),
    }
}

/// A type built: invalid when it passes the size or depth limit.
fn checked(t: Ty) -> Type {
    if t.too_big() {
        return invalid(format!("the type is larger than {}", cty::SIZE_LIMIT));
    }
    if t.depth() > 64 {
        return invalid("the type is deeper than 64");
    }
    of(t)
}

/// A type of one primitive alternative, built field by field so that a
/// constant needs no destructor.
const fn prim(any: bool, null: bool, boolean: bool, num: Option<Num>, string: bool) -> Type {
    of(Ty {
        any,
        null,
        boolean,
        num,
        str: if string { Some(Strs::All) } else { None },
        list: None,
        map: None,
        variants: None,
        opaque: BTreeSet::new(),
        refs: BTreeSet::new(),
    })
}

/// A type whose references hold their tables (F17 12): a part of a
/// recursive type's unfolding outlives the type it came from.
fn strong(ty: Ty) -> Type {
    of(cty::strengthen(&ty))
}

impl Type {
    /// The bottom type `never`, which has no values.
    pub const NEVER: Type = prim(false, false, false, None, false);
    /// The top type `any`, which admits every value.
    pub const ANY: Type = prim(true, false, false, None, false);
    /// The type of the language's null.
    pub const NULL: Type = prim(false, true, false, None, false);
    /// The `bool` type.
    pub const BOOL: Type = prim(false, false, true, None, false);
    /// The `int` type (whole numbers within `±(2^53 - 1)`).
    pub const INT: Type = prim(false, false, false, Some(Num::Int), false);
    /// The `float` type (finite IEEE 754 binary64 numbers).
    pub const FLOAT: Type = prim(false, false, false, Some(Num::Float), false);
    /// The `string` type.
    pub const STRING: Type = prim(false, false, false, None, true);

    pub(crate) fn from_ty(ty: Ty) -> Type {
        strong(ty)
    }

    /// The checker's type of a type known to be valid.
    pub(crate) fn into_ty(self) -> Ty {
        self.ty
    }

    /// The checker's type, or the message of `bad-type`.
    pub(crate) fn ty(&self) -> Result<&Ty, String> {
        match &self.bad {
            Some(m) => Err(m.to_string()),
            None => Ok(&self.ty),
        }
    }

    /// The `bad-type` failure of an invalid type; `None` for a valid one.
    pub fn err(&self) -> Option<Error> {
        let m = self.bad.as_deref()?;
        Some(Error::one(Diagnostic::bare(
            Phase::Declare,
            code::BAD_TYPE,
            m,
        )))
    }

    /// The kind of a type of one alternative; [`TypeKind::Mixed`] for two
    /// or more.
    pub fn kind(&self) -> TypeKind {
        if self.bad.is_some() {
            return TypeKind::Invalid;
        }
        // F17 12.5: a recursive type answers for its unfolding.
        let t = &*self.ty.view();
        if t.any {
            return TypeKind::Any;
        }
        if t.kinds() > 1 {
            return TypeKind::Mixed;
        }
        if t.null {
            TypeKind::Null
        } else if t.boolean {
            TypeKind::Bool
        } else if let Some(n) = t.num {
            match n {
                Num::Int => TypeKind::Int,
                Num::Float => TypeKind::Float,
            }
        } else if t.str.is_some() {
            TypeKind::String
        } else if t.list.is_some() {
            TypeKind::List
        } else if t.variants.is_some() {
            TypeKind::Union
        } else if t.map.is_some() {
            TypeKind::Record
        } else if !t.opaque.is_empty() {
            TypeKind::Opaque
        } else {
            TypeKind::Never
        }
    }

    fn valid(&self) -> Option<&Ty> {
        match self.bad {
            Some(_) => None,
            None => Some(&self.ty),
        }
    }

    /// The unfolding of a valid type (F17 12.5): what its accessors read.
    fn unfolded(&self) -> Option<Ty> {
        self.valid().map(|t| t.view().into_owned())
    }

    /// The element type of its list alternative, or `None`.
    pub fn elem(&self) -> Option<Type> {
        self.unfolded()?.list.as_deref().map(|e| strong(e.clone()))
    }

    /// The fields of the record of its map alternative, in code point order
    /// of their keys. A tagged union answers for its flat record (F5 2.3),
    /// which is what a read sees. Empty when it has no map alternative.
    pub fn fields(&self) -> Vec<FieldType> {
        let t = self.unfolded();
        match t.as_ref().and_then(|t| t.map.as_deref()) {
            None => Vec::new(),
            Some(r) => r
                .fields
                .iter()
                .map(|(k, f)| FieldType {
                    key: k.clone(),
                    ty: strong(f.ty.clone()),
                    optional: f.optional,
                })
                .collect(),
        }
    }

    /// One field of the record of its map alternative, by key.
    pub fn field(&self, key: &str) -> Option<FieldType> {
        let t = self.unfolded()?;
        let r = t.map.as_deref()?;
        r.fields.get(key).map(|f| FieldType {
            key: key.to_string(),
            ty: strong(f.ty.clone()),
            optional: f.optional,
        })
    }

    /// The rest of its map alternative: [`Type::NEVER`] for an exact record,
    /// [`Type::ANY`] for an open one, `T` for `map<T>`. `None` when it has no
    /// map alternative.
    pub fn rest(&self) -> Option<Type> {
        self.unfolded()?
            .map
            .as_deref()
            .map(|r| strong(r.rest.clone()))
    }

    /// Its alternatives, one of each kind: null, bool, the number, the
    /// string or literal set, the list, the map type, and each opaque type.
    /// `any` is its own one alternative; `never` and an invalid type have
    /// none.
    pub fn alternatives(&self) -> Vec<Type> {
        let Some(t) = self.unfolded() else {
            return Vec::new();
        };
        if t.any {
            return vec![Type::ANY];
        }
        let mut out = Vec::new();
        if t.null {
            out.push(Type::NULL);
        }
        if t.boolean {
            out.push(Type::BOOL);
        }
        if let Some(n) = t.num {
            out.push(of(Ty {
                num: Some(n),
                ..base()
            }));
        }
        if let Some(s) = &t.str {
            out.push(of(Ty {
                str: Some(s.clone()),
                ..base()
            }));
        }
        if let Some(e) = &t.list {
            out.push(strong(Ty {
                list: Some(e.clone()),
                ..base()
            }));
        }
        if t.map.is_some() {
            out.push(strong(Ty {
                map: t.map.clone(),
                variants: t.variants.clone(),
                ..base()
            }));
        }
        for o in &t.opaque {
            out.push(of(Ty::opaque(o)));
        }
        out
    }

    /// The variants of a tagged union, in the order of printing (F5 2.4);
    /// empty for any other type.
    pub fn variants(&self) -> Vec<Type> {
        let t = self.unfolded();
        match t.as_ref().and_then(|t| t.variants.as_deref()) {
            None => Vec::new(),
            Some(vs) => vs.iter().map(|r| strong(Ty::rec(r.clone()))).collect(),
        }
    }

    /// The strings of its literal set, sorted; empty when its string
    /// alternative is every string, or it has none.
    pub fn literals(&self) -> Vec<String> {
        match self.unfolded().and_then(|t| t.str) {
            Some(Strs::Lits(l)) => l.into_iter().collect(),
            _ => Vec::new(),
        }
    }

    /// The tag key of a tagged union (F5 2.1), or `None`.
    pub fn tag(&self) -> Option<String> {
        self.unfolded()?.tag()
    }

    /// The type without null (`05-types.md` 4.5).
    pub fn non_null(&self) -> Type {
        match self.valid() {
            Some(t) => strong(t.nonnull()),
            None => self.clone(),
        }
    }

    /// Whether null is one of its values. `any` is nullable.
    pub fn is_nullable(&self) -> bool {
        self.unfolded().is_some_and(|t| t.any || t.null)
    }

    /// Whether its map alternative refuses every key it does not list.
    pub fn is_exact(&self) -> bool {
        self.unfolded()
            .is_some_and(|t| t.map.is_some() && t.variants().iter().all(|r| r.rest.is_never()))
    }

    /// The name of an opaque type; empty for any other kind.
    pub fn opaque_name(&self) -> String {
        if self.kind() != TypeKind::Opaque {
            return String::new();
        }
        self.ty
            .view()
            .opaque
            .iter()
            .next()
            .cloned()
            .unwrap_or_default()
    }

    /// Fits(S, T) (`05-types.md` 4.2): every value of `self` is a value of
    /// `want`. False when either is invalid.
    pub fn fits(&self, want: &Type) -> bool {
        match (self.valid(), want.valid()) {
            (Some(s), Some(t)) => {
                // F17 12.5 item 7: past the bound of pairs, false.
                let w = cty::watch();
                let r = cty::fits(s, t).is_some();
                r && !w.passed()
            }
            _ => false,
        }
    }

    /// Whether the two are the same type: the same by structure (F5 2.5),
    /// whatever the names of their recursive types (F17 12).
    pub fn equal(&self, u: &Type) -> bool {
        match (self.valid(), u.valid()) {
            (Some(a), Some(b)) => {
                let w = cty::watch();
                let r = cty::equal(a, b);
                r && !w.passed()
            }
            _ => self == u,
        }
    }
}

impl fmt::Display for Type {
    /// The type text (`05-types.md` 3.9). An invalid type prints as
    /// `invalid`.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.valid() {
            Some(t) => f.write_str(&cty::print(t)),
            None => f.write_str("invalid"),
        }
    }
}

/// The first invalid type of a list, as an invalid type.
fn first_invalid<'a>(ts: impl IntoIterator<Item = &'a Type>) -> Option<Type> {
    ts.into_iter().find(|t| t.bad.is_some()).cloned()
}

/// `T[]`.
pub fn list_type(elem: &Type) -> Type {
    match elem.valid() {
        Some(e) => checked(Ty::list(e.clone())),
        None => elem.clone(),
    }
}

/// `map<T>`: a map whose every key has a value of type `T`.
pub fn map_type(value: &Type) -> Type {
    match value.valid() {
        Some(v) => checked(Ty::map_of(v.clone())),
        None => value.clone(),
    }
}

/// The type or null: `T | null`.
pub fn nullable(t: &Type) -> Type {
    match t.valid() {
        Some(x) => of(x.clone().with_null()),
        None => t.clone(),
    }
}

/// The union of string literals: `"a" | "b"`. With none it is invalid.
pub fn literal_type(values: &[&str]) -> Type {
    if values.is_empty() {
        return invalid("a literal type has one string or more");
    }
    checked(Ty::lits(values.iter().copied()))
}

/// A record with the fields given, whose other keys have values of type
/// `rest`: [`Type::NEVER`] for an exact record, [`Type::ANY`] for an open
/// one. A key given twice makes it invalid.
pub fn record_type(fields: &[FieldType], rest: &Type) -> Type {
    if let Some(b) = first_invalid(fields.iter().map(|f| &f.ty).chain([rest])) {
        return b;
    }
    let mut m = BTreeMap::new();
    for f in fields {
        let field = cty::Field {
            ty: f.ty.ty.clone(),
            optional: f.optional,
            doc: Default::default(),
        };
        if m.insert(f.key.clone(), field).is_some() {
            return invalid(format!("the key {:?} is written twice", f.key));
        }
    }
    checked(Ty::rec(Rec::new(m, rest.ty.clone())))
}

/// `record_type(fields, &Type::NEVER)`: a record that refuses every key it
/// does not list.
pub fn exact_record_type(fields: &[FieldType]) -> Type {
    record_type(fields, &Type::NEVER)
}

/// The union of types, as type text joins alternatives (`05-types.md`
/// 3.4). Two or more records form a tagged union, and the union is invalid
/// when they have no tag (F5 2.1).
pub fn union_type(types: &[Type]) -> Type {
    if let Some(b) = first_invalid(types) {
        return b;
    }
    let ts: Vec<Ty> = types.iter().map(|t| t.ty.clone()).collect();
    let w = cty::watch();
    let u = cty::union_of_alts(&ts);
    built(u, w.passed())
}

/// A union a constructor built: invalid with no tag, or when a Join of its
/// records passed the bound of pairs (F17 12.5 item 7). The references it
/// reaches hold their tables: a definition read through a reference holds
/// its own table weakly, and the union outlives the types it came from.
fn built(u: Option<Ty>, passed: bool) -> Type {
    if passed {
        return invalid("comparing the types takes more than 4096 pairs of types");
    }
    match u {
        Some(u) => checked(cty::strengthen(&u)),
        None => invalid("the records have no tag"),
    }
}

/// The tagged union of records (F5 2.1). It is invalid unless each type is
/// one record, there are two or more, and they have a tag.
pub fn tagged_union_type(records: &[Type]) -> Type {
    if let Some(b) = first_invalid(records) {
        return b;
    }
    let mut recs = Vec::new();
    for r in records {
        if r.kind() != TypeKind::Record {
            return invalid(format!("{r} is not one record"));
        }
        match r.ty.view().map.as_deref() {
            Some(m) => recs.push(m.clone()),
            None => return invalid(format!("{r} is not one record")),
        }
    }
    if recs.len() < 2 {
        return invalid("a tagged union has two variants or more");
    }
    let w = cty::watch();
    let u = Ty::of_variants(recs);
    built(u, w.passed())
}

/// The opaque type of a name. An environment that uses it must declare the
/// name.
pub fn opaque_type_named(name: &str) -> Type {
    of(Ty::opaque(name))
}

/// Join(A, B) (`05-types.md` 4.1): the least type both fit. Invalid when
/// either is, or when the Join passes the bound of F17 12.5 item 7.
pub fn join(a: &Type, b: &Type) -> Type {
    if let Some(x) = first_invalid([a, b]) {
        return x;
    }
    let w = cty::watch();
    let j = cty::join(&a.ty, &b.ty);
    if w.passed() {
        return invalid("comparing the types takes more than 4096 pairs of types");
    }
    checked(cty::strengthen(&j))
}

/// The type a JSON Schema maps to, with the options given (F1 2). The
/// schema is JSON text. An error is the `declare` error `bad-type`.
pub fn type_from_schema(schema: &str, opts: SchemaOptions) -> Result<Type, Error> {
    let bad = |m: String| Error::one(Diagnostic::bare(Phase::Declare, code::BAD_TYPE, m));
    let j = crate::json::parse(schema).map_err(bad)?;
    crate::schema::ty_schema(&j, opts).map(strong).map_err(bad)
}

/// SchemaOf(T) (`f16-entry-points.md` 11.5): the JSON Schema of a type, as
/// its JSON text: no white space, the keys of every object in code point
/// order, so two implementations give the same text for one type. A field's
/// description is its `"description"`; a recursive type is written once,
/// under `"$defs"`. Refused with the `declare` error `bad-type` when the
/// type holds an opaque type or a type variable, or reaches two recursive
/// types of one name; an invalid type gives its own error. It charges
/// nothing and needs no environment.
pub fn schema_of(t: &Type) -> Result<String, Error> {
    let ty = t.ty().map_err(bad_type)?;
    crate::schema::schema_of(ty)
        .map(|j| crate::json::to_text(&j))
        .map_err(bad_type)
}

fn bad_type(m: String) -> Error {
    Error::one(Diagnostic::bare(Phase::Declare, code::BAD_TYPE, m))
}

/// The options of [`conform_with`] and [`conform_all_with`]
/// (`f04-conversion.md` 7.2).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct ConformOptions {
    /// The step limit (7.3), or `None` for no limit. The default is the
    /// default step limit, 1,000,000, whatever the limits of the environment
    /// that resolved the type.
    pub steps: Option<u64>,
}

impl Default for ConformOptions {
    fn default() -> ConformOptions {
        ConformOptions {
            steps: Some(crate::conform::FREE_STEPS),
        }
    }
}

/// The conformance function (`f04-conversion.md` 7): `Ok(())` when the
/// value conforms to the type, else the diagnostic of the first failure. It
/// is not a run, but it charges the steps a run's walk charges, against the
/// default step limit: past it, `steps-exceeded` with no path (7.3). Host
/// data that holds one list at many places is charged at each. An invalid
/// type gives its `bad-type` diagnostic.
pub fn conform(v: &Value<'_>, t: &Type) -> Result<(), Diagnostic> {
    conform_with(v, t, &ConformOptions::default())
}

/// [`conform`] with the step limit of `options` (7.2, 7.3).
pub fn conform_with(v: &Value<'_>, t: &Type, options: &ConformOptions) -> Result<(), Diagnostic> {
    use crate::conform::{limit_diagnostic, test, Free};
    let ty = t
        .ty()
        .map_err(|m| Diagnostic::bare(Phase::Declare, code::BAD_TYPE, m))?;
    match test(v, ty, &mut Free::new(options.steps)) {
        Ok(Some(f)) => Err(f.diagnostic()),
        Ok(None) => Ok(()),
        // F17 12.7 item 3: past the default depth limit; 7.3: past the
        // step limit.
        Err(code) => Err(limit_diagnostic(code, options.steps)),
    }
}

/// Every failure of the value, up to `max` (`f08-errors.md` 6), in the
/// order of 6.2. A maximum less than 1 is read as 1. The list is empty
/// exactly when the value conforms, and its first entry is the diagnostic
/// of [`conform`]. It counts steps as [`conform`] does (6.4): a limit ends
/// the list with its diagnostic.
pub fn conform_all(v: &Value<'_>, t: &Type, max: usize) -> Vec<Diagnostic> {
    conform_all_with(v, t, max, &ConformOptions::default())
}

/// [`conform_all`] with the step limit of `options` (F4 7.2, F8 6.4).
pub fn conform_all_with(
    v: &Value<'_>,
    t: &Type,
    max: usize,
    options: &ConformOptions,
) -> Vec<Diagnostic> {
    let ty = match t.ty() {
        Ok(ty) => ty,
        Err(m) => return vec![Diagnostic::bare(Phase::Declare, code::BAD_TYPE, m)],
    };
    crate::conform::all(v, ty, max.max(1), options.steps)
        .iter()
        .map(crate::conform::Failure::diagnostic)
        .collect()
}
