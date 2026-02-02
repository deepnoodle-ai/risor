package math

import (
	"context"
	"math"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/object"
)

func TestAbs(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected object.Object
	}{
		{"positive float", object.NewFloat(3.5), object.NewFloat(3.5)},
		{"negative float", object.NewFloat(-3.5), object.NewFloat(3.5)},
		{"zero float", object.NewFloat(0), object.NewFloat(0)},
		{"positive int", object.NewInt(5), object.NewInt(5)},
		{"negative int", object.NewInt(-5), object.NewInt(5)},
		{"zero int", object.NewInt(0), object.NewInt(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Abs(ctx, tt.input)
			assert.Nil(t, err)
			assert.True(t, object.Equals(result, tt.expected), "got %s, want %s", result.Inspect(), tt.expected.Inspect())
		})
	}
}

func TestAbsErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Abs(ctx)
	assert.NotNil(t, err)

	_, err = Abs(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestSign(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected int64
	}{
		{"positive int", object.NewInt(5), 1},
		{"negative int", object.NewInt(-5), -1},
		{"zero int", object.NewInt(0), 0},
		{"positive float", object.NewFloat(3.5), 1},
		{"negative float", object.NewFloat(-3.5), -1},
		{"zero float", object.NewFloat(0.0), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Sign(ctx, tt.input)
			assert.Nil(t, err)
			switch r := result.(type) {
			case *object.Int:
				assert.Equal(t, r.Value(), tt.expected)
			case *object.Float:
				assert.Equal(t, int64(r.Value()), tt.expected)
			}
		})
	}
}

func TestSignNaN(t *testing.T) {
	ctx := context.Background()
	result, err := Sign(ctx, object.NewFloat(math.NaN()))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsNaN(f.Value()))
}

func TestSignErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Sign(ctx)
	assert.NotNil(t, err)

	_, err = Sign(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestCeil(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected object.Object
	}{
		{"positive float", object.NewFloat(3.2), object.NewFloat(4.0)},
		{"negative float", object.NewFloat(-3.2), object.NewFloat(-3.0)},
		{"whole float", object.NewFloat(3.0), object.NewFloat(3.0)},
		{"int", object.NewInt(5), object.NewInt(5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Ceil(ctx, tt.input)
			assert.Nil(t, err)
			assert.True(t, object.Equals(result, tt.expected))
		})
	}
}

func TestCeilErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Ceil(ctx)
	assert.NotNil(t, err)

	_, err = Ceil(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestFloor(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected object.Object
	}{
		{"positive float", object.NewFloat(3.7), object.NewFloat(3.0)},
		{"negative float", object.NewFloat(-3.2), object.NewFloat(-4.0)},
		{"whole float", object.NewFloat(3.0), object.NewFloat(3.0)},
		{"int", object.NewInt(5), object.NewInt(5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Floor(ctx, tt.input)
			assert.Nil(t, err)
			assert.True(t, object.Equals(result, tt.expected))
		})
	}
}

func TestFloorErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Floor(ctx)
	assert.NotNil(t, err)

	_, err = Floor(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestRound(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"round down", object.NewFloat(3.4), 3.0},
		{"round up", object.NewFloat(3.5), 4.0},
		{"negative round", object.NewFloat(-3.5), -4.0},
		{"whole number", object.NewFloat(5.0), 5.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Round(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestRoundErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Round(ctx)
	assert.NotNil(t, err)

	_, err = Round(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestTrunc(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"positive", object.NewFloat(3.7), 3.0},
		{"negative", object.NewFloat(-3.7), -3.0},
		{"whole", object.NewFloat(5.0), 5.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Trunc(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestTruncErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Trunc(ctx)
	assert.NotNil(t, err)

	_, err = Trunc(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestClamp(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		x, lo, hi float64
		expected  float64
	}{
		{"in range", 5, 0, 10, 5},
		{"below range", -5, 0, 10, 0},
		{"above range", 15, 0, 10, 10},
		{"at lower bound", 0, 0, 10, 0},
		{"at upper bound", 10, 0, 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Clamp(ctx, object.NewFloat(tt.x), object.NewFloat(tt.lo), object.NewFloat(tt.hi))
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestClampErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Clamp(ctx, object.NewFloat(5))
	assert.NotNil(t, err)

	// lo > hi
	_, err = Clamp(ctx, object.NewFloat(5), object.NewFloat(10), object.NewFloat(0))
	assert.NotNil(t, err)

	_, err = Clamp(ctx, object.NewString("x"), object.NewFloat(0), object.NewFloat(10))
	assert.NotNil(t, err)
}

func TestMin(t *testing.T) {
	ctx := context.Background()

	// Two arguments
	result, err := Min(ctx, object.NewFloat(3.5), object.NewFloat(2.1))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 2.1)

	// Multiple arguments (variadic)
	result, err = Min(ctx, object.NewFloat(5), object.NewFloat(2), object.NewFloat(8), object.NewFloat(1))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 1.0)

	// Single list argument
	result, err = Min(ctx, object.NewList([]object.Object{
		object.NewFloat(5), object.NewFloat(2), object.NewFloat(8),
	}))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 2.0)

	// Mixed int/float
	result, err = Min(ctx, object.NewInt(3), object.NewFloat(2.5))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 2.5)
}

func TestMinErrors(t *testing.T) {
	ctx := context.Background()
	// No arguments
	_, err := Min(ctx)
	assert.NotNil(t, err)

	// Empty list
	_, err = Min(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Min(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.NotNil(t, err)
}

func TestMax(t *testing.T) {
	ctx := context.Background()

	// Two arguments
	result, err := Max(ctx, object.NewFloat(3.5), object.NewFloat(2.1))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 3.5)

	// Multiple arguments (variadic)
	result, err = Max(ctx, object.NewFloat(5), object.NewFloat(2), object.NewFloat(8), object.NewFloat(1))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 8.0)

	// Single list argument
	result, err = Max(ctx, object.NewList([]object.Object{
		object.NewFloat(5), object.NewFloat(2), object.NewFloat(8),
	}))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 8.0)
}

func TestMaxErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Max(ctx)
	assert.NotNil(t, err)

	_, err = Max(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)
}

func TestSum(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{
			"ints",
			object.NewList([]object.Object{
				object.NewInt(1), object.NewInt(2), object.NewInt(3),
			}),
			6.0,
		},
		{
			"floats",
			object.NewList([]object.Object{
				object.NewFloat(1.5), object.NewFloat(2.5), object.NewFloat(3.0),
			}),
			7.0,
		},
		{
			"empty list",
			object.NewList([]object.Object{}),
			0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Sum(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestSumErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Sum(ctx)
	assert.NotNil(t, err)

	_, err = Sum(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestSqrt(t *testing.T) {
	ctx := context.Background()
	result, err := Sqrt(ctx, object.NewFloat(16.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 4.0)
}

func TestSqrtErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Sqrt(ctx)
	assert.NotNil(t, err)

	_, err = Sqrt(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestCbrt(t *testing.T) {
	ctx := context.Background()
	result, err := Cbrt(ctx, object.NewFloat(27.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 3.0)

	result, err = Cbrt(ctx, object.NewFloat(-8.0))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), -2.0)
}

func TestCbrtErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Cbrt(ctx)
	assert.NotNil(t, err)
}

func TestPow(t *testing.T) {
	ctx := context.Background()
	result, err := Pow(ctx, object.NewFloat(2.0), object.NewFloat(3.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 8.0)
}

func TestPowErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Pow(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)
}

func TestExp(t *testing.T) {
	ctx := context.Background()
	result, err := Exp(ctx, object.NewFloat(1.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.Abs(f.Value()-math.E) < 1e-10)

	result, err = Exp(ctx, object.NewFloat(0.0))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 1.0)
}

func TestExpErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Exp(ctx)
	assert.NotNil(t, err)
}

func TestLog(t *testing.T) {
	ctx := context.Background()
	result, err := Log(ctx, object.NewFloat(math.E))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.Abs(f.Value()-1.0) < 1e-10)
}

func TestLogErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Log(ctx)
	assert.NotNil(t, err)
}

func TestLog10(t *testing.T) {
	ctx := context.Background()
	result, err := Log10(ctx, object.NewFloat(100.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 2.0)
}

func TestLog2(t *testing.T) {
	ctx := context.Background()
	result, err := Log2(ctx, object.NewFloat(8.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 3.0)
}

func TestSin(t *testing.T) {
	ctx := context.Background()
	result, err := Sin(ctx, object.NewFloat(0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 0.0)

	result, err = Sin(ctx, object.NewFloat(math.Pi/2))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.Abs(f.Value()-1.0) < 1e-10)
}

func TestCos(t *testing.T) {
	ctx := context.Background()
	result, err := Cos(ctx, object.NewFloat(0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 1.0)
}

func TestTan(t *testing.T) {
	ctx := context.Background()
	result, err := Tan(ctx, object.NewFloat(0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 0.0)
}

func TestAsin(t *testing.T) {
	ctx := context.Background()
	result, err := Asin(ctx, object.NewFloat(1.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.Abs(f.Value()-math.Pi/2) < 1e-10)
}

func TestAcos(t *testing.T) {
	ctx := context.Background()
	result, err := Acos(ctx, object.NewFloat(1.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 0.0)
}

func TestAtan(t *testing.T) {
	ctx := context.Background()
	result, err := Atan(ctx, object.NewFloat(0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 0.0)
}

func TestAtan2(t *testing.T) {
	ctx := context.Background()
	result, err := Atan2(ctx, object.NewFloat(1.0), object.NewFloat(1.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.Abs(f.Value()-math.Pi/4) < 1e-10)
}

func TestAtan2Errors(t *testing.T) {
	ctx := context.Background()
	_, err := Atan2(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)
}

func TestHypot(t *testing.T) {
	ctx := context.Background()
	result, err := Hypot(ctx, object.NewFloat(3.0), object.NewFloat(4.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 5.0)
}

func TestHypotErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Hypot(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)
}

func TestSinh(t *testing.T) {
	ctx := context.Background()
	result, err := Sinh(ctx, object.NewFloat(0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 0.0)
}

func TestCosh(t *testing.T) {
	ctx := context.Background()
	result, err := Cosh(ctx, object.NewFloat(0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 1.0)
}

func TestTanh(t *testing.T) {
	ctx := context.Background()
	result, err := Tanh(ctx, object.NewFloat(0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 0.0)
}

func TestDegrees(t *testing.T) {
	ctx := context.Background()
	result, err := Degrees(ctx, object.NewFloat(math.Pi))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 180.0)

	result, err = Degrees(ctx, object.NewFloat(math.Pi/2))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), 90.0)
}

func TestDegreesErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Degrees(ctx)
	assert.NotNil(t, err)
}

func TestRadians(t *testing.T) {
	ctx := context.Background()
	result, err := Radians(ctx, object.NewFloat(180.0))
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.Abs(f.Value()-math.Pi) < 1e-10)

	result, err = Radians(ctx, object.NewFloat(90.0))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.Abs(f.Value()-math.Pi/2) < 1e-10)
}

func TestRadiansErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Radians(ctx)
	assert.NotNil(t, err)
}

func TestIsFinite(t *testing.T) {
	ctx := context.Background()

	result, err := IsFinite(ctx, object.NewFloat(1.0))
	assert.Nil(t, err)
	b, ok := result.(*object.Bool)
	assert.True(t, ok)
	assert.True(t, b.Value())

	result, err = IsFinite(ctx, object.NewFloat(math.Inf(1)))
	assert.Nil(t, err)
	b, ok = result.(*object.Bool)
	assert.True(t, ok)
	assert.False(t, b.Value())

	result, err = IsFinite(ctx, object.NewFloat(math.NaN()))
	assert.Nil(t, err)
	b, ok = result.(*object.Bool)
	assert.True(t, ok)
	assert.False(t, b.Value())
}

func TestIsFiniteErrors(t *testing.T) {
	ctx := context.Background()
	_, err := IsFinite(ctx)
	assert.NotNil(t, err)
}

func TestIsInf(t *testing.T) {
	ctx := context.Background()

	result, err := IsInf(ctx, object.NewFloat(math.Inf(1)))
	assert.Nil(t, err)
	b, ok := result.(*object.Bool)
	assert.True(t, ok)
	assert.True(t, b.Value())

	result, err = IsInf(ctx, object.NewFloat(1.0))
	assert.Nil(t, err)
	b, ok = result.(*object.Bool)
	assert.True(t, ok)
	assert.False(t, b.Value())
}

func TestIsInfErrors(t *testing.T) {
	ctx := context.Background()
	_, err := IsInf(ctx)
	assert.NotNil(t, err)
}

func TestIsNaN(t *testing.T) {
	ctx := context.Background()

	result, err := IsNaN(ctx, object.NewFloat(math.NaN()))
	assert.Nil(t, err)
	b, ok := result.(*object.Bool)
	assert.True(t, ok)
	assert.True(t, b.Value())

	result, err = IsNaN(ctx, object.NewFloat(1.0))
	assert.Nil(t, err)
	b, ok = result.(*object.Bool)
	assert.True(t, ok)
	assert.False(t, b.Value())
}

func TestIsNaNErrors(t *testing.T) {
	ctx := context.Background()
	_, err := IsNaN(ctx)
	assert.NotNil(t, err)
}

func TestModule(t *testing.T) {
	m := Module()
	assert.NotNil(t, m)
	assert.Equal(t, m.Name().Value(), "math")

	// Verify key functions exist
	functions := []string{
		"abs", "sign", "ceil", "floor", "round", "trunc", "clamp",
		"min", "max", "sum",
		"sqrt", "cbrt", "pow", "exp", "log", "log10", "log2",
		"sin", "cos", "tan", "asin", "acos", "atan", "atan2", "hypot",
		"sinh", "cosh", "tanh",
		"degrees", "radians",
		"is_finite", "is_inf", "is_nan",
	}
	for _, name := range functions {
		_, ok := m.GetAttr(name)
		assert.True(t, ok, "missing function: %s", name)
	}

	// Verify constants (lowercase)
	pi, ok := m.GetAttr("pi")
	assert.True(t, ok)
	piFloat, ok := pi.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, piFloat.Value(), math.Pi)

	e, ok := m.GetAttr("e")
	assert.True(t, ok)
	eFloat, ok := e.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, eFloat.Value(), math.E)

	tau, ok := m.GetAttr("tau")
	assert.True(t, ok)
	tauFloat, ok := tau.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, tauFloat.Value(), 2*math.Pi)

	inf, ok := m.GetAttr("inf")
	assert.True(t, ok)
	infFloat, ok := inf.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsInf(infFloat.Value(), 1))

	nan, ok := m.GetAttr("nan")
	assert.True(t, ok)
	nanFloat, ok := nan.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsNaN(nanFloat.Value()))
}
