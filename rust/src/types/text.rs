//! Type text (`05-types.md` 3.1 and 3.2).
//!
//! Every implementation parses the type text of its environment, with or
//! without a type checker, to learn the names of the variables and the counts
//! of each function's parameters (ruling A2, `05-types.md` 19.5). This module
//! is the grammar only. The rules beyond it (a key written twice, two record
//! alternatives, an undeclared name, a rest that is not a list, the depth of a
//! type) belong to the checker.

use crate::source::Source;
use crate::syntax::lexer::lex;
use crate::syntax::token::{Tok, Token, P};
use std::fmt;

/// Type text may nest brackets of any kind at most this deep (20.5).
pub const MAX_BRACKETS: u32 = 64;

/// A type as written. Parentheses are dropped.
#[derive(Clone, Debug, PartialEq)]
pub enum Type {
    Null,
    Bool,
    Int,
    Float,
    String,
    Any,
    Never,
    /// A string literal: a literal set of one string.
    Lit(String),
    /// `map<T>`
    MapOf(Box<Type>),
    Record(Record),
    /// An opaque type name, two characters or more.
    Name(String),
    /// A type variable of a signature: one upper-case letter.
    Var(char),
    /// `T[]`
    List(Box<Type>),
    /// Two or more alternatives, as written.
    Union(Vec<Type>),
}

#[derive(Clone, Debug, PartialEq)]
pub struct Record {
    pub fields: Vec<Field>,
    pub rest: Rest,
}

#[derive(Clone, Debug, PartialEq)]
pub enum Rest {
    /// No `...`: the record is exact.
    Exact,
    /// `...`: the rest is `any`.
    Any,
    /// `...T`
    Typed(Box<Type>),
}

#[derive(Clone, Debug, PartialEq)]
pub struct Field {
    pub key: String,
    pub optional: bool,
    pub ty: Type,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Bound {
    Ordered,
    Number,
}

/// A signature: `(s: string, n?: int) => string !`.
#[derive(Clone, Debug, PartialEq)]
pub struct Sig {
    pub params: Vec<Param>,
    pub result: Type,
    pub fails: bool,
    pub bounds: Vec<(char, Bound)>,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Param {
    pub rest: bool,
    pub name: String,
    pub optional: bool,
    pub ty: ParamType,
}

#[derive(Clone, Debug, PartialEq)]
pub enum ParamType {
    Fn(FnType),
    Type(Type),
}

/// A parameter that takes a lambda. `result` is `None` for `unknown`.
#[derive(Clone, Debug, PartialEq)]
pub struct FnType {
    pub params: Vec<(String, Type)>,
    pub result: Option<Type>,
}

/// `Recv "." Word ":" Sig`
#[derive(Clone, Debug, PartialEq)]
pub struct Method {
    pub recv: Type,
    pub name: String,
    pub sig: Sig,
}

impl Sig {
    /// The parameters with no `?` and no `...` (22.3).
    pub fn required(&self) -> usize {
        self.params
            .iter()
            .filter(|p| !p.optional && !p.rest)
            .count()
    }

    /// The number of parameters, or `None` when the last is a rest parameter.
    pub fn at_most(&self) -> Option<usize> {
        match self.params.last() {
            Some(p) if p.rest => None,
            _ => Some(self.params.len()),
        }
    }

    /// The parameter at argument `i` (22.3).
    pub fn param_at(&self, i: usize) -> Option<&Param> {
        match self.params.get(i) {
            Some(p) => Some(p),
            None => self.params.last().filter(|p| p.rest),
        }
    }

