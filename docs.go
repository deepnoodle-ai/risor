package risor

import (
	"encoding/json"
	"sort"
	stdtime "time"

	"github.com/risor-io/risor/builtins"
	"github.com/risor-io/risor/object"
)

// DocsOption configures documentation retrieval.
type DocsOption func(*docsOptions)

type docsOptions struct {
	category string
	topic    string
	quick    bool
	all      bool
}

// DocsCategory filters documentation to a specific category.
// Valid categories: "builtins", "types", "modules", "syntax", "errors"
func DocsCategory(cat string) DocsOption {
	return func(o *docsOptions) {
		o.category = cat
	}
}

// DocsTopic retrieves documentation for a specific topic.
// Examples: "filter", "string", "math.sqrt"
func DocsTopic(topic string) DocsOption {
	return func(o *docsOptions) {
		o.topic = topic
	}
}

// DocsQuick returns a concise quick reference.
func DocsQuick() DocsOption {
	return func(o *docsOptions) {
		o.quick = true
	}
}

// DocsAll returns complete documentation (may be large).
func DocsAll() DocsOption {
	return func(o *docsOptions) {
		o.all = true
	}
}

// Documentation provides structured access to Risor documentation.
type Documentation struct {
	data any
}

// JSON returns the documentation as a JSON string.
func (d *Documentation) JSON() string {
	b, _ := json.MarshalIndent(d.data, "", "  ")
	return string(b)
}

// Data returns the raw documentation data.
func (d *Documentation) Data() any {
	return d.data
}

// Version is the current Risor version.
const Version = "2.0.0"

// docsRisorInfo provides basic Risor information.
type docsRisorInfo struct {
	Version        string `json:"version"`
	Description    string `json:"description"`
	ExecutionModel string `json:"execution_model"`
}

// docsSyntaxPattern describes a syntax pattern.
type docsSyntaxPattern struct {
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
}

// docsQuickReference is the quick reference structure.
type docsQuickReference struct {
	Risor         docsRisorInfo       `json:"risor"`
	SyntaxQuickRef []docsSyntaxPattern `json:"syntax_quick_ref"`
	Topics        map[string]string    `json:"topics"`
	Next          []string             `json:"next"`
}

