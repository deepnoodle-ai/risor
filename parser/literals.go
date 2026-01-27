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

func (p *Parser) parseInt() (ast.Node, bool) {
	tok, lit := p.curToken, p.curToken.Literal
	var value int64
	var err error
	if strings.HasPrefix(lit, "0x") {
		value, err = strconv.ParseInt(lit[2:], 16, 64) // hexadecimal
	} else if strings.HasPrefix(lit, "0b") {
		value, err = strconv.ParseInt(lit[2:], 2, 64) // binary
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
		return nil, false
	}
	return &ast.Int{ValuePos: tok.StartPosition, Literal: lit, Value: value}, true
}

func (p *Parser) parseFloat() (ast.Node, bool) {
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
		return nil, false
	}
	return &ast.Float{ValuePos: tok.StartPosition, Literal: lit, Value: value}, true
}

func (p *Parser) parseBoolean() (ast.Node, bool) {
	return &ast.Bool{
		ValuePos: p.curToken.StartPosition,
		Literal:  p.curToken.Literal,
		Value:    p.curTokenIs(token.TRUE),
	}, true
}

func (p *Parser) parseNil() (ast.Node, bool) {
	return &ast.Nil{NilPos: p.curToken.StartPosition}, true
}

func (p *Parser) parseString() (ast.Node, bool) {
	strToken := p.curToken
	// STRING (single or double quotes) - plain strings, no interpolation
	if strToken.Type == token.STRING {
		return &ast.String{
			ValuePos: strToken.StartPosition,
			Literal:  strToken.Literal,
			Value:    strToken.Literal,
		}, true
	}
	// TEMPLATE (backticks) - check for ${expr} interpolation
	if !strings.Contains(strToken.Literal, "${") {
		return &ast.String{
			ValuePos: strToken.StartPosition,
			Literal:  strToken.Literal,
			Value:    strToken.Literal,
		}, true
	}
	// Template string with ${expr} interpolation
	tmpl, err := tmpl.Parse(strToken.Literal)
	if err != nil {
		p.setTokenError(strToken, "%s", err.Error())
		return nil, false
	}
	var exprs []ast.Expr
	for _, e := range tmpl.Fragments() {
		if !e.IsVariable() {
			continue
		}
		tmplAst, err := Parse(p.ctx, e.Value(), &Config{Filename: p.l.Filename()})
		if err != nil {
			p.setTokenError(strToken, "in template interpolation: %s", err.Error())
			return nil, false
		}
		statements := tmplAst.Stmts
		if len(statements) == 0 {
			exprs = append(exprs, nil)
		} else if len(statements) > 1 {
			p.setTokenError(strToken, "template contains more than one expression")
			return nil, false
		} else {
			stmt := statements[0]
			expr, ok := stmt.(ast.Expr)
			if !ok {
				p.setTokenError(strToken, "template contains an unexpected statement type")
				return nil, false
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
	}, true
}

func (p *Parser) parseList() (ast.Node, bool) {
	lbrack := p.curToken.StartPosition
	items := p.parseExprList(token.RBRACKET)
	if items == nil {
		return nil, false
	}
	rbrack := p.curToken.StartPosition
	return &ast.List{Lbrack: lbrack, Items: items, Rbrack: rbrack}, true
}

// parseExprList parses a comma-separated list of expressions until the end token.
// It wraps parseNodeList and ensures all items are expressions.
func (p *Parser) parseExprList(end token.Type) []ast.Expr {
	nodes := p.parseNodeList(end)
	if nodes == nil {
		return nil
	}
	exprs := make([]ast.Expr, len(nodes))
	for i, node := range nodes {
		expr, ok := node.(ast.Expr)
		if !ok {
			p.setTokenError(p.curToken, "expected expression in list")
			return nil
		}
		exprs[i] = expr
	}
	return exprs
}

// parseNodeList parses a comma-separated list of nodes until the end token.
// Supports trailing commas and newlines between elements.
func (p *Parser) parseNodeList(end token.Type) []ast.Node {
	list := make([]ast.Node, 0)
	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}
	// Skip leading newlines
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	// Allow empty lists/calls with only newlines before the closing token
	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}
	p.nextToken()
	node := p.parseNode(LOWEST)
	if node == nil {
		if !p.hadNewError() {
			p.setTokenError(p.curToken, "invalid syntax in list")
		}
		return nil
	}
	list = append(list, node)
	for p.peekTokenIs(token.COMMA) {
		// Move to the comma
		if err := p.nextToken(); err != nil {
			return nil
		}
		// Skip newlines after comma
		for p.peekTokenIs(token.NEWLINE) {
			if err := p.nextToken(); err != nil {
				return nil
			}
		}
		// Check for trailing comma (list ended after newlines)
		if p.peekTokenIs(end) {
			break
		}
		// Move to the next element
		if err := p.nextToken(); err != nil {
			return nil
		}
		node = p.parseNode(LOWEST)
		if node == nil {
			return nil
		}
		list = append(list, node)
	}
	// Skip trailing newlines
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil
		}
	}
	if !p.expectPeek("list", end) {
		return nil
	}
	return list
}