    pub fn has_fn_param(&self) -> bool {
        self.params.iter().any(|p| matches!(p.ty, ParamType::Fn(_)))
    }
}

/// Text that does not fit the grammar. The caller gives the code:
/// `bad-type` for a `Type`, `bad-signature` for a `Sig` or a `Method`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct TypeTextError {
    pub message: String,
}

impl fmt::Display for TypeTextError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

type R<T> = Result<T, TypeTextError>;

/// Parse a `Type`. A type variable is an error here.
pub fn parse_type(text: &str) -> R<Type> {
    let mut p = TypeParser::new(text, false)?;
    let t = p.ty()?;
    p.end()?;
    Ok(t)
}

/// Parse a `Recv` or a type of a signature: a `Type` in which type
/// variables may stand.
pub fn parse_type_vars(text: &str) -> R<Type> {
    let mut p = TypeParser::new(text, true)?;
    let t = p.ty()?;
    p.end()?;
    Ok(t)
}

/// Parse a `Sig`, and apply the order rules of 3.6 that 19.5 makes errors of
/// every implementation.
pub fn parse_sig(text: &str) -> R<Sig> {
    let mut p = TypeParser::new(text, true)?;
    let s = p.sig()?;
    p.end()?;
    Ok(s)
}

/// Parse a `Method`: `Recv "." Word ":" Sig`.
pub fn parse_method(text: &str) -> R<Method> {
    let mut p = TypeParser::new(text, true)?;
    let recv = p.alt()?;
    p.expect(P::Dot)?;
    let name = p.word()?;
    p.expect(P::Colon)?;
    let sig = p.sig()?;
    p.end()?;
    Ok(Method { recv, name, sig })
}

struct TypeParser {
    toks: Vec<Token>,
    pos: usize,
    vars: bool,
    depth: u32,
    /// The depth (ruling AA3) of the construct parsed last.
    d: u32,
}

fn fail<T>(message: impl Into<String>) -> R<T> {
    Err(TypeTextError {
        message: message.into(),
    })
}

/// An opaque type name: an upper-case letter, then one or more word
/// characters.
fn is_type_name(w: &str) -> bool {
    let mut cs = w.chars();
    matches!(cs.next(), Some(c) if c.is_ascii_uppercase()) && w.len() >= 2
}

impl TypeParser {
    fn new(text: &str, vars: bool) -> R<TypeParser> {
        let src = Source::new(text);
        let toks = match lex(&src) {
            Ok(t) => t,
            Err(d) => return fail(d.message),
        };
        Ok(TypeParser {
            toks,
            pos: 0,
            vars,
            depth: 0,
            d: 0,
        })
    }

    fn peek(&self) -> &Tok {
        &self.toks[self.pos].tok
    }

    fn peek_n(&self, n: usize) -> &Tok {
        &self.toks[(self.pos + n).min(self.toks.len() - 1)].tok
    }

    fn at(&self, p: P) -> bool {
        matches!(self.peek(), Tok::Punct(q) if *q == p)
    }

    fn at_word(&self, w: &str) -> bool {
        matches!(self.peek(), Tok::Word(x) if x == w)
    }

    fn bump(&mut self) {
        if self.pos + 1 < self.toks.len() {
            self.pos += 1;
        }
    }

    fn expect(&mut self, p: P) -> R<()> {
        if self.at(p) {
            self.bump();
            Ok(())
        } else {
            fail(format!("expected {}", p.text()))
        }
    }

    fn word(&mut self) -> R<String> {
        match self.peek().clone() {
            Tok::Word(w) => {
                self.bump();
                Ok(w)
            }
            _ => fail("expected a word"),
        }
    }

    fn end(&self) -> R<()> {
        match self.peek() {
            Tok::Eof => Ok(()),
            _ => fail("unexpected text after the type"),
        }
    }

    /// Consume an opening bracket and count it (20.5).
    fn open(&mut self, p: P) -> R<()> {
        self.expect(p)?;
        if self.depth >= MAX_BRACKETS {
            return fail("type text nests brackets more than 64 deep");
        }
        self.depth += 1;
        Ok(())
    }

    fn close(&mut self, p: P) -> R<()> {
        self.expect(p)?;
        self.depth -= 1;
        Ok(())
    }

    /// Ruling AA3: a `[]` suffix has depth 1 more than what it follows, a
    /// bracketed form 1 more than the deepest of its contents, any other text
    /// 0. Text deeper than 64 is refused.
    fn deeper(&mut self, inner: u32) -> R<()> {
        if inner + 1 > MAX_BRACKETS {
            return fail("type text is deeper than 64");
        }
        self.d = inner + 1;
        Ok(())
    }

    /// `Type = Alt { "|" Alt }`
    fn ty(&mut self) -> R<Type> {
        let first = self.alt()?;
        if !self.at(P::Pipe) {
            return Ok(first);
        }
        let mut max = self.d;
        let mut alts = vec![first];
        while self.at(P::Pipe) {
            self.bump();
            alts.push(self.alt()?);
            max = max.max(self.d);
        }
        self.d = max;
        Ok(Type::Union(alts))
    }

    /// `Alt = Prim { "[" "]" }`
    fn alt(&mut self) -> R<Type> {
        let mut t = self.prim()?;
        while self.at(P::LBracket) {
            self.open(P::LBracket)?;
            self.close(P::RBracket)?;
            self.deeper(self.d)?;
            t = Type::List(Box::new(t));
        }
        Ok(t)
    }

