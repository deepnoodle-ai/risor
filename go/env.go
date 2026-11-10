package risor

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
)

// NoLimit turns off a step or allocation limit.
const NoLimit = eval.NoLimit

// ZeroLimit in Limits.Steps or Limits.Alloc sets a limit of zero, since a
// zero field keeps the default.
const ZeroLimit = -2

// Call is the call handle a host function receives: Context, Charge and
// Allocate (spec/core/03-host.md 3.5.2). When Charge or Allocate returns an
// error, return it: the run then ends with the limit failure.
type Call = eval.Call

// ErrCallEnded is returned by Charge and Allocate of a call handle used after
// its function returned (03-host.md 3.5.2 item 6). Such a use charges nothing
// and changes no run; Context then gives a cancelled context.
var ErrCallEnded = eval.ErrCallEnded

// Native is the implementation of a host function. For a method of an
// opaque type, args[0] is the receiver. An error it returns fails the run
// with function-error; a panic fails it with function-panic.
type Native func(c *Call, args []any) (any, error)

// Opaque is implemented by a host value that is an opaque value of a
// declared opaque type. OpaqueType returns the type's name. Two opaque
// values are == when they are == in Go.
type Opaque interface {
	OpaqueType() string
}

// Usage is what a run charged: steps and allocation units.
type Usage = eval.Usage

// Vars supplies the values of variables to a run. A run asks for each name
// at most once, and only for the names it reads.
type Vars = eval.Vars

// Map is a Vars backed by a Go map.
type Map map[string]any

// Var returns the value of a variable.
func (m Map) Var(name string) (any, bool) {
	v, ok := m[name]
	return v, ok
}

// Decl is one declaration of an environment.
type Decl interface{ apply(*Env) }

type declFunc func(*Env)

func (f declFunc) apply(e *Env) { f(e) }

// Version declares the specification version the environment wants. Only
// "3.0" is offered.
func Version(v string) Decl { return declFunc(func(e *Env) { e.version = v; e.versionSet = true }) }

// Var declares one variable with its type, in type text.
func Var(name, typeText string) Decl {
	return declFunc(func(e *Env) { e.addVar(name, typeText) })
}

// Declare declares one variable for each field of a record type written in
// type text, as a case file's env does: "{order: {total: float}, n: int}".
func Declare(recordType string) Decl {
	return declFunc(func(e *Env) { e.declare(recordType) })
}

// Function declares a host function. Name is "slug", or "geo.distance" in a
// namespace. Type is its signature in type text.
type Function struct {
	Name string
	Type string
	Impl Native
}

func (f Function) apply(e *Env) { e.addFunc(f) }

// OpaqueType declares an opaque type and its methods. Each method's Type is
// its signature in type text, without the receiver.
type OpaqueType struct {
	Name    string
	Methods map[string]Function
}

func (o OpaqueType) apply(e *Env) { e.addOpaque(o) }

// Limits declares the limits of a run (03-host.md 3.6). A zero field keeps
// the default: 1,000,000 steps, 1,000,000 units, no deadline, depth 200.
// NoLimit turns the step or allocation limit off.
type Limits struct {
	Steps   int64
	Alloc   int64
	Timeout time.Duration
	Depth   int
}

func (l Limits) apply(e *Env) {
	if l.Steps < ZeroLimit || l.Alloc < ZeroLimit || l.Timeout < 0 {
		e.errf(diag.CodeBadLimit, "a limit must be a whole number of 0 or more")
		return
	}
	switch l.Steps {
	case 0:
	case ZeroLimit:
		e.limits.Steps = 0
	default:
		e.limits.Steps = l.Steps
	}
	switch l.Alloc {
	case 0:
	case ZeroLimit:
		e.limits.Alloc = 0
	default:
		e.limits.Alloc = l.Alloc
	}
	if l.Timeout != 0 {
		e.limits.Timeout = l.Timeout
	}
	if l.Depth != 0 {
		if l.Depth < 1 || l.Depth > 1000 {
			e.errf(diag.CodeBadLimit, "the depth limit must be from 1 to 1000")
		}
		e.limits.Depth = l.Depth
	}
}

// Without removes library functions from the environment.
func Without(names ...string) Decl {
	return declFunc(func(e *Env) {
		for _, n := range names {
			e.removed[n] = true
		}
	})
}

// RejectClasses gives the whole set of rejected check classes.
func RejectClasses(classes ...string) Decl {
	return declFunc(func(e *Env) {
		for _, c := range classes {
			if !checkClasses[c] {
				e.errf(diag.CodeBadClass, fmt.Sprintf("%q is not a check class", c))
			}
		}
		e.reject = append(make([]string, 0, len(classes)), classes...)
	})
}

