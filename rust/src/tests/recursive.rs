//! Recursive named types (F17 12): what the suite cannot show. A comparison
//! that reaches the bound of pairs through a long chain of types needs no
//! more stack than the crate states; a recursive type leaks nothing; the
//! conformance function stops past the default depth limit.

use super::host::stated_stack;
use crate::check::ty::{print, Ty, TypeNames};
use crate::{parse, CompileOptions, Data, Env, Expected, Pos};
use std::sync::Weak;

/// Two chains of `n` type lines, each line holding the next and the last
/// the first: `A0` to `A{n-1}` and `B0` to `B{n-1}`.
fn chains(n: usize) -> String {
    let mut src = String::new();
    for p in ["A", "B"] {
        for i in 0..n {
            src.push_str(&format!("type {p}{i} = {{n: {p}{}}};\n", (i + 1) % n));
        }
    }
    src
}

/// Fits(A0, B0) enters the pair (Ai, Bi) inside (Ai-1, Bi-1), so 4,100
/// lines of each make the 4,097th pair in progress inside 4,096 others:
/// the bound ends the computation, and the check reports it on the value.
/// The same comparison through the environment's named types, an
/// environment's description, the type API, hover and completion. All on a
/// thread with the stack `DESIGN.md` states for the default depth limit.
#[test]
fn a_comparison_that_reaches_the_bound_runs_within_the_stated_stack() {
    std::thread::Builder::new()
        .name("pairs".into())
        .stack_size(stated_stack(200))
        .spawn(deep_comparisons)
        .unwrap()
        .join()
        .unwrap();
}

