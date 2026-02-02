package testing

import (
	"context"
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/object"
	"github.com/deepnoodle-ai/risor/v2/op"
)

// Ensure context package is available for the builtin wrapper signatures
var _ context.Context

// TestContext is the "t" object passed to test functions.
// It provides assertion methods and test control via GetAttr().
type TestContext struct {
	name       string                   // Test name
	failed     bool                     // Whether the test has failed
	skipped    bool                     // Whether the test was skipped
	skipReason string                   // Reason for skipping
	logs       []string                 // Messages from t.log()
	failures   []AssertionError         // Assertion failures
	filename   string                   // Source file for error reporting
	attrs      map[string]object.Object // Cached method wrappers
}

// NewTestContext creates a new TestContext for a test function.
func NewTestContext(name, filename string) *TestContext {
	t := &TestContext{
		name:     name,
		filename: filename,
	}
	t.initAttrs()
	return t
}

// initAttrs creates the builtin method wrappers.
func (t *TestContext) initAttrs() {
	t.attrs = map[string]object.Object{
		"name": object.NewString(t.name),
		"assert": object.NewBuiltin("assert", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinAssert(args...)
		}),
		"assert_eq": object.NewBuiltin("assert_eq", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinAssertEq(args...)
		}),
		"assert_ne": object.NewBuiltin("assert_ne", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinAssertNe(args...)
		}),
		"assert_nil": object.NewBuiltin("assert_nil", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinAssertNil(args...)
		}),
		"assert_error": object.NewBuiltin("assert_error", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinAssertError(args...)
		}),
		"skip": object.NewBuiltin("skip", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinSkip(args...)
		}),
		"fail": object.NewBuiltin("fail", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinFail(args...)
		}),
		"log": object.NewBuiltin("log", func(_ context.Context, args ...object.Object) (object.Object, error) {
			return t.builtinLog(args...)
		}),
	}
}

// --- Object interface ---

func (t *TestContext) Type() object.Type {
	return "test_context"
}

func (t *TestContext) Inspect() string {
	return fmt.Sprintf("TestContext(%s)", t.name)
}

func (t *TestContext) Interface() any {
	return t
}

func (t *TestContext) Equals(other object.Object) bool {
	if otherT, ok := other.(*TestContext); ok {
		return t == otherT
	}
	return false
}

func (t *TestContext) Attrs() []object.AttrSpec {
	// TODO: Migrate to AttrRegistry for introspection support
	return nil
}

func (t *TestContext) GetAttr(name string) (object.Object, bool) {
	// Check for dynamic name property first
	if name == "name" {
		return object.NewString(t.name), true
	}
	if attr, ok := t.attrs[name]; ok {
		return attr, true
	}
	return nil, false
}

func (t *TestContext) SetAttr(name string, value object.Object) error {
	return object.TypeErrorf("test context attributes are read-only")
}

func (t *TestContext) IsTruthy() bool {
	return true
}

func (t *TestContext) RunOperation(opType op.BinaryOpType, right object.Object) (object.Object, error) {
	return nil, object.TypeErrorf("unsupported operation on test context: %v", opType)
}

// --- Test state accessors ---

// Name returns the test name.
func (t *TestContext) Name() string {
	return t.name
}

// Failed returns true if the test has failed.
func (t *TestContext) Failed() bool {
	return t.failed
}

// Skipped returns true if the test was skipped.
func (t *TestContext) Skipped() bool {
	return t.skipped
}

// SkipReason returns the reason for skipping.
func (t *TestContext) SkipReason() string {
	return t.skipReason
}

// Logs returns all logged messages.
func (t *TestContext) Logs() []string {
	return t.logs
}

// Failures returns all assertion failures.
func (t *TestContext) Failures() []AssertionError {
	return t.failures
}

// --- Builtin method implementations ---

// builtinAssert implements t.assert(cond, msg?)
func (t *TestContext) builtinAssert(args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("assert: expected 1-2 arguments, got %d", len(args))
	}
	if !args[0].IsTruthy() {
		msg := "assertion failed"
		if len(args) == 2 {
			if s, ok := args[1].(*object.String); ok {
				msg = s.Value()
			} else {
				msg = args[1].Inspect()
			}
		}
		t.addFailure(msg, args[0], nil)
	}
	return object.Nil, nil
}

