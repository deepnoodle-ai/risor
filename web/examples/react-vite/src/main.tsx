// One page that uses every package: a checked condition field in a Mantine
// form (UC-1), a preview against sample data (UC-2), and a plain editor
// with a result view from @risor/react.

import { StrictMode, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Button, Code, Container, Group, MantineProvider, SegmentedControl, SimpleGrid, Stack, Text, Textarea, Title } from "@mantine/core";
import "@mantine/core/styles.css";
import { describeSample, RisorInput, RisorPreview } from "@risor/mantine";
import type { Description, RisorInputHandle } from "@risor/mantine";
import "@risor/mantine/styles.css";
import { RisorEditor, RisorResult, useRisorRun } from "@risor/react";

const SAMPLE = {
  order: { id: "A-1001", total: 182.5, items: [{ sku: "KB-1", qty: 2, price: 45 }, { sku: "MS-3", qty: 1, price: 92.5 }], coupon: null },
  customer: { name: "Ada", tier: "gold" },
};

/** As a Go server would send it: the sample's variables, plus one host function. */
const WITH_FUNCTION: Description = {
  ...describeSample(SAMPLE),
  functions: { "geo.distanceKm": "(from: string, to: string) => float" },
};

function ConditionForm() {
  const input = useRef<RisorInputHandle>(null);
  const [value, setValue] = useState("order.total > 100 && customer.tier == \"gold\"");
  const [envKind, setEnvKind] = useState("sample");
  const [submitted, setSubmitted] = useState<string | null>(null);
  const environment = envKind === "sample" ? describeSample(SAMPLE) : WITH_FUNCTION;
  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <Title order={3}>A condition field</Title>
        <SegmentedControl size="xs" value={envKind} onChange={setEnvKind} data={[{ value: "sample", label: "Sample data" }, { value: "functions", label: "With a host function" }]} />
      </Group>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          const problems = (await input.current?.validate()) ?? [];
          setSubmitted(problems.length === 0 ? `Saved: ${value}` : `Blocked: ${problems[0].message}`);
        }}
      >
        <Stack gap="sm">
          <RisorInput
            ref={input}
            label="Run this step when"
            required
            description="A Risor expression that gives a bool. Try order. and wait for the fields."
            environment={environment}
            expected="bool"
            value={value}
            onChange={setValue}
            onSubmit={() => (document.activeElement as HTMLElement | null)?.closest("form")?.requestSubmit()}
          />
          <Group>
            <Button type="submit" size="xs">Save</Button>
            {submitted && <Text size="sm" data-testid="submitted">{submitted}</Text>}
          </Group>
        </Stack>
      </form>
      <RisorPreview code={value} environment={environment} vars={SAMPLE} expected="bool" />
    </Stack>
  );
}

function Playground() {
  const [code, setCode] = useState(`const lines = order.items.map(i => i.qty * i.price);
{
  count: len(lines),
  sum: lines.reduce((a, b) => a + b, 0.0),
  first: order.items[0].sku,
  // A template:
  note: \`\${customer.name} (\${customer.tier})\`,
}`);
  const [vars, setVars] = useState(JSON.stringify(SAMPLE, null, 2));
  const parsed = useMemo(() => {
    try {
      return { ok: true as const, value: JSON.parse(vars) as Record<string, unknown> };
    } catch (e) {
      return { ok: false as const, message: (e as Error).message };
    }
  }, [vars]);
  const environment = useMemo(() => (parsed.ok ? describeSample(parsed.value) : undefined), [parsed]);
  const { result, running, rerun } = useRisorRun({ code, environment, vars: parsed.ok ? parsed.value : {} });
  return (
    <Stack gap="sm">
      <Title order={3}>An editor and a result</Title>
      <SimpleGrid cols={{ base: 1, md: 2 }}>
        <Stack gap={4}>
          <Text size="sm" fw={500}>Program</Text>
          <RisorEditor value={code} onChange={setCode} environment={environment} onRun={() => void rerun()} minLines={10} aria-label="Program" />
          <Text size="xs" c="dimmed">Ctrl/⌘-Enter runs. Shift-Alt-F formats.</Text>
        </Stack>
        <Stack gap={4}>
          <Text size="sm" fw={500}>Variables (JSON)</Text>
          <Textarea value={vars} onChange={(e) => setVars(e.currentTarget.value)} autosize minRows={10} styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)" } }} error={parsed.ok ? undefined : parsed.message} />
        </Stack>
      </SimpleGrid>
      <Text size="sm" fw={500}>Result</Text>
      <RisorResult result={result} text={code} running={running} />
      <Text size="xs" c="dimmed">
        Try an endless program: <Code>{"len(range(100000000).map(i => i))"}</Code>
      </Text>
    </Stack>
  );
}

function App() {
  return (
    <MantineProvider defaultColorScheme="auto">
      <Container size="md" py="xl">
        <Stack gap="xl">
          <Title order={1}>Risor in the browser</Title>
          <ConditionForm />
          <Playground />
        </Stack>
      </Container>
    </MantineProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
