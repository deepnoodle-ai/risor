import { Group, Loader, Paper, Text } from "@mantine/core";
import type { ReactNode } from "react";
import { RisorResult, useRisorRun } from "@deepnoodle/risor-react";
import type { Description, RunCallback } from "@deepnoodle/risor-react";

export interface RisorPreviewProps {
  code: string;
  environment?: Description;
  /** The values of the variables. */
  vars?: Record<string, unknown>;
  expected?: string;
  /** Run on the host's server instead of in the worker: needed when the environment has host functions. */
  run?: RunCallback;
  /** The deadline of a run in the worker. Default 2,000 ms. */
  timeoutMs?: number;
  /** The pause after typing before a run. Default 400 ms. */
  debounce?: number;
  title?: ReactNode;
  className?: string;
}

/** The result of an expression against sample data, updated after each pause in typing. */
export function RisorPreview({ code, environment, vars, expected, run, timeoutMs, debounce, title = "Preview", className }: RisorPreviewProps) {
  const { result, running } = useRisorRun({ code, environment, vars, expected, run, timeoutMs, debounce });
  return (
    <Paper withBorder p="sm" radius="sm" className={["risor-mantine-preview", className].filter(Boolean).join(" ")}>
      <Group justify="space-between" mb={6} wrap="nowrap">
        <Text size="xs" fw={600} c="dimmed" tt="uppercase">
          {title}
        </Text>
        {running && <Loader size={12} />}
      </Group>
      <RisorResult result={result} text={code} running={running} />
    </Paper>
  );
}
