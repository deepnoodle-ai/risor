// Package parser is used to generate the abstract syntax tree (AST) for a program.
//
// A parser is created by calling New() with a lexer as input. The parser should
// then be used only once, by calling parser.Parse() to produce the AST.
package parser

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/lexer"
	"github.com/risor-io/risor/internal/tmpl"
	"github.com/risor-io/risor/internal/token"
)

type (
	prefixParseFn func() ast.Node
	infixParseFn  func(ast.Node) ast.Node
)

// statementTerminators defines tokens that can end a statement.
//
// NEWLINE HANDLING POLICY:
//  1. Trailing operators continue expressions: "x +\ny" parses as one expression
//  2. Newlines at start of line terminate expressions: "x\ny" parses as two statements
//  3. Inside parentheses: leading/trailing newlines allowed: "(\nx + y\n)"
//  4. Inside brackets/braces: newlines after commas allowed: "[1,\n2]"
//  5. Ternary expressions: newlines allowed around ? and : operators
//  6. Postfix operators (++, --) must be on same line as operand
//
// This policy follows "trailing operator continues" semantics common in many
// languages, avoiding ambiguity about whether "x\n+ y" means one expression
// or two statements (it's the latter, and produces an error since +y is invalid).
var statementTerminators = map[token.Type]bool{
	token.SEMICOLON: true,
	token.NEWLINE:   true,
	token.RBRACE:    true,
	token.EOF:       true,
}

// Parse the provided input as Risor source code and return the AST. This is
// shorthand way to create a Lexer and Parser and then call Parse on that.
func Parse(ctx context.Context, input string, options ...Option) (*ast.Program, error) {
	// Extract filename from options before creating the parser, so that lexer
	// errors in the first tokens have proper location context.
	var filename string
	for _, opt := range options {
		var probe Parser
		opt(&probe)
		if probe.filename != "" {
			filename = probe.filename
			break
		}
	}

	l := lexer.New(input)
	if filename != "" {
		l.SetFilename(filename)
	}

	p := New(l, options...)
	return p.Parse(ctx)
}

// Option is a configuration function for a Lexer.
type Option func(*Parser)

// WithFile sets the file name for the Lexer.
//
// Deprecated: Use WithFilename instead.
func WithFile(file string) Option {
	return func(l *Parser) {
		l.filename = file
	}
}

// WithFilename sets the file name for the Lexer.
func WithFilename(filename string) Option {
	return func(l *Parser) {
		l.filename = filename
	}
}

// WithMaxDepth sets the maximum nesting depth for the parser.
// This prevents stack overflow on deeply nested input.
// The default is 500.
func WithMaxDepth(depth int) Option {
	return func(p *Parser) {
		p.maxDepth = depth
	}
}

// DefaultMaxDepth is the default maximum nesting depth for parsing.
const DefaultMaxDepth = 500

// Parser object
type Parser struct {
	// the Context supplied in the Parse() call
	ctx context.Context

	// l is our lexer
	l *lexer.Lexer

	// prevToken holds the previous token, which we already processed.
	prevToken token.Token

	// curToken holds the current token from the lexer.
	curToken token.Token

	// peekToken holds the next token from the lexer.
	peekToken token.Token

	// parsing errors collected during parsing
	errors []ParserError

	// stmtErrorCount tracks error count at start of current statement.
	// Used by inner methods to detect if an error was added during this statement.
	stmtErrorCount int

	// prefixParseFns holds a map of parsing methods for
	// prefix-based syntax.
	prefixParseFns map[token.Type]prefixParseFn

	// infixParseFns holds a map of parsing methods for
	// infix-based syntax.
	infixParseFns map[token.Type]infixParseFn

	// are we inside a ternary expression?
	//
	// Nested ternary expressions are illegal :)
	tern bool

	// The filename of the input
	filename string

	// Current recursion depth
	depth int

	// Maximum allowed recursion depth
	maxDepth int
}

// New returns a Parser for the program provided by the given Lexer.
func New(l *lexer.Lexer, options ...Option) *Parser {
	// Create the parser and apply any provided options
	p := &Parser{
		l:              l,
		prefixParseFns: map[token.Type]prefixParseFn{},
		infixParseFns:  map[token.Type]infixParseFn{},
		maxDepth:       DefaultMaxDepth,
	}
	for _, opt := range options {
		opt(p)
	}

	// Prime the token pump
	p.nextToken() // makes curToken=<empty>, peekToken=token[0]
	p.nextToken() // makes curToken=token[0], peekToken=token[1]

	// Register prefix-functions
	p.registerPrefix(token.TEMPLATE, p.parseString)
	p.registerPrefix(token.BANG, p.parsePrefixExpr)
	p.registerPrefix(token.EOF, p.illegalToken)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.FLOAT, p.parseFloat)
	p.registerPrefix(token.FUNCTION, p.parseFunc)
	p.registerPrefix(token.IDENT, p.parseIdent)
	p.registerPrefix(token.IF, p.parseIf)
	p.registerPrefix(token.ILLEGAL, p.illegalToken)
	p.registerPrefix(token.INT, p.parseInt)
	p.registerPrefix(token.LBRACE, p.parseMapOrSet)
	p.registerPrefix(token.LBRACKET, p.parseList)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpr)
	p.registerPrefix(token.MINUS, p.parsePrefixExpr)
	p.registerPrefix(token.NEWLINE, p.parseNewline)
	p.registerPrefix(token.NIL, p.parseNil)
	p.registerPrefix(token.PIPE, p.parsePrefixExpr)
	p.registerPrefix(token.STRING, p.parseString)
	p.registerPrefix(token.SWITCH, p.parseSwitch)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.SPREAD, p.parseSpread)
	p.registerPrefix(token.TRY, p.parseTry)

	// Register infix functions
	p.registerInfix(token.AND, p.parseInfixExpr)
	p.registerInfix(token.ASSIGN, p.parseAssign)
	p.registerInfix(token.ASTERISK_EQUALS, p.parseAssign)
	p.registerInfix(token.ASTERISK, p.parseInfixExpr)
	p.registerInfix(token.AMPERSAND, p.parseInfixExpr)
	p.registerInfix(token.EQ, p.parseInfixExpr)
	p.registerInfix(token.GT_EQUALS, p.parseInfixExpr)
	p.registerInfix(token.GT_GT, p.parseInfixExpr)
	p.registerInfix(token.GT, p.parseInfixExpr)
	p.registerInfix(token.IN, p.parseIn)
	p.registerInfix(token.LBRACKET, p.parseIndex)
	p.registerInfix(token.LPAREN, p.parseCall)
	p.registerInfix(token.LT_EQUALS, p.parseInfixExpr)
	p.registerInfix(token.LT_LT, p.parseInfixExpr)
	p.registerInfix(token.LT, p.parseInfixExpr)
	p.registerInfix(token.MINUS_EQUALS, p.parseAssign)
	p.registerInfix(token.MINUS, p.parseInfixExpr)
	p.registerInfix(token.MOD, p.parseInfixExpr)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpr)
	p.registerInfix(token.NOT, p.parseNotIn)
	p.registerInfix(token.NULLISH, p.parseInfixExpr)
	p.registerInfix(token.OR, p.parseInfixExpr)
	p.registerInfix(token.PERIOD, p.parseGetAttr)
	p.registerInfix(token.QUESTION_DOT, p.parseOptionalChain)
	p.registerInfix(token.PIPE, p.parsePipe)
	p.registerInfix(token.PLUS_EQUALS, p.parseAssign)
	p.registerInfix(token.PLUS, p.parseInfixExpr)
	p.registerInfix(token.POW, p.parseInfixExpr)
	p.registerInfix(token.QUESTION, p.parseTernary)
	p.registerInfix(token.SLASH_EQUALS, p.parseAssign)
	p.registerInfix(token.SLASH, p.parseInfixExpr)

	return p
}

