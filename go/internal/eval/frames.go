package eval

import (
	"slices"

	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
)

// Frames (spec/packages/f17-recursion.md 11.5). Each call of a lambda has its
// own frame: the parameters and the names its body binds, outside the
// lambdas written inside it. A lambda reads an outer name from the frame
// of the call where it was written, by the frame's link up. The program has
// the first frame. A lambda never outlives the call where it was written,
// so the frames are a stack: the values of every frame are in one slice,
// run.vals, and run.frames holds where each frame starts and its link.
//
// This is the one path of every program (02-evaluation.md 10.2, 11.1): a
// program with no lambda that calls itself runs on it too.

// frameRec is a frame on the stack.
type frameRec struct {
	base int // the index of the frame's first value in run.vals
	up   int // the frame where the frame's lambda was written; -1 for the program's
}

// place is where a name's value is: the frame hops links up from the frame
// of the code that reads it, at index idx.
type place struct{ hops, idx int }

// layout gives each slot of the static rules its frame and index. A slot
// belongs to the innermost lambda around the place that binds it, or to
// the program.
type layout struct {
	level []int // by slot: the level of its frame, 0 for the program's
	idx   []int // by slot: its index in its frame
	// size is the number of values of each lambda's frame; top is that of
	// the program's frame.
	size map[*syntax.Lambda]int
	top  int
	// lamLevel is the level of each lambda's own frame: 1 more than the
	// level of the frame where it is written.
	lamLevel map[*syntax.Lambda]int
}

func newLayout(prog *syntax.Program, res *static.Result) *layout {
	ly := &layout{level: make([]int, res.NSlots), idx: make([]int, res.NSlots),
		size: map[*syntax.Lambda]int{}, lamLevel: map[*syntax.Lambda]int{}}
	for i := range ly.level {
		ly.level[i] = -1
	}
	var owners []*syntax.Lambda // nil for the program
	counts := []int{0}
	add := func(key any) {
		s, ok := res.Slots[key]
		if !ok || s < 0 || s >= len(ly.level) || ly.level[s] >= 0 {
			return
		}
		k := len(counts) - 1
		ly.level[s], ly.idx[s] = k, counts[k]
		counts[k]++
	}
	var pattern func(p syntax.Pattern)
	pattern = func(p syntax.Pattern) {
		switch p := p.(type) {
		case *syntax.NamePat:
			add(p)
		case *syntax.MapPat:
			for _, f := range p.Fields {
				add(f)
			}
			if p.Rest != nil {
				add(p.Rest)
			}
		case *syntax.ListPat:
			for _, e := range p.Elems {
				if e.Name != nil {
					add(e.Name)
				}
			}
			if p.Rest != nil {
				add(p.Rest)
			}
		}
	}
	var visit func(n syntax.Node)
	visit = func(n syntax.Node) {
		switch n := n.(type) {
		case nil:
			return
		case *syntax.Lambda:
			owners = append(owners, n)
			counts = append(counts, 0)
			ly.lamLevel[n] = len(counts) - 1
			for _, k := range syntax.Children(n) {
				visit(k)
			}
			ly.size[n] = counts[len(counts)-1]
			owners = owners[:len(owners)-1]
			counts = counts[:len(counts)-1]
			return
		case *syntax.NamePat, *syntax.MapPat, *syntax.ListPat:
			pattern(n.(syntax.Pattern))
		case *syntax.Try:
			add(n)
		}
		for _, k := range syntax.Children(n) {
			visit(k)
		}
	}
	visit(prog)
	ly.top = counts[0]
	return ly
}

// place is the place of a slot, read by code in a frame of level from.
func (ly *layout) place(slot, from int) place {
	return place{hops: from - ly.level[slot], idx: ly.idx[slot]}
}

// frameAt is the index of the frame hops links up from the current one.
func (r *run) frameAt(hops int) int {
	f := r.fp
	for ; hops > 0; hops-- {
		f = r.frames[f].up
	}
	return f
}

