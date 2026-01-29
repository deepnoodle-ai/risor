/**
 * Compiler tests.
 */

import { describe, it, expect } from "vitest";
import { parse } from "../parser/parser.js";
import { Compiler, CompilerError } from "./compiler.js";
import { Op, opName } from "../bytecode/opcode.js";
import { Code, Constant, ConstantType } from "../bytecode/code.js";

function compileSource(source: string): Code {
  const program = parse(source);
  const compiler = new Compiler({ source });
  return compiler.compile(program);
}

function getInstructions(code: Code): number[] {
  return [...code.instructions];
}

function instructionNames(code: Code): string[] {
  const names: string[] = [];
  const instrs = code.instructions;
  let i = 0;
  while (i < instrs.length) {
    const op = instrs[i] as Op;
    names.push(opName(op));
    i++;
    // Skip operands based on opcode
    switch (op) {
      case Op.Call:
      case Op.CallSpread:
      case Op.JumpBackward:
      case Op.JumpForward:
      case Op.PopJumpForwardIfFalse:
      case Op.PopJumpForwardIfTrue:
      case Op.PopJumpForwardIfNotNil:
      case Op.PopJumpForwardIfNil:
      case Op.LoadAttr:
      case Op.LoadFast:
      case Op.LoadFree:
      case Op.LoadGlobal:
      case Op.LoadConst:
      case Op.LoadAttrOrNil:
      case Op.StoreAttr:
      case Op.StoreFast:
      case Op.StoreFree:
      case Op.StoreGlobal:
      case Op.BinaryOp:
      case Op.CompareOp:
      case Op.BuildList:
      case Op.BuildMap:
      case Op.BuildString:
      case Op.ListAppend:
      case Op.ListExtend:
      case Op.MapMerge:
      case Op.MapSet:
      case Op.BinarySubscr:
      case Op.StoreSubscr:
      case Op.ContainsOp:
      case Op.Length:
      case Op.Unpack:
      case Op.Swap:
      case Op.Copy:
      case Op.Partial:
        i += 1;
        break;
      case Op.Slice:
      case Op.LoadClosure:
      case Op.MakeCell:
      case Op.PushExcept:
        i += 2;
        break;
    }
  }
  return names;
}

