package syntax

import (
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

// F3 7.3: a text block is read in time linear in its source text.
func TestTextBlockIsLinear(t *testing.T) {
	n := 200000
	src := "\"\"\"\n" + strings.Repeat("  line of text\n", n) + "  \"\"\""
	start := time.Now()
	prog := parse(t, src)
	assert.True(t, time.Since(start) < 2*time.Second, "took %v", time.Since(start))
	s := prog.Result.(*String)
	assert.Equal(t, len(s.Value), n*len("line of text\n")-1)
	// With an interpolation on every line.
	src = "\"\"\"\n" + strings.Repeat("  a${1}\n", n/10) + "  \"\"\""
	prog = parse(t, src)
	assert.Equal(t, len(prog.Result.(*Template).Parts), 2*(n/10)+1)
}

// F3 2: the forms, and what they read where no token starts.
func TestStringForms(t *testing.T) {
	checkDumps(t, [][2]string{
		{`r"a\nb"`, `"a\\nb"`},
		{`r'say "hi"'`, `"say \"hi\""`},
		{`"r\"x\""`, `"r\"x\""`},
		{"// r\"\n1", `1`},
		{"`r\"${1}\"`", `(template "r\"" 1 "\"")`},
		{"r\"\"\"\n  a\\b\n  \"\"\"", `"a\\b"`},
		{"\"\"\"\r\n  a\r  b\r\n  \"\"\"", `"a\nb"`},
	})
	checkErrors(t, []errCase{
		// A CR alone ends a block line but not a source line (F3 5.3 item 3).
		{"\"\"\"\r  a\r b\r  \"\"\"", "text-block-indent", [4]int{1, 10, 1, 11}},
	})
}
