package risor

// AssistPanics is the count of faults assist has turned into an empty
// result, for the tests of package risor_test.
func AssistPanics() int64 { return assistPanics.Load() }
