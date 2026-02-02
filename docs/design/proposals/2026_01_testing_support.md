# First-Class Testing Support for Risor v2

## Problem Statement

Risor currently lacks built-in testing capabilities for user scripts and programs. Developers who want to test their Risor code must either:

1. Write tests in Go that call into Risor
2. Create ad-hoc assertion patterns in their scripts
3. Use external testing frameworks not designed for Risor

This friction discourages test-driven development and makes it harder to build robust Risor applications. As Risor matures, first-class testing support becomes essential for developers building production-quality scripts.

## Goals

- **Natural syntax**: Testing should feel native to Risor, not bolted on
- **Familiar patterns**: Draw inspiration from Go's `testing` package and modern testing frameworks
- **Zero-config discovery**: Auto-detect tests via file patterns, minimal boilerplate
- **CLI integration**: `risor test` command for running tests
- **Rich assertions**: Built-in assertion functions with clear, diff-based error messages
- **Table-driven tests**: First-class support for parameterized testing
- **Snapshot testing**: Capture and compare output for regression testing
- **Fast iteration**: Watch mode for re-running tests on file changes
- **Benchmarking**: Performance measurement capabilities

## Solution Overview

### 1. Test File Convention

Test files follow the naming convention `*_test.risor`:

```
project/
├── calculator.risor
├── calculator_test.risor
├── utils/
│   ├── format.risor
│   └── format_test.risor
└── lib/
    └── math_test.risor
```

### 2. Test Function Convention

Test functions are named with a `test_` prefix and receive a test context `t`:

```risor
function test_addition(t) {
    result := add(2, 3)
    t.assert_eq(result, 5)
}

function test_string_formatting(t) {
    name := "world"
    t.assert_eq(format("Hello, {}", name), "Hello, world")
}
```

### 3. CLI Integration

A new `risor test` command discovers and runs tests:

```bash
# Run all tests in current directory
risor test

# Run tests in specific directory
risor test ./lib/...

# Run tests matching a pattern
risor test -run "calculator"

# Run with verbose output
risor test -v

# Watch mode: re-run on file changes
risor test -watch

# Run benchmarks
risor test -bench

# Update snapshots
risor test -update-snapshots

# Run with coverage
risor test -cover
```

## Implementation Details

### Test Context Object (`t`)

The test context provides assertion methods and test control:

```risor
// Basic assertions
t.assert(condition)              // Assert condition is truthy
t.assert(condition, "message")   // Assert with custom message
t.assert_eq(got, want)           // Assert equality
t.assert_ne(got, want)           // Assert inequality
t.assert_nil(value)              // Assert value is nil
t.assert_not_nil(value)          // Assert value is not nil
t.assert_error(err)              // Assert value is an error
t.assert_no_error(err)           // Assert value is not an error
t.assert_contains(haystack, needle)  // Assert string/list contains value
t.assert_len(value, length)      // Assert length equals

// Test control
t.fail()                         // Mark test as failed
t.fail(message)                  // Mark as failed with message
t.skip()                         // Skip this test
t.skip(reason)                   // Skip with reason
t.log(args...)                   // Log message (shown with -v)

// Snapshots
t.assert_snapshot(value)         // Compare against stored snapshot
t.assert_snapshot(value, name)   // Named snapshot

// Test metadata
t.name                           // Current test name (string)
```

### Fixtures and Lifecycle

Fixtures provide reusable setup/teardown logic with configurable scopes:

```risor
// Per-test fixture (default): runs before/after each test
function setup() {
    return {
        "calc": calculator.new()
    }
}

function teardown(ctx) {
    ctx.calc.close()
}

// Per-file fixture: runs once for all tests in this file
function setup_file() {
    return {
        "db": connect_mock_db()
    }
}

function teardown_file(ctx) {
    ctx.db.close()
}

function test_database_query(t) {
    // Access fixtures via t.ctx (merged from all scopes)
    result := t.ctx.db.query("SELECT 1")
    t.assert_eq(result, 1)
}
```

**Fixture Scopes:**
| Scope | Setup Function | Teardown Function | Lifecycle |
|-------|---------------|-------------------|-----------|
| Test | `setup()` | `teardown(ctx)` | Before/after each test |
| File | `setup_file()` | `teardown_file(ctx)` | Once per test file |

Fixtures are composable: file-level fixtures are created first, then per-test fixtures. The context is merged with per-test values taking precedence.

