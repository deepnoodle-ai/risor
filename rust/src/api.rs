//! The steps a host takes with a program (`05-host-api.md` E.4, E.6, E.8):
//! the expected result, check, compile, run and measure, and the tools that
//! read a program's text.

use crate::assist::{Completion, Hover};
use crate::check::ty::Ty;
use crate::check::{self, Want};
use crate::compile::Program;
use crate::diag::{Diagnostic, Error, Phase};
use crate::env::Env;
use crate::eval::{RunOptions, Usage};
use crate::host::{Host, Vars};
use crate::schema::SchemaOptions;
use crate::source::{Pos, Source, Span};
use crate::syntax::{Expr, ParseOptions};
use crate::typ::Type;
use crate::value::Value;
use std::collections::BTreeMap;

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

    /// The expected type of an entry point (`f16-entry-points.md` 12.3 item 5):
    /// `condition` is not accepted.
    pub(crate) fn want_entry(&self, env: &Env) -> Result<Ty, Error> {
        let bad = |m: &str| {
            Error::one(Diagnostic::bare(
                Phase::Declare,
                crate::diag::code::BAD_TYPE,
                m,
            ))
        };
        match &self.0 {
            Exp::Nothing => Err(bad("an entry point expected result must name a type")),
            Exp::Condition => Err(bad("condition is not a type for an entry point")),
            Exp::Text(t) => check::expected_type(env, t).map_err(Error::one),
            Exp::Type(t) => Ok(t.clone()),
            Exp::Bad(m) => Err(bad(m)),
        }
    }
}

/// One parameter of an exported entry point (`f16-entry-points.md` 12.6).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct EntryParam {
    /// The parameter's name, or `None` for a destructuring pattern or `_`.
    pub name: Option<String>,
    /// The parameter's annotated type.
    pub ty: Type,
    /// The parameter's description (11.2), or `None`.
    pub doc: Option<String>,
    /// `true` when the parameter has a default.
    pub optional: bool,
}

/// Metadata for one `export const` entry point (`f16-entry-points.md` 12.6).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Entry {
    /// The `const` name of the entry point.
    pub name: String,
    /// The doc comment description attached to `export`, or `None`.
    pub doc: Option<String>,
    /// The entry point's parameters, in written order.
    pub params: Vec<EntryParam>,
    /// The record type a call by record conforms its input to, or `None` when
    /// the entry point has no input record (12.5).
    pub input_type: Option<Type>,
    /// The entry point's checked result type, or `None` for an unchecked compile.
    pub result_type: Option<Type>,
}

/// A place where a checked program can still fail on a value (`05-types.md`
/// 18.3), for a class the environment does not reject.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Site {
    /// The residual check class: `dynamic`, `index`, `divide`, `value` or `host`.
    pub class: &'static str,
    /// The source span of the operation that may fail.
    pub span: Span,
}

fn sites(src: &Source, sites: &[check::Site]) -> Vec<Site> {
    sites
        .iter()
        .map(|s| Site {
            class: s.class,
            span: src.line_span(s.span),
        })
        .collect()
}

/// What a check gives (`03-host.md` 3.2.1): the program's type, the
/// places where it can still fail on a value (for the classes the
/// environment does not reject), and its `export const` entry points.
#[derive(Clone, Debug, PartialEq)]
pub struct CheckResult {
    /// The result type (`never` when the program has no result expression).
    pub ty: Type,
    /// Residual failure sites for check classes the environment does not reject.
    pub sites: Vec<Site>,
    /// F16 12.6: the program's `export const` entry points in text order.
    pub entries: Vec<Entry>,
}

/// The options of a compile.
#[derive(Clone, Debug, PartialEq)]
pub struct CompileOptions {
    /// Compile with the check (`03-host.md` 3.2.1 item 3), the default.
    /// Without it, only the static rules apply and a run reports classes by
    /// the column "not checked" (item 4).
    pub check: bool,
    /// The expected result of the program. On a result-less program, any
    /// expected result fails with `declare` `no-result` (F16 12.3 item 4).
    pub expected: Expected,
    /// Per-entry expected result types (`f16-entry-points.md` 12.3 item 5).
    pub entry_expected: BTreeMap<String, Expected>,
}

impl Default for CompileOptions {
    fn default() -> CompileOptions {
        CompileOptions {
            check: true,
            expected: Expected::None,
            entry_expected: BTreeMap::new(),
        }
    }
}

impl CompileOptions {
    /// Expect the named entry point to return a value fitting `type_text`
    /// (`f16-entry-points.md` 12.3 item 5).
    pub fn expect_entry(mut self, name: impl Into<String>, type_text: &str) -> Self {
        self.entry_expected
            .insert(name.into(), Expected::text(type_text));
        self
    }

    /// Expect the named entry point to return a value fitting `ty`
    /// (`f16-entry-points.md` 12.3 item 5).
    pub fn expect_entry_type(mut self, name: impl Into<String>, ty: &Type) -> Self {
        self.entry_expected.insert(name.into(), Expected::of(ty));
        self
    }

