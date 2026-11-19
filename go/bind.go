package risor

//go:generate go run ./internal/gen/fastpath

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/render"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/typetext"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// This file derives host functions and the methods of opaque types from Go
// functions (design 19-13, ruling S4). Func and Methods give the same
// Function and OpaqueType values a host writes by hand: a signature as type
// text and a Native. They add no rule to the language (03-host.md 3.3.1
// item 7).
//
// A signature is derived from the Go function's type:
//
//	Go                                   Signature
//	a first *Call or context.Context     not a parameter: it gets the call
//	                                     handle, or the run's context
//	each other parameter                 a parameter, its type by TypeOf
//	                                     (F1 6.2), named a1, a2, ... or by
//	                                     Params
//	no result, (R)                       => null, => R
//	(error), (R, error)                  => null !, => R !
//
// Refused, as declaration errors reported by NewEnv or With:
//
//	a value that is not a Go function, a variadic function, other
//	results, a *Call that is not first, a Go type no row of F1 6.2
//	covers, a Go interface type other than any (error is the
//	failure mark only as the last result)            bad-type
//	a parameter of a Go function type (3.5.1 item 3),
//	Params with the wrong count or a bad name, two Go methods with
//	one Risor name                                   bad-signature
//	an option naming no method, a method name that
//	is not a word                                    bad-name
//
// A Go interface type is refused so that a later declaration of opaque
// interfaces (design 19-12) can give it a meaning. A host that wants such a
// function writes its signature: when a Function from Func or Methods has a
// Type that is not the derived one, that text is the signature, and the
// Go function supplies only the implementation. The text must have the Go
// function's count of parameters, none optional and none rest.
//
// At a call, each argument is converted to its Go parameter type: a value
// of that Go type is passed as it is, an any parameter takes the value as
// the run has it, and every other value is converted by the rules of
// Decode (F10 6.4), except that an opaque value is passed to a parameter of
// its own Go type. An argument that does not convert fails the call with
// function-error, whose message names the argument. An error the function
// returns fails it with function-error, and a panic with function-panic.
// A result is returned to the run as a host value (03-host.md 3.4): an
// integer past 2^53 - 1 fails the run with number-range where it is read.
//
// An adapted function charges nothing beyond the step of its call. One
// that takes *Call charges through it, as a hand-written Native does.

// FuncOption is an option of Func.
type FuncOption interface{ funcOption() }

// MethodOption is an option of Methods.
type MethodOption interface{ methodOption() }

type paramsOption []string

func (paramsOption) funcOption() {}

// Params names the parameters of a derived signature, in order, for the
// Go parameters after any first *Call or context.Context. Without it they
// are a1, a2, and so on: Go keeps no parameter names at run time.
func Params(names ...string) FuncOption { return paramsOption(append([]string(nil), names...)) }

type forMethodOption struct {
	goName string
	opts   []FuncOption
}

func (forMethodOption) methodOption() {}

// ForMethod gives the options of Func to one method of Methods, named by
// its Go name: risor.ForMethod("SetText", risor.Params("text")).
func ForMethod(goName string, opts ...FuncOption) MethodOption {
	return forMethodOption{goName: goName, opts: append([]FuncOption(nil), opts...)}
}

type renameOption struct{ goName, name string }

func (renameOption) methodOption() {}

// Rename gives a method of Methods, named by its Go name, a Risor name of
// its own in place of the derived one.
func Rename(goName, name string) MethodOption { return renameOption{goName: goName, name: name} }

type skipOption []string

func (skipOption) methodOption() {}

// Skip leaves methods out of Methods, named by their Go names.
func Skip(goNames ...string) MethodOption { return skipOption(append([]string(nil), goNames...)) }

// bindErr is a declaration error that Func or Methods found. The
// environment reports it when the declaration is applied.
type bindErr struct{ code, msg string }

