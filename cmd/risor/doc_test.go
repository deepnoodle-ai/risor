package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestDocHandler_Overview(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show overview with builtins, modules, and types
	assert.True(t, contains(output, "BUILTINS") || contains(output, "builtins"))
	assert.True(t, contains(output, "MODULES") || contains(output, "modules"))
	assert.True(t, contains(output, "TYPES") || contains(output, "types"))
}

func TestDocHandler_Type(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "string"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show string type info
	assert.True(t, contains(output, "string"))
	assert.True(t, contains(output, "METHODS") || contains(output, "methods"))
}

func TestDocHandler_Module(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "math"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show math module info
	assert.True(t, contains(output, "math"))
	assert.True(t, contains(output, "sqrt") || contains(output, "abs"))
}

func TestDocHandler_Builtin(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "len"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show len function info
	assert.True(t, contains(output, "len"))
}

func TestDocHandler_ModuleFunction(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "math.sqrt"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show math.sqrt function info
	assert.True(t, contains(output, "sqrt"))
}

func TestDocHandler_Unknown(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Run(docHandler)

	err := app.ExecuteArgs([]string{"doc", "nonexistent_topic_xyz"})

	assert.NotNil(t, err)
	assert.True(t, contains(err.Error(), "unknown"))
}

func TestTypeDocs(t *testing.T) {
	// Verify all type docs have required fields
	for name, spec := range typeDocs {
		assert.True(t, spec.Name != "", "type %s should have name", name)
		assert.True(t, spec.Doc != "", "type %s should have doc", name)
	}
}

func TestModuleDocs(t *testing.T) {
	// Verify all module docs have required fields
	for name, mod := range moduleDocs {
		assert.True(t, mod.Doc != "", "module %s should have doc", name)
		assert.True(t, len(mod.Funcs) > 0, "module %s should have functions", name)

		for _, fn := range mod.Funcs {
			assert.True(t, fn.Name != "", "function in %s should have name", name)
			assert.True(t, fn.Doc != "", "function %s.%s should have doc", name, fn.Name)
		}
	}
}

func TestFormatSignature(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"foo", nil, "foo()"},
		{"bar", []string{}, "bar()"},
		{"add", []string{"a", "b"}, "add(a, b)"},
		{"greet", []string{"name", "msg?"}, "greet(name, msg?)"},
	}

	for _, tt := range tests {
		result := formatSignature(tt.name, tt.args)
		assert.Equal(t, result, tt.expected)
	}
}

func TestDocHandler_QuickReference(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Flags(
			cli.String("format", "f").Enum("json", "text", "markdown"),
			cli.Bool("quick", "q"),
			cli.Bool("all", "a"),
		).
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "--quick"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show quick reference with syntax patterns
	assert.True(t, contains(output, "Risor"))
	assert.True(t, contains(output, "SYNTAX") || contains(output, "syntax"))
	assert.True(t, contains(output, "let x = 1"))
}

func TestDocHandler_QuickJSON(t *testing.T) {
	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Flags(
			cli.String("format", "f").Enum("json", "text", "markdown"),
			cli.Bool("quick", "q"),
			cli.Bool("all", "a"),
		).
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "--quick", "--format", "json"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should be valid JSON with expected fields
	assert.True(t, contains(output, `"risor"`))
	assert.True(t, contains(output, `"version"`))
	assert.True(t, contains(output, `"syntax_quick_ref"`))
	assert.True(t, contains(output, `"topics"`))
}

func TestDocHandler_Syntax(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Flags(
			cli.String("format", "f").Enum("json", "text", "markdown"),
			cli.Bool("quick", "q"),
			cli.Bool("all", "a"),
		).
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "syntax"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show syntax reference
	assert.True(t, contains(output, "LITERALS") || contains(output, "literals"))
	assert.True(t, contains(output, "FUNCTIONS") || contains(output, "functions"))
}

func TestDocHandler_Errors(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Flags(
			cli.String("format", "f").Enum("json", "text", "markdown"),
			cli.Bool("quick", "q"),
			cli.Bool("all", "a"),
		).
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "errors"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show error patterns
	assert.True(t, contains(output, "TYPE_ERROR") || contains(output, "type_error"))
	assert.True(t, contains(output, "NAME_ERROR") || contains(output, "name_error"))
}

func TestDocHandler_MarkdownFormat(t *testing.T) {
	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Flags(
			cli.String("format", "f").Enum("json", "text", "markdown"),
			cli.Bool("quick", "q"),
			cli.Bool("all", "a"),
		).
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "--format", "markdown"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should output markdown with headers and tables
	assert.True(t, contains(output, "# Risor"))
	assert.True(t, contains(output, "## Built-in Functions"))
	assert.True(t, contains(output, "|"))
}

func TestDocHandler_JSONFormat(t *testing.T) {
	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Flags(
			cli.String("format", "f").Enum("json", "text", "markdown"),
			cli.Bool("quick", "q"),
			cli.Bool("all", "a"),
		).
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "--format", "json"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should be valid JSON with expected structure
	assert.True(t, contains(output, `"builtins"`))
	assert.True(t, contains(output, `"modules"`))
	assert.True(t, contains(output, `"types"`))
}

func TestDocHandler_BuiltinsCategory(t *testing.T) {
	app := cli.New("risor").SetColorEnabled(false)
	app.Command("doc").
		Args("topic?").
		Flags(
			cli.String("format", "f").Enum("json", "text", "markdown"),
			cli.Bool("quick", "q"),
			cli.Bool("all", "a"),
		).
		Run(docHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"doc", "builtins", "--format", "json"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show builtins category
	assert.True(t, contains(output, `"category": "builtins"`))
	assert.True(t, contains(output, `"functions"`))
}

func TestSyntaxSections(t *testing.T) {
	// Verify all syntax sections have required fields
	for _, section := range syntaxSections {
		assert.True(t, section.Name != "", "section should have name")
		assert.True(t, len(section.Items) > 0, "section %s should have items", section.Name)

		for _, item := range section.Items {
			assert.True(t, item.Syntax != "", "item in %s should have syntax", section.Name)
			assert.True(t, item.Notes != "", "item in %s should have notes", section.Name)
		}
	}
}

func TestErrorPatterns(t *testing.T) {
	// Verify all error patterns have required fields
	for _, pattern := range errorPatterns {
		assert.True(t, pattern.Type != "", "pattern should have type")
		assert.True(t, pattern.MessagePattern != "", "pattern %s should have message pattern", pattern.Type)
		assert.True(t, len(pattern.Causes) > 0, "pattern %s should have causes", pattern.Type)
		assert.True(t, len(pattern.Examples) > 0, "pattern %s should have examples", pattern.Type)

		for _, ex := range pattern.Examples {
			assert.True(t, ex.Error != "", "example in %s should have error", pattern.Type)
			assert.True(t, ex.BadCode != "", "example in %s should have bad code", pattern.Type)
			assert.True(t, ex.Fix != "", "example in %s should have fix", pattern.Type)
		}
	}
}