    /// Expect the named entry point to return a value fitting the JSON Schema
    /// `schema` (`f16-entry-points.md` 12.3 item 5).
    pub fn expect_entry_schema(
        mut self,
        name: impl Into<String>,
        schema: &str,
        opts: SchemaOptions,
    ) -> Result<Self, Error> {
        let exp = Expected::schema(schema, opts)?;
        self.entry_expected.insert(name.into(), exp);
        Ok(self)
    }
}

/// Validate `expected` and `entry_expected` for both checked and unchecked
/// compiles (`f16-entry-points.md` 12.3 items 4 and 5).
pub(crate) fn validate_expected(
    env: &Env,
    x: &Expr,
    expected: &Expected,
    entry_expected: &BTreeMap<String, Expected>,
) -> Result<(Want, BTreeMap<String, Ty>), Error> {
    let want = expected.want(env)?;
    if !x.has_result() && !matches!(want, Want::Nothing) {
        return Err(Error::one(Diagnostic::bare(
            Phase::Declare,
            crate::diag::code::NO_RESULT,
            "the program has no result expression",
        )));
    }
    let mut entry_wants = BTreeMap::new();
    if !entry_expected.is_empty() {
        let valid_names = crate::compile::entry_names(&x.ast);
        for (name, exp) in entry_expected {
            if !valid_names.contains(name) {
                return Err(Error::one(Diagnostic::bare(
                    Phase::Declare,
                    crate::diag::code::BAD_NAME,
                    format!("{name} is not an entry point of the program"),
                )));
            }
            let ty = exp.want_entry(env)?;
            entry_wants.insert(name.clone(), ty);
        }
    }
    Ok((want, entry_wants))
}

impl Env {
    /// Check a program (`03-host.md` 3.2.1). A program that fails the check
    /// gives its diagnostics, ordered by 6.1, as an error of phase `check`.
    pub fn check(&self, x: &Expr, expected: &Expected) -> Result<CheckResult, Error> {
        self.check_with(
            x,
            &CompileOptions {
                expected: expected.clone(),
                ..CompileOptions::default()
            },
        )
    }

    /// Check a program with options, including per-entry expected results
    /// (`f16-entry-points.md` 12.3 item 5).
    pub fn check_with(&self, x: &Expr, options: &CompileOptions) -> Result<CheckResult, Error> {
        let (want, entry_wants) =
            validate_expected(self, x, &options.expected, &options.entry_expected)?;
        let r = check::check_with_entries(self, x, &want, &entry_wants);
        if !r.diagnostics.is_empty() {
            return Err(Error {
                phase: Phase::Check,
                diagnostics: r.diagnostics,
            });
        }
        Ok(CheckResult {
            ty: Type::from_ty(r.ty.expect("a program that checks has a type")),
            sites: sites(&x.source, &r.sites),
            entries: r
                .entries
                .iter()
                .map(crate::compile::CompiledEntry::to_public)
                .collect(),
        })
    }

    /// Compile a program. With the check (the default) it fails exactly
    /// when [`Env::check_with`] does.
    pub fn compile(&self, x: &Expr, options: CompileOptions) -> Result<Program, Error> {
        let (want, entry_wants) =
            validate_expected(self, x, &options.expected, &options.entry_expected)?;
        if !options.check {
            return crate::compile::compile_unchecked(self, x.clone());
        }
        check::compile_checked_with_entries(self, x.clone(), &want, &entry_wants).map(|(p, _)| p)
    }

    /// Completion at a position of a program's text, which need not parse
    /// (F12 3.5). A position past the end of its line is read as the end of
    /// the line.
    pub fn complete(&self, text: &str, pos: Pos, expected: &Expected) -> Completion {
        self.complete_with(text, pos, expected, ParseOptions::default())
    }

    /// Completion with parse options: a lower source limit. A text longer
    /// than the limit, or a limit out of range, gives an empty list whose
    /// span is empty at `pos` (F12 3.1 item 5); completion never fails.
    pub fn complete_with(
        &self,
        text: &str,
        pos: Pos,
        expected: &Expected,
        opts: ParseOptions,
    ) -> Completion {
        if !opts.admits(text) {
            return Completion {
                span: Span {
                    start: pos,
                    end: pos,
                },
                items: Vec::new(),
            };
        }
        let want = expected.want(self).unwrap_or(Want::Nothing);
        let src = Source::new(text);
        crate::assist::complete(self, text, &want, src.offset(pos), opts).public(&src)
    }

    /// Hover at a position of a program's text (F12 3.13); `None` where
    /// there is nothing to say.
    pub fn hover(&self, text: &str, pos: Pos, expected: &Expected) -> Option<Hover> {
        self.hover_with(text, pos, expected, ParseOptions::default())
    }

    /// Hover with parse options: a lower source limit. A text longer than
    /// the limit, or a limit out of range, gives no hover (F12 3.1 item 5).
    pub fn hover_with(
        &self,
        text: &str,
        pos: Pos,
        expected: &Expected,
        opts: ParseOptions,
    ) -> Option<Hover> {
        if !opts.admits(text) {
            return None;
        }
        let want = expected.want(self).unwrap_or(Want::Nothing);
        let src = Source::new(text);
        crate::assist::hover(self, text, &want, src.offset(pos), opts).map(|h| h.public(&src))
    }
}

