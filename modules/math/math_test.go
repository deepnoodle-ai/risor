package math

import (
	"context"
	"math"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
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
			result := Abs(ctx, tt.input)
			assert.True(t, object.Equals(result, tt.expected), "got %s, want %s", result.Inspect(), tt.expected.Inspect())
		})
	}
}

func TestAbsErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	result := Abs(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type
	result = Abs(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestSqrt(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"int 4", object.NewInt(4), 2.0},
		{"int 9", object.NewInt(9), 3.0},
		{"float 2", object.NewFloat(2.0), math.Sqrt(2.0)},
		{"float 16", object.NewFloat(16.0), 4.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sqrt(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestSqrtErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	result := Sqrt(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type
	result = Sqrt(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestMax(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		x    object.Object
		y    object.Object
		want float64
	}{
		{"both positive", object.NewFloat(3.5), object.NewFloat(2.1), 3.5},
		{"both negative", object.NewFloat(-5.0), object.NewFloat(-2.0), -2.0},
		{"mixed", object.NewFloat(-1.0), object.NewFloat(1.0), 1.0},
		{"ints", object.NewInt(5), object.NewInt(10), 10.0},
		{"mixed types", object.NewInt(3), object.NewFloat(2.5), 3.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Max(ctx, tt.x, tt.y)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestMaxErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	result := Max(ctx, object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	// Wrong type
	result = Max(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Max(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.True(t, object.IsError(result))
}

func TestMin(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		x    object.Object
		y    object.Object
		want float64
	}{
		{"both positive", object.NewFloat(3.5), object.NewFloat(2.1), 2.1},
		{"both negative", object.NewFloat(-5.0), object.NewFloat(-2.0), -5.0},
		{"mixed", object.NewFloat(-1.0), object.NewFloat(1.0), -1.0},
		{"ints", object.NewInt(5), object.NewInt(10), 5.0},
		{"mixed types", object.NewInt(3), object.NewFloat(2.5), 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Min(ctx, tt.x, tt.y)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestMinErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	result := Min(ctx, object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	// Wrong type
	result = Min(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Min(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.True(t, object.IsError(result))
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
				object.NewInt(1),
				object.NewInt(2),
				object.NewInt(3),
			}),
			6.0,
		},
		{
			"floats",
			object.NewList([]object.Object{
				object.NewFloat(1.5),
				object.NewFloat(2.5),
				object.NewFloat(3.0),
			}),
			7.0,
		},
		{
			"mixed",
			object.NewList([]object.Object{
				object.NewInt(1),
				object.NewFloat(2.5),
				object.NewInt(3),
			}),
			6.5,
		},
		{
			"empty list",
			object.NewList([]object.Object{}),
			0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sum(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestSumErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	result := Sum(ctx)
	assert.True(t, object.IsError(result))

	// Not a list
	result = Sum(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))

	// List with invalid types
	result = Sum(ctx, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewString("invalid"),
	}))
	assert.True(t, object.IsError(result))
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
			result := Ceil(ctx, tt.input)
			assert.True(t, object.Equals(result, tt.expected))
		})
	}
}

func TestCeilErrors(t *testing.T) {
	ctx := context.Background()
	result := Ceil(ctx)
	assert.True(t, object.IsError(result))

	result = Ceil(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
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
			result := Floor(ctx, tt.input)
			assert.True(t, object.Equals(result, tt.expected))
		})
	}
}

func TestFloorErrors(t *testing.T) {
	ctx := context.Background()
	result := Floor(ctx)
	assert.True(t, object.IsError(result))

	result = Floor(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestSin(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"zero", object.NewFloat(0), 0.0},
		{"pi/2", object.NewFloat(math.Pi / 2), 1.0},
		{"pi", object.NewFloat(math.Pi), 0.0},
		{"int zero", object.NewInt(0), 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sin(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestSinErrors(t *testing.T) {
	ctx := context.Background()
	result := Sin(ctx)
	assert.True(t, object.IsError(result))

	result = Sin(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestCos(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"zero", object.NewFloat(0), 1.0},
		{"pi/2", object.NewFloat(math.Pi / 2), 0.0},
		{"pi", object.NewFloat(math.Pi), -1.0},
		{"int zero", object.NewInt(0), 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Cos(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestCosErrors(t *testing.T) {
	ctx := context.Background()
	result := Cos(ctx)
	assert.True(t, object.IsError(result))

	result = Cos(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestTan(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"zero", object.NewFloat(0), 0.0},
		{"pi/4", object.NewFloat(math.Pi / 4), 1.0},
		{"int zero", object.NewInt(0), 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Tan(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestTanErrors(t *testing.T) {
	ctx := context.Background()
	result := Tan(ctx)
	assert.True(t, object.IsError(result))

	result = Tan(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestMod(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		x    object.Object
		y    object.Object
		want float64
	}{
		{"10 mod 3", object.NewFloat(10.0), object.NewFloat(3.0), 1.0},
		{"ints", object.NewInt(10), object.NewInt(3), 1.0},
		{"negative", object.NewFloat(-10.0), object.NewFloat(3.0), -1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Mod(ctx, tt.x, tt.y)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestModErrors(t *testing.T) {
	ctx := context.Background()
	result := Mod(ctx, object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Mod(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Mod(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.True(t, object.IsError(result))
}

func TestLog(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"e", object.NewFloat(math.E), 1.0},
		{"1", object.NewFloat(1.0), 0.0},
		{"int e^2", object.NewFloat(math.E * math.E), 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Log(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestLogErrors(t *testing.T) {
	ctx := context.Background()
	result := Log(ctx)
	assert.True(t, object.IsError(result))

	result = Log(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestLog10(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"10", object.NewFloat(10.0), 1.0},
		{"100", object.NewFloat(100.0), 2.0},
		{"1", object.NewFloat(1.0), 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Log10(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestLog10Errors(t *testing.T) {
	ctx := context.Background()
	result := Log10(ctx)
	assert.True(t, object.IsError(result))

	result = Log10(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestLog2(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"2", object.NewFloat(2.0), 1.0},
		{"8", object.NewFloat(8.0), 3.0},
		{"1", object.NewFloat(1.0), 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Log2(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestLog2Errors(t *testing.T) {
	ctx := context.Background()
	result := Log2(ctx)
	assert.True(t, object.IsError(result))

	result = Log2(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestPow(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		x    object.Object
		y    object.Object
		want float64
	}{
		{"2^3", object.NewFloat(2.0), object.NewFloat(3.0), 8.0},
		{"ints", object.NewInt(2), object.NewInt(3), 8.0},
		{"square root", object.NewFloat(4.0), object.NewFloat(0.5), 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Pow(ctx, tt.x, tt.y)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestPowErrors(t *testing.T) {
	ctx := context.Background()
	result := Pow(ctx, object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Pow(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Pow(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.True(t, object.IsError(result))
}

func TestPow10(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected float64
	}{
		{"10^2", object.NewFloat(2.0), 100.0},
		{"10^0", object.NewFloat(0.0), 1.0},
		{"10^3", object.NewInt(3), 1000.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Pow10(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestPow10Errors(t *testing.T) {
	ctx := context.Background()
	result := Pow10(ctx)
	assert.True(t, object.IsError(result))

	result = Pow10(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestAtan2(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		y    object.Object
		x    object.Object
		want float64
	}{
		{"1,1", object.NewFloat(1.0), object.NewFloat(1.0), math.Pi / 4},
		{"0,1", object.NewFloat(0.0), object.NewFloat(1.0), 0.0},
		{"1,0", object.NewFloat(1.0), object.NewFloat(0.0), math.Pi / 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Atan2(ctx, tt.y, tt.x)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.want) < 1e-10)
		})
	}
}

func TestAtan2Errors(t *testing.T) {
	ctx := context.Background()
	result := Atan2(ctx, object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Atan2(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.True(t, object.IsError(result))

	result = Atan2(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.True(t, object.IsError(result))
}

func TestIsInf(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected bool
	}{
		{"positive inf", object.NewFloat(math.Inf(1)), true},
		{"negative inf", object.NewFloat(math.Inf(-1)), true},
		{"normal", object.NewFloat(1.0), false},
		{"zero", object.NewFloat(0.0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInf(ctx, tt.input)
			b, ok := result.(*object.Bool)
			assert.True(t, ok)
			assert.Equal(t, b.Value(), tt.expected)
		})
	}
}

func TestIsInfErrors(t *testing.T) {
	ctx := context.Background()
	result := IsInf(ctx)
	assert.True(t, object.IsError(result))

	result = IsInf(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
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
			result := Round(ctx, tt.input)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestRoundErrors(t *testing.T) {
	ctx := context.Background()
	result := Round(ctx)
	assert.True(t, object.IsError(result))

	result = Round(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestInf(t *testing.T) {
	ctx := context.Background()

	// Default positive infinity
	result := Inf(ctx)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsInf(f.Value(), 1))

	// Positive sign
	result = Inf(ctx, object.NewInt(1))
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsInf(f.Value(), 1))

	// Negative sign
	result = Inf(ctx, object.NewInt(-1))
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsInf(f.Value(), -1))
}

func TestInfErrors(t *testing.T) {
	ctx := context.Background()

	// Too many arguments
	result := Inf(ctx, object.NewInt(1), object.NewInt(2))
	assert.True(t, object.IsError(result))

	// Wrong type
	result = Inf(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestModule(t *testing.T) {
	m := Module()
	assert.NotNil(t, m)
	assert.Equal(t, m.Name().Value(), "math")

	// Verify key functions exist
	_, ok := m.GetAttr("abs")
	assert.True(t, ok)

	_, ok = m.GetAttr("sqrt")
	assert.True(t, ok)

	_, ok = m.GetAttr("sin")
	assert.True(t, ok)

	// Verify constants
	pi, ok := m.GetAttr("PI")
	assert.True(t, ok)
	piFloat, ok := pi.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, piFloat.Value(), math.Pi)

	e, ok := m.GetAttr("E")
	assert.True(t, ok)
	eFloat, ok := e.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, eFloat.Value(), math.E)
}
