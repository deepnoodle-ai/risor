package bytecode

import "github.com/deepnoodle-ai/risor/v2/op"

// copyStrings returns a copy of the given string slice.
func copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// copyAny returns a copy of the given any slice.
func copyAny(src []any) []any {
	if src == nil {
		return nil
	}
	dst := make([]any, len(src))
	copy(dst, src)
	return dst
}

// copyInstructions returns a copy of the given instruction slice.
func copyInstructions(src []op.Code) []op.Code {
	if src == nil {
		return nil
	}
	dst := make([]op.Code, len(src))
	copy(dst, src)
	return dst
}

// copyLocations returns a copy of the given location slice.
func copyLocations(src []SourceLocation) []SourceLocation {
	if src == nil {
		return nil
	}
	dst := make([]SourceLocation, len(src))
	copy(dst, src)
	return dst
}

// copyHandlers returns a copy of the given exception handler slice.
func copyHandlers(src []ExceptionHandler) []ExceptionHandler {
	if src == nil {
		return nil
	}
	dst := make([]ExceptionHandler, len(src))
	copy(dst, src)
	return dst
}
