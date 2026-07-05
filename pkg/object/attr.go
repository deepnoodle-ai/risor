package object

import "slices"

// AttrSpec describes an attribute available on an object.
// This provides metadata for introspection, documentation, and tooling.
type AttrSpec struct {
	// Name is the attribute name (e.g., "split", "append").
	Name string

	// Doc is a short description of what the attribute does.
	Doc string

	// Args lists parameter names (e.g., ["sep"] or ["old", "new"]).
	// Empty for attributes that take no arguments.
	Args []string

	// Variadic reports whether the final argument accepts any number of
	// values (zero or more). When true, the last entry in Args names the
	// variadic parameter.
	Variadic bool

	// Returns describes the return type (e.g., "list", "string", "bool").
	Returns string
}

// DisplayArgs returns the argument names formatted for display, with the final
// argument suffixed with "..." when the attribute is variadic. The returned
// slice is safe to modify; it never aliases Args.
func (s AttrSpec) DisplayArgs() []string {
	if !s.Variadic || len(s.Args) == 0 {
		return slices.Clone(s.Args)
	}
	args := slices.Clone(s.Args)
	args[len(args)-1] += "..."
	return args
}

// FuncSpec describes a builtin function.
// This provides metadata for introspection, documentation, and tooling.
type FuncSpec struct {
	// Name is the function name (e.g., "len", "sorted").
	Name string

	// Doc is a short description of what the function does.
	Doc string

	// Args lists parameter names (e.g., ["obj"] or ["items", "key"]).
	Args []string

	// Returns describes the return type (e.g., "int", "list").
	Returns string

	// Example shows a short usage example (optional).
	Example string
}

// TypeSpec describes a Risor type.
type TypeSpec struct {
	// Name is the type name (e.g., "string", "list").
	Name string

	// Doc is a description of the type.
	Doc string

	// Attrs lists the attributes/methods available on this type.
	Attrs []AttrSpec
}

// Introspectable is implemented by objects that can describe their attributes.
// This enables tooling like :methods, risor doc, and autocomplete.
type Introspectable interface {
	// Attrs returns the attribute specifications for this object.
	Attrs() []AttrSpec
}

// AttrNames returns just the attribute names from a slice of AttrSpec.
// This is a convenience helper for common use cases.
func AttrNames(attrs []AttrSpec) []string {
	names := make([]string, len(attrs))
	for i, attr := range attrs {
		names[i] = attr.Name
	}
	return names
}

// FindAttr searches for an attribute by name in a slice of AttrSpec.
// Returns the AttrSpec and true if found, or zero value and false if not.
func FindAttr(attrs []AttrSpec, name string) (AttrSpec, bool) {
	for _, attr := range attrs {
		if attr.Name == name {
			return attr, true
		}
	}
	return AttrSpec{}, false
}
