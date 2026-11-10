// Package casemap is the case table of spec/core/04-library.md 3.8, built
// from suite/casemap.json (Unicode 16.0.0), which internal/gen copies into
// casemap_gen.go. It never uses the platform's own case tables.
package casemap

import (
	"encoding/json"
	"strconv"
	"strings"
)

var upper, lower map[rune]rune

func init() {
	var f struct {
		Upper map[string]string `json:"upper"`
		Lower map[string]string `json:"lower"`
	}
	if err := json.Unmarshal([]byte(sourceJSON), &f); err != nil {
		panic("casemap: " + err.Error())
	}
	upper, lower = table(f.Upper), table(f.Lower)
}

func table(m map[string]string) map[rune]rune {
	out := make(map[rune]rune, len(m))
	for k, v := range m {
		a, err1 := strconv.ParseUint(k, 16, 32)
		b, err2 := strconv.ParseUint(v, 16, 32)
		if err1 != nil || err2 != nil {
			panic("casemap: a code point is not hex: " + k)
		}
		out[rune(a)] = rune(b)
	}
	return out
}

// Sizes returns the number of keys of the two tables.
func Sizes() (int, int) { return len(upper), len(lower) }

// Upper maps each code point of s by upper(c).
func Upper(s string) string { return mapString(s, upper) }

// Lower maps each code point of s by lower(c).
func Lower(s string) string { return mapString(s, lower) }

func mapString(s string, t map[rune]rune) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		if m, ok := t[c]; ok {
			c = m
		}
		b.WriteRune(c)
	}
	return b.String()
}
