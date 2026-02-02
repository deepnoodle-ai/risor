package object

import (
	"github.com/deepnoodle-ai/risor/v2/op"
)

// Compare two objects using the given comparison operator. An error is
// returned if either of the objects is not comparable.
func Compare(opType op.CompareOpType, a, b Object) (Object, error) {
	switch opType {
	case op.Equal:
		return NewBool(a.Equals(b)), nil
	case op.NotEqual:
		return NewBool(!a.Equals(b)), nil
	}

	comparable, ok := a.(Comparable)
	if !ok {
		return nil, TypeErrorf("expected a comparable object (got %s)", a.Type())
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
	return a.RunOperation(opType, b)
}
