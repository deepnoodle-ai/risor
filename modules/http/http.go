package http

import (
	"context"
	"net/http"

	"github.com/risor-io/risor/object"
)

func NewHttpRequest(ctx context.Context, args ...object.Object) object.Object {
	numArgs := len(args)
	if numArgs < 1 || numArgs > 2 {
		return object.NewArgsRangeError("fetch", 1, 2, numArgs)
	}
	urlArg, argErr := object.AsString(args[0])
	if argErr != nil {
		return argErr
	}
	var errObj *object.Error
	var params *object.Map
	if numArgs == 2 {
		params, errObj = object.AsMap(args[1])
		if errObj != nil {
			return errObj
		}
	}
	req, errObj := NewRequestFromParams(urlArg, params)
	if errObj != nil {
		return errObj
	}

	return req
}

func MethodCmd(method string) object.BuiltinFunction {
	return func(ctx context.Context, args ...object.Object) object.Object {
		numArgs := len(args)
		if numArgs < 1 || numArgs > 3 {
			return object.NewArgsRangeError("fetch", 1, 3, numArgs)
		}
		urlArg, argErr := object.AsString(args[0])
		if argErr != nil {
			return argErr
		}

		var errObj *object.Error

		params := object.NewMap(map[string]object.Object{
			"method": object.NewString(method),
		})

		if numArgs > 1 {
			headers, errObj := object.AsMap(args[1])
			if errObj != nil {
				return errObj
			}
			params.Set("headers", headers)
		}

		if numArgs > 2 {
			var key string
			switch method {
			case http.MethodGet, http.MethodHead, http.MethodDelete:
				key = "params"
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				key = "body"
			}
			params.Set(key, args[2])
		}

		req, errObj := NewRequestFromParams(urlArg, params)
		if errObj != nil {
			return errObj
		}

		return req
	}
}

func Builtins() map[string]object.Object {
	return map[string]object.Object{
		"fetch": object.NewBuiltin("fetch", Fetch),
	}
}

func Module() *object.Module {
	return object.NewBuiltinsModule("http", map[string]object.Object{
		"delete":  object.NewBuiltin("http.delete", MethodCmd(http.MethodDelete)),
		"get":     object.NewBuiltin("http.get", MethodCmd(http.MethodGet)),
		"head":    object.NewBuiltin("http.head", MethodCmd(http.MethodHead)),
		"patch":   object.NewBuiltin("http.patch", MethodCmd(http.MethodPatch)),
		"post":    object.NewBuiltin("http.post", MethodCmd(http.MethodPost)),
		"put":     object.NewBuiltin("http.put", MethodCmd(http.MethodPut)),
		"request": object.NewBuiltin("http.request", NewHttpRequest),
	})
}
