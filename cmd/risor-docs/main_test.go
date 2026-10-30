package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestListModules(t *testing.T) {
	// The tool runs from the repository root.
	wd, err := os.Getwd()
	assert.Nil(t, err)
	assert.Nil(t, os.Chdir(filepath.Join("..", "..")))
	defer os.Chdir(wd)

	names, err := listModules()
	assert.Nil(t, err)
	assert.Equal(t, names, []string{
		filepath.Join("pkg", "modules", "math"),
		filepath.Join("pkg", "modules", "rand"),
		filepath.Join("pkg", "modules", "regexp"),
	})
	for _, name := range names {
		_, err := os.Stat(filepath.Join(name, filepath.Base(name)+".md"))
		assert.Nil(t, err)
	}
}
