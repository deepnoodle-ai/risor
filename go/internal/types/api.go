package types

import "fmt"

// Helpers of the binding's structural type API.

// IsFloat reports whether t's number alternative is float.
func (t *Type) IsFloat() bool { return t.u().num == numFloat }

// Alternatives splits t into its alternatives, one for each kind it has:
// null, bool, the number (int or float), the string or literal set, the
// list, the map alternative (a record or a tagged union), each opaque type
// and each type variable. any is its own one alternative; never has none.
func Alternatives(t *Type) []*Type {
	t = t.u()
	if t.any {
		return []*Type{Any}
	}
	var out []*Type
	if t.null {
		out = append(out, Null)
	}
	if t.boolean {
		out = append(out, Bool)
	}
	switch t.num {
	case numInt:
		out = append(out, Int)
	case numFloat:
		out = append(out, Float)
	}
	switch t.str {
	case strAll:
		out = append(out, String)
	case strLits:
		out = append(out, &Type{str: strLits, lits: t.lits})
	}
	if t.list != nil {
		out = append(out, List(t.list))
	}
	if t.rec != nil {
		out = append(out, &Type{rec: t.rec, uni: t.uni})
	}
	for _, o := range t.opaque {
		out = append(out, Opaque(o))
	}
	for _, v := range t.vars {
		out = append(out, Var(v))
	}
	return out
}

// UnionOf is the type of the alternatives ts, as `|` joins them in type
// text (F5 3.1): the records among them, a union contributing its
// variants, must form a tagged union when they are two or more. A
// reference stays by name unless it shares a kind other than null with
// another alternative, as in type text (F17 12.2 item 5).
func UnionOf(ts []*Type) (*Type, error) {
	out, ok := Alternatives3(ts)
	if !ok {
		return nil, fmt.Errorf("two or more records that form no tagged union in one type")
	}
	return out, nil
}

// Limit reports an error when t passes the limits of a type (05-types.md
// 20.4, 20.7).
func Limit(t *Type) error {
	if Depth(t) > MaxDepth {
		return fmt.Errorf("the type is deeper than %d", MaxDepth)
	}
	if Size(t) > MaxSize {
		return fmt.Errorf("the type is larger than %d", MaxSize)
	}
	return nil
}
