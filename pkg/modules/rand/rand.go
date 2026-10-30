package rand

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

// Seed is deprecated and does nothing.
// As of Go 1.20, the global random source is automatically seeded.
func Seed() {}

// Random returns a random float in [0.0, 1.0).
// Equivalent to Python's random.random() or JavaScript's Math.random().
func Random(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("rand.random: expected 0 arguments, got %d", len(args))
	}
	return object.NewFloat(rand.Float64()), nil
}

// Int returns a random integer.
// With no arguments: returns a random non-negative int64.
// With one argument n: returns a random int in [0, n).
// With two arguments min, max: returns a random int in [min, max).
func Int(ctx context.Context, args ...object.Object) (object.Object, error) {
	switch len(args) {
	case 0:
		return object.NewInt(rand.Int63()), nil
	case 1:
		max, err := object.AsInt(args[0])
		if err != nil {
			return nil, err
		}
		if max <= 0 {
			return nil, fmt.Errorf("rand.int: max must be positive, got %d", max)
		}
		return object.NewInt(rand.Int63n(max)), nil
	case 2:
		min, err := object.AsInt(args[0])
		if err != nil {
			return nil, err
		}
		max, err := object.AsInt(args[1])
		if err != nil {
			return nil, err
		}
		if max <= min {
			return nil, fmt.Errorf("rand.int: max must be greater than min, got min=%d max=%d", min, max)
		}
		return object.NewInt(between(min, max-1)), nil
	default:
		return nil, fmt.Errorf("rand.int: expected 0-2 arguments, got %d", len(args))
	}
}

// Randint returns a random integer in [a, b] inclusive.
// Matches Python's random.randint(a, b) behavior.
func Randint(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("rand.randint: expected 2 arguments, got %d", len(args))
	}
	a, err := object.AsInt(args[0])
	if err != nil {
		return nil, err
	}
	b, err := object.AsInt(args[1])
	if err != nil {
		return nil, err
	}
	if b < a {
		return nil, fmt.Errorf("rand.randint: b must be >= a, got a=%d b=%d", a, b)
	}
	return object.NewInt(between(a, b)), nil
}

// between returns a random integer in [lo, hi] inclusive. lo must be <= hi.
// The span is computed in uint64 so that ranges wider than math.MaxInt64,
// up to the full int64 range, do not overflow.
func between(lo, hi int64) int64 {
	span := uint64(hi) - uint64(lo)
	if span < math.MaxInt64 {
		return lo + rand.Int63n(int64(span)+1)
	}
	// The span does not fit Int63n. Rejection-sample from the full uint64
	// range; at least half of all values are accepted.
	for {
		v := rand.Uint64()
		if v <= span {
			return int64(uint64(lo) + v)
		}
	}
}

// Uniform returns a random float in [a, b].
// Matches Python's random.uniform(a, b) behavior.
func Uniform(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("rand.uniform: expected 2 arguments, got %d", len(args))
	}
	a, err := object.AsFloat(args[0])
	if err != nil {
		return nil, err
	}
	b, err := object.AsFloat(args[1])
	if err != nil {
		return nil, err
	}
	return object.NewFloat(a + rand.Float64()*(b-a)), nil
}

// Normal returns a random float from a normal (Gaussian) distribution.
// With no arguments: mean=0, stddev=1 (standard normal).
// With two arguments: mean=mu, stddev=sigma.
func Normal(ctx context.Context, args ...object.Object) (object.Object, error) {
	var mu, sigma float64 = 0, 1
	switch len(args) {
	case 0:
		// Use defaults
	case 2:
		var err error
		mu, err = object.AsFloat(args[0])
		if err != nil {
			return nil, err
		}
		sigma, err = object.AsFloat(args[1])
		if err != nil {
			return nil, err
		}
		if sigma < 0 {
			return nil, fmt.Errorf("rand.normal: sigma must be non-negative, got %f", sigma)
		}
	default:
		return nil, fmt.Errorf("rand.normal: expected 0 or 2 arguments, got %d", len(args))
	}
	return object.NewFloat(mu + sigma*rand.NormFloat64()), nil
}