impl Program {
    /// Whether the program has a result expression (`f16-entry-points.md` 12.2).
    pub fn has_result(&self) -> bool {
        self.parsed.has_result()
    }

    /// The program's `export const` entry points in text order
    /// (`f16-entry-points.md` 12.6).
    pub fn entries(&self) -> &[Entry] {
        &self.public_entries
    }

    /// Run the program with variables lent for the run: its value, or the
    /// failure, with one `run` diagnostic (or `declare` `no-result` if the
    /// program has no result expression).
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
        if !self.has_result() {
            return (
                Err(Error::one(Diagnostic::bare(
                    Phase::Declare,
                    crate::diag::code::NO_RESULT,
                    "the program has no result expression",
                ))),
                Usage::default(),
            );
        }
        let out = self.outcome_with(vars, opts);
        let result = out.result.map_err(|f| Error::one(self.diagnostic(&f)));
        (result, out.usage)
    }

    /// Call an `export const` entry point positionally (`f16-entry-points.md` 12.5).
    pub fn call<'h>(
        &'h self,
        name: &str,
        args: &[&'h dyn Host],
        vars: &'h dyn Vars,
    ) -> Result<Value<'h>, Error> {
        self.call_measure(name, args, vars).0
    }

    /// Call an `export const` entry point positionally, with run options.
    pub fn call_with<'h>(
        &'h self,
        name: &str,
        args: &[&'h dyn Host],
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> Result<Value<'h>, Error> {
        self.call_measure_with(name, args, vars, opts).0
    }

    /// Call an `export const` entry point positionally, returning value and usage.
    pub fn call_measure<'h>(
        &'h self,
        name: &str,
        args: &[&'h dyn Host],
        vars: &'h dyn Vars,
    ) -> (Result<Value<'h>, Error>, Usage) {
        self.call_measure_with(name, args, vars, RunOptions::default())
    }

    /// Call an `export const` entry point positionally with options, returning
    /// value and usage.
    pub fn call_measure_with<'h>(
        &'h self,
        name: &str,
        args: &[&'h dyn Host],
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> (Result<Value<'h>, Error>, Usage) {
        self.call_outcome_with(name, args, vars, opts)
    }

    /// Call an `export const` entry point by record (`f16-entry-points.md` 12.5).
    pub fn call_record<'h>(
        &'h self,
        name: &str,
        input: &'h dyn Host,
        vars: &'h dyn Vars,
    ) -> Result<Value<'h>, Error> {
        self.call_record_measure(name, input, vars).0
    }

    /// Call an `export const` entry point by record, with run options.
    pub fn call_record_with<'h>(
        &'h self,
        name: &str,
        input: &'h dyn Host,
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> Result<Value<'h>, Error> {
        self.call_record_measure_with(name, input, vars, opts).0
    }

    /// Call an `export const` entry point by record, returning value and usage.
    pub fn call_record_measure<'h>(
        &'h self,
        name: &str,
        input: &'h dyn Host,
        vars: &'h dyn Vars,
    ) -> (Result<Value<'h>, Error>, Usage) {
        self.call_record_measure_with(name, input, vars, RunOptions::default())
    }

    /// Call an `export const` entry point by record with options, returning
    /// value and usage.
    pub fn call_record_measure_with<'h>(
        &'h self,
        name: &str,
        input: &'h dyn Host,
        vars: &'h dyn Vars,
        opts: RunOptions,
    ) -> (Result<Value<'h>, Error>, Usage) {
        self.call_record_outcome_with(name, input, vars, opts)
    }

    /// The program's check result, when it was compiled with the check.
    pub fn check_result(&self) -> Option<CheckResult> {
        Some(CheckResult {
            ty: Type::from_ty(self.ty.clone()?),
            sites: sites(&self.parsed.source, self.sites.as_ref()?),
            entries: self.public_entries.clone(),
        })
    }
}

/// The truthiness test (`01-language.md` 6.5): `null`, `false`, `0`, `""`,
/// `[]` and `{}` are falsy, and every other value truthy.
pub fn truthy(v: &Value<'_>) -> bool {
    v.truthy()
}

/// The JSON text of a value (`03-host.md` 3.2.4): keys in code point order,
/// numbers in the rendering of `01-language.md`. It fails with an error of
/// phase `run` and one diagnostic: `wrong-kind` on an opaque value,
/// `number-range` on a number that cannot enter, and `depth-exceeded` past
/// depth 200.
pub fn json_text(v: &Value<'_>) -> Result<String, Error> {
    crate::eval::to_json(v).map_err(|code| {
        let message = match code {
            "wrong-kind" => "an opaque value has no JSON form",
            "number-range" => "the number is out of range",
            _ => "the value is deeper than 200",
        };
        Error::one(Diagnostic::bare(Phase::Run, code, message))
    })
}
