// Package counts asks one question: what does it cost to make an optimized
// evaluation charge exactly the steps that the unoptimized evaluation of the
// program as written would charge?
//
// Three optimizations are each built three ways:
//
//   - reference:  no optimization; this defines the count
//   - free:       optimized, charging only for the work it actually does
//   - preserving: optimized, charging what the reference charges, failing
//     with the same error at the same point
//
// The optimizations are constant folding, a fused path of field reads, and
// membership in a constant list done as a set lookup.
//
//	GOWORK=off go test ./lab/counts -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package counts

import "errors"

var (
	errSteps = errors.New("step budget exceeded")
	errField = errors.New("no such field")
	errType  = errors.New("type error")
)

type runtime struct {
	globals []any
	left    int64
}

func (rt *runtime) step(n int64) error {
	rt.left -= n
	if rt.left < 0 {
		return errSteps
	}
	return nil
}

type node func(*runtime) (any, error)

func glob(i int) node  { return func(rt *runtime) (any, error) { return rt.globals[i], nil } }
func konst(v any) node { return func(*runtime) (any, error) { return v, nil } }

// ---------------------------------------------------------------- folding

// mul charges cost steps: 1 for itself, plus the steps of any constant
// operand that the compiler folded away. Nothing that can fail lies between
// those charges in the reference, so one charge is the same as several.
func mul(x, y node, cost int64) node {
	return func(rt *runtime) (any, error) {
		a, err := x(rt)
		if err != nil {
			return nil, err
		}
		b, err := y(rt)
		if err != nil {
			return nil, err
		}
		if err := rt.step(cost); err != nil {
			return nil, err
		}
		if a, ok := a.(int64); ok {
			if b, ok := b.(int64); ok {
				return a * b, nil
			}
		}
		return nil, errType
	}
}

// charged is a folded constant with no operator above it to carry its cost.
func charged(v any, cost int64) node {
	return func(rt *runtime) (any, error) {
		if err := rt.step(cost); err != nil {
			return nil, err
		}
		return v, nil
	}
}

// and is &&, which charges nothing itself.
func and(x, y node) node {
	return func(rt *runtime) (any, error) {
		a, err := x(rt)
		if err != nil || a != true {
			return a, err
		}
		return y(rt)
	}
}

// ------------------------------------------------------------ fused paths

// field is one field read: the reference.
func field(x node, name string) node {
	return func(rt *runtime) (any, error) {
		v, err := x(rt)
		if err != nil {
			return nil, err
		}
		if err := rt.step(1); err != nil {
			return nil, err
		}
		if m, ok := v.(map[string]any); ok {
			if f, ok := m[name]; ok {
				return f, nil
			}
		}
		return nil, errField
	}
}

// pathFree reads the whole path in one closure and charges one step.
func pathFree(x node, names ...string) node {
	return func(rt *runtime) (any, error) {
		v, err := x(rt)
		if err != nil {
			return nil, err
		}
		if err := rt.step(1); err != nil {
			return nil, err
		}
		for _, name := range names {
			m, ok := v.(map[string]any)
			if !ok {
				return nil, errField
			}
			if v, ok = m[name]; !ok {
				return nil, errField
			}
		}
		return v, nil
	}
}

// pathPreserving charges the whole path at once. If a read fails part of the
// way, it gives back the steps of the reads that never happened. If the
// budget cannot cover the whole path, it reads one field at a time, so the
// run ends with the error the reference would give.
func pathPreserving(x node, names ...string) node {
	n := int64(len(names))
	return func(rt *runtime) (any, error) {
		v, err := x(rt)
		if err != nil {
			return nil, err
		}
		if rt.left < n {
			return pathSlow(rt, v, names)
		}
		rt.left -= n
		for i, name := range names {
			m, ok := v.(map[string]any)
			if ok {
				v, ok = m[name]
			}
			if !ok {
				rt.left += n - int64(i) - 1
				return nil, errField
			}
		}
		return v, nil
	}
}

func pathSlow(rt *runtime, v any, names []string) (any, error) {
	for _, name := range names {
		if err := rt.step(1); err != nil {
			return nil, err
		}
		m, ok := v.(map[string]any)
		if ok {
			v, ok = m[name]
		}
		if !ok {
			return nil, errField
		}
	}
	return v, nil
}

// ------------------------------------------------------------- membership

// inScan is x in [constants]: the reference compares in order and charges
// one step per element compared.
func inScan(x node, list []string) node {
	return func(rt *runtime) (any, error) {
		v, err := x(rt)
		if err != nil {
			return nil, err
		}
		s, ok := v.(string)
		if !ok {
			return nil, errType
		}
		for _, item := range list {
			if err := rt.step(1); err != nil {
				return nil, err
			}
			if item == s {
				return true, nil
			}
		}
		return false, nil
	}
}

// inSetFree is one lookup and one step.
func inSetFree(x node, list []string) node {
	set := map[string]int64{}
	for i, item := range list {
		if _, ok := set[item]; !ok {
			set[item] = int64(i) + 1
		}
	}
	return func(rt *runtime) (any, error) {
		v, err := x(rt)
		if err != nil {
			return nil, err
		}
		s, ok := v.(string)
		if !ok {
			return nil, errType
		}
		if err := rt.step(1); err != nil {
			return nil, err
		}
		return set[s] > 0, nil
	}
}

// inSetPreserving is one lookup. The set maps each value to the number of
// comparisons the scan makes before it finds it.
func inSetPreserving(x node, list []string) node {
	set := map[string]int64{}
	for i, item := range list {
		if _, ok := set[item]; !ok {
			set[item] = int64(i) + 1
		}
	}
	miss := int64(len(list))
	return func(rt *runtime) (any, error) {
		v, err := x(rt)
		if err != nil {
			return nil, err
		}
		s, ok := v.(string)
		if !ok {
			return nil, errType
		}
		cost, found := set[s]
		if !found {
			cost = miss
		}
		if err := rt.step(cost); err != nil {
			return nil, err
		}
		return found, nil
	}
}
