/**
 * Built-in functions for Risor.
 */

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
  RisorBuiltin,
  RisorIter,
  RisorError,
  toJS,
  fromJS,
} from "../object/object.js";

/**
 * Create the standard builtins map.
 */
export function createBuiltins(): Map<string, RisorObject> {
  const builtins = new Map<string, RisorObject>();

  // print - output values to console
  builtins.set(
    "print",
    new RisorBuiltin("print", (args) => {
      const parts = args.map((arg) =>
        arg.type === ObjectType.String ? (arg as RisorString).value : arg.inspect()
      );
      console.log(parts.join(" "));
      return NIL;
    })
  );

  // len - get length of string, list, or map
  builtins.set(
    "len",
    new RisorBuiltin("len", (args) => {
      if (args.length !== 1) {
        throw new Error("len() takes exactly 1 argument");
      }
      const obj = args[0];
      switch (obj.type) {
        case ObjectType.String:
          return new RisorInt((obj as RisorString).value.length);
        case ObjectType.List:
          return new RisorInt((obj as RisorList).items.length);
        case ObjectType.Map:
          return new RisorInt((obj as RisorMap).size);
        default:
          throw new Error(`len() not supported for ${obj.type}`);
      }
    })
  );

  // type - get type name of value
  builtins.set(
    "type",
    new RisorBuiltin("type", (args) => {
      if (args.length !== 1) {
        throw new Error("type() takes exactly 1 argument");
      }
      return new RisorString(args[0].type);
    })
  );

  // string - convert to string
  builtins.set(
    "string",
    new RisorBuiltin("string", (args) => {
      if (args.length !== 1) {
        throw new Error("string() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type === ObjectType.String) {
        return obj;
      }
      return new RisorString(obj.inspect());
    })
  );

  // int - convert to integer
  builtins.set(
    "int",
    new RisorBuiltin("int", (args) => {
      if (args.length !== 1) {
        throw new Error("int() takes exactly 1 argument");
      }
      const obj = args[0];
      switch (obj.type) {
        case ObjectType.Int:
          return obj;
        case ObjectType.Float:
          return new RisorInt(Math.trunc((obj as RisorFloat).value));
        case ObjectType.String: {
          const n = parseInt((obj as RisorString).value, 10);
          if (isNaN(n)) {
            throw new Error(`cannot convert "${(obj as RisorString).value}" to int`);
          }
          return new RisorInt(n);
        }
        case ObjectType.Bool:
          return new RisorInt((obj as typeof TRUE).value ? 1 : 0);
        default:
          throw new Error(`cannot convert ${obj.type} to int`);
      }
    })
  );

  // float - convert to float
  builtins.set(
    "float",
    new RisorBuiltin("float", (args) => {
      if (args.length !== 1) {
        throw new Error("float() takes exactly 1 argument");
      }
      const obj = args[0];
      switch (obj.type) {
        case ObjectType.Float:
          return obj;
        case ObjectType.Int:
          return new RisorFloat((obj as RisorInt).value);
        case ObjectType.String: {
          const n = parseFloat((obj as RisorString).value);
          if (isNaN(n)) {
            throw new Error(`cannot convert "${(obj as RisorString).value}" to float`);
          }
          return new RisorFloat(n);
        }
        default:
          throw new Error(`cannot convert ${obj.type} to float`);
      }
    })
  );

  // bool - convert to boolean
  builtins.set(
    "bool",
    new RisorBuiltin("bool", (args) => {
      if (args.length !== 1) {
        throw new Error("bool() takes exactly 1 argument");
      }
      return toBool(args[0].isTruthy());
    })
  );

  // list - create or convert to list
  builtins.set(
    "list",
    new RisorBuiltin("list", (args) => {
      if (args.length === 0) {
        return new RisorList([]);
      }
      const obj = args[0];
      switch (obj.type) {
        case ObjectType.List:
          return new RisorList([...(obj as RisorList).items]);
        case ObjectType.String:
          return new RisorList([...(obj as RisorString).value].map((c) => new RisorString(c)));
        case ObjectType.Iter:
          return (obj as RisorIter).toList();
        case ObjectType.Map:
          return new RisorList((obj as RisorMap).getKeys());
        default:
          throw new Error(`cannot convert ${obj.type} to list`);
      }
    })
  );

  // map - create empty map or convert to map
  builtins.set(
    "map",
    new RisorBuiltin("map", (args) => {
      if (args.length === 0) {
        return new RisorMap();
      }
      throw new Error("map() with arguments not yet implemented");
    })
  );

  // iter - create iterator
  builtins.set(
    "iter",
    new RisorBuiltin("iter", (args) => {
      if (args.length === 0) {
        return new RisorIter([]);
      }
      const obj = args[0];
      switch (obj.type) {
        case ObjectType.List:
          return new RisorIter([...(obj as RisorList).items]);
        case ObjectType.String:
          return new RisorIter([...(obj as RisorString).value].map((c) => new RisorString(c)));
        case ObjectType.Map:
          return new RisorIter((obj as RisorMap).getKeys());
        case ObjectType.Iter:
          return obj;
        default:
          throw new Error(`cannot create iterator from ${obj.type}`);
      }
    })
  );

  // range - create range iterator
  builtins.set(
    "range",
    new RisorBuiltin("range", (args) => {
      let start = 0;
      let end = 0;
      let step = 1;

      if (args.length === 1) {
        end = (args[0] as RisorInt).value;
      } else if (args.length === 2) {
        start = (args[0] as RisorInt).value;
        end = (args[1] as RisorInt).value;
      } else if (args.length === 3) {
        start = (args[0] as RisorInt).value;
        end = (args[1] as RisorInt).value;
        step = (args[2] as RisorInt).value;
      } else {
        throw new Error("range() takes 1 to 3 arguments");
      }

      if (step === 0) {
        throw new Error("range() step cannot be zero");
      }

      const items: RisorObject[] = [];
      if (step > 0) {
        for (let i = start; i < end; i += step) {
          items.push(new RisorInt(i));
        }
      } else {
        for (let i = start; i > end; i += step) {
          items.push(new RisorInt(i));
        }
      }
      return new RisorIter(items);
    })
  );

  // error - create error
  builtins.set(
    "error",
    new RisorBuiltin("error", (args) => {
      if (args.length !== 1) {
        throw new Error("error() takes exactly 1 argument");
      }
      const msg =
        args[0].type === ObjectType.String ? (args[0] as RisorString).value : args[0].inspect();
      return new RisorError(msg);
    })
  );

  // assert - assertion for testing
  builtins.set(
    "assert",
    new RisorBuiltin("assert", (args) => {
      if (args.length < 1) {
        throw new Error("assert() requires at least 1 argument");
      }
      if (!args[0].isTruthy()) {
        const msg = args.length > 1 ? (args[1] as RisorString).value : "assertion failed";
        throw new Error(msg);
      }
      return NIL;
    })
  );

  // keys - get map keys
  builtins.set(
    "keys",
    new RisorBuiltin("keys", (args) => {
      if (args.length !== 1) {
        throw new Error("keys() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type !== ObjectType.Map) {
        throw new Error(`keys() requires map, got ${obj.type}`);
      }
      return new RisorIter((obj as RisorMap).getKeys());
    })
  );

  // values - get map values
  builtins.set(
    "values",
    new RisorBuiltin("values", (args) => {
      if (args.length !== 1) {
        throw new Error("values() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type !== ObjectType.Map) {
        throw new Error(`values() requires map, got ${obj.type}`);
      }
      return new RisorIter((obj as RisorMap).getValues());
    })
  );

  // sorted - return sorted list
  builtins.set(
    "sorted",
    new RisorBuiltin("sorted", (args) => {
      if (args.length !== 1) {
        throw new Error("sorted() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type !== ObjectType.List) {
        throw new Error(`sorted() requires list, got ${obj.type}`);
      }
      const items = [...(obj as RisorList).items];
      items.sort((a, b) => {
        if (a.type === ObjectType.Int && b.type === ObjectType.Int) {
          return (a as RisorInt).value - (b as RisorInt).value;
        }
        if (a.type === ObjectType.Float && b.type === ObjectType.Float) {
          return (a as RisorFloat).value - (b as RisorFloat).value;
        }
        if (a.type === ObjectType.String && b.type === ObjectType.String) {
          return (a as RisorString).value.localeCompare((b as RisorString).value);
        }
        return 0;
      });
      return new RisorList(items);
    })
  );

  // reversed - return reversed list
  builtins.set(
    "reversed",
    new RisorBuiltin("reversed", (args) => {
      if (args.length !== 1) {
        throw new Error("reversed() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type === ObjectType.List) {
        return new RisorList([...(obj as RisorList).items].reverse());
      }
      if (obj.type === ObjectType.String) {
        return new RisorString([...(obj as RisorString).value].reverse().join(""));
      }
      throw new Error(`reversed() requires list or string, got ${obj.type}`);
    })
  );

  // min - minimum value
  builtins.set(
    "min",
    new RisorBuiltin("min", (args) => {
      if (args.length === 0) {
        throw new Error("min() requires at least 1 argument");
      }
      let items: RisorObject[];
      if (args.length === 1 && args[0].type === ObjectType.List) {
        items = (args[0] as RisorList).items;
      } else {
        items = args;
      }
      if (items.length === 0) {
        throw new Error("min() of empty sequence");
      }
      let min = items[0];
      for (let i = 1; i < items.length; i++) {
        const val = items[i];
        if (compareValues(val, min) < 0) {
          min = val;
        }
      }
      return min;
    })
  );

  // max - maximum value
  builtins.set(
    "max",
    new RisorBuiltin("max", (args) => {
      if (args.length === 0) {
        throw new Error("max() requires at least 1 argument");
      }
      let items: RisorObject[];
      if (args.length === 1 && args[0].type === ObjectType.List) {
        items = (args[0] as RisorList).items;
      } else {
        items = args;
      }
      if (items.length === 0) {
        throw new Error("max() of empty sequence");
      }
      let max = items[0];
      for (let i = 1; i < items.length; i++) {
        const val = items[i];
        if (compareValues(val, max) > 0) {
          max = val;
        }
      }
      return max;
    })
  );

  // sum - sum of numbers
  builtins.set(
    "sum",
    new RisorBuiltin("sum", (args) => {
      if (args.length === 0) {
        throw new Error("sum() requires at least 1 argument");
      }
      let items: RisorObject[];
      if (args.length === 1 && args[0].type === ObjectType.List) {
        items = (args[0] as RisorList).items;
      } else {
        items = args;
      }
      let total = 0;
      let isFloat = false;
      for (const item of items) {
        if (item.type === ObjectType.Int) {
          total += (item as RisorInt).value;
        } else if (item.type === ObjectType.Float) {
          total += (item as RisorFloat).value;
          isFloat = true;
        } else {
          throw new Error(`cannot sum ${item.type}`);
        }
      }
      return isFloat ? new RisorFloat(total) : new RisorInt(total);
    })
  );

  // abs - absolute value
  builtins.set(
    "abs",
    new RisorBuiltin("abs", (args) => {
      if (args.length !== 1) {
        throw new Error("abs() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type === ObjectType.Int) {
        return new RisorInt(Math.abs((obj as RisorInt).value));
      }
      if (obj.type === ObjectType.Float) {
        return new RisorFloat(Math.abs((obj as RisorFloat).value));
      }
      throw new Error(`abs() requires number, got ${obj.type}`);
    })
  );

  // round - round to nearest integer
  builtins.set(
    "round",
    new RisorBuiltin("round", (args) => {
      if (args.length !== 1) {
        throw new Error("round() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type === ObjectType.Int) {
        return obj;
      }
      if (obj.type === ObjectType.Float) {
        return new RisorInt(Math.round((obj as RisorFloat).value));
      }
      throw new Error(`round() requires number, got ${obj.type}`);
    })
  );

  // floor - floor value
  builtins.set(
    "floor",
    new RisorBuiltin("floor", (args) => {
      if (args.length !== 1) {
        throw new Error("floor() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type === ObjectType.Int) {
        return obj;
      }
      if (obj.type === ObjectType.Float) {
        return new RisorInt(Math.floor((obj as RisorFloat).value));
      }
      throw new Error(`floor() requires number, got ${obj.type}`);
    })
  );

  // ceil - ceiling value
  builtins.set(
    "ceil",
    new RisorBuiltin("ceil", (args) => {
      if (args.length !== 1) {
        throw new Error("ceil() takes exactly 1 argument");
      }
      const obj = args[0];
      if (obj.type === ObjectType.Int) {
        return obj;
      }
      if (obj.type === ObjectType.Float) {
        return new RisorInt(Math.ceil((obj as RisorFloat).value));
      }
      throw new Error(`ceil() requires number, got ${obj.type}`);
    })
  );

  return builtins;
}

/**
 * Compare two values, returning negative, zero, or positive.
 */
function compareValues(a: RisorObject, b: RisorObject): number {
  if (a.type === ObjectType.Int && b.type === ObjectType.Int) {
    return (a as RisorInt).value - (b as RisorInt).value;
  }
  if (a.type === ObjectType.Float || b.type === ObjectType.Float) {
    const av = a.type === ObjectType.Int ? (a as RisorInt).value : (a as RisorFloat).value;
    const bv = b.type === ObjectType.Int ? (b as RisorInt).value : (b as RisorFloat).value;
    return av - bv;
  }
  if (a.type === ObjectType.String && b.type === ObjectType.String) {
    return (a as RisorString).value.localeCompare((b as RisorString).value);
  }
  return 0;
}
