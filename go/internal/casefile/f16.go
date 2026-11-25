package casefile

import "fmt"

// Additions of package F16, slice D2 (spec/packages/f16-entry-points.md 12.10)
// to the case format: entry, args, input, entryReturns, and expect.entries.

// ExpEntry is one expected entry point in expect.entries (F16 12.10).
type ExpEntry struct {
	Name           string
	Doc            *string
	HasDoc         bool // distinguishes absent doc from "doc": null
	Params         []ExpEntryParam
	HasParams      bool
	Input          *string
	HasInput       bool // distinguishes absent input from "input": null
	InputSchema    any
	HasInputSchema bool
	Result         *string
}

// ExpEntryParam is one expected parameter in ExpEntry.Params.
type ExpEntryParam struct {
	Name     *string // nil for null (unnamed pattern or _)
	Type     string
	Doc      *string
	HasDoc   bool
	Optional bool
}

var (
	expEntryFields      = set("name", "doc", "params", "input", "inputSchema", "result")
	expEntryParamFields = set("name", "type", "doc", "optional")
)

func (v *validator) validateD2Fields(m map[string]any, where string) error {
	if x, ok := m["entry"]; ok {
		if _, ok := x.(string); !ok {
			return v.invalid("%s: entry is not a string", where)
		}
	}
	if x, ok := m["args"]; ok {
		if _, ok := x.([]any); !ok {
			return v.invalid("%s: args is not an array", where)
		}
	}
	if x, ok := m["entryReturns"]; ok {
		em, ok := x.(map[string]any)
		if !ok {
			return v.invalid("%s: entryReturns is not an object", where)
		}
		for k, val := range em {
			if _, ok := val.(string); !ok {
				return v.invalid("%s: entryReturns.%s is not a string", where, k)
			}
		}
	}
	return nil
}

func (v *validator) buildD2(c *Case, m map[string]any, where string) error {
	c.Entry = optString(m, "entry")
	_, hasArgs := m["args"]
	_, hasInput := m["input"]
	if hasArgs && hasInput {
		return v.invalid("%s has both args and input", where)
	}
	if (hasArgs || hasInput) && c.Entry == nil {
		return v.invalid("%s has args or input without entry", where)
	}
	if hasArgs {
		ca, err := v.opaque(m["args"], c.Opaque, where)
		if err != nil {
			return err
		}
		c.Args, c.HasArgs = ca.([]any), true
	}
	if hasInput {
		ci, err := v.opaque(m["input"], c.Opaque, where)
		if err != nil {
			return err
		}
		c.Input, c.HasInput = ci, true
	}
	if x, ok := m["entryReturns"]; ok {
		c.EntryReturns = map[string]string{}
		for k, val := range x.(map[string]any) {
			c.EntryReturns[k] = val.(string)
		}
	}
	return nil
}

func (v *validator) expEntries(x any, where string) ([]ExpEntry, error) {
	arr, ok := x.([]any)
	if !ok {
		return nil, v.invalid("%s: entries is not an array", where)
	}
	out := make([]ExpEntry, len(arr))
	for i, el := range arr {
		ew := fmt.Sprintf("%s: entry %d", where, i)
		om, ok := el.(map[string]any)
		if !ok {
			return nil, v.invalid("%s is not an object", ew)
		}
		if err := v.checkFields(om, expEntryFields, ew); err != nil {
			return nil, err
		}
		name, ok := om["name"].(string)
		if !ok {
			return nil, v.invalid("%s has no name", ew)
		}
		ee := ExpEntry{Name: name}
		if dv, ok := om["doc"]; ok {
			ee.HasDoc = true
			if dv != nil {
				s, ok := dv.(string)
				if !ok {
					return nil, v.invalid("%s: doc is neither a string nor null", ew)
				}
				ee.Doc = &s
			}
		}
		if pv, ok := om["params"]; ok {
			pa, ok := pv.([]any)
			if !ok {
				return nil, v.invalid("%s: params is not an array", ew)
			}
			ee.HasParams = true
			ee.Params = make([]ExpEntryParam, len(pa))
			for j, pel := range pa {
				pw := fmt.Sprintf("%s: param %d", ew, j)
				pm, ok := pel.(map[string]any)
				if !ok {
					return nil, v.invalid("%s is not an object", pw)
				}
				if err := v.checkFields(pm, expEntryParamFields, pw); err != nil {
					return nil, err
				}
				nv, hasN := pm["name"]
				if !hasN {
					return nil, v.invalid("%s has no name", pw)
				}
				var ep ExpEntryParam
				if nv != nil {
					s, ok := nv.(string)
					if !ok {
						return nil, v.invalid("%s: name is neither a string nor null", pw)
					}
					ep.Name = &s
				}
				ts, ok := pm["type"].(string)
				if !ok {
					return nil, v.invalid("%s has no type", pw)
				}
				ep.Type = ts
				if dv, ok := pm["doc"]; ok {
					ep.HasDoc = true
					if dv != nil {
						s, ok := dv.(string)
						if !ok {
							return nil, v.invalid("%s: doc is neither a string nor null", pw)
						}
						ep.Doc = &s
					}
				}
				opt, ok := pm["optional"].(bool)
				if !ok {
					return nil, v.invalid("%s has no optional boolean", pw)
				}
				ep.Optional = opt
				ee.Params[j] = ep
			}
		}
		if iv, ok := om["input"]; ok {
			ee.HasInput = true
			if iv != nil {
				s, ok := iv.(string)
				if !ok {
					return nil, v.invalid("%s: input is neither a string nor null", ew)
				}
				ee.Input = &s
			}
		}
		if sv, ok := om["inputSchema"]; ok {
			ee.InputSchema, ee.HasInputSchema = sv, true
		}
		if rv, ok := om["result"]; ok {
			s, ok := rv.(string)
			if !ok {
				return nil, v.invalid("%s: result is not a string", ew)
			}
			ee.Result = &s
		}
		out[i] = ee
	}
	return out, nil
}
