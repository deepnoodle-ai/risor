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
	// Narrows names the callback parameter whose true facts narrow the
	// result (filter, find); Tested lists the tested argument positions.
	Narrows string `json:"narrows"`
	Tested  []int  `json:"tested"`
	// Renders names the parameters whose argument is rendered as text.
	Renders []string `json:"renders"`
	// Group is the entry's group: core, or an opt-in group of package F6.
	Group string `json:"group"`
	// Type and Value are a constant's (F6 3.3); a constant has no
	// signature.
	Type  string   `json:"type"`
	Value *float64 `json:"value"`
	// Extends names the Core entry a record of package F7 takes the place
	// of: list.sortBy.
	Extends string `json:"extends"`
	// Doc is the record's doc, which hover shows (package F12 3.13).
	Doc string `json:"doc"`

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
	// Lits is the literal set of a parameter whose type is one, as order of
	// sortBy (f07-queries.md 6.2). Nil for any other parameter.
	Lits []string
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

	// GroupEntries are the function records of suite/library-groups.json,
	// in its order (package F6 3).
	GroupEntries []*Entry
	// GroupFunctions are those by full name: json.parse.
	GroupFunctions = map[string]*Entry{}
	// Constants are the constant records by full name: math.pi.
	Constants = map[string]*Entry{}
	// Groups are the opt-in groups of F6.
	Groups = map[string]bool{"json": true, "time": true, "math": true, "base64": true}
	// AllEntries are the functions and methods of both files.
	AllEntries []*Entry
)

// Namespace returns the group of a full name, json for json.parse, or "".
func Namespace(name string) string {
	if i := strings.IndexByte(name, '.'); i > 0 {
		return name[:i]
	}
	return ""
}

// Method returns the method of a kind by name.
func Method(k value.Kind, name string) (*Entry, bool) {
	e, ok := methods[k][name]
	return e, ok
}

// KindWord is the word that names a method's receiver kind in an entry name
// (03-host.md 3.3.1): string, list or map.
func KindWord(k value.Kind) string {
	switch k {
	case value.String:
		return "string"
	case value.List:
		return "list"
	case value.Map:
		return "map"
	}
	return ""
}

// KindWordOf is the kind word of a receiver's type text: string, list or
// map.
func KindWordOf(recv string) string {
	switch {
	case recv == "string":
		return "string"
	case strings.HasSuffix(recv, "[]"):
		return "list"
	}
	return "map"
}

// EntryName is an entry's name as a removal names it: len, string.repeat.
func (e *Entry) EntryName() string {
	if e.IsMethod() {
		return KindWord(e.Kind) + "." + e.Name
	}
	return e.Name
}

// IsEntryName reports whether a name names an entry (03-host.md 3.3.1).
func IsEntryName(n string) bool {
	if _, ok := Functions[n]; ok {
		return true
	}
	if _, ok := GroupFunctions[n]; ok { // F6 2.6: by the full name
		return true
	}
	if _, ok := Constants[n]; ok {
		return true
	}
	for k, m := range methods {
		for name := range m {
			if KindWord(k)+"."+name == n {
				return true
			}
		}
	}
	return false
}

// MethodKinds returns the kinds that have a library method of that name.
func MethodKinds(name string) []value.Kind {
	var out []value.Kind
	for _, k := range []value.Kind{value.String, value.List, value.Map} {
		if _, ok := methods[k][name]; ok {
			out = append(out, k)
		}
	}
	return out
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
	var groups []*Entry
	if err := json.Unmarshal([]byte(sourceGroupsJSON), &groups); err != nil {
		panic("library: the groups table does not parse: " + err.Error())
	}
	for _, e := range groups {
		if (e.Type == "") == (e.Signature == "") {
			panic("library: " + e.Name + " needs exactly one of type and signature")
		}
		if e.Type != "" {
			if e.Value == nil {
				panic("library: the constant " + e.Name + " has no value")
			}
			Constants[e.Name] = e
			continue
		}
		GroupEntries = append(GroupEntries, e)
	}
	// Package F13's records (f13-math.md 5): eleven functions of the group
	// math, beside F6's.
	var maths []*Entry
	if err := json.Unmarshal([]byte(sourceMathJSON), &maths); err != nil {
		panic("library: the math table does not parse: " + err.Error())
	}
	GroupEntries = append(GroupEntries, maths...)
	// Package F7's records (f07-queries.md 6.1): a record with extends takes
	// the place of the Core record; the others are new entries.
	var queries []*Entry
	if err := json.Unmarshal([]byte(sourceQueriesJSON), &queries); err != nil {
		panic("library: the queries table does not parse: " + err.Error())
	}
	for _, q := range queries {
		if q.Extends == "" {
			Entries = append(Entries, q)
			continue
		}
		replaced := false
		for i, e := range Entries {
			if e.Receiver != "" && KindWordOf(e.Receiver)+"."+e.Name == q.Extends {
				if e.Name != q.Name || e.Receiver != q.Receiver || e.Steps != q.Steps || e.Units != q.Units {
					panic("library: " + q.Name + " does not extend " + q.Extends + " by the rules of F7 6.1")
				}
				Entries[i] = q
				replaced = true
			}
		}
		if !replaced {
			panic("library: " + q.Extends + " is not a Core entry")
		}
	}
	AllEntries = append(append([]*Entry(nil), Entries...), GroupEntries...)
	for _, e := range AllEntries {
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
			if e.Group != "" && e.Group != "core" {
				GroupFunctions[e.Name] = e
			} else {
				Functions[e.Name] = e
			}
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
			p.Lits = append(p.Lits, pr.Text)
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
