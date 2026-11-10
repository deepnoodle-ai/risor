//! The checker against the worked examples of `05-types.md` 21.

use risor::check::{check, expected, ty::print};
use risor::{parse, Data, Env, HostError};

fn env(reject: &[&str]) -> Env {
    Env::builder()
        .vars(
            "{input:   {role: \"admin\" | \"member\" | \"owner\", note?: string,
                        tags: string[], priority: int | null,
                        options: {retries?: int, backoff?: string} | null},
              gate:    {failed: int, score: float,
                        items: {name: string, status: \"fail\" | \"pass\",
                                owner: string | null, ms: float}[]},
              report:  {verdict: \"APPROVE\" | \"REJECT\", body: string} | null,
              scores:  map<float>,
              event:   {action: string,
                        pull_request?: {number: int, head: {sha: string}}, ...},
              blob:    any,
              started: Time,
              ids:     (int | null)[]}",
        )
        .opaque("Time")
        .method_impl(
            "Time.format",
            "(layout: string) => string",
            Some(std::sync::Arc::new(|_, _| Ok(Data::Str(String::new())))),
        )
        .function("rate", "(code: string) => float !", |_, _| {
            Err(HostError("x".into()))
        })
        .function("geo.distance", "(a: string, b: string) => float", |_, _| {
            Ok(Data::Float(0.0))
        })
        .reject(reject)
        .build()
        .unwrap()
}

/// The result as the chapter writes it: the type and its sites, or the
/// diagnostics.
fn run(src: &str, reject: &[&str], exp: Option<&str>) -> String {
    let e = env(reject);
    let p = parse(src).unwrap();
    let x = expected(&e, exp).unwrap();
    let r = check(&e, &p, &x);
    let span = |s: risor::LineSpan| {
        let [a, b, c, d] = s.0;
        if a == 1 && c == 1 {
            format!("{b}-{d}")
        } else {
            format!("{a}:{b}-{c}:{d}")
        }
    };
    if r.diagnostics.is_empty() {
        let mut out = print(r.ty.as_ref().unwrap());
        for s in &r.sites {
            out.push_str(&format!(
                "; {} {}",
                s.class,
                span(p.source.line_span(s.span))
            ));
        }
        out
    } else {
        r.diagnostics
            .iter()
            .map(|d| format!("{} {}", d.code, span(d.span.unwrap())))
            .collect::<Vec<_>>()
            .join("; ")
    }
}

const CC: &[&str] = &["constant-condition"];

