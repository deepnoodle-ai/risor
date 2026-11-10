// Package casefile reads case files (spec/core/03-host.md 3.10): it decodes
// the JSON, validates the file as 3.10.10 and 3.10.11 say, and lays each case
// over the header.
package casefile

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
)

// File is a valid case file.
type File struct {
	Path  string
	Cases []*Case
}

// Case is one case, with the header laid over it (3.10.2).
type Case struct {
	Name string
	// Source is the program text, read from File when the case gives a file.
	Source string
	// File is the path the case gave, or empty.
	File string

	Version      string // empty when absent
	Env          *string
	Opaque       []string
	Methods      map[string]*Stub
	Functions    map[string]*Stub
	Reject       []string
	HasReject    bool
	Returns      *string
	Vars         map[string]any // nil when absent
	Limits       *Limits
	Skip         *string
	Precondition *bool
	// Check is false for a case compiled with the static rules alone
	// (ruling AP1). Nil means true.
	Check  *bool
	Expect *Expect
}

// Limits is the field limits of a case.
type Limits struct {
	Steps, Allocation, Depth *int64
}

// Stub is a host function given as a table of rows (3.10.4).
type Stub struct {
	Type  *string
	Calls []*Row
}

// Row is one row of a stub.
type Row struct {
	Args     []any
	HasArgs  bool
	Result   any
	Error    *string
	Charge   int64
	Allocate int64
}

// Expect is the field expect: a result, or a failure when Phase is set.
type Expect struct {
	Phase       string // empty for a result
	Diagnostics []*ExpDiag

	Type      *string
	Sites     []Site
	HasSites  bool
	Value     any
	HasValue  bool
	Truthy    *bool
	Steps     *int64
	Allocated *int64
}

// IsFailure reports whether the expect is a failure.
func (e *Expect) IsFailure() bool { return e.Phase != "" }

// Site is one site of a result.
type Site struct {
	Class string
	Span  [4]int
}

// ExpDiag is one expected diagnostic.
type ExpDiag struct {
	Code    string
	Class   *string
	Span    *[4]int
	Path    []any
	HasPath bool
	Message *string
	Data    any
	HasData bool
}

// InvalidError is the reason a file is invalid. Every case of the file then
// has the verdict invalid.
type InvalidError struct {
	Path   string
	Reason string
	// Names are the names of the cases that could be read, for reporting.
	Names []string
}

// Error returns the path and the reason.
func (e *InvalidError) Error() string { return e.Path + ": invalid: " + e.Reason }

// Load reads and validates a case file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &InvalidError{Path: path, Reason: err.Error()}
	}
	return Parse(data, path)
}

// Parse validates the bytes of a case file. path names the file, and its
// directory is where a case's file field is read from.
func Parse(data []byte, path string) (*File, error) {
	raw, err := DecodeJSON(data)
	if err != nil {
		return nil, &InvalidError{Path: path, Reason: err.Error()}
	}
	return FromValue(raw, path)
}

// LoadNoExpect reads a case file in which a case may have no expect, as the
// outcome command of ruling AJ takes.
func LoadNoExpect(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &InvalidError{Path: path, Reason: err.Error()}
	}
	raw, err := DecodeJSON(data)
	if err != nil {
		return nil, &InvalidError{Path: path, Reason: err.Error()}
	}
	v := &validator{path: path, dir: filepath.Dir(path), noExpect: true}
	return v.file(raw)
}

// FromValue validates a decoded case file, or a list of cases with no
// header such as the cases of a library.json entry.
func FromValue(raw any, path string) (*File, error) {
	v := &validator{path: path, dir: filepath.Dir(path)}
	f, err := v.file(raw)
	if err != nil {
		err.(*InvalidError).Names = v.names
		return nil, err
	}
	return f, nil
}

type validator struct {
	// noExpect accepts a case with no expect (the outcome command).
	noExpect bool
	path     string
	dir      string
	names    []string
}

func (v *validator) invalid(format string, args ...any) error {
	return &InvalidError{Path: v.path, Reason: fmt.Sprintf(format, args...)}
}

