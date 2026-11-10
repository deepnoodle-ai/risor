// Package jsonschema maps a JSON Schema to a type, by package F1 (spec/
// packages/f01-host-types.md, sections 3 and 4). A schema is a decoded JSON
// value: map[string]any, []any, float64, string, bool or nil.
package jsonschema

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v3/internal/types"
)

// Options are the three options of F1 4.
type Options struct {
	Closed          bool
	AbsentIsNull    bool
	DefaultsApplied bool
}

// Error is a schema the mapping refuses: the declare error bad-type.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

// MaxDepth is the limit on the depth of a schema and of its type (3.10).
const MaxDepth = 64

type result struct {
	t     *types.Type
	depth int
}

type mapper struct {
	root   any
	o      Options
	cache  map[string]result
	inprog map[string]bool
}

func (m *mapper) fail(format string, args ...any) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
}

// Map gives the type of a schema with the options (3.1). Close is applied
// once, to the type of the root (4.1).
func Map(root any, o Options) (t *types.Type, err error) {
	m := &mapper{root: root, o: o, cache: map[string]result{}, inprog: map[string]bool{}}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*Error)
			if !ok {
				panic(r)
			}
			t, err = nil, e
		}
	}()
	m.inprog[""] = true
	r := m.schema(root, true)
	t = types.Close(r.t)
	if types.Depth(t) > MaxDepth {
		m.fail("the type is deeper than %d", MaxDepth)
	}
	return t, nil
}

func isSchema(v any) bool {
	switch v.(type) {
	case bool, map[string]any:
		return true
	}
	return false
}

// schema is M(S) (3.1). It returns the type and the depth of S (3.10).
func (m *mapper) schema(s any, root bool) result {
	var res result
	switch s := s.(type) {
	case bool:
		if s {
			return result{t: types.Any}
		}
		return result{t: types.Never}
	case map[string]any:
		res = m.object(s, root)
	default:
		m.fail("a schema must be an object or a boolean")
	}
	if res.depth > MaxDepth {
		m.fail("the schema is deeper than %d", MaxDepth)
	}
	return res
}

func (m *mapper) object(s map[string]any, root bool) result {
	if _, ok := s["$id"]; ok && !root {
		m.fail("$id below the root is not read")
	}
	if _, ok := s["$dynamicRef"]; ok {
		m.fail("$dynamicRef is not read")
	}
	if _, ok := s["$recursiveRef"]; ok {
		m.fail("$recursiveRef is not read")
	}
	nullable := s["nullable"] == true
	if ref, ok := s["$ref"]; ok {
		r := m.ref(ref)
		t := r.t
		if nullable {
			t = types.Join(t, types.Null)
		}
		return result{t: t, depth: r.depth + 1}
	}
	var parts []*types.Type
	depth := -1
	add := func(r result) *types.Type {
		depth = max(depth, r.depth)
		return r.t
	}
	_, hasType := s["type"]
	_, hasEnum := s["enum"]
	_, hasConst := s["const"]
	if hasType || hasEnum || hasConst {
		parts = append(parts, m.kindPart(s, add))
	}
	if v, ok := s["allOf"]; ok {
		parts = append(parts, pick(m.branches("allOf", v, add)))
	}
	for _, k := range []string{"anyOf", "oneOf"} {
		if v, ok := s[k]; ok {
			bs := m.branches(k, v, add)
			m.tagTest(bs)
			t := types.Never
			for _, b := range bs {
				t = types.Join(t, b)
			}
			parts = append(parts, t)
		}
	}
	t := types.Any
	if len(parts) > 0 {
		t = pick(parts)
	}
	if nullable {
		t = types.Join(t, types.Null)
	}
	return result{t: t, depth: depth + 1}
}

func (m *mapper) branches(k string, v any, add func(result) *types.Type) []*types.Type {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		m.fail("%s must be a non-empty array of schemas", k)
	}
	out := make([]*types.Type, len(arr))
	for i, b := range arr {
		out[i] = add(m.schema(b, false))
	}
	return out
}

