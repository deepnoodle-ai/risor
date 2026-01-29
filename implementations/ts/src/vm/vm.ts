/**
 * Risor Virtual Machine - bytecode execution engine.
 */

import { Code, Constant, ConstantType } from "../bytecode/code.js";
import { Op, BinaryOpType, CompareOpType } from "../bytecode/opcode.js";
import { Frame, ExceptionHandler } from "./frame.js";
import {
  RisorObject,
  ObjectType,
  NIL,
  TRUE,
  FALSE,
  toBool,
  RisorInt,
  RisorFloat,
  RisorString,
  RisorList,
  RisorMap,
  RisorClosure,
  RisorBuiltin,
  RisorCell,
  RisorError,
  RisorIter,
} from "../object/object.js";

/** Maximum call stack depth. */
const MAX_STACK_DEPTH = 1024;
/** Maximum call frame depth. */
const MAX_FRAME_DEPTH = 256;

/**
 * VM execution error.
 */
export class VMError extends Error {
  constructor(
    message: string,
    public readonly line?: number,
    public readonly column?: number
  ) {
    super(message);
    this.name = "VMError";
  }
}

/**
 * VM configuration options.
 */
export interface VMConfig {
  /** Global variables (builtins, etc.). */
  globals?: Map<string, RisorObject>;
}

/**
 * Risor Virtual Machine.
 */
export class VM {
  /** Value stack. */
  private stack: RisorObject[] = [];
  /** Stack pointer (index of next free slot). */
  private sp: number = 0;
  /** Call frames. */
  private frames: Frame[] = [];
  /** Current frame. */
  private frame!: Frame;
  /** Global variables. */
  private globals: RisorObject[];
  /** Global names for lookup. */
  private globalNames: Map<string, number>;
  /** Exception handlers. */
  private exceptionHandlers: ExceptionHandler[] = [];
  /** Whether we're in a finally block. */
  private inFinally: boolean = false;
  /** Pending exception for rethrow. */
  private pendingException: RisorObject | null = null;

  constructor(config: VMConfig = {}) {
    this.globals = [];
    this.globalNames = new Map();

    // Initialize globals from config
    if (config.globals) {
      for (const [name, value] of config.globals) {
        const index = this.globals.length;
        this.globals.push(value);
        this.globalNames.set(name, index);
      }
    }
  }

  /**
   * Execute compiled bytecode.
   */
  run(code: Code): RisorObject {
    // Reset state
    this.stack = new Array(MAX_STACK_DEPTH);
    this.sp = 0;
    this.frames = [];
    this.exceptionHandlers = [];

    // Register globals from code
    for (const name of code.globalNames) {
      if (!this.globalNames.has(name)) {
        const index = this.globals.length;
        this.globals.push(NIL);
        this.globalNames.set(name, index);
      }
    }

    // Extend globals array if needed
    while (this.globals.length < code.globalCount) {
      this.globals.push(NIL);
    }

    // Create initial frame with pre-allocated locals
    const locals: RisorObject[] = new Array(code.localCount).fill(NIL);
    this.frame = new Frame(code, 0, locals);
    this.frames.push(this.frame);

    // Execute
    return this.execute();
  }

