package gen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LibEntry is one entry of suite/library.json, with its signature parsed.
type LibEntry struct {
	Name      string
	Receiver  string // type text of the receiver; "" for a function
	Signature string
	Params    []LibParam
	Result    string
	Where     map[string]string // type variable to bound: ordered, number
	Calls     []string          // names of the callback parameters
	// Group is the opt-in group of an entry of package F6 (json, time, math,
	// base64); "" for an entry of the Core library or of F7.
	Group string
	// Const is set for a constant of a group, such as math.pi; Type is then
	// its type.
	Const bool
	Type  string
	// Package is "F6" or "F7" for an entry those packages add, "" for Core.
	Package string
}

// Local gives an entry's name without its group: parse for json.parse.
func (e *LibEntry) Local() string {
	if i := strings.IndexByte(e.Name, '.'); i >= 0 && e.Group != "" {
		return e.Name[i+1:]
	}
	return e.Name
}

// LibParam is one parameter of a library entry.
type LibParam struct {
	Name string
	Opt  bool
	Rest bool
	Type string   // type text; for a callback, the whole function type
	Fn   *LibFunc // the callback's type, when the parameter takes a function
}

// LibFunc is the type of a callback parameter.
type LibFunc struct {
	Params []string // type text of each parameter
	Result string
}

// Required is the count of parameters a call must supply.
func (e *LibEntry) Required() int {
	n := 0
	for _, p := range e.Params {
		if !p.Opt && !p.Rest {
			n++
		}
	}
	return n
}

// Max is the count of arguments a call may supply, or -1 with a rest.
func (e *LibEntry) Max() int {
	for _, p := range e.Params {
		if p.Rest {
			return -1
		}
	}
	return len(e.Params)
}

// Library is the Core library, read from suite/library.json.
type Library struct {
	Entries   []*LibEntry
	Functions []*LibEntry
	Methods   []*LibEntry
	// Groups holds the entries of package F6, from library-groups.json, and
	// Queries the methods of package F7, from library-queries.json. A
	// query that extends a Core entry (sortBy) replaces it in Methods.
	Groups  []*LibEntry
	Queries []*LibEntry
}

type rawEntry struct {
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Receiver  string   `json:"receiver"`
	Calls     []string `json:"calls"`
	Group     string   `json:"group"`
	Type      string   `json:"type"`
	Extends   any      `json:"extends"`
}

// FindLibrary looks for suite/library.json from dir upward.
func FindLibrary(dir string) (string, error) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(d, "suite", "library.json")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		up := filepath.Dir(d)
		if up == d {
			return "", fmt.Errorf("suite/library.json not found above %s", dir)
		}
		d = up
	}
}

// LoadLibrary reads and parses library.json.
func LoadLibrary(path string) (*Library, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw []rawEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	lib := &Library{}
	defer lib.loadPackages(filepath.Dir(path))
	for _, r := range raw {
		e, err := parseEntry(r)
		if err != nil {
			return nil, fmt.Errorf("%s: entry %s: %w", path, r.Name, err)
		}
		lib.Entries = append(lib.Entries, e)
		if e.Receiver == "" {
			lib.Functions = append(lib.Functions, e)
		} else {
			lib.Methods = append(lib.Methods, e)
		}
	}
	return lib, nil
}

// loadPackages reads the records of packages F6 and F7 that stand beside
// library.json, when they exist (suite/README.md). The correctly rounded
// functions of F13 are in a file of their own and are not read.
func (lib *Library) loadPackages(dir string) {
	var groups, queries []rawEntry
	if data, err := os.ReadFile(filepath.Join(dir, "library-groups.json")); err == nil {
		_ = json.Unmarshal(data, &groups)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "library-queries.json")); err == nil {
		_ = json.Unmarshal(data, &queries)
	}
	for _, r := range groups {
		if r.Signature == "" {
			lib.Groups = append(lib.Groups, &LibEntry{Name: r.Name, Group: r.Group, Const: true, Type: r.Type, Package: "F6"})
			continue
		}
		e, err := parseEntry(r)
		if err != nil {
			continue
		}
		e.Group, e.Package = r.Group, "F6"
		lib.Groups = append(lib.Groups, e)
	}
	for _, r := range queries {
		e, err := parseEntry(r)
		if err != nil {
			continue
		}
		e.Package = "F7"
		lib.Queries = append(lib.Queries, e)
	}
}

