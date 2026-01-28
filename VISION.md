# Risor v2 Vision

**Risor is an embeddable expression and scripting language for Go applications.**

## What Risor Is For

Risor exists to give Go developers a safe, fast way to evaluate user-provided
expressions and scripts at runtime. It fills the gap between hardcoded logic
and embedding a heavier runtime like Python or V8.

**Primary use cases:**

- **Expression evaluation** — Dynamic filters, computed fields, validation rules
- **String templating** — Interpolated strings with embedded expressions
- **Configuration logic** — When static config isn't enough but a full language is too much
- **User-defined automation** — Let users write small scripts that extend your app
- **Data transformation** — Process data with user-provided logic

**Who it's for:**

Go developers building applications that need runtime flexibility without
sacrificing safety or simplicity.

## Design Principles

These principles guide every decision. When in conflict, earlier principles win.

1. **Correctness** — Well-defined behavior in all cases; no silent failures.
2. **Clarity** — One obvious way to do things; explicit over implicit.
3. **Foundation** — A small, stable core that others build upon.
4. **Focused** — Solve the embedding use case well; defer features that add complexity without clear value.
5. **Elegant** — Intuitive syntax familiar to anyone who's read or written TypeScript.

## What We Build

- A complete, well-specified language
- A fast bytecode compiler and VM
- A clean Go API for embedding
- Clear documentation and examples
- A REPL and CLI for development
- LSP support for editor integration

## What We Don't Build

**Not a general-purpose language.** Risor is not trying to replace Python or
JavaScript for writing applications. It's a tool for Go developers to add
flexibility to their applications.

**Not for large programs.** Risor can handle longer scripts, but that's not
where we focus. Tooling and language features prioritize expressions and
small scripts.

**Not an ecosystem.** No package manager, no plugin system, no third-party
module registry. Extension happens through Go code, not Risor code.

**Not backwards compatible with v1.** Risor v2 is a clean break. We're not
constrained by v1 decisions. Migration is not a priority.

**Not maximum performance at all costs.** We care about performance, but not
at the expense of correctness or clarity. "Fast enough" is the goal.

## How Extension Works

Risor is intentionally minimal. You extend it by:

1. **Adding builtins** — Write Go functions, expose them to scripts
2. **Providing data via Env** — Pass Go values into the script context
3. **Wrapping in your application** — Build your own CLI, API, or tooling around Risor

The integration point is Go code, not Risor code. This keeps the core small
and lets each application tailor Risor to its needs.

## Decision Framework

When evaluating a feature or change, ask:

1. **Does this serve embedded use cases?** If it only helps standalone scripting, it's probably out of scope.
2. **Does this keep the core small?** If it can be a built-in function instead of a language feature, prefer that.
3. **Is this the only way to do it?** If we already have a way, we need a strong reason to add another.
4. **Will this surprise users?** If behavior isn't obvious, reconsider.
5. **Can we say no?** The best features are the ones we don't add. Every feature is maintenance forever.

## Success Looks Like

- A Go developer can embed Risor in under 10 lines of code
- Users can write expressions without reading documentation
- Error messages tell you exactly what's wrong
