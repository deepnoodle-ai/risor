# Risor Language Grammar

**Version: 2.0**

This document provides a formal BNF/EBNF grammar specification for the Risor scripting language. The notation is compatible with parser/lexer generation tools and follows conventions similar to ISO EBNF.

## Notation

This specification uses a BNF-based notation similar to EBNF with the following conventions:

- Any sequence of characters given in single-quotes denotes a terminal sequence: `'let'`
- Special terminal sequences requiring specification are given in angle brackets: `<unicode letter>`
- Normal parentheses specify priority between operations: `(A B)`
- Sequence of rules A and B: `A B`
- Choice between rules A and B: `A | B`
- Optional use of rule A: `[A]`
- Repetition (zero or more) of rule A: `{A}`
- One or more repetitions: `A {A}` or explicitly noted
- Rule names starting with capital letters denote lexical rules
- Rule names starting with lowercase letters denote syntactic rules

---

## Lexical Grammar

### Whitespace and Comments

```ebnf
LF:
    <unicode character Line Feed U+000A>

CR:
    <unicode character Carriage Return U+000D>

ShebangLine:
    '#!' {<any character excluding CR and LF>}

SingleLineComment:
    '//' {<any character excluding CR and LF>}

MultiLineComment:
    '/*' {MultiLineComment | <any character>} '*/'

WS:
    <one of: SPACE U+0020, TAB U+0009>

NL:
    LF | (CR [LF])

Hidden:
    MultiLineComment | SingleLineComment | WS
```

### Keywords

```ebnf
CASE:       'case'
CATCH:      'catch'
CONST:      'const'
DEFAULT:    'default'
ELSE:       'else'
FALSE:      'false'
FINALLY:    'finally'
FUNCTION:   'function'
IF:         'if'
IN:         'in'
LET:        'let'
MATCH:      'match'
NIL:        'nil'
NOT:        'not'
RETURN:     'return'
STRUCT:     'struct'      (* reserved for future use *)
SWITCH:     'switch'
THROW:      'throw'
TRUE:       'true'
TRY:        'try'
```

**Note:** The following are contextual keywords that may be used as identifiers in most contexts:
- `when`, `else`, `all`, `any`, `require`, `rule` (proposed for v2)

### Operators and Punctuation

```ebnf
(* Arithmetic *)
PLUS:           '+'
MINUS:          '-'
ASTERISK:       '*'
SLASH:          '/'
MOD:            '%'
POW:            '**'

(* Bitwise *)
AMPERSAND:      '&'
BITOR:          '|'
CARET:          '^'
LT_LT:          '<<'
GT_GT:          '>>'

(* Logical *)
AND:            '&&'
OR:             '||'
BANG:           '!'

(* Comparison *)
EQ:             '=='
NOT_EQ:         '!='
LT:             '<'
GT:             '>'
LT_EQUALS:      '<='
GT_EQUALS:      '>='

(* Assignment *)
ASSIGN:         '='
PLUS_EQUALS:    '+='
MINUS_EQUALS:   '-='
ASTERISK_EQUALS: '*='
SLASH_EQUALS:   '/='

(* Increment/Decrement *)
PLUS_PLUS:      '++'
MINUS_MINUS:    '--'

(* Other Operators *)
ARROW:          '=>'
PIPE:           '|>'
SPREAD:         '...'
NULLISH:        '??'
QUESTION_DOT:   '?.'

(* Delimiters *)
LPAREN:         '('
RPAREN:         ')'
LBRACE:         '{'
RBRACE:         '}'
LBRACKET:       '['
RBRACKET:       ']'
COMMA:          ','
COLON:          ':'
SEMICOLON:      ';'
PERIOD:         '.'
QUESTION:       '?'
BACKTICK:       '`'
```

### Literals

#### Numbers

```ebnf
DecDigit:
    '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'

DecDigitNoZero:
    '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'

HexDigit:
    DecDigit | 'A' | 'B' | 'C' | 'D' | 'E' | 'F'
             | 'a' | 'b' | 'c' | 'd' | 'e' | 'f'

OctalDigit:
    '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7'

BinaryDigit:
    '0' | '1'

DecimalLiteral:
    DecDigit {DecDigit}

HexLiteral:
    '0' ('x' | 'X') HexDigit {HexDigit}

OctalLiteral:
    '0' OctalDigit {OctalDigit}

BinaryLiteral:
    '0' ('b' | 'B') BinaryDigit {BinaryDigit}

