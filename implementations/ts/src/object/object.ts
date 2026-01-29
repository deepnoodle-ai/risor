/**
 * Risor object system - runtime values for the VM.
 */

import { Code } from "../bytecode/code.js";

/**
 * Object type enumeration.
 */
export const enum ObjectType {
  Nil = "nil",
  Bool = "bool",
  Int = "int",
  Float = "float",
  String = "string",
  List = "list",
  Map = "map",
  Closure = "closure",
  Builtin = "builtin",
  Cell = "cell",
  Iter = "iter",
  Error = "error",
}

/**
 * Base interface for all Risor objects.
 */
export interface RisorObject {
  /** Object type identifier. */
  readonly type: ObjectType;
  /** String representation for debugging. */
  inspect(): string;
  /** Check truthiness. */
  isTruthy(): boolean;
  /** Check equality with another object. */
  equals(other: RisorObject): boolean;
  /** Get hash key for map usage. */
  hashKey?(): string;
}

/**
 * Nil singleton - represents absence of value.
 */
export class RisorNil implements RisorObject {
  readonly type = ObjectType.Nil;

  inspect(): string {
    return "nil";
  }

  isTruthy(): boolean {
    return false;
  }

  equals(other: RisorObject): boolean {
    return other.type === ObjectType.Nil;
  }

  hashKey(): string {
    return "nil";
  }
}

/** The singleton nil value. */
export const NIL = Object.freeze(new RisorNil());

/**
 * Boolean value.
 */
export class RisorBool implements RisorObject {
  readonly type = ObjectType.Bool;

  constructor(public readonly value: boolean) {}

  inspect(): string {
    return this.value ? "true" : "false";
  }

  isTruthy(): boolean {
    return this.value;
  }

  equals(other: RisorObject): boolean {
    return other.type === ObjectType.Bool && (other as RisorBool).value === this.value;
  }

  hashKey(): string {
    return this.value ? "true" : "false";
  }
}

/** Singleton true value. */
export const TRUE = Object.freeze(new RisorBool(true));
/** Singleton false value. */
export const FALSE = Object.freeze(new RisorBool(false));

/** Get boolean singleton. */
export function toBool(value: boolean): RisorBool {
  return value ? TRUE : FALSE;
}

/**
 * Integer value.
 */
export class RisorInt implements RisorObject {
  readonly type = ObjectType.Int;

  constructor(public readonly value: number) {}

  inspect(): string {
    return String(this.value);
  }

  isTruthy(): boolean {
    return this.value !== 0;
  }

  equals(other: RisorObject): boolean {
    if (other.type === ObjectType.Int) {
      return (other as RisorInt).value === this.value;
    }
    if (other.type === ObjectType.Float) {
      return (other as RisorFloat).value === this.value;
    }
    return false;
  }

  hashKey(): string {
    return `int:${this.value}`;
  }
}

/**
 * Floating-point value.
 */
export class RisorFloat implements RisorObject {
  readonly type = ObjectType.Float;

  constructor(public readonly value: number) {}

  inspect(): string {
    return String(this.value);
  }

  isTruthy(): boolean {
    return this.value !== 0;
  }

  equals(other: RisorObject): boolean {
    if (other.type === ObjectType.Float) {
      return (other as RisorFloat).value === this.value;
    }
    if (other.type === ObjectType.Int) {
      return (other as RisorInt).value === this.value;
    }
    return false;
  }

  hashKey(): string {
    return `float:${this.value}`;
  }
}

/**
 * String value.
 */
export class RisorString implements RisorObject {
  readonly type = ObjectType.String;

  constructor(public readonly value: string) {}

  inspect(): string {
    return `"${this.value}"`;
  }

  isTruthy(): boolean {
    return this.value.length > 0;
  }

  equals(other: RisorObject): boolean {
    return other.type === ObjectType.String && (other as RisorString).value === this.value;
  }

  hashKey(): string {
    return `str:${this.value}`;
  }

  /** Get character at index. */
  charAt(index: number): RisorString {
    const len = this.value.length;
    const idx = index < 0 ? len + index : index;
    if (idx < 0 || idx >= len) {
      throw new Error(`string index out of range: ${index}`);
    }
    return new RisorString(this.value[idx]);
  }

  /** Get substring. */
  slice(start?: number, end?: number): RisorString {
    const len = this.value.length;
    const s = start === undefined ? 0 : start < 0 ? Math.max(0, len + start) : start;
    const e = end === undefined ? len : end < 0 ? Math.max(0, len + end) : end;
    return new RisorString(this.value.slice(s, e));
  }
}

