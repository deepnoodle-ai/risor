//! Package F10: the load of a config file (`f10-config.md` 5).
//!
//! A load makes the steps of 5.2 in order and stops at the first that
//! fails: 0 declare (the expected type), 1 parse, 2 check and compile, 3 run,
//! 4 conform (every failure, up to a maximum), and 5 decode, when a host type
//! is given. Only step 3 charges. The value is copied into [`Data`]: a load
//! gives no value that borrows the variables (5.5 item 3).
//!
//! ```
//! use risor::{load_value, Env, Expected, LoadOptions};
//! let env = Env::builder().var("env", "map<string>").build().unwrap();
//! let mut vars = std::collections::BTreeMap::new();
//! vars.insert("env".to_string(), risor::Data::Map(Default::default()));
//! let opts = LoadOptions {
//!     expected: Expected::text("{port: int}"),
//!     max: 1,
//!     ..LoadOptions::default()
//! };
//! let got = load_value(&env, "{port: 8080}", &vars, opts).unwrap();
//! assert!(matches!(got, risor::Data::Map(_)));
//! ```

use crate::api::Expected;
use crate::check::{compile_checked, Want};
use crate::decode::Decode;
use crate::diag::Diagnostic;
use crate::env::Env;
use crate::eval::{RunOptions, Usage};
use crate::host::{Data, Vars};
use crate::hosttype::HostType;
use crate::syntax::{ParseOptions, SOURCE_LIMIT};

/// Load step 0: declare (resolving the expected type and options).
pub const STEP_DECLARE: u8 = 0;
/// Load step 1: parse.
pub const STEP_PARSE: u8 = 1;
/// Load step 2: check and compile.
pub const STEP_CHECK: u8 = 2;
/// Load step 3: run.
pub const STEP_RUN: u8 = 3;
/// Load step 4: conform.
pub const STEP_CONFORM: u8 = 4;
/// Load step 5: decode.
pub const STEP_DECODE: u8 = 5;

/// The options of a load (`f10-config.md` 5.1).
#[derive(Clone, Debug)]
pub struct LoadOptions {
    /// The type T of the value: type text, which may name the
    /// environment's named types; a JSON Schema, which a host maps with the
    /// option `closed` and no other (5.6); or a type. [`Expected::None`] is
    /// `any`, and [`Expected::Condition`] is no type: step 0 fails.
    pub expected: Expected,
    /// The most `not-conforming` diagnostics step 4 reports; 0 is read as 1.
    pub max: usize,
    /// The source limit (5.1), which step 1 applies; by default
    /// [`SOURCE_LIMIT`]. A limit out of range is `bad-limit`, at step 0.
    pub source_limit: usize,
    /// Run options for step 3 (for example cancellation).
    pub run: RunOptions,
}

impl Default for LoadOptions {
    fn default() -> LoadOptions {
        LoadOptions {
            expected: Expected::None,
            max: 1,
            source_limit: SOURCE_LIMIT,
            run: RunOptions::default(),
        }
    }
}

/// What a load gives on success: the value and the usage of its run.
#[derive(Clone, Debug)]
pub(crate) struct Loaded<T> {
    pub value: T,
    pub usage: Usage,
}