IntegerLiteral:
    DecimalLiteral | HexLiteral | OctalLiteral | BinaryLiteral

FloatLiteral:
    DecimalLiteral '.' DecimalLiteral

NumberLiteral:
    IntegerLiteral | FloatLiteral
```

#### Booleans and Nil

```ebnf
BooleanLiteral:
    'true' | 'false'

NilLiteral:
    'nil'
```

#### Strings

```ebnf
EscapeSequence:
    '\\' ('a' | 'b' | 'f' | 'n' | 'r' | 't' | 'v' | '\\' | 'e' | '\'' | '"')
    | '\\x' HexDigit HexDigit
    | '\\u' HexDigit HexDigit HexDigit HexDigit
    | '\\U' HexDigit HexDigit HexDigit HexDigit HexDigit HexDigit HexDigit HexDigit
    | '\\' OctalDigit OctalDigit OctalDigit

SingleQuotedString:
    '\'' {EscapeSequence | <any character except '\'' or '\\' or CR or LF>} '\''

DoubleQuotedString:
    '"' {EscapeSequence | <any character except '"' or '\\' or CR or LF>} '"'

StringLiteral:
    SingleQuotedString | DoubleQuotedString

TemplateString:
    '`' {TemplateChar | TemplateExpr} '`'

TemplateChar:
    <any character except '`' or '${'> | '$' <not followed by '{'>

TemplateExpr:
    '${' expression '}'
```

### Identifiers

```ebnf
Letter:
    <unicode letter (categories Lu, Ll, Lt, Lm, Lo)>

UnicodeDigit:
    <unicode digit (category Nd)>

IdentifierStart:
    Letter | '_'

IdentifierPart:
    Letter | UnicodeDigit | '_'

Identifier:
    IdentifierStart {IdentifierPart}
```

**Note:** Identifiers may contain Unicode letters but not non-ASCII characters after the identifier body.

---

## Syntax Grammar

### Program Structure

```ebnf
program:
    [ShebangLine NL]
    {statement}
    EOF

statement:
    varStatement
    | constStatement
    | returnStatement
    | functionDeclaration
    | blockStatement
    | tryStatement
    | throwStatement
    | assignmentStatement
    | postfixStatement
    | setAttrStatement
    | expressionStatement
```

### Statements

#### Variable Declarations

```ebnf
varStatement:
    'let' (simpleVar | multiVar | objectDestructure | arrayDestructure)

simpleVar:
    Identifier '=' expression

multiVar:
    Identifier {',' Identifier} '=' expression

objectDestructure:
    '{' [destructureBinding {',' destructureBinding}] '}' '=' expression

destructureBinding:
    Identifier [':' Identifier] ['=' expression]

arrayDestructure:
    '[' [arrayDestructureElement {',' arrayDestructureElement}] ']' '=' expression

arrayDestructureElement:
    Identifier ['=' expression]
```

#### Constant Declarations

```ebnf
constStatement:
    'const' Identifier '=' expression
```

#### Return Statement

```ebnf
returnStatement:
    'return' [expression]
```

#### Function Declaration

```ebnf
functionDeclaration:
    'function' Identifier '(' [parameterList] ')' block

parameterList:
    parameter {',' parameter}

parameter:
    simpleParameter
    | destructureParameter
    | restParameter

simpleParameter:
    Identifier ['=' expression]

destructureParameter:
    objectDestructureParam
    | arrayDestructureParam

objectDestructureParam:
    '{' [destructureBinding {',' destructureBinding}] '}'

arrayDestructureParam:
    '[' [arrayDestructureElement {',' arrayDestructureElement}] ']'

restParameter:
    '...' Identifier
```

#### Block Statement

```ebnf
blockStatement:
    block

block:
    '{' {statement} '}'
```

#### Try/Catch/Finally

```ebnf
tryStatement:
    'try' block [catchClause] [finallyClause]

catchClause:
    'catch' [Identifier] block

finallyClause:
    'finally' block
```

**Note:** At least one of `catchClause` or `finallyClause` must be present.

#### Throw Statement

```ebnf
throwStatement:
    'throw' [expression]
```

#### Assignment Statements

```ebnf
assignmentStatement:
    (Identifier | indexExpr | getAttrExpr) assignmentOp expression

assignmentOp:
    '=' | '+=' | '-=' | '*=' | '/='

postfixStatement:
    (Identifier | indexExpr | getAttrExpr) postfixOp

