# Enhanced Documentation System

**Status**: Draft
**Author**: Curtis
**Date**: 2026-01-27

## Summary

Enhance the existing `risor doc` command with progressive disclosure and structured output formats. This makes documentation accessible to both human users (terminal-friendly text) and LLM agents (JSON for tooling), while keeping a single, discoverable entry point.

## Motivation

The current `risor doc` command provides detailed documentation, but there's room to improve:

- **Quick orientation** - "what can Risor do?" in 30 seconds
- **Progressive disclosure** - start simple, drill down as needed
- **Structured output** - JSON/Markdown for tooling and LLM integration
- **Error guidance** - help debugging common mistakes

## Design Principles

1. **Progressive Disclosure**: Start minimal, expand on demand
2. **Dual Audience**: Human-readable text, machine-parseable JSON
3. **Example-Rich**: Code samples for every concept
4. **Single Entry Point**: One command to learn, `risor doc`

## CLI Interface

### Current Behavior (preserved)

```bash
risor doc                 # List all topics
risor doc string          # Show string type docs
risor doc filter          # Show builtin function docs
risor doc math            # Show module docs
risor doc math.sqrt       # Show specific function
```

### New: Output Formats

```bash
risor doc --format text       # Human-readable (default)
risor doc --format json       # Structured JSON for tooling/LLMs
risor doc --format markdown   # Markdown output
```

### New: Quick Reference Mode

```bash
risor doc --quick             # Concise overview + syntax cheatsheet
risor doc -q                  # Short flag
```

### New: Expanded Categories

```bash
risor doc syntax              # Complete syntax reference (new)
risor doc errors              # Common errors and fixes (new)
```

### New: Completeness Control

```bash
risor doc --all               # Everything in one response
risor doc types --all         # All types with all methods expanded
```

### Combined Examples

```bash
# Human learning Risor
risor doc --quick

# LLM getting oriented
risor doc --quick --format json

# LLM exploring builtins
risor doc builtins --format json

# Deep dive for LLM
risor doc filter --format json
```

## Information Hierarchy

```
Level 0: Quick Reference (risor doc --quick)
├── What is Risor (1 paragraph)
├── Execution model (source → bytecode → VM)
├── Core syntax cheatsheet (10 patterns)
└── Available topics for deeper exploration

Level 1: Category Overview (risor doc <category>)
├── builtins  - List all with signatures
├── types     - All types with method summaries
├── modules   - Module list with function summaries
├── syntax    - Comprehensive syntax reference
└── errors    - Common errors and fixes

Level 2: Deep Dive (risor doc <topic>)
├── filter            - Full docs + examples
├── string            - All methods with examples
├── string.split      - Specific method details
├── math.sqrt         - Module function details
```

## Output Formats

### Quick Reference (--quick --format json)

```json
{
  "risor": {
    "version": "2.0.0",
    "description": "Fast embedded scripting language for Go",
    "execution_model": "source → lexer → parser → compiler → bytecode → vm"
  },
  "syntax_quick_ref": [
    {"pattern": "let x = 1", "description": "Variable declaration"},
    {"pattern": "const PI = 3.14", "description": "Constant declaration"},
    {"pattern": "x => x * 2", "description": "Arrow function"},
    {"pattern": "function name(a, b) { }", "description": "Named function"},
    {"pattern": "[1, 2, 3]", "description": "List literal"},
    {"pattern": "{key: value}", "description": "Map literal"},
    {"pattern": "obj.method()", "description": "Method call"},
    {"pattern": "list.map(fn).filter(fn)", "description": "Method chaining"},
    {"pattern": "let {a, b} = obj", "description": "Destructuring"},
    {"pattern": "{...a, ...b}", "description": "Spread operator"}
  ],
  "topics": {
    "builtins": "27 built-in functions (len, map, filter, range, ...)",
    "types": "14 types (string, list, map, int, float, ...)",
    "modules": "4 modules (math, rand, regexp, time)",
    "syntax": "Complete syntax reference",
    "errors": "Common errors and debugging"
  },
  "next": [
    "risor doc builtins",
    "risor doc syntax",
    "risor doc types string"
  ]
}
```

### Category Overview (risor doc builtins --format json)

```json
{
  "category": "builtins",
  "description": "Built-in functions available in the global scope",
  "count": 27,
  "functions": [
    {
      "name": "len",
      "signature": "len(obj) -> int",
      "doc": "Returns the length of a string, list, or map"
    },
    {
      "name": "map",
      "signature": "map(items, fn) -> list",
      "doc": "Transform each element using fn"
    },
    {
      "name": "filter",
      "signature": "filter(items, fn) -> list",
      "doc": "Keep elements where fn returns true"
    }
  ],
  "next": "Use 'risor doc <name>' for full docs and examples"
}
```

### Deep Dive (risor doc filter --format json)

