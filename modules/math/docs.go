package math

import "github.com/deepnoodle-ai/risor/v2/object"

// Docs returns documentation for the math module.
func Docs() []object.FuncSpec {
	return mathDocs
}

// ModuleDoc returns the module-level documentation.
func ModuleDoc() string {
	return "Mathematical functions and constants"
}

var mathDocs = []object.FuncSpec{
	// Constants
	{Name: "pi", Doc: "Pi (3.14159...)", Returns: "float"},
	{Name: "e", Doc: "Euler's number (2.718...)", Returns: "float"},
	{Name: "tau", Doc: "Tau (2*pi)", Returns: "float"},
	{Name: "inf", Doc: "Positive infinity", Returns: "float"},
	{Name: "nan", Doc: "Not a number", Returns: "float"},
	// Basic math
	{Name: "abs", Doc: "Absolute value", Args: []string{"x"}, Returns: "int|float"},
	{Name: "sign", Doc: "Sign of x (-1, 0, or 1)", Args: []string{"x"}, Returns: "int"},
	{Name: "floor", Doc: "Floor (round down)", Args: []string{"x"}, Returns: "float"},
	{Name: "ceil", Doc: "Ceiling (round up)", Args: []string{"x"}, Returns: "float"},
	{Name: "round", Doc: "Round to nearest integer", Args: []string{"x"}, Returns: "float"},
	{Name: "trunc", Doc: "Truncate toward zero", Args: []string{"x"}, Returns: "float"},
	{Name: "min", Doc: "Minimum of values", Args: []string{"x..."}, Returns: "float"},
	{Name: "max", Doc: "Maximum of values", Args: []string{"x..."}, Returns: "float"},
	{Name: "clamp", Doc: "Clamp x to [min, max]", Args: []string{"x", "min", "max"}, Returns: "float"},
	{Name: "sum", Doc: "Sum of list elements", Args: []string{"items"}, Returns: "float"},
	// Powers and roots
	{Name: "sqrt", Doc: "Square root", Args: []string{"x"}, Returns: "float"},
	{Name: "cbrt", Doc: "Cube root", Args: []string{"x"}, Returns: "float"},
	{Name: "pow", Doc: "Power (x^y)", Args: []string{"x", "y"}, Returns: "float"},
	{Name: "exp", Doc: "e^x", Args: []string{"x"}, Returns: "float"},
	// Logarithms
	{Name: "log", Doc: "Natural logarithm", Args: []string{"x"}, Returns: "float"},
	{Name: "log10", Doc: "Base-10 logarithm", Args: []string{"x"}, Returns: "float"},
	{Name: "log2", Doc: "Base-2 logarithm", Args: []string{"x"}, Returns: "float"},
	// Trigonometry
	{Name: "sin", Doc: "Sine", Args: []string{"x"}, Returns: "float"},
	{Name: "cos", Doc: "Cosine", Args: []string{"x"}, Returns: "float"},
	{Name: "tan", Doc: "Tangent", Args: []string{"x"}, Returns: "float"},
	{Name: "asin", Doc: "Arc sine", Args: []string{"x"}, Returns: "float"},
	{Name: "acos", Doc: "Arc cosine", Args: []string{"x"}, Returns: "float"},
	{Name: "atan", Doc: "Arc tangent", Args: []string{"x"}, Returns: "float"},
	{Name: "atan2", Doc: "Arc tangent of y/x", Args: []string{"y", "x"}, Returns: "float"},
	{Name: "hypot", Doc: "Euclidean distance", Args: []string{"x", "y"}, Returns: "float"},
	// Hyperbolic
	{Name: "sinh", Doc: "Hyperbolic sine", Args: []string{"x"}, Returns: "float"},
	{Name: "cosh", Doc: "Hyperbolic cosine", Args: []string{"x"}, Returns: "float"},
	{Name: "tanh", Doc: "Hyperbolic tangent", Args: []string{"x"}, Returns: "float"},
	// Conversions
	{Name: "degrees", Doc: "Radians to degrees", Args: []string{"x"}, Returns: "float"},
	{Name: "radians", Doc: "Degrees to radians", Args: []string{"x"}, Returns: "float"},
	// Checks
	{Name: "is_inf", Doc: "Check if value is infinity", Args: []string{"x"}, Returns: "bool"},
	{Name: "is_finite", Doc: "Check if value is finite", Args: []string{"x"}, Returns: "bool"},
	{Name: "is_nan", Doc: "Check if value is NaN", Args: []string{"x"}, Returns: "bool"},
}
