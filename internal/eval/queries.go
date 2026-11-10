package eval

import (
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is the library entries of package F7, queries
// (spec/packages/f07-queries.md 6): sortBy with a direction, minBy, maxBy,
// uniqueBy, indexBy, countBy and mapValues.

func init() {
	for k, f := range map[string]libFunc{
		"list.sortBy":   sortBy,
		"list.minBy":    func(r *run, c *libCall) any { return pickBy(r, c, true) },
		"list.maxBy":    func(r *run, c *libCall) any { return pickBy(r, c, false) },
		"list.uniqueBy": uniqueBy,
		"list.indexBy":  indexBy,
		"list.countBy":  countBy,
		"map.mapValues": mapValues,
	} {
		implemented[k] = f
	}
	// The table of entry information was made before init: refresh the
	// entries this file implements.
	for e, in := range infos {
		if f, ok := implemented[Key(e)]; ok {
			in.impl = f
		}
	}
}

// keysOf calls f once for each element, in order (2 steps each).
func keysOf(r *run, c *libCall) (elems, keys []any) {
	lam, n := c.args[0].(*lambdaCode), c.n()
	elems, keys = make([]any, n), make([]any, n)
	for i := 0; i < n; i++ {
		elems[i] = c.elem(r, i)
		keys[i] = r.callback(c, lam, elems[i], i)
	}
	return elems, keys
}

// sortBy is 6.3: a stable sort by the keys, ascending or descending.
func sortBy(r *run, c *libCall) any {
	desc := len(c.args) > 1 && c.args[1] == "desc"
	elems, keys := keysOf(r, c)
	n := len(elems)
	r.ordered(c, n, func(i int) any { return keys[i] }, false)
	f := fenv{c: c, n: int64(n), keysW: strW(keys)}
	r.libSteps(c, f, 2*int64(n))
	r.libUnits(c, f)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	if desc {
		stableSort(idx, func(a, b int) bool { return less(keys[b], keys[a]) })
	} else {
		stableSort(idx, func(a, b int) bool { return less(keys[a], keys[b]) })
	}
	out := make([]any, n)
	for i, j := range idx {
		out[i] = elems[j]
	}
	return out
}

// pickBy is minBy and maxBy (6.4, 6.5): the first element with the least
// or the greatest key.
func pickBy(r *run, c *libCall, min bool) any {
	elems, keys := keysOf(r, c)
	n := len(elems)
	r.ordered(c, n, func(i int) any { return keys[i] }, false)
	f := fenv{c: c, n: int64(n), keysW: strW(keys)}
	r.libSteps(c, f, 2*int64(n))
	r.libUnits(c, f)
	if n == 0 {
		return nil
	}
	best := 0
	for i := 1; i < n; i++ {
		if (min && less(keys[i], keys[best])) || (!min && less(keys[best], keys[i])) {
			best = i
		}
	}
	return elems[best]
}

// uniqueBy is 6.6: the first element with each key.
func uniqueBy(r *run, c *libCall) any {
	elems, keys := keysOf(r, c)
	n := len(elems)
	r.ordered(c, n, func(i int) any { return keys[i] }, false)
	f := fenv{c: c, n: int64(n), keysW: strW(keys)}
	r.libSteps(c, f, 2*int64(n))
	seen := make(map[any]bool, n)
	out := []any{}
	for i, k := range keys {
		if !seen[k] {
			seen[k] = true
			out = append(out, elems[i])
		}
	}
	f.k = int64(len(out))
	r.libUnits(c, f)
	return out
}

// stringKeys calls f for each element and tests each result when that call
// returns: a string, else wrong-kind, template result (6.2).
func stringKeys(r *run, c *libCall) (elems []any, keys []string, w int64) {
	lam, n := c.args[0].(*lambdaCode), c.n()
	elems, keys = make([]any, n), make([]string, n)
	for i := 0; i < n; i++ {
		elems[i] = c.elem(r, i)
		k := r.callback(c, lam, elems[i], i)
		ks, ok := k.(string)
		if !ok {
			r.raise(fail("wrong-kind", "result", c.span, messages.Word("name", c.e.Name), messages.Val("kind", k)))
		}
		keys[i] = ks
		w += value.Weight(ks)
	}
	return elems, keys, w
}

// indexBy is 6.7: each key mapped to the last element with it.
func indexBy(r *run, c *libCall) any {
	elems, keys, w := stringKeys(r, c)
	f := fenv{c: c, n: int64(len(elems)), keysW: w}
	r.libSteps(c, f, 2*f.n)
	r.libUnits(c, f)
	out := make(map[string]any, len(keys))
	for i, k := range keys {
		out[k] = elems[i]
	}
	return out
}

// countBy is 6.8: the number of elements with each key.
func countBy(r *run, c *libCall) any {
	elems, keys, w := stringKeys(r, c)
	counts := make(map[string]int, len(keys))
	for _, k := range keys {
		counts[k]++
	}
	f := fenv{c: c, n: int64(len(elems)), g: int64(len(counts)), keysW: w}
	r.libSteps(c, f, 2*f.n)
	r.libUnits(c, f)
	out := make(map[string]any, len(counts))
	for k, v := range counts {
		out[k] = numBox(float64(v))
	}
	return out
}

// mapValues is 6.9: the keys put in key order first, then f for each entry
// with its value and its key.
func mapValues(r *run, c *libCall) any {
	lam := c.args[0].(*lambdaCode)
	n := value.Len(c.recv)
	var w int64
	value.ForEachKey(c.recv, func(k string) { w += value.Weight(k) })
	f := fenv{c: c, n: int64(n), keysW: w}
	r.libSteps(c, f, 2*int64(n))
	keys := value.Keys(c.recv)
	out := make(map[string]any, n)
	for i, k := range keys {
		v, _ := value.Get(c.recv, k)
		v = r.read(v, c.span)
		r.step(2)
		c.idx = i // F8 4.2 item 2: the key's position
		r.enterCall(c.measure)
		if lam.nparams > 0 {
			lam.bind(r, 0, v, true)
		}
		if lam.nparams > 1 {
			lam.bind(r, 1, k, true)
		}
		out[k] = lam.body(r)
		r.depth -= c.measure
	}
	r.libUnits(c, f)
	return out
}
