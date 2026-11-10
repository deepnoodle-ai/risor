package eval

import (
	"encoding/base64"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is the eight entries of package F6, opt-in groups
// (spec/packages/f06-groups.md 6 to 9). No result comes from a platform
// parser where the chapter gives its own rule.

var groupImplemented = withMath(map[string]libFunc{
	"json.stringify": jsonStringify,
	"json.parse":     jsonParse,
	"time.parse":     timeParse,
	"time.format":    timeFormat,
	"time.duration":  timeDuration,
	"math.sqrt": func(r *run, c *libCall) any {
		x := c.args[0].(float64)
		if x < 0 {
			r.invalid(c, "not-real", messages.Word("name", c.e.Name))
		}
		return zero(math.Sqrt(x))
	},
	"base64.encode": base64Encode,
	"base64.decode": base64Decode,
})

// withMath adds the entries of package F13 (math.go).
func withMath(m map[string]libFunc) map[string]libFunc {
	for k, f := range mathEntries {
		m[k] = f
	}
	return m
}

// --- json ------------------------------------------------------------------

// jsonStringify is 6.1: the JSON form of x, by string's rendering walk.
func jsonStringify(r *run, c *libCall) any {
	f := fenv{c: c}
	before := r.steps
	var b renderer
	b.r, b.span, b.opaque = r, c.span, "json"
	b.json(c.args[0])
	s := b.String()
	f.walkSteps, f.resultW = r.steps-before, value.Weight(s)
	r.libSteps(c, f, f.walkSteps)
	r.libUnits(c, f)
	return s
}

// jsonPass is the pass of 6.2.4 over a JSON text: it creates no value.
type jsonPass struct {
	s       string
	i       int
	notJSON bool
	badNum  string // "integer-range" or "number-range": the first bad number
	depth   int    // d of 6.2.5
	c, u    int64
}

func isJSONWS(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

func (p *jsonPass) ws() {
	for p.i < len(p.s) && isJSONWS(p.s[p.i]) {
		p.i++
	}
}

// frame is one open array or object of the pass.
type frame struct {
	obj   bool
	n     int64           // values of an array
	keys  map[string]bool // distinct keys of an object
	state byte            // what comes next: see value
}

// run reads the whole text with an explicit stack. It returns false when
// the text is not JSON.
func (p *jsonPass) run() bool {
	var stack []*frame
	p.ws()
	expectValue := true
	for {
		if expectValue {
			if p.i >= len(p.s) {
				return false
			}
			p.c++
			switch ch := p.s[p.i]; {
			case ch == '{' || ch == '[':
				p.i++
				fr := &frame{obj: ch == '{'}
				if fr.obj {
					fr.keys = map[string]bool{}
				}
				stack = append(stack, fr)
				if len(stack) > p.depth {
					p.depth = len(stack)
				}
				p.ws()
				if p.i < len(p.s) && ((fr.obj && p.s[p.i] == '}') || (!fr.obj && p.s[p.i] == ']')) {
					p.i++
					p.close(fr)
					stack = stack[:len(stack)-1]
					expectValue = false
					break
				}
				if fr.obj {
					if !p.key(fr) {
						return false
					}
				}
				continue
			case ch == '"':
				s, ok := p.str()
				if !ok {
					return false
				}
				p.u += 1 + value.Size(s)
			case ch == '-' || (ch >= '0' && ch <= '9'):
				if !p.number() {
					return false
				}
			case strings.HasPrefix(p.s[p.i:], "true"):
				p.i += 4
			case strings.HasPrefix(p.s[p.i:], "false"):
				p.i += 5
			case strings.HasPrefix(p.s[p.i:], "null"):
				p.i += 4
			default:
				return false
			}
			expectValue = false
		}
		// After a value.
		p.ws()
		if len(stack) == 0 {
			return p.i == len(p.s)
		}
		fr := stack[len(stack)-1]
		if !fr.obj {
			fr.n++
		}
		if p.i >= len(p.s) {
			return false
		}
		switch p.s[p.i] {
		case ',':
			p.i++
			p.ws()
			if fr.obj {
				if !p.key(fr) {
					return false
				}
			}
			expectValue = true
		case '}', ']':
			if (p.s[p.i] == '}') != fr.obj {
				return false
			}
			p.i++
			p.close(fr)
			stack = stack[:len(stack)-1]
		default:
			return false
		}
	}
}

// key reads a member's key and its colon, leaving the place at its value.
func (p *jsonPass) key(fr *frame) bool {
	if p.i >= len(p.s) || p.s[p.i] != '"' {
		return false
	}
	k, ok := p.str()
	if !ok {
		return false
	}
	fr.keys[k] = true
	p.ws()
	if p.i >= len(p.s) || p.s[p.i] != ':' {
		return false
	}
	p.i++
	p.ws()
	return true
}

func (p *jsonPass) close(fr *frame) {
	if fr.obj {
		p.u += 1 + 2*int64(len(fr.keys))
	} else {
		p.u += 1 + fr.n
	}
}

func hexVal(s string) (rune, bool) {
	if len(s) < 4 {
		return 0, false
	}
	var v rune
	for i := 0; i < 4; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			v = v*16 + rune(c-'0')
		case c >= 'a' && c <= 'f':
			v = v*16 + rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			v = v*16 + rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return v, true
}

// str reads a String at the place (6.2.1, 6.2.3) and gives its value.
func (p *jsonPass) str() (string, bool) {
	p.i++ // "
	var b strings.Builder
	for {
		if p.i >= len(p.s) {
			return "", false
		}
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			return b.String(), true
		case c < 0x20:
			return "", false
		case c == '\\':
			if p.i+1 >= len(p.s) {
				return "", false
			}
			e := p.s[p.i+1]
			p.i += 2
			switch e {
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				v, ok := hexVal(p.s[p.i:])
				if !ok {
					return "", false
				}
				p.i += 4
				switch {
				case v >= 0xD800 && v <= 0xDBFF:
					if strings.HasPrefix(p.s[p.i:], `\u`) {
						if lo, ok := hexVal(p.s[p.i+2:]); ok && lo >= 0xDC00 && lo <= 0xDFFF {
							p.i += 6
							b.WriteRune(0x10000 + (v-0xD800)<<10 + (lo - 0xDC00))
							continue
						}
					}
					b.WriteRune(0xFFFD)
				case v >= 0xDC00 && v <= 0xDFFF:
					b.WriteRune(0xFFFD)
				default:
					b.WriteRune(v)
				}
			default:
				return "", false
			}
		default:
			b.WriteByte(c)
			p.i++
		}
	}
}

// number reads a Number (6.2.1) and tests it by 6.2.3, noting the first
// number that breaks items 2 or 3.
func (p *jsonPass) number() bool {
	start := p.i
	if p.s[p.i] == '-' {
		p.i++
	}
	digits := func() int {
		n := 0
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
			n++
		}
		return n
	}
	if p.i >= len(p.s) {
		return false
	}
	intStart := p.i
	if p.s[p.i] == '0' {
		p.i++
	} else if digits() == 0 {
		return false
	}
	intText := p.s[intStart:p.i]
	integer := true
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		if digits() == 0 {
			return false
		}
		integer = false
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		if digits() == 0 {
			return false
		}
		integer = false
	}
	if p.badNum != "" {
		return true
	}
	if integer {
		if !wholeDigits(intText) {
			p.badNum = "integer-range"
		}
		return true
	}
	if f, _ := strconv.ParseFloat(p.s[start:p.i], 64); math.IsInf(f, 0) {
		p.badNum = "number-range"
	}
	return true
}

