package runner

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/bridge"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// The differential test of package F11: every case of the suite whose vars
// can be decoded into generated Go struct types is run with those structs,
// declared from their Go types, with the typed layer on and off. The two
// outcomes must be the same: value, failure, steps and units.

var simpleKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var anyType = reflect.TypeFor[any]()

// genType gives a Go type that holds a JSON value: a struct for an object
// with simple keys, a slice of one struct for a list of objects of one
// shape, and a scalar type for a scalar. ok is false when the value has
// no struct to make.
func genType(v any, depth int) (reflect.Type, bool) {
	if depth > 20 {
		return nil, false
	}
	switch x := v.(type) {
	case nil:
		return anyType, true
	case bool:
		return reflect.TypeFor[bool](), true
	case string:
		return reflect.TypeFor[string](), true
	case float64:
		if value.IsWhole(x) {
			return reflect.TypeFor[int64](), true
		}
		return reflect.TypeFor[float64](), true
	case []any:
		if len(x) == 0 {
			return reflect.TypeFor[[]any](), true
		}
		first, ok := genType(x[0], depth+1)
		if !ok {
			return nil, false
		}
		for _, e := range x[1:] {
			t, ok := genType(e, depth+1)
			if !ok || t != first {
				return reflect.TypeFor[[]any](), true
			}
		}
		return reflect.SliceOf(first), true
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			if !simpleKey.MatchString(k) {
				return nil, false
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fs := make([]reflect.StructField, len(keys))
		for i, k := range keys {
			t, ok := genType(x[k], depth+1)
			if !ok {
				return nil, false
			}
			fs[i] = reflect.StructField{Name: fmt.Sprintf("F%d", i), Type: t, Tag: reflect.StructTag(`json:"` + k + `"`)}
		}
		return reflect.StructOf(fs), true
	}
	return nil, false
}

// structVars decodes each var whose value is an object into a generated
// struct, alternately by value and by pointer.
func structVars(vars map[string]any) (map[string]any, map[string]reflect.Type, bool) {
	out := map[string]any{}
	types := map[string]reflect.Type{}
	n := 0
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := vars[k]
		out[k] = v
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		t, ok := genType(m, 0)
		if !ok || t.Kind() != reflect.Struct {
			continue
		}
		p := reflect.New(t)
		if err := risor.Decode(v, p.Interface()); err != nil {
			continue
		}
		if n%2 == 0 {
			out[k], types[k] = p.Interface(), t
		} else {
			out[k], types[k] = p.Elem().Interface(), t
		}
		n++
	}
	return out, types, n > 0
}

type typedOutcome struct {
	plans                   int
	phase, code, class, msg string
	span                    string
	v                       any
	steps, units            int64
}

func (a typedOutcome) same(b typedOutcome) bool {
	if a.phase != b.phase || a.code != b.code || a.class != b.class || a.msg != b.msg || a.span != b.span ||
		a.steps != b.steps || a.units != b.units {
		return false
	}
	return value.Equal(a.v, b.v) && reflect.TypeOf(a.v) == reflect.TypeOf(b.v) && jsonForm(a.v) == jsonForm(b.v)
}

func runTyped(c *casefile.Case, vars map[string]any, types map[string]reflect.Type, on bool) (typedOutcome, bool) {
	var o typedOutcome
	env, err := buildEnv(nil, c, &stubState{})
	if err != nil {
		return o, false
	}
	decls := []risor.Decl{risor.TypedLayer(on)}
	for k, rt := range types {
		t, err := typeOfReflect(rt)
		if err != nil {
			return o, false
		}
		decls = append(decls, risor.VarType(k, t))
	}
	env, err = env.With(decls...)
	if err != nil {
		return o, false
	}
	x, err := parseCase(c)
	if err != nil {
		return o, false
	}
	var opts []risor.CheckOption
	if c.Returns != nil && *c.Returns != "condition" {
		opts = append(opts, risor.Expect(*c.Returns))
	}
	prog, err := env.Compile(x, opts...)
	if err != nil {
		var e *risor.Error
		if errors.As(err, &e) {
			d := e.Diagnostics[0]
			o.phase, o.code = string(d.Phase), d.Code
		}
		return o, false
	}
	o.plans = bridge.TypedPlans(prog)
	v, u, err := prog.Measure(context.Background(), risor.Map(vars))
	o.steps, o.units = u.Steps, u.Alloc
	if err != nil {
		var e *risor.Error
		if errors.As(err, &e) {
			d := e.Diagnostics[0]
			o.phase, o.code, o.class, o.msg = string(d.Phase), d.Code, d.Class, d.Message
			if d.Span != nil {
				o.span = d.Span.String()
			}
		}
		return o, true
	}
	o.v = v
	return o, true
}

func typeOfReflect(rt reflect.Type) (risor.Type, error) {
	t, err := bridge.TypeOf(rt)
	if err != nil {
		return risor.Type{}, err
	}
	return t.(risor.Type), nil
}

func TestTypedLayerDifferential(t *testing.T) {
	root := filepath.Join("..", "..", "..", "suite")
	if _, err := os.Stat(root); err != nil {
		t.Skip("no suite")
	}
	ran, compiled, considered, planned := 0, 0, 0, 0
	for _, d := range SuiteDirs {
		files, err := Files(filepath.Join(root, d))
		if err != nil {
			continue
		}
		for _, path := range files {
			f, err := casefile.Load(path)
			if err != nil {
				continue
			}
			for _, c := range f.Cases {
				if c.Skip != nil || c.Vars == nil || c.Conform != nil || c.Render != nil || c.Expect == nil ||
					c.Expect.Runner != nil || c.Expect.Analysis != nil || (c.Check != nil && !*c.Check) {
					continue
				}
				vars, types, ok := structVars(c.Vars)
				if !ok {
					continue
				}
				considered++
				on, okOn := runTyped(c, vars, types, true)
				off, okOff := runTyped(c, vars, types, false)
				if okOn != okOff || !on.same(off) {
					t.Errorf("%s: %s: typed layer on %+v, off %+v", filepath.Base(path), c.Name, on, off)
					continue
				}
				if okOn {
					ran++
					if on.plans > 0 {
						planned++
					}
				}
				if on.phase != "check" && on.phase != "declare" {
					compiled++
				} else if os.Getenv("RISOR_TYPED_DEBUG") != "" {
					t.Logf("not compiled: %s: %s: %s %s", filepath.Base(path), c.Name, on.phase, on.code)
				}
			}
		}
	}
	t.Logf("cases with struct vars: %d; compiled: %d; ran both ways and compared: %d, of which the typed layer planned reads in %d", considered, compiled, ran, planned)
	if ran == 0 {
		t.Fatal("no case ran")
	}
	_ = math.Inf
	_ = strings.TrimSpace
}
