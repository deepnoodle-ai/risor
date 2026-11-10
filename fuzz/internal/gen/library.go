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
}

type rawEntry struct {
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Receiver  string   `json:"receiver"`
	Calls     []string `json:"calls"`
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
