package parser

import (
	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/token"
)

// Statement parsing methods for the Parser.
// This file contains methods that parse statement constructs:
// - Variable declarations (let, const)
// - Destructuring patterns
// - Return, throw statements
// - Assignment statements
// - Postfix operators (x++, x--)
// - Try/catch/finally

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
	p.eatNewlines()

	bindings := []ast.DestructureBinding{}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.cancelled() {
			return nil
		}
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
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
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // Move to ','
			p.nextToken() // Move past ','
			p.eatNewlines()
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
	p.eatNewlines()

	elements := []ast.ArrayDestructureElement{}

	for !p.curTokenIs(token.RBRACKET) && !p.curTokenIs(token.EOF) {
		if p.cancelled() {
			return nil
		}
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
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
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // Move to ','
			p.nextToken() // Move past ','
			p.eatNewlines()
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

// parseAssignmentValue parses the right hand side of an assignment statement.
func (p *Parser) parseAssignmentValue() ast.Expr {
	// Save the assignment token (=) before eatNewlines potentially changes prevToken
	assignToken := p.prevToken
	p.eatNewlines()
	result := p.parseExpression(LOWEST)
	if result == nil {
		// Only add error if none was added during parsing
		if !p.hadNewError() {
			p.setError(NewParserError(ErrorOpts{
				ErrType:       "parse error",
				Message:       "assignment is missing a value",
				File:          p.l.Filename(),
				StartPosition: assignToken.StartPosition,
				EndPosition:   assignToken.EndPosition,
				SourceCode:    p.l.GetLineText(assignToken),
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

func (p *Parser) parseAssign(name ast.Node) (ast.Node, bool) {
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
		return nil, false
	}
	p.nextToken() // move to the RHS value
	p.eatNewlines()
	right := p.parseExpression(LOWEST)
	if right == nil {
		p.setTokenError(p.curToken, "invalid assignment statement value")
		return nil, false
	}
	if index != nil {
		return &ast.Assign{Name: nil, Index: index, OpPos: opPos, Op: op, Value: right}, true
	}
	return &ast.Assign{Name: ident, Index: nil, OpPos: opPos, Op: op, Value: right}, true
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

func (p *Parser) parseTry() (ast.Node, bool) {
	tryPos := p.curToken.StartPosition

	// Expect opening brace for try block
	if !p.expectPeek("try statement", token.LBRACE) {
		return nil, false
	}

	// Parse try block
	tryBlock := p.parseBlock()
	if tryBlock == nil {
		return nil, false
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
			return nil, false
		}

		catchBlock = p.parseBlock()
		if catchBlock == nil {
			return nil, false
		}
	}

	// Check for finally (allow newlines before it)
	if p.skipNewlinesAndPeek(token.FINALLY) {
		p.nextToken() // move to "finally"
		finallyPos = p.curToken.StartPosition

		// Expect opening brace for finally block
		if !p.expectPeek("finally block", token.LBRACE) {
			return nil, false
		}

		finallyBlock = p.parseBlock()
		if finallyBlock == nil {
			return nil, false
		}
	}

	// Require at least one of catch or finally
	if catchBlock == nil && finallyBlock == nil {
		p.setTokenError(p.curToken, "try statement requires at least one of catch or finally")
		return nil, false
	}

	return &ast.Try{
		Try:          tryPos,
		Body:         tryBlock,
		Catch:        catchPos,
		CatchIdent:   catchIdent,
		CatchBlock:   catchBlock,
		Finally:      finallyPos,
		FinallyBlock: finallyBlock,
	}, true
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
