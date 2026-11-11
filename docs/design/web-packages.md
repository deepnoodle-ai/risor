# The npm packages for Risor in the browser

**Status:** Draft
**Author:** Curtis Myzie, with Claude
**Date:** 2026-11-10
**Workflow:** Spec, then build in parallel. The package boundaries need the
owner's eye; the parts inside each package are ordinary front-end work.
**PRD:** `docs/prds/web-packages.md`

## Context

The TypeScript implementation in `ts/` can run, check, format, complete and
hover, but it is private and ships `.ts` source. The website copies it and
builds an editor, a worker and an output panel by hand. Customers who embed
Risor need the same parts in their own pages. The PRD gives the users and the
use cases; this document gives the packages, their boundaries and their
dependencies.

## Goals

- `npm install risor` gives the language, built, typed, with no dependencies.
- A React page gets a checked, completing Risor field from one component
  (`RisorInput`) and one environment description.
- No user program runs on the page's main thread.
- One CodeMirror extension holds all editor behaviour. React and Vue only
  wrap it.
- `risor-site-v2` drops `app/vendor/risor-v3` and its hand-built editor.

## Non-goals

- A language server, Monaco, Svelte or Angular wrappers, a VS Code extension
  (PRD, "Not in scope").
- Changes to the language. The packages only call the public API of `ts/`,
  plus one new call (`tokens`, below).

## Proposal

### The packages

```
risor  (ts/)                       no dependencies
 │
 ├── @risor/worker                 risor
 │     │
 │     └── @risor/codemirror       risor, @risor/worker
 │           │                     peers: @codemirror/state, view, language,
 │           │                            autocomplete, lint, commands
 │           ├── @risor/react      @risor/codemirror, @risor/worker
 │           │     │               peer: react >=18
 │           │     └── @risor/mantine   @risor/react
 │           │                          peers: react, @mantine/core >=7.17
 │           └── @risor/vue        @risor/codemirror, @risor/worker
 │                                 peer: vue >=3.4
 │
 └── @risor/grammar                no dependencies (data: a TextMate grammar)
```

| Package | Holds | Why it is its own package |
| --- | --- | --- |
| `risor` | The language: `createEnv`, `compile`, `run`, `check`, `formatProgram`, `complete`, `hover`, `envFromDescription`, and the new `tokens`. | Usable in Node, Deno, Bun and servers, with no browser code. |
| `@risor/worker` | The **service**: one interface for check, complete, hover, format and run, with two implementations, in a web worker and on the main thread. The worker script. Position conversion. Variable types from sample JSON (`typeOfJSON`, moved from the site). A JSON-text tree that keeps number text, for result views. | Framework-free. A host on Svelte or plain DOM uses it with `@risor/codemirror` and needs nothing else. |
| `@risor/codemirror` | One CodeMirror 6 extension, `risor(options)`: colours, lint (check), completion, hover, format command, Mod-Enter to run, a theme on CSS variables. | All editor behaviour in one place, shared by React, Vue and plain DOM. |
| `@risor/react` | `RisorEditor`, `RisorInput`, `RisorResult`, `RisorDiagnostics`, `RisorProvider`; hooks `useRisorService`, `useRisorCheck`, `useRisorRun`. Unstyled except for one optional CSS file. | React is a peer; no styling framework. |
| `@risor/mantine` | `RisorInput` in Mantine's input shell (label, description, error, required), `RisorPreview`, `RisorPlayground`. | Mantine is the site's framework and a common one for customers; it must not leak into `@risor/react`. |
| `@risor/vue` | `RisorEditor`, `RisorInput`, `RisorResult` with `v-model`. Written with `defineComponent` and `h`, so the package needs no SFC compiler. | Vue is a peer. |
| `@risor/grammar` | The TextMate grammar of `f12-tools.md` 6, as JSON, and a Shiki language object. | Static pages (Shiki at build time), VS Code and GitHub have no JavaScript runtime of ours. |

**Why the service is not inside `@risor/codemirror`:** a result view, a
form's submit check and a preview all need the service and no editor. **Why
`@risor/react` depends on `@risor/codemirror` and does not split the editor
out:** every React user of these packages wants the editor; the bundler drops
the editor from a page that imports only `RisorResult` (`sideEffects: false`).

