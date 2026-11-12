# The npm packages for Risor in the browser

**Status:** Draft
**Author:** Curtis Myzie, with Claude
**Date:** 2026-11-10
**Workflow:** Spec, then build in parallel. The package boundaries need the
owner's eye; the parts inside each package are ordinary front-end work.
**PRD:** `docs/prds/web-packages.md`

## Context

The TypeScript implementation in `ts/` can run, check, format, complete and
hover, but it was private and shipped `.ts` source. The website copies it and
builds an editor, a worker and an output panel by hand. Customers who embed
Risor need the same parts in their own pages. The PRD gives the users and the
use cases; this document gives the packages, their boundaries and their
dependencies.

## Goals

- `npm install risor@alpha` gives the language, built, typed, with no dependencies.
- A Mantine form gets a checked, completing Risor field from one component
  (`RisorInput`) and one environment description.
- No user program runs on the page's main thread. The main thread loads at
  most 20 KB gzipped of Risor (the lexer, for colours).
- One CodeMirror extension holds all editor behaviour. React and Vue only
  wrap it.
- `risor-site-v2` drops `app/vendor/risor-v3` and its hand-built editor.

## Non-goals

- A language server, Monaco, Svelte or Angular wrappers, a VS Code
  extension, a playground component (PRD, "Not in scope").
- Changes to the language. The packages call the public API of `ts/`, plus
  one new call (`tokens`, below).

## Proposal

### The packages

```
risor  (ts/)                       no dependencies
 │                                 entries: risor, risor/tokens, risor/testing
 ├── @risor/service                risor
 │     │
 │     └── @risor/codemirror       risor (risor/tokens only), @risor/service
 │           │                     peers: @codemirror/state, view, language,
 │           │                            autocomplete, lint, commands
 │           ├── @risor/react      @risor/codemirror, @risor/service
 │           │     │               peer: react >=18
 │           │     └── @risor/mantine   @risor/react
 │           │                          peers: react, @mantine/core >=7.17
 │           └── @risor/vue        (later) @risor/codemirror, @risor/service
 │                                 peer: vue >=3.4
 │
 └── @risor/grammar                (later) no dependencies: a TextMate grammar
```

