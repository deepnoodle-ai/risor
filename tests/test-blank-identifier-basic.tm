// Test blank identifier "_" basic usage
// expected value: 42
// expected type: int

// Blank identifier in let discards values
let _ = "discarded"
let _ = [1, 2, 3]

// Regular assignment to _ discards values
_ = "also discarded"

// Return a value to verify the test
42
