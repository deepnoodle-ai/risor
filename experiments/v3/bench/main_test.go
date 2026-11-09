package bench

import (
	"os"
	"strconv"
	"testing"
)

// ballast is pointer-free memory that stays live, so the collector sees a
// heap the size of a running service's and not the few megabytes of a
// benchmark binary. With the default GOGC a collection then starts after
// BALLAST_MB of new allocation, not after about 4 MB.
var ballast []byte

func TestMain(m *testing.M) {
	if mb, _ := strconv.Atoi(os.Getenv("BALLAST_MB")); mb > 0 {
		ballast = make([]byte, mb<<20)
	}
	os.Exit(m.Run())
}
