package risor

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/jsonschema"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is package F1, host types (spec/packages/f01-host-types.md):
// a variable's type from a JSON Schema, the description of an environment,
// and types derived from Go types.

// Type is a type of the language (05-types.md 2), the checker's own. Its
// String method prints it by 05-types.md 3.9, which is also its
// description (F1 5.1). The structural type API (typeapi.go) builds and
// reads one; the zero Type stands for never.
type Type struct {
	t *types.Type
	// rt is the Go type a type from TypeOf was derived from, for the
	// typed layer (package F11); nil for every other type.
	rt reflect.Type
	// err is set for an invalid type (Err).
	err error
}

// String prints the type.
func (t Type) String() string {
	if t.err != nil {
		return invalidTypeText
	}
	if t.t == nil {
		return "never"
	}
	return t.t.String()
}

// SchemaOption is one of the three options of F1 4: what the host did to a
// value before a run.
type SchemaOption string

// The options of a schema.
const (
	Closed          SchemaOption = "closed"
	AbsentIsNull    SchemaOption = "absent-is-null"
	DefaultsApplied SchemaOption = "defaults-applied"
)

func schemaOptions(opts []SchemaOption) (jsonschema.Options, error) {
	var o jsonschema.Options
	for _, x := range opts {
		switch x {
		case Closed:
			o.Closed = true
		case AbsentIsNull:
			o.AbsentIsNull = true
		case DefaultsApplied:
			o.DefaultsApplied = true
		default:
			return o, fmt.Errorf("%q is not a schema option", string(x))
		}
	}
	return o, nil
}

func decodeSchema(schema any) (any, error) {
	switch s := schema.(type) {
	case []byte:
		return casefile.DecodeJSON(s)
	case json.RawMessage:
		return casefile.DecodeJSON(s)
	}
	return schemaData(reflect.ValueOf(schema), 0)
}

// schemaData reads a schema given as Go values as JSON data: a map with
// string keys of any value type as map[string]any, a slice or an array of
// any element type as []any, a number of any Go type as float64, and a
// pointer or an interface as what it holds.
func schemaData(v reflect.Value, depth int) (any, error) {
	if depth > 1000 {
		return nil, errors.New("the schema nests too deep")
	}
	if !v.IsValid() {
		return nil, nil
	}
	if v.Type() == reflect.TypeFor[json.Number]() {
		f, err := strconv.ParseFloat(v.String(), 64)
		if err != nil || math.IsInf(f, 0) {
			return nil, fmt.Errorf("the schema number %s is not a finite number", v.String())
		}
		return f, nil
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil, nil
		}
		return schemaData(v.Elem(), depth)
	case reflect.Bool:
		return v.Bool(), nil
	case reflect.String:
		return v.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(v.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return v.Float(), nil
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return nil, nil
		}
		out := make([]any, v.Len())
		for i := range out {
			x, err := schemaData(v.Index(i), depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("a schema map has keys of type %v, not string", v.Type().Key())
		}
		if v.IsNil() {
			return nil, nil
		}
		out := make(map[string]any, v.Len())
		it := v.MapRange()
		for it.Next() {
			x, err := schemaData(it.Value(), depth+1)
			if err != nil {
				return nil, err
			}
			out[it.Key().String()] = x
		}
		return out, nil
	}
	return nil, fmt.Errorf("a schema holds a value of type %v", v.Type())
}

// TypeFromSchema maps a JSON Schema to a type (F1 3). schema is JSON text
// ([]byte or json.RawMessage), or the schema as Go values: nil, bool, a
// number of any Go type, string, a slice or array of any element type, and
// a map with string keys of any value type ([]string for required, a
// map[string]map[string]any for properties), at any depth. An error is an
// *Error of phase declare with the code bad-type.
func TypeFromSchema(schema any, opts ...SchemaOption) (Type, error) {
	t, err := mapSchema(schema, opts)
	if err != nil {
		return Type{}, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType, Message: err.Error()}}}
	}
	return Type{t: t}, nil
}

