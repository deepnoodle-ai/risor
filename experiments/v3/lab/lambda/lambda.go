// Package lambda asks one question: how should a list method run the
// function literal it is given, as in xs.filter(x => x < n)?
//
//   - call_pooled: the literal is a closure value; each item is a call that
//     takes a frame from a pool and returns it (the spike, when no inner
//     function can capture the frame)
//
//   - call_alloc:  the same, with a new frame per call (the spike, when an
//     inner function can capture the frame)
//
//   - inline:      the literal's body is compiled into the enclosing
//     function, its parameter is a slot of the enclosing frame, and the
//     method runs the body directly: no closure value, no frame, no call
//
//     GOWORK=off go test ./lab/lambda -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package lambda

import "errors"

var (
	errType  = errors.New("type error")
	errArity = errors.New("wrong number of arguments")
	errDepth = errors.New("call depth exceeded")
)

type runtime struct {
	globals []any
	left    int64
	depth   int
	pool    []*frame
}

type frame struct {
	slots  []any
	parent *frame
	rt     *runtime
	inline [4]any
}

type node func(*frame) (any, error)

type proto struct {
	nparams, nslots int
	body            node
	recycle         bool
}

type closure struct {
	proto *proto
	env   *frame
}

func glob(i int) node  { return func(fr *frame) (any, error) { return fr.rt.globals[i], nil } }
func slot(i int) node  { return func(fr *frame) (any, error) { return fr.slots[i], nil } }
func outer(i int) node { return func(fr *frame) (any, error) { return fr.parent.slots[i], nil } }

func field(x node, name string) node {
	return func(fr *frame) (any, error) {
		v, err := x(fr)
		if err != nil {
			return nil, err
		}
		fr.rt.left--
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return f, nil
			}
		}
		return nil, errType
	}
}

func greater(x, y node) node {
	return func(fr *frame) (any, error) {
		a, err := x(fr)
		if err != nil {
			return nil, err
		}
		b, err := y(fr)
		if err != nil {
			return nil, err
		}
		fr.rt.left--
		if a, ok := a.(int64); ok {
			if b, ok := b.(int64); ok {
				return a > b, nil
			}
		}
		return nil, errType
	}
}

// function makes a closure value, as the spike does for a function literal.
func function(p *proto) node {
	return func(fr *frame) (any, error) { return &closure{proto: p, env: fr}, nil }
}

func (rt *runtime) call1(fn any, arg any) (any, error) {
	cl, ok := fn.(*closure)
	if !ok {
		return nil, errType
	}
	p := cl.proto
	if p.nparams != 1 {
		return nil, errArity
	}
	rt.depth++
	if rt.depth > 1000 {
		return nil, errDepth
	}
	var f *frame
	if n := len(rt.pool); p.recycle && n > 0 {
		f = rt.pool[n-1]
		rt.pool = rt.pool[:n-1]
	} else {
		f = &frame{rt: rt}
	}
	f.parent = cl.env
	f.slots = f.inline[:p.nslots]
	f.slots[0] = arg
	out, err := p.body(f)
	rt.depth--
	if p.recycle {
		clear(f.slots)
		f.parent = nil
		rt.pool = append(rt.pool, f)
	}
	return out, err
}

// filterCall is list.filter(fn) where fn is a closure value.
func filterCall(list, fn node) node {
	return func(fr *frame) (any, error) {
		l, err := list(fr)
		if err != nil {
			return nil, err
		}
		f, err := fn(fr)
		if err != nil {
			return nil, err
		}
		items, ok := l.([]any)
		if !ok {
			return nil, errType
		}
		rt := fr.rt
		out := []any{}
		for _, item := range items {
			rt.left--
			keep, err := rt.call1(f, item)
			if err != nil {
				return nil, err
			}
			if keep == true {
				out = append(out, item)
			}
		}
		return out, nil
	}
}

// filterInline is list.filter(x => body) where the compiler saw the literal.
// The parameter is slot param of the current frame.
func filterInline(list node, param int, body node) node {
	return func(fr *frame) (any, error) {
		l, err := list(fr)
		if err != nil {
			return nil, err
		}
		items, ok := l.([]any)
		if !ok {
			return nil, errType
		}
		rt := fr.rt
		out := []any{}
		for _, item := range items {
			rt.left--
			fr.slots[param] = item
			keep, err := body(fr)
			if err != nil {
				return nil, err
			}
			if keep == true {
				out = append(out, item)
			}
		}
		fr.slots[param] = nil
		return out, nil
	}
}

// mapCall and mapInline are list.map, for the nested case.
func mapCall(list, fn node) node {
	return func(fr *frame) (any, error) {
		l, err := list(fr)
		if err != nil {
			return nil, err
		}
		f, err := fn(fr)
		if err != nil {
			return nil, err
		}
		items := l.([]any)
		rt := fr.rt
		out := make([]any, len(items))
		for i, item := range items {
			rt.left--
			if out[i], err = rt.call1(f, item); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
}

func mapInline(list node, param int, body node) node {
	return func(fr *frame) (any, error) {
		l, err := list(fr)
		if err != nil {
			return nil, err
		}
		items := l.([]any)
		rt := fr.rt
		out := make([]any, len(items))
		for i, item := range items {
			rt.left--
			fr.slots[param] = item
			if out[i], err = body(fr); err != nil {
				return nil, err
			}
		}
		fr.slots[param] = nil
		return out, nil
	}
}
