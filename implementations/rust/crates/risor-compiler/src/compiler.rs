//! Two-pass bytecode compiler for Risor.
//!
//! Pass 1: Collect function declarations for forward references
//! Pass 2: Compile AST to bytecode

use crate::symbol_table::{Resolution, Scope, ScopeManager, Symbol};
use risor_bytecode::{BinaryOpType, Code, CodeBuilder, CompareOpType, Constant, Op};
use risor_lexer::Position;
use risor_parser::*;
use std::rc::Rc;
use thiserror::Error;

/// Compilation errors.
#[derive(Error, Debug, Clone)]
pub enum CompilerError {
    #[error("{message} at line {line}, column {column}")]
    Compile {
        message: String,
        line: usize,
        column: usize,
    },
}

impl CompilerError {
    fn new(message: impl Into<String>, pos: Position) -> Self {
        Self::Compile {
            message: message.into(),
            line: pos.line_number(),
            column: pos.column_number(),
        }
    }
}

/// Compiler configuration.
#[derive(Debug, Default)]
pub struct CompilerConfig {
    /// Available global names (builtins).
    pub global_names: Vec<String>,
    /// Source filename.
    pub filename: String,
    /// Source code.
    pub source: String,
}

/// Placeholder value for forward jumps.
const PLACEHOLDER: u16 = 0xFFFF;

/// Bytecode compiler for Risor.
pub struct Compiler {
    main: CodeBuilder,
    current_idx: usize, // 0 for main, otherwise index into children stack
    children_stack: Vec<CodeBuilder>,
    scopes: ScopeManager,
    #[allow(dead_code)]
    global_names: Vec<String>,
    #[allow(dead_code)]
    filename: String,
    #[allow(dead_code)]
    source: String,
    func_index: usize,
    error: Option<CompilerError>,
}

impl Compiler {
    /// Create a new compiler.
    pub fn new(config: CompilerConfig) -> Self {
        let main = CodeBuilder::new(
            "main".to_string(),
            String::new(),
            false,
            config.source.clone(),
            config.filename.clone(),
        );

        let mut scopes = ScopeManager::new();

        // Register global names
        for name in &config.global_names {
            scopes.insert_variable(name);
        }

        Self {
            main,
            current_idx: 0,
            children_stack: Vec::new(),
            scopes,
            global_names: config.global_names,
            filename: config.filename,
            source: config.source,
            func_index: 0,
            error: None,
        }
    }

    /// Get a mutable reference to the current code builder.
    fn current(&mut self) -> &mut CodeBuilder {
        if self.current_idx == 0 {
            &mut self.main
        } else {
            &mut self.children_stack[self.current_idx - 1]
        }
    }

    /// Compile a program to bytecode.
    pub fn compile(mut self, program: &Program) -> Result<Rc<Code>, CompilerError> {
        // Pass 1: Collect function declarations
        self.collect_function_declarations(program);

        // Pass 2: Compile statements
        let last_idx = program.stmts.len().saturating_sub(1);
        for (i, stmt) in program.stmts.iter().enumerate() {
            let is_last = i == last_idx;
            self.compile_statement_with_context(stmt, is_last);
            if let Some(err) = self.error.take() {
                return Err(err);
            }
        }

        // Emit halt at end of main
        self.emit(Op::Halt);

        // For the root scope, all variables are global
        // Use the scope's local count as global count
        let global_count = self.scopes.current().local_count();
        let global_names = self.scopes.current().local_names();

        Ok(self.main.to_code(
            0, // locals are actually globals at root level
            global_count,
            vec![],
            global_names,
        ))
    }

    // ===========================================================================
    // Pass 1: Collect Function Declarations
    // ===========================================================================

    fn collect_function_declarations(&mut self, program: &Program) {
        for stmt in &program.stmts {
            if let Stmt::Func(func) = stmt {
                if let Some(name) = &func.name {
                    self.scopes.insert_constant(&name.name);
                }
            }
        }
    }

    // ===========================================================================
    // Statement Compilation
    // ===========================================================================

    fn compile_statement(&mut self, stmt: &Stmt) {
        self.compile_statement_with_context(stmt, false);
    }

    fn compile_statement_with_context(&mut self, stmt: &Stmt, is_last: bool) {
        match stmt {
            Stmt::Var(var) => self.compile_var_stmt(var),
            Stmt::MultiVar(multi) => self.compile_multi_var_stmt(multi),
            Stmt::ObjectDestructure(obj) => self.compile_object_destructure_stmt(obj),
            Stmt::ArrayDestructure(arr) => self.compile_array_destructure_stmt(arr),
            Stmt::Const(cnst) => self.compile_const_stmt(cnst),
            Stmt::Return(ret) => self.compile_return_stmt(ret),
            Stmt::Assign(assign) => self.compile_assign_stmt(assign),
            Stmt::SetAttr(set_attr) => self.compile_set_attr_stmt(set_attr),
            Stmt::Postfix(postfix) => self.compile_postfix_stmt(postfix),
            Stmt::Throw(throw) => self.compile_throw_stmt(throw),
            Stmt::Func(func) => {
                // Named function as statement
                if let Some(name) = &func.name {
                    self.scopes.insert_variable(&name.name);
                }
                self.compile_func_lit(func);
                if let Some(name) = &func.name {
                    if let Some(res) = self.scopes.resolve(&name.name) {
                        self.store_resolution(&res);
                    }
                } else {
                    self.emit(Op::PopTop);
                }
            }
            Stmt::Try(try_expr) => {
                self.compile_try_expr(try_expr);
                self.emit(Op::PopTop);
            }
            Stmt::Expr(expr) => {
                self.compile_expr(expr);
                // Don't pop the result of the last expression statement
                // so we can return it as the program result
                if !is_last || !self.is_expression(expr) {
                    self.emit(Op::PopTop);
                }
            }
        }
    }