#[test]
fn section_21() {
    type Row<'a> = (u32, &'a str, &'a [&'a str], Option<&'a str>, &'a str);
    let rows: &[Row] = &[
        (1, "gate.failed > 0", CC, None, "bool"),
        (2, "input.role in [\"admin\", \"owner\"]", CC, None, "bool"),
        (3, "report.verdict", CC, None, "maybe-null 1-7"),
        (4, "report?.verdict ?? \"REJECT\"", CC, None, "\"APPROVE\" | \"REJECT\""),
        (5, "report?.verdict.toLowerCase()", CC, None, "string | null"),
        (6, "input.note.toUpperCase()", CC, None, "maybe-absent 7-11"),
        (7, "input?.note.toUpperCase()", CC, None, "maybe-null 1-12"),
        (8, "input?.note?.toUpperCase() ?? \"\"", CC, None, "string"),
        (9, "event?.pull_request?.head.sha", CC, None, "string | null"),
        (10, "gate.items[0].name", CC, None, "string; index 1-14"),
        (11, "gate.items?.[0]?.name", CC, None, "string | null"),
        (12, "gate.items[len(gate.items) / 2]", CC, None, "kind 12-31"),
        (13, "scores[input.role]", CC, None, "maybe-absent 8-18"),
        (14, "blob.x.y + 1", CC, None, "any; dynamic 1-5; dynamic 1-7; dynamic 1-9"),
        (15, "blob.x", &["constant-condition", "dynamic"], None, "unchecked 1-5"),
        (16, "gate.items[0].name", &["constant-condition", "index"], None, "may-fail 1-14"),
        (17, "started.format(\"iso\")", CC, None, "string"),
        (18, "inptu.role", CC, None, "undefined-name 1-6"),
        (19, "len", CC, None, "not-a-value 1-4"),
        (20, "gate.score / len(gate.items)", CC, None, "float; divide 1-29"),
        (21, "gate.failed / 0", CC, None, "zero-divisor 15-16"),
        (22, "gate.score % 2", CC, None, "float"),
        (23, "2 ** 10", CC, None, "int"),
        (24, "2 ** gate.failed", CC, None, "float"),
        (25, "2 ** gate.score", CC, None, "kind 6-16"),
        (26, "gate.failed + \" failures\"", CC, None, "kind 1-26"),
        (27, "-input.role", CC, None, "kind 2-12"),
        (28, "input.priority + 1", CC, None, "maybe-null 1-15"),
        (29, "gate.failed == \"0\"", CC, None, "never-equal 1-19"),
        (30, "`at ${started}`", CC, None, "kind 7-14"),
        (31, "{a: 1, a: 2}", CC, None, "duplicate-key 8-9"),
        (32, "{...gate, failed: \"none\"}", CC, None, "{failed: \"none\", items: {ms: float, name: string, owner: string | null, status: \"fail\" | \"pass\"}[], score: float}"),
        (33, "report != null && report.verdict == \"APPROVE\"", CC, None, "bool"),
        (34, "report && report.body", CC, None, "string | null"),
        (35, "report && report.verdict == \"APPROVE\"", CC, None, "bool | null"),
        (36, "report && report.verdict == \"APPROVE\"", CC, Some("bool"), "result-mismatch 1-38"),
        (37, "report && report.verdict == \"APPROVE\"", CC, Some("condition"), "bool | null"),
        (38, "input?.note || \"none\"", CC, None, "string"),
        (39, "input.priority || 3", CC, None, "int"),
        (40, "gate.failed || \"none\"", CC, None, "int | \"none\""),
        (41, "input.options && input.options?.retries", CC, None, "int | {} | null"),
        (42, "gate.items && gate.items[0].name", CC, None, "string | never[]; index 15-28"),
        (43, "started ? started.format(\"iso\") : \"\"", CC, None, "always-truthy 1-8"),
        (44, "input.role || \"member\"", CC, None, "always-truthy 1-11"),
        (45, "null || gate.failed", CC, None, "always-falsy 1-5"),
        (46, "!started", CC, None, "always-truthy 2-9"),
        (47, "started || started", &[], None, "Time"),
        (48, "started", CC, Some("condition"), "always-truthy 1-8"),
        (49, "gate.items", CC, Some("condition"), "{ms: float, name: string, owner: string | null, status: \"fail\" | \"pass\"}[]"),
        (50, "blob || \"x\"", CC, None, "any"),
        (51, "if (report) { report.verdict } else { \"none\" }", CC, None, "\"APPROVE\" | \"REJECT\" | \"none\""),
        (52, "if (!report) { \"none\" } else if (report.verdict == \"APPROVE\") { \"go\" } else { \"stop\" }", CC, None, "\"go\" | \"none\" | \"stop\""),
        (53, "if (gate.failed > 0) { const n = gate.failed; n * 2 } else { 0 }", CC, None, "int"),
        (54, "if (started) { 1 } else { 2 }", CC, None, "always-truthy 5-12"),
        (55, "match input.role { \"admin\" | \"owner\" => \"write\", \"member\" => \"read\" }", CC, None, "\"read\" | \"write\""),
        (56, "match input.role { \"admin\" => 1, \"guest\" => 0 }", CC, None, "non-exhaustive 1-48; never-equal 34-41"),
        (57, "match input.role { \"admin\" => 1, _ => 0, \"owner\" => 2 }", CC, None, "unreachable-arm 42-54"),
        (58, "match input.role { \"admin\" => 1, \"admin\" => 2, _ => 0 }", CC, None, "duplicate-case 34-41"),
        (59, "match input.priority { null => \"normal\", 1 | 2 => \"high\", _ => \"low\" }", CC, None, "\"high\" | \"low\" | \"normal\""),
        (60, "{ const {retries = 3, backoff = \"1s\"} = input.options ?? {}; {retries, backoff} }", CC, None, "{backoff: string, retries: int}"),
        (61, "{ const {role, ...rest} = input; rest }", CC, None, "{note?: string, options: {backoff?: string, retries?: int} | null, priority: int | null, tags: string[]}"),
        (62, "{ const {action, ...rest} = event; rest }", CC, None, "{pull_request?: {head: {sha: string}, number: int}, ...}"),
        (63, "{ const {note, ...rest} = input; rest }", CC, None, "maybe-absent 10-14"),
        (64, "{ const [first, second = \"\"] = input.tags; first }", CC, None, "string; index 10-15"),
        (65, "const a = 1; const a = 2; a", CC, None, "duplicate-name 20-21"),
        (66, "const geo = 1; geo", CC, None, "shadows-namespace 7-10; not-a-value 16-19"),
        (67, "const t = gate.failed; t(1)", CC, None, "call-of-parameter 24-25"),
        (68, "gate.items.filter(t => t.status == \"fail\").map(t => t.name)", CC, None, "string[]"),
        (69, "gate.items.map(t => { const slow = t.ms > 500; {name: t.name, slow} })", CC, None, "{name: string, slow: bool}[]"),
        (70, "scores.entries().map(({key, value}) => `${key}=${value}`)", CC, None, "string[]"),
        (71, "gate.items.reduce((m, t) => {...m, [t.name]: t.status}, {})", CC, None, "map<\"fail\" | \"pass\">"),
        (72, "gate.items.reduce((a, t) => a + t.ms, 0)", CC, None, "float"),
        (73, "gate.items.filter(t => t.owner).map(t => t.name)", CC, None, "string[]"),
        (74, "gate.items.filter(t => t.status)", CC, None, "always-truthy 24-32"),
        (75, "gate.items.map(t => t.owner).filter(o => o != null)", CC, None, "string[]"),
        (76, "ids.filter(x => x != null).sum()", CC, None, "int"),
        (77, "ids.sum()", CC, None, "arg-mismatch 1-10"),
        (78, "gate.items.map()", CC, None, "arity 1-17"),
        (79, "gate.items.map((t, i, z) => t)", CC, None, "arity 16-30"),
        (80, "lenn(gate.items)", CC, None, "undefined-function 1-5"),
        (81, "len(5)", CC, None, "arg-mismatch 5-6"),
        (82, "rate(input.role)", CC, None, "float; host 1-17"),
        (83, "rate(report?.body)", CC, None, "maybe-null 6-18"),
        (84, "rate(x => x)", CC, None, "not-a-value 6-12"),
        (85, "const pct = (n, digits = 0) => round(n * 10 ** digits); pct(gate.score) + pct(gate.failed, 2)", CC, None, "int"),
        (86, "const name = t => t.nmae; gate.items.map(name)", CC, None, "no-field 21-25"),
        (87, "const twice = (f, x) => f(f(x)); twice(n => n + 1, 2)", CC, None, "call-of-parameter 25-26; call-of-parameter 27-28; lambda-to-lambda 40-50"),
        (88, "const inc = n => n + 1; [1, 2].map(inc)", CC, None, "int[]"),
        (89, "try { int(report?.body ?? \"\") } catch { 0 }", CC, None, "int"),
        (90, "(try { int(blob) } catch { 0 }) > 10", CC, None, "bool"),
        (91, "try { 1 } catch (err) { fail(err.message, err.data) }", CC, None, "int; value 25-52"),
        (92, "try { int(blob) } catch (err) { err.code == \"not-conformin\" ? 0 : fail(err.message) }", CC, None, "never-equal 33-60"),
        (93, "try { blob.x } catch (err) { err.stack }", CC, None, "no-field 34-39"),
        (94, "try { 1 } catch (err) { match err.code { \"failed\" => 1 } }", CC, None, "non-exhaustive 25-57"),
        (95, "try { blob.x } catch (err) { err.path }", CC, None, "any"),
        (96, "gate.failed > 0 ? 1 : \"a\"", CC, Some("int"), "result-mismatch 23-26"),
        (97, "{listen: \":80\", max_bod: 1}", CC, Some("{listen: string, max_body?: int}"), "result-mismatch 17-24"),
        (98, "{max_body: \"8\"}", CC, Some("{listen: string, max_body?: int}"), "result-mismatch 1-16; result-mismatch 12-15"),
        (99, "[1, 2.5, \"x\"]", CC, Some("int[]"), "result-mismatch 5-8; result-mismatch 10-13"),
        (100, "gate.items", CC, Some("bool"), "result-mismatch 1-11"),
        (106, "[].sum()", CC, None, "int"),
        (107, "[1, 2].join(\",\")", CC, None, "arg-mismatch 2-3; arg-mismatch 5-6"),
        (108, "[1].flatMap(x => x)", CC, None, "arg-mismatch 18-19"),
        (109, "[].map(x => x + 1)", CC, None, "int[]"),
        (110, "string(started)", CC, None, "kind 8-15"),
        (111, "string(blob)", CC, None, "string; dynamic 8-12"),
        (112, "`${ {a: blob} }`", CC, None, "string; dynamic 5-14"),
        (113, "gate.items.reduce((a, t) => [a], 0)", CC, None, "arg-mismatch 29-32"),
        (114, "gate.items.reduce((a, t) => gate.failed > 0 ? blob : a + t.ms, 0)", CC, None, "any; dynamic 54-55"),
        (115, "\"a\" in []", CC, None, "bool"),
    ];
    let mut bad = Vec::new();
    for (n, src, reject, exp, want) in rows {
        let got = run(src, reject, *exp);
        if got != *want {
            bad.push(format!("{n}: {src}\n   want {want}\n   got  {got}"));
        }
    }
    assert!(
        bad.is_empty(),
        "{} of {} differ:\n{}",
        bad.len(),
        rows.len(),
        bad.join("\n")
    );
}

