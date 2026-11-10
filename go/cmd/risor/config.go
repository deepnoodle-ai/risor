package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
)

// The commands eval and check (package F10 9). The layout of diagnostics,
// errors of the command line and --pretty are package F12's; until then a
// diagnostic is written to standard error in the one-line form of F8, and
// an error of the command line has the status 2.

const evalUsage = `usage: risor eval FILE [--type TEXT | --schema FILE] [--format text|json] [--var NAME=TEXT ...] [--vars FILE.json] [--env NAME ...]
       risor check FILE [--type TEXT | --schema FILE] [--var NAME=TEXT ...] [--vars FILE.json] [--env NAME ...]
`

type configFlags struct {
	file    string
	typ     *string
	schema  *string
	format  string
	strVars map[string]string
	varFile string
	envKeys []string
}

func parseConfigFlags(args []string, eval bool) (*configFlags, error) {
	f := &configFlags{format: "text", strVars: map[string]string{}}
	val := func(i *int, name string) (string, error) {
		a := args[*i]
		if v, ok := strings.CutPrefix(a, name+"="); ok {
			return v, nil
		}
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%s needs a value", name)
		}
		*i++
		return args[*i], nil
	}
	is := func(a, name string) bool { return a == name || strings.HasPrefix(a, name+"=") }
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case is(a, "--type"):
			v, err := val(&i, "--type")
			if err != nil {
				return nil, err
			}
			f.typ = &v
		case is(a, "--schema"):
			v, err := val(&i, "--schema")
			if err != nil {
				return nil, err
			}
			f.schema = &v
		case eval && is(a, "--format"):
			v, err := val(&i, "--format")
			if err != nil {
				return nil, err
			}
			if v != "text" && v != "json" {
				return nil, fmt.Errorf("--format is text or json, not %q", v)
			}
			f.format = v
		case is(a, "--var"):
			v, err := val(&i, "--var")
			if err != nil {
				return nil, err
			}
			name, text, ok := strings.Cut(v, "=")
			if !ok || name == "" {
				return nil, fmt.Errorf("--var takes NAME=TEXT, not %q", v)
			}
			if _, dup := f.strVars[name]; dup {
				return nil, fmt.Errorf("the variable %s is named twice", name)
			}
			f.strVars[name] = text
		case is(a, "--vars"):
			v, err := val(&i, "--vars")
			if err != nil {
				return nil, err
			}
			if f.varFile != "" {
				return nil, errors.New("--vars is given twice")
			}
			f.varFile = v
		case is(a, "--env"):
			v, err := val(&i, "--env")
			if err != nil {
				return nil, err
			}
			f.envKeys = append(f.envKeys, v)
		case strings.HasPrefix(a, "-") && a != "-":
			return nil, fmt.Errorf("unknown flag %s", a)
		default:
			if f.file != "" {
				return nil, fmt.Errorf("one FILE only, found %s and %s", f.file, a)
			}
			f.file = a
		}
	}
	if f.file == "" {
		return nil, errors.New("no FILE")
	}
	if f.typ != nil && f.schema != nil {
		return nil, errors.New("--type and --schema exclude each other")
	}
	return f, nil
}

// environment declares and supplies the variables of the flags (9.4).
func (f *configFlags) environment() (*risor.Env, risor.Map, error) {
	var decls []risor.Decl
	vars := risor.Map{}
	names := map[string]bool{}
	add := func(name string) error {
		if names[name] {
			return fmt.Errorf("the variable %s is named twice", name)
		}
		names[name] = true
		return nil
	}
	keys := make([]string, 0, len(f.strVars))
	for k := range f.strVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := add(k); err != nil {
			return nil, nil, err
		}
		decls = append(decls, risor.Var(k, "string"))
		vars[k] = f.strVars[k]
	}
	if f.varFile != "" {
		data, err := os.ReadFile(f.varFile)
		if err != nil {
			return nil, nil, err
		}
		raw, err := casefile.DecodeJSON(data)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", f.varFile, err)
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s: not a JSON object", f.varFile)
		}
		ks := make([]string, 0, len(obj))
		for k := range obj {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			if err := add(k); err != nil {
				return nil, nil, err
			}
			// The literal type of the value (F1 3.4) is the type of a schema
			// that is the value's const.
			t, err := risor.TypeFromSchema(map[string]any{"const": obj[k]})
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s: %v", f.varFile, k, err)
			}
			decls = append(decls, risor.VarType(k, t))
			vars[k] = obj[k]
		}
	}
	if len(f.envKeys) > 0 {
		if err := add("env"); err != nil {
			return nil, nil, err
		}
		decls = append(decls, risor.Var("env", "map<string>"))
		m := map[string]any{}
		for _, k := range f.envKeys {
			if v, ok := os.LookupEnv(k); ok {
				m[k] = v
			}
		}
		vars["env"] = m
	}
	env, err := risor.NewEnv(decls...)
	return env, vars, err
}