  /**
   * Main execution loop.
   * @param minFrameDepth - stop execution when frame depth drops below this
   */
  private execute(minFrameDepth: number = 0): RisorObject {
    while (!this.frame.isAtEnd()) {
      // Check if we've returned from all frames at our level
      if (this.frames.length < minFrameDepth) {
        return this.sp > 0 ? this.stack[this.sp - 1] : NIL;
      }
      const op = this.frame.readOp() as Op;

      try {
        switch (op) {
          // Execution Control
          case Op.Nop:
            break;

          case Op.Halt:
            return this.sp > 0 ? this.pop() : NIL;

          case Op.Call:
            this.opCall(this.frame.readOp());
            break;

          case Op.CallSpread:
            this.opCallSpread(this.frame.readOp());
            break;

          case Op.ReturnValue:
            if (this.opReturnValue()) {
              return this.sp > 0 ? this.pop() : NIL;
            }
            break;

          // Jumps
          case Op.JumpForward:
            this.frame.jumpForward(this.frame.readOp());
            break;

          case Op.JumpBackward:
            this.frame.jumpBackward(this.frame.readOp());
            break;

          case Op.PopJumpForwardIfFalse: {
            const offset = this.frame.readOp();
            if (!this.pop().isTruthy()) {
              this.frame.jumpForward(offset);
            }
            break;
          }

          case Op.PopJumpForwardIfTrue: {
            const offset = this.frame.readOp();
            if (this.pop().isTruthy()) {
              this.frame.jumpForward(offset);
            }
            break;
          }

          case Op.PopJumpForwardIfNil: {
            const offset = this.frame.readOp();
            if (this.pop().type === ObjectType.Nil) {
              this.frame.jumpForward(offset);
            }
            break;
          }

          case Op.PopJumpForwardIfNotNil: {
            const offset = this.frame.readOp();
            const value = this.pop();
            if (value.type !== ObjectType.Nil) {
              this.push(value);
              this.frame.jumpForward(offset);
            }
            break;
          }

          // Load Operations
          case Op.LoadConst:
            this.opLoadConst(this.frame.readOp());
            break;

          case Op.LoadFast:
            this.push(this.frame.getLocal(this.frame.readOp()));
            break;

          case Op.LoadGlobal:
            this.push(this.globals[this.frame.readOp()]);
            break;

          case Op.LoadFree:
            this.push(this.frame.getFree(this.frame.readOp()));
            break;

          case Op.LoadAttr:
            this.opLoadAttr(this.frame.readOp());
            break;

          case Op.LoadAttrOrNil:
            this.opLoadAttrOrNil(this.frame.readOp());
            break;

          // Store Operations
          case Op.StoreFast:
            this.frame.setLocal(this.frame.readOp(), this.pop());
            break;

          case Op.StoreGlobal:
            this.globals[this.frame.readOp()] = this.pop();
            break;

          case Op.StoreFree:
            this.frame.setFree(this.frame.readOp(), this.pop());
            break;

          case Op.StoreAttr:
            this.opStoreAttr(this.frame.readOp());
            break;

          // Binary/Unary Operations
          case Op.BinaryOp:
            this.opBinaryOp(this.frame.readOp() as BinaryOpType);
            break;

          case Op.CompareOp:
            this.opCompareOp(this.frame.readOp() as CompareOpType);
            break;

          case Op.UnaryNegative:
            this.opUnaryNegative();
            break;

          case Op.UnaryNot:
            this.push(toBool(!this.pop().isTruthy()));
            break;

          // Container Building
          case Op.BuildList:
            this.opBuildList(this.frame.readOp());
            break;

          case Op.BuildMap:
            this.opBuildMap(this.frame.readOp());
            break;

          case Op.BuildString:
            this.opBuildString(this.frame.readOp());
            break;

          case Op.ListAppend:
            this.opListAppend();
            break;

          case Op.ListExtend:
            this.opListExtend();
            break;

          case Op.MapMerge:
            this.opMapMerge();
            break;

          case Op.MapSet:
            this.opMapSet();
            break;

          // Container Access
          case Op.BinarySubscr:
            this.frame.readOp(); // Skip operand (unused)
            this.opBinarySubscr();
            break;

          case Op.StoreSubscr:
            this.frame.readOp(); // Skip operand (unused)
            this.opStoreSubscr();
            break;

          case Op.ContainsOp:
            this.frame.readOp(); // Skip operand (unused)
            this.opContainsOp();
            break;

          case Op.Length:
            this.opLength();
            break;

          case Op.Slice:
            this.frame.readOp(); // Skip operands
            this.frame.readOp();
            this.opSlice();
            break;

          case Op.Unpack:
            this.opUnpack(this.frame.readOp());
            break;

          // Stack Manipulation
          case Op.Swap: {
            const depth = this.frame.readOp();
            const top = this.stack[this.sp - 1];
            this.stack[this.sp - 1] = this.stack[this.sp - 1 - depth];
            this.stack[this.sp - 1 - depth] = top;
            break;
          }

          case Op.Copy: {
            const depth = this.frame.readOp();
            this.push(this.stack[this.sp - 1 - depth]);
            break;
          }

          case Op.PopTop:
            this.pop();
            break;

          // Constants
          case Op.Nil:
            this.push(NIL);
            break;

          case Op.False:
            this.push(FALSE);
            break;

          case Op.True:
            this.push(TRUE);
            break;

          // Closures
          case Op.LoadClosure:
            this.opLoadClosure(this.frame.readOp(), this.frame.readOp());
            break;

          case Op.MakeCell:
            this.opMakeCell(this.frame.readOp(), this.frame.readOp());
            break;

          // Partial Application
          case Op.Partial:
            this.frame.readOp(); // Skip operand
            // For now, just leave the value on stack
            break;

          // Exception Handling
          case Op.PushExcept:
            this.opPushExcept(this.frame.readOp(), this.frame.readOp());
            break;

          case Op.PopExcept:
            this.exceptionHandlers.pop();
            break;

          case Op.Throw:
            this.opThrow();
            break;

          case Op.EndFinally:
            this.opEndFinally();
            break;

          default:
            throw new VMError(`unknown opcode: ${op}`);
        }
      } catch (e) {
        if (e instanceof VMError) {
          // Try to handle with exception handler
          if (!this.handleException(new RisorError(e.message))) {
            throw e;
          }
        } else {
          throw e;
        }
      }
    }

    return this.sp > 0 ? this.pop() : NIL;
  }

