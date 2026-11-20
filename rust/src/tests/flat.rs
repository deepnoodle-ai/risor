//! What opens no level costs no stack (`01-language.md` 13.1 and 13.2 item
//! 6, `f04-conversion.md` 2.8, `f07-queries.md` 2.5).
//!
//! Each flat form is written 100,000 links long and goes through every
//! entry point of the crate: parsing, analysis, the formatter, the check,
//! a compile with and without the check, a run, completion, hover, a
//! template and a load. Each test runs on a thread with the stack that
//! `DESIGN.md` (Stack) states for the default depth limit in this build,
//! and no more: a pass that recursed once for each link would overflow it.

use super::host::stated_stack;
use crate::{
    format, format_template, json_text, load_value, parse, parse_template, CompileOptions, Data,
    Env, Expected, HostList, HostMap, LoadOptions, Pos, TemplateMode, Vars, View,
};
use std::collections::BTreeMap;

/// The links of each form.
const N: usize = 100_000;

/// Host data that a chain of any length reads: as a map, its key `a` is
/// the map itself, `l` is the list and `k` is 7; as a list, its item 0 is
/// the list itself, item 1 is 7 and item 2 is the map.
struct Ring;

impl HostMap for Ring {
    fn len(&self) -> usize {
        3
    }
    fn get(&self, key: &str) -> Option<View<'_>> {
        match key {
            "a" => Some(View::Map(self)),
            "k" => Some(View::Int(7)),
            "l" => Some(View::List(self)),
            _ => None,
        }
    }
    fn keys(&self) -> Vec<&str> {
        vec!["a", "k", "l"]
    }
}

impl HostList for Ring {
    fn len(&self) -> usize {
        3
    }
    fn get(&self, index: usize) -> Option<View<'_>> {
        match index {
            0 => Some(View::List(self)),
            1 => Some(View::Int(7)),
            2 => Some(View::Map(self)),
            _ => None,
        }
    }
}

/// The variables: `r` and `l` are the ring, the others are plain data.
struct Host(BTreeMap<String, Data>);

impl Vars for Host {
    fn get(&self, name: &str) -> Option<View<'_>> {
        match name {
            "r" => Some(View::Map(&Ring)),
            "l" => Some(View::List(&Ring)),
            _ => Vars::get(&self.0, name),
        }
    }
}

fn host() -> Host {
    Host(
        [
            ("y", Data::Str("a".into())),
            ("n", Data::Int(1)),
            ("b", Data::Bool(false)),
            ("o", Data::Null),
            ("m", Data::Map(BTreeMap::new())),
            ("xs", Data::List(Vec::new())),
        ]
        .into_iter()
        .map(|(k, v)| (k.to_string(), v))
        .collect(),
    )
}

fn env() -> Env {
    Env::builder()
        .var("y", "string")
        .var("n", "int")
        .var("b", "bool")
        .var("o", "int | null")
        .var("r", "any")
        .var("l", "any")
        .var("m", "map<int>")
        .var("xs", "int[]")
        .function("f", "(x: string) => string", |_, _| {
            Ok(Data::Str("a".into()))
        })
        .build()
        .unwrap()
}

/// A flat form and what it gives.
struct Form {
    /// The source with `n` links.
    src: fn(usize) -> String,
    /// The type the check gives, or the code of its first diagnostic.
    ty: Result<&'static str, &'static str>,
    /// The value of the run as JSON, or the code of its failure.
    value: Result<String, &'static str>,
    steps: u64,
    units: u64,
    /// The links of the text the formatter is given. Fewer than `N` only
    /// where the formatted text grows as the square of the links.
    format: usize,
    /// Whether the form is an expression, which a template part holds.
    expression: bool,
    /// Whether a compile without the check fails too, with the code of
    /// `ty`: a static rule (F4 6.1).
    static_rule: bool,
}

impl Form {
    fn new(
        src: fn(usize) -> String,
        ty: &'static str,
        value: &str,
        steps: u64,
        units: u64,
    ) -> Form {
        Form {
            src,
            ty: Ok(ty),
            value: Ok(value.to_string()),
            steps,
            units,
            format: N,
            expression: true,
            static_rule: false,
        }
    }

