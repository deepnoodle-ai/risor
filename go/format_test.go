package risor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/runner"
)

// suiteText is a program or template text of the suite.
type suiteText struct {
	where    string
	text     string
	template bool
}

// suiteTexts collects every program text of the suite: the source of each
// case, the file a case names, and the text of each format case, from the
// case files and the catalogs, runner cases' inner files included.
func suiteTexts(t *testing.T) []suiteText {
	t.Helper()
	var out []suiteText
	seen := map[suiteText]bool{}
	add := func(s suiteText) {
		k := s
		k.where = ""
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	var walk func(where, dir string, x any, template bool)
	walk = func(where, dir string, x any, template bool) {
		switch v := x.(type) {
		case []any:
			for _, e := range v {
				walk(where, dir, e, template)
			}
		case map[string]any:
			tpl := template
			if m, ok := v["template"].(string); ok && (m == "string" || m == "value") {
				tpl = true
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				e := v[k]
				s, isText := e.(string)
				switch {
				case k == "source" && isText:
					add(suiteText{where, s, tpl})
				case k == "format" && isText:
					add(suiteText{where, s, false})
				case k == "formatTemplate" && isText:
					add(suiteText{where, s, true})
				case k == "file" && isText:
					if data, err := os.ReadFile(filepath.Join(dir, s)); err == nil {
						add(suiteText{where, string(data), tpl})
					}
				case k == "runner" && isText:
					var inner any
					if json.Unmarshal([]byte(s), &inner) == nil {
						walk(where, dir, inner, false)
					}
				default:
					walk(where, dir, e, tpl)
				}
			}
		}
	}
	root := filepath.Join("..", "suite")
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var x any
		if err := json.Unmarshal(data, &x); err != nil {
			return nil
		}
		walk(p, filepath.Dir(p), x, false)
		return nil
	})
	assert.Nil(t, err)
	return out
}

// TestFormatLawsOverSuite formats every program text of the suite and tests
// the laws of package F12 2.2 on each that parses (F12 7.6 item 3): the
// formatted text formats to itself, and parses to the same tree.
func TestFormatLawsOverSuite(t *testing.T) {
	texts := suiteTexts(t)
	n := 0
	for _, s := range texts {
		format := risor.Format
		if s.template {
			format = risor.FormatTemplate
		}
		out, err := format(s.text)
		if err != nil {
			continue
		}
		n++
		again, err := format(out)
		if err != nil {
			t.Errorf("%s: %q: the formatted text does not parse: %v\n%s", s.where, s.text, err, out)
			continue
		}
		if again != out {
			t.Errorf("%s: %q: not idempotent:\n%s\n---\n%s", s.where, s.text, out, again)
		}
		if a, b := runner.TreeOf(s.template, s.text), runner.TreeOf(s.template, out); a != b {
			t.Errorf("%s: %q: the tree differs:\n%s\n---\n%s", s.where, s.text, a, b)
		}
	}
	t.Logf("%d texts, %d formatted", len(texts), n)
	assert.True(t, n > 5000)
}