// derivation is what Func and Methods record on a Function.
type derivation struct {
	// text is the derived signature, or "" when it was not derived.
	text string
	// errs are always reported; ifaceErrs only when the host gave no
	// signature of its own.
	errs, ifaceErrs []bindErr
	// params is the count of parameters the Go function takes from a
	// call: the arguments, without a receiver or a first *Call or
	// context.Context.
	params     int
	reflective bool
}

func (b *derivation) fail(code, format string, args ...any) {
	b.errs = append(b.errs, bindErr{code, fmt.Sprintf(format, args...)})
}

// Reflective reports whether Func or Methods adapted the function by
// reflection, which is slower than the adapters with no reflection that
// they choose for common shapes. It is false for a hand-written Impl.
func (f Function) Reflective() bool { return f.bind != nil && f.bind.reflective }

type funcConfig struct {
	names    []string
	hasNames bool
}

func (c *funcConfig) apply(opts []FuncOption) {
	for _, o := range opts {
		if p, ok := o.(paramsOption); ok {
			c.names, c.hasNames = p, true
		}
	}
}

// Func declares a host function whose signature is derived from fn's Go
// type, by the rules at the top of this file. Type holds the derived
// signature as type text, and Impl an adapter. A derivation error is
// reported by NewEnv or With, as a declaration error.
//
// The adapter uses no reflection when fn has one of these shapes, where
// each parameter type is string, int, int64, float64, bool or any, and T
// is one of the same six:
//
//   - no parameter; one; two of one type; a string and one other; or three
//     of one type; with no result, error, T or (T, error);
//   - a first *Call or context.Context and at most one parameter, with
//     error or (T, error).
//
// Every other shape is adapted by reflection, about ten times slower a
// call (Reflective).
func Func(name string, fn any, opts ...FuncOption) Function {
	return bindFunc(name, fn, opts, true)
}

// bindFunc is Func; with fast false it always adapts by reflection, for
// the tests that compare the two.
func bindFunc(name string, fn any, opts []FuncOption, fast bool) Function {
	var cfg funcConfig
	cfg.apply(opts)
	f := Function{Name: name, bind: &derivation{}}
	rv := reflect.ValueOf(fn)
	if !rv.IsValid() || rv.Kind() != reflect.Func {
		f.bind.fail(diag.CodeBadType, "%s is not a Go function", goTypeName(fn))
		return f
	}
	a := &adapter{what: name}
	derive(f.bind, a, rv.Type(), cfg)
	if len(f.bind.errs) > 0 {
		return f
	}
	f.Type = f.bind.text
	if rv.IsNil() {
		return f // no implementation: compile reports no-implementation
	}
	if fast {
		if impl := fastFunc(fn, a); impl != nil {
			f.Impl = impl
			return f
		}
	}
	f.bind.reflective = true
	f.Impl = a.reflective(rv)
	return f
}

func goTypeName(v any) string {
	if v == nil {
		return "nil"
	}
	return reflect.TypeOf(v).String()
}

// opaqueBinding is what Methods records on an OpaqueType.
type opaqueBinding struct{ errs []bindErr }

func (b *opaqueBinding) fail(code, format string, args ...any) {
	b.errs = append(b.errs, bindErr{code, fmt.Sprintf(format, args...)})
}

// Methods declares the opaque type name, whose methods are the exported
// methods of the Go type T, each adapted as Func adapts a function. T is
// the Go type of the opaque values: it must implement Opaque, and its
// OpaqueType must give name.
//
//   - The methods are T's method set as Go defines it, promoted methods
//     included: for a pointer type *S, the methods of S and of *S; for a
//     value type S, only those declared on S. OpaqueType is not one.
//   - A method's Risor name is its Go name with its first word in lower
//     case, unless Rename gives one. The first word is the longest run of
//     upper-case letters at the start, except that when a lower-case
//     letter follows a run of two or more, the run's last letter begins
//     the next word: URL gives url, ID id, HTTPStatus httpStatus, UserID
//     userID, XAxis xAxis, ID2 id2 and SetText setText. A plural
//     initialism splits before its s: URLs gives urLs.
//   - An interface type T is refused: it is left to opaque interfaces
//     (design 19-12).
//
// A method's adapter uses no reflection when, after its receiver, it
// takes no parameter and returns nothing, error, T or (T, error), or
// takes one parameter and returns nothing or error, with the types of
// Func's list. For a value type S, each call copies the receiver. Errors
// are reported by NewEnv or With, as declaration errors.
func Methods[T any](name string, opts ...MethodOption) OpaqueType {
	return bindMethods[T](name, opts, true)
}

