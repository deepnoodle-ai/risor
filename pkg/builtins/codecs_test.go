package builtins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestBinaryCodecs(t *testing.T) {
	codecs := []string{
		"base64",
		"base32",
		"hex",
	}
	ctx := context.Background()
	value := "Farfalle"
	for _, codec := range codecs {
		codecName := object.NewString(codec)
		t.Run(codec, func(t *testing.T) {
			encoded, err := Encode(ctx, object.NewString(value), codecName)
			if err != nil {
				t.Fatalf("encoding error: %v", err)
			}
			decoded, err := Decode(ctx, encoded, codecName)
			if err != nil {
				t.Fatalf("decoding error: %v", err)
			}
			assert.Equal(t, decoded, object.NewBytes([]byte(value)))
		})
	}
}

func TestUnknownCodec(t *testing.T) {
	ctx := context.Background()
	_, err := Encode(ctx, object.NewString("oops"), object.NewString("unknown"))
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "codec not found: unknown")
}

func TestJsonCodec(t *testing.T) {
	ctx := context.Background()
	value := "thumbs up 👍🏼"
	encoded, err := Encode(ctx, object.NewString(value), object.NewString("json"))
	if err != nil {
		t.Fatalf("encoding error: %v", err)
	}
	assert.Equal(t, encoded, object.NewString("\""+value+"\""))
	decoded, err := Decode(ctx, encoded, object.NewString("json"))
	if err != nil {
		t.Fatalf("decoding error: %v", err)
	}
	assert.Equal(t, decoded, object.NewString(value))
}

func TestJsonEncodeNull(t *testing.T) {
	ctx := context.Background()
	jsonName := object.NewString("json")
	encoded, err := Encode(ctx, object.Nil, jsonName)
	assert.Nil(t, err)
	assert.Equal(t, encoded, object.NewString("null"))

	decoded, err := Decode(ctx, encoded, jsonName)
	assert.Nil(t, err)
	assert.Equal(t, decoded, object.Object(object.Nil))

	encoded, err = Encode(ctx, object.NewList([]object.Object{object.Nil, object.NewInt(1)}), jsonName)
	assert.Nil(t, err)
	assert.Equal(t, encoded, object.NewString("[null,1]"))
}

// Values with no JSON form are a type error, at the top level or nested,
// instead of encoding as {} or null.
func TestJsonEncodeRejectsValuesWithoutJSONForm(t *testing.T) {
	ctx := context.Background()
	jsonName := object.NewString("json")
	closure := object.NewClosure(bytecode.NewFunction(bytecode.FunctionParams{}))
	builtin := object.NewBuiltin("b", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		return object.Nil, nil
	})
	module := object.NewBuiltinsModule("m", map[string]object.Object{})
	errValue := object.NewError(errors.New("boom"))
	for _, value := range []object.Object{closure, builtin, module, errValue} {
		for _, wrapped := range []object.Object{
			value,
			object.NewList([]object.Object{value}),
			object.NewMap(map[string]object.Object{"k": value}),
			object.NewList([]object.Object{object.NewMap(map[string]object.Object{"k": value})}),
			object.NewMapValueIter(object.NewMap(map[string]object.Object{"k": value})),
		} {
			_, err := Encode(ctx, wrapped, jsonName)
			assert.NotNil(t, err, wrapped.Inspect())
			assert.Contains(t, err.Error(), "cannot encode "+string(value.Type())+" as JSON")
		}
	}
}

// JSON nested more deeply than MaxNestingDepth is an error, not an error
// value returned as the result.
func TestJsonDecodeTooDeepIsError(t *testing.T) {
	ctx := context.Background()
	deep := strings.Repeat("[", 150) + "1" + strings.Repeat("]", 150)
	result, err := Decode(ctx, object.NewString(deep), object.NewString("json"))
	assert.Nil(t, result)
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, object.ErrNestingDepth)
}

func decodeWithLimit(maxListLength int64, src, codec string) (object.Object, error) {
	ctx := object.WithLimits(context.Background(), object.Limits{MaxListLength: maxListLength})
	return Decode(ctx, object.NewString(src), object.NewString(codec))
}

// decode counts every list element and map entry in the decoded value against
// one budget of MaxListLength, as the conversion of a value to Go does.
func TestDecodeElementBudget(t *testing.T) {
	for _, tc := range []struct {
		codec string
		src   string
		count int64
	}{
		{"json", `[1, 2, 3, 4]`, 4},
		{"json", `{"a": 1, "b": 2, "c": 3, "d": 4}`, 4},
		{"json", `{"a": [1, 2, 3]}`, 4},
		{"json", `[[1, 2], [3, 4]]`, 6},
		{"json", `[{}, {}, [], {"a": null}]`, 5},
		{"json", `{"a": 1, "a": 2}`, 2},
		{"csv", "a,b\nc,d\n", 6},
		{"csv", "a\nb\nc\n", 6},
	} {
		_, err := decodeWithLimit(tc.count-1, tc.src, tc.codec)
		assertValueError(t, err, "decoded value too large (exceeds limit of")
		result, err := decodeWithLimit(tc.count, tc.src, tc.codec)
		assert.Nil(t, err, tc.src)
		assert.NotNil(t, result, tc.src)
	}
}

