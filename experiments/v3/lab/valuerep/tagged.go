package valuerep

import "math"

// Value is a tagged value of three words. A number or a bool lives in num,
// and ref holds a tag that says which. Everything else is the host's value,
// unchanged, in ref.
type Value struct {
	ref any
	num uint64
}

type (
	tagInt   struct{}
	tagFloat struct{}
	tagBool  struct{}
)

func intV(i int64) Value     { return Value{tagInt{}, uint64(i)} }
func floatV(f float64) Value { return Value{tagFloat{}, math.Float64bits(f)} }
func boolV(b bool) Value {
	if b {
		return Value{tagBool{}, 1}
	}
	return Value{tagBool{}, 0}
}

// wrap brings a host value into the evaluator.
func wrap(v any) Value {
	switch v := v.(type) {
	case int64:
		return intV(v)
	case float64:
		return floatV(v)
	case bool:
		return boolV(v)
	}
	return Value{ref: v}
}

// unwrap returns a value to the host.
func unwrap(v Value) any {
	switch v.ref.(type) {
	case tagInt:
		return int64(v.num)
	case tagFloat:
		return math.Float64frombits(v.num)
	case tagBool:
		return v.num != 0
	}
	return v.ref
}

type vnode func(*frame) Value

func vkonst(v any) vnode { w := wrap(v); return func(*frame) Value { return w } }
func vglob(i int) vnode  { return func(fr *frame) Value { return wrap(fr.rt.globals[i]) } }
func vslot(i int) vnode  { return func(fr *frame) Value { return fr.vslots[i] } }

func vfield(x vnode, name string) vnode {
	return func(fr *frame) Value {
		v := x(fr)
		fr.rt.tick()
		if m, ok := v.ref.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return wrap(f)
			}
		}
		panic(errType)
	}
}

func vnumber(v Value) float64 {
	switch v.ref.(type) {
	case tagInt:
		return float64(int64(v.num))
	case tagFloat:
		return math.Float64frombits(v.num)
	}
	panic(errType)
}

func varith(op byte, x, y vnode) vnode {
	switch op {
	case '+':
		return func(fr *frame) Value {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a.ref.(type) {
			case tagInt:
				if _, ok := b.ref.(tagInt); ok {
					return intV(addInt(int64(a.num), int64(b.num)))
				}
			case tagFloat:
				if _, ok := b.ref.(tagFloat); ok {
					fa, fb := math.Float64frombits(a.num), math.Float64frombits(b.num)
					return floatV(finite(fa+fb, fa, fb))
				}
			}
			fa, fb := vnumber(a), vnumber(b)
			return floatV(finite(fa+fb, fa, fb))
		}
	case '-':
		return func(fr *frame) Value {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a.ref.(type) {
			case tagInt:
				if _, ok := b.ref.(tagInt); ok {
					return intV(subInt(int64(a.num), int64(b.num)))
				}
			case tagFloat:
				if _, ok := b.ref.(tagFloat); ok {
					fa, fb := math.Float64frombits(a.num), math.Float64frombits(b.num)
					return floatV(finite(fa-fb, fa, fb))
				}
			}
			fa, fb := vnumber(a), vnumber(b)
			return floatV(finite(fa-fb, fa, fb))
		}
	case '*':
		return func(fr *frame) Value {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			switch a.ref.(type) {
			case tagInt:
				if _, ok := b.ref.(tagInt); ok {
					return intV(mulInt(int64(a.num), int64(b.num)))
				}
			case tagFloat:
				if _, ok := b.ref.(tagFloat); ok {
					fa, fb := math.Float64frombits(a.num), math.Float64frombits(b.num)
					return floatV(finite(fa*fb, fa, fb))
				}
			}
			fa, fb := vnumber(a), vnumber(b)
			return floatV(finite(fa*fb, fa, fb))
		}
	case '%':
		return func(fr *frame) Value {
			a, b := x(fr), y(fr)
			fr.rt.tick()
			if _, ok := a.ref.(tagInt); ok {
				if _, ok := b.ref.(tagInt); ok {
					return intV(modInt(int64(a.num), int64(b.num)))
				}
			}
			panic(errType)
		}
	}
	panic("op")
}

func vless(x, y vnode) vnode {
	return func(fr *frame) Value {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		switch a.ref.(type) {
		case tagInt:
			if _, ok := b.ref.(tagInt); ok {
				return boolV(int64(a.num) < int64(b.num))
			}
		case tagFloat:
			if _, ok := b.ref.(tagFloat); ok {
				return boolV(math.Float64frombits(a.num) < math.Float64frombits(b.num))
			}
		}
		return boolV(vnumber(a) < vnumber(b))
	}
}

func vgreater(x, y vnode) vnode {
	return func(fr *frame) Value {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		switch a.ref.(type) {
		case tagInt:
			if _, ok := b.ref.(tagInt); ok {
				return boolV(int64(a.num) > int64(b.num))
			}
		case tagFloat:
			if _, ok := b.ref.(tagFloat); ok {
				return boolV(math.Float64frombits(a.num) > math.Float64frombits(b.num))
			}
		}
		return boolV(vnumber(a) > vnumber(b))
	}
}

func vtruthy(v Value) bool {
	switch r := v.ref.(type) {
	case tagBool, tagInt:
		return v.num != 0
	case tagFloat:
		return math.Float64frombits(v.num) != 0
	case nil:
		return false
	case string:
		return r != ""
	}
	return true
}

func vand(x, y vnode) vnode {
	return func(fr *frame) Value {
		if v := x(fr); !vtruthy(v) {
			return v
		}
		return y(fr)
	}
}

func vreduce(list, init, body vnode) vnode {
	return func(fr *frame) Value {
		items := list(fr).ref.([]any)
		acc := init(fr)
		rt := fr.rt
		for _, item := range items {
			rt.tick()
			fr.vslots[0], fr.vslots[1] = acc, wrap(item)
			acc = body(fr)
		}
		return acc
	}
}
