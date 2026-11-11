package crmath

// The fast phase's tables and constants are in tables_data.go, which
// TestTables (tables_test.go) generates: it computes each value with this
// package's own big-number kernels at 256 bits, whose relative error is
// below 2^-(256−Guard) = 2^-216, and rounds it to a pair. A value v stored
// as (h, l), h = fl(v), l = fl(v − h), is within 2^-106·|v| of v. The test
// recomputes every value on each run and fails if the file differs. No
// table digit comes from Go's math package.

type fastTables struct {
	// exp: 2^(i/64) and 2^(j/4096), i, j in [0, 64); ln 2/4096; 1/6.
	exp64, exp4096 [64][2]float64
	ln2by4096      [2]float64
	c6             [2]float64

	// log: r_i, the double nearest 1/(1 + (i + 1/2)/512) (r_0 = 1), and
	// −log r_i; ln 2; (−1)^k/(k+1) for k = 0 … 4; 1/ln 10.
	logR   [512]float64
	logRL  [512][2]float64
	ln2    [2]float64
	l1p    [5][2]float64
	ilog10 [2]float64

	// sin, cos: sin(j/64), cos(j/64), j in [0, 52); 1/120, −1/6, 1/24.
	sin64, cos64 [52][2]float64
	s120, s6     [2]float64
	c24          [2]float64

	// The bits of 2/π: bit i (weight 2^-i, i ≥ 1) is bit 63 − (i−1)%64 of
	// word (i−1)/64. 1280 bits, the last within one unit.
	twoOverPi [20]uint64
	halfPi    [2]float64
	pi        [2]float64

	// atan: atan(j/64), j in [0, 64]; −1/3, 1/5.
	atan64 [65][2]float64
	a3, a5 [2]float64
}

func tab() *fastTables { return &tabs }
