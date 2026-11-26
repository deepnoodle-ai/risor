import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { describe, test } from "node:test";

import {
  buildRustHarness,
  extractBlocks,
  selectVerifiedBlocks,
  verifyGoGuide,
  verifyRustGuide,
  verifyTsGuide,
} from "./check-host-guides.mjs";

function hasCommand(cmd) {
  try {
    execFileSync(cmd, ["--version"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

describe("extractBlocks and selectVerifiedBlocks", () => {
  test("fails when a go/ts/rust fenced block is missing a directive", () => {
    for (const [lang, fenceLang] of [
      ["go", "go"],
      ["ts", "ts"],
      ["rust", "rust"],
    ]) {
      const md = `# Example\n\n\`\`\`${fenceLang}\n// missing directive\n\`\`\`\n`;
      const blocks = extractBlocks(md, "test.md");
      assert.throws(
        () => selectVerifiedBlocks(blocks, lang, "test.md"),
        /missing a <!-- risor-verify:/,
      );
    }
  });

  test("fails when a bare fence or unrecognized language tag is missing a directive", () => {
    for (const fenceLang of ["", "golang", "js"]) {
      const md = `# Example\n\n\`\`\`${fenceLang}\n// missing directive\n\`\`\`\n`;
      const blocks = extractBlocks(md, "test.md");
      assert.throws(
        () => selectVerifiedBlocks(blocks, "go", "test.md"),
        /missing a <!-- risor-verify:/,
      );
    }
  });

  test("recognizes tilde fences and requires directives on them", () => {
    const md = [
      "# Two examples",
      "",
      '<!-- risor-verify: {"mode": "ts"} -->',
      "```ts",
      'const ok = "```not a close";',
      "```",
      "",
      "~~~ts",
      "this is unannotated and invalid",
      "~~~",
    ].join("\n");
    const blocks = extractBlocks(md, "tilde.md");
    assert.equal(blocks.length, 2);
    assert.equal(blocks[1].lang, "ts");
    assert.throws(
      () => selectVerifiedBlocks(blocks, "ts", "tilde.md"),
      /missing a <!-- risor-verify:/,
    );
  });

  test("ignores non-code fences such as bash and toml without directives", () => {
    const md = [
      "# Install",
      "",
      "```bash",
      "npm install @deepnoodle/risor",
      "```",
      "",
      '<!-- risor-verify: {"mode": "ts"} -->',
      "```ts",
      'import { parse } from "@deepnoodle/risor";',
      'parse("1 + 1");',
      "```",
    ].join("\n");
    const blocks = extractBlocks(md, "test.md");
    const { runnable, skipped } = selectVerifiedBlocks(blocks, "ts", "test.md");
    assert.equal(runnable.length, 1);
    assert.equal(skipped, 0);
  });

  test("fails on invalid JSON, dangling directive, or skip without reason", () => {
    assert.throws(
      () => extractBlocks("<!-- risor-verify: {bad} -->\n```ts\n1\n```\n", "bad.md"),
      /invalid JSON/,
    );
    assert.throws(
      () => extractBlocks('<!-- risor-verify: {"mode": "ts"} -->\nnot a fence\n', "dangling.md"),
      /not followed by a fenced code block/,
    );
    const skipNoReason = extractBlocks(
      '<!-- risor-verify: {"mode": "skip"} -->\n```ts\n1\n```\n',
      "skip.md",
    );
    assert.throws(
      () => selectVerifiedBlocks(skipNoReason, "ts", "skip.md"),
      /requires a non-empty "reason"/,
    );
  });

  test("rejects mismatched mode for guide language", () => {
    const blocks = extractBlocks(
      '<!-- risor-verify: {"mode": "go"} -->\n```ts\n1\n```\n',
      "mismatch.md",
    );
    assert.throws(
      () => selectVerifiedBlocks(blocks, "ts", "mismatch.md"),
      /invalid mode "go" for ts guide/,
    );
  });
});

describe("verifyTsGuide", () => {
  test("passes a valid TS example and checks stdout", () => {
    const md = [
      "# Quick start",
      "",
      '<!-- risor-verify: {"mode": "ts", "expectStdout": "42\\n"} -->',
      "```ts",
      'import { createEnv, parse } from "@deepnoodle/risor";',
      'const out = createEnv().compile(parse("40 + 2")).run();',
      "console.log(out);",
      "```",
    ].join("\n");
    const res = verifyTsGuide(md, "inline-ts.md");
    assert.equal(res.verified, 1);
  });

  test("catches stdout mismatches and runtime errors", () => {
    const wrongStdout = [
      '<!-- risor-verify: {"mode": "ts", "expectStdout": "99\\n"} -->',
      "```ts",
      'import { createEnv, parse } from "@deepnoodle/risor";',
      'console.log(createEnv().compile(parse("1 + 1")).run());',
      "```",
    ].join("\n");
    assert.throws(() => verifyTsGuide(wrongStdout, "wrong-stdout.md"), /stdout mismatch/);

    const runtimeErr = [
      '<!-- risor-verify: {"mode": "ts"} -->',
      "```ts",
      'import { createEnv, parse } from "@deepnoodle/risor";',
      'createEnv().compile(parse("unknownVar")).run();',
      "```",
    ].join("\n");
    assert.throws(() => verifyTsGuide(runtimeErr, "runtime-err.md"), /execution failed/);
  });
});

describe("verifyGoGuide", () => {
  test("passes a valid Go example and catches compile/stdout failures", () => {
    const good = [
      '<!-- risor-verify: {"mode": "go", "expectStdout": "42\\n"} -->',
      "```go",
      "package main",
      "",
      'import (',
      '\t"context"',
      '\t"fmt"',
      '\trisor "github.com/deepnoodle-ai/risor/go/v3"',
      ")",
      "",
      "func main() {",
      "\tenv, _ := risor.NewEnv()",
      '\texpr, _ := risor.Parse("40 + 2")',
      "\tprog, _ := env.Compile(expr)",
      "\tv, _ := prog.Run(context.Background(), nil)",
      "\tn, _ := risor.AsInt(v)",
      "\tfmt.Println(n)",
      "}",
      "```",
    ].join("\n");
    const res = verifyGoGuide(good, "inline-go.md");
    assert.equal(res.verified, 1);

    const badCompile = [
      '<!-- risor-verify: {"mode": "go"} -->',
      "```go",
      "package main",
      'import risor "github.com/deepnoodle-ai/risor/go/v3"',
      "func main() { risor.DoesNotExist() }",
      "```",
    ].join("\n");
    assert.throws(() => verifyGoGuide(badCompile, "bad-go.md"), /verification failed/);

    const badStdout = good.replace('"42\\n"', '"99\\n"');
    assert.throws(() => verifyGoGuide(badStdout, "bad-stdout-go.md"), /stdout mismatch/);
  });
});

describe("verifyRustGuide", () => {
  test("generates test harness modules for Rust examples", () => {
    const blocks = extractBlocks(
      '<!-- risor-verify: {"mode": "rust"} -->\n```rust\nfn main() { assert_eq!(1 + 1, 2); }\n```\n',
      "rust.md",
    );
    const { runnable } = selectVerifiedBlocks(blocks, "rust", "rust.md");
    const harness = buildRustHarness(runnable, "rust.md");
    assert.match(harness, /mod ex_1/);
    assert.match(harness, /fn run\(\)/);
  });

  test("compiles and runs a minimal Rust example offline when cargo is available", (t) => {
    if (!hasCommand("cargo")) {
      t.skip("cargo not installed in this environment");
      return;
    }
    const good = [
      '<!-- risor-verify: {"mode": "rust"} -->',
      "```rust",
      "use risor::{parse, CompileOptions, Env, NoVars};",
      "fn main() -> Result<(), Box<dyn std::error::Error>> {",
      '    let p = Env::default().compile(&parse("40 + 2")?, CompileOptions::default())?;',
      "    assert_eq!(p.run(&NoVars)?.as_i64(), Some(42));",
      "    Ok(())",
      "}",
      "```",
    ].join("\n");
    const res = verifyRustGuide(good, "inline-rust.md");
    assert.equal(res.verified, 1);

    const badAssert = good.replace("Some(42)", "Some(99)");
    assert.throws(() => verifyRustGuide(badAssert, "bad-rust.md"), /verification failed/);
  });
});