func mapSchema(schema any, opts []SchemaOption) (*types.Type, error) {
	o, err := schemaOptions(opts)
	if err != nil {
		return nil, err
	}
	s, err := decodeSchema(schema)
	if err != nil {
		return nil, err
	}
	return jsonschema.Map(s, o)
}

// VarSchema declares a variable whose type is given by a JSON Schema, with
// options (F1 2.1). An error of the schema is the declare error bad-type.
func VarSchema(name string, schema any, opts ...SchemaOption) Decl {
	return declFunc(func(e *Env) {
		t, err := mapSchema(schema, opts)
		if err != nil {
			e.errf(diag.CodeBadType, fmt.Sprintf("variable %s: %v", name, err))
			return
		}
		e.putVarType(name, t)
	})
}

// VarType declares a variable of a type: one the structural type API built
// (RecordType, ListType, ...), or one from TypeOf, TypeFromSchema or
// ParseType. Nothing is parsed: the type is the checker's own.
func VarType(name string, t Type) Decl {
	return declFunc(func(e *Env) {
		if p := t.problem(); p != "" {
			e.errf(diag.CodeBadType, fmt.Sprintf("variable %s: %s", name, p))
			return
		}
		e.putVarType(name, t.t)
		if d := e.vars[name]; d != nil && d.ty == t.t {
			d.rt = t.rt
		}
	})
}

func (e *Env) putVarType(name string, t *types.Type) {
	if !e.checkName(name, "variable") {
		return
	}
	if e.own["v:"+name] {
		e.errf(diag.CodeNameClash, fmt.Sprintf("variable %s is declared twice", name))
		return
	}
	e.own["v:"+name] = true
	e.vars[name] = &varDecl{name: name, ty: t}
}

// undeclaredOpaque reports the first opaque name in t that is not declared.
func undeclaredOpaque(t *types.Type, declared map[string]bool) (string, bool) {
	for _, n := range t.OpaqueNames() {
		if !declared[n] {
			return n, false
		}
	}
	if el := t.Elem(); el != nil {
		if n, ok := undeclaredOpaque(el, declared); !ok {
			return n, false
		}
	}
	if r := t.Record(); r != nil {
		for _, f := range r.Fields {
			if n, ok := undeclaredOpaque(f.Type, declared); !ok {
				return n, false
			}
		}
		return undeclaredOpaque(r.Rest, declared)
	}
	return "", true
}

// TypeOf derives the type of a Go type by the binding rules of F1 6.2: a
// type that implements Opaque gives its opaque name, asked of the type's
// zero value; one that implements encoding.TextMarshaler gives string; then
// bool, string, the integer and float types, slices and arrays, maps with
// string keys, pointers (T or null), every interface type (any), and
// structs (an exact record of the fields encoding/json would write).
// Anything else, a type that contains itself, and a type deeper than 64
// are errors with the code bad-type.
func TypeOf[T any]() (Type, error) {
	return typeOfReflect(reflect.TypeFor[T]())
}

func typeOfReflect(rt reflect.Type) (Type, error) {
	d := &deriver{inprog: map[reflect.Type]bool{}, done: map[reflect.Type]*types.Type{}}
	t, err := d.derive(rt)
	if err == nil && types.Depth(t) > types.MaxDepth {
		err = errors.New("the type is deeper than 64")
	}
	if err == nil && types.Size(t) > types.MaxSize {
		err = fmt.Errorf("the type is larger than %d", types.MaxSize)
	}
	if err != nil {
		return Type{}, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType,
			Message: fmt.Sprintf("%v: %v", rt, err)}}}
	}
	return Type{t: t, rt: rt}, nil
}

var (
	opaqueIface = reflect.TypeFor[value.Opaque]()
	textIface   = reflect.TypeFor[interface{ MarshalText() ([]byte, error) }]()
)

type deriver struct {
	inprog map[reflect.Type]bool
	// done shares the type of a Go type met twice, so a type that uses one
	// struct in many places is built once (05-types.md 20.7).
	done map[reflect.Type]*types.Type
}

