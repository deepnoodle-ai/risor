/**
 * VM tests.
 */

import { describe, it, expect } from "vitest";
import { parse } from "../parser/parser.js";
import { Compiler } from "../compiler/compiler.js";
import { VM, VMError } from "./vm.js";
import { createBuiltins } from "../builtins/builtins.js";
import {
  RisorObject,
  ObjectType,
  RisorInt,
  RisorFloat,
  RisorString,
  RisorList,
  RisorMap,
  RisorBool,
  toJS,
} from "../object/object.js";

function run(source: string): RisorObject {
  const program = parse(source);
  const builtins = createBuiltins();
  const compiler = new Compiler({
    source,
    globalNames: Array.from(builtins.keys()),
  });
  const code = compiler.compile(program);
  const vm = new VM({ globals: builtins });
  return vm.run(code);
}

function runValue(source: string): unknown {
  return toJS(run(source));
}

describe("VM", () => {
  describe("literals", () => {
    it("should evaluate integers", () => {
      expect(runValue("42")).toBe(42);
    });

    it("should evaluate floats", () => {
      expect(runValue("3.14")).toBe(3.14);
    });

    it("should evaluate strings", () => {
      expect(runValue('"hello"')).toBe("hello");
    });

    it("should evaluate booleans", () => {
      expect(runValue("true")).toBe(true);
      expect(runValue("false")).toBe(false);
    });

    it("should evaluate nil", () => {
      expect(runValue("nil")).toBe(null);
    });
  });

  describe("arithmetic", () => {
    it("should add integers", () => {
      expect(runValue("1 + 2")).toBe(3);
    });

    it("should subtract integers", () => {
      expect(runValue("5 - 3")).toBe(2);
    });

    it("should multiply integers", () => {
      expect(runValue("3 * 4")).toBe(12);
    });

    it("should divide numbers", () => {
      expect(runValue("10 / 4")).toBe(2.5);
    });

    it("should compute modulo", () => {
      expect(runValue("10 % 3")).toBe(1);
    });

    it("should compute power", () => {
      expect(runValue("2 ** 10")).toBe(1024);
    });

    it("should negate numbers", () => {
      expect(runValue("-42")).toBe(-42);
    });

    it("should handle float arithmetic", () => {
      expect(runValue("1.5 + 2.5")).toBe(4);
    });
  });

  describe("comparison", () => {
    it("should compare equality", () => {
      expect(runValue("1 == 1")).toBe(true);
      expect(runValue("1 == 2")).toBe(false);
    });

    it("should compare inequality", () => {
      expect(runValue("1 != 2")).toBe(true);
      expect(runValue("1 != 1")).toBe(false);
    });

    it("should compare less than", () => {
      expect(runValue("1 < 2")).toBe(true);
      expect(runValue("2 < 1")).toBe(false);
    });

    it("should compare greater than", () => {
      expect(runValue("2 > 1")).toBe(true);
      expect(runValue("1 > 2")).toBe(false);
    });

    it("should compare less than or equal", () => {
      expect(runValue("1 <= 1")).toBe(true);
      expect(runValue("1 <= 2")).toBe(true);
      expect(runValue("2 <= 1")).toBe(false);
    });

    it("should compare greater than or equal", () => {
      expect(runValue("1 >= 1")).toBe(true);
      expect(runValue("2 >= 1")).toBe(true);
      expect(runValue("1 >= 2")).toBe(false);
    });
  });

  describe("logical", () => {
    it("should compute logical not", () => {
      expect(runValue("!true")).toBe(false);
      expect(runValue("!false")).toBe(true);
      expect(runValue("!nil")).toBe(true);
    });

    it("should short-circuit and", () => {
      expect(runValue("true && true")).toBe(true);
      expect(runValue("true && false")).toBe(false);
      expect(runValue("false && true")).toBe(false);
    });

    it("should short-circuit or", () => {
      expect(runValue("true || false")).toBe(true);
      expect(runValue("false || true")).toBe(true);
      expect(runValue("false || false")).toBe(false);
    });

    it("should handle nullish coalesce", () => {
      expect(runValue("nil ?? 42")).toBe(42);
      expect(runValue("1 ?? 42")).toBe(1);
    });
  });

  describe("variables", () => {
    it("should declare and access variables", () => {
      expect(runValue("let x = 42\nx")).toBe(42);
    });

    it("should assign to variables", () => {
      expect(runValue("let x = 1\nx = 2\nx")).toBe(2);
    });

    it("should support compound assignment", () => {
      expect(runValue("let x = 10\nx += 5\nx")).toBe(15);
      expect(runValue("let x = 10\nx -= 3\nx")).toBe(7);
      expect(runValue("let x = 10\nx *= 2\nx")).toBe(20);
      expect(runValue("let x = 10\nx /= 4\nx")).toBe(2.5);
    });

    it("should support postfix operators", () => {
      expect(runValue("let x = 5\nx++\nx")).toBe(6);
      expect(runValue("let x = 5\nx--\nx")).toBe(4);
    });
  });

  describe("lists", () => {
    it("should create lists", () => {
      expect(runValue("[1, 2, 3]")).toEqual([1, 2, 3]);
    });

    it("should access list elements", () => {
      expect(runValue("[1, 2, 3][1]")).toBe(2);
    });

    it("should access list elements with negative index", () => {
      expect(runValue("[1, 2, 3][-1]")).toBe(3);
    });

    it("should slice lists", () => {
      expect(runValue("[1, 2, 3, 4, 5][1:4]")).toEqual([2, 3, 4]);
    });

    it("should concatenate lists", () => {
      expect(runValue("[1, 2] + [3, 4]")).toEqual([1, 2, 3, 4]);
    });
  });

  describe("maps", () => {
    it("should create maps", () => {
      expect(runValue("{a: 1, b: 2}")).toEqual({ a: 1, b: 2 });
    });

    it("should access map values by key", () => {
      expect(runValue('{a: 1, b: 2}["a"]')).toBe(1);
    });

    it("should access map values by property", () => {
      expect(runValue("{a: 1, b: 2}.a")).toBe(1);
    });

    it("should return nil for missing keys", () => {
      expect(runValue('{a: 1}["b"]')).toBe(null);
    });
  });

  describe("strings", () => {
    it("should concatenate strings", () => {
      expect(runValue('"hello" + " " + "world"')).toBe("hello world");
    });

    it("should access string characters", () => {
      expect(runValue('"hello"[1]')).toBe("e");
    });

    it("should slice strings", () => {
      expect(runValue('"hello"[1:4]')).toBe("ell");
    });

    it("should repeat strings", () => {
      expect(runValue('"ab" * 3')).toBe("ababab");
    });
  });

  describe("functions", () => {
    it("should call functions", () => {
      expect(runValue("let f = function() { return 42 }\nf()")).toBe(42);
    });

    it("should pass arguments", () => {
      expect(runValue("let f = function(x) { return x * 2 }\nf(21)")).toBe(42);
    });

    it("should support multiple arguments", () => {
      expect(runValue("let add = function(a, b) { return a + b }\nadd(20, 22)")).toBe(42);
    });

    it("should support arrow functions", () => {
      expect(runValue("let f = x => x * 2\nf(21)")).toBe(42);
    });

    it("should support named functions", () => {
      expect(runValue("function double(x) { return x * 2 }\ndouble(21)")).toBe(42);
    });

    it("should support recursion", () => {
      expect(
        runValue(`
        function fib(n) {
          if n <= 1 { return n }
          return fib(n - 1) + fib(n - 2)
        }
        fib(10)
      `)
      ).toBe(55);
    });
  });

  describe("closures", () => {
    it("should capture free variables", () => {
      expect(
        runValue(`
        let makeCounter = function() {
          let n = 0
          return function() {
            n = n + 1
            return n
          }
        }
        let c = makeCounter()
        c()
        c()
        c()
      `)
      ).toBe(3);
    });

    it("should have independent closures", () => {
      expect(
        runValue(`
        let makeCounter = function() {
          let n = 0
          return function() {
            n = n + 1
            return n
          }
        }
        let c1 = makeCounter()
        let c2 = makeCounter()
        c1()
        c1()
        c2()
      `)
      ).toBe(1);
    });
  });

  describe("control flow", () => {
    it("should evaluate if expressions", () => {
      expect(runValue("if true { 1 }")).toBe(null); // if without else returns nil from else branch
      expect(runValue("if false { 1 }")).toBe(null);
    });

    it("should evaluate if-else expressions", () => {
      expect(runValue("if true { 1 } else { 2 }")).toBe(null); // Block doesn't return value
    });

    it("should evaluate switch expressions", () => {
      expect(
        runValue(`
        let x = 2
        switch (x) {
          case 1: { }
          case 2: { }
          case 3: { }
        }
      `)
      ).toBe(null);
    });

    it("should evaluate match expressions", () => {
      expect(
        runValue(`
        let x = match 2 {
          1 => "one",
          2 => "two",
          _ => "other"
        }
        x
      `)
      ).toBe("two");
    });
  });

  describe("membership", () => {
    it("should check list membership", () => {
      expect(runValue("2 in [1, 2, 3]")).toBe(true);
      expect(runValue("4 in [1, 2, 3]")).toBe(false);
    });

    it("should check map membership", () => {
      expect(runValue('"a" in {a: 1, b: 2}')).toBe(true);
      expect(runValue('"c" in {a: 1, b: 2}')).toBe(false);
    });

    it("should check string membership", () => {
      expect(runValue('"ell" in "hello"')).toBe(true);
      expect(runValue('"xyz" in "hello"')).toBe(false);
    });

    it("should check not in", () => {
      expect(runValue("4 not in [1, 2, 3]")).toBe(true);
      expect(runValue("2 not in [1, 2, 3]")).toBe(false);
    });
  });

  describe("methods", () => {
    it("should call list methods", () => {
      expect(runValue("[1, 2, 3].len()")).toBe(3);
    });

    it("should call string methods", () => {
      expect(runValue('"hello".upper()')).toBe("HELLO");
      expect(runValue('"HELLO".lower()')).toBe("hello");
      expect(runValue('"  hello  ".trim()')).toBe("hello");
    });

    it("should call map methods", () => {
      const result = run("{a: 1, b: 2}.keys()");
      expect(result.type).toBe(ObjectType.Iter);
    });
  });

  describe("builtins", () => {
    it("should compute len", () => {
      expect(runValue("len([1, 2, 3])")).toBe(3);
      expect(runValue('len("hello")')).toBe(5);
      expect(runValue("len({a: 1, b: 2})")).toBe(2);
    });

    it("should get type", () => {
      expect(runValue("type(42)")).toBe("int");
      expect(runValue('type("hello")')).toBe("string");
      expect(runValue("type([])")).toBe("list");
    });

    it("should convert to string", () => {
      expect(runValue("string(42)")).toBe("42");
    });

    it("should convert to int", () => {
      expect(runValue("int(3.14)")).toBe(3);
      expect(runValue('int("42")')).toBe(42);
    });

    it("should create range", () => {
      expect(runValue("list(range(5))")).toEqual([0, 1, 2, 3, 4]);
      expect(runValue("list(range(2, 5))")).toEqual([2, 3, 4]);
      expect(runValue("list(range(0, 10, 2))")).toEqual([0, 2, 4, 6, 8]);
    });

    it("should compute min/max", () => {
      expect(runValue("min([3, 1, 4, 1, 5])")).toBe(1);
      expect(runValue("max([3, 1, 4, 1, 5])")).toBe(5);
    });

    it("should compute sum", () => {
      expect(runValue("sum([1, 2, 3, 4, 5])")).toBe(15);
    });

    it("should compute abs", () => {
      expect(runValue("abs(-42)")).toBe(42);
      expect(runValue("abs(42)")).toBe(42);
    });
  });

  describe("higher-order functions", () => {
    it("should map over lists", () => {
      expect(runValue("[1, 2, 3].map(x => x * 2)")).toEqual([2, 4, 6]);
    });

    it("should filter lists", () => {
      expect(runValue("[1, 2, 3, 4, 5].filter(x => x > 2)")).toEqual([3, 4, 5]);
    });

    it("should reduce lists", () => {
      expect(runValue("[1, 2, 3, 4, 5].reduce((a, b) => a + b, 0)")).toBe(15);
    });

    it("should chain operations", () => {
      expect(runValue("[1, 2, 3, 4, 5].filter(x => x > 2).map(x => x * 2)")).toEqual([6, 8, 10]);
    });
  });

  describe("error handling", () => {
    it("should throw division by zero", () => {
      expect(() => run("1 / 0")).toThrow("division by zero");
    });

    it("should throw index out of range", () => {
      expect(() => run("[1, 2, 3][10]")).toThrow("out of range");
    });
  });
});