fn deep_comparisons() {
    let n = 4100;
    let env = Env::builder().var_record("{x: any}").build().unwrap();
    // Type lines: the resolution of one component of 8,200 lines, then a
    // fit past the bound.
    let src = format!("{}const a = x as A0;\nconst b: B0 = a;\n1", chains(n));
    let line = 2 * n as u32 + 2;
    let e = env
        .compile(&parse(&src).unwrap(), CompileOptions::default())
        .err()
        .unwrap();
    assert_eq!(e.diagnostics.len(), 1, "{:?}", e.diagnostics);
    let d = &e.diagnostics[0];
    assert_eq!(d.code, "check-limit");
    assert_eq!(
        d.message,
        "comparing the types here takes more than 4096 pairs of types"
    );
    assert_eq!(
        d.span.map(|s| (s.start.line, s.start.column)),
        Some((line, 15))
    );
    // Below the bound: the chains are one shorter than the bound.
    let src = format!("{}const a = x as A0;\nconst b: B0 = a;\nb", chains(4000));
    let r = env.check(&parse(&src).unwrap(), &Expected::None).unwrap();
    assert_eq!(r.ty.to_string(), "B0");
    // Hover and completion check the program too.
    let src = format!("{}const a = x as A0;\nconst b: B0 = a;\nb", chains(n));
    let at = Pos {
        line: line + 1,
        column: 1,
    };
    let _ = env.hover(&src, at, &Expected::None);
    let _ = env.complete(&src, at, &Expected::None);

    // The environment's named types: one list of 8,200, its description,
    // and a fit of the type API, false past the bound.
    let mut b = Env::builder();
    for p in ["A", "B"] {
        for i in 0..n {
            b = b.named_type(&format!("{p}{i}"), &format!("{{n: {p}{}}}", (i + 1) % n));
        }
    }
    let env = b.var_record("{x: A0}").build().unwrap();
    let text = env.description_text();
    assert!(text.contains(r#""A0":"{n: A1}""#));
    let a0 = env.parse_type("A0").unwrap();
    let b0 = env.parse_type("B0").unwrap();
    assert!(!a0.fits(&b0));
    assert!(!a0.equal(&b0));
    assert!(crate::join(&a0, &b0).err().is_some());
    let e = env
        .compile(
            &parse("const b: B0 = x; 1").unwrap(),
            CompileOptions::default(),
        )
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "check-limit");
}

/// A recursive type's table lives while a type that reaches it lives, and
/// no longer: its definitions hold their own references weakly.
#[test]
fn a_recursive_type_leaks_nothing() {
    let env = Env::builder().var_record("{x: any}").build().unwrap();
    let src = "type Tree = {kids: Tree[]}; const t: Tree = x as Tree; t.kids[0]";
    let parsed = parse(src).unwrap();
    let weak: Weak<crate::check::ty::Table> = {
        let r = crate::compile::resolve(&env, &parsed);
        assert_eq!(r.tables.len(), 1);
        std::sync::Arc::downgrade(&r.tables[0])
    };
    assert!(weak.upgrade().is_none(), "the resolution's table was kept");
    // A checked type holds its table, and lets it go.
    let ty = env.check(&parsed, &Expected::None).unwrap().ty;
    assert_eq!(ty.to_string(), "Tree");
    let fields = ty.fields();
    assert_eq!(fields[0].ty.to_string(), "Tree[]");
    drop(ty);
    assert_eq!(fields[0].ty.elem().unwrap().fields()[0].key, "kids");
    drop(fields);
    // An environment's recursive types, and a compiled program.
    let env = Env::builder()
        .named_type("Dir", "{files: File[], dirs: Dir[]}")
        .named_type("File", "{name: string, parent: Dir | null}")
        .var_record("{d: Dir}")
        .build()
        .unwrap();
    let refs = crate::check::ty::reach(env.data().var_types.iter());
    assert_eq!(refs.len(), 2);
    let weak = std::sync::Arc::downgrade(&refs[0].table().unwrap());
    drop(refs);
    let p = env
        .compile(
            &parse("d.files[0].parent").unwrap(),
            CompileOptions::default(),
        )
        .unwrap();
    assert_eq!(p.check_result().unwrap().ty.to_string(), "Dir | null");
    drop(p);
    drop(env);
    assert!(weak.upgrade().is_none(), "the environment's table was kept");
}

/// Printing and every walk over a type end on a cycle: a reference prints
/// by its name, and the walks that follow references visit each once.
#[test]
fn walks_end_on_a_cycle() {
    let env = Env::builder()
        .named_type("Json", "null | bool | float | string | Json[] | map<Json>")
        .opaque("Tok")
        .named_type("Cell", "{t: Tok, next: Cell | null}")
        .build()
        .unwrap();
    let names = &env.data().names;
    let json = names.named.iter().find(|(n, _)| n == "Json").unwrap();
    assert_eq!(print(&json.1), "Json");
    assert_eq!(
        print(&json.1.view()),
        "bool | float | string | Json[] | map<Json> | null"
    );
    assert!(!crate::check::ty::has_any(&json.1));
    let cell = names.named.iter().find(|(n, _)| n == "Cell").unwrap();
    assert!(crate::check::ty::has_opaque(&cell.1));
    assert!(crate::check::ty::equal(&json.1, &json.1));
    assert!(crate::check::ty::fits(&cell.1, &Ty::any()).is_some());
}

/// The conformance function counts the levels it enters and fails past
/// 200, with no path; `ConformAll` ends with the same diagnostic.
#[test]
fn the_conformance_function_stops_past_200_levels() {
    let env = Env::builder().named_type("Nest", "Nest[]").build().unwrap();
    let t = env.parse_type("Nest").unwrap();
    let nested = |n: usize| {
        let mut d = Data::List(vec![]);
        for _ in 1..n {
            d = Data::List(vec![d]);
        }
        d
    };
    let ok = crate::Value::from_data(&nested(200));
    assert!(crate::conform(&ok, &t).is_ok());
    assert!(crate::conform_all(&ok, &t, 5).is_empty());
    let deep = crate::Value::from_data(&nested(201));
    let d = crate::conform(&deep, &t).err().unwrap();
    assert_eq!(d.code, "depth-exceeded");
    assert_eq!(d.class, Some("limit"));
    assert!(d.path.is_none());
    assert!(d.span.is_none());
    let all = crate::conform_all(&deep, &t, 5);
    assert_eq!(all.len(), 1);
    assert_eq!(all[0].code, "depth-exceeded");
}

/// A key a recursive record type does not allow: the message prints the
/// type by its name (F17 12.6), as a written record would print whole.
#[test]
fn an_extra_key_prints_the_record_type_by_its_name() {
    let env = Env::default();
    let messages = |src: &str| -> Vec<String> {
        let e = env
            .check(&parse(src).unwrap(), &Expected::None)
            .err()
            .unwrap();
        e.diagnostics.into_iter().map(|d| d.message).collect()
    };
    let lines = "type Rt0 = {name: string, files: Rt1[], dirs: Rt0[]}; \
                 type Rt1 = {name: string, parent: Rt0 | null};";
    assert_eq!(
        messages(&format!("{lines} const v: Rt0 = {{x_1: 1}}; 0")),
        [
            r#"required field "dirs" has no entry"#,
            r#"Rt0 does not allow key "x_1""#
        ]
    );
    assert_eq!(
        messages(&format!(
            "{lines} const v: Rt0 | null = {{name: \"a\", files: [], dirs: [], x: 1}}; 0"
        )),
        [r#"Rt0 does not allow key "x""#]
    );
}

/// The named types of [`chains`], for an environment.
fn chain_env(n: usize) -> crate::EnvBuilder {
    let mut b = Env::builder();
    for p in ["A", "B"] {
        for i in 0..n {
            b = b.named_type(&format!("{p}{i}"), &format!("{{n: {p}{}}}", (i + 1) % n));
        }
    }
    b
}

/// Equal visits two records' keys in code point order (F17 12.5 item 6): at
/// `a` before the key `z` that one lacks. So `{a: A0, z: int}` and `{a: B0,
/// zz: int}` enter the pairs of the two chains, and pass the bound, before
/// the keys differ. A union the type API builds is invalid when the Join of
/// its records passes the bound.
#[test]
fn equal_compares_a_key_before_a_later_key_that_differs() {
    let env = chain_env(4100)
        .named_type("Xa", "{a: A0, z: int}")
        .named_type("Xb", "{a: B0, zz: int}")
        .build()
        .unwrap();
    let names = &env.data().names;
    let (xa, xb) = (names.lookup("Xa").unwrap(), names.lookup("Xb").unwrap());
    let w = crate::check::ty::watch();
    assert!(!crate::check::ty::equal(&xa, &xb));
    assert!(w.passed(), "the comparison stopped at the keys");
    drop(w);

    use crate::typ::{literal_type, record_type, tagged_union_type, FieldType};
    let variant = |tag: &str, v: &str| {
        let field = |key: &str, ty| FieldType {
            key: key.into(),
            ty,
            optional: false,
        };
        record_type(
            &[
                field("kind", literal_type(&[tag])),
                field("v", env.parse_type(v).unwrap()),
            ],
            &crate::typ::Type::NEVER,
        )
    };
    let u = tagged_union_type(&[variant("a", "A0"), variant("b", "B0")]);
    assert!(u.ty().is_err(), "{u}");
    let u = crate::typ::union_type(&[variant("a", "A0"), variant("b", "B0")]);
    assert!(u.ty().is_err(), "{u}");
    let u = tagged_union_type(&[variant("a", "A0"), variant("b", "A0")]);
    assert!(u.ty().is_ok(), "{u}");
}

/// A union the type API builds holds the tables of the recursive types its
/// variants read through a reference, and outlives the types it came from.
/// Each constructor is tested alone, with a fresh table: the input dropped,
/// conformance with only the union alive, then the table released with it.
#[test]
fn a_built_union_holds_its_recursive_tables() {
    use crate::typ::{
        literal_type, record_type, tagged_union_type, type_from_schema, union_type, FieldType, Type,
    };
    let node = |next: Option<Data>| {
        let mut m = std::collections::BTreeMap::new();
        m.insert("kind".to_string(), Data::Str("a".into()));
        if let Some(n) = next {
            m.insert("next".to_string(), n);
        }
        Data::Map(m)
    };
    let data = node(Some(node(Some(node(None)))));
    type Make = fn(&[Type]) -> Type;
    let build: [(&str, Make); 2] = [
        ("tagged_union_type", tagged_union_type),
        ("union_type", union_type),
    ];
    for (name, make) in build {
        let a = type_from_schema(
            r##"{"$defs": {"node": {"type": "object", "properties": {"kind": {"const": "a"},
                "next": {"$ref": "#/$defs/node"}}, "required": ["kind"],
                "additionalProperties": false}}, "$ref": "#/$defs/node"}"##,
            Default::default(),
        )
        .unwrap();
        assert_eq!(a.to_string(), "Node");
        let b = record_type(
            &[FieldType {
                key: "kind".into(),
                ty: literal_type(&["b"]),
                optional: false,
            }],
            &Type::NEVER,
        );
        let weak = std::sync::Arc::downgrade(
            &crate::check::ty::reach(std::iter::once(a.ty().unwrap()))[0]
                .table()
                .unwrap(),
        );
        let u = make(&[a, b]);
        assert_eq!(
            u.to_string(),
            r#"{kind: "a", next?: Node} | {kind: "b"}"#,
            "{name}"
        );
        assert!(
            crate::conform(&crate::Value::from_data(&data), &u).is_ok(),
            "{name}"
        );
        assert!(weak.upgrade().is_some(), "{name}: the table was let go");
        drop(u);
        assert!(weak.upgrade().is_none(), "{name}: the table was kept");
    }
}

