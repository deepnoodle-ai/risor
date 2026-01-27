package errors

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/wonton/color"
)

// Formatter formats errors with colors and professional styling.
type Formatter struct {
	// UseColor enables ANSI color codes in output.
	UseColor bool
}

// NewFormatter creates a new error formatter.
func NewFormatter(useColor bool) *Formatter {
	return &Formatter{UseColor: useColor}
}

// Colors used for error formatting
var (
	colorError      = color.Red
	colorErrorBold  = color.BrightRed
	colorCode       = color.BrightBlack
	colorLocation   = color.Cyan
	colorLineNum    = color.BrightBlack
	colorPipe       = color.BrightBlack
	colorSource     = color.White
	colorCaret      = color.BrightRed
	colorHint       = color.BrightYellow
	colorNote       = color.BrightBlue
)

// FormattedError represents an error ready for display.
type FormattedError struct {
	Code        ErrorCode
	Kind        string             // "error", "parse error", "compile error", etc.
	Message     string
	Filename    string
	Line        int
	Column      int
	EndColumn   int                // For multi-character underlines
	SourceLines []SourceLineEntry  // Multiple lines for context
	Hint        string             // "Did you mean?" suggestion
	Note        string             // Additional context
	Stack       []StackFrame       // Stack trace for runtime errors
}

// SourceLineEntry represents a line of source code with its number.
type SourceLineEntry struct {
	Number int
	Text   string
	IsMain bool // True if this is the line with the error
}

// Format formats the error as a string using a consistent Rust-like style.
func (f *Formatter) Format(err *FormattedError) string {
	return f.FormatWithPrefix(err, "")
}

// FormatWithPrefix formats the error with an optional prefix like "[1/5]".
func (f *Formatter) FormatWithPrefix(err *FormattedError, prefix string) string {
	var b strings.Builder

	// Calculate line number width for consistent alignment
	lineNumWidth := 2
	if err.Line >= 100 {
		lineNumWidth = len(fmt.Sprintf("%d", err.Line))
	}

	// Error header: "error[E2001]: message" or "error[1/5]: message"
	f.writeHeader(&b, err, prefix)

	// Location arrow: "  --> file.risor:10:5"
	f.writeLocation(&b, err, lineNumWidth)

	// Source context with line numbers
	f.writeSource(&b, err, lineNumWidth)

	// Hint (e.g., "Did you mean?")
	if err.Hint != "" {
		f.writeHint(&b, err.Hint, lineNumWidth)
	}

	// Note
	if err.Note != "" {
		f.writeNote(&b, err.Note, lineNumWidth)
	}

	// Stack trace
	if len(err.Stack) > 0 {
		f.writeStack(&b, err.Stack, lineNumWidth)
	}

	return b.String()
}

func (f *Formatter) writeHeader(b *strings.Builder, err *FormattedError, prefix string) {
	// Determine what to show: "error", "error[E2001]", or "error[1/5]"
	label := "error"
	if err.Kind != "" && err.Kind != "error" {
		label = err.Kind
	}

	if f.UseColor {
		b.WriteString(colorErrorBold.Apply(label))
	} else {
		b.WriteString(label)
	}

	// Add code or prefix in brackets
	if err.Code != "" {
		bracket := fmt.Sprintf("[%s]", err.Code)
		if f.UseColor {
			b.WriteString(colorCode.Apply(bracket))
		} else {
			b.WriteString(bracket)
		}
	} else if prefix != "" {
		bracket := fmt.Sprintf("[%s]", prefix)
		if f.UseColor {
			b.WriteString(colorCode.Apply(bracket))
		} else {
			b.WriteString(bracket)
		}
	}

	// Message
	if f.UseColor {
		b.WriteString(colorError.Apply(": "))
	} else {
		b.WriteString(": ")
	}
	b.WriteString(err.Message)
	b.WriteString("\n")
}

func (f *Formatter) writeLocation(b *strings.Builder, err *FormattedError, lineNumWidth int) {
	if err.Line == 0 && err.Filename == "" {
		return
	}

	padding := strings.Repeat(" ", lineNumWidth)

	// Arrow line: "  --> file.risor:10:5"
	arrow := "-->"
	if f.UseColor {
		b.WriteString(colorLineNum.Apply(padding))
		b.WriteString(colorLocation.Apply(arrow))
		b.WriteString(" ")
	} else {
		b.WriteString(padding)
		b.WriteString(arrow)
		b.WriteString(" ")
	}

	loc := ""
	if err.Filename != "" {
		loc = err.Filename
		if err.Line > 0 {
			loc += fmt.Sprintf(":%d:%d", err.Line, err.Column)
		}
	} else if err.Line > 0 {
		loc = fmt.Sprintf("%d:%d", err.Line, err.Column)
	}

	if f.UseColor {
		b.WriteString(colorLocation.Apply(loc))
	} else {
		b.WriteString(loc)
	}
	b.WriteString("\n")
}

