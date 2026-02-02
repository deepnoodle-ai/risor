// Package testing provides a testing framework for Risor scripts.
package testing

import (
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

// Status represents the outcome of a test.
type Status int

const (
	StatusPassed Status = iota
	StatusFailed
	StatusSkipped
	StatusError
)

// String returns the string representation of a Status.
func (s Status) String() string {
	switch s {
	case StatusPassed:
		return "PASS"
	case StatusFailed:
		return "FAIL"
	case StatusSkipped:
		return "SKIP"
	case StatusError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// AssertionError represents a failed assertion in a test.
type AssertionError struct {
	Message string        // Description of the failure
	File    string        // Source filename
	Line    int           // Line number where assertion failed
	Got     object.Object // Actual value (may be nil)
	Want    object.Object // Expected value (may be nil)
}

// TestResult holds the outcome of a single test function.
type TestResult struct {
	Name       string           // Test function name (e.g., "test_addition")
	Status     Status           // Pass, fail, skip, or error
	Duration   time.Duration    // How long the test took
	Failures   []AssertionError // Assertion failures
	Logs       []string         // Output from t.log()
	SkipReason string           // Why the test was skipped
	Error      error            // Error if Status == StatusError
}

// FileResult holds the results of all tests in a single file.
type FileResult struct {
	Filename   string        // Path to the test file
	Tests      []*TestResult // Results for each test function
	CompileErr error         // Error if file failed to compile
}

// Passed returns the number of passed tests in this file.
func (f *FileResult) Passed() int {
	count := 0
	for _, t := range f.Tests {
		if t.Status == StatusPassed {
			count++
		}
	}
	return count
}

// Failed returns the number of failed tests in this file.
func (f *FileResult) Failed() int {
	count := 0
	for _, t := range f.Tests {
		if t.Status == StatusFailed {
			count++
		}
	}
	return count
}

// Skipped returns the number of skipped tests in this file.
func (f *FileResult) Skipped() int {
	count := 0
	for _, t := range f.Tests {
		if t.Status == StatusSkipped {
			count++
		}
	}
	return count
}

// Errors returns the number of errored tests in this file.
func (f *FileResult) Errors() int {
	count := 0
	for _, t := range f.Tests {
		if t.Status == StatusError {
			count++
		}
	}
	return count
}

// Summary aggregates results across all test files.
type Summary struct {
	Files    []*FileResult // Results for each test file
	Passed   int           // Total passed tests
	Failed   int           // Total failed tests
	Skipped  int           // Total skipped tests
	Errors   int           // Total errored tests
	Duration time.Duration // Total time for all tests
}

// TotalTests returns the total number of tests run.
func (s *Summary) TotalTests() int {
	return s.Passed + s.Failed + s.Skipped + s.Errors
}

// Success returns true if all tests passed (no failures or errors).
func (s *Summary) Success() bool {
	return s.Failed == 0 && s.Errors == 0
}

// ComputeTotals recalculates the aggregate counts from all file results.
func (s *Summary) ComputeTotals() {
	s.Passed = 0
	s.Failed = 0
	s.Skipped = 0
	s.Errors = 0
	for _, f := range s.Files {
		s.Passed += f.Passed()
		s.Failed += f.Failed()
		s.Skipped += f.Skipped()
		s.Errors += f.Errors()
	}
}