/// has-opaque reads each variant of a tagged union: the flat record joins
/// `Token` and `any` to `any`, so a rendering of it is `kind`, not a
/// `dynamic` site (`05-types.md` 7.3).
#[test]
fn has_opaque_reads_the_variants_of_a_union() {
    let env = Env::builder()
        .opaque("Token")
        .var_record(r#"{x: {tag: "a", value: Token} | {tag: "b", value: any}}"#)
        .build()
        .unwrap();
    let ty = env.data().var_types[0].clone();
    assert!(crate::check::ty::has_opaque(&ty));
    let e = env
        .check(&parse("`${x}`").unwrap(), &Expected::None)
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "kind");
}

/// A host's named type names only declared opaque types (`05-host-api.md`
/// E.7), at any depth, in a union's variants and through references.
#[test]
fn a_host_named_type_names_declared_opaque_types() {
    use crate::typ::{list_type, opaque_type_named};
    let tok = list_type(&opaque_type_named("Token"));
    let e = Env::builder()
        .named_type_of("Alias", &tok)
        .build()
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "bad-type");
    assert!(Env::builder()
        .opaque("Token")
        .named_type_of("Alias", &tok)
        .build()
        .is_ok());
    // Through a reference: a recursive type that holds the opaque type.
    let host = Env::builder()
        .opaque("Token")
        .named_type("Cell", "{t: Token, next: Cell | null}")
        .build()
        .unwrap();
    let cell = host.parse_type("Cell").unwrap();
    let e = Env::builder()
        .named_type_of("Alias", &cell)
        .build()
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "bad-type");
}

