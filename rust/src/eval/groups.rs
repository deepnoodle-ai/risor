//! Package F6: the entries of the opt-in groups (`spec/packages/f06-groups.md`
//! 6 to 9). Each is written here by the chapter's rules: JSON is read by a
//! scanner of the grammar of 6.2.1 with an explicit stack, times use the
//! proleptic Gregorian calendar of 7.1, and base64 is RFC 4648 with the
//! strict rules of 9.2. No platform parser decides a result.

use super::{fail, Run, R};
use crate::messages::json_string;
use crate::number;
use crate::source::Range;
use crate::value::{release, size, MapVal, Str, Value};
use std::collections::HashMap;
use std::rc::Rc;

const MAX_SAFE: &str = "9007199254740991";

impl<'h> Run<'h> {
    /// An entry of a group, after the tests and the call's step.
    pub(super) fn group_entry(
        &mut self,
        name: &str,
        x: Value<'h>,
        span: Range,
    ) -> R<'h, Value<'h>> {
        match name {
            "json.stringify" => {
                let mut out = super::Rendering::new(String::new());
                self.json_mode = true;
                let r = self.render(&x, &mut out, false, span);
                self.json_mode = false;
                r?;
                let sz = out.weight() / 64;
                self.charge(sz)?;
                self.alloc(1 + sz)?;
                let s = self.rendered(out, |r, o| r.render(&x, o, false, span))?;
                Ok(Value::str(s))
            }
            "json.parse" => {
                let s = x.as_str().unwrap_or_default();
                self.charge(8 * size(s))?;
                self.json_parse(s, span)
            }
            "time.parse" => {
                let s = x.as_str().unwrap_or_default();
                self.charge(size(s))?;
                match parse_time(s) {
                    Some(t) => Ok(Value::Num(t)),
                    None => Err(fail(
                        "invalid-argument",
                        "not-a-time",
                        span,
                        vec![("text", json_string(s))],
                    )),
                }
            }
            "time.format" => {
                let t = match x {
                    Value::Num(n) => n,
                    _ => 0.0,
                };
                let Some(text) = format_time(t) else {
                    return Err(fail(
                        "invalid-argument",
                        "year-range",
                        span,
                        vec![("t", number::to_text(t))],
                    ));
                };
                let sz = size(&text);
                self.charge(sz)?;
                self.alloc(1 + sz)?;
                Ok(Value::str(text))
            }
            "time.duration" => {
                let s = x.as_str().unwrap_or_default();
                self.charge(size(s))?;
                match parse_duration(s) {
                    None => Err(fail(
                        "invalid-argument",
                        "not-a-duration",
                        span,
                        vec![("text", json_string(s))],
                    )),
                    Some(None) => Err(self.entry_range(name, span)),
                    Some(Some(v)) => Ok(Value::Num(v)),
                }
            }
            "math.sqrt" => {
                let n = match x {
                    Value::Num(n) => n,
                    _ => 0.0,
                };
                if n < 0.0 {
                    return Err(fail(
                        "invalid-argument",
                        "not-real",
                        span,
                        vec![("name", name.to_string())],
                    ));
                }
                let r = n.sqrt();
                Ok(Value::Num(if r == 0.0 { 0.0 } else { r }))
            }
            "base64.encode" => {
                let s = x.as_str().unwrap_or_default();
                // 9.1 item 2: the result has 4 * ceil(b / 3) code points,
                // found before anything is charged or built.
                let sz = (s.len().div_ceil(3) * 4) as u64 / 64;
                self.charge(size(s) + sz)?;
                self.alloc(1 + sz)?;
                Ok(Value::str(b64_encode(s.as_bytes())))
            }
            "base64.decode" => {
                let s = x.as_str().unwrap_or_default();
                self.charge(size(s))?;
                let Some(bytes) = b64_decode(s) else {
                    return Err(fail("invalid-argument", "not-base64", span, vec![]));
                };
                let Ok(text) = String::from_utf8(bytes) else {
                    return Err(fail("invalid-argument", "not-utf8", span, vec![]));
                };
                self.alloc(1 + size(&text))?;
                Ok(Value::str(text))
            }
            _ => Err(self.internal(span)),
        }
    }

    /// `json.parse` (6.2): pass 1 validates the grammar over the whole text
    /// and finds the first bad number, the depth, `c` and `u` without
    /// allocating values; then the charges; then pass 2 builds the value.
    fn json_parse(&mut self, s: &str, span: Range) -> R<'h, Value<'h>> {
        let mut p = JsonScan {
            s,
            b: s.as_bytes(),
            i: 0,
        };
        let not_json = || fail("invalid-argument", "not-json", span, vec![]);
        let Some(sc) = p.scan(false) else {
            return Err(not_json());
        };
        match sc.bad_number {
            Some(BadNumber::Integer) => {
                return Err(fail("invalid-argument", "integer-range", span, vec![]))
            }
            Some(BadNumber::Infinite) => return Err(self.entry_range("json.parse", span)),
            None => {}
        }
        if self.m.depth + sc.depth > self.m.limits.depth {
            return Err(self.limit(super::DEPTH_EXCEEDED, Some(span)));
        }
        self.charge(sc.c)?;
        self.alloc(sc.u)?;
        p.i = 0;
        let built = p.scan(true).ok_or_else(not_json)?;
        Ok(built.value)
    }
}

