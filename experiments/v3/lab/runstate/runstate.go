// Package runstate asks one question: where should the state of one Run
// live, so that a short expression costs one allocation or none and a
// compiled program is still safe to run from many goroutines?
//
// The program is gate.failed > 0. The options:
//
//   - alloc_spike: a new run object per Run, the size of the spike's (384 B)
//
//   - alloc_small: a new run object per Run, trimmed to what a short
//     expression needs
//
//   - pool:        run objects from a sync.Pool, cleared before they go back
//
//   - caller:      the caller owns the run object and passes it in
//
//     GOWORK=off go test ./lab/runstate -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package runstate

import (
	"errors"
	"sync"
)

var (
	errType      = errors.New("type error")
	errUndefined = errors.New("undefined variable")
)

// frame is the spike's frame: six inline slots.
type frame struct {
	slots  []any
	parent *frame
	rt     *runtime
	ret    any
	inline [6]any
}

type runtime struct {
	globals []any
	left    int64
	depth   int
	pool    []*frame
	args    []any
}

// spikeRun is everything the spike allocates for one Run.
type spikeRun struct {
	rt   runtime
	top  frame
	gbuf [4]any
	abuf [4]any
}

// smallRun keeps the counter, room for four globals and four locals, and
// nothing else. Anything more is allocated only by a program that needs it.
type smallRun struct {
	rt   runtime
	top  smallFrame
	gbuf [4]any
}

type smallFrame struct {
	slots  []any
	rt     *runtime
	inline [4]any
}

type node func(rt *runtime) (any, error)

type Program struct {
	body    node
	globals []string
	nslots  int
}

// gate.failed > 0
func compile() *Program {
	gate := func(rt *runtime) (any, error) { return rt.globals[0], nil }
	failed := func(rt *runtime) (any, error) {
		v, _ := gate(rt)
		rt.left--
		if m, ok := v.(map[string]any); ok {
			if f, ok := m["failed"]; ok {
				return f, nil
			}
		}
		return nil, errType
	}
	body := func(rt *runtime) (any, error) {
		v, err := failed(rt)
		if err != nil {
			return nil, err
		}
		rt.left--
		if f, ok := v.(float64); ok {
			return f > 0, nil
		}
		return nil, errType
	}
	return &Program{body: body, globals: []string{"gate"}}
}

func (p *Program) bind(rt *runtime, buf []any, env map[string]any) error {
	if len(p.globals) <= len(buf) {
		rt.globals = buf[:len(p.globals)]
	} else {
		rt.globals = make([]any, len(p.globals))
	}
	for i, name := range p.globals {
		v, ok := env[name]
		if !ok {
			return errUndefined
		}
		rt.globals[i] = v
	}
	return nil
}

func (p *Program) RunAllocSpike(env map[string]any, steps int64) (any, error) {
	r := &spikeRun{}
	r.rt.left = steps
	r.rt.args = r.abuf[:0]
	r.top.rt = &r.rt
	r.top.slots = r.top.inline[:p.nslots]
	if err := p.bind(&r.rt, r.gbuf[:], env); err != nil {
		return nil, err
	}
	return p.body(&r.rt)
}

func (p *Program) RunAllocSmall(env map[string]any, steps int64) (any, error) {
	r := &smallRun{}
	r.rt.left = steps
	r.top.rt = &r.rt
	r.top.slots = r.top.inline[:p.nslots]
	if err := p.bind(&r.rt, r.gbuf[:], env); err != nil {
		return nil, err
	}
	return p.body(&r.rt)
}

var runs = sync.Pool{New: func() any { return &spikeRun{} }}

func (p *Program) RunPooled(env map[string]any, steps int64) (any, error) {
	r := runs.Get().(*spikeRun)
	out, err := p.runWith(r, env, steps)
	// Drop every reference to the host's data before the object is shared.
	r.rt.globals = nil
	clear(r.gbuf[:])
	clear(r.top.inline[:p.nslots])
	runs.Put(r)
	return out, err
}

// RunWith runs with state the caller owns. The caller must not share r
// between goroutines.
func (p *Program) RunWith(r *spikeRun, env map[string]any, steps int64) (any, error) {
	return p.runWith(r, env, steps)
}

func (p *Program) runWith(r *spikeRun, env map[string]any, steps int64) (any, error) {
	r.rt.left = steps
	r.rt.args = r.abuf[:0]
	r.top.rt = &r.rt
	r.top.slots = r.top.inline[:p.nslots]
	if err := p.bind(&r.rt, r.gbuf[:], env); err != nil {
		return nil, err
	}
	return p.body(&r.rt)
}