func (f *Formatter) writeSource(b *strings.Builder, err *FormattedError, lineNumWidth int) {
	if len(err.SourceLines) == 0 {
		return
	}

	padding := strings.Repeat(" ", lineNumWidth)

	// Empty pipe line for visual separation
	if f.UseColor {
		b.WriteString(colorLineNum.Apply(padding))
		b.WriteString(colorPipe.Apply(" |\n"))
	} else {
		b.WriteString(padding)
		b.WriteString(" |\n")
	}

	for _, line := range err.SourceLines {
		// Line number: " 6 |"
		lineNumStr := fmt.Sprintf("%*d", lineNumWidth, line.Number)
		if f.UseColor {
			b.WriteString(colorLineNum.Apply(lineNumStr))
			b.WriteString(colorPipe.Apply(" | "))
		} else {
			b.WriteString(lineNumStr)
			b.WriteString(" | ")
		}

		// Source text
		if f.UseColor {
			b.WriteString(colorSource.Apply(line.Text))
		} else {
			b.WriteString(line.Text)
		}
		b.WriteString("\n")

		// Caret line for the main error line
		if line.IsMain && err.Column > 0 {
			if f.UseColor {
				b.WriteString(colorLineNum.Apply(padding))
				b.WriteString(colorPipe.Apply(" | "))
			} else {
				b.WriteString(padding)
				b.WriteString(" | ")
			}

			// Spaces to reach the error column
			caretPad := strings.Repeat(" ", err.Column-1)
			b.WriteString(caretPad)

			// Carets under the error
			caretLen := 1
			if err.EndColumn > err.Column {
				caretLen = err.EndColumn - err.Column + 1
			}
			carets := strings.Repeat("^", caretLen)
			if f.UseColor {
				b.WriteString(colorCaret.Apply(carets))
			} else {
				b.WriteString(carets)
			}
			b.WriteString("\n")
		}
	}
}

func (f *Formatter) writeHint(b *strings.Builder, hint string, lineNumWidth int) {
	padding := strings.Repeat(" ", lineNumWidth)

	// Empty line then hint
	if f.UseColor {
		b.WriteString(colorLineNum.Apply(padding))
		b.WriteString(colorPipe.Apply(" |\n"))
		b.WriteString(colorLineNum.Apply(padding))
		b.WriteString(colorPipe.Apply(" = "))
		b.WriteString(colorHint.Apply("hint: "))
	} else {
		b.WriteString(padding)
		b.WriteString(" |\n")
		b.WriteString(padding)
		b.WriteString(" = ")
		b.WriteString("hint: ")
	}
	b.WriteString(hint)
	b.WriteString("\n")
}

func (f *Formatter) writeNote(b *strings.Builder, note string, lineNumWidth int) {
	padding := strings.Repeat(" ", lineNumWidth)

	if f.UseColor {
		b.WriteString(colorLineNum.Apply(padding))
		b.WriteString(colorPipe.Apply(" = "))
		b.WriteString(colorNote.Apply("note: "))
	} else {
		b.WriteString(padding)
		b.WriteString(" = ")
		b.WriteString("note: ")
	}
	b.WriteString(note)
	b.WriteString("\n")
}

func (f *Formatter) writeStack(b *strings.Builder, stack []StackFrame, lineNumWidth int) {
	padding := strings.Repeat(" ", lineNumWidth)

	// Empty line then stack trace
	if f.UseColor {
		b.WriteString(colorLineNum.Apply(padding))
		b.WriteString(colorPipe.Apply(" |\n"))
		b.WriteString(colorLineNum.Apply(padding))
		b.WriteString(colorPipe.Apply(" = "))
		b.WriteString(colorNote.Apply("stack trace:\n"))
	} else {
		b.WriteString(padding)
		b.WriteString(" |\n")
		b.WriteString(padding)
		b.WriteString(" = ")
		b.WriteString("stack trace:\n")
	}

	for _, frame := range stack {
		if f.UseColor {
			b.WriteString(colorLineNum.Apply(padding))
			b.WriteString(colorPipe.Apply("     "))
		} else {
			b.WriteString(padding)
			b.WriteString("     ")
		}
		b.WriteString(frame.String())
		b.WriteString("\n")
	}
}

// FormatMultiple formats multiple errors with consistent styling.
func (f *Formatter) FormatMultiple(errs []*FormattedError) string {
	if len(errs) == 0 {
		return ""
	}

	// Single error - no numbering needed
	if len(errs) == 1 {
		return f.Format(errs[0])
	}

	var b strings.Builder
	total := len(errs)

	for i, err := range errs {
		if i > 0 {
			b.WriteString("\n")
		}
		prefix := fmt.Sprintf("%d/%d", i+1, total)
		b.WriteString(f.FormatWithPrefix(err, prefix))
	}

	// Summary
	b.WriteString("\n")
	summary := fmt.Sprintf("found %d errors", total)
	if f.UseColor {
		b.WriteString(colorErrorBold.Apply(summary))
	} else {
		b.WriteString(summary)
	}
	b.WriteString("\n")

	return b.String()
}
