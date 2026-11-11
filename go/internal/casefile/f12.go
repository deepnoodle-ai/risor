package casefile

import (
	"fmt"
	"strings"
)

// The additions of package F12 7 to the case format: the format case, the
// completion case and the hover case.

// Mark is the code point that stands for the position in a completion or
// a hover case (F12 7.2).
const Mark = '‸'

// Tools is the field of a case of package F12: what kind of case it is, its
// text, and what it expects.
type Tools struct {
	// Kind is format, formatTemplate, complete or hover.
	Kind string
	// Text is the field's text. For complete and hover, Source is the text
	// without the mark and Line and Column the mark's position.
	Text, Source string
	Line, Column int

	// Formatted is the expected formatted text, or nil.
	Formatted *string
	// Completion is the expected completion, or nil.
	Completion *CompletionExpect
	// Hover is the expected hover; HasHover is set when the expect has the
	// field hover, and Hover is nil for null.
	Hover    *HoverExpect
	HasHover bool
}

// CompletionExpect is the object C of F12 7.2.
type CompletionExpect struct {
	Span  [4]int
	Items []CandidateExpect
}

// CandidateExpect is one expected candidate. Detail and Insert are nil
// when the candidate does not state them.
type CandidateExpect struct {
	Label, Kind string
	Detail      *NullText
	Insert      *string
}

// NullText is a field that is a string or null.
type NullText struct {
	Null bool
	Text string
}

// HoverExpect is the object R of F12 7.3. A field is nil when R does not
// have it.
type HoverExpect struct {
	Kind                 string
	Span                 [4]int
	Type, Signature, Doc *NullText
	Declaration          *[4]int
	DeclarationNull      bool
	HasDeclaration       bool
}

var (
	toolKinds         = []string{"format", "formatTemplate", "complete", "hover"}
	formatCaseFields  = set("name", "format", "formatTemplate", "env", "vars", "skip", "expect")
	assistCaseFields  = set("name", "complete", "hover", "version", "env", "opaque", "methods", "functions", "removed", "groups", "types", "returns", "skip", "expect")
	candidateFields   = set("label", "kind", "detail", "insert")
	completionFields  = set("span", "items")
	hoverRecordFields = set("kind", "span", "type", "signature", "doc", "declaration")
	candidateKinds    = set("name", "lambda", "variable", "function", "namespace", "field", "method", "constant", "key", "literal", "type")
	hoverKinds        = set("name", "lambda", "variable", "function", "namespace", "field", "method", "constant")
)

// toolKind is the F12 field a case has, or "".
func toolKind(m map[string]any) string {
	for _, k := range toolKinds {
		if _, ok := m[k]; ok {
			return k
		}
	}
	return ""
}

// buildTools builds a case of package F12 (7.1 to 7.4).
func (v *validator) buildTools(m map[string]any, where string) (*Case, error) {
	kind := ""
	for _, k := range toolKinds {
		if _, ok := m[k]; ok {
			if kind != "" {
				return nil, v.invalid("%s has both %s and %s", where, kind, k)
			}
			kind = k
		}
	}
	for _, k := range []string{"source", "file", "conform", "render", "runner"} {
		if _, ok := m[k]; ok {
			return nil, v.invalid("%s has both %s and %s", where, k, kind)
		}
	}
	allowed := assistCaseFields
	if kind == "format" || kind == "formatTemplate" {
		allowed = formatCaseFields
	}
	if err := v.checkFields(m, allowed, where); err != nil {
		return nil, err
	}
	text, ok := m[kind].(string)
	if !ok {
		return nil, v.invalid("%s: %s is not a string", where, kind)
	}
	t := &Tools{Kind: kind, Text: text, Source: text}
	if kind == "complete" || kind == "hover" {
		if strings.Count(text, string(Mark)) != 1 {
			return nil, v.invalid("%s: the mark U+2038 does not stand exactly once in %s", where, kind)
		}
		i := strings.IndexRune(text, Mark)
		t.Source = text[:i] + text[i+len(string(Mark)):]
		before := []rune(strings.TrimPrefix(text[:i], string(rune(0xFEFF))))
		t.Line, t.Column = 1, 1
		for _, r := range before {
			if r == '\n' {
				t.Line++
				t.Column = 1
			} else {
				t.Column++
			}
		}
	}
	// The rest of the case is built as a program case with the text as its
	// source, and the expect read here.
	mm := map[string]any{}
	for k, x := range m {
		if k != kind {
			mm[k] = x
		}
	}
	mm["source"] = t.Source
	exp, _ := m["expect"].(map[string]any)
	if exp == nil {
		return nil, v.invalid("%s: expect is not an object", where)
	}
	if ph, isFail := exp["phase"]; isFail {
		want := "parse"
		if kind == "complete" || kind == "hover" {
			want = "declare"
		}
		if ph != want {
			return nil, v.invalid("%s: a %s case fails only in phase %s", where, kind, want)
		}
	} else {
		mm["expect"] = map[string]any{}
		if err := v.toolsExpect(t, exp, where+": expect"); err != nil {
			return nil, err
		}
	}
	c, err := v.build(mm, where)
	if err != nil {
		return nil, err
	}
	c.ExpectRaw = m["expect"]
	c.Tools = t
	return c, nil
}

