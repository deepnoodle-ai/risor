from dis import dis


def incrementer():
    x = 0
    y = 0

    def increment():
        nonlocal x, y
        x += 1
        return x

    return increment


inc = incrementer()
print(inc())
print(inc())

print("----")
print("dis for incrementer:")
dis(incrementer)