// advanceToken moves to the next token from the lexer without error checking.
// Used internally by synchronize() during error recovery.
func (p *Parser) advanceToken() {
	p.prevToken = p.curToken
	p.curToken = p.peekToken
	p.peekToken, _ = p.l.Next()
}

// nextToken moves to the next token from the lexer, updating all of
// prevToken, curToken, and peekToken.
func (p *Parser) nextToken() error {
	var err error
	p.prevToken = p.curToken
	p.curToken = p.peekToken
	p.peekToken, err = p.l.Next()
	if err == nil {
		return nil // success
	}
	// The lexer encountered an error. We consider all lexer errors
	// "syntax errors" and parsing will now be considered broken.
	p.addError(NewSyntaxError(ErrorOpts{
		Cause:         err,
		File:          p.l.Filename(),
		StartPosition: p.peekToken.StartPosition,
		EndPosition:   p.peekToken.EndPosition,
		SourceCode:    p.l.GetLineText(p.peekToken),
	}))
	return err
}

// Parse the program that is provided via the lexer.
// Returns the AST and any errors encountered. If there are errors, the AST
// may be partial (containing only successfully parsed statements).
func (p *Parser) Parse(ctx context.Context) (*ast.Program, error) {
	p.ctx = ctx
	// It's possible for errors to already exist because we read tokens from
	// the lexer in the constructor.
	if p.hasErrors() {
		return nil, NewErrors(p.errors)
	}
	// Parse the entire input program as a series of statements.
	// When a statement fails, we synchronize and continue to collect more errors.
	var statements []ast.Node
	for p.curToken.Type != token.EOF {
		// Check for context timeout
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// Stop if we've collected too many errors
		if p.tooManyErrors() {
			break
		}
		// Track error count for this statement so inner methods can detect new errors
		p.stmtErrorCount = len(p.errors)
		stmt := p.parseStatementStrict()
		if stmt != nil {
			statements = append(statements, stmt)
		} else if p.hadNewError() {
			// Statement failed - synchronize and continue
			p.synchronize()
		}
		p.nextToken()
	}
	if p.hasErrors() {
		return &ast.Program{Stmts: statements}, NewErrors(p.errors)
	}
	return &ast.Program{Stmts: statements}, nil
}

// registerPrefix registers a function for handling a prefix-based statement.
func (p *Parser) registerPrefix(tokenType token.Type, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix registers a function for handling an infix-based statement.
func (p *Parser) registerInfix(tokenType token.Type, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// MaxErrors is the maximum number of errors to collect before stopping.
const MaxErrors = 10

// addError appends an error to the errors slice.
func (p *Parser) addError(err ParserError) {
	p.errors = append(p.errors, err)
}

// hasErrors returns true if any errors have been recorded.
func (p *Parser) hasErrors() bool {
	return len(p.errors) > 0
}

// tooManyErrors returns true if error limit has been reached.
func (p *Parser) tooManyErrors() bool {
	return len(p.errors) >= MaxErrors
}

// hadNewError returns true if an error was added during the current statement.
func (p *Parser) hadNewError() bool {
	return len(p.errors) > p.stmtErrorCount
}

// synchronize skips tokens until a statement boundary is reached.
// This is used for error recovery to continue parsing after an error.
func (p *Parser) synchronize() {
	for !p.curTokenIs(token.EOF) {
		// Stop at statement terminators
		if statementTerminators[p.curToken.Type] {
			return
		}
		// Stop at statement-starting keywords
		switch p.curToken.Type {
		case token.LET, token.CONST, token.RETURN, token.IF,
			token.FUNCTION, token.SWITCH, token.TRY, token.THROW:
			return
		}
		prevPos := p.curToken.StartPosition
		p.advanceToken()
		// Safety: if we didn't advance (lexer stuck), bail out
		if p.curToken.StartPosition == prevPos {
			return
		}
	}
}

func (p *Parser) noPrefixParseFnError(t token.Token) {
	p.addError(NewParserError(ErrorOpts{
		ErrType:       "parse error",
		Message:       fmt.Sprintf("invalid syntax (unexpected %q)", t.Literal),
		File:          p.l.Filename(),
		StartPosition: t.StartPosition,
		EndPosition:   t.EndPosition,
		SourceCode:    p.l.GetLineText(t),
	}))
}

// peekError raises an error if the next token is not the expected type.
func (p *Parser) peekError(context string, expected token.Type, got token.Token) {
	gotDesc := tokenDescription(got)
	expDesc := tokenTypeDescription(expected)
	p.addError(NewParserError(ErrorOpts{
		ErrType: "parse error",
		Message: fmt.Sprintf("unexpected %s while parsing %s (expected %s)",
			gotDesc, context, expDesc),
		File:          p.l.Filename(),
		StartPosition: got.StartPosition,
		EndPosition:   got.EndPosition,
		SourceCode:    p.l.GetLineText(got),
	}))
}

func (p *Parser) setError(err ParserError) {
	p.addError(err)
}

// cancelled checks if the parsing context has been cancelled.
// Returns true if cancelled, in which case parsing should stop.
func (p *Parser) cancelled() bool {
	if p.ctx == nil {
		return false
	}
	select {
	case <-p.ctx.Done():
		p.setError(NewParserError(ErrorOpts{
			ErrType: "context error",
			Message: p.ctx.Err().Error(),
		}))
		return true
	default:
		return false
	}
}

func (p *Parser) parseStatementStrict() ast.Node {
	stmt := p.parseStatement()
	if stmt == nil {
		return nil
	}
	// statement should end with a semicolon or the next token should be
	// a statement terminator
	if !p.curTokenIs(token.SEMICOLON) && !statementTerminators[p.peekToken.Type] {
		p.setTokenError(p.curToken, "unexpected token %q following statement", p.peekToken.Literal)
		return nil
	}
	return stmt
}

func (p *Parser) parseStatement() ast.Node {
	var stmt ast.Node
	switch p.curToken.Type {
	case token.LET:
		stmt = p.parseLet()
	case token.CONST:
		stmt = p.parseConst()
	case token.RETURN:
		stmt = p.parseReturn()
	case token.THROW:
		stmt = p.parseThrow()
	case token.NEWLINE:
		stmt = nil
	default:
		stmt = p.parseExpressionStatement()
	}
	// Consume trailing semicolon if present
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseLet() ast.Node {
	letPos := p.curToken.StartPosition

	// Check for object destructuring: let { a, b } = obj
	if p.peekTokenIs(token.LBRACE) {
		return p.parseObjectDestructure(letPos)
	}

	// Check for array destructuring: let [a, b] = arr
	if p.peekTokenIs(token.LBRACKET) {
		return p.parseArrayDestructure(letPos)
	}

	if !p.expectPeek("let statement", token.IDENT) {
		return nil
	}
	idents := []*ast.Ident{p.newIdent(p.curToken)}
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if !p.expectPeek("let statement", token.IDENT) {
			return nil
		}
		idents = append(idents, p.newIdent(p.curToken))
	}
	if !p.expectPeek("let statement", token.ASSIGN) {
		return nil
	}
	p.nextToken()
	value := p.parseAssignmentValue()
	if value == nil {
		return nil
	}
	if len(idents) > 1 {
		return &ast.MultiVar{Let: letPos, Names: idents, Value: value}
	}
	return &ast.Var{Let: letPos, Name: idents[0], Value: value}
}

func (p *Parser) parseObjectDestructure(letPos token.Position) ast.Node {
	p.nextToken() // Move to '{'
	lbrace := p.curToken.StartPosition
	p.nextToken() // Move past '{'

	bindings := []ast.DestructureBinding{}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.cancelled() {
			return nil
		}
		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected identifier in destructuring pattern")
			return nil
		}

		key := p.curToken.Literal
		alias := "" // By default, alias is empty (use key as variable name)
		var defaultValue ast.Expr

		// Check for alias: { a: x }
		if p.peekTokenIs(token.COLON) {
			p.nextToken() // Move to ':'
			if !p.expectPeek("destructuring alias", token.IDENT) {
				return nil
			}
			alias = p.curToken.Literal
		}

		// Check for default value: { a = 10 } or { a: x = 10 }
		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // Move to '='
			p.nextToken() // Move past '='
			defaultValue = p.parseExpression(LOWEST)
			if defaultValue == nil {
				return nil
			}
		}

		bindings = append(bindings, ast.DestructureBinding{Key: key, Alias: alias, Default: defaultValue})

		// Check for comma or end
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // Move to ','
			p.nextToken() // Move past ','
		} else if p.peekTokenIs(token.RBRACE) {
			p.nextToken() // Move to '}'
		} else {
			p.setTokenError(p.peekToken, "expected ',' or '}' in destructuring pattern")
			return nil
		}
	}

	if !p.curTokenIs(token.RBRACE) {
		p.setTokenError(p.curToken, "expected '}' to close destructuring pattern")
		return nil
	}
	rbrace := p.curToken.StartPosition

	if len(bindings) == 0 {
		p.setTokenError(p.curToken, "destructuring pattern cannot be empty")
		return nil
	}

	// Expect '='
	if !p.expectPeek("destructuring assignment", token.ASSIGN) {
		return nil
	}

	p.nextToken()
	value := p.parseAssignmentValue()
	if value == nil {
		return nil
	}

	return &ast.ObjectDestructure{
		Let:      letPos,
		Lbrace:   lbrace,
		Bindings: bindings,
		Rbrace:   rbrace,
		Value:    value,
	}
}

