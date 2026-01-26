package rand

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
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

func Float(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("rand.float", 0, args); err != nil {
		return err
	}
	return object.NewFloat(rand.Float64())
}

func Int(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("rand.int", 0, args); err != nil {
		return err
	}
	return object.NewInt(rand.Int63())
}

func IntN(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("rand.intn", 1, args); err != nil {
		return err
	}
	n, err := object.AsInt(args[0])
	if err != nil {
		return err
	}
	return object.NewInt(rand.Int63n(n))
}

func NormFloat(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("rand.norm_float", 0, args); err != nil {
		return err
	}
	return object.NewFloat(rand.NormFloat64())
}

func ExpFloat(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("rand.exp_float", 0, args); err != nil {
		return err
	}
	return object.NewFloat(rand.ExpFloat64())
}

func Shuffle(ctx context.Context, args ...object.Object) object.Object {
	if err := object.Require("rand.shuffle", 1, args); err != nil {
		return err
	}
	ls, err := object.AsList(args[0])
	if err != nil {
		return err
	}
	items := ls.Value()
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
	return ls
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
