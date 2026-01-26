package object

import (
	"github.com/risor-io/risor/op"
)

// Compare two objects using the given comparison operator. An Error object is
// returned if either of the objects is not comparable.
func Compare(opType op.CompareOpType, a, b Object) (Object, error) {
	switch opType {
	case op.Equal:
		return a.Equals(b), nil
	case op.NotEqual:
		return Not(a.Equals(b).(*Bool)), nil
	}

	comparable, ok := a.(Comparable)
	if !ok {
		return nil, TypeErrorf("type error: expected a comparable object (got %s)", a.Type())
	}
	value, err := comparable.Compare(b)
	if err != nil {
		return nil, err
	}

	switch opType {
	case op.LessThan:
		return NewBool(value < 0), nil
	case op.LessThanOrEqual:
		return NewBool(value <= 0), nil
	case op.GreaterThan:
		return NewBool(value > 0), nil
	case op.GreaterThanOrEqual:
		return NewBool(value >= 0), nil
	default:
		return nil, EvalErrorf("eval error: unknown object comparison operator: %d", opType)
	}
}

// BinaryOp performs a binary operation on two objects, given an operator.
func BinaryOp(opType op.BinaryOpType, a, b Object) (Object, error) {
	switch opType {
	case op.And:
		aTruthy := a.IsTruthy()
		bTruthy := b.IsTruthy()
		if aTruthy && bTruthy {
			return b, nil
		} else if aTruthy {
			return b, nil // return b because it's falsy
		} else {
			return a, nil // return a because it's falsy
		}
	case op.Or:
		if a.IsTruthy() {
			return a, nil
		}
		return b, nil
	}
	// In Risor v2, RunOperation should return a separate error value
	result := a.RunOperation(opType, b)
	switch result := result.(type) {
	case *Error:
		if result.IsRaised() {
			return nil, result.Unwrap()
		}
		return result, nil
	default:
		return result, nil
	}
}
