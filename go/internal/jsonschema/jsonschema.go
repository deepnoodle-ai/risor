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

	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
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
	root  any
	o     Options
	cache map[string]result
	// inprog holds each place being mapped, with the count of guards open
	// when its mapping began (F17 12.4). A place is the root "", or the
	// tokens of its pointer, each after a zero byte; at is the place being
	// mapped now.
	inprog map[string]int
	at     string
	// guard counts the guards open: array items, properties and maps.
	guard int
	// rootName names the reference to the root (F17 12.4).
	rootName string
	// named holds the reference of each target met while it was being
	// mapped, names the target of each name, order the targets in the
	// order named, and defs the type each one mapped to.
	named    map[string]*types.Named
	refNames map[string]string
	order    []string
	defs     map[string]*types.Type
}

func (m *mapper) fail(format string, args ...any) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
}

// Map gives the type of a schema with the options (3.1). Close is applied
// once, to the type of the root (4.1). A target of $ref met again while it
// is being mapped is a recursive type (F17 12.4); rootName names the
// reference to the root, "Root" when empty.
func Map(root any, o Options, rootName string) (t *types.Type, err error) {
	if rootName == "" {
		rootName = "Root"
	}
	m := &mapper{root: root, o: o, cache: map[string]result{}, inprog: map[string]int{}, rootName: rootName,
		named: map[string]*types.Named{}, refNames: map[string]string{}, defs: map[string]*types.Type{}}
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case *Error:
				t, err = nil, e
			case *types.Incomplete:
				// F17 12.4: Pick, Meet or a tag would look into a schema
				// still being mapped.
				t, err = nil, &Error{Msg: fmt.Sprintf("%s is combined by allOf, or tested for a tag, inside its own definition", e.Name)}
			case *types.PairLimit:
				t, err = nil, &Error{Msg: fmt.Sprintf("the schema needs more than %d pairs of types to compare", types.MaxPairs)}
			default:
				panic(r)
			}
		}
	}()
	t = m.schema(root, "").t
	m.setDefs()
	if len(m.order) > 0 {
		// F17 12.4 item 5 (R316): the schema's own type follows 12.2 item 6
		// too, so that every type a description prints reads back.
		if n, bad := types.SharedDeep(t); bad {
			m.fail("%s shares a kind with another type of its union; give the union a definition of its own", n.Name)
		}
	}
	t = types.Close(t)
	if types.Size(t) > types.MaxSize {
		m.fail("the type is larger than %d", types.MaxSize)
	}
	if types.Depth(t) > MaxDepth {
		m.fail("the type is deeper than %d", MaxDepth)
	}
	return t, nil
}

// setDefs sets the definition of each reference the mapping made: its
// target's type, with the references at its top replaced by theirs. Each
// is tested as a type line is (F17 12.2 items 6, 12.6).
func (m *mapper) setDefs() {
	if len(m.order) == 0 {
		return
	}
	ns := make([]*types.Named, len(m.order))
	defs := make([]*types.Type, len(m.order))
	for i, k := range m.order {
		ns[i], defs[i] = m.named[k], m.defs[k]
	}
	if n, bad := types.SetDefs(ns, defs); bad {
		m.fail("%s shares a kind with another type of its union; give the union a definition of its own", n.Name)
	}
	for _, n := range ns {
		if types.Size(n.Def()) > types.MaxSize {
			m.fail("the type of %s is larger than %d", n.Name, types.MaxSize)
		}
		if types.Depth(n.Def()) > MaxDepth {
			m.fail("the type of %s is deeper than %d", n.Name, MaxDepth)
		}
	}
}

// nameOf is the reference to a target met while it is being mapped (F17
// 12.4): named by the last token of its pointer with its first letter
// upper-cased, or by rootName for the root.
func (m *mapper) nameOf(key, ref string) *types.Named {
	if n := m.named[key]; n != nil {
		return n
	}
	name := m.rootName
	if key != "" {
		tok := lastTok(key)
		if tok != "" && tok[0] >= 'a' && tok[0] <= 'z' {
			tok = string(tok[0]-'a'+'A') + tok[1:]
		}
		name = tok
	}
	if !isTypeName(name) {
		m.fail("the $ref %q makes a recursive type, and %q is not a type name", ref, name)
	}
	if _, clash := m.refNames[name]; clash {
		m.fail("two recursive types of the schema are named %s", name)
	}
	n := types.NewNamed(name)
	m.named[key], m.refNames[name] = n, key
	m.order = append(m.order, key)
	return n
}