func (p *Parser) parseArrayDestructure(letPos token.Position) ast.Node {
	p.nextToken() // Move to '['
	lbrack := p.curToken.StartPosition
	p.nextToken() // Move past '['

	elements := []ast.ArrayDestructureElement{}

	for !p.curTokenIs(token.RBRACKET) && !p.curTokenIs(token.EOF) {
		if p.cancelled() {
			return nil
		}
		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected identifier in array destructuring pattern")
			return nil
		}

		elem := ast.ArrayDestructureElement{
			Name: p.newIdent(p.curToken),
		}

		// Check for default value
		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // Move to '='
			p.nextToken() // Move past '='
			elem.Default = p.parseExpression(LOWEST)
			if elem.Default == nil {
				return nil
			}
		}

		elements = append(elements, elem)

		// Check for comma or end
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // Move to ','
			p.nextToken() // Move past ','
		} else if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // Move to ']'
		} else {
			p.setTokenError(p.peekToken, "expected ',' or ']' in array destructuring pattern")
			return nil
		}
	}

	if !p.curTokenIs(token.RBRACKET) {
		p.setTokenError(p.curToken, "expected ']' to close array destructuring pattern")
		return nil
	}
	rbrack := p.curToken.StartPosition

	if len(elements) == 0 {
		p.setTokenError(p.curToken, "array destructuring pattern cannot be empty")
		return nil
	}

	// Expect '='
	if !p.expectPeek("array destructuring assignment", token.ASSIGN) {
		return nil
	}

	p.nextToken()
	value := p.parseAssignmentValue()
	if value == nil {
		return nil
	}

	return &ast.ArrayDestructure{
		Let:      letPos,
		Lbrack:   lbrack,
		Elements: elements,
		Rbrack:   rbrack,
		Value:    value,
	}
}

func (p *Parser) parseConst() *ast.Const {
	constPos := p.curToken.StartPosition
	if !p.expectPeek("const statement", token.IDENT) {
		return nil
	}
	ident := p.newIdent(p.curToken)
	if !p.expectPeek("const statement", token.ASSIGN) {
		return nil
	}
	p.nextToken()
	value := p.parseAssignmentValue()
	if value == nil {
		return nil
	}
	return &ast.Const{Const: constPos, Name: ident, Value: value}
}

// Parses the right hand side of an assignment statement.
func (p *Parser) parseAssignmentValue() ast.Expr {
	result := p.parseExpression(LOWEST)
	if result == nil {
		// Only add error if none was added during parsing
		if !p.hadNewError() {
			p.setError(NewParserError(ErrorOpts{
				ErrType:       "parse error",
				Message:       "assignment is missing a value",
				File:          p.l.Filename(),
				StartPosition: p.prevToken.EndPosition,
				EndPosition:   p.prevToken.EndPosition,
				SourceCode:    p.l.GetLineText(p.prevToken),
			}))
		}
		return nil
	}
	return result
}

func (p *Parser) parseReturn() *ast.Return {
	returnPos := p.curToken.StartPosition
	if p.peekTokenIs(token.SEMICOLON) ||
		p.peekTokenIs(token.NEWLINE) ||
		p.peekTokenIs(token.RBRACE) ||
		p.peekTokenIs(token.EOF) {
		return &ast.Return{Return: returnPos, Value: nil}
	}
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}
	return &ast.Return{Return: returnPos, Value: value}
}

func (p *Parser) parseExpressionStatement() ast.Node {
	expr := p.parseNode(LOWEST)
	if expr == nil {
		// Only add error if none was added during parsing
		if !p.hadNewError() {
			p.setTokenError(p.curToken, "invalid syntax")
		}
		return nil
	}
	return expr
}

func (p *Parser) parseNode(precedence int) ast.Node {
	if p.curToken.Type == token.EOF || p.hadNewError() {
		return nil
	}
	// Check recursion depth
	p.depth++
	if p.depth > p.maxDepth {
		p.setTokenError(p.curToken, "maximum nesting depth exceeded")
		p.depth--
		return nil
	}
	defer func() { p.depth-- }()

	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken)
		return nil
	}
	leftExp := prefix()
	if p.hadNewError() || leftExp == nil {
		return nil
	}
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		if err := p.nextToken(); err != nil {
			return nil
		}
		leftExp = infix(leftExp)
		if p.hadNewError() {
			break
		}
	}
	// Check for postfix operators (++ or --)
	if p.peekTokenIs(token.PLUS_PLUS) || p.peekTokenIs(token.MINUS_MINUS) {
		p.nextToken()
		return p.parsePostfix(leftExp)
	}
	return leftExp
}

func (p *Parser) parseExpression(precedence int) ast.Expr {
	node := p.parseNode(precedence)
	if node == nil {
		return nil
	}
	if p.hadNewError() {
		return nil
	}
	if expr, ok := node.(ast.Expr); ok {
		return expr
	}
	p.setTokenError(p.prevToken, "expected expression")
	return nil
}

func (p *Parser) illegalToken() ast.Node {
	p.setError(NewParserError(ErrorOpts{
		ErrType:       "parse error",
		Message:       fmt.Sprintf("illegal token %s", p.curToken.Literal),
		File:          p.l.Filename(),
		StartPosition: p.curToken.StartPosition,
		EndPosition:   p.curToken.EndPosition,
		SourceCode:    p.l.GetLineText(p.curToken),
	}))
	return nil
}

func (p *Parser) setTokenError(t token.Token, msg string, args ...interface{}) ast.Node {
	p.setError(NewParserError(ErrorOpts{
		ErrType:       "parse error",
		Message:       fmt.Sprintf(msg, args...),
		File:          p.l.Filename(),
		StartPosition: t.StartPosition,
		EndPosition:   t.EndPosition,
		SourceCode:    p.l.GetLineText(t),
	}))
	return nil
}

