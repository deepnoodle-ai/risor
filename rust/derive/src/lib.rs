//! `#[derive(Host)]`: the type and the presentation of a Rust struct, for
//! Risor v3 package F1 (`spec/packages/f01-host-types.md` 6.3).
//!
//! The crate has no dependency. It reads the token stream of the item
//! itself, and writes the implementation as text.
//!
//! For a struct with named fields, the derive implements
//! `risor::HostType` (the type: an exact record of the fields it presents)
//! and `risor::Host` with `risor::HostMap` (the presentation: a map of those
//! fields, read in place). Each field's type must implement `HostType` and
//! `Host`.
//!
//! | Attribute on a field | Effect |
//! | --- | --- |
//! | none | A required field, under the field's Rust name (`r#type` is `type`) |
//! | `#[risor(rename = "k")]` | The field is presented under the key `k` |
//! | `#[risor(skip)]` | The field is not presented and is not in the record |
//! | `#[risor(absent_if_none)]` | On `Option<T>` only: an optional field `f?: T`, absent when `None` |
//!
//! Attributes may be combined in one `#[risor(...)]` or written apart. Any
//! other content of `#[risor(...)]`, two fields under one key, an `enum`, a
//! `union`, a tuple struct and a unit struct fail the build.
//!
//! ```
//! use risor_derive::Host;
//!
//! #[derive(Host)]
//! struct Order {
//!     id: u64,
//!     #[risor(rename = "customer-name")]
//!     customer: String,
//!     #[risor(absent_if_none)]
//!     note: Option<String>,
//!     #[risor(skip)]
//!     cache: Vec<u8>,
//! }
//!
//! assert_eq!(
//!     risor::describe_host_type::<Order>().unwrap(),
//!     "{\"customer-name\": string, id: int, note?: string}"
//! );
//! ```
//!
//! These fail the build. A Rust `enum`:
//!
//! ```compile_fail
//! #[derive(risor_derive::Host)]
//! enum Shape { Circle, Square }
//! ```
//!
//! A tuple struct:
//!
//! ```compile_fail
//! #[derive(risor_derive::Host)]
//! struct Point(f64, f64);
//! ```
//!
//! `absent_if_none` on a field that is not an `Option`:
//!
//! ```compile_fail
//! #[derive(risor_derive::Host)]
//! struct S { #[risor(absent_if_none)] a: String }
//! ```
//!
//! An attribute the derive does not know:
//!
//! ```compile_fail
//! #[derive(risor_derive::Host)]
//! struct S { #[risor(flatten)] a: String }
//! ```
//!
//! Two fields under one key:
//!
//! ```compile_fail
//! #[derive(risor_derive::Host)]
//! struct S { a: String, #[risor(rename = "a")] b: String }
//! ```
//!
//! A field whose type has no rule (here a `char`):
//!
//! ```compile_fail
//! #[derive(risor_derive::Host)]
//! struct S { c: char }
//! ```

use proc_macro::{Delimiter, Spacing, TokenStream, TokenTree};

#[proc_macro_derive(Host, attributes(risor))]
pub fn derive_host(input: TokenStream) -> TokenStream {
    match derive(input) {
        Ok(code) => code.parse().expect("the generated code parses"),
        Err(msg) => format!("::core::compile_error!({msg:?});")
            .parse()
            .expect("compile_error parses"),
    }
}

struct FieldSpec {
    ident: String,
    key: String,
    ty: String,
    skip: bool,
    absent_if_none: bool,
}

struct Item {
    name: String,
    /// The generic parameters as written, without the angle brackets.
    params: Vec<Vec<TokenTree>>,
    /// The where clause's predicates, without `where`.
    where_clause: String,
    fields: Vec<FieldSpec>,
}

fn derive(input: TokenStream) -> Result<String, String> {
    let item = parse_item(input)?;
    let mut seen: Vec<&str> = Vec::new();
    for f in item.fields.iter().filter(|f| !f.skip) {
        if seen.contains(&f.key.as_str()) {
            return Err(format!(
                "derive(Host): two fields of {} are presented under the key {:?}",
                item.name, f.key
            ));
        }
        seen.push(&f.key);
    }
    Ok(generate(&item))
}

