// Package builtins defines a default set of built-in functions.
package builtins

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"
	"strconv"
	"unicode"

	"github.com/risor-io/risor/arg"
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/object"
)

func Len(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("len", 1, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case object.Container:
		return arg.Len()
	case *object.Buffer:
		return object.NewInt(int64(arg.Value().Len()))
	default:
		return object.TypeErrorf("type error: len() unsupported argument (%s given)", args[0].Type())
	}
}

func Sprintf(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("sprintf", 1, 64, args); err != nil {
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
	if err := arg.Require("delete", 2, args); err != nil {
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

func Set(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("set", 0, 1, args); err != nil {
		return err
	}
	set := object.NewSetWithSize(0)
	if len(args) == 0 {
		return set
	}
	arg := args[0]
	iter, err := object.AsIterator(arg)
	if err != nil {
		return err
	}
	for {
		val, ok := iter.Next(ctx)
		if !ok {
			break
		}
		if res := set.Add(val); object.IsError(res) {
			return res
		}
	}
	return set
}

func List(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("list", 0, 1, args); err != nil {
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
	arg := args[0]
	iter, err := object.AsIterator(arg)
	if err != nil {
		return err
	}
	var items []object.Object
	for {
		val, ok := iter.Next(ctx)
		if !ok {
			break
		}
		items = append(items, val)
	}
	return object.NewList(items)
}

func Map(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("map", 0, 1, args); err != nil {
		return err
	}
	result := object.NewMap(nil)
	if len(args) == 0 {
		return result
	}
	arg := args[0]
	list, ok := arg.(*object.List)
	if ok {
		for _, obj := range list.Value() {
			subListObj, ok := obj.(*object.List)
			if !ok || len(subListObj.Value()) != 2 {
				return object.Errorf("value error: map() received a list with an unsupported structure")
			}
			subList := subListObj.Value()
			key, ok := subList[0].(*object.String)
			if !ok {
				return object.Errorf("value error: map() received a list with an unsupported structure")
			}
			result.Set(key.Value(), subList[1])
		}
		return result
	}
	iter, err := object.AsIterator(arg)
	if err != nil {
		return err
	}
	for {
		if _, ok := iter.Next(ctx); !ok {
			break
		}
		entry, _ := iter.Entry()
		k, v := entry.Key(), entry.Value()
		switch k := k.(type) {
		case *object.String:
			result.Set(k.Value(), v)
		default:
			result.Set(k.Inspect(), v)
		}
	}
	return result
}

func String(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("string", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewString("")
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.Buffer:
		return object.NewString(arg.Value().String())
	case *object.ByteSlice:
		return object.NewString(string(arg.Value()))
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

func FloatSlice(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("float_slice", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewFloatSlice(nil)
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.FloatSlice:
		return arg.Clone()
	case *object.Int:
		val := arg.Value()
		return object.NewFloatSlice(make([]float64, val))
	case *object.List:
		items := arg.Value()
		floats := make([]float64, len(items))
		for i, item := range items {
			switch item := item.(type) {
			case *object.Byte:
				floats[i] = float64(item.Value())
			case *object.Int:
				floats[i] = float64(item.Value())
			case *object.Float:
				floats[i] = item.Value()
			default:
				return object.TypeErrorf(
					"type error: float_slice() list item unsupported (%s given)",
					item.Type())
			}
		}
		return object.NewFloatSlice(floats)
	default:
		return object.TypeErrorf("type error: float_slice() unsupported argument (%s given)",
			args[0].Type())
	}
}

func ByteSlice(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("byte_slice", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewByteSlice(nil)
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.Buffer:
		return object.NewByteSlice(arg.Value().Bytes())
	case *object.ByteSlice:
		return arg.Clone()
	case *object.String:
		return object.NewByteSlice([]byte(arg.Value()))
	case *object.Int:
		val := arg.Value()
		return object.NewByteSlice(make([]byte, val))
	case *object.List:
		items := arg.Value()
		bytes := make([]byte, len(items))
		for i, item := range items {
			switch item := item.(type) {
			case *object.Int:
				bytes[i] = byte(item.Value())
			case *object.Byte:
				bytes[i] = item.Value()
			default:
				return object.TypeErrorf(
					"type error: byte_slice() list item unsupported (%s given)",
					item.Type())
			}
		}
		return object.NewByteSlice(bytes)
	default:
		return object.TypeErrorf("type error: byte_slice() unsupported argument (%s given)",
			args[0].Type())
	}
}

func Buffer(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("buffer", 0, 1, args); err != nil {
		return err
	}
	if len(args) == 0 {
		return object.NewBuffer(new(bytes.Buffer))
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.Buffer:
		return object.NewBufferFromBytes(arg.Value().Bytes())
	case *object.ByteSlice:
		return object.NewBufferFromBytes(arg.Value())
	case *object.String:
		return object.NewBufferFromBytes([]byte(arg.Value()))
	case *object.Int:
		// Special case: treat the value as the size to allocate
		val := arg.Value()
		return object.NewBufferFromBytes(make([]byte, val))
	case io.Reader:
		bytes, err := io.ReadAll(arg)
		if err != nil {
			return object.NewError(err)
		}
		return object.NewBufferFromBytes(bytes)
	default:
		return object.TypeErrorf("type error: buffer() unsupported argument (%s given)",
			args[0].Type())
	}
}

func Type(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("type", 1, args); err != nil {
		return err
	}
	return object.NewString(string(args[0].Type()))
}

func Assert(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("assert", 1, 2, args); err != nil {
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
	if err := arg.Require("any", 1, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case *object.List:
		for _, val := range arg.Value() {
			if val.IsTruthy() {
				return object.True
			}
		}
	case *object.Set:
		for _, val := range arg.Value() {
			if val.IsTruthy() {
				return object.True
			}
		}
	case *object.ByteSlice:
		for _, val := range arg.Value() {
			if val != 0 {
				return object.True
			}
		}
	case *object.Buffer:
		for _, val := range arg.Value().Bytes() {
			if val != 0 {
				return object.True
			}
		}
	case object.Iterable:
		iter := arg.Iter()
		for {
			val, ok := iter.Next(ctx)
			if !ok {
				break
			}
			if val.IsTruthy() {
				return object.True
			}
		}
	case object.Iterator:
		for {
			val, ok := arg.Next(ctx)
			if !ok {
				break
			}
			if val.IsTruthy() {
				return object.True
			}
		}
	default:
		return object.TypeErrorf("type error: any() argument must be a container (%s given)", args[0].Type())
	}
	return object.False
}

func All(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("all", 1, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case *object.List:
		for _, val := range arg.Value() {
			if !val.IsTruthy() {
				return object.False
			}
		}
	case *object.Set:
		for _, val := range arg.Value() {
			if !val.IsTruthy() {
				return object.False
			}
		}
	case *object.ByteSlice:
		for _, val := range arg.Value() {
			if val == 0 {
				return object.False
			}
		}
	case *object.Buffer:
		for _, val := range arg.Value().Bytes() {
			if val == 0 {
				return object.False
			}
		}
	case object.Iterable:
		iter := arg.Iter()
		for {
			val, ok := iter.Next(ctx)
			if !ok {
				break
			}
			if !val.IsTruthy() {
				return object.False
			}
		}
	case object.Iterator:
		for {
			val, ok := arg.Next(ctx)
			if !ok {
				break
			}
			if !val.IsTruthy() {
				return object.False
			}
		}
	default:
		return object.TypeErrorf("type error: all() argument must be a container (%s given)", args[0].Type())
	}
	return object.True
}

func Bool(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("bool", 0, 1, args); err != nil {
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
	if err := arg.RequireRange("sorted", 1, 2, args); err != nil {
		return err
	}
	arg := args[0]
	var items []object.Object
	switch arg := arg.(type) {
	case *object.List:
		items = arg.Value()
	case *object.Map:
		items = arg.Keys().Value()
	case *object.Set:
		items = arg.List().Value()
	case *object.String:
		items = arg.Runes()
	case *object.ByteSlice:
		items = arg.Integers()
	default:
		return object.TypeErrorf("type error: sorted() unsupported argument (%s given)", arg.Type())
	}
	resultItems := make([]object.Object, len(items))
	copy(resultItems, items)
	if len(args) == 2 {
		fn, ok := args[1].(*object.Function)
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
			return object.TypeErrorf(sortErr.Error())
		}
	} else {
		if err := object.Sort(resultItems); err != nil {
			return err
		}
	}
	return object.NewList(resultItems)
}

func Reversed(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("reversed", 1, args); err != nil {
		return err
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.List:
		return arg.Reversed()
	case *object.String:
		return arg.Reversed()
	case *object.ByteSlice:
		return arg.Reversed()
	default:
		return object.TypeErrorf("type error: reversed() unsupported argument (%s given)",
			arg.Type())
	}
}

func GetAttr(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("getattr", 2, 3, args); err != nil {
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
	if err := arg.RequireRange("call", 1, 64, args); err != nil {
		return err
	}
	switch fn := args[0].(type) {
	case *object.Function:
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
	if err := arg.Require("keys", 1, args); err != nil {
		return err
	}
	arg := args[0]
	switch arg := arg.(type) {
	case *object.Map:
		return arg.Keys()
	case *object.List:
		return arg.Keys()
	case *object.Set:
		return arg.List()
	case object.Iterable:
		return iterKeys(ctx, arg.Iter())
	case object.Iterator:
		return iterKeys(ctx, arg)
	default:
		return object.TypeErrorf("type error: keys() unsupported argument (%s given)", arg.Type())
	}
}

func iterKeys(ctx context.Context, iter object.Iterator) object.Object {
	var keys []object.Object
	for {
		if _, ok := iter.Next(ctx); !ok {
			break
		}
		entry, _ := iter.Entry()
		keys = append(keys, entry.Key())
	}
	return object.NewList(keys)
}

func Byte(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("byte", 0, 1, args); err != nil {
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
	if err := arg.RequireRange("int", 0, 1, args); err != nil {
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
	if err := arg.RequireRange("float", 0, 1, args); err != nil {
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

func Ord(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("ord", 1, args); err != nil {
		return err
	}
	switch obj := args[0].(type) {
	case *object.String:
		runes := []rune(obj.Value())
		if len(runes) != 1 {
			return object.Errorf("value error: ord() expected a character, but string of length %d found", len(obj.Value()))
		}
		return object.NewInt(int64(runes[0]))
	}
	return object.Errorf("value error: ord() expected a string of length 1 (%s given)", args[0].Type())
}

func Chr(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("chr", 1, args); err != nil {
		return err
	}
	switch obj := args[0].(type) {
	case *object.Int:
		v := obj.Value()
		if v < 0 {
			return object.Errorf("value error: chr() argument out of range (%d given)", v)
		}
		if v > unicode.MaxRune {
			return object.Errorf("value error: chr() argument out of range (%d given)", v)
		}
		return object.NewString(string(rune(v)))
	}
	return object.TypeErrorf("type error: chr() expected an int (%s given)", args[0].Type())
}

func Error(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("error", 1, 64, args); err != nil {
		return err
	}
	switch arg := args[0].(type) {
	case *object.Error:
		// Return a copy of the error that has its raised flag set.
		// It's possible an error was passed that did not have raised set.
		return object.NewError(arg.Value()).WithRaised(true)
	case *object.String:
		msg := arg
		var msgArgs []interface{}
		for _, arg := range args[1:] {
			msgArgs = append(msgArgs, arg.Interface())
		}
		return object.Errorf(msg.Value(), msgArgs...)
	default:
		return object.TypeErrorf("type error: error() expected a string or error (%s given)",
			args[0].Type())
	}
}

func Try(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("try", 1, 64, args); err != nil {
		return err
	}
	var lastErr *object.Error
	try := func(arg object.Object) (object.Object, error) {
		switch obj := arg.(type) {
		case *object.Function:
			var callArgs []object.Object
			if len(obj.Parameters()) > 0 && lastErr != nil {
				callArgs = append(callArgs, lastErr)
			}
			callFunc, found := object.GetCallFunc(ctx)
			if !found {
				return nil, errz.EvalErrorf("eval error: context did not contain a call function")
			}
			return callFunc(ctx, obj, callArgs)
		case object.Callable:
			var callArgs []object.Object
			if lastErr != nil {
				callArgs = append(callArgs, lastErr)
			}
			result := obj.Call(ctx, callArgs...)
			if err, ok := result.(*object.Error); ok && err.IsRaised() {
				return nil, err
			}
			return result, nil
		default:
			return obj, nil
		}
	}
	for _, arg := range args {
		result, err := try(arg)
		if err != nil {
			var tmpErr errz.Error
			if errors.As(err, &tmpErr) && tmpErr.IsFatal() {
				// This indicates an unrecoverable evaluation error
				return object.NewError(err)
			}
			lastErr = object.NewError(err).WithRaised(false)
			continue
		}
		return result
	}
	return object.Nil
}

func Iter(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("iter", 1, args); err != nil {
		return err
	}
	container, ok := args[0].(object.Iterable)
	if !ok {
		return object.TypeErrorf("type error: iter() expected an iterable (%s given)", args[0].Type())
	}
	return container.Iter()
}

func Hash(ctx context.Context, args ...object.Object) object.Object {
	nArgs := len(args)
	if nArgs < 1 || nArgs > 2 {
		return object.ArgsErrorf("args error: hash() takes 1 or 2 arguments (%d given)", nArgs)
	}
	data, err := object.AsBytes(args[0])
	if err != nil {
		return err
	}
	alg := "sha256"
	if nArgs == 2 {
		var err *object.Error
		alg, err = object.AsString(args[1])
		if err != nil {
			return err
		}
	}
	var h hash.Hash
	// Hash `data` using the algorithm specified by `alg` and return the result as a byte_slice.
	// Support `alg` values: sha256, sha512, sha1, md5
	switch alg {
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	case "sha1":
		h = sha1.New()
	case "md5":
		h = md5.New()
	default:
		return object.Errorf("value error: hash() algorithm must be one of sha256, sha512, sha1, md5")
	}
	h.Write(data)
	return object.NewByteSlice(h.Sum(nil))
}

func Make(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("make", 1, 2, args); err != nil {
		return err
	}
	typ := args[0]
	size := 0
	if len(args) == 2 {
		switch arg := args[1].(type) {
		case *object.Int:
			size = int(arg.Value())
		default:
			return object.TypeErrorf("type error: make() expected an int (%s given)", arg.Type())
		}
	}
	if size < 0 {
		return object.Errorf("value error: make() size must be >= 0 (%d given)", size)
	}
	switch typ := typ.(type) {
	case *object.List:
		return object.NewList(make([]object.Object, 0, size))
	case *object.Map:
		return object.NewMap(make(map[string]object.Object, size))
	case *object.Set:
		return object.NewSetWithSize(size)
	case *object.Builtin:
		name := typ.Name()
		switch name {
		case "list":
			return object.NewList(make([]object.Object, 0, size))
		case "map":
			return object.NewMap(make(map[string]object.Object, size))
		case "set":
			return object.NewSetWithSize(size)
		default:
			return object.Errorf("value error: make() unsupported type name (%s given)", name)
		}
	default:
		return object.TypeErrorf("type error: make() unsupported type (%s given)", typ.Type())
	}
}

func Coalesce(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("coalesce", 0, 64, args); err != nil {
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
	if err := arg.Require("chunk", 2, args); err != nil {
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
	if err := arg.Require("is_hashable", 1, args); err != nil {
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
		"call":     object.NewBuiltin("call", Call),
		"chr":      object.NewBuiltin("chr", Chr),
		"chunk":    object.NewBuiltin("chunk", Chunk),
		"coalesce": object.NewBuiltin("coalesce", Coalesce),
		"decode":   object.NewBuiltin("decode", Decode),
		"delete":   object.NewBuiltin("delete", Delete),
		"encode":   object.NewBuiltin("encode", Encode),
		"error":    object.NewBuiltin("error", Error),
		"float":    object.NewBuiltin("float", Float),
		"getattr":  object.NewBuiltin("getattr", GetAttr),
		"hash":     object.NewBuiltin("hash", Hash),
		"int":      object.NewBuiltin("int", Int),
		"keys":     object.NewBuiltin("keys", Keys),
		"len":      object.NewBuiltin("len", Len),
		"list":     object.NewBuiltin("list", List),
		"map":      object.NewBuiltin("map", Map),
		"ord":      object.NewBuiltin("ord", Ord),
		"reversed": object.NewBuiltin("reversed", Reversed),
		"set":      object.NewBuiltin("set", Set),
		"sorted":   object.NewBuiltin("sorted", Sorted),
		"sprintf":  object.NewBuiltin("sprintf", Sprintf),
		"string":   object.NewBuiltin("string", String),
		"try":      object.NewBuiltin("try", Try),
		"type":     object.NewBuiltin("type", Type),
	}
}