// newIdent creates a new Ident node from a token.
func (p *Parser) newIdent(tok token.Token) *ast.Ident {
	return &ast.Ident{NamePos: tok.StartPosition, Name: tok.Literal}
}

func (p *Parser) parseIdent() ast.Node {
	if p.curToken.Literal == "" {
		p.setTokenError(p.curToken, "invalid identifier")
		return nil
	}
	ident := p.newIdent(p.curToken)

	// Check for single-param arrow function: x => expr
	if p.peekTokenIs(token.ARROW) {
		arrowPos := p.curToken.StartPosition
		p.nextToken() // move to '=>'
		return p.parseArrowBody(arrowPos, []*ast.Ident{ident}, nil)
	}

	return ident
}

func (p *Parser) parseInt() ast.Node {
	tok, lit := p.curToken, p.curToken.Literal
	var value int64
	var err error
	if strings.HasPrefix(lit, "0x") {
		value, err = strconv.ParseInt(lit[2:], 16, 64) // hexadecimal
	} else if strings.HasPrefix(lit, "0") && len(lit) > 1 {
		value, err = strconv.ParseInt(lit[1:], 8, 64) // octal
	} else {
		value, err = strconv.ParseInt(lit, 10, 64) // decimal
	}
	if err != nil {
		p.setError(NewParserError(ErrorOpts{
			ErrType:       "parse error",
			Message:       fmt.Sprintf("invalid integer: %s", lit),
			File:          p.l.Filename(),
			StartPosition: tok.StartPosition,
			EndPosition:   tok.EndPosition,
			SourceCode:    p.l.GetLineText(tok),
		}))
		return nil
	}
	return &ast.Int{ValuePos: tok.StartPosition, Literal: lit, Value: value}
}

func (p *Parser) parseFloat() ast.Node {
	tok, lit := p.curToken, p.curToken.Literal
	value, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		p.setError(NewParserError(ErrorOpts{
			ErrType:       "parse error",
			Message:       fmt.Sprintf("invalid float: %s", lit),
			File:          p.l.Filename(),
			StartPosition: p.curToken.StartPosition,
			EndPosition:   p.curToken.EndPosition,
			SourceCode:    p.l.GetLineText(p.curToken),
		}))
		return nil
	}
	return &ast.Float{ValuePos: tok.StartPosition, Literal: lit, Value: value}
}

func (p *Parser) parseSwitch() ast.Node {
	switchPos := p.curToken.StartPosition
	if !p.expectPeek("switch statement", token.LPAREN) { // move to the "("
		return nil
	}
	lparen := p.curToken.StartPosition
	p.nextToken() // move past the "("
	switchValue := p.parseExpression(LOWEST)
	if switchValue == nil {
		return nil
	}
	if !p.expectPeek("switch statement", token.RPAREN) { // move to the ")"
		return nil
	}
	rparen := p.curToken.StartPosition
	if !p.expectPeek("switch statement", token.LBRACE) {
		return nil
	}
	lbrace := p.curToken.StartPosition
	p.nextToken()
	p.eatNewlines()
	// Process the switch case statements
	var cases []*ast.Case
	var defaultCaseCount int
	// Each time through this loop we process one case statement
	for !p.curTokenIs(token.RBRACE) {
		if p.cancelled() {
			return nil
		}
		if p.curTokenIs(token.EOF) {
			p.setTokenError(p.prevToken, "unterminated switch statement")
			return nil
		}
		if p.curToken.Literal != "case" && p.curToken.Literal != "default" {
			p.setTokenError(p.curToken, "expected 'case' or 'default' (got %s)", p.curToken.Literal)
			return nil
		}
		casePos := p.curToken.StartPosition
		var isDefaultCase bool
		var caseExprs []ast.Expr
		if p.curTokenIs(token.DEFAULT) {
			isDefaultCase = true
		} else if p.curTokenIs(token.CASE) {
			p.nextToken() // move to the token following "case"
			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil
			}
			caseExprs = append(caseExprs, expr)
			for p.peekTokenIs(token.COMMA) {
				p.nextToken() // move to the comma
				p.nextToken() // move to the following expression
				expr = p.parseExpression(LOWEST)
				if expr == nil {
					return nil
				}
				caseExprs = append(caseExprs, expr)
			}
		} else {
			p.setTokenError(p.curToken, "expected 'case' or 'default' (got %s)", p.curToken.Literal)
			return nil
		}
		if !p.expectPeek("switch statement", token.COLON) {
			return nil
		}
		colonPos := p.curToken.StartPosition
		// Now we are at the block of code to be executed for this case
		p.nextToken()
		p.eatNewlines()
		// An empty case statement is valid
		if p.curTokenIs(token.CASE) || p.curTokenIs(token.DEFAULT) || p.curTokenIs(token.RBRACE) {
			if isDefaultCase {
				defaultCaseCount++
				if defaultCaseCount > 1 {
					p.setTokenError(p.curToken, "switch statement has multiple default blocks")
					return nil
				}
				cases = append(cases, &ast.Case{
					Case:    casePos,
					Exprs:   nil,
					Colon:   colonPos,
					Body:    nil,
					Default: true,
				})
			} else {
				cases = append(cases, &ast.Case{
					Case:    casePos,
					Exprs:   caseExprs,
					Colon:   colonPos,
					Body:    nil,
					Default: false,
				})
			}
			continue
		}
		blockLbrace := p.curToken.StartPosition
		var blockStatements []ast.Node
		for {
			if p.cancelled() {
				return nil
			}
			// Skip over newlines and semicolons
			for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) {
				if err := p.nextToken(); err != nil {
					return nil
				}
			}
			// Any of these tokens indicate the end of the current case
			if p.curTokenIs(token.CASE) ||
				p.curTokenIs(token.DEFAULT) ||
				p.curTokenIs(token.RBRACE) ||
				p.curTokenIs(token.EOF) {
				break
			}
			// Parse one statement
			if s := p.parseStatement(); s != nil {
				blockStatements = append(blockStatements, s)
			}
			if !p.curTokenIs(token.SEMICOLON) &&
				!statementTerminators[p.peekToken.Type] &&
				!p.peekTokenIs(token.CASE) &&
				!p.peekTokenIs(token.DEFAULT) &&
				!p.peekTokenIs(token.RBRACE) {
				p.peekError("case statement", token.SEMICOLON, p.peekToken)
				return nil
			}
			// Move to the token just beyond the statement
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
		// For case blocks we use the same position for both braces since there are no actual braces
		block := &ast.Block{Lbrace: blockLbrace, Stmts: blockStatements, Rbrace: blockLbrace}
		if isDefaultCase {
			defaultCaseCount++
			if defaultCaseCount > 1 {
				p.setTokenError(p.curToken, "switch statement has multiple default blocks")
				return nil
			}
			cases = append(cases, &ast.Case{
				Case:    casePos,
				Exprs:   nil,
				Colon:   colonPos,
				Body:    block,
				Default: true,
			})
		} else {
			cases = append(cases, &ast.Case{
				Case:    casePos,
				Exprs:   caseExprs,
				Colon:   colonPos,
				Body:    block,
				Default: false,
			})
		}
	}
	rbrace := p.curToken.StartPosition
	return &ast.Switch{
		Switch: switchPos,
		Lparen: lparen,
		Value:  switchValue,
		Rparen: rparen,
		Lbrace: lbrace,
		Cases:  cases,
		Rbrace: rbrace,
	}
}

func (p *Parser) parseBoolean() ast.Node {
	return &ast.Bool{
		ValuePos: p.curToken.StartPosition,
		Literal:  p.curToken.Literal,
		Value:    p.curTokenIs(token.TRUE),
	}
}

