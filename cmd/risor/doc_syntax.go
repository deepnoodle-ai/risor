package main

// syntaxSections contains the complete syntax reference for Risor.
var syntaxSections = []SyntaxSection{
	{
		Name: "literals",
		Items: []SyntaxItem{
			{Syntax: "42", Type: "int", Notes: "Integer literal"},
			{Syntax: "3.14", Type: "float", Notes: "Float literal"},
			{Syntax: `"hello"`, Type: "string", Notes: "String literal with escapes"},
			{Syntax: "`raw`", Type: "string", Notes: "Raw string (no escapes)"},
			{Syntax: "true, false", Type: "bool", Notes: "Boolean literals"},
			{Syntax: "null", Type: "null", Notes: "Null value"},
		},
	},
	{
		Name: "collections",
		Items: []SyntaxItem{
			{Syntax: "[a, b, c]", Type: "list", Notes: "List literal"},
			{Syntax: "{k: v}", Type: "map", Notes: "Map literal"},
			{Syntax: "{k}", Type: "map", Notes: "Shorthand for {k: k}"},
			{Syntax: "[...a, ...b]", Type: "list", Notes: "List spread"},
			{Syntax: "{...a, ...b}", Type: "map", Notes: "Map spread"},
		},
	},
	{
		Name: "variables",
		Items: []SyntaxItem{
			{Syntax: "let x = value", Notes: "Mutable variable"},
			{Syntax: "const X = value", Notes: "Immutable constant"},
			{Syntax: "let {a, b} = obj", Notes: "Destructuring assignment"},
			{Syntax: "let [x, y] = list", Notes: "List destructuring"},
		},
	},
	{
		Name: "functions",
		Items: []SyntaxItem{
			{Syntax: "function name(a, b) { body }", Notes: "Named function"},
			{Syntax: "let f = function(a) { body }", Notes: "Anonymous function"},
			{Syntax: "x => expr", Notes: "Arrow function (single param)"},
			{Syntax: "(a, b) => expr", Notes: "Arrow function (multiple params)"},
			{Syntax: "(a, b) => { stmts }", Notes: "Arrow function with block"},
		},
	},
	{
		Name: "control_flow",
		Items: []SyntaxItem{
			{Syntax: "if (cond) { } else { }", Notes: "Conditional (is an expression)"},
			{Syntax: "switch (val) { case x: ... }", Notes: "Switch statement"},
			{Syntax: "try { } catch e { }", Notes: "Error handling"},
			{Syntax: "throw error(msg)", Notes: "Raise an error"},
			{Syntax: "return value", Notes: "Return from function"},
		},
	},
	{
		Name: "operators",
		Items: []SyntaxItem{
			{Syntax: "+ - * / %", Notes: "Arithmetic"},
			{Syntax: "== != < > <= >=", Notes: "Comparison"},
			{Syntax: "&& || !", Notes: "Logical"},
			{Syntax: "?? ?.", Notes: "Nil coalescing, optional chain"},
			{Syntax: "|", Notes: "Pipe operator"},
		},
	},
	{
		Name: "method_chaining",
		Items: []SyntaxItem{
			{Syntax: "obj.method()", Notes: "Method call"},
			{Syntax: "list.map(f).filter(g)", Notes: "Chained methods"},
			{Syntax: "obj?.method()", Notes: "Optional chaining"},
		},
	},
}

// syntaxQuickRef contains the 10 most essential syntax patterns.
var syntaxQuickRef = []SyntaxPattern{
	{Pattern: "let x = 1", Description: "Variable declaration"},
	{Pattern: "const PI = 3.14", Description: "Constant declaration"},
	{Pattern: "x => x * 2", Description: "Arrow function"},
	{Pattern: "function name(a, b) { }", Description: "Named function"},
	{Pattern: "[1, 2, 3]", Description: "List literal"},
	{Pattern: "{key: value}", Description: "Map literal"},
	{Pattern: "obj.method()", Description: "Method call"},
	{Pattern: "list.map(fn).filter(fn)", Description: "Method chaining"},
	{Pattern: "let {a, b} = obj", Description: "Destructuring"},
	{Pattern: "{...a, ...b}", Description: "Spread operator"},
}
