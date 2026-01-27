// Test blank identifier "_" in function parameters
// expected value: 30
// expected type: int

// Function that ignores first parameter
function ignoreFirst(_, b) {
    return b * 2
}

// Function that ignores second parameter
function ignoreSecond(a, _) {
    return a * 3
}

// Function that ignores multiple parameters
function ignoreMany(_, x, _, y, _) {
    return x + y
}

// Arrow function with blank identifier
let double = (_, n) => n * 2

// Verify: ignoreFirst(100, 5) = 10, ignoreSecond(4, 999) = 12
// ignoreMany(1, 2, 3, 4, 5) = 2 + 4 = 6, double(0, 1) = 2
ignoreFirst(100, 5) + ignoreSecond(4, 999) + ignoreMany(1, 2, 3, 4, 5) + double(0, 1)
// 10 + 12 + 6 + 2 = 30