// bindMethods is Methods; with fast false it always adapts by reflection.
func bindMethods[T any](name string, opts []MethodOption, useFast bool) OpaqueType {
	o := OpaqueType{Name: name, Methods: map[string]Function{}, bind: &opaqueBinding{}}
	ob := o.bind
	rt := reflect.TypeFor[T]()
	switch {
	case rt.Kind() == reflect.Interface:
		ob.fail(diag.CodeBadType, "opaque type %s: %v is an interface type, not the Go type of opaque values", name, rt)
		return o
	case !rt.Implements(opaqueIface):
		hint := ""
		if rt.Kind() != reflect.Pointer && reflect.PointerTo(rt).Implements(opaqueIface) {
			hint = fmt.Sprintf("; *%v does", rt)
		}
		ob.fail(diag.CodeBadType, "opaque type %s: %v does not implement risor.Opaque%s", name, rt, hint)
		return o
	}
	if got := reflect.Zero(rt).Interface().(Opaque).OpaqueType(); got != name {
		ob.fail(diag.CodeBadType, "opaque type %s: the values of %v name the opaque type %s", name, rt, got)
		return o
	}

	renames := map[string]string{}
	skips := map[string]bool{}
	cfgs := map[string]*funcConfig{}
	var named []string
	for _, opt := range opts {
		switch x := opt.(type) {
		case renameOption:
			renames[x.goName] = x.name
			named = append(named, x.goName)
		case skipOption:
			for _, n := range x {
				skips[n] = true
				named = append(named, n)
			}
		case forMethodOption:
			c := cfgs[x.goName]
			if c == nil {
				c = &funcConfig{}
				cfgs[x.goName] = c
			}
			c.apply(x.opts)
			named = append(named, x.goName)
		}
	}
	for _, n := range named {
		if m, ok := rt.MethodByName(n); !ok || m.Name == "OpaqueType" {
			ob.fail(diag.CodeBadName, "opaque type %s: %v has no method %s", name, rt, n)
		}
	}

	// The method expressions are taken on a pointer type, so that every
	// T uses the one instantiation of fastMethod that pointer types share.
	mt := rt
	var fast func(fn any, a *adapter) Native
	if rt.Kind() == reflect.Pointer {
		fast = func(fn any, a *adapter) Native { return fastMethod[T](fn, a, recvAs[T]) }
	} else {
		mt = reflect.PointerTo(rt)
		fast = func(fn any, a *adapter) Native { return fastMethod[*T](fn, a, recvAddr[T]) }
	}
	if !useFast {
		fast = func(any, *adapter) Native { return nil }
	}
	goNames := map[string]string{} // Risor name -> Go name
	for i := 0; i < rt.NumMethod(); i++ {
		m := rt.Method(i)
		if m.Name == "OpaqueType" || skips[m.Name] {
			continue
		}
		rname, ok := renames[m.Name]
		if !ok {
			rname = methodName(m.Name)
		}
		if !value.IsWord(rname) {
			ob.fail(diag.CodeBadName, "opaque type %s: %q, the name of method %s, is not a word", name, rname, m.Name)
			continue
		}
		if prev, dup := goNames[rname]; dup {
			ob.fail(diag.CodeBadSignature, "opaque type %s: the methods %s and %s of %v are both named %s", name, prev, m.Name, rt, rname)
			delete(o.Methods, rname)
			continue
		}
		goNames[rname] = m.Name
		cfg := cfgs[m.Name]
		if cfg == nil {
			cfg = &funcConfig{}
		}
		me, _ := mt.MethodByName(m.Name)
		o.Methods[rname] = bindMethod(me.Func, rname, *cfg, fast)
	}
	return o
}