// A decode that exceeds the budget stops when it reaches the limit, instead of
// building the whole value and then rejecting it.
func TestDecodeStopsAtBudget(t *testing.T) {
	ctx := object.WithLimits(context.Background(), object.Limits{MaxListLength: 100})
	for codec, src := range map[string]string{
		"json": "[" + strings.Repeat("0,", 100_000) + "0]",
		"csv":  strings.Repeat("a,b\n", 50_000),
	} {
		input := object.NewString(src)
		name := object.NewString(codec)
		allocs := testing.AllocsPerRun(5, func() {
			_, err := Decode(ctx, input, name)
			assert.NotNil(t, err)
		})
		assert.True(t, allocs < 1000, "%s: %v allocations", codec, allocs)
	}
}

func TestDecodeJSONValues(t *testing.T) {
	ctx := context.Background()
	for src, want := range map[string]string{
		`[1, 2.5, -0, 1e3, 12345678901234567890]`: `[1, 2.5, -0, 1000, 12345678901234567000]`,
		`{"b": 1, "a": "x", "a": "y"}`:            `{"a": "y", "b": 1}`,
		`{"a": {"b": [true, false, null]}}`:       `{"a": {"b": [true, false, null]}}`,
		` [ ] `:                                   `[]`,
		`{}`:                                      `{}`,
		`"é\ud800"`:                               `"é�"`,
		`null`:                                    `null`,
	} {
		result, err := Decode(ctx, object.NewString(src), object.NewString("json"))
		assert.Nil(t, err, src)
		assert.Equal(t, result.Inspect(), want, src)
	}
	// Numbers decode as floats, as encoding/json decodes them into any
	result, err := Decode(ctx, object.NewString(`[1]`), object.NewString("json"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.Object(object.NewList([]object.Object{object.NewFloat(1)})))
}

// Invalid JSON returns the error that json.Unmarshal returns.
func TestDecodeJSONErrors(t *testing.T) {
	ctx := context.Background()
	for src, want := range map[string]string{
		``:           "unexpected end of JSON input",
		`[1,`:        "unexpected end of JSON input",
		`1 2`:        "invalid character '2' after top-level value",
		`{"a" 1}`:    "invalid character '1' after object key",
		`[1e400, 2]`: "json: cannot unmarshal number 1e400 into Go value of type float64",
	} {
		_, err := Decode(ctx, object.NewString(src), object.NewString("json"))
		assert.NotNil(t, err, src)
		assert.Equal(t, err.Error(), want, src)
	}
	// The error names only the first 64 bytes of a long number
	long := "1" + strings.Repeat("0", 100_000)
	_, err := Decode(ctx, object.NewString(long), object.NewString("json"))
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "json: cannot unmarshal number "+long[:64]+"... into Go value of type float64")
}

// JSON nested exactly MaxNestingDepth deep decodes.
func TestJsonDecodeAtMaxDepth(t *testing.T) {
	ctx := context.Background()
	n := object.MaxNestingDepth
	for _, src := range []string{
		strings.Repeat("[", n) + strings.Repeat("]", n),
		strings.Repeat(`{"a":`, n-1) + "{}" + strings.Repeat("}", n-1),
	} {
		_, err := Decode(ctx, object.NewString(src), object.NewString("json"))
		assert.Nil(t, err)
	}
	for _, src := range []string{
		strings.Repeat("[", n+1) + strings.Repeat("]", n+1),
		strings.Repeat(`{"a":`, n) + "{}" + strings.Repeat("}", n),
	} {
		_, err := Decode(ctx, object.NewString(src), object.NewString("json"))
		assert.ErrorIs(t, err, object.ErrNestingDepth)
	}
}

