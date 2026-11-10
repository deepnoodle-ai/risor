package risor

import (
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is package F10: the load of a config file and the decode of
// its value into Go types (spec/packages/f10-config.md 5 and 6).

// The steps of a load (F10 5.2).
const (
	StepDeclare = 0
	StepParse   = 1
	StepCheck   = 2
	StepRun     = 3
	StepConform = 4
	StepDecode  = 5
)

// LoadError is the failure of a load: the step that failed, its
// diagnostics, and the usage of the run when step 3 was reached. It
// unwraps to an *Error with the same diagnostics.
type LoadError struct {
	Step        int
	Diagnostics []Diagnostic
	// Usage is the usage of the run; HasUsage is false when the load
	// stopped before step 3.
	Usage    Usage
	HasUsage bool
}

// Error returns the step and the first diagnostic as text.
func (e *LoadError) Error() string {
	return fmt.Sprintf("load: step %d: %v", e.Step, &Error{Diagnostics: e.Diagnostics})
}

// Unwrap gives the diagnostics as an *Error.
func (e *LoadError) Unwrap() error { return &Error{Diagnostics: e.Diagnostics} }

// LoadOption is an option of a load.
type LoadOption interface{ loadOption() }

type loadType struct {
	text   string
	schema any
	isText bool
	t      *Type
}

func (loadType) loadOption() {}

type loadMax int

func (loadMax) loadOption() {}

// LoadType gives the expected type of LoadValue as type text, resolved in
// the environment.
func LoadType(text string) LoadOption { return loadType{text: text, isText: true} }

// LoadSchema gives the expected type of LoadValue as a JSON Schema, mapped
// with the option Closed and no other (F10 5.6).
func LoadSchema(schema any) LoadOption { return loadType{schema: schema} }

// LoadTypeOf gives the expected type of LoadValue as a type: from TypeOf,
// TypeFromSchema or Env.ParseType.
func LoadTypeOf(t Type) LoadOption { return loadType{t: &t} }

// MaxFailures is the most failures step 4 reports (F10 5.2 item 1). The
// default is 1; a maximum below 1 is read as 1.
func MaxFailures(n int) LoadOption { return loadMax(n) }

// Load loads a config file into dst, a value of a Go type: the steps 0 to 5
// of F10 5.2 with the expected type TypeOf[T]. A field the value lacks
// keeps what dst held, so a host fills its defaults into dst first. A
// failure is a *LoadError.
func Load[T any](ctx context.Context, env *Env, source string, vars Vars, dst *T, opts ...LoadOption) error {
	t, err := TypeOf[T]()
	if err != nil {
		return &LoadError{Step: StepDeclare, Diagnostics: err.(*Error).Diagnostics}
	}
	v, u, err := load(ctx, env, source, vars, &t, opts)
	if err != nil {
		return err
	}
	if err := Decode(v, dst); err != nil {
		var e *Error
		errors.As(err, &e)
		return &LoadError{Step: StepDecode, Diagnostics: e.Diagnostics, Usage: u, HasUsage: true}
	}
	return nil
}

// LoadValue loads a config file and gives its value and usage: steps 0 to
// 4 of F10 5.2, with the expected type of LoadType, LoadSchema or
// LoadTypeOf. With none, the check has no expected result and step 4 is
// not made. A failure is a *LoadError.
func LoadValue(ctx context.Context, env *Env, source string, vars Vars, opts ...LoadOption) (any, Usage, error) {
	if env == nil {
		var err error
		if env, err = NewEnv(); err != nil {
			return nil, Usage{}, &LoadError{Step: StepDeclare, Diagnostics: err.(*Error).Diagnostics}
		}
	}
	var t *Type
	for _, o := range opts {
		lt, ok := o.(loadType)
		if !ok {
			continue
		}
		switch {
		case lt.t != nil:
			t = lt.t
		case lt.isText:
			tt, err := env.ParseType(lt.text)
			if err != nil {
				return nil, Usage{}, &LoadError{Step: StepDeclare, Diagnostics: err.(*Error).Diagnostics}
			}
			t = &tt
		default:
			tt, err := TypeFromSchema(lt.schema, Closed)
			if err != nil {
				return nil, Usage{}, &LoadError{Step: StepDeclare, Diagnostics: err.(*Error).Diagnostics}
			}
			t = &tt
		}
	}
	return load(ctx, env, source, vars, t, opts)
}

func load(ctx context.Context, env *Env, source string, vars Vars, t *Type, opts []LoadOption) (any, Usage, error) {
	max := 1
	for _, o := range opts {
		if m, ok := o.(loadMax); ok {
			max = int(m)
		}
	}
	if env == nil {
		var err error
		if env, err = NewEnv(); err != nil {
			return nil, Usage{}, &LoadError{Step: StepDeclare, Diagnostics: err.(*Error).Diagnostics}
		}
	}
	if t != nil && t.problem() != "" {
		return nil, Usage{}, &LoadError{Step: StepDeclare, Diagnostics: []Diagnostic{{Phase: PhaseDeclare,
			Code: diag.CodeBadType, Message: "the expected result: " + t.problem()}}}
	}
	x, err := Parse(source)
	if err != nil {
		return nil, Usage{}, &LoadError{Step: StepParse, Diagnostics: err.(*Error).Diagnostics}
	}
	var copts []CheckOption
	if t != nil {
		copts = append(copts, ExpectType(*t))
	}
	prog, err := env.Compile(x, copts...)
	if err != nil {
		e := err.(*Error)
		step := StepCheck
		if e.Phase() == PhaseDeclare {
			step = StepDeclare
		}
		return nil, Usage{}, &LoadError{Step: step, Diagnostics: e.Diagnostics}
	}
	v, u, err := prog.Measure(ctx, vars)
	if err != nil {
		return nil, u, &LoadError{Step: StepRun, Diagnostics: err.(*Error).Diagnostics, Usage: u, HasUsage: true}
	}
	if t != nil {
		if ds := ConformAll(v, *t, max); len(ds) > 0 {
			return nil, u, &LoadError{Step: StepConform, Diagnostics: ds, Usage: u, HasUsage: true}
		}
	}
	return v, u, nil
}

// Decode decodes a value a run returned into dst, a non-nil pointer, as
// for json.Unmarshal (F10 6.1, 6.4). The value must conform to TypeOf of
// the destination's type: a key with no field, a missing key of a required
// field, null where the Go type cannot hold it, and a value of another
// kind are each a failure at its path, as is a number the Go type cannot
// hold, text its UnmarshalText refuses, and an opaque value. A field whose
// key the value lacks keeps what dst held. What Decode gives shares no
// memory with v. A failure is an *Error of phase run; a dst that is not a
// non-nil pointer, or whose type has no derived type, is of phase declare.
func Decode(v any, dst any) error {
	rv := reflect.ValueOf(dst)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadType,
			Message: fmt.Sprintf("the destination of a decode is a non-nil pointer, not %T", dst)}}}
	}
	if _, err := typeOfReflect(rv.Elem().Type()); err != nil {
		return err
	}
	d := &decoder{deriver: &deriver{inprog: map[reflect.Type]bool{}, done: map[reflect.Type]*types.Type{}}}
	d.decode(v, rv.Elem(), []any{}, 0)
	if len(d.fails) > 0 {
		return &Error{Diagnostics: d.fails}
	}
	return nil
}

