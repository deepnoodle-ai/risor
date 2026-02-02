// Array Operations Benchmark
// Tests: map, filter, closures, list iteration

// Generate array of numbers 1..1000
let numbers = Array.from({length: 100000}, (_, i) => i + 1)

// Chain: filter evens, square them, sum
let evens = numbers.filter(x => x % 2 == 0)
let squares = evens.map(x => x * x)
let result = squares.reduce((acc, x) => acc + x, 0)

console.log(result)
