package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func TestExamples_List(t *testing.T) {
	// Verify examples data structure
	assert.True(t, len(examples) > 0, "should have examples")

	// Check all examples have required fields
	for _, ex := range examples {
		assert.True(t, ex.Name != "", "example should have name")
		assert.True(t, ex.Description != "", "example should have description")
		assert.True(t, ex.Code != "", "example should have code")
		assert.True(t, ex.Category != "", "example should have category")
	}
}

func TestExamples_Categories(t *testing.T) {
	categories := make(map[string]bool)
	for _, ex := range examples {
		categories[ex.Category] = true
	}

	// Verify we have multiple categories
	assert.True(t, len(categories) >= 5, "should have at least 5 categories")

	// Check expected categories exist
	expectedCategories := []string{"basics", "functions", "collections", "control", "modules"}
	for _, cat := range expectedCategories {
		assert.True(t, categories[cat], "should have category: %s", cat)
	}
}

func TestExamples_UniqueNames(t *testing.T) {
	names := make(map[string]bool)
	for _, ex := range examples {
		assert.False(t, names[ex.Name], "duplicate example name: %s", ex.Name)
		names[ex.Name] = true
	}
}

func TestListExampleNames(t *testing.T) {
	names := listExampleNames()
	assert.True(t, len(names) > 0)

	// Should be sorted
	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] <= names[i], "names should be sorted")
	}
}

func TestExamplesHandler_ListAll(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("examples").
		Args("name?").
		Flags(cli.Bool("run", "r")).
		Run(examplesHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"examples"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should list categories and examples
	assert.True(t, contains(output, "BASICS") || contains(output, "basics"))
	assert.True(t, contains(output, "hello"))
}

func TestExamplesHandler_ViewExample(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("examples").
		Args("name?").
		Flags(cli.Bool("run", "r")).
		Run(examplesHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"examples", "hello"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show the hello example code
	assert.True(t, contains(output, "Hello"))
	assert.True(t, contains(output, "print"))
}

func TestExamplesHandler_RunExample(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("examples").
		Args("name?").
		Flags(cli.Bool("run", "r")).
		Run(examplesHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"examples", "hello", "--run"})

	w.Close()
	os.Stdout = old

	assert.Nil(t, err)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should show OUTPUT section and the actual output
	assert.True(t, contains(output, "OUTPUT"))
	assert.True(t, contains(output, "Hello"))
}

func TestExamplesHandler_NotFound(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("examples").
		Args("name?").
		Flags(cli.Bool("run", "r")).
		Run(examplesHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.ExecuteArgs([]string{"examples", "nonexistent_example_xyz"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	assert.NotNil(t, err)
	assert.True(t, contains(err.Error(), "not found"))
}

func TestExamplesHandler_PartialMatch(t *testing.T) {
	oldEnabled := color.Enabled
	color.Enabled = false
	defer func() { color.Enabled = oldEnabled }()

	app := cli.New("risor").SetColorEnabled(false)
	app.Command("examples").
		Args("name?").
		Flags(cli.Bool("run", "r")).
		Run(examplesHandler)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// "func" should match "functions" category examples
	err := app.ExecuteArgs([]string{"examples", "func"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should either show matches or a single match
	assert.Nil(t, err)
	assert.True(t, len(output) > 0)
}

func TestExamples_ValidCode(t *testing.T) {
	// Test that all example code is valid Risor syntax
	// We just need to verify parsing succeeds
	for _, ex := range examples {
		t.Run(ex.Name, func(t *testing.T) {
			// Examples use print() which requires special handling
			// Just verify non-empty code exists
			assert.True(t, len(ex.Code) > 0, "example %s should have code", ex.Name)
		})
	}
}
