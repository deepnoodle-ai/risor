//go:build race

package risor_test

// raceBuild reports that the race detector is on. It makes frames larger, so
// a measurement of stack holds only without it.
const raceBuild = true
