package testing

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	stdt "testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/object"
)

func TestStatus_String(t *stdt.T) {
	assert.Equal(t, StatusPassed.String(), "PASS")
	assert.Equal(t, StatusFailed.String(), "FAIL")
	assert.Equal(t, StatusSkipped.String(), "SKIP")
	assert.Equal(t, StatusError.String(), "ERROR")
}

func TestTestContext_Name(t *stdt.T) {
	tc := NewTestContext("test_example", "example_test.risor")
	assert.Equal(t, tc.Name(), "test_example")

	// Check name via GetAttr
	nameAttr, ok := tc.GetAttr("name")
	assert.True(t, ok)
	assert.Equal(t, nameAttr.(*object.String).Value(), "test_example")
}

func TestTestContext_Assert(t *stdt.T) {
	ctx := context.Background()

	t.Run("passes on truthy", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertFn, _ := tc.GetAttr("assert")
		builtin := assertFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.True)
		assert.Nil(t, err)
		assert.False(t, tc.Failed())
	})

	t.Run("fails on falsy", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertFn, _ := tc.GetAttr("assert")
		builtin := assertFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.False)
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
		assert.Equal(t, len(tc.Failures()), 1)
		assert.Equal(t, tc.Failures()[0].Message, "assertion failed")
	})

	t.Run("custom message", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertFn, _ := tc.GetAttr("assert")
		builtin := assertFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.False, object.NewString("custom msg"))
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
		assert.Equal(t, tc.Failures()[0].Message, "custom msg")
	})
}

func TestTestContext_AssertEq(t *stdt.T) {
	ctx := context.Background()

	t.Run("passes on equal", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertEqFn, _ := tc.GetAttr("assert_eq")
		builtin := assertEqFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewInt(5), object.NewInt(5))
		assert.Nil(t, err)
		assert.False(t, tc.Failed())
	})

	t.Run("fails on not equal", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertEqFn, _ := tc.GetAttr("assert_eq")
		builtin := assertEqFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewInt(4), object.NewInt(5))
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
		assert.Equal(t, len(tc.Failures()), 1)
		failure := tc.Failures()[0]
		assert.Equal(t, failure.Message, "values are not equal")
		assert.Equal(t, failure.Got.(*object.Int).Value(), int64(4))
		assert.Equal(t, failure.Want.(*object.Int).Value(), int64(5))
	})
}

func TestTestContext_AssertNe(t *stdt.T) {
	ctx := context.Background()

	t.Run("passes on not equal", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertNeFn, _ := tc.GetAttr("assert_ne")
		builtin := assertNeFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewInt(4), object.NewInt(5))
		assert.Nil(t, err)
		assert.False(t, tc.Failed())
	})

	t.Run("fails on equal", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertNeFn, _ := tc.GetAttr("assert_ne")
		builtin := assertNeFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewInt(5), object.NewInt(5))
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
		assert.Equal(t, tc.Failures()[0].Message, "values should not be equal")
	})
}

func TestTestContext_AssertNil(t *stdt.T) {
	ctx := context.Background()

	t.Run("passes on nil", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertNilFn, _ := tc.GetAttr("assert_nil")
		builtin := assertNilFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.Nil)
		assert.Nil(t, err)
		assert.False(t, tc.Failed())
	})

	t.Run("fails on non-nil", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertNilFn, _ := tc.GetAttr("assert_nil")
		builtin := assertNilFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewInt(42))
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
	})
}

func TestTestContext_AssertError(t *stdt.T) {
	ctx := context.Background()

	t.Run("passes on error", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertErrFn, _ := tc.GetAttr("assert_error")
		builtin := assertErrFn.(*object.Builtin)

		errObj := object.NewError(object.TypeErrorf("some error"))
		_, err := builtin.Call(ctx, errObj)
		assert.Nil(t, err)
		assert.False(t, tc.Failed())
	})

	t.Run("fails on non-error", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		assertErrFn, _ := tc.GetAttr("assert_error")
		builtin := assertErrFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewString("not an error"))
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
	})
}

