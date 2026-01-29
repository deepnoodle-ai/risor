import { describe, it, expect } from "vitest";
import { parse, ParserError } from "./parser.js";
import * as ast from "../ast/nodes.js";

describe("Parser", () => {
  describe("literals", () => {
    it("should parse integers", () => {
      const prog = parse("42");
      expect(prog.stmts).toHaveLength(1);
      const expr = prog.stmts[0] as ast.IntLit;
      expect(expr).toBeInstanceOf(ast.IntLit);
      expect(expr.value).toBe(42n);
    });

    it("should parse hex integers", () => {
      const prog = parse("0xFF");
      const expr = prog.stmts[0] as ast.IntLit;
      expect(expr.value).toBe(255n);
    });

    it("should parse binary integers", () => {
      const prog = parse("0b1010");
      const expr = prog.stmts[0] as ast.IntLit;
      expect(expr.value).toBe(10n);
    });

    it("should parse floats", () => {
      const prog = parse("3.14");
      const expr = prog.stmts[0] as ast.FloatLit;
      expect(expr).toBeInstanceOf(ast.FloatLit);
      expect(expr.value).toBeCloseTo(3.14);
    });

    it("should parse strings", () => {
      const prog = parse('"hello"');
      const expr = prog.stmts[0] as ast.StringLit;
      expect(expr).toBeInstanceOf(ast.StringLit);
      expect(expr.value).toBe("hello");
    });

    it("should parse booleans", () => {
      const prog = parse("true");
      const expr = prog.stmts[0] as ast.BoolLit;
      expect(expr).toBeInstanceOf(ast.BoolLit);
      expect(expr.value).toBe(true);
    });

    it("should parse nil", () => {
      const prog = parse("nil");
      const expr = prog.stmts[0] as ast.NilLit;
      expect(expr).toBeInstanceOf(ast.NilLit);
    });
  });

  describe("identifiers", () => {
    it("should parse identifiers", () => {
      const prog = parse("foo");
      const expr = prog.stmts[0] as ast.Ident;
      expect(expr).toBeInstanceOf(ast.Ident);
      expect(expr.name).toBe("foo");
    });
  });

  describe("prefix expressions", () => {
    it("should parse negation", () => {
      const prog = parse("-5");
      const expr = prog.stmts[0] as ast.PrefixExpr;
      expect(expr).toBeInstanceOf(ast.PrefixExpr);
      expect(expr.op).toBe("-");
      expect((expr.right as ast.IntLit).value).toBe(5n);
    });

    it("should parse bang", () => {
      const prog = parse("!true");
      const expr = prog.stmts[0] as ast.PrefixExpr;
      expect(expr.op).toBe("!");
    });

    it("should parse not", () => {
      const prog = parse("not true");
      const expr = prog.stmts[0] as ast.PrefixExpr;
      expect(expr.op).toBe("not");
    });
  });

  describe("infix expressions", () => {
    it("should parse addition", () => {
      const prog = parse("1 + 2");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr).toBeInstanceOf(ast.InfixExpr);
      expect(expr.op).toBe("+");
      expect((expr.left as ast.IntLit).value).toBe(1n);
      expect((expr.right as ast.IntLit).value).toBe(2n);
    });

    it("should parse multiplication with higher precedence", () => {
      const prog = parse("1 + 2 * 3");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr.op).toBe("+");
      expect((expr.left as ast.IntLit).value).toBe(1n);
      const right = expr.right as ast.InfixExpr;
      expect(right.op).toBe("*");
    });

    it("should parse power operator (right-associative)", () => {
      const prog = parse("2 ** 3 ** 2");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr.op).toBe("**");
      expect((expr.left as ast.IntLit).value).toBe(2n);
      const right = expr.right as ast.InfixExpr;
      expect(right.op).toBe("**");
      expect((right.left as ast.IntLit).value).toBe(3n);
      expect((right.right as ast.IntLit).value).toBe(2n);
    });

    it("should parse comparison operators", () => {
      const prog = parse("a == b");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr.op).toBe("==");
    });

    it("should parse logical operators", () => {
      const prog = parse("a && b || c");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr.op).toBe("||");
    });

    it("should parse nullish coalescing", () => {
      const prog = parse("a ?? b");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr.op).toBe("??");
    });
  });

  describe("grouped expressions", () => {
    it("should parse parentheses", () => {
      const prog = parse("(1 + 2) * 3");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr.op).toBe("*");
      const left = expr.left as ast.InfixExpr;
      expect(left.op).toBe("+");
    });
  });

  describe("collections", () => {
    it("should parse list literals", () => {
      const prog = parse("[1, 2, 3]");
      const expr = prog.stmts[0] as ast.ListLit;
      expect(expr).toBeInstanceOf(ast.ListLit);
      expect(expr.items).toHaveLength(3);
    });

    it("should parse empty list", () => {
      const prog = parse("[]");
      const expr = prog.stmts[0] as ast.ListLit;
      expect(expr.items).toHaveLength(0);
    });

    it("should parse map literals", () => {
      const prog = parse("{a: 1, b: 2}");
      const expr = prog.stmts[0] as ast.MapLit;
      expect(expr).toBeInstanceOf(ast.MapLit);
      expect(expr.items).toHaveLength(2);
    });

    it("should parse empty map", () => {
      const prog = parse("{}");
      const expr = prog.stmts[0] as ast.MapLit;
      expect(expr.items).toHaveLength(0);
    });

    it("should parse map shorthand", () => {
      const prog = parse("{a, b}");
      const expr = prog.stmts[0] as ast.MapLit;
      expect(expr.items).toHaveLength(2);
      // Shorthand: {a} becomes {a: a}
      expect((expr.items[0].key as ast.Ident).name).toBe("a");
      expect((expr.items[0].value as ast.Ident).name).toBe("a");
    });

    it("should parse spread in map", () => {
      const prog = parse("{...a, b: 1}");
      const expr = prog.stmts[0] as ast.MapLit;
      expect(expr.items).toHaveLength(2);
      expect(expr.items[0].key).toBeNull();
    });
  });

  describe("function calls", () => {
    it("should parse function call", () => {
      const prog = parse("foo()");
      const expr = prog.stmts[0] as ast.CallExpr;
      expect(expr).toBeInstanceOf(ast.CallExpr);
      expect((expr.func as ast.Ident).name).toBe("foo");
      expect(expr.args).toHaveLength(0);
    });

    it("should parse function call with args", () => {
      const prog = parse("foo(1, 2)");
      const expr = prog.stmts[0] as ast.CallExpr;
      expect(expr.args).toHaveLength(2);
    });

    it("should parse method call", () => {
      const prog = parse("obj.method()");
      const expr = prog.stmts[0] as ast.ObjectCallExpr;
      expect(expr).toBeInstanceOf(ast.ObjectCallExpr);
      expect(expr.optional).toBe(false);
    });

    it("should parse optional method call", () => {
      const prog = parse("obj?.method()");
      const expr = prog.stmts[0] as ast.ObjectCallExpr;
      expect(expr.optional).toBe(true);
    });
  });

  describe("property access", () => {
    it("should parse property access", () => {
      const prog = parse("obj.prop");
      const expr = prog.stmts[0] as ast.GetAttrExpr;
      expect(expr).toBeInstanceOf(ast.GetAttrExpr);
      expect(expr.attr.name).toBe("prop");
    });

    it("should parse optional chaining", () => {
      const prog = parse("obj?.prop");
      const expr = prog.stmts[0] as ast.GetAttrExpr;
      expect(expr.optional).toBe(true);
    });

    it("should parse chained access", () => {
      const prog = parse("a.b.c");
      const expr = prog.stmts[0] as ast.GetAttrExpr;
      expect(expr.attr.name).toBe("c");
      const inner = expr.object as ast.GetAttrExpr;
      expect(inner.attr.name).toBe("b");
    });
  });

  describe("index expressions", () => {
    it("should parse index access", () => {
      const prog = parse("arr[0]");
      const expr = prog.stmts[0] as ast.IndexExpr;
      expect(expr).toBeInstanceOf(ast.IndexExpr);
    });

    it("should parse slice", () => {
      const prog = parse("arr[1:3]");
      const expr = prog.stmts[0] as ast.SliceExpr;
      expect(expr).toBeInstanceOf(ast.SliceExpr);
      expect((expr.low as ast.IntLit).value).toBe(1n);
      expect((expr.high as ast.IntLit).value).toBe(3n);
    });

    it("should parse slice with omitted bounds", () => {
      const prog = parse("arr[:3]");
      const expr = prog.stmts[0] as ast.SliceExpr;
      expect(expr.low).toBeNull();

      const prog2 = parse("arr[1:]");
      const expr2 = prog2.stmts[0] as ast.SliceExpr;
      expect(expr2.high).toBeNull();
    });
  });

  describe("if expressions", () => {
    it("should parse if expression", () => {
      const prog = parse("if (x) { y }");
      const expr = prog.stmts[0] as ast.IfExpr;
      expect(expr).toBeInstanceOf(ast.IfExpr);
      expect((expr.condition as ast.Ident).name).toBe("x");
    });

    it("should parse if-else", () => {
      const prog = parse("if (x) { y } else { z }");
      const expr = prog.stmts[0] as ast.IfExpr;
      expect(expr.alternative).not.toBeNull();
    });

    it("should parse if without parentheses", () => {
      const prog = parse("if x { y }");
      const expr = prog.stmts[0] as ast.IfExpr;
      expect((expr.condition as ast.Ident).name).toBe("x");
    });
  });

  describe("switch expressions", () => {
    it("should parse switch", () => {
      const prog = parse("switch (x) { case 1: y }");
      const expr = prog.stmts[0] as ast.SwitchExpr;
      expect(expr).toBeInstanceOf(ast.SwitchExpr);
      expect(expr.cases).toHaveLength(1);
    });

    it("should parse default case", () => {
      const prog = parse("switch (x) { case 1: y\n default: z }");
      const expr = prog.stmts[0] as ast.SwitchExpr;
      expect(expr.cases).toHaveLength(2);
      expect(expr.cases[1].isDefault).toBe(true);
    });
  });

  describe("match expressions", () => {
    it("should parse match", () => {
      const prog = parse('match x { 1 => "one", _ => "other" }');
      const expr = prog.stmts[0] as ast.MatchExpr;
      expect(expr).toBeInstanceOf(ast.MatchExpr);
      expect(expr.arms).toHaveLength(1);
      expect(expr.defaultArm).not.toBeNull();
    });

    it("should parse match with wildcard", () => {
      const prog = parse("match x { _ => 0 }");
      const expr = prog.stmts[0] as ast.MatchExpr;
      expect(expr.defaultArm?.pattern).toBeInstanceOf(ast.WildcardPattern);
    });
  });

  describe("function expressions", () => {
    it("should parse anonymous function", () => {
      const prog = parse("function() { return 1 }");
      const expr = prog.stmts[0] as ast.FuncLit;
      expect(expr).toBeInstanceOf(ast.FuncLit);
      expect(expr.name).toBeNull();
    });

    it("should parse named function", () => {
      const prog = parse("function add(a, b) { return a + b }");
      const expr = prog.stmts[0] as ast.FuncLit;
      expect(expr.name?.name).toBe("add");
      expect(expr.params).toHaveLength(2);
    });

    it("should parse arrow function", () => {
      const prog = parse("x => x * 2");
      const expr = prog.stmts[0] as ast.FuncLit;
      expect(expr).toBeInstanceOf(ast.FuncLit);
      expect(expr.params).toHaveLength(1);
    });

    it("should parse arrow function with parens", () => {
      const prog = parse("(x, y) => x + y");
      const expr = prog.stmts[0] as ast.FuncLit;
      expect(expr.params).toHaveLength(2);
    });

    it("should parse arrow function with block body", () => {
      const prog = parse("x => { return x * 2 }");
      const expr = prog.stmts[0] as ast.FuncLit;
      expect(expr.body.stmts).toHaveLength(1);
    });

    it("should parse function with default params", () => {
      const prog = parse("function foo(a, b = 1) { return a + b }");
      const expr = prog.stmts[0] as ast.FuncLit;
      expect(expr.defaults.has("b")).toBe(true);
    });

    it("should parse function with rest param", () => {
      const prog = parse("function foo(...args) { return args }");
      const expr = prog.stmts[0] as ast.FuncLit;
      expect(expr.restParam?.name).toBe("args");
    });
  });

  describe("statements", () => {
    describe("let statements", () => {
      it("should parse let statement", () => {
        const prog = parse("let x = 42");
        const stmt = prog.stmts[0] as ast.VarStmt;
        expect(stmt).toBeInstanceOf(ast.VarStmt);
        expect(stmt.name.name).toBe("x");
      });

      it("should parse multi-var let", () => {
        const prog = parse("let x, y = [1, 2]");
        const stmt = prog.stmts[0] as ast.MultiVarStmt;
        expect(stmt).toBeInstanceOf(ast.MultiVarStmt);
        expect(stmt.names).toHaveLength(2);
      });

      it("should parse object destructuring", () => {
        const prog = parse("let { a, b } = obj");
        const stmt = prog.stmts[0] as ast.ObjectDestructureStmt;
        expect(stmt).toBeInstanceOf(ast.ObjectDestructureStmt);
        expect(stmt.bindings).toHaveLength(2);
      });

      it("should parse object destructuring with alias", () => {
        const prog = parse("let { a: x } = obj");
        const stmt = prog.stmts[0] as ast.ObjectDestructureStmt;
        expect(stmt.bindings[0].alias).toBe("x");
      });

      it("should parse array destructuring", () => {
        const prog = parse("let [a, b] = arr");
        const stmt = prog.stmts[0] as ast.ArrayDestructureStmt;
        expect(stmt).toBeInstanceOf(ast.ArrayDestructureStmt);
        expect(stmt.elements).toHaveLength(2);
      });
    });

    describe("const statements", () => {
      it("should parse const statement", () => {
        const prog = parse("const X = 42");
        const stmt = prog.stmts[0] as ast.ConstStmt;
        expect(stmt).toBeInstanceOf(ast.ConstStmt);
        expect(stmt.name.name).toBe("X");
      });
    });

    describe("return statements", () => {
      it("should parse return with value", () => {
        const prog = parse("return 42");
        const stmt = prog.stmts[0] as ast.ReturnStmt;
        expect(stmt).toBeInstanceOf(ast.ReturnStmt);
        expect(stmt.value).not.toBeNull();
      });

      it("should parse empty return", () => {
        const prog = parse("return\n");
        const stmt = prog.stmts[0] as ast.ReturnStmt;
        expect(stmt.value).toBeNull();
      });
    });

    describe("assignment statements", () => {
      it("should parse simple assignment", () => {
        const prog = parse("x = 42");
        const stmt = prog.stmts[0] as ast.AssignStmt;
        expect(stmt).toBeInstanceOf(ast.AssignStmt);
        expect(stmt.op).toBe("=");
      });

      it("should parse compound assignment", () => {
        const prog = parse("x += 1");
        const stmt = prog.stmts[0] as ast.AssignStmt;
        expect(stmt.op).toBe("+=");
      });

      it("should parse property assignment", () => {
        const prog = parse("obj.prop = 42");
        const stmt = prog.stmts[0] as ast.SetAttrStmt;
        expect(stmt).toBeInstanceOf(ast.SetAttrStmt);
      });

      it("should parse index assignment", () => {
        const prog = parse("arr[0] = 42");
        const stmt = prog.stmts[0] as ast.AssignStmt;
        expect(stmt.target).toBeInstanceOf(ast.IndexExpr);
      });
    });

    describe("postfix statements", () => {
      it("should parse increment", () => {
        const prog = parse("x++");
        const stmt = prog.stmts[0] as ast.PostfixStmt;
        expect(stmt).toBeInstanceOf(ast.PostfixStmt);
        expect(stmt.op).toBe("++");
      });

      it("should parse decrement", () => {
        const prog = parse("x--");
        const stmt = prog.stmts[0] as ast.PostfixStmt;
        expect(stmt.op).toBe("--");
      });
    });

    describe("throw statements", () => {
      it("should parse throw", () => {
        const prog = parse('throw "error"');
        const stmt = prog.stmts[0] as ast.ThrowStmt;
        expect(stmt).toBeInstanceOf(ast.ThrowStmt);
      });
    });
  });

  describe("try/catch/finally", () => {
    it("should parse try-catch", () => {
      const prog = parse("try { foo() } catch e { bar() }");
      const expr = prog.stmts[0] as ast.TryExpr;
      expect(expr).toBeInstanceOf(ast.TryExpr);
      expect(expr.catchIdent?.name).toBe("e");
      expect(expr.catchBlock).not.toBeNull();
    });

    it("should parse try-finally", () => {
      const prog = parse("try { foo() } finally { cleanup() }");
      const expr = prog.stmts[0] as ast.TryExpr;
      expect(expr.catchBlock).toBeNull();
      expect(expr.finallyBlock).not.toBeNull();
    });

    it("should parse try-catch-finally", () => {
      const prog = parse("try { foo() } catch { bar() } finally { cleanup() }");
      const expr = prog.stmts[0] as ast.TryExpr;
      expect(expr.catchBlock).not.toBeNull();
      expect(expr.finallyBlock).not.toBeNull();
    });
  });

  describe("in/not in expressions", () => {
    it("should parse in expression", () => {
      const prog = parse("x in list");
      const expr = prog.stmts[0] as ast.InExpr;
      expect(expr).toBeInstanceOf(ast.InExpr);
    });

    it("should parse not in expression", () => {
      const prog = parse("x not in list");
      const expr = prog.stmts[0] as ast.NotInExpr;
      expect(expr).toBeInstanceOf(ast.NotInExpr);
    });
  });

  describe("pipe expressions", () => {
    it("should parse pipe", () => {
      const prog = parse("x | f | g");
      const expr = prog.stmts[0] as ast.PipeExpr;
      expect(expr).toBeInstanceOf(ast.PipeExpr);
      expect(expr.exprs).toHaveLength(3);
    });
  });

  describe("spread expressions", () => {
    it("should parse spread in list", () => {
      const prog = parse("[...a, 1, 2]");
      const expr = prog.stmts[0] as ast.ListLit;
      expect(expr.items[0]).toBeInstanceOf(ast.SpreadExpr);
    });

    it("should parse spread in call", () => {
      const prog = parse("foo(...args)");
      const expr = prog.stmts[0] as ast.CallExpr;
      expect(expr.args[0]).toBeInstanceOf(ast.SpreadExpr);
    });
  });

  describe("complex expressions", () => {
    it("should parse chained method calls", () => {
      const prog = parse("arr.filter(x => x > 0).map(x => x * 2)");
      const expr = prog.stmts[0] as ast.ObjectCallExpr;
      expect(expr).toBeInstanceOf(ast.ObjectCallExpr);
    });

    it("should parse nested function calls", () => {
      const prog = parse("foo(bar(baz()))");
      const expr = prog.stmts[0] as ast.CallExpr;
      expect(expr).toBeInstanceOf(ast.CallExpr);
    });

    it("should parse complex precedence", () => {
      const prog = parse("1 + 2 * 3 ** 4 == 5");
      const expr = prog.stmts[0] as ast.InfixExpr;
      expect(expr.op).toBe("==");
    });
  });

  describe("multiline", () => {
    it("should parse multiple statements", () => {
      const prog = parse("let x = 1\nlet y = 2\nx + y");
      expect(prog.stmts).toHaveLength(3);
    });

    it("should allow chaining after newline", () => {
      const prog = parse("obj\n  .method()");
      const expr = prog.stmts[0] as ast.ObjectCallExpr;
      expect(expr).toBeInstanceOf(ast.ObjectCallExpr);
    });

    it("should allow newlines in collections", () => {
      const prog = parse("[\n  1,\n  2,\n  3\n]");
      const expr = prog.stmts[0] as ast.ListLit;
      expect(expr.items).toHaveLength(3);
    });
  });

  describe("error handling", () => {
    it("should throw on unexpected token", () => {
      expect(() => parse("let = 42")).toThrow(ParserError);
    });

    it("should throw on unclosed paren", () => {
      expect(() => parse("(1 + 2")).toThrow(ParserError);
    });

    it("should throw on unclosed bracket", () => {
      expect(() => parse("[1, 2")).toThrow(ParserError);
    });

    it("should throw on unclosed brace", () => {
      expect(() => parse("{a: 1")).toThrow(ParserError);
    });
  });
});
