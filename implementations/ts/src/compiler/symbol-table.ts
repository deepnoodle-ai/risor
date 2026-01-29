/**
 * Symbol table for tracking variable scopes during compilation.
 */

/**
 * Scope types for variables.
 */
export const enum Scope {
  Local = "local",
  Global = "global",
  Free = "free",
}

/**
 * A symbol representing a variable in a scope.
 */
export interface Symbol {
  /** Variable name. */
  name: string;
  /** Index in local/global array. */
  index: number;
  /** Whether this is a constant (cannot be reassigned). */
  isConstant: boolean;
  /** Optional value (for compile-time optimization). */
  value?: unknown;
}

/**
 * Resolution of a symbol reference.
 */
export interface Resolution {
  /** The resolved symbol. */
  symbol: Symbol;
  /** Scope where the symbol lives. */
  scope: Scope;
  /** Depth: how many function levels up for free variables. */
  depth: number;
  /** Index in closure's free variable array (for free scope). */
  freeIndex: number;
}

/**
 * Symbol table for a scope (function or block).
 */
export class SymbolTable {
  /** Unique identifier (e.g., "root.0.1"). */
  readonly id: string;

  /** Parent scope (null for global). */
  readonly parent: SymbolTable | null;

  /** Whether this is a block scope (inherits parent's symbol indices). */
  readonly isBlock: boolean;

  /** Symbols defined in this scope, by name. */
  private symbolsByName: Map<string, Symbol> = new Map();

  /** Free variables captured by this scope, by name. */
  private freeByName: Map<string, Resolution> = new Map();

  /** All symbols in order of definition. */
  private symbols: Symbol[] = [];

  /** All free variable resolutions in order. */
  private freeVars: Resolution[] = [];

  /** Child scopes. */
  private children: SymbolTable[] = [];

  /** Counter for generating child IDs. */
  private childCounter: number = 0;

  constructor(id: string, parent: SymbolTable | null, isBlock: boolean) {
    this.id = id;
    this.parent = parent;
    this.isBlock = isBlock;
  }

  /**
   * Create a new child function scope.
   */
  newChild(): SymbolTable {
    const childId = `${this.id}.${this.childCounter++}`;
    const child = new SymbolTable(childId, this, false);
    this.children.push(child);
    return child;
  }

  /**
   * Create a new block scope (if/else/loop).
   * Block scopes share the parent's symbol indices.
   */
  newBlock(): SymbolTable {
    const blockId = `${this.id}.block${this.childCounter++}`;
    const block = new SymbolTable(blockId, this, true);
    this.children.push(block);
    return block;
  }

  /**
   * Insert a variable in this scope.
   * Returns the symbol with its assigned index.
   */
  insertVariable(name: string): Symbol {
    // Check if already defined in this scope
    const existing = this.symbolsByName.get(name);
    if (existing) {
      return existing;
    }

    // For block scopes, use parent's next index
    const index = this.isBlock
      ? this.parent!.nextLocalIndex()
      : this.symbols.length;

    const symbol: Symbol = {
      name,
      index,
      isConstant: false,
    };

    this.symbolsByName.set(name, symbol);
    this.symbols.push(symbol);

    // Also add to parent if block scope
    if (this.isBlock) {
      this.parent!.symbolsByName.set(name, symbol);
      this.parent!.symbols.push(symbol);
    }

    return symbol;
  }

  /**
   * Insert a constant in this scope.
   */
  insertConstant(name: string, value?: unknown): Symbol {
    const symbol = this.insertVariable(name);
    symbol.isConstant = true;
    symbol.value = value;
    return symbol;
  }

  /**
   * Claim a slot for the blank identifier "_".
   */
  claimSlot(): number {
    const index = this.isBlock
      ? this.parent!.nextLocalIndex()
      : this.symbols.length;

    // Create a placeholder symbol
    const symbol: Symbol = {
      name: "_",
      index,
      isConstant: false,
    };
    this.symbols.push(symbol);

    if (this.isBlock) {
      this.parent!.symbols.push(symbol);
    }

    return index;
  }

  /**
   * Get the next local index.
   */
  nextLocalIndex(): number {
    if (this.isBlock) {
      return this.parent!.nextLocalIndex();
    }
    return this.symbols.length;
  }

  /**
   * Resolve a symbol by name.
   * Returns the resolution including scope and depth.
   */
  resolve(name: string): Resolution | null {
    return this.resolveWithDepth(name, 0);
  }

  private resolveWithDepth(name: string, depth: number): Resolution | null {
    // Check this scope
    const symbol = this.symbolsByName.get(name);
    if (symbol) {
      // Local in current scope
      const scope = this.isRootScope() ? Scope.Global : Scope.Local;
      return {
        symbol,
        scope,
        depth: 0,
        freeIndex: -1,
      };
    }

    // Check if already marked as free
    const free = this.freeByName.get(name);
    if (free) {
      return free;
    }

    // Check parent scope
    if (this.parent) {
      const parentResolution = this.parent.resolveWithDepth(
        name,
        this.isBlock ? depth : depth + 1
      );

      if (parentResolution) {
        // If we're in a block, just pass through
        if (this.isBlock) {
          return parentResolution;
        }

        // If parent is global scope, keep as global
        if (parentResolution.scope === Scope.Global) {
          return parentResolution;
        }

        // Otherwise, mark as free variable in this scope
        const freeIndex = this.freeVars.length;
        const freeResolution: Resolution = {
          symbol: parentResolution.symbol,
          scope: Scope.Free,
          depth: parentResolution.depth + 1,
          freeIndex,
        };

        this.freeByName.set(name, freeResolution);
        this.freeVars.push(freeResolution);

        return freeResolution;
      }
    }

    return null;
  }

  /**
   * Check if a symbol is defined in this scope.
   */
  isDefined(name: string): boolean {
    return this.symbolsByName.has(name);
  }

  /**
   * Check if this is the root (global) scope.
   */
  isRootScope(): boolean {
    return this.parent === null;
  }

  /**
   * Get a symbol by name in this scope only.
   */
  getLocal(name: string): Symbol | null {
    return this.symbolsByName.get(name) ?? null;
  }

  /**
   * Get all symbols in this scope.
   */
  getSymbols(): Symbol[] {
    return [...this.symbols];
  }

  /**
   * Get all free variable resolutions.
   */
  getFreeVars(): Resolution[] {
    return [...this.freeVars];
  }

  /**
   * Get a free variable resolution by index.
   */
  getFreeVar(index: number): Resolution | null {
    return this.freeVars[index] ?? null;
  }

  /**
   * Get the number of local variables.
   */
  localCount(): number {
    if (this.isBlock) {
      return this.parent!.localCount();
    }
    return this.symbols.length;
  }

  /**
   * Get the number of free variables.
   */
  freeCount(): number {
    return this.freeVars.length;
  }

  /**
   * Get local variable names in order.
   */
  localNames(): string[] {
    return this.symbols.map((s) => s.name);
  }
}

/**
 * Create a root symbol table.
 */
export function createRootSymbolTable(): SymbolTable {
  return new SymbolTable("root", null, false);
}
