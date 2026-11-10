// Package types is the fuzzer's own small model of the types of
// spec/core/05-types.md: type text read and printed (3.1, 3.9), the type of
// a host value, and the conformance of a value to a type (18.1). It knows
// nothing of any implementation.
package types

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Type is a type: any, or a set of alternatives with at most one of each
// kind (2.1). The zero Type with Any false is never.
type Type struct {
	Any  bool
	Null bool
	Bool bool
	Num  int      // 0 absent, 1 int, 2 float
	Str  bool     // the alternative string
	Lits []string // a literal set, sorted; used when Str is false
	List *Type    // the element type, when a list is an alternative
	Rec  *Record  // the record, when a map is an alternative
	// Union holds the variants when the map alternative is a tagged union of
	// package F5 (f05-unions.md 2): two or more records with a tag. Rec is
	// then nil, and Tag is the tag's key.
	Union []*Record
	Tag   string
	Opq   []string // opaque names, sorted
}

// Record is a record: its fields, sorted by key, and its rest (nil is
// never).
type Record struct {
	Fields []Field
	Rest   *Type
}

// Field is one field of a record.
type Field struct {
	Key string
	Opt bool
	T   *Type
}

// Common types.
var (
	Any   = &Type{Any: true}
	Never = &Type{}
)

// Prim makes a type of one primitive alternative by its word.
func Prim(word string) *Type {
	switch word {
	case "null":
		return &Type{Null: true}
	case "bool":
		return &Type{Bool: true}
	case "int":
		return &Type{Num: 1}
	case "float":
		return &Type{Num: 2}
	case "string":
		return &Type{Str: true}
	case "any":
		return Any
	}
	return Never
}

// ListOf makes the type T[].
func ListOf(t *Type) *Type { return &Type{List: t} }

// IsNever says whether t is the empty set.
func (t *Type) IsNever() bool {
	return !t.Any && !t.Null && !t.Bool && t.Num == 0 && !t.Str && len(t.Lits) == 0 && t.List == nil && t.Rec == nil && len(t.Union) == 0 && len(t.Opq) == 0
}

// Copy makes a shallow copy of t.
func (t *Type) Copy() *Type { c := *t; return &c }

// Join gives the union of two types (05-types.md 4.1, for the cases the
// fuzzer needs). Two records are joined field by field: a field in only one
// becomes optional. Core never holds two records in one type.
func Join(a, b *Type) *Type {
	if a.Any || b.Any {
		return Any
	}
	c := a.Copy()
	c.Null = a.Null || b.Null
	c.Bool = a.Bool || b.Bool
	c.Num = max(a.Num, b.Num)
	switch {
	case a.Str || b.Str:
		c.Str, c.Lits = true, nil
	default:
		c.Lits = union(a.Lits, b.Lits)
	}
	switch {
	case a.List == nil:
		c.List = b.List
	case b.List != nil:
		c.List = Join(a.List, b.List)
	}
	switch {
	case len(a.Union) > 0 || len(b.Union) > 0:
		// A Join never makes a union (f05-unions.md 4.1): the records join.
		var all []*Record
		for _, x := range []*Type{a, b} {
			all = append(all, x.records()...)
		}
		r := all[0]
		for _, x := range all[1:] {
			r = joinRec(r, x)
		}
		c.Rec, c.Union, c.Tag = r, nil, ""
	case a.Rec == nil:
		c.Rec = b.Rec
	case b.Rec != nil:
		c.Rec = joinRec(a.Rec, b.Rec)
	}
	c.Opq = union(a.Opq, b.Opq)
	return c
}

// records gives the records of t's map alternative: the record, or the
// variants of a union.
func (t *Type) records() []*Record {
	if t.Rec != nil {
		return []*Record{t.Rec}
	}
	return t.Union
}

// FindTag gives the tag of two or more records (f05-unions.md 2.1): the
// least key that each requires with a literal set and nothing else, no
// string in two of the sets. It reports false when they have none.
func FindTag(rs []*Record) (string, bool) {
	if len(rs) < 2 {
		return "", false
	}
	var keys []string
	for _, f := range rs[0].Fields {
		keys = append(keys, f.Key)
	}
	sort.Strings(keys)
	for _, k := range keys {
		seen := map[string]bool{}
		ok := true
		for _, r := range rs {
			f := r.field(k)
			if f == nil || f.Opt || !isLitSet(f.T) {
				ok = false
				break
			}
			for _, l := range f.T.Lits {
				if seen[l] {
					ok = false
				}
				seen[l] = true
			}
		}
		if ok {
			return k, true
		}
	}
	return "", false
}

