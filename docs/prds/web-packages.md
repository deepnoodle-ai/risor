# Risor in the browser: npm packages for running, editing and showing Risor
Status: Draft
PRD PR: #TBD  Implementation PR: #TBD  Updated: 2026-11-10 (after the PRD review)

## Problem

A product that embeds Risor lets its own users write Risor: a condition on a
workflow step, a template, a query over JSON, a config value. Those users
write the expression in a web page. Today a host has nothing to give them
but a plain text box. The user learns about a typo, an unknown field or a
wrong type only after they save and the server runs the expression.

The Risor language already has every part an editor needs. The TypeScript
implementation in `ts/` runs, checks, formats, completes and hovers, and it
gives the same answers as the Go implementation, because the conformance
suite tests both (`spec/packages/f12-tools.md` 1.2). But none of it is
installable:

- `ts/package.json` is `private`, and its exports point at `.ts` source.
  No one outside this repository can `npm install` it.
- The website (`risor-site-v2`) copies `ts/src` into `app/vendor/` with a
  script, then builds its own editor, web worker, runner, error display and
  output display on top (about 1,400 lines in `app/components/v3/`,
  `app/lib/v3/engine.ts` and `app/routes/v3.play.tsx`). A customer who wants
  the same thing must copy it again.
- The site's editor is a `<textarea>` under a coloured copy, with a
  regular-expression highlighter. It has no completion, no hover and no
  marks at the place of an error.

If we do nothing, each customer builds a weaker editor, or none, and the
promise of v3 ("checked before it runs") stops at the server.

## Context

What exists in this repository:

| Part | Where | State |
| --- | --- | --- |
| Run, check, compile, limits | `ts/src/host/api.ts` | Done, passes Core and the packages built so far |
| Format a program or template | `formatProgram`, `formatTemplate` (F12 2) | Done |
| Completion and hover at a position | `complete`, `hover` (F12 3) | Done |
| An environment as JSON | Go `Env.Describe()`, TS `env.describe()`, TS `envFromDescription()` (F1 5.3, 5.4) | Done |
| The editor grammar (TextMate) | `spec/packages/f12-tools.md` 6 | Specified, not built |
| A language server | Dropped (D24) | Completion, hover and describe are calls instead |

The description is the key fact. A Go server can send `env.Describe()` to the
browser. The browser builds the same environment from it and checks the
user's text against the exact variables, functions and types the server will
use. No other part needs to agree by hand.

The bundle cost: `ts/src` built and minified as one ES module is 420 KB, or
141 KB gzipped (ran: tsc 5.9.3 and esbuild 0.25.0, 2026-11-10). The largest
parts are the math tables (62 KB) and the case map (37 KB).

How comparable projects do it (from memory, not re-checked):

- **CEL** has no official browser editor. Teams that want one write it.
- **JSONata** ships the engine on npm. Its exerciser site has its own Monaco
  editor that is not a package.
- **Monaco and CodeMirror language packages** (for example
  `@codemirror/lang-javascript`) ship highlighting, completion and lint as
  one editor extension, with the framework wrappers as separate packages.

## Users and job

Primary user: **a front-end developer at a company that embeds Risor.** Their
job: "Let my users write a Risor expression in my app, with completion, with
errors shown where they are while they type, and with a preview of the result
against sample data, without my team building an editor." We optimize for
this user.

Secondary users:

- **The Risor website** (`risor-site-v2`): the playground, the "Try it"
  buttons on every example, and the home page's live example.
- **A developer evaluating Risor** in a browser or in Node, who wants
  `npm install` and a five-line example.

## Concepts

