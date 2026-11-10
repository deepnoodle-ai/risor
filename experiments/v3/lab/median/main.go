// Command median reads `go test -bench` output on stdin and prints the median
// of each benchmark's runs as a Markdown table.
//
//	GOWORK=off go test ./lab/valuerep -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
)

var line = regexp.MustCompile(`^Benchmark(\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op(?:\s+(\d+) B/op\s+(\d+) allocs/op)?`)

func main() {
	type row struct {
		ns     []float64
		b, all string
	}
	rows := map[string]*row{}
	var order []string
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		m := line.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		r := rows[m[1]]
		if r == nil {
			r = &row{}
			rows[m[1]] = r
			order = append(order, m[1])
		}
		ns, _ := strconv.ParseFloat(m[2], 64)
		r.ns = append(r.ns, ns)
		r.b, r.all = m[3], m[4]
	}
	fmt.Println("| Benchmark | Median | B/op | allocs/op | Spread |")
	fmt.Println("| --- | ---: | ---: | ---: | ---: |")
	for _, name := range order {
		r := rows[name]
		slices.Sort(r.ns)
		n := len(r.ns)
		med := r.ns[n/2]
		if n%2 == 0 {
			med = (r.ns[n/2-1] + r.ns[n/2]) / 2
		}
		spread := (r.ns[n-1] - r.ns[0]) / med * 100
		fmt.Printf("| %s | %s | %s | %s | %.0f%% |\n", name, format(med), r.b, r.all, spread)
	}
}

func format(ns float64) string {
	switch {
	case ns < 100:
		return fmt.Sprintf("%.1f ns", ns)
	case ns < 1000:
		return fmt.Sprintf("%.0f ns", ns)
	case ns < 1e6:
		return fmt.Sprintf("%.1f µs", ns/1e3)
	}
	return fmt.Sprintf("%.2f ms", ns/1e6)
}
