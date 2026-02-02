import time

start = time.time()


def max(array):
    if not array:
        return 0
    cur = array[0]
    for i, val in enumerate(array):
        if i > 0 and val > cur:
            cur = val
    return cur


a = list(range(99999))
print("MAX", max(a))
print("dt:", time.time() - start)