// Exponential returns a random float from an exponential distribution.
// With no arguments: lambda=1.
// With one argument: lambda (rate parameter).
func Exponential(ctx context.Context, args ...object.Object) (object.Object, error) {
	var lambda float64 = 1
	switch len(args) {
	case 0:
		// Use default
	case 1:
		var err error
		lambda, err = object.AsFloat(args[0])
		if err != nil {
			return nil, err
		}
		if lambda <= 0 {
			return nil, fmt.Errorf("rand.exponential: lambda must be positive, got %f", lambda)
		}
	default:
		return nil, fmt.Errorf("rand.exponential: expected 0 or 1 arguments, got %d", len(args))
	}
	// ExpFloat64 returns exponential with rate=1, scale by 1/lambda
	return object.NewFloat(rand.ExpFloat64() / lambda), nil
}

// Choice returns a random element from a list.
// Matches Python's random.choice(seq) behavior.
func Choice(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("rand.choice: expected 1 argument, got %d", len(args))
	}
	ls, err := object.AsList(args[0])
	if err != nil {
		return nil, err
	}
	items := ls.Value()
	if len(items) == 0 {
		return nil, fmt.Errorf("rand.choice: cannot choose from empty list")
	}
	return items[rand.Intn(len(items))], nil
}

// Sample returns k unique random elements from a list (without replacement).
// Matches Python's random.sample(seq, k) behavior.
func Sample(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("rand.sample: expected 2 arguments, got %d", len(args))
	}
	ls, err := object.AsList(args[0])
	if err != nil {
		return nil, err
	}
	k, err := object.AsInt(args[1])
	if err != nil {
		return nil, err
	}
	items := ls.Value()
	n := int64(len(items))
	if k < 0 {
		return nil, fmt.Errorf("rand.sample: k must be non-negative, got %d", k)
	}
	if k > n {
		return nil, fmt.Errorf("rand.sample: k (%d) cannot be larger than list length (%d)", k, n)
	}
	// Fisher-Yates partial shuffle
	result := make([]object.Object, k)
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	for i := range k {
		j := i + rand.Int63n(n-i)
		indices[i], indices[j] = indices[j], indices[i]
		result[i] = items[indices[i]]
	}
	return object.NewList(result), nil
}

// Shuffle randomly reorders the elements of a list in place.
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

// maxBytes is the largest n accepted by Bytes. The result is a list with one
// object per byte, so this bounds a single call to a few tens of MiB.
const maxBytes = 1 << 20

// Bytes returns a list of n random bytes (0-255).
// Useful for generating random data.
func Bytes(ctx context.Context, args ...object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("rand.bytes: expected 1 argument, got %d", len(args))
	}
	n, err := object.AsInt(args[0])
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, fmt.Errorf("rand.bytes: n must be non-negative, got %d", n)
	}
	if n > maxBytes {
		return nil, object.ValueErrorf("rand.bytes: n too large (%d exceeds limit of %d)", n, maxBytes)
	}
	result := make([]object.Object, n)
	for i := range n {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		result[i] = object.NewInt(int64(rand.Intn(256)))
	}
	return object.NewList(result), nil
}

func Module() *object.Module {
	return object.NewBuiltinsModule("rand", map[string]object.Object{
		"random":      object.NewBuiltin("random", Random),
		"int":         object.NewBuiltin("int", Int),
		"randint":     object.NewBuiltin("randint", Randint),
		"uniform":     object.NewBuiltin("uniform", Uniform),
		"normal":      object.NewBuiltin("normal", Normal),
		"exponential": object.NewBuiltin("exponential", Exponential),
		"choice":      object.NewBuiltin("choice", Choice),
		"sample":      object.NewBuiltin("sample", Sample),
		"shuffle":     object.NewBuiltin("shuffle", Shuffle),
		"bytes":       object.NewBuiltin("bytes", Bytes),
	})
}