    /// Check if an expression produces a value (vs a statement-like expression)
    fn is_expression(&self, expr: &Expr) -> bool {
        matches!(
            expr,
            Expr::Int(_)
                | Expr::Float(_)
                | Expr::Bool(_)
                | Expr::String(_)
                | Expr::Nil(_)
                | Expr::Ident(_)
                | Expr::List(_)
                | Expr::Map(_)
                | Expr::Index(_)
                | Expr::Call(_)
                | Expr::GetAttr(_)
                | Expr::ObjectCall(_)
                | Expr::Func(_)
                | Expr::Infix(_)
                | Expr::Prefix(_)
                | Expr::If(_)
                | Expr::Switch(_)
                | Expr::Match(_)
                | Expr::Pipe(_)
                | Expr::Slice(_)
                | Expr::In(_)
                | Expr::NotIn(_)
                | Expr::Spread(_)
        )
    }

    fn compile_var_stmt(&mut self, stmt: &VarStmt) {
        self.compile_expr(&stmt.value);
        let symbol = self.scopes.insert_variable(&stmt.name.name);
        self.store_symbol(&symbol);
    }

    fn compile_multi_var_stmt(&mut self, stmt: &MultiVarStmt) {
        self.compile_expr(&stmt.value);
        // Unpack into multiple variables
        self.emit1(Op::Unpack, stmt.names.len() as u16);
        for name in stmt.names.iter().rev() {
            let symbol = self.scopes.insert_variable(&name.name);
            self.store_symbol(&symbol);
        }
    }

    fn compile_object_destructure_stmt(&mut self, stmt: &ObjectDestructureStmt) {
        self.compile_expr(&stmt.value);

        for binding in &stmt.bindings {
            // Duplicate object on stack
            self.emit1(Op::Copy, 0);

            // Load property
            let name_index = self.current().add_name(&binding.key);
            self.emit1(Op::LoadAttrOrNil, name_index as u16);

            // Check for default value
            if let Some(default) = &binding.default_value {
                let skip_default = self.emit_jump_forward(Op::PopJumpForwardIfNotNil);
                self.emit(Op::PopTop);
                self.compile_expr(default);
                self.patch_jump(skip_default);
            }

            // Store to variable (alias or key name)
            let var_name = binding.alias.as_ref().unwrap_or(&binding.key);
            let symbol = self.scopes.insert_variable(var_name);
            self.store_symbol(&symbol);
        }

        // Pop original object
        self.emit(Op::PopTop);
    }

    fn compile_array_destructure_stmt(&mut self, stmt: &ArrayDestructureStmt) {
        self.compile_expr(&stmt.value);

        for (i, elem) in stmt.elements.iter().enumerate() {
            // Duplicate array on stack
            self.emit1(Op::Copy, 0);

            // Load index
            let idx_const = self.current().add_constant(Constant::Int(i as i64));
            self.emit1(Op::LoadConst, idx_const as u16);
            self.emit1(Op::BinarySubscr, 0);

            // Check for default value
            if let Some(default) = &elem.default_value {
                let skip_default = self.emit_jump_forward(Op::PopJumpForwardIfNotNil);
                self.emit(Op::PopTop);
                self.compile_expr(default);
                self.patch_jump(skip_default);
            }

            // Store to variable
            let symbol = self.scopes.insert_variable(&elem.name.name);
            self.store_symbol(&symbol);
        }

        // Pop original array
        self.emit(Op::PopTop);
    }

    fn compile_const_stmt(&mut self, stmt: &ConstStmt) {
        self.compile_expr(&stmt.value);
        let symbol = self.scopes.insert_constant(&stmt.name.name);
        self.store_symbol(&symbol);
    }

    fn compile_return_stmt(&mut self, stmt: &ReturnStmt) {
        if let Some(value) = &stmt.value {
            self.compile_expr(value);
        } else {
            self.emit(Op::Nil);
        }
        self.emit(Op::ReturnValue);
    }