  // ===========================================================================
  // Stack Operations
  // ===========================================================================

  private push(value: RisorObject): void {
    if (this.sp >= MAX_STACK_DEPTH) {
      throw new VMError("stack overflow");
    }
    this.stack[this.sp++] = value;
  }

  private pop(): RisorObject {
    if (this.sp <= 0) {
      throw new VMError("stack underflow");
    }
    return this.stack[--this.sp];
  }

  private peek(depth: number = 0): RisorObject {
    return this.stack[this.sp - 1 - depth];
  }

  // ===========================================================================
  // Opcode Implementations
  // ===========================================================================

  private opLoadConst(index: number): void {
    const constant = this.frame.getConstant(index) as Constant;
    switch (constant.type) {
      case ConstantType.Nil:
        this.push(NIL);
        break;
      case ConstantType.Bool:
        this.push(toBool(constant.value as boolean));
        break;
      case ConstantType.Int:
        this.push(new RisorInt(constant.value as number));
        break;
      case ConstantType.Float:
        this.push(new RisorFloat(constant.value as number));
        break;
      case ConstantType.String:
        this.push(new RisorString(constant.value as string));
        break;
      case ConstantType.Function:
        this.push(new RisorClosure(constant.value as Code, []));
        break;
    }
  }

  private opLoadAttr(nameIndex: number): void {
    const obj = this.pop();
    const name = this.frame.getName(nameIndex);

    // Check for method first
    const method = this.getMethod(obj, name);
    if (method) {
      this.push(method);
      return;
    }

    // Then check for property
    if (obj.type === ObjectType.Map) {
      const value = (obj as RisorMap).get(new RisorString(name));
      if (value !== undefined) {
        this.push(value);
        return;
      }
    }

    throw new VMError(`attribute '${name}' not found on ${obj.type}`);
  }

  private opLoadAttrOrNil(nameIndex: number): void {
    const obj = this.pop();
    const name = this.frame.getName(nameIndex);

    // Check for method first
    const method = this.getMethod(obj, name);
    if (method) {
      this.push(method);
      return;
    }

    // Then check for property
    if (obj.type === ObjectType.Map) {
      const value = (obj as RisorMap).get(new RisorString(name));
      this.push(value ?? NIL);
      return;
    }

    this.push(NIL);
  }

  private opStoreAttr(nameIndex: number): void {
    const value = this.pop();
    const obj = this.pop();
    const name = this.frame.getName(nameIndex);

    if (obj.type === ObjectType.Map) {
      (obj as RisorMap).set(new RisorString(name), value);
      return;
    }

    throw new VMError(`cannot set attribute '${name}' on ${obj.type}`);
  }

  private opBinaryOp(opType: BinaryOpType): void {
    const right = this.pop();
    const left = this.pop();

    switch (opType) {
      case BinaryOpType.Add:
        this.push(this.add(left, right));
        break;
      case BinaryOpType.Subtract:
        this.push(this.subtract(left, right));
        break;
      case BinaryOpType.Multiply:
        this.push(this.multiply(left, right));
        break;
      case BinaryOpType.Divide:
        this.push(this.divide(left, right));
        break;
      case BinaryOpType.Modulo:
        this.push(this.modulo(left, right));
        break;
      case BinaryOpType.Power:
        this.push(this.power(left, right));
        break;
      case BinaryOpType.BitwiseAnd:
        this.push(this.bitwiseAnd(left, right));
        break;
      case BinaryOpType.BitwiseOr:
        this.push(this.bitwiseOr(left, right));
        break;
      case BinaryOpType.Xor:
        this.push(this.bitwiseXor(left, right));
        break;
      case BinaryOpType.LShift:
        this.push(this.leftShift(left, right));
        break;
      case BinaryOpType.RShift:
        this.push(this.rightShift(left, right));
        break;
      default:
        throw new VMError(`unknown binary operation: ${opType}`);
    }
  }