    fn prim(&mut self) -> R<Type> {
        match self.peek().clone() {
            Tok::Word(w) => {
                let t = match w.as_str() {
                    "null" => Type::Null,
                    "bool" => Type::Bool,
                    "int" => Type::Int,
                    "float" => Type::Float,
                    "string" => Type::String,
                    "any" => Type::Any,
                    "never" => Type::Never,
                    "map" => {
                        self.bump();
                        self.open(P::Lt)?;
                        let t = self.ty()?;
                        self.close(P::Gt)?;
                        self.deeper(self.d)?;
                        return Ok(Type::MapOf(Box::new(t)));
                    }
                    w if w.len() == 1 && w.as_bytes()[0].is_ascii_uppercase() => {
                        if !self.vars {
                            return fail(format!(
                                "the type variable {w} stands outside a signature"
                            ));
                        }
                        Type::Var(w.as_bytes()[0] as char)
                    }
                    w if is_type_name(w) => Type::Name(w.to_string()),
                    w => return fail(format!("{w} is not a type")),
                };
                self.bump();
                self.d = 0;
                Ok(t)
            }
            Tok::Str(s, f) if f.is_line() => {
                self.bump();
                self.d = 0;
                Ok(Type::Lit(s))
            }
            Tok::Punct(P::LBrace) => self.record(),
            Tok::Punct(P::LParen) => {
                self.open(P::LParen)?;
                let t = self.ty()?;
                self.close(P::RParen)?;
                self.deeper(self.d)?;
                Ok(t)
            }
            _ => fail("expected a type"),
        }
    }

    fn record(&mut self) -> R<Type> {
        self.open(P::LBrace)?;
        let mut fields = Vec::new();
        let mut rest = Rest::Exact;
        let mut max = 0;
        loop {
            if self.at(P::RBrace) {
                break;
            }
            if self.at(P::Ellipsis) {
                self.bump();
                rest = if self.at(P::Comma) || self.at(P::RBrace) {
                    Rest::Any
                } else {
                    let t = self.ty()?;
                    max = max.max(self.d);
                    Rest::Typed(Box::new(t))
                };
                if self.at(P::Comma) {
                    self.bump();
                }
                break;
            }
            let key = match self.peek().clone() {
                Tok::Word(w) => w,
                Tok::Str(s, f) if f.is_line() => s,
                _ => return fail("expected a field"),
            };
            self.bump();
            let optional = self.at(P::Question);
            if optional {
                self.bump();
            }
            self.expect(P::Colon)?;
            let ty = self.ty()?;
            max = max.max(self.d);
            fields.push(Field { key, optional, ty });
            if self.at(P::Comma) {
                self.bump();
            } else {
                break;
            }
        }
        self.close(P::RBrace)?;
        self.deeper(max)?;
        Ok(Type::Record(Record { fields, rest }))
    }

    fn sig(&mut self) -> R<Sig> {
        self.open(P::LParen)?;
        let mut params = Vec::new();
        let mut max = 0;
        if !self.at(P::RParen) {
            loop {
                params.push(self.param()?);
                max = max.max(self.d);
                if self.at(P::Comma) {
                    self.bump();
                } else {
                    break;
                }
            }
        }
        self.close(P::RParen)?;
        self.deeper(max)?;
        self.expect(P::Arrow)?;
        let result = self.ty()?;
        let fails = self.at(P::Bang);
        if fails {
            self.bump();
        }
        let mut bounds = Vec::new();
        if self.at_word("where") {
            self.bump();
            loop {
                let v = self.word()?;
                if v.len() != 1 || !v.as_bytes()[0].is_ascii_uppercase() {
                    return fail("a bound names a type variable");
                }
                self.expect(P::Colon)?;
                let b = match self.word()?.as_str() {
                    "ordered" => Bound::Ordered,
                    "number" => Bound::Number,
                    _ => return fail("a bound is ordered or number"),
                };
                bounds.push((v.as_bytes()[0] as char, b));
                if self.at(P::Comma) {
                    self.bump();
                } else {
                    break;
                }
            }
        }
        // 3.6, made an error of every implementation by 19.5.
        let mut seen_optional = false;
        for (i, p) in params.iter().enumerate() {
            if p.rest && i + 1 != params.len() {
                return fail("a rest parameter must be the last");
            }
            if p.optional {
                seen_optional = true;
            } else if !p.rest && seen_optional {
                return fail("an optional parameter stands before a required one");
            }
        }
        Ok(Sig {
            params,
            result,
            fails,
            bounds,
        })
    }