enum BadNumber {
    Integer,
    Infinite,
}

struct Scanned<'h> {
    value: Value<'h>,
    bad_number: Option<BadNumber>,
    depth: u32,
    c: u64,
    u: u64,
}

struct JsonScan<'a> {
    s: &'a str,
    b: &'a [u8],
    i: usize,
}

enum Frame<'h> {
    Array {
        items: Vec<Value<'h>>,
        len: usize,
    },
    Object {
        entries: Vec<(Str<'h>, Value<'h>)>,
        keys: HashMap<String, usize>,
        key: Option<String>,
    },
}

impl<'a> JsonScan<'a> {
    fn ws(&mut self) {
        while matches!(self.b.get(self.i), Some(b' ' | b'\t' | b'\n' | b'\r')) {
            self.i += 1;
        }
    }

    fn peek(&self) -> Option<u8> {
        self.b.get(self.i).copied()
    }

    fn eat(&mut self, c: u8) -> bool {
        if self.peek() == Some(c) {
            self.i += 1;
            true
        } else {
            false
        }
    }

    fn word(&mut self, w: &[u8]) -> bool {
        if self.b.get(self.i..self.i + w.len()) == Some(w) {
            self.i += w.len();
            true
        } else {
            false
        }
    }

    fn hex4(&mut self) -> Option<u32> {
        let mut v = 0u32;
        for _ in 0..4 {
            let b = self.peek()?;
            let d = match b {
                b'0'..=b'9' => (b - b'0') as u32,
                b'a'..=b'f' => (b - b'a' + 10) as u32,
                b'A'..=b'F' => (b - b'A' + 10) as u32,
                _ => return None,
            };
            v = v * 16 + d;
            self.i += 1;
        }
        Some(v)
    }

    /// Measure a JSON string after its opening quote without allocating it;
    /// returns its UTF-16 weight (`02-evaluation.md` 4).
    fn string_weight(&mut self) -> Option<u64> {
        let mut w = 0u64;
        loop {
            let b = self.peek()?;
            if b < 0x80 {
                self.i += 1;
                match b {
                    b'"' => return Some(w),
                    b'\\' => {
                        let e = self.peek()?;
                        self.i += 1;
                        match e {
                            b'"' | b'\\' | b'/' | b'b' | b'f' | b'n' | b'r' | b't' => w += 1,
                            b'u' => {
                                let v = self.hex4()?;
                                if (0xD800..0xDC00).contains(&v) {
                                    let save = self.i;
                                    if self.word(b"\\u") {
                                        if let Some(lo) = self.hex4() {
                                            if (0xDC00..0xE000).contains(&lo) {
                                                w += 2;
                                                continue;
                                            }
                                        }
                                    }
                                    self.i = save;
                                    w += 1;
                                } else {
                                    w += 1;
                                }
                            }
                            _ => return None,
                        }
                    }
                    0..=0x1F => return None,
                    _ => w += 1,
                }
            } else {
                let ch = self.s[self.i..].chars().next()?;
                self.i += ch.len_utf8();
                w += if ch as u32 > 0xFFFF { 2 } else { 1 };
            }
        }
    }

    /// A string, after its opening quote (6.2.3 for escapes).
    fn string(&mut self) -> Option<String> {
        let mut out = String::new();
        loop {
            let b = self.peek()?;
            if b < 0x80 {
                self.i += 1;
                match b {
                    b'"' => return Some(out),
                    b'\\' => {
                        let e = self.peek()?;
                        self.i += 1;
                        match e {
                            b'"' => out.push('"'),
                            b'\\' => out.push('\\'),
                            b'/' => out.push('/'),
                            b'b' => out.push('\u{8}'),
                            b'f' => out.push('\u{c}'),
                            b'n' => out.push('\n'),
                            b'r' => out.push('\r'),
                            b't' => out.push('\t'),
                            b'u' => {
                                let v = self.hex4()?;
                                if (0xD800..0xDC00).contains(&v) {
                                    // A high surrogate directly followed by a low one.
                                    let save = self.i;
                                    if self.word(b"\\u") {
                                        if let Some(lo) = self.hex4() {
                                            if (0xDC00..0xE000).contains(&lo) {
                                                let cp =
                                                    0x10000 + ((v - 0xD800) << 10) + (lo - 0xDC00);
                                                out.push(char::from_u32(cp)?);
                                                continue;
                                            }
                                        }
                                    }
                                    self.i = save;
                                    out.push('\u{FFFD}');
                                } else if (0xDC00..0xE000).contains(&v) {
                                    out.push('\u{FFFD}');
                                } else {
                                    out.push(char::from_u32(v)?);
                                }
                            }
                            _ => return None,
                        }
                    }
                    0..=0x1F => return None,
                    _ => out.push(b as char),
                }
            } else {
                let ch = self.s[self.i..].chars().next()?;
                self.i += ch.len_utf8();
                out.push(ch);
            }
        }
    }

