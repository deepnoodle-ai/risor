// Fibonacci Benchmark
// Tests: recursive function calls, arithmetic

function fibonacci(n: number): number {
    if (n <= 1) {
        return n
    }
    return fibonacci(n - 1) + fibonacci(n - 2)
}

// Run benchmark
let result = fibonacci(30)
console.log(result)