    /// `Param = [ "..." ] Word [ "?" ] ":" ( Fn | Type )`
    fn param(&mut self) -> R<Param> {
        let rest = self.at(P::Ellipsis);
        if rest {
            self.bump();
        }
        let name = self.word()?;
        let optional = self.at(P::Question);
        if optional {
            self.bump();
        }
        self.expect(P::Colon)?;
        // 3.2: `(` then `)`, or `(` then a word then `:`, starts an `Fn`.
        let is_fn = self.at(P::LParen)
            && (matches!(self.peek_n(1), Tok::Punct(P::RParen))
                || (matches!(self.peek_n(1), Tok::Word(_))
                    && matches!(self.peek_n(2), Tok::Punct(P::Colon))));
        let ty = if is_fn {
            ParamType::Fn(self.fn_type()?)
        } else {
            ParamType::Type(self.ty()?)
        };
        Ok(Param {
            rest,
            name,
            optional,
            ty,
        })
    }

    fn fn_type(&mut self) -> R<FnType> {
        self.open(P::LParen)?;
        let mut params = Vec::new();
        let mut max = 0;
        if !self.at(P::RParen) {
            loop {
                let name = self.word()?;
                self.expect(P::Colon)?;
                params.push((name, self.ty()?));
                max = max.max(self.d);
                if self.at(P::Comma) {
                    self.bump();
                } else {
                    break;
                }
            }
        }
        self.close(P::RParen)?;
        self.deeper(max)?;
        let bracket = self.d;
        self.expect(P::Arrow)?;
        let result = if self.at_word("unknown") {
            self.bump();
            None
        } else {
            Some(self.ty()?)
        };
        self.d = bracket.max(self.d);
        Ok(FnType { params, result })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn types() {
        assert_eq!(parse_type("int").unwrap(), Type::Int);
        assert_eq!(
            parse_type("int | string[]").unwrap(),
            Type::Union(vec![Type::Int, Type::List(Box::new(Type::String))])
        );
        assert_eq!(
            parse_type("map<map<int>>").unwrap(),
            Type::MapOf(Box::new(Type::MapOf(Box::new(Type::Int))))
        );
        let r = parse_type("{a: int, \"b-c\"?: string, ...}").unwrap();
        match r {
            Type::Record(r) => {
                assert_eq!(r.fields.len(), 2);
                assert!(r.fields[1].optional);
                assert_eq!(r.rest, Rest::Any);
            }
            _ => panic!(),
        }
        assert_eq!(
            parse_type("{}").unwrap(),
            Type::Record(Record {
                fields: vec![],
                rest: Rest::Exact
            })
        );
        assert!(parse_type("{...float}").is_ok());
        assert!(parse_type("{if: int, null: bool}").is_ok());
        assert_eq!(parse_type("Time").unwrap(), Type::Name("Time".into()));
    }

    #[test]
    fn type_errors() {
        for t in [
            "number",
            "boolean",
            "undefined",
            "void",
            "nil",
            "object",
            "Array<int>",
            "int?",
            "condition",
            "unknown",
            "T",
            "`a`",
            "int |",
            "{a int}",
            "map<int",
            "(int",
            "int int",
            "",
        ] {
            assert!(parse_type(t).is_err(), "{t}");
        }
    }

    #[test]
    fn signatures() {
        let s = parse_sig("(s: string, n?: int) => string !").unwrap();
        assert_eq!(s.required(), 1);
        assert_eq!(s.at_most(), Some(2));
        assert!(s.fails);
        let s = parse_sig("(f: (x: T, i: int) => unknown, ...rest: T[]) => T[] where T: ordered")
            .unwrap();
        assert!(s.has_fn_param());
        assert_eq!(s.at_most(), None);
        assert_eq!(s.bounds, vec![('T', Bound::Ordered)]);
        assert!(parse_sig("(a?: int, b: int) => int").is_err());
        assert!(parse_sig("(...a: int[], b: int) => int").is_err());
        assert!(parse_sig("() => unknown").is_err());
        let m = parse_method("Point.dist: (o: Point) => float").unwrap();
        assert_eq!(m.name, "dist");
    }

    #[test]
    fn bracket_limit() {
        assert!(parse_type(&format!("int{}", "[]".repeat(64))).is_ok());
        assert!(parse_type(&format!("int{}", "[]".repeat(65))).is_err());
        assert!(parse_type(&format!("({}int[]{})", "(".repeat(62), ")".repeat(62))).is_ok());
        assert!(parse_type(&format!("(({}int[]{}))", "(".repeat(62), ")".repeat(62))).is_err());
        let ok = format!("{}int{}", "(".repeat(64), ")".repeat(64));
        assert!(parse_type(&ok).is_ok());
        let deep = format!("{}int{}", "(".repeat(65), ")".repeat(65));
        assert!(parse_type(&deep).is_err());
        let huge = format!("{}int{}", "(".repeat(1_000_000), ")".repeat(1_000_000));
        assert!(parse_type(&huge).is_err());
    }
}