| Concept | Definition | Relation |
| --- | --- | --- |
| Risor package (`risor`) | The language for JavaScript: run, check, format, complete, hover. | The TypeScript implementation in `ts/`, built and published. |
| Environment description | JSON that says which variables, functions and types a host gives a program. | Already exists (F1 5.3). The server makes it; the browser reads it. |
| Environment description | (above) | The one way to tell every package what a program may use. Made by Go's `Env.Describe()`, by `createEnv(...).describe()`, or from sample JSON by `describeSample`. |
| Risor service | Checks, completes, hovers, formats and runs Risor away from the page's main thread, and ends a run that does not stop. | New, in `@risor/service`. Replaces the site's `useRunner` and worker. |
| Risor editor | A code editor for Risor with colours, completion, hover, error marks and format. | New. A CodeMirror 6 extension, wrapped for React (and later Vue). |
| Risor input | A form field for one expression, with a label, a description and an error, in Mantine's style. | New, in `@risor/mantine`. The editor in the shape of a form control. |
| Result view | Shows a run's value as a tree, its type, its steps and allocation, or its error. | New. |

## Use cases

### UC-1: Add a checked expression field to a form

A customer's Go server stores workflow steps. Each step has a condition.

1. The server sends the environment description with the form:
   `env.Describe()`.
2. The page renders:

   ```tsx
   import { RisorInput } from "@risor/mantine";

   const ref = useRef<RisorInputHandle>(null);

   <RisorInput
     ref={ref}
     label="Run this step when"
     description="A Risor expression. It must give a bool."
     environment={description}
     expected="bool"
     {...form.getInputProps("condition")}
   />
   ```

3. The user types `order.totl > 100`. Within 300 ms the word `totl` is
   underlined and the field shows the checker's message for the missing
   field. The user types `order.`, and a list of the fields of `order`
   appears.
4. On submit, the form awaits `ref.current.validate()`. It checks the text
   as it is now, not as it was at the last pause, and returns the problems.
   The form blocks submit while there is one.
5. The server checks the text again before it saves. The field helps the
   user; it is not the gate.

Acceptance:

- [ ] With a description from Go's `Env.Describe()`, the field completes
      the variables, functions and record fields that the description
      declares.
- [ ] A check error is marked at its span, and its message is the message
      the Go server would give for the same text (the suite makes the two
      agree).
- [ ] `expected="bool"` makes `order.total` (a float) an error.
- [ ] Text typed and submitted at once, before the pause ends, is blocked
      by `validate()` if it has an error.
- [ ] The field works with no `environment`: it then checks against an
      empty environment.
- [ ] `environment` takes a description and nothing else. A value that is
      not a description is a console error that names the prop, and the
      field checks against an empty environment.
- [ ] The field's code on the main thread, without CodeMirror, is at most
      20 KB gzipped. The rest of Risor loads in the worker.
- [ ] The field renders on the server (SSR) as a plain text box with the
      value, and becomes the editor on the client.

### UC-2: Preview the result against sample data

1. The page adds `<RisorPreview code={value} environment={description}
   vars={sample} />`.
2. The user changes the expression. After a pause, the preview shows the
   value as a tree, its type, the steps and allocation used, or the error
   with its line and column.
3. The user writes an expression that runs past the limits. The preview
   shows the limit error. The page does not freeze.

A real host declares functions, and the browser has no implementation of
them. An environment made from a description that declares a function
compiles nothing (f01 5.4 item 4). So a customer whose environment has
functions gives the preview a `run` callback, and runs the preview on their
own server with the real environment:

```tsx
<RisorPreview code={value} vars={sample}
  run={(code, vars) => fetch("/api/risor/preview", { method: "POST",
    body: JSON.stringify({ code, vars }) }).then((r) => r.json())} />
```

Acceptance:

- [ ] Without `run`, every run happens in a web worker. A run that does
      not answer within its deadline (default 2,000 ms) plus a grace of
      1,500 ms ends the worker and shows "stopped".
- [ ] The description's step, allocation and depth limits apply to a run
      in the worker. The deadline is the preview's `timeoutMs` prop, since
      a description has none.
