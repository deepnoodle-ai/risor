# Risor Stability Promise and Versioning Policy

> Before **3.0.0** is tagged, Risor v3 is a prerelease. Between alphas
> (`3.0.0-alpha.N`) the language, specification, cost table, and host APIs may
> change without notice. Release candidates (`3.0.0-rc.N`) take only fixes.
> The stability promises below take effect at **3.0.0**.

This document states what users and host embedders can rely on across Risor
3.x releases: what may change, what is a breaking change, how the specification
version relates to the Go, TypeScript, and Rust packages, how deprecations
work, and how Risor v2 is supported.

## 1. Compatibility promise within 3.x

Within major version **3**, every minor release (`3.1`, `3.2`, ...) is
**additive only**.

### 1.1 Language and evaluation

- A program that parses, checks, and runs under specification version `3.x`
  continues to parse, check, and produce the **same value** (or the **same
  failure code and error class**) in every later `3.y` (`y >= x`), aside from
  nondeterministic `deadline-exceeded` and `cancelled` outcomes
  (`spec/core/03-host.md` 3.6.2).
- **Step counts and allocation unit counts are fixed per specification version**
  (`spec/core/03-host.md` 3.6.3, `04-library.md` 1.2, decision D16). For any
  run that succeeds, or fails with a non-limit code, the exact step and unit
  counts never change within a major version.
- Every `3.x` implementation offers every earlier minor specification version
  of major version 3 (`03-host.md` 3.9.2). An environment that declares
  `version: "3.0"` does not expose syntax or library entries added in `3.1`.
  An unknown specification version is refused with `unsupported-version` and is
  never silently substituted.
- An environment that omits a version declaration defaults to the newest
  specification version the implementation offers (`03-host.md` 3.3.1). Hosts
  that store user-authored programs should pin `3.0` (or their target `3.x`)
  explicitly and advance the pinned version deliberately.
- Words reserved in `3.0` for a later version (`nil`, `let`, `var`, `for`,
  `while`, `do`, `break`, `continue`, `return`, `function`, `throw`, `finally`,
  `import`; `spec/core/01-language.md` 2.4) may gain meaning in a later `3.x`
  minor version. Because they are rejected in `3.0`, giving them meaning later
  does not break valid `3.0` programs.
- The namespaces `json`, `time`, `math`, `base64`, and `regexp` are reserved
  for the library (`spec/core/03-host.md` 3.3.3): host functions may not be
  declared in them, and later `3.x` minor versions may add entries to them.
  `json`, `time`, `math`, and `base64` already ship `3.0` entries in opt-in
  groups (`suite/library-groups.json`, `spec/packages/f13-math.md` 1.1);
  `regexp` has no entries yet.

### 1.2 Standard library

Per `spec/core/04-library.md` 1.2 and `spec/packages/f06-groups.md` 1.4, within
major version 3:

| Surface | Rule within 3.x |
| --- | --- |
| Library entries and opt-in groups | Only grow. Existing entries and groups are never removed or renamed. |
| Entry signatures | May gain an optional last parameter only if every existing call keeps its exact result, failures, steps, and units. Nothing else changes. |
| Results and failures | Never change for any input. |
| Step and allocation formulas | Never change. A change to a result or a cost requires a new entry with a new name; the old entry stays. |
| Shared rules | Text/order rules (`04-library.md` section 3), the pinned Unicode 16.0.0 case table (`suite/casemap.json`), and the 25-code-point whitespace set never change. |
| Failure codes and `run` message templates | Never change. |
| Non-normative metadata | `doc` strings, `wrong` hint lists, and additional conformance cases may change. |

### 1.3 Diagnostics and messages

Per `spec/core/03-host.md` 3.8.4:

- **`run`-phase messages never change within 3.x.** Programs can read
  `err.message` in `catch` blocks and pay allocation units for its length, so
  run messages are part of the deterministic contract.
- **`declare`, `parse`, and `check` messages may improve in a minor release**,
  as may optional diagnostic fields (`help`, `expected`, `found`, `fix`) and
  hint tables (`suite/foreign-names.json`). Callers testing non-run failures
  programmatically should assert on `code` and `phase`, not exact wording.
- **Diagnostic codes are never renamed or reused within 3.x.** A targeted parse
  or check error code may stop firing for inputs that a later `3.x` minor
  version turns into valid programs.

