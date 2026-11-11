package syntax

import (
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

// F3 7.3: a text block is read in time linear in its source text. The test
// compares two sizes, so it does not depend on how fast the machine is: a
// lexer that looked at each earlier line again would take 64 times as long
// for 8 times the text.
func TestTextBlockIsLinear(t *testing.T) {
	block := func(lines int) string {
		return "\"\"\"\n" + strings.Repeat("  line of text\n", lines) + "  \"\"\""
	}
	timeOf := func(lines int) time.Duration {
		src := block(lines)
		best := time.Duration(1 << 62)
		for i := 0; i < 3; i++ {
			start := time.Now()
			parse(t, src)
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	n := 200000
	small, large := timeOf(n/8), timeOf(n)
	assert.True(t, large < 24*small+50*time.Millisecond, "%d lines took %v, %d lines took %v", n/8, small, n, large)
	prog := parse(t, block(n))
	s := prog.Result.(*String)
	assert.Equal(t, len(s.Value), n*len("line of text\n")-1)
	// With an interpolation on every line.
	src := "\"\"\"\n" + strings.Repeat("  a${1}\n", n/10) + "  \"\"\""
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
