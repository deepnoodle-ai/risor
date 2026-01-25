// github issue: n/a
// expected value: '"hello"'
// expected type: string

let s = "\"hello\""
let j = json.unmarshal(s)
assert(type(j) == "string")
assert(j == "hello")

s = json.marshal("hello")
assert(type(s) == "string")
assert(s == "\"hello\"")

s