    fn compile_assign_stmt(&mut self, stmt: &AssignStmt) {
        match &stmt.target {
            AssignTarget::Ident(ident) => {
                let Some(resolution) = self.scopes.resolve(&ident.name) else {
                    self.report_error(format!("undefined variable: {}", ident.name), ident.position);
                    return;
                };

                if resolution.symbol.is_constant {
                    self.report_error(
                        format!("cannot assign to constant: {}", ident.name),
                        ident.position,
                    );
                    return;
                }

                if stmt.op == "=" {
                    self.compile_expr(&stmt.value);
                } else {
                    // Compound assignment
                    self.load_resolution(&resolution);
                    self.compile_expr(&stmt.value);
                    self.compile_compound_op(&stmt.op);
                }

                self.store_resolution(&resolution);
            }
            AssignTarget::Index(index) => {
                // Index assignment
                self.compile_expr(&index.object);
                self.compile_expr(&index.index);

                if stmt.op == "=" {
                    self.compile_expr(&stmt.value);
                } else {
                    // Compound assignment to index
                    self.emit1(Op::Copy, 1); // Duplicate object
                    self.emit1(Op::Copy, 1); // Duplicate index
                    self.emit1(Op::BinarySubscr, 0);
                    self.compile_expr(&stmt.value);
                    self.compile_compound_op(&stmt.op);
                }

                self.emit1(Op::StoreSubscr, 0);
            }
        }
    }

    fn compile_set_attr_stmt(&mut self, stmt: &SetAttrStmt) {
        self.compile_expr(&stmt.object);
        let name_index = self.current().add_name(&stmt.attr.name);

        if stmt.op == "=" {
            self.compile_expr(&stmt.value);
        } else {
            // Compound assignment
            self.emit1(Op::Copy, 0); // Duplicate object
            self.emit1(Op::LoadAttr, name_index as u16);
            self.compile_expr(&stmt.value);
            self.compile_compound_op(&stmt.op);
        }

        self.emit1(Op::StoreAttr, name_index as u16);
    }

    fn compile_postfix_stmt(&mut self, stmt: &PostfixStmt) {
        let Expr::Ident(ident) = &stmt.operand else {
            self.report_error("postfix operator requires identifier", stmt.operand.pos());
            return;
        };

        let Some(resolution) = self.scopes.resolve(&ident.name) else {
            self.report_error(format!("undefined variable: {}", ident.name), ident.position);
            return;
        };

        // Load current value
        self.load_resolution(&resolution);

        // Add/subtract 1
        let one_const = self.current().add_constant(Constant::Int(1));
        self.emit1(Op::LoadConst, one_const as u16);

        if stmt.op == "++" {
            self.emit1(Op::BinaryOp, BinaryOpType::Add as u16);
        } else {
            self.emit1(Op::BinaryOp, BinaryOpType::Subtract as u16);
        }

        // Store back
        self.store_resolution(&resolution);
    }

    fn compile_throw_stmt(&mut self, stmt: &ThrowStmt) {
        self.compile_expr(&stmt.value);
        self.emit(Op::Throw);
    }

    // ===========================================================================
    // Expression Compilation
    // ===========================================================================

    fn compile_expr(&mut self, expr: &Expr) {
        let pos = expr.pos();
        self.current().set_position(pos.line, pos.column);

        match expr {
            Expr::Int(int_lit) => {
                let const_idx = self.current().add_constant(Constant::Int(int_lit.value));
                self.emit1(Op::LoadConst, const_idx as u16);
            }
            Expr::Float(float_lit) => {
                let const_idx = self.current().add_constant(Constant::Float(float_lit.value));
                self.emit1(Op::LoadConst, const_idx as u16);
            }
            Expr::Bool(bool_lit) => {
                self.emit(if bool_lit.value { Op::True } else { Op::False });
            }
            Expr::Nil(_) => {
                self.emit(Op::Nil);
            }
            Expr::String(string_lit) => {
                let const_idx = self
                    .current()
                    .add_constant(Constant::String(string_lit.value.clone().into()));
                self.emit1(Op::LoadConst, const_idx as u16);
            }
            Expr::Ident(ident) => self.compile_ident(ident),
            Expr::Prefix(prefix) => self.compile_prefix_expr(prefix),
            Expr::Infix(infix) => self.compile_infix_expr(infix),
            Expr::Spread(spread) => self.compile_spread_expr(spread),
            Expr::List(list) => self.compile_list_lit(list),
            Expr::Map(map) => self.compile_map_lit(map),
            Expr::Func(func) => self.compile_func_lit(func),
            Expr::Call(call) => self.compile_call_expr(call),
            Expr::GetAttr(get_attr) => self.compile_get_attr_expr(get_attr),
            Expr::ObjectCall(obj_call) => self.compile_object_call_expr(obj_call),
            Expr::Index(index) => self.compile_index_expr(index),
            Expr::Slice(slice) => self.compile_slice_expr(slice),
            Expr::If(if_expr) => self.compile_if_expr(if_expr),
            Expr::Switch(switch) => self.compile_switch_expr(switch),
            Expr::Match(match_expr) => self.compile_match_expr(match_expr),
            Expr::In(in_expr) => self.compile_in_expr(in_expr),
            Expr::NotIn(not_in) => self.compile_not_in_expr(not_in),
            Expr::Pipe(pipe) => self.compile_pipe_expr(pipe),
            Expr::Try(try_expr) => self.compile_try_expr(try_expr),
        }
    }

    fn compile_ident(&mut self, ident: &Ident) {
        let Some(resolution) = self.scopes.resolve(&ident.name) else {
            self.report_error(format!("undefined variable: {}", ident.name), ident.position);
            return;
        };
        self.load_resolution(&resolution);
    }

