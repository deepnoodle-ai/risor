package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/eval"
	"github.com/deepnoodle-ai/wonton/cli"
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

// configFlagDefs are the flags of eval (eval true) or check.
func configFlagDefs(eval bool) []cli.Flag {
	fs := []cli.Flag{
		cli.String("type").Placeholder("TEXT").Help("Check the result against the type TEXT"),
		cli.String("schema").Placeholder("FILE").Help("Check the result against the JSON Schema in FILE, closed"),
		formatFlag(),
	}
	if eval {
		fs = append(fs,
			cli.Bool("pretty").Help("Write the value as indented JSON"),
			cli.String("max-failures").Placeholder("N").Default("10").Help("Report at most N failures of the result's conformance").
				ValidateWith(wholeNumberOf("--max-failures", 1)),
		)
	} else {
		fs = append(fs, cli.Bool("print-type").Help("Write the printed result type"))
	}
	fs = append(fs, envFlagDefs()...)
	return append(fs, colorFlagDef())
}

// configFlagsOf reads the arguments of eval or check.
func configFlagsOf(ctx *cli.Context, eval bool) (*configFlags, error) {
	f := &configFlags{
		file:   ctx.Arg(0),
		format: ctx.String("format"),
		color:  colorOf(ctx),
	}
	if ctx.IsSet("type") {
		v := ctx.String("type")
		f.typ = &v
	}
	if ctx.IsSet("schema") {
		v := ctx.String("schema")
		f.schema = &v
	}
	if f.typ != nil && f.schema != nil {
		return nil, cmdErrorf("--type and --schema exclude each other")
	}
	if eval {
		f.pretty = ctx.Bool("pretty")
		n, err := wholeNumber("--max-failures", ctx.String("max-failures"), 1)
		if err != nil {
			return nil, err
		}
		f.maxFail = n
	} else {
		f.printType = ctx.Bool("print-type")
	}
	var err error
	if f.env, err = envFlagsOf(ctx); err != nil {
		return nil, err
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

// commandLineError writes an error of the command line (F12 4.9).
func commandLineError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "risor: %v\n", err)
	return 2
}

func configCommand(ctx *cli.Context, isEval bool) int {
	stdin, stdout, stderr := ctx.Stdin(), ctx.Stdout(), ctx.Stderr()
	f, err := configFlagsOf(ctx, isEval)
	if err != nil {
		return commandLineError(stderr, err)
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
	if !isEval {
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
	if !isEval {
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
		text, err = rendering(env, v)
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
	return cr.Type.String(), nil
}

// rendering is the rendering of a value (01-language.md 12) under the
// environment's depth limit. It is not a run, so it charges no steps or units.
func rendering(env *risor.Env, v any) (string, error) {
	depth := eval.DefaultLimits.Depth
	if desc, err := env.Describe(); err == nil {
		var d struct {
			Limits struct{ Depth int } `json:"limits"`
		}
		if json.Unmarshal(desc, &d) == nil && d.Limits.Depth > 0 {
			depth = d.Limits.Depth
		}
	}
	text, d := eval.RenderText(v, depth)
	if d != nil {
		return "", &risor.Error{Diagnostics: []risor.Diagnostic{*d}}
	}
	return text, nil
}