func (p *Parser) parseNil() ast.Node {
	return &ast.Nil{NilPos: p.curToken.StartPosition}
}

func (p *Parser) parsePrefixExpr() ast.Node {
	opPos := p.curToken.StartPosition
	op := p.curToken.Literal
	if err := p.nextToken(); err != nil {
		return nil
	}
	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid prefix expression")
		return nil
	}
	return &ast.Prefix{OpPos: opPos, Op: op, X: right}
}

func (p *Parser) parseSpread() ast.Node {
	ellipsis := p.curToken.StartPosition
	if err := p.nextToken(); err != nil {
		return nil
	}
	// Parse the expression to be spread
	value := p.parseExpression(PREFIX)
	if value == nil {
		p.setTokenError(p.curToken, "expected expression after spread operator")
		return nil
	}
	return &ast.Spread{Ellipsis: ellipsis, X: value}
}

func (p *Parser) parseNewline() ast.Node {
	p.nextToken()
	return nil
}

func (p *Parser) parsePostfix(leftNode ast.Node) ast.Node {
	// Validate that the operand is assignable (Ident, Index, or GetAttr)
	expr, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid operand for postfix operator")
		return nil
	}
	switch expr.(type) {
	case *ast.Ident, *ast.Index, *ast.GetAttr:
		// Valid assignable expressions
	default:
		p.setTokenError(p.curToken, "cannot apply postfix operator to this expression")
		return nil
	}
	return &ast.Postfix{
		X:     expr,
		OpPos: p.curToken.StartPosition,
		Op:    p.curToken.Literal,
	}
}

func (p *Parser) parseInfixExpr(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid expression")
		return nil
	}
	opPos := p.curToken.StartPosition
	op := p.curToken.Literal
	precedence := p.currentPrecedence()
	p.nextToken()
	for p.curTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	right := p.parseExpression(precedence)
	if right == nil {
		p.setTokenError(p.curToken, "invalid expression")
		return nil
	}
	return &ast.Infix{X: left, OpPos: opPos, Op: op, Y: right}
}

func (p *Parser) parseTernary(conditionNode ast.Node) ast.Node {
	condition, ok := conditionNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid ternary expression")
		return nil
	}
	if p.tern {
		p.setTokenError(p.curToken, "nested ternary expression detected")
		return nil
	}
	p.tern = true
	defer func() { p.tern = false }()

	questionPos := p.curToken.StartPosition
	p.nextToken() // move past the '?'
	// Skip newlines after '?'
	p.eatNewlines()
	precedence := p.currentPrecedence()
	ifTrue := p.parseExpression(precedence)
	if ifTrue == nil {
		if !p.hadNewError() {
			p.setTokenError(p.curToken, "invalid syntax in ternary if true expression")
		}
		return nil
	}
	// Allow newlines before the colon
	if !p.skipNewlinesAndPeek(token.COLON) {
		p.peekError("ternary expression", token.COLON, p.peekToken)
		return nil
	}
	p.nextToken() // move to the ":"
	colonPos := p.curToken.StartPosition
	p.nextToken() // move past the ":"
	// Skip newlines after colon
	p.eatNewlines()
	ifFalse := p.parseExpression(precedence)
	if ifFalse == nil {
		if !p.hadNewError() {
			p.setTokenError(p.curToken, "invalid syntax in ternary if false expression")
		}
		return nil
	}
	return &ast.Ternary{
		Cond:     condition,
		Question: questionPos,
		IfTrue:   ifTrue,
		Colon:    colonPos,
		IfFalse:  ifFalse,
	}
}

func (p *Parser) parseGroupedExpr() ast.Node {
	openParen := p.curToken.StartPosition
	p.nextToken() // move past '('

	// Skip newlines after opening paren - newlines are allowed inside parens
	p.eatNewlines()

	// Check for empty params arrow function: () => ...
	if p.curTokenIs(token.RPAREN) {
		if p.peekTokenIs(token.ARROW) {
			p.nextToken() // move to '=>'
			return p.parseArrowBody(openParen, nil, nil)
		}
		p.setTokenError(p.curToken, "empty parentheses require arrow function syntax")
		return nil
	}

	// Parse first item - could be expression or arrow param with default
	// Use parseNode instead of parseExpression to allow Assign nodes for defaults
	firstItem := p.parseNode(LOWEST)
	if firstItem == nil {
		return nil
	}

	// Check if we have a comma (multiple items = must be arrow function)
	var items []ast.Node
	items = append(items, firstItem)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // move to ','
		p.nextToken() // move past ','
		// Skip newlines after comma
		p.eatNewlines()
		item := p.parseNode(LOWEST)
		if item == nil {
			return nil
		}
		items = append(items, item)
	}

	// Skip newlines before closing paren
	if !p.skipNewlinesAndPeek(token.RPAREN) {
		p.peekError("grouped expression or arrow function", token.RPAREN, p.peekToken)
		return nil
	}
	p.nextToken() // move to ')'

	// Check for arrow function
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // move to '=>'
		return p.parseArrowParams(openParen, items)
	}

	// Not an arrow function - must be a single grouped expression
	if len(items) > 1 {
		p.setTokenError(p.curToken, "comma-separated expressions require arrow function syntax: (x, y) => ...")
		return nil
	}

	// Ensure the single item is an expression
	expr, ok := firstItem.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "expected expression in grouped expression")
		return nil
	}

	return expr
}

// parseArrowParams validates items as arrow function parameters and parses the body
func (p *Parser) parseArrowParams(arrowPos token.Position, items []ast.Node) ast.Node {
	params := make([]*ast.Ident, 0, len(items))
	defaults := make(map[string]ast.Expr)

	for _, item := range items {
		switch v := item.(type) {
		case *ast.Ident:
			params = append(params, v)
		case *ast.Assign:
			// Handle default parameter: x = value
			if v.Name == nil {
				p.setTokenError(p.curToken, "invalid arrow function parameter")
				return nil
			}
			params = append(params, v.Name)
			defaults[v.Name.Name] = v.Value
		default:
			p.setTokenError(p.curToken, "invalid arrow function parameter: expected identifier")
			return nil
		}
	}

	return p.parseArrowBody(arrowPos, params, defaults)
}

// parseArrowBody parses the body of an arrow function (expression or block)
func (p *Parser) parseArrowBody(arrowPos token.Position, params []*ast.Ident, defaults map[string]ast.Expr) ast.Node {
	p.nextToken() // move past '=>'

	var body *ast.Block

	if p.curTokenIs(token.LBRACE) {
		// Block body: (x) => { ... }
		body = p.parseBlock()
		if body == nil {
			return nil
		}
	} else {
		// Expression body: (x) => x + 1
		// Wrap in implicit return
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			p.setTokenError(p.curToken, "invalid arrow function body")
			return nil
		}
		returnStmt := &ast.Return{Return: arrowPos, Value: expr}
		body = &ast.Block{Lbrace: arrowPos, Stmts: []ast.Node{returnStmt}, Rbrace: arrowPos}
	}

	if defaults == nil {
		defaults = make(map[string]ast.Expr)
	}

	// Arrow functions currently don't support rest parameters (nil)
	return &ast.Func{
		Func:      arrowPos,
		Name:      nil,
		Lparen:    arrowPos,
		Params:    params,
		Defaults:  defaults,
		RestParam: nil,
		Rparen:    arrowPos,
		Body:      body,
	}
}

