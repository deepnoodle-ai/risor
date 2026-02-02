package math

import (
	"context"
	"fmt"
	"math"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

// Abs returns the absolute value of x.
func Abs(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.abs: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		v := arg.Value()
		if v < 0 {
			v = -v
		}
		return object.NewInt(v), nil
	case *object.Float:
		return object.NewFloat(math.Abs(arg.Value())), nil
	default:
		return nil, object.TypeErrorf("math.abs: expected number, got %s", args[0].Type())
	}
}

// Sign returns -1 for negative, 0 for zero, or 1 for positive numbers.
func Sign(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sign: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		v := arg.Value()
		if v < 0 {
			return object.NewInt(-1), nil
		} else if v > 0 {
			return object.NewInt(1), nil
		}
		return object.NewInt(0), nil
	case *object.Float:
		v := arg.Value()
		if math.IsNaN(v) {
			return object.NewFloat(math.NaN()), nil
		}
		if v < 0 {
			return object.NewFloat(-1), nil
		} else if v > 0 {
			return object.NewFloat(1), nil
		}
		return object.NewFloat(0), nil
	default:
		return nil, object.TypeErrorf("math.sign: expected number, got %s", args[0].Type())
	}
}

// Ceil returns the smallest integer greater than or equal to x.
func Ceil(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.ceil: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		return arg, nil
	case *object.Float:
		return object.NewFloat(math.Ceil(arg.Value())), nil
	default:
		return nil, object.TypeErrorf("math.ceil: expected number, got %s", args[0].Type())
	}
}

// Floor returns the largest integer less than or equal to x.
func Floor(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.floor: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		return arg, nil
	case *object.Float:
		return object.NewFloat(math.Floor(arg.Value())), nil
	default:
		return nil, object.TypeErrorf("math.floor: expected number, got %s", args[0].Type())
	}
}

// Round returns the nearest integer, rounding half away from zero.
func Round(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.round: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		return arg, nil
	case *object.Float:
		return object.NewFloat(math.Round(arg.Value())), nil
	default:
		return nil, object.TypeErrorf("math.round: expected number, got %s", args[0].Type())
	}
}

// Trunc returns the integer part of x, truncating toward zero.
func Trunc(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.trunc: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		return arg, nil
	case *object.Float:
		return object.NewFloat(math.Trunc(arg.Value())), nil
	default:
		return nil, object.TypeErrorf("math.trunc: expected number, got %s", args[0].Type())
	}
}

// Clamp constrains x to the range [lo, hi].
func Clamp(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("math.clamp: expected 3 arguments, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	lo, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	hi, err := object.AsFloat(args[2])
	if err != nil {
		return nil, err
	}
	if lo > hi {
		return nil, fmt.Errorf("math.clamp: lo (%v) must be <= hi (%v)", lo, hi)
	}
	result := x
	if result < lo {
		result = lo
	}
	if result > hi {
		result = hi
	}
	return object.NewFloat(result), nil
}

// Min returns the smallest of its arguments.
// Can be called with multiple arguments or a single list.
func Min(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("math.min: expected at least 1 argument")
	}
	// If single list argument, extract values from it
	values := args
	if len(args) == 1 {
		if list, ok := args[0].(*object.List); ok {
			values = list.Value()
			if len(values) == 0 {
				return nil, fmt.Errorf("math.min: empty list")
			}
		}
	}
	minVal, err := object.AsFloat(values[0])
	if err != nil {
		return nil, err
	}
	for _, arg := range values[1:] {
		v, err := object.AsFloat(arg)
		if err != nil {
			return nil, err
		}
		if v < minVal {
			minVal = v
		}
	}
	return object.NewFloat(minVal), nil
}

// Max returns the largest of its arguments.
// Can be called with multiple arguments or a single list.
func Max(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("math.max: expected at least 1 argument")
	}
	// If single list argument, extract values from it
	values := args
	if len(args) == 1 {
		if list, ok := args[0].(*object.List); ok {
			values = list.Value()
			if len(values) == 0 {
				return nil, fmt.Errorf("math.max: empty list")
			}
		}
	}
	maxVal, err := object.AsFloat(values[0])
	if err != nil {
		return nil, err
	}
	for _, arg := range values[1:] {
		v, err := object.AsFloat(arg)
		if err != nil {
			return nil, err
		}
		if v > maxVal {
			maxVal = v
		}
	}
	return object.NewFloat(maxVal), nil
}

// Sum returns the sum of all numbers in a list.
func Sum(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sum: expected 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*object.List)
	if !ok {
		return nil, object.TypeErrorf("math.sum: expected list, got %s", args[0].Type())
	}
	values := list.Value()
	if len(values) == 0 {
		return object.NewFloat(0), nil
	}
	var sum float64
	for _, value := range values {
		v, err := object.AsFloat(value)
		if err != nil {
			return nil, err
		}
		sum += v
	}
	return object.NewFloat(sum), nil
}

// Sqrt returns the square root of x.
func Sqrt(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sqrt: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Sqrt(x)), nil
}

// Cbrt returns the cube root of x.
func Cbrt(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.cbrt: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Cbrt(x)), nil
}