func (p *Parser) parseMapOrSet() (ast.Node, bool) {
	lbrace := p.curToken.StartPosition
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil, false
		}
	}
	// Empty {} turns into an empty map (not a set)
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		rbrace := p.curToken.StartPosition
		return &ast.Map{Lbrace: lbrace, Items: nil, Rbrace: rbrace}, true
	}
	p.nextToken() // move to the first key or spread

	items := []ast.MapItem{}

	// Parse first item (could be spread or key-value)
	item := p.parseMapItem()
	if item == nil {
		return nil, false
	}
	items = append(items, *item)

	// Parse remaining items
	for !p.peekTokenIs(token.RBRACE) {
		if p.cancelled() {
			return nil, false
		}
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
			break
		}
		if !p.expectPeek("map", token.COMMA) {
			return nil, false
		}
		for p.peekTokenIs(token.NEWLINE) {
			if err := p.nextToken(); err != nil {
				return nil, false
			}
		}
		if p.peekTokenIs(token.RBRACE) {
			break
		}
		p.nextToken() // move to the key or spread

		item := p.parseMapItem()
		if item == nil {
			return nil, false
		}
		items = append(items, *item)

		if !p.peekTokenIs(token.COMMA) {
			break
		}
	}
	for p.peekTokenIs(token.NEWLINE) {
		if err := p.nextToken(); err != nil {
			return nil, false
		}
	}
	if !p.expectPeek("map", token.RBRACE) {
		return nil, false
	}
	rbrace := p.curToken.StartPosition
	return &ast.Map{Lbrace: lbrace, Items: items, Rbrace: rbrace}, true
}

