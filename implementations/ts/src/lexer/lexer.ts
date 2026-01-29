import {
  Token,
  TokenKind,
  Position,
  newPosition,
  newToken,
  lookupIdentifier,
  advancePosition,
} from "../token/token.js";

/**
 * Lexer error with position information.
 */
export class LexerError extends Error {
  constructor(
    message: string,
    public readonly position: Position
  ) {
    super(`${message} at line ${position.line + 1}, column ${position.column + 1}`);
    this.name = "LexerError";
  }
}

/**
 * Saved lexer state for backtracking.
 */
interface LexerState {
  position: number;
  nextPosition: number;
  ch: string;
  line: number;
  column: number;
  lineStart: number;
  prevToken: Token | null;
}

/**
 * Lexer tokenizes Risor source code.
 */
export class Lexer {
  private input: string;
  private characters: string[];
  private position: number = -1;
  private nextPosition: number = 0;
  private ch: string = "";
  private line: number = 0;
  private column: number = -1;
  private lineStart: number = 0;
  private file: string;
  private tokenStartPosition: Position;
  private prevToken: Token | null = null;

  constructor(input: string, file: string = "<stdin>") {
    this.input = input;
    this.characters = [...input]; // Handle Unicode properly
    this.file = file;
    this.tokenStartPosition = this.currentPosition();
    this.readChar();
  }

  /**
   * Save the current lexer state for backtracking.
   */
  saveState(): LexerState {
    return {
      position: this.position,
      nextPosition: this.nextPosition,
      ch: this.ch,
      line: this.line,
      column: this.column,
      lineStart: this.lineStart,
      prevToken: this.prevToken,
    };
  }

  /**
   * Restore a previously saved lexer state.
   */
  restoreState(state: LexerState): void {
    this.position = state.position;
    this.nextPosition = state.nextPosition;
    this.ch = state.ch;
    this.line = state.line;
    this.column = state.column;
    this.lineStart = state.lineStart;
    this.prevToken = state.prevToken;
  }

  /**
   * Get the current position.
   */
  private currentPosition(): Position {
    return newPosition(this.position, this.lineStart, this.line, this.column, this.file);
  }

  /**
   * Read the next character.
   */
  private readChar(): void {
    if (this.nextPosition >= this.characters.length) {
      this.ch = "\0";
    } else {
      this.ch = this.characters[this.nextPosition];
    }
    this.position = this.nextPosition;
    this.nextPosition++;
    this.column++;
  }

  /**
   * Peek at the next character without consuming it.
   */
  private peekChar(): string {
    if (this.nextPosition >= this.characters.length) {
      return "\0";
    }
    return this.characters[this.nextPosition];
  }

  /**
   * Peek n characters ahead without consuming.
   */
  private peekCharN(n: number): string {
    const idx = this.position + n;
    if (idx >= this.characters.length) {
      return "\0";
    }
    return this.characters[idx];
  }

  /**
   * Skip whitespace (spaces and tabs, not newlines).
   */
  private skipWhitespace(): void {
    while (this.ch === " " || this.ch === "\t") {
      this.readChar();
    }
  }

  /**
   * Skip to end of line.
   */
  private skipToEndOfLine(): void {
    while (this.ch !== "\n" && this.ch !== "\0") {
      this.readChar();
    }
  }

  /**
   * Handle newline and update line tracking.
   */
  private handleNewline(): void {
    this.line++;
    this.column = -1;
    this.lineStart = this.nextPosition;
  }

  /**
   * Start tracking a new token.
   */
  private startToken(): void {
    this.tokenStartPosition = this.currentPosition();
  }

  /**
   * Create a token with the current position as the end.
   */
  private makeToken(kind: TokenKind, literal: string): Token {
    const tok = newToken(
      kind,
      literal,
      this.tokenStartPosition,
      advancePosition(this.currentPosition(), 1)
    );
    this.prevToken = tok;
    return tok;
  }

