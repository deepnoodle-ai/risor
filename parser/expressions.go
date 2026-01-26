package parser

import (
	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/token"
)

// Expression parsing methods for the Parser.
// This file contains methods that parse expression constructs:
// - Identifiers and prefix/infix expressions
// - Ternary expressions
// - Grouped expressions and arrow functions
// - Control flow expressions (if, switch)
// - Block parsing
// - Index/slice expressions
// - Call expressions and pipes
// - Membership (in, not in)
// - Attribute access (get/set)

func (p *Parser) parseIdent() (ast.Node, bool) {
	if p.curToken.Literal == "" {
		p.setTokenError(p.curToken, "invalid identifier")
		return nil, false
	}
	ident := p.newIdent(p.curToken)

	// Check for single-param arrow function: x => expr
	if p.peekTokenIs(token.ARROW) {
		arrowPos := p.curToken.StartPosition
		p.nextToken() // move to '=>'
		return p.parseArrowBody(arrowPos, []*ast.Ident{ident}, nil)
	}

	return ident, true
}

func (p *Parser) parsePrefixExpr() (ast.Node, bool) {
	opPos := p.curToken.StartPosition
	op := p.curToken.Literal
	if err := p.nextToken(); err != nil {
		return nil, false
	}
	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid prefix expression")
		return nil, false
	}
	return &ast.Prefix{OpPos: opPos, Op: op, X: right}, true
}

func (p *Parser) parseInfixExpr(leftNode ast.Node) (ast.Node, bool) {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid expression")
		return nil, false
	}
	opPos := p.curToken.StartPosition
	op := p.curToken.Literal
	precedence := p.currentPrecedence()
	p.nextToken()
	for p.curTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil, false
		}
	}
	right := p.parseExpression(precedence)
	if right == nil {
		p.setTokenError(p.curToken, "invalid expression")
		return nil, false
	}
	return &ast.Infix{X: left, OpPos: opPos, Op: op, Y: right}, true
}

func (p *Parser) parseTernary(conditionNode ast.Node) (ast.Node, bool) {
	condition, ok := conditionNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid ternary expression")
		return nil, false
	}
	if p.tern {
		p.setTokenError(p.curToken, "nested ternary expression detected")
		return nil, false
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
		return nil, false
	}
	// Allow newlines before the colon
	if !p.skipNewlinesAndPeek(token.COLON) {
		p.peekError("ternary expression", token.COLON, p.peekToken)
		return nil, false
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
		return nil, false
	}
	return &ast.Ternary{
		Cond:     condition,
		Question: questionPos,
		IfTrue:   ifTrue,
		Colon:    colonPos,
		IfFalse:  ifFalse,
	}, true
}

func (p *Parser) parseGroupedExpr() (ast.Node, bool) {
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
		return nil, false
	}

	// Parse first item - could be expression or arrow param with default
	// Use parseNode instead of parseExpression to allow Assign nodes for defaults
	firstItem := p.parseNode(LOWEST)
	if firstItem == nil {
		return nil, false
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
			return nil, false
		}
		items = append(items, item)
	}

	// Skip newlines before closing paren
	if !p.skipNewlinesAndPeek(token.RPAREN) {
		p.peekError("grouped expression or arrow function", token.RPAREN, p.peekToken)
		return nil, false
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
		return nil, false
	}

	// Ensure the single item is an expression
	expr, ok := firstItem.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "expected expression in grouped expression")
		return nil, false
	}

	return expr, true
}

// parseArrowParams validates items as arrow function parameters and parses the body
func (p *Parser) parseArrowParams(arrowPos token.Position, items []ast.Node) (ast.Node, bool) {
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
				return nil, false
			}
			params = append(params, v.Name)
			defaults[v.Name.Name] = v.Value
		default:
			p.setTokenError(p.curToken, "invalid arrow function parameter: expected identifier")
			return nil, false
		}
	}

	return p.parseArrowBody(arrowPos, params, defaults)
}