postfixOp:
    '++' | '--'

setAttrStatement:
    getAttrExpr assignmentOp expression
```

#### Expression Statement

```ebnf
expressionStatement:
    expression
```

### Expressions

#### Expression Precedence (lowest to highest)

| Precedence | Operators | Associativity |
|------------|-----------|---------------|
| 1 (lowest) | `\|\|` | left |
| 2 | `&&` | left |
| 3 | `??` | left |
| 4 | `\|>` (pipe) | left |
| 5 | `==`, `!=` | left |
| 6 | `<`, `<=`, `>`, `>=`, `in`, `not in` | left |
| 7 | `\|`, `^` (bitwise) | left |
| 8 | `&` (bitwise) | left |
| 9 | `<<`, `>>` | left |
| 10 | `+`, `-` | left |
| 11 | `*`, `/`, `%` | left |
| 12 | `**` | right |
| 13 | prefix `-`, `!`, `not` | right (unary) |
| 14 (highest) | postfix `++`, `--`, call, index, member | left |

#### Expression Grammar

```ebnf
expression:
    orExpr

orExpr:
    andExpr {'||' andExpr}

andExpr:
    nullishExpr {'&&' nullishExpr}

nullishExpr:
    pipeExpr {'??' pipeExpr}

pipeExpr:
    equalityExpr {'|>' equalityExpr}

equalityExpr:
    comparisonExpr {('==' | '!=') comparisonExpr}

comparisonExpr:
    bitwiseOrExpr {('<' | '<=' | '>' | '>=' | 'in' | 'not' 'in') bitwiseOrExpr}

bitwiseOrExpr:
    bitwiseXorExpr {'|' bitwiseXorExpr}

bitwiseXorExpr:
    bitwiseAndExpr {'^' bitwiseAndExpr}

bitwiseAndExpr:
    shiftExpr {'&' shiftExpr}

shiftExpr:
    additiveExpr {('<<' | '>>') additiveExpr}

additiveExpr:
    multiplicativeExpr {('+' | '-') multiplicativeExpr}

multiplicativeExpr:
    powerExpr {('*' | '/' | '%') powerExpr}

powerExpr:
    prefixExpr ['**' powerExpr]    (* right associative *)

prefixExpr:
    {prefixOp} postfixExpr

prefixOp:
    '-' | '!' | 'not'

postfixExpr:
    primaryExpr {postfixSuffix}

postfixSuffix:
    callSuffix
    | indexSuffix
    | sliceSuffix
    | memberSuffix
    | optionalChainSuffix
    | postfixIncDec

callSuffix:
    '(' [argumentList] ')'

argumentList:
    argument {',' argument}

argument:
    expression
    | spreadExpr

spreadExpr:
    '...' expression

indexSuffix:
    '[' expression ']'

sliceSuffix:
    '[' [expression] ':' [expression] ']'

memberSuffix:
    '.' Identifier
    | '.' callSuffix

optionalChainSuffix:
    '?.' Identifier
    | '?.' callSuffix

postfixIncDec:
    '++' | '--'
```

#### Primary Expressions

```ebnf
primaryExpr:
    Identifier
    | literal
    | groupedExpr
    | listLiteral
    | mapLiteral
    | functionLiteral
    | arrowFunction
    | ifExpr
    | switchExpr
    | matchExpr
    | tryExpr

literal:
    NumberLiteral
    | StringLiteral
    | TemplateString
    | BooleanLiteral
    | NilLiteral

groupedExpr:
    '(' expression ')'
```

#### Collection Literals

```ebnf
listLiteral:
    '[' [listItems] ']'

listItems:
    listItem {',' listItem} [',']

listItem:
    expression
    | spreadExpr

mapLiteral:
    '{' [mapItems] '}'

mapItems:
    mapItem {',' mapItem} [',']

mapItem:
    mapKeyValue
    | mapShorthand
    | mapSpread
    | mapDefaultValue

mapKeyValue:
    expression ':' expression

mapShorthand:
    Identifier

mapSpread:
    '...' expression

mapDefaultValue:
    Identifier '=' expression
```

#### Function Literals

```ebnf
functionLiteral:
    'function' [Identifier] '(' [parameterList] ')' block
```

#### Arrow Functions

```ebnf
arrowFunction:
    arrowParams '=>' arrowBody

arrowParams:
    Identifier
    | '(' [parameterList] ')'
    | objectDestructureParam
    | arrayDestructureParam

