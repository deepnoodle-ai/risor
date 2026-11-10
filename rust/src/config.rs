//! Package F10: the load of a config file (`f10-config.md` 5).
//!
//! A load makes the steps of 5.2 in order and stops at the first that
//! fails: 0 declare (the expected type), 1 parse, 2 check and compile, 3 run,
//! 4 conform (every failure, up to a maximum), and 5 decode, when a host type
//! is given. Only step 3 charges. The value is copied into [`Data`]: a load
//! gives no value that borrows the variables (5.5 item 3).
//!
//! ```
//! use risor::config::{load, Expected};
//! let env = risor::Env::builder().var("env", "map<string>").build().unwrap();
//! let mut vars = std::collections::BTreeMap::new();
//! vars.insert("env".to_string(), risor::Data::Map(Default::default()));
//! let got = load(&env, "{port: 8080}", Expected::Text("{port: int}"), &vars, 1).unwrap();
//! assert_eq!(got.usage.steps, 0);
//! ```

use crate::check::{compile_checked, expected_schema, Expected as Exp};
use crate::decode::Decode;
use crate::diag::Diagnostic;
use crate::env::Env;
use crate::eval::Usage;
use crate::host::{Data, Vars};
use crate::hosttype::HostType;
use crate::json::Json;
use crate::schema::SchemaOptions;

/// The expected type of a load (`f10-config.md` 5.1).
pub enum Expected<'a> {
    /// Type text, which may name the environment's named types.
    Text(&'a str),
    /// A JSON Schema, mapped with the option `closed` and no other (5.6).
    Schema(&'a Json),
}

/// What a load gives on success: the value and the usage of its run.
#[derive(Clone, Debug)]
pub struct Loaded<T> {
    pub value: T,
    pub usage: Usage,
}

/// The failure of a load: the step that failed (0 to 5), its diagnostics,
/// and the usage when the run was reached (5.2 item 4).
#[derive(Clone, Debug)]
pub struct LoadError {
    pub step: u8,
    pub diagnostics: Vec<Diagnostic>,
    pub usage: Option<Usage>,
}

impl std::fmt::Display for LoadError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "the load failed at step {}: {}",
            self.step,
            crate::render::render_line(&self.diagnostics).trim_end()
        )
    }
}

impl std::error::Error for LoadError {}

fn fail(step: u8, diagnostics: Vec<Diagnostic>, usage: Option<Usage>) -> LoadError {
    LoadError {
        step,
        diagnostics,
        usage,
    }
}

/// Steps 0 to 4: the value, copied, and the usage.
pub fn load(
    env: &Env,
    text: &str,
    expected: Expected<'_>,
    vars: &dyn Vars,
    max: usize,
) -> Result<Loaded<Data>, LoadError> {
    // Step 0.
    let ty = match expected {
        Expected::Text(t) => env.ty(t).map_err(|e| fail(0, e.diagnostics, None))?.ty,
        Expected::Schema(s) => {
            let opts = SchemaOptions {
                closed: true,
                ..SchemaOptions::default()
            };
            match expected_schema(s, opts).map_err(|d| fail(0, vec![d], None))? {
                Exp::Type(t) => t,
                _ => unreachable!("a schema maps to a type"),
            }
        }
    };
    run_steps(env, text, ty, vars, max)
}

fn run_steps(
    env: &Env,
    text: &str,
    ty: crate::check::ty::Ty,
    vars: &dyn Vars,
    max: usize,
) -> Result<Loaded<Data>, LoadError> {
    // Step 1.
    let parsed = crate::syntax::parse(text).map_err(|e| fail(1, e.diagnostics, None))?;
    // Step 2: always with the check.
    let (program, _) = compile_checked(env, parsed, &Exp::Type(ty.clone()))
        .map_err(|e| fail(2, e.diagnostics, None))?;
    // Step 3.
    let out = program.run(vars);
    let usage = out.usage;
    let v = out
        .result
        .map_err(|f| fail(3, vec![program.diagnostic(&f)], Some(usage)))?;
    // Step 4: every failure, up to the maximum; not catchable, no charge.
    let all = crate::conform::all(&v, &ty, max.max(1));
    if !all.is_empty() {
        return Err(fail(
            4,
            all.iter().map(|f| f.diagnostic()).collect(),
            Some(usage),
        ));
    }
    Ok(Loaded {
        value: v.to_data(),
        usage,
    })
}

