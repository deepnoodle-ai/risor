package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	stdtime "time"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/tui"
	"github.com/risor-io/risor/builtins"
	"github.com/risor-io/risor/modules/math"
	"github.com/risor-io/risor/modules/rand"
	"github.com/risor-io/risor/modules/regexp"
	"github.com/risor-io/risor/modules/time"
	"github.com/risor-io/risor/object"
)

// Type documentation
var typeDocs = map[string]object.TypeSpec{
	"string": {
		Name:  "string",
		Doc:   "Immutable sequence of Unicode characters",
		Attrs: getStringAttrs(),
	},
	"list": {
		Name:  "list",
		Doc:   "Mutable ordered collection of values",
		Attrs: getListAttrs(),
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
		Attrs: getBytesAttrs(),
	},
	"time": {
		Name:  "time",
		Doc:   "Point in time with nanosecond precision",
		Attrs: getTimeAttrs(),
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

// Module documentation
var moduleDocs = map[string]struct {
	Doc   string
	Funcs []object.FuncSpec
}{
	"math": {
		Doc: "Mathematical functions and constants",
		Funcs: []object.FuncSpec{
			{Name: "abs", Doc: "Absolute value", Args: []string{"x"}, Returns: "int|float"},
			{Name: "atan2", Doc: "Arctangent of y/x", Args: []string{"y", "x"}, Returns: "float"},
			{Name: "ceil", Doc: "Ceiling (round up)", Args: []string{"x"}, Returns: "float"},
			{Name: "cos", Doc: "Cosine", Args: []string{"x"}, Returns: "float"},
			{Name: "floor", Doc: "Floor (round down)", Args: []string{"x"}, Returns: "float"},
			{Name: "inf", Doc: "Return infinity", Args: []string{"sign?"}, Returns: "float"},
			{Name: "is_inf", Doc: "Check if value is infinity", Args: []string{"x"}, Returns: "bool"},
			{Name: "log", Doc: "Natural logarithm", Args: []string{"x"}, Returns: "float"},
			{Name: "log10", Doc: "Base-10 logarithm", Args: []string{"x"}, Returns: "float"},
			{Name: "log2", Doc: "Base-2 logarithm", Args: []string{"x"}, Returns: "float"},
			{Name: "max", Doc: "Maximum of two values", Args: []string{"x", "y"}, Returns: "float"},
			{Name: "min", Doc: "Minimum of two values", Args: []string{"x", "y"}, Returns: "float"},
			{Name: "mod", Doc: "Modulo", Args: []string{"x", "y"}, Returns: "float"},
			{Name: "pow", Doc: "Power (x^y)", Args: []string{"x", "y"}, Returns: "float"},
			{Name: "pow10", Doc: "Power of 10", Args: []string{"n"}, Returns: "float"},
			{Name: "round", Doc: "Round to nearest integer", Args: []string{"x"}, Returns: "float"},
			{Name: "sin", Doc: "Sine", Args: []string{"x"}, Returns: "float"},
			{Name: "sqrt", Doc: "Square root", Args: []string{"x"}, Returns: "float"},
			{Name: "sum", Doc: "Sum of list elements", Args: []string{"items"}, Returns: "float"},
			{Name: "tan", Doc: "Tangent", Args: []string{"x"}, Returns: "float"},
			{Name: "E", Doc: "Euler's number (2.718...)", Args: nil, Returns: "float"},
			{Name: "PI", Doc: "Pi (3.14159...)", Args: nil, Returns: "float"},
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

func docHandler(ctx *cli.Context) error {
	topic := ctx.Arg(0)
	outputFormat := ctx.String("output")

	if outputFormat == "json" {
		return docHandlerJSON(topic)
	}

	// Styles
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	headingStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})
	mutedStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 120, G: 120, B: 130})

	if topic == "" {
		// Show overview
		tui.Print(tui.Stack(
			tui.Text("Risor Language Reference").Style(titleStyle),
			tui.Text(""),
			tui.Text("BUILTINS").Style(headingStyle),
		).Gap(0))

		docs := builtins.Docs()
		for _, fn := range docs {
			sig := formatSignature(fn.Name, fn.Args)
			tui.Print(tui.Group(
				tui.Text("  %s", sig).Style(nameStyle),
				tui.Text("  %s", fn.Doc).Style(docStyle),
			))
		}

		tui.Print(tui.Text(""))
		tui.Print(tui.Text("MODULES").Style(headingStyle))

		var moduleNames []string
		for name := range moduleDocs {
			moduleNames = append(moduleNames, name)
		}
		sort.Strings(moduleNames)

		for _, name := range moduleNames {
			mod := moduleDocs[name]
			tui.Print(tui.Group(
				tui.Text("  %s", name).Style(nameStyle),
				tui.Text("  %s", mod.Doc).Style(docStyle),
			))
		}

		tui.Print(tui.Text(""))
		tui.Print(tui.Text("TYPES").Style(headingStyle))

		var typeNames []string
		for name := range typeDocs {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)

		for _, name := range typeNames {
			t := typeDocs[name]
			tui.Print(tui.Group(
				tui.Text("  %s", name).Style(nameStyle),
				tui.Text("  %s", t.Doc).Style(docStyle),
			))
		}

		tui.Print(tui.Text(""))
		tui.Print(tui.Text("Use 'risor doc <topic>' for details").Style(mutedStyle))

		return nil
	}

	// Check if it's a module.function reference
	if strings.Contains(topic, ".") {
		parts := strings.SplitN(topic, ".", 2)
		moduleName, funcName := parts[0], parts[1]

		if mod, ok := moduleDocs[moduleName]; ok {
			for _, fn := range mod.Funcs {
				if fn.Name == funcName {
					printFuncDoc(moduleName+"."+fn.Name, fn, titleStyle, headingStyle, nameStyle, docStyle)
					return nil
				}
			}
			return fmt.Errorf("function %q not found in module %q", funcName, moduleName)
		}
	}

	// Check types first (so "string" shows the type, not the builtin)
	if t, ok := typeDocs[topic]; ok {
		tui.Print(tui.Stack(
			tui.Text("Type: %s", t.Name).Style(titleStyle),
			tui.Text("%s", t.Doc).Style(docStyle),
		).Gap(0))

		if len(t.Attrs) > 0 {
			tui.Print(tui.Text(""))
			tui.Print(tui.Text("METHODS").Style(headingStyle))
			for _, attr := range t.Attrs {
				sig := formatSignature(attr.Name, attr.Args)
				tui.Print(tui.Group(
					tui.Text("  .%s", sig).Style(nameStyle),
					tui.Text("  %s", attr.Doc).Style(docStyle),
				))
			}
		}
		return nil
	}

	// Check if it's a module
	if mod, ok := moduleDocs[topic]; ok {
		tui.Print(tui.Stack(
			tui.Text("Module: %s", topic).Style(titleStyle),
			tui.Text("%s", mod.Doc).Style(docStyle),
			tui.Text(""),
			tui.Text("FUNCTIONS").Style(headingStyle),
		).Gap(0))

		for _, fn := range mod.Funcs {
			sig := formatSignature(fn.Name, fn.Args)
			tui.Print(tui.Group(
				tui.Text("  %s", sig).Style(nameStyle),
				tui.Text("  %s", fn.Doc).Style(docStyle),
			))
		}
		return nil
	}

	// Check if it's a builtin
	for _, fn := range builtins.Docs() {
		if fn.Name == topic {
			printFuncDoc(fn.Name, fn, titleStyle, headingStyle, nameStyle, docStyle)
			return nil
		}
	}

	return fmt.Errorf("unknown topic: %q", topic)
}

func printFuncDoc(name string, fn object.FuncSpec, titleStyle, headingStyle, nameStyle, docStyle tui.Style) {
	sig := formatSignature(name, fn.Args)

	tui.Print(tui.Stack(
		tui.Text("%s", sig).Style(titleStyle),
		tui.Text(""),
		tui.Text("%s", fn.Doc).Style(docStyle),
	).Gap(0))

	if fn.Returns != "" {
		tui.Print(tui.Text(""))
		tui.Print(tui.Group(
			tui.Text("Returns: ").Style(headingStyle),
			tui.Text("%s", fn.Returns).Style(nameStyle),
		))
	}

	if fn.Example != "" {
		tui.Print(tui.Text(""))
		tui.Print(tui.Text("EXAMPLE").Style(headingStyle))
		tui.Print(tui.Text("  %s", fn.Example).Style(nameStyle))
	}
}

func formatSignature(name string, args []string) string {
	if len(args) == 0 {
		return name + "()"
	}
	return name + "(" + strings.Join(args, ", ") + ")"
}

// docHandlerJSON outputs documentation in JSON format
func docHandlerJSON(topic string) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if topic == "" {
		// Overview - output all docs
		result := struct {
			Builtins []object.FuncSpec          `json:"builtins"`
			Modules  map[string]json.RawMessage `json:"modules"`
			Types    map[string]json.RawMessage `json:"types"`
		}{
			Builtins: builtins.Docs(),
			Modules:  make(map[string]json.RawMessage),
			Types:    make(map[string]json.RawMessage),
		}

		for name, mod := range moduleDocs {
			modJSON, _ := json.Marshal(struct {
				Doc   string            `json:"doc"`
				Funcs []object.FuncSpec `json:"functions"`
			}{mod.Doc, mod.Funcs})
			result.Modules[name] = modJSON
		}

		for name, t := range typeDocs {
			typeJSON, _ := json.Marshal(struct {
				Name    string            `json:"name"`
				Doc     string            `json:"doc"`
				Methods []object.AttrSpec `json:"methods,omitempty"`
			}{t.Name, t.Doc, t.Attrs})
			result.Types[name] = typeJSON
		}

		return enc.Encode(result)
	}

	// Check module.function reference
	if strings.Contains(topic, ".") {
		parts := strings.SplitN(topic, ".", 2)
		moduleName, funcName := parts[0], parts[1]

		if mod, ok := moduleDocs[moduleName]; ok {
			for _, fn := range mod.Funcs {
				if fn.Name == funcName {
					return enc.Encode(struct {
						Type     string          `json:"type"`
						Module   string          `json:"module"`
						Function object.FuncSpec `json:"function"`
					}{"module_function", moduleName, fn})
				}
			}
			return fmt.Errorf("function %q not found in module %q", funcName, moduleName)
		}
	}

	// Check types
	if t, ok := typeDocs[topic]; ok {
		return enc.Encode(struct {
			Type    string            `json:"type"`
			Name    string            `json:"name"`
			Doc     string            `json:"doc"`
			Methods []object.AttrSpec `json:"methods,omitempty"`
		}{"type", t.Name, t.Doc, t.Attrs})
	}

	// Check modules
	if mod, ok := moduleDocs[topic]; ok {
		return enc.Encode(struct {
			Type  string            `json:"type"`
			Name  string            `json:"name"`
			Doc   string            `json:"doc"`
			Funcs []object.FuncSpec `json:"functions"`
		}{"module", topic, mod.Doc, mod.Funcs})
	}

	// Check builtins
	for _, fn := range builtins.Docs() {
		if fn.Name == topic {
			return enc.Encode(struct {
				Type     string          `json:"type"`
				Function object.FuncSpec `json:"function"`
			}{"builtin", fn})
		}
	}

	return fmt.Errorf("unknown topic: %q", topic)
}

// Helper functions to get attrs from types
// These create temporary instances just to access the Attrs() method

func getStringAttrs() []object.AttrSpec {
	s := object.NewString("")
	return s.Attrs()
}

func getListAttrs() []object.AttrSpec {
	ls := object.NewList(nil)
	return ls.Attrs()
}

func getBytesAttrs() []object.AttrSpec {
	b := object.NewBytes(nil)
	return b.Attrs()
}

func getTimeAttrs() []object.AttrSpec {
	t := object.NewTime(stdtime.Now())
	return t.Attrs()
}

// Suppress unused import warnings - these are used to ensure modules are linked
var (
	_ = math.Module
	_ = rand.Module
	_ = regexp.Module
	_ = time.Module
)
