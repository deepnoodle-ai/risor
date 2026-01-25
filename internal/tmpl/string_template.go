// Package tmpl is used to parse Risor string templates.
package tmpl

import "fmt"

type Fragment struct {
	// value is the fragment text. If the fragment is an expression, this will
	// the expression text without the ${} delimiters.
	value string
	// isVariable is true if this is an expression, false if it is raw text.
	isVariable bool
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
// expressions in the form ${expression}.
func Parse(s string) (*Template, error) {
	runes := []rune(s)

	getChar := func(index int) rune {
		if index < 0 || index >= len(runes) {
			return 0
		}
		return runes[index]
	}

	template := &Template{value: s}

	// Iterate through all runes in the string to find ${variable}s. We build up
	// a list of string "fragments", which are either raw text or variables.
	var curFragment *Fragment
	for i := 0; i < len(runes); i++ {
		char := getChar(i)
		peekChar := getChar(i + 1)
		if char == '$' && peekChar == '{' {
			// Start of an expression ${...}
			if curFragment != nil && curFragment.IsVariable() {
				return nil, fmt.Errorf("invalid '${' in template: %v", s)
			}
			curFragment = &Fragment{
				isVariable: true,
				value:      "",
			}
			template.fragments = append(template.fragments, curFragment)
			i++ // Skip the '{' as well
			continue
		} else if char == '}' && curFragment != nil && curFragment.IsVariable() {
			// End of an expression
			curFragment = nil
			continue
		}
		// Append current character to the current fragment
		if curFragment == nil {
			curFragment = &Fragment{
				isVariable: false,
				value:      "",
			}
			template.fragments = append(template.fragments, curFragment)
		}
		curFragment.value += string(char)
	}

	if curFragment != nil && curFragment.isVariable {
		return nil, fmt.Errorf("missing '}' in template: %v", s)
	}
	return template, nil
}