### 1.4 Host bindings and tools

- **Host APIs** (Go, TypeScript, Rust) follow semantic versioning: minor
  releases may add functions, methods, options, or struct/object fields without
  breaking existing callers. In Go, the CLI-only companion package
  `github.com/deepnoodle-ai/risor/go/v3/risorcli` exists solely to support the
  separate `go/cmd/risor` CLI module and is outside the frozen host embedding
  API promise (hosts embed `risor` and test with `risor/risortest`).
- **Conformance suite and case format:** new case fields and result fields are
  additive; valid `3.0` case files remain valid across `3.x`.
- **Formatter (`F12`):** formatted output for existing syntax is frozen within
  `3.x` so CI format checks (`risor fmt --check`) do not break on upgrades.
  Output may change within `3.x` only for clear bug fixes (such as violations
  of `F12` 2.2's laws: idempotence, equal syntax tree, equal counts) or for
  new syntax introduced in a minor specification version.
- **Assist (completion and hover):** additive improvements (new candidates
  introduced by a new minor specification version, richer documentation) are
  non-breaking.

## 2. Specification version vs. package versions

Risor separates the **specification version** from the **package versions** of
the three implementations, the Go CLI, and companion web packages.

| Artifact | Identifier | Tag / registry |
| --- | --- | --- |
| Specification (`spec/`, `suite/`) | `major.minor` (e.g., `3.0`, `3.1`) | Recorded in `spec/` and `suite/` |
| Go module | `github.com/deepnoodle-ai/risor/go/v3` | Git tag `go/v3.x.y` (never bare `v3.*`; see R200) |
| Go CLI | `github.com/deepnoodle-ai/risor/go/cmd/risor` | Git tag `go/cmd/risor/vX.Y.Z` (separate Go module so `go/v3` keeps zero external dependencies) |
| TypeScript & web packages | `@deepnoodle/risor`, `@deepnoodle/risor-service`, `@deepnoodle/risor-codemirror`, `@deepnoodle/risor-react`, `@deepnoodle/risor-mantine` | Git tag `npm-v3.x.y`, staged in lockstep via `web/scripts/version.ts` and `.github/workflows/release-npm.yml` (a maintainer approves each staged package with 2FA on npmjs.com before publication) |
| Rust crates | `risor`, `risor-derive` | Git tag `rust-v3.x.y`, published to crates.io at `3.0.0` and `3.x.y` |

### How the version numbers relate

1. **Major versions match.** All `3.x.y` packages implement specification major
   version `3`. A breaking change to the specification increments every
   implementation to `4.0.0` (Go module path `/go/v4`).
2. **Host-API-only minor bumps are allowed.** A new specification minor version
   (e.g., `3.1`) always bumps the package minor version of each implementation
   that ships it. Implementation-only additive host API features may also bump
   that implementation's minor version without bumping the specification
   version, so a package's minor version is always **greater than or equal to**
   the newest specification minor version it implements.
3. **Patch versions are independent and target the latest `3.x` minor only.**
   Bug fixes, performance work, and security patches increment `PATCH`
   (`3.x.y`) only for the affected implementation(s). Go `go/v3.0.2`, npm
   `npm-v3.0.1`, and Rust `rust-v3.0.0` are all valid `3.0` releases without
   empty cross-language bumps. Because every `3.x` minor release is strictly
   additive and safe to upgrade to, patches ship on the latest `3.x` minor
   release line only; older `3.x` minor lines do not receive backports.
4. **Companion npm packages move in lockstep.** `@deepnoodle/risor` and the
   four `@deepnoodle/risor-*` packages share one version number set by
   `node web/scripts/version.ts <version>`.
5. **Rust workspace crates move in lockstep.** `risor` and `risor-derive`
   publish to crates.io at `3.0.0` and share the same `3.x.y` version under tag
   `rust-v3.x.y` so `features = ["derive"]` resolves cleanly.
6. **Three-implementation parity for spec releases.** A specification minor
   version (`3.1`, `3.2`, ...) is considered released only when Go, TypeScript,
   and Rust all pass its conformance suite (decision D27).

## 3. What counts as a breaking change

### Breaking (requires a new major version, `4.0`)

- Changing the result, failure code, or error class of any valid `3.x` program.
- Changing the step count or allocation unit count of any valid `3.x` program
  (for runs that succeed or fail with a non-limit code), operator, or library
  entry — including repricing a cost formula or adding an optimization that
  alters counts (D16; usage on limit-code failures stays unspecified per
  `spec/core/03-host.md` 3.6.3).
- Changing a `run`-phase diagnostic message template or rendered text.
- Changing the pinned Unicode 16.0.0 case table (`suite/casemap.json`), the
  25-code-point whitespace set, or number/JSON rendering rules.
- Removing or renaming a library entry, opt-in group, diagnostic code, or
  exported host API symbol, or narrowing the inputs an existing entry or API
  accepts.
- Tightening static or type checking so a previously valid program under a
  pinned `3.x` specification version fails to check (unless fixing a bug where
  an implementation violated the written specification).
- Lowering fixed limits or default limits in a way that rejects previously
  valid `3.x` programs (source nesting limit of 100, source size cap of
  1,048,576 code points, maximum depth limit of 400, default limits of
  1,000,000 steps / 1,000,000 units / depth 200).
- Raising minimum supported language runtimes (Go, Node, Rust MSRV) beyond
  what is needed for security — note: if runtime minimums are bumped in a
  minor release, announce one minor version ahead.

### Not breaking (allowed in `3.x` minor or patch releases)

- Adding syntax, library entries, opt-in groups, optional trailing parameters
  (preserving all existing results and counts), or host API additions — gated
  by the new specification minor version where observable to programs.
- Improving `declare`, `parse`, or `check` diagnostic messages, `help` text, or
  `wrong` / `foreign-names.json` hints.
- Fixing a conformance bug where an implementation diverged from `spec/` and
  `suite/` (even if a broken program relied on the buggy behavior).
- Changing usage counts or spans on runs that fail with a **limit code**
  (`steps-exceeded`, `allocation-exceeded`, `depth-exceeded`,
  `deadline-exceeded`, `cancelled`), which `spec/core/03-host.md` 3.6.1 and
  3.6.3 explicitly leave unspecified.
- Performance and memory optimizations that preserve exact step and unit
  counts and all observable behavior.
- Adding fields to diagnostic objects (bindings, runners, and hosts ignore
  unknown fields per `spec/core/03-host.md` 3.7.1 item 1) or adding optional
  keys to environment descriptions that are omitted when the new feature is not
  used (`spec/packages/f01-host-types.md` 5.3 item 4; under this policy,
  consumers reading environment descriptions must ignore unknown keys).

## 4. Deprecations

### Language and standard library

- Language forms, library entries, and `run` messages are **never removed in
  3.x**.
- When an entry or feature is superseded, the replacement is introduced under a
  new name in a minor version. The old entry is marked deprecated in the
  specification, documentation, and its `doc` string, remains fully supported
  for the rest of `3.x`, and is eligible for removal only in `4.0`.

### Host APIs (Go, TypeScript, Rust)

- A deprecated host API is marked with the ecosystem's standard mechanism:
  - Go: `// Deprecated: ...` doc comment naming the replacement.
  - TypeScript: `@deprecated` JSDoc tag naming the replacement.
  - Rust: `#[deprecated(since = "3.x.0", note = "...")]` attribute.
- Deprecated host APIs remain functional for the remainder of `3.x` and carry
  the deprecation notice for at least one minor release before any `4.0`
  removal.

## 5. Support policy for Risor v2

Risor v2 (`github.com/deepnoodle-ai/risor/v2`, Go only) lives on the `v2`
branch. Existing Go imports of `/v2` continue to work without change.

- **Role:** v2 is a scripting language (statements, loops, mutation, modules).
  v3 is a separate embedded expression language, not an in-place upgrade for
  v2 scripts (`experiments/v3/design/00-overview.md` 5.3, `21-scope.md` 1).
  Scripts that require loops or mutation stay on v2.
- **Maintenance scope:** v2 is in maintenance mode. It receives security fixes
  and critical bug fixes (no new language features). Fixes land via pull
  requests targeting `--base v2` and release as `v2.x.y` tags.
- **Maintenance window:** Risor v2 receives security fixes and critical bug
  fixes for **12 months after 3.0.0 ships**, after which the `v2` branch moves
  to archive / best-effort status.
