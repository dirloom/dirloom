package filter

import "testing"

func TestGitIgnoreConformance(t *testing.T) {
	matcher := NewGitIgnore()
	rootPatterns := []byte(`# comment
*.log
!important.log
/root-only
cache/
docs/**/generated.txt
\!literal
name\ with\ spaces.txt
`)
	if err := matcher.AddPatterns(rootPatterns, "", ".gitignore"); err != nil {
		t.Fatal(err)
	}
	if err := matcher.AddPatterns([]byte("*.tmp\n!keep.tmp\n"), "nested", "nested/.gitignore"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		dir     bool
		ignored bool
	}{
		{"debug.log", false, true},
		{"nested/debug.log", false, true},
		{"important.log", false, false},
		{"root-only", false, true},
		{"nested/root-only", false, false},
		{"cache", true, true},
		{"cache/file.txt", false, true},
		{"docs/generated.txt", false, true},
		{"docs/a/b/generated.txt", false, true},
		{"!literal", false, true},
		{"name with spaces.txt", false, true},
		{"nested/file.tmp", false, true},
		{"nested/keep.tmp", false, false},
		{"sibling/file.tmp", false, false},
	}

	for _, test := range tests {
		if got := matcher.Match(test.path, test.dir); got != test.ignored {
			t.Errorf("Match(%q, %v) = %v, want %v", test.path, test.dir, got, test.ignored)
		}
	}
}

func TestGitIgnoreLaterRulesWinOnlyInsideLayer(t *testing.T) {
	matcher := NewGitIgnore()
	if err := matcher.AddPatterns([]byte("*.log\n!important.log\n"), "", ".gitignore"); err != nil {
		t.Fatal(err)
	}
	if matcher.Match("important.log", false) {
		t.Fatal("negation should re-include within the gitignore layer")
	}
}

func TestGitIgnoreInvalidPatternIsANonMatch(t *testing.T) {
	matcher := NewGitIgnore()
	if err := matcher.AddPatterns([]byte("[[:unknown:]]\n"), "", ".gitignore"); err != nil {
		t.Fatal(err)
	}
	if matcher.Match("file.txt", false) {
		t.Fatal("invalid Git pattern should be skipped")
	}
}
