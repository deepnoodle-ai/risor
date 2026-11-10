package valuerep

// node is the spike's representation: every value is an any.
type node func(*frame) any

func konst(v any) node { return func(*frame) any { return v } }
func glob(i int) node  { return func(fr *frame) any { return fr.rt.globals[i] } }
func slot(i int) node  { return func(fr *frame) any { return fr.slots[i] } }

func field(x node, name string) node {
	return func(fr *frame) any {
		v := x(fr)
		fr.rt.tick()
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return f
			}
		}
		panic(errType)
	}
}

func number(v any) float64 {
	switch v := v.(type) {
	case int64:
		return float64(v)
	case float64:
		return v
	}
	panic(errType)
}

func arith(op byte, x, y node) node {
	switch op {
	case '+':
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return addInt(a, b)
				}
			case float64:
				if b, ok := b.(float64); ok {
					return finite(a+b, a, b)
				}
			}
			fa, fb := number(a), number(b)
			return finite(fa+fb, fa, fb)
		}
	case '-':
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return subInt(a, b)
				}
			case float64:
				if b, ok := b.(float64); ok {
					return finite(a-b, a, b)
				}
			}
			fa, fb := number(a), number(b)
			return finite(fa-fb, fa, fb)
		}
	case '*':
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a := a.(type) {
			case int64:
				if b, ok := b.(int64); ok {
					return mulInt(a, b)
				}
			case float64:
				if b, ok := b.(float64); ok {
					return finite(a*b, a, b)
				}
			}
			fa, fb := number(a), number(b)
			return finite(fa*fb, fa, fb)
		}
	case '%':
		return func(fr *frame) any {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			if a, ok := a.(int64); ok {
				if b, ok := b.(int64); ok {
					return modInt(a, b)
				}
			}
			panic(errType)
		}
	}
	panic("op")
}

func less(x, y node) node {
	return func(fr *frame) any {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		switch a := a.(type) {
		case int64:
			if b, ok := b.(int64); ok {
				return a < b
			}
		case float64:
			if b, ok := b.(float64); ok {
				return a < b
			}
		}
		return number(a) < number(b)
	}
}

func greater(x, y node) node {
	return func(fr *frame) any {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		switch a := a.(type) {
		case int64:
			if b, ok := b.(int64); ok {
				return a > b
			}
		case float64:
			if b, ok := b.(float64); ok {
				return a > b
			}
		}
		return number(a) > number(b)
	}
}

func truthy(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case nil:
		return false
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v != ""
	}
	return true
}

func and(x, y node) node {
	return func(fr *frame) any {
		if v := x(fr); !truthy(v) {
			return v
		}
		return y(fr)
	}
}

// reduce runs body once per item with the accumulator and the item in slots
// 0 and 1 of the current frame.
func reduce(list, init, body node) node {
	return func(fr *frame) any {
		items := list(fr).([]any)
		acc := init(fr)
		rt := fr.rt
		for _, item := range items {
			rt.tick()
			fr.slots[0], fr.slots[1] = acc, item
			acc = body(fr)
		}
		return acc
	}
}
