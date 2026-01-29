//! Compiled bytecode container.

use std::rc::Rc;

/// Source location for an instruction.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct SourceLocation {
    pub line: usize,
    pub column: usize,
}

/// Exception handler entry.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ExceptionHandler {
    /// Start offset of the try block.
    pub start: usize,
    /// End offset of the try block.
    pub end: usize,
    /// Offset of the catch block (or usize::MAX if none).
    pub catch_offset: usize,
    /// Offset of the finally block (or usize::MAX if none).
    pub finally_offset: usize,
    /// Catch variable name (or None).
    pub catch_var: Option<String>,
}

/// Constant value in the constant pool.
#[derive(Debug, Clone)]
pub enum Constant {
    Nil,
    Bool(bool),
    Int(i64),
    Float(f64),
    String(Rc<str>),
    Function(Rc<Code>),
}

impl PartialEq for Constant {
    fn eq(&self, other: &Self) -> bool {
        match (self, other) {
            (Constant::Nil, Constant::Nil) => true,
            (Constant::Bool(a), Constant::Bool(b)) => a == b,
            (Constant::Int(a), Constant::Int(b)) => a == b,
            (Constant::Float(a), Constant::Float(b)) => a.to_bits() == b.to_bits(),
            (Constant::String(a), Constant::String(b)) => a == b,
            (Constant::Function(a), Constant::Function(b)) => Rc::ptr_eq(a, b),
            _ => false,
        }
    }
}

/// Immutable compiled bytecode.
#[derive(Debug, Clone)]
pub struct Code {
    /// Unique identifier.
    pub id: String,
    /// Function name (empty for module/anonymous).
    pub name: String,
    /// Whether this is a named function.
    pub is_named: bool,
    /// Bytecode instructions (opcodes + operands).
    pub instructions: Vec<u16>,
    /// Constant pool.
    pub constants: Vec<Constant>,
    /// Attribute name pool.
    pub names: Vec<String>,
    /// Number of local variables.
    pub local_count: usize,
    /// Number of global variables.
    pub global_count: usize,
    /// Local variable names.
    pub local_names: Vec<String>,
    /// Global variable names.
    pub global_names: Vec<String>,
    /// Child code blocks (nested functions).
    pub children: Vec<Rc<Code>>,
    /// Source code (for error messages).
    pub source: String,
    /// Source filename.
    pub filename: String,
    /// Source locations for each instruction.
    pub locations: Vec<SourceLocation>,
    /// Exception handlers.
    pub exception_handlers: Vec<ExceptionHandler>,
    /// Maximum call arguments (for stack sizing).
    pub max_call_args: usize,
}

impl Code {
    /// Get the source location for an instruction index.
    pub fn get_location(&self, instr_index: usize) -> Option<&SourceLocation> {
        self.locations.get(instr_index)
    }

    /// Get a child code block by index.
    pub fn get_child(&self, index: usize) -> Option<&Rc<Code>> {
        self.children.get(index)
    }
}

/// Mutable code builder used during compilation.
#[derive(Debug)]
pub struct CodeBuilder {
    pub id: String,
    pub name: String,
    pub is_named: bool,
    pub instructions: Vec<u16>,
    pub constants: Vec<Constant>,
    pub names: Vec<String>,
    pub name_map: std::collections::HashMap<String, usize>,
    pub children: Vec<CodeBuilder>,
    pub source: String,
    pub filename: String,
    pub locations: Vec<SourceLocation>,
    pub exception_handlers: Vec<ExceptionHandler>,
    pub max_call_args: usize,
    pub current_line: usize,
    pub current_column: usize,
}

impl CodeBuilder {
    /// Create a new code builder.
    pub fn new(
        id: String,
        name: String,
        is_named: bool,
        source: String,
        filename: String,
    ) -> Self {
        Self {
            id,
            name,
            is_named,
            instructions: Vec::new(),
            constants: Vec::new(),
            names: Vec::new(),
            name_map: std::collections::HashMap::new(),
            children: Vec::new(),
            source,
            filename,
            locations: Vec::new(),
            exception_handlers: Vec::new(),
            max_call_args: 0,
            current_line: 0,
            current_column: 0,
        }
    }

    /// Set current source position for subsequent instructions.
    pub fn set_position(&mut self, line: usize, column: usize) {
        self.current_line = line;
        self.current_column = column;
    }

    /// Emit a single instruction with no operands.
    pub fn emit(&mut self, opcode: u16) -> usize {
        let offset = self.instructions.len();
        self.instructions.push(opcode);
        self.locations.push(SourceLocation {
            line: self.current_line,
            column: self.current_column,
        });
        offset
    }

