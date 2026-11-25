// Package outcome reads the lines of an implementation's outcome command
// (suite/README.md) and compares them.
package outcome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Line is one line of an outcome command.
type Line struct {
	Name     string
	Type     string // the type the check gave; "" when absent
	HasType  bool
	Sites    []any // the sites the check gave, each {"class", "span"}
	HasSites bool
	Phase    string // "" when phase is null or absent
	HasPhase bool
	Value    any
	HasValue bool
	Diags    []map[string]any
	// Description (package F1) and Analysis (package F2), when the
	// implementation writes them; Has* is false when it does not.
	Description, Analysis       any
	HasDescription, HasAnalysis bool
	Steps                       any // nil when absent
	Units                       any
	// Fields holds every decoded top-level JSON field on the line so Diff
	// compares every field emitted by any outcome line shape (conform, render,
	// runner, tools, etc.).
	Fields map[string]any
	Raw    string
}

// nonPhaseKeys are top-level keys that identify valid outcome lines without a
// phase field: render (F8), runner (F9), and successful tools cases (F12).
var nonPhaseKeys = []string{"line", "invalid", "verdicts", "formatted", "completion", "hover"}

// Parse reads one line.
func Parse(raw string) (*Line, error) {
	var m map[string]json.RawMessage
	dec := json.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("text after the object")
	}
	l := &Line{Fields: make(map[string]any, len(m)), Raw: raw}
	for k, v := range m {
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		l.Fields[k] = val
	}
	get := func(k string, into any) error {
		v, ok := m[k]
		if !ok {
			return nil
		}
		return json.Unmarshal(v, into)
	}
	if err := get("name", &l.Name); err != nil {
		return nil, fmt.Errorf("name: %w", err)
	}
	if _, ok := m["name"]; !ok {
		return nil, fmt.Errorf("no name")
	}
	if v, ok := m["type"]; ok {
		l.HasType = true
		if err := json.Unmarshal(v, &l.Type); err != nil {
			return nil, fmt.Errorf("type: %w", err)
		}
	}
	if v, ok := m["sites"]; ok {
		l.HasSites = true
		if err := json.Unmarshal(v, &l.Sites); err != nil {
			return nil, fmt.Errorf("sites: %w", err)
		}
	}
	var phase *string
	if err := get("phase", &phase); err != nil {
		return nil, fmt.Errorf("phase: %w", err)
	}
	if _, ok := m["phase"]; ok {
		l.HasPhase = true
		if phase != nil {
			l.Phase = *phase
		}
	} else {
		valid := false
		for _, k := range nonPhaseKeys {
			if _, ok := m[k]; ok {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("no phase")
		}
	}
	if v, ok := m["value"]; ok {
		l.HasValue = true
		if err := json.Unmarshal(v, &l.Value); err != nil {
			return nil, fmt.Errorf("value: %w", err)
		}
	}
	if err := get("diagnostics", &l.Diags); err != nil {
		return nil, fmt.Errorf("diagnostics: %w", err)
	}
	if v, ok := m["description"]; ok {
		l.HasDescription = true
		if err := json.Unmarshal(v, &l.Description); err != nil {
			return nil, fmt.Errorf("description: %w", err)
		}
	}
	if v, ok := m["analysis"]; ok {
		l.HasAnalysis = true
		if err := json.Unmarshal(v, &l.Analysis); err != nil {
			return nil, fmt.Errorf("analysis: %w", err)
		}
	}
	if err := get("steps", &l.Steps); err != nil {
		return nil, err
	}
	if err := get("units", &l.Units); err != nil {
		return nil, err
	}
	return l, nil
}

// ParseAll reads the standard output of one run of a command.
func ParseAll(out []byte) ([]*Line, error) {
	var lines []*Line
	for i, raw := range bytes.Split(out, []byte("\n")) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		l, err := Parse(string(raw))
		if err != nil {
			return lines, fmt.Errorf("line %d: %w", i+1, err)
		}
		lines = append(lines, l)
	}
	return lines, nil
}

var limitCodes = map[string]bool{
	"steps-exceeded": true, "allocation-exceeded": true, "depth-exceeded": true,
	"deadline-exceeded": true, "cancelled": true,
}

// IsLimit says whether a line is a run failure of class limit.
func (l *Line) IsLimit() bool {
	if l.Phase != "run" || len(l.Diags) != 1 {
		return false
	}
	code, _ := l.Diags[0]["code"].(string)
	class, _ := l.Diags[0]["class"].(string)
	return limitCodes[code] || class == "limit"
}

// Code gives the code of the first diagnostic, or "".
func (l *Line) Code() string {
	if len(l.Diags) == 0 {
		return ""
	}
	c, _ := l.Diags[0]["code"].(string)
	return c
}

// Kind names the outcome: "value", or the phase and the first code.
func (l *Line) Kind() string {
	if l.Phase == "" {
		return "value"
	}
	return l.Phase + ":" + l.Code()
}

// canonicalDiagFields is the order in which known diagnostic fields are compared.
var canonicalDiagFields = []string{"code", "span", "class", "message", "path", "trace"}

// canonicalExtraTopFields are known top-level fields beyond the core program
// fields, compared in deterministic order (conform, render, runner, tools).
var canonicalExtraTopFields = []string{
	"conforms", "path", "message", "agree", "failures",
	"line", "detail", "json",
	"invalid", "verdicts", "report", "text",
	"formatted", "completion", "hover",
}