var (
	headerFields = set("cases", "source", "file", "version", "env", "opaque", "methods",
		"functions", "reject", "returns", "vars", "limits", "precondition", "check")
	caseFields = set("name", "source", "file", "version", "env", "opaque", "methods",
		"functions", "reject", "returns", "vars", "limits", "skip", "expect", "precondition", "check")
	resultFields  = set("type", "sites", "value", "truthy", "steps", "allocated")
	failureFields = set("phase", "diagnostics", "steps", "allocated")
	diagFields    = set("code", "class", "span", "path", "message", "data")
	stubFields    = set("type", "calls")
	rowFields     = set("args", "result", "error", "charge", "allocate")
	limitFields   = set("steps", "allocation", "depth")
	siteFields    = set("class", "span")
	phases        = set("declare", "parse", "check", "run")
	siteClasses   = set("dynamic", "index", "divide", "value", "host")
	diagClasses   = set("constant-condition", "dynamic", "index", "divide", "value", "host",
		"environment", "range", "limit")

	versionRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	methodRe  = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]+\.[A-Za-z_][A-Za-z0-9_]*$`)
	codeRe    = regexp.MustCompile(`^[a-z]+(-[a-z0-9]+)*$`)
)

func set(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func (v *validator) checkFields(obj map[string]any, allowed map[string]bool, where string) error {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !allowed[k] {
			return v.invalid("%s has the field %q, which is not allowed", where, k)
		}
	}
	return nil
}

func (v *validator) file(raw any) (*File, error) {
	var header map[string]any
	var cases []any
	switch r := raw.(type) {
	case []any:
		cases = r
	case map[string]any:
		if err := v.checkFields(r, headerFields, "the header"); err != nil {
			return nil, err
		}
		c, ok := r["cases"]
		if !ok {
			return nil, v.invalid("the file object has no field cases")
		}
		if cases, ok = c.([]any); !ok {
			return nil, v.invalid("cases is not an array")
		}
		header = map[string]any{}
		for k, val := range r {
			if k != "cases" {
				header[k] = val
			}
		}
		if err := v.fields(header, "the header"); err != nil {
			return nil, err
		}
	default:
		return nil, v.invalid("the file is neither an object with cases nor an array")
	}
	// Collect names first, so that an invalid file still reports them.
	for _, c := range cases {
		if m, ok := c.(map[string]any); ok {
			if n, ok := m["name"].(string); ok {
				v.names = append(v.names, n)
			}
		}
	}
	f := &File{Path: v.path}
	seen := map[string]bool{}
	for i, c := range cases {
		m, ok := c.(map[string]any)
		if !ok {
			return nil, v.invalid("case %d is not an object", i)
		}
		where := fmt.Sprintf("case %d", i)
		if err := v.checkFields(m, caseFields, where); err != nil {
			return nil, err
		}
		name, ok := m["name"].(string)
		if !ok {
			return nil, v.invalid("%s has no name", where)
		}
		where = fmt.Sprintf("case %q", name)
		if seen[name] {
			return nil, v.invalid("the name %q is used twice", name)
		}
		seen[name] = true
		if err := v.fields(m, where); err != nil {
			return nil, err
		}
		if s, ok := m["skip"]; ok {
			if _, ok := s.(string); !ok {
				return nil, v.invalid("%s: skip is not a string", where)
			}
		}
		if _, ok := m["expect"]; !ok {
			if !v.noExpect {
				return nil, v.invalid("%s has no expect", where)
			}
			m["expect"] = map[string]any{}
		}
		merged := map[string]any{}
		for k, val := range header {
			merged[k] = val
		}
		_, hasSource := m["source"]
		_, hasFile := m["file"]
		if hasSource || hasFile {
			delete(merged, "source")
			delete(merged, "file")
		}
		for k, val := range m {
			merged[k] = val
		}
		cs, err := v.build(merged, where)
		if err != nil {
			return nil, err
		}
		f.Cases = append(f.Cases, cs)
	}
	return f, nil
}

func isWhole(x any) (int64, bool) {
	f, ok := x.(float64)
	if !ok || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, false
	}
	return int64(f), true
}

func stringList(x any) ([]string, bool) {
	a, ok := x.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(a))
	for i, e := range a {
		s, ok := e.(string)
		if !ok {
			return nil, false
		}
		out[i] = s
	}
	return out, true
}

// fields checks the JSON types of the fields that the header and a case
// share (the schema's $defs/fields).
func (v *validator) fields(m map[string]any, where string) error {
	for _, k := range []string{"source", "file", "version", "env", "returns"} {
		if x, ok := m[k]; ok {
			s, ok := x.(string)
			if !ok {
				return v.invalid("%s: %s is not a string", where, k)
			}
			if k == "file" && s == "" {
				return v.invalid("%s: file is empty", where)
			}
			if k == "version" && !versionRe.MatchString(s) {
				return v.invalid("%s: version %q is not major.minor", where, s)
			}
		}
	}
	for _, k := range []string{"opaque", "reject"} {
		if x, ok := m[k]; ok {
			if _, ok := stringList(x); !ok {
				return v.invalid("%s: %s is not a list of strings", where, k)
			}
		}
	}
	if x, ok := m["methods"]; ok {
		mm, ok := x.(map[string]any)
		if !ok {
			return v.invalid("%s: methods is not an object", where)
		}
		for k, s := range mm {
			if !methodRe.MatchString(k) {
				return v.invalid("%s: the method name %q is not Type.method", where, k)
			}
			if err := v.stub(s, where+": method "+k); err != nil {
				return err
			}
		}
	}
	if x, ok := m["functions"]; ok {
		fm, ok := x.(map[string]any)
		if !ok {
			return v.invalid("%s: functions is not an object", where)
		}
		for name, s := range fm {
			if err := v.stub(s, where+": function "+name); err != nil {
				return err
			}
		}
	}
	if x, ok := m["vars"]; ok {
		if _, ok := x.(map[string]any); !ok {
			return v.invalid("%s: vars is not an object", where)
		}
	}
	if x, ok := m["limits"]; ok {
		lm, ok := x.(map[string]any)
		if !ok {
			return v.invalid("%s: limits is not an object", where)
		}
		if err := v.checkFields(lm, limitFields, where+": limits"); err != nil {
			return err
		}
		for k, val := range lm {
			n, ok := isWhole(val)
			if !ok || n < 0 || (k == "depth" && (n < 1 || n > 1000)) {
				return v.invalid("%s: limits.%s is out of range", where, k)
			}
		}
	}
	for _, k := range []string{"precondition", "check"} {
		if x, ok := m[k]; ok {
			if _, ok := x.(bool); !ok {
				return v.invalid("%s: %s is not a boolean", where, k)
			}
		}
	}
	return nil
}

func (v *validator) stub(s any, where string) error {
	switch st := s.(type) {
	case string:
		return nil
	case map[string]any:
		if err := v.checkFields(st, stubFields, where); err != nil {
			return err
		}
		if t, ok := st["type"]; ok {
			if _, ok := t.(string); !ok {
				return v.invalid("%s: type is not a string", where)
			}
		}
		if c, ok := st["calls"]; ok {
			rows, ok := c.([]any)
			if !ok {
				return v.invalid("%s: calls is not an array", where)
			}
			for i, r := range rows {
				rm, ok := r.(map[string]any)
				if !ok {
					return v.invalid("%s: row %d is not an object", where, i)
				}
				w := fmt.Sprintf("%s: row %d", where, i)
				if err := v.checkFields(rm, rowFields, w); err != nil {
					return err
				}
				if a, ok := rm["args"]; ok {
					if _, ok := a.([]any); !ok {
						return v.invalid("%s: args is not an array", w)
					}
				}
				_, hasResult := rm["result"]
				e, hasError := rm["error"]
				if hasResult == hasError {
					return v.invalid("%s must have exactly one of result and error", w)
				}
				if hasError {
					if _, ok := e.(string); !ok {
						return v.invalid("%s: error is not a string", w)
					}
				}
				for _, k := range []string{"charge", "allocate"} {
					if x, ok := rm[k]; ok {
						if n, ok := isWhole(x); !ok || n < 0 {
							return v.invalid("%s: %s is not a whole number of 0 or more", w, k)
						}
					}
				}
			}
		}
		return nil
	}
	return v.invalid("%s is neither a signature nor an object", where)
}

func optString(m map[string]any, k string) *string {
	if x, ok := m[k]; ok {
		s := x.(string)
		return &s
	}
	return nil
}

// build makes a Case from the merged fields, whose JSON types are checked.
func (v *validator) build(m map[string]any, where string) (*Case, error) {
	c := &Case{Name: m["name"].(string)}
	src, hasSource := m["source"]
	file, hasFile := m["file"]
	if hasSource == hasFile {
		return nil, v.invalid("%s must have exactly one of source and file", where)
	}
	if hasSource {
		c.Source = src.(string)
	} else {
		c.File = file.(string)
		data, err := os.ReadFile(filepath.Join(v.dir, filepath.FromSlash(c.File)))
		if err != nil {
			return nil, v.invalid("%s: the file %q cannot be read", where, c.File)
		}
		c.Source = string(data)
	}
	if s := optString(m, "version"); s != nil {
		c.Version = *s
	}
	c.Env = optString(m, "env")
	c.Returns = optString(m, "returns")
	c.Skip = optString(m, "skip")
	if x, ok := m["opaque"]; ok {
		c.Opaque, _ = stringList(x)
	}
	if x, ok := m["reject"]; ok {
		c.Reject, _ = stringList(x)
		c.HasReject = true
	}
	if x, ok := m["methods"]; ok {
		c.Methods = map[string]*Stub{}
		for k, s := range x.(map[string]any) {
			st, err := v.buildStub(s, where, k, c.Opaque)
			if err != nil {
				return nil, err
			}
			c.Methods[k] = st
		}
	}
	if x, ok := m["precondition"]; ok {
		b := x.(bool)
		c.Precondition = &b
	}
	if x, ok := m["check"]; ok {
		b := x.(bool)
		c.Check = &b
	}
	if x, ok := m["limits"]; ok {
		c.Limits = &Limits{}
		for k, val := range x.(map[string]any) {
			n, _ := isWhole(val)
			switch k {
			case "steps":
				c.Limits.Steps = &n
			case "allocation":
				c.Limits.Allocation = &n
			case "depth":
				c.Limits.Depth = &n
			}
		}
	}
	if x, ok := m["functions"]; ok {
		c.Functions = map[string]*Stub{}
		for name, s := range x.(map[string]any) {
			st, err := v.buildStub(s, where, name, c.Opaque)
			if err != nil {
				return nil, err
			}
			c.Functions[name] = st
		}
	}
	if x, ok := m["vars"]; ok {
		cv, err := v.opaque(x, c.Opaque, where)
		if err != nil {
			return nil, err
		}
		c.Vars = cv.(map[string]any)
		// 3.10.10 item 9: a key of vars that is not a field of env. When env
		// does not fit the grammar, the case fails at declare instead.
		var fields map[string]bool
		if c.Env == nil {
			fields = map[string]bool{}
		} else if ty, err := typetext.ParseType(*c.Env); err == nil {
			if r, ok := ty.RecordFields(); ok {
				fields = map[string]bool{}
				for _, f := range r.Fields {
					fields[f.Key] = true
				}
			}
		}
		if fields != nil {
			for k := range c.Vars {
				if !fields[k] {
					return nil, v.invalid("%s: vars has %q, which env does not declare", where, k)
				}
			}
		}
	}
	e, err := v.expect(m["expect"], where)
	if err != nil {
		return nil, err
	}
	if e.HasValue {
		if e.Value, err = v.opaque(e.Value, c.Opaque, where); err != nil {
			return nil, err
		}
	}
	// Ruling AP1: a case with "check": false states no type and no sites,
	// and a check failure it expects has static codes only.
	if c.Check != nil && !*c.Check {
		if e.Type != nil || e.HasSites {
			return nil, v.invalid("%s: a case with \"check\": false states no type and no sites", where)
		}
		if e.Phase == "check" {
			for _, d := range e.Diagnostics {
				if !StaticCodes[d.Code] {
					return nil, v.invalid("%s: a case with \"check\": false expects static codes only, not %s", where, d.Code)
				}
			}
		}
	}
	if (e.Phase == "parse" || e.Phase == "declare") && len(e.Diagnostics) > 1 {
		return nil, v.invalid("%s: a failure of phase %s states one diagnostic", where, e.Phase)
	}
	c.Expect = e
	return c, nil
}

func (v *validator) buildStub(s any, where, name string, opaque []string) (*Stub, error) {
	st := &Stub{}
	switch s := s.(type) {
	case string:
		st.Type = &s
	case map[string]any:
		st.Type = optString(s, "type")
		if calls, ok := s["calls"]; ok {
			for _, r := range calls.([]any) {
				rm := r.(map[string]any)
				row := &Row{}
				if a, ok := rm["args"]; ok {
					ca, err := v.opaque(a, opaque, where)
					if err != nil {
						return nil, err
					}
					row.Args, row.HasArgs = ca.([]any), true
				}
				if res, ok := rm["result"]; ok {
					cr, err := v.opaque(res, opaque, where)
					if err != nil {
						return nil, err
					}
					row.Result = cr
				}
				row.Error = optString(rm, "error")
				row.Charge, _ = isWhole(rm["charge"])
				row.Allocate, _ = isWhole(rm["allocate"])
				st.Calls = append(st.Calls, row)
			}
		}
	}
	// 3.10.10 item 11: the base environment of the suite declares no host
	// function and no method, so a stub needs its type.
	if st.Type == nil {
		return nil, v.invalid("%s: the stub %q has no type", where, name)
	}
	return st, nil
}

// OpaqueValue is an opaque value of a case file (3.10.3 item 7). Two are the
// same host value when their type and id are equal.
type OpaqueValue struct {
	Type string
	ID   int64
}

// OpaqueType returns the declared type name.
func (o OpaqueValue) OpaqueType() string { return o.Type }

// opaque replaces each "$opaque" object of a host value or expected value.
func (v *validator) opaque(x any, declared []string, where string) (any, error) {
	switch t := x.(type) {
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			ce, err := v.opaque(e, declared, where)
			if err != nil {
				return nil, err
			}
			out[i] = ce
		}
		return out, nil
	case map[string]any:
		if o, ok := t["$opaque"]; ok {
			om, ok := o.(map[string]any)
			if len(t) != 1 || !ok || len(om) != 2 {
				return nil, v.invalid("%s: a $opaque object is not of the form {\"$opaque\": {\"type\": T, \"id\": n}}", where)
			}
			ty, ok1 := om["type"].(string)
			id, ok2 := isWhole(om["id"])
			if !ok1 || !ok2 || id < 0 {
				return nil, v.invalid("%s: a $opaque object needs a type and an id", where)
			}
			found := false
			for _, d := range declared {
				found = found || d == ty
			}
			if !found {
				return nil, v.invalid("%s: the opaque type %q is not declared", where, ty)
			}
			return OpaqueValue{Type: ty, ID: id}, nil
		}
		out := make(map[string]any, len(t))
		for k, e := range t {
			ce, err := v.opaque(e, declared, where)
			if err != nil {
				return nil, err
			}
			out[k] = ce
		}
		return out, nil
	}
	return x, nil
}

func (v *validator) span(x any, where string) (*[4]int, error) {
	a, ok := x.([]any)
	if !ok || len(a) != 4 {
		return nil, v.invalid("%s: a span is not four numbers", where)
	}
	var s [4]int
	for i, e := range a {
		n, ok := isWhole(e)
		if !ok || n < 1 {
			return nil, v.invalid("%s: a span holds a number that is not a whole number of 1 or more", where)
		}
		s[i] = int(n)
	}
	return &s, nil
}

func (v *validator) count(x any, where, k string) (*int64, error) {
	n, ok := isWhole(x)
	if !ok || n < 0 {
		return nil, v.invalid("%s: %s is not a whole number of 0 or more", where, k)
	}
	return &n, nil
}

func (v *validator) expect(x any, where string) (*Expect, error) {
	m, ok := x.(map[string]any)
	if !ok {
		return nil, v.invalid("%s: expect is not an object", where)
	}
	where += ": expect"
	e := &Expect{}
	if ph, ok := m["phase"]; ok {
		if err := v.checkFields(m, failureFields, where); err != nil {
			return nil, err
		}
		p, ok := ph.(string)
		if !ok || !phases[p] {
			return nil, v.invalid("%s: phase is not one of the four phases", where)
		}
		e.Phase = p
		ds, ok := m["diagnostics"].([]any)
		if !ok || len(ds) == 0 {
			return nil, v.invalid("%s: a failure needs a non-empty list of diagnostics", where)
		}
		for i, d := range ds {
			ed, err := v.diagnostic(d, fmt.Sprintf("%s: diagnostic %d", where, i))
			if err != nil {
				return nil, err
			}
			e.Diagnostics = append(e.Diagnostics, ed)
		}
		if p != "run" {
			if _, ok := m["steps"]; ok {
				return nil, v.invalid("%s: steps with a phase that is not run", where)
			}
			if _, ok := m["allocated"]; ok {
				return nil, v.invalid("%s: allocated with a phase that is not run", where)
			}
		}
	} else {
		if err := v.checkFields(m, resultFields, where); err != nil {
			return nil, err
		}
		e.Type = optString(m, "type")
		if x, ok := m["type"]; ok {
			if _, ok := x.(string); !ok {
				return nil, v.invalid("%s: type is not a string", where)
			}
		}
		if x, ok := m["sites"]; ok {
			a, ok := x.([]any)
			if !ok {
				return nil, v.invalid("%s: sites is not an array", where)
			}
			e.HasSites = true
			for _, s := range a {
				sm, ok := s.(map[string]any)
				if !ok {
					return nil, v.invalid("%s: a site is not an object", where)
				}
				if err := v.checkFields(sm, siteFields, where+": site"); err != nil {
					return nil, err
				}
				cl, ok := sm["class"].(string)
				if !ok || !siteClasses[cl] {
					return nil, v.invalid("%s: a site has no valid class", where)
				}
				sp, err := v.span(sm["span"], where)
				if err != nil {
					return nil, err
				}
				e.Sites = append(e.Sites, Site{Class: cl, Span: *sp})
			}
		}
		if x, ok := m["value"]; ok {
			e.Value, e.HasValue = x, true
		}
		if x, ok := m["truthy"]; ok {
			b, ok := x.(bool)
			if !ok {
				return nil, v.invalid("%s: truthy is not a boolean", where)
			}
			e.Truthy = &b
		}
	}
	var err error
	if x, ok := m["steps"]; ok {
		if e.Steps, err = v.count(x, where, "steps"); err != nil {
			return nil, err
		}
	}
	if x, ok := m["allocated"]; ok {
		if e.Allocated, err = v.count(x, where, "allocated"); err != nil {
			return nil, err
		}
	}
	// Ruling A5: a case that expects a limit code states no usage.
	if e.Phase == "run" && (e.Steps != nil || e.Allocated != nil) {
		for _, d := range e.Diagnostics {
			if diag.LimitCodes[d.Code] {
				return nil, v.invalid("%s: a case that expects %s may not state steps or allocated", where, d.Code)
			}
		}
	}
	return e, nil
}

func (v *validator) diagnostic(x any, where string) (*ExpDiag, error) {
	m, ok := x.(map[string]any)
	if !ok {
		return nil, v.invalid("%s is not an object", where)
	}
	if err := v.checkFields(m, diagFields, where); err != nil {
		return nil, err
	}
	d := &ExpDiag{}
	code, ok := m["code"].(string)
	if !ok || !codeRe.MatchString(code) {
		return nil, v.invalid("%s has no valid code", where)
	}
	d.Code = code
	if x, ok := m["class"]; ok {
		cl, ok := x.(string)
		if !ok || !diagClasses[cl] {
			return nil, v.invalid("%s: class is not valid", where)
		}
		d.Class = &cl
	}
	if x, ok := m["span"]; ok {
		sp, err := v.span(x, where)
		if err != nil {
			return nil, err
		}
		d.Span = sp
	}
	if x, ok := m["path"]; ok {
		a, ok := x.([]any)
		if !ok || len(a) > 64 {
			return nil, v.invalid("%s: path is not a list of at most 64 steps", where)
		}
		for _, s := range a {
			switch s := s.(type) {
			case string:
			case float64:
				if n, ok := isWhole(s); !ok || n < 0 {
					return nil, v.invalid("%s: a path step is not a key or an index", where)
				}
			default:
				return nil, v.invalid("%s: a path step is not a key or an index", where)
			}
		}
		d.Path, d.HasPath = a, true
	}
	if x, ok := m["message"]; ok {
		s, ok := x.(string)
		if !ok {
			return nil, v.invalid("%s: message is not a string", where)
		}
		d.Message = &s
	}
	if x, ok := m["data"]; ok {
		d.Data, d.HasData = x, true
	}
	return d, nil
}

// StaticCodes are the eleven codes of the static rules (05-types.md 22.1).
var StaticCodes = map[string]bool{
	"undefined-name": true, "undefined-function": true, "not-a-value": true,
	"call-of-parameter": true, "lambda-to-lambda": true, "duplicate-name": true,
	"shadows-namespace": true, "duplicate-key": true, "arity": true,
	"duplicate-case": true, "unreachable-arm": true,
}
