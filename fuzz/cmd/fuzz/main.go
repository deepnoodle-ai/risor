// Command fuzz is the differential fuzzer of Risor v3. It generates case
// files, runs each in every implementation through its outcome command
// (suite/README.md), and reports every difference. See ../../README.md.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
	"github.com/deepnoodle-ai/risor/fuzz/internal/impl"
	"github.com/deepnoodle-ai/risor/fuzz/internal/outcome"
	"github.com/deepnoodle-ai/risor/fuzz/internal/shrink"
)

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ";") }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }

type config struct {
	seed       uint64
	cases      int
	rounds     int
	impls      []*impl.Impl
	timeout    time.Duration
	out        string
	shrink     bool
	maxPerKind int
	opts       gen.Options
	lib        *gen.Library
	libNames   map[string]bool
	tmp        string
	verbose    bool
	useKnown   bool
	lacks      bool // the probe found an entry that an implementation lacks
	onlyRound  int
	onlyIndex  int
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		seed       = flag.Uint64("seed", 1, "the seed; the same seed gives the same cases")
		cases      = flag.Int("cases", 100, "cases in each round (one case file a round)")
		rounds     = flag.Int("rounds", 10, "rounds")
		implList   = flag.String("impls", "go,ts,rust", "implementations to run, by name")
		typed      = flag.Int("typed", 5, "weight of the typed mode")
		loose      = flag.Int("loose", 3, "weight of the loose mode")
		mangled    = flag.Int("mangled", 2, "weight of the mangled mode")
		limitShare = flag.Int("limits", 25, "percent of cases with a small limit; -1 for none")
		noCheck    = flag.Int("nocheck", 20, "percent of cases with \"check\": false")
		timeout    = flag.Duration("timeout", 60*time.Second, "timeout of one command on one file")
		out        = flag.String("out", "findings", "directory for findings")
		root       = flag.String("root", "", "repository root (found from the working directory)")
		emit       = flag.String("emit", "", "write the case files to this directory and run nothing")
		replay     = flag.String("replay", "", "run one finding, or one case file, and print each line")
		noShrink   = flag.Bool("noshrink", false, "do not shrink findings")
		noProbe    = flag.Bool("noprobe", false, "do not probe which library entries each implementation has")
		maxPerKind = flag.Int("max", 3, "findings written for each distinct difference")
		exclude    = flag.String("exclude", "", "library entries not to call, by name, comma separated")
		useKnown   = flag.Bool("known", true, "count the known differences apart (see known.go)")
		noF3       = flag.Bool("nof3", false, "leave out the string forms of package F3")
		noF4       = flag.Bool("nof4", false, "leave out the forms of packages F4 and F5")
		noF6       = flag.Bool("nof6", false, "leave out the opt-in groups of package F6")
		noF7       = flag.Bool("nof7", false, "leave out the projection and the methods of package F7")
		soundMode  = flag.Bool("sound", false, "the soundness mode: test the promise of 05-types.md 18.4 (-cases is programs a round)")
		only       = flag.String("case", "", "R:I: make and run only case I of round R (with -seed and the mode weights)")
		verbose    = flag.Bool("v", false, "print each round, and the outcomes by code")
		noBuild    = flag.Bool("nobuild", false, "do not build; run the programs built before")
		cmds       multiFlag
		builds     multiFlag
	)
	flag.Var(&cmds, "cmd", "name=command: the outcome command of an implementation, run in its directory (repeatable)")
	flag.Var(&builds, "build", "name=command: a build command run once first; empty for none (repeatable)")
	flag.Parse()

	libPath, err := gen.FindLibrary(".")
	if *root != "" {
		libPath = filepath.Join(*root, "suite", "library.json")
	}
	if err != nil && *root == "" {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		return 2
	}
	repo := filepath.Dir(filepath.Dir(libPath))
	lib, err := gen.LoadLibrary(libPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		return 2
	}
	cfg := &config{
		seed: *seed, cases: *cases, rounds: *rounds, timeout: *timeout,
		out: *out, shrink: !*noShrink, maxPerKind: *maxPerKind, lib: lib,
		verbose: *verbose, libNames: map[string]bool{}, useKnown: *useKnown,
		opts: gen.Options{Typed: *typed, Loose: *loose, Mangled: *mangled, LimitShare: *limitShare, NoCheckShare: *noCheck, NoF3: *noF3, NoF4: *noF4, NoF6: *noF6, NoF7: *noF7, Exclude: map[string]bool{}},
	}
	for _, e := range lib.Entries {
		cfg.libNames[e.Name] = true
	}
	cfg.onlyRound = -1
	if *only != "" {
		if _, err := fmt.Sscanf(*only, "%d:%d", &cfg.onlyRound, &cfg.onlyIndex); err != nil {
			fmt.Fprintln(os.Stderr, "fuzz: -case wants R:I")
			return 2
		}
		cfg.rounds = cfg.onlyRound + 1
	}
	for _, n := range strings.Split(*exclude, ",") {
		if n = strings.TrimSpace(n); n != "" {
			cfg.opts.Exclude[n] = true
		}
	}

	if *emit != "" {
		return emitFiles(cfg, *emit)
	}

	cfg.tmp, err = os.MkdirTemp("", "risor-fuzz-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		return 2
	}
	defer os.RemoveAll(cfg.tmp)
	defs := impl.Defaults(repo, cfg.tmp)
	for _, c := range cmds {
		name, cmd, _ := strings.Cut(c, "=")
		if defs[name] == nil {
			defs[name] = &impl.Impl{Name: name, Dir: repo}
		}
		defs[name].Cmd = strings.Fields(cmd)
	}
	for _, b := range builds {
		name, cmd, _ := strings.Cut(b, "=")
		if defs[name] != nil {
			defs[name].Build = strings.Fields(cmd)
		}
	}
	for _, n := range strings.Split(*implList, ",") {
		n = strings.TrimSpace(n)
		if defs[n] == nil {
			fmt.Fprintf(os.Stderr, "fuzz: no implementation %q\n", n)
			return 2
		}
		cfg.impls = append(cfg.impls, defs[n])
	}
	for _, im := range cfg.impls {
		if *noBuild && im.Name == "go" {
			// The Go program is built into a temporary directory; without a
			// build, run it from source.
			im.Cmd = []string{"go", "run", "./cmd/outcome"}
		}
		if *noBuild {
			continue
		}
		if err := im.Prepare(10 * time.Minute); err != nil {
			if _, serr := os.Stat(im.Cmd[0]); serr == nil && filepath.IsAbs(im.Cmd[0]) {
				fmt.Fprintf(os.Stderr, "fuzz: %s did not build; running the program built before\n", im.Name)
				continue
			}
			fmt.Fprintln(os.Stderr, "fuzz:", err)
			return 2
		}
	}
	if *replay != "" {
		return replayFile(cfg, *replay)
	}
	if !*noProbe {
		probe(cfg, libPath)
	}
	if *soundMode {
		return soundness(cfg)
	}
	return fuzz(cfg)
}

