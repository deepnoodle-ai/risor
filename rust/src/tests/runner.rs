//! The case-file runner: validation (`03-host.md` 3.10.10) and verdicts.

use crate::json;
use crate::testing::{load_doc, run_case, Verdict};
use std::path::Path;

fn verdicts(text: &str) -> Result<Vec<Verdict>, String> {
    let doc = json::parse(text)?;
    let cases = load_doc(&doc, Path::new("."))?;
    Ok(cases.iter().map(run_case).collect())
}

fn invalid(text: &str) {
    assert!(verdicts(text).is_err(), "should be invalid: {text}");
}

/// The example file of 3.10.12.
const EXAMPLE: &str = r#"{
  "env": "{order: {total: float, coupon: string | null}, d: int}",
  "functions": {
    "rate": {"type": "(code: string) => float !",
             "calls": [{"args": ["SAVE10"], "result": 0.1},
                       {"error": "unknown coupon"}]}
  },
  "returns": "float",
  "cases": [
    {"name": "a known coupon gives its rate",
     "source": "order.total * (1 - rate(order.coupon ?? \"none\"))",
     "vars": {"order": {"total": 200, "coupon": "SAVE10"}, "d": 1},
     "expect": {"type": "float", "value": 180}},
    {"name": "an unknown coupon fails the run with the stub's text",
     "source": "order.total * (1 - rate(order.coupon ?? \"none\"))",
     "vars": {"order": {"total": 200, "coupon": "NOPE"}, "d": 1},
     "expect": {"phase": "run",
                "diagnostics": [{"code": "function-error", "class": "host",
                                 "message": "unknown coupon"}]}},
    {"name": "a caught error is charged for its value and its message",
     "source": "try { 1 / d } catch (err) { err.message }",
     "returns": "float | string",
     "vars": {"d": 0},
     "expect": {"type": "float | string",
                "value": "the divisor of / is zero",
                "steps": 3, "allocated": 10}},
    {"name": "a misspelled field is a check error",
     "source": "order.totl * 2",
     "expect": {"phase": "check",
                "diagnostics": [{"code": "no-field", "span": [1, 7, 1, 11]}]}},
    {"name": "a reserved word cannot be bound",
     "source": "const match = 1; match",
     "returns": "int",
     "expect": {"phase": "parse",
                "diagnostics": [{"code": "reserved-word",
                                 "span": [1, 7, 1, 12]}]}}
  ]
}"#;

#[test]
fn the_example_of_3_10_12() {
    let v = verdicts(EXAMPLE).unwrap();
    assert_eq!(v.len(), 5);
    // With the checker every case of the example is judged.
    assert_eq!(v, vec![Verdict::Pass; 5]);
}

#[test]
fn parse_verdicts() {
    let v = verdicts(
        r#"[
        {"name": "a", "source": "1 +", "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token", "span": [1, 4, 1, 4]}]}},
        {"name": "b", "source": "1 +", "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token", "span": [1, 1, 1, 2]}]}},
        {"name": "c", "source": "1 + 2", "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token"}]}},
        {"name": "d", "source": "1 +", "expect": {}},
        {"name": "e", "source": "1 + 2", "expect": {}},
        {"name": "f", "source": "1 + 2", "expect": {"value": 3}},
        {"name": "g", "source": "1 +", "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token"}]}, "skip": "not now"},
        {"name": "h", "source": "1 +", "expect": {"phase": "run", "diagnostics": [{"code": "wrong-kind"}]}}
    ]"#,
    )
    .unwrap();
    assert_eq!(v[0], Verdict::Pass);
    assert!(matches!(&v[1], Verdict::Fail(d) if d[0].name == "diagnostics"));
    assert!(matches!(&v[2], Verdict::Fail(d) if d[0].name == "outcome"));
    assert!(matches!(&v[3], Verdict::Fail(d) if d[0].name == "outcome"));
    assert_eq!(v[4], Verdict::Pass);
    assert_eq!(v[5], Verdict::Pass);
    assert!(matches!(v[6], Verdict::Skip(_)));
    assert!(matches!(&v[7], Verdict::Fail(d) if d[0].name == "outcome"));
}

