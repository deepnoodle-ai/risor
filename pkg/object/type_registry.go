package object

// typeDocEntry holds documentation for a type.
type typeDocEntry struct {
	name        string
	description string
	attrsFn     func() []AttrSpec
}

// typeRegistry holds type documentation.
var typeRegistry = map[Type]typeDocEntry{}

// RegisterType registers a type with its documentation.
// This should be called from init() functions in type implementation files.
func RegisterType(t Type, description string, attrsFn func() []AttrSpec) {
	typeRegistry[t] = typeDocEntry{
		name:        string(t),
		description: description,
		attrsFn:     attrsFn,
	}
}

// TypeDocs returns documentation for all registered types.
func TypeDocs() []TypeSpec {
	specs := make([]TypeSpec, 0, len(typeRegistry))
	for _, entry := range typeRegistry {
		var attrs []AttrSpec
		if entry.attrsFn != nil {
			attrs = entry.attrsFn()
		}
		specs = append(specs, TypeSpec{
			Name:  entry.name,
			Doc:   entry.description,
			Attrs: attrs,
		})
	}
	return specs
}

// TypeDoc returns documentation for a specific type.
func TypeDoc(t Type) (TypeSpec, bool) {
	entry, ok := typeRegistry[t]
	if !ok {
		return TypeSpec{}, false
	}
	var attrs []AttrSpec
	if entry.attrsFn != nil {
		attrs = entry.attrsFn()
	}
	return TypeSpec{
		Name:  entry.name,
		Doc:   entry.description,
		Attrs: attrs,
	}, true
}

// TypeDocsMap returns documentation for all registered types as a map.
func TypeDocsMap() map[string]TypeSpec {
	result := make(map[string]TypeSpec, len(typeRegistry))
	for _, entry := range typeRegistry {
		var attrs []AttrSpec
		if entry.attrsFn != nil {
			attrs = entry.attrsFn()
		}
		result[entry.name] = TypeSpec{
			Name:  entry.name,
			Doc:   entry.description,
			Attrs: attrs,
		}
	}
	return result
}
