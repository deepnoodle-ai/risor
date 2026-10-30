package object

import (
	"context"
	"encoding/json"
)

// MaxNestingDepth is the maximum number of nested containers (lists, maps,
// iterators, and cells, counting the outermost one) that recursive operations will
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
// the values it yields, and those may include the iterator itself. A cell
// counts, since converting or measuring it visits its value, and that value
// may hold the cell.
func isNestedContainer(obj Object) bool {
	switch obj.(type) {
	case *List, *Map, *Iter, *Cell:
		return true
	}
	return false
}

// ToInterface converts obj to a Go value, as Object.Interface does, but
// returns an error instead of a nil result. It returns ErrNestingDepth when
// obj is too deeply nested or contains a cycle. It returns a value error when
// the result would have more elements in total than the list length limit in
// ctx. The total counts every list item, map entry, iterator item, and range
// element, and counts a container again each time it is referenced. If ctx
// is canceled while an iterator is converted, it returns ctx.Err().
func ToInterface(ctx context.Context, obj Object) (any, error) {
	return toInterface(obj, nil, newConvBudget(ctx))
}

// ToInterfaces converts each of objs as ToInterface does. All of objs share
// one element budget, so the total across them is bounded by the list length
// limit in ctx.
func ToInterfaces(ctx context.Context, objs []Object) ([]any, error) {
	b := newConvBudget(ctx)
	result := make([]any, len(objs))
	for i, obj := range objs {
		value, err := toInterface(obj, nil, b)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

// ToJSONValue converts obj to a Go value for encoding/json, as ToInterface
// does, with the same nesting and element limits. null converts to nil. It
// returns a type error if obj is, or contains, a value with no JSON form: a
// function, builtin, module, or error.
func ToJSONValue(ctx context.Context, obj Object) (any, error) {
	b := newConvBudget(ctx)
	b.jsonOnly = true
	return toInterface(obj, nil, b)
}

// interfaceOrNil converts obj with the default limits and returns nil on
// error. Interface methods of containers use it. Code that has a context
// must call ToInterface instead, so that the limits in the context apply.
func interfaceOrNil(obj Object) any {
	result, err := toInterface(obj, nil, newConvBudget(context.Background()))
	if err != nil {
		return nil
	}
	return result
}

func toInterface(obj Object, path nestPath[Object], b *convBudget) (any, error) {
	switch obj := obj.(type) {
	case *List:
		return obj.toInterface(path, b)
	case *Map:
		return obj.toInterface(path, b)
	case *Iter:
		return obj.toInterface(path, b)
	case *Range:
		return obj.toInterface(b)
	case *Cell:
		// Cell.Interface would convert the value with the default limits.
		// The cell goes on the path, so a cell that holds itself, directly
		// or through a container, returns ErrNestingDepth.
		if obj.value == nil {
			return nil, nil
		}
		value := *obj.value
		if isNestedContainer(value) {
			p, err := path.descend(obj, value)
			if err != nil {
				return nil, err
			}
			path = p
		}
		return toInterface(value, path, b)
	}
	if !b.jsonOnly {
		return obj.Interface(), nil
	}
	// An error's Go value is a Go error, which encoding/json would write as
	// {}. Other values whose Go value is nil, apart from null, have none.
	if _, ok := obj.(*Error); ok {
		return nil, newTypeErrorf("cannot encode %s as JSON", obj.Type())
	}
	v := obj.Interface()
	if v == nil && obj.Type() != NIL {
		return nil, newTypeErrorf("cannot encode %s as JSON", obj.Type())
	}
	return v, nil
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
	case *Cell:
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
	case *Cell:
		return "cell(...)"
	case *Iter:
		// Inspecting an iterator does not visit its values, so it is safe
		return obj.Inspect()
	}
	return "[...]"
}
