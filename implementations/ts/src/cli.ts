#!/usr/bin/env node
/**
 * Risor CLI - Command-line interface for the Risor scripting language.
 */

import { startRepl } from "./repl.js";
import { runFile } from "./runner.js";

const VERSION = "2.0.0";

function printUsage(): void {
  console.log(`
Risor v${VERSION} - Fast, embedded scripting language

Usage:
  risor [options] [file] [args...]

Options:
  -h, --help      Show this help message
  -v, --version   Show version
  -e, --eval      Evaluate code from command line
  -i, --interactive  Start REPL after running file

Examples:
  risor                    Start interactive REPL
  risor script.risor       Run a script file
  risor -e "print(1 + 2)"  Evaluate code
  risor -i script.risor    Run script then start REPL
`);
}

function printVersion(): void {
  console.log(`Risor ${VERSION}`);
}

async function main(): Promise<void> {
  const args = process.argv.slice(2);
  let evalCode: string | null = null;
  let interactive = false;
  let file: string | null = null;
  const scriptArgs: string[] = [];

  // Parse arguments
  let i = 0;
  while (i < args.length) {
    const arg = args[i];

    if (arg === "-h" || arg === "--help") {
      printUsage();
      process.exit(0);
    } else if (arg === "-v" || arg === "--version") {
      printVersion();
      process.exit(0);
    } else if (arg === "-e" || arg === "--eval") {
      i++;
      if (i >= args.length) {
        console.error("Error: -e requires an argument");
        process.exit(1);
      }
      evalCode = args[i];
    } else if (arg === "-i" || arg === "--interactive") {
      interactive = true;
    } else if (arg.startsWith("-")) {
      console.error(`Error: Unknown option: ${arg}`);
      printUsage();
      process.exit(1);
    } else {
      // First non-option is the file, rest are script args
      file = arg;
      scriptArgs.push(...args.slice(i + 1));
      break;
    }
    i++;
  }

  try {
    if (evalCode !== null) {
      // Evaluate code from command line
      const result = await runCode(evalCode);
      if (result !== undefined && result !== null) {
        console.log(result);
      }
      if (interactive) {
        await startRepl();
      }
    } else if (file !== null) {
      // Run a file
      const result = await runFile(file);
      // Print result for REPL-like behavior (but scripts usually use print explicitly)
      if (result !== null && result.type !== "nil") {
        console.log(result.inspect());
      }
      if (interactive) {
        await startRepl();
      }
    } else {
      // Start REPL
      await startRepl();
    }
  } catch (err) {
    console.error(`Error: ${err instanceof Error ? err.message : err}`);
    process.exit(1);
  }
}

async function runCode(code: string): Promise<string | null> {
  const { Lexer } = await import("./lexer/lexer.js");
  const { Parser } = await import("./parser/parser.js");
  const { Compiler } = await import("./compiler/compiler.js");
  const { VM } = await import("./vm/vm.js");
  const { createBuiltins } = await import("./builtins/builtins.js");

  const lexer = new Lexer(code);
  const parser = new Parser(lexer);
  const program = parser.parse();

  const errors = parser.getErrors();
  if (errors.length > 0) {
    throw new Error(errors[0].message);
  }

  const builtins = createBuiltins();
  const globalNames = Array.from(builtins.keys());

  const compiler = new Compiler({ globalNames });
  const bytecode = compiler.compile(program);

  const vm = new VM({ globals: builtins });
  const result = vm.run(bytecode);

  if (result.type !== "nil") {
    return result.inspect();
  }
  return null;
}

main();
