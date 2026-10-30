// Package tmpl is used to parse Risor string templates.
package tmpl

import "fmt"

type Fragment struct {
	// value is the fragment text. If the fragment is an expression, this will
	// the expression text without the ${} delimiters.
	value string
	// isVariable is true if this is an expression, false if it is raw text.
	isVariable bool
	// offset is the index, in runes, of the start of value in the template.
	offset int
}

// Value returns the fragment text. If the fragment is an expression, this will
// the expression text without the ${} delimiters.
func (f *Fragment) Value() string {
	return f.value
}

// IsVariable returns true if this is an expression, false if it is raw text.
func (f *Fragment) IsVariable() bool {
	return f.isVariable
}

// Offset returns the index, in runes, where the fragment text starts in the
// template. For an expression, this is the index just after the "${".
func (f *Fragment) Offset() int {
	return f.offset
}

// Template defines a string template which may contain any number of
// expressions within.
type Template struct {
	// value is the original string that defines the template
	value string
	// fragments is a list of fragments that together form the entire template
	fragments []*Fragment
}

// Value returns the original string that defines the template.
func (t *Template) Value() string {
	return t.value
}

// Fragments returns the list of fragments that together form the entire
// template.
func (t *Template) Fragments() []*Fragment {
	return t.fragments
}

// Parse parses a string into a Template struct. The string may contain 0-N
// expressions in the form ${expression}. An expression ends at the "}" that
// matches its "${". Braces inside the expression, and braces inside its string
// literals, do not end it.
func Parse(s string) (*Template, error) {
	runes := []rune(s)
	template := &Template{value: s}

	// textStart is the start of the raw text that precedes the next expression
	textStart := 0
	addText := func(end int) {
		if end > textStart {
			template.fragments = append(template.fragments, &Fragment{
				value:  string(runes[textStart:end]),
				offset: textStart,
			})
		}
	}
	for i := 0; i < len(runes); i++ {
		if runes[i] != '$' || i+1 >= len(runes) || runes[i+1] != '{' {
			continue
		}
		addText(i)
		start := i + 2 // skip the "${"
		end, ok := expressionEnd(runes, start)
		if !ok {
			return nil, fmt.Errorf("missing '}' in template: %v", s)
		}
		template.fragments = append(template.fragments, &Fragment{
			value:      string(runes[start:end]),
			isVariable: true,
			offset:     start,
		})
		i = end // the closing '}'
		textStart = end + 1
	}
	addText(len(runes))
	return template, nil
}

// expressionEnd returns the index of the '}' that closes the expression
// starting at the given index. It skips nested braces, string literals and
// comments. A "//" comment ends at a newline or at the '}' that closes the
// expression, so `${ x // note }` is closed. A "/* */" comment ends only at
// "*/". It returns false if the expression is not closed.
func expressionEnd(runes []rune, start int) (int, bool) {
	depth := 0
	inLineComment := false
	for i := start; i < len(runes); i++ {
		r := runes[i]
		if inLineComment {
			switch {
			case r == '\n':
				inLineComment = false
			case r == '}' && depth == 0:
				return i, true
			}
			continue
		}
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch {
		case r == '/' && next == '/':
			inLineComment = true
			i++
			continue
		case r == '/' && next == '*':
			// Skip to the "*/" that ends the comment
			i += 2
			for i+1 < len(runes) && (runes[i] != '*' || runes[i+1] != '/') {
				i++
			}
			if i+1 >= len(runes) {
				return 0, false
			}
			i++ // the '/' of "*/"
			continue
		}
		switch r {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i, true
			}
			depth--
		case '"', '\'':
			// Skip to the closing quote. A backslash escapes the next rune.
			for i++; i < len(runes) && runes[i] != r; i++ {
				if runes[i] == '\\' {
					i++
				}
			}
			if i >= len(runes) {
				return 0, false
			}
		}
	}
	return 0, false
}
