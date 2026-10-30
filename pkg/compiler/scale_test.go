package compiler

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

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

// compileBytes returns the fewest bytes allocated by several compiles of prog.
// Bytes allocated measure the work done without depending on machine load,
// and taking the fewest guards against allocations by other goroutines.
func compileBytes(t *testing.T, prog *ast.Program, src string) uint64 {
	t.Helper()
	best := uint64(1<<64 - 1)
	var before, after runtime.MemStats
	for i := 0; i < 3; i++ {
		runtime.ReadMemStats(&before)
		_, err := Compile(prog, &Config{Source: src})
		runtime.ReadMemStats(&after)
		assert.Nil(t, err)
		best = min(best, after.TotalAlloc-before.TotalAlloc)
	}
	return best
}

// TestCompileWorkLinearInFunctionCount guards against compile work that grows
// with the square of the number of functions. It counts bytes allocated rather
// than time, so machine load does not affect it. Eight times the functions
// must allocate well under 64 times as much. Both quadratic paths fixed in
// #531 exceed the bound at these sizes: splitting the source for each
// instruction, and building the global names for each function. It does not
// see quadratic work that does not allocate; BenchmarkCompileManyFunctions
// measures time.
func TestCompileWorkLinearInFunctionCount(t *testing.T) {
	const small, large = 100, 800
	smallProg, smallSrc := parseManyFunctions(t, small)
	largeProg, largeSrc := parseManyFunctions(t, large)

	smallBytes := compileBytes(t, smallProg, smallSrc)
	largeBytes := compileBytes(t, largeProg, largeSrc)

	ratio := float64(largeBytes) / float64(smallBytes)
	t.Logf("%d functions: %d bytes, %d functions: %d bytes, ratio %.1f", small, smallBytes, large, largeBytes, ratio)
	// Linear code gives 8.4. A higher bound would miss smaller quadratic
	// terms: one of 8n² bytes already gives about 11.6.
	assert.True(t, ratio < 12, "compile allocations grew %.1fx for %dx the functions", ratio, large/small)
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

// TestToBytecodeOnChildSetsGlobalNames checks that converting a child code
// gives the result the global names, because the result is a root bytecode.
func TestToBytecodeOnChildSetsGlobalNames(t *testing.T) {
	input := `let total = 0
function add(a, b) { return a + b + total }
add(1, 2)`

	c, err := New(nil)
	assert.Nil(t, err)
	program, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	code, err := c.CompileAST(program)
	assert.Nil(t, err)

	var child *Code
	for _, fc := range code.Flatten() {
		if !fc.IsRoot() {
			child = fc
			break
		}
	}
	assert.NotNil(t, child)

	bc := child.ToBytecode()
	assert.True(t, bc.Root() == bc)
	assert.Equal(t, bc.GlobalNames(), code.GlobalNames())
	assert.Equal(t, bc.GlobalNameCount(), code.GlobalsCount())
	assert.True(t, bc.GlobalNameCount() > 0)
}
