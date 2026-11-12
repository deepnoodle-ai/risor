# @risor/react

React components for [Risor](https://github.com/deepnoodle-ai/risor): an editor
with errors as you type, completion and hover, and a view of a run's result.
Unstyled except for one optional CSS file. A Mantine package, `@risor/mantine`, is planned (#634).

> Risor v3 is an alpha. This API will change before 3.0.

```sh
npm install @risor/react
```

CodeMirror and React are peer dependencies, so the page has one copy of
each. npm 7 and later install them; with pnpm or Yarn, add
`@codemirror/state @codemirror/view @codemirror/autocomplete @codemirror/lint @codemirror/commands`.

```tsx
import { useRef, useState } from "react";
import { describeSample, RisorEditor, RisorResult, useRisorRun } from "@risor/react";
import type { RisorEditorHandle } from "@risor/react";
import "@risor/react/styles.css";

const sample = { order: { total: 120.5, items: [{ sku: "A1" }] } };
const environment = describeSample(sample);

export function Rule() {
  const [code, setCode] = useState("order.total > 100");
  const editor = useRef<RisorEditorHandle>(null);
  const { result, running } = useRisorRun({ code, environment, vars: sample });
  return (
    <form onSubmit={async (e) => {
      e.preventDefault();
      const problems = await editor.current!.validate(); // fresh, not from the last pause
      if (problems.length === 0) save(code);
    }}>
      <RisorEditor ref={editor} value={code} onChange={setCode} environment={environment} expected="bool" singleLine />
      <RisorResult result={result} text={code} running={running} />
    </form>
  );
}
```

| Export | Is |
| --- | --- |
| `RisorEditor` | The editor. Controlled (`value`, `onChange`). Its ref has `validate()`, `format()`, `focus()`. On the server it renders the text in a `<pre>`. |
| `RisorResult` | A run's value as a tree, its type and usage; or its problems with a caret under the line. |
| `RisorDiagnostics` | Problems as a list. |
| `useRisorRun` | Runs the text after each pause. Pass `run` to run on your server instead (needed when the environment has host functions). |
| `useRisorCheck` | Checks the text after each pause; `validate()` checks it now. |
| `RisorProvider` | Gives the components one service. Without it they share one worker per page. |

The server still checks the text before it saves it. The editor helps the
user; it is not the gate.

License: Apache-2.0