### Table-Driven Tests

First-class support for parameterized testing:

```risor
function test_fibonacci(t) {
    cases := [
        {name: "zero",  input: 0, want: 0},
        {name: "one",   input: 1, want: 1},
        {name: "five",  input: 5, want: 5},
        {name: "ten",   input: 10, want: 55},
    ]

    for tc in cases {
        t.run(tc.name, function(t) {
            got := fibonacci(tc.input)
            t.assert_eq(got, tc.want)
        })
    }
}
```

Sub-tests created with `t.run()` appear in output with their full path:

```
=== RUN   test_fibonacci
=== RUN   test_fibonacci/zero
=== RUN   test_fibonacci/one
=== RUN   test_fibonacci/five
=== RUN   test_fibonacci/ten
--- PASS: test_fibonacci (0.001s)
```

### Benchmarks

Benchmark functions use the `bench_` prefix:

```risor
function bench_string_concat(b) {
    for i in range(b.n) {
        result := "hello" + " " + "world"
    }
}

function bench_string_format(b) {
    for i in range(b.n) {
        result := format("{} {}", "hello", "world")
    }
}
```

Run with `risor test -bench`:

```
BenchmarkStringConcat    1000000    1234 ns/op
BenchmarkStringFormat     500000    2345 ns/op
```

### Snapshot Testing

Snapshots capture output for regression testing, inspired by Jest:

```risor
function test_report_generation(t) {
    report := generate_report(sample_data)
    t.assert_snapshot(report)  // Compares against stored snapshot
}

function test_json_output(t) {
    result := api.format_response(data)
    t.assert_snapshot(result, "api_response")  // Named snapshot
}
```

**Snapshot workflow:**
1. First run creates snapshot files in `__snapshots__/` directory
2. Subsequent runs compare output against stored snapshots
3. Use `risor test -update-snapshots` to regenerate after intentional changes

**Snapshot file format** (`calculator_test.risor.snap`):
```
// test_report_generation
{
  "total": 42,
  "items": ["a", "b", "c"]
}

// test_json_output/api_response
{"status": "ok", "data": [...]}
```

Snapshots are especially useful for:
- Complex data structures that are tedious to assert field-by-field
- Output formats (JSON, formatted strings, reports)
- Catching unintended changes in behavior

### Error Messages

Assertions produce clear, actionable error messages with smart introspection and diffs:

```
--- FAIL: test_user_validation (0.002s)
    user_test.risor:15: assertion failed
        assert_eq(got, want)
        got:  "invalid"
        want: "valid"

--- FAIL: test_list_operations (0.001s)
    list_test.risor:28: assertion failed
        assert_contains(list, item)
        list: [1, 2, 3]
        item: 4

--- FAIL: test_config_parsing (0.003s)
    config_test.risor:42: assertion failed
        assert_eq(got, want)
        diff:
          {
            "name": "test",
        -   "value": 100,
        +   "value": 200,
            "enabled": true
          }
```

**Error message features:**
- **Source location**: File and line number for quick navigation
- **Value inspection**: Shows actual values, not just "not equal"
- **Diff output**: For maps and lists, shows exactly what differs
- **Type information**: When types mismatch, shows both types

### Watch Mode

Watch mode enables rapid development iteration:

```bash
risor test -watch
```

**Behavior:**
- Monitors `*.risor` and `*_test.risor` files for changes
- Re-runs affected tests when files change
- Only re-runs tests in changed files and their dependents
- Clears terminal and shows fresh results on each run

```
Watching for changes... (press q to quit)

=== RUN   test_addition
--- PASS: test_addition (0.001s)

PASS - 1 passed

[12:34:56] Waiting for changes...
```

### The `testing` Module

A new `testing` module provides utilities for test code:

```risor
import testing

function test_with_timeout(t) {
    // Run with timeout
    testing.with_timeout(5.0, function() {
        slow_operation()
    })
}

function test_mock_time(t) {
    // Mock the time module
    testing.mock("time.now", function() {
        return testing.fixed_time("2025-01-01T00:00:00Z")
    })

    // Test code using time.now() gets mocked value
    t.assert_eq(time.now().format("2006"), "2025")
}

function test_temp_files(t) {
    // Create temporary file that's cleaned up after test
    path := testing.temp_file("test-*.txt")
    write_file(path, "content")
    t.assert_eq(read_file(path), "content")
}
```