// expected is the load option of --type or --schema.
func (f *configFlags) expected() ([]risor.LoadOption, error) {
	switch {
	case f.typ != nil:
		return []risor.LoadOption{risor.LoadType(*f.typ)}, nil
	case f.schema != nil:
		data, err := os.ReadFile(*f.schema)
		if err != nil {
			return nil, err
		}
		s, err := casefile.DecodeJSON(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", *f.schema, err)
		}
		return []risor.LoadOption{risor.LoadSchema(s)}, nil
	}
	return nil, nil
}

func writeDiagnostics(w io.Writer, err error) {
	var e *risor.Error
	if errors.As(err, &e) {
		fmt.Fprint(w, risor.RenderLine(e.Diagnostics))
		return
	}
	fmt.Fprintf(w, "risor: %v\n", err)
}

func evalCommand(args []string, stdout, stderr io.Writer) int {
	return configCommand(args, stdout, stderr, true)
}

func checkCommand(args []string, stdout, stderr io.Writer) int {
	return configCommand(args, stdout, stderr, false)
}

func configCommand(args []string, stdout, stderr io.Writer, eval bool) int {
	f, err := parseConfigFlags(args, eval)
	if err != nil {
		fmt.Fprintf(stderr, "risor: %v\n%s", err, evalUsage)
		return 2
	}
	data, err := os.ReadFile(f.file)
	if err != nil {
		fmt.Fprintf(stderr, "risor: %v\n", err)
		return 2
	}
	opts, err := f.expected()
	if err != nil {
		fmt.Fprintf(stderr, "risor: %v\n", err)
		return 2
	}
	env, vars, err := f.environment()
	if err != nil {
		var e *risor.Error
		if errors.As(err, &e) {
			writeDiagnostics(stderr, err)
			return 1
		}
		fmt.Fprintf(stderr, "risor: %v\n", err)
		return 2
	}
	if !eval {
		// Steps 0, 1 and 2 of a load (9.3).
		if err := f.checkSteps(env, string(data)); err != nil {
			writeDiagnostics(stderr, err)
			return 1
		}
		return 0
	}
	v, _, err := risor.LoadValue(context.Background(), env, string(data), vars, opts...)
	if err != nil {
		writeDiagnostics(stderr, err)
		return 1
	}
	var text string
	if f.format == "json" {
		text, err = risor.JSONText(v)
	} else {
		text, err = rendering(v)
	}
	if err != nil {
		writeDiagnostics(stderr, err)
		return 1
	}
	fmt.Fprint(stdout, text+"\n")
	return 0
}

// checkSteps makes steps 0 to 2 of a load: declare, parse, check (9.3).
func (f *configFlags) checkSteps(env *risor.Env, source string) error {
	var copts []risor.CheckOption
	switch {
	case f.typ != nil:
		t, err := env.ParseType(*f.typ)
		if err != nil {
			return err
		}
		copts = append(copts, risor.ExpectType(t))
	case f.schema != nil:
		data, err := os.ReadFile(*f.schema)
		if err != nil {
			return err
		}
		sch, err := casefile.DecodeJSON(data)
		if err != nil {
			return fmt.Errorf("%s: %v", *f.schema, err)
		}
		copts = append(copts, risor.ExpectSchema(sch, risor.Closed))
	}
	x, err := risor.Parse(source)
	if err != nil {
		return err
	}
	_, err = env.Check(x, copts...)
	return err
}

// rendering is the rendering of a value (01-language.md 12), as string(v)
// gives it.
func rendering(v any) (string, error) {
	env, err := risor.NewEnv(risor.Var("v", "any"))
	if err != nil {
		return "", err
	}
	x, err := risor.Parse("string(v)")
	if err != nil {
		return "", err
	}
	p, err := env.Compile(x, risor.NoCheck)
	if err != nil {
		return "", err
	}
	s, err := p.Run(context.Background(), risor.Map{"v": v})
	if err != nil {
		return "", err
	}
	return s.(string), nil
}