    fn compile_prefix_expr(&mut self, expr: &PrefixExpr) {
        self.compile_expr(&expr.right);

        match expr.op.as_str() {
            "-" => {
                self.emit(Op::UnaryNegative);
            }
            "!" | "not" => {
                self.emit(Op::UnaryNot);
            }
            _ => {
                self.report_error(format!("unknown prefix operator: {}", expr.op), expr.op_pos);
            }
        }
    }

    fn compile_infix_expr(&mut self, expr: &InfixExpr) {
        // Short-circuit operators
        if expr.op == "&&" {
            self.compile_expr(&expr.left);
            let jump_false = self.emit_jump_forward(Op::PopJumpForwardIfFalse);
            self.compile_expr(&expr.right);
            let jump_end = self.emit_jump_forward(Op::JumpForward);
            self.patch_jump(jump_false);
            self.emit(Op::False);
            self.patch_jump(jump_end);
            return;
        }

        if expr.op == "||" {
            self.compile_expr(&expr.left);
            let jump_true = self.emit_jump_forward(Op::PopJumpForwardIfTrue);
            self.compile_expr(&expr.right);
            let jump_end = self.emit_jump_forward(Op::JumpForward);
            self.patch_jump(jump_true);
            self.emit(Op::True);
            self.patch_jump(jump_end);
            return;
        }

        if expr.op == "??" {
            self.compile_expr(&expr.left);
            let jump_not_nil = self.emit_jump_forward(Op::PopJumpForwardIfNotNil);
            self.emit(Op::PopTop);
            self.compile_expr(&expr.right);
            self.patch_jump(jump_not_nil);
            return;
        }

        // Regular operators
        self.compile_expr(&expr.left);
        self.compile_expr(&expr.right);

        // Binary operations
        if let Some(bin_op) = self.get_binary_op(&expr.op) {
            self.emit1(Op::BinaryOp, bin_op as u16);
            return;
        }

        // Comparison operations
        if let Some(cmp_op) = self.get_compare_op(&expr.op) {
            self.emit1(Op::CompareOp, cmp_op as u16);
            return;
        }

        self.report_error(format!("unknown operator: {}", expr.op), expr.op_pos);
    }

    fn compile_spread_expr(&mut self, expr: &SpreadExpr) {
        if let Some(e) = &expr.expr {
            self.compile_expr(e);
        }
    }

    fn compile_list_lit(&mut self, expr: &ListLit) {
        let mut has_spread = false;
        let mut count = 0;

        for item in &expr.items {
            if let Expr::Spread(spread) = item {
                if !has_spread && count > 0 {
                    self.emit1(Op::BuildList, count);
                }
                has_spread = true;
                if let Some(e) = &spread.expr {
                    self.compile_expr(e);
                    if count > 0 || expr.items.iter().position(|i| std::ptr::eq(i, item)).unwrap() > 0
                    {
                        self.emit1(Op::ListExtend, 0);
                    }
                }
                count = 0;
            } else {
                self.compile_expr(item);
                if has_spread {
                    self.emit1(Op::ListAppend, 0);
                }
                count += 1;
            }
        }

        if !has_spread {
            self.emit1(Op::BuildList, count);
        } else if count > 0 {
            self.emit1(Op::BuildList, count);
            self.emit1(Op::ListExtend, 0);
        }
    }

    fn compile_map_lit(&mut self, expr: &MapLit) {
        let mut has_spread = false;
        let mut count = 0;

        for item in &expr.items {
            if item.key.is_none() {
                // Spread
                if !has_spread && count > 0 {
                    self.emit1(Op::BuildMap, count);
                }
                has_spread = true;
                self.compile_expr(&item.value);
                if count > 0 || expr.items.iter().position(|i| std::ptr::eq(i, item)).unwrap() > 0 {
                    self.emit1(Op::MapMerge, 0);
                }
                count = 0;
            } else {
                // Key-value pair - for identifier keys, use the name as a string constant
                if let Some(Expr::Ident(ident)) = &item.key {
                    let const_idx = self
                        .current()
                        .add_constant(Constant::String(ident.name.clone().into()));
                    self.emit1(Op::LoadConst, const_idx as u16);
                } else if let Some(key) = &item.key {
                    self.compile_expr(key);
                }
                self.compile_expr(&item.value);
                if has_spread {
                    self.emit1(Op::MapSet, 0);
                }
                count += 1;
            }
        }

        if !has_spread {
            self.emit1(Op::BuildMap, count);
        } else if count > 0 {
            self.emit1(Op::BuildMap, count);
            self.emit1(Op::MapMerge, 0);
        }
    }