- [ ] Numbers show the text Risor gave them: `1.0` stays `1.0`, an int
      beyond 2^53 is exact.
- [ ] With `run`, the preview shows what the callback returns, in the same
      view. The result's JSON shape is documented in the README. (A Go
      helper that makes it from a run is a follow-up.)
- [ ] Without `run`, when the description declares a function, the
      preview says in one sentence that it needs `run`, and still shows
      check errors.

### UC-3: Build the website's playground from the packages

1. `risor-site-v2` removes `app/vendor/risor-v3` and `sync:risor`, and
   depends on `risor`, `@risor/react` and `@risor/mantine`.
2. The playground, the "Try it" buttons and the home page example use the
   packages' editor, service and result view. The playground page itself
   stays in the site: only the site needs it.

Acceptance:

- [ ] `pnpm check:v3` passes with `risor` from npm.
- [ ] The playground keeps every feature it has today: variables as JSON,
      the check switch, limits, format, share links, examples.
- [ ] The playground gains completion, hover and error marks.

### UC-4: Run Risor from JavaScript

```ts
import { createEnv } from "risor";

const env = createEnv({ vars: "{order: {total: float}}" });
const program = env.compile("order.total > 100");
program.run({ order: { total: 120 } }); // true
```

Acceptance:

- [ ] `npm install risor` works in Node 22 and later, Deno, Bun, and every
      bundler that reads `exports`, with type declarations.
- [ ] The package has no dependencies. Its main entry imports no `node:`
      module. (`risor/testing`, the F9 binding, reads files and is for
      Node.)

### UC-5: Colour Risor in static pages and other editors (a later track)

1. The site renders docs with Shiki. It loads the Risor TextMate grammar
   from `@risor/grammar`.
2. A customer's docs use the same grammar with Shiki. (highlight.js and
   Prism are not covered.)

Acceptance:

- [ ] The grammar meets `f12-tools.md` 6 and passes its own scope tests.
- [ ] Shiki renders every example in `app/content/v3` with no unknown
      tokens.

## Requirements

- R-1: Each package is ES modules only, with `.d.ts` files, `sideEffects:
  false`, and an Apache-2.0 license.
- R-2: A framework is a peer dependency, never a dependency. A page that uses
  `@risor/react` installs React once.
- R-3: The editor never runs a user's program on the main thread. Checking,
  completion and hover may run on the main thread or in the worker; the
  default is the worker. Colouring runs on the main thread with
  `risor/tokens`, an entry that holds only the lexer (12 KB gzipped,
  measured).
- R-4: Every position a package exposes counts the units of a JavaScript
  string (UTF-16), as the DOM and CodeMirror do. The packages convert from
  Risor's code points at one place.
- R-5: The editor's colours come from Risor's own lexer, not from a second
  grammar. (The TextMate grammar is for places with no JavaScript runtime
  of ours: Shiki at build time, VS Code, GitHub.)
- R-6: Every component takes a `className` and exposes CSS variables for
  colour. The React and Vue components carry no styling framework. Mantine
  styling is only in `@risor/mantine`.
- R-6a: Every package names an environment one way: a description.
  `describeSample(sample)` makes one from sample JSON; `createEnv(options)
  .describe()` makes one from options.
- R-7: The packages release together, with one version number, which is the
  version of the Risor alpha they were built from (`3.0.0-alpha.N`).
- R-8: Each package has a README with a runnable example, and the
  repository has one example app that uses all of them.

## Not in scope

- **A language server.** D24 dropped it. The calls cover the editor's needs.
- **Monaco.** It is several megabytes and does not render on the server. A
  customer on Monaco can use the `risor` calls directly. Revisit on demand.
- **Svelte, Solid, Angular.** No user has asked. The CodeMirror extension
  works in any of them without a wrapper.
- **A playground component.** The site's playground stays in the site,
  built from the packages. No customer has asked for one.
- **A VS Code extension.** The grammar makes it small, but it is its own
  track.
