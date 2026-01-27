package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/tui"
	"github.com/risor-io/risor/builtins"
	"github.com/risor-io/risor/modules/math"
	"github.com/risor-io/risor/modules/rand"
	"github.com/risor-io/risor/modules/regexp"
	"github.com/risor-io/risor/object"
)

// typeDocs returns type documentation from the dynamic registry.
func typeDocs() map[string]object.TypeSpec {
	return object.TypeDocsMap()
}

// Module documentation - generated from modules themselves
var moduleDocs = map[string]struct {
	Doc   string
	Funcs []object.FuncSpec
}{
	"math":   {Doc: math.ModuleDoc(), Funcs: math.Docs()},
	"rand":   {Doc: rand.ModuleDoc(), Funcs: rand.Docs()},
	"regexp": {Doc: regexp.ModuleDoc(), Funcs: regexp.Docs()},
}

func docHandler(ctx *cli.Context) error {
	topic := ctx.Arg(0)
	format := ctx.String("format")
	quick := ctx.Bool("quick")
	all := ctx.Bool("all")

	// Handle --quick mode
	if quick {
		return docQuickReference(format)
	}

	// Handle --all mode
	if all {
		return docAll(format, topic)
	}

	// Route based on format
	switch format {
	case "json":
		return docHandlerJSON(topic)
	case "markdown":
		return docHandlerMarkdown(topic)
	default:
		return docHandlerText(topic)
	}
}

// docQuickReference outputs a concise overview for quick orientation.
func docQuickReference(format string) error {
	ref := QuickReference{
		Risor: RisorInfo{
			Version:        docVersion,
			Description:    "Fast embedded scripting language for Go",
			ExecutionModel: "source → lexer → parser → compiler → bytecode → vm",
		},
		SyntaxQuickRef: syntaxQuickRef,
		Topics: map[string]string{
			"builtins": fmt.Sprintf("%d built-in functions (len, map, filter, range, ...)", len(builtins.Docs())),
			"types":    fmt.Sprintf("%d types (string, list, map, int, float, ...)", len(typeDocs())),
			"modules":  fmt.Sprintf("%d modules (math, rand, regexp)", len(moduleDocs)),
			"syntax":   "Complete syntax reference",
			"errors":   "Common errors and debugging",
		},
		Next: []string{
			"risor doc builtins",
			"risor doc syntax",
			"risor doc types",
		},
	}

	switch format {
	case "json":
		return jsonEncode(ref)
	case "markdown":
		return docQuickMarkdown(ref)
	default:
		return docQuickText(ref)
	}
}

func docQuickText(ref QuickReference) error {
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	headingStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})
	mutedStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 120, G: 120, B: 130})

	topics := []string{"builtins", "types", "modules", "syntax", "errors"}

	tui.Print(tui.Stack(
		tui.Text("Risor v%s", ref.Risor.Version).Style(titleStyle),
		tui.Text("%s", ref.Risor.Description).Style(docStyle),
		tui.Text(""),
		tui.Text("Execution: %s", ref.Risor.ExecutionModel).Style(mutedStyle),
		tui.Text(""),
		tui.Text("SYNTAX QUICK REFERENCE").Style(headingStyle),
		tui.ForEach(ref.SyntaxQuickRef, func(pat SyntaxPattern, _ int) tui.View {
			return tui.Group(
				tui.Text("  %s", pat.Pattern).Style(nameStyle),
				tui.Text("  %s", pat.Description).Style(docStyle),
			)
		}),
		tui.Text(""),
		tui.Text("TOPICS").Style(headingStyle),
		tui.ForEach(topics, func(topic string, _ int) tui.View {
			return tui.Group(
				tui.Text("  %s", topic).Style(nameStyle),
				tui.Text("  %s", ref.Topics[topic]).Style(docStyle),
			)
		}),
		tui.Text(""),
		tui.Text("Next: %s", strings.Join(ref.Next, ", ")).Style(mutedStyle),
	).Gap(0))
	fmt.Println() // Final newline

	return nil
}