// The most failures a decode reports, and its depth (F10 6.1 items 8, 10).
const (
	decodeMaxFailures = 100
	decodeMaxDepth    = 200
)

type decoder struct {
	deriver *deriver
	fails   []Diagnostic
}

var (
	textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
	jsonUnmarshaler = reflect.TypeFor[json.Unmarshaler]()
)

func (d *decoder) full() bool { return len(d.fails) >= decodeMaxFailures }

func (d *decoder) fail(path []any, code, msg string) {
	if d.full() {
		return
	}
	diag := Diagnostic{Phase: PhaseRun, Code: code, Class: "value", Message: msg, Trace: []TraceEntry{}}
	if path != nil {
		diag.Path = append([]any{}, path...)
	}
	if code == "depth-exceeded" {
		diag.Class = "limit"
	}
	d.fails = append(d.fails, diag)
}

// wrong is the template wrong of F4 4.4 against the type derived from rt.
func (d *decoder) wrong(path []any, rt reflect.Type, v any) {
	t, err := d.deriver.nested(rt)
	expected := rt.String()
	if err == nil {
		expected = t.String()
	}
	d.fail(path, "not-conforming", messages.Build("not-conforming", "wrong", messages.Word("expected", expected), messages.Val("found", v)))
}

func extend(path []any, k any) []any {
	out := make([]any, len(path)+1)
	copy(out, path)
	out[len(path)] = k
	return out
}