// Parses an entire if, else if, else block. Else-ifs are handled recursively.
func (p *Parser) parseIf() ast.Node {
	ifPos := p.curToken.StartPosition
	if !p.expectPeek("an if expression", token.LPAREN) { // move to the "("
		return nil
	}
	lparen := p.curToken.StartPosition
	p.nextToken() // move past the "("
	cond := p.parseExpression(LOWEST)
	if cond == nil {
		return nil
	}
	if !p.expectPeek("an if expression", token.RPAREN) { // move to the ")"
		return nil
	}
	rparen := p.curToken.StartPosition
	if !p.expectPeek("an if expression", token.LBRACE) { // move to the "{"
		return nil
	}
	consequence := p.parseBlock()
	if consequence == nil {
		return nil
	}
	var alternative *ast.Block
	if p.peekTokenIs(token.ELSE) {
		p.nextToken()                // move to the "else"
		if p.peekTokenIs(token.IF) { // this is an "else if"
			p.nextToken() // move to the "if"
			nestedIfPos := p.curToken.StartPosition
			nestedIf := p.parseIf()
			alternative = &ast.Block{
				Lbrace: nestedIfPos,
				Stmts:  []ast.Node{nestedIf},
				Rbrace: nestedIfPos,
			}
			return &ast.If{
				If:          ifPos,
				Lparen:      lparen,
				Cond:        cond,
				Rparen:      rparen,
				Consequence: consequence,
				Alternative: alternative,
			}
		}
		if !p.expectPeek("an if expression", token.LBRACE) {
			return nil
		}
		alternative = p.parseBlock()
		if alternative == nil {
			return nil
		}
	}
	return &ast.If{
		If:          ifPos,
		Lparen:      lparen,
		Cond:        cond,
		Rparen:      rparen,
		Consequence: consequence,
		Alternative: alternative,
	}
}

func (p *Parser) parseBlock() *ast.Block {
	lbrace := p.curToken.StartPosition
	statements := []ast.Node{}
	if err := p.nextToken(); err != nil { // Move past the '{'
		return nil
	}
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.cancelled() {
			return nil
		}
		stmt := p.parseStatementStrict()
		if stmt != nil {
			statements = append(statements, stmt)
		}
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	if p.curTokenIs(token.EOF) {
		p.setTokenError(p.curToken, "unterminated block statement")
		return nil
	}
	rbrace := p.curToken.StartPosition
	return &ast.Block{Lbrace: lbrace, Stmts: statements, Rbrace: rbrace}
}

func (p *Parser) parseFunc() ast.Node {
	funcPos := p.curToken.StartPosition
	var ident *ast.Ident
	if p.peekTokenIs(token.IDENT) { // Read optional function name
		p.nextToken()
		ident = p.newIdent(p.curToken)
	}
	if !p.expectPeek("function", token.LPAREN) { // Move to the "("
		return nil
	}
	lparen := p.curToken.StartPosition
	defaults, params, restParam := p.parseFuncParams()
	if defaults == nil { // parseFuncParams encountered an error
		return nil
	}
	rparen := p.curToken.StartPosition
	if !p.expectPeek("function", token.LBRACE) { // move to the "{"
		return nil
	}
	body := p.parseBlock()
	return &ast.Func{
		Func:      funcPos,
		Name:      ident,
		Lparen:    lparen,
		Params:    params,
		Defaults:  defaults,
		RestParam: restParam,
		Rparen:    rparen,
		Body:      body,
	}
}

func (p *Parser) parseFuncParams() (map[string]ast.Expr, []*ast.Ident, *ast.Ident) {
	// If the next parameter is ")", then there are no parameters
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return map[string]ast.Expr{}, nil, nil
	}
	defaults := map[string]ast.Expr{}
	params := make([]*ast.Ident, 0)
	var restParam *ast.Ident
	p.nextToken()
	for !p.curTokenIs(token.RPAREN) { // Keep going until we find a ")"
		if p.cancelled() {
			return nil, nil, nil
		}
		if p.curTokenIs(token.EOF) {
			p.setTokenError(p.prevToken, "unterminated function parameters")
			return nil, nil, nil
		}

		// Check for rest parameter: ...ident
		if p.curTokenIs(token.SPREAD) {
			if restParam != nil {
				p.setTokenError(p.curToken, "only one rest parameter is allowed")
				return nil, nil, nil
			}
			p.nextToken() // Move past ...
			if !p.curTokenIs(token.IDENT) {
				p.setTokenError(p.curToken, "expected identifier after ... in rest parameter")
				return nil, nil, nil
			}
			restParam = p.newIdent(p.curToken)
			p.nextToken()
			// Rest parameter must be last
			if !p.curTokenIs(token.RPAREN) {
				p.setTokenError(p.curToken, "rest parameter must be the last parameter")
				return nil, nil, nil
			}
			continue
		}

		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected an identifier (got %s)", p.curToken.Literal)
			return nil, nil, nil
		}
		ident := p.newIdent(p.curToken)
		params = append(params, ident)
		if err := p.nextToken(); err != nil {
			return nil, nil, nil
		}
		// If there is "=expr" after the name then expr is a default value
		if p.curTokenIs(token.ASSIGN) {
			p.nextToken()
			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil, nil, nil
			}
			defaults[ident.String()] = expr
			p.nextToken()
		}
		if p.curTokenIs(token.COMMA) {
			p.nextToken()
		}
	}
	return defaults, params, restParam
}

func (p *Parser) parseReserved() ast.Node {
	p.setTokenError(p.curToken, "reserved keyword: %s", p.curToken.Literal)
	return nil
}

func (p *Parser) parseReservedInfix(_ ast.Node) ast.Node {
	p.setTokenError(p.curToken, "reserved operator: %s", p.curToken.Literal)
	return nil
}

func (p *Parser) parseString() ast.Node {
	strToken := p.curToken
	// STRING (single or double quotes) - plain strings, no interpolation
	if strToken.Type == token.STRING {
		return &ast.String{
			ValuePos: strToken.StartPosition,
			Literal:  strToken.Literal,
			Value:    strToken.Literal,
		}
	}
	// TEMPLATE (backticks) - check for ${expr} interpolation
	if !strings.Contains(strToken.Literal, "${") {
		return &ast.String{
			ValuePos: strToken.StartPosition,
			Literal:  strToken.Literal,
			Value:    strToken.Literal,
		}
	}
	// Template string with ${expr} interpolation
	tmpl, err := tmpl.Parse(strToken.Literal)
	if err != nil {
		p.setTokenError(strToken, "%s", err.Error())
		return nil
	}
	var exprs []ast.Expr
	for _, e := range tmpl.Fragments() {
		if !e.IsVariable() {
			continue
		}
		tmplAst, err := Parse(p.ctx, e.Value(), WithFilename(p.l.Filename()))
		if err != nil {
			p.setTokenError(strToken, "in template interpolation: %s", err.Error())
			return nil
		}
		statements := tmplAst.Stmts
		if len(statements) == 0 {
			exprs = append(exprs, nil)
		} else if len(statements) > 1 {
			p.setTokenError(strToken, "template contains more than one expression")
			return nil
		} else {
			stmt := statements[0]
			expr, ok := stmt.(ast.Expr)
			if !ok {
				p.setTokenError(strToken, "template contains an unexpected statement type")
				return nil
			}
			exprs = append(exprs, expr)
		}
	}
	return &ast.String{
		ValuePos: strToken.StartPosition,
		Literal:  strToken.Literal,
		Value:    strToken.Literal,
		Template: tmpl,
		Exprs:    exprs,
	}
}

func (p *Parser) parseList() ast.Node {
	lbrack := p.curToken.StartPosition
	items := p.parseExprList(token.RBRACKET)
	if items == nil {
		return nil
	}
	rbrack := p.curToken.StartPosition
	return &ast.List{Lbrack: lbrack, Items: items, Rbrack: rbrack}
}