    /// A form that is not an expression: a program with `const` or `type`
    /// lines.
    fn program(
        src: fn(usize) -> String,
        ty: &'static str,
        value: &str,
        steps: u64,
        units: u64,
    ) -> Form {
        Form {
            expression: false,
            ..Form::new(src, ty, value, steps, units)
        }
    }

    /// A form whose check fails at a limit that `05-types.md` 20 states;
    /// a compile without the check runs it to `value`.
    fn over_the_limit(src: fn(usize) -> String, value: &str, steps: u64, units: u64) -> Form {
        Form {
            ty: Err("check-limit"),
            ..Form::new(src, "", value, steps, units)
        }
    }
}

/// The parts of a form, joined: `n` of them, each from its index.
fn each(n: usize, sep: &str, part: impl Fn(usize) -> String) -> String {
    (0..n).map(part).collect::<Vec<_>>().join(sep)
}

/// The position after the last code point of a text.
fn end(text: &str) -> Pos {
    let line = text.matches('\n').count() as u32 + 1;
    let last = text.rsplit('\n').next().unwrap_or("");
    Pos {
        line,
        column: last.chars().count() as u32 + 1,
    }
}

/// The value and the usage of a run, or the code of its failure.
fn run(
    env: &Env,
    src: &str,
    check: bool,
) -> Result<(Result<String, &'static str>, u64, u64), String> {
    let x = parse(src).map_err(|e| e.first().code.to_string())?;
    let options = CompileOptions {
        check,
        expected: Expected::None,
    };
    let p = env
        .compile(&x, options)
        .map_err(|e| e.first().code.to_string())?;
    let vars = host();
    let (r, usage) = p.measure(&vars);
    let value = match &r {
        Ok(v) => Ok(json_text(v).unwrap()),
        Err(e) => Err(e.first().code),
    };
    Ok((value, usage.steps, usage.allocated))
}

/// One form through every entry point.
fn through_every_entry_point(form: &Form) {
    let env = env();
    let src = (form.src)(N);
    let name: String = src.chars().take(40).collect();

    // Parsing and analysis.
    let x = parse(&src).unwrap_or_else(|e| panic!("{name}: {e}"));
    let analysis = x.analyze();
    assert!(
        analysis.refs.len() + analysis.calls.len() > 0
            || analysis.literal.is_some()
            || !form.expression,
        "{name}"
    );

    // The check, and a compile with it.
    let checked = env.check(&x, &Expected::None);
    match form.ty {
        Ok(t) => {
            let r = checked.unwrap_or_else(|e| panic!("{name}: {e}"));
            assert_eq!(r.ty.to_string(), t, "{name}");
            let want = (form.value.clone(), form.steps, form.units);
            assert_eq!(run(&env, &src, true), Ok(want), "{name}");
        }
        Err(code) => {
            assert_eq!(checked.unwrap_err().first().code, code, "{name}");
            assert_eq!(run(&env, &src, true), Err(code.to_string()), "{name}");
        }
    }

    // A compile without the check, and its run.
    let want = match (form.static_rule, form.ty) {
        (true, Err(code)) => Err(code.to_string()),
        _ => Ok((form.value.clone(), form.steps, form.units)),
    };
    assert_eq!(run(&env, &src, false), want, "{name}");

    // The formatter: its text parses.
    let short = (form.src)(form.format);
    let text = format(&short).unwrap_or_else(|e| panic!("{name}: {e}"));
    assert!(parse(&text).is_ok(), "{name}");

    // Completion after the last link and in the middle of the text; hover
    // on the first token and on the last.
    let dot = format!("{src}.");
    env.complete(&dot, end(&dot), &Expected::None);
    let half: String = src.chars().take(src.chars().count() / 2).collect();
    env.complete(&src, end(&half), &Expected::None);
    env.hover(&src, Pos { line: 1, column: 1 }, &Expected::None);
    let last = end(&src);
    let before = Pos {
        column: last.column.saturating_sub(2).max(1),
        ..last
    };
    env.hover(&src, before, &Expected::None);

    // A template with the form in a part, and its formatter.
    if form.expression {
        let text = format!("a ${{{{ {src} }}}} b");
        let t = parse_template(&text).unwrap_or_else(|e| panic!("{name}: {e}"));
        assert_eq!(
            t.analyze(TemplateMode::String).refs.len(),
            analysis.refs.len()
        );
        let whole = env.check(&t.program(TemplateMode::String), &Expected::None);
        match form.ty {
            Ok(_) => assert_eq!(whole.unwrap().ty.to_string(), "string", "{name}"),
            Err(code) => assert_eq!(whole.unwrap_err().first().code, code, "{name}"),
        }
        let text = format!("a ${{{{ {short} }}}} b");
        assert!(format_template(&text).is_ok(), "{name}");
    }

    // A load: the check, the run and the conformance of the value.
    let loaded = load_value(&env, &src, &host(), LoadOptions::default());
    match (&form.ty, &form.value) {
        (Ok(_), Ok(_)) => assert!(loaded.is_ok(), "{name}: {:?}", loaded.err()),
        (Ok(_), Err(code)) => {
            let e = loaded.unwrap_err();
            assert_eq!((e.step, e.diagnostics[0].code), (3, *code), "{name}");
        }
        (Err(code), _) => {
            let e = loaded.unwrap_err();
            assert_eq!((e.step, e.diagnostics[0].code), (2, *code), "{name}");
        }
    }
}

