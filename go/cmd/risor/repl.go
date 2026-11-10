package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
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
	held  string // the held const lines, as program text
	color bool
	// internal maps each name shadow gave a held binding to the name its
	// author wrote, so that no internal name reaches the user.
	internal map[string]string
	// undo takes a span of the last entry's renamed program back to the
	// text the author wrote; nil when shadow renamed nothing.
	undo func(risor.Span) risor.Span
}

// result is what an entry shows.
type result struct {
	value string // the value's JSON form, and U+000A; empty for none
	diags string // the rendered diagnostics, or empty
	more  bool   // the entry needs more lines
}

// entryKind classifies an entry's text: whether it parses with an
// expression, parses as const lines alone, needs more lines, or fails.
func entryKind(text string) (hasExpr bool, ok bool, more bool, err error) {
	if _, err := risor.Parse(text); err == nil {
		return true, true, false, nil
	} else {
		if x, err2 := risor.Parse(text + "\nnull"); err2 == nil && onlyConsts(x, text) {
			return false, true, false, nil
		}
		ds, _ := diagnosticsOf(err)
		if len(ds) > 0 && atEnd(ds[0], text) {
			return false, false, true, nil
		}
		return false, false, false, err
	}
}

// onlyConsts reports whether a program is the const lines of text, then
// the null the REPL added.
func onlyConsts(x *risor.Expr, text string) bool {
	f := syntax.NewFile(text + "\nnull")
	p, err := syntax.Parse(f)
	if err != nil {
		return false
	}
	_, isNull := p.Result.(*syntax.Null)
	return isNull && p.Result.Span().Start >= len([]rune(strings.TrimPrefix(text, string(rune(0xFEFF)))))
}

// atEnd reports whether a parse diagnostic stands at the end of the text,
// where the grammar needs more.
func atEnd(d risor.Diagnostic, text string) bool {
	if d.Span == nil {
		return false
	}
	f := syntax.NewFile(text)
	end := f.Pos(f.Len())
	switch d.Code {
	case diag.CodeUnterminatedString, diag.CodeUnterminatedComment, diag.CodeTextBlockClose:
		return d.Span.End == end
	}
	return d.Span.Start == end
}

// lineCount is the number of lines of a text.
func lineCount(s string) int { return strings.Count(s, "\n") + 1 }

// program joins the held lines and an entry.
func (s *session) program(entry string) (string, int) {
	if s.held == "" {
		return entry, 0
	}
	return s.held + "\n" + entry, lineCount(s.held)
}

// eval evaluates one entry.
func (s *session) eval(entry string) result {
	if strings.TrimSpace(entry) == "" {
		return result{}
	}
	hasExpr, ok, more, err := entryKind(entry)
	if more {
		return result{more: true}
	}
	if !ok {
		return result{diags: s.render(err, entry, 0)}
	}
	text := entry
	if !hasExpr {
		text += "\nnull"
	}
	full, k := s.program(text)
	authoredFull := full
	full, renamed, passes := shadow(full, k)
	s.undo = nil
	if len(passes) > 0 {
		s.undo = spanBack(full, authoredFull, passes)
	}
	if s.internal == nil {
		s.internal = map[string]string{}
	}
	for fresh, name := range renamed {
		s.internal[fresh] = s.authored(name)
	}
	x, err := risor.Parse(full)
	if err != nil {
		return result{diags: s.render(err, entry, k)}
	}
	p, _, err := s.env.CheckCompile(x)
	if err != nil {
		return result{diags: s.render(err, entry, k)}
	}
	v, err := p.Run(context.Background(), s.vars)
	if err != nil {
		return result{diags: s.render(err, entry, k)}
	}
	s.held = heldLines(full)
	if !hasExpr {
		return result{}
	}
	js, err := risor.JSONText(v)
	if err != nil {
		if ds, ok := diagnosticsOf(err); ok && len(ds) > 0 && ds[0].Code == "wrong-kind" {
			return result{value: "an opaque value, which cannot be shown\n"}
		}
		return result{diags: s.render(err, entry, k)}
	}
	return result{value: js + "\n"}
}

// heldLines is the text of a program's const lines.
func heldLines(text string) string {
	f := syntax.NewFile(text)
	p, err := syntax.Parse(f)
	if err != nil || len(p.Consts) == 0 {
		return ""
	}
	return f.Text(0, p.Consts[len(p.Consts)-1].Span().End)
}