// wholeDigits reports whether the integer the digits name is at most
// 9007199254740991, compared on the digits (6.2.3 item 2).
func wholeDigits(d string) bool {
	const max = "9007199254740991"
	if len(d) != len(max) {
		return len(d) < len(max)
	}
	return d <= max
}

// jsonBuild makes the value of a JSON text that passed (6.2.2). The text
// is known to be JSON, so it is read again without tests, with an explicit
// stack.
func jsonBuild(s string) any {
	p := &jsonPass{s: s}
	type open struct {
		list []any
		m    map[string]any
		key  string
		obj  bool
	}
	var stack []*open
	var result any
	put := func(v any) bool { // gives true when the value is the whole text
		if len(stack) == 0 {
			result = v
			return true
		}
		top := stack[len(stack)-1]
		if top.obj {
			top.m[top.key] = v
		} else {
			top.list = append(top.list, v)
		}
		return false
	}
	for {
		p.ws()
		var v any
		ch := s[p.i]
		switch {
		case ch == '{' || ch == '[':
			p.i++
			o := &open{obj: ch == '{'}
			if o.obj {
				o.m = map[string]any{}
			} else {
				o.list = []any{}
			}
			stack = append(stack, o)
			p.ws()
			if s[p.i] == '}' || s[p.i] == ']' {
				p.i++
				stack = stack[:len(stack)-1]
				if o.obj {
					v = o.m
				} else {
					v = o.list
				}
				break
			}
			if o.obj {
				o.key, _ = p.str()
				p.ws()
				p.i++ // :
			}
			continue
		case ch == '"':
			v, _ = p.str()
		case ch == 't':
			p.i += 4
			v = true
		case ch == 'f':
			p.i += 5
			v = false
		case ch == 'n':
			p.i += 4
			v = nil
		default:
			start := p.i
			p.number()
			f, _ := strconv.ParseFloat(s[start:p.i], 64)
			v = zero(f)
		}
		for {
			if put(v) {
				return result
			}
			p.ws()
			top := stack[len(stack)-1]
			if s[p.i] == ',' {
				p.i++
				if top.obj {
					p.ws()
					top.key, _ = p.str()
					p.ws()
					p.i++ // :
				}
				break
			}
			p.i++ // } or ]
			stack = stack[:len(stack)-1]
			if top.obj {
				v = top.m
			} else {
				v = top.list
			}
		}
	}
}

