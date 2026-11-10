//! A strict reader of JSON text, for case files and data files.
//!
//! It refuses an object that repeats a key and a number that reads as an
//! infinity (`03-host.md` 3.10.10, items 1 and 13), and reads every number by
//! the rule of text to number (`01-language.md` 6.3 rule 2), which is the JSON
//! number grammar. An object keeps its keys in the order written.

use std::fmt;

#[derive(Clone, Debug, PartialEq)]
pub enum Json {
    Null,
    Bool(bool),
    Num(f64),
    Str(String),
    Arr(Vec<Json>),
    Obj(Vec<(String, Json)>),
}

impl Json {
    pub fn get(&self, key: &str) -> Option<&Json> {
        match self {
            Json::Obj(m) => m.iter().find(|(k, _)| k == key).map(|(_, v)| v),
            _ => None,
        }
    }

    pub fn as_str(&self) -> Option<&str> {
        match self {
            Json::Str(s) => Some(s),
            _ => None,
        }
    }

    pub fn as_obj(&self) -> Option<&[(String, Json)]> {
        match self {
            Json::Obj(m) => Some(m),
            _ => None,
        }
    }

    pub fn as_arr(&self) -> Option<&[Json]> {
        match self {
            Json::Arr(a) => Some(a),
            _ => None,
        }
    }

    pub fn as_f64(&self) -> Option<f64> {
        match self {
            Json::Num(n) => Some(*n),
            _ => None,
        }
    }

    /// A number with no fractional part (JSON Schema's `integer`).
    pub fn as_integer(&self) -> Option<f64> {
        match self {
            Json::Num(n) if n.fract() == 0.0 => Some(*n),
            _ => None,
        }
    }

    pub fn kind(&self) -> &'static str {
        match self {
            Json::Null => "null",
            Json::Bool(_) => "boolean",
            Json::Num(_) => "number",
            Json::Str(_) => "string",
            Json::Arr(_) => "array",
            Json::Obj(_) => "object",
        }
    }
}

impl fmt::Display for Json {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Json::Null => f.write_str("null"),
            Json::Bool(b) => write!(f, "{b}"),
            Json::Num(n) => write!(f, "{n}"),
            Json::Str(s) => write!(f, "{s:?}"),
            Json::Arr(a) => {
                f.write_str("[")?;
                for (i, v) in a.iter().enumerate() {
                    if i > 0 {
                        f.write_str(", ")?;
                    }
                    write!(f, "{v}")?;
                }
                f.write_str("]")
            }
            Json::Obj(m) => {
                f.write_str("{")?;
                for (i, (k, v)) in m.iter().enumerate() {
                    if i > 0 {
                        f.write_str(", ")?;
                    }
                    write!(f, "{k:?}: {v}")?;
                }
                f.write_str("}")
            }
        }
    }
}

/// Nesting deeper than this is refused, so that reading never overflows the
/// stack.
pub const MAX_DEPTH: usize = 512;

pub fn parse(text: &str) -> Result<Json, String> {
    let mut r = Reader {
        s: text.as_bytes(),
        i: 0,
        text,
    };
    r.ws();
    let v = r.value(0)?;
    r.ws();
    if r.i != r.s.len() {
        return Err(r.err("text after the JSON value"));
    }
    Ok(v)
}

struct Reader<'a> {
    s: &'a [u8],
    i: usize,
    text: &'a str,
}

