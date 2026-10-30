package object

import (
	"math"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestNegativeShiftCount(t *testing.T) {
	cases := []struct {
		name  string
		left  Object
		opTyp op.BinaryOpType
		right Object
	}{
		{"int << -1", NewInt(1), op.LShift, NewInt(-1)},
		{"int >> -1", NewInt(1), op.RShift, NewInt(-1)},
		{"byte << -1", NewByte(1), op.LShift, NewInt(-1)},
		{"byte >> -1", NewByte(1), op.RShift, NewInt(-1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.left.RunOperation(tc.opTyp, tc.right)
			assert.Nil(t, result)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "negative shift count")
		})
	}

	// Non-negative shifts are unchanged
	result, err := NewByte(1).RunOperation(op.LShift, NewInt(8))
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(256))
	result, err = NewInt(-8).RunOperation(op.RShift, NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(-4))
}

func TestBytesStringSymmetric(t *testing.T) {
	s := NewString("a")
	b := NewBytes([]byte("a"))
	assert.True(t, s.Equals(b))
	assert.True(t, b.Equals(s))
	assert.False(t, NewString("a").Equals(NewBytes([]byte("b"))))

	pairs := []struct{ str, byts string }{
		{"a", "a"}, {"a", "b"}, {"b", "a"}, {"", "a"}, {"ab", "a"},
	}
	for _, p := range pairs {
		sc, err := NewString(p.str).Compare(NewBytes([]byte(p.byts)))
		assert.Nil(t, err)
		bc, err := NewBytes([]byte(p.byts)).Compare(NewString(p.str))
		assert.Nil(t, err)
		assert.Equal(t, sc, -bc, p.str, p.byts)
	}

	ls := NewList([]Object{NewBytes([]byte("a"))})
	assert.Equal(t, ls.Contains(NewString("a")), True)
	assert.Equal(t, ls.Index(NewString("a")), int64(0))
}

func TestNaNComparisons(t *testing.T) {
	nan := NewFloat(math.NaN())
	others := []Object{nan, NewFloat(1), NewInt(1), NewByte(1)}
	ordered := []op.CompareOpType{op.LessThan, op.LessThanOrEqual, op.GreaterThan, op.GreaterThanOrEqual}
	for _, other := range others {
		for _, opType := range ordered {
			result, err := Compare(opType, nan, other)
			assert.Nil(t, err)
			assert.Equal(t, result, False, "NaN", opType, other.Inspect())
			result, err = Compare(opType, other, nan)
			assert.Nil(t, err)
			assert.Equal(t, result, False, other.Inspect(), opType, "NaN")
		}
		result, err := Compare(op.Equal, nan, other)
		assert.Nil(t, err)
		assert.Equal(t, result, False)
		result, err = Compare(op.NotEqual, other, nan)
		assert.Nil(t, err)
		assert.Equal(t, result, True)
	}
}

func TestSortNaNLast(t *testing.T) {
	nan := math.NaN()
	items := []Object{
		NewFloat(3), NewFloat(nan), NewInt(1), NewFloat(nan),
		NewFloat(2), NewByte(0), NewFloat(math.Inf(1)), NewFloat(math.Inf(-1)),
	}
	assert.Nil(t, Sort(items))
	var got []string
	for _, item := range items {
		got = append(got, item.Inspect())
	}
	assert.Equal(t, got, []string{"-Inf", "0", "1", "2", "3", "+Inf", "NaN", "NaN"})
}

func TestIntPowerFloat(t *testing.T) {
	result, err := NewInt(2).RunOperation(op.Power, NewFloat(0.5))
	assert.Nil(t, err)
	assert.Equal(t, result, NewFloat(math.Sqrt2))
	result, err = NewInt(2).RunOperation(op.Power, NewFloat(-1))
	assert.Nil(t, err)
	assert.Equal(t, result, NewFloat(0.5))
	result, err = NewInt(2).RunOperation(op.Power, NewFloat(3))
	assert.Nil(t, err)
	assert.Equal(t, result, NewFloat(8))
}

func TestIntFloatExactComparison(t *testing.T) {
	const big = int64(1) << 53 // 9007199254740992
	bigInt := NewInt(big)
	nextInt := NewInt(big + 1)
	bigFloat := NewFloat(float64(big))

	// Equality is transitive: big+1 is not representable as float64
	assert.True(t, bigInt.Equals(bigFloat))
	assert.True(t, bigFloat.Equals(bigInt))
	assert.False(t, nextInt.Equals(bigFloat))
	assert.False(t, bigFloat.Equals(nextInt))
	assert.False(t, bigInt.Equals(nextInt))

	cmp, err := nextInt.Compare(bigFloat)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 1)
	cmp, err = bigFloat.Compare(nextInt)
	assert.Nil(t, err)
	assert.Equal(t, cmp, -1)

	cases := []struct {
		i    int64
		f    float64
		want int
	}{
		{1, 1.0, 0},
		{1, 1.5, -1},
		{2, 1.5, 1},
		{-1, -1.5, 1},
		{-2, -1.5, -1},
		{0, math.Copysign(0, -1), 0},
		{math.MaxInt64, math.Exp2(63), -1},
		{math.MinInt64, -math.Exp2(63), 0},
		{math.MinInt64, math.Nextafter(-math.Exp2(63), math.Inf(-1)), 1},
		{math.MaxInt64, math.Inf(1), -1},
		{math.MinInt64, math.Inf(-1), 1},
		{0, math.NaN(), -1},
	}
	for _, tc := range cases {
		assert.Equal(t, compareIntFloat(tc.i, tc.f), tc.want, tc.i, tc.f)
	}
}
