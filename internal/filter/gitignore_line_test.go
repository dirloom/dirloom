package filter

import (
	"strings"
	"testing"

	gitignore "github.com/git-pkgs/gitignore"

	"github.com/dirloom/dirloom/internal/artifact"
)

func TestLongGitIgnoreLine(t *testing.T) {
	fits := strings.Repeat("a", MaxGitIgnoreLine)
	above := fits + "a"
	for _, tc := range []struct {
		name string
		data string
		want int
	}{
		{"empty", "", 0},
		{"short lines", "a\nb\n\nc", 0},
		{"the longest line, then a rule", fits + "\nafter\n", 0},
		{"the longest line, last without a newline", "before\n" + fits, 0},
		{"the longest line, carriage return counted", fits[1:] + "\r\nafter\r\n", 0},
		{"one byte above, first line", above + "\nafter\n", 1},
		{"one byte above, last line without a newline", "a\nb\n" + above, 3},
		{"carriage return pushes it above", fits + "\r\nafter\n", 1},
		{"the first long line is named", "a\n" + above + "\n" + above + "\n", 2},
	} {
		if got := LongGitIgnoreLine([]byte(tc.data)); got != tc.want {
			t.Errorf("%s: line %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestMaxGitIgnoreLineIsTheMatcherLimit pins MaxGitIgnoreLine to the
// dependency: it reads a line of that length and the rules after it, and
// one byte more silently drops both. If an update lifts the limit, the
// second half fails, and Dirloom's refusal can go.
func TestMaxGitIgnoreLineIsTheMatcherLimit(t *testing.T) {
	read := func(length int, ending string) (longRule, after bool) {
		long := strings.Repeat("a", length-len(strings.TrimSuffix(ending, "\n")))
		matcher := gitignore.New("")
		matcher.AddPatterns([]byte("/"+long[1:]+ending+"after.txt\n"), "")
		return matcher.MatchPath(long[1:], false), matcher.MatchPath("after.txt", false)
	}
	for _, ending := range []string{"\n", "\r\n"} {
		if long, after := read(MaxGitIgnoreLine, ending); !long || !after {
			t.Fatalf("line of %d bytes with %q: long rule %v, rule after it %v", MaxGitIgnoreLine, ending, long, after)
		}
		if long, after := read(MaxGitIgnoreLine+1, ending); long || after {
			t.Fatalf("line of %d bytes with %q: long rule %v, rule after it %v; the dependency lifted its limit", MaxGitIgnoreLine+1, ending, long, after)
		}
	}

	// A last line without a newline has the same limit.
	last := func(length int) bool {
		matcher := gitignore.New("")
		matcher.AddPatterns([]byte("/"+strings.Repeat("a", length-1)), "")
		return matcher.MatchPath(strings.Repeat("a", length-1), false)
	}
	if !last(MaxGitIgnoreLine) || last(MaxGitIgnoreLine+1) {
		t.Fatalf("last line: %d bytes read %v, %d bytes read %v", MaxGitIgnoreLine, last(MaxGitIgnoreLine), MaxGitIgnoreLine+1, last(MaxGitIgnoreLine+1))
	}
}

// TestAddPatternsRefusesALongLine is the last guard: a line the matcher
// would drop never reaches it silently.
func TestAddPatternsRefusesALongLine(t *testing.T) {
	matcher := NewGitIgnore()
	data := []byte("first.txt\n" + strings.Repeat("a", MaxGitIgnoreLine+1) + "\nafter.txt\n")
	err := matcher.AddPatterns(data, "", "sub/.gitignore")
	if !artifact.IsInternal(err) || !strings.Contains(err.Error(), `"sub/.gitignore"`) || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("AddPatterns: %v", err)
	}
	if matcher.Match("first.txt", false) {
		t.Fatal("a refused file added some of its rules")
	}
	if err := matcher.AddPatterns([]byte(strings.Repeat("a", MaxGitIgnoreLine)+"\nafter.txt\n"), "", ".gitignore"); err != nil || !matcher.Match("after.txt", false) {
		t.Fatalf("the longest line: %v, rule after it %v", err, matcher.Match("after.txt", false))
	}
}
