package risor

import "github.com/deepnoodle-ai/risor/go/v3/internal/types"

// ReflectiveFunc is Func with the adapter by reflection whatever fn's
// shape, for the tests of package risor_test that compare the two paths.
func ReflectiveFunc(name string, fn any, opts ...FuncOption) Function {
	return bindFunc(name, fn, opts, false)
}

// MethodName gives the Risor name Methods derives from a Go method name.
func MethodName(goName string) string { return methodName(goName) }

// ReflectiveMethods is Methods with every method adapted by reflection.
func ReflectiveMethods[T any](name string, opts ...MethodOption) OpaqueType {
	return bindMethods[T](name, opts, false)
}

// TypeSize is the size of a type (05-types.md 20.7), and DefSize the size
// of the definition of the recursive type it names, or -1.
func TypeSize(t Type) int { return types.Size(t.inner()) }

func DefSize(t Type) int {
	if n, ok := t.inner().RefOnly(); ok && n.Def() != nil {
		return types.Size(n.Def())
	}
	return -1
}
