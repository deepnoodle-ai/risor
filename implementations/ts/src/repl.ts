/**
 * Risor REPL - Read-Eval-Print Loop for interactive scripting.
 */

import * as readline from "readline";
import { Lexer } from "./lexer/lexer.js";
import { Parser } from "./parser/parser.js";
import { Compiler } from "./compiler/compiler.js";
import { VM } from "./vm/vm.js";
import { createBuiltins } from "./builtins/builtins.js";
import { RisorObject, ObjectType } from "./object/object.js";

const PROMPT = ">>> ";
const CONTINUE_PROMPT = "... ";

/**
 * Start the interactive REPL.
 */
export async function startRepl(): Promise<void> {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: true,
  });

  console.log("Risor v2.0.0 - Type 'exit' or Ctrl+D to quit");
  console.log("");

  const builtins = createBuiltins();
  let buffer = "";
  let inMultiline = false;

  const prompt = (): void => {
    rl.question(inMultiline ? CONTINUE_PROMPT : PROMPT, (input) => {
      if (input === undefined) {
        // Ctrl+D
        console.log("\nGoodbye!");
        rl.close();
        return;
      }

      const line = input.trim();

      // Handle exit commands
      if (!inMultiline && (line === "exit" || line === "quit")) {
        console.log("Goodbye!");
        rl.close();
        return;
      }

      // Handle special commands
      if (!inMultiline && line.startsWith("/")) {
        handleCommand(line);
        prompt();
        return;
      }

      // Accumulate input
      buffer += (buffer ? "\n" : "") + input;

      // Check if input is complete
      if (isComplete(buffer)) {
        if (buffer.trim()) {
          try {
            const result = evaluate(buffer, builtins);
            if (result !== null && result.type !== ObjectType.Nil) {
              console.log(result.inspect());
            }
          } catch (err) {
            console.error(`Error: ${err instanceof Error ? err.message : err}`);
          }
        }
        buffer = "";
        inMultiline = false;
      } else {
        inMultiline = true;
      }

      prompt();
    });
  };

  prompt();
}

/**
 * Check if the input is syntactically complete.
 */
function isComplete(input: string): boolean {
  // Simple heuristic: check for balanced braces/brackets/parens
  let braces = 0;
  let brackets = 0;
  let parens = 0;
  let inString = false;
  let stringChar = "";

  for (let i = 0; i < input.length; i++) {
    const c = input[i];

    if (inString) {
      if (c === stringChar && input[i - 1] !== "\\") {
        inString = false;
      }
      continue;
    }

    switch (c) {
      case '"':
      case "'":
      case "`":
        inString = true;
        stringChar = c;
        break;
      case "{":
        braces++;
        break;
      case "}":
        braces--;
        break;
      case "[":
        brackets++;
        break;
      case "]":
        brackets--;
        break;
      case "(":
        parens++;
        break;
      case ")":
        parens--;
        break;
    }
  }

  // Input is complete if all brackets are balanced and not in a string
  return braces === 0 && brackets === 0 && parens === 0 && !inString;
}

/**
 * Evaluate code and return the result.
 */
function evaluate(code: string, builtins: Map<string, RisorObject>): RisorObject | null {
  const lexer = new Lexer(code);
  const parser = new Parser(lexer);
  const program = parser.parse();

  const errors = parser.getErrors();
  if (errors.length > 0) {
    throw new Error(errors[0].message);
  }

  const globalNames = Array.from(builtins.keys());
  const compiler = new Compiler({ globalNames });
  const bytecode = compiler.compile(program);

  const vm = new VM({ globals: builtins });
  return vm.run(bytecode);
}

/**
 * Handle REPL commands.
 */
function handleCommand(cmd: string): void {
  const parts = cmd.slice(1).split(/\s+/);
  const command = parts[0].toLowerCase();

  switch (command) {
    case "help":
      console.log(`
REPL Commands:
  /help     Show this help
  /clear    Clear the screen
  /exit     Exit the REPL
`);
      break;

    case "clear":
      console.clear();
      break;

    case "exit":
    case "quit":
      process.exit(0);

    default:
      console.log(`Unknown command: /${command}. Type /help for available commands.`);
  }
}
