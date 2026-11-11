package risor

import (
	"errors"
	"fmt"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
)

// The structural type API: a host builds, reads, joins and compares the
// checker's own types (05-types.md 3), with no type text in between. A
// Type is immutable and may be shared by goroutines.
//
// A constructor returns a Type and no error, so types compose in one
// expression: risor.ListType(risor.Nullable(risor.TypeString)). A result
// past the limits of a type (depth 64, size 100,000; 05-types.md 20.4,
// 20.7), or a construction that has no type (an empty literal set, two
// records that form no tagged union, a bad opaque name), is an invalid
// type: its Err gives the bad-type error with the reason. A constructor or
// Join given an invalid type gives an invalid type with the first error;
// every declaration that takes a type (VarType, NamedTypeOf, ExpectType,
// LoadTypeOf) refuses one with that error when the environment is built or
// the program checked.

// TypeKind is the kind of a type of one alternative.
type TypeKind int

// The kinds of a type.
const (
	KindNever   TypeKind = iota // no value
	KindAny                     // every value
	KindNull                    // null
	KindBool                    // bool
	KindInt                     // int
	KindFloat                   // float
	KindString                  // string, or a set of literal strings
	KindList                    // a list
	KindRecord                  // a record: a map type
	KindUnion                   // a tagged union of records (F5)
	KindOpaque                  // an opaque type, by name
	KindVar                     // a type variable of a signature
	KindMixed                   // two or more alternatives, T | null among them
	KindInvalid                 // an invalid type: see Type.Err
)

// The simple types.
var (
	TypeNever  = Type{t: types.Never}
	TypeAny    = Type{t: types.Any}
	TypeNull   = Type{t: types.Null}
	TypeBool   = Type{t: types.Bool}
	TypeInt    = Type{t: types.Int}
	TypeFloat  = Type{t: types.Float}
	TypeString = Type{t: types.String}
)

// invalidTypeText is what String gives for an invalid type.
const invalidTypeText = "<invalid type>"

// FieldType is a field of a record type: its key, its type, and whether
// it may be absent.
type FieldType struct {
	Key      string
	Type     Type
	Optional bool
}

func invalid(format string, args ...any) Type {
	return Type{err: &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType,
		Message: fmt.Sprintf(format, args...)}}}}
}

func limited(t *types.Type) Type {
	if err := types.Limit(t); err != nil {
		return invalid("%v", err)
	}
	return Type{t: t}
}

func (t Type) inner() *types.Type {
	if t.t == nil {
		return types.Never
	}
	return t.t
}

// firstErr is the first invalid type of ts, or the zero Type.
func firstErr(ts ...Type) (Type, bool) {
	for _, t := range ts {
		if t.err != nil {
			return Type{err: t.err}, true
		}
	}
	return Type{}, false
}

// Err is nil for a valid type. For an invalid type it is the *Error of
// phase declare and code bad-type that says why.
func (t Type) Err() error { return t.err }

// problem is the reason a declaration refuses t, or "": an invalid type,
// or the zero Type, which no constructor gives.
func (t Type) problem() string {
	if t.err != nil {
		var e *Error
		if errors.As(t.err, &e) && len(e.Diagnostics) > 0 {
			return e.Diagnostics[0].Message
		}
		return t.err.Error()
	}
	if t.t == nil {
		return "no type"
	}
	return ""
}

// LiteralType is the set of the literal strings given, such as
// "a" | "b". With no string it is an invalid type.
func LiteralType(strs ...string) Type {
	if len(strs) == 0 {
		return invalid("a literal set needs a string")
	}
	return limited(types.Lit(strs...))
}

// ListType is the list type E[].
func ListType(elem Type) Type {
	if bad, ok := firstErr(elem); ok {
		return bad
	}
	return limited(types.List(elem.inner()))
}