// decode decodes v into dst, which is settable, at path.
func (d *decoder) decode(v any, dst reflect.Value, path []any, depth int) {
	if d.full() {
		return
	}
	if depth > decodeMaxDepth {
		d.fail(nil, "depth-exceeded", messages.Build("depth-exceeded", "", messages.Word("limit", strconv.Itoa(decodeMaxDepth))))
		return
	}
	rt := dst.Type()
	v, rerr := value.Read(v)
	if rerr != value.ReadOK {
		d.fail(path, "number-range", "the host number cannot enter the language")
		return
	}
	k := value.KindOf(v)
	if k == value.OpaqueKind {
		d.fail(path, "not-conforming", fmt.Sprintf("an opaque value of type %s is not data, and is not decoded", value.OpaqueTypeName(v)))
		return
	}
	// An opaque Go type refuses every value (6.4 item 8).
	if rt.Implements(opaqueIface) && !(rt.Kind() == reflect.Pointer && rt.Elem().Implements(opaqueIface)) {
		d.fail(path, "not-conforming", fmt.Sprintf("%v is an opaque type, and no value is decoded into it", rt))
		return
	}
	switch rt.Kind() {
	case reflect.Interface:
		if k == value.Null {
			dst.Set(reflect.Zero(rt))
			return
		}
		x, ok := d.plain(v, path, depth)
		if !ok {
			return
		}
		xv := reflect.ValueOf(x)
		if !xv.Type().AssignableTo(rt) {
			d.wrong(path, rt, v)
			return
		}
		dst.Set(xv)
		return
	case reflect.Pointer:
		if k == value.Null {
			dst.Set(reflect.Zero(rt))
			return
		}
		if dst.IsNil() {
			n := reflect.New(rt.Elem())
			d.decode(v, n.Elem(), path, depth)
			dst.Set(n)
			return
		}
		d.decode(v, dst.Elem(), path, depth)
		return
	}
	pt := reflect.PointerTo(rt)
	if pt.Implements(textUnmarshaler) {
		s, ok := v.(string)
		if !ok {
			d.wrong(path, rt, v)
			return
		}
		if err := dst.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s)); err != nil {
			d.fail(path, "not-conforming", fmt.Sprintf("%v refuses the text: %v", rt, err))
		}
		return
	}
	if pt.Implements(jsonUnmarshaler) {
		text, err := JSONText(v)
		if err != nil {
			d.fail(path, "not-conforming", err.Error())
			return
		}
		if err := dst.Addr().Interface().(json.Unmarshaler).UnmarshalJSON([]byte(text)); err != nil {
			d.fail(path, "not-conforming", fmt.Sprintf("%v refuses the value: %v", rt, err))
		}
		return
	}
	switch rt.Kind() {
	case reflect.Bool:
		b, ok := v.(bool)
		if !ok {
			d.wrong(path, rt, v)
			return
		}
		dst.SetBool(b)
	case reflect.String:
		s, ok := v.(string)
		if !ok {
			d.wrong(path, rt, v)
			return
		}
		dst.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f, ok := v.(float64)
		if !ok || !value.IsWhole(f) {
			d.wrong(path, rt, v)
			return
		}
		lim := math.Ldexp(1, rt.Bits()-1)
		if f < -lim || f >= lim || dst.OverflowInt(int64(f)) {
			d.fail(path, "not-conforming", fmt.Sprintf("%s does not fit %v", value.FormatNumber(f), rt))
			return
		}
		dst.SetInt(int64(f))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		f, ok := v.(float64)
		if !ok || !value.IsWhole(f) {
			d.wrong(path, rt, v)
			return
		}
		if f < 0 || f >= math.Ldexp(1, rt.Bits()) || dst.OverflowUint(uint64(f)) {
			d.fail(path, "not-conforming", fmt.Sprintf("%s does not fit %v", value.FormatNumber(f), rt))
			return
		}
		dst.SetUint(uint64(f))
	case reflect.Float32:
		f, ok := v.(float64)
		if !ok {
			d.wrong(path, rt, v)
			return
		}
		if math.Abs(f) > math.MaxFloat32 {
			d.fail(path, "not-conforming", fmt.Sprintf("%s does not fit %v", value.FormatNumber(f), rt))
			return
		}
		dst.SetFloat(float64(float32(f)))
	case reflect.Float64:
		f, ok := v.(float64)
		if !ok {
			d.wrong(path, rt, v)
			return
		}
		dst.SetFloat(f)
	case reflect.Slice, reflect.Array:
		if k != value.List {
			d.wrong(path, rt, v)
			return
		}
		n := value.Len(v)
		var out reflect.Value
		if rt.Kind() == reflect.Slice {
			out = reflect.MakeSlice(rt, n, n)
		} else {
			if n != rt.Len() {
				d.fail(path, "not-conforming", fmt.Sprintf("%v holds %d elements, found a list of %d", rt, rt.Len(), n))
				return
			}
			out = reflect.New(rt).Elem()
		}
		for i := 0; i < n && !d.full(); i++ {
			d.decode(value.At(v, i), out.Index(i), extend(path, float64(i)), depth+1)
		}
		dst.Set(out)
	case reflect.Map:
		if k != value.Map {
			d.wrong(path, rt, v)
			return
		}
		out := reflect.MakeMapWithSize(rt, value.Len(v))
		for _, key := range value.Keys(v) {
			if d.full() {
				break
			}
			x, _ := value.Get(v, key)
			ev := reflect.New(rt.Elem()).Elem()
			d.decode(x, ev, extend(path, key), depth+1)
			kv := reflect.New(rt.Key()).Elem()
			kv.SetString(key)
			out.SetMapIndex(kv, ev)
		}
		dst.Set(out)
	case reflect.Struct:
		if k != value.Map {
			d.wrong(path, rt, v)
			return
		}
		d.decodeStruct(v, dst, path, depth)
	default:
		d.wrong(path, rt, v)
	}
}