### Test Discovery

The test runner discovers tests by:

1. Finding all `*_test.risor` files in the target directories
2. Parsing each file to find `test_*` and `bench_*` functions
3. Running tests in parallel by default (configurable with `-parallel`)

### CLI Command Structure

```go
// In cmd/risor/main.go
app.Command("test").
    Description("Run tests").
    Args("patterns...").
    Flags(
        cli.String("run", "r").Help("Run only tests matching pattern"),
        cli.Bool("verbose", "v").Help("Verbose output"),
        cli.Bool("watch", "w").Help("Watch mode: re-run on file changes"),
        cli.Bool("bench", "b").Help("Run benchmarks"),
        cli.Bool("update-snapshots", "u").Help("Update snapshot files"),
        cli.Int("parallel", "p").Default("4").Help("Number of parallel tests"),
        cli.Bool("cover", "").Help("Enable coverage"),
        cli.String("timeout", "").Default("10m").Help("Test timeout"),
        cli.String("output", "o").Enum("text", "json", "tap").Help("Output format"),
        cli.Bool("fail-fast", "").Help("Stop on first failure"),
    ).
    Run(testHandler)
```

### Output Formats

**Text (default):**
```
=== RUN   test_addition
--- PASS: test_addition (0.001s)
=== RUN   test_subtraction
--- PASS: test_subtraction (0.000s)
=== RUN   test_division_by_zero
--- FAIL: test_division_by_zero (0.001s)
    calc_test.risor:25: expected error, got nil

FAIL
2 passed, 1 failed
```

**JSON (`-o json`):**
```json
{
  "passed": 2,
  "failed": 1,
  "skipped": 0,
  "duration": "0.003s",
  "tests": [
    {"name": "test_addition", "status": "pass", "duration": "0.001s"},
    {"name": "test_subtraction", "status": "pass", "duration": "0.000s"},
    {"name": "test_division_by_zero", "status": "fail", "duration": "0.001s",
     "error": "calc_test.risor:25: expected error, got nil"}
  ]
}
```

**TAP (`-o tap`):**
```
TAP version 13
1..3
ok 1 - test_addition
ok 2 - test_subtraction
not ok 3 - test_division_by_zero
  ---
  message: expected error, got nil
  at: calc_test.risor:25
  ...
```

## Code Examples

### Complete Test File Example

```risor
// calculator_test.risor

import calculator  // The module being tested

// Setup creates test fixtures
function setup() {
    return {
        "calc": calculator.new()
    }
}

// Basic test
function test_add(t) {
    result := t.ctx.calc.add(2, 3)
    t.assert_eq(result, 5)
}

// Test with multiple assertions
function test_subtract(t) {
    calc := t.ctx.calc

    t.assert_eq(calc.subtract(5, 3), 2)
    t.assert_eq(calc.subtract(0, 5), -5)
    t.assert_eq(calc.subtract(5, 5), 0)
}

// Table-driven test
function test_multiply(t) {
    cases := [
        {a: 2, b: 3, want: 6},
        {a: 0, b: 5, want: 0},
        {a: -1, b: 5, want: -5},
        {a: 100, b: 100, want: 10000},
    ]

    for tc in cases {
        t.run(sprintf("%d*%d", tc.a, tc.b), function(t) {
            got := t.ctx.calc.multiply(tc.a, tc.b)
            t.assert_eq(got, tc.want)
        })
    }
}

// Test error handling
function test_divide_by_zero(t) {
    result := t.ctx.calc.divide(10, 0)
    t.assert_error(result)
    t.assert_contains(string(result), "division by zero")
}

// Skip test conditionally
function test_advanced_feature(t) {
    if !calculator.ADVANCED_ENABLED {
        t.skip("advanced features not enabled")
    }
    // ...test code...
}

// Benchmark
function bench_add(b) {
    calc := calculator.new()
    for i in range(b.n) {
        calc.add(i, i)
    }
}
```

### Running Tests

