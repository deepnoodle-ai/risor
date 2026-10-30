package compiler

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// tryWithLargeBody returns a try statement whose try block compiles to more
// than 65,535 instruction words, followed by the given handler clauses.
func tryWithLargeBody(handlers string) string {
	var sb strings.Builder
	sb.WriteString("let x = 0\ntry {\n")
	for i := 0; i < 20000; i++ {
		sb.WriteString("x = 1\n")
	}
	sb.WriteString("x = true\n} ")
	sb.WriteString(handlers)
	sb.WriteString("\n")
	return sb.String()
}

// The catch and finally offsets of PUSH_EXCEPT are uint16 operands. A try
// block too large for them must fail to compile instead of producing
// bytecode whose offsets have wrapped.
func TestTryBlockTooLarge(t *testing.T) {
	tests := []string{
		"finally { x = 2 }",
		"catch e { x = 2 }",
		"catch e { x = 2 } finally { x = 3 }",
	}
	for _, handlers := range tests {
		t.Run(handlers, func(t *testing.T) {
			parsed, err := parser.Parse(context.Background(), tryWithLargeBody(handlers), nil)
			assert.Nil(t, err)
			_, err = Compile(parsed, nil)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "try block too large")
		})
	}
}