fn parse_item(input: TokenStream) -> Result<Item, String> {
    let toks: Vec<TokenTree> = input.into_iter().collect();
    let mut i = 0;
    skip_attrs_and_vis(&toks, &mut i);
    match toks.get(i) {
        Some(TokenTree::Ident(id)) if id.to_string() == "struct" => i += 1,
        Some(TokenTree::Ident(id)) if id.to_string() == "enum" => {
            return Err(
                "derive(Host) takes a struct with named fields: a Rust enum has no type in F1"
                    .into(),
            )
        }
        _ => return Err("derive(Host) takes a struct with named fields".into()),
    }
    let name = match toks.get(i) {
        Some(TokenTree::Ident(id)) => id.to_string(),
        _ => return Err("derive(Host): the struct has no name".into()),
    };
    i += 1;
    let mut params = Vec::new();
    if is_punct(toks.get(i), '<') {
        i += 1;
        let mut depth = 0usize;
        let mut cur: Vec<TokenTree> = Vec::new();
        loop {
            let Some(t) = toks.get(i) else {
                return Err("derive(Host): unclosed generic parameters".into());
            };
            i += 1;
            if let TokenTree::Punct(p) = t {
                match p.as_char() {
                    '<' => depth += 1,
                    '>' if is_arrow_end(&toks, i - 1) => {}
                    '>' if depth == 0 => break,
                    '>' => depth -= 1,
                    ',' if depth == 0 => {
                        params.push(std::mem::take(&mut cur));
                        continue;
                    }
                    _ => {}
                }
            }
            cur.push(t.clone());
        }
        if !cur.is_empty() {
            params.push(cur);
        }
    }
    let mut where_toks: Vec<TokenTree> = Vec::new();
    let body = loop {
        match toks.get(i) {
            Some(TokenTree::Group(g)) if g.delimiter() == Delimiter::Brace => break g.stream(),
            Some(TokenTree::Group(g)) if g.delimiter() == Delimiter::Parenthesis => {
                return Err(
                    "derive(Host) takes a struct with named fields, not a tuple struct".into(),
                )
            }
            Some(TokenTree::Punct(p)) if p.as_char() == ';' => {
                return Err(
                    "derive(Host) takes a struct with named fields, not a unit struct".into(),
                )
            }
            Some(TokenTree::Ident(id)) if id.to_string() == "where" => i += 1,
            Some(t) => {
                where_toks.push(t.clone());
                i += 1;
            }
            None => return Err("derive(Host): the struct has no body".into()),
        }
    };
    let mut where_clause = text(&where_toks);
    if !where_clause.is_empty() && !where_clause.trim_end().ends_with(',') {
        where_clause.push(',');
    }
    Ok(Item {
        name,
        params,
        where_clause,
        fields: parse_fields(body)?,
    })
}

/// `->` inside generic bounds: the `>` of an arrow does not close.
fn is_arrow_end(toks: &[TokenTree], at: usize) -> bool {
    at > 0
        && matches!(&toks[at - 1], TokenTree::Punct(p) if p.as_char() == '-' && p.spacing() == Spacing::Joint)
}

fn is_punct(t: Option<&TokenTree>, c: char) -> bool {
    matches!(t, Some(TokenTree::Punct(p)) if p.as_char() == c)
}

fn skip_attrs_and_vis(toks: &[TokenTree], i: &mut usize) {
    loop {
        match toks.get(*i) {
            Some(TokenTree::Punct(p)) if p.as_char() == '#' => *i += 2,
            Some(TokenTree::Ident(id)) if id.to_string() == "pub" => {
                *i += 1;
                if let Some(TokenTree::Group(g)) = toks.get(*i) {
                    if g.delimiter() == Delimiter::Parenthesis {
                        *i += 1;
                    }
                }
            }
            _ => return,
        }
    }
}

fn text(toks: &[TokenTree]) -> String {
    toks.iter().cloned().collect::<TokenStream>().to_string()
}

