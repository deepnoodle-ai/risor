//! The messages of `parse` diagnostics (`f08-errors.md` 3.2 to 3.4).

use crate::messages::{build, char_param};
use crate::source::{Range, Source};

/// The rendering of a parameter of kind `token` (`f08-errors.md` 3.2): the
/// source text at the span, as visible text; the empty span at the end of
/// input as `end of input`; a span from `not` to the end of `in` as `not in`.
pub(crate) fn token_text(src: &Source, span: Range) -> String {
    if span.start >= span.end {
        return "end of input".to_string();
    }
    let cs = &src.chars()[span.start as usize..span.end as usize];
    let word = |c: char| c.is_ascii_alphanumeric() || c == '_';
    let n = cs.len();
    if n >= 6
        && cs[..3] == ['n', 'o', 't']
        && cs[n - 2..] == ['i', 'n']
        && !word(cs[3])
        && !word(cs[n - 3])
    {
        return "not in".to_string();
    }
    src.param_text(span)
}

/// The template of a code that the text at the span decides (3.4), when
/// the place that reports it does not name one.
fn auto_template(src: &Source, code: &str, span: Range) -> &'static str {
    let text = src.slice(span);
    match code {
        "unterminated-string" => {
            if text.starts_with('`') {
                "template"
            } else if text.starts_with("\"\"\"") || text.starts_with("r\"\"\"") {
                "text-block"
            } else if text.starts_with("${{") {
                "part"
            } else {
                "string"
            }
        }
        "bad-escape" => {
            let hex = |s: &str| !s.is_empty() && s.chars().all(|c| c.is_ascii_hexdigit());
            if let Some(rest) = text.strip_prefix("\\u") {
                if rest.len() == 4 && hex(rest) {
                    "surrogate"
                } else if rest.starts_with('{') && rest.ends_with('}') && rest.len() > 2 {
                    "range"
                } else {
                    "unicode"
                }
            } else {
                "unknown"
            }
        }
        "literal-range" => {
            let lower = text.to_ascii_lowercase();
            if lower.starts_with("0x") || !(lower.contains('.') || lower.contains('e')) {
                "whole"
            } else {
                "finite"
            }
        }
        "text-block-indent" => match text.chars().next() {
            Some('\t') => "tab",
            Some(' ') => "space",
            _ => "default",
        },
        "reserved-word" => "operand",
        _ => "default",
    }
}

/// The message of a `parse` diagnostic: its template, chosen by the place
/// that reports it or, when `template` is empty, by the text at the span;
/// `token` and `char` from the span; the other parameters as given.
pub(crate) fn parse_message(
    src: &Source,
    code: &str,
    template: &str,
    span: Range,
    extra: &[(&str, String)],
) -> String {
    let template = if template.is_empty() {
        auto_template(src, code, span)
    } else {
        template
    };
    let mut params: Vec<(&str, String)> = vec![("token", token_text(src, span))];
    if let Some(c) = src.chars().get(span.start as usize) {
        params.push(("char", char_param(*c)));
    }
    params.extend(extra.iter().cloned());
    build(code, template, &params)
}
