package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
	"github.com/deepnoodle-ai/risor/fuzz/internal/shrink"
	"github.com/deepnoodle-ai/risor/fuzz/internal/sound"
	"github.com/deepnoodle-ai/risor/fuzz/internal/types"
)

// soundness runs the soundness mode: -cases programs a round, each with
// gen.ValueSets value sets, judged against 05-types.md 18.4.
func soundness(cfg *config) int {
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		return 2
	}
	type stat struct {
		programs, clean, careful, carefulClean int
		runs                                   map[string]int
		sites                                  map[string]int
		undecided                              int
		withAs, withIs, withUnion              int // clean programs with an as, an is, a union variable
	}
	st := make([]stat, len(cfg.impls))
	for i := range st {
		st[i].runs, st[i].sites = map[string]int{}, map[string]int{}
	}
	groups := map[string]int{}
	var order []string
	findings, disagree := 0, 0
	for round := range cfg.rounds {
		var progs []*gen.SoundProgram
		var cases []*gen.Case
		for i := range cfg.cases {
			if cfg.onlyRound >= 0 && (round != cfg.onlyRound || i != cfg.onlyIndex) {
				continue
			}
			p := gen.GenerateSound(cfg.lib, cfg.opts, cfg.seed, round, i)
			progs = append(progs, p)
			cases = append(cases, p.Cases...)
		}
		if len(cases) == 0 {
			continue
		}
		results := runAll(cfg, cases)
		at := 0
		for _, p := range progs {
			_, nodes := ast.PrintSpans(p.Cases[0].Prog)
			for k := range cfg.impls {
				st[k].programs++
				if p.Careful {
					st[k].careful++
				}
				first := results[k][at]
				if first.line == nil {
					continue
				}
				if v := sound.Judge(first.line, nodes); v.Clean {
					st[k].clean++
					src := p.Cases[0].Text()
					if strings.Contains(src, " as ") {
						st[k].withAs++
					}
					if strings.Contains(src, " is ") {
						st[k].withIs++
					}
					if strings.Contains(p.Cases[0].VarTypes(), `"`) {
						st[k].withUnion++
					}
					if p.Careful {
						st[k].carefulClean++
					}
					for _, s := range first.line.Sites {
						if m, ok := s.(map[string]any); ok {
							c, _ := m["class"].(string)
							st[k].sites[c]++
						}
					}
				}
			}
			for j, c := range p.Cases {
				row := make([]res, len(cfg.impls))
				for k := range cfg.impls {
					row[k] = results[k][at+j]
				}
				if signature(cfg, row) != "" {
					disagree++
				}
				sig, vs := soundSig(cfg, row, nodes)
				for k, v := range vs {
					if v.Clean {
						st[k].runs[v.Outcome]++
						if v.Undecided != "" {
							st[k].undecided++
						}
					}
				}
				if sig == "" {
					continue
				}
				key := sigKey(sig)
				groups[key]++
				if groups[key] == 1 {
					order = append(order, key)
				}
				findings++
				if groups[key] > cfg.maxPerKind {
					continue
				}
				small := c
				if cfg.shrink {
					small = shrink.Shrink(c, sig, func(cands []*gen.Case) []string {
						for j, k := range cands {
							k.Name = fmt.Sprintf("s%d", j)
						}
						rr := runAll(cfg, cands)
						out := make([]string, len(cands))
						for j, k := range cands {
							if k.Prog == nil || !conformsAll(k) {
								continue
							}
							_, kn := ast.PrintSpans(k.Prog)
							row := make([]res, len(cfg.impls))
							for x := range cfg.impls {
								row[x] = rr[x][j]
							}
							out[j], _ = soundSig(cfg, row, kn)
						}
						return out
					}, shrink.Options{})
					small.Name = c.Name
				}
				final := runAll(cfg, []*gen.Case{small})
				frow := make([]res, len(cfg.impls))
				for k := range cfg.impls {
					frow[k] = final[k][0]
				}
				_, sn := ast.PrintSpans(small.Prog)
				_, svs := soundSig(cfg, frow, sn)
				path := filepath.Join(cfg.out, fmt.Sprintf("sound-%d-%d.json", cfg.seed, findings))
				writeSound(cfg, path, round, p, small, c, frow, svs, sig)
				fmt.Printf("violation %s: %s: %s\n", path, oneLine(small.Text(), 60), key)
			}
			at += len(p.Cases)
		}
		if cfg.verbose {
			fmt.Printf("round %d: %d violations so far\n", round, findings)
		}
	}
	fmt.Println("soundness:")
	for k, im := range cfg.impls {
		s := st[k]
		fmt.Printf("  %s: %d programs, %d checked clean (%d of %d careful, %d of %d careless), %d undecided runs\n",
			im.Name, s.programs, s.clean, s.carefulClean, s.careful, s.clean-s.carefulClean, s.programs-s.careful, s.undecided)
		fmt.Printf("    clean programs with an as %d, with an is %d, with a literal set or union in a declared type %d\n", s.withAs, s.withIs, s.withUnion)
		fmt.Printf("    sites in clean programs: %s\n", counts(s.sites))
		fmt.Printf("    runs of clean programs: %s\n", counts(s.runs))
	}
	fmt.Printf("  %d runs where the implementations' lines differ (the differential mode reports these)\n", disagree)
	if len(order) > 0 {
		fmt.Println("distinct violations:")
		for _, k := range order {
			fmt.Printf("  %5d  %s\n", groups[k], k)
		}
	}
	fmt.Printf("%d violations, %d distinct\n", findings, len(order))
	if findings > 0 {
		return 1
	}
	return 0
}