// baseAt is the base of the frame hops links up from the current one.
func (r *run) baseAt(hops int) int {
	if hops == 0 {
		return r.base
	}
	return r.frames[r.frameAt(hops)].base
}

// get reads the value at a place.
func (r *run) get(p place) any {
	if p.hops == 0 {
		return r.vals[r.base+p.idx]
	}
	return r.vals[r.baseAt(p.hops)+p.idx]
}

// set writes the value at a place.
func (r *run) set(p place, v any) { r.vals[r.baseAt(p.hops)+p.idx] = v }

// enter pushes a frame of n values for a lambda written in frame up, and
// makes it the current frame. It gives what leave restores.
func (r *run) enter(up, n int) (fp, base int) {
	fp, base = r.fp, r.base
	b := len(r.vals)
	if cap(r.vals)-b < n {
		r.vals = slices.Grow(r.vals, n)
	}
	// The values past the end of the stack are nil: leave and a try clear
	// a frame's values when they drop it.
	r.vals = r.vals[:b+n]
	r.frames = append(r.frames, frameRec{base: b, up: up})
	r.fp, r.base = len(r.frames)-1, b
	return fp, base
}

// leave drops the current frame and restores the frame enter left.
func (r *run) leave(fp, base int) {
	clear(r.vals[r.base:])
	r.vals = r.vals[:r.base]
	r.frames = r.frames[:r.fp]
	r.fp, r.base = fp, base
}

// frameMark is the state of the frame stack, which a try restores when it
// catches a failure.
type frameMark struct{ nvals, nframes, fp, base int }

func (r *run) frameMark() frameMark {
	return frameMark{len(r.vals), len(r.frames), r.fp, r.base}
}

func (r *run) restoreFrames(m frameMark) {
	clear(r.vals[m.nvals:])
	r.vals = r.vals[:m.nvals]
	r.frames = r.frames[:m.nframes]
	r.fp, r.base = m.fp, m.base
}

// fn is a lambda handed to a library entry: its code, and the number of
// frames up from the frame of the call to the frame where it was written.
// A library entry calls it while the frame of its call is the current one.
//
// A lambda given at a parameter with a function type of a const-bound
// lambda (f17-recursion.md 13.4) is called from frames that are not its
// call's, so it is a closure: abs is set, and up is the index of the frame
// where it was evaluated. No function outlives the call where it was
// evaluated, so that frame is on the stack whenever the closure runs.
type fn struct {
	*lambdaCode
	hops int
	abs  bool
	up   int
}

// frameOf is the frame where a lambda handed on was written.
func (r *run) frameOf(l *fn) int {
	if l.abs {
		return l.up
	}
	return r.frameAt(l.hops)
}

// enterFn pushes the frame of a call of a lambda handed to a library entry.
func (r *run) enterFn(l *fn) (fp, base int) {
	return r.enter(r.frameOf(l), l.size)
}

// cbEnter makes current the frame of a library entry's next call of its
// lambda. The calls an entry makes of a lambda never overlap: each ends
// before the entry makes the next. So one frame serves them all. The first
// call pushes it, and each later call binds its values again, as a call
// binds its own names (f17-recursion.md 11.5 item 1): a name of the frame is
// never read before the call binds it. cbLeave drops the frame when the
// entry returns; a failure drops it with the frames a try restores.
func (r *run) cbEnter(c *libCall, l *fn) {
	if c.cb == l && r.fp == c.cbFP {
		return
	}
	r.cbLeave(c)
	c.cbSaveFP, c.cbSaveBase = r.enterFn(l)
	c.cb, c.cbFP = l, r.fp
}

// cbLeave drops the frame of a library call's lambda, if it has one.
func (r *run) cbLeave(c *libCall) {
	if c.cb != nil {
		r.leave(c.cbSaveFP, c.cbSaveBase)
		c.cb = nil
	}
}
