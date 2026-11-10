package valuerep

import "cmp"

// Typed closures: the checker says a node is an int, a float or a bool, so
// the closure returns that type and nothing is boxed between operators. A
// value read from the host is still an any and is unboxed with a check.
type (
	inode func(*frame) int64
	fnode func(*frame) float64
	bnode func(*frame) bool
)

func coerceInt(v any) int64 {
	switch v := v.(type) {
	case int64:
		return v
	case float64: // a JSON number that the schema declares an integer
		if i := int64(v); float64(i) == v && uint64(i+maxSafe) <= 2*maxSafe {
			return i
		}
	}
	panic(errType)
}

func coerceFloat(v any) float64 {
	switch v := v.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	}
	panic(errType)
}

// unboxInt adapts a boxed node to a typed one, as at a field read.
func unboxInt(x node) inode {
	return func(fr *frame) int64 {
		v := x(fr)
		if i, ok := v.(int64); ok {
			return i
		}
		return coerceInt(v)
	}
}

func unboxFloat(x node) fnode {
	return func(fr *frame) float64 {
		v := x(fr)
		if f, ok := v.(float64); ok {
			return f
		}
		return coerceFloat(v)
	}
}

// boxInt and boxBool adapt a typed node to a boxed one, as at the result.
func boxInt(x inode) node  { return func(fr *frame) any { return x(fr) } }
func boxBool(x bnode) node { return func(fr *frame) any { return x(fr) } }

func ikonst(v int64) inode   { return func(*frame) int64 { return v } }
func fkonst(v float64) fnode { return func(*frame) float64 { return v } }
func iglob(i int) inode      { return unboxInt(glob(i)) }
func fglob(i int) fnode      { return unboxFloat(glob(i)) }
func islot(i int) inode      { return func(fr *frame) int64 { return fr.slots[i].(int64) } }
func islotTyped(i int) inode { return func(fr *frame) int64 { return fr.ints[i] } }

func iarith(op byte, x, y inode) inode {
	switch op {
	case '+':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return addInt(a, b) }
	case '-':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return subInt(a, b) }
	case '*':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return mulInt(a, b) }
	case '%':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return modInt(a, b) }
	}
	panic("op")
}

// iarithWrap is iarith with the 64-bit overflow checks, and iarithNone has
// no check at all. They show what the range check costs.
func iarithWrap(op byte, x, y inode) inode {
	switch op {
	case '+':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return addWrap(a, b) }
	case '-':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return subWrap(a, b) }
	case '*':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return mulWrap(a, b) }
	}
	panic("op")
}

func iarithNone(op byte, x, y inode) inode {
	switch op {
	case '+':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return a + b }
	case '-':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return a - b }
	case '*':
		return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return a * b }
	}
	panic("op")
}

func farith(op byte, x, y fnode) fnode {
	switch op {
	case '+':
		return func(fr *frame) float64 { a, b := x(fr), y(fr); fr.rt.tick(); return finite(a+b, a, b) }
	case '*':
		return func(fr *frame) float64 { a, b := x(fr), y(fr); fr.rt.tick(); return finite(a*b, a, b) }
	}
	panic("op")
}

func iless(x, y inode) bnode {
	return func(fr *frame) bool { a, b := x(fr), y(fr); fr.rt.tick(); return a < b }
}

func igreater(x, y inode) bnode {
	return func(fr *frame) bool { a, b := x(fr), y(fr); fr.rt.tick(); return a > b }
}

func fgreater(x, y fnode) bnode {
	return func(fr *frame) bool { a, b := x(fr), y(fr); fr.rt.tick(); return a > b }
}

func band(x, y bnode) bnode {
	return func(fr *frame) bool { return x(fr) && y(fr) }
}

// One generic function serves int64, float64 and string. Each instantiation
// has its own shape, so there is no dictionary on the hot path.
func lessG[T cmp.Ordered](x, y func(*frame) T) bnode {
	return func(fr *frame) bool { a, b := x(fr), y(fr); fr.rt.tick(); return a < b }
}

func greaterG[T cmp.Ordered](x, y func(*frame) T) bnode {
	return func(fr *frame) bool { a, b := x(fr), y(fr); fr.rt.tick(); return a > b }
}

// ireduce keeps the accumulator and the item boxed in the frame's slots, so
// the body's result is boxed once per iteration.
func ireduce(list node, init int64, body inode) inode {
	return func(fr *frame) int64 {
		items := list(fr).([]any)
		var acc any = init
		rt := fr.rt
		for _, item := range items {
			rt.tick()
			fr.slots[0], fr.slots[1] = acc, item
			acc = body(fr)
		}
		return acc.(int64)
	}
}

// ireduceTyped keeps them in typed slots, so nothing is boxed.
func ireduceTyped(list node, init int64, body inode) inode {
	return func(fr *frame) int64 {
		items := list(fr).([]any)
		acc := init
		rt := fr.rt
		for _, item := range items {
			rt.tick()
			fr.ints[0], fr.ints[1] = acc, coerceInt(item)
			acc = body(fr)
		}
		return acc
	}
}
