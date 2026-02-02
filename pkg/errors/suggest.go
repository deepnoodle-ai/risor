package errors

import (
	"sort"
	"strings"
)

// MaxSuggestionDistance is the maximum edit distance for a suggestion to be considered.
const MaxSuggestionDistance = 3

// MaxSuggestions is the maximum number of suggestions to return.
const MaxSuggestions = 3

// Suggestion represents a suggested correction with its edit distance.
type Suggestion struct {
	Value    string
	Distance int
}

// SuggestSimilar finds similar strings from candidates for the given target.
// Returns up to MaxSuggestions suggestions within MaxSuggestionDistance.
func SuggestSimilar(target string, candidates []string) []Suggestion {
	if len(target) == 0 || len(candidates) == 0 {
		return nil
	}

	target = strings.ToLower(target)
	var suggestions []Suggestion

	for _, candidate := range candidates {
		// Skip empty candidates and exact matches
		if candidate == "" || strings.ToLower(candidate) == target {
			continue
		}

		dist := levenshteinDistance(target, strings.ToLower(candidate))

		// Only consider suggestions within the threshold
		// Scale threshold based on word length for better UX
		threshold := MaxSuggestionDistance
		if len(target) <= 3 {
			threshold = 1
		} else if len(target) <= 5 {
			threshold = 2
		}

		if dist <= threshold {
			suggestions = append(suggestions, Suggestion{
				Value:    candidate,
				Distance: dist,
			})
		}
	}

	// Sort by distance (closest first), then alphabetically
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Distance != suggestions[j].Distance {
			return suggestions[i].Distance < suggestions[j].Distance
		}
		return suggestions[i].Value < suggestions[j].Value
	})

	// Limit to MaxSuggestions
	if len(suggestions) > MaxSuggestions {
		suggestions = suggestions[:MaxSuggestions]
	}

	return suggestions
}

// FormatSuggestions formats suggestions as a user-friendly string.
// Returns empty string if no suggestions.
func FormatSuggestions(suggestions []Suggestion) string {
	if len(suggestions) == 0 {
		return ""
	}

	if len(suggestions) == 1 {
		return "Did you mean '" + suggestions[0].Value + "'?"
	}

	var b strings.Builder
	b.WriteString("Did you mean one of: ")
	for i, s := range suggestions {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("'")
		b.WriteString(s.Value)
		b.WriteString("'")
	}
	b.WriteString("?")
	return b.String()
}

// levenshteinDistance computes the edit distance between two strings.
// This is an optimized implementation using two rows instead of a full matrix.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Convert to runes for proper Unicode handling
	aRunes := []rune(a)
	bRunes := []rune(b)

	// Ensure a is the shorter string for space optimization
	if len(aRunes) > len(bRunes) {
		aRunes, bRunes = bRunes, aRunes
	}

	lenA := len(aRunes)
	lenB := len(bRunes)

	// Use two rows instead of full matrix
	prev := make([]int, lenA+1)
	curr := make([]int, lenA+1)

	// Initialize first row
	for i := 0; i <= lenA; i++ {
		prev[i] = i
	}

	// Fill in the rest
	for j := 1; j <= lenB; j++ {
		curr[0] = j
		for i := 1; i <= lenA; i++ {
			cost := 1
			if aRunes[i-1] == bRunes[j-1] {
				cost = 0
			}
			curr[i] = min(
				prev[i]+1,      // deletion
				curr[i-1]+1,    // insertion
				prev[i-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[lenA]
}