func (d *deriver) derive(rt reflect.Type) (*types.Type, error) {
	if rt == nil || rt.Kind() == reflect.Interface {
		// F1 6.2: every interface type is any; the value it holds is presented
		// by the rules of its own type, a nil interface as null.
		return types.Any, nil
	}
	if rt.Implements(opaqueIface) {
		if rt.Kind() == reflect.Pointer && rt.Elem().Implements(opaqueIface) {
			// *T of an opaque T is T or null.
		} else {
			name := reflect.Zero(rt).Interface().(value.Opaque).OpaqueType()
			return types.Opaque(name), nil
		}
	}
	if rt.Implements(textIface) && !(rt.Kind() == reflect.Pointer && rt.Elem().Implements(textIface)) {
		return types.String, nil
	}
	switch rt.Kind() {
	case reflect.Bool:
		return types.Bool, nil
	case reflect.String:
		return types.String, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return types.Int, nil
	case reflect.Float32, reflect.Float64:
		return types.Float, nil
	case reflect.Slice, reflect.Array:
		el, err := d.nested(rt.Elem())
		if err != nil {
			return nil, err
		}
		return types.List(el), nil
	case reflect.Map:
		if rt.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("a map key must be a string, found %v", rt.Key())
		}
		el, err := d.nested(rt.Elem())
		if err != nil {
			return nil, err
		}
		return types.MapOf(el), nil
	case reflect.Pointer:
		el, err := d.nested(rt.Elem())
		if err != nil {
			return nil, err
		}
		return types.WithNull(el), nil
	case reflect.Struct:
		var fs []types.Field
		for _, f := range value.StructFields(rt) {
			ft, err := d.nested(f.Type)
			if err != nil {
				return nil, err
			}
			fs = append(fs, types.Field{Key: f.Key, Type: ft, Optional: f.Optional()})
		}
		return types.Rec(fs, nil), nil
	}
	return nil, fmt.Errorf("no rule of the binding covers %v", rt)
}

// nested derives a type inside another, refusing a type that contains
// itself (F1 6.1).
func (d *deriver) nested(rt reflect.Type) (*types.Type, error) {
	if d.inprog[rt] {
		return nil, fmt.Errorf("%v contains itself", rt)
	}
	if t, ok := d.done[rt]; ok {
		return t, nil
	}
	switch rt.Kind() {
	case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map, reflect.Pointer:
		d.inprog[rt] = true
		defer delete(d.inprog, rt)
	}
	t, err := d.derive(rt)
	if err == nil && types.Size(t) > types.MaxSize {
		err = fmt.Errorf("the type is larger than %d", types.MaxSize)
	}
	if err == nil {
		d.done[rt] = t
	}
	return t, err
}

