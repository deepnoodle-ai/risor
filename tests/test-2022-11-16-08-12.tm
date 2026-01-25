// github issue: https://github.com/risor-io/risor/issues/6
// expected value: 11
// expected type: int

let s = "\ntest\t\"str\\"

let raw = `
test	"str\`

assert(s == raw)

len(s)
