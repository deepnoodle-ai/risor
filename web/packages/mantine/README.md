# @risor/mantine

[Risor](https://github.com/deepnoodle-ai/risor) in Mantine's style: a checked
expression input with a label, a description and an error, and a preview of
the result against sample data. Mantine 7.17 or later.

> Risor v3 is an alpha. This API will change before 3.0.

```sh
npm install @risor/mantine
```

```tsx
import { useRef } from "react";
import { useForm } from "@mantine/form";
import { RisorInput, RisorPreview } from "@risor/mantine";
import type { RisorInputHandle } from "@risor/mantine";
import "@risor/mantine/styles.css";

// `description` is what your Go server's env.Describe() returned.
export function StepCondition({ description, sample }) {
  const form = useForm({ initialValues: { condition: "order.total > 100" } });
  const input = useRef<RisorInputHandle>(null);
  return (
    <form onSubmit={form.onSubmit(async (values) => {
      if ((await input.current!.validate()).length > 0) return;
      await save(values);
    })}>
      <RisorInput
        ref={input}
        label="Run this step when"
        description="A Risor expression that gives a bool"
        environment={description}
        expected="bool"
        {...form.getInputProps("condition")}
      />
      <RisorPreview
        code={form.values.condition}
        vars={sample}
        // The environment has host functions, so the preview runs on the server:
        run={(code, vars) => fetch("/api/risor/preview", { method: "POST", body: JSON.stringify({ code, vars }) }).then((r) => r.json())}
      />
    </form>
  );
}
```

`RisorInput` shows the checker's first message as its error, unless you pass
`error`, which wins. `validate()` on its ref checks the text as it is now:
await it on submit. Your server still checks the text before it saves it.

The preview's `run` returns the `RunResult` shape documented in
`@risor/service`. Without `run`, the preview runs in a web worker, which
works when the environment has no host functions.

License: Apache-2.0