// parseArrowBody parses the body of an arrow function (expression or block)
func (p *Parser) parseArrowBody(arrowPos token.Position, params []*ast.Ident, defaults map[string]ast.Expr) (ast.Node, bool) {
	p.nextToken() // move past '=>'

	var body *ast.Block

	if p.curTokenIs(token.LBRACE) {
		// Block body: (x) => { ... }
		body = p.parseBlock()
		if body == nil {
			return nil, false
		}
	} else {
		// Expression body: (x) => x + 1
		// Wrap in implicit return
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			p.setTokenError(p.curToken, "invalid arrow function body")
			return nil, false
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
	}, true
}

// parseIf parses an entire if, else if, else block. Else-ifs are handled recursively.
func (p *Parser) parseIf() (ast.Node, bool) {
	ifPos := p.curToken.StartPosition
	if !p.expectPeek("an if expression", token.LPAREN) { // move to the "("
		return nil, false
	}
	lparen := p.curToken.StartPosition
	p.nextToken() // move past the "("
	cond := p.parseExpression(LOWEST)
	if cond == nil {
		return nil, false
	}
	if !p.expectPeek("an if expression", token.RPAREN) { // move to the ")"
		return nil, false
	}
	rparen := p.curToken.StartPosition
	if !p.expectPeek("an if expression", token.LBRACE) { // move to the "{"
		return nil, false
	}
	consequence := p.parseBlock()
	if consequence == nil {
		return nil, false
	}
	var alternative *ast.Block
	if p.peekTokenIs(token.ELSE) {
		p.nextToken()                // move to the "else"
		if p.peekTokenIs(token.IF) { // this is an "else if"
			p.nextToken() // move to the "if"
			nestedIfPos := p.curToken.StartPosition
			nestedIf, ok := p.parseIf()
			if !ok {
				return nil, false
			}
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
			}, true
		}
		if !p.expectPeek("an if expression", token.LBRACE) {
			return nil, false
		}
		alternative = p.parseBlock()
		if alternative == nil {
			return nil, false
		}
	}
	return &ast.If{
		If:          ifPos,
		Lparen:      lparen,
		Cond:        cond,
		Rparen:      rparen,
		Consequence: consequence,
		Alternative: alternative,
	}, true
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

func (p *Parser) parseSwitch() (ast.Node, bool) {
	switchPos := p.curToken.StartPosition
	if !p.expectPeek("switch statement", token.LPAREN) {
		return nil, false
	}
	lparen := p.curToken.StartPosition
	p.nextToken() // move past the "("
	switchValue := p.parseExpression(LOWEST)
	if switchValue == nil {
		return nil, false
	}
	if !p.expectPeek("switch statement", token.RPAREN) {
		return nil, false
	}
	rparen := p.curToken.StartPosition
	if !p.expectPeek("switch statement", token.LBRACE) {
		return nil, false
	}
	lbrace := p.curToken.StartPosition
	p.nextToken()
	p.eatNewlines()

	// Process the switch case statements
	var cases []*ast.Case
	var defaultCaseCount int

	for !p.curTokenIs(token.RBRACE) {
		if p.cancelled() {
			return nil, false
		}
		if p.curTokenIs(token.EOF) {
			p.setTokenError(p.prevToken, "unterminated switch statement")
			return nil, false
		}
		caseNode, isDefault := p.parseSwitchCase()
		if caseNode == nil {
			return nil, false
		}
		if isDefault {
			defaultCaseCount++
			if defaultCaseCount > 1 {
				p.setTokenError(p.curToken, "switch statement has multiple default blocks")
				return nil, false
			}
		}
		cases = append(cases, caseNode)
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
	}, true
}