    /// Emit an instruction with one operand.
    pub fn emit1(&mut self, opcode: u16, operand: u16) -> usize {
        let offset = self.instructions.len();
        self.instructions.push(opcode);
        self.instructions.push(operand);
        let loc = SourceLocation {
            line: self.current_line,
            column: self.current_column,
        };
        self.locations.push(loc);
        self.locations.push(loc);
        offset
    }

    /// Emit an instruction with two operands.
    pub fn emit2(&mut self, opcode: u16, operand1: u16, operand2: u16) -> usize {
        let offset = self.instructions.len();
        self.instructions.push(opcode);
        self.instructions.push(operand1);
        self.instructions.push(operand2);
        let loc = SourceLocation {
            line: self.current_line,
            column: self.current_column,
        };
        self.locations.push(loc);
        self.locations.push(loc);
        self.locations.push(loc);
        offset
    }

    /// Patch an operand at a specific offset.
    pub fn patch(&mut self, offset: usize, value: u16) {
        self.instructions[offset] = value;
    }

    /// Get current instruction offset.
    pub fn offset(&self) -> usize {
        self.instructions.len()
    }

    /// Add a constant and return its index.
    pub fn add_constant(&mut self, value: Constant) -> usize {
        // Check for existing constant (for deduplication)
        for (i, c) in self.constants.iter().enumerate() {
            if c == &value {
                return i;
            }
        }
        let index = self.constants.len();
        self.constants.push(value);
        index
    }

    /// Add a name and return its index.
    pub fn add_name(&mut self, name: &str) -> usize {
        if let Some(&index) = self.name_map.get(name) {
            return index;
        }
        let index = self.names.len();
        self.names.push(name.to_string());
        self.name_map.insert(name.to_string(), index);
        index
    }

    /// Create a child code builder.
    pub fn create_child(&mut self, id: String, name: String, is_named: bool) -> usize {
        let child = CodeBuilder::new(
            id,
            name,
            is_named,
            self.source.clone(),
            self.filename.clone(),
        );
        let index = self.children.len();
        self.children.push(child);
        index
    }

    /// Get a mutable reference to a child.
    pub fn get_child_mut(&mut self, index: usize) -> Option<&mut CodeBuilder> {
        self.children.get_mut(index)
    }

    /// Add an exception handler.
    pub fn add_exception_handler(&mut self, handler: ExceptionHandler) {
        self.exception_handlers.push(handler);
    }

    /// Update max call args if needed.
    pub fn update_max_call_args(&mut self, args: usize) {
        if args > self.max_call_args {
            self.max_call_args = args;
        }
    }

    /// Convert to immutable Code.
    pub fn to_code(
        self,
        local_count: usize,
        global_count: usize,
        local_names: Vec<String>,
        global_names: Vec<String>,
    ) -> Rc<Code> {
        // First convert all children
        let children: Vec<Rc<Code>> = self
            .children
            .into_iter()
            .map(|child| child.to_code(0, 0, vec![], vec![]))
            .collect();

        Rc::new(Code {
            id: self.id,
            name: self.name,
            is_named: self.is_named,
            instructions: self.instructions,
            constants: self.constants,
            names: self.names,
            local_count,
            global_count,
            local_names,
            global_names,
            children,
            source: self.source,
            filename: self.filename,
            locations: self.locations,
            exception_handlers: self.exception_handlers,
            max_call_args: self.max_call_args,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_code_builder() {
        let mut builder = CodeBuilder::new(
            "main".to_string(),
            "".to_string(),
            false,
            "".to_string(),
            "test.rs".to_string(),
        );

        builder.set_position(1, 5);
        let offset = builder.emit(1);
        assert_eq!(offset, 0);

        let offset2 = builder.emit1(2, 42);
        assert_eq!(offset2, 1);
        assert_eq!(builder.offset(), 3);

        builder.patch(2, 100);
        assert_eq!(builder.instructions[2], 100);
    }

    #[test]
    fn test_constant_deduplication() {
        let mut builder = CodeBuilder::new(
            "main".to_string(),
            "".to_string(),
            false,
            "".to_string(),
            "test.rs".to_string(),
        );

        let idx1 = builder.add_constant(Constant::Int(42));
        let idx2 = builder.add_constant(Constant::Int(42));
        assert_eq!(idx1, idx2);
        assert_eq!(builder.constants.len(), 1);
    }

    #[test]
    fn test_name_deduplication() {
        let mut builder = CodeBuilder::new(
            "main".to_string(),
            "".to_string(),
            false,
            "".to_string(),
            "test.rs".to_string(),
        );

        let idx1 = builder.add_name("foo");
        let idx2 = builder.add_name("foo");
        assert_eq!(idx1, idx2);
        assert_eq!(builder.names.len(), 1);
    }
}
