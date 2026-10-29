package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestOpenLogFileUsesUserCacheDir(t *testing.T) {
	dir := t.TempDir()
	// os.UserCacheDir reads XDG_CACHE_HOME on Linux, HOME on macOS, and
	// LocalAppData on Windows.
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("LocalAppData", dir)
	cacheDir, err := os.UserCacheDir()
	assert.Nil(t, err)

	logFile, err := openLogFile()
	assert.Nil(t, err)
	defer logFile.Close()

	want := filepath.Join(cacheDir, "risor", "risor-lsp.log")
	assert.Equal(t, logFile.Name(), want)
	_, err = os.Stat(want)
	assert.Nil(t, err)
}
