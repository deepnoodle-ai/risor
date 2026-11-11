package risor

import (
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/deepnoodle-ai/risor/go/v3/internal/check"
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/go/v3/internal/library"
	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
)

// envCache is what every check and compile of one environment reads: the
// names of the static rules and the base of the compile's configuration.
// An environment does not change once built, so it is made once, on the
// first use, and shared by every goroutine.
type envCache struct {
	once     sync.Once
	names    *static.Names
	varNames []string
	cfg      eval.Config
	missing  []Diagnostic
	goTypes  map[string]reflect.Type
	// libFrom is the parent's cache, whose library names this one shares.
	libFrom *envCache
	ceOnce  sync.Once
	ce      *check.Env
}

// checkEnv is the environment the type rules read, made once.
func (c *envCache) checkEnv(e *Env) *check.Env {
	c.ceOnce.Do(func() { c.ce = e.checkEnv(c.names, c.varNames) })
	return c.ce
}

// scopeChanged reports whether this environment, while it is built,
// declares an opaque type or a named type: then the types of its inherited
// declarations are made again.
func (e *Env) scopeChanged() bool {
	for k := range e.own {
		if strings.HasPrefix(k, "o:") || strings.HasPrefix(k, "n:") {
			return true
		}
	}
	return false
}

func (e *Env) cached() *envCache {
	c := e.cache
	if c == nil { // an environment made outside build
		c = &envCache{}
		e.cache = c
	}
	c.once.Do(func() { e.fillCache(c) })
	return c
}

func (e *Env) fillCache(c *envCache) {
	names := &static.Names{Vars: make(map[string]int, len(e.vars)), Funcs: map[string]*static.Func{},
		Namespaces: map[string]bool{}, Library: map[string]*library.Entry{}, Types: e.scope()}
	varNames := make([]string, 0, len(e.vars))
	for n := range e.vars {
		varNames = append(varNames, n)
	}
	sort.Strings(varNames)
	for i, n := range varNames {
		names.Vars[n] = i
	}
	cfg := eval.Config{Names: names, Methods: map[string]map[string]*eval.Method{}, Limits: e.limits,
		Removed: e.removed, VarNames: varNames}
	fnames := make([]string, 0, len(e.funcs))
	for n := range e.funcs {
		fnames = append(fnames, n)
	}
	sort.Strings(fnames)
	for i, n := range fnames {
		f := e.funcs[n]
		names.Funcs[n] = &static.Func{Name: f.name, Namespace: f.ns, Sig: f.sig, Index: i}
		if f.ns != "" {
			names.Namespaces[f.ns] = true
		}
		if f.impl == nil {
			c.missing = append(c.missing, Diagnostic{Phase: PhaseDeclare, Code: diag.CodeNoImplementation,
				Message: "function " + n + " has no implementation"})
		}
		cfg.Hosts = append(cfg.Hosts, eval.HostFunc(f.impl))
	}
	onames := make([]string, 0, len(e.opaque))
	for n := range e.opaque {
		onames = append(onames, n)
	}
	sort.Strings(onames)
	for _, name := range onames {
		o := e.opaque[name]
		ms := map[string]*eval.Method{}
		mnames := make([]string, 0, len(o.methods))
		for m := range o.methods {
			mnames = append(mnames, m)
		}
		sort.Strings(mnames)
		for _, m := range mnames {
			f := o.methods[m]
			if f.impl == nil {
				c.missing = append(c.missing, Diagnostic{Phase: PhaseDeclare, Code: diag.CodeNoImplementation,
					Message: "method " + name + "." + m + " has no implementation"})
			}
			ms[m] = &eval.Method{Name: m, Min: f.sig.MinArgs(), Max: f.sig.MaxArgs(), Impl: eval.HostFunc(f.impl)}
		}
		cfg.Methods[name] = ms
	}
	if lf := c.libFrom; lf != nil {
		names.Library, names.Constants = lf.names.Library, lf.names.Constants
		for g := range e.groups {
			names.Namespaces[g] = true
		}
	} else {
		for n, en := range library.Functions {
			if !e.removed[n] {
				names.Library[n] = en
			}
		}
		e.groupNames(names)
	}
	c.libFrom = nil
	for _, n := range varNames {
		if rt := e.vars[n].rt; rt != nil {
			if c.goTypes == nil {
				c.goTypes = map[string]reflect.Type{}
			}
			c.goTypes[n] = rt
		}
	}
	c.names, c.varNames, c.cfg = names, varNames, cfg
}
