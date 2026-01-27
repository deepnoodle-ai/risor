// Package builtins defines a default set of built-in functions.
package builtins

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/risor-io/risor/object"
)

func Len(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("len: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case object.Container:
		return arg.Len(), nil
	default:
		return nil, fmt.Errorf("type error: len() unsupported argument (%s given)", args[0].Type())
	}
}

func Sprintf(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 64 {
		return nil, fmt.Errorf("sprintf: expected 1-64 arguments, got %d", len(args))
	}
	fs, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	fmtArgs := make([]interface{}, len(args)-1)
	for i, v := range args[1:] {
		fmtArgs[i] = v.Interface()
	}
	result := object.NewString(fmt.Sprintf(fs, fmtArgs...))
	return result, nil
}

// Error creates an error value without throwing it. Use throw to raise the error.
// Example: let err = error("file %s not found", filename)
func Error(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 64 {
		return nil, fmt.Errorf("error: expected 1-64 arguments, got %d", len(args))
	}
	fs, err := object.AsString(args[0])
	if err != nil {
		return nil, err
	}
	fmtArgs := make([]interface{}, len(args)-1)
	for i, v := range args[1:] {
		fmtArgs[i] = v.Interface()
	}
	return object.NewError(fmt.Errorf(fs, fmtArgs...)), nil
}

func List(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("list: expected 0-1 arguments, got %d", len(args))
	}
	if len(args) == 0 {
		return object.NewList(nil), nil
	}
	// Reject int arguments explicitly
	if _, ok := args[0].(*object.Int); ok {
		return nil, fmt.Errorf("type error: list() expected an enumerable (int given)")
	}
	enumerable, ok := args[0].(object.Enumerable)
	if !ok {
		return nil, fmt.Errorf("type error: list() expected an enumerable (%s given)", args[0].Type())
	}
	var items []object.Object
	enumerable.Enumerate(ctx, func(key, value object.Object) bool {
		items = append(items, value)
		return true
	})
	return object.NewList(items), nil
}

func String(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("string: expected 0-1 arguments, got %d", len(args))
	}
	if len(args) == 0 {
		return object.NewString(""), nil
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.String:
		return object.NewString(arg.Value()), nil
	case io.Reader:
		bytes, err := io.ReadAll(arg)
		if err != nil {
			return nil, err
		}
		return object.NewString(string(bytes)), nil
	default:
		if s, ok := arg.(fmt.Stringer); ok {
			return object.NewString(s.String()), nil
		}
		return object.NewString(args[0].Inspect()), nil
	}
}

func Type(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("type: expected 1 argument, got %d", len(args))
	}
	return object.NewString(string(args[0].Type())), nil
}

func Assert(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("assert: expected 1-2 arguments, got %d", len(args))
	}
	if !args[0].IsTruthy() {
		if len(args) == 2 {
			switch arg := args[1].(type) {
			case *object.String:
				return nil, fmt.Errorf("%s", arg.Value())
			default:
				return nil, fmt.Errorf("%s", args[1].Inspect())
			}
		}
		return nil, fmt.Errorf("assertion failed")
	}
	return object.Nil, nil
}

func Any(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("any: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.List:
		for _, val := range arg.Value() {
			if val.IsTruthy() {
				return object.True, nil
			}
		}
	case object.Enumerable:
		found := false
		arg.Enumerate(ctx, func(key, value object.Object) bool {
			if value.IsTruthy() {
				found = true
				return false
			}
			return true
		})
		if found {
			return object.True, nil
		}
	default:
		return nil, fmt.Errorf("type error: any() argument must be a container (%s given)", args[0].Type())
	}
	return object.False, nil
}

func All(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("all: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.List:
		for _, val := range arg.Value() {
			if !val.IsTruthy() {
				return object.False, nil
			}
		}
	case object.Enumerable:
		allTruthy := true
		arg.Enumerate(ctx, func(key, value object.Object) bool {
			if !value.IsTruthy() {
				allTruthy = false
				return false
			}
			return true
		})
		if !allTruthy {
			return object.False, nil
		}
	default:
		return nil, fmt.Errorf("type error: all() argument must be a container (%s given)", args[0].Type())
	}
	return object.True, nil
}