describe("Compiler", () => {
  describe("literals", () => {
    it("should compile integers", () => {
      const code = compileSource("42");
      expect(code.constants.some((c) => c.type === ConstantType.Int && c.value === 42)).toBe(true);
      expect(instructionNames(code)).toContain("LoadConst");
    });

    it("should compile floats", () => {
      const code = compileSource("3.14");
      expect(code.constants.some((c) => c.type === ConstantType.Float && c.value === 3.14)).toBe(
        true
      );
    });

    it("should compile strings", () => {
      const code = compileSource('"hello"');
      expect(
        code.constants.some((c) => c.type === ConstantType.String && c.value === "hello")
      ).toBe(true);
    });

    it("should compile booleans", () => {
      const code = compileSource("true");
      expect(instructionNames(code)).toContain("True");

      const code2 = compileSource("false");
      expect(instructionNames(code2)).toContain("False");
    });

    it("should compile nil", () => {
      const code = compileSource("nil");
      expect(instructionNames(code)).toContain("Nil");
    });
  });

  describe("operators", () => {
    it("should compile prefix operators", () => {
      const code = compileSource("-42");
      expect(instructionNames(code)).toContain("UnaryNegative");

      const code2 = compileSource("!true");
      expect(instructionNames(code2)).toContain("UnaryNot");
    });

    it("should compile binary operators", () => {
      const code = compileSource("1 + 2");
      expect(instructionNames(code)).toContain("BinaryOp");
    });

    it("should compile comparison operators", () => {
      const code = compileSource("1 < 2");
      expect(instructionNames(code)).toContain("CompareOp");
    });

    it("should compile short-circuit and", () => {
      const code = compileSource("true && false");
      expect(instructionNames(code)).toContain("PopJumpForwardIfFalse");
    });

    it("should compile short-circuit or", () => {
      const code = compileSource("true || false");
      expect(instructionNames(code)).toContain("PopJumpForwardIfTrue");
    });

    it("should compile nullish coalesce", () => {
      const code = compileSource("nil ?? 42");
      expect(instructionNames(code)).toContain("PopJumpForwardIfNotNil");
    });
  });

  describe("collections", () => {
    it("should compile lists", () => {
      const code = compileSource("[1, 2, 3]");
      expect(instructionNames(code)).toContain("BuildList");
    });

    it("should compile empty list", () => {
      const code = compileSource("[]");
      expect(instructionNames(code)).toContain("BuildList");
    });

    it("should compile maps", () => {
      const code = compileSource("{a: 1, b: 2}");
      expect(instructionNames(code)).toContain("BuildMap");
    });

    it("should compile empty map", () => {
      const code = compileSource("{}");
      expect(instructionNames(code)).toContain("BuildMap");
    });
  });

  describe("variables", () => {
    it("should compile let statements", () => {
      const code = compileSource("let x = 42");
      expect(code.localNames).toContain("x");
      expect(instructionNames(code)).toContain("StoreGlobal");
    });

    it("should compile const statements", () => {
      const code = compileSource("const x = 42");
      expect(code.localNames).toContain("x");
    });

    it("should compile variable access", () => {
      const code = compileSource("let x = 42\nx");
      expect(instructionNames(code)).toContain("LoadGlobal");
    });

    it("should compile assignment", () => {
      const code = compileSource("let x = 1\nx = 2");
      const names = instructionNames(code);
      expect(names.filter((n) => n === "StoreGlobal").length).toBe(2);
    });

    it("should compile compound assignment", () => {
      const code = compileSource("let x = 1\nx += 2");
      expect(instructionNames(code)).toContain("BinaryOp");
    });

    it("should compile postfix operators", () => {
      const code = compileSource("let x = 1\nx++");
      expect(instructionNames(code)).toContain("BinaryOp");
    });
  });

  describe("functions", () => {
    it("should compile function expressions", () => {
      const code = compileSource("function(x) { return x }");
      expect(code.constants.length).toBeGreaterThan(0);
    });

    it("should compile named functions", () => {
      const code = compileSource("function add(a, b) { return a + b }");
      expect(code.constants.length).toBeGreaterThan(0);
    });

    it("should compile arrow functions", () => {
      const code = compileSource("x => x * 2");
      expect(code.constants.length).toBeGreaterThan(0);
    });

    it("should compile arrow functions with parens", () => {
      const code = compileSource("(x, y) => x + y");
      expect(code.constants.length).toBeGreaterThan(0);
    });

    it("should compile function calls", () => {
      const code = compileSource("let f = x => x\nf(42)");
      expect(instructionNames(code)).toContain("Call");
    });
  });

  describe("closures", () => {
    it("should compile closures with free variables", () => {
      const code = compileSource(`
        function outer() {
          let x = 1
          function inner() { return x }
          return inner
        }
      `);
      // The inner function should capture x
      expect(code.constants.length).toBeGreaterThan(0);
    });
  });

  describe("control flow", () => {
    it("should compile if expressions", () => {
      const code = compileSource("if true { 1 }");
      expect(instructionNames(code)).toContain("PopJumpForwardIfFalse");
    });

    it("should compile if-else expressions", () => {
      const code = compileSource("if true { 1 } else { 2 }");
      expect(instructionNames(code)).toContain("JumpForward");
    });

    it("should compile switch expressions", () => {
      const code = compileSource("switch (1) { case 1: 2 }");
      expect(instructionNames(code)).toContain("CompareOp");
    });

    it("should compile match expressions", () => {
      const code = compileSource('match 1 { 1 => "one", _ => "other" }');
      expect(instructionNames(code)).toContain("CompareOp");
    });
  });

  describe("access expressions", () => {
    it("should compile property access", () => {
      const code = compileSource("let obj = {}\nobj.prop");
      expect(instructionNames(code)).toContain("LoadAttr");
    });

    it("should compile optional chaining", () => {
      const code = compileSource("let obj = nil\nobj?.prop");
      expect(instructionNames(code)).toContain("PopJumpForwardIfNil");
    });

    it("should compile index access", () => {
      const code = compileSource("let arr = []\narr[0]");
      expect(instructionNames(code)).toContain("BinarySubscr");
    });

    it("should compile slicing", () => {
      const code = compileSource("let arr = []\narr[1:3]");
      expect(instructionNames(code)).toContain("Slice");
    });
  });

  describe("membership", () => {
    it("should compile in operator", () => {
      const code = compileSource("1 in [1, 2, 3]");
      expect(instructionNames(code)).toContain("ContainsOp");
    });

    it("should compile not in operator", () => {
      const code = compileSource("4 not in [1, 2, 3]");
      expect(instructionNames(code)).toContain("ContainsOp");
      expect(instructionNames(code)).toContain("UnaryNot");
    });
  });

  describe("exception handling", () => {
    it("should compile try-catch", () => {
      const code = compileSource("try { 1 } catch { 2 }");
      expect(instructionNames(code)).toContain("PushExcept");
      expect(instructionNames(code)).toContain("PopExcept");
    });

    it("should compile try-finally", () => {
      const code = compileSource("try { 1 } finally { 2 }");
      expect(instructionNames(code)).toContain("EndFinally");
    });

    it("should compile throw", () => {
      const code = compileSource('throw "error"');
      expect(instructionNames(code)).toContain("Throw");
    });
  });

  describe("return statements", () => {
    it("should compile return with value", () => {
      const code = compileSource("function f() { return 42 }\nf");
      // ReturnValue is in the child code (function), not main
      expect(code.constants.length).toBeGreaterThan(0);
      const funcConst = code.constants.find(
        (c) => c.type === ConstantType.Function
      ) as Constant;
      expect(funcConst).toBeDefined();
      const funcCode = funcConst.value as Code;
      expect(instructionNames(funcCode)).toContain("ReturnValue");
    });

    it("should compile empty return", () => {
      const code = compileSource("function f() { return }\nf");
      // ReturnValue is in the child code (function), not main
      const funcConst = code.constants.find(
        (c) => c.type === ConstantType.Function
      ) as Constant;
      expect(funcConst).toBeDefined();
      const funcCode = funcConst.value as Code;
      expect(instructionNames(funcCode)).toContain("ReturnValue");
    });
  });

  describe("destructuring", () => {
    it("should compile object destructuring", () => {
      const code = compileSource("let { a, b } = {a: 1, b: 2}");
      expect(code.localNames).toContain("a");
      expect(code.localNames).toContain("b");
    });

    it("should compile array destructuring", () => {
      const code = compileSource("let [a, b] = [1, 2]");
      expect(code.localNames).toContain("a");
      expect(code.localNames).toContain("b");
    });

    it("should compile multi-variable declarations", () => {
      const code = compileSource("let x, y = [1, 2]");
      expect(code.localNames).toContain("x");
      expect(code.localNames).toContain("y");
      expect(instructionNames(code)).toContain("Unpack");
    });
  });

  describe("error handling", () => {
    it("should throw on undefined variable", () => {
      expect(() => compileSource("x")).toThrow(CompilerError);
    });

    it("should throw on assignment to constant", () => {
      expect(() => compileSource("const x = 1\nx = 2")).toThrow(CompilerError);
    });
  });
});

describe("Code", () => {
  it("should be immutable", () => {
    const code = compileSource("42");
    expect(() => {
      (code as any).instructions.push(0);
    }).toThrow();
  });

  it("should track source locations", () => {
    const code = compileSource("42");
    const loc = code.getLocation(0);
    expect(loc).toBeDefined();
    expect(loc!.line).toBe(0);
    expect(loc!.column).toBe(0);
  });
});
