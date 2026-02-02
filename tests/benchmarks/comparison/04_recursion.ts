// Recursion Benchmark
// Tests: recursive calls, stack management

function factorial(n: number): number {
    if (n <= 1) {
        return 1
    }
    return n * factorial(n - 1)
}

function sumTo(n: number): number {
    if (n <= 0) {
        return 0
    }
    return n + sumTo(n - 1)
}

function ackermann(m: number, n: number): number {
    if (m == 0) {
        return n + 1
    }
    if (n == 0) {
        return ackermann(m - 1, 1)
    }
    return ackermann(m - 1, ackermann(m, n - 1))
}

// Run benchmarks
let f20 = factorial(20)
let s1000 = sumTo(1000)
let a33 = ackermann(3, 3)

console.log({factorial: f20, sum: s1000, ackermann: a33})
