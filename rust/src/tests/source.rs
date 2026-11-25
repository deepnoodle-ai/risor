//! The size of a source text (`01-language.md` 2.1 items 4 to 6,
//! `03-host.md` 3.6.1 items 5 and 6): the exact default limit, a host's
//! lower limit on each call that takes source text, and a limit out of
//! range. The suite states the rule with small limits; these tests state it
//! at the default, which no case file holds.

use crate::syntax::parser::within;
use crate::{
    format, format_template, format_template_with, format_with, load_value, load_with, parse,
    parse_template, parse_template_with, parse_with, render_detail, render_json, render_line, Env,
    Error, Expected, LoadOptions, ParseOptions, Phase, Pos, Span, SOURCE_LIMIT,
};
use std::collections::BTreeMap;

const LIMIT: usize = SOURCE_LIMIT;

fn opts(source_limit: usize) -> ParseOptions {
    ParseOptions { source_limit }
}

/// `1` and spaces: a program of exactly `n` code points.
fn spaces(n: usize) -> String {
    format!("1{}", " ".repeat(n - 1))
}

/// A string literal of exactly `n` code points, each inside it of four
/// bytes in UTF-8.
fn wide(n: usize) -> String {
    format!("\"{}\"", "\u{1F600}".repeat(n - 2))
}

/// The error is `source-too-large` for the limit, with no span.
fn too_large(e: &Error, limit: usize) {
    assert_eq!(e.phase, Phase::Parse);
    assert_eq!(e.diagnostics.len(), 1);
    let d = e.first();
    assert_eq!(
        (d.phase, d.code, d.span),
        (Phase::Parse, "source-too-large", None)
    );
    assert_eq!(
        d.message,
        format!("the source text is longer than the limit of {limit} code points")
    );
}

fn bad_limit(e: &Error) {
    assert_eq!(e.phase, Phase::Declare);
    assert_eq!(e.first().code, "bad-limit");
    assert_eq!(e.first().span, None);
}

#[test]
fn the_default_limit_is_1_048_576_code_points() {
    assert_eq!(LIMIT, 1_048_576);
    assert_eq!(ParseOptions::default().source_limit, LIMIT);
    assert_eq!(LoadOptions::default().source_limit, LIMIT);
    assert!(parse(&spaces(LIMIT)).is_ok());
    too_large(&parse(&spaces(LIMIT + 1)).unwrap_err(), LIMIT);
}

/// The count is of code points, not of bytes: a text of the limit whose
/// every code point is four bytes is about 4 MiB, and it parses.
#[test]
fn the_limit_counts_code_points_not_bytes() {
    let text = wide(LIMIT);
    assert_eq!(text.len(), 4 * (LIMIT - 2) + 2);
    let x = parse(&text).unwrap();
    assert_eq!(x.text().chars().count(), LIMIT);
    too_large(&parse(&wide(LIMIT + 1)).unwrap_err(), LIMIT);
    // Two bytes a code point, between the limit and four times it.
    let e = format!("\"{}\"", "\u{e9}".repeat(LIMIT - 2));
    assert!(parse(&e).is_ok());
    let e = format!("\"{}\"", "\u{e9}".repeat(LIMIT - 1));
    too_large(&parse(&e).unwrap_err(), LIMIT);
}

/// A U+FEFF at the start counts, though it is not part of the program.
#[test]
fn a_leading_byte_order_mark_counts() {
    let text = format!("\u{feff}{}", spaces(LIMIT - 1));
    assert!(parse(&text).is_ok());
    let text = format!("\u{feff}{}", spaces(LIMIT));
    too_large(&parse(&text).unwrap_err(), LIMIT);
}

/// Nothing of the text is read but its length (2.1 item 5).
#[test]
fn the_limit_is_found_before_the_text_is_lexed() {
    too_large(&parse(&"@".repeat(LIMIT + 1)).unwrap_err(), LIMIT);
    too_large(
        &parse(&format!("\"{}", "a".repeat(LIMIT))).unwrap_err(),
        LIMIT,
    );
    too_large(&parse_with("@@", opts(1)).unwrap_err(), 1);
}

#[test]
fn the_byte_length_decides_most_texts() {
    assert!(within("", 0));
    assert!(!within("a", 0));
    assert!(within("abc", 3));
    assert!(!within("abcd", 3));
    // 2 bytes, 1 code point: counted.
    assert!(within("\u{e9}", 1));
    // 8 bytes is more than 4 a code point: over without a count.
    assert!(!within("\u{1F600}\u{1F600}", 1));
    assert!(within("\u{1F600}\u{1F600}", 2));
}

#[test]
fn templates_are_held_to_the_limit() {
    let text = format!("a{}", " ".repeat(LIMIT - 1));
    assert!(parse_template(&text).is_ok());
    let text = format!("a{}", " ".repeat(LIMIT));
    too_large(&parse_template(&text).unwrap_err(), LIMIT);
    assert!(parse_template_with("a${{ 1 }}", opts(9)).is_ok());
    too_large(&parse_template_with("ab${{ 1 }}", opts(9)).unwrap_err(), 9);
    // The whole text is refused before a part is read.
    too_large(&parse_template_with("${{ @@@ }}", opts(9)).unwrap_err(), 9);
}

#[test]
fn the_formatter_is_held_to_the_limit() {
    too_large(&format(&spaces(LIMIT + 1)).unwrap_err(), LIMIT);
    let t = format!("a{}", " ".repeat(LIMIT));
    too_large(&format_template(&t).unwrap_err(), LIMIT);
    assert_eq!(format_with("1+2", opts(3)).unwrap(), "1 + 2\n");
    too_large(&format_with("1 + 2", opts(4)).unwrap_err(), 4);
    too_large(&format_template_with("a ${{ 1 }}", opts(9)).unwrap_err(), 9);
}