#[test]
fn source_from_a_file_and_the_header_rule() {
    let v = verdicts(
        r#"{"source": "1 +", "cases": [
        {"name": "a", "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token"}]}},
        {"name": "b", "source": "2", "expect": {}},
        {"name": "c", "precondition": false, "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token"}]}}
    ]}"#,
    )
    .unwrap();
    assert_eq!(v, vec![Verdict::Pass, Verdict::Pass, Verdict::Pass]);
    let dir = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests");
    let doc = json::parse(r#"[{"name": "a", "file": "data/program.risor", "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token", "span": [2, 3, 2, 4]}]}}]"#).unwrap();
    let cases = load_doc(&doc, &dir).unwrap();
    assert_eq!(run_case(&cases[0]), Verdict::Pass);
    let doc = json::parse(r#"[{"name": "a", "file": "data/none.risor", "expect": {}}]"#).unwrap();
    assert!(load_doc(&doc, &dir).is_err());
}

#[test]
fn a_declare_failure_of_the_grammar() {
    let v = verdicts(
        r#"[
        {"name": "a", "env": "{x: number}", "source": "1", "expect": {}},
        {"name": "b", "functions": {"f": "(g: (x: int) => int) => int"}, "source": "1", "expect": {}},
        {"name": "c", "returns": "condition", "source": "1", "expect": {}},
        {"name": "d", "reject": ["bogus"], "source": "1", "expect": {}}
    ]"#,
    )
    .unwrap();
    assert!(matches!(&v[0], Verdict::Fail(d) if d[0].found.contains("bad-type")));
    assert!(matches!(&v[1], Verdict::Fail(d) if d[0].found.contains("bad-signature")));
    assert_eq!(v[2], Verdict::Pass);
    assert!(matches!(&v[3], Verdict::Fail(d) if d[0].found.contains("bad-class")));
}