// tagTest refuses branches whose records would form a tagged union, until
// F5 (3.9).
func (m *mapper) tagTest(bs []*types.Type) {
	var recs []*types.Record
	for _, b := range bs {
		if r := b.Record(); r != nil {
			recs = append(recs, r)
		}
	}
	if len(recs) < 2 {
		return
	}
	for _, f := range recs[0].Fields {
		seen := map[string]bool{}
		tag := true
		for _, r := range recs {
			g, ok := r.Field(f.Key)
			if !ok || g.Optional || g.Type.Kinds() != 1 || g.Type.Lits() == nil {
				tag = false
				break
			}
			for _, l := range g.Type.Lits() {
				if seen[l] {
					tag = false
				}
				seen[l] = true
			}
			if !tag {
				break
			}
		}
		if tag {
			m.fail("the branches have the tag %q: a tagged union needs package F5", f.Key)
		}
	}
}

var typeNames = map[string]bool{"null": true, "boolean": true, "integer": true, "number": true,
	"string": true, "array": true, "object": true}

// names reads the keyword type (3.3).
func (m *mapper) names(v any) []string {
	var out []string
	switch v := v.(type) {
	case string:
		out = []string{v}
	case []any:
		if len(v) == 0 {
			m.fail("type must not be an empty array")
		}
		for _, e := range v {
			s, ok := e.(string)
			if !ok {
				m.fail("type must hold strings")
			}
			out = append(out, s)
		}
	default:
		m.fail("type must be a string or an array of strings")
	}
	for _, n := range out {
		if !typeNames[n] {
			m.fail("%q is not a type name", n)
		}
	}
	return out
}

func allows(names []string, v any) bool {
	if names == nil {
		return true
	}
	for _, n := range names {
		switch x := v.(type) {
		case nil:
			if n == "null" {
				return true
			}
		case bool:
			if n == "boolean" {
				return true
			}
		case float64:
			if n == "number" || (n == "integer" && x == math.Trunc(x)) {
				return true
			}
		case string:
			if n == "string" {
				return true
			}
		case []any:
			if n == "array" {
				return true
			}
		case map[string]any:
			if n == "object" {
				return true
			}
		}
	}
	return false
}

// Literal is the literal type of a JSON value (3.4).
func Literal(v any) *types.Type {
	switch x := v.(type) {
	case nil:
		return types.Null
	case bool:
		return types.Bool
	case float64:
		if x == math.Trunc(x) && math.Abs(x) <= 9007199254740991 {
			return types.Int
		}
		return types.Float
	case string:
		return types.Lit(x)
	case []any:
		e := types.Never
		for _, el := range x {
			e = types.Join(e, Literal(el))
		}
		return types.List(e)
	case map[string]any:
		fs := make([]types.Field, 0, len(x))
		for k, el := range x {
			fs = append(fs, types.Field{Key: k, Type: Literal(el)})
		}
		return types.Rec(fs, nil)
	}
	return types.Any
}

// kindPart is the kind part of S (3.2).
func (m *mapper) kindPart(s map[string]any, add func(result) *types.Type) *types.Type {
	var names []string
	if v, ok := s["type"]; ok {
		names = m.names(v)
	}
	var values []any
	hasValues := false
	if v, ok := s["const"]; ok {
		values, hasValues = []any{v}, true
	} else if v, ok := s["enum"]; ok {
		arr, ok := v.([]any)
		if !ok {
			m.fail("enum must be an array")
		}
		values, hasValues = arr, true
	}
	if hasValues {
		t := types.Never
		for _, v := range values {
			if allows(names, v) {
				t = types.Join(t, Literal(v))
			}
		}
		return t
	}
	t := types.Never
	for _, n := range names {
		switch n {
		case "null":
			t = types.Join(t, types.Null)
		case "boolean":
			t = types.Join(t, types.Bool)
		case "integer":
			t = types.Join(t, types.Int)
		case "number":
			t = types.Join(t, types.Float)
		case "string":
			t = types.Join(t, types.String)
		case "array":
			t = types.Join(t, types.List(m.items(s, add)))
		case "object":
			t = types.Join(t, types.RecType(m.record(s, add)))
		}
	}
	return t
}

