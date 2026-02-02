package risor_test

import (
	"context"
	"log"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/risor/v2/vm"
)

func BenchmarkRisor_Fibonacci35(b *testing.B) {
	script := `
    func fibonacci(n) {
        if (n <= 1) {
            return n
        }
        return fibonacci(n-1) + fibonacci(n-2)
    }
    fibonacci(35)
    `

	ctx := context.Background()

	ast, err := parser.Parse(ctx, script, nil)
	if err != nil {
		log.Fatal(err)
	}

	code, err := compiler.Compile(ast, nil)
	if err != nil {
		log.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := vm.Run(ctx, code)
		if err != nil {
			b.Fatal(err)
		}
		if result.Interface().(int64) != 9227465 {
			b.Fatalf("unexpected result: %v", result)
		}
	}
}
