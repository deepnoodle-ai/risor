// Closure Benchmark
// Tests: closure creation, captured variable access

function makeAccumulator(initial: number) {
    let total = initial
    return {
        add: function(n: number) {
            total = total + n
            return total
        },
        get: function() {
            return total
        }
    }
}

// Create accumulators and run many operations
let acc1 = makeAccumulator(0)
let acc2 = makeAccumulator(100)

// Use functional iteration
Array.from({length: 100000}, (_, i) => i).map(i => {
    acc1.add(i)
    acc2.add(i * 2)
    return i
})

let result = acc1.get() + acc2.get()
console.log(result)