  private opCompareOp(opType: CompareOpType): void {
    const right = this.pop();
    const left = this.pop();

    switch (opType) {
      case CompareOpType.Eq:
        this.push(toBool(left.equals(right)));
        break;
      case CompareOpType.NotEq:
        this.push(toBool(!left.equals(right)));
        break;
      case CompareOpType.Lt:
        this.push(toBool(this.compare(left, right) < 0));
        break;
      case CompareOpType.LtEquals:
        this.push(toBool(this.compare(left, right) <= 0));
        break;
      case CompareOpType.Gt:
        this.push(toBool(this.compare(left, right) > 0));
        break;
      case CompareOpType.GtEquals:
        this.push(toBool(this.compare(left, right) >= 0));
        break;
    }
  }

  private opUnaryNegative(): void {
    const value = this.pop();
    if (value.type === ObjectType.Int) {
      this.push(new RisorInt(-(value as RisorInt).value));
    } else if (value.type === ObjectType.Float) {
      this.push(new RisorFloat(-(value as RisorFloat).value));
    } else {
      throw new VMError(`cannot negate ${value.type}`);
    }
  }

  private opBuildList(count: number): void {
    const items: RisorObject[] = new Array(count);
    for (let i = count - 1; i >= 0; i--) {
      items[i] = this.pop();
    }
    this.push(new RisorList(items));
  }

  private opBuildMap(count: number): void {
    const entries: [RisorObject, RisorObject][] = [];
    for (let i = 0; i < count; i++) {
      const value = this.pop();
      const key = this.pop();
      entries.unshift([key, value]);
    }
    this.push(new RisorMap(entries));
  }

  private opBuildString(count: number): void {
    const parts: string[] = new Array(count);
    for (let i = count - 1; i >= 0; i--) {
      const obj = this.pop();
      parts[i] = obj.type === ObjectType.String ? (obj as RisorString).value : obj.inspect();
    }
    this.push(new RisorString(parts.join("")));
  }

  private opListAppend(): void {
    const item = this.pop();
    const list = this.peek() as RisorList;
    list.append(item);
  }

  private opListExtend(): void {
    const items = this.pop() as RisorList;
    const list = this.peek() as RisorList;
    list.extend(items);
  }

  private opMapMerge(): void {
    const source = this.pop() as RisorMap;
    const target = this.peek() as RisorMap;
    target.merge(source);
  }

  private opMapSet(): void {
    const value = this.pop();
    const key = this.pop();
    const map = this.peek() as RisorMap;
    map.set(key, value);
  }

  private opBinarySubscr(): void {
    const index = this.pop();
    const obj = this.pop();

    if (obj.type === ObjectType.List) {
      if (index.type !== ObjectType.Int) {
        throw new VMError(`list index must be int, got ${index.type}`);
      }
      this.push((obj as RisorList).get((index as RisorInt).value));
    } else if (obj.type === ObjectType.Map) {
      const value = (obj as RisorMap).get(index);
      this.push(value ?? NIL);
    } else if (obj.type === ObjectType.String) {
      if (index.type !== ObjectType.Int) {
        throw new VMError(`string index must be int, got ${index.type}`);
      }
      this.push((obj as RisorString).charAt((index as RisorInt).value));
    } else {
      throw new VMError(`cannot index ${obj.type}`);
    }
  }

  private opStoreSubscr(): void {
    const value = this.pop();
    const index = this.pop();
    const obj = this.pop();

    if (obj.type === ObjectType.List) {
      if (index.type !== ObjectType.Int) {
        throw new VMError(`list index must be int, got ${index.type}`);
      }
      (obj as RisorList).set((index as RisorInt).value, value);
    } else if (obj.type === ObjectType.Map) {
      (obj as RisorMap).set(index, value);
    } else {
      throw new VMError(`cannot index-assign to ${obj.type}`);
    }
  }

  private opContainsOp(): void {
    const item = this.pop();
    const container = this.pop();

    if (container.type === ObjectType.List) {
      const list = container as RisorList;
      const found = list.items.some((i) => i.equals(item));
      this.push(toBool(found));
    } else if (container.type === ObjectType.Map) {
      this.push(toBool((container as RisorMap).has(item)));
    } else if (container.type === ObjectType.String) {
      if (item.type !== ObjectType.String) {
        throw new VMError(`'in' requires string, got ${item.type}`);
      }
      const found = (container as RisorString).value.includes((item as RisorString).value);
      this.push(toBool(found));
    } else {
      throw new VMError(`cannot check membership in ${container.type}`);
    }
  }