// Pow returns x raised to the power y.
func Pow(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("math.pow: expected 2 arguments, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Pow(x, y)), nil
}

// Exp returns e raised to the power x.
func Exp(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.exp: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Exp(x)), nil
}

// Log returns the natural logarithm of x.
func Log(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.log: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Log(x)), nil
}

// Log10 returns the base-10 logarithm of x.
func Log10(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.log10: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Log10(x)), nil
}

// Log2 returns the base-2 logarithm of x.
func Log2(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.log2: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Log2(x)), nil
}

// Sin returns the sine of x (in radians).
func Sin(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sin: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Sin(x)), nil
}

// Cos returns the cosine of x (in radians).
func Cos(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.cos: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Cos(x)), nil
}

// Tan returns the tangent of x (in radians).
func Tan(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.tan: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Tan(x)), nil
}

// Asin returns the arcsine of x (in radians).
func Asin(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.asin: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Asin(x)), nil
}

// Acos returns the arccosine of x (in radians).
func Acos(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.acos: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Acos(x)), nil
}

// Atan returns the arctangent of x (in radians).
func Atan(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.atan: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Atan(x)), nil
}

// Atan2 returns the arctangent of y/x, using the signs to determine the quadrant.
func Atan2(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("math.atan2: expected 2 arguments, got %d", len(args))
	}
	y, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	x, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Atan2(y, x)), nil
}

// Hypot returns the Euclidean distance sqrt(x*x + y*y).
func Hypot(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("math.hypot: expected 2 arguments, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Hypot(x, y)), nil
}

// Sinh returns the hyperbolic sine of x.
func Sinh(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sinh: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Sinh(x)), nil
}

// Cosh returns the hyperbolic cosine of x.
func Cosh(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.cosh: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Cosh(x)), nil
}

// Tanh returns the hyperbolic tangent of x.
func Tanh(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.tanh: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Tanh(x)), nil
}

// Degrees converts radians to degrees.
func Degrees(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.degrees: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(x * 180 / math.Pi), nil
}

// Radians converts degrees to radians.
func Radians(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.radians: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(x * math.Pi / 180), nil
}

// IsFinite returns true if x is neither infinite nor NaN.
func IsFinite(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.is_finite: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewBool(!math.IsInf(x, 0) && !math.IsNaN(x)), nil
}

// IsInf returns true if x is positive or negative infinity.
func IsInf(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.is_inf: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewBool(math.IsInf(x, 0)), nil
}

// IsNaN returns true if x is NaN (not a number).
func IsNaN(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.is_nan: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewBool(math.IsNaN(x)), nil
}

func Module() *object.Module {
	return object.NewBuiltinsModule("math", map[string]object.Object{
		// Constants (lowercase)
		"pi":  object.NewFloat(math.Pi),
		"e":   object.NewFloat(math.E),
		"tau": object.NewFloat(2 * math.Pi),
		"inf": object.NewFloat(math.Inf(1)),
		"nan": object.NewFloat(math.NaN()),

		// Basic operations
		"abs":   object.NewBuiltin("abs", Abs),
		"sign":  object.NewBuiltin("sign", Sign),
		"ceil":  object.NewBuiltin("ceil", Ceil),
		"floor": object.NewBuiltin("floor", Floor),
		"round": object.NewBuiltin("round", Round),
		"trunc": object.NewBuiltin("trunc", Trunc),
		"clamp": object.NewBuiltin("clamp", Clamp),

		// Min/max/sum
		"min": object.NewBuiltin("min", Min),
		"max": object.NewBuiltin("max", Max),
		"sum": object.NewBuiltin("sum", Sum),

		// Powers and logarithms
		"sqrt":  object.NewBuiltin("sqrt", Sqrt),
		"cbrt":  object.NewBuiltin("cbrt", Cbrt),
		"pow":   object.NewBuiltin("pow", Pow),
		"exp":   object.NewBuiltin("exp", Exp),
		"log":   object.NewBuiltin("log", Log),
		"log10": object.NewBuiltin("log10", Log10),
		"log2":  object.NewBuiltin("log2", Log2),

		// Trigonometry
		"sin":   object.NewBuiltin("sin", Sin),
		"cos":   object.NewBuiltin("cos", Cos),
		"tan":   object.NewBuiltin("tan", Tan),
		"asin":  object.NewBuiltin("asin", Asin),
		"acos":  object.NewBuiltin("acos", Acos),
		"atan":  object.NewBuiltin("atan", Atan),
		"atan2": object.NewBuiltin("atan2", Atan2),
		"hypot": object.NewBuiltin("hypot", Hypot),

		// Hyperbolic
		"sinh": object.NewBuiltin("sinh", Sinh),
		"cosh": object.NewBuiltin("cosh", Cosh),
		"tanh": object.NewBuiltin("tanh", Tanh),

		// Angle conversion
		"degrees": object.NewBuiltin("degrees", Degrees),
		"radians": object.NewBuiltin("radians", Radians),

		// Predicates
		"is_finite": object.NewBuiltin("is_finite", IsFinite),
		"is_inf":    object.NewBuiltin("is_inf", IsInf),
		"is_nan":    object.NewBuiltin("is_nan", IsNaN),
	})
}
