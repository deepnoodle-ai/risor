package parser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// To update golden files, set the environment variable:
//
//	UPDATE_GOLDEN=1 go test -run TestGolden ./parser/...
func updateGolden() bool {
	return os.Getenv("UPDATE_GOLDEN") == "1"
}

// TestGolden runs golden tests by comparing parser output against known-good files.
//
// Golden tests work as follows:
// 1. Read .risor files from testdata/golden/
// 2. Parse each file and get the AST's String() representation
// 3. Compare against corresponding .golden file
//
// To update golden files when the AST representation changes:
//
//	UPDATE_GOLDEN=1 go test -run TestGolden ./parser/...
func TestGolden(t *testing.T) {
	goldenDir := "testdata/golden"

	// Find all .risor files in the golden directory
	files, err := filepath.Glob(filepath.Join(goldenDir, "*.risor"))
	if err != nil {
		t.Fatalf("failed to glob golden files: %v", err)
	}

	if len(files) == 0 {
		t.Skip("no golden test files found")
	}

	for _, risorFile := range files {
		baseName := strings.TrimSuffix(filepath.Base(risorFile), ".risor")
		t.Run(baseName, func(t *testing.T) {
			// Read the input file
			input, err := os.ReadFile(risorFile)
			if err != nil {
				t.Fatalf("failed to read input file: %v", err)
			}

			// Parse the input
			program, err := Parse(context.Background(), string(input), &Config{Filename: risorFile})
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			// Get the AST string representation
			actual := program.String()

			// Golden file path
			goldenFile := strings.TrimSuffix(risorFile, ".risor") + ".golden"

			if updateGolden() {
				// Update the golden file
				err := os.WriteFile(goldenFile, []byte(actual), 0o644)
				if err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Logf("updated golden file: %s", goldenFile)
				return
			}

			// Read the expected output
			expected, err := os.ReadFile(goldenFile)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("golden file not found: %s\nRun with UPDATE_GOLDEN=1 to create it.\nActual output:\n%s", goldenFile, actual)
				}
				t.Fatalf("failed to read golden file: %v", err)
			}

			// Compare
			assert.Equal(t, string(expected), actual)
		})
	}
}

// TestGoldenErrors tests parsing files that should produce errors.
// These files are in testdata/golden/errors/ and their .golden files
// contain the expected error messages.
func TestGoldenErrors(t *testing.T) {
	goldenDir := "testdata/golden/errors"

	// Find all .risor files in the errors directory
	files, err := filepath.Glob(filepath.Join(goldenDir, "*.risor"))
	if err != nil {
		t.Fatalf("failed to glob golden error files: %v", err)
	}

	if len(files) == 0 {
		t.Skip("no golden error test files found")
	}

	for _, risorFile := range files {
		baseName := strings.TrimSuffix(filepath.Base(risorFile), ".risor")
		t.Run(baseName, func(t *testing.T) {
			// Read the input file
			input, err := os.ReadFile(risorFile)
			if err != nil {
				t.Fatalf("failed to read input file: %v", err)
			}

			// Parse the input - expect an error
			_, parseErr := Parse(context.Background(), string(input), &Config{Filename: risorFile})
			if parseErr == nil {
				t.Fatalf("expected parse error, but parsing succeeded")
			}

			// Get the error message
			actual := parseErr.Error()

			// Golden file path
			goldenFile := strings.TrimSuffix(risorFile, ".risor") + ".golden"

			if updateGolden() {
				// Update the golden file
				err := os.WriteFile(goldenFile, []byte(actual), 0o644)
				if err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Logf("updated golden file: %s", goldenFile)
				return
			}

			// Read the expected output
			expected, err := os.ReadFile(goldenFile)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("golden file not found: %s\nRun with UPDATE_GOLDEN=1 to create it.\nActual error:\n%s", goldenFile, actual)
				}
				t.Fatalf("failed to read golden file: %v", err)
			}

			// Compare
			assert.Equal(t, string(expected), actual)
		})
	}
}