```bash
# Run all tests
$ risor test
=== RUN   test_add
--- PASS: test_add (0.001s)
=== RUN   test_subtract
--- PASS: test_subtract (0.001s)
=== RUN   test_multiply
=== RUN   test_multiply/2*3
=== RUN   test_multiply/0*5
=== RUN   test_multiply/-1*5
=== RUN   test_multiply/100*100
--- PASS: test_multiply (0.002s)
=== RUN   test_divide_by_zero
--- PASS: test_divide_by_zero (0.001s)
=== RUN   test_advanced_feature
--- SKIP: test_advanced_feature (0.000s)
    advanced features not enabled

PASS
4 passed, 0 failed, 1 skipped

# Run specific test
$ risor test -run multiply
=== RUN   test_multiply
...

# Verbose output shows t.log() calls
$ risor test -v
```

## Implementation Phases

### Phase 1: MVP
Minimal viable testing - enough to write and run real tests.

- `risor test` command with file discovery (`*_test.risor`)
- `test_*` function convention
- Test context with core assertions (`assert`, `assert_eq`, `assert_ne`, `assert_nil`, `assert_error`)
- Basic text output (PASS/FAIL with source locations)
- `t.skip()`, `t.fail()`, `t.log()`

### Phase 2: Production-Ready
Features needed for serious test suites.

- Fixtures with scopes (`setup`/`teardown`, `setup_file`/`teardown_file`)
- Sub-tests with `t.run()` for table-driven testing
- Diff-based error messages for maps and lists
- Additional assertions (`assert_contains`, `assert_len`, `assert_not_nil`, `assert_no_error`)
- JSON and TAP output formats
- `-run` pattern filtering
- `-fail-fast` mode
- Parallel test execution

### Phase 3: Power Features
Developer experience and performance tooling.

- Watch mode (`-watch`)
- Snapshot testing (`t.assert_snapshot`, `-update-snapshots`)
- Benchmarks (`bench_*` functions, `-bench`)
- `testing` module utilities (timeouts, temp files, mocking hooks)
- Coverage reporting

## Testing the Testing Framework

The testing infrastructure itself will be tested via Go tests that:

1. Create temporary test files
2. Run the test discovery and execution
3. Verify correct pass/fail detection
4. Verify output formatting
5. Test edge cases (panics, timeouts, etc.)

## Backward Compatibility

This feature is purely additive. Existing Risor code continues to work unchanged. The `testing` module is opt-in, and the `risor test` command only processes `*_test.risor` files.

## Alternatives Considered

### 1. Assert as Global Function
Instead of `t.assert_eq()`, use global `assert_eq()`. Rejected because:
- Harder to associate failures with specific tests
- No way to implement skip/fail without test context
- Less explicit about what's being tested

### 2. Test Classes/Objects
Use object-oriented test suites like pytest. Rejected because:
- Adds complexity
- Go-style test functions are simpler and familiar
- Risor doesn't have classes

### 3. Decorator-Based Discovery
Use decorators like `@test` to mark tests. Rejected because:
- Risor doesn't have decorators
- Naming convention is simpler and matches Go

## Design Decisions

1. **Fixture scopes**: Two scopes (per-test, per-file) provide flexibility without complexity. Session-scoped fixtures were considered but deferred to keep the initial implementation simple.

2. **Snapshot storage**: Snapshots live in `__snapshots__/` directories alongside test files, following Jest's convention.

3. **Lightweight execution**: Tests run in the same VM for speed. Use fixtures for setup/teardown to manage state between tests.

## Open Questions

1. **Coverage**: How granular should coverage reporting be? Line-level or function-level?

2. **Mocking**: Should the `testing` module include a full mocking framework, or keep it minimal? Initial lean: minimal, with hooks for users to build their own.

3. **Async tests**: If Risor adds async/await, how should async tests be handled?

4. **Doc tests**: Should Risor support testing code examples in documentation comments (like Rust's doc tests)? This could be a future addition.

## Related Work

| Framework | Key Inspiration |
|-----------|----------------|
| Go's `testing` | Function naming convention, table-driven tests, benchmark API |
| pytest | Scoped fixtures, smart assertion introspection, minimal boilerplate |
| Jest | Snapshot testing, watch mode, zero-config discovery |
| Rust's cargo test | Parallelism by default, first-class toolchain integration |

## Conclusion

First-class testing support makes Risor a complete platform for building production-quality scripts. By combining the best patterns from Go (simplicity, table-driven tests), pytest (fixtures, introspection), and Jest (snapshots, watch mode), Risor's testing framework will feel both familiar and powerful.

The phased implementation prioritizes the core developer workflow first, then adds productivity features like watch mode and snapshots, ensuring each release delivers immediate value while building toward a comprehensive solution.