/// `05-types.md` 19.5: with the checker, an undeclared type name and a key written
/// twice deeper in a type are `bad-type` at the declare step.
#[test]
fn checker_declare_errors() {
    for t in [
        "{q: Foo}",
        "{q: {a: int, a: int}}",
        "{x: {a: int} | {b: string}}",
    ] {
        let e = Env::builder().vars(t).build().err().expect(t);
        assert_eq!(e.first().code, "bad-type", "{t}");
    }
    let e = Env::builder()
        .function("f", "(...xs: int) => int", |_, _| Ok(Data::Null))
        .build()
        .err()
        .unwrap();
    assert_eq!(e.first().code, "bad-signature");
    let e = Env::builder()
        .method_impl("Time.f", "() => int", None)
        .build()
        .err()
        .unwrap();
    assert_eq!(e.first().code, "bad-signature");
    assert!(Env::builder().vars("{x: {a: int, ...}}").build().is_ok());
}

/// 20.3: at most 1,000 lambda body checks; 20.6: what opens no level costs no
/// stack in the checker either.
#[test]
fn checker_limits() {
    let e = env(CC);
    let mut src = String::from("const f = x => x;\n");
    src.push_str(&vec!["f(1)"; 1001].join(" + "));
    let p = parse(&src).unwrap();
    let r = check(&e, &p, &risor::check::Expected::Nothing);
    let limits: Vec<_> = r
        .diagnostics
        .iter()
        .filter(|d| d.code == "check-limit")
        .collect();
    assert_eq!(limits.len(), 1, "the 1,001st use");
    let mut src = String::from("const f = x => x;\n");
    src.push_str(&vec!["f(1)"; 1000].join(" + "));
    let r = check(&e, &parse(&src).unwrap(), &risor::check::Expected::Nothing);
    assert!(r.diagnostics.is_empty());
    let n = 100_000;
    for src in [
        vec!["gate.failed"; n].join(" + "),
        format!("{}0", "false ? 1 : ".repeat(n)),
        format!(
            "if (false) {{ 0 }}{} else {{ 2 }}",
            " else if (false) { 1 }".repeat(n)
        ),
        format!("input{}", ".tags"),
        vec!["gate.failed > 0"; n].join(" && "),
    ] {
        let r = check(&e, &parse(&src).unwrap(), &risor::check::Expected::Nothing);
        assert!(r.ty.is_some(), "{}", &src[..40]);
    }
}