// render writes the diagnostics of a failure with the entry's text: a span
// in the held lines is left out, and a span in the entry is moved up by
// the k lines before it.
func (s *session) render(err error, entry string, k int) string {
	ds, ok := diagnosticsOf(err)
	if !ok {
		return "risor: " + err.Error() + "\n"
	}
	move := func(sp *risor.Span) *risor.Span {
		if sp == nil || sp.Start.Line <= k {
			return nil
		}
		m := *sp
		m.Start.Line -= k
		m.End.Line -= k
		return &m
	}
	out := make([]risor.Diagnostic, len(ds))
	back := func(sp *risor.Span) *risor.Span {
		if sp == nil || s.undo == nil {
			return sp
		}
		m := s.undo(*sp)
		return &m
	}
	for i, d := range ds {
		d.Span = move(back(d.Span))
		var tr []diag.TraceEntry
		for _, t := range d.Trace {
			if m := move(back(&t.Span)); m != nil {
				t.Span = *m
				tr = append(tr, t)
			}
		}
		d.Trace = tr
		d.Message = s.authoredText(d.Message)
		for j := range d.Trace {
			d.Trace[j].Name = s.authored(d.Trace[j].Name)
		}
		out[i] = d
	}
	var b strings.Builder
	writeDiagnostics(&b, out, &entry, "text", s.color)
	return b.String()
}

// shadow renames each held binding that a later const line of the entry
// binds again: the held const line and every use that refers to it, up to
// and including the value of the line that binds the name again. k is the
// number of lines before the entry.
//
// It returns the new text, each fresh name with the name it stands for,
// and the edits of each pass, so that a span can be taken back.
func shadow(text string, k int) (string, map[string]string, [][]edit) {
	renamed := map[string]string{}
	var passes [][]edit
	if k == 0 {
		return text, renamed, passes
	}
	for guard := 0; guard < 1000; guard++ {
		f := syntax.NewFile(text)
		p, err := syntax.Parse(f)
		if err != nil {
			return text, renamed, passes
		}
		heldEnd := -1
		for i, c := range p.Consts {
			if f.Pos(c.Span().Start).Line <= k {
				heldEnd = i
			}
		}
		// The first held name that a later line binds again.
		h, name := -1, ""
	find:
		for i := 0; i <= heldEnd; i++ {
			for _, n := range boundNames(p.Consts[i].Pattern) {
				for j := i + 1; j < len(p.Consts); j++ {
					for _, m := range boundNames(p.Consts[j].Pattern) {
						if m == n {
							h, name = i, n
							break find
						}
					}
				}
			}
		}
		if h < 0 {
			return text, renamed, passes
		}
		fresh := freshName(name, text)
		renamed[fresh] = name
		var eds []edit
		bindSite(p.Consts[h].Pattern, name, fresh, &eds)
		for j := h + 1; j < len(p.Consts); j++ {
			c := p.Consts[j]
			rebinds := false
			for _, m := range boundNames(c.Pattern) {
				if m == name {
					rebinds = true
				}
			}
			renamePatternDefaults(c.Pattern, name, fresh, &eds)
			renameFree(c.Value, name, fresh, &eds)
			if rebinds {
				break
			}
			if j == len(p.Consts)-1 {
				renameFree(p.Result, name, fresh, &eds)
			}
		}
		passes = append(passes, append([]edit(nil), eds...))
		text = applyEdits(f.Runes(), eds)
	}
	return text, renamed, passes
}

