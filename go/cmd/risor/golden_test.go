package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// The golden-file tests of the command-line program (package F12 1.2 item
// 2, 4.2 item 5). Each directory of testdata/cli is one run: the file args
// holds the arguments, one a line; stdin, when present, is standard input;
// every other file but those named want.* is copied to a temporary
// directory, which is the working directory of the run. want.stdout,
// want.stderr and want.status hold the bytes written and the exit status.
// After the run, the files named want.files/NAME hold what the run left in
// NAME. Run with -update to write the want files.

var update = flag.Bool("update", false, "write the golden files of testdata/cli")

func TestGolden(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	root, err := filepath.Abs(filepath.Join("testdata", "cli"))
	assert.NoError(t, err)
	entries, err := os.ReadDir(root)
	assert.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) { golden(t, filepath.Join(root, name)) })
	}
}

func golden(t *testing.T, dir string) {
	work := t.TempDir()
	var args []string
	var stdin []byte
	var keep []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		switch {
		case rel == "args":
			for _, l := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
				if l != "" {
					args = append(args, l)
				}
			}
		case rel == "stdin":
			stdin = data
		case strings.HasPrefix(rel, "want."):
			if strings.HasPrefix(rel, "want.files"+string(filepath.Separator)) {
				keep = append(keep, strings.TrimPrefix(rel, "want.files"+string(filepath.Separator)))
			}
		default:
			dst := filepath.Join(work, rel)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			return os.WriteFile(dst, data, 0o644)
		}
		return nil
	})
	assert.NoError(t, err)
	t.Chdir(work)
	var out, errb bytes.Buffer
	status := runStdin(args, bytes.NewReader(stdin), &out, &errb)
	got := map[string]string{
		"want.stdout": out.String(),
		"want.stderr": errb.String(),
		"want.status": strconv.Itoa(status) + "\n",
	}
	sort.Strings(keep)
	for _, k := range keep {
		data, err := os.ReadFile(filepath.Join(work, k))
		if err != nil {
			data = []byte("(missing)\n")
		}
		got[filepath.Join("want.files", k)] = string(data)
	}
	for k, v := range got {
		p := filepath.Join(dir, k)
		if *update {
			assert.NoError(t, os.WriteFile(p, []byte(v), 0o644))
			continue
		}
		want, err := os.ReadFile(p)
		if err != nil {
			want = nil
		}
		assert.Equal(t, v, string(want), k)
	}
}
