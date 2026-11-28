package packaging_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The package-manager updaters order versions with .github/scripts/lib/semver.sh
// (SemVer 2.0.0 precedence) instead of GNU `sort -V`, which put 0.4.0 before
// 0.4.0-rc1: a Scoop manifest at a release candidate refused the stable release
// (anomaly A6 of the v0.4.0-rc1 plan, decision D11).

func bashCommand(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	return exec.Command(bashPath(t), args...)
}

func semverLib(t *testing.T) string {
	t.Helper()
	return filepath.ToSlash(filepath.Join(repoRoot(t), ".github", "scripts", "lib", "semver.sh"))
}

// semverQuery is one call of the library. An empty string stands for an empty
// argument.
type semverQuery struct {
	fn, a, b string
}

type semverAnswer struct {
	out    string
	status int
}

// runSemver runs every query in one bash process and returns the output and
// exit status of each call.
func runSemver(t *testing.T, env []string, queries []semverQuery) []semverAnswer {
	t.Helper()
	var input strings.Builder
	for _, q := range queries {
		// "|" is not whitespace, so an empty field is kept.
		fmt.Fprintf(&input, "%s|%s|%s\n", q.fn, q.a, q.b)
	}
	// The calls run in this shell, with no subshell, as in the updaters.
	driver := `set -u
source "$1"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
while IFS='|' read -r fn a b; do
  status=0
  "$fn" "$a" "$b" >"$tmp" 2>/dev/null || status=$?
  out=""
  IFS= read -r out <"$tmp" || true
  printf '%s %s\n' "$status" "$out"
done`
	cmd := bashCommand(t, "-c", driver, "semver-driver", semverLib(t))
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(input.String())
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("semver driver failed: %v", err)
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n"), "\n")
	if len(lines) != len(queries) {
		t.Fatalf("semver driver answered %d queries, want %d:\n%s", len(lines), len(queries), raw)
	}
	answers := make([]semverAnswer, len(lines))
	for i, line := range lines {
		var status int
		var out string
		if _, err := fmt.Sscanf(line, "%d", &status); err != nil {
			t.Fatalf("unparsable driver line %q", line)
		}
		if _, rest, ok := strings.Cut(line, " "); ok {
			out = rest
		}
		answers[i] = semverAnswer{out: out, status: status}
	}
	return answers
}

