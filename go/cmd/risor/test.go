package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/risorcli"
	"github.com/deepnoodle-ai/wonton/cli"
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

// testFlagDefs are the flags of test.
func testFlagDefs() []cli.Flag {
	return []cli.Flag{
		cli.Strings("run").Placeholder("TEXT").Help("Run only the cases whose names contain TEXT; repeatable"),
		cli.Bool("update").Help("Update mode: write each case's actual result as its expect"),
		formatFlag(),
		cli.String("suite").Placeholder("DIR").ValidateWith(nonEmpty).Help("Run the conformance suite at DIR"),
		colorFlagDef(),
	}
}

// testFlagsOf reads the arguments of test (F9 8.1).
func testFlagsOf(ctx *cli.Context) (*testFlags, error) {
	f := &testFlags{
		paths:  ctx.Args(),
		runs:   ctx.Strings("run"),
		update: ctx.Bool("update"),
		format: ctx.String("format"),
		suite:  ctx.String("suite"),
		color:  colorOf(ctx),
	}
	if f.suite != "" && (len(f.paths) > 0 || len(f.runs) > 0 || f.update) {
		return nil, cmdErrorf("--suite takes no PATH, --run or --update")
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

func (c *counts) add(r risorcli.CaseResult) {
	c.cases++
	switch r.Status {
	case "updated":
		c.updated++
	case "pass":
		c.pass++
	case "fail":
		c.fail++
	case "skip":
		c.skip++
	case "invalid":
		c.invalid++
	}
}

func testCommand(cc *cli.Context) int {
	stdin, stdout, stderr := cc.Stdin(), cc.Stdout(), cc.Stderr()
	f, err := testFlagsOf(cc)
	if err != nil {
		return commandLineError(stderr, err)
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
	color := f.format == "text" && f.color.on(stdout, stdout)
	for _, path := range files {
		c.files++
		out, err := runCaseFile(path, f.update, selected, stdin)
		if err != nil {
			c.badFiles++
			reason := err.Error()
			if f.format == "json" {
				fmt.Fprintln(stdout, jsonLine(map[string]any{"file": path, "message": reason, "status": "invalid"}))
			} else {
				fmt.Fprintf(stdout, "%s\n%s\n", path, paintWord("  INVALID", color))
				for _, l := range strings.Split(reason, "\n") {
					fmt.Fprintf(stdout, "    %s\n", risorcli.Visible(l))
				}
			}
			continue
		}
		for _, r := range out.Results {
			c.add(r)
		}
		if f.format == "json" {
			for _, r := range out.Results {
				fmt.Fprintln(stdout, caseLine(path, r))
			}
		} else if out.Report != "" {
			fmt.Fprintf(stdout, "%s\n%s", path, paintReport(out.Report, color))
		}
		if f.update && path != "-" {
			if out.UpdateErr != nil {
				fmt.Fprintf(stderr, "risor test: %s: %v\n", path, out.UpdateErr)
				c.missing = true
			} else if out.Written {
				if err := os.WriteFile(path, out.UpdatedText, 0o644); err != nil {
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

// runCaseFile reads and runs a case file: a program case file when its name
// ends in .test.risor, else JSON (F9 8.2 item 1).
func runCaseFile(path string, update bool, selected func(string) bool, stdin io.Reader) (risorcli.CaseFileOutcome, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return risorcli.CaseFileOutcome{}, err
	}
	return risorcli.RunCaseFile(path, data, update, selected)
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
func caseLine(file string, r risorcli.CaseResult) string {
	m := map[string]any{"file": file, "name": r.Case, "status": r.Status}
	switch r.Status {
	case "updated":
	case "fail":
		ds := make([]any, len(r.Diffs))
		for i, d := range r.Diffs {
			ds[i] = d
		}
		m["differs"] = ds
	case "invalid":
		if r.Fixture != nil {
			m["fixture"] = r.Fixture
			m["message"] = r.Message
		} else {
			m["message"] = r.Reason
		}
	case "skip":
		m["reason"] = r.Reason
	}
	return jsonLine(m)
}

func jsonLine(m map[string]any) string {
	s, _ := risor.JSONText(m)
	return s
}

// suiteCommand is risor test --suite (F9 8.6).
func suiteCommand(f *testFlags, stdout, stderr io.Writer) int {
	var total counts
	for _, sec := range risorcli.RunSuite(f.suite) {
		if sec.Err != nil {
			fmt.Fprintf(stderr, "risor test: %s: %v\n", filepath.Join(f.suite, sec.Rel), sec.Err)
			total.missing = true
			continue
		}
		var c counts
		for _, r := range sec.Results {
			c.add(r)
			total.add(r)
		}
		if f.format == "json" {
			for _, r := range sec.Results {
				fmt.Fprintln(stdout, caseLine(r.File, r))
			}
			continue
		}
		fmt.Fprintf(stdout, "%s: %d pass, %d fail, %d skip, %d invalid\n", sec.Rel, c.pass, c.fail, c.skip, c.invalid)
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
