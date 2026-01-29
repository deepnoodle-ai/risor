//! Risor Virtual Machine - bytecode execution engine.

use std::cell::RefCell;
use std::collections::HashMap;
use std::rc::Rc;

use risor_bytecode::{BinaryOpType, Code, CompareOpType, Constant, Op};
use thiserror::Error;

use crate::frame::{ExceptionHandler, Frame};
use crate::object::{Builtin, BuiltinContext, Closure, Object, RisorIter, RisorMap};

/// Maximum call stack depth.
const MAX_STACK_DEPTH: usize = 1024;
/// Maximum call frame depth.
const MAX_FRAME_DEPTH: usize = 256;

/// VM execution error.
#[derive(Error, Debug)]
#[error("{message}")]
pub struct VMError {
    pub message: String,
    pub line: Option<usize>,
    pub column: Option<usize>,
}

impl VMError {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            line: None,
            column: None,
        }
    }
}

impl From<String> for VMError {
    fn from(message: String) -> Self {
        Self::new(message)
    }
}

/// VM configuration options.
#[derive(Default)]
pub struct VMConfig {
    /// Global variables (builtins, etc.).
    pub globals: HashMap<String, Object>,
}

/// Risor Virtual Machine.
pub struct VM {
    /// Value stack.
    stack: Vec<Object>,
    /// Stack pointer (index of next free slot).
    sp: usize,
    /// Call frames.
    frames: Vec<Frame>,
    /// Global variables.
    globals: Vec<Object>,
    /// Global names for lookup.
    global_names: HashMap<String, usize>,
    /// Exception handlers.
    exception_handlers: Vec<ExceptionHandler>,
    /// Pending exception for rethrow.
    pending_exception: Option<Object>,
}

impl VM {
    /// Create a new VM.
    pub fn new(config: VMConfig) -> Self {
        let mut vm = Self {
            stack: vec![Object::Nil; MAX_STACK_DEPTH],
            sp: 0,
            frames: Vec::with_capacity(MAX_FRAME_DEPTH),
            globals: Vec::new(),
            global_names: HashMap::new(),
            exception_handlers: Vec::new(),
            pending_exception: None,
        };

        // Initialize globals from config
        for (name, value) in config.globals {
            let index = vm.globals.len();
            vm.globals.push(value);
            vm.global_names.insert(name, index);
        }

        vm
    }

    /// Execute compiled bytecode.
    pub fn run(&mut self, code: Rc<Code>) -> Result<Object, VMError> {
        // Reset state
        self.sp = 0;
        self.frames.clear();
        self.exception_handlers.clear();

        // Register globals from code
        for name in &code.global_names {
            if !self.global_names.contains_key(name) {
                let index = self.globals.len();
                self.globals.push(Object::Nil);
                self.global_names.insert(name.clone(), index);
            }
        }

        // Extend globals array if needed
        while self.globals.len() < code.global_count {
            self.globals.push(Object::Nil);
        }

        // Create initial frame
        let locals = vec![Object::Nil; code.local_count];
        let frame = Frame::new(code, 0, locals, Vec::new());
        self.frames.push(frame);

        // Execute
        self.execute(0)
    }

