//! The steps a host takes with a program (`05-host-api.md` E.4, E.6, E.8):
//! the expected result, check, compile, run and measure, and the tools that
//! read a program's text.

use crate::assist::{Completion, Hover};
use crate::check::ty::Ty;
use crate::check::{self, Site, Want};
use crate::compile::Program;
use crate::diag::{Diagnostic, Error, Phase};
use crate::env::Env;
use crate::eval::{RunOptions, Usage};
use crate::host::Vars;
use crate::schema::SchemaOptions;
use crate::source::{Pos, Source};
use crate::syntax::Expr;
use crate::typ::Type;
use crate::value::Value;

/// What the host expects of a program's result (`03-host.md` 3.2.3): a
/// type, a condition, or nothing.
#[derive(Clone, Debug, PartialEq)]
pub struct Expected(Exp);

#[derive(Clone, Debug, PartialEq)]
enum Exp {
    Nothing,
    Condition,
    /// Type text, read against the environment of the check.
    Text(String),
    Type(Ty),
    /// An invalid type: the message of `bad-type`.
    Bad(String),
}

impl Default for Expected {
    fn default() -> Expected {
        Expected::None
    }
}

impl Expected {
    /// Any result is accepted.
    #[allow(non_upper_case_globals)]
    pub const None: Expected = Expected(Exp::Nothing);

    /// The result will be tested for truthiness ([`truthy`]).
    #[allow(non_upper_case_globals)]
    pub const Condition: Expected = Expected(Exp::Condition);

    /// A type, by type text, which may name the environment's opaque and
    /// named types. The text `condition` is [`Expected::Condition`]. Text
    /// that does not read as a type is the `declare` error `bad-type` when
    /// a check or compile uses it.
    pub fn text(text: &str) -> Expected {
        Expected(Exp::Text(text.to_string()))
    }

    /// A type, by a JSON Schema given as JSON text (`f10-config.md` 2.1).
    /// Only the option `closed` is accepted: nobody supplies a result. An
    /// error is the `declare` error `bad-type`.
    pub fn schema(schema: &str, opts: SchemaOptions) -> Result<Expected, Error> {
        let bad = |m: String| {
            Error::one(Diagnostic::bare(
                Phase::Declare,
                crate::diag::code::BAD_TYPE,
                m,
            ))
        };
        let j = crate::json::parse(schema).map_err(bad)?;
        match check::expected_schema(&j, opts).map_err(Error::one)? {
            Want::Type(t) => Ok(Expected(Exp::Type(t))),
            _ => unreachable!("a schema maps to a type"),
        }
    }

    /// A type the host built or parsed.
    pub fn of(t: &Type) -> Expected {
        match t.ty() {
            Ok(ty) => Expected(Exp::Type(ty.clone())),
            Err(m) => Expected(Exp::Bad(m)),
        }
    }

    /// The expected result as the checker reads it, in an environment.
    pub(crate) fn want(&self, env: &Env) -> Result<Want, Error> {
        Ok(match &self.0 {
            Exp::Nothing => Want::Nothing,
            Exp::Condition => Want::Condition,
            Exp::Text(t) => check::expected(env, Some(t)).map_err(Error::one)?,
            Exp::Type(t) => Want::Type(t.clone()),
            Exp::Bad(m) => {
                return Err(Error::one(Diagnostic::bare(
                    Phase::Declare,
                    crate::diag::code::BAD_TYPE,
                    m.clone(),
                )))
            }
        })
    }
}

/// What a check gives (`03-host.md` 3.2.1): the program's type, and the
/// places where it can still fail on a value, for the classes the
/// environment does not reject.
#[derive(Clone, Debug, PartialEq)]
pub struct CheckResult {
    pub ty: Type,
    pub sites: Vec<Site>,
}

/// The options of a compile.
#[derive(Clone, Debug, PartialEq)]
pub struct CompileOptions {
    /// Compile with the check (`03-host.md` 3.2.1 item 3), the default.
    /// Without it, only the static rules apply and a run reports classes by
    /// the column "not checked" (item 4).
    pub check: bool,
    /// The expected result. Without the check it is not read.
    pub expected: Expected,
}

