package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/tui"
)

// replApp implements tui.InlineApplication for the Risor REPL.
type replApp struct {
	runner      *tui.InlineApp
	ctx         context.Context
	vm          *replVM
	input       string
	cursorPos   int
	history     []string
	historyIdx  int
	historyPath string
	showTiming  bool
	multiLine   bool // true when input contains newlines
}

func runRepl(ctx context.Context, env map[string]any) error {
	// Load history
	history, historyPath := loadHistory()

	// Create VM with environment
	vm, err := newReplVM(env)
	if err != nil {
		return err
	}

	app := &replApp{
		ctx:         ctx,
		vm:          vm,
		history:     history,
		historyIdx:  -1,
		historyPath: historyPath,
	}

	app.runner = tui.NewInlineApp(tui.InlineAppConfig{
		BracketedPaste: true,
		KittyKeyboard:  true,
	})

	// Print branded header
	app.runner.Print(app.headerView())

	return app.runner.Run(app)
}

// headerView returns the branded REPL header with gradient logo
func (app *replApp) headerView() tui.View {
	// ASCII art logo
	artLines := []string{
		"  ██████╗ ██╗███████╗ ██████╗ ██████╗ ",
		"  ██╔══██╗██║██╔════╝██╔═══██╗██╔══██╗",
		"  ██████╔╝██║███████╗██║   ██║██████╔╝",
		"  ██╔══██╗██║╚════██║██║   ██║██╔══██╗",
		"  ██║  ██║██║███████║╚██████╔╝██║  ██║",
		"  ╚═╝  ╚═╝╚═╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝",
	}

	// Find max width for consistent gradient
	maxWidth := 0
	for _, line := range artLines {
		if w := len([]rune(line)); w > maxWidth {
			maxWidth = w
		}
	}

	// Build logo with gradient
	logoViews := make([]tui.View, len(artLines))
	for row, line := range artLines {
		runes := []rune(line)
		charViews := make([]tui.View, len(runes))

		for col, r := range runes {
			t := float64(col) / float64(maxWidth-1)
			c := interpolateGradient(t)
			charViews[col] = tui.Text("%c", r).Style(tui.NewStyle().WithFgRGB(c))
		}
		logoViews[row] = tui.Group(charViews...)
	}

	// Style constants
	accentColor := tui.RGB{R: 250, G: 180, B: 80}
	mutedColor := tui.RGB{R: 140, G: 140, B: 155}

	versionLine := tui.Group(
		tui.Text("  ◆ ").Style(tui.NewStyle().WithFgRGB(accentColor)),
		tui.Text("v%s", version).Style(tui.NewStyle().WithFgRGB(mutedColor)),
	)

	helpLine := tui.Group(
		tui.Text("  ◆ ").Style(tui.NewStyle().WithFgRGB(accentColor)),
		tui.Text("Type :help for commands").Style(tui.NewStyle().WithFgRGB(mutedColor)),
	)

	// Combine all views
	var views []tui.View
	views = append(views, tui.Text(""))
	views = append(views, logoViews...)
	views = append(views, tui.Text(""))
	views = append(views, versionLine)
	views = append(views, helpLine)
	views = append(views, tui.Text(""))
	return tui.Stack(views...).Gap(0)
}

// interpolateGradient returns a color along the Risor gradient (warm orange to gold)
func interpolateGradient(t float64) tui.RGB {
	type colorStop struct {
		pos   float64
		color tui.RGB
	}
	stops := []colorStop{
		{0.0, tui.RGB{R: 255, G: 150, B: 50}},   // warm orange
		{0.35, tui.RGB{R: 255, G: 200, B: 80}},  // gold
		{0.65, tui.RGB{R: 255, G: 180, B: 100}}, // amber
		{1.0, tui.RGB{R: 255, G: 140, B: 60}},   // deep orange
	}

	// Find the two stops we're between
	var lo, hi colorStop
	for i := 0; i < len(stops)-1; i++ {
		if t >= stops[i].pos && t <= stops[i+1].pos {
			lo = stops[i]
			hi = stops[i+1]
			break
		}
	}

	// Interpolate between the two stops
	localT := (t - lo.pos) / (hi.pos - lo.pos)
	return tui.RGB{
		R: uint8(float64(lo.color.R) + localT*(float64(hi.color.R)-float64(lo.color.R))),
		G: uint8(float64(lo.color.G) + localT*(float64(hi.color.G)-float64(lo.color.G))),
		B: uint8(float64(lo.color.B) + localT*(float64(hi.color.B)-float64(lo.color.B))),
	}
}

