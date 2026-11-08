package filter

import (
	"math/rand/v2"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"
)

// matchSegmentsReference is the recursive definition that matchSegments
// replaced (D7 of the v0.4.0-rc1 plan): "**" matches zero or more segments,
// any other segment matches one segment through path.Match. Tests compare
// the two; it is exponential, so only on small inputs.
func matchSegmentsReference(pattern, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if pattern[0] == "**" {
		if matchSegmentsReference(pattern[1:], value) {
			return true
		}
		return len(value) > 0 && matchSegmentsReference(pattern, value[1:])
	}
	if len(value) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], value[0])
	return err == nil && matched && matchSegmentsReference(pattern[1:], value[1:])
}

// sequences returns every sequence of at most maxLen items of alphabet.
func sequences(alphabet []string, maxLen int) [][]string {
	out := [][]string{{}}
	frontier := [][]string{{}}
	for range maxLen {
		var next [][]string
		for _, s := range frontier {
			for _, item := range alphabet {
				next = append(next, append(append([]string(nil), s...), item))
			}
		}
		out = append(out, next...)
		frontier = next
	}
	return out
}

// TestMatchSegmentsIsTheRecursiveDefinition compares the two on every
// pattern of up to five segments over "**", a literal, "*" and a class, and
// every path of up to five segments over two names: 86,000 cases.
func TestMatchSegmentsIsTheRecursiveDefinition(t *testing.T) {
	patterns := sequences([]string{"**", "a", "*", "[ab]"}, 5)
	values := sequences([]string{"a", "b"}, 5)
	for _, p := range patterns {
		for _, v := range values {
			if got, want := matchSegments(p, v), matchSegmentsReference(p, v); got != want {
				t.Fatalf("matchSegments(%q, %q) = %t, the definition gives %t", p, v, got, want)
			}
		}
	}
}

// TestCollapsedPatternsMatchTheSame: collapsing runs of "**" at compile time
// changes no result.
func TestCollapsedPatternsMatchTheSame(t *testing.T) {
	patterns := sequences([]string{"**", "a", "*"}, 6)
	values := sequences([]string{"a", "b"}, 4)
	for _, p := range patterns {
		collapsed := collapseDoubleStars(append([]string(nil), p...))
		for _, v := range values {
			if got, want := matchSegments(collapsed, v), matchSegmentsReference(p, v); got != want {
				t.Fatalf("collapsed %q gives %t on %q, the definition of %q gives %t", collapsed, got, v, p, want)
			}
		}
	}
	if got := collapseDoubleStars([]string{"**", "**", "a", "**", "**", "**"}); len(got) != 3 {
		t.Fatalf("collapseDoubleStars = %q", got)
	}
}

// TestMatchSegmentsOnRandomInputs widens the alphabet: "?", prefixes,
// classes that match nothing, invalid patterns, longer sequences.
func TestMatchSegmentsOnRandomInputs(t *testing.T) {
	patternItems := []string{"**", "**", "a", "b", "ab", "*", "?", "a*", "*b", "[ab]", "[!a]", "[", "x"}
	valueItems := []string{"a", "b", "ab", "ba", "abc", "x"}
	rng := rand.New(rand.NewPCG(5, 0xd7))
	pick := func(items []string, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = items[rng.IntN(len(items))]
		}
		return out
	}
	for range 200_000 {
		p := pick(patternItems, rng.IntN(9))
		v := pick(valueItems, rng.IntN(9))
		if got, want := matchSegments(p, v), matchSegmentsReference(p, v); got != want {
			t.Fatalf("matchSegments(%q, %q) = %t, the definition gives %t", p, v, got, want)
		}
	}
}

