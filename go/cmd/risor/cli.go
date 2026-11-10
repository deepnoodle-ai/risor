package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
)

// What the commands share (package F12 4.2, 4.3): reading inputs, the
// environment of a command, colour, and the layout of diagnostics.

// cmdError is an error of the command line (F12 4.9): one or more lines on
// standard error, nothing on standard output, and the status 2.
type cmdError struct{ msg string }

func (e *cmdError) Error() string { return e.msg }

func cmdErrorf(format string, args ...any) error {
	return &cmdError{fmt.Sprintf(format, args...)}
}

// flagValue reads the value of a flag at args[*i]: --name=value or --name
// value.
func flagValue(args []string, i *int, name string) (string, error) {
	a := args[*i]
	if v, ok := strings.CutPrefix(a, name+"="); ok {
		return v, nil
	}
	if *i+1 >= len(args) {
		return "", cmdErrorf("%s needs a value", name)
	}
	*i++
	return args[*i], nil
}

func isFlag(a, name string) bool { return a == name || strings.HasPrefix(a, name+"=") }

// wholeNumber reads a whole number of at least min.
func wholeNumber(name, v string, min int64) (int64, error) {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < min {
		return 0, cmdErrorf("%s takes a whole number of %d or more, not %q", name, min, v)
	}
	return n, nil
}

// readInput reads a FILE or a PATH as UTF-8 text; - is standard input
// (F12 4.2 item 1).
func readInput(path string, stdin io.Reader) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		if stdin == nil {
			return "", cmdErrorf("standard input cannot be read here")
		}
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", cmdErrorf("%s cannot be read: %v", path, err)
	}
	if !utf8.Valid(data) {
		return "", cmdErrorf("%s is not UTF-8 text", path)
	}
	return string(data), nil
}

// colorFlag is --color WHEN (F12 4.2 item 4).
type colorFlag struct{ when string }

func (c *colorFlag) parse(args []string, i *int) (bool, error) {
	if !isFlag(args[*i], "--color") {
		return false, nil
	}
	v, err := flagValue(args, i, "--color")
	if err != nil {
		return true, err
	}
	switch v {
	case "auto", "always", "never":
		c.when = v
		return true, nil
	}
	return true, cmdErrorf("--color is auto, always or never, not %q", v)
}

// on reports whether the text written to w is coloured. Under auto, no
// colour is written when standard output or w is not a terminal, or when
// the process variable NO_COLOR is set and not empty.
func (c *colorFlag) on(w, stdout io.Writer) bool {
	switch c.when {
	case "always":
		return true
	case "never":
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isTerminal(w) && isTerminal(stdout)
}

// isTerminal reports whether a stream is a terminal: a file that is a
// character device.
func isTerminal(w any) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

const (
	sgrRed    = "\x1b[1;31m"
	sgrYellow = "\x1b[33m"
	sgrCyan   = "\x1b[36m"
	sgrReset  = "\x1b[0m"
)

// paint colours a detail rendering: the word error and the carets.
func paint(text string) string {
	lines := strings.SplitAfter(text, "\n")
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "error["):
			lines[i] = sgrRed + "error" + sgrReset + l[len("error"):]
		case strings.HasPrefix(l, "    ") && strings.Contains(l, "^") && strings.Trim(l, " ^\n") == "":
			j := strings.IndexByte(l, '^')
			k := strings.LastIndexByte(l, '^') + 1
			lines[i] = l[:j] + sgrRed + l[j:k] + sgrReset + l[k:]
		}
	}
	return strings.Join(lines, "")
}

// writeDiagnostics writes diagnostics in the detail form of F8 5.3 with the
// source text, or in the JSON form of F8 5.5 (F12 4.2 item 3).
func writeDiagnostics(w io.Writer, ds []risor.Diagnostic, source *string, format string, color bool) {
	if format == "json" {
		fmt.Fprint(w, risor.RenderJSON(ds))
		return
	}
	var text string
	if source != nil {
		text = risor.RenderDetail(ds, *source)
	} else {
		text = risor.RenderDetail(ds)
	}
	if color {
		text = paint(text)
	}
	fmt.Fprint(w, text)
}

// diagnosticsOf gives the diagnostics of a failure, or false for an error
// that holds none.
func diagnosticsOf(err error) ([]risor.Diagnostic, bool) {
	var le *risor.LoadError
	if errors.As(err, &le) {
		return le.Diagnostics, true
	}
	var e *risor.Error
	if errors.As(err, &e) {
		return e.Diagnostics, true
	}
	return nil, false
}

// envFlags are the flags of F12 4.3, the environment of a command.
type envFlags struct {
	strVars     map[string]string
	varFile     string
	envKeys     []string
	description string
	groups      []string
	steps       *int64
	alloc       *int64
	depth       *int64
}

func newEnvFlags() *envFlags { return &envFlags{strVars: map[string]string{}} }