// decodeStruct is the walk of a record (F8 6.2 step 6) over the fields a
// struct presents: the declared fields in key order, then every other key.
func (d *decoder) decodeStruct(v any, dst reflect.Value, path []any, depth int) {
	fields := value.StructFields(dst.Type())
	listed := map[string]bool{}
	for _, f := range fields {
		listed[f.Key] = true
		if d.full() {
			return
		}
		x, ok := value.Get(v, f.Key)
		if !ok {
			if !f.Optional() {
				d.fail(extend(path, f.Key), "not-conforming", messages.Build("not-conforming", "missing", messages.Val("key", f.Key)))
			}
			continue
		}
		fv, ok := fieldAlloc(dst, f.Index)
		if !ok {
			d.fail(extend(path, f.Key), "not-conforming", fmt.Sprintf("the field %s cannot be set", f.Key))
			continue
		}
		d.decode(x, fv, extend(path, f.Key), depth+1)
	}
	for _, key := range value.Keys(v) {
		if d.full() {
			return
		}
		if !listed[key] {
			d.fail(extend(path, key), "not-conforming", messages.Build("not-conforming", "extra", messages.Val("key", key)))
		}
	}
}

// fieldAlloc is the field at index, allocating embedded pointers on the
// way.
func fieldAlloc(v reflect.Value, index []int) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !v.CanSet() {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v, v.CanSet()
}

// plain copies a value into plain Go data: nil, bool, float64, string,
// []any or map[string]any (6.4 item 9).
func (d *decoder) plain(v any, path []any, depth int) (any, bool) {
	if depth > decodeMaxDepth {
		d.fail(nil, "depth-exceeded", messages.Build("depth-exceeded", "", messages.Word("limit", strconv.Itoa(decodeMaxDepth))))
		return nil, false
	}
	v, rerr := value.Read(v)
	if rerr != value.ReadOK {
		d.fail(path, "number-range", "the host number cannot enter the language")
		return nil, false
	}
	switch value.KindOf(v) {
	case value.Null:
		return nil, true
	case value.Bool, value.Number:
		return v, true
	case value.String:
		return strings.Clone(v.(string)), true
	case value.List:
		n := value.Len(v)
		out := make([]any, n)
		for i := 0; i < n; i++ {
			x, ok := d.plain(value.At(v, i), extend(path, float64(i)), depth+1)
			if !ok {
				return nil, false
			}
			out[i] = x
		}
		return out, true
	case value.Map:
		out := make(map[string]any, value.Len(v))
		for _, k := range value.Keys(v) {
			x, _ := value.Get(v, k)
			y, ok := d.plain(x, extend(path, k), depth+1)
			if !ok {
				return nil, false
			}
			out[k] = y
		}
		return out, true
	}
	d.fail(path, "not-conforming", fmt.Sprintf("an opaque value of type %s is not data, and is not decoded", value.OpaqueTypeName(v)))
	return nil, false
}