```json
{
  "name": "filter",
  "category": "builtin",
  "signature": "filter(items, fn) -> list",
  "doc": "Returns a new list containing only elements for which fn returns true",
  "parameters": [
    {"name": "items", "type": "list", "doc": "The list to filter"},
    {"name": "fn", "type": "function", "doc": "Predicate function: (element) -> bool"}
  ],
  "returns": {
    "type": "list",
    "doc": "New list with matching elements"
  },
  "examples": [
    {
      "code": "filter([1, 2, 3, 4, 5], x => x > 2)",
      "result": "[3, 4, 5]",
      "explanation": "Keep numbers greater than 2"
    },
    {
      "code": "filter([\"a\", \"bb\", \"ccc\"], s => len(s) > 1)",
      "result": "[\"bb\", \"ccc\"]",
      "explanation": "Keep strings longer than 1 character"
    }
  ],
  "related": ["map", "reduce", "any", "all"],
  "common_errors": [
    {
      "mistake": "filter(items, x > 2)",
      "fix": "filter(items, x => x > 2)",
      "explanation": "Filter requires a function, not an expression"
    }
  ]
}
```

## New Content: Syntax Reference

The `risor doc syntax` command provides a comprehensive syntax reference:

```json
{
  "category": "syntax",
  "sections": [
    {
      "name": "literals",
      "items": [
        {"syntax": "42", "type": "int", "notes": "Integer literal"},
        {"syntax": "3.14", "type": "float", "notes": "Float literal"},
        {"syntax": "\"hello\"", "type": "string", "notes": "String literal"},
        {"syntax": "`raw`", "type": "string", "notes": "Raw string (no escapes)"},
        {"syntax": "true, false", "type": "bool", "notes": "Boolean literals"},
        {"syntax": "nil", "type": "nil", "notes": "Null value"}
      ]
    },
    {
      "name": "collections",
      "items": [
        {"syntax": "[a, b, c]", "type": "list", "notes": "List literal"},
        {"syntax": "{k: v}", "type": "map", "notes": "Map literal"},
        {"syntax": "{k}", "type": "map", "notes": "Shorthand for {k: k}"},
        {"syntax": "[...a, ...b]", "type": "list", "notes": "List spread"},
        {"syntax": "{...a, ...b}", "type": "map", "notes": "Map spread"}
      ]
    },
    {
      "name": "variables",
      "items": [
        {"syntax": "let x = value", "notes": "Mutable variable"},
        {"syntax": "const X = value", "notes": "Immutable constant"},
        {"syntax": "let {a, b} = obj", "notes": "Destructuring assignment"},
        {"syntax": "let [x, y] = list", "notes": "List destructuring"}
      ]
    },
    {
      "name": "functions",
      "items": [
        {"syntax": "function name(a, b) { body }", "notes": "Named function"},
        {"syntax": "let f = function(a) { body }", "notes": "Anonymous function"},
        {"syntax": "x => expr", "notes": "Arrow function (single param)"},
        {"syntax": "(a, b) => expr", "notes": "Arrow function (multiple params)"},
        {"syntax": "(a, b) => { stmts }", "notes": "Arrow function with block"}
      ]
    },
    {
      "name": "control_flow",
      "items": [
        {"syntax": "if (cond) { } else { }", "notes": "Conditional (is an expression)"},
        {"syntax": "switch (val) { case x: ... }", "notes": "Switch statement"},
        {"syntax": "try { } catch e { }", "notes": "Error handling"},
        {"syntax": "throw error(msg)", "notes": "Raise an error"},
        {"syntax": "return value", "notes": "Return from function"}
      ]
    },
    {
      "name": "operators",
      "items": [
        {"syntax": "+ - * / %", "notes": "Arithmetic"},
        {"syntax": "== != < > <= >=", "notes": "Comparison"},
        {"syntax": "&& || !", "notes": "Logical"},
        {"syntax": "?? ?.", "notes": "Nil coalescing, optional chain"},
        {"syntax": "|", "notes": "Pipe operator"}
      ]
    },
    {
      "name": "method_chaining",
      "items": [
        {"syntax": "obj.method()", "notes": "Method call"},
        {"syntax": "list.map(f).filter(g)", "notes": "Chained methods"},
        {"syntax": "obj?.method()", "notes": "Optional chaining"}
      ]
    }
  ]
}
```

## New Content: Error Reference

The `risor doc errors` command helps debug common issues:

```json
{
  "category": "errors",
  "patterns": [
    {
      "type": "type_error",
      "message_pattern": "type error: expected X, got Y",
      "causes": [
        "Passing wrong type to function",
        "Method called on incompatible type"
      ],
      "examples": [
        {
          "error": "type error: expected int, got string",
          "bad_code": "len(123)",
          "fix": "len(\"123\") or len([1, 2, 3])",
          "explanation": "len() works on strings, lists, and maps, not ints"
        }
      ]
    },
    {
      "type": "name_error",
      "message_pattern": "name error: X is not defined",
      "causes": [
        "Typo in variable name",
        "Variable used before declaration",
        "Builtin not available (no-default-globals mode)"
      ],
      "examples": [
        {
          "error": "name error: 'maths' is not defined",
          "bad_code": "maths.sqrt(4)",
          "fix": "math.sqrt(4)",
          "explanation": "Module name is 'math', not 'maths'"
        }
      ]
    },
    {
      "type": "syntax_error",
      "message_pattern": "syntax error at line N",
      "causes": [
        "Missing closing bracket/brace",
        "Invalid token",
        "Incomplete statement"
      ],
      "examples": [
        {
          "error": "syntax error: expected '}'",
          "bad_code": "if x { print(x)",
          "fix": "if x { print(x) }",
          "explanation": "Block must be closed with '}'"
        }
      ]
    }
  ]
}
```