  private opLength(): void {
    const obj = this.pop();
    if (obj.type === ObjectType.List) {
      this.push(new RisorInt((obj as RisorList).items.length));
    } else if (obj.type === ObjectType.Map) {
      this.push(new RisorInt((obj as RisorMap).size));
    } else if (obj.type === ObjectType.String) {
      this.push(new RisorInt((obj as RisorString).value.length));
    } else {
      throw new VMError(`cannot get length of ${obj.type}`);
    }
  }

  private opSlice(): void {
    const high = this.pop();
    const low = this.pop();
    const obj = this.pop();

    const start = low.type === ObjectType.Nil ? undefined : (low as RisorInt).value;
    const end = high.type === ObjectType.Nil ? undefined : (high as RisorInt).value;

    if (obj.type === ObjectType.List) {
      this.push((obj as RisorList).slice(start, end));
    } else if (obj.type === ObjectType.String) {
      this.push((obj as RisorString).slice(start, end));
    } else {
      throw new VMError(`cannot slice ${obj.type}`);
    }
  }

  private opUnpack(count: number): void {
    const obj = this.pop();
    if (obj.type !== ObjectType.List) {
      throw new VMError(`cannot unpack ${obj.type}`);
    }
    const list = obj as RisorList;
    if (list.items.length < count) {
      throw new VMError(`not enough values to unpack: expected ${count}, got ${list.items.length}`);
    }
    // Push values in reverse order so they can be popped in order
    for (let i = count - 1; i >= 0; i--) {
      this.push(list.items[i]);
    }
  }

  private opCall(argCount: number): void {
    const callable = this.stack[this.sp - 1 - argCount];

    if (callable.type === ObjectType.Closure) {
      const closure = callable as RisorClosure;
      const args: RisorObject[] = [];
      for (let i = 0; i < argCount; i++) {
        args.unshift(this.pop());
      }
      this.pop(); // Pop the closure

      if (this.frames.length >= MAX_FRAME_DEPTH) {
        throw new VMError("call stack overflow");
      }

      const frame = Frame.fromClosure(closure, this.sp, args);
      this.frames.push(frame);
      this.frame = frame;
    } else if (callable.type === ObjectType.Builtin) {
      const builtin = callable as RisorBuiltin;
      const args: RisorObject[] = [];
      for (let i = 0; i < argCount; i++) {
        args.unshift(this.pop());
      }
      this.pop(); // Pop the builtin

      const result = builtin.fn(args);
      this.push(result);
    } else {
      throw new VMError(`cannot call ${callable.type}`);
    }
  }

  private opCallSpread(argCount: number): void {
    // Collect args, expanding any lists at the end
    const args: RisorObject[] = [];
    for (let i = 0; i < argCount; i++) {
      const arg = this.pop();
      if (arg.type === ObjectType.List) {
        // Spread the list
        args.unshift(...(arg as RisorList).items);
      } else {
        args.unshift(arg);
      }
    }

    const callable = this.pop();

    if (callable.type === ObjectType.Closure) {
      const closure = callable as RisorClosure;

      if (this.frames.length >= MAX_FRAME_DEPTH) {
        throw new VMError("call stack overflow");
      }

      const frame = Frame.fromClosure(closure, this.sp, args);
      this.frames.push(frame);
      this.frame = frame;
    } else if (callable.type === ObjectType.Builtin) {
      const builtin = callable as RisorBuiltin;
      const result = builtin.fn(args);
      this.push(result);
    } else {
      throw new VMError(`cannot call ${callable.type}`);
    }
  }

  private opReturnValue(): boolean {
    const result = this.pop();
    this.frames.pop();

    if (this.frames.length === 0) {
      this.push(result);
      return true; // Signal to halt
    }

    this.frame = this.frames[this.frames.length - 1];
    this.push(result);
    return false;
  }

  private opLoadClosure(constIndex: number, freeCount: number): void {
    const constant = this.frame.getConstant(constIndex) as Constant;
    if (constant.type !== ConstantType.Function) {
      throw new VMError("expected function constant for closure");
    }

    const code = constant.value as Code;
    const freeVars: RisorCell[] = [];

    // Pop free var cells from stack
    for (let i = 0; i < freeCount; i++) {
      const cell = this.pop();
      if (cell.type !== ObjectType.Cell) {
        throw new VMError("expected cell for closure free variable");
      }
      freeVars.unshift(cell as RisorCell);
    }

    this.push(new RisorClosure(code, freeVars));
  }