// LiveView returns the prompt view for the live region.
func (app *replApp) LiveView() tui.View {
	// Choose prompt based on multiline state
	prompt := ">>> "
	if app.multiLine {
		prompt = "... "
	}

	// Build the input line with cursor
	inputRunes := []rune(app.input)
	var beforeCursor, cursorChar, afterCursor string

	if app.cursorPos < len(inputRunes) {
		beforeCursor = string(inputRunes[:app.cursorPos])
		cursorChar = string(inputRunes[app.cursorPos])
		afterCursor = string(inputRunes[app.cursorPos+1:])
	} else {
		beforeCursor = app.input
		cursorChar = " "
		afterCursor = ""
	}

	// Style for prompt
	promptColor := tui.RGB{R: 250, G: 180, B: 80}

	return tui.Stack(
		tui.Divider(),
		tui.Group(
			tui.Text("%s", prompt).Style(tui.NewStyle().WithFgRGB(promptColor).WithBold()),
			tui.Text("%s", beforeCursor),
			tui.Text("%s", cursorChar).Reverse(),
			tui.Text("%s", afterCursor),
		),
		tui.Divider(),
	)
}

// HandleEvent processes keyboard events.
func (app *replApp) HandleEvent(event tui.Event) []tui.Cmd {
	keyEvent, ok := event.(tui.KeyEvent)
	if !ok {
		return nil
	}

	// Handle paste events
	if keyEvent.Paste != "" {
		app.insertString(keyEvent.Paste)
		app.updateMultiLine()
		return nil
	}

	switch keyEvent.Key {
	case tui.KeyEnter:
		// Shift+Enter for multi-line
		if keyEvent.Shift {
			app.insertRune('\n')
			app.updateMultiLine()
			return nil
		}
		// Submit
		return app.submit()

	case tui.KeyCtrlC:
		// Clear input if not empty, otherwise quit
		if len(app.input) > 0 {
			app.input = ""
			app.cursorPos = 0
			app.multiLine = false
			app.historyIdx = -1
			return nil
		}
		return []tui.Cmd{tui.Quit()}

	case tui.KeyCtrlD:
		if len(app.input) == 0 {
			return []tui.Cmd{tui.Quit()}
		}
		app.deleteChar()

	case tui.KeyBackspace:
		app.backspace()
		app.updateMultiLine()

	case tui.KeyArrowLeft:
		if keyEvent.Ctrl {
			app.wordLeft()
		} else if app.cursorPos > 0 {
			app.cursorPos--
		}

	case tui.KeyArrowRight:
		if keyEvent.Ctrl {
			app.wordRight()
		} else if app.cursorPos < len([]rune(app.input)) {
			app.cursorPos++
		}

	case tui.KeyHome, tui.KeyCtrlA:
		app.cursorPos = 0

	case tui.KeyEnd, tui.KeyCtrlE:
		app.cursorPos = len([]rune(app.input))

	case tui.KeyCtrlU:
		app.input = ""
		app.cursorPos = 0
		app.multiLine = false

	case tui.KeyCtrlK:
		inputRunes := []rune(app.input)
		app.input = string(inputRunes[:app.cursorPos])
		app.updateMultiLine()

	case tui.KeyCtrlW:
		// Delete word backward
		app.deleteWordBackward()
		app.updateMultiLine()

	case tui.KeyArrowUp:
		app.historyUp()
		app.updateMultiLine()

	case tui.KeyArrowDown:
		app.historyDown()
		app.updateMultiLine()

	default:
		if keyEvent.Rune != 0 {
			app.insertRune(keyEvent.Rune)
			if keyEvent.Rune == '\n' {
				app.updateMultiLine()
			}
		}
	}

	return nil
}

func (app *replApp) updateMultiLine() {
	app.multiLine = strings.Contains(app.input, "\n")
}

