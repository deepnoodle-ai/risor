package risor

import (
	"strconv"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
)

// MaxSource is the most code points a source text may have, 1,048,576
// (spec/core/01-language.md 2.1 item 4). A longer text fails to parse with
// the code source-too-large, of phase parse, with no span.
const MaxSource = syntax.MaxSource

// ParseOption is an option of a call that takes source text: Parse,
// ParseTemplate, Format and FormatTemplate. SourceLimit is the one option.
type ParseOption interface{ parseOption() }

// SourceLimit lowers the source limit of a call that takes source text
// (03-host.md 3.6.1): the most code points the text may have. It is a whole
// number from 0 to MaxSource; any other value is the declare error
// bad-limit. It is an option of each call, not a declaration of the
// environment, because parse needs no environment. Parse, ParseTemplate,
// Format and FormatTemplate take it as a ParseOption; Complete and Hover as
// a CheckOption, giving their empty result for a text over the limit; Load
// and LoadValue as a LoadOption. Check and Compile take a parsed program and
// ignore it.
type SourceLimit int

func (SourceLimit) parseOption() {}
func (SourceLimit) checkOption() {}
func (SourceLimit) loadOption()  {}

// sourceLimit gives the source limit an option list sets, the last one
// given, or MaxSource when none is given.
func sourceLimit[T any](opts []T) (int, error) {
	limit := MaxSource
	for _, o := range opts {
		if l, ok := any(o).(SourceLimit); ok {
			if l < 0 || int(l) > MaxSource {
				return 0, &Error{Diagnostics: []Diagnostic{{Phase: PhaseDeclare, Code: diag.CodeBadLimit,
					Message: "the source limit is " + strconv.Itoa(int(l)) + ", not a whole number from 0 to " + strconv.Itoa(MaxSource)}}}
			}
			limit = int(l)
		}
	}
	return limit, nil
}

// checkSource refuses a text over the source limit of opts, before it is
// lexed (01-language.md 2.1 item 5).
func checkSource[T any](source string, opts []T) error {
	limit, err := sourceLimit(opts)
	if err != nil {
		return err
	}
	if syntax.TooLarge(source, limit) {
		return &Error{Diagnostics: []Diagnostic{syntax.SizeDiagnostic(limit)}}
	}
	return nil
}
