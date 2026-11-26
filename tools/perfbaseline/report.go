package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
)

// readResults reads JSON Lines result files.
func readResults(paths []string) ([]result, error) {
	var all []result
	for _, path := range paths {
		file, err := os.Open(path) // #nosec G304 -- a results file given on the command line.
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 1<<16), 1<<24)
		for line := 1; scanner.Scan(); line++ {
			if len(strings.TrimSpace(scanner.Text())) == 0 {
				continue
			}
			var r result
			if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
				_ = file.Close()
				return nil, fmt.Errorf("%s:%d: %w", path, line, err)
			}
			all = append(all, r)
		}
		err = scanner.Err()
		_ = file.Close()
		if err != nil {
			return nil, err
		}
	}
	return all, nil
}

// stat is the distribution of one measure over the successful runs.
type stat struct {
	N                int
	Median, Min, Max float64
}

func distribution(values []float64) stat {
	if len(values) == 0 {
		return stat{}
	}
	sorted := slices.Clone(values)
	sort.Float64s(sorted)
	n := len(sorted)
	median := sorted[n/2]
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return stat{N: n, Median: median, Min: sorted[0], Max: sorted[n-1]}
}

// percentile returns the nearest-rank percentile p (0 < p <= 100).
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	sort.Float64s(sorted)
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	return sorted[min(max(rank, 1), len(sorted))-1]
}

// measure names a value of a result and its unit.
type measure struct {
	Key   string
	Title string
	Scale float64 // divides the raw value
	Get   func(result) float64
}

const mebibyte = 1 << 20

var measures = []measure{
	{"wall", "Time (s)", 1e9, func(r result) float64 { return float64(r.WallNs) }},
	{"heap", "Go heap peak (MiB)", mebibyte, func(r result) float64 { return float64(r.HeapPeak) }},
	{"live", "Live heap peak (MiB)", mebibyte, func(r result) float64 { return float64(r.HeapLivePeak) }},
	{"commit", "Process commit peak (MiB)", mebibyte, func(r result) float64 { return float64(r.PeakPrivate) }},
	{"ws", "Working set peak (MiB)", mebibyte, func(r result) float64 { return float64(r.PeakWorkingSet) }},
	{"job", "Job commit peak (MiB)", mebibyte, func(r result) float64 { return float64(r.JobPeak) }},
	{"alloc", "Allocated (MiB)", mebibyte, func(r result) float64 { return float64(r.AllocBytes) }},
	{"objects", "Allocations (millions)", 1e6, func(r result) float64 { return float64(r.AllocObjects) }},
	{"gc", "GC cycles", 1, func(r result) float64 { return float64(r.GCCycles) }},
	{"out", "Output (MiB)", mebibyte, func(r result) float64 { return float64(r.StdoutBytes) }},
}

func findMeasure(key string) (measure, bool) {
	for _, m := range measures {
		if m.Key == key {
			return m, true
		}
	}
	return measure{}, false
}

// group collects the runs of one scenario under one label.
type group struct {
	Label, Scenario string
	OK, Failed      []result
}

func groupResults(all []result) []*group {
	index := map[string]*group{}
	var groups []*group
	order := map[string]int{}
	for i, s := range scenarios() {
		order[s.Name] = i
	}
	for _, r := range all {
		key := r.Label + "\x00" + r.Scenario
		g := index[key]
		if g == nil {
			g = &group{Label: r.Label, Scenario: r.Scenario}
			index[key] = g
			groups = append(groups, g)
		}
		if r.OK {
			g.OK = append(g.OK, r)
		} else {
			g.Failed = append(g.Failed, r)
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Label != groups[j].Label {
			return groups[i].Label < groups[j].Label
		}
		return order[groups[i].Scenario] < order[groups[j].Scenario]
	})
	return groups
}

func (g *group) stat(m measure) stat {
	values := make([]float64, 0, len(g.OK))
	for _, r := range g.OK {
		values = append(values, m.Get(r)/m.Scale)
	}
	return distribution(values)
}