func emitFiles(cfg *config, dir string) int {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		return 2
	}
	for r := range cfg.rounds {
		cases := generateRound(cfg, r)
		path := filepath.Join(dir, fmt.Sprintf("%d-%d.json", cfg.seed, r))
		if err := os.WriteFile(path, gen.File(cases), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "fuzz:", err)
			return 2
		}
	}
	return 0
}

func generateRound(cfg *config, round int) []*gen.Case {
	if cfg.onlyRound >= 0 {
		if round != cfg.onlyRound {
			return nil
		}
		return []*gen.Case{gen.Generate(cfg.lib, cfg.opts, cfg.seed, round, cfg.onlyIndex)}
	}
	out := make([]*gen.Case, cfg.cases)
	for i := range out {
		out[i] = gen.Generate(cfg.lib, cfg.opts, cfg.seed, round, i)
	}
	return out
}

// res is what one implementation gave for one case: a line, or a failure.
type res struct {
	line *outcome.Line
	fail string
}

// runImpl runs one implementation on a set of cases. When the command
// fails as a whole, it splits the set in two and runs each half, down to
// the one case that fails.
func runImpl(cfg *config, im *impl.Impl, cases []*gen.Case) []res {
	out := make([]res, len(cases))
	path := filepath.Join(cfg.tmp, fmt.Sprintf("%s-%d.json", im.Name, time.Now().UnixNano()))
	if err := os.WriteFile(path, gen.File(cases), 0o644); err != nil {
		panic(err)
	}
	defer os.Remove(path)
	r, err := im.Run(path, cfg.timeout)
	if err != nil {
		for i := range out {
			out[i].fail = err.Error()
		}
		return out
	}
	lines, perr := outcome.ParseAll(r.Stdout)
	if r.Failed() || perr != nil {
		if len(cases) == 1 {
			switch {
			case r.Failed():
				out[0].fail = r.Describe()
			default:
				out[0].fail = "bad line: " + perr.Error()
			}
			return out
		}
		h := len(cases) / 2
		return append(runImpl(cfg, im, cases[:h]), runImpl(cfg, im, cases[h:])...)
	}
	byName := map[string]*outcome.Line{}
	for _, l := range lines {
		byName[l.Name] = l
	}
	for i, c := range cases {
		if l := byName[c.Name]; l != nil {
			out[i].line = l
		} else {
			out[i].fail = "missing line"
		}
	}
	return out
}