// FuzzMatchSegments compares the two on arbitrary slash-separated inputs of
// at most ten segments each.
func FuzzMatchSegments(f *testing.F) {
	for _, seed := range [][2]string{{"**/a", "x/a"}, {"a/**/b", "a/b"}, {"**/**/*", "a/b"}, {"[ab]/**", "c"}, {"**", ""}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, pattern, value string) {
		p := strings.Split(pattern, "/")
		v := strings.Split(value, "/")
		if value == "" {
			v = nil
		}
		if len(p) > 10 || len(v) > 10 {
			return
		}
		if got, want := matchSegments(p, v), matchSegmentsReference(p, v); got != want {
			t.Fatalf("matchSegments(%q, %q) = %t, the definition gives %t", p, v, got, want)
		}
	})
}

// repeat returns n copies of segments, one after the other.
func repeat(n int, segments ...string) []string {
	out := make([]string, 0, n*len(segments))
	for range n {
		out = append(out, segments...)
	}
	return out
}

// TestMatchSegmentsBacktracksInPolynomialTime: 2,000 "**" before a segment
// that never matches cost the recursive matcher about 10^9 calls at depth 2
// and 10^12 at depth 3 (F4 of the D7 audit). The iterative one answers at
// once, on the uncollapsed pattern and through the matcher, which collapses
// it.
func TestMatchSegmentsBacktracksInPolynomialTime(t *testing.T) {
	matcher, err := NewIgnoreMatcher([]string{strings.Repeat("**/", 2000) + "nothing"})
	if err != nil {
		t.Fatal(err)
	}
	pattern := append(repeat(2000, "**"), "nothing")
	done := make(chan bool, 1)
	go func() {
		done <- matchSegments(pattern, []string{"a", "b", "c"}) || matchSegments(pattern, strings.Split("a/b/c/d/e/f/g/h", "/")) ||
			matcher.Match("a/b/c/d/e/f/g/h", true)
	}()
	select {
	case matched := <-done:
		if matched {
			t.Fatal("the pattern matched a path that has no segment named nothing")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("matching 2,000 \"**\" did not end within 5 s")
	}
}

// TestCompiledStarRunsCostNothingPerEntry: a pattern of one million "**"
// compiles to two segments, so 20,000 entries cost 20,000 short matches,
// not 2·10^10 steps (D7 of the v0.4.0-rc1 plan).
func TestCompiledStarRunsCostNothingPerEntry(t *testing.T) {
	matcher, err := NewIgnoreMatcher([]string{strings.Repeat("**/", 1_000_000) + "x"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for i := range 20_000 {
		if matcher.Match("dir/file-"+strconv.Itoa(i), false) {
			t.Fatal("matched an entry that is not x")
		}
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("20,000 entries took %s", elapsed)
	}
}

// TestMatchSegmentsStackDoesNotGrowWithThePattern: a snapshot capture can
// hold a pattern of millions of segments, each of which was a frame of the
// recursive matcher, and overflowing the stack is fatal, beyond any recover
// (F1 of the D7 audit). With the stack capped at 1 MiB, patterns of one
// million segments must match and fail as they should; the recursion would
// end the test binary. The patterns are given to matchSegments uncollapsed,
// and one alternates "**" with a literal, which no collapsing shortens.
func TestMatchSegmentsStackDoesNotGrowWithThePattern(t *testing.T) {
	stars := append(repeat(1_000_000, "**"), "x")
	alternating := repeat(500_000, "**", "a")
	matcher, err := NewIgnoreMatcher([]string{strings.Repeat("**/", 1_000_000) + "x"})
	if err != nil {
		t.Fatal(err)
	}
	defer debug.SetMaxStack(debug.SetMaxStack(1 << 20))
	if !matchSegments(stars, []string{"a", "b", "x"}) || matchSegments(stars, []string{"a", "b", "y"}) {
		t.Fatal("one million \"**\" then x does not match a/b/x alone")
	}
	if matchSegments(alternating, []string{"a", "a", "x"}) {
		t.Fatal("500,000 \"**/a\" matched a/a/x")
	}
	if !matcher.Match("a/b/x", false) || matcher.Match("a/b/y", false) {
		t.Fatal("the compiled pattern does not match a/b/x alone")
	}
}