func formatStat(s stat) string {
	if s.N == 0 {
		return "—"
	}
	digits := func(v float64) string {
		switch {
		case v == 0:
			return "0"
		case math.Abs(v) >= 100:
			return fmt.Sprintf("%.0f", v)
		case math.Abs(v) >= 10:
			return fmt.Sprintf("%.1f", v)
		default:
			return fmt.Sprintf("%.2f", v)
		}
	}
	if s.Min == s.Max {
		return digits(s.Median)
	}
	return fmt.Sprintf("%s (%s–%s)", digits(s.Median), digits(s.Min), digits(s.Max))
}

func runSummarize(args []string) error {
	flags := flag.NewFlagSet("summarize", flag.ContinueOnError)
	label := flags.String("label", "", "only this label")
	columns := flags.String("measures", "wall,heap,live,commit,ws,alloc,objects", "comma-separated measures: "+measureKeys())
	format := flags.String("format", "markdown", "markdown, or bench for benchstat")
	if err := flags.Parse(args); err != nil {
		return err
	}
	all, err := readResults(flags.Args())
	if err != nil {
		return err
	}
	if *label != "" {
		all = slices.DeleteFunc(all, func(r result) bool { return r.Label != *label })
	}
	if *format == "bench" {
		return writeBench(os.Stdout, all)
	}
	var selected []measure
	for _, key := range strings.Split(*columns, ",") {
		m, ok := findMeasure(strings.TrimSpace(key))
		if !ok {
			return fmt.Errorf("unknown measure %q (%s)", key, measureKeys())
		}
		selected = append(selected, m)
	}
	return writeMarkdown(os.Stdout, groupResults(all), selected)
}

func measureKeys() string {
	keys := make([]string, len(measures))
	for i, m := range measures {
		keys[i] = m.Key
	}
	return strings.Join(keys, ",")
}

func writeMarkdown(w io.Writer, groups []*group, selected []measure) error {
	header := "| Label | Scenario | Runs |"
	rule := "| --- | --- | ---: |"
	for _, m := range selected {
		header += " " + m.Title + " |"
		rule += " ---: |"
	}
	if _, err := fmt.Fprintln(w, header+"\n"+rule); err != nil {
		return err
	}
	var watch []*group
	for _, g := range groups {
		row := fmt.Sprintf("| %s | `%s` | %d/%d |", g.Label, g.Scenario, len(g.OK), len(g.OK)+len(g.Failed))
		for _, m := range selected {
			row += " " + formatStat(g.stat(m)) + " |"
		}
		if _, err := fmt.Fprintln(w, row); err != nil {
			return err
		}
		if len(g.OK) > 0 && g.OK[0].Watch != nil {
			watch = append(watch, g)
		}
	}
	if len(watch) > 0 {
		if err := writeWatchMarkdown(w, watch); err != nil {
			return err
		}
	}
	for _, g := range groups {
		for _, r := range g.Failed {
			if _, err := fmt.Fprintf(w, "\n- %s `%s` run %d: exit %d, oom=%t, %s; commit peak %s, job peak %s; stderr: %q\n",
				g.Label, g.Scenario, r.Run, r.Exit, r.OOM, r.Error, mib(r.PeakPrivate), mib(r.JobPeak), firstLine(r.StderrTail)); err != nil {
				return err
			}
		}
	}
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "fatal error") || strings.Contains(line, "out of memory") {
			return strings.TrimSpace(line)
		}
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func writeWatchMarkdown(w io.Writer, groups []*group) error {
	if _, err := fmt.Fprintln(w, "\n| Label | Scenario | Runs | Bursts converged | Operations per burst | Convergence median (ms) | p90 (ms) | max (ms) | Startup (ms) | Retained heap first → last (MiB) | Retained heap max (MiB) | Generations | Errors |\n"+
		"| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |"); err != nil {
		return err
	}
	for _, g := range groups {
		var latencies, startups []float64
		var first, last, high float64
		bursts, converged, ops, generations, errs := 0, 0, 0, 0, 0
		for i, r := range g.OK {
			wr := r.Watch
			for _, l := range wr.LatenciesNs {
				latencies = append(latencies, float64(l)/1e6)
			}
			startups = append(startups, float64(wr.StartupNs)/1e6)
			bursts += wr.Bursts
			converged += wr.Converged
			ops = wr.OpsPerBurst
			generations += wr.Generations
			errs += wr.Errors
			if len(wr.RetainedHeap) > 0 {
				f := float64(wr.RetainedHeap[0]) / mebibyte
				l := float64(wr.RetainedHeap[len(wr.RetainedHeap)-1]) / mebibyte
				if i == 0 {
					first, last = f, l
				} else {
					first, last = min(first, f), max(last, l)
				}
				for _, h := range wr.RetainedHeap {
					high = max(high, float64(h)/mebibyte)
				}
			}
		}
		lat := distribution(latencies)
		if _, err := fmt.Fprintf(w, "| %s | `%s` | %d | %d/%d | %d | %.0f | %.0f | %.0f | %s | %.1f → %.1f | %.1f | %d | %d |\n",
			g.Label, g.Scenario, len(g.OK), converged, bursts, ops, lat.Median, percentile(latencies, 90), lat.Max,
			formatStat(distribution(startups)), first, last, high, generations, errs); err != nil {
			return err
		}
	}
	return nil
}