// parse reads an ENV flag at args[*i]. It reports whether args[*i] is one.
func (f *envFlags) parse(args []string, i *int) (bool, error) {
	a := args[*i]
	switch {
	case isFlag(a, "--var"):
		v, err := flagValue(args, i, "--var")
		if err != nil {
			return true, err
		}
		name, text, ok := strings.Cut(v, "=")
		if !ok || name == "" {
			return true, cmdErrorf("--var takes NAME=TEXT, not %q", v)
		}
		if _, dup := f.strVars[name]; dup {
			return true, cmdErrorf("the variable %s is named twice", name)
		}
		f.strVars[name] = text
	case isFlag(a, "--vars"):
		v, err := flagValue(args, i, "--vars")
		if err != nil {
			return true, err
		}
		if f.varFile != "" {
			return true, cmdErrorf("--vars is given twice")
		}
		f.varFile = v
	case isFlag(a, "--env"):
		v, err := flagValue(args, i, "--env")
		if err != nil {
			return true, err
		}
		f.envKeys = append(f.envKeys, v)
	case isFlag(a, "--description"):
		v, err := flagValue(args, i, "--description")
		if err != nil {
			return true, err
		}
		if f.description != "" {
			return true, cmdErrorf("--description is given twice")
		}
		f.description = v
	case isFlag(a, "--group"):
		v, err := flagValue(args, i, "--group")
		if err != nil {
			return true, err
		}
		f.groups = append(f.groups, v)
	case isFlag(a, "--steps"), isFlag(a, "--allocation"), isFlag(a, "--depth"):
		name := a
		if k := strings.IndexByte(a, '='); k >= 0 {
			name = a[:k]
		}
		v, err := flagValue(args, i, name)
		if err != nil {
			return true, err
		}
		n, err := wholeNumber(name, v, 0)
		if err != nil {
			return true, err
		}
		switch name {
		case "--steps":
			f.steps = &n
		case "--allocation":
			f.alloc = &n
		default:
			f.depth = &n
		}
	default:
		return false, nil
	}
	return true, nil
}

func (f *envFlags) limits() []risor.Decl {
	if f.steps == nil && f.alloc == nil && f.depth == nil {
		return nil
	}
	l := risor.Limits{}
	if f.steps != nil {
		l.Steps = *f.steps
		if l.Steps == 0 {
			l.Steps = risor.ZeroLimit
		}
	}
	if f.alloc != nil {
		l.Alloc = *f.alloc
		if l.Alloc == 0 {
			l.Alloc = risor.ZeroLimit
		}
	}
	if f.depth != nil {
		l.Depth = int(*f.depth)
		if l.Depth == 0 {
			l.Depth = -1 // a depth of 0 is bad-limit, at declare
		}
	}
	return []risor.Decl{l}
}

// environment builds the environment of the flags and the values of its
// variables (F12 4.3). A failure is a *cmdError, or an *risor.Error of
// phase declare.
func (f *envFlags) environment() (*risor.Env, risor.Map, error) {
	vars := risor.Map{}
	names := map[string]bool{}
	add := func(name string) error {
		if names[name] {
			return cmdErrorf("the variable %s is named twice", name)
		}
		names[name] = true
		return nil
	}
	var decls []risor.Decl
	var declared map[string]bool // the description's variables
	if f.description != "" {
		data, err := os.ReadFile(f.description)
		if err != nil {
			return nil, nil, cmdErrorf("%s cannot be read: %v", f.description, err)
		}
		d, err := risor.Description(data)
		if err != nil {
			return nil, nil, cmdErrorf("%s: %v", f.description, err)
		}
		decls = append(decls, d)
		declared = descriptionVars(data)
	}
	supply := func(name string, v any, decl risor.Decl) error {
		if err := add(name); err != nil {
			return err
		}
		if declared != nil {
			if !declared[name] {
				return cmdErrorf("the description declares no variable %s", name)
			}
		} else {
			decls = append(decls, decl)
		}
		vars[name] = v
		return nil
	}
	keys := make([]string, 0, len(f.strVars))
	for k := range f.strVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := supply(k, f.strVars[k], risor.Var(k, "string")); err != nil {
			return nil, nil, err
		}
	}
	if f.varFile != "" {
		data, err := os.ReadFile(f.varFile)
		if err != nil {
			return nil, nil, cmdErrorf("%s cannot be read: %v", f.varFile, err)
		}
		raw, err := casefile.DecodeJSON(data)
		if err != nil {
			return nil, nil, cmdErrorf("%s: %v", f.varFile, err)
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, cmdErrorf("%s: not a JSON object", f.varFile)
		}
		ks := make([]string, 0, len(obj))
		for k := range obj {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			var decl risor.Decl
			if declared == nil {
				// The literal type of the value (F1 3.4) is the type of a
				// schema that is the value's const.
				t, err := risor.TypeFromSchema(map[string]any{"const": obj[k]})
				if err != nil {
					return nil, nil, cmdErrorf("%s: %s: %v", f.varFile, k, err)
				}
				decl = risor.VarType(k, t)
			}
			if err := supply(k, obj[k], decl); err != nil {
				return nil, nil, err
			}
		}
	}
	if len(f.envKeys) > 0 {
		m := map[string]any{}
		for _, k := range f.envKeys {
			if v, ok := os.LookupEnv(k); ok {
				m[k] = v
			}
		}
		if err := supply("env", m, risor.Var("env", "map<string>")); err != nil {
			return nil, nil, err
		}
	}
	if len(f.groups) > 0 {
		decls = append(decls, risor.Groups(f.groups...))
	}
	decls = append(decls, f.limits()...)
	env, err := risor.NewEnv(decls...)
	return env, vars, err
}

// descriptionVars are the variables a description declares.
func descriptionVars(data []byte) map[string]bool {
	var d struct {
		Variables map[string]any `json:"variables"`
	}
	out := map[string]bool{}
	if json.Unmarshal(data, &d) == nil {
		for k := range d.Variables {
			out[k] = true
		}
	}
	return out
}