func counts(m map[string]int) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var parts []string
	for _, k := range ks {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// soundSig judges each implementation's line and names the violations: for
// each implementation that broke a promise, its name, the promise and the
// code. It is "" when every implementation kept the promise.
func soundSig(cfg *config, row []res, nodes []ast.NodeSpan) (string, []sound.Verdict) {
	vs := make([]sound.Verdict, len(row))
	var parts []string
	for k, r := range row {
		if r.line == nil {
			continue
		}
		vs[k] = sound.Judge(r.line, nodes)
		if vs[k].Violation != "" {
			parts = append(parts, fmt.Sprintf("%s:p%d:%s", cfg.impls[k].Name, vs[k].Promise, vs[k].Outcome))
		}
	}
	return strings.Join(parts, " "), vs
}

// sigKey groups violations: shared by every implementation, or not.
func sigKey(sig string) string {
	parts := strings.Fields(sig)
	var impls, rest []string
	for _, p := range parts {
		i := strings.IndexByte(p, ':')
		impls = append(impls, p[:i])
		rest = append(rest, p[i+1:])
	}
	same := true
	for _, r := range rest {
		if r != rest[0] {
			same = false
		}
	}
	if same && len(impls) == 3 {
		return "shared by all: " + rest[0]
	}
	return "in " + strings.Join(impls, ",") + " only: " + strings.Join(rest, " ")
}

// conformsAll says whether each variable's value conforms to its declared
// type, so that a shrunk case still meets the precondition.
func conformsAll(c *gen.Case) bool {
	for _, v := range c.Vars {
		t, err := types.Parse(v.Type)
		if err != nil || !types.Conforms(v.Value, t) {
			return false
		}
	}
	return true
}

func writeSound(cfg *config, path string, round int, p *gen.SoundProgram, small, orig *gen.Case, rs []res, vs []sound.Verdict, sig string) {
	var b bytes.Buffer
	b.WriteString("{\n")
	fmt.Fprintf(&b, "  \"seed\": %d,\n  \"round\": %d,\n  \"program\": %q,\n  \"careful\": %v,\n", cfg.seed, round, p.Name, p.Careful)
	sj, _ := json.Marshal(sig)
	fmt.Fprintf(&b, "  \"violation\": %s,\n", sj)
	fmt.Fprintf(&b, "  \"case\": %s,\n", small.JSON())
	b.WriteString("  \"lines\": {\n")
	for i, r := range rs {
		raw := ""
		if r.line != nil {
			raw = r.line.Raw
		} else {
			j, _ := json.Marshal(map[string]string{"failure": r.fail})
			raw = string(j)
		}
		sep := ","
		if i == len(rs)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "    %q: %s%s\n", cfg.impls[i].Name, raw, sep)
	}
	b.WriteString("  },\n  \"verdicts\": {\n")
	for i, v := range vs {
		j, _ := json.Marshal(v.Violation + v.Undecided)
		sep := ","
		if i == len(vs)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "    %q: %s%s\n", cfg.impls[i].Name, j, sep)
	}
	b.WriteString("  },\n")
	fmt.Fprintf(&b, "  \"original\": %s\n}\n", orig.JSON())
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
	}
}
