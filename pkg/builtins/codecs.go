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
	"io"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

var (
	mutex  sync.RWMutex
	codecs = map[string]*Codec{}

	jsonCodec = &Codec{Encode: encodeJSON, Decode: decodeJSON}
	csvCodec  = &Codec{Encode: encodeCsv, Decode: decodeCsv}
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
	RegisterCodec("json", jsonCodec)
	RegisterCodec("csv", csvCodec)
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
	// Check before encoding, so that a value whose encoding is too large,
	// such as a list that holds one string many times, is never encoded
	if err := object.CheckFormatSize(ctx, "encode", args[0]); err != nil {
		return nil, err
	}
	result, err := codec.Encode(ctx, args[0])
	if err != nil {
		return nil, err
	}
	switch result := result.(type) {
	case *object.String:
		err = object.CheckBytesSize(ctx, "encode", len(result.Value()))
	case *object.Bytes:
		err = object.CheckBytesSize(ctx, "encode", len(result.Value()))
	}
	if err != nil {
		return nil, err
	}
	return result, nil
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
	nativeObject, err := object.ToJSONValue(ctx, obj)
	if err != nil {
		return nil, err
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
	result, err := codec.Decode(ctx, args[0])
	if err != nil {
		return nil, err
	}
	// The json and csv codecs charge the budget as they build the value, so
	// they stop at the limit. Check the result of any other codec, such as
	// one registered by the host, after the fact.
	if codec != jsonCodec && codec != csvCodec {
		if err := checkDecoded(newDecodeBudget(ctx), result, 0); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// decodeBudget bounds the total number of elements in one decoded value. It
// counts every list element and map entry at any depth, as the conversion of
// a value to Go counts them, against the list length limit in ctx. So a value
// that decodes can be converted to Go, for example as a script result.
// Decoding can produce a list element for every two bytes of input, and each
// element takes far more memory than two bytes.
type decodeBudget struct {
	ctx       context.Context
	remaining int64
	limit     int64
	// unchecked counts the elements taken since ctx was last checked
	unchecked int
}

func newDecodeBudget(ctx context.Context) *decodeBudget {
	limit := object.GetLimits(ctx).MaxListLength
	return &decodeBudget{ctx: ctx, remaining: limit, limit: limit}
}

// take charges n elements to the budget. It returns a value error if the
// budget has fewer than n elements left. Call it before building the
// elements, so a value over the limit is never built. It also returns
// ctx.Err() if ctx is canceled.
func (b *decodeBudget) take(n int) error {
	if uint64(n) > uint64(b.remaining) {
		return object.ValueErrorf("decoded value too large (exceeds limit of %d elements)", b.limit)
	}
	b.remaining -= int64(n)
	b.unchecked += n
	if b.unchecked >= 1024 {
		b.unchecked = 0
		return b.ctx.Err()
	}
	return nil
}

// checkDecoded charges the lists and maps in obj to b. depth is the number of
// lists and maps that enclose obj. It returns object.ErrNestingDepth for a
// list or map nested more deeply than object.MaxNestingDepth, which includes
// one that contains itself.
func checkDecoded(b *decodeBudget, obj object.Object, depth int) error {
	switch obj := obj.(type) {
	case *object.List:
		if err := b.enter(len(obj.Value()), depth); err != nil {
			return err
		}
		for _, item := range obj.Value() {
			if err := checkDecoded(b, item, depth+1); err != nil {
				return err
			}
		}
	case *object.Map:
		if err := b.enter(len(obj.Value()), depth); err != nil {
			return err
		}
		for _, value := range obj.Value() {
			if err := checkDecoded(b, value, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// enter charges a list or map of n elements, enclosed by depth lists and
// maps, to the budget. It returns object.ErrNestingDepth if depth is already
// object.MaxNestingDepth, as converting a value from Go does.
func (b *decodeBudget) enter(n, depth int) error {
	if depth >= object.MaxNestingDepth {
		return object.ErrNestingDepth
	}
	return b.take(n)
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
	// Check the syntax before building anything, so that invalid JSON fails
	// with the error json.Unmarshal returns. Unmarshal checks the syntax
	// before it decodes, so on invalid input it builds nothing.
	if !json.Valid(data) {
		var discard any
		if err := json.Unmarshal(data, &discard); err != nil {
			return nil, err
		}
		return nil, errors.New("invalid JSON")
	}
	// Build the Risor value straight from the input, charging each element
	// to the budget before it is built, so that a value over the limit stops
	// at the limit and no Go copy of the value is built.
	d := jsonDecoder{data: data, budget: newDecodeBudget(ctx)}
	return d.value(0)
}

// jsonDecoder builds a Risor value from JSON that json.Valid accepts, so it
// does not check the syntax. Values decode as json.Unmarshal decodes them
// into an any: numbers are floats, and a repeated object key keeps its last
// value.
type jsonDecoder struct {
	data   []byte
	pos    int
	budget *decodeBudget
}

func (d *jsonDecoder) skipSpace() {
	for d.pos < len(d.data) {
		switch d.data[d.pos] {
		case ' ', '\t', '\n', '\r':
			d.pos++
		default:
			return
		}
	}
}

// value decodes the value at d.pos. depth is the number of arrays and
// objects that enclose it.
func (d *jsonDecoder) value(depth int) (object.Object, error) {
	d.skipSpace()
	switch d.data[d.pos] {
	case '[', '{':
		if depth >= object.MaxNestingDepth {
			return nil, object.ErrNestingDepth
		}
		if d.data[d.pos] == '[' {
			return d.array(depth)
		}
		return d.object(depth)
	case '"':
		s, err := d.string()
		if err != nil {
			return nil, err
		}
		return object.NewString(s), nil
	case 't':
		d.pos += len("true")
		return object.True, nil
	case 'f':
		d.pos += len("false")
		return object.False, nil
	case 'n':
		d.pos += len("null")
		return object.Nil, nil
	}
	return d.number()
}

// array decodes the array at d.pos. Each element is charged to the budget
// before it is decoded.
func (d *jsonDecoder) array(depth int) (object.Object, error) {
	d.pos++ // [
	items := []object.Object{}
	d.skipSpace()
	if d.data[d.pos] == ']' {
		d.pos++
		return object.NewList(items), nil
	}
	for {
		if err := d.budget.take(1); err != nil {
			return nil, err
		}
		item, err := d.value(depth + 1)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		d.skipSpace()
		c := d.data[d.pos] // , or ]
		d.pos++
		if c == ']' {
			return object.NewList(items), nil
		}
	}
}

// object decodes the object at d.pos. Each member is charged to the budget
// before its value is decoded, including a member whose key repeats.
func (d *jsonDecoder) object(depth int) (object.Object, error) {
	d.pos++ // {
	items := map[string]object.Object{}
	d.skipSpace()
	if d.data[d.pos] == '}' {
		d.pos++
		return object.NewMap(items), nil
	}
	for {
		d.skipSpace()
		key, err := d.string()
		if err != nil {
			return nil, err
		}
		d.skipSpace()
		d.pos++ // :
		if err := d.budget.take(1); err != nil {
			return nil, err
		}
		value, err := d.value(depth + 1)
		if err != nil {
			return nil, err
		}
		items[key] = value
		d.skipSpace()
		c := d.data[d.pos] // , or }
		d.pos++
		if c == '}' {
			return object.NewMap(items), nil
		}
	}
}

// string decodes the string literal at d.pos.
func (d *jsonDecoder) string() (string, error) {
	start := d.pos
	escaped := false
	d.pos++ // "
	for d.data[d.pos] != '"' {
		if d.data[d.pos] == '\\' {
			escaped = true
			d.pos++ // the escaped byte
		}
		d.pos++
	}
	d.pos++ // "
	literal := d.data[start:d.pos]
	if content := literal[1 : len(literal)-1]; !escaped && utf8.Valid(content) {
		return string(content), nil
	}
	// Let encoding/json unquote the rest, so that escapes and invalid UTF-8
	// decode exactly as json.Unmarshal decodes them
	var s string
	if err := json.Unmarshal(literal, &s); err != nil {
		return "", err
	}
	return s, nil
}

// number decodes the number literal at d.pos as a float. For a number out of
// range, it returns the error json.Unmarshal returns, except that the error
// names at most jsonNumberMessageSize bytes of the literal.
func (d *jsonDecoder) number() (object.Object, error) {
	start := d.pos
	for d.pos < len(d.data) && isJSONNumberByte(d.data[d.pos]) {
		d.pos++
	}
	literal := string(d.data[start:d.pos])
	f, err := strconv.ParseFloat(literal, 64)
	if err != nil {
		if len(literal) > jsonNumberMessageSize {
			literal = literal[:jsonNumberMessageSize] + "..."
		}
		return nil, &json.UnmarshalTypeError{
			Value:  "number " + literal,
			Type:   reflect.TypeFor[float64](),
			Offset: int64(d.pos),
		}
	}
	return object.NewFloat(f), nil
}

// jsonNumberMessageSize is the number of bytes of a number literal that an
// error message names, as object.QuoteMessage bounds an input in a message.
// A number literal can be as long as the input.
const jsonNumberMessageSize = 64

func isJSONNumberByte(c byte) bool {
	return (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E'
}

// decodeCsv returns a list with a list of strings for each record. The
// budget is charged for each record and each of its fields before they are
// built.
func decodeCsv(ctx context.Context, obj object.Object) (object.Object, error) {
	data, err := object.AsBytes(obj)
	if err != nil {
		return nil, err
	}
	budget := newDecodeBudget(ctx)
	reader := csv.NewReader(bytes.NewReader(data))
	records := []object.Object{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := budget.take(1 + len(record)); err != nil {
			return nil, err
		}
		fields := make([]object.Object, len(record))
		for i, field := range record {
			fields[i] = object.NewString(field)
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
