package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
)

// The commands eval and check: the steps of a load are package F10 9's;
// the layout of diagnostics, the flags of the environment, --format for
// check, --print-type, --pretty, --max-failures, --color and the statuses
// are package F12 4's.

type configFlags struct {
	file      string
	typ       *string
	schema    *string
	format    string
	pretty    bool
	printType bool
	maxFail   int64
	env       *envFlags
	color     colorFlag
}

func parseConfigFlags(args []string, eval bool) (*configFlags, error) {
	f := &configFlags{format: "text", maxFail: 10, env: newEnvFlags(), color: colorFlag{"auto"}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if ok, err := f.env.parse(args, &i); ok || err != nil {
			if err != nil {
				return nil, err
			}
			continue
		}
		if ok, err := f.color.parse(args, &i); ok || err != nil {
			if err != nil {
				return nil, err
			}
			continue
		}
		switch {
		case isFlag(a, "--type"):
			v, err := flagValue(args, &i, "--type")
			if err != nil {
				return nil, err
			}
			f.typ = &v
		case isFlag(a, "--schema"):
			v, err := flagValue(args, &i, "--schema")
			if err != nil {
				return nil, err
			}
			f.schema = &v
		case isFlag(a, "--format"):
			v, err := flagValue(args, &i, "--format")
			if err != nil {
				return nil, err
			}
			if v != "text" && v != "json" {
				return nil, cmdErrorf("--format is text or json, not %q", v)
			}
			f.format = v
		case eval && a == "--pretty":
			f.pretty = true
		case eval && isFlag(a, "--max-failures"):
			v, err := flagValue(args, &i, "--max-failures")
			if err != nil {
				return nil, err
			}
			if f.maxFail, err = wholeNumber("--max-failures", v, 1); err != nil {
				return nil, err
			}
		case !eval && a == "--print-type":
			f.printType = true
		case len(a) > 1 && a[0] == '-':
			return nil, cmdErrorf("unknown flag %s", a)
		default:
			if f.file != "" {
				return nil, cmdErrorf("one FILE only, found %s and %s", f.file, a)
			}
			f.file = a
		}
	}
	if f.file == "" {
		return nil, cmdErrorf("no FILE")
	}
	if f.typ != nil && f.schema != nil {
		return nil, cmdErrorf("--type and --schema exclude each other")
	}
	return f, nil
}

// expected is the load option of --type or --schema.
func (f *configFlags) expected() ([]risor.LoadOption, any, error) {
	switch {
	case f.typ != nil:
		return []risor.LoadOption{risor.LoadType(*f.typ)}, nil, nil
	case f.schema != nil:
		text, err := readInput(*f.schema, nil)
		if err != nil {
			return nil, nil, err
		}
		s, err := casefile.DecodeJSON([]byte(text))
		if err != nil {
			return nil, nil, cmdErrorf("%s: %v", *f.schema, err)
		}
		return []risor.LoadOption{risor.LoadSchema(s)}, s, nil
	}
	return nil, nil, nil
}

func evalCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return configCommand(args, stdin, stdout, stderr, true)
}

func checkCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return configCommand(args, stdin, stdout, stderr, false)
}

// commandLineError writes an error of the command line (F12 4.9).
func commandLineError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "risor: %v\n", err)
	return 2
}

func configCommand(args []string, stdin io.Reader, stdout, stderr io.Writer, eval bool) int {
	f, err := parseConfigFlags(args, eval)
	if err != nil {
		fmt.Fprintf(stderr, "risor: %v\n%s", err, usage)
		return 2
	}
	source, err := readInput(f.file, stdin)
	if err != nil {
		return commandLineError(stderr, err)
	}
	opts, schema, err := f.expected()
	if err != nil {
		return commandLineError(stderr, err)
	}
	// Diagnostics go to standard error for eval, to standard output for
	// check (F12 4.2 item 2).
	dw := stderr
	if !eval {
		dw = stdout
	}
	color := f.format == "text" && f.color.on(dw, stdout)
	fail := func(err error) int {
		ds, ok := diagnosticsOf(err)
		if !ok {
			return commandLineError(stderr, err)
		}
		writeDiagnostics(dw, ds, &source, f.format, color)
		return 1
	}
	env, vars, err := f.env.environment()
	if err != nil {
		return fail(err)
	}
	if !eval {
		t, err := checkSteps(env, f, schema, source)
		if err != nil {
			return fail(err)
		}
		if f.printType {
			fmt.Fprintln(stdout, t)
		}
		return 0
	}
	opts = append(opts, risor.MaxFailures(int(f.maxFail)))
	v, _, err := risor.LoadValue(context.Background(), env, source, vars, opts...)
	if err != nil {
		return fail(err)
	}
	var text string
	switch {
	case f.pretty:
		text, err = risor.JSONText(v)
		if err == nil {
			var b bytes.Buffer
			if json.Indent(&b, []byte(text), "", "  ") == nil {
				text = b.String()
			}
		}
	case f.format == "json":
		text, err = risor.JSONText(v)
	default:
		text, err = rendering(v)
	}
	if err != nil {
		return fail(err)
	}
	fmt.Fprint(stdout, text+"\n")
	return 0
}

// checkSteps makes steps 0, 1 and 2 of a load: declare, parse, check (F10
// 9.3). It gives the printed result type.
func checkSteps(env *risor.Env, f *configFlags, schema any, source string) (string, error) {
	var copts []risor.CheckOption
	switch {
	case f.typ != nil:
		t, err := env.ParseType(*f.typ)
		if err != nil {
			return "", err
		}
		copts = append(copts, risor.ExpectType(t))
	case f.schema != nil:
		copts = append(copts, risor.ExpectSchema(schema, risor.Closed))
	}
	x, err := risor.Parse(source)
	if err != nil {
		return "", err
	}
	cr, err := env.Check(x, copts...)
	if err != nil {
		return "", err
	}
	if cr == nil {
		return "", errors.New("no check result")
	}
	return cr.Type, nil
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
