package eval

import (
	"sort"
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
	"github.com/deepnoodle-ai/wonton/assert"
)

// The registry has exactly the entries of library.json, and each entry's
// parameters are those of its record's signature (04-library.md 1.3 item 1).
func TestRegistryMatchesTable(t *testing.T) {
	var want []string
	for _, e := range library.Entries {
		want = append(want, Key(e))
		assert.True(t, Implemented(e), Key(e))
		sig, err := typetext.ParseSig(e.Signature)
		assert.NoError(t, err)
		assert.Equal(t, len(e.Params), len(sig.Params), e.Name)
		for i, p := range sig.Params {
			assert.Equal(t, e.Params[i].Name, p.Name, e.Name)
			assert.Equal(t, e.Params[i].Callback >= 0, p.Fn != nil, e.Name)
		}
		assert.Equal(t, e.MinArgs(), sig.MinArgs(), e.Name)
		assert.Equal(t, e.MaxArgs(), sig.MaxArgs(), e.Name)
	}
	got := RegistryKeys()
	sort.Strings(want)
	sort.Strings(got)
	assert.Equal(t, got, want)
	assert.Equal(t, len(got), 59) // 53 of Core and 6 of F7
}

func TestStableSort(t *testing.T) {
	vals := []int{3, 1, 2, 1, 3, 0}
	idx := []int{0, 1, 2, 3, 4, 5}
	comparisons := 0
	stableSort(idx, func(a, b int) bool { comparisons++; return vals[a] < vals[b] })
	assert.Equal(t, idx, []int{5, 1, 3, 2, 0, 4})
	big := make([]int, 1<<16)
	for i := range big {
		big[i] = (i * 7919) % 1000
	}
	idx = make([]int, len(big))
	for i := range idx {
		idx[i] = i
	}
	comparisons = 0
	stableSort(idx, func(a, b int) bool { comparisons++; return big[a] < big[b] })
	assert.True(t, comparisons <= len(big)*16, "comparisons %d", comparisons)
	for i := 1; i < len(idx); i++ {
		a, b := big[idx[i-1]], big[idx[i]]
		assert.True(t, a < b || (a == b && idx[i-1] < idx[i]))
	}
}

func TestOccurrences(t *testing.T) {
	count := func(s, t string) int { return occurrences(s, t, nil) }
	assert.Equal(t, count("aaa", "aa"), 1)
	assert.Equal(t, count("aaaa", "aa"), 2)
	assert.Equal(t, count("abc", "x"), 0)
	long := ""
	for i := 0; i < 70; i++ {
		long += "a"
	}
	assert.Equal(t, count(long+long+"b"+long, long), 3)
	assert.Equal(t, count("ab", long), 0)
}

func TestFixed(t *testing.T) {
	for _, c := range []struct {
		x    float64
		d    int
		want string
	}{
		{2.5, 0, "3"}, {-2.5, 0, "-3"}, {1.005, 2, "1.00"}, {-0.001, 2, "0.00"},
		{1e20, 2, "100000000000000000000.00"}, {0.1, 20, "0.10000000000000000555"},
		{7, 0, "7"}, {9.995, 2, "9.99"}, {0.5, 0, "1"}, {99.5, 0, "100"},
	} {
		assert.Equal(t, fixed(c.x, c.d), c.want, "%v %d", c.x, c.d)
	}
}
