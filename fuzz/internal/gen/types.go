package gen

import (
	"sort"
	"strings"
)

// Kind is the rough kind the generator tracks for an expression.
type Kind int

// The kinds. KInt is a whole number of small magnitude, safe to multiply.
const (
	KAny Kind = iota
	KNum
	KInt
	KStr
	KBool
	KNull
	KList
	KMap
	KOpq
)

// Ty is a rough type: a kind, the element of a list, and the known fields of
// a map. Len is the known length of a list, or -1.
type Ty struct {
	K      Kind
	Elem   *Ty
	Fields []TField // known keys of a map, sorted; nil for unknown keys
	Len    int
	Opq    string // the opaque type name
}

// TField is a known key of a map and the type of its value.
type TField struct {
	Name string
	T    *Ty
}

var (
	tyAny  = &Ty{K: KAny, Len: -1}
	tyNum  = &Ty{K: KNum, Len: -1}
	tyInt  = &Ty{K: KInt, Len: -1}
	tyStr  = &Ty{K: KStr, Len: -1}
	tyBool = &Ty{K: KBool, Len: -1}
	tyNull = &Ty{K: KNull, Len: -1}
)

func listOf(e *Ty, n int) *Ty { return &Ty{K: KList, Elem: e, Len: n} }

func recordOf(fs []TField) *Ty {
	sort.Slice(fs, func(i, j int) bool { return fs[i].Name < fs[j].Name })
	return &Ty{K: KMap, Fields: fs, Len: -1}
}

func mapAny() *Ty { return &Ty{K: KMap, Len: -1} }

// isNum says whether t is a number of any size.
func (t *Ty) isNum() bool { return t.K == KNum || t.K == KInt }

// ordered says whether values of t can be ordered with < (numbers, strings).
func (t *Ty) ordered() bool { return t.isNum() || t.K == KStr }

// fits says whether a value of type t can stand where want is asked.
func fits(t, want *Ty) bool {
	switch want.K {
	case KAny:
		return t.K != KOpq
	case KNum:
		return t.isNum()
	case KInt:
		return t.K == KInt
	case KList:
		if t.K != KList {
			return false
		}
		if want.Elem == nil || want.Elem.K == KAny {
			return t.Elem == nil || t.Elem.K != KOpq
		}
		return t.Elem != nil && fits(t.Elem, want.Elem)
	case KMap:
		if t.K != KMap {
			return false
		}
		for _, f := range want.Fields {
			g := t.field(f.Name)
			if g == nil || !fits(g, f.T) {
				return false
			}
		}
		return true
	case KOpq:
		return t.K == KOpq && t.Opq == want.Opq
	}
	return t.K == want.K
}

func (t *Ty) field(name string) *Ty {
	for _, f := range t.Fields {
		if f.Name == name {
			return f.T
		}
	}
	return nil
}

// tyFromText reads library type text into a rough type, with the bindings b
// for its type variables. It reports whether the type may be null.
func tyFromText(text string, b map[string]*Ty) (*Ty, bool) {
	alts := splitTop(text, '|')
	nullable := false
	var pick *Ty
	for _, a := range alts {
		a = strings.TrimSpace(a)
		if a == "null" {
			nullable = true
			continue
		}
		if pick == nil {
			pick = tyAlt(a, b)
		}
	}
	if pick == nil {
		return tyNull, true
	}
	return pick, nullable
}