/// Run the forms on a thread with the stack stated for the default depth
/// limit. An overflow of that stack aborts the test process.
fn on_the_stated_stack(forms: fn() -> Vec<Form>) {
    std::thread::Builder::new()
        .stack_size(stated_stack(200))
        .spawn(move || {
            for form in forms() {
                through_every_entry_point(&form);
            }
        })
        .unwrap()
        .join()
        .unwrap();
}

/// One of each step of a chain: two method calls, a projection, a method
/// call with a lambda, an index, a member, an optional member, an index by
/// a string and an index into a string. A string goes in and a string
/// comes out.
const STEPS: &str = r#".trim().split("")[*].map(x => {a: {a: {a: x}}})[0].a?.a["a"][0]"#;

/// `as` and `is` open a level only for their type (F4 2.8).
#[test]
fn chains_of_as_and_is() {
    on_the_stated_stack(|| {
        vec![
            // The chain of issue 649, with the result it states.
            Form::new(
                |n| format!("y{}", " as any".repeat(n)),
                "any",
                "\"a\"",
                0,
                0,
            ),
            Form::new(
                |n| format!("y{}", " as string".repeat(n)),
                "string",
                "\"a\"",
                N as u64,
                0,
            ),
            // Every `is` after the first tests a bool: the check reports
            // each as a constant condition, and the run gives true.
            Form {
                ty: Err("always-truthy"),
                ..Form::new(
                    |n| format!("y is any{}", " is bool".repeat(n - 1)),
                    "bool",
                    "true",
                    N as u64 - 1,
                    0,
                )
            },
            Form::new(
                |n| format!("y{}", " as any is bool".repeat(n / 2)),
                "bool",
                "true",
                N as u64 / 2,
                0,
            ),
            // `as` between operators of its own level: the two kinds of
            // node alternate on one spine.
            Form::new(
                |n| format!("n in [1]{}", " as any in [true]".repeat(n / 2)),
                "bool",
                "true",
                N as u64 / 2 + 1,
                N as u64 + 2,
            ),
            // The last link fails, after every link before it conformed.
            Form {
                value: Err("not-conforming"),
                ..Form::new(
                    |n| format!("y{} as int", " as any".repeat(n - 1)),
                    "int",
                    "",
                    1,
                    0,
                )
            },
        ]
    });
}

