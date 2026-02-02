package rand

import "github.com/deepnoodle-ai/risor/v2/object"

// Docs returns documentation for the rand module.
func Docs() []object.FuncSpec {
	return randDocs
}

// ModuleDoc returns the module-level documentation.
func ModuleDoc() string {
	return "Random number generation"
}

var randDocs = []object.FuncSpec{
	{Name: "random", Doc: "Random float in [0.0, 1.0)", Returns: "float"},
	{Name: "int", Doc: "Random integer", Args: []string{"max?"}, Returns: "int"},
	{Name: "randint", Doc: "Random int in [a, b] inclusive", Args: []string{"a", "b"}, Returns: "int"},
	{Name: "uniform", Doc: "Random float in [a, b]", Args: []string{"a", "b"}, Returns: "float"},
	{Name: "normal", Doc: "Random from normal distribution", Args: []string{"mu?", "sigma?"}, Returns: "float"},
	{Name: "exponential", Doc: "Random from exponential distribution", Args: []string{"lambda?"}, Returns: "float"},
	{Name: "choice", Doc: "Random element from list", Args: []string{"list"}, Returns: "any"},
	{Name: "sample", Doc: "Random k elements from list", Args: []string{"list", "k"}, Returns: "list"},
	{Name: "shuffle", Doc: "Shuffle list in place", Args: []string{"list"}, Returns: "list"},
	{Name: "bytes", Doc: "Random bytes", Args: []string{"n"}, Returns: "list"},
}