### The service (`@risor/worker`)

```ts
interface RisorService {
  /** An environment the service keeps, so each keystroke does not rebuild it. */
  environment(spec: EnvSpec): RisorEnvironment;
  format(text: string): Promise<FormatResult>;
  dispose(): void;
}

interface RisorEnvironment {
  check(text: string, expected?: string): Promise<Problem[]>;
  complete(text: string, offset: number): Promise<Completion | null>;
  hover(text: string, offset: number): Promise<Hover | null>;
  run(text: string, vars: Record<string, unknown>, opts?: { expected?: string; check?: boolean }): Promise<RunResult>;
  release(): void;
}

/** JSON only, so it can cross to the worker. */
type EnvSpec =
  | { description: Description; limits?: Limits }   // from Go's Env.Describe()
  | { vars?: string; variables?: Record<string, string>;
      schemas?: Record<string, { schema: unknown }>; groups?: string[];
      types?: ...; limits?: Limits }                   // EnvOptions without functions or opaque
  | { sample: Record<string, unknown>; groups?: string[]; limits?: Limits };  // types inferred

function workerService(options?: { worker?: () => Worker; graceMs?: number }): RisorService;
function localService(): RisorService;   // main thread; for Node, tests, and tiny pages
```

- **Positions are UTF-16 offsets** at this interface (PRD R-4). The service
  converts to and from Risor's code-point offsets and line/column positions.
  `Problem` carries `from`, `to` (UTF-16), and `line`, `column` (1-based,
  code points, as Risor prints them) for display.
- **The worker client** sends each request with an id and keeps one timer
  per run: the run's deadline plus a grace (default 1,500 ms, as the site
  uses today). On expiry it terminates the worker, resolves the run as
  `{ ok: false, stopped: true }`, starts a new worker on the next request,
  and re-sends each live environment's spec. Check, complete and hover have
  no deadline of their own; the checker is bounded by the source size.
- **Stale requests are dropped.** The client tags each check, complete and
  hover with the text's version. A newer request supersedes an older one of
  the same kind for the same environment, so a fast typist does not queue
  work.
- **The worker script** is loaded with
  `new Worker(new URL("./worker.js", import.meta.url), { type: "module" })`.
  Vite, webpack 5, Rollup and esbuild with a plugin understand this form.
  `options.worker` lets a host build the worker itself when its bundler
  does not.
- **A description with a function** cannot run (f01 5.4 item 4). `run`
  returns that as a `declare` problem with the core's message; `check`,
  `complete` and `hover` work.

### The editor (`@risor/codemirror`)

```ts
function risor(options: {
  service?: RisorService;            // default: one shared workerService()
  environment?: EnvSpec;
  expected?: string;                 // type the program must have, e.g. "bool"
  onDiagnostics?: (p: Problem[]) => void;
  onRun?: () => void;                // Mod-Enter
  lintDelay?: number;                // default 300 ms
}): Extension;

function setRisorEnvironment(view: EditorView, spec: EnvSpec): void;  // a StateEffect underneath
function formatRisor(view: EditorView): Promise<boolean>;             // a command
const risorTheme: Extension;   // colours from --risor-* CSS variables, light and dark defaults
```

- **Colours.** A `ViewPlugin` calls `tokens(text)` from `risor` on the main
  thread and builds mark decorations. It is not a Lezer grammar, so the
  colours always match the lexer (PRD R-5). Lexing is linear and the texts
  are expressions and configs, not large files; it reruns on each change.
  The gaps between tokens hold white space and comments; a gap that is not
  all white space is coloured as a comment. A text that does not lex keeps
  the colours of its last good prefix.
- **Lint** uses `@codemirror/lint` with the service's `check`. **Completion**
  uses `@codemirror/autocomplete` with `complete`; a candidate's `kind`
  picks the icon and `detail` shows its type. **Hover** uses `hoverTooltip`
  with `hover`, and shows the signature, type and doc.