| Package | Holds | Why it is its own package |
| --- | --- | --- |
| `risor` | The language: `createEnv` and its `Env` (`compile`, `check`, `describe`), `Program` (`run`, `measure`), `format`, `env.complete`, `env.hover`, `Env.fromDescription`, `jsonText`: the API of 05 Part E (R200). `risor/tokens`: colours, from the lexer only. | Usable in Node, Deno, Bun and servers, with no browser code. |
| `@risor/service` | The **service**: one interface for check, complete, hover, format and run, with two implementations, in a web worker and on the main thread. The worker script. Position conversion. `describeSample`, which makes a description from sample JSON (the site's `typeOfJSON`, moved). A tree of Risor's JSON text, for result views. | Framework-free. A host on Svelte or plain DOM uses it with `@risor/codemirror` and needs nothing else. |
| `@risor/codemirror` | One CodeMirror 6 extension, `risor(options)`: colours, lint (check), completion, hover, format command, Mod-Enter to run, single-line mode, a theme on CSS variables. | All editor behaviour in one place, shared by React, Vue and plain DOM. |
| `@risor/react` | `RisorEditor`, `RisorResult`, `RisorDiagnostics`, `RisorProvider`; hooks `useRisorService`, `useRisorCheck`, `useRisorRun`. Unstyled except for one optional CSS file. | React is a peer; no styling framework. |
| `@risor/mantine` | `RisorInput` (the editor in Mantine's input shell: label, description, error, required) and `RisorPreview`. | Mantine is the site's framework and a common one for customers; it must not leak into `@risor/react`. |
| `@risor/vue` (later) | `RisorEditor`, `RisorResult` with `v-model`. Written with `defineComponent` and `h`, so the package needs no SFC compiler. | Vue is a peer. |
| `@risor/grammar` (later) | The TextMate grammar of `f12-tools.md` 6, as JSON, and a Shiki language object. | Static pages (Shiki at build time), VS Code and GitHub have no JavaScript runtime of ours. |

`@risor/react` has no `RisorInput`: the input with a label and an error is
Mantine's. A React page without Mantine uses `RisorEditor` with
`singleLine`.

**Why the service is not inside `@risor/codemirror`:** a result view, a
form's submit check and a preview all need the service and no editor. **Why
`@risor/react` depends on `@risor/codemirror` and does not split the editor
out:** every React user of these packages wants the editor; the bundler drops
the editor from a page that imports only `RisorResult` (`sideEffects: false`).

### An environment is a description

Every package names an environment one way: a `Description` (f01 5.3), the
JSON that Go's `Env.Describe()` and TypeScript's `env.describe()` make.

| A host has | It passes |
| --- | --- |
| A Go server with the real environment | `env.Describe()`, sent to the page |
| Options in TypeScript, no functions | `createEnv(options).describe()` |
| Only sample data | `describeSample(sample, { groups })`, from `@risor/service`. `groups` names the opt-in library groups (F6) the program may use. |
| Nothing | No `environment`: an empty one |

The service checks the value with `Env.fromDescription`. A value that is not
a description is a console error that names the prop, and the service uses
an empty environment. A description carries steps, allocation and depth
limits; they apply to every run. It carries no deadline, so a run takes
`timeoutMs` from its caller (default 2,000 ms).

### The service (`@risor/service`)

```ts
interface RisorService {
  /** The service keeps the environment, so each keystroke does not rebuild it. */
  environment(description?: Description): RisorEnvironment;
  format(text: string): Promise<FormatResult>;
  dispose(): void;
}

interface RisorEnvironment {
  check(text: string, options?: { expected?: string }): Promise<Problem[]>;
  complete(text: string, offset: number): Promise<Completion | null>;
  hover(text: string, offset: number): Promise<Hover | null>;
  run(text: string, vars: Record<string, unknown>,
      options?: { expected?: string; check?: boolean; timeoutMs?: number }): Promise<RunResult>;
  release(): void;
}

function workerService(options?: { worker?: () => Worker; graceMs?: number }): RisorService;
function localService(): RisorService;   // main thread; for Node, tests, and small pages
function describeSample(sample: Record<string, unknown>, options?: { groups?: string[] }): Description;
```

- **Positions are UTF-16 offsets** at this interface (PRD R-4). The service
  converts to and from Risor's code-point offsets and positions. A
  `Problem` carries `from`, `to` (UTF-16), and `line`, `column` (1-based,
  code points, as Risor prints them) for display.
- **`RunResult` is JSON** with a documented shape: `{ ok: true, json, type,
  usage, ms }` or `{ ok: false, stage, problems, usage?, ms? }`, where
  `stage` is `declare`, `compile`, `run` or `stopped`. `json` is Risor's
  JSON text of the value. A host's server can make the same shape, which is
  what the preview's `run` callback returns (PRD UC-2).
- **A released environment opens again on its next call.** React's
  StrictMode releases in an effect's cleanup and then uses the same object;
  the browser test found this.
- **The worker client** sends each request with an id and keeps one timer
  per run: its `timeoutMs` plus a grace (default 1,500 ms, as the site uses
  today). The timer starts when the run is sent to a worker that has
  answered once. A new worker gets each environment's `open` before any
  other request, so the first answer comes once the worker has loaded, and
  its download and start do not count against the run. A worker that fails
  to load fails every call (`onerror`). A worker that does not answer
  within `loadTimeoutMs` (default 30,000 ms) is ended, and every pending
  call fails; the next call starts a new worker. A value `postMessage`
  cannot clone fails its call with a result. A run's timer is clamped to
  `setTimeout`'s range. On expiry the client terminates
  the worker and resolves the run as stopped (stage `stopped`). It starts a
  new worker, sends it each live environment's description, and sends it
  again every other request that was in flight: a check, complete, hover,
  validate, format or queued run. Each of these settles from the new
  worker, so no promise waits forever. A test holds a check in flight
  while a run times out, and gets the check's answer from the new worker. Check, complete and
  hover have no deadline of their own (#635); the checker is bounded by the source
  size.
- **Each editor has its own environment.** `service.environment(d)` returns
  a new object on each call, and each `risor(...)` extension and each React
  hook calls it once. Two editors with the same description never supersede
  each other's checks.
- **Stale requests are dropped.** A newer check, complete or hover for the
  same environment supersedes an older one of the same kind that has not
  yet gone to the worker. The older call resolves `null`, and the caller
  ignores it. A fast typist does not queue work. `validate` is never
  superseded and never resolves `null`, and `validateRisor` uses it, so a
  lint keystroke cannot drop a submit's check.
- **The worker script** is loaded with
  `new Worker(new URL("./worker.js", import.meta.url), { type: "module" })`.
  Vite, webpack 5, Rollup and esbuild with a plugin understand this form.
  `options.worker` lets a host build the worker itself when its bundler
  does not.
- **A description with a function or a method** compiles nothing (f01 5.4
  item 4).
  `run` returns that as one problem that says the environment's functions
  run only on the host, and names the `run` callback. `check`, `complete`
  and `hover` work.

### The editor (`@risor/codemirror`)

```ts
function risor(options: {
  service?: RisorService;            // default: one shared workerService(), made on first use
  environment?: Description;
  expected?: string;                 // type the program must have, e.g. "bool"
  singleLine?: boolean;
  onDiagnostics?: (p: Problem[]) => void;
  onRun?: () => void;                // Mod-Enter, and Enter in single-line mode
  lintDelay?: number;                // default 300 ms
}): Extension;

function validateRisor(view: EditorView): Promise<Problem[]>;  // checks the text now; skips the delay
function formatRisor(view: EditorView): Promise<boolean>;      // a command
const risorTheme: Extension;   // colours from --risor-* CSS variables, light and dark defaults
```

- **Fixed options.** The extension's options are fixed when it is made. To
  change the environment or the expected type, a host puts `risor(...)` in a
  `Compartment` and reconfigures it; `RisorEditor` does this when its props
  change.
- **Colours.** A `ViewPlugin` calls `tokens(text)` from `risor/tokens` on
  the main thread and builds mark decorations. It is not a Lezer grammar,
  so the colours always match the lexer (PRD R-5). Lexing is linear, and
  Risor texts are expressions and configs, not large files; it reruns on
  each change (20 KB of text lexes in about 3 ms, measured on the suite's
  largest program). A text that does not lex is coloured to the error; the
  rest of that line is `invalid`, and colouring starts again on the next
  line. `tokens` counts code points; the plugin converts each offset to
  UTF-16 with the service's `TextIndex`, the one conversion R-4 asks for. A
  test puts an astral character before other tokens.
- **Lint** uses `@codemirror/lint` with the service's `check`.
  **Completion** uses `@codemirror/autocomplete` with `complete`; a
  candidate's `kind` picks the icon and `detail` shows its type. **Hover**
  uses `hoverTooltip` with `hover`, and shows the signature, type and doc.
- **`validateRisor`** sends a check of the current text at once and
  resolves with its problems, so a form's submit never reads stale results
  (PRD UC-1).
- **Single-line mode:** a transaction filter replaces a newline with a
  space, and Enter calls `onRun`.

### The new core call: `tokens` (in review in #631)

```ts
// risor/tokens only: the main entry is exactly 05 Part E
export type TokenClass = "keyword" | "constant" | "name" | "function" | "property"
  | "number" | "string" | "template" | "interpolation" | "operator"
  | "punctuation" | "comment" | "invalid";
export interface ColourToken { class: TokenClass; start: number; end: number }  // code points
export function tokens(text: string): ColourToken[];
```

It flattens the lexer's tokens, including those inside template
interpolations, and colours the comments in the gaps between tokens. It
never throws. Its tests check that over every program in the suite the
ranges are in order, do not overlap, and cover every code point that is
not white space. A leading byte order mark is in no range. `risor/tokens`
is 12 KB gzipped (measured with esbuild 0.25.0 on #631; most of it is the message table the lexer imports). It is a
TypeScript binding call, not a spec call (PRD question 2).

### The React package

```tsx
<RisorProvider service={workerService()}>   {/* optional: one shared service */}
  <RisorEditor ref={ref} value={code} onChange={setCode} environment={description}
               expected="bool" onDiagnostics={setProblems} minLines={6} />
  <RisorResult result={runResult} />         {/* tree, type, usage, or problems */}
  <RisorDiagnostics problems={problems} text={code} />
</RisorProvider>

await ref.current.validate();                  // fresh problems, for a submit
const { result, running } = useRisorRun({ code, environment, vars, debounce: 400, run? });
const { problems, validate } = useRisorCheck({ code, environment, expected: "bool" });
```

- `RisorEditor` is controlled. It creates the `EditorView` in an effect and
  maps prop changes to transactions and effects, never to a new view.
- **SSR.** On the server and before mount, `RisorEditor` renders a `<pre>`
  with the text, the editor's padding and line height, so the page does not
  shift when CodeMirror takes over.
- `useRisorRun` takes an optional `run(code, vars)` that returns a
  `RunResult`. With it, the hook calls the host instead of the worker. Only
  the newest call's result is shown; an older answer that arrives later is
  dropped. A rejected promise, or a value that is not a `RunResult`, shows
  as an error result. The host's server owns the deadline of its own runs:
  the hook cannot stop work on the server, so it sets no timer.
- `RisorResult` renders the JSON tree from `@risor/service`, so a number
  shows the exact text Risor gave it. It shows the type, steps and
  allocation, and for an error the code, message, a caret under the line,
  and the trace.
- Styling is by CSS variables (`--risor-*`) and an optional
  `@risor/react/styles.css`.

### The Mantine package

`RisorInput` puts `RisorEditor` (single-line by default, `multiline` to
allow lines) inside `Input.Wrapper` and copies the look of Mantine's `Input`
(border, radius, focus ring, error colour) through Mantine's CSS variables,
so it sits in a Mantine form beside a `TextInput`. It works with
`@mantine/form`'s `getInputProps` (`value`, `onChange`, `error`). An `error`
prop wins over the checker's message. Its ref has `validate()`.
The editable element gets the label as its accessible name
(`aria-labelledby`), and an error sets `aria-invalid` and links the message
(`aria-describedby`); a `contenteditable` element is not named by `for`.
`RisorPreview` is `useRisorRun` and `RisorResult` in a `Paper`, with the
same optional `run`.

### Where the code lives

```
ts/                       the `risor` package: gains a build, `tokens`, `risor/tokens`
web/
  package.json            npm workspaces: packages/*, examples/*, ../ts
  packages/service/
  packages/codemirror/
  packages/react/
  packages/mantine/
  examples/react-vite/    one app that uses every package
```

`ts/` stays where it is: the conformance runner and CI already use it. It
becomes publishable (built, step 2):

- `npm run build` runs `tsc -p tsconfig.build.json` into `ts/dist/`, `.js`
  and `.d.ts`, with `rewriteRelativeImportExtensions`.
- `exports` maps `.`, `./tokens` and `./testing` to `dist/` for consumers,
  with a `risor-source` condition to the `.ts` source, which the workspace's
  TypeScript and Vite configs turn on, so `web/` builds against the source
  with no build step in between. npm workspaces accept `../ts` as a member
  (tried with npm 11).
- `private` is removed; `files` lists `dist`, `src`, `README.md` and
  `LICENSE` (copied from the root at pack time).

Each package in `web/` builds with `tsc` alone. None needs a bundler: CSS is
a plain file and the editor theme is CodeMirror's own `EditorView.theme`.

### Tests

| Package | Tests |
| --- | --- |
| `risor` | The suite, as today. `tokens`: unit tests over every class, and the cover test over every program of the suite. CI packs the package. |
| `@risor/service` | `node --test` on `localService`: position conversion with astral characters, the JSON tree, `describeSample` (with the cases of the site's `typeOfJSON`). The worker client against a fake `Worker`: timeout, restart, re-sent environments, superseded requests. |
| `@risor/codemirror` | `node --test` with an `EditorState` (no DOM): decorations for each class, lint ranges in UTF-16, single-line filter. |
| `@risor/react`, `@risor/mantine` | Server rendering to a string; one browser test in Playwright over `examples/react-vite`: type, see a mark, accept a completion, validate on submit, run, see the tree, stop a run that loops. A size check: esbuild bundles `RisorInput` with CodeMirror, React and Mantine external, so it measures the Risor packages and `risor/tokens`. It is at most 20 KB gzipped. |

CI gains one job, `web`, that runs `npm ci`, `build`, `test` and the
Playwright test, on changes to `ts/` or `web/`.

### Release

- One version for all packages, the alpha's: `3.0.0-alpha.N` (PRD R-7). A
  script sets it in every `package.json` and the internal dependency ranges.
- A GitHub Actions workflow publishes on the tag `v3.0.0-alpha.N` with
  `npm publish --provenance --tag alpha`, after the `web` job passes. No
  version is `latest` until 3.0.0, so installs name the tag:
  `npm install risor@alpha`.
- Until the first publish, the site depends on the packages through a local
  link for development and stays on its vendored copy in production.

### Order of work, one pull request each

1. This PRD and spec.
2. `ts/`: the build, `exports`, `tokens` and `risor/tokens`. (#631.)
3. `@risor/service` and `@risor/codemirror`.
4. `@risor/react` and `examples/react-vite`.
5. `@risor/mantine`.
6. The release workflow, and the first publish.
7. `risor-site-v2`: move to the packages (in that repository).
8. `@risor/vue`.
9. `@risor/grammar`, and Shiki on the site.

## Alternatives considered

1. **One package, `risor`, with subpath exports** (`risor/react`,
   `risor/codemirror`, ...). One name, one version, nothing to keep in step.
   Rejected: the peer dependencies would be optional peers of one package,
   and a Node user who installs `risor` to run programs would see warnings
   or installs for React, Mantine and six CodeMirror packages. Separate
   packages make each peer real.
2. **A Lezer grammar for the colours**, the usual way for CodeMirror. It
   gives incremental parsing and folding. Rejected: it is a second grammar
   that must track the lexer by hand, for texts that are small enough to
   lex whole. Risor already has a lexer that the suite tests.
3. **Fold `@risor/service` into `@risor/codemirror`.** Rejected: a result
   view or a submit check would then pull in CodeMirror.
4. **Headless React only, no Mantine package.** The site and the first
   customers use Mantine; making each of them restyle the field is the work
   the PRD is meant to remove. The Mantine package is small because it only
   composes `@risor/react`.
5. **An `environment` prop that also takes options or sample JSON.**
   Rejected: a description fits the options shape by structure, so a wrong
   pass type-checks and silently drops the functions. One shape, and a
   helper for each source, cannot be passed wrongly.

## Tradeoffs and consequences

- **Six packages to version, build and publish** (eight with Vue and the
  grammar). The lockstep version and one workflow keep the cost down, but a
  fix in one package bumps all.
- **The core grows a public call.** `tokens` becomes API that a later
  release must keep, and Go and Rust do not have it.
- **The worker URL pattern is a bundler contract.** Most bundlers support
  it; some setups (older Next.js configs, Jest) do not. The `worker` option
  is the escape hatch, and `localService` is the fallback.
- **Colours on every keystroke lex the whole text.** Fine for expressions
  and configs. A 5,000-line text would feel it; Risor programs are not that
  shape.
- **A preview with host functions needs the host's server.** The packages
  make that one callback, but the host writes the endpoint.

## Open questions

1. **Is `tokens` a TypeScript-only call, or an F12 spec call** that Go and
   Rust must offer too? Recommendation: TypeScript only. The owner decides;
   it does not block the build.
2. **Names.** `risor` and `@risor/*` (PRD question 1). The owner claims
   them on npm before step 6.
3. **Mantine range.** Peer `>=7.17` (PRD question 4). Tests run on 7.17
   today; a run on the latest waits for the owner's answer. Leaning to the
   range: the site is on 7.
4. **Should a description with functions compile** and fail only at a call
   it cannot make (f01 5.4 item 4)? A spec change, for the owner (PRD
   question 6). The `run` callback does not depend on it.