## Library API

```go
package risor

// Docs returns structured documentation about Risor.
// Useful for tooling, editor integrations, and LLM agents.
func Docs(opts ...DocsOption) *Documentation

// Options
func DocsCategory(cat string) DocsOption     // "builtins", "types", etc.
func DocsTopic(topic string) DocsOption      // specific item
func DocsQuick() DocsOption                  // quick reference mode

// Example usage
docs := risor.Docs(risor.DocsQuick())
fmt.Println(docs.JSON())
fmt.Println(docs.Text())

// Deep dive
docs := risor.Docs(
    risor.DocsCategory("types"),
    risor.DocsTopic("string"),
)
```

## Implementation Plan

### Phase 1: Output Format Support

1. **Add format flag** to existing `doc.go`:
   - `--format text` (default, current behavior)
   - `--format json`
   - `--format markdown`

2. **Define JSON structures** in `cmd/risor/doc_types.go`:
   - Reuse existing `FuncSpec`, `AttrSpec`, `TypeSpec`
   - Add wrapper types for JSON serialization

### Phase 2: Quick Reference Mode

3. **Add `--quick` flag** for concise overview:
   - Version and description
   - Syntax cheatsheet
   - Topic list with counts

### Phase 3: New Categories

4. **Add `syntax` category**:
   - Comprehensive syntax reference
   - Organized by category (literals, functions, operators, etc.)

5. **Add `errors` category**:
   - Common error patterns
   - Causes and fixes

### Phase 4: Library API

6. **Export `risor.Docs()`** in `risor.go`:
   - Wraps CLI doc infrastructure
   - Functional options pattern

## File Changes

```
cmd/risor/
├── doc.go           # Modify: add --format, --quick, --all flags
├── doc_types.go     # New: JSON output structures
├── doc_syntax.go    # New: syntax reference data
└── doc_errors.go    # New: error guidance data

risor.go             # Modify: add Docs() export
```

## Usage Examples

### Human Workflow

```bash
# New user learning Risor
$ risor doc --quick
# Sees: overview, syntax cheatsheet, available topics

$ risor doc syntax
# Sees: complete syntax reference

$ risor doc string
# Sees: all string methods with examples
```

### LLM Agent Workflow

```
Human: Help me write Risor code to process a list of users

LLM: [runs: risor doc --quick --format json]
     [learns: Risor basics, available topics]
     [runs: risor doc builtins --format json]
     [learns: map, filter, reduce are available]
     [runs: risor doc list --format json]
     [learns: list methods like .map(), .filter()]

     Here's how to process users in Risor:

     let users = [{name: "Alice", age: 30}, {name: "Bob", age: 25}]
     let adults = users.filter(u => u.age >= 18)
     let names = adults.map(u => u.name)
```

### Claude Code Integration

When Claude Code detects a `.risor` file or Risor context, it can:

1. Run `risor doc --quick --format json` to get oriented
2. Cache the output for the session
3. Drill down with `risor doc <topic> --format json` as needed

### MCP Server (Future)

The doc system could be exposed as an MCP server:

```json
{
  "tools": [
    {
      "name": "risor_docs",
      "description": "Get Risor language documentation",
      "inputSchema": {
        "type": "object",
        "properties": {
          "quick": {"type": "boolean"},
          "category": {"type": "string", "enum": ["builtins", "types", "modules", "syntax", "errors"]},
          "topic": {"type": "string"}
        }
      }
    }
  ]
}
```

## Alternatives Considered

### New `risor info` Command

**Pros**: Clean separation of concerns
**Cons**: Two commands to learn, duplication

**Decision**: Rejected. Enhancing `risor doc` is simpler and more discoverable.

### Static llm.txt File

**Pros**: Simple, follows convention
**Cons**: Single file can't scale, no progressive disclosure

**Decision**: Rejected. A static file either includes too little (not useful) or too much (wastes context).

### External Documentation Website API

**Pros**: Always up-to-date, rich content
**Cons**: Requires network, latency, may not be available

**Decision**: Rejected. Documentation should work offline.

## Success Criteria

1. LLM can learn Risor basics in < 3 CLI calls
2. Full documentation fits in < 50K tokens (`--all`)
3. Quick reference fits in < 2K tokens (`--quick`)
4. JSON output is valid and well-structured
5. Backward compatible - existing `risor doc` usage unchanged
6. Examples are runnable and correct

## Open Questions

1. Should `--quick` be the default when no arguments provided?
2. Should we include REPL command documentation?
3. Should syntax reference include operator precedence table?
4. Should `--all` include error guidance or keep it separate?

## References

- [llm.txt specification](https://llmstxt.org/)
- Existing `risor doc` implementation in `cmd/risor/doc.go`
- [MCP server specification](https://modelcontextprotocol.io/)
