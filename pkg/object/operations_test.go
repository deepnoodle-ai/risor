package object

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

func TestCompareNonComparable(t *testing.T) {
	m1 := NewMap(nil)
	m2 := NewMap(nil)
	_, err := Compare(op.LessThan, m1, m2)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "type error: expected a comparable object (got map)")
}

func TestCompareUnknownComparison(t *testing.T) {
	obj1 := NewInt(1)
	obj2 := NewInt(2)
	_, err := Compare(op.CompareOpType(222), obj1, obj2)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "eval error: unknown object comparison operator: 222")
}

func TestAndOperator(t *testing.T) {
	type testCase struct {
		left  Object
		right Object
		want  Object
	}
	testCases := []testCase{
		{NewInt(1), NewInt(1), NewInt(1)},
		{NewInt(1), NewInt(0), NewInt(0)},
		{NewInt(0), NewInt(1), NewInt(0)},
		{NewInt(0), NewInt(0), NewInt(0)},
		{NewInt(1), NewBool(true), NewBool(true)},
		{NewInt(1), NewBool(false), NewBool(false)},
		{NewInt(0), NewBool(true), NewInt(0)},
		{NewInt(0), NewBool(false), NewInt(0)},
		{NewBool(true), NewInt(1), NewInt(1)},
		{NewBool(true), NewInt(0), NewInt(0)},
	}
	for _, tc := range testCases {
		result, err := BinaryOp(op.And, tc.left, tc.right)
		assert.NoError(t, err)
		assert.Equal(t, result, tc.want)
	}
}

func TestOrOperator(t *testing.T) {
	type testCase struct {
		left  Object
		right Object
		want  Object
	}
	testCases := []testCase{
		{NewInt(1), NewInt(1), NewInt(1)},
		{NewInt(1), NewInt(0), NewInt(1)},
		{NewInt(0), NewInt(1), NewInt(1)},
		{NewInt(0), NewInt(0), NewInt(0)},
		{NewInt(1), NewBool(true), NewInt(1)},
		{NewInt(1), NewBool(false), NewInt(1)},
		{NewInt(0), NewBool(true), NewBool(true)},
		{NewInt(0), NewBool(false), NewBool(false)},
		{NewBool(true), NewInt(1), NewBool(true)},
		{NewBool(true), NewInt(0), NewBool(true)},
	}
	for _, tc := range testCases {
		result, err := BinaryOp(op.Or, tc.left, tc.right)
		assert.NoError(t, err)
		assert.Equal(t, result, tc.want)
	}
}