#[test]
fn invalid_files() {
    let ok_case = r#""name": "a", "source": "1", "expect": {}"#;
    // Sanity: the base case is valid.
    assert!(verdicts(&format!("[{{{ok_case}}}]")).is_ok());
    invalid("[");
    invalid("5");
    invalid(r#"{"source": "1"}"#);
    invalid(&format!(r#"[{{{ok_case}, "name": "b"}}]"#));
    invalid(&format!(r#"[{{{ok_case}, "bogus": 1}}]"#));
    invalid(&format!(r#"{{"name": "x", "cases": [{{{ok_case}}}]}}"#));
    invalid(&format!(r#"{{"skip": "x", "cases": [{{{ok_case}}}]}}"#));
    invalid(&format!(r#"[{{{ok_case}}}, {{{ok_case}}}]"#));
    invalid(r#"[{"source": "1", "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": "1"}]"#);
    invalid(r#"[{"name": "a", "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "file": "x", "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": 1, "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "vars": {"x": 1}, "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "env": "{y: int}", "vars": {"x": 1}, "expect": {}}]"#);
    invalid(
        r#"[{"name": "a", "source": "1", "functions": {"f": {"type": "() => int", "calls": [{"result": 1, "error": "e"}]}}, "expect": {}}]"#,
    );
    invalid(
        r#"[{"name": "a", "source": "1", "functions": {"f": {"type": "() => int", "calls": [{}]}}, "expect": {}}]"#,
    );
    invalid(r#"[{"name": "a", "source": "1", "functions": {"f": {"calls": []}}, "expect": {}}]"#);
    invalid(
        r#"[{"name": "a", "source": "1", "functions": {"f": {"type": "() => int", "bogus": 1}}, "expect": {}}]"#,
    );
    invalid(
        r#"[{"name": "a", "source": "1", "expect": {"phase": "run", "value": 1, "diagnostics": [{"code": "x"}]}}]"#,
    );
    invalid(
        r#"[{"name": "a", "source": "1", "expect": {"phase": "parse", "steps": 1, "diagnostics": [{"code": "x"}]}}]"#,
    );
    invalid(
        r#"[{"name": "a", "source": "1", "expect": {"phase": "run", "steps": 1, "diagnostics": [{"code": "steps-exceeded"}]}}]"#,
    );
    invalid(r#"[{"name": "a", "source": "1", "expect": {"phase": "run", "diagnostics": []}}]"#);
    invalid(
        r#"[{"name": "a", "source": "1", "expect": {"phase": "run", "diagnostics": [{"code": "Bad"}]}}]"#,
    );
    invalid(
        r#"[{"name": "a", "source": "1", "expect": {"phase": "run", "diagnostics": [{"code": "x", "span": [1, 0, 1, 2]}]}}]"#,
    );
    invalid(
        r#"[{"name": "a", "source": "1", "expect": {"phase": "run", "diagnostics": [{"code": "x", "bogus": 1}]}}]"#,
    );
    invalid(r#"[{"name": "a", "source": "1", "expect": {"value": 1e400}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "expect": {"diagnostics": []}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "limits": {"depth": 0}, "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "limits": {"steps": 1.5}, "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "limits": {"time": 1}, "expect": {}}]"#);
    invalid(r#"[{"name": "a", "source": "1", "version": "3", "expect": {}}]"#);
    invalid(
        r#"[{"name": "a", "source": "1", "methods": {"time.add": "() => int"}, "expect": {}}]"#,
    );
    invalid(r#"[{"name": "a", "source": "1", "precondition": "no", "expect": {}}]"#);
    invalid(
        r#"[{"name": "a", "source": "1", "expect": {"sites": [{"class": "range", "span": [1, 1, 1, 2]}]}}]"#,
    );
}

/// `03-host.md` 3.10.3: the case field `check`, and the files it makes invalid.
#[test]
fn the_check_field() {
    let v = verdicts(
        r#"[{"name": "a", "check": false, "env": "{a: any}", "vars": {"a": 1},
             "source": "a < 2 < 3",
             "expect": {"phase": "run", "diagnostics": [{"code": "wrong-kind", "class": "dynamic"}]}}]"#,
    )
    .unwrap();
    assert_eq!(v, vec![Verdict::Pass]);
    // Checked, the same program is a check failure.
    let v = verdicts(
        r#"[{"name": "a", "env": "{a: any}", "vars": {"a": 1}, "source": "a < 2 < 3",
             "expect": {"phase": "run", "diagnostics": [{"code": "wrong-kind"}]}}]"#,
    )
    .unwrap();
    assert!(matches!(&v[0], Verdict::Fail(d) if d[0].name == "outcome"));
    invalid(r#"[{"name": "a", "check": false, "source": "1", "expect": {"type": "int"}}]"#);
    invalid(r#"[{"name": "a", "check": false, "source": "1", "expect": {"sites": []}}]"#);
    invalid(
        r#"[{"name": "a", "check": false, "source": "1", "expect": {"phase": "check", "diagnostics": [{"code": "kind"}]}}]"#,
    );
    invalid(r#"[{"name": "a", "check": "no", "source": "1", "expect": {}}]"#);
    assert!(verdicts(r#"{"check": false, "cases": [{"name": "a", "source": "1", "expect": {"phase": "check", "diagnostics": [{"code": "duplicate-key"}]}}]}"#).is_ok());
}

/// `f09-testing.md` 4.3: in update mode a skipped case keeps `skip` and is
/// not written, also a format, completion or hover case with no `expect`.
/// Such a case with no `skip` differs in `outcome`.
#[test]
fn update_keeps_a_skipped_tool_case() {
    let text = r#"[
  {"name": "a", "format": "1+2", "skip": "not now"},
  {"name": "b", "complete": "le‸", "skip": "not now"},
  {"name": "c", "hover": "len‸([1])", "skip": "not now"},
  {"name": "d", "format": "1+2"}
]"#;
    let cases = crate::testing::load_doc_lax(&json::parse(text).unwrap(), Path::new(".")).unwrap();
    let (results, new) = crate::testing::update::update_text(text, &cases);
    for (name, v) in &results[..3] {
        assert_eq!(v, &Verdict::Skip("not now".into()), "{name}");
    }
    assert!(matches!(&results[3].1, Verdict::Fail(d) if d[0].name == "outcome"));
    assert_eq!(new, None);
}

/// The static codes by the directory of a case file (`03-host.md` 3.10.13):
/// `untagged-union` and `check-limit` in `suite/conversion/`, `suite/unions/`
/// and `suite/entry-points/` (F4 6.1 item 4, F5 11.2 item 3), and
/// `name-clash` only in `suite/entry-points/` (F16 3.1 item 11).
#[test]
fn static_codes_by_directory() {
    use crate::testing::case::{is_static, Statics};
    let load = |dir: &str, text: &str| load_doc(&json::parse(text).unwrap(), Path::new(dir));
    for (dir, want) in [
        ("suite/core-run", Statics::Core),
        ("suite/conversion", Statics::ProgramTypes),
        ("suite/unions", Statics::ProgramTypes),
        ("suite/entry-points", Statics::EntryPoints),
        ("suite/recursion", Statics::Recursion),
    ] {
        assert_eq!(Statics::of_dir(Path::new(dir)), want, "{dir}");
    }
    for (code, core, types, entry, rec) in [
        ("arity", true, true, true, true),
        ("untagged-union", false, true, true, true),
        ("check-limit", false, true, true, true),
        ("name-clash", false, false, true, true),
        ("circular-type", false, false, false, true),
        ("kind", false, false, false, false),
    ] {
        assert_eq!(is_static(code, Statics::Core), core, "{code}");
        assert_eq!(is_static(code, Statics::ProgramTypes), types, "{code}");
        assert_eq!(is_static(code, Statics::EntryPoints), entry, "{code}");
        assert_eq!(is_static(code, Statics::Recursion), rec, "{code}");
    }

    // Validation, 3.10.10 item 17: a case with "check": false expects
    // static codes only.
    let unchecked = |code: &str| {
        format!(
            r#"[{{"name": "a", "check": false, "source": "1", "expect": {{"phase": "check", "diagnostics": [{{"code": "{code}"}}]}}}}]"#
        )
    };
    for dir in ["suite/conversion", "suite/unions", "suite/entry-points"] {
        assert!(load(dir, &unchecked("untagged-union")).is_ok(), "{dir}");
    }
    assert!(load("suite/core-run", &unchecked("untagged-union")).is_err());
    assert!(load("suite/entry-points", &unchecked("name-clash")).is_ok());
    assert!(load("suite/conversion", &unchecked("name-clash")).is_err());
    assert!(load("suite/unions", &unchecked("name-clash")).is_err());

    // Classification, 3.10.8: a case that expects a code that is not
    // static needs the checker, so a runner with none skips it.
    let clash = r#"[{"name": "a", "types": [{"name": "Order", "type": "{id: string}"}],
        "source": "type Order = int; 1",
        "expect": {"phase": "check", "diagnostics": [{"code": "name-clash", "span": [1, 6, 1, 11]}]}}]"#;
    let entry = load("suite/entry-points", clash).unwrap();
    assert_eq!(crate::testing::run_case_unchecked(&entry[0]), Verdict::Pass);
    assert_eq!(run_case(&entry[0]), Verdict::Pass);
    let conversion = load("suite/conversion", clash).unwrap();
    assert!(matches!(
        crate::testing::run_case_unchecked(&conversion[0]),
        Verdict::Skip(_)
    ));
    assert_eq!(run_case(&conversion[0]), Verdict::Pass);
}