/**
 * List (array) value.
 */
export class RisorList implements RisorObject {
  readonly type = ObjectType.List;

  constructor(public readonly items: RisorObject[]) {}

  inspect(): string {
    return `[${this.items.map((i) => i.inspect()).join(", ")}]`;
  }

  isTruthy(): boolean {
    return this.items.length > 0;
  }

  equals(other: RisorObject): boolean {
    if (other.type !== ObjectType.List) return false;
    const otherList = other as RisorList;
    if (this.items.length !== otherList.items.length) return false;
    return this.items.every((item, i) => item.equals(otherList.items[i]));
  }

  /** Get item at index. */
  get(index: number): RisorObject {
    const len = this.items.length;
    const idx = index < 0 ? len + index : index;
    if (idx < 0 || idx >= len) {
      throw new Error(`list index out of range: ${index}`);
    }
    return this.items[idx];
  }

  /** Set item at index (mutates). */
  set(index: number, value: RisorObject): void {
    const len = this.items.length;
    const idx = index < 0 ? len + index : index;
    if (idx < 0 || idx >= len) {
      throw new Error(`list index out of range: ${index}`);
    }
    this.items[idx] = value;
  }

  /** Get slice of list. */
  slice(start?: number, end?: number): RisorList {
    const len = this.items.length;
    const s = start === undefined ? 0 : start < 0 ? Math.max(0, len + start) : start;
    const e = end === undefined ? len : end < 0 ? Math.max(0, len + end) : end;
    return new RisorList(this.items.slice(s, e));
  }

  /** Append item (mutates). */
  append(item: RisorObject): void {
    this.items.push(item);
  }

  /** Extend with items from another list (mutates). */
  extend(other: RisorList): void {
    this.items.push(...other.items);
  }
}

/**
 * Map (dictionary) value.
 */
export class RisorMap implements RisorObject {
  readonly type = ObjectType.Map;
  private readonly data: Map<string, RisorObject>;
  private readonly keys: Map<string, RisorObject>;

  constructor(entries?: [RisorObject, RisorObject][]) {
    this.data = new Map();
    this.keys = new Map();
    if (entries) {
      for (const [key, value] of entries) {
        this.set(key, value);
      }
    }
  }

  inspect(): string {
    const pairs: string[] = [];
    for (const [hashKey, value] of this.data) {
      const key = this.keys.get(hashKey)!;
      pairs.push(`${key.inspect()}: ${value.inspect()}`);
    }
    return `{${pairs.join(", ")}}`;
  }

  isTruthy(): boolean {
    return this.data.size > 0;
  }

  equals(other: RisorObject): boolean {
    if (other.type !== ObjectType.Map) return false;
    const otherMap = other as RisorMap;
    if (this.data.size !== otherMap.data.size) return false;
    for (const [hashKey, value] of this.data) {
      const otherValue = otherMap.data.get(hashKey);
      if (!otherValue || !value.equals(otherValue)) return false;
    }
    return true;
  }

  /** Get value by key. */
  get(key: RisorObject): RisorObject | undefined {
    const hashKey = this.getHashKey(key);
    return this.data.get(hashKey);
  }

  /** Set value by key (mutates). */
  set(key: RisorObject, value: RisorObject): void {
    const hashKey = this.getHashKey(key);
    this.data.set(hashKey, value);
    this.keys.set(hashKey, key);
  }

  /** Check if key exists. */
  has(key: RisorObject): boolean {
    const hashKey = this.getHashKey(key);
    return this.data.has(hashKey);
  }

  /** Delete key (mutates). */
  delete(key: RisorObject): boolean {
    const hashKey = this.getHashKey(key);
    this.keys.delete(hashKey);
    return this.data.delete(hashKey);
  }

  /** Get number of entries. */
  get size(): number {
    return this.data.size;
  }

  /** Get all keys. */
  getKeys(): RisorObject[] {
    return Array.from(this.keys.values());
  }

  /** Get all values. */
  getValues(): RisorObject[] {
    return Array.from(this.data.values());
  }

  /** Get all entries. */
  entries(): [RisorObject, RisorObject][] {
    const result: [RisorObject, RisorObject][] = [];
    for (const [hashKey, value] of this.data) {
      result.push([this.keys.get(hashKey)!, value]);
    }
    return result;
  }

  /** Merge another map into this one (mutates). */
  merge(other: RisorMap): void {
    for (const [key, value] of other.entries()) {
      this.set(key, value);
    }
  }

  private getHashKey(key: RisorObject): string {
    if (key.hashKey) {
      return key.hashKey();
    }
    throw new Error(`unhashable type: ${key.type}`);
  }
}

