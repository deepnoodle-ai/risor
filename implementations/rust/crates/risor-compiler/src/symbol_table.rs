//! Symbol table for tracking variable scopes during compilation.

use std::collections::HashMap;

/// Scope types for variables.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Scope {
    Local,
    Global,
    Free,
}

/// A symbol representing a variable in a scope.
#[derive(Debug, Clone)]
pub struct Symbol {
    /// Variable name.
    pub name: String,
    /// Index in local/global array.
    pub index: u16,
    /// Whether this is a constant (cannot be reassigned).
    pub is_constant: bool,
}

/// Resolution of a symbol reference.
#[derive(Debug, Clone)]
pub struct Resolution {
    /// The resolved symbol.
    pub symbol: Symbol,
    /// Scope where the symbol lives.
    pub scope: Scope,
    /// Depth: how many function levels up for free variables.
    pub depth: usize,
    /// Index in closure's free variable array (for free scope).
    pub free_index: usize,
}

/// Symbol table for a scope (function or block).
#[derive(Debug)]
pub struct SymbolTable {
    /// Unique identifier (e.g., "root.0.1").
    id: String,
    /// Whether this is a block scope (inherits parent's symbol indices).
    is_block: bool,
    /// Symbols defined in this scope, by name.
    symbols_by_name: HashMap<String, Symbol>,
    /// Free variables captured by this scope, by name.
    free_by_name: HashMap<String, Resolution>,
    /// All symbols in order of definition.
    symbols: Vec<Symbol>,
    /// All free variable resolutions in order.
    free_vars: Vec<Resolution>,
    /// Counter for generating child IDs.
    child_counter: usize,
}

impl SymbolTable {
    /// Create a new symbol table.
    pub fn new(id: String, is_block: bool) -> Self {
        Self {
            id,
            is_block,
            symbols_by_name: HashMap::new(),
            free_by_name: HashMap::new(),
            symbols: Vec::new(),
            free_vars: Vec::new(),
            child_counter: 0,
        }
    }

    /// Create the root symbol table.
    pub fn root() -> Self {
        Self::new("root".to_string(), false)
    }

    /// Get the ID of this symbol table.
    pub fn id(&self) -> &str {
        &self.id
    }

    /// Generate a child ID.
    pub fn next_child_id(&mut self) -> String {
        let id = format!("{}.{}", self.id, self.child_counter);
        self.child_counter += 1;
        id
    }

    /// Generate a block ID.
    pub fn next_block_id(&mut self) -> String {
        let id = format!("{}.block{}", self.id, self.child_counter);
        self.child_counter += 1;
        id
    }

    /// Insert a variable in this scope.
    pub fn insert_variable(&mut self, name: &str) -> Symbol {
        // Check if already defined in this scope
        if let Some(existing) = self.symbols_by_name.get(name) {
            return existing.clone();
        }

        let index = self.symbols.len() as u16;
        let symbol = Symbol {
            name: name.to_string(),
            index,
            is_constant: false,
        };

        self.symbols_by_name.insert(name.to_string(), symbol.clone());
        self.symbols.push(symbol.clone());
        symbol
    }

    /// Insert a constant in this scope.
    pub fn insert_constant(&mut self, name: &str) -> Symbol {
        let mut symbol = self.insert_variable(name);
        symbol.is_constant = true;
        if let Some(s) = self.symbols_by_name.get_mut(name) {
            s.is_constant = true;
        }
        if let Some(s) = self.symbols.iter_mut().find(|s| s.name == name) {
            s.is_constant = true;
        }
        symbol
    }

    /// Claim a slot for the blank identifier "_".
    pub fn claim_slot(&mut self) -> u16 {
        let index = self.symbols.len() as u16;
        let symbol = Symbol {
            name: "_".to_string(),
            index,
            is_constant: false,
        };
        self.symbols.push(symbol);
        index
    }

