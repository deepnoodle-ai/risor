// Package errors asks one question: should a runtime error travel as a
// panic that Run recovers, or as an error value that every closure returns?
//
// The same trees are built both ways. The benchmarks measure the path with
// no error, which is nearly every evaluation, and the path with one.
//
//	GOWORK=off go test ./lab/errors -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package errors

import "fmt"

// Error is a runtime error with the position of the node that raised it.
type Error struct {
	Line, Col int
	Msg       string
}

func (e *Error) Error() string { return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg) }

type runtime struct {
	globals []any
	steps   int64
	max     int64
	stack   []frame // one frame per call depth
	depth   int
}

type frame struct {
	rt     *runtime
	slots  []any
	inline [1]any
}

// enter returns the frame for a call with one argument.
func (rt *runtime) enter(arg any) *frame {
	f := &rt.stack[rt.depth]
	rt.depth++
	f.rt, f.slots = rt, f.inline[:]
	f.slots[0] = arg
	return f
}

var errBudget = &Error{Msg: "step budget exceeded"}

// ------------------------------------------------------------------ panic

type pnode func(*frame) any

func (rt *runtime) tick() {
	rt.steps++
	if rt.steps > rt.max {
		panic(errBudget)
	}
}

func pglob(i int) pnode    { return func(fr *frame) any { return fr.rt.globals[i] } }
func pslot(i int) pnode    { return func(fr *frame) any { return fr.slots[i] } }
func pkonst(v any) pnode   { return func(*frame) any { return v } }
func pparen(x pnode) pnode { return func(fr *frame) any { return x(fr) } }

func pfield(x pnode, name string, col int) pnode {
	return func(fr *frame) any {
		v := x(fr)
		fr.rt.tick()
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return f
			}
			panic(&Error{1, col, "no field " + name})
		}
		panic(&Error{1, col, "not a map"})
	}
}

func padd(x, y pnode, col int) pnode {
	return func(fr *frame) any {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		if a, ok := a.(int64); ok {
			if b, ok := b.(int64); ok {
				return a + b
			}
		}
		panic(&Error{1, col, "cannot add"})
	}
}

func pless(x, y pnode, col int) pnode {
	return func(fr *frame) any {
		a, b := x(fr), y(fr)
		fr.rt.tick()
		if a, ok := a.(int64); ok {
			if b, ok := b.(int64); ok {
				return a < b
			}
		}
		panic(&Error{1, col, "cannot compare"})
	}
}

// pcount is list.count(pred) with the item in slot 0.
func pcount(list, pred pnode) pnode {
	return func(fr *frame) any {
		var n int64
		for _, item := range list(fr).([]any) {
			fr.rt.tick()
			fr.slots[0] = item
			if pred(fr) == true {
				n++
			}
		}
		return n
	}
}

// pfunc is a function of one argument. Its body is set after the tree that
// calls it is built, so that it can call itself.
type pfunc struct{ body pnode }

func pcall(fn *pfunc, arg pnode) pnode {
	return func(fr *frame) any {
		v := arg(fr)
		rt := fr.rt
		rt.tick()
		out := fn.body(rt.enter(v))
		rt.depth--
		return out
	}
}

func pif(cond, then, els pnode) pnode {
	return func(fr *frame) any {
		if cond(fr) == true {
			return then(fr)
		}
		return els(fr)
	}
}

// ptry is try(x, fallback): it needs its own recover.
func ptry(x, fallback pnode) pnode {
	return func(fr *frame) (out any) {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(*Error)
				if !ok || e == errBudget {
					panic(r)
				}
				out = fallback(fr)
			}
		}()
		return x(fr)
	}
}

func prun(body pnode, fr *frame) (out any, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*Error)
			if !ok {
				panic(r)
			}
			out, err = nil, e
		}
	}()
	return body(fr), nil
}

// ------------------------------------------------------------------ error

type enode func(*frame) (any, error)

func (rt *runtime) tickErr() error {
	rt.steps++
	if rt.steps > rt.max {
		return errBudget
	}
	return nil
}

func eglob(i int) enode    { return func(fr *frame) (any, error) { return fr.rt.globals[i], nil } }
func eslot(i int) enode    { return func(fr *frame) (any, error) { return fr.slots[i], nil } }
func ekonst(v any) enode   { return func(*frame) (any, error) { return v, nil } }
func eparen(x enode) enode { return func(fr *frame) (any, error) { return x(fr) } }

func efield(x enode, name string, col int) enode {
	return func(fr *frame) (any, error) {
		v, err := x(fr)
		if err != nil {
			return nil, err
		}
		if err := fr.rt.tickErr(); err != nil {
			return nil, err
		}
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return f, nil
			}
			return nil, &Error{1, col, "no field " + name}
		}
		return nil, &Error{1, col, "not a map"}
	}
}

func eadd(x, y enode, col int) enode {
	return func(fr *frame) (any, error) {
		a, err := x(fr)
		if err != nil {
			return nil, err
		}
		b, err := y(fr)
		if err != nil {
			return nil, err
		}
		if err := fr.rt.tickErr(); err != nil {
			return nil, err
		}
		if a, ok := a.(int64); ok {
			if b, ok := b.(int64); ok {
				return a + b, nil
			}
		}
		return nil, &Error{1, col, "cannot add"}
	}
}

func eless(x, y enode, col int) enode {
	return func(fr *frame) (any, error) {
		a, err := x(fr)
		if err != nil {
			return nil, err
		}
		b, err := y(fr)
		if err != nil {
			return nil, err
		}
		if err := fr.rt.tickErr(); err != nil {
			return nil, err
		}
		if a, ok := a.(int64); ok {
			if b, ok := b.(int64); ok {
				return a < b, nil
			}
		}
		return nil, &Error{1, col, "cannot compare"}
	}
}

func ecount(list, pred enode) enode {
	return func(fr *frame) (any, error) {
		l, err := list(fr)
		if err != nil {
			return nil, err
		}
		var n int64
		for _, item := range l.([]any) {
			if err := fr.rt.tickErr(); err != nil {
				return nil, err
			}
			fr.slots[0] = item
			v, err := pred(fr)
			if err != nil {
				return nil, err
			}
			if v == true {
				n++
			}
		}
		return n, nil
	}
}

type efunc struct{ body enode }

func ecall(fn *efunc, arg enode) enode {
	return func(fr *frame) (any, error) {
		v, err := arg(fr)
		if err != nil {
			return nil, err
		}
		rt := fr.rt
		if err := rt.tickErr(); err != nil {
			return nil, err
		}
		out, err := fn.body(rt.enter(v))
		rt.depth--
		return out, err
	}
}

func eif(cond, then, els enode) enode {
	return func(fr *frame) (any, error) {
		c, err := cond(fr)
		if err != nil {
			return nil, err
		}
		if c == true {
			return then(fr)
		}
		return els(fr)
	}
}

func etry(x, fallback enode) enode {
	return func(fr *frame) (any, error) {
		v, err := x(fr)
		if err != nil {
			if err == errBudget {
				return nil, err
			}
			return fallback(fr)
		}
		return v, nil
	}
}

func erun(body enode, fr *frame) (any, error) { return body(fr) }

// erunNet is erun with a safety net: a panic in a host function, or a bug in
// the evaluator, becomes an error and does not cross Run.
func erunNet(body enode, fr *frame) (out any, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, &Error{Msg: fmt.Sprint("internal error: ", r)}
		}
	}()
	return body(fr)
}