    /// Main execution loop.
    fn execute(&mut self, min_frame_depth: usize) -> Result<Object, VMError> {
        while !self.current_frame().is_at_end() {
            // Check if we've returned from all frames at our level
            if self.frames.len() < min_frame_depth {
                return Ok(if self.sp > 0 {
                    self.stack[self.sp - 1].clone()
                } else {
                    Object::Nil
                });
            }

            let op = self.current_frame_mut().read_op();
            let op = unsafe { std::mem::transmute::<u16, Op>(op) };

            match op {
                // Execution Control
                Op::Nop => {}

                Op::Halt => {
                    return Ok(if self.sp > 0 {
                        self.pop()
                    } else {
                        Object::Nil
                    });
                }

                Op::Call => {
                    let arg_count = self.current_frame_mut().read_op() as usize;
                    self.op_call(arg_count)?;
                }

                Op::CallSpread => {
                    let arg_count = self.current_frame_mut().read_op() as usize;
                    self.op_call_spread(arg_count)?;
                }

                Op::ReturnValue => {
                    if self.op_return_value() {
                        return Ok(if self.sp > 0 {
                            self.pop()
                        } else {
                            Object::Nil
                        });
                    }
                }

                // Jumps
                Op::JumpForward => {
                    let offset = self.current_frame_mut().read_op() as usize;
                    self.current_frame_mut().jump_forward(offset);
                }

                Op::JumpBackward => {
                    let offset = self.current_frame_mut().read_op() as usize;
                    self.current_frame_mut().jump_backward(offset);
                }

                Op::PopJumpForwardIfFalse => {
                    let offset = self.current_frame_mut().read_op() as usize;
                    let value = self.pop();
                    if !value.is_truthy() {
                        self.current_frame_mut().jump_forward(offset);
                    }
                }

                Op::PopJumpForwardIfTrue => {
                    let offset = self.current_frame_mut().read_op() as usize;
                    let value = self.pop();
                    if value.is_truthy() {
                        self.current_frame_mut().jump_forward(offset);
                    }
                }

                Op::PopJumpForwardIfNil => {
                    let offset = self.current_frame_mut().read_op() as usize;
                    let value = self.pop();
                    if matches!(value, Object::Nil) {
                        self.current_frame_mut().jump_forward(offset);
                    }
                }

                Op::PopJumpForwardIfNotNil => {
                    let offset = self.current_frame_mut().read_op() as usize;
                    let value = self.pop();
                    if !matches!(value, Object::Nil) {
                        self.push(value);
                        self.current_frame_mut().jump_forward(offset);
                    }
                }

                // Load Operations
                Op::LoadConst => {
                    let index = self.current_frame_mut().read_op() as usize;
                    self.op_load_const(index)?;
                }

                Op::LoadFast => {
                    let index = self.current_frame_mut().read_op() as usize;
                    let value = self.current_frame().get_local(index).clone();
                    self.push(value);
                }

                Op::LoadGlobal => {
                    let index = self.current_frame_mut().read_op() as usize;
                    let value = self.globals[index].clone();
                    self.push(value);
                }

                Op::LoadFree => {
                    let index = self.current_frame_mut().read_op() as usize;
                    let value = self.current_frame().get_free(index);
                    self.push(value);
                }

                Op::LoadAttr => {
                    let name_index = self.current_frame_mut().read_op() as usize;
                    self.op_load_attr(name_index)?;
                }

                Op::LoadAttrOrNil => {
                    let name_index = self.current_frame_mut().read_op() as usize;
                    self.op_load_attr_or_nil(name_index)?;
                }

                // Store Operations
                Op::StoreFast => {
                    let index = self.current_frame_mut().read_op() as usize;
                    let value = self.pop();
                    self.current_frame_mut().set_local(index, value);
                }

                Op::StoreGlobal => {
                    let index = self.current_frame_mut().read_op() as usize;
                    let value = self.pop();
                    self.globals[index] = value;
                }

                Op::StoreFree => {
                    let index = self.current_frame_mut().read_op() as usize;
                    let value = self.pop();
                    self.current_frame().set_free(index, value);
                }

                Op::StoreAttr => {
                    let name_index = self.current_frame_mut().read_op() as usize;
                    self.op_store_attr(name_index)?;
                }

                // Binary/Unary Operations
                Op::BinaryOp => {
                    let op_type = self.current_frame_mut().read_op();
                    let op_type = unsafe { std::mem::transmute::<u16, BinaryOpType>(op_type) };
                    self.op_binary_op(op_type)?;
                }

                Op::CompareOp => {
                    let op_type = self.current_frame_mut().read_op();
                    let op_type = unsafe { std::mem::transmute::<u16, CompareOpType>(op_type) };
                    self.op_compare_op(op_type)?;
                }

                Op::UnaryNegative => {
                    self.op_unary_negative()?;
                }

                Op::UnaryNot => {
                    let value = self.pop();
                    self.push(Object::Bool(!value.is_truthy()));
                }

                // Container Building
                Op::BuildList => {
                    let count = self.current_frame_mut().read_op() as usize;
                    self.op_build_list(count)?;
                }

                Op::BuildMap => {
                    let count = self.current_frame_mut().read_op() as usize;
                    self.op_build_map(count)?;
                }

                Op::BuildString => {
                    let count = self.current_frame_mut().read_op() as usize;
                    self.op_build_string(count)?;
                }

                Op::ListAppend => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_list_append()?;
                }

                Op::ListExtend => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_list_extend()?;
                }

                Op::MapMerge => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_map_merge()?;
                }

