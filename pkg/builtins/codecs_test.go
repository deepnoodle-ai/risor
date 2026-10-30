package builtins

import (
	"context"
	"errors"
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
