package tests

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/risor/v2/vm"
)

type TestCase struct {
	Name              string
	Text              string
	ExpectedValue     string
	ExpectedType      string
	ExpectedErr       string
	ExpectedErrLine   int
	ExpectedErrColumn int
}

func readFile(name string) string {
	data, err := os.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func parseExpectedValue(filename, text string) (TestCase, error) {
	result := TestCase{}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "// ") {
			continue
		}
		line = strings.TrimPrefix(line, "// ")
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "expected value":
			result.ExpectedValue = val
		case "expected type":
			result.ExpectedType = val
		case "expected error":
			result.ExpectedErr = val
		case "expected error line":
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return result, err
			}
			result.ExpectedErrLine = intVal
		case "expected error column":
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return result, err
			}
			result.ExpectedErrColumn = intVal
		}
	}
	return result, nil
}

func getTestCase(name string) (TestCase, error) {
	input := readFile(name)
	testCase, err := parseExpectedValue(name, input)
	testCase.Name = name
	testCase.Text = input
	return testCase, err
}

// execute runs the input and returns the result as an object.Object
// This uses the internal VM to get the raw object for test comparison
func execute(ctx context.Context, input string) (object.Object, error) {
	// Use the internal vm.Run to get object.Object for accurate test comparison
	code, err := risor.Compile(ctx, input, risor.WithEnv(risor.Builtins()))
	if err != nil {
		return nil, err
	}
	// Use vm.Run directly to get object.Object
	return vm.Run(ctx, code, vm.WithGlobals(risor.Builtins()))
}

func listTestFiles() []string {
	files, err := os.ReadDir(".")
	if err != nil {
		panic(err)
	}
	var testFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".tm") {
			testFiles = append(testFiles, f.Name())
		}
	}
	return testFiles
}

func TestFiles(t *testing.T) {
	only := "" // test-2022-12-03-08-12
	for _, name := range listTestFiles() {
		if !strings.HasSuffix(name, ".tm") {
			continue
		}
		if only != "" && !strings.Contains(name, only) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			tc, err := getTestCase(name)
			assert.Nil(t, err)
			ctx := context.Background()
			result, err := execute(ctx, tc.Text)
			expectedType := object.Type(tc.ExpectedType)

			if tc.ExpectedValue != "" {
				if result == nil {
					t.Fatalf("expected value %q, got nil", tc.ExpectedValue)
				} else {
					assert.Equal(t, result.Inspect(), tc.ExpectedValue)
				}
			}
			if tc.ExpectedType != "" {
				if result == nil {
					t.Fatalf("expected type %q, got nil", tc.ExpectedType)
				} else {
					assert.Equal(t, result.Type(), expectedType)
				}
			}
			if tc.ExpectedErr != "" {
				assert.NotNil(t, err)
				assert.Equal(t, err.Error(), tc.ExpectedErr)
			}
			if tc.ExpectedErrColumn != 0 {
				assert.NotNil(t, err)
				parserErr, ok := err.(parser.ParserError)
				assert.True(t, ok)
				fmt.Println("--- Friendly error output for", name)
				fmt.Println(parserErr.FriendlyErrorMessage())
				fmt.Println("---")
				assert.Equal(t,

					parserErr.StartPosition().ColumnNumber(), tc.ExpectedErrColumn,

					"The column number is incorrect")
			}
			if tc.ExpectedErrLine != 0 {
				assert.NotNil(t, err)
				parserErr, ok := err.(parser.ParserError)
				assert.True(t, ok)
				fmt.Println("--- Friendly error output for", name)
				fmt.Println(parserErr.FriendlyErrorMessage())
				fmt.Println("---")
				assert.Equal(t,

					parserErr.StartPosition().LineNumber(), tc.ExpectedErrLine,

					"The line number is incorrect")
			}
		})
	}
}