  /**
   * Get the next token.
   */
  nextToken(): Token {
    this.skipWhitespace();

    // Handle shebang at start of file
    if (this.line === 0 && this.position <= 1 && this.ch === "#" && this.peekChar() === "!") {
      this.skipToEndOfLine();
      this.skipWhitespace();
    }

    // Handle comments
    while (this.ch === "/") {
      if (this.peekChar() === "/") {
        // Single-line comment
        this.skipToEndOfLine();
        this.skipWhitespace();
      } else if (this.peekChar() === "*") {
        // Multi-line comment
        this.readChar(); // consume /
        this.readChar(); // consume *
        while (!((this.ch as string) === "*" && this.peekChar() === "/") && (this.ch as string) !== "\0") {
          if ((this.ch as string) === "\n") {
            this.handleNewline();
          }
          this.readChar();
        }
        if ((this.ch as string) !== "\0") {
          this.readChar(); // consume *
          this.readChar(); // consume /
        }
        this.skipWhitespace();
      } else {
        break;
      }
    }

    this.startToken();

    // EOF
    if (this.ch === "\0") {
      return this.makeToken(TokenKind.EOF, "");
    }

    // Newline
    if (this.ch === "\n") {
      this.handleNewline();
      this.readChar();
      return this.makeToken(TokenKind.NEWLINE, "\n");
    }

    // Carriage return (handle \r\n as single newline)
    if (this.ch === "\r") {
      this.readChar();
      if ((this.ch as string) === "\n") {
        this.handleNewline();
        this.readChar();
      }
      return this.makeToken(TokenKind.NEWLINE, "\n");
    }

    // String literals
    if (this.ch === '"' || this.ch === "'") {
      return this.readString(this.ch);
    }

    // Template literals
    if (this.ch === "`") {
      return this.readBacktick();
    }

    // Numbers
    if (isDigit(this.ch)) {
      return this.readNumber();
    }

    // Identifiers and keywords
    if (isLetter(this.ch)) {
      return this.readIdentifier();
    }

    // Operators and punctuation
    const tok = this.readOperator();
    if (tok) {
      return tok;
    }

    // Unknown character
    const ch = this.ch;
    this.readChar();
    return this.makeToken(TokenKind.ILLEGAL, ch);
  }

  /**
   * Read an identifier or keyword.
   */
  private readIdentifier(): Token {
    const start = this.position;
    while (isLetter(this.ch) || isDigit(this.ch)) {
      this.readChar();
    }
    const literal = this.characters.slice(start, this.position).join("");
    const kind = lookupIdentifier(literal);
    return this.makeToken(kind, literal);
  }

  /**
   * Read a number literal (int, float, hex, binary, octal).
   */
  private readNumber(): Token {
    const start = this.position;

    // Check for hex, binary, or octal
    if (this.ch === "0") {
      const next = this.peekChar().toLowerCase();
      if (next === "x") {
        // Hexadecimal
        this.readChar(); // consume 0
        this.readChar(); // consume x
        while (isHexDigit(this.ch)) {
          this.readChar();
        }
        const literal = this.characters.slice(start, this.position).join("");
        this.checkTrailingAlphanumeric(literal);
        return this.makeToken(TokenKind.INT, literal);
      } else if (next === "b") {
        // Binary
        this.readChar(); // consume 0
        this.readChar(); // consume b
        while (this.ch === "0" || this.ch === "1") {
          this.readChar();
        }
        const literal = this.characters.slice(start, this.position).join("");
        this.checkTrailingAlphanumeric(literal);
        return this.makeToken(TokenKind.INT, literal);
      } else if (isDigit(next) && next !== "." && next !== "e" && next !== "E") {
        // Octal
        this.readChar(); // consume leading 0
        while (isOctalDigit(this.ch)) {
          this.readChar();
        }
        const literal = this.characters.slice(start, this.position).join("");
        this.checkTrailingAlphanumeric(literal);
        return this.makeToken(TokenKind.INT, literal);
      }
    }

    // Decimal number (int or float)
    while (isDigit(this.ch)) {
      this.readChar();
    }

    let isFloat = false;

    // Check for decimal point
    if (this.ch === "." && isDigit(this.peekChar())) {
      isFloat = true;
      this.readChar(); // consume .
      while (isDigit(this.ch)) {
        this.readChar();
      }
    }

    // Check for exponent
    if (this.ch === "e" || this.ch === "E") {
      isFloat = true;
      this.readChar(); // consume e/E
      if ((this.ch as string) === "+" || (this.ch as string) === "-") {
        this.readChar();
      }
      while (isDigit(this.ch)) {
        this.readChar();
      }
    }

    const literal = this.characters.slice(start, this.position).join("");
    this.checkTrailingAlphanumeric(literal);
    return this.makeToken(isFloat ? TokenKind.FLOAT : TokenKind.INT, literal);
  }

