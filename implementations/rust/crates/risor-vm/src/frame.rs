//! Call frame management for the Risor VM.

use std::cell::RefCell;
use std::rc::Rc;

use risor_bytecode::Code;

use crate::object::Object;

/// A call frame representing a function invocation.
#[derive(Debug)]
pub struct Frame {
    /// Instruction pointer - current position in bytecode.
    pub ip: usize,
    /// Base pointer - start of this frame's stack region.
    pub bp: usize,
    /// The code being executed.
    pub code: Rc<Code>,
    /// Local variables.
    pub locals: Vec<Object>,
    /// Free variables (closures).
    pub free_vars: Vec<Rc<RefCell<Object>>>,
}

impl Frame {
    /// Create a new frame.
    pub fn new(
        code: Rc<Code>,
        bp: usize,
        locals: Vec<Object>,
        free_vars: Vec<Rc<RefCell<Object>>>,
    ) -> Self {
        Self {
            ip: 0,
            bp,
            code,
            locals,
            free_vars,
        }
    }

    /// Create a frame from a closure.
    pub fn from_closure(
        code: Rc<Code>,
        free_vars: Vec<Rc<RefCell<Object>>>,
        bp: usize,
        args: Vec<Object>,
    ) -> Self {
        // Pre-allocate locals array with nil values
        let mut locals = vec![Object::Nil; code.local_count];

        // Copy arguments to locals
        for (i, arg) in args.into_iter().enumerate() {
            if i < locals.len() {
                locals[i] = arg;
            }
        }

        Self::new(code, bp, locals, free_vars)
    }

    /// Read the current instruction and advance IP.
    pub fn read_op(&mut self) -> u16 {
        let op = self.code.instructions[self.ip];
        self.ip += 1;
        op
    }

    /// Check if we've reached the end of the bytecode.
    pub fn is_at_end(&self) -> bool {
        self.ip >= self.code.instructions.len()
    }

    /// Jump forward by offset.
    pub fn jump_forward(&mut self, offset: usize) {
        self.ip += offset;
    }

    /// Jump backward by offset.
    pub fn jump_backward(&mut self, offset: usize) {
        self.ip -= offset;
    }

    /// Get local variable.
    pub fn get_local(&self, index: usize) -> &Object {
        &self.locals[index]
    }

    /// Set local variable.
    pub fn set_local(&mut self, index: usize, value: Object) {
        self.locals[index] = value;
    }

    /// Get free variable (from closure).
    pub fn get_free(&self, index: usize) -> Object {
        self.free_vars[index].borrow().clone()
    }

    /// Set free variable (in closure cell).
    pub fn set_free(&self, index: usize, value: Object) {
        *self.free_vars[index].borrow_mut() = value;
    }

    /// Get constant from code's constant pool.
    pub fn get_constant(&self, index: usize) -> &risor_bytecode::Constant {
        &self.code.constants[index]
    }

    /// Get name from code's name pool.
    pub fn get_name(&self, index: usize) -> &str {
        &self.code.names[index]
    }
}

/// Exception handler entry for try-catch-finally.
#[derive(Debug, Clone)]
pub struct ExceptionHandler {
    /// Start of try block (instruction index).
    pub start: usize,
    /// End of try block (instruction index).
    pub end: usize,
    /// Catch block offset (or usize::MAX if none).
    pub catch_offset: usize,
    /// Finally block offset (or usize::MAX if none).
    pub finally_offset: usize,
    /// Stack depth at entry.
    pub stack_depth: usize,
    /// Frame depth at entry.
    pub frame_depth: usize,
}