arrowBody:
    expression
    | block
```

#### If Expression

```ebnf
ifExpr:
    'if' '(' expression ')' block ['else' block]
```

#### Switch Expression

```ebnf
switchExpr:
    'switch' '(' expression ')' '{' {caseClause} '}'

caseClause:
    'case' caseExprs ':' {statement}
    | 'default' ':' {statement}

caseExprs:
    expression {',' expression}
```

#### Match Expression

```ebnf
matchExpr:
    'match' expression '{' {matchArm} defaultArm '}'

matchArm:
    pattern [guard] '=>' expression

defaultArm:
    '_' '=>' expression

guard:
    'if' expression

pattern:
    literalPattern
    | wildcardPattern
```

**Note:** Object patterns and list patterns are planned for future versions.

```ebnf
literalPattern:
    expression

wildcardPattern:
    '_'
```

#### Try Expression

```ebnf
tryExpr:
    'try' expression
```

**Note:** `try` as an expression evaluates the expression and returns the result or throws an error.

### In and Not In Expressions

```ebnf
inExpr:
    expression 'in' expression

notInExpr:
    expression 'not' 'in' expression
```

### Pipe Expression

```ebnf
pipeExpr:
    expression '|>' expression {'|>' expression}
```

**Semantics:** In `a |> f(x)`, the pipe operator transforms this to `f(a, x)`, inserting the left operand as the first argument to the function call on the right.

---

## Proposed v2 Extensions

The following grammar extensions are proposed for Risor v2. They are documented here for completeness but may not yet be implemented.

### When Expression

```ebnf
whenExpr:
    'when' '{' {whenArm} elseArm '}'

whenArm:
    expression '=>' expression

elseArm:
    'else' '=>' expression
```

### All/Any Blocks

```ebnf
allExpr:
    'all' '{' {expression} '}'

anyExpr:
    'any' '{' {expression} '}'
```

### Require Statement

```ebnf
requireStatement:
    'require' expression ['else' expression]
    | 'require' pattern 'from' expression ['else' expression]
    | 'require' pattern 'from' expression 'if' expression ['else' expression]
```

### Rule Declaration

```ebnf
ruleDeclaration:
    'rule' Identifier '(' [parameterList] ')' '{' ruleBody '}'

ruleBody:
    {requireStatement | varStatement | expression}
```

### Extended Patterns (for match and require)

```ebnf
pattern:
    literalPattern
    | identifierPattern
    | wildcardPattern
    | objectPattern
    | listPattern

identifierPattern:
    Identifier

objectPattern:
    '{' [keyPattern {',' keyPattern}] '}'

keyPattern:
    Identifier [':' pattern]

listPattern:
    '[' [pattern {',' pattern}] [spreadPattern] ']'

spreadPattern:
    '...' [Identifier]
