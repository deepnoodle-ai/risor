import { describe, it, expect } from "vitest";
import { Lexer, tokenize, LexerError } from "./lexer.js";
import { TokenKind } from "../token/token.js";

describe("Lexer", () => {
  describe("basic tokens", () => {
    it("should tokenize empty input", () => {
      const tokens = tokenize("");
      expect(tokens).toHaveLength(1);
      expect(tokens[0].kind).toBe(TokenKind.EOF);
    });

    it("should tokenize identifiers", () => {
      const tokens = tokenize("foo bar _baz");
      expect(tokens[0].kind).toBe(TokenKind.IDENT);
      expect(tokens[0].literal).toBe("foo");
      expect(tokens[1].kind).toBe(TokenKind.IDENT);
      expect(tokens[1].literal).toBe("bar");
      expect(tokens[2].kind).toBe(TokenKind.IDENT);
      expect(tokens[2].literal).toBe("_baz");
    });

    it("should tokenize keywords", () => {
      const input = "let const function return if else true false nil";
      const tokens = tokenize(input);
      expect(tokens[0].kind).toBe(TokenKind.LET);
      expect(tokens[1].kind).toBe(TokenKind.CONST);
      expect(tokens[2].kind).toBe(TokenKind.FUNCTION);
      expect(tokens[3].kind).toBe(TokenKind.RETURN);
      expect(tokens[4].kind).toBe(TokenKind.IF);
      expect(tokens[5].kind).toBe(TokenKind.ELSE);
      expect(tokens[6].kind).toBe(TokenKind.TRUE);
      expect(tokens[7].kind).toBe(TokenKind.FALSE);
      expect(tokens[8].kind).toBe(TokenKind.NIL);
    });
  });

  describe("numbers", () => {
    it("should tokenize integers", () => {
      const tokens = tokenize("42 0 123456789");
      expect(tokens[0].kind).toBe(TokenKind.INT);
      expect(tokens[0].literal).toBe("42");
      expect(tokens[1].kind).toBe(TokenKind.INT);
      expect(tokens[1].literal).toBe("0");
      expect(tokens[2].kind).toBe(TokenKind.INT);
      expect(tokens[2].literal).toBe("123456789");
    });

    it("should tokenize floats", () => {
      const tokens = tokenize("3.14 0.5 1.0 2.5e10 1e-5 3E+2");
      expect(tokens[0].kind).toBe(TokenKind.FLOAT);
      expect(tokens[0].literal).toBe("3.14");
      expect(tokens[1].kind).toBe(TokenKind.FLOAT);
      expect(tokens[1].literal).toBe("0.5");
      expect(tokens[2].kind).toBe(TokenKind.FLOAT);
      expect(tokens[2].literal).toBe("1.0");
      expect(tokens[3].kind).toBe(TokenKind.FLOAT);
      expect(tokens[3].literal).toBe("2.5e10");
      expect(tokens[4].kind).toBe(TokenKind.FLOAT);
      expect(tokens[4].literal).toBe("1e-5");
      expect(tokens[5].kind).toBe(TokenKind.FLOAT);
      expect(tokens[5].literal).toBe("3E+2");
    });

    it("should tokenize hexadecimal numbers", () => {
      const tokens = tokenize("0xFF 0x0 0xDEADBEEF 0xAbCd");
      expect(tokens[0].kind).toBe(TokenKind.INT);
      expect(tokens[0].literal).toBe("0xFF");
      expect(tokens[1].kind).toBe(TokenKind.INT);
      expect(tokens[1].literal).toBe("0x0");
      expect(tokens[2].kind).toBe(TokenKind.INT);
      expect(tokens[2].literal).toBe("0xDEADBEEF");
      expect(tokens[3].kind).toBe(TokenKind.INT);
      expect(tokens[3].literal).toBe("0xAbCd");
    });

    it("should tokenize binary numbers", () => {
      const tokens = tokenize("0b1010 0b0 0b11111111");
      expect(tokens[0].kind).toBe(TokenKind.INT);
      expect(tokens[0].literal).toBe("0b1010");
      expect(tokens[1].kind).toBe(TokenKind.INT);
      expect(tokens[1].literal).toBe("0b0");
      expect(tokens[2].kind).toBe(TokenKind.INT);
      expect(tokens[2].literal).toBe("0b11111111");
    });

    it("should tokenize octal numbers", () => {
      const tokens = tokenize("0755 0644");
      expect(tokens[0].kind).toBe(TokenKind.INT);
      expect(tokens[0].literal).toBe("0755");
      expect(tokens[1].kind).toBe(TokenKind.INT);
      expect(tokens[1].literal).toBe("0644");
    });

    it("should error on invalid number literals", () => {
      expect(() => tokenize("123abc")).toThrow(LexerError);
      expect(() => tokenize("0xGHI")).toThrow(LexerError);
    });
  });

  describe("strings", () => {
    it("should tokenize double-quoted strings", () => {
      const tokens = tokenize('"hello" "world"');
      expect(tokens[0].kind).toBe(TokenKind.STRING);
      expect(tokens[0].literal).toBe("hello");
      expect(tokens[1].kind).toBe(TokenKind.STRING);
      expect(tokens[1].literal).toBe("world");
    });

    it("should tokenize single-quoted strings", () => {
      const tokens = tokenize("'hello' 'world'");
      expect(tokens[0].kind).toBe(TokenKind.STRING);
      expect(tokens[0].literal).toBe("hello");
      expect(tokens[1].kind).toBe(TokenKind.STRING);
      expect(tokens[1].literal).toBe("world");
    });

    it("should handle escape sequences", () => {
      const tokens = tokenize('"hello\\nworld" "tab\\there" "quote\\"here"');
      expect(tokens[0].literal).toBe("hello\nworld");
      expect(tokens[1].literal).toBe("tab\there");
      expect(tokens[2].literal).toBe('quote"here');
    });

    it("should handle hex escapes", () => {
      const tokens = tokenize('"\\x41\\x42\\x43"');
      expect(tokens[0].literal).toBe("ABC");
    });

    it("should handle unicode escapes", () => {
      const tokens = tokenize('"\\u0048\\u0065\\u006C\\u006C\\u006F"');
      expect(tokens[0].literal).toBe("Hello");
    });

    it("should handle octal escapes", () => {
      const tokens = tokenize('"\\101\\102\\103"');
      expect(tokens[0].literal).toBe("ABC");
    });

    it("should error on unterminated strings", () => {
      expect(() => tokenize('"hello')).toThrow(LexerError);
      expect(() => tokenize('"hello\n"')).toThrow(LexerError);
    });

    it("should error on invalid escape sequences", () => {
      expect(() => tokenize('"\\z"')).toThrow(LexerError);
    });
  });

  describe("template literals", () => {
    it("should tokenize backtick strings", () => {
      const tokens = tokenize("`hello world`");
      expect(tokens[0].kind).toBe(TokenKind.TEMPLATE);
      expect(tokens[0].literal).toBe("hello world");
    });

    it("should handle multiline template literals", () => {
      const tokens = tokenize("`hello\nworld`");
      expect(tokens[0].kind).toBe(TokenKind.TEMPLATE);
      expect(tokens[0].literal).toBe("hello\nworld");
    });

    it("should not process escapes in template literals", () => {
      const tokens = tokenize("`hello\\nworld`");
      expect(tokens[0].literal).toBe("hello\\nworld");
    });

    it("should error on unterminated template literals", () => {
      expect(() => tokenize("`hello")).toThrow(LexerError);
    });
  });

  describe("operators", () => {
    it("should tokenize arithmetic operators", () => {
      const tokens = tokenize("+ - * / % **");
      expect(tokens[0].kind).toBe(TokenKind.PLUS);
      expect(tokens[1].kind).toBe(TokenKind.MINUS);
      expect(tokens[2].kind).toBe(TokenKind.ASTERISK);
      expect(tokens[3].kind).toBe(TokenKind.SLASH);
      expect(tokens[4].kind).toBe(TokenKind.MOD);
      expect(tokens[5].kind).toBe(TokenKind.POW);
    });

    it("should tokenize comparison operators", () => {
      const tokens = tokenize("== != < > <= >=");
      expect(tokens[0].kind).toBe(TokenKind.EQ);
      expect(tokens[1].kind).toBe(TokenKind.NOT_EQ);
      expect(tokens[2].kind).toBe(TokenKind.LT);
      expect(tokens[3].kind).toBe(TokenKind.GT);
      expect(tokens[4].kind).toBe(TokenKind.LT_EQUALS);
      expect(tokens[5].kind).toBe(TokenKind.GT_EQUALS);
    });

    it("should tokenize logical operators", () => {
      const tokens = tokenize("&& || !");
      expect(tokens[0].kind).toBe(TokenKind.AND);
      expect(tokens[1].kind).toBe(TokenKind.OR);
      expect(tokens[2].kind).toBe(TokenKind.BANG);
    });

    it("should tokenize bitwise operators", () => {
      const tokens = tokenize("& | ^ << >>");
      expect(tokens[0].kind).toBe(TokenKind.AMPERSAND);
      expect(tokens[1].kind).toBe(TokenKind.PIPE);
      expect(tokens[2].kind).toBe(TokenKind.CARET);
      expect(tokens[3].kind).toBe(TokenKind.LT_LT);
      expect(tokens[4].kind).toBe(TokenKind.GT_GT);
    });

    it("should tokenize assignment operators", () => {
      const tokens = tokenize("= += -= *= /=");
      expect(tokens[0].kind).toBe(TokenKind.ASSIGN);
      expect(tokens[1].kind).toBe(TokenKind.PLUS_EQUALS);
      expect(tokens[2].kind).toBe(TokenKind.MINUS_EQUALS);
      expect(tokens[3].kind).toBe(TokenKind.ASTERISK_EQUALS);
      expect(tokens[4].kind).toBe(TokenKind.SLASH_EQUALS);
    });

    it("should tokenize increment/decrement", () => {
      const tokens = tokenize("++ --");
      expect(tokens[0].kind).toBe(TokenKind.PLUS_PLUS);
      expect(tokens[1].kind).toBe(TokenKind.MINUS_MINUS);
    });

    it("should tokenize special operators", () => {
      const tokens = tokenize("=> ?? ?. ...");
      expect(tokens[0].kind).toBe(TokenKind.ARROW);
      expect(tokens[1].kind).toBe(TokenKind.NULLISH);
      expect(tokens[2].kind).toBe(TokenKind.QUESTION_DOT);
      expect(tokens[3].kind).toBe(TokenKind.SPREAD);
    });
  });

  describe("punctuation", () => {
    it("should tokenize brackets", () => {
      const tokens = tokenize("( ) [ ] { }");
      expect(tokens[0].kind).toBe(TokenKind.LPAREN);
      expect(tokens[1].kind).toBe(TokenKind.RPAREN);
      expect(tokens[2].kind).toBe(TokenKind.LBRACKET);
      expect(tokens[3].kind).toBe(TokenKind.RBRACKET);
      expect(tokens[4].kind).toBe(TokenKind.LBRACE);
      expect(tokens[5].kind).toBe(TokenKind.RBRACE);
    });

    it("should tokenize separators", () => {
      const tokens = tokenize(", ; : .");
      expect(tokens[0].kind).toBe(TokenKind.COMMA);
      expect(tokens[1].kind).toBe(TokenKind.SEMICOLON);
      expect(tokens[2].kind).toBe(TokenKind.COLON);
      expect(tokens[3].kind).toBe(TokenKind.PERIOD);
    });
  });

  describe("comments", () => {
    it("should skip single-line comments", () => {
      const tokens = tokenize("foo // this is a comment\nbar");
      expect(tokens[0].kind).toBe(TokenKind.IDENT);
      expect(tokens[0].literal).toBe("foo");
      expect(tokens[1].kind).toBe(TokenKind.NEWLINE);
      expect(tokens[2].kind).toBe(TokenKind.IDENT);
      expect(tokens[2].literal).toBe("bar");
    });

    it("should skip multi-line comments", () => {
      const tokens = tokenize("foo /* this is\na comment */ bar");
      expect(tokens[0].kind).toBe(TokenKind.IDENT);
      expect(tokens[0].literal).toBe("foo");
      expect(tokens[1].kind).toBe(TokenKind.IDENT);
      expect(tokens[1].literal).toBe("bar");
    });

    it("should skip shebang", () => {
      const tokens = tokenize("#!/usr/bin/env risor\nfoo");
      expect(tokens[0].kind).toBe(TokenKind.NEWLINE);
      expect(tokens[1].kind).toBe(TokenKind.IDENT);
      expect(tokens[1].literal).toBe("foo");
    });
  });

  describe("newlines", () => {
    it("should tokenize newlines", () => {
      const tokens = tokenize("foo\nbar");
      expect(tokens[0].kind).toBe(TokenKind.IDENT);
      expect(tokens[1].kind).toBe(TokenKind.NEWLINE);
      expect(tokens[2].kind).toBe(TokenKind.IDENT);
    });

    it("should handle CRLF", () => {
      const tokens = tokenize("foo\r\nbar");
      expect(tokens[0].kind).toBe(TokenKind.IDENT);
      expect(tokens[1].kind).toBe(TokenKind.NEWLINE);
      expect(tokens[2].kind).toBe(TokenKind.IDENT);
    });
  });

  describe("position tracking", () => {
    it("should track line and column", () => {
      const tokens = tokenize("foo\nbar");
      expect(tokens[0].start.line).toBe(0);
      expect(tokens[0].start.column).toBe(0);
      expect(tokens[2].start.line).toBe(1);
      expect(tokens[2].start.column).toBe(0);
    });
  });

  describe("state save/restore", () => {
    it("should support backtracking", () => {
      const lexer = new Lexer("foo bar baz");
      const tok1 = lexer.nextToken();
      expect(tok1.literal).toBe("foo");

      const state = lexer.saveState();
      const tok2 = lexer.nextToken();
      expect(tok2.literal).toBe("bar");

      lexer.restoreState(state);
      const tok3 = lexer.nextToken();
      expect(tok3.literal).toBe("bar");
    });
  });

  describe("complex expressions", () => {
    it("should tokenize a let statement", () => {
      const tokens = tokenize("let x = 42");
      expect(tokens.map((t) => t.kind)).toEqual([
        TokenKind.LET,
        TokenKind.IDENT,
        TokenKind.ASSIGN,
        TokenKind.INT,
        TokenKind.EOF,
      ]);
    });

    it("should tokenize a function definition", () => {
      const tokens = tokenize("function add(a, b) { return a + b }");
      expect(tokens.map((t) => t.kind)).toEqual([
        TokenKind.FUNCTION,
        TokenKind.IDENT,
        TokenKind.LPAREN,
        TokenKind.IDENT,
        TokenKind.COMMA,
        TokenKind.IDENT,
        TokenKind.RPAREN,
        TokenKind.LBRACE,
        TokenKind.RETURN,
        TokenKind.IDENT,
        TokenKind.PLUS,
        TokenKind.IDENT,
        TokenKind.RBRACE,
        TokenKind.EOF,
      ]);
    });

    it("should tokenize an arrow function", () => {
      const tokens = tokenize("x => x * 2");
      expect(tokens.map((t) => t.kind)).toEqual([
        TokenKind.IDENT,
        TokenKind.ARROW,
        TokenKind.IDENT,
        TokenKind.ASTERISK,
        TokenKind.INT,
        TokenKind.EOF,
      ]);
    });

    it("should tokenize array and object literals", () => {
      const tokens = tokenize('[1, 2, 3] {a: 1, b: "two"}');
      expect(tokens.map((t) => t.kind)).toEqual([
        TokenKind.LBRACKET,
        TokenKind.INT,
        TokenKind.COMMA,
        TokenKind.INT,
        TokenKind.COMMA,
        TokenKind.INT,
        TokenKind.RBRACKET,
        TokenKind.LBRACE,
        TokenKind.IDENT,
        TokenKind.COLON,
        TokenKind.INT,
        TokenKind.COMMA,
        TokenKind.IDENT,
        TokenKind.COLON,
        TokenKind.STRING,
        TokenKind.RBRACE,
        TokenKind.EOF,
      ]);
    });

    it("should tokenize match expression", () => {
      const tokens = tokenize('match x { 1 => "one", _ => "other" }');
      expect(tokens[0].kind).toBe(TokenKind.MATCH);
      expect(tokens[1].kind).toBe(TokenKind.IDENT);
      expect(tokens[2].kind).toBe(TokenKind.LBRACE);
    });
  });
});
