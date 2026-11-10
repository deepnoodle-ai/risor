package ast

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var formValues = []string{
	"", "a", "a\nb", "a\n", "\n", "\n\n", "  a\n    b", "\ta\n b", "a \n  ", " ",
	"say \"\"\"hi\"\"\"", `C:\new`, "${x}", "it's", "x\ry", "\u0000\u001f", "😀\n e\u0301",
	"`tick`", "\"", "''", "a\\", "line\nbreak\n\n  end  ",
}

// Every form of a string prints as text that holds the value: a raw string
// only where it can, a text block always. Without an implementation the
// test checks the text's shape; with RISOR_TS set to the ts directory, it
// asks that implementation whether each form equals the one-line string.
func TestStringForms(t *testing.T) {
	var cases []string
	for i, v := range formValues {
		for _, form := range []Str{
			{Val: v, Quote: '"', Raw: true}, {Val: v, Quote: '\'', Raw: true},
			{Val: v, Block: 1}, {Val: v, Block: 1, Indent: "  "}, {Val: v, Block: 1, Indent: "\t"},
			{Val: v, Block: 2, Indent: "  "}, {Val: v, Block: 2},
		} {
			text := StrText(&form)
			if form.Block != 0 && strings.HasPrefix(text, `r"""`) != (form.Block == 2) && rawBlockOK(v) {
				t.Errorf("%q: block form %d printed as %q", v, form.Block, text)
			}
			src := fmt.Sprintf("%s == %s", text, QuoteString(v, '"', 0))
			j, _ := json.Marshal(map[string]any{"name": fmt.Sprintf("f%d-%d", i, len(cases)), "source": src})
			cases = append(cases, string(j))
		}
		tp := &Template{Block: true, Indent: "  ", Parts: []Part{{Text: v}, {X: &Str{Val: v, Block: 1, Indent: "    "}}, {Text: v}}}
		src := fmt.Sprintf("%s == %s", PrintExpr(tp), QuoteString(v+v+v, '"', 0))
		j, _ := json.Marshal(map[string]any{"name": fmt.Sprintf("t%d", i), "source": src})
		cases = append(cases, string(j))
	}
	dir := os.Getenv("RISOR_TS")
	if dir == "" {
		return
	}
	file := filepath.Join(t.TempDir(), "forms.json")
	if err := os.WriteFile(file, []byte("["+strings.Join(cases, ",\n")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "scripts/outcome.ts", file)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("%d lines for %d cases", len(lines), len(cases))
	}
	for i, line := range lines {
		if !strings.Contains(line, `"value":true`) {
			t.Errorf("%s\n  gave %s", cases[i], line)
		}
	}
}