// parseSwitchCase parses a single case or default clause in a switch statement.
// Returns the Case node and whether it's a default case.
func (p *Parser) parseSwitchCase() (*ast.Case, bool) {
	if p.curToken.Literal != "case" && p.curToken.Literal != "default" {
		p.setTokenError(p.curToken, "expected 'case' or 'default' (got %s)", p.curToken.Literal)
		return nil, false
	}

	casePos := p.curToken.StartPosition
	isDefault := p.curTokenIs(token.DEFAULT)
	var caseExprs []ast.Expr

	if !isDefault {
		// Parse case expressions (comma-separated)
		p.nextToken()
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			return nil, false
		}
		caseExprs = append(caseExprs, expr)
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // move to comma
			p.nextToken() // move to expression
			expr = p.parseExpression(LOWEST)
			if expr == nil {
				return nil, false
			}
			caseExprs = append(caseExprs, expr)
		}
	}

	if !p.expectPeek("switch case", token.COLON) {
		return nil, false
	}
	colonPos := p.curToken.StartPosition
	p.nextToken()
	p.eatNewlines()

	// Parse the case body
	body := p.parseCaseBody()
	if body == nil && p.hadNewError() {
		return nil, false
	}

	return &ast.Case{
		Case:    casePos,
		Exprs:   caseExprs,
		Colon:   colonPos,
		Body:    body,
		Default: isDefault,
	}, isDefault
}

// parseCaseBody parses the statements in a switch case until the next case/default/rbrace.
// Returns nil for empty case bodies (which are valid).
func (p *Parser) parseCaseBody() *ast.Block {
	// Empty case body is valid
	if p.curTokenIs(token.CASE) || p.curTokenIs(token.DEFAULT) || p.curTokenIs(token.RBRACE) {
		return nil
	}

	blockPos := p.curToken.StartPosition
	var statements []ast.Node

	for {
		if p.cancelled() {
			return nil
		}
		// Skip newlines and semicolons
		for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) {
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
		// End of case body?
		if p.curTokenIs(token.CASE) || p.curTokenIs(token.DEFAULT) ||
			p.curTokenIs(token.RBRACE) || p.curTokenIs(token.EOF) {
			break
		}
		// Parse one statement
		if s := p.parseStatement(); s != nil {
			statements = append(statements, s)
		}
		// Check for proper statement termination
		if !p.curTokenIs(token.SEMICOLON) && !statementTerminators[p.peekToken.Type] &&
			!p.peekTokenIs(token.CASE) && !p.peekTokenIs(token.DEFAULT) && !p.peekTokenIs(token.RBRACE) {
			p.peekError("case statement", token.SEMICOLON, p.peekToken)
			return nil
		}
		if err := p.nextToken(); err != nil {
			return nil
		}
	}

	// Case blocks use same position for both braces (no actual braces in source)
	return &ast.Block{Lbrace: blockPos, Stmts: statements, Rbrace: blockPos}
}

func (p *Parser) parseIndex(leftNode ast.Node) (ast.Node, bool) {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid index expression")
		return nil, false
	}
	lbrack := p.curToken.StartPosition
	var firstIndex, secondIndex ast.Expr
	if !p.peekTokenIs(token.COLON) {
		p.nextToken() // move to the first index
		firstIndex = p.parseExpression(LOWEST)
		if firstIndex == nil {
			return nil, false
		}
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // move to the "]"
			rbrack := p.curToken.StartPosition
			return &ast.Index{X: left, Lbrack: lbrack, Index: firstIndex, Rbrack: rbrack}, true
		}
	}
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // move to the ":"
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // move to the "]"
			rbrack := p.curToken.StartPosition
			return &ast.Slice{X: left, Lbrack: lbrack, Low: firstIndex, High: nil, Rbrack: rbrack}, true
		}
		p.nextToken() // move to the second index
		secondIndex = p.parseExpression(LOWEST)
		if secondIndex == nil {
			return nil, false
		}
	}
	if !p.expectPeek("an index expression", token.RBRACKET) {
		return nil, false
	}
	rbrack := p.curToken.StartPosition
	return &ast.Slice{X: left, Lbrack: lbrack, Low: firstIndex, High: secondIndex, Rbrack: rbrack}, true
}

func (p *Parser) parseCall(functionNode ast.Node) (ast.Node, bool) {
	function, ok := functionNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid call expression")
		return nil, false
	}
	lparen := p.curToken.StartPosition
	arguments := p.parseNodeList(token.RPAREN)
	if arguments == nil {
		return nil, false
	}
	rparen := p.curToken.StartPosition
	return &ast.Call{Fun: function, Lparen: lparen, Args: arguments, Rparen: rparen}, true
}