// The result of a codec registered by the host is checked against the same
// budget, since decode cannot know how the codec built it.
func TestDecodeChecksCustomCodecResult(t *testing.T) {
	var result object.Object
	err := RegisterCodec("test-custom-result", &Codec{
		Decode: func(ctx context.Context, obj object.Object) (object.Object, error) {
			return result, nil
		},
	})
	assert.Nil(t, err)
	ctx := object.WithLimits(context.Background(), object.Limits{MaxListLength: 3})
	name := object.NewString("test-custom-result")
	ints := func(n int) []object.Object {
		items := make([]object.Object, n)
		for i := range items {
			items[i] = object.NewInt(int64(i))
		}
		return items
	}
	cyclic := object.NewList(nil)
	cyclic.Append(cyclic)

	for _, value := range []object.Object{
		object.NewList(ints(4)),
		object.NewMap(map[string]object.Object{"a": object.Nil, "b": object.Nil, "c": object.Nil, "d": object.Nil}),
		object.NewList([]object.Object{object.NewList(ints(3))}),
		object.NewMap(map[string]object.Object{"a": object.NewList(ints(3))}),
		cyclic,
	} {
		result = value
		_, err := Decode(ctx, object.NewString(""), name)
		assert.NotNil(t, err, value.Inspect())
	}

	// With a large budget, a cyclic result fails on its depth
	result = cyclic
	_, err = Decode(context.Background(), object.NewString(""), name)
	assert.ErrorIs(t, err, object.ErrNestingDepth)

	for _, value := range []object.Object{
		object.NewList(ints(3)),
		object.NewList([]object.Object{object.NewList(ints(2))}),
		object.NewString("abc"),
	} {
		result = value
		decoded, err := Decode(ctx, object.NewString(""), name)
		assert.Nil(t, err, value.Inspect())
		assert.Equal(t, decoded, value)
	}
}

// A decode stops with ctx.Err() when ctx is canceled.
func TestDecodeContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for codec, src := range map[string]string{
		"json": "[" + strings.Repeat("0,", 2000) + "0]",
		"csv":  strings.Repeat("a,b\n", 1000),
	} {
		_, err := Decode(ctx, object.NewString(src), object.NewString(codec))
		assert.ErrorIs(t, err, context.Canceled, codec)
	}
}

// Invalid CSV returns the error that csv.Reader returns.
func TestDecodeCsvError(t *testing.T) {
	_, err := Decode(context.Background(), object.NewString("a,b\nc"), object.NewString("csv"))
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "record on line 2: wrong number of fields")
}

// decode("json") returns the value, or the error, that decoding with
// json.Unmarshal and converting the result with FromGo returns.
func FuzzDecodeJSON(f *testing.F) {
	for _, seed := range []string{
		``, `null`, `true`, `false`, `0`, `-0`, `1.5e-3`, `1E+2`, `1e400`,
		`12345678901234567890`, `""`, `"a\"b\\c\/d\b\f\n\r\t"`, `"é😀"`,
		`"\ud800"`, "\"\xff\"", `"é"`, `[]`, `{}`, ` [ 1 , [ ] , { } ] `,
		`{"a": 1, "a": 2}`, `{"a": {"b": [null, true, "x"]}}`, `[1,`, `1 2`,
		`{"a" 1}`, `[1e400, 2]`, `["x", 1e999]`, "1" + strings.Repeat("0", 400),
	} {
		f.Add(seed)
	}
	ctx := context.Background()
	name := object.NewString("json")
	f.Fuzz(func(t *testing.T, src string) {
		got, err := Decode(ctx, object.NewString(src), name)
		var v any
		if wantErr := json.Unmarshal([]byte(src), &v); wantErr != nil {
			assert.NotNil(t, err)
			// A value that is both too deep and has a number out of range
			// fails on whichever comes first.
			if errors.Is(err, object.ErrNestingDepth) {
				return
			}
			// The error names at most 64 bytes of a number literal
			var gotType, wantType *json.UnmarshalTypeError
			if errors.As(wantErr, &wantType) && len(wantType.Value) > len("number ")+64 {
				assert.ErrorAs(t, err, &gotType)
				assert.Equal(t, gotType.Value, wantType.Value[:len("number ")+64]+"...")
				return
			}
			assert.Equal(t, err.Error(), wantErr.Error())
			return
		}
		want, wantErr := object.DefaultRegistry().FromGo(v)
		if wantErr != nil {
			assert.ErrorIs(t, err, object.ErrNestingDepth)
			return
		}
		assert.Nil(t, err)
		assert.Equal(t, got.Inspect(), want.Inspect())
		assert.True(t, got.Equals(want))
	})
}

func BenchmarkDecodeJSON(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("[")
	for i := range 1000 {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id": %d, "name": "user %d", "active": true, "score": %d.5, "tags": ["a", "b", "c"], "address": {"city": "Springfield", "zip": null}}`, i, i, i)
	}
	sb.WriteString("]")
	input := object.NewString(sb.String())
	name := object.NewString("json")
	ctx := context.Background()
	b.SetBytes(int64(sb.Len()))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Decode(ctx, input, name); err != nil {
			b.Fatal(err)
		}
	}
}