var checkClasses = map[string]bool{"constant-condition": true, "dynamic": true, "index": true,
	"divide": true, "value": true, "host": true}

type varDecl struct {
	name string
	typ  *typetext.Type
}

type funcDecl struct {
	name, ns string
	sig      *typetext.Sig
	impl     Native
}

type opaqueDecl struct {
	name    string
	methods map[string]*funcDecl
}

// Env is an environment: the declarations a program is compiled and run
// against. It does not change after NewEnv, and may be shared.
type Env struct {
	version    string
	versionSet bool
	vars       map[string]*varDecl
	funcs      map[string]*funcDecl
	opaque     map[string]*opaqueDecl
	removed    map[string]bool
	reject     []string
	limits     eval.Limits
	envTypes   // the declarations as types, for the checker (check.go)

	// while building
	own  map[string]bool
	errs []Diagnostic
}

// NewEnv builds an environment. A declaration error is an *Error of phase
// declare.
func NewEnv(decls ...Decl) (*Env, error) {
	e := &Env{
		version: "3.0", vars: map[string]*varDecl{}, funcs: map[string]*funcDecl{},
		opaque: map[string]*opaqueDecl{}, removed: map[string]bool{}, limits: eval.DefaultLimits,
	}
	return e.build(decls)
}

// With returns a child environment: its declarations replace the parent's
// of the same name (03-host.md 3.3.2).
func (e *Env) With(decls ...Decl) (*Env, error) {
	c := &Env{version: e.version, versionSet: e.versionSet, reject: e.reject, limits: e.limits,
		vars: map[string]*varDecl{}, funcs: map[string]*funcDecl{}, opaque: map[string]*opaqueDecl{},
		removed: map[string]bool{}}
	for k, v := range e.vars {
		c.vars[k] = v
	}
	for k, v := range e.funcs {
		c.funcs[k] = v
	}
	for k, v := range e.opaque {
		c.opaque[k] = v
	}
	for k, v := range e.removed {
		c.removed[k] = v
	}
	return c.build(decls)
}

func (e *Env) build(decls []Decl) (*Env, error) {
	e.own = map[string]bool{}
	for _, d := range decls {
		d.apply(e)
	}
	if e.version != "3.0" {
		e.errf(diag.CodeUnsupportedVersion, fmt.Sprintf("version %q is not offered", e.version))
	}
	e.checkClashes()
	e.typeEnv()
	e.own = nil
	if len(e.errs) > 0 {
		errs := e.errs
		e.errs = nil
		return nil, &Error{Diagnostics: errs}
	}
	return e, nil
}

func (e *Env) errf(code, msg string) {
	e.errs = append(e.errs, Diagnostic{Phase: PhaseDeclare, Code: code, Message: msg})
}

var reservedNamespaces = map[string]bool{"json": true, "time": true, "math": true, "base64": true}

var opaqueNameRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]+$`)

func (e *Env) checkName(n, what string) bool {
	if !syntax.IsName(n) {
		e.errf(diag.CodeBadName, fmt.Sprintf("%q is not a name for a %s", n, what))
		return false
	}
	return true
}

func (e *Env) addVar(name, text string) {
	t, err := typetext.ParseType(text)
	if err != nil {
		e.errf(diag.CodeBadType, fmt.Sprintf("variable %s: %v", name, err))
		return
	}
	e.putVar(name, t)
}

func (e *Env) putVar(name string, t *typetext.Type) {
	if !e.checkName(name, "variable") {
		return
	}
	if e.own["v:"+name] {
		e.errf(diag.CodeNameClash, fmt.Sprintf("variable %s is declared twice", name))
		return
	}
	e.own["v:"+name] = true
	e.vars[name] = &varDecl{name: name, typ: t}
}

func (e *Env) declare(text string) {
	t, err := typetext.ParseType(text)
	if err != nil {
		e.errf(diag.CodeBadType, fmt.Sprintf("env: %v", err))
		return
	}
	rec, ok := t.RecordFields()
	if !ok {
		e.errf(diag.CodeBadType, "env is not one record type")
		return
	}
	if rec.HasRest {
		e.errf(diag.CodeBadType, "the record of env has no rest")
		return
	}
	for _, f := range rec.Fields {
		if f.Optional {
			e.errf(diag.CodeBadType, "the record of env has no optional field")
			return
		}
	}
	for _, f := range rec.Fields {
		e.putVar(f.Key, f.Type)
	}
}

func (e *Env) parseFuncName(full string) (ns, name string, ok bool) {
	parts := strings.Split(full, ".")
	if len(parts) > 2 {
		e.errf(diag.CodeBadName, fmt.Sprintf("%q has more than one namespace", full))
		return "", "", false
	}
	for _, p := range parts {
		if !e.checkName(p, "function") {
			return "", "", false
		}
	}
	if len(parts) == 2 {
		if reservedNamespaces[parts[0]] {
			e.errf(diag.CodeReservedNamespace, fmt.Sprintf("namespace %s is reserved for the library", parts[0]))
			return "", "", false
		}
		return parts[0], parts[1], true
	}
	return "", parts[0], true
}

func (e *Env) parseSig(what, text string) (*typetext.Sig, bool) {
	sig, err := typetext.ParseSig(text)
	if err != nil {
		e.errf(diag.CodeBadSignature, fmt.Sprintf("%s: %v", what, err))
		return nil, false
	}
	optional := false
	for i, p := range sig.Params {
		if p.Fn != nil {
			e.errf(diag.CodeBadSignature, fmt.Sprintf("%s: a host function takes no function", what))
			return nil, false
		}
		// 05-types.md 3.6: optional parameters follow required ones; a rest
		// parameter is last and its type is a list.
		switch {
		case p.Rest:
			isList := len(p.Type.Alts) == 1 && p.Type.Alts[0].Lists > 0
			if i != len(sig.Params)-1 || p.Optional || !isList {
				e.errf(diag.CodeBadSignature, fmt.Sprintf("%s: a rest parameter is last and is a list", what))
				return nil, false
			}
		case p.Optional:
			optional = true
		case optional:
			e.errf(diag.CodeBadSignature, fmt.Sprintf("%s: an optional parameter comes before a required one", what))
			return nil, false
		}
	}
	return sig, true
}

func (e *Env) addFunc(f Function) {
	ns, name, ok := e.parseFuncName(f.Name)
	if !ok {
		return
	}
	sig, ok := e.parseSig(f.Name, f.Type)
	if !ok {
		return
	}
	if e.own["f:"+f.Name] {
		e.errf(diag.CodeNameClash, fmt.Sprintf("function %s is declared twice", f.Name))
		return
	}
	e.own["f:"+f.Name] = true
	e.funcs[f.Name] = &funcDecl{name: name, ns: ns, sig: sig, impl: f.Impl}
}

func (e *Env) addOpaque(o OpaqueType) {
	if !opaqueNameRe.MatchString(o.Name) {
		e.errf(diag.CodeBadName, fmt.Sprintf("%q is not an opaque type name", o.Name))
		return
	}
	if e.own["o:"+o.Name] {
		e.errf(diag.CodeNameClash, fmt.Sprintf("opaque type %s is declared twice", o.Name))
		return
	}
	e.own["o:"+o.Name] = true
	od := &opaqueDecl{name: o.Name, methods: map[string]*funcDecl{}}
	for m, f := range o.Methods {
		sig, ok := e.parseSig(o.Name+"."+m, f.Type)
		if !ok {
			continue
		}
		od.methods[m] = &funcDecl{name: m, sig: sig, impl: f.Impl}
	}
	e.opaque[o.Name] = od
}

// AddMethod declares a method of an opaque type declared in the same
// environment, as a case file's methods field does.
func AddMethod(typeName, method string, f Function) Decl {
	return declFunc(func(e *Env) {
		od, ok := e.opaque[typeName]
		if !ok {
			e.errf(diag.CodeBadSignature, fmt.Sprintf("%s.%s: %s is not a declared opaque type", typeName, method, typeName))
			return
		}
		sig, ok := e.parseSig(typeName+"."+method, f.Type)
		if !ok {
			return
		}
		cp := &opaqueDecl{name: od.name, methods: map[string]*funcDecl{}}
		for k, v := range od.methods {
			cp.methods[k] = v
		}
		cp.methods[method] = &funcDecl{name: method, sig: sig, impl: f.Impl}
		e.opaque[typeName] = cp
	})
}

// checkClashes applies the clashes of 03-host.md 3.3.4 between kinds.
func (e *Env) checkClashes() {
	namespaces := map[string]bool{}
	for _, f := range e.funcs {
		if f.ns != "" {
			namespaces[f.ns] = true
		}
	}
	names := make([]string, 0, len(e.funcs))
	for n := range e.funcs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f := e.funcs[n]
		if f.ns != "" {
			continue
		}
		if _, ok := e.vars[n]; ok {
			e.errf(diag.CodeNameClash, fmt.Sprintf("%s is a variable and a function", n))
		}
		if _, ok := library.Functions[n]; ok && !e.removed[n] {
			e.errf(diag.CodeNameClash, fmt.Sprintf("function %s has the name of a library function", n))
		}
		if namespaces[n] {
			e.errf(diag.CodeNameClash, fmt.Sprintf("%s is a function and a namespace", n))
		}
	}
	for ns := range namespaces {
		if _, ok := e.vars[ns]; ok {
			e.errf(diag.CodeNameClash, fmt.Sprintf("variable %s has the name of a namespace", ns))
		}
	}
}

// Compile applies the static rules (05-types.md 22) and compiles a parsed
// program. This implementation has no type checker yet: it compiles without
// the type rules. A static diagnostic fails the compile with phase check; a
// function with no implementation fails it with no-implementation.
func (e *Env) Compile(x *Expr, opts ...CheckOption) (*Program, error) {
	names := &static.Names{Vars: map[string]int{}, Funcs: map[string]*static.Func{},
		Namespaces: map[string]bool{}, Library: map[string]*library.Entry{}}
	cfg := &eval.Config{Names: names, Methods: map[string]map[string]*eval.Method{}, Limits: e.limits}
	varNames := make([]string, 0, len(e.vars))
	for n := range e.vars {
		varNames = append(varNames, n)
	}
	sort.Strings(varNames)
	for i, n := range varNames {
		names.Vars[n] = i
	}
	cfg.VarNames = varNames
	var missing []Diagnostic
	fnames := make([]string, 0, len(e.funcs))
	for n := range e.funcs {
		fnames = append(fnames, n)
	}
	sort.Strings(fnames)
	for _, n := range fnames {
		f := e.funcs[n]
		sf := &static.Func{Name: f.name, Namespace: f.ns, Sig: f.sig, Index: len(cfg.Hosts)}
		names.Funcs[n] = sf
		if f.ns != "" {
			names.Namespaces[f.ns] = true
		}
		if f.impl == nil {
			missing = append(missing, Diagnostic{Phase: PhaseDeclare, Code: diag.CodeNoImplementation,
				Message: "function " + n + " has no implementation"})
		}
		cfg.Hosts = append(cfg.Hosts, eval.HostFunc(f.impl))
	}
	for name, o := range e.opaque {
		ms := map[string]*eval.Method{}
		for m, f := range o.methods {
			if f.impl == nil {
				missing = append(missing, Diagnostic{Phase: PhaseDeclare, Code: diag.CodeNoImplementation,
					Message: "method " + name + "." + m + " has no implementation"})
			}
			ms[m] = &eval.Method{Name: m, Min: f.sig.MinArgs(), Max: f.sig.MaxArgs(), Impl: eval.HostFunc(f.impl)}
		}
		cfg.Methods[name] = ms
	}
	for n, en := range library.Functions {
		if !e.removed[n] {
			names.Library[n] = en
		}
	}
	if len(missing) > 0 {
		return nil, &Error{Diagnostics: missing}
	}
	res, cr, err := e.analyze(x, names, varNames, opts)
	if err != nil {
		return nil, err
	}
	p, err := eval.Compile(x.tree, x.file, res, cfg)
	if err != nil {
		return nil, &Error{Diagnostics: []Diagnostic{{Phase: PhaseRun, Code: "internal-error", Message: err.Error()}}}
	}
	return &Program{p: p, x: x, dynamic: dynamicSpans(x, cr)}, nil
}

// Program is a compiled program. It does not change, and several goroutines
// may run it at once.
type Program struct {
	p *eval.Program
	x *Expr
	// dynamic is set for a checked program: the spans at a dynamic site.
	dynamic map[[4]int]bool
}

// Run runs the program with the values of its variables. vars may be nil
// when the program reads none.
func (p *Program) Run(ctx context.Context, vars Vars) (any, error) {
	v, _, err := p.Measure(ctx, vars)
	return v, err
}

// Measure runs the program and also returns its usage, for a run that fails
// as for one that succeeds.
func (p *Program) Measure(ctx context.Context, vars Vars) (any, Usage, error) {
	if vars == nil {
		vars = Map(nil)
	}
	v, u, d := p.p.Run(ctx, vars)
	if d != nil {
		if p.dynamic != nil {
			checkedClass(d, p.dynamic)
		}
		return nil, u, &Error{Diagnostics: []Diagnostic{*d}}
	}
	return v, u, nil
}

// Expr returns the parsed program.
func (p *Program) Expr() *Expr { return p.x }