  /**
   * Check for invalid trailing alphanumeric after number.
   */
  private checkTrailingAlphanumeric(literal: string): void {
    if (isLetter(this.ch)) {
      throw new LexerError(
        `Invalid number literal: ${literal}${this.ch}`,
        this.currentPosition()
      );
    }
  }

  /**
   * Read a quoted string literal.
   */
  private readString(quote: string): Token {
    const chars: string[] = [];
    this.readChar(); // consume opening quote

    while (this.ch !== quote && this.ch !== "\0" && this.ch !== "\n") {
      if (this.ch === "\\") {
        this.readChar();
        const escaped = this.readEscapeSequence();
        chars.push(escaped);
      } else {
        chars.push(this.ch);
        this.readChar();
      }
    }

    if (this.ch !== quote) {
      throw new LexerError("Unterminated string literal", this.currentPosition());
    }

    this.readChar(); // consume closing quote
    return this.makeToken(TokenKind.STRING, chars.join(""));
  }

  /**
   * Read an escape sequence.
   */
  private readEscapeSequence(): string {
    const ch = this.ch;
    this.readChar();

    switch (ch) {
      case "n":
        return "\n";
      case "r":
        return "\r";
      case "t":
        return "\t";
      case "\\":
        return "\\";
      case '"':
        return '"';
      case "'":
        return "'";
      case "a":
        return "\x07"; // bell
      case "b":
        return "\b";
      case "f":
        return "\f";
      case "v":
        return "\v";
      case "e":
        return "\x1b"; // escape
      case "0":
      case "1":
      case "2":
      case "3":
        // Octal escape \0nn
        return this.readOctalEscape(ch);
      case "x":
        // Hex escape \xHH
        return this.readHexEscape(2);
      case "u":
        // Unicode escape \uHHHH
        return this.readHexEscape(4);
      case "U":
        // Unicode escape \UHHHHHHHH
        return this.readHexEscape(8);
      default:
        throw new LexerError(`Invalid escape sequence: \\${ch}`, this.currentPosition());
    }
  }

  /**
   * Read an octal escape sequence.
   */
  private readOctalEscape(firstDigit: string): string {
    let value = parseInt(firstDigit, 8);
    for (let i = 0; i < 2; i++) {
      if (!isOctalDigit(this.ch)) {
        break;
      }
      value = value * 8 + parseInt(this.ch, 8);
      this.readChar();
    }
    return String.fromCharCode(value);
  }

  /**
   * Read a hex escape sequence with n digits.
   */
  private readHexEscape(n: number): string {
    let hex = "";
    for (let i = 0; i < n; i++) {
      if (!isHexDigit(this.ch)) {
        throw new LexerError(
          `Invalid hex escape sequence (expected ${n} digits)`,
          this.currentPosition()
        );
      }
      hex += this.ch;
      this.readChar();
    }
    return String.fromCodePoint(parseInt(hex, 16));
  }

  /**
   * Read a backtick template literal (raw string).
   */
  private readBacktick(): Token {
    const chars: string[] = [];
    this.readChar(); // consume opening backtick

    while (this.ch !== "`" && this.ch !== "\0") {
      if (this.ch === "\n") {
        this.handleNewline();
      }
      chars.push(this.ch);
      this.readChar();
    }

    if (this.ch !== "`") {
      throw new LexerError("Unterminated template literal", this.currentPosition());
    }

    this.readChar(); // consume closing backtick
    return this.makeToken(TokenKind.TEMPLATE, chars.join(""));
  }

