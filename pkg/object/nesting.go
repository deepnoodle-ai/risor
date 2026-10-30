package object

import "encoding/json"

// MaxNestingDepth is the maximum number of nested containers (lists, maps, and
// iterators, counting the outermost one) that recursive operations will
// traverse. It applies to Equals, Compare, MarshalJSON, ToInterface, and
// conversion between Risor and Go values. A container that contains itself, directly or
// indirectly, is treated as exceeding the limit.
const MaxNestingDepth = 128

// ErrNestingDepth is returned when a value is nested more deeply than
// MaxNestingDepth or contains a reference cycle. Use errors.Is to detect it.
var ErrNestingDepth error = newValueErrorf(
	"maximum nesting depth of %d exceeded (value is too deeply nested or contains a cycle)",
	MaxNestingDepth)

// nestPath holds the containers on the current traversal path, outermost
// first. It is local to a single traversal, so concurrent traversals of the
// same value do not interfere. Containers are only pushed when they have a
// nested container to descend into, so traversing flat values never allocates.
type nestPath[T comparable] []T

// descend is called by a container (parent) before recursing into a nested
// container (child). It returns the path holding the child's ancestors, or
// ErrNestingDepth if the child is already on the path or would be deeper
// than MaxNestingDepth.
func (p nestPath[T]) descend(parent, child T) (nestPath[T], error) {
	// The parent is on top of the path if it already descended into an
	// earlier child. It can't be there for any other reason, since that
	// would mean it is its own ancestor, which is rejected below.
	if n := len(p); n == 0 || p[n-1] != parent {
		p = append(p, parent)
	}
	if len(p) >= MaxNestingDepth {
		return p, ErrNestingDepth
	}
	for _, ancestor := range p {
		if ancestor == child {
			return p, ErrNestingDepth
		}
	}
	return p, nil
}

// objectPair is a pair of containers being compared by Equals or Compare.
type objectPair [2]Object

// isNestedContainer reports whether obj holds other objects that a recursive
// operation may descend into. An iterator counts, since converting it visits
// the values it yields, and those may include the iterator itself.
func isNestedContainer(obj Object) bool {
	switch obj.(type) {
	case *List, *Map, *Iter:
		return true
	}
	return false
}

// ToInterface converts obj to a Go value, as Object.Interface does, but
// returns ErrNestingDepth instead of a nil result when obj is too deeply
// nested or contains a cycle.
func ToInterface(obj Object) (any, error) {
	return toInterface(obj, nil)
}

func toInterface(obj Object, path nestPath[Object]) (any, error) {
	switch obj := obj.(type) {
	case *List:
		return obj.toInterface(path)
	case *Map:
		return obj.toInterface(path)
	case *Iter:
		return obj.toInterface(path)
	}
	return obj.Interface(), nil
}

func equalsNested(a, b Object, path nestPath[objectPair]) bool {
	switch a := a.(type) {
	case *List:
		return a.equals(b, path)
	case *Map:
		return a.equals(b, path)
	}
	return a.Equals(b)
}

func marshalJSONNested(obj Object, path nestPath[Object]) ([]byte, error) {
	switch obj := obj.(type) {
	case *List:
		return obj.marshalJSON(path)
	case *Map:
		return obj.marshalJSON(path)
	}
	return json.Marshal(obj)
}

func inspectNested(obj Object, path nestPath[Object]) string {
	switch obj := obj.(type) {
	case *List:
		return obj.inspect(path)
	case *Map:
		return obj.inspect(path)
	}
	return obj.Inspect()
}

// inspectPlaceholder is printed in place of a container that is already being
// printed (a cycle) or that is nested more deeply than MaxNestingDepth.
func inspectPlaceholder(obj Object) string {
	switch obj := obj.(type) {
	case *Map:
		return "{...}"
	case *Iter:
		// Inspecting an iterator does not visit its values, so it is safe
		return obj.Inspect()
	}
	return "[...]"
}
