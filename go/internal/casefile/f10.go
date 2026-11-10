package casefile

import "fmt"

// The additions of package F10 10 to the case format: returnsSchema and
// load.

// ReturnsSchema is the expected result by schema (F10 10.1).
type ReturnsSchema struct {
	Schema  any
	Options []string
}

var returnsSchemaFields = set("schema", "options")

// buildF10 reads returnsSchema and load of a program case, after the
// header rule.
func (v *validator) buildF10(c *Case, m map[string]any, where string) error {
	if x, ok := m["returnsSchema"]; ok {
		if _, both := m["returns"]; both {
			return v.invalid("%s has both returns and returnsSchema", where)
		}
		o, ok := x.(map[string]any)
		if !ok {
			return v.invalid("%s: returnsSchema is not an object", where)
		}
		if err := v.checkFields(o, returnsSchemaFields, where+": returnsSchema"); err != nil {
			return err
		}
		s, ok := o["schema"]
		if !ok {
			return v.invalid("%s: returnsSchema has no schema", where)
		}
		rs := &ReturnsSchema{Schema: s}
		if opts, ok := o["options"]; ok {
			list, ok := stringList(opts)
			if !ok {
				return v.invalid("%s: returnsSchema options is not a list of strings", where)
			}
			seen := map[string]bool{}
			for _, x := range list {
				if !schemaOptions[x] || seen[x] {
					return v.invalid("%s: returnsSchema options names %q, which is not an option or is named twice", where, x)
				}
				seen[x] = true
			}
			rs.Options = list
		}
		c.ReturnsSchema = rs
	}
	if x, ok := m["load"]; ok {
		b, ok := x.(bool)
		if !ok {
			return v.invalid("%s: load is not true or false", where)
		}
		c.Load = b
	}
	if c.Load {
		if c.ReturnsSchema == nil && (c.Returns == nil || *c.Returns == "condition") {
			return v.invalid("%s: a load case needs an expected type", where)
		}
		if c.Check != nil && !*c.Check {
			return v.invalid("%s: a load case always checks", where)
		}
	}
	if !c.Load && c.Expect != nil {
		for _, d := range c.Expect.Diagnostics {
			if d.SpanNull {
				return v.invalid("%s: \"span\": null outside a load case", where)
			}
		}
	}
	return nil
}

// factsOf reads the field facts of package F14 10.1: a list of objects,
// each with exactly the key someNotNull, a list of strings.
func factsOf(x any) ([][]string, error) {
	arr, ok := x.([]any)
	if !ok {
		return nil, fmt.Errorf("facts is not an array")
	}
	out := [][]string{}
	for _, e := range arr {
		o, ok := e.(map[string]any)
		if !ok || len(o) != 1 {
			return nil, fmt.Errorf("a host fact is not an object with the one key someNotNull")
		}
		names, ok := stringList(o["someNotNull"])
		if !ok {
			return nil, fmt.Errorf("someNotNull is not an array of strings")
		}
		if names == nil {
			names = []string{}
		}
		out = append(out, names)
	}
	return out, nil
}