// isTypeName reports an upper-case letter followed by one or more of
// [A-Za-z0-9_] (F4 3.2).
func isTypeName(s string) bool {
	if len(s) < 2 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func isSchema(v any) bool {
	switch v.(type) {
	case bool, map[string]any:
		return true
	}
	return false
}

// schema is M(S) (3.1) of the schema s at the place key. It returns the
// type and the depth of S (3.10). A place is mapped once, and its type is
// shared however the mapping reaches it: through a $ref, or as a part of
// the schema that holds it, the root included. A place that a $ref meets
// again while it is being mapped is a recursive type (F17 12.4).
func (m *mapper) schema(s any, key string) result {
	if r, ok := m.cache[key]; ok {
		return r
	}
	if g, ok := m.inprog[key]; ok {
		// Met again while it is being mapped, as a part of the schema that
		// holds it: the place a $ref reached first is the recursive type
		// (F17 12.4 item 1).
		ptr := pointerOf(key)
		if m.guard == g {
			m.fail("the schema at %q holds itself with no array item, property or map between", ptr)
		}
		return result{t: types.Ref(m.nameOf(key, ptr))}
	}
	saved := m.at
	m.at = key
	m.inprog[key] = m.guard
	r := m.schemaAt(s, key == "")
	delete(m.inprog, key)
	m.at = saved
	if n := m.named[key]; n != nil {
		m.defs[key] = r.t
		r.t = types.Ref(n)
	}
	m.cache[key] = r
	return r
}

// child is the place of a schema inside the one being mapped: under the
// keyword k, then at each member or index of tok.
func (m *mapper) child(k string, tok ...string) string {
	key := m.at + keyTok(k)
	for _, t := range tok {
		key += keyTok(t)
	}
	return key
}

// keyTok is one token of a place's key: a NUL, then the token with each
// NUL and each \x01 in it escaped, so that no two paths share a key.
func keyTok(t string) string {
	if strings.ContainsAny(t, "\x00\x01") {
		t = strings.NewReplacer("\x01", "\x01\x01", "\x00", "\x01\x02").Replace(t)
	}
	return "\x00" + t
}

// pointerOf is the JSON Pointer fragment of a place's key, for messages.
func pointerOf(key string) string {
	if key == "" {
		return "#"
	}
	var b strings.Builder
	b.WriteByte('#')
	for _, t := range strings.Split(key[1:], "\x00") {
		t = lastTok("\x00" + t)
		b.WriteByte('/')
		b.WriteString(strings.NewReplacer("~", "~0", "/", "~1").Replace(t))
	}
	return b.String()
}

// lastTok is the last token of a place's key, unescaped.
func lastTok(key string) string {
	t := key[strings.LastIndexByte(key, 0)+1:]
	if strings.IndexByte(t, 1) >= 0 {
		t = strings.NewReplacer("\x01\x01", "\x01", "\x01\x02", "\x00").Replace(t)
	}
	return t
}

// schemaAt is M(S) of the schema at the place being mapped.
func (m *mapper) schemaAt(s any, root bool) result {
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
	// 05-types.md 20.7: a type over the size limit is bad-type.
	// Each part is tested as it is mapped, so no operation on it (Pick,
	// Meet, Join, Close) works on a type past the limit.
	if types.Size(res.t) > types.MaxSize {
		m.fail("the type is larger than %d", types.MaxSize)
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
			parts = append(parts, branchesJoin(m.branches(k, v, add)))
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
		out[i] = add(m.schema(b, m.child(k, strconv.Itoa(i))))
	}
	return out
}

// branchesJoin is the part of an anyOf or oneOf (F5 8.1): when the records
// of the branches form a tagged union, a type whose map alternative is that
// union and whose other alternatives are the Join of the branches' others;
// otherwise the Join of the branches. A reference stands beside the
// records, as in type text, unless it shares a kind with another branch
// and its definition is known (F17 12.4).
func branchesJoin(bs []*types.Type) *types.Type {
	if t, ok := types.Alternatives3(bs); ok {
		return t
	}
	return types.JoinAll(bs)
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
		es := make([]*types.Type, len(x))
		for i, el := range x {
			es[i] = Literal(el)
		}
		return types.List(types.JoinAll(es))
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
		var ts []*types.Type
		for _, v := range values {
			if allows(names, v) {
				ts = append(ts, Literal(v))
			}
		}
		return types.JoinAll(ts)
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

// items is the element type of an array (3.5). An item is a guarded place
// (F17 12.4).
func (m *mapper) items(s map[string]any, add func(result) *types.Type) *types.Type {
	m.guard++
	defer func() { m.guard-- }()
	e := types.Never
	var es []*types.Type // the items of a list, joined once, not once for each item
	schemaList := func(k string, v any) {
		arr, ok := v.([]any)
		if !ok {
			m.fail("%s must be an array of schemas", k)
		}
		for i, x := range arr {
			if !isSchema(x) {
				m.fail("an element of %s is not a schema", k)
			}
			es = append(es, add(m.schema(x, m.child(k, strconv.Itoa(i)))))
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
		e = types.Join(e, add(m.schema(v, m.child("items"))))
	default:
		if _, isArr := v.([]any); !isArr {
			m.fail("items must be a schema or an array of schemas")
		}
		schemaList("items", v)
		if ai, ok := s["additionalItems"]; ok {
			if !isSchema(ai) {
				m.fail("additionalItems must be a schema")
			}
			e = types.Join(e, add(m.schema(ai, m.child("additionalItems"))))
		} else {
			e = types.Join(e, types.Any)
		}
	}
	return types.Join(e, types.JoinAll(es))
}

// record is the record of an object (3.8, 4). A property and a map's value
// are guarded places (F17 12.4).
func (m *mapper) record(s map[string]any, add func(result) *types.Type) *types.Record {
	m.guard++
	defer func() { m.guard-- }()
	r0 := types.Any
	ap, hasAP := s["additionalProperties"]
	_, hasPP := s["patternProperties"]
	if hasAP {
		r0 = add(m.schema(ap, m.child("additionalProperties")))
		if pp, ok := s["patternProperties"]; ok {
			obj, ok := pp.(map[string]any)
			if !ok {
				m.fail("patternProperties must be an object of schemas")
			}
			for _, k := range sortedKeys(obj) {
				if !isSchema(obj[k]) {
					m.fail("a value of patternProperties is not a schema")
				}
				r0 = types.Join(r0, add(m.schema(obj[k], m.child("patternProperties", k))))
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
			t := add(m.schema(ps, m.child("properties", k)))
			f := types.Field{Key: k, Type: t, Optional: !required[k]}
			if f.Optional {
				def, hasDef := any(nil), false
				if po, ok := ps.(map[string]any); ok {
					def, hasDef = po["default"]
				}
				// F1 4.4: the fill is kept apart from the base until the
				// meets are done.
				var fill *types.Type
				switch {
				case m.o.DefaultsApplied && hasDef:
					fill = Literal(def)
				case m.o.AbsentIsNull:
					fill = types.Null
				}
				if fill != nil {
					f = types.Field{Key: k, Type: types.Join(t, fill), Base: t, Fill: fill}
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

// ref resolves and maps a $ref (3.7). A target met again while it is being
// mapped is a recursive type, named by nameOf, when an array item, a
// property or a map stands between; otherwise the cycle is an error (F17
// 12.4).
func (m *mapper) ref(v any) result {
	s, ok := v.(string)
	if !ok {
		m.fail("$ref must be a string")
	}
	target, key := m.resolve(s)
	if g, ok := m.inprog[key]; ok {
		if m.guard == g {
			m.fail("the $ref %q refers to itself with no array item, property or map between", s)
		}
		return result{t: types.Ref(m.nameOf(key, s))}
	}
	return m.schema(target, key)
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
		key.WriteString(keyTok(tok))
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
	if len(left) == 1 { // what steps 2 and 3 give, with no look inside it
		return left[0]
	}
	// Step 2 (F5 8.3): every type left is one map alternative, a record or a
	// union: their meet, folded from the left. A meet that fails sends the
	// whole list to step 3. The kinds of every type are read, so a
	// reference among them is looked into even after one that is not a map
	// (F17 12.4 item 4).
	allMap := true
	for _, t := range left {
		if t.Kinds() != 1 || t.Record() == nil {
			allMap = false
		}
	}
	if allMap {
		acc := left[0]
		ok := true
		for _, t := range left[1:] {
			if acc, ok = meetT(acc, t); !ok {
				break
			}
		}
		if ok {
			return acc
		}
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

// meetT meets two map alternatives (F5 8.3): two records by Meet; a record
// and a union by the meets with each variant, in the order the parts stand,
// when they form a tagged union; two unions fail.
func meetT(a, b *types.Type) (*types.Type, bool) {
	ua, ub := a.Union(), b.Union()
	switch {
	case ua == nil && ub == nil:
		return types.RecType(meet(a.Record(), b.Record())), true
	case ua != nil && ub != nil:
		return nil, false
	}
	var vs []*types.Record
	if ua != nil {
		for _, v := range ua.Variants {
			vs = append(vs, meet(v, b.Record()))
		}
	} else {
		for _, v := range ub.Variants {
			vs = append(vs, meet(a.Record(), v))
		}
	}
	u, ok := types.FormUnion(vs)
	return u, ok
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
			ta = fa.BaseOf()
		}
		if inB {
			tb = fb.BaseOf()
		}
		// F1 3.6 item 3: the base by Pick, the fill by Join.
		base := pick([]*types.Type{ta, tb})
		var fill *types.Type
		if (inA && fa.Fill != nil) || (inB && fb.Fill != nil) {
			fill = types.Never
			if inA && fa.Fill != nil {
				fill = types.Join(fill, fa.Fill)
			}
			if inB && fb.Fill != nil {
				fill = types.Join(fill, fb.Fill)
			}
		}
		req := (inA && !fa.Optional) || (inB && !fb.Optional) || fill != nil
		f := types.Field{Key: k, Type: base, Optional: !req}
		if fill != nil {
			f = types.Field{Key: k, Type: types.Join(base, fill), Base: base, Fill: fill}
		}
		fields = append(fields, f)
	}
	r := types.Rec(fields, pick([]*types.Type{a.Rest, b.Rest})).Record()
	r.Closable = a.Closable && b.Closable
	return r
}
