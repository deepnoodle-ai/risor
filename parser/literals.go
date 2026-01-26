package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/risor-io/risor/ast"
	"github.com/risor-io/risor/internal/tmpl"
	"github.com/risor-io/risor/internal/token"
)

// Literal parsing methods for the Parser.
// This file contains methods that parse literal values and compound literals:
// - Numeric literals (int, float)
// - Boolean and nil literals
// - String literals (including template strings)
// - List literals
// - Map literals
// - Function literals
// - Spread expressions
// - Reserved keyword handling

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

func (p *Parser) parseReserved() ast.Node {
	p.setTokenError(p.curToken, "reserved keyword: %s", p.curToken.Literal)
	return nil
}

func (p *Parser) parseReservedInfix(_ ast.Node) ast.Node {
	p.setTokenError(p.curToken, "reserved operator: %s", p.curToken.Literal)
	return nil
}
