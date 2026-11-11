//! The public surface of `05-host-api.md` Part E, reached the way a host
//! reaches it: through the crate root alone.

use risor::{
    conform, conform_all, exact_record_type, format, join, json_text, list_type, literal_type,
    load_value, map_type, nullable, opaque_type_named, parse, parse_template, record_type,
    render_line, tagged_union_type, truthy, type_from_schema, union_type, CompileOptions, Data,
    Env, Expected, FieldType, LoadOptions, NoVars, OpaqueRef, Pos, SchemaOptions, TemplateMode,
    Type, TypeKind, Value,
};
use std::collections::BTreeMap;

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
    let opts = |expected| LoadOptions { expected, max: 3 };
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
    assert_eq!(e.unwrap_err().step, 2);
    let e = load_value(&Env::default(), "1", &NoVars, opts(Expected::text("Nope")));
    assert_eq!(e.unwrap_err().step, 0);
}

#[test]
fn templates_and_tools() {
    let t = parse_template("a ${{ x }} b ${{ y + 1 }}").unwrap();
    assert!(!t.is_single());
    assert_eq!(t.part(1).unwrap().text(), "a ${{ x }} b ${{ y + 1 }}");
    assert!(t.part(2).is_none());
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
}