    fn compile_func_lit(&mut self, func: &FuncLit) {
        // Save current state
        let parent_idx = self.current_idx;

        // Create new scope for function
        let func_id = format!("{}", self.func_index);
        self.func_index += 1;
        let func_name = func.name.as_ref().map(|n| n.name.clone()).unwrap_or_default();

        self.scopes.enter_function();

        // Create code builder for function
        let child_idx = if parent_idx == 0 {
            self.main.create_child(func_id, func_name.clone(), func.name.is_some())
        } else {
            self.children_stack[parent_idx - 1].create_child(
                func_id,
                func_name.clone(),
                func.name.is_some(),
            )
        };

        // Move child to stack for editing
        let child = if parent_idx == 0 {
            std::mem::replace(
                &mut self.main.children[child_idx],
                CodeBuilder::new(
                    String::new(),
                    String::new(),
                    false,
                    String::new(),
                    String::new(),
                ),
            )
        } else {
            std::mem::replace(
                &mut self.children_stack[parent_idx - 1].children[child_idx],
                CodeBuilder::new(
                    String::new(),
                    String::new(),
                    false,
                    String::new(),
                    String::new(),
                ),
            )
        };

        self.children_stack.push(child);
        self.current_idx = self.children_stack.len();

        // Register parameters
        for param in &func.params {
            match param {
                FuncParam::Ident(ident) => {
                    self.scopes.insert_variable(&ident.name);
                }
                FuncParam::ObjectDestructure { .. } => {
                    self.scopes.current_mut().claim_slot();
                }
                FuncParam::ArrayDestructure { .. } => {
                    self.scopes.current_mut().claim_slot();
                }
            }
        }

        // Register rest parameter
        if let Some(rest) = &func.rest_param {
            self.scopes.insert_variable(&rest.name);
        }

        // Compile function body
        for stmt in &func.body.stmts {
            self.compile_statement(stmt);
        }

        // Ensure function returns
        let last_was_return = func
            .body
            .stmts
            .last()
            .map(|s| matches!(s, Stmt::Return(_)))
            .unwrap_or(false);
        if !last_was_return {
            self.emit(Op::Nil);
            self.emit(Op::ReturnValue);
        }

        // Get free variables and local info before restoring scope
        let scope = self.scopes.exit_scope();
        let free_vars: Vec<_> = scope.free_vars().to_vec();
        let local_count = scope.local_count();
        let local_names = scope.local_names();

        // Move child back from stack
        let mut completed_child = self.children_stack.pop().unwrap();

        // Convert child to Code and add as constant
        let func_code = std::mem::replace(
            &mut completed_child,
            CodeBuilder::new(String::new(), String::new(), false, String::new(), String::new()),
        )
        .to_code(local_count, 0, local_names, vec![]);

        // Put placeholder back
        if parent_idx == 0 {
            self.main.children[child_idx] = completed_child;
        } else {
            self.children_stack[parent_idx - 1].children[child_idx] = completed_child;
        }

        // Restore parent state
        self.current_idx = parent_idx;

        // Add function as constant
        let func_const_idx = self.current().add_constant(Constant::Function(func_code));

        // Emit closure loading with free variables
        if !free_vars.is_empty() {
            for free_var in &free_vars {
                self.emit2(
                    Op::MakeCell,
                    free_var.symbol.index,
                    (free_var.depth - 1) as u16,
                );
            }
            self.emit2(Op::LoadClosure, func_const_idx as u16, free_vars.len() as u16);
        } else {
            self.emit1(Op::LoadConst, func_const_idx as u16);
        }
    }

    fn compile_call_expr(&mut self, expr: &CallExpr) {
        self.compile_expr(&expr.func);

        let mut has_spread = false;
        let arg_count = expr.args.len();

        for arg in &expr.args {
            if matches!(arg, Expr::Spread(_)) {
                has_spread = true;
            }
            self.compile_expr(arg);
        }

        self.current().update_max_call_args(arg_count);

        if has_spread {
            self.emit1(Op::CallSpread, arg_count as u16);
        } else {
            self.emit1(Op::Call, arg_count as u16);
        }
    }

    fn compile_get_attr_expr(&mut self, expr: &GetAttrExpr) {
        self.compile_expr(&expr.object);
        let name_index = self.current().add_name(&expr.attr.name);

        if expr.optional {
            let skip_if_nil = self.emit_jump_forward(Op::PopJumpForwardIfNil);
            self.emit1(Op::LoadAttr, name_index as u16);
            let skip_end = self.emit_jump_forward(Op::JumpForward);
            self.patch_jump(skip_if_nil);
            self.emit(Op::Nil);
            self.patch_jump(skip_end);
        } else {
            self.emit1(Op::LoadAttr, name_index as u16);
        }
    }

    fn compile_object_call_expr(&mut self, expr: &ObjectCallExpr) {
        self.compile_expr(&expr.object);

        // Get method name
        let method_name = if let Expr::Ident(ident) = &expr.call.func {
            &ident.name
        } else {
            self.report_error("expected method name", expr.call.func.pos());
            return;
        };
        let name_index = self.current().add_name(method_name);

        if expr.optional {
            let skip_if_nil = self.emit_jump_forward(Op::PopJumpForwardIfNil);

            self.emit1(Op::Copy, 0);
            self.emit1(Op::LoadAttr, name_index as u16);
            self.emit1(Op::Swap, 1);

            for arg in &expr.call.args {
                self.compile_expr(arg);
            }

            self.emit1(Op::Call, (expr.call.args.len() + 1) as u16);

            let skip_end = self.emit_jump_forward(Op::JumpForward);
            self.patch_jump(skip_if_nil);
            self.emit(Op::Nil);
            self.patch_jump(skip_end);
        } else {
            self.emit1(Op::Copy, 0);
            self.emit1(Op::LoadAttr, name_index as u16);
            self.emit1(Op::Swap, 1);

            for arg in &expr.call.args {
                self.compile_expr(arg);
            }

            self.current().update_max_call_args(expr.call.args.len() + 1);
            self.emit1(Op::Call, (expr.call.args.len() + 1) as u16);
        }
    }