// docsTypeInfo summarizes a type.
type docsTypeInfo struct {
	Name        string   `json:"name"`
	Doc         string   `json:"doc"`
	MethodCount int      `json:"method_count,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

// docsModuleInfo summarizes a module.
type docsModuleInfo struct {
	Name      string   `json:"name"`
	Doc       string   `json:"doc"`
	FuncCount int      `json:"function_count"`
	Functions []string `json:"functions,omitempty"`
}

// docsSyntaxSection groups related syntax items.
type docsSyntaxSection struct {
	Name  string           `json:"name"`
	Items []docsSyntaxItem `json:"items"`
}

// docsSyntaxItem describes a single syntax construct.
type docsSyntaxItem struct {
	Syntax string `json:"syntax"`
	Type   string `json:"type,omitempty"`
	Notes  string `json:"notes"`
}

// docsErrorPattern describes a common error pattern.
type docsErrorPattern struct {
	Type           string              `json:"type"`
	MessagePattern string              `json:"message_pattern"`
	Causes         []string            `json:"causes"`
	Examples       []docsErrorExample  `json:"examples"`
}

// docsErrorExample shows a specific error case.
type docsErrorExample struct {
	Error       string `json:"error"`
	BadCode     string `json:"bad_code"`
	Fix         string `json:"fix"`
	Explanation string `json:"explanation"`
}

// docsFullDocumentation contains all documentation.
type docsFullDocumentation struct {
	Risor    docsRisorInfo                `json:"risor"`
	Builtins []object.FuncSpec            `json:"builtins"`
	Modules  map[string]docsModuleInfo    `json:"modules"`
	Types    map[string]docsTypeInfo      `json:"types"`
	Syntax   []docsSyntaxSection          `json:"syntax"`
	Errors   []docsErrorPattern           `json:"errors"`
}

// Docs returns structured documentation about Risor.
// Useful for tooling, editor integrations, and LLM agents.
//
// Example:
//
//	// Quick reference
//	docs := risor.Docs(risor.DocsQuick())
//	fmt.Println(docs.JSON())
//
//	// Full documentation
//	docs := risor.Docs(risor.DocsAll())
//
//	// Specific category
//	docs := risor.Docs(risor.DocsCategory("builtins"))
func Docs(opts ...DocsOption) *Documentation {
	o := &docsOptions{}
	for _, opt := range opts {
		opt(o)
	}

	if o.quick {
		return &Documentation{data: buildQuickReference()}
	}

	if o.all {
		return &Documentation{data: buildFullDocumentation()}
	}

	if o.category != "" {
		return &Documentation{data: buildCategoryDocs(o.category)}
	}

	if o.topic != "" {
		return &Documentation{data: buildTopicDocs(o.topic)}
	}

	// Default: return quick reference
	return &Documentation{data: buildQuickReference()}
}

func buildQuickReference() docsQuickReference {
	return docsQuickReference{
		Risor: docsRisorInfo{
			Version:        Version,
			Description:    "Fast embedded scripting language for Go",
			ExecutionModel: "source → lexer → parser → compiler → bytecode → vm",
		},
		SyntaxQuickRef: docsSyntaxQuickRef,
		Topics: map[string]string{
			"builtins": "Built-in functions (len, map, filter, range, ...)",
			"types":    "Types (string, list, map, int, float, ...)",
			"modules":  "Modules (math, rand, regexp, time)",
			"syntax":   "Complete syntax reference",
			"errors":   "Common errors and debugging",
		},
		Next: []string{
			"risor.Docs(risor.DocsCategory(\"builtins\"))",
			"risor.Docs(risor.DocsCategory(\"syntax\"))",
			"risor.Docs(risor.DocsAll())",
		},
	}
}

func buildFullDocumentation() docsFullDocumentation {
	full := docsFullDocumentation{
		Risor: docsRisorInfo{
			Version:        Version,
			Description:    "Fast embedded scripting language for Go",
			ExecutionModel: "source → lexer → parser → compiler → bytecode → vm",
		},
		Builtins: builtins.Docs(),
		Modules:  make(map[string]docsModuleInfo),
		Types:    make(map[string]docsTypeInfo),
		Syntax:   docsSyntaxSections,
		Errors:   docsErrorPatterns,
	}

	for name, mod := range docsModuleDocs {
		funcNames := make([]string, len(mod.Funcs))
		for i, fn := range mod.Funcs {
			funcNames[i] = fn.Name
		}
		full.Modules[name] = docsModuleInfo{
			Name:      name,
			Doc:       mod.Doc,
			FuncCount: len(mod.Funcs),
			Functions: funcNames,
		}
	}

	for name, t := range docsTypeDocs {
		methods := make([]string, len(t.Attrs))
		for i, attr := range t.Attrs {
			methods[i] = attr.Name
		}
		full.Types[name] = docsTypeInfo{
			Name:        name,
			Doc:         t.Doc,
			MethodCount: len(t.Attrs),
			Methods:     methods,
		}
	}

	return full
}

func buildCategoryDocs(category string) any {
	switch category {
	case "builtins":
		return map[string]any{
			"category":    "builtins",
			"description": "Built-in functions available in the global scope",
			"count":       len(builtins.Docs()),
			"functions":   builtins.Docs(),
		}
	case "types":
		types := make([]docsTypeInfo, 0, len(docsTypeDocs))
		var typeNames []string
		for name := range docsTypeDocs {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)
		for _, name := range typeNames {
			t := docsTypeDocs[name]
			types = append(types, docsTypeInfo{
				Name:        name,
				Doc:         t.Doc,
				MethodCount: len(t.Attrs),
			})
		}
		return map[string]any{
			"category":    "types",
			"description": "Risor types and their methods",
			"count":       len(docsTypeDocs),
			"types":       types,
		}
	case "modules":
		modules := make([]docsModuleInfo, 0, len(docsModuleDocs))
		var moduleNames []string
		for name := range docsModuleDocs {
			moduleNames = append(moduleNames, name)
		}
		sort.Strings(moduleNames)
		for _, name := range moduleNames {
			mod := docsModuleDocs[name]
			modules = append(modules, docsModuleInfo{
				Name:      name,
				Doc:       mod.Doc,
				FuncCount: len(mod.Funcs),
			})
		}
		return map[string]any{
			"category":    "modules",
			"description": "Available modules",
			"count":       len(docsModuleDocs),
			"modules":     modules,
		}
	case "syntax":
		return map[string]any{
			"category":    "syntax",
			"description": "Complete syntax reference",
			"sections":    docsSyntaxSections,
		}
	case "errors":
		return map[string]any{
			"category":    "errors",
			"description": "Common error patterns and fixes",
			"patterns":    docsErrorPatterns,
		}
	default:
		return map[string]any{
			"error": "unknown category: " + category,
		}
	}
}

func buildTopicDocs(topic string) any {
	// Check types
	if t, ok := docsTypeDocs[topic]; ok {
		return map[string]any{
			"type":    "type",
			"name":    t.Name,
			"doc":     t.Doc,
			"methods": t.Attrs,
		}
	}

	// Check modules
	if mod, ok := docsModuleDocs[topic]; ok {
		return map[string]any{
			"type":      "module",
			"name":      topic,
			"doc":       mod.Doc,
			"functions": mod.Funcs,
		}
	}

	// Check builtins
	for _, fn := range builtins.Docs() {
		if fn.Name == topic {
			return map[string]any{
				"type":     "builtin",
				"function": fn,
			}
		}
	}

	return map[string]any{
		"error": "unknown topic: " + topic,
	}
}

// Type documentation (internal)
var docsTypeDocs = map[string]object.TypeSpec{
	"string": {
		Name:  "string",
		Doc:   "Immutable sequence of Unicode characters",
		Attrs: docsGetStringAttrs(),
	},
	"list": {
		Name:  "list",
		Doc:   "Mutable ordered collection of values",
		Attrs: docsGetListAttrs(),
	},
	"map": {
		Name: "map",
		Doc:  "Mutable key-value mapping with string keys",
	},
	"int": {
		Name: "int",
		Doc:  "64-bit signed integer",
	},
	"float": {
		Name: "float",
		Doc:  "64-bit floating point number",
	},
	"bool": {
		Name: "bool",
		Doc:  "Boolean value (true or false)",
	},
	"bytes": {
		Name:  "bytes",
		Doc:   "Mutable sequence of bytes",
		Attrs: docsGetBytesAttrs(),
	},
	"time": {
		Name:  "time",
		Doc:   "Point in time with nanosecond precision",
		Attrs: docsGetTimeAttrs(),
	},
	"nil": {
		Name: "nil",
		Doc:  "Absence of a value",
	},
	"error": {
		Name: "error",
		Doc:  "Error value that can be thrown or returned",
	},
	"function": {
		Name: "function",
		Doc:  "User-defined function or closure",
	},
	"builtin": {
		Name: "builtin",
		Doc:  "Built-in function implemented in Go",
	},
	"module": {
		Name: "module",
		Doc:  "Collection of related functions and values",
	},
	"range": {
		Name: "range",
		Doc:  "Lazy sequence of integers",
	},
}

// Module documentation (internal)
var docsModuleDocs = map[string]struct {
	Doc   string
	Funcs []object.FuncSpec
}{
	"math": {
		Doc: "Mathematical functions and constants",
		Funcs: []object.FuncSpec{
			{Name: "abs", Doc: "Absolute value", Args: []string{"x"}, Returns: "int|float"},
			{Name: "ceil", Doc: "Ceiling (round up)", Args: []string{"x"}, Returns: "float"},
			{Name: "cos", Doc: "Cosine", Args: []string{"x"}, Returns: "float"},
			{Name: "floor", Doc: "Floor (round down)", Args: []string{"x"}, Returns: "float"},
			{Name: "log", Doc: "Natural logarithm", Args: []string{"x"}, Returns: "float"},
			{Name: "max", Doc: "Maximum of two values", Args: []string{"x", "y"}, Returns: "float"},
			{Name: "min", Doc: "Minimum of two values", Args: []string{"x", "y"}, Returns: "float"},
			{Name: "pow", Doc: "Power (x^y)", Args: []string{"x", "y"}, Returns: "float"},
			{Name: "round", Doc: "Round to nearest integer", Args: []string{"x"}, Returns: "float"},
			{Name: "sin", Doc: "Sine", Args: []string{"x"}, Returns: "float"},
			{Name: "sqrt", Doc: "Square root", Args: []string{"x"}, Returns: "float"},
			{Name: "tan", Doc: "Tangent", Args: []string{"x"}, Returns: "float"},
			{Name: "PI", Doc: "Pi (3.14159...)", Args: nil, Returns: "float"},
			{Name: "E", Doc: "Euler's number (2.718...)", Args: nil, Returns: "float"},
		},
	},
	"rand": {
		Doc: "Random number generation",
		Funcs: []object.FuncSpec{
			{Name: "float", Doc: "Random float in [0.0, 1.0)", Args: nil, Returns: "float"},
			{Name: "int", Doc: "Random int in [0, n)", Args: []string{"n"}, Returns: "int"},
			{Name: "seed", Doc: "Seed the random generator", Args: []string{"seed"}, Returns: "nil"},
			{Name: "shuffle", Doc: "Shuffle list in place", Args: []string{"list"}, Returns: "list"},
		},
	},
	"regexp": {
		Doc: "Regular expression matching",
		Funcs: []object.FuncSpec{
			{Name: "compile", Doc: "Compile a regular expression", Args: []string{"pattern"}, Returns: "regexp"},
			{Name: "match", Doc: "Check if pattern matches string", Args: []string{"pattern", "s"}, Returns: "bool"},
			{Name: "find", Doc: "Find first match", Args: []string{"pattern", "s"}, Returns: "string"},
			{Name: "find_all", Doc: "Find all matches", Args: []string{"pattern", "s"}, Returns: "list"},
			{Name: "replace_all", Doc: "Replace all matches", Args: []string{"pattern", "s", "repl"}, Returns: "string"},
			{Name: "split", Doc: "Split by pattern", Args: []string{"pattern", "s"}, Returns: "list"},
		},
	},
	"time": {
		Doc: "Time and date operations",
		Funcs: []object.FuncSpec{
			{Name: "now", Doc: "Current time", Args: nil, Returns: "time"},
			{Name: "parse", Doc: "Parse time string", Args: []string{"layout", "value"}, Returns: "time"},
			{Name: "since", Doc: "Duration since time", Args: []string{"t"}, Returns: "float"},
			{Name: "sleep", Doc: "Sleep for duration (seconds)", Args: []string{"seconds"}, Returns: "nil"},
			{Name: "unix", Doc: "Create time from Unix timestamp", Args: []string{"sec", "nsec?"}, Returns: "time"},
		},
	},
}

// Syntax quick reference
var docsSyntaxQuickRef = []docsSyntaxPattern{
	{Pattern: "let x = 1", Description: "Variable declaration"},
	{Pattern: "const PI = 3.14", Description: "Constant declaration"},
	{Pattern: "x => x * 2", Description: "Arrow function"},
	{Pattern: "function name(a, b) { }", Description: "Named function"},
	{Pattern: "[1, 2, 3]", Description: "List literal"},
	{Pattern: "{key: value}", Description: "Map literal"},
	{Pattern: "obj.method()", Description: "Method call"},
	{Pattern: "list.map(fn).filter(fn)", Description: "Method chaining"},
	{Pattern: "let {a, b} = obj", Description: "Destructuring"},
	{Pattern: "{...a, ...b}", Description: "Spread operator"},
}

// Syntax sections
var docsSyntaxSections = []docsSyntaxSection{
	{
		Name: "literals",
		Items: []docsSyntaxItem{
			{Syntax: "42", Type: "int", Notes: "Integer literal"},
			{Syntax: "3.14", Type: "float", Notes: "Float literal"},
			{Syntax: `"hello"`, Type: "string", Notes: "String literal with escapes"},
			{Syntax: "`raw`", Type: "string", Notes: "Raw string (no escapes)"},
			{Syntax: "true, false", Type: "bool", Notes: "Boolean literals"},
			{Syntax: "nil", Type: "nil", Notes: "Null value"},
		},
	},
	{
		Name: "collections",
		Items: []docsSyntaxItem{
			{Syntax: "[a, b, c]", Type: "list", Notes: "List literal"},
			{Syntax: "{k: v}", Type: "map", Notes: "Map literal"},
			{Syntax: "{k}", Type: "map", Notes: "Shorthand for {k: k}"},
			{Syntax: "[...a, ...b]", Type: "list", Notes: "List spread"},
			{Syntax: "{...a, ...b}", Type: "map", Notes: "Map spread"},
		},
	},
	{
		Name: "variables",
		Items: []docsSyntaxItem{
			{Syntax: "let x = value", Notes: "Mutable variable"},
			{Syntax: "const X = value", Notes: "Immutable constant"},
			{Syntax: "let {a, b} = obj", Notes: "Destructuring assignment"},
			{Syntax: "let [x, y] = list", Notes: "List destructuring"},
		},
	},
	{
		Name: "functions",
		Items: []docsSyntaxItem{
			{Syntax: "function name(a, b) { body }", Notes: "Named function"},
			{Syntax: "let f = function(a) { body }", Notes: "Anonymous function"},
			{Syntax: "x => expr", Notes: "Arrow function (single param)"},
			{Syntax: "(a, b) => expr", Notes: "Arrow function (multiple params)"},
			{Syntax: "(a, b) => { stmts }", Notes: "Arrow function with block"},
		},
	},
	{
		Name: "control_flow",
		Items: []docsSyntaxItem{
			{Syntax: "if (cond) { } else { }", Notes: "Conditional (is an expression)"},
			{Syntax: "switch (val) { case x: ... }", Notes: "Switch statement"},
			{Syntax: "try { } catch e { }", Notes: "Error handling"},
			{Syntax: "throw error(msg)", Notes: "Raise an error"},
			{Syntax: "return value", Notes: "Return from function"},
		},
	},
	{
		Name: "operators",
		Items: []docsSyntaxItem{
			{Syntax: "+ - * / %", Notes: "Arithmetic"},
			{Syntax: "== != < > <= >=", Notes: "Comparison"},
			{Syntax: "&& || !", Notes: "Logical"},
			{Syntax: "?? ?.", Notes: "Nil coalescing, optional chain"},
			{Syntax: "|", Notes: "Pipe operator"},
		},
	},
	{
		Name: "method_chaining",
		Items: []docsSyntaxItem{
			{Syntax: "obj.method()", Notes: "Method call"},
			{Syntax: "list.map(f).filter(g)", Notes: "Chained methods"},
			{Syntax: "obj?.method()", Notes: "Optional chaining"},
		},
	},
}

// Error patterns
var docsErrorPatterns = []docsErrorPattern{
	{
		Type:           "type_error",
		MessagePattern: "type error: expected X, got Y",
		Causes: []string{
			"Passing wrong type to function",
			"Method called on incompatible type",
		},
		Examples: []docsErrorExample{
			{
				Error:       "type error: expected string, got int",
				BadCode:     `"hello".split(123)`,
				Fix:         `"hello".split(",")`,
				Explanation: "split() requires a string separator",
			},
		},
	},
	{
		Type:           "name_error",
		MessagePattern: "name error: X is not defined",
		Causes: []string{
			"Typo in variable or function name",
			"Variable used before declaration",
		},
		Examples: []docsErrorExample{
			{
				Error:       "name error: 'maths' is not defined",
				BadCode:     `maths.sqrt(4)`,
				Fix:         `math.sqrt(4)`,
				Explanation: "Module name is 'math', not 'maths'",
			},
		},
	},
	{
		Type:           "syntax_error",
		MessagePattern: "syntax error at line N",
		Causes: []string{
			"Missing closing bracket/brace",
			"Invalid token",
		},
		Examples: []docsErrorExample{
			{
				Error:       "syntax error: expected '}'",
				BadCode:     `if x { print(x)`,
				Fix:         `if x { print(x) }`,
				Explanation: "Block must be closed with '}'",
			},
		},
	},
}

// Helper functions to get attrs from types
func docsGetStringAttrs() []object.AttrSpec {
	s := object.NewString("")
	return s.Attrs()
}

func docsGetListAttrs() []object.AttrSpec {
	ls := object.NewList(nil)
	return ls.Attrs()
}

func docsGetBytesAttrs() []object.AttrSpec {
	b := object.NewBytes(nil)
	return b.Attrs()
}

func docsGetTimeAttrs() []object.AttrSpec {
	t := object.NewTime(stdtime.Now())
	return t.Attrs()
}