- **Visual (block) editing of expressions.** A different product.
- **Running host functions written in JavaScript inside the worker.** The
  `run` callback (UC-2) covers a preview that needs functions.

## Decisions

- **CodeMirror 6 for the editor.** It is modular (the parts we use are about
  150 KB minified, from memory), works on mobile, and is framework-free, so
  React and Vue wrap one extension. Rejected: Monaco (size, no SSR); a
  `<textarea>` overlay (what the site has; no completion or hover).
- **The environment crosses to the worker as a description.** Functions
  cannot be sent to a worker. A description is JSON, the Go server already
  makes it, and `envFromDescription` already reads it. Rejected: building
  the environment on the main thread and sending it (not possible).
- **One repository.** The packages live in this repository under `web/`,
  next to `ts/`. A change to the TypeScript implementation and the editor
  that shows it land in one pull request, and CI tests them together.
  Rejected: a separate repository (versions drift; two PRs for one change).
- **The preview runs on the host's server when the environment has
  functions.** The browser cannot run them, and the server already has
  them. Rejected: a worker module of JavaScript functions (each host would
  write its functions twice, in Go and in JavaScript).
- **One shape for an environment: a description.** It crosses to the
  worker as JSON, Go already makes it, and one shape cannot be passed
  wrongly. Rejected: also taking `EnvOptions` or sample JSON in the prop
  (a raw description fits the options shape by structure and silently
  loses its functions).
- **Release order: React and Mantine first, Vue and the grammar after.**
  The site and the first customers use React and Mantine. Vue follows once
  the React API has settled, so both wrappers share one design.
- **Unscoped `risor` for the language, `@risor/*` for the rest.** It
  matches the Go module's name and `npm install risor` is the shortest
  thing to say. Rejected: `@risor/core` (one more thing to explain);
  `@deepnoodle/*` (ties the language to the company in its name).

## Success

- The website uses only the packages: `app/vendor/risor-v3` is gone.
- A new developer puts a checked Risor field in a Vite + React app in under
  15 minutes, from the README alone. We test this once by hand before the
  first release.
- First customer use: one Deep Noodle product or customer ships `RisorInput`.

Must not get worse: the cost of a run in the browser. A run through the
service in the worker adds no more than 2 ms to the run itself for a small
program.

## Risks and open questions

1. **Names on npm (blocks the publish, not the build).** `risor` and the
   `@risor` scope are free today (checked 2026-11-10). The owner must claim
   both before someone else does.
2. **`tokens` is a new TypeScript call.** Colours need the tokens, and
   `ts/src/index.ts` did not export the lexer. The build adds `tokens(text)`
   in its own entry, `risor/tokens`, as a TypeScript binding call, not a
   spec call. Open: should it be a call of F12, so Go and Rust have it too?
   Recommendation: no, until a non-browser editor needs it.
3. **Bundle size.** The full core is 141 KB gzipped. It loads only in the
   worker, so the page's first paint does not wait for it. The main thread
   loads `risor/tokens` (12 KB) for colours.
4. **Mantine versions.** The site uses Mantine 7.17; Mantine is at 9.8.
   Recommendation: `@risor/mantine` supports 7 and later in its peer range,
   tested with 7 and the latest. Open: or upgrade the site to the latest and
   support only that.
5. **Vue demand.** No Vue user is known, but the owner asked for Vue. It
   is the last step of the plan, after the React API has settled.
6. **Should a description with functions compile?** Today f01 5.4 item 4
   makes it compile nothing. If it compiled and failed only at a call of a
   function it lacks, a worker preview would run every program that calls
   no host function. That is a spec change, for the owner. The `run`
   callback does not depend on it.
7. **SSR.** CodeMirror needs the DOM. The components render a plain
   `<textarea>`-like element on the server and upgrade on mount. Risk: a
   layout shift. Measure it on the site's playground.
