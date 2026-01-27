package object

import "time"

func init() {
	// Register all types with their documentation.
	// Method lists are generated dynamically from the type implementations.

	RegisterType(STRING, "Immutable sequence of Unicode characters", func() []AttrSpec {
		return NewString("").Attrs()
	})

	RegisterType(LIST, "Mutable ordered collection of values", func() []AttrSpec {
		return NewList(nil).Attrs()
	})

	RegisterType(MAP, "Mutable key-value mapping with string keys", func() []AttrSpec {
		return NewMap(nil).Attrs()
	})

	RegisterType(INT, "64-bit signed integer", nil)

	RegisterType(FLOAT, "64-bit floating point number", nil)

	RegisterType(BOOL, "Boolean value (true or false)", nil)

	RegisterType(BYTE, "Single byte value (0-255)", nil)

	RegisterType(BYTES, "Mutable sequence of bytes", func() []AttrSpec {
		return NewBytes(nil).Attrs()
	})

	RegisterType(TIME, "Point in time with nanosecond precision", func() []AttrSpec {
		return NewTime(time.Now()).Attrs()
	})

	RegisterType(NIL, "Absence of a value", nil)

	RegisterType(ERROR, "Error value that can be thrown or returned", func() []AttrSpec {
		return NewError(nil).Attrs()
	})

	RegisterType(FUNCTION, "User-defined function or closure", nil)

	RegisterType(BUILTIN, "Built-in function implemented in Go", func() []AttrSpec {
		return NewNoopBuiltin("").Attrs()
	})

	// Module attrs are dynamic based on contents, just register __name__
	RegisterType(MODULE, "Collection of related functions and values", nil)

	RegisterType("range", "Lazy sequence of integers", func() []AttrSpec {
		return NewRange(0, 0, 1).Attrs()
	})
}