  private opMakeCell(localIndex: number, depth: number): void {
    // Create or reference a cell for a captured variable
    if (depth === 0) {
      // Local variable in current frame
      const value = this.frame.getLocal(localIndex);
      const cell = new RisorCell(value);
      // Replace local with cell? For now just push cell
      this.push(cell);
    } else {
      // Free variable from enclosing scope
      const cell = this.frame.freeVars[localIndex];
      this.push(cell);
    }
  }

  private opPushExcept(catchOffset: number, finallyOffset: number): void {
    this.exceptionHandlers.push({
      start: this.frame.ip,
      end: 0, // Will be set when handler is used
      catchOffset: catchOffset === 0xffff ? -1 : this.frame.ip + catchOffset,
      finallyOffset: finallyOffset === 0xffff ? -1 : this.frame.ip + finallyOffset,
      stackDepth: this.sp,
      frameDepth: this.frames.length,
    });
  }

  private opThrow(): void {
    const value = this.pop();
    const error = value.type === ObjectType.Error ? value : new RisorError(value.inspect());

    if (!this.handleException(error)) {
      throw new VMError((error as RisorError).message);
    }
  }

  private opEndFinally(): void {
    this.inFinally = false;
    if (this.pendingException) {
      const exc = this.pendingException;
      this.pendingException = null;
      if (!this.handleException(exc)) {
        throw new VMError((exc as RisorError).message);
      }
    }
  }

  private handleException(error: RisorObject): boolean {
    while (this.exceptionHandlers.length > 0) {
      const handler = this.exceptionHandlers[this.exceptionHandlers.length - 1];

      // Unwind frames if needed
      while (this.frames.length > handler.frameDepth) {
        this.frames.pop();
        if (this.frames.length > 0) {
          this.frame = this.frames[this.frames.length - 1];
        }
      }

      // Restore stack depth
      this.sp = handler.stackDepth;

      if (handler.catchOffset >= 0) {
        // Jump to catch block
        this.frame.ip = handler.catchOffset;
        this.push(error);
        this.exceptionHandlers.pop();
        return true;
      } else if (handler.finallyOffset >= 0) {
        // Jump to finally block, save exception for rethrow
        this.frame.ip = handler.finallyOffset;
        this.pendingException = error;
        this.inFinally = true;
        this.exceptionHandlers.pop();
        return true;
      }

      this.exceptionHandlers.pop();
    }

    return false;
  }

  // ===========================================================================
  // Arithmetic Helpers
  // ===========================================================================

