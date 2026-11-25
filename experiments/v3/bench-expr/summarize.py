#!/usr/bin/env python3
"""Summarize `go test -bench=. -benchmem -count=N` output into median tables."""

import collections
import re
import statistics as st
import sys

LINE_RE = re.compile(
    r"^Benchmark(\w+)/([^/]+)/(\w+)-\d+\s+\d+\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op"
)

GROUPS = [
    ("CompileMaps", ["risor_v3", "risor_v3_nocheck", "deepnoodle_expr", "deepnoodle_expr_check", "expr_lang"]),
    ("CompileStructs", ["risor_v3", "risor_v3_nocheck", "deepnoodle_expr", "expr_lang"]),
    ("RunMaps", ["risor_v3", "deepnoodle_expr", "expr_lang"]),
    ("RunStructs", ["risor_v3", "deepnoodle_expr", "expr_lang"]),
    ("TemplateCompile", ["risor_v3", "risor_v3_nocheck", "deepnoodle_expr", "expr_lang"]),
    ("TemplateRun", ["risor_v3", "deepnoodle_expr", "expr_lang"]),
]


def fmt_ns(ns: float | None) -> str:
    if ns is None:
        return "—"
    if ns < 1_000:
        return f"{ns:.0f} ns"
    if ns < 1_000_000:
        return f"{ns / 1_000:.2f} µs"
    return f"{ns / 1_000_000:.2f} ms"


def fmt_ratio(a: float | None, b: float | None) -> str:
    if a is None or b is None or a == 0:
        return "—"
    return f"{b / a:.2f}x"


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit(f"usage: {sys.argv[0]} <bench-output.txt>")

    data: dict[str, dict[str, dict[str, list[tuple[float, int, int]]]]] = collections.defaultdict(
        lambda: collections.defaultdict(lambda: collections.defaultdict(list))
    )
    order: dict[str, list[str]] = collections.defaultdict(list)

    with open(sys.argv[1], encoding="utf-8") as f:
        for line in f:
            m = LINE_RE.match(line.strip())
            if not m:
                continue
            grp, case, impl, ns, b, a = m.groups()
            if case not in order[grp]:
                order[grp].append(case)
            data[grp][case][impl].append((float(ns), int(b), int(a)))

    for grp, impls in GROUPS:
        if grp not in data:
            continue
        print(f"## {grp}\n")
        header = ["Case"] + impls + ["dexpr/v3", "expr-lang/v3"]
        print("| " + " | ".join(header) + " |")
        print("| --- | " + " | ".join(["---:"] * (len(header) - 1)) + " |")
        max_spread = 0.0
        for case in order[grp]:
            med_ns: dict[str, float] = {}
            for impl in impls:
                samples = data[grp][case].get(impl)
                if samples:
                    ns_vals = [x[0] for x in samples]
                    m = st.median(ns_vals)
                    med_ns[impl] = m
                    if m > 0:
                        max_spread = max(max_spread, (max(ns_vals) - min(ns_vals)) / m)
            row = [case] + [fmt_ns(med_ns.get(impl)) for impl in impls]
            row.append(fmt_ratio(med_ns.get("risor_v3"), med_ns.get("deepnoodle_expr")))
            row.append(fmt_ratio(med_ns.get("risor_v3"), med_ns.get("expr_lang")))
            print("| " + " | ".join(row) + " |")

        print(f"\nMax (max-min)/median spread: {max_spread:.1%}\n")

        alloc_header = ["Case"] + impls
        print("| " + " | ".join(alloc_header) + " |")
        print("| --- | " + " | ".join(["---:"] * (len(alloc_header) - 1)) + " |")
        for case in order[grp]:
            row = [case]
            for impl in impls:
                samples = data[grp][case].get(impl)
                if samples:
                    b_med = int(round(st.median(x[1] for x in samples)))
                    a_med = int(round(st.median(x[2] for x in samples)))
                    row.append(f"{b_med} B / {a_med}")
                else:
                    row.append("—")
            print("| " + " | ".join(row) + " |")
        print()


if __name__ == "__main__":
    main()
