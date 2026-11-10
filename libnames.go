package risor

import (
	"sort"

	"github.com/deepnoodle-ai/risor/v3/internal/library"
)

// LibraryNames are the names of the library's entries an environment has:
// those of Core and of the groups it turns on, less the entries it removed.
type LibraryNames struct {
	// Functions are the functions called by name, such as len, sorted. A
	// variable or a host function of one of these names shadows it or
	// clashes with it (03-host.md 3.3.4).
	Functions []string
	// Namespaces are the groups that are on, such as json, sorted. A
	// variable of one of these names clashes with the group.
	Namespaces []string
	// GroupFunctions are the entries of those groups by full name, such as
	// json.parse, sorted.
	GroupFunctions []string
	// Methods are the methods by the kind of their receiver: "string",
	// "list" and "map", each sorted.
	Methods map[string][]string
}

// LibraryNames lists the names of the library's entries the environment
// has, so that a host can tell which names a variable would shadow.
func (e *Env) LibraryNames() LibraryNames {
	out := LibraryNames{Methods: map[string][]string{}}
	for n := range library.Functions {
		if !e.removed[n] {
			out.Functions = append(out.Functions, n)
		}
	}
	for g := range e.groups {
		out.Namespaces = append(out.Namespaces, g)
	}
	for n := range library.GroupFunctions {
		if e.groups[library.Namespace(n)] && !e.removed[n] {
			out.GroupFunctions = append(out.GroupFunctions, n)
		}
	}
	for _, en := range library.Entries {
		if !en.IsMethod() {
			continue
		}
		k := library.KindWord(en.Kind)
		if !e.removed[k+"."+en.Name] {
			out.Methods[k] = append(out.Methods[k], en.Name)
		}
	}
	sort.Strings(out.Functions)
	sort.Strings(out.Namespaces)
	sort.Strings(out.GroupFunctions)
	for k := range out.Methods {
		sort.Strings(out.Methods[k])
	}
	return out
}