```

---

## Lexical Conventions

### Automatic Semicolon Insertion

Risor uses newlines as statement terminators. The following rules govern how newlines are treated:

1. **Statement-terminating newlines:** A newline terminates a statement when it follows:
   - An identifier
   - A literal (number, string, boolean, nil)
   - A closing delimiter: `)`, `]`, `}`
   - A postfix operator: `++`, `--`

2. **Continuation newlines:** A newline does NOT terminate a statement when:
   - It follows a binary operator: `+`, `-`, `*`, `/`, etc.
   - It follows an opening delimiter: `(`, `[`, `{`
   - It follows a comma: `,`
   - It is inside parentheses, brackets, or braces after a comma

3. **Explicit semicolons:** Semicolons `;` may be used as explicit statement terminators.

### Comments

- Single-line comments: `// comment until end of line`
- Multi-line comments: `/* comment */` (may be nested)
- Shebang: `#!/usr/bin/env risor` (only at file start, line 1)

### String Escape Sequences

| Sequence | Meaning |
|----------|---------|
| `\a` | Alert (bell) |
| `\b` | Backspace |
| `\f` | Form feed |
| `\n` | Newline |
| `\r` | Carriage return |
| `\t` | Horizontal tab |
| `\v` | Vertical tab |
| `\\` | Backslash |
| `\e` | Escape (0x1B) |
| `\'` | Single quote |
| `\"` | Double quote |
| `\xHH` | Byte with hex value HH |
| `\uHHHH` | Unicode code point HHHH |
| `\UHHHHHHHH` | Unicode code point HHHHHHHH |
| `\OOO` | Byte with octal value OOO (000-377) |

### Template Strings

Template strings use backticks and support embedded expressions:

```javascript
`Hello, ${name}!`
`1 + 1 = ${1 + 1}`
```

The `${...}` syntax embeds an expression whose result is converted to a string.

---

## Tokens Summary

```ebnf
Token:
    (* Whitespace and Comments *)
    ShebangLine | SingleLineComment | MultiLineComment | WS | NL

    (* Keywords *)
    | 'case' | 'catch' | 'const' | 'default' | 'else' | 'false'
    | 'finally' | 'function' | 'if' | 'in' | 'let' | 'match'
    | 'nil' | 'not' | 'return' | 'struct' | 'switch' | 'throw'
    | 'true' | 'try'

    (* Operators *)
    | '+' | '-' | '*' | '/' | '%' | '**'
    | '&' | '|' | '^' | '<<' | '>>'
    | '&&' | '||' | '!'
    | '==' | '!=' | '<' | '>' | '<=' | '>='
    | '=' | '+=' | '-=' | '*=' | '/='
    | '++' | '--'
    | '=>' | '|>' | '...' | '??' | '?.'

    (* Delimiters *)
    | '(' | ')' | '{' | '}' | '[' | ']'
    | ',' | ':' | ';' | '.' | '?'

    (* Literals *)
    | IntegerLiteral | FloatLiteral
    | StringLiteral | TemplateString
    | BooleanLiteral | NilLiteral

    (* Identifiers *)
    | Identifier

    (* End of File *)
    | EOF
```

---

## Examples

### Variable Declarations

```javascript
let x = 42
let name = "Alice"
const PI = 3.14159

// Multiple assignment
let a, b = [1, 2]

// Object destructuring
let { name, age } = person
let { name: n, age: a = 0 } = person

// Array destructuring
let [first, second] = items
let [head, ...tail] = list
```

### Functions

```javascript
// Named function
function add(a, b) {
    return a + b
}

// With default parameters
function greet(name = "World") {
    return "Hello, " + name
}

// With rest parameter
function sum(...numbers) {
    return numbers.reduce((a, b) => a + b, 0)
}

// Arrow functions
let double = x => x * 2
let add = (a, b) => a + b
let process = x => {
    let y = x * 2
    return y + 1
}

// Destructuring in parameters
function point({x, y}) {
    return x + y
}

let coords = ([x, y]) => x * y
```

### Control Flow

```javascript
// If expression
let result = if (x > 0) { "positive" } else { "non-positive" }

// Switch expression
let day = switch (n) {
    case 0: "Sunday"
    case 1: "Monday"
    case 2, 3, 4: "Midweek"
    default: "Weekend"
}

// Match expression
let describe = match value {
    0 => "zero"
    1 => "one"
    _ => "other"
}
```

### Collections

```javascript
// Lists
let numbers = [1, 2, 3]
let mixed = [1, "two", true]
let spread = [0, ...numbers, 4]

// Maps
let person = {name: "Alice", age: 30}
let shorthand = {name, age}
let merged = {...defaults, ...overrides}
```

### Operators

```javascript
// Arithmetic
let sum = 1 + 2
let power = 2 ** 10

// Comparison
let eq = a == b
let contains = item in list
let missing = item not in list

// Logical
let both = a && b
let either = a || b

// Nullish coalescing
let value = x ?? default

// Optional chaining
let name = user?.profile?.name

// Pipe operator
let result = data |> filter(x => x > 0) |> map(x => x * 2)
```

### Error Handling

```javascript
// Try/catch/finally
try {
    riskyOperation()
} catch err {
    handleError(err)
} finally {
    cleanup()
}

// Throw
throw "error message"
throw error("something went wrong", {code: 500})

// Try expression
let result = try parseJSON(data)
```

---

## Conformance Notes

This grammar is designed to be compatible with common parser generator tools. When implementing:

1. **Lexer mode for template strings:** Template strings require a special lexer mode to handle `${...}` interpolation.

2. **Newline handling:** The lexer must track newlines to support automatic semicolon insertion.

3. **Operator precedence:** Binary expression parsing should use precedence climbing or Pratt parsing.

4. **Lookahead for arrows:** Distinguishing `(x) => ...` from `(x)` requires lookahead for `=>`.

5. **Contextual keywords:** v2 keywords like `when`, `all`, `any` may need contextual handling if backward compatibility is required.
