// expected value: [1, 2, 3, 4, 5]
// expected type: list

let l = [1, 2, 3]

let funcs = [l.append]

funcs[0](4)
funcs[0](5)

l