// WithQueries gives the methods with those of F7: sortBy with a direction
// in place of Core's, and the six methods F7 adds.
func (lib *Library) WithQueries() []*LibEntry {
	var out []*LibEntry
	for _, m := range lib.Methods {
		replaced := false
		for _, q := range lib.Queries {
			if q.Name == m.Name && q.Receiver == m.Receiver {
				replaced = true
			}
		}
		if !replaced {
			out = append(out, m)
		}
	}
	return append(out, lib.Queries...)
}

func parseEntry(r rawEntry) (*LibEntry, error) {
	e := &LibEntry{Name: r.Name, Receiver: r.Receiver, Signature: r.Signature, Calls: r.Calls, Where: map[string]string{}}
	sig := strings.TrimSpace(r.Signature)
	if !strings.HasPrefix(sig, "(") {
		return nil, fmt.Errorf("signature %q does not start with (", sig)
	}
	end := matchParen(sig, 0)
	if end < 0 {
		return nil, fmt.Errorf("signature %q: no matching )", sig)
	}
	for _, p := range splitTop(sig[1:end], ',') {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var lp LibParam
		if strings.HasPrefix(p, "...") {
			lp.Rest = true
			p = p[3:]
		}
		colon := strings.Index(p, ":")
		if colon < 0 {
			return nil, fmt.Errorf("parameter %q has no type", p)
		}
		lp.Name = strings.TrimSpace(p[:colon])
		if strings.HasSuffix(lp.Name, "?") {
			lp.Opt = true
			lp.Name = strings.TrimSuffix(lp.Name, "?")
		}
		lp.Type = strings.TrimSpace(p[colon+1:])
		if strings.HasPrefix(lp.Type, "(") {
			fe := matchParen(lp.Type, 0)
			f := &LibFunc{}
			for _, fp := range splitTop(lp.Type[1:fe], ',') {
				fp = strings.TrimSpace(fp)
				if c := strings.Index(fp, ":"); c >= 0 {
					f.Params = append(f.Params, strings.TrimSpace(fp[c+1:]))
				}
			}
			rest := strings.TrimSpace(lp.Type[fe+1:])
			f.Result = strings.TrimSpace(strings.TrimPrefix(rest, "=>"))
			lp.Fn = f
		}
		e.Params = append(e.Params, lp)
	}
	rest := strings.TrimSpace(sig[end+1:])
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "=>"))
	if i := strings.Index(rest, " where "); i >= 0 {
		for _, w := range strings.Split(rest[i+7:], ",") {
			kv := strings.SplitN(w, ":", 2)
			if len(kv) == 2 {
				e.Where[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
		rest = strings.TrimSpace(rest[:i])
	}
	e.Result = strings.TrimSpace(strings.TrimSuffix(rest, "!"))
	for _, c := range e.Calls {
		found := false
		for _, p := range e.Params {
			if p.Name == c && p.Fn != nil {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("callback %q is not a function parameter", c)
		}
	}
	return e, nil
}

func matchParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(', '{', '[', '<':
			depth++
		case ')', '}', ']', '>':
			if s[i] == '>' && i > 0 && s[i-1] == '=' {
				continue // the arrow =>
			}
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitTop(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '{', '[', '<':
			depth++
		case ')', '}', ']', '>':
			if s[i] == '>' && i > 0 && s[i-1] == '=' {
				continue
			}
			depth--
		default:
			if s[i] == sep && depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}