// builtinAssertEq implements t.assert_eq(got, want, msg?)
func (t *TestContext) builtinAssertEq(args ...object.Object) (object.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("assert_eq: expected 2-3 arguments, got %d", len(args))
	}
	got, want := args[0], args[1]
	if !got.Equals(want) {
		msg := "values are not equal"
		if len(args) == 3 {
			if s, ok := args[2].(*object.String); ok {
				msg = s.Value()
			} else {
				msg = args[2].Inspect()
			}
		}
		t.addFailureWithValues(msg, got, want)
	}
	return object.Nil, nil
}

// builtinAssertNe implements t.assert_ne(got, want, msg?)
func (t *TestContext) builtinAssertNe(args ...object.Object) (object.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("assert_ne: expected 2-3 arguments, got %d", len(args))
	}
	got, want := args[0], args[1]
	if got.Equals(want) {
		msg := "values should not be equal"
		if len(args) == 3 {
			if s, ok := args[2].(*object.String); ok {
				msg = s.Value()
			} else {
				msg = args[2].Inspect()
			}
		}
		t.addFailureWithValues(msg, got, want)
	}
	return object.Nil, nil
}

// builtinAssertNil implements t.assert_nil(val, msg?)
func (t *TestContext) builtinAssertNil(args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("assert_nil: expected 1-2 arguments, got %d", len(args))
	}
	val := args[0]
	if val != object.Nil {
		msg := "expected nil"
		if len(args) == 2 {
			if s, ok := args[1].(*object.String); ok {
				msg = s.Value()
			} else {
				msg = args[1].Inspect()
			}
		}
		t.addFailureWithValues(msg, val, object.Nil)
	}
	return object.Nil, nil
}

// builtinAssertError implements t.assert_error(val, msg?)
func (t *TestContext) builtinAssertError(args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("assert_error: expected 1-2 arguments, got %d", len(args))
	}
	val := args[0]
	if _, isErr := val.(*object.Error); !isErr {
		msg := "expected error"
		if len(args) == 2 {
			if s, ok := args[1].(*object.String); ok {
				msg = s.Value()
			} else {
				msg = args[1].Inspect()
			}
		}
		t.addFailure(msg, val, nil)
	}
	return object.Nil, nil
}

// builtinSkip implements t.skip(reason?)
func (t *TestContext) builtinSkip(args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("skip: expected 0-1 arguments, got %d", len(args))
	}
	t.skipped = true
	if len(args) == 1 {
		if s, ok := args[0].(*object.String); ok {
			t.skipReason = s.Value()
		} else {
			t.skipReason = args[0].Inspect()
		}
	}
	return object.Nil, nil
}

// builtinFail implements t.fail(msg?)
func (t *TestContext) builtinFail(args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("fail: expected 0-1 arguments, got %d", len(args))
	}
	msg := "test failed"
	if len(args) == 1 {
		if s, ok := args[0].(*object.String); ok {
			msg = s.Value()
		} else {
			msg = args[0].Inspect()
		}
	}
	t.addFailure(msg, nil, nil)
	return object.Nil, nil
}

// builtinLog implements t.log(args...)
func (t *TestContext) builtinLog(args ...object.Object) (object.Object, error) {
	parts := make([]string, len(args))
	for i, arg := range args {
		if s, ok := arg.(*object.String); ok {
			parts[i] = s.Value()
		} else {
			parts[i] = arg.Inspect()
		}
	}
	t.logs = append(t.logs, strings.Join(parts, " "))
	return object.Nil, nil
}

// --- Internal helpers ---

func (t *TestContext) addFailure(msg string, got, want object.Object) {
	t.failed = true
	t.failures = append(t.failures, AssertionError{
		Message: msg,
		File:    t.filename,
		Got:     got,
		Want:    want,
	})
}

func (t *TestContext) addFailureWithValues(msg string, got, want object.Object) {
	t.failed = true
	t.failures = append(t.failures, AssertionError{
		Message: msg,
		File:    t.filename,
		Got:     got,
		Want:    want,
	})
}