// Describe gives the description of the environment (F1 5.3): a JSON
// object, as text in its JSON form (01-language.md 12), keys in code point
// order.
func (e *Env) Describe() ([]byte, error) {
	var b strings.Builder
	w := func(s string) { b.WriteString(s) }
	obj := func(keys []string, val func(k string)) {
		w("{")
		for i, k := range keys {
			if i > 0 {
				w(",")
			}
			w(value.QuoteJSON(k) + ":")
			val(k)
		}
		w("}")
	}
	sorted := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	arr := func(xs []string) {
		w("[")
		for i, x := range xs {
			if i > 0 {
				w(",")
			}
			w(value.QuoteJSON(x))
		}
		w("]")
	}
	funcNames := map[string]bool{}
	for n := range e.funcs {
		funcNames[n] = true
	}
	varNames := map[string]bool{}
	for n := range e.vars {
		varNames[n] = true
	}
	opNames := map[string]bool{}
	for n := range e.opaque {
		opNames[n] = true
	}
	removed := []string{}
	for n, r := range e.removed {
		if r {
			removed = append(removed, n)
		}
	}
	sort.Strings(removed)
	reject := append([]string(nil), e.reject...)
	if e.reject == nil {
		reject = []string{"constant-condition"}
	}
	sort.Strings(reject)
	limit := func(n int64) string {
		if n < 0 {
			return "null"
		}
		return fmt.Sprint(n)
	}
	top := []string{"functions", "limits", "opaque", "reject", "removed", "variables", "version"}
	if len(e.namedTypes) > 0 { // F4 8: only when a named type exists
		top = []string{"functions", "limits", "opaque", "reject", "removed", "types", "variables", "version"}
	}
	if len(e.groups) > 0 { // F6 2.7: only when a group is on
		top = append([]string{"functions", "groups"}, top[1:]...)
	}
	facts := e.describedFacts()
	if len(facts) > 0 { // F14 9.1: only when a host fact exists
		top = append([]string{"facts"}, top...)
	}
	obj(top, func(k string) {
		switch k {
		case "facts":
			w("[")
			for i, f := range facts {
				if i > 0 {
					w(",")
				}
				w(`{"someNotNull":`)
				arr(f)
				w("}")
			}
			w("]")
		case "functions":
			obj(sorted(funcNames), func(n string) { w(value.QuoteJSON(sigText(e.hostSigs[n]))) })
		case "limits":
			w(`{"allocation":` + limit(e.limits.Alloc) + `,"depth":` + fmt.Sprint(e.limits.Depth) +
				`,"steps":` + limit(e.limits.Steps) + `}`)
		case "opaque":
			obj(sorted(opNames), func(n string) {
				ms := map[string]bool{}
				for m := range e.methodSigs[n] {
					ms[m] = true
				}
				obj(sorted(ms), func(m string) { w(value.QuoteJSON(sigText(e.methodSigs[n][m]))) })
			})
		case "reject":
			arr(reject)
		case "removed":
			arr(removed)
		case "variables":
			obj(sorted(varNames), func(n string) { w(value.QuoteJSON(e.varTypes[n].String())) })
		case "groups":
			arr(e.groupList())
		case "types":
			tn := map[string]bool{}
			for n := range e.namedTypes {
				tn[n] = true
			}
			obj(sorted(tn), func(n string) { w(value.QuoteJSON(e.namedTypes[n].String())) })
		case "version":
			w(value.QuoteJSON(e.version))
		}
	})
	return []byte(b.String()), nil
}

// sigText prints a signature by F1 5.2.
func sigText(s *types.Sig) string {
	if s == nil {
		return ""
	}
	parts := make([]string, len(s.Params))
	for i, p := range s.Params {
		var b strings.Builder
		if p.Rest {
			b.WriteString("...")
		}
		b.WriteString(p.Name)
		if p.Optional {
			b.WriteString("?")
		}
		b.WriteString(": ")
		t := p.Type
		if p.Rest {
			t = types.List(t)
		}
		b.WriteString(t.String())
		parts[i] = b.String()
	}
	out := "(" + strings.Join(parts, ", ") + ") => " + s.Result.String()
	if s.Fails {
		out += " !"
	}
	if len(s.Bounds) > 0 {
		vs := make([]string, 0, len(s.Bounds))
		for v := range s.Bounds {
			vs = append(vs, v)
		}
		sort.Strings(vs)
		for i, v := range vs {
			vs[i] = v + ": " + s.Bounds[v]
		}
		out += " where " + strings.Join(vs, ", ")
	}
	return out
}