func docQuickMarkdown(ref QuickReference) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Risor v%s\n\n", ref.Risor.Version))
	sb.WriteString(fmt.Sprintf("%s\n\n", ref.Risor.Description))
	sb.WriteString(fmt.Sprintf("**Execution:** `%s`\n\n", ref.Risor.ExecutionModel))
	sb.WriteString("## Syntax Quick Reference\n\n")
	sb.WriteString("| Pattern | Description |\n")
	sb.WriteString("|---------|-------------|\n")
	for _, pat := range ref.SyntaxQuickRef {
		sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", pat.Pattern, pat.Description))
	}
	sb.WriteString("\n## Topics\n\n")
	for _, topic := range []string{"builtins", "types", "modules", "syntax", "errors"} {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", topic, ref.Topics[topic]))
	}
	sb.WriteString("\n## Next Steps\n\n")
	for _, cmd := range ref.Next {
		sb.WriteString(fmt.Sprintf("- `%s`\n", cmd))
	}
	fmt.Print(sb.String())
	return nil
}

// docAll outputs complete documentation for a category or everything.
func docAll(format, topic string) error {
	switch format {
	case "json":
		return docAllJSON(topic)
	case "markdown":
		return docAllMarkdown(topic)
	default:
		return docAllText(topic)
	}
}

func docAllJSON(topic string) error {
	switch topic {
	case "", "all":
		// Return everything
		full := FullDocumentation{
			Risor: RisorInfo{
				Version:        docVersion,
				Description:    "Fast embedded scripting language for Go",
				ExecutionModel: "source → lexer → parser → compiler → bytecode → vm",
			},
			Builtins: builtins.Docs(),
			Modules:  make(map[string]ModuleInfo),
			Types:    make(map[string]TypeDetail),
			Syntax:   syntaxSections,
			Errors:   errorPatterns,
		}
		for name, mod := range moduleDocs {
			funcNames := make([]string, len(mod.Funcs))
			for i, fn := range mod.Funcs {
				funcNames[i] = fn.Name
			}
			full.Modules[name] = ModuleInfo{
				Name:      name,
				Doc:       mod.Doc,
				FuncCount: len(mod.Funcs),
				Functions: funcNames,
			}
		}
		for name, t := range typeDocs() {
			full.Types[name] = TypeDetail{
				Type:    "type",
				Name:    t.Name,
				Doc:     t.Doc,
				Methods: t.Attrs,
			}
		}
		return jsonEncode(full)

	case "builtins":
		return jsonEncode(BuiltinsOverview{
			CategoryOverview: CategoryOverview{
				Category:    "builtins",
				Description: "Built-in functions available in the global scope",
				Count:       len(builtins.Docs()),
			},
			Functions: builtins.Docs(),
		})

	case "types":
		types := make([]TypeInfo, 0, len(typeDocs()))
		for name, t := range typeDocs() {
			methods := make([]string, len(t.Attrs))
			for i, attr := range t.Attrs {
				methods[i] = attr.Name
			}
			types = append(types, TypeInfo{
				Name:        name,
				Doc:         t.Doc,
				MethodCount: len(t.Attrs),
				Methods:     methods,
			})
		}
		return jsonEncode(TypesOverview{
			CategoryOverview: CategoryOverview{
				Category:    "types",
				Description: "Risor types and their methods",
				Count:       len(typeDocs()),
			},
			Types: types,
		})

	case "modules":
		modules := make([]ModuleInfo, 0, len(moduleDocs))
		for name, mod := range moduleDocs {
			funcNames := make([]string, len(mod.Funcs))
			for i, fn := range mod.Funcs {
				funcNames[i] = fn.Name
			}
			modules = append(modules, ModuleInfo{
				Name:      name,
				Doc:       mod.Doc,
				FuncCount: len(mod.Funcs),
				Functions: funcNames,
			})
		}
		return jsonEncode(ModulesOverview{
			CategoryOverview: CategoryOverview{
				Category:    "modules",
				Description: "Available modules",
				Count:       len(moduleDocs),
			},
			Modules: modules,
		})

	case "syntax":
		return jsonEncode(SyntaxOverview{
			CategoryOverview: CategoryOverview{
				Category:    "syntax",
				Description: "Complete syntax reference",
			},
			Sections: syntaxSections,
		})

	case "errors":
		return jsonEncode(ErrorsOverview{
			CategoryOverview: CategoryOverview{
				Category:    "errors",
				Description: "Common error patterns and fixes",
				Count:       len(errorPatterns),
			},
			Patterns: errorPatterns,
		})

	default:
		return fmt.Errorf("unknown category for --all: %q (use builtins, types, modules, syntax, errors)", topic)
	}
}