func isLitSet(t *Type) bool {
	return len(t.Lits) > 0 && !t.Any && !t.Null && !t.Bool && t.Num == 0 && !t.Str && t.List == nil && t.Rec == nil && len(t.Union) == 0 && len(t.Opq) == 0
}

// MakeUnion makes a type whose map alternative is the union of rs, or
// reports false when they form none.
func MakeUnion(rs []*Record) (*Type, bool) {
	tag, ok := FindTag(rs)
	if !ok {
		return nil, false
	}
	return &Type{Union: rs, Tag: tag}, true
}

// variantOf gives the variant of a union whose tag set holds s.
func (t *Type) variantOf(s string) *Record {
	for _, r := range t.Union {
		for _, l := range r.field(t.Tag).T.Lits {
			if l == s {
				return r
			}
		}
	}
	return nil
}

func union(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	set := map[string]bool{}
	for _, s := range append(append([]string(nil), a...), b...) {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func joinRec(a, b *Record) *Record {
	r := &Record{}
	keys := map[string]bool{}
	for _, f := range a.Fields {
		keys[f.Key] = true
	}
	for _, f := range b.Fields {
		keys[f.Key] = true
	}
	var ks []string
	for k := range keys {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fa, fb := a.field(k), b.field(k)
		switch {
		case fa != nil && fb != nil:
			r.Fields = append(r.Fields, Field{Key: k, Opt: fa.Opt || fb.Opt, T: Join(fa.T, fb.T)})
		case fa != nil:
			r.Fields = append(r.Fields, Field{Key: k, Opt: true, T: fa.T})
		default:
			r.Fields = append(r.Fields, Field{Key: k, Opt: true, T: fb.T})
		}
	}
	switch {
	case a.Rest == nil:
		r.Rest = b.Rest
	case b.Rest == nil:
		r.Rest = a.Rest
	default:
		r.Rest = Join(a.Rest, b.Rest)
	}
	return r
}

func (r *Record) field(k string) *Field {
	for i := range r.Fields {
		if r.Fields[i].Key == k {
			return &r.Fields[i]
		}
	}
	return nil
}

// OfValue gives the narrowest type of a host value as JSON decodes it:
// whole numbers in range are int, maps are exact records.
func OfValue(v any) *Type {
	switch v := v.(type) {
	case nil:
		return &Type{Null: true}
	case bool:
		return &Type{Bool: true}
	case float64:
		if IsWhole(v) {
			return &Type{Num: 1}
		}
		return &Type{Num: 2}
	case string:
		return &Type{Str: true}
	case []any:
		el := Never
		for _, x := range v {
			el = Join(el, OfValue(x))
		}
		return ListOf(el)
	case map[string]any:
		if n, ok := OpaqueName(v); ok {
			return &Type{Opq: []string{n}}
		}
		r := &Record{}
		for _, k := range sortedKeys(v) {
			r.Fields = append(r.Fields, Field{Key: k, T: OfValue(v[k])})
		}
		return &Type{Rec: r}
	}
	return Any
}

// IsWhole says whether a number is a whole number (01-language.md 6.2).
func IsWhole(f float64) bool {
	return f == math.Trunc(f) && math.Abs(f) <= 9007199254740991
}

// OpaqueName gives the type name of an opaque value as a case file and an
// outcome line write it: {"$opaque": {"type": T, "id": N}}.
func OpaqueName(m map[string]any) (string, bool) {
	if len(m) != 1 {
		return "", false
	}
	o, ok := m["$opaque"].(map[string]any)
	if !ok {
		return "", false
	}
	n, ok := o["type"].(string)
	return n, ok
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Conforms says whether a value conforms to a type (18.1).
func Conforms(v any, t *Type) bool {
	if t.Any {
		return true
	}
	switch v := v.(type) {
	case nil:
		return t.Null
	case bool:
		return t.Bool
	case float64:
		return t.Num == 2 || t.Num == 1 && IsWhole(v)
	case string:
		if t.Str {
			return true
		}
		for _, l := range t.Lits {
			if l == v {
				return true
			}
		}
		return false
	case []any:
		if t.List == nil {
			return false
		}
		for _, x := range v {
			if !Conforms(x, t.List) {
				return false
			}
		}
		return true
	case map[string]any:
		if n, ok := OpaqueName(v); ok {
			for _, o := range t.Opq {
				if o == n {
					return true
				}
			}
			return false
		}
		rec := t.Rec
		if len(t.Union) > 0 {
			// f05-unions.md 7.1: the tag first, then that variant.
			tv, ok := v[t.Tag].(string)
			if !ok {
				return false
			}
			rec = t.variantOf(tv)
		}
		if rec == nil {
			return false
		}
		return conformsRecord(v, rec)
	}
	return false
}

func conformsRecord(v map[string]any, rec *Record) bool {
	{
		for _, f := range rec.Fields {
			x, ok := v[f.Key]
			if !ok {
				if !f.Opt {
					return false
				}
				continue
			}
			if !Conforms(x, f.T) {
				return false
			}
		}
		for k, x := range v {
			if rec.field(k) != nil {
				continue
			}
			if rec.Rest == nil || !Conforms(x, rec.Rest) {
				return false
			}
		}
		return true
	}
}

// String prints a type as 05-types.md 3.9 says.
func (t *Type) String() string {
	if t.Any {
		return "any"
	}
	if t.IsNever() {
		return "never"
	}
	var alts []string
	if t.Bool {
		alts = append(alts, "bool")
	}
	switch t.Num {
	case 1:
		alts = append(alts, "int")
	case 2:
		alts = append(alts, "float")
	}
	if t.Str {
		alts = append(alts, "string")
	} else {
		for _, l := range t.Lits {
			alts = append(alts, jsonString(l))
		}
	}
	if t.List != nil {
		el := t.List.String()
		if topBar(el) {
			el = "(" + el + ")"
		}
		alts = append(alts, el+"[]")
	}
	if t.Rec != nil {
		alts = append(alts, t.Rec.String())
	}
	if len(t.Union) > 0 {
		// f05-unions.md 2.4: by the least string of each variant's tag set.
		vs := append([]*Record(nil), t.Union...)
		least := func(r *Record) string {
			ls := append([]string(nil), r.field(t.Tag).T.Lits...)
			sort.Strings(ls)
			return ls[0]
		}
		sort.Slice(vs, func(i, j int) bool { return least(vs[i]) < least(vs[j]) })
		for _, r := range vs {
			alts = append(alts, r.String())
		}
	}
	alts = append(alts, t.Opq...)
	if t.Null {
		alts = append(alts, "null")
	}
	return strings.Join(alts, " | ")
}

func (r *Record) String() string {
	if len(r.Fields) == 0 {
		if r.Rest == nil {
			return "{}"
		}
		return "map<" + r.Rest.String() + ">"
	}
	parts := make([]string, 0, len(r.Fields)+1)
	fs := append([]Field(nil), r.Fields...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].Key < fs[j].Key })
	for _, f := range fs {
		k := f.Key
		if !isWord(k) {
			k = jsonString(k)
		}
		if f.Opt {
			k += "?"
		}
		parts = append(parts, k+": "+f.T.String())
	}
	switch {
	case r.Rest == nil:
	case r.Rest.Any:
		parts = append(parts, "...")
	default:
		parts = append(parts, "..."+r.Rest.String())
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func topBar(s string) bool {
	depth := 0
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr:
			if c == '\\' {
				i++
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '(' || c == '{' || c == '<' || c == '[':
			depth++
		case c == ')' || c == '}' || c == '>' || c == ']':
			depth--
		case c == '|' && depth == 0:
			return true
		}
	}
	return false
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Parse reads type text: the grammar of 05-types.md 3.1 without Var and
// Sig. An opaque Name is kept as a name.
func Parse(text string) (*Type, error) {
	p := &parser{s: text}
	t, err := p.typ()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i != len(p.s) {
		return nil, fmt.Errorf("type text %q: text left at %d", text, p.i)
	}
	return t, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) space() {
	for p.i < len(p.s) && strings.IndexByte(" \t\n\r", p.s[p.i]) >= 0 {
		p.i++
	}
}

func (p *parser) eat(tok string) bool {
	p.space()
	if strings.HasPrefix(p.s[p.i:], tok) {
		p.i += len(tok)
		return true
	}
	return false
}

func (p *parser) word() string {
	p.space()
	j := p.i
	for j < len(p.s) && (p.s[j] == '_' || p.s[j] >= 'a' && p.s[j] <= 'z' || p.s[j] >= 'A' && p.s[j] <= 'Z' || j > p.i && p.s[j] >= '0' && p.s[j] <= '9') {
		j++
	}
	w := p.s[p.i:j]
	p.i = j
	return w
}

func (p *parser) str() (string, error) {
	p.space()
	if p.i >= len(p.s) || p.s[p.i] != '"' && p.s[p.i] != '\'' {
		return "", fmt.Errorf("want a string at %d", p.i)
	}
	q := p.s[p.i]
	j := p.i + 1
	for j < len(p.s) && p.s[j] != q {
		if p.s[j] == '\\' {
			j++
		}
		j++
	}
	if j >= len(p.s) {
		return "", fmt.Errorf("unterminated string")
	}
	raw := p.s[p.i+1 : j]
	p.i = j + 1
	if q == '\'' {
		raw = strings.ReplaceAll(strings.ReplaceAll(raw, `\'`, `'`), `"`, `\"`)
	}
	var out string
	if err := json.Unmarshal([]byte(`"`+raw+`"`), &out); err != nil {
		return "", fmt.Errorf("string %q: %w", raw, err)
	}
	return out, nil
}

// typ reads a Type: its alternatives join, and its records form a tagged
// union or are an error (f05-unions.md 3.1).
func (p *parser) typ() (*Type, error) {
	t := Never
	var recs []*Record
	for {
		u, err := p.alt()
		if err != nil {
			return nil, err
		}
		recs = append(recs, u.records()...)
		u = u.Copy()
		u.Rec, u.Union, u.Tag = nil, nil, ""
		t = Join(t, u)
		if !p.eat("|") {
			break
		}
	}
	switch {
	case t.Any:
		return Any, nil
	case len(recs) == 1:
		t = t.Copy()
		t.Rec = recs[0]
	case len(recs) > 1:
		tag, ok := FindTag(recs)
		if !ok {
			return nil, fmt.Errorf("records that form no tagged union")
		}
		t = t.Copy()
		t.Union, t.Tag = recs, tag
	}
	return t, nil
}

func (p *parser) alt() (*Type, error) {
	t, err := p.prim()
	if err != nil {
		return nil, err
	}
	for {
		save := p.i
		if p.eat("[") && p.eat("]") {
			t = ListOf(t)
			continue
		}
		p.i = save
		return t, nil
	}
}

func (p *parser) prim() (*Type, error) {
	p.space()
	if p.i >= len(p.s) {
		return nil, fmt.Errorf("type text ends early")
	}
	switch c := p.s[p.i]; {
	case c == '(':
		p.i++
		t, err := p.typ()
		if err != nil {
			return nil, err
		}
		if !p.eat(")") {
			return nil, fmt.Errorf("want )")
		}
		return t, nil
	case c == '"' || c == '\'':
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		return &Type{Lits: []string{s}}, nil
	case c == '{':
		p.i++
		return p.record()
	}
	w := p.word()
	switch w {
	case "null", "bool", "int", "float", "string", "any":
		return Prim(w), nil
	case "never":
		return Never, nil
	case "map":
		if !p.eat("<") {
			return nil, fmt.Errorf("want <")
		}
		t, err := p.typ()
		if err != nil {
			return nil, err
		}
		if !p.eat(">") {
			return nil, fmt.Errorf("want >")
		}
		return &Type{Rec: &Record{Rest: t}}, nil
	case "":
		return nil, fmt.Errorf("want a type at %d", p.i)
	}
	if len(w) >= 2 && w[0] >= 'A' && w[0] <= 'Z' {
		return &Type{Opq: []string{w}}, nil
	}
	return nil, fmt.Errorf("not a type word: %q", w)
}

func (p *parser) record() (*Type, error) {
	r := &Record{}
	seen := map[string]bool{}
	for {
		if p.eat("}") {
			break
		}
		if p.eat("...") {
			p.space()
			if p.i < len(p.s) && (p.s[p.i] == ',' || p.s[p.i] == '}') {
				r.Rest = Any
			} else {
				t, err := p.typ()
				if err != nil {
					return nil, err
				}
				r.Rest = t
			}
		} else {
			var key string
			p.space()
			if p.i < len(p.s) && (p.s[p.i] == '"' || p.s[p.i] == '\'') {
				s, err := p.str()
				if err != nil {
					return nil, err
				}
				key = s
			} else {
				key = p.word()
				if key == "" {
					return nil, fmt.Errorf("want a key at %d", p.i)
				}
			}
			if seen[key] {
				return nil, fmt.Errorf("key %q twice", key)
			}
			seen[key] = true
			f := Field{Key: key, Opt: p.eat("?")}
			if !p.eat(":") {
				return nil, fmt.Errorf("want : after %q", key)
			}
			t, err := p.typ()
			if err != nil {
				return nil, err
			}
			f.T = t
			r.Fields = append(r.Fields, f)
		}
		if p.eat("}") {
			break
		}
		if !p.eat(",") {
			return nil, fmt.Errorf("want , or } at %d", p.i)
		}
	}
	sort.Slice(r.Fields, func(i, j int) bool { return r.Fields[i].Key < r.Fields[j].Key })
	return &Type{Rec: r}, nil
}