// Versions in increasing SemVer precedence: the example of SemVer 2.0.0 §11,
// then every kind of Dirloom version, alphas, release candidates, stable
// releases and patches.
var semverOrdered = [][]string{
	{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0"},
	{
		"0.1.0", "0.1.1", "0.2.0", "0.3.0", "0.3.1", "0.3.2", "0.3.3",
		"0.4.0-a1", "0.4.0-a2", "0.4.0-a3", "0.4.0-a7", "0.4.0-a8",
		"0.4.0-rc1", "0.4.0-rc2",
		"0.4.0", "0.4.1", "0.4.2", "0.4.10",
		"0.5.0-a1", "0.5.0-rc1", "0.5.0", "1.0.0", "2.0.0", "10.0.0",
	},
}

func TestSemverCompareOrdersEveryPair(t *testing.T) {
	var queries []semverQuery
	var want []string
	for _, chain := range semverOrdered {
		for i, a := range chain {
			for j, b := range chain {
				queries = append(queries, semverQuery{"semver_compare", a, b})
				switch {
				case i < j:
					want = append(want, "-1")
				case i > j:
					want = append(want, "1")
				default:
					want = append(want, "0")
				}
			}
		}
	}
	for i, got := range runSemver(t, nil, queries) {
		q := queries[i]
		if got.status != 0 || got.out != want[i] {
			t.Errorf("semver_compare %s %s = %q (status %d), want %q", q.a, q.b, got.out, got.status, want[i])
		}
	}
}

func TestSemverCompareFollowsSemVerRules(t *testing.T) {
	cases := []struct{ a, b, want string }{
		// A6: a release follows its own release candidates.
		{"0.4.0", "0.4.0-rc1", "1"},
		{"0.4.0-rc1", "0.4.0", "-1"},
		// Numeric parts compare as numbers, of any length.
		{"0.4.10", "0.4.9", "1"},
		{"18446744073709551616.0.0", "18446744073709551615.0.0", "1"},
		{"0.0.99999999999999999999", "0.1.0", "-1"},
		// Numeric pre-release identifiers compare as numbers and precede
		// alphanumeric ones.
		{"1.0.0-rc.10", "1.0.0-rc.9", "1"},
		{"1.0.0-1", "1.0.0-a", "-1"},
		{"1.0.0-0a", "1.0.0-1", "1"},
		// Alphanumeric identifiers compare in ASCII order, whatever their
		// digits: under SemVer, rc10 precedes rc9.
		{"0.4.0-rc10", "0.4.0-rc9", "-1"},
		{"0.4.0-a10", "0.4.0-a9", "-1"},
		{"1.0.0-RC1", "1.0.0-a1", "-1"},
		{"1.0.0-a-b", "1.0.0-a", "1"},
		// Build metadata does not count.
		{"1.0.0+build.2", "1.0.0+build.1", "0"},
		{"1.0.0-rc1+x", "1.0.0-rc1", "0"},
		{"1.0.0+x", "1.0.0-rc1", "1"},
	}
	queries := make([]semverQuery, len(cases))
	for i, c := range cases {
		queries[i] = semverQuery{"semver_compare", c.a, c.b}
	}
	for _, env := range [][]string{nil, {"LC_ALL=C"}, {"LC_ALL=en_US.UTF-8"}, {"LC_ALL=C.UTF-8"}} {
		for i, got := range runSemver(t, env, queries) {
			c := cases[i]
			if got.status != 0 || got.out != c.want {
				t.Errorf("%v: semver_compare %s %s = %q (status %d), want %q", env, c.a, c.b, got.out, got.status, c.want)
			}
		}
	}
}

func TestSemverValid(t *testing.T) {
	valid := []string{
		"0.0.0", "0.4.0", "0.4.0-a8", "0.4.0-rc1", "1.0.0-rc.1", "1.0.0-0a", "1.0.0-x-y-z.--",
		"1.0.0+001", "1.0.0-alpha+exp.sha.5114f85", "99999999999999999999.0.0",
	}
	invalid := []string{
		"", "v0.4.0", "0.4", "0.4.0.1", "00.4.0", "0.04.0", "0.4.00", "0.4.0-", "0.4.0-01",
		"0.4.0-rc..1", "0.4.0-rc_1", "0.4.0+", "0.4.0+a..b", "0.4.0-é", " 0.4.0", "0.4.0 ",
		"0.4.0\n", "0.4.0-rc1 0.4.0", "-0.4.0", "0.4.0--", "0.4.0-+x",
	}
	var queries []semverQuery
	for _, v := range valid {
		queries = append(queries, semverQuery{"semver_valid", v, ""})
	}
	for _, v := range invalid {
		if strings.ContainsAny(v, "\t\n") {
			continue // one query per line: covered by the update decision test
		}
		queries = append(queries, semverQuery{"semver_valid", v, ""})
	}
	// "0.4.0--" is valid SemVer: "-" is an alphanumeric identifier.
	isValid := map[string]bool{"0.4.0--": true}
	for _, v := range valid {
		isValid[v] = true
	}
	for i, got := range runSemver(t, nil, queries) {
		v := queries[i].a
		if (got.status == 0) != isValid[v] {
			t.Errorf("semver_valid %q: status %d, want valid=%v", v, got.status, isValid[v])
		}
	}
}

func TestSemverUpdateDecision(t *testing.T) {
	cases := []struct {
		current, next string
		want          string
		status        int
	}{
		{"", "0.3.3", "newer", 0},                 // first manifest
		{"0.3.3", "0.3.3", "same", 0},             // already there
		{"0.3.2", "0.3.3", "newer", 0},            // patch
		{"0.3.3", "0.3.2", "not-newer", 0},        // patch regression
		{"0.3.3", "0.4.0", "newer", 0},            // the expected v0.4.0 update
		{"0.4.0-rc1", "0.4.0", "newer", 0},        // A6: rc to stable
		{"0.4.0", "0.4.0-rc1", "not-newer", 0},    // A6: stable to its rc
		{"0.4.0-a8", "0.4.0-rc1", "newer", 0},     // alpha to rc
		{"0.4.0-rc1", "0.4.0-a8", "not-newer", 0}, // rc to alpha
		{"0.4.0-a7", "0.4.0-a8", "newer", 0},      // alpha to alpha
		{"0.3.3", "0.4.0-rc1", "newer", 0},        // a manual prerelease test
		{"0.4.0", "0.4.1", "newer", 0},
		{"0.4.1", "0.4.0", "not-newer", 0},
		{"0.4.0+a", "0.4.0+b", "not-newer", 0}, // equal precedence
		{"0.3.3", "v0.4.0", "", 2},             // not a version: fail closed
		{"0.3.3", "", "", 2},
		{"latest", "0.4.0", "", 2},
		{"0.4", "0.4.0", "", 2},
	}
	queries := make([]semverQuery, len(cases))
	for i, c := range cases {
		queries[i] = semverQuery{"semver_update_decision", c.current, c.next}
	}
	for i, got := range runSemver(t, nil, queries) {
		c := cases[i]
		if got.out != c.want || got.status != c.status {
			t.Errorf("semver_update_decision %q %q = %q (status %d), want %q (status %d)",
				c.current, c.next, got.out, got.status, c.want, c.status)
		}
	}
}

// libraryBlock returns the lines between "# BEGIN semver.sh" and
// "# END semver.sh", inclusive, with the indentation of the BEGIN line
// removed.
func libraryBlock(t *testing.T, path string) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	start, end := -1, -1
	indent := ""
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "# BEGIN semver.sh" {
			if start >= 0 {
				t.Fatalf("%s: two BEGIN semver.sh lines", path)
			}
			start = i
			indent = line[:len(line)-len(trimmed)]
		}
		if trimmed == "# END semver.sh" && start >= 0 && end < 0 {
			end = i
		}
	}
	if start < 0 || end < 0 {
		t.Fatalf("%s: no BEGIN/END semver.sh block", path)
	}
	block := make([]string, 0, end-start+1)
	for _, line := range lines[start : end+1] {
		if line == "" {
			block = append(block, line)
			continue
		}
		if !strings.HasPrefix(line, indent) {
			t.Fatalf("%s: line %q is not indented like the block", path, line)
		}
		block = append(block, strings.TrimPrefix(line, indent))
	}
	return block
}

func TestScoopBucketWorkflowCopiesTheSemverLibrary(t *testing.T) {
	want := libraryBlock(t, filepath.Join(repoRoot(t), ".github", "scripts", "lib", "semver.sh"))
	got := libraryBlock(t, filepath.Join(repoRoot(t), "packaging", "scoop", "update.yml"))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("packaging/scoop/update.yml must copy the semver.sh block verbatim\n--- got ---\n%s\n--- want ---\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// versionSortRE matches a version sort in a pipeline, such as
// `printf ... | sort -V | tail -n1`, not a mention of it in a comment.
var versionSortRE = regexp.MustCompile(`\|\s*sort\s+(?:-[A-Za-z]*V[A-Za-z]*|--version-sort)`)

func TestPackagingNeverOrdersVersionsWithSortV(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{filepath.Join(root, ".github"), filepath.Join(root, "packaging")} {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if m := versionSortRE.Find(body); m != nil {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s orders versions with %q; use .github/scripts/lib/semver.sh", rel, m)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
