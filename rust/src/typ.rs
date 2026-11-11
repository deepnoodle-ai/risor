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
    Never,
    Any,
    Null,
    Bool,
    Int,
    Float,
    /// Every string, or a set of string literals.
    String,
    List,
    /// Any map type: a record or `map<T>`.
    Record,
    /// A tagged union of records.
    Union,
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
    pub key: String,
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
    })
}

impl Type {
    pub const NEVER: Type = prim(false, false, false, None, false);
    pub const ANY: Type = prim(true, false, false, None, false);
    /// The type of the language's null.
    pub const NULL: Type = prim(false, true, false, None, false);
    pub const BOOL: Type = prim(false, false, true, None, false);
    pub const INT: Type = prim(false, false, false, Some(Num::Int), false);
    pub const FLOAT: Type = prim(false, false, false, Some(Num::Float), false);
    pub const STRING: Type = prim(false, false, false, None, true);

    pub(crate) fn from_ty(ty: Ty) -> Type {
        of(ty)
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
        let t = &self.ty;
        if self.bad.is_some() {
            return TypeKind::Invalid;
        }
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

    /// The element type of its list alternative, or `None`.
    pub fn elem(&self) -> Option<Type> {
        self.valid()?.list.as_deref().map(|e| of(e.clone()))
    }

    /// The fields of the record of its map alternative, in code point order
    /// of their keys. A tagged union answers for its flat record (F5 2.3),
    /// which is what a read sees. Empty when it has no map alternative.
    pub fn fields(&self) -> Vec<FieldType> {
        match self.valid().and_then(|t| t.map.as_deref()) {
            None => Vec::new(),
            Some(r) => r
                .fields
                .iter()
                .map(|(k, f)| FieldType {
                    key: k.clone(),
                    ty: of(f.ty.clone()),
                    optional: f.optional,
                })
                .collect(),
        }
    }

    /// One field of the record of its map alternative, by key.
    pub fn field(&self, key: &str) -> Option<FieldType> {
        let r = self.valid()?.map.as_deref()?;
        r.fields.get(key).map(|f| FieldType {
            key: key.to_string(),
            ty: of(f.ty.clone()),
            optional: f.optional,
        })
    }

    /// The rest of its map alternative: [`Type::NEVER`] for an exact record,
    /// [`Type::ANY`] for an open one, `T` for `map<T>`. `None` when it has no
    /// map alternative.
    pub fn rest(&self) -> Option<Type> {
        self.valid()?.map.as_deref().map(|r| of(r.rest.clone()))
    }

    /// Its alternatives, one of each kind: null, bool, the number, the
    /// string or literal set, the list, the map type, and each opaque type.
    /// `any` is its own one alternative; `never` and an invalid type have
    /// none.
    pub fn alternatives(&self) -> Vec<Type> {
        let Some(t) = self.valid() else {
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
            out.push(of(Ty {
                list: Some(e.clone()),
                ..base()
            }));
        }
        if t.map.is_some() {
            out.push(of(Ty {
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
        match self.valid().and_then(|t| t.variants.as_deref()) {
            None => Vec::new(),
            Some(vs) => vs.iter().map(|r| of(Ty::rec(r.clone()))).collect(),
        }
    }

    /// The strings of its literal set, sorted; empty when its string
    /// alternative is every string, or it has none.
    pub fn literals(&self) -> Vec<String> {
        match self.valid().and_then(|t| t.str.as_ref()) {
            Some(Strs::Lits(l)) => l.iter().cloned().collect(),
            _ => Vec::new(),
        }
    }

    /// The tag key of a tagged union (F5 2.1), or `None`.
    pub fn tag(&self) -> Option<String> {
        self.valid()?.tag()
    }

    /// The type without null (`05-types.md` 4.5).
    pub fn non_null(&self) -> Type {
        match self.valid() {
            Some(t) => of(t.nonnull()),
            None => self.clone(),
        }
    }

    /// Whether null is one of its values. `any` is nullable.
    pub fn is_nullable(&self) -> bool {
        self.valid().is_some_and(|t| t.any || t.null)
    }

    /// Whether its map alternative refuses every key it does not list.
    pub fn is_exact(&self) -> bool {
        self.valid()
            .is_some_and(|t| t.map.is_some() && t.variants().iter().all(|r| r.rest.is_never()))
    }

    /// The name of an opaque type; empty for any other kind.
    pub fn opaque_name(&self) -> String {
        if self.kind() != TypeKind::Opaque {
            return String::new();
        }
        self.ty.opaque.iter().next().cloned().unwrap_or_default()
    }

    /// Fits(S, T) (`05-types.md` 4.2): every value of `self` is a value of
    /// `want`. False when either is invalid.
    pub fn fits(&self, want: &Type) -> bool {
        match (self.valid(), want.valid()) {
            (Some(s), Some(t)) => cty::fits(s, t).is_some(),
            _ => false,
        }
    }

    /// Whether the two are the same type.
    pub fn equal(&self, u: &Type) -> bool {
        self == u
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
    match cty::union_of_alts(&ts) {
        Some(u) => checked(u),
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
        recs.push(
            r.ty.map
                .as_deref()
                .cloned()
                .expect("a record type has a record"),
        );
    }
    if recs.len() < 2 {
        return invalid("a tagged union has two variants or more");
    }
    match Ty::of_variants(recs) {
        Some(t) => checked(t),
        None => invalid("the records have no tag"),
    }
}

/// The opaque type of a name. An environment that uses it must declare the
/// name.
pub fn opaque_type_named(name: &str) -> Type {
    of(Ty::opaque(name))
}

/// Join(A, B) (`05-types.md` 4.1): the least type both fit. Invalid when
/// either is.
pub fn join(a: &Type, b: &Type) -> Type {
    if let Some(x) = first_invalid([a, b]) {
        return x;
    }
    checked(cty::join(&a.ty, &b.ty))
}

/// The type a JSON Schema maps to, with the options given (F1 2). The
/// schema is JSON text. An error is the `declare` error `bad-type`.
pub fn type_from_schema(schema: &str, opts: SchemaOptions) -> Result<Type, Error> {
    let bad = |m: String| Error::one(Diagnostic::bare(Phase::Declare, code::BAD_TYPE, m));
    let j = crate::json::parse(schema).map_err(bad)?;
    crate::schema::ty_schema(&j, opts).map(of).map_err(bad)
}

/// The conformance function (`05-host-api.md` A.13): `Ok(())` when the
/// value conforms to the type, else the diagnostic of the first failure. It
/// is not a run: no limit, no usage. An invalid type gives its `bad-type`
/// diagnostic.
pub fn conform(v: &Value<'_>, t: &Type) -> Result<(), Diagnostic> {
    use crate::conform::{test, Free};
    let ty = t
        .ty()
        .map_err(|m| Diagnostic::bare(Phase::Declare, code::BAD_TYPE, m))?;
    match test(v, ty, &mut Free) {
        Ok(Some(f)) => Err(f.diagnostic()),
        _ => Ok(()),
    }
}

/// Every failure of the value, up to `max` (`f08-errors.md` 6), in the
/// order of 6.2. A maximum less than 1 is read as 1. The list is empty
/// exactly when the value conforms, and its first entry is the diagnostic
/// of [`conform`].
pub fn conform_all(v: &Value<'_>, t: &Type, max: usize) -> Vec<Diagnostic> {
    let ty = match t.ty() {
        Ok(ty) => ty,
        Err(m) => return vec![Diagnostic::bare(Phase::Declare, code::BAD_TYPE, m)],
    };
    crate::conform::all(v, ty, max.max(1))
        .iter()
        .map(crate::conform::Failure::diagnostic)
        .collect()
}