  private add(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value + (right as RisorInt).value);
    }
    if (left.type === ObjectType.Float || right.type === ObjectType.Float) {
      const l = this.toNumber(left);
      const r = this.toNumber(right);
      return new RisorFloat(l + r);
    }
    if (left.type === ObjectType.String && right.type === ObjectType.String) {
      return new RisorString((left as RisorString).value + (right as RisorString).value);
    }
    if (left.type === ObjectType.List && right.type === ObjectType.List) {
      return new RisorList([...(left as RisorList).items, ...(right as RisorList).items]);
    }
    throw new VMError(`cannot add ${left.type} and ${right.type}`);
  }

  private subtract(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value - (right as RisorInt).value);
    }
    if (left.type === ObjectType.Float || right.type === ObjectType.Float) {
      return new RisorFloat(this.toNumber(left) - this.toNumber(right));
    }
    throw new VMError(`cannot subtract ${right.type} from ${left.type}`);
  }

  private multiply(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value * (right as RisorInt).value);
    }
    if (left.type === ObjectType.Float || right.type === ObjectType.Float) {
      return new RisorFloat(this.toNumber(left) * this.toNumber(right));
    }
    if (left.type === ObjectType.String && right.type === ObjectType.Int) {
      return new RisorString((left as RisorString).value.repeat((right as RisorInt).value));
    }
    throw new VMError(`cannot multiply ${left.type} and ${right.type}`);
  }

  private divide(left: RisorObject, right: RisorObject): RisorObject {
    const r = this.toNumber(right);
    if (r === 0) {
      throw new VMError("division by zero");
    }
    return new RisorFloat(this.toNumber(left) / r);
  }

  private modulo(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      const r = (right as RisorInt).value;
      if (r === 0) {
        throw new VMError("modulo by zero");
      }
      return new RisorInt((left as RisorInt).value % r);
    }
    throw new VMError(`cannot modulo ${left.type} and ${right.type}`);
  }

  private power(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt(Math.pow((left as RisorInt).value, (right as RisorInt).value));
    }
    return new RisorFloat(Math.pow(this.toNumber(left), this.toNumber(right)));
  }

  private bitwiseAnd(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value & (right as RisorInt).value);
    }
    throw new VMError(`cannot bitwise AND ${left.type} and ${right.type}`);
  }

  private bitwiseOr(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value | (right as RisorInt).value);
    }
    throw new VMError(`cannot bitwise OR ${left.type} and ${right.type}`);
  }

  private bitwiseXor(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value ^ (right as RisorInt).value);
    }
    throw new VMError(`cannot bitwise XOR ${left.type} and ${right.type}`);
  }

  private leftShift(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value << (right as RisorInt).value);
    }
    throw new VMError(`cannot left shift ${left.type} by ${right.type}`);
  }

  private rightShift(left: RisorObject, right: RisorObject): RisorObject {
    if (left.type === ObjectType.Int && right.type === ObjectType.Int) {
      return new RisorInt((left as RisorInt).value >> (right as RisorInt).value);
    }
    throw new VMError(`cannot right shift ${left.type} by ${right.type}`);
  }

  private compare(left: RisorObject, right: RisorObject): number {
    if (
      (left.type === ObjectType.Int || left.type === ObjectType.Float) &&
      (right.type === ObjectType.Int || right.type === ObjectType.Float)
    ) {
      return this.toNumber(left) - this.toNumber(right);
    }
    if (left.type === ObjectType.String && right.type === ObjectType.String) {
      return (left as RisorString).value.localeCompare((right as RisorString).value);
    }
    throw new VMError(`cannot compare ${left.type} and ${right.type}`);
  }

  private toNumber(obj: RisorObject): number {
    if (obj.type === ObjectType.Int) return (obj as RisorInt).value;
    if (obj.type === ObjectType.Float) return (obj as RisorFloat).value;
    throw new VMError(`expected number, got ${obj.type}`);
  }

  // ===========================================================================
  // Method Lookup
  // ===========================================================================

  private getMethod(obj: RisorObject, name: string): RisorObject | null {
    switch (obj.type) {
      case ObjectType.String:
        return this.getStringMethod(obj as RisorString, name);
      case ObjectType.List:
        return this.getListMethod(obj as RisorList, name);
      case ObjectType.Map:
        return this.getMapMethod(obj as RisorMap, name);
      default:
        return null;
    }
  }

  private getStringMethod(str: RisorString, name: string): RisorObject | null {
    switch (name) {
      case "len":
        return new RisorBuiltin("len", () => new RisorInt(str.value.length));
      case "upper":
        return new RisorBuiltin("upper", () => new RisorString(str.value.toUpperCase()));
      case "lower":
        return new RisorBuiltin("lower", () => new RisorString(str.value.toLowerCase()));
      case "trim":
        return new RisorBuiltin("trim", () => new RisorString(str.value.trim()));
      case "split":
        return new RisorBuiltin("split", (args) => {
          const sep = args.length > 0 ? (args[0] as RisorString).value : "";
          return new RisorList(str.value.split(sep).map((s) => new RisorString(s)));
        });
      case "contains":
        return new RisorBuiltin("contains", (args) => {
          const sub = (args[0] as RisorString).value;
          return toBool(str.value.includes(sub));
        });
      case "starts_with":
        return new RisorBuiltin("starts_with", (args) => {
          const prefix = (args[0] as RisorString).value;
          return toBool(str.value.startsWith(prefix));
        });
      case "ends_with":
        return new RisorBuiltin("ends_with", (args) => {
          const suffix = (args[0] as RisorString).value;
          return toBool(str.value.endsWith(suffix));
        });
      case "replace":
        return new RisorBuiltin("replace", (args) => {
          const old = (args[0] as RisorString).value;
          const newStr = (args[1] as RisorString).value;
          return new RisorString(str.value.replaceAll(old, newStr));
        });
      default:
        return null;
    }
  }

  private getListMethod(list: RisorList, name: string): RisorObject | null {
    switch (name) {
      case "len":
        return new RisorBuiltin("len", () => new RisorInt(list.items.length));
      case "append":
        return new RisorBuiltin("append", (args) => {
          list.append(args[0]);
          return NIL;
        });
      case "pop":
        return new RisorBuiltin("pop", () => {
          if (list.items.length === 0) {
            throw new VMError("pop from empty list");
          }
          return list.items.pop()!;
        });
      case "map":
        return new RisorBuiltin("map", (args) => {
          // args[0] is self (list), args[1] is the callback
          const fn = args[1];
          const result: RisorObject[] = [];
          for (const item of list.items) {
            result.push(this.callFunction(fn, [item]));
          }
          return new RisorList(result);
        });
      case "filter":
        return new RisorBuiltin("filter", (args) => {
          // args[0] is self (list), args[1] is the callback
          const fn = args[1];
          const result: RisorObject[] = [];
          for (const item of list.items) {
            if (this.callFunction(fn, [item]).isTruthy()) {
              result.push(item);
            }
          }
          return new RisorList(result);
        });
      case "reduce":
        return new RisorBuiltin("reduce", (args) => {
          // args[0] is self (list), args[1] is callback, args[2] is optional initial value
          const fn = args[1];
          let acc = args.length > 2 ? args[2] : list.items[0];
          const start = args.length > 2 ? 0 : 1;
          for (let i = start; i < list.items.length; i++) {
            acc = this.callFunction(fn, [acc, list.items[i]]);
          }
          return acc;
        });
      case "each":
        return new RisorBuiltin("each", (args) => {
          // args[0] is self (list), args[1] is the callback
          const fn = args[1];
          for (let i = 0; i < list.items.length; i++) {
            this.callFunction(fn, [list.items[i], new RisorInt(i)]);
          }
          return NIL;
        });
      case "join":
        return new RisorBuiltin("join", (args) => {
          const sep = args.length > 0 ? (args[0] as RisorString).value : "";
          return new RisorString(list.items.map((i) => i.inspect()).join(sep));
        });
      case "reverse":
        return new RisorBuiltin("reverse", () => {
          return new RisorList([...list.items].reverse());
        });
      case "sort":
        return new RisorBuiltin("sort", () => {
          const sorted = [...list.items].sort((a, b) => {
            try {
              return this.compare(a, b);
            } catch {
              return 0;
            }
          });
          return new RisorList(sorted);
        });
      case "contains":
        return new RisorBuiltin("contains", (args) => {
          return toBool(list.items.some((i) => i.equals(args[0])));
        });
      case "index":
        return new RisorBuiltin("index", (args) => {
          const idx = list.items.findIndex((i) => i.equals(args[0]));
          return idx >= 0 ? new RisorInt(idx) : NIL;
        });
      default:
        return null;
    }
  }

  private getMapMethod(map: RisorMap, name: string): RisorObject | null {
    switch (name) {
      case "len":
        return new RisorBuiltin("len", () => new RisorInt(map.size));
      case "keys":
        return new RisorBuiltin("keys", () => new RisorIter(map.getKeys()));
      case "values":
        return new RisorBuiltin("values", () => new RisorIter(map.getValues()));
      case "entries":
        return new RisorBuiltin("entries", () => {
          return new RisorIter(map.entries().map(([k, v]) => new RisorList([k, v])));
        });
      case "get":
        return new RisorBuiltin("get", (args) => {
          const value = map.get(args[0]);
          if (value !== undefined) return value;
          return args.length > 1 ? args[1] : NIL;
        });
      case "set":
        return new RisorBuiltin("set", (args) => {
          map.set(args[0], args[1]);
          return NIL;
        });
      case "delete":
        return new RisorBuiltin("delete", (args) => {
          return toBool(map.delete(args[0]));
        });
      case "has":
        return new RisorBuiltin("has", (args) => {
          return toBool(map.has(args[0]));
        });
      case "each":
        return new RisorBuiltin("each", (args) => {
          // args[0] is self (map), args[1] is the callback
          const fn = args[1];
          for (const [key, value] of map.entries()) {
            this.callFunction(fn, [key, value]);
          }
          return NIL;
        });
      default:
        return null;
    }
  }

  private callFunction(fn: RisorObject, args: RisorObject[]): RisorObject {
    if (fn.type === ObjectType.Builtin) {
      return (fn as RisorBuiltin).fn(args);
    }
    if (fn.type === ObjectType.Closure) {
      // Save current state
      const savedFrame = this.frame;

      // Set up call
      const closure = fn as RisorClosure;
      const frame = Frame.fromClosure(closure, this.sp, args);
      this.frames.push(frame);
      this.frame = frame;

      // Execute until return - stop when we drop back to current frame depth
      const result = this.execute(this.frames.length);

      // Restore state (frame should already be popped by return)
      this.frame = savedFrame;

      return result;
    }
    throw new VMError(`cannot call ${fn.type}`);
  }
}
