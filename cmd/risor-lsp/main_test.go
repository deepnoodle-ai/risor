package main

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestOpenLogFileRestrictsPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix permission bits")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
	cacheDir, err := os.UserCacheDir()
	assert.Nil(t, err)

	// A log from an earlier version, created world-readable.
	path := filepath.Join(cacheDir, "risor", "risor-lsp.log")
	assert.Nil(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.Nil(t, os.WriteFile(path, nil, 0o644))

	logFile, err := openLogFile()
	assert.Nil(t, err)
	defer logFile.Close()

	info, err := os.Stat(path)
	assert.Nil(t, err)
	assert.Equal(t, info.Mode().Perm(), os.FileMode(0o600))
}
