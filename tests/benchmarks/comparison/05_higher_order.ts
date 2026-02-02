// Higher-Order Functions Benchmark
// Tests: function passing, composition

function compose<A, B, C>(f: (b: B) => C, g: (a: A) => B): (a: A) => C {
    return x => f(g(x))
}

// Build pipeline functions
let double = (x: number) => x * 2
let addOne = (x: number) => x + 1
let square = (x: number) => x * x
let addTen = (x: number) => x + 10

// Test composition (nested)
let composed = compose(addTen, compose(double, compose(addOne, square)))

// Apply to many values
let numbers = Array.from({length: 50000}, (_, i) => i + 1)
let step1 = numbers.map(composed)
let results = step1.reduce((acc, x) => acc + x, 0)

console.log(results)
