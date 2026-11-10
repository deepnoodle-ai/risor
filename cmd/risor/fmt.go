package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/cli"
)

// The command fmt (package F12 4.6).

type fmtFlags struct {
	paths    []string
	write    bool
	check    bool
	template bool
	color    colorFlag
}

// fmtFlagDefs are the flags of fmt.
func fmtFlagDefs() []cli.Flag {
	return []cli.Flag{
		cli.Bool("write").Help("Write each file's formatted text back to the file"),
		cli.Bool("check").Help("Write the path of each file that is not formatted"),
		cli.Bool("template").Help("Format each input as a template"),
		colorFlagDef(),
	}
}

// fmtFlagsOf reads the arguments of fmt.
func fmtFlagsOf(ctx *cli.Context) (*fmtFlags, error) {
	f := &fmtFlags{
		paths:    ctx.Args(),
		write:    ctx.Bool("write"),
		check:    ctx.Bool("check"),
		template: ctx.Bool("template"),
		color:    colorOf(ctx),
	}
	if f.write && f.check {
		return nil, cmdErrorf("--write and --check exclude each other")
	}
	return f, nil
}

// fmtInput is one input of fmt: its path as given or found, and its text.
type fmtInput struct {
	path, text string
}

// findRisorFiles searches a directory for files whose names end in .risor,
// by the rules of package F9 8.2 items 2 and 3.
func findRisorFiles(p string) ([]string, error) {
	var rels [][]string
	err := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != p && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".risor") {
			rel, _ := filepath.Rel(p, path)
			rels = append(rels, strings.Split(filepath.ToSlash(rel), "/"))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rels, func(i, j int) bool {
		a, b := rels[i], rels[j]
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return lessCodePoints(a[k], b[k])
			}
		}
		return len(a) < len(b)
	})
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = filepath.Join(append([]string{p}, r...)...)
	}
	return out, nil
}

func fmtCommand(ctx *cli.Context) int {
	stdin, stdout, stderr := ctx.Stdin(), ctx.Stdout(), ctx.Stderr()
	f, err := fmtFlagsOf(ctx)
	if err != nil {
		return commandLineError(stderr, err)
	}
	paths := f.paths
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	// Every input is read before anything is written: an input that cannot
	// be read is an error of the command line.
	var inputs []fmtInput
	for _, p := range paths {
		if p == "-" {
			if f.write {
				return commandLineError(stderr, cmdErrorf("--write cannot be given with standard input"))
			}
			text, err := readInput("-", stdin)
			if err != nil {
				return commandLineError(stderr, err)
			}
			inputs = append(inputs, fmtInput{"-", text})
			continue
		}
		st, err := os.Stat(p)
		if err != nil {
			return commandLineError(stderr, cmdErrorf("%s cannot be read: %v", p, err))
		}
		files := []string{p}
		if st.IsDir() {
			if f.template {
				return commandLineError(stderr, cmdErrorf("%s is a directory, which --template does not search", p))
			}
			if files, err = findRisorFiles(p); err != nil {
				return commandLineError(stderr, cmdErrorf("%s cannot be read: %v", p, err))
			}
		}
		for _, file := range files {
			text, err := readInput(file, stdin)
			if err != nil {
				return commandLineError(stderr, err)
			}
			inputs = append(inputs, fmtInput{file, text})
		}
	}
	format := risor.Format
	if f.template {
		format = risor.FormatTemplate
	}
	color := f.color.on(stderr, stdout)
	status := 0
	for _, in := range inputs {
		out, err := format(in.text)
		if err != nil {
			// The path, then the first parse diagnostic (4.6 item 6).
			ds, _ := diagnosticsOf(err)
			if len(ds) > 1 {
				ds = ds[:1]
			}
			fmt.Fprintln(stderr, in.path)
			src := in.text
			writeDiagnostics(stderr, ds, &src, "text", color)
			status = 1
			continue
		}
		switch {
		case f.check:
			if out != in.text {
				fmt.Fprintln(stdout, in.path)
				status = 1
			}
		case f.write:
			if out != in.text {
				if err := os.WriteFile(in.path, []byte(out), 0o644); err != nil {
					return commandLineError(stderr, cmdErrorf("%s cannot be written: %v", in.path, err))
				}
			}
		default:
			fmt.Fprint(stdout, out)
		}
	}
	return status
}