/// `05-types.md` 20.7: the doubling chain of the probe returns at once. Each test
/// of a type's size is constant work, and nothing past the limit is built.
#[test]
fn the_doubling_chain_is_constant_work() {
    let mut lines = vec!["const a0 = {l: 1, r: 1};".to_string()];
    for k in 1..=40 {
        lines.push(format!("const a{k} = {{l: a{}, r: a{}}};", k - 1, k - 1));
    }
    lines.push("[a40, a39].len()".into());
    let src = lines.join("\n");
    let start = std::time::Instant::now();
    let p = risor::parse(&src).unwrap();
    let r = risor::check(&risor::Env::default(), &p, &risor::Expected::Nothing);
    assert!(start.elapsed().as_secs() < 2);
    let lines: Vec<u32> = r
        .diagnostics
        .iter()
        .filter(|d| d.code == "check-limit")
        .map(|d| d.span.unwrap().0[0])
        .collect();
    // Over at line 16; `a15` is then `never`, and the chain grows again
    // from size 3 until line 32.
    assert_eq!(lines, [16, 32]);
}

/// `05-types.md` 20.7: building a type is at most n log n in the number of values
/// joined. Four times the values must take well under sixteen times as long,
/// which a quadratic join would. Each measure is the best of three.
#[test]
fn large_literal_sets_are_n_log_n() {
    fn best(f: &dyn Fn()) -> f64 {
        (0..3)
            .map(|_| {
                let t = std::time::Instant::now();
                f();
                t.elapsed().as_secs_f64()
            })
            .fold(f64::INFINITY, f64::min)
    }
    let schema = |n: usize| {
        let vals: Vec<String> = (0..n).map(|i| format!("\"v{i}\"")).collect();
        risor::json::parse(&format!("{{\"enum\": [{}]}}", vals.join(","))).unwrap()
    };
    let matchp = |n: usize| {
        let arms: Vec<String> = (0..n).map(|i| format!("\"v{i}\" => \"r{i}\"")).collect();
        risor::parse(&format!("match s {{ {}, _ => \"z\" }}", arms.join(", "))).unwrap()
    };
    let text = |n: usize| {
        let alts: Vec<String> = (0..n).map(|i| format!("\"v{i}\"")).collect();
        format!("{{s: {}}}", alts.join(" | "))
    };
    let (a, b) = (8_000, 32_000);
    let (sa, sb) = (schema(a), schema(b));
    let ratio_schema = best(&|| {
        let _ = risor::ty_schema(&sb, risor::SchemaOptions::default());
    }) / best(&|| {
        let _ = risor::ty_schema(&sa, risor::SchemaOptions::default());
    });
    let env = risor::Env::builder().vars("{s: string}").build().unwrap();
    let (ma, mb) = (matchp(a), matchp(b));
    let ratio_match = best(&|| {
        risor::check(&env, &mb, &risor::Expected::Nothing);
    }) / best(&|| {
        risor::check(&env, &ma, &risor::Expected::Nothing);
    });
    let (ta, tb) = (text(a), text(b));
    let ratio_text = best(&|| {
        let _ = risor::Env::builder().vars(&tb).build();
    }) / best(&|| {
        let _ = risor::Env::builder().vars(&ta).build();
    });
    for (what, r) in [
        ("schema", ratio_schema),
        ("match", ratio_match),
        ("type text", ratio_text),
    ] {
        assert!(
            r < 9.0,
            "{what}: four times the values took {r:.1} times as long"
        );
    }
}
