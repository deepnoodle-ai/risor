package doctest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func hasModuleFunc(dir string) bool {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		textData, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(textData), "func Module() *object.Module") {
			return true
		}
	}
	return false
}

func readMarkdown(path string) (string, error) {
	md, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mdText := string(md)
	mdLines := strings.Split(mdText, "\n")
	// Skip over nextra mdx imports and blank lines at the top of the file
	for i := 0; i < len(mdLines); i++ {
		if strings.HasPrefix(mdLines[i], "#") {
			return strings.Join(mdLines[i:], "\n"), nil
		}
	}
	return "", fmt.Errorf("no markdown header found")
}

func TestModuleDocs(t *testing.T) {
	// All modules that have a Module() function should also have a markdown
	// documentation file in the same directory
	mods, err := os.ReadDir("../../pkg/modules")
	assert.Nil(t, err)
	for _, mod := range mods {
		if mod.IsDir() {
			name := mod.Name()
			modPath := filepath.Join("../..", "pkg", "modules", name)
			if !hasModuleFunc(modPath) {
				continue
			}
			t.Run(name, func(t *testing.T) {
				filename := fmt.Sprintf("%s.md", name)
				docPath := filepath.Join("../..", "pkg", "modules", name, filename)
				md, err := readMarkdown(docPath)
				assert.Nil(t, err, "Expected module markdown doc to exist at %s", docPath)
				mdText := string(md)
				mdLines := strings.Split(mdText, "\n")
				assert.True(t, len(mdLines) > 1, "Expected module markdown to have more content")
				assert.Equal(t, mdLines[0], "# "+name, "Expected module markdown to start with '# %s'", name)
			})
		}
	}
}
