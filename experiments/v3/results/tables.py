import re, sys, statistics as st, collections
rows = collections.defaultdict(lambda: collections.defaultdict(list))
order = []
for line in open(sys.argv[1]):
    m = re.match(r'Benchmark(\w+)/(\w+)/(\w+)-\d+\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op', line)
    if not m: continue
    grp, case, impl, ns, b, a = m.groups()
    key = (grp, case)
    if key not in order: order.append(key)
    rows[key][impl].append((float(ns), int(b), int(a)))
impls = ['expr', 'risor', 'risor_vm', 'risor_vm_floor', 'spike']
def fmt(ns):
    if ns is None: return '-'
    if ns < 1000: return f'{ns:.0f} ns'
    if ns < 1e6: return f'{ns/1000:.1f} µs'
    return f'{ns/1e6:.2f} ms'
maxspread = 0
print(f'{"case":24}' + ''.join(f'{i:>14}' for i in impls) + '   vm_floor/spike  spike/expr')
for key in order:
    med = {}
    for i in impls:
        v = rows[key].get(i)
        if v:
            ns = [x[0] for x in v]
            med[i] = st.median(ns)
            maxspread = max(maxspread, (max(ns)-min(ns))/med[i])
    r1 = med['risor_vm_floor']/med['spike']
    r2 = f"{med['spike']/med['expr']:.2f}x" if 'expr' in med else '-'
    print(f'{key[1]:24}' + ''.join(f'{fmt(med.get(i)):>14}' for i in impls) + f'{r1:12.1f}x {r2:>10}')
print()
print(f'{"case":24}' + ''.join(f'{i:>18}' for i in impls))
for key in order:
    s = ''
    for i in impls:
        v = rows[key].get(i)
        s += f'{(str(v[0][1])+" B/"+str(v[0][2])) if v else "-":>18}'
    print(f'{key[1]:24}' + s)
print('max (max-min)/median spread:', f'{maxspread:.0%}')