func (app *replApp) wordLeft() {
	inputRunes := []rune(app.input)
	if app.cursorPos == 0 {
		return
	}

	// Skip any spaces/punctuation behind us
	for app.cursorPos > 0 && !isWordChar(inputRunes[app.cursorPos-1]) {
		app.cursorPos--
	}
	// Move to the start of the word
	for app.cursorPos > 0 && isWordChar(inputRunes[app.cursorPos-1]) {
		app.cursorPos--
	}
}

func (app *replApp) wordRight() {
	inputRunes := []rune(app.input)
	n := len(inputRunes)
	if app.cursorPos >= n {
		return
	}

	// Skip current word
	for app.cursorPos < n && isWordChar(inputRunes[app.cursorPos]) {
		app.cursorPos++
	}
	// Skip spaces/punctuation
	for app.cursorPos < n && !isWordChar(inputRunes[app.cursorPos]) {
		app.cursorPos++
	}
}

func (app *replApp) deleteWordBackward() {
	if app.cursorPos == 0 {
		return
	}

	inputRunes := []rune(app.input)
	endPos := app.cursorPos

	// Skip any spaces/punctuation behind us
	for app.cursorPos > 0 && !isWordChar(inputRunes[app.cursorPos-1]) {
		app.cursorPos--
	}
	// Move to the start of the word
	for app.cursorPos > 0 && isWordChar(inputRunes[app.cursorPos-1]) {
		app.cursorPos--
	}

	// Delete from cursorPos to endPos
	app.input = string(inputRunes[:app.cursorPos]) + string(inputRunes[endPos:])
	app.historyIdx = -1
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (app *replApp) submit() []tui.Cmd {
	input := strings.TrimSpace(app.input)

	if input == "" {
		// Clear input state
		app.input = ""
		app.cursorPos = 0
		app.historyIdx = -1
		app.multiLine = false
		return nil
	}

	// Handle REPL commands (start with :) - these don't need continuation
	if strings.HasPrefix(input, ":") {
		// Clear input state
		app.input = ""
		app.cursorPos = 0
		app.historyIdx = -1
		app.multiLine = false

		// Style for prompt
		promptColor := tui.RGB{R: 250, G: 180, B: 80}

		// Print input to scrollback
		app.runner.Print(tui.Group(
			tui.Text(">>> ").Style(tui.NewStyle().WithFgRGB(promptColor).WithBold()),
			tui.Text("%s", input),
		))

		return app.handleCommand(input)
	}

	// Try to evaluate - check if input is incomplete
	start := time.Now()
	result, err := app.vm.Eval(app.ctx, input)
	elapsed := time.Since(start)

	// Check if the error indicates incomplete input
	if err != nil && isIncompleteInput(err) {
		// Don't clear input - add a newline and continue in multi-line mode
		app.input = app.input + "\n"
		app.cursorPos = len([]rune(app.input))
		app.multiLine = true
		return nil
	}

	// Input is complete (success or real error) - clear state and print
	app.input = ""
	app.cursorPos = 0
	app.historyIdx = -1
	app.multiLine = false

	// Style for prompt
	promptColor := tui.RGB{R: 250, G: 180, B: 80}

	// Print input to scrollback
	app.runner.Print(tui.Group(
		tui.Text(">>> ").Style(tui.NewStyle().WithFgRGB(promptColor).WithBold()),
		tui.Text("%s", input),
	))

	// Add to history
	app.history = append(app.history, input)
	appendToHistory(app.historyPath, input)

	// Print result
	if err != nil {
		app.runner.Print(tui.Text("%s", err.Error()).Fg(tui.ColorRed).Wrap())
	} else if result != nil {
		app.printResult(result)
	}

	// Optionally show timing
	if app.showTiming {
		app.runner.Print(tui.Text("%v", elapsed).Style(
			tui.NewStyle().WithFgRGB(tui.RGB{R: 140, G: 140, B: 155}),
		))
	}

	return nil
}

// isIncompleteInput returns true if the error indicates the input is incomplete
// and the user should continue typing (e.g., unclosed bracket, incomplete block).
// Note: We don't auto-continue for string literals since Risor strings can't span lines.
func isIncompleteInput(err error) bool {
	msg := err.Error()

	// Don't auto-continue for string/escape errors - these can't be fixed by adding lines
	if strings.Contains(msg, "string literal") || strings.Contains(msg, "escape sequence") {
		return false
	}

	// Auto-continue for structural incompleteness:
	// - "unterminated block statement"
	// - "unterminated function parameters"
	// - "unterminated switch statement"
	if strings.Contains(msg, "unterminated") {
		return true
	}

	// Auto-continue for unexpected end of file (unclosed brackets, parens, etc.)
	// e.g., "unexpected end of file while parsing map (expected })"
	if strings.Contains(msg, "unexpected end of file") {
		return true
	}

	return false
}

func (app *replApp) handleCommand(input string) []tui.Cmd {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	mutedStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 140, G: 140, B: 155})
	accentStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 250, G: 180, B: 80})

	switch cmd {
	case ":type", ":t":
		if len(parts) < 2 {
			app.runner.Print(tui.Text("  Usage: :type <expression>").Style(mutedStyle))
			return nil
		}
		expr := strings.TrimSpace(input[len(parts[0]):])
		obj, err := app.vm.EvalObject(app.ctx, expr)
		if err != nil {
			app.runner.Print(tui.Text("  %s", err.Error()).Fg(tui.ColorRed))
			return nil
		}
		app.runner.Print(tui.Group(
			tui.Text("  ").Style(mutedStyle),
			tui.Text("%s", obj.Type()).Style(accentStyle),
		))

	case ":methods", ":m":
		if len(parts) < 2 {
			app.runner.Print(tui.Text("  Usage: :methods <expression>").Style(mutedStyle))
			return nil
		}
		expr := strings.TrimSpace(input[len(parts[0]):])
		obj, err := app.vm.EvalObject(app.ctx, expr)
		if err != nil {
			app.runner.Print(tui.Text("  %s", err.Error()).Fg(tui.ColorRed))
			return nil
		}
		introspectable, ok := obj.(object.Introspectable)
		if !ok {
			app.runner.Print(tui.Group(
				tui.Text("  ").Style(mutedStyle),
				tui.Text("%s", obj.Type()).Style(accentStyle),
				tui.Text(" has no methods").Style(mutedStyle),
			))
			return nil
		}
		attrs := introspectable.Attrs()
		if len(attrs) == 0 {
			app.runner.Print(tui.Group(
				tui.Text("  ").Style(mutedStyle),
				tui.Text("%s", obj.Type()).Style(accentStyle),
				tui.Text(" has no methods").Style(mutedStyle),
			))
			return nil
		}
		app.runner.Print(tui.Group(
			tui.Text("  ").Style(mutedStyle),
			tui.Text("%s", obj.Type()).Style(accentStyle),
			tui.Text(" methods:").Style(mutedStyle),
		))
		// Display methods with signatures
		for _, attr := range attrs {
			var sig string
			if len(attr.Args) > 0 {
				sig = fmt.Sprintf(".%s(%s)", attr.Name, strings.Join(attr.Args, ", "))
			} else {
				sig = fmt.Sprintf(".%s()", attr.Name)
			}
			app.runner.Print(tui.Group(
				tui.Text("    %s", sig).Style(accentStyle),
				tui.Text("  %s", attr.Doc).Style(mutedStyle),
			))
		}

	case ":help", ":h", ":?":
		app.runner.Print(tui.Stack(
			tui.Text(""),
			tui.Group(
				tui.Text("  :help, :h, :?   ").Style(accentStyle),
				tui.Text("  Show this help").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  :type, :t <expr>").Style(accentStyle),
				tui.Text("  Show type of expression").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  :methods <expr> ").Style(accentStyle),
				tui.Text("  List methods on a value").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  :env            ").Style(accentStyle),
				tui.Text("  List available globals").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  :timing         ").Style(accentStyle),
				tui.Text("  Toggle execution timing").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  :clear, :cls    ").Style(accentStyle),
				tui.Text("  Clear the screen").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  :exit, :quit    ").Style(accentStyle),
				tui.Text("  Exit the REPL").Style(mutedStyle),
			),
			tui.Text(""),
			tui.Group(
				tui.Text("  Shift+Enter").Style(accentStyle),
				tui.Text("   Multi-line input").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  Ctrl+C      ").Style(accentStyle),
				tui.Text("   Clear input / Exit").Style(mutedStyle),
			),
			tui.Group(
				tui.Text("  Ctrl+W      ").Style(accentStyle),
				tui.Text("   Delete word backward").Style(mutedStyle),
			),
			tui.Text(""),
		).Gap(0))

	case ":clear", ":cls":
		app.runner.ClearScrollback()
		app.runner.Print(app.headerView())

	case ":env":
		names := app.vm.GlobalNames()
		if len(names) == 0 {
			app.runner.Print(tui.Text("  (no globals)").Style(mutedStyle))
		} else {
			// Display in columns
			app.runner.Print(tui.Text("  %s", strings.Join(names, ", ")).Style(mutedStyle).Wrap())
		}

	case ":timing":
		app.showTiming = !app.showTiming
		if app.showTiming {
			app.runner.Print(tui.Text("  Timing enabled").Style(mutedStyle))
		} else {
			app.runner.Print(tui.Text("  Timing disabled").Style(mutedStyle))
		}

	case ":exit", ":quit", ":q":
		return []tui.Cmd{tui.Quit()}

	default:
		app.runner.Print(tui.Text("  Unknown command: %s", cmd).Fg(tui.ColorRed))
	}

	return nil
}