func runAll(cfg *config, cases []*gen.Case) [][]res {
	out := make([][]res, len(cfg.impls))
	done := make(chan int)
	for i, im := range cfg.impls {
		go func() {
			out[i] = runImpl(cfg, im, cases)
			done <- i
		}()
	}
	for range cfg.impls {
		<-done
	}
	return out
}

// signature names a difference: for each pair of implementations, the
// first field where their lines differ. It is "" when all agree.
func signature(cfg *config, rs []res) string {
	var parts []string
	any := false
	for i := range rs {
		for j := i + 1; j < len(rs); j++ {
			d := ""
			switch {
			case rs[i].fail != "" || rs[j].fail != "":
				if rs[i].fail != "" {
					d = "fail:" + failKind(rs[i].fail)
				}
				if rs[j].fail != "" {
					d += "/fail:" + failKind(rs[j].fail)
				}
			default:
				d = outcome.Diff(rs[i].line, rs[j].line)
			}
			if d != "" {
				any = true
			}
			parts = append(parts, cfg.impls[i].Name+"-"+cfg.impls[j].Name+"="+d)
		}
	}
	if !any {
		return ""
	}
	return strings.Join(parts, " ")
}

func failKind(f string) string {
	switch {
	case f == "timeout", f == "missing line":
		return f
	case strings.HasPrefix(f, "bad line"):
		return "bad-line"
	}
	return "status"
}

// kindKey names a difference for grouping. When every implementation
// failed the check, the key names, for each implementation, the codes of
// the diagnostics that no other implementation gave: that is closer to a
// cause than the counts. Otherwise it names what each implementation gave.
func kindKey(cfg *config, rs []res, sig string) string {
	allCheck := true
	for _, r := range rs {
		if r.line == nil || r.line.Phase != "check" {
			allCheck = false
		}
	}
	if allCheck {
		var parts []string
		for i, r := range rs {
			var codes []string
			seen := map[string]bool{}
			for _, d := range r.line.Diags {
				alone := true
				for j, o := range rs {
					if j == i {
						continue
					}
					for _, e := range o.line.Diags {
						if outcome.Equal(d["code"], e["code"]) && outcome.Equal(d["span"], e["span"]) {
							alone = false
						}
					}
				}
				c, _ := d["code"].(string)
				if alone && !seen[c] {
					seen[c] = true
					codes = append(codes, c)
				}
			}
			if len(codes) > 0 {
				sort.Strings(codes)
				parts = append(parts, cfg.impls[i].Name+" alone: "+strings.Join(codes, ","))
			}
		}
		if len(parts) > 0 {
			return "check | " + strings.Join(parts, "; ")
		}
	}
	var parts []string
	for i, r := range rs {
		k := "fail:" + failKind(r.fail)
		if r.fail == "" {
			k = r.line.Kind()
		}
		parts = append(parts, cfg.impls[i].Name+"="+k)
	}
	return strings.Join(parts, " ") + " | " + sig
}

// missing says whether a difference comes from a library entry that one
// implementation does not have yet.
func missing(cfg *config, c *gen.Case, rs []res) string {
	for _, r := range rs {
		if r.line != nil {
			if n := r.line.MissingEntry(c.Text(), cfg.libNames); n != "" {
				return n
			}
		}
	}
	return ""
}

type stats map[gen.Mode]map[string]map[string]int

func (s stats) add(m gen.Mode, im, kind string) {
	if s[m] == nil {
		s[m] = map[string]map[string]int{}
	}
	if s[m][im] == nil {
		s[m][im] = map[string]int{}
	}
	s[m][im][kind]++
}