func (v *validator) nullText(x any, where, k string) (*NullText, error) {
	switch s := x.(type) {
	case nil:
		return &NullText{Null: true}, nil
	case string:
		return &NullText{Text: s}, nil
	}
	return nil, v.invalid("%s: %s is not a string or null", where, k)
}

// toolsExpect reads a format result, a completion result or a hover result
// (7.1 to 7.3).
func (v *validator) toolsExpect(t *Tools, m map[string]any, where string) error {
	if v.noExpect && len(m) == 0 {
		return nil
	}
	switch t.Kind {
	case "format", "formatTemplate":
		if err := v.checkFields(m, set("formatted"), where); err != nil {
			return err
		}
		s, ok := m["formatted"].(string)
		if !ok {
			return v.invalid("%s: formatted is not a string", where)
		}
		t.Formatted = &s
	case "complete":
		if err := v.checkFields(m, set("completion"), where); err != nil {
			return err
		}
		cm, ok := m["completion"].(map[string]any)
		if !ok {
			return v.invalid("%s: completion is not an object", where)
		}
		if err := v.checkFields(cm, completionFields, where+": completion"); err != nil {
			return err
		}
		sp, err := v.span(cm["span"], where)
		if err != nil {
			return err
		}
		ce := &CompletionExpect{Span: *sp, Items: []CandidateExpect{}}
		items, ok := cm["items"].([]any)
		if !ok {
			return v.invalid("%s: completion has no list items", where)
		}
		for i, x := range items {
			w := fmt.Sprintf("%s: candidate %d", where, i)
			im, ok := x.(map[string]any)
			if !ok {
				return v.invalid("%s is not an object", w)
			}
			if err := v.checkFields(im, candidateFields, w); err != nil {
				return err
			}
			label, ok1 := im["label"].(string)
			kind, ok2 := im["kind"].(string)
			if !ok1 || !ok2 || !candidateKinds[kind] {
				return v.invalid("%s needs a label and a kind", w)
			}
			c := CandidateExpect{Label: label, Kind: kind}
			if d, ok := im["detail"]; ok {
				if c.Detail, err = v.nullText(d, w, "detail"); err != nil {
					return err
				}
			}
			if d, ok := im["insert"]; ok {
				s, ok := d.(string)
				if !ok {
					return v.invalid("%s: insert is not a string", w)
				}
				c.Insert = &s
			}
			ce.Items = append(ce.Items, c)
		}
		t.Completion = ce
	case "hover":
		if err := v.checkFields(m, set("hover"), where); err != nil {
			return err
		}
		x, ok := m["hover"]
		if !ok {
			return v.invalid("%s has no hover", where)
		}
		t.HasHover = true
		if x == nil {
			return nil
		}
		hm, ok := x.(map[string]any)
		if !ok {
			return v.invalid("%s: hover is not null or an object", where)
		}
		if err := v.checkFields(hm, hoverRecordFields, where+": hover"); err != nil {
			return err
		}
		kind, ok := hm["kind"].(string)
		if !ok || !hoverKinds[kind] {
			return v.invalid("%s: hover has no valid kind", where)
		}
		sp, err := v.span(hm["span"], where)
		if err != nil {
			return err
		}
		h := &HoverExpect{Kind: kind, Span: *sp}
		for k, p := range map[string]**NullText{"type": &h.Type, "signature": &h.Signature, "doc": &h.Doc} {
			if y, ok := hm[k]; ok {
				if *p, err = v.nullText(y, where, k); err != nil {
					return err
				}
			}
		}
		if y, ok := hm["declaration"]; ok {
			h.HasDeclaration = true
			if y == nil {
				h.DeclarationNull = true
			} else if h.Declaration, err = v.span(y, where); err != nil {
				return err
			}
		}
		t.Hover = h
	}
	return nil
}