func Filter(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("filter: expected 2 arguments, got %d", len(args))
	}
	fn, ok := args[1].(object.Callable)
	if !ok {
		return nil, fmt.Errorf("type error: filter() expected a callable (%s given)", args[1].Type())
	}
	var result []object.Object
	switch container := args[0].(type) {
	case *object.List:
		for _, val := range container.Value() {
			decision, err := fn.Call(ctx, val)
			if err != nil {
				return nil, err
			}
			if decision.IsTruthy() {
				result = append(result, val)
			}
		}
	case object.Enumerable:
		var filterErr error
		container.Enumerate(ctx, func(key, value object.Object) bool {
			decision, err := fn.Call(ctx, value)
			if err != nil {
				filterErr = err
				return false
			}
			if decision.IsTruthy() {
				result = append(result, value)
			}
			return true
		})
		if filterErr != nil {
			return nil, filterErr
		}
	default:
		return nil, fmt.Errorf("type error: filter() argument must be a container (%s given)", args[0].Type())
	}
	return object.NewList(result), nil
}

func Bool(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("bool: expected 0-1 arguments, got %d", len(args))
	}
	if len(args) == 0 {
		return object.False, nil
	}
	if args[0].IsTruthy() {
		return object.True, nil
	}
	return object.False, nil
}

func Sorted(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("sorted: expected 1-2 arguments, got %d", len(args))
	}
	arg := args[0]
	var items []object.Object
	switch arg := arg.(type) {
	case *object.List:
		items = arg.Value()
	case *object.Map:
		items = arg.Keys().Value()
	case *object.String:
		items = arg.Runes()
	default:
		return nil, fmt.Errorf("type error: sorted() unsupported argument (%s given)", arg.Type())
	}
	resultItems := make([]object.Object, len(items))
	copy(resultItems, items)
	if len(args) == 2 {
		callable, ok := args[1].(object.Callable)
		if !ok {
			return nil, fmt.Errorf("type error: sorted() expected a function as the second argument (%s given)", args[1].Type())
		}
		var sortErr error
		sort.SliceStable(resultItems, func(i, j int) bool {
			result, err := callable.Call(ctx, resultItems[i], resultItems[j])
			if err != nil {
				sortErr = err
				return false
			}
			return result.IsTruthy()
		})
		if sortErr != nil {
			return nil, sortErr
		}
	} else {
		if err := object.Sort(resultItems); err != nil {
			return nil, err
		}
	}
	return object.NewList(resultItems), nil
}

func Reversed(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("reversed: expected 1 argument, got %d", len(args))
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.List:
		return arg.Reversed(), nil
	case *object.String:
		return arg.Reversed(), nil
	default:
		return nil, fmt.Errorf("type error: reversed() unsupported argument (%s given)", arg.Type())
	}
}

func GetAttr(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("getattr: expected 2-3 arguments, got %d", len(args))
	}
	attrName, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}
	if attr, found := args[0].GetAttr(attrName); found {
		return attr, nil
	}
	if len(args) == 3 {
		return args[2], nil
	}
	return nil, fmt.Errorf("type error: getattr() %s object has no attribute %q",
		args[0].Type(), attrName)
}

func Call(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) < 1 || len(args) > 64 {
		return nil, fmt.Errorf("call: expected 1-64 arguments, got %d", len(args))
	}
	callable, ok := args[0].(object.Callable)
	if !ok {
		return nil, fmt.Errorf("type error: call() unsupported argument (%s given)", args[0].Type())
	}
	return callable.Call(ctx, args[1:]...)
}

func Keys(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("keys: expected 1 argument, got %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Map:
		return arg.Keys(), nil
	case *object.List:
		return arg.Keys(), nil
	case object.Enumerable:
		var keys []object.Object
		arg.Enumerate(ctx, func(key, value object.Object) bool {
			keys = append(keys, key)
			return true
		})
		return object.NewList(keys), nil
	default:
		return nil, fmt.Errorf("type error: keys() unsupported argument (%s given)", args[0].Type())
	}
}

func Byte(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("byte: expected 0-1 arguments, got %d", len(args))
	}
	if len(args) == 0 {
		return object.NewByte(0), nil
	}
	switch obj := args[0].(type) {
	case *object.Int:
		return object.NewByte(byte(obj.Value())), nil
	case *object.Byte:
		return object.NewByte(obj.Value()), nil
	case *object.Float:
		return object.NewByte(byte(obj.Value())), nil
	case *object.String:
		if i, err := strconv.ParseInt(obj.Value(), 0, 8); err == nil {
			return object.NewByte(byte(i)), nil
		}
		return nil, fmt.Errorf("value error: invalid literal for byte(): %q", obj.Value())
	default:
		return nil, fmt.Errorf("type error: byte() unsupported argument (%s given)", args[0].Type())
	}
}