func docAllText(topic string) error {
	// For text format, just show the regular detailed view
	switch topic {
	case "", "all":
		// Show everything - this could be very long
		if err := docHandlerText(""); err != nil {
			return err
		}
		tui.Print(tui.Text(""))
		if err := docSyntaxText(); err != nil {
			return err
		}
		tui.Print(tui.Text(""))
		return docErrorsText()
	case "syntax":
		return docSyntaxText()
	case "errors":
		return docErrorsText()
	default:
		return docHandlerText(topic)
	}
}

func docAllMarkdown(topic string) error {
	switch topic {
	case "", "all":
		var sb strings.Builder
		sb.WriteString("# Risor Language Reference\n\n")
		sb.WriteString(builtinsMarkdown())
		sb.WriteString("\n")
		sb.WriteString(modulesMarkdown())
		sb.WriteString("\n")
		sb.WriteString(typesMarkdown())
		sb.WriteString("\n")
		sb.WriteString(syntaxMarkdown())
		sb.WriteString("\n")
		sb.WriteString(errorsMarkdown())
		fmt.Print(sb.String())
		return nil
	case "syntax":
		fmt.Print(syntaxMarkdown())
		return nil
	case "errors":
		fmt.Print(errorsMarkdown())
		return nil
	default:
		return docHandlerMarkdown(topic)
	}
}