/// The failure of a load: the step that failed (0 to 5), its diagnostics,
/// and the usage when the run was reached (5.2 item 4).
#[derive(Clone, Debug)]
pub struct LoadError {
    /// The load step that failed: [`STEP_DECLARE`] (0) through [`STEP_DECODE`] (5).
    pub step: u8,
    /// The diagnostics reported at `step`.
    pub diagnostics: Vec<Diagnostic>,
    /// The usage of step 3 when the run was reached; `None` if stopped earlier.
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

/// Load a config file into a value (`f10-config.md` 5): steps 0 to 4. The
/// value is copied.
pub fn load_value(
    env: &Env,
    text: &str,
    vars: &dyn Vars,
    opts: LoadOptions,
) -> Result<Data, LoadError> {
    // Step 0.
    let parse = ParseOptions {
        source_limit: opts.source_limit,
    };
    parse
        .limit()
        .map_err(|e| fail(STEP_DECLARE, e.diagnostics, None))?;
    let ty = match opts
        .expected
        .want(env)
        .map_err(|e| fail(STEP_DECLARE, e.diagnostics, None))?
    {
        Want::Type(t) => t,
        Want::Nothing => crate::check::ty::Ty::any(),
        Want::Condition => {
            return Err(fail(
                STEP_DECLARE,
                vec![Diagnostic::bare(
                    crate::diag::Phase::Declare,
                    crate::diag::code::BAD_TYPE,
                    "a load expects a type, and a condition is not one",
                )],
                None,
            ))
        }
    };
    run_steps(env, text, parse, ty, vars, opts.max, opts.run).map(|l| l.value)
}

fn run_steps(
    env: &Env,
    text: &str,
    parse: ParseOptions,
    ty: crate::check::ty::Ty,
    vars: &dyn Vars,
    max: usize,
    run: RunOptions,
) -> Result<Loaded<Data>, LoadError> {
    // Step 1.
    let parsed = crate::syntax::parse_with(text, parse)
        .map_err(|e| fail(STEP_PARSE, e.diagnostics, None))?;
    if !parsed.has_result() {
        return Err(fail(
            STEP_DECLARE,
            vec![Diagnostic::bare(
                crate::diag::Phase::Declare,
                crate::diag::code::NO_RESULT,
                "the program has no result expression",
            )],
            None,
        ));
    }
    // Step 2: always with the check.
    let (program, _) = compile_checked(env, parsed, &Want::Type(ty.clone()))
        .map_err(|e| fail(STEP_CHECK, e.diagnostics, None))?;
    // Step 3.
    let out = program.outcome_with(vars, run);
    let usage = out.usage;
    let v = out
        .result
        .map_err(|f| fail(STEP_RUN, vec![program.diagnostic(&f)], Some(usage)))?;
    // Step 4: every failure, up to the maximum; not catchable, not part of
    // the run's usage. It counts steps against the default limit (F10 5.2
    // item 1, F8 6.4).
    let all = crate::conform::all(&v, &ty, max.max(1), Some(crate::conform::FREE_STEPS));
    if !all.is_empty() {
        return Err(fail(
            STEP_CONFORM,
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
pub fn load<H: Decode + HostType>(
    env: &Env,
    text: &str,
    vars: &dyn Vars,
    max: usize,
) -> Result<H, LoadError> {
    load_with(
        env,
        text,
        vars,
        LoadOptions {
            max,
            ..LoadOptions::default()
        },
    )
}

/// [`load`] with [`LoadOptions`]: `max`, `source_limit` (step 1) and `run`
/// (step 3 cancellation). `opts.expected` is ignored: the expected type is
/// always derived from `H`. A `source_limit` out of range is `bad-limit`, at
/// step 0.
pub fn load_with<H: Decode + HostType>(
    env: &Env,
    text: &str,
    vars: &dyn Vars,
    opts: LoadOptions,
) -> Result<H, LoadError> {
    let parse = ParseOptions {
        source_limit: opts.source_limit,
    };
    parse
        .limit()
        .map_err(|e| fail(STEP_DECLARE, e.diagnostics, None))?;
    let ty = crate::hosttype::type_of::<H>().map_err(|e| {
        fail(
            STEP_DECLARE,
            vec![Diagnostic::bare(
                crate::diag::Phase::Declare,
                crate::diag::code::BAD_TYPE,
                e.0,
            )],
            None,
        )
    })?;
    let got = run_steps(env, text, parse, ty.into_ty(), vars, opts.max, opts.run)?;
    let v = crate::value::Value::from_data(&got.value);
    // Step 5.
    crate::decode::decode_max::<H>(&v, opts.max).map_err(|d| fail(STEP_DECODE, d, Some(got.usage)))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::schema::SchemaOptions;
    use std::collections::BTreeMap;

    fn vars(s: &str) -> BTreeMap<String, Data> {
        let mut m = BTreeMap::new();
        m.insert("s".to_string(), Data::Str(s.to_string()));
        m
    }

    fn env() -> Env {
        Env::builder()
            .var("s", "string")
            .groups(&["json"])
            .build()
            .unwrap()
    }

    #[test]
    fn the_steps_in_order() {
        let e = env();
        let opts = |expected: Expected, max: usize| LoadOptions {
            expected,
            max,
            ..LoadOptions::default()
        };
        let r = load_value(
            &e,
            "{port: 1}",
            &vars(""),
            opts(Expected::text("{port: int}"), 1),
        );
        assert_eq!(
            r.unwrap(),
            Data::Map([("port".into(), Data::Float(1.0))].into())
        );
        let fails = |src: &str, exp: Expected, s: &str| load_value(&e, src, &vars(s), opts(exp, 3));
        assert_eq!(
            fails("{port: 1}", Expected::text("Nope"), "")
                .unwrap_err()
                .step,
            0
        );
        assert_eq!(
            fails("{port: ", Expected::text("{port: int}"), "")
                .unwrap_err()
                .step,
            1
        );
        assert_eq!(
            fails("{prot: 1}", Expected::text("{port: int}"), "")
                .unwrap_err()
                .step,
            2
        );
        assert_eq!(
            fails(
                "1 / 0 > 1 ? {port: 1} : {port: 2}",
                Expected::text("{port: int}"),
                ""
            )
            .unwrap_err()
            .step,
            2
        );
        let e3 = fails("{port: int(s)}", Expected::text("{port: int}"), "x").unwrap_err();
        assert_eq!(e3.step, 3);
        assert!(e3.usage.is_some());
        // Step 4: through `any`, and not catchable.
        let e4 = fails(
            "try { json.parse(s) } catch { {port: 0} }",
            Expected::text("{port: int}"),
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
        let s = Expected::schema(
            r#"{"type": "object", "properties": {"port": {"type": "integer"}}, "required": ["port"]}"#,
            SchemaOptions {
                closed: true,
                ..SchemaOptions::default()
            },
        )
        .unwrap();
        let opts = |expected: Expected| LoadOptions {
            expected,
            max: 1,
            ..LoadOptions::default()
        };
        let r = load_value(&e, "{port: 1, prot: 2}", &vars(""), opts(s.clone()));
        assert_eq!(r.unwrap_err().step, 2);
        let r = load_value(
            &e,
            "json.parse(s)",
            &vars(r#"{"port": 1, "prot": 2}"#),
            opts(s),
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
        let got = load::<BTreeMap<String, u8>>(&e, "{a: 1, b: 2}", &vars(""), 1).unwrap();
        assert_eq!(got.get("b"), Some(&2));
        let err = load::<BTreeMap<String, u8>>(&e, "{a: 300}", &vars(""), 1).unwrap_err();
        assert_eq!(err.step, 5);
    }

    #[test]
    fn no_result_program_fails_step_declare() {
        let e = Env::default();
        let src = "export const f = (x: int) => x + 1;";
        let err = load_value(&e, src, &crate::NoVars, LoadOptions::default()).unwrap_err();
        assert_eq!(err.step, STEP_DECLARE);
        assert_eq!(err.diagnostics[0].phase, crate::Phase::Declare);
        assert_eq!(err.diagnostics[0].code, "no-result");

        let err = load::<i64>(&e, src, &crate::NoVars, 1).unwrap_err();
        assert_eq!(err.step, STEP_DECLARE);
        assert_eq!(err.diagnostics[0].phase, crate::Phase::Declare);
        assert_eq!(err.diagnostics[0].code, "no-result");
    }
}
