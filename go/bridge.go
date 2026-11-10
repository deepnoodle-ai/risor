package risor

import (
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/bridge"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
)

func init() {
	bridge.EnvTypes = func(env any) (map[string]*types.Type, map[string]*types.Sig, map[string]map[string]*types.Sig) {
		e := env.(*Env)
		return e.varTypes, e.hostSigs, e.methodSigs
	}
	bridge.FuncText = func(env any, name string) (string, bool) {
		e := env.(*Env)
		if f, ok := e.funcs[name]; ok && f.sig != nil {
			return f.sig.String(), true
		}
		if t, m, ok := strings.Cut(name, "."); ok {
			if o, ok := e.opaque[t]; ok {
				if f, ok := o.methods[m]; ok && f.sig != nil {
					return f.sig.String(), true
				}
			}
		}
		return "", false
	}
}
