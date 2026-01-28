// expected value: [1, 2, 3, 4]
// expected type: list

let a = [1,2,3]
a.append(4)
assert(a[3] == 4)

a.clear()
assert(len(a) == 0)

a.extend([1,2])
assert(len(a) == 2)

a.extend([3,4])
assert(len(a) == 4)

a