func (p *Parser) parsePipe(firstNode ast.Node) (ast.Node, bool) {
	first, ok := firstNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid pipe expression")
		return nil, false
	}
	exprs := []ast.Expr{first}
	for {
		// Move past the pipe operator itself
		if err := p.nextToken(); err != nil {
			return nil, false
		}
		// Advance across any extra newlines
		p.eatNewlines()
		// Parse the next expression and add it to the ast.Pipe Arguments
		expr := p.parseExpression(PIPE)
		if expr == nil {
			p.setTokenError(p.curToken, "invalid pipe expression")
			return nil, false
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
	return &ast.Pipe{Exprs: exprs}, true
}

func (p *Parser) parseIn(leftNode ast.Node) (ast.Node, bool) {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid in expression")
		return nil, false
	}
	inPos := p.curToken.StartPosition
	if err := p.nextToken(); err != nil {
		return nil, false
	}
	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid in expression")
		return nil, false
	}
	return &ast.In{X: left, InPos: inPos, Y: right}, true
}

func (p *Parser) parseNotIn(leftNode ast.Node) (ast.Node, bool) {
	left, ok := leftNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid not in expression")
		return nil, false
	}

	notInPos := p.curToken.StartPosition

	// Check if the next token is IN
	if !p.peekTokenIs(token.IN) {
		p.setTokenError(p.peekToken, "expected 'in' after 'not' (got %s)", p.peekToken.Literal)
		return nil, false
	}

	// Move to the IN token
	if err := p.nextToken(); err != nil {
		return nil, false
	}

	// Move past the IN token to parse the right operand
	if err := p.nextToken(); err != nil {
		return nil, false
	}

	right := p.parseExpression(PREFIX)
	if right == nil {
		p.setTokenError(p.curToken, "invalid not in expression")
		return nil, false
	}

	return &ast.NotIn{X: left, NotInPos: notInPos, Y: right}, true
}

func (p *Parser) parseGetAttr(objNode ast.Node) (ast.Node, bool) {
	obj, ok := objNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid attribute expression")
		return nil, false
	}
	period := p.curToken.StartPosition
	p.nextToken()
	p.eatNewlines()
	if !p.curTokenIs(token.IDENT) {
		p.setTokenError(p.curToken, "expected an identifier after %q", ".")
		return nil, false
	}
	name := p.newIdent(p.curToken)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		callNode, ok := p.parseCall(name)
		if !ok {
			return nil, false
		}
		call, ok := callNode.(*ast.Call)
		if !ok {
			p.setTokenError(p.curToken, "invalid attribute expression")
			return nil, false
		}
		return &ast.ObjectCall{X: obj, Period: period, Call: call, Optional: false}, true
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
			return nil, false
		}
		return &ast.SetAttr{X: obj, Period: period, Attr: name, OpPos: opPos, Op: opLiteral, Value: right}, true
	}
	return &ast.GetAttr{X: obj, Period: period, Attr: name, Optional: false}, true
}

func (p *Parser) parseOptionalChain(objNode ast.Node) (ast.Node, bool) {
	obj, ok := objNode.(ast.Expr)
	if !ok {
		p.setTokenError(p.curToken, "invalid optional chain expression")
		return nil, false
	}
	period := p.curToken.StartPosition
	p.nextToken()
	p.eatNewlines()
	if !p.curTokenIs(token.IDENT) {
		p.setTokenError(p.curToken, "expected an identifier after %q", "?.")
		return nil, false
	}
	name := p.newIdent(p.curToken)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		callNode, ok := p.parseCall(name)
		if !ok {
			return nil, false
		}
		call, ok := callNode.(*ast.Call)
		if !ok {
			p.setTokenError(p.curToken, "invalid optional chain expression")
			return nil, false
		}
		return &ast.ObjectCall{X: obj, Period: period, Call: call, Optional: true}, true
	}
	// Optional chaining does not support assignment
	return &ast.GetAttr{X: obj, Period: period, Attr: name, Optional: true}, true
}