/// The lines `R0` to `R{k}`, each a record of two fields of the line
/// before, and a recursive `Xs` whose top reaches `Tree` through `Alias`,
/// a line that is not recursive.
fn doubling(k: usize) -> Vec<(String, String)> {
    let mut v = vec![("R0".to_string(), "int".to_string())];
    for i in 1..=k {
        v.push((format!("R{i}"), format!("{{a: R{0}, b: R{0}}}", i - 1)));
    }
    v.push(("Tree".into(), format!("{{kids: Tree[], payload: R{k}}}")));
    v.push(("Alias".into(), "Tree".into()));
    v.push((
        "Xs".into(),
        format!("{{again: Xs, payload: R{k}}}[] | Alias"),
    ));
    v
}

/// F17 12.6 item 1: a definition is tested for size with the references at
/// its top replaced (12.2 item 4). `Xs` reaches `Tree`'s definition through
/// `Alias`: its text is under the limit and its definition over it.
#[test]
fn a_definition_is_tested_for_size_once_its_top_is_replaced() {
    for (k, over) in [(14, false), (15, true)] {
        let lines = doubling(k);
        let src: String = lines
            .iter()
            .map(|(n, t)| format!("type {n} = {t}; "))
            .collect::<String>()
            + "1";
        let r = Env::default().check(&parse(&src).unwrap(), &Expected::None);
        match r {
            Ok(_) => assert!(!over, "k = {k}: the lines check"),
            Err(e) => {
                assert!(over, "k = {k}: {:?}", e.diagnostics);
                assert_eq!(e.diagnostics.len(), 1);
                assert_eq!(e.diagnostics[0].code, "check-limit");
                assert_eq!(
                    e.diagnostics[0].message,
                    crate::messages::build("check-limit", "size", &[])
                );
            }
        }
        let mut b = Env::builder();
        for (n, t) in &lines {
            b = b.named_type(n, t);
        }
        match b.build() {
            Ok(_) => assert!(!over, "k = {k}: the environment builds"),
            Err(e) => {
                assert!(over, "k = {k}: {:?}", e.diagnostics);
                assert!(e.diagnostics.iter().all(|d| d.code == "bad-type"));
            }
        }
    }
}