const maxResultLines = 50

func (app *replApp) printResult(result any) {
	// Color scheme
	numberStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 100}) // yellow-gold
	stringStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 150, G: 220, B: 150}) // soft green
	boolStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220})   // soft purple
	typeStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 100, B: 110})   // muted gray

	switch v := result.(type) {
	case error:
		app.runner.Print(tui.Text("%s", v.Error()).Fg(tui.ColorRed))

	case nil:
		app.runner.Print(tui.Group(
			tui.Text("null").Style(boolStyle),
		))

	case int64:
		app.runner.Print(tui.Group(
			tui.Text("%d", v).Style(numberStyle),
			tui.Text(" int").Style(typeStyle),
		))

	case float64:
		app.runner.Print(tui.Group(
			tui.Text("%g", v).Style(numberStyle),
			tui.Text(" float").Style(typeStyle),
		))

	case bool:
		app.runner.Print(tui.Group(
			tui.Text("%v", v).Style(boolStyle),
			tui.Text(" bool").Style(typeStyle),
		))

	case string:
		// Truncate long strings
		display := v
		truncated := false
		if len(v) > 500 {
			display = v[:500]
			truncated = true
		}
		if truncated {
			app.runner.Print(tui.Group(
				tui.Text("%q", display).Style(stringStyle).Wrap(),
				tui.Text("… +%d chars", len(v)-500).Style(typeStyle),
			))
		} else {
			app.runner.Print(tui.Group(
				tui.Text("%q", display).Style(stringStyle).Wrap(),
				tui.Text(" string").Style(typeStyle),
			))
		}

	case byte:
		app.runner.Print(tui.Group(
			tui.Text("%d", v).Style(numberStyle),
			tui.Text(" byte").Style(typeStyle),
		))

	case []byte:
		app.runner.Print(tui.Group(
			tui.Text("%q", string(v)).Style(stringStyle).Wrap(),
			tui.Text(" bytes(%d)", len(v)).Style(typeStyle),
		))

	case []any:
		app.printList(v, typeStyle)

	case map[string]any:
		app.printMap(v, typeStyle)

	default:
		// For other types, format as string
		formatted := fmt.Sprintf("%v", v)
		lines := strings.Split(formatted, "\n")
		if len(lines) > maxResultLines {
			truncated := strings.Join(lines[:maxResultLines], "\n")
			app.runner.Print(tui.Text("%s", truncated).Wrap())
			app.runner.Print(tui.Text("… +%d lines", len(lines)-maxResultLines).Style(typeStyle))
		} else {
			app.runner.Print(tui.Text("%s", formatted).Wrap())
		}
		app.runner.Print(tui.Text(" %T", v).Style(typeStyle))
	}
}

