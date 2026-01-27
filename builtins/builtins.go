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

func Len(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("len", 1, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case object.Container:
		return arg.Len()
	default:
		return object.TypeErrorf("type error: len() unsupported argument (%s given)", args[0].Type())
	}
}

func Sprintf(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("sprintf", 1, 64, args); err != nil {
		return err
	}
	fs, err := object.AsString(args[0])
	if err != nil {
		return err
	}
	fmtArgs := make([]interface{}, len(args)-1)
	for i, v := range args[1:] {
		fmtArgs[i] = v.Interface()
	}
	result := object.NewString(fmt.Sprintf(fs, fmtArgs...))
	return result
}

func Delete(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("delete", 2, args); err != nil {
		return err
	}
	container, ok := args[0].(object.Container)
	if !ok {
		return object.TypeErrorf("type error: delete() unsupported argument (%s given)", args[0].Type())
	}
	if err := container.DelItem(args[1]); err != nil {
		return err
	}
	return object.Nil
}

func List(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("list", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewList(nil)
	}
	if intObj, ok := args[0].(*object.Int); ok {
		count := intObj.Value()
		if count < 0 {
			return object.Errorf("value error: list() argument must be >= 0 (%d given)", count)
		}
		arr := make([]object.Object, count)
		for i := 0; i < int(count); i++ {
			arr[i] = object.Nil
		}
		return object.NewList(arr)
	}
	enumerable, ok := args[0].(object.Enumerable)
	if !ok {
		return object.TypeErrorf("type error: list() expected an enumerable (%s given)", args[0].Type())
	}
	var items []object.Object
	enumerable.Enumerate(ctx, func(key, value object.Object) bool {
		items = append(items, value)
		return true
	})
	return object.NewList(items)
}

func String(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("string", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewString("")
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.String:
		return object.NewString(arg.Value())
	case io.Reader:
		bytes, err := io.ReadAll(arg)
		if err != nil {
			return object.NewError(err)
		}
		return object.NewString(string(bytes))
	default:
		if s, ok := arg.(fmt.Stringer); ok {
			return object.NewString(s.String())
		}
		return object.NewString(args[0].Inspect())
	}
}

func Type(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("type", 1, args); err != nil {
		return err
	}
	return object.NewString(string(args[0].Type()))
}

func Assert(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("assert", 1, 2, args); err != nil {
		return err
	}
	if !args[0].IsTruthy() {
		if len(args) == 2 {
			switch arg := args[1].(type) {
			case *object.String:
				return object.Errorf(arg.Value())
			default:
				return object.Errorf(args[1].Inspect())
			}
		}
		return object.Errorf("assertion failed")
	}
	return object.Nil
}

