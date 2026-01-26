package builtins

import (
	"context"
	"testing"

	"github.com/risor-io/risor/object"
	"github.com/stretchr/testify/require"
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
			require.Equal(t, object.NewByteSlice([]byte(value)), decoded)
		})
	}
}

func TestUnknownCodec(t *testing.T) {
	ctx := context.Background()
	encoded := Encode(ctx, object.NewString("oops"), object.NewString("unknown"))
	errObj, ok := encoded.(*object.Error)
	require.True(t, ok)
	require.Equal(t, "codec not found: unknown", errObj.Value().Error())
}

func TestJsonCodec(t *testing.T) {
	ctx := context.Background()
	value := "thumbs up 👍🏼"
	encoded := Encode(ctx, object.NewString(value), object.NewString("json"))
	if errObj, ok := encoded.(*object.Error); ok {
		t.Fatalf("encoding error: %v", errObj)
	}
	require.Equal(t, object.NewString("\""+value+"\""), encoded)
	decoded := Decode(ctx, encoded, object.NewString("json"))
	if errObj, ok := decoded.(*object.Error); ok {
		t.Fatalf("decoding error: %v", errObj)
	}
	require.Equal(t, object.NewString(value), decoded)
}
