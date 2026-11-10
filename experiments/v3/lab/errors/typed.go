package errors

// The same question for typed closures, which return an int64 or a bool
// instead of an any. With error returns each closure returns two values.

type (
	pinode func(*frame) int64
	pbnode func(*frame) bool
	einode func(*frame) (int64, error)
	ebnode func(*frame) (bool, error)
)

func piglob(i int) pinode {
	return func(fr *frame) int64 {
		if v, ok := fr.rt.globals[i].(int64); ok {
			return v
		}
		panic(&Error{1, 1, "not an int"})
	}
}

func piadd(x, y pinode) pinode {
	return func(fr *frame) int64 { a, b := x(fr), y(fr); fr.rt.tick(); return a + b }
}

func piless(x, y pinode) pbnode {
	return func(fr *frame) bool { a, b := x(fr), y(fr); fr.rt.tick(); return a < b }
}

func pbox(x pbnode) pnode { return func(fr *frame) any { return x(fr) } }

func eiglob(i int) einode {
	return func(fr *frame) (int64, error) {
		if v, ok := fr.rt.globals[i].(int64); ok {
			return v, nil
		}
		return 0, &Error{1, 1, "not an int"}
	}
}

func eiadd(x, y einode) einode {
	return func(fr *frame) (int64, error) {
		a, err := x(fr)
		if err != nil {
			return 0, err
		}
		b, err := y(fr)
		if err != nil {
			return 0, err
		}
		if err := fr.rt.tickErr(); err != nil {
			return 0, err
		}
		return a + b, nil
	}
}

func eiless(x, y einode) ebnode {
	return func(fr *frame) (bool, error) {
		a, err := x(fr)
		if err != nil {
			return false, err
		}
		b, err := y(fr)
		if err != nil {
			return false, err
		}
		if err := fr.rt.tickErr(); err != nil {
			return false, err
		}
		return a < b, nil
	}
}

func ebox(x ebnode) enode {
	return func(fr *frame) (any, error) {
		v, err := x(fr)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
}