// parseMapItem parses a single map item: spread (...obj), shorthand (a), shorthand with default (a = expr), or key-value (a: b).
func (p *Parser) parseMapItem() *ast.MapItem {
	// Check for spread expression
	if p.curTokenIs(token.SPREAD) {
		spreadNode, ok := p.parseSpread()
		if !ok {
			return nil
		}
		spread, ok := spreadNode.(ast.Expr)
		if !ok {
			p.setTokenError(p.curToken, "invalid spread expression")
			return nil
		}
		return &ast.MapItem{Key: nil, Value: spread}
	}

	// Check for shorthand syntax: {a, b} means {a: a, b: b}
	// Also handles shorthand with default: {a = 1} for destructuring
	if p.curTokenIs(token.IDENT) {
		// Simple shorthand: {a} or {a, b}
		if p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.RBRACE) || p.peekTokenIs(token.NEWLINE) {
			ident := p.newIdent(p.curToken)
			key := &ast.String{
				ValuePos: ident.Pos(),
				Literal:  ident.Name,
				Value:    ident.Name,
			}
			return &ast.MapItem{Key: key, Value: ident}
		}

		// Shorthand with default: {a = expr} - used in destructuring
		if p.peekTokenIs(token.ASSIGN) {
			ident := p.newIdent(p.curToken)
			key := &ast.String{
				ValuePos: ident.Pos(),
				Literal:  ident.Name,
				Value:    ident.Name,
			}
			p.nextToken() // move to '='
			p.nextToken() // move to the default expression
			defaultExpr := p.parseExpression(LOWEST)
			if defaultExpr == nil {
				return nil
			}
			// Store as DefaultValue so convertMapToDestructureParam can extract the default
			value := &ast.DefaultValue{
				Name:    ident,
				Default: defaultExpr,
			}
			return &ast.MapItem{Key: key, Value: value}
		}
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

func (p *Parser) parseFunc() (ast.Node, bool) {
	funcPos := p.curToken.StartPosition
	var ident *ast.Ident
	if p.peekTokenIs(token.IDENT) { // Read optional function name
		p.nextToken()
		ident = p.newIdent(p.curToken)
	}
	if !p.expectPeek("function", token.LPAREN) { // Move to the "("
		return nil, false
	}
	lparen := p.curToken.StartPosition
	defaults, params, restParam := p.parseFuncParams()
	if defaults == nil { // parseFuncParams encountered an error
		return nil, false
	}
	rparen := p.curToken.StartPosition
	if !p.expectPeek("function", token.LBRACE) { // move to the "{"
		return nil, false
	}
	body := p.parseBlock()
	if body == nil {
		return nil, false
	}
	return &ast.Func{
		Func:      funcPos,
		Name:      ident,
		Lparen:    lparen,
		Params:    params,
		Defaults:  defaults,
		RestParam: restParam,
		Rparen:    rparen,
		Body:      body,
	}, true
}

func (p *Parser) parseFuncParams() (map[string]ast.Expr, []ast.FuncParam, *ast.Ident) {
	// If the next parameter is ")", then there are no parameters
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return map[string]ast.Expr{}, nil, nil
	}
	defaults := map[string]ast.Expr{}
	params := make([]ast.FuncParam, 0)
	var restParam *ast.Ident
	p.nextToken()
	p.eatNewlines()
	for !p.curTokenIs(token.RPAREN) { // Keep going until we find a ")"
		if p.cancelled() {
			return nil, nil, nil
		}
		for p.curTokenIs(token.NEWLINE) {
			if err := p.nextToken(); err != nil {
				return nil, nil, nil
			}
		}
		// After eating newlines, check if we reached the closing paren
		if p.curTokenIs(token.RPAREN) {
			break
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
			p.eatNewlines()
			// Rest parameter must be last
			if !p.curTokenIs(token.RPAREN) {
				p.setTokenError(p.curToken, "rest parameter must be the last parameter")
				return nil, nil, nil
			}
			continue
		}

		// Check for object destructuring parameter: {a, b}
		if p.curTokenIs(token.LBRACE) {
			param := p.parseObjectDestructureParam()
			if param == nil {
				return nil, nil, nil
			}
			params = append(params, param)
			if p.curTokenIs(token.COMMA) {
				p.nextToken()
				p.eatNewlines()
			}
			continue
		}

		// Check for array destructuring parameter: [a, b]
		if p.curTokenIs(token.LBRACKET) {
			param := p.parseArrayDestructureParam()
			if param == nil {
				return nil, nil, nil
			}
			params = append(params, param)
			if p.curTokenIs(token.COMMA) {
				p.nextToken()
				p.eatNewlines()
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
			p.eatNewlines()
			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil, nil, nil
			}
			defaults[ident.String()] = expr
			p.nextToken()
		}
		if p.curTokenIs(token.COMMA) {
			p.nextToken()
			p.eatNewlines()
		}
	}
	return defaults, params, restParam
}