// writeBench writes the successful runs in the Go benchmark format, one
// line per run, for benchstat.
func writeBench(w io.Writer, all []result) error {
	if len(all) > 0 {
		if _, err := fmt.Fprintf(w, "goos: %s\ngoarch: %s\npkg: github.com/dirloom/dirloom/tools/perfbaseline\n", all[0].GOOS, all[0].GOARCH); err != nil {
			return err
		}
	}
	for _, r := range all {
		if !r.OK {
			continue
		}
		if _, err := fmt.Fprintf(w, "BenchmarkWorkflow/%s/%s 1 %d ns/op %d B/op %d allocs/op %d heap-peak-B %d commit-peak-B %d ws-peak-B\n",
			r.Label, r.Scenario, r.WallNs, r.AllocBytes, r.AllocObjects, r.HeapPeak, r.PeakPrivate, r.PeakWorkingSet); err != nil {
			return err
		}
	}
	return nil
}

// runCompare compares the medians of two campaigns, scenario by scenario,
// and flags every deviation at or above the investigation threshold (D8:
// 20%). A flag asks for a manual investigation; it never fails anything.
func runCompare(args []string) error {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	base := flags.String("base", "", "results of the reference campaign")
	next := flags.String("new", "", "results of the campaign to compare")
	baseLabel := flags.String("base-label", "baseline", "label of the reference runs")
	newLabel := flags.String("new-label", "baseline", "label of the compared runs")
	threshold := flags.Float64("threshold", 0.20, "investigation threshold, as a fraction of the reference median")
	keys := flags.String("measures", "wall,alloc,heap,commit", "comma-separated measures: "+measureKeys())
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *base == "" || *next == "" {
		return errors.New("compare: -base and -new are required")
	}
	load := func(path, label string) (map[string]*group, error) {
		all, err := readResults([]string{path})
		if err != nil {
			return nil, err
		}
		out := map[string]*group{}
		for _, g := range groupResults(all) {
			if g.Label == label {
				out[g.Scenario] = g
			}
		}
		return out, nil
	}
	reference, err := load(*base, *baseLabel)
	if err != nil {
		return err
	}
	compared, err := load(*next, *newLabel)
	if err != nil {
		return err
	}
	fmt.Println("| Scenario | Measure | Reference median | New median | Change | |\n| --- | --- | ---: | ---: | ---: | --- |")
	flagged := 0
	for _, sc := range scenarios() {
		a, b := reference[sc.Name], compared[sc.Name]
		if a == nil || b == nil {
			continue
		}
		for _, key := range strings.Split(*keys, ",") {
			m, ok := findMeasure(strings.TrimSpace(key))
			if !ok {
				return fmt.Errorf("unknown measure %q", key)
			}
			sa, sb := a.stat(m), b.stat(m)
			if sa.N == 0 || sb.N == 0 || sa.Median == 0 {
				continue
			}
			change := (sb.Median - sa.Median) / sa.Median
			note := ""
			if math.Abs(change) >= *threshold {
				note = "investigate"
				flagged++
			}
			fmt.Printf("| `%s` | %s | %.3g | %.3g | %+.1f%% | %s |\n", sc.Name, m.Title, sa.Median, sb.Median, 100*change, note)
		}
	}
	fmt.Printf("\n%d deviation(s) at or above %.0f%%: each needs a manual investigation, not a rejection.\n", flagged, 100**threshold)
	return nil
}
