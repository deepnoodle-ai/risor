// Test blank identifier "_" in multi-variable assignment
// expected value: 5
// expected type: int

// Discard first value, keep second
let _, b = [1, 2]

// Discard second value, keep first
let c, _ = [10, 20]

// Discard both ends, keep middle
let _, middle, _ = [100, 200, 300]

// Verify values are correct
// b = 2, c = 10, middle = 200
b + (c / 10) + (middle / 100)  // 2 + 1 + 2 = 5