/// Steps 0 to 5 with T = D(H), the type derived from `H` (F1 6): the value
/// decoded into `H`.
pub fn load_into<H: Decode + HostType>(
    env: &Env,
    text: &str,
    vars: &dyn Vars,
    max: usize,
) -> Result<Loaded<H>, LoadError> {
    let ty = crate::hosttype::type_of::<H>().map_err(|e| {
        fail(
            0,
            vec![Diagnostic::bare(
                crate::diag::Phase::Declare,
                crate::diag::code::BAD_TYPE,
                e.0,
            )],
            None,
        )
    })?;
    let got = run_steps(env, text, ty, vars, max)?;
    let v = crate::value::Value::from_data(&got.value);
    // Step 5.
    let value = crate::decode::decode::<H>(&v, max).map_err(|d| fail(5, d, Some(got.usage)))?;
    Ok(Loaded {
        value,
        usage: got.usage,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    fn vars(s: &str) -> BTreeMap<String, Data> {
        let mut m = BTreeMap::new();
        m.insert("s".to_string(), Data::Str(s.to_string()));
        m
    }

    fn env() -> Env {
        Env::builder()
            .var("s", "string")
            .group("json")
            .build()
            .unwrap()
    }

    #[test]
    fn the_steps_in_order() {
        let e = env();
        let t = Expected::Text("{port: int}");
        let r = load(&e, "{port: 1}", t, &vars(""), 1).unwrap();
        assert_eq!(
            r.value,
            Data::Map([("port".into(), Data::Float(1.0))].into())
        );
        let fails = |src: &str, exp: Expected<'_>, s: &str| load(&e, src, exp, &vars(s), 3);
        assert_eq!(
            fails("{port: 1}", Expected::Text("Nope"), "")
                .unwrap_err()
                .step,
            0
        );
        assert_eq!(
            fails("{port: ", Expected::Text("{port: int}"), "")
                .unwrap_err()
                .step,
            1
        );
        assert_eq!(
            fails("{prot: 1}", Expected::Text("{port: int}"), "")
                .unwrap_err()
                .step,
            2
        );
        assert_eq!(
            fails(
                "1 / 0 > 1 ? {port: 1} : {port: 2}",
                Expected::Text("{port: int}"),
                ""
            )
            .unwrap_err()
            .step,
            2
        );
        let e3 = fails("{port: int(s)}", Expected::Text("{port: int}"), "x").unwrap_err();
        assert_eq!(e3.step, 3);
        assert!(e3.usage.is_some());
        // Step 4: through `any`, and not catchable.
        let e4 = fails(
            "try { json.parse(s) } catch { {port: 0} }",
            Expected::Text("{port: int}"),
            r#"{"port": "x", "debug": true}"#,
        )
        .unwrap_err();
        assert_eq!(e4.step, 4);
        assert_eq!(e4.diagnostics.len(), 2);
        assert!(e4.diagnostics[0].span.is_none());
    }

    #[test]
    fn a_schema_is_closed() {
        let e = env();
        let s = crate::json::parse(
            r#"{"type": "object", "properties": {"port": {"type": "integer"}}, "required": ["port"]}"#,
        )
        .unwrap();
        let r = load(&e, "{port: 1, prot: 2}", Expected::Schema(&s), &vars(""), 1);
        assert_eq!(r.unwrap_err().step, 2);
        let r = load(
            &e,
            "json.parse(s)",
            Expected::Schema(&s),
            &vars(r#"{"port": 1, "prot": 2}"#),
            1,
        );
        let err = r.unwrap_err();
        assert_eq!(err.step, 4);
        assert_eq!(
            err.diagnostics[0].message,
            "the type does not allow key \"prot\""
        );
    }

    #[test]
    fn into_a_host_type() {
        let e = env();
        let got = load_into::<BTreeMap<String, u8>>(&e, "{a: 1, b: 2}", &vars(""), 1).unwrap();
        assert_eq!(got.value.get("b"), Some(&2));
        let err = load_into::<BTreeMap<String, u8>>(&e, "{a: 300}", &vars(""), 1).unwrap_err();
        assert_eq!(err.step, 5);
    }
}