/// Completion over the limit gives an empty list at the position, and hover
/// gives none (F12 3.1 item 5).
#[test]
fn completion_and_hover_give_nothing_over_the_limit() {
    let env = Env::builder()
        .var_record("{user: {name: string}}")
        .build()
        .unwrap();
    let none = &Expected::None;
    let at = |column| Pos { line: 1, column };
    let empty = |column| Span {
        start: at(column),
        end: at(column),
    };
    // Over the default limit.
    let text = format!("use{}", " ".repeat(LIMIT - 2));
    let c = env.complete(&text, at(4), none);
    assert!(c.items.is_empty());
    assert_eq!(c.span, empty(4));
    assert!(env.hover(&text, at(1), none).is_none());
    // Within it, the same word is helped.
    let text = format!("use{}", " ".repeat(LIMIT - 3));
    assert_eq!(env.complete(&text, at(4), none).items.len(), 1);
    assert!(env
        .hover(&format!("user{}", " ".repeat(LIMIT - 4)), at(1), none)
        .is_some());
    // A host's lower limit.
    let c = env.complete_with("use", at(4), none, opts(3));
    assert_eq!(c.items.len(), 1);
    let c = env.complete_with("use ", at(4), none, opts(3));
    assert!(c.items.is_empty());
    assert_eq!(c.span, empty(4));
    assert!(env.hover_with("user", at(2), none, opts(4)).is_some());
    assert!(env.hover_with("user.name", at(2), none, opts(4)).is_none());
}

#[test]
fn a_load_is_held_to_the_limit() {
    let env = Env::builder().build().unwrap();
    let vars: BTreeMap<String, crate::Data> = BTreeMap::new();
    let load = |text: &str, source_limit| {
        load_value(
            &env,
            text,
            &vars,
            LoadOptions {
                source_limit,
                ..LoadOptions::default()
            },
        )
    };
    assert!(load("{a: 1}", 6).is_ok());
    let e = load("{a: 12}", 6).unwrap_err();
    assert_eq!((e.step, e.diagnostics[0].code), (1, "source-too-large"));
    assert_eq!(e.diagnostics[0].span, None);
    let e = load_value(&env, &spaces(LIMIT + 1), &vars, LoadOptions::default()).unwrap_err();
    assert_eq!((e.step, e.diagnostics[0].code), (1, "source-too-large"));
    // A host type.
    let got = load_with::<BTreeMap<String, u8>>(&env, "{a: 1}", &vars, 1, opts(6)).unwrap();
    assert_eq!(got.get("a"), Some(&1));
    let e = load_with::<BTreeMap<String, u8>>(&env, "{a: 12}", &vars, 1, opts(6)).unwrap_err();
    assert_eq!((e.step, e.diagnostics[0].code), (1, "source-too-large"));
}

/// A source limit above 1,048,576 is `bad-limit`, of phase `declare`;
/// completion and hover give their empty result (`03-host.md` 3.6.1 item 5).
/// 0 to 1,048,576 is in range.
#[test]
fn a_limit_out_of_range_is_bad_limit() {
    let over = opts(LIMIT + 1);
    bad_limit(&parse_with("1", over).unwrap_err());
    bad_limit(&parse_template_with("a", over).unwrap_err());
    bad_limit(&format_with("1", over).unwrap_err());
    bad_limit(&format_template_with("a", over).unwrap_err());
    let env = Env::builder().var("user", "int").build().unwrap();
    let at = Pos { line: 1, column: 4 };
    let c = env.complete_with("use", at, &Expected::None, over);
    assert!(c.items.is_empty());
    assert_eq!(c.span, Span { start: at, end: at });
    let at = Pos { line: 1, column: 1 };
    assert!(env.hover_with("user", at, &Expected::None, over).is_none());
    let vars: BTreeMap<String, crate::Data> = BTreeMap::new();
    let opts_over = LoadOptions {
        source_limit: LIMIT + 1,
        ..LoadOptions::default()
    };
    let e = load_value(&env, "1", &vars, opts_over).unwrap_err();
    assert_eq!((e.step, e.diagnostics[0].code), (0, "bad-limit"));
    let e = load_with::<u8>(&env, "1", &vars, 1, over).unwrap_err();
    assert_eq!((e.step, e.diagnostics[0].code), (0, "bad-limit"));
    // The bounds are in range.
    assert!(parse_with(&spaces(LIMIT), opts(LIMIT)).is_ok());
    too_large(&parse_with("1", opts(0)).unwrap_err(), 0);
    assert_eq!(
        parse_with("", opts(0)).unwrap_err().first().code,
        "unexpected-token"
    );
}

/// A diagnostic with no span renders in every form (`f08-errors.md` 5).
#[test]
fn a_diagnostic_with_no_span_renders() {
    let e = parse_with("1 + 23", opts(5)).unwrap_err();
    let line = render_line(&e.diagnostics);
    assert!(line.contains("source-too-large"), "{line}");
    let detail = render_detail(&e.diagnostics, Some("1 + 23"));
    assert!(detail.contains("source-too-large"), "{detail}");
    assert!(render_detail(&e.diagnostics, None).contains("source-too-large"));
    let json = render_json(&e.diagnostics);
    assert!(json.contains("\"source-too-large\""), "{json}");
    assert!(!json.contains("\"span\""), "{json}");
    assert!(e.to_string().contains("source-too-large"));
}
