// Test blank identifier "_" in destructuring
// expected value: 11
// expected type: int

// Array destructuring - discard first element
let [_, second] = [1, 2]

// Array destructuring - discard middle element
let [first, _, third] = [10, 20, 30]

// Object destructuring - discard 'x' property
let {x: _, y} = {x: 100, y: 5}

// Verify values: second=2, first=10, third=30, y=5
second + first / 10 + third / 10 + y  // 2 + 1 + 3 + 5 = 11
