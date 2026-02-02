package docs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestDocs_Quick(t *testing.T) {
	docs := Docs(DocsQuick())
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "version"))
	assert.True(t, strings.Contains(j, "syntax_quick_ref"))
	assert.True(t, strings.Contains(j, "topics"))
}

func TestDocs_All(t *testing.T) {
	docs := Docs(DocsAll())
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "builtins"))
	assert.True(t, strings.Contains(j, "modules"))
	assert.True(t, strings.Contains(j, "types"))
	assert.True(t, strings.Contains(j, "syntax"))
	assert.True(t, strings.Contains(j, "errors"))
}

func TestDocs_Category_Builtins(t *testing.T) {
	docs := Docs(DocsCategory("builtins"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "builtins"))
	assert.True(t, strings.Contains(j, "functions"))
}

func TestDocs_Category_Types(t *testing.T) {
	docs := Docs(DocsCategory("types"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "types"))
}

func TestDocs_Category_Modules(t *testing.T) {
	docs := Docs(DocsCategory("modules"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "modules"))
}

func TestDocs_Category_Syntax(t *testing.T) {
	docs := Docs(DocsCategory("syntax"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "sections"))
}

func TestDocs_Category_Errors(t *testing.T) {
	docs := Docs(DocsCategory("errors"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "patterns"))
}

func TestDocs_Topic_Type(t *testing.T) {
	docs := Docs(DocsTopic("string"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "string"))
	assert.True(t, strings.Contains(j, "methods"))
}

func TestDocs_Topic_Module(t *testing.T) {
	docs := Docs(DocsTopic("math"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "math"))
	assert.True(t, strings.Contains(j, "functions"))
}

func TestDocs_Topic_Builtin(t *testing.T) {
	docs := Docs(DocsTopic("len"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "len"))
	assert.True(t, strings.Contains(j, "builtin"))
}

func TestDocs_Default(t *testing.T) {
	// Default returns quick reference
	docs := Docs()
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, strings.Contains(j, "syntax_quick_ref"))
}

func TestDocs_Data(t *testing.T) {
	docs := Docs(DocsQuick())
	data := docs.Data()

	assert.NotNil(t, data)

	// Should be a docsQuickReference
	ref, ok := data.(docsQuickReference)
	assert.True(t, ok)
	assert.Equal(t, ref.Risor.Version, Version)
}

func TestDocs_ValidJSON(t *testing.T) {
	// Test that all doc modes produce valid JSON
	testCases := []struct {
		name string
		opts []DocsOption
	}{
		{"quick", []DocsOption{DocsQuick()}},
		{"all", []DocsOption{DocsAll()}},
		{"category_builtins", []DocsOption{DocsCategory("builtins")}},
		{"category_types", []DocsOption{DocsCategory("types")}},
		{"category_modules", []DocsOption{DocsCategory("modules")}},
		{"category_syntax", []DocsOption{DocsCategory("syntax")}},
		{"category_errors", []DocsOption{DocsCategory("errors")}},
		{"topic_string", []DocsOption{DocsTopic("string")}},
		{"topic_math", []DocsOption{DocsTopic("math")}},
		{"topic_len", []DocsOption{DocsTopic("len")}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			docs := Docs(tc.opts...)
			j := docs.JSON()

			var result any
			err := json.Unmarshal([]byte(j), &result)
			assert.Nil(t, err, "should produce valid JSON for %s", tc.name)
		})
	}
}

func TestDocumentedModulesExist(t *testing.T) {
	// Verify that all documented modules have valid entries
	docs := Docs(DocsCategory("modules"))
	data := docs.Data().(map[string]any)
	modules := data["modules"].([]docsModuleInfo)

	assert.True(t, len(modules) > 0, "should have documented modules")

	for _, mod := range modules {
		t.Run(mod.Name, func(t *testing.T) {
			assert.True(t, mod.Name != "", "module should have a name")
			assert.True(t, mod.FuncCount > 0, "module %q should have functions", mod.Name)
		})
	}
}