- **Single-line mode** (`RisorInput`): a transaction filter replaces a
  newline with a space when `multiline: false`, and Enter calls `onRun`.

### The new core call: `tokens`

```ts
// risor
export interface ColourToken { kind: TokenClass; start: number; end: number }  // code points
export type TokenClass = "keyword" | "word" | "constant" | "number" | "string"
  | "template" | "escape" | "punct" | "operator" | "comment" | "invalid";
export function tokens(text: string): ColourToken[];
```

It flattens the lexer's tokens, including those inside template
interpolations, adds the comments from the gaps, and stops at the first
lexing error with an `invalid` token to the end. It is a TypeScript binding
call, not a spec call (open question 1).

### The React package

```tsx
<RisorProvider service={workerService()}>   {/* optional: one shared service */}
  <RisorEditor value={code} onChange={setCode} environment={spec}
               expected="bool" onDiagnostics={setProblems} minLines={6} />
  <RisorInput value={expr} onChange={setExpr} environment={spec} />
  <RisorResult result={runResult} />         {/* tree, type, usage, or problems */}
  <RisorDiagnostics problems={problems} text={code} />
</RisorProvider>

const { result, running } = useRisorRun({ code, environment: spec, vars, debounce: 400 });
const problems = useRisorCheck({ code, environment: spec, expected: "bool" });
```

- `RisorEditor` is controlled. It creates the `EditorView` in an effect and
  maps prop changes to transactions and effects, never to a new view.
- **SSR.** On the server and before mount, `RisorEditor` renders a
  `<pre>` with the text, the editor's padding and line height, so the page
  does not shift when CodeMirror takes over.
- `RisorResult` renders the JSON tree from `@risor/worker`, so a number shows
  the exact text Risor gave it. It shows the type, steps and allocation, and
  for an error the code, message, a caret under the line, and the trace.
- Styling is by CSS variables (`--risor-*`) and an optional
  `@risor/react/styles.css`.

### The Mantine package

`RisorInput` puts `RisorEditor` inside `Input.Wrapper` and copies the look of
Mantine's `Input` (border, radius, focus ring, error colour) through
Mantine's CSS variables, so it sits in a Mantine form beside a `TextInput`.
It works with `@mantine/form`'s `getInputProps` (`value`, `onChange`,
`error`). `RisorPreview` is `useRisorRun` and `RisorResult` in a `Paper`.
`RisorPlayground` is the site's playground made generic: editor, variables,
check switch, limits, format, result.

### Where the code lives

```
ts/                       the `risor` package (exists): gains a build and `tokens`
web/
  package.json            npm workspaces: packages/*, examples/*, ../ts
  packages/worker/
  packages/codemirror/
  packages/react/
  packages/mantine/
  packages/vue/
  packages/grammar/
  examples/react-vite/    one app that uses every React package
```

`ts/` stays where it is: the conformance runner and CI already use it. It
becomes publishable:

- `npm run build` runs `tsc` with `rewriteRelativeImportExtensions` into
  `ts/dist/` (`.js` and `.d.ts`). It leaves out `src/suite/` and
  `src/testing.ts`, which import `node:fs`. (Tried: the emit works; the
  build `tsconfig` needs `lib: ["es2024", "dom"]` or small ambient types for
  `AbortSignal`, `TextEncoder` and `TextDecoder`.)
- `exports` maps `.` to `dist/` for consumers, with a `risor-source`
  condition to `src/index.ts`, which the workspace's TypeScript and Vite
  configs turn on, so `web/` builds against the source with no build step
  in between.
- `private` is removed; `files` lists `dist`, `README.md`, `LICENSE`.

Each package in `web/` builds with `tsc` alone. None needs a bundler: CSS is
a plain file and the editor theme is CodeMirror's own `EditorView.theme`.

### Tests