/// A star's level ends with its projection (F7 2.5), so projections that
/// alternate with method calls are flat.
#[test]
fn projections_between_method_calls() {
    on_the_stated_stack(|| {
        vec![
            // The chain of issue 649, with the result it states.
            Form::new(
                |n| format!("[[y]]{}", "[*].map(x => x)".repeat(n)),
                "string[][]",
                "[[\"a\"]]",
                500_000,
                400_004,
            ),
            // Projections with a read.
            Form::new(
                |n| format!("[{{a: y}}]{}", "[*].a.map(x => {a: x})".repeat(n / 2)),
                "{a: string}[]",
                "[{\"a\":\"a\"}]",
                300_000,
                350_005,
            ),
        ]
    });
}

/// The receiver of a step opens no level (13.1): each kind of step alone.
#[test]
fn chains_of_one_step() {
    on_the_stated_stack(|| {
        let n = N as u64;
        vec![
            // The chain of issue 649, with the result it states.
            Form::new(
                |n| format!("y{}", ".trim()".repeat(n)),
                "string",
                "\"a\"",
                n,
                n,
            ),
            Form::new(
                |n| format!("f(y){}", ".trim()".repeat(n)),
                "string",
                "\"a\"",
                n + 1,
                n,
            ),
            Form::new(|n| format!("r{}.k", ".a".repeat(n)), "any", "7", n + 1, 0),
            Form::new(|n| format!("r{}?.k", "?.a".repeat(n)), "any", "7", n + 1, 0),
            Form::new(|n| format!("y{}", "[0]".repeat(n)), "string", "\"a\"", n, n),
            Form::new(|n| format!("l{}[1]", "[0]".repeat(n)), "any", "7", n + 1, 0),
            Form::new(
                |n| format!("r{}[\"k\"]", "[\"a\"]".repeat(n)),
                "any",
                "7",
                n + 1,
                0,
            ),
            Form::new(
                |n| format!("l{}?.[1]", "?.[0]".repeat(n)),
                "any",
                "7",
                n + 1,
                0,
            ),
        ]
    });
}

