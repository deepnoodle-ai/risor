package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/render"
	"github.com/deepnoodle-ai/risor/v3/internal/runner"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// testFlags are the arguments of risor test (F9 8.1).
type testFlags struct {
	paths  []string
	runs   []string
	update bool
	format string
	suite  string
	color  colorFlag // F12 4.7 item 1
}

func parseTestFlags(args []string) (*testFlags, error) {
	f := &testFlags{format: "text", color: colorFlag{"auto"}}
	value := func(i *int, name string) (string, error) {
		a := args[*i]
		if v, ok := strings.CutPrefix(a, name+"="); ok {
			return v, nil
		}
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%s needs a value", name)
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if ok, err := f.color.parse(args, &i); ok || err != nil {
			if err != nil {
				return nil, err
			}
			continue
		}
		switch {
		case a == "--run" || strings.HasPrefix(a, "--run="):
			v, err := value(&i, "--run")
			if err != nil {
				return nil, err
			}
			f.runs = append(f.runs, v)
		case a == "--update":
			f.update = true
		case a == "--format" || strings.HasPrefix(a, "--format="):
			v, err := value(&i, "--format")
			if err != nil {
				return nil, err
			}
			if v != "text" && v != "json" {
				return nil, fmt.Errorf("--format is text or json, not %q", v)
			}
			f.format = v
		case a == "--suite" || strings.HasPrefix(a, "--suite="):
			v, err := value(&i, "--suite")
			if err != nil {
				return nil, err
			}
			f.suite = v
		case a == "--":
			f.paths = append(f.paths, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			return nil, fmt.Errorf("unknown flag %s", a)
		default:
			f.paths = append(f.paths, a)
		}
	}
	if f.suite != "" && (len(f.paths) > 0 || len(f.runs) > 0 || f.update) {
		return nil, errors.New("--suite takes no PATH, --run or --update")
	}
	return f, nil
}

// counts are the totals of the summary line (F9 8.3).
type counts struct {
	files, cases, pass, fail, skip, invalid, updated, badFiles int
	missing                                                    bool
}

func (c *counts) status() int {
	switch {
	case c.badFiles > 0 || c.invalid > 0 || c.missing:
		return 2
	case c.fail > 0:
		return 1
	}
	return 0
}

func (c *counts) add(r runner.Result) {
	c.cases++
	switch {
	case r.Updated:
		c.updated++
	case r.Verdict == runner.Pass:
		c.pass++
	case r.Verdict == runner.Fail:
		c.fail++
	case r.Verdict == runner.Skip:
		c.skip++
	case r.Verdict == runner.Invalid:
		c.invalid++
	}
}

func testCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f, err := parseTestFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "risor test: %v\n%s", err, usage)
		return 2
	}
	if f.suite != "" {
		return suiteCommand(f, stdout, stderr)
	}
	if len(f.paths) == 0 {
		f.paths = []string{"."}
	}
	var files []string
	var c counts
	for _, p := range f.paths {
		if p == "-" { // standard input (F12 4.2 item 1)
			files = append(files, p)
			continue
		}
		found, err := findFiles(p)
		if err != nil {
			fmt.Fprintf(stderr, "risor test: %s: no such file or directory\n", p)
			c.missing = true
			continue
		}
		files = append(files, found...)
	}
	var selected func(string) bool
	if len(f.runs) > 0 {
		selected = func(name string) bool {
			for _, r := range f.runs {
				if strings.Contains(name, r) {
					return true
				}
			}
			return false
		}
	}
	ctx := &runner.Ctx{Update: f.update}
	color := f.format == "text" && f.color.on(stdout, stdout)
	for _, path := range files {
		c.files++
		lf, err := loadFile(path, f.update, stdin)
		if err != nil {
			c.badFiles++
			reason := err.Error()
			var inv *casefile.InvalidError
			if errors.As(err, &inv) {
				reason = inv.Reason
			}
			if f.format == "json" {
				fmt.Fprintln(stdout, jsonLine(map[string]any{"file": path, "message": reason, "status": "invalid"}))
			} else {
				fmt.Fprintf(stdout, "%s\n%s\n", path, paintWord("  INVALID", color))
				for _, l := range strings.Split(reason, "\n") {
					fmt.Fprintf(stdout, "    %s\n", render.Visible(l))
				}
			}
			continue
		}
		rs := runner.RunLoaded(ctx, lf, selected)
		for _, r := range rs {
			c.add(r)
		}
		if f.format == "json" {
			for _, r := range rs {
				fmt.Fprintln(stdout, caseLine(path, r))
			}
		} else if rep := runner.Report(rs); rep != "" {
			fmt.Fprintf(stdout, "%s\n%s", path, paintReport(rep, color))
		}
		if f.update && path != "-" {
			text, written, err := runner.UpdatedText(lf, rs)
			if err != nil {
				fmt.Fprintf(stderr, "risor test: %s: %v\n", path, err)
				c.missing = true
			} else if written {
				if err := os.WriteFile(path, text, 0o644); err != nil {
					fmt.Fprintf(stderr, "risor test: %s: %v\n", path, err)
					c.missing = true
				}
			}
		}
	}
	if f.format == "text" {
		line := fmt.Sprintf("%d files, %d cases: %d pass, %d fail, %d skip, %d invalid",
			c.files, c.cases, c.pass, c.fail, c.skip, c.invalid)
		if f.update {
			line += fmt.Sprintf(", %d updated", c.updated)
		}
		if c.badFiles > 0 {
			line += fmt.Sprintf("; %d files invalid", c.badFiles)
		}
		fmt.Fprintln(stdout, line)
	}
	return c.status()
}

