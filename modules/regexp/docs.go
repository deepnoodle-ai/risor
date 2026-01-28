package regexp

import "github.com/risor-io/risor/object"

// Docs returns documentation for the regexp module.
func Docs() []object.FuncSpec {
	return regexpDocs
}

// ModuleDoc returns the module-level documentation.
func ModuleDoc() string {
	return "Regular expression matching"
}

var regexpDocs = []object.FuncSpec{
	{Name: "compile", Doc: "Compile a regular expression", Args: []string{"pattern"}, Returns: "regexp"},
	{Name: "match", Doc: "Check if pattern matches string", Args: []string{"pattern", "s"}, Returns: "bool"},
	{Name: "find", Doc: "Find first match", Args: []string{"pattern", "s"}, Returns: "string|nil"},
	{Name: "find_all", Doc: "Find all matches", Args: []string{"pattern", "s", "n?"}, Returns: "list"},
	{Name: "search", Doc: "Find index of first match", Args: []string{"pattern", "s"}, Returns: "int"},
	{Name: "replace", Doc: "Replace matches", Args: []string{"pattern", "s", "repl", "count?"}, Returns: "string"},
	{Name: "split", Doc: "Split by pattern", Args: []string{"pattern", "s", "n?"}, Returns: "list"},
	{Name: "escape", Doc: "Escape metacharacters", Args: []string{"s"}, Returns: "string"},
}
