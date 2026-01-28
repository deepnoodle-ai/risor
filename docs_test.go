package risor

import (
	"encoding/json"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestDocs_Quick(t *testing.T) {
	docs := Docs(DocsQuick())
	json := docs.JSON()

	assert.True(t, len(json) > 0)
	assert.True(t, contains(json, "version"))
	assert.True(t, contains(json, "syntax_quick_ref"))
	assert.True(t, contains(json, "topics"))
}

func TestDocs_All(t *testing.T) {
	docs := Docs(DocsAll())
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "builtins"))
	assert.True(t, contains(j, "modules"))
	assert.True(t, contains(j, "types"))
	assert.True(t, contains(j, "syntax"))
	assert.True(t, contains(j, "errors"))
}

func TestDocs_Category_Builtins(t *testing.T) {
	docs := Docs(DocsCategory("builtins"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "builtins"))
	assert.True(t, contains(j, "functions"))
}

func TestDocs_Category_Types(t *testing.T) {
	docs := Docs(DocsCategory("types"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "types"))
}

func TestDocs_Category_Modules(t *testing.T) {
	docs := Docs(DocsCategory("modules"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "modules"))
}

func TestDocs_Category_Syntax(t *testing.T) {
	docs := Docs(DocsCategory("syntax"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "sections"))
}

func TestDocs_Category_Errors(t *testing.T) {
	docs := Docs(DocsCategory("errors"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "patterns"))
}

func TestDocs_Topic_Type(t *testing.T) {
	docs := Docs(DocsTopic("string"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "string"))
	assert.True(t, contains(j, "methods"))
}

func TestDocs_Topic_Module(t *testing.T) {
	docs := Docs(DocsTopic("math"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "math"))
	assert.True(t, contains(j, "functions"))
}

func TestDocs_Topic_Builtin(t *testing.T) {
	docs := Docs(DocsTopic("len"))
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "len"))
	assert.True(t, contains(j, "builtin"))
}

func TestDocs_Default(t *testing.T) {
	// Default returns quick reference
	docs := Docs()
	j := docs.JSON()

	assert.True(t, len(j) > 0)
	assert.True(t, contains(j, "syntax_quick_ref"))
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
