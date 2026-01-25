// Package parser is used to generate the abstract syntax tree (AST) for a program.
//
// A parser is created by calling New() with a lexer as input. The parser should
// then be used only once, by calling parser.Parse() to produce the AST.
package parser

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/tmpl"
	"github.com/risor-io/risor/lexer"
	"github.com/risor-io/risor/token"
)

type (
	prefixParseFn  func() ast.Node
	infixParseFn   func(ast.Node) ast.Node
	postfixParseFn func() ast.Statement
)

var statementTerminators = map[token.Type]bool{
	token.SEMICOLON:   true,
	token.NEWLINE:     true,
	token.RBRACE:      true,
	token.EOF:         true,
	token.PLUS_PLUS:   true,
	token.MINUS_MINUS: true,
}

// Parse the provided input as Risor source code and return the AST. This is
// shorthand way to create a Lexer and Parser and then call Parse on that.
func Parse(ctx context.Context, input string, options ...Option) (*ast.Program, error) {
	l := lexer.New(input)
	p := New(l, options...)
	if p.filename != "" {
		// If an option specified a filename, pass that through to the lexer.
		l.SetFilename(p.filename)
	}
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

	// the parsing error, if any
	err ParserError

	// prefixParseFns holds a map of parsing methods for
	// prefix-based syntax.
	prefixParseFns map[token.Type]prefixParseFn

	// infixParseFns holds a map of parsing methods for
	// infix-based syntax.
	infixParseFns map[token.Type]infixParseFn

	// postfixParseFns holds a map of parsing methods for
	// postfix-based syntax.
	postfixParseFns map[token.Type]postfixParseFn

	// are we inside a ternary expression?
	//
	// Nested ternary expressions are illegal :)
	tern bool

	// The filename of the input
	filename string
}

// New returns a Parser for the program provided by the given Lexer.
func New(l *lexer.Lexer, options ...Option) *Parser {
	// Create the parser and apply any provided options
	p := &Parser{
		l:               l,
		prefixParseFns:  map[token.Type]prefixParseFn{},
		infixParseFns:   map[token.Type]infixParseFn{},
		postfixParseFns: map[token.Type]postfixParseFn{},
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
	p.registerPrefix(token.FOR, p.parseFor)
	p.registerPrefix(token.FROM, p.parseFromImport)
	p.registerPrefix(token.FUNCTION, p.parseFunc)
	p.registerPrefix(token.GO, p.parseReserved)
	p.registerPrefix(token.IDENT, p.parseIdent)
	p.registerPrefix(token.IF, p.parseIf)
	p.registerPrefix(token.ILLEGAL, p.illegalToken)
	p.registerPrefix(token.IMPORT, p.parseImport)
	p.registerPrefix(token.INT, p.parseInt)
	p.registerPrefix(token.LBRACE, p.parseMapOrSet)
	p.registerPrefix(token.LBRACKET, p.parseList)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpr)
	p.registerPrefix(token.MINUS, p.parsePrefixExpr)
	p.registerPrefix(token.NEWLINE, p.parseNewline)
	p.registerPrefix(token.NIL, p.parseNil)
	p.registerPrefix(token.PIPE, p.parsePrefixExpr)
	p.registerPrefix(token.RANGE, p.parseRange)
	p.registerPrefix(token.STRING, p.parseString)
	p.registerPrefix(token.SWITCH, p.parseSwitch)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.SEND, p.parseReserved)
	p.registerPrefix(token.SPREAD, p.parseSpread)

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
	p.registerInfix(token.PIPE_GT, p.parsePipeForward)
	p.registerInfix(token.PLUS_EQUALS, p.parseAssign)
	p.registerInfix(token.PLUS, p.parseInfixExpr)
	p.registerInfix(token.POW, p.parseInfixExpr)
	p.registerInfix(token.QUESTION, p.parseTernary)
	p.registerInfix(token.SEND, p.parseReservedInfix)
	p.registerInfix(token.SLASH_EQUALS, p.parseAssign)
	p.registerInfix(token.SLASH, p.parseInfixExpr)

	// Register postfix functions
	p.registerPostfix(token.MINUS_MINUS, p.parsePostfix)
	p.registerPostfix(token.PLUS_PLUS, p.parsePostfix)
	return p
}

// nextToken moves to the next token from the lexer, updating all of
// prevToken, curToken, and peekToken.
func (p *Parser) nextToken() error {
	// If we have an error, we can't move forward
	if p.err != nil {
		return p.err
	}
	var err error
	p.prevToken = p.curToken
	p.curToken = p.peekToken
	p.peekToken, err = p.l.Next()
	if err == nil {
		return nil // success
	}
	// The lexer encountered an error. We consider all lexer errors
	// "syntax errors" and parsing will now be considered broken.
	p.err = NewSyntaxError(ErrorOpts{
		Cause:         err,
		File:          p.l.Filename(),
		StartPosition: p.peekToken.StartPosition,
		EndPosition:   p.peekToken.EndPosition,
		SourceCode:    p.l.GetLineText(p.peekToken),
	})
	return p.err
}

// Parse the program that is provided via the lexer.
func (p *Parser) Parse(ctx context.Context) (*ast.Program, error) {
	p.ctx = ctx
	// It's possible for an error to already exist because we read tokens from
	// the lexer in the constructor. Parsing is already broken if so.
	if p.err != nil {
		return nil, p.err
	}
	// Parse the entire input program as a series of statements.
	// Parsing stops on the first occurrence of an error.
	var statements []ast.Node
	for p.curToken.Type != token.EOF {
		// Check for context timeout
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		stmt := p.parseStatementStrict()
		if stmt != nil {
			statements = append(statements, stmt)
		}
		if err := p.nextToken(); err != nil {
			return nil, err
		}
	}
	return ast.NewProgram(statements), p.err
}

