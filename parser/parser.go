// Package parser is used to generate the abstract syntax tree (AST) for a program.
//
// A parser is created by calling New() with a lexer as input. The parser should
// then be used only once, by calling parser.Parse() to produce the AST.
package parser

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/lexer"
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
