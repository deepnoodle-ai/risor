/**
 * Risor Runner - Execute Risor scripts from files.
 */

import * as fs from "fs";
import * as path from "path";
import { Lexer } from "./lexer/lexer.js";
import { Parser } from "./parser/parser.js";
import { Compiler } from "./compiler/compiler.js";
import { VM } from "./vm/vm.js";
import { createBuiltins } from "./builtins/builtins.js";
import { RisorObject, ObjectType } from "./object/object.js";

/**
 * Run a Risor script file.
 */
export async function runFile(filepath: string): Promise<RisorObject | null> {
  // Resolve the path
  const resolved = path.resolve(filepath);

  // Check if file exists
  if (!fs.existsSync(resolved)) {
    throw new Error(`File not found: ${filepath}`);
  }

  // Read the file
  const code = fs.readFileSync(resolved, "utf-8");

  // Run the code
  return runCode(code, resolved);
}

/**
 * Run Risor code and return the result.
 */
export function runCode(code: string, filename: string = "<input>"): RisorObject | null {
  const lexer = new Lexer(code);
  const parser = new Parser(lexer);
  const program = parser.parse();

  const errors = parser.getErrors();
  if (errors.length > 0) {
    throw new Error(`Parse error in ${filename}: ${errors[0].message}`);
  }

  const builtins = createBuiltins();
  const globalNames = Array.from(builtins.keys());

  const compiler = new Compiler({ globalNames, filename });
  const bytecode = compiler.compile(program);

  const vm = new VM({ globals: builtins });

  try {
    const result = vm.run(bytecode);
    return result;
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    throw new Error(`Runtime error in ${filename}: ${message}`);
  }
}