func fuzz(cfg *config) int {
	st := stats{}
	groups := map[string]int{}
	var order []string
	missingCount := map[string]int{}
	codes := map[string]int{}
	knownCount := map[string]int{}
	findings := 0
	total := 0
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		return 2
	}
	for round := range cfg.rounds {
		cases := generateRound(cfg, round)
		if len(cases) == 0 {
			continue
		}
		results := runAll(cfg, cases)
		for i, c := range cases {
			total++
			rs := make([]res, len(cfg.impls))
			for k := range cfg.impls {
				rs[k] = results[k][i]
				kind := "fail"
				if rs[k].line != nil {
					kind = rs[k].line.Phase
					if kind == "" {
						kind = "value"
					}
				}
				st.add(c.Mode, cfg.impls[k].Name, kind)
				if k == 0 && rs[k].line != nil {
					codes[string(c.Mode)+" "+rs[k].line.Kind()]++
				}
			}
			sig := signature(cfg, rs)
			if sig == "" {
				continue
			}
			if n := missing(cfg, c, rs); cfg.lacks && n != "" {
				missingCount[n]++
				continue
			}
			if cfg.useKnown {
				if k := known(c, rs); k != "" {
					knownCount[k]++
					continue
				}
			}
			key := kindKey(cfg, rs, sig)
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
					for j := range cands {
						row := make([]res, len(cfg.impls))
						for k := range cfg.impls {
							row[k] = rr[k][j]
						}
						out[j] = signature(cfg, row)
					}
					return out
				}, shrink.Options{})
				small.Name = c.Name
			}
			final := runAll(cfg, []*gen.Case{small})
			row := make([]res, len(cfg.impls))
			for k := range cfg.impls {
				row[k] = final[k][0]
			}
			path := filepath.Join(cfg.out, fmt.Sprintf("%d-%d.json", cfg.seed, findings))
			writeFinding(cfg, path, round, i, c, small, row, sig)
			fmt.Printf("finding %s: %s %s: %s\n", path, c.Mode, oneLine(small.Text(), 60), key)
		}
		if cfg.verbose {
			fmt.Printf("round %d: %d cases, %d findings so far\n", round, total, findings)
		}
	}
	printStats(cfg, st)
	if cfg.verbose {
		var ks []string
		for k := range codes {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		fmt.Printf("outcomes in %s:\n", cfg.impls[0].Name)
		for _, k := range ks {
			fmt.Printf("  %6d  %s\n", codes[k], k)
		}
	}
	if len(missingCount) > 0 {
		var names []string
		for n := range missingCount {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Println("differences from a library entry one implementation lacks (not findings):")
		for _, n := range names {
			fmt.Printf("  %-12s %d\n", n, missingCount[n])
		}
	}
	if len(knownCount) > 0 {
		var ks []string
		for k := range knownCount {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		fmt.Println("known differences, by package offered (not findings):")
		for _, k := range ks {
			fmt.Printf("  %6d  %s\n", knownCount[k], k)
		}
	}
	if len(order) > 0 {
		fmt.Println("distinct differences:")
		for _, k := range order {
			fmt.Printf("  %5d  %s\n", groups[k], k)
		}
	}
	fmt.Printf("%d cases, %d findings, %d distinct\n", total, findings, len(order))
	if findings > 0 {
		return 1
	}
	return 0
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return fmt.Sprintf("%q...", string(r[:n]))
	}
	return fmt.Sprintf("%q", s)
}

func printStats(cfg *config, st stats) {
	kinds := []string{"value", "run", "check", "parse", "declare", "fail"}
	fmt.Printf("%-8s %-5s", "mode", "impl")
	for _, k := range kinds {
		fmt.Printf(" %8s", k)
	}
	fmt.Println()
	for _, m := range []gen.Mode{gen.Typed, gen.Loose, gen.Mangled} {
		for _, im := range cfg.impls {
			c := st[m][im.Name]
			n := 0
			for _, v := range c {
				n += v
			}
			if n == 0 {
				continue
			}
			fmt.Printf("%-8s %-5s", m, im.Name)
			for _, k := range kinds {
				fmt.Printf(" %7.1f%%", 100*float64(c[k])/float64(n))
			}
			fmt.Printf("  of %d\n", n)
		}
	}
}

func writeFinding(cfg *config, path string, round, index int, orig, small *gen.Case, rs []res, sig string) {
	var b bytes.Buffer
	b.WriteString("{\n")
	fmt.Fprintf(&b, "  \"seed\": %d,\n  \"round\": %d,\n  \"index\": %d,\n  \"mode\": %q,\n", cfg.seed, round, index, orig.Mode)
	sj, _ := json.Marshal(sig)
	fmt.Fprintf(&b, "  \"difference\": %s,\n", sj)
	fmt.Fprintf(&b, "  \"case\": %s,\n", small.JSON())
	b.WriteString("  \"lines\": {\n")
	for i, r := range rs {
		raw := r.fail
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
	b.WriteString("  },\n")
	fmt.Fprintf(&b, "  \"original\": %s\n}\n", orig.JSON())
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
	}
}

// replayFile runs a finding's case, or a whole case file, in each
// implementation and prints each line.
func replayFile(cfg *config, path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		return 2
	}
	var probe map[string]json.RawMessage
	file := path
	if json.Unmarshal(data, &probe) == nil && probe["case"] != nil {
		file = filepath.Join(cfg.tmp, "replay.json")
		_ = os.WriteFile(file, append(append([]byte("["), probe["case"]...), ']'), 0o644)
	}
	status := 0
	var all [][]*outcome.Line
	for _, im := range cfg.impls {
		r, err := im.Run(file, cfg.timeout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fuzz:", err)
			return 2
		}
		fmt.Printf("== %s\n%s", im.Name, r.Stdout)
		if r.Failed() {
			fmt.Printf("(%s)\n", r.Describe())
			status = 1
		}
		lines, _ := outcome.ParseAll(r.Stdout)
		all = append(all, lines)
	}
	for i := 1; i < len(all); i++ {
		for j := range all[0] {
			if j < len(all[i]) {
				if d := outcome.Diff(all[0][j], all[i][j]); d != "" {
					fmt.Printf("%s: %s and %s differ in %s\n", all[0][j].Name, cfg.impls[0].Name, cfg.impls[i].Name, d)
					status = 1
				}
			}
		}
	}
	return status
}

