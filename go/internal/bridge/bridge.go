// Package bridge lets the runner read what an environment declares, as
// types, without adding to the binding's exported API. The root package
// sets the functions when it is loaded.
package bridge

import (
	"reflect"

	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
)

// EnvTypes gives an environment's declarations as types: each variable's
// type, each host function's signature by its full name, and each method's
// signature by type and name. env is a *risor.Env.
var EnvTypes func(env any) (vars map[string]*types.Type, funcs map[string]*types.Sig, methods map[string]map[string]*types.Sig)

// FuncText gives the signature text of a host function or method an
// environment declares: name is "f", "ns.f" or "T.m". env is a *risor.Env.
var FuncText func(env any, name string) (string, bool)

// TypeOf is risor.TypeOf for a Go type known at run time; it gives a
// risor.Type.
var TypeOf func(rt reflect.Type) (any, error)

// TypedPlans counts the reads the typed layer planned in a *risor.Program
// (package F11), for tests.
var TypedPlans func(prog any) int