func Any(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("any", 1, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case *object.List:
		for _, val := range arg.Value() {
			if val.IsTruthy() {
				return object.True
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
			return object.True
		}
	default:
		return object.TypeErrorf("type error: any() argument must be a container (%s given)", args[0].Type())
	}
	return object.False
}

func All(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("all", 1, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case *object.List:
		for _, val := range arg.Value() {
			if !val.IsTruthy() {
				return object.False
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
			return object.False
		}
	default:
		return object.TypeErrorf("type error: all() argument must be a container (%s given)", args[0].Type())
	}
	return object.True
}

func Filter(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("filter", 2, args); err != nil {
		return err
	}
	fn, ok := args[1].(object.Callable)
	if !ok {
		return object.TypeErrorf("type error: filter() expected a callable (%s given)", args[1].Type())
	}
	var result []object.Object
	switch container := args[0].(type) {
	case *object.List:
		for _, val := range container.Value() {
			decision := fn.Call(ctx, val)
			if object.IsError(decision) {
				return decision
			}
			if decision.IsTruthy() {
				result = append(result, val)
			}
		}
	case object.Enumerable:
		var filterErr object.Object
		container.Enumerate(ctx, func(key, value object.Object) bool {
			decision := fn.Call(ctx, value)
			if object.IsError(decision) {
				filterErr = decision
				return false
			}
			if decision.IsTruthy() {
				result = append(result, value)
			}
			return true
		})
		if filterErr != nil {
			return filterErr
		}
	default:
		return object.TypeErrorf("type error: filter() argument must be a container (%s given)", args[0].Type())
	}
	return object.NewList(result)
}

func Bool(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("bool", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.False
	}
	if args[0].IsTruthy() {
		return object.True
	}
	return object.False
}

func Sorted(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("sorted", 1, 2, args); err != nil {
		return err
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
		return object.TypeErrorf("type error: sorted() unsupported argument (%s given)", arg.Type())
	}
	resultItems := make([]object.Object, len(items))
	copy(resultItems, items)
	if len(args) == 2 {
		fn, ok := args[1].(*object.Closure)
		if !ok {
			return object.TypeErrorf("type error: sorted() expected a function as the second argument (%s given)", args[1].Type())
		}
		callFunc, found := object.GetCallFunc(ctx)
		if !found {
			return object.EvalErrorf("eval error: context did not contain a call function")
		}
		var sortErr error
		sort.SliceStable(resultItems, func(i, j int) bool {
			result, err := callFunc(ctx, fn, []object.Object{resultItems[i], resultItems[j]})
			if err != nil {
				sortErr = err
				return false
			}
			return result.IsTruthy()
		})
		if sortErr != nil {
			return object.TypeErrorf("%s", sortErr.Error())
		}
	} else {
		if err := object.Sort(resultItems); err != nil {
			return err
		}
	}
	return object.NewList(resultItems)
}

func Reversed(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("reversed", 1, args); err != nil {
		return err
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.List:
		return arg.Reversed()
	case *object.String:
		return arg.Reversed()
	default:
		return object.TypeErrorf("type error: reversed() unsupported argument (%s given)",
			arg.Type())
	}
}

func GetAttr(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("getattr", 2, 3, args); err != nil {
		return err
	}
	attrName, err := object.AsString(args[1])
	if err != nil {
		return err
	}
	if attr, found := args[0].GetAttr(attrName); found {
		return attr
	}
	if len(args) == 3 {
		return args[2]
	}
	return object.TypeErrorf("type error: getattr() %s object has no attribute %q",
		args[0].Type(), attrName)
}

func Call(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("call", 1, 64, args); err != nil {
		return err
	}
	switch fn := args[0].(type) {
	case *object.Closure:
		callFunc, found := object.GetCallFunc(ctx)
		if !found {
			return object.EvalErrorf("eval error: context did not contain a call function")
		}
		result, err := callFunc(ctx, fn, args[1:])
		if err != nil {
			return object.Errorf(err.Error())
		}
		return result
	case object.Callable:
		return fn.Call(ctx, args[1:]...)
	default:
		return object.TypeErrorf("type error: call() unsupported argument (%s given)",
			args[0].Type())
	}
}

func Keys(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("keys", 1, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case *object.Map:
		return arg.Keys()
	case *object.List:
		return arg.Keys()
	case object.Enumerable:
		var keys []object.Object
		arg.Enumerate(ctx, func(key, value object.Object) bool {
			keys = append(keys, key)
			return true
		})
		return object.NewList(keys)
	default:
		return object.TypeErrorf("type error: keys() unsupported argument (%s given)", args[0].Type())
	}
}

func Byte(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("byte", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewByte(0)
	}
	switch obj := args[0].(type) {
	case *object.Int:
		return object.NewByte(byte(obj.Value()))
	case *object.Byte:
		return object.NewByte(obj.Value())
	case *object.Float:
		return object.NewByte(byte(obj.Value()))
	case *object.String:
		if i, err := strconv.ParseInt(obj.Value(), 0, 8); err == nil {
			return object.NewByte(byte(i))
		}
		return object.Errorf("value error: invalid literal for byte(): %q", obj.Value())
	default:
		return object.TypeErrorf("type error: byte() unsupported argument (%s given)", args[0].Type())
	}
}

func Int(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("int", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewInt(0)
	}
	switch obj := args[0].(type) {
	case *object.Int:
		return obj
	case *object.Byte:
		return object.NewInt(int64(obj.Value()))
	case *object.Float:
		return object.NewInt(int64(obj.Value()))
	case *object.String:
		if i, err := strconv.ParseInt(obj.Value(), 0, 64); err == nil {
			return object.NewInt(i)
		}
		return object.Errorf("value error: invalid literal for int(): %q", obj.Value())
	default:
		return object.TypeErrorf("type error: int() unsupported argument (%s given)", args[0].Type())
	}
}

func Float(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("float", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewFloat(0)
	}
	switch obj := args[0].(type) {
	case *object.Int:
		return object.NewFloat(float64(obj.Value()))
	case *object.Byte:
		return object.NewFloat(float64(obj.Value()))
	case *object.Float:
		return obj
	case *object.String:
		if f, err := strconv.ParseFloat(obj.Value(), 64); err == nil {
			return object.NewFloat(f)
		}
		return object.Errorf("value error: invalid literal for float(): %q", obj.Value())
	default:
		return object.TypeErrorf("type error: float() unsupported argument (%s given)", args[0].Type())
	}
}

func Coalesce(ctx context.Context, args ...object.Object) object.Object {
	if err := object.RequireRange("coalesce", 0, 64, args); err != nil {
		return err
	}
	for _, arg := range args {
		if arg != object.Nil {
			return arg
		}
	}
	return object.Nil
}

func Chunk(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("chunk", 2, args); err != nil {
		return err
	}
	list, ok := args[0].(*object.List)
	if !ok {
		return object.TypeErrorf("type error: chunk() expected a list (%s given)", args[0].Type())
	}
	listSize := int64(list.Size())
	chunkSizeObj, ok := args[1].(*object.Int)
	if !ok {
		return object.TypeErrorf("type error: chunk() expected an int (%s given)", args[1].Type())
	}
	chunkSize := chunkSizeObj.Value()
	if chunkSize <= 0 {
		return object.Errorf("value error: chunk() size must be > 0 (%d given)", chunkSize)
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
	return object.NewList(chunks)
}

func IsHashable(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("is_hashable", 1, args); err != nil {
		return err
	}
	_, ok := args[0].(object.Hashable)
	return object.NewBool(ok)
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
		"delete":   object.NewBuiltin("delete", Delete),
		"encode":   object.NewBuiltin("encode", Encode),
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
