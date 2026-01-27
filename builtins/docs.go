package builtins

import "github.com/risor-io/risor/object"

// Docs returns documentation for all builtin functions.
func Docs() []object.FuncSpec {
	return builtinDocs
}

var builtinDocs = []object.FuncSpec{
	{
		Name:    "all",
		Doc:     "Return true if all elements are truthy",
		Args:    []string{"items"},
		Returns: "bool",
		Example: "all([true, 1, \"yes\"])",
	},
	{
		Name:    "any",
		Doc:     "Return true if any element is truthy",
		Args:    []string{"items"},
		Returns: "bool",
		Example: "any([false, 0, \"yes\"])",
	},
	{
		Name:    "assert",
		Doc:     "Raise an error if condition is false",
		Args:    []string{"condition", "message?"},
		Returns: "nil",
		Example: "assert(x > 0, \"x must be positive\")",
	},
	{
		Name:    "bool",
		Doc:     "Convert value to boolean",
		Args:    []string{"value?"},
		Returns: "bool",
		Example: "bool(1)",
	},
	{
		Name:    "byte",
		Doc:     "Convert value to byte (0-255)",
		Args:    []string{"value?"},
		Returns: "byte",
		Example: "byte(65)",
	},
	{
		Name:    "call",
		Doc:     "Call a function with arguments",
		Args:    []string{"fn", "args..."},
		Returns: "any",
		Example: "call(add, 1, 2)",
	},
	{
		Name:    "chunk",
		Doc:     "Split list into chunks of size n",
		Args:    []string{"list", "size"},
		Returns: "list",
		Example: "chunk([1, 2, 3, 4, 5], 2)",
	},
	{
		Name:    "coalesce",
		Doc:     "Return first non-nil argument",
		Args:    []string{"values..."},
		Returns: "any",
		Example: "coalesce(nil, nil, \"default\")",
	},
	{
		Name:    "decode",
		Doc:     "Decode data from a format (json, base64, hex, etc.)",
		Args:    []string{"format", "data"},
		Returns: "any",
		Example: "decode(\"json\", '{\"a\": 1}')",
	},
	{
		Name:    "encode",
		Doc:     "Encode data to a format (json, base64, hex, etc.)",
		Args:    []string{"format", "value"},
		Returns: "string",
		Example: "encode(\"json\", {a: 1})",
	},
	{
		Name:    "error",
		Doc:     "Create an error value (does not throw)",
		Args:    []string{"message", "args..."},
		Returns: "error",
		Example: "error(\"file %s not found\", name)",
	},
	{
		Name:    "filter",
		Doc:     "Keep elements where fn returns true",
		Args:    []string{"items", "fn"},
		Returns: "list",
		Example: "filter([1, 2, 3, 4], x => x > 2)",
	},
	{
		Name:    "float",
		Doc:     "Convert value to float",
		Args:    []string{"value?"},
		Returns: "float",
		Example: "float(\"3.14\")",
	},
	{
		Name:    "getattr",
		Doc:     "Get attribute from object with optional default",
		Args:    []string{"obj", "name", "default?"},
		Returns: "any",
		Example: "getattr(obj, \"name\", \"unknown\")",
	},
	{
		Name:    "int",
		Doc:     "Convert value to integer",
		Args:    []string{"value?"},
		Returns: "int",
		Example: "int(\"42\")",
	},
	{
		Name:    "keys",
		Doc:     "Get keys from map or indices from list",
		Args:    []string{"container"},
		Returns: "list",
		Example: "keys({a: 1, b: 2})",
	},
	{
		Name:    "len",
		Doc:     "Return length of container",
		Args:    []string{"container"},
		Returns: "int",
		Example: "len([1, 2, 3])",
	},
	{
		Name:    "list",
		Doc:     "Convert enumerable to list",
		Args:    []string{"enumerable?"},
		Returns: "list",
		Example: "list(range(5))",
	},
	{
		Name:    "range",
		Doc:     "Generate a sequence of integers",
		Args:    []string{"start_or_stop", "stop?", "step?"},
		Returns: "range",
		Example: "range(1, 10, 2)",
	},
	{
		Name:    "reversed",
		Doc:     "Return reversed copy of list or string",
		Args:    []string{"sequence"},
		Returns: "list|string",
		Example: "reversed([1, 2, 3])",
	},
	{
		Name:    "sorted",
		Doc:     "Return sorted copy of list",
		Args:    []string{"items", "key?"},
		Returns: "list",
		Example: "sorted([3, 1, 2])",
	},
	{
		Name:    "sprintf",
		Doc:     "Format string with arguments",
		Args:    []string{"format", "args..."},
		Returns: "string",
		Example: "sprintf(\"%s: %d\", \"count\", 42)",
	},
	{
		Name:    "string",
		Doc:     "Convert value to string",
		Args:    []string{"value?"},
		Returns: "string",
		Example: "string(123)",
	},
	{
		Name:    "type",
		Doc:     "Return type name of value",
		Args:    []string{"value"},
		Returns: "string",
		Example: "type([1, 2, 3])",
	},
}
