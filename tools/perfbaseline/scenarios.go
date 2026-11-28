package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// scenario is one complete workflow: a dirloom command line run in a fresh
// process, exactly as cmd/dirloom runs it (cli.Execute), or a watch driven
// by bursts of changes.
type scenario struct {
	Name     string
	Priority string // critical or high, from the LOT 6 framing (plan §3.4)
	Group    string
	About    string
	Needs    []string // fixtures
	// Dir is the working directory, relative to the fixtures; "" is the
	// fixture directory itself.
	Dir string
	// Args are the dirloom arguments; "@" starts a path under the fixtures.
	Args    []string
	Exits   []int  // accepted exit statuses
	Expect  string // a substring the start of stdout must hold, if any
	Timeout time.Duration
	Watch   *watchParams
}

// watchParams drive a watch: after the startup, a burst of changes every
// Interval until Duration, each ending with a sentinel file whose arrival
// in a state.changed is the convergence of the burst.
type watchParams struct {
	Duration time.Duration
	Interval time.Duration
	Files    int // files created in a new directory by each burst
}

func (s scenario) args(root string) []string {
	out := make([]string, len(s.Args))
	for i, a := range s.Args {
		out[i] = a
		if strings.HasPrefix(a, "@") {
			out[i] = filepath.Join(root, filepath.FromSlash(a[1:]))
		} else if prefix, path, ok := strings.Cut(a, ":@"); ok {
			out[i] = prefix + ":" + filepath.Join(root, filepath.FromSlash(path))
		}
	}
	return out
}

func (s scenario) dir(root string) string {
	return filepath.Join(root, filepath.FromSlash(s.Dir))
}

// fixturePaths returns what the scenario reads under the fixtures: its
// working directory and every path argument.
func (s scenario) fixturePaths(root string) []string {
	var out []string
	if s.Dir != "" {
		out = append(out, s.dir(root))
	}
	for _, a := range s.Args {
		if strings.HasPrefix(a, "@") {
			out = append(out, filepath.Join(root, filepath.FromSlash(a[1:])))
		} else if _, path, ok := strings.Cut(a, ":@"); ok {
			out = append(out, filepath.Join(root, filepath.FromSlash(path)))
		}
	}
	return out
}