| Package | Tests |
| --- | --- |
| `risor` | The suite, as today. `tokens` gets unit tests over every token class and a property test: the tokens and gaps cover the text exactly. |
| `@risor/worker` | `node --test` on `localService`: position conversion with astral characters, the JSON tree, `typeOfJSON` (moved with its cases from the site's `check:v3`). The worker client against a fake `Worker`: timeout, restart, re-sent environments, dropped stale requests. |
| `@risor/codemirror` | `node --test` with an `EditorState` (no DOM): decorations for a sample of each token class, lint ranges, completion at a cut. |
| `@risor/react`, `@risor/vue` | Server rendering to a string; one browser test each in Playwright over `examples/`: type, see a mark, accept a completion, run, see the tree. |
| `@risor/grammar` | Scope tests with `vscode-textmate` (f12 6 item 5), and every example in `app/content/v3` of the site lexes with no `invalid` scope. |

CI gains one job, `web`, that runs `npm ci`, `build`, `test` and the
Playwright test, on changes to `ts/` or `web/`.

### Release

- One version for all packages, the alpha's: `3.0.0-alpha.N` (PRD R-7). A
  script sets it in every `package.json` and the internal dependency ranges.
- A GitHub Actions workflow publishes on the tag `v3.0.0-alpha.N` with
  `npm publish --provenance --tag alpha`, after the `web` job passes.
- Until the first publish, the site depends on the packages through a local
  link for development and stays on its vendored copy in production.

### Order of work, one pull request each

1. This PRD and spec.
2. `ts/`: the build, `exports`, and `tokens`.
3. `@risor/worker` and `@risor/codemirror`.
4. `@risor/react` and `examples/react-vite`.
5. `@risor/mantine`.
6. `@risor/grammar`.
7. The release workflow, and the first publish.
8. `risor-site-v2`: move to the packages (in that repository).
9. `@risor/vue`.

## Alternatives considered

1. **One package, `risor`, with subpath exports** (`risor/react`,
   `risor/codemirror`, ...). One name, one version, nothing to keep in step.
   Rejected: the peer dependencies would be optional peers of one package,
   and a Node user who installs `risor` to run programs would see warnings
   or installs for React, Vue, Mantine and six CodeMirror packages. Separate
   packages make each peer real.
2. **A Lezer grammar for the colours**, the usual way for CodeMirror. It
   gives incremental parsing and folding. Rejected: it is a second grammar
   that must track the lexer by hand, for texts that are small enough to
   lex whole. Risor already has a lexer that the suite tests.
3. **Fewer packages: fold `@risor/worker` into `@risor/codemirror`.**
   Rejected: a result view or a submit check would then pull in CodeMirror.
4. **Headless React only, no Mantine package.** The site and the first
   customers use Mantine; making each of them restyle the field is the work
   the PRD is meant to remove. The Mantine package is small because it only
   composes `@risor/react`.

## Tradeoffs and consequences

- **Seven packages to version, build and publish.** The lockstep version
  and one workflow keep the cost down, but a fix in one package bumps all.
- **The core grows a public call.** `tokens` becomes API that a later
  release must keep, and Go and Rust do not have it.
- **The worker URL pattern is a bundler contract.** Most bundlers support
  it; some setups (older Next.js configs, Jest) do not. The `worker` option
  is the escape hatch, and `localService` is the fallback.
- **Colours on every keystroke lex the whole text.** Fine for expressions
  and configs. A 5,000-line text would feel it; Risor programs are not that
  shape, and the measurement goes in the codemirror tests as a guard.
- **141 KB gzipped core.** It loads in the worker. The main thread also
  loads it for `tokens`. A follow-up can split `tokens` and the lexer into a
  small entry (`risor/lexer`) so the main thread loads only that.

## Open questions

1. **Is `tokens` a TypeScript-only call, or an F12 spec call** that Go and
   Rust must offer too? Recommendation: TypeScript only for now, since only
   browser editors need it. The owner decides; it does not block the build.
2. **Names.** `risor` and `@risor/*` (PRD question 1). The owner claims
   them on npm before step 7.
3. **Mantine range.** Peer `>=7.17`, tested on 7.17 and the latest, or
   latest only (PRD question 4). Leaning to the range: the site is on 7.
4. **Should `run` in the worker accept a module of host functions** now, so
   a customer preview can call functions? Recommendation: no, not in the
   first release (PRD question 6). Revisit after the first customer.