// registerPrefix registers a function for handling a prefix-based statement.
func (p *Parser) registerPrefix(tokenType token.Type, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix registers a function for handling an infix-based statement.
func (p *Parser) registerInfix(tokenType token.Type, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// registerPostfix registers a function for handling a postfix-based statement.
func (p *Parser) registerPostfix(tokenType token.Type, fn postfixParseFn) {
	p.postfixParseFns[tokenType] = fn
}

func (p *Parser) noPrefixParseFnError(t token.Token) {
	if p.err != nil {
		return
	}
	p.err = NewParserError(ErrorOpts{
		ErrType:       "parse error",
		Message:       fmt.Sprintf("invalid syntax (unexpected %q)", t.Literal),
		File:          p.l.Filename(),
		StartPosition: t.StartPosition,
		EndPosition:   t.EndPosition,
		SourceCode:    p.l.GetLineText(t),
	})
}

// peekError raises an error if the next token is not the expected type.
func (p *Parser) peekError(context string, expected token.Type, got token.Token) {
	if p.err != nil {
		return
	}
	gotDesc := tokenDescription(got)
	expDesc := tokenTypeDescription(expected)
	p.err = NewParserError(ErrorOpts{
		ErrType: "parse error",
		Message: fmt.Sprintf("unexpected %s while parsing %s (expected %s)",
			gotDesc, context, expDesc),
		File:          p.l.Filename(),
		StartPosition: got.StartPosition,
		EndPosition:   got.EndPosition,
		SourceCode:    p.l.GetLineText(got),
	})
}

func (p *Parser) setError(err ParserError) {
	if p.err != nil {
		return
	}
	p.err = err
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
	case token.BREAK:
		stmt = p.parseBreak()
	case token.CONTINUE:
		stmt = p.parseContinue()
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
	tok := p.curToken

	// Check for object destructuring: let { a, b } = obj
	if p.peekTokenIs(token.LBRACE) {
		return p.parseObjectDestructure(tok)
	}

	// Check for array destructuring: let [a, b] = arr
	if p.peekTokenIs(token.LBRACKET) {
		return p.parseArrayDestructure(tok)
	}

	if !p.expectPeek("let statement", token.IDENT) {
		return nil
	}
	idents := []*ast.Ident{ast.NewIdent(p.curToken)}
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if !p.expectPeek("let statement", token.IDENT) {
			return nil
		}
		idents = append(idents, ast.NewIdent(p.curToken))
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
		return ast.NewMultiVar(tok, idents, value)
	}
	return ast.NewVar(tok, idents[0], value)
}

func (p *Parser) parseObjectDestructure(letToken token.Token) ast.Node {
	p.nextToken() // Move to '{'
	p.nextToken() // Move past '{'

	bindings := []ast.DestructureBinding{}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected identifier in destructuring pattern")
			return nil
		}

		key := p.curToken.Literal
		alias := "" // By default, alias is empty (use key as variable name)
		var defaultValue ast.Expression

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

	return ast.NewObjectDestructure(letToken, bindings, value)
}

func (p *Parser) parseArrayDestructure(letToken token.Token) ast.Node {
	p.nextToken() // Move to '['
	p.nextToken() // Move past '['

	elements := []ast.ArrayDestructureElement{}

	for !p.curTokenIs(token.RBRACKET) && !p.curTokenIs(token.EOF) {
		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected identifier in array destructuring pattern")
			return nil
		}

		elem := ast.ArrayDestructureElement{
			Name: ast.NewIdent(p.curToken),
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

	return ast.NewArrayDestructure(letToken, elements, value)
}

func (p *Parser) parseConst() *ast.Const {
	tok := p.curToken
	if !p.expectPeek("const statement", token.IDENT) {
		return nil
	}
	ident := ast.NewIdent(p.curToken)
	if !p.expectPeek("const statement", token.ASSIGN) {
		return nil
	}
	p.nextToken()
	value := p.parseAssignmentValue()
	if value == nil {
		return nil
	}
	return ast.NewConst(tok, ident, value)
}

// Parses the right hand side of an assignment statement.
func (p *Parser) parseAssignmentValue() ast.Expression {
	result := p.parseExpression(LOWEST)
	if result == nil {
		p.setError(NewParserError(ErrorOpts{
			ErrType:       "parse error",
			Message:       "assignment is missing a value",
			File:          p.l.Filename(),
			StartPosition: p.prevToken.EndPosition,
			EndPosition:   p.prevToken.EndPosition,
			SourceCode:    p.l.GetLineText(p.prevToken),
		}))
		return nil
	}
	return result
}

func (p *Parser) parseReturn() *ast.Return {
	returnToken := p.curToken
	if p.peekTokenIs(token.SEMICOLON) ||
		p.peekTokenIs(token.NEWLINE) ||
		p.peekTokenIs(token.RBRACE) ||
		p.peekTokenIs(token.EOF) {
		return ast.NewReturn(returnToken, nil)
	}
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}
	return ast.NewReturn(returnToken, value)
}

func (p *Parser) parseBreak() *ast.Control {
	return ast.NewControl(p.curToken, nil)
}

func (p *Parser) parseContinue() *ast.Control {
	return ast.NewControl(p.curToken, nil)
}

func (p *Parser) parseExpressionStatement() ast.Node {
	expr := p.parseNode(LOWEST)
	if expr == nil {
		p.setTokenError(p.curToken, "invalid syntax")
		return nil
	}
	return expr
}

