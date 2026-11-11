package risor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
)

// Host facts, package F14 (spec/packages/f14-host-facts.md): statements an
// environment declares about its variables' values, beyond their types.

// hostFact is one host fact of the kind someNotNull: its members, as
// declared.
type hostFact struct {
	members []string
}

// SomeNotNull declares a host fact (F14 2.2): in every run, at least one of
// the variables named is not null. It is part of the precondition, as a
// declared type is: the host promises it, and nothing tests it at a run.
// The checker uses it where a program reads a member after testing every
// other member null, as in report ?? approve, which then lacks null.
//
// The names are variables of the environment or of its parent; a host
// fact may come before them among the declarations. A list with no name,
// a name that is no variable, and a variable named twice are the declare
// error bad-name. A child keeps its parent's host facts, less those that
// name a variable it replaces (F14 2.6).
//
// Every member must be a variable that the same environment declares. A
// host that declares only the variables a program reads must filter its
// facts to those names: it keeps a fact whose members are all declared and
// leaves out every other fact whole. It must not drop a member from a fact,
// which would make the fact claim more: of a and b, one is not null does
// not say that a is not null.
//
// The fact is about values, not about which step ran. A member whose value
// can be null even when its step ran, such as the output of a step that may
// give null, must not be in a fact.
func SomeNotNull(names ...string) Decl {
	return declFunc(func(e *Env) {
		e.ownFacts = append(e.ownFacts, hostFact{members: append([]string(nil), names...)})
	})
}

// buildFacts settles the host facts of an environment being built (F14
// 2.5, 2.6): the parent's kept, then its own, each tested against the
// variables.
func (e *Env) buildFacts() {
	var kept []hostFact
	for _, f := range e.facts { // inherited
		ok := true
		for _, m := range f.members {
			if e.own["v:"+m] || e.vars[m] == nil {
				ok = false
				break
			}
		}
		if ok {
			kept = append(kept, f)
		}
	}
	for _, f := range e.ownFacts {
		if len(f.members) == 0 {
			e.errf(diag.CodeBadName, "a host fact someNotNull names no variable")
			continue
		}
		bad := false
		seen := map[string]bool{}
		for _, m := range f.members {
			if e.vars[m] == nil {
				e.errf(diag.CodeBadName, fmt.Sprintf("the host fact someNotNull names %q, which is no variable of the environment", m))
				bad = true
				continue
			}
			if seen[m] {
				e.errf(diag.CodeBadName, fmt.Sprintf("the host fact someNotNull names the variable %s twice", m))
				bad = true
				continue
			}
			seen[m] = true
		}
		if !bad {
			kept = append(kept, f)
		}
	}
	e.facts, e.ownFacts = kept, nil
}

// factIndexes gives the host facts as the indexes of their members in the
// sorted variable names.
func (e *Env) factIndexes(varNames []string) [][]int {
	if len(e.facts) == 0 {
		return nil
	}
	idx := make(map[string]int, len(varNames))
	for i, n := range varNames {
		idx[n] = i
	}
	out := make([][]int, len(e.facts))
	for i, f := range e.facts {
		for _, m := range f.members {
			out[i] = append(out[i], idx[m])
		}
	}
	return out
}

// describedFacts are the host facts as the description gives them (F14
// 9.1): each one's members sorted, distinct facts once, ordered by their
// lists of members.
func (e *Env) describedFacts() [][]string {
	seen := map[string]bool{}
	var out [][]string
	for _, f := range e.facts {
		ms := append([]string(nil), f.members...)
		sort.Strings(ms)
		k := strings.Join(ms, "\x00")
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, ms)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
	return out
}