/// Every kind of step in one chain, and that chain under `as`, `is` and an
/// operator of their level.
#[test]
fn chains_of_every_step() {
    on_the_stated_stack(|| {
        vec![
            Form::new(
                |n| format!("y{}", STEPS.repeat(n / 9)),
                "string",
                "\"a\"",
                144_443,
                199_998,
            ),
            // The steps the ring can take, plain and optional.
            Form::new(
                |n| format!("r{}.k", r#".a?.a["a"]?.["a"].l[0]?.[0][2]"#.repeat(n / 8)),
                "any",
                "7",
                N as u64 + 1,
                0,
            ),
            Form::new(
                |n| {
                    format!(
                        "y{} as any{}",
                        STEPS.repeat(n / 18),
                        " is string in [true, false] as any is bool as any".repeat(n / 10)
                    )
                },
                "any",
                "true",
                112_214,
                129_990,
            ),
        ]
    });
}

/// The operators that 13.2 item 6 names, through the entry points that
/// `what_opens_no_level_costs_no_stack` (`syntax.rs`) and
/// `flat_programs_run` (`host.rs`) do not reach.
#[test]
fn operators_of_the_limit() {
    on_the_stated_stack(|| {
        let n = N as u64;
        vec![
            Form::new(
                |n| vec!["n"; n].join(" + "),
                "int",
                &N.to_string(),
                n - 1,
                0,
            ),
            Form::new(|n| vec!["b"; n].join(" && "), "bool", "false", 0, 0),
            Form::new(|n| vec!["o"; n].join(" ?? "), "int | null", "null", 0, 0),
            // The formatter writes each `else` branch of a `? :` that it
            // breaks one indent deeper than the branch before, so its text
            // for `n` branches has about 2 `n` squared code points: 20 GB
            // at 100,000 (measured: 2,010,002 at 1,000).
            Form {
                format: 1_000,
                ..Form::new(|n| format!("{}n", "b ? n : ".repeat(n)), "int", "1", 0, 0)
            },
        ]
    });
}

/// The `else if` parts and the `const` lines that 13.2 item 6 names, and
/// the items of a list, through the same entry points.
#[test]
fn lines_and_parts_of_the_limit() {
    on_the_stated_stack(|| {
        let n = N as u64;
        vec![
            Form::new(
                |n| {
                    format!(
                        "if (b) {{ 0 }}{} else {{ 2 }}",
                        " else if (b) { 1 }".repeat(n)
                    )
                },
                "int",
                "2",
                0,
                0,
            ),
            Form {
                expression: false,
                ..Form::new(
                    |n| {
                        let mut t = String::new();
                        for i in 0..n {
                            t.push_str(&format!("const a{i} = {i};\n"));
                        }
                        t + "a0"
                    },
                    "int",
                    "0",
                    0,
                    0,
                )
            },
            Form::new(
                |n| format!("[{}]", vec!["n"; n].join(", ")),
                "int[]",
                &format!("[{}]", vec!["1"; N].join(",")),
                0,
                n + 1,
            ),
        ]
    });
}

/// The arguments of a call and the entries of a map (13.1: "the next
/// item, entry, argument"). A record of 100,000 fields has size 100,001,
/// over the limit of `05-types.md` 20.7: the check gives `check-limit` at
/// the map, and a compile without the check runs it. One entry fewer is at
/// the limit, and checks.
#[test]
fn arguments_and_entries() {
    on_the_stated_stack(|| {
        let n = N as u64;
        // A literal map of 100,000 entries: its JSON, with its keys in order.
        let mut keys: Vec<String> = (0..N).map(|i| format!("k{i}")).collect();
        keys.sort();
        let map = format!(
            "{{{}}}",
            each(keys.len(), ",", |i| format!(
                "\"{}\":{}",
                keys[i],
                &keys[i][1..]
            ))
        );
        vec![
            Form::new(
                |n| format!("max({})", vec!["n"; n].join(", ")),
                "int",
                "1",
                n,
                0,
            ),
            Form::over_the_limit(
                |n| format!("len({{{}}})", each(n, ", ", |i| format!("k{i}: n"))),
                &N.to_string(),
                1,
                2 * n + 1,
            ),
            Form::new(
                |n| format!("len({{{}}})", each(n - 1, ", ", |i| format!("k{i}: n"))),
                "int",
                &(N - 1).to_string(),
                1,
                2 * n - 1,
            ),
            // A literal program: analysis gives its value (F2 3.7).
            Form::over_the_limit(
                |n| format!("{{{}}}", each(n, ", ", |i| format!("k{i}: {i}"))),
                &map,
                0,
                2 * n + 1,
            ),
        ]
    });
}

/// The arms of a `match`, and the literals of one arm.
#[test]
fn arms_and_their_literals() {
    on_the_stated_stack(|| {
        let n = N as u64;
        vec![
            Form::new(
                |n| {
                    let arms = each(n, ", ", |i| format!("\"k{i}\" => {i}"));
                    format!("match y {{ {arms}, _ => -1 }}")
                },
                "int",
                "-1",
                n + 2,
                0,
            ),
            Form::new(
                |n| {
                    let lits = each(n, " | ", |i| format!("\"k{i}\""));
                    format!("match y {{ {lits} => 1, _ => 2 }}")
                },
                "int",
                "2",
                2,
                0,
            ),
        ]
    });
}

/// The parameters of a lambda, and the fields and elements of a pattern
/// (13.1: "the next ... parameter, field, element"). Each default reads
/// the name bound before it, so every pass keeps the names of one long
/// scope.
#[test]
fn parameters_and_patterns() {
    on_the_stated_stack(|| {
        let n = N as u64;
        vec![
            Form::program(
                |n| {
                    let params = each(n - 1, "", |i| format!(", a{} = a{i}", i + 1));
                    format!("const g = (a0 = 0{params}) => a{};\ng()", n - 1)
                },
                "int",
                "0",
                2,
                0,
            ),
            Form::program(
                |n| {
                    let fields = each(n - 1, "", |i| format!(", k{} = k{i}", i + 1));
                    format!("const {{k0 = 0{fields}}} = m;\nk{}", n - 1)
                },
                "int",
                "0",
                n,
                0,
            ),
            Form::program(
                |n| {
                    let elems = each(n - 1, "", |i| format!(", a{} = a{i}", i + 1));
                    format!("const [a0 = 0{elems}] = xs;\na{}", n - 1)
                },
                "int",
                "0",
                n,
                0,
            ),
        ]
    });
}

/// The alternatives and the fields of a program type (F4 2.8), and `type`
/// lines (F16 2.4). A record type of 100,000 fields is over the size limit,
/// a static rule (F4 6.1): every compile gives `check-limit`. One field
/// fewer is at the limit, and the walk reads each field.
#[test]
fn program_types_and_type_lines() {
    on_the_stated_stack(|| {
        let n = N as u64;
        vec![
            Form::new(
                |n| format!("r is {}", each(n, " | ", |i| format!("\"k{i}\""))),
                "bool",
                "false",
                1,
                0,
            ),
            Form::new(
                |n| {
                    let alts = each(n - 1, " | ", |i| format!("\"k{i}\""));
                    format!("len(y as {alts} | \"a\")")
                },
                "int",
                "1",
                2,
                0,
            ),
            Form {
                static_rule: true,
                ..Form::over_the_limit(
                    |n| format!("r is {{{}}}", each(n, ", ", |i| format!("k{i}?: int"))),
                    "",
                    0,
                    0,
                )
            },
            Form::new(
                |n| format!("r is {{{}}}", each(n - 1, ", ", |i| format!("k{i}?: int"))),
                "bool",
                "false",
                n + 3,
                0,
            ),
            Form::program(
                |n| format!("{}\nn", each(n, "\n", |i| format!("type T{i} = int;"))),
                "int",
                "1",
                0,
                0,
            ),
            // Each line names the line before it.
            Form::program(
                |n| {
                    let lines = each(n - 1, "\n", |i| format!("type T{} = T{i};", i + 1));
                    format!("type T0 = int;\n{lines}\nn as T{}", n - 1)
                },
                "int",
                "1",
                1,
                0,
            ),
        ]
    });
}

/// The parts of a template literal (each `${` opens a level of its own),
/// and comments (13.1: "white space, comments").
#[test]
fn template_parts_and_comments() {
    on_the_stated_stack(|| {
        let n = N as u64;
        vec![
            Form::new(
                |n| format!("`{}`", "${y}".repeat(n)),
                "string",
                &format!("\"{}\"", "a".repeat(N)),
                1_563,
                1_563,
            ),
            Form::new(|n| format!("{}n", "/* c */ ".repeat(n)), "int", "1", 0, 0),
            Form::new(|n| format!("{}n", "// c\n".repeat(n)), "int", "1", 0, 0),
            Form::new(
                |n| format!("[{}n]", "n, /* c */ ".repeat(n)),
                "int[]",
                &format!("[{}]", vec!["1"; N + 1].join(",")),
                0,
                n + 2,
            ),
        ]
    });
}

/// The parts of a template of F2 (2.2): each `${{` opens a level of its
/// own, so parts side by side are flat. Its analysis, its program, a run and
/// its formatter.
#[test]
fn parts_of_a_template() {
    std::thread::Builder::new()
        .stack_size(stated_stack(200))
        .spawn(|| {
            let env = env();
            let text = "${{ y }}-".repeat(N);
            let t = parse_template(&text).unwrap();
            assert_eq!(t.analyze(TemplateMode::String).refs.len(), N);
            let p = t.program(TemplateMode::String);
            let r = env.check(&p, &Expected::None).unwrap();
            assert_eq!(r.ty.to_string(), "string");
            let prog = env.compile(&p, CompileOptions::default()).unwrap();
            let vars = host();
            let out = prog.run(&vars).unwrap();
            assert_eq!(json_text(&out).unwrap(), format!("\"{}\"", "a-".repeat(N)));
            assert!(format_template(&text).is_ok());
        })
        .unwrap()
        .join()
        .unwrap();
}