fn parse_fields(body: TokenStream) -> Result<Vec<FieldSpec>, String> {
    let toks: Vec<TokenTree> = body.into_iter().collect();
    let mut out = Vec::new();
    let mut i = 0;
    while i < toks.len() {
        let mut spec = FieldSpec {
            ident: String::new(),
            key: String::new(),
            ty: String::new(),
            skip: false,
            absent_if_none: false,
        };
        let mut rename: Option<String> = None;
        // Attributes and visibility.
        loop {
            match toks.get(i) {
                Some(TokenTree::Punct(p)) if p.as_char() == '#' => {
                    if let Some(TokenTree::Group(g)) = toks.get(i + 1) {
                        read_attr(g.stream(), &mut spec, &mut rename)?;
                    }
                    i += 2;
                }
                Some(TokenTree::Ident(id)) if id.to_string() == "pub" => {
                    i += 1;
                    if let Some(TokenTree::Group(g)) = toks.get(i) {
                        if g.delimiter() == Delimiter::Parenthesis {
                            i += 1;
                        }
                    }
                }
                _ => break,
            }
        }
        let ident = match toks.get(i) {
            Some(TokenTree::Ident(id)) => id.to_string(),
            _ => return Err("derive(Host): a field has no name".into()),
        };
        i += 1;
        if !is_punct(toks.get(i), ':') {
            return Err(format!("derive(Host): the field {ident} has no type"));
        }
        i += 1;
        // The type runs to a comma outside angle brackets.
        let mut depth = 0usize;
        let mut ty: Vec<TokenTree> = Vec::new();
        while let Some(t) = toks.get(i) {
            if let TokenTree::Punct(p) = t {
                match p.as_char() {
                    '<' => depth += 1,
                    '>' if is_arrow_end(&toks, i) => {}
                    '>' => depth = depth.saturating_sub(1),
                    ',' if depth == 0 => break,
                    _ => {}
                }
            }
            ty.push(t.clone());
            i += 1;
        }
        i += 1; // the comma
        spec.key = rename.unwrap_or_else(|| ident.strip_prefix("r#").unwrap_or(&ident).to_string());
        spec.ident = ident;
        spec.ty = text(&ty);
        out.push(spec);
    }
    Ok(out)
}

/// One attribute of a field: `risor(...)` is read, every other is left to
/// its own macro.
fn read_attr(
    attr: TokenStream,
    spec: &mut FieldSpec,
    rename: &mut Option<String>,
) -> Result<(), String> {
    let toks: Vec<TokenTree> = attr.into_iter().collect();
    match toks.first() {
        Some(TokenTree::Ident(id)) if id.to_string() == "risor" => {}
        _ => return Ok(()),
    }
    let Some(TokenTree::Group(g)) = toks.get(1) else {
        return Err("derive(Host): write #[risor(...)]".into());
    };
    let inner: Vec<TokenTree> = g.stream().into_iter().collect();
    let mut i = 0;
    while i < inner.len() {
        let word = match &inner[i] {
            TokenTree::Ident(id) => id.to_string(),
            t => return Err(format!("derive(Host): {t} is not an attribute of risor")),
        };
        i += 1;
        match word.as_str() {
            "skip" => spec.skip = true,
            "absent_if_none" => spec.absent_if_none = true,
            "rename" => {
                if !is_punct(inner.get(i), '=') {
                    return Err("derive(Host): write rename = \"key\"".into());
                }
                let lit = match inner.get(i + 1) {
                    Some(TokenTree::Literal(l)) => l.to_string(),
                    _ => return Err("derive(Host): write rename = \"key\"".into()),
                };
                *rename = Some(string_literal(&lit)?);
                i += 2;
            }
            other => {
                return Err(format!(
                    "derive(Host): {other} is not an attribute of risor; there are rename, skip and absent_if_none"
                ))
            }
        }
        match inner.get(i) {
            None => {}
            Some(TokenTree::Punct(p)) if p.as_char() == ',' => i += 1,
            Some(t) => return Err(format!("derive(Host): unexpected {t} in #[risor(...)]")),
        }
    }
    Ok(())
}

/// The value of a Rust string literal: `"..."` with its escapes, or a raw
/// string `r"..."`, `r#"..."#`.
fn string_literal(lit: &str) -> Result<String, String> {
    let bad = || format!("derive(Host): rename takes a string literal, not {lit}");
    if let Some(raw) = lit.strip_prefix('r') {
        let hashes = raw.len() - raw.trim_start_matches('#').len();
        let body = &raw[hashes..raw.len() - hashes];
        return body
            .strip_prefix('"')
            .and_then(|b| b.strip_suffix('"'))
            .map(String::from)
            .ok_or_else(bad);
    }
    let body = lit
        .strip_prefix('"')
        .and_then(|b| b.strip_suffix('"'))
        .ok_or_else(bad)?;
    let mut out = String::new();
    let mut cs = body.chars().peekable();
    while let Some(c) = cs.next() {
        if c != '\\' {
            out.push(c);
            continue;
        }
        match cs.next().ok_or_else(bad)? {
            'n' => out.push('\n'),
            'r' => out.push('\r'),
            't' => out.push('\t'),
            '0' => out.push('\0'),
            '\\' => out.push('\\'),
            '"' => out.push('"'),
            '\'' => out.push('\''),
            'x' => {
                let h: String = cs.by_ref().take(2).collect();
                let n = u32::from_str_radix(&h, 16).map_err(|_| bad())?;
                out.push(char::from_u32(n).ok_or_else(bad)?);
            }
            'u' => {
                if cs.next() != Some('{') {
                    return Err(bad());
                }
                let h: String = cs.by_ref().take_while(|c| *c != '}').collect();
                let n = u32::from_str_radix(&h.replace('_', ""), 16).map_err(|_| bad())?;
                out.push(char::from_u32(n).ok_or_else(bad)?);
            }
            '\n' => {
                while cs.peek().is_some_and(|c| c.is_whitespace()) {
                    cs.next();
                }
            }
            _ => return Err(bad()),
        }
    }
    Ok(out)
}