    fn compile_index_expr(&mut self, expr: &IndexExpr) {
        self.compile_expr(&expr.object);
        self.compile_expr(&expr.index);
        self.emit1(Op::BinarySubscr, 0);
    }

    fn compile_slice_expr(&mut self, expr: &SliceExpr) {
        self.compile_expr(&expr.object);

        if let Some(low) = &expr.low {
            self.compile_expr(low);
        } else {
            self.emit(Op::Nil);
        }

        if let Some(high) = &expr.high {
            self.compile_expr(high);
        } else {
            self.emit(Op::Nil);
        }

        self.emit2(Op::Slice, 0, 0);
    }

    fn compile_if_expr(&mut self, expr: &IfExpr) {
        self.compile_expr(&expr.condition);

        let jump_false = self.emit_jump_forward(Op::PopJumpForwardIfFalse);

        // Compile consequence - keep the last expression's value
        self.compile_block_as_expr(&expr.consequence.stmts);

        if let Some(alt) = &expr.alternative {
            let jump_end = self.emit_jump_forward(Op::JumpForward);
            self.patch_jump(jump_false);

            self.compile_block_as_expr(&alt.stmts);

            self.patch_jump(jump_end);
        } else {
            let jump_end = self.emit_jump_forward(Op::JumpForward);
            self.patch_jump(jump_false);
            self.emit(Op::Nil);
            self.patch_jump(jump_end);
        }
    }

    /// Compile a block that should return the value of its last expression
    fn compile_block_as_expr(&mut self, stmts: &[Stmt]) {
        if stmts.is_empty() {
            self.emit(Op::Nil);
            return;
        }

        let last_idx = stmts.len() - 1;
        for (i, stmt) in stmts.iter().enumerate() {
            let is_last = i == last_idx;
            if is_last {
                // For the last statement, if it's an expression, keep its value
                if let Stmt::Expr(expr) = stmt {
                    self.compile_expr(expr);
                    // Don't pop - this is the block's result
                } else {
                    self.compile_statement(stmt);
                    self.emit(Op::Nil);
                }
            } else {
                self.compile_statement(stmt);
            }
        }
    }

    fn compile_switch_expr(&mut self, expr: &SwitchExpr) {
        self.compile_expr(&expr.value);

        let mut jump_ends = Vec::new();

        for case in &expr.cases {
            if case.is_default {
                self.emit(Op::PopTop);
                for stmt in &case.body.stmts {
                    self.compile_statement(stmt);
                }
                self.emit(Op::Nil);
                break;
            }

            let mut next_case_jumps = Vec::new();
            if let Some(exprs) = &case.exprs {
                for case_expr in exprs {
                    self.emit1(Op::Copy, 0);
                    self.compile_expr(case_expr);
                    self.emit1(Op::CompareOp, CompareOpType::Eq as u16);
                    let jump_match = self.emit_jump_forward(Op::PopJumpForwardIfTrue);
                    next_case_jumps.push(jump_match);
                }
            }

            let jump_next_case = self.emit_jump_forward(Op::JumpForward);

            for jump in next_case_jumps {
                self.patch_jump(jump);
            }

            self.emit(Op::PopTop);
            for stmt in &case.body.stmts {
                self.compile_statement(stmt);
            }
            self.emit(Op::Nil);
            jump_ends.push(self.emit_jump_forward(Op::JumpForward));

            self.patch_jump(jump_next_case);
        }

        for jump in jump_ends {
            self.patch_jump(jump);
        }
    }

    fn compile_match_expr(&mut self, expr: &MatchExpr) {
        self.compile_expr(&expr.subject);

        let mut jump_ends = Vec::new();

        for arm in &expr.arms {
            self.emit1(Op::Copy, 0);

            if let Pattern::Literal(lit_expr) = &arm.pattern {
                self.compile_expr(lit_expr);
            }

            self.emit1(Op::CompareOp, CompareOpType::Eq as u16);

            if let Some(guard) = &arm.guard {
                let jump_no_match = self.emit_jump_forward(Op::PopJumpForwardIfFalse);
                self.compile_expr(guard);
                let jump_no_guard = self.emit_jump_forward(Op::PopJumpForwardIfFalse);

                self.emit(Op::PopTop);
                self.compile_expr(&arm.result);
                jump_ends.push(self.emit_jump_forward(Op::JumpForward));

                self.patch_jump(jump_no_match);
                self.patch_jump(jump_no_guard);
            } else {
                let jump_no_match = self.emit_jump_forward(Op::PopJumpForwardIfFalse);

                self.emit(Op::PopTop);
                self.compile_expr(&arm.result);
                jump_ends.push(self.emit_jump_forward(Op::JumpForward));

                self.patch_jump(jump_no_match);
            }
        }

        // Default arm
        if let Some(default) = &expr.default_arm {
            self.emit(Op::PopTop);
            self.compile_expr(&default.result);
        } else {
            self.emit(Op::PopTop);
            self.emit(Op::Nil);
        }

        for jump in jump_ends {
            self.patch_jump(jump);
        }
    }