// parseObjectDestructureParam parses an object destructuring parameter: {a, b, c: alias = default}
func (p *Parser) parseObjectDestructureParam() *ast.ObjectDestructureParam {
	lbrace := p.curToken.StartPosition
	bindings := []ast.DestructureBinding{}

	p.nextToken() // move past '{'
	p.eatNewlines()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.cancelled() {
			return nil
		}
		p.eatNewlines()

		// After eating newlines, check if we reached the closing brace
		if p.curTokenIs(token.RBRACE) {
			break
		}

		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected identifier in destructuring pattern")
			return nil
		}

		binding := ast.DestructureBinding{Key: p.curToken.Literal}
		p.nextToken()

		// Check for alias: {key: alias}
		if p.curTokenIs(token.COLON) {
			p.nextToken() // move past ':'
			if !p.curTokenIs(token.IDENT) {
				p.setTokenError(p.curToken, "expected identifier after ':' in destructuring pattern")
				return nil
			}
			binding.Alias = p.curToken.Literal
			p.nextToken()
		}

		// Check for default value: {key = value} or {key: alias = value}
		if p.curTokenIs(token.ASSIGN) {
			p.nextToken() // move past '='
			p.eatNewlines()
			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil
			}
			binding.Default = expr
			p.nextToken()
		}

		bindings = append(bindings, binding)

		if p.curTokenIs(token.COMMA) {
			p.nextToken()
			p.eatNewlines()
		}
	}

	if !p.curTokenIs(token.RBRACE) {
		p.setTokenError(p.curToken, "expected '}' in destructuring pattern")
		return nil
	}
	rbrace := p.curToken.StartPosition
	p.nextToken() // move past '}'

	return &ast.ObjectDestructureParam{
		Lbrace:   lbrace,
		Bindings: bindings,
		Rbrace:   rbrace,
	}
}

// parseArrayDestructureParam parses an array destructuring parameter: [a, b = default]
func (p *Parser) parseArrayDestructureParam() *ast.ArrayDestructureParam {
	lbrack := p.curToken.StartPosition
	elements := []ast.ArrayDestructureElement{}

	p.nextToken() // move past '['
	p.eatNewlines()

	for !p.curTokenIs(token.RBRACKET) && !p.curTokenIs(token.EOF) {
		if p.cancelled() {
			return nil
		}
		p.eatNewlines()

		// After eating newlines, check if we reached the closing bracket
		if p.curTokenIs(token.RBRACKET) {
			break
		}

		if !p.curTokenIs(token.IDENT) {
			p.setTokenError(p.curToken, "expected identifier in array destructuring pattern")
			return nil
		}

		elem := ast.ArrayDestructureElement{Name: p.newIdent(p.curToken)}
		p.nextToken()

		// Check for default value: [a = value]
		if p.curTokenIs(token.ASSIGN) {
			p.nextToken() // move past '='
			p.eatNewlines()
			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil
			}
			elem.Default = expr
			p.nextToken()
		}

		elements = append(elements, elem)

		if p.curTokenIs(token.COMMA) {
			p.nextToken()
			p.eatNewlines()
		}
	}

	if !p.curTokenIs(token.RBRACKET) {
		p.setTokenError(p.curToken, "expected ']' in array destructuring pattern")
		return nil
	}
	rbrack := p.curToken.StartPosition
	p.nextToken() // move past ']'

	return &ast.ArrayDestructureParam{
		Lbrack:   lbrack,
		Elements: elements,
		Rbrack:   rbrack,
	}
}

func (p *Parser) parseSpread() (ast.Node, bool) {
	ellipsis := p.curToken.StartPosition
	if err := p.nextToken(); err != nil {
		return nil, false
	}
	// Parse the expression to be spread
	value := p.parseExpression(PREFIX)
	if value == nil {
		p.setTokenError(p.curToken, "expected expression after spread operator")
		return nil, false
	}
	return &ast.Spread{Ellipsis: ellipsis, X: value}, true
}

func (p *Parser) parseNewline() (ast.Node, bool) {
	p.nextToken()
	return nil, true
}

func (p *Parser) parseReserved() (ast.Node, bool) {
	p.setTokenError(p.curToken, "reserved keyword: %s", p.curToken.Literal)
	return nil, false
}

func (p *Parser) parseReservedInfix(_ ast.Node) (ast.Node, bool) {
	p.setTokenError(p.curToken, "reserved operator: %s", p.curToken.Literal)
	return nil, false
}