// probe runs the cases of library.json in each implementation and leaves
// out of generation every entry that one implementation reports it does
// not have.
func probe(cfg *config, libPath string) {
	data, err := os.ReadFile(libPath)
	if err != nil {
		return
	}
	var raw []struct {
		Name  string            `json:"name"`
		Cases []json.RawMessage `json:"cases"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return
	}
	var b bytes.Buffer
	b.WriteString("[")
	var srcs []string
	var owner []string
	n := 0
	for ei, e := range raw {
		for ci, c := range e.Cases {
			var m map[string]any
			if json.Unmarshal(c, &m) != nil {
				continue
			}
			m["name"] = fmt.Sprintf("probe-%d-%d", ei, ci)
			delete(m, "skip")
			j, _ := json.Marshal(m)
			if n > 0 {
				b.WriteString(",\n")
			}
			b.Write(j)
			src, _ := m["source"].(string)
			srcs = append(srcs, src)
			owner = append(owner, e.Name)
			n++
		}
	}
	b.WriteString("]\n")
	path := filepath.Join(cfg.tmp, "probe.json")
	_ = os.WriteFile(path, b.Bytes(), 0o644)
	lacks := map[string][]string{}
	// per[i][k] is the entry case k says implementation i lacks, or "".
	per := make([][]string, len(cfg.impls))
	ran := make([]bool, len(cfg.impls))
	for i, im := range cfg.impls {
		per[i] = make([]string, len(srcs))
		r, err := im.Run(path, cfg.timeout)
		if err != nil || r.Failed() {
			fmt.Printf("probe: %s could not run the library cases; no entry left out for it\n", im.Name)
			continue
		}
		ran[i] = true
		lines, _ := outcome.ParseAll(r.Stdout)
		for k, l := range lines {
			if k < len(srcs) {
				per[i][k] = l.MissingEntry(srcs[k], cfg.libNames)
			}
		}
	}
	// A case that every implementation reports the same way is the suite's
	// own (trim on a number); a case only some report shows a lack.
	for k := range srcs {
		for i := range cfg.impls {
			name := per[i][k]
			if !ran[i] || name == "" || name != owner[k] {
				continue
			}
			agree := true
			for j := range cfg.impls {
				if ran[j] && per[j][k] != name {
					agree = false
				}
			}
			if !agree {
				cfg.lacks = true
			}
			if !agree && !cfg.opts.Exclude[name] {
				cfg.opts.Exclude[name] = true
				lacks[cfg.impls[i].Name] = append(lacks[cfg.impls[i].Name], name)
			}
		}
	}
	for _, im := range cfg.impls {
		if len(lacks[im.Name]) > 0 {
			fmt.Printf("probe: %s lacks %s; left out of generation\n", im.Name, strings.Join(lacks[im.Name], " "))
		}
	}
}
