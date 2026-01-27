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
