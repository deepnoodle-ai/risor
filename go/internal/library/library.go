// Package library reads the specification table suite/library.json
// (spec/core/04-library.md 1.3), copied into table_gen.go by internal/gen.
// It holds the records only; the evaluator implements the entries.
package library

import (
	"encoding/json"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// Fail is one record of an entry's fails.
type Fail struct {
	Code     string `json:"code"`
	Template string `json:"template"`
}

// Entry is one library entry.
type Entry struct {
	Name      string   `json:"name"`
	Receiver  string   `json:"receiver"`
	Signature string   `json:"signature"`
	Calls     []string `json:"calls"`
	Steps     string   `json:"steps"`
	Units     string   `json:"units"`
	Fails     []Fail   `json:"fails"`

	// Sig is the parsed signature.
	Sig *typetext.Sig `json:"-"`
	// StepsF and UnitsF are the parsed formulas.
	StepsF, UnitsF *Formula `json:"-"`
	// Kind is the receiver's kind for a method: String, List or Map.
	Kind value.Kind `json:"-"`
	// Params describes each parameter, for the tests of 2.6 and the shape
	// test.
	Params []Param `json:"-"`
}

// Param is what the run tests of one parameter.
type Param struct {
	Name string
	// Callback is the count of arguments the entry calls the lambda with,
	// or -1 when the parameter is not a callback.
	Callback int
	// Kinds is the set of kinds the parameter accepts. Any is set for any.
	Kinds map[value.Kind]bool
	Any   bool
	// Whole is set when a number must be whole (type int).
	Whole bool
	// Ordered is set for a parameter bound "ordered" (template unordered).
	Ordered bool
	Rest    bool
}

// IsMethod reports whether the entry is a method.
func (e *Entry) IsMethod() bool { return e.Receiver != "" }

// MinArgs and MaxArgs are the counts of 05-types.md 22.3; MaxArgs is -1 with
// a rest parameter.
func (e *Entry) MinArgs() int { return e.Sig.MinArgs() }

// MaxArgs is the most arguments; -1 with a rest parameter.
func (e *Entry) MaxArgs() int { return e.Sig.MaxArgs() }

// ParamAt returns the parameter at argument i (22.3), or nil.
func (e *Entry) ParamAt(i int) *Param {
	if i < len(e.Params) {
		return &e.Params[i]
	}
	if n := len(e.Params); n > 0 && e.Params[n-1].Rest {
		return &e.Params[n-1]
	}
	return nil
}

var (
	// Entries are all the records, in the order of the table.
	Entries []*Entry
	// Functions are the entries with no receiver, by name.
	Functions = map[string]*Entry{}
	methods   = map[value.Kind]map[string]*Entry{}
)

// Method returns the method of a kind by name.
func Method(k value.Kind, name string) (*Entry, bool) {
	e, ok := methods[k][name]
	return e, ok
}

// IsMethodName reports whether some kind has a library method of that name.
func IsMethodName(name string) bool {
	for _, m := range methods {
		if _, ok := m[name]; ok {
			return true
		}
	}
	return false
}

func init() {
	if err := json.Unmarshal([]byte(sourceJSON), &Entries); err != nil {
		panic("library: the table does not parse: " + err.Error())
	}
	for _, e := range Entries {
		sig, err := typetext.ParseSig(e.Signature)
		if err != nil {
			panic("library: " + e.Name + ": " + err.Error())
		}
		e.Sig = sig
		if e.Steps == "" || e.Units == "" {
			panic("library: " + e.Name + " has no steps or no units")
		}
		if e.StepsF, err = ParseFormula(e.Steps); err != nil {
			panic("library: " + e.Name + ": " + err.Error())
		}
		if e.UnitsF, err = ParseFormula(e.Units); err != nil {
			panic("library: " + e.Name + ": " + err.Error())
		}
		bounds := map[string]string{}
		for _, b := range sig.Bounds {
			bounds[b.Var] = b.Bound
		}
		for _, p := range sig.Params {
			pp := Param{Name: p.Name, Callback: -1, Rest: p.Rest}
			if p.Fn != nil {
				pp.Callback = len(p.Fn.Params)
				pp.Any = true
			} else {
				t := p.Type
				if p.Rest && len(t.Alts) == 1 && t.Alts[0].Lists > 0 {
					// ...rest: T[] takes arguments of type T.
					a := *t.Alts[0]
					a.Lists--
					t = &typetext.Type{Alts: []*typetext.Alt{&a}}
				}
				pp.Kinds = map[value.Kind]bool{}
				kindsOf(t, bounds, &pp)
			}
			e.Params = append(e.Params, pp)
		}
		if e.Receiver == "" {
			Functions[e.Name] = e
			continue
		}
		switch {
		case e.Receiver == "string":
			e.Kind = value.String
		case strings.HasSuffix(e.Receiver, "[]"):
			e.Kind = value.List
		default:
			e.Kind = value.Map
		}
		if methods[e.Kind] == nil {
			methods[e.Kind] = map[string]*Entry{}
		}
		methods[e.Kind][e.Name] = e
	}
}

func kindsOf(t *typetext.Type, bounds map[string]string, p *Param) {
	hasFloat := false
	for _, a := range t.Alts {
		if a.Lists > 0 {
			p.Kinds[value.List] = true
			continue
		}
		pr := a.Prim
		switch pr.Kind {
		case typetext.PrimWord:
			switch pr.Text {
			case "null":
				p.Kinds[value.Null] = true
			case "bool":
				p.Kinds[value.Bool] = true
			case "int":
				p.Kinds[value.Number] = true
				p.Whole = true
			case "float":
				p.Kinds[value.Number] = true
				hasFloat = true
			case "string":
				p.Kinds[value.String] = true
			case "any", "never":
				p.Any = true
			}
		case typetext.PrimString:
			p.Kinds[value.String] = true
		case typetext.PrimMap, typetext.PrimRecord:
			p.Kinds[value.Map] = true
		case typetext.PrimParen:
			kindsOf(pr.Elem, bounds, p)
		case typetext.PrimVar:
			switch bounds[pr.Text] {
			case "number":
				p.Kinds[value.Number] = true
				hasFloat = true
			case "ordered":
				p.Kinds[value.Number] = true
				p.Kinds[value.String] = true
				p.Ordered = true
				hasFloat = true
			default:
				p.Any = true
			}
		case typetext.PrimName:
			p.Kinds[value.OpaqueKind] = true
		}
	}
	if hasFloat {
		p.Whole = false
	}
}
