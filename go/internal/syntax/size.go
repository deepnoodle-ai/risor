package syntax

import (
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// MaxSource is the most code points a source text may have
// (01-language.md 2.1 item 4). A host may set a lower source limit, never a
// higher one (03-host.md 3.6.1).
const MaxSource = diag.MaxSource

// TooLarge reports whether a source text has more code points than limit.
// The count is of the decoded text (03-host.md 3.4.5): each maximal subpart
// of ill-formed UTF-8 is one code point. A leading U+FEFF counts. It reads
// no more of the text than it must: a text of at most limit bytes has at
// most limit code points, and a text of more than 4 * limit bytes has more,
// since no code point of the decoded text takes more than four bytes.
func TooLarge(source string, limit int) bool {
	n := len(source)
	if n <= limit {
		return false
	}
	if n > 4*limit {
		return true
	}
	if !utf8.ValidString(source) {
		source = value.ReplaceInvalid(source)
	}
	return utf8.RuneCountInString(source) > limit
}

// SizeDiagnostic is the diagnostic of a text over the source limit: the
// code source-too-large, of phase parse, with no span (01-language.md 2.1
// item 5).
func SizeDiagnostic(limit int) diag.Diagnostic {
	return diag.Diagnostic{
		Phase:   diag.PhaseParse,
		Code:    diag.CodeSourceTooLarge,
		Message: messages.Build(diag.CodeSourceTooLarge, "", messages.Val("limit", float64(limit))),
	}
}
