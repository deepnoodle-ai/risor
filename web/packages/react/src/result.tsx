import { useMemo } from "react";
import type { ReactNode } from "react";
import { parseJsonTree } from "@risor/service";
import type { JsonNode, Problem, RunResult } from "@risor/service";

/** A JSON value as a tree: objects and arrays fold. */
export function RisorValue({ json, openDepth = 2 }: { json: string; openDepth?: number }) {
  const tree = useMemo(() => {
    try {
      return parseJsonTree(json);
    } catch {
      return null;
    }
  }, [json]);
  if (!tree) return <pre className="risor-value">{json}</pre>;
  return (
    <div className="risor-value" role="tree">
      <Node node={tree} depth={0} openDepth={openDepth} />
    </div>
  );
}

function Scalar({ node }: { node: Exclude<JsonNode, { kind: "array" | "object" }> }) {
  switch (node.kind) {
    case "null":
      return <span className="risor-v-null">null</span>;
    case "bool":
      return <span className="risor-v-bool">{String(node.value)}</span>;
    case "number":
      return <span className="risor-v-number">{node.text}</span>;
    case "string":
      return <span className="risor-v-string">{JSON.stringify(node.value)}</span>;
  }
}

function Node({ node, depth, openDepth, label }: { node: JsonNode; depth: number; openDepth: number; label?: ReactNode }) {
  const key = label !== undefined ? <span className="risor-v-key">{label}: </span> : null;
  if (node.kind !== "array" && node.kind !== "object") {
    return (
      <div className="risor-v-row" role="treeitem">
        {key}
        <Scalar node={node} />
      </div>
    );
  }
  const entries: [ReactNode, JsonNode][] = node.kind === "array" ? node.items.map((n, i) => [i, n]) : node.entries.map(([k, n]) => [k, n]);
  const [open, close] = node.kind === "array" ? ["[", "]"] : ["{", "}"];
  if (entries.length === 0) {
    return (
      <div className="risor-v-row" role="treeitem">
        {key}
        <span className="risor-v-punct">{open + close}</span>
      </div>
    );
  }
  const summary = node.kind === "array" ? `${entries.length} item${entries.length === 1 ? "" : "s"}` : `${entries.length} key${entries.length === 1 ? "" : "s"}`;
  return (
    <details className="risor-v-group" open={depth < openDepth} role="treeitem">
      <summary>
        {key}
        <span className="risor-v-punct">{open}</span>
        <span className="risor-v-summary">{` ${summary} `}</span>
        <span className="risor-v-punct">{close}</span>
      </summary>
      <div className="risor-v-children" role="group">
        {entries.map(([k, n], i) => (
          <Node key={i} node={n} depth={depth + 1} openDepth={openDepth} label={k} />
        ))}
      </div>
    </details>
  );
}

/** Problems as a list: each with its code, its place and its message, and a caret under its line when `text` is given. */
export function RisorDiagnostics({ problems, text, className }: { problems: Problem[]; text?: string; className?: string }) {
  if (problems.length === 0) return null;
  const lines = text?.split("\n");
  return (
    <ul className={["risor-diagnostics", className].filter(Boolean).join(" ")}>
      {problems.map((p, i) => {
        const line = p.line !== undefined && lines ? lines[p.line - 1] : undefined;
        const width = p.from !== undefined && p.to !== undefined ? Math.max(1, Math.min(p.to - p.from, (line?.length ?? 0) - (p.column ?? 1) + 1)) : 1;
        return (
          <li key={i} className={`risor-diagnostic risor-diagnostic-${p.phase}`}>
            <div>
              <code className="risor-diagnostic-code">{p.code}</code>
              {p.line !== undefined && <span className="risor-diagnostic-place">{` ${p.line}:${p.column}`}</span>}
              <span className="risor-diagnostic-message">{` ${p.message}`}</span>
            </div>
            {line !== undefined && p.column !== undefined && (
              <pre className="risor-diagnostic-frame">
                {`${line}\n${" ".repeat(Array.from(line).slice(0, p.column - 1).join("").length)}${"^".repeat(width)}`}
              </pre>
            )}
            {p.trace && p.trace.length > 0 && (
              <ol className="risor-diagnostic-trace">
                {p.trace.map((t, j) => (
                  <li key={j}>
                    {`in ${t.name} at ${t.line}:${t.column}`}
                  </li>
                ))}
              </ol>
            )}
          </li>
        );
      })}
    </ul>
  );
}

/** The result of a run: the value as a tree with its type and usage, or the problems. */
export function RisorResult({ result, text, running, className, showUsage = true }: { result: RunResult | null; text?: string; running?: boolean; className?: string; showUsage?: boolean }) {
  const classes = ["risor-result", running ? "risor-result-running" : "", className ?? ""].filter(Boolean).join(" ");
  if (!result) return <div className={classes} aria-busy={running}>{running ? "Running…" : null}</div>;
  const usage = result.usage;
  return (
    <div className={classes} aria-busy={running}>
      {result.ok ? (
        <>
          <RisorValue json={result.json} />
          {result.type && <div className="risor-result-type">type <code>{result.type}</code></div>}
        </>
      ) : (
        <>
          <div className="risor-result-stage">{result.stage === "stopped" ? "Stopped" : result.stage === "run" ? "Run error" : result.stage === "compile" ? "Does not compile" : "Environment error"}</div>
          <RisorDiagnostics problems={result.problems} text={text} />
        </>
      )}
      {showUsage && usage && (
        <div className="risor-result-usage">
          {`${usage.steps.toLocaleString()} steps · ${usage.allocated.toLocaleString()} allocation units${result.ms !== undefined ? ` · ${result.ms < 1 ? "<1" : Math.round(result.ms)} ms` : ""}`}
        </div>
      )}
    </div>
  );
}
