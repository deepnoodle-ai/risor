// expected value: [1, 2, 2.2, 3]
// expected type: list

s1 := {1, 2.2}
s2 := {2, 2.2}

s1.add(3)

s1.union(s2) | sorted