// docHandlerText outputs documentation in text format (original behavior).
func docHandlerText(topic string) error {
	// Styles
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	headingStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})
	mutedStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 120, G: 120, B: 130})

	if topic == "" {
		// Show overview
		docs := builtins.Docs()

		var moduleNames []string
		for name := range moduleDocs {
			moduleNames = append(moduleNames, name)
		}
		sort.Strings(moduleNames)

		var typeNames []string
		for name := range typeDocs() {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)

		categories := []struct{ name, doc string }{
			{"syntax", "Complete syntax reference"},
			{"errors", "Common errors and debugging"},
		}

		// Calculate max widths for each section
		maxBuiltinWidth := 0
		for _, fn := range docs {
			sig := formatSignature(fn.Name, fn.Args)
			if len(sig) > maxBuiltinWidth {
				maxBuiltinWidth = len(sig)
			}
		}

		maxModuleWidth := 0
		for _, name := range moduleNames {
			if len(name) > maxModuleWidth {
				maxModuleWidth = len(name)
			}
		}

		maxTypeWidth := 0
		for _, name := range typeNames {
			if len(name) > maxTypeWidth {
				maxTypeWidth = len(name)
			}
		}

		maxCategoryWidth := 0
		for _, cat := range categories {
			if len(cat.name) > maxCategoryWidth {
				maxCategoryWidth = len(cat.name)
			}
		}

		// Styles for signature parts
		parenStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 140, G: 140, B: 150})
		argStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 160, B: 220})

		tui.Print(tui.Stack(
			tui.Text("Risor Language Reference").Style(titleStyle),
			tui.Text(""),
			tui.Text("BUILTINS").Style(headingStyle),
			tui.ForEach(docs, func(fn object.FuncSpec, _ int) tui.View {
				sig := formatSignature(fn.Name, fn.Args)
				padding := strings.Repeat(" ", maxBuiltinWidth-len(sig))
				args := strings.Join(fn.Args, ", ")
				return tui.Group(
					tui.Text("  %s", fn.Name).Style(nameStyle),
					tui.Text("(").Style(parenStyle),
					tui.Text("%s", args).Style(argStyle),
					tui.Text(")%s", padding).Style(parenStyle),
					tui.Text("  %s", fn.Doc).Style(docStyle),
				)
			}),
			tui.Text(""),
			tui.Text("MODULES").Style(headingStyle),
			tui.ForEach(moduleNames, func(name string, _ int) tui.View {
				mod := moduleDocs[name]
				padded := fmt.Sprintf("%-*s", maxModuleWidth, name)
				return tui.Group(
					tui.Text("  %s", padded).Style(nameStyle),
					tui.Text("  %s", mod.Doc).Style(docStyle),
				)
			}),
			tui.Text(""),
			tui.Text("TYPES").Style(headingStyle),
			tui.ForEach(typeNames, func(name string, _ int) tui.View {
				t := typeDocs()[name]
				padded := fmt.Sprintf("%-*s", maxTypeWidth, name)
				return tui.Group(
					tui.Text("  %s", padded).Style(nameStyle),
					tui.Text("  %s", t.Doc).Style(docStyle),
				)
			}),
			tui.Text(""),
			tui.Text("CATEGORIES").Style(headingStyle),
			tui.ForEach(categories, func(cat struct{ name, doc string }, _ int) tui.View {
				padded := fmt.Sprintf("%-*s", maxCategoryWidth, cat.name)
				return tui.Group(
					tui.Text("  %s", padded).Style(nameStyle),
					tui.Text("  %s", cat.doc).Style(docStyle),
				)
			}),
			tui.Text(""),
			tui.Text("Use 'risor doc <topic>' for details").Style(mutedStyle),
		).Gap(0))
		fmt.Println() // Final newline

		return nil
	}

	// Handle special categories
	switch topic {
	case "syntax":
		return docSyntaxText()
	case "errors":
		return docErrorsText()
	case "builtins":
		return docBuiltinsText()
	case "types":
		return docTypesText()
	case "modules":
		return docModulesText()
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
	if t, ok := typeDocs()[topic]; ok {
		views := []tui.View{
			tui.Text("Type: %s", t.Name).Style(titleStyle),
			tui.Text("%s", t.Doc).Style(docStyle),
		}
		if len(t.Attrs) > 0 {
			views = append(views,
				tui.Text(""),
				tui.Text("METHODS").Style(headingStyle),
				tui.ForEach(t.Attrs, func(attr object.AttrSpec, _ int) tui.View {
					sig := formatSignature(attr.Name, attr.Args)
					return tui.Group(
						tui.Text("  .%s", sig).Style(nameStyle),
						tui.Text("  %s", attr.Doc).Style(docStyle),
					)
				}),
			)
		}
		tui.Print(tui.Stack(views...).Gap(0))
		fmt.Println()
		return nil
	}

	// Check if it's a module
	if mod, ok := moduleDocs[topic]; ok {
		// Calculate max signature width for alignment
		maxSigWidth := 0
		for _, fn := range mod.Funcs {
			sig := formatSignature(fn.Name, fn.Args)
			if len(sig) > maxSigWidth {
				maxSigWidth = len(sig)
			}
		}

		// Styles for signature parts
		parenStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 140, G: 140, B: 150})
		argStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 160, B: 220})

		tui.Print(tui.Stack(
			tui.Text("Module: %s", topic).Style(titleStyle),
			tui.Text("%s", mod.Doc).Style(docStyle),
			tui.Text(""),
			tui.Text("FUNCTIONS").Style(headingStyle),
			tui.ForEach(mod.Funcs, func(fn object.FuncSpec, _ int) tui.View {
				sig := formatSignature(fn.Name, fn.Args)
				padding := strings.Repeat(" ", maxSigWidth-len(sig))
				args := strings.Join(fn.Args, ", ")
				return tui.Group(
					tui.Text("  %s", fn.Name).Style(nameStyle),
					tui.Text("(").Style(parenStyle),
					tui.Text("%s", args).Style(argStyle),
					tui.Text(")%s", padding).Style(parenStyle),
					tui.Text("  %s", fn.Doc).Style(docStyle),
				)
			}),
		).Gap(0))
		fmt.Println()
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

