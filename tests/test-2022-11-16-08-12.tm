// github issue: https://github.com/deepnoodle-ai/risor/issues/6
// expected value: 11
// expected type: int

let s = "\ntest\t\"str\\"

let raw = `
test	"str\`

assert(s == raw)

len(s)