    /// A number (6.2.1, 6.2.3): its value, or which rule it breaks.
    fn number(&mut self) -> Option<Result<f64, BadNumber>> {
        let start = self.i;
        self.eat(b'-');
        let int_start = self.i;
        match self.peek() {
            Some(b'0') => self.i += 1,
            Some(b'1'..=b'9') => {
                while matches!(self.peek(), Some(b'0'..=b'9')) {
                    self.i += 1;
                }
            }
            _ => return None,
        }
        let int_end = self.i;
        let mut integer = true;
        if self.eat(b'.') {
            integer = false;
            if !matches!(self.peek(), Some(b'0'..=b'9')) {
                return None;
            }
            while matches!(self.peek(), Some(b'0'..=b'9')) {
                self.i += 1;
            }
        }
        if matches!(self.peek(), Some(b'e' | b'E')) {
            integer = false;
            self.i += 1;
            if matches!(self.peek(), Some(b'+' | b'-')) {
                self.i += 1;
            }
            if !matches!(self.peek(), Some(b'0'..=b'9')) {
                return None;
            }
            while matches!(self.peek(), Some(b'0'..=b'9')) {
                self.i += 1;
            }
        }
        let text = &self.s[start..self.i];
        if integer {
            let digits = &self.s[int_start..int_end];
            let over = digits.len() > MAX_SAFE.len()
                || (digits.len() == MAX_SAFE.len() && digits > MAX_SAFE);
            if over {
                return Some(Err(BadNumber::Integer));
            }
        }
        let v: f64 = text.parse().ok()?;
        if !v.is_finite() {
            return Some(Err(BadNumber::Infinite));
        }
        Some(Ok(if v == 0.0 { 0.0 } else { v }))
    }

    fn scan<'h>(&mut self, build: bool) -> Option<Scanned<'h>> {
        let mut stack: Vec<Frame<'h>> = Vec::new();
        let mut bad_number: Option<BadNumber> = None;
        let mut depth: u32 = 0;
        let mut c: u64 = 0;
        let mut u: u64 = 0;
        self.ws();
        let result: Option<Value<'h>>;
        // Read a value, then close containers as far as the text allows.
        'value: loop {
            self.ws();
            c += 1;
            let v: Value<'h> = match self.peek()? {
                b'{' => {
                    self.i += 1;
                    stack.push(Frame::Object {
                        entries: Vec::new(),
                        keys: HashMap::new(),
                        key: None,
                    });
                    depth = depth.max(stack.len() as u32);
                    self.ws();
                    if self.eat(b'}') {
                        stack.pop();
                        u += 1;
                        if build {
                            Value::Map(Rc::new(MapVal(Vec::new())))
                        } else {
                            Value::Null
                        }
                    } else {
                        if !self.eat(b'"') {
                            return None;
                        }
                        let k = self.string()?;
                        self.ws();
                        if !self.eat(b':') {
                            return None;
                        }
                        if let Some(Frame::Object { key, .. }) = stack.last_mut() {
                            *key = Some(k);
                        }
                        continue 'value;
                    }
                }
                b'[' => {
                    self.i += 1;
                    stack.push(Frame::Array {
                        items: Vec::new(),
                        len: 0,
                    });
                    depth = depth.max(stack.len() as u32);
                    self.ws();
                    if self.eat(b']') {
                        stack.pop();
                        u += 1;
                        if build {
                            Value::list(Vec::new())
                        } else {
                            Value::Null
                        }
                    } else {
                        continue 'value;
                    }
                }
                b'"' => {
                    self.i += 1;
                    if build {
                        let s = self.string()?;
                        u += 1 + size(&s);
                        Value::str(s)
                    } else {
                        let w = self.string_weight()?;
                        u += 1 + w / 64;
                        Value::Null
                    }
                }
                b't' if self.word(b"true") => Value::Bool(true),
                b'f' if self.word(b"false") => Value::Bool(false),
                b'n' if self.word(b"null") => Value::Null,
                b'-' | b'0'..=b'9' => match self.number()? {
                    Ok(n) => Value::Num(n),
                    Err(b) => {
                        if bad_number.is_none() {
                            bad_number = Some(b);
                        }
                        Value::Null
                    }
                },
                _ => return None,
            };
            // Place the value, and close what ends after it.
            let mut v = v;
            loop {
                match stack.last_mut() {
                    None => {
                        result = Some(v);
                        break 'value;
                    }
                    Some(Frame::Array { items, len }) => {
                        if build {
                            items.push(v);
                        }
                        *len += 1;
                        self.ws();
                        if self.eat(b',') {
                            continue 'value;
                        }
                        if !self.eat(b']') {
                            return None;
                        }
                        let Some(Frame::Array { items, len }) = stack.pop() else {
                            return None;
                        };
                        u += 1 + len as u64;
                        v = if build {
                            Value::list(items)
                        } else {
                            Value::Null
                        };
                    }
                    Some(Frame::Object { entries, keys, key }) => {
                        let k = key.take()?;
                        if build {
                            if let Some(&idx) = keys.get(&k) {
                                release(std::mem::replace(&mut entries[idx].1, v));
                            } else {
                                let idx = entries.len();
                                keys.insert(k.clone(), idx);
                                entries.push((Str::new(k), v));
                            }
                        } else {
                            keys.entry(k).or_insert(0);
                        }
                        self.ws();
                        if self.eat(b',') {
                            self.ws();
                            if !self.eat(b'"') {
                                return None;
                            }
                            let k = self.string()?;
                            self.ws();
                            if !self.eat(b':') {
                                return None;
                            }
                            *key = Some(k);
                            continue 'value;
                        }
                        if !self.eat(b'}') {
                            return None;
                        }
                        let Some(Frame::Object {
                            mut entries, keys, ..
                        }) = stack.pop()
                        else {
                            return None;
                        };
                        u += 1 + 2 * keys.len() as u64;
                        v = if build {
                            entries.sort_unstable_by(|a, b| a.0.as_str().cmp(b.0.as_str()));
                            Value::Map(Rc::new(MapVal(entries)))
                        } else {
                            Value::Null
                        };
                    }
                }
            }
        }
        self.ws();
        if self.i != self.b.len() {
            return None;
        }
        Some(Scanned {
            value: result?,
            bad_number,
            depth,
            c,
            u,
        })
    }
}

