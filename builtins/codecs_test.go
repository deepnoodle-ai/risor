package builtins

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
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
			encoded := Encode(ctx, object.NewString(value), codecName)
			if errObj, ok := encoded.(*object.Error); ok {
				t.Fatalf("encoding error: %v", errObj)
			}
			decoded := Decode(ctx, encoded, codecName)
			if errObj, ok := decoded.(*object.Error); ok {
				t.Fatalf("decoding error: %v", errObj)
			}
			assert.Equal(t, decoded, object.NewBytes([]byte(value)))
		})
	}
}

func TestUnknownCodec(t *testing.T) {
	ctx := context.Background()
	encoded := Encode(ctx, object.NewString("oops"), object.NewString("unknown"))
	errObj, ok := encoded.(*object.Error)
	assert.True(t, ok)
	assert.Equal(t, errObj.Value().Error(), "codec not found: unknown")
}

func TestJsonCodec(t *testing.T) {
	ctx := context.Background()
	value := "thumbs up 👍🏼"
	encoded := Encode(ctx, object.NewString(value), object.NewString("json"))
	if errObj, ok := encoded.(*object.Error); ok {
		t.Fatalf("encoding error: %v", errObj)
	}
	assert.Equal(t, encoded, object.NewString("\""+value+"\""))
	decoded := Decode(ctx, encoded, object.NewString("json"))
	if errObj, ok := decoded.(*object.Error); ok {
		t.Fatalf("decoding error: %v", errObj)
	}
	assert.Equal(t, decoded, object.NewString(value))
}