// bindMethod adapts the method expression fv, whose first parameter is
// the receiver.
func bindMethod(fv reflect.Value, name string, cfg funcConfig, fast func(any, *adapter) Native) Function {
	f := Function{Name: name, bind: &derivation{}}
	a := &adapter{what: "method " + name, first: 1}
	derive(f.bind, a, fv.Type(), cfg)
	if len(f.bind.errs) > 0 {
		return f
	}
	f.Type = f.bind.text
	fn := fv.Interface()
	if impl := fast(fn, a); impl != nil {
		f.Impl = impl
		return f
	}
	f.bind.reflective = true
	f.Impl = a.reflective(fv)
	return f
}

func recvAs[T any](a *adapter, args []any) (T, error) {
	if r, ok := args[0].(T); ok {
		return r, nil
	}
	var r T
	return r, a.slow(args, 0, &r)
}

func recvAddr[T any](a *adapter, args []any) (*T, error) {
	switch r := args[0].(type) {
	case T:
		return &r, nil
	case *T:
		if r != nil {
			return r, nil
		}
	}
	var r *T
	return r, a.slow(args, 0, &r)
}

// methodName gives the Risor name of a Go method by the rule of Methods.
func methodName(goName string) string {
	n := 0 // bytes in the run of upper-case letters
	runes := 0
	for n < len(goName) {
		r, size := utf8.DecodeRuneInString(goName[n:])
		if !unicode.IsUpper(r) {
			break
		}
		n += size
		runes++
	}
	if n < len(goName) && runes > 1 {
		if r, _ := utf8.DecodeRuneInString(goName[n:]); unicode.IsLower(r) {
			_, size := utf8.DecodeLastRuneInString(goName[:n])
			n -= size
		}
	}
	return strings.ToLower(goName[:n]) + goName[n:]
}

var (
	callType  = reflect.TypeFor[*Call]()
	ctxType   = reflect.TypeFor[context.Context]()
	errorType = reflect.TypeFor[error]()
)

// The first parameter that is not an argument, and the results.
const (
	leadNone = iota
	leadCall
	leadCtx
)

const (
	resNone = iota
	resErr
	resVal
	resValErr
)