func Int(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("int: expected 0-1 arguments, got %d", len(args))
	}
	if len(args) == 0 {
		return object.NewInt(0), nil
	}
	switch obj := args[0].(type) {
	case *object.Int:
		return obj, nil
	case *object.Byte:
		return object.NewInt(int64(obj.Value())), nil
	case *object.Float:
		return object.NewInt(int64(obj.Value())), nil
	case *object.String:
		if i, err := strconv.ParseInt(obj.Value(), 0, 64); err == nil {
			return object.NewInt(i), nil
		}
		return nil, fmt.Errorf("value error: invalid literal for int(): %q", obj.Value())
	default:
		return nil, fmt.Errorf("type error: int() unsupported argument (%s given)", args[0].Type())
	}
}

func Float(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("float: expected 0-1 arguments, got %d", len(args))
	}
	if len(args) == 0 {
		return object.NewFloat(0), nil
	}
	switch obj := args[0].(type) {
	case *object.Int:
		return object.NewFloat(float64(obj.Value())), nil
	case *object.Byte:
		return object.NewFloat(float64(obj.Value())), nil
	case *object.Float:
		return obj, nil
	case *object.String:
		if f, err := strconv.ParseFloat(obj.Value(), 64); err == nil {
			return object.NewFloat(f), nil
		}
		return nil, fmt.Errorf("value error: invalid literal for float(): %q", obj.Value())
	default:
		return nil, fmt.Errorf("type error: float() unsupported argument (%s given)", args[0].Type())
	}
}

func Coalesce(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) > 64 {
		return nil, fmt.Errorf("coalesce: expected 0-64 arguments, got %d", len(args))
	}
	for _, arg := range args {
		if arg != object.Nil {
			return arg, nil
		}
	}
	return object.Nil, nil
}

func Chunk(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("chunk: expected 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*object.List)
	if !ok {
		return nil, fmt.Errorf("type error: chunk() expected a list (%s given)", args[0].Type())
	}
	listSize := int64(list.Size())
	chunkSizeObj, ok := args[1].(*object.Int)
	if !ok {
		return nil, fmt.Errorf("type error: chunk() expected an int (%s given)", args[1].Type())
	}
	chunkSize := chunkSizeObj.Value()
	if chunkSize <= 0 {
		return nil, fmt.Errorf("value error: chunk() size must be > 0 (%d given)", chunkSize)
	}
	items := list.Value()
	nChunks := listSize / chunkSize
	if listSize%chunkSize != 0 {
		nChunks++
	}
	chunks := make([]object.Object, nChunks)
	for i := int64(0); i < nChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > listSize {
			end = listSize
		}
		chunk := make([]object.Object, end-start)
		copy(chunk, items[start:end])
		chunks[i] = object.NewList(chunk)
	}
	return object.NewList(chunks), nil
}

func Builtins() map[string]object.Object {
	return map[string]object.Object{
		"all":      object.NewBuiltin("all", All),
		"any":      object.NewBuiltin("any", Any),
		"assert":   object.NewBuiltin("assert", Assert),
		"bool":     object.NewBuiltin("bool", Bool),
		"byte":     object.NewBuiltin("byte", Byte),
		"call":     object.NewBuiltin("call", Call),
		"chunk":    object.NewBuiltin("chunk", Chunk),
		"coalesce": object.NewBuiltin("coalesce", Coalesce),
		"decode":   object.NewBuiltin("decode", Decode),
		"encode":   object.NewBuiltin("encode", Encode),
		"error":    object.NewBuiltin("error", Error),
		"filter":   object.NewBuiltin("filter", Filter),
		"float":    object.NewBuiltin("float", Float),
		"getattr":  object.NewBuiltin("getattr", GetAttr),
		"int":      object.NewBuiltin("int", Int),
		"keys":     object.NewBuiltin("keys", Keys),
		"len":      object.NewBuiltin("len", Len),
		"list":     object.NewBuiltin("list", List),
		"reversed": object.NewBuiltin("reversed", Reversed),
		"sorted":   object.NewBuiltin("sorted", Sorted),
		"sprintf":  object.NewBuiltin("sprintf", Sprintf),
		"string":   object.NewBuiltin("string", String),
		"type":     object.NewBuiltin("type", Type),
	}
}
