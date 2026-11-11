//! The three renderings of diagnostics as text: one line, detail and JSON
//! (`f08-errors.md` 5).

use crate::diag::{Diagnostic, PathStep};
use crate::host::Data;
use crate::messages::{cut, push_json_char, visible};

/// What a rendering reads of one diagnostic (5.1 item 3).
#[derive(Clone, Debug, Default)]
pub struct View<'a> {
    pub phase: &'a str,
    pub code: &'a str,
    pub class: Option<&'a str>,
    pub message: &'a str,
    pub span: Option<[u32; 4]>,
    pub path: Option<&'a [PathStep]>,
    pub data: Option<&'a Data>,
    /// The trace, from the outermost in: the span, the name and the index.
    pub trace: Vec<([u32; 4], &'a str, Option<usize>)>,
}

impl<'a> View<'a> {
    pub fn of(d: &'a Diagnostic) -> View<'a> {
        View {
            phase: d.phase.as_str(),
            code: d.code,
            class: d.class,
            message: &d.message,
            span: d.span.map(|s| s.arr()),
            path: d.path.as_deref().map(|p| p.as_slice()),
            data: d.data.as_deref(),
            trace: d
                .trace()
                .iter()
                .map(|e| (e.span.arr(), e.name.as_str(), e.index))
                .collect(),
        }
    }
}

fn span_text(s: [u32; 4]) -> String {
    format!("{}:{}-{}:{}", s[0], s[1], s[2], s[3])
}

/// A `Word` (`01-language.md` 2.4 rule 1).
fn is_word(k: &str) -> bool {
    let mut cs = k.chars();
    cs.next()
        .is_some_and(|c| c.is_ascii_alphabetic() || c == '_')
        && cs.all(|c| c.is_ascii_alphanumeric() || c == '_')
}

/// A path written from its steps (5.1 item 5).
pub fn path_text(path: &[PathStep]) -> String {
    let mut out = String::new();
    for step in path {
        match step {
            PathStep::Key(k) if is_word(k) => {
                out.push('.');
                out.push_str(&cut(k));
            }
            PathStep::Key(k) => {
                let mut j = String::from("\"");
                for c in k.chars() {
                    push_json_char(&mut j, c);
                }
                j.push('"');
                out.push('[');
                out.push_str(&cut(&j));
                out.push(']');
            }
            PathStep::Index(i) => out.push_str(&format!("[{i}]")),
        }
    }
    out
}

/// The one-line form of one diagnostic, without its line feed (5.2).
fn line_of(d: &View<'_>) -> String {
    let mut out = format!("error[{}]", d.code);
    if let Some(s) = d.span {
        out.push(' ');
        out.push_str(&span_text(s));
    }
    out.push_str(": ");
    out.push_str(&visible(d.message));
    if let Some(p) = d.path.filter(|p| !p.is_empty()) {
        out.push_str(" at ");
        out.push_str(&path_text(p));
    }
    out
}

/// The one-line form (5.2).
pub fn line(ds: &[View<'_>]) -> String {
    let mut out = String::new();
    for d in ds {
        out.push_str(&line_of(d));
        out.push('\n');
    }
    out
}

/// The excerpt and the caret line of a span (5.4).
fn excerpt(source: &str, s: [u32; 4]) -> (String, String) {
    let text = source.strip_prefix('\u{feff}').unwrap_or(source);
    let line_no = s[0] as usize;
    let line: Vec<char> = text
        .split('\n')
        .nth(line_no.saturating_sub(1))
        .unwrap_or("")
        .chars()
        .collect();
    let n = line.len() as i64;
    let a = s[1] as i64;
    let mut b = if s[2] == s[0] { s[3] as i64 } else { n + 1 };
    if b <= a {
        b = a + 1;
    }
    let shown = |c: char| {
        let u = c as u32;
        if u < 0x20 || (0x7f..=0x9f).contains(&u) {
            ' '
        } else {
            c
        }
    };
    let (start, mut ex) = if n <= 120 {
        (1, line.iter().map(|c| shown(*c)).collect::<String>())
    } else {
        let mut st = a - 40;
        if st < 1 {
            st = 1;
        }
        if st > n - 119 {
            st = n - 119;
        }
        let mut e = String::new();
        if st > 1 {
            e.push_str("...");
        }
        for k in st..st + 120 {
            if let Some(c) = line.get((k - 1) as usize) {
                e.push(shown(*c));
            }
        }
        (st, e)
    };
    let lead = if ex.starts_with("...") && n > 120 {
        3
    } else {
        0
    };
    if n > 120 && start + 120 <= n {
        ex.push_str("...");
    }
    let spaces = (lead + a - start).max(0) as usize;
    let carets = (b.min(start + 120) - a).max(1) as usize;
    (ex, format!("{}{}", " ".repeat(spaces), "^".repeat(carets)))
}

/// The detail form (5.3).
pub fn detail(ds: &[View<'_>], source: Option<&str>) -> String {
    let mut out = String::new();
    for d in ds {
        out.push_str(&line_of(d));
        out.push('\n');
        if let (Some(s), Some(src)) = (d.span, source) {
            let (ex, carets) = excerpt(src, s);
            out.push_str("    ");
            out.push_str(&ex);
            out.push('\n');
            out.push_str("    ");
            out.push_str(&carets);
            out.push('\n');
        }
        if d.phase == "run" {
            for (span, name, index) in d.trace.iter().rev() {
                out.push_str("  in ");
                out.push_str(name);
                if let Some(i) = index {
                    out.push_str(&format!(", element {i}"));
                }
                out.push_str(", at ");
                out.push_str(&span_text(*span));
                out.push('\n');
            }
        }
    }
    out
}

fn json_str(out: &mut String, s: &str) {
    out.push('"');
    for c in s.chars() {
        push_json_char(out, c);
    }
    out.push('"');
}

/// `data` in its JSON form (5.6), or `None` past nesting 200 or 10,000
/// code points. It stops writing at the bound, so shared or cyclic host
/// data costs no more than the bound.
fn data_json(d: &Data) -> Option<String> {
    struct W {
        out: String,
        count: usize,
    }
    impl W {
        fn put(&mut self, s: &str) -> Option<()> {
            self.count += s.chars().count();
            self.out.push_str(s);
            (self.count <= 10_000).then_some(())
        }
        fn string(&mut self, s: &str) -> Option<()> {
            let mut j = String::new();
            json_str(&mut j, s);
            self.put(&j)
        }
    }
    fn go(d: &Data, w: &mut W, depth: u32) -> Option<()> {
        match d {
            Data::Null | Data::Opaque(_) => w.put("null"),
            Data::Bool(b) => w.put(if *b { "true" } else { "false" }),
            Data::Int(i) => w.put(&crate::number::to_text(*i as f64)),
            Data::Float(f) => w.put(&crate::number::to_text(*f)),
            Data::Str(s) => w.string(s),
            Data::List(l) => {
                if depth + 1 > 200 {
                    return None;
                }
                w.put("[")?;
                for (i, x) in l.iter().enumerate() {
                    if i > 0 {
                        w.put(",")?;
                    }
                    go(x, w, depth + 1)?;
                }
                w.put("]")
            }
            Data::Map(m) => {
                if depth + 1 > 200 {
                    return None;
                }
                w.put("{")?;
                for (i, (k, x)) in m.iter().enumerate() {
                    if i > 0 {
                        w.put(",")?;
                    }
                    w.string(k)?;
                    w.put(":")?;
                    go(x, w, depth + 1)?;
                }
                w.put("}")
            }
        }
    }
    let mut w = W {
        out: String::new(),
        count: 0,
    };
    go(d, &mut w, 0)?;
    Some(w.out)
}

/// The JSON form (5.5).
pub fn json(ds: &[View<'_>]) -> String {
    let mut out = String::new();
    for d in ds {
        out.push('{');
        if let Some(c) = d.class {
            out.push_str("\"class\":");
            json_str(&mut out, c);
            out.push(',');
        }
        out.push_str("\"code\":");
        json_str(&mut out, d.code);
        if d.code == "failed" {
            out.push_str(",\"data\":");
            match d.data.and_then(data_json) {
                Some(j) => out.push_str(&j),
                None => out.push_str("null"),
            }
        }
        out.push_str(",\"message\":");
        json_str(&mut out, d.message);
        if d.code == "not-conforming" {
            out.push_str(",\"path\":[");
            for (i, step) in d.path.unwrap_or_default().iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                match step {
                    PathStep::Key(k) => json_str(&mut out, k),
                    PathStep::Index(n) => out.push_str(&n.to_string()),
                }
            }
            out.push(']');
        }
        out.push_str(",\"phase\":");
        json_str(&mut out, d.phase);
        if let Some(s) = d.span {
            out.push_str(&format!(",\"span\":[{},{},{},{}]", s[0], s[1], s[2], s[3]));
        }
        if d.phase == "run" {
            out.push_str(",\"trace\":[");
            for (i, (s, name, index)) in d.trace.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                out.push_str("{\"index\":");
                match index {
                    Some(n) => out.push_str(&n.to_string()),
                    None => out.push_str("null"),
                }
                out.push_str(",\"name\":");
                json_str(&mut out, name);
                out.push_str(&format!(
                    ",\"span\":[{},{},{},{}]}}",
                    s[0], s[1], s[2], s[3]
                ));
            }
            out.push(']');
        }
        out.push_str("}\n");
    }
    out
}

/// The one-line form of diagnostics a stage gave.
pub fn render_line(ds: &[Diagnostic]) -> String {
    line(&ds.iter().map(View::of).collect::<Vec<_>>())
}

/// The detail form, with the text that was parsed when there is one.
pub fn render_detail(ds: &[Diagnostic], source: Option<&str>) -> String {
    detail(&ds.iter().map(View::of).collect::<Vec<_>>(), source)
}

/// The JSON form.
pub fn render_json(ds: &[Diagnostic]) -> String {
    json(&ds.iter().map(View::of).collect::<Vec<_>>())
}