// derive derives the signature of the Go function type ft into b, and
// fills the adapter a. When a.first is 1, ft's first parameter is the
// receiver.
func derive(b *derivation, a *adapter, ft reflect.Type, cfg funcConfig) {
	if ft.IsVariadic() {
		b.fail(diag.CodeBadType, "%v is variadic, and a variadic Go function has no derived signature", ft)
		return
	}
	n := ft.NumIn()
	i := a.first
	if a.first == 1 {
		a.in = append(a.in, ft.In(0))
		a.quick = append(a.quick, reflect.Invalid)
	}
	if i < n {
		switch ft.In(i) {
		case callType:
			a.lead = leadCall
			i++
		case ctxType:
			a.lead = leadCtx
			i++
		}
	}
	d := &deriver{inprog: map[reflect.Type]bool{}, done: map[reflect.Type]*types.Type{}, strict: true}
	var params []string
	for ; i < n; i++ {
		t := ft.In(i)
		a.in = append(a.in, t)
		a.quick = append(a.quick, quickKind(t))
		pos := len(a.in) - a.first // the parameter's place, from 1
		switch {
		case t == callType:
			b.fail(diag.CodeBadType, "a *risor.Call is only the first parameter")
			continue
		case t.Kind() == reflect.Func:
			b.fail(diag.CodeBadSignature, "parameter %d is a Go function, and a host function takes no function", pos)
			continue
		}
		typ, err := deriveIn(d, t)
		if err != nil {
			b.fail(diag.CodeBadType, "parameter %d: %v", pos, err)
			continue
		}
		params = append(params, typ)
	}
	b.params = n - a.first - boolInt(a.lead != leadNone)

	result := "null"
	switch ft.NumOut() {
	case 0:
		a.res = resNone
	case 1:
		if ft.Out(0) == errorType {
			a.res = resErr
			break
		}
		a.res = resVal
		typ, err := deriveIn(d, ft.Out(0))
		if err != nil {
			b.fail(diag.CodeBadType, "the result: %v", err)
		}
		result = typ
	case 2:
		if ft.Out(1) != errorType {
			b.fail(diag.CodeBadType, "%v has two results, and the second is not error", ft)
			return
		}
		a.res = resValErr
		typ, err := deriveIn(d, ft.Out(0))
		if err != nil {
			b.fail(diag.CodeBadType, "the result: %v", err)
		}
		result = typ
	default:
		b.fail(diag.CodeBadType, "%v has more than two results", ft)
		return
	}
	if len(d.ifaces) > 0 {
		names := make([]string, len(d.ifaces))
		for i, t := range d.ifaces {
			names[i] = t.String()
		}
		b.ifaceErrs = append(b.ifaceErrs, bindErr{diag.CodeBadType, fmt.Sprintf(
			"the Go interface type %s has no derived type: write the signature in Type", strings.Join(names, ", "))})
	}

	names := cfg.names
	if !cfg.hasNames {
		names = make([]string, b.params)
		for i := range names {
			names[i] = fmt.Sprintf("a%d", i+1)
		}
	} else if len(names) != b.params {
		b.fail(diag.CodeBadSignature, "Params gives %d names, and the Go function takes %d parameters", len(names), b.params)
	} else {
		seen := map[string]bool{}
		for _, nm := range names {
			if !syntax.IsName(nm) {
				b.fail(diag.CodeBadSignature, "%q is not a name for a parameter", nm)
			} else if seen[nm] {
				b.fail(diag.CodeBadSignature, "parameter %s is named twice", nm)
			}
			seen[nm] = true
		}
	}
	if len(b.errs) > 0 || len(b.ifaceErrs) > 0 {
		return
	}
	var s strings.Builder
	s.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			s.WriteString(", ")
		}
		s.WriteString(names[i])
		s.WriteString(": ")
		s.WriteString(p)
	}
	s.WriteString(") => ")
	s.WriteString(result)
	if a.res == resErr || a.res == resValErr {
		s.WriteString(" !")
	}
	b.text = s.String()
}

// deriveIn derives the type of a parameter or a result as type text.
func deriveIn(d *deriver, t reflect.Type) (string, error) {
	ty, err := d.nested(t)
	if err == nil && types.Depth(ty) > types.MaxDepth {
		err = fmt.Errorf("the type is deeper than %d", types.MaxDepth)
	}
	if err != nil {
		return "", fmt.Errorf("%v: %w", t, err)
	}
	return ty.String(), nil
}

// bound applies the derivation of a Function from Func or Methods when it is
// declared: it reports what the derivation found, and tests a signature
// the host wrote against the Go function. It reports whether the
// declaration goes on.
func (e *Env) bound(what string, f Function) bool {
	b := f.bind
	if b == nil {
		return true
	}
	for _, x := range b.errs {
		e.errf(x.code, what+": "+x.msg)
	}
	if len(b.errs) > 0 {
		return false
	}
	if f.Type == "" && len(b.ifaceErrs) > 0 {
		for _, x := range b.ifaceErrs {
			e.errf(x.code, what+": "+x.msg)
		}
		return false
	}
	if f.Type == b.text {
		return true
	}
	sig, err := typetext.ParseSig(f.Type)
	if err != nil {
		return true // parseSig reports it
	}
	for _, p := range sig.Params {
		if p.Optional || p.Rest {
			e.errf(diag.CodeBadSignature, fmt.Sprintf("%s: parameter %s is optional or rest, and the Go function takes every parameter", what, p.Name))
			return false
		}
	}
	if len(sig.Params) != b.params {
		e.errf(diag.CodeBadSignature, fmt.Sprintf("%s: the signature has %d parameters, and the Go function takes %d", what, len(sig.Params), b.params))
		return false
	}
	return true
}