// jsonParse is 6.2.
func jsonParse(r *run, c *libCall) any {
	s := c.args[0].(string)
	r.step(8 * value.Size(s))
	p := &jsonPass{s: s}
	switch {
	case !p.run():
		r.invalid(c, "not-json")
	case p.badNum == "integer-range":
		r.invalid(c, "integer-range")
	case p.badNum == "number-range":
		r.entryRange(c)
	case r.depth+r.walk+p.depth > r.maxDepth:
		r.depthFail()
	}
	r.step(p.c)
	r.unit(p.u)
	return jsonBuild(s)
}

// --- time ------------------------------------------------------------------

// daysFrom is days(y, m, d) of 7.1: days from 1970-01-01 in the proleptic
// Gregorian calendar.
func daysFrom(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// civil is the inverse of daysFrom.
func civil(z int64) (y, m, d int64) {
	z += 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y = yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = doy - (153*mp+2)/5 + 1
	m = mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func leap(y int64) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func monthLen(y, m int64) int64 {
	switch m {
	case 2:
		if leap(y) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// fixedDigits reads n ASCII digits at s[i:].
func fixedDigits(s string, i, n int) (int64, bool) {
	if i+n > len(s) {
		return 0, false
	}
	var v int64
	for k := 0; k < n; k++ {
		c := s[i+k]
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int64(c-'0')
	}
	return v, true
}

// parseTime reads the accepted form of 7.2. It gives W and the fraction's
// digits.
func parseTime(s string) (w int64, frac string, ok bool) {
	at := func(i int, c byte) bool { return i < len(s) && s[i] == c }
	y, ok1 := fixedDigits(s, 0, 4)
	mo, ok2 := fixedDigits(s, 5, 2)
	d, ok3 := fixedDigits(s, 8, 2)
	h, ok4 := fixedDigits(s, 11, 2)
	mi, ok5 := fixedDigits(s, 14, 2)
	se, ok6 := fixedDigits(s, 17, 2)
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && at(4, '-') && at(7, '-') && at(10, 'T') && at(13, ':') && at(16, ':')) {
		return 0, "", false
	}
	if mo < 1 || mo > 12 || d < 1 || d > monthLen(y, mo) || h > 23 || mi > 59 || se > 59 {
		return 0, "", false
	}
	i := 19
	if at(i, '.') {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i+1 {
			return 0, "", false
		}
		frac = s[i+1 : j]
		i = j
	}
	var off int64
	switch {
	case at(i, 'Z') && i+1 == len(s):
	case (at(i, '+') || at(i, '-')) && i+6 == len(s) && at(i+3, ':'):
		oh, ok1 := fixedDigits(s, i+1, 2)
		om, ok2 := fixedDigits(s, i+4, 2)
		if !ok1 || !ok2 || oh > 23 || om > 59 {
			return 0, "", false
		}
		off = oh*3600 + om*60
		if s[i] == '-' {
			off = -off
		}
	default:
		return 0, "", false
	}
	w = 86400*daysFrom(y, mo, d) + 3600*h + 60*mi + se - off
	return w, frac, true
}

// nearest is the double nearest to the exact W + 0.frac, ties to even
// (7.2 item 4), read from a decimal text by the rule of text to number.
func nearest(w int64, frac string) float64 {
	if strings.Trim(frac, "0") == "" {
		return zero(float64(w))
	}
	if w >= 0 {
		f, _ := strconv.ParseFloat(strconv.FormatInt(w, 10)+"."+frac, 64)
		return f
	}
	// W + F = -((|W| - 1) + (1 - F)). 1 - F has as many digits as F: the
	// digits of 10^n - D, which is above zero since F is.
	g := []byte(frac)
	borrow := 0
	for i := len(g) - 1; i >= 0; i-- {
		d := -int(g[i]-'0') - borrow
		borrow = 0
		if d < 0 {
			d += 10
			borrow = 1
		}
		g[i] = byte('0' + d)
	}
	f, _ := strconv.ParseFloat("-"+strconv.FormatInt(-w-1, 10)+"."+string(g), 64)
	return zero(f)
}

func timeParse(r *run, c *libCall) any {
	s := c.args[0].(string)
	r.step(value.Size(s))
	w, frac, ok := parseTime(s)
	if !ok {
		r.invalid(c, "not-a-time", messages.Val("text", s))
	}
	return nearest(w, frac)
}

const (
	minTime = -62167219200
	maxTime = 253402300800
)

// formatTime is the text of 7.3 for t.
func formatTime(t float64) string {
	w := math.Floor(t)
	wi := int64(w)
	days := floorDiv(wi, 86400)
	secs := wi - days*86400
	y, m, d := civil(days)
	var b strings.Builder
	pad := func(v int64, n int) {
		s := strconv.FormatInt(v, 10)
		for len(s) < n {
			s = "0" + s
		}
		b.WriteString(s)
	}
	pad(y, 4)
	b.WriteByte('-')
	pad(m, 2)
	b.WriteByte('-')
	pad(d, 2)
	b.WriteByte('T')
	pad(secs/3600, 2)
	b.WriteByte(':')
	pad(secs/60%60, 2)
	b.WriteByte(':')
	pad(secs%60, 2)
	if w != t {
		b.WriteByte('.')
		b.WriteString(fractionDigits(t, wi))
	}
	b.WriteByte('Z')
	return b.String()
}

// fractionDigits writes V − W with as many digits after the point as V
// has, where V is the exact value of the shortest text of t (7.3 item 3).
func fractionDigits(t float64, w int64) string {
	text := strconv.FormatFloat(t, 'e', -1, 64) // the shortest digits
	mant, exp, _ := strings.Cut(text, "e")
	neg := strings.HasPrefix(mant, "-")
	mant = strings.TrimPrefix(mant, "-")
	ip, fp, _ := strings.Cut(mant, ".")
	digits := ip + fp
	e, _ := strconv.Atoi(exp)
	// V = digits × 10^(e - len(fp)); k digits after the point.
	k := len(fp) - e
	if k <= 0 {
		return ""
	}
	m := new(big.Int)
	m.SetString(digits, 10)
	if neg {
		m.Neg(m)
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(k)), nil)
	wv := new(big.Int).Mul(big.NewInt(w), scale)
	m.Sub(m, wv)
	s := m.String()
	for len(s) < k {
		s = "0" + s
	}
	return s
}

func timeFormat(r *run, c *libCall) any {
	t := c.args[0].(float64)
	if w := math.Floor(t); w < minTime || w >= maxTime {
		r.invalid(c, "year-range", messages.Val("t", t))
	}
	s := formatTime(t)
	f := fenv{c: c, resultW: value.Weight(s)}
	r.charge(c, f)
	return s
}

// duration reads a duration text (7.4) and gives the decimal text of its
// exact value, summed in columns and carried once, so in time linear in s.
func duration(s string) (string, bool) {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	type term struct {
		ip, fp string
		shift  int   // 3 for ms
		mul    int64 // 1, 60, 3600, 86400
	}
	var terms []term
	maxFrac, maxInt := 0, 0
	for i := 0; i < len(s); {
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i {
			return "", false
		}
		t := term{ip: s[i:j]}
		i = j
		if i < len(s) && s[i] == '.' {
			j = i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j == i+1 {
				return "", false
			}
			t.fp = s[i+1 : j]
			i = j
		}
		switch {
		case strings.HasPrefix(s[i:], "ms"):
			t.mul, t.shift = 1, 3
			i += 2
		case i < len(s) && s[i] == 's':
			t.mul, i = 1, i+1
		case i < len(s) && s[i] == 'm':
			t.mul, i = 60, i+1
		case i < len(s) && s[i] == 'h':
			t.mul, i = 3600, i+1
		case i < len(s) && s[i] == 'd':
			t.mul, i = 86400, i+1
		default:
			return "", false
		}
		terms = append(terms, t)
		maxFrac = max(maxFrac, len(t.fp)+t.shift)
		maxInt = max(maxInt, len(t.ip))
	}
	if len(terms) == 0 {
		return "", false
	}
	// Column c holds 10^(c - maxFrac). Each digit times the unit is added in
	// its column; the carry is made once at the end.
	cols := make([]int64, maxFrac+maxInt+8)
	for _, t := range terms {
		ds := t.ip + t.fp
		low := maxFrac - len(t.fp) - t.shift // column of the last digit
		for k := 0; k < len(ds); k++ {
			col := low + len(ds) - 1 - k
			cols[col] += int64(ds[k]-'0') * t.mul
		}
	}
	var carry int64
	for i := range cols {
		v := cols[i] + carry
		cols[i] = v % 10
		carry = v / 10
	}
	for carry > 0 {
		cols = append(cols, carry%10)
		carry /= 10
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	top := len(cols) - 1
	for top > maxFrac && cols[top] == 0 {
		top--
	}
	for i := top; i >= maxFrac; i-- {
		b.WriteByte(byte('0' + cols[i]))
	}
	if maxFrac > 0 {
		b.WriteByte('.')
		for i := maxFrac - 1; i >= 0; i-- {
			b.WriteByte(byte('0' + cols[i]))
		}
	}
	return b.String(), true
}

func timeDuration(r *run, c *libCall) any {
	s := c.args[0].(string)
	r.step(value.Size(s))
	text, ok := duration(s)
	if !ok {
		r.invalid(c, "not-a-duration", messages.Val("text", s))
	}
	f, _ := strconv.ParseFloat(text, 64)
	if math.IsInf(f, 0) {
		r.entryRange(c)
	}
	return zero(f)
}

// --- base64 ----------------------------------------------------------------

func base64Encode(r *run, c *libCall) any {
	s := c.args[0].(string)
	out := base64.StdEncoding.EncodeToString([]byte(s)) // RFC 4648 4, with padding
	r.charge(c, fenv{c: c, resultW: int64(len(out))})
	return out
}

func b64Val(ch byte) (v int, std, url bool) {
	switch {
	case ch >= 'A' && ch <= 'Z':
		return int(ch - 'A'), false, false
	case ch >= 'a' && ch <= 'z':
		return int(ch-'a') + 26, false, false
	case ch >= '0' && ch <= '9':
		return int(ch-'0') + 52, false, false
	case ch == '+':
		return 62, true, false
	case ch == '/':
		return 63, true, false
	case ch == '-':
		return 62, false, true
	case ch == '_':
		return 63, false, true
	}
	return -1, false, false
}

// decode64 is 9.2 items 1 to 4: the bytes, or false.
func decode64(s string) ([]byte, bool) {
	t := strings.TrimRight(s, "=")
	p := len(s) - len(t)
	n := len(t)
	switch n % 4 {
	case 1:
		return nil, false
	case 0:
		if p != 0 {
			return nil, false
		}
	default:
		if p != 0 && p != 4-n%4 {
			return nil, false
		}
	}
	var std, url bool
	vals := make([]int, n)
	for i := 0; i < n; i++ {
		v, st, u := b64Val(t[i])
		if v < 0 {
			return nil, false
		}
		std, url = std || st, url || u
		vals[i] = v
	}
	if std && url {
		return nil, false
	}
	out := make([]byte, 0, n*3/4)
	i := 0
	for ; i+4 <= n; i += 4 {
		x := vals[i]<<18 | vals[i+1]<<12 | vals[i+2]<<6 | vals[i+3]
		out = append(out, byte(x>>16), byte(x>>8), byte(x))
	}
	switch n - i {
	case 2:
		if vals[i+1]&0xF != 0 {
			return nil, false
		}
		out = append(out, byte(vals[i]<<2|vals[i+1]>>4))
	case 3:
		if vals[i+2]&0x3 != 0 {
			return nil, false
		}
		x := vals[i]<<10 | vals[i+1]<<4 | vals[i+2]>>2
		out = append(out, byte(x>>8), byte(x))
	}
	return out, true
}

func base64Decode(r *run, c *libCall) any {
	s := c.args[0].(string)
	r.step(value.Size(s))
	b, ok := decode64(s)
	if !ok {
		r.invalid(c, "not-base64")
	}
	if !utf8.Valid(b) { // well-formed UTF-8: no overlong form, no surrogate
		r.invalid(c, "not-utf8")
	}
	out := string(b)
	r.unit(1 + value.Size(out))
	return out
}