    fn compile_in_expr(&mut self, expr: &InExpr) {
        self.compile_expr(&expr.right);
        self.compile_expr(&expr.left);
        self.emit1(Op::ContainsOp, 0);
    }

    fn compile_not_in_expr(&mut self, expr: &NotInExpr) {
        self.compile_expr(&expr.right);
        self.compile_expr(&expr.left);
        self.emit1(Op::ContainsOp, 0);
        self.emit(Op::UnaryNot);
    }

    fn compile_pipe_expr(&mut self, expr: &PipeExpr) {
        // Compile first expression
        self.compile_expr(&expr.exprs[0]);

        // For each subsequent expression, pass result as argument
        for pipe_arg in &expr.exprs[1..] {
            if let Expr::Call(call) = pipe_arg {
                self.compile_expr(&call.func);
                self.emit1(Op::Swap, 1);

                for arg in &call.args {
                    self.compile_expr(arg);
                }

                self.emit1(Op::Call, (call.args.len() + 1) as u16);
            } else if let Expr::Ident(_) = pipe_arg {
                self.compile_expr(pipe_arg);
                self.emit1(Op::Swap, 1);
                self.emit1(Op::Call, 1);
            } else {
                self.compile_expr(pipe_arg);
                self.emit1(Op::Partial, 0);
            }
        }
    }

    fn compile_try_expr(&mut self, expr: &TryExpr) {
        let try_start = self.current().offset();

        self.emit2(Op::PushExcept, PLACEHOLDER, PLACEHOLDER);
        let push_except_offset = self.current().offset() - 2;

        // Compile try body
        for stmt in &expr.body.stmts {
            self.compile_statement(stmt);
        }
        self.emit(Op::Nil);
        self.emit(Op::PopExcept);

        let jump_after_try = self.emit_jump_forward(Op::JumpForward);

        // Catch block
        let catch_start_offset = if expr.catch_block.is_some() {
            let offset = self.current().offset();

            if let Some(catch_ident) = &expr.catch_ident {
                let symbol = self.scopes.insert_variable(&catch_ident.name);
                self.store_symbol(&symbol);
            } else {
                self.emit(Op::PopTop);
            }

            if let Some(catch_block) = &expr.catch_block {
                for stmt in &catch_block.stmts {
                    self.compile_statement(stmt);
                }
            }
            self.emit(Op::Nil);
            Some(offset)
        } else {
            None
        };

        let jump_after_catch = self.emit_jump_forward(Op::JumpForward);

        // Finally block
        let finally_start_offset = if let Some(finally_block) = &expr.finally_block {
            let offset = self.current().offset();

            for stmt in &finally_block.stmts {
                self.compile_statement(stmt);
            }
            self.emit(Op::EndFinally);
            Some(offset)
        } else {
            None
        };

        // Patch jumps
        self.patch_jump(jump_after_try);
        self.patch_jump(jump_after_catch);

        // Patch PushExcept operands
        if let Some(offset) = catch_start_offset {
            self.current()
                .patch(push_except_offset, (offset - push_except_offset) as u16);
        }
        if let Some(offset) = finally_start_offset {
            self.current()
                .patch(push_except_offset + 1, (offset - push_except_offset) as u16);
        }

        // Add exception handler entry
        self.current().add_exception_handler(risor_bytecode::ExceptionHandler {
            start: try_start,
            end: catch_start_offset.unwrap_or(finally_start_offset.unwrap_or(try_start)),
            catch_offset: catch_start_offset.unwrap_or(usize::MAX),
            finally_offset: finally_start_offset.unwrap_or(usize::MAX),
            catch_var: expr.catch_ident.as_ref().map(|i| i.name.clone()),
        });
    }

    // ===========================================================================
    // Helper Methods
    // ===========================================================================

    fn emit(&mut self, opcode: Op) -> usize {
        self.current().emit(opcode as u16)
    }

    fn emit1(&mut self, opcode: Op, operand: u16) -> usize {
        self.current().emit1(opcode as u16, operand)
    }

    fn emit2(&mut self, opcode: Op, operand1: u16, operand2: u16) -> usize {
        self.current().emit2(opcode as u16, operand1, operand2)
    }

    fn emit_jump_forward(&mut self, opcode: Op) -> usize {
        self.emit1(opcode, PLACEHOLDER)
    }

    fn patch_jump(&mut self, offset: usize) {
        let current_offset = self.current().offset();
        let jump_distance = (current_offset - offset - 2) as u16;
        self.current().patch(offset + 1, jump_distance);
    }