// items is the element type of an array (3.5).
func (m *mapper) items(s map[string]any, add func(result) *types.Type) *types.Type {
	e := types.Never
	schemaList := func(k string, v any) {
		arr, ok := v.([]any)
		if !ok {
			m.fail("%s must be an array of schemas", k)
		}
		for _, x := range arr {
			if !isSchema(x) {
				m.fail("an element of %s is not a schema", k)
			}
			e = types.Join(e, add(m.schema(x, false)))
		}
	}
	if v, ok := s["prefixItems"]; ok {
		schemaList("prefixItems", v)
	}
	v, ok := s["items"]
	switch {
	case !ok:
		e = types.Join(e, types.Any)
	case isSchema(v):
		e = types.Join(e, add(m.schema(v, false)))
	default:
		if _, isArr := v.([]any); !isArr {
			m.fail("items must be a schema or an array of schemas")
		}
		schemaList("items", v)
		if ai, ok := s["additionalItems"]; ok {
			if !isSchema(ai) {
				m.fail("additionalItems must be a schema")
			}
			e = types.Join(e, add(m.schema(ai, false)))
		} else {
			e = types.Join(e, types.Any)
		}
	}
	return e
}

// record is the record of an object (3.8, 4).
func (m *mapper) record(s map[string]any, add func(result) *types.Type) *types.Record {
	r0 := types.Any
	ap, hasAP := s["additionalProperties"]
	_, hasPP := s["patternProperties"]
	if hasAP {
		r0 = add(m.schema(ap, false))
		if pp, ok := s["patternProperties"]; ok {
			obj, ok := pp.(map[string]any)
			if !ok {
				m.fail("patternProperties must be an object of schemas")
			}
			for _, k := range sortedKeys(obj) {
				if !isSchema(obj[k]) {
					m.fail("a value of patternProperties is not a schema")
				}
				r0 = types.Join(r0, add(m.schema(obj[k], false)))
			}
		}
	}
	required := map[string]bool{}
	var reqOrder []string
	if v, ok := s["required"]; ok {
		arr, ok := v.([]any)
		if !ok {
			m.fail("required must be an array of strings")
		}
		for _, x := range arr {
			n, ok := x.(string)
			if !ok {
				m.fail("required must hold strings")
			}
			if !required[n] {
				reqOrder = append(reqOrder, n)
			}
			required[n] = true
		}
	}
	var fields []types.Field
	propsV, hasProps := s["properties"]
	props := map[string]any{}
	if hasProps {
		obj, ok := propsV.(map[string]any)
		if !ok {
			m.fail("properties must be an object of schemas")
		}
		props = obj
		for _, k := range sortedKeys(obj) {
			ps := obj[k]
			if !isSchema(ps) {
				m.fail("the property %q is not a schema", k)
			}
			t := add(m.schema(ps, false))
			f := types.Field{Key: k, Type: t, Optional: !required[k]}
			if f.Optional {
				def, hasDef := any(nil), false
				if po, ok := ps.(map[string]any); ok {
					def, hasDef = po["default"]
				}
				switch {
				case m.o.DefaultsApplied && hasDef:
					f.Optional, f.Type = false, types.Join(t, Literal(def))
				case m.o.AbsentIsNull:
					f.Optional, f.Type = false, types.Join(t, types.Null)
				}
			}
			fields = append(fields, f)
		}
	}
	for _, n := range reqOrder {
		if _, ok := props[n]; !ok {
			fields = append(fields, types.Field{Key: n, Type: r0})
		}
	}
	rec := types.Rec(fields, r0).Record()
	rec.Closable = m.o.Closed && hasProps && !hasAP && !hasPP
	return rec
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// ref resolves and maps a $ref (3.7). A target is mapped once and shared;
// a target in progress is a cycle.
func (m *mapper) ref(v any) result {
	s, ok := v.(string)
	if !ok {
		m.fail("$ref must be a string")
	}
	target, key := m.resolve(s)
	if m.inprog[key] {
		m.fail("the $ref %q is a cycle", s)
	}
	if r, ok := m.cache[key]; ok {
		return r
	}
	m.inprog[key] = true
	r := m.schema(target, false)
	delete(m.inprog, key)
	m.cache[key] = r
	return r
}

// resolve reads a local JSON Pointer fragment (3.7 items 1 to 6).
func (m *mapper) resolve(ref string) (any, string) {
	if !strings.HasPrefix(ref, "#") {
		m.fail("the $ref %q is not local", ref)
	}
	frag, ok := percentDecode(ref[1:])
	if !ok {
		m.fail("the $ref %q is not well percent-encoded", ref)
	}
	if frag == "" {
		return m.root, ""
	}
	if frag[0] != '/' {
		m.fail("the $ref %q is not a JSON Pointer", ref)
	}
	cur := m.root
	var key strings.Builder
	for _, raw := range strings.Split(frag[1:], "/") {
		tok, ok := unescapeToken(raw)
		if !ok {
			m.fail("the $ref %q has a bad escape", ref)
		}
		key.WriteString("\x00" + tok)
		switch c := cur.(type) {
		case map[string]any:
			next, ok := c[tok]
			if !ok {
				m.fail("the $ref %q names nothing", ref)
			}
			cur = next
		case []any:
			i, ok := arrayIndex(tok, len(c))
			if !ok {
				m.fail("the $ref %q names nothing", ref)
			}
			cur = c[i]
		default:
			m.fail("the $ref %q names nothing", ref)
		}
	}
	if !isSchema(cur) {
		m.fail("the $ref %q names what is not a schema", ref)
	}
	return cur, key.String()
}

func percentDecode(s string) (string, bool) {
	if !strings.Contains(s, "%") {
		return s, true
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b = append(b, s[i])
			continue
		}
		if i+2 >= len(s) {
			return "", false
		}
		n, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
		if err != nil || !isHex(s[i+1]) || !isHex(s[i+2]) {
			return "", false
		}
		b = append(b, byte(n))
		i += 2
	}
	if !utf8.Valid(b) {
		return "", false
	}
	return string(b), true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func unescapeToken(t string) (string, bool) {
	if !strings.Contains(t, "~") {
		return t, true
	}
	var b strings.Builder
	for i := 0; i < len(t); i++ {
		if t[i] != '~' {
			b.WriteByte(t[i])
			continue
		}
		if i+1 >= len(t) {
			return "", false
		}
		switch t[i+1] {
		case '0':
			b.WriteByte('~')
		case '1':
			b.WriteByte('/')
		default:
			return "", false
		}
		i++
	}
	return b.String(), true
}

func arrayIndex(t string, n int) (int, bool) {
	if t == "" || (len(t) > 1 && t[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(t); i++ {
		if t[i] < '0' || t[i] > '9' {
			return 0, false
		}
	}
	i, err := strconv.Atoi(t)
	if err != nil || i >= n {
		return 0, false
	}
	return i, true
}

// pick is Pick of 3.6 item 2.
func pick(ts []*types.Type) *types.Type {
	var left []*types.Type
	for _, t := range ts {
		if !t.IsAny() {
			left = append(left, t)
		}
	}
	if len(left) == 0 {
		return types.Any
	}
	allRec := true
	for _, t := range left {
		if t.Kinds() != 1 || t.Record() == nil {
			allRec = false
			break
		}
	}
	if allRec {
		acc := left[0]
		for _, t := range left[1:] {
			acc = types.RecType(meet(acc.Record(), t.Record()))
		}
		return acc
	}
	for i, t := range left {
		ok := true
		for j, u := range left {
			if i == j {
				continue
			}
			if f, used := types.Fits(t, u); !f || used {
				ok = false
				break
			}
		}
		if ok {
			return t
		}
	}
	return left[0]
}

// meet is Meet of 3.6 item 3.
func meet(a, b *types.Record) *types.Record {
	keys := map[string]bool{}
	for _, f := range a.Fields {
		keys[f.Key] = true
	}
	for _, f := range b.Fields {
		keys[f.Key] = true
	}
	var fields []types.Field
	for k := range keys {
		fa, inA := a.Field(k)
		fb, inB := b.Field(k)
		ta, tb := a.Rest, b.Rest
		if inA {
			ta = fa.Type
		}
		if inB {
			tb = fb.Type
		}
		req := (inA && !fa.Optional) || (inB && !fb.Optional)
		fields = append(fields, types.Field{Key: k, Type: pick([]*types.Type{ta, tb}), Optional: !req})
	}
	r := types.Rec(fields, pick([]*types.Type{a.Rest, b.Rest})).Record()
	r.Closable = a.Closable && b.Closable
	return r
}