fn generate(item: &Item) -> String {
    // impl<params> ... for Name<args> where bounds
    let mut decl = Vec::new();
    let mut args = Vec::new();
    let mut bounds = String::new();
    for p in &item.params {
        let full = text(p);
        // Drop a default (`T = X`) from the declaration.
        let decl_text = match p
            .iter()
            .position(|t| matches!(t, TokenTree::Punct(q) if q.as_char() == '='))
        {
            Some(at) => text(&p[..at]),
            None => full,
        };
        decl.push(decl_text);
        match p.first() {
            Some(TokenTree::Punct(q)) if q.as_char() == '\'' => {
                if let Some(TokenTree::Ident(l)) = p.get(1) {
                    args.push(format!("'{l}"));
                }
            }
            Some(TokenTree::Ident(id)) if id.to_string() == "const" => {
                if let Some(TokenTree::Ident(n)) = p.get(1) {
                    args.push(n.to_string());
                }
            }
            Some(TokenTree::Ident(id)) => {
                args.push(id.to_string());
                bounds.push_str(&format!("{id}: ::risor::HostType + ::risor::Host,"));
            }
            _ => {}
        }
    }
    let (g_decl, g_args) = if decl.is_empty() {
        (String::new(), String::new())
    } else {
        (
            format!("<{}>", decl.join(", ")),
            format!("<{}>", args.join(", ")),
        )
    };
    let where_all = format!("{}{}", item.where_clause, bounds);
    let where_text = if where_all.is_empty() {
        String::new()
    } else {
        format!("where {where_all}")
    };
    let name = &item.name;
    let fields: Vec<&FieldSpec> = item.fields.iter().filter(|f| !f.skip).collect();

    let mut ty_body = String::new();
    let mut get_arms = String::new();
    let mut keys = String::new();
    for f in &fields {
        let key = format!("{:?}", f.key);
        let id = &f.ident;
        let ty = &f.ty;
        if f.absent_if_none {
            ty_body.push_str(&format!(
                "r.field({key}, cx.of::<<{ty} as ::risor::OptionLike>::Inner>()?, true)?;\n"
            ));
            get_arms.push_str(&format!(
                "{key} => ::risor::OptionLike::present(&self.{id}).map(::risor::Host::view),\n"
            ));
            keys.push_str(&format!(
                "if ::risor::OptionLike::present(&self.{id}).is_some() {{ k.push({key}); }}\n"
            ));
        } else {
            ty_body.push_str(&format!("r.field({key}, cx.of::<{ty}>()?, false)?;\n"));
            get_arms.push_str(&format!(
                "{key} => ::core::option::Option::Some(::risor::Host::view(&self.{id})),\n"
            ));
            keys.push_str(&format!("k.push({key});\n"));
        }
    }
    format!(
        r#"
#[automatically_derived]
impl{g_decl} ::risor::HostType for {name}{g_args} {where_text} {{
    fn host_type(
        cx: &mut ::risor::TypeCx,
    ) -> ::core::result::Result<::risor::check::ty::Ty, ::risor::TypeError> {{
        cx.record::<Self>(|cx, r| {{
            #[allow(unused_variables)]
            let (cx, r) = (cx, r);
            {ty_body}
            ::core::result::Result::Ok(())
        }})
    }}
}}

#[automatically_derived]
impl{g_decl} ::risor::Host for {name}{g_args} {where_text} {{
    fn view(&self) -> ::risor::View<'_> {{
        ::risor::View::Map(self)
    }}
}}

#[automatically_derived]
impl{g_decl} ::risor::HostMap for {name}{g_args} {where_text} {{
    fn len(&self) -> usize {{
        ::risor::HostMap::keys(self).len()
    }}
    fn get(&self, key: &str) -> ::core::option::Option<::risor::View<'_>> {{
        match key {{
            {get_arms}
            _ => ::core::option::Option::None,
        }}
    }}
    fn keys(&self) -> ::std::vec::Vec<&str> {{
        #[allow(unused_mut)]
        let mut k: ::std::vec::Vec<&str> = ::std::vec::Vec::new();
        {keys}
        k
    }}
}}
"#
    )
}
