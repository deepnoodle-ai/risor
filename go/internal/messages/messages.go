// Package messages builds run messages from the catalog suite/messages.json
// (spec/core/03-host.md 3.8). The catalog is copied into catalog_gen.go by
// internal/gen; no message is written by hand.
package messages

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// Entry is the catalog entry of one code.
type Entry struct {
	Code      string            `json:"code"`
	Phase     string            `json:"phase"`
	Caught    *bool             `json:"caught"`
	Params    map[string]string `json:"params"`
	Templates map[string]string `json:"templates"`
}

var catalog = map[string]*Entry{}

func init() {
	var entries []*Entry
	if err := json.Unmarshal([]byte(sourceJSON), &entries); err != nil {
		panic("messages: the catalog does not parse: " + err.Error())
	}
	for _, e := range entries {
		catalog[e.Code] = e
	}
}

// Lookup returns the catalog entry of a code.
func Lookup(code string) (*Entry, bool) {
	e, ok := catalog[code]
	return e, ok
}

// Caught reports whether catch catches a run code (01-language.md 11.3).
func Caught(code string) bool {
	e, ok := catalog[code]
	return ok && e.Caught != nil && *e.Caught
}

// Codes returns the codes of the catalog, sorted.
func Codes() []string {
	out := make([]string, 0, len(catalog))
	for c := range catalog {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Param is one parameter of a message, before it is rendered.
type Param struct {
	Name string
	// Text is a word, or the rendering of a number for kind number.
	Text string
	// Value is the value for kinds value, string and kind.
	Value    any
	HasValue bool
}

// Word makes a parameter of kind word or number from its text.
func Word(name, text string) Param { return Param{Name: name, Text: text} }

// Val makes a parameter from a value; the catalog's kind renders it.
func Val(name string, v any) Param { return Param{Name: name, Value: v, HasValue: true} }

// Build fills the template of a code with its parameters. The template is
// named; "" names the only template.
func Build(code, template string, params ...Param) string {
	e, ok := catalog[code]
	if !ok {
		return code
	}
	if template == "" {
		template = "default"
	}
	t, ok := e.Templates[template]
	if !ok {
		return fmt.Sprintf("%s (no template %q)", code, template)
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(t, '{')
		if i < 0 {
			b.WriteString(t)
			break
		}
		j := strings.IndexByte(t[i:], '}')
		if j < 0 {
			b.WriteString(t)
			break
		}
		b.WriteString(t[:i])
		name := t[i+1 : i+j]
		b.WriteString(render(e.Params[name], find(params, name)))
		t = t[i+j+1:]
	}
	return b.String()
}

func find(ps []Param, name string) Param {
	for _, p := range ps {
		if p.Name == name {
			return p
		}
	}
	return Param{Name: name, Text: "?"}
}

// render renders a parameter by its kind (03-host.md 3.8.3), cut at 64
// code points.
func render(kind string, p Param) string {
	if !p.HasValue {
		return value.Cut(p.Text)
	}
	v := p.Value
	switch kind {
	case "kind":
		return value.KindOf(v).String()
	case "string":
		if s, ok := v.(string); ok {
			return cutJSON(s)
		}
	case "number":
		if f, ok := v.(float64); ok {
			return value.Cut(value.FormatNumber(f))
		}
	}
	// kind value, and anything else.
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return value.Cut(value.FormatNumber(x))
	case string:
		return cutJSON(x)
	}
	return value.KindOf(v).String()
}

// cutJSON renders the JSON form of a string, cut, reading no more of a long
// string than the cut needs.
func cutJSON(s string) string {
	n := 0
	for i := range s {
		if n == 70 {
			s = s[:i]
			break
		}
		n++
	}
	return value.Cut(value.QuoteJSON(s))
}