// Description builds the declarations of a description (F1 5.4). The
// environment it gives can check programs; a compile fails with
// no-implementation while it declares a host function or a method. A
// description that is not a JSON object of the form of 5.3 is an error of
// this binding, not a diagnostic.
func Description(data []byte) (Decl, error) {
	raw, err := casefile.DecodeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("risor: a description is not JSON: %w", err)
	}
	d, ok := raw.(map[string]any)
	keys := []string{"version", "variables", "functions", "opaque", "removed", "reject", "limits"}
	_, hasTypes := d["types"]
	_, hasGroups := d["groups"]
	_, hasFacts := d["facts"]
	if !ok || len(d) != len(keys)+boolInt(hasTypes)+boolInt(hasGroups)+boolInt(hasFacts) {
		return nil, errors.New("risor: a description is an object with exactly the keys of F1 5.3")
	}
	bad := func(k string) (Decl, error) {
		return nil, fmt.Errorf("risor: the description's %s is not of the form of F1 5.3", k)
	}
	for _, k := range keys {
		if _, ok := d[k]; !ok {
			return bad(k)
		}
	}
	var decls []Decl
	version, ok := d["version"].(string)
	if !ok {
		return bad("version")
	}
	decls = append(decls, Version(version))
	strMap := func(k string, v any) (map[string]string, bool) {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		out := map[string]string{}
		for n, x := range m {
			s, ok := x.(string)
			if !ok {
				return nil, false
			}
			out[n] = s
		}
		return out, true
	}
	vars, ok := strMap("variables", d["variables"])
	if !ok {
		return bad("variables")
	}
	for _, n := range sortedStrKeys(vars) {
		decls = append(decls, Var(n, vars[n]))
	}
	funcs, ok := strMap("functions", d["functions"])
	if !ok {
		return bad("functions")
	}
	for _, n := range sortedStrKeys(funcs) {
		decls = append(decls, Function{Name: n, Type: funcs[n]})
	}
	ops, ok := d["opaque"].(map[string]any)
	if !ok {
		return bad("opaque")
	}
	for _, n := range sortedAnyKeys(ops) {
		ms, ok := strMap("opaque", ops[n])
		if !ok {
			return bad("opaque")
		}
		o := OpaqueType{Name: n, Methods: map[string]Function{}}
		for m, sig := range ms {
			o.Methods[m] = Function{Type: sig}
		}
		decls = append(decls, o)
	}
	if hasFacts { // F14 9.2
		fs, ok := d["facts"].([]any)
		if !ok {
			return bad("facts")
		}
		for _, f := range fs {
			o, ok := f.(map[string]any)
			if !ok || len(o) != 1 {
				return bad("facts")
			}
			arr, ok := o["someNotNull"].([]any)
			if !ok {
				return bad("facts")
			}
			names := make([]string, len(arr))
			for i, x := range arr {
				if names[i], ok = x.(string); !ok {
					return bad("facts")
				}
			}
			decls = append(decls, SomeNotNull(names...))
		}
	}
	strList := func(v any) ([]string, bool) {
		arr, ok := v.([]any)
		if !ok {
			return nil, false
		}
		out := make([]string, len(arr))
		for i, x := range arr {
			s, ok := x.(string)
			if !ok {
				return nil, false
			}
			out[i] = s
		}
		return out, true
	}
	removed, ok := strList(d["removed"])
	if !ok {
		return bad("removed")
	}
	decls = append(decls, Without(removed...))
	reject, ok := strList(d["reject"])
	if !ok {
		return bad("reject")
	}
	decls = append(decls, RejectClasses(reject...))
	lm, ok := d["limits"].(map[string]any)
	if !ok || len(lm) != 3 {
		return bad("limits")
	}
	var lim Limits
	for _, k := range []string{"steps", "allocation", "depth"} {
		v, ok := lm[k]
		if !ok {
			return bad("limits")
		}
		var n int64
		switch x := v.(type) {
		case nil:
			if k == "depth" {
				return bad("limits")
			}
			n = NoLimit
		case float64:
			if x != math.Trunc(x) || x < 0 || x > 1<<53 {
				decls = append(decls, declFunc(func(e *Env) {
					e.errf(diag.CodeBadLimit, fmt.Sprintf("the limit %s is not a whole number of 0 or more", k))
				}))
				continue
			}
			n = int64(x)
			if n == 0 && k != "depth" {
				n = ZeroLimit
			}
		default:
			return bad("limits")
		}
		switch k {
		case "steps":
			lim.Steps = n
		case "allocation":
			lim.Alloc = n
		case "depth":
			if n == 0 {
				n = -1 // out of range: bad-limit
			}
			lim.Depth = int(n)
		}
	}
	decls = append(decls, lim)
	if hasGroups { // F6 2.7 item 2
		gl, ok := strList(d["groups"])
		if !ok {
			return bad("groups")
		}
		decls = append(decls, Groups(gl...))
	}
	if hasTypes { // F4 8 item 3: each by type text; no text names another
		tm, ok := strMap("types", d["types"])
		if !ok {
			return bad("types")
		}
		for _, n := range sortedStrKeys(tm) {
			decls = append(decls, NamedType(n, tm[n]))
		}
	}
	return declFunc(func(e *Env) {
		for _, d := range decls {
			d.apply(e)
		}
	}), nil
}

func sortedStrKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedAnyKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