// RecordType is a record with the fields given and a rest: TypeNever for
// an exact record (no other key), TypeAny for an open record ({..., ...}),
// any other type for a typed rest ({..., ...T}). A key given twice makes
// an invalid type.
func RecordType(fields []FieldType, rest Type) Type {
	if bad, ok := firstErr(rest); ok {
		return bad
	}
	fs := make([]types.Field, len(fields))
	seen := map[string]bool{}
	for i, f := range fields {
		if bad, ok := firstErr(f.Type); ok {
			return bad
		}
		if seen[f.Key] {
			return invalid("the key %q is given twice", f.Key)
		}
		seen[f.Key] = true
		fs[i] = types.Field{Key: f.Key, Type: f.Type.inner(), Optional: f.Optional}
	}
	return limited(types.Rec(fs, rest.inner()))
}

// ExactRecordType is RecordType with no other key allowed.
func ExactRecordType(fields ...FieldType) Type { return RecordType(fields, TypeNever) }

// MapType is map<T>: a record with no listed field whose every key has
// type T.
func MapType(value Type) Type {
	if bad, ok := firstErr(value); ok {
		return bad
	}
	return limited(types.MapOf(value.inner()))
}

// UnionType is the type of the alternatives given, as | writes them in
// type text: their Join, where records among them, two or more, must form
// a tagged union (F5 3.1), else the type is invalid. TaggedUnionType says
// the same for records only.
func UnionType(alts ...Type) Type {
	if bad, ok := firstErr(alts...); ok {
		return bad
	}
	ts := make([]*types.Type, len(alts))
	for i, a := range alts {
		ts[i] = a.inner()
	}
	u, err := types.UnionOf(ts)
	if err != nil {
		return invalid("%v", err)
	}
	return limited(u)
}

// TaggedUnionType is the tagged union of the record types given (F5 2):
// they must share a tag field of literal strings, whose sets share no
// string, else the type is invalid. A tagged union given among them
// contributes its variants.
func TaggedUnionType(variants ...Type) Type {
	if bad, ok := firstErr(variants...); ok {
		return bad
	}
	for _, v := range variants {
		if k := v.Kind(); k != KindRecord && k != KindUnion {
			return invalid("a variant of a tagged union is a record, not %v", v)
		}
	}
	if len(variants) < 2 {
		return invalid("a tagged union needs two records or more")
	}
	return UnionType(variants...)
}

// Nullable is t | null.
func Nullable(t Type) Type {
	if bad, ok := firstErr(t); ok {
		return bad
	}
	return limited(types.WithNull(t.inner()))
}

// OpaqueTypeNamed is the opaque type of that name; a name that is not one
// of an opaque type makes an invalid type. The environment that reads it
// must declare it (OpaqueType).
func OpaqueTypeNamed(name string) Type {
	if !opaqueNameRe.MatchString(name) {
		return invalid("%q is not a name for an opaque type", name)
	}
	return Type{t: types.Opaque(name)}
}

// Join is the Join of two types (05-types.md 7.5): the least type that
// both fit, by the checker's rule. Two records join to one record.
func Join(a, b Type) Type {
	if bad, ok := firstErr(a, b); ok {
		return bad
	}
	return limited(types.Join(a.inner(), b.inner()))
}

// Fits reports whether every value of t is a value of want (05-types.md
// 7.3). A fit through any counts: any fits every type and every type
// fits any, as the checker reads them. With an invalid type it is false.
func (t Type) Fits(want Type) bool {
	if t.err != nil || want.err != nil {
		return false
	}
	ok, _ := types.Fits(t.inner(), want.inner())
	return ok
}

// Equal reports whether two types are the same type. With an invalid type
// it is false.
func (t Type) Equal(u Type) bool {
	if t.err != nil || u.err != nil {
		return false
	}
	return types.Equal(t.inner(), u.inner())
}

// Kind is the kind of a type of one alternative: KindNever for never and
// for the zero Type, KindInvalid for an invalid type, and KindMixed for a
// type of two alternatives or more. A nullable type is mixed: the kind of
// T | null is KindMixed, and t.NonNull().Kind() is T's.
func (t Type) Kind() TypeKind {
	if t.err != nil {
		return KindInvalid
	}
	in := t.inner()
	alts := types.Alternatives(in)
	switch len(alts) {
	case 0:
		return KindNever
	case 1:
	default:
		return KindMixed
	}
	a := alts[0]
	switch {
	case a.IsAny():
		return KindAny
	case a.HasNull():
		return KindNull
	case a.HasBool():
		return KindBool
	case a.IsInt():
		return KindInt
	case a.IsFloat():
		return KindFloat
	case a.HasString():
		return KindString
	case a.Elem() != nil:
		return KindList
	case a.Union() != nil:
		return KindUnion
	case a.Record() != nil:
		return KindRecord
	case len(a.OpaqueNames()) == 1:
		return KindOpaque
	}
	return KindVar
}

