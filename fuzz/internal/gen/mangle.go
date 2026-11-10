package gen

import (
	"strings"
	"unicode/utf8"
)

// oddTokens are tokens a mangled program may gain: follow-on forms, tokens no
// rule accepts, reserved words, bad numbers and bad escapes.
var oddTokens = []string{
	"===", "!==", "+=", "++", "|>", "|", "&", "$", "#", "@", "\\", "^", "~",
	`"""a"""`, `'''`, `r"x"`, `f'y'`, "String.raw`z`", ".5", "5.", "010", "0b1",
	"1__0", "0x", "9007199254740992", "1e400", `"\q"`, `"\u{110000}"`,
	`"\uD800"`, `"\uDC00x"`, "`${", "?.", "?.[", "[*]", "=>", "...", ":", ";",
	",", "?", "{", "}", "(", ")", "[", "]", "as", "is", "match {", "nil",
	"let", "return", "const", "else", "catch", "try", "if", "not", "in",
	"_", "x = 1", "`a\\\nb`", "/*", "*/", "//", "<T>", "finally", "1.e5",
	"\u00a0", "\ufeff", "é", "𝄞", "\u0000", "\u007f", "\u0085", "\u2028", "\r",
}

// oddChars are code points a mangled program may gain in place of another.
var oddChars = []string{
	"\ufeff", "\u00a0", "\u0000", "\u0001", "\u007f", "\u0085", "\u2028",
	"é", "𝄞", "\r", "\t", "\n", "\"", "'", "`", "\\", "{", "}", "$", "#",
	"(", ")", "[", "]", ".", "?", "!", "-", "*", "=", "<", "_", "0", "9", "e",
}

// tokens splits source text roughly into tokens and white space. It knows
// strings and templates well enough to keep them whole.
func tokens(src string) []string {
	var out []string
	i := 0
	multi := []string{"...", "?.[", "**", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "=>"}
	for i < len(src) {
		c := src[i]
		start := i
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			for i < len(src) && strings.IndexByte(" \t\n\r", src[i]) >= 0 {
				i++
			}
		case c == '"' || c == '\'' || c == '`':
			i++
			for i < len(src) && src[i] != c {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i = min(i+1, len(src))
		case isWordByte(c):
			for i < len(src) && (isWordByte(src[i]) || src[i] == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9') {
				i++
			}
		default:
			n := 0
			for _, m := range multi {
				if strings.HasPrefix(src[i:], m) {
					n = len(m)
					break
				}
			}
			if n == 0 {
				_, n = utf8.DecodeRuneInString(src[i:])
			}
			i += n
		}
		out = append(out, src[start:i])
	}
	return out
}

// mangle applies one to three changes to the text of a program.
func (g *Gen) mangle(src string) string {
	r := g.r
	k := 1 + r.IntN(3)
	for range k {
		src = g.mangleOnce(src)
	}
	return src
}

func (g *Gen) mangleOnce(src string) string {
	r := g.r
	ts := tokens(src)
	if len(ts) == 0 {
		return pick(r, oddTokens)
	}
	i := r.IntN(len(ts))
	switch r.IntN(12) {
	case 0: // delete a token
		ts = append(ts[:i:i], ts[i+1:]...)
	case 1: // double a token
		ts = append(ts[:i+1:i+1], ts[i:]...)
	case 2: // swap two tokens
		j := r.IntN(len(ts))
		ts[i], ts[j] = ts[j], ts[i]
	case 3, 4: // replace a token
		ts[i] = pick(r, oddTokens)
	case 5: // insert a token
		ts = append(ts[:i:i], append([]string{" " + pick(r, oddTokens) + " "}, ts[i:]...)...)
	case 6, 7: // change a character
		rs := []rune(src)
		j := r.IntN(len(rs))
		return string(rs[:j]) + pick(r, oddChars) + string(rs[j+1:])
	case 8: // cut the text short: unterminated literals and comments
		rs := []rune(src)
		return string(rs[:r.IntN(len(rs)+1)])
	case 9: // a byte order mark, at the start or elsewhere
		if r.IntN(2) == 0 {
			return "\ufeff" + src
		}
		ts[i] = "\ufeff" + ts[i]
	case 10: // deep nesting around the result or a token
		n := 97 + r.IntN(6)
		open, close := pick(r, []string{"(", "[", "-", "!", "`${", "{a: "}), ""
		switch open {
		case "(":
			close = ")"
		case "[":
			close = "]"
		case "`${":
			close = "}`"
		case "{a: ":
			close = "}"
		}
		return strings.Repeat(open, n) + "1" + strings.Repeat(close, n)
	default: // a comment somewhere
		ts = append(ts[:i:i], append([]string{pick(r, []string{"/* c */", "// c\n", "/* /* */", "/*", "//"})}, ts[i:]...)...)
	}
	return strings.Join(ts, "")
}
