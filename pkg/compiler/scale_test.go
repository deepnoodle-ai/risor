package compiler

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// manyFunctionsSource returns a script that defines n top-level functions.
func manyFunctionsSource(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "function f%d(a, b) {\n    let x = a + b * %d\n    if (x > 10) { return x - 1 }\n    return x\n}\n", i, i)
	}
	return sb.String()
}

func parseManyFunctions(tb testing.TB, n int) (*ast.Program, string) {
	tb.Helper()
	src := manyFunctionsSource(n)
	prog, err := parser.Parse(context.Background(), src, nil)
	assert.Nil(tb, err)
	return prog, src
}

// fastestCompile returns the fastest of several compiles of prog, to reduce
// noise from the scheduler and the garbage collector.
func fastestCompile(t *testing.T, prog *ast.Program, src string) time.Duration {
	t.Helper()
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 3; i++ {
		start := time.Now()
		_, err := Compile(prog, &Config{Source: src})
		assert.Nil(t, err)
		best = min(best, time.Since(start))
	}
	return best
}

// TestCompileTimeLinearInFunctionCount guards against compile time that grows
// with the square of the number of functions. Eight times the functions must
// take well under 64 times as long. It compares two sizes on the same machine
// rather than checking an absolute time.
func TestCompileTimeLinearInFunctionCount(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	const small, large = 1000, 8000
	smallProg, smallSrc := parseManyFunctions(t, small)
	largeProg, largeSrc := parseManyFunctions(t, large)

	smallTime := fastestCompile(t, smallProg, smallSrc)
	largeTime := fastestCompile(t, largeProg, largeSrc)

	ratio := float64(largeTime) / float64(smallTime)
	t.Logf("%d functions: %v, %d functions: %v, ratio %.1f", small, smallTime, large, largeTime, ratio)
	assert.True(t, ratio < 24, "compile time grew %.1fx for %dx the functions", ratio, large/small)
}

func BenchmarkCompileManyFunctions(b *testing.B) {
	for _, n := range []int{1000, 4000, 16000} {
		b.Run(fmt.Sprintf("functions=%d", n), func(b *testing.B) {
			prog, src := parseManyFunctions(b, n)
			b.SetBytes(int64(len(src)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Compile(prog, &Config{Source: src}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