// ---- time (7) ----------------------------------------------------------------

/// The days from 1970-01-01 to a date of the proleptic Gregorian calendar.
fn days_from_civil(y: i64, m: i64, d: i64) -> i64 {
    let y = if m <= 2 { y - 1 } else { y };
    let era = y.div_euclid(400);
    let yoe = y - era * 400;
    let mp = (m + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

/// The date of a count of days from 1970-01-01.
fn civil_from_days(z: i64) -> (i64, i64, i64) {
    let z = z + 719468;
    let era = z.div_euclid(146097);
    let doe = z - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let m = if mp < 10 { mp + 3 } else { mp - 9 };
    (if m <= 2 { y + 1 } else { y }, m, d)
}

fn leap(y: i64) -> bool {
    (y % 4 == 0 && y % 100 != 0) || y % 400 == 0
}

fn month_len(y: i64, m: i64) -> i64 {
    match m {
        2 => {
            if leap(y) {
                29
            } else {
                28
            }
        }
        4 | 6 | 9 | 11 => 30,
        _ => 31,
    }
}

/// The double nearest to the exact value of a decimal: an integer `n` (with
/// its sign) times 10 to the power of minus `scale`.
fn nearest(neg: bool, digits: &str, scale: usize) -> f64 {
    let d = digits.trim_start_matches('0');
    if d.is_empty() {
        return 0.0;
    }
    let text = format!("{}{}e-{}", if neg { "-" } else { "" }, d, scale);
    let v: f64 = text.parse().unwrap_or(f64::INFINITY);
    if v == 0.0 {
        0.0
    } else {
        v
    }
}

/// `a - b` for decimal digit strings with `a >= b`.
fn sub_digits(a: &str, b: &str) -> String {
    let a: Vec<u8> = a.bytes().rev().map(|c| c - b'0').collect();
    let b: Vec<u8> = b.bytes().rev().map(|c| c - b'0').collect();
    let mut out = Vec::with_capacity(a.len());
    let mut borrow = 0i8;
    for (i, &x) in a.iter().enumerate() {
        let mut d = x as i8 - borrow - *b.get(i).unwrap_or(&0) as i8;
        if d < 0 {
            d += 10;
            borrow = 1;
        } else {
            borrow = 0;
        }
        out.push(b'0' + d as u8);
    }
    while out.len() > 1 && *out.last().unwrap() == b'0' {
        out.pop();
    }
    out.reverse();
    String::from_utf8(out).unwrap_or_default()
}

/// `time.parse` (7.2): the time a text names, or `None`.
fn parse_time(s: &str) -> Option<f64> {
    let b: Vec<char> = s.chars().collect();
    let num = |i: usize, n: usize| -> Option<i64> {
        let mut v = 0i64;
        for k in 0..n {
            let c = *b.get(i + k)?;
            if !c.is_ascii_digit() {
                return None;
            }
            v = v * 10 + (c as i64 - '0' as i64);
        }
        Some(v)
    };
    let at = |i: usize, c: char| b.get(i) == Some(&c);
    let y = num(0, 4)?;
    if !at(4, '-') {
        return None;
    }
    let mo = num(5, 2)?;
    if !at(7, '-') {
        return None;
    }
    let d = num(8, 2)?;
    if !at(10, 'T') {
        return None;
    }
    let h = num(11, 2)?;
    if !at(13, ':') {
        return None;
    }
    let mi = num(14, 2)?;
    if !at(16, ':') {
        return None;
    }
    let se = num(17, 2)?;
    let mut i = 19;
    let mut frac = String::new();
    if at(i, '.') {
        i += 1;
        while let Some(c) = b.get(i) {
            if !c.is_ascii_digit() {
                break;
            }
            frac.push(*c);
            i += 1;
        }
        if frac.is_empty() {
            return None;
        }
    }
    let offset = match b.get(i) {
        Some('Z') => {
            i += 1;
            0
        }
        Some(&sign @ ('+' | '-')) => {
            let oh = num(i + 1, 2)?;
            if !at(i + 3, ':') {
                return None;
            }
            let om = num(i + 4, 2)?;
            if oh > 23 || om > 59 {
                return None;
            }
            i += 6;
            let o = oh * 3600 + om * 60;
            if sign == '-' {
                -o
            } else {
                o
            }
        }
        _ => return None,
    };
    if i != b.len() {
        return None;
    }
    if !(1..=12).contains(&mo) || d < 1 || d > month_len(y, mo) || h > 23 || mi > 59 || se > 59 {
        return None;
    }
    let w = 86400 * days_from_civil(y, mo, d) + 3600 * h + 60 * mi + se - offset;
    if frac.is_empty() {
        return Some(w as f64);
    }
    // The exact value W + 0.F, as an integer over 10^n.
    let n = frac.len();
    let (neg, digits) = if w >= 0 {
        (false, format!("{}{}", w, frac))
    } else {
        // W + F = -(|W| - F) = -((|W| * 10^n - F) / 10^n).
        let big = format!("{}{}", -w, "0".repeat(n));
        (true, sub_digits(&big, &frac))
    };
    Some(nearest(neg, &digits, n))
}

/// The exact decimal of the number-to-text form of `t`: sign, digits and
/// the count of digits after the point.
fn exact_decimal(t: f64) -> (bool, String, usize) {
    let text = number::to_text(t);
    let (neg, rest) = match text.strip_prefix('-') {
        Some(r) => (true, r),
        None => (false, text.as_str()),
    };
    let (mant, exp) = match rest.split_once('e') {
        Some((m, e)) => (m, e.parse::<i64>().unwrap_or(0)),
        None => (rest, 0),
    };
    let (ip, fp) = mant.split_once('.').unwrap_or((mant, ""));
    let digits = format!("{ip}{fp}");
    let scale = fp.len() as i64 - exp;
    if scale <= 0 {
        (
            neg,
            format!("{}{}", digits, "0".repeat((-scale) as usize)),
            0,
        )
    } else {
        (neg, digits, scale as usize)
    }
}

/// `time.format` (7.3): the text of a time, or `None` outside years 0000 to
/// 9999.
fn format_time(t: f64) -> Option<String> {
    let w = t.floor();
    if !(-62167219200.0..253402300800.0).contains(&w) {
        return None;
    }
    let w = w as i64;
    let days = w.div_euclid(86400);
    let secs = w.rem_euclid(86400);
    let (y, m, d) = civil_from_days(days);
    let mut out = format!(
        "{:04}-{:02}-{:02}T{:02}:{:02}:{:02}",
        y,
        m,
        d,
        secs / 3600,
        secs / 60 % 60,
        secs % 60
    );
    if t != w as f64 {
        let (neg, digits, k) = exact_decimal(t);
        // F = (V - W) * 10^k.
        let wk = format!("{}{}", w.unsigned_abs(), "0".repeat(k));
        let f = if w == 0 {
            digits.trim_start_matches('0').to_string()
        } else if neg {
            sub_digits(&wk, &digits)
        } else {
            sub_digits(&digits, &wk)
        };
        out.push('.');
        out.push_str(&"0".repeat(k.saturating_sub(f.len())));
        out.push_str(f.trim_end_matches('0'));
    }
    out.push('Z');
    Some(out)
}

// Fixed-width decimal accumulator for `time.duration`: every binary64 value
// and midpoint between adjacent binary64 values terminates by 10^-1075 (half of
// the smallest subnormal 2^-1074) and overflows below 10^309. The 1,120
// fractional columns (45 guard digits below 10^-1075), 320 integer columns,
// and streaming tail windows use O(1) scratch memory; on the exact-comparison
// path (when lower and upper estimates straddle a binary64 midpoint),
// `compare_duration_tail` additionally retains O(k) term records, where `k` is
// the number of terms with nonzero truncated tails. Every valid duration rounds
// to the exact nearest binary64, ties to even.
const DUR_FRAC_COLS: usize = 1120;
const DUR_INT_COLS: usize = 320;
const DUR_TOTAL_COLS: usize = DUR_FRAC_COLS + DUR_INT_COLS;
const DUR_GUARD_COLS: usize = DUR_FRAC_COLS - 1075;
const DUR_TAIL_WIN: usize = 1024;

/// Convert a non-negative base-10 column array (column `c` = 10^(c - DUR_FRAC_COLS))
/// to the nearest `f64`, ties to even.
fn cols_to_f64(cols: &[u128]) -> f64 {
    let mut top = cols.len() - 1;
    while top > DUR_FRAC_COLS && cols[top] == 0 {
        top -= 1;
    }
    let mut lo = 0usize;
    while lo < DUR_FRAC_COLS && cols[lo] == 0 {
        lo += 1;
    }
    if top == DUR_FRAC_COLS && cols[DUR_FRAC_COLS] == 0 && lo == DUR_FRAC_COLS {
        return 0.0;
    }
    let mut text = String::with_capacity(top - lo + 2);
    for i in (DUR_FRAC_COLS..=top).rev() {
        text.push((b'0' + cols[i] as u8) as char);
    }
    if lo < DUR_FRAC_COLS {
        text.push('.');
        for i in (lo..DUR_FRAC_COLS).rev() {
            text.push((b'0' + cols[i] as u8) as char);
        }
    }
    let v: f64 = text.parse().unwrap_or(f64::INFINITY);
    if v == 0.0 {
        0.0
    } else {
        v
    }
}

/// Compare the sum of all fractional digits strictly below `10^-DUR_FRAC_COLS`
/// against `deficit * 10^-DUR_FRAC_COLS`, in fixed-width streaming windows.
fn compare_duration_tail(b: &[u8], deficit: u128, eps: u128) -> std::cmp::Ordering {
    use std::cmp::Ordering;
    let mut i = usize::from(b.first() == Some(&b'-'));
    let mut long_terms: Vec<(usize, usize, u128, usize)> = Vec::new();
    while i < b.len() {
        while i < b.len() && b[i].is_ascii_digit() {
            i += 1;
        }
        let (fp_start, fp_end) = if b.get(i) == Some(&b'.') {
            i += 1;
            let start = i;
            while i < b.len() && b[i].is_ascii_digit() {
                i += 1;
            }
            (start, i)
        } else {
            (i, i)
        };
        let (mul, shift): (u128, usize) = match (b.get(i), b.get(i + 1)) {
            (Some(b'm'), Some(b's')) => {
                i += 2;
                (1, 3)
            }
            (Some(b's'), _) => {
                i += 1;
                (1, 0)
            }
            (Some(b'm'), _) => {
                i += 1;
                (60, 0)
            }
            (Some(b'h'), _) => {
                i += 1;
                (3600, 0)
            }
            (Some(b'd'), _) => {
                i += 1;
                (86_400, 0)
            }
            _ => break,
        };
        let cutoff = DUR_FRAC_COLS - shift;
        if fp_end - fp_start > cutoff {
            let mut trim = fp_end;
            while trim > fp_start + cutoff && b[trim - 1] == b'0' {
                trim -= 1;
            }
            if trim > fp_start + cutoff {
                long_terms.push((fp_start, trim, mul, shift));
            }
        }
    }

    let mut lo = DUR_FRAC_COLS;
    let mut rem = deficit;
    let mut win = vec![0u128; DUR_TAIL_WIN];
    loop {
        win.fill(0);
        let hi = lo + DUR_TAIL_WIN;
        for &(fp_start, fp_trim, mul, shift) in &long_terms {
            let fp_len = fp_trim - fp_start;
            let k_start = lo - shift;
            let k_end = (hi - shift).min(fp_len);
            for k in k_start..k_end {
                let d = (b[fp_start + k] - b'0') as u128;
                if d != 0 {
                    win[DUR_TAIL_WIN - 1 - (k - k_start)] += d * mul;
                }
            }
        }
        long_terms.retain(|&(fp_start, fp_trim, _, shift)| fp_trim - fp_start > hi - shift);
        let any_below_hi = !long_terms.is_empty();

        let mut carry = 0u128;
        for cell in &mut win {
            let sum = *cell + carry;
            *cell = sum % 10;
            carry = sum / 10;
        }
        if carry > rem {
            return Ordering::Greater;
        }
        rem -= carry;
        for j in (0..DUR_TAIL_WIN).rev() {
            if rem > eps {
                return Ordering::Less;
            }
            let r10 = rem * 10;
            if win[j] > r10 {
                return Ordering::Greater;
            }
            rem = r10 - win[j];
        }
        if rem == 0 {
            return if any_below_hi {
                Ordering::Greater
            } else {
                Ordering::Equal
            };
        }
        if !any_below_hi {
            return Ordering::Less;
        }
        lo = hi;
    }
}

/// `time.duration` (7.4): `None` for a text that is no duration, `Some(None)`
/// when the sum is too large for a number.
fn parse_duration(s: &str) -> Option<Option<f64>> {
    let b = s.as_bytes();
    let neg = b.first() == Some(&b'-');
    let mut i = usize::from(neg);
    if i == b.len() {
        return None;
    }
    let mut cols: Vec<u128> = vec![0; DUR_TOTAL_COLS];
    let mut overflow = false;
    let mut tail_terms: u128 = 0;

    while i < b.len() {
        let ip_start = i;
        while i < b.len() && b[i].is_ascii_digit() {
            i += 1;
        }
        let ip_end = i;
        if ip_end == ip_start {
            return None;
        }
        let (fp_start, fp_end) = if b.get(i) == Some(&b'.') {
            i += 1;
            let start = i;
            while i < b.len() && b[i].is_ascii_digit() {
                i += 1;
            }
            if i == start {
                return None;
            }
            (start, i)
        } else {
            (i, i)
        };
        let (mul, shift): (u128, usize) = match (b.get(i), b.get(i + 1)) {
            (Some(b'm'), Some(b's')) => {
                i += 2;
                (1, 3)
            }
            (Some(b's'), _) => {
                i += 1;
                (1, 0)
            }
            (Some(b'm'), _) => {
                i += 1;
                (60, 0)
            }
            (Some(b'h'), _) => {
                i += 1;
                (3600, 0)
            }
            (Some(b'd'), _) => {
                i += 1;
                (86_400, 0)
            }
            _ => return None,
        };
        if !overflow {
            let mut ip_nz = ip_start;
            while ip_nz < ip_end && b[ip_nz] == b'0' {
                ip_nz += 1;
            }
            if ip_end - ip_nz > DUR_INT_COLS + shift {
                overflow = true;
            } else {
                for (p, &ch) in b[ip_nz..ip_end].iter().rev().enumerate() {
                    let d = (ch - b'0') as u128;
                    if d != 0 {
                        let col = DUR_FRAC_COLS + p - shift;
                        if col >= DUR_TOTAL_COLS {
                            overflow = true;
                            break;
                        }
                        cols[col] += d * mul;
                    }
                }
            }
            if !overflow {
                let cutoff = DUR_FRAC_COLS - shift;
                let fp_len = fp_end - fp_start;
                let keep = fp_len.min(cutoff);
                for k in 0..keep {
                    let d = (b[fp_start + k] - b'0') as u128;
                    if d != 0 {
                        cols[DUR_FRAC_COLS - 1 - shift - k] += d * mul;
                    }
                }
                if fp_len > cutoff && b[fp_start + cutoff..fp_end].iter().any(|&c| c != b'0') {
                    tail_terms += 1;
                }
            }
        }
    }

    if overflow {
        return Some(None);
    }
    let mut carry = 0u128;
    for col in &mut cols {
        let sum = *col + carry;
        *col = sum % 10;
        carry = sum / 10;
    }
    if carry > 0 {
        return Some(None);
    }
    let finish = |v: f64| -> Option<Option<f64>> {
        if !v.is_finite() {
            Some(None)
        } else if neg && v != 0.0 {
            Some(Some(-v))
        } else {
            Some(Some(if v == 0.0 { 0.0 } else { v }))
        }
    };
    let f_a = cols_to_f64(&cols);
    if !f_a.is_finite() {
        return Some(None);
    }
    if tail_terms == 0 {
        return finish(f_a);
    }

    // Exact sum lies strictly in (A, B), where B = A + eps * 10^-DUR_FRAC_COLS.
    let eps = tail_terms * 86_400;
    let mut cols_b = cols.clone();
    let mut c = eps;
    for col in &mut cols_b {
        if c == 0 {
            break;
        }
        let sum = *col + c;
        *col = sum % 10;
        c = sum / 10;
    }
    let f_b = cols_to_f64(&cols_b);
    if cols[..DUR_GUARD_COLS].iter().all(|&d| d == 0) {
        return finish(f_b);
    }
    if f_b.is_finite() && f_a.to_bits() == f_b.to_bits() {
        return finish(f_a);
    }
    let mut cols_m = cols_b;
    for col in &mut cols_m[..DUR_GUARD_COLS] {
        *col = 0;
    }
    let f_m = cols_to_f64(&cols_m);
    let mut deficit: u128 = 0;
    let mut place: u128 = 1;
    let mut borrow: u128 = 0;
    for &digit in &cols[..DUR_GUARD_COLS] {
        let d = digit + borrow;
        let out = if d == 0 {
            borrow = 0;
            0
        } else {
            borrow = 1;
            10 - d
        };
        if out != 0 {
            deficit += out * place;
        }
        if place <= eps {
            place *= 10;
        }
    }
    match compare_duration_tail(b, deficit, eps) {
        std::cmp::Ordering::Less => finish(f_a),
        std::cmp::Ordering::Equal => finish(f_m),
        std::cmp::Ordering::Greater => finish(f_b),
    }
}

// ---- base64 (9) -----------------------------------------------------------------

const B64: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

fn b64_encode(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len().div_ceil(3) * 4);
    for chunk in bytes.chunks(3) {
        let b = [
            chunk[0],
            *chunk.get(1).unwrap_or(&0),
            *chunk.get(2).unwrap_or(&0),
        ];
        let n = (b[0] as u32) << 16 | (b[1] as u32) << 8 | b[2] as u32;
        for k in 0..4 {
            if k <= chunk.len() {
                out.push(B64[(n >> (18 - 6 * k) & 63) as usize] as char);
            } else {
                out.push('=');
            }
        }
    }
    out
}

/// `base64.decode`'s text rules (9.2 items 1 to 4): the bytes, or `None`.
fn b64_decode(s: &str) -> Option<Vec<u8>> {
    let chars: Vec<char> = s.chars().collect();
    let p = chars.iter().rev().take_while(|c| **c == '=').count();
    let t = &chars[..chars.len() - p];
    let mut std_alpha = false;
    let mut url_alpha = false;
    let mut vals = Vec::with_capacity(t.len());
    for &c in t {
        let v = match c {
            'A'..='Z' => c as u32 - 'A' as u32,
            'a'..='z' => c as u32 - 'a' as u32 + 26,
            '0'..='9' => c as u32 - '0' as u32 + 52,
            '+' => {
                std_alpha = true;
                62
            }
            '/' => {
                std_alpha = true;
                63
            }
            '-' => {
                url_alpha = true;
                62
            }
            '_' => {
                url_alpha = true;
                63
            }
            _ => return None,
        };
        vals.push(v);
    }
    if std_alpha && url_alpha {
        return None;
    }
    let n = vals.len();
    match n % 4 {
        1 => return None,
        0 if p != 0 => return None,
        r @ (2 | 3) if p != 0 && p != 4 - r => return None,
        _ => {}
    }
    let mut out = Vec::with_capacity(n / 4 * 3 + 2);
    for chunk in vals.chunks(4) {
        match chunk.len() {
            4 => {
                let v = chunk[0] << 18 | chunk[1] << 12 | chunk[2] << 6 | chunk[3];
                out.extend([(v >> 16) as u8, (v >> 8) as u8, v as u8]);
            }
            3 => {
                if chunk[2] & 3 != 0 {
                    return None;
                }
                let v = chunk[0] << 18 | chunk[1] << 12 | chunk[2] << 6;
                out.extend([(v >> 16) as u8, (v >> 8) as u8]);
            }
            2 => {
                if chunk[1] & 15 != 0 {
                    return None;
                }
                let v = chunk[0] << 18 | chunk[1] << 12;
                out.push((v >> 16) as u8);
            }
            _ => return None,
        }
    }
    Some(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn times() {
        assert_eq!(parse_time("1970-01-01T00:00:00Z"), Some(0.0));
        assert_eq!(parse_time("2024-02-29T12:30:45Z"), Some(1709209845.0));
        assert_eq!(
            parse_time("2024-02-29T12:30:45.5+01:00"),
            Some(1709206245.5)
        );
        assert_eq!(parse_time("1969-12-31T23:59:59.9Z"), Some(-0.1));
        assert_eq!(parse_time("0000-01-01T00:00:00Z"), Some(-62167219200.0));
        assert_eq!(parse_time("2023-02-29T00:00:00Z"), None);
        assert_eq!(
            format_time(-0.25).as_deref(),
            Some("1969-12-31T23:59:59.75Z")
        );
        assert_eq!(
            format_time(1e-7).as_deref(),
            Some("1970-01-01T00:00:00.0000001Z")
        );
        assert_eq!(
            format_time(253402300799.5).as_deref(),
            Some("9999-12-31T23:59:59.5Z")
        );
        assert_eq!(format_time(253402300800.0), None);
    }

    #[test]
    fn durations() {
        assert_eq!(parse_duration("1h30m"), Some(Some(5400.0)));
        assert_eq!(parse_duration("250ms"), Some(Some(0.25)));
        assert_eq!(parse_duration("0.1s0.2s"), Some(Some(0.3)));
        assert_eq!(parse_duration("-0s"), Some(Some(0.0)));
        assert_eq!(parse_duration("1"), None);
    }

    #[test]
    fn base64() {
        assert_eq!(b64_encode("😀".as_bytes()), "8J+YgA==");
        assert_eq!(b64_decode("8J-YgA").unwrap(), "😀".as_bytes());
        assert!(b64_decode("YR==").is_none());
        assert!(b64_decode("YQ=").is_none());
        assert!(b64_decode("Pj4+Pz8_").is_none());
    }
}
