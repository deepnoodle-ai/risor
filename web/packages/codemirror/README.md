# @deepnoodle/risor-codemirror

[Risor](https://github.com/deepnoodle-ai/risor) for CodeMirror 6: colours from
Risor's own lexer, errors as you type, completion, hover, format
(Shift-Alt-F) and a run key (Mod-Enter). Works in any framework.

> Risor v3 is an alpha. This API will change before 3.0.

```sh
npm install @deepnoodle/risor-codemirror @codemirror/state @codemirror/view @codemirror/autocomplete @codemirror/lint
```

```ts
import { EditorView, basicSetup } from "codemirror";
import { risor, validateRisor } from "@deepnoodle/risor-codemirror";
import { describeSample } from "@deepnoodle/risor-service";

const view = new EditorView({
  parent: document.querySelector("#editor")!,
  doc: "order.total > 100",
  extensions: [
    basicSetup,
    risor({
      environment: describeSample({ order: { total: 120.5 } }),
      expected: "bool",
      onDiagnostics: (problems) => console.log(problems),
    }),
  ],
});

// On submit: check the text as it is now.
const problems = await validateRisor(view);
```

| Option | Default | Does |
| --- | --- | --- |
| `environment` | empty | A description (see `@deepnoodle/risor-service`) |
| `expected` | none | The type the text must have, as type text |
| `singleLine` | `false` | Newlines become spaces; Enter calls `onRun` |
| `service` | a shared worker service | Where checks and completions run |
| `lintDelay` | `300` | The pause after typing before a check, in ms |
| `completion` | `true` | `false` to configure `autocompletion` yourself with `risorCompletions` |

The options are fixed when the extension is made. To change the environment,
put `risor(...)` in a `Compartment` and reconfigure it.

Colours are CSS variables with light and dark defaults: `--risor-keyword`,
`--risor-string`, `--risor-number`, `--risor-comment`, `--risor-function`,
`--risor-property`, `--risor-operator`, `--risor-constant`,
`--risor-interpolation`, `--risor-invalid`.

License: Apache-2.0
