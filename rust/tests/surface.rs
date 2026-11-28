//! The public surface of `05-host-api.md` Part E, reached the way a host
//! reaches it: through the crate root alone.

use risor::{
    conform, conform_all, exact_record_type, format, join, json_text, list_type, literal_type,
    load_value, load_with, map_type, nullable, opaque_type_named, parse, parse_template,
    record_type, render_line, schema_of, tagged_union_type, truthy, type_from_schema, union_type,
    CompileOptions, Data, Entry, EntryParam, Env, Expected, FieldType, Host, LoadOptions, NoVars,
    OpaqueRef, PathStep, Phase, Pos, RunOptions, SchemaOptions, TemplateMode, Type, TypeKind,
    Value, SOURCE_LIMIT, STEP_CHECK, STEP_CONFORM, STEP_DECLARE, STEP_DECODE, STEP_PARSE, STEP_RUN,
};
use std::collections::BTreeMap;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};

fn field(key: &str, ty: Type) -> FieldType {
    FieldType {
        key: key.into(),
        ty,
        optional: false,
    }
}

#[test]
fn types_are_built_and_read() {
    assert_eq!(Type::INT.kind(), TypeKind::Int);
    assert_eq!(Type::NEVER.kind(), TypeKind::Never);
    assert_eq!(list_type(&Type::STRING).to_string(), "string[]");
    assert_eq!(map_type(&Type::FLOAT).to_string(), "map<float>");
    let rec = exact_record_type(&[field("a", Type::INT), field("b", Type::STRING)]);
    assert_eq!(rec.kind(), TypeKind::Record);
    assert!(rec.is_exact());
    assert_eq!(rec.rest(), Some(Type::NEVER));
    let open = record_type(&[field("a", Type::INT)], &Type::ANY);
    assert_eq!(open.to_string(), "{a: int, ...}");
    assert!(!open.is_exact());
    // A reader of one kind reads the matching alternative of a mixed type.
    let maybe = nullable(&rec);
    assert_eq!(maybe.kind(), TypeKind::Mixed);
    assert!(maybe.is_nullable());
    assert_eq!(maybe.fields().len(), 2);
    assert_eq!(maybe.field("a").map(|f| f.ty), Some(Type::INT));
    assert_eq!(maybe.non_null(), rec);
    assert_eq!(maybe.alternatives().len(), 2);
    assert_eq!(nullable(&list_type(&Type::INT)).elem(), Some(Type::INT));
    let lits = literal_type(&["b", "a"]);
    assert_eq!(lits.literals(), ["a", "b"]);
    assert!(Type::STRING.literals().is_empty());
    assert!(lits.fits(&Type::STRING));
    assert!(!Type::STRING.fits(&lits));
    assert!(join(&Type::INT, &Type::NULL).equal(&nullable(&Type::INT)));
    assert_eq!(opaque_type_named("Time").opaque_name(), "Time");
    assert_eq!(Type::INT.opaque_name(), "");
    assert!(Type::ANY.err().is_none());
}

#[test]
fn unions_and_invalid_types() {
    let circle = exact_record_type(&[
        field("kind", literal_type(&["circle"])),
        field("r", Type::FLOAT),
    ]);
    let rect = exact_record_type(&[
        field("kind", literal_type(&["rect"])),
        field("w", Type::FLOAT),
    ]);
    let u = tagged_union_type(&[circle.clone(), rect.clone()]);
    assert_eq!(u.kind(), TypeKind::Union);
    assert_eq!(u.tag().as_deref(), Some("kind"));
    assert_eq!(u.variants().len(), 2);
    assert!(union_type(&[circle, rect]).equal(&u));
    // A construction with no type is invalid, and says why.
    let bad = tagged_union_type(&[Type::INT]);
    assert_eq!(bad.kind(), TypeKind::Invalid);
    assert_eq!(bad.err().unwrap().first().code, "bad-type");
    assert_eq!(literal_type(&[]).kind(), TypeKind::Invalid);
    assert!(!bad.fits(&Type::ANY));
    // P2: invalid types are never equal, even with identical messages.
    let bad_a = literal_type(&[]);
    let bad_b = literal_type(&[]);
    assert!(!bad_a.equal(&bad_b));
    assert!(!bad_a.equal(&bad_a));
    assert!(!bad_a.equal(&Type::ANY));
    assert!(!Type::ANY.equal(&bad_a));
    let dup = exact_record_type(&[field("a", Type::INT), field("a", Type::INT)]);
    assert!(dup.err().is_some());
    // An invalid type declared is `bad-type`.
    let e = Env::builder().var_type("x", &dup).build().err().unwrap();
    assert_eq!(e.first().code, "bad-type");
}