func TestTestContext_Skip(t *stdt.T) {
	ctx := context.Background()

	t.Run("marks test as skipped", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		skipFn, _ := tc.GetAttr("skip")
		builtin := skipFn.(*object.Builtin)

		_, err := builtin.Call(ctx)
		assert.Nil(t, err)
		assert.True(t, tc.Skipped())
		assert.Equal(t, tc.SkipReason(), "")
	})

	t.Run("with reason", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		skipFn, _ := tc.GetAttr("skip")
		builtin := skipFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewString("not implemented"))
		assert.Nil(t, err)
		assert.True(t, tc.Skipped())
		assert.Equal(t, tc.SkipReason(), "not implemented")
	})
}

func TestTestContext_Fail(t *stdt.T) {
	ctx := context.Background()

	t.Run("marks test as failed", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		failFn, _ := tc.GetAttr("fail")
		builtin := failFn.(*object.Builtin)

		_, err := builtin.Call(ctx)
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
		assert.Equal(t, tc.Failures()[0].Message, "test failed")
	})

	t.Run("with message", func(t *stdt.T) {
		tc := NewTestContext("test", "test.risor")
		failFn, _ := tc.GetAttr("fail")
		builtin := failFn.(*object.Builtin)

		_, err := builtin.Call(ctx, object.NewString("custom failure"))
		assert.Nil(t, err)
		assert.True(t, tc.Failed())
		assert.Equal(t, tc.Failures()[0].Message, "custom failure")
	})
}

func TestTestContext_Log(t *stdt.T) {
	ctx := context.Background()
	tc := NewTestContext("test", "test.risor")
	logFn, _ := tc.GetAttr("log")
	builtin := logFn.(*object.Builtin)

	_, err := builtin.Call(ctx, object.NewString("hello"), object.NewInt(42))
	assert.Nil(t, err)
	assert.Equal(t, len(tc.Logs()), 1)
	assert.Equal(t, tc.Logs()[0], "hello 42")
}

func TestDiscoverTestFiles(t *stdt.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Create some test files
	assert.Nil(t, os.WriteFile(filepath.Join(tmpDir, "calc_test.risor"), []byte(""), 0o644))
	assert.Nil(t, os.WriteFile(filepath.Join(tmpDir, "util_test.risor"), []byte(""), 0o644))
	assert.Nil(t, os.WriteFile(filepath.Join(tmpDir, "helper.risor"), []byte(""), 0o644))

	// Create a subdirectory with more tests
	subDir := filepath.Join(tmpDir, "sub")
	assert.Nil(t, os.Mkdir(subDir, 0o755))
	assert.Nil(t, os.WriteFile(filepath.Join(subDir, "sub_test.risor"), []byte(""), 0o644))

	t.Run("discovers in directory", func(t *stdt.T) {
		files, err := DiscoverTestFiles([]string{tmpDir})
		assert.Nil(t, err)
		assert.Equal(t, len(files), 2) // calc_test.risor and util_test.risor
	})

	t.Run("recursive with ...", func(t *stdt.T) {
		files, err := DiscoverTestFiles([]string{tmpDir + "/..."})
		assert.Nil(t, err)
		assert.Equal(t, len(files), 3) // Includes sub/sub_test.risor
	})

	t.Run("specific file", func(t *stdt.T) {
		files, err := DiscoverTestFiles([]string{filepath.Join(tmpDir, "calc_test.risor")})
		assert.Nil(t, err)
		assert.Equal(t, len(files), 1)
	})

	t.Run("ignores non-test files", func(t *stdt.T) {
		files, err := DiscoverTestFiles([]string{filepath.Join(tmpDir, "helper.risor")})
		assert.Nil(t, err)
		assert.Equal(t, len(files), 0)
	})
}

func TestOutput_StartTest(t *stdt.T) {
	var buf bytes.Buffer
	output := NewOutput(OutputConfig{Writer: &buf})

	output.StartTest("test_addition")
	assert.Equal(t, buf.String(), "=== RUN   test_addition\n")
}