    /// Get the next local index.
    pub fn next_local_index(&self) -> u16 {
        self.symbols.len() as u16
    }

    /// Check if a symbol is defined in this scope only.
    pub fn is_defined(&self, name: &str) -> bool {
        self.symbols_by_name.contains_key(name)
    }

    /// Get a symbol by name in this scope only.
    pub fn get_local(&self, name: &str) -> Option<&Symbol> {
        self.symbols_by_name.get(name)
    }

    /// Get all symbols in this scope.
    pub fn symbols(&self) -> &[Symbol] {
        &self.symbols
    }

    /// Get all free variable resolutions.
    pub fn free_vars(&self) -> &[Resolution] {
        &self.free_vars
    }

    /// Get a free variable resolution by index.
    pub fn get_free_var(&self, index: usize) -> Option<&Resolution> {
        self.free_vars.get(index)
    }

    /// Get the number of local variables.
    pub fn local_count(&self) -> usize {
        self.symbols.len()
    }

    /// Get the number of free variables.
    pub fn free_count(&self) -> usize {
        self.free_vars.len()
    }

    /// Get local variable names in order.
    pub fn local_names(&self) -> Vec<String> {
        self.symbols.iter().map(|s| s.name.clone()).collect()
    }

    /// Add a free variable resolution.
    pub fn add_free_var(&mut self, name: &str, resolution: Resolution) -> usize {
        if let Some(existing) = self.free_by_name.get(name) {
            return existing.free_index;
        }
        let index = self.free_vars.len();
        let mut res = resolution;
        res.free_index = index;
        self.free_by_name.insert(name.to_string(), res.clone());
        self.free_vars.push(res);
        index
    }

    /// Get a free variable by name.
    pub fn get_free(&self, name: &str) -> Option<&Resolution> {
        self.free_by_name.get(name)
    }

    /// Check if this is a block scope.
    pub fn is_block(&self) -> bool {
        self.is_block
    }
}

/// Scope manager that handles the hierarchy of symbol tables.
#[derive(Debug)]
pub struct ScopeManager {
    /// Stack of symbol tables (innermost is last).
    scopes: Vec<SymbolTable>,
}

impl ScopeManager {
    /// Create a new scope manager with a root scope.
    pub fn new() -> Self {
        Self {
            scopes: vec![SymbolTable::root()],
        }
    }

    /// Get the current (innermost) scope.
    pub fn current(&self) -> &SymbolTable {
        self.scopes.last().unwrap()
    }

    /// Get the current scope mutably.
    pub fn current_mut(&mut self) -> &mut SymbolTable {
        self.scopes.last_mut().unwrap()
    }

    /// Check if we're in the root (global) scope.
    pub fn is_root(&self) -> bool {
        self.scopes.len() == 1
    }

    /// Enter a new function scope.
    pub fn enter_function(&mut self) -> &mut SymbolTable {
        let id = self.current_mut().next_child_id();
        let scope = SymbolTable::new(id, false);
        self.scopes.push(scope);
        self.scopes.last_mut().unwrap()
    }

    /// Enter a new block scope.
    pub fn enter_block(&mut self) -> &mut SymbolTable {
        let id = self.current_mut().next_block_id();
        let scope = SymbolTable::new(id, true);
        self.scopes.push(scope);
        self.scopes.last_mut().unwrap()
    }

    /// Exit the current scope and return it.
    pub fn exit_scope(&mut self) -> SymbolTable {
        if self.scopes.len() <= 1 {
            panic!("cannot exit root scope");
        }
        self.scopes.pop().unwrap()
    }

    /// Insert a variable in the current scope.
    pub fn insert_variable(&mut self, name: &str) -> Symbol {
        self.current_mut().insert_variable(name)
    }

    /// Insert a constant in the current scope.
    pub fn insert_constant(&mut self, name: &str) -> Symbol {
        self.current_mut().insert_constant(name)
    }

