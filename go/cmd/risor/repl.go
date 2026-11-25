package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/risorcli"
	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/tui"
)

// The REPL of package F12 5, a product of this implementation. An entry is
// zero or more const lines, then one expression or nothing. The session
// holds the const lines of the entries that succeeded; an entry's program
// is the held lines, then the entry. A later const of a held name shadows
// it: the held binding is renamed, with every use of it, so that the lines
// after see the new binding and what was held keeps its own.

const replHelp = `Enter const lines and an expression. Commands:
  :reset  forget the held const lines
  :quit   leave
`

// session is the state of a REPL.
type session struct {
	env   *risor.Env
	vars  risor.Map
	color bool
	core  *risorcli.Session
}

func (s *session) coreSession() *risorcli.Session {
	if s.core == nil {
		s.core = risorcli.NewSession(s.env, s.vars)
	}
	return s.core
}

// result is what an entry shows.
type result struct {
	value string // the value's JSON form, and U+000A; empty for none
	diags string // the rendered diagnostics, or empty
	more  bool   // the entry needs more lines
}

// eval evaluates one entry.
func (s *session) eval(entry string) result {
	r := s.coreSession().Eval(entry)
	return result{
		value: r.Value,
		diags: s.formatError(r.Diagnostics, r.Err, entry),
		more:  r.More,
	}
}

func (s *session) formatError(ds []risor.Diagnostic, err error, entry string) string {
	if len(ds) > 0 {
		var b strings.Builder
		writeDiagnostics(&b, ds, &entry, "text", s.color)
		return b.String()
	}
	if err != nil {
		return "risor: " + err.Error() + "\n"
	}
	return ""
}

// replFlagDefs are the flags of the REPL: those of F12 4.3, and --color.
func replFlagDefs() []cli.Flag {
	return append(envFlagDefs(), colorFlagDef())
}

// errInterrupted is an entry the author abandoned (Ctrl+C).
var errInterrupted = errors.New("interrupted")

// lineReader reads the lines of the REPL. It gives io.EOF at the end of
// input and errInterrupted when the author abandons the line.
type lineReader interface {
	readLine(prompt string) (string, error)
}

// scanReader reads lines from a stream that is not a terminal, with no
// prompt and no editing: piped input and tests.
type scanReader struct{ sc *bufio.Scanner }

func (r *scanReader) readLine(string) (string, error) {
	if r.sc.Scan() {
		return r.sc.Text(), nil
	}
	if err := r.sc.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// promptReader reads lines from the terminal with the line editor of
// wonton/tui: cursor movement, word and line deletion, and history. A
// reader that is not a terminal gives the editor the keys a terminal in
// raw mode sends, which is how the tests drive it.
type promptReader struct {
	in      io.Reader
	out     io.Writer
	history []string
	style   tui.Style
}

func (r *promptReader) readLine(prompt string) (string, error) {
	line, err := tui.Prompt(prompt,
		tui.WithHistory(&r.history),
		tui.WithPromptInput(r.in),
		tui.WithPromptOutput(r.out),
		tui.WithPromptStyle(r.style),
		tui.WithPromptFrame(false), // a plain REPL prompt
	)
	// The editor ends the line with "\n" while the terminal is still in
	// raw mode, where "\n" moves down but does not return to column 0.
	// Return the cursor, so the value starts at the left edge.
	fmt.Fprint(r.out, "\r")
	switch {
	case errors.Is(err, tui.ErrEOF):
		return "", io.EOF
	case errors.Is(err, tui.ErrInterrupted):
		return "", errInterrupted
	}
	return line, err
}

// replCommand is risor with no command (F12 5): the REPL, in the
// environment of the flags of 4.3.
func replCommand(ctx *cli.Context) int {
	stdin, stdout, stderr := ctx.Stdin(), ctx.Stdout(), ctx.Stderr()
	color := colorOf(ctx)
	ef, err := envFlagsOf(ctx)
	if err != nil {
		return commandLineError(stderr, err)
	}
	env, vars, err := ef.environment()
	if err != nil {
		if ds, ok := diagnosticsOf(err); ok {
			writeDiagnostics(stderr, ds, nil, "text", color.on(stderr, stdout))
			return 1
		}
		return commandLineError(stderr, err)
	}
	s := &session{env: env, vars: vars, color: color.on(stdout, stdout)}
	// The line editor is used when standard input and output are
	// terminals; otherwise lines are read plainly, with no prompt.
	var in lineReader
	if f, ok := stdin.(*os.File); ok && isTerminal(f) && isTerminal(stdout) {
		style := tui.NewStyle()
		if s.color {
			style = style.WithForeground(tui.ColorCyan)
		}
		in = &promptReader{in: f, out: stdout, style: style}
	} else {
		sc := bufio.NewScanner(stdin)
		sc.Buffer(make([]byte, 64*1024), 16<<20)
		in = &scanReader{sc}
	}
	return repl(s, in, stdout, stderr)
}

// repl reads and evaluates entries until the input ends or :quit.
func repl(s *session, in lineReader, stdout, stderr io.Writer) int {
	var pending []string
	for {
		prompt := "> "
		if len(pending) > 0 {
			prompt = ". "
		}
		line, err := in.readLine(prompt)
		if errors.Is(err, errInterrupted) {
			pending = nil
			continue
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return commandLineError(stderr, err)
			}
			break
		}
		if len(pending) == 0 {
			switch strings.TrimSpace(line) {
			case ":quit", ":exit":
				return 0
			case ":reset":
				s.coreSession().Reset()
				continue
			case ":help":
				fmt.Fprint(stdout, replHelp)
				continue
			}
		}
		pending = append(pending, line)
		r := s.eval(strings.Join(pending, "\n"))
		if r.more {
			continue
		}
		pending = nil
		fmt.Fprint(stdout, r.value)
		fmt.Fprint(stdout, r.diags)
	}
	if len(pending) > 0 {
		// The input ended inside an entry: show why it does not parse.
		text := strings.Join(pending, "\n")
		ds, err := s.coreSession().EOFError(text)
		fmt.Fprint(stdout, s.formatError(ds, err, text))
	}
	return 0
}