/**
 * Cell for captured variables in closures.
 * Allows mutation of captured values.
 */
export class RisorCell implements RisorObject {
  readonly type = ObjectType.Cell;

  constructor(public value: RisorObject) {}

  inspect(): string {
    return `<cell: ${this.value.inspect()}>`;
  }

  isTruthy(): boolean {
    return true;
  }

  equals(other: RisorObject): boolean {
    return this === other;
  }
}

/**
 * Closure - a function with captured free variables.
 */
export class RisorClosure implements RisorObject {
  readonly type = ObjectType.Closure;

  constructor(
    public readonly code: Code,
    public readonly freeVars: RisorCell[]
  ) {}

  inspect(): string {
    const name = this.code.name || "anonymous";
    return `<closure: ${name}>`;
  }

  isTruthy(): boolean {
    return true;
  }

  equals(other: RisorObject): boolean {
    return this === other;
  }
}

/**
 * Builtin function type.
 */
export type BuiltinFn = (args: RisorObject[]) => RisorObject;

/**
 * Builtin function wrapper.
 */
export class RisorBuiltin implements RisorObject {
  readonly type = ObjectType.Builtin;

  constructor(
    public readonly name: string,
    public readonly fn: BuiltinFn
  ) {}

  inspect(): string {
    return `<builtin: ${this.name}>`;
  }

  isTruthy(): boolean {
    return true;
  }

  equals(other: RisorObject): boolean {
    return this === other;
  }
}

/**
 * Iterator for iteration operations.
 */
export class RisorIter implements RisorObject {
  readonly type = ObjectType.Iter;
  private index = 0;

  constructor(private readonly items: RisorObject[]) {}

  inspect(): string {
    return `<iter: ${this.items.length} items>`;
  }

  isTruthy(): boolean {
    return true;
  }

  equals(other: RisorObject): boolean {
    return this === other;
  }

  /** Get next item or undefined if exhausted. */
  next(): RisorObject | undefined {
    if (this.index >= this.items.length) {
      return undefined;
    }
    return this.items[this.index++];
  }

  /** Check if there are more items. */
  hasNext(): boolean {
    return this.index < this.items.length;
  }

  /** Reset iterator. */
  reset(): void {
    this.index = 0;
  }

  /** Get remaining items as list. */
  toList(): RisorList {
    return new RisorList(this.items.slice(this.index));
  }
}

/**
 * Error value for runtime errors.
 */
export class RisorError implements RisorObject {
  readonly type = ObjectType.Error;

  constructor(public readonly message: string) {}

  inspect(): string {
    return `error: ${this.message}`;
  }

  isTruthy(): boolean {
    return true;
  }

  equals(other: RisorObject): boolean {
    return other.type === ObjectType.Error && (other as RisorError).message === this.message;
  }
}

/**
 * Create a Risor object from a JavaScript value.
 */
export function fromJS(value: unknown): RisorObject {
  if (value === null || value === undefined) {
    return NIL;
  }
  if (typeof value === "boolean") {
    return toBool(value);
  }
  if (typeof value === "number") {
    return Number.isInteger(value) ? new RisorInt(value) : new RisorFloat(value);
  }
  if (typeof value === "string") {
    return new RisorString(value);
  }
  if (Array.isArray(value)) {
    return new RisorList(value.map(fromJS));
  }
  if (typeof value === "object") {
    const entries: [RisorObject, RisorObject][] = Object.entries(value).map(([k, v]) => [
      new RisorString(k),
      fromJS(v),
    ]);
    return new RisorMap(entries);
  }
  throw new Error(`cannot convert ${typeof value} to Risor object`);
}

/**
 * Convert a Risor object to a JavaScript value.
 */
export function toJS(obj: RisorObject): unknown {
  switch (obj.type) {
    case ObjectType.Nil:
      return null;
    case ObjectType.Bool:
      return (obj as RisorBool).value;
    case ObjectType.Int:
      return (obj as RisorInt).value;
    case ObjectType.Float:
      return (obj as RisorFloat).value;
    case ObjectType.String:
      return (obj as RisorString).value;
    case ObjectType.List:
      return (obj as RisorList).items.map(toJS);
    case ObjectType.Map: {
      const result: Record<string, unknown> = {};
      for (const [key, value] of (obj as RisorMap).entries()) {
        if (key.type === ObjectType.String) {
          result[(key as RisorString).value] = toJS(value);
        }
      }
      return result;
    }
    default:
      return obj.inspect();
  }
}