                Op::MapSet => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_map_set()?;
                }

                // Container Access
                Op::BinarySubscr => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_binary_subscr()?;
                }

                Op::StoreSubscr => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_store_subscr()?;
                }

                Op::ContainsOp => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_contains_op()?;
                }

                Op::Length => {
                    let _ = self.current_frame_mut().read_op();
                    self.op_length()?;
                }

                Op::Slice => {
                    let _ = self.current_frame_mut().read_op();
                    let _ = self.current_frame_mut().read_op();
                    self.op_slice()?;
                }

                Op::Unpack => {
                    let count = self.current_frame_mut().read_op() as usize;
                    self.op_unpack(count)?;
                }

                // Stack Manipulation
                Op::Swap => {
                    let depth = self.current_frame_mut().read_op() as usize;
                    let top = self.stack[self.sp - 1].clone();
                    self.stack[self.sp - 1] = self.stack[self.sp - 1 - depth].clone();
                    self.stack[self.sp - 1 - depth] = top;
                }

                Op::Copy => {
                    let depth = self.current_frame_mut().read_op() as usize;
                    let value = self.stack[self.sp - 1 - depth].clone();
                    self.push(value);
                }

                Op::PopTop => {
                    self.pop();
                }

                // Constants
                Op::Nil => {
                    self.push(Object::Nil);
                }

                Op::False => {
                    self.push(Object::Bool(false));
                }

                Op::True => {
                    self.push(Object::Bool(true));
                }

                // Closures
                Op::LoadClosure => {
                    let const_index = self.current_frame_mut().read_op() as usize;
                    let free_count = self.current_frame_mut().read_op() as usize;
                    self.op_load_closure(const_index, free_count)?;
                }

                Op::MakeCell => {
                    let local_index = self.current_frame_mut().read_op() as usize;
                    let depth = self.current_frame_mut().read_op() as usize;
                    self.op_make_cell(local_index, depth)?;
                }

                // Partial Application
                Op::Partial => {
                    let _ = self.current_frame_mut().read_op();
                    // For now, leave value on stack
                }

                // Exception Handling
                Op::PushExcept => {
                    let catch_offset = self.current_frame_mut().read_op() as usize;
                    let finally_offset = self.current_frame_mut().read_op() as usize;
                    self.op_push_except(catch_offset, finally_offset);
                }

                Op::PopExcept => {
                    self.exception_handlers.pop();
                }

                Op::Throw => {
                    self.op_throw()?;
                }

                Op::EndFinally => {
                    self.op_end_finally()?;
                }
            }
        }

        Ok(if self.sp > 0 {
            self.pop()
        } else {
            Object::Nil
        })
    }

    // ===========================================================================
    // Helper Methods
    // ===========================================================================

    fn current_frame(&self) -> &Frame {
        self.frames.last().unwrap()
    }

    fn current_frame_mut(&mut self) -> &mut Frame {
        self.frames.last_mut().unwrap()
    }

    fn push(&mut self, value: Object) {
        if self.sp >= MAX_STACK_DEPTH {
            panic!("stack overflow");
        }
        self.stack[self.sp] = value;
        self.sp += 1;
    }

    fn pop(&mut self) -> Object {
        if self.sp == 0 {
            panic!("stack underflow");
        }
        self.sp -= 1;
        std::mem::replace(&mut self.stack[self.sp], Object::Nil)
    }

    fn peek(&self, depth: usize) -> &Object {
        &self.stack[self.sp - 1 - depth]
    }

    // ===========================================================================
    // Opcode Implementations
    // ===========================================================================

    fn op_load_const(&mut self, index: usize) -> Result<(), VMError> {
        let constant = self.current_frame().get_constant(index).clone();
        let value = match constant {
            Constant::Nil => Object::Nil,
            Constant::Bool(b) => Object::Bool(b),
            Constant::Int(n) => Object::Int(n),
            Constant::Float(n) => Object::Float(n),
            Constant::String(s) => Object::String(s),
            Constant::Function(code) => Object::Closure(Rc::new(Closure::new(code, Vec::new()))),
        };
        self.push(value);
        Ok(())
    }

    fn op_load_attr(&mut self, name_index: usize) -> Result<(), VMError> {
        let obj = self.pop();
        let name = self.current_frame().get_name(name_index).to_string();

        // Check for method first
        if let Some(method) = self.get_method(&obj, &name) {
            self.push(method);
            return Ok(());
        }

        // Then check for property
        if let Object::Map(map) = &obj {
            if let Some(value) = map.borrow().get(&Object::String(name.clone().into())) {
                self.push(value);
                return Ok(());
            }
        }

        Err(VMError::new(format!(
            "attribute '{}' not found on {}",
            name,
            obj.type_name()
        )))
    }

    fn op_load_attr_or_nil(&mut self, name_index: usize) -> Result<(), VMError> {
        let obj = self.pop();
        let name = self.current_frame().get_name(name_index).to_string();

        // Check for method first
        if let Some(method) = self.get_method(&obj, &name) {
            self.push(method);
            return Ok(());
        }

        // Then check for property
        if let Object::Map(map) = &obj {
            if let Some(value) = map.borrow().get(&Object::String(name.clone().into())) {
                self.push(value);
                return Ok(());
            }
        }

        self.push(Object::Nil);
        Ok(())
    }

    fn op_store_attr(&mut self, name_index: usize) -> Result<(), VMError> {
        let value = self.pop();
        let obj = self.pop();
        let name = self.current_frame().get_name(name_index).to_string();

        if let Object::Map(map) = &obj {
            map.borrow_mut().set(Object::String(name.into()), value);
            return Ok(());
        }

        Err(VMError::new(format!(
            "cannot set attribute '{}' on {}",
            name,
            obj.type_name()
        )))
    }

    fn op_binary_op(&mut self, op_type: BinaryOpType) -> Result<(), VMError> {
        let right = self.pop();
        let left = self.pop();

        let result = match op_type {
            BinaryOpType::Add => self.add(&left, &right)?,
            BinaryOpType::Subtract => self.subtract(&left, &right)?,
            BinaryOpType::Multiply => self.multiply(&left, &right)?,
            BinaryOpType::Divide => self.divide(&left, &right)?,
            BinaryOpType::Modulo => self.modulo(&left, &right)?,
            BinaryOpType::Power => self.power(&left, &right)?,
            BinaryOpType::BitwiseAnd => self.bitwise_and(&left, &right)?,
            BinaryOpType::BitwiseOr => self.bitwise_or(&left, &right)?,
            BinaryOpType::Xor => self.bitwise_xor(&left, &right)?,
            BinaryOpType::LShift => self.left_shift(&left, &right)?,
            BinaryOpType::RShift => self.right_shift(&left, &right)?,
            _ => return Err(VMError::new(format!("unknown binary operation"))),
        };

        self.push(result);
        Ok(())
    }

    fn op_compare_op(&mut self, op_type: CompareOpType) -> Result<(), VMError> {
        let right = self.pop();
        let left = self.pop();

        let result = match op_type {
            CompareOpType::Eq => left.equals(&right),
            CompareOpType::NotEq => !left.equals(&right),
            CompareOpType::Lt => left
                .compare(&right)
                .map(|o| o.is_lt())
                .ok_or_else(|| VMError::new("cannot compare values"))?,
            CompareOpType::LtEquals => left
                .compare(&right)
                .map(|o| o.is_le())
                .ok_or_else(|| VMError::new("cannot compare values"))?,
            CompareOpType::Gt => left
                .compare(&right)
                .map(|o| o.is_gt())
                .ok_or_else(|| VMError::new("cannot compare values"))?,
            CompareOpType::GtEquals => left
                .compare(&right)
                .map(|o| o.is_ge())
                .ok_or_else(|| VMError::new("cannot compare values"))?,
        };

        self.push(Object::Bool(result));
        Ok(())
    }

    fn op_unary_negative(&mut self) -> Result<(), VMError> {
        let value = self.pop();
        let result = match value {
            Object::Int(n) => Object::Int(-n),
            Object::Float(n) => Object::Float(-n),
            _ => {
                return Err(VMError::new(format!(
                    "cannot negate {}",
                    value.type_name()
                )))
            }
        };
        self.push(result);
        Ok(())
    }

    fn op_build_list(&mut self, count: usize) -> Result<(), VMError> {
        let mut items = Vec::with_capacity(count);
        for _ in 0..count {
            items.push(self.pop());
        }
        items.reverse();
        self.push(Object::List(Rc::new(RefCell::new(items))));
        Ok(())
    }

    fn op_build_map(&mut self, count: usize) -> Result<(), VMError> {
        let mut map = RisorMap::new();
        for _ in 0..count {
            let value = self.pop();
            let key = self.pop();
            map.set(key, value);
        }
        self.push(Object::Map(Rc::new(RefCell::new(map))));
        Ok(())
    }

    fn op_build_string(&mut self, count: usize) -> Result<(), VMError> {
        let mut parts = Vec::with_capacity(count);
        for _ in 0..count {
            let obj = self.pop();
            let s = match &obj {
                Object::String(s) => s.to_string(),
                _ => format!("{}", obj),
            };
            parts.push(s);
        }
        parts.reverse();
        self.push(Object::String(parts.join("").into()));
        Ok(())
    }

    fn op_list_append(&mut self) -> Result<(), VMError> {
        let item = self.pop();
        let list = self.peek(0);
        if let Object::List(items) = list {
            items.borrow_mut().push(item);
            Ok(())
        } else {
            Err(VMError::new("expected list"))
        }
    }

    fn op_list_extend(&mut self) -> Result<(), VMError> {
        let source = self.pop();
        let list = self.peek(0);
        if let (Object::List(target), Object::List(source)) = (list, &source) {
            target.borrow_mut().extend(source.borrow().iter().cloned());
            Ok(())
        } else {
            Err(VMError::new("expected lists"))
        }
    }

    fn op_map_merge(&mut self) -> Result<(), VMError> {
        let source = self.pop();
        let map = self.peek(0);
        if let (Object::Map(target), Object::Map(source)) = (map, &source) {
            target.borrow_mut().merge(&source.borrow());
            Ok(())
        } else {
            Err(VMError::new("expected maps"))
        }
    }

    fn op_map_set(&mut self) -> Result<(), VMError> {
        let value = self.pop();
        let key = self.pop();
        let map = self.peek(0);
        if let Object::Map(map) = map {
            map.borrow_mut().set(key, value);
            Ok(())
        } else {
            Err(VMError::new("expected map"))
        }
    }

    fn op_binary_subscr(&mut self) -> Result<(), VMError> {
        let index = self.pop();
        let obj = self.pop();

        let result = match (&obj, &index) {
            (Object::List(items), Object::Int(i)) => {
                let items = items.borrow();
                let idx = if *i < 0 {
                    (items.len() as i64 + i) as usize
                } else {
                    *i as usize
                };
                if idx >= items.len() {
                    return Err(VMError::new(format!("list index out of range: {}", i)));
                }
                items[idx].clone()
            }
            (Object::Map(map), _) => map.borrow().get(&index).unwrap_or(Object::Nil),
            (Object::String(s), Object::Int(i)) => {
                let chars: Vec<char> = s.chars().collect();
                let idx = if *i < 0 {
                    (chars.len() as i64 + i) as usize
                } else {
                    *i as usize
                };
                if idx >= chars.len() {
                    return Err(VMError::new(format!("string index out of range: {}", i)));
                }
                Object::String(chars[idx].to_string().into())
            }
            _ => {
                return Err(VMError::new(format!(
                    "cannot index {}",
                    obj.type_name()
                )))
            }
        };

        self.push(result);
        Ok(())
    }

    fn op_store_subscr(&mut self) -> Result<(), VMError> {
        let value = self.pop();
        let index = self.pop();
        let obj = self.pop();

        match (&obj, &index) {
            (Object::List(items), Object::Int(i)) => {
                let mut items = items.borrow_mut();
                let idx = if *i < 0 {
                    (items.len() as i64 + i) as usize
                } else {
                    *i as usize
                };
                if idx >= items.len() {
                    return Err(VMError::new(format!("list index out of range: {}", i)));
                }
                items[idx] = value;
            }
            (Object::Map(map), _) => {
                map.borrow_mut().set(index, value);
            }
            _ => {
                return Err(VMError::new(format!(
                    "cannot index-assign to {}",
                    obj.type_name()
                )))
            }
        }

        Ok(())
    }

    fn op_contains_op(&mut self) -> Result<(), VMError> {
        let item = self.pop();
        let container = self.pop();

        let result = match &container {
            Object::List(items) => items.borrow().iter().any(|i| i.equals(&item)),
            Object::Map(map) => map.borrow().has(&item),
            Object::String(s) => {
                if let Object::String(sub) = &item {
                    s.contains(sub.as_ref())
                } else {
                    return Err(VMError::new("'in' requires string"));
                }
            }
            _ => {
                return Err(VMError::new(format!(
                    "cannot check membership in {}",
                    container.type_name()
                )))
            }
        };

        self.push(Object::Bool(result));
        Ok(())
    }

    fn op_length(&mut self) -> Result<(), VMError> {
        let obj = self.pop();
        let len = match &obj {
            Object::List(items) => items.borrow().len(),
            Object::Map(map) => map.borrow().len(),
            Object::String(s) => s.len(),
            _ => {
                return Err(VMError::new(format!(
                    "cannot get length of {}",
                    obj.type_name()
                )))
            }
        };
        self.push(Object::Int(len as i64));
        Ok(())
    }

    fn op_slice(&mut self) -> Result<(), VMError> {
        let high = self.pop();
        let low = self.pop();
        let obj = self.pop();

        let start = match &low {
            Object::Nil => None,
            Object::Int(n) => Some(*n as isize),
            _ => return Err(VMError::new("slice indices must be integers")),
        };

        let end = match &high {
            Object::Nil => None,
            Object::Int(n) => Some(*n as isize),
            _ => return Err(VMError::new("slice indices must be integers")),
        };

        let result = match &obj {
            Object::List(items) => {
                let items = items.borrow();
                let len = items.len() as isize;
                let s = start.map(|i| if i < 0 { (len + i).max(0) } else { i }).unwrap_or(0) as usize;
                let e = end.map(|i| if i < 0 { (len + i).max(0) } else { i }).unwrap_or(len as isize) as usize;
                let s = s.min(items.len());
                let e = e.min(items.len());
                Object::List(Rc::new(RefCell::new(items[s..e].to_vec())))
            }
            Object::String(str) => {
                let chars: Vec<char> = str.chars().collect();
                let len = chars.len() as isize;
                let s = start.map(|i| if i < 0 { (len + i).max(0) } else { i }).unwrap_or(0) as usize;
                let e = end.map(|i| if i < 0 { (len + i).max(0) } else { i }).unwrap_or(len as isize) as usize;
                let s = s.min(chars.len());
                let e = e.min(chars.len());
                Object::String(chars[s..e].iter().collect::<String>().into())
            }
            _ => {
                return Err(VMError::new(format!(
                    "cannot slice {}",
                    obj.type_name()
                )))
            }
        };

        self.push(result);
        Ok(())
    }

    fn op_unpack(&mut self, count: usize) -> Result<(), VMError> {
        let obj = self.pop();
        if let Object::List(items) = obj {
            let items = items.borrow();
            if items.len() < count {
                return Err(VMError::new(format!(
                    "not enough values to unpack: expected {}, got {}",
                    count,
                    items.len()
                )));
            }
            for i in (0..count).rev() {
                self.push(items[i].clone());
            }
            Ok(())
        } else {
            Err(VMError::new(format!("cannot unpack {}", obj.type_name())))
        }
    }

    fn op_call(&mut self, arg_count: usize) -> Result<(), VMError> {
        let callable = self.stack[self.sp - 1 - arg_count].clone();

        match callable {
            Object::Closure(closure) => {
                let mut args = Vec::with_capacity(arg_count);
                for _ in 0..arg_count {
                    args.push(self.pop());
                }
                args.reverse();
                self.pop(); // Pop closure

                if self.frames.len() >= MAX_FRAME_DEPTH {
                    return Err(VMError::new("call stack overflow"));
                }

                let frame = Frame::from_closure(
                    closure.code.clone(),
                    closure.free_vars.clone(),
                    self.sp,
                    args,
                );
                self.frames.push(frame);
            }
            Object::Builtin(builtin) => {
                let mut args = Vec::with_capacity(arg_count);
                for _ in 0..arg_count {
                    args.push(self.pop());
                }
                args.reverse();
                self.pop(); // Pop builtin

                let result = builtin.call(self, &args)?;
                self.push(result);
            }
            _ => {
                return Err(VMError::new(format!(
                    "cannot call {}",
                    callable.type_name()
                )))
            }
        }

        Ok(())
    }

    fn op_call_spread(&mut self, arg_count: usize) -> Result<(), VMError> {
        // Collect args, expanding lists
        let mut args = Vec::new();
        for _ in 0..arg_count {
            let arg = self.pop();
            if let Object::List(items) = arg {
                for item in items.borrow().iter().rev() {
                    args.push(item.clone());
                }
            } else {
                args.push(arg);
            }
        }
        args.reverse();

        let callable = self.pop();

        match callable {
            Object::Closure(closure) => {
                if self.frames.len() >= MAX_FRAME_DEPTH {
                    return Err(VMError::new("call stack overflow"));
                }

                let frame = Frame::from_closure(
                    closure.code.clone(),
                    closure.free_vars.clone(),
                    self.sp,
                    args,
                );
                self.frames.push(frame);
            }
            Object::Builtin(builtin) => {
                let result = builtin.call(self, &args)?;
                self.push(result);
            }
            _ => {
                return Err(VMError::new(format!(
                    "cannot call {}",
                    callable.type_name()
                )))
            }
        }

        Ok(())
    }

    fn op_return_value(&mut self) -> bool {
        let result = self.pop();
        self.frames.pop();

        if self.frames.is_empty() {
            self.push(result);
            return true;
        }

        self.push(result);
        false
    }

    fn op_load_closure(&mut self, const_index: usize, free_count: usize) -> Result<(), VMError> {
        let constant = self.current_frame().get_constant(const_index).clone();
        let code = match constant {
            Constant::Function(code) => code,
            _ => return Err(VMError::new("expected function constant for closure")),
        };

        let mut free_vars = Vec::with_capacity(free_count);
        for _ in 0..free_count {
            let cell = self.pop();
            if let Object::Cell(cell) = cell {
                free_vars.push(cell);
            } else {
                return Err(VMError::new("expected cell for closure free variable"));
            }
        }
        free_vars.reverse();

        self.push(Object::Closure(Rc::new(Closure::new(code, free_vars))));
        Ok(())
    }

    fn op_make_cell(&mut self, local_index: usize, depth: usize) -> Result<(), VMError> {
        if depth == 0 {
            let value = self.current_frame().get_local(local_index).clone();
            let cell = Rc::new(RefCell::new(value));
            self.push(Object::Cell(cell));
        } else {
            let cell = self.current_frame().free_vars[local_index].clone();
            self.push(Object::Cell(cell));
        }
        Ok(())
    }

    fn op_push_except(&mut self, catch_offset: usize, finally_offset: usize) {
        let ip = self.current_frame().ip;
        self.exception_handlers.push(ExceptionHandler {
            start: ip,
            end: 0,
            catch_offset: if catch_offset == 0xFFFF {
                usize::MAX
            } else {
                ip + catch_offset
            },
            finally_offset: if finally_offset == 0xFFFF {
                usize::MAX
            } else {
                ip + finally_offset
            },
            stack_depth: self.sp,
            frame_depth: self.frames.len(),
        });
    }

    fn op_throw(&mut self) -> Result<(), VMError> {
        let value = self.pop();
        let error = match &value {
            Object::Error(_) => value,
            _ => Object::Error(format!("{}", value).into()),
        };

        if !self.handle_exception(error.clone()) {
            if let Object::Error(msg) = error {
                return Err(VMError::new(msg.to_string()));
            }
        }
        Ok(())
    }

    fn op_end_finally(&mut self) -> Result<(), VMError> {
        if let Some(exc) = self.pending_exception.take() {
            if !self.handle_exception(exc.clone()) {
                if let Object::Error(msg) = exc {
                    return Err(VMError::new(msg.to_string()));
                }
            }
        }
        Ok(())
    }

    fn handle_exception(&mut self, error: Object) -> bool {
        while let Some(handler) = self.exception_handlers.pop() {
            // Unwind frames
            while self.frames.len() > handler.frame_depth {
                self.frames.pop();
            }

            self.sp = handler.stack_depth;

            if handler.catch_offset != usize::MAX {
                if let Some(frame) = self.frames.last_mut() {
                    frame.ip = handler.catch_offset;
                }
                self.push(error);
                return true;
            } else if handler.finally_offset != usize::MAX {
                if let Some(frame) = self.frames.last_mut() {
                    frame.ip = handler.finally_offset;
                }
                self.pending_exception = Some(error);
                return true;
            }
        }

        false
    }

    // ===========================================================================
    // Arithmetic Helpers
    // ===========================================================================

    fn add(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a + b)),
            (Object::Float(a), Object::Float(b)) => Ok(Object::Float(a + b)),
            (Object::Int(a), Object::Float(b)) => Ok(Object::Float(*a as f64 + b)),
            (Object::Float(a), Object::Int(b)) => Ok(Object::Float(a + *b as f64)),
            (Object::String(a), Object::String(b)) => {
                Ok(Object::String(format!("{}{}", a, b).into()))
            }
            (Object::List(a), Object::List(b)) => {
                let mut result = a.borrow().clone();
                result.extend(b.borrow().iter().cloned());
                Ok(Object::List(Rc::new(RefCell::new(result))))
            }
            _ => Err(VMError::new(format!(
                "cannot add {} and {}",
                left.type_name(),
                right.type_name()
            ))),
        }
    }

    fn subtract(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a - b)),
            (Object::Float(a), Object::Float(b)) => Ok(Object::Float(a - b)),
            (Object::Int(a), Object::Float(b)) => Ok(Object::Float(*a as f64 - b)),
            (Object::Float(a), Object::Int(b)) => Ok(Object::Float(a - *b as f64)),
            _ => Err(VMError::new(format!(
                "cannot subtract {} from {}",
                right.type_name(),
                left.type_name()
            ))),
        }
    }

    fn multiply(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a * b)),
            (Object::Float(a), Object::Float(b)) => Ok(Object::Float(a * b)),
            (Object::Int(a), Object::Float(b)) => Ok(Object::Float(*a as f64 * b)),
            (Object::Float(a), Object::Int(b)) => Ok(Object::Float(a * *b as f64)),
            (Object::String(s), Object::Int(n)) => {
                Ok(Object::String(s.repeat(*n as usize).into()))
            }
            _ => Err(VMError::new(format!(
                "cannot multiply {} and {}",
                left.type_name(),
                right.type_name()
            ))),
        }
    }

    fn divide(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        let r = right.as_number().ok_or_else(|| VMError::new("division requires numbers"))?;
        if r == 0.0 {
            return Err(VMError::new("division by zero"));
        }
        let l = left.as_number().ok_or_else(|| VMError::new("division requires numbers"))?;
        Ok(Object::Float(l / r))
    }

    fn modulo(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => {
                if *b == 0 {
                    return Err(VMError::new("modulo by zero"));
                }
                Ok(Object::Int(a % b))
            }
            _ => Err(VMError::new(format!(
                "cannot modulo {} and {}",
                left.type_name(),
                right.type_name()
            ))),
        }
    }

    fn power(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int((*a as f64).powi(*b as i32) as i64)),
            _ => {
                let l = left.as_number().ok_or_else(|| VMError::new("power requires numbers"))?;
                let r = right.as_number().ok_or_else(|| VMError::new("power requires numbers"))?;
                Ok(Object::Float(l.powf(r)))
            }
        }
    }

    fn bitwise_and(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a & b)),
            _ => Err(VMError::new("bitwise AND requires integers")),
        }
    }

    fn bitwise_or(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a | b)),
            _ => Err(VMError::new("bitwise OR requires integers")),
        }
    }

    fn bitwise_xor(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a ^ b)),
            _ => Err(VMError::new("bitwise XOR requires integers")),
        }
    }

    fn left_shift(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a << b)),
            _ => Err(VMError::new("left shift requires integers")),
        }
    }

    fn right_shift(&self, left: &Object, right: &Object) -> Result<Object, VMError> {
        match (left, right) {
            (Object::Int(a), Object::Int(b)) => Ok(Object::Int(a >> b)),
            _ => Err(VMError::new("right shift requires integers")),
        }
    }

    // ===========================================================================
    // Method Lookup
    // ===========================================================================

    fn get_method(&self, obj: &Object, name: &str) -> Option<Object> {
        match obj {
            Object::String(s) => self.get_string_method(s, name),
            Object::List(items) => self.get_list_method(items, name),
            Object::Map(map) => self.get_map_method(map, name),
            _ => None,
        }
    }

    fn get_string_method(&self, s: &Rc<str>, name: &str) -> Option<Object> {
        let s = s.clone();
        let builtin = match name {
            "len" => {
                let s = s.clone();
                Builtin::with_closure(name, move |_, _| Ok(Object::Int(s.len() as i64)))
            }
            "upper" => {
                let s = s.clone();
                Builtin::with_closure(name, move |_, _| Ok(Object::String(s.to_uppercase().into())))
            }
            "lower" => {
                let s = s.clone();
                Builtin::with_closure(name, move |_, _| Ok(Object::String(s.to_lowercase().into())))
            }
            "trim" => {
                let s = s.clone();
                Builtin::with_closure(name, move |_, _| Ok(Object::String(s.trim().into())))
            }
            "split" => {
                let s = s.clone();
                Builtin::with_closure(name, move |_, args| {
                    let sep = if args.len() > 1 {
                        match &args[1] {
                            Object::String(sep) => sep.to_string(),
                            _ => String::new(),
                        }
                    } else {
                        String::new()
                    };
                    let parts: Vec<Object> = s
                        .split(&sep)
                        .map(|p| Object::String(p.into()))
                        .collect();
                    Ok(Object::List(Rc::new(RefCell::new(parts))))
                })
            }
            "contains" => {
                let s = s.clone();
                Builtin::with_closure(name, move |_, args| {
                    if let Object::String(sub) = &args[1] {
                        Ok(Object::Bool(s.contains(sub.as_ref())))
                    } else {
                        Err("contains requires string".into())
                    }
                })
            }
            _ => return None,
        };
        Some(Object::Builtin(Rc::new(builtin)))
    }

    fn get_list_method(
        &self,
        items: &Rc<RefCell<Vec<Object>>>,
        name: &str,
    ) -> Option<Object> {
        let items = items.clone();
        let builtin = match name {
            "len" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, _| {
                    Ok(Object::Int(items.borrow().len() as i64))
                })
            }
            "append" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, args| {
                    items.borrow_mut().push(args[1].clone());
                    Ok(Object::Nil)
                })
            }
            "pop" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, _| {
                    items
                        .borrow_mut()
                        .pop()
                        .ok_or_else(|| "pop from empty list".into())
                })
            }
            "map" => {
                let items = items.clone();
                Builtin::with_closure(name, move |ctx, args| {
                    let func = &args[1];
                    let mut result = Vec::new();
                    for item in items.borrow().iter() {
                        result.push(ctx.call_function(func, &[item.clone()])?);
                    }
                    Ok(Object::List(Rc::new(RefCell::new(result))))
                })
            }
            "filter" => {
                let items = items.clone();
                Builtin::with_closure(name, move |ctx, args| {
                    let func = &args[1];
                    let mut result = Vec::new();
                    for item in items.borrow().iter() {
                        if ctx.call_function(func, &[item.clone()])?.is_truthy() {
                            result.push(item.clone());
                        }
                    }
                    Ok(Object::List(Rc::new(RefCell::new(result))))
                })
            }
            "reduce" => {
                let items = items.clone();
                Builtin::with_closure(name, move |ctx, args| {
                    let func = &args[1];
                    let items_ref = items.borrow();
                    let (mut acc, start) = if args.len() > 2 {
                        (args[2].clone(), 0)
                    } else {
                        (items_ref.first().cloned().unwrap_or(Object::Nil), 1)
                    };
                    for item in items_ref.iter().skip(start) {
                        acc = ctx.call_function(func, &[acc, item.clone()])?;
                    }
                    Ok(acc)
                })
            }
            "each" => {
                let items = items.clone();
                Builtin::with_closure(name, move |ctx, args| {
                    let func = &args[1];
                    for (i, item) in items.borrow().iter().enumerate() {
                        ctx.call_function(func, &[item.clone(), Object::Int(i as i64)])?;
                    }
                    Ok(Object::Nil)
                })
            }
            "join" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, args| {
                    let sep = if args.len() > 1 {
                        match &args[1] {
                            Object::String(s) => s.to_string(),
                            _ => String::new(),
                        }
                    } else {
                        String::new()
                    };
                    let parts: Vec<String> =
                        items.borrow().iter().map(|i| format!("{}", i)).collect();
                    Ok(Object::String(parts.join(&sep).into()))
                })
            }
            "reverse" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, _| {
                    let mut result = items.borrow().clone();
                    result.reverse();
                    Ok(Object::List(Rc::new(RefCell::new(result))))
                })
            }
            "sort" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, _| {
                    let mut result = items.borrow().clone();
                    result.sort_by(|a, b| a.compare(b).unwrap_or(std::cmp::Ordering::Equal));
                    Ok(Object::List(Rc::new(RefCell::new(result))))
                })
            }
            "contains" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, args| {
                    Ok(Object::Bool(items.borrow().iter().any(|i| i.equals(&args[1]))))
                })
            }
            "index" => {
                let items = items.clone();
                Builtin::with_closure(name, move |_, args| {
                    for (i, item) in items.borrow().iter().enumerate() {
                        if item.equals(&args[1]) {
                            return Ok(Object::Int(i as i64));
                        }
                    }
                    Ok(Object::Nil)
                })
            }
            _ => return None,
        };
        Some(Object::Builtin(Rc::new(builtin)))
    }

    fn get_map_method(&self, map: &Rc<RefCell<RisorMap>>, name: &str) -> Option<Object> {
        let map = map.clone();
        let builtin = match name {
            "len" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, _| {
                    Ok(Object::Int(map.borrow().len() as i64))
                })
            }
            "keys" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, _| {
                    Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(
                        map.borrow().keys(),
                    )))))
                })
            }
            "values" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, _| {
                    Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(
                        map.borrow().values(),
                    )))))
                })
            }
            "entries" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, _| {
                    let entries: Vec<Object> = map
                        .borrow()
                        .entries()
                        .into_iter()
                        .map(|(k, v)| Object::List(Rc::new(RefCell::new(vec![k, v]))))
                        .collect();
                    Ok(Object::Iter(Rc::new(RefCell::new(RisorIter::new(entries)))))
                })
            }
            "get" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, args| {
                    let value = map.borrow().get(&args[1]);
                    Ok(value.unwrap_or_else(|| {
                        if args.len() > 2 {
                            args[2].clone()
                        } else {
                            Object::Nil
                        }
                    }))
                })
            }
            "set" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, args| {
                    map.borrow_mut().set(args[1].clone(), args[2].clone());
                    Ok(Object::Nil)
                })
            }
            "delete" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, args| {
                    Ok(Object::Bool(map.borrow_mut().delete(&args[1])))
                })
            }
            "has" => {
                let map = map.clone();
                Builtin::with_closure(name, move |_, args| {
                    Ok(Object::Bool(map.borrow().has(&args[1])))
                })
            }
            "each" => {
                let map = map.clone();
                Builtin::with_closure(name, move |ctx, args| {
                    let func = &args[1];
                    for (key, value) in map.borrow().entries() {
                        ctx.call_function(func, &[key, value])?;
                    }
                    Ok(Object::Nil)
                })
            }
            _ => return None,
        };
        Some(Object::Builtin(Rc::new(builtin)))
    }
}

impl BuiltinContext for VM {
    fn call_function(&mut self, func: &Object, args: &[Object]) -> Result<Object, String> {
        match func {
            Object::Builtin(builtin) => builtin.call(self, args),
            Object::Closure(closure) => {
                let frame = Frame::from_closure(
                    closure.code.clone(),
                    closure.free_vars.clone(),
                    self.sp,
                    args.to_vec(),
                );
                self.frames.push(frame);

                let min_depth = self.frames.len();
                self.execute(min_depth).map_err(|e| e.message)
            }
            _ => Err(format!("cannot call {}", func.type_name())),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_vm_new() {
        let _vm = VM::new(VMConfig::default());
    }
}
