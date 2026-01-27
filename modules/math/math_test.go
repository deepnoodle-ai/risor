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
			result, err := Abs(ctx, tt.input)
			assert.Nil(t, err)
			assert.True(t, object.Equals(result, tt.expected), "got %s, want %s", result.Inspect(), tt.expected.Inspect())
		})
	}
}

func TestAbsErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	_, err := Abs(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = Abs(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Sqrt(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestSqrtErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	_, err := Sqrt(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = Sqrt(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Max(ctx, tt.x, tt.y)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestMaxErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	_, err := Max(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Max(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Max(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.NotNil(t, err)
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
			result, err := Min(ctx, tt.x, tt.y)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestMinErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	_, err := Min(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Min(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Min(ctx, object.NewFloat(1.0), object.NewString("b"))
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
	// Wrong argument count
	_, err := Sum(ctx)
	assert.NotNil(t, err)

	// Not a list
	_, err = Sum(ctx, object.NewString("hello"))
	assert.NotNil(t, err)

	// List with invalid types
	_, err = Sum(ctx, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewString("invalid"),
	}))
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
			result, err := Sin(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestSinErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Sin(ctx)
	assert.NotNil(t, err)

	_, err = Sin(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Cos(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestCosErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Cos(ctx)
	assert.NotNil(t, err)

	_, err = Cos(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Tan(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestTanErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Tan(ctx)
	assert.NotNil(t, err)

	_, err = Tan(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Mod(ctx, tt.x, tt.y)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestModErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Mod(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Mod(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Mod(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.NotNil(t, err)
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
			result, err := Log(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestLogErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Log(ctx)
	assert.NotNil(t, err)

	_, err = Log(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Log10(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestLog10Errors(t *testing.T) {
	ctx := context.Background()
	_, err := Log10(ctx)
	assert.NotNil(t, err)

	_, err = Log10(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Log2(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.expected) < 1e-10)
		})
	}
}

func TestLog2Errors(t *testing.T) {
	ctx := context.Background()
	_, err := Log2(ctx)
	assert.NotNil(t, err)

	_, err = Log2(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Pow(ctx, tt.x, tt.y)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.want)
		})
	}
}

func TestPowErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Pow(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Pow(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Pow(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.NotNil(t, err)
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
			result, err := Pow10(ctx, tt.input)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.Equal(t, f.Value(), tt.expected)
		})
	}
}

func TestPow10Errors(t *testing.T) {
	ctx := context.Background()
	_, err := Pow10(ctx)
	assert.NotNil(t, err)

	_, err = Pow10(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
			result, err := Atan2(ctx, tt.y, tt.x)
			assert.Nil(t, err)
			f, ok := result.(*object.Float)
			assert.True(t, ok)
			assert.True(t, math.Abs(f.Value()-tt.want) < 1e-10)
		})
	}
}

func TestAtan2Errors(t *testing.T) {
	ctx := context.Background()
	_, err := Atan2(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Atan2(ctx, object.NewString("a"), object.NewFloat(1.0))
	assert.NotNil(t, err)

	_, err = Atan2(ctx, object.NewFloat(1.0), object.NewString("b"))
	assert.NotNil(t, err)
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
			result, err := IsInf(ctx, tt.input)
			assert.Nil(t, err)
			b, ok := result.(*object.Bool)
			assert.True(t, ok)
			assert.Equal(t, b.Value(), tt.expected)
		})
	}
}

func TestIsInfErrors(t *testing.T) {
	ctx := context.Background()
	_, err := IsInf(ctx)
	assert.NotNil(t, err)

	_, err = IsInf(ctx, object.NewString("hello"))
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

func TestInf(t *testing.T) {
	ctx := context.Background()

	// Default positive infinity
	result, err := Inf(ctx)
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsInf(f.Value(), 1))

	// Positive sign
	result, err = Inf(ctx, object.NewInt(1))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsInf(f.Value(), 1))

	// Negative sign
	result, err = Inf(ctx, object.NewInt(-1))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.True(t, math.IsInf(f.Value(), -1))
}

func TestInfErrors(t *testing.T) {
	ctx := context.Background()

	// Too many arguments
	_, err := Inf(ctx, object.NewInt(1), object.NewInt(2))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Inf(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
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