func docSyntaxText() error {
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	headingStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})

	views := []tui.View{
		tui.Text("Risor Syntax Reference").Style(titleStyle),
		tui.Text(""),
	}

	for _, section := range syntaxSections {
		views = append(views,
			tui.Text("%s", strings.ToUpper(section.Name)).Style(headingStyle),
			tui.ForEach(section.Items, func(item SyntaxItem, _ int) tui.View {
				return tui.Group(
					tui.Text("  %s", item.Syntax).Style(nameStyle),
					tui.Text("  %s", item.Notes).Style(docStyle),
				)
			}),
			tui.Text(""),
		)
	}

	tui.Print(tui.Stack(views...).Gap(0))
	fmt.Println()
	return nil
}

func docErrorsText() error {
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	headingStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})
	errorStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 100, B: 100})

	views := []tui.View{
		tui.Text("Common Errors and Fixes").Style(titleStyle),
		tui.Text(""),
	}

	for _, pattern := range errorPatterns {
		// Build views for causes
		causeViews := []tui.View{tui.Text("  Causes:").Style(docStyle)}
		for _, cause := range pattern.Causes {
			causeViews = append(causeViews, tui.Text("    - %s", cause).Style(docStyle))
		}

		// Build views for examples
		exampleViews := []tui.View{}
		for _, ex := range pattern.Examples {
			exampleViews = append(exampleViews,
				tui.Text("  %s", ex.Error).Style(errorStyle),
				tui.Text("    Bad:  %s", ex.BadCode).Style(nameStyle),
				tui.Text("    Fix:  %s", ex.Fix).Style(nameStyle),
				tui.Text("    %s", ex.Explanation).Style(docStyle),
				tui.Text(""),
			)
		}

		views = append(views,
			tui.Text("%s", strings.ToUpper(pattern.Type)).Style(headingStyle),
			tui.Text("  Pattern: %s", pattern.MessagePattern).Style(docStyle),
			tui.Stack(causeViews...).Gap(0),
			tui.Text(""),
			tui.Stack(exampleViews...).Gap(0),
		)
	}

	tui.Print(tui.Stack(views...).Gap(0))
	fmt.Println()
	return nil
}

func docBuiltinsText() error {
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})

	docs := builtins.Docs()
	tui.Print(tui.Stack(
		tui.Text("Built-in Functions").Style(titleStyle),
		tui.Text(""),
		tui.ForEach(docs, func(fn object.FuncSpec, _ int) tui.View {
			sig := formatSignature(fn.Name, fn.Args)
			return tui.Group(
				tui.Text("  %s", sig).Style(nameStyle),
				tui.Text("  %s", fn.Doc).Style(docStyle),
			)
		}),
	).Gap(0))
	fmt.Println()
	return nil
}

func docTypesText() error {
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})

	var typeNames []string
	for name := range typeDocs() {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	tui.Print(tui.Stack(
		tui.Text("Risor Types").Style(titleStyle),
		tui.Text(""),
		tui.ForEach(typeNames, func(name string, _ int) tui.View {
			t := typeDocs()[name]
			methodCount := len(t.Attrs)
			if methodCount > 0 {
				return tui.Group(
					tui.Text("  %s", name).Style(nameStyle),
					tui.Text("  %s (%d methods)", t.Doc, methodCount).Style(docStyle),
				)
			}
			return tui.Group(
				tui.Text("  %s", name).Style(nameStyle),
				tui.Text("  %s", t.Doc).Style(docStyle),
			)
		}),
	).Gap(0))
	fmt.Println()
	return nil
}

func docModulesText() error {
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	docStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})

	var moduleNames []string
	for name := range moduleDocs {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	tui.Print(tui.Stack(
		tui.Text("Risor Modules").Style(titleStyle),
		tui.Text(""),
		tui.ForEach(moduleNames, func(name string, _ int) tui.View {
			mod := moduleDocs[name]
			return tui.Group(
				tui.Text("  %s", name).Style(nameStyle),
				tui.Text("  %s (%d functions)", mod.Doc, len(mod.Funcs)).Style(docStyle),
			)
		}),
	).Gap(0))
	fmt.Println()
	return nil
}

