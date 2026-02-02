package builtins

import "github.com/deepnoodle-ai/risor/v2/pkg/object"

// Entry defines a builtin function along with its documentation.
// This ensures documentation stays in sync with implementations.
type Entry struct {
	Name    string
	Fn      object.BuiltinFunction
	Doc     string
	Args    []string
	Returns string
	Example string
}

// Registry holds all builtin function definitions.
// The documentation and implementation are defined together.
var registry = []Entry{
	{
		Name:    "all",
		Fn:      All,
		Doc:     "Return true if all elements are truthy",
		Args:    []string{"items"},
		Returns: "bool",
		Example: "all([true, 1, \"yes\"])",
	},
	{
		Name:    "any",
		Fn:      Any,
		Doc:     "Return true if any element is truthy",
		Args:    []string{"items"},
		Returns: "bool",
		Example: "any([false, 0, \"yes\"])",
	},
	{
		Name:    "assert",
		Fn:      Assert,
		Doc:     "Raise an error if condition is false",
		Args:    []string{"condition", "message?"},
		Returns: "nil",
		Example: "assert(x > 0, \"x must be positive\")",
	},
	{
		Name:    "bool",
		Fn:      Bool,
		Doc:     "Convert value to boolean",
		Args:    []string{"value?"},
		Returns: "bool",
		Example: "bool(1)",
	},
	{
		Name:    "byte",
		Fn:      Byte,
		Doc:     "Convert value to byte (0-255)",
		Args:    []string{"value?"},
		Returns: "byte",
		Example: "byte(65)",
	},
	{
		Name:    "bytes",
		Fn:      Bytes,
		Doc:     "Convert value to bytes",
		Args:    []string{"value?"},
		Returns: "bytes",
		Example: "bytes(\"hello\")",
	},
	{
		Name:    "call",
		Fn:      Call,
		Doc:     "Call a function with arguments",
		Args:    []string{"fn", "args..."},
		Returns: "any",
		Example: "call(add, 1, 2)",
	},
	{
		Name:    "chunk",
		Fn:      Chunk,
		Doc:     "Split list into chunks of size n",
		Args:    []string{"list", "size"},
		Returns: "list",
		Example: "chunk([1, 2, 3, 4, 5], 2)",
	},
	{
		Name:    "coalesce",
		Fn:      Coalesce,
		Doc:     "Return first non-nil argument",
		Args:    []string{"values..."},
		Returns: "any",
		Example: "coalesce(nil, nil, \"default\")",
	},
	{
		Name:    "decode",
		Fn:      Decode,
		Doc:     "Decode data from a format (json, base64, hex, etc.)",
		Args:    []string{"format", "data"},
		Returns: "any",
		Example: "decode(\"json\", '{\"a\": 1}')",
	},
	{
		Name:    "encode",
		Fn:      Encode,
		Doc:     "Encode data to a format (json, base64, hex, etc.)",
		Args:    []string{"format", "value"},
		Returns: "string",
		Example: "encode(\"json\", {a: 1})",
	},
	{
		Name:    "error",
		Fn:      Error,
		Doc:     "Create an error value (does not throw)",
		Args:    []string{"message", "args..."},
		Returns: "error",
		Example: "error(\"file %s not found\", name)",
	},
	{
		Name:    "filter",
		Fn:      Filter,
		Doc:     "Keep elements where fn returns true",
		Args:    []string{"items", "fn"},
		Returns: "list",
		Example: "filter([1, 2, 3, 4], x => x > 2)",
	},
	{
		Name:    "float",
		Fn:      Float,
		Doc:     "Convert value to float",
		Args:    []string{"value?"},
		Returns: "float",
		Example: "float(\"3.14\")",
	},
	{
		Name:    "getattr",
		Fn:      GetAttr,
		Doc:     "Get attribute from object with optional default",
		Args:    []string{"obj", "name", "default?"},
		Returns: "any",
		Example: "getattr(obj, \"name\", \"unknown\")",
	},
	{
		Name:    "int",
		Fn:      Int,
		Doc:     "Convert value to integer",
		Args:    []string{"value?"},
		Returns: "int",
		Example: "int(\"42\")",
	},
	{
		Name:    "keys",
		Fn:      Keys,
		Doc:     "Get keys from map or indices from list",
		Args:    []string{"container"},
		Returns: "list",
		Example: "keys({a: 1, b: 2})",
	},
	{
		Name:    "len",
		Fn:      Len,
		Doc:     "Return length of container",
		Args:    []string{"container"},
		Returns: "int",
		Example: "len([1, 2, 3])",
	},
	{
		Name:    "list",
		Fn:      List,
		Doc:     "Convert enumerable to list",
		Args:    []string{"enumerable?"},
		Returns: "list",
		Example: "list(range(5))",
	},
	{
		Name:    "range",
		Fn:      Range,
		Doc:     "Generate a sequence of integers",
		Args:    []string{"start_or_stop", "stop?", "step?"},
		Returns: "range",
		Example: "range(1, 10, 2)",
	},
	{
		Name:    "reversed",
		Fn:      Reversed,
		Doc:     "Return reversed copy of list or string",
		Args:    []string{"sequence"},
		Returns: "list|string",
		Example: "reversed([1, 2, 3])",
	},
	{
		Name:    "sorted",
		Fn:      Sorted,
		Doc:     "Return sorted copy of list",
		Args:    []string{"items", "key?"},
		Returns: "list",
		Example: "sorted([3, 1, 2])",
	},
	{
		Name:    "sprintf",
		Fn:      Sprintf,
		Doc:     "Format string with arguments",
		Args:    []string{"format", "args..."},
		Returns: "string",
		Example: "sprintf(\"%s: %d\", \"count\", 42)",
	},
	{
		Name:    "string",
		Fn:      String,
		Doc:     "Convert value to string",
		Args:    []string{"value?"},
		Returns: "string",
		Example: "string(123)",
	},
	{
		Name:    "type",
		Fn:      Type,
		Doc:     "Return type name of value",
		Args:    []string{"value"},
		Returns: "string",
		Example: "type([1, 2, 3])",
	},
}

// Builtins returns all builtin functions as a map for use by the VM.
func Builtins() map[string]object.Object {
	result := make(map[string]object.Object, len(registry))
	for _, entry := range registry {
		result[entry.Name] = object.NewBuiltin(entry.Name, entry.Fn)
	}
	return result
}

// Docs returns documentation for all builtin functions.
func Docs() []object.FuncSpec {
	specs := make([]object.FuncSpec, len(registry))
	for i, entry := range registry {
		specs[i] = object.FuncSpec{
			Name:    entry.Name,
			Doc:     entry.Doc,
			Args:    entry.Args,
			Returns: entry.Returns,
			Example: entry.Example,
		}
	}
	return specs
}
