package rand

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/risor-io/risor/object"
)

func Seed() {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}

func Float(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("rand.float: expected 0 arguments, got %d", len(args))
	}
	return object.NewFloat(rand.Float64()), nil
}

func Int(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("rand.int: expected 0 arguments, got %d", len(args))
	}
	return object.NewInt(rand.Int63()), nil
}

func IntN(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("rand.intn: expected 1 argument, got %d", len(args))
	}
	n, err := object.AsInt(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewInt(rand.Int63n(n)), nil
}

func NormFloat(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("rand.norm_float: expected 0 arguments, got %d", len(args))
	}
	return object.NewFloat(rand.NormFloat64()), nil
}

func ExpFloat(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("rand.exp_float: expected 0 arguments, got %d", len(args))
	}
	return object.NewFloat(rand.ExpFloat64()), nil
}

func Shuffle(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("rand.shuffle: expected 1 argument, got %d", len(args))
	}
	ls, err := object.AsList(args[0])
	if err != nil {
		return nil, err
	}
	items := ls.Value()
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
	return ls, nil
}

func Module() *object.Module {
	return object.NewBuiltinsModule("rand", map[string]object.Object{
		"float":      object.NewBuiltin("float", Float),
		"int":        object.NewBuiltin("int", Int),
		"intn":       object.NewBuiltin("intn", IntN),
		"norm_float": object.NewBuiltin("norm_float", NormFloat),
		"exp_float":  object.NewBuiltin("exp_float", ExpFloat),
		"shuffle":    object.NewBuiltin("shuffle", Shuffle),
	})
}