// Diff compares two lines as suite/README.md says: equal apart from the
// spelling of a number, across every field written on the line. For a run
// failure of class limit the span, trace and usage are not fixed (03-host.md
// 3.6.1 item 4, 3.6.3 item 3; suite/README.md; f08-errors.md 8), so only the
// fixed fields are compared. It gives "" when the lines agree, else the name
// of the first field that differs.
func Diff(a, b *Line) string {
	if a.Name != b.Name {
		return "name"
	}
	if a.HasDescription != b.HasDescription || (a.HasDescription && !Equal(a.Description, b.Description)) {
		return "description"
	}
	if a.HasType != b.HasType || a.Type != b.Type {
		return "type"
	}
	if a.HasSites != b.HasSites || !Equal(a.Sites, b.Sites) {
		return "sites"
	}
	if a.HasPhase != b.HasPhase || a.Phase != b.Phase {
		return "phase"
	}
	if a.HasValue != b.HasValue || !Equal(a.Value, b.Value) {
		return "value"
	}
	_, hasDiagsA := a.Fields["diagnostics"]
	_, hasDiagsB := b.Fields["diagnostics"]
	if hasDiagsA != hasDiagsB || len(a.Diags) != len(b.Diags) {
		return "diagnostics.count"
	}
	limit := a.IsLimit() || b.IsLimit()
	for i := range a.Diags {
		if d := diffDiag(a.Diags[i], b.Diags[i], limit); d != "" {
			return "diagnostics." + d
		}
	}
	checked := map[string]bool{
		"name": true, "description": true, "type": true, "sites": true,
		"phase": true, "value": true, "diagnostics": true,
		"steps": true, "units": true, "analysis": true,
	}
	for _, k := range canonicalExtraTopFields {
		checked[k] = true
		if !sameField(a.Fields, b.Fields, k) {
			return k
		}
	}
	if !limit {
		if !sameField(a.Fields, b.Fields, "steps") {
			return "steps"
		}
		if !sameField(a.Fields, b.Fields, "units") {
			return "units"
		}
	}
	if a.HasAnalysis != b.HasAnalysis || (a.HasAnalysis && !Equal(a.Analysis, b.Analysis)) {
		return "analysis"
	}
	var other []string
	for k := range a.Fields {
		if !checked[k] {
			other = append(other, k)
		}
	}
	for k := range b.Fields {
		if !checked[k] {
			if _, inA := a.Fields[k]; !inA {
				other = append(other, k)
			}
		}
	}
	sort.Strings(other)
	for _, k := range other {
		if !sameField(a.Fields, b.Fields, k) {
			return k
		}
	}
	return ""
}

func sameField(a, b map[string]any, k string) bool {
	va, okA := a[k]
	vb, okB := b[k]
	if okA != okB {
		return false
	}
	if !okA {
		return true
	}
	return Equal(va, vb)
}

func diffDiag(a, b map[string]any, limit bool) string {
	checked := make(map[string]bool, len(canonicalDiagFields))
	for _, f := range canonicalDiagFields {
		checked[f] = true
		if limit && (f == "span" || f == "trace") {
			continue
		}
		if !sameField(a, b, f) {
			return f
		}
	}
	var other []string
	for k := range a {
		if !checked[k] {
			other = append(other, k)
		}
	}
	for k := range b {
		if !checked[k] {
			if _, inA := a[k]; !inA {
				other = append(other, k)
			}
		}
	}
	sort.Strings(other)
	for _, k := range other {
		if !sameField(a, b, k) {
			return k
		}
	}
	return ""
}

// Equal compares two decoded JSON values. Numbers compare as doubles, so 2
// and 2.0 are equal, and so are 0 and -0; keys compare as sets.
func Equal(a, b any) bool {
	switch a := a.(type) {
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && a == y
	case float64:
		y, ok := b.(float64)
		return ok && a == y
	case string:
		y, ok := b.(string)
		return ok && a == y
	case []any:
		y, ok := b.([]any)
		if !ok || len(a) != len(y) {
			return false
		}
		for i := range a {
			if !Equal(a[i], y[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(a) != len(y) {
			return false
		}
		for k, v := range a {
			w, ok := y[k]
			if !ok || !Equal(v, w) {
				return false
			}
		}
		return true
	}
	return false
}

var noMethod = regexp.MustCompile(`has no method (\w+)$`)

// MissingEntry gives the name of a library entry a line says the
// implementation does not have, or "". It reads the name of an
// undefined-function from the source text at its span, and the name of a
// method from the message of a wrong-kind failure.
func (l *Line) MissingEntry(source string, lib map[string]bool) string {
	if len(l.Diags) == 0 {
		return ""
	}
	d := l.Diags[0]
	switch {
	case l.Phase == "check" && d["code"] == "undefined-function":
		name := spanText(source, d["span"])
		if i := strings.IndexByte(name, '('); i >= 0 {
			name = name[:i]
		}
		if lib[name] {
			return name
		}
	case l.Phase == "run" && d["code"] == "wrong-kind":
		msg, _ := d["message"].(string)
		if m := noMethod.FindStringSubmatch(msg); m != nil && lib[m[1]] {
			return m[1]
		}
	}
	return ""
}

// spanText gives the text of a span [l1, c1, l2, c2]: lines end at U+000A,
// columns count code points (03-host.md 3.7.7).
func spanText(src string, span any) string {
	s, ok := span.([]any)
	if !ok || len(s) != 4 {
		return ""
	}
	n := make([]int, 4)
	for i := range s {
		f, ok := s[i].(float64)
		if !ok {
			return ""
		}
		n[i] = int(f)
	}
	var out []rune
	line, col := 1, 1
	for _, r := range src {
		after := line > n[0] || line == n[0] && col >= n[1]
		before := line < n[2] || line == n[2] && col < n[3]
		if after && before {
			out = append(out, r)
		}
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return strings.TrimSpace(string(out))
}