/// The type of `p ? a : q ? b : u` is Join(A, Join(B, U)); with `u: any`
/// that is `any` at once, and no Join of A and B is made. Rings of 64 and
/// 65 records would take 4,160 pairs to join.
#[test]
fn a_chain_of_conditionals_joins_only_from_its_end() {
    let ring = |p: &str, n: usize| -> String {
        (0..n)
            .map(|i| format!("type {p}{i} = {{next: {p}{}}}; ", (i + 1) % n))
            .collect()
    };
    let env = Env::builder()
        .var_record("{p: bool, q: bool, u: any, x: any}")
        .build()
        .unwrap();
    let src = format!(
        "{}{}const a = x as P0; const b = x as Q0; p ? a : q ? b : u",
        ring("P", 64),
        ring("Q", 65)
    );
    let r = env.check(&parse(&src).unwrap(), &Expected::None).unwrap();
    assert_eq!(r.ty.to_string(), "any");
    // The Join that the chain does make passes the bound.
    let src = format!(
        "{}{}const a = x as P0; const b = x as Q0; p ? a : b",
        ring("P", 64),
        ring("Q", 65)
    );
    let e = env
        .check(&parse(&src).unwrap(), &Expected::None)
        .err()
        .unwrap();
    assert_eq!(e.diagnostics[0].code, "check-limit");
}

/// The families of the case "the bound: a join in a chain of ? : is on the
/// innermost conditional": `A0` to `A12` and `B0` to `B12`, each record two
/// lists of the next. A Join of `A0` and `B0` passes the bound of pairs.
fn families(b: crate::EnvBuilder) -> crate::EnvBuilder {
    let mut b = b;
    for p in ["A", "B"] {
        for i in 0..13 {
            let next = format!("{p}{}", (i + 1) % 13);
            b = b.named_type(&format!("{p}{i}"), &format!("{{x: {next}[], y: {next}[]}}"));
        }
    }
    b
}

