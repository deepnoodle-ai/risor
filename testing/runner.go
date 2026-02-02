package testing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/bytecode"
	"github.com/deepnoodle-ai/risor/v2/object"
	"github.com/deepnoodle-ai/risor/v2/vm"
)

// Config holds configuration for running tests.
type Config struct {
	// Patterns specifies files or directories to search for tests.
	// Default is current directory.
	Patterns []string

	// RunPattern filters tests to run by name regex.
	RunPattern string

	// Verbose enables verbose output (shows t.log() messages).
	Verbose bool
}

// DiscoverTestFiles finds all *_test.risor files matching the given patterns.
// If no patterns are provided, searches the current directory.
func DiscoverTestFiles(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	var files []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		// Check if it's a glob pattern
		if strings.Contains(pattern, "*") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
			}
			for _, m := range matches {
				if isTestFile(m) && !seen[m] {
					files = append(files, m)
					seen[m] = true
				}
			}
			continue
		}

		// Handle "..." suffix for recursive search
		recursive := false
		searchDir := pattern
		if strings.HasSuffix(pattern, "/...") || strings.HasSuffix(pattern, "...") {
			recursive = true
			searchDir = strings.TrimSuffix(strings.TrimSuffix(pattern, "..."), "/")
			if searchDir == "" {
				searchDir = "."
			}
		}

		// Check if it's a directory
		info, err := os.Stat(searchDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("path not found: %s", searchDir)
			}
			return nil, err
		}

		if info.IsDir() {
			if recursive {
				err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && isTestFile(path) && !seen[path] {
						files = append(files, path)
						seen[path] = true
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
			} else {
				entries, err := os.ReadDir(searchDir)
				if err != nil {
					return nil, err
				}
				for _, e := range entries {
					if !e.IsDir() && isTestFile(e.Name()) {
						path := filepath.Join(searchDir, e.Name())
						if !seen[path] {
							files = append(files, path)
							seen[path] = true
						}
					}
				}
			}
		} else {
			// It's a file
			if isTestFile(pattern) && !seen[pattern] {
				files = append(files, pattern)
				seen[pattern] = true
			}
		}
	}

	return files, nil
}

// isTestFile returns true if the filename matches *_test.risor.
func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.risor")
}

// DiscoverTestFunctions finds all test_* functions in compiled code.
func DiscoverTestFunctions(code *bytecode.Code) []string {
	var tests []string
	names := code.FunctionNames()
	for _, name := range names {
		if strings.HasPrefix(name, "test_") {
			tests = append(tests, name)
		}
	}
	return tests
}

// Run executes tests according to the given configuration.
func Run(ctx context.Context, cfg *Config) (*Summary, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// Discover test files
	files, err := DiscoverTestFiles(cfg.Patterns)
	if err != nil {
		return nil, err
	}

	// Compile run pattern if provided
	var runRe *regexp.Regexp
	if cfg.RunPattern != "" {
		runRe, err = regexp.Compile(cfg.RunPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid run pattern: %w", err)
		}
	}

	summary := &Summary{}
	start := time.Now()

	// Process each test file
	for _, file := range files {
		fileResult := runTestFile(ctx, file, runRe)
		summary.Files = append(summary.Files, fileResult)
	}

	summary.Duration = time.Since(start)
	summary.ComputeTotals()

	return summary, nil
}

// runTestFile executes all tests in a single file.
func runTestFile(ctx context.Context, filename string, runRe *regexp.Regexp) *FileResult {
	result := &FileResult{Filename: filename}

	// Read the source
	source, err := os.ReadFile(filename)
	if err != nil {
		result.CompileErr = err
		return result
	}

	// Compile with standard builtins
	env := risor.Builtins()
	code, err := risor.Compile(ctx, string(source),
		risor.WithFilename(filename),
		risor.WithEnv(env),
	)
	if err != nil {
		result.CompileErr = err
		return result
	}

	// Find test functions
	testNames := DiscoverTestFunctions(code)

	// Filter by run pattern
	if runRe != nil {
		var filtered []string
		for _, name := range testNames {
			if runRe.MatchString(name) {
				filtered = append(filtered, name)
			}
		}
		testNames = filtered
	}

	// Run each test
	for _, name := range testNames {
		testResult := runSingleTest(ctx, code, env, filename, name)
		result.Tests = append(result.Tests, testResult)
	}

	return result
}

// runSingleTest executes a single test function.
func runSingleTest(ctx context.Context, code *bytecode.Code, env map[string]any, filename, testName string) *TestResult {
	result := &TestResult{Name: testName}
	start := time.Now()

	// Create a fresh VM for this test
	machine, err := vm.New(code, vm.WithGlobals(env))
	if err != nil {
		result.Status = StatusError
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Run the file to populate globals (functions, variables, etc.)
	if err := machine.Run(ctx); err != nil {
		result.Status = StatusError
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Get the test function
	testFn, err := machine.Get(testName)
	if err != nil {
		result.Status = StatusError
		result.Error = fmt.Errorf("test function %q not found: %w", testName, err)
		result.Duration = time.Since(start)
		return result
	}

	closure, ok := testFn.(*object.Closure)
	if !ok {
		result.Status = StatusError
		result.Error = fmt.Errorf("test function %q is not a function (got %s)", testName, testFn.Type())
		result.Duration = time.Since(start)
		return result
	}

	// Create the test context
	testCtx := NewTestContext(testName, filename)

	// Call the test function with the test context
	_, err = machine.Call(ctx, closure, []object.Object{testCtx})
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = StatusError
		result.Error = err
		return result
	}

	// Collect results from the test context
	result.Logs = testCtx.Logs()
	result.Failures = testCtx.Failures()

	if testCtx.Skipped() {
		result.Status = StatusSkipped
		result.SkipReason = testCtx.SkipReason()
	} else if testCtx.Failed() {
		result.Status = StatusFailed
	} else {
		result.Status = StatusPassed
	}

	return result
}