func (p *Parser) parseNode(precedence int) ast.Node {
	if p.curToken.Type == token.EOF || p.err != nil {
		return nil
	}
	postfix := p.postfixParseFns[p.curToken.Type]
	if postfix != nil {
		return postfix()
	}
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken)
		return nil
	}
	leftExp := prefix()
	if p.err != nil || leftExp == nil {
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
		if p.err != nil {
			break
		}
	}
	return leftExp
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	node := p.parseNode(precedence)
	if node == nil {
		return nil
	}
	if p.err != nil {
		return nil
	}
	if expr, ok := node.(ast.Expression); ok {
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

func (p *Parser) parseIdent() ast.Node {
	if p.curToken.Literal == "" {
		p.setTokenError(p.curToken, "invalid identifier")
		return nil
	}
	ident := ast.NewIdent(p.curToken)

	// Check for single-param arrow function: x => expr
	if p.peekTokenIs(token.ARROW) {
		arrowToken := p.curToken
		p.nextToken() // move to '=>'
		return p.parseArrowBody(arrowToken, []*ast.Ident{ident}, nil)
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
	return ast.NewInt(tok, value)
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
	return ast.NewFloat(tok, value)
}

func (p *Parser) parseSwitch() ast.Node {
	switchToken := p.curToken
	p.nextToken()
	switchValue := p.parseExpression(LOWEST)
	if switchValue == nil {
		return nil
	}
	if !p.expectPeek("switch statement", token.LBRACE) {
		return nil
	}
	p.nextToken()
	p.eatNewlines()
	// Process the switch case statements
	var cases []*ast.Case
	var defaultCaseCount int
	// Each time through this loop we process one case statement
	for !p.curTokenIs(token.RBRACE) {
		if p.curTokenIs(token.EOF) {
			p.setTokenError(p.prevToken, "unterminated switch statement")
			return nil
		}
		if p.curToken.Literal != "case" && p.curToken.Literal != "default" {
			p.setTokenError(p.curToken, "expected 'case' or 'default' (got %s)", p.curToken.Literal)
			return nil
		}
		caseToken := p.curToken
		var isDefaultCase bool
		var caseExprs []ast.Expression
		if p.curTokenIs(token.DEFAULT) {
			isDefaultCase = true
		} else if p.curTokenIs(token.CASE) {
			p.nextToken() // move to the token following "case"
			caseExprs = append(caseExprs, p.parseExpression(LOWEST))
			for p.peekTokenIs(token.COMMA) {
				p.nextToken() // move to the comma
				p.nextToken() // move to the following expression
				caseExprs = append(caseExprs, p.parseExpression(LOWEST))
			}
		} else {
			p.setTokenError(p.curToken, "expected 'case' or 'default' (got %s)", p.curToken.Literal)
			return nil
		}
		if !p.expectPeek("switch statement", token.COLON) {
			return nil
		}
		// Now we are at the block of code to be executed for this case
		p.nextToken()
		p.eatNewlines()
		// An empty case statement is valid
		if p.curTokenIs(token.CASE) || p.curTokenIs(token.DEFAULT) || p.curTokenIs(token.RBRACE) {
			if isDefaultCase {
				defaultCaseCount++
				if defaultCaseCount > 1 {
					p.setTokenError(caseToken, "switch statement has multiple default blocks")
					return nil
				}
				cases = append(cases, ast.NewDefaultCase(caseToken, nil))
			} else {
				cases = append(cases, ast.NewCase(caseToken, caseExprs, nil))
			}
			continue
		}
		blockFirstToken := p.curToken
		var blockStatements []ast.Node
		for {
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
		block := ast.NewBlock(blockFirstToken, blockStatements)
		if isDefaultCase {
			defaultCaseCount++
			if defaultCaseCount > 1 {
				p.setTokenError(caseToken, "switch statement has multiple default blocks")
				return nil
			}
			cases = append(cases, ast.NewDefaultCase(caseToken, block))
		} else {
			cases = append(cases, ast.NewCase(caseToken, caseExprs, block))
		}
	}
	return ast.NewSwitch(switchToken, switchValue, cases)
}

// validateImportPath ensures that a given path string only contains valid identifiers
// separated by slash characters. Returns error if invalid.
func validateImportPath(path string) error {
	// Remove quotes if present - these are added when we convert an
	// identifier to a string in parseImport
	path = strings.Trim(path, "\"")

	// Valid path pattern: one or more valid identifiers separated by forward slashes
	// An identifier must start with a letter or underscore and can contain letters, digits, or underscores
	validPath := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)(\/[a-zA-Z_][a-zA-Z0-9_]*)*$`)

	if !validPath.MatchString(path) {
		// If path doesn't match pattern, provide a more specific error
		if strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
			return fmt.Errorf("path cannot begin or end with a slash")
		}
		if strings.Contains(path, "//") {
			return fmt.Errorf("path cannot contain empty components")
		}
		// Check individual components to provide better error messages
		for _, part := range strings.Split(path, "/") {
			if part == "" {
				return fmt.Errorf("empty path component")
			}
			if len(part) > 0 && !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(part) {
				return fmt.Errorf("invalid identifier '%s' in path", part)
			}
		}
		return fmt.Errorf("invalid import path format")
	}
	return nil
}

func (p *Parser) parseImport() ast.Node {
	importToken := p.curToken

	// Check for ES-style import: import { x, y } from 'm'
	if p.peekTokenIs(token.LBRACE) {
		return p.parseESImport(importToken)
	}

	// Support both identifier and string formats for module names
	if !p.peekTokenIs(token.IDENT) && !p.peekTokenIs(token.STRING) {
		p.peekError("an import statement", token.IDENT, p.peekToken)
		return nil
	}
	p.nextToken() // Move to the module name or path

	var pathStr *ast.String

	if p.curTokenIs(token.IDENT) {
		// Create a string from identifier without adding quotes
		identName := p.curToken.Literal
		pathStr = ast.NewString(token.Token{
			Type:          token.STRING,
			Literal:       identName,
			StartPosition: p.curToken.StartPosition,
			EndPosition:   p.curToken.EndPosition,
		})
	} else if p.curTokenIs(token.STRING) {
		// Handle string literal module path (e.g., "mydir/foo")
		path := p.parseString()
		if path == nil {
			p.setTokenError(p.curToken, "invalid module path in import statement")
			return nil
		}
		var ok bool
		pathStr, ok = path.(*ast.String)
		if !ok {
			p.setTokenError(p.curToken, "expected string literal for module path")
			return nil
		}
	}

	// Validate the path format
	pathValue := pathStr.Value()
	if err := validateImportPath(pathValue); err != nil {
		p.setTokenError(p.curToken, "invalid import path: %s", err.Error())
		return nil
	}

	var alias *ast.Ident
	if p.peekTokenIs(token.AS) {
		p.nextToken()
		if !p.expectPeek("an import statement", token.IDENT) {
			return nil
		}
		alias = ast.NewIdent(p.curToken)
	}

	return ast.NewImport(importToken, pathStr, alias)
}

// parseESImport parses ES-style imports: import { x, y } from 'm'
func (p *Parser) parseESImport(importToken token.Token) ast.Node {
	p.nextToken() // Move to '{'

	// Parse the list of imports inside braces
	imports := []*ast.Import{}
	p.nextToken() // Move past '{'

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected identifier in import list")
			return nil
		}

		importName := p.curToken.Literal
		importNameToken := p.curToken

		var alias *ast.Ident
		if p.peekTokenIs(token.AS) {
			p.nextToken() // Move to 'as'
			if !p.expectPeek("import alias", token.IDENT) {
				return nil
			}
			alias = ast.NewIdent(p.curToken)
		}

		// Create an Import node for this name
		pathStr := ast.NewString(token.Token{
			Type:          token.STRING,
			Literal:       importName,
			StartPosition: importNameToken.StartPosition,
			EndPosition:   importNameToken.EndPosition,
		})
		imports = append(imports, ast.NewImport(importNameToken, pathStr, alias))

		// Check for comma or end of list
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // Move to ','
			p.nextToken() // Move past ','
		} else if p.peekTokenIs(token.RBRACE) {
			p.nextToken() // Move to '}'
		} else {
			p.setTokenError(p.peekToken, "expected ',' or '}' in import list")
			return nil
		}
	}

	if !p.curTokenIs(token.RBRACE) {
		p.setTokenError(p.curToken, "expected '}' to close import list")
		return nil
	}

	// Validate we have at least one import
	if len(imports) == 0 {
		p.setTokenError(p.curToken, "import list cannot be empty")
		return nil
	}

	// Expect 'from' keyword
	if !p.expectPeek("ES import", token.FROM) {
		return nil
	}

	// Parse the module path
	if !p.peekTokenIs(token.STRING) && !p.peekTokenIs(token.IDENT) {
		p.peekError("ES import", token.STRING, p.peekToken)
		return nil
	}
	p.nextToken()

	var modulePath string
	if p.curTokenIs(token.STRING) {
		modulePath = strings.Trim(p.curToken.Literal, "\"'")
	} else {
		modulePath = p.curToken.Literal
	}

	// Validate the path
	if err := validateImportPath(modulePath); err != nil {
		p.setTokenError(p.curToken, "invalid import path: %s", err.Error())
		return nil
	}

	// Create parent module identifier
	parentIdent := ast.NewIdent(token.Token{
		Type:          token.IDENT,
		Literal:       modulePath,
		StartPosition: p.curToken.StartPosition,
		EndPosition:   p.curToken.EndPosition,
	})

	return ast.NewFromImport(importToken, []*ast.Ident{parentIdent}, imports, true)
}

func (p *Parser) parseFromImport() ast.Node {
	fromToken := p.curToken

	// Support both identifier and string formats for module names
	if !p.peekTokenIs(token.IDENT) && !p.peekTokenIs(token.STRING) {
		p.peekError("a from-import statement", token.IDENT, p.peekToken)
		return nil
	}

	p.nextToken() // Move to the first module part (identifier or string)

	parentModule := make([]*ast.Ident, 0)

	if p.curTokenIs(token.STRING) {
		// Handle string literal module path (e.g., "mydir/foo")
		path := p.parseString()
		if path == nil {
			p.setTokenError(p.curToken, "invalid module path in from-import statement")
			return nil
		}

		strPath, ok := path.(*ast.String)
		if !ok {
			p.setTokenError(p.curToken, "expected string literal for module path")
			return nil
		}

		// Validate the path format
		pathValue := strPath.Value()
		if err := validateImportPath(pathValue); err != nil {
			p.setTokenError(p.curToken, "invalid import path: %s", err.Error())
			return nil
		}

		// Convert the path to an identifier for compatibility with existing code
		pathValue = strPath.Value()

		// Use the whole string as a single parent module
		pathIdent := ast.NewIdent(token.Token{
			Type:          token.IDENT,
			Literal:       pathValue,
			StartPosition: p.curToken.StartPosition,
			EndPosition:   p.curToken.EndPosition,
		})
		parentModule = append(parentModule, pathIdent)

		p.nextToken()
	} else {
		// Handle the traditional dot-separated format for module paths
		for p.curTokenIs(token.IDENT) {
			parentModule = append(parentModule, ast.NewIdent(p.curToken))
			if err := p.nextToken(); err != nil {
				return nil
			}
			if !p.curTokenIs(token.PERIOD) {
				break
			}
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
	}

	if !p.curTokenIs(token.IMPORT) {
		p.setError(NewParserError(ErrorOpts{
			ErrType:       "parse error",
			Message:       "from-import is missing import statement",
			File:          p.l.Filename(),
			StartPosition: p.prevToken.EndPosition,
			EndPosition:   p.prevToken.EndPosition,
			SourceCode:    p.l.GetLineText(p.prevToken),
		}))
		return nil
	}

	importToken := p.curToken

	// If the imports are surrounded by parentheses, we are in a grouped import
	// which may span multiple lines
	isGrouped := false
	if p.peekTokenIs(token.LPAREN) {
		isGrouped = true
		p.nextToken()
		for p.peekTokenIs(token.NEWLINE) {
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
	}

	// Move to the first identifier
	if !p.expectPeek("a from-import statement", token.IDENT) {
		return nil
	}

	var imports []*ast.Import
	for {
		ident := p.curToken
		// Create a string from identifier without adding quotes
		pathStr := ast.NewString(token.Token{
			Type:          token.STRING,
			Literal:       ident.Literal,
			StartPosition: ident.StartPosition,
			EndPosition:   ident.EndPosition,
		})

		var alias *ast.Ident
		if p.peekTokenIs(token.AS) {
			p.nextToken()
			if !p.expectPeek("a from-import statement", token.IDENT) {
				return nil
			}
			alias = ast.NewIdent(p.curToken)
		}
		thisImport := ast.NewImport(importToken, pathStr, alias)
		imports = append(imports, thisImport)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if isGrouped {
				for p.peekTokenIs(token.NEWLINE) {
					if err := p.nextToken(); err != nil {
						return nil
					}
				}
				if p.peekTokenIs(token.RPAREN) {
					break
				}
			}
			// Advance to the next identifier
			if !p.expectPeek("a from-import statement", token.IDENT) {
				return nil
			}
		} else {
			break
		}
	}

	if isGrouped {
		if !p.expectPeek("a from-import statement", token.RPAREN) {
			return nil
		}
	}

	return ast.NewFromImport(fromToken, parentModule, imports, isGrouped)
}

func (p *Parser) parseBoolean() ast.Node {
	return ast.NewBool(p.curToken, p.curTokenIs(token.TRUE))
}

func (p *Parser) parseNil() ast.Node {
	return ast.NewNil(p.curToken)
}

func (p *Parser) parsePrefixExpr() ast.Node {
	operator := p.curToken
	if err := p.nextToken(); err != nil {
		return nil
	}
	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid prefix expression")
		return nil
	}
	return ast.NewPrefix(operator, right)
}

func (p *Parser) parseSpread() ast.Node {
	spreadToken := p.curToken
	if err := p.nextToken(); err != nil {
		return nil
	}
	// Parse the expression to be spread
	value := p.parseExpression(PREFIX)
	if value == nil {
		p.setTokenError(p.curToken, "expected expression after spread operator")
		return nil
	}
	return ast.NewSpread(spreadToken, value)
}

func (p *Parser) parseNewline() ast.Node {
	p.nextToken()
	return nil
}

func (p *Parser) parsePostfix() ast.Statement {
	return ast.NewPostfix(p.prevToken, p.curToken.Literal)
}

func (p *Parser) parseInfixExpr(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid expression")
		return nil
	}
	firstToken := p.curToken
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
	return ast.NewInfix(firstToken, left, firstToken.Literal, right)
}

func (p *Parser) parseTernary(conditionNode ast.Node) ast.Node {
	condition, ok := conditionNode.(ast.Expression)
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

	firstToken := p.curToken // the "?"
	p.nextToken()            // move past the '?'
	precedence := p.currentPrecedence()
	ifTrue := p.parseExpression(precedence)
	if ifTrue == nil {
		p.setTokenError(p.curToken, "invalid syntax in ternary if true expression")
	}
	if !p.expectPeek("ternary expression", token.COLON) { // moves to the ":"
		return nil
	}
	p.nextToken() // moves after the ":"
	ifFalse := p.parseExpression(precedence)
	if ifFalse == nil {
		p.setTokenError(p.curToken, "invalid syntax in ternary if false expression")
	}
	return ast.NewTernary(firstToken, condition, ifTrue, ifFalse)
}

func (p *Parser) parseGroupedExpr() ast.Node {
	openParen := p.curToken
	p.nextToken() // move past '('

	// Check for empty params arrow function: () => ...
	if p.curTokenIs(token.RPAREN) {
		if p.peekTokenIs(token.ARROW) {
			p.nextToken() // move to '=>'
			return p.parseArrowBody(openParen, nil, nil)
		}
		p.setTokenError(openParen, "empty parentheses require arrow function syntax")
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
		item := p.parseNode(LOWEST)
		if item == nil {
			return nil
		}
		items = append(items, item)
	}

	if !p.expectPeek("grouped expression or arrow function", token.RPAREN) {
		return nil
	}

	// Check for arrow function
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // move to '=>'
		return p.parseArrowParams(openParen, items)
	}

	// Not an arrow function - must be a single grouped expression
	if len(items) > 1 {
		p.setTokenError(openParen, "comma-separated expressions require arrow function syntax: (x, y) => ...")
		return nil
	}

	// Ensure the single item is an expression
	expr, ok := firstItem.(ast.Expression)
	if !ok {
		p.setTokenError(openParen, "expected expression in grouped expression")
		return nil
	}

	return expr
}

// parseArrowParams validates items as arrow function parameters and parses the body
func (p *Parser) parseArrowParams(arrowToken token.Token, items []ast.Node) ast.Node {
	params := make([]*ast.Ident, 0, len(items))
	defaults := make(map[string]ast.Expression)

	for _, item := range items {
		switch v := item.(type) {
		case *ast.Ident:
			params = append(params, v)
		case *ast.Assign:
			// Handle default parameter: x = value
			ident := v.NameIdent()
			if ident == nil {
				p.setTokenError(arrowToken, "invalid arrow function parameter")
				return nil
			}
			params = append(params, ident)
			defaults[ident.Literal()] = v.Value()
		default:
			p.setTokenError(arrowToken, "invalid arrow function parameter: expected identifier")
			return nil
		}
	}

	return p.parseArrowBody(arrowToken, params, defaults)
}

// parseArrowBody parses the body of an arrow function (expression or block)
func (p *Parser) parseArrowBody(arrowToken token.Token, params []*ast.Ident, defaults map[string]ast.Expression) ast.Node {
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
			p.setTokenError(arrowToken, "invalid arrow function body")
			return nil
		}
		returnStmt := ast.NewReturn(arrowToken, expr)
		body = ast.NewBlock(arrowToken, []ast.Node{returnStmt})
	}

	if defaults == nil {
		defaults = make(map[string]ast.Expression)
	}

	// Arrow functions currently don't support rest parameters (nil)
	return ast.NewFunc(arrowToken, nil, params, defaults, nil, body)
}

// Parses an entire if, else if, else block. Else-ifs are handled recursively.
func (p *Parser) parseIf() ast.Node {
	ifToken := p.curToken
	p.nextToken() // move past the "if"
	cond := p.parseExpression(LOWEST)
	if cond == nil {
		return nil
	}
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
			nestedIfToken := p.curToken
			nestedIf := p.parseIf()
			alternative := ast.NewBlock(nestedIfToken, []ast.Node{nestedIf})
			return ast.NewIf(ifToken, cond, consequence, alternative)
		}
		if !p.expectPeek("an if expression", token.LBRACE) {
			return nil
		}
		alternative = p.parseBlock()
		if alternative == nil {
			return nil
		}
	}
	return ast.NewIf(ifToken, cond, consequence, alternative)
}

func (p *Parser) parseFor() ast.Node {
	forToken := p.curToken
	p.nextToken() // Move past 'for'

	// Check for simple form: "for { ... }"
	if p.curTokenIs(token.LBRACE) {
		consequence := p.parseBlock()
		if consequence == nil {
			return nil
		}
		return ast.NewSimpleFor(forToken, consequence)
	}

	// Check for "for x in iterable" or "for i, v in iterable" form
	if p.curTokenIs(token.IDENT) {
		// Collect variable identifiers
		variables := []*ast.Ident{ast.NewIdent(p.curToken)}

		// Check for comma-separated variables: "for i, v in ..."
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // Move to ','
			if !p.expectPeek("for-in loop", token.IDENT) {
				return nil
			}
			variables = append(variables, ast.NewIdent(p.curToken))
		}

		// Now check for 'in' keyword
		if p.peekTokenIs(token.IN) {
			p.nextToken() // Move to 'in'
			p.nextToken() // Move past 'in'

			// Parse the iterable expression
			iterable := p.parseExpression(LOWEST)
			if iterable == nil {
				p.setTokenError(p.curToken, "invalid iterable in for-in loop")
				return nil
			}

			// Expect opening brace directly
			if !p.expectPeek("for-in loop", token.LBRACE) {
				return nil
			}

			consequence := p.parseBlock()
			if consequence == nil {
				return nil
			}

			return ast.NewForIn(forToken, variables, iterable, consequence)
		}

		// Not a for-in loop, need to backtrack and continue with regular for loop parsing
		// But we've consumed tokens... we need a different approach
		// Actually, if we got here with multiple variables but no 'in', it's a syntax error
		if len(variables) > 1 {
			p.setTokenError(p.curToken, "expected 'in' keyword after comma-separated variables in for loop")
			return nil
		}
		// Single variable without 'in' - fall through to regular for loop parsing
		// We need to "undo" by treating the identifier as part of init statement
	}

	// Parse the initialization or condition
	var init, condition, post ast.Node
	if !p.curTokenIs(token.SEMICOLON) {
		init = p.parseStatement()
		if init == nil {
			p.setTokenError(p.curToken, "invalid for loop expression")
			return nil
		}
	}

	// Check for condition-only form: "for condition { ... }"
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		consequence := p.parseBlock()
		if consequence == nil {
			return nil
		}
		return ast.NewFor(forToken, init, consequence, nil, nil)
	}

	// Check if the init statement is a range statement
	var isRangeLoop bool
	switch initNode := init.(type) {
	case *ast.Var:
		_, expr := initNode.Value()
		if _, ok := expr.(*ast.Range); ok {
			isRangeLoop = true
		}
	case *ast.MultiVar:
		_, expr := initNode.Value()
		if _, ok := expr.(*ast.Range); ok {
			isRangeLoop = true
		}
	}

	if isRangeLoop {
		// This is a range-based for loop
		if !p.expectPeek("for range loopx", token.LBRACE) {
			return nil
		}
		consequence := p.parseBlock()
		if consequence == nil {
			return nil
		}
		// Use the init node (which contains the range expression) as the condition
		return ast.NewFor(forToken, init, consequence, nil, nil)
	}

	// If we've reached here, we're dealing with a three-part for loop
	if !p.curTokenIs(token.SEMICOLON) {
		p.setTokenError(p.curToken, "expected semicolon after for loop initialization")
		return nil
	}
	p.nextToken() // Move past the first semicolon

	// Parse the condition
	if !p.curTokenIs(token.SEMICOLON) {
		condition = p.parseExpression(LOWEST)
		if condition == nil {
			p.setTokenError(p.curToken, "invalid for loop condition")
			return nil
		}
	}
	if !p.expectPeek("for loop", token.SEMICOLON) {
		return nil
	}
	p.nextToken() // Move past the second semicolon

	// Parse the post statement
	if !p.curTokenIs(token.LBRACE) {
		if p.curTokenIs(token.IDENT) && (p.peekTokenIs(token.PLUS_PLUS) || p.peekTokenIs(token.MINUS_MINUS)) {
			identToken := p.curToken
			p.nextToken()
			post = ast.NewPostfix(identToken, p.curToken.Literal)
		} else {
			post = p.parseStatement()
		}
		if post == nil {
			p.setTokenError(p.curToken, "invalid for loop post statement")
			return nil
		}
	}

	// Expect the opening brace of the loop body
	if !p.expectPeek("for loop", token.LBRACE) {
		return nil
	}

	consequence := p.parseBlock()
	if consequence == nil {
		return nil
	}

	return ast.NewFor(forToken, condition, consequence, init, post)
}

func (p *Parser) parseBlock() *ast.Block {
	blockToken := p.curToken // Should be '{'
	statements := []ast.Node{}
	if err := p.nextToken(); err != nil { // Move past the '{'
		return nil
	}
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatementStrict()
		if stmt != nil {
			statements = append(statements, stmt)
		}
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	if p.curTokenIs(token.EOF) {
		p.setTokenError(blockToken, "unterminated block statement")
		return nil
	}
	return ast.NewBlock(blockToken, statements)
}

func (p *Parser) parseFunc() ast.Node {
	funcToken := p.curToken
	var ident *ast.Ident
	if p.peekTokenIs(token.IDENT) { // Read optional function name
		p.nextToken()
		ident = ast.NewIdent(p.curToken)
	}
	if !p.expectPeek("function", token.LPAREN) { // Move to the "("
		return nil
	}
	defaults, params, restParam := p.parseFuncParams()
	if !p.expectPeek("function", token.LBRACE) { // move to the "{"
		return nil
	}
	return ast.NewFunc(funcToken, ident, params, defaults, restParam, p.parseBlock())
}

func (p *Parser) parseFuncParams() (map[string]ast.Expression, []*ast.Ident, *ast.Ident) {
	// If the next parameter is ")", then there are no parameters
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return map[string]ast.Expression{}, nil, nil
	}
	defaults := map[string]ast.Expression{}
	params := make([]*ast.Ident, 0)
	var restParam *ast.Ident
	p.nextToken()
	for !p.curTokenIs(token.RPAREN) { // Keep going until we find a ")"
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
			restParam = ast.NewIdent(p.curToken)
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
		ident := ast.NewIdent(p.curToken)
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
	p.setTokenError(p.curToken, fmt.Sprintf("reserved keyword: %s", p.curToken.Literal))
	return nil
}

func (p *Parser) parseReservedInfix(_ ast.Node) ast.Node {
	p.setTokenError(p.curToken, fmt.Sprintf("reserved operator: %s", p.curToken.Literal))
	return nil
}

func (p *Parser) parseString() ast.Node {
	strToken := p.curToken
	// STRING (single or double quotes) - plain strings, no interpolation
	if strToken.Type == token.STRING {
		return ast.NewString(strToken)
	}
	// TEMPLATE (backticks) - check for ${expr} interpolation
	if !strings.Contains(strToken.Literal, "${") {
		return ast.NewString(strToken)
	}
	// Template string with ${expr} interpolation
	tmpl, err := tmpl.Parse(strToken.Literal)
	if err != nil {
		p.setTokenError(strToken, err.Error())
		return nil
	}
	var exprs []ast.Expression
	for _, e := range tmpl.Fragments() {
		if !e.IsVariable() {
			continue
		}
		tmplAst, err := Parse(p.ctx, e.Value())
		if err != nil {
			p.setTokenError(strToken, err.Error())
			return nil
		}
		statements := tmplAst.Statements()
		if len(statements) == 0 {
			exprs = append(exprs, nil)
		} else if len(statements) > 1 {
			p.setTokenError(strToken, "template contains more than one expression")
			return nil
		} else {
			stmt := statements[0]
			expr, ok := stmt.(ast.Expression)
			if !ok {
				p.setTokenError(strToken, "template contains an unexpected statement type")
				return nil
			}
			exprs = append(exprs, expr)
		}
	}
	return ast.NewTemplatedString(strToken, tmpl, exprs)
}

func (p *Parser) parseList() ast.Node {
	bracket := p.curToken
	items := p.parseExprList(token.RBRACKET)
	return ast.NewList(bracket, items)
}

func (p *Parser) parseExprList(end token.Type) []ast.Expression {
	list := make([]ast.Expression, 0)
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
		list = append(list, p.parseExpression(LOWEST))
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
		list = append(list, p.parseNode(LOWEST))
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
	left, ok := leftNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid index expression")
		return nil
	}
	indexToken := p.curToken
	var firstIndex, secondIndex ast.Expression
	if !p.peekTokenIs(token.COLON) {
		p.nextToken() // move to the first index
		firstIndex = p.parseExpression(LOWEST)
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // move to the "]"
			return ast.NewIndex(indexToken, left, firstIndex)
		}
	}
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // move to the ":"
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // move to the "]"
			return ast.NewSlice(indexToken, left, firstIndex, nil)
		}
		p.nextToken() // move to the second index
		secondIndex = p.parseExpression(LOWEST)
	}
	if !p.expectPeek("an index expression", token.RBRACKET) {
		return nil
	}
	return ast.NewSlice(indexToken, left, firstIndex, secondIndex)
}

func (p *Parser) parseAssign(name ast.Node) ast.Node {
	operator := p.curToken
	var ident *ast.Ident
	var index *ast.Index
	switch node := name.(type) {
	case *ast.Ident:
		ident = node
	case *ast.Index:
		index = node
	default:
		p.setTokenError(operator, "unexpected token for assignment: %s", name.Literal())
		return nil
	}
	switch operator.Type {
	case token.PLUS_EQUALS, token.MINUS_EQUALS, token.SLASH_EQUALS,
		token.ASTERISK_EQUALS, token.ASSIGN:
		// this is a valid operator
	default:
		p.setTokenError(operator, "unsupported operator for assignment: %s", operator.Literal)
		return nil
	}
	p.nextToken() // move to the RHS value
	right := p.parseExpression(LOWEST)
	if right == nil {
		p.setTokenError(p.curToken, "invalid assignment statement value")
		return nil
	}
	if index != nil {
		return ast.NewAssignIndex(operator, index, right)
	}
	return ast.NewAssign(operator, ident, right)
}

func (p *Parser) parseCall(functionNode ast.Node) ast.Node {
	function, ok := functionNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid call expression")
		return nil
	}
	callToken := p.curToken
	arguments := p.parseNodeList(token.RPAREN)
	if arguments == nil {
		return nil
	}
	return ast.NewCall(callToken, function, arguments)
}

func (p *Parser) parsePipe(firstNode ast.Node) ast.Node {
	first, ok := firstNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid pipe expression")
		return nil
	}
	pipeToken := p.curToken
	exprs := []ast.Expression{first}
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
	return ast.NewPipe(pipeToken, exprs)
}

func (p *Parser) parsePipeForward(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid pipe forward expression")
		return nil
	}
	pipeToken := p.curToken

	// Move past the |> operator
	if err := p.nextToken(); err != nil {
		return nil
	}

	// Advance across any newlines
	p.eatNewlines()

	// Parse the right-hand side (the function)
	right := p.parseExpression(PIPE)
	if right == nil {
		p.setTokenError(p.curToken, "invalid pipe forward expression")
		return nil
	}

	result := ast.NewPipeForward(pipeToken, left, right)

	// Check for chained |> operators
	for p.peekTokenIs(token.PIPE_GT) {
		p.nextToken() // move to the next |>
		pipeToken = p.curToken
		p.nextToken() // move past |>
		p.eatNewlines()
		right = p.parseExpression(PIPE)
		if right == nil {
			p.setTokenError(p.curToken, "invalid pipe forward expression")
			return nil
		}
		result = ast.NewPipeForward(pipeToken, result, right)
	}

	return result
}

func (p *Parser) parseIn(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid in expression")
		return nil
	}
	inToken := p.curToken
	if err := p.nextToken(); err != nil {
		return nil
	}
	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid in expression")
		return nil
	}
	return ast.NewIn(inToken, left, right)
}

func (p *Parser) parseNotIn(leftNode ast.Node) ast.Node {
	left, ok := leftNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid not in expression")
		return nil
	}

	notToken := p.curToken

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

	return ast.NewNotIn(notToken, left, right)
}

func (p *Parser) parseRange() ast.Node {
	rangeToken := p.curToken
	if err := p.nextToken(); err != nil {
		return nil
	}
	if p.curTokenIs(token.LBRACE) {
		p.setTokenError(p.curToken, "invalid range expression (unexpected \"{\")")
		return nil
	}
	container := p.parseExpression(PREFIX)
	if container == nil {
		p.setTokenError(p.curToken, "invalid range expression")
		return nil
	}
	return ast.NewRange(rangeToken, container)
}

func (p *Parser) parseMapOrSet() ast.Node {
	firstToken := p.curToken
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	// Empty {} turns into an empty map (not a set)
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return ast.NewMap(firstToken, nil)
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
	return ast.NewMap(firstToken, items)
}

// parseMapItem parses a single map item: either a spread (...obj) or a key-value pair.
func (p *Parser) parseMapItem() *ast.MapItem {
	// Check for spread expression
	if p.curTokenIs(token.SPREAD) {
		spreadNode := p.parseSpread()
		if spreadNode == nil {
			return nil
		}
		spread, ok := spreadNode.(ast.Expression)
		if !ok {
			p.setTokenError(p.curToken, "invalid spread expression")
			return nil
		}
		return &ast.MapItem{Key: nil, Value: spread}
	}

	// Regular key-value pair
	key := p.parseExpression(LOWEST)
	if !p.expectPeek("map", token.COLON) {
		return nil
	}
	p.nextToken() // move to the value
	value := p.parseExpression(LOWEST)
	return &ast.MapItem{Key: key, Value: value}
}

func (p *Parser) parseGetAttr(objNode ast.Node) ast.Node {
	obj, ok := objNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid attribute expression")
		return nil
	}
	period := p.curToken
	p.nextToken()
	p.eatNewlines()
	if !p.curTokenIs(token.IDENT) {
		p.setTokenError(p.curToken, "expected an identifier after %q", ".")
		return nil
	}
	name := p.parseIdent().(*ast.Ident)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		callNode := p.parseCall(name)
		call, ok := callNode.(ast.Expression)
		if !ok {
			p.setTokenError(p.curToken, "invalid attribute expression")
			return nil
		}
		return ast.NewObjectCall(period, obj, call)
	} else if p.peekTokenIs(token.ASSIGN) ||
		p.peekTokenIs(token.PLUS_EQUALS) ||
		p.peekTokenIs(token.MINUS_EQUALS) ||
		p.peekTokenIs(token.ASTERISK_EQUALS) ||
		p.peekTokenIs(token.SLASH_EQUALS) {
		p.nextToken() // move to the operator
		operator := p.curToken
		p.nextToken() // move to the value
		right := p.parseExpression(LOWEST)
		if right == nil {
			p.setTokenError(p.curToken, "invalid assignment statement value")
			return nil
		}
		return ast.NewSetAttr(operator, obj, name, right)
	}
	return ast.NewGetAttr(period, obj, name)
}

func (p *Parser) parseOptionalChain(objNode ast.Node) ast.Node {
	obj, ok := objNode.(ast.Expression)
	if !ok {
		p.setTokenError(p.curToken, "invalid optional chain expression")
		return nil
	}
	tok := p.curToken
	p.nextToken()
	p.eatNewlines()
	if !p.curTokenIs(token.IDENT) {
		p.setTokenError(p.curToken, "expected an identifier after %q", "?.")
		return nil
	}
	name := p.parseIdent().(*ast.Ident)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		callNode := p.parseCall(name)
		call, ok := callNode.(ast.Expression)
		if !ok {
			p.setTokenError(p.curToken, "invalid optional chain expression")
			return nil
		}
		return ast.NewOptionalObjectCall(tok, obj, call)
	}
	// Optional chaining does not support assignment
	return ast.NewOptionalGetAttr(tok, obj, name)
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
