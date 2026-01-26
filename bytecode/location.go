package bytecode

import "fmt"

// SourceLocation represents a position in source code.
// This is a minimal representation that only stores line and column.
// Filename and source text are stored once on the Code object.
type SourceLocation struct {
	Line   int // 1-based line number
	Column int // 1-based column number
}

// String returns a formatted string representation of the source location.
func (s SourceLocation) String() string {
	return fmt.Sprintf("%d:%d", s.Line, s.Column)
}

// IsZero returns true if the location has not been set.
func (s SourceLocation) IsZero() bool {
	return s.Line == 0 && s.Column == 0
}