func tyAlt(a string, b map[string]*Ty) *Ty {
	if strings.HasSuffix(a, "[]") {
		inner, _ := tyFromText(strings.TrimSuffix(a, "[]"), b)
		return listOf(inner, -1)
	}
	switch {
	case a == "int":
		return tyInt
	case a == "float":
		return tyNum
	case a == "string":
		return tyStr
	case a == "bool":
		return tyBool
	case a == "any", a == "never", a == "unknown":
		return tyAny
	case strings.HasPrefix(a, "map<"):
		return mapAny()
	case strings.HasPrefix(a, "{"):
		var fs []TField
		for _, f := range splitTop(a[1:len(a)-1], ',') {
			kv := strings.SplitN(f, ":", 2)
			if len(kv) == 2 {
				t, _ := tyFromText(strings.TrimSpace(kv[1]), b)
				fs = append(fs, TField{Name: strings.TrimSpace(kv[0]), T: t})
			}
		}
		return recordOf(fs)
	case strings.HasPrefix(a, "(") && strings.HasSuffix(a, ")"):
		t, _ := tyFromText(a[1:len(a)-1], b)
		return t
	case len(a) == 1 && a[0] >= 'A' && a[0] <= 'Z':
		if t, ok := b[a]; ok && t != nil {
			return t
		}
		return tyAny
	}
	return tyAny
}

// typeText writes the env type text of a host value.
func typeText(v any, precise bool) string {
	if !precise {
		return "any"
	}
	switch v := v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		if v == float64(int64(v)) && v <= maxWhole && v >= -maxWhole {
			return "int"
		}
		return "float"
	case string:
		return "string"
	case []any:
		return "any[]"
	case map[string]any:
		if _, ok := v["$opaque"]; ok {
			return v["$opaque"].(map[string]any)["type"].(string)
		}
		keys := sortedKeys(v)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = typeKey(k) + ": " + typeText(v[k], false)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "any"
}

func typeKey(k string) string {
	if isWord(k) {
		return k
	}
	return jsonString(k)
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		ok := c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9'
		if !ok {
			return false
		}
	}
	return true
}

// isName says whether s is a Name: a word that is not reserved and not _.
func isName(s string) bool {
	return isWord(s) && s != "_" && !reserved[s]
}

var reserved = map[string]bool{
	"true": true, "false": true, "null": true, "const": true, "if": true,
	"else": true, "in": true, "not": true, "try": true, "catch": true,
	"match": true, "as": true, "is": true, "nil": true, "let": true,
	"var": true, "for": true, "while": true, "do": true, "break": true,
	"continue": true, "return": true, "function": true, "throw": true,
	"finally": true, "import": true,
}

// reservedList is the 26 reserved words in a fixed order.
var reservedList = []string{
	"true", "false", "null", "const", "if", "else", "in", "not", "try",
	"catch", "match", "as", "is", "nil", "let", "var", "for", "while", "do",
	"break", "continue", "return", "function", "throw", "finally", "import",
}

// tyOfValue gives the rough type of a host value.
func tyOfValue(v any) *Ty {
	switch v := v.(type) {
	case nil:
		return tyNull
	case bool:
		return tyBool
	case float64:
		if v == float64(int64(v)) && v <= 1000 && v >= -1000 {
			return tyInt
		}
		return tyNum
	case string:
		return tyStr
	case []any:
		var elem *Ty
		for _, x := range v {
			t := tyOfValue(x)
			switch {
			case elem == nil:
				elem = t
			case elem.isNum() && t.isNum() && elem.K != t.K:
				elem = tyNum
			case !sameKind(elem, t):
				elem = tyAny
			}
		}
		if elem == nil {
			elem = tyAny
		}
		return listOf(elem, len(v))
	case map[string]any:
		if o, ok := v["$opaque"]; ok {
			return &Ty{K: KOpq, Opq: o.(map[string]any)["type"].(string), Len: -1}
		}
		var fs []TField
		for _, k := range sortedKeys(v) {
			fs = append(fs, TField{Name: k, T: tyOfValue(v[k])})
		}
		return recordOf(fs)
	}
	return tyAny
}

func sameKind(a, b *Ty) bool {
	if a.K == KAny || b.K == KAny {
		return false
	}
	if a.isNum() && b.isNum() {
		return a.K == b.K
	}
	if a.K != b.K {
		return false
	}
	if a.K == KList {
		return a.Elem != nil && b.Elem != nil && sameKind(a.Elem, b.Elem)
	}
	if a.K == KMap {
		return false
	}
	return true
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