func (p *Parser) parseExprList(end token.Type) []ast.Expr {
	list := make([]ast.Expr, 0)
	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	p.nextToken()
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		p.setTokenError(p.curToken, "invalid syntax in list expression")
		return nil
	}
	list = append(list, expr)
	for p.peekTokenIs(token.COMMA) {
		// move to the comma
		if err := p.nextToken(); err != nil {
			return nil
		}
		// advance across any extra newlines
		for p.peekTokenIs(token.NEWLINE) {
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
		// check if the list has ended after the newlines
		if p.peekTokenIs(end) {
			break
		}
		// move to the next expression
		if err := p.nextToken(); err != nil {
			return nil
		}
		expr = p.parseExpression(LOWEST)
		if expr == nil {
			return nil
		}
		list = append(list, expr)
	}
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	if !p.expectPeek("an expression list", end) {
		return nil
	}
	return list
}

func (p *Parser) parseNodeList(end token.Type) []ast.Node {
	list := make([]ast.Node, 0)
	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	p.nextToken()
	expr := p.parseNode(LOWEST)
	if expr == nil {
		p.setTokenError(p.curToken, "invalid syntax in list expression")
		return nil
	}
	list = append(list, expr)
	for p.peekTokenIs(token.COMMA) {
		// move to the comma
		if err := p.nextToken(); err != nil {
			return nil
		}
		// advance across any extra newlines
		for p.peekTokenIs(token.NEWLINE) {
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
		// check if the list has ended after the newlines
		if p.peekTokenIs(end) {
			break
		}
		// move to the next expression
		if err := p.nextToken(); err != nil {
			return nil
		}
		expr = p.parseNode(LOWEST)
		if expr == nil {
			return nil
		}
		list = append(list, expr)
	}
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	if !p.expectPeek("a node list", end) {
		return nil
	}
	return list
}

func (p *Parser) parseIndex(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid index expression")
		return nil
	}
	lbrack := p.curToken.StartPosition
	var firstIndex, secondIndex ast.Expr
	if !p.peekTokenIs(token.COLON) {
		p.nextToken() // move to the first index
		firstIndex = p.parseExpression(LOWEST)
		if firstIndex == nil {
			return nil
		}
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // move to the "]"
			rbrack := p.curToken.StartPosition
			return &ast.Index{X: left, Lbrack: lbrack, Index: firstIndex, Rbrack: rbrack}
		}
	}
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // move to the ":"
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // move to the "]"
			rbrack := p.curToken.StartPosition
			return &ast.Slice{X: left, Lbrack: lbrack, Low: firstIndex, High: nil, Rbrack: rbrack}
		}
		p.nextToken() // move to the second index
		secondIndex = p.parseExpression(LOWEST)
		if secondIndex == nil {
			return nil
		}
	}
	if !p.expectPeek("an index expression", token.RBRACKET) {
		return nil
	}
	rbrack := p.curToken.StartPosition
	return &ast.Slice{X: left, Lbrack: lbrack, Low: firstIndex, High: secondIndex, Rbrack: rbrack}
}

func (p *Parser) parseAssign(name ast.Node) ast.Node {
	opPos := p.curToken.StartPosition
	op := p.curToken.Literal
	var ident *ast.Ident
	var index *ast.Index
	switch node := name.(type) {
	case *ast.Ident:
		ident = node
	case *ast.Index:
		index = node
	default:
		p.setTokenError(p.curToken, "unexpected token for assignment: %s", name.String())
		return nil
	}
	switch p.curToken.Type {
	case token.PLUS_EQUALS, token.MINUS_EQUALS, token.SLASH_EQUALS,
		token.ASTERISK_EQUALS, token.ASSIGN:
		// this is a valid operator
	default:
		p.setTokenError(p.curToken, "unsupported operator for assignment: %s", op)
		return nil
	}
	p.nextToken() // move to the RHS value
	right := p.parseExpression(LOWEST)
	if right == nil {
		p.setTokenError(p.curToken, "invalid assignment statement value")
		return nil
	}
	if index != nil {
		return &ast.Assign{Name: nil, Index: index, OpPos: opPos, Op: op, Value: right}
	}
	return &ast.Assign{Name: ident, Index: nil, OpPos: opPos, Op: op, Value: right}
}

func (p *Parser) parseCall(functionNode ast.Node) ast.Node {
	function, ok := functionNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid call expression")
		return nil
	}
	lparen := p.curToken.StartPosition
	arguments := p.parseNodeList(token.RPAREN)
	if arguments == nil {
		return nil
	}
	rparen := p.curToken.StartPosition
	return &ast.Call{Fun: function, Lparen: lparen, Args: arguments, Rparen: rparen}
}

func (p *Parser) parsePipe(firstNode ast.Node) ast.Node {
	first, ok := firstNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid pipe expression")
		return nil
	}
	exprs := []ast.Expr{first}
	for {
		// Move past the pipe operator itself
		if err := p.nextToken(); err != nil {
			return nil
		}
		// Advance across any extra newlines
		p.eatNewlines()
		// Parse the next expression and add it to the ast.Pipe Arguments
		expr := p.parseExpression(PIPE)
		if expr == nil {
			p.setTokenError(p.curToken, "invalid pipe expression")
			return nil
		}
		exprs = append(exprs, expr)
		// Another pipe character continues the expression
		if p.peekTokenIs(token.PIPE) {
			p.nextToken() // move to the next "|"
			continue
		} else {
			// Anything else indicates the end of the pipe expression
			break
		}
	}
	return &ast.Pipe{Exprs: exprs}
}

func (p *Parser) parseIn(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid in expression")
		return nil
	}
	inPos := p.curToken.StartPosition
	if err := p.nextToken(); err != nil {
		return nil
	}
	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid in expression")
		return nil
	}
	return &ast.In{X: left, InPos: inPos, Y: right}
}

func (p *Parser) parseNotIn(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid not in expression")
		return nil
	}

	notInPos := p.curToken.StartPosition

	// Check if the next token is IN
	if !p.peekTokenIs(token.IN) {
		p.setTokenError(p.peekToken, "expected 'in' after 'not' (got %s)", p.peekToken.Literal)
		return nil
	}

	// Move to the IN token
	if err := p.nextToken(); err != nil {
		return nil
	}

	// Move past the IN token to parse the right operand
	if err := p.nextToken(); err != nil {
		return nil
	}

	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid not in expression")
		return nil
	}

	return &ast.NotIn{X: left, NotInPos: notInPos, Y: right}
}

func (p *Parser) parseMapOrSet() ast.Node {
	lbrace := p.curToken.StartPosition
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	// Empty {} turns into an empty map (not a set)
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		rbrace := p.curToken.StartPosition
		return &ast.Map{Lbrace: lbrace, Items: nil, Rbrace: rbrace}
	}
	p.nextToken() // move to the first key or spread

	items := []ast.MapItem{}

	// Parse first item (could be spread or key-value)
	item := p.parseMapItem()
	if item == nil {
		return nil
	}
	items = append(items, *item)

	// Parse remaining items
	for !p.peekTokenIs(token.RBRACE) {
		if p.cancelled() {
			return nil
		}
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
			break
		}
		if !p.expectPeek("map", token.COMMA) {
			return nil
		}
		for p.peekTokenIs(token.NEWLINE) {
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
		if p.peekTokenIs(token.RBRACE) {
			break
		}
		p.nextToken() // move to the key or spread

		item := p.parseMapItem()
		if item == nil {
			return nil
		}
		items = append(items, *item)

		if !p.peekTokenIs(token.COMMA) {
			break
		}
	}
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	if !p.expectPeek("map", token.RBRACE) {
		return nil
	}
	rbrace := p.curToken.StartPosition
	return &ast.Map{Lbrace: lbrace, Items: items, Rbrace: rbrace}
}

