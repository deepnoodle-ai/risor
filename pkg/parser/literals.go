package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v2/internal/tmpl"
	"github.com/deepnoodle-ai/risor/v2/internal/token"
	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
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
			Closing:  strToken.EndPosition,
			Literal:  strToken.Literal,
			Value:    strToken.Literal,
		}, true
	}
	// TEMPLATE (backticks) - check for ${expr} interpolation
	if !strings.Contains(strToken.Literal, "${") {
		return &ast.String{
			ValuePos: strToken.StartPosition,
			Closing:  strToken.EndPosition,
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
	// The template text starts just after the opening backtick. Fragment
	// offsets count runes in that text.
	textRunes := []rune(strToken.Literal)
	textStart := strToken.StartPosition.Advance(1)
	var exprs []ast.Expr
	for _, e := range tmpl.Fragments() {
		if !e.IsVariable() {
			continue
		}
		// The nested parse gets only the depth budget that remains here
		remaining := p.maxDepth - p.depth
		if remaining < 1 {
			p.setTokenError(strToken, "maximum nesting depth exceeded")
			return nil, false
		}
		// Parse the expression in place in the source, so that its AST
		// positions and errors refer to the source and not to the fragment.
		start := advancePosition(textStart, textRunes[:e.Offset()])
		end := start.Char + utf8.RuneCountInString(e.Value())
		sub := New(p.l.Sub(start, end), &Config{
			Filename: p.l.Filename(),
			MaxDepth: remaining,
		})
		tmplAst, err := sub.Parse(p.ctx)
		if err != nil {
			p.addInterpolationError(strToken, err)
			return nil, false
		}
		statements := tmplAst.Stmts
		if len(statements) == 0 {
			p.setTokenError(strToken, "empty template interpolation")
			return nil, false
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
		Closing:  strToken.EndPosition,
		Literal:  strToken.Literal,
		Value:    strToken.Literal,
		Template: tmpl,
		Exprs:    exprs,
	}, true
}

// advancePosition returns the position just after the given runes, which
// start at pos in the source.
func advancePosition(pos token.Position, runes []rune) token.Position {
	for _, r := range runes {
		if r == '\n' {
			pos = token.Position{
				Char:      pos.Char + 1,
				LineStart: pos.Char + 1,
				Line:      pos.Line + 1,
				Column:    0,
				File:      pos.File,
			}
		} else {
			pos = pos.Advance(1)
		}
	}
	return pos
}

// addInterpolationError reports the error from parsing an interpolation of
// the template string strToken. Its position is already a source position.
// Later errors in the same interpolation are usually caused by the first, so
// only the first is reported.
func (p *Parser) addInterpolationError(strToken token.Token, err error) {
	errs, ok := err.(*Errors)
	if !ok || errs.Count() == 0 {
		p.setTokenError(strToken, "in template interpolation: %s", err.Error())
		return
	}
	e := errs.First()
	p.setError(NewParserError(ErrorOpts{
		ErrType:       e.Type(),
		Message:       "in template interpolation: " + e.Message(),
		File:          e.File(),
		StartPosition: e.StartPosition(),
		EndPosition:   e.EndPosition(),
		SourceCode:    e.SourceCode(),
	}))
}

func (p *Parser) parseList() (ast.Node, bool) {
	lbrack := p.curToken.StartPosition
	allowDefaults := p.destructureCandidate
	p.destructureCandidate = false
	items := p.parseExprList(token.RBRACKET, allowDefaults)
	if items == nil {
		return nil, false
	}
	rbrack := p.curToken.StartPosition
	return &ast.List{Lbrack: lbrack, Items: items, Rbrack: rbrack}, true
}

// parseExprList parses a comma-separated list of expressions until the end token.
// It wraps parseNodeList and ensures all items are expressions.
// Special case: if allowDefaults is set, Assign nodes with simple ident LHS are
// converted to DefaultValue to support array destructuring with defaults in
// arrow functions: ([a = 1]) => ...
func (p *Parser) parseExprList(end token.Type, allowDefaults bool) []ast.Expr {
	nodes := p.parseNodeList(end)
	if nodes == nil {
		return nil
	}
	exprs := make([]ast.Expr, len(nodes))
	for i, node := range nodes {
		// First try direct expression
		if expr, ok := node.(ast.Expr); ok {
			exprs[i] = expr
			continue
		}
		// Handle Assign with simple ident LHS - convert to DefaultValue
		// This supports array destructuring defaults: [a = 1]
		if assign, ok := node.(*ast.Assign); ok && assign.Name != nil && assign.Op == "=" {
			if !allowDefaults {
				p.setDefaultValueError(assign.Name)
				return nil
			}
			p.noteCandidateDefault(assign.Name)
			exprs[i] = &ast.DefaultValue{
				Name:    assign.Name,
				Default: assign.Value,
			}
			continue
		}
		p.setTokenError(p.curToken, "expected expression in list")
		return nil
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
	node := p.parseListElement()
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
		node = p.parseListElement()
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

func (p *Parser) parseMap() (ast.Node, bool) {
	lbrace := p.curToken.StartPosition
	allowDefaults := p.destructureCandidate
	p.destructureCandidate = false
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
	item := p.parseMapItem(allowDefaults)
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

		item := p.parseMapItem(allowDefaults)
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

// parseMapItem parses a single map item: spread (...obj), shorthand (a),
// key-value (a: b), or a default (a = expr), which is allowed only in an arrow
// function destructuring parameter.
func (p *Parser) parseMapItem(allowDefaults bool) *ast.MapItem {
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
	// Also handles a default, {a = 1}, in an arrow function destructuring
	// parameter
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

		// Default in an arrow function destructuring parameter: {a = expr}
		if p.peekTokenIs(token.ASSIGN) {
			ident := p.newIdent(p.curToken)
			if !allowDefaults {
				p.setDefaultValueError(ident)
				return nil
			}
			p.noteCandidateDefault(ident)
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
	// Alias with a default in an arrow function destructuring parameter:
	// {a: b = expr}
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
		alias := p.newIdent(p.curToken)
		if !allowDefaults {
			p.setDefaultValueError(alias)
			return nil
		}
		p.noteCandidateDefault(alias)
		p.nextToken() // move to '='
		p.nextToken() // move to the default expression
		defaultExpr := p.parseExpression(LOWEST)
		if defaultExpr == nil {
			return nil
		}
		return &ast.MapItem{Key: key, Value: &ast.DefaultValue{Name: alias, Default: defaultExpr}}
	}
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
			if !p.endListItem(token.RPAREN, "function parameters") {
				return nil, nil, nil
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
			if !p.endListItem(token.RPAREN, "function parameters") {
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
			p.eatNewlines()
			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil, nil, nil
			}
			defaults[ident.String()] = expr
			p.nextToken()
		}
		if !p.endListItem(token.RPAREN, "function parameters") {
			return nil, nil, nil
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

		if !p.endListItem(token.RBRACE, "destructuring pattern") {
			return nil
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

		if !p.endListItem(token.RBRACKET, "array destructuring pattern") {
			return nil
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

// parseListElement parses one element of a list literal or call argument
// list, where a spread expression is allowed.
func (p *Parser) parseListElement() ast.Node {
	if p.curTokenIs(token.SPREAD) {
		node, ok := p.parseSpread()
		if !ok {
			return nil
		}
		return node
	}
	return p.parseNode(LOWEST)
}

// parseMisplacedSpread is the prefix function for '...'. Contexts that
// allow spread parse it directly, so reaching this is always an error.
func (p *Parser) parseMisplacedSpread() (ast.Node, bool) {
	p.setTokenError(p.curToken, "spread operator (...) is only allowed in list literals, map literals, and call arguments")
	return nil, false
}

// endListItem moves past the comma that follows an item of a function
// parameter list or destructuring pattern, along with any newlines around it.
// The current token must be a comma or the closing token; otherwise it reports
// an error and returns false. At the end of the input it returns true, so the
// caller reports the unterminated list.
func (p *Parser) endListItem(end token.Type, context string) bool {
	p.eatNewlines()
	switch p.curToken.Type {
	case token.COMMA:
		p.nextToken()
		p.eatNewlines()
		return true
	case end, token.EOF:
		return true
	}
	p.setTokenError(p.curToken, "expected ',' or '%s' in %s (got %s)", end, context, tokenDescription(p.curToken))
	return false
}

// noteCandidateDefault records a default parsed in a destructuring candidate.
// parseGroupedExpr reports it if the group is not arrow function parameters.
func (p *Parser) noteCandidateDefault(name *ast.Ident) {
	if p.candidateDefault == nil {
		p.candidateDefault = name
	}
}

// setDefaultValueError reports a `name = value` default where only arrow
// function destructuring parameters allow one.
func (p *Parser) setDefaultValueError(name *ast.Ident) {
	tok := token.Token{
		Type:          token.IDENT,
		Literal:       name.Name,
		StartPosition: name.NamePos,
		EndPosition:   name.NamePos.Advance(utf8.RuneCountInString(name.Name) - 1),
	}
	p.setTokenError(tok, "default value for %q is only allowed in arrow function destructuring parameters", name.Name)
}

func (p *Parser) parseNewline() (ast.Node, bool) {
	p.nextToken()
	return nil, true
}