// adapter is what an adapted function needs at a call.
type adapter struct {
	// what names the function in a message: "upper", "method setText".
	what string
	// first is 1 for a method: args[0] is the receiver.
	first int
	// in is the Go type of each value of args.
	in []reflect.Type
	// quick is, for each value of args, the kind its Go type is converted
	// as by quickly, or reflect.Invalid for the decoder.
	quick []reflect.Kind
	lead  int
	res   int
}

// quickKind gives the kind by which quickly converts to t: a bool, string
// or number kind, or an empty interface, when t has no conversion of its
// own (Opaque, UnmarshalText or UnmarshalJSON); else reflect.Invalid.
func quickKind(t reflect.Type) reflect.Kind {
	pt := reflect.PointerTo(t)
	if t.Implements(opaqueIface) || pt.Implements(textUnmarshaler) || pt.Implements(jsonUnmarshaler) {
		return reflect.Invalid
	}
	switch k := t.Kind(); k {
	case reflect.Bool, reflect.String, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return k
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return k
		}
	}
	return reflect.Invalid
}

// quickly converts a value of the language to dst by its kind k, taking
// exactly what the decoder takes. It reports false when it did not, and
// the decoder then converts or gives the failure.
func quickly(k reflect.Kind, v any, dst reflect.Value) bool {
	switch k {
	case reflect.Interface:
		if v == nil {
			dst.Set(reflect.Zero(dst.Type()))
		} else {
			dst.Set(reflect.ValueOf(v))
		}
		return true
	case reflect.Bool:
		b, ok := v.(bool)
		if ok {
			dst.SetBool(b)
		}
		return ok
	case reflect.String:
		s, ok := v.(string)
		if ok {
			dst.SetString(s)
		}
		return ok
	case reflect.Float64:
		f, ok := v.(float64)
		if ok {
			dst.SetFloat(f)
		}
		return ok
	case reflect.Float32:
		f, ok := v.(float64)
		if ok && math.Abs(f) <= math.MaxFloat32 {
			dst.SetFloat(float64(float32(f)))
			return true
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f, ok := v.(float64)
		if ok && value.IsWhole(f) && !dst.OverflowInt(int64(f)) {
			dst.SetInt(int64(f))
			return true
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		f, ok := v.(float64)
		if ok && value.IsWhole(f) && f >= 0 && !dst.OverflowUint(uint64(f)) {
			dst.SetUint(uint64(f))
			return true
		}
	}
	return false
}

// slow converts args[i] into *dst, for an argument the adapter's fast
// test did not take. It gives the error of an argument that does not
// convert.
func (a *adapter) slow(args []any, i int, dst any) error {
	return a.conv(args, i, reflect.ValueOf(dst).Elem())
}

// conv converts args[i] into dst, a settable value of type a.in[i].
func (a *adapter) conv(args []any, i int, dst reflect.Value) error {
	v := args[i]
	if v != nil {
		if rv := reflect.ValueOf(v); rv.Type() == dst.Type() {
			dst.Set(rv)
			return nil
		}
	}
	if k := a.quick[i]; k != reflect.Invalid && quickly(k, v, dst) {
		return nil
	}
	d := &decoder{args: true, max: 1}
	d.decode(v, dst, []any{}, 0)
	if len(d.fails) == 0 {
		return nil
	}
	f := d.fails[0]
	where := fmt.Sprintf("argument %d of %s", i-a.first+1, a.what)
	if i < a.first {
		where = "the receiver of " + a.what
	}
	if len(f.Path) > 0 {
		where += ", at " + strings.TrimPrefix(render.Path(f.Path), ".")
	}
	return &argError{msg: where + ": " + f.Message}
}

// argError is the error of an argument that does not convert to its Go
// parameter type. The run fails with function-error and this message.
type argError struct{ msg string }

func (e *argError) Error() string { return e.msg }

func (a *adapter) argStr(args []any, i int) (string, error) {
	if s, ok := args[i].(string); ok {
		return s, nil
	}
	var s string
	return s, a.slow(args, i, &s)
}

func (a *adapter) argInt(args []any, i int) (int, error) {
	if f, ok := args[i].(float64); ok && value.IsWhole(f) && f >= math.MinInt && f <= math.MaxInt {
		return int(f), nil
	}
	var n int
	return n, a.slow(args, i, &n)
}

func (a *adapter) argInt64(args []any, i int) (int64, error) {
	if f, ok := args[i].(float64); ok && value.IsWhole(f) {
		return int64(f), nil
	}
	var n int64
	return n, a.slow(args, i, &n)
}

func (a *adapter) argFloat64(args []any, i int) (float64, error) {
	if f, ok := args[i].(float64); ok {
		return f, nil
	}
	var f float64
	return f, a.slow(args, i, &f)
}

func (a *adapter) argBool(args []any, i int) (bool, error) {
	if b, ok := args[i].(bool); ok {
		return b, nil
	}
	var b bool
	return b, a.slow(args, i, &b)
}

// intResult makes a Go integer result a value. One past 2^53 - 1 is
// returned as it is, and the run's read of it fails with number-range
// (03-host.md 3.4.4).
func intResult(n int64) any {
	if n <= value.MaxWhole && n >= -value.MaxWhole {
		return value.Num(float64(n))
	}
	return n
}

func floatResult(f float64) any { return value.Num(f) }

// reflective is the adapter of fv by reflection.
func (a *adapter) reflective(fv reflect.Value) Native {
	nIn := len(a.in) + boolInt(a.lead != leadNone)
	return func(c *Call, args []any) (any, error) {
		if len(args) != len(a.in) {
			return nil, fmt.Errorf("%s takes %d arguments, found %d", a.what, len(a.in)-a.first, len(args)-a.first)
		}
		in := make([]reflect.Value, nIn)
		j := 0
		for i := range args {
			if i == a.first && a.lead != leadNone {
				in[j] = leadValue(a.lead, c)
				j++
			}
			if x := args[i]; x != nil && reflect.TypeOf(x) == a.in[i] {
				in[j] = reflect.ValueOf(x) // no copy of a value of the type
				j++
				continue
			}
			v := reflect.New(a.in[i]).Elem()
			if err := a.conv(args, i, v); err != nil {
				return nil, err
			}
			in[j] = v
			j++
		}
		if j < nIn {
			in[j] = leadValue(a.lead, c)
		}
		out := fv.Call(in)
		switch a.res {
		case resErr:
			return nil, errorOf(out[0])
		case resVal:
			return resultOf(out[0]), nil
		case resValErr:
			if err := errorOf(out[1]); err != nil {
				return nil, err
			}
			return resultOf(out[0]), nil
		}
		return nil, nil
	}
}

func leadValue(lead int, c *Call) reflect.Value {
	if lead == leadCall {
		return reflect.ValueOf(c)
	}
	if ctx := c.Context(); ctx != nil {
		return reflect.ValueOf(ctx)
	}
	return reflect.Zero(ctxType)
}

func errorOf(v reflect.Value) error {
	if v.IsNil() {
		return nil
	}
	return v.Interface().(error)
}

var (
	intType     = reflect.TypeFor[int]()
	int64Type   = reflect.TypeFor[int64]()
	float64Type = reflect.TypeFor[float64]()
)

// resultOf makes a result a value as the fast adapters do.
func resultOf(v reflect.Value) any {
	switch v.Type() {
	case intType, int64Type:
		return intResult(v.Int())
	case float64Type:
		return floatResult(v.Float())
	}
	return v.Interface()
}
