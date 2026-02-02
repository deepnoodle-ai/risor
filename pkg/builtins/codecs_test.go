package builtins

import (
	"context"
	"testing"

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
