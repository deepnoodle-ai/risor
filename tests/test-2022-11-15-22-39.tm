// github issue: n/a
// expected value: 10
// expected type: int

let i = 0

for {
    i++
    if i == 10 {
        break
    }
}

i