/// F17 12.5 item 7 (R317): the bound passed in a host signature is
/// `bad-signature`; in a variable's type it is `bad-type`. A union of two
/// records whose fields join past the bound passes it as it is declared.
#[test]
fn the_bound_in_a_signature_is_bad_signature() {
    let sig = r#"(v: {k: "a", t: A0} | {k: "b", t: B0}) => int"#;
    let e = families(Env::builder())
        .function("f", sig, |_, _| Ok(Data::Int(1)))
        .build()
        .err()
        .unwrap();
    let codes: Vec<&str> = e.diagnostics.iter().map(|d| d.code).collect();
    assert_eq!(codes, ["bad-signature"]);
    let e = families(Env::builder())
        .var_record(r#"{v: {k: "a", t: A0} | {k: "b", t: B0}}"#)
        .build()
        .err()
        .unwrap();
    let codes: Vec<&str> = e.diagnostics.iter().map(|d| d.code).collect();
    assert_eq!(codes, ["bad-type"]);
    // One family on both sides joins at once.
    assert!(families(Env::builder())
        .function(
            "f",
            r#"(v: {k: "a", t: A0} | {k: "b", t: A0}) => int"#,
            |_, _| { Ok(Data::Int(1)) }
        )
        .build()
        .is_ok());
}

/// F4 3.5: a type line is tested for depth (step 5) before size (step 6), so
/// one both deeper than 64 and over the size limit gives `depth`. So does
/// a recursive line, and an environment's named type says why it is
/// `bad-type` by its depth.
#[test]
fn depth_is_tested_before_size() {
    let mut lines = vec![("R0".to_string(), "int".to_string())];
    for i in 1..=15 {
        lines.push((format!("R{i}"), format!("{{a: R{0}, b: R{0}}}", i - 1)));
    }
    let deep = format!("int{}", "[]".repeat(64));
    for (name, ty) in [
        ("Xx", format!("{{a: R15, b: R15, c: {deep}}}")),
        ("Yy", format!("{{a: R15, b: R15, c: {deep}, s: Yy[]}}")),
    ] {
        let mut all = lines.clone();
        all.push((name.to_string(), ty));
        let src: String = all
            .iter()
            .map(|(n, t)| format!("type {n} = {t}; "))
            .collect::<String>()
            + "1";
        let e = Env::default()
            .check(&parse(&src).unwrap(), &Expected::None)
            .err()
            .unwrap();
        assert_eq!(e.diagnostics.len(), 1, "{name}: {:?}", e.diagnostics);
        assert_eq!(e.diagnostics[0].code, "check-limit");
        assert_eq!(
            e.diagnostics[0].message,
            crate::messages::build("check-limit", "depth", &[]),
            "{name}"
        );
        let mut b = Env::builder();
        for (n, t) in &all {
            b = b.named_type(n, t);
        }
        let e = b.build().err().unwrap();
        assert_eq!(e.diagnostics.len(), 1, "{name}: {:?}", e.diagnostics);
        assert_eq!(e.diagnostics[0].code, "bad-type");
        assert!(
            e.diagnostics[0].message.contains("deeper than 64"),
            "{name}: {}",
            e.diagnostics[0].message
        );
    }
}

/// F5 6 (R319): a map literal with no entry that surely gives the tag of a
/// union is `missing`, with the tag as its key, at a result and at an
/// argument alike.
#[test]
fn a_literal_with_no_tag_names_the_missing_tag() {
    let messages = |src: &str| -> Vec<(&'static str, String)> {
        let e = Env::default()
            .check(&parse(src).unwrap(), &Expected::None)
            .err()
            .unwrap();
        e.diagnostics
            .into_iter()
            .map(|d| (d.code, d.message))
            .collect()
    };
    let line = r#"type Rt0 = {k: "num", v: float} | {k: "add", l: int, r: int};"#;
    let want = r#"required field "k" has no entry"#.to_string();
    assert_eq!(
        messages(&format!("{line} const v: Rt0 = {{}}; 0")),
        [("result-mismatch", want.clone())]
    );
    assert_eq!(
        messages(&format!(
            "{line} const f = (x: Rt0): int => 0; f({{v: 1.5}})"
        )),
        [("arg-mismatch", want)]
    );
}