// parseMapItem parses a single map item: either a spread (...obj) or a key-value pair.
func (p *Parser) parseMapItem() *ast.MapItem {
	// Check for spread expression
	if p.curTokenIs(token.SPREAD) {
		spreadNode := p.parseSpread()
		if spreadNode == nil {
			return nil
		}
		spread, ok := spreadNode.(ast.Expr)
		if !ok {
			p.setTokenError(p.curToken, "invalid spread expression")
			return nil
		}
		return &ast.MapItem{Key: nil, Value: spread}
	}

	// Regular key-value pair
	key := p.parseExpression(LOWEST)
	if key == nil {
		return nil
	}
	if !p.expectPeek("map", token.COLON) {
		return nil
	}
	p.nextToken() // move to the value
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}
	return &ast.MapItem{Key: key, Value: value}
}

func (p *Parser) parseGetAttr(objNode ast.Node) ast.Node {
	obj, ok := objNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid attribute expression")
		return nil
	}
	period := p.curToken.StartPosition
	p.nextToken()
	p.eatNewlines()
	if !p.curTokenIs(token.IDENT) {
		p.setTokenError(p.curToken, "expected an identifier after %q", ".")
		return nil
	}
	name := p.newIdent(p.curToken)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		callNode := p.parseCall(name)
		call, ok := callNode.(*ast.Call)
		if !ok {
			p.setTokenError(p.curToken, "invalid attribute expression")
			return nil
		}
		return &ast.ObjectCall{X: obj, Period: period, Call: call, Optional: false}
	} else if p.peekTokenIs(token.ASSIGN) ||
		p.peekTokenIs(token.PLUS_EQUALS) ||
		p.peekTokenIs(token.MINUS_EQUALS) ||
		p.peekTokenIs(token.ASTERISK_EQUALS) ||
		p.peekTokenIs(token.SLASH_EQUALS) {
		p.nextToken() // move to the operator
		opPos := p.curToken.StartPosition
		opLiteral := p.curToken.Literal
		p.nextToken() // move to the value
		right := p.parseExpression(LOWEST)
		if right == nil {
			p.setTokenError(p.curToken, "invalid assignment statement value")
			return nil
		}
		return &ast.SetAttr{X: obj, Period: period, Attr: name, OpPos: opPos, Op: opLiteral, Value: right}
	}
	return &ast.GetAttr{X: obj, Period: period, Attr: name, Optional: false}
}

func (p *Parser) parseOptionalChain(objNode ast.Node) ast.Node {
	obj, ok := objNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid optional chain expression")
		return nil
	}
	period := p.curToken.StartPosition
	p.nextToken()
	p.eatNewlines()
	if !p.curTokenIs(token.IDENT) {
		p.setTokenError(p.curToken, "expected an identifier after %q", "?.")
		return nil
	}
	name := p.newIdent(p.curToken)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		callNode := p.parseCall(name)
		call, ok := callNode.(*ast.Call)
		if !ok {
			p.setTokenError(p.curToken, "invalid optional chain expression")
			return nil
		}
		return &ast.ObjectCall{X: obj, Period: period, Call: call, Optional: true}
	}
	// Optional chaining does not support assignment
	return &ast.GetAttr{X: obj, Period: period, Attr: name, Optional: true}
}

// curTokenIs returns true if the current token has the given type.
func (p *Parser) curTokenIs(t token.Type) bool {
	return p.curToken.Type == t
}

// peekTokenIs returns true if the next token has the given type.
func (p *Parser) peekTokenIs(t token.Type) bool {
	return p.peekToken.Type == t
}

// expectPeek validates if the next token is of the given type, and advances if
// it is. If it's a different type, then an error is stored.
func (p *Parser) expectPeek(context string, t token.Type) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(context, t, p.peekToken)
	return false
}

// peekPrecedence returns the precedence of the next token.
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

// currentPrecedence returns the precedence of the current token.
func (p *Parser) currentPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) eatNewlines() {
	for p.curTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return
		}
	}
}

// skipNewlinesAndPeek checks if the given token type appears after optional
// newlines. If found, it skips the newlines and returns true (with peekToken
// now being the target). If not found, it returns false without consuming
// any tokens.
func (p *Parser) skipNewlinesAndPeek(targetType token.Type) bool {
	// If peek is already the target, no newlines to skip
	if p.peekTokenIs(targetType) {
		return true
	}
	// If peek is not a newline, target doesn't follow
	if !p.peekTokenIs(token.NEWLINE) {
		return false
	}
	// Save parser and lexer state
	savedCur := p.curToken
	savedPeek := p.peekToken
	savedLexer := p.l.SaveState()

	// Skip through newlines
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			// Restore state on error
			p.curToken = savedCur
			p.peekToken = savedPeek
			p.l.RestoreState(savedLexer)
			return false
		}
	}

	// Check if we found the target
	if p.peekTokenIs(targetType) {
		// Success - keep the new state (newlines consumed)
		return true
	}

	// Target not found - restore state
	p.curToken = savedCur
	p.peekToken = savedPeek
	p.l.RestoreState(savedLexer)
	return false
}

func (p *Parser) parseTry() ast.Node {
	tryPos := p.curToken.StartPosition

	// Expect opening brace for try block
	if !p.expectPeek("try statement", token.LBRACE) {
		return nil
	}

	// Parse try block
	tryBlock := p.parseBlock()
	if tryBlock == nil {
		return nil
	}

	var catchPos token.Position
	var catchIdent *ast.Ident
	var catchBlock *ast.Block
	var finallyPos token.Position
	var finallyBlock *ast.Block

	// Check for catch (allow newlines before it)
	if p.skipNewlinesAndPeek(token.CATCH) {
		p.nextToken() // move to "catch"
		catchPos = p.curToken.StartPosition

		// Check for optional catch identifier
		if p.peekTokenIs(token.IDENT) {
			p.nextToken() // move to identifier
			catchIdent = p.newIdent(p.curToken)
		}

		// Expect opening brace for catch block
		if !p.expectPeek("catch block", token.LBRACE) {
			return nil
		}

		catchBlock = p.parseBlock()
		if catchBlock == nil {
			return nil
		}
	}

	// Check for finally (allow newlines before it)
	if p.skipNewlinesAndPeek(token.FINALLY) {
		p.nextToken() // move to "finally"
		finallyPos = p.curToken.StartPosition

		// Expect opening brace for finally block
		if !p.expectPeek("finally block", token.LBRACE) {
			return nil
		}

		finallyBlock = p.parseBlock()
		if finallyBlock == nil {
			return nil
		}
	}

	// Require at least one of catch or finally
	if catchBlock == nil && finallyBlock == nil {
		p.setTokenError(p.curToken, "try statement requires at least one of catch or finally")
		return nil
	}

	return &ast.Try{
		Try:          tryPos,
		Body:         tryBlock,
		Catch:        catchPos,
		CatchIdent:   catchIdent,
		CatchBlock:   catchBlock,
		Finally:      finallyPos,
		FinallyBlock: finallyBlock,
	}
}

func (p *Parser) parseThrow() ast.Node {
	throwPos := p.curToken.StartPosition

	// Check if throw has a value
	if p.peekTokenIs(token.SEMICOLON) ||
		p.peekTokenIs(token.NEWLINE) ||
		p.peekTokenIs(token.RBRACE) ||
		p.peekTokenIs(token.EOF) {
		p.setTokenError(p.curToken, "throw statement requires a value")
		return nil
	}

	p.nextToken()
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}

	return &ast.Throw{Throw: throwPos, Value: value}
}