// Alternatives splits a type into its alternatives, one of each kind:
// null, bool, the number, the string or literal set, the list, the map
// type (a record or a tagged union), and each opaque type. any is its one
// alternative; never, and an invalid type, have none. The kinds other than
// null are those of t.NonNull().Alternatives(), each alternative's Kind().
func (t Type) Alternatives() []Type {
	alts := types.Alternatives(t.inner())
	out := make([]Type, len(alts))
	for i, a := range alts {
		out[i] = Type{t: a}
	}
	return out
}

// IsNullable reports whether null is a value of t (any included).
func (t Type) IsNullable() bool { in := t.inner(); return in.IsAny() || in.HasNull() }

// NonNull is t without null.
func (t Type) NonNull() Type { return Type{t: types.Nonnull(t.inner())} }

// Elem is the element type of t's list alternative, or false.
func (t Type) Elem() (Type, bool) {
	if e := t.inner().Elem(); e != nil {
		return Type{t: e}, true
	}
	return Type{}, false
}

// Fields are the fields of t's map alternative in key order, each with
// its mark; for a tagged union, those of its flattened record. Nil when t
// has no map alternative.
func (t Type) Fields() []FieldType {
	r := t.inner().Record()
	if r == nil {
		return nil
	}
	out := make([]FieldType, len(r.Fields))
	for i, f := range r.Fields {
		out[i] = FieldType{Key: f.Key, Type: Type{t: f.Type}, Optional: f.Optional}
	}
	return out
}

// Rest is the rest of t's map alternative: TypeNever for an exact record,
// TypeAny for an open one. False when t has no map alternative.
func (t Type) Rest() (Type, bool) {
	r := t.inner().Record()
	if r == nil {
		return Type{}, false
	}
	return Type{t: r.Rest}, true
}

// IsExact reports whether t's map alternative is an exact record.
func (t Type) IsExact() bool {
	r := t.inner().Record()
	return r != nil && r.Rest.IsNever()
}

// Literals are the strings of t's literal set, sorted; nil when t's string
// alternative is every string, or t has none.
func (t Type) Literals() []string {
	in := t.inner()
	if !in.HasString() || in.IsStringAll() {
		return nil
	}
	return append([]string(nil), in.Lits()...)
}

// OpaqueName is the name of an opaque type of one alternative, or "".
func (t Type) OpaqueName() string {
	if ns := t.inner().OpaqueNames(); len(ns) == 1 && t.Kind() == KindOpaque {
		return ns[0]
	}
	return ""
}

// Tag is the tag field of t's tagged union, or false.
func (t Type) Tag() (string, bool) {
	if u := t.inner().Union(); u != nil {
		return u.Tag, true
	}
	return "", false
}

// Variants are the records of t's tagged union, in the order of their
// least tag string; nil when t has none.
func (t Type) Variants() []Type {
	u := t.inner().Union()
	if u == nil {
		return nil
	}
	out := make([]Type, len(u.Variants))
	for i, v := range u.Variants {
		out[i] = Type{t: types.RecType(v)}
	}
	return out
}

// Field is the field of t's map alternative under key, as Fields lists
// it, without building the list.
func (t Type) Field(key string) (FieldType, bool) {
	r := t.inner().Record()
	if r == nil {
		return FieldType{}, false
	}
	f, ok := r.Field(key)
	if !ok {
		return FieldType{}, false
	}
	return FieldType{Key: f.Key, Type: Type{t: f.Type}, Optional: f.Optional}, true
}

// IsZero reports whether t is the zero Type: a Type no constructor gives,
// read as never by the accessors and refused by a declaration. It differs
// from an invalid type, which a constructor gives for a construction that
// has no type and which carries the reason in Err; IsZero is false for an
// invalid type.
func (t Type) IsZero() bool { return t.t == nil && t.err == nil }