  /**
   * Read an operator or punctuation token.
   */
  private readOperator(): Token | null {
    const ch = this.ch;
    const next = this.peekChar();

    // Three-character operators
    if (ch === "." && next === "." && this.peekCharN(2) === ".") {
      this.readChar();
      this.readChar();
      this.readChar();
      return this.makeToken(TokenKind.SPREAD, "...");
    }

    // Two-character operators
    switch (ch) {
      case "=":
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.EQ, "==");
        }
        if (next === ">") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.ARROW, "=>");
        }
        break;
      case "!":
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.NOT_EQ, "!=");
        }
        break;
      case "<":
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.LT_EQUALS, "<=");
        }
        if (next === "<") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.LT_LT, "<<");
        }
        break;
      case ">":
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.GT_EQUALS, ">=");
        }
        if (next === ">") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.GT_GT, ">>");
        }
        break;
      case "&":
        if (next === "&") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.AND, "&&");
        }
        break;
      case "|":
        if (next === "|") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.OR, "||");
        }
        break;
      case "+":
        if (next === "+") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.PLUS_PLUS, "++");
        }
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.PLUS_EQUALS, "+=");
        }
        break;
      case "-":
        if (next === "-") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.MINUS_MINUS, "--");
        }
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.MINUS_EQUALS, "-=");
        }
        break;
      case "*":
        if (next === "*") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.POW, "**");
        }
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.ASTERISK_EQUALS, "*=");
        }
        break;
      case "/":
        if (next === "=") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.SLASH_EQUALS, "/=");
        }
        break;
      case "?":
        if (next === "?") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.NULLISH, "??");
        }
        if (next === ".") {
          this.readChar();
          this.readChar();
          return this.makeToken(TokenKind.QUESTION_DOT, "?.");
        }
        break;
    }

    // Single-character operators
    const singleCharTokens: Map<string, TokenKind> = new Map([
      ["+", TokenKind.PLUS],
      ["-", TokenKind.MINUS],
      ["*", TokenKind.ASTERISK],
      ["/", TokenKind.SLASH],
      ["%", TokenKind.MOD],
      ["=", TokenKind.ASSIGN],
      ["!", TokenKind.BANG],
      ["<", TokenKind.LT],
      [">", TokenKind.GT],
      ["&", TokenKind.AMPERSAND],
      ["|", TokenKind.PIPE],
      ["^", TokenKind.CARET],
      ["(", TokenKind.LPAREN],
      [")", TokenKind.RPAREN],
      ["[", TokenKind.LBRACKET],
      ["]", TokenKind.RBRACKET],
      ["{", TokenKind.LBRACE],
      ["}", TokenKind.RBRACE],
      [",", TokenKind.COMMA],
      [";", TokenKind.SEMICOLON],
      [":", TokenKind.COLON],
      [".", TokenKind.PERIOD],
      ["?", TokenKind.QUESTION],
    ]);

    const kind = singleCharTokens.get(ch);
    if (kind !== undefined) {
      this.readChar();
      return this.makeToken(kind, ch);
    }

    return null;
  }

  /**
   * Get the line text for a given position.
   */
  getLineText(pos: Position): string {
    let end = pos.lineStart;
    while (end < this.input.length && this.input[end] !== "\n") {
      end++;
    }
    return this.input.slice(pos.lineStart, end);
  }
}

/**
 * Check if a character is a letter (for identifiers).
 */
function isLetter(ch: string): boolean {
  return (
    (ch >= "a" && ch <= "z") ||
    (ch >= "A" && ch <= "Z") ||
    ch === "_"
  );
}

/**
 * Check if a character is a digit.
 */
function isDigit(ch: string): boolean {
  return ch >= "0" && ch <= "9";
}

/**
 * Check if a character is a hex digit.
 */
function isHexDigit(ch: string): boolean {
  return (
    (ch >= "0" && ch <= "9") ||
    (ch >= "a" && ch <= "f") ||
    (ch >= "A" && ch <= "F")
  );
}

/**
 * Check if a character is an octal digit.
 */
function isOctalDigit(ch: string): boolean {
  return ch >= "0" && ch <= "7";
}

/**
 * Tokenize an input string into an array of tokens.
 */
export function tokenize(input: string, file?: string): Token[] {
  const lexer = new Lexer(input, file);
  const tokens: Token[] = [];
  let tok: Token;
  do {
    tok = lexer.nextToken();
    tokens.push(tok);
  } while (tok.kind !== TokenKind.EOF);
  return tokens;
}