func TestOutput_EndTest(t *stdt.T) {
	t.Run("passed test", func(t *stdt.T) {
		var buf bytes.Buffer
		output := NewOutput(OutputConfig{Writer: &buf})

		result := &TestResult{
			Name:   "test_addition",
			Status: StatusPassed,
		}
		output.EndTest(result)

		assert.Contains(t, buf.String(), "PASS")
		assert.Contains(t, buf.String(), "test_addition")
	})

	t.Run("failed test with failures", func(t *stdt.T) {
		var buf bytes.Buffer
		output := NewOutput(OutputConfig{Writer: &buf})

		result := &TestResult{
			Name:   "test_fail",
			Status: StatusFailed,
			Failures: []AssertionError{
				{Message: "values are not equal", Got: object.NewInt(4), Want: object.NewInt(5)},
			},
		}
		output.EndTest(result)

		assert.Contains(t, buf.String(), "FAIL")
		assert.Contains(t, buf.String(), "values are not equal")
		assert.Contains(t, buf.String(), "got")
		assert.Contains(t, buf.String(), "want")
	})

	t.Run("skipped test", func(t *stdt.T) {
		var buf bytes.Buffer
		output := NewOutput(OutputConfig{Writer: &buf})

		result := &TestResult{
			Name:       "test_skip",
			Status:     StatusSkipped,
			SkipReason: "not implemented",
		}
		output.EndTest(result)

		assert.Contains(t, buf.String(), "SKIP")
		assert.Contains(t, buf.String(), "not implemented")
	})
}

func TestSummary(t *stdt.T) {
	summary := &Summary{
		Files: []*FileResult{
			{
				Tests: []*TestResult{
					{Status: StatusPassed},
					{Status: StatusPassed},
					{Status: StatusFailed},
				},
			},
			{
				Tests: []*TestResult{
					{Status: StatusSkipped},
				},
			},
		},
	}

	summary.ComputeTotals()

	assert.Equal(t, summary.Passed, 2)
	assert.Equal(t, summary.Failed, 1)
	assert.Equal(t, summary.Skipped, 1)
	assert.Equal(t, summary.Errors, 0)
	assert.Equal(t, summary.TotalTests(), 4)
	assert.False(t, summary.Success())
}

func TestRun_Integration(t *stdt.T) {
	// Create a temp directory with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "math_test.risor")

	source := `
function add(a, b) {
    return a + b
}

function test_addition(t) {
    t.assert_eq(add(2, 3), 5)
}

function test_subtraction(t) {
    t.assert_eq(5 - 3, 2)
}

function test_skip_example(t) {
    t.skip("not ready")
}

function test_failure(t) {
    t.assert_eq(1, 2)
}
`
	assert.Nil(t, os.WriteFile(testFile, []byte(source), 0o644))

	ctx := context.Background()
	summary, err := Run(ctx, &Config{Patterns: []string{tmpDir}})
	assert.Nil(t, err)

	assert.Equal(t, len(summary.Files), 1)
	assert.Equal(t, summary.Passed, 2)  // test_addition and test_subtraction
	assert.Equal(t, summary.Skipped, 1) // test_skip_example
	assert.Equal(t, summary.Failed, 1)  // test_failure
	assert.False(t, summary.Success())
}

func TestRun_RunPattern(t *stdt.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "filter_test.risor")

	source := `
function test_alpha(t) {
    t.assert(true)
}

function test_beta(t) {
    t.assert(true)
}

function test_gamma(t) {
    t.assert(true)
}
`
	assert.Nil(t, os.WriteFile(testFile, []byte(source), 0o644))

	ctx := context.Background()
	summary, err := Run(ctx, &Config{
		Patterns:   []string{tmpDir},
		RunPattern: "alpha|gamma",
	})
	assert.Nil(t, err)

	assert.Equal(t, summary.TotalTests(), 2)
	assert.Equal(t, summary.Passed, 2)
}