// docHandlerJSON outputs documentation in JSON format.
func docHandlerJSON(topic string) error {
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

		for name, t := range typeDocs() {
			typeJSON, _ := json.Marshal(struct {
				Name    string            `json:"name"`
				Doc     string            `json:"doc"`
				Methods []object.AttrSpec `json:"methods,omitempty"`
			}{t.Name, t.Doc, t.Attrs})
			result.Types[name] = typeJSON
		}

		return jsonEncode(result)
	}

	// Handle special categories
	switch topic {
	case "syntax":
		return jsonEncode(SyntaxOverview{
			CategoryOverview: CategoryOverview{
				Category:    "syntax",
				Description: "Complete syntax reference",
			},
			Sections: syntaxSections,
		})
	case "errors":
		return jsonEncode(ErrorsOverview{
			CategoryOverview: CategoryOverview{
				Category:    "errors",
				Description: "Common error patterns and fixes",
				Count:       len(errorPatterns),
			},
			Patterns: errorPatterns,
		})
	case "builtins":
		return jsonEncode(BuiltinsOverview{
			CategoryOverview: CategoryOverview{
				Category:    "builtins",
				Description: "Built-in functions available in the global scope",
				Count:       len(builtins.Docs()),
				Next:        "Use 'risor doc <name>' for full docs and examples",
			},
			Functions: builtins.Docs(),
		})
	case "types":
		types := make([]TypeInfo, 0, len(typeDocs()))
		var typeNames []string
		for name := range typeDocs() {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)
		for _, name := range typeNames {
			t := typeDocs()[name]
			types = append(types, TypeInfo{
				Name:        name,
				Doc:         t.Doc,
				MethodCount: len(t.Attrs),
			})
		}
		return jsonEncode(TypesOverview{
			CategoryOverview: CategoryOverview{
				Category:    "types",
				Description: "Risor types and their methods",
				Count:       len(typeDocs()),
				Next:        "Use 'risor doc <type>' for methods",
			},
			Types: types,
		})
	case "modules":
		modules := make([]ModuleInfo, 0, len(moduleDocs))
		var moduleNames []string
		for name := range moduleDocs {
			moduleNames = append(moduleNames, name)
		}
		sort.Strings(moduleNames)
		for _, name := range moduleNames {
			mod := moduleDocs[name]
			modules = append(modules, ModuleInfo{
				Name:      name,
				Doc:       mod.Doc,
				FuncCount: len(mod.Funcs),
			})
		}
		return jsonEncode(ModulesOverview{
			CategoryOverview: CategoryOverview{
				Category:    "modules",
				Description: "Available modules",
				Count:       len(moduleDocs),
				Next:        "Use 'risor doc <module>' for functions",
			},
			Modules: modules,
		})
	}

	// Check module.function reference
	if strings.Contains(topic, ".") {
		parts := strings.SplitN(topic, ".", 2)
		moduleName, funcName := parts[0], parts[1]

		if mod, ok := moduleDocs[moduleName]; ok {
			for _, fn := range mod.Funcs {
				if fn.Name == funcName {
					return jsonEncode(ModuleFunctionDetail{
						Type:     "module_function",
						Module:   moduleName,
						Function: fn,
					})
				}
			}
			return fmt.Errorf("function %q not found in module %q", funcName, moduleName)
		}
	}

	// Check types
	if t, ok := typeDocs()[topic]; ok {
		return jsonEncode(TypeDetail{
			Type:    "type",
			Name:    t.Name,
			Doc:     t.Doc,
			Methods: t.Attrs,
		})
	}

	// Check modules
	if mod, ok := moduleDocs[topic]; ok {
		return jsonEncode(ModuleDetail{
			Type:  "module",
			Name:  topic,
			Doc:   mod.Doc,
			Funcs: mod.Funcs,
		})
	}

	// Check builtins
	for _, fn := range builtins.Docs() {
		if fn.Name == topic {
			return jsonEncode(BuiltinDetail{
				Type:     "builtin",
				Function: fn,
			})
		}
	}

	return fmt.Errorf("unknown topic: %q", topic)
}

