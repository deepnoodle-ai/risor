// Package textsize asks one question: in what unit should the cost model
// measure text, so that a Go and a JavaScript implementation get the same
// number and neither pays much to know it?
//
// A Go string knows its UTF-8 byte length. A JavaScript string knows its
// UTF-16 length. Neither knows its code point count. This package measures
// what each unit costs Go to compute for a string it only reads, and what a
// charge in blocks of code points adds to equality and concatenation.
//
//	GOWORK=off go test ./lab/textsize -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package textsize

import (
	"encoding/binary"
	"math/bits"
	"unicode/utf8"
	"unsafe"
)

// Block is the amount of text that one unit of text size covers.
const Block = 64

// Bytes is the UTF-8 length: free in Go, a scan in JavaScript.
func Bytes(s string) int { return len(s) }

// CodePoints is a scan in both languages.
func CodePoints(s string) int { return utf8.RuneCountInString(s) }

// UTF16 is the UTF-16 length: a scan in Go, free in JavaScript. A code
// point starts at every byte that is not a continuation byte, and one that
// starts with a byte of 0xF0 or more takes two units.
func UTF16(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c&0xC0 != 0x80 {
			n++
		}
		if c >= 0xF0 {
			n++
		}
	}
	return n
}

// count returns the number of continuation bytes in s and the number of
// bytes that start a four-byte sequence. It reads eight bytes at a time and
// skips a word that is all ASCII with one test.
func count(s string) (cont, four int) {
	const high = 0x8080808080808080
	b := unsafe.Slice(unsafe.StringData(s), len(s))
	i := 0
	for ; i+8 <= len(b); i += 8 {
		w := binary.LittleEndian.Uint64(b[i:])
		hi := w & high
		if hi == 0 {
			continue
		}
		cont += bits.OnesCount64(hi &^ (w << 1))
		four += bits.OnesCount64(hi & (w << 1) & (w << 2) & (w << 3))
	}
	for ; i < len(b); i++ {
		c := b[i]
		if c&0xC0 == 0x80 {
			cont++
		}
		if c >= 0xF0 {
			four++
		}
	}
	return cont, four
}

// WeightSafe is Weight without package unsafe: it builds each word from
// eight indexed bytes, which the compiler turns into one load.
func WeightSafe(s string) int {
	const high = 0x8080808080808080
	cont, four := 0, 0
	i := 0
	for ; i+8 <= len(s); i += 8 {
		w := uint64(s[i]) | uint64(s[i+1])<<8 | uint64(s[i+2])<<16 | uint64(s[i+3])<<24 |
			uint64(s[i+4])<<32 | uint64(s[i+5])<<40 | uint64(s[i+6])<<48 | uint64(s[i+7])<<56
		hi := w & high
		if hi == 0 {
			continue
		}
		cont += bits.OnesCount64(hi &^ (w << 1))
		four += bits.OnesCount64(hi & (w << 1) & (w << 2) & (w << 3))
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c&0xC0 == 0x80 {
			cont++
		}
		if c >= 0xF0 {
			four++
		}
	}
	return len(s) - cont + four
}

// CodePointsFast and Weight are the same counts as CodePoints and UTF16,
// eight bytes at a time.
func CodePointsFast(s string) int {
	cont, _ := count(s)
	return len(s) - cont
}

// Weight is the proposed measure of text: the number of code points, with a
// code point above U+FFFF counted twice. It equals the UTF-16 length, so a
// JavaScript string knows it already. It is never more than the UTF-8
// length, so a Go string shorter than a block is known to be zero blocks
// without a scan.
func Weight(s string) int {
	cont, four := count(s)
	return len(s) - cont + four
}

// Blocks is the proposed text size: whole blocks of 64 units of weight.
func Blocks(s string) int64 {
	if len(s) < Block {
		return 0
	}
	return int64(Weight(s) / Block)
}

// BlocksOf2 is the size of a + b, which is not Blocks(a) + Blocks(b).
func BlocksOf2(a, b string) int64 {
	if len(a)+len(b) < Block {
		return 0
	}
	return int64((Weight(a) + Weight(b)) / Block)
}

type runtime struct {
	steps int64 // steps left
	units int64 // allocation units left
}

// Equal, three ways.

// EqualPlain charges one step whatever the length.
func EqualPlain(rt *runtime, a, b string) bool {
	rt.steps--
	return a == b
}

// EqualByLength charges by the shorter operand, which needs the weight of
// both when both are long.
func EqualByLength(rt *runtime, a, b string) bool {
	rt.steps--
	if len(a) >= Block && len(b) >= Block {
		rt.steps -= int64(min(Weight(a), Weight(b)) / Block)
	}
	return a == b
}

// EqualByLengthStdlib is EqualByLength in code points, counted with the
// standard library.
func EqualByLengthStdlib(rt *runtime, a, b string) bool {
	rt.steps--
	if len(a) >= Block && len(b) >= Block {
		rt.steps -= int64(min(utf8.RuneCountInString(a), utf8.RuneCountInString(b)) / Block)
	}
	return a == b
}

// EqualByPrefix charges by the text that a comparison has to look at:
// the common prefix. Two long strings that differ early cost one step and
// are not scanned.
func EqualByPrefix(rt *runtime, a, b string) bool {
	rt.steps--
	if len(a) < Block || len(b) < Block {
		return a == b
	}
	p := commonPrefix(a, b)
	if p >= Block {
		rt.steps -= int64(Weight(a[:p]) / Block)
	}
	return p == len(a) && p == len(b)
}

// commonPrefix returns the length in bytes of the longest common prefix that
// ends on a code point boundary of a.
func commonPrefix(a, b string) int {
	n := min(len(a), len(b))
	i := 0
	for ; i+8 <= n; i += 8 {
		if binary.LittleEndian.Uint64([]byte(a[i:i+8])) != binary.LittleEndian.Uint64([]byte(b[i:i+8])) {
			break
		}
	}
	for i < n && a[i] == b[i] {
		i++
	}
	// Step back out of a code point that only partly matched.
	for i > 0 && i < len(a) && a[i]&0xC0 == 0x80 {
		i--
	}
	return i
}

// Concat, three ways.

// ConcatPlain charges one step and nothing for the memory.
func ConcatPlain(rt *runtime, a, b string) string {
	rt.steps--
	return a + b
}

// ConcatCharged charges steps and allocation units for the result, before
// the result exists.
func ConcatCharged(rt *runtime, a, b string) (string, bool) {
	n := BlocksOf2(a, b)
	rt.steps -= 1 + n
	rt.units -= 1 + n
	if rt.steps < 0 || rt.units < 0 {
		return "", false
	}
	return a + b, true
}

// ConcatChargedStdlib charges by code points counted with the standard
// library.
func ConcatChargedStdlib(rt *runtime, a, b string) (string, bool) {
	var n int64
	if len(a)+len(b) >= Block {
		n = int64((utf8.RuneCountInString(a) + utf8.RuneCountInString(b)) / Block)
	}
	rt.steps -= 1 + n
	rt.units -= 1 + n
	if rt.steps < 0 || rt.units < 0 {
		return "", false
	}
	return a + b, true
}

// ConcatChargedBytes is the same in UTF-8 bytes, which Go knows for free.
func ConcatChargedBytes(rt *runtime, a, b string) (string, bool) {
	n := int64((len(a) + len(b)) / Block)
	rt.steps -= 1 + n
	rt.units -= 1 + n
	if rt.steps < 0 || rt.units < 0 {
		return "", false
	}
	return a + b, true
}