func scenarios() []scenario {
	diffJSON := func(a, b string, extra ...string) []string {
		return append([]string{"diff", "--format", "json", a, b}, extra...)
	}
	list := []scenario{
		// Critical: two large snapshots compared, with and without moves.
		{Name: "snapdiff-100k", Priority: "critical", Group: "snapshot diff", About: "diff snapshot: snapshot:, 100,000 nodes, 10% of directories relocated, moves detected",
			Needs: []string{"snap-wide-100k"}, Args: diffJSON("snapshot:@snap-wide-100k/a.json", "snapshot:@snap-wide-100k/b10.json"), Exits: []int{1}},
		{Name: "snapdiff-100k-nomoves", Priority: "critical", Group: "snapshot diff", About: "the same with --no-moves",
			Needs: []string{"snap-wide-100k"}, Args: diffJSON("snapshot:@snap-wide-100k/a.json", "snapshot:@snap-wide-100k/b10.json", "--no-moves"), Exits: []int{1}},
		{Name: "snapdiff-1m", Priority: "critical", Group: "snapshot diff", About: "diff snapshot: snapshot:, 1,000,000 nodes, 10% relocated, moves detected",
			Needs: []string{"snap-wide-1m"}, Args: diffJSON("snapshot:@snap-wide-1m/a.json", "snapshot:@snap-wide-1m/b10.json"), Exits: []int{1}},
		{Name: "snapdiff-1m-nomoves", Priority: "critical", Group: "snapshot diff", About: "the same with --no-moves",
			Needs: []string{"snap-wide-1m"}, Args: diffJSON("snapshot:@snap-wide-1m/a.json", "snapshot:@snap-wide-1m/b10.json", "--no-moves"), Exits: []int{1}},
		{Name: "snapdiff-2m", Priority: "critical", Group: "snapshot diff", About: "diff snapshot: snapshot:, 2,000,000 nodes (the limit), 10% relocated, moves detected",
			Needs: []string{"snap-wide-2m"}, Args: diffJSON("snapshot:@snap-wide-2m/a.json", "snapshot:@snap-wide-2m/b10.json"), Exits: []int{1}},
		{Name: "snapdiff-2m-nomoves", Priority: "critical", Group: "snapshot diff", About: "the same with --no-moves",
			Needs: []string{"snap-wide-2m"}, Args: diffJSON("snapshot:@snap-wide-2m/a.json", "snapshot:@snap-wide-2m/b10.json", "--no-moves"), Exits: []int{1}},
		{Name: "snapdiff-2m-b50", Priority: "critical", Group: "snapshot diff", About: "2,000,000 nodes, 50% of directories relocated, moves detected",
			Needs: []string{"snap-wide-2m"}, Args: diffJSON("snapshot:@snap-wide-2m/a.json", "snapshot:@snap-wide-2m/b50.json"), Exits: []int{1}},
		{Name: "snapdiff-2m-b50-nomoves", Priority: "critical", Group: "snapshot diff", About: "the same with --no-moves: 1,000,000 removed and 1,000,000 added",
			Needs: []string{"snap-wide-2m"}, Args: diffJSON("snapshot:@snap-wide-2m/a.json", "snapshot:@snap-wide-2m/b50.json", "--no-moves"), Exits: []int{1}},
		{Name: "snapdiff-long-2m", Priority: "critical", Group: "snapshot diff", About: "two 252 MiB documents of 2,000,000 long-named nodes, 10% relocated, moves detected",
			Needs: []string{"snap-long-2m"}, Args: diffJSON("snapshot:@snap-long-2m/a.json", "snapshot:@snap-long-2m/b10.json"), Exits: []int{1}},

		// Critical: malformed inputs close to the limits.
		{Name: "bad-keys", Priority: "critical", Group: "malformed input", About: "verify a 250 MiB object of unique keys (finding F6)",
			Needs: []string{"bad", "live-empty"}, Args: []string{"verify", "--format", "json", "@bad/keys.json", "@live-empty"}, Exits: []int{3}, Expect: `"code": "missing_snapshot_field"`},
		{Name: "bad-keys-after-2m", Priority: "critical", Group: "malformed input", About: "diff a valid 2,000,000-node snapshot, then the 250 MiB object of unique keys",
			Needs: []string{"bad", "snap-wide-2m"}, Args: diffJSON("snapshot:@snap-wide-2m/a.json", "snapshot:@bad/keys.json"), Exits: []int{3}, Expect: `"code": "missing_snapshot_field"`},
		{Name: "bad-nodes-over", Priority: "critical", Group: "malformed input", About: "verify a snapshot of 2,000,001 nodes, one above the limit",
			Needs: []string{"bad", "live-empty"}, Args: []string{"verify", "--format", "json", "@bad/nodes-over.json", "@live-empty"}, Exits: []int{3}, Expect: `"snapshot_limit_exceeded"`},
		{Name: "bad-bytes-over", Priority: "critical", Group: "malformed input", About: "verify a 271 MiB snapshot, above the 256 MiB limit",
			Needs: []string{"bad", "live-empty"}, Args: []string{"verify", "--format", "json", "@bad/bytes-over.json", "@live-empty"}, Exits: []int{3}, Expect: `"snapshot_limit_exceeded"`},
		{Name: "bad-nesting", Priority: "critical", Group: "malformed input", About: "verify 3,000,000 nested arrays (6 MB)",
			Needs: []string{"bad", "live-empty"}, Args: []string{"verify", "--format", "json", "@bad/nesting.json", "@live-empty"}, Exits: []int{3}, Expect: `"snapshot_limit_exceeded"`},
		{Name: "limit-ignore-4096", Priority: "critical", Group: "malformed input", About: "verify a snapshot with 4,096 capture.ignore patterns (the limit) against 100,000 live nodes",
			Needs: []string{"bad", "live-wide-100k"}, Args: []string{"verify", "--format", "json", "@bad/ignore-4096.json", "@live-wide-100k"}, Exits: []int{1}},
		{Name: "limit-gitignore-short", Priority: "critical", Group: "malformed input", About: "fingerprint 100,000 live nodes under a 1 MiB .gitignore of short patterns",
			Needs: []string{"live-gi-short"}, Args: []string{"fingerprint", "--format", "json", "@live-gi-short"}, Exits: []int{0}},
		{Name: "limit-gitignore-long", Priority: "critical", Group: "malformed input", About: "fingerprint 100,000 live nodes under a .gitignore of 16 lines of 65,000 bytes",
			Needs: []string{"live-gi-long"}, Args: []string{"fingerprint", "--format", "json", "@live-gi-long"}, Exits: []int{0}},
		{Name: "limit-gitignore-wild", Priority: "critical", Group: "malformed input", About: "the same with lines of alternating wildcards",
			Needs: []string{"live-gi-wild"}, Args: []string{"fingerprint", "--format", "json", "@live-gi-wild"}, Exits: []int{0}},

		// High: loading a snapshot.
		{Name: "load-2m", Priority: "high", Group: "snapshot load", About: "verify a 2,000,000-node snapshot against an empty directory: the load dominates",
			Needs: []string{"snap-wide-2m", "live-empty"}, Args: []string{"verify", "--format", "json", "@snap-wide-2m/a.json", "@live-empty"}, Exits: []int{1}},
		{Name: "load-long-2m", Priority: "high", Group: "snapshot load", About: "the same with the 252 MiB long-named document",
			Needs: []string{"snap-long-2m", "live-empty"}, Args: []string{"verify", "--format", "json", "@snap-long-2m/a.json", "@live-empty"}, Exits: []int{1}},

		// High: snapshot and verify on large live trees.
		{Name: "snapshot-live-1m", Priority: "high", Group: "live tree", About: "snapshot of 1,000,000 live nodes, written to stdout",
			Needs: []string{"live-wide-1m"}, Args: []string{"snapshot", "@live-wide-1m"}, Exits: []int{0}},
		{Name: "snapshot-live-2m", Priority: "high", Group: "live tree", About: "snapshot of 2,000,000 live nodes, written to stdout",
			Needs: []string{"live-wide-2m"}, Args: []string{"snapshot", "@live-wide-2m"}, Exits: []int{0}},
		{Name: "verify-live-100k", Priority: "high", Group: "live tree", About: "verify 100,000 live nodes against their snapshot (match)",
			Needs: []string{"live-wide-100k", "snaplive"}, Args: []string{"verify", "--format", "json", "@snaplive/wide-100k.json", "@live-wide-100k"}, Exits: []int{0}},
		{Name: "verify-live-1m", Priority: "high", Group: "live tree", About: "verify 1,000,000 live nodes against their snapshot (match)",
			Needs: []string{"live-wide-1m", "snaplive"}, Args: []string{"verify", "--format", "json", "@snaplive/wide-1m.json", "@live-wide-1m"}, Exits: []int{0}},
		{Name: "verify-live-2m", Priority: "high", Group: "live tree", About: "verify 2,000,000 live nodes against their snapshot (match)",
			Needs: []string{"live-wide-2m", "snaplive"}, Args: []string{"verify", "--format", "json", "@snaplive/wide-2m.json", "@live-wide-2m"}, Exits: []int{0}},
		{Name: "verify-live-deep-100k", Priority: "high", Group: "live tree", About: "verify 100,001 live nodes in 100 chains of 50 directories (match)",
			Needs: []string{"live-deep-100k", "snaplive"}, Args: []string{"verify", "--format", "json", "@snaplive/deep-100k.json", "@live-deep-100k"}, Exits: []int{0}},
		{Name: "difflive-2m", Priority: "high", Group: "live tree", About: "diff snapshot: of 2,000,000 nodes against the live tree (no change)",
			Needs: []string{"live-wide-2m", "snaplive"}, Args: diffJSON("snapshot:@snaplive/wide-2m.json", "live:@live-wide-2m"), Exits: []int{0}},

		// High: large Git observations.
		{Name: "gitdiff-1m", Priority: "high", Group: "Git", About: "diff git:HEAD~1 git:HEAD, 1,000,000 nodes, 10% relocated, moves detected",
			Needs: []string{"git-wide-1m"}, Dir: "git-wide-1m", Args: diffJSON("git:HEAD~1", "git:HEAD"), Exits: []int{1}},
		{Name: "gitdiff-2m", Priority: "high", Group: "Git", About: "diff git:HEAD~1 git:HEAD, 2,000,000 nodes (the Git limit), 10% relocated, moves detected",
			Needs: []string{"git-wide-2m"}, Dir: "git-wide-2m", Args: diffJSON("git:HEAD~1", "git:HEAD"), Exits: []int{1}},
		{Name: "gitdiff-2m-nomoves", Priority: "high", Group: "Git", About: "the same with --no-moves",
			Needs: []string{"git-wide-2m"}, Dir: "git-wide-2m", Args: diffJSON("git:HEAD~1", "git:HEAD", "--no-moves"), Exits: []int{1}},
		{Name: "snapgitdiff-2m", Priority: "high", Group: "Git", About: "diff snapshot: git:HEAD, 2,000,000 nodes on each side",
			Needs: []string{"git-wide-2m", "snap-wide-2m"}, Dir: "git-wide-2m", Args: diffJSON("snapshot:@snap-wide-2m/a.json", "git:HEAD"), Exits: []int{1}},
		{Name: "git-bomb", Priority: "high", Group: "Git", About: "history --limit 1 on a tree of 2,306,353 shared nodes: refused at the node limit",
			Needs: []string{"git-bomb"}, Dir: "git-bomb", Args: []string{"history", "--limit", "1", "--format", "json"}, Exits: []int{5}, Expect: `"git_tree_too_large"`},
		{Name: "git-chain", Priority: "high", Group: "Git", About: "diff two chains of 16,382 directories, just under 256 MiB of paths each",
			Needs: []string{"git-chain"}, Dir: "git-chain", Args: diffJSON("git:main~1", "git:main"), Exits: []int{1}},
		{Name: "git-chain-over", Priority: "high", Group: "Git", About: "diff the chain under the limit with one of 16,385 directories, above it: refused",
			Needs: []string{"git-chain"}, Dir: "git-chain", Args: diffJSON("git:main", "git:over"), Exits: []int{5}, Expect: `"git_tree_too_large"`},

		// High: a deep Git history.
		{Name: "history-10k-2000", Priority: "high", Group: "history", About: "history --limit 2000 over a tree of about 10,000 entries",
			Needs: []string{"git-history-10k"}, Dir: "git-history-10k", Args: []string{"history", "--limit", "2000", "--format", "json"}, Exits: []int{0}},
		{Name: "history-100k-200", Priority: "high", Group: "history", About: "history --limit 200 over a tree of about 100,000 entries",
			Needs: []string{"git-history-100k"}, Dir: "git-history-100k", Args: []string{"history", "--limit", "200", "--format", "json"}, Exits: []int{0}},

		// High: a long watch with bursts of changes.
		{Name: "watch-bursts-100k", Priority: "high", Group: "watch", About: "native watch of 100,000 nodes for 5 minutes, a burst of about 800 operations every 15 s",
			Needs: []string{"live-watch"}, Args: []string{"watch", "--format", "ndjson", "@live-watch"}, Exits: []int{0},
			Watch: &watchParams{Duration: 5 * time.Minute, Interval: 15 * time.Second, Files: 400}},
	}
	for i := range list {
		if list[i].Timeout == 0 {
			list[i].Timeout = 20 * time.Minute
		}
	}
	return list
}

func findScenario(name string) (scenario, error) {
	for _, s := range scenarios() {
		if s.Name == name {
			return s, nil
		}
	}
	return scenario{}, fmt.Errorf("unknown scenario %q", name)
}