// docHandlerMarkdown outputs documentation in Markdown format.
func docHandlerMarkdown(topic string) error {
	if topic == "" {
		var sb strings.Builder
		sb.WriteString("# Risor Language Reference\n\n")
		sb.WriteString(builtinsMarkdown())
		sb.WriteString("\n")
		sb.WriteString(modulesMarkdown())
		sb.WriteString("\n")
		sb.WriteString(typesMarkdown())
		fmt.Print(sb.String())
		return nil
	}

	// Handle special categories
	switch topic {
	case "syntax":
		fmt.Print(syntaxMarkdown())
		return nil
	case "errors":
		fmt.Print(errorsMarkdown())
		return nil
	case "builtins":
		fmt.Print(builtinsMarkdown())
		return nil
	case "types":
		fmt.Print(typesMarkdown())
		return nil
	case "modules":
		fmt.Print(modulesMarkdown())
		return nil
	}

	// Check module.function reference
	if strings.Contains(topic, ".") {
		parts := strings.SplitN(topic, ".", 2)
		moduleName, funcName := parts[0], parts[1]

		if mod, ok := moduleDocs[moduleName]; ok {
			for _, fn := range mod.Funcs {
				if fn.Name == funcName {
					fmt.Print(funcMarkdown(moduleName+"."+fn.Name, fn))
					return nil
				}
			}
			return fmt.Errorf("function %q not found in module %q", funcName, moduleName)
		}
	}

	// Check types
	if t, ok := typeDocs()[topic]; ok {
		fmt.Print(typeMarkdown(t))
		return nil
	}

	// Check modules
	if mod, ok := moduleDocs[topic]; ok {
		fmt.Print(moduleMarkdown(topic, mod.Doc, mod.Funcs))
		return nil
	}

	// Check builtins
	for _, fn := range builtins.Docs() {
		if fn.Name == topic {
			fmt.Print(funcMarkdown(fn.Name, fn))
			return nil
		}
	}

	return fmt.Errorf("unknown topic: %q", topic)
}

// Markdown helper functions

func builtinsMarkdown() string {
	var sb strings.Builder
	sb.WriteString("## Built-in Functions\n\n")
	sb.WriteString("| Function | Description |\n")
	sb.WriteString("|----------|-------------|\n")
	for _, fn := range builtins.Docs() {
		sig := formatSignature(fn.Name, fn.Args)
		sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", sig, fn.Doc))
	}
	return sb.String()
}