// loadFile reads a case file: a program case file when its name ends in
// .test.risor, else JSON (F9 8.2 item 1).
func loadFile(path string, update bool, stdin io.Reader) (*runner.LoadedFile, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, &casefile.InvalidError{Path: path, Reason: err.Error()}
	}
	opts := casefile.Options{Update: update}
	lf := &runner.LoadedFile{Text: data}
	if strings.HasSuffix(path, ".test.risor") {
		lf.Program = true
		opts.Update = false
		lf.File, err = runner.LoadProgram(data, path, opts)
	} else {
		lf.File, err = casefile.ParseWith(data, path, opts)
	}
	if err != nil {
		return nil, err
	}
	return lf, nil
}

// findFiles is F9 8.2: a file is run whatever its name; a directory is
// searched for .test.json and .test.risor files, passing over names that
// begin with a dot and not following symbolic links, in code point order of
// their relative paths, part by part.
func findFiles(p string) ([]string, error) {
	st, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return []string{p}, nil
	}
	var rels [][]string
	err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != p && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || d.IsDir() {
			return nil
		}
		n := d.Name()
		if strings.HasSuffix(n, ".test.json") || strings.HasSuffix(n, ".test.risor") {
			rel, _ := filepath.Rel(p, path)
			rels = append(rels, strings.Split(filepath.ToSlash(rel), "/"))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rels, func(i, j int) bool {
		a, b := rels[i], rels[j]
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return lessCodePoints(a[k], b[k])
			}
		}
		return len(a) < len(b)
	})
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = filepath.Join(append([]string{p}, r...)...)
	}
	return out, nil
}

// lessCodePoints compares two texts by code point; Go's string order is
// byte order, which is code point order for UTF-8.
func lessCodePoints(a, b string) bool { return a < b }

// caseLine is one line of the JSON output (F9 8.4).
func caseLine(file string, r runner.Result) string {
	m := map[string]any{"file": file, "name": r.Case, "status": r.Status()}
	switch {
	case r.Updated:
	case r.Verdict == runner.Fail:
		ds := make([]any, len(r.Diffs))
		for i, d := range r.Diffs {
			ds[i] = d.Name
		}
		m["differs"] = ds
	case r.Verdict == runner.Invalid:
		if r.Fixture != nil {
			m["fixture"] = r.Fixture
			m["message"] = r.Message
		} else {
			m["message"] = r.Reason
		}
	case r.Verdict == runner.Skip:
		m["reason"] = r.Reason
	}
	return jsonLine(m)
}

func jsonLine(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(value.QuoteJSON(k) + ":")
		writeJSON(&b, m[k])
	}
	b.WriteByte('}')
	return b.String()
}

func writeJSON(b *strings.Builder, v any) {
	switch x := v.(type) {
	case string:
		b.WriteString(value.QuoteJSON(x))
	case float64:
		b.WriteString(value.FormatNumber(x))
	case int:
		fmt.Fprint(b, x)
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, e)
		}
		b.WriteByte(']')
	default:
		b.WriteString("null")
	}
}

// suiteCommand is risor test --suite (F9 8.6).
func suiteCommand(f *testFlags, stdout, stderr io.Writer) int {
	type entry struct {
		rel string
		run func() ([]runner.Result, error)
	}
	var es []entry
	for _, d := range runner.SuiteDirs {
		dir := filepath.Join(f.suite, d)
		es = append(es, entry{d, func() ([]runner.Result, error) { return runner.RunSuiteDir(dir) }})
	}
	for _, c := range runner.SuiteCatalogs {
		path, key := filepath.Join(f.suite, c.Rel), c.Key
		es = append(es, entry{c.Rel, func() ([]runner.Result, error) { return runner.RunSuiteCatalog(path, key) }})
	}
	sort.Slice(es, func(i, j int) bool { return es[i].rel < es[j].rel })
	var total counts
	for _, e := range es {
		rs, err := e.run()
		if err != nil {
			fmt.Fprintf(stderr, "risor test: %s: %v\n", filepath.Join(f.suite, e.rel), err)
			total.missing = true
			continue
		}
		var c counts
		for _, r := range rs {
			c.add(r)
			total.add(r)
		}
		if f.format == "json" {
			for _, r := range rs {
				fmt.Fprintln(stdout, caseLine(r.File, r))
			}
			continue
		}
		fmt.Fprintf(stdout, "%s: %d pass, %d fail, %d skip, %d invalid\n", e.rel, c.pass, c.fail, c.skip, c.invalid)
	}
	return total.status()
}

// paintWord colours the word of a verdict line of F9 6 when color is set
// (F9 8.3 item 4): FAIL and INVALID in red, SKIP in yellow, UPDATED in cyan.
func paintWord(line string, color bool) string {
	if !color {
		return line
	}
	for _, w := range []struct{ word, sgr string }{{"FAIL", sgrRed}, {"INVALID", sgrRed}, {"SKIP", sgrYellow}, {"UPDATED", sgrCyan}} {
		if rest, ok := strings.CutPrefix(line, "  "+w.word); ok && (rest == "" || rest[0] == ' ' || rest[0] == ':' || rest[0] == '\n') {
			return "  " + w.sgr + w.word + sgrReset + rest
		}
	}
	return line
}

// paintReport colours the verdict lines of a report.
func paintReport(rep string, color bool) string {
	if !color {
		return rep
	}
	lines := strings.SplitAfter(rep, "\n")
	for i, l := range lines {
		lines[i] = paintWord(l, true)
	}
	return strings.Join(lines, "")
}
