package main

// errorPatterns contains common error patterns and their fixes.
var errorPatterns = []ErrorPattern{
	{
		Type:           "type_error",
		MessagePattern: "type error: expected X, got Y",
		Causes: []string{
			"Passing wrong type to function",
			"Method called on incompatible type",
			"Arithmetic operation on non-numeric types",
		},
		Examples: []ErrorExample{
			{
				Error:       "type error: expected string, got int",
				BadCode:     `"hello".split(123)`,
				Fix:         `"hello".split(",")`,
				Explanation: "split() requires a string separator, not an integer",
			},
			{
				Error:       "type error: expected int|float, got string",
				BadCode:     `"5" + 3`,
				Fix:         `int("5") + 3`,
				Explanation: "Use int() or float() to convert strings to numbers",
			},
			{
				Error:       "type error: len() requires string, list, or map",
				BadCode:     `len(123)`,
				Fix:         `len(string(123)) or len([1, 2, 3])`,
				Explanation: "len() works on containers (string, list, map), not numbers",
			},
		},
	},
	{
		Type:           "name_error",
		MessagePattern: "name error: X is not defined",
		Causes: []string{
			"Typo in variable or function name",
			"Variable used before declaration",
			"Builtin or module not available (restricted environment)",
			"Variable out of scope",
		},
		Examples: []ErrorExample{
			{
				Error:       "name error: 'maths' is not defined",
				BadCode:     `maths.sqrt(4)`,
				Fix:         `math.sqrt(4)`,
				Explanation: "Module name is 'math', not 'maths'",
			},
			{
				Error:       "name error: 'x' is not defined",
				BadCode:     `let y = x + 1`,
				Fix:         `let x = 10; let y = x + 1`,
				Explanation: "Declare variables before using them",
			},
			{
				Error:       "name error: 'print' is not defined",
				BadCode:     `print("hello")`,
				Fix:         `// print is not a builtin in Risor v2`,
				Explanation: "Use encode(\"json\", value) or return values directly",
			},
		},
	},
	{
		Type:           "syntax_error",
		MessagePattern: "syntax error at line N",
		Causes: []string{
			"Missing closing bracket, brace, or parenthesis",
			"Invalid token or character",
			"Incomplete statement",
			"Missing comma in list or map",
		},
		Examples: []ErrorExample{
			{
				Error:       "syntax error: expected '}'",
				BadCode:     `if x { print(x)`,
				Fix:         `if x { print(x) }`,
				Explanation: "Block must be closed with '}'",
			},
			{
				Error:       "syntax error: expected ')'",
				BadCode:     `foo(1, 2`,
				Fix:         `foo(1, 2)`,
				Explanation: "Function calls must be closed with ')'",
			},
			{
				Error:       "syntax error: unexpected ','",
				BadCode:     `[1, 2, 3,]`,
				Fix:         `[1, 2, 3]`,
				Explanation: "Trailing commas are not allowed in lists",
			},
		},
	},
	{
		Type:           "index_error",
		MessagePattern: "index out of bounds",
		Causes: []string{
			"Accessing list element beyond its length",
			"Negative index without proper handling",
			"Off-by-one error in loop bounds",
		},
		Examples: []ErrorExample{
			{
				Error:       "index error: index 5 out of bounds for list of length 3",
				BadCode:     `let items = [1, 2, 3]; items[5]`,
				Fix:         `let items = [1, 2, 3]; items[2]`,
				Explanation: "List indices are 0-based; length 3 means valid indices are 0, 1, 2",
			},
		},
	},
	{
		Type:           "key_error",
		MessagePattern: "key not found",
		Causes: []string{
			"Accessing non-existent map key",
			"Typo in key name",
		},
		Examples: []ErrorExample{
			{
				Error:       "key error: key 'naem' not found",
				BadCode:     `let user = {name: "Alice"}; user.naem`,
				Fix:         `let user = {name: "Alice"}; user.name`,
				Explanation: "Check for typos in key names",
			},
			{
				Error:       "key error: key 'age' not found",
				BadCode:     `let user = {name: "Alice"}; user.age`,
				Fix:         `let user = {name: "Alice"}; user.age ?? 0`,
				Explanation: "Use ?? (nil coalescing) to provide a default for missing keys",
			},
		},
	},
	{
		Type:           "argument_error",
		MessagePattern: "wrong number of arguments",
		Causes: []string{
			"Too few or too many arguments to function",
			"Forgot required argument",
			"Passing arguments in wrong order",
		},
		Examples: []ErrorExample{
			{
				Error:       "argument error: range() requires 1-3 arguments, got 0",
				BadCode:     `range()`,
				Fix:         `range(10)`,
				Explanation: "range() requires at least a stop value",
			},
			{
				Error:       "argument error: filter() requires 2 arguments, got 1",
				BadCode:     `filter([1, 2, 3])`,
				Fix:         `filter([1, 2, 3], x => x > 1)`,
				Explanation: "filter() requires both a list and a predicate function",
			},
		},
	},
}