impl Reader<'_> {
    fn err(&self, msg: &str) -> String {
        format!("{msg} at byte {}", self.i)
    }

    fn ws(&mut self) {
        while self.i < self.s.len() && matches!(self.s[self.i], b' ' | b'\t' | b'\n' | b'\r') {
            self.i += 1;
        }
    }

    fn lit(&mut self, word: &str, v: Json) -> Result<Json, String> {
        if self.s[self.i..].starts_with(word.as_bytes()) {
            self.i += word.len();
            Ok(v)
        } else {
            Err(self.err("not JSON"))
        }
    }

    fn value(&mut self, depth: usize) -> Result<Json, String> {
        if depth > MAX_DEPTH {
            return Err(self.err("JSON nested too deep"));
        }
        match self.s.get(self.i) {
            None => Err(self.err("the JSON text ends early")),
            Some(b'n') => self.lit("null", Json::Null),
            Some(b't') => self.lit("true", Json::Bool(true)),
            Some(b'f') => self.lit("false", Json::Bool(false)),
            Some(b'"') => Ok(Json::Str(self.string()?)),
            Some(b'[') => {
                self.i += 1;
                let mut items = Vec::new();
                self.ws();
                if self.s.get(self.i) == Some(&b']') {
                    self.i += 1;
                    return Ok(Json::Arr(items));
                }
                loop {
                    self.ws();
                    items.push(self.value(depth + 1)?);
                    self.ws();
                    match self.s.get(self.i) {
                        Some(b',') => self.i += 1,
                        Some(b']') => {
                            self.i += 1;
                            return Ok(Json::Arr(items));
                        }
                        _ => return Err(self.err("expected , or ]")),
                    }
                }
            }
            Some(b'{') => {
                self.i += 1;
                let mut entries: Vec<(String, Json)> = Vec::new();
                self.ws();
                if self.s.get(self.i) == Some(&b'}') {
                    self.i += 1;
                    return Ok(Json::Obj(entries));
                }
                loop {
                    self.ws();
                    if self.s.get(self.i) != Some(&b'"') {
                        return Err(self.err("expected a key"));
                    }
                    let k = self.string()?;
                    if entries.iter().any(|(e, _)| *e == k) {
                        return Err(self.err(&format!("the key {k:?} is repeated")));
                    }
                    self.ws();
                    if self.s.get(self.i) != Some(&b':') {
                        return Err(self.err("expected :"));
                    }
                    self.i += 1;
                    self.ws();
                    let v = self.value(depth + 1)?;
                    entries.push((k, v));
                    self.ws();
                    match self.s.get(self.i) {
                        Some(b',') => self.i += 1,
                        Some(b'}') => {
                            self.i += 1;
                            return Ok(Json::Obj(entries));
                        }
                        _ => return Err(self.err("expected , or }")),
                    }
                }
            }
            Some(b'-' | b'0'..=b'9') => self.number(),
            _ => Err(self.err("not JSON")),
        }
    }

    fn number(&mut self) -> Result<Json, String> {
        let start = self.i;
        let digits = |r: &mut Self| -> usize {
            let a = r.i;
            while r.i < r.s.len() && r.s[r.i].is_ascii_digit() {
                r.i += 1;
            }
            r.i - a
        };
        if self.s.get(self.i) == Some(&b'-') {
            self.i += 1;
        }
        if self.s.get(self.i) == Some(&b'0') {
            self.i += 1;
        } else if digits(self) == 0 {
            return Err(self.err("bad number"));
        }
        if self.s.get(self.i) == Some(&b'.') {
            self.i += 1;
            if digits(self) == 0 {
                return Err(self.err("bad number"));
            }
        }
        if matches!(self.s.get(self.i), Some(b'e' | b'E')) {
            self.i += 1;
            if matches!(self.s.get(self.i), Some(b'+' | b'-')) {
                self.i += 1;
            }
            if digits(self) == 0 {
                return Err(self.err("bad number"));
            }
        }
        let t = &self.text[start..self.i];
        let v: f64 = t.parse().map_err(|_| self.err("bad number"))?;
        if !v.is_finite() {
            return Err(self.err(&format!("the number {t} reads as an infinity")));
        }
        Ok(Json::Num(if v == 0.0 { 0.0 } else { v }))
    }

    fn hex4(&mut self) -> Result<u32, String> {
        if self.i + 4 > self.s.len() {
            return Err(self.err("bad \\u escape"));
        }
        let t = &self.text[self.i..self.i + 4];
        let v = u32::from_str_radix(t, 16).map_err(|_| self.err("bad \\u escape"))?;
        if !t.bytes().all(|b| b.is_ascii_hexdigit()) {
            return Err(self.err("bad \\u escape"));
        }
        self.i += 4;
        Ok(v)
    }

    /// A string. A lone surrogate reads as U+FFFD (`03-host.md` 3.4.5).
    fn string(&mut self) -> Result<String, String> {
        self.i += 1;
        let mut out = String::new();
        loop {
            let start = self.i;
            while self.i < self.s.len() && !matches!(self.s[self.i], b'"' | b'\\' | 0..=0x1f) {
                self.i += 1;
            }
            out.push_str(&self.text[start..self.i]);
            match self.s.get(self.i) {
                None => return Err(self.err("unterminated string")),
                Some(b'"') => {
                    self.i += 1;
                    return Ok(out);
                }
                Some(b'\\') => {
                    self.i += 1;
                    let e = *self.s.get(self.i).ok_or_else(|| self.err("bad escape"))?;
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
                            if (0xD800..0xDC00).contains(&v)
                                && self.s.get(self.i) == Some(&b'\\')
                                && self.s.get(self.i + 1) == Some(&b'u')
                            {
                                let save = self.i;
                                self.i += 2;
                                let lo = self.hex4()?;
                                if (0xDC00..0xE000).contains(&lo) {
                                    let cp = 0x10000 + ((v - 0xD800) << 10) + (lo - 0xDC00);
                                    out.push(char::from_u32(cp).unwrap_or('\u{FFFD}'));
                                    continue;
                                }
                                self.i = save;
                            }
                            out.push(char::from_u32(v).unwrap_or('\u{FFFD}'));
                        }
                        _ => return Err(self.err("bad escape")),
                    }
                }
                Some(_) => return Err(self.err("a control character in a string")),
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reads_and_refuses() {
        assert_eq!(
            parse(r#"{"a": [1, 2.5, "xé", null, true]}"#).unwrap(),
            Json::Obj(vec![(
                "a".into(),
                Json::Arr(vec![
                    Json::Num(1.0),
                    Json::Num(2.5),
                    Json::Str("x\u{e9}".into()),
                    Json::Null,
                    Json::Bool(true)
                ])
            )])
        );
        assert!(parse(r#"{"a": 1, "a": 2}"#).is_err());
        assert!(parse("1e400").is_err());
        assert!(parse("01").is_err());
        assert_eq!(parse(r#""\ud800""#).unwrap(), Json::Str("\u{FFFD}".into()));
        assert_eq!(parse(r#""😀""#).unwrap(), Json::Str("\u{1F600}".into()));
    }
}
