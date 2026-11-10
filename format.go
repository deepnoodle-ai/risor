package risor

import (
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v3/internal/format"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is the formatter of package F12 (spec/packages/f12-tools.md,
// section 2).

// Format gives the formatted text of a program (F12 2.1): the canonical
// text, with a line width of 80 and an indentation of two spaces. It needs
// no environment and does not check the program. A text that does not parse
// gives an *Error with its first parse diagnostic.
func Format(source string) (string, error) {
	if !utf8.ValidString(source) {
		source = value.ReplaceInvalid(source)
	}
	out, err, f := format.Program(source)
	if err != nil {
		return "", &Error{Diagnostics: []Diagnostic{err.Diagnostic(f)}}
	}
	return out, nil
}

// FormatTemplate gives the formatted text of a template (F12 2.14): each
// text run as it stands, and each part's expression laid out on one line
// between "${{ " and " }}" when it can be. A template that does not parse
// gives an *Error with its first parse diagnostic.
func FormatTemplate(source string) (string, error) {
	if !utf8.ValidString(source) {
		source = value.ReplaceInvalid(source)
	}
	out, err, f := format.Template(source)
	if err != nil {
		return "", &Error{Diagnostics: []Diagnostic{err.Diagnostic(f)}}
	}
	return out, nil
}
