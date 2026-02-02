package builtins

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

var (
	mutex  sync.RWMutex
	codecs = map[string]*Codec{}
)

// Codec contains an Encode and a Decode function
type Codec struct {
	Encode func(context.Context, object.Object) (object.Object, error)
	Decode func(context.Context, object.Object) (object.Object, error)
}

func init() {
	RegisterCodec("base64", &Codec{Encode: encodeBase64, Decode: decodeBase64})
	RegisterCodec("base32", &Codec{Encode: encodeBase32, Decode: decodeBase32})
	RegisterCodec("hex", &Codec{Encode: encodeHex, Decode: decodeHex})
	RegisterCodec("json", &Codec{Encode: encodeJSON, Decode: decodeJSON})
	RegisterCodec("csv", &Codec{Encode: encodeCsv, Decode: decodeCsv})
	RegisterCodec("urlquery", &Codec{Encode: encodeUrlQuery, Decode: decodeUrlQuery})
}

// RegisterCodec registers a new codec
func RegisterCodec(name string, codec *Codec) error {
	mutex.Lock()
	defer mutex.Unlock()

	if _, exists := codecs[name]; exists {
		return errors.New("codec already registered: " + name)
	}
	codecs[name] = codec
	return nil
}

// GetCodec retrieves a codec by its name
func GetCodec(name string) (*Codec, error) {
	mutex.RLock()
	defer mutex.RUnlock()

	codec, exists := codecs[name]
	if !exists {
		return nil, errors.New("codec not found: " + name)
	}
	return codec, nil
}

func Encode(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("encode: expected 2 arguments, got %d", len(args))
	}
	encoding, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}
	codec, codecErr := GetCodec(encoding)
	if codecErr != nil {
		return nil, codecErr
	}
	return codec.Encode(ctx, args[0])
}

func encodeBase64(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	return object.NewString(base64.StdEncoding.EncodeToString(data)), nil
}

func encodeBase32(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	return object.NewString(base32.StdEncoding.EncodeToString(data)), nil
}

func encodeHex(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	return object.NewString(hex.EncodeToString(data)), nil
}

func encodeJSON(ctx context.Context, obj object.Object) (object.Object, error) {
	nativeObject := obj.Interface()
	if nativeObject == nil {
		return nil, object.ValueErrorf("encode() does not support %T", obj)
	}
	jsonBytes, err := json.Marshal(nativeObject)
	if err != nil {
		return nil, err
	}
	return object.NewString(string(jsonBytes)), nil
}

func encodeUrlQuery(ctx context.Context, obj object.Object) (object.Object, error) {
	str, err := object.AsString(obj)
	if err != nil {
		return nil, err
	}
	return object.NewString(url.QueryEscape(str)), nil
}

func asStringList(list *object.List) ([]string, error) {
	items := list.Value()
	result := make([]string, len(items))
	for i, item := range items {
		switch item := item.(type) {
		case *object.String:
			result[i] = item.Value()
		default:
			result[i] = item.Inspect()
		}
	}
	return result, nil
}

func csvStringListFromMap(m *object.Map, keys []string) ([]string, error) {
	items := m.Value()
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		v, ok := items[key]
		if !ok {
			result = append(result, "")
			continue
		}
		switch v := v.(type) {
		case *object.String:
			result = append(result, v.Value())
		default:
			result = append(result, v.Inspect())
		}
	}
	return result, nil
}

func encodeCsv(ctx context.Context, obj object.Object) (object.Object, error) {
	list, ok := obj.(*object.List)
	if !ok {
		return nil, object.TypeErrorf("encode(obj, \"csv\") requires a list (got %s)", obj.Type())
	}
	items := list.Value()
	if len(items) == 0 {
		return nil, object.ValueErrorf("encode(obj, \"csv\") requires a non-empty List")
	}
	records := make([][]string, 0, len(items))
	switch outer := items[0].(type) {
	case *object.List:
		for _, item := range items {
			innerList, ok := item.(*object.List)
			if !ok {
				return nil, object.ValueErrorf("encode(obj, \"csv\") requires a list of lists (got %s)", item.Type())
			}
			strList, err := asStringList(innerList)
			if err != nil {
				return nil, err
			}
			records = append(records, strList)
		}
	case *object.Map:
		keys := outer.StringKeys()
		sort.Strings(keys)
		records = append(records, keys)
		for _, item := range items {
			innerMap, ok := item.(*object.Map)
			if !ok {
				return nil, object.ValueErrorf("encode(obj, \"csv\") requires a list of maps (got %s)", item.Type())
			}
			strList, err := csvStringListFromMap(innerMap, keys)
			if err != nil {
				return nil, err
			}
			records = append(records, strList)
		}
	default:
		return nil, object.ValueErrorf("encode(obj, \"csv\") requires a list of lists or maps (got list of %s)", items[0].Type())
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.WriteAll(records); err != nil {
		return nil, err
	}
	return object.NewString(buf.String()), nil
}

func Decode(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("decode: expected 2 arguments, got %d", len(args))
	}
	encoding, err := object.AsString(args[1])
	if err != nil {
		return nil, err
	}
	codec, codecErr := GetCodec(encoding)
	if codecErr != nil {
		return nil, codecErr
	}
	return codec.Decode(ctx, args[0])
}

func decodeBase64(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	enc := base64.StdEncoding
	dst := make([]byte, enc.DecodedLen(len(data)))
	count, decodeErr := enc.Decode(dst, data)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return object.NewBytes(dst[:count]), nil
}

func decodeBase32(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	enc := base32.StdEncoding
	dst := make([]byte, enc.DecodedLen(len(data)))
	count, decodeErr := enc.Decode(dst, data)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return object.NewBytes(dst[:count]), nil
}

func decodeHex(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	dst := make([]byte, hex.DecodedLen(len(data)))
	count, decodeErr := hex.Decode(dst, data)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return object.NewBytes(dst[:count]), nil
}

func decodeJSON(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	var result interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}
	return object.FromGoType(result), nil
}

func decodeCsv(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(data))
	recs, ioErr := reader.ReadAll()
	if ioErr != nil {
		return nil, ioErr
	}
	records := make([]object.Object, 0, len(recs))
	for _, record := range recs {
		fields := make([]object.Object, 0, len(record))
		for _, field := range record {
			fields = append(fields, object.NewString(field))
		}
		records = append(records, object.NewList(fields))
	}
	return object.NewList(records), nil
}

// decodeUrlQuery wraps url.QueryUnescape
func decodeUrlQuery(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsString(obj)
	if err != nil {
		return nil, err
	}
	result, escErr := url.QueryUnescape(data)
	if escErr != nil {
		return nil, escErr
	}
	return object.NewString(result), nil
}