func modulesMarkdown() string {
	var sb strings.Builder
	sb.WriteString("## Modules\n\n")

	var moduleNames []string
	for name := range moduleDocs {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	for _, name := range moduleNames {
		mod := moduleDocs[name]
		sb.WriteString(fmt.Sprintf("### %s\n\n", name))
		sb.WriteString(fmt.Sprintf("%s\n\n", mod.Doc))
		sb.WriteString("| Function | Description |\n")
		sb.WriteString("|----------|-------------|\n")
		for _, fn := range mod.Funcs {
			sig := formatSignature(fn.Name, fn.Args)
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", sig, fn.Doc))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func typesMarkdown() string {
	var sb strings.Builder
	sb.WriteString("## Types\n\n")

	var typeNames []string
	for name := range typeDocs() {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	for _, name := range typeNames {
		t := typeDocs()[name]
		sb.WriteString(fmt.Sprintf("### %s\n\n", name))
		sb.WriteString(fmt.Sprintf("%s\n\n", t.Doc))
		if len(t.Attrs) > 0 {
			sb.WriteString("| Method | Description |\n")
			sb.WriteString("|--------|-------------|\n")
			for _, attr := range t.Attrs {
				sig := formatSignature(attr.Name, attr.Args)
				sb.WriteString(fmt.Sprintf("| `.%s` | %s |\n", sig, attr.Doc))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func syntaxMarkdown() string {
	var sb strings.Builder
	sb.WriteString("## Syntax Reference\n\n")

	for _, section := range syntaxSections {
		sb.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(section.Name)))
		sb.WriteString("| Syntax | Notes |\n")
		sb.WriteString("|--------|-------|\n")
		for _, item := range section.Items {
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", item.Syntax, item.Notes))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func errorsMarkdown() string {
	var sb strings.Builder
	sb.WriteString("## Common Errors\n\n")

	for _, pattern := range errorPatterns {
		sb.WriteString(fmt.Sprintf("### %s\n\n", pattern.Type))
		sb.WriteString(fmt.Sprintf("**Pattern:** `%s`\n\n", pattern.MessagePattern))
		sb.WriteString("**Causes:**\n")
		for _, cause := range pattern.Causes {
			sb.WriteString(fmt.Sprintf("- %s\n", cause))
		}
		sb.WriteString("\n**Examples:**\n\n")
		for _, ex := range pattern.Examples {
			sb.WriteString(fmt.Sprintf("- Error: `%s`\n", ex.Error))
			sb.WriteString(fmt.Sprintf("  - Bad: `%s`\n", ex.BadCode))
			sb.WriteString(fmt.Sprintf("  - Fix: `%s`\n", ex.Fix))
			sb.WriteString(fmt.Sprintf("  - %s\n\n", ex.Explanation))
		}
	}
	return sb.String()
}

func funcMarkdown(name string, fn object.FuncSpec) string {
	var sb strings.Builder
	sig := formatSignature(name, fn.Args)
	sb.WriteString(fmt.Sprintf("## %s\n\n", sig))
	sb.WriteString(fmt.Sprintf("%s\n\n", fn.Doc))
	if fn.Returns != "" {
		sb.WriteString(fmt.Sprintf("**Returns:** `%s`\n\n", fn.Returns))
	}
	if fn.Example != "" {
		sb.WriteString("**Example:**\n\n")
		sb.WriteString(fmt.Sprintf("```risor\n%s\n```\n", fn.Example))
	}
	return sb.String()
}

func typeMarkdown(t object.TypeSpec) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Type: %s\n\n", t.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", t.Doc))
	if len(t.Attrs) > 0 {
		sb.WriteString("### Methods\n\n")
		sb.WriteString("| Method | Description |\n")
		sb.WriteString("|--------|-------------|\n")
		for _, attr := range t.Attrs {
			sig := formatSignature(attr.Name, attr.Args)
			sb.WriteString(fmt.Sprintf("| `.%s` | %s |\n", sig, attr.Doc))
		}
	}
	return sb.String()
}

func moduleMarkdown(name, doc string, funcs []object.FuncSpec) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Module: %s\n\n", name))
	sb.WriteString(fmt.Sprintf("%s\n\n", doc))
	sb.WriteString("### Functions\n\n")
	sb.WriteString("| Function | Description |\n")
	sb.WriteString("|----------|-------------|\n")
	for _, fn := range funcs {
		sig := formatSignature(fn.Name, fn.Args)
		sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", sig, fn.Doc))
	}
	return sb.String()
}

func printFuncDoc(name string, fn object.FuncSpec, titleStyle, headingStyle, nameStyle, docStyle tui.Style) {
	sig := formatSignature(name, fn.Args)

	views := []tui.View{
		tui.Text("%s", sig).Style(titleStyle),
		tui.Text(""),
		tui.Text("%s", fn.Doc).Style(docStyle),
	}

	if fn.Returns != "" {
		views = append(views,
			tui.Text(""),
			tui.Group(
				tui.Text("Returns: ").Style(headingStyle),
				tui.Text("%s", fn.Returns).Style(nameStyle),
			),
		)
	}

	if fn.Example != "" {
		views = append(views,
			tui.Text(""),
			tui.Text("EXAMPLE").Style(headingStyle),
			tui.Text("  %s", fn.Example).Style(nameStyle),
		)
	}

	tui.Print(tui.Stack(views...).Gap(0))
	fmt.Println()
}

func formatSignature(name string, args []string) string {
	if len(args) == 0 {
		return name + "()"
	}
	return name + "(" + strings.Join(args, ", ") + ")"
}

func jsonEncode(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}


