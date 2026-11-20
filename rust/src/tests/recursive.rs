//! Recursive named types (F17 12): what the suite cannot show. A comparison
//! that reaches the bound of pairs through a long chain of types needs no
//! more stack than the crate states; a recursive type leaks nothing; the
//! conformance function stops past the default depth limit.

use super::host::stated_stack;
use crate::check::ty::{print, Ty};
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
