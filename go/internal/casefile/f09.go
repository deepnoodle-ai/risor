package casefile

import (
	"fmt"
	"os"
	"path/filepath"
)

// The additions of package F9 9 to the case format, and the options a
// runner of a host's own files gives the reader (F9 7.1).

// Options change how a file is read.
type Options struct {
	// Update reads the file for update mode: a case may have no expect
	// (F9 4.2 item 1).
	Update bool
	// BaseVar and BaseFunc report whether the base environment declares a
	// variable or a host function or method ("T.m"). A stub with no type
	// takes its signature from the base (03-host.md 3.10.4), and vars may
	// supply a variable of the base. Nil: the default environment, which
	// declares none.
	BaseVar, BaseFunc func(name string) bool
	// Read reads the text of a case's file, given the path relative to the
	// file's directory. Nil reads the file system.
	Read func(ref string) (string, error)
}

// RunnerExpect is the expect of a runner case (F9 3.3).
type RunnerExpect struct {
	Invalid  bool
	Verdicts []ExpVerdict
	Report   *string
	Text     *string
}

// ExpVerdict is one entry of verdicts (F9 3.4).
type ExpVerdict struct {
	Name, Status string
	Differs      []string // nil when absent
	Fixture      []any    // nil when absent
	Message      *string
}

var (
	runnerCaseFields   = set("name", "runner", "update", "program", "skip", "expect")
	runnerExpectFields = set("invalid", "verdicts", "report", "text")
	verdictFields      = set("name", "status", "differs", "fixture", "message")
	verdictStatuses    = set("pass", "fail", "skip", "invalid", "updated")
)

// ParseWith validates the bytes of a case file with options.
func ParseWith(data []byte, path string, o Options) (*File, error) {
	raw, err := DecodeJSON(data)
	if err != nil {
		return nil, &InvalidError{Path: path, Reason: err.Error()}
	}
	return FromValueWith(raw, path, o)
}

// FromValueWith validates a decoded case file with options.
func FromValueWith(raw any, path string, o Options) (*File, error) {
	v := &validator{path: path, dir: filepath.Dir(path), opts: o, noExpect: o.Update}
	f, err := v.file(raw)
	if err != nil {
		err.(*InvalidError).Names = v.names
		return nil, err
	}
	return f, nil
}

func (v *validator) readFile(ref string) (string, error) {
	if v.opts.Read != nil {
		return v.opts.Read(ref)
	}
	data, err := os.ReadFile(filepath.Join(v.dir, filepath.FromSlash(ref)))
	return string(data), err
}

// checkRunnerFlags is 9.2 items 1 and 2 for any case.
func (v *validator) checkRunnerFlags(m map[string]any, where string) error {
	r, hasRunner := m["runner"]
	_, isText := r.(string)
	if hasRunner {
		switch r.(type) {
		case string, map[string]any, []any:
		default:
			return v.invalid("%s: runner is not a string, an object or an array", where)
		}
	}
	for _, k := range []string{"update", "program"} {
		x, ok := m[k]
		if !ok {
			continue
		}
		b, ok := x.(bool)
		if !ok {
			return v.invalid("%s: %s is not true or false", where, k)
		}
		if !hasRunner {
			return v.invalid("%s: %s in a case with no runner", where, k)
		}
		if b && !isText {
			return v.invalid("%s: %s true beside a runner that is not a string", where, k)
		}
	}
	return nil
}

// buildRunner builds a runner case (F9 3).
func (v *validator) buildRunner(c *Case, m map[string]any, where string) (*Case, error) {
	if err := v.checkFields(m, runnerCaseFields, where); err != nil {
		return nil, err
	}
	c.Skip = optString(m, "skip")
	c.Dir = v.dir
	switch r := m["runner"].(type) {
	case string:
		c.RunnerText = &r
	default:
		c.Runner = r
	}
	c.Update, _ = m["update"].(bool)
	c.Program, _ = m["program"].(bool)
	em, ok := m["expect"].(map[string]any)
	if !ok {
		if _, has := m["expect"]; !has && v.noExpect {
			c.Expect = &Expect{Runner: &RunnerExpect{}}
			return c, nil
		}
		return nil, v.invalid("%s: expect is not an object", where)
	}
	where += ": expect"
	if err := v.checkFields(em, runnerExpectFields, where); err != nil {
		return nil, err
	}
	re := &RunnerExpect{}
	if x, ok := em["invalid"]; ok {
		if b, ok := x.(bool); !ok || !b || len(em) != 1 {
			return nil, v.invalid("%s: invalid is not true, or stands beside another field", where)
		}
		re.Invalid = true
		c.Expect = &Expect{Runner: re}
		return c, nil
	}
	vs, ok := em["verdicts"].([]any)
	if !ok {
		return nil, v.invalid("%s: neither invalid nor a list of verdicts", where)
	}
	re.Verdicts = []ExpVerdict{}
	for i, x := range vs {
		ev, err := v.verdict(x, fmt.Sprintf("%s: verdict %d", where, i), c.Update)
		if err != nil {
			return nil, err
		}
		re.Verdicts = append(re.Verdicts, ev)
	}
	for _, k := range []string{"report", "text"} {
		x, ok := em[k]
		if !ok {
			continue
		}
		s, ok := x.(string)
		if !ok {
			return nil, v.invalid("%s: %s is not a string", where, k)
		}
		if k == "report" {
			re.Report = &s
		} else {
			if !c.Update {
				return nil, v.invalid("%s: text when update is not true", where)
			}
			re.Text = &s
		}
	}
	c.Expect = &Expect{Runner: re}
	return c, nil
}

func (v *validator) verdict(x any, where string, update bool) (ExpVerdict, error) {
	var ev ExpVerdict
	m, ok := x.(map[string]any)
	if !ok {
		return ev, v.invalid("%s is not an object", where)
	}
	if err := v.checkFields(m, verdictFields, where); err != nil {
		return ev, err
	}
	name, ok1 := m["name"].(string)
	st, ok2 := m["status"].(string)
	if !ok1 || !ok2 || !verdictStatuses[st] {
		return ev, v.invalid("%s needs a name and a status of the five", where)
	}
	if st == "updated" && !update {
		return ev, v.invalid("%s: updated when update is not true", where)
	}
	ev.Name, ev.Status = name, st
	if x, ok := m["differs"]; ok {
		list, ok := stringList(x)
		if !ok || st != "fail" {
			return ev, v.invalid("%s: differs is not a list of strings, or stands beside %s", where, st)
		}
		ev.Differs = list
	}
	if x, ok := m["fixture"]; ok {
		if st != "invalid" {
			return ev, v.invalid("%s: fixture beside %s", where, st)
		}
		d, err := v.diagnostic(map[string]any{"code": "x", "path": x}, where)
		if err != nil {
			return ev, err
		}
		ev.Fixture = d.Path
	}
	if x, ok := m["message"]; ok {
		s, ok := x.(string)
		if !ok || st != "invalid" {
			return ev, v.invalid("%s: message is not a string, or stands beside %s", where, st)
		}
		ev.Message = &s
	}
	return ev, nil
}