func (app *replApp) printList(list []any, typeStyle tui.Style) {
	if len(list) == 0 {
		app.runner.Print(tui.Group(
			tui.Text("[]"),
			tui.Text(" list(0)").Style(typeStyle),
		))
		return
	}

	// Format as JSON-like output
	formatted := formatValue(list)
	lines := strings.Split(formatted, "\n")

	if len(lines) > maxResultLines {
		truncated := strings.Join(lines[:maxResultLines], "\n")
		app.runner.Print(tui.Text("%s", truncated).Wrap())
		app.runner.Print(tui.Text("… +%d lines", len(lines)-maxResultLines).Style(typeStyle))
	} else {
		app.runner.Print(tui.Text("%s", formatted).Wrap())
	}
	app.runner.Print(tui.Text(" list(%d)", len(list)).Style(typeStyle))
}

func (app *replApp) printMap(m map[string]any, typeStyle tui.Style) {
	if len(m) == 0 {
		app.runner.Print(tui.Group(
			tui.Text("{}"),
			tui.Text(" map(0)").Style(typeStyle),
		))
		return
	}

	// Format as JSON-like output
	formatted := formatValue(m)
	lines := strings.Split(formatted, "\n")

	if len(lines) > maxResultLines {
		truncated := strings.Join(lines[:maxResultLines], "\n")
		app.runner.Print(tui.Text("%s", truncated).Wrap())
		app.runner.Print(tui.Text("… +%d lines", len(lines)-maxResultLines).Style(typeStyle))
	} else {
		app.runner.Print(tui.Text("%s", formatted).Wrap())
	}
	app.runner.Print(tui.Text(" map(%d)", len(m)).Style(typeStyle))
}