// spanBack takes a span of the text shadow made (to) back to the text it
// was given (from), undoing the passes' edits from the last one.
func spanBack(to, from string, passes [][]edit) func(risor.Span) risor.Span {
	tf, ff := syntax.NewFile(to), syntax.NewFile(from)
	offset := func(f *syntax.File, p diag.Pos) int {
		// The first offset at p's line, then its column.
		lo, hi := 0, f.Len()
		for lo < hi {
			mid := (lo + hi) / 2
			if f.Pos(mid).Line < p.Line {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return lo + p.Column - 1
	}
	pos := func(p diag.Pos, end bool) diag.Pos {
		off := offset(tf, p)
		for i := len(passes) - 1; i >= 0; i-- {
			off = editBack(passes[i], off, end)
		}
		return ff.Pos(off)
	}
	return func(sp risor.Span) risor.Span {
		return risor.Span{Start: pos(sp.Start, false), End: pos(sp.End, true)}
	}
}

// editBack takes an offset in the text after a set of edits back to the
// text before them. An offset inside a replacement goes to its start, or,
// for the end of a span, to its end.
func editBack(eds []edit, off int, end bool) int {
	sorted := append([]edit(nil), eds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start < sorted[j].start })
	delta := 0
	for _, e := range sorted {
		ns := e.start + delta
		ne := ns + len([]rune(e.text))
		if off <= ns {
			break
		}
		if off >= ne {
			delta += ne - ns - (e.end - e.start)
			continue
		}
		if end {
			return e.end
		}
		return e.start
	}
	return off - delta
}

// authored is the name the author wrote for a name of the program.
func (s *session) authored(name string) string {
	for {
		orig, ok := s.internal[name]
		if !ok || orig == name {
			return name
		}
		name = orig
	}
}

// authoredText replaces each internal name, as a whole word, in a message.
func (s *session) authoredText(msg string) string {
	if len(s.internal) == 0 || !strings.Contains(msg, "__") {
		return msg
	}
	var b strings.Builder
	rs := []rune(msg)
	for i := 0; i < len(rs); {
		if isNameStart(rs[i]) && (i == 0 || !isNamePart(rs[i-1])) {
			j := i
			for j < len(rs) && isNamePart(rs[j]) {
				j++
			}
			b.WriteString(s.authored(string(rs[i:j])))
			i = j
			continue
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}

func isNameStart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isNamePart(r rune) bool { return isNameStart(r) || (r >= '0' && r <= '9') }

// edit replaces the code points from start to end.
type edit struct {
	start, end int
	text       string
}

func applyEdits(src []rune, eds []edit) string {
	// Apply from the end, so that offsets stay valid.
	for i := 1; i < len(eds); i++ {
		for j := i; j > 0 && eds[j].start > eds[j-1].start; j-- {
			eds[j], eds[j-1] = eds[j-1], eds[j]
		}
	}
	out := append([]rune(nil), src...)
	for _, e := range eds {
		out = append(out[:e.start], append([]rune(e.text), out[e.end:]...)...)
	}
	return string(out)
}

// freshName is a name for a shadowed binding that the text does not use.
func freshName(name, text string) string {
	for n := 1; ; n++ {
		c := name + "__" + strconv.Itoa(n)
		if !strings.Contains(text, c) {
			return c
		}
	}
}

func boundNames(p syntax.Pattern) []string {
	var out []string
	switch p := p.(type) {
	case *syntax.NamePat:
		if !p.Discard {
			out = append(out, p.Name)
		}
	case *syntax.MapPat:
		for _, f := range p.Fields {
			out = append(out, f.Name)
		}
		if p.Rest != nil {
			out = append(out, p.Rest.Name)
		}
	case *syntax.ListPat:
		for _, e := range p.Elems {
			if e.Name != nil && !e.Name.Discard {
				out = append(out, e.Name.Name)
			}
		}
		if p.Rest != nil {
			out = append(out, p.Rest.Name)
		}
	}
	return out
}

// bindSite renames the place a pattern binds name.
func bindSite(p syntax.Pattern, name, fresh string, eds *[]edit) {
	switch p := p.(type) {
	case *syntax.NamePat:
		if p.Name == name {
			*eds = append(*eds, edit{p.Span().Start, p.Span().End, fresh})
		}
	case *syntax.MapPat:
		for _, f := range p.Fields {
			if f.Name != name {
				continue
			}
			if f.KeySpan == f.NameSpan { // {a} is {a: a}
				*eds = append(*eds, edit{f.NameSpan.Start, f.NameSpan.End, name + ": " + fresh})
			} else {
				*eds = append(*eds, edit{f.NameSpan.Start, f.NameSpan.End, fresh})
			}
		}
		if p.Rest != nil && p.Rest.Name == name {
			*eds = append(*eds, edit{p.Rest.Span().Start, p.Rest.Span().End, fresh})
		}
	case *syntax.ListPat:
		for _, e := range p.Elems {
			if e.Name != nil && e.Name.Name == name {
				*eds = append(*eds, edit{e.Name.Span().Start, e.Name.Span().End, fresh})
			}
		}
		if p.Rest != nil && p.Rest.Name == name {
			*eds = append(*eds, edit{p.Rest.Span().Start, p.Rest.Span().End, fresh})
		}
	}
}

// renamePatternDefaults renames the free uses of name in a pattern's
// defaults, up to a field that binds name.
func renamePatternDefaults(p syntax.Pattern, name, fresh string, eds *[]edit) {
	switch p := p.(type) {
	case *syntax.MapPat:
		for _, f := range p.Fields {
			renameFree(f.Default, name, fresh, eds)
			if f.Name == name {
				return
			}
		}
	case *syntax.ListPat:
		for _, e := range p.Elems {
			renameFree(e.Default, name, fresh, eds)
			if e.Name != nil && e.Name.Name == name {
				return
			}
		}
	}
}

func bindsName(p syntax.Pattern, name string) bool {
	for _, n := range boundNames(p) {
		if n == name {
			return true
		}
	}
	return false
}

// renameFree renames each use of name in n that is not bound inside n.
func renameFree(n syntax.Node, name, fresh string, eds *[]edit) {
	switch n := n.(type) {
	case nil:
	case *syntax.Ident:
		if n.Name == name {
			*eds = append(*eds, edit{n.Span().Start, n.Span().End, fresh})
		}
	case *syntax.Block:
		for _, c := range n.Consts {
			renamePatternDefaults(c.Pattern, name, fresh, eds)
			renameFree(c.Value, name, fresh, eds)
			if bindsName(c.Pattern, name) {
				return
			}
		}
		renameFree(n.Result, name, fresh, eds)
	case *syntax.Map:
		for _, x := range n.Entries {
			if e, ok := x.(*syntax.Entry); ok && e.Shorthand && e.Key == name {
				*eds = append(*eds, edit{e.Span().Start, e.Span().End, name + ": " + fresh})
				continue
			}
			renameFree(x, name, fresh, eds)
		}
	case *syntax.Lambda:
		for _, p := range n.Params {
			renamePatternDefaults(p.Pattern, name, fresh, eds)
			renameFree(p.Default, name, fresh, eds)
			if bindsName(p.Pattern, name) {
				return
			}
		}
		renameFree(n.Body, name, fresh, eds)
	case *syntax.Try:
		renameFree(n.Body, name, fresh, eds)
		if n.Name != name {
			renameFree(n.Handler, name, fresh, eds)
		}
	case *syntax.Call:
		// A call of a const-bound lambda names it by its callee.
		if n.Namespace == "" && n.Name == name {
			*eds = append(*eds, edit{n.NameSpan.Start, n.NameSpan.End, fresh})
		}
		for _, a := range n.Args {
			renameFree(a, name, fresh, eds)
		}
	default:
		for _, c := range syntax.Children(n) {
			renameFree(c, name, fresh, eds)
		}
	}
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
// wonton/tui: cursor movement, word and line deletion, and history.
type promptReader struct {
	out     io.Writer
	history []string
	style   tui.Style
}

func (r *promptReader) readLine(prompt string) (string, error) {
	line, err := tui.Prompt(prompt,
		tui.WithHistory(&r.history),
		tui.WithPromptOutput(r.out),
		tui.WithPromptStyle(r.style),
	)
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
	// The line editor reads the process's own standard input, so it is
	// used only when that is the stream given and it is a terminal.
	var in lineReader
	if f, ok := stdin.(*os.File); ok && f == os.Stdin && isTerminal(f) && isTerminal(stdout) {
		style := tui.NewStyle()
		if s.color {
			style = style.WithForeground(tui.ColorCyan)
		}
		in = &promptReader{out: stdout, style: style}
	} else {
		sc := bufio.NewScanner(stdin)
		sc.Buffer(make([]byte, 64*1024), 16<<20)
		in = &scanReader{sc}
	}
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
				s.held = ""
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
		_, _, _, err := entryKind(strings.Join(pending, "\n"))
		if err == nil {
			_, err = risor.Parse(strings.Join(pending, "\n"))
		}
		if err != nil {
			fmt.Fprint(stdout, s.render(err, strings.Join(pending, "\n"), 0))
		}
	}
	return 0
}