#[test]
fn an_environment_of_types_the_host_built() {
    let order = exact_record_type(&[field("id", Type::STRING)]);
    let env = Env::builder()
        .named_type_of("Order", &order)
        .var_type("o", &order)
        .groups(&["json"])
        .without(&["len"])
        .build()
        .unwrap();
    assert_eq!(
        env.parse_type("Order[]").unwrap().to_string(),
        "{id: string}[]"
    );
    let names = env.library_names();
    assert!(!names.functions.contains(&"len".to_string()));
    assert!(names.functions.contains(&"min".to_string()));
    assert_eq!(names.namespaces, ["json"]);
    assert!(names.group_functions.contains(&"json.parse".to_string()));
    assert!(names.methods["string"].contains(&"repeat".to_string()));
    assert!(names.methods["list"].windows(2).all(|w| w[0] < w[1]));
    let x = parse("o.id").unwrap();
    let r = env.check(&x, &Expected::text("string")).unwrap();
    assert_eq!(r.ty, Type::STRING);
    assert!(env.check(&x, &Expected::of(&Type::INT)).is_err());
    assert!(env.check(&x, &Expected::Condition).is_ok());
    let schema = Expected::schema(r#"{"type": "string"}"#, SchemaOptions::default()).unwrap();
    assert!(env.check(&x, &schema).is_ok());
}

#[test]
fn compile_run_and_measure() {
    let env = Env::builder().var("n", "int").build().unwrap();
    let x = parse("n * 2").unwrap();
    assert_eq!(x.text(), "n * 2");
    assert_eq!(parse("1 + 2;").unwrap().text(), "1 + 2");
    assert_eq!(parse("  1 + 2 ; ").unwrap().text(), "1 + 2");
    assert_eq!(parse("(1 + 2);").unwrap().text(), "(1 + 2)");
    assert_eq!(parse("((1 + 2));").unwrap().text(), "((1 + 2))");
    assert_eq!(parse("const a = 1; a;").unwrap().text(), "const a = 1; a");
    assert_eq!(
        parse("const a = 1; (a + 1);").unwrap().text(),
        "const a = 1; (a + 1)"
    );
    assert_eq!(
        parse("type Id = int; const a: Id = 1; a;").unwrap().text(),
        "type Id = int; const a: Id = 1; a"
    );
    assert_eq!(
        parse("export const f = (x: int) => x;").unwrap().text(),
        "export const f = (x: int) => x;"
    );
    assert_eq!(
        parse("  // note\nexport const f = (x: int) => x;  ")
            .unwrap()
            .text(),
        "export const f = (x: int) => x;"
    );
    assert_eq!(
        parse("f((a + 1))").unwrap().analyze().calls[0].args[0]
            .as_ref()
            .unwrap()
            .text(),
        "(a + 1)"
    );
    assert_eq!(
        parse_template("${{(a + 1)}}")
            .unwrap()
            .part(0)
            .unwrap()
            .text(),
        "(a + 1)"
    );
    let p = env.compile(&x, CompileOptions::default()).unwrap();
    assert_eq!(p.check_result().unwrap().ty, Type::INT);
    let mut vars = BTreeMap::new();
    vars.insert("n".to_string(), Data::Int(21));
    let v = p.run(&vars).unwrap();
    assert_eq!(json_text(&v).unwrap(), "42");
    assert!(truthy(&v));
    let (r, usage) = p.measure(&vars);
    assert!(r.is_ok() && usage.steps > 0);
    // A failure gives its usage too, and one `run` diagnostic.
    let unchecked = CompileOptions {
        check: false,
        ..CompileOptions::default()
    };
    let p = env
        .compile(&parse("1 / (n - n)").unwrap(), unchecked)
        .unwrap();
    assert!(p.check_result().is_none());
    let (r, usage) = p.measure(&vars);
    let e = r.unwrap_err();
    assert_eq!(e.first().code, "divide-by-zero");
    assert!(usage.steps > 0);
    assert!(render_line(&e.diagnostics).contains("divide-by-zero"));
}

#[test]
fn conform_needs_no_environment() {
    let t = exact_record_type(&[field("at", opaque_type_named("Time"))]);
    let mut m = BTreeMap::new();
    m.insert("at".to_string(), Data::Opaque(OpaqueRef::new("Time", 5u64)));
    let v = Value::from_data(&Data::Map(m.clone()));
    assert!(conform(&v, &t).is_ok());
    m.insert("at".to_string(), Data::Opaque(OpaqueRef::new("Date", 5u64)));
    let v = Value::from_data(&Data::Map(m));
    assert_eq!(conform(&v, &t).unwrap_err().code, "not-conforming");
    assert_eq!(conform_all(&v, &t, 5).len(), 1);
}

#[test]
fn a_config_file_loads() {
    let ty = type_from_schema(
        r#"{"type": "object", "properties": {"port": {"type": "integer"}}, "required": ["port"]}"#,
        SchemaOptions {
            closed: true,
            ..SchemaOptions::default()
        },
    )
    .unwrap();
    let opts = |expected| LoadOptions {
        expected,
        max: 3,
        ..LoadOptions::default()
    };
    let got = load_value(
        &Env::default(),
        "{port: 80}",
        &NoVars,
        opts(Expected::of(&ty)),
    );
    let want: BTreeMap<String, Data> = [("port".to_string(), Data::Float(80.0))].into();
    assert_eq!(got.unwrap(), Data::Map(want));
    let e = load_value(
        &Env::default(),
        "{port: 80, x: 1}",
        &NoVars,
        opts(Expected::of(&ty)),
    );
    assert_eq!(e.unwrap_err().step, STEP_CHECK);
    let e = load_value(&Env::default(), "1", &NoVars, opts(Expected::text("Nope")));
    assert_eq!(e.unwrap_err().step, STEP_DECLARE);
    assert_eq!(SOURCE_LIMIT, 1_048_576);
    assert_eq!(
        [
            STEP_DECLARE,
            STEP_PARSE,
            STEP_CHECK,
            STEP_RUN,
            STEP_CONFORM,
            STEP_DECODE
        ],
        [0, 1, 2, 3, 4, 5]
    );
}

#[test]
fn load_honors_cancellation() {
    let cancel = Arc::new(AtomicBool::new(true));
    let opts = LoadOptions {
        expected: Expected::text("int"),
        run: RunOptions {
            cancel: Some(Arc::clone(&cancel)),
        },
        ..LoadOptions::default()
    };
    let e = load_value(
        &Env::default(),
        "len(range(5000).map(x => x + 1))",
        &NoVars,
        opts.clone(),
    )
    .unwrap_err();
    assert_eq!(e.step, STEP_RUN);
    assert_eq!(e.diagnostics[0].code, "cancelled");
    assert!(e.usage.is_some());

    // Cancellation also works through `load_with` into a host type.
    cancel.store(true, Ordering::Relaxed);
    let e = load_with::<i64>(
        &Env::default(),
        "len(range(5000).map(x => x + 1))",
        &NoVars,
        opts,
    )
    .unwrap_err();
    assert_eq!(e.step, STEP_RUN);
    assert_eq!(e.diagnostics[0].code, "cancelled");
    assert!(e.usage.is_some());
}

#[test]
fn opaque_value_accessors() {
    assert!(Value::from_data(&Data::Null).is_null());
    assert_eq!(Value::from_data(&Data::Bool(true)).as_bool(), Some(true));
    assert_eq!(Value::from_data(&Data::Int(42)).as_f64(), Some(42.0));
    assert_eq!(Value::from_data(&Data::Int(42)).as_i64(), Some(42));
    assert_eq!(Value::from_data(&Data::Float(1.5)).as_f64(), Some(1.5));
    assert_eq!(Value::from_data(&Data::Float(1.5)).as_i64(), None);
    assert_eq!(
        Value::from_data(&Data::Str("hi".into())).as_str(),
        Some("hi")
    );
    let list = Value::from_data(&Data::List(vec![Data::Int(1), Data::Int(2)]));
    assert!(list.is_list());
    assert!(!list.is_map());
    assert_eq!(list.len(), Some(2));
    assert_eq!(list.is_empty(), Some(false));
    assert_eq!(list.elem(1).as_i64(), Some(2));
    assert!(list.elem(99).is_null());
    let mut m = BTreeMap::new();
    m.insert("k".to_string(), Data::Str("v".into()));
    let map = Value::from_data(&Data::Map(m));
    assert!(map.is_map());
    assert_eq!(
        map.field("k").and_then(|v| v.as_str().map(str::to_string)),
        Some("v".into())
    );
    let mut keys = Vec::new();
    map.for_each_key(|k| keys.push(k.to_string()));
    assert_eq!(keys, ["k"]);
    let o = OpaqueRef::new("Token", 9u8);
    let ov = Value::from_data(&Data::Opaque(o.clone()));
    assert!(ov.as_opaque().is_some_and(|x| x.same(&o)));
}

#[test]
fn templates_and_tools() {
    let t = parse_template("a ${{ x }} b ${{ y + 1 }}").unwrap();
    assert!(!t.is_single());
    assert_eq!(t.part(0).unwrap().text(), "x");
    assert_eq!(t.part(1).unwrap().text(), "y + 1");
    assert!(t.part(2).is_none());
    assert_eq!(parse("  // note\n x + 1 ").unwrap().text(), "x + 1");
    let call_arg = parse("f(a + 1, 2)").unwrap().analyze().calls[0].args[0]
        .clone()
        .unwrap();
    assert_eq!(call_arg.text(), "a + 1");
    assert_eq!(t.analyze(TemplateMode::String).names, ["x", "y"]);
    assert_eq!(format("1+2").unwrap(), "1 + 2\n");
    let env = Env::builder()
        .var_record("{user: {name: string}}")
        .build()
        .unwrap();
    let c = env.complete("1 +\nuser.na", Pos { line: 2, column: 8 }, &Expected::None);
    assert!(c.items.iter().any(|i| i.label == "name"));
    let h = env
        .hover(
            "1 +\nuser.name",
            Pos { line: 2, column: 7 },
            &Expected::None,
        )
        .unwrap();
    assert_eq!(h.ty.as_deref(), Some("string"));
    assert_eq!(h.span.to_string(), "[2, 6, 2, 10]");
    assert_eq!(c.span.start, Pos { line: 2, column: 6 });
}

#[test]
fn every_span_is_lines_and_columns() {
    let x = parse("1 +\n  f(a.b)").unwrap();
    let a = x.analyze();
    let at = |line, column| Pos { line, column };
    assert_eq!(a.refs[0].span.start, at(2, 5));
    assert_eq!(a.calls[0].span.end, at(2, 9));
    assert_eq!(a.calls[0].arg_spans[0].to_string(), "[2, 5, 2, 8]");
    let t = parse_template("a\n${{ x }}").unwrap();
    match &t.segments()[1] {
        risor::Segment::Part { expr, .. } => assert_eq!(expr.start, at(2, 5)),
        s => panic!("{s:?}"),
    }
    let env = Env::builder().var("m", "map<int>").build().unwrap();
    let r = env.check(&parse("\nm.k").unwrap(), &Expected::None);
    assert_eq!(r.unwrap_err().first().span.unwrap().start.line, 2);
    let env = Env::builder().var("xs", "int[]").build().unwrap();
    let r = env
        .check(&parse("\nxs[0]").unwrap(), &Expected::None)
        .unwrap();
    assert_eq!(r.sites[0].span.start, at(2, 1));
}

#[test]
fn json_text_fails_with_an_error() {
    let v = Value::from_data(&Data::Opaque(OpaqueRef::new("Time", 1u8)));
    let e = json_text(&v).unwrap_err();
    assert_eq!(e.first().code, "wrong-kind");
}

#[test]
fn decode_takes_no_maximum() {
    let v = Value::from_data(&Data::List(vec![Data::Float(1.0), Data::Float(2.0)]));
    assert_eq!(risor::decode::<Vec<u8>>(&v).unwrap(), [1, 2]);
    let v = Value::from_data(&Data::Float(300.0));
    let e = risor::decode::<u8>(&v).unwrap_err();
    assert_eq!(e.first().code, "not-conforming");
}

#[test]
fn unchecked_programs_and_invalid_types() {
    let unchecked = CompileOptions {
        check: false,
        ..CompileOptions::default()
    };
    let p = Env::default()
        .compile(&parse("1").unwrap(), unchecked)
        .unwrap();
    assert!(p.check_result().is_none());
    let bad = literal_type(&[]);
    let v = Value::from_data(&Data::Null);
    assert_eq!(conform(&v, &bad).unwrap_err().code, "bad-type");
    assert_eq!(conform_all(&v, &bad, 0)[0].code, "bad-type");
}

#[test]
fn schema_of_and_documentation() {
    let env = Env::builder()
        .var_record("{w: {/** The city. */ city: string, n?: int}}")
        .opaque("Token")
        .opaque_doc("Token", "A session token.")
        .function("rate", "(code: string) => float", |_, _| {
            Ok(Data::Float(1.0))
        })
        .function_doc("rate", "The rate.")
        .build()
        .unwrap();
    let p = env
        .compile(&parse("w").unwrap(), CompileOptions::default())
        .unwrap();
    let t = p.check_result().unwrap().ty.clone();
    assert_eq!(
        risor::schema_of(&t).unwrap(),
        r#"{"additionalProperties":false,"properties":{"city":{"description":"The city.","type":"string"},"n":{"type":"integer"}},"required":["city"],"type":"object"}"#
    );
    let e = risor::schema_of(&opaque_type_named("Token")).unwrap_err();
    assert_eq!(e.first().code, "bad-type");
    let h = env
        .hover("rate(\"x\")", Pos { line: 1, column: 1 }, &Expected::None)
        .unwrap();
    assert_eq!(h.doc.as_deref(), Some("The rate."));
    let h = env
        .hover("w.city", Pos { line: 1, column: 3 }, &Expected::None)
        .unwrap();
    assert_eq!(h.doc.as_deref(), Some("The city."));
    // A child keeps the documentation it inherits.
    let child = env.with().build().unwrap();
    let h = child
        .hover(
            "const f = (t: Token) => 1; 1",
            Pos {
                line: 1,
                column: 15,
            },
            &Expected::None,
        )
        .unwrap();
    assert_eq!(h.doc.as_deref(), Some("A session token."));
}

#[test]
fn export_const_entries_and_call_stage() {
    let env = Env::builder().var("tax", "float").build().unwrap();
    let src = r#"
type Units = "metric" | "imperial";
const mult = 1 + tax;
/** Compute total price. */
export const price = (
  /** Base amount. */
  base: float,
  /** Currency units. */
  units: Units = "metric",
): {total: float, units: string} => ({total: base * mult, units});
/** Sum a record. */
export const sumRec = (q: {a: int, b: int}) => q.a + q.b;
export const noRec = (_: int) => 42;
"#;
    let parsed = parse(src).unwrap();

    // CheckResult and Program both expose Entries in text order.
    let opts = CompileOptions::default()
        .expect_entry("price", "{total: float, units: string}")
        .expect_entry_type("sumRec", &Type::INT)
        .expect_entry_schema("noRec", r#"{"type": "integer"}"#, SchemaOptions::default())
        .unwrap();
    let cr = env.check_with(&parsed, &opts).unwrap();
    assert_eq!(cr.ty, Type::NEVER);
    assert_eq!(cr.entries.len(), 3);
    assert_eq!(
        cr.entries[0],
        Entry {
            name: "price".into(),
            doc: Some("Compute total price.".into()),
            params: vec![
                EntryParam {
                    name: Some("base".into()),
                    ty: Type::FLOAT,
                    doc: Some("Base amount.".into()),
                    optional: false,
                },
                EntryParam {
                    name: Some("units".into()),
                    ty: literal_type(&["metric", "imperial"]),
                    doc: Some("Currency units.".into()),
                    optional: true,
                },
            ],
            input_type: Some(exact_record_type(&[
                field("base", Type::FLOAT),
                FieldType {
                    key: "units".into(),
                    ty: literal_type(&["metric", "imperial"]),
                    optional: true,
                },
            ])),
            result_type: Some(exact_record_type(&[
                field("total", Type::FLOAT),
                field("units", Type::STRING),
            ])),
        }
    );
    // SchemaOf(input_type) carries parameter descriptions.
    let price_schema = schema_of(cr.entries[0].input_type.as_ref().unwrap()).unwrap();
    assert!(price_schema.contains(r#""description":"Base amount.""#));
    assert!(price_schema.contains(r#""description":"Currency units.""#));
    assert!(cr.entries[2].input_type.is_none());

    let p = env.compile(&parsed, opts).unwrap();
    assert!(!p.has_result());
    assert_eq!(p.entries(), cr.entries.as_slice());

    // Running a result-less program fails declare `no-result` with 0 steps/units.
    let (res, usage) = p.measure(&NoVars);
    let err = res.unwrap_err();
    assert_eq!(err.phase, Phase::Declare);
    assert_eq!(err.first().code, "no-result");
    assert_eq!(usage.steps, 0);
    assert_eq!(usage.allocated, 0);

    // Positional call and call_measure with host vars.
    let mut vars = BTreeMap::new();
    vars.insert("tax".to_string(), Data::Float(0.25));
    let (v, usage) = p.call_measure("price", &[&100.0f64], &vars);
    assert_eq!(
        json_text(&v.unwrap()).unwrap(),
        r#"{"total":125,"units":"metric"}"#
    );
    assert!(usage.steps > 0);

    // Call by record (Row 2 with omitted optional field, and Row 1 single record).
    let mut rec_input = BTreeMap::new();
    rec_input.insert("base".to_string(), Data::Float(80.0));
    let rec_input = Data::Map(rec_input);
    let v = p.call_record("price", &rec_input, &vars).unwrap();
    assert_eq!(json_text(&v).unwrap(), r#"{"total":100,"units":"metric"}"#);

    let mut sum_input = BTreeMap::new();
    sum_input.insert("a".to_string(), Data::Int(10));
    sum_input.insert("b".to_string(), Data::Int(32));
    let sum_input = Data::Map(sum_input);
    let v = p.call_record("sumRec", &sum_input, &NoVars).unwrap();
    assert_eq!(json_text(&v).unwrap(), "42");

    // Row 3 fails declare `no-input-record`.
    let e = p
        .call_record("noRec", &Data::Map(BTreeMap::new()), &NoVars)
        .unwrap_err();
    assert_eq!(e.phase, Phase::Declare);
    assert_eq!(e.first().code, "no-input-record");

    // Positional conformance error prefixes path with 0-based index and uses 0 steps/units.
    let (res, usage) = p.call_measure("price", &[&"bad", &"wrong"], &vars);
    let err = res.unwrap_err();
    assert_eq!(err.phase, Phase::Run);
    assert_eq!(err.diagnostics.len(), 2);
    assert_eq!(err.diagnostics[0].code, "not-conforming");
    assert_eq!(err.diagnostics[0].span, None);
    assert_eq!(
        err.diagnostics[0].path.as_deref(),
        Some(&vec![PathStep::Index(0)])
    );
    assert_eq!(
        err.diagnostics[1].path.as_deref(),
        Some(&vec![PathStep::Index(1)])
    );
    assert_eq!(usage.steps, 0);
    assert_eq!(usage.allocated, 0);

    // Concurrent calls on one compiled program are isolated.
    std::thread::scope(|s| {
        let mut handles = Vec::new();
        for i in 1..=4i64 {
            let p = &p;
            handles.push(s.spawn(move || {
                let mut v = BTreeMap::new();
                v.insert("tax".to_string(), Data::Float(0.1 * i as f64));
                let arg = Data::Float(10.0);
                let out = p
                    .call_with("price", &[&arg as &dyn Host], &v, RunOptions::default())
                    .unwrap();
                json_text(&out).unwrap()
            }));
        }
        let got: Vec<String> = handles.into_iter().map(|h| h.join().unwrap()).collect();
        assert_eq!(
            got,
            vec![
                r#"{"total":11,"units":"metric"}"#,
                r#"{"total":12,"units":"metric"}"#,
                r#"{"total":13,"units":"metric"}"#,
                r#"{"total":14,"units":"metric"}"#,
            ]
        );
    });

    // Unchecked compile exposes Entries with result_type == None and still conforms arguments.
    let unchecked = env
        .compile(
            &parsed,
            CompileOptions {
                check: false,
                ..CompileOptions::default()
            }
            .expect_entry("price", "string"),
        )
        .unwrap();
    assert!(unchecked.entries()[0].result_type.is_none());
    let e = unchecked.call("price", &[&"nope"], &vars).unwrap_err();
    assert_eq!(e.phase, Phase::Run);
    assert_eq!(e.first().code, "not-conforming");

    // `condition` is rejected for an entry point expected result.
    let mut bad_opts = CompileOptions::default();
    bad_opts
        .entry_expected
        .insert("price".into(), Expected::Condition);
    let e = env.check_with(&parsed, &bad_opts).unwrap_err();
    assert_eq!(e.phase, Phase::Declare);
    assert_eq!(e.first().code, "bad-type");
}

#[test]
fn export_const_conformance_caps_at_100_diagnostics() {
    let parsed = parse("export const f = (xs: int[], ys: int[]) => len(xs) + len(ys);").unwrap();
    let p = Env::default()
        .compile(&parsed, CompileOptions::default())
        .unwrap();
    let bad_xs = Data::List((0..60).map(|_| Data::Str("a".into())).collect());
    let bad_ys = Data::List((0..60).map(|_| Data::Str("b".into())).collect());
    let (res, usage) = p.call_measure("f", &[&bad_xs, &bad_ys], &NoVars);
    let err = res.unwrap_err();
    assert_eq!(err.phase, Phase::Run);
    assert_eq!(err.diagnostics.len(), 100);
    assert_eq!(
        err.diagnostics[0].path.as_deref(),
        Some(&vec![PathStep::Index(0), PathStep::Index(0)])
    );
    assert_eq!(
        err.diagnostics[59].path.as_deref(),
        Some(&vec![PathStep::Index(0), PathStep::Index(59)])
    );
    assert_eq!(
        err.diagnostics[60].path.as_deref(),
        Some(&vec![PathStep::Index(1), PathStep::Index(0)])
    );
    assert_eq!(
        err.diagnostics[99].path.as_deref(),
        Some(&vec![PathStep::Index(1), PathStep::Index(39)])
    );
    assert_eq!(usage.steps, 0);
    assert_eq!(usage.allocated, 0);
}

#[test]
fn annotated_entry_result_fitted_by_annotation() {
    for (src, want) in [
        (
            "export const f = (): {a: int, b?: int} => ({a: 1});",
            "{a: int}",
        ),
        ("export const f = (x: int): int | null => { x };", "int"),
    ] {
        let parsed = parse(src).unwrap();
        let opts = CompileOptions::default().expect_entry("f", want);
        let err = Env::default().check_with(&parsed, &opts).unwrap_err();
        assert_eq!(err.phase, Phase::Check);
        assert_eq!(err.first().code, "result-mismatch");
    }
}

#[test]
fn depth_64_entry_param_has_no_input_record() {
    let t64 = format!("int{}", "[]".repeat(64));
    let src = format!("export const f = (x: {t64}) => len(x);");
    let parsed = parse(&src).unwrap();
    let env = Env::default();

    let cr = env.check(&parsed, &Expected::None).unwrap();
    assert_eq!(cr.entries.len(), 1);
    assert!(cr.entries[0].input_type.is_none());

    let p = env.compile(&parsed, CompileOptions::default()).unwrap();
    assert!(p.entries()[0].input_type.is_none());

    let mut input = BTreeMap::new();
    input.insert("x".to_string(), Data::List(vec![]));
    let err = p.call_record("f", &Data::Map(input), &NoVars).unwrap_err();
    assert_eq!(err.phase, Phase::Declare);
    assert_eq!(err.first().code, "no-input-record");

    let empty = Data::List(vec![]);
    let out = p.call("f", &[&empty], &NoVars).unwrap();
    assert_eq!(json_text(&out).unwrap(), "0");
}