    /// Resolve a symbol by name, traversing up the scope chain.
    pub fn resolve(&mut self, name: &str) -> Option<Resolution> {
        self.resolve_with_depth(name, 0)
    }

    fn resolve_with_depth(&mut self, name: &str, depth: usize) -> Option<Resolution> {
        let scope_count = self.scopes.len();
        if scope_count == 0 {
            return None;
        }

        // Check current scope
        let current_idx = scope_count - 1;
        let is_root = current_idx == 0;
        let is_block = self.scopes[current_idx].is_block();

        // Check for local symbol
        if let Some(symbol) = self.scopes[current_idx].get_local(name) {
            let scope = if is_root { Scope::Global } else { Scope::Local };
            return Some(Resolution {
                symbol: symbol.clone(),
                scope,
                depth: 0,
                free_index: usize::MAX,
            });
        }

        // Check if already marked as free
        if let Some(free) = self.scopes[current_idx].get_free(name) {
            return Some(free.clone());
        }

        // Check parent scopes
        if scope_count > 1 {
            // Temporarily remove current scope to recurse
            let current = self.scopes.pop().unwrap();
            let new_depth = if current.is_block() { depth } else { depth + 1 };

            let parent_resolution = self.resolve_with_depth(name, new_depth);

            // Restore current scope
            self.scopes.push(current);

            if let Some(parent_res) = parent_resolution {
                // If in block, just pass through
                if is_block {
                    return Some(parent_res);
                }

                // If parent is global scope, keep as global
                if parent_res.scope == Scope::Global {
                    return Some(parent_res);
                }

                // Otherwise, mark as free variable in this scope
                let free_index = self.current_mut().add_free_var(
                    name,
                    Resolution {
                        symbol: parent_res.symbol.clone(),
                        scope: Scope::Free,
                        depth: parent_res.depth + 1,
                        free_index: 0, // Will be set by add_free_var
                    },
                );

                return Some(Resolution {
                    symbol: parent_res.symbol,
                    scope: Scope::Free,
                    depth: parent_res.depth + 1,
                    free_index,
                });
            }
        }

        None
    }
}

impl Default for ScopeManager {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_symbol_table() {
        let mut table = SymbolTable::root();

        let x = table.insert_variable("x");
        assert_eq!(x.name, "x");
        assert_eq!(x.index, 0);
        assert!(!x.is_constant);

        let y = table.insert_constant("y");
        assert_eq!(y.name, "y");
        assert_eq!(y.index, 1);
        assert!(table.get_local("y").unwrap().is_constant);

        assert!(table.is_defined("x"));
        assert!(table.is_defined("y"));
        assert!(!table.is_defined("z"));
    }

    #[test]
    fn test_scope_manager() {
        let mut mgr = ScopeManager::new();

        // Insert global
        mgr.insert_variable("global");
        assert!(mgr.is_root());

        // Enter function scope
        mgr.enter_function();
        assert!(!mgr.is_root());

        // Insert local
        mgr.insert_variable("local");

        // Resolve local
        let local_res = mgr.resolve("local");
        assert!(local_res.is_some());
        assert_eq!(local_res.as_ref().unwrap().scope, Scope::Local);

        // Resolve global from function
        let global_res = mgr.resolve("global");
        assert!(global_res.is_some());
        assert_eq!(global_res.as_ref().unwrap().scope, Scope::Global);

        // Exit function scope
        mgr.exit_scope();
        assert!(mgr.is_root());
    }

    #[test]
    fn test_free_variables() {
        let mut mgr = ScopeManager::new();

        // Enter outer function
        mgr.enter_function();
        mgr.insert_variable("x");

        // Enter inner function
        mgr.enter_function();

        // Resolve x from inner (should be free)
        let res = mgr.resolve("x");
        assert!(res.is_some());
        assert_eq!(res.as_ref().unwrap().scope, Scope::Free);

        // Check free vars
        assert_eq!(mgr.current().free_count(), 1);
    }
}
