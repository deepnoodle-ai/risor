package risor

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