impl Default for CompileOptions {
    fn default() -> CompileOptions {
        CompileOptions {
            check: true,
            expected: Expected::None,
        }
    }
}

impl Env {
    /// Check a program (`03-host.md` 3.2.1). A program that fails the check
    /// gives its diagnostics, ordered by 6.1, as an error of phase `check`.
    pub fn check(&self, x: &Expr, expected: &Expected) -> Result<CheckResult, Error> {
        let want = expected.want(self)?;
        let r = check::check(self, x, &want);
        if !r.diagnostics.is_empty() {
            return Err(Error {
                phase: Phase::Check,
                diagnostics: r.diagnostics,
            });
        }
        Ok(CheckResult {
            ty: Type::from_ty(r.ty.expect("a program that checks has a type")),
            sites: r.sites,
        })
    }

    /// Compile a program. With the check (the default) it fails exactly
    /// when [`Env::check`] does.
    pub fn compile(&self, x: &Expr, options: CompileOptions) -> Result<Program, Error> {
        if !options.check {
            return crate::compile::compile_unchecked(self, x.clone());
        }
        let want = options.expected.want(self)?;
        check::compile_checked(self, x.clone(), &want).map(|(p, _)| p)
    }

    /// Completion at a position of a program's text, which need not parse
    /// (F12 3.5). A position past the end of its line is read as the end of
    /// the line.
    pub fn complete(&self, text: &str, pos: Pos, expected: &Expected) -> Completion {
        let want = expected.want(self).unwrap_or(Want::Nothing);
        let offset = Source::new(text).offset(pos);
        crate::assist::complete(self, text, &want, offset)
    }

    /// Hover at a position of a program's text (F12 3.13); `None` where
    /// there is nothing to say.
    pub fn hover(&self, text: &str, pos: Pos, expected: &Expected) -> Option<Hover> {
        let want = expected.want(self).unwrap_or(Want::Nothing);
        let offset = Source::new(text).offset(pos);
        crate::assist::hover(self, text, &want, offset)
    }
}

impl Program {
    /// Run the program with variables lent for the run: its value, or the
    /// failure, with one `run` diagnostic.
    pub fn run<'h>(&'h self, vars: &'h dyn Vars) -> Result<Value<'h>, Error> {
        self.measure(vars).0
    }

    /// Run with options: a signal that cancels the run.
    pub fn run_with<'h>(
        &'h self,
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> Result<Value<'h>, Error> {
        self.measure_with(vars, opts).0
    }

    /// Run, and give what the run used whether it succeeds or fails
    /// (`03-host.md` 3.6.3).
    pub fn measure<'h>(&'h self, vars: &'h dyn Vars) -> (Result<Value<'h>, Error>, Usage) {
        self.measure_with(vars, RunOptions::default())
    }

    /// Measure with options.
    pub fn measure_with<'h>(
        &'h self,
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> (Result<Value<'h>, Error>, Usage) {
        let out = self.outcome_with(vars, opts);
        let result = out.result.map_err(|f| Error::one(self.diagnostic(&f)));
        (result, out.usage)
    }

    /// The program's check result, when it was compiled with the check.
    pub fn check_result(&self) -> Option<CheckResult> {
        Some(CheckResult {
            ty: Type::from_ty(self.ty.clone()?),
            sites: self.sites.clone()?,
        })
    }
}

/// The truthiness test (`01-language.md` 6.5): `null`, `false`, `0`, `""`,
/// `[]` and `{}` are falsy, and every other value truthy.
pub fn truthy(v: &Value<'_>) -> bool {
    v.truthy()
}

/// The JSON text of a value (`03-host.md` 3.2.4): keys in code point order,
/// numbers in the rendering of `01-language.md`. It fails with the code
/// `wrong-kind` on an opaque value and `depth-exceeded` past depth 200.
pub fn json_text(v: &Value<'_>) -> Result<String, &'static str> {
    crate::eval::to_json(v)
}
