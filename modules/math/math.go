package math

import (
	"context"
	"fmt"
	"math"

	"github.com/risor-io/risor/object"
)

func Abs(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.abs: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		v := arg.Value()
		if v < 0 {
			v *= -1
		}
		return object.NewInt(v), nil
	case *object.Float:
		v := arg.Value()
		if v < 0 {
			v *= -1
		}
		return object.NewFloat(v), nil
	default:
		return nil, fmt.Errorf("type error: argument to math.abs not supported, got=%s", args[0].Type())
	}
}

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

func Sqrt(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sqrt: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		v := arg.Value()
		return object.NewFloat(math.Sqrt(float64(v))), nil
	case *object.Float:
		v := arg.Value()
		return object.NewFloat(math.Sqrt(v)), nil
	default:
		return nil, fmt.Errorf("type error: argument to math.sqrt not supported, got=%s", args[0].Type())
	}
}

func Max(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("math.max: expected 2 arguments, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Max(x, y)), nil
}

func Min(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("math.min: expected 2 arguments, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Min(x, y)), nil
}

func Sum(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sum: expected 1 argument, got %d", len(args))
	}
	arg := args[0]
	var array []object.Object
	switch arg := arg.(type) {
	case *object.List:
		array = arg.Value()
	default:
		return nil, fmt.Errorf("type error: %s object is not iterable", arg.Type())
	}
	if len(array) == 0 {
		return object.NewFloat(0), nil
	}
	var sum float64
	for _, value := range array {
		switch val := value.(type) {
		case *object.Int:
			sum += float64(val.Value())
		case *object.Float:
			sum += val.Value()
		default:
			return nil, fmt.Errorf("value error: invalid input for math.sum: %s", val.Type())
		}
	}
	return object.NewFloat(sum), nil
}

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
		return nil, fmt.Errorf("type error: argument to math.ceil not supported, got=%s", args[0].Type())
	}
}

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
		return nil, fmt.Errorf("type error: argument to math.floor not supported, got=%s", args[0].Type())
	}
}

func Sin(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.sin: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		return object.NewFloat(math.Sin(float64(arg.Value()))), nil
	case *object.Float:
		return object.NewFloat(math.Sin(arg.Value())), nil
	default:
		return nil, fmt.Errorf("type error: argument to math.sin not supported, got=%s", args[0].Type())
	}
}

func Cos(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.cos: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Int:
		return object.NewFloat(math.Cos(float64(arg.Value()))), nil
	case *object.Float:
		return object.NewFloat(math.Cos(arg.Value())), nil
	default:
		return nil, fmt.Errorf("type error: argument to math.cos not supported, got=%s", args[0].Type())
	}
}

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

func Mod(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("math.mod: expected 2 arguments, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Mod(x, y)), nil
}

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

func Pow10(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.pow10: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Pow10(int(x))), nil
}

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

func Round(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("math.round: expected 1 argument, got %d", len(args))
	}
	x, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(math.Round(x)), nil
}

func Inf(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("math.inf: expected 0-1 arguments, got %d", len(args))
	}
	sign := 1
	if len(args) == 1 {
		arg, err := object.AsInt(args[0])
		if err != nil {
			return nil, err
		}
		sign = int(arg)
	}
	return object.NewFloat(math.Inf(sign)), nil
}

func Module() *object.Module {
	return object.NewBuiltinsModule("math", map[string]object.Object{
		"abs":    object.NewBuiltin("abs", Abs),
		"atan2":  object.NewBuiltin("atan2", Atan2),
		"ceil":   object.NewBuiltin("ceil", Ceil),
		"cos":    object.NewBuiltin("cos", Cos),
		"E":      object.NewFloat(math.E),
		"floor":  object.NewBuiltin("floor", Floor),
		"inf":    object.NewBuiltin("inf", Inf),
		"is_inf": object.NewBuiltin("is_inf", IsInf),
		"log":    object.NewBuiltin("log", Log),
		"log10":  object.NewBuiltin("log10", Log10),
		"log2":   object.NewBuiltin("log2", Log2),
		"max":    object.NewBuiltin("max", Max),
		"min":    object.NewBuiltin("min", Min),
		"mod":    object.NewBuiltin("mod", Mod),
		"PI":     object.NewFloat(math.Pi),
		"pow":    object.NewBuiltin("pow", Pow),
		"pow10":  object.NewBuiltin("pow10", Pow10),
		"round":  object.NewBuiltin("round", Round),
		"sin":    object.NewBuiltin("sin", Sin),
		"sqrt":   object.NewBuiltin("sqrt", Sqrt),
		"sum":    object.NewBuiltin("sum", Sum),
		"tan":    object.NewBuiltin("tan", Tan),
	})
}