func formatValue(v any) string {
	// Use JSON marshaling for nice formatting
	data, err := jsonMarshalIndent(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

func jsonMarshalIndent(v any) ([]byte, error) {
	// Simple JSON formatting
	switch val := v.(type) {
	case []any:
		if len(val) == 0 {
			return []byte("[]"), nil
		}
		var parts []string
		for _, item := range val {
			itemBytes, _ := jsonMarshalIndent(item)
			parts = append(parts, "  "+string(itemBytes))
		}
		return []byte("[\n" + strings.Join(parts, ",\n") + "\n]"), nil

	case map[string]any:
		if len(val) == 0 {
			return []byte("{}"), nil
		}
		var parts []string
		for k, item := range val {
			itemBytes, _ := jsonMarshalIndent(item)
			parts = append(parts, fmt.Sprintf("  %q: %s", k, string(itemBytes)))
		}
		return []byte("{\n" + strings.Join(parts, ",\n") + "\n}"), nil

	case string:
		return []byte(fmt.Sprintf("%q", val)), nil

	default:
		return []byte(fmt.Sprintf("%v", val)), nil
	}
}

func (app *replApp) insertRune(r rune) {
	inputRunes := []rune(app.input)
	inputRunes = append(inputRunes[:app.cursorPos], append([]rune{r}, inputRunes[app.cursorPos:]...)...)
	app.input = string(inputRunes)
	app.cursorPos++
	app.historyIdx = -1
}

func (app *replApp) insertString(s string) {
	for _, r := range s {
		app.insertRune(r)
	}
}

func (app *replApp) backspace() {
	if app.cursorPos > 0 {
		inputRunes := []rune(app.input)
		inputRunes = append(inputRunes[:app.cursorPos-1], inputRunes[app.cursorPos:]...)
		app.input = string(inputRunes)
		app.cursorPos--
		app.historyIdx = -1
	}
}

func (app *replApp) deleteChar() {
	inputRunes := []rune(app.input)
	if app.cursorPos < len(inputRunes) {
		inputRunes = append(inputRunes[:app.cursorPos], inputRunes[app.cursorPos+1:]...)
		app.input = string(inputRunes)
	}
}

func (app *replApp) historyUp() {
	if len(app.history) == 0 {
		return
	}

	if app.historyIdx == -1 {
		app.historyIdx = len(app.history)
	}

	if app.historyIdx > 0 {
		app.historyIdx--
		app.input = app.history[app.historyIdx]
		app.cursorPos = len([]rune(app.input))
	}
}

func (app *replApp) historyDown() {
	if app.historyIdx == -1 {
		return
	}

	app.historyIdx++
	if app.historyIdx >= len(app.history) {
		app.input = ""
		app.historyIdx = -1
	} else {
		app.input = app.history[app.historyIdx]
	}
	app.cursorPos = len([]rune(app.input))
}

func loadHistory() ([]string, string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, ""
	}

	historyPath := filepath.Join(homeDir, ".risor_history")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil, historyPath
	}

	lines := strings.Split(string(data), "\n")
	history := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			history = append(history, line)
		}
	}
	return history, historyPath
}

func appendToHistory(path, line string) {
	if path == "" || line == "" {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
}