    fn load_resolution(&mut self, resolution: &Resolution) {
        match resolution.scope {
            Scope::Local => self.emit1(Op::LoadFast, resolution.symbol.index),
            Scope::Global => self.emit1(Op::LoadGlobal, resolution.symbol.index),
            Scope::Free => self.emit1(Op::LoadFree, resolution.free_index as u16),
        };
    }

    fn store_resolution(&mut self, resolution: &Resolution) {
        match resolution.scope {
            Scope::Local => self.emit1(Op::StoreFast, resolution.symbol.index),
            Scope::Global => self.emit1(Op::StoreGlobal, resolution.symbol.index),
            Scope::Free => self.emit1(Op::StoreFree, resolution.free_index as u16),
        };
    }

    fn store_symbol(&mut self, symbol: &Symbol) {
        if self.scopes.is_root() {
            self.emit1(Op::StoreGlobal, symbol.index);
        } else {
            self.emit1(Op::StoreFast, symbol.index);
        }
    }

    fn get_binary_op(&self, op: &str) -> Option<BinaryOpType> {
        match op {
            "+" => Some(BinaryOpType::Add),
            "-" => Some(BinaryOpType::Subtract),
            "*" => Some(BinaryOpType::Multiply),
            "/" => Some(BinaryOpType::Divide),
            "%" => Some(BinaryOpType::Modulo),
            "**" => Some(BinaryOpType::Power),
            "^" => Some(BinaryOpType::Xor),
            "<<" => Some(BinaryOpType::LShift),
            ">>" => Some(BinaryOpType::RShift),
            "&" => Some(BinaryOpType::BitwiseAnd),
            "|" => Some(BinaryOpType::BitwiseOr),
            _ => None,
        }
    }

    fn get_compare_op(&self, op: &str) -> Option<CompareOpType> {
        match op {
            "<" => Some(CompareOpType::Lt),
            "<=" => Some(CompareOpType::LtEquals),
            "==" => Some(CompareOpType::Eq),
            "!=" => Some(CompareOpType::NotEq),
            ">" => Some(CompareOpType::Gt),
            ">=" => Some(CompareOpType::GtEquals),
            _ => None,
        }
    }

    fn compile_compound_op(&mut self, op: &str) {
        match op {
            "+=" => {
                self.emit1(Op::BinaryOp, BinaryOpType::Add as u16);
            }
            "-=" => {
                self.emit1(Op::BinaryOp, BinaryOpType::Subtract as u16);
            }
            "*=" => {
                self.emit1(Op::BinaryOp, BinaryOpType::Multiply as u16);
            }
            "/=" => {
                self.emit1(Op::BinaryOp, BinaryOpType::Divide as u16);
            }
            _ => {
                self.report_error(
                    format!("unknown compound operator: {}", op),
                    Position::default(),
                );
            }
        }
    }

    fn report_error(&mut self, message: impl Into<String>, position: Position) {
        if self.error.is_none() {
            self.error = Some(CompilerError::new(message, position));
        }
    }
}

/// Compile source code to bytecode.
pub fn compile(source: &str, config: CompilerConfig) -> Result<Rc<Code>, CompilerError> {
    let program = risor_parser::parse(source).map_err(|e| CompilerError::Compile {
        message: e.to_string(),
        line: 0,
        column: 0,
    })?;
    let compiler = Compiler::new(CompilerConfig {
        source: source.to_string(),
        ..config
    });
    compiler.compile(&program)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn compile_source(source: &str) -> Rc<Code> {
        compile(source, CompilerConfig::default()).expect("compilation failed")
    }

    #[test]
    fn test_compile_integers() {
        let code = compile_source("42");
        assert!(code.constants.iter().any(|c| matches!(c, Constant::Int(42))));
    }

    #[test]
    fn test_compile_strings() {
        let code = compile_source(r#""hello""#);
        assert!(code
            .constants
            .iter()
            .any(|c| matches!(c, Constant::String(s) if s.as_ref() == "hello")));
    }

    #[test]
    fn test_compile_variable() {
        let code = compile_source("let x = 42");
        // At the root level, variables are treated as globals
        assert!(code.global_names.contains(&"x".to_string()));
    }

    #[test]
    fn test_compile_list() {
        let code = compile_source("[1, 2, 3]");
        assert!(code
            .instructions
            .iter()
            .any(|&i| i == Op::BuildList as u16));
    }

    #[test]
    fn test_compile_map() {
        let code = compile_source("{a: 1, b: 2}");
        assert!(code.instructions.iter().any(|&i| i == Op::BuildMap as u16));
    }

    #[test]
    fn test_compile_function() {
        let code = compile_source("function f(x) { return x }");
        assert!(code
            .constants
            .iter()
            .any(|c| matches!(c, Constant::Function(_))));
    }

    #[test]
    fn test_compile_if() {
        let code = compile_source("if true { 1 }");
        assert!(code
            .instructions
            .iter()
            .any(|&i| i == Op::PopJumpForwardIfFalse as u16));
    }

    #[test]
    fn test_undefined_variable_error() {
        let result = compile("x", CompilerConfig::default());
        assert!(result.is_err());
    }

    #[test]
    fn test_constant_assignment_error() {
        let result = compile("const x = 1\nx = 2", CompilerConfig::default());
        assert!(result.is_err());
    }
